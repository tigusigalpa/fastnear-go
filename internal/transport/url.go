package transport

import (
	"net/url"
	"strings"
)

// unreserved RFC 3986 characters kept verbatim in a path segment, plus
// ':' which FastNear accepts in public keys ("ed25519:...").
const unreservedChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~:"

// EncodeSegment percent-encodes a single raw path segment. Unlike
// url.PathEscape it preserves ':' which FastNear treats literally in
// public keys, while '/' becomes %2F and '%' becomes %25 (so an
// already-encoded value is never produced accidentally from raw input:
// callers always pass raw values and encoding happens exactly once).
func EncodeSegment(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(unreservedChars, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperHex(c >> 4))
		b.WriteByte(upperHex(c & 0x0f))
	}
	return b.String()
}

func upperHex(n byte) byte {
	const digits = "0123456789ABCDEF"
	return digits[n]
}

// BuildPath joins raw segments into an escaped path with a leading '/'.
// Empty segments are skipped so trailing slashes never appear.
func BuildPath(segments ...string) string {
	var b strings.Builder
	for _, s := range segments {
		if s == "" {
			continue
		}
		b.WriteByte('/')
		b.WriteString(EncodeSegment(s))
	}
	if b.Len() == 0 {
		return "/"
	}
	return b.String()
}

// Query is an ordered list of query parameters. Ordered so that apiKey
// can be appended last deterministically.
type Query [][2]string

// Set appends or replaces a parameter.
func (q *Query) Set(key, value string) {
	for i := range *q {
		if (*q)[i][0] == key {
			(*q)[i][1] = value
			return
		}
	}
	*q = append(*q, [2]string{key, value})
}

// Encode renders the query string using standard form encoding.
func (q Query) Encode() string {
	v := make(url.Values, len(q))
	for _, kv := range q {
		v.Set(kv[0], kv[1])
	}
	return v.Encode()
}
