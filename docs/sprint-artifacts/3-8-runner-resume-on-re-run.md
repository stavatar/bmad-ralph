# Story 3.8: Runner Resume on Re-run

Status: done

## Story

As a пользователь,
I want ralph run to continue from the first incomplete task on re-run, recovering dirty state if needed,
so that interrupted runs resume automatically without manual cleanup.

## Acceptance Criteria

```gherkin
Scenario: Resume from first incomplete task (AC1)
  Given sprint-tasks.md has tasks: [x] task1, [x] task2, [ ] task3, [ ] task4
  When ralph run starts
  Then scanner finds first "- [ ]" = task3
  And execution begins from task3 (FR12)

Scenario: All tasks completed (AC2)
  Given sprint-tasks.md has only "- [x]" tasks
  When ralph run starts
  Then reports "all tasks completed"
  And exits with code 0

Scenario: Dirty tree recovery on re-run (AC3)
  Given working tree is dirty (interrupted previous run)
  When ralph run starts
  And GitClient.HealthCheck returns ErrDirtyTree
  Then executes git checkout -- . for recovery (FR12)
  And logs warning about recovery
  And proceeds with first incomplete task

Scenario: Soft validation warning (AC4)
  Given sprint-tasks.md contains no "- [ ]" and no "- [x]" markers
  When scanner parses file
  Then outputs warning recommending file check (FR12)
  And exits (no tasks to process)

Scenario: Re-run after partial completion (AC5)
  Given 5 tasks total, 3 completed in previous run
  When ralph run re-invoked
  Then starts from task 4 (first "- [ ]")
  And does not re-execute completed tasks
```

## Tasks / Subtasks

- [x] Task 1: Replace HealthCheck with RecoverDirtyState at Execute() startup (AC: 3)
  - [x] 1.1 In `Execute()` (runner/runner.go:111-114), replace `r.Git.HealthCheck(ctx)` + error check with `RecoverDirtyState(ctx, r.Git)` call. On error, wrap with `fmt.Errorf("runner: startup: %w", err)`. Add comment on discarded bool: `// recovered bool unused — no startup logging plumbing yet`
  - [x] 1.2 Update Execute() doc comment (runner/runner.go:105-110) to mention startup dirty state recovery: "Startup: recovers dirty working tree (RecoverDirtyState), non-dirty health errors abort"
  - [x] 1.3 No new imports, functions, types, or sentinels — reuses existing `RecoverDirtyState`

- [x] Task 2: Update TestRunner_Execute_HealthCheckErrors for new startup behavior (AC: 3)
  - [x] 2.1 Rename `TestRunner_Execute_HealthCheckErrors` to `TestRunner_Execute_StartupErrors`. Add `wantRestoreCount int` field to the test table struct (needed by new cases 2.3-2.5)
  - [x] 2.2 Remove "dirty tree" test case from table — dirty tree is now recovered, not error. This case moves to Task 3
  - [x] 2.3 Update "detached HEAD" case: `wantErrContains` from `"runner: health check:"` to `"runner: startup:"`. Add `wantErrContainsInner: "runner: dirty state recovery:"`. `wantErrIs: runner.ErrDetachedHead` unchanged (errors.Is traverses %w chain). Set `wantRestoreCount: 0` (no restore attempted for non-dirty errors)
  - [x] 2.4 Update "generic git error" case: same prefix change. Multi-layer error: verify outer `"runner: startup:"`, intermediate `"runner: dirty state recovery:"`, inner `"git not found"`. Add `wantErrContainsIntermediate string` field to struct (or use separate assertion lines) to verify all 3 layers — per Story 3.4 multi-layer error wrapping pattern
  - [x] 2.5 Add new case "dirty tree restore fails": `HealthCheckErrors: []error{runner.ErrDirtyTree}`, `RestoreCleanError: errors.New("restore failed")`, `wantErrContains: "runner: startup:"`, `wantErrContainsInner: "restore failed"`, `wantRestoreCount: 1`
  - [x] 2.6 All cases verify `HealthCheckCount`, `HeadCommitCount` (0 — startup error before loop), `RestoreCleanCount`

