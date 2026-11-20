package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// apiErrorBody is the REST error envelope used by FastNear API,
// Transactions, Transfers and KV: {"error": "...", "message": "..."}.
type apiErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// DecodeJSON unmarshals a successful response body into out.
// A JSON literal null is a valid "not found / out of range" answer:
// it returns (false, nil) so callers can return nil without an error.
func DecodeJSON(body []byte, out any) (bool, error) {
	if jsonx.IsNull(body) {
		return false, nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, err
	}
	return true, nil
}

// ErrorFromResponse converts a non-2xx (or refused 3xx) HTTP response
// into a *fastnearerr.Error, preserving status, code, body and
// Retry-After.
func ErrorFromResponse(op, family, requestURL string, resp *http.Response, body []byte) *fastnearerr.Error {
	e := &fastnearerr.Error{
		Op:         op,
		Family:     family,
		StatusCode: resp.StatusCode,
		RequestURL: requestURL,
		RawBody:    body,
	}
	if len(e.RawBody) > 8<<10 {
		e.RawBody = e.RawBody[:8<<10]
	}
	var parsed apiErrorBody
	if err := json.Unmarshal(body, &parsed); err == nil {
		e.Code = parsed.Error
		e.Message = parsed.Message
	}
	if e.Message == "" {
		e.Message = http.StatusText(resp.StatusCode)
	}
	if e.Code == "" {
		e.Code = codeForStatus(resp.StatusCode)
	}
	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			e.RetryAfter = d
		}
	}
	return e
}

// ErrorFromTransport wraps a transport-level failure (DNS, TLS,
// timeout, body read errors) into an *Error with StatusCode 0.
func ErrorFromTransport(op, family, requestURL string, err error) *fastnearerr.Error {
	code := "transport_error"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.Is(err, context.Canceled):
		code = "canceled"
	}
	return &fastnearerr.Error{
		Op:         op,
		Family:     family,
		Code:       code,
		Message:    "request failed",
		RequestURL: requestURL,
		Err:        err,
	}
}

// SerializationError builds an *Error for invalid JSON or a schema
// mismatch in an otherwise successful response.
func SerializationError(op, family, requestURL string, body []byte, err error) *fastnearerr.Error {
	return &fastnearerr.Error{
		Op:         op,
		Family:     family,
		Code:       "serialization_error",
		Message:    "failed to decode response",
		RequestURL: requestURL,
		RawBody:    body,
		Err:        err,
	}
}

// ResponseTooLargeError is returned when the body exceeds the
// configured byte cap.
func ResponseTooLargeError(op, family, requestURL string, limit int64) *fastnearerr.Error {
	return &fastnearerr.Error{
		Op:         op,
		Family:     family,
		Code:       "response_too_large",
		Message:    "response body exceeds the configured limit of " + strconv.FormatInt(limit, 10) + " bytes",
		RequestURL: requestURL,
	}
}

func codeForStatus(status int) string {
	switch status {
	case 400:
		return "bad_request"
	case 401, 403:
		return "authentication_error"
	case 402:
		return "quota_exceeded"
	case 404:
		return "not_found"
	case 429:
		return "rate_limited"
	default:
		if status >= 500 {
			return "server_error"
		}
		return "http_" + strconv.Itoa(status)
	}
}

// parseRetryAfter handles both "seconds" and HTTP-date forms.
func parseRetryAfter(h string) (time.Duration, bool) {
	if h == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(h); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
