# FinFocus — Vantage Adapter Design (Draft v0.1)

> Target: FinFocus Core v0.x + FOCUS 1.2 alignment. Owner: Richard. Status: Draft for review.

---

## 1) Summary
Design and implement a **Vantage** cost-source adapter that ingests normalized cost/usage data from Vantage’s REST API and maps it into FinFocus’s internal schema with **FinOps FOCUS 1.2** fields. The adapter supports historical backfills, daily incrementals, tag/label dimensions, and forecast snapshots.

---

## 2) Goals & Non‑Goals
**Goals**
- Fetch costs via **/costs** using a Cost Report token or Workspace token.
- Support **daily granularity**; **group by** common dimensions (account/project/service/resource/region/provider/tags).
- Capture **list, net, and amortized** costs when present; include taxes, credits, refunds if exposed.
- Expose **metadata**: source report, query params, currency, generation time.
- Implement **incremental sync** with bookmarks and backoff on rate limits.
- Emit FOCUS 1.2 compatible records.

**Non‑Goals**
- Managing Vantage resources (creating reports/folders/dashboards) beyond what’s needed for read.
- Direct optimization recommendations (handled by FinFocus analyzers).

---

## 3) High‑Level Flow
1. **Auth**: API token (service or user) via `Authorization: Bearer <token>`.
2. **Discovery** (optional): enumerate workspaces and find the target **Cost Report** by name or token.
3. **Ingest**: call `/costs` with date range, granularity, group_bys, and metrics.
4. **Normalize**: map response rows → FinFocus `CostRecord` (FOCUS 1.2 fields included).
5. **Persist**: write to FinFocus data store (parquet/DB) with idempotency keys.
6. **Bookmarks**: store `last_successful_end_date` per (workspace, report_token, filter hash).
7. **Emit**: optional **forecast** via `/cost_reports/{token}/forecast` and weekly snapshot.

---

## 4) Configuration (finfocus‑plugin‑vantage)
```yaml
version: 0.1
source: vantage
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}
params:
  workspace_token: "ws_..."         # optional if using cost_report_token
  cost_report_token: "cr_..."       # preferred for stable queries
  start_date: "2024-01-01"          # ISO date, default: 12 months back
  end_date: null                     # default: today
  granularity: "day"                # day|month
  group_bys: ["provider","service","account","project","region","resource_id","tags"]
  metrics: ["cost","usage","effective_unit_price"]
  include_forecast: true
  tag_prefix_filters: ["user:","kubernetes.io/"]
  request_timeout_seconds: 60
  page_size: 5000
  max_retries: 5
```

---

## 5) Endpoint Usage
- **GET /costs**
  - Query params: `workspace_token|cost_report_token`, `start_at`, `end_at`, `granularity`, `group_bys[]`, `metrics[]`, pagination cursors.
- **GET /cost_reports/{token}/forecast** (optional)
- **POST /cost_reports/{token}/exports** (optional CSV backfills for very large ranges)
- **GET /workspaces** (optional discovery)

> Adapter prefers **Cost Report token** for stable/curated filters; falls back to Workspace + VQL filter when needed.

---

## 6) Schema Mapping (FinFocus ⟷ Vantage ⟷ FOCUS 1.2)
| FinFocus Field | Vantage Source | FOCUS 1.2 Field |
|---|---|---|
| `timestamp` | bucket start (daily) | `usage_start_time` |
| `provider` | `provider` dim | `cloud_provider` |
| `service` | `service` dim | `service_name` |
| `account_id` | `account` dim | `billing_account_id` |
| `subscription_id` | provider‑specific | `subscription_id` |
| `project` | `project` dim | `project_id`/`project_name` |
| `region` | `region` dim | `region` |
| `resource_id` | `resource_id` dim | `resource_id` |
| `labels` | tags map | `labels` |
| `usage_amount` | `usage_quantity` | `usage_amount` |
| `usage_unit` | `usage_unit` | `usage_unit` |
| `list_cost` | `list_cost` (if present) | `list_cost` |
| `net_cost` | `cost` | `net_cost` |
| `amortized_cost` | `amortized_cost` (if present) | `amortized_net_amortized_cost` |
| `tax_cost` | `tax` (if separate) | `taxes` |
| `credit_amount` | `credit` | `credits` |
| `refund_amount` | `refund` | `refunds` |
| `currency` | `currency` | `billing_currency` |
| `source_report_token` | config | `source_id` |
| `query_hash` | params hash | `line_item_id` (adapter idempotency) |

> Note: actual availability of fields varies by provider and Vantage configuration. Adapter fills missing fields with `null` and flags in `diagnostics`.

