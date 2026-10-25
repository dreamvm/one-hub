package relay_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkoukk/tiktoken-go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/base"
	"one-api/providers/gemini"
	"one-api/providers/openai"
	"one-api/relay"
	"one-api/types"
)

// Synthetic byte ranks keep this a strictly offline dataflow test. This is
// NOT a claim that our counts equal either model's real tokenizer or invoice.
type meteringByteRanks struct{}

func (meteringByteRanks) LoadTiktokenBpe(string) (map[string]int, error) {
	m := make(map[string]int, 256)
	for i := 0; i < 256; i++ {
		m[string([]byte{byte(i)})] = i
	}
	return m, nil
}

type meteringRoundTrip func(*http.Request) (*http.Response, error)

func (f meteringRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A malformed SSE handler sends its parse error and then the real reader sends
// EOF. Drain that EOF before restoring shared fixture globals; otherwise the
// reader's undelivered-error timeout may log during the next test fixture.
type meteringErrorDrainProvider struct {
	*openai.OpenAIProvider
	t *testing.T
}

func (p *meteringErrorDrainProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	stream, err := p.OpenAIProvider.CreateChatCompletionStream(request)
	if err != nil {
		return stream, err
	}
	return &meteringErrorDrainStream{StreamReaderInterface: stream, t: p.t}, nil
}

type meteringErrorDrainStream struct {
	requester.StreamReaderInterface[string]
	t      *testing.T
	errors <-chan error
}

func (s *meteringErrorDrainStream) Recv() (<-chan string, <-chan error) {
	data, errors := s.StreamReaderInterface.Recv()
	s.errors = errors
	return data, errors
}

func (s *meteringErrorDrainStream) Close() {
	defer s.StreamReaderInterface.Close()
	select {
	case err := <-s.errors:
		require.ErrorIs(s.t, err, io.EOF, "drain the actual reader's terminal EOF")
	case <-time.After(2 * time.Second):
		s.t.Fatal("malformed-stream fixture did not finish")
	}
}

var meteringEncoderOnce sync.Once

func meteringEncoders(t *testing.T) {
	t.Helper()
	oldDisabled, oldApprox := config.DisableTokenEncoders, config.ApproximateTokenEnabled
	oldOption := viper.Get("disable_token_encoders")
	viper.Set("disable_token_encoders", false)
	meteringEncoderOnce.Do(func() {
		tiktoken.SetBpeLoader(meteringByteRanks{})
		common.InitTokenEncoders()
	})
	config.DisableTokenEncoders, config.ApproximateTokenEnabled = false, false
	t.Cleanup(func() {
		config.DisableTokenEncoders, config.ApproximateTokenEnabled = oldDisabled, oldApprox
		viper.Set("disable_token_encoders", oldOption)
		tiktoken.SetBpeLoader(tiktoken.NewDefaultBpeLoader())
	})
}

func meteringProvider(t *testing.T, name string, native bool, body string, code int) (base.ProviderInterface, *types.ChatCompletionRequest, func(int, int), *int) {
	t.Helper()
	db, c, _, request := searchQuotaFixture(t, false)
	meteringEncoders(t)
	request.Model = name
	ratios := datatypes.NewJSONType(map[string]float64{config.UsageExtraCache: 0.25, config.UsageExtraReasoning: 1})
	model.PricingInstance.Prices[name] = &model.Price{Type: model.TokensPriceType, Input: 1, Output: 2, ExtraRatios: &ratios}
	url, proxy := "https://synthetic-metering.invalid", ""
	channel := &model.Channel{Id: 1, Type: config.ChannelTypeOpenAI, BaseURL: &url, Proxy: &proxy, Key: "synthetic-only"}
	var provider base.ProviderInterface
	if native {
		provider = (&gemini.GeminiProviderFactory{}).Create(channel)
	} else {
		provider = openai.CreateOpenAIProvider(channel, url)
	}
	provider.SetContext(c)
	calls := new(int)
	oldClient := requester.HTTPClient
	requester.HTTPClient = &http.Client{Transport: meteringRoundTrip(func(r *http.Request) (*http.Response, error) {
		*calls++
		if r.URL.Host != "synthetic-metering.invalid" {
			t.Fatalf("unexpected upstream destination %s", r.URL.Host)
		}
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	t.Cleanup(func() { requester.HTTPClient = oldClient })
	check := func(spent, records int) {
		t.Helper()
		searchQuotaBalances(t, db, false, spent, records)
	}
	return provider, request, check, calls
}

func meteringOpenAIBody(text string, usage any, stream bool) string {
	message := map[string]any{"id": "synthetic", "object": "chat.completion", "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}}
	if usage != nil {
		message["usage"] = usage
	}
	if !stream {
		b, _ := json.Marshal(message)
		return string(b)
	}
	chunk := map[string]any{"id": "synthetic", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": text}}}}
	b, _ := json.Marshal(chunk)
	out := "data: " + string(b) + "\n\n"
	if usage != nil {
		b, _ := json.Marshal(map[string]any{"choices": []any{}, "usage": usage})
		out += "data: " + string(b) + "\n\n"
	}
	return out + "data: [DONE]\n\n"
}

func meteringGeminiBody(text string, usage any, stream bool) string {
	message := map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"role": "model", "parts": []any{map[string]any{"text": text}}}, "finishReason": "STOP"}}}
	if usage != nil {
		message["usageMetadata"] = usage
	}
	b, _ := json.Marshal(message)
	if stream {
		return "data: " + string(b) + "\n\n"
	}
	return string(b)
}

func TestRC10MeteringEncoderFallbackAndReportedUsage(t *testing.T) {
	for _, name := range []string{"gpt-6-luna", "gemini-3.8-flash"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", name, stream), func(t *testing.T) {
				usage := &types.Usage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110,
					PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 80}, CompletionTokensDetails: types.CompletionTokensDetails{ReasoningTokens: 3}}
				p, request, check, calls := meteringProvider(t, name, false, meteringOpenAIBody("synthetic answer", usage, stream), 200)
				request.Stream = stream
				_, err := tiktoken.EncodingForModel(name)
				require.Error(t, err)
				require.Same(t, common.GetTokenEncoder("gpt-3.5-turbo"), common.GetTokenEncoder(name))
				estimate := common.CountTokenMessages(request.Messages, name, 0)
				require.NotEqual(t, 100, estimate)
				require.Nil(t, relay.ExecuteChatForTest(p.(*openai.OpenAIProvider).Context, p, request))
				require.Equal(t, 1, *calls)
				// 100 input - 80 cached + 80*0.25 + 10 output*2 = 60.
				check(60, 1)
				t.Logf("estimate=%d, upstream=100/10, settled=60 synthetic quota units", estimate)
			})
		}
	}
}

