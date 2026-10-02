package mockserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"one-api/.github/smoke/mockserver"
)

func TestDependencyReachabilityBoundaries(t *testing.T) {
	for _, target := range []string{"localhost:6379", "example.com:443", "database:22", "cache:3306", "127.0.0.1:5432", "database:5432/"} {
		t.Run(target, func(t *testing.T) {
			reachable, err := mockserver.DependencyReachable(target, func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("unexpected dial outside the fixed fixture targets")
				return nil, nil
			})
			require.Error(t, err)
			require.False(t, reachable)
		})
	}
	for _, target := range []string{"database:3306", "database:5432", "cache:6379"} {
		t.Run(target, func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			reachable, err := mockserver.DependencyReachable(target, func(ctx context.Context, network, address string) (net.Conn, error) {
				require.Equal(t, "tcp", network)
				require.Equal(t, target, address)
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), time.Second)
				return local, nil
			})
			require.NoError(t, err)
			require.True(t, reachable)
			_, err = peer.Write([]byte("normal fixture"))
			require.Error(t, err, "probe must close its connection")
		})
	}
}

func TestDependencyReachabilityDeadline(t *testing.T) {
	start := time.Now()
	reachable, err := mockserver.DependencyReachable("cache:6379", func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	require.NoError(t, err)
	require.False(t, reachable)
	require.Less(t, time.Since(start), 2*time.Second)
}
