package mockserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeFixtureRoundtripAndCorruption(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, corrupt := range []bool{false, true} {
			blocks := claudeBlocks()
			if corrupt {
				blocks[0]["signature"] = "corrupted"
			}
			body, err := json.Marshal(object{"model": "claude-smoke", "stream": stream, "messages": []object{
				{"role": "user", "content": "synthetic"},
				{"role": "assistant", "content": blocks},
				{"role": "user", "content": []object{
					{"type": "tool_result", "tool_use_id": "fixture-tool-0", "content": `{"ok":true}`},
					{"type": "tool_result", "tool_use_id": "fixture-tool-1", "content": `{"ok":true}`},
				}},
			}})
			require.NoError(t, err)
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			r.Header.Set("x-api-key", "fixture-claude-key")
			w := httptest.NewRecorder()
			New().ServeHTTP(w, r)
			if corrupt {
				require.Equal(t, 400, w.Code)
				require.Contains(t, w.Body.String(), "signed blocks changed")
			} else {
				require.Equal(t, 200, w.Code)
				require.Contains(t, w.Body.String(), "Claude工具往返成功")
				if stream {
					require.Contains(t, w.Body.String(), `"message_stop"`)
				}
			}
		}
	}
}
