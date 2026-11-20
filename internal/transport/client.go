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
	HTTPClient *http.Client
	BaseURL    string
	APIKey     string
	AuthMode   AuthMode
	Family     string
	Timeout    time.Duration
	Retry      RetryPolicy
	Logger     *slog.Logger
	UserAgent  string
	MaxBytes   int64
	// ConfigErr, when non-nil, is returned (wrapped) by every Do call.
	// It lets family clients surface configuration problems lazily.
	ConfigErr error
	// Middleware hooks, invoked per attempt.
	Middleware []Middleware
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
	ownsClient bool
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
	if p.HTTPClient != nil {
		c.hc = p.HTTPClient
	} else {
		c.ownsClient = true
		c.hc = &http.Client{
			Transport:     defaultHTTPTransport(),
			CheckRedirect: c.checkRedirect,
		}
	}
	return c
}

func defaultHTTPTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		ForceAttemptHTTP2:   true,
	}
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

	maxAttempts := 1 + c.retry.MaxRetries
	if r.NoRetry && !c.retry.RetryNonIdempotent {
		maxAttempts = 1
	}
	budgetStart := time.Now()

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
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

		resp, err := c.doOnce(ctx, r, fullURL, scrubbedURL)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !fastnearerr.IsRetryable(err) {
			return nil, err
		}
	}
	return nil, lastErr
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
		return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, err)
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
	c.log(r, resp, elapsed, err)
	if err != nil {
		return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, err)
	}
	defer resp.Body.Close()

	body, err := c.readBody(resp)
	if err != nil {
		var e *fastnearerr.Error
		if errors.As(err, &e) {
			e.Op = r.Op
			e.Family = c.family
			e.RequestURL = scrubbedURL
			return nil, e
		}
		return nil, ErrorFromTransport(r.Op, c.family, scrubbedURL, err)
	}

	// Deliberately unfollowed redirect is a successful response.
	if r.NoFollowRedirect && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body, EffectiveURL: resp.Request.URL.String()}, nil
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, &fastnearerr.Error{
			Op: r.Op, Family: c.family, StatusCode: resp.StatusCode,
			Code: "redirect_refused", Message: "redirect to a different host refused",
			RequestURL: scrubbedURL, RawBody: body,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrorFromResponse(r.Op, c.family, scrubbedURL, resp, body)
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body, EffectiveURL: resp.Request.URL.String()}, nil
}

// readBody enforces the configured response-size cap.
func (c *Client) readBody(resp *http.Response) ([]byte, error) {
	lr := io.LimitReader(resp.Body, c.maxBytes+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > c.maxBytes {
		return nil, &fastnearerr.Error{Code: "response_too_large",
			Message: fmt.Sprintf("response body exceeds the configured limit of %d bytes", c.maxBytes)}
	}
	return body, nil
}

// buildURL joins base URL, encoded segments and query; appends apiKey
// last when query auth is active.
func (c *Client) buildURL(r Request) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + BuildPath(r.Segments...)
	u.RawPath = ""
	q := make(Query, 0, len(r.Query)+1)
	q = append(q, r.Query...)
	if c.authMode == AuthQuery && c.apiKey != "" {
		q.Set("apiKey", c.apiKey)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
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
	prev := via[len(via)-1]
	if !strings.EqualFold(req.URL.Host, prev.URL.Host) {
		return http.ErrUseLastResponse
	}
	// Re-attach apiKey on redirect when query auth is in use; the
	// Location header never carries it.
	if c.authMode == AuthQuery && c.apiKey != "" {
		q := req.URL.Query()
		if q.Get("apiKey") == "" {
			q.Set("apiKey", c.apiKey)
			req.URL.RawQuery = q.Encode()
		}
	}
	return nil
}

// log emits one slog record per attempt without secrets.
func (c *Client) log(r Request, resp *http.Response, elapsed time.Duration, err error) {
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
		attrs = append(attrs, "error", err.Error())
	}
	if err != nil || status >= 400 {
		c.logger.Log(context.Background(), slog.LevelWarn, "fastnear request failed", attrs...)
	} else {
		c.logger.Debug("fastnear request", attrs...)
	}
}

// scrubKey removes the apiKey query parameter from a URL string.
func scrubKey(raw string) string {
	if !strings.Contains(raw, "apiKey=") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "fastnear://invalid-url"
	}
	q := u.Query()
	q.Del("apiKey")
	u.RawQuery = q.Encode()
	return u.String()
}

func rOpFamily(r Request) string {
	if i := strings.IndexByte(r.Op, '.'); i > 0 {
		return r.Op[:i]
	}
	return ""
}
