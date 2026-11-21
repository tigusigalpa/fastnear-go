package fastnear

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// String returns a diagnostic summary with the API key masked.
func (c Config) String() string {
	return fmt.Sprintf("fastnear.Config{Network:%s AuthMode:%s APIKey:%s Archival:%t Timeout:%s MaxRetries:%d}", c.network, c.authMode, c.MaskedAPIKey(), c.archival, c.timeout, c.maxRetries)
}

// GoString keeps credentials masked in %#v output too.
func (c Config) GoString() string { return c.String() }

// Config is the immutable client configuration produced by Options.
// Obtain it through Client.Config(); construct it through NewClient
// options rather than directly.
type Config struct {
	apiKey           string
	network          Network
	authMode         AuthMode
	archival         bool
	timeout          time.Duration
	connectTimeout   time.Duration
	maxRetries       int
	retryBaseDelay   time.Duration
	maxResponseBytes int64
	baseURLs         map[Family]string
	userAgent        string
	logger           *slog.Logger
	httpClient       *http.Client
	retryNonIdem     bool
	middleware       []func(*http.Request, *http.Response, time.Duration)
}

// MaskedAPIKey returns "***" + last 4 characters of the key, or "".
// The raw key is never exposed through the config.
func (c Config) MaskedAPIKey() string {
	if len(c.apiKey) <= 4 {
		if c.apiKey == "" {
			return ""
		}
		return "***"
	}
	return "***" + c.apiKey[len(c.apiKey)-4:]
}

// HasAPIKey reports whether an API key is configured.
func (c Config) HasAPIKey() bool { return c.apiKey != "" }

// Network returns the configured network.
func (c Config) Network() Network { return c.network }

// AuthMode returns the configured authentication mode.
func (c Config) AuthMode() AuthMode { return c.authMode }

// Archival reports whether the RPC client targets the archival node.
func (c Config) Archival() bool { return c.archival }

// Timeout returns the per-request total timeout.
func (c Config) Timeout() time.Duration { return c.timeout }

// ConnectTimeout returns the TLS/dial timeout.
func (c Config) ConnectTimeout() time.Duration { return c.connectTimeout }

// MaxRetries returns the number of retries after the first attempt.
func (c Config) MaxRetries() int { return c.maxRetries }

// RetryBaseDelay returns the base backoff delay.
func (c Config) RetryBaseDelay() time.Duration { return c.retryBaseDelay }

// MaxResponseBytes returns the response body size cap.
func (c Config) MaxResponseBytes() int64 { return c.maxResponseBytes }

// UserAgent returns the configured User-Agent.
func (c Config) UserAgent() string { return c.userAgent }

// BaseURL resolves the effective base URL for a family honoring
// WithBaseURL overrides.
func (c Config) BaseURL(f Family) (string, error) {
	if u, ok := c.baseURLs[f]; ok && u != "" {
		return u, nil
	}
	return f.BaseURL(c.network, c.archival)
}
