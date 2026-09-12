package types_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/types"
)

func TestToolCallExtraContentJSONRoundTrip(t *testing.T) {
	for _, extra := range []string{
		`{"google":{"thought_signature":"opaque-test-signature+/="}}`,
		`{"google":{"thought_signature":"opaque-test-signature"},"vendor":{"nested":[1,true,"中文"]}}`,
		`{"vendor":{"opaque":"metadata"}}`,
		`{}`,
	} {
		t.Run(extra, func(t *testing.T) {
			input := `{"id":"call_test","type":"function","index":0,"function":{"name":"read_file","arguments":"{}"},"extra_content":` + extra + `}`
			var call types.ChatCompletionToolCalls
			require.NoError(t, json.Unmarshal([]byte(input), &call))
			encoded, err := json.Marshal(call)
			require.NoError(t, err)
			require.JSONEq(t, input, string(encoded))
		})
	}
}

func TestToolCallWithoutExtraContentUnchanged(t *testing.T) {
	input := `{"id":"call_plain","type":"function","index":0,"function":{"name":"read_file","arguments":"{}"}}`
	var call types.ChatCompletionToolCalls
	require.NoError(t, json.Unmarshal([]byte(input), &call))
	encoded, err := json.Marshal(call)
	require.NoError(t, err)
	require.JSONEq(t, input, string(encoded))
}

func TestToolCallStreamSplitKeepsEachToolMetadata(t *testing.T) {
	input := `[
		{"id":"call_first","type":"function","index":0,"function":{"name":"read_file","arguments":"{\"path\":\"中文.txt\"}"},"extra_content":{"google":{"thought_signature":"signature-first"}}},
		{"id":"call_second","type":"function","index":1,"function":{"name":"list_files","arguments":"{}"},"extra_content":{"google":{"thought_signature":"signature-second"},"vendor":{"flag":true}}},
		{"id":"call_unsigned","type":"function","index":2,"function":{"name":"get_info","arguments":"{}"}}
	]`
	var calls []*types.ChatCompletionToolCalls
	require.NoError(t, json.Unmarshal([]byte(input), &calls))
	choice := types.ChatCompletionStreamChoice{
		Index: 2,
		Delta: types.ChatCompletionStreamChoiceDelta{
			Role: types.ChatMessageRoleAssistant, ToolCalls: calls,
		},
	}
	chunks := choice.ConvertOpenaiStream()
	require.Len(t, chunks, 2*len(calls)+1)
	for i, original := range calls {
		header, body := chunks[2*i], chunks[2*i+1]
		require.Equal(t, choice.Index, header.Index)
		require.Len(t, header.Delta.ToolCalls, 1)
		require.Len(t, body.Delta.ToolCalls, 1)
		headCall, bodyCall := header.Delta.ToolCalls[0], body.Delta.ToolCalls[0]
		require.Equal(t, i, headCall.Index)
		require.Equal(t, i, bodyCall.Index)
		require.Equal(t, original.Id, headCall.Id)
		require.Empty(t, bodyCall.Id)
		require.Equal(t, original.Function.Name, headCall.Function.Name)
		require.Empty(t, headCall.Function.Arguments)
		require.Empty(t, bodyCall.Function.Name)
		require.Equal(t, original.Function.Arguments, bodyCall.Function.Arguments)

		var expected []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(input), &expected))
		for _, item := range []struct {
			call *types.ChatCompletionToolCalls
			want json.RawMessage
		}{{headCall, expected[i]["extra_content"]}, {bodyCall, nil}} {
			encoded, err := json.Marshal(item.call)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &fields))
			if item.want == nil {
				require.NotContains(t, fields, "extra_content")
			} else {
				require.JSONEq(t, string(item.want), string(fields["extra_content"]))
			}
		}
	}
	require.Equal(t, types.FinishReasonToolCalls, chunks[len(chunks)-1].FinishReason)
}

func TestLegacyFunctionStreamSplitUnchanged(t *testing.T) {
	choice := types.ChatCompletionStreamChoice{
		Delta: types.ChatCompletionStreamChoiceDelta{
			FunctionCall: &types.ChatCompletionToolCallsFunction{Name: "get_info", Arguments: "{}"},
		},
	}
	chunks := choice.ConvertOpenaiStream()
	require.Len(t, chunks, 3)
	require.Equal(t, "get_info", chunks[0].Delta.FunctionCall.Name)
	require.Equal(t, "{}", chunks[1].Delta.FunctionCall.Arguments)
	require.Empty(t, chunks[0].Delta.ToolCalls)
	require.Equal(t, types.FinishReasonFunctionCall, chunks[2].FinishReason)
}
