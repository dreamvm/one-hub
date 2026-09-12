package gemini_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/gemini"
	"one-api/types"
)

func geminiRequest(t *testing.T, extra string) *types.ChatCompletionRequest {
	t.Helper()
	var r types.ChatCompletionRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"fixture","messages":[{"role":"user","content":"test"}],`+extra+`}`), &r))
	return &r
}

func TestGeminiToolSchemaAndControls(t *testing.T) {
	for _, mode := range []string{"none", "auto", "required"} {
		r := geminiRequest(t, `"tool_choice":"`+mode+`","tools":[{"type":"function","function":{"name":"write_file","parameters":{"type":"object","properties":{"examples":{"type":"string","examples":["中文"]},"meta":{"type":"object","additionalProperties":{"type":"string"}}},"additionalProperties":false}}}]`)
		before, _ := json.Marshal(r)
		g, e := gemini.ConvertFromChatOpenai(r)
		require.Nil(t, e)
		wire, err := json.Marshal(g)
		require.NoError(t, err)
		var body map[string]any
		require.NoError(t, json.Unmarshal(wire, &body))
		declaration := body["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0].(map[string]any)
		require.Contains(t, declaration, "parametersJsonSchema")
		require.NotContains(t, declaration, "parameters")
		require.Contains(t, string(wire), `"examples":["中文"]`)
		require.Contains(t, string(wire), `"additionalProperties":false`)
		require.Contains(t, body, "toolConfig")
		require.Equal(t, map[string]string{"none": "NONE", "auto": "AUTO", "required": "ANY"}[mode], body["toolConfig"].(map[string]any)["functionCallingConfig"].(map[string]any)["mode"])
		after, _ := json.Marshal(r)
		require.JSONEq(t, string(before), string(after), "conversion must not mutate input")
	}
}

func TestGeminiDoesNotLoseMixedTools(t *testing.T) {
	g, e := gemini.ConvertFromChatOpenai(geminiRequest(t, `"tools":[{"type":"function","function":{"name":"googleSearch"}},{"type":"function","function":{"name":"codeExecution"}},{"type":"function","function":{"name":"urlContext"}},{"type":"function","function":{"name":"write_file","parameters":{"type":"object","properties":{}}}}]`))
	require.Nil(t, e)
	wire, _ := json.Marshal(g.Tools)
	for _, value := range []string{"googleSearch", "codeExecution", "urlContext", "write_file"} {
		require.Contains(t, string(wire), value)
	}
}

func TestGeminiRejectsInvalidToolArguments(t *testing.T) {
	for _, args := range []string{`{broken`, `null`, `[]`, `"text"`} {
		messages := []types.ChatCompletionMessage{{Role: "assistant", ToolCalls: []*types.ChatCompletionToolCalls{{Id: "a", Function: &types.ChatCompletionToolCallsFunction{Name: "write_file", Arguments: args}}}}}
		_, _, e := gemini.OpenAIToGeminiChatContent(messages)
		require.NotNil(t, e, args)
		require.Equal(t, 400, e.StatusCode)
	}
}

func TestGeminiToolControlErrors(t *testing.T) {
	for _, fields := range []string{
		`"tool_choice":"unknown"`,
		`"tool_choice":{"type":"function","function":{"name":123}}`,
		`"tool_choice":{"type":"function","function":{"name":"missing"}}`,
		`"tool_choice":"required"`,
		`"parallel_tool_calls":false`,
		`"tools":[null]`,
		`"tools":[{"type":"function","function":{"name":"x","strict":true}}]`,
	} {
		t.Run(fields, func(t *testing.T) {
			require.NotPanics(t, func() {
				_, e := gemini.ConvertFromChatOpenai(geminiRequest(t, fields))
				require.NotNil(t, e)
				require.Equal(t, 400, e.StatusCode)
			})
		})
	}
	for _, choice := range []string{`"required"`, `{"type":"function","function":{"name":"x"}}`} {
		g, e := gemini.ConvertFromChatOpenai(geminiRequest(t, `"tool_choice":`+choice+`,"tools":[{"type":"function","function":{"name":"x","strict":true}}]`))
		require.Nil(t, e)
		b, _ := json.Marshal(g)
		require.NotContains(t, string(b), `"strict"`)
		require.Equal(t, "ANY", g.ToolConfig.FunctionCallingConfig.Mode)
	}
}
