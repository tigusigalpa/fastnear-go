package fastnear

import "github.com/tigusigalpa/fastnear-go/fastnearerr"

// Aliases keep the error API available from the root package.

type (
	// Error is the single error type produced by every SDK call.
	Error = fastnearerr.Error
	// RPCError carries a JSON-RPC "error" object.
	RPCError = fastnearerr.RPCError
)

var (
	// IsNotFound reports whether err is a 404 API error.
	IsNotFound = fastnearerr.IsNotFound
	// IsAuth reports whether err is a 401/403 authentication error.
	IsAuth = fastnearerr.IsAuth
	// IsQuotaExceeded reports whether err is a 402 quota_exceeded error.
	IsQuotaExceeded = fastnearerr.IsQuotaExceeded
	// IsRateLimited reports whether err is a 429 rate-limit error.
	IsRateLimited = fastnearerr.IsRateLimited
	// IsRetryable reports whether retrying may succeed.
	IsRetryable = fastnearerr.IsRetryable
	// As extracts an *Error from an error chain.
	As = fastnearerr.As
	// AsError is an alias for As.
	AsError = fastnearerr.As
)
