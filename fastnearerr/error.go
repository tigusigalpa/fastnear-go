// Package fastnearerr defines the unified error model shared by all
// FastNear API families. Every failure returned by this module is either
// an *Error or wraps one, so errors.As can always recover it.
package fastnearerr

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// maxRawBody is the cap on RawBody kept for diagnostics (8 KiB).
const maxRawBody = 8 << 10

// Error is the single error type produced by every FastNear SDK call.
// It never contains the API key: RequestURL is scrubbed and Error()
// never prints RawBody.
type Error struct {
	// Op identifies the logical operation, e.g. "rpc.view_account".
	Op string
	// Family identifies the API surface: "accounts", "rpc", "tx",
	// "data", "transfers" or "kv".
	Family string
	// StatusCode is the HTTP status, 0 for pure transport failures.
	StatusCode int
	// Code is a machine-readable error identifier taken from the
	// response body ("quota_exceeded", "invalid_request") or assigned
	// by the SDK ("validation_error", "response_too_large").
	Code string
	// Message is a human-readable description.
	Message string
	// RPC is populated only for JSON-RPC level errors.
	RPC *RPCError
	// RetryAfter is parsed from the Retry-After response header.
	RetryAfter time.Duration
	// RequestURL is the final request URL with credentials removed.
	RequestURL string
	// RawBody holds up to 8 KiB of the response body.
	RawBody []byte
	// Err is the wrapped underlying error (transport failures,
	// JSON decoding problems, configuration issues).
	Err error
}

// RPCError carries a JSON-RPC "error" object returned with HTTP 200.
type RPCError struct {
	Code    int
	Message string
	// Data is the raw "data" member of the error object.
	Data json.RawMessage
	// Name is data.name when present (e.g. "HANDLER_ERROR").
	Name string
}

// Error returns a single-line description without secrets or bodies.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	var s string
	switch {
	case e.Op != "":
		s = "fastnear: " + e.Op
	case e.Family != "":
		s = "fastnear: " + e.Family
	default:
		s = "fastnear"
	}
	if e.StatusCode != 0 {
		s += fmt.Sprintf(": http %d", e.StatusCode)
	}
	if e.Code != "" {
		s += ": " + e.Code
	}
	if e.RPC != nil {
		s += fmt.Sprintf(": rpc error %d", e.RPC.Code)
		if e.RPC.Name != "" {
			s += " (" + e.RPC.Name + ")"
		}
		if e.RPC.Message != "" {
			s += ": " + e.RPC.Message
		}
	}
	if e.Message != "" {
		s += ": " + e.Message
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

// Unwrap exposes the underlying error so errors.Is/As work, e.g.
// errors.Is(err, context.DeadlineExceeded) after a timeout.
func (e *Error) Unwrap() error { return e.Err }

// Temporary reports whether the failure is worth retrying.
func (e *Error) Temporary() bool { return IsRetryable(e) }

// New builds an *Error for a failed transport/HTTP call.
func New(op, family string, statusCode int, code, message string, raw []byte, err error) *Error {
	return &Error{
		Op:         op,
		Family:     family,
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		RawBody:    truncate(raw),
		Err:        err,
	}
}

// NewRPC builds an *Error for a JSON-RPC error object.
func NewRPC(op, family, requestURL string, rpc *RPCError, raw []byte) *Error {
	code := fmt.Sprintf("rpc_error_%d", rpc.Code)
	if rpc.Name != "" {
		code = rpc.Name
	}
	return &Error{
		Op:         op,
		Family:     family,
		StatusCode: 200,
		Code:       code,
		Message:    rpc.Message,
		RPC:        rpc,
		RequestURL: requestURL,
		RawBody:    truncate(raw),
	}
}

// Validation returns an *Error for a local argument problem detected
// before any network call.
func Validation(op, family, message string) *Error {
	return &Error{Op: op, Family: family, Code: "validation_error", Message: message}
}

// Config returns an *Error for a client configuration problem.
func Config(op, family, message string) *Error {
	return &Error{Op: op, Family: family, Code: "configuration_error", Message: message}
}

func truncate(b []byte) []byte {
	if len(b) > maxRawBody {
		return b[:maxRawBody]
	}
	return b
}

// As extracts an *Error from an error chain.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
