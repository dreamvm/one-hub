package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"one-api/common"
	"one-api/types"
)

const geminiEmptyRetryChannelsEnv = "GEMINI_EMPTY_RESPONSE_RETRY_CHANNELS"

type geminiPreflightOptions struct {
	maxPreludeBytes int
	totalTimeout    time.Duration
}

func defaultGeminiPreflightOptions() geminiPreflightOptions {
	return geminiPreflightOptions{maxPreludeBytes: 1 << 20, totalTimeout: 180 * time.Second}
}

func geminiEmptyRetryEnabled(channelID int) bool {
	if channelID <= 0 {
		return false
	}
	for _, field := range strings.Split(os.Getenv(geminiEmptyRetryChannelsEnv), ",") {
		id, err := strconv.Atoi(strings.TrimSpace(field))
		if err == nil && id == channelID {
			return true
		}
	}
	return false
}

func geminiRequestHasBuiltins(request *GeminiChatRequest) bool {
	for _, tool := range request.Tools {
		if tool.CodeExecution != nil || tool.GoogleSearch != nil || tool.GoogleSearchRetrieval != nil || tool.UrlContext != nil {
			return true
		}
	}
	return false
}

type geminiRawSender func(*http.Request) (*http.Response, *types.OpenAIErrorWithStatusCode)

// preflightGeminiStream does not start the shared stream reader. It consumes a
// bounded raw SSE prefix synchronously and replays that exact prefix once the
// request is committed. No downstream bytes or external tool calls can escape
// from a discarded attempt. The second request uses identical serialized bytes.
func preflightGeminiStream(req *http.Request, clientCtx context.Context, send geminiRawSender, allowRetry bool, options geminiPreflightOptions) (*http.Response, *types.Usage, *types.OpenAIErrorWithStatusCode) {
	if options.maxPreludeBytes <= 0 || options.maxPreludeBytes > 1<<20 {
		options.maxPreludeBytes = 1 << 20
	}
	if options.totalTimeout <= 0 || options.totalTimeout > 180*time.Second {
		options.totalTimeout = 180 * time.Second
	}
	// Starting with the request context preserves proxy routing values. Binding
	// the incoming request separately adds cancellation missing in the legacy
	// request builder without replacing its existing context values.
	deadline := time.Now().Add(options.totalTimeout)
	if clientDeadline, ok := clientCtx.Deadline(); ok && clientDeadline.Before(deadline) {
		deadline = clientDeadline
	}
	ctx, cancel := context.WithDeadline(req.Context(), deadline)
	stopClient := context.AfterFunc(clientCtx, cancel)
	var cleanupOnce sync.Once
	cleanup := func() { cleanupOnce.Do(func() { stopClient(); cancel() }) }
	prior := &types.Usage{}
	if req.GetBody == nil {
		allowRetry = false
	}
	for attempt := 0; attempt < 2; attempt++ {
		if clientCtx.Err() != nil {
			cancel()
		}
		if err := ctx.Err(); err != nil {
			cleanup()
			return nil, prior, geminiPreflightError(err)
		}
		attemptReq := req.Clone(ctx)
		if req.GetBody != nil {
			var err error
			attemptReq.Body, err = req.GetBody()
			if err != nil {
				cleanup()
				return nil, prior, geminiPreflightError(err)
			}
		}
		resp, sendErr := send(attemptReq)
		if attemptReq.Body != nil {
			attemptReq.Body.Close()
		}
		if sendErr != nil {
			contextErr := ctx.Err()
			cleanup()
			if contextErr != nil {
				return nil, prior, geminiPreflightError(contextErr)
			}
			// No generic relay replay may escape this channel-specific bound,
			// including when the token did not explicitly pin this channel.
			boundedErr := *sendErr
			boundedErr.LocalError = true
			return nil, prior, &boundedErr
		}
		body := newGeminiOwnedBody(resp.Body)
		stopBody := context.AfterFunc(ctx, func() { body.Close() })
		prefix, tail, usage, err := readGeminiPrelude(body, options.maxPreludeBytes)
		if err == nil {
			resp.Body = &geminiReplayBody{
				Reader: io.MultiReader(bytes.NewReader(prefix), tail, body), body: body,
				cleanup: func() { stopBody(); cleanup() },
			}
			return resp, prior, nil
		}
		stopBody()
		body.Close()
		if usageErr := mergeGeminiUsage(prior, usage); usageErr != nil {
			err = usageErr
		}
		var candidateErr *CandidateResponseError
		if attempt == 0 && allowRetry && errors.As(err, &candidateErr) && candidateErr.Retryable() && ctx.Err() == nil {
			continue
		}
		contextErr := ctx.Err()
		cleanup()
		if contextErr != nil && !errors.As(err, &candidateErr) {
			err = contextErr
		}
		return nil, prior, geminiPreflightError(err)
	}
	panic("Gemini preflight exceeded its two-attempt bound")
}

