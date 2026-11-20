// Package accounts is the FastNear API client: indexed account views
// (balances, FT/NFT, staking pools, public-key lookup).
// Base URL: https://api.fastnear.com (testnet: https://test.api.fastnear.com).
// See https://docs.fastnear.com/api
package accounts

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
	"github.com/tigusigalpa/fastnear-go/internal/validate"
)

const family = "accounts"

// Client is the FastNear API client. Obtain it from the root
// fastnear.Client; it is safe for concurrent use.
type Client struct {
	t   *transport.Client
	err error
}

// NewClient is used internally by fastnear.Client. The err parameter
// carries a configuration error that every method returns.
func NewClient(t *transport.Client, err error) *Client {
	return &Client{t: t, err: err}
}

// dtoPtr is a pointer to a DTO that can store its raw body.
type dtoPtr[T any] interface {
	*T
	SetRaw(json.RawMessage)
}

// get performs a GET request and decodes a successful JSON body into a
// new T. A JSON null yields (nil, nil) without an error.
func get[T any, P dtoPtr[T]](c *Client, ctx context.Context, op string, segs ...string) (*T, error) {
	op = family + "." + op
	if c == nil {
		return nil, fastnearerr.Config(op, family, "uninitialized client")
	}
	if c.err != nil {
		return nil, &fastnearerr.Error{Op: op, Family: family, Code: "configuration_error", Message: c.err.Error(), Err: c.err}
	}
	if c.t == nil {
		return nil, fastnearerr.Config(op, family, "uninitialized client")
	}
	resp, err := c.t.Do(ctx, transport.Request{Method: http.MethodGet, Segments: segs, Op: op})
	if err != nil {
		return nil, err
	}
	out := P(new(T))
	ok, err := transport.DecodeJSON(resp.Body, out)
	if err != nil {
		return nil, transport.SerializationError(op, family, resp.EffectiveURL, resp.Body, err)
	}
	if !ok {
		return nil, nil
	}
	out.SetRaw(resp.Body)
	return out, nil
}

// validate wraps a local argument error into a validation *Error.
func validateErr(op string, err error) error {
	return &fastnearerr.Error{Op: family + "." + op, Family: family, Code: "validation_error", Message: err.Error()}
}

// Health reports whether the service is up.
// See https://docs.fastnear.com/api/system/health
func (c *Client) Health(ctx context.Context) (*Health, error) {
	return get[Health](c, ctx, "health", "health")
}

// Status returns indexer sync state: heights, latency, version.
// See https://docs.fastnear.com/api/system/status
func (c *Client) Status(ctx context.Context) (*ServiceStatus, error) {
	return get[ServiceStatus](c, ctx, "status", "status")
}

// FTContracts lists FT contract ids the account interacted with (v0).
//
// Deprecated: use FTBalances — the v1 endpoint also returns balances
// and last-update block heights.
// See https://docs.fastnear.com/api/v0/account-ft
func (c *Client) FTContracts(ctx context.Context, accountID string) (*ContractIDs, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("ft_contracts", err)
	}
	return get[ContractIDs](c, ctx, "ft_contracts", "v0", "account", accountID, "ft")
}

// NFTContracts lists NFT contract ids the account interacted with (v0).
//
// Deprecated: use NFTHoldings — the v1 endpoint also returns
// last-update block heights.
// See https://docs.fastnear.com/api/v0/account-nft
func (c *Client) NFTContracts(ctx context.Context, accountID string) (*ContractIDs, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("nft_contracts", err)
	}
	return get[ContractIDs](c, ctx, "nft_contracts", "v0", "account", accountID, "nft")
}

// StakingPools lists staking pool ids of the account (v0).
//
// Deprecated: use Staking — the v1 endpoint also returns last-update
// block heights.
// See https://docs.fastnear.com/api/v0/account-staking
func (c *Client) StakingPools(ctx context.Context, accountID string) (*PoolIDs, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("staking_pools", err)
	}
	return get[PoolIDs](c, ctx, "staking_pools", "v0", "account", accountID, "staking")
}

