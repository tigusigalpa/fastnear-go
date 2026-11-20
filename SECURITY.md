# Security Policy

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories
on https://github.com/tigusigalpa/fastnear-go or by email to
sovletig@gmail.com. Do not open public issues for vulnerabilities.

## API keys

- The SDK never writes the API key to logs, errors or `Error()` output.
- Prefer Bearer authentication (default). Query-parameter auth
  (`WithAuthMode(fastnear.AuthQuery)`) may leak the key into server
  access logs — use it only where headers are unavailable.
- Do not embed a server-side key into browser or mobile applications;
  use FastNear browser keys instead.

## Transaction signing

This SDK does not hold private keys and is not a wallet. `SendTx` and
`BroadcastTxAsync` accept already-signed transactions.
