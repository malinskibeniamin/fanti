# Fanti codebase quality audit — 2026-07-12

## Review scope

- **Fixed point:** `origin/main@c114f25`
- **Mode:** deep whole-snapshot audit followed by complete remediation
- **Sources:** project rules, application code, protobuf contracts, migrations, Docker/Compose, CI, tests, production build, and rendered UI
- **Hats:** product/spec ✅ | engineering standards ✅ | complexity/value ✅ | adversarial ✅ | resilience ✅ | visual/design ✅ | test/performance ✅ | independent cross-family review ✅
- **Generated files:** treated as evidence only

## Executive summary

**Verdict: all reported findings fixed.** The audit found no P0, 10 P1, 10 P2, and 2 P3 issues. This PR now resolves all 22 findings with regression coverage. It corrects study scheduling and conversion persistence, closes validation and upload-limit gaps, restores reliable test/CI coverage, makes reader interactions keyboard-accessible, adds mobile WebKit coverage, removes unreachable surface area, and cuts the production build from about 22 MB to **1.10 MB**.

## Resolution summary

| Finding | Status | Resolution |
|---|---:|---|
| P1-01 — Future reviews returned as due | ✅ Fixed | Both character and word queries now enforce `due_time <= now()`; integration coverage includes future cards. |
| P1-02 — Unresolvable reader character links | ✅ Fixed | `GetCharacter` synthesizes CEDICT-only single-character resources with learning state; reader integration covers a non-curated glyph. |
| P1-03 — Empty fresh Docker deployment | ✅ Fixed | The container runs idempotent `seed --if-empty` before serving, records completed bootstrap data, and keeps seed inputs outside the persistent data volume. |
| P1-04 — Running conversions stranded after restart | ✅ Fixed | Startup reconciles interrupted jobs to a visible retryable failure; failed jobs can run again; restart behavior is integration-tested. |
| P1-05 — Arbitrary exception rewrites | ✅ Fixed | Resolutions must be one rune, match a stored exception source, and be one of its offered options; invalid/stale requests are covered. |
| P1-06 — Stale metadata after re-add | ✅ Fixed | Book upsert refreshes every derived mutable field and chapter set atomically while preserving reading progress. |
| P1-07 — Unbounded uploads/archive expansion | ✅ Fixed | Browser files cap at 64 MiB, Connect requests at 65 MiB, EPUB entries at 64 MiB, and cumulative expansion at 256 MiB. |
| P1-08 — Frontend unit suite red | ✅ Fixed | Vitest now uses a deterministic non-opaque DOM/storage environment; all 41 tests pass without external flags. |
| P1-09 — 8.99 MB cold homepage | ✅ Fixed | Removed globally loaded CJK faces and oversized logo use; production output is 1.10 MB with one 48.3 KB font and a budget regression test. |
| P1-10 — Tap-to-define unavailable by keyboard | ✅ Fixed | Tappable reader glyphs use shared semantic buttons, retain inline layout, and support focus plus Enter activation. |
| P2-01 — CI omitted frontend unit tests | ✅ Fixed | The web CI job now runs `bun run test`. |
| P2-02 — Debounced saves dropped | ✅ Fixed | Debounced callbacks expose flush/cancel; conversion start serializes and awaits the latest layout saves, while mission edits flush on unmount. |
| P2-03 — Edited chapter titles dropped | ✅ Fixed | Library chapters use edited layout titles, matching export behavior. |
| P2-04 — Empty books accepted | ✅ Fixed | Parsed inputs with no readable characters are rejected; TXT, SRT, and EPUB cases are covered. |
| P2-05 — Failed reset looked successful | ✅ Fixed | Conversion state clears only after delete succeeds. |
| P2-06 — Unreachable code/dependencies | ✅ Fixed | Removed the inventory route, orphan screens/helpers, 49 unused UI modules, and eight runtime dependencies; route tree and lockfile regenerated. |
| P2-07 — Raw controls bypassed UI seam | ✅ Fixed | Production buttons, inputs, and textareas now use shared primitives; a recursive regression test guards the boundary. |
| P2-08 — Desktop-Chromium-only E2E | ✅ Fixed | Playwright now runs desktop Chromium plus a focused mobile WebKit project in CI. |
| P2-09 — Mobile study tabs wrapped | ✅ Fixed | Tabs remain one horizontally scrollable row at narrow widths. |
| P2-10 — Daily lesson collapsed at 320 px | ✅ Fixed | The lesson action stacks full-width below content on narrow screens. |
| P3-01 — Duplicate speech helper | ✅ Fixed | All callers use `@/lib/speak`; the duplicate helper is deleted. |
| P3-02 — Oversized EPUB entries truncated | ✅ Fixed | EPUB extraction checks metadata and reads one byte beyond the boundary, returning an explicit error instead of truncated content. |

## Verification evidence

| Command/evidence | Result |
|---|---|
| `cd backend && golangci-lint run ./...` | pass; 0 issues |
| `cd backend && go test -short ./...` | pass |
| `cd backend && go test -run Integration ./...` | pass |
| `buf lint proto && buf generate && git diff --exit-code -- backend/gen web/src/gen` | pass |
| `cd web && bun run lint:fix` | pass; no fixes needed |
| `cd web && bun run type:check` | pass |
| `cd web && bun run test` | pass; 14 files, 41 tests |
| `cd web && bun run build` | pass; 1,103.5 KB emitted, one 48.3 KB font |
| `cd web && E2E_LEAN_SEED=1 bunx playwright test` | pass; 10/10 across Chromium and mobile WebKit |
| `git diff --check` | pass |
| Visual review | pass; home, study, and reader at 320 px plus desktop home |
| Docker lifecycle checks | pass in Go regression tests; full local image build could not fetch Docker Hub base-image metadata |

## Final assessment

No reported P0–P3 finding remains open. The only local evidence gap is the complete Docker image assembly, which was blocked before project build steps by unavailable Docker Hub metadata; the Docker startup contract and idempotent seed lifecycle are covered by repository tests. GitHub Actions remains the final clean-environment gate for the pushed branch.
