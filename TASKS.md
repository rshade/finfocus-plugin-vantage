# FinFocus Vantage Plugin - Development Tasks to v0.1.0

## Release and tag rules (family standard, added 2026-10-06)

Every release-please config in this family needs these three settings on the `.` package, and a test that fails
when any is wrong:

1. `"include-component-in-tag": false`. Without it a config that sets `package-name` creates tags like
   `finfocus-plugin-<name>-v0.1.0`. `release.yml` runs on `release: created`, GoReleaser parses the tag as semver,
   fails, and the release has no binaries. opencost hit this on 2026-10-06; azure-public sets the key.
2. `"initial-version": "0.1.0"`. Without it the first release PR can propose 1.0.0 (opencost issue 77).
3. `.release-please-manifest.json` keeps `"."` at `0.0.0` until the first release PR merges. A `0.1.0` before then
   records 0.1.0 as already shipped.

The test reads `release-please-config.json` and the manifest (`jq` or Go). Break check: delete each key in turn
and the test fails.

A merged release PR is not a release. Done means: the tag is plain `vX.Y.Z`, the GoReleaser run is green, and
`gh release view vX.Y.Z --json assets --jq '.assets | length'` is above 0. Release tooling (goreleaser,
release-please, Homebrew, Docker) is ask-first: change it only when the task or the invocation says so. The agent
never deletes or moves a tag or release; the owner re-cuts one.

State of this repo on 2026-10-06: not compliant. `package-name` is set, `include-component-in-tag` and
`initial-version` are absent, the manifest is `0.0.0`, and no tag exists. Fix the config before the first release
PR merges.

## Current State

### Repository Status

- **Local checkout**: `/github/go/src/github.com/rshade/finfocus-plugin-vantage`
- **GitHub repo**: `rshade/finfocus-plugin-vantage`
- **Branch**: `main` (up to date with remote)
- **Status**: VT-1.1 + VT-1.2 COMPLETE ✅; VT-1.3 and initial VT-2/VT-3 work in progress

### Version Requirements (VT-1.2 Complete)

- **Go version**: 1.27.1 ✅ (required by finfocus-spec v0.7.0)
- **finfocus-spec**: v0.7.0 ✅ (added to go.mod)
  - FOCUS 1.4 cost/usage columns available
  - Per-request credentials support available
  - ax-go v0.7.0 available transitively (plugins do not import directly)

### Codebase Architecture (gRPC migration in progress)

**Current design**: pluginsdk gRPC server with GetActualCost mapping. The legacy
sync adapter, Sink interface, and hand-written HTTP client have been retired.

- `cmd/finfocus-plugin-vantage/main.go`: pluginsdk gRPC server entry point
- `internal/plugin/`: CostSource implementation and RPC to Vantage mapping
- `internal/vantageapi/`: wrapper around the official Vantage Go client

**Migration status for v0.1.0**:

- ✅ Replace Cobra CLI with **gRPC server** (pluginsdk.Serve)
- ✅ Upgrade Go to 1.27.1 and add finfocus-spec v0.7.0
- ✅ Retire legacy `Sink`, sync adapter, and HTTP client (VT-0.2)
- ✅ Rename module to `github.com/rshade/finfocus-plugin-vantage`
- 🟡 Implement `CostSourceService` methods (GetActualCost, Supports, and Name are implemented; projected/pricing/estimate return UNIMPLEMENTED)
- ✅ Document unavailable FOCUS 1.4 invoice and commitment source fields
- ✅ Support per-request credentials (v0.7.0 feature)
- ✅ Use the official Vantage client through `internal/vantageapi`

### Build & Test Status (VT-1.1 & VT-1.2 Complete)

```bash
$ go build ./...
(Success - no errors) ✅

$ go test ./...
ok  github.com/rshade/finfocus-plugin-vantage/cmd/finfocus-plugin-vantage
ok  github.com/rshade/finfocus-plugin-vantage/internal/plugin
ok  github.com/rshade/finfocus-plugin-vantage/internal/vantageapi

$ go vet ./...
(No issues) ✅

Go version: 1.27.1 ✅
Module: github.com/rshade/finfocus-plugin-vantage ✅
finfocus-spec: v0.7.0 ✅
```

### Vantage API Endpoints Used

- **`/costs`**: Paginated cost data retrieval (Query struct with date_range,
  group_bys, metrics)
- **`/forecast`**: Forecast snapshots per report_token (ForecastQuery struct)
- **Auth**: Bearer token in Authorization header (redacted in logs)
- **Rate limiting**: Handled by pager with backoff logic

