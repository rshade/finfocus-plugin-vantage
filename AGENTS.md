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

- **Language**: Go 1.24.7+
- **Imports**: Standard library → third-party → internal packages
- **Naming**: camelCase variables/functions, PascalCase exported types
- **Error Handling**: Return errors, fmt.Errorf wrapping, context cancellation checks
- **Types**: Structs with json/yaml tags, explicit interface implementation
- **Logging**: Structured with adapter=vantage, operation, attempt fields
- **Security**: Never log tokens, use env vars for secrets

## Project Structure

- `cmd/finfocus-plugin-vantage/` - CLI with Cobra
- `internal/vantage/client/` - REST client with retry/backoff
- `internal/vantage/adapter/` - FOCUS 1.2 mapping/sync logic
- `test/wiremock/` - Mock server configs

## Vantage API Client

The plugin wraps the official Vantage Go client (`github.com/vantage-sh/vantage-go` v0.1.13, MIT) behind `internal/vantageapi` interface. Legacy hand-written client (`internal/vantage/client`) remains for now; migration tracked by task VT-0.2.

**Client setup:**
- Base URL defaults to `https://api.vantage.sh/v2` (the official API server)
- Bearer token passed via `Authorization: Bearer <token>` header
- Read-scope Service token recommended (no Write permissions needed)
- All endpoints wrapped by `internal/vantageapi` interface (not direct generated client usage)

**Key endpoints:**
- `GET /costs`: Query costs with date range, groupings, and settings
- `GET /cost_reports/{token}/forecasted_costs`: Forecast snapshots per provider/service
- Response amounts are decimal strings (never float for money)

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

## Testing Requirements

- ≥80% client coverage, ≥70% overall
- Contract tests with Wiremock, golden file validation
- `make wiremock-up/down` for mock server