- [x] Task 3: Test dirty tree recovery at startup succeeds (AC: 3, 2)
  - [x] 3.1 Create `TestRunner_Execute_DirtyTreeRecoveryAtStartup` — standalone test (not table-driven, single focused scenario)
  - [x] 3.2 Setup: `MockGitClient{HealthCheckErrors: []error{runner.ErrDirtyTree}}`, tasks = `allDoneTasks` (recovery → scan → no open tasks → return nil)
  - [x] 3.3 Assertions: `err == nil` (execution proceeded), `mock.HealthCheckCount == 1` (RecoverDirtyState calls HealthCheck once), `mock.RestoreCleanCount == 1` (recovery performed), `mock.HeadCommitCount == 0` (no open tasks → no session executed)
  - [x] 3.4 This test covers AC3 (dirty recovery) + AC2 (all done) intersection

- [x] Task 4: Verify existing tests cover remaining ACs + fix pre-existing field init gaps (AC: 1, 2, 4, 5)
  - [x] 4.1 AC1 (resume from first [ ]): Already covered by `TestRunner_Execute_SequentialExecution` — scanner finds open tasks, Claude picks first `- [ ]` via prompt. No new test needed
  - [x] 4.2 AC2 (all done → nil): Already covered by `TestRunner_Execute_AllTasksDone`. HealthCheckCount expectation unchanged — source of the call changes (Execute directly → RecoverDirtyState internally) but count stays 1. Initialize missing `ResumeExtractFn: noopResumeExtractFn` and `SleepFn: noopSleepFn` fields on this test's Runner struct (M3 nil-pointer safety pattern)
  - [x] 4.3 AC4 (no markers → ErrNoTasks): Already covered by `TestRunner_Execute_ErrNoTasks`. Initialize missing injectable fields if absent (same M3 pattern)
  - [x] 4.4 AC5 (partial completion): Same as AC1 behavior. Already covered by existing tests where tasks file has mix of [x] and [ ]
  - [x] 4.5 Fix pre-existing M3 violations: add `ResumeExtractFn: noopResumeExtractFn`, `SleepFn: noopSleepFn` to `TestRunner_Execute_HeadCommitBeforeFails` and `TestRunner_Execute_ReadTasksFails` Runner structs
  - [x] 4.6 Run full test suite: `go test ./runner/...` — verify zero failures, zero regressions

## Dev Notes

### Architecture Constraints

- **Dependency direction**: `runner → session, config` (unchanged, no new imports)
- **Error handling**: `fmt.Errorf("runner: startup: %w", err)` — caller context prefix wrapping RecoverDirtyState's `"runner: dirty state recovery: %w"` (same pattern as retry path: `"runner: retry: recover: %w"`)
- **No new sentinels, types, or functions** — pure wiring of existing `RecoverDirtyState` into startup
- **Packages return errors, cmd/ formats output**: ErrNoTasks from scanner returned to cmd/ralph which formats as warning. Runner does NOT log warnings directly
- **AC3 "logs warning about recovery" deferred**: `RecoverDirtyState` returns `(bool, error)` where `bool` = recovery happened. The `bool` is discarded at startup (`_`) because there is no cmd/ralph plumbing to surface it yet. Warning logging for recovery will be wired when cmd/ralph integrates runner (future story). Runner returns `nil` on successful recovery — cmd/ralph cannot distinguish "was clean" from "was dirty, recovered"
- **Config immutability**: No new config fields

### KISS/DRY/SRP Analysis

**KISS:** Minimal change — replace 1 function call (HealthCheck → RecoverDirtyState) at Execute() startup. No new abstractions, no new types, no new files. The RecoverDirtyState function already encapsulates the exact behavior needed.

**DRY:** RecoverDirtyState is already used in the retry path (runner.go:211). Reusing it at startup eliminates duplicated recovery logic. Both callsites wrap with caller-context prefix (`"runner: startup:"` / `"runner: retry: recover:"`).

**SRP:** Execute() orchestrates the loop. RecoverDirtyState handles git recovery. Scanner handles task parsing. Each has single responsibility. No function scope increases.

### Code Change Summary

**Production code change — 3 lines in runner/runner.go:**
```go
// BEFORE (runner.go:111-114):
if err := r.Git.HealthCheck(ctx); err != nil {
    return fmt.Errorf("runner: health check: %w", err)
}

// AFTER:
// recovered bool unused — no startup logging plumbing yet
if _, err := RecoverDirtyState(ctx, r.Git); err != nil {
    return fmt.Errorf("runner: startup: %w", err)
}
```