---

## 7) Tag & Dimension Strategy
- Normalize tag keys to lowercase kebab (configurable), preserve originals in `labels_raw`.
- Merge **provider native tags** and **Kubernetes labels** when present.
- Optional tag allow/deny lists; drop high‑cardinality keys by regex (e.g., pod UID).

---

## 8) Pagination, Limits, Retries
- Use server cursors; page size configurable.
- Exponential backoff on HTTP 429/5xx with jitter.
- Respect `X-RateLimit-*` headers when provided; sleep until reset.

---

## 9) Incremental & Backfill
- **Backfill**: chunk by month to limit payload size; fall back to CSV export for > 12 months.
- **Incremental**: daily sync runs `D-3 → D-1` to catch late postings; configurable lag window.
- **Idempotency**: key on `(workspace|report_token, date, dims, metrics)`.

---

## 10) Forecast Support (Optional)
- Store `/forecast` result as a **separate metric family** with `metric_type = forecast`.
- Snapshot weekly; keep last 8 snapshots for MAPE evaluation.

---

## 11) Error & Observability
- Structured errors with `adapter=vantage`, `operation`, `attempt`, `correlation_id`.
- Emit counters: rows_ingested, pages, bytes, http_codes, throttles, retries, duration.
- Toggle verbose logging of raw pages for troubleshooting (PII‑safe).

---

## 12) Security & Secrets
- Token provided via env var/secret store; never logged.
- Optional proxy support.
- Per‑customer workspace/report scoping to enforce least privilege.

---

## 13) Minimal Interface (Go)
```go
// Package: finfocus/adapters/vantage

type Config struct {
    Token            string
    WorkspaceToken   string
    CostReportToken  string
    StartDate        time.Time
    EndDate          *time.Time
    Granularity      string   // "day"|"month"
    GroupBys         []string // provider,service,account,project,region,resource_id,tags
    Metrics          []string // cost, usage, effective_unit_price, amortized_cost, taxes, credits
    IncludeForecast  bool
    PageSize         int
    Timeout          time.Duration
    MaxRetries       int
}

type Client interface {
    Costs(ctx context.Context, q Query) (Page, error)
    Forecast(ctx context.Context, reportToken string, q ForecastQuery) (Forecast, error)
}

func (a *Adapter) Sync(ctx context.Context, cfg Config, sink Sink) error
```

---

## 14) Test Plan
- **Unit**: param building, pagination, mapping (golden JSON → records).
- **Contract**: run against a mocked Vantage server (wiremock) + a live smoke test (skipped in CI without `LIVE=1`).
- **Performance**: backfill 12 months x 6 dims; assert < N minutes and < M GiB RAM on sample data.

---

## 15) Rollout
1. Implement read‑only costs path.
2. Add forecast snapshots.
3. Add CSV export path for large backfills.
4. Harden with retries and late‑posting lag window.
5. Ship docs + example config + `finfocus pull --source=vantage`.

---

## 16) Open Questions
- Confirm availability of **amortized** vs **net** per provider and whether Vantage separates **taxes/credits/refunds** consistently by row.
- Preferred handling for **enterprise discounts** and **SP/RI** allocations if surfaced.
- Do we need VQL filter support in v1 or rely exclusively on curated Cost Reports?

---

## 17) Appendix — Example Request
```
GET /costs?cost_report_token=cr_XXXX&start_at=2025-08-01&end_at=2025-08-31&granularity=day&group_bys[]=provider&group_bys[]=service&group_bys[]=region&group_bys[]=tags&metrics[]=cost&metrics[]=usage&metrics[]=effective_unit_price&page_size=5000
Authorization: Bearer *****
```

**Notes**: Adapter records `query_hash` of all params for traceability.

---

## 18) Future Work
- Enrich with **Kubernetes allocation** heuristics (namespace/workload) when available in tags.
- Add **SaaS connectors** coverage matrix (Fastly, Databricks, GitHub, ClickHouse Cloud) sourced via Vantage.
- Optional **real‑time** pull using shorter windows when rate limits allow.


---

## 19) Build Plan with OpenCode v0.15.3+ and GrokZeroFree

This section adapts the design for fully automated implementation using **OpenCode v0.15.3+** orchestrating the **GrokZeroFree** model. It defines project structure, prompt packs, code‑gen workflow, CI, and acceptance criteria so the adapter can be taken from zero → release with minimal manual edits.

