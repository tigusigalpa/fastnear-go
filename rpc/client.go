// Package rpc is the canonical NEAR JSON-RPC client. Every call is a
// POST to the base URL root with {"jsonrpc","id","method","params"};
// archival access is a client-level profile (WithArchival), not a
// separate surface.
// See https://docs.fastnear.com/rpc
package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
	"github.com/tigusigalpa/fastnear-go/internal/validate"
)

const family = "rpc"

// Client is the JSON-RPC client. Obtain it from fastnear.Client; it is
// safe for concurrent use.
type Client struct {
	t   *transport.Client
	err error
	ids atomic.Uint64
}

// NewClient is used internally by fastnear.Client. The err parameter
// carries a configuration error that every method returns.
func NewClient(t *transport.Client, err error) *Client {
	return &Client{t: t, err: err}
}

// envelope is the JSON-RPC 2.0 request frame.
type envelope struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// rpcResponse is the JSON-RPC 2.0 response frame.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

// dtoPtr is a pointer to a DTO that can store its raw body.
type dtoPtr[T any] interface {
	*T
	SetRaw(json.RawMessage)
}

// call executes one JSON-RPC method and returns the raw "result".
// A JSON-RPC -32000 server error is retried exactly once.
func (c *Client) call(ctx context.Context, op, method string, params any, noRetry bool) (json.RawMessage, string, error) {
	op = family + "." + op
	if c == nil {
		return nil, "", fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return nil, "", &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	if c.t == nil {
		return nil, "", fastnearerr.Config(op, family, "uninitialized client")
	}

	retried := false
	for {
		result, reqURL, err := c.callOnce(ctx, op, method, params, noRetry)
		if err == nil {
			return result, reqURL, nil
		}
		var e *fastnearerr.Error
		if retried || !asError(err, &e) || e.RPC == nil || e.RPC.Code != -32000 {
			return nil, reqURL, err
		}
		retried = true
	}
}

func asError(err error, out **fastnearerr.Error) bool {
	e, ok := fastnearerr.As(err)
	if !ok {
		return false
	}
	*out = e
	return true
}

// callOnce performs a single JSON-RPC round trip.
func (c *Client) callOnce(ctx context.Context, op, method string, params any, noRetry bool) (json.RawMessage, string, error) {
	if params == nil {
		params = []any{}
	}
	id := "fastnear-" + strconv.FormatUint(c.ids.Add(1), 10)
	body, err := json.Marshal(envelope{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, "", &fastnearerr.Error{Op: op, Family: family, Code: "serialization_error", Message: "failed to encode request", Err: err}
	}
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodPost, Body: body, Op: op, NoRetry: noRetry})
	if err != nil {
		return nil, "", err
	}
	var frame rpcResponse
	if err := json.Unmarshal(resp.Body, &frame); err != nil {
		return nil, resp.EffectiveURL, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
	}
	if frame.Error != nil {
		rpcErr := &fastnearerr.RPCError{
			Code:    frame.Error.Code,
			Message: frame.Error.Message,
			Data:    jsonx.Clone(frame.Error.Data),
		}
		var meta struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(frame.Error.Data, &meta) == nil {
			rpcErr.Name = meta.Name
		}
		return nil, resp.EffectiveURL, fastnearerr.NewRPC(op, family, resp.EffectiveURL, rpcErr, resp.Body)
	}
	return frame.Result, resp.EffectiveURL, nil
}

// query is a shorthand for call with method "query".
func (c *Client) query(ctx context.Context, op string, params map[string]any) (json.RawMessage, string, error) {
	return c.call(ctx, op, "query", params, false)
}

// decodeResult unmarshals the raw result into a new T DTO.
func decodeResult[T any, P dtoPtr[T]](op, reqURL string, result json.RawMessage) (*T, error) {
	if jsonx.IsNull(result) {
		return nil, nil
	}
	out := P(new(T))
	if err := json.Unmarshal(result, out); err != nil {
		return nil, transport.SerializationError(family+"."+op, family, reqURL, result, err)
	}
	out.SetRaw(result)
	return out, nil
}

func validateErr(op string, err error) error {
	return &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "validation_error", Message: err.Error()}
}

// queryParams builds `query` params for the given request_type.
func queryParams(requestType string, at *BlockRef) map[string]any {
	p := map[string]any{"request_type": requestType}
	mergeBlockRef(p, at)
	return p
}

