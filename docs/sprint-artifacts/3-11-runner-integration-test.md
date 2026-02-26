# Story 3.11: Runner Integration Test

Status: Done

## Story

As a developer,
I want comprehensive integration tests for Runner.Execute that exercise the full flow through scenario-based MockClaude and MockGitClient,
so that all runner components work correctly together and regressions are caught.

## Acceptance Criteria

```gherkin
Scenario: Happy path — all tasks complete (AC1)
  Given scenario JSON with 3 execute steps (all produce commits)
  And MockGitClient returns health OK + commit sequence
  And sprint-tasks.md fixture with 3 open tasks
  When runner.Run executes
  Then 3 sessions launched sequentially
  And runner exits with code 0

Scenario: Retry + resume-extraction flow (AC2)
  Given scenario JSON: execute (no commit) → resume-extraction → execute (commit)
  And MockGitClient returns no-commit then commit
  When runner.Run executes
  Then execute_attempts increments to 1
  And resume-extraction invoked with session_id
  And retry succeeds with commit

Scenario: Emergency stop on max retries (AC3)
  Given scenario JSON: 3 executes all without commit
  And max_iterations = 3
  When runner.Run executes
  Then execute_attempts reaches 3
  And runner returns ErrMaxRetries
  And error contains task info

Scenario: Resume on re-run after partial completion (AC4)
  Given sprint-tasks.md with 2 completed + 1 open task
  And scenario JSON for 1 execute (commit)
  When runner.Run executes
  Then starts from task 3 (first open)
  And completes successfully

Scenario: Dirty tree recovery at startup (AC5)
  Given MockGitClient.HealthCheck returns ErrDirtyTree
  When runner starts
  Then recovery executed (git checkout -- .)
  And then proceeds normally

Scenario: Resume-extraction failure triggers recovery (AC6)
  Given scenario JSON: execute (no commit) → resume-extraction (exit code non-zero)
  And MockGitClient shows dirty tree after failed resume
  When runner processes resume-extraction failure
  Then triggers dirty state recovery (git checkout -- .)
  And retries execute from clean state
  And execute_attempts is correctly tracked through the recovery

Scenario: Uses bridge golden file output as input (AC7)
  Given sprint-tasks.md from bridge golden file testdata (not hand-crafted)
  When used as runner test input
  Then validates bridge→runner data contract
  And proves end-to-end compatibility

Scenario: Test isolation (AC8)
  Given integration test
  When test runs
  Then uses t.TempDir() for all file operations
  And no shared state between test cases
  And build tag //go:build integration
```

## Tasks / Subtasks

- [x] Task 1: Create test infrastructure for Runner.Execute integration tests (AC: 8)
  - [x] 1.1 Add `//go:build integration` build tag at top of new test functions (use existing `runner_integration_test.go` file)
  - [x] 1.2 Create helper `setupRunnerIntegration(t, tmpDir, tasksContent string, scenario testutil.Scenario, git *testutil.MockGitClient) (*Runner, string)` that returns Runner + stateDir. All 7 tests share identical boilerplate — helper is justified per DRY
  - [x] 1.3 Helper MUST set ALL Runner fields: Cfg (via testConfig), Git, TasksFile (via writeTasksFile), ReviewFn (cleanReviewFn default — tests override), ResumeExtractFn (noopResumeExtractFn default — tests override), SleepFn (noopSleepFn), Knowledge (NoOpKnowledgeWriter) — prevents nil-pointer panics (Story 3.6 learning)
  - [x] 1.4 Reuse existing helpers: `writeTasksFile`, `headCommitPairs`, `testConfig`, `noopSleepFn`, `noopResumeExtractFn`, `cleanReviewFn`, `copyFixtureToDir`

