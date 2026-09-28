package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"one-api/common/logger"
	"one-api/middleware"
)

func TestRequestLogsExcludeURLCredentials(t *testing.T) {
	for _, test := range []struct {
		name, method, route, target string
		requestError                bool
	}{
		{"mcp", "POST", "/mcp/:accessToken", "/mcp/credential-fixture", false},
		{"mcp_sse", "GET", "/mcp/sse/:accessToken", "/mcp/sse/credential-fixture", false},
		{"mcp_message", "POST", "/mcp/message/:accessToken", "/mcp/message/credential-fixture", false},
		{"telegram", "POST", "/api/telegram/:token", "/api/telegram/credential-fixture", false},
		{"gemini", "POST", "/v1beta/models/:model", "/v1beta/models/test?key=credential-fixture", false},
		{"repeated", "POST", "/v1beta/models/:model", "/v1beta/models/test?key=sk-credential-fixture&key=credential-fixture", false},
		{"encoded_key", "POST", "/v1beta/models/:model", "/v1beta/models/test?%6b%65%79=credential-fixture", false},
		{"encoded_value", "POST", "/v1beta/models/:model", "/v1beta/models/test?key=%63redential-fixture", false},
		{"malformed_query", "POST", "/v1beta/models/:model", "/v1beta/models/test?key=%ZZcredential-fixture", false},
		{"semicolon_query", "POST", "/v1beta/models/:model", "/v1beta/models/test?key=credential-fixture;other=value", false},
		{"oauth", "GET", "/oauth/github", "/oauth/github?code=credential-fixture&state=credential-fixture", false},
		{"payment_signature", "GET", "/api/payment/notify/:id", "/api/payment/notify/test?sign=credential-fixture&amount=1.00", false},
		{"unknown", "GET", "", "/mcp/credential-fixture/extra", false},
		{"wrong_method", "PUT", "/mcp/:accessToken", "/mcp/credential-fixture", false},
		{"error_message", "GET", "/failure", "/failure?key=credential-fixture", true},
		{"ordinary", "GET", "/api/user/", "/api/user/?page=2&size=10", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			core, observed := observer.New(zapcore.InfoLevel)
			previous := logger.Logger
			previousDebug := gin.DebugPrintFunc
			logger.Logger = zap.New(core)
			t.Cleanup(func() { logger.Logger = previous; gin.DebugPrintFunc = previousDebug })
			r := gin.New()
			middleware.SetUpLogger(r)
			seen := ""
			handler := func(c *gin.Context) {
				seen = c.Request.URL.RequestURI()
				c.Set(logger.RequestIdKey, "request-fixture")
				if test.requestError {
					_ = c.Error(errors.New("upstream failed for " + c.Request.URL.String()))
				}
				c.Status(http.StatusNoContent)
			}
			if test.route != "" {
				method := test.method
				if test.name == "wrong_method" {
					method = http.MethodPost
				}
				r.Handle(method, test.route, handler)
			}
			r.NoRoute(handler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(test.method, test.target, nil))
			require.Equal(t, http.StatusNoContent, w.Code)
			require.Equal(t, test.target, seen, "logging must not rewrite signed query or routed request")
			entries := observed.All()
			require.Len(t, entries, 1)
			encoded, err := json.Marshal(entries)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "credential-fixture")
			require.NotContains(t, string(encoded), "%63redential-fixture")
			fields := entries[0].ContextMap()
			path := test.route
			if path == "" || test.name == "wrong_method" {
				path = "[unmatched]"
			}
			require.Equal(t, path, fields["path"])
			require.Equal(t, "request-fixture", fields["request_id"])
			require.Equal(t, int64(http.StatusNoContent), fields["status"])
			require.Equal(t, test.method, fields["method"])
			require.Contains(t, fields, "latency")
		})
	}
}

