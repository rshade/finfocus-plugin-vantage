# Forecast Snapshots

This document explains how the FinFocus Vantage plugin handles forecast data
and snapshots.

## Overview

The plugin supports optional forecast functionality that retrieves cost
projections from Vantage's `/cost_reports/{token}/forecast` endpoint. Forecast
data is stored as a separate metric type to distinguish it from actual historical
cost data.

**Key features:**

- Retrieves projected cost data from Vantage API
- Stores forecast as separate records with `metric_type="forecast"`
- Supports weekly snapshots with configurable retention
- Enables MAPE (Mean Absolute Percentage Error) accuracy evaluation
- Fully integrated with regular sync operations

## Forecast Data Flow

### Step 1: Forecast Generation

When enabled, the adapter calls Vantage's forecast endpoint:

```http
GET /cost_reports/{report_token}/forecast
Authorization: Bearer {api_token}
```

**Request parameters**:

- Cost Report or Workspace token
- Date range (typically future periods)
- Grouping dimensions (provider, service, account, etc.)
- Metrics to include (cost, usage, etc.)

### Step 2: Data Mapping

Forecast response is mapped to FOCUS 1.2 schema with `metric_type="forecast"`:

```json
{
  "timestamp": "2025-01-01T00:00:00Z",
  "metric_type": "forecast",
  "cloud_provider": "AWS",
  "service_name": "EC2",
  "billing_account_id": "123456789",
  "net_cost": 1500.00,
  "usage_amount": 720.0,
  "source_id": "cr_abc123",
  "line_item_id": "forecast_2024-10-01_aws_ec2"
}
```

### Step 3: Snapshot Creation & Storage

- Forecast records are persisted via the Sink interface (same as cost data)
- Multiple snapshots retained for historical comparison
- Each snapshot includes generation timestamp
- Records deduplicated by idempotency key

### Step 4: Retention & Cleanup

- **Default retention**: Last 8 snapshots
- **Default frequency**: Weekly snapshots
- Older snapshots automatically removed

## Snapshot Schedule

### Frequency

- **Default**: Weekly snapshots
- **Configurable**: Can be adjusted based on requirements
- **Trigger**: Manual via CLI or automated scheduling

### Retention

- **Default**: Keep last 8 snapshots
- **Purpose**: Enable MAPE (Mean Absolute Percentage Error) evaluation
- **Cleanup**: Automatic removal of older snapshots

## Usage & Examples

### Example 1: Enable Forecast in Regular Sync

Include forecast snapshots with daily cost syncs:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_abc123"
  granularity: "day"
  include_forecast: true  # Enable forecast
  page_size: 5000
  max_retries: 5
```

Run regular pull operation:

```bash
finfocus-vantage pull --config config.yaml
# Syncs both historical costs and forecast snapshots
```

### Example 2: Generate Forecast Snapshot Separately

Generate forecast data as standalone operation:

```bash
finfocus-vantage forecast --config config.yaml --out ./data/forecast.json
```

Output file format:

```json
[
  {
    "timestamp": "2025-02-01T00:00:00Z",
    "metric_type": "forecast",
    "cloud_provider": "AWS",
    "service_name": "EC2",
    "billing_account_id": "123456789",
    "net_cost": 2500.50,
    "usage_amount": 750.0
  },
  {
    "timestamp": "2025-03-01T00:00:00Z",
    "metric_type": "forecast",
    "cloud_provider": "AWS",
    "service_name": "RDS",
    "billing_account_id": "123456789",
    "net_cost": 1200.75,
    "usage_amount": 240.0
  }
]
```

### Example 3: Query Forecast Records

Query forecast data alongside actual costs:

```sql
-- Get all forecast records
SELECT * FROM costs WHERE metric_type = 'forecast'

