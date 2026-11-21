package rpc

import (
	"context"
	"encoding/json"
	"errors"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

type captured struct {
	path string
	env  struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
}

func writeRPCResponse(w http.ResponseWriter, r *http.Request, body string) {
	var request struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)
	var response map[string]any
	_ = json.Unmarshal([]byte(body), &response)
	response["id"] = request.ID
	_ = json.NewEncoder(w).Encode(response)
}

func newTestClient(t *testing.T, respond func(c captured) any) (*Client, *captured) {
	t.Helper()
	record := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record.path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&record.env)
		result := respond(*record)
		resp := map[string]any{"jsonrpc": "2.0", "id": record.env.ID, "result": result}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family,
		Timeout: 5 * time.Second, Retry: transport.RetryPolicy{MaxRetries: 0}, MaxBytes: 1 << 20,
	})
	return NewClient(tr, nil), record
}

func TestPostToRootAndEnvelope(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any {
		return map[string]any{"amount": "100", "locked": "0", "code_hash": "x", "storage_usage": 1, "storage_paid_at": 0, "block_height": 5, "block_hash": "h"}
	})
	_, err := c.ViewAccount(context.Background(), "root.near", nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.path != "/" {
		t.Fatalf("path = %q, want / (JSON-RPC posts to root)", record.path)
	}
	if record.env.JSONRPC != "2.0" || record.env.Method != "query" {
		t.Fatalf("bad envelope: %+v", record.env)
	}
	if !strings.HasPrefix(record.env.ID, "fastnear-") {
		t.Fatalf("id = %q", record.env.ID)
	}
	var p map[string]any
	_ = json.Unmarshal(record.env.Params, &p)
	if p["request_type"] != "view_account" || p["account_id"] != "root.near" || p["finality"] != "final" {
		t.Fatalf("params = %v", p)
	}
}

func TestBlockRefByHeight(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any {
		return map[string]any{"author": "a", "header": map[string]any{}, "chunks": []any{}}
	})
	_, err := c.ViewAccount(context.Background(), "root.near", &BlockRef{Height: uint64Ptr(42)})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	_ = json.Unmarshal(record.env.Params, &p)
	if p["block_id"] != float64(42) {
		if _, has := p["finality"]; has {
			t.Fatalf("finality must be absent when block_id set: %v", p)
		}
		t.Fatalf("params = %v", p)
	}
}

func TestGasPriceNullAndByBlock(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any { return map[string]any{"gas_price": "100000000"} })
	if _, err := c.GasPrice(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	var arr []any
	if err := json.Unmarshal(record.env.Params, &arr); err != nil || len(arr) != 1 || arr[0] != nil {
		t.Fatalf("gas_price params must be [null], got %s", record.env.Params)
	}
	if _, err := c.GasPrice(context.Background(), &BlockID{Height: uint64Ptr(7)}); err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	_ = json.Unmarshal(record.env.Params, &p)
	if p["block_id"] != float64(7) {
		t.Fatalf("params = %v", p)
	}
}

func TestCallFunctionArgsBase64(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any {
		return map[string]any{"result": []int{123, 34, 115, 117, 112, 112, 108, 121, 34, 58, 34, 49, 48, 34, 125}, "logs": []string{}, "block_height": 1, "block_hash": "h"}
	})
	res, err := c.CallFunction(context.Background(), CallFunctionRequest{
		AccountID: "wrap.near", MethodName: "ft_total_supply", Args: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	_ = json.Unmarshal(record.env.Params, &p)
	if p["args_base64"] != "e30=" {
		t.Fatalf("args_base64 = %v, want e30=", p["args_base64"])
	}
	if res.String() != `{"supply":"10"}` {
		t.Fatalf("result = %q", res.String())
	}
	var decoded map[string]string
	if err := res.JSON(&decoded); err != nil || decoded["supply"] != "10" {
		t.Fatalf("JSON() = %v %v", decoded, err)
	}
}

func TestRPCErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResponse(w, r, `{"jsonrpc":"2.0","id":"x","error":{"code":-32000,"message":"Server error","data":{"name":"HANDLER_ERROR","cause":{"info":{"account_id":"bad.near"}}}}}`)
	}))
	defer srv.Close()
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family, Timeout: 5 * time.Second,
		Retry:    transport.RetryPolicy{MaxRetries: 0},
		MaxBytes: 1 << 20,
	})
	c := NewClient(tr, nil)
	_, err := c.ViewAccount(context.Background(), "bad.near", nil)
	var e *fastnearerr.Error
	if !errors.As(err, &e) {
		t.Fatalf("not *Error: %T", err)
	}
	if e.RPC == nil || e.RPC.Code != -32000 || e.RPC.Name != "HANDLER_ERROR" {
		t.Fatalf("rpc error = %+v", e.RPC)
	}
}

func TestRPCServerErrorRetriedOnce(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			writeRPCResponse(w, r, `{"jsonrpc":"2.0","id":"x","error":{"code":-32000,"message":"Server error"}}`)
			return
		}
		writeRPCResponse(w, r, `{"jsonrpc":"2.0","id":"x","result":{"gas_price":"1"}}`)
	}))
	defer srv.Close()
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family, Timeout: 5 * time.Second,
		Retry:    transport.RetryPolicy{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, Budget: time.Second},
		MaxBytes: 1 << 20,
	})
	c := NewClient(tr, nil)
	gp, err := c.GasPrice(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success after one retry: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d", atomic.LoadInt32(&calls))
	}
	if gp.GasPrice != "1" {
		t.Fatalf("gp = %+v", gp)
	}
}

