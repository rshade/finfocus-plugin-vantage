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

Configuration fixed in `b3150ec`: plain tags and initial version are explicit; regression tests reject
missing keys and a premature manifest version. The manifest remains `0.0.0` until publication.
This repository runs GoReleaser on tag pushes.

## Release ownership correction

Release-please owns release creation and manifest updates. The agent mistakenly
created the plain `v0.1.0` tag and triggered publication directly at `292dc65`.
The subsequent archive repair also finished before the user's interruption.
The temporary repair workflow has been removed and the manifest restored to
`0.0.0` because no release-please release PR has merged. The existing tag and
release have not been deleted or moved. Their disposition belongs to the owner.

VT-5.2 remains pending through the intended release-please process. Its workflow
currently fails because `RELEASE_PLEASE_TOKEN` is not configured. Do not bypass
release-please by creating tags, releases, or manually advancing its manifest.

## Scope and status

This plan covers the first **gRPC actual-cost plugin** release. The old sync
CLI, adapters, sinks, and WireMock deployment have been removed. Their issue
bodies describe broader legacy features and are not evidence that those
features shipped. This ledger records implemented behavior and executable
acceptance checks rather than placeholder file names or obsolete protobufs.

Dependencies: Go 1.27.1, finfocus-spec v0.7.5, vantage-go v0.1.14. gRPC is
pinned to the patched commit for GO-2026-6443 pending a stable patched release.

### Phase 0: Official client

- [x] VT-0.2 — Retire the unused hand-written client and sync adapter.
  Production API calls use `internal/vantageapi`; its mock tests exercise
  generated-client serialization, headers, pagination links, and status errors.

### Phase 1: Architecture

- [x] VT-1.1 — Rename module, executable, and build artifacts to FinFocus.
  `go build ./...` and `make build` succeed; CI uploads the correct filenames.
- [x] VT-1.2 — Upgrade Go and FinFocus spec. Current descriptor, FOCUS, and
  per-request credential interfaces compile. `go mod verify` validates modules.
- [x] VT-1.3 — Replace Cobra with `pluginsdk.Serve`. The executable lifecycle
  test verifies version, `PORT` within two seconds, RPC discovery, and graceful
  SIGTERM shutdown.

### Phase 2: Actual costs

- [x] VT-2.1 — Request/response mapping documented in `docs/MAPPING.md` and
  implemented in `internal/plugin/plugin.go`. Tests cover empty data, invalid
  queries, authentication, throttling, cancellation, malformed amounts,
  pagination, currency, and date boundaries. Current protobuf responses return
  daily cost rows, not the retired CLI's monthly pricing fields.
- [x] VT-2.2 — Actual-cost implementation validated offline. Queries use
  `costs.*` VQL dimensions, provider/service/resource selectors, escaped cloud
  tag tuples, billed settings, and UTC daily windows. Transient errors retry
  with pacing and cancellation; response fields use source data.
- [ ] VT-2.2 live acceptance — Requires a configured Vantage account with an
  imported resource interval. `TestLiveVantageActualCost` exists and is opt-in;
  no live query was performed. Offline success is not live validation.

### Phase 3: RPC support

- [x] VT-3.1 — `Supports` discovers AWS, Azure, GCP, Kubernetes, and custom
  provider categories. Retrieval requires a Vantage report containing the
  resource and a resolvable exact service. The plugin name is `vantage`; cloud
  provider selection uses the actual cloud rather than the plugin name.
- [x] VT-3.2 — `Name` returns `vantage`; tested through the executable's RPC.
- [x] VT-3.3 — Optional pricing spec, projected cost, and estimate methods
  explicitly return `Unimplemented`.
- [x] VT-3.4 — Structured logs include trace ID and operation without tokens.
- [x] VT-3.5 — Real SDK gRPC integration verifies per-request credentials
  override environment credentials, metadata advertises the capability, and
  missing/oversized credentials receive appropriate status codes.
- [x] VT-3.6 — FOCUS source availability documented in
  `docs/FOCUS_1_4_MAPPING.md`. Validated records are optional; missing invoice
  detail and commitment data remain empty. Mixed currencies are rejected;
  non-USD legacy-only output is rejected to prevent silent USD attribution.

### Phase 4: Acceptance tests

- [x] VT-4.1 — Unit scenarios and golden row fixtures cover mapping, errors,
  service inference, tags, usage, billing account overrides, and pagination.
  Latest race coverage: **84.5% overall, 90.1% API wrapper, 83.8% plugin**.
  `make test-coverage` and CI enforce 70%, 80%, and 80% respectively.
- [x] VT-4.2 — Real SDK server plus mocked Vantage HTTP API exercises auth,
  query parameters, retry, multiple pages, and exact golden output. Default
  tests need no Vantage credentials or external API calls.
- [x] VT-4.3 — Installed FinFocus core launches the plugin against a mock API
  with Pulumi state and returns the expected USD cost. Run this optional test
  with `FINFOCUS_CORE_BINARY`; it passed locally during acceptance.
- [ ] VT-4.3 live end-to-end — Not run; requires the same external account
  setup as VT-2.2 live acceptance. Optional for v0.1.0.

### Phase 5: Release

- [x] VT-5.1 — Correct binary name and injected version; GoReleaser config
  validates. Linux/macOS/Windows amd64 and arm64 archives include documentation
  and SHA-256 checksums.
- [ ] VT-5.2 — Publish plain `v0.1.0`, obtain a successful GoReleaser run, and
  verify downloadable artifacts, checksum, version, and startup.
- [x] VT-5.3 documentation — README and usage/API guides match current RPCs,
  required service selectors, pagination, credentials, and limitations.
- [ ] VT-5.3 installer acceptance — Install the published archive into an
  isolated FinFocus home and check discovery/version.

## Reproducible checks

```sh
make build
make test-coverage
make lint
make govulncheck
go mod verify
npm run markdownlint
FINFOCUS_CORE_BINARY=/absolute/path/to/finfocus \
  go test -race ./test/integration -run TestPluginBinary -v
```

Release configuration regression checks also reject either missing required
key and a manifest claiming v0.1.0 before initial publication. These mutations
were exercised and restored before committing.

## Remaining work after v0.1.0

Live account acceptance is explicitly outstanding until external credentials
and a known resource interval are configured. See `docs/USAGE.md#validation`.

The original plan deferred example configuration expansion (#25), deployment
operations (#29), diagnostics (#31), broader automation (#33), repository
hardening (#39), and the ongoing dependency dashboard (#43). Those are not
part of the initial actual-cost implementation. The legacy CLI demo (#38) was
removed; the real core mock test now serves as the executable usage example.

Original issues with legacy sync/circuit-breaker/idempotency criteria remain
open unless their entire acceptance criteria have been satisfied. Release
success does not imply those historical features are implemented.

## Implementation evidence

- `e6beba2`: valid resource VQL, service selection, retry/pacing, redaction,
  currency/date/account checks, and query/API error regression tests.
- `1c49346`: SDK gRPC golden tests, all-page default queries, binary lifecycle,
  core mock acceptance, and opt-in live test.
- `b3150ec`: mandatory coverage/security gates, corrected artifact uploads,
  and first-release configuration regression checks.
