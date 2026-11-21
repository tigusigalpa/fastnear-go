package transport

import (
	"context"
	"errors"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

func newTestClient(t *testing.T, srv *httptest.Server, mutate func(*Params)) *Client {
	t.Helper()
	p := Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL,
		APIKey:   "test-key-1234",
		Family:   "accounts",
		Timeout:  5 * time.Second,
		Retry:    RetryPolicy{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond, Budget: 10 * time.Second},
		MaxBytes: 1 << 20,
	}
	p.Retry.sleeper = func(_ context.Context, _ time.Duration) error { return nil }
	if mutate != nil {
		mutate(&p)
	}
	return New(p)
}

func TestBearerAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"health"}, Op: "accounts.health"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer test-key-1234" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestQueryAuthModeAndScrubbing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apiKey") != "test-key-1234" {
			t.Errorf("apiKey not appended")
		}
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv, func(p *Params) { p.AuthMode = AuthQuery })
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "accounts.x"})
	if err == nil {
		t.Fatal("expected error")
	}
	var e *fastnearerr.Error
	if !errors.As(err, &e) {
		t.Fatalf("not *Error: %T", err)
	}
	if strings.Contains(e.RequestURL, "test-key-1234") {
		t.Fatalf("key leaked in RequestURL: %q", e.RequestURL)
	}
	if strings.Contains(e.Error(), "test-key-1234") {
		t.Fatalf("key leaked in Error(): %q", e.Error())
	}
	if e.Code != "invalid_api_key" || e.StatusCode != 403 {
		t.Fatalf("got code=%q status=%d", e.Code, e.StatusCode)
	}
}

func TestQuotaExceededNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(402)
		_, _ = w.Write([]byte(`{"error":"quota_exceeded"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "accounts.x"})
	if err == nil || !fastnearerr.IsQuotaExceeded(err) {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls = %d, want 1", atomic.LoadInt32(&calls))
	}
	if fastnearerr.IsRetryable(err) {
		t.Fatal("402 must not be retryable")
	}
}

func TestRateLimitRetriedWithRetryAfter(t *testing.T) {
	var calls int32
	var slept []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv, func(p *Params) {
		p.Retry.sleeper = func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		}
	})
	resp, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "accounts.x"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d", atomic.LoadInt32(&calls))
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("slept = %v", slept)
	}
	_ = resp
}

func TestServerErrorRetriedThenFails(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "accounts.x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&calls) != 3 { // 1 + 2 retries
		t.Fatalf("calls = %d, want 3", atomic.LoadInt32(&calls))
	}
	e, _ := fastnearerr.As(err)
	if e.StatusCode != 503 || e.Code != "server_error" {
		t.Fatalf("code=%q status=%d", e.Code, e.StatusCode)
	}
}

func TestNoRetryOnNonIdempotent(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Do(context.Background(), Request{Method: "POST", Body: []byte(`{}`), NoRetry: true, Op: "rpc.send_tx"})
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls = %d, want 1 (send_tx must not be retried)", atomic.LoadInt32(&calls))
	}
}

func TestRedirectFollowedWithAuth(t *testing.T) {
	var finalAuth string
	var finalPath string
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	mux.HandleFunc("/v0/last_block/final", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/v0/block/123", http.StatusFound)
	})
	mux.HandleFunc("/v0/block/123", func(w http.ResponseWriter, r *http.Request) {
		finalAuth = r.Header.Get("Authorization")
		finalPath = r.URL.Path
		_, _ = w.Write([]byte(`{"block":{}}`))
	})
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	resp, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"v0", "last_block", "final"}, Op: "data.last_final"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != 200 || finalPath != "/v0/block/123" {
		t.Fatalf("status=%d path=%q", resp.StatusCode, finalPath)
	}
	if finalAuth != "Bearer test-key-1234" {
		t.Fatalf("auth lost on redirect: %q", finalAuth)
	}
}

func TestNoFollowRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/v0/block_opt/456")
		w.WriteHeader(302)
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	resp, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"v0", "last_block", "optimistic"}, NoFollowRedirect: true, Op: "data.last_opt"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "/v0/block_opt/456" {
		t.Fatalf("status=%d location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestCrossHostRedirectRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.example.com/x", http.StatusFound)
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "data.x"})
	if err == nil {
		t.Fatal("expected redirect refusal")
	}
	e, _ := fastnearerr.As(err)
	if e == nil || e.Code != "redirect_refused" {
		t.Fatalf("err = %v", err)
	}
}

func TestResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 2048))
	}))
	defer srv.Close()
	c := newTestClient(t, srv, func(p *Params) { p.MaxBytes = 1024 })
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "data.x"})
	if err == nil {
		t.Fatal("expected error")
	}
	e, _ := fastnearerr.As(err)
	if e == nil || e.Code != "response_too_large" {
		t.Fatalf("err = %v", err)
	}
}

func TestTimeoutIsRetryable(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	c := newTestClient(t, srv, func(p *Params) { p.Timeout = 50 * time.Millisecond })
	_, err := c.Do(context.Background(), Request{Method: "GET", Segments: []string{"x"}, Op: "x.y"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	e, _ := fastnearerr.As(err)
	if e == nil || e.Code != "timeout" {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("calls = %d, want 3", atomic.LoadInt32(&calls))
	}
}

func TestConfigErrPropagates(t *testing.T) {
	c := New(Params{ConfigErr: errors.New("transfers is mainnet-only"), Family: "transfers"})
	_, err := c.Do(context.Background(), Request{Method: "GET", Op: "transfers.query"})
	if err == nil {
		t.Fatal("expected config error")
	}
	e, _ := fastnearerr.As(err)
	if e == nil || e.Code != "configuration_error" {
		t.Fatalf("err = %v", err)
	}
}
