# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Apply connection timeouts to dialing and TLS, honor proxy environment settings,
  and share the default HTTP connection pool across API families.
- Preserve redirect restrictions and lightweight height probes with custom HTTP
  clients; refuse HTTPS downgrades and keep mirror authentication within the
  original domain allowlist.
- Redact query credentials in transport errors and logs, mask formatted Config
  values, and cap diagnostic bodies and Retry-After arithmetic.
- Apply retry limits to JSON-RPC errors and transaction batches. Transaction
  submission now requires the explicit non-idempotent retry option for both HTTP
  and JSON-RPC failures; WithMaxRetries(0) disables both.
- Send empty JSON arguments as {}, avoid conflicting block references, validate
  RPC response envelopes and batch IDs, and correct protocol-config parameters.
- Decode array-valued maintenance/validator results and nested execution outcomes;
  report the aggregate transaction status, including receipt failures.
- Format negative fractional amounts correctly, support zero-value arithmetic,
  and handle negative token precision without panicking.
- Preserve initial KV cursors, detect cursor cycles, bound contract-state scans,
  and protect cached token metadata from caller mutations.
- Reject overflowing redirect heights and malformed timestamp prefixes; retry
  failed block-header reads without advancing the watcher cursor.

### Changed

- Renamed `transfers.TransfersPage` to `transfers.Page` to avoid repeating the
  package name. Update explicit type references; `Client.Query` returns `*Page`.
- Expanded the README with complete startup, error handling, pagination,
  configuration and transaction examples, plus practical scope limitations.
- Unit HTTP fixtures now use in-memory connections rather than TCP sockets.
- Corrected the Staticcheck GitHub Action reference in CI.

## [1.0.0] - 2026-11-20

### Added

- All 80 FastNear operations across six API families: accounts (FastNear API), RPC (incl. archival), transactions, NEAR Data, transfers and KV FastData.
- Unified error model (`fastnearerr.Error`) with `Is*` helpers and `Retry-After` support.
- HTTP transport with retries, exponential backoff, redirect handling for NEAR Data, response-size cap and `slog` logging.
- Pagination iterators and `iter.Seq2` sequences for all three FastNear pagination mechanisms.
- `Amount`/`TokenAmount` integer-arithmetic types for NEAR and fungible-token balances.
- Recipes: account snapshot, wallet assets, NFT gallery, feeds, block watcher and more.
- Transaction broadcast (`SendTx`, `BroadcastTxAsync`) for externally signed transactions.