- [x] Task 2: Happy path — all tasks complete (AC: 1, 8)
  - [x] 2.1 Test name: `TestRunner_Execute_Integration_HappyPath`
  - [x] 2.2 Setup: 3 open tasks in sprint-tasks.md, 3 MockClaude scenario steps (all exit 0, each with unique session_id), MockGitClient with 3 HeadCommit pairs (different SHAs = commit detected), HealthCheck OK
  - [x] 2.3 ReviewFn: use counting closure + MaxIterations=3 (same pattern as TestRunner_Execute_SequentialExecution unit test — processes first open task 3 times without marking done, outer loop exits when i reaches MaxIterations)
  - [x] 2.4 Assert: no error, HealthCheckCount=1, HeadCommitCount=6 (3 pairs), review called 3 times
  - [x] 2.5 Verify via `testutil.ReadInvocationArgs` that each execute session got correct `-p` prompt with task content and `--max-turns` flag

- [x] Task 3: Retry + resume-extraction flow (AC: 2)
  - [x] 3.1 Test name: `TestRunner_Execute_Integration_RetryWithResume`
  - [x] 3.2 Setup: 3 open tasks, MaxIterations=3, 2 MockClaude scenario steps (1st exit 0, 2nd exit 0), MockGitClient HeadCommit pairs: [aaa,aaa] (same=no commit), then [aaa,bbb] (different=commit)
  - [x] 3.3 ResumeExtractFn: `trackingResumeExtract` closure that records session_id and call count
  - [x] 3.4 ReviewFn: reviewAndMarkDoneFn (mark all tasks done after successful retry)
  - [x] 3.5 Assert: no error, resume called once with session_id "retry-1", HeadCommitCount=4, SleepFn called once

- [x] Task 4: Emergency stop on max retries (AC: 3)
  - [x] 4.1 Test name: `TestRunner_Execute_Integration_MaxRetriesEmergencyStop`
  - [x] 4.2 Setup: 3 open tasks, max_iterations=3, 3 MockClaude scenario steps (all exit 0), MockGitClient HeadCommit: 3 pairs all same SHA (no commit)
  - [x] 4.3 Assert: `errors.Is(err, config.ErrMaxRetries)`, `strings.Contains(err.Error(), "execute attempts exhausted")`, `strings.Contains(err.Error(), "Task one")`
  - [x] 4.4 Verify resume called 2 times, sleep called 2 times (last attempt triggers emergency stop BEFORE resume/sleep)

- [x] Task 5: Resume on re-run after partial completion (AC: 4)
  - [x] 5.1 Test name: `TestRunner_Execute_Integration_ResumeAfterPartialCompletion`
  - [x] 5.2 Setup: sprint-tasks.md with 2 `[x]` + 1 `[ ]` tasks, 1 MockClaude scenario step (exit 0), MockGitClient: HealthCheck OK, 1 HeadCommit pair (different=commit)
  - [x] 5.3 ReviewFn: reviewAndMarkDoneFn (mark last task done)
  - [x] 5.4 Assert: no error, only 1 session launched (not 3), HeadCommitCount=2

- [x] Task 6: Dirty tree recovery at startup (AC: 5)
  - [x] 6.1 Test name: `TestRunner_Execute_Integration_DirtyTreeRecovery`
  - [x] 6.2 Setup: 3 open tasks, MockGitClient: HealthCheck returns ErrDirtyTree first call then OK, RestoreClean OK, HeadCommit pair (commit), 1 MockClaude step (exit 0)
  - [x] 6.3 ReviewFn: reviewAndMarkDoneFn (mark task done)
  - [x] 6.4 Assert: no error, RestoreCleanCount=1, HealthCheckCount=1

