# Usage

## Installation

Install a published binary through FinFocus:

```sh
finfocus plugin install github.com/rshade/finfocus-plugin-vantage@v0.1.0
```

Or build from source with `make build`. The executable is a gRPC server
launched by FinFocus. `--version` prints the build version. Running it without
arguments announces `PORT=<port>`; SIGTERM stops it gracefully.

## Configuration

Set `FINFOCUS_VANTAGE_TOKEN` to a read-only Vantage service token and
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` to the report selector. The latter is
required even when credentials arrive per request. Configure secrets in your
shell or secret manager rather than committing them.

A request can supply `x-finfocus-credential-vantage-token` in gRPC metadata.
It overrides the environment token for that request. The SDK checks credential
sizes before invoking the plugin. Credentials and upstream error bodies are
excluded from logs and public error messages.

`FINFOCUS_VANTAGE_BASE_URL` defaults to `https://api.vantage.sh/v2`. HTTPS is
required except for loopback HTTP URLs used by tests.

## Resource requests

`GetActualCost` accepts `resource_id`, `start`, `end`, and the current spec's
`resource` descriptor. Supply provider and type in the descriptor; for legacy
clients, use the `provider` and `resource_type` tags. `Supports` recognizes
`aws`, `azure`, `gcp`, `kubernetes`, and `custom`; successful retrieval still
depends on the selected report containing that resource.

The service selector is required. The plugin infers EC2, S3, Lambda, RDS, and
DynamoDB from common AWS resource types or ARNs. For other types/providers,
supply `tags.service` with the exact Vantage service name. It takes precedence
over inferred service names. Cloud tags become escaped VQL tag filters;
routing metadata is excluded. Descriptor region and ARN are used when present.
See [Mapping](MAPPING.md) for exact rules.

For a Pulumi deployment:

```sh
finfocus cost actual --pulumi-state state.json \
  --from 2026-09-01 --to 2026-09-30 --adapter vantage --output json
```

Use exported state containing real cloud resource IDs. A preview may lack the
cloud ID required to retrieve historical billing. Vantage's imported billing
can arrive days after usage; an empty result does not establish zero spend.

## Results and limitations

The plugin returns daily rows in the half-open interval `[start, end)`.
Vantage date filters use UTC calendar dates; partial-day intervals include only
matching daily timestamps. FinFocus sums rows into resource totals. Default
queries follow every page because current core clients consume one response.
Explicit `page_size`/`page_token` requests receive a page and its continuation.

Mixed billing currencies produce `FailedPrecondition`. USD results can use
legacy cost fields. Non-USD results need valid FOCUS source fields to convey
currency reliably. No currency conversion is performed. Invoice/commitment
fields remain empty when unavailable. Projected cost, pricing spec, and
estimate RPCs return `Unimplemented`.

Requests are paced to one per second per plugin instance. Rate-limit and
transient server failures receive bounded retries, respecting retry headers.
Cancellation stops pacing and retry waits. See [API setup](VANTAGE_API.md).

## Validation

Offline tests and coverage gates:

```sh
make test-coverage
make lint
make govulncheck
```

To exercise your installed FinFocus core against the mock API:

```sh
FINFOCUS_CORE_BINARY=/absolute/path/to/finfocus \
  go test -race ./test/integration -run TestPluginBinary -v
```

Live account testing is opt-in. Configure the token/report environment above,
plus `FINFOCUS_VANTAGE_TEST_RESOURCE`, `FINFOCUS_VANTAGE_TEST_PROVIDER`,
`FINFOCUS_VANTAGE_TEST_SERVICE`, `FINFOCUS_VANTAGE_TEST_START`, and
`FINFOCUS_VANTAGE_TEST_END`. Dates use `YYYY-MM-DD`; select an imported interval
with known positive spend for the resource. Then run:

```sh
FINFOCUS_VANTAGE_LIVE_TEST=1 \
  go test ./test/integration -run TestLiveVantageActualCost -v
```

This live test contacts Vantage. It is skipped by default and was not run for
the initial offline acceptance checks.