func TestRC10MeteringOpenAIMissingUsageEstimatesFinalFee(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			answer := "synthetic answer"
			p, request, check, calls := meteringProvider(t, "gpt-6-luna", false, meteringOpenAIBody(answer, nil, stream), 200)
			request.Stream = stream
			input := common.CountTokenMessages(request.Messages, request.Model, 0)
			output := common.CountTokenText(answer, request.Model)
			require.Nil(t, relay.ExecuteChatForTest(p.(*openai.OpenAIProvider).Context, p, request))
			require.Equal(t, 1, *calls)
			check(input+2*output, 1)
			t.Logf("no upstream usage: estimate becomes final fee, input=%d/output=%d", input, output)
		})
	}
}

func TestRC10MeteringOpenAIReportedZeroCompletionMustKeepPrompt(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			usage := &types.Usage{PromptTokens: 100, TotalTokens: 100, PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 80}}
			p, request, check, _ := meteringProvider(t, "gpt-6-luna", false, meteringOpenAIBody("", usage, stream), 200)
			request.Stream = stream
			require.Nil(t, relay.ExecuteChatForTest(p.(*openai.OpenAIProvider).Context, p, request))
			// Authoritative nonzero input usage with zero completion must not be
			// replaced by a local estimate, nor lose its cache details.
			check(40, 1)
		})
	}
}

func TestRC10MeteringGeminiCacheAndThoughtsReachSettlement(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			usage := map[string]int{"promptTokenCount": 100, "cachedContentTokenCount": 80, "candidatesTokenCount": 10, "thoughtsTokenCount": 5, "totalTokenCount": 115}
			p, request, check, calls := meteringProvider(t, "gemini-3.8-flash", true, meteringGeminiBody("synthetic answer", usage, stream), 200)
			request.Stream = stream
			require.Nil(t, relay.ExecuteChatForTest(p.(*gemini.GeminiProvider).Context, p, request))
			require.Equal(t, 1, *calls)
			require.Equal(t, 15, p.GetUsage().CompletionTokens, "thoughts included exactly once")
			check(70, 1)
		})
	}
}

