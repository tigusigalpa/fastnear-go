// Package jsonx holds small helpers for working with json.RawMessage
// shared by the family packages.
package jsonx

import (
	"bytes"
	"encoding/json"
)

// Raw is a convenience alias kept for readability in DTO fields.
type Raw = json.RawMessage

// RawCarrier is embedded into DTOs so each carries the untouched
// response body: new upstream fields are never lost.
type RawCarrier struct {
	Raw json.RawMessage `json:"-"`
}

// SetRaw stores a copy of the raw body.
func (m *RawCarrier) SetRaw(r json.RawMessage) { m.Raw = Clone(r) }

var nullLiteral = []byte("null")

// IsNull reports whether raw is empty or the JSON literal null.
func IsNull(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, nullLiteral)
}

// Clone returns a copy of raw so callers cannot mutate shared buffers.
func Clone(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

// Unmarshal decodes raw into v. A JSON null or empty input is a no-op.
func Unmarshal(raw json.RawMessage, v any) error {
	if IsNull(raw) {
		return nil
	}
	return json.Unmarshal(raw, v)
}
