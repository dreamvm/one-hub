package claude_test

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/base"
	bedrock "one-api/providers/bedrock/category"
	"one-api/providers/claude"
	vertex "one-api/providers/vertexai/category"
	"one-api/types"
)

func request(t *testing.T, input string) *types.ChatCompletionRequest {
	t.Helper()
	var r types.ChatCompletionRequest
	require.NoError(t, json.Unmarshal([]byte(input), &r))
	r.Model = "fixture"
	r.MaxTokens = 4096
	return &r
}

func TestClaudeNonStreamSignedRoundTrip(t *testing.T) {
	var response claude.ClaudeResponse
	require.NoError(t, json.Unmarshal([]byte(`{"id":"fixture","role":"assistant","stop_reason":"tool_use","usage":{"input_tokens":2,"output_tokens":3},"content":[{"type":"thinking","thinking":"synthetic","signature":"sig"},{"type":"text","text":"检查"},{"type":"tool_use","id":"a","name":"read_file","input":{}},{"type":"redacted_thinking","data":"opaque"},{"type":"text","text":"文件"},{"type":"tool_use","id":"b","name":"list_files","input":{}}]}`), &response))
	r := request(t, `{"messages":[]}`)
	p := &claude.ClaudeProvider{BaseProvider: base.BaseProvider{Usage: &types.Usage{}}}
	out, e := claude.ConvertToChatOpenai(p, &response, r)
	require.Nil(t, e)
	require.Len(t, out.Choices, 1)
	message := out.Choices[0].Message
	require.Equal(t, "检查文件", message.Content)
	require.Len(t, message.ToolCalls, 2)
	r.Messages = []types.ChatCompletionMessage{message, {Role: "tool", ToolCallID: "b", Content: "B"}, {Role: "tool", ToolCallID: "a", Content: "A"}}
	next, e := claude.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	wire, _ := json.Marshal(next.Messages[0].Content)
	original, _ := json.Marshal(response.Content)
	require.JSONEq(t, string(original), string(wire))
	message.ToolCalls[0].Function.Arguments = `{"changed":true}`
	_, e = claude.ConvertFromChatOpenai(r)
	require.NotNil(t, e, "do not replay stale signed blocks")
}

func TestClaudeInvalidHistoryAndControls(t *testing.T) {
	for _, input := range []string{
		`{"messages":[{"role":"tool","tool_call_id":"missing","content":"bad"}]}`,
		`{"messages":[{"role":"assistant","tool_calls":[{"id":"a","function":{"name":"x","arguments":"{}"}}]}]}`,
		`{"messages":[{"role":"assistant","tool_calls":[null]}]}`,
		`{"messages":[],"tool_choice":{"type":"function","function":{"name":7}}}`,
		`{"messages":[],"tool_choice":"required"}`,
		`{"messages":[],"tools":[null]}`,
		`{"messages":[],"reasoning":{"max_tokens":1024},"tool_choice":"required","tools":[{"type":"function","function":{"name":"x"}}]}`,
	} {
		t.Run(input, func(t *testing.T) {
			require.NotPanics(t, func() {
				_, e := claude.ConvertFromChatOpenai(request(t, input))
				require.NotNil(t, e)
				require.Equal(t, 400, e.StatusCode)
			})
		})
	}
}

func TestClaudeMediaAndNativeCloudEnvelopes(t *testing.T) {
	r := request(t, `{"messages":[{"role":"assistant","tool_calls":[{"id":"a","function":{"name":"read","arguments":"{}"}}]},{"role":"tool","tool_call_id":"a","is_error":true,"content":[{"type":"text","text":"失败"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"fixture"}},{"type":"document","source":{"type":"text","media_type":"text/plain","data":"文件内容"}}]}]}`)
	out, e := claude.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	b, _ := json.Marshal(out)
	require.Contains(t, string(b), `"is_error":true`)
	require.Contains(t, string(b), "文件内容")
	var native claude.ClaudeRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"fixture","max_tokens":2000,"thinking":{"type":"adaptive","display":"omitted"},"output_config":{"effort":"high"},"tools":[{"name":"read","strict":true,"input_examples":[{}],"defer_loading":true,"allowed_callers":["direct"],"input_schema":{"type":"object"}}],"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"synthetic","signature":"sig"}]}]}`), &native))
	for _, envelope := range []any{native, bedrock.ClaudeRequest{ClaudeRequest: &native, AnthropicVersion: bedrock.AnthropicVersion}, vertex.ClaudeRequest{ClaudeRequest: &native, AnthropicVersion: vertex.AnthropicVersion}} {
		b, e := json.Marshal(envelope)
		require.NoError(t, e)
		for _, key := range []string{"strict", "input_examples", "defer_loading", "allowed_callers", "output_config", "signature", "omitted"} {
			require.Contains(t, string(b), key)
		}
		if _, ok := envelope.(claude.ClaudeRequest); !ok {
			require.Contains(t, string(b), "anthropic_version")
		}
	}
}

