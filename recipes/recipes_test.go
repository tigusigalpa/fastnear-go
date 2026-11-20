package recipes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/tx"
)

// mux serves a mock of every family on one server; tests wire the
// root client with WithBaseURL per family.
func newRoot(t *testing.T, handler http.Handler) *fastnear.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := fastnear.NewClient(
		fastnear.WithAPIKey("k"),
		fastnear.WithMaxRetries(0),
		fastnear.WithBaseURL(fastnear.FamilyAccounts, srv.URL),
		fastnear.WithBaseURL(fastnear.FamilyRPC, srv.URL),
		fastnear.WithBaseURL(fastnear.FamilyTx, srv.URL),
		fastnear.WithBaseURL(fastnear.FamilyData, srv.URL),
		fastnear.WithBaseURL(fastnear.FamilyTransfers, srv.URL),
		fastnear.WithBaseURL(fastnear.FamilyKV, srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func rpcReply(result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": "x", "result": result}
}

// TestAccountSnapshot combines accounts.full + rpc.view_account.
func TestAccountSnapshot(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account/root.near/full" {
			json.NewEncoder(w).Encode(map[string]any{
				"account_id": "root.near",
				"tokens":     []any{map[string]any{"contract_id": "wrap.near", "balance": "5"}},
			})
			return
		}
		// RPC call
		json.NewEncoder(w).Encode(rpcReply(map[string]any{
			"amount": "1500000000000000000000000", "locked": "0",
			"storage_usage": 100, "block_height": 1, "block_hash": "H",
		}))
	}))
	snap, err := New(root).AccountSnapshot(context.Background(), "root.near")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.Full == nil || snap.OnChain == nil {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.Balance == nil || snap.Balance.NEAR() != "1.5" {
		t.Fatalf("balance = %v", snap.Balance)
	}
}

// TestFTBalance: indexed miss → ft_balance_of fallback.
func TestFTBalance(t *testing.T) {
	var sawCall bool
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account/a.near/ft" {
			json.NewEncoder(w).Encode(map[string]any{
				"account_id": "a.near", "tokens": []any{},
			})
			return
		}
		sawCall = true
		// call_function result: byte array of the JSON string "42"
		quoted := `"42"`
		bytes := make([]int, 0, len(quoted))
		for i := 0; i < len(quoted); i++ {
			bytes = append(bytes, int(quoted[i]))
		}
		json.NewEncoder(w).Encode(rpcReply(map[string]any{
			"result": bytes, "block_height": 1, "block_hash": "H",
		}))
	}))
	raw, found, err := New(root).FTBalance(context.Background(), "a.near", "wrap.near")
	if err != nil {
		t.Fatalf("FTBalance: %v", err)
	}
	if !found || raw != "42" || !sawCall {
		t.Fatalf("raw=%q found=%v sawCall=%v", raw, found, sawCall)
	}
}

