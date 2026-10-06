# Changelog

All notable changes to the FinFocus Vantage plugin are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Replaced the CLI entry point with the FinFocus pluginsdk gRPC server.
- Added Vantage-backed `GetActualCost`, provider support discovery, per-request
  credentials, FOCUS record construction when source data is sufficient, and
  pagination and rate limit retry handling.
- Added request mapping, usage, Vantage API, and FOCUS 1.4 field documentation.
- Added unit and mocked Vantage API integration coverage for the RPC adapter.
- Added trace_id and operation fields to structured RPC request logs.

---

## [0.1.0] - 2024-10-23

### New Features

#### Core Functionality

- **Vantage API Integration**: Initial implementation of REST client for
  Vantage cost API with support for both Cost Report and Workspace tokens
- **Cost Data Ingestion**: Complete support for fetching cost data from
  Vantage with daily and monthly granularity options
- **FOCUS 1.2 Compliance**: Full mapping of Vantage cost data to FinOps
  FOCUS 1.2 schema for standardized cost analytics
- **Multi-Dimension Grouping**: Support for grouping costs by provider,
  service, account, project, region, resource_id, and tags
- **Metric Support**: Retrieve multiple cost metrics including net cost,
  amortized cost, usage, effective unit price, taxes, and credits

#### Resilience & Performance

- **Exponential Backoff Retries**: Automatic retry logic with exponential
  backoff and jitter for transient failures (429, 5xx errors)
- **Rate Limit Handling**: Respects Vantage API rate limits with
  `X-RateLimit-*` header support
- **Pagination Support**: Server-side cursor pagination with configurable
  page sizes (1-10,000 records per page)
- **Configurable Timeouts**: Adjustable HTTP request timeouts to handle
  slow networks

#### Sync Strategies

- **Incremental Sync**: Daily synchronization with configurable lag window
  (D-3 to D-1) to account for late cost postings
- **Historical Backfill**: Support for importing historical cost data (12+
  months)
- **Idempotency Keys**: Deterministic key generation ensures duplicate
  prevention across sync runs
- **Bookmark Tracking**: Stores last successful sync points per cost report
  for safe recovery on failures

#### Forecast Support

- **Forecast Snapshots**: Optional weekly forecast snapshots for cost
  projections
- **MAPE Evaluation**: Retention of last 8 snapshots enables accuracy
  assessment (Mean Absolute Percentage Error)
- **Separate Metric Family**: Forecasts stored with `metric_type="forecast"`
  for easy filtering
- **Full Integration**: Forecasts included in regular sync operations or
  generated separately

#### Configuration

- **YAML Configuration**: Flexible YAML-based configuration with environment
  variable substitution
- **Comprehensive Validation**: Startup validation of all configuration
  parameters with clear error messages
- **Environment Overrides**: Support for environment variable overrides of
  configuration values
- **Sensible Defaults**: Reasonable defaults for timeouts, page sizes, and
  retry counts

#### CLI

- **`pull` Command**: Daily incremental sync of cost data
- **`backfill` Command**: Historical data import for initial setup
- **`forecast` Command**: Generate standalone forecast snapshots
- **Version Flag**: Display plugin version and Go version information

### Documentation

- **CONFIG.md**: Comprehensive configuration reference with 7 real-world
  configuration patterns
- **TROUBLESHOOTING.md**: 12 common issues with detailed solutions including
  authentication, rate limits, pagination, and data mapping
- **FORECAST.md**: Complete forecast feature documentation with usage
  examples and SQL query patterns
- **README.md**: Quick start guide with feature overview
- **Design Document**: Technical architecture and design decisions

### Testing Infrastructure

- **Wiremock Integration**: Mock HTTP server for testing without live API
  calls
- **Golden Fixture Tests**: Test fixtures and expected outputs for validation
- **Contract Tests**: API contract tests against mock server
- **Unit Tests**: Comprehensive unit tests for client and adapter logic

### Security

- **Token Redaction**: API tokens never logged or exposed in error messages
- **Environment Variable Support**: Tokens passed via environment variables,
  never hardcoded
- **Secure Defaults**: Uses Cost Report tokens (narrowest scope) by default
- **No Plaintext Secrets**: Configuration validates token presence without
  storing in plain files

### Code Quality

- **Golangci-lint**: Full linting compliance with revive, govet, gocyclo, gofmt
- **Code Comments**: Public API and critical functions documented with
  comments
- **Error Handling**: Explicit error handling with wrapped errors for
  debugging