### Open Issues Summary

- **Total open**: 20 issues (4 stale CLI-feature issues closed: #13, #14, #15, #17)
- **Disposition**:
  - **Covering v0.1.0** (15): Issues in VT-tasks
    ([#10](https://github.com/rshade/finfocus-plugin-vantage/issues/10),
    [#11](https://github.com/rshade/finfocus-plugin-vantage/issues/11),
    [#12](https://github.com/rshade/finfocus-plugin-vantage/issues/12),
    [#16](https://github.com/rshade/finfocus-plugin-vantage/issues/16),
    [#18-21](https://github.com/rshade/finfocus-plugin-vantage/issues/18),
    [#27-28](https://github.com/rshade/finfocus-plugin-vantage/issues/27),
    [#30](https://github.com/rshade/finfocus-plugin-vantage/issues/30),
    [#34-35](https://github.com/rshade/finfocus-plugin-vantage/issues/34))
  - **Stale/obsolete (CLOSED)** (4) - not planned:
    [#13](https://github.com/rshade/finfocus-plugin-vantage/issues/13),
    [#14](https://github.com/rshade/finfocus-plugin-vantage/issues/14),
    [#15](https://github.com/rshade/finfocus-plugin-vantage/issues/15),
    [#17](https://github.com/rshade/finfocus-plugin-vantage/issues/17)
    (CLI sync features: incremental sync, backfill, bookmarks, forecast)
  - **Needs decision** (1):
    [#38](https://github.com/rshade/finfocus-plugin-vantage/issues/38)
    (demo scripts—clarify purpose for gRPC plugin)
  - **post-v0.1.0** (4): [#25](https://github.com/rshade/finfocus-plugin-vantage/issues/25),
    [#29](https://github.com/rshade/finfocus-plugin-vantage/issues/29),
    [#31](https://github.com/rshade/finfocus-plugin-vantage/issues/31),
    [#33](https://github.com/rshade/finfocus-plugin-vantage/issues/33),
    [#39](https://github.com/rshade/finfocus-plugin-vantage/issues/39),
    [#43](https://github.com/rshade/finfocus-plugin-vantage/issues/43)

---

## Vantage API Client Adoption

**Phase 0 (DONE):** Wrapper for official `github.com/vantage-sh/vantage-go` v0.1.13 (MIT).
- Interface: `internal/vantageapi/Client` (insulates from generated code changes)
- Base URL: `https://api.vantage.sh/v2` (v2 API)
- Security: http:// URLs rejected except for loopback addresses (testing guard for bearer tokens)
- Legacy `internal/vantage/client` and `internal/vantage/adapter` were removed
  after confirming there were no remaining production callers.

**VT-0.2: Retire Legacy HTTP Client (DONE)**
- Removed the unused legacy sync adapter and hand-written HTTP client.
- Removed WireMock contract-test infrastructure and the obsolete CLI demo.
- The gRPC plugin's HTTP integration test uses an in-process mock server.
- Acceptance: no legacy package imports; tests and lint pass.

**API Discrepancies to Fix** (from research file `../finfocus-pm/research/vantage-api-facts.md`):
Tasks below reference these corrections:

1. Base URL: Add `/v2` segment (plugin uses `/`, spec uses `/v2`) ✅ (vantageapi defaults to /v2)
2. Parameters: Rename `start_at`/`end_at` → `start_date`/`end_date`, `granularity` → `date_bin`, `metrics` → `settings[*]`, `cursor`/`page_size` → `page`/`limit`
3. Response: Map `data[]` → `costs[]`, `next_cursor`/`has_more` → `links.next`, amounts as strings not floats, `tags` is array not map
4. Forecast: Path `/forecast` → `/forecasted_costs`, add `provider`/`service` params, response fields `date`, `amount`, `provider`, `service`
5. Settings: Replace `metrics[]` with `settings[amortize]`, `settings[include_*]`, `settings[aggregate_by]`
6. Rate limits: Header `x-rate-limit-reset` is Unix timestamp, not seconds; add 5 req/5s throttle for report endpoints
7. VQL filters: `resource_id` filter needs `provider` AND `service`; support single-quote only syntax
8. Currency: Support multi-currency rows; fail or flag if mixed; per-row `currency` field
9. Tokens: Report tokens are query selectors, not credentials; bearer token is the real credential
10. Latency: Two-day data clip by default; $0 can mean "not imported yet"

## Development Phases to v0.1.0

Each task has clear acceptance checks. **BLOCKED-ON-CREDENTIALS** tasks cannot
be validated without a live Vantage API token/account.

### Phase 1: Architectural Migration (CRITICAL: gates all subsequent work)

#### VT-1.1: Rename Module & Update Remote

- **Related issues**: (Architecture prerequisite, no GitHub issue)
- **Description**: Migrate codebase from the legacy product name to FinFocus
  (`finfocus-plugin-vantage`)
- **Files touched**: `go.mod`, `cmd/*/main.go` (package imports),
  `.github/**/*.yml` (artifact names), `Makefile` (binary name), `README.md`,
  `CLAUDE.md`
- **Commands**:
  - `git remote set-url origin git@github.com:rshade/finfocus-plugin-vantage.git`
  - Search-replace module path: legacy product name →
    `github.com/rshade/finfocus-plugin-vantage`
  - Rename binary in Makefile/cmd: legacy name →
    `finfocus-plugin-vantage`
  - Update CLAUDE.md project overview
- **Acceptance check**:
  - `go mod tidy` succeeds with no changes
  - `go build ./cmd/finfocus-plugin-vantage` builds successfully
  - `go test ./...` passes
  - Remote URL is `git@github.com:rshade/finfocus-plugin-vantage.git`

#### VT-1.2: Upgrade Go to 1.27.1 & Add finfocus-spec v0.7.0

- **Related issues**: (Architecture prerequisite, no GitHub issue)
- **Description**: Upgrade Go version to 1.27.1 (required by finfocus-spec v0.7.0)
  and add finfocus-spec v0.7.0 to `go.mod`
- **Files touched**: `go.mod`, `go.sum`
- **Reference**: finfocus-spec v0.7.0 includes FOCUS 1.4 columns, per-request
  credentials, and contract commitment features
- **Commands**:
  - Update `go.mod`: change `go 1.25.0` to `go 1.27.1`
  - `go get github.com/rshade/finfocus-spec@v0.7.0`
  - `go mod tidy`
- **Acceptance check**:
  - `go.mod` Go version is 1.27.1
  - `go.mod` contains `finfocus-spec v0.7.0` (or newer)
  - `go mod verify` passes
  - Pluginsdk import works: `github.com/rshade/finfocus-spec/sdk/go/pluginsdk`
  - Verify v0.7.0 features available: `pluginsdk.PerRequestCredentialConsumer`,
    `FocusRecordBuilder.WithInvoiceDetailID()`

#### VT-1.3: Replace Cobra CLI with gRPC Server Skeleton

- **Description**: Remove `cmd/finfocus-plugin-vantage/main.go` Cobra
  implementation; replace with minimal gRPC server using pluginsdk.Serve()
- **Files touched**: `cmd/finfocus-plugin-vantage/main.go`,
  `internal/plugin/plugin.go` (new file)
- **Reference**: `../finfocus-plugin-aws-public/cmd/finfocus-plugin-aws-public/main.go`
- **Code pattern**:

```go
// cmd/finfocus-plugin-vantage/main.go
func main() {
  plugin := &Plugin{/* ... */}
  if err := pluginsdk.Serve(context.Background(), plugin); err != nil {
    log.Fatalf("failed to serve: %v", err)
  }
}

// internal/plugin/plugin.go
type Plugin struct{ /* ... */ }
func (p *Plugin) Name(ctx context.Context, _ *empty.Empty)
  (*wrapperspb.StringValue, error) {
  return wrapperspb.String("vantage"), nil
}
func (p *Plugin) Supports(ctx context.Context,
  req *finfocuspb.ResourceDescriptor)
  (*finfocuspb.SupportsResponse, error) {
  return &finfocuspb.SupportsResponse{Supported: true}, nil // TODO
}
func (p *Plugin) GetActualCost(ctx context.Context,
  req *finfocuspb.GetActualCostRequest)
  (*finfocuspb.GetActualCostResponse, error) {
  return nil, errors.New("not yet implemented")
}
```

- **Acceptance check**:
  - `go build ./cmd/finfocus-plugin-vantage` succeeds
  - Binary runs without panic: `./bin/finfocus-plugin-vantage` (should emit
    `PORT=<port>` to stdout and serve gRPC)
  - `PORT` announcement appears on stdout within 2 seconds
  - Server shuts down gracefully on SIGTERM

---

### Phase 2: Core GetActualCost RPC Implementation

#### VT-2.1: Define GetActualCost Request/Response Mapping

- **Related issues**: [#10](https://github.com/rshade/finfocus-plugin-vantage/issues/10),
  [#11](https://github.com/rshade/finfocus-plugin-vantage/issues/11),
  [#12](https://github.com/rshade/finfocus-plugin-vantage/issues/12),
  [#30](https://github.com/rshade/finfocus-plugin-vantage/issues/30)
- **Description**: Design mapping from finfocus-spec `GetActualCostRequest` to
  Vantage API query, and response back to `GetActualCostResponse`
- **Files touched**: `docs/MAPPING.md` (new), `internal/plugin/adapter.go` (new)
- **Key questions to document**:
  - How to extract Vantage resource identifier from `ResourceDescriptor`
    (provider, resource_type, tags)?
  - Which Vantage API fields map to GetActualCostResponse fields (cost,
    currency, billing_detail)?
  - How to handle multi-day aggregation (if request spans multiple days, query
    Vantage and sum)?
  - Error handling: what if Vantage returns no data, rate limit exceeded, auth
    fails?
- **Reference**: CLAUDE.md mentions Vantage query uses `provider`, `service`,
  `account_id`, `region`, `resource_id`, `labels`
- **Acceptance check**:
  - `docs/MAPPING.md` exists with detailed mappings
  - `internal/plugin/adapter.go` has function signature: `func AdaptVantageToFinfocus(...)`
  - Mapping handles at least 5 error cases (no data, rate limit, auth, timeout,
    malformed)

#### VT-2.2: Implement GetActualCost Core Logic (BLOCKED-ON-CREDENTIALS)

**Implementation complete:** Current resource descriptor support, documented
VQL filters, required service resolution, explicit billed cost settings,
context status preservation, bounded 429/5xx retries, and shared API pacing.
Regression tests exercise query construction, redaction, window boundaries,
and caller billing account overrides. Live-account validation remains separate.

- **Related issues**: [#16](https://github.com/rshade/finfocus-plugin-vantage/issues/16)
- **Description**: Fetch actual costs from Vantage API, map to finfocus-spec
  response
- **Files touched**: `internal/plugin/plugin.go`, `internal/plugin/adapter.go`,
  `internal/vantage/client/adapter.go` (refactor existing mapping)
- **Implementation steps**:
  1. Extract Vantage query from `ResourceDescriptor` + `GetActualCostRequest`
  2. Query Vantage `/costs` endpoint with client
  3. Aggregate results (if multi-day)
  4. Map to `GetActualCostResponse` with cost, currency, billing_detail
  5. Handle errors: return gRPC error codes (INVALID_ARGUMENT, UNAVAILABLE,
     UNAUTHENTICATED, etc.)
- **Acceptance check**:
  - RPC responds to valid request (requires live Vantage token: **BLOCKED**)
  - Invalid requests (missing provider, invalid date range) return gRPC errors
    with ErrorCode enum
  - Rate limit errors (429) trigger exponential backoff
  - Response includes cost_per_month (or daily cost extrapolated to month),
    currency, billing_detail
  - Unit tests mock Vantage client responses (use existing test fixtures from
    contract tests)

---

### Phase 3: Supporting RPC Methods

#### VT-3.1: Implement Supports() RPC

- **Description**: Check if plugin supports a given resource (provider, type,
  region)
- **Files touched**: `internal/plugin/supports.go` (new)
- **Logic**:
  - Provider: "vantage" only (or empty? depends on architecture - clarify with
    finfocus team)
  - Type: Any resource type (Vantage is cloud-agnostic)
  - Region: Any region (Vantage aggregates across regions)
- **Acceptance check**:
  - `Supports(provider="vantage", type="ec2", region="us-east-1")` →
    `supported=true`
  - `Supports(provider="aws", type="ec2", region="us-east-1")` →
    `supported=false` (wrong provider)

#### VT-3.2: Implement Name() RPC

- **Description**: Return plugin name for discovery
- **Files touched**: `internal/plugin/plugin.go` (update Name method)
- **Implementation**: Return "vantage"
- **Acceptance check**: `Name()` returns "vantage"

#### VT-3.3: Implement optional GetPricingSpec() RPC

- **Description**: Return pricing data structure (optional for finfocus core)
- **Files touched**: `internal/plugin/plugin.go`
- **Decision**: Return unimplemented error or basic pricing details?
- **Acceptance check**: Method exists, returns error or valid response

#### VT-3.4: Handle trace_id Metadata

- **Description**: Extract and log trace_id from gRPC metadata (finfocus-spec
  v0.6+)
- **Files touched**: `internal/plugin/plugin.go` (all RPC methods)
- **Pattern**: See finfocus-plugin-aws-public for example
- **Acceptance check**: trace_id from request metadata appears in structured
  logs (if logging is implemented)

#### VT-3.5: Implement Per-Request Credentials (v0.7.0 Feature)

- **Description**: Support opt-in per-request Vantage API token via gRPC
  metadata headers (`x-finfocus-credential-vantage-token`)
- **Files touched**: `internal/plugin/plugin.go`, `internal/vantage/client/client.go`
- **Implementation**:
  1. Declare `PerRequestCredentialConsumer` interface in plugin (just one method,
     `ConsumesPerRequestCredentials()`)
  2. Extract credentials from gRPC metadata using
     `pluginsdk.ExtractCredentials(ctx)`
  3. Read named credential `"vantage-token"` (or decide on name)
  4. Use per-request token if present, fall back to env var `FINFOCUS_VANTAGE_TOKEN`
  5. Add `supports_per_request_credentials: "true"` to `GetPluginInfo` metadata
  6. Log which auth method is used (per-request vs env var)
- **Reference**: finfocus-spec v0.7.0 `pluginsdk/credentials.go` - metadata key
  is `x-finfocus-credential-<name>` (lowercase), max 16 credentials, 64 char
  name, 4096 char value, 16KB total per request
- **Acceptance check**:
  - Plugin compiles and declares `PerRequestCredentialConsumer`
  - `GetPluginInfo` includes `supports_per_request_credentials: "true"`
  - Extract credentials works when header is present
  - Falls back to env var when header is absent
  - Invalid credentials (missing, too long) are rejected with proper error codes

#### VT-3.6: Support FOCUS 1.4 Billing/Invoice Fields (v0.7.0 Feature)

- **Description**: Add FOCUS 1.4 fields to actual cost results (requires analysis
  of Vantage data availability)
- **Files touched**: `internal/plugin/adapter.go` (refactor mapping logic),
  `docs/FOCUS_1_4_MAPPING.md` (new)
- **v0.7.0 FOCUS 1.4 fields**:
  - `invoice_detail_id` (field 67): Unique invoice line identifier
  - `commitment_program_eligibility_details` (field 68): JSON object string with
    commitment metadata
- **Decision needed**: Can Vantage data provide these fields?
  - Does Vantage CUR export `invoice_id` or similar?
  - Does Vantage expose commitment program metadata?
  - Should we leave these nil/empty for v0.1.0 and add in v0.2.0?
- **Builder pattern**: Use `pluginsdk.FocusRecordBuilder` (v0.7.0 feature)
  instead of custom struct - verify if mapping can reuse
  `WithInvoiceDetailID()` and `WithCommitmentProgramEligibilityDetails()` methods
- **Acceptance check**:
  - `docs/FOCUS_1_4_MAPPING.md` explains which Vantage fields map to invoice
    fields (or why they're empty)
  - Adapter populates fields when data is available
  - Can toggle FOCUS 1.4 fields on/off without breaking FOCUS 1.2 compatibility
  - If using pluginsdk.FocusRecordBuilder, verify validation rules work

---

### Phase 4: Testing & Validation

#### VT-4.1: Unit Tests for GetActualCost Mapping

- **Related issues**: [#20](https://github.com/rshade/finfocus-plugin-vantage/issues/20),
  [#21](https://github.com/rshade/finfocus-plugin-vantage/issues/21)
- **Description**: Test Vantage → finfocus-spec response mapping with golden
  fixtures
- **Files touched**: `internal/plugin/plugin_test.go` (new),
  `internal/testdata/vantage_responses.json` (fixtures)
- **Test cases**: 10+ scenarios (different resource types, regions, aggregation
  windows, error responses)
- **Acceptance check**:
  - All tests pass: `go test ./internal/plugin/...`
  - ≥80% coverage on `plugin.go`, ≥75% on `adapter.go`

#### VT-4.2: Integration Test with Mock Vantage

- **Related issues**: [#18](https://github.com/rshade/finfocus-plugin-vantage/issues/18),
  [#19](https://github.com/rshade/finfocus-plugin-vantage/issues/19),
  [#28](https://github.com/rshade/finfocus-plugin-vantage/issues/28),
  [#34](https://github.com/rshade/finfocus-plugin-vantage/issues/34)
- **Description**: Test plugin RPC against mocked Vantage API (Wiremock or go
  httptest)
- **Files touched**: `test/integration/plugin_test.go` (new)
- **Setup**: Mock server serving `/costs` + `/forecast` endpoints
- **Test flow**: Start gRPC server, make RPC call, verify response
- **Acceptance check**:
  - Test runs without external network: `go test -tags=integration ./test/integration/...`
  - Test passes: RPC response populated with cost data

#### VT-4.3: End-to-End Test with finfocus Core (if feasible)

- **Description**: Test plugin when invoked by finfocus core (optional for
  v0.1.0)
- **Files touched**: `test/e2e/` (new)
- **Requirements**: Requires Vantage token (BLOCKED-ON-CREDENTIALS)
- **Acceptance check**: Plugin launches, core connects, cost query succeeds

---

### Phase 5: Release & Distribution

#### VT-5.1: Finalize Binary Naming & Build Artifacts

- **Related issues**: [#35](https://github.com/rshade/finfocus-plugin-vantage/issues/35)
- **Description**: Ensure Makefile builds `bin/finfocus-plugin-vantage`, matches
  registry expectations
- **Files touched**: `Makefile`, `.goreleaser.yaml`
- **Registry expectation** (from finfocus core): Binary named
  `finfocus-plugin-vantage` with `--version` flag support
- **Acceptance check**:
  - `make build` produces `bin/finfocus-plugin-vantage`
  - `./bin/finfocus-plugin-vantage --version` returns version string
  - Version matches Git tag (via `-ldflags "-X main.version=..."``)

#### VT-5.2: GitHub Release & Tag v0.1.0

- **Related issues**: [#27](https://github.com/rshade/finfocus-plugin-vantage/issues/27)
- **Description**: Tag v0.1.0, create GitHub release with binary artifact
- **Files touched**: `CHANGELOG.md` (updated), Git tag, GitHub Release (via API)
- **Artifacts**: Single `finfocus-plugin-vantage` binary (linux/amd64 initially)
- **Acceptance check**:
  - Git tag `v0.1.0` exists: `git tag -l v0.1.0`
  - GitHub Release page shows binary + changelog
  - Binary downloads and runs: `./finfocus-plugin-vantage` (announces PORT)

#### VT-5.3: Document Usage & Installation

- **Description**: Update README + add docs for finfocus integration (different
  from legacy product docs)
- **Files touched**: `README.md`, `docs/USAGE.md` (new),
  `docs/VANTAGE_API.md` (setup guide)
- **Content**:
  - Installation: `finfocus plugin install github:rshade/finfocus-plugin-vantage@v0.1.0`
  - Configuration: Vantage API token setup (env var or config file)
  - Supported resource types: Which clouds/services (determine based on Vantage
    capabilities)
  - Limitations: No projected costs, only actual (from Vantage CUR data)
- **Acceptance check**: README updated with finfocus references, installation
  tested locally

---

## Issue Index (20 Open Issues, 4 Closed)

| # | Title | Disposition | Note |
| --- | --- | --- | --- |
| [#10](https://github.com/rshade/finfocus-plugin-vantage/issues/10) | 2.5: Logger Interface | VT-2.1 | Refactor existing |
| [#11](https://github.com/rshade/finfocus-plugin-vantage/issues/11) | 3.1: Vantage→FOCUS | VT-2.1 | Refactor output |
| [#12](https://github.com/rshade/finfocus-plugin-vantage/issues/12) | 3.2: Tag Normalization | VT-2.1 | Adapt for gRPC |
| [#16](https://github.com/rshade/finfocus-plugin-vantage/issues/16) | 4.4: Error Recovery | VT-2.2 | Reuse in gRPC |
| [#18](https://github.com/rshade/finfocus-plugin-vantage/issues/18) | 6.1: Wiremock Setup | VT-4.2 | Reuse for gRPC |
| [#19](https://github.com/rshade/finfocus-plugin-vantage/issues/19) | 6.2: Contract Tests | VT-4.2 | Test gRPC |
| [#20](https://github.com/rshade/finfocus-plugin-vantage/issues/20) | 6.3: Golden Fixtures | VT-4.1 | Still needed |
| [#21](https://github.com/rshade/finfocus-plugin-vantage/issues/21) | 6.4: Coverage | VT-4.1 | Still needed |
| [#25](https://github.com/rshade/finfocus-plugin-vantage/issues/25) | 7.3: Example Configs | post-v0.1.0 | Token setup |
| [#27](https://github.com/rshade/finfocus-plugin-vantage/issues/27) | 8.1: Release v0.1.0 | VT-5.2 | Tag & release |
| [#28](https://github.com/rshade/finfocus-plugin-vantage/issues/28) | 8.2: E2E Mocks | VT-4.2 | Test gRPC |
| [#29](https://github.com/rshade/finfocus-plugin-vantage/issues/29) | 8.3: Deployment Docs | post-v0.1.0 | gRPC plugin |
| [#30](https://github.com/rshade/finfocus-plugin-vantage/issues/30) | 3.3: Idempotency | VT-2.1 | Dedup logic |
| [#31](https://github.com/rshade/finfocus-plugin-vantage/issues/31) | 3.4: Diagnostics | post-v0.1.0 | Quality metrics |
| [#33](https://github.com/rshade/finfocus-plugin-vantage/issues/33) | 9.1: CI/CD | post-v0.1.0 | Automation |
| [#34](https://github.com/rshade/finfocus-plugin-vantage/issues/34) | 9.2: Wiremock | VT-4.2 | Reuse for gRPC |
| [#35](https://github.com/rshade/finfocus-plugin-vantage/issues/35) | 9.3: Makefile | VT-5.1 | Version targets |
| [#38](https://github.com/rshade/finfocus-plugin-vantage/issues/38) | 9.6: Demo Scripts | needs-decision | gRPC purpose? |
| [#39](https://github.com/rshade/finfocus-plugin-vantage/issues/39) | 9.7: Repo Hardening | post-v0.1.0 | Validation |
| [#43](https://github.com/rshade/finfocus-plugin-vantage/issues/43) | Renovate Dashboard | post-v0.1.0 | Ongoing |

**Closed Issues (Not Planned):**

- #13 - 4.1: Incremental Sync (CLI feature, obsolete)
- #14 - 4.2: Backfill Logic (CLI feature, obsolete)
- #15 - 4.3: Bookmarks (CLI sync state, obsolete)
- #17 - 5.1: Forecast (CLI feature, obsolete)

---

## Open Questions

1. **Vantage Account Access for Testing**:
   - Do we have a free/trial Vantage account for end-to-end testing?
   - Who provides the API token for VT-2.2 integration testing?
   - Should v0.1.0 be released without live Vantage testing, or should testing
     be marked BLOCKED?

2. **Provider Naming Convention**:
   - Should GetActualCost accept `provider="vantage"` or `provider=""` (Vantage
     is cloud-agnostic)?
   - How does finfocus core expect to identify Vantage as the provider in the
     plugin routing layer?
   - Reference: Check finfocus-spec for provider naming conventions in
     pluginsdk.

3. **Cost Grouping & Aggregation**:
   - When a request comes for a single resource, how do we identify it in
     Vantage data?
   - Vantage groups costs by dimensions (provider, service, account, region,
     resource_id, tags).
   - Should we query Vantage with `group_bys=[resource_id]` and filter by the
     requested ResourceDescriptor?
   - Or does Vantage have a direct resource lookup by ARN/ID?

4. **Historical Window**:
   - GetActualCost RPC: should we return costs for today, yesterday, or a
     range?
   - finfocus-spec contract: does GetActualCostRequest specify a date range?
   - Vantage CUR data has D-3 lag; should we enforce that or query most recent
     available?

5. **Currency Handling**:
   - If Vantage returns costs in multiple currencies (per resource), how do we
     handle?
   - finfocus-spec GetActualCostResponse has single `currency` field.
   - Should we fail or convert to a base currency (USD)?

6. **Authentication**:
   - How does finfocus core pass Vantage API token to plugin?
   - Via environment variable (VANTAGE_TOKEN, FINFOCUS_VANTAGE_TOKEN)?
   - Via config file in $HOME/.finfocus/plugins/vantage/config.json?
   - Reference: Check how finfocus-plugin-aws-public handles AWS credentials
     (IAM roles, env vars, etc.).

7. **Supported Clouds**:
   - Vantage can aggregate costs from AWS, GCP, Azure, etc.
   - Does v0.1.0 support all clouds, or should we scope to specific providers?
   - How do we communicate supported providers via Supports() RPC?

8. **Per-Request Credentials (v0.7.0 Feature)**:
   - Should v0.1.0 implement VT-3.5 per-request credentials (via
     `x-finfocus-credential-*` headers)?
   - Or is v0.1.0 env-var-only, with per-request as v0.2.0 feature?
   - What should the credential name be? `vantage-token`, `token`, or
     configurable?
   - Should we validate that the token is non-empty and < 4KB?

9. **FOCUS 1.4 Support (v0.7.0 Feature)**:
   - Does Vantage CUR export invoice detail IDs or commitment metadata?
   - Should v0.1.0 implement VT-3.6 (invoice/commitment fields) or leave nil?
   - Can we reuse `pluginsdk.FocusRecordBuilder` from v0.7.0, or does the
     existing `internal/vantage/adapter/mapping.go` need significant refactoring?
   - Should FOCUS 1.4 field population be behind a feature flag (toggleable)?

10. **pluginsdk Compatibility**:
    - Should we use `pluginsdk.FocusRecordBuilder` (v0.7.0) for GetActualCost
      responses, or continue with manual struct building?
    - The builder has validation built-in; does using it cost performance or
      introduce breaking changes when upgrading to v0.8.0+?

11. **Release Cadence**:
    - After v0.1.0, what's the update frequency?
    - Will there be patch releases (v0.1.1) or move straight to v0.2.0 with new
      features?
    - Any SLOs for bug fixes or security updates?

---

## Checklist for v0.1.0 Release Readiness

- [x] Go version upgraded to 1.27.1
- [x] Module renamed to `finfocus-plugin-vantage`
- [x] finfocus-spec v0.7.0 dependency added
- [x] gRPC server replaces Cobra CLI
- [x] GetActualCost RPC implemented (unit and mocked Vantage API integration tests)
- [x] Supports() + Name() RPC implemented
- [x] trace_id metadata handling (v0.7.0 feature; structured logs include trace_id and operation)
- [x] Per-request credentials support (v0.7.0 feature, VT-3.5)
- [x] FOCUS 1.4 fields support or explicit nil (invoice/commitment data unavailable; documented)
- [x] ≥70% overall test coverage (90.2% overall; 81.6% internal/plugin)
- [x] No external API calls in unit/integration tests
- [x] Binary builds: `go build ./cmd/finfocus-plugin-vantage`
- [x] Binary runs and announces PORT: `./bin/finfocus-plugin-vantage`
- [x] All tests pass: `make test`
- [x] Linting passes: `make lint`
- [x] README updated with finfocus references
- [x] CHANGELOG.md updated (Unreleased migration notes; release entry still pending)
- [ ] Git tag v0.1.0 created
- [ ] GitHub Release published with binary artifact

---

## Estimated Effort

| Phase | ID | Effort | Est. Days |
| --- | --- | --- | --- |
| 1: Architecture | VT-1.1–1.2 | 4-6 hours | 0.5–1 |
| 2: GetActualCost | VT-2.1–2.2 | 8–12 hours | 1–1.5 |
| 3: Supporting RPCs + v0.7.0 | VT-3.1–3.6 | 8–10 hours | 1–1.25 |
| 4: Testing | VT-4.1–4.3 | 6–8 hours | 0.75–1 |
| 5: Release | VT-5.1–5.3 | 3–4 hours | 0.5 |
| **Total** | | **29–40 hours** | **3.75–4.75 days** |

---

## References

- **finfocus-spec v0.7.0**: Protobuf definitions at
  `../finfocus-spec/proto/finfocus/v1/` and SDK docs at
  `../finfocus-spec/sdk/go/pluginsdk/`
  - Per-request credentials: `pluginsdk/credentials.go`
  - FOCUS 1.4 builder: `pluginsdk.FocusRecordBuilder.WithInvoiceDetailID()`
  - Validation: `pluginsdk.ValidateActualCostResponse()`
- **finfocus-spec Changelog**: `../finfocus-spec/CHANGELOG.md` (v0.7.0 entry
  lists per-request credentials, FOCUS 1.4 fields, contract commitments)
- **finfocus-plugin-aws-public**: Reference implementation at
  `../finfocus-plugin-aws-public/` (complete plugin for comparison)
- **Vantage API Docs**: See [Vantage REST API](https://docs.vantage.sh/api)
  (public API reference)
- **finfocus Core Architecture**: `../finfocus/internal/plugin/` (how plugins
  are discovered/invoked)
- **Design Document**: `pulumi_cost_vantage_adapter_design_draft_v_0.md` (OLD;
  needs update post-architecture-change)

## Acceptance work completed after the dependency merges

- VT-4.1: Added versioned mock response and expected mapping fixtures. Current
  race-tested coverage is above the task's plugin and client requirements.
- VT-4.2: The mock API test now crosses the actual SDK gRPC server and its
  credential middleware. It verifies bearer authentication, documented query
  parameters, 429 recovery, two-page retrieval, and the golden mapped results.
- VT-3.5: Real gRPC tests verify per-request credential metadata, missing and
  oversized credential errors, and the advertised credential capability.
- VT-1.3 / VT-5.1: A subprocess test builds the executable, checks `--version`,
  connects to its announced port, queries Name/GetPluginInfo, and sends SIGTERM.
- VT-4.3: FinFocus core successfully launches the plugin and retrieves USD 6.00
  from a local mock Vantage API for a Pulumi state resource. This acceptance test
  is enabled with `FINFOCUS_CORE_BINARY`; live Vantage validation is separate.
