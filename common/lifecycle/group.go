// Package lifecycle owns application work across HTTP admission and shutdown.
// A deadline failure is not evidence that a worker stopped. The caller must
// exit nonzero without closing shared resources underneath unfinished work.
package lifecycle

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrWorker = errors.New("shutdown_worker_failed")

// Group seals registration before waiting; Add and Wait cannot race at zero.
type Group struct {
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	active         int
	closed, failed bool
}

func NewGroup() *Group {
	ctx, cancel := context.WithCancel(context.Background())
	return &Group{ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

var Background = NewGroup()

func (g *Group) Context() context.Context { return g.ctx }
func (g *Group) Stopping() bool           { return g.ctx.Err() != nil }
func (g *Group) Fail()                    { g.mu.Lock(); g.failed = true; g.mu.Unlock() }

func (g *Group) enter() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	g.active++
	return true
}

func (g *Group) leave() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if g.closed && g.active == 0 {
		close(g.done)
	}
}

// Register tracks work started by another component. Call the returned finish once.
func (g *Group) Register() (finish func(), accepted bool) {
	if !g.enter() {
		g.Fail()
		return nil, false
	}
	return g.leave, true
}

// Go registers before starting. Rejected work never executes and blocks success.
func (g *Group) Go(f func()) bool {
	finish, accepted := g.Register()
	if !accepted {
		return false
	}
	go func() { defer finish(); f() }()
	return true
}

func (g *Group) Seal() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		g.closed = true
		g.cancel()
		if g.active == 0 {
			close(g.done)
		}
	}
}

func (g *Group) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.done:
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.failed {
			return ErrWorker
		}
		return ctx.Err()
	}
}

func (g *Group) Stop(ctx context.Context) error { g.Seal(); return g.Wait(ctx) }

func (g *Group) Sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-g.ctx.Done():
		return false
	case <-t.C:
		return !g.Stopping()
	}
}

// Within bounds the caller, not an arbitrary callback. On timeout its goroutine
// may still run; callers must not declare completion or close its database.
func Within(ctx context.Context, f func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- f() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
}