func TestRC10MeteringGeminiMissingUsageMustKeepInputEstimate(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			answer := "synthetic answer"
			p, request, check, _ := meteringProvider(t, "gemini-3.8-flash", true, meteringGeminiBody(answer, nil, stream), 200)
			request.Stream = stream
			input := common.CountTokenMessages(request.Messages, request.Model, 0)
			output := common.CountTokenText(answer, request.Model)
			require.Nil(t, relay.ExecuteChatForTest(p.(*gemini.GeminiProvider).Context, p, request))
			check(input+2*output, 1)
		})
	}
}

func TestRC10MeteringUpstreamFailureReleasesReservation(t *testing.T) {
	for _, name := range []string{"gpt-6-luna", "gemini-3.8-flash"} {
		t.Run(name, func(t *testing.T) {
			p, request, check, calls := meteringProvider(t, name, false, `{"error":{"message":"synthetic failure","type":"fixture"}}`, 500)
			require.NotNil(t, relay.ExecuteChatForTest(p.(*openai.OpenAIProvider).Context, p, request))
			require.Equal(t, 1, *calls)
			check(0, 0)
		})
	}
}

func TestRC10MeteringResponsesReportedZeroOutput(t *testing.T) {
	body := `{"id":"synthetic","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":0,"total_tokens":100,"input_tokens_details":{"cached_tokens":80}}}`
	p, _, check, calls := meteringProvider(t, "gpt-6-luna", false, body, 200)
	request := &types.OpenAIResponsesRequest{Model: "gpt-6-luna", Input: strings.Repeat("synthetic ", 10)}
	require.Nil(t, relay.ExecuteResponsesForMeteringTest(p.(*openai.OpenAIProvider).Context, p, request))
	require.Equal(t, 1, *calls)
	check(40, 1)
}

func TestRC10MeteringRefundThenRetryAttemptDoesNotDoubleCharge(t *testing.T) {
	p, request, check, _ := meteringProvider(t, "gpt-6-luna", false, "", 200)
	calls := 0
	requester.HTTPClient = &http.Client{Transport: meteringRoundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		code, body := 500, `{"error":{"message":"synthetic failure","type":"fixture"}}`
		if calls == 2 {
			code = 200
			body = meteringOpenAIBody("synthetic answer", &types.Usage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110}, false)
		}
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}
	c := p.(*openai.OpenAIProvider).Context
	require.NotNil(t, relay.ExecuteChatForTest(c, p, request))
	check(0, 0)
	require.Nil(t, relay.ExecuteChatForTest(c, p, request))
	check(120, 1)
	require.Equal(t, 2, calls, "two ordinary attempt cores, not a claim of routing/retry selection coverage")
}

func TestRC10MeteringStreamFailureAfterTextUsesExistingPartialSettlement(t *testing.T) {
	answer := "synthetic partial"
	body := strings.TrimSuffix(meteringOpenAIBody(answer, nil, true), "data: [DONE]\n\n") + "data: {invalid\n\n"
	p, request, check, calls := meteringProvider(t, "gpt-6-luna", false, body, 200)
	request.Stream = true
	want := common.CountTokenMessages(request.Messages, request.Model, 0) + 2*common.CountTokenText(answer, request.Model)
	// RC10 responseStreamClient reports the stream error downstream but returns
	// nil to the quota layer, which bills accepted partial work. Preserve policy.
	actual := p.(*openai.OpenAIProvider)
	observed := &meteringErrorDrainProvider{OpenAIProvider: actual, t: t}
	require.Nil(t, relay.ExecuteChatForTest(actual.Context, observed, request))
	require.Equal(t, 1, *calls)
	check(want, 1)
}

func TestRC10MeteringAllZeroUsageRetainsExistingFallback(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			answer := "synthetic answer"
			p, request, check, _ := meteringProvider(t, "gpt-6-luna", false, meteringOpenAIBody(answer, &types.Usage{}, stream), 200)
			request.Stream = stream
			want := common.CountTokenMessages(request.Messages, request.Model, 0) + 2*common.CountTokenText(answer, request.Model)
			require.Nil(t, relay.ExecuteChatForTest(p.(*openai.OpenAIProvider).Context, p, request))
			check(want, 1)
		})
	}
}

