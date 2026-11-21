package recipes

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

func TestStateEntriesStopsCursorCycles(t *testing.T) {
	calls := 0
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(rpcReply(r, map[string]any{
			"values": []any{}, "last_key": "c2FtZQ==", "block_height": 1,
		}))
	}))
	var lastErr error
	for _, err := range New(root).StateEntries(context.Background(), "c.near", "", nil) {
		lastErr = err
	}
	if e, ok := fastnearerr.As(lastErr); !ok || e.Code != "pagination_loop" || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, lastErr)
	}
}

func TestTokenRegistryReturnsIndependentMetadata(t *testing.T) {
	calls := 0
	root := newRoot(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		payload := `{"name":"Token","symbol":"TOK","icon":"original","decimals":6}`
		data := make([]int, len(payload))
		for i := range payload {
			data[i] = int(payload[i])
		}
		_ = json.NewEncoder(w).Encode(rpcReply(r, map[string]any{"result": data}))
	}))
	reg := NewTokenRegistry(root)
	first, err := reg.Metadata(context.Background(), "token.near")
	if err != nil {
		t.Fatal(err)
	}
	first.Decimals = 0
	*first.Icon = "changed"
	second, err := reg.Metadata(context.Background(), "token.near")
	if err != nil || second.Decimals != 6 || *second.Icon != "original" || calls != 1 {
		t.Fatalf("cache changed: %+v calls=%d err=%v", second, calls, err)
	}
}
