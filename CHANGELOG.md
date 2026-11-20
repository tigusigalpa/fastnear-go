# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-20

### Added

- All 80 FastNear operations across six API families: accounts (FastNear API), RPC (incl. archival), transactions, NEAR Data, transfers and KV FastData.
- Unified error model (`fastnearerr.Error`) with `Is*` helpers and `Retry-After` support.
- HTTP transport with retries, exponential backoff, redirect handling for NEAR Data, response-size cap and `slog` logging.
- Pagination iterators and `iter.Seq2` sequences for all three FastNear pagination mechanisms.
- `Amount`/`TokenAmount` integer-arithmetic types for NEAR and fungible-token balances.
- Recipes: account snapshot, wallet assets, NFT gallery, feeds, block watcher and more.
- Transaction broadcast (`SendTx`, `BroadcastTxAsync`) for externally signed transactions.
