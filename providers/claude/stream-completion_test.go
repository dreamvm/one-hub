package claude_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/claude"
	"one-api/types"
)

func TestClaudeEOFRequiresMessageStop(t *testing.T) {
	h := claude.ClaudeStreamHandler{Request: &types.ChatCompletionRequest{Model: "fixture"}}
	data, errs := make(chan string, 10), make(chan error, 5)
	require.ErrorIs(t, h.EndError(), io.ErrUnexpectedEOF)
	for _, event := range []string{`{"type":"message_start","message":{"id":"fixture"}}`, `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`} {
		b := []byte("data: " + event)
		h.HandlerStream(&b, data, errs)
	}
	require.Empty(t, errs)
	require.ErrorIs(t, h.EndError(), io.ErrUnexpectedEOF)
	b := []byte(`data: {"type":"message_stop"}`)
	h.HandlerStream(&b, data, errs)
	require.ErrorIs(t, <-errs, io.EOF)
	require.NoError(t, h.EndError())
}
