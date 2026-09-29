package midjourney_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"one-api/common"
	"one-api/common/config"
	img "one-api/common/image"
	"one-api/common/logger"
	"one-api/controller"
	"one-api/model"
	"one-api/router"
)

func mjFixture(t *testing.T) (*gin.Engine, *gorm.DB, []model.Token) {
	t.Helper()
	oldLogger, oldMode := logger.Logger, gin.Mode()
	logger.Logger = zap.NewNop()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { logger.Logger = oldLogger; gin.SetMode(oldMode) })
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "mj.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB, oldRedis := model.DB, config.RedisEnabled
	oldGroups, oldLimiters, oldPublic := model.GlobalUserGroupRatio.UserGroup, model.GlobalUserGroupRatio.APILimiter, model.GlobalUserGroupRatio.PublicGroup
	model.DB, config.RedisEnabled = db, false
	t.Cleanup(func() {
		controller.DeactivateMidjourneyTaskBulk()
		model.DB, config.RedisEnabled = oldDB, oldRedis
		model.GlobalUserGroupRatio.UserGroup, model.GlobalUserGroupRatio.APILimiter, model.GlobalUserGroupRatio.PublicGroup = oldGroups, oldLimiters, oldPublic
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.Midjourney{}, &model.User{}, &model.Token{}, &model.UserGroup{}))
	require.NoError(t, db.Create(&model.UserGroup{Symbol: "default", Ratio: 1, APIRate: 10000}).Error)
	model.GlobalUserGroupRatio.Load()
	for _, u := range []model.User{{Id: 1, Username: "mj-owner", AccessToken: "mj-owner-access-fixture", AffCode: "mj-owner-aff", Group: "default", Status: config.UserStatusEnabled}, {Id: 2, Username: "mj-other", AccessToken: "mj-other-access-fixture", AffCode: "mj-other-aff", Group: "default", Status: config.UserStatusEnabled}} {
		require.NoError(t, db.Create(&u).Error)
	}
	viper.Set("user_token_secret", "midjourney-test-fixture-only")
	require.NoError(t, common.InitUserToken())
	tokens := []model.Token{{UserId: 1, Name: "owner", UnlimitedQuota: true, ExpiredTime: -1}, {UserId: 2, Name: "other", UnlimitedQuota: true, ExpiredTime: -1}}
	for i := range tokens {
		require.NoError(t, db.Create(&tokens[i]).Error)
	}
	r := gin.New()
	router.SetRelayRouter(r)
	return r, db, tokens
}

func mjTask(t *testing.T, db *gorm.DB, imageURL string) model.Midjourney {
	t.Helper()
	task := model.Midjourney{UserId: 1, MjId: "fixture-task", ChannelId: 10, ImageUrl: imageURL, Progress: "20%", Status: "IN_PROGRESS", SubmitTime: time.Now().UnixMilli()}
	require.NoError(t, db.Create(&task).Error)
	return task
}

func TestMJNotifyCannotWriteTaskState(t *testing.T) {
	for _, prefix := range []string{"/mj", "/mj-fast/mj", "/mj-relax/mj", "/mj-turbo/mj"} {
		for _, identity := range []string{"anonymous", "other", "owner"} {
			t.Run(prefix+identity, func(t *testing.T) {
				r, db, tokens := mjFixture(t)
				before := mjTask(t, db, "https://example.com/original.png")
				payload := []byte(`{"id":"fixture-task","status":"SUCCESS","progress":"100%","imageUrl":"http://127.0.0.1/private","promptEn":"forged","finishTime":99}`)
				req := httptest.NewRequest(http.MethodPost, prefix+"/notify", bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				if identity != "anonymous" {
					idx := 0
					if identity == "other" {
						idx = 1
					}
					req.Header.Set("mj-api-secret", tokens[idx].Key)
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if identity == "owner" {
					require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				} else {
					require.NotEqual(t, http.StatusOK, w.Code, "foreign or absent identity was accepted")
				}
				var after model.Midjourney
				require.NoError(t, db.First(&after, before.Id).Error)
				require.Equal(t, before, after, "notification input must never become authoritative task state")
			})
		}
	}
}

func TestMJImageRejectsStoredPrivateTargets(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(w, "private fixture")
	}))
	defer server.Close()
	for _, prefix := range []string{"/mj", "/mj-fast/mj", "/mj-relax/mj", "/mj-turbo/mj"} {
		t.Run(prefix, func(t *testing.T) {
			r, db, _ := mjFixture(t)
			mjTask(t, db, server.URL)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, prefix+"/image/fixture-task", nil))
			require.NotEqual(t, http.StatusOK, w.Code)
			require.NotContains(t, w.Body.String(), "private fixture")
		})
	}
	require.Zero(t, hits.Load(), "stored historical URLs must be checked before connecting")
}

