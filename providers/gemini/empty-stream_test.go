package gemini_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/providers/gemini"
	"one-api/types"
)

// Synthetic fixtures only: these tests do not contact a provider or execute a tool.
type emptyResponseFixture struct {
	name       string
	parts      string
	reason     string
	wantError  bool
	wantFinish string
	wantText   string
	wantImage  bool
	wantTool   bool
}

func emptyResponseFixtures() []emptyResponseFixture {
	return []emptyResponseFixture{
		{name: "empty_stop", parts: `[]`, reason: "STOP", wantError: true},
		{name: "empty_part_stop", parts: `[{}]`, reason: "STOP", wantError: true},
		{name: "thought_only_stop", parts: `[{"text":"internal fixture thought","thought":true}]`, reason: "STOP", wantError: true},
		{name: "empty_max_tokens", parts: `[]`, reason: "MAX_TOKENS", wantError: true},
		{name: "thought_only_max_tokens", parts: `[{"text":"internal fixture thought","thought":true}]`, reason: "MAX_TOKENS", wantError: true},
		{name: "text_max_tokens", parts: `[{"text":"中文未完成内容"}]`, reason: "MAX_TOKENS", wantFinish: types.FinishReasonLength, wantText: "中文未完成内容"},
		{name: "malformed_function_call", parts: `[]`, reason: "MALFORMED_FUNCTION_CALL", wantError: true},
		{name: "unexpected_tool_call", parts: `[]`, reason: "UNEXPECTED_TOOL_CALL", wantError: true},
		{name: "missing_thought_signature", parts: `[]`, reason: "MISSING_THOUGHT_SIGNATURE", wantError: true},
		{name: "malformed_response", parts: `[]`, reason: "MALFORMED_RESPONSE", wantError: true},
		{name: "unknown_finish", parts: `[]`, reason: "FUTURE_UNKNOWN_REASON", wantError: true},
		{name: "safety_filter", parts: `[]`, reason: "SAFETY", wantFinish: types.FinishReasonContentFilter},
		{name: "recitation_filter", parts: `[]`, reason: "RECITATION", wantFinish: types.FinishReasonContentFilter},
		{name: "normal_text", parts: `[{"text":"中文完成内容"}]`, reason: "STOP", wantFinish: types.FinishReasonStop, wantText: "中文完成内容"},
		// A real one-pixel PNG avoids network/image-generation dependencies. No
		// storage drive is initialized by this package's unit tests.
		{name: "normal_image", parts: `[{"inlineData":{"mimeType":"image/png","data":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6ZQAAAABJRU5ErkJggg=="}}]`, reason: "STOP", wantFinish: types.FinishReasonStop, wantImage: true},
		{name: "complete_tool", parts: `[{"functionCall":{"id":"fixture-tool","name":"read_file","args":{"path":"fixture.txt"}},"thoughtSignature":"opaque-fixture-signature"}]`, reason: "STOP", wantFinish: types.FinishReasonToolCalls, wantTool: true},
		{name: "tool_rejected_at_finish", parts: `[{"functionCall":{"name":"write_file","args":{"path":"never-execute.txt"}}}]`, reason: "MALFORMED_FUNCTION_CALL", wantError: true},
		{name: "code_execution_result", parts: `[{"codeExecutionResult":{"outcome":"OUTCOME_OK","output":"42"}}]`, reason: "STOP", wantFinish: types.FinishReasonStop, wantText: "42"},
	}
}

func emptyResponseEvent(parts, reason string) string {
	finish := ""
	if reason != "" {
		finish = fmt.Sprintf(`,"finishReason":%q`, reason)
	}
	return fmt.Sprintf(`{"responseId":"fixture-empty-guard","candidates":[{"index":0,"content":{"role":"model","parts":%s}%s}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0,"thoughtsTokenCount":8141,"totalTokenCount":8151}}`, parts, finish)
}