func TestClaudeTruncatedArgumentsAreNotSuccessful(t *testing.T) {
	h := claude.ClaudeStreamHandler{Request: request(t, `{"messages":[]}`), Usage: &types.Usage{}, Prefix: `data:`}
	data := make(chan string, 10)
	errs := make(chan error, 10)
	for _, event := range []string{`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"a","name":"write","input":{}}}`, `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"script\":\"中文"}}`, `{"type":"content_block_stop","index":0}`, `{"type":"message_stop"}`} {
		b := []byte("data:" + event)
		h.HandlerStream(&b, data, errs)
	}
	require.NotEmpty(t, errs)
	require.NotErrorIs(t, <-errs, io.EOF)
	for len(data) > 0 {
		var c types.ChatCompletionStreamResponse
		require.NoError(t, json.Unmarshal([]byte(<-data), &c))
		for _, choice := range c.Choices {
			require.Nil(t, choice.FinishReason)
		}
	}
}

func TestClaudeToolControls(t *testing.T) {
	r := request(t, `{"tool_choice":"none","parallel_tool_calls":false,"tools":[{"type":"function","function":{"name":"write_file","strict":true,"parameters":{"type":"object","properties":{}}}}],"messages":[{"role":"user","content":"test"}]}`)
	c, e := claude.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	require.Equal(t, "none", c.ToolChoice.Type)
	require.True(t, c.ToolChoice.DisableParallelToolUse)
	b, _ := json.Marshal(c.Tools)
	require.Contains(t, string(b), `"strict":true`)
}

func TestClaudeToolHistory(t *testing.T) {
	r := request(t, `{"messages":[{"role":"assistant","content":"先读取","tool_calls":[{"id":"a","type":"function","function":{"name":"read_file","arguments":"{}"}},{"id":"b","type":"function","function":{"name":"list_files","arguments":"{}"}}]},{"role":"tool","tool_call_id":"a","content":"A"},{"role":"tool","tool_call_id":"b","content":"B"}]}`)
	c, e := claude.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	require.Len(t, c.Messages, 2)
	b, _ := json.Marshal(c.Messages)
	require.Contains(t, string(b), "先读取")
}

func TestClaudeStreamToolRoundTrip(t *testing.T) {
	r := request(t, `{"messages":[{"role":"user","content":"test"}]}`)
	h := claude.ClaudeStreamHandler{Request: r, Usage: &types.Usage{}, Prefix: `data: {"type"`}
	data := make(chan string, 64)
	errs := make(chan error, 64)
	for _, event := range []string{
		`{"type":"message_start","message":{"id":"fixture-id","role":"assistant","usage":{"input_tokens":12}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"synthetic thought"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"synthetic-signature"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":"先写文件"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"a","name":"write_file","input":{}}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"text\":\"中文"}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\\n正文\"}"}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"content_block_start","index":3,"content_block":{"type":"tool_use","id":"b","name":"list_files","input":{}}}`,
		`{"type":"content_block_stop","index":3}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":25}}`,
		`{"type":"message_stop"}`,
	} {
		line := []byte("data: " + event)
		h.HandlerStream(&line, data, errs)
	}
	for len(errs) > 0 {
		require.ErrorIs(t, <-errs, io.EOF)
	}
	var message types.ChatCompletionMessage
	message.Role = "assistant"
	calls := make(map[int]*types.ChatCompletionToolCalls)
	var metadata json.RawMessage
	for len(data) > 0 {
		wire := <-data
		var chunk types.ChatCompletionStreamResponse
		require.NoError(t, json.Unmarshal([]byte(wire), &chunk))
		require.Equal(t, "fixture-id", chunk.ID)
		for _, choice := range chunk.Choices {
			require.Zero(t, choice.Index)
			message.Content = message.StringContent() + choice.Delta.Content
			for _, delta := range choice.Delta.ToolCalls {
				if calls[delta.Index] == nil {
					calls[delta.Index] = &types.ChatCompletionToolCalls{Type: "function", Function: &types.ChatCompletionToolCallsFunction{}}
				}
				call := calls[delta.Index]
				if delta.Id != "" {
					call.Id = delta.Id
				}
				if delta.Function.Name != "" {
					call.Function.Name = delta.Function.Name
				}
				call.Function.Arguments += delta.Function.Arguments
				if len(delta.ExtraContent) > 0 {
					call.ExtraContent = delta.ExtraContent
					metadata = delta.ExtraContent
				}
			}
		}
	}
	require.Len(t, calls, 2)
	require.Equal(t, "a", calls[0].Id)
	require.Equal(t, "b", calls[1].Id)
	require.JSONEq(t, `{"text":"中文\n正文"}`, calls[0].Function.Arguments)
	require.JSONEq(t, `{}`, calls[1].Function.Arguments)
	require.Contains(t, string(metadata), "synthetic-signature")
	message.ToolCalls = []*types.ChatCompletionToolCalls{calls[0], calls[1]}
	r.Messages = []types.ChatCompletionMessage{message, {Role: "tool", ToolCallID: "a", Content: "ok"}, {Role: "tool", ToolCallID: "b", Content: "ok"}}
	next, e := claude.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	b, _ := json.Marshal(next.Messages)
	require.Contains(t, string(b), `"signature":"synthetic-signature"`)
	require.Contains(t, string(b), "synthetic thought")
	require.Contains(t, string(b), "先写文件")
}
