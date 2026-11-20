package tx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	tr := transport.New(transport.Params{
		BaseURL: srv.URL, APIKey: "k", Family: family,
		Timeout:  5 * time.Second,
		Retry:    transport.RetryPolicy{MaxRetries: 0},
		MaxBytes: 1 << 20,
	})
	return NewClient(tr, nil), srv
}

// TestAccountHistory verifies the POST body and page decoding.
func TestAccountHistory(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"account_txs": []map[string]any{{
				"account_id": "root.near", "transaction_hash": "H1",
				"tx_block_height": 100, "tx_block_timestamp": "1700000000000",
				"tx_index": 0, "is_signer": true, "is_success": true,
			}},
			"resume_token": "abc", "txs_count": 1,
		})
	})
	desc, signer, limit := true, true, uint32(10)
	page, err := c.AccountHistory(context.Background(), AccountHistoryQuery{
		AccountID: "root.near", Desc: &desc, IsSigner: &signer, Limit: &limit,
	})
	if err != nil {
		t.Fatalf("AccountHistory: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v0/account" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if gotBody["account_id"] != "root.near" || gotBody["desc"] != true ||
		gotBody["is_signer"] != true || gotBody["limit"] != float64(10) {
		t.Fatalf("body = %v", gotBody)
	}
	if page == nil || len(page.AccountTxs) != 1 || page.ResumeToken != "abc" || page.TxsCount != 1 {
		t.Fatalf("page = %+v", page)
	}
	if !page.HasMore() || page.NextToken() != "abc" {
		t.Fatal("HasMore/NextToken wrong")
	}
	if len(page.Raw) == 0 {
		t.Fatal("raw not preserved")
	}
}

// TestIteratorWalksPages follows resume_token until it comes back empty.
func TestIteratorWalksPages(t *testing.T) {
	var tokens []string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		tok, _ := body["resume_token"].(string)
		tokens = append(tokens, tok)
		if tok == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"account_txs":  []map[string]any{{"transaction_hash": "H1"}},
				"resume_token": "next", "txs_count": 1,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"account_txs":  []map[string]any{{"transaction_hash": "H2"}},
				"resume_token": "", "txs_count": 1,
			})
		}
	})
	it := c.AccountHistoryIterator(NewAccountHistoryQuery("root.near"), 0)
	var rows []AccountTxRow
	for {
		page, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		rows = append(rows, page...)
	}
	if len(rows) != 2 || rows[0].TransactionHash != "H1" || rows[1].TransactionHash != "H2" {
		t.Fatalf("rows = %+v", rows)
	}
	if len(tokens) != 2 || tokens[0] != "" || tokens[1] != "next" {
		t.Fatalf("tokens = %v", tokens)
	}
}

// TestSeq streams all rows via iter.Seq2.
func TestSeq(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		tok := ""
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		if s, ok := body["resume_token"].(string); ok {
			tok = s
		}
		next := "n" + tok
		if calls >= 3 {
			next = ""
		}
		json.NewEncoder(w).Encode(map[string]any{
			"account_txs":  []map[string]any{{"transaction_hash": "H"}},
			"resume_token": next, "txs_count": 1,
		})
	})
	n := 0
	for _, err := range c.AccountHistorySeq(context.Background(), NewAccountHistoryQuery("root.near")) {
		if err != nil {
			t.Fatalf("seq err: %v", err)
		}
		n++
	}
	if n != 3 {
		t.Fatalf("rows = %d", n)
	}
}

// TestIteratorLoopDetection fails on a repeated resume_token.
func TestIteratorLoopDetection(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"account_txs":  []map[string]any{{"transaction_hash": "H"}},
			"resume_token": "same", "txs_count": 1,
		})
	})
	it := c.AccountHistoryIterator(NewAccountHistoryQuery("a.near"), 0)
	if _, ok, err := it.Next(context.Background()); !ok || err != nil {
		t.Fatalf("page1: ok=%v err=%v", ok, err)
	}
	_, _, err := it.Next(context.Background())
	var fe *fastnearerr.Error
	if !errors.As(err, &fe) || fe.Code != "pagination_loop" {
		t.Fatalf("err = %v", err)
	}
}

