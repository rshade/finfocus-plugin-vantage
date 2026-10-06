# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working
with code in this repository.

## Project Overview

**finfocus-plugin-vantage** is a Go-based adapter that fetches normalized
cost/usage data from Vantage's REST API and maps it into FinFocus's
internal schema with FinOps FOCUS 1.2 fields. The adapter supports
historical backfills, daily incremental syncs, tag/label dimensions,
and forecast snapshots.

**Key Reference**: `pulumi_cost_vantage_adapter_design_draft_v_0.md`
contains the complete technical design.

## Build & Development Commands

```bash
make build          # Build the binary
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run golangci-lint
make fmt            # Format code with gofmt/goimports
make vet            # Run go vet
make tidy           # Verify go mod tidy doesn't change anything
make verify         # Run fmt, vet, and tidy (used by CI)
make wiremock-up    # Start mock server (for contract tests)
make wiremock-down  # Stop mock server
make demo           # Run pull against mocks and print records
go test ./... -v    # Run tests with verbose output
go test -run TestName  # Run single test
```

**CI Integration**: CI workflows invoke Make targets exclusively - always update
Makefile when adding validation steps, never add raw bash to CI workflows.

**GitHub Workflows & Docker Images**: See `.github/workflows/CLAUDE.md` for
complete workflow documentation. **CRITICAL**: When using Docker images in
GitHub Actions workflows, ALWAYS use specific version tags (e.g.,
`wiremock/wiremock:3.13.1`) or major.x notation (e.g., `wiremock/wiremock:3x`).
NEVER use major version only tags (e.g., `:3`) as they often don't exist on
Docker Hub. The `validate-workflows.yml` automatically verifies all Docker
images on PR changes.

## Module Dependencies

**Plugin is Self-Contained**: This plugin has NO external dependencies on
`finfocus` or `finfocus-spec`. It builds and tests independently.

**Note on go.work**: The `go.work` file at
`$GOPATH/src/github.com/rshade/go.work` has been disabled
(moved to `go.work.disabled`) since the plugin doesn't require it.
Remote CI builds work fine without workspace files - this is by design.

**Key Points**:

- `go.mod` is clean and publishable (no local replace directives)
- Binary builds successfully with `make build`
- All tests pass without workspace file
- Remote CI operates without any go.work dependency

## Code Style & Standards

- **Language**: Go 1.22+
- **Imports**: Standard library → third-party → internal packages
- **Naming**: camelCase for variables/functions, PascalCase for exported types
- **Error Handling**: Return errors explicitly, use `fmt.Errorf` for wrapping,
  always check context cancellation
- **Types**: Structs with `json`/`yaml` tags, implement interfaces explicitly
- **Logging**: Structured logging with fields: `adapter=vantage`, `operation`,
  `attempt`, `correlation_id`
- **Security**: Never log tokens; use environment variables for secrets;
  redact `Authorization` headers in logs

**Coverage Requirements**:

- Client package: ≥80% coverage
- Overall: ≥70% coverage

## Build Configuration

**Version Embedding**: The plugin embeds version at build time:

```go
// cmd/finfocus-plugin-vantage/main.go
var version = "dev"
```

Makefile sets this via LDFLAGS:

```makefile
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
```

**Important**: Unlike finfocus, the plugin does NOT reference
`github.com/rshade/finfocus/pkg/version` - it's fully independent.

**Cross-Platform Builds**: Always use `CGO_ENABLED=0` for static builds:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" ...
```

## Project Architecture

### Directory Structure

```text
cmd/finfocus-plugin-vantage/        # CLI entry point with Cobra commands
# (pull, backfill, forecast)
internal/vantage/
  ├── client/                   # REST client with retry/backoff logic
  ├── adapter/                  # Mapping and sync logic for FOCUS 1.2
  │   ├── config.go             # Config struct with yaml/json tags
  │   ├── adapter.go            # Sync orchestration
  │   ├── mapping.go            # Vantage row → CostRecord + FOCUS 1.2
  │   └── normalize.go           # Tag normalization, allow/deny filters
  └── contracts/                # Golden test fixtures
