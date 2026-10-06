# Configuration Reference

This document describes the configuration options for the FinFocus Vantage
plugin.

## Configuration File Format

The plugin accepts configuration via YAML files. Environment variable
substitution is supported using `${VAR_NAME}` syntax (e.g.,
`${FINFOCUS_VANTAGE_TOKEN}`).

Configuration follows the FinFocus adapter pattern with three top-level
sections:

- `version`: Configuration schema version (currently `0.1`)
- `source`: Plugin identifier (must be `vantage`)
- `credentials`: Authentication credentials (token)
- `params`: Adapter parameters (query options, timeouts, pagination)

## Minimal Configuration

The absolute minimum configuration requires only:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_abc123def456"
  granularity: "day"
```

## Complete Example Configuration (Design Section 4)

This example shows all available configuration options with descriptions:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}  # API token from Vantage (via env var)
params:
  # Token selection (must provide one; cost_report_token preferred)
  workspace_token: "ws_..."         # optional if using cost_report_token
  cost_report_token: "cr_..."       # preferred for stable queries

  # Date range (ISO format: YYYY-MM-DD)
  start_date: "2024-01-01"          # default: 12 months back from today
  end_date: "2024-12-31"            # default: today; set null or omit current

  # Aggregation and metrics
  granularity: "day"                # "day" or "month"
  group_bys:
    - provider
    - service
    - account
    - project
    - region
    - resource_id
    - tags
  metrics:
    - cost
    - usage
    - effective_unit_price
    - amortized_cost
    - taxes
    - credits

  # Optional forecast snapshots
  include_forecast: true

  # Tag normalization (optional prefix filter)
  tag_prefix_filters: ["user:", "kubernetes.io/"]

  # HTTP client tuning
  request_timeout_seconds: 60
  page_size: 5000
  max_retries: 5
```

## Configuration Parameters

### Credentials Section

#### credentials.token

- **Type**: `string`
- **Required**: Yes
- **Environment Variable**: `FINFOCUS_VANTAGE_TOKEN`
- **Description**: API authentication token issued by Vantage. Can be a service
  account token or user token with appropriate permissions.
- **Example**:

  ```yaml
  credentials:
    token: ${FINFOCUS_VANTAGE_TOKEN}
  ```

- **Security**: Never logged or printed in error messages. Always provided via
  environment variable or secrets management system; never hardcoded in YAML.

---

### Parameters Section

#### params.cost_report_token

- **Type**: `string`
- **Required**: Either `cost_report_token` or `workspace_token` must be provided
- **Environment Variable**: `FINFOCUS_VANTAGE_COST_REPORT_TOKEN`
- **Description**: Cost Report token for querying a curated, pre-filtered cost
  dataset. **Preferred over workspace_token** for stable, consistent results
  and better performance. Cost Report tokens scope access to specific cost
  reports with predefined filters and grouping.
- **Example**:

  ```yaml
  params:
    cost_report_token: "cr_a1b2c3d4e5f6g7h8i9j0"
  ```

#### params.workspace_token

- **Type**: `string`
- **Required**: Either `workspace_token` or `cost_report_token` must be provided
- **Environment Variable**: `FINFOCUS_VANTAGE_WORKSPACE_TOKEN`
- **Description**: Workspace token for accessing raw cost data at the workspace
  level. Used when Cost Report tokens are not available. Provides broader
  access but may require additional filtering or VQL queries.
- **Example**:

  ```yaml
  params:
    workspace_token: "ws_a1b2c3d4e5f6g7h8i9j0"
  ```

#### params.start_date

- **Type**: `string` (ISO 8601 date format: `YYYY-MM-DD`)
- **Required**: No
- **Default**: 12 months before today
- **Environment Variable**: `FINFOCUS_VANTAGE_START_DATE`
- **Description**: Inclusive start date for cost data retrieval. Must be in
  `YYYY-MM-DD` format. Used for historical backfills and initial data import.