- [x] Task 7: Resume-extraction failure triggers recovery (AC: 6)
  - [x] 7.1 Test name: `TestRunner_Execute_Integration_ResumeFailureRecovery`
  - [x] 7.2 Setup: 3 open tasks, max_iterations=3, MockClaude: 2 steps, MockGitClient: HeadCommit [aaa,aaa, aaa,bbb], HealthCheck: OK(startup) → ErrDirtyTree(after resume) → OK(beyond)
  - [x] 7.3 ResumeExtractFn: returns nil — resume "failure" simulated by dirty tree side effect on MockGitClient. RecoverDirtyState detects dirty tree and calls RestoreClean
  - [x] 7.4 Assert: no error (retry succeeds), RestoreCleanCount=1, resume called once, HealthCheckCount ≥ 2

- [x] Task 8: Bridge golden file as input (AC: 7)
  - [x] 8.1 Test name: `TestRunner_Execute_Integration_BridgeGoldenFileContract`
  - [x] 8.2 Read `bridge/testdata/TestBridge_MergeWithCompleted.golden` via os.ReadFile and pass content to setupRunnerIntegration (has 2 `[x]` + 3 `[ ]`)
  - [x] 8.3 Setup: MaxIterations=3, 3 MockClaude steps (all exit 0), MockGitClient: 3 HeadCommit pairs (all different = commit)
  - [x] 8.4 ReviewFn: cleanReviewFn (same pattern as happy path — MaxIterations=3 processes first open task 3 times, outer loop exits at i=3)
  - [x] 8.5 Assert: no error, HeadCommitCount=6 (3 sessions). Scanner works correctly DESPITE extra lines

- [x] Task 9: Run full test suite (AC: all)
  - [x] 9.1 `go test ./runner/ -tags=integration` — all 15 tests pass (8 existing + 7 new)
  - [x] 9.2 `go test ./...` — no regressions in unit tests (all packages pass)

## Dev Notes

### Architecture Constraints

- **Build tag**: `//go:build integration` — already present in `runner_integration_test.go`
- **File**: add new tests to existing `runner/runner_integration_test.go` (do NOT create new file)
- **Dependency direction**: `runner` test → `config`, `session`, `internal/testutil` (unchanged)
- **Test isolation**: `t.TempDir()` for each test, no shared state
- **Mock Claude self-reexec**: `TestMain` in `runner/testmain_test.go` (separate file!) dispatches via `testutil.RunMockClaude()`. Do NOT add another TestMain — Go allows only one per package
- **Error wrapping verification**: `errors.Is` for sentinels, `strings.Contains` for message content
- **MaxIterations dual usage**: controls BOTH outer task loop iterations (runner.go:125) AND inner executeAttempts limit per review cycle (runner.go:209). Set carefully per test scenario
- **HeadCommit(after) only on success**: on ExitError, HeadCommit(after) is SKIPPED (runner.go:197 is in else branch). HeadCommitCount = 2 per successful attempt, 1 per ExitError attempt

### KISS/DRY/SRP Analysis

**KISS:**
- Each test = one scenario, one `Runner.Execute` call, clear assertions
- No test framework abstractions beyond existing helpers
- No shared test state or test ordering dependencies

**DRY:**
- Reuse ALL existing test helpers: `writeTasksFile`, `headCommitPairs`, `testConfig`, `noopSleepFn`, `noopResumeExtractFn`, `cleanReviewFn`, `reviewAndMarkDoneFn`, `trackingResumeExtract`, `trackingSleep`
- Extract `setupRunnerIntegration` helper — all 7 tests share identical Runner construction boilerplate (DRY justified)
- Do NOT duplicate unit test coverage — integration tests verify component interaction, not individual error paths

**SRP:**
- Integration tests verify components work together (runner → session → git → scan → knowledge)
- Unit tests already cover individual component error paths
- No overlap: integration tests use real `session.Execute` → MockClaude subprocess, unit tests use mock functions

### What Integration Tests Add Over Unit Tests

