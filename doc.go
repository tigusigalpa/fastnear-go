// Package fastnear is a client for FastNear, an infrastructure provider
// for NEAR Protocol. One API key unlocks six API surfaces:
//
//   - accounts: indexed account views (balances, FT/NFT, staking pools)
//
//   - rpc:      canonical NEAR JSON-RPC (state, blocks, send_tx)
//
//   - tx:       indexed transaction history
//
//   - data:     NEAR Data API (raw block documents, headers, shards)
//
//   - transfers: asset movement history (mainnet only)
//
//   - kv:       indexed contract key-value storage (KV FastData)
//
//     client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
//     if err != nil { ... }
//     full, err := client.Accounts().AccountFull(ctx, "root.near")
//
// See https://docs.fastnear.com/ for the upstream documentation.
package fastnear
