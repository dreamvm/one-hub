package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/types"
)

type streamBlock struct {
	content   map[string]any
	toolIndex int
	arguments strings.Builder
	closed    bool
}
type streamState struct {
	id       string
	created  int64
	blocks   map[int]*streamBlock
	order    []int
	tools    int
	finished bool
	stopped  bool
	failed   bool
}

func (h *ClaudeStreamHandler) EndError() error {
	if h.state == nil || !h.state.finished || !h.state.stopped || h.state.failed {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// Content-block indices identify native blocks, not OpenAI choices or tools.
// Never synthesize a new call ID for a continuation fragment.
func (h *ClaudeStreamHandler) HandlerStream(rawLine *[]byte, dataChan chan string, errChan chan error) {
	line := bytes.TrimSpace(*rawLine)
	if bytes.HasPrefix(line, []byte("data:")) {
		line = bytes.TrimSpace(line[5:])
	} else if !strings.HasPrefix(h.Prefix, "{") || !bytes.HasPrefix(line, []byte("{")) {
		*rawLine = nil
		return
	}
	if h.state == nil {
		h.state = &streamState{id: "chatcmpl-" + utils.GetUUID(), created: utils.GetTimestamp(), blocks: make(map[int]*streamBlock)}
	}
	s := h.state
	if s.failed {
		*rawLine = requester.StreamClosed
		return
	}
	fail := func(message string) {
		s.failed = true
		errChan <- &requester.ProtocolError{Message: "Claude stream: " + message}
		*rawLine = requester.StreamClosed
	}
	var event struct {
		Type    string         `json:"type"`
		Index   int            `json:"index"`
		Message ClaudeResponse `json:"message"`
		Content map[string]any `json:"content_block"`
		Delta   Delta          `json:"delta"`
		Usage   Usage          `json:"usage"`
		Error   *ClaudeError   `json:"error"`
	}
	if json.Unmarshal(line, &event) != nil {
		fail("invalid event JSON")
		return
	}
	if event.Type == "error" {
		fail("upstream error event")
		return
	}
	if h.Usage == nil {
		h.Usage = &types.Usage{}
	}
	choice := types.ChatCompletionStreamChoice{Index: 0}
	switch event.Type {
	case "message_start":
		if event.Message.Id != "" {
			s.id = event.Message.Id
		}
		choice.Delta.Role = "assistant"
		h.Usage.PromptTokens = event.Message.Usage.InputTokens + event.Message.Usage.CacheCreationInputTokens + event.Message.Usage.CacheReadInputTokens
		h.Usage.PromptTokensDetails.CachedWriteTokens = event.Message.Usage.CacheCreationInputTokens
		h.Usage.PromptTokensDetails.CachedReadTokens = event.Message.Usage.CacheReadInputTokens
	case "content_block_start":
		if s.blocks[event.Index] != nil || event.Content == nil || s.finished {
			fail("duplicate or invalid content block")
			return
		}
		block := &streamBlock{content: event.Content, toolIndex: -1}
		s.blocks[event.Index] = block
		s.order = append(s.order, event.Index)
		switch event.Content["type"] {
		case "tool_use":
			id, _ := event.Content["id"].(string)
			name, _ := event.Content["name"].(string)
			if id == "" || name == "" {
				fail("tool name or ID missing")
				return
			}
			for _, i := range s.order[:len(s.order)-1] {
				if s.blocks[i].content["id"] == id {
					fail("duplicate tool ID")
					return
				}
			}
			block.toolIndex = s.tools
			s.tools++
			args := ""
			if input, ok := event.Content["input"].(map[string]any); ok && len(input) > 0 {
				b, _ := json.Marshal(input)
				args = string(b)
				block.arguments.WriteString(args)
			}
			choice.Delta.ToolCalls = []*types.ChatCompletionToolCalls{{Index: block.toolIndex, Id: id, Type: "function", Function: &types.ChatCompletionToolCallsFunction{Name: name, Arguments: args}}}
		case "text":
			choice.Delta.Content, _ = event.Content["text"].(string)
		case "thinking":
			if h.Request.Functions != nil {
				fail("signed thinking requires modern tools/tool_calls")
				return
			}
			choice.Delta.ReasoningContent, _ = event.Content["thinking"].(string)
		case "redacted_thinking":
			if h.Request.Functions != nil {
				fail("signed thinking requires modern tools/tool_calls")
				return
			}
			return
		default:
			fail("unsupported content block; use native Messages API")
			return
		}
	case "content_block_delta":
		block := s.blocks[event.Index]
		if block == nil || block.closed {
			fail("delta without open content block")
			return
		}
		switch event.Delta.Type {
		case "input_json_delta":
			if block.toolIndex < 0 {
				fail("tool delta on non-tool block")
				return
			}
			block.arguments.WriteString(event.Delta.PartialJson)
			choice.Delta.ToolCalls = []*types.ChatCompletionToolCalls{{Index: block.toolIndex, Function: &types.ChatCompletionToolCallsFunction{Arguments: event.Delta.PartialJson}}}
		case "text_delta":
			if block.content["type"] != "text" {
				fail("text delta on non-text block")
				return
			}
			old, _ := block.content["text"].(string)
			block.content["text"] = old + event.Delta.Text
			choice.Delta.Content = event.Delta.Text
		case "thinking_delta":
			if block.content["type"] != "thinking" {
				fail("thinking delta on non-thinking block")
				return
			}
			old, _ := block.content["thinking"].(string)
			block.content["thinking"] = old + event.Delta.Thinking
			choice.Delta.ReasoningContent = event.Delta.Thinking
		case "signature_delta":
			if block.content["type"] != "thinking" {
				fail("signature delta on non-thinking block")
				return
			}
			old, _ := block.content["signature"].(string)
			block.content["signature"] = old + event.Delta.Signature
			return
		default:
			fail("unsupported content delta")
			return
		}
	case "content_block_stop":
		block := s.blocks[event.Index]
		if block == nil || block.closed {
			fail("stop without open content block")
			return
		}
		block.closed = true
		if block.toolIndex < 0 {
			return
		}
		args := block.arguments.String()
		if args == "" {
			args = "{}"
			choice.Delta.ToolCalls = []*types.ChatCompletionToolCalls{{Index: block.toolIndex, Function: &types.ChatCompletionToolCallsFunction{Arguments: args}}}
		}
		var input map[string]any
		if json.Unmarshal([]byte(args), &input) != nil || input == nil {
			fail("incomplete or invalid tool arguments")
			return
		}
		block.content["input"] = input
		if choice.Delta.ToolCalls == nil {
			return
		}
	case "message_delta":
		if event.Delta.StopReason == "" {
			return
		}
		if s.finished {
			fail("duplicate message finish")
			return
		}
		var content []map[string]any
		for _, index := range s.order {
			block := s.blocks[index]
			if !block.closed {
				fail("message ended before content block completed")
				return
			}
			content = append(content, block.content)
		}
		s.finished = true
		metadata := packAssistant(content)
		choice.Delta.ExtraContent = metadata
		if s.tools > 0 {
			choice.Delta.ToolCalls = []*types.ChatCompletionToolCalls{{Index: 0, Function: &types.ChatCompletionToolCallsFunction{}, ExtraContent: metadata}}
		}
		choice.FinishReason = stopReasonClaude2OpenAI(event.Delta.StopReason)
		h.Usage.CompletionTokens = event.Usage.OutputTokens
		h.Usage.TotalTokens = h.Usage.PromptTokens + h.Usage.CompletionTokens
	case "message_stop":
		if !s.finished {
			fail("message stopped without final delta")
			return
		}
		s.stopped = true
		errChan <- io.EOF
		*rawLine = requester.StreamClosed
		return
	case "ping":
		return
	default:
		return
	}
	h.Usage.TextBuilder.WriteString(choice.Delta.Content)
	if h.Request.Functions != nil && choice.Delta.ToolCalls != nil {
		if s.tools > 1 {
			fail("legacy function_call cannot represent parallel tools")
			return
		}
		choice.Delta.FunctionCall = choice.Delta.ToolCalls[0].Function
		choice.Delta.ToolCalls = nil
		if choice.FinishReason == types.FinishReasonToolCalls {
			choice.FinishReason = types.FinishReasonFunctionCall
		}
	}
	response := types.ChatCompletionStreamResponse{ID: s.id, Object: "chat.completion.chunk", Created: s.created, Model: h.Request.Model, Choices: []types.ChatCompletionStreamChoice{choice}}
	if s.finished {
		response.Usage = h.Usage
	}
	b, err := json.Marshal(response)
	if err != nil {
		fail("cannot encode converted response")
		return
	}
	dataChan <- string(b)
}
