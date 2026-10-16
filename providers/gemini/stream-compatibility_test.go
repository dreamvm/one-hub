package gemini_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/gemini"
	"one-api/types"
)

func streamCompatProvider(t *testing.T, server *httptest.Server, ctx context.Context, builtin bool) *gemini.GeminiProvider {
	t.Helper()
	oldClient, oldLogger := requester.HTTPClient, logger.Logger
	requester.HTTPClient, logger.Logger = server.Client(), zap.NewNop()
	t.Cleanup(func() { requester.HTTPClient, logger.Logger = oldClient, oldLogger })
	proxy, url := "", server.URL
	channel := &model.Channel{Id: 1, BaseURL: &url, Proxy: &proxy, Key: "synthetic-fixture"}
	if builtin {
		plugin := datatypes.NewJSONType(model.PluginType{"code_execution": {"enable": true}})
		channel.Plugin = &plugin
	}
	p := (&gemini.GeminiProviderFactory{}).Create(channel).(*gemini.GeminiProvider)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	p.SetContext(c)
	// Match the estimate installed by RelayHandler, which a reported snapshot replaces.
	p.SetUsage(&types.Usage{PromptTokens: 3})
	return p
}

func streamCompatRequest() *types.ChatCompletionRequest {
	high := "high"
	return &types.ChatCompletionRequest{Model: "gemini-fixture", Stream: true, MaxTokens: 10000,
		ReasoningEffort: &high, Messages: []types.ChatCompletionMessage{{Role: "user", Content: "synthetic input"}}}
}

func streamCompatDrain(stream requester.StreamReaderInterface[string]) (string, error) {
	defer stream.Close()
	data, errs := stream.Recv()
	var output strings.Builder
	for {
		select {
		case chunk := <-data:
			output.WriteString(chunk)
		case err := <-errs:
			if err == io.EOF {
				return output.String(), nil
			}
			return output.String(), err
		}
	}
}

func TestGeminiStreamCompatibility(t *testing.T) {
	empty := "data: " + emptyResponseEvent(`[]`, "STOP") + "\n\n"
	malformed := "data: " + emptyResponseEvent(`[]`, "MALFORMED_FUNCTION_CALL") + "\n\n"
	normal := "data: " + emptyResponseEvent(`[{"text":"normal-ok"}]`, "STOP") + "\n\n"
	for _, tc := range []struct {
		name, selected, first, second, code string
		builtin, functions, streamError     bool
		heartbeat                           bool
		n, calls                            int
	}{
		{name: "builtin-empty", selected: "1", first: empty, builtin: true, code: "empty_response", calls: 1},
		{name: "builtin-malformed", selected: "1", first: malformed, builtin: true, code: "malformed_function_call", calls: 1},
		{name: "retry-empty", selected: "1", first: empty, second: normal, calls: 2},
		{name: "retry-malformed", selected: "1", first: malformed, second: normal, calls: 2},
		{name: "functions-suppress-builtin", selected: "1", first: empty, second: normal, builtin: true, functions: true, calls: 2},
		{name: "two-empties", selected: "1", first: empty, second: empty, code: "empty_response", calls: 2},
		{name: "heartbeat-no-replay", selected: "1", first: empty, heartbeat: true, code: "empty_response", calls: 1},
		{name: "multi-candidate-request", selected: "1", first: empty, n: 2, code: "empty_response", calls: 1},
		{name: "unselected-empty", selected: "2", first: empty, streamError: true, calls: 1},
		{name: "normal", selected: "1", first: normal, calls: 1},
		{name: "builtin-normal", selected: "1", first: normal, builtin: true, calls: 1},
		{name: "unselected-normal", first: normal, calls: 1},
		{name: "invalid-json", selected: "1", first: "data: {invalid\n\n", streamError: true, calls: 1},
		{name: "truncated", selected: "1", first: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}]}\n\n", streamError: true, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS", tc.selected)
			var calls atomic.Int32
			var bodies [][]byte
			var mu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				mu.Lock()
				bodies = append(bodies, body)
				mu.Unlock()
				w.Header().Set("Content-Type", "text/event-stream")
				if calls.Add(1) == 1 {
					io.WriteString(w, tc.first)
				} else {
					io.WriteString(w, tc.second)
				}
			}))
			defer server.Close()
			p := streamCompatProvider(t, server, context.Background(), tc.builtin)
			if tc.heartbeat {
				p.Context.Set("token_setting", &model.TokenSetting{Heartbeat: model.HeartbeatSetting{Enabled: true}})
			}
			req := streamCompatRequest()
			if tc.n > 0 {
				req.N = &tc.n
			}
			if tc.functions {
				req.Functions = []*types.ChatCompletionFunction{{Name: "fixture_function"}}
			}
			stream, apiErr := p.CreateChatCompletionStream(req)
			if tc.code != "" {
				if stream != nil {
					stream.Close()
				}
				require.NotNil(t, apiErr)
				require.Equal(t, tc.code, apiErr.Code)
				require.Equal(t, 502, apiErr.StatusCode)
				require.True(t, apiErr.LocalError, "must not escape the two-attempt bound via generic relay retries")
			} else {
				require.Nil(t, apiErr)
				output, err := streamCompatDrain(stream)
				if tc.streamError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Contains(t, output, "normal-ok")
					require.Equal(t, 10*tc.calls, p.Usage.PromptTokens)
					require.Equal(t, 8141*tc.calls, p.Usage.CompletionTokens)
					require.Equal(t, 8151*tc.calls, p.Usage.TotalTokens)
				}
			}
			require.EqualValues(t, tc.calls, calls.Load())
			mu.Lock()
			defer mu.Unlock()
			for _, body := range bodies {
				require.True(t, bytes.Equal(bodies[0], body), "retry body must be byte-identical")
				require.Contains(t, string(body), `"thinkingLevel":"HIGH"`)
				require.Contains(t, string(body), `"maxOutputTokens":10000`)
				if tc.functions {
					require.NotContains(t, string(body), "codeExecution")
				}
			}
		})
	}
}

func TestGeminiStreamCompatibilityClientCancel(t *testing.T) {
	t.Setenv("GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS", "1")
	started, upstreamDone := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": heartbeat\n\n")
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(upstreamDone)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := streamCompatProvider(t, server, ctx, false)
	done := make(chan struct{})
	go func() {
		defer close(done)
		stream, _ := p.CreateChatCompletionStream(streamCompatRequest())
		if stream != nil {
			streamCompatDrain(stream)
		}
	}()
	<-started
	cancel()
	select {
	case <-upstreamDone:
	case <-time.After(time.Second):
		server.CloseClientConnections()
		<-done
		t.Fatal("client cancellation did not reach the upstream")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("provider did not return after cancellation")
	}
}