-- Compare actual vs forecast by month
SELECT
  DATE_TRUNC('month', timestamp) as month,
  SUM(CASE WHEN metric_type = 'forecast' THEN net_cost ELSE 0 END) as
    forecast_cost,
  SUM(CASE WHEN metric_type != 'forecast' THEN net_cost ELSE 0 END) as
    actual_cost
FROM costs
WHERE cloud_provider = 'AWS'
GROUP BY DATE_TRUNC('month', timestamp)
ORDER BY month DESC

-- Get latest forecast snapshot
SELECT * FROM costs
WHERE metric_type = 'forecast'
AND line_item_id LIKE 'forecast_latest_%'
```

### Example 4: Configuration with Snapshot Settings

Configure forecast frequency and retention:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_abc123"
  include_forecast: true
  # Optional: configure snapshot behavior
  # forecast_frequency: "weekly"      # weekly (default), daily, monthly
  # forecast_retention_count: 8       # Keep 8 snapshots (default)
  granularity: "day"
  group_bys:
    - provider
    - service
    - account
  metrics:
    - cost
    - usage
```

## Output Locations

### File Output

When using the `forecast` command with `--out`:

```text
./data/forecast.json  # JSON format
./data/forecast.parquet  # Parquet format (if supported)
```

### Database Storage

Forecast records are stored in the same data store as regular cost data but
tagged with `metric_type = "forecast"`.

### Sink Integration

Forecast data is written through the same Sink interface as cost data,
allowing flexible storage options (database, parquet files, etc.).

## Analysis and Reporting

### MAPE Evaluation

Compare forecast accuracy using the retained snapshots:

- **Current forecast** vs **actual costs** when they become available
- **Historical forecasts** vs **realized costs**
- **Trend analysis** for forecast quality improvement

### Reporting

Forecast data enables:

- **Budget planning** with projected costs
- **Cost trend analysis** including future projections
- **Anomaly detection** comparing forecast vs actual
- **Scenario planning** for cost optimization decisions

## Limitations

### Data Availability

- Forecast requires a Cost Report token (not available with Workspace tokens)
- Forecast data may not be available for all Vantage configurations
- Forecast accuracy depends on historical data quality and patterns

### Time Horizons

- Typical forecast horizon: 3-12 months
- Granularity: Usually monthly for longer forecasts
- Confidence intervals: May not always be provided

### Performance Considerations

- Forecast API calls may be slower than regular cost queries
- Large forecast datasets may require pagination
- Additional storage space for multiple snapshots

## Troubleshooting

### Common Issues

#### "Forecast not available"

- Verify Cost Report token is configured
- Check if forecast is enabled in Vantage workspace
- Confirm sufficient historical data exists

#### "Empty forecast results"

- Check date ranges (forecast covers future periods)
- Verify report configuration includes forecast-enabled services
- Review Vantage documentation for forecast requirements

#### "Forecast accuracy issues"

- Ensure sufficient historical data (minimum 3-6 months recommended)
- Check for significant cost pattern changes
- Consider adjusting forecast parameters in Vantage

### Debug Logging

Enable verbose logging to inspect forecast API calls:

```bash
export FINFOCUS_VANTAGE_VERBOSE=true
./bin/finfocus-plugin-vantage forecast --config config.yaml
```

Look for log entries with `operation=forecast` for troubleshooting.

## Best Practices

### Configuration Best Practices

1. **Enable selectively**: Only enable forecast when needed to reduce
   API load
2. **Schedule appropriately**: Weekly snapshots typically sufficient for
   most use cases
3. **Monitor accuracy**: Regularly review forecast vs actual cost comparisons

### Operations

1. **Separate storage**: Consider isolating forecast data for analysis
2. **Version control**: Track forecast snapshot versions for audit trails
3. **Alerting**: Set up alerts for forecast generation failures

### Analysis

1. **Compare regularly**: Use MAPE calculations to assess forecast quality
2. **Adjust parameters**: Refine Vantage forecast settings based on accuracy metrics
3. **Document assumptions**: Record any manual adjustments or overrides
