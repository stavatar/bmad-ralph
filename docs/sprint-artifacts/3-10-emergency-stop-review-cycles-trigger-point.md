# Story 3.10: Emergency Stop — Review Cycles Trigger Point

Status: done

## Story

As a runner,
I want to track the review_cycles counter and stop at max_review_iterations,
so that infinite execute-review loops are prevented (FR24).

## Acceptance Criteria

```gherkin
Scenario: review_cycles counter increments via configurable stub (AC1)
  Given task enters review phase (configurable stub from Story 3.5)
  And review stub configured to return findings 2 times then clean
  When execute-review cycle repeats
  Then review_cycles counter increments to 2
  And on third review (clean) counter resets to 0

Scenario: Emergency stop at max_review_iterations (AC2)
  Given max_review_iterations = 3 (config default)
  And review_cycles reaches 3
  When runner checks counter
  Then triggers emergency stop (FR24)
  And exits with code 1
  And message indicates review cycle exhaustion

Scenario: Counter is per-task (AC3)
  Given task A had review_cycles = 2
  When task A completes (clean review) and task B starts
  Then task B has review_cycles = 0

Scenario: Counter resets on clean review (AC4)
  Given review_cycles = 1
  When review returns clean (no findings)
  Then review_cycles resets to 0
  And task marked complete (by review in Epic 4)

Scenario: Configurable max_review_iterations (AC5)
  Given config has max_review_iterations = 5
  When review_cycles reaches 5
  Then emergency stop triggers

Scenario: Trigger point prepared for Epic 4 (AC6)
  Given review stub returns "clean" in Epic 3
  When review is replaced with real review in Epic 4
  Then review_cycles counter already integrated
  And no runner loop changes needed for FR24
```

## Tasks / Subtasks

- [x] Task 1: Add ErrMaxReviewCycles sentinel to config/errors.go (AC: 2, 5)
  - [x] 1.1 Add `ErrMaxReviewCycles = errors.New("max review cycles exceeded")` to existing `var (...)` block alongside ErrMaxRetries
  - [x] 1.2 No new imports — `errors` already imported

- [x] Task 2: Add review cycle loop in runner/runner.go Execute (AC: 1, 2, 3, 4)
  - [x] 2.1 Wrap existing inner retry loop (lines 161-224) + ReviewFn call (line 231) in a new `for {` review cycle loop
  - [x] 2.2 Declare `reviewCycles := 0` per task — inside outer loop body, before review cycle loop (AC3: per-task by code structure)
  - [x] 2.3 Change `if _, err := r.ReviewFn(ctx, rc)` to `rr, err := r.ReviewFn(ctx, rc)` — capture ReviewResult
  - [x] 2.4 After ReviewFn: `if rr.Clean { break }` — exit review cycle loop, proceed to next task (AC4: counter resets implicitly)
  - [x] 2.5 After clean check: `reviewCycles++` then `if reviewCycles >= r.Cfg.MaxReviewIterations { return error }`
  - [x] 2.6 Error format: `fmt.Errorf("runner: review cycles exhausted (%d/%d) for %q (check logs for details): %w", reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text, config.ErrMaxReviewCycles)`
  - [x] 2.7 Move `executeAttempts := 0` INSIDE review cycle loop (reset per review cycle — commit was detected, new execute starts fresh)
  - [x] 2.8 Update Execute doc comment: add "review cycle loop: checks ReviewResult.Clean, stops at MaxReviewIterations (FR24)"
  - [x] 2.9 Run `go fmt ./runner/` after editing (Story 3.9 M1 — continuation line indentation)

- [x] Task 3: Map ErrMaxReviewCycles to exitPartial in cmd/ralph/exit.go (AC: 2)
  - [x] 3.1 Add `|| errors.Is(err, config.ErrMaxReviewCycles)` to existing ErrMaxRetries check on line 43-44 (same exit code, one `if` statement)
  - [x] 3.2 Update doc comment on line 21: add `ErrMaxReviewCycles` to the mapping description

