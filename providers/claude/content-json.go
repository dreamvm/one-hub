package claude

import (
	"bytes"
	"encoding/json"
)

// Preserve opaque response blocks, including empty thinking strings and vendor
// extension fields. A changed Go value must not accidentally replay stale bytes.
func (c *ResContent) UnmarshalJSON(data []byte) error {
	type plain ResContent
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	known, err := json.Marshal(decoded)
	if err != nil {
		return err
	}
	*c = ResContent(decoded)
	c.original = append(json.RawMessage(nil), data...)
	c.known = known
	return nil
}

func (c ResContent) MarshalJSON() ([]byte, error) {
	type plain ResContent
	encoded, err := json.Marshal(plain(c))
	if err != nil {
		return nil, err
	}
	if len(c.original) > 0 && bytes.Equal(encoded, c.known) {
		return c.original, nil
	}
	// These fields are required even when the content string is empty.
	if c.Type == "thinking" || c.Type == "text" {
		var fields map[string]any
		if err := json.Unmarshal(encoded, &fields); err != nil {
			return nil, err
		}
		if c.Type == "thinking" {
			fields["thinking"] = c.Thinking
		} else {
			fields["text"] = c.Text
		}
		return json.Marshal(fields)
	}
	return encoded, nil
}
