# Story 3.3: GitClient Interface + ExecGitClient

Status: done

## Story

As a runner,
I need a GitClient interface with ExecGitClient implementation,
so that the runner can validate git repository health at startup and detect new commits after execute-sessions.

## Acceptance Criteria

1. **Git health check passes on clean repo:**
   Given project directory is a git repo, working tree is clean, HEAD is not detached, not in merge/rebase state, when `GitClient.HealthCheck(ctx)` is called, then returns nil error.

2. **Git health check fails on dirty tree:**
   Given working tree has uncommitted changes, when `GitClient.HealthCheck(ctx)` is called, then returns `ErrDirtyTree` sentinel error.

3. **Git health check fails on detached HEAD:**
   Given HEAD is detached, when `GitClient.HealthCheck(ctx)` is called, then returns error indicating detached HEAD (`ErrDetachedHead` sentinel).

4. **Git health check fails during merge/rebase:**
   Given repo is in merge or rebase state, when `GitClient.HealthCheck(ctx)` is called, then returns error indicating merge/rebase in progress (`ErrMergeInProgress` sentinel).

5. **Commit detection via HEAD comparison:**
   Given initial HEAD is "abc123", when execute session completes and `GitClient.HeadCommit(ctx)` returns "def456", then commit is detected (new HEAD != old HEAD). `HeadCommit` returns full 40-char hex SHA from `git rev-parse HEAD`; callers compare via string equality (`oldSHA != newSHA`). Comparison logic lives in the runner loop (Story 3.5), not in GitClient.

6. **No commit detection:**
   Given initial HEAD is "abc123", when execute session completes and `GitClient.HeadCommit(ctx)` still returns "abc123", then no commit detected (trigger retry/resume-extraction).

7. **ExecGitClient uses exec.CommandContext:**
   Given ExecGitClient implementation, when any git command is executed, then uses `exec.CommandContext(ctx, "git", ...)` with context propagation, and `cmd.Dir` is set to `config.ProjectRoot`.

## Tasks / Subtasks

