package gemini

import (
	"fmt"
	"strings"
)

// CandidateResponseError classifies an upstream completion without retaining
// prompts, thoughts, tool arguments, signatures, or the free-form finishMessage.
// Retryable describes the candidate only. A caller must also prove that no
// response has reached the client before retrying the request.
type CandidateResponseError struct {
	reason    string
	code      string
	retryable bool
}

func (e *CandidateResponseError) Error() string {
	return fmt.Sprintf("Gemini returned an unusable response (code=%s, finish_reason=%s)", e.code, e.reason)
}

func (e *CandidateResponseError) Reason() string  { return e.reason }
func (e *CandidateResponseError) Code() string    { return e.code }
func (e *CandidateResponseError) Retryable() bool { return e.retryable }

func candidateResponseError(reason, code string, retryable bool) *CandidateResponseError {
	// Unknown enum text is untrusted upstream data, not a diagnostic message.
	// Never reflect arbitrary strings in a client-facing error.
	switch reason {
	case "STOP", "MAX_TOKENS", "MALFORMED_FUNCTION_CALL", "UNEXPECTED_TOOL_CALL",
		"TOO_MANY_TOOL_CALLS", "MISSING_THOUGHT_SIGNATURE", "MALFORMED_RESPONSE",
		"OTHER", "FINISH_REASON_UNSPECIFIED", "NO_CANDIDATES", "MISSING_FINISH_REASON",
		"IMAGE_OTHER", "NO_IMAGE", "LANGUAGE":
	default:
		if !isGeminiContentFilter(reason) {
			reason = "UNKNOWN"
		}
	}
	return &CandidateResponseError{reason: reason, code: code, retryable: retryable}
}

func isGeminiContentFilter(reason string) bool {
	switch reason {
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII",
		"IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_RECITATION", "ESCALATION",
		"PUP_LIMITED_DISABLED":
		return true
	default:
		return false
	}
}

func hasCandidateDeliverable(parts []GeminiPart) bool {
	for _, part := range parts {
		if part.FunctionCall != nil || part.ExecutableCode != nil || part.CodeExecutionResult != nil {
			return true
		}
		if part.InlineData != nil && strings.HasPrefix(part.InlineData.MimeType, "image/") && part.InlineData.Data != "" {
			return true
		}
		if !part.Thought && strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}

// A stream supplies deliverables and buffered tools from earlier chunks too.
func validateCandidateCompletion(candidate GeminiChatCandidate, deliverable, hasTools bool) *CandidateResponseError {
	if candidate.FinishReason == nil || *candidate.FinishReason == "" {
		return candidateResponseError("MISSING_FINISH_REASON", "incomplete_response", false)
	}
	reason := *candidate.FinishReason
	if reason == "MALFORMED_FUNCTION_CALL" {
		return candidateResponseError(reason, "malformed_function_call", !deliverable)
	}
	if hasTools && reason != "STOP" {
		return candidateResponseError(reason, "incomplete_tool_response", false)
	}
	switch reason {
	case "STOP":
		if !deliverable && !hasTools {
			return candidateResponseError(reason, "empty_response", true)
		}
		return nil
	case "MAX_TOKENS":
		if !deliverable {
			return candidateResponseError(reason, "empty_response", false)
		}
		return nil
	default:
		if isGeminiContentFilter(reason) {
			return nil
		}
		return candidateResponseError(reason, "invalid_finish_reason", false)
	}
}
