package lifecycle

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Admission counts the entire handler, including its deferred accounting. It
// passes the original ResponseWriter through, preserving streaming/hijacking.
type Admission struct{ work *Group }

func NewAdmission() *Admission                      { return &Admission{work: NewGroup()} }
func (a *Admission) Close()                         { a.work.Seal() }
func (a *Admission) Wait(ctx context.Context) error { return a.work.Wait(ctx) }
func (a *Admission) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.work.enter() {
			w.Header().Set("Connection", "close")
			http.Error(w, "server draining", http.StatusServiceUnavailable)
			return
		}
		defer a.work.leave()
		next.ServeHTTP(w, r)
	})
}

// Serve never cancels request contexts on TERM. All phases share one deadline.
// No close callback or success is reached after any incomplete/failed phase.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler,
	timeout time.Duration, stopWorkers, finalize func(context.Context) error,
	closeDB func() error) error {
	if timeout <= 0 || stopWorkers == nil || finalize == nil || closeDB == nil {
		return errors.New("shutdown_invalid_configuration")
	}
	admission := NewAdmission()
	server := &http.Server{Handler: admission.Handler(handler)}
	exited := make(chan error, 1)
	go func() { exited <- server.Serve(listener) }()
	select {
	case err := <-exited:
		admission.Close()
		return err
	case <-ctx.Done():
	}
	admission.Close()
	drain, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(drain); err != nil {
		return errors.New("shutdown_http_deadline")
	}
	if err := admission.Wait(drain); err != nil {
		return errors.New("shutdown_requests_incomplete")
	}
	if err := <-exited; !errors.Is(err, http.ErrServerClosed) {
		return errors.New("shutdown_http_failed")
	}
	if err := Within(drain, func() error { return stopWorkers(drain) }); err != nil {
		return errors.New("shutdown_workers_incomplete")
	}
	if err := Within(drain, func() error { return finalize(drain) }); err != nil {
		return errors.New("shutdown_accounting_incomplete")
	}
	if err := Within(drain, closeDB); err != nil {
		return errors.New("shutdown_database_close_failed")
	}
	return nil
}
