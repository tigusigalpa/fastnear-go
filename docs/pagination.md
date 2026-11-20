# Pagination & iterators

У FastNear три разных механизма пагинации — SDK скрывает их за
единым интерфейсом: итератор `Next(ctx) (items, ok, err)` и
ленивая `iter.Seq2` последовательность.

## Механизмы upstream

| Семейство | Курсор | Конец выборки |
|---|---|---|
| tx `/v0/account` | `resume_token` | пустая строка `""` |
| transfers `/v0/transfers` | `resume_token` | пустая строка `""` |
| kv `POST /v0/...` | `page_token` | поле **отсутствует** в ответе |
| rpc `view_state` | `last_key` | `null` |
| rpc `view_access_key_list` | `last_key` | `null` |

Курсор непрозрачен: передавайте его обратно без изменений — SDK делает
это сам внутри итераторов.

## Ручная страница

```go
q := tx.NewAccountHistoryQuery("intents.near")
page, err := client.Tx().AccountHistory(ctx, q)
if err != nil { ... }
if page.HasMore() {
    next := q.WithResumeToken(page.NextToken())
    page2, _ := client.Tx().AccountHistory(ctx, next)
}
```

## Итератор (контроль + лимит страниц)

```go
it := client.Tx().AccountHistoryIterator(q, 100) // maxPages; 0 = 100
for {
    rows, ok, err := it.Next(ctx)
    if err != nil { return err }
    if !ok { break }
    consume(rows)
}
pos := it.Token() // можно сохранить и продолжить позже
```

Итератор детектирует зацикленный курсор (`pagination_loop`) и
превышение лимита (`pagination_limit`).

## iter.Seq2 (Go 1.23+)

```go
for row, err := range client.Tx().AccountHistorySeq(ctx, q) {
    if err != nil { return err }
    consume(row)
}

for t, err := range client.Transfers().All(ctx, transfers.Query{
    AccountID: "a.near", Direction: transfers.DirectionReceiver,
}) { ... }

for e, err := range client.KV().LatestByAccountSeq(ctx, "social.near", kv.LatestQuery{}) { ... }
```

Seq-методы: `tx.AccountHistorySeq`, `transfers.All`,
`kv.LatestByAccountSeq`, `kv.LatestByPredecessorSeq`,
`kv.HistoryByAccountSeq`, `kv.HistoryByPredecessorSeq`,
`kv.HistoryByKeySeq`, `kv.AllByPredecessorSeq`,
`recipes.StateEntries`, `recipes.AccountTxHistory`,
`recipes.TransferHistory`.

`break` из range корректно останавливает итерацию — лишних запросов нет.