**Behavior change:**
| Startup state | Before (Story 3.5) | After (Story 3.8) |
|---|---|---|
| Clean repo | HealthCheck → nil → proceed | RecoverDirtyState → (false, nil) → proceed |
| Dirty tree | HealthCheck → ErrDirtyTree → **error returned** | RecoverDirtyState → RestoreClean → (true, nil) → **proceed** |
| Detached HEAD | HealthCheck → ErrDetachedHead → error | RecoverDirtyState → error (passthrough) → error |
| Merge/rebase | HealthCheck → ErrMergeInProgress → error | RecoverDirtyState → error (passthrough) → error |

**Error message format change for non-dirty errors:**
- Before: `"runner: health check: git: HEAD is detached"`
- After: `"runner: startup: runner: dirty state recovery: git: HEAD is detached"`
- Note: `errors.Is(err, ErrDetachedHead)` still works through `%w` chain — no impact on error type checking

### Why NOT a Separate StartupRecovery Function

RecoverDirtyState already does exactly what's needed:
1. Calls HealthCheck
2. If ErrDirtyTree → RestoreClean → return (true, nil)
3. If other error → wrap and return error
4. If clean → return (false, nil)

Creating a wrapper adds no value (YAGNI). The caller context (`"runner: startup:"`) is the only differentiation, and that's handled by the `fmt.Errorf` wrapping.

### Existing Test Coverage Map

| AC | Test | Status |
|----|------|--------|
| AC1 (first [ ] task) | `TestRunner_Execute_SequentialExecution` | Already passes |
| AC2 (all done) | `TestRunner_Execute_AllTasksDone` | Already passes |
| AC3 (dirty recovery) | `TestRunner_Execute_DirtyTreeRecoveryAtStartup` | **NEW** |
| AC3 (recovery fails) | `TestRunner_Execute_StartupErrors` "dirty tree restore fails" | **NEW case** |
| AC3 (non-dirty error) | `TestRunner_Execute_StartupErrors` "detached HEAD", "generic" | **UPDATED** |
| AC4 (no markers) | `TestRunner_Execute_ErrNoTasks` | Already passes |
| AC5 (partial) | Same as AC1 | Already passes |

### Previous Story Intelligence (Story 3.7)

Key learnings to apply:
- **M1 (verify mock data contents)**: DirtyTreeRecoveryAtStartup test must verify `RestoreCleanCount == 1`, not just `err == nil`
- **M2 (inner error != outer prefix)**: StartupErrors `wantErrContainsInner` must use unique substring (e.g., `"restore failed"`, not `"runner: startup"`)
- **M3 (vacuous test)**: DirtyTreeRecoveryAtStartup must link test context to code path — use `allDoneTasks` to prove execution proceeded past recovery
- **L1 (doc comment accuracy)**: Update Execute() doc comment to reflect new startup behavior

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
| `allDoneTasks` constant | test_helpers_test.go:25 | All-done tasks content |
| `threeOpenTasks` constant | test_helpers_test.go:16 | Three open tasks content |
| `noMarkersTasks` constant | test_helpers_test.go:31 | No-marker tasks content |

### Existing Sentinel Errors (do NOT duplicate)

- `config.ErrMaxRetries` — retry exhaustion (cross-package sentinel)
- `config.ErrNoTasks` — no tasks found
- `runner.ErrNoCommit` — no commit detected
- `runner.ErrDirtyTree` — dirty working tree
- `runner.ErrDetachedHead` — detached HEAD
- `runner.ErrMergeInProgress` — merge/rebase in progress

### Git Intelligence (Recent Commits)

```
3584c6b Story 3.5 — Runner loop skeleton
16fc58f Stories 3.3+3.4 — GitClient, MockGitClient, RecoverDirtyState
9248eb8 Story 3.2 — Sprint-tasks scanner
675a3e4 Knowledge extraction
b6ebc7d Story 3.1 — Execute prompt template
```

Stories 3.6 and 3.7 are in working tree (uncommitted):
- 3.6: Retry logic with ResumeExtractFunc, SleepFn, inner retry loop, 12 new tests
- 3.7: KnowledgeWriter interface, ResumeExtraction function, trackingKnowledgeWriter

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Replace HealthCheck with RecoverDirtyState at startup (3 lines), update doc comment |
| `runner/runner_test.go` | Rename+update HealthCheckErrors test, add DirtyTreeRecoveryAtStartup test |

**Files NOT to modify:**
- `runner/git.go` — RecoverDirtyState already correct
- `runner/scan.go` — Scanner already finds first [ ]
- `runner/knowledge.go` — No knowledge changes
- `runner/test_helpers_test.go` — All needed helpers exist
- `config/` — No new config fields or sentinels
- `session/` — No session changes

