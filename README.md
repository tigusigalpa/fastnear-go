# FastNear Golang Client/SDK/Library

![FastNear API Client/SDK/Library](https://i.postimg.cc/TYHn1gSX/fastnear-go-banner-api.jpg)

A Go client for [FastNear](https://docs.fastnear.com/) and NEAR Protocol.
Read balances, call contracts, walk transaction history, inspect blocks, or submit
a transaction you have already signed. Six API families share one client, while
each service keeps its own methods and response types.

[![CI](https://github.com/tigusigalpa/fastnear-go/actions/workflows/ci.yml/badge.svg)](https://github.com/tigusigalpa/fastnear-go/actions/workflows/ci.yml)
[![Tests](https://github.com/tigusigalpa/fastnear-go/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/tigusigalpa/fastnear-go/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tigusigalpa/marketaux-go.svg)](https://pkg.go.dev/github.com/tigusigalpa/marketaux-go)
[![CodeQL](https://github.com/tigusigalpa/fastnear-go/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/tigusigalpa/fastnear-go/actions/workflows/codeql.yml)
[![Codecov](https://codecov.io/gh/tigusigalpa/fastnear-go/graph/badge.svg)](https://codecov.io/gh/tigusigalpa/fastnear-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.23-blue.svg)](go.mod)
[![GitHub Release](https://img.shields.io/github/v/release/tigusigalpa/fastnear-go?style=flat-square)](https://github.com/tigusigalpa/fastnear-go/releases)

Requires **Go 1.23+**. Uses only the standard library, with context cancellation,
bounded retries, exact balance formatting, page iterators and `iter.Seq2` streams.

## Install

```sh
go get github.com/tigusigalpa/fastnear-go@latest
```

Get an API key from the [FastNear dashboard](https://dashboard.fastnear.com/).
The client permits requests without a key; acceptance and anonymous limits are
up to the service. Configure a key explicitly for deployed applications.

## Your first request

Set `FASTNEAR_API_KEY` in your environment, then run this program:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    fastnear "github.com/tigusigalpa/fastnear-go"
)

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error {
    client, err := fastnear.NewClient(
        fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")),
    )
    if err != nil {
        return err
    }
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    account, err := client.Accounts().AccountFull(ctx, "root.near")
    if err != nil {
        return err
    }
    if account == nil {
        fmt.Println("No indexed account data returned")
        return nil
    }
    fmt.Printf("%d tokens, %d staking pools\n", len(account.Tokens), len(account.Pools))
    return nil
}
```

Reuse one client across goroutines. Family clients are created lazily and share
an HTTP connection pool. `Close` releases idle connections; it does not cancel
active requests or permanently disable the client. Use a context to stop work.

## Choose the right API

| Accessor | Use it for | Mainnet host | Testnet |
|---|---|---|---|
| `client.Accounts()` | Indexed tokens, NFTs, staking and public-key lookups | `api.fastnear.com` | Yes |
| `client.RPC()` | On-chain state, contract calls, blocks and transaction submission | `rpc.mainnet.fastnear.com` | Yes |
| `client.Tx()` | Indexed transaction and receipt history | `tx.main.fastnear.com` | Yes |
| `client.Data()` | Block documents, headers, chunks and shards | `mainnet.neardata.xyz` | Yes |
| `client.Transfers()` | Native NEAR and FT movements | `transfers.main.fastnear.com` | No |
| `client.KV()` | Indexed contract values and write history | `kv.main.fastnear.com` | Yes |

Start with `Accounts` for a wallet-style overview. Use `RPC` when a specific
on-chain state or block matters. Indexed views can lag behind the chain;
responses from different families are not an atomic snapshot.

Select testnet with `fastnear.WithTestnet()`. Using `Transfers` on testnet returns
a configuration error unless you provide an explicit base URL override.
Add `fastnear.WithArchival()` for older RPC state. That option changes the RPC
endpoint, leaving other families on their configured endpoints.

See the [endpoint map](docs/endpoints.md) for individual methods and
[upstream notes](docs/UPSTREAM-NOTES.md) for documentation discrepancies.

## Common tasks

These snippets assume an initialized `client` and `ctx`. Import each family
package you use, for example `github.com/tigusigalpa/fastnear-go/rpc`.

### Read an exact NEAR balance

```go
account, err := client.RPC().ViewAccount(ctx, "root.near", nil)
if err != nil {
    return err
}
if account == nil {
    return fmt.Errorf("no account state returned")
}
balance, err := fastnear.FromYocto(account.Amount)
if err != nil {
    return err
}
fmt.Println(balance.NEAR(), "NEAR")
```

A nil block reference requests finalized state. For a particular block, create
`ref := rpc.AtHeight(height)` or `rpc.AtHash(hash)` and pass `&ref`.
`rpc.AtFinality(rpc.FinalityOptimistic)` requests fresher state that can still
change before finalization.

### Call a contract view method

```go
result, err := client.RPC().CallFunction(ctx, rpc.CallFunctionRequest{
    AccountID:  "wrap.near",
    MethodName: "ft_metadata",
})
if err != nil {
    return err
}
if result == nil || result.IsEmpty() {
    return fmt.Errorf("ft_metadata returned no data")
}
var metadata struct {
    Name     string `json:"name"`
    Symbol   string `json:"symbol"`
    Decimals uint8  `json:"decimals"`
}
if err := result.JSON(&metadata); err != nil {
    return err
}
fmt.Println(metadata.Name, metadata.Symbol)
```

`CallFunction` JSON-encodes `Args`, then base64-encodes the bytes for NEAR.
Omitting `Args` sends `{}`. Use `CallFunctionRaw` with `ArgsBase64` when you
already have encoded arguments.

### Walk transaction history

```go
query := tx.NewAccountHistoryQuery("intents.near")
for row, err := range client.Tx().AccountHistorySeq(ctx, query) {
    if err != nil {
        return err
    }
    fmt.Println(row.TransactionHash)
}
```

Rows arrive a page at a time. Breaking out of the loop stops further requests.
A stream can emit rows and then fail, so make consumers safe to resume rather
than assuming a failed stream produced no data.

### Poll the finalized block height

```go
height, err := client.Data().LastFinalHeight(ctx)
if err != nil {
    return err
}
fmt.Println("Finalized height:", height)
```

Height helpers read the redirect's `Location` without downloading the block.
Use `BlockHeaders` for headers and `Block` for a full document with shard
payloads. Full blocks can be large; the default response limit is 32 MiB.
Optimistic blocks can change before finalization.

### Read a KV value

```go
values, err := client.KV().LatestByExactKey(ctx,
    "social.near", "james.near", "graph/follow/sleet.near", false)
if err != nil {
    return err
}
if values != nil {
    for _, entry := range values.Entries {
        fmt.Printf("%s = %s\n", entry.Key, entry.Value)
    }
}
```

Pass raw keys, including `/` and `%`. The SDK escapes a key as one path segment;
pre-escaping it yourself encodes it twice. Values are arbitrary JSON:
`entry.ValueAs(&target)` decodes a value, and `entry.ValueIsNull()` checks for null.

### Submit an already signed transaction

```go
outcome, err := client.RPC().SendTx(ctx, signedTxBase64, rpc.WaitFinal)
if err != nil {
    return err
}
if outcome == nil || outcome.Status() == nil {
    return fmt.Errorf("no aggregate execution status returned")
}
if !outcome.Status().IsSuccess() {
    return fmt.Errorf("transaction did not succeed: %+v", outcome.Status())
}
```

You provide the signed base64 payload. The SDK does not hold private keys,
serialize Borsh transactions or sign them. A successful RPC call means a
response was received; inspect the aggregate status for contract or receipt
failures. Earlier wait levels may not have an aggregate status yet.

Submission calls are not retried by default. If a response is lost, the
transaction may still have reached the node. Check its status by hash and sender
before deciding to submit again.

## Configuration

Options are applied in order. Invalid options fail `NewClient`; unsupported
family/network combinations are reported when that family is used.

| Option | Default | Controls |
|---|---|---|
| `WithAPIKey(key)` | Empty | Credential; an empty key omits authentication |
| `WithTestnet()` / `WithNetwork(network)` | Mainnet | Network for all families |
| `WithArchival()` | Off | Archival endpoint for RPC |
| `WithTimeout(duration)` | 30 s | Each HTTP attempt, including reading its body |
| `WithConnectTimeout(duration)` | 5 s | Dial and TLS handshake timeouts for the built-in transport |
| `WithMaxRetries(count)` | 2 | Extra attempts; zero disables retries |
| `WithRetryBaseDelay(duration)` | 200 ms | Starting backoff delay |
| `WithMaxResponseBytes(bytes)` | 32 MiB | Maximum response body size |
| `WithHTTPClient(client)` | Built-in client | Custom transport, cookie jar and HTTP client timeout |
| `WithBaseURL(family, url)` | Family endpoint | Proxy, staging service or test fixture for one family |
| `WithLogger(logger)` | Off | Structured logging with `log/slog` |
| `WithMiddleware(hook)` | None | Observation hook for every HTTP attempt |
| `WithNonIdempotentRetry(true)` | Off | Permit transaction submission retries |
| `WithAuthMode(fastnear.AuthQuery)` | Bearer | Put the key in `apiKey` instead of the Authorization header |
| `WithUserAgent(value)` | `fastnear-go/<version> (...)` | User-Agent header |

```go
client, err := fastnear.NewClient(
    fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")),
    fastnear.WithTestnet(),
    fastnear.WithArchival(),
    fastnear.WithTimeout(20*time.Second),
    fastnear.WithConnectTimeout(3*time.Second),
    fastnear.WithMaxRetries(1),
    fastnear.WithMaxResponseBytes(64<<20),
    fastnear.WithLogger(slog.Default()),
)
```

Use a context deadline to bound the **whole operation**, including retries and
backoff. `WithTimeout` applies to one attempt. The retry budget is 15 seconds
from the first attempt's start: the initial attempt can use its full timeout,
while subsequent attempts are limited to the remaining retry budget.

The built-in transport respects `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`.
With a custom `http.Client`, the SDK copies the client and shares its transport
and cookie jar without modifying the original. SDK redirect checks run before
your `CheckRedirect` callback. Configure a custom transport's dialer and TLS
timeout yourself; `WithConnectTimeout` does not alter it.

Redirects stay on the original host, except NEAR Data may also use mirrors below
its configured base domain. HTTPS downgrades are refused. Height probes still
avoid following redirects with custom clients. Base URL overrides accept
absolute HTTP(S) URLs, with optional path prefixes and no embedded credentials,
query parameters or fragments.

Prefer Bearer auth. Query auth can expose a key in external access logs even
though SDK errors and logs scrub it. Middleware receives the actual request and
response, including credentials: record only what you need, protect shared hook
state, and avoid dumping headers or URLs.

## Errors and missing data

Request failures use `*fastnear.Error`, an alias of `*fastnearerr.Error`.
`NewClient` option errors are ordinary Go errors. Amount parsers return
structured errors too.

```go
if err != nil {
    switch {
    case errors.Is(err, context.Canceled):
        return err
    case fastnear.IsNotFound(err):
        fmt.Println("Resource not found")
    case fastnear.IsQuotaExceeded(err):
        return fmt.Errorf("FastNear quota exhausted: %w", err)
    default:
        if detail, ok := fastnear.As(err); ok {
            log.Printf("%s: status=%d code=%s retry_after=%s",
                detail.Op, detail.StatusCode, detail.Code, detail.RetryAfter)
        }
        return err
    }
}
```

An HTTP 404 is an error. A successful JSON `null` generally returns `(nil, nil)`.
Some indexed lookups return a non-nil envelope with a nil member instead, such
as the `Block` field of `tx.BlockPage`. Check the documented result shape before
dereferencing pointers. Empty or malformed JSON is a decoding error.

Retries cover network failures, HTTP 408/425/429 and 5xx responses. Valid
`Retry-After` headers are honored when they fit the remaining budget. JSON-RPC
`-32000` can be retried once within the same configured attempt limit.
Authentication failures, quota exhaustion, invalid arguments, malformed responses
and canceled contexts are terminal. `WithMaxRetries(0)` disables HTTP and RPC
retries. Batches containing submission methods follow the submission retry rule.

The client has already applied its retry policy when it returns an error.
Account for that if you add an application-level retry loop. See
[the error reference](docs/errors.md) for fields and helpers.

## Pagination and resuming work

| Family | Continuation field | End of results |
|---|---|---|
| Transactions and transfers | `resume_token` | Empty string |
| KV FastData | `page_token` | Field absent or null |
| RPC state/key lists | `last_key` | Null |

Tokens are opaque: save them unchanged and keep the same filters when resuming.
For page-level processing and checkpoints:

```go
query := tx.NewAccountHistoryQuery("intents.near")
it := client.Tx().AccountHistoryIterator(query, 100)
for {
    rows, ok, err := it.Next(ctx)
    if err != nil {
        return err
    }
    if !ok {
        break
    }
    // Process and persist the rows before saving it.Token().
    fmt.Println("Rows:", len(rows), "next cursor:", it.Token())
}
```

Resume with `query.WithResumeToken(savedToken)`. KV sequences preserve a query's
initial `PageToken`. Iterators detect repeated cursors and default to a cap of
**100 pages**. Transaction and transfer iterator constructors accept a different
cap. Hitting a cap returns `pagination_limit` rather than silently truncating
results. The state-scan recipe also stops after 100 pages; use `RPC().ViewState`
directly for longer scans or to pin each page to a specific block.

An iterator is stateful and must not be called concurrently. Create separate
iterators for independent consumers. More examples: [pagination](docs/pagination.md).

## Balances without floating-point rounding

`fastnear.Amount` stores integer yoctoNEAR: one NEAR is 10^24 yoctoNEAR.
`FromNEAR("1.5")`, `FromYocto("1500000000000000000000000")`, `NEAR()` and
`Yocto()` preserve exact values. Arithmetic returns new amounts. Subtraction can
produce negative values; zero-value and nil amounts behave as zero.

For fungible tokens, pair the raw integer with the contract's decimals:

```go
balance := fastnear.TokenAmount{Raw: "1234567", Decimals: 6}
fmt.Println(balance.Format(4)) // 1.2345, truncated rather than rounded
```

Negative precision is treated as zero. Invalid raw input formats as `"0"`, so
`TokenAmount.Format` is a display helper rather than an input validator.
Transfer fields such as `HumanAmount` and `UsdAmount` are approximate floats
from upstream; use raw amount strings for accounting.

Response DTOs retain raw JSON in `Raw` for fields not yet represented by typed
members. Integer timestamps and heights remain integers; upstream strings stay
strings. Default decoding into `map[string]any` uses float64, so use typed fields
or `json.Decoder.UseNumber` when inspecting large numbers in raw responses.

## Recipes and runnable examples

`recipes.New(client)` combines account snapshots, FT balances with a contract-call
fallback, NFT and staking views, KV reads, state scans and history streams.
Snapshots combine independent indexed and RPC reads, without a shared-height
guarantee. See [the recipes reference](docs/recipes.md).

`recipes.NewTokenRegistry(client)` caches `ft_metadata` to format token balances.
Reuse it and call `Forget(tokenID)` after metadata changes. Entries live for the
registry's lifetime, so account for cache growth when scanning many contracts.

The [examples directory](examples/) is a separate Go module using the local
checkout through a `replace` directive. After setting your API key:

```sh
cd examples
go run ./snapshot root.near
go run ./watch-blocks
go run ./account-history intents.near
go run ./ft-balance root.near wrap.near
go run ./fastdata
go run ./send-tx "$SIGNED_TX_BASE64"
```

PowerShell environment setup:

```powershell
$env:FASTNEAR_API_KEY = "your-key"
```

For Bash or another POSIX shell, use `export FASTNEAR_API_KEY="your-key"`.

## Development and scope

From the repository root:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Build examples separately with `go build ./...` from `examples/`. Unit tests
use HTTP over in-memory connections and need neither a key nor network access.
Live smoke tests are opt-in: set `FASTNEAR_INTEGRATION=1` and run
`go test -tags=integration ./...`. They contact FastNear and may consume quota.

This is an HTTP client. It does not provide WebSocket subscriptions or run a
local indexer. Availability, retention, indexing delay and API limits are
properties of the upstream services.

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines and
[SECURITY.md](SECURITY.md) for private vulnerability reporting.

## License

[MIT](LICENSE).
