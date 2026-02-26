# Story 3.9: Emergency Stop — Execute Attempts Exhausted

Status: done

## Story

As a пользователь ralph run,
I want the system to automatically stop when AI exhausts max execute attempts on a task,
so that I don't waste resources on a stuck task and get a clear message about what failed.

## Acceptance Criteria

```gherkin
Scenario: Emergency stop when execute_attempts reaches max (AC1)
  Given max_iterations = 3 (config default)
  And current task has execute_attempts = 3
  When runner checks counter before next attempt
  Then triggers emergency stop (FR23)
  And exits with code 1

Scenario: Informative stop message (AC2)
  Given emergency stop triggered by execute_attempts
  When stop message is generated
  Then includes task name/description that failed
  And includes number of attempts made
  And includes suggestion to check logs
  And message uses fatih/color for visibility (via cmd/)

Scenario: Configurable max_iterations (AC3)
  Given config has max_iterations = 5
  When execute_attempts reaches 5
  Then emergency stop triggers
  And does not trigger at 3 or 4

Scenario: Emergency stop is non-interactive (AC4)
  Given emergency stop triggers
  When system stops
  Then does NOT prompt for user input (не interactive gate)
  And simply exits with error code + message
  And interactive gate upgrade is in Epic 5
```

## Tasks / Subtasks

- [x] Task 1: Enhance error message in runner.go (AC: 2)
  - [x] 1.1 In `runner/runner.go:206`, change `return fmt.Errorf("runner: %w", config.ErrMaxRetries)` to include task description, attempt count, max iterations, and suggestion: `fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q (check logs for details): %w", executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text, config.ErrMaxRetries)`
  - [x] 1.2 No new imports, types, sentinels, or functions — pure string format change in existing error return

- [x] Task 2: Map ErrMaxRetries → exitPartial (1) in cmd/ralph/exit.go (AC: 1)
  - [x] 2.1 In `cmd/ralph/exit.go:exitCode()`, add `errors.Is(err, config.ErrMaxRetries)` check returning `exitPartial` — insert BEFORE the final `return exitFatal` (after context.Canceled check)
  - [x] 2.2 No new imports needed — `config` and `errors` already imported

- [x] Task 3: Strengthen TestRunner_Execute_NoCommitDetected with new message + tracking (AC: 1, 2)
  - [x] 3.1 In `TestRunner_Execute_NoCommitDetected` (runner_test.go:759), switch from `noopResumeExtractFn`/`noopSleepFn` to `trackingResumeExtract`/`trackingSleep` for count assertions
  - [x] 3.2 Add message assertions: `strings.Contains(err.Error(), "execute attempts exhausted")`, `strings.Contains(err.Error(), "1/1")`, `strings.Contains(err.Error(), "Task one")`, `strings.Contains(err.Error(), "check logs")`
  - [x] 3.3 Add tracking assertions: `re.count == 0` (emergency stop fires before resume extract at MaxIterations=1), `ts.count == 0` (no backoff sleep), `mock.HeadCommitCount == 2` (before+after), `mock.HealthCheckCount == 1` (startup only)
  - [x] 3.4 In `TestRunner_Execute_ExitErrorWithParseFailure` (runner_test.go:1550), add `strings.Contains(err.Error(), "execute attempts exhausted")` assertion — this test also hits ErrMaxRetries but lacks message verification

- [x] Task 4: Convert MaxRetriesExhausted to table-driven with configurable max (AC: 2, 3)
  - [x] 4.1 Convert `TestRunner_Execute_MaxRetriesExhausted` to table-driven with struct `{name string, maxIter, wantResumeCount, wantSleepCount, wantHeadCommitCount, wantHealthCheckCount int, wantCountFormat string}`
  - [x] 4.2 Case "max 3 default": maxIter=3, 3 scenario steps, HeadCommits=6× "aaa", wantResumeCount=2, wantSleepCount=2, wantHeadCommitCount=6, wantHealthCheckCount=3, wantCountFormat="3/3"
  - [x] 4.3 Case "max 5 configurable": maxIter=5, 5 scenario steps, HeadCommits=10× "aaa", wantResumeCount=4, wantSleepCount=4, wantHeadCommitCount=10, wantHealthCheckCount=5, wantCountFormat="5/5". This proves stop does NOT trigger at 3 or 4 (4 resume extracts means loop ran through attempts 1-4)
  - [x] 4.4 Both cases verify: `errors.Is(err, config.ErrMaxRetries)`, `strings.Contains("execute attempts exhausted")`, `strings.Contains("Task one")`, `strings.Contains("check logs")`, `strings.Contains(wantCountFormat)`, and all mock counts from struct

