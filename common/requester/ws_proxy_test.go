package requester_test

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/types"
)

func websocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	t.Cleanup(server.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	peer := <-accepted
	t.Cleanup(func() { _ = client.Close(); _ = peer.Close() })
	return client, peer
}

func TestWSProxyWaitsForUsageBeforeReturning(t *testing.T) {
	old := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = old })
	user, gatewayUser := websocketPair(t)
	gatewaySupplier, supplier := websocketPair(t)
	entered, release := make(chan struct{}), make(chan struct{})
	proxy := requester.NewWSProxy(gatewayUser, gatewaySupplier, time.Second,
		func(source requester.MessageSource, _ int, _ []byte) (bool, *types.UsageEvent, []byte, error) {
			if source == requester.SupplierMessage {
				return true, &types.UsageEvent{InputTokens: 1}, nil, nil
			}
			return true, nil, nil, nil
		}, func(*types.UsageEvent) error { close(entered); <-release; return nil })
	proxy.Start()
	finished := make(chan struct{})
	go func() { proxy.Wait(); close(finished) }()
	require.NoError(t, supplier.WriteMessage(websocket.TextMessage, []byte("usage")))
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("usage callback not reached")
	}
	_ = user.Close()
	early := false
	select {
	case <-finished:
		early = true
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	proxy.Close()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("proxy did not join workers")
	}
	require.False(t, early, "settlement must wait for an in-flight usage callback")
}

func TestWSProxyClosesSupplierBeforeBlockedBudgetError(t *testing.T) {
	old := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = old })
	user, gatewayUser := websocketPair(t)
	gatewaySupplier, supplier := websocketPair(t)
	proxy := requester.NewWSProxy(gatewayUser, gatewaySupplier, 3*time.Second,
		func(source requester.MessageSource, _ int, _ []byte) (bool, *types.UsageEvent, []byte, error) {
			return true, &types.UsageEvent{InputTokens: 1}, nil, nil
		}, func(*types.UsageEvent) error { return errors.New("fixture budget exhausted") })
	proxy.Start()
	// Do not read the user connection: error delivery must not keep a known
	// underfunded upstream alive while this large response blocks on backpressure.
	require.NoError(t, supplier.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", 16<<20))))
	_ = supplier.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := supplier.ReadMessage()
	_ = user.Close()
	proxy.Close()
	proxy.Wait()
	require.Error(t, err)
	var timeout net.Error
	require.False(t, errors.As(err, &timeout) && timeout.Timeout(), "supplier stayed open while client delivery blocked")
}