**Note:** `RunOnce` (runner.go:240-296) still uses direct `HealthCheck` call (not RecoverDirtyState). This is a standalone utility function NOT used by Execute(). Out of scope for Story 3.8 — may be retired in Epic 4 (marked as "Walking skeleton function" in doc comment).

**No new files** — all changes in existing files.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.8] — AC and technical requirements
- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Key Invariants] — Mutation Asymmetry
- [Source: runner/runner.go:111-114] — Current HealthCheck at startup (to be replaced)
- [Source: runner/runner.go:298-313] — RecoverDirtyState (reused for startup)
- [Source: runner/runner.go:211-213] — RecoverDirtyState in retry path (consistency reference)
- [Source: runner/scan.go:41-70] — ScanTasks returns ErrNoTasks for no markers, ScanResult.HasOpenTasks() for open check
- [Source: runner/git.go:15-26] — GitClient interface + sentinel errors
- [Source: runner/runner_test.go:300-385] — TestRunner_Execute_HealthCheckErrors (to be updated)
- [Source: runner/runner_test.go:227-260] — TestRunner_Execute_AllTasksDone (AC2 coverage)
- [Source: runner/runner_test.go:262-294] — TestRunner_Execute_ErrNoTasks (AC4 coverage)
- [Source: .claude/rules/go-testing-patterns.md] — 50+ testing patterns

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: Replaced `r.Git.HealthCheck(ctx)` with `RecoverDirtyState(ctx, r.Git)` at Execute() startup (3 lines changed). Updated doc comment. No new imports/types/sentinels — pure DRY reuse of existing function.
- Task 2: Renamed `TestRunner_Execute_HealthCheckErrors` → `TestRunner_Execute_StartupErrors`. Removed "dirty tree" error case (dirty is now recovered). Added `wantRestoreCount`, `wantErrContainsIntermediate`, `restoreErr` fields. Updated all prefixes from `"runner: health check:"` to `"runner: startup:"`. Added multi-layer error verification per Story 3.4 pattern. Added "dirty tree restore fails" case.
- Task 3: Created `TestRunner_Execute_DirtyTreeRecoveryAtStartup` — standalone test covering AC3+AC2 intersection. Verifies recovery + all-done flow.
- Task 4: Verified existing AC coverage (AC1/AC2/AC4/AC5 already covered). Fixed M3 nil-pointer safety gaps: added `ResumeExtractFn`/`SleepFn` to 4 pre-existing tests (`AllTasksDone`, `ErrNoTasks`, `HeadCommitBeforeFails`, `ReadTasksFails`). Full test suite: 0 failures, 0 regressions.

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Reviewer Model:** Claude Opus 4.6
**Review Outcome:** Approve with minor fixes (auto-applied)

### Findings Summary: 0 High, 3 Medium, 1 Low

### Action Items

- [x] [M1] Add `wantErrContainsIntermediate: "runner: dirty state recovery:"` to "dirty tree restore fails" test case for multi-layer consistency [runner/runner_test.go:341-350]
- [x] [M2] Update stale section comment from "Task 7: Health check and error paths (AC: #4)" to "Story 3.8: Startup errors" [runner/runner_test.go:300-302]
- [x] [M3] Update stale inline comment from "HealthCheck at startup" to "RecoverDirtyState at startup" in AllTasksDone test [runner/runner_test.go:255]
- [ ] [L1] DirtyTreeRecoveryAtStartup uses allDoneTasks — AC3 "proceeds with first incomplete task" not directly verified with open tasks (acceptable as-is: code path is unconditional, YAGNI)

### SRP/YAGNI/DRY Assessment: All excellent — minimal change, no new abstractions, existing function reused.

### Change Log

- 2026-02-26: Story 3.8 implementation complete. Replaced HealthCheck with RecoverDirtyState at startup, updated tests, fixed M3 field init gaps.
- 2026-02-26: Code review — 4 findings (0H/3M/1L), 3 fixed automatically. L1 deferred (YAGNI).

### File List

- runner/runner.go (modified: startup recovery, doc comment)
- runner/runner_test.go (modified: renamed+updated StartupErrors, new DirtyTreeRecoveryAtStartup, M3 field init fixes)
- docs/sprint-artifacts/sprint-status.yaml (modified: 3-8 status updates)
- docs/sprint-artifacts/3-8-runner-resume-on-re-run.md (modified: task checkboxes, dev agent record)