- [x] Task 5: Update exit code tests (AC: 1)
  - [x] 5.1 In `cmd/ralph/exit_test.go:TestExitCode_TableDriven`, change `{"ErrMaxRetries", config.ErrMaxRetries, exitFatal}` to `{"ErrMaxRetries", config.ErrMaxRetries, exitPartial}`
  - [x] 5.2 Change `{"wrapped ErrMaxRetries", fmt.Errorf("runner: %w", config.ErrMaxRetries), exitFatal}` to `exitPartial`
  - [x] 5.3 Run full test suite: `go test ./...` — verify zero failures

## Dev Notes

### Architecture Constraints

- **Dependency direction**: `runner → session, config`, `cmd/ralph → runner, config` (unchanged)
- **Error handling**: `fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q ...: %w", ...)` — existing error return enhanced with context
- **Sentinel preserved**: `config.ErrMaxRetries` sentinel unchanged, `errors.Is` chain still works
- **No new sentinels, types, or functions** — pure enhancement of existing error message + exit code mapping
- **Packages return errors, cmd/ formats output**: runner returns enriched ErrMaxRetries, cmd/ralph maps to exit 1
- **AC2 "uses fatih/color"**: Already handled by `main.go:56` — `color.Red("Error: %v", err)` displays all non-interrupted errors in red. Cobra is silenced (`SilenceErrors = true`), and `run()` calls `exitCode(err)` then `color.Red`. No additional code needed — the enhanced error message will automatically appear in red

### KISS/DRY/SRP Analysis

**KISS:** Two minimal changes — (1) enhance error format string in runner.go (1 line), (2) add 3-line check in exit.go. No new abstractions, types, files, or functions.

**DRY:** No new code paths. Enhanced existing error return. Exit code mapping reuses existing patterns in exitCode().

**SRP:** runner/runner.go returns informative error (its job). cmd/ralph/exit.go maps to exit code (its job). No responsibility leakage.

### Code Change Summary

**runner/runner.go:206 — 1 line changed:**
```go
// BEFORE:
return fmt.Errorf("runner: %w", config.ErrMaxRetries)

// AFTER:
return fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q (check logs for details): %w",
    executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text, config.ErrMaxRetries)
```

**cmd/ralph/exit.go — 3 lines added (before final return exitFatal):**
```go
if errors.Is(err, config.ErrMaxRetries) {
    return exitPartial
}
```

**Example error output:**
```
runner: execute attempts exhausted (3/3) for "- [ ] Task one" (check logs for details): max retries exceeded
```

### Why result.OpenTasks[0].Text Is Safe

At the point where `executeAttempts >= MaxIterations` returns, we are inside the inner retry loop. The inner loop is entered ONLY when `result.HasOpenTasks()` returned true earlier in the same outer iteration. Therefore `result.OpenTasks[0]` is guaranteed non-empty — no bounds check needed.

### Existing Test Coverage Map

| AC | Test | Status |
|----|------|--------|
| AC1 (stop + exit 1) | `TestRunner_Execute_MaxRetriesExhausted` + `TestExitCode_TableDriven` | **UPDATED** |
| AC2 (informative msg) | `MaxRetriesExhausted`, `NoCommitDetected`, `ExitErrorWithParseFailure` | **UPDATED** (new assertions) |
| AC3 (configurable max) | `TestRunner_Execute_MaxRetriesExhausted` "max 5" case | **NEW** (table-driven) |
| AC4 (non-interactive) | All existing tests return error, no stdin read | Already covered |

### Previous Story Intelligence (Story 3.8)

Key learnings applied:
- **Stale doc comments**: No doc comments need updating — error return has no doc comment
- **Inner error != outer prefix**: New assertions use `"execute attempts exhausted"` (unique, not confused with `"runner:"` prefix) and `"Task one"` (unique task text from fixture)
- **M3 nil-pointer safety**: All Runner struct fields initialized in existing tests (fixed in Story 3.8)

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
| `trackingResumeExtract` | test_helpers_test.go:152 | Record resume extract calls |
| `trackingSleep` | test_helpers_test.go:165 | Record sleep calls |
| `threeOpenTasks` constant | test_helpers_test.go:16 | Three open tasks content |

### Existing Sentinel Errors (do NOT duplicate)

- `config.ErrMaxRetries` — retry exhaustion (cross-package, REUSED)
- `config.ErrNoTasks` — no tasks found
- `runner.ErrNoCommit` — no commit detected
- `runner.ErrDirtyTree` — dirty working tree
- `runner.ErrDetachedHead` — detached HEAD
- `runner.ErrMergeInProgress` — merge/rebase in progress

### Git Intelligence (Recent Commits)

```
8276c57 Stories 3.6, 3.7, 3.8 — retry logic, resume extraction, startup recovery
3584c6b Story 3.5 — Runner loop skeleton
16fc58f Stories 3.3+3.4 — GitClient, MockGitClient, RecoverDirtyState
```

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Enhance error format string (1 line) |
| `runner/runner_test.go` | Update assertions + table-drive MaxRetriesExhausted |
| `cmd/ralph/exit.go` | Add ErrMaxRetries → exitPartial mapping (3 lines) |
| `cmd/ralph/exit_test.go` | Update 2 table cases: exitFatal → exitPartial |

