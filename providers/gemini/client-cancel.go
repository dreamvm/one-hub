package gemini

import (
	"context"
	"net/http"
	"one-api/types"
	"os"
	"strconv"
	"strings"
)

// Cancellation is independent of empty-response retries. The callback lives only
// until the incoming HTTP request ends, including normal handler completion.
// Keep the outgoing context (proxy values and existing deadlines) as the parent;
// no new timeout, replay, generation parameter, or response buffering is added.
func (p *GeminiProvider) bindClientCancellation(req *http.Request) *http.Request {
	if p.Context == nil || p.Context.Request == nil || p.GetChannel() == nil {
		return req
	}
	enabled := false
	for _, id := range strings.Split(os.Getenv("GEMINI_CANCEL_CHANNELS"), ",") {
		if strings.TrimSpace(id) == strconv.Itoa(p.GetChannel().Id) {
			enabled = true
			break
		}
	}
	if !enabled {
		return req
	}
	incoming := p.Context.Request.Context()
	ctx, cancel := context.WithCancel(req.Context())
	context.AfterFunc(incoming, cancel)
	// AfterFunc is asynchronous; do not issue an already-canceled request.
	if incoming.Err() != nil {
		cancel()
	}
	return req.WithContext(ctx)
}

// A disconnected caller must not trigger failover or channel disablement.
func (p *GeminiProvider) clientCancellationError(err *types.OpenAIErrorWithStatusCode) *types.OpenAIErrorWithStatusCode {
	if err == nil || p.Context == nil || p.Context.Request == nil || p.Context.Request.Context().Err() == nil {
		return err
	}
	for _, id := range strings.Split(os.Getenv("GEMINI_CANCEL_CHANNELS"), ",") {
		if p.GetChannel() != nil && strings.TrimSpace(id) == strconv.Itoa(p.GetChannel().Id) {
			copyErr := *err
			copyErr.LocalError = true
			return &copyErr
		}
	}
	return err
}
