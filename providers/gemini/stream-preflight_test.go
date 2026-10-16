package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"one-api/common"
	"one-api/common/logger"
	"one-api/common/requester"
	"one-api/common/utils"
	"one-api/types"
)

func TestMain(m *testing.M) {
	previous := logger.Logger
	logger.Logger = zap.NewNop()
	code := m.Run()
	logger.Logger = previous
	os.Exit(code)
}

func preflightEvent(parts []GeminiPart, finish string, usage *GeminiUsageMetadata) string {
	candidate := GeminiChatCandidate{Content: GeminiChatContent{Parts: parts}}
	if finish != "" {
		candidate.FinishReason = &finish
	}
	b, _ := json.Marshal(GeminiChatResponse{Candidates: []GeminiChatCandidate{candidate}, UsageMetadata: usage})
	return "data: " + string(b) + "\n\n"
}

func preflightSuccess() string {
	return preflightEvent([]GeminiPart{{Text: "中文成功"}}, "STOP", nil)
}

func preflightOptions() geminiPreflightOptions {
	return geminiPreflightOptions{maxPreludeBytes: 1 << 20, totalTimeout: 2 * time.Second}
}

func preflightInstallHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	previous := requester.HTTPClient
	requester.HTTPClient = client
	t.Cleanup(func() { requester.HTTPClient = previous })
}

func preflightHTTPTest(t *testing.T, first string, allowRetry bool) (string, int, *types.Usage, *types.OpenAIErrorWithStatusCode) {
	t.Helper()
	var calls atomic.Int32
	var requestBodies []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requestBodies = append(requestBodies, string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			io.WriteString(w, first)
		} else {
			io.WriteString(w, preflightSuccess())
		}
	}))
	t.Cleanup(server.Close)
	preflightInstallHTTPClient(t, server.Client())
	requestBody := `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"HIGH"},"maxOutputTokens":10000}}`
	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(requestBody))
	defer req.Body.Close()
	r := requester.NewHTTPRequester("", nil)
	resp, prior, apiErr := preflightGeminiStream(req, context.Background(), r.SendRequestRaw, allowRetry, preflightOptions())
	var output string
	if resp != nil {
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read replay: %v", err)
		}
		output = string(b)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, body := range requestBodies {
		if body != requestBody {
			t.Fatalf("retry changed high, budget, or serialized request: %s", body)
		}
	}
	return output, int(calls.Load()), prior, apiErr
}

func TestGeminiPreflightEmptyThenSuccess(t *testing.T) {
	for _, reason := range []string{"STOP", "MALFORMED_FUNCTION_CALL"} {
		t.Run(reason, func(t *testing.T) {
			first := preflightEvent(nil, reason, &GeminiUsageMetadata{PromptTokenCount: 12, ThoughtsTokenCount: 7, TotalTokenCount: 19})
			output, calls, prior, err := preflightHTTPTest(t, first, true)
			if err != nil || calls != 2 || output != preflightSuccess() {
				t.Fatalf("err=%v calls=%d output=%q", err, calls, output)
			}
			if prior.PromptTokens != 12 || prior.CompletionTokens != 7 || prior.CompletionTokensDetails.ReasoningTokens != 7 || prior.TotalTokens != 19 {
				t.Fatalf("discarded usage lost: %+v", prior)
			}
		})
	}
}

func TestGeminiPreflightThoughtPreludeDiscarded(t *testing.T) {
	first := preflightEvent([]GeminiPart{{Thought: true, Text: "private thought never returned"}}, "", &GeminiUsageMetadata{PromptTokenCount: 4, ThoughtsTokenCount: 1, TotalTokenCount: 5}) +
		preflightEvent(nil, "STOP", &GeminiUsageMetadata{PromptTokenCount: 4, ThoughtsTokenCount: 3, TotalTokenCount: 7})
	output, calls, prior, err := preflightHTTPTest(t, first, true)
	if err != nil || calls != 2 || strings.Contains(output, "private thought") || prior.PromptTokens != 4 || prior.CompletionTokens != 3 || prior.TotalTokens != 7 {
		t.Fatalf("thought retry result: calls=%d usage=%+v err=%v output=%q", calls, prior, err, output)
	}
}