- **Logging Interface**: Pluggable logging for flexibility in structured
  logging

### Project Structure

- **Module Organization**: Clean separation between CLI, client, and adapter
  packages
- **Interface-Based Design**: Pluggable client and sink interfaces for
  extensibility
- **Standard Go Conventions**: Follows Go idioms and project structure
  best practices

### CI/CD Readiness

- **Build Targets**: `make build`, `make test`, `make lint` for development
  workflow
- **Mock Infrastructure**: Docker-based Wiremock setup for testing
- **Version Management**: Semantic versioning support via git tags

---

### Known Limitations

#### Feature Scope

1. **No VQL Filter Support**: v0.1.0 relies on curated Cost Reports instead
   of raw VQL filters. VQL filtering is planned for v0.2.0+

2. **No CSV Export Fallback**: Very large backfills (>12 months) cannot fall
   back to CSV export. Chunking into monthly imports recommended

3. **Forecast Requirements**: Forecast requires Cost Report token and cannot
   use Workspace tokens. Forecast data requires 3-6 months historical data

4. **No Real-Time Streaming**: Adapter uses polling, not real-time event
   streaming for new data

#### API Constraints

1. **Field Availability**: Not all FOCUS fields available for all providers.
   Vantage availability varies by cloud provider and customer configuration

2. **Metrics Availability**: Some metrics (e.g., amortized_cost) may not be
   available for all providers. GCP amortized costs limited to certain
   scopes

3. **Tag Cardinality**: High-cardinality tags (e.g., pod UIDs) can increase
   output dramatically. Filtering recommended via `tag_prefix_filters`

4. **Late Posting Lag**: Cost data takes 2-3 days to fully post. Current day
   is incomplete

#### Performance Constraints

1. **Memory Usage**: Large page sizes and many dimensions increase memory
   usage. Conservative defaults recommended for resource-limited systems

2. **Pagination Limits**: Page size capped at 10,000 records. Very large
   datasets require multiple calls

3. **Rate Limiting**: Subject to Vantage API rate limits. Heavy use may
   trigger backoff

#### Operational Constraints

1. **Single Instance**: Concurrent adapter instances not supported. Use cron
   or scheduled jobs for serialization

2. **No Automatic Chunking**: Large date ranges must be manually chunked into
   months for efficiency

3. **Bookmark Dependency**: Incremental sync relies on sink correctly
   persisting bookmarks

---

### Upgrading

There are no previous versions. Installation instructions for v0.1.0:

```bash
# Clone repository
git clone https://github.com/FinFocus/finfocus-plugin-vantage.git
cd finfocus-plugin-vantage

# Build from source
go install ./cmd/finfocus-vantage

# Or download binary from releases
```

Configuration for v0.1.0:

```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  cost_report_token: "cr_..."
  granularity: "day"
```

---

### Contributors

#### Initial Release (v0.1.0)

- Richard Shade (@rshade) - Design and implementation
- FinFocus Team - Architecture guidance and FOCUS 1.2 reference
- Vantage API - Cost data provider

### Acknowledgments

- FinOps Foundation for FOCUS 1.2 specification
- Vantage for API support and documentation
- Go community for standard libraries and tooling

---

### Known Issues

None documented for v0.1.0. Please report issues via GitHub issues.

---

### Future Roadmap

#### v0.2.0 (Q4 2024 - Q1 2025)

- VQL filter support for custom cost report queries
- CSV export fallback for large backfills (>12 months)
- Performance optimizations for high-dimension datasets
- Additional provider-specific field mappings

#### v0.3.0 (Q1 2025 - Q2 2025)

- SaaS connector enrichment (Fastly, Databricks, GitHub, ClickHouse Cloud)
- Real-time data streaming support
- Custom metric definitions
- Advanced booking and allocation rules

#### v1.0.0 (Q2 2025+)

- Production-grade monitoring and observability
- Full feature parity with all Vantage API endpoints
- Multi-language client SDKs
- Enterprise support and SLAs

---

### Links

- **Documentation**: [docs/](docs/)
- **GitHub Issues**: [Issues](https://github.com/FinFocus/finfocus-plugin-vantage/issues)
- **Design Document**: [pulumi_cost_vantage_adapter_design_draft_v_0.md](pulumi_cost_vantage_adapter_design_draft_v_0.md)
- **FOCUS 1.2 Spec**: [finops-foundation.org/focus](https://finops-foundation.org/focus/)