**Files NOT to modify:**
- `config/errors.go` — ErrMaxRetries already correct
- `config/config.go` — MaxIterations already configurable
- `runner/git.go` — No git changes
- `runner/scan.go` — No scan changes
- `runner/knowledge.go` — No knowledge changes
- `runner/test_helpers_test.go` — All needed helpers exist

**No new files** — all changes in existing files.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.9] — AC and technical requirements
- [Source: runner/runner.go:203-207] — Current emergency stop return (to be enhanced)
- [Source: cmd/ralph/exit.go:22-44] — exitCode mapping (to add ErrMaxRetries → 1)
- [Source: runner/runner_test.go:1117-1181] — TestRunner_Execute_MaxRetriesExhausted (to be table-driven)
- [Source: runner/runner_test.go:759-801] — TestRunner_Execute_NoCommitDetected (assertions to add)
- [Source: cmd/ralph/exit_test.go:33-35] — ErrMaxRetries test cases (exitFatal → exitPartial)
- [Source: cmd/ralph/main.go:48-57] — exitCode() call + color.Red error display (AC2 fatih/color)
- [Source: runner/runner_test.go:1550-1600] — TestRunner_Execute_ExitErrorWithParseFailure (message assertion to add)
- [Source: .claude/rules/go-testing-patterns.md] — 50+ testing patterns

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Review Outcome:** Changes Requested (all fixed)
**Total Findings:** 6 (0H / 4M / 2L)

### Action Items

- [x] [M1] gofmt violation: runner/runner.go:207 continuation line indentation — fixed by `go fmt`
- [x] [M2] Stale doc comment: cmd/ralph/exit.go:20-21 "everything else → 4" missing ErrMaxRetries → 1
- [x] [M3] Stale test comment: cmd/ralph/exit_test.go:32 "Sentinel errors — map to exitFatal" → removed "map to exitFatal"
- [x] [M4] Missing inner error text assertion: runner_test.go NoCommitDetected + MaxRetriesExhausted — added "max retries exceeded" sentinel text check
- [ ] [L1] Task text includes "- [ ]" prefix in error message — out of scope (TaskEntry.Text design from Story 3.2)
- [ ] [L2] sprint-status.yaml not in File List — standard tracking infrastructure

### Recurring Patterns

- **Stale doc comments** — 5th occurrence in Epic 3 (3.2, 3.3, 3.5, 3.8, 3.9)
- **Inner error assertion** — established pattern, was temporarily dropped

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No debug issues encountered — all 5 tasks implemented cleanly.

### Completion Notes List

- Task 1: Enhanced error format string in runner.go:206 — now includes attempt count (N/M), task description, and "check logs" suggestion. Pure string format change, no new types/functions/imports.
- Task 2: Added `errors.Is(err, config.ErrMaxRetries) → exitPartial` mapping in exit.go, placed after context.Canceled check (before final exitFatal). SRP: runner returns error, cmd/ maps exit code.
- Task 3: Strengthened NoCommitDetected test with tracking helpers (trackingResumeExtract, trackingSleep) replacing noop variants. Added 4 message assertions + 4 tracking count assertions. Added message assertion to ExitErrorWithParseFailure.
- Task 4: Converted MaxRetriesExhausted to table-driven with 2 cases: "max 3 default" and "max 5 configurable". Dynamically generates scenario steps and HeadCommit slices. "max 5" case proves AC3 — stop does NOT trigger at 3 or 4.
- Task 5: Updated 2 exit_test.go table cases from exitFatal → exitPartial. Full test suite passes (0 failures).
- SRP focus: runner returns enriched error (its job), cmd/ralph maps to exit code (its job) — no responsibility leakage.
- YAGNI focus: No new abstractions, types, files, or functions. Zero new sentinels.
- DRY focus: Reused existing test helpers (testConfig, trackingResumeExtract, trackingSleep, fatalReviewFn, writeTasksFile). Table-driven test generates data dynamically instead of duplicating.

### Change Log

- 2026-02-26: Implemented Story 3.9 — emergency stop with informative message and exit code 1 mapping
- 2026-02-26: Code review fixes (4M) — gofmt, stale doc comments (exit.go, exit_test.go), inner error text assertions restored

### File List

- runner/runner.go (modified — enhanced error format string, line 206)
- runner/runner_test.go (modified — strengthened NoCommitDetected, table-driven MaxRetriesExhausted, message assertion in ExitErrorWithParseFailure)
- cmd/ralph/exit.go (modified — added ErrMaxRetries → exitPartial mapping)
- cmd/ralph/exit_test.go (modified — updated 2 table cases exitFatal → exitPartial)
