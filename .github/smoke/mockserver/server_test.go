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
		data, err := json.Marshal(map[string]any{"contents": []map[string]any{{"role": "model", "parts": parts}, {"role": "function", "parts": results}}})
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
