package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
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
	noRetry := false
	for i, call := range calls {
		if call.Method == "" {
			return nil, fastnearerr.Validation(op, family, "batch method must not be empty")
		}
		switch call.Method {
		case "send_tx", "broadcast_tx_async", "broadcast_tx_commit":
			noRetry = true
		}
		params := call.Params
		if params == nil {
			params = []any{}
		}
		frames[i] = envelope{
			JSONRPC: "2.0",
			ID:      "fastnear-" + strconv.FormatUint(c.ids.Add(1), 10),
			Method:  call.Method,
			Params:  params,
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
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodPost, Body: body, Op: op, NoRetry: noRetry})
	if err != nil {
		return nil, err
	}

	var items []rpcResponse
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return nil, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
	}
	byID := make(map[string]rpcResponse, len(items))
	expected := make(map[string]bool, len(frames))
	for _, frame := range frames {
		expected[frame.ID] = true
	}
	for _, item := range items {
		var id string
		if err := json.Unmarshal(item.ID, &id); err != nil {
			return nil, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
		}
		_, duplicate := byID[id]
		if !expected[id] || duplicate {
			return nil, fastnearerr.New(op, family, 200, "serialization_error", "unexpected or duplicate batch response id", resp.Body, nil)
		}
		if err := item.validate(id); err != nil {
			return nil, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
		}
		byID[id] = item
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
			out.Errors[i], _ = fastnearerr.As(item.rpcError(ops[i], resp.EffectiveURL, resp.Body))
			out.Errors[i] = c.t.RedactError(out.Errors[i])
			continue
		}
		out.Responses[i] = item.Result
	}
	return out, nil
}
