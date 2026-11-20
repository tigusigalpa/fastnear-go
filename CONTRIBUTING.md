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

- Unit tests never touch the network (`httptest` only).
- Fixtures live in `<family>/testdata/`; regenerate goldens with `go test -update`.
- Integration tests require `FASTNEAR_API_KEY` and the `integration` build tag:
  `FASTNEAR_API_KEY=... go test -tags=integration ./...`

## Rules

- Zero external dependencies — standard library only.
- No floats for monetary values.
- Never log the API key or the `Authorization` header.