### 19.1 Toolchain & Versions
- **Language:** Go 1.22+
- **Build:** `make`, `golangci-lint` (v1.60+), `go test`
- **Mocks:** `wiremock/wiremock:3` (Docker)
- **Packaging:** modules per repo; semantic import paths
- **OpenCode:** v0.15.3+ with **Projects**, **Runners**, **Pipelines**, **Guardrails**
- **Model:** **GrokZeroFree**
  - Suggested limits for prompts: ≤ 16k tokens input, ≤ 4k output
  - Constrain steps to deterministic templates; avoid chatter in code blocks
  - Use JSON‑only tool responses where indicated

### 19.2 Repository Layout (multi‑repo)
- `finfocus/` (existing)
  - `internal/focus/` — FOCUS 1.2 types and helpers
  - `pkg/ingest/` — Sink interface + parquet/DB writers
- `finfocus-plugin-vantage/` (new)
  - `cmd/finfocus-vantage/` — CLI entry (`pull`, `backfill`, `forecast`)
  - `internal/vantage/client/` — REST client + auth
  - `internal/vantage/adapter/` — mapping & sync logic
  - `internal/vantage/contracts/` — golden samples, fixtures
  - `test/wiremock/` — mappings + recordings
  - `docs/` — README, config, troubleshooting
  - `Makefile`, `.golangci.yml`, `Dockerfile`

### 19.3 OpenCode Project Definition
Create `opencode.project.yaml` in `finfocus-plugin-vantage/`:
```yaml
version: 0.15
project: finfocus-plugin-vantage
model: GrokZeroFree
concurrency: 3
runners:
  - id: bootstrap
    entry: prompts/bootstrap.md
  - id: client
    entry: prompts/client.md
  - id: adapter
    entry: prompts/adapter.md
  - id: tests
    entry: prompts/tests.md
  - id: docs
    entry: prompts/docs.md
pipelines:
  - name: full_build
    steps: [bootstrap, client, adapter, tests, docs]
  - name: fast_iter
    steps: [adapter, tests]
artifacts:
  out_dir: .
  allow_overwrite: false
_guardrails:
  - type: regex_block
    pattern: "(?i)hardcoded\s*token|Authorization:\s*Bearer\s+[A-Za-z0-9._-]+"
  - type: file_glob_block
    globs: ["**/*.pem", "**/.env"]
```

### 19.4 Prompt Pack (role‑split)
**prompts/bootstrap.md** — scaffolding & repo hygiene
```
SYSTEM: You are a senior Go engineer. Generate minimal, buildable scaffolds only.
GOALS: Create module, Makefile, lint config, CLI skeleton, config types, and env wiring.
CONSTRAINTS: No example secrets. All code must compile. Add package comments.
OUTPUT: Write files. Do not include prose outside code fences when generating multiple files.
TASKS:
1) go mod init github.com/FinFocus/finfocus-plugin-vantage
2) Create Makefile with targets: build, test, lint, wiremock-up, wiremock-down, fmt.
3) Create cmd/finfocus-vantage/main.go with Cobra CLI: commands pull, backfill, forecast.
4) Create internal/vantage/adapter/config.go with Config struct (see Section 13).
5) Create .golangci.yml with revive, govet, gocyclo, gofmt.
6) Create docs/CONFIG.md with YAML example from Section 4.
```

**prompts/client.md** — HTTP client + pagination
```
SYSTEM: Implement a resilient Vantage REST client in Go.
INPUTS: Section 5 (Endpoint Usage), Section 8 (Retries), Section 11 (Observability).
OUTPUT: Files under internal/vantage/client/ with unit tests.
REQUIREMENTS:
- Interface: Costs(ctx, q) and Forecast(ctx, token, q) as in Section 13.
- HTTP: context timeouts, retry/backoff on 429/5xx, page cursors.
- Rate limits: honor X-RateLimit headers when present.
- Logging: structured (adapter=vantage, op, attempt) behind an interface.
- Coverage: ≥80% for client package.
```

**prompts/adapter.md** — mapping & sync
```
SYSTEM: Implement adapter mapping to FinFocus FOCUS 1.2.
INPUTS: Section 6 (Schema Mapping), Section 7 (Tags), Section 9 (Incremental).
OUTPUT: internal/vantage/adapter/{adapter.go,mapping.go,normalize.go}
REQUIREMENTS:
- Map Vantage rows → CostRecord with FOCUS 1.2 fields.
- Tag normalization: lower-kebab, allowlist/denylist, raw preservation.
- Idempotency: deterministic key across dims/metrics/date.
- Incremental sync: D-3 to D-1 lag window, bookmarks persisted via Sink.
- Forecast snapshot path (separate metric_type).
```