- [x] Task 1: Create `runner/git.go` with GitClient interface, sentinel errors, and ExecGitClient implementation (AC: #1-#7)
  - [x] 1.1 Move `GitClient` interface from `runner/runner.go` to `runner/git.go`. Change method signature: `HasNewCommit(ctx context.Context) (bool, error)` → `HeadCommit(ctx context.Context) (string, error)`. The interface has exactly 2 methods: `HealthCheck(ctx) error` and `HeadCommit(ctx) (string, error)`
  - [x] 1.2 Define sentinel errors in `runner/git.go`:
    ```go
    var (
        ErrDirtyTree      = errors.New("git: working tree is dirty")
        ErrDetachedHead   = errors.New("git: HEAD is detached")
        ErrMergeInProgress = errors.New("git: merge or rebase in progress")
    )
    ```
  - [x] 1.3 Define `ExecGitClient` struct: `type ExecGitClient struct { Dir string }` — `Dir` is set to `config.ProjectRoot` by caller
  - [x] 1.4 Implement `ExecGitClient.HealthCheck(ctx context.Context) error`:
    - Run `exec.CommandContext(ctx, "git", "status", "--porcelain")` with `cmd.Dir = e.Dir`
    - If output is non-empty → return `fmt.Errorf("runner: git health check: %w", ErrDirtyTree)`
    - Run `exec.CommandContext(ctx, "git", "symbolic-ref", "-q", "HEAD")` with `cmd.Dir = e.Dir`
    - If exit code != 0 → return `fmt.Errorf("runner: git health check: %w", ErrDetachedHead)`
    - Check for merge/rebase state: run `exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")` to get git dir path (may be relative `.git` or absolute), then use `filepath.Join(gitDir, "MERGE_HEAD")`, `filepath.Join(gitDir, "rebase-merge")`, `filepath.Join(gitDir, "rebase-apply")` and check ANY of them via `os.Stat` — if any exists, merge/rebase in progress
    - If merge/rebase detected → return `fmt.Errorf("runner: git health check: %w", ErrMergeInProgress)`
    - All checks pass → return nil
    - For any unexpected command failures → return `fmt.Errorf("runner: git health check: %w", err)`
    - CRITICAL: sentinel errors MUST be the `%w` argument (not `.Error()` string) for `errors.Is` to work
  - [x] 1.5 Implement `ExecGitClient.HeadCommit(ctx context.Context) (string, error)`:
    - Run `exec.CommandContext(ctx, "git", "rev-parse", "HEAD")` with `cmd.Dir = e.Dir`
    - Return `strings.TrimSpace(string(output)), nil` on success
    - Return `"", fmt.Errorf("runner: git head commit: %w", err)` on failure
  - [x] 1.6 Run `sed -i 's/\r$//' runner/git.go` to fix line endings

- [x] Task 2: Create `runner/git_test.go` with unit tests for ExecGitClient (AC: #1-#7)
  - [x] 2.1 Create `runner/git_test.go` with package `runner` (internal test). Use `t.TempDir()` with real `git init` for each test scenario. All tests require real git binary — use `exec.LookPath("git")` to skip if not available with `t.Skip("git not in PATH")`
  - [x] 2.2 `TestExecGitClient_HealthCheck_CleanRepo` — init git repo, create initial commit, verify `HealthCheck` returns nil
  - [x] 2.3 `TestExecGitClient_HealthCheck_DirtyTree` — init git repo, create initial commit, create uncommitted file, verify `errors.Is(err, ErrDirtyTree)` AND `strings.Contains(err.Error(), "dirty")`
  - [x] 2.4 `TestExecGitClient_HealthCheck_DetachedHead` — init git repo, create 2 commits, checkout specific SHA (detached), verify `errors.Is(err, ErrDetachedHead)` AND `strings.Contains(err.Error(), "detached")`
  - [x] 2.5 `TestExecGitClient_HealthCheck_MergeInProgress` — init git repo, create conflicting merge state (two branches with conflicting changes, then `git merge` that produces conflict), verify `errors.Is(err, ErrMergeInProgress)` AND `strings.Contains(err.Error(), "merge")`
  - [x] 2.6 `TestExecGitClient_HeadCommit_Success` — init git repo, create commit, verify returned SHA matches `git rev-parse HEAD`, verify `strings.TrimSpace` (no trailing newline)
  - [x] 2.7 `TestExecGitClient_HeadCommit_NotARepo` — use empty temp dir (no git init), verify error is returned, verify `strings.Contains(err.Error(), "runner: git head commit:")`
  - [x] 2.8 `TestExecGitClient_HealthCheck_NotARepo` — use empty temp dir, verify error wrapping with `strings.Contains(err.Error(), "runner: git health check:")`
  - [x] 2.9 `TestExecGitClient_HealthCheck_ContextCanceled` — create repo, cancel context before calling HealthCheck, verify error contains context cancellation. Also verify error is still wrapped with `strings.Contains(err.Error(), "runner: git health check:")`
  - [x] 2.10 `TestExecGitClient_HeadCommit_EmptyRepo` — init git repo with no commits, verify error (HEAD doesn't exist)
  - [x] 2.11 Run `sed -i 's/\r$//' runner/git_test.go` to fix line endings

- [x] Task 3: Refactor `runner/runner.go` and `runner/runner_integration_test.go` for interface change (AC: #5, #6, #7)
  - [x] 3.1 In `runner/runner.go`: remove the `GitClient` interface definition (moved to `git.go`). Remove the comment block about Story 3.3. The interface import remains automatic since `git.go` is in same package
  - [x] 3.2 In `runner/runner.go` line 89: change `HasNewCommit(ctx)` call to `HeadCommit(ctx)`. Discard the returned SHA with `_` for now (Story 3.5 will use the return value for before/after comparison). Update error message to `"runner: head commit: %w"`:
    ```go
    if _, err := rc.Git.HeadCommit(ctx); err != nil {
        return fmt.Errorf("runner: head commit: %w", err)
    }
    ```
  - [x] 3.3 In `runner/runner_integration_test.go`: update `mockGitClient` struct — change `hasNewCommit bool` + `hasNewCommitErr error` to `headCommit string` + `headCommitErr error`. Update `HasNewCommit` method → `HeadCommit` method returning `(string, error)`
  - [x] 3.4 Update all `mockGitClient` usages in integration tests:
    - `hasNewCommit: true` → `headCommit: "abc123"` (non-empty string simulates commit)
    - `hasNewCommitErr: errors.New(...)` → `headCommitErr: errors.New(...)`
    - Test `TestRunOnce_WalkingSkeleton_HasNewCommitFails` → rename to `TestRunOnce_WalkingSkeleton_HeadCommitFails`, update all assertions from "check commit" to "head commit"
    - Specifically: line 310 assertion `strings.Contains(err.Error(), "runner: check commit:")` → `"runner: head commit:"`
  - [x] 3.5 Run `sed -i 's/\r$//' runner/runner.go runner/runner_integration_test.go` to fix line endings

- [x] Task 4: Run all tests and verify (AC: all)
  - [x] 4.1 Run `go test ./runner/...` — all tests pass (git_test.go + existing tests)
  - [x] 4.2 Run `go test ./config/...` — all config tests pass (no changes expected)
  - [x] 4.3 Run `go test ./...` — full regression, no breakage
  - [x] 4.4 Run `go vet ./...` — no issues
  - [x] 4.5 Verify interface has exactly 2 methods: `grep -c 'func.*context.Context' runner/git.go` should show HealthCheck and HeadCommit
  - [x] 4.6 Verify sentinel errors defined: `grep 'var Err' runner/git.go` should show ErrDirtyTree, ErrDetachedHead, ErrMergeInProgress
  - [x] 4.7 Verify no `exec.Command(` without Context in `runner/git.go`: `grep 'exec\.Command(' runner/git.go` should return 0 results (only `exec.CommandContext`)

## Prerequisites

- Story 1.8 (session package with `exec.CommandContext` pattern — establishes subprocess pattern)
- Story 3.2 (sprint-tasks scanner — completes before this story, establishes runner package patterns)

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Primary files to create:** `runner/git.go` (interface + implementation + sentinel errors) and `runner/git_test.go` (tests with real git). Also modify `runner/runner.go` (remove interface stub, update call site) and `runner/runner_integration_test.go` (update mock).

**Interface change:** Current `runner/runner.go:23-26` has a placeholder `GitClient` interface with `HasNewCommit(ctx) (bool, error)`. Story 3.3 replaces this with the proper interface in `runner/git.go`:
```go
type GitClient interface {
    HealthCheck(ctx context.Context) error
    HeadCommit(ctx context.Context) (string, error)
}
```
The method `HasNewCommit` returns `(bool, error)` — the new `HeadCommit` returns `(string, error)`. The caller (runner loop, Story 3.5) does the before/after SHA comparison. This is a deliberate design: GitClient returns data, runner makes decisions.

**Current mockGitClient in runner_integration_test.go (lines 19-31):**
```go
type mockGitClient struct {
    healthCheckErr  error
    hasNewCommit    bool
    hasNewCommitErr error
}
func (m *mockGitClient) HealthCheck(ctx context.Context) error {
    return m.healthCheckErr
}
func (m *mockGitClient) HasNewCommit(ctx context.Context) (bool, error) {
    return m.hasNewCommit, m.hasNewCommitErr
}
```
Must be updated to match new interface. All usages (lines 69, 139, 170, 204, 240, 261, 303, 336) need updating from `hasNewCommit: true` to `headCommit: "abc123"`.

**exec.CommandContext pattern (from session/session.go:56-58):**
```go
cmd := exec.CommandContext(ctx, opts.Command, args...)
cmd.Dir = opts.Dir
```
ExecGitClient follows the same pattern but with `"git"` as the command and `e.Dir` as the directory.

**Sentinel errors (existing pattern from config/errors.go):**
```go
var ErrNoTasks = errors.New("no tasks found")
```
Story 3.3 adds runner-scoped sentinel errors in `runner/git.go`. These are NOT in `config/` because they're git-specific to the runner package.

**Error wrapping convention:**
All errors in `runner/git.go` use `fmt.Errorf("runner: <operation>: %w", err)` pattern. Examples:
- `fmt.Errorf("runner: git health check: %w", ErrDirtyTree)`
- `fmt.Errorf("runner: git head commit: %w", err)`

### Architecture Compliance

**Dependency direction:** `runner` → `config` (only uses `config.ProjectRoot` indirectly via `ExecGitClient.Dir`). No new package dependencies introduced. `ExecGitClient` uses only stdlib: `context`, `errors`, `fmt`, `os`, `os/exec`, `path/filepath`, `strings`.

**Interface in consumer package:** `GitClient` interface defined in `runner/git.go` (consumer package), NOT in a separate `git/` package. This follows the established pattern — see `runner/runner.go` comment: "Consumer-side interface per naming convention".

**No wall-clock timeout:** Git commands use `exec.CommandContext(ctx)` for cancellation. No `time.After` or explicit timeouts. Context propagation from `cmd/ralph/` → `runner` → `ExecGitClient`.

**Minimal surface (2 methods only):**
- `HealthCheck` — validates repo state (clean, not detached, not in merge/rebase). Returns sentinel errors for different failure modes
- `HeadCommit` — returns full 40-char hex SHA from `git rev-parse HEAD`. Comparison logic in runner loop, NOT in GitClient

**HeadCommit error semantics (for Stories 3.5/3.6):**
- `HeadCommit` error (git command failed) = **fatal**, propagate error upward. Runner should NOT retry.
- Same SHA before/after execute (`oldSHA == newSHA`) = **"no commit detected"**, trigger retry/resume-extraction (Story 3.6).
- These are distinct paths: error ≠ "no commit". Story 3.5 runner loop must handle both.

**Design note for Story 3.4 (MockGitClient):**
MockGitClient will need to support returning different SHAs on successive `HeadCommit` calls (e.g., sequence `["abc", "abc", "def"]`) to test the retry→detect-commit flow in Story 3.11. The interface itself is stateless; the mock manages call counts internally.

**No logging from package:** ExecGitClient returns errors, never logs. `cmd/ralph/` decides logging level.

### Testing Strategy

**Real git binary approach:** Tests use `t.TempDir()` + `git init` to create real repositories. This is appropriate because:
- Git operations are inherently OS-level subprocess calls
- Mocking `exec.CommandContext` would test the mock, not the implementation
- `t.TempDir()` auto-cleans, no test pollution
- Git is available on CI (GitHub Actions) and locally

**git availability guard:** Start each test with:
```go
if _, err := exec.LookPath("git"); err != nil {
    t.Skip("git not in PATH")
}
```

**Test helper for git repo setup:**
```go
func initGitRepo(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    // git init, configure user, create initial commit
    return dir
}
```
Extract this helper when used ≥2 times (fixture copy pattern from Story 2.7).

**Error testing requirements (from project rules):**
- Every test verifies error with `errors.Is(err, ErrSentinel)` AND `strings.Contains(err.Error(), "substring")`
- Never bare `err != nil` without content check
- `t.Errorf`/`t.Fatalf` in assertions, NEVER `t.Logf`

**WSL/NTFS consideration:** Git tests with `t.TempDir()` run on WSL temp filesystem (not NTFS) so symlink/permission issues are unlikely. If merge conflict setup fails on WSL, use `t.Skipf` with clear documentation (from `.claude/rules/wsl-ntfs.md`).

### Existing Code Patterns to Follow

**exec.CommandContext pattern (session/session.go:56-58):**
```go
cmd := exec.CommandContext(ctx, opts.Command, args...)
cmd.Dir = opts.Dir
```

**Error wrapping with errors.As (session/session.go:72-78):**
```go
var exitErr *exec.ExitError
if errors.As(err, &exitErr) {
    result.ExitCode = exitErr.ExitCode()
    return result, fmt.Errorf("session: claude: exit %d: %w", result.ExitCode, err)
}
```
ExecGitClient should use `errors.As` for `*exec.ExitError` to distinguish exit codes from other errors.

**Table-driven tests (config/constants_test.go, runner/scan_test.go):**
```go
tests := []struct {
    name string
    // ...
}{
    {"scenario name", ...},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

### What NOT to Do

- Do NOT create a separate `git/` package — interface lives in `runner/`
- Do NOT add methods beyond `HealthCheck` and `HeadCommit` — minimal surface
- Do NOT add `IsClean()` or `IsDirtyTree()` — `HealthCheck` returns sentinel errors
- Do NOT implement dirty state recovery (`git checkout -- .`) — that's Story 3.4
- Do NOT add wall-clock timeouts — context propagation only
- Do NOT call `os.Exit` — return errors
- Do NOT log from the package — return errors/results
- Do NOT add `FeedbackPrefix` or task scanning to git.go — wrong package concern
- Do NOT put sentinel errors in `config/errors.go` — they're runner-scoped
- Do NOT use `exec.Command()` without context — always `exec.CommandContext(ctx, ...)`
- Do NOT hardcode `"git"` path — use bare `"git"` (relies on PATH, which is fine for this tool)
- Do NOT define `var update = flag.Bool("update", ...)` in `git_test.go` — it's already declared in `runner/prompt_test.go` (same package, compile error)
- Do NOT use `context.Background()` or `context.TODO()` in production code
- Do NOT use type assertions `err.(*exec.ExitError)` — use `errors.As(err, &target)`
- Do NOT double-wrap errors: if inner function already wraps, don't add another prefix

### Previous Story Intelligence (Story 3.2)

**Patterns from Story 3.2 (Sprint-Tasks Scanner):**
- Review found 6 issues (0H/2M/4L) — key patterns:
  - **M1: Stale doc comments** — when refactoring changes function behavior, update doc comment immediately
  - **M2: Symmetric nil checks** — if testing OpenTasks populated, also assert DoneTasks nil (and vice versa)
  - **L2: Dead struct fields** — don't leave unused fields in test structs
  - **L4: Edge case field assertions** — verify ALL struct fields in edge cases, not just counts
- `ScanTasks` wraps errors as `"runner: scan tasks: %w"` — established package error prefix pattern
- Integration test update: when interface changes, ALL mockGitClient usages must update

**Patterns from Story 3.1 (Execute Prompt Template):**
- Review found 7 issues (0H/2M/5L) — key patterns:
  - **M1: if/else over if/not-if** — use `{{if}}/{{else}}/{{end}}` not duplicate conditionals
  - **L3: Comment accuracy** — doc comments must match reality (counts, "all"/"every" claims)
  - **L4: Missing negative checks** — when testing absence, add explicit negative assertions

### Git Intelligence

Recent commits (last 5):
- `9248eb8` — Story 3.2: Sprint-tasks scanner (runner/scan.go, runner/scan_test.go, runner/runner.go refactored)
- `675a3e4` — Knowledge extraction update (testing patterns, code-review workflow)
- `b6ebc7d` — Story 3.1: Execute prompt template (runner/prompts/execute.md, runner/prompt_test.go)
- `660ccef` — Epic 2 complete (bridge package, 7 stories)
- `ba40438` — Story 2.1: Sprint-tasks format contract

**Relevant patterns from recent work:**
- Files follow `runner/<noun>.go` + `runner/<noun>_test.go` pattern
- Integration tests in `runner/runner_integration_test.go` with `//go:build integration`
- TestMain dispatch for mock claude in integration tests
- `sed -i 's/\r$//'` after every file write (NTFS/WSL)

### Project Structure Notes

- `runner/git.go` — CREATE (GitClient interface + ExecGitClient + sentinel errors)
- `runner/git_test.go` — CREATE (unit tests with real git)
- `runner/runner.go` — MODIFY (remove interface stub, update HasNewCommit → HeadCommit call)
- `runner/runner_integration_test.go` — MODIFY (update mockGitClient struct + all usages)

No new packages. No new dependencies. Alignment with existing `runner/` structure confirmed.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story-3.3 — AC, technical notes, prerequisites]
- [Source: docs/project-context.md#Subprocess-Pattern — exec.CommandContext, cmd.Dir, no wall-clock timeout]
- [Source: docs/project-context.md#Naming-Convention — interfaces in consumer package]
- [Source: runner/runner.go:23-26 — Current GitClient interface stub to replace]
- [Source: runner/runner.go:89-91 — HasNewCommit call site to update]
- [Source: runner/runner_integration_test.go:17-31 — mockGitClient to update]
- [Source: session/session.go:56-58 — exec.CommandContext pattern to follow]
- [Source: session/session.go:72-78 — errors.As pattern for exec.ExitError]
- [Source: config/errors.go — Sentinel error pattern (ErrNoTasks)]
- [Source: docs/sprint-artifacts/3-2-sprint-tasks-scanner.md — Previous story dev notes, review patterns]
- [Source: docs/sprint-artifacts/3-1-execute-prompt-template.md — Previous story dev notes, review patterns]
- [Source: .claude/rules/go-testing-patterns.md — Error testing, assertion, test structure patterns]
- [Source: .claude/rules/wsl-ntfs.md — WSL/NTFS patterns, git binary, t.Skipf for platform issues]

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Created `runner/git.go` with GitClient interface (2 methods: HealthCheck, HeadCommit), 3 sentinel errors (ErrDirtyTree, ErrDetachedHead, ErrMergeInProgress), and ExecGitClient struct implementation
- HealthCheck order: merge/rebase check → detached HEAD → dirty tree (most-specific-first to distinguish merge conflict from regular dirty tree)
- All git commands use `exec.CommandContext(ctx)` with `cmd.Dir = e.Dir` — no exec.Command without context
- Created `runner/git_test.go` with 10 test functions using real git repos via `t.TempDir()`, `initGitRepo`, and `runGit` helpers
- Refactored `runner/runner.go`: removed GitClient interface stub, updated `HasNewCommit` → `HeadCommit` call with new error message `"runner: head commit:"`
- Refactored `runner/runner_integration_test.go`: updated mockGitClient struct and all 8 usages, renamed test from `HasNewCommitFails` → `HeadCommitFails`
- All 10 ExecGitClient tests pass, full regression (all packages) passes, `go vet` clean

### Change Log

- 2026-02-26: Story 3.3 implementation — GitClient interface + ExecGitClient with full test coverage
- 2026-02-26: Code review fixes — 6 findings (0H/3M/3L): extracted runGit helper, fixed hardcoded "master", added rebase test, fixed stale comments

### File List

- runner/git.go (NEW) — GitClient interface, sentinel errors, ExecGitClient implementation
- runner/git_test.go (NEW) — 10 unit tests with real git repos, runGit helper
- runner/runner.go (MODIFIED) — Removed GitClient interface stub, updated HasNewCommit → HeadCommit, fixed doc comment
- runner/runner_integration_test.go (MODIFIED) — Updated mockGitClient to new interface, renamed test, fixed stale comment
- docs/sprint-artifacts/sprint-status.yaml (MODIFIED) — Story 3-3 status: ready-for-dev → in-progress → review → done
- docs/sprint-artifacts/3-3-gitclient-interface-execgitclient.md (MODIFIED) — Story file itself
