// Package kv is the KV FastData client: indexed key-value storage of
// NEAR contracts. Latest values, history by key/account/predecessor,
// and multi-key fetches.
// Base URL: https://kv.main.fastnear.com (testnet: https://kv.test.fastnear.com).
// See https://docs.fastnear.com/fastdata/kv
package kv

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
	"github.com/tigusigalpa/fastnear-go/internal/validate"
)

const family = "kv"

// Client is the KV FastData client. Obtain it from fastnear.Client;
// it is safe for concurrent use.
type Client struct {
	t   *transport.Client
	err error
}

// NewClient is used internally by fastnear.Client. The err parameter
// carries a configuration error that every method returns.
func NewClient(t *transport.Client, err error) *Client {
	return &Client{t: t, err: err}
}

type dtoPtr[T any] interface {
	*T
	SetRaw(json.RawMessage)
}

func (c *Client) guard(op string) error {
	op = family + "." + op
	if c == nil {
		return fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	if c.t == nil {
		return fastnearerr.Config(op, family, "uninitialized client")
	}
	return nil
}

func validateErr(op string, err error) error {
	return &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "validation_error", Message: err.Error()}
}

func (c *Client) request(ctx context.Context, op, method string, segs []string, query transport.Query, payload any) (*transport.Response, error) {
	if err := c.guard(op); err != nil {
		return nil, err
	}
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "serialization_error", Message: "failed to encode request", Err: err}
		}
	}
	return c.t.Do(ctx, transport.Request{Method: method, Segments: segs, Query: query, Body: body, Op: family + "." + op})
}

func decode[T any, P dtoPtr[T]](resp *transport.Response, op string) (*T, error) {
	out := P(new(T))
	ok, err := transport.DecodeJSON(resp.Body, out)
	if err != nil {
		return nil, transport.SerializationError(family+"."+op, family, resp.EffectiveURL, resp.Body, err)
	}
	if !ok {
		return nil, nil
	}
	out.SetRaw(resp.Body)
	return out, nil
}

// LatestByExactKey returns the latest value of one exact key.
// The key is sent as ONE encoded segment: "graph/follow/x.near"
// becomes "graph%2Ffollow%2Fx.near".
// See https://docs.fastnear.com/fastdata/kv/get-latest-key
func (c *Client) LatestByExactKey(ctx context.Context, contractID, predecessorID, key string, includeMetadata bool) (*List, error) {
	if err := validate.AccountID(contractID); err != nil {
		return nil, validateErr("latest_by_exact_key", err)
	}
	if err := validate.AccountID(predecessorID); err != nil {
		return nil, validateErr("latest_by_exact_key", err)
	}
	if key == "" {
		return nil, validateErr("latest_by_exact_key", errEmptyKey)
	}
	var q transport.Query
	if includeMetadata {
		q.Set("include_metadata", "true")
	}
	resp, err := c.request(ctx, "latest_by_exact_key", http.MethodGet,
		[]string{"v0", "latest", contractID, predecessorID, key}, q, nil)
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "latest_by_exact_key")
}

// HistoryByExactKey returns the write history of one exact key.
// See https://docs.fastnear.com/fastdata/kv/get-history-key
func (c *Client) HistoryByExactKey(ctx context.Context, contractID, predecessorID, key string, asc bool, limit *uint32, includeMetadata bool, pageToken *string) (*List, error) {
	if err := validate.AccountID(contractID); err != nil {
		return nil, validateErr("history_by_exact_key", err)
	}
	if err := validate.AccountID(predecessorID); err != nil {
		return nil, validateErr("history_by_exact_key", err)
	}
	if key == "" {
		return nil, validateErr("history_by_exact_key", errEmptyKey)
	}
	if limit != nil {
		if err := validate.Limit("limit", int(*limit), 1, 200); err != nil {
			return nil, validateErr("history_by_exact_key", err)
		}
	}
	var q transport.Query
	if asc {
		q.Set("asc", "true")
	}
	if limit != nil {
		q.Set("limit", strconv.FormatUint(uint64(*limit), 10))
	}
	if includeMetadata {
		q.Set("include_metadata", "true")
	}
	if pageToken != nil {
		q.Set("page_token", *pageToken)
	}
	resp, err := c.request(ctx, "history_by_exact_key", http.MethodGet,
		[]string{"v0", "history", contractID, predecessorID, key}, q, nil)
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "history_by_exact_key")
}

// LatestByAccount returns latest values written to a contract.
// See https://docs.fastnear.com/fastdata/kv/latest-by-account
func (c *Client) LatestByAccount(ctx context.Context, contractID string, q LatestQuery) (*List, error) {
	if err := validate.AccountID(contractID); err != nil {
		return nil, validateErr("latest_by_account", err)
	}
	if err := q.validate(); err != nil {
		return nil, validateErr("latest_by_account", err)
	}
	resp, err := c.request(ctx, "latest_by_account", http.MethodPost,
		[]string{"v0", "latest", contractID}, nil, q.marshal())
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "latest_by_account")
}