test/wiremock/                  # Mock server configurations
docs/                           # User documentation
```

### Core Interfaces

```go
// Client interface (internal/vantage/client)
type Client interface {
    Costs(ctx context.Context, q Query) (Page, error)
    Forecast(ctx context.Context, reportToken string, q ForecastQuery)
        (Forecast, error)
}

// Adapter interface (internal/vantage/adapter)
type Adapter interface {
    Sync(ctx context.Context, cfg Config, sink Sink) error
}

// Sink (from finfocus, persists cost records)
type Sink interface {
    Write(ctx context.Context, records []CostRecord) error
    UpdateBookmark(ctx context.Context, key string, value interface{}) error
}
```

## Vantage API Client

Official client (`github.com/vantage-sh/vantage-go` v0.1.13, MIT) wrapped by `internal/vantageapi` interface. Legacy hand-written client (`internal/vantage/client`) to be retired per VT-0.2.

**Setup:** `NewClient(baseURL, token)` validates http:// URLs are loopback-only (security guard for tokens).
**Base URL:** Defaults to `https://api.vantage.sh/v2` (v2 API, not v1).
**Auth:** Bearer token via `Authorization` header (never logged or printed).
**Amounts:** Always decimal strings; never use `float64` for money.

## Key Features & Implementation Notes

### Schema Mapping (Vantage → FinFocus → FOCUS 1.2)

Critical fields:

- `timestamp` (Vantage bucket start) → `usage_start_time`
- `provider` → `cloud_provider`
- `service` → `service_name`
- `account` → `billing_account_id`
- `region` → `region`
- `resource_id` → `resource_id`
- `cost` (net) → `net_cost`
- `amortized_cost` (if present) → `net_amortized_cost`
- `tags` → `labels` (normalized to lowercase kebab-case)

Missing fields become `nil` with a diagnostic note.

### Tag Normalization Strategy

- Normalize tag keys to lowercase kebab-case (configurable)
- Support allow/deny lists via regex
- Preserve original values in `labels_raw`
- Drop high-cardinality keys (e.g., pod UID)

### Incremental Sync & Idempotency

- **Incremental window**: D-3 to D-1 (configurable lag for late postings)
- **Bookmarks**: Store `last_successful_end_date` per
  `(workspace|report_token, date_hash)`
- **Idempotency key**: Deterministic hash of `(date, dimensions, metrics)`
  —same inputs always produce same keys

### Pagination & Retries

- Use server cursors (configurable page size, default 5000)
- Exponential backoff with jitter on HTTP 429/5xx
- Honor `X-RateLimit-*` headers when present
- Configurable max retries and request timeout

### Forecast Snapshots

- Stored as separate records with `metric_type="forecast"`
- Weekly snapshots; keep last 8 for MAPE evaluation

## Configuration

See `pulumi_cost_vantage_adapter_design_draft_v_0.md` Section 4 for
full YAML config reference.

**Required**:

```yaml
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}  # Via env var, never logged
params:
  cost_report_token: "cr_..."         # Preferred over workspace_token
```

**Optional**:

- `start_date`, `end_date`: ISO dates (default: 12 months back → today)
- `granularity`: `"day"` or `"month"` (default: `"day"`)
- `group_bys`: dimensions to group by (provider, service, account,
  project, region, resource_id, tags)
- `metrics`: cost types (cost, usage, effective_unit_price,
  amortized_cost, taxes, credits)
- `include_forecast`: enable forecast snapshots
- `page_size`: API page size (default 5000)
- `request_timeout_seconds`, `max_retries`: resilience config

## Testing Strategy

### Unit Tests

- Parameter building, pagination logic, tag normalization
- Golden JSON → CostRecord mapping assertions
- All client operations with `httptest`

