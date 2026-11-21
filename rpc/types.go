package rpc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// ViewAccountResult is the on-chain account state.
// See https://docs.fastnear.com/rpc/account/view-account
type ViewAccountResult struct {
	jsonx.RawCarrier
	Amount                  string          `json:"amount"`
	Locked                  string          `json:"locked"`
	CodeHash                string          `json:"code_hash"`
	StorageUsage            uint64          `json:"storage_usage"`
	StoragePaidAt           uint64          `json:"storage_paid_at"`
	GlobalContractAccountID *string         `json:"global_contract_account_id"`
	GlobalContractHash      *string         `json:"global_contract_hash"`
	State                   json.RawMessage `json:"state"`
	BootstrapNonce          *uint64         `json:"bootstrap_nonce"`
	BlockHeight             uint64          `json:"block_height"`
	BlockHash               string          `json:"block_hash"`
}

// AccessKeyView is a single access key.
// See https://docs.fastnear.com/rpc/account/view-access-key
type AccessKeyView struct {
	jsonx.RawCarrier
	Nonce       uint64          `json:"nonce"`
	Permission  json.RawMessage `json:"permission"`
	BlockHeight uint64          `json:"block_height"`
	BlockHash   string          `json:"block_hash"`
}

// AccessKeyInfo pairs a public key with its access key data.
type AccessKeyInfo struct {
	PublicKey string          `json:"public_key"`
	AccessKey json.RawMessage `json:"access_key"`
}

// AccessKeyList is a page of access keys.
// See https://docs.fastnear.com/rpc/account/view-access-key-list
type AccessKeyList struct {
	jsonx.RawCarrier
	Keys        []AccessKeyInfo `json:"keys"`
	LastKey     *string         `json:"last_key"`
	BlockHeight uint64          `json:"block_height"`
	BlockHash   string          `json:"block_hash"`
}

// HasMore reports whether another page exists.
func (l *AccessKeyList) HasMore() bool { return l != nil && l.LastKey != nil }

// ChunkHeader is a chunk descriptor inside a block.
type ChunkHeader struct {
	ChunkHash      string          `json:"chunk_hash"`
	PrevBlockHash  string          `json:"prev_block_hash"`
	ShardID        uint32          `json:"shard_id"`
	HeightCreated  uint64          `json:"height_created"`
	HeightIncluded uint64          `json:"height_included"`
	GasUsed        uint64          `json:"gas_used"`
	GasLimit       uint64          `json:"gas_limit"`
	Outcome        json.RawMessage `json:"outcome"`
	TxRoot         string          `json:"tx_root"`
}

// BlockHeader is the block header inside BlockView.
type BlockHeader struct {
	Height           uint64          `json:"height"`
	Hash             string          `json:"hash"`
	PrevHash         string          `json:"prev_hash"`
	EpochID          string          `json:"epoch_id"`
	NextEpochID      string          `json:"next_epoch_id"`
	Timestamp        uint64          `json:"timestamp"`
	GasPrice         string          `json:"gas_price"`
	TotalSupply      string          `json:"total_supply"`
	ChunksIncluded   uint64          `json:"chunks_included"`
	NumApprovals     *uint64         `json:"num_approvals"`
	ChallengesResult json.RawMessage `json:"challenges_result"`
	LatestRestored   json.RawMessage `json:"latest_restored_protocol_state"`
}

// BlockView is a full block (header + chunk headers).
// See https://docs.fastnear.com/rpc/block/block-by-height
type BlockView struct {
	jsonx.RawCarrier
	Author string        `json:"author"`
	Header BlockHeader   `json:"header"`
	Chunks []ChunkHeader `json:"chunks"`
}

// ChunkView is a full chunk with transactions and receipts.
// See https://docs.fastnear.com/rpc/protocol/chunk-by-hash
type ChunkView struct {
	jsonx.RawCarrier
	Author       string            `json:"author"`
	Header       ChunkHeader       `json:"header"`
	Transactions []json.RawMessage `json:"transactions"`
	Receipts     []json.RawMessage `json:"receipts"`
}

// BlockEffect describes a transaction's side effects within a block.
type BlockEffect struct {
	TransactionHash string   `json:"transaction_hash"`
	TxNonce         string   `json:"tx_nonce"`
	ReceiptIDs      []string `json:"receipt_ids"`
}

// BlockEffects is the response of the block_effects method.
// See https://docs.fastnear.com/rpc/block/block-effects
type BlockEffects struct {
	jsonx.RawCarrier
	BlockID      string        `json:"block_id"`
	BlockEffects []BlockEffect `json:"block_effects"`
}

