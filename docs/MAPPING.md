# GetActualCost mapping

The v0.1.0 RPC adapter queries a configured Vantage Cost Report. Set
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` to the report selector and provide the
Vantage bearer token as `FINFOCUS_VANTAGE_TOKEN` or the per-request credential
`x-finfocus-credential-vantage-token`.

| FinFocus input | Vantage query |
| --- | --- |
| `start`, `end` | `start_date`, `end_date`, UTC calendar dates |
| `resource_id` | VQL `resource_id` equality filter |
| `tags["provider"]` | Required VQL `provider` equality filter |
| `tags["service"]` | Optional VQL `service` equality filter |
| remaining tags | VQL `labels.<key>` equality filters |
| requested page size | API `limit` (bounded to 1,000) |

The v0.7.0 `GetActualCostRequest` does not contain a `ResourceDescriptor`.
Accordingly provider and service are read from tags, with AWS provider
recognition as a fallback for AWS ARNs. A resource type cannot safely be
translated into a Vantage service name without a caller-supplied service tag.

Each Vantage row becomes one `ActualCostResult`: the decimal amount is parsed
to float64 as required by the RPC, the accrued date becomes `timestamp`, and
`source` is `vantage`. Malformed amounts or dates return `DATA_LOSS`. Mixed
currencies return `FAILED_PRECONDITION`, since the RPC does not provide a
per-row currency field. Empty responses return an empty result list. Vantage
errors return `UNAVAILABLE`; invalid request ranges return `INVALID_ARGUMENT`;
missing credentials return `UNAUTHENTICATED`.

HTTP 429 responses are retried up to three times with exponential delays of
200 ms, 400 ms, and 800 ms. Cancellation interrupts the wait. After retries,
rate limiting is reported as `UNAVAILABLE`.

When a row includes billing account ID, service, currency, and positive usage
with a unit, the adapter builds and validates a FOCUS record using the SDK
builder. It omits the FOCUS record when those required source fields are absent.
The API report token is a
selector, not a credential. Bearer authentication is always supplied
separately. Vantage's date-only API bounds mean the requested timestamp
interval is narrowed to UTC calendar dates. Callers should account for
Vantage's normal billing data import delay. The RPC page token contains the
Vantage `page` query value from its next link; malformed next links return
`DATA_LOSS`. The token remains opaque to FinFocus callers.