Unit tests mock `session.Execute` at function level — they verify runner logic with injected mock responses. Integration tests use **real `session.Execute`** with a **MockClaude subprocess** — they verify:
1. CLI flag assembly (`-p`, `--max-turns`, `--model`, `--resume`) reaches MockClaude correctly
2. JSON output parsing from real subprocess stdout works end-to-end
3. Prompt assembly includes correct task content from scanned sprint-tasks.md
4. File I/O chain: read tasks → scan → assemble prompt → write to session → parse output
5. Bridge golden file compatibility — real bridge output is parseable by runner's scanner

### Existing Integration Test File Structure

Current `runner_integration_test.go` (296 lines) contains:
- 7 `TestRunOnce_WalkingSkeleton_*` tests (RunOnce single-step flow)
- 1 `TestRunReview_WalkingSkeleton_SessionFails` test
- NOTE: `TestMain` is in separate `runner/testmain_test.go` file (handles MOCK_EXIT_EMPTY + RunMockClaude dispatch). Do NOT add another TestMain

New tests add `TestRunner_Execute_Integration_*` — different naming prefix, clear separation.

### Mock Claude Scenario Setup Pattern

```go
scenario := testutil.Scenario{
    Name: "descriptive-name",
    Steps: []testutil.ScenarioStep{
        {Type: "execute", ExitCode: 0, SessionID: "sess-1"},
        {Type: "execute", ExitCode: 0, SessionID: "sess-2"},
    },
}
stateDir := testutil.SetupMockClaude(t, scenario)
// Later: args := testutil.ReadInvocationArgs(t, stateDir, 0)
```

### MockGitClient Sequence Pattern

```go
git := &testutil.MockGitClient{
    HeadCommits: headCommitPairs(
        [2]string{"aaa", "bbb"}, // iteration 1: commit detected
        [2]string{"bbb", "ccc"}, // iteration 2: commit detected
    ),
}
// HealthCheckCount, HeadCommitCount, RestoreCleanCount tracked automatically
```

### Bridge Golden File Contract (AC7)

Use `bridge/testdata/TestBridge_MergeWithCompleted.golden` — contains:
```markdown
## Epic: Authentication

- [x] Implement user login endpoint
  source: stories/auth.md#AC-1
- [x] Add input validation for login
  source: stories/auth.md#AC-2
- [ ] Implement password reset flow
  source: stories/auth.md#AC-3
- [ ] Add two-factor authentication
  source: stories/auth.md#AC-4
- [ ] Create session management service
  source: stories/auth.md#AC-5
```

Scanner should find: 3 open tasks, 2 done tasks. Runner starts from first `- [ ]` task.
Scanner IGNORES `source:` annotation lines and `## Epic:` headers — they don't match task regex patterns. Test validates that bridge output format with these extra lines doesn't break scan, and mixed `[x]`/`[ ]` markers are correctly categorized.

### Runner.Execute Flow Summary (for test planning)

