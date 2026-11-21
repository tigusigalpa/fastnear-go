package fastnear

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// Option configures NewClient. Options are applied in order and each
// may return an error, which aborts NewClient.
type Option func(*Config) error

// WithAPIKey sets the FastNear API key. The key is sent as a Bearer
// token by default and never logged. An empty key disables the
// Authorization header (some endpoints answer anonymously).
func WithAPIKey(key string) Option {
	return func(c *Config) error {
		if strings.ContainsFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return errors.New("fastnear: api key must not contain whitespace or control characters")
		}
		c.apiKey = key
		return nil
	}
}

// WithNetwork selects mainnet or testnet.
func WithNetwork(n Network) Option {
	return func(c *Config) error {
		if n != Mainnet && n != Testnet {
			return fmt.Errorf("fastnear: unknown network %q", n)
		}
		c.network = n
		return nil
	}
}

// WithTestnet is a shorthand for WithNetwork(Testnet).
func WithTestnet() Option { return WithNetwork(Testnet) }

// WithArchival routes the RPC client to the archival node
// (archival-rpc.*.fastnear.com) which serves full chain history.
func WithArchival() Option {
	return func(c *Config) error {
		c.archival = true
		return nil
	}
}

// WithAuthMode selects Bearer (default) or query-parameter auth.
// Query mode is intended only for clients that cannot set headers;
// the key may leak into access logs.
func WithAuthMode(m AuthMode) Option {
	return func(c *Config) error {
		if m != AuthBearer && m != AuthQuery {
			return fmt.Errorf("fastnear: unknown auth mode %q", m)
		}
		c.authMode = m
		return nil
	}
}

// WithTimeout sets the per-request total timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(c *Config) error {
		if d <= 0 {
			return errors.New("fastnear: timeout must be positive")
		}
		c.timeout = d
		return nil
	}
}

// WithConnectTimeout sets the TLS/dial timeout (default 5s). It only
// applies to the built-in transport; a custom http.Client overrides it.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *Config) error {
		if d <= 0 {
			return errors.New("fastnear: connect timeout must be positive")
		}
		c.connectTimeout = d
		return nil
	}
}

// WithMaxRetries sets how many times a failed request is retried
// (default 2, i.e. at most 3 attempts).
func WithMaxRetries(n int) Option {
	return func(c *Config) error {
		if n < 0 {
			return errors.New("fastnear: max retries must be >= 0")
		}
		c.maxRetries = n
		return nil
	}
}

// WithRetryBaseDelay sets the base backoff delay (default 200ms).
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *Config) error {
		if d <= 0 {
			return errors.New("fastnear: retry base delay must be positive")
		}
		c.retryBaseDelay = d
		return nil
	}
}

// WithMaxResponseBytes sets the response body cap (default 32 MiB;
// NEAR Data blocks with shards can be large).
func WithMaxResponseBytes(n int64) Option {
	return func(c *Config) error {
		if n <= 0 {
			return errors.New("fastnear: max response bytes must be positive")
		}
		c.maxResponseBytes = n
		return nil
	}
}

// WithHTTPClient supplies a custom *http.Client. The SDK copies it without
// changing the original, shares its Transport and Jar, and applies its redirect
// safety checks before calling the custom CheckRedirect hook.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Config) error {
		if hc == nil {
			return errors.New("fastnear: http client must not be nil")
		}
		c.httpClient = hc
		return nil
	}
}

// WithBaseURL overrides the base URL of one family — useful for
// staging, proxies or local mocks.
func WithBaseURL(f Family, rawURL string) Option {
	return func(c *Config) error {
		if _, ok := baseURLs[f]; !ok {
			return fmt.Errorf("fastnear: unknown family %q", f)
		}
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(rawURL, "#") {
			return errors.New("fastnear: base URL must be an absolute HTTP(S) URL without credentials, query or fragment")
		}
		if c.baseURLs == nil {
			c.baseURLs = make(map[Family]string)
		}
		c.baseURLs[f] = strings.TrimRight(rawURL, "/")
		return nil
	}
}

// WithUserAgent overrides the default
// "fastnear-go/<version> (+https://github.com/tigusigalpa/fastnear-go)".
func WithUserAgent(ua string) Option {
	return func(c *Config) error {
		if ua == "" || strings.ContainsFunc(ua, func(r rune) bool { return r < ' ' || r == 127 }) {
			return errors.New("fastnear: user agent must be non-empty and contain no control characters")
		}
		c.userAgent = ua
		return nil
	}
}

// WithLogger enables structured logging via slog. Method, path,
// status, duration and response size are logged; the API key and the
// Authorization header are never logged.
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) error {
		c.logger = l
		return nil
	}
}

// WithNonIdempotentRetry allows retries of sendTx/broadcastTx* calls.
// Resubmitting an identical SignedTransaction is nonce-safe on NEAR,
// but the SDK leaves this decision to the caller.
func WithNonIdempotentRetry(enabled bool) Option {
	return func(c *Config) error {
		c.retryNonIdem = enabled
		return nil
	}
}

// WithMiddleware registers a hook invoked after every HTTP attempt.
func WithMiddleware(mw func(req *http.Request, resp *http.Response, elapsed time.Duration)) Option {
	return func(c *Config) error {
		if mw != nil {
			c.middleware = append(c.middleware, mw) //nolint:bodyclose // Hooks observe the response; the transport owns and closes its body.
		}
		return nil
	}
}
