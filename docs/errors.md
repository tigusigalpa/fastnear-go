# Error model

Все вызовы SDK возвращают один тип — `*fastnearerr.Error`
(также доступен как `fastnear.Error`).

```go
type Error struct {
    Op         string         // "family.operation", напр. "tx.account"
    Family     string         // "accounts" | "rpc" | "tx" | "data" | "transfers" | "kv"
    StatusCode int            // HTTP status; 0 — сетевой/локальный сбой
    Code       string         // машинный код (см. таблицу)
    Message    string         // человекочитаемое описание
    RequestURL string         // эффективный URL без apiKey
    RawBody    []byte         // тело ответа (до 8 KiB)
    RetryAfter time.Duration  // из заголовка Retry-After (429/503)
    RPC        *RPCError      // заполнено для JSON-RPC ошибок
    Err        error          // обёрнутая причина (Unwrap)
}
```

`err.Error()` никогда не содержит API-ключа: `apiKey` вычищается из
URL ещё до попадания в ошибку. Проверено тестом
`fastnearerr.TestErrorStringNoSecrets`.

## Коды

| Code | Когда |
|---|---|
| `validation_error` | локальная проверка аргументов до запроса |
| `configuration_error` | клиент создан с ошибкой конфигурации |
| `serialization_error` | не удалось кодировать запрос или декодировать ответ |
| `transport_error` | DNS/TLS/сокет ошибка (StatusCode = 0) |
| `timeout` / `canceled` | контекст |
| `response_too_large` | тело превысило `WithMaxResponseBytes` |
| `redirect_refused` | кросс-хостный редирект вне allowlist |
| `bad_request` (400) | сервер отверг запрос |
| `authentication_error` (401/403) | ключ отсутствует/невалиден |
| `quota_exceeded` (402) | тарифный лимит |
| `not_found` (404) | объект не найден |
| `rate_limited` (429) | rate limit; см. `RetryAfter` |
| `server_error` (5xx) | сбой апстрима |
| `pagination_limit` / `pagination_loop` | итераторы: лимит страниц или зацикленный курсор |

JSON-RPC ошибки декодируются в `RPCError{Code, Message, Data}` и
доступны через `e.RPC`.

## Хелперы

```go
fastnear.IsNotFound(err)      // 404
fastnear.IsAuth(err)          // 401/403
fastnear.IsQuotaExceeded(err) // 402
fastnear.IsRateLimited(err)   // 429 (+ e.RetryAfter)
fastnear.IsRetryable(err)     // 429, 5xx, транспортные сбои
fe, ok := fastnear.As(err)    // *Error из цепочки
```

## Ретраи

- Повторяются: 429 (с учётом `Retry-After`), 5xx, транспортные ошибки.
- Не повторяются: 4xx (кроме 429), ошибки валидации/сериализации,
  отменённый контекст.
- `send_tx`/`broadcast_tx*` помечены `NoRetry`: повторная отправка
  nonce-safe на NEAR, но решение оставлено вызывающему
  (`WithNonIdempotentRetry`).
- Бюджет ретраев по умолчанию 15s на запрос; настраивается
  `WithMaxRetries`, `WithRetryBaseDelay`.