// PublicKeyAccounts resolves a public key to the accounts that can use
// it (v1, primary key type only).
// See https://docs.fastnear.com/api/v1/public-key
func (c *Client) PublicKeyAccounts(ctx context.Context, publicKey string) (*PublicKeyLookup, error) {
	if err := validate.PublicKey(publicKey); err != nil {
		return nil, validateErr("public_key_accounts", err)
	}
	return get[PublicKeyLookup](c, ctx, "public_key_accounts", "v1", "public_key", publicKey)
}

// PublicKeyAccountsAll resolves a public key to all accounts, including
// those holding it via a different key type (v1).
// See https://docs.fastnear.com/api/v1/public-key-all
func (c *Client) PublicKeyAccountsAll(ctx context.Context, publicKey string) (*PublicKeyLookup, error) {
	if err := validate.PublicKey(publicKey); err != nil {
		return nil, validateErr("public_key_accounts_all", err)
	}
	return get[PublicKeyLookup](c, ctx, "public_key_accounts_all", "v1", "public_key", publicKey, "all")
}

// PublicKeyAccountsLegacy resolves a public key to accounts (v0).
//
// Deprecated: use PublicKeyAccounts.
// See https://docs.fastnear.com/api/v0/public-key
func (c *Client) PublicKeyAccountsLegacy(ctx context.Context, publicKey string) (*PublicKeyLookup, error) {
	if err := validate.PublicKey(publicKey); err != nil {
		return nil, validateErr("public_key_accounts_legacy", err)
	}
	return get[PublicKeyLookup](c, ctx, "public_key_accounts_legacy", "v0", "public_key", publicKey)
}

// PublicKeyAccountsAllLegacy resolves a public key to all accounts (v0).
//
// Deprecated: use PublicKeyAccountsAll.
// See https://docs.fastnear.com/api/v0/public-key-all
func (c *Client) PublicKeyAccountsAllLegacy(ctx context.Context, publicKey string) (*PublicKeyLookup, error) {
	if err := validate.PublicKey(publicKey); err != nil {
		return nil, validateErr("public_key_accounts_all_legacy", err)
	}
	return get[PublicKeyLookup](c, ctx, "public_key_accounts_all_legacy", "v0", "public_key", publicKey, "all")
}

// FTBalances returns fungible-token balances of the account (v1).
// See https://docs.fastnear.com/api/v1/account-ft
func (c *Client) FTBalances(ctx context.Context, accountID string) (*FTBalances, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("ft_balances", err)
	}
	return get[FTBalances](c, ctx, "ft_balances", "v1", "account", accountID, "ft")
}

// NFTHoldings returns NFT contract holdings of the account (v1).
// See https://docs.fastnear.com/api/v1/account-nft
func (c *Client) NFTHoldings(ctx context.Context, accountID string) (*NFTHoldings, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("nft_holdings", err)
	}
	return get[NFTHoldings](c, ctx, "nft_holdings", "v1", "account", accountID, "nft")
}

// Staking returns staking pools of the account (v1).
// See https://docs.fastnear.com/api/v1/account-staking
func (c *Client) Staking(ctx context.Context, accountID string) (*StakingPools, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("staking", err)
	}
	return get[StakingPools](c, ctx, "staking", "v1", "account", accountID, "staking")
}

// AccountFull returns the combined indexed account view: staking
// pools, fungible-token balances, NFTs and account state.
// See https://docs.fastnear.com/api/v1/account-full
func (c *Client) AccountFull(ctx context.Context, accountID string) (*AccountFull, error) {
	if err := validate.AccountID(accountID); err != nil {
		return nil, validateErr("account_full", err)
	}
	return get[AccountFull](c, ctx, "account_full", "v1", "account", accountID, "full")
}

// FTTopHolders returns the top holders of a fungible token. The list
// size is fixed by the server.
// See https://docs.fastnear.com/api/v1/ft-top
func (c *Client) FTTopHolders(ctx context.Context, tokenID string) (*FTTopHolders, error) {
	if err := validate.AccountID(tokenID); err != nil {
		return nil, validateErr("ft_top_holders", err)
	}
	return get[FTTopHolders](c, ctx, "ft_top_holders", "v1", "ft", tokenID, "top")
}
