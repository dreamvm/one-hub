package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
)

func TestShutdownSchedulerWaitsRunningWriter(t *testing.T) {
	s, err := gocron.NewScheduler()
	if err != nil {
		t.Fatal(err)
	}
	tm := &TaskManager{scheduler: s, jobs: make(map[string]*JobInfo)}
	entered, release := make(chan struct{}), make(chan struct{})
	if err := tm.AddJob("writer", gocron.DurationJob(time.Hour), gocron.NewTask(func() { close(entered); <-release }), gocron.WithStartAt(gocron.WithStartImmediately())); err != nil {
		t.Fatal(err)
	}
	s.Start()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("job not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- tm.Shutdown(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("writer not joined: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := tm.AddJob("late", gocron.DurationJob(time.Hour), gocron.NewTask(func() { t.Error("late job ran") })); err == nil {
		t.Fatal("registered after shutdown")
	}
}

func TestShutdownSchedulerPropagatesUnjoinedJob(t *testing.T) {
	s, err := gocron.NewScheduler(gocron.WithStopTimeout(20 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	tm := &TaskManager{scheduler: s, jobs: make(map[string]*JobInfo)}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	if err := tm.AddJob("held", gocron.DurationJob(time.Hour), gocron.NewTask(func() { defer close(done); close(entered); <-release }), gocron.WithStartAt(gocron.WithStartImmediately())); err != nil {
		t.Fatal(err)
	}
	s.Start()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("job not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = tm.Shutdown(ctx)
	if err == nil {
		t.Fatal("gocron timeout hidden")
	}
	close(release)
	<-done
}
