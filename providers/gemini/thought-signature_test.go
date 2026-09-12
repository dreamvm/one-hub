package gemini_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/gemini"
	"one-api/types"
)

// reassembleToolCalls models a client receiving real JSON chunks and sending the
// assembled assistant message back on the next request. Fixtures contain no keys.
func reassembleToolCalls(t *testing.T, choice types.ChatCompletionStreamChoice) []*types.ChatCompletionToolCalls {
	t.Helper()
	assembled := make([]map[string]any, len(choice.Delta.ToolCalls))
	for _, chunk := range choice.ConvertOpenaiStream() {
		encoded, err := json.Marshal(chunk)
		require.NoError(t, err)
		var wire struct {
			Delta struct {
				ToolCalls []map[string]any `json:"tool_calls"`
			} `json:"delta"`
		}
		require.NoError(t, json.Unmarshal(encoded, &wire))
		for _, call := range wire.Delta.ToolCalls {
			index := int(call["index"].(float64))
			require.GreaterOrEqual(t, index, 0)
			require.Less(t, index, len(assembled))
			if assembled[index] == nil {
				assembled[index] = map[string]any{"function": map[string]any{"arguments": ""}}
			}
			target := assembled[index]
			for key, value := range call {
				if key != "function" {
					target[key] = value
				}
			}
			function := call["function"].(map[string]any)
			targetFunction := target["function"].(map[string]any)
			if name, ok := function["name"]; ok {
				targetFunction["name"] = name
			}
			targetFunction["arguments"] = targetFunction["arguments"].(string) + function["arguments"].(string)
		}
	}
	encoded, err := json.Marshal(assembled)
	require.NoError(t, err)
	var calls []*types.ChatCompletionToolCalls
	require.NoError(t, json.Unmarshal(encoded, &calls))
	return calls
}

func TestGeminiThoughtSignatureRoundTrip(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		signatures []string
	}{
		{"single_signed", []string{`"opaque-signature+/="`}},
		{"parallel_first_signed", []string{`"first-signature"`, ""}},
		{"parallel_both_signed", []string{`"first-signature"`, `"second-signature"`}},
		{"unsigned", []string{""}},
	} {
		for _, mode := range []string{"nonstream", "stream"} {
			t.Run(fixture.name+"/"+mode, func(t *testing.T) {
				candidate := gemini.GeminiChatCandidate{Content: gemini.GeminiChatContent{Role: "model"}}
				names := []string{"read_file", "list_files"}
				for i, signature := range fixture.signatures {
					candidate.Content.Parts = append(candidate.Content.Parts, gemini.GeminiPart{
						FunctionCall: &gemini.GeminiFunctionCall{
							Name: names[i], Args: map[string]interface{}{"path": "中文文档.txt"},
						},
						ThoughtSignature: json.RawMessage(signature),
					})
				}
				request := &types.ChatCompletionRequest{Model: "gemini-test-fixture"}
				var calls []*types.ChatCompletionToolCalls
				if mode == "stream" {
					choice := candidate.ToOpenAIStreamChoice(request)
					require.Equal(t, types.FinishReasonToolCalls, choice.FinishReason)
					calls = reassembleToolCalls(t, choice)
				} else {
					choice := candidate.ToOpenAIChoice(request)
					require.Equal(t, types.FinishReasonToolCalls, choice.FinishReason)
					calls = choice.Message.ToolCalls
				}
				require.Len(t, calls, len(fixture.signatures))
				messages := []types.ChatCompletionMessage{{Role: types.ChatMessageRoleAssistant, ToolCalls: calls}}
				ids := make(map[string]bool)
				for _, call := range calls {
					require.NotEmpty(t, call.Id)
					require.False(t, ids[call.Id], "parallel calls must keep distinct IDs")
					ids[call.Id] = true
					messages = append(messages, types.ChatCompletionMessage{
						Role: types.ChatMessageRoleTool, ToolCallID: call.Id, Content: "fixture result",
					})
				}
				wire, err := json.Marshal(messages)
				require.NoError(t, err)
				var nextRequest []types.ChatCompletionMessage
				require.NoError(t, json.Unmarshal(wire, &nextRequest))
				contents, _, apiErr := gemini.OpenAIToGeminiChatContent(nextRequest)
				require.Nil(t, apiErr)
				require.Len(t, contents, 2)
				require.Equal(t, "model", contents[0].Role)
				require.Len(t, contents[0].Parts, len(fixture.signatures))
				require.Len(t, contents[1].Parts, len(fixture.signatures))
				for i, signature := range fixture.signatures {
					part := contents[0].Parts[i]
					require.Equal(t, signature, string(part.ThoughtSignature))
					require.Equal(t, names[i], part.FunctionCall.Name)
					require.Equal(t, "中文文档.txt", part.FunctionCall.Args["path"])
					require.Equal(t, names[i], contents[1].Parts[i].FunctionResponse.Name)
				}
			})
		}
	}
}

