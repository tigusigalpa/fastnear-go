package kv

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

func TestKVSequenceResumesAndCanBeReused(t *testing.T) {
	var tokens []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var query map[string]any
		_ = json.NewDecoder(r.Body).Decode(&query)
		token, _ := query["page_token"].(string)
		tokens = append(tokens, token)
		if token == "saved" {
			_, _ = w.Write([]byte(`{"entries":[{"key":"a"}],"page_token":"next"}`))
			return
		}
		_, _ = w.Write([]byte(`{"entries":[{"key":"b"}]}`))
	})
	start := "saved"
	seq := c.LatestByAccountSeq(context.Background(), "social.near", LatestQuery{PageToken: &start})
	for i := 0; i < 2; i++ {
		count := 0
		for _, err := range seq {
			if err != nil {
				t.Fatal(err)
			}
			count++
		}
		if count != 2 {
			t.Fatalf("run %d returned %d rows", i, count)
		}
	}
	if len(tokens) != 4 || tokens[0] != "saved" || tokens[2] != "saved" {
		t.Fatalf("resume tokens = %v", tokens)
	}
}

func TestKVIteratorCancellationAndCursorCycle(t *testing.T) {
	calls := 0
	it := NewIterator(func(_ *string) (*List, error) {
		calls++
		next := "a"
		if calls == 2 {
			next = "b"
		}
		return &List{PageToken: &next}, nil
	}, 10)
	for i := 0; i < 2; i++ {
		if _, _, err := it.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err := it.Next(context.Background())
	if e, ok := fastnearerr.As(err); !ok || e.Code != "pagination_loop" {
		t.Fatalf("cycle missed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	it = NewIterator(func(*string) (*List, error) { t.Fatal("called after cancellation"); return nil, nil }, 0)
	if _, _, err := it.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