// TestBlockByHeightAndHash checks the block_id variants.
func TestBlockByHeightAndHash(t *testing.T) {
	var ids []any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		ids = append(ids, body["block_id"])
		json.NewEncoder(w).Encode(map[string]any{
			"block": map[string]any{"block_height": 5, "block_hash": "BH"},
		})
	})
	p, err := c.Block(context.Background(), uint64(5), true, true)
	if err != nil || p == nil || p.Block == nil || p.Block.BlockHeight != 5 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if _, err := c.Block(context.Background(), "HashABC", false, false); err != nil {
		t.Fatalf("hash: %v", err)
	}
	if len(ids) != 2 || ids[0] != float64(5) || ids[1] != "HashABC" {
		t.Fatalf("ids = %v", ids)
	}
	if _, err := c.Block(context.Background(), 3.14, false, false); err == nil {
		t.Fatal("want validation error for float block_id")
	}
	if _, err := c.Block(context.Background(), "", false, false); err == nil {
		t.Fatal("want validation error for empty hash")
	}
}

// TestBlockNull: block not indexed → JSON null → nil page.
func TestBlockNull(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"block": null}`))
	})
	p, err := c.Block(context.Background(), uint64(1), false, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p == nil || p.Block != nil {
		t.Fatalf("p = %+v", p)
	}
}

// TestBlocks range query.
func TestBlocks(t *testing.T) {
	var got map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got)
		json.NewEncoder(w).Encode(map[string]any{"blocks": []any{
			map[string]any{"block_height": 10}, map[string]any{"block_height": 9},
		}})
	})
	from, to, lim := uint64(1), uint64(10), uint32(50)
	p, err := c.Blocks(context.Background(), BlocksQuery{FromBlockHeight: &from, ToBlockHeight: &to, Limit: &lim, Desc: true})
	if err != nil || p == nil || len(p.Blocks) != 2 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if got["from_block_height"] != float64(1) || got["to_block_height"] != float64(10) ||
		got["limit"] != float64(50) || got["desc"] != true {
		t.Fatalf("body = %v", got)
	}
	bad := uint32(0)
	if _, err := c.Blocks(context.Background(), BlocksQuery{Limit: &bad}); err == nil {
		t.Fatal("want limit validation error")
	}
}

// TestReceiptAndTransactions.
func TestReceiptAndTransactions(t *testing.T) {
	var paths []string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/v0/receipt":
			json.NewEncoder(w).Encode(map[string]any{
				"receipt":     map[string]any{"receipt_id": "R1"},
				"transaction": map[string]any{"signer_id": "a.near"},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"transactions": []any{map[string]any{"hash": "H"}, nil},
			})
		}
	})
	rp, err := c.Receipt(context.Background(), "R1")
	if err != nil || rp == nil || rp.Receipt == nil || rp.Receipt.ReceiptID != "R1" {
		t.Fatalf("rp=%+v err=%v", rp, err)
	}
	tp, err := c.Transactions(context.Background(), []string{"H1", "H2"})
	if err != nil || tp == nil || len(tp.Transactions) != 2 {
		t.Fatalf("tp=%+v err=%v", tp, err)
	}
	if len(paths) != 2 || paths[0] != "/v0/receipt" || paths[1] != "/v0/transactions" {
		t.Fatalf("paths = %v", paths)
	}
	if _, err := c.Receipt(context.Background(), ""); err == nil {
		t.Fatal("want empty receipt_id error")
	}
	if _, err := c.Transactions(context.Background(), nil); err == nil {
		t.Fatal("want empty hashes error")
	}
	hashes := make([]string, 21)
	if _, err := c.Transactions(context.Background(), hashes); err == nil {
		t.Fatal("want >20 hashes error")
	}
}