func TestGeminiRequestExtraContentCompatibility(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		extra string
		want  string
	}{
		{"signed", `{"google":{"thought_signature":"opaque-signature"}}`, `"opaque-signature"`},
		{"unknown_fields", `{"google":{"thought_signature":"opaque-signature","future":true},"vendor":{"key":"value"}}`, `"opaque-signature"`},
		{"empty", `{}`, ""},
		{"null", `null`, ""},
		{"other_vendor", `{"vendor":{"thought_signature":"not-google"}}`, ""},
		{"invalid_google_type", `{"google":123}`, ""},
		{"invalid_extra_type", `[]`, ""},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			input := `[{"role":"assistant","tool_calls":[{"id":"call_fixture","type":"function","function":{"name":"get_info","arguments":"{}"},"extra_content":` + fixture.extra + `}]}]`
			var messages []types.ChatCompletionMessage
			require.NoError(t, json.Unmarshal([]byte(input), &messages))
			contents, _, apiErr := gemini.OpenAIToGeminiChatContent(messages)
			require.Nil(t, apiErr)
			require.Len(t, contents, 1)
			require.Len(t, contents[0].Parts, 1)
			require.Equal(t, fixture.want, string(contents[0].Parts[0].ThoughtSignature))
		})
	}
}

func TestGeminiSSEToolSignatureForwarding(t *testing.T) {
	handler := gemini.GeminiStreamHandler{
		Usage: &types.Usage{}, Request: &types.ChatCompletionRequest{Model: "gemini-test-fixture"},
	}
	line := []byte(`data: {"responseId":"response-fixture","candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"functionCall":{"name":"get_info","args":{}},"thoughtSignature":"sse-signature"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2,"totalTokenCount":12}}`)
	data := make(chan string, 8)
	errors := make(chan error, 1)
	handler.HandlerStream(&line, data, errors)
	require.Empty(t, errors)
	require.Len(t, data, 4)

	var chunks []types.ChatCompletionStreamResponse
	for len(data) > 0 {
		var chunk types.ChatCompletionStreamResponse
		require.NoError(t, json.Unmarshal([]byte(<-data), &chunk))
		require.Len(t, chunk.Choices, 1)
		chunks = append(chunks, chunk)
	}
	call := chunks[0].Choices[0].Delta.ToolCalls[0]
	encoded, err := json.Marshal(call)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.JSONEq(t, `{"google":{"thought_signature":"sse-signature"}}`, string(fields["extra_content"]))
	require.Equal(t, "get_info", call.Function.Name)
	require.Equal(t, "{}", chunks[1].Choices[0].Delta.ToolCalls[0].Function.Arguments)
	require.Equal(t, types.FinishReasonToolCalls, chunks[2].Choices[0].FinishReason)
	require.Equal(t, types.FinishReasonStop, chunks[3].Choices[0].FinishReason)
	require.Equal(t, 12, handler.Usage.TotalTokens)
}