func TestRC10MeteringGeminiDefaultCacheRatioIsNeutral(t *testing.T) {
	usage := map[string]int{"promptTokenCount": 100, "cachedContentTokenCount": 80, "candidatesTokenCount": 10, "thoughtsTokenCount": 5, "totalTokenCount": 115}
	p, request, check, _ := meteringProvider(t, "gemini-3.8-flash", true, meteringGeminiBody("synthetic answer", usage, false), 200)
	model.PricingInstance.Prices[request.Model].ExtraRatios = nil
	require.Nil(t, relay.ExecuteChatForTest(p.(*gemini.GeminiProvider).Context, p, request))
	check(130, 1)
}

func TestRC10MeteringResponsesPositiveAndMissingControls(t *testing.T) {
	for _, reported := range []bool{true, false} {
		t.Run(fmt.Sprint(reported), func(t *testing.T) {
			answer := "synthetic answer"
			message := map[string]any{"id": "synthetic", "status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": answer}}}}}
			if reported {
				message["usage"] = map[string]any{"input_tokens": 100, "output_tokens": 10, "total_tokens": 110, "input_tokens_details": map[string]int{"cached_tokens": 80}}
			}
			body, err := json.Marshal(message)
			require.NoError(t, err)
			p, _, check, _ := meteringProvider(t, "gpt-6-luna", false, string(body), 200)
			request := &types.OpenAIResponsesRequest{Model: "gpt-6-luna", Input: strings.Repeat("synthetic ", 10)}
			want := 60
			if !reported {
				want = common.CountTokenInputMessages(request.Input, request.Model, 0) + 2*common.CountTokenText(answer, request.Model)
			}
			require.Nil(t, relay.ExecuteResponsesForMeteringTest(p.(*openai.OpenAIProvider).Context, p, request))
			check(want, 1)
		})
	}
}

func TestMeteringInputOnlyUsageRetainsStreamText(t *testing.T) {
	for _, placement := range []string{"top", "choice"} {
		for _, mode := range []string{"input-only-after-text", "positive-after-text", "input-only-empty", "input-only-mid-text"} {
			t.Run(fmt.Sprintf("%s/%s", placement, mode), func(t *testing.T) {
				answer := "synthetic answer"
				completion := 0
				if mode == "positive-after-text" {
					completion = 10
				}
				if mode == "input-only-empty" {
					answer = ""
				}
				usage := &types.Usage{
					PromptTokens:        100,
					CompletionTokens:    completion,
					TotalTokens:         100 + completion,
					PromptTokensDetails: types.PromptTokensDetails{CachedTokens: 80},
				}
				body := meteringOpenAIBody(answer, usage, true)
				if placement == "choice" {
					prefix := strings.TrimSuffix(meteringOpenAIBody(answer, nil, true), "data: [DONE]\n\n")
					chunk, err := json.Marshal(map[string]any{
						"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "usage": usage}},
					})
					require.NoError(t, err)
					body = prefix + "data: " + string(chunk) + "\n\ndata: [DONE]\n\n"
				}
				if mode == "input-only-mid-text" {
					body = strings.TrimSuffix(body, "data: [DONE]\n\n")
					// A later content frame must remain writable after usage updates.
					body += meteringOpenAIBody(" tail", nil, true)
					answer += " tail"
				}
				provider, request, check, calls := meteringProvider(t, "gpt-6-luna", false, body, 200)
				request.Stream = true
				output := completion
				if output == 0 && answer != "" {
					output = common.CountTokenText(answer, request.Model)
				}
				actual := provider.(*openai.OpenAIProvider)
				require.Nil(t, relay.ExecuteChatForTest(actual.Context, provider, request))
				require.Equal(t, 1, *calls)
				t.Logf("reported input=100 cached=80 completion=%d; fixture text bytes=%d; expected quota=%d; actual completion=%d accumulated bytes=%d",
					completion, len(answer), 40+2*output, actual.Usage.CompletionTokens, actual.Usage.TextBuilder.Len())
				check(40+2*output, 1)
			})
		}
	}
}
