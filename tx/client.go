// Package tx is the Transactions API client: indexed history of
// transactions, blocks and receipts. All calls are POST with a JSON
// body. Base URL: https://tx.main.fastnear.com
// (testnet: https://tx.test.fastnear.com).
// See https://docs.fastnear.com/tx
package tx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

const family = "tx"

// Client is the Transactions API client. Obtain it from fastnear.Client;
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

// post performs a POST request with a JSON body and decodes a
// successful response into a new T. JSON null yields (nil, nil).
func post[T any, P dtoPtr[T]](ctx context.Context, c *Client, op string, segs []string, payload any) (*T, error) {
	op = family + "." + op
	if c == nil {
		return nil, fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return nil, &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	if c.t == nil {
		return nil, fastnearerr.Config(op, family, "uninitialized client")
	}
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, &fastnearerr.Error{Op: op, Family: family, Code: "serialization_error", Message: "failed to encode request", Err: err}
		}
	}
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodPost, Segments: segs, Body: body, Op: op})
	if err != nil {
		return nil, err
	}
	out := P(new(T))
	ok, err := transport.DecodeJSON(resp.Body, out)
	if err != nil {
		return nil, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
	}
	if !ok {
		return nil, nil
	}
	out.SetRaw(resp.Body)
	return out, nil
}

func validateErr(op string, err error) error {
	return &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "validation_error", Message: err.Error()}
}

// AccountHistory returns one page of the indexed account history.
// Use AccountHistoryIterator/AccountHistorySeq to walk all pages.
// See https://docs.fastnear.com/tx/account
func (c *Client) AccountHistory(ctx context.Context, q AccountHistoryQuery) (*AccountHistoryPage, error) {
	if err := validateQuery(q); err != nil {
		return nil, validateErr("account", err)
	}
	return post[AccountHistoryPage](ctx, c, "account", []string{"v0", "account"}, q.marshal())
}

// AccountHistoryIterator returns a paginated iterator over the
// account history. maxPages<=0 uses the default cap of 100.
func (c *Client) AccountHistoryIterator(q AccountHistoryQuery, maxPages int) *AccountHistoryIterator {
	if maxPages <= 0 {
		maxPages = 100
	}
	return &AccountHistoryIterator{client: c, query: q, maxPages: maxPages}
}

// Block returns one indexed block by height (uint64) or hash (string).
// block is null when the block is not indexed.
// See https://docs.fastnear.com/tx/block
func (c *Client) Block(ctx context.Context, blockID any, withTransactions, withReceipts bool) (*BlockPage, error) {
	var id any
	switch v := blockID.(type) {
	case uint64:
		id = v
	case int:
		if v < 0 {
			return nil, validateErr("block", errors.New("block_id must be >= 0"))
		}
		id = uint64(v)
	case int64:
		if v < 0 {
			return nil, validateErr("block", errors.New("block_id must be >= 0"))
		}
		id = uint64(v)
	case string:
		if v == "" {
			return nil, validateErr("block", errors.New("block_id must not be empty"))
		}
		id = v
	default:
		return nil, validateErr("block", fmt.Errorf("block_id must be a height (uint64) or a base58 hash (string), got %T", blockID))
	}
	payload := map[string]any{"block_id": id}
	if withTransactions {
		payload["with_transactions"] = true
	}
	if withReceipts {
		payload["with_receipts"] = true
	}
	return post[BlockPage](ctx, c, "block", []string{"v0", "block"}, payload)
}

// Blocks returns indexed blocks in a height range. limit is 1..100.
// See https://docs.fastnear.com/tx/blocks
func (c *Client) Blocks(ctx context.Context, q BlocksQuery) (*BlocksPage, error) {
	if q.Limit != nil && (*q.Limit < 1 || *q.Limit > 100) {
		return nil, validateErr("blocks", fmt.Errorf("limit must be in [1, 100], got %d", *q.Limit))
	}
	payload := map[string]any{}
	if q.FromBlockHeight != nil {
		payload["from_block_height"] = *q.FromBlockHeight
	}
	if q.ToBlockHeight != nil {
		payload["to_block_height"] = *q.ToBlockHeight
	}
	if q.Limit != nil {
		payload["limit"] = *q.Limit
	}
	if q.Desc {
		payload["desc"] = true
	}
	return post[BlocksPage](ctx, c, "blocks", []string{"v0", "blocks"}, payload)
}

// Receipt returns an indexed receipt; both fields are nil when the
// receipt is not found.
// See https://docs.fastnear.com/tx/receipt
func (c *Client) Receipt(ctx context.Context, receiptID string) (*ReceiptPage, error) {
	if receiptID == "" {
		return nil, validateErr("receipt", errors.New("receipt_id must not be empty"))
	}
	return post[ReceiptPage](ctx, c, "receipt", []string{"v0", "receipt"}, map[string]any{"receipt_id": receiptID})
}

// Transactions fetches up to 20 transactions by hash.
// See https://docs.fastnear.com/tx/transactions
func (c *Client) Transactions(ctx context.Context, txHashes []string) (*TransactionsPage, error) {
	if len(txHashes) == 0 {
		return nil, validateErr("transactions", errors.New("tx_hashes must not be empty"))
	}
	if len(txHashes) > 20 {
		return nil, validateErr("transactions", fmt.Errorf("tx_hashes is limited to 20, got %d", len(txHashes)))
	}
	return post[TransactionsPage](ctx, c, "transactions", []string{"v0", "transactions"}, map[string]any{"tx_hashes": txHashes})
}

// BlocksQuery describes /v0/blocks.
type BlocksQuery struct {
	FromBlockHeight *uint64
	ToBlockHeight   *uint64
	Limit           *uint32 // 1..100
	Desc            bool
}
