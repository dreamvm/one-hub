package claude

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"one-api/common/image"
	"one-api/types"
)

type assistantMetadata struct {
	Anthropic *struct {
		Content []json.RawMessage `json:"content"`
	} `json:"anthropic,omitempty"`
}

func packAssistant(content any) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"anthropic": map[string]any{"content": content}})
	return b
}

func replayAssistant(msg *types.ChatCompletionMessage) ([]json.RawMessage, error) {
	if msg.Role != types.ChatMessageRoleAssistant {
		return nil, nil
	}
	extra := msg.ExtraContent
	if len(extra) == 0 && len(msg.ToolCalls) > 0 && msg.ToolCalls[0] != nil {
		extra = msg.ToolCalls[0].ExtraContent
	}
	if len(extra) == 0 {
		return nil, nil
	}
	var metadata assistantMetadata
	if json.Unmarshal(extra, &metadata) != nil {
		return nil, fmt.Errorf("invalid assistant metadata")
	}
	if metadata.Anthropic == nil {
		return nil, nil
	}
	var calls []ResContent
	var text strings.Builder
	for _, raw := range metadata.Anthropic.Content {
		var c ResContent
		if json.Unmarshal(raw, &c) != nil {
			return nil, fmt.Errorf("invalid assistant content")
		}
		switch c.Type {
		case "tool_use":
			calls = append(calls, c)
		case "thinking":
			if c.Signature == "" {
				return nil, fmt.Errorf("thinking signature is missing")
			}
		case "redacted_thinking":
			if c.Data == "" {
				return nil, fmt.Errorf("redacted thinking data is missing")
			}
		case "text":
			text.WriteString(c.Text)
		default:
			return nil, fmt.Errorf("unsupported replay block type")
		}
	}
	if text.String() != msg.StringContent() {
		return nil, fmt.Errorf("assistant text changed since signed response; remove stale metadata before editing")
	}
	if len(calls) != len(msg.ToolCalls) {
		return nil, fmt.Errorf("assistant metadata does not match tool calls")
	}
	for i, c := range calls {
		call := msg.ToolCalls[i]
		if call == nil || call.Function == nil || c.Id != call.Id || c.Name != call.Function.Name {
			return nil, fmt.Errorf("assistant metadata does not match tool calls")
		}
		var args any
		if json.Unmarshal([]byte(call.Function.Arguments), &args) != nil || !reflect.DeepEqual(args, c.Input) {
			return nil, fmt.Errorf("tool arguments changed since signed assistant response")
		}
	}
	return metadata.Anthropic.Content, nil
}

func normalizeToolHistory(request *ClaudeRequest) error {
	var messages []Message
	pending := make(map[string]bool)
	lastResults := false
	for _, message := range request.Messages {
		b, err := json.Marshal(message.Content)
		if err != nil {
			return err
		}
		var blocks []map[string]any
		if json.Unmarshal(b, &blocks) != nil {
			return fmt.Errorf("invalid message blocks")
		}
		onlyResults := len(blocks) > 0
		for _, block := range blocks {
			if block["type"] != "tool_result" {
				onlyResults = false
			}
		}
		if onlyResults {
			if message.Role != "user" {
				return fmt.Errorf("tool results require user role")
			}
			for _, block := range blocks {
				id, _ := block["tool_use_id"].(string)
				if id == "" || !pending[id] {
					return fmt.Errorf("tool result has missing, duplicate or unmatched tool_use_id")
				}
				delete(pending, id)
			}
			if lastResults {
				previous := messages[len(messages)-1].Content.([]map[string]any)
				messages[len(messages)-1].Content = append(previous, blocks...)
			} else {
				message.Content = blocks
				messages = append(messages, message)
			}
			lastResults = true
			continue
		}
		if len(pending) > 0 {
			return fmt.Errorf("all tool results must immediately follow the assistant tool calls")
		}
		lastResults = false
		for _, block := range blocks {
			if block["type"] == "tool_use" {
				id, _ := block["id"].(string)
				if message.Role != "assistant" || id == "" || pending[id] {
					return fmt.Errorf("invalid or duplicate tool use")
				}
				pending[id] = true
			}
		}
		messages = append(messages, message)
	}
	if len(pending) > 0 {
		return fmt.Errorf("tool results are missing")
	}
	request.Messages = messages
	return nil
}

func toolResultContent(value any) (any, error) {
	if value == nil {
		return "", nil
	}
	if text, ok := value.(string); ok {
		return text, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var blocks []map[string]any
	if json.Unmarshal(b, &blocks) != nil {
		return nil, fmt.Errorf("tool result must be text or content blocks")
	}
	for i, block := range blocks {
		switch block["type"] {
		case "text":
			if _, ok := block["text"].(string); !ok {
				return nil, fmt.Errorf("invalid text result")
			}
		case "image", "document":
			if _, ok := block["source"].(map[string]any); !ok {
				return nil, fmt.Errorf("missing media source")
			}
		case "image_url":
			img, _ := block["image_url"].(map[string]any)
			url, _ := img["url"].(string)
			if url == "" {
				return nil, fmt.Errorf("missing image URL")
			}
			mime, data, e := image.GetImageFromUrl(url)
			if e != nil {
				return nil, e
			}
			kind := "image"
			if mime == "application/pdf" {
				kind = "document"
			}
			blocks[i] = map[string]any{"type": kind, "source": map[string]any{"type": "base64", "media_type": mime, "data": data}}
		case "file":
			file, _ := block["file"].(map[string]any)
			data, _ := file["file_data"].(string)
			const prefix = "data:application/pdf;base64,"
			if !strings.HasPrefix(data, prefix) {
				return nil, fmt.Errorf("file results require inline PDF data; native document blocks also supported")
			}
			blocks[i] = map[string]any{"type": "document", "source": map[string]any{"type": "base64", "media_type": "application/pdf", "data": strings.TrimPrefix(data, prefix)}}
		default:
			return nil, fmt.Errorf("unsupported tool result block")
		}
	}
	return blocks, nil
}
