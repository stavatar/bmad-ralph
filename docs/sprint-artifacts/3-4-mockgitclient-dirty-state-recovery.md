# Story 3.4: MockGitClient + Dirty State Recovery

Status: done

## Story

As a runner,
I need to recover clean git state when a dirty working tree is detected at startup (interrupted session),
so that execution can proceed safely after crash recovery.

As a test developer,
I need a MockGitClient in `internal/testutil/` for unit-testing runner logic without real git,
so that runner tests are fast, deterministic, and don't require OS subprocess calls.

## Acceptance Criteria

1. **Dirty state recovery via git checkout:**
   Given working tree is dirty (interrupted session), when runner detects dirty state at startup (FR12), then executes `git checkout -- .` to restore clean state, and returns recovery info (not an error) to caller, and proceeds with execution after recovery.

2. **MockGitClient implements GitClient interface:**
   Given MockGitClient in `internal/testutil/mock_git.go`, when used in tests, then implements all GitClient methods (HealthCheck, HeadCommit, and any recovery method added), and returns preconfigured responses per test scenario, and tracks method call counts for assertions.

3. **MockGitClient simulates commit detection:**
   Given MockGitClient configured with HEAD sequence `["abc", "abc", "def"]`, when HeadCommit called three times, then returns `"abc"`, `"abc"`, `"def"` in order, and test can verify commit detection logic (new HEAD != old HEAD).

4. **MockGitClient simulates health check failure:**
   Given MockGitClient configured with HealthCheck returning ErrDirtyTree, when runner calls HealthCheck at startup, then receives ErrDirtyTree, and test can verify dirty state recovery path.

5. **Recovery only runs when dirty tree detected:**
   Given working tree is clean, when runner starts, then `git checkout -- .` is NOT executed, and execution proceeds normally.

## Tasks / Subtasks

