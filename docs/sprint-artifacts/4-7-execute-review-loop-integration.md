# Story 4.7: Execute-Review Loop Integration

Status: Ready for Review

## Story

As a runner,
I want to inject review-findings.md content into the execute prompt on each review cycle iteration,
so that the execute session sees findings and fixes them before the next review.

## Acceptance Criteria

```gherkin
Scenario: Review replaces stub in runner loop (AC1)
  Given runner loop from Story 3.5 with configurable review stub
  When Epic 4 integration complete
  Then real review session replaces stub
  And ReviewResult contract preserved (Story 3.5)
  And runner loop code changes are minimal (seam design)

Scenario: Execute-review cycle with review_cycles counter (AC2)
  Given execute completed with commit
  When review finds 1 CONFIRMED finding
  Then review_cycles increments to 1 (Story 3.10)
  And next execute launched (reads review-findings.md)
  And after fix commit -> review again (cycle 2)

Scenario: Clean review after fix cycle (AC3)
  Given review_cycles = 1 (one previous findings cycle)
  When second review is clean
  Then [x] marked, review-findings.md cleared
  And review_cycles resets to 0
  And runner proceeds to next task

Scenario: Max review iterations triggers emergency stop (AC4)
  Given max_review_iterations = 3 (config)
  And review_cycles reaches 3 without clean review
  When runner checks counter (Story 3.10 logic)
  Then triggers emergency stop with exit code 1 (FR24)
  And message includes: task name, cycles completed, remaining findings

Scenario: Full task lifecycle in runner loop (AC5)
  Given sprint-tasks.md with 1 open task
  When runner executes full cycle:
    execute -> commit -> review -> findings -> execute -> commit -> review -> clean
  Then task marked [x]
  And 2 execute sessions + 2 review sessions launched
  And review_cycles = 0 at end

Scenario: Execute sees findings and fixes them (AC6)
  Given review-findings.md contains 2 CONFIRMED findings
  When next execute session launches
  Then execute prompt includes review-findings content (Story 3.1)
  And Claude addresses findings instead of implementing from scratch
  And runs tests after fix
  And commits on green tests

Scenario: Ralph does not distinguish first execute from fix execute (AC7)
  Given review-findings.md is non-empty
  When ralph launches execute
  Then uses same execute prompt template (Story 3.1)
  And same session type (fresh, not --resume)
  And ralph makes no distinction between "first" and "fix" (FR18)

Scenario: Each fix iteration creates separate commit (AC8)
  Given execute-review-execute-review cycle
  When each execute fixes issues
  Then each fix execute creates its own commit (FR18)
  And commits are separate from initial implementation commit
```

## Tasks / Subtasks

- [x] Task 1: Move prompt assembly inside review cycle loop and inject findings (AC: 2, 6, 7)
  - [x] 1.1 In `Runner.Execute` (runner.go:262-282), move prompt assembly from BEFORE the review cycle loop to INSIDE it (first thing inside `for {` at line 285). This ensures each execute iteration gets a fresh prompt
  - [x] 1.2 Before assembling prompt, read `review-findings.md` from `r.Cfg.ProjectRoot` via `os.ReadFile`. If `os.ErrNotExist` — empty string (not an error). If other read error — return wrapped error
  - [x] 1.3 Set `HasFindings: len(strings.TrimSpace(findingsContent)) > 0` in `config.TemplateData`. This activates the `{{if .HasFindings}}` conditional in execute.md (Story 3.1)
  - [x] 1.4 Add `"__FINDINGS_CONTENT__": findingsContent` to the replacements map alongside existing `"__FORMAT_CONTRACT__"`. This injects actual findings text where execute.md has `__FINDINGS_CONTENT__` placeholder
  - [x] 1.5 Move `opts := session.Options{...}` inside the loop too (prompt changes each iteration). Keep same fields: `Command`, `Dir`, `MaxTurns`, `Model` (ModelExecute), `OutputJSON`, `DangerouslySkipPermissions`
  - [x] 1.6 Error wrapping for findings read error: `fmt.Errorf("runner: read findings: %w", err)`
  - [x] 1.7 Verify: first iteration (no findings file) → `HasFindings: false` → prompt shows "## Proceed" section. After non-clean review → `HasFindings: true` → prompt shows "## Review Findings" section with content

