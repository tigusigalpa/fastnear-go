package accounts

import (
	"context"
	"errors"
	httptest "github.com/tigusigalpa/fastnear-go/internal/testutil"
	"net/http"
	"testing"
	"time"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
	"github.com/tigusigalpa/fastnear-go/internal/transport"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	tr := transport.New(transport.Params{
		HTTPClient: srv.Client(), BaseURL: srv.URL, APIKey: "k", Family: family,
		Timeout:  5 * time.Second,
		Retry:    transport.RetryPolicy{MaxRetries: 0},
		MaxBytes: 1 << 20,
	})
	return NewClient(tr, nil)
}

// TestPaths verifies the request path and method of all 14 operations.
func TestPaths(t *testing.T) {
	cases := []struct {
		name string
		path string
		call func(c *Client) error
	}{
		{"health", "/health", func(c *Client) error { _, e := c.Health(context.Background()); return e }},
		{"status", "/status", func(c *Client) error { _, e := c.Status(context.Background()); return e }},
		{"ft_contracts", "/v0/account/root.near/ft", func(c *Client) error { _, e := c.FTContracts(context.Background(), "root.near"); return e }},
		{"nft_contracts", "/v0/account/root.near/nft", func(c *Client) error { _, e := c.NFTContracts(context.Background(), "root.near"); return e }},
		{"staking_pools", "/v0/account/root.near/staking", func(c *Client) error { _, e := c.StakingPools(context.Background(), "root.near"); return e }},
		{"pk_legacy", "/v0/public_key/ed25519:AbC", func(c *Client) error {
			_, e := c.PublicKeyAccountsLegacy(context.Background(), "ed25519:AbC")
			return e
		}},
		{"pk_all_legacy", "/v0/public_key/ed25519:AbC/all", func(c *Client) error {
			_, e := c.PublicKeyAccountsAllLegacy(context.Background(), "ed25519:AbC")
			return e
		}},
		{"ft_balances", "/v1/account/root.near/ft", func(c *Client) error { _, e := c.FTBalances(context.Background(), "root.near"); return e }},
		{"account_full", "/v1/account/root.near/full", func(c *Client) error { _, e := c.AccountFull(context.Background(), "root.near"); return e }},
		{"nft_holdings", "/v1/account/root.near/nft", func(c *Client) error { _, e := c.NFTHoldings(context.Background(), "root.near"); return e }},
		{"staking", "/v1/account/root.near/staking", func(c *Client) error { _, e := c.Staking(context.Background(), "root.near"); return e }},
		{"ft_top", "/v1/ft/usdt.tether-token.near/top", func(c *Client) error {
			_, e := c.FTTopHolders(context.Background(), "usdt.tether-token.near")
			return e
		}},
		{"pk", "/v1/public_key/ed25519:AbC", func(c *Client) error { _, e := c.PublicKeyAccounts(context.Background(), "ed25519:AbC"); return e }},
		{"pk_all", "/v1/public_key/ed25519:AbC/all", func(c *Client) error { _, e := c.PublicKeyAccountsAll(context.Background(), "ed25519:AbC"); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotMethod, gotAuth string
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotMethod, gotAuth = r.URL.Path, r.Method, r.Header.Get("Authorization")
				_, _ = w.Write([]byte(`{}`))
			})
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if gotPath != tc.path {
				t.Fatalf("path = %q, want %q", gotPath, tc.path)
			}
			if gotMethod != http.MethodGet {
				t.Fatalf("method = %q", gotMethod)
			}
			if gotAuth != "Bearer k" {
				t.Fatalf("auth = %q", gotAuth)
			}
		})
	}
}

func TestAccountFullDecode(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"account_id": "root.near",
			"pools": [{"pool_id": "pool.near", "last_update_block_height": 100}],
			"tokens": [{"contract_id": "wrap.near", "balance": "1500", "last_update_block_height": 99}],
			"nfts": [],
			"state": {"balance": "10", "locked": "0", "storage_bytes": 1000}
		}`))
	})
	full, err := c.AccountFull(context.Background(), "root.near")
	if err != nil {
		t.Fatal(err)
	}
	if full.AccountID != "root.near" || len(full.Tokens) != 1 || full.State == nil {
		t.Fatalf("bad decode: %+v", full)
	}
	if *full.Tokens[0].Balance != "1500" {
		t.Fatalf("balance must stay a string: %v", *full.Tokens[0].Balance)
	}
	if len(full.Raw) == 0 {
		t.Fatal("Raw must be populated")
	}
}

func TestPublicKeyValidation(t *testing.T) {
	c := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("request must not be sent for invalid key")
	})
	for _, bad := range []string{"", "abc", "rsa:xx", "ed25519:"} {
		_, err := c.PublicKeyAccounts(context.Background(), bad)
		var e *fastnearerr.Error
		if !errors.As(err, &e) || e.Code != "validation_error" {
			t.Fatalf("key %q: err = %v", bad, err)
		}
	}
}

func TestAccountIDValidation(t *testing.T) {
	c := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("request must not be sent for invalid id")
	})
	for _, bad := range []string{"", "has space.near", "has/slash", string(make([]byte, 65))} {
		_, err := c.AccountFull(context.Background(), bad)
		var e *fastnearerr.Error
		if !errors.As(err, &e) || e.Code != "validation_error" {
			t.Fatalf("id %q: err = %v", bad, err)
		}
	}
}

func TestConfigErr(t *testing.T) {
	c := NewClient(nil, errors.New("cfg"))
	_, err := c.Health(context.Background())
	var e *fastnearerr.Error
	if !errors.As(err, &e) || e.Code != "configuration_error" {
		t.Fatalf("err = %v", err)
	}
}