### Contract Tests

**Local Development:**

- Run Wiremock server via `make wiremock-up`
- Docker Compose automatically mounts `test/wiremock/mappings/` directory
- Stub mappings are loaded on container startup
- Mock `/costs` endpoint (multi-page pagination)
- Mock `/forecast` endpoint
- Assert adapter produces correct records

**GitHub Actions CI:**

- WireMock runs as a service container (no volume mount support)
- **CRITICAL**: Stub mappings must be loaded via Admin API after service starts
- The `contract.yml` workflow includes a step to POST each mapping file:

  ```bash
  for mapping in test/wiremock/mappings/*.json; do
    curl -X POST -H "Content-Type: application/json" \
      -d @"$mapping" http://localhost:8080/__admin/mappings
  done
  ```

- Verify mappings loaded by checking `/__admin/mappings` endpoint
- **Common Issue**: If tests fail with "No response could be served as there
  are no stub mappings in this WireMock instance", the mapping load step
  likely failed or is missing

### Fixtures Location

- Golden samples: `internal/vantage/contracts/`
- Wiremock mappings: `test/wiremock/mappings/`
- Current mappings: `costs_page1.json`, `costs_page2.json`, `forecast.json`

## Code Quality & Linting

**golangci-lint v2.5.0 Configuration**:

- Uses strict defaults (no custom `.golangci.yml` file)
- Runs all recommended linters with strict enforcement
- **Zero issues allowed** - all code must pass without modification

**Common Linting Fixes** (when issues arise):

1. **Float Comparisons in Tests**: Use `assert.InEpsilon(t, expected, actual, 0.01)`
   instead of `assert.Equal()` for floating-point assertions

2. **Error Assertions**: Use `require.Error(t, err)` for setup error checks,
   `assert.Error()` for optional checks

3. **Magic Numbers**: Extract to named constants at package level:

   ```go
   const (
       defaultTimeout = 60 * time.Second
       defaultRetries = 5
   )
   ```

4. **Global Variables**: Avoid globals; use factory functions instead:

   ```go
   func buildRootCmd() *cobra.Command { /* ... */ }
   // Call in main(): cmd := buildRootCmd()
   ```

5. **Cognitive Complexity (>20)**: Extract helper methods:

   ```go
   func (a *Adapter) syncSingleRange(...) {
       a.applyBookmark(...)      // extracted
       records, err := a.fetchAndCollectRecords(...)  // extracted
       a.updateBookmark(...)     // extracted
   }
   ```

6. **Unused Returns**: Remove from signature if never used (unparam check)

7. **Named Returns**: Remove from signature; declare variables in function body

**Pre-commit Checklist**:

```bash
make lint   # Ensure zero issues
make test   # All tests pass
make build  # Binary builds successfully
```

## API Endpoints Used

- **GET /costs**: Query params include workspace/cost_report token, date
  range, granularity, group_bys, metrics, pagination cursors
- **GET /cost_reports/{token}/forecast**: Optional forecast data
- **POST /cost_reports/{token}/exports**: Optional CSV backfill for large
  ranges (>12 months)
- **GET /workspaces**: Optional discovery

Prefer **Cost Report token** for stable/curated filters; fall back to
Workspace token when needed.

## Common Development Workflows

### Run tests before commit

```bash
make lint && make test
```

### Debug a single test

```bash
go test -run TestCostsMapping -v
```

### Start mock server for manual testing

```bash
make wiremock-up
# Then run: ./finfocus-vantage pull --config test/config.yaml
make wiremock-down
```

### Add verbose logging for troubleshooting

Set `VANTAGE_DEBUG=1` in environment (implementation detail—check
adapter/diagnostics.go)

## Security & Secrets

- **Token handling**: Read from `FINFOCUS_VANTAGE_TOKEN` env var or `.env`
  (dev only); never log
