# Coding Guide

## Build, install, and static checks

| Goal | Command | Notes |
| --- | --- | --- |
| Build the local binary | `make build` | Writes `./aimgr` |
| Show install target for this OS | `make os-info` | Use before `make install` if the install path matters |
| Install the binary | `make install` | Installs to the OS-specific path reported by `make os-info` |
| Format Go code | `make fmt` | Run before commit |
| Run static analysis | `make vet` | Baseline local static-analysis check; CI also runs `golangci-lint` from `.github/workflows/build.yml` |

For test selection and session-close validation, use `docs/TESTING.md`.

## CRITICAL: Repository Safety for Testing

**NEVER run `aimgr repo` commands against the global repository during testing or bug reproduction!**

The default repository location is `~/.local/share/ai-config/repo/` which contains your real aimgr configuration. Testing against this will corrupt your development environment.

**Safe methods:**

| Method | Usage |
|--------|-------|
| Environment variable (recommended) | `export AIMGR_REPO_PATH=/tmp/test-repo-$(date +%s)` |
| Config file | `printf 'repo:\n  path: /tmp/test-repo\n' > /tmp/test-config.yaml && ./aimgr --config /tmp/test-config.yaml repo init` |
| Go tests | `repo.NewManagerWithPath(t.TempDir())` |

**Bottom line**: Every test operation MUST explicitly specify a temporary repository location. No exceptions.

## CRITICAL: Use Locally Built Binary

**ALWAYS use `./aimgr` (the locally built binary) when testing changes, NOT `aimgr` from PATH!**

Version managers (mise, asdf, etc.) may install older versions that are found first in PATH.

```bash
# CORRECT: Use local binary
./aimgr --version
./aimgr repo init

# WRONG: May use mise/asdf version
aimgr --version
```

## Common change map

| If you are changing... | Start with... |
| --- | --- |
| CLI flags, command UX, or exit behavior | `cmd/` and matching `cmd/*_test.go` files |
| Repo add/sync/remove/repair behavior | `pkg/repo/`, `pkg/repomanifest/`, `pkg/sourcemetadata/`, `cmd/repo_*.go` |
| Install or uninstall behavior | `pkg/install/`, `pkg/tools/`, `cmd/install.go`, `cmd/uninstall.go` |
| Resource parsing or validation | `pkg/resource/`, `cmd/resource_validate.go`, `test/resource_validate_test.go` |
| Workspace cache or Git interactions | `pkg/workspace/`, `pkg/source/`, `test/workspace_*`, `test/git_*` |
| Output formatting or machine-readable output | `pkg/output/`, `cmd/list*.go`, `cmd/repo_*` |

## Concurrency and Locking Model (Repo/Workspace Mutations)

All repo mutations are coordinated with OS-backed advisory file locks under:

`<repo>/.workspace/locks/`

Lock files:

- Repo-wide lock: `<repo>/.workspace/locks/repo.lock` (single path for both read and write)
- Workspace metadata lock: `<repo>/.workspace/locks/workspace-metadata.lock`
- Per-cache lock: `<repo>/.workspace/locks/caches/<cache-hash>.lock`

Repo lock modes:

- **Read/shared lock**: allows concurrent readers on POSIX (`flock` `LOCK_SH`)
- **Write/exclusive lock**: blocks all other readers/writers (`flock` `LOCK_EX`)
- Read and write modes use the **same `repo.lock` file**.

Lock ordering is strict and must never be reversed:

1. repo lock
2. cache lock
3. workspace metadata lock

Rules:

- Top-level mutating CLI commands are outermost repo-lock holders.
- Workspace cache operations may take cache lock and metadata lock, but must not try to take repo lock from inside those sections.
- Workspace metadata lock is only for short metadata read-modify-write sections (not long clone/fetch/pull work).
- Cache and workspace metadata locks remain **exclusive-only**.
- Locks are intentionally **non-reentrant per process and path**: acquiring the same lock path twice in one process fails with `ErrNonReentrantLock`.
- Same-process lock transition attempts on the same path (read→write or write→read) also fail through the same non-reentrant contract (no implicit upgrade/downgrade protocol).

Scope / limitations:

