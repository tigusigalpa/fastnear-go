package data

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// Health is the response of GET /health.
// See https://docs.fastnear.com/neardata/system/health
type Health struct {
	jsonx.RawCarrier
	Status string `json:"status"`
}

// BlockHeader is the canonical block header. The upstream document
// carries many more fields (approvals, merkle roots, signatures);
// they stay available via the parent's Raw. prev_height is null at
// the first indexed block.
type BlockHeader struct {
	Height           uint64  `json:"height"`
	Hash             string  `json:"hash"`
	PrevHash         string  `json:"prev_hash"`
	PrevHeight       *uint64 `json:"prev_height"`
	Timestamp        uint64  `json:"timestamp"`
	TimestampNanosec string  `json:"timestamp_nanosec"`
	EpochID          string  `json:"epoch_id"`
	NextEpochID      string  `json:"next_epoch_id"`
	GasPrice         string  `json:"gas_price"`
	TotalSupply      string  `json:"total_supply"`
	ChunksIncluded   uint64  `json:"chunks_included"`
}

// Time parses the nanosecond timestamp into time.Time, falling back
// to the nanosecond `timestamp` field when the nanosecond string is
// absent or malformed.
func (h *BlockHeader) Time() (time.Time, error) {
	ns, err := strconv.ParseUint(h.TimestampNanosec, 10, 64)
	if err != nil {
		ns = h.Timestamp
	}
	return time.Unix(int64(ns/1e9), int64(ns%1e9)), nil
}

// ChunkHeader is the chunk descriptor inside a block envelope.
type ChunkHeader struct {
	ChunkHash            string `json:"chunk_hash"`
	PrevBlockHash        string `json:"prev_block_hash"`
	ShardID              uint32 `json:"shard_id"`
	HeightCreated        uint64 `json:"height_created"`
	HeightIncluded       uint64 `json:"height_included"`
	GasUsed              uint64 `json:"gas_used"`
	GasLimit             uint64 `json:"gas_limit"`
	OutcomeRoot          string `json:"outcome_root"`
	OutgoingReceiptsRoot string `json:"outgoing_receipts_root"`
	TxRoot               string `json:"tx_root"`
}

// BlockEnvelope is a block header plus its chunk headers — the light
// weight way to watch the chain.
type BlockEnvelope struct {
	jsonx.RawCarrier
	Author string        `json:"author"`
	Header BlockHeader   `json:"header"`
	Chunks []ChunkHeader `json:"chunks"`
}

// SignedTransactionDocument is a transaction inside a chunk.
type SignedTransactionDocument struct {
	SignerID    string            `json:"signer_id"`
	PublicKey   string            `json:"public_key"`
	Nonce       uint64            `json:"nonce"`
	ReceiverID  string            `json:"receiver_id"`
	Actions     []json.RawMessage `json:"actions"`
	Signature   string            `json:"signature"`
	Hash        string            `json:"hash"`
	PriorityFee uint64            `json:"priority_fee"`
}

// ReceiptDocument is a receipt inside a chunk.
type ReceiptDocument struct {
	PredecessorID string          `json:"predecessor_id"`
	ReceiverID    string          `json:"receiver_id"`
	ReceiptID     string          `json:"receipt_id"`
	Priority      uint64          `json:"priority"`
	Receipt       json.RawMessage `json:"receipt"`
}

// ExecutionOutcomeDocument is the outcome of a transaction or receipt.
type ExecutionOutcomeDocument struct {
	BlockHash string            `json:"block_hash"`
	ID        string            `json:"id"`
	Outcome   json.RawMessage   `json:"outcome"`
	Proof     []json.RawMessage `json:"proof"`
}

// ExecutionWithReceipt pairs an execution outcome with its receipt.
type ExecutionWithReceipt struct {
	ExecutionOutcome ExecutionOutcomeDocument `json:"execution_outcome"`
	Receipt          json.RawMessage          `json:"receipt"`
	TxHash           string                   `json:"tx_hash"`
}

// ChunkTransactionWrapper pairs a transaction with its outcome.
type ChunkTransactionWrapper struct {
	Transaction SignedTransactionDocument `json:"transaction"`
	Outcome     ExecutionWithReceipt      `json:"outcome"`
}

// ChunkDocument is a full chunk payload.
type ChunkDocument struct {
	jsonx.RawCarrier
	Author       string                    `json:"author"`
	Header       ChunkHeader               `json:"header"`
	Transactions []ChunkTransactionWrapper `json:"transactions"`
	Receipts     []ReceiptDocument         `json:"receipts"`
}

// StateChangeItem is one state-change entry of a shard.
type StateChangeItem struct {
	Cause  json.RawMessage `json:"cause"`
	Change json.RawMessage `json:"change"`
	Type   string          `json:"type"`
}

// ShardDocument is a shard's payload inside a block document.
type ShardDocument struct {
	jsonx.RawCarrier
	ShardID                  uint64                 `json:"shard_id"`
	Chunk                    *ChunkDocument         `json:"chunk"`
	ReceiptExecutionOutcomes []ExecutionWithReceipt `json:"receipt_execution_outcomes"`
	StateChanges             []StateChangeItem      `json:"state_changes"`
}

// BlockDocument is the heaviest NEAR Data response: header, chunk
// headers and every shard payload. Use BlockHeaders when you only
// need to follow the chain.
type BlockDocument struct {
	jsonx.RawCarrier
	Block  BlockEnvelope   `json:"block"`
	Shards []ShardDocument `json:"shards"`
}
