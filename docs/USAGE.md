# Usage

## Install and launch

Build the binary with `make build`, or install a published release using the
FinFocus plugin installer. The binary is a gRPC server launched by FinFocus;
it is not a sync CLI. `--version` prints the injected build version.

## Configure credentials

Set `FINFOCUS_VANTAGE_TOKEN` to a read-only Vantage service token and
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` to the Cost Report selector. Credentials
can instead be passed for one request using the metadata key
`x-finfocus-credential-vantage-token`. The per-request value takes precedence.
Neither credential is logged.

## Request mapping

For finfocus-spec v0.7.0, supply `resource_id`, `start`, `end`, and `tags` in
`GetActualCost`. The `provider` tag is required; `service` is optional. Other
tags become Vantage label filters. The adapter queries daily data and returns
one result per Vantage row. Unsupported projected-cost, pricing-spec, and
estimate RPCs return gRPC `UNIMPLEMENTED`.

The `Supports` RPC accepts the provider names `aws`, `azure`, `gcp`,
`kubernetes`, and `custom`. Cost data is limited to what has been imported into
Vantage. See [the mapping](MAPPING.md) for date boundaries, errors, and current
pagination limits.
