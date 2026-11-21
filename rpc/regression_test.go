package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

func TestRPCSubmissionAndReadRetryPolicies(t *testing.T) {
	for _, tc := range []struct {
		name           string
		write, enabled bool
		retries, want  int
	}{
		{"read-disabled", false, false, 0, 1},
		{"read-once", false, false, 3, 2},
		{"write-disabled", true, false, 3, 1},
		{"write-opted-in", true, true, 3, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				writeRPCResponse(w, r, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"Server error"}}`)
			}))
			defer srv.Close()
			c := NewClient(transport.New(transport.Params{BaseURL: srv.URL, HTTPClient: srv.Client(), Retry: transport.RetryPolicy{
				MaxRetries: tc.retries, RetryNonIdempotent: tc.enabled, BaseDelay: time.Millisecond,
			}}), nil)
			var err error
			if tc.write {
				_, err = c.SendTx(context.Background(), "AAAA", WaitNone)
			} else {
				_, err = c.GasPrice(context.Background(), nil)
			}
			if err == nil || calls != tc.want {
				t.Fatalf("calls=%d want=%d err=%v", calls, tc.want, err)
			}
		})
	}
}

func TestContractDefaultsAndBlockReferenceWireFormat(t *testing.T) {
	c, captured := newTestClient(t, func(_ captured) any { return map[string]any{"result": []int{}} })
	if _, err := c.CallFunction(context.Background(), CallFunctionRequest{AccountID: "wrap.near", MethodName: "ft_metadata"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(captured.env.Params), `"args_base64":"e30="`) {
		t.Fatalf("nil args are not {}: %s", captured.env.Params)
	}
	ref := AtHeight(42)
	if _, err := c.ViewAccessKeyList(context.Background(), "a.near", &AccessKeyListOptions{At: &ref}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(captured.env.Params), "finality") || !strings.Contains(string(captured.env.Params), `"block_id":42`) {
		t.Fatalf("conflicting reference: %s", captured.env.Params)
	}
	if _, err := c.ExperimentalProtocolConfig(context.Background(), &ref); err != nil {
		t.Fatal(err)
	}
	if string(captured.env.Params) != `{"block_id":42}` {
		t.Fatalf("unexpected protocol params: %s", captured.env.Params)
	}
}

func TestInvalidRPCEnvelopesAreErrors(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"jsonrpc":"2.0","id":"wrong","result":null}`, `{"jsonrpc":"1.0","id":"fastnear-1","result":null}`, `{"jsonrpc":"2.0","id":"fastnear-1"}`, `{"jsonrpc":"2.0","id":"fastnear-1","result":null,"error":{"code":-32000}}`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			c := NewClient(transport.New(transport.Params{BaseURL: srv.URL, HTTPClient: srv.Client()}), nil)
			_, err := c.Status(context.Background())
			if e, ok := fastnearerr.As(err); !ok || e.Code != "serialization_error" {
				t.Fatalf("invalid response accepted: %v", err)
			}
		})
	}
}

func TestBatchSubmissionIsNotRetried(t *testing.T) {
	for _, method := range []string{"send_tx", "broadcast_tx_async", "broadcast_tx_commit"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(503) }))
			defer srv.Close()
			c := NewClient(transport.New(transport.Params{BaseURL: srv.URL, HTTPClient: srv.Client(), Retry: transport.DefaultRetryPolicy()}), nil)
			if _, err := c.Batch(context.Background(), Call{Method: method}); err == nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestBatchRejectsDuplicateIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []envelope
		_ = json.NewDecoder(r.Body).Decode(&requests)
		frame := map[string]any{"jsonrpc": "2.0", "id": requests[0].ID, "result": nil}
		_ = json.NewEncoder(w).Encode([]any{frame, frame})
	}))
	defer srv.Close()
	c := NewClient(transport.New(transport.Params{BaseURL: srv.URL, HTTPClient: srv.Client()}), nil)
	if _, err := c.Batch(context.Background(), Call{Method: "status"}); err == nil {
		t.Fatal("duplicate IDs accepted")
	}
}

func TestBatchErrorRedactsReflectedCredentials(t *testing.T) {
	const key = "private-batch-key-1234"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requests []envelope
		_ = json.NewDecoder(r.Body).Decode(&requests)
		_ = json.NewEncoder(w).Encode([]any{map[string]any{"jsonrpc": "2.0", "id": requests[0].ID, "error": map[string]any{"code": -32602, "message": key, "data": map[string]any{"key": key}}}})
	}))
	defer srv.Close()
	c := NewClient(transport.New(transport.Params{BaseURL: srv.URL, HTTPClient: srv.Client(), APIKey: key}), nil)
	result, err := c.Batch(context.Background(), Call{Method: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Err() == nil {
		t.Fatal("missing RPC error")
	}
	if strings.Contains(result.Err().Error(), key) || strings.Contains(string(result.Errors[0].RawBody), key) || strings.Contains(string(result.Errors[0].RPC.Data), key) {
		t.Fatal("batch error exposed credentials")
	}
}

func TestAggregateStatusAndNestedExecutionOutcome(t *testing.T) {
	var result FinalExecutionOutcome
	err := json.Unmarshal([]byte(`{"final_execution_status":"FINAL","status":{"Failure":{"ActionError":{}}},"transaction_outcome":{"id":"tx","outcome":{"status":{"SuccessReceiptId":"receipt"},"gas_burnt":12,"logs":["accepted"]}},"receipts_outcome":[{"id":"receipt","outcome":{"status":{"Failure":{}},"gas_burnt":34}}]}`), &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status() == nil || result.Status().IsSuccess() || len(result.Status().Failure) == 0 {
		t.Fatal("receipt failure reported as success")
	}
	if result.TransactionOutcome.GasBurnt != 12 || result.TransactionOutcome.Outcome == nil || result.ReceiptsOutcome[0].GasBurnt != 34 {
		t.Fatal("nested outcomes lost")
	}
	for _, state := range []string{`"Unknown"`, `"NotStarted"`, `"Started"`} {
		var status ExecutionStatus
		if err := json.Unmarshal([]byte(state), &status); err != nil || status.IsSuccess() {
			t.Fatalf("unit status: %+v %v", status, err)
		}
	}
	var call CallResult
	if err := json.Unmarshal([]byte(`{"result":[256]}`), &call); err == nil {
		t.Fatal("overflowing byte accepted")
	}
}

func TestArrayResults(t *testing.T) {
	c, _ := newTestClient(t, func(request captured) any {
		if request.env.Method == "maintenance_windows" {
			return []any{map[string]any{"start": 12, "end": 34}}
		}
		return []any{map[string]any{"account_id": "validator.near", "stake": "10"}}
	})
	windows, err := c.MaintenanceWindows(context.Background(), "validator.near")
	if err != nil || len(windows.Ranges) != 1 || windows.Ranges[0].Start != 12 || windows.Ranges[0].End != 34 {
		t.Fatalf("windows=%+v err=%v", windows, err)
	}
	validators, err := c.ExperimentalValidatorsOrdered(context.Background(), nil)
	if err != nil || len(validators.Validators) != 1 || validators.Validators[0].AccountID != "validator.near" {
		t.Fatalf("validators=%+v err=%v", validators, err)
	}
}
