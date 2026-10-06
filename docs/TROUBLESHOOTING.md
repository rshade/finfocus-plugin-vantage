# Troubleshooting

## Plugin does not start

Run `./bin/finfocus-plugin-vantage --version` to confirm the binary works.
When launching through FinFocus, check that the plugin's `PORT=<port>` startup
announcement is present on stdout and that stderr contains no startup error.

## Vantage authentication fails

Set `FINFOCUS_VANTAGE_TOKEN` to a read-only Vantage service token. When using
per-request credentials, check that the caller sends the
`x-finfocus-credential-vantage-token` metadata key. A per-request token takes
precedence over the environment variable. Set
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` to the report selector; it is not an API
credential.

## GetActualCost returns no rows

Check that the request contains a time range, resource ID, and provider, and
that those values match the selected Cost Report. Vantage may not have imported
recent cloud billing data yet. Provider and optional service filters are passed
as VQL filters; inspect the plugin logs for the operation and trace ID when
diagnosing a failed request. Credentials are not logged.

## Pagination or API errors

The plugin returns the next Vantage page through `next_page_token`. Continue
requesting pages until the token is empty. API throttling or availability
failures are returned as gRPC `UNAVAILABLE`; retry the RPC with backoff.

## Run the local integration test

The mock API test runs without Docker or external services:

```sh
go test ./internal/plugin -run TestGetActualCostAgainstMockVantageAPI -v
```