func geminiPreflightError(err error) *types.OpenAIErrorWithStatusCode {
	code := "gemini_stream_preflight_failed"
	message := errors.New("Gemini response could not be completed before streaming")
	var candidateErr *CandidateResponseError
	if errors.As(err, &candidateErr) {
		code = candidateErr.Code()
		message = candidateErr
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "gemini_stream_deadline_exceeded"
		message = errors.New("Gemini response exceeded the total stream deadline")
	} else if errors.Is(err, context.Canceled) {
		code = "gemini_stream_canceled"
		message = errors.New("Gemini response canceled")
	}
	// LocalError prevents the generic relay from replaying the bounded attempts.
	return common.ErrorWrapperLocal(message, code, http.StatusBadGateway)
}

// readGeminiPrelude only regards empty STOP and malformed empty calls as retry
// candidates. Network errors, invalid events, content filters, and token limits
// are never retried. A tool, visible text, image, or built-in part commits the
// attempt immediately, even when that same event later reports a failure.
func readGeminiPrelude(body io.Reader, limit int) ([]byte, io.Reader, *types.Usage, error) {
	limited := &io.LimitedReader{R: body, N: int64(limit)}
	reader := bufio.NewReaderSize(limited, 4096)
	prefix := make([]byte, 0, min(limit, 4096))
	usage := &types.Usage{}
	lineStart := 0
	var terminalErr error
	for {
		fragment, readErr := reader.ReadSlice('\n')
		prefix = append(prefix, fragment...)
		if len(prefix) >= limit {
			// At the memory cap, commit the prefix rather than retrying or buffering
			// more. reader still owns any prefetched bytes; body owns the rest.
			return prefix, reader, usage, nil
		}
		if readErr == bufio.ErrBufferFull {
			continue
		}
		line := bytes.TrimSpace(prefix[lineStart:])
		lineStart = len(prefix)
		if bytes.HasPrefix(line, []byte("data:")) {
			var response GeminiChatResponse
			if json.Unmarshal(bytes.TrimSpace(line[5:]), &response) != nil || response.ErrorInfo != nil {
				// Preserve the existing provider's error parsing and sanitization.
				return prefix, reader, usage, nil
			}
			if response.UsageMetadata != nil {
				converted, err := geminiStreamUsage(response.UsageMetadata)
				if err != nil {
					return nil, nil, usage, err
				}
				usage = &converted // cumulative snapshots replace, never sum
			}
			if response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "" {
				return prefix, reader, usage, nil
			}
			if terminalErr != nil && len(response.Candidates) > 0 {
				// Only usage-only events may follow an empty terminal event. An
				// unexpected later candidate must be validated without replay.
				return prefix, reader, usage, nil
			}
			for _, candidate := range response.Candidates {
				if hasCandidateDeliverable(candidate.Content.Parts) || candidate.GroundingMetadata != nil {
					return prefix, reader, usage, nil
				}
			}
			// Multiple candidates are conservatively committed rather than risking
			// replay when a later candidate might still produce a deliverable.
			if len(response.Candidates) > 1 {
				return prefix, reader, usage, nil
			}
			if len(response.Candidates) == 1 && response.Candidates[0].FinishReason != nil {
				if err := validateCandidateCompletion(response.Candidates[0], false, false); err != nil {
					var candidateErr *CandidateResponseError
					if !errors.As(err, &candidateErr) || !candidateErr.Retryable() {
						return nil, nil, usage, err
					}
					terminalErr = err
				} else {
					return prefix, reader, usage, nil
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF && terminalErr != nil {
				// The final cumulative snapshot can arrive after STOP. Keep the
				// same memory/deadline bounds while reading it before discarding.
				return nil, nil, usage, terminalErr
			}
			return nil, nil, usage, readErr
		}
	}
}

// The body is owned independently of replay buffering so cancellation, discard,
// and downstream Close close the actual socket at most once.
type geminiOwnedBody struct {
	io.ReadCloser
	once sync.Once
	err  error
}

func newGeminiOwnedBody(body io.ReadCloser) *geminiOwnedBody {
	return &geminiOwnedBody{ReadCloser: body}
}

func (b *geminiOwnedBody) Close() error {
	b.once.Do(func() { b.err = b.ReadCloser.Close() })
	return b.err
}

type geminiReplayBody struct {
	io.Reader
	body    *geminiOwnedBody
	cleanup func()
	once    sync.Once
}

func (b *geminiReplayBody) Close() error {
	b.once.Do(b.cleanup)
	return b.body.Close()
}

func geminiStreamUsage(metadata *GeminiUsageMetadata) (types.Usage, error) {
	if metadata == nil {
		return types.Usage{}, nil
	}
	for _, n := range []int{metadata.PromptTokenCount, metadata.CandidatesTokenCount, metadata.TotalTokenCount,
		metadata.ThoughtsTokenCount, metadata.CachedContentTokenCount, metadata.ToolUsePromptTokenCount} {
		if n < 0 {
			return types.Usage{}, errors.New("negative Gemini usage")
		}
	}
	for _, details := range [][]GeminiUsageMetadataDetails{metadata.PromptTokensDetails, metadata.CandidatesTokensDetails} {
		for _, detail := range details {
			if detail.TokenCount < 0 {
				return types.Usage{}, errors.New("negative Gemini usage detail")
			}
		}
	}
	if metadata.ThoughtsTokenCount > math.MaxInt-metadata.CandidatesTokenCount {
		return types.Usage{}, errors.New("Gemini completion usage overflow")
	}
	usage := ConvertOpenAIUsage(metadata)
	return usage, usage.Validate()
}

// Add the final cumulative snapshot from each discarded attempt exactly once.
// Validate before mutation and keep the caller's TextBuilder in place.
func mergeGeminiUsage(dst, src *types.Usage) error {
	if src == nil || dst == nil {
		return nil
	}
	if err := src.Validate(); err != nil {
		return err
	}
	if err := dst.Validate(); err != nil {
		return err
	}
	fields := func(u *types.Usage) []*int {
		p, c := &u.PromptTokensDetails, &u.CompletionTokensDetails
		return []*int{&u.PromptTokens, &u.CompletionTokens, &u.TotalTokens,
			&p.AudioTokens, &p.CachedTokens, &p.TextTokens, &p.ImageTokens,
			&p.CachedTokensInternal, &p.CachedWriteTokens, &p.CachedReadTokens,
			&c.AudioTokens, &c.TextTokens, &c.ReasoningTokens,
			&c.AcceptedPredictionTokens, &c.RejectedPredictionTokens, &c.ImageTokens}
	}
	left, right := fields(dst), fields(src)
	for i := range left {
		if *right[i] > math.MaxInt-*left[i] {
			return errors.New("Gemini usage sum overflow")
		}
	}
	if dst.PromptTokens+src.PromptTokens > math.MaxInt-(dst.CompletionTokens+src.CompletionTokens) {
		return errors.New("Gemini total usage overflow")
	}
	for key, value := range src.ExtraTokens {
		if value > math.MaxInt-dst.ExtraTokens[key] {
			return errors.New("Gemini extra usage overflow")
		}
	}
	for key, value := range src.ExtraBilling {
		if value.CallCount > math.MaxInt-dst.ExtraBilling[key].CallCount {
			return errors.New("Gemini extra billing overflow")
		}
	}
	for i := range left {
		*left[i] += *right[i]
	}
	for key, value := range src.ExtraTokens {
		if dst.ExtraTokens == nil {
			dst.ExtraTokens = make(map[string]int)
		}
		dst.ExtraTokens[key] += value
	}
	for key, value := range src.ExtraBilling {
		if dst.ExtraBilling == nil {
			dst.ExtraBilling = make(map[string]types.ExtraBilling)
		}
		billing := dst.ExtraBilling[key]
		billing.Type = value.Type
		billing.CallCount += value.CallCount
		dst.ExtraBilling[key] = billing
	}
	return nil
}