type mjRoundTripper func(*http.Request) (*http.Response, error)

func (f mjRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMJImagePublicBinaryControl(t *testing.T) {
	r, db, _ := mjFixture(t)
	mjTask(t, db, "https://example.com/pixel?sig=a%2Fb")
	old := img.ImageHttpClients
	oldWorker := config.CFWorkerImageUrl
	t.Cleanup(func() { img.ImageHttpClients = old; config.CFWorkerImageUrl = oldWorker })
	// This endpoint has a raw binary contract even if the separate Worker mode is enabled.
	config.CFWorkerImageUrl = "http://worker.invalid/unexpected"
	img.ImageHttpClients = &http.Client{Transport: mjRoundTripper(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "example.com", req.URL.Host)
		require.Equal(t, "sig=a%2Fb", req.URL.RawQuery)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(strings.NewReader("public image fixture"))}, nil
	})}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/mj/image/fixture-task", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, "public image fixture", w.Body.String())
}

// Keep notification success compatible with the existing empty HTTP 200 response.
func TestMJNotifyRejectsMalformedAndMissingTasks(t *testing.T) {
	r, _, tokens := mjFixture(t)
	for _, body := range []string{`{`, `{"id":"missing"}`, `{}`} {
		req := httptest.NewRequest(http.MethodPost, "/mj/notify", strings.NewReader(body))
		req.Header.Set("mj-api-secret", tokens[0].Key)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.NotEqual(t, 200, w.Code)
		var result map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		require.NotEmpty(t, result["description"])
	}
}

type mjTrackedBody struct {
	io.Reader
	closed bool
}

func (b *mjTrackedBody) Close() error { b.closed = true; return nil }

type mjBrokenReader struct{}

func (mjBrokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestMJImageResponseFailureBoundaries(t *testing.T) {
	for _, kind := range []string{"upstream error", "broken body", "oversized", "missing content type"} {
		t.Run(kind, func(t *testing.T) {
			r, db, _ := mjFixture(t)
			mjTask(t, db, "https://example.com/image")
			old := img.ImageHttpClients
			t.Cleanup(func() { img.ImageHttpClients = old })
			body := &mjTrackedBody{Reader: strings.NewReader("upstream-detail-fixture")}
			status := http.StatusBadGateway
			switch kind {
			case "broken body":
				status = 200
				body.Reader = mjBrokenReader{}
			case "oversized":
				status = 200
				body.Reader = io.LimitReader(zeroMJReader{}, 20*1024*1024+1)
			case "missing content type":
				status = 200
				body.Reader = strings.NewReader("image fixture")
			}
			img.ImageHttpClients = &http.Client{Transport: mjRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: body, Header: make(http.Header)}, nil
			})}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/mj/image/fixture-task", nil))
			if kind == "missing content type" {
				require.Equal(t, 200, w.Code)
				require.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
				require.Equal(t, "image fixture", w.Body.String())
			} else {
				require.Equal(t, http.StatusBadGateway, w.Code)
				require.NotContains(t, w.Body.String(), "upstream-detail-fixture")
				require.Less(t, w.Body.Len(), 100)
			}
			require.True(t, body.closed)
		})
	}
}

type zeroMJReader struct{}

func (zeroMJReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestMJNotifyCompletedTaskAndInvalidMode(t *testing.T) {
	r, db, tokens := mjFixture(t)
	task := mjTask(t, db, "https://example.com/image")
	require.NoError(t, db.Model(&task).Updates(map[string]any{"progress": "100%", "status": "SUCCESS"}).Error)
	for _, prefix := range []string{"/mj", "/invalid/mj"} {
		req := httptest.NewRequest(http.MethodPost, prefix+"/notify", strings.NewReader(`{"id":"fixture-task","status":"FAILURE","progress":"0%"}`))
		req.Header.Set("mj-api-secret", tokens[0].Key)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if prefix == "/mj" {
			require.Equal(t, 200, w.Code)
		} else {
			require.NotEqual(t, 200, w.Code)
		}
		var saved model.Midjourney
		require.NoError(t, db.First(&saved, task.Id).Error)
		require.Equal(t, "SUCCESS", saved.Status)
		require.Equal(t, "100%", saved.Progress)
	}
}
