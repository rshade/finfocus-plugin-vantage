# Vantage API setup

Create a Vantage service token with read access to the Cost Report API. Set the
bearer token as `FINFOCUS_VANTAGE_TOKEN`; set
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` to the token identifying the report whose
costs should be queried. The report token selects a report and is not an API
credential.

The plugin uses Vantage's v2 API through `internal/vantageapi` and sends the
service token with bearer authentication. For per-request isolation, FinFocus
may supply the bearer token as gRPC metadata named
`x-finfocus-credential-vantage-token`. This credential overrides the
environment token for that request.

Requests use daily bins, UTC calendar dates, and provider, service, account,
region, and resource ID groupings. Provider, resource ID, optional service,
and supplied labels are expressed as VQL filters. Vantage API throttling or
availability errors are returned as gRPC `UNAVAILABLE`. The response currently
returns Vantage's next page through the FinFocus `next_page_token`. The RPC
token is opaque to callers, even though the plugin internally maps it to the
Vantage page number.

Use the Vantage API documentation for service token and Cost Report setup:
[Vantage REST API](https://docs.vantage.sh/api).
