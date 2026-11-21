# Contributing

## Setup

- Go 1.23+
- `golangci-lint` (optional but recommended)

## Commands

```sh
go build ./...
go test -race ./...
go vet ./...
golangci-lint run
```

## Tests

- Unit tests use `internal/testutil` HTTP servers over `net.Pipe`. They exercise
  HTTP encoding, redirects and cancellation without opening network sockets.
- Keep fixtures alongside the tests, use synthetic credentials, and add a
  regression test when fixing a behavior bug.
- Build the runnable examples separately: `cd examples && go build ./...`.
- Live integration tests require the `integration` build tag and the explicit
  `FASTNEAR_INTEGRATION=1` switch. An API key is optional for anonymous access:
  `FASTNEAR_API_KEY=... FASTNEAR_INTEGRATION=1 go test -tags=integration ./...`.

## Rules

- Zero external dependencies — standard library only.
- No floats for monetary values.
- Never log the API key or the `Authorization` header.
