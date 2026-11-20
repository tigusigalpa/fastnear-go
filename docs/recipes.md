# Recipes — готовые сценарии

Пакет `recipes` собирает типовые сценарии из нескольких вызовов.
Тонкий слой: для полного контроля используйте семейные клиенты.

```go
svc := recipes.New(client)
```

## 1. AccountSnapshot — полный снимок аккаунта

```go
snap, err := svc.AccountSnapshot(ctx, "root.near")
// snap.Full — индексированные балансы (accounts API)
// snap.OnChain — view_account (RPC final)
// snap.Balance, snap.Locked — *fastnear.Amount
// snap.StorageUsage — байты
```

## 2. FTBalance — баланс токена (с фолбэком)

```go
raw, found, err := svc.FTBalance(ctx, "a.near", "wrap.near")
// Сначала индекс (/v1/account/{id}/ft), затем ft_balance_of.
```

## 3. NFTContracts — NFT-контракты аккаунта

```go
nfts, err := svc.NFTContracts(ctx, "a.near")
```

## 4. StakingSummary — стейкинг-пулы

```go
pools, err := svc.StakingSummary(ctx, "a.near")
```

## 5. TopHolders — топ держателей FT

```go
top, err := svc.TopHolders(ctx, "usdt.tether-token.near")
```

## 6. AccountsByPublicKey — аккаунты по ключу

```go
lookup, err := svc.AccountsByPublicKey(ctx, "ed25519:...")
```

## 7. ViewCallJSON — вызов view-метода контракта

```go
var meta struct{ Name string `json:"name"`; Decimals uint8 `json:"decimals"` }
err := svc.ViewCallJSON(ctx, "wrap.near", "ft_metadata", nil, &meta)

// с аргументами:
var bal string
err = svc.ViewCallJSON(ctx, "wrap.near", "ft_balance_of",
    map[string]any{"account_id": "a.near"}, &bal)
```

## 8. StateEntries — полный скан состояния контракта

```go
for item, err := range svc.StateEntries(ctx, "lockup.near", recipes.EncodePrefix("STATE"), nil) {
    if err != nil { ... }
    _ = item.Key // []byte
}
```

Ходит по `last_key`-страницам `view_state` автоматически.

## 9. KVValue — значение по точному ключу

```go
var v map[string]any
found, err := svc.KVValue(ctx, "social.near", "james.near",
    "graph/follow/sleet.near", &v)
```

## 10. AccountTxHistory — история транзакций (стрим)

```go
q := tx.NewAccountHistoryQuery("intents.near")
desc := true
q.Desc = &desc
for row, err := range svc.AccountTxHistory(ctx, q) { ... }
```

## 11. TransferHistory — история переводов (стрим)

```go
for t, err := range svc.TransferHistory(ctx, "a.near", transfers.DirectionReceiver) { ... }
```

## NEAR Data helpers

```go
h, err := svc.LatestFinalHeight(ctx)          // дёшево: читает 302 Location
env, err := svc.BlockHeaders(ctx, h)          // лёгкий конверт блока
doc, err := svc.Block(ctx, h)                 // полный документ (может быть ~MB)
```

## TokenRegistry — метаданные и форматирование FT

```go
reg := recipes.NewTokenRegistry(client)

meta, err := reg.Metadata(ctx, "usdt.tether-token.near")  // ft_metadata, кэшируется

bal, found, err := reg.BalanceOf(ctx, "a.near", "usdt.tether-token.near")
if found {
    fmt.Println(bal.Format(2), bal.Meta.Symbol)           // "1.50 USDt"
    fmt.Println(bal.Raw)                                  // "1500000"
}

s, err := reg.FormatAmount(ctx, "usdt.tether-token.near", "1500000", 6)

reg.Forget("usdt.tether-token.near") // сбросить кэш после апгрейда токена
```

Все суммы считаются целочисленно (`fastnear.TokenAmount`,
`fastnear.Amount`) — float64 не используется для точных значений.
