# Changelog

## Unreleased

## 0.1.0

### Added

- FinFocus SDK gRPC server with actual cost retrieval and provider discovery.
- Current resource descriptors and compatibility with legacy routing tags.
- Required resource/service VQL filtering and AWS service inference.
- Environment and per-request Vantage credentials with redacted errors.
- Daily billed-cost mapping, usage mapping, and validated FOCUS records when
  source fields are sufficient.
- Default retrieval across all pages and explicit caller pagination.
- Shared request pacing, bounded transient retries, and cancellation support.
- Golden fixtures, SDK gRPC integration, binary lifecycle tests, and an
  optional FinFocus core test against the mock API.
- Six platform archives and SHA-256 checksums through GoReleaser.

### Security

- Patched gRPC transport dependency for GO-2026-6443 and mandatory scanning.
- HTTPS API transport, loopback-only HTTP exception, and disabled redirects.

### Limitations

- Actual imported costs only; projected costs and pricing are unavailable.
- Invoice detail and commitment fields are unavailable from the source API.
- Mixed currencies are rejected; non-USD rows require valid FOCUS fields.
- Live Vantage account validation is opt-in and separate from offline tests.

The prior 2024 entry described the retired sync CLI draft, not a published
FinFocus release. This is the first tagged gRPC plugin release.
