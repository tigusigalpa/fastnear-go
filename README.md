# fastnear-go

FastNear API client for Go: RPC, indexed accounts, transactions, transfers, NEAR Data and KV FastData.

[![Go Reference](https://pkg.go.dev/badge/github.com/tigusigalpa/fastnear-go.svg)](https://pkg.go.dev/github.com/tigusigalpa/fastnear-go)
[![CI](https://github.com/tigusigalpa/fastnear-go/actions/workflows/ci.yml/badge.svg)](https://github.com/tigusigalpa/fastnear-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **Status:** scaffold. Documentation is completed in Phase 6 — see `docs/`.

## Install

```sh
go get github.com/tigusigalpa/fastnear-go@latest
```

## Quick start

```go
client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
if err != nil {
    log.Fatal(err)
}
defer client.Close()

full, err := client.Accounts().AccountFull(context.Background(), "root.near")
```

One API key works across all six surfaces. Get a key at https://dashboard.fastnear.com/.

## License

MIT — see [LICENSE](LICENSE).