// --- Accounts and keys -------------------------------------------------

// ViewAccount returns the canonical account state at a block.
// See https://docs.fastnear.com/rpc/account/view-account
func (c *Client) ViewAccount(ctx context.Context, accountID string, at *BlockRef) (*ViewAccountResult, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("view_account", err)
	}
	p := queryParams("view_account", at)
	p["account_id"] = accountID
	result, u, err := c.query(ctx, "view_account", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[ViewAccountResult]("view_account", u, result)
}

// ViewAccessKey returns a single access key. Requires the FULL public
// key; ml-dsa-65-hash handles are rejected by the node.
// See https://docs.fastnear.com/rpc/account/view-access-key
func (c *Client) ViewAccessKey(ctx context.Context, accountID, publicKey string, at *BlockRef) (*AccessKeyView, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("view_access_key", err)
	}
	if err := validate.PublicKey(publicKey); err != nil {
		return nil, validateErr("view_access_key", err)
	}
	if len(publicKey) >= len("ml-dsa-65-hash:") && publicKey[:len("ml-dsa-65-hash:")] == "ml-dsa-65-hash:" {
		return nil, validateErr("view_access_key", errHashHandle)
	}
	p := queryParams("view_access_key", at)
	p["account_id"] = accountID
	p["public_key"] = publicKey
	result, u, err := c.query(ctx, "view_access_key", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[AccessKeyView]("view_access_key", u, result)
}

var errHashHandle = errorString("ml-dsa-65-hash: handles are not accepted here; pass the full ml-dsa-65: public key")

type errorString string

func (e errorString) Error() string { return string(e) }

// ViewAccessKeyList returns one page of the account's access keys.
// See https://docs.fastnear.com/rpc/account/view-access-key-list
func (c *Client) ViewAccessKeyList(ctx context.Context, accountID string, opts *AccessKeyListOptions) (*AccessKeyList, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("view_access_key_list", err)
	}
	var at *BlockRef
	p := queryParams("view_access_key_list", nil)
	if opts != nil {
		if opts.AfterKey != nil {
			p["after_key"] = *opts.AfterKey
		}
		if opts.Limit != nil {
			p["limit"] = *opts.Limit
		}
		at = opts.At
	}
	mergeBlockRef(p, at)
	p["account_id"] = accountID
	result, u, err := c.query(ctx, "view_access_key_list", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[AccessKeyList]("view_access_key_list", u, result)
}

// --- Blocks and chunks -------------------------------------------------

