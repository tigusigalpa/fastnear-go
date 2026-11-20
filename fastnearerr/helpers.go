package fastnearerr

import (
	"context"
	"errors"
	"net"
)

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool {
	e, ok := As(err)
	return ok && e.StatusCode == 404
}

// IsAuth reports whether err is an authentication failure (401/403).
func IsAuth(err error) bool {
	e, ok := As(err)
	return ok && (e.StatusCode == 401 || e.StatusCode == 403)
}

// IsQuotaExceeded reports whether err is a 402 quota_exceeded error.
// This is terminal: retrying is pointless until the plan is topped up.
func IsQuotaExceeded(err error) bool {
	e, ok := As(err)
	return ok && (e.StatusCode == 402 || e.Code == "quota_exceeded")
}

// IsRateLimited reports whether err is a 429 rate-limit error.
// Inspect (*Error).RetryAfter for the suggested delay.
func IsRateLimited(err error) bool {
	e, ok := As(err)
	return ok && e.StatusCode == 429
}

// IsRetryable reports whether retrying the same request may succeed.
// It implements the matrix from the SDK specification: network
// failures and 408/425/429/5xx are retryable; deterministic client
// errors, quota exhaustion and JSON-RPC request errors are not.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	e, ok := As(err)
	if !ok {
		// A bare transport error (net.Error, url.Error, ...).
		var ne net.Error
		if errors.As(err, &ne) {
			return true
		}
		return errors.Is(err, context.DeadlineExceeded)
	}
	if e.Code == "quota_exceeded" || e.Code == "validation_error" || e.Code == "configuration_error" {
		return false
	}
	if e.RPC != nil {
		switch e.RPC.Code {
		case -32601, -32602: // method not found / invalid params
			return false
		case -32000: // server error: one retry is allowed
			return true
		}
		return false
	}
	switch e.StatusCode {
	case 408, 425, 429, 500, 502, 503, 504:
		return true
	case 0:
		return e.Err != nil && !errors.Is(e.Err, context.Canceled)
	}
	return false
}