- [x] Task 1: Extend GitClient interface with RestoreClean method (AC: #1, #2)
  - [x] 1.1 In `runner/git.go`: add `RestoreClean(ctx context.Context) error` to the GitClient interface (see current interface in "Existing Code" section). Interface becomes 3 methods: HealthCheck, HeadCommit, RestoreClean
  - [x] 1.2 Implement `ExecGitClient.RestoreClean(ctx context.Context) error`: run `exec.CommandContext(ctx, "git", "checkout", "--", ".")` with `cmd.Dir = e.Dir`. On success return nil. On failure return `fmt.Errorf("runner: git restore clean: %w", err)`
  - [x] 1.3 Update `ExecGitClient` doc comment to mention all 3 methods
  - [x] 1.4 Run `sed -i 's/\r$//' runner/git.go`

- [x] Task 2: Add ExecGitClient.RestoreClean tests to `runner/git_test.go` (AC: #1, #5)
  - [x] 2.1 `TestExecGitClient_RestoreClean_DirtyRepo` — create repo, modify tracked file, call RestoreClean, verify content restored, verify HealthCheck now returns nil
  - [x] 2.2 `TestExecGitClient_RestoreClean_CleanRepo` — create clean repo, call RestoreClean, verify returns nil (no-op on clean state)
  - [x] 2.3 `TestExecGitClient_RestoreClean_NotARepo` — use empty temp dir (no git init), call RestoreClean, verify error contains `"runner: git restore clean:"`
  - [x] 2.4 Run `sed -i 's/\r$//' runner/git_test.go`

- [x] Task 3: Create MockGitClient in `internal/testutil/mock_git.go` (AC: #2, #3, #4)
  - [x] 3.1 Create `internal/testutil/mock_git.go` with package `testutil`
  - [x] 3.2 Define `MockGitClient` struct with sequence-based responses and call tracking
  - [x] 3.3 Implement `MockGitClient.HealthCheck(ctx context.Context) error`
  - [x] 3.4 Implement `MockGitClient.HeadCommit(ctx context.Context) (string, error)`
  - [x] 3.5 Implement `MockGitClient.RestoreClean(ctx context.Context) error`
  - [x] 3.6 Run `sed -i 's/\r$//' internal/testutil/mock_git.go`

- [x] Task 4: Create MockGitClient unit tests (AC: #2, #3, #4)
  - [x] 4.1 Create `internal/testutil/mock_git_test.go`
  - [x] 4.2 `TestMockGitClient_HealthCheck_Sequence` — configure with `[]error{ErrDirtyTree, nil}`, call twice, verify returns dirty then nil, verify HealthCheckCount == 2
  - [x] 4.3 `TestMockGitClient_HeadCommit_Sequence` — configure with `[]string{"abc", "abc", "def"}`, call three times, verify sequence, verify HeadCommitCount == 3
  - [x] 4.4 `TestMockGitClient_HeadCommit_Error` — configure with HeadCommitErrors, verify error returned and SHA is empty
  - [x] 4.5 `TestMockGitClient_RestoreClean_CallTracking` — call RestoreClean, verify count incremented, verify nil error
  - [x] 4.6 `TestMockGitClient_RestoreClean_Error` — configure with RestoreCleanError, verify error returned
  - [x] 4.7 `TestMockGitClient_ZeroValue` — `MockGitClient{}` (all zero values), call each method once, verify HealthCheck returns nil, HeadCommit returns ("", nil), RestoreClean returns nil, verify all counts == 1
  - [x] 4.8 Compile-time interface check: `var _ runner.GitClient = (*MockGitClient)(nil)` in `mock_git.go` itself (not a test file — no circular import because `mock_git.go` already imports `runner`)
  - [x] 4.9 Run `sed -i 's/\r$//' internal/testutil/mock_git_test.go`

- [x] Task 5: Implement dirty state recovery in runner (AC: #1, #5)
  - [x] 5.1 Add `RecoverDirtyState` function in `runner/runner.go`. NOT wired into RunOnce/Run (Story 3.8).
  - [x] 5.2 Run `sed -i 's/\r$//' runner/runner.go`

- [x] Task 6: Test dirty state recovery function (AC: #1, #4, #5)
  - [x] 6.1 Create `runner/runner_test.go` with `package runner_test` (external test package to avoid import cycle testutil→runner→testutil)
  - [x] 6.2 Use single table-driven `TestRecoverDirtyState_Scenarios` with 6 subtests: clean repo, dirty recovery succeeds, dirty recovery fails, detached HEAD, merge in progress, context canceled
  - [x] 6.3 Run `sed -i 's/\r$//' runner/runner_test.go`

- [x] Task 7: Update integration test mock (AC: #2)
  - [x] 7.1 Replaced local `mockGitClient` with shared `testutil.MockGitClient`
  - [x] 7.2 Changed `package runner` → `package runner_test` to avoid import cycle (testutil imports runner)
  - [x] 7.3 Updated all mock initialization patterns and added `runner.` prefix to exported types
  - [x] 7.4 Run `sed -i 's/\r$//' runner/runner_integration_test.go`

- [x] Task 8: Run all tests and verify (AC: all)
  - [x] 8.1 Run `go test ./internal/testutil/...` — 6 MockGitClient tests + 9 mock_claude tests pass
  - [x] 8.2 Run `go test ./runner/...` — all 31 runner tests pass (13 git + 6 recovery + scan + prompt)
  - [x] 8.3 Run `go test -tags=integration ./runner/...` — all 39 tests pass with shared mock
  - [x] 8.4 Run `go test ./...` — full regression, no breakage
  - [x] 8.5 Run `go vet ./...` — no issues

## Prerequisites

- Story 3.3 (GitClient interface + ExecGitClient) — DONE, provides GitClient interface, ExecGitClient, sentinel errors
- Story 1.11 (Test infrastructure: mock Claude) — DONE, provides `internal/testutil/` package structure and mock pattern reference

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Files to create:**
- `internal/testutil/mock_git.go` — MockGitClient struct with sequence-based responses + call tracking
- `internal/testutil/mock_git_test.go` — MockGitClient unit tests

**Files to modify:**
- `runner/git.go` — Add `RestoreClean(ctx) error` to GitClient interface + ExecGitClient implementation
- `runner/git_test.go` — Add RestoreClean tests
- `runner/runner.go` — Add `RecoverDirtyState` function (or new `runner/recovery.go`)
- `runner/runner_integration_test.go` — Replace local mockGitClient with shared `testutil.MockGitClient`

**New test file:**
- `runner/runner_test.go` — RecoverDirtyState unit tests (uses MockGitClient, NOT integration test)

**IMPORTANT: Story 3.3 files are on disk but NOT committed.** The git diff shows modifications to `runner/runner.go` and `runner/runner_integration_test.go`, plus untracked `runner/git.go` and `runner/git_test.go`. These files contain the GitClient interface and ExecGitClient that this story builds upon.

### Architecture Compliance

**Dependency direction and circular import:** `internal/testutil/mock_git.go` (a regular .go file, NOT a test file) imports `runner` for the GitClient interface. `runner/*_test.go` files import `testutil` for the mock. No circular dependency arises because Go's import cycle prohibition applies to non-test packages only — `runner` package itself never imports `testutil` in non-test code. The compile-time check `var _ runner.GitClient = (*MockGitClient)(nil)` goes directly in `mock_git.go` (already imports `runner`).

**Interface extension:** Adding `RestoreClean` to GitClient changes it from 2 to 3 methods. This is the minimal extension needed for dirty state recovery. Do NOT add more methods (like `IsClean`, `Status`, etc.).

**Config immutability:** RecoverDirtyState takes GitClient (not config). Config is read-only, passed via RunConfig.

**No logging from packages:** `RecoverDirtyState` returns `(bool, error)`. The bool indicates recovery happened. `cmd/ralph/` logs the warning. The runner package NEVER calls `fmt.Println` or any log function.

### MockGitClient Design (Reference: mock_claude.go pattern)

Follow the established mock pattern from `internal/testutil/mock_claude.go`:
- **Scenario-based responses:** Ordered sequences (HeadCommits slice → returns in order)
- **Call tracking:** Counters for assertion (HealthCheckCount, HeadCommitCount, RestoreCleanCount)
- **Preconfigured errors:** Per-method error configuration

**Sequence behavior:**
- `HeadCommit` returns `HeadCommits[index]` for each call. If index exceeds slice length, return last element (prevents test panics on unexpected calls)
- `HealthCheck` returns `HealthCheckErrors[index]`. If index exceeds slice length, return nil (healthy by default)
- `RestoreClean` returns single `RestoreCleanError` (no sequence needed — called at most once per run)

**Zero-value behavior:** `MockGitClient{}` (all zero values) should:
- `HealthCheck` → returns nil (healthy)
- `HeadCommit` → returns `""` (empty SHA — caller should handle)
- `RestoreClean` → returns nil (success)

### Dirty State Recovery Design

**Recovery flow in runner:**
```
runner.Run(ctx, cfg) or runner.RunOnce(ctx, rc):
  1. RecoverDirtyState(ctx, git) → (recovered bool, err error)
  2. If err != nil → return err (fatal: detached HEAD, merge, recovery failed)
  3. If recovered → caller logs warning (cmd/ralph or test asserts)
  4. Proceed with normal execution
```

**What `git checkout -- .` does:** Discards ALL uncommitted changes to tracked files. Does NOT remove untracked files. This is the correct recovery for an interrupted Claude session that left dirty changes.

**When NOT to recover:** ErrDetachedHead and ErrMergeInProgress are NOT auto-recoverable. These indicate a more serious state that requires user intervention. Return the error and let cmd/ralph handle it.

**Merge+dirty precedence:** Because HealthCheck checks merge/rebase BEFORE dirty tree, a repo that is both dirty and in merge/rebase state returns ErrMergeInProgress (not ErrDirtyTree). RecoverDirtyState correctly does NOT attempt recovery in this case — merge/rebase requires user intervention.

**Error prefix note:** `RunOnce` wraps HealthCheck errors as `"runner: git health:"`, while `ExecGitClient.HealthCheck` wraps as `"runner: git health check:"`. `RecoverDirtyState` calls `HealthCheck` directly (not through RunOnce), so `errors.Is` works correctly against sentinel errors. Do NOT call RecoverDirtyState from within RunOnce's error handling — it is a separate startup step (Story 3.8).

### Existing Code to Be Aware Of

**Current GitClient interface (runner/git.go):**
```go
type GitClient interface {
    HealthCheck(ctx context.Context) error
    HeadCommit(ctx context.Context) (string, error)
}
```

**Current sentinel errors (runner/git.go):**
```go
var (
    ErrDirtyTree       = errors.New("git: working tree is dirty")
    ErrDetachedHead    = errors.New("git: HEAD is detached")
    ErrMergeInProgress = errors.New("git: merge or rebase in progress")
)
```

**Current local mockGitClient (runner/runner_integration_test.go:17-31):**
```go
type mockGitClient struct {
    healthCheckErr error
    headCommit     string
    headCommitErr  error
}
```
This local mock must be replaced with the shared `testutil.MockGitClient` in Task 7.

**ExecGitClient pattern for new method (runner/git.go):**
```go
func (e *ExecGitClient) RestoreClean(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "git", "checkout", "--", ".")
    cmd.Dir = e.Dir
    if _, err := cmd.Output(); err != nil {
        return fmt.Errorf("runner: git restore clean: %w", err)
    }
    return nil
}
```
Using `cmd.Output()` (not `cmd.Run()`) is intentional — on failure, `*exec.ExitError` from `Output()` includes stderr in `err.Stderr` for better error diagnostics. Matches the existing `HeadCommit` pattern.

### Error Wrapping Convention

All errors in runner package follow `fmt.Errorf("runner: <operation>: %w", err)`:
- `"runner: git restore clean: %w"` — ExecGitClient.RestoreClean failure
- `"runner: dirty state recovery: %w"` — RecoverDirtyState wrapping RestoreClean error
- Sentinel errors (ErrDirtyTree, etc.) are passed through as-is (already wrapped by HealthCheck)

### Testing Strategy

**MockGitClient tests** (in `internal/testutil/mock_git_test.go`):
- Test sequence returns, call counting, zero-value behavior
- These are simple unit tests, no external deps

**RecoverDirtyState tests** (in `runner/runner_test.go` — new file, NOT integration):
- Use `testutil.MockGitClient` directly — no real git needed
- Table-driven for clean/dirty/detached/merge/recovery-fail scenarios
- Verify: returned bool, returned error, RestoreCleanCount

**ExecGitClient.RestoreClean tests** (in `runner/git_test.go` — extends existing):
- Real git repos via `initGitRepo` helper (reuse from Story 3.3)
- Test actual `git checkout -- .` behavior with dirty files
- Use `runGit` helper for setup

**Integration tests** (in `runner/runner_integration_test.go`):
- Replace local mock with shared MockGitClient
- Existing tests continue to work with new mock API

### What NOT to Do

- Do NOT use `exec.Command()` without context — always `exec.CommandContext(ctx, ...)`
- Do NOT use `context.TODO()` in production code — always propagate ctx from caller
- Do NOT call `os.Exit` from runner — return errors, let cmd/ralph handle exit codes
- Do NOT use `git reset --hard` — too destructive, `git checkout -- .` is sufficient
- Do NOT use `git clean -fd` — removes untracked files which may be intentional
- Do NOT auto-recover from detached HEAD or merge/rebase — user-intervention states
- Do NOT log from runner package — return (bool, error), caller decides output
- Do NOT wire RecoverDirtyState into RunOnce/Run — Story 3.8 does that
- Do NOT create a separate `recovery/` or `git/` package — recovery lives in runner
- Do NOT add `IsClean()`, `Status()`, or other extra methods to GitClient
- Do NOT define `var update = flag.Bool("update", ...)` in new test files — already in `runner/prompt_test.go`
- Do NOT duplicate MockGitClient fields — use slices for sequences, not separate fields

### Previous Story Intelligence

**From Story 3.3 (GitClient Interface):**
- HealthCheck order: merge/rebase first (most-specific), then detached HEAD, then dirty tree
- Error wrapping: sentinel errors as `%w` argument for `errors.Is` to work
- `runGit` helper extracted to avoid boilerplate (reuse in Task 2 tests)
- `initGitRepo` helper creates repo with initial commit (reuse in Task 2 tests)
- Review fixed 6 issues: hardcoded branch name, untested indicators, duplicate helpers
- All 10 ExecGitClient tests pass on WSL/NTFS

**From Story 3.2 (Sprint-Tasks Scanner):**
- Stale doc comments after refactoring — update immediately when behavior changes
- Symmetric nil checks in assertions — if testing one path, verify other path's state too
- Edge case field assertions — verify ALL struct fields, not just counts

**From Story 3.1 (Execute Prompt Template):**
- if/else over duplicate conditionals
- Comment accuracy — doc comments must match reality

### Git Intelligence

**Uncommitted changes from Story 3.3:**
```
M  runner/runner.go (removed GitClient stub, updated HasNewCommit → HeadCommit)
M  runner/runner_integration_test.go (updated local mockGitClient)
?? runner/git.go (NEW: GitClient interface + ExecGitClient)
?? runner/git_test.go (NEW: 10 tests with real git)
?? docs/sprint-artifacts/3-3-gitclient-interface-execgitclient.md
```

**Recent commits:**
- `9248eb8` Story 3.2: Sprint-tasks scanner
- `675a3e4` Knowledge extraction update
- `b6ebc7d` Story 3.1: Execute prompt template

### Project Structure Notes

```
internal/testutil/
  mock_git.go       ← CREATE (MockGitClient)
  mock_git_test.go  ← CREATE (MockGitClient tests)
  mock_claude.go    ← EXISTS (reference for mock pattern)
runner/
  git.go            ← MODIFY (add RestoreClean to interface + ExecGitClient impl)
  git_test.go       ← MODIFY (add RestoreClean tests)
  runner.go         ← MODIFY (add RecoverDirtyState function)
  runner_test.go    ← CREATE (RecoverDirtyState unit tests)
  runner_integration_test.go ← MODIFY (replace local mock with shared MockGitClient)
```

No new packages. No new dependencies. `internal/testutil/mock_git.go` imports `runner` for the GitClient interface — this is the only cross-package import added.

**Task dependency order:** Tasks 1-2 (ExecGitClient.RestoreClean) and Tasks 3-4 (MockGitClient) are independent. Task 5 depends on Task 1 (interface). Task 6 depends on Tasks 3+5. Task 7 depends on Task 3. Task 8 depends on all. **Recommended order: 1 → 3 → 7 → 2 → 4 → 5 → 6 → 8** (fixes compilation immediately after interface change).

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story-3.4 — AC, technical notes, prerequisites]
- [Source: docs/project-context.md#Subprocess-Pattern — exec.CommandContext, cmd.Dir, no wall-clock timeout]
- [Source: docs/project-context.md#Testing — MockGitClient in internal/testutil/, scenario-based]
- [Source: docs/architecture/project-structure-boundaries.md — internal/testutil/mock_git.go location]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md — Mock prefix, interface in consumer]
- [Source: runner/git.go — GitClient interface, ExecGitClient, sentinel errors (Story 3.3)]
- [Source: runner/git_test.go — runGit helper, initGitRepo helper, 10 existing tests]
- [Source: runner/runner.go — RunOnce, RunConfig struct, current HealthCheck call]
- [Source: runner/runner_integration_test.go — local mockGitClient to replace]
- [Source: internal/testutil/mock_claude.go — Mock pattern reference (scenario-based, call tracking)]
- [Source: docs/sprint-artifacts/3-3-gitclient-interface-execgitclient.md — Previous story dev notes]
- [Source: .claude/rules/go-testing-patterns.md — All testing patterns]
- [Source: .claude/rules/wsl-ntfs.md — WSL/NTFS patterns]

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- Import cycle fix: Dev Notes claimed `package runner` test files can import testutil→runner without cycle. In practice Go detects this as "import cycle not allowed in test". Fixed by switching both `runner_test.go` and `runner_integration_test.go` to `package runner_test` (external test package).
- DirtyRepo test: initial approach added new untracked file + `git add`. `git checkout -- .` doesn't remove staged-but-never-committed files. Fixed by modifying existing tracked file (README.md) instead.
- CRLF portability: `git checkout` restores files with CRLF on WSL/NTFS. Fixed assertion to use `strings.TrimSpace` for cross-platform comparison.

### Completion Notes List

- GitClient interface extended from 2 to 3 methods (HealthCheck, HeadCommit, RestoreClean)
- ExecGitClient.RestoreClean implemented using `git checkout -- .` with `cmd.Output()` pattern
- MockGitClient created in `internal/testutil/` with sequence-based responses and call tracking
- Compile-time interface check `var _ runner.GitClient = (*MockGitClient)(nil)` in mock_git.go
- RecoverDirtyState function added to runner.go — NOT wired into RunOnce/Run (Story 3.8)
- Integration test migrated from local mockGitClient to shared testutil.MockGitClient
- Both runner_test.go and runner_integration_test.go use `package runner_test` to avoid import cycle
- Total: 6 MockGitClient tests + 3 RestoreClean tests + 7 RecoverDirtyState scenarios = 16 new tests
- All tests pass with integration tag, full regression passes (go test ./... + go vet ./...)

### Change Log

- 2026-02-26: Story 3.4 implementation — MockGitClient, RestoreClean, RecoverDirtyState, integration test migration
- 2026-02-26: Code review — 7 findings (0H/4M/3L), all 7 fixed:
  - M1: RecoverDirtyState consistent error wrapping on all paths
  - M2: MockGitClient.HealthCheckErrors doc comment accuracy
  - M3: wantHealthCheckCount assertion added to RecoverDirtyState tests
  - M4: context.DeadlineExceeded test case added (7th scenario)
  - L1: wantErrContainsInner for "restore failed" inner error verification
  - L2: HealthCheck beyond-length behavior test added to mock_git_test.go
  - L3: copyFixtureToDir helper extracted, 4 boilerplate blocks replaced

### File List

**New files:**
- `internal/testutil/mock_git.go` — MockGitClient struct with sequence-based responses and call tracking
- `internal/testutil/mock_git_test.go` — 6 MockGitClient unit tests
- `runner/runner_test.go` — 7 RecoverDirtyState scenario tests (package runner_test)

**Modified files:**
- `runner/git.go` — Added RestoreClean to GitClient interface + ExecGitClient implementation
- `runner/git_test.go` — Added 3 ExecGitClient_RestoreClean tests
- `runner/runner.go` — Added RecoverDirtyState function
- `runner/runner_integration_test.go` — Replaced local mockGitClient with shared testutil.MockGitClient, changed to package runner_test
- `docs/sprint-artifacts/sprint-status.yaml` — Story status ready-for-dev → in-progress → review