func TestRequestRecoveryDoesNotDumpCredentials(t *testing.T) {
	core, observed := observer.New(zapcore.InfoLevel)
	previous := logger.Logger
	previousDebug := gin.DebugPrintFunc
	logger.Logger = zap.New(core)
	t.Cleanup(func() { logger.Logger = previous; gin.DebugPrintFunc = previousDebug })
	var stderr bytes.Buffer
	previousWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &stderr
	t.Cleanup(func() { gin.DefaultErrorWriter = previousWriter })
	r := gin.New()
	middleware.SetUpLogger(r)
	r.GET("/mcp/:accessToken", func(c *gin.Context) { panic("panic with credential-fixture") })
	req := httptest.NewRequest(http.MethodGet, "/mcp/credential-fixture?key=credential-fixture", nil)
	req.Header.Set("Cookie", "session=credential-fixture")
	req.Header.Set("Authorization", "Bearer credential-fixture")
	w := httptest.NewRecorder()
	require.NotPanics(t, func() { r.ServeHTTP(w, req) })
	require.Equal(t, http.StatusInternalServerError, w.Code)
	encoded, err := json.Marshal(observed.All())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "credential-fixture")
	require.NotContains(t, stderr.String(), "credential-fixture")
	require.NotEmpty(t, observed.All(), "safe failure diagnostics must remain")
}

func TestDebugRedirectDoesNotLogCredentials(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	oldLogger, oldDebug, oldWriter, oldMode := logger.Logger, gin.DebugPrintFunc, gin.DefaultWriter, gin.Mode()
	logger.Logger = zap.New(core)
	var output bytes.Buffer
	gin.DefaultWriter = &output
	gin.SetMode(gin.DebugMode)
	t.Cleanup(func() {
		logger.Logger = oldLogger
		gin.DebugPrintFunc = oldDebug
		gin.DefaultWriter = oldWriter
		gin.SetMode(oldMode)
	})
	r := gin.New()
	middleware.SetUpLogger(r)
	r.POST("/api/telegram/:token", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/telegram/credential-fixture/?key=credential-fixture", nil))
	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	require.Equal(t, "/api/telegram/credential-fixture?key=credential-fixture", w.Header().Get("Location"))
	encoded, err := json.Marshal(observed.All())
	require.NoError(t, err)
	require.NotContains(t, output.String()+string(encoded), "credential-fixture")
}

func TestBrokenConnectionKeepsSafePanicDiagnostics(t *testing.T) {
	core, observed := observer.New(zapcore.InfoLevel)
	oldLogger, oldDebug := logger.Logger, gin.DebugPrintFunc
	logger.Logger = zap.New(core)
	t.Cleanup(func() { logger.Logger = oldLogger; gin.DebugPrintFunc = oldDebug })
	r := gin.New()
	middleware.SetUpLogger(r)
	r.GET("/mcp/:accessToken", func(c *gin.Context) {
		panic(&net.OpError{Op: "write credential-fixture", Net: "tcp", Err: &os.SyscallError{Syscall: "write", Err: syscall.EPIPE}})
	})
	w := httptest.NewRecorder()
	require.NotPanics(t, func() { r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/mcp/credential-fixture", nil)) })
	require.Empty(t, w.Body.String(), "do not write a response to a broken connection")
	require.NotEmpty(t, observed.All(), "broken connections must not silently lose recovery diagnostics")
	encoded, err := json.Marshal(observed.All())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "credential-fixture")
}

func TestProtocolRecoveryDoesNotLogCredentials(t *testing.T) {
	for _, test := range []struct {
		name     string
		recovery gin.HandlerFunc
	}{
		{"openai", middleware.RelayPanicRecover()}, {"claude", middleware.RelayCluadePanicRecover()},
		{"gemini", middleware.RelayGeminiPanicRecover()}, {"midjourney", middleware.RelayMJPanicRecover()},
		{"suno", middleware.RelaySunoPanicRecover()}, {"kling", middleware.RelayKlingPanicRecover()},
	} {
		t.Run(test.name, func(t *testing.T) {
			core, observed := observer.New(zapcore.InfoLevel)
			oldLogger, oldDebug := logger.Logger, gin.DebugPrintFunc
			logger.Logger = zap.New(core)
			t.Cleanup(func() { logger.Logger = oldLogger; gin.DebugPrintFunc = oldDebug })
			r := gin.New()
			middleware.SetUpLogger(r)
			r.GET("/relay", test.recovery, func(c *gin.Context) { panic("credential-fixture") })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/relay?key=credential-fixture", nil))
			require.Equal(t, http.StatusInternalServerError, w.Code)
			require.Contains(t, w.Body.String(), "one_hub_panic", "protocol error contract must remain")
			encoded, err := json.Marshal(observed.All())
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "credential-fixture")
		})
	}
}