func TestSendTxNotRetriedOnHTTP500(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family, Timeout: 5 * time.Second,
		Retry:    transport.RetryPolicy{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, Budget: time.Second},
		MaxBytes: 1 << 20,
	})
	c := NewClient(tr, nil)
	_, err := c.SendTx(context.Background(), "AAAA", WaitNone)
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("send_tx retried %d times; must not retry", atomic.LoadInt32(&calls))
	}
}

func TestViewStatePagination(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any {
		return map[string]any{
			"values": []map[string]string{{"key": "QQ==", "value": "MQ=="}},
			"proof":  []any{}, "last_key": "QUJD", "block_height": 1, "block_hash": "h",
		}
	})
	lim := uint32(10)
	res, err := c.ViewState(context.Background(), ViewStateRequest{
		AccountID: "lockup.near", PrefixBase64: "U1RBVEU=", Limit: &lim,
	})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	_ = json.Unmarshal(record.env.Params, &p)
	if p["prefix_base64"] != "U1RBVEU=" || p["limit"] != float64(10) {
		t.Fatalf("params = %v", p)
	}
	if !res.HasMore() || *res.LastKey != "QUJD" {
		t.Fatalf("pagination: %+v", res)
	}
	entries, err := res.Entries()
	if err != nil || len(entries) != 1 || string(entries[0].Key) != "A" || string(entries[0].Value) != "1" {
		t.Fatalf("entries = %v %v", entries, err)
	}
}

func TestAccessKeyListLastKey(t *testing.T) {
	c, _ := newTestClient(t, func(_ captured) any {
		return map[string]any{
			"keys":     []map[string]any{{"public_key": "ed25519:abc", "access_key": map[string]any{"nonce": 1}}},
			"last_key": "K", "block_height": 1, "block_hash": "h",
		}
	})
	res, err := c.ViewAccessKeyList(context.Background(), "root.near", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasMore() || len(res.Keys) != 1 || res.Keys[0].PublicKey != "ed25519:abc" {
		t.Fatalf("res = %+v", res)
	}
}

func TestHealthOKAndUnhealthy(t *testing.T) {
	c, _ := newTestClient(t, func(_ captured) any { return nil })
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("healthy node must return nil: %v", err)
	}
	c2, _ := newTestClient(t, func(_ captured) any { return map[string]any{"detail": "no peers"} })
	if err := c2.Health(context.Background()); err == nil {
		t.Fatal("unhealthy node must return error")
	}
}

func TestMetricsIsPlainGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/metrics" {
			t.Errorf("metrics must be GET /metrics, got %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte("# HELP\nnear_peers 5\n"))
	}))
	defer srv.Close()
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family, Timeout: 5 * time.Second,
		Retry: transport.RetryPolicy{MaxRetries: 0}, MaxBytes: 1 << 20,
	})
	c := NewClient(tr, nil)
	m, err := c.Metrics(context.Background())
	if err != nil || !strings.Contains(m, "near_peers 5") {
		t.Fatalf("metrics = %q %v", m, err)
	}
}

func TestBroadcastTxAsyncReturnsHash(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any { return "HASH123" })
	h, err := c.BroadcastTxAsync(context.Background(), "AAAA")
	if err != nil || h != "HASH123" {
		t.Fatalf("h = %q %v", h, err)
	}
	if record.env.Method != "broadcast_tx_async" {
		t.Fatalf("method = %q", record.env.Method)
	}
}

func TestBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var frames []map[string]any
		_ = json.NewDecoder(r.Body).Decode(&frames)
		var out []any
		for _, f := range frames {
			if f["method"] == "gas_price" {
				out = append(out, map[string]any{"jsonrpc": "2.0", "id": f["id"], "result": map[string]any{"gas_price": "1"}})
			} else {
				out = append(out, map[string]any{"jsonrpc": "2.0", "id": f["id"], "error": map[string]any{"code": -32601, "message": "Method not found"}})
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family, Timeout: 5 * time.Second,
		Retry: transport.RetryPolicy{MaxRetries: 0}, MaxBytes: 1 << 20,
	})
	c := NewClient(tr, nil)
	res, err := c.Batch(context.Background(),
		Call{Method: "gas_price", Params: []any{nil}},
		Call{Method: "no_such_method", Params: []any{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Errors[0] != nil || res.Errors[1] == nil {
		t.Fatalf("errors = %v", res.Errors)
	}
	if res.Errors[1].RPC == nil || res.Errors[1].RPC.Code != -32601 {
		t.Fatalf("err1 = %+v", res.Errors[1])
	}
	if res.Err() == nil {
		t.Fatal("Err() must report the failure")
	}
}

func TestNoParamsAreEmptyArray(t *testing.T) {
	c, record := newTestClient(t, func(_ captured) any { return map[string]any{} })
	if _, err := c.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if string(record.env.Params) != "[]" {
		t.Fatalf("params must be [], got %s", record.env.Params)
	}
}

func TestHashHandleRejected(t *testing.T) {
	c, _ := newTestClient(t, func(_ captured) any { return nil })
	_, err := c.ViewAccessKey(context.Background(), "root.near", "ml-dsa-65-hash:ABC", nil)
	var e *fastnearerr.Error
	if !errors.As(err, &e) || e.Code != "validation_error" {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func uint64Ptr(v uint64) *uint64 { return &v }
