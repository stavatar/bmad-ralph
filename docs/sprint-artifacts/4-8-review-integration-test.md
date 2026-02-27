# Story 4.8: Review Integration Test

Status: Ready for Review

## Story

As a developer,
I want comprehensive integration tests for the review pipeline covering clean review, findings cycle, fix cycle, and emergency stop,
so that review correctness is guaranteed before v0.1 release.

## Acceptance Criteria

```gherkin
Scenario: Clean review — single pass (AC1)
  Given scenario JSON: execute (commit) -> review (clean)
  And MockGitClient returns commit after execute
  When runner.Execute runs single task
  Then 1 execute + 1 review session launched
  And task marked [x]
  And review-findings.md absent or empty
  And exit code 0

Scenario: Findings -> fix -> clean review (AC2)
  Given scenario JSON: execute (commit) -> review (findings) -> execute (commit) -> review (clean)
  And review-findings.md created after first review
  When runner.Execute runs
  Then 2 execute + 2 review sessions
  And review_cycles = 1 after first findings, 0 after clean
  And task marked [x] at end

Scenario: Emergency stop on max review cycles (AC3)
  Given scenario JSON: 3 cycles of execute (commit) -> review (findings)
  And max_review_iterations = 3
  When runner.Execute runs
  Then review_cycles reaches 3
  And runner returns ErrMaxReviewCycles
  And error contains task info + cycles count

Scenario: Multi-task with mixed outcomes (AC4)
  Given sprint-tasks.md with 3 tasks
  And scenario JSON: task1 (clean), task2 (1 fix cycle), task3 (clean)
  When runner.Execute runs
  Then all 3 tasks marked [x]
  And review_cycles properly reset between tasks

Scenario: Review determines outcome via file state (AC5)
  Given mock review session that writes [x] to sprint-tasks.md
  And mock review session that writes review-findings.md
  When runner checks outcome
  Then correctly determines clean vs findings from file state
  And does not parse Claude session output

Scenario: Bridge golden file as input (AC6)
  Given sprint-tasks.md from bridge golden file testdata (Story 2.5)
  When used as runner test input
  Then validates full pipeline: bridge output -> execute -> review

Scenario: v0.1 smoke test note (AC7)
  Given all integration tests pass
  When considering release readiness
  Then manual smoke test with real Claude CLI recommended before v0.1 tag
  And documented in test file comments

Scenario: Manual prompt validation checklist documented (AC8)
  Given review prompt from 4.1 and sub-agent prompts from 4.2
  When manual testing before v0.1 tag
  Then checklist covers: (1) planted bug detected by correct sub-agent
  And (2) clean code yields no false positives
  And (3) findings contain all 4 fields
  And (4) clean review produces [x] + empty findings
  And (5) findings review does NOT mark [x]
  And checklist is in runner/testdata/manual_smoke_checklist.md
```

## Tasks / Subtasks

- [x] Task 1: Export RealReview function for test access (AC: all)
  - [x] 1.1 In `runner/runner.go`, rename `realReview` to `RealReview` (uppercase R). This exports the function so `runner_test` package can wire it as `ReviewFn`
  - [x] 1.2 Update `Run()` (runner.go:478) to reference `RealReview` instead of `realReview`
  - [x] 1.3 Update doc comment on `RealReview` to note: "Exported for integration testing (Story 4.8). Production wiring via Run()"
  - [x] 1.4 Verify: `go build ./...` passes, existing tests unaffected

- [x] Task 2: Extend MockClaude with file-writing side effects (AC: 5)
  - [x] 2.1 In `internal/testutil/mock_claude.go`, add fields to `ScenarioStep`:
    - `WriteFiles map[string]string` — relative path to project root -> content to write
    - `DeleteFiles []string` — relative paths to remove
  - [x] 2.2 In `RunMockClaude`, after writing JSON output to stdout and before incrementing counter: read `MOCK_CLAUDE_PROJECT_ROOT` env var. If set and step has WriteFiles/DeleteFiles, apply file side effects
  - [x] 2.3 WriteFiles: `os.WriteFile(filepath.Join(projectRoot, relPath), []byte(content), 0644)` for each entry
  - [x] 2.4 DeleteFiles: `os.Remove(filepath.Join(projectRoot, relPath))` for each entry. Ignore `os.ErrNotExist` (idempotent)
  - [x] 2.5 Add unit test `TestRunMockClaude_WriteFiles` in `internal/testutil/mock_claude_test.go` — verify file side effects applied when `MOCK_CLAUDE_PROJECT_ROOT` is set
  - [x] 2.6 Update `TestScenarioStep_ZeroValue` in `mock_claude_test.go` — add assertions for `WriteFiles == nil` and `len(DeleteFiles) == 0` (zero-value coverage for new fields)
  - [x] 2.7 Verify: existing MockClaude tests pass (no regression from new optional fields)

