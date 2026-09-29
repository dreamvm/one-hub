package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/controller"
	"one-api/model"
	provider "one-api/providers/midjourney"
)

func TestMJTaskHandlerRejectsUnboundResults(t *testing.T) {
	for _, kind := range []string{"foreign channel", "unrequested id", "duplicate response"} {
		t.Run(kind, func(t *testing.T) {
			oldClient := requester.HTTPClient
			requester.HTTPClient = &http.Client{}
			t.Cleanup(func() { requester.HTTPClient = oldClient })
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "poll.db")), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			old := model.DB
			model.DB = db
			t.Cleanup(func() { model.DB = old; _ = sqlDB.Close() })
			require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
			task := &model.Midjourney{UserId: 1, ChannelId: 10, MjId: "shared-id", Progress: "20%", Status: "IN_PROGRESS", SubmitTime: time.Now().UnixMilli()}
			require.NoError(t, db.Create(task).Error)
			before := *task
			result := provider.MidjourneyDto{MjId: task.MjId, Progress: "100%", Status: "SUCCESS", ImageUrl: "https://example.com/forged.png", SubmitTime: task.SubmitTime}
			channelID := 10
			if kind == "foreign channel" {
				channelID = 20
			}
			if kind == "unrequested id" {
				result.MjId = "unrequested"
			}
			results := []provider.MidjourneyDto{result}
			if kind == "duplicate response" {
				conflict := result
				conflict.ImageUrl = "https://example.com/conflicting.png"
				results = append(results, conflict)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "upstream-fixture", r.Header.Get("mj-api-secret"))
				_ = json.NewEncoder(w).Encode(results)
			}))
			defer server.Close()
			channel := &model.Channel{Id: channelID, BaseURL: &server.URL, Key: "upstream-fixture"}
			var pollErr error
			require.NotPanics(t, func() {
				pollErr = controller.MjTaskHandler(channel, []string{task.MjId}, map[string]*model.Midjourney{task.MjId: task})
			})
			require.Error(t, pollErr)
			var after model.Midjourney
			require.NoError(t, db.First(&after, task.Id).Error)
			require.Equal(t, before, after)
		})
	}
}

func TestMJBoundPollingUsesChannelProxy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "proxy.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB, oldClient, oldLogger := model.DB, requester.HTTPClient, logger.Logger
	model.DB, logger.Logger = db, zap.NewNop()
	var proxyAddress string
	transport := &http.Transport{Proxy: utils.ProxyFunc, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != proxyAddress {
			return nil, errors.New("fixture blocks direct upstream access")
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	requester.HTTPClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		model.DB, requester.HTTPClient, logger.Logger = oldDB, oldClient, oldLogger
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.Midjourney{}))
	task := &model.Midjourney{UserId: 1, ChannelId: 10, MjId: "proxy-task", Progress: "20%", Status: "IN_PROGRESS", SubmitTime: time.Now().UnixMilli()}
	require.NoError(t, db.Create(task).Error)
	hits := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		require.Equal(t, "upstream.invalid", r.URL.Host)
		require.Equal(t, "channel-proxy-fixture", r.Header.Get("mj-api-secret"))
		_ = json.NewEncoder(w).Encode([]provider.MidjourneyDto{{MjId: task.MjId, Progress: "100%", Status: "SUCCESS", SubmitTime: task.SubmitTime}})
	}))
	defer proxy.Close()
	proxyAddress = proxy.Listener.Addr().String()
	baseURL := "http://upstream.invalid"
	channel := &model.Channel{Id: 10, BaseURL: &baseURL, Key: "channel-proxy-fixture", Proxy: &proxy.URL}
	require.NoError(t, controller.MjTaskHandler(channel, []string{task.MjId}, map[string]*model.Midjourney{task.MjId: task}))
	require.Equal(t, 1, hits)
	var saved model.Midjourney
	require.NoError(t, db.First(&saved, task.Id).Error)
	require.Equal(t, "SUCCESS", saved.Status)
	require.Equal(t, "100%", saved.Progress)
}