- **Redaction**: All `Authorization` headers and token values must be
  redacted from logs
- **Least privilege**: Use cost_report token (scoped to single report)
  instead of workspace token when possible
- **No secrets in commits**: `.env` and credential files are in `.gitignore`

### AI Tool Security (opencode.json)

The `opencode.json` configuration uses a whitelist model for bash command access:

```json
"permission": {
  "bash": {
    "git diff": "allow",
    "git log": "allow",
    "make lint": "allow",
    "make test": "allow",
    // ... 25+ specific commands listed
    "*": "ask"  // All other commands require user approval
  }
}
```

**Key Principle**: Never use `"bash": true` (blanket access). Always enumerate
allowed commands explicitly. The fallback `"*": "ask"` ensures visibility
into any unapproved command attempts.

## References

- **Design Document**: `pulumi_cost_vantage_adapter_design_draft_v_0.md`
- **AGENTS.md**: Existing guidelines (build targets, code style, interfaces,
  testing)
- **finfocus**: Located at `../finfocus` (local dependency)
  - FOCUS 1.2 Schema: `internal/focus/`
  - Sink Interface: `pkg/ingest/`
- **finfocus-spec**: Located at `../finfocus-spec` (local dependency)

## Known Constraints & Assumptions

- Assumes `finfocus` is available (FOCUS types, Sink interface)
- Vantage API provides fields: `provider`, `service`, `account`, `project`,
  `region`, `resource_id`, `tags`, `cost`, `usage`, `amortized_cost`
  (when available), `currency`
- Actual field availability varies by provider and Vantage configuration
- No VQL filter support in v1; relies on curated Cost Reports

## Development Roadmap & Issue Tracking

**v0.1.0 MVP Structure**: 9 phases, 36 GitHub issues, 107-144 estimated hours

### Phases Overview