- [x] Task 2: Unit test for findings injection into prompt assembly (AC: 6, 7)
  - [x] 2.1 Add test `TestRunner_Execute_FindingsInjection` in `runner/runner_test.go`. This test verifies the Runner.Execute code path reads review-findings.md and passes it to the prompt
  - [x] 2.2 Setup: `t.TempDir()`, write `sprint-tasks.md` with 1 open task, write `review-findings.md` with sample findings content ("## [HIGH] Test finding\n- **ЧТО не так** — test issue")
  - [x] 2.3 Create a `ReviewFunc` that: first call → returns `ReviewResult{Clean: false}` (triggers findings cycle); second call → writes `allDoneTasks` to sprint-tasks.md + returns `ReviewResult{Clean: true}` (exits loop). Track call count
  - [x] 2.4 Use `MockGitClient` with `headCommitPairs`: 2 pairs (one per execute iteration), each with different before/after SHAs (commit detected)
  - [x] 2.5 Assert ReviewFn called exactly 2 times (2 review cycles)
  - [x] 2.6 NOTE: Cannot directly verify prompt content without integration tests (Story 4.8). This test verifies the FLOW: findings present → cycle continues → second execute → clean review exits. The prompt assembly correctness is verified by existing `TestPrompt_Execute_*` golden tests + Story 4.8 integration tests

- [x] Task 3: Unit test for findings absent on first execute (AC: 7)
  - [x] 3.1 Add test `TestRunner_Execute_NoFindingsFirstIteration` — verify that when no review-findings.md exists, the flow works identically to existing tests (regression check)
  - [x] 3.2 This is essentially the existing happy path behavior — can be verified by confirming existing `TestRunner_Execute_*` tests still pass with the refactored code
  - [x] 3.3 If existing tests already cover this scenario adequately, skip new test (merge into Task 2 table)

- [x] Task 4: Unit test for findings read error (AC: 6)
  - [x] 4.1 Add test case for `os.ReadFile` failure on `review-findings.md` (not ErrNotExist — e.g., file-as-directory trick for WSL)
  - [x] 4.2 Verify error wraps with `"runner: read findings:"` prefix
  - [x] 4.3 Can be table-driven with Task 2 if structurally similar

- [x] Task 5: Run full test suite (AC: all)
  - [x] 5.1 `go test ./runner/` — all tests pass including new and refactored
  - [x] 5.2 `go test ./...` — no regressions
  - [x] 5.3 `go build ./...` — clean build

## Dev Notes

### Architecture Constraints

- **Minimal runner.go changes**: The loop structure, ReviewFn wiring, review_cycles counter, and emergency stop logic ALL exist. Only the prompt assembly needs adjustment
- **Dependency direction**: `runner → session, config` (unchanged). No new dependencies
- **Two-stage assembly**: `config.AssemblePrompt(executeTemplate, TemplateData{HasFindings: true}, {"__FINDINGS_CONTENT__": content})` — same pattern, now with findings
- **go:embed**: `executeTemplate` already embeds `runner/prompts/execute.md` which has `{{if .HasFindings}}` conditional
- **Config immutability**: `r.Cfg` read-only, as always

### Key Design Decision: Prompt Assembly Inside Review Cycle Loop

**Current code** (runner.go:262-282):
```go
// Prompt assembled ONCE before review cycle loop — misses findings injection
prompt, err := config.AssemblePrompt(
    executeTemplate,
    config.TemplateData{GatesEnabled: r.Cfg.GatesEnabled},
    map[string]string{"__FORMAT_CONTRACT__": config.SprintTasksFormat()},
)
opts := session.Options{..., Prompt: prompt}

reviewCycles := 0
for {
    // execute with same prompt — NEVER includes findings!
    ...
    rr, err := r.ReviewFn(ctx, rc)
    if rr.Clean { break }
    reviewCycles++ // findings exist, but next execute doesn't see them
}
```

**Required change**: Move prompt assembly inside the `for {` loop:
```go
reviewCycles := 0
for {
    // Read findings file (absent = empty, not error)
    findingsContent := readFindings(r.Cfg.ProjectRoot)

    // Assemble prompt with findings context
    prompt, err := config.AssemblePrompt(
        executeTemplate,
        config.TemplateData{
            GatesEnabled: r.Cfg.GatesEnabled,
            HasFindings: len(strings.TrimSpace(findingsContent)) > 0,
        },
        map[string]string{
            "__FORMAT_CONTRACT__": config.SprintTasksFormat(),
            "__FINDINGS_CONTENT__": findingsContent,
        },
    )
    opts := session.Options{..., Prompt: prompt}

    // Execute → review → cycle or break
    ...
}
```

This ensures:
1. First execute: review-findings.md absent → `HasFindings: false` → "## Proceed" section
2. After non-clean review: review-findings.md has content → `HasFindings: true` → "## Review Findings" section with actual findings
3. After clean review: loop breaks, clean handling cleared the file

