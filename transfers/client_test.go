package transfers

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

// TestQuery verifies the POST body and page decoding.
func TestQuery(t *testing.T) {
	var got map[string]any
	var gotMethod, gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transfers": []map[string]any{{
				"account_id": "a.near", "other_account_id": "b.near",
				"asset_id": "native:near", "asset_type": "native",
				"transfer_type": "transfer", "amount": "1000000000000000000000000",
				"block_height": "123", "block_timestamp": "1700000000000",
				"transaction_id": "H", "action_index": 0,
			}},
			"resume_token": "tok",
		})
	})
	asset, desc, minAmount := "native:near", true, "1000"
	lim := uint32(10)
	p, err := c.Query(context.Background(), Query{
		AccountID: "a.near", Direction: DirectionReceiver,
		AssetID: &asset, Desc: &desc, MinAmount: &minAmount, Limit: &lim,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v0/transfers" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if got["account_id"] != "a.near" || got["direction"] != "receiver" ||
		got["asset_id"] != "native:near" || got["min_amount"] != "1000" ||
		got["limit"] != float64(10) {
		t.Fatalf("body = %v", got)
	}
	if p == nil || len(p.Transfers) != 1 || !p.HasMore() || p.NextToken() != "tok" {
		t.Fatalf("page = %+v", p)
	}
	row := p.Transfers[0]
	if row.Amount != "1000000000000000000000000" || row.BlockHeight != "123" {
		t.Fatalf("row = %+v", row)
	}
	if len(p.Raw) == 0 {
		t.Fatal("raw not preserved")
	}
}

// TestQueryValidation rejects bad input before any request.
func TestQueryValidation(t *testing.T) {
	c := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request expected")
	})
	if _, err := c.Query(context.Background(), Query{Direction: DirectionSender}); err == nil {
		t.Fatal("want missing account_id error")
	}
	if _, err := c.Query(context.Background(), Query{AccountID: "a.near"}); err == nil {
		t.Fatal("want missing direction error")
	}
	if _, err := c.Query(context.Background(), Query{AccountID: "a.near", Direction: "both"}); err == nil {
		t.Fatal("want bad direction error")
	}
	bad := uint32(101)
	if _, err := c.Query(context.Background(), Query{AccountID: "a.near", Direction: DirectionSender, Limit: &bad}); err == nil {
		t.Fatal("want limit error")
	}
}

// TestIterator walks pages until an empty resume_token.
func TestIterator(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		next := "t" + string(rune('a'+calls))
		if calls == 2 {
			next = ""
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transfers":    []map[string]any{{"transaction_id": "H"}},
			"resume_token": next,
		})
	})
	it := c.Iterator(Query{AccountID: "a.near", Direction: DirectionSender}, 0)
	total := 0
	for {
		rows, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		total += len(rows)
	}
	if total != 2 || calls != 2 {
		t.Fatalf("total=%d calls=%d", total, calls)
	}
}
