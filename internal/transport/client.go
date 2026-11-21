// Package transport implements the shared HTTP layer for all FastNear
// API families: URL building, authentication, retries, redirects,
// response-size limits and structured logging. The API key never
// appears in logs, errors or returned URLs.
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

// AuthMode selects how the API key is transmitted.
type AuthMode string

const (
	// AuthBearer sends "Authorization: Bearer <key>" (default, preferred).
	AuthBearer AuthMode = "bearer"
	// AuthQuery appends "?apiKey=<key>" — only for environments where
	// headers are unavailable. The key may end up in server logs.
	AuthQuery AuthMode = "query"
)

// Middleware is an optional observability hook invoked after each
// completed HTTP round trip (attempt), before retries are considered.
// It is a type alias so plain funcs can be used interchangeably.
type Middleware = func(req *http.Request, resp *http.Response, elapsed time.Duration)

// Params configures a Client bound to a single API family base URL.
type Params struct {
	HTTPClient     *http.Client
	BaseURL        string
	APIKey         string
	AuthMode       AuthMode
	Family         string
	Timeout        time.Duration
	ConnectTimeout time.Duration
	Retry          RetryPolicy
	Logger         *slog.Logger
	UserAgent      string
	MaxBytes       int64
	// ConfigErr, when non-nil, is returned (wrapped) by every Do call.
	// It lets family clients surface configuration problems lazily.
	ConfigErr error
	// Middleware hooks, invoked per attempt.
	Middleware []Middleware
	// RedirectHostSuffixes lists host suffixes (e.g. ".neardata.xyz")
	// a cross-host redirect may target. Any other cross-host redirect
	// is refused, so credentials never leave the allowed domain set.
	RedirectHostSuffixes []string
}

// Client executes requests against one family base URL. It is safe for
// concurrent use; per-family Clients may share the same *http.Client.
type Client struct {
	hc         *http.Client
	baseURL    string
	apiKey     string
	authMode   AuthMode
	family     string
	timeout    time.Duration
	retry      RetryPolicy
	logger     *slog.Logger
	ua         string
	maxBytes   int64
	configErr  error
	middleware []Middleware
	redirHosts []string
}

// New builds a transport client. When Params.HTTPClient is nil a
// tuned client with redirect control is created.
func New(p Params) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(p.BaseURL, "/"),
		apiKey:     p.APIKey,
		authMode:   p.AuthMode,
		family:     p.Family,
		timeout:    p.Timeout,
		retry:      p.Retry,
		logger:     p.Logger,
		ua:         p.UserAgent,
		maxBytes:   p.MaxBytes,
		configErr:  p.ConfigErr,
		middleware: p.Middleware,
	}
	for _, s := range p.RedirectHostSuffixes {
		c.redirHosts = append(c.redirHosts, strings.ToLower(s))
	}
	if c.ua == "" {
		c.ua = "fastnear-go/1.0.0 (+https://github.com/tigusigalpa/fastnear-go)"
	}
	if c.timeout <= 0 {
		c.timeout = 30 * time.Second
	}
	if c.maxBytes <= 0 {
		c.maxBytes = 32 << 20
	}
	if c.authMode == "" {
		c.authMode = AuthBearer
	}
	if c.retry.BaseDelay <= 0 {
		c.retry.BaseDelay = 200 * time.Millisecond
	}
	if c.retry.MaxDelay <= 0 {
		c.retry.MaxDelay = 5 * time.Second
	}
	if c.retry.Budget <= 0 {
		c.retry.Budget = 15 * time.Second
	}
	if p.HTTPClient != nil {
		copyClient := *p.HTTPClient
		customRedirect := copyClient.CheckRedirect
		copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if err := c.checkRedirect(req, via); err != nil {
				return err
			}
			if customRedirect != nil {
				return customRedirect(req, via)
			}
			return nil
		}
		c.hc = &copyClient
	} else {
		c.hc = &http.Client{
			Transport:     defaultHTTPTransport(p.ConnectTimeout),
			CheckRedirect: c.checkRedirect,
		}
	}
	return c
}

