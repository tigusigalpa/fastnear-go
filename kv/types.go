package kv

import (
	"encoding/json"
	"fmt"

	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// Entry is one indexed key-value record of a contract.
// block_timestamp is nanoseconds as an integer (unlike Transfers,
// where it is a string). Value is arbitrary JSON.
type Entry struct {
	PredecessorID    string          `json:"predecessor_id"`
	CurrentAccountID string          `json:"current_account_id"`
	BlockHeight      uint64          `json:"block_height"`
	BlockTimestamp   uint64          `json:"block_timestamp"`
	Key              string          `json:"key"`
	Value            json.RawMessage `json:"value"`
	ActionIndex      uint32          `json:"action_index"`
	ReceiptID        string          `json:"receipt_id"`
	SignerID         string          `json:"signer_id"`
	TxHash           string          `json:"tx_hash"`
}

// ValueAs unmarshals the entry's arbitrary JSON value into v.
func (e *Entry) ValueAs(v any) error { return json.Unmarshal(e.Value, v) }

// ValueIsNull reports whether the stored value is a JSON null.
func (e *Entry) ValueIsNull() bool { return jsonx.IsNull(e.Value) }

// List is one page of entries. PageToken is ABSENT on the last page
// (unlike resume_token elsewhere, which is an empty string).
type List struct {
	jsonx.RawCarrier
	Entries   []Entry `json:"entries"`
	PageToken *string `json:"page_token"`
}

// HasMore reports whether a next page exists.
func (l *List) HasMore() bool { return l != nil && l.PageToken != nil }

// Items returns the page entries.
func (l *List) Items() []Entry { return l.Entries }

// NextToken returns the continuation token ("" = end).
func (l *List) NextToken() string {
	if l == nil || l.PageToken == nil {
		return ""
	}
	return *l.PageToken
}

// Multi is the /v0/multi response: one entry per requested key,
// possibly null when the key has no value.
type Multi struct {
	jsonx.RawCarrier
	Entries []*Entry `json:"entries"`
}

// LatestQuery is the body of POST /v0/latest/... endpoints.
// Key and KeyPrefix are mutually exclusive.
type LatestQuery struct {
	Key             *string
	KeyPrefix       *string
	Limit           *uint32 // 1..200; server default 50
	IncludeMetadata bool
	PageToken       *string
}

// HistoryQuery is the body of POST /v0/history/... endpoints.
type HistoryQuery struct {
	Key             *string
	KeyPrefix       *string
	Asc             bool
	Limit           *uint32 // 1..200; server default 50
	IncludeMetadata bool
	PageToken       *string
}

func (q LatestQuery) validate() error {
	if q.Key != nil && q.KeyPrefix != nil {
		return fmt.Errorf("key and key_prefix are mutually exclusive")
	}
	if q.Limit != nil && (*q.Limit < 1 || *q.Limit > 200) {
		return fmt.Errorf("limit must be in [1, 200], got %d", *q.Limit)
	}
	return nil
}

func (q HistoryQuery) validate() error {
	if q.Key != nil && q.KeyPrefix != nil {
		return fmt.Errorf("key and key_prefix are mutually exclusive")
	}
	if q.Limit != nil && (*q.Limit < 1 || *q.Limit > 200) {
		return fmt.Errorf("limit must be in [1, 200], got %d", *q.Limit)
	}
	return nil
}

func (q LatestQuery) marshal() map[string]any {
	m := map[string]any{}
	if q.Key != nil {
		m["key"] = *q.Key
	}
	if q.KeyPrefix != nil {
		m["key_prefix"] = *q.KeyPrefix
	}
	if q.Limit != nil {
		m["limit"] = *q.Limit
	}
	if q.IncludeMetadata {
		m["include_metadata"] = true
	}
	if q.PageToken != nil {
		m["page_token"] = *q.PageToken
	}
	return m
}

func (q HistoryQuery) marshal() map[string]any {
	m := map[string]any{}
	if q.Key != nil {
		m["key"] = *q.Key
	}
	if q.KeyPrefix != nil {
		m["key_prefix"] = *q.KeyPrefix
	}
	if q.Asc {
		m["asc"] = true
	}
	if q.Limit != nil {
		m["limit"] = *q.Limit
	}
	if q.IncludeMetadata {
		m["include_metadata"] = true
	}
	if q.PageToken != nil {
		m["page_token"] = *q.PageToken
	}
	return m
}
