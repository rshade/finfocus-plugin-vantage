# FinFocus Vantage Plugin

A FinFocus gRPC cost source that retrieves imported actual cloud costs from
Vantage Cost Reports. Projected costs and pricing estimates are unavailable.

## Install

```sh
finfocus plugin install github.com/rshade/finfocus-plugin-vantage@v0.1.0
```

Release archives contain `finfocus-plugin-vantage` (`.exe` on Windows) for
Linux, macOS, and Windows on amd64 and arm64, with SHA-256 checksums.

Configure `FINFOCUS_VANTAGE_TOKEN` with a read-only Vantage service token and
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` with your report selector. FinFocus can
also pass `x-finfocus-credential-vantage-token` in gRPC metadata; that token
takes precedence for the request. Neither token is logged.

```sh
finfocus cost actual --pulumi-state state.json \
  --from 2026-09-01 --to 2026-09-30 --adapter vantage --output json
```

See [Usage](docs/USAGE.md) for resource selection, date boundaries, and
[Vantage API setup](docs/VANTAGE_API.md) for configuration.

## Cost retrieval

Requests need a resource ID, provider, service, and start/end timestamps.
Current FinFocus resource descriptors supply provider, type, and region;
legacy clients can supply routing tags. The plugin infers common AWS services
from resource types or ARNs. Other services require a `service` tag containing
Vantage's exact service name.

Daily rows use the interval `start <= timestamp < end`. A default request
fetches all Vantage pages; callers that explicitly request pagination receive
`next_page_token`. Mixed currencies are rejected. Non-USD rows require a
validated FOCUS record so FinFocus can identify their billing currency.

Validated FOCUS records are included when source fields permit construction.
Invoice detail and commitment metadata are unavailable from the costs
endpoint. See [request/response mapping](docs/MAPPING.md) and
[FOCUS field availability](docs/FOCUS_1_4_MAPPING.md).

## Development

```sh
make build
./bin/finfocus-plugin-vantage --version
make test-coverage
make lint
make govulncheck
npm run markdownlint
```

The binary announces `PORT=<port>` and serves gRPC when launched by FinFocus.
Production API access uses the official Vantage client through
`internal/vantageapi`. Tests exercise the SDK server and an in-process mock
HTTP API. Optional tests cover FinFocus core and live Vantage accounts; see
[Usage](docs/USAGE.md#validation).