func defaultHTTPTransport(connectTimeout time.Duration) *http.Transport {
	if connectTimeout <= 0 {
		connectTimeout = 5 * time.Second
	}
	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: connectTimeout,
		ForceAttemptHTTP2:   true,
	}
}

// NewHTTPClient builds the shared connection pool used by the root client.
// Each family copies the client and installs its own redirect checks.
func NewHTTPClient(connectTimeout time.Duration) *http.Client {
	return &http.Client{Transport: defaultHTTPTransport(connectTimeout)}
}

// CloseIdleConnections releases keep-alive connections. Safe to call
// multiple times.
func (c *Client) CloseIdleConnections() {
	if c != nil && c.hc != nil {
		c.hc.CloseIdleConnections()
	}
}

// Response is a completed, successful (2xx or deliberately unfollowed
// 3xx) HTTP response.
type Response struct {
	StatusCode   int
	Header       http.Header
	Body         []byte
	EffectiveURL string
}

type ctxKeyNoRedirect struct{}

// Do executes the request with auth, retries, size limits and error
// mapping. On success a *Response is returned; every failure is a
// *fastnearerr.Error.
func (c *Client) Do(ctx context.Context, r Request) (*Response, error) {
	if c == nil {
		return nil, fastnearerr.Config(r.Op, rOpFamily(r), "uninitialized client")
	}
	if c.configErr != nil {
		return nil, &fastnearerr.Error{Op: r.Op, Family: c.family, Code: "configuration_error", Err: c.configErr}
	}
	fullURL, err := c.buildURL(r)
	if err != nil {
		return nil, &fastnearerr.Error{Op: r.Op, Family: c.family, Code: "validation_error", Message: err.Error()}
	}
	scrubbedURL := scrubKey(fullURL)

	maxRetries := c.retry.MaxRetries
	if r.NoRetry && !c.retry.RetryNonIdempotent {
		maxRetries = 0
	}
	budgetStart := time.Now()

	var lastErr error
	rpcRetries := 0
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, err)
		}
		if attempt > 0 {
			wait := c.retry.backoff(attempt - 1)
			if re, ok := fastnearerr.As(lastErr); ok && re.RetryAfter > 0 {
				remaining := c.retry.Budget - time.Since(budgetStart)
				if re.RetryAfter > remaining {
					return nil, lastErr
				}
				wait = re.RetryAfter
			} else if time.Since(budgetStart)+wait > c.retry.Budget {
				return nil, lastErr
			}
			if err := c.retry.sleep(ctx, wait); err != nil {
				return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, err)
			}
		}

		attemptCtx := ctx
		cancel := func() {}
		if attempt > 0 {
			remaining := c.retry.Budget - time.Since(budgetStart)
			if remaining <= 0 {
				return nil, lastErr
			}
			attemptCtx, cancel = context.WithTimeout(ctx, remaining)
		}
		resp, err := c.doOnce(attemptCtx, r, fullURL, scrubbedURL)
		cancel()
		if err == nil && r.CheckResponse != nil {
			err = r.CheckResponse(resp)
		}
		if err == nil {
			return resp, nil
		}
		if e, ok := fastnearerr.As(err); ok {
			err = c.RedactError(e)
		}
		lastErr = err
		if attempt >= maxRetries {
			return nil, err
		}
		if e, ok := fastnearerr.As(err); ok && e.RPC != nil {
			if rpcRetries >= 1 {
				return nil, err
			}
			rpcRetries++
		}
		if !fastnearerr.IsRetryable(err) {
			return nil, err
		}
	}
}

