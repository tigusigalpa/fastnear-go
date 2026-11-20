// Package data is the NEAR Data client: raw block/chunk/shard
// documents straight from the indexer. It is the heaviest surface —
// full blocks can reach tens of megabytes, so prefer BlockHeaders
// when you only need to follow the chain.
// Base URL: https://mainnet.neardata.xyz (testnet:
// https://testnet.neardata.xyz). MAINNET endpoints redirect to
// sibling mirror hosts (a0.mainnet.neardata.xyz, …) for archival
// blocks; the transport follows redirects within the configured
// base domain only.
// See https://docs.fastnear.com/neardata
package data

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

const family = "data"

// Client is the NEAR Data client. Obtain it from fastnear.Client;
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
	if c == nil || c.t == nil {
		return fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	return nil
}

// get issues a GET and decodes a successful response into a new T.
// noFollow returns the raw 3xx response instead of following it.
func get[T any, P dtoPtr[T]](c *Client, ctx context.Context, op string, segs []string, noFollow bool) (*T, *transport.Response, error) {
	if err := c.guard(op); err != nil {
		return nil, nil, err
	}
	resp, err := c.t.Do(ctx, transport.Request{
		Method:           http.MethodGet,
		Segments:         segs,
		NoFollowRedirect: noFollow,
		Op:               family + "." + op,
	})
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, resp, nil
	}
	out := P(new(T))
	ok, err := transport.DecodeJSON(resp.Body, out)
	if err != nil {
		return nil, nil, transport.SerializationError(family+"."+op, family, resp.EffectiveURL, resp.Body, err)
	}
	if !ok {
		return nil, resp, nil
	}
	out.SetRaw(resp.Body)
	return out, resp, nil
}

// Health probes the service.
// See https://docs.fastnear.com/neardata/system/health
func (c *Client) Health(ctx context.Context) (*Health, error) {
	h, _, err := get[Health](c, ctx, "health", []string{"health"}, false)
	return h, err
}

// FirstBlock returns the first indexed block document. Upstream
// answers 302 to the archived block; the transport follows it.
// See https://docs.fastnear.com/neardata/blocks/first
func (c *Client) FirstBlock(ctx context.Context) (*BlockDocument, error) {
	doc, _, err := get[BlockDocument](c, ctx, "first_block", []string{"v0", "first_block"}, false)
	return doc, err
}

// FirstBlockHeight returns only the height of the first indexed
// block, without downloading the (multi-megabyte) block document.
func (c *Client) FirstBlockHeight(ctx context.Context) (uint64, error) {
	return c.resolveHeight(ctx, "first_block", []string{"v0", "first_block"})
}

// LastBlockFinal returns the last FINAL block document.
// See https://docs.fastnear.com/neardata/blocks/last-final
func (c *Client) LastBlockFinal(ctx context.Context) (*BlockDocument, error) {
	doc, _, err := get[BlockDocument](c, ctx, "last_block_final", []string{"v0", "last_block", "final"}, false)
	return doc, err
}

// LastBlockOptimistic returns the last optimistic (not yet final)
// block document. Upstream answers 302 to /v0/block_opt/{h}.
// See https://docs.fastnear.com/neardata/blocks/last-optimistic
func (c *Client) LastBlockOptimistic(ctx context.Context) (*BlockDocument, error) {
	doc, _, err := get[BlockDocument](c, ctx, "last_block_optimistic", []string{"v0", "last_block", "optimistic"}, false)
	return doc, err
}

// LastFinalHeight returns the height of the last final block —
// the cheap way to poll for chain progress.
func (c *Client) LastFinalHeight(ctx context.Context) (uint64, error) {
	return c.resolveHeight(ctx, "last_block_final", []string{"v0", "last_block", "final"})
}

// LastOptimisticHeight returns the height of the last optimistic
// block without downloading the block document.
func (c *Client) LastOptimisticHeight(ctx context.Context) (uint64, error) {
	return c.resolveHeight(ctx, "last_block_optimistic", []string{"v0", "last_block", "optimistic"})
}

// resolveHeight asks for a locator endpoint WITHOUT following its
// redirect and reads the block height out of the Location header.
// If the endpoint answers 200 JSON instead, the height is decoded
// from the block document's header.
func (c *Client) resolveHeight(ctx context.Context, op string, segs []string) (uint64, error) {
	doc, resp, err := get[BlockDocument](c, ctx, op, segs, true)
	if err != nil {
		return 0, err
	}
	if resp != nil && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		h, err := parseLocationHeight(resp.Header.Get("Location"))
		if err != nil {
			return 0, &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "serialization_error", Message: "cannot parse redirect Location: " + err.Error(), RequestURL: resp.EffectiveURL}
		}
		return h, nil
	}
	if doc != nil {
		return doc.Block.Header.Height, nil
	}
	return 0, &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "serialization_error", Message: "empty response", RequestURL: resp.EffectiveURL}
}

// Block returns the full FINAL block document at height. If the
// height is ahead of the final head, upstream redirects to the
// optimistic variant — the transport follows, so the returned
// document is whatever the chain actually has (check
// doc.Block.Header.Height when it matters).
// See https://docs.fastnear.com/neardata/blocks/block
func (c *Client) Block(ctx context.Context, height uint64) (*BlockDocument, error) {
	doc, _, err := get[BlockDocument](c, ctx, "block", []string{"v0", "block", strconv.FormatUint(height, 10)}, false)
	return doc, err
}

// BlockOpt returns the optimistic (pre-final) block document at
// height. Faster than Block for fresh data, but the block may still
// be reorganized.
// See https://docs.fastnear.com/neardata/blocks/block-opt
func (c *Client) BlockOpt(ctx context.Context, height uint64) (*BlockDocument, error) {
	doc, _, err := get[BlockDocument](c, ctx, "block_opt", []string{"v0", "block_opt", strconv.FormatUint(height, 10)}, false)
	return doc, err
}

// BlockHeaders returns only the block header + chunk headers — the
// light-weight way to watch the chain (~KBs vs ~MBs for Block).
// See https://docs.fastnear.com/neardata/blocks/headers
func (c *Client) BlockHeaders(ctx context.Context, height uint64) (*BlockEnvelope, error) {
	env, _, err := get[BlockEnvelope](c, ctx, "headers", []string{"v0", "block", strconv.FormatUint(height, 10), "headers"}, false)
	return env, err
}

// Chunk returns the full chunk document (transactions + receipts) of
// shardID inside the block at height.
// See https://docs.fastnear.com/neardata/chunks/chunk
func (c *Client) Chunk(ctx context.Context, height, shardID uint64) (*ChunkDocument, error) {
	doc, _, err := get[ChunkDocument](c, ctx, "chunk", []string{"v0", "block", strconv.FormatUint(height, 10), "chunk", strconv.FormatUint(shardID, 10)}, false)
	return doc, err
}

// Shard returns a shard's payload of the block at height: chunk plus
// receipt execution outcomes and state changes.
// See https://docs.fastnear.com/neardata/shards/shard
func (c *Client) Shard(ctx context.Context, height, shardID uint64) (*ShardDocument, error) {
	doc, _, err := get[ShardDocument](c, ctx, "shard", []string{"v0", "block", strconv.FormatUint(height, 10), "shard", strconv.FormatUint(shardID, 10)}, false)
	return doc, err
}