- **Phase 1**: Bootstrap ✅ (complete)
- **Phase 2**: CLI & Configuration (3 issues, #3-#5)
- **Phase 3**: REST Client (5 issues, #6-#10)
- **Phase 4**: Adapter Core (4 issues, #11-#12, #30-#31)
- **Phase 5**: Sync & Backfill (4 issues, #13-#16)
- **Phase 6**: Forecast (1 issue, #17)
- **Phase 7**: Testing (5 issues, #18-#22)
- **Phase 8**: Documentation & Release (7 issues, #23-#29)
- **Phase 9**: CI/CD & Hardening (7 issues, #33-#39)

### Issue Structure

Each GitHub issue includes:

- **Goal**: What to build
- **Acceptance Criteria**: Testable checkpoints (must all pass before closing)
- **Effort**: S(mall) 1-2h, M(edium) 3-5h, L(arge) 6-8h
- **Dependencies**: What issues must be done first
- **References**: Design sections and prompt files

**Example workflow:**

1. Pick issue from GitHub (e.g., #3: CLI Bootstrap)
2. Read acceptance criteria
3. Use corresponding prompt file (e.g., `prompts/bootstrap.md`)
4. Reference design document (e.g., Section 13)
5. Implement until all criteria checked
6. Run `make lint && make test`
7. Open PR, review, merge

### Prompt Files & Phases

```text
prompts/bootstrap.md              → Phase 2 (CLI & Config)
prompts/client.md                 → Phase 3 (REST Client)
prompts/adapter.md                → Phases 4-5 (Adapter & Sync)
prompts/tests.md                  → Phase 7 (Testing)
prompts/docs.md                   → Phase 8 (Documentation)
prompts/ci-and-repo-hardening.md  → Phase 9 (CI/CD & Hardening)
```

### Critical Path

Must follow this order (dependencies matter):

```text
Phase 2 (CLI) → Phase 3 (Client) → Phase 4 (Adapter) → Phase 5 (Sync)
                                 ↓
                            Phase 6 (Forecast)
                                 ↓
Phase 7 (Testing can start after Phase 3) → Phase 8 (Docs) → Phase 9 (Release)
```

### For AI-Assisted Development (Claude Code / OpenCode)

1. Load `pulumi_cost_vantage_adapter_design_draft_v_0.md` as context
2. Select phase prompt file (e.g., `prompts/client.md`)
3. Reference related GitHub issues for acceptance criteria
4. Generate code until all criteria met
5. Commit with Conventional Commits format (see below)

### Conventional Commits & Commitlint

**Format**: `type(scope): subject`

**Types**: feat, fix, docs, style, refactor, perf, test, chore, ci, revert
**Scopes**: infra, client, adapter, config, test, docs

**Examples**:

```text
feat(client): add exponential backoff retry logic
fix(adapter): correct FOCUS field mapping for amortized_cost
test(client): add pagination contract tests
chore(deps): update Go modules
```

**Validation**:

```bash
cat COMMIT_MESSAGE.md | npx commitlint
# Or use npm script:
npm run commitlint -- COMMIT_MESSAGE.md
```

Configuration in `commitlint.config.js` enforces:

- Type must be one of 10 allowed types
- Scope must be lowercase
- Subject required, no trailing period
- Header max 100 characters

### Markdown Linting

**Command**:

```bash
npm run markdownlint
```

Lints all `.md` files in the repository (excluding `node_modules`).
Uses `markdownlint-cli` for consistent markdown formatting.

### Repository Organization

- `prompts/` - OpenCode/Claude Code implementation guides (now includes
  PROJECT_SUMMARY.md and DEVELOPMENT_ROADMAP.md)
- `docs/` - User documentation (config, troubleshooting, deployment)
- `.github/` - GitHub workflows, templates, actions config
- `internal/vantage/` - Adapter code (client, adapter, contracts)
- `cmd/finfocus-plugin-vantage/` - CLI entry point
- `test/wiremock/` - Mock server fixtures

### Success Criteria for v0.1.0

- ✓ All 36 issues closed
- ✓ ≥70% overall test coverage
- ✓ ≥80% client package coverage
- ✓ 0 linting errors (`make lint` passes)
- ✓ 0 security vulnerabilities
- ✓ Wiremock contract tests passing
- ✓ Golden fixture tests passing
- ✓ End-to-end integration test passing
- ✓ Complete documentation (CONFIG.md, TROUBLESHOOTING.md, FORECAST.md)
- ✓ GitHub Actions CI/CD passing on all workflows
- ✓ v0.1.0 GitHub release published

## Post-Implementation Cleanup

Once all 36 GitHub issues are closed and v0.1.0 is feature-complete:

### Prompts Directory

The `prompts/` directory contains implementation guides for Claude Code /
OpenCode and should be archived or removed once development is complete:

**Files to remove or archive:**

```bash
prompts/bootstrap.md                  # No longer needed
prompts/client.md                     # No longer needed
prompts/adapter.md                    # No longer needed
prompts/tests.md                      # No longer needed
prompts/docs.md                       # No longer needed
prompts/ci-and-repo-hardening.md      # No longer needed
prompts/PROJECT_SUMMARY.md            # Archive to docs/ or remove
prompts/DEVELOPMENT_ROADMAP.md        # Archive to docs/ or remove
```

#### Optional: Archive instead of delete

```bash
mkdir docs/archive-prompts/
mv prompts/* docs/archive-prompts/
rmdir prompts
```

### Keep in Repository Root

- `CLAUDE.md` - Future developer reference (this file, keep updated)
- `TODO.md` - Implementation record and history (keep for reference)
- `pulumi_cost_vantage_adapter_design_draft_v_0.md` - Technical design
  (keep indefinitely)
- `AGENTS.md` - Build/test procedures (keep updated)

### Documentation to Keep

Everything in `docs/` directory:

- `docs/CONFIG.md` - User-facing configuration reference
- `docs/TROUBLESHOOTING.md` - Support and debugging
- `docs/FORECAST.md` - Feature documentation
- `docs/DEPLOYMENT.md` - Operations guide
- `docs/examples/` - Example configurations

This keeps the repository clean while preserving historical implementation
guidance in archived form if needed for future versions.

## Documentation

### Documentation Files (Issues #5, #23, #24, #26)

The following user-facing documentation has been created:

1. **docs/CONFIG.md** (Issue #5: ≥10 issues)
   - Comprehensive configuration reference with all 12 config fields
   - YAML example from design section 4 with all options
   - Complete environment variable reference table
   - 7 configuration patterns: Quick Start, Backfill, Daily Sync,
     High-Granularity Analysis, Conservative Setup, Tag Filtering,
     Multi-Cloud
   - Security best practices with DO/DON'T lists
   - Validation error troubleshooting

2. **docs/TROUBLESHOOTING.md** (Issue #23: ≥10 issues)
   - 12 documented common issues with solutions:
     - Authentication Failed (401)
     - Rate Limit Exceeded (429) with backoff explanation
     - Invalid Configuration with error table
     - Pagination Errors & Cursor Issues
     - Missing or Null Fields
     - Connection Timeout
     - Data Duplication or Missing Records
     - Tag/Field Mapping Issues
     - Memory or Performance Issues
     - Wiremock Mock Server Issues
     - Unresponsive or Stalled Sync
     - Cost Data Discrepancies
   - Verbose logging section with VANTAGE_DEBUG=1
   - Wiremock recording capture instructions
   - Getting help guidelines

3. **docs/FORECAST.md** (Issue #24: ≥3 examples)
   - Forecast data flow explanation (4 steps)
   - Snapshot schedule and retention policy
   - 4 usage examples:
     - Enable forecast in regular sync
     - Generate forecast separately via CLI
     - Query forecast records with SQL examples
     - Configure snapshot settings
   - Output locations and Sink integration
   - MAPE evaluation for forecast accuracy
   - Troubleshooting section
   - Best practices for configuration and operations

4. **CHANGELOG.md** (Issue #26: All requirements)
   - New Features section covering all v0.1.0 features
   - Known Limitations (6 categories)
   - Breaking Changes (none for v0.1.0)
   - Upgrading guide with installation and config
   - Contributors acknowledged
   - Future Roadmap (v0.2.0, v0.3.0, v1.0.0)

### Documentation Linting Standards

All documentation passes `npm run markdownlint` validation:

- Line length ≤80 characters
- Proper heading hierarchy (H1 → H2 → H3)
- Blank lines around code fences
- Blank lines around lists
- Proper markdown syntax
- EOF newline on all files

### Documentation Cross-References

Documentation files link to each other:

- CONFIG.md → TROUBLESHOOTING.md (for common errors)
- CONFIG.md → FORECAST.md (for forecast feature)
- TROUBLESHOOTING.md → CONFIG.md (for configuration)
- TROUBLESHOOTING.md → FORECAST.md (for forecast issues)
- FORECAST.md → CONFIG.md (for configuration options)
- CHANGELOG.md → docs/CONFIG.md, docs/TROUBLESHOOTING.md, docs/FORECAST.md

---

## Troubleshooting

**Auth errors**: Verify `FINFOCUS_VANTAGE_TOKEN` is set and valid
**Rate limits (429)**: Adapter automatically backs off; check
`X-RateLimit-Reset` header
**Pagination issues**: Enable verbose logging and inspect cursor values
**Tag mapping failures**: Review `tag_prefix_filters` and allow/deny lists
in config
**Commit validation fails**: Ensure message follows Conventional Commits
format; check `commitlint.config.js`
**Linting errors**: See "Code Quality & Linting" section above for common
fixes; all 151 previous issues have been resolved
**Issue not clear**: Read GitHub issue body for acceptance criteria;
reference corresponding prompt file; check design document section

---
