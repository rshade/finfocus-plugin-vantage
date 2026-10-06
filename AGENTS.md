# AGENTS.md - FinFocus Vantage Plugin

## Build/Lint/Test Commands

- `make build` - Build binary
- `make test` - Run all tests with race detection
- `make test-coverage` - Run tests with coverage report
- `make lint` - Run golangci-lint with strict checks
- `make fmt` - Format with gofmt/goimports
- `make clean` - Remove built artifacts
- `go test -run TestName ./... -v` - Run single test
- `golangci-lint run` - Direct lint command
- `npm run commitlint -- COMMIT_MESSAGE.md` - Validate commit message format
- `npm run markdownlint` - Lint markdown files

## Code Style Guidelines

- **Language**: Go 1.27.1+
- **Imports**: Standard library → third-party → internal packages
- **Naming**: camelCase variables/functions, PascalCase exported types
- **Error Handling**: Return errors, fmt.Errorf wrapping, context cancellation checks
- **Types**: Structs with json/yaml tags, explicit interface implementation
- **Logging**: Structured with adapter=vantage, operation, attempt fields
- **Security**: Never log tokens, use env vars for secrets

## Project Structure

- `cmd/finfocus-plugin-vantage/` - gRPC plugin entry point
- `internal/plugin/` - FinFocus RPC implementation and Vantage mapping
- `internal/vantageapi/` - wrapper around the official Vantage Go client

## Vantage API Client

The plugin wraps the official Vantage Go client (`github.com/vantage-sh/vantage-go` v0.1.13, MIT) behind the `internal/vantageapi` interface. Production API access should go through this wrapper.

**Client setup:**
- Base URL defaults to `https://api.vantage.sh/v2` (the official API server)
- Bearer token passed via `Authorization: Bearer <token>` header
- Read-scope Service token recommended (no Write permissions needed)
- All endpoints wrapped by `internal/vantageapi` interface (not direct generated client usage)

**Key endpoint:** `GET /costs` queries cost data. Amounts are decimal strings; do not use floating point for money.

**Using the wrapper:**

```go
client, err := vantageapi.NewClient("https://api.vantage.sh/v2", "your-token")
resp, err := client.GetCosts(ctx, &GetCostsParams{
    CostReportToken: &token,
    StartDate: &"2026-10-01",
    EndDate: &"2026-10-31",
    DateBin: &"day",
    Groupings: []string{"provider", "service", "region"},
    Limit: &5000,
})
```

## Testing

Run `make test` and `make lint`. Plugin integration tests use an in-process mock Vantage API.
