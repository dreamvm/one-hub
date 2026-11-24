package gemini

import (
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"one-api/model"
	"one-api/types"
	"os"
	"testing"
	"time"
)

func cancelTestRequest(t *testing.T, id int, ctx context.Context) *http.Request {
	t.Helper()
	proxy := ""
	p := (GeminiProviderFactory{}).Create(&model.Channel{Id: id, Proxy: &proxy}).(*GeminiProvider)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(ctx)
	p.SetContext(c)
	req, e := p.getChatRequest(&GeminiChatRequest{Model: "fixture", Stream: true}, false)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { req.Body.Close() })
	return req
}
func TestGeminiClientCancelScope(t *testing.T) {
	t.Setenv("GEMINI_CANCEL_CHANNELS", "7101")
	for _, id := range []int{7101, 7102, 7103} {
		ctx, cancel := context.WithCancel(context.Background())
		req := cancelTestRequest(t, id, ctx)
		cancel()
		select {
		case <-req.Context().Done():
			if id != 7101 {
				t.Fatalf("changed channel %d", id)
			}
		case <-time.After(150 * time.Millisecond):
			if id == 7101 {
				t.Fatal("configured synthetic channel upstream did not cancel")
			}
		}
	}
}
func TestGeminiClientCancelBeforeHeaders(t *testing.T) {
	t.Setenv("GEMINI_CANCEL_CHANNELS", "7101")
	started, stopped := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := cancelTestRequest(t, 7101, ctx)
	req.URL, _ = req.URL.Parse(srv.URL)
	done := make(chan struct{})
	go func() {
		resp, _ := srv.Client().Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		close(done)
	}()
	<-started
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		srv.CloseClientConnections()
		t.Error("upstream not canceled")
	}
	<-done
}
func TestGeminiClientLongResponse(t *testing.T) {
	t.Setenv("GEMINI_CANCEL_CHANNELS", "7101")
	delay := 20 * time.Millisecond
	if os.Getenv("GEMINI_LONG_TEST") == "1" {
		delay = 205 * time.Second
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-time.After(delay):
			io.WriteString(w, "data: complete\n\n")
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := cancelTestRequest(t, 7101, ctx)
	req.URL, _ = req.URL.Parse(srv.URL)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil || string(b) != "data: complete\n\n" {
		t.Fatalf("long response: %q %v", b, err)
	}
}
func TestGeminiClientAlreadyCanceled(t *testing.T) {
	t.Setenv("GEMINI_CANCEL_CHANNELS", "7101")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if cancelTestRequest(t, 7101, ctx).Context().Err() == nil {
		t.Fatal("already canceled context lost")
	}
}
func TestGeminiClientCancelDuringBody(t *testing.T) {
	t.Setenv("GEMINI_CANCEL_CHANNELS", "7101")
	stopped := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(stopped)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := cancelTestRequest(t, 7101, ctx)
	req.URL, _ = req.URL.Parse(srv.URL)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b := make([]byte, len("data: first\n\n"))
	if _, err = io.ReadFull(resp.Body, b); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		srv.CloseClientConnections()
		t.Fatal("body not canceled")
	}
	if _, err = io.ReadAll(resp.Body); err == nil {
		t.Fatal("canceled response reported complete")
	}
}
func TestGeminiClientCancelErrorPolicy(t *testing.T) {
	t.Setenv("GEMINI_CANCEL_CHANNELS", "7101")
	proxy := ""
	p := (GeminiProviderFactory{}).Create(&model.Channel{Id: 7101, Proxy: &proxy}).(*GeminiProvider)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(ctx)
	p.SetContext(c)
	original := &types.OpenAIErrorWithStatusCode{StatusCode: 503}
	if p.clientCancellationError(original) != original {
		t.Fatal("normal retry policy changed")
	}
	cancel()
	got := p.clientCancellationError(original)
	if !got.LocalError || original.LocalError {
		t.Fatal("cancellation must be local without mutating original")
	}
	p.Channel.Id = 7102
	if p.clientCancellationError(original) != original {
		t.Fatal("unconfigured synthetic channel policy changed")
	}
}