- **Example**:

  ```yaml
  params:
    start_date: "2024-01-01"
  ```

- **Notes**:
  - Dates are interpreted as UTC
  - For incremental syncs, use a window that captures late postings (typically
    D-3 to D-1)

#### params.end_date

- **Type**: `string` (ISO 8601 date format: `YYYY-MM-DD`) or `null`
- **Required**: No
- **Default**: Current date
- **Environment Variable**: `FINFOCUS_VANTAGE_END_DATE`
- **Description**: Inclusive end date for cost data retrieval. Omit or set to
  `null` for the current date. Useful for backfills or specific date ranges.
- **Example**:

  ```yaml
  params:
    end_date: "2024-12-31"  # Specific date
    # or omit for today:
    # end_date: null
  ```

#### params.granularity

- **Type**: `string`
- **Required**: Yes
- **Default**: `"day"`
- **Allowed Values**: `"day"`, `"month"`
- **Environment Variable**: `FINFOCUS_VANTAGE_GRANULARITY`
- **Description**: Time-series granularity for cost aggregation. `"day"` provides
  daily buckets; `"month"` aggregates to monthly totals.
- **Example**:

  ```yaml
  params:
    granularity: "day"
  ```

- **Notes**:
  - Daily granularity is recommended for change detection and variance analysis
  - Monthly granularity reduces record count and API pages for large date ranges

#### params.group_bys

- **Type**: `array` of `string`
- **Required**: No
- **Default**: `["provider","service","account","project","region","resource_id","tags"]`
- **Environment Variable**: Not supported (must use YAML)
- **Description**: Cost dimensions to group results by. Controls which attributes
  are included as separate rows. Availability depends on Vantage configuration
  and the selected cost report.
- **Valid Values**:
  - `provider`: Cloud provider (AWS, GCP, Azure, etc.)
  - `service`: Cloud service (EC2, RDS, Storage, etc.)
  - `account`: Billing account or AWS account ID
  - `project`: GCP project or similar organizational unit
  - `region`: Geographic region
  - `resource_id`: Cloud resource identifier
  - `tags`: Custom tags/labels applied to resources
- **Example**:

  ```yaml
  params:
    group_bys:
      - provider
      - service
      - account
      - region
      - tags
  ```

- **Notes**:
  - More dimensions = more granular data but higher API page count
  - Including `tags` can significantly increase record count (high cardinality)
  - Ensure selected dimensions are available in your Cost Report

#### params.metrics

- **Type**: `array` of `string`
- **Required**: No
- **Default**: `["cost","usage","effective_unit_price"]`
- **Environment Variable**: Not supported (must use YAML)
- **Description**: Cost metrics to retrieve. Determines which cost fields are
  populated in responses. Availability varies by provider and metric type.
- **Valid Values**:
  - `cost`: Net cost (after discounts, before taxes)
  - `amortized_cost`: Amortized cost including reserved instance allocations
  - `usage`: Usage quantity in native units
  - `effective_unit_price`: Computed unit price
  - `taxes`: Tax amounts
  - `credits`: Credit amounts (free tier, promotional, etc.)
  - `refunds`: Refund amounts
- **Example**:

  ```yaml
  params:
    metrics:
      - cost
      - amortized_cost
      - usage
      - taxes
      - credits
  ```

- **Notes**:
  - Not all metrics are available for all providers
  - Including more metrics may increase API response size
  - Missing metrics in responses are filled with `null` values

#### params.include_forecast

- **Type**: `boolean`
- **Required**: No
- **Default**: `true`
- **Environment Variable**: Not supported (must use YAML)
- **Description**: Whether to fetch and include forecast snapshots in sync
  operations. Forecasts are stored as separate records with
  `metric_type="forecast"`.
- **Example**:

  ```yaml
  params:
    include_forecast: true
  ```

- **Notes**:
  - Forecasts require a separate API call
  - Snapshots are captured weekly and last 8 weeks are retained
  - Disable if forecast functionality is not needed to reduce API calls

