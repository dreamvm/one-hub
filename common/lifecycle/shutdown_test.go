package lifecycle_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"one-api/common/lifecycle"
)

func await[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("test deadline")
		var v T
		return v
	}
}

func listener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestShutdownHTTPOrdinaryAndStreamingSettlement(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "SSE"}[stream], func(t *testing.T) {
			l := listener(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			var settled, wrote, closed atomic.Bool
			worker := lifecycle.NewGroup()
			worker.Go(func() { <-release; wrote.Store(true) })
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer settled.Store(true) // terminal accounting completes after response
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: partial\n\n")
					w.(http.Flusher).Flush()
				}
				close(entered)
				<-release
				if r.Context().Err() != nil {
					t.Error("TERM canceled admitted request")
				}
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
			})
			result := make(chan error, 1)
			go func() {
				result <- lifecycle.Serve(ctx, l, handler, time.Second,
					func(ctx context.Context) error {
						if !settled.Load() {
							return errors.New("settlement not awaited")
						}
						return worker.Stop(ctx)
					},
					func(context.Context) error {
						if !wrote.Load() {
							return errors.New("writer not joined")
						}
						return nil
					},
					func() error { closed.Store(true); return nil })
			}()
			response := make(chan error, 1)
			go func() {
				r, err := http.Get("http://" + l.Addr().String())
				if err == nil {
					defer r.Body.Close()
					body, e := io.ReadAll(r.Body)
					err = e
					if string(body) != "data: [DONE]\n\n" && string(body) != "data: partial\n\ndata: [DONE]\n\n" {
						err = errors.New("truncated response")
					}
				}
				response <- err
			}()
			await(t, entered)
			cancel()
			select {
			case err := <-result:
				t.Fatalf("exited with active request: %v", err)
			case <-time.After(20 * time.Millisecond):
			}
			if closed.Load() {
				t.Fatal("closed database under admitted request")
			}
			close(release)
			if err := await(t, result); err != nil {
				t.Fatal(err)
			}
			if err := await(t, response); err != nil {
				t.Fatal(err)
			}
			if !closed.Load() || !settled.Load() || !wrote.Load() {
				t.Fatal("incomplete shutdown")
			}
		})
	}
}

func TestShutdownDeadlineDoesNotCloseDatabase(t *testing.T) {
	l := listener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, clientDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var finalized, closed atomic.Bool
	result := make(chan error, 1)
	go func() {
		result <- lifecycle.Serve(ctx, l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-release
			_, _ = io.WriteString(w, "complete")
		}), 40*time.Millisecond,
			func(context.Context) error { return nil }, func(context.Context) error { finalized.Store(true); return nil }, func() error { closed.Store(true); return nil })
	}()
	go func() {
		defer close(clientDone)
		r, err := http.Get("http://" + l.Addr().String())
		if err == nil {
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
		}
	}()
	await(t, entered)
	cancel()
	if err := await(t, result); err == nil || err.Error() != "shutdown_http_deadline" {
		t.Fatalf("unexpected %v", err)
	}
	if finalized.Load() || closed.Load() {
		t.Fatal("advanced after failed drain")
	}
	close(release)
	await(t, clientDone)
}

func TestShutdownPhaseFailureDoesNotAdvance(t *testing.T) {
	for _, fail := range []string{"worker", "accounting", "database"} {
		t.Run(fail, func(t *testing.T) {
			l := listener(t)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var calls []string
			step := func(name string) error {
				calls = append(calls, name)
				if name == fail {
					return errors.New("private detail")
				}
				return nil
			}
			err := lifecycle.Serve(ctx, l, http.NotFoundHandler(), time.Second, func(context.Context) error { return step("worker") }, func(context.Context) error { return step("accounting") }, func() error { return step("database") })
			if err == nil || err.Error() == "private detail" {
				t.Fatalf("unsafe result %v", err)
			}
			expect := map[string]int{"worker": 1, "accounting": 2, "database": 3}[fail]
			if len(calls) != expect {
				t.Fatalf("advanced: %v", calls)
			}
		})
	}
}

func TestShutdownUncooperativeWorkerDeadline(t *testing.T) {
	l := listener(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release, returned := make(chan struct{}), make(chan struct{})
	var closed atomic.Bool
	err := lifecycle.Serve(ctx, l, http.NotFoundHandler(), 30*time.Millisecond, func(context.Context) error { defer close(returned); <-release; return nil }, func(context.Context) error { return nil }, func() error { closed.Store(true); return nil })
	if err == nil || closed.Load() {
		t.Fatal("unjoined worker accepted")
	}
	close(release)
	await(t, returned)
}

func TestAdmissionSealDeniesNewWorkButWaitsDeferredSettlement(t *testing.T) {
	a := lifecycle.NewAdmission()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	h := a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); close(entered); <-release }))
	go func() { defer close(done); h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil)) }()
	await(t, entered)
	a.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
	if w.Code != 503 || calls.Load() != 1 {
		t.Fatal("admitted after seal")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if a.Wait(ctx) == nil {
		t.Fatal("lost active work")
	}
	close(release)
	await(t, done)
	if err := a.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionKeepsOriginalHijackerAndCountsItsHandler(t *testing.T) {
	a := lifecycle.NewAdmission()
	hijacked, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s := httptest.NewServer(a.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()
		close(hijacked)
		<-release
	})))
	defer s.Close()
	c, err := net.Dial("tcp", s.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, _ = io.WriteString(c, "GET / HTTP/1.1\r\nHost: fixture\r\n\r\n")
	await(t, hijacked)
	a.Close()
	go func() { _ = a.Wait(context.Background()); close(done) }()
	select {
	case <-done:
		t.Fatal("hijack omitted")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	await(t, done)
	_, _ = bufio.NewReader(c).ReadByte()
}

func TestGroupConcurrentSealAndRegistration(t *testing.T) {
	g := lifecycle.NewGroup()
	var wg sync.WaitGroup
	var ran atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); g.Go(func() { ran.Add(1) }) }()
	}
	g.Seal()
	wg.Wait()
	err := g.Wait(context.Background())
	if err != nil && !errors.Is(err, lifecycle.ErrWorker) {
		t.Fatal(err)
	}
	before := ran.Load()
	if g.Go(func() { ran.Add(1) }) {
		t.Fatal("late registration accepted")
	}
	if ran.Load() != before {
		t.Fatal("late callback ran")
	}
	if !errors.Is(g.Wait(context.Background()), lifecycle.ErrWorker) {
		t.Fatal("rejected writer hidden")
	}
}

func TestGroupSleepAndFailure(t *testing.T) {
	g := lifecycle.NewGroup()
	done := make(chan struct{})
	g.Go(func() {
		defer close(done)
		if g.Sleep(time.Hour) {
			t.Error("sleep not canceled")
		}
	})
	g.Seal()
	await(t, done)
	if err := g.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	g.Fail()
	if !errors.Is(g.Wait(context.Background()), lifecycle.ErrWorker) {
		t.Fatal("failure hidden")
	}
}
