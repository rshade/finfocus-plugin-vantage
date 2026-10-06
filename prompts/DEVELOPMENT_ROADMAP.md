# FinFocus Vantage Plugin - Development Roadmap Summary

**Status:** ✅ Complete - 31 GitHub issues created
**Target Release:** v0.1.0 (January 2026)
**Total Estimated Effort:** 90-110 hours (~11-14 developer weeks)

---

## Overview

This document summarizes the complete development roadmap for the
FinFocus Vantage Plugin v0.1.0. The roadmap has been converted into
31 actionable GitHub issues organized across 8 phases.

All issues are in the GitHub repository and ready for implementation.

---

## Issue Organization

### Phase 1: Project Bootstrap ✅ (COMPLETE)

**Status:** Done - All scaffolding complete

### Phase 2: CLI & Configuration (3 issues: #3-#5)

**Target:** Oct 30, 2025

- **#3**: 1.1 - Create Cobra CLI Bootstrap (L, 4-6h)
- **#4**: 1.2 - Implement Config Types & YAML Parsing (M, 2-3h)
- **#5**: 1.3 - Create CONFIG.md Documentation (S, 1-2h)

### Phase 3: REST Client Implementation (5 issues: #6-#10)

**Target:** Nov 13, 2025

- **#6**: 2.1 - Create HTTP Client with Auth & Models (L, 5-7h)
- **#7**: 2.2 - Implement Pagination & Cursor Handling (M, 3-4h)
- **#8**: 2.3 - Add Retry Logic & Rate Limit Handling (M, 3-4h)
- **#9**: 2.4 - Implement Forecast Endpoint (S, 2-3h)
- **#10**: 2.5 - Create Logger Interface & Redaction (S, 2h)

### Phase 4: Adapter Core Mapping (4 issues: #11-#12, #30-#31)

**Target:** Nov 27, 2025

- **#11**: 3.1 - Implement Vantage → FOCUS 1.2 Schema Mapping (L, 6-8h)
- **#12**: 3.2 - Implement Tag Normalization & Filtering (M, 4-5h)
- **#30**: 3.3 - Generate Idempotency Keys (S, 1-2h)
- **#31**: 3.4 - Add Diagnostics & Missing Field Tracking (S, 2h)

### Phase 5: Incremental & Backfill Sync (4 issues: #13-#16)

**Target:** Dec 4, 2025

- **#13**: 4.1 - Implement Incremental Sync Logic (L, 6-8h)
- **#14**: 4.2 - Implement Backfill Logic (M, 4-5h)
- **#15**: 4.3 - Implement Bookmark Persistence (M, 3-4h)
- **#16**: 4.4 - Add Sync Error Recovery & Retries (M, 4h)

### Phase 6: Forecast & Snapshots (1 issue: #17)

**Target:** Dec 10, 2025

- **#17**: 5.1 - Implement Forecast Snapshot Storage (M, 4h)

### Phase 7: Testing & Quality (5 issues: #18-#22)

**Target:** Dec 13, 2025

- **#18**: 6.1 - Create Wiremock Contract Test Setup (M, 3-4h)
- **#19**: 6.2 - Implement Contract Tests (M, 4h)
- **#20**: 6.3 - Implement Golden Fixture Tests for Mapping (L, 6h)
- **#21**: 6.4 - Validate Coverage Requirements (S, 1-2h)
- **#22**: 6.5 - Lint & Code Quality (S, 1h)

### Phase 8: Documentation (4 issues: #23-#26)

**Target:** Dec 20, 2025

- **#23**: 7.1 - Create TROUBLESHOOTING.md (S, 2h)
- **#24**: 7.2 - Create FORECAST.md (S, 1-2h)
- **#25**: 7.3 - Create Example Configs (S, 1-2h)
- **#26**: 7.4 - Create CHANGELOG.md (S, 1h)

### Phase 9: Release & Polish (3 issues: #27-#29)

**Target:** Dec 20, 2025