func runEmptyResponseStream(t *testing.T, events ...string) ([]types.ChatCompletionStreamChoice, []error) {
	t.Helper()
	h := gemini.GeminiStreamHandler{
		Request: &types.ChatCompletionRequest{Model: "gemini-test-fixture"},
		Usage:   &types.Usage{},
	}
	data, errs := make(chan string, 64), make(chan error, 16)
	for _, event := range events {
		line := []byte("data: " + event)
		h.HandlerStream(&line, data, errs)
		if string(line) == "stream_closed" {
			break
		}
	}
	var resultErrors []error
	for len(errs) > 0 {
		err := <-errs
		var payload struct {
			Error json.RawMessage `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(err.Error()), &payload), "stream failures must be valid OpenAI-compatible error JSON")
		require.NotEmpty(t, payload.Error)
		resultErrors = append(resultErrors, err)
	}
	if len(resultErrors) == 0 {
		if err := h.EndError(); err != nil {
			resultErrors = append(resultErrors, err)
		}
	}
	var choices []types.ChatCompletionStreamChoice
	for len(data) > 0 {
		var chunk types.ChatCompletionStreamResponse
		require.NoError(t, json.Unmarshal([]byte(<-data), &chunk))
		choices = append(choices, chunk.Choices...)
	}
	return choices, resultErrors
}

func TestGeminiEmptyResponseStreamClassification(t *testing.T) {
	for _, fixture := range emptyResponseFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			choices, errs := runEmptyResponseStream(t, emptyResponseEvent(fixture.parts, fixture.reason))
			if fixture.wantError {
				require.NotEmpty(t, errs, "a response without a deliverable must not silently complete successfully")
				for _, choice := range choices {
					require.Nil(t, choice.FinishReason, "a failed response must not emit a successful final choice")
					require.Empty(t, choice.Delta.ToolCalls, "a rejected function call must never reach the tool executor")
				}
				return
			}
			require.Empty(t, errs)
			var text strings.Builder
			var finishes []any
			var imageCount, namedTools int
			for _, choice := range choices {
				text.WriteString(choice.Delta.Content)
				imageCount += len(choice.Delta.Image)
				if choice.FinishReason != nil {
					finishes = append(finishes, choice.FinishReason)
				}
				for _, call := range choice.Delta.ToolCalls {
					if call.Function != nil && call.Function.Name != "" {
						namedTools++
						require.Equal(t, "read_file", call.Function.Name)
					}
				}
			}
			require.Equal(t, []any{fixture.wantFinish}, finishes)
			if fixture.wantText != "" {
				require.Contains(t, text.String(), fixture.wantText)
			}
			if fixture.wantImage {
				require.Equal(t, 1, imageCount)
			}
			if fixture.wantTool {
				require.Equal(t, 1, namedTools)
			}
		})
	}
}

func TestGeminiEmptyResponseGuardRemembersEarlierDeliverable(t *testing.T) {
	for _, reason := range []string{"STOP", "MAX_TOKENS"} {
		t.Run(reason, func(t *testing.T) {
			choices, errs := runEmptyResponseStream(t,
				emptyResponseEvent(`[{"text":"already delivered"}]`, ""),
				emptyResponseEvent(`[]`, reason),
			)
			require.Empty(t, errs, "an empty terminal chunk must not erase earlier valid output")
			require.Len(t, choices, 2)
			require.Equal(t, "already delivered", choices[0].Delta.Content)
			require.Equal(t, gemini.ConvertFinishReason(reason), choices[1].FinishReason)
		})
	}
}

func TestGeminiEmptyResponseGuardNeverEmitsRejectedBufferedTool(t *testing.T) {
	choices, errs := runEmptyResponseStream(t,
		emptyResponseEvent(`[{"functionCall":{"name":"write_file","args":{"path":"never-execute.txt"}}}]`, ""),
		emptyResponseEvent(`[]`, "MALFORMED_FUNCTION_CALL"),
	)
	require.NotEmpty(t, errs)
	for _, choice := range choices {
		require.Empty(t, choice.Delta.ToolCalls)
		require.Nil(t, choice.FinishReason)
	}
}