- [x] Task 4: Update testConfig helper (AC: all)
  - [x] 4.1 In `runner/test_helpers_test.go:140`, add `MaxReviewIterations: 3` to testConfig return struct (matches defaults.yaml, prevents zero-value triggering emergency stop on any non-clean review)

- [x] Task 5: Update TestRunner_Execute_ReviewFuncSequence (AC: 1, 4)
  - [x] 5.1 Update config: `MaxIterations: 1` (single task), set `MaxReviewIterations: 5` explicitly (won't hit max)
  - [x] 5.2 Mock data stays same: 3 execute steps (each produces commit), 3 HeadCommit pairs — unchanged because 1 task × 3 review cycles = 3 executes
  - [x] 5.3 ReviewFn stays same: not-clean twice, then clean (callCount logic unchanged)
  - [x] 5.4 Update comment on line 599: remove "NOTE: Execute currently discards ReviewResult.Clean" — it no longer discards
  - [x] 5.5 After clean review the task completes, outer loop exits (maxIterations=1). No more mock steps needed
  - [x] 5.6 Verify: callCount == 3 (2 non-clean + 1 clean), no error
  - [x] 5.7 Verify existing count assertions remain correct: HealthCheckCount=1 (startup), HeadCommitCount=6 (3 pairs unchanged)

- [x] Task 6: Add TestRunner_Execute_MaxReviewCyclesExhausted table-driven (AC: 2, 5)
  - [x] 6.1 Table struct: `{name string, maxReviewIter int, wantReviewCount int, wantResumeCount int, wantSleepCount int, wantHeadCommitCount int, wantHealthCheckCount int, wantCountFormat string}`
  - [x] 6.2 Case "max 3 default": maxReviewIter=3, maxIterations=1, 3 execute steps (all exit 0, all commit), HeadCommits=3 pairs (aaa/bbb, bbb/ccc, ccc/ddd), review always `{Clean: false}`, wantReviewCount=3, wantResumeCount=0, wantSleepCount=0, wantHeadCommitCount=6, wantHealthCheckCount=1 (startup only), wantCountFormat="3/3"
  - [x] 6.3 Case "max 5 configurable": maxReviewIter=5, maxIterations=1, 5 execute steps, HeadCommits=5 pairs, wantReviewCount=5, wantResumeCount=0, wantSleepCount=0, wantHeadCommitCount=10, wantHealthCheckCount=1, wantCountFormat="5/5"
  - [x] 6.4 Both cases verify: `errors.Is(err, config.ErrMaxReviewCycles)`, `strings.Contains("review cycles exhausted")`, `strings.Contains("Task one")`, `strings.Contains("check logs")`, `strings.Contains(wantCountFormat)`, `strings.Contains("max review cycles exceeded")` sentinel text, all mock counts including `re.count == wantResumeCount` and `ts.count == wantSleepCount` (symmetric with MaxRetriesExhausted — prove no accidental retry backoff)

- [x] Task 7: Add TestRunner_Execute_ReviewCyclesPerTask (AC: 3)
  - [x] 7.1 Define inline `twoOpenTasks` content in test: `"# Sprint Tasks\n\n- [ ] Task one\n- [ ] Task two\n"` (no constant needed — single use per KISS)
  - [x] 7.2 Config: maxIterations=3 (generous for 2 tasks), maxReviewIterations=2 (low threshold — proves per-task reset)
  - [x] 7.3 4 execute steps (2 per task), 4 HeadCommit pairs (all different = commit each time)
  - [x] 7.4 ReviewFn with callCount: calls 1,3 → `{Clean: false}`; call 2 → `{Clean: true}` + write tasks with first task done ("- [x] Task one\n- [ ] Task two"); call 4 → `{Clean: true}` + write `allDoneTasks`
  - [x] 7.5 Test logic: if reviewCycles persisted from task A (=1), task B's first non-clean would make it 2 → `2 >= 2` → emergency stop error. Test proves NO error (counter resets per task)
  - [x] 7.6 Assert: no error, reviewCount=4 (2 per task), HeadCommitCount=8, HealthCheckCount=1, re.count=0, ts.count=0

- [x] Task 8: Add exit code tests for ErrMaxReviewCycles (AC: 2)
  - [x] 8.1 Add `{"ErrMaxReviewCycles", config.ErrMaxReviewCycles, exitPartial}` to TestExitCode_TableDriven
  - [x] 8.2 Add `{"wrapped ErrMaxReviewCycles", fmt.Errorf("runner: %w", config.ErrMaxReviewCycles), exitPartial}`

- [x] Task 9: Run full test suite (AC: all)
  - [x] 9.1 `go test ./...` — verify zero failures

## Dev Notes

### Architecture Constraints

- **Dependency direction**: `runner -> session, config`, `cmd/ralph -> runner, config` (unchanged)
- **Error handling**: `fmt.Errorf("runner: review cycles exhausted (%d/%d) for %q ...: %w", ...)` — same pattern as Story 3.9
- **Sentinel placement**: `config.ErrMaxReviewCycles` in config/errors.go (cross-package sentinel pattern — exit.go needs `errors.Is` detection without importing runner; see Story 3.6 review finding: "cross-package sentinels in config/errors.go")
  - Epic suggests runner package — deviation justified by established codebase convention from review learnings
- **No new types or functions** — one new sentinel + loop structure change in Execute + exit code mapping
- **AC2 "exits with code 1"**: exit.go maps ErrMaxReviewCycles → exitPartial (=1), same as ErrMaxRetries
- **AC2 "message uses fatih/color"**: Already handled by main.go:56 `color.Red("Error: %v", err)` — same as Story 3.9

### KISS/DRY/SRP Analysis

**KISS:** One new sentinel, one review cycle loop wrapping existing code, one exit code mapping line. No new abstractions, types, files, or functions.

**DRY:** Same error message pattern as Story 3.9. Same exit code mapping consolidation (one `if` with `||`). Reuses all existing test helpers (testConfig, headCommitPairs, fatalReviewFn, writeTasksFile, noopResumeExtractFn, noopSleepFn). Error format `"runner: ... exhausted (%d/%d) for %q (check logs for details): %w"` duplicated with Story 3.9 — only 2 call sites, KISS > DRY (no helper extraction).

**SRP:** runner returns informative error (its job). cmd/ralph maps to exit code (its job). Config holds cross-package sentinel (its job). No responsibility leakage.

### Code Change Summary

**runner/runner.go — Execute method restructure:**
```go
// BEFORE (lines 160-234, simplified):
executeAttempts := 0
for { /* execute retry loop */ break }
if _, err := r.ReviewFn(ctx, rc); err != nil { return ... }

// AFTER:
reviewCycles := 0
for { // review cycle loop
    executeAttempts := 0
    for { /* execute retry loop (unchanged) */ break }
    rr, err := r.ReviewFn(ctx, rc)
    if err != nil { return fmt.Errorf("runner: review: %w", err) }
    if rr.Clean { break }
    reviewCycles++
    if reviewCycles >= r.Cfg.MaxReviewIterations {
        return fmt.Errorf("runner: review cycles exhausted (%d/%d) for %q (check logs for details): %w",
            reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text, config.ErrMaxReviewCycles)
    }
}
```

**config/errors.go — 1 line added to var block:**
```go
ErrMaxReviewCycles = errors.New("max review cycles exceeded")
```

**cmd/ralph/exit.go — 1 line changed:**
```go
// BEFORE:
if errors.Is(err, config.ErrMaxRetries) {
// AFTER:
if errors.Is(err, config.ErrMaxRetries) || errors.Is(err, config.ErrMaxReviewCycles) {
```

**Example error output:**
```
runner: review cycles exhausted (3/3) for "- [ ] Task one" (check logs for details): max review cycles exceeded
```

### Why result.OpenTasks[0].Text Is Safe

At the point where `reviewCycles >= MaxReviewIterations` fires, we are inside the review cycle loop. The review cycle loop is entered ONLY after `result.HasOpenTasks()` returned true (line 134). Therefore `result.OpenTasks[0]` is guaranteed non-empty — same safety reasoning as Story 3.9.

### Why executeAttempts Resets Inside Review Cycle Loop

When review finds issues, the previous execute was successful (commit detected). The retry counter tracks consecutive no-commit failures within ONE review cycle. Starting a new review cycle means a new execute phase — executeAttempts resets to 0 by code structure (declared inside review cycle loop).

### Existing Test Coverage Map

| AC | Test | Status |
|----|------|--------|
| AC1 (counter increments + reset) | `TestRunner_Execute_ReviewFuncSequence` | **UPDATED** |
| AC2 (emergency stop + exit 1) | `MaxReviewCyclesExhausted` + `TestExitCode_TableDriven` | **NEW** |
| AC3 (per-task counter) | `TestRunner_Execute_ReviewCyclesPerTask` | **NEW** |
| AC4 (clean review reset) | `TestRunner_Execute_ReviewFuncSequence` | **UPDATED** (same as AC1) |
| AC5 (configurable max) | `MaxReviewCyclesExhausted` "max 5" case | **NEW** (table-driven) |
| AC6 (prepared for Epic 4) | All review cycle tests use stub | Structural — verified by design |

### Previous Story Intelligence (Story 3.9)

Key learnings applied:
- **Stale doc comments**: Execute doc comment MUST be updated (recurring: 5th occurrence in Epic 3)
- **Inner error != outer prefix**: Assertions use `"review cycles exhausted"` (unique, not `"runner:"` prefix) and `"Task one"` (unique task text from fixture)
- **Sentinel text assertion**: Include `"max review cycles exceeded"` inner cause check (Story 3.9 M4 fix)
- **gofmt after Edit**: Run `go fmt` on runner.go after editing (Story 3.9 M1 fix)
- **Cross-package sentinel pattern**: config/errors.go, NOT runner/ (Story 3.6 review finding)

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
| `reviewAndMarkDoneFn(path, counter)` | test_helpers_test.go:201 | Review + mark tasks done |
| `threeOpenTasks` constant | test_helpers_test.go:16 | Three open tasks content |
| `allDoneTasks` constant | test_helpers_test.go:25 | All done tasks content |

### Existing Sentinel Errors (do NOT duplicate)

- `config.ErrMaxRetries` — retry exhaustion (cross-package)
- `config.ErrNoTasks` — no tasks found (cross-package)
- `runner.ErrNoCommit` — no commit detected (unused in prod, retained for future)
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
| `config/errors.go` | Add ErrMaxReviewCycles sentinel (1 line in var block) |
| `runner/runner.go` | Wrap execute+review in review cycle loop, capture ReviewResult |
| `runner/runner_test.go` | Update ReviewFuncSequence, add MaxReviewCyclesExhausted + PerTask |
| `runner/test_helpers_test.go` | Add MaxReviewIterations: 3 to testConfig |
| `cmd/ralph/exit.go` | Add ErrMaxReviewCycles to exitPartial mapping (1 line) |
| `cmd/ralph/exit_test.go` | Add 2 table cases for ErrMaxReviewCycles |

**Files NOT to modify:**
- `config/config.go` — MaxReviewIterations already in Config struct
- `config/defaults.yaml` — max_review_iterations: 3 already set
- `runner/git.go` — No git changes
- `runner/scan.go` — No scan changes
- `runner/knowledge.go` — No knowledge changes
- `cmd/ralph/main.go` — Error display already handles all errors via color.Red

**No new files** — all changes in existing files.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.10] — AC and technical requirements
- [Source: runner/runner.go:112-237] — Execute method (to add review cycle loop)
- [Source: runner/runner.go:231] — Current ReviewFn call discarding Clean (`_`)
- [Source: config/errors.go:9-12] — Existing sentinel var block (to add ErrMaxReviewCycles)
- [Source: cmd/ralph/exit.go:43-45] — ErrMaxRetries check (to add ErrMaxReviewCycles)
- [Source: runner/runner_test.go:601-650] — TestRunner_Execute_ReviewFuncSequence (to update)
- [Source: runner/runner_test.go:1148-1249] — TestRunner_Execute_MaxRetriesExhausted (pattern for new test)
- [Source: runner/test_helpers_test.go:140-147] — testConfig helper (to add MaxReviewIterations)
- [Source: cmd/ralph/exit_test.go:33-35] — Exit code table (to add ErrMaxReviewCycles cases)
- [Source: config/config.go:23] — MaxReviewIterations field already in Config struct
- [Source: config/defaults.yaml:4] — max_review_iterations: 3 already in defaults
- [Source: .claude/rules/go-testing-patterns.md] — 50+ testing patterns

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None — clean implementation, all tests passed first try.

