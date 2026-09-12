package gemini_test

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/gemini"
	"one-api/types"
)

func TestGeminiMultipleToolEventsFinishOnce(t *testing.T) {
	h := gemini.GeminiStreamHandler{Request: &types.ChatCompletionRequest{Model: "fixture"}, Usage: &types.Usage{}}
	data := make(chan string, 30)
	errs := make(chan error, 5)
	for _, event := range []string{
		`{"responseId":"fixture","candidates":[{"index":0,"content":{"parts":[{"text":"开始"},{"functionCall":{"name":"write_file","args":{"text":"中文\n正文"}},"thoughtSignature":"sig-a"}]}}]}`,
		`{"responseId":"fixture","candidates":[{"index":0,"content":{"parts":[{"functionCall":{"name":"list_files","args":{}},"thoughtSignature":"sig-b"}]},"finishReason":"STOP"}]}`,
	} {
		b := []byte("data: " + event)
		h.HandlerStream(&b, data, errs)
	}
	require.Empty(t, errs)
	finishes := 0
	names := make(map[int]string)
	var text string
	for len(data) > 0 {
		var c types.ChatCompletionStreamResponse
		require.NoError(t, json.Unmarshal([]byte(<-data), &c))
		for _, choice := range c.Choices {
			text += choice.Delta.Content
			if choice.FinishReason != nil {
				finishes++
				require.Equal(t, "tool_calls", choice.FinishReason)
			}
			for _, tool := range choice.Delta.ToolCalls {
				if tool.Function.Name != "" {
					names[tool.Index] = tool.Function.Name
				}
			}
		}
	}
	require.Equal(t, 1, finishes)
	require.Equal(t, map[int]string{0: "write_file", 1: "list_files"}, names)
	require.Equal(t, "开始", text)
	require.NoError(t, h.EndError())
}

func TestGeminiEOFRequiresFinish(t *testing.T) {
	h := gemini.GeminiStreamHandler{Request: &types.ChatCompletionRequest{Model: "fixture"}}
	require.ErrorIs(t, h.EndError(), io.ErrUnexpectedEOF)
	data, errs := make(chan string, 10), make(chan error, 5)
	b := []byte(`data: {"candidates":[{"content":{"parts":[{"text":"partial"}]}}]}`)
	h.HandlerStream(&b, data, errs)
	require.Empty(t, errs)
	require.ErrorIs(t, h.EndError(), io.ErrUnexpectedEOF)
}

func TestGeminiSSEWhitespaceAndMalformedJSON(t *testing.T) {
	for _, prefix := range []string{"data:", "data: ", "data:\t"} {
		h := gemini.GeminiStreamHandler{Request: &types.ChatCompletionRequest{Model: "fixture"}}
		data, errs := make(chan string, 10), make(chan error, 5)
		b := []byte(prefix + `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`)
		h.HandlerStream(&b, data, errs)
		require.Empty(t, errs)
		require.NoError(t, h.EndError())
		b = []byte(prefix + `{"invalid`)
		h.HandlerStream(&b, data, errs)
		var wire map[string]any
		require.NoError(t, json.Unmarshal([]byte((<-errs).Error()), &wire))
		require.Contains(t, wire, "error")
		require.Equal(t, "stream_closed", string(b))
	}
}

func TestGeminiRejectsIncompleteFunctionStream(t *testing.T) {
	for _, fragment := range []string{`{"name":"write_file","willContinue":true}`, `{"name":"write_file","partialArgs":[{"jsonPath":"$.text","stringValue":"中文"}]}`} {
		h := gemini.GeminiStreamHandler{Request: &types.ChatCompletionRequest{Model: "fixture"}, Usage: &types.Usage{}}
		data := make(chan string, 10)
		errs := make(chan error, 5)
		b := []byte(`data: {"candidates":[{"index":0,"content":{"parts":[{"functionCall":` + fragment + `}]}}]}`)
		h.HandlerStream(&b, data, errs)
		require.NotEmpty(t, errs)
		require.Empty(t, data, "never fabricate an empty completed call from an unsupported fragment")
	}
}

func TestGeminiNativeToolIDAndEmptyArguments(t *testing.T) {
	p := gemini.GeminiPart{FunctionCall: &gemini.GeminiFunctionCall{ID: "native-a", Name: "read_file"}}
	call := p.ToOpenAITool()
	require.Equal(t, "native-a", call.Id)
	require.JSONEq(t, `{}`, call.Function.Arguments)
	contents, _, e := gemini.OpenAIToGeminiChatContent([]types.ChatCompletionMessage{{Role: "assistant", ToolCalls: []*types.ChatCompletionToolCalls{call}}, {Role: "tool", ToolCallID: call.Id, Content: "ok"}})
	require.Nil(t, e)
	require.Equal(t, "native-a", contents[0].Parts[0].FunctionCall.ID)
	require.Equal(t, "native-a", contents[1].Parts[0].FunctionResponse.ID)
}
