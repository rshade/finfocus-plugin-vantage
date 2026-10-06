# GetActualCost mapping

The plugin queries a configured Vantage Cost Report through `/v2/costs`.
`FINFOCUS_VANTAGE_COST_REPORT_TOKEN` selects the report. Authenticate with
`FINFOCUS_VANTAGE_TOKEN` or the per-request metadata credential
`x-finfocus-credential-vantage-token`.

| FinFocus input | Vantage query |
| --- | --- |
| `resource.provider` | `costs.provider` equality filter |
| `resource.resource_type` | Service inference for supported AWS types |
| `tags["service"]` | Explicit Vantage service, overriding inference |
| `arn`, otherwise `resource_id` | `costs.resource_id` equality filter |
| `resource.region` | Optional `costs.region` equality filter |
| cloud tags | `(tags.name, tags.value) IN (...)` filter |
| `start`, `end` | UTC calendar dates in `start_date`, `end_date` |
| `page_size`, `page_token` | API `limit`, `page` |

Vantage requires provider and service for resource filters. The plugin infers
service names for AWS EC2 instances, S3 buckets, Lambda functions, RDS
instances, and DynamoDB tables from Pulumi resource types or AWS ARNs. Other
resources must supply the exact Vantage service name in `tags["service"]`.
Missing service information returns `INVALID_ARGUMENT` before an API call.
See [Vantage's VQL reference](https://docs.vantage.sh/vql_cost_report).

With a resource descriptor, its provider and region are authoritative. A cloud
tag named `provider`, `region`, or `sku` remains a billing tag. For older hosts
without a descriptor, `provider` and `resource_type` tags supply those dimensions;
`sku` and `region` are treated as host metadata rather than cloud tag filters.
Tag names and values retain their original case. Quoted values are escaped,
and tag filters are sorted for reproducible queries.

Queries use daily bins and billed costs: amortization is disabled, and credits,
refunds, discounts, and taxes are included. Saved report defaults do not silently
change this cost basis. The response contains one result per row, with no monthly
extrapolation or currency conversion. RPC amounts use `float64` because that is
the protobuf field type; the API wrapper retains decimal strings.

The query uses Vantage's date-only bounds. Returned buckets are restricted to
`start <= timestamp < end`, so a row on the exclusive end date is excluded.
Use UTC midnight boundaries for complete daily costs; partial days are not
prorated. Empty reports return no results rather than a fabricated zero charge.
Vantage's billing import delay can leave recent periods empty.

Pagination follows the page number in `links.next`. The RPC token is opaque to
callers. `total_count` is unset because the API does not provide a whole-query
row count. Mixed currencies within a page are rejected. Non-USD rows require
sufficient source fields to build a validated FOCUS record, because the legacy
RPC fields have no currency field and FinFocus otherwise defaults to USD.

A validated FOCUS record is included when account, service, currency, and
positive usage with a unit are available. Numeric usage and usage objects with
a value keyed by the declared unit are supported; unknown usage objects are
not interpreted. The caller's `billing_account_id`, when present, overrides
the source billing account ID. Missing invoice and commitment fields remain
unset; see [FOCUS field availability](FOCUS_1_4_MAPPING.md).

| Failure | gRPC status |
| --- | --- |
| Invalid range, pagination token, query, or missing service | `INVALID_ARGUMENT` |
| Missing credentials or HTTP 401 | `UNAUTHENTICATED` |
| HTTP 403 | `PERMISSION_DENIED` |
| HTTP 404 | `NOT_FOUND` |
| Mixed currencies, unrepresentable currency, HTTP 402 | `FAILED_PRECONDITION` |
| Malformed amounts, dates, currency, or page links | `DATA_LOSS` |
| Cancellation or deadline | `CANCELLED` or `DEADLINE_EXCEEDED` |
| Exhausted throttling/transient failures | `UNAVAILABLE` |

HTTP 429, 500, 502, 503, and 504 are retried up to three times. Backoff starts
at 200 ms and doubles; `Retry-After` and the Unix-time `x-rate-limit-reset`
header can extend the delay up to one minute. Cancellation interrupts waits.
The plugin spaces API requests at least one second apart across requests.
API response bodies and credentials are not included in gRPC error messages.

When pagination fields are absent, all Vantage pages are retrieved before
returning, for compatibility with FinFocus hosts that consume one response.
Repeated or backward page links fail with `DATA_LOSS`. Hosts that set a page
size receive one page and must follow `next_page_token`. Legacy routing tags
that duplicate descriptor dimensions are omitted from billing filters, because
FinFocus can inject these tags during actual-cost request construction.
