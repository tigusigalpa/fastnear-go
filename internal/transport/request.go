package transport

// Request describes a single logical API call. Segments are raw values;
// the transport encodes them. Body, when non-nil, is sent verbatim and
// Content-Type: application/json is applied unless overridden.
type Request struct {
	// Method is the HTTP method: GET or POST.
	Method string
	// Segments are raw path segments appended to the base URL.
	Segments []string
	// Query holds optional query parameters (apiKey excluded; the
	// transport adds it when AuthQuery is used).
	Query Query
	// Headers are extra per-request headers.
	Headers map[string]string
	// Body is the raw request payload (usually JSON).
	Body []byte
	// Op names the logical call for logging and errors, e.g.
	// "accounts.account_full".
	Op string
	// NoRetry marks non-idempotent calls (sendTx, broadcastTx*) that
	// must not be retried unless RetryNonIdempotent is enabled.
	NoRetry bool
	// NoFollowRedirect asks the transport to return the 3xx response
	// untouched (used to read Location without downloading a block).
	NoFollowRedirect bool
	// CheckResponse maps protocol errors in an HTTP-success response into
	// the shared retry policy (used for JSON-RPC errors).
	CheckResponse func(*Response) error
}
