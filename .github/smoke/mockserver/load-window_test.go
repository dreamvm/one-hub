package mockserver_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"one-api/.github/smoke/mockserver"
)

func loadControl(server *mockserver.Server, action, key string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/fixture/load-window", strings.NewReader(`{"action":"`+action+`"}`))
	r.Header.Set("Authorization", "Bearer "+key)
	server.ServeHTTP(w, r)
	return w
}

func loadState(t *testing.T, server *mockserver.Server) map[string]int {
	t.Helper()
	var state struct {
		Window map[string]int `json:"window"`
	}
	require.NoError(t, json.Unmarshal(loadControl(server, "state", "fixture-load-control").Body.Bytes(), &state))
	return state.Window
}

func TestBoundedLoadWindowOverlapDrainAndNormalControl(t *testing.T) {
	server := mockserver.New()
	require.Equal(t, 400, loadControl(server, "arm", "wrong-key").Code)
	require.Equal(t, 400, loadControl(server, "unknown", "fixture-load-control").Code)
	require.Equal(t, 200, loadControl(server, "arm", "fixture-load-control").Code)
	require.Equal(t, 409, loadControl(server, "arm", "fixture-load-control").Code)
	require.Equal(t, 409, gateControl(server, "arm", "fixture-gate-control").Code)
	var workers sync.WaitGroup
	outputs := make([]*httptest.ResponseRecorder, 4)
	for i := range outputs {
		outputs[i] = httptest.NewRecorder()
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			server.ServeHTTP(outputs[i], gateRequest(i%2 == 1))
		}(i)
	}
	require.Eventually(t, func() bool { return loadState(t, server)["active"] == 4 }, time.Second, time.Millisecond)
	require.Equal(t, 409, loadControl(server, "disarm", "fixture-load-control").Code)
	refused := httptest.NewRecorder()
	server.ServeHTTP(refused, gateRequest(false))
	require.Equal(t, 400, refused.Code)
	workers.Wait()
	for _, output := range outputs {
		require.Equal(t, 200, output.Code)
		require.Contains(t, output.Body.String(), "中文对话成功")
	}
	require.Equal(t, map[string]int{"started": 4, "completed": 4, "cancelled": 0, "active": 0, "peak": 4}, loadState(t, server))
	require.Equal(t, 200, loadControl(server, "disarm", "fixture-load-control").Code)
	require.Equal(t, 200, gateControl(server, "arm", "fixture-gate-control").Code)
	require.Equal(t, 409, loadControl(server, "arm", "fixture-load-control").Code)
	require.Equal(t, 200, gateControl(server, "release", "fixture-gate-control").Code)
	normal := httptest.NewRecorder()
	server.ServeHTTP(normal, gateRequest(false))
	require.Equal(t, 200, normal.Code)
	require.Contains(t, normal.Body.String(), "中文对话成功")
}

func TestBoundedLoadWindowCancellationAndRequestBudget(t *testing.T) {
	server := mockserver.New()
	require.Equal(t, 200, loadControl(server, "arm", "fixture-load-control").Code)
	for i := 0; i < 32; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		w := httptest.NewRecorder()
		server.ServeHTTP(w, gateRequest(false).WithContext(ctx))
		require.Equal(t, 400, w.Code)
	}
	require.Equal(t, map[string]int{"started": 32, "completed": 0, "cancelled": 32, "active": 0, "peak": 1}, loadState(t, server))
	exhausted := httptest.NewRecorder()
	server.ServeHTTP(exhausted, gateRequest(false))
	require.Equal(t, 400, exhausted.Code)
	require.Equal(t, 32, loadState(t, server)["started"])
	require.Equal(t, 200, loadControl(server, "disarm", "fixture-load-control").Code)
}
