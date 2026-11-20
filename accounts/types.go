package accounts

import (
	"github.com/tigusigalpa/fastnear-go/internal/jsonx"
)

// Health is the response of GET /health.
// See https://docs.fastnear.com/api/system/health
type Health struct {
	jsonx.RawCarrier
	Status string `json:"status"`
}

// ServiceStatus is the response of GET /status.
// See https://docs.fastnear.com/api/system/status
type ServiceStatus struct {
	jsonx.RawCarrier
	Version                   string   `json:"version"`
	SyncBlockHeight           *uint64  `json:"sync_block_height"`
	SyncLatencySec            *float64 `json:"sync_latency_sec"`
	SyncBlockTimestampNanosec *string  `json:"sync_block_timestamp_nanosec"`
	SyncBalanceBlockHeight    *uint64  `json:"sync_balance_block_height"`
}

// ContractIDs is the v0 response with bare contract ids (no block
// heights; use FTBalances/NFTHoldings for the v1 shape).
type ContractIDs struct {
	jsonx.RawCarrier
	AccountID   string   `json:"account_id"`
	ContractIDs []string `json:"contract_ids"`
}

// PoolIDs is the v0 staking response with bare pool ids.
type PoolIDs struct {
	jsonx.RawCarrier
	AccountID string   `json:"account_id"`
	Pools     []string `json:"pools"`
}

// PublicKeyLookup maps a public key to account ids.
type PublicKeyLookup struct {
	jsonx.RawCarrier
	PublicKey  string   `json:"public_key"`
	AccountIDs []string `json:"account_ids"`
}

// TokenRow is one fungible-token balance entry.
type TokenRow struct {
	ContractID            string  `json:"contract_id"`
	Balance               *string `json:"balance"`
	LastUpdateBlockHeight *uint64 `json:"last_update_block_height"`
}

// FTBalances is GET /v1/account/{id}/ft.
// See https://docs.fastnear.com/api/v1/account-ft
type FTBalances struct {
	jsonx.RawCarrier
	AccountID string     `json:"account_id"`
	Tokens    []TokenRow `json:"tokens"`
}

// NFTRow is one NFT contract entry.
type NFTRow struct {
	ContractID            string  `json:"contract_id"`
	LastUpdateBlockHeight *uint64 `json:"last_update_block_height"`
}

// NFTHoldings is GET /v1/account/{id}/nft.
// See https://docs.fastnear.com/api/v1/account-nft
type NFTHoldings struct {
	jsonx.RawCarrier
	AccountID string   `json:"account_id"`
	Tokens    []NFTRow `json:"tokens"`
}

// PoolRow is one staking-pool entry.
type PoolRow struct {
	PoolID                string  `json:"pool_id"`
	LastUpdateBlockHeight *uint64 `json:"last_update_block_height"`
}

// StakingPools is GET /v1/account/{id}/staking.
// See https://docs.fastnear.com/api/v1/account-staking
type StakingPools struct {
	jsonx.RawCarrier
	AccountID string    `json:"account_id"`
	Pools     []PoolRow `json:"pools"`
}

// AccountState is the canonical on-chain state embedded in AccountFull.
type AccountState struct {
	Balance      *string `json:"balance"`
	Locked       *string `json:"locked"`
	StorageBytes *uint64 `json:"storage_bytes"`
}

// AccountFull is GET /v1/account/{id}/full: the combined indexed view.
// See https://docs.fastnear.com/api/v1/account-full
type AccountFull struct {
	jsonx.RawCarrier
	AccountID string        `json:"account_id"`
	Pools     []PoolRow     `json:"pools"`
	Tokens    []TokenRow    `json:"tokens"`
	NFTs      []NFTRow      `json:"nfts"`
	State     *AccountState `json:"state"`
}

// AccountBalanceRow is one holder entry of FTTopHolders.
type AccountBalanceRow struct {
	AccountID string  `json:"account_id"`
	Balance   *string `json:"balance"`
}

// FTTopHolders is GET /v1/ft/{token}/top.
// See https://docs.fastnear.com/api/v1/ft-top
type FTTopHolders struct {
	jsonx.RawCarrier
	TokenID  string              `json:"token_id"`
	Accounts []AccountBalanceRow `json:"accounts"`
}