// TestViewCallJSON decodes a JSON struct result.
func TestViewCallJSON(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{"name":"Wrapped NEAR","decimals":24}`
		bytes := make([]int, 0, len(payload))
		for i := 0; i < len(payload); i++ {
			bytes = append(bytes, int(payload[i]))
		}
		json.NewEncoder(w).Encode(rpcReply(map[string]any{
			"result": bytes, "block_height": 1, "block_hash": "H",
		}))
	}))
	var meta struct {
		Name     string `json:"name"`
		Decimals uint8  `json:"decimals"`
	}
	if err := New(root).ViewCallJSON(context.Background(), "wrap.near", "ft_metadata", nil, &meta); err != nil {
		t.Fatalf("view call: %v", err)
	}
	if meta.Name != "Wrapped NEAR" || meta.Decimals != 24 {
		t.Fatalf("meta = %+v", meta)
	}
}

// TestStateEntries follows last_key pagination.
func TestStateEntries(t *testing.T) {
	calls := 0
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		params := req["params"].(map[string]any)
		var lastKey any
		var values []map[string]any
		if params["after_key_base64"] == nil {
			values = []map[string]any{{"key": "azE=", "value": "djE="}}
			lastKey = "azI="
		} else {
			values = []map[string]any{{"key": "azM=", "value": "djM="}}
		}
		json.NewEncoder(w).Encode(rpcReply(map[string]any{
			"values": values, "last_key": lastKey,
			"block_height": 1, "block_hash": "H",
		}))
	}))
	n := 0
	for item, err := range New(root).StateEntries(context.Background(), "c.near", EncodePrefix("k"), nil) {
		if err != nil {
			t.Fatalf("seq err: %v", err)
		}
		if len(item.Key) == 0 {
			t.Fatal("empty key")
		}
		n++
	}
	if n != 2 || calls != 2 {
		t.Fatalf("n=%d calls=%d", n, calls)
	}
}

// TestKVValue decodes a stored JSON value.
func TestKVValue(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v0/latest/social.near/root.near/graph%2Ffollow%2Fx.near" {
			t.Errorf("path = %s", r.URL.EscapedPath())
		}
		json.NewEncoder(w).Encode(map[string]any{
			"entries": []map[string]any{{"key": "k", "value": map[string]any{"v": 7}}},
		})
	}))
	var v struct {
		V int `json:"v"`
	}
	found, err := New(root).KVValue(context.Background(), "social.near", "root.near", "graph/follow/x.near", &v)
	if err != nil || !found || v.V != 7 {
		t.Fatalf("found=%v v=%+v err=%v", found, v, err)
	}
}

// TestKVValueMissing: no entries → found=false.
func TestKVValueMissing(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"entries": []any{}})
	}))
	found, err := New(root).KVValue(context.Background(), "a.near", "b.near", "k", nil)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

// TestTxHistory streams history rows.
func TestTxHistory(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		next := ""
		if body["resume_token"] == nil {
			next = "p2"
		}
		json.NewEncoder(w).Encode(map[string]any{
			"account_txs":  []map[string]any{{"transaction_hash": "H"}},
			"resume_token": next, "txs_count": 1,
		})
	}))
	n := 0
	for _, err := range New(root).AccountTxHistory(context.Background(), tx.NewAccountHistoryQuery("a.near")) {
		if err != nil {
			t.Fatalf("seq err: %v", err)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("rows = %d", n)
	}
}

// TestTransferHistory streams transfer rows.
func TestTransferHistory(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"transfers":    []map[string]any{{"transaction_id": "H", "amount": "1"}},
			"resume_token": "",
		})
	}))
	n := 0
	for row, err := range New(root).TransferHistory(context.Background(), "a.near", "receiver") {
		if err != nil {
			t.Fatalf("seq err: %v", err)
		}
		if row.Amount != "1" {
			t.Fatalf("row = %+v", row)
		}
		n++
	}
	if n != 1 {
		t.Fatalf("rows = %d", n)
	}
}

// TestTokenRegistry resolves metadata once and formats balances.
func TestTokenRegistry(t *testing.T) {
	var methods []string
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account/a.near/ft" {
			json.NewEncoder(w).Encode(map[string]any{
				"account_id": "a.near",
				"tokens":     []any{map[string]any{"contract_id": "usdt.tether-token.near", "balance": "1500000"}},
			})
			return
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		params := req["params"].(map[string]any)
		methods = append(methods, params["method_name"].(string))
		var payload string
		switch params["method_name"] {
		case "ft_metadata":
			payload = `{"spec":"ft-1.0.0","name":"Tether USD","symbol":"USDt","decimals":6}`
		default:
			payload = `"0"`
		}
		bytes := make([]int, 0, len(payload))
		for i := 0; i < len(payload); i++ {
			bytes = append(bytes, int(payload[i]))
		}
		json.NewEncoder(w).Encode(rpcReply(map[string]any{
			"result": bytes, "block_height": 1, "block_hash": "H",
		}))
	}))
	reg := NewTokenRegistry(root)
	bal, found, err := reg.BalanceOf(context.Background(), "a.near", "usdt.tether-token.near")
	if err != nil || !found {
		t.Fatalf("bal=%+v found=%v err=%v", bal, found, err)
	}
	if bal.Meta.Symbol != "USDt" || bal.Format(2) != "1.50" {
		t.Fatalf("bal = %+v", bal)
	}
	// Second call must reuse the cache — only one ft_metadata call.
	if _, err := reg.Metadata(context.Background(), "usdt.tether-token.near"); err != nil {
		t.Fatalf("metadata: %v", err)
	}
	metaCalls := 0
	for _, m := range methods {
		if m == "ft_metadata" {
			metaCalls++
		}
	}
	if metaCalls != 1 {
		t.Fatalf("ft_metadata calls = %d", metaCalls)
	}
	if got, err := reg.FormatAmount(context.Background(), "usdt.tether-token.near", "1500000", 6); err != nil || got != "1.500000" {
		t.Fatalf("format = %q err=%v", got, err)
	}
}

// TestLatestFinalHeight parses the NEAR Data redirect.
func TestLatestFinalHeight(t *testing.T) {
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/v0/block/4242")
		w.WriteHeader(http.StatusFound)
	}))
	h, err := New(root).LatestFinalHeight(context.Background())
	if err != nil || h != 4242 {
		t.Fatalf("h=%d err=%v", h, err)
	}
}
