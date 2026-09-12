package requester

import "encoding/json"

// ProtocolError serializes as an OpenAI SSE error without exposing raw upstream
// payloads. Its cause remains available for errors.Is/As in internal callers.
type ProtocolError struct {
	Message string
	Cause   error
}

func (e *ProtocolError) Error() string {
	b, _ := json.Marshal(map[string]any{"error": map[string]string{
		"type": "upstream_protocol_error", "code": "invalid_stream", "message": e.Message,
	}})
	return string(b)
}

func (e *ProtocolError) Unwrap() error { return e.Cause }