- **#27**: 8.1 - Create GitHub Release (S, 30m)
- **#28**: 8.2 - Verify End-to-End with Mocks (S, 1h)
- **#29**: 8.3 - Document Deployment & Operations (M, 2-3h)

---

## Issue Effort Distribution

| Size | Count | Hours | % of Total |
|------|-------|-------|-----------|
| **L (Large)** | 4 | 24-32 | 27% |
| **M (Medium)** | 13 | 46-58 | 53% |
| **S (Small)** | 14 | 20-20 | 20% |
| **TOTAL** | **31** | **90-110** | **100%** |

---

## Implementation Guide

### For Claude Code Sessions

Each issue includes:

- ✅ Clear acceptance criteria (checklist format)
- ✅ Dependency information
- ✅ Effort estimate
- ✅ Design section references
- ✅ Prompt file references

**Workflow:**

1. Pick an issue from GitHub
2. Read the issue body for acceptance criteria
3. Review referenced prompt and design sections
4. Implement until all criteria are checked
5. Open PR, request review
6. Merge and close issue

### For OpenCode / GrokZeroFree

Each issue can be fed into OpenCode with the design document
(`pulumi_cost_vantage_adapter_design_draft_v_0.md`) and corresponding
prompt file:

- **Phase 2**: Use `prompts/bootstrap.md` (with issues #3-5)
- **Phase 3**: Use `prompts/client.md` (with issues #6-10)
- **Phase 4**: Use `prompts/adapter.md` (with issues #11-12, #30-31)
- **Phase 5**: Continue with `prompts/adapter.md` (issues #13-16)
- **Phase 6**: Add to `prompts/adapter.md` (issue #17)
- **Phase 7**: Use `prompts/tests.md` (with issues #18-22)
- **Phase 8**: Use `prompts/docs.md` (with issues #23-26)
- **Phase 9**: Manual release process (issues #27-29)

---

## Critical Path

The implementation must follow this order:

```text
Phase 1 ✅ (bootstrap complete)
    ↓
Phase 2 (CLI must be ready first)
    ↓
Phase 3 (Client needed before adapter)
    ↓
Phase 4 (Adapter mapping core)
    ↓
Phase 5 (Sync logic depends on 3 + 4)
    ↓
Phase 6 (Forecast needs 3 + 5)
    ↓
Phase 7 (Testing can start after 3)
    ↓
Phase 8 (Docs after all features)
    ↓
Phase 9 (Release final)
```

---

## Success Criteria

- ✅ All 31 issues closed
- ✅ ≥70% overall test coverage
- ✅ ≥80% client package coverage
- ✅ 0 linting errors
- ✅ 0 security vulnerabilities
- ✅ Wiremock contract tests passing
- ✅ Golden fixture tests passing
- ✅ End-to-end smoke test passing
- ✅ All documentation complete
- ✅ GitHub release v0.1.0 published

---

## Resources

- **Design Document**: `pulumi_cost_vantage_adapter_design_draft_v_0.md`
  (all sections referenced)
- **Prompts**: `prompts/{bootstrap,client,adapter,tests,docs}.md`
- **TODO**: `TODO.md` (detailed phase breakdown)
- **CLAUDE.md**: Developer guidance for this repository
- **Makefile**: Build, test, lint targets

---

## Next Steps

1. **Start Phase 2**: Pick issue #3 and implement Cobra CLI
2. **Follow dependencies**: Each issue lists what it depends on
3. **Use the prompts**: Reference the corresponding prompt file for
   implementation guidance
4. **Test as you go**: Each issue has specific test coverage requirements
5. **Document everything**: See phases 8-9 for documentation needs

---

## Contact & Questions

All issues have:

- Acceptance criteria (testable)
- Effort estimates (planning)
- Dependencies (ordering)
- References (where to look)

For architecture questions, refer to the design document.
For code style questions, refer to `CLAUDE.md` and `AGENTS.md`.