```
Execute(ctx)
├─ RecoverDirtyState()              ← AC5: dirty tree recovery (HealthCheck once, no re-check after restore)
├─ for i := 0; i < MaxIterations; i++    ← MaxIterations = outer loop limit AND retry limit (dual usage!)
│  ├─ ReadFile(tasks) → ScanTasks()
│  ├─ if !HasOpenTasks → return nil  ← AC4: partial completion
│  ├─ AssemblePrompt()
│  ├─ reviewCycles := 0
│  └─ for { // review cycle
│     ├─ executeAttempts := 0
│     └─ for { // retry loop
│        ├─ HeadCommit(before)       ← always called
│        ├─ session.Execute()        ← MockClaude subprocess
│        ├─ if ExitError → needsRetry=true, HeadCommit(after) SKIPPED
│        ├─ else → HeadCommit(after) ← only on success (runner.go:197)
│        ├─ if needsRetry (no commit OR ExitError):
│        │  ├─ attempts++            ← FIRST (line 208)
│        │  ├─ if >= max → error     ← SECOND — AC3: emergency stop (resume/sleep NOT called)
│        │  ├─ ResumeExtractFn()     ← AFTER max check — AC6: resume failure
│        │  ├─ RecoverDirtyState()   ← HealthCheck + optional RestoreClean
│        │  └─ SleepFn(backoff)      ← exponential: 1s, 2s, 4s...
│        └─ break (commit detected)
│     ├─ ReviewFn()
│     └─ if Clean → break            ← AC1: happy path
```

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `testConfig(tmpDir, maxIter)` | test_helpers_test.go:140 | Config with standard defaults |
| `cleanReviewFn` | test_helpers_test.go:39 | Clean review stub |
| `fatalReviewFn(t)` | test_helpers_test.go:44 | Review that fails test if called |
| `noopResumeExtractFn` | test_helpers_test.go:179 | No-op resume extract |
| `noopSleepFn` | test_helpers_test.go:176 | No-op sleep |
| `writeTasksFile(t, dir, content)` | test_helpers_test.go:55 | Write sprint-tasks.md |
| `headCommitPairs(pairs...)` | test_helpers_test.go:66 | Generate HeadCommit mock sequences |
| `reviewAndMarkDoneFn(path, counter)` | test_helpers_test.go:201 | Review + mark ALL tasks done at once (writes `allDoneTasks`) |
| `copyFixtureToDir(t, tmpDir, fixture)` | test_helpers_test.go | Copy testdata fixture to temp dir |
| `trackingResumeExtract` | test_helpers_test.go | Tracks ResumeExtractFn calls |
| `trackingSleep` | test_helpers_test.go | Tracks SleepFn calls |
| `threeOpenTasks` constant | test_helpers_test.go:16 | Three open tasks content |
| `allDoneTasks` constant | test_helpers_test.go:25 | All done tasks content |

### Existing Sentinel Errors (do NOT duplicate)

- `config.ErrMaxRetries` — retry exhaustion (cross-package)
- `config.ErrMaxReviewCycles` — review cycle exhaustion (cross-package)
- `config.ErrNoTasks` — no tasks found (cross-package)
- `runner.ErrDirtyTree` — dirty working tree
- `runner.ErrDetachedHead` — detached HEAD
- `runner.ErrMergeInProgress` — merge/rebase in progress

### Previous Story Intelligence (Story 3.10)

Key learnings applied:
- **Initialize ALL injectable fields** on test Runner structs — prevents nil-pointer panics (Story 3.6 M5)
- **Inner error ≠ outer prefix** in assertions — use unique inner cause string (Story 3.7 M3)
- **Call count assertions** for ALL mocks — HeadCommitCount, HealthCheckCount, RestoreCleanCount, resume count, sleep count (Story 3.4 M4)
- **gofmt after Edit** — run `go fmt` on modified files (Story 3.9 M1)
- **sed CRLF** — run `sed -i 's/\r$//'` after Write on NTFS (CLAUDE.md)
- **Scenario.Name field** — always set on `testutil.Scenario` structs for debugging (Story 3.4 learning)

### Git Intelligence (Recent Commits)

```
b5624bd Story 3.10 — Emergency stop review cycles (FR24)
8276c57 Stories 3.6-3.8 — retry, resume extraction, startup recovery
3584c6b Story 3.5 — Runner loop skeleton
16fc58f Stories 3.3+3.4 — GitClient, MockGitClient, dirty state recovery
```

