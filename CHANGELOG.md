# Changelog

## Unreleased

## [0.1.1](https://github.com/rshade/finfocus-plugin-vantage/compare/v0.1.0...v0.1.1) (2026-10-06)


### Bug Fixes

* **release:** accept release-please bumps, skip generated changelog lint ([#121](https://github.com/rshade/finfocus-plugin-vantage/issues/121)) ([5da035b](https://github.com/rshade/finfocus-plugin-vantage/commit/5da035bf002ee7629629510597db863fccfed81b))
* **release:** follow the family release pattern so v0.1.1 ships correct archives ([#118](https://github.com/rshade/finfocus-plugin-vantage/issues/118)) ([87e6eca](https://github.com/rshade/finfocus-plugin-vantage/commit/87e6eca6a490250d5e3e3a2ff9fd515aa6e85b64))
* **release:** match archive names used by URL installation ([084d826](https://github.com/rshade/finfocus-plugin-vantage/commit/084d826434a88bdd915f528ed76877ca6ed8c0dc))
* restore release-please ownership of publication ([235dac8](https://github.com/rshade/finfocus-plugin-vantage/commit/235dac8fbc2cee13c9eba74de38980b6920acff7)), closes [#27](https://github.com/rshade/finfocus-plugin-vantage/issues/27)

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
