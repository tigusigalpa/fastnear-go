package transfers

import (
	"fmt"

	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// Direction filters transfers by role. Required by the API.
type Direction string

const (
	// DirectionSender lists transfers where the account is the sender.
	DirectionSender Direction = "sender"
	// DirectionReceiver lists transfers where the account is the
	// receiver.
	DirectionReceiver Direction = "receiver"
)

// Query is the /v0/transfers request body. Only non-nil optional
// fields are sent. to_timestamp_ms is an EXCLUSIVE bound.
// See https://docs.fastnear.com/transfers/query
type Query struct {
	// AccountID is required.
	AccountID string
	// Direction is required: "sender" or "receiver".
	Direction Direction
	// AssetID filters by asset: "native:near" or a contract id.
	AssetID         *string
	Desc            *bool
	FromTimestampMs *uint64
	ToTimestampMs   *uint64
	IgnoreSystem    *bool
	// MinAmount is a raw u128 string in the asset's base units.
	// Passing numbers is unsupported on purpose: u128 does not fit
	// int64.
	MinAmount      *string
	MinHumanAmount *float64
	MinUsdAmount   *float64
	// Limit is 1..100.
	Limit *uint32
	// ResumeToken is an opaque server cursor; pass it back unchanged.
	ResumeToken *string
}

func (q Query) validate() error {
	if q.AccountID == "" {
		return fmt.Errorf("account_id must not be empty")
	}
	if q.Direction != DirectionSender && q.Direction != DirectionReceiver {
		return fmt.Errorf("direction must be \"sender\" or \"receiver\", got %q", q.Direction)
	}
	if q.Limit != nil && (*q.Limit < 1 || *q.Limit > 100) {
		return fmt.Errorf("limit must be in [1, 100], got %d", *q.Limit)
	}
	return nil
}

func (q Query) marshal() map[string]any {
	m := map[string]any{
		"account_id": q.AccountID,
		"direction":  string(q.Direction),
	}
	if q.AssetID != nil {
		m["asset_id"] = *q.AssetID
	}
	if q.Desc != nil {
		m["desc"] = *q.Desc
	}
	if q.FromTimestampMs != nil {
		m["from_timestamp_ms"] = *q.FromTimestampMs
	}
	if q.ToTimestampMs != nil {
		m["to_timestamp_ms"] = *q.ToTimestampMs
	}
	if q.IgnoreSystem != nil {
		m["ignore_system"] = *q.IgnoreSystem
	}
	if q.MinAmount != nil {
		m["min_amount"] = *q.MinAmount
	}
	if q.MinHumanAmount != nil {
		m["min_human_amount"] = *q.MinHumanAmount
	}
	if q.MinUsdAmount != nil {
		m["min_usd_amount"] = *q.MinUsdAmount
	}
	if q.Limit != nil {
		m["limit"] = *q.Limit
	}
	if q.ResumeToken != nil {
		m["resume_token"] = *q.ResumeToken
	}
	return m
}

// TransferRow is one asset movement entry. block_height and
// block_timestamp are STRINGS per the upstream schema; amount is a
// u128 string in the asset's base units.
type TransferRow struct {
	AccountID           string   `json:"account_id"`
	OtherAccountID      string   `json:"other_account_id"`
	PredecessorID       string   `json:"predecessor_id"`
	ReceiptAccountID    string   `json:"receipt_account_id"`
	SignerID            string   `json:"signer_id"`
	AssetID             string   `json:"asset_id"`
	AssetType           string   `json:"asset_type"`
	TransferType        string   `json:"transfer_type"`
	Amount              string   `json:"amount"`
	HumanAmount         *float64 `json:"human_amount"`
	UsdAmount           *float64 `json:"usd_amount"`
	StartOfBlockBalance string   `json:"start_of_block_balance"`
	EndOfBlockBalance   string   `json:"end_of_block_balance"`
	BlockHeight         string   `json:"block_height"`
	BlockTimestamp      string   `json:"block_timestamp"`
	TransactionID       string   `json:"transaction_id"`
	ReceiptID           string   `json:"receipt_id"`
	ActionIndex         uint32   `json:"action_index"`
	LogIndex            uint32   `json:"log_index"`
	TransferIndex       uint32   `json:"transfer_index"`
	MethodName          *string  `json:"method_name"`
}

// Asset type hints (the API returns strings; these are the known
// values, not a closed enum).
const (
	AssetTypeNative = "native"
	AssetTypeFT     = "ft"
)

// TransfersPage is one /v0/transfers page.
type TransfersPage struct {
	jsonx.RawCarrier
	Transfers   []TransferRow `json:"transfers"`
	ResumeToken string        `json:"resume_token"`
}

// HasMore reports whether a next page exists.
func (p *TransfersPage) HasMore() bool { return p != nil && p.ResumeToken != "" }

// Items returns the page rows.
func (p *TransfersPage) Items() []TransferRow { return p.Transfers }

// NextToken returns the continuation token ("" = end).
func (p *TransfersPage) NextToken() string { return p.ResumeToken }