**prompts/tests.md** — contract + golden tests
```
SYSTEM: Create tests using Wiremock + golden JSON fixtures.
OUTPUT: test files + docker-compose or Makefile targets.
REQUIREMENTS:
- Golden samples in internal/vantage/contracts/ with representative fields.
- Wiremock mappings for /costs pagination and /forecast.
- Contract tests asserting mapping correctness and idempotency.
```

**prompts/docs.md** — user docs
```
SYSTEM: Generate README.md and docs/CONFIG.md updates.
REQUIREMENTS: Quickstart, config, auth, examples, troubleshooting, FAQ.
```

### 19.5 Acceptance Criteria (AC)
1. `make build` produces `finfocus-vantage` binary (Linux/macOS).
2. `make test` passes with ≥85% coverage on `client` and ≥70% overall.
3. `finfocus-vantage pull --config ./config.yaml` runs against Wiremock and writes N>0 records via a file sink.
4. Mapping aligns with Section 6; missing fields are null + diagnostic flag.
5. Incremental sync honors D-3→D-1 lag and updates bookmarks.
6. Forecast snapshots stored as separate metric family.

### 19.6 CI/CD (GitHub Actions)
- Workflows: `ci.yml` (lint+test), `contract.yml` (wiremock matrix), `release.yml` (tag → build artifacts)
- Caching: Go modules, build cache
- Artifacts: binary + SBOM (`go version -m` + `cyclonedx-gomod` optional)

### 19.7 Security & Secrets
- Read token from `FINFOCUS_VANTAGE_TOKEN` or `.env` via `direnv` (dev only)
- No tokens in logs; redaction middleware
- Least‑privilege: cost‑report token preferred

### 19.8 Developer UX
- `make wiremock-up` brings up mock server and seeds mappings
- `make demo` runs a pull against mocks and prints first 10 records
- `make fmt` and `make lint` enforce style

### 19.9 Sample CLI Contracts
```
# Backfill last 12 months
finfocus-vantage backfill --config ./config.yaml --months 12

# Daily incremental (to be cron’d)
finfocus-vantage pull --config ./config.yaml

# Forecast snapshot
finfocus-vantage forecast --config ./config.yaml --out ./data/forecast.json
```

### 19.10 Migration/Extensibility
- Add providers or SaaS datasets exposed by Vantage by extending mapping tables.
- Introduce parquet sink by implementing `Sink` interface in `finfocus`.

### 19.11 Known Model Constraints (GrokZeroFree)
- Prefer smaller, iterative prompts (client → adapter → tests) to avoid context overflow.
- Use deterministic templates; avoid streaming multi‑file diffs—emit full files per step.
- If output truncates, split files (e.g., `adapter_part1.go`, then merge with OpenCode `combine` op).

### 19.12 Release Checklist
- [ ] CI green on main
- [ ] README Quickstart verified end‑to‑end
- [ ] Version tag `v0.1.0`
- [ ] CHANGELOG entry with features and caveats
- [ ] Sample config + mock recordings published in `docs/`


---

## 20) Prompt Files (copy‑paste ready)

> Place these under `finfocus-plugin-vantage/prompts/`. Each prompt is self‑contained and tuned for **OpenCode v0.15.3+** with **GrokZeroFree**. They instruct the model to emit files in multi‑file format blocks.

### prompts/bootstrap.md
```
SYSTEM
You are a senior Go engineer. Generate minimal, buildable scaffolds only. Follow repo hygiene and keep code deterministic.

GUARDRAILS
- Never print or hardcode secrets.
- All code must compile on Go 1.22+.
- Add package comments to every new package.
- Emit files using the exact multi‑file format:
  ```path/to/file
  <contents>
  ```
- Do not include explanations outside file blocks.

CONTEXT
We are creating a new repo `finfocus-plugin-vantage` that implements a Vantage adapter for FinFocus. Reference design Sections 4, 5, 8, 11, 13, 19.

TASKS
1) Initialize module: `github.com/FinFocus/finfocus-plugin-vantage`.
2) Create Makefile with targets: `build`, `test`, `lint`, `fmt`, `wiremock-up`, `wiremock-down`, `demo`.
3) Create `.golangci.yml` with revive, govet, gocyclo, gofmt, goimports rules.
4) Create Cobra CLI skeleton in `cmd/finfocus-vantage/main.go` with commands: `pull`, `backfill`, `forecast`.
5) Create config types in `internal/vantage/adapter/config.go` (mirror Section 13, with yaml tags).
6) Create docs/CONFIG.md populated from Section 4 (example YAML + notes).
7) Create `README.md` (short) linking to docs/CONFIG.md and usage examples.

OUTPUT
Generate the necessary files using multi‑file blocks only. Keep code small and compiling; stub unimplemented behavior with TODOs.
```