func TestGeminiPreflightNoRetryBoundaries(t *testing.T) {
	for _, reason := range []string{"MAX_TOKENS", "SAFETY", "RECITATION", "OTHER", "UNKNOWN_REASON", "MISSING_THOUGHT_SIGNATURE", "UNEXPECTED_TOOL_CALL"} {
		t.Run(reason, func(t *testing.T) {
			_, calls, _, _ := preflightHTTPTest(t, preflightEvent(nil, reason, nil), true)
			if calls != 1 {
				t.Fatalf("unsafe retry for %s: %d", reason, calls)
			}
		})
	}
	parts := map[string][]GeminiPart{
		"text":             {{Text: "committed"}},
		"tool":             {{FunctionCall: &GeminiFunctionCall{Name: "run_command", Args: map[string]interface{}{"command": "true"}}}},
		"image":            {{InlineData: &GeminiInlineData{MimeType: "image/png", Data: "AA=="}}},
		"execution":        {{ExecutableCode: &GeminiPartExecutableCode{Language: "PYTHON", Code: "print(1)"}}},
		"execution-result": {{CodeExecutionResult: &GeminiPartCodeExecutionResult{Output: "1"}}},
	}
	for name, part := range parts {
		t.Run(name, func(t *testing.T) {
			first := preflightEvent(part, "", nil) + preflightEvent(nil, "MALFORMED_FUNCTION_CALL", nil)
			output, calls, _, err := preflightHTTPTest(t, first, true)
			if calls != 1 || err != nil || output != first {
				t.Fatalf("committed output replayed or changed: calls=%d err=%v", calls, err)
			}
		})
	}
	first := preflightEvent(nil, "STOP", nil)
	_, calls, _, err := preflightHTTPTest(t, first, false)
	if calls != 1 || err == nil || err.StatusCode != http.StatusBadGateway {
		t.Fatalf("built-in/non-replayable request retried: calls=%d err=%v", calls, err)
	}
}

type preflightCountingBody struct {
	io.Reader
	closed atomic.Int32
}

func (b *preflightCountingBody) Close() error { b.closed.Add(1); return nil }

func TestGeminiPreflightAlwaysEmptyExactlyTwoAndClose(t *testing.T) {
	var bodies []*preflightCountingBody
	send := func(_ *http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
		body := &preflightCountingBody{Reader: strings.NewReader(preflightEvent(nil, "STOP", &GeminiUsageMetadata{PromptTokenCount: 2, ThoughtsTokenCount: 3, TotalTokenCount: 5}))}
		bodies = append(bodies, body)
		return &http.Response{Body: body, StatusCode: http.StatusOK}, nil
	}
	req, _ := http.NewRequest(http.MethodPost, "http://test.invalid", strings.NewReader("{}"))
	resp, usage, err := preflightGeminiStream(req, context.Background(), send, true, preflightOptions())
	if resp != nil || err == nil || err.StatusCode != 502 || err.Code != "empty_response" || !err.LocalError || len(bodies) != 2 {
		t.Fatalf("resp=%v calls=%d err=%+v", resp, len(bodies), err)
	}
	if usage.PromptTokens != 4 || usage.CompletionTokens != 6 || usage.TotalTokens != 10 {
		t.Fatalf("failed attempt usage was not aggregated: %+v", usage)
	}
	for _, body := range bodies {
		if body.closed.Load() != 1 {
			t.Fatalf("body close count %d", body.closed.Load())
		}
	}
}

func TestGeminiPreflightReplayCloseIdempotent(t *testing.T) {
	body := &preflightCountingBody{Reader: strings.NewReader(preflightSuccess())}
	req, _ := http.NewRequest(http.MethodPost, "http://test.invalid", strings.NewReader("{}"))
	resp, _, err := preflightGeminiStream(req, context.Background(), func(_ *http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
		return &http.Response{Body: body}, nil
	}, true, preflightOptions())
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for i := 0; i < 10; i++ {
		wait.Add(1)
		go func() { defer wait.Done(); resp.Body.Close() }()
	}
	wait.Wait()
	if body.closed.Load() != 1 {
		t.Fatalf("close count=%d", body.closed.Load())
	}
}

func TestGeminiPreflightMemoryBoundCommitsExactBytes(t *testing.T) {
	first := preflightEvent([]GeminiPart{{Thought: true, Text: strings.Repeat("x", 9000)}}, "", nil) + preflightEvent(nil, "STOP", nil)
	prefix, tail, _, err := readGeminiPrelude(strings.NewReader(first), 1024)
	if err != nil || len(prefix) != 1024 {
		t.Fatalf("prefix=%d err=%v", len(prefix), err)
	}
	// Full preflight includes the raw body after the limited reader.
	req, _ := http.NewRequest(http.MethodPost, "http://test.invalid", strings.NewReader("{}"))
	calls := 0
	options := preflightOptions()
	options.maxPreludeBytes = 1024
	resp, _, apiErr := preflightGeminiStream(req, context.Background(), func(_ *http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
		calls++
		return &http.Response{Body: io.NopCloser(strings.NewReader(first))}, nil
	}, true, options)
	if apiErr != nil {
		t.Fatal(apiErr)
	}
	defer resp.Body.Close()
	got, readErr := io.ReadAll(resp.Body)
	if readErr != nil || string(got) != first || calls != 1 || tail == nil {
		t.Fatalf("bounded replay changed data: calls=%d err=%v", calls, readErr)
	}
}

