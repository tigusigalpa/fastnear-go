package data

import (
	"context"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
	"net/http"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family,
		Timeout:  5 * time.Second,
		Retry:    transport.RetryPolicy{MaxRetries: 0},
		MaxBytes: 8 << 20,
	})
	return NewClient(tr, nil)
}

const blockDoc = `{"block":{"author":"v.near","header":{"height":100,"hash":"BH","prev_hash":"PH","timestamp":1700000000000,"timestamp_nanosec":"1700000000000000000","gas_price":"100000000","total_supply":"1","chunks_included":1},"chunks":[{"chunk_hash":"CH","shard_id":0,"height_created":100,"height_included":100,"gas_used":0,"gas_limit":1,"outcome_root":"o","outgoing_receipts_root":"r","tx_root":"t"}]},"shards":[{"shard_id":0,"chunk":{"author":"v.near","header":{"chunk_hash":"CH"},"transactions":[],"receipts":[]},"receipt_execution_outcomes":[],"state_changes":[]}]}`

// TestPaths covers all data operations.
func TestPaths(t *testing.T) {
	cases := []struct {
		name string
		path string
		call func(c *Client) error
	}{
		{"health", "/health", func(c *Client) error { _, e := c.Health(context.Background()); return e }},
		{"first_block", "/v0/first_block", func(c *Client) error { _, e := c.FirstBlock(context.Background()); return e }},
		{"last_final", "/v0/last_block/final", func(c *Client) error { _, e := c.LastBlockFinal(context.Background()); return e }},
		{"last_opt", "/v0/last_block/optimistic", func(c *Client) error { _, e := c.LastBlockOptimistic(context.Background()); return e }},
		{"block", "/v0/block/100", func(c *Client) error { _, e := c.Block(context.Background(), 100); return e }},
		{"block_opt", "/v0/block_opt/100", func(c *Client) error { _, e := c.BlockOpt(context.Background(), 100); return e }},
		{"headers", "/v0/block/100/headers", func(c *Client) error { _, e := c.BlockHeaders(context.Background(), 100); return e }},
		{"chunk", "/v0/block/100/chunk/2", func(c *Client) error { _, e := c.Chunk(context.Background(), 100, 2); return e }},
		{"shard", "/v0/block/100/shard/3", func(c *Client) error { _, e := c.Shard(context.Background(), 100, 3); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(`{"status":"ok","block":` + `{"header":{"height":100}}` + `,"header":{"height":100},"shard_id":0}`))
			})
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if gotPath != tc.path {
				t.Fatalf("path = %q, want %q", gotPath, tc.path)
			}
		})
	}
}

// TestBlockDecode decodes a full block document.
func TestBlockDecode(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(blockDoc))
	})
	doc, err := c.Block(context.Background(), 100)
	if err != nil {
		t.Fatalf("Block: %v", err)
	}
	if doc == nil || doc.Block.Header.Height != 100 || doc.Block.Author != "v.near" {
		t.Fatalf("doc = %+v", doc)
	}
	if len(doc.Shards) != 1 || doc.Shards[0].Chunk == nil {
		t.Fatalf("shards = %+v", doc.Shards)
	}
	if len(doc.Raw) == 0 {
		t.Fatal("raw not preserved")
	}
	tm, err := doc.Block.Header.Time()
	if err != nil || tm.IsZero() {
		t.Fatalf("time = %v, %v", tm, err)
	}
}

// TestLastOptimisticRedirect: the locator endpoint 302s to
// /v0/block_opt/{h}; the transport follows same-host redirects.
func TestLastOptimisticRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/last_block/optimistic", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/v0/block_opt/555", http.StatusFound)
	})
	mux.HandleFunc("/v0/block_opt/555", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"block":{"header":{"height":555,"timestamp":1}},"shards":[]}`))
	})
	c := newTestClient(t, mux.ServeHTTP)
	doc, err := c.LastBlockOptimistic(context.Background())
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if doc.Block.Header.Height != 555 {
		t.Fatalf("height = %d", doc.Block.Header.Height)
	}
}

// TestHeightFromLocation: LastOptimisticHeight parses the redirect
// Location instead of downloading the block.
func TestHeightFromLocation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v0/last_block/optimistic" {
			w.Header().Set("Location", "/v0/block_opt/777")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path == "/v0/last_block/final" {
			w.Header().Set("Location", "/v0/block/700")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path == "/v0/first_block" {
			// absolute cross-host-style Location still parses
			w.Header().Set("Location", "https://a0.mainnet.neardata.xyz/v0/block/9820210")
			w.WriteHeader(http.StatusFound)
			return
		}
		t.Errorf("unexpected path %s", r.URL.Path)
	})
	if h, err := c.LastOptimisticHeight(context.Background()); err != nil || h != 777 {
		t.Fatalf("h=%d err=%v", h, err)
	}
	if h, err := c.LastFinalHeight(context.Background()); err != nil || h != 700 {
		t.Fatalf("h=%d err=%v", h, err)
	}
	if h, err := c.FirstBlockHeight(context.Background()); err != nil || h != 9820210 {
		t.Fatalf("h=%d err=%v", h, err)
	}
}

// TestCrossHostRedirect: mirror hosts under the base domain are
// followed when the suffix is allowed; other hosts are refused.
func TestCrossHostRedirect(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"block":{"header":{"height":9820210}},"shards":[]}`))
	}))
	defer mirror.Close()

	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, mirror.URL+"/v0/block/9820210", http.StatusFound)
	}
	c := newTestClient(t, handler)
	// No allowed suffixes → refused (mirror is a different host).
	if _, err := c.FirstBlock(context.Background()); err == nil {
		t.Fatal("want redirect refusal")
	}
}

// TestParseLocationHeight unit-tests the Location parser.
func TestParseLocationHeight(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"/v0/block/123", 123, true},
		{"/v0/block_opt/456", 456, true},
		{"https://a0.mainnet.neardata.xyz/v0/block/9820210", 9820210, true},
		{"/v0/block/9?x=1", 9, true},
		{"", 0, false},
		{"/v0/other/1", 0, false},
		{"/v0/block/", 0, false},
		{"/v0/block/abc", 0, false},
	} {
		got, err := parseLocationHeight(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("parse(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parse(%q) expected error", tc.in)
		}
	}
}
