# Testing Guide

Quick reference for testing ai-config-manager.

## Commands

```bash
make test             # All tests (vet -> unit -> integration)
make unit-test        # Fast unit tests (<5s)
make integration-test # Network-dependent tests (~30s)
make e2e-test         # Full CLI tests (~1-2min)
```

## Critical Rules

- **ALWAYS** use `t.TempDir()` for temporary directories
- **ALWAYS** use `repo.NewManagerWithPath(tmpDir)` in tests -- NEVER `NewManager()`
- **NEVER** write to `~/.local/share/ai-config/` in tests
- **ALWAYS** run `make integration-test` before finishing larger work such as a bigger feature, an epic, or broad cross-cutting changes. Focused tests are not enough for session close in those cases.

## Concurrency Test Expectations

- Use deterministic coordination for concurrent-process tests (file-based ready/release markers).
- Do **not** rely on sleep-only timing to trigger contention.
- For CLI/process-level contention, run the locally built binary (`./aimgr`) or the E2E test-built binary, and always set `AIMGR_REPO_PATH` to a temp repo.
- Validate both:
  - safety outcomes (manifest/source metadata remains valid, no corrupted repo state)
  - user-facing behavior (second process waits or fails with clear lock-acquisition errors under contention)
- Keep contention tests isolated to temp repos and temp source fixtures.

For repo lock primitives, include focused coverage for:

- shared/shared behavior (concurrent readers succeed on POSIX)
- shared/exclusive contention both directions (reader blocks writer, writer blocks reader)
- in-process same-path non-reentrancy (second acquisition fails with `ErrNonReentrantLock`)
- same-path transition attempts (read→write and write→read fail with `ErrNonReentrantLock`)
- explicit Windows behavior verification for the read/write API (including exclusive-only fallback when applicable)

## Persistence / Atomic-Write Expectations

Repo-managed state persistence uses atomic replacement (not plain in-place
`os.WriteFile`) for:

- `ai.repo.yaml`
- `.metadata/sources.json`
- resource metadata files under `.metadata/...`
- `.workspace/.cache-metadata.json`

When adding/changing tests around these files, keep expectations aligned with
the implemented write sequence:

1. temp file in same directory
2. write + file `fsync`
3. rename replacement
4. parent-directory `fsync` where supported

Locking and atomic writes are complementary: tests that exercise concurrent
mutations should still validate lock-mediated serialization, while crash-safety
behavior focuses on atomic replacement semantics.

## Full Guide

Read **[docs/contributor-guide/testing.md](contributor-guide/testing.md)** for test types, isolation patterns, table-driven tests, and troubleshooting.
