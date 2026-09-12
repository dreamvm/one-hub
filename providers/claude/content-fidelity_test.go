package claude_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/base"
	"one-api/providers/claude"
	"one-api/types"
)

func TestClaudeOpaqueContentPreserved(t *testing.T) {
	for _, raw := range []string{
		`{"type":"thinking","thinking":"","signature":"opaque","future_field":{"v":1}}`,
		`{"type":"text","text":"","citations":[]}`,
		`{"type":"tool_use","id":"a","name":"read","input":{},"caller":{"type":"direct"}}`,
		`{"type":"redacted_thinking","data":"opaque","future_field":true}`,
	} {
		var block claude.ResContent
		require.NoError(t, json.Unmarshal([]byte(raw), &block))
		wire, err := json.Marshal(block)
		require.NoError(t, err)
		require.JSONEq(t, raw, string(wire))
	}
}

func TestClaudeOpaqueContentDoesNotReplayStaleEdits(t *testing.T) {
	var block claude.ResContent
	require.NoError(t, json.Unmarshal([]byte(`{"type":"text","text":"original","future_field":true}`), &block))
	block.Text = "changed"
	encoded, err := json.Marshal(block)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "changed")
	require.NotContains(t, string(encoded), "original")
}

func TestClaudeEmptyThinkingSurvivesCompatibleRoundtrip(t *testing.T) {
	const raw = `{"id":"fixture","role":"assistant","stop_reason":"tool_use","usage":{"input_tokens":2,"output_tokens":3},"content":[{"type":"thinking","thinking":"","signature":"opaque"},{"type":"tool_use","id":"a","name":"read","input":{},"caller":{"type":"direct"}}]}`
	var response claude.ClaudeResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &response))
	p := &claude.ClaudeProvider{BaseProvider: base.BaseProvider{Usage: &types.Usage{}}}
	r := &types.ChatCompletionRequest{Model: "fixture", MaxTokens: 4096}
	out, e := claude.ConvertToChatOpenai(p, &response, r)
	require.Nil(t, e)
	r.Messages = []types.ChatCompletionMessage{out.Choices[0].Message, {Role: "tool", ToolCallID: "a", Content: "ok"}}
	next, e := claude.ConvertFromChatOpenai(r)
	require.Nil(t, e)
	wire, err := json.Marshal(next.Messages[0].Content)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"thinking","thinking":"","signature":"opaque"},{"type":"tool_use","id":"a","name":"read","input":{},"caller":{"type":"direct"}}]`, string(wire))
}
