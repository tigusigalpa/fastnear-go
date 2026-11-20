// Package recipes composes the six family clients into ready-made
// scenarios: account snapshots, token lookups, history streams and
// block access. Every recipe is a thin layer — for full control use
// the family clients directly.
package recipes

import (
	"context"
	"encoding/base64"
	"iter"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/accounts"
	"github.com/tigusigalpa/fastnear-go/data"
	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/rpc"
	"github.com/tigusigalpa/fastnear-go/transfers"
	"github.com/tigusigalpa/fastnear-go/tx"
)

// Service bundles recipe helpers around a root client.
type Service struct {
	c *fastnear.Client
}

// New creates the recipe service over an existing client.
func New(c *fastnear.Client) *Service { return &Service{c: c} }

// Snapshot is recipe 1: the complete picture of an account in one
// call pair — indexed balances (accounts API) plus the on-chain state
// (RPC view_account) at final finality.
type Snapshot struct {
	AccountID string
	Full      *accounts.AccountFull
	OnChain   *rpc.ViewAccountResult
	// Balance/StorageUsage are convenient typed projections of
	// OnChain; nil when the RPC side failed or returned null.
	Balance      *fastnear.Amount
	Locked       *fastnear.Amount
	StorageUsage uint64
}

// AccountSnapshot fetches the indexed full view and the on-chain
// account state. Either side may be nil when the account is absent
// from that surface.
func (s *Service) AccountSnapshot(ctx context.Context, accountID string) (*Snapshot, error) {
	full, err := s.c.Accounts().AccountFull(ctx, accountID)
	if err != nil {
		return nil, err
	}
	onchain, err := s.c.RPC().ViewAccount(ctx, accountID, nil)
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{AccountID: accountID, Full: full, OnChain: onchain}
	if onchain != nil {
		snap.StorageUsage = onchain.StorageUsage
		if a, aerr := fastnear.FromYocto(onchain.Amount); aerr == nil {
			snap.Balance = a
		}
		if l, lerr := fastnear.FromYocto(onchain.Locked); lerr == nil {
			snap.Locked = l
		}
	}
	return snap, nil
}

// FTBalance is recipe 2: the balance of one fungible token for an
// account. It first consults the indexed view; when the token is not
// listed there it falls back to ft_balance_of on the contract.
// Returns ("", false, nil) when the account holds no such token.
func (s *Service) FTBalance(ctx context.Context, accountID, tokenID string) (string, bool, error) {
	indexed, err := s.c.Accounts().FTBalances(ctx, accountID)
	if err != nil {
		return "", false, err
	}
	if indexed != nil {
		for _, t := range indexed.Tokens {
			if t.ContractID == tokenID && t.Balance != nil {
				return *t.Balance, true, nil
			}
		}
	}
	res, err := s.c.RPC().CallFunction(ctx, rpc.CallFunctionRequest{
		AccountID:  tokenID,
		MethodName: "ft_balance_of",
		Args:       map[string]any{"account_id": accountID},
	})
	if err != nil {
		return "", false, err
	}
	if res == nil || res.IsEmpty() {
		return "", false, nil
	}
	var raw string
	if err := res.JSON(&raw); err != nil {
		return "", false, &fastnearerr.Error{Op: "recipes.ft_balance", Family: "recipes", Code: "serialization_error", Message: "ft_balance_of returned non-string", Err: err}
	}
	return raw, true, nil
}

// NFTContracts is recipe 3: the NFT contracts an account interacts
// with (indexed view).
func (s *Service) NFTContracts(ctx context.Context, accountID string) (*accounts.NFTHoldings, error) {
	return s.c.Accounts().NFTHoldings(ctx, accountID)
}

// StakingSummary is recipe 4: the staking pools of an account with
// their last-indexed heights.
func (s *Service) StakingSummary(ctx context.Context, accountID string) (*accounts.StakingPools, error) {
	return s.c.Accounts().Staking(ctx, accountID)
}

// TopHolders is recipe 5: the largest holders of a fungible token.
func (s *Service) TopHolders(ctx context.Context, tokenID string) (*accounts.FTTopHolders, error) {
	return s.c.Accounts().FTTopHolders(ctx, tokenID)
}

// AccountsByPublicKey is recipe 6: every account controlled by a
// public key.
func (s *Service) AccountsByPublicKey(ctx context.Context, publicKey string) (*accounts.PublicKeyLookup, error) {
	return s.c.Accounts().PublicKeyAccounts(ctx, publicKey)
}

