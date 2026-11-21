// Package transfers is the Transfers API client: indexed asset
// movement (native NEAR and FT) for an account. MAINNET ONLY —
// there is no testnet endpoint.
// Base URL: https://transfers.main.fastnear.com
// See https://docs.fastnear.com/transfers
package transfers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

const family = "transfers"

// Client is the Transfers API client. Obtain it from fastnear.Client;
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

// Query returns one page of asset transfers.
// See https://docs.fastnear.com/transfers/query
func (c *Client) Query(ctx context.Context, q Query) (*Page, error) {
	if err := c.guard("query"); err != nil {
		return nil, err
	}
	if err := q.validate(); err != nil {
		return nil, &fastnearerr.Error{Op: family + ".query", Family: family, Code: "validation_error", Message: err.Error()}
	}
	body, err := json.Marshal(q.marshal())
	if err != nil {
		return nil, &fastnearerr.Error{Op: family + ".query", Family: family, Code: "serialization_error", Message: "failed to encode request", Err: err}
	}
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodPost, Segments: []string{"v0", "transfers"}, Body: body, Op: family + ".query"})
	if err != nil {
		return nil, err
	}
	out := &Page{}
	ok, err := transport.DecodeJSON(resp.Body, out)
	if err != nil {
		return nil, transport.SerializationError(family+".query", family, resp.EffectiveURL, resp.Body, err)
	}
	if !ok {
		return nil, nil
	}
	out.SetRaw(resp.Body)
	return out, nil
}

// Iterator returns a paginated iterator over transfers.
// maxPages<=0 uses the default cap of 100.
func (c *Client) Iterator(q Query, maxPages int) *Iterator {
	if maxPages <= 0 {
		maxPages = 100
	}
	return &Iterator{client: c, query: q, maxPages: maxPages}
}
