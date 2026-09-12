package mockserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/.github/smoke/mockserver"
	"one-api/providers/gemini"
	"one-api/types"
)

func TestMockRejectsUnknownRoutesAndKeys(t *testing.T) {
	for _, fixture := range []struct{ path, key string }{
		{"/unexpected", "fixture-gemini-key"},
		{"/v1beta/models/gemini-smoke:generateContent", "wrong-key"},
	} {
		request := httptest.NewRequest(http.MethodPost, fixture.path, strings.NewReader(`{}`))
		request.Header.Set("x-goog-api-key", fixture.key)
		response := httptest.NewRecorder()
		mockserver.New().ServeHTTP(response, request)
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
}

func TestMockRequiresBothOriginalSignaturesAndToolResults(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		var parts, results []map[string]any
		for i, name := range mockserver.Names {
			signature := mockserver.Signatures[i]
			if corrupt && i == 1 {
				signature = "corrupt"
			}
			parts = append(parts, map[string]any{"functionCall": map[string]any{"name": name, "args": map[string]any{"file": "中文测试.docx"}}, "thoughtSignature": signature})
			results = append(results, map[string]any{"functionResponse": map[string]any{"name": name, "response": map[string]any{"content": `{"ok":true}`}}})
		}
		data, err := json.Marshal(map[string]any{"contents": []map[string]any{{"role": "model", "parts": parts}, {"role": "user", "parts": results}}})
		require.NoError(t, err)
		for _, mode := range []string{"generateContent", "streamGenerateContent"} {
			request := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-smoke:"+mode, bytes.NewReader(data))
			request.Header.Set("x-goog-api-key", "fixture-gemini-key")
			response := httptest.NewRecorder()
			mockserver.New().ServeHTTP(response, request)
			if corrupt {
				require.Equal(t, http.StatusBadRequest, response.Code)
				require.Contains(t, response.Body.String(), "tool signature")
			} else {
				require.Equal(t, http.StatusOK, response.Code)
				require.Contains(t, response.Body.String(), "工具签名往返成功")
			}
		}
	}
}

func TestMockRejectsInvalidGeminiToolRoles(t *testing.T) {
	for _, roles := range [][2]string{{"model", "function"}, {"model", "model"}, {"user", "user"}, {"assistant", "user"}, {"model", ""}} {
		for _, mode := range []string{"generateContent", "streamGenerateContent"} {
			t.Run(roles[0]+"/"+roles[1]+"/"+mode, func(t *testing.T) {
				var calls, results []map[string]any
				for i, name := range mockserver.Names {
					calls = append(calls, map[string]any{"functionCall": map[string]any{"name": name, "args": map[string]any{"file": "中文测试.docx"}}, "thoughtSignature": mockserver.Signatures[i]})
					results = append(results, map[string]any{"functionResponse": map[string]any{"name": name, "response": map[string]any{"content": `{"ok":true}`}}})
				}
				data, err := json.Marshal(map[string]any{"contents": []map[string]any{{"role": roles[0], "parts": calls}, {"role": roles[1], "parts": results}}})
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-smoke:"+mode, bytes.NewReader(data))
				request.Header.Set("x-goog-api-key", "fixture-gemini-key")
				response := httptest.NewRecorder()
				mockserver.New().ServeHTTP(response, request)
				require.Equal(t, http.StatusBadRequest, response.Code)
				require.Contains(t, response.Body.String(), "role")
			})
		}
	}
}

// Exercise the real converter against the wire-level mock contract. A permissive
// mock must not hide a regression that changes functionResponse back to function.
func TestConvertedToolResultsMeetMockContract(t *testing.T) {
	candidate := gemini.GeminiChatCandidate{Content: gemini.GeminiChatContent{Role: "model"}}
	for i, name := range mockserver.Names {
		signature, err := json.Marshal(mockserver.Signatures[i])
		require.NoError(t, err)
		candidate.Content.Parts = append(candidate.Content.Parts, gemini.GeminiPart{
			FunctionCall: &gemini.GeminiFunctionCall{
				Name: name, Args: map[string]any{"file": "中文测试.docx"},
			},
			ThoughtSignature: signature,
		})
	}
	choice := candidate.ToOpenAIChoice(&types.ChatCompletionRequest{Model: "gemini-smoke"})
	messages := []types.ChatCompletionMessage{choice.Message}
	for _, call := range choice.Message.ToolCalls {
		messages = append(messages, types.ChatCompletionMessage{
			Role: types.ChatMessageRoleTool, ToolCallID: call.Id, Content: `{"ok":true}`,
		})
	}
	contents, _, apiErr := gemini.OpenAIToGeminiChatContent(messages)
	require.Nil(t, apiErr)
	require.Len(t, contents, 2)
	for _, mode := range []string{"generateContent", "streamGenerateContent"} {
		t.Run(mode, func(t *testing.T) {
			for _, corruptRole := range []bool{false, true} {
				wireContents := append([]gemini.GeminiChatContent(nil), contents...)
				if corruptRole {
					wireContents[1].Role = "function"
				}
				body, err := json.Marshal(gemini.GeminiChatRequest{Contents: wireContents})
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-smoke:"+mode, bytes.NewReader(body))
				request.Header.Set("x-goog-api-key", "fixture-gemini-key")
				response := httptest.NewRecorder()
				mockserver.New().ServeHTTP(response, request)
				if corruptRole {
					require.Equal(t, http.StatusBadRequest, response.Code)
					require.Contains(t, response.Body.String(), "role")
				} else {
					require.Equal(t, http.StatusOK, response.Code)
					require.Contains(t, response.Body.String(), "工具签名往返成功")
					if mode == "streamGenerateContent" {
						require.Equal(t, "text/event-stream", response.Header().Get("Content-Type"))
						require.True(t, strings.HasPrefix(response.Body.String(), "data: "))
					}
				}
			}
		})
	}
}