func TestMJBoundPollingPreservesDuplicateRowsAndNormalUpdates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "normal.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB, oldClient, oldLogger := model.DB, requester.HTTPClient, logger.Logger
	oldRedis, oldBatch := config.RedisEnabled, config.BatchUpdateEnabled
	model.DB, requester.HTTPClient, logger.Logger = db, &http.Client{}, zap.NewNop()
	config.RedisEnabled, config.BatchUpdateEnabled = false, false
	t.Cleanup(func() {
		model.DB, requester.HTTPClient, logger.Logger = oldDB, oldClient, oldLogger
		config.RedisEnabled, config.BatchUpdateEnabled = oldRedis, oldBatch
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.Midjourney{}, &model.User{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "poll-user", Quota: 100}).Error)
	now := time.Now().UnixMilli()
	tasks := []*model.Midjourney{
		{UserId: 1, ChannelId: 10, MjId: "shared-id", Progress: "20%", Status: "IN_PROGRESS", SubmitTime: now},
		{UserId: 1, ChannelId: 10, MjId: "shared-id", Progress: "20%", Status: "IN_PROGRESS", SubmitTime: now},
		{UserId: 1, ChannelId: 20, MjId: "shared-id", Progress: "20%", Status: "IN_PROGRESS", SubmitTime: now, Quota: 5},
		{UserId: 1, ChannelId: 20, MjId: "", Progress: "20%", SubmitTime: now},
	}
	for _, task := range tasks {
		require.NoError(t, db.Create(task).Error)
	}
	groups, missing := controller.GroupMJTasksForTest(tasks)
	require.Equal(t, []int{tasks[3].Id}, missing)
	require.Len(t, groups[10]["shared-id"], 2)
	require.Len(t, groups[20]["shared-id"], 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/mj/task/list-by-condition", r.URL.Path)
		var request struct {
			IDs []string `json:"ids"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, []string{"shared-id"}, request.IDs)
		result := provider.MidjourneyDto{MjId: "shared-id", SubmitTime: now, Status: "SUCCESS", Progress: "100%", ImageUrl: "https://example.com/channel-a.png", Buttons: []string{"button"}, Properties: &provider.Properties{}}
		if r.Header.Get("mj-api-secret") == "channel-b-fixture" {
			result.Status = "FAILURE"
			result.FailReason = "fixture failure"
			result.ImageUrl = ""
		} else {
			require.Equal(t, "channel-a-fixture", r.Header.Get("mj-api-secret"))
		}
		require.NoError(t, json.NewEncoder(w).Encode([]provider.MidjourneyDto{result, result}))
	}))
	defer server.Close()
	for _, channelID := range []int{10, 20} {
		key := "channel-a-fixture"
		if channelID == 20 {
			key = "channel-b-fixture"
		}
		channel := &model.Channel{Id: channelID, BaseURL: &server.URL, Key: key}
		require.NoError(t, controller.PollMJTaskBatchForTest(channel, []string{"shared-id"}, groups[channelID]))
		require.NoError(t, controller.PollMJTaskBatchForTest(channel, []string{"shared-id"}, groups[channelID]), "identical response remains idempotent in the single poller")
	}
	for _, task := range tasks[:3] {
		var saved model.Midjourney
		require.NoError(t, db.First(&saved, task.Id).Error)
		require.Equal(t, "100%", saved.Progress)
		if task.ChannelId == 10 {
			require.Equal(t, "SUCCESS", saved.Status)
			require.Equal(t, "https://example.com/channel-a.png", saved.ImageUrl)
			require.JSONEq(t, `["button"]`, saved.Buttons)
			require.NotEmpty(t, saved.Properties)
		} else {
			require.Equal(t, "FAILURE", saved.Status)
			require.Empty(t, saved.ImageUrl)
		}
	}
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	require.Equal(t, 105, user.Quota)
}