// doOnce performs a single attempt including timeout and body limits.
func (c *Client) doOnce(ctx context.Context, r Request, fullURL, scrubbedURL string) (*Response, error) {
	reqCtx := ctx
	cancel := func() {}
	if c.timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()
	if r.NoFollowRedirect {
		reqCtx = context.WithValue(reqCtx, ctxKeyNoRedirect{}, true)
	}

	hreq, err := http.NewRequestWithContext(reqCtx, r.Method, fullURL, bytes.NewReader(r.Body))
	if err != nil {
		return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, c.sanitizeError(err))
	}
	hreq.Header.Set("Accept", "application/json")
	if r.Body != nil {
		hreq.Header.Set("Content-Type", "application/json")
	}
	if c.authMode == AuthBearer && c.apiKey != "" {
		hreq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	hreq.Header.Set("User-Agent", c.ua)
	for k, v := range r.Headers {
		hreq.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := c.hc.Do(hreq)
	elapsed := time.Since(start)
	for _, mw := range c.middleware {
		mw(hreq, resp, elapsed)
	}
	c.log(ctx, r, resp, elapsed, err)
	if err != nil {
		return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, c.sanitizeError(err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := c.readBody(resp)
	if err != nil {
		var e *fastnearerr.Error
		if errors.As(err, &e) {
			e.Op = r.Op
			e.Family = c.family
			e.RequestURL = scrubbedURL
			return nil, e
		}
		return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, c.sanitizeError(err))
	}

	// Deliberately unfollowed redirect is a successful response.
	if r.NoFollowRedirect && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body, EffectiveURL: scrubKey(resp.Request.URL.String())}, nil
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		e := fastnearerr.New(r.Op, c.family, resp.StatusCode, "redirect_refused", "redirect refused by client policy", body, nil)
		e.RequestURL = scrubbedURL
		return nil, e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrorFromResponse(r.Op, c.family, scrubbedURL, resp, body)
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body, EffectiveURL: scrubKey(resp.Request.URL.String())}, nil
}

// readBody enforces the configured response-size cap.
func (c *Client) readBody(resp *http.Response) ([]byte, error) {
	lr := io.LimitReader(resp.Body, c.maxBytes)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	// Probe separately to avoid overflow when maxBytes == math.MaxInt64.
	var extra [1]byte
	n, err := io.ReadFull(resp.Body, extra[:])
	if n > 0 {
		return nil, &fastnearerr.Error{Code: "response_too_large",
			Message: fmt.Sprintf("response body exceeds the configured limit of %d bytes", c.maxBytes)}
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	return body, nil
}

// buildURL joins base URL, encoded segments and query; appends apiKey
// last when query auth is active. The path is concatenated verbatim so
// segment escaping happens exactly once (assigning to url.URL.Path
// would re-encode '%').
func (c *Client) buildURL(r Request) (string, error) {
	if _, err := url.Parse(c.baseURL); err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(len(c.baseURL) + 64)
	b.WriteString(c.baseURL)
	b.WriteString(BuildPath(r.Segments...))
	q := make(Query, 0, len(r.Query)+1)
	q = append(q, r.Query...)
	if c.authMode == AuthQuery && c.apiKey != "" {
		q.Set("apiKey", c.apiKey)
	}
	if qs := q.Encode(); qs != "" {
		b.WriteByte('?')
		b.WriteString(qs)
	}
	return b.String(), nil
}

// checkRedirect allows at most 5 same-host redirects, preserves auth,
// and refuses cross-host redirects by surfacing the 3xx response.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 0 {
		if nf, ok := via[0].Context().Value(ctxKeyNoRedirect{}).(bool); ok && nf {
			return http.ErrUseLastResponse
		}
	}
	if len(via) >= 5 {
		return errors.New("fastnear: too many redirects")
	}
	original := via[0]
	if req.URL.User != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") ||
		(original.URL.Scheme == "https" && req.URL.Scheme != "https") {
		return http.ErrUseLastResponse
	}
	if !strings.EqualFold(req.URL.Host, original.URL.Host) &&
		(!c.redirectHostAllowed(req.URL.Hostname()) || req.URL.Port() != original.URL.Port()) {
		return http.ErrUseLastResponse
	}
	// Restore header auth for trusted mirrors; net/http drops it on some
	// cross-host redirects. Evaluate every hop against the original host.
	if c.authMode == AuthBearer && c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	// Re-attach apiKey on redirect when query auth is in use; the
	// Location header never carries it.
	if c.authMode == AuthQuery && c.apiKey != "" {
		q := req.URL.Query()
		q.Set("apiKey", c.apiKey)
		req.URL.RawQuery = q.Encode()
		req.Header.Del("Referer")
	}
	return nil
}

