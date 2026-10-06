# FinFocus Vantage Plugin

A FinFocus gRPC cost source that retrieves actual cloud costs from Vantage Cost
Reports. It supports AWS, Azure, GCP, Kubernetes, and custom provider data that
Vantage has imported. Projected costs and pricing estimates are not provided.

## Build and run

```sh
make build
./bin/finfocus-plugin-vantage --version
```

When started by FinFocus, the plugin announces its dynamically assigned port
and serves the cost source gRPC API. Configure `FINFOCUS_VANTAGE_TOKEN` with a
read-only Vantage service token and `FINFOCUS_VANTAGE_COST_REPORT_TOKEN` with
the Cost Report token used to select billing data. A per-request token can be
sent as `x-finfocus-credential-vantage-token`; it takes precedence over the
environment variable.

`GetActualCost` requires a start and end timestamp, a resource ID, and a
provider. In finfocus-spec v0.7.0, the request has no resource descriptor, so
provider and optional Vantage service filters are supplied in tags as
`provider` and `service`. See [Usage](docs/USAGE.md) and
[Vantage API configuration](docs/VANTAGE_API.md).

The adapter returns imported actual cost rows. Vantage data arrival time
depends on the upstream cloud billing import. The RPC follows Vantage page
links through `next_page_token`; currency mixing within a page is an error.
Validated FOCUS records are included when Vantage supplies the required
account, service, currency, and usage values. See [mapping details](docs/MAPPING.md) and
[FOCUS 1.4 field availability](docs/FOCUS_1_4_MAPPING.md).

## Development

```sh
go build ./cmd/finfocus-plugin-vantage
go test ./...
go vet ./...
```

The gRPC implementation is in `internal/plugin` and uses the official Vantage
client through the `internal/vantageapi` wrapper. Its mock API integration test
uses an in-process HTTP server, so no external mock service is required.
