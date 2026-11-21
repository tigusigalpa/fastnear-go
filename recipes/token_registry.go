package recipes

import (
	"context"
	"sync"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/rpc"
)

// TokenMeta is the NEP-141 ft_metadata document.
type TokenMeta struct {
	Spec     string  `json:"spec"`
	Name     string  `json:"name"`
	Symbol   string  `json:"symbol"`
	Icon     *string `json:"icon"`
	Decimals uint8   `json:"decimals"`
}

// TokenBalance is a raw token balance paired with the token's
// metadata; Format renders the human amount.
type TokenBalance struct {
	fastnear.TokenAmount
	Meta TokenMeta
}

// TokenRegistry resolves ft_metadata and caches successful responses for the
// lifetime of the registry. It also fetches ft_balance_of.
// Safe for concurrent use.
type TokenRegistry struct {
	rpc *rpc.Client
	svc *Service

	mu    sync.Mutex
	cache map[string]*TokenMeta
}

// NewTokenRegistry builds a registry over the root client.
func NewTokenRegistry(c *fastnear.Client) *TokenRegistry {
	return &TokenRegistry{rpc: c.RPC(), svc: New(c), cache: make(map[string]*TokenMeta)}
}

// Metadata returns the ft_metadata of a contract, cached per
// contract id.
func (r *TokenRegistry) Metadata(ctx context.Context, tokenID string) (*TokenMeta, error) {
	r.mu.Lock()
	if m, ok := r.cache[tokenID]; ok {
		copyMeta := cloneMeta(m)
		r.mu.Unlock()
		return copyMeta, nil
	}
	r.mu.Unlock()

	res, err := r.rpc.CallFunction(ctx, rpc.CallFunctionRequest{
		AccountID:  tokenID,
		MethodName: "ft_metadata",
	})
	if err != nil {
		return nil, err
	}
	if res == nil || res.IsEmpty() {
		return nil, &fastnearerr.Error{Op: "recipes.ft_metadata", Family: "recipes", Code: "not_found", Message: "empty ft_metadata for " + tokenID}
	}
	m := new(TokenMeta)
	if err := res.JSON(m); err != nil {
		return nil, &fastnearerr.Error{Op: "recipes.ft_metadata", Family: "recipes", Code: "serialization_error", Message: "cannot decode ft_metadata", Err: err}
	}
	r.mu.Lock()
	r.cache[tokenID] = m
	r.mu.Unlock()
	return cloneMeta(m), nil
}

func cloneMeta(m *TokenMeta) *TokenMeta {
	copyMeta := *m
	if m.Icon != nil {
		icon := *m.Icon
		copyMeta.Icon = &icon
	}
	return &copyMeta
}

// Forget evicts a cached metadata entry (e.g. after a token upgrade).
func (r *TokenRegistry) Forget(tokenID string) {
	r.mu.Lock()
	delete(r.cache, tokenID)
	r.mu.Unlock()
}

// BalanceOf returns the account's balance of a token together with
// the token metadata needed to format it. found=false means no
// balance (account never touched the token).
func (r *TokenRegistry) BalanceOf(ctx context.Context, accountID, tokenID string) (*TokenBalance, bool, error) {
	meta, err := r.Metadata(ctx, tokenID)
	if err != nil {
		return nil, false, err
	}
	raw, found, err := r.svc.FTBalance(ctx, accountID, tokenID)
	if err != nil || !found {
		return nil, found, err
	}
	return &TokenBalance{
		TokenAmount: fastnear.TokenAmount{Raw: raw, Decimals: meta.Decimals},
		Meta:        *meta,
	}, true, nil
}

// FormatAmount renders a raw base-unit balance using the token's
// decimals (metadata is fetched and cached).
func (r *TokenRegistry) FormatAmount(ctx context.Context, tokenID, raw string, precision int) (string, error) {
	meta, err := r.Metadata(ctx, tokenID)
	if err != nil {
		return "", err
	}
	return fastnear.TokenAmount{Raw: raw, Decimals: meta.Decimals}.Format(precision), nil
}
