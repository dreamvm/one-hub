package check_channel

import (
	"context"
	"testing"
	"time"

	"one-api/common/cache"
	"one-api/common/config"
	providers_base "one-api/providers/base"
	"one-api/types"
)

type streamTestProvider struct {
	providers_base.ChatInterface
	complete func()
}

const probesPerModel = 5

func (p streamTestProvider) CreateChatCompletion(*types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	p.complete()
	return nil, &types.OpenAIErrorWithStatusCode{OpenAIError: types.OpenAIError{Message: "synthetic diagnostic reply"}}
}

func useInMemoryImageCache(t *testing.T) {
	t.Helper()
	redisEnabled := config.RedisEnabled
	config.RedisEnabled = false
	cache.InitCacheManager()
	t.Cleanup(func() {
		config.RedisEnabled = redisEnabled
		cache.InitCacheManager()
	})
}

func TestRunStreamCompletesOrderedResults(t *testing.T) {
	useInMemoryImageCache(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	models := []string{"gpt-fixture-first", "gpt-fixture-second"}
	check := &CheckChannel{
		Models:        models,
		ChatInterface: streamTestProvider{complete: func() {}},
	}
	results := make(chan *ModelResult)
	go check.RunStream(ctx, results)

	for _, model := range models {
		select {
		case result, ok := <-results:
			if !ok || result.Model != model || len(result.Process) != probesPerModel {
				t.Fatalf("missing or incomplete diagnostics for %s: %+v", model, result)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for diagnostics")
		}
	}
	select {
	case _, ok := <-results:
		if ok {
			t.Fatal("received an extra diagnostic result")
		}
	case <-ctx.Done():
		t.Fatal("worker left the result channel open")
	}
}

func TestRunStreamStopsAfterClientCancellation(t *testing.T) {
	useInMemoryImageCache(t)
	for _, name := range []string{"during probe", "while result is unread"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			model := "gpt-fixture-cancel"
			cancelAfterProbe := 1
			if name == "while result is unread" {
				cancelAfterProbe = probesPerModel
			}
			probeReached := make(chan struct{})
			calls := 0
			check := &CheckChannel{
				Models: []string{model, "gpt-fixture-must-not-start"},
				ChatInterface: streamTestProvider{complete: func() {
					calls++
					if calls == cancelAfterProbe {
						close(probeReached)
						if cancelAfterProbe == 1 {
							<-ctx.Done()
						}
					}
				}},
			}
			results := make(chan *ModelResult)
			workerDone := make(chan struct{})
			go func() {
				defer close(workerDone)
				check.RunStream(ctx, results)
			}()

			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-probeReached:
				cancel()
			case <-timer.C:
				t.Fatal("worker did not reach the cancellation fixture")
			}
			select {
			case <-workerDone:
				if calls != cancelAfterProbe {
					t.Fatalf("started extra probes after cancellation: got %d, want %d", calls, cancelAfterProbe)
				}
				if _, ok := <-results; ok {
					t.Fatal("unexpected diagnostic result after cancellation")
				}
			case <-timer.C:
				t.Fatal("worker remained blocked after cancellation")
			}
		})
	}
}