### What Already Works (from previous stories)

| Component | Story | Status |
|-----------|-------|--------|
| `realReview` wired in `Run()` | 4.3 | Done (runner.go:461) |
| Review cycle loop + `reviewCycles` counter | 3.5, 3.10 | Done (runner.go:284-369) |
| Emergency stop `config.ErrMaxReviewCycles` | 3.10 | Done (runner.go:365-368) |
| `DetermineReviewOutcome` file-state logic | 4.3 | Done (runner.go:134-159) |
| Execute prompt `{{if .HasFindings}}` conditional | 3.1 | Done (execute.md:61-68) |
| `__FINDINGS_CONTENT__` placeholder in execute.md | 3.1 | Done (execute.md:68) |
| `config.TemplateData.HasFindings` field | 3.1 | Done (config/prompt.go:30) |
| Review prompt with sub-agents, verification, findings write | 4.1-4.6 | Done |
| `cleanReviewFn` test helper | 3.5 | Done (test_helpers_test.go:40) |
| Clean review: `[x]` marking + findings clear (prompt) | 4.5 | Done (review.md:77-90) |
| Findings write: overwrite review-findings.md (prompt) | 4.6 | Done (review.md:94-113) |

### What This Story Adds

**ONE code change**: read `review-findings.md` and inject into execute prompt per review cycle iteration.

**Estimated diff**: ~15 lines changed in `Runner.Execute`, ~50-80 lines of new tests.

### Error Handling for Findings Read

```
os.ErrNotExist → empty string, not error (normal first-execute case)
os.ReadFile success → use content
os.ReadFile other error → return fmt.Errorf("runner: read findings: %w", err)
```

This mirrors the `DetermineReviewOutcome` pattern (runner.go:151-154).

### Execute Prompt Conditional (Already Implemented, execute.md:61-74)

```markdown
{{- if .HasFindings}}

## Review Findings — MUST FIX FIRST

The following review findings were confirmed and MUST be addressed...

__FINDINGS_CONTENT__
{{- else}}

## Proceed

No pending review findings. Proceed with the next open task.
{{- end}}
```

Story 4.7 activates this conditional — previously `HasFindings` was always `false`.

### Test Strategy

**Unit tests** (this story): verify execute-review cycle flow with findings file:
- Findings present → cycle continues → second execute runs
- Findings absent → normal flow (regression)
- Findings read error → wrapped error returned

**Integration tests** (Story 4.8): verify prompt CONTENT includes findings via `ReadInvocationArgs` + self-reexec pattern.

### Story 4.6 Code Review Learnings (apply to 4.7)

- **DRY in prompts**: keep constraint statements in ONE canonical section (Invariants), not duplicated
- **Scope-creep guard completeness**: implement ALL absence guards listed in task spec
- **Never silently discard return values**: capture and assert on all returns

### Story 4.3 Code Review Learnings (apply to 4.7)

- **Unused param = doc lie**: if parameter isn't used, remove or document
- **errors.Is convention**: always `errors.Is(err, target)` not type assertions
- **Signature change cascade**: grep ALL callers when changing function signatures
- **Dead stub removal**: update doc comments immediately when replacing stubs

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `testConfig(tmpDir, maxIter)` | test_helpers_test.go:141 | Config with standard defaults |
| `cleanReviewFn` | test_helpers_test.go:40 | Clean review stub |
| `writeTasksFile(t, dir, content)` | test_helpers_test.go:56 | Write sprint-tasks.md |
| `headCommitPairs(pairs...)` | test_helpers_test.go:67 | Generate HeadCommits for MockGitClient |
| `noopSleepFn` | test_helpers_test.go:178 | No-op sleep for tests |
| `noopResumeExtractFn` | test_helpers_test.go:181 | No-op resume extract |
| `threeOpenTasks` | test_helpers_test.go:17 | Three open tasks content |
| `allDoneTasks` | test_helpers_test.go:26 | All done tasks content |
| `setupRunnerIntegration` | test_helpers_test.go:204 | Runner with all fields for integration tests |

### Sentinel Errors (do NOT duplicate)

Existing: `config.ErrMaxRetries`, `config.ErrMaxReviewCycles`, `config.ErrNoTasks`, `runner.ErrNoCommit`, `runner.ErrDirtyTree`, `runner.ErrDetachedHead`, `runner.ErrMergeInProgress`

No new sentinels needed for this story.

### KISS/DRY/SRP Analysis

