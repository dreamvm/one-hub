package mockserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"one-api/.github/smoke/mockserver"
)

func gateControl(server *mockserver.Server, action, key string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/fixture/response-gate", strings.NewReader(`{"action":"`+action+`"}`))
	r.Header.Set("Authorization", "Bearer "+key)
	server.ServeHTTP(w, r)
	return w
}

func gateRequest(stream bool) *http.Request {
	body := `{"model":"openai-smoke","stream":false}`
	if stream {
		body = `{"model":"openai-smoke","stream":true}`
	}
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer fixture-openai-key")
	return r
}

func TestResponseGateBoundsAndNormalControls(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, action := range []string{"release", "cancel"} {
			t.Run(action+map[bool]string{false: "/json", true: "/sse"}[stream], func(t *testing.T) {
				server := mockserver.New()
				require.Equal(t, 400, gateControl(server, "arm", "wrong-key").Code)
				require.Equal(t, 400, gateControl(server, "unknown", "fixture-gate-control").Code)
				require.Equal(t, 200, gateControl(server, "arm", "fixture-gate-control").Code)
				require.Equal(t, 409, gateControl(server, "arm", "fixture-gate-control").Code)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				w, done := httptest.NewRecorder(), make(chan struct{})
				go func() { defer close(done); server.ServeHTTP(w, gateRequest(stream).WithContext(ctx)) }()
				require.Eventually(t, func() bool {
					return strings.Contains(gateControl(server, "state", "fixture-gate-control").Body.String(), `"entered":true`)
				}, time.Second, time.Millisecond)
				select {
				case <-done:
					t.Fatal("response escaped the armed gate")
				default:
				}
				second := httptest.NewRecorder()
				server.ServeHTTP(second, gateRequest(false))
				require.Equal(t, 400, second.Code, "no queue behind the single held request")
				if action == "release" {
					require.Equal(t, 200, gateControl(server, "release", "fixture-gate-control").Code)
				} else {
					cancel()
				}
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("gate did not release resources")
				}
				if action == "release" {
					require.Equal(t, 200, w.Code)
					require.Contains(t, w.Body.String(), "中文对话成功")
					require.Contains(t, w.Body.String(), `"total_tokens":14`)
				} else {
					if stream {
						require.Equal(t, 200, w.Code)
						require.Contains(t, w.Body.String(), `"error"`)
					} else {
						require.Equal(t, 408, w.Code)
					}
				}
				var state map[string]bool
				require.NoError(t, json.Unmarshal(gateControl(server, "state", "fixture-gate-control").Body.Bytes(), &state))
				require.False(t, state["armed"])
				normal := httptest.NewRecorder()
				server.ServeHTTP(normal, gateRequest(stream))
				require.Equal(t, 200, normal.Code)
				require.Contains(t, normal.Body.String(), "中文对话成功")
			})
		}
	}
}

func TestResponseGateTimeout(t *testing.T) {
	server := mockserver.New()
	require.Equal(t, 200, gateControl(server, "arm", "fixture-gate-control").Code)
	start := time.Now()
	w := httptest.NewRecorder()
	server.ServeHTTP(w, gateRequest(false))
	require.Equal(t, 504, w.Code)
	require.Less(t, time.Since(start), 10*time.Second)
	require.Contains(t, gateControl(server, "state", "fixture-gate-control").Body.String(), `"armed":false`)
}
