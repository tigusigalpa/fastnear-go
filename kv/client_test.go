package kv

import (
	"context"
	"encoding/json"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
	"io"
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
		MaxBytes: 1 << 20,
	})
	return NewClient(tr, nil)
}

// TestPaths covers all nine KV operations.
func TestPaths(t *testing.T) {
	lim := uint32(10)
	cases := []struct {
		name   string
		method string
		path   string
		call   func(c *Client) error
	}{
		{"latest_exact", "GET", "/v0/latest/social.near/root.near/graph%2Ffollow%2Fx.near", func(c *Client) error {
			_, e := c.LatestByExactKey(context.Background(), "social.near", "root.near", "graph/follow/x.near", false)
			return e
		}},
		{"history_exact", "GET", "/v0/history/social.near/root.near/k1", func(c *Client) error {
			_, e := c.HistoryByExactKey(context.Background(), "social.near", "root.near", "k1", true, &lim, false, nil)
			return e
		}},
		{"latest_account", "POST", "/v0/latest/social.near", func(c *Client) error {
			_, e := c.LatestByAccount(context.Background(), "social.near", LatestQuery{})
			return e
		}},
		{"latest_predecessor", "POST", "/v0/latest/social.near/root.near", func(c *Client) error {
			_, e := c.LatestByPredecessor(context.Background(), "social.near", "root.near", LatestQuery{})
			return e
		}},
		{"history_account", "POST", "/v0/history/social.near", func(c *Client) error {
			_, e := c.HistoryByAccount(context.Background(), "social.near", HistoryQuery{})
			return e
		}},
		{"history_predecessor", "POST", "/v0/history/social.near/root.near", func(c *Client) error {
			_, e := c.HistoryByPredecessor(context.Background(), "social.near", "root.near", HistoryQuery{})
			return e
		}},
		{"history_key", "POST", "/v0/history", func(c *Client) error {
			_, e := c.HistoryByKey(context.Background(), "k1", HistoryQuery{})
			return e
		}},
		{"all_predecessor", "POST", "/v0/all/root.near", func(c *Client) error {
			_, e := c.AllByPredecessor(context.Background(), "root.near", &lim, false, nil)
			return e
		}},
		{"multi", "POST", "/v0/multi", func(c *Client) error {
			_, e := c.Multi(context.Background(), []string{"a.near/b.near/k"}, false)
			return e
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotMethod string
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotMethod = r.URL.EscapedPath(), r.Method
				_, _ = w.Write([]byte(`{"entries":[]}`))
			})
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if gotMethod != tc.method || gotPath != tc.path {
				t.Fatalf("%s %s, want %s %s", gotMethod, gotPath, tc.method, tc.path)
			}
		})
	}
}

// TestExactKeyQueryParams checks GET query flags.
func TestExactKeyQueryParams(t *testing.T) {
	var gotQuery string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"entries":[]}`))
	})
	lim, tok := uint32(7), "pt"
	_, err := c.HistoryByExactKey(context.Background(), "a.near", "b.near", "k", true, &lim, true, &tok)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	for _, want := range []string{"asc=true", "limit=7", "include_metadata=true", "page_token=pt"} {
		if !contains(gotQuery, want) {
			t.Fatalf("query %q missing %q", gotQuery, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestLatestBody checks the POST body marshalling.
func TestLatestBody(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"entries":[{"key":"k","value":{"a":1}}],"page_token":"nxt"}`))
	})
	k, lim := "k1", uint32(50)
	l, err := c.LatestByAccount(context.Background(), "social.near", LatestQuery{
		Key: &k, Limit: &lim, IncludeMetadata: true,
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if got["key"] != "k1" || got["limit"] != float64(50) || got["include_metadata"] != true {
		t.Fatalf("body = %v", got)
	}
	if l == nil || len(l.Entries) != 1 || !l.HasMore() || l.NextToken() != "nxt" {
		t.Fatalf("list = %+v", l)
	}
	var v map[string]int
	if err := l.Entries[0].ValueAs(&v); err != nil || v["a"] != 1 {
		t.Fatalf("value = %v, err=%v", v, err)
	}
}

// TestValidation covers local checks.
func TestValidation(t *testing.T) {
	c := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request expected")
	})
	if _, err := c.LatestByExactKey(context.Background(), "a.near", "b.near", "", false); err == nil {
		t.Fatal("want empty key error")
	}
	k1, k2 := "a", "b"
	if _, err := c.LatestByAccount(context.Background(), "a.near", LatestQuery{Key: &k1, KeyPrefix: &k2}); err == nil {
		t.Fatal("want key/key_prefix conflict error")
	}
	bad := uint32(201)
	if _, err := c.LatestByAccount(context.Background(), "a.near", LatestQuery{Limit: &bad}); err == nil {
		t.Fatal("want limit error")
	}
	if _, err := c.Multi(context.Background(), nil, false); err == nil {
		t.Fatal("want empty keys error")
	}
	if _, err := c.Multi(context.Background(), make([]string, 101), false); err == nil {
		t.Fatal("want >100 keys error")
	}
	if _, err := c.LatestByAccount(context.Background(), "bad id", LatestQuery{}); err == nil {
		t.Fatal("want bad account_id error")
	}
}

// TestMulti decodes the multi response with a null slot.
func TestMulti(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"entries":[{"key":"k1","value":null},null]}`))
	})
	m, err := c.Multi(context.Background(), []string{"a.near/b.near/k1", "a.near/b.near/k2"}, false)
	if err != nil || m == nil {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if len(m.Entries) != 2 || m.Entries[0] == nil || m.Entries[1] != nil {
		t.Fatalf("entries = %+v", m.Entries)
	}
	if !m.Entries[0].ValueIsNull() {
		t.Fatal("want null value")
	}
}

// TestIteratorWalksPages follows page_token (absent = end).
func TestIteratorWalksPages(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		if _, has := body["page_token"]; has {
			_, _ = w.Write([]byte(`{"entries":[{"key":"k2"}]}`)) // no page_token = last page
			return
		}
		_, _ = w.Write([]byte(`{"entries":[{"key":"k1"}],"page_token":"p1"}`))
	})
	it := NewIterator(func(token *string) (*List, error) {
		return c.LatestByAccount(context.Background(), "social.near", LatestQuery{PageToken: token})
	}, 0)
	var keys []string
	for {
		rows, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		for _, e := range rows {
			keys = append(keys, e.Key)
		}
	}
	if len(keys) != 2 || keys[0] != "k1" || keys[1] != "k2" || calls != 2 {
		t.Fatalf("keys=%v calls=%d", keys, calls)
	}
}