**KISS:**
- ONE code change: move prompt assembly inside loop + add findings read
- Same two-stage assembly pattern, same TemplateData struct, same replacements map
- No new types, interfaces, or abstractions

**DRY:**
- Reuses `config.AssemblePrompt` (same API)
- Reuses `os.ReadFile` + `errors.Is(err, os.ErrNotExist)` pattern (same as `DetermineReviewOutcome`)
- Reuses existing test helpers (`writeTasksFile`, `testConfig`, `headCommitPairs`, `noopSleepFn`)

**SRP:**
- Story 4.7 = findings injection into execute prompt ONLY
- Loop structure (Story 3.5), review wiring (Story 4.3), review prompt content (4.1-4.6) untouched
- Integration test validation deferred to Story 4.8

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Refactor `Runner.Execute`: move prompt assembly inside review cycle loop, add findings reading (~15 lines changed) |
| `runner/runner_test.go` | Add 2-3 test cases for findings injection flow + findings read error |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/prompts/execute.md:61-74` | `{{if .HasFindings}}` conditional — verify placeholder exists |
| `config/prompt.go:25-40` | `TemplateData.HasFindings` field — verify it exists |
| `runner/test_helpers_test.go` | Existing test helpers to reuse |
| `config/constants.go` | Constants referenced in assertions |

**Files NOT to create**: No new files. All changes go in existing `runner/runner.go` and `runner/runner_test.go`.

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.7] — AC and technical requirements
- [Source: runner/runner.go:236-373] — `Runner.Execute` function (loop to refactor)
- [Source: runner/runner.go:262-271] — Current prompt assembly (to move inside loop)
- [Source: runner/runner.go:134-159] — `DetermineReviewOutcome` (os.ReadFile + os.ErrNotExist pattern)
- [Source: runner/prompts/execute.md:61-74] — `{{if .HasFindings}}` conditional + `__FINDINGS_CONTENT__`
- [Source: config/prompt.go:30] — `HasFindings bool` field in TemplateData
- [Source: runner/test_helpers_test.go] — Existing test helpers
- [Source: docs/sprint-artifacts/4-6-findings-write.md#Dev Notes] — Story 4.6 learnings
- [Source: docs/sprint-artifacts/4-3-review-session-logic.md#Dev Notes] — Story 4.3 learnings
- [Source: .claude/rules/code-quality-patterns.md] — Error wrapping, stale doc comments
- [Source: .claude/rules/test-error-patterns.md] — Error testing patterns

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: Moved prompt assembly + opts creation inside review cycle `for {}` loop in `Runner.Execute`. Added findings file reading with `os.ReadFile` + `errors.Is(err, os.ErrNotExist)` pattern. Sets `HasFindings` in `TemplateData` and injects `__FINDINGS_CONTENT__` into replacements map. Updated doc comment to reflect new flow order. ~15 lines changed.
- Task 2: Added `TestRunner_Execute_FindingsInjection` — verifies 2-cycle flow: findings present → review not-clean → second execute → review clean → exit. Asserts ReviewFn called exactly 2 times and HeadCommitCount == 4.
- Task 3: Existing 28 Execute tests already cover "no findings file" scenario (none write review-findings.md). All pass after refactor — no new test needed per Task 3.3.
- Task 4: Added `TestRunner_Execute_FindingsReadError` — file-as-directory trick causes non-ErrNotExist error. Asserts `"runner: read findings:"` prefix in error message.
- Task 5: Full suite passes: `go test ./runner/` (all pass), `go test ./...` (zero regressions), `go build ./...` (clean).

### Implementation Plan

- ONE code change in `Runner.Execute`: moved prompt assembly inside review cycle loop, added findings reading
- Same two-stage assembly pattern (`config.AssemblePrompt`), same `TemplateData` struct
- Reused `os.ReadFile` + `errors.Is(err, os.ErrNotExist)` pattern from `DetermineReviewOutcome`
- Reused existing test helpers: `testConfig`, `headCommitPairs`, `writeTasksFile`, `noopSleepFn`, `noopResumeExtractFn`

### File List

- `runner/runner.go` — modified: moved prompt assembly inside review cycle loop, added findings file reading
- `runner/runner_test.go` — modified: added `TestRunner_Execute_FindingsInjection`, `TestRunner_Execute_FindingsReadError`
- `docs/sprint-artifacts/4-7-execute-review-loop-integration.md` — modified: tasks marked complete, dev record updated
- `docs/sprint-artifacts/sprint-status.yaml` — modified: story status ready-for-dev → in-progress

### Change Log

- 2026-02-27: Implemented execute-review loop integration — findings injection into execute prompt per review cycle iteration