#### params.tag_prefix_filters

- **Type**: `array` of `string`
- **Required**: No
- **Default**: `["user:", "kubernetes.io/"]`
- **Environment Variable**: Not supported (must use YAML)
- **Description**: Tag key prefixes to include during processing. Used to filter
  high-cardinality tags and reduce noise. Only tags starting with these
  prefixes are normalized and included in labels.
- **Example**:

  ```yaml
  params:
    tag_prefix_filters:
      - "user:"
      - "kubernetes.io/"
      - "cost-center:"
  ```

- **Notes**:
  - Empty or omitted list disables tag filtering (all tags included)
  - Filtering happens after normalization
  - Raw tag values are preserved in `labels_raw` for audit purposes

#### params.request_timeout_seconds

- **Type**: `integer`
- **Required**: No
- **Default**: `60`
- **Allowed Range**: ≥ 1
- **Environment Variable**: `FINFOCUS_VANTAGE_TIMEOUT`
- **Description**: HTTP request timeout in seconds. Controls how long to wait
  for API responses before timing out.
- **Example**:

  ```yaml
  params:
    request_timeout_seconds: 120
  ```

- **Recommended Values**:
  - `60` (default): Suitable for most configurations
  - `120`: For large page sizes or slow networks
  - `30`: For fast, reliable networks with small pages

#### params.page_size

- **Type**: `integer`
- **Required**: No
- **Default**: `5000`
- **Allowed Range**: 1–10,000
- **Environment Variable**: `FINFOCUS_VANTAGE_PAGE_SIZE`
- **Description**: Number of records to fetch per API request. Larger pages
  reduce total number of API calls; smaller pages reduce memory usage per
  request.
- **Example**:

  ```yaml
  params:
    page_size: 5000
  ```

- **Recommended Values**:
  - `5000` (default): Balance between throughput and memory
  - `10000`: Maximum, for large date ranges with few dimensions
  - `1000`: Conservative, for memory-constrained environments

#### params.max_retries

- **Type**: `integer`
- **Required**: No
- **Default**: `5`
- **Allowed Range**: ≥ 0
- **Environment Variable**: `FINFOCUS_VANTAGE_MAX_RETRIES`
- **Description**: Maximum number of retry attempts for transient API failures
  (HTTP 429, 5xx errors). Uses exponential backoff with jitter.
- **Example**:

  ```yaml
  params:
    max_retries: 5
  ```

- **Notes**:
  - Retries use exponential backoff to avoid overwhelming rate-limited APIs
  - Rate limit headers (X-RateLimit-Reset) are honored when present
  - Set to `0` to disable retries (fail fast)

## Authentication

### Token Management

The plugin requires a valid Vantage API token for authentication. Tokens are
passed via the `credentials.token` configuration field and can be provided in
multiple ways:

1. **Environment Variable (Recommended)**

   ```yaml
   credentials:
     token: ${FINFOCUS_VANTAGE_TOKEN}
   ```

   Set the environment variable before running the adapter:

   ```bash
   export FINFOCUS_VANTAGE_TOKEN="your_token_here"
   finfocus-vantage pull --config config.yaml
   ```

2. **Secrets Management System**

   For production environments, use a secrets manager:

   ```bash
   export FINFOCUS_VANTAGE_TOKEN=$(aws secretsmanager get-secret-value \
     --secret-id vantage-token --query SecretString --output text)
   ```

3. **Direct File (Development Only)**

   **WARNING**: Only for development. Never commit tokens to version control.

   ```yaml
   credentials:
     token: "vantage_token_value_here"
   ```

### Token Types

**Cost Report Token** (Preferred)

- Scoped to a specific cost report with predefined filters
- Provides stable, consistent results
- Better performance and security
- Recommended for production use
- Format: `cr_*`

**Workspace Token** (Fallback)

- Broader access to all workspace data
- Useful when Cost Report tokens unavailable
- May require additional VQL filtering
- Less preferred due to broader scope
- Format: `ws_*`