// redirectHostAllowed reports whether a cross-host redirect target is
// within the configured suffix allowlist (e.g. ".mainnet.neardata.xyz"
// allows "a0.mainnet.neardata.xyz" but not "evilmainnet.neardata.xyz").
func (c *Client) redirectHostAllowed(host string) bool {
	host = strings.ToLower(host)
	for _, s := range c.redirHosts {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

// log emits one slog record per attempt without secrets.
func (c *Client) log(ctx context.Context, r Request, resp *http.Response, elapsed time.Duration, err error) {
	if c.logger == nil {
		return
	}
	status := 0
	size := int64(-1)
	if resp != nil {
		status = resp.StatusCode
		size = resp.ContentLength
	}
	attrs := []any{
		"family", c.family,
		"method", r.Method,
		"op", r.Op,
		"status", status,
		"duration_ms", elapsed.Milliseconds(),
		"bytes", size,
	}
	if err != nil {
		attrs = append(attrs, "error", c.sanitizeError(err).Error())
	}
	if err != nil || status >= 400 {
		c.logger.Log(ctx, slog.LevelWarn, "fastnear request failed", attrs...)
	} else {
		c.logger.DebugContext(ctx, "fastnear request", attrs...)
	}
}

// scrubKey removes the apiKey query parameter from a URL string.
func scrubKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "fastnear://invalid-url"
	}
	q := u.Query()
	q.Del("apiKey")
	u.RawQuery = q.Encode()
	u.User = nil
	return u.String()
}

// sanitizeError preserves error inspection while scrubbing URL-bearing errors
// and messages supplied by custom transports.
func (c *Client) sanitizeError(err error) error {
	if e, ok := err.(*url.Error); ok {
		copyError := *e
		copyError.URL = scrubKey(e.URL)
		copyError.Err = c.sanitizeError(e.Err)
		return &copyError
	}
	if err != nil {
		if msg := c.redact(err.Error()); msg != err.Error() {
			return &redactedError{err: err, message: msg}
		}
	}
	return err
}

// RedactError scrubs a protocol error constructed after an HTTP response,
// including per-call batch errors. It does not mutate the supplied error.
func (c *Client) RedactError(e *fastnearerr.Error) *fastnearerr.Error {
	if e == nil {
		return nil
	}
	copyError := *e
	copyError.Code, copyError.Message = c.redact(e.Code), c.redact(e.Message)
	copyError.RawBody = []byte(c.redact(string(e.RawBody)))
	if len(copyError.RawBody) > 8<<10 {
		copyError.RawBody = copyError.RawBody[:8<<10]
	}
	copyError.Err = c.sanitizeError(e.Err)
	if e.RPC != nil {
		copyRPC := *e.RPC
		copyRPC.Message = c.redact(e.RPC.Message)
		copyRPC.Name = c.redact(e.RPC.Name)
		copyRPC.Data = []byte(c.redact(string(e.RPC.Data)))
		copyError.RPC = &copyRPC
	}
	return &copyError
}

type redactedError struct {
	err     error
	message string
}

func (e *redactedError) Error() string { return e.message }
func (e *redactedError) Unwrap() error { return e.err }

func (c *Client) redact(s string) string {
	if c.apiKey == "" {
		return s
	}
	for _, key := range []string{url.QueryEscape(c.apiKey), url.PathEscape(c.apiKey), c.apiKey} {
		s = strings.ReplaceAll(s, key, "[REDACTED]")
	}
	return s
}

func rOpFamily(r Request) string {
	if i := strings.IndexByte(r.Op, '.'); i > 0 {
		return r.Op[:i]
	}
	return ""
}