// ViewCallJSON is recipe 7: a contract view call with JSON arguments
// and a typed result. args may be nil (sent as {}) and out any JSON
// target (e.g. *string, *MyStruct). An empty result leaves out
// untouched.
func (s *Service) ViewCallJSON(ctx context.Context, contractID, method string, args any, out any) error {
	res, err := s.c.RPC().CallFunction(ctx, rpc.CallFunctionRequest{
		AccountID:  contractID,
		MethodName: method,
		Args:       args,
	})
	if err != nil {
		return err
	}
	if res == nil || res.IsEmpty() {
		return nil
	}
	if err := res.JSON(out); err != nil {
		return &fastnearerr.Error{Op: "recipes.view_call", Family: "recipes", Code: "serialization_error", Message: "cannot decode view call result", Err: err}
	}
	return nil
}

// StateEntries is recipe 8: a full contract-state scan under
// prefixBase64, walking last_key pages automatically.
func (s *Service) StateEntries(ctx context.Context, accountID, prefixBase64 string, limit *uint32) iter.Seq2[rpc.StateItem, error] {
	return func(yield func(rpc.StateItem, error) bool) {
		var after *string
		for {
			page, err := s.c.RPC().ViewState(ctx, rpc.ViewStateRequest{
				AccountID:      accountID,
				PrefixBase64:   prefixBase64,
				AfterKeyBase64: after,
				Limit:          limit,
			})
			if err != nil {
				var zero rpc.StateItem
				yield(zero, err)
				return
			}
			if page == nil {
				return
			}
			items, err := page.Entries()
			if err != nil {
				var zero rpc.StateItem
				yield(zero, err)
				return
			}
			for _, it := range items {
				if !yield(it, nil) {
					return
				}
			}
			if !page.HasMore() {
				return
			}
			after = page.LastKey
		}
	}
}

// KVValue is recipe 9: the latest value stored under an exact key,
// decoded into out. Returns found=false when the key has no value.
func (s *Service) KVValue(ctx context.Context, contractID, predecessorID, key string, out any) (bool, error) {
	list, err := s.c.KV().LatestByExactKey(ctx, contractID, predecessorID, key, false)
	if err != nil {
		return false, err
	}
	if list == nil || len(list.Entries) == 0 || list.Entries[0].ValueIsNull() {
		return false, nil
	}
	if out != nil {
		if err := list.Entries[0].ValueAs(out); err != nil {
			return false, &fastnearerr.Error{Op: "recipes.kv_value", Family: "recipes", Code: "serialization_error", Message: "cannot decode KV value", Err: err}
		}
	}
	return true, nil
}

// AccountTxHistory is recipe 10: the full indexed transaction history
// of an account, streamed. Filters live on the query — see
// tx.AccountHistoryQuery for is_signer/is_receiver/is_success and the
// block-height bounds.
func (s *Service) AccountTxHistory(ctx context.Context, q tx.AccountHistoryQuery) iter.Seq2[tx.AccountTxRow, error] {
	return s.c.Tx().AccountHistorySeq(ctx, q)
}

// TransferHistory is recipe 11: all asset transfers of an account in
// one direction, streamed.
func (s *Service) TransferHistory(ctx context.Context, accountID string, dir transfers.Direction) iter.Seq2[transfers.TransferRow, error] {
	return s.c.Transfers().All(ctx, transfers.Query{AccountID: accountID, Direction: dir})
}

// LatestFinalHeight returns the height of the last final block —
// the cheap chain-progress probe.
func (s *Service) LatestFinalHeight(ctx context.Context) (uint64, error) {
	return s.c.Data().LastFinalHeight(ctx)
}

// BlockHeaders returns the light block envelope (header + chunk
// headers) at a height.
func (s *Service) BlockHeaders(ctx context.Context, height uint64) (*data.BlockEnvelope, error) {
	return s.c.Data().BlockHeaders(ctx, height)
}

// Block returns the full block document at a height.
func (s *Service) Block(ctx context.Context, height uint64) (*data.BlockDocument, error) {
	return s.c.Data().Block(ctx, height)
}

// EncodePrefix is a helper for StateEntries: base64-encodes a raw key
// prefix, e.g. EncodePrefix("STATE") == "U1RBVEU=".
func EncodePrefix(prefix string) string {
	return base64.StdEncoding.EncodeToString([]byte(prefix))
}
