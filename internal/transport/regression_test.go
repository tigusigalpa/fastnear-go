package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTransportFailureRedactsQueryKeyAndPreservesCause(t *testing.T) {
	const key = "secret+key/1234"
	var logs bytes.Buffer
	c := New(Params{
		BaseURL: "https://example.test", APIKey: key, AuthMode: AuthQuery,
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})},
	})
	_, err := c.Do(context.Background(), Request{Method: "GET", Op: "test.failure"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cause lost: %v", err)
	}
	var ue *url.Error
	if !errors.As(err, &ue) {
		t.Fatalf("url.Error lost: %v", err)
	}
	for _, value := range []string{err.Error(), ue.Error(), logs.String()} {
		if strings.Contains(value, key) || strings.Contains(value, url.QueryEscape(key)) {
			t.Fatalf("API key leaked: %s", value)
		}
	}
}

func TestCustomClientRetainsSDKRedirectRules(t *testing.T) {
	var blocks, hooks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/locator" {
			http.Redirect(w, r, "/block", http.StatusFound)
			return
		}
		blocks++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	custom := srv.Client()
	custom.CheckRedirect = func(*http.Request, []*http.Request) error { hooks++; return nil }
	c := New(Params{BaseURL: srv.URL, HTTPClient: custom})
	resp, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"locator"}, NoFollowRedirect: true})
	if err != nil || resp.StatusCode != 302 || blocks != 0 || hooks != 0 {
		t.Fatalf("redirect probe followed: resp=%v err=%v", resp, err)
	}
	if _, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"locator"}}); err != nil {
		t.Fatal(err)
	}
	if blocks != 1 || hooks != 1 {
		t.Fatalf("blocks=%d hooks=%d", blocks, hooks)
	}
	// The caller's client and callback are unchanged.
	if c.hc == custom {
		t.Fatal("SDK must copy the supplied client")
	}
}

func TestRedirectsKeepCredentialsWithinOriginalTrustBoundary(t *testing.T) {
	c := New(Params{BaseURL: "https://data.example.test", APIKey: "key", RedirectHostSuffixes: []string{".data.example.test"}})
	origin, _ := http.NewRequest("GET", "https://data.example.test/", nil)
	for _, target := range []string{"http://data.example.test/x", "https://evil.test/x", "https://mirror.data.example.test:444/x", "https://user:pass@data.example.test/x"} {
		req, _ := http.NewRequest("GET", target, nil)
		if err := c.checkRedirect(req, []*http.Request{origin}); !errors.Is(err, http.ErrUseLastResponse) {
			t.Fatalf("allowed unsafe redirect %s: %v", target, err)
		}
	}
	mirror, _ := http.NewRequest("GET", "https://a0.data.example.test/x", nil)
	if err := c.checkRedirect(mirror, []*http.Request{origin}); err != nil {
		t.Fatal(err)
	}
	if mirror.Header.Get("Authorization") != "Bearer key" {
		t.Fatal("trusted mirror lost authentication")
	}
	// A mirror cannot expand trust to its own subdomains.
	out, _ := http.NewRequest("GET", "https://evil.test/x", nil)
	if err := c.checkRedirect(out, []*http.Request{origin, mirror}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatal("mirror expanded trust")
	}
}

func TestConnectTimeoutAndResponseLimitExtremes(t *testing.T) {
	c := New(Params{ConnectTimeout: 123 * time.Millisecond})
	tr := c.hc.Transport.(*http.Transport)
	if tr.TLSHandshakeTimeout != 123*time.Millisecond || tr.DialContext == nil || tr.Proxy == nil {
		t.Fatal("connect settings not applied")
	}
	c.maxBytes = math.MaxInt64
	body, err := c.readBody(&http.Response{Body: io.NopCloser(strings.NewReader("ok"))})
	if err != nil || string(body) != "ok" {
		t.Fatalf("max limit overflowed: body=%q err=%v", body, err)
	}
}

func TestProtocolDecodeErrorIsNotRetried(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Do(context.Background(), Request{Method: "GET", CheckResponse: func(*Response) error {
		return SerializationError("test", "rpc", "", nil, errors.New("invalid frame"))
	}})
	if err == nil || calls != 1 || fastnearerr.IsRetryable(err) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRetryAfterDoesNotOverflow(t *testing.T) {
	d, ok := parseRetryAfter("9223372036854775807")
	if !ok || d <= 0 {
		t.Fatalf("Retry-After overflowed: %v %t", d, ok)
	}
	p := RetryPolicy{BaseDelay: time.Duration(math.MaxInt64), MaxDelay: 5 * time.Second}
	if d := p.backoff(10); d < 0 || d > 5*time.Second {
		t.Fatalf("backoff overflowed: %v", d)
	}
}

func TestEmptyJSONIsNotMissingData(t *testing.T) {
	var result any
	for _, body := range []string{"", "  ", "{"} {
		if _, err := DecodeJSON([]byte(body), &result); err == nil {
			t.Fatalf("invalid JSON accepted: %q", body)
		}
	}
	if found, err := DecodeJSON([]byte(" null "), &result); err != nil || found {
		t.Fatalf("null = %v %v", found, err)
	}
}
