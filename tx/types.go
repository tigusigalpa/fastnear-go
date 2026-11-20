package tx

import (
	"encoding/json"
	"fmt"

	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// AccountHistoryQuery is the /v0/account request body. Only non-nil
// fields are sent. to_tx_block_height is an EXCLUSIVE upper bound;
// from_tx_block_height is inclusive.
// See https://docs.fastnear.com/tx/account
type AccountHistoryQuery struct {
	// AccountID is required.
	AccountID string
	// Desc sorts descending by block height when true.
	Desc               *bool
	FromTxBlockHeight  *uint64
	ToTxBlockHeight    *uint64
	IsSigner           *bool
	IsAnySigner        *bool
	IsRealSigner       *bool
	IsDelegatedSigner  *bool
	IsPredecessor      *bool
	IsReceiver         *bool
	IsRealReceiver     *bool
	IsFunctionCall     *bool
	IsActionArg        *bool
	IsEventLog         *bool
	IsExplicitRefundTo *bool
	IsSuccess          *bool
	// Limit is 1..200.
	Limit *uint32
	// ResumeToken is an opaque server cursor; pass it back unchanged.
	ResumeToken *string
}

// NewAccountHistoryQuery builds a query for the given account.
func NewAccountHistoryQuery(accountID string) AccountHistoryQuery {
	return AccountHistoryQuery{AccountID: accountID}
}

// WithResumeToken returns a copy of q pinned to a resume token.
func (q AccountHistoryQuery) WithResumeToken(token string) AccountHistoryQuery {
	q.ResumeToken = &token
	return q
}

// marshal renders only the set fields.
func (q AccountHistoryQuery) marshal() map[string]any {
	m := map[string]any{"account_id": q.AccountID}
	if q.Desc != nil {
		m["desc"] = *q.Desc
	}
	if q.FromTxBlockHeight != nil {
		m["from_tx_block_height"] = *q.FromTxBlockHeight
	}
	if q.ToTxBlockHeight != nil {
		m["to_tx_block_height"] = *q.ToTxBlockHeight
	}
	if q.IsSigner != nil {
		m["is_signer"] = *q.IsSigner
	}
	if q.IsAnySigner != nil {
		m["is_any_signer"] = *q.IsAnySigner
	}
	if q.IsRealSigner != nil {
		m["is_real_signer"] = *q.IsRealSigner
	}
	if q.IsDelegatedSigner != nil {
		m["is_delegated_signer"] = *q.IsDelegatedSigner
	}
	if q.IsPredecessor != nil {
		m["is_predecessor"] = *q.IsPredecessor
	}
	if q.IsReceiver != nil {
		m["is_receiver"] = *q.IsReceiver
	}
	if q.IsRealReceiver != nil {
		m["is_real_receiver"] = *q.IsRealReceiver
	}
	if q.IsFunctionCall != nil {
		m["is_function_call"] = *q.IsFunctionCall
	}
	if q.IsActionArg != nil {
		m["is_action_arg"] = *q.IsActionArg
	}
	if q.IsEventLog != nil {
		m["is_event_log"] = *q.IsEventLog
	}
	if q.IsExplicitRefundTo != nil {
		m["is_explicit_refund_to"] = *q.IsExplicitRefundTo
	}
	if q.IsSuccess != nil {
		m["is_success"] = *q.IsSuccess
	}
	if q.Limit != nil {
		m["limit"] = *q.Limit
	}
	if q.ResumeToken != nil {
		m["resume_token"] = *q.ResumeToken
	}
	return m
}

// AccountTxRow is one account-history row (roles + tx coordinates).
type AccountTxRow struct {
	AccountID          string `json:"account_id"`
	TransactionHash    string `json:"transaction_hash"`
	TxBlockHeight      uint64 `json:"tx_block_height"`
	TxBlockTimestamp   string `json:"tx_block_timestamp"`
	TxIndex            uint32 `json:"tx_index"`
	IsSigner           bool   `json:"is_signer"`
	IsAnySigner        bool   `json:"is_any_signer"`
	IsRealSigner       bool   `json:"is_real_signer"`
	IsDelegatedSigner  bool   `json:"is_delegated_signer"`
	IsPredecessor      bool   `json:"is_predecessor"`
	IsReceiver         bool   `json:"is_receiver"`
	IsRealReceiver     bool   `json:"is_real_receiver"`
	IsFunctionCall     bool   `json:"is_function_call"`
	IsActionArg        bool   `json:"is_action_arg"`
	IsEventLog         bool   `json:"is_event_log"`
	IsExplicitRefundTo bool   `json:"is_explicit_refund_to"`
	IsSuccess          bool   `json:"is_success"`
}

// AccountHistoryPage is one /v0/account page. ResumeToken is always
// present in the response; an empty string means the end.
type AccountHistoryPage struct {
	jsonx.RawCarrier
	AccountTxs  []AccountTxRow `json:"account_txs"`
	ResumeToken string         `json:"resume_token"`
	TxsCount    uint64         `json:"txs_count"`
}

// HasMore reports whether a next page exists.
func (p *AccountHistoryPage) HasMore() bool { return p != nil && p.ResumeToken != "" }

// Items returns the page rows.
func (p *AccountHistoryPage) Items() []AccountTxRow { return p.AccountTxs }

// NextToken returns the opaque continuation token ("" = end).
func (p *AccountHistoryPage) NextToken() string { return p.ResumeToken }

// BlockRow is an indexed block header row.
type BlockRow struct {
	AuthorID        string  `json:"author_id"`
	BlockHash       string  `json:"block_hash"`
	BlockHeight     uint64  `json:"block_height"`
	BlockOrdinal    *uint64 `json:"block_ordinal"`
	BlockTimestamp  string  `json:"block_timestamp"`
	ChunksIncluded  uint64  `json:"chunks_included"`
	EpochID         string  `json:"epoch_id"`
	GasBurnt        string  `json:"gas_burnt"`
	GasPrice        string  `json:"gas_price"`
	NextEpochID     string  `json:"next_epoch_id"`
	NumReceipts     uint32  `json:"num_receipts"`
	NumTransactions uint32  `json:"num_transactions"`
	PrevBlockHash   string  `json:"prev_block_hash"`
	PrevBlockHeight *uint64 `json:"prev_block_height"`
	ProtocolVersion uint32  `json:"protocol_version"`
	TokensBurnt     string  `json:"tokens_burnt"`
	TotalSupply     string  `json:"total_supply"`
}

// BlockTxRow is a transaction inside a block page.
type BlockTxRow struct {
	GasBurnt         uint64 `json:"gas_burnt"`
	IsCompleted      bool   `json:"is_completed"`
	IsRelayed        bool   `json:"is_relayed"`
	IsSuccess        bool   `json:"is_success"`
	LastBlockHeight  uint64 `json:"last_block_height"`
	Nonce            uint64 `json:"nonce"`
	PriorityFee      uint64 `json:"priority_fee"`
	RealReceiverID   string `json:"real_receiver_id"`
	RealSignerID     string `json:"real_signer_id"`
	ReceiverID       string `json:"receiver_id"`
	ShardID          uint64 `json:"shard_id"`
	SignerID         string `json:"signer_id"`
	SignerPublicKey  string `json:"signer_public_key"`
	TokensBurnt      string `json:"tokens_burnt"`
	TransactionHash  string `json:"transaction_hash"`
	TxBlockHash      string `json:"tx_block_hash"`
	TxBlockHeight    uint64 `json:"tx_block_height"`
	TxBlockTimestamp string `json:"tx_block_timestamp"`
	TxIndex          uint32 `json:"tx_index"`
}

// ReceiptRow is a receipt row inside block and receipt pages.
type ReceiptRow struct {
	ReceiptID          string `json:"receipt_id"`
	BlockHeight        uint64 `json:"block_height"`
	BlockTimestamp     string `json:"block_timestamp"`
	ReceiptIndex       uint32 `json:"receipt_index"`
	AppearBlockHeight  uint64 `json:"appear_block_height"`
	AppearReceiptIndex uint32 `json:"appear_receipt_index"`
	TransactionHash    string `json:"transaction_hash"`
	PredecessorID      string `json:"predecessor_id"`
	ReceiverID         string `json:"receiver_id"`
	ReceiptType        string `json:"receipt_type"`
	IsSuccess          bool   `json:"is_success"`
	ShardID            uint64 `json:"shard_id"`
	Priority           uint64 `json:"priority"`
	TxBlockHeight      uint64 `json:"tx_block_height"`
	TxBlockTimestamp   string `json:"tx_block_timestamp"`
	TxIndex            uint32 `json:"tx_index"`
}

// BlockPage is the /v0/block response: block (null = not found) with
// optional transactions and receipts.
type BlockPage struct {
	jsonx.RawCarrier
	Block    *BlockRow    `json:"block"`
	Txs      []BlockTxRow `json:"block_txs"`
	Receipts []ReceiptRow `json:"block_receipts"`
}

// BlocksPage is the /v0/blocks response.
type BlocksPage struct {
	jsonx.RawCarrier
	Blocks []BlockRow `json:"blocks"`
}

// ReceiptPage is the /v0/receipt response; both fields are null when
// nothing is found.
type ReceiptPage struct {
	jsonx.RawCarrier
	Receipt     *ReceiptRow     `json:"receipt"`
	Transaction json.RawMessage `json:"transaction"`
}

// TransactionsPage is the /v0/transactions response.
type TransactionsPage struct {
	jsonx.RawCarrier
	Transactions []json.RawMessage `json:"transactions"`
}

// validateQuery checks AccountHistoryQuery constraints.
func validateQuery(q AccountHistoryQuery) error {
	if q.AccountID == "" {
		return fmt.Errorf("account_id must not be empty")
	}
	if q.Limit != nil && (*q.Limit < 1 || *q.Limit > 200) {
		return fmt.Errorf("limit must be in [1, 200], got %d", *q.Limit)
	}
	return nil
}