### prompts/client.md
```
SYSTEM
You are implementing a resilient HTTP client for Vantage’s REST API in Go.

GUARDRAILS
- No secrets in code or logs; redact `Authorization` header.
- Respect context timeouts and cancellations.
- Retries with exponential backoff + jitter on 429/5xx.
- Honor `X-RateLimit-*` headers when present.
- ≥80% coverage for the `client` package.
- Emit files using the multi‑file format; no prose.

INPUTS
Use the design’s Section 5 (Endpoint Usage), Section 8 (Pagination/Retry), Section 11 (Observability), Section 13 (Interfaces), and 19.5 (AC).

TASKS
1) Create package `internal/vantage/client` with:
   - `client.go`: public `Client` interface and constructor `New`.
   - `http.go`: low‑level HTTP, backoff, rate‑limit handling, redact logs.
   - `models.go`: request/response structs for `/costs` and `/forecast`.
   - `pager.go`: cursor pagination helpers.
   - `logger.go`: minimal interface used by adapter; default no‑op.
2) Implement `Costs(ctx, q)` (cursor pagination) and `Forecast(ctx, token, q)`.
3) Unit tests under `internal/vantage/client/` using `httptest`.
4) Add small examples in `_test.go` demonstrating usage.

OUTPUT
Emit all files and tests with multi‑file blocks only.
```

### prompts/adapter.md
```
SYSTEM
You are implementing the Vantage adapter mapping and sync pipeline for FinFocus in Go.

GUARDRAILS
- Deterministic mapping; no randomization.
- Missing fields become `nil` and add a diagnostic note.
- Tag normalization: lower‑kebab; keep raw copy.
- Idempotency key is stable across dims/date/metrics.
- Emit files only; no prose.

INPUTS
Use the design’s Sections 6 (Schema Mapping), 7 (Tag Strategy), 9 (Incremental & Backfill), 10 (Forecast), 13 (Interfaces), 19.5 (AC).

TASKS
1) Create package `internal/vantage/adapter`:
   - `adapter.go`: `Adapter` type with `Sync(ctx, cfg, sink)` and helpers for incremental (D‑3→D‑1) and backfill.
   - `mapping.go`: Vantage row → FinFocus `CostRecord` + FOCUS 1.2 fields.
   - `normalize.go`: tag normalization, allow/deny filters, label merging.
   - `config.go`: (if not present) `Config` struct with yaml/json tags.
   - `diagnostics.go`: lightweight diag struct + toggles.
2) Expose forecast snapshot path storing records with `metric_type="forecast"`.
3) Persist bookmarks through provided `Sink` interface (assume exists in `finfocus`).
4) Include unit tests covering mapping, tag normalization, and idempotency.

OUTPUT
Emit all files and tests with multi‑file blocks only.
```

### prompts/tests.md
```
SYSTEM
Create contract tests using Wiremock and golden fixtures.

GUARDRAILS
- No external network calls; use Wiremock.
- Put fixtures under `internal/vantage/contracts/`.
- Deterministic assertions (no time.Now() in golden content).
- Emit files only; no prose.

INPUTS
Use design Sections 5, 6, 8, 9, 10, 14, and 19.5.

TASKS
1) Add `test/wiremock/` with Docker compose or Make targets to run `wiremock/wiremock:3`.
2) Create Wiremock mappings for `/costs` (2‑3 pages) and `/forecast`.
3) Create golden JSON inputs and expected `CostRecord` arrays.
4) Write Go tests under `internal/vantage/adapter/` and `internal/vantage/client/` that:
   - spin up Wiremock (via Make),
   - hit mock endpoints,
   - compare adapter output to goldens,
   - assert idempotency (same inputs → same keys).
5) Add `make wiremock-up`/`wiremock-down` and `make demo` targets if missing.

OUTPUT
Emit files (mappings, fixtures, tests) with multi‑file blocks only.
```

### prompts/docs.md
```
SYSTEM
Generate concise user docs for the plugin.

GUARDRAILS
- No secrets.
- Keep examples accurate and runnable with mocks.
- Emit files only; no prose.

INPUTS
Use Sections 4, 5, 9, 10, 15, 19.5–19.12.

TASKS
1) `README.md`: Overview, Quickstart, features, limitations, and links.
2) `docs/CONFIG.md`: Detailed config reference with YAML, env vars, and notes.
3) `docs/TROUBLESHOOTING.md`: Common errors (auth, 429s, pagination), how to enable verbose logs, how to capture mock recordings.
4) `docs/FORECAST.md`: How forecast snapshots work and where they’re written.

OUTPUT
Emit the documentation files with multi‑file blocks only.
```

