package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

// Call is one entry of a JSON-RPC batch request.
type Call struct {
	// Method is the JSON-RPC method, e.g. "query" or "block".
	Method string
	// Params is the method's params object.
	Params any
	// Op optionally names the call for error reporting
	// (default "batch[i]").
	Op string
}

// BatchResult holds per-call results matched by request id. A failed
// call does not affect the others.
type BatchResult struct {
	// Responses[i] is the raw "result" of calls[i] (nil on error).
	Responses []json.RawMessage
	// Errors[i] is the *fastnearerr.Error of calls[i] (nil on success).
	Errors []*fastnearerr.Error
}

// Err returns a non-nil error when any call failed (the first one).
func (r *BatchResult) Err() error {
	for _, e := range r.Errors {
		if e != nil {
			return e
		}
	}
	return nil
}

// Batch sends several JSON-RPC calls in one HTTP request. Note that a
// batch costs the same number of FastNear credits as individual
// requests; it saves latency, not quota. A transport-level failure
// aborts the whole batch; JSON-RPC errors are reported per call.
func (c *Client) Batch(ctx context.Context, calls ...Call) (*BatchResult, error) {
	const op = "rpc.batch"
	if c == nil {
		return nil, fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return nil, &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	if c.t == nil {
		return nil, fastnearerr.Config(op, family, "uninitialized client")
	}
	if len(calls) == 0 {
		return &BatchResult{}, nil
	}
	if len(calls) > 100 {
		return nil, &fastnearerr.Error{Op: op, Family: family, Code: "validation_error", Message: "batch is limited to 100 calls"}
	}

	frames := make([]envelope, len(calls))
	ops := make([]string, len(calls))
	for i, call := range calls {
		frames[i] = envelope{
			JSONRPC: "2.0",
			ID:      "fastnear-" + strconv.FormatUint(c.ids.Add(1), 10),
			Method:  call.Method,
			Params:  call.Params,
		}
		ops[i] = call.Op
		if ops[i] == "" {
			ops[i] = "rpc.batch[" + strconv.Itoa(i) + "]"
		}
	}
	body, err := json.Marshal(frames)
	if err != nil {
		return nil, &fastnearerr.Error{Op: op, Family: family, Code: "serialization_error", Message: "failed to encode batch", Err: err}
	}
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodPost, Body: body, Op: op})
	if err != nil {
		return nil, err
	}

	var items []rpcResponse
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return nil, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
	}
	byID := make(map[string]rpcResponse, len(items))
	for _, item := range items {
		var id string
		if json.Unmarshal(item.ID, &id) == nil {
			byID[id] = item
		}
	}

	out := &BatchResult{
		Responses: make([]json.RawMessage, len(calls)),
		Errors:    make([]*fastnearerr.Error, len(calls)),
	}
	for i, frame := range frames {
		item, ok := byID[frame.ID]
		if !ok {
			out.Errors[i] = &fastnearerr.Error{Op: ops[i], Family: family, Code: "missing_response", Message: "no response for batch id " + frame.ID, RequestURL: resp.EffectiveURL}
			continue
		}
		if item.Error != nil {
			rpcErr := &fastnearerr.RPCError{
				Code:    item.Error.Code,
				Message: item.Error.Message,
				Data:    jsonx.Clone(item.Error.Data),
			}
			var meta struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(item.Error.Data, &meta) == nil {
				rpcErr.Name = meta.Name
			}
			out.Errors[i] = fastnearerr.NewRPC(ops[i], family, resp.EffectiveURL, rpcErr, item.Result)
			continue
		}
		out.Responses[i] = item.Result
	}
	return out, nil
}