All runner infrastructure is in place. No new types or interfaces needed — only test code.

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner_integration_test.go` | Add 7 new `TestRunner_Execute_Integration_*` test functions |
| `runner/test_helpers_test.go` | Add `setupRunnerIntegration` helper (all 7 tests share boilerplate — DRY justified) |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `bridge/testdata/TestBridge_MergeWithCompleted.golden` | Copy as sprint-tasks.md input (AC7) |
| `runner/runner.go` | Reference for Execute flow |
| `internal/testutil/mock_claude.go` | Scenario setup API |
| `internal/testutil/mock_git.go` | MockGitClient API |

**No new files** — all changes in existing test files.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.11] — AC and technical requirements
- [Source: runner/runner_integration_test.go:1-296] — Existing integration tests (RunOnce walking skeleton)
- [Source: runner/runner.go:113-250] — Execute method (flow under test)
- [Source: runner/runner_test.go:1-2122] — Unit tests (do NOT duplicate coverage)
- [Source: runner/test_helpers_test.go:1-211] — Test helpers to reuse
- [Source: internal/testutil/mock_claude.go] — MockClaude scenario infrastructure
- [Source: internal/testutil/mock_git.go] — MockGitClient implementation
- [Source: bridge/testdata/TestBridge_MergeWithCompleted.golden] — Bridge golden file for AC7
- [Source: config/shared/sprint-tasks-format.md] — Sprint-tasks format contract
- [Source: .claude/rules/go-testing-patterns.md] — 50+ testing patterns
- [Source: runner/testmain_test.go] — TestMain with MOCK_EXIT_EMPTY + RunMockClaude dispatch (do NOT duplicate)
- [Source: docs/sprint-artifacts/3-10-emergency-stop-review-cycles-trigger-point.md] — Previous story intelligence

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Implementation Plan

All 7 integration tests use the same pattern: setup Runner via `setupRunnerIntegration` helper → configure mock dependencies → call `r.Execute(ctx)` → assert results and mock call counts.

Focus areas per user request: **SRP** (each test = one scenario, no overlap with unit tests), **YAGNI** (no new types/interfaces/files, no extra assertions beyond AC), **DRY** (all 7 tests share `setupRunnerIntegration` helper, reuse 10+ existing helpers).

### Completion Notes List

- Added `setupRunnerIntegration` helper to `test_helpers_test.go` — all 7 tests share identical Runner construction boilerplate (DRY justified). Helper sets ALL Runner fields including Knowledge (Story 3.6 learning).
- Added `testutil` import to `test_helpers_test.go` (needed for Scenario and MockGitClient types).
- 7 new integration tests in `runner_integration_test.go`: HappyPath, RetryWithResume, MaxRetriesEmergencyStop, ResumeAfterPartialCompletion, DirtyTreeRecovery, ResumeFailureRecovery, BridgeGoldenFileContract.
- AC6 implementation note: ResumeExtractFn returns nil (not error) — the "resume failure" is simulated by MockGitClient returning ErrDirtyTree on next HealthCheck, which triggers RecoverDirtyState recovery. The current code (runner.go:214-216) fatally returns on ResumeExtractFn error, so the test validates the recovery pipeline through dirty tree side effect instead.
- AC7: Bridge golden file read via `os.ReadFile("../bridge/testdata/...")` and passed as content to setupRunnerIntegration. Validates source: annotations and ## Epic: headers are ignored by scanner.
- All tests use t.TempDir(), no shared state, //go:build integration tag (AC8).

### Change Log

- 2026-02-26: Implemented Story 3.11 — 7 Runner.Execute integration tests covering happy path, retry+resume, emergency stop, partial completion resume, dirty tree recovery, resume failure recovery, and bridge golden file contract.
- 2026-02-26: Code review — 4 findings (0H/2M/2L), all 4 fixed: M1 inner error assertion for max retries, M2 prompt content verification via ReadInvocationArgs, L1 SleepFn tracking in resume failure test, L2 backoff duration assertion in retry test.

### File List

- `runner/runner_integration_test.go` (modified) — added 7 `TestRunner_Execute_Integration_*` test functions
- `runner/test_helpers_test.go` (modified) — added `setupRunnerIntegration` helper, added `testutil` import
- `docs/sprint-artifacts/sprint-status.yaml` (modified) — story status: ready-for-dev → in-progress → review
- `docs/sprint-artifacts/3-11-runner-integration-test.md` (modified) — tasks marked [x], Dev Agent Record, File List, Change Log, Status