### Security Best Practices

✅ **DO:**

- Store tokens in environment variables or secrets managers
- Use Cost Report tokens (narrowest scope principle)
- Rotate tokens periodically
- Use different tokens for different environments (dev/staging/prod)
- Restrict token permissions to read-only cost access
- Monitor API usage for suspicious activity

❌ **DON'T:**

- Hardcode tokens in configuration files
- Commit tokens to version control (use `.gitignore`)
- Log or print token values in debug output
- Share tokens via email or chat
- Use workspace tokens when Cost Report tokens available
- Reuse tokens across multiple environments

---

## Environment Variables Reference

All parameters can be overridden via environment variables. The following table
shows the complete mapping:

| Parameter | Env Variable | Format | Example |
|---|---|---|---|
| credentials.token | `FINFOCUS_VANTAGE_TOKEN` | string | `vantage_3f4g...` |
| workspace_token | `FINFOCUS_VANTAGE_WS_TOKEN` | string | `ws_a1b2c3...` |
| cost_report_token | `FINFOCUS_VANTAGE_CR_TOKEN` | string | `cr_a1b2c3...` |
| start_date | `FINFOCUS_VANTAGE_START_DATE` | YYYY-MM-DD | `2024-01-01` |
| end_date | `FINFOCUS_VANTAGE_END_DATE` | YYYY-MM-DD | `2024-12-31` |
| granularity | `FINFOCUS_VANTAGE_GRANULARITY` | day\|month | `day` |
| request_timeout_seconds | `FINFOCUS_VANTAGE_TIMEOUT` | integer | `60` |
| page_size | `FINFOCUS_VANTAGE_PAGE_SIZE` | integer | `5000` |
| max_retries | `FINFOCUS_VANTAGE_MAX_RETRIES` | integer | `5` |

**Note**: Array parameters (`group_bys`, `metrics`, `tag_prefix_filters`) must
be configured in the YAML file; environment variable overrides are not supported
for arrays.

---

## Common Configuration Patterns

### Pattern 1: Quick Start (Minimal Configuration)

Suitable for initial testing and evaluation:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_your_report_token_here"
  granularity: "day"
```

**Use When**:

- First-time setup and testing
- Using predefined cost reports with sensible defaults
- Default date range (12 months back) is acceptable

---

### Pattern 2: Historical Backfill (12-Month Import)

For importing historical data on first run:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_your_report_token_here"
  start_date: "2024-01-01"
  end_date: null  # Current date
  granularity: "day"
  page_size: 10000    # Larger pages for faster throughput
  max_retries: 5      # Retry transient failures
```

**Use When**:

- Initial data import from Vantage
- Importing multiple months of historical data
- Need to maximize API throughput

**Notes**:

- Larger `page_size` (10,000) reduces API calls
- Consider running during off-peak hours
- Incremental syncs after will be much faster

---

### Pattern 3: Daily Incremental Sync (Cron Job)

For scheduled daily cost updates (e.g., via cron or Kubernetes CronJob):

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_your_report_token_here"
  granularity: "day"
  # Omit start_date/end_date; defaults capture D-3 to D-1
  # (accounting for late postings)
  page_size: 5000
  max_retries: 5
  request_timeout_seconds: 120  # Allow time for retries
```

**Use When**:

- Running as scheduled job (cron, Kubernetes, Lambda)
- Want to capture cost data daily
- Late postings (cost updates) expected up to 3 days after

**Cron Example**:

```bash
# Daily at 2 AM UTC
0 2 * * * /usr/local/bin/finfocus-vantage pull --config \
  /etc/finfocus/config.yaml
```

---

### Pattern 4: High-Granularity Analysis (All Dimensions)

For detailed cost analysis with all grouping dimensions:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_your_report_token_here"
  start_date: "2024-11-01"
  end_date: "2024-11-30"
  granularity: "day"
  group_bys:
    - provider
    - service
    - account
    - project
    - region
    - resource_id
    - tags
  metrics:
    - cost
    - amortized_cost
    - usage
    - taxes
    - credits
  page_size: 5000
  max_retries: 5
```