// LatestByPredecessor returns latest values written to a contract by
// one predecessor.
// See https://docs.fastnear.com/fastdata/kv/latest-by-predecessor
func (c *Client) LatestByPredecessor(ctx context.Context, contractID, predecessorID string, q LatestQuery) (*List, error) {
	if err := validate.AccountID(contractID); err != nil {
		return nil, validateErr("latest_by_predecessor", err)
	}
	if err := validate.AccountID(predecessorID); err != nil {
		return nil, validateErr("latest_by_predecessor", err)
	}
	if err := q.validate(); err != nil {
		return nil, validateErr("latest_by_predecessor", err)
	}
	resp, err := c.request(ctx, "latest_by_predecessor", http.MethodPost,
		[]string{"v0", "latest", contractID, predecessorID}, nil, q.marshal())
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "latest_by_predecessor")
}

// HistoryByAccount returns the write history of a contract.
// See https://docs.fastnear.com/fastdata/kv/history-by-account
func (c *Client) HistoryByAccount(ctx context.Context, contractID string, q HistoryQuery) (*List, error) {
	if err := validate.AccountID(contractID); err != nil {
		return nil, validateErr("history_by_account", err)
	}
	if err := q.validate(); err != nil {
		return nil, validateErr("history_by_account", err)
	}
	resp, err := c.request(ctx, "history_by_account", http.MethodPost,
		[]string{"v0", "history", contractID}, nil, q.marshal())
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "history_by_account")
}

// HistoryByPredecessor returns the write history of a contract by one
// predecessor.
// See https://docs.fastnear.com/fastdata/kv/history-by-predecessor
func (c *Client) HistoryByPredecessor(ctx context.Context, contractID, predecessorID string, q HistoryQuery) (*List, error) {
	if err := validate.AccountID(contractID); err != nil {
		return nil, validateErr("history_by_predecessor", err)
	}
	if err := validate.AccountID(predecessorID); err != nil {
		return nil, validateErr("history_by_predecessor", err)
	}
	if err := q.validate(); err != nil {
		return nil, validateErr("history_by_predecessor", err)
	}
	resp, err := c.request(ctx, "history_by_predecessor", http.MethodPost,
		[]string{"v0", "history", contractID, predecessorID}, nil, q.marshal())
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "history_by_predecessor")
}

// HistoryByKey returns the write history of a key across all
// contracts.
// See https://docs.fastnear.com/fastdata/kv/history-by-key
func (c *Client) HistoryByKey(ctx context.Context, key string, q HistoryQuery) (*List, error) {
	if key == "" {
		return nil, validateErr("history_by_key", errEmptyKey)
	}
	q.Key = &key
	if err := q.validate(); err != nil {
		return nil, validateErr("history_by_key", err)
	}
	resp, err := c.request(ctx, "history_by_key", http.MethodPost,
		[]string{"v0", "history"}, nil, q.marshal())
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "history_by_key")
}

// AllByPredecessor returns all latest values written by one
// predecessor across contracts.
// See https://docs.fastnear.com/fastdata/kv/all-by-predecessor
func (c *Client) AllByPredecessor(ctx context.Context, predecessorID string, limit *uint32, includeMetadata bool, pageToken *string) (*List, error) {
	if err := validate.AccountID(predecessorID); err != nil {
		return nil, validateErr("all_by_predecessor", err)
	}
	if limit != nil && (*limit < 1 || *limit > 200) {
		return nil, validateErr("all_by_predecessor", errLimit())
	}
	payload := map[string]any{}
	if limit != nil {
		payload["limit"] = *limit
	}
	if includeMetadata {
		payload["include_metadata"] = true
	}
	if pageToken != nil {
		payload["page_token"] = *pageToken
	}
	resp, err := c.request(ctx, "all_by_predecessor", http.MethodPost,
		[]string{"v0", "all", predecessorID}, nil, payload)
	if err != nil {
		return nil, err
	}
	return decode[List](resp, "all_by_predecessor")
}

// Multi fetches up to 100 keys in one call. Keys use the
// "contract/predecessor/key" form. Entries line up with the request
// order; a nil entry means "no value".
// See https://docs.fastnear.com/fastdata/kv/multi
func (c *Client) Multi(ctx context.Context, keys []string, includeMetadata bool) (*Multi, error) {
	if len(keys) == 0 {
		return nil, validateErr("multi", errEmptyKey)
	}
	if len(keys) > 100 {
		return nil, validateErr("multi", errTooManyKeys(len(keys)))
	}
	payload := map[string]any{"keys": keys}
	if includeMetadata {
		payload["include_metadata"] = true
	}
	resp, err := c.request(ctx, "multi", http.MethodPost, []string{"v0", "multi"}, nil, payload)
	if err != nil {
		return nil, err
	}
	return decode[Multi](resp, "multi")
}

type stringErr string

func (e stringErr) Error() string { return string(e) }

const errEmptyKey = stringErr("key must not be empty")

func errLimit() error { return stringErr("limit must be in [1, 200]") }

func errTooManyKeys(n int) error {
	return stringErr("keys is limited to 100, got " + strconv.Itoa(n))
}
