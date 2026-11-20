# UPSTREAM-NOTES — расхождения документации и OpenAPI

Правило проекта: матрица эндпоинтов из задания сверяется с живыми
OpenAPI-спеками; при расхождении побеждает спека. Ниже зафиксировано
всё, что было уточнено по живым спекам и ответам серверов
(проверено 2026-11-20).

## RPC (rpc.mainnet.fastnear.com)

1. **42 метода вместо 40** — живой OpenAPI содержит 42 операции
   (план в задании говорил о 40). Реализованы все 42.
2. `gas_price` принимает параметр `[null]` (и массив с block_id) —
   в SDK `GasPrice(ctx, *BlockID)` с `nil` для «текущей» цены.
3. `broadcast_tx_async` тоже принимает `wait_until` (в матрице
   указано только у `send_tx`). SDK: `BroadcastTxAsync` использует
   серверный дефолт; `SendTx` принимает `wait_until` явно.
4. `view_state` и `view_access_key_list` возвращают `last_key`;
   продолжение передаётся как `after_key_base64` / `after_key`.
   Подтверждено схемой OpenAPI.
5. `call_function` требует base64-аргументы; `e30=` — это `{}`.
   SDK кодирует `Args` автоматически (`CallFunction`), либо принимает
   готовый base64 (`CallFunctionRaw`).
6. `finality` обязателен для блок-запросов вида `latest_block`;
   в `query`-параметрах `finality` и `block_id` взаимоисключающие.
   SDK: тип `BlockRef` с конструкторами `Final/AtFinality/AtHeight/AtHash`.
7. `changes` поддерживает `changes_type: account_changes` и другие
   виды — SDK: конструкторы `AccountChanges`, `DataChanges`,
   `AllAccessKeyChanges`.

## NEAR Data (mainnet.neardata.xyz)

8. `GET /v0/last_block/final` отвечает **302** на `/v0/block/{h}`,
   `GET /v0/last_block/optimistic` — **302** на `/v0/block_opt/{h}`,
   `GET /v0/first_block` — **302 на другой хост**
   `https://a0.mainnet.neardata.xyz/v0/block/{h}` (зеркальные хосты
   архива). Транспорт разрешает кросс-хостные редиректы только внутри
   домена базового URL (`.mainnet.neardata.xyz` и т.п.).
9. `GET /v0/block/{h}` для высоты выше финального блока отвечает 404,
   а не редиректом; `block_opt` ведёт себя аналогично.
10. Документ блока — `{block, shards}`; `header.prev_height` = `null`
    на первом блоке (в DTO — `*uint64`); `header.timestamp` — число,
    `timestamp_nanosec` — строка.
11. Скоростной способ узнать высоту — не скачивать мегабайтный блок,
    а читать `Location` 302-ответа: `LastFinalHeight`,
    `LastOptimisticHeight`, `FirstBlockHeight`.

## Transfers (transfers.main.fastnear.com)

12. `block_height` и `block_timestamp` в строках ответа — **строки**
    (не числа), `amount` — строка u128. `min_amount` принимает только
    строку: u128 не помещается в float64.
13. Только mainnet — тестнет-эндпоинта нет в спеке.

## KV FastData (kv.main.fastnear.com)

14. Точный ключ идёт **одним сегментом пути**: `graph/follow/x.near`
    → `graph%2Ffollow%2Fx.near`. SDK кодирует сегмент целиком.
15. Пагинация — `page_token` **отсутствует** на последней странице
    (у tx/transfers `resume_token` присутствует пустой строкой).
    DTO: `PageToken *string`.
16. `entry.block_timestamp` — число наносекунд, а не строка.
17. `key` и `key_prefix` в POST-запросах взаимоисключающие —
    проверяется клиентом до отправки.

## Transactions (tx.main.fastnear.com)

18. Все пять операций — POST; `to_tx_block_height` — исключающая
    верхняя граница, `from_tx_block_height` — включающая.
19. `txs_count` в ответе `/v0/account` — общее число строк выборки,
    а не размер страницы.

## FastNear API (api.fastnear.com)

20. Ответы семейства — JSON; `null`-поля означают «нет данных»
    (`balance: null` у неактивного токена и т.п.) — в DTO указатели.

## Общее

21. Bearer-аутентификация — единственный рекомендуемый способ;
    `?apiKey=` поддержан транспортом для сред без заголовков,
    но ключ при этом маскируется из `EffectiveURL`/ошибок.