// CallResult is a contract view call outcome.
// See https://docs.fastnear.com/rpc/contract/call-function
type CallResult struct {
	jsonx.RawCarrier
	Result      []byte   `json:"-"`
	Logs        []string `json:"logs"`
	BlockHeight uint64   `json:"block_height"`
	BlockHash   string   `json:"block_hash"`
}

// UnmarshalJSON decodes the wire form where result is an array of
// byte values.
func (r *CallResult) UnmarshalJSON(data []byte) error {
	// []byte would try base64; force numeric-array decoding.
	var raw struct {
		Result      []uint32 `json:"result"`
		Logs        []string `json:"logs"`
		BlockHeight uint64   `json:"block_height"`
		BlockHash   string   `json:"block_hash"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Result = make([]byte, len(raw.Result))
	for i, v := range raw.Result {
		if v > 255 {
			return fmt.Errorf("fastnear: result byte at index %d exceeds 255", i)
		}
		r.Result[i] = byte(v)
	}
	r.Logs = raw.Logs
	r.BlockHeight = raw.BlockHeight
	r.BlockHash = raw.BlockHash
	return nil
}

// Bytes returns the raw result bytes.
func (r *CallResult) Bytes() []byte { return r.Result }

// String returns the result bytes as UTF-8 text.
func (r *CallResult) String() string { return string(r.Result) }

// JSON unmarshals the result bytes as JSON into v.
func (r *CallResult) JSON(v any) error { return json.Unmarshal(r.Result, v) }

// IsEmpty reports whether the result byte array is empty.
func (r *CallResult) IsEmpty() bool { return len(r.Result) == 0 }

// StateItem is one decoded key/value pair from view_state.
type StateItem struct {
	Key   []byte
	Value []byte
}

// StateItemWire is the base64 wire form of StateItem.
type StateItemWire struct {
	Key   string          `json:"key"`
	Value string          `json:"value"`
	Proof json.RawMessage `json:"proof"`
}

// ViewStateResult is a contract storage page.
// See https://docs.fastnear.com/rpc/contract/view-state
type ViewStateResult struct {
	jsonx.RawCarrier
	values      []StateItemWire
	Proof       []json.RawMessage `json:"proof"`
	LastKey     *string           `json:"last_key"`
	BlockHeight uint64            `json:"block_height"`
	BlockHash   string            `json:"block_hash"`
}

// UnmarshalJSON keeps the base64 values in wire form; Entries()
// decodes them lazily.
func (r *ViewStateResult) UnmarshalJSON(data []byte) error {
	var aux struct {
		Values      []StateItemWire   `json:"values"`
		Proof       []json.RawMessage `json:"proof"`
		LastKey     *string           `json:"last_key"`
		BlockHeight uint64            `json:"block_height"`
		BlockHash   string            `json:"block_hash"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.values = aux.Values
	r.Proof = aux.Proof
	r.LastKey = aux.LastKey
	r.BlockHeight = aux.BlockHeight
	r.BlockHash = aux.BlockHash
	return nil
}

// Values returns the raw (base64) wire items for advanced use.
func (r *ViewStateResult) Values() []StateItemWire { return r.values }

// Entries decodes all key/value pairs from base64.
func (r *ViewStateResult) Entries() ([]StateItem, error) {
	out := make([]StateItem, 0, len(r.values))
	for i, it := range r.values {
		k, err := base64.StdEncoding.DecodeString(it.Key)
		if err != nil {
			return nil, fmt.Errorf("fastnear: invalid base64 key at index %d", i)
		}
		v, err := base64.StdEncoding.DecodeString(it.Value)
		if err != nil {
			return nil, fmt.Errorf("fastnear: invalid base64 value at index %d", i)
		}
		out = append(out, StateItem{Key: k, Value: v})
	}
	return out, nil
}

// HasMore reports whether another page exists.
func (r *ViewStateResult) HasMore() bool { return r != nil && r.LastKey != nil }

// ContractCodeView is deployed contract code (base64) and its hash.
type ContractCodeView struct {
	jsonx.RawCarrier
	CodeBase64  string `json:"code_base64"`
	Hash        string `json:"hash"`
	BlockHeight uint64 `json:"block_height"`
	BlockHash   string `json:"block_hash"`
}

// Code returns the decoded contract bytecode.
func (v *ContractCodeView) Code() ([]byte, error) {
	return base64.StdEncoding.DecodeString(v.CodeBase64)
}

// GasPrice is the current or per-block gas price in yoctoNEAR.
// See https://docs.fastnear.com/rpc/protocol/gas-price
type GasPrice struct {
	jsonx.RawCarrier
	GasPrice string `json:"gas_price"`
}

// PeerInfo describes one connected peer.
type PeerInfo struct {
	AccountID     *string         `json:"account_id"`
	Addr          string          `json:"addr"`
	PeerID        string          `json:"peer_id"`
	IsOutbound    *bool           `json:"is_outbound"`
	LastHeight    *uint64         `json:"last_height"`
	TrackedShards json.RawMessage `json:"tracked_shards"`
}

// NetworkInfo is the node network view.
// See https://docs.fastnear.com/rpc/protocol/network-info
type NetworkInfo struct {
	jsonx.RawCarrier
	NumActivePeers  int               `json:"num_active_peers"`
	PeerMaxCount    uint32            `json:"peer_max_count"`
	PeerMinCount    uint32            `json:"peer_min_count"`
	ActivePeers     []PeerInfo        `json:"active_peers"`
	KnownProducers  []json.RawMessage `json:"known_producers"`
	MostWeightPeers []json.RawMessage `json:"most_weight_peers"`
}

// SyncInfo is the sync section of NodeStatus.
type SyncInfo struct {
	LatestBlockHash     string  `json:"latest_block_hash"`
	LatestBlockHeight   uint64  `json:"latest_block_height"`
	LatestStateRoot     string  `json:"latest_state_root"`
	Syncing             bool    `json:"syncing"`
	EarliestBlockHash   *string `json:"earliest_block_hash"`
	EarliestBlockHeight *uint64 `json:"earliest_block_height"`
	EpochID             *string `json:"epoch_id"`
}

// NodeVersion describes the node build.
type NodeVersion struct {
	Version string `json:"version"`
	Build   string `json:"build"`
}

// NodeStatus is the node status response.
// See https://docs.fastnear.com/rpc/protocol/status
type NodeStatus struct {
	jsonx.RawCarrier
	Version         NodeVersion       `json:"version"`
	ChainID         string            `json:"chain_id"`
	ProtocolVersion uint32            `json:"protocol_version"`
	SyncInfo        SyncInfo          `json:"sync_info"`
	Validators      []json.RawMessage `json:"validators"`
}

// GenesisConfig is the chain genesis configuration.
// See https://docs.fastnear.com/rpc/protocol/genesis-config
type GenesisConfig struct {
	jsonx.RawCarrier
	ChainID               string `json:"chain_id"`
	GenesisHeight         uint64 `json:"genesis_height"`
	ProtocolVersion       uint32 `json:"protocol_version"`
	TotalSupply           string `json:"total_supply"`
	MinGasPrice           string `json:"min_gas_price"`
	EpochLength           uint64 `json:"epoch_length"`
	NumBlockProducerSeats uint64 `json:"num_block_producer_seats"`
}

// ClientConfig is the node client configuration.
// See https://docs.fastnear.com/rpc/protocol/client-config
type ClientConfig struct {
	jsonx.RawCarrier
	Version         NodeVersion `json:"version"`
	ChainID         string      `json:"chain_id"`
	TrackedShards   []uint64    `json:"tracked_shards"`
	Archive         bool        `json:"archive"`
	LogSummaryStyle string      `json:"log_summary_style"`
}

// StateChange is one entry of the `changes` response.
type StateChange struct {
	Cause json.RawMessage `json:"cause"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// StateChanges is the response of the `changes` method.
// See https://docs.fastnear.com/rpc/protocol/changes
type StateChanges struct {
	jsonx.RawCarrier
	BlockHash string        `json:"block_hash"`
	Changes   []StateChange `json:"changes"`
}

// MaintenanceWindow is one maintenance slot range.
type MaintenanceWindow struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
	// Deprecated: upstream windows use Start and End.
	Height uint64 `json:"height"`
	// Deprecated: the account ID belongs to the request, not each range.
	AccountID string `json:"account_id"`
}

// MaintenanceWindows is the validator maintenance schedule.
// See https://docs.fastnear.com/rpc/protocol/maintenance-windows
type MaintenanceWindows struct {
	jsonx.RawCarrier
	AccountID string              `json:"account_id"`
	Ranges    []MaintenanceWindow `json:"ranges"`
}

// UnmarshalJSON decodes the upstream array into the Ranges collection.
func (m *MaintenanceWindows) UnmarshalJSON(data []byte) error {
	if b := bytes.TrimSpace(data); len(b) > 0 && b[0] == '[' {
		var ranges []MaintenanceWindow
		if err := json.Unmarshal(data, &ranges); err != nil {
			return err
		}
		*m = MaintenanceWindows{Ranges: ranges}
		return nil
	}
	type plain MaintenanceWindows
	return json.Unmarshal(data, (*plain)(m))
}

// LightClientProof is a light client inclusion proof.
// See https://docs.fastnear.com/rpc/protocol/light-client-proof
type LightClientProof struct {
	jsonx.RawCarrier
	BlockHeaderLite  json.RawMessage `json:"block_header_lite"`
	BlockProof       json.RawMessage `json:"block_proof"`
	OutcomeProof     json.RawMessage `json:"outcome_proof"`
	OutcomeRootProof json.RawMessage `json:"outcome_root_proof"`
	PrevBlockHash    *string         `json:"prev_block_hash"`
}

// LightClientBlock is the response of next_light_client_block.
// See https://docs.fastnear.com/rpc/protocol/next-light-client-block
type LightClientBlock struct {
	jsonx.RawCarrier
	BlockHeaderLite    json.RawMessage `json:"block_header_lite"`
	CurrBlockHash      string          `json:"curr_block_hash"`
	NextBlockInnerHash string          `json:"next_block_inner_hash"`
	ApprovalsAfterNext json.RawMessage `json:"approvals_after_next"`
}

// LightClientBlockProof proves a block in the light client protocol.
type LightClientBlockProof struct {
	jsonx.RawCarrier
	BlockHeaderLite json.RawMessage `json:"block_header_lite"`
	BlockProof      json.RawMessage `json:"block_proof"`
}

// ProtocolConfig is the protocol configuration at a block.
type ProtocolConfig struct {
	jsonx.RawCarrier
	ProtocolVersion uint32          `json:"protocol_version"`
	ChainID         string          `json:"chain_id"`
	GenesisHeight   uint64          `json:"genesis_height"`
	EpochLength     uint64          `json:"epoch_length"`
	RuntimeConfig   json.RawMessage `json:"runtime_config"`
}

// CongestionLevel is a shard congestion report (all-fields-shards).
type CongestionLevel struct {
	jsonx.RawCarrier
	CongestionInfo json.RawMessage `json:"congestion_info"`
}

// SplitStorageInfo reports the node's split storage state.
type SplitStorageInfo struct {
	jsonx.RawCarrier
	HeadHeight     *uint64  `json:"head_height"`
	HotHeadHeight  *uint64  `json:"hot_head_height"`
	ColdHeadHeight *uint64  `json:"cold_head_height"`
	ShardUids      []uint64 `json:"shard_uids"`
}

// ExecutionStatus is the outcome status union of an execution.
type ExecutionStatus struct {
	SuccessValue     *string         `json:"SuccessValue"`
	SuccessReceiptID *string         `json:"SuccessReceiptId"`
	Failure          json.RawMessage `json:"Failure"`
	Unknown          *bool           `json:"Unknown"`
	// Pending is a unit status such as "NotStarted" or "Started".
	Pending string `json:"-"`
}

// UnmarshalJSON accepts both the object and unit-string forms of NEAR status.
func (s *ExecutionStatus) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var state string
		if err := json.Unmarshal(data, &state); err != nil {
			return err
		}
		*s = ExecutionStatus{Pending: state}
		if state == "Unknown" {
			b := true
			s.Unknown = &b
		}
		return nil
	}
	type plain ExecutionStatus
	var parsed plain
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*s = ExecutionStatus(parsed)
	return nil
}

// IsSuccess reports whether the execution succeeded.
func (s ExecutionStatus) IsSuccess() bool {
	return s.SuccessValue != nil || s.SuccessReceiptID != nil
}

// ExecutionOutcomeView is one execution outcome entry.
type ExecutionOutcomeView struct {
	BlockHash   string          `json:"block_hash"`
	ID          string          `json:"id"`
	Logs        []string        `json:"logs"`
	Proof       json.RawMessage `json:"proof"`
	ReceiptIDs  []string        `json:"receipt_ids"`
	Status      ExecutionStatus `json:"status"`
	TokensBurnt string          `json:"tokens_burnt"`
	GasBurnt    uint64          `json:"gas_burnt"`
	ExecutorID  string          `json:"executor_id"`
	Metadata    json.RawMessage `json:"metadata"`
	// Outcome contains the node's nested outcome object. The fields above
	// also expose its values for compatibility with existing callers.
	Outcome *ExecutionOutcome `json:"outcome"`
}

// ExecutionOutcome contains execution details inside a proof-bearing outcome.
type ExecutionOutcome struct {
	Logs        []string        `json:"logs"`
	ReceiptIDs  []string        `json:"receipt_ids"`
	Status      ExecutionStatus `json:"status"`
	TokensBurnt string          `json:"tokens_burnt"`
	GasBurnt    uint64          `json:"gas_burnt"`
	ExecutorID  string          `json:"executor_id"`
	Metadata    json.RawMessage `json:"metadata"`
}

// UnmarshalJSON decodes the nested outcome and populates its convenience fields.
func (o *ExecutionOutcomeView) UnmarshalJSON(data []byte) error {
	type plain ExecutionOutcomeView
	var parsed plain
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*o = ExecutionOutcomeView(parsed)
	if o.Outcome != nil {
		o.Logs, o.ReceiptIDs, o.Status = o.Outcome.Logs, o.Outcome.ReceiptIDs, o.Outcome.Status
		o.TokensBurnt, o.GasBurnt, o.ExecutorID, o.Metadata = o.Outcome.TokensBurnt, o.Outcome.GasBurnt, o.Outcome.ExecutorID, o.Outcome.Metadata
	}
	return nil
}

// FinalExecutionOutcome is the outcome of send_tx/tx/tx_status.
// See https://docs.fastnear.com/rpc/transaction/send-tx
type FinalExecutionOutcome struct {
	jsonx.RawCarrier
	FinalExecutionStatus string                 `json:"final_execution_status"`
	FinalStatus          *ExecutionStatus       `json:"status"`
	ExecutionOutcome     *ExecutionOutcomeView  `json:"execution_outcome"`
	ReceiptsOutcome      []ExecutionOutcomeView `json:"receipts_outcome"`
	Transaction          json.RawMessage        `json:"transaction"`
	TransactionOutcome   *ExecutionOutcomeView  `json:"transaction_outcome"`
}

// Status returns the aggregate status, including receipt failures. It is nil
// when the node has not returned an aggregate execution status yet.
func (o *FinalExecutionOutcome) Status() *ExecutionStatus {
	if o == nil {
		return nil
	}
	return o.FinalStatus
}

// Receipt is the EXPERIMENTAL_receipt result.
type Receipt struct {
	jsonx.RawCarrier
	PredecessorID string          `json:"predecessor_id"`
	ReceiptID     string          `json:"receipt_id"`
	ReceiverID    string          `json:"receiver_id"`
	Body          json.RawMessage `json:"receipt"`
}

// ReceiptToTx maps a receipt to its originating transaction.
type ReceiptToTx struct {
	jsonx.RawCarrier
	ReceiptID       string `json:"receipt_id"`
	TransactionHash string `json:"transaction_hash"`
}

// ValidatorInfo is one validator entry.
type ValidatorInfo struct {
	AccountID         string          `json:"account_id"`
	PublicKey         string          `json:"public_key"`
	Stake             string          `json:"stake"`
	Shards            json.RawMessage `json:"shards"`
	NumProducedBlocks *uint64         `json:"num_produced_blocks"`
	NumExpectedBlocks *uint64         `json:"num_expected_blocks"`
	IsChunkOnly       *bool           `json:"is_chunk_only"`
}

// ValidatorSet is the validators response.
// See https://docs.fastnear.com/rpc/validators/validators-current
type ValidatorSet struct {
	jsonx.RawCarrier
	CurrentValidators []ValidatorInfo   `json:"current_validators"`
	NextValidators    []ValidatorInfo   `json:"next_validators"`
	EpochStartHeight  uint64            `json:"epoch_start_height"`
	EpochHeight       uint64            `json:"epoch_height"`
	Proposals         []json.RawMessage `json:"current_proposals"`
}

// ValidatorsOrdered is the EXPERIMENTAL_validators_ordered result.
type ValidatorsOrdered struct {
	jsonx.RawCarrier
	Validators []ValidatorInfo `json:"validators"`
}

// UnmarshalJSON decodes the upstream array into the Validators collection.
func (v *ValidatorsOrdered) UnmarshalJSON(data []byte) error {
	if b := bytes.TrimSpace(data); len(b) > 0 && b[0] == '[' {
		var validators []ValidatorInfo
		if err := json.Unmarshal(data, &validators); err != nil {
			return err
		}
		*v = ValidatorsOrdered{Validators: validators}
		return nil
	}
	type plain ValidatorsOrdered
	return json.Unmarshal(data, (*plain)(v))
}
