package fastnear

import (
	"errors"
	"fmt"
)

// Network selects the NEAR deployment the client talks to.
type Network string

const (
	// Mainnet is the production NEAR network.
	Mainnet Network = "mainnet"
	// Testnet is the public NEAR test network.
	Testnet Network = "testnet"
)

// Family identifies one of the six FastNear API surfaces.
type Family string

const (
	// FamilyAccounts is the FastNear API: indexed account views.
	FamilyAccounts Family = "accounts"
	// FamilyRPC is the canonical NEAR JSON-RPC endpoint.
	FamilyRPC Family = "rpc"
	// FamilyTx is the Transactions API: indexed history.
	FamilyTx Family = "tx"
	// FamilyData is the NEAR Data API: raw block documents.
	FamilyData Family = "data"
	// FamilyTransfers is the Transfers API (mainnet only).
	FamilyTransfers Family = "transfers"
	// FamilyKV is the KV FastData API: contract key-value index.
	FamilyKV Family = "kv"
)

// AuthMode selects how the API key is transmitted.
type AuthMode string

const (
	// AuthBearer sends "Authorization: Bearer <key>" (default).
	AuthBearer AuthMode = "bearer"
	// AuthQuery appends "?apiKey=<key>" to every request. Use only
	// where headers are unavailable: the key may end up in logs.
	AuthQuery AuthMode = "query"
)

// baseURLs is the default endpoint matrix per family and network.
var baseURLs = map[Family]map[Network]string{
	FamilyAccounts: {
		Mainnet: "https://api.fastnear.com",
		Testnet: "https://test.api.fastnear.com",
	},
	FamilyRPC: {
		Mainnet: "https://rpc.mainnet.fastnear.com",
		Testnet: "https://rpc.testnet.fastnear.com",
	},
	FamilyTx: {
		Mainnet: "https://tx.main.fastnear.com",
		Testnet: "https://tx.test.fastnear.com",
	},
	FamilyData: {
		Mainnet: "https://mainnet.neardata.xyz",
		Testnet: "https://testnet.neardata.xyz",
	},
	FamilyTransfers: {
		Mainnet: "https://transfers.main.fastnear.com",
	},
	FamilyKV: {
		Mainnet: "https://kv.main.fastnear.com",
		Testnet: "https://kv.test.fastnear.com",
	},
}

var archivalBaseURLs = map[Network]string{
	Mainnet: "https://archival-rpc.mainnet.fastnear.com",
	Testnet: "https://archival-rpc.testnet.fastnear.com",
}

// SupportsTestnet reports whether the family has a testnet endpoint.
// The Transfers API is mainnet-only.
func (f Family) SupportsTestnet() bool {
	_, ok := baseURLs[f][Testnet]
	return ok
}

// BaseURL resolves the default base URL for the family on the given
// network. When archival is true and the family is RPC, the archival
// endpoint is returned. An error is returned for unsupported
// combinations (e.g. Transfers on testnet).
func (f Family) BaseURL(n Network, archival bool) (string, error) {
	if f == FamilyRPC && archival {
		u, ok := archivalBaseURLs[n]
		if !ok {
			return "", fmt.Errorf("fastnear: unknown network %q", n)
		}
		return u, nil
	}
	byNet, ok := baseURLs[f]
	if !ok {
		return "", fmt.Errorf("fastnear: unknown family %q", f)
	}
	u, ok := byNet[n]
	if !ok {
		if f == FamilyTransfers {
			return "", errors.New("fastnear: Transfers API is mainnet-only; testnet is not supported")
		}
		return "", fmt.Errorf("fastnear: family %q does not support network %q", f, n)
	}
	return u, nil
}