func TestGeminiPreflightCancellationAndTotalDeadline(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				if mode == "deadline" && attempt == 1 {
					time.Sleep(65 * time.Millisecond)
					io.WriteString(w, preflightEvent(nil, "STOP", nil))
					return
				}
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			preflightInstallHTTPClient(t, server.Client())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				go func() { <-started; cancel() }()
			}
			req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
			options := preflightOptions()
			options.totalTimeout = 120 * time.Millisecond
			r := requester.NewHTTPRequester("", nil)
			start := time.Now()
			resp, _, err := preflightGeminiStream(req, ctx, r.SendRequestRaw, true, options)
			elapsed := time.Since(start)
			if resp != nil || err == nil || elapsed > 500*time.Millisecond {
				t.Fatalf("not canceled: elapsed=%s err=%v", elapsed, err)
			}
			if mode == "cancel" && (calls.Load() != 1 || err.Code != "gemini_stream_canceled") {
				t.Fatalf("cancel calls=%d err=%v", calls.Load(), err)
			}
			if mode == "deadline" && (calls.Load() != 2 || err.Code != "gemini_stream_deadline_exceeded" || elapsed > 170*time.Millisecond) {
				t.Fatalf("total deadline reset: elapsed=%s calls=%d err=%v", elapsed, calls.Load(), err)
			}
		})
	}
}

func TestGeminiPreflightPreservesProxyAndCancellation(t *testing.T) {
	key := utils.ProxyHTTPAddrKey
	requestCtx := context.WithValue(context.Background(), key, "http://proxy.invalid")
	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(requestCtx, http.MethodPost, "http://test.invalid", strings.NewReader("{}"))
	resp, _, err := preflightGeminiStream(req, clientCtx, func(r *http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
		if r.Context().Value(key) != "http://proxy.invalid" {
			t.Fatal("proxy context lost")
		}
		if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > 180*time.Second {
			t.Fatal("missing total deadline")
		}
		return &http.Response{Body: io.NopCloser(strings.NewReader(preflightSuccess()))}, nil
	}, true, preflightOptions())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestGeminiPreflightAlreadyCanceledSendsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequest(http.MethodPost, "http://test.invalid", strings.NewReader("{}"))
	calls := 0
	resp, _, err := preflightGeminiStream(req, ctx, func(_ *http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
		calls++
		return &http.Response{Body: io.NopCloser(strings.NewReader(preflightSuccess()))}, nil
	}, true, preflightOptions())
	if calls != 0 || resp != nil || err == nil || err.Code != "gemini_stream_canceled" || !err.LocalError {
		t.Fatalf("already-canceled request escaped: calls=%d resp=%v err=%v", calls, resp, err)
	}
}