### Completion Notes List

- Task 1: Added `ErrMaxReviewCycles` sentinel to `config/errors.go` var block (1 line)
- Task 2: Restructured `runner.Execute` — wrapped retry loop + ReviewFn in review cycle loop. `reviewCycles` per-task, `executeAttempts` per review cycle. Captured `ReviewResult`, break on clean, error on max
- Task 3: Added `ErrMaxReviewCycles` to exitPartial mapping in `exit.go` (one `||` addition), updated doc comment
- Task 4: Added `MaxReviewIterations: 3` to `testConfig` helper (prevents zero-value traps)
- Task 5: Updated `ReviewFuncSequence` test — now exercises review cycle loop with maxIterations=1
- Task 6: Added `MaxReviewCyclesExhausted` table-driven test — "max 3" and "max 5" cases, symmetric with MaxRetriesExhausted (proves no retry/backoff artifacts)
- Task 7: Added `ReviewCyclesPerTask` test — proves counter resets per-task (low threshold 2, no error across 2 tasks)
- Task 8: Added 2 exit code table cases: `ErrMaxReviewCycles` and wrapped variant → `exitPartial`
- Task 9: Full test suite passes — zero failures, zero regressions

### Change Log

- 2026-02-26: Implemented Story 3.10 — review cycle loop with emergency stop at MaxReviewIterations (FR24)
- 2026-02-26: Addressed code review findings — 6 items resolved (4M/2L)

