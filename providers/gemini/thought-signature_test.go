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
				require.Equal(t, "user", contents[1].Role, "Gemini functionResponse uses a user turn")
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

func TestGeminiToolResultTurns(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		messages   string
		roles      []string
		partCounts []int
	}{
		{
			name: "legacy_named_function",
			messages: `[{"role":"assistant","function_call":{"name":"read_file","arguments":"{}"}},
				{"role":"function","name":"read_file","content":"中文结果"}]`,
			roles: []string{"model", "user"}, partCounts: []int{1, 1},
		},
		{
			name: "ordinary_user_turns_stay_separate",
			messages: `[{"role":"user","content":"保留前一条用户消息"},
				{"role":"function","name":"read_file","content":"中文结果"},
				{"role":"user","content":"保留后一条用户消息"},
				{"role":"function","name":"list_files","content":"第二个结果"}]`,
			roles: []string{"user", "user", "user", "user"}, partCounts: []int{1, 1, 1, 1},
		},
		{
			name: "consecutive_results_merge_but_new_calls_start_new_turns",
			messages: `[{"role":"assistant","tool_calls":[
				{"id":"a","type":"function","function":{"name":"read_file","arguments":"{}"}},
				{"id":"b","type":"function","function":{"name":"list_files","arguments":"{}"}}]},
				{"role":"tool","tool_call_id":"a","content":"第一个结果"},
				{"role":"tool","tool_call_id":"b","content":"第二个结果"},
				{"role":"assistant","tool_calls":[{"id":"c","type":"function","function":{"name":"read_file","arguments":"{}"}}]},
				{"role":"tool","tool_call_id":"c","content":"下一轮结果"}]`,
			roles: []string{"model", "user", "model", "user"}, partCounts: []int{2, 2, 1, 1},
		},
		{
			name:     "explicit_name_stays_compatible",
			messages: `[{"role":"tool","name":"read_file","content":"具名结果"}]`,
			roles:    []string{"user"}, partCounts: []int{1},
		},
		{
			name: "empty_name_resolves_from_tool_call_id",
			messages: `[{"role":"assistant","tool_calls":[{"id":"a","type":"function","function":{"name":"read_file","arguments":"{}"}}]},
				{"role":"tool","name":" ","tool_call_id":"a","content":"按 ID 匹配结果"}]`,
			roles: []string{"model", "user"}, partCounts: []int{1, 1},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var messages []types.ChatCompletionMessage
			require.NoError(t, json.Unmarshal([]byte(fixture.messages), &messages))
			contents, _, apiErr := gemini.OpenAIToGeminiChatContent(messages)
			require.Nil(t, apiErr)
			require.Len(t, contents, len(fixture.roles))
			var resultTexts []string
			for i, content := range contents {
				require.Equal(t, fixture.roles[i], content.Role)
				require.Len(t, content.Parts, fixture.partCounts[i])
				for _, part := range content.Parts {
					if part.FunctionResponse != nil {
						require.Equal(t, "user", content.Role)
						require.NotEmpty(t, part.FunctionResponse.Name)
						response, ok := part.FunctionResponse.Response.(gemini.GeminiFunctionResponseContent)
						require.True(t, ok)
						require.Equal(t, part.FunctionResponse.Name, response.Name)
						resultTexts = append(resultTexts, response.Content)
					} else if content.Role == "user" {
						require.Len(t, content.Parts, 1, "do not append results to an ordinary user message")
					}
				}
			}
			var expectedTexts []string
			for _, message := range messages {
				if message.Role == types.ChatMessageRoleTool || message.Role == types.ChatMessageRoleFunction {
					expectedTexts = append(expectedTexts, message.StringContent())
				}
			}
			require.Equal(t, expectedTexts, resultTexts, "keep result order and contents")
		})
	}
}

func TestGeminiRejectsUnresolvableToolResults(t *testing.T) {
	for _, input := range []string{
		`[{"role":"tool","tool_call_id":"unknown","content":"result"}]`,
		`[{"role":"function","content":"result"}]`,
		`[{"role":"tool","name":"","content":"result"}]`,
		`[{"role":"function","name":"  ","content":"result"}]`,
	} {
		t.Run(input, func(t *testing.T) {
			var messages []types.ChatCompletionMessage
			require.NoError(t, json.Unmarshal([]byte(input), &messages))
			require.NotPanics(t, func() {
				contents, _, apiErr := gemini.OpenAIToGeminiChatContent(messages)
				require.Empty(t, contents)
				require.NotNil(t, apiErr)
				require.Equal(t, 400, apiErr.StatusCode)
			})
		})
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
	require.Len(t, data, 3)

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
	require.Equal(t, 12, handler.Usage.TotalTokens)
}