func TestGeminiPreflightNoHTTPOrNetworkRetry(t *testing.T) {
	for _, code := range []int{400, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			req, _ := http.NewRequest(http.MethodPost, "http://test.invalid", strings.NewReader("{}"))
			_, _, err := preflightGeminiStream(req, context.Background(), func(_ *http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
				calls++
				return nil, common.ErrorWrapper(errors.New("upstream error"), "http_error", code)
			}, true, preflightOptions())
			if calls != 1 || err == nil || err.StatusCode != code || !err.LocalError {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
	_, calls, _, err := preflightHTTPTest(t, "", true)
	if calls != 1 || err == nil || err.Code == "gemini_stream_canceled" {
		t.Fatalf("EOF misclassified or retried: calls=%d err=%v", calls, err)
	}
}

func TestGeminiPreflightUsageMergeAllNumericFields(t *testing.T) {
	dst, src := &types.Usage{}, &types.Usage{}
	for _, usage := range []*types.Usage{dst, src} {
		value := reflect.ValueOf(usage).Elem()
		for _, name := range []string{"PromptTokensDetails", "CompletionTokensDetails"} {
			detail := value.FieldByName(name)
			for i := 0; i < detail.NumField(); i++ {
				detail.Field(i).SetInt(2)
			}
		}
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens = 2, 2, 4
		usage.ExtraTokens = map[string]int{"extra": 2}
		usage.ExtraBilling = map[string]types.ExtraBilling{"builtin": {Type: "call", CallCount: 2}}
	}
	dst.TextBuilder.WriteString("retained")
	if err := mergeGeminiUsage(dst, src); err != nil {
		t.Fatal(err)
	}
	value := reflect.ValueOf(dst).Elem()
	for _, name := range []string{"PromptTokensDetails", "CompletionTokensDetails"} {
		detail := value.FieldByName(name)
		for i := 0; i < detail.NumField(); i++ {
			if detail.Field(i).Int() != 4 {
				t.Fatalf("lost field %s.%s", name, detail.Type().Field(i).Name)
			}
		}
	}
	if dst.PromptTokens != 4 || dst.CompletionTokens != 4 || dst.TotalTokens != 8 || dst.TextBuilder.String() != "retained" || dst.ExtraTokens["extra"] != 4 || dst.ExtraBilling["builtin"].CallCount != 4 {
		t.Fatalf("bad merged usage: %+v", dst)
	}
}

func TestGeminiPreflightUsageRejectsInvalidSumAtomically(t *testing.T) {
	for _, src := range []*types.Usage{
		{PromptTokens: -1}, {PromptTokens: math.MaxInt},
		{CompletionTokens: math.MaxInt}, {TotalTokens: math.MaxInt},
		{PromptTokensDetails: types.PromptTokensDetails{AudioTokens: math.MaxInt}},
		{ExtraTokens: map[string]int{"extra": math.MaxInt}},
		{ExtraBilling: map[string]types.ExtraBilling{"builtin": {CallCount: math.MaxInt}}},
	} {
		dst := &types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2,
			PromptTokensDetails: types.PromptTokensDetails{AudioTokens: 1},
			ExtraTokens:         map[string]int{"extra": 1}, ExtraBilling: map[string]types.ExtraBilling{"builtin": {CallCount: 1}}}
		before, _ := json.Marshal(dst)
		if err := mergeGeminiUsage(dst, src); err == nil {
			t.Fatal("invalid cumulative usage accepted")
		}
		after, _ := json.Marshal(dst)
		if string(before) != string(after) {
			t.Fatal("failed merge partially mutated usage")
		}
	}
	for _, usage := range []*GeminiUsageMetadata{
		{CandidatesTokenCount: -1, ThoughtsTokenCount: 2},
		{CandidatesTokenCount: math.MaxInt, ThoughtsTokenCount: 1},
		{PromptTokensDetails: []GeminiUsageMetadataDetails{{TokenCount: -1}}},
	} {
		if _, err := geminiStreamUsage(usage); err == nil {
			t.Fatal("invalid raw usage accepted")
		}
	}
}

func TestGeminiPreflightUsageSnapshotsReplaceRatherThanAccumulate(t *testing.T) {
	first := preflightEvent(nil, "", &GeminiUsageMetadata{PromptTokenCount: 11, TotalTokenCount: 11}) +
		preflightEvent(nil, "STOP", &GeminiUsageMetadata{PromptTokenCount: 12, TotalTokenCount: 12})
	_, calls, prior, err := preflightHTTPTest(t, first, true)
	if err != nil || calls != 2 || prior.PromptTokens != 12 {
		t.Fatalf("calls=%d prior=%+v err=%v", calls, prior, err)
	}
	h := &GeminiStreamHandler{Request: &types.ChatCompletionRequest{Model: "fixture"}, Usage: &types.Usage{}, priorUsage: prior}
	data := make(chan string, 10)
	for _, count := range []int{20, 21} {
		if err := h.convertToOpenaiStream(&GeminiChatResponse{UsageMetadata: &GeminiUsageMetadata{PromptTokenCount: count, TotalTokenCount: count}}, data); err != nil {
			t.Fatal(err)
		}
	}
	if h.Usage.PromptTokens != 33 || h.Usage.TotalTokens != 33 {
		t.Fatalf("usage counted more than once: %+v", h.Usage)
	}
}

func TestGeminiPreflightCommittedStreamStillCancels(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, preflightEvent([]GeminiPart{{Text: "visible"}}, "", nil))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(done)
	}))
	defer server.Close()
	preflightInstallHTTPClient(t, server.Client())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
	r := requester.NewHTTPRequester("", nil)
	resp, _, err := preflightGeminiStream(req, ctx, r.SendRequestRaw, true, preflightOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel not propagated after preflight committed")
	}
}

func TestGeminiPreflightDoesNotRetryNonReplayableBodyOrMultipleCandidates(t *testing.T) {
	for _, mode := range []string{"body", "candidates"} {
		t.Run(mode, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "http://fixture.invalid", strings.NewReader("{}"))
			first := preflightEvent(nil, "STOP", nil)
			if mode == "body" {
				req.GetBody = nil
			} else {
				first = "data: {\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"},{\"index\":1,\"finishReason\":\"STOP\"}]}\n\n"
			}
			calls := 0
			resp, _, err := preflightGeminiStream(req, context.Background(), func(*http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode) {
				calls++
				return &http.Response{Body: io.NopCloser(strings.NewReader(first))}, nil
			}, true, preflightOptions())
			if resp != nil {
				resp.Body.Close()
			}
			if calls != 1 || (mode == "body" && err == nil) {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