- [x] Task 3: Create setupReviewIntegration helper (AC: all, DRY)
  - [x] 3.1 In `runner/test_helpers_test.go`, add `setupReviewIntegration(t, tmpDir, tasksContent string, scenario testutil.Scenario, git *testutil.MockGitClient) (*runner.Runner, string)` helper
  - [x] 3.2 Same as `setupRunnerIntegration` but: (a) sets `ReviewFn = runner.RealReview`, (b) calls `t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)` so MockClaude subprocess can write file side effects
  - [x] 3.3 Returns (Runner, stateDir) — same as setupRunnerIntegration
  - [x] 3.4 Doc comment: "setupReviewIntegration creates a Runner with RealReview for full review pipeline integration tests. Unlike setupRunnerIntegration, uses real review session via MockClaude subprocess with file side effects."

- [x] Task 4: Test clean review — single pass (AC: 1, 5, 7)
  - [x] 4.1 Test name: `TestRunner_Execute_ReviewIntegration_CleanReview` in `runner/runner_review_integration_test.go` with `//go:build integration` tag
  - [x] 4.2 Setup: 1 open task (`"- [ ] Implement feature X"`), scenario with 2 steps:
    - Step 0: `{Type: "execute", ExitCode: 0, SessionID: "exec-1"}` — execute session
    - Step 1: `{Type: "review", ExitCode: 0, SessionID: "rev-1", WriteFiles: {"sprint-tasks.md": markedContent}, DeleteFiles: ["review-findings.md"]}` — clean review writes [x] + clears findings
  - [x] 4.3 MockGitClient: HeadCommits `headCommitPairs([2]string{"aaa", "bbb"})` — 1 commit detected
  - [x] 4.4 Assert: no error, HeadCommitCount == 2 (1 execute session), v0.1 smoke test comment
  - [x] 4.5 Verify via `testutil.ReadInvocationArgs(t, stateDir, 0)` that execute session got `-p` prompt with execute template content
  - [x] 4.6 Verify via `testutil.ReadInvocationArgs(t, stateDir, 1)` that review session got `-p` prompt with review content (task text via __TASK_CONTENT__)
  - [x] 4.7 Verify final sprint-tasks.md has `[x]` on the task
  - [x] 4.8 Verify review-findings.md absent (deleted by review side effect)

- [x] Task 5: Test findings -> fix -> clean review (AC: 2, 5)
  - [x] 5.1 Test name: `TestRunner_Execute_ReviewIntegration_FindingsFixClean`
  - [x] 5.2 Setup: 1 open task, scenario with 4 steps:
    - Step 0: execute (commit)
    - Step 1: review (findings) — WriteFiles: {"review-findings.md": findingsContent}
    - Step 2: execute (commit) — fix cycle
    - Step 3: review (clean) — WriteFiles: {"sprint-tasks.md": markedContent}, DeleteFiles: ["review-findings.md"]
  - [x] 5.3 MockGitClient: 2 HeadCommit pairs (2 execute sessions, 2 commits)
  - [x] 5.4 Assert: no error, HeadCommitCount == 4 (2 execute sessions)
  - [x] 5.5 Verify step 2 (fix execute) prompt contains findings content via ReadInvocationArgs — this validates Story 4.7's findings injection
  - [x] 5.6 Verify final sprint-tasks.md has [x], review-findings.md absent

- [x] Task 6: Test emergency stop on max review cycles (AC: 3)
  - [x] 6.1 Test name: `TestRunner_Execute_ReviewIntegration_MaxReviewCycles`
  - [x] 6.2 Setup: 1 open task, `MaxReviewIterations = 3`, scenario with 6 steps:
    - 3 cycles of: execute (commit) + review (findings)
  - [x] 6.3 MockGitClient: 3 HeadCommit pairs (3 execute sessions)
  - [x] 6.4 Assert: `errors.Is(err, config.ErrMaxReviewCycles)`, `strings.Contains(err.Error(), "review cycles exhausted")`, `strings.Contains(err.Error(), task name)`
  - [x] 6.5 Verify 3 execute + 3 review sessions via step count (6 total MockClaude invocations)

