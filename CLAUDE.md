# Repository guidance

This repository builds a FinFocus gRPC cost source for Vantage. The plugin
implements `GetActualCost` in `internal/plugin`; API calls go through the
official Vantage Go client wrapper in `internal/vantageapi`.

## Commands

```sh
make build
make test
make lint
make fmt
make vet
make tidy
```

The mock API integration test uses an in-process HTTP server. Docker and
WireMock are not required.

## Architecture

- `cmd/finfocus-plugin-vantage/`: gRPC server entry point
- `internal/plugin/`: FinFocus RPC implementation and response mapping
- `internal/vantageapi/`: interface and adapter for the official Vantage client
- `docs/`: user setup, API, mapping, and FOCUS field documentation

The former CLI sync workflow, `Sink` adapter, and hand-written HTTP client were
removed. Do not reintroduce them; add functionality through the plugin RPC and
the `vantageapi.Client` interface.

## Implementation conventions

- Go version: 1.27.1 (see `go.mod`)
- Keep money as decimal strings or exact decimals; do not convert to `float64`.
- Check context cancellation and wrap errors with useful operation context.
- Use structured logs; never log Vantage credentials.
- Keep the wrapper boundary around generated official-client types.