## Senior Developer Review (AI)

**Review Outcome:** Changes Requested
**Review Date:** 2026-02-26
**Total Findings:** 6 (0 High, 4 Medium, 2 Low)

### Action Items

- [x] [M1] File List incomplete — `go fmt` side effects not documented [runner/git.go, runner/runner_integration_test.go, runner/scan_test.go]
- [x] [M2] Unused `wantReviewCount` field in MaxReviewCyclesExhausted — added reviewCount tracking + assertion [runner/runner_test.go:710]
- [x] [M3] New sentinel ErrMaxReviewCycles not in config/errors_test.go sentinel unwrap table — added 3 cases [config/errors_test.go]
- [x] [M4] Incorrect AC reference "(AC4)" in code comment — removed wrong AC ref [runner/runner.go:163]
- [x] [L1] Doc comment "review on success" ambiguous — rewritten for clarity [runner/runner.go:105-112]
- [x] [L2] Unnecessary os.WriteFile in ReviewFuncSequence — removed dead code [runner/runner_test.go:638]

### File List

- `config/errors.go` — added `ErrMaxReviewCycles` sentinel
- `config/errors_test.go` — added 3 ErrMaxReviewCycles sentinel unwrap cases (M3)
- `runner/runner.go` — restructured Execute with review cycle loop, updated doc comment (M4, L1)
- `runner/runner_test.go` — updated ReviewFuncSequence, added MaxReviewCyclesExhausted + ReviewCyclesPerTask, review count tracking (M2, L2)
- `runner/test_helpers_test.go` — added MaxReviewIterations to testConfig
- `runner/git.go` — `go fmt` cosmetic (blank line removal)
- `runner/runner_integration_test.go` — `go fmt` cosmetic (trailing newline)
- `runner/scan_test.go` — `go fmt` cosmetic (struct alignment)
- `cmd/ralph/exit.go` — added ErrMaxReviewCycles to exitPartial mapping, updated doc comment
- `cmd/ralph/exit_test.go` — added 2 table cases for ErrMaxReviewCycles