**Use When**:

- Analyzing cost drivers and anomalies
- Need resource-level detail with custom tags
- Performing cost allocation or showback

**Notes**:

- High dimensionality increases record count
- May result in many API pages
- Tag cardinality can be very high; use tag_prefix_filters if needed

---

### Pattern 5: Conservative (Low Resource Usage)

For memory-constrained environments or production stability:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_your_report_token_here"
  granularity: "month"           # Less data
  group_bys:
    - provider
    - service
    - account
  metrics:
    - cost
  page_size: 1000                # Smaller batches
  request_timeout_seconds: 90
  max_retries: 3                 # Fewer retries to fail fast
```

**Use When**:

- Running on memory-limited systems
- Network is unreliable (prefer smaller batches)
- Want predictable, stable performance over raw throughput

---

### Pattern 6: With Tag Filtering (Kubernetes Multi-Tenant)

For environments with many tags but only some are relevant:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_your_report_token_here"
  granularity: "day"
  group_bys:
    - provider
    - service
    - account
    - region
    - tags
  tag_prefix_filters:
    - "k8s:"                # Kubernetes tags
    - "team:"               # Team assignment
    - "cost-center:"        # Finance tags
    - "environment:"        # env tags
  metrics:
    - cost
    - usage
```

**Use When**:

- Multi-tenant Kubernetes environments
- Need to filter for specific tag domains
- Want to reduce noise from pod UIDs and other ephemeral identifiers

---

### Pattern 7: Multi-Cloud Setup (Multiple Cost Reports)

If you have separate cost reports per cloud provider:

#### config-aws.yaml

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_aws_production"
  granularity: "day"
```

#### config-gcp.yaml

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_gcp_production"
  granularity: "day"
```

**Script to sync both**:

```bash
#!/bin/bash
for config in config-aws.yaml config-gcp.yaml; do
  echo "Syncing $config..."
  finfocus-vantage pull --config "$config" || exit 1
done
echo "All syncs completed"
```

---

## Configuration Validation

The adapter validates configuration at startup. Common validation errors:

| Error | Cause | Solution |
|---|---|---|
| `token is required` | Missing token | Set env var or YAML token |
| `workspace/report token required` | Both missing | Provide at least one token |
| `granularity must be day or month` | Invalid value | Use `"day"` or `"month"` |
| `invalid start_date format` | Not ISO format | Use `YYYY-MM-DD` |
| `page_size cannot exceed 10000` | Too large | Use ≤ 10,000 |
| `timeout must be >= 1 second` | Invalid value | Use positive integer |

---

## Data Mapping

Configuration parameters control which data is retrieved and how it's
structured:

- **Date Range** (`start_date`, `end_date`): Controls historical vs current
  data
- **Granularity** (`granularity`): Time bucket size in output
- **Dimensions** (`group_bys`): Which cost attributes create separate rows
- **Metrics** (`metrics`): Which cost types are included
- **Tags** (`tag_prefix_filters`): Which labels are included
- **Resilience** (`max_retries`, `request_timeout_seconds`): How to handle
  failures

All parameters are passed to Vantage API and control the returned cost records.

---

## Notes

- Configuration files support YAML syntax; JSON is not supported
- All ISO dates must be in UTC; no timezone conversion is performed
- Environment variable overrides apply to scalar fields only (arrays stay in
  YAML)
- Token values are redacted from logs and error messages for security
- The adapter prefers Cost Report tokens over Workspace tokens for better
  performance and security
- Missing or null metrics in Vantage responses are preserved as `null` in
  output
- Tag normalization converts keys to lowercase kebab-case (e.g., `CostCenter`
  → `cost-center`)
- Incremental sync uses bookmarks to track the last successful sync date per
  report
