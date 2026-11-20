# fastnear-go

FastNear API client for Go: RPC, indexed accounts, transactions, transfers, NEAR Data and KV FastData.

[![Go Reference](https://pkg.go.dev/badge/github.com/tigusigalpa/fastnear-go.svg)](https://pkg.go.dev/github.com/tigusigalpa/fastnear-go)
[![CI](https://github.com/tigusigalpa/fastnear-go/actions/workflows/ci.yml/badge.svg)](https://github.com/tigusigalpa/fastnear-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

One API key unlocks six FastNear surfaces. All **80 operations** are
implemented as typed methods with context support, exact integer
arithmetic for balances, retries, pagination iterators and
`iter.Seq2` streams (Go 1.23+).

## Install

```sh
go get github.com/tigusigalpa/fastnear-go@latest
```

## Quick start

```go
package main

import (
    "context"
    "log"
    "os"

    fastnear "github.com/tigusigalpa/fastnear-go"
)

func main() {
    client, err := fastnear.NewClient(
        fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()
    full, err := client.Accounts().AccountFull(ctx, "root.near")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("tokens: %d, pools: %d", len(full.Tokens), len(full.Pools))
}
```

Get a key at <https://dashboard.fastnear.com/>. Requests work
anonymously too, with lower rate limits.

## Six families

| Accessor | Surface | Base URL (mainnet) |
|---|---|---|
| `client.Accounts()` | Indexed account views | `api.fastnear.com` |
| `client.RPC()` | NEAR JSON-RPC (42 methods) | `rpc.mainnet.fastnear.com` |
| `client.Tx()` | Indexed tx/receipt/block history | `tx.main.fastnear.com` |
| `client.Data()` | Raw block documents | `mainnet.neardata.xyz` |
| `client.Transfers()` | Asset movement history | `transfers.main.fastnear.com` |
| `client.KV()` | Contract KV storage | `kv.main.fastnear.com` |

Every method takes a `context.Context` first. Transfers is
mainnet-only; the other families work on testnet via
`fastnear.WithTestnet()`. `client.RPC()` supports
`fastnear.WithArchival()` for the archival node.

## Examples

```go
// Indexed view: tokens + NFTs + staking pools in one call.
full, _ := client.Accounts().AccountFull(ctx, "root.near")

// On-chain view: native balance as exact integer amount.
acct, _ := client.RPC().ViewAccount(ctx, "root.near", nil)
near, _ := fastnear.FromYocto(acct.Amount)

// Contract view call with JSON args.
res, _ := client.RPC().CallFunction(ctx, rpc.CallFunctionRequest{
    AccountID:  "wrap.near",
    MethodName: "ft_metadata",
})
var meta struct{ Name string `json:"name"` }
_ = res.JSON(&meta)

// Transaction history, streamed.
q := tx.NewAccountHistoryQuery("intents.near")
for row, err := range client.Tx().AccountHistorySeq(ctx, q) { ... }

// Cheap chain-progress probe (reads the 302 Location, not the block).
height, _ := client.Data().LastFinalHeight(ctx)

// KV FastData: '/' inside a key is encoded as %2F automatically.
list, _ := client.KV().LatestByExactKey(ctx,
    "social.near", "james.near", "graph/follow/sleet.near", false)

// Send a PRE-SIGNED transaction (signing is out of scope).
outcome, _ := client.RPC().SendTx(ctx, signedTxBase64, rpc.WaitFinal)
```

## Configuration

```go
client, err := fastnear.NewClient(
    fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")),
    fastnear.WithTestnet(),                 // or WithNetwork()
    fastnear.WithTimeout(30*time.Second),
    fastnear.WithMaxRetries(2),             // retries: 429 + 5xx + transport
    fastnear.WithRetryBaseDelay(200*time.Millisecond),
    fastnear.WithMaxResponseBytes(32<<20),  // NEAR Data blocks are large
    fastnear.WithHTTPClient(customHTTP),    // optional
    fastnear.WithLogger(slog.Default()),    // slog, never logs the key
    fastnear.WithBaseURL(fastnear.FamilyData, "https://staging..."),
    fastnear.WithMiddleware(func(req, resp, elapsed) { ... }),
)
```

`WithNonIdempotentRetry(true)` additionally retries `send_tx`/
`broadcast_tx*` (nonce-safe on NEAR, off by default).

## Money and time

Balances are u128 strings upstream — the SDK never uses float64 for
exact values:

- `fastnear.Amount` — NEAR/yoctoNEAR (`FromYocto`, `FromNEAR`,
  `NEAR()`, `Yocto()`), 10^24 yocto per NEAR.
- `fastnear.TokenAmount{Raw, Decimals}` — FT balances,
  `Format(precision)` for human output.
- `recipes.TokenRegistry` — caches `ft_metadata`, formats balances.

Timestamps arrive as ns strings or numbers; block heights are
`uint64`. Large values stay exact.

## Errors

One error type — `fastnear.Error` (`Op`, `Family`, `StatusCode`,
`Code`, `Message`, `RequestURL`, `RetryAfter`, `RPC`):

```go
if fe, ok := fastnear.As(err); ok && fe.Code == "rate_limited" {
    time.Sleep(fe.RetryAfter)
}
```

See [docs/errors.md](docs/errors.md) for the full code table and
retry policy. The API key never appears in errors, logs or
`EffectiveURL`.

## Pagination

`resume_token` (tx, transfers), `page_token` (kv) and `last_key`
(rpc) are all handled by iterators and `iter.Seq2` streams —
see [docs/pagination.md](docs/pagination.md).

## Recipes

`recipes.New(client)` bundles common multi-call scenarios: account
snapshots, token balances with `ft_balance_of` fallback, contract
state scans, KV reads, history streams and a caching token registry —
see [docs/recipes.md](docs/recipes.md).

## Runnable examples

`examples/` is a separate module (replace-pinned to the SDK):

```sh
cd examples
FASTNEAR_API_KEY=... go run ./snapshot root.near
FASTNEAR_API_KEY=... go run ./watch-blocks
FASTNEAR_API_KEY=... go run ./account-history intents.near
FASTNEAR_API_KEY=... go run ./ft-balance a.near wrap.near
FASTNEAR_API_KEY=... go run ./fastdata
FASTNEAR_API_KEY=... go run ./send-tx <signed_tx_base64>
```

## Scope notes

- Read-only + `sendTx`: Borsh serialization and ed25519 signing are
  intentionally out of scope — bring your own signed payload.
- Fixtures contain no real keys, e-mails, IPs or personal data.
- Upstream vs spec discrepancies: [docs/UPSTREAM-NOTES.md](docs/UPSTREAM-NOTES.md).
- Full endpoint map (80 ops): [docs/endpoints.md](docs/endpoints.md).

## Contributing & security

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
Report vulnerabilities privately per SECURITY.md — do not file public
issues with secrets or personal data.

## License

MIT — see [LICENSE](LICENSE).
