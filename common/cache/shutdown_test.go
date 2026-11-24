package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"one-api/common/config"
	"one-api/common/lifecycle"
)

func cacheShutdownFixture(t *testing.T) *lifecycle.Group {
	t.Helper()
	oldCache, oldRedisEnabled, oldGroup := kvCache, config.RedisEnabled, lifecycle.Background
	group := lifecycle.NewGroup()
	config.RedisEnabled, lifecycle.Background = false, group
	InitCacheManager()
	t.Cleanup(func() {
		group.Seal()
		kvCache, config.RedisEnabled, lifecycle.Background = oldCache, oldRedisEnabled, oldGroup
	})
	return group
}

func TestShutdownTracksCacheFillAfterWaiterTimesOut(t *testing.T) {
	group := cacheShutdownFixture(t)
	started := make(chan struct{})
	released := make(chan struct{})
	release := sync.OnceFunc(func() { close(released) })
	defer func() {
		release()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := group.Stop(ctx); err != nil {
			t.Errorf("cache fill did not stop during cleanup: %v", err)
		}
	}()
	key := "shutdown-fixture-active-fill"
	_, err := GetOrSetCache(key, time.Minute, func() (string, error) {
		close(started)
		<-released
		return "fixture-value", nil
	}, 0)
	if err == nil || err.Error() != "超时" {
		t.Fatalf("waiter's existing timeout was lost: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("fill never reached its blocking loader")
	}
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelStop()
	if err := group.Stop(stopCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown did not wait for the live cache fill: %v", err)
	}

	release()
	joinCtx, cancelJoin := context.WithTimeout(context.Background(), time.Second)
	defer cancelJoin()
	if err := group.Wait(joinCtx); err != nil {
		t.Fatalf("shutdown failed after the fill completed: %v", err)
	}
	if value, err := GetCache[string](key); err != nil || value != "fixture-value" {
		t.Fatalf("completed fill missing from cache: %q, %v", value, err)
	}
}

func TestShutdownRejectsLateCacheFill(t *testing.T) {
	group := cacheShutdownFixture(t)
	group.Seal()
	loaded := make(chan struct{}, 1)
	key := "shutdown-fixture-late-fill"
	_, err := GetOrSetCache(key, time.Minute, func() (bool, error) {
		loaded <- struct{}{}
		return true, nil
	}, time.Second)
	if !errors.Is(err, lifecycle.ErrWorker) {
		t.Fatalf("late fill was not rejected after sealing: %v", err)
	}
	select {
	case <-loaded:
		t.Fatal("late fill executed its loader")
	default:
	}
	if _, err := GetCache[bool](key); !errors.Is(err, CacheNotFound) {
		t.Fatalf("late fill touched the cache: %v", err)
	}
}