- [x] Task 7: Test multi-task with mixed outcomes (AC: 4)
  - [x] 7.1 Test name: `TestRunner_Execute_ReviewIntegration_MultiTaskMixed`
  - [x] 7.2 Setup: 3 open tasks, scenario with 8 steps:
    - Task 1: execute + review (clean) — 2 steps
    - Task 2: execute + review (findings) + execute + review (clean) — 4 steps
    - Task 3: execute + review (clean) — 2 steps
  - [x] 7.3 Each review step's WriteFiles must write the correct incremental [x] state of sprint-tasks.md
  - [x] 7.4 MockGitClient: 4 HeadCommit pairs (4 execute sessions: 1 + 2 + 1)
  - [x] 7.5 Assert: no error, all 3 tasks marked [x] in final sprint-tasks.md
  - [x] 7.6 Assert HeadCommitCount == 8 (4 execute sessions x 2 calls each)

- [x] Task 8: Test bridge golden file as input (AC: 6)
  - [x] 8.1 Test name: `TestRunner_Execute_ReviewIntegration_BridgeGoldenFile`
  - [x] 8.2 Read `bridge/testdata/TestBridge_MergeWithCompleted.golden` — has 2 [x] + 3 [ ] tasks
  - [x] 8.3 Setup: scenario for 3 open tasks with clean reviews (6 steps: 3x execute + 3x review)
  - [x] 8.4 Each review step writes [x] for the next completed task
  - [x] 8.5 Assert: no error, validates bridge->runner->review data contract end-to-end

- [x] Task 9: Create manual smoke test checklist (AC: 7, 8)
  - [x] 9.1 Create `runner/testdata/manual_smoke_checklist.md` with v0.1 pre-release checklist
  - [x] 9.2 Checklist items from AC8: (1) planted bug detection, (2) false positive resistance, (3) findings structure, (4) clean review behavior, (5) non-clean review behavior
  - [x] 9.3 Add test file comment referencing checklist location

- [x] Task 10: Run full test suite (AC: all)
  - [x] 10.1 `go test ./internal/testutil/` — MockClaude extension tests pass
  - [x] 10.2 `go test ./runner/` — all unit tests pass (including RealReview rename)
  - [x] 10.3 `go test ./runner/ -tags=integration` — all integration tests pass (old + new)
  - [x] 10.4 `go test ./...` — no regressions
  - [x] 10.5 `go build ./...` — clean build

## Dev Notes

### Architecture Constraints