// Block returns a block by height, hash or finality.
// See https://docs.fastnear.com/rpc/block/block-by-height and
// https://docs.fastnear.com/rpc/block/block-by-id
func (c *Client) Block(ctx context.Context, ref BlockRef) (*BlockView, error) {
	params := ref.queryParams()
	result, u, err := c.call(ctx, "block", "block", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[BlockView]("block", u, result)
}

// BlockByID returns a block by height or hash.
// See https://docs.fastnear.com/rpc/block/block-by-id
func (c *Client) BlockByID(ctx context.Context, id BlockID) (*BlockView, error) {
	result, u, err := c.call(ctx, "block", "block", map[string]any{"block_id": id.value()}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[BlockView]("block", u, result)
}

// LatestBlock returns the latest block at the given finality.
// See https://docs.fastnear.com/rpc/protocol/latest-block
func (c *Client) LatestBlock(ctx context.Context, finality Finality) (*BlockView, error) {
	if finality == "" {
		finality = FinalityFinal
	}
	result, u, err := c.call(ctx, "latest_block", "block", map[string]any{"finality": string(finality)}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[BlockView]("latest_block", u, result)
}

// BlockEffects returns the transaction effects of a block.
// See https://docs.fastnear.com/rpc/block/block-effects
func (c *Client) BlockEffects(ctx context.Context, id BlockID) (*BlockEffects, error) {
	result, u, err := c.call(ctx, "block_effects", "block_effects", map[string]any{"block_id": id.value()}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[BlockEffects]("block_effects", u, result)
}

// Chunk returns a chunk by its hash.
// See https://docs.fastnear.com/rpc/protocol/chunk-by-hash
func (c *Client) Chunk(ctx context.Context, chunkHash string) (*ChunkView, error) {
	if chunkHash == "" {
		return nil, validateErr("chunk", errEmptyArg("chunk_hash"))
	}
	result, u, err := c.call(ctx, "chunk", "chunk", map[string]any{"chunk_id": chunkHash}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ChunkView]("chunk", u, result)
}

// ChunkByBlockShard returns the chunk of a block's shard.
// See https://docs.fastnear.com/rpc/protocol/chunk-by-block-shard
func (c *Client) ChunkByBlockShard(ctx context.Context, id BlockID, shardID uint32) (*ChunkView, error) {
	params := map[string]any{"block_id": id.value(), "shard_id": shardID}
	result, u, err := c.call(ctx, "chunk", "chunk", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ChunkView]("chunk", u, result)
}

// --- Contracts ---------------------------------------------------------

// CallFunction invokes a contract view method with JSON args.
// See https://docs.fastnear.com/rpc/contract/call-function
func (c *Client) CallFunction(ctx context.Context, req CallFunctionRequest) (*CallResult, error) {
	if err := validate.AccountID(req.AccountID); err != nil {
		return nil, validateErr("call_function", err)
	}
	if req.MethodName == "" {
		return nil, validateErr("call_function", errEmptyArg("method_name"))
	}
	argsJSON, err := json.Marshal(req.Args)
	if err != nil {
		return nil, &fastnearerr.Error{Op: family + ".call_function", Family: family, Code: "serialization_error", Message: "failed to encode args", Err: err}
	}
	return c.callFunction64(ctx, req.AccountID, req.MethodName, base64.StdEncoding.EncodeToString(argsJSON), req.At, "call_function")
}

// CallFunctionRaw invokes a view method with pre-encoded base64 args.
// See https://docs.fastnear.com/rpc/contract/call-function
func (c *Client) CallFunctionRaw(ctx context.Context, req CallFunctionRawRequest) (*CallResult, error) {
	if err := validate.AccountID(req.AccountID); err != nil {
		return nil, validateErr("call_function", err)
	}
	if req.MethodName == "" {
		return nil, validateErr("call_function", errEmptyArg("method_name"))
	}
	if _, err := base64.StdEncoding.DecodeString(req.ArgsBase64); err != nil {
		return nil, validateErr("call_function", errorString("args_base64 is not valid base64"))
	}
	return c.callFunction64(ctx, req.AccountID, req.MethodName, req.ArgsBase64, req.At, "call_function")
}

func (c *Client) callFunction64(ctx context.Context, accountID, method, args64 string, at *BlockRef, op string) (*CallResult, error) {
	p := queryParams("call_function", at)
	p["account_id"] = accountID
	p["method_name"] = method
	p["args_base64"] = args64
	result, u, err := c.query(ctx, op, p)
	if err != nil {
		return nil, err
	}
	return decodeResult[CallResult](op, u, result)
}

// ViewState scans contract storage under a base64 key prefix.
// See https://docs.fastnear.com/rpc/contract/view-state
func (c *Client) ViewState(ctx context.Context, req ViewStateRequest) (*ViewStateResult, error) {
	if err := validate.AccountID(req.AccountID); err != nil {
		return nil, validateErr("view_state", err)
	}
	p := queryParams("view_state", req.At)
	p["account_id"] = req.AccountID
	p["prefix_base64"] = req.PrefixBase64
	if req.AfterKeyBase64 != nil {
		p["after_key_base64"] = *req.AfterKeyBase64
	}
	if req.Limit != nil {
		p["limit"] = *req.Limit
	}
	if req.IncludeProof {
		p["include_proof"] = true
	}
	result, u, err := c.query(ctx, "view_state", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[ViewStateResult]("view_state", u, result)
}

// ViewCode returns deployed contract code for an account.
// See https://docs.fastnear.com/rpc/contract/view-code
func (c *Client) ViewCode(ctx context.Context, accountID string, at *BlockRef) (*ContractCodeView, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("view_code", err)
	}
	p := queryParams("view_code", at)
	p["account_id"] = accountID
	result, u, err := c.query(ctx, "view_code", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[ContractCodeView]("view_code", u, result)
}

// ViewGlobalContractCode returns code deployed globally by hash.
// See https://docs.fastnear.com/rpc/contract/view-global-contract-code
func (c *Client) ViewGlobalContractCode(ctx context.Context, codeHash string, at *BlockRef) (*ContractCodeView, error) {
	if codeHash == "" {
		return nil, validateErr("view_global_contract_code", errEmptyArg("code_hash"))
	}
	p := queryParams("view_global_contract_code", at)
	p["code_hash"] = codeHash
	result, u, err := c.query(ctx, "view_global_contract_code", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[ContractCodeView]("view_global_contract_code", u, result)
}

// ViewGlobalContractCodeByAccountID resolves global contract code by
// the account that references it.
// See https://docs.fastnear.com/rpc/contract/view-global-contract-code-by-account-id
func (c *Client) ViewGlobalContractCodeByAccountID(ctx context.Context, accountID string, at *BlockRef) (*ContractCodeView, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("view_global_contract_code_by_account_id", err)
	}
	p := queryParams("view_global_contract_code_by_account_id", at)
	p["account_id"] = accountID
	result, u, err := c.query(ctx, "view_global_contract_code_by_account_id", p)
	if err != nil {
		return nil, err
	}
	return decodeResult[ContractCodeView]("view_global_contract_code_by_account_id", u, result)
}

// --- Protocol and diagnostics ------------------------------------------

// GasPrice returns the gas price; nil queries the latest block.
// See https://docs.fastnear.com/rpc/protocol/gas-price and
// https://docs.fastnear.com/rpc/protocol/gas-price-by-block
func (c *Client) GasPrice(ctx context.Context, id *BlockID) (*GasPrice, error) {
	var params any
	if id == nil {
		params = []any{nil}
	} else {
		params = map[string]any{"block_id": id.value()}
	}
	result, u, err := c.call(ctx, "gas_price", "gas_price", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[GasPrice]("gas_price", u, result)
}

// Health returns nil when the node is healthy.
// See https://docs.fastnear.com/rpc/protocol/health
func (c *Client) Health(ctx context.Context) error {
	result, u, err := c.call(ctx, "health", "health", nil, false)
	if err != nil {
		return err
	}
	if !jsonx.IsNull(result) {
		return &fastnearerr.Error{Op: family + ".health", Family: family, Code: "node_unhealthy", Message: "node reports unhealthy state", RequestURL: u, RawBody: result}
	}
	return nil
}

// Metrics returns the Prometheus metrics document as text (not JSON).
// See https://docs.fastnear.com/rpc/protocol/metrics
func (c *Client) Metrics(ctx context.Context) (string, error) {
	op := family + ".metrics"
	if c == nil || c.t == nil {
		return "", fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return "", &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodGet, Segments: []string{"metrics"}, Op: op})
	if err != nil {
		return "", err
	}
	return string(resp.Body), nil
}

// NetworkInfo returns the node network view.
// See https://docs.fastnear.com/rpc/protocol/network-info
func (c *Client) NetworkInfo(ctx context.Context) (*NetworkInfo, error) {
	result, u, err := c.call(ctx, "network_info", "network_info", nil, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[NetworkInfo]("network_info", u, result)
}

// Status returns the node status.
// See https://docs.fastnear.com/rpc/protocol/status
func (c *Client) Status(ctx context.Context) (*NodeStatus, error) {
	result, u, err := c.call(ctx, "status", "status", nil, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[NodeStatus]("status", u, result)
}

// GenesisConfig returns the chain genesis configuration.
// See https://docs.fastnear.com/rpc/protocol/genesis-config
func (c *Client) GenesisConfig(ctx context.Context) (*GenesisConfig, error) {
	result, u, err := c.call(ctx, "genesis_config", "genesis_config", nil, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[GenesisConfig]("genesis_config", u, result)
}

// ClientConfig returns the node client configuration.
// See https://docs.fastnear.com/rpc/protocol/client-config
func (c *Client) ClientConfig(ctx context.Context) (*ClientConfig, error) {
	result, u, err := c.call(ctx, "client_config", "client_config", nil, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ClientConfig]("client_config", u, result)
}

// Changes runs a `changes` query built by AccountChanges,
// DataChanges or AllAccessKeyChanges.
// See https://docs.fastnear.com/rpc/protocol/changes
func (c *Client) Changes(ctx context.Context, q StateChangesQuery) (*StateChanges, error) {
	if q.params == nil {
		return nil, validateErr("changes", errorString("empty query; build it with AccountChanges/DataChanges/AllAccessKeyChanges"))
	}
	result, u, err := c.call(ctx, "changes", "changes", q.params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[StateChanges]("changes", u, result)
}

// MaintenanceWindows returns the maintenance windows of a validator.
// See https://docs.fastnear.com/rpc/protocol/maintenance-windows
func (c *Client) MaintenanceWindows(ctx context.Context, accountID string) (*MaintenanceWindows, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("maintenance_windows", err)
	}
	result, u, err := c.call(ctx, "maintenance_windows", "maintenance_windows", map[string]any{"account_id": accountID}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[MaintenanceWindows]("maintenance_windows", u, result)
}

// ExperimentalProtocolConfig returns the protocol configuration.
// See https://docs.fastnear.com/rpc/protocol/experimental-protocol-config
func (c *Client) ExperimentalProtocolConfig(ctx context.Context, at *BlockRef) (*ProtocolConfig, error) {
	ref := Final()
	if at != nil {
		ref = *at
	}
	params := map[string]any{"block_reference": ref.queryParams()}
	result, u, err := c.call(ctx, "experimental_protocol_config", "EXPERIMENTAL_protocol_config", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ProtocolConfig]("experimental_protocol_config", u, result)
}

// ExperimentalCongestionLevel returns shard congestion info.
// See https://docs.fastnear.com/rpc/protocol/experimental-congestion-level
func (c *Client) ExperimentalCongestionLevel(ctx context.Context, id *BlockID, shardID uint32) (*CongestionLevel, error) {
	params := map[string]any{"shard_id": shardID}
	if id != nil {
		params["block_id"] = id.value()
	}
	result, u, err := c.call(ctx, "experimental_congestion_level", "EXPERIMENTAL_congestion_level", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[CongestionLevel]("experimental_congestion_level", u, result)
}

// ExperimentalSplitStorageInfo returns split storage info.
// See https://docs.fastnear.com/rpc/protocol/experimental-split-storage-info
func (c *Client) ExperimentalSplitStorageInfo(ctx context.Context) (*SplitStorageInfo, error) {
	result, u, err := c.call(ctx, "experimental_split_storage_info", "EXPERIMENTAL_split_storage_info", nil, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[SplitStorageInfo]("experimental_split_storage_info", u, result)
}

// --- Light client --------------------------------------------------------

// LightClientProof proves inclusion of a transaction or receipt.
// See https://docs.fastnear.com/rpc/protocol/light-client-proof
func (c *Client) LightClientProof(ctx context.Context, req LightClientProofRequest) (*LightClientProof, error) {
	params, err := lightClientProofParams(req)
	if err != nil {
		return nil, validateErr("light_client_proof", err)
	}
	result, u, err := c.call(ctx, "light_client_proof", "light_client_proof", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[LightClientProof]("light_client_proof", u, result)
}

// ExperimentalLightClientProof is the legacy light client proof.
//
// Deprecated: use LightClientProof.
// See https://docs.fastnear.com/rpc/protocol/experimental-light-client-proof
func (c *Client) ExperimentalLightClientProof(ctx context.Context, req LightClientProofRequest) (*LightClientProof, error) {
	params, err := lightClientProofParams(req)
	if err != nil {
		return nil, validateErr("experimental_light_client_proof", err)
	}
	result, u, err := c.call(ctx, "experimental_light_client_proof", "EXPERIMENTAL_light_client_proof", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[LightClientProof]("experimental_light_client_proof", u, result)
}

func lightClientProofParams(req LightClientProofRequest) (map[string]any, error) {
	if req.LightClientHead == "" {
		return nil, errEmptyArg("light_client_head")
	}
	typ := req.Type
	if typ == "" {
		typ = ProofTransaction
	}
	p := map[string]any{"type": string(typ), "light_client_head": req.LightClientHead}
	switch typ {
	case ProofTransaction:
		if req.TransactionHash == "" {
			return nil, errEmptyArg("transaction_hash")
		}
		p["transaction_hash"] = req.TransactionHash
		p["sender_id"] = req.SenderID
	case ProofReceipt:
		if req.ReceiptID == "" {
			return nil, errEmptyArg("receipt_id")
		}
		p["receipt_id"] = req.ReceiptID
		p["receiver_id"] = req.SenderID
	default:
		return nil, errorString("type must be \"transaction\" or \"receipt\"")
	}
	return p, nil
}

// NextLightClientBlock returns the light client block after the given
// head hash.
// See https://docs.fastnear.com/rpc/protocol/next-light-client-block
func (c *Client) NextLightClientBlock(ctx context.Context, lastBlockHash string) (*LightClientBlock, error) {
	if lastBlockHash == "" {
		return nil, validateErr("next_light_client_block", errEmptyArg("last_block_hash"))
	}
	result, u, err := c.call(ctx, "next_light_client_block", "next_light_client_block", map[string]any{"last_block_hash": lastBlockHash}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[LightClientBlock]("next_light_client_block", u, result)
}

// ExperimentalLightClientBlockProof proves a block for the light client.
// See https://docs.fastnear.com/rpc/protocol/experimental-light-client-block-proof
func (c *Client) ExperimentalLightClientBlockProof(ctx context.Context, blockHash, lightClientHead string) (*LightClientBlockProof, error) {
	if blockHash == "" || lightClientHead == "" {
		return nil, validateErr("experimental_light_client_block_proof", errEmptyArg("block_hash/light_client_head"))
	}
	params := map[string]any{"block_hash": blockHash, "light_client_head": lightClientHead}
	result, u, err := c.call(ctx, "experimental_light_client_block_proof", "EXPERIMENTAL_light_client_block_proof", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[LightClientBlockProof]("experimental_light_client_block_proof", u, result)
}

// --- Transactions --------------------------------------------------------

// SendTx broadcasts a signed transaction and waits per waitUntil.
// It is NOT retried automatically (see WithNonIdempotentRetry).
// See https://docs.fastnear.com/rpc/transaction/send-tx
func (c *Client) SendTx(ctx context.Context, signedTxBase64 string, waitUntil WaitUntil) (*FinalExecutionOutcome, error) {
	if signedTxBase64 == "" {
		return nil, validateErr("send_tx", errEmptyArg("signed_tx_base64"))
	}
	if waitUntil == "" {
		waitUntil = WaitExecutedOptimistic
	}
	params := map[string]any{"signed_tx_base64": signedTxBase64, "wait_until": string(waitUntil)}
	result, u, err := c.call(ctx, "send_tx", "send_tx", params, true)
	if err != nil {
		return nil, err
	}
	return decodeResult[FinalExecutionOutcome]("send_tx", u, result)
}

// BroadcastTxAsync submits a signed transaction without waiting;
// it returns the transaction hash.
// See https://docs.fastnear.com/rpc/transaction/broadcast-tx-async
func (c *Client) BroadcastTxAsync(ctx context.Context, signedTxBase64 string) (string, error) {
	if signedTxBase64 == "" {
		return "", validateErr("broadcast_tx_async", errEmptyArg("signed_tx_base64"))
	}
	params := map[string]any{"signed_tx_base64": signedTxBase64}
	result, _, err := c.call(ctx, "broadcast_tx_async", "broadcast_tx_async", params, true)
	if err != nil {
		return "", err
	}
	var hash string
	if err := json.Unmarshal(result, &hash); err != nil {
		return "", transport.SerializationError(family+".broadcast_tx_async", family, "", result, err)
	}
	return hash, nil
}

// BroadcastTxCommit submits a signed transaction and waits for final
// execution.
//
// Deprecated: use SendTx with WaitFinal.
// See https://docs.fastnear.com/rpc/transaction/broadcast-tx-commit
func (c *Client) BroadcastTxCommit(ctx context.Context, signedTxBase64 string) (*FinalExecutionOutcome, error) {
	if signedTxBase64 == "" {
		return nil, validateErr("broadcast_tx_commit", errEmptyArg("signed_tx_base64"))
	}
	params := map[string]any{"signed_tx_base64": signedTxBase64}
	result, u, err := c.call(ctx, "broadcast_tx_commit", "broadcast_tx_commit", params, true)
	if err != nil {
		return nil, err
	}
	return decodeResult[FinalExecutionOutcome]("broadcast_tx_commit", u, result)
}

// TxStatus polls the status of a transaction by hash and sender.
// See https://docs.fastnear.com/rpc/transaction/tx-status
func (c *Client) TxStatus(ctx context.Context, txHash, senderAccountID string, waitUntil WaitUntil) (*FinalExecutionOutcome, error) {
	return c.txLookup(ctx, "tx_status", "tx_status", txHash, senderAccountID, waitUntil)
}

// Tx fetches a transaction outcome by hash and sender.
// See https://docs.fastnear.com/rpc/transaction/tx
func (c *Client) Tx(ctx context.Context, txHash, senderAccountID string, waitUntil WaitUntil) (*FinalExecutionOutcome, error) {
	return c.txLookup(ctx, "tx", "tx", txHash, senderAccountID, waitUntil)
}

// ExperimentalTxStatus is the legacy status poller.
//
// Deprecated: use TxStatus.
// See https://docs.fastnear.com/rpc/transaction/experimental-tx-status
func (c *Client) ExperimentalTxStatus(ctx context.Context, txHash, senderAccountID string, waitUntil WaitUntil) (*FinalExecutionOutcome, error) {
	return c.txLookup(ctx, "experimental_tx_status", "EXPERIMENTAL_tx_status", txHash, senderAccountID, waitUntil)
}

func (c *Client) txLookup(ctx context.Context, op, method, txHash, senderID string, waitUntil WaitUntil) (*FinalExecutionOutcome, error) {
	if txHash == "" {
		return nil, validateErr(op, errEmptyArg("tx_hash"))
	}
	if err := validate.AccountID(senderID); err != nil {
		return nil, validateErr(op, err)
	}
	if waitUntil == "" {
		waitUntil = WaitExecutedOptimistic
	}
	params := map[string]any{
		"tx_hash":           txHash,
		"sender_account_id": senderID,
		"wait_until":        string(waitUntil),
	}
	result, u, err := c.call(ctx, op, method, params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[FinalExecutionOutcome](op, u, result)
}

// ExperimentalReceipt returns a receipt by id.
// See https://docs.fastnear.com/rpc/transaction/experimental-receipt
func (c *Client) ExperimentalReceipt(ctx context.Context, receiptID string) (*Receipt, error) {
	if receiptID == "" {
		return nil, validateErr("experimental_receipt", errEmptyArg("receipt_id"))
	}
	result, u, err := c.call(ctx, "experimental_receipt", "EXPERIMENTAL_receipt", map[string]any{"receipt_id": receiptID}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[Receipt]("experimental_receipt", u, result)
}

// ExperimentalReceiptToTx maps a receipt to its transaction. Requires
// a node with save_receipt_to_tx; otherwise UNKNOWN_RECEIPT is
// returned.
// See https://docs.fastnear.com/rpc/transaction/experimental-receipt-to-tx
func (c *Client) ExperimentalReceiptToTx(ctx context.Context, receiptID string, blockHeight *uint64) (*ReceiptToTx, error) {
	if receiptID == "" {
		return nil, validateErr("experimental_receipt_to_tx", errEmptyArg("receipt_id"))
	}
	params := map[string]any{"receipt_id": receiptID}
	if blockHeight != nil {
		params["block_height"] = *blockHeight
	}
	result, u, err := c.call(ctx, "experimental_receipt_to_tx", "EXPERIMENTAL_receipt_to_tx", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ReceiptToTx]("experimental_receipt_to_tx", u, result)
}

// --- Validators ----------------------------------------------------------

// Validators returns the validator set of the current epoch.
// See https://docs.fastnear.com/rpc/validators/validators-current
func (c *Client) Validators(ctx context.Context) (*ValidatorSet, error) {
	result, u, err := c.call(ctx, "validators", "validators", []any{nil}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ValidatorSet]("validators", u, result)
}

// ValidatorsByEpoch returns the validator set of a past epoch.
// See https://docs.fastnear.com/rpc/validators/validators-by-epoch
func (c *Client) ValidatorsByEpoch(ctx context.Context, epochID string) (*ValidatorSet, error) {
	if epochID == "" {
		return nil, validateErr("validators", errEmptyArg("epoch_id"))
	}
	result, u, err := c.call(ctx, "validators", "validators", map[string]any{"epoch_id": epochID}, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ValidatorSet]("validators", u, result)
}

// ExperimentalValidatorsOrdered returns validators in assignment order.
// See https://docs.fastnear.com/rpc/validators/experimental-validators-ordered
func (c *Client) ExperimentalValidatorsOrdered(ctx context.Context, id *BlockID) (*ValidatorsOrdered, error) {
	params := map[string]any{}
	if id != nil {
		params["block_id"] = id.value()
	}
	result, u, err := c.call(ctx, "experimental_validators_ordered", "EXPERIMENTAL_validators_ordered", params, false)
	if err != nil {
		return nil, err
	}
	return decodeResult[ValidatorsOrdered]("experimental_validators_ordered", u, result)
}

func errEmptyArg(name string) error {
	return errorString(name + " must not be empty")
}