- These locks are implemented with `flock` on Unix/POSIX builds.
- On Windows, locks use `LockFileEx`/`UnlockFileEx` from the Windows API.
- **Windows fallback behavior**: repo read-lock APIs currently map to exclusive `LockFileEx` locking, so concurrent shared readers are not currently enabled on Windows.
- `flock` does not provide fairness guarantees; under heavy contention, lock starvation is possible. `aimgr` uses timeout-bounded acquisition rather than starvation prevention.
- Locks serialize *aimgr* mutation paths; they do not prevent arbitrary external tools from modifying files directly.

## Repo Lock Command Matrix

Use this matrix when adding or changing commands that touch shared repo state.

| Command / Path | Lock mode | Why |
|---|---|---|
| `aimgr repo init` | write | Creates/updates repo structure and metadata. |
| `aimgr repo add` | write | Adds resources/metadata to shared repo state. |
| `aimgr repo sync` | write | Mutates repo content and source metadata. |
| `aimgr repo rebuild` | write | Orchestrates soft drop + sync under one lock. |
| `aimgr repo remove` | write | Removes source entries and associated metadata. |
| `aimgr repo drop` | write | Deletes repository data. |
| `aimgr repo apply-manifest` | write | Applies source mutations from manifest. |
| `aimgr repo override-source` | write | Mutates source overrides/metadata. |
| `aimgr repo prune` | write | Deletes unreferenced `.workspace/` Git caches. |
| `aimgr repo repair` | write | Rewrites repo metadata/content for consistency. |
| `aimgr repo verify --fix` | write | Applies corrective repo mutations. |
| `aimgr repo verify` (read-only) | read | Scans shared repo files/metadata without mutation. |
| `aimgr repo info` | read | Reads repo manifest/resources/metadata snapshot. |
| `aimgr repo list` | read | Reads resource/package/metadata listings. |
| `aimgr repo describe` / `repo show` | read | Reads resource/package definitions + metadata. |
| `aimgr repo show-manifest` | read | Reads repo manifest snapshot. |
| `aimgr install` | read or write | Uses read lock for explicit resource installs; zero-arg manifest install takes write lock when effective `sources:` may bootstrap/sync shared repo sources. |
| `installFromManifest` (internal) | read or write | Uses write lock when manifest-declared sources can mutate `ai.repo.yaml`/repo content during bootstrap; otherwise read-only install path remains read lock. |
| package install expansion (`installPackage`) | read (via `install`) | Reads package definitions and member resources from shared repo state. |
| `aimgr repair` (project) | read | Expands package refs and validates repo-backed declared resources before reconcile. |
| `aimgr verify` (project) | read | Resolves package refs and repo-backed membership during manifest checks/reconcile wrapper. |
| `aimgr list` (installed resources) | read | Expands manifest package refs by reading repo package definitions. |
| `aimgr uninstall` | read | Reads repo package definitions for `package/*` uninstall expansion. |
| repo-backed shell completions | try-read (non-blocking) | Completions must stay responsive; return no dynamic suggestions while writers hold lock. |

Intentional no-lock helpers:

- `cmd.ExpandPattern` does not acquire repo locks itself. Callers must hold the appropriate repo lock before invoking it when a stable repo snapshot is required.

## Atomic Write Model (Repo-Managed State Files)

Repo-managed state files are written with **atomic replacement**, not in-place
truncating writes.

State files covered by this model include:

- `ai.repo.yaml`
- `.metadata/sources.json`
- Resource metadata files under `.metadata/...` (for example
  `.metadata/skills/<name>-metadata.json`)
- `.workspace/.cache-metadata.json`

Write sequence (via `pkg/fileutil.AtomicWrite`):

1. Create a temporary file in the **same directory** as the target file.
2. Write full file contents to the temp file.
3. `fsync` the temp file.
4. Rename the temp file over the destination path.
5. `fsync` the parent directory where supported.

Important limits:

- Parent directories must already exist before writing.
- Atomic replacement protects against partial-file writes during process crashes,
  but it does not by itself resolve concurrent read-modify-write races; locks
  still provide serialization for mutation paths.
- Parent-directory `fsync` is best-effort by platform; on Windows this layer
  currently does not perform directory `fsync`.