- **Build tag**: `//go:build integration` — same as Story 3.11 integration tests
- **Test file**: `runner/runner_review_integration_test.go` (SEPARATE from Story 3.11's `runner_integration_test.go`)
- **TestMain**: already in `runner/testmain_test.go` — handles MOCK_EXIT_EMPTY + RunMockClaude dispatch. Do NOT add another TestMain
- **Dependency direction**: `runner` test -> `config`, `session`, `internal/testutil` (unchanged)
- **Config immutability**: `r.Cfg` read-only, as always
- **Error wrapping verification**: `errors.Is` for sentinels, `strings.Contains` for message content

### Key Design Decision: Export RealReview

`realReview` (runner.go:70) is currently unexported. Integration tests in `runner_test` package cannot access it. Renaming to `RealReview` (uppercase) exports it for test wiring:

```go
r.ReviewFn = runner.RealReview
```

This is the minimal change needed. Alternative approaches (internal test package, RunReview-style wrapper) add complexity without benefit. `RealReview` is wired by `Run()` in production, by tests directly for integration testing.

**Exported API surface note**: project context says ">10 exports = SRP violation". Runner currently exports ~8 functions. Adding `RealReview` keeps it under limit.

### Key Design Decision: MockClaude File Side Effects

`realReview` calls `session.Execute` (MockClaude subprocess) then `DetermineReviewOutcome` (checks file state). For DetermineReviewOutcome to return correct results, MockClaude must write files during review steps.

Extension to `ScenarioStep`:
```go
type ScenarioStep struct {
    // ... existing fields ...
    WriteFiles  map[string]string `json:"write_files,omitempty"`  // relPath → content
    DeleteFiles []string          `json:"delete_files,omitempty"` // relPaths to delete
}
```

Extension to `RunMockClaude`:
```go
// After writing JSON output, before incrementing counter:
projectRoot := os.Getenv("MOCK_CLAUDE_PROJECT_ROOT")
if projectRoot != "" {
    for relPath, content := range step.WriteFiles {
        os.WriteFile(filepath.Join(projectRoot, relPath), []byte(content), 0644)
    }
    for _, relPath := range step.DeleteFiles {
        os.Remove(filepath.Join(projectRoot, relPath)) // ignore ErrNotExist
    }
}
```

Tests set `t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)` so MockClaude subprocess inherits the env var and can write to the test's tmpDir.

**Flat paths only**: `WriteFiles` uses `os.WriteFile` without `os.MkdirAll`, so `relPath` must not contain subdirectories. All current scenarios use top-level files (`sprint-tasks.md`, `review-findings.md`). If future scenarios need subdirs, add `os.MkdirAll(filepath.Dir(...))` before `os.WriteFile`.

### Review Session Flow (for test planning)

```
Runner.Execute(ctx)
├─ RecoverDirtyState()
├─ for i := 0; i < MaxIterations; i++
│  ├─ ReadFile(tasks) → ScanTasks()
│  ├─ reviewCycles := 0
│  └─ for { // review cycle
│     ├─ Read findings → AssemblePrompt → session.Execute (MockClaude step N: execute)
│     ├─ HeadCommit(before/after) → commit detected
│     ├─ ReviewFn = RealReview:
│     │  ├─ ReadFile(tasks) → ScanTasks → currentTask
│     │  ├─ AssemblePrompt(reviewTemplate, {__TASK_CONTENT__: currentTask})
│     │  ├─ session.Execute (MockClaude step N+1: review, writes side effects)
│     │  └─ DetermineReviewOutcome(tasksFile, currentTask, projectRoot)
│     │     ├─ Re-read sprint-tasks.md (now has [x] from side effects)
│     │     ├─ Check review-findings.md (absent/empty vs non-empty)
│     │     └─ Return ReviewResult{Clean: true/false}
│     └─ if Clean → break (next task) else reviewCycles++
```

### MockClaude Step Counts Per Scenario

| Scenario | Execute steps | Review steps | Total MockClaude steps | HeadCommit calls |
|----------|:---:|:---:|:---:|:---:|
| Clean single pass (AC1) | 1 | 1 | 2 | 2 |
| Findings→fix→clean (AC2) | 2 | 2 | 4 | 4 |
| Max review cycles (AC3, max=3) | 3 | 3 | 6 | 6 |
| Multi-task mixed (AC4, 3 tasks) | 4 | 4 | 8 | 8 |
| Bridge golden (AC6, 3 tasks) | 3 | 3 | 6 | 6 |

### Sprint-tasks.md Content Per Review Step (AC4: Multi-task)

```
Initial:  "- [ ] Task one\n- [ ] Task two\n- [ ] Task three\n"
After T1 clean review:  "- [x] Task one\n- [ ] Task two\n- [ ] Task three\n"
After T2 findings review: no change to tasks (findings written instead)
After T2 fix clean review: "- [x] Task one\n- [x] Task two\n- [ ] Task three\n"
After T3 clean review:  "- [x] Task one\n- [x] Task two\n- [x] Task three\n"
```

### What Already Works (from previous stories)

| Component | Story | Status |
|-----------|-------|--------|
| `realReview` function (to be exported) | 4.3 | Done (runner.go:70) |
| `DetermineReviewOutcome` (exported) | 4.3 | Done (runner.go:134) |
| `Runner.Execute` full review cycle loop | 3.5, 4.7 | Done |
| Emergency stop `config.ErrMaxReviewCycles` | 3.10 | Done |
| Findings injection into execute prompt | 4.7 | Done |
| Review prompt with sub-agents, verification | 4.1-4.6 | Done |
| MockClaude subprocess infrastructure | 1.11 | Done |
| `setupRunnerIntegration` helper | 3.11 | Done |
| `headCommitPairs`, `writeTasksFile` helpers | 3.5 | Done |
| Bridge golden file `TestBridge_MergeWithCompleted.golden` | 2.5 | Done |

### What This Story Adds

1. **Export RealReview**: rename `realReview` → `RealReview` (~3 lines changed in runner.go)
2. **Extend MockClaude**: add WriteFiles/DeleteFiles to ScenarioStep + file-writing logic (~20 lines in mock_claude.go)
3. **6 review integration tests**: clean, findings-fix, emergency stop, multi-task, bridge golden, file state (~250-350 lines in runner_review_integration_test.go)
4. **Helper**: setupReviewIntegration (~15 lines in test_helpers_test.go)
5. **Manual checklist**: runner/testdata/manual_smoke_checklist.md (~30 lines)

### Error Handling Patterns

- `errors.Is(err, config.ErrMaxReviewCycles)` for emergency stop test
- `strings.Contains(err.Error(), "review cycles exhausted")` for error message
- `strings.Contains(err.Error(), "Task one")` for task name in error
- No new sentinel errors — all existing

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `testConfig(tmpDir, maxIter)` | test_helpers_test.go:141 | Config with standard defaults |
| `setupRunnerIntegration` | test_helpers_test.go:204 | Base Runner setup (will be extended) |
| `headCommitPairs(pairs...)` | test_helpers_test.go:67 | Generate HeadCommits for MockGitClient |
| `writeTasksFile(t, dir, content)` | test_helpers_test.go:56 | Write sprint-tasks.md |
| `noopSleepFn` | test_helpers_test.go:178 | No-op sleep for tests |
| `noopResumeExtractFn` | test_helpers_test.go:181 | No-op resume extract |
| `assertArgsContainFlag` | test_helpers_test.go:90 | Verify CLI flag exists |
| `assertArgsContainFlagValue` | test_helpers_test.go:101 | Verify CLI flag+value |
| `argValueAfterFlag` | test_helpers_test.go:130 | Extract flag value from args |
| `testutil.SetupMockClaude` | mock_claude.go:183 | Setup scenario environment |
| `testutil.ReadInvocationArgs` | mock_claude.go:213 | Read logged CLI args |

### Sentinel Errors (do NOT duplicate)

Existing: `config.ErrMaxRetries`, `config.ErrMaxReviewCycles`, `config.ErrNoTasks`, `runner.ErrDirtyTree`, `runner.ErrDetachedHead`, `runner.ErrMergeInProgress`, `runner.ErrNoCommit`

No new sentinels needed for this story.

### KISS/DRY/SRP Analysis

**KISS:**
- Export RealReview: one rename, no new abstractions
- MockClaude extension: generic WriteFiles/DeleteFiles, no review-specific logic in mock
- Each test = one scenario, one `r.Execute(ctx)` call, clear assertions

**DRY:**
- `setupReviewIntegration` helper — 6 tests share identical boilerplate (DRY justified)
- Reuses ALL existing test helpers from Story 3.11
- WriteFiles content defined once per test, no duplication

**SRP:**
- MockClaude file side effects = general-purpose capability (not review-specific)
- Review integration tests verify review pipeline, not individual functions
- No overlap with Story 3.11 integration tests (those use mock ReviewFn)
- Manual checklist is a separate artifact (not code)

### Story 4.7 Code Review Learnings (apply to 4.8)

- **DRY in prompts**: keep constraint statements in ONE canonical section
- **Never silently discard return values**: capture and assert on all returns
- **Negative constraint assertions**: when AC says "never X", assert the negative text

### Story 3.11 Code Review Learnings (apply to 4.8)

- **Initialize ALL injectable fields** on test Runner structs — prevents nil-pointer panics
- **Inner error != outer prefix** in assertions — use unique inner cause string
- **Call count assertions** for ALL mocks — HeadCommitCount, HealthCheckCount, etc.
- **Scenario.Name field** — always set on `testutil.Scenario` structs for debugging

### Project Structure Notes

**Files to CREATE:**
| File | Purpose |
|------|---------|
| `runner/runner_review_integration_test.go` | 6 review pipeline integration tests with `//go:build integration` tag |
| `runner/testdata/manual_smoke_checklist.md` | v0.1 pre-release manual validation checklist |

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Rename `realReview` → `RealReview`, update `Run()` reference (~3 lines) |
| `internal/testutil/mock_claude.go` | Add WriteFiles/DeleteFiles to ScenarioStep + file-writing logic in RunMockClaude (~20 lines) |
| `runner/test_helpers_test.go` | Add `setupReviewIntegration` helper (~15 lines) |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/runner.go:70-125` | RealReview implementation (review flow under test) |
| `runner/runner.go:134-159` | DetermineReviewOutcome (file-state logic) |
| `runner/runner.go:239-390` | Runner.Execute (main loop with review cycle) |
| `runner/runner_integration_test.go` | Existing Story 3.11 tests (naming pattern reference) |
| `bridge/testdata/TestBridge_MergeWithCompleted.golden` | Bridge golden file for AC6 |
| `runner/prompts/review.md` | Review prompt (verify sub-agent references in test assertions) |

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.8] — AC and technical requirements
- [Source: runner/runner.go:70-125] — `realReview` function (to be exported as RealReview)
- [Source: runner/runner.go:134-159] — `DetermineReviewOutcome` (file-state check)
- [Source: runner/runner.go:239-390] — `Runner.Execute` (main loop with review cycles)
- [Source: runner/runner_integration_test.go:298-602] — Story 3.11 integration tests (pattern reference)
- [Source: runner/test_helpers_test.go:200-219] — `setupRunnerIntegration` helper (to extend)
- [Source: internal/testutil/mock_claude.go:14-28] — ScenarioStep struct (to extend)
- [Source: internal/testutil/mock_claude.go:61-177] — RunMockClaude (to add file side effects)
- [Source: runner/testmain_test.go] — TestMain dispatch (do NOT duplicate)
- [Source: bridge/testdata/TestBridge_MergeWithCompleted.golden] — Bridge golden file for AC6
- [Source: runner/prompts/review.md] — Review prompt with sub-agents and verification
- [Source: .claude/rules/code-quality-patterns.md] — Error wrapping, stale doc comments
- [Source: .claude/rules/test-error-patterns.md] — Error testing patterns
- [Source: .claude/rules/test-assertions.md] — Assertion patterns
- [Source: docs/sprint-artifacts/4-7-execute-review-loop-integration.md#Dev Notes] — Story 4.7 learnings
- [Source: docs/sprint-artifacts/3-11-runner-integration-test.md#Dev Notes] — Story 3.11 learnings

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: Renamed `realReview` → `RealReview` in runner.go. Updated `Run()` reference. Doc comment updated with export rationale.
- Task 2: Extended `ScenarioStep` with `WriteFiles map[string]string` and `DeleteFiles []string`. Added file side-effects logic in `RunMockClaude` using `MOCK_CLAUDE_PROJECT_ROOT` env var. Added `TestRunMockClaude_WriteFiles` test and zero-value assertions for new fields. All existing MockClaude tests pass.
- Task 3: Added `setupReviewIntegration` helper in test_helpers_test.go — sets `ReviewFn = runner.RealReview` and `MOCK_CLAUDE_PROJECT_ROOT` env var. DRY-justified for 5 review integration tests.
- Task 4: `TestRunner_Execute_ReviewIntegration_CleanReview` — 1 execute + 1 review (clean). Verifies task [x], findings absent, prompt content assertions.
- Task 5: `TestRunner_Execute_ReviewIntegration_FindingsFixClean` — 2 execute + 2 review. Verifies findings injection in fix-execute prompt (Story 4.7 validation), task [x] at end.
- Task 6: `TestRunner_Execute_ReviewIntegration_MaxReviewCycles` — 3 cycles emergency stop. Verifies `errors.Is(err, config.ErrMaxReviewCycles)`, error message content.
- Task 7: `TestRunner_Execute_ReviewIntegration_MultiTaskMixed` — 3 tasks (clean, 1 fix cycle, clean). Verifies all 3 tasks [x], HeadCommitCount == 8.
- Task 8: `TestRunner_Execute_ReviewIntegration_BridgeGoldenFile` — bridge golden file as input. Validates bridge→runner→review data contract end-to-end.
- Task 9: Created `runner/testdata/manual_smoke_checklist.md` with 5 v0.1 pre-release checklist items.
- Task 10: Full suite passes — all packages, integration + unit, clean build.

### Implementation Plan

- Export RealReview: 1 rename, 1 doc comment update, 1 Run() reference update
- MockClaude extension: generic WriteFiles/DeleteFiles, 1 env var, ~20 lines
- 5 review integration tests in new file, reusing setupReviewIntegration helper
- Manual checklist as separate markdown artifact

### File List

- `runner/runner.go` — modified: `realReview` → `RealReview`, updated `Run()`, updated doc comment
- `internal/testutil/mock_claude.go` — modified: `WriteFiles`/`DeleteFiles` fields + file side-effects in `RunMockClaude`
- `internal/testutil/mock_claude_test.go` — modified: `TestRunMockClaude_WriteFiles` + zero-value assertions
- `runner/test_helpers_test.go` — modified: `setupReviewIntegration` helper
- `runner/runner_review_integration_test.go` — created: 5 review pipeline integration tests
- `runner/testdata/manual_smoke_checklist.md` — created: v0.1 pre-release manual validation checklist
- `docs/sprint-artifacts/4-8-review-integration-test.md` — modified: tasks complete, dev record
- `docs/sprint-artifacts/sprint-status.yaml` — modified: story status → review

### Change Log

- 2026-02-27: Implemented review integration tests — 5 end-to-end tests, MockClaude file side-effects, manual smoke checklist
