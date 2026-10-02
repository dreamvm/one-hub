package router_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"
	"one-api/router"
)

func releaseRouter(t *testing.T) *gin.Engine {
	t.Helper()
	oldMode, oldLogger := gin.Mode(), logger.Logger
	gin.SetMode(gin.TestMode)
	logger.Logger = zap.NewNop()
	t.Cleanup(func() {
		gin.SetMode(oldMode)
		logger.Logger = oldLogger
	})
	r := gin.New()
	router.SetRelayRouter(r)
	return r
}

func TestRealtimeReleaseGate(t *testing.T) {
	r := releaseRouter(t)
	// No database, Redis or upstream is initialized: the release rejection must
	// precede authentication as well as the WebSocket and billing handlers.
	for _, tc := range []struct {
		name, path, authorization, protocols string
	}{
		{name: "no model or credentials", path: "/v1/realtime"},
		{name: "model query", path: "/v1/realtime?model=fixture-realtime"},
		{name: "invalid bearer", path: "/v1/realtime", authorization: "Bearer invalid"},
		{name: "bearer requiring lookup", path: "/v1/realtime?model=fixture-realtime", authorization: "Bearer sk-" + strings.Repeat("f", 48)},
		{name: "subprotocol requiring lookup", path: "/v1/realtime", protocols: "realtime, openai-insecure-api-key." + strings.Repeat("f", 48)},
		{name: "encoded path", path: "/v1/%72ealtime?model=fixture-realtime"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Sec-WebSocket-Version", "13")
			nonce := make([]byte, 16)
			_, err := rand.Read(nonce)
			require.NoError(t, err)
			req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(nonce))
			req.Header.Set("Authorization", tc.authorization)
			req.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusNotImplemented, w.Code)
			require.Empty(t, w.Header().Get("Upgrade"))
			require.Empty(t, w.Header().Get("Sec-WebSocket-Accept"))
			var body struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, "realtime_disabled_for_release", body.Error.Message)
			require.Equal(t, "one_hub_error", body.Error.Type)
		})
	}
	t.Run("trailing slash redirects to rejected endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/realtime/", nil))
		require.Equal(t, http.StatusMovedPermanently, w.Code)
		require.Equal(t, "/v1/realtime", w.Header().Get("Location"))
		follow := httptest.NewRecorder()
		r.ServeHTTP(follow, httptest.NewRequest(http.MethodGet, w.Header().Get("Location"), nil))
		require.Equal(t, http.StatusNotImplemented, follow.Code)
	})
}

func TestRealtimeReleaseGatePreservesOrdinaryAuthentication(t *testing.T) {
	r := releaseRouter(t)
	for _, stream := range []string{"false", "true"} {
		t.Run("chat stream="+stream, func(t *testing.T) {
			body := `{"model":"fixture-chat","stream":` + stream + `,"messages":[{"role":"user","content":"hello"}]}`
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.NotContains(t, w.Body.String(), "realtime_disabled_for_release")
		})
	}
}

func TestRealtimeReleaseGateDoesNotTouchExhaustedToken(t *testing.T) {
	r := releaseRouter(t)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "release.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB, oldRedis, oldCache := model.DB, config.RedisEnabled, config.MemoryCacheEnabled
	model.DB, config.RedisEnabled, config.MemoryCacheEnabled = db, false, false
	t.Cleanup(func() {
		model.DB, config.RedisEnabled, config.MemoryCacheEnabled = oldDB, oldRedis, oldCache
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))
	user := model.User{Id: 1, Username: "release-fixture", Status: config.UserStatusEnabled, Quota: 1000}
	token := model.Token{Id: 1, UserId: user.Id, Key: strings.Repeat("f", 48), Status: config.TokenStatusEnabled, ExpiredTime: -1}
	require.NoError(t, db.Create(&user).Error)
	// A legacy-format token fixture does not need the current signing service.
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&token).Error)
	queries, updates := 0, 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("release_query", func(*gorm.DB) { queries++ }))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("release_update", func(*gorm.DB) { updates++ }))
	for _, subprotocol := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=fixture-realtime", nil)
		req.Header.Set("Upgrade", "websocket")
		if subprotocol {
			req.Header.Set("Sec-WebSocket-Protocol", "realtime, openai-insecure-api-key."+token.Key)
		} else {
			req.Header.Set("Authorization", "Bearer "+token.Key)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotImplemented, w.Code)
	}
	require.Zero(t, queries, "release rejection must precede token lookup")
	require.Zero(t, updates, "exhausted-token status must not be changed")
	var after model.Token
	require.NoError(t, db.First(&after, token.Id).Error)
	require.Equal(t, config.TokenStatusEnabled, after.Status)
	require.Zero(t, after.RemainQuota)
	require.Zero(t, after.UsedQuota)
	// Legitimate control: this fixture really reaches the ordinary exhausted-
	// token update when authentication is called outside the release gate.
	_, err = model.ValidateUserToken(token.Key)
	require.ErrorIs(t, err, model.ErrTokenQuotaExhausted)
	require.NoError(t, db.First(&after, token.Id).Error)
	require.Equal(t, config.TokenStatusExhausted, after.Status)
}
