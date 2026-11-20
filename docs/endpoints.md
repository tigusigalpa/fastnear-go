# Endpoint map — все 80 операций

Одна точка входа — `fastnear.Client`; каждое семейство — свой клиент,
свой base URL и своё имя в ошибках (`Family`). Порядок: метод SDK →
upstream path.

## Accounts — `client.Accounts()` — api.fastnear.com

| Метод | HTTP | Path |
|---|---|---|
| `Health` | GET | `/health` |
| `Status` | GET | `/status` |
| `FTContracts` | GET | `/v0/account/{id}/ft` |
| `NFTContracts` | GET | `/v0/account/{id}/nft` |
| `StakingPools` | GET | `/v0/account/{id}/staking` |
| `PublicKeyAccountsLegacy` | GET | `/v0/public_key/{pk}` |
| `PublicKeyAccountsAllLegacy` | GET | `/v0/public_key/{pk}/all` |
| `FTBalances` | GET | `/v1/account/{id}/ft` |
| `AccountFull` | GET | `/v1/account/{id}/full` |
| `NFTHoldings` | GET | `/v1/account/{id}/nft` |
| `Staking` | GET | `/v1/account/{id}/staking` |
| `FTTopHolders` | GET | `/v1/ft/{token}/top` |
| `PublicKeyAccounts` | GET | `/v1/public_key/{pk}` |
| `PublicKeyAccountsAll` | GET | `/v1/public_key/{pk}/all` |

## RPC — `client.RPC()` — rpc.mainnet.fastnear.com (42 метода)

Все методы — `POST /` с JSON-RPC 2.0 телом. Метод SDK = имя
JSON-RPC-метода без `EXPERIMENTAL_` префикса-удобства:

`ViewAccount`, `ViewAccessKey`, `ViewAccessKeyList`, `Block`,
`BlockByID`, `LatestBlock`, `BlockEffects`, `Chunk`, `ChunkByBlockShard`,
`CallFunction`, `CallFunctionRaw`, `ViewState`, `ViewCode`,
`ViewGlobalContractCode`, `ViewGlobalContractCodeByAccountID`,
`GasPrice`, `Health`, `Metrics`, `NetworkInfo`, `Status`,
`GenesisConfig`, `ClientConfig`, `Changes`, `MaintenanceWindows`,
`ExperimentalProtocolConfig`, `ExperimentalCongestionLevel`,
`ExperimentalSplitStorageInfo`, `LightClientProof`,
`ExperimentalLightClientProof`, `NextLightClientBlock`,
`ExperimentalLightClientBlockProof`, `SendTx`, `BroadcastTxAsync`,
`BroadcastTxCommit`, `TxStatus`, `Tx`, `ExperimentalTxStatus`,
`ExperimentalReceipt`, `ExperimentalReceiptToTx`, `Validators`,
`ValidatorsByEpoch`, `Batch` (любые методы одним запросом).

## Transactions — `client.Tx()` — tx.main.fastnear.com

| Метод | HTTP | Path |
|---|---|---|
| `AccountHistory` (+`AccountHistoryIterator`, `AccountHistorySeq`) | POST | `/v0/account` |
| `Block` | POST | `/v0/block` |
| `Blocks` | POST | `/v0/blocks` |
| `Receipt` | POST | `/v0/receipt` |
| `Transactions` | POST | `/v0/transactions` |

## NEAR Data — `client.Data()` — mainnet.neardata.xyz

| Метод | HTTP | Path | Примечание |
|---|---|---|---|
| `Health` | GET | `/health` | |
| `FirstBlock` / `FirstBlockHeight` | GET | `/v0/first_block` | 302 на зеркальный хост |
| `LastBlockFinal` / `LastFinalHeight` | GET | `/v0/last_block/final` | 302 → `/v0/block/{h}` |
| `LastBlockOptimistic` / `LastOptimisticHeight` | GET | `/v0/last_block/optimistic` | 302 → `/v0/block_opt/{h}` |
| `Block` | GET | `/v0/block/{h}` | |
| `BlockOpt` | GET | `/v0/block_opt/{h}` | |
| `BlockHeaders` | GET | `/v0/block/{h}/headers` | лёгкий формат |
| `Chunk` | GET | `/v0/block/{h}/chunk/{shard}` | |
| `Shard` | GET | `/v0/block/{h}/shard/{shard}` | |

## Transfers — `client.Transfers()` — transfers.main.fastnear.com (mainnet only)

| Метод | HTTP | Path |
|---|---|---|
| `Query` (+`Iterator`, `All`) | POST | `/v0/transfers` |

## KV FastData — `client.KV()` — kv.main.fastnear.com

| Метод | HTTP | Path |
|---|---|---|
| `LatestByExactKey` | GET | `/v0/latest/{contract}/{predecessor}/{key}` |
| `HistoryByExactKey` | GET | `/v0/history/{contract}/{predecessor}/{key}` |
| `LatestByAccount` (+Seq) | POST | `/v0/latest/{contract}` |
| `LatestByPredecessor` (+Seq) | POST | `/v0/latest/{contract}/{predecessor}` |
| `HistoryByAccount` (+Seq) | POST | `/v0/history/{contract}` |
| `HistoryByPredecessor` (+Seq) | POST | `/v0/history/{contract}/{predecessor}` |
| `HistoryByKey` (+Seq) | POST | `/v0/history` |
| `AllByPredecessor` (+Seq) | POST | `/v0/all/{predecessor}` |
| `Multi` | POST | `/v0/multi` |

Итого: 14 + 42 + 5 + 9 + 1 + 9 = **80 операций**.
