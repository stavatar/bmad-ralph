# Story 4.3: Review Session Logic

Status: done

## Story

As a runner,
I want to launch a review session after successful execute (commit detected), passing the review prompt to a fresh Claude Code session,
so that code changes are reviewed before the task is marked complete.

## Acceptance Criteria

```gherkin
Scenario: Review launches after commit detection (AC1)
  Given execute session completed with new commit (FR8)
  When runner proceeds to review phase
  Then launches fresh Claude Code session with review prompt (FR13, FR14)
  And session uses --max-turns from config
  And session is independent of execute session context

Scenario: Review replaces stub from Story 3.5 (AC2)
  Given runner loop with ReviewResult contract
  When review session completes
  Then returns ReviewResult{Clean: bool} matching Story 3.5 contract
  And FindingsPath deferred — runner detects findings via os.Stat (AC5)

Scenario: Review session captures output (AC3)
  Given review session runs
  When Claude processes review prompt
  Then session.Execute returns SessionResult with session_id
  And runner can determine review outcome from file state

Scenario: Review session uses fresh context (AC4)
  Given execute session was session abc-123
  When review session launches
  Then review session gets new session_id (not --resume)
  And review has no memory of execute session internals (FR14)

Scenario: Review outcome determined from file state (AC5)
  Given review session completed
  When runner checks sprint-tasks.md for [x] on current task
  And checks review-findings.md via os.Stat + os.ErrNotExist
  Then computes ReviewResult{Clean: true} if [x] present and findings absent/empty
  Or computes ReviewResult{Clean: false} if findings file non-empty (FindingsPath not needed — path is known)
  And this is the Go code bridge between Claude behavior and ReviewResult contract
```

## Tasks / Subtasks

- [x] Task 1: Implement `realReview` function replacing `defaultReviewStub` (AC: 1, 2, 3, 4)
  - [ ] 1.1 Create function `realReview(ctx context.Context, rc RunConfig) (ReviewResult, error)` in `runner/runner.go` (NOT a new file — MVP runner boundary)
  - [ ] 1.2 Assemble review prompt via `config.AssemblePrompt(reviewTemplate, config.TemplateData{}, replacements)` — review prompt uses `__TASK_CONTENT__` placeholder for current task text
  - [ ] 1.3 Build `session.Options` with: `Prompt`, `MaxTurns` from `rc.Cfg.MaxTurns`, `Model` from `rc.Cfg.ModelReview` (NOT ModelExecute), `OutputJSON: true`, `DangerouslySkipPermissions: true`, `Dir: rc.Cfg.ProjectRoot`, `Command: rc.Cfg.ClaudeCommand`
  - [ ] 1.4 Call `session.Execute(ctx, opts)` — fresh session (no --resume)
  - [ ] 1.5 Parse result via `session.ParseResult(raw, elapsed)` — extract session_id for logging
  - [ ] 1.6 On `session.Execute` error: if `*exec.ExitError` → log but proceed to file-state check (review may have partially written); else → return wrapped error
  - [ ] 1.7 After session completes, call `determineReviewOutcome()` (Task 2) to compute ReviewResult from file state

- [x] Task 2: Implement `determineReviewOutcome` function (AC: 5)
  - [ ] 2.1 Function signature: `determineReviewOutcome(tasksFile string, currentTaskText string, projectRoot string) (ReviewResult, error)`
  - [ ] 2.2 Check if current task is now `[x]` in sprint-tasks.md: re-read file, scan for task text, check if line now matches `config.TaskDoneRegex`
  - [ ] 2.3 Check review-findings.md: `os.Stat(filepath.Join(projectRoot, "review-findings.md"))` — absent = clean (`errors.Is(err, os.ErrNotExist)`)
  - [ ] 2.4 If file exists: `os.ReadFile` → check `len(strings.TrimSpace(content)) > 0` for non-empty
  - [ ] 2.5 Logic: `Clean = taskMarkedDone AND (findingsAbsent OR findingsEmpty)`. If task not marked done but no findings — treat as NOT clean (review session may have failed before writing anything)
  - [ ] 2.6 Return `ReviewResult{Clean: clean}` — no FindingsPath field needed yet (ReviewResult only has Clean bool per Story 3.5 contract)
  - [ ] 2.7 Error wrapping: `fmt.Errorf("runner: determine review outcome: %w", err)` for ALL returns

- [x] Task 3: Wire `realReview` into `Run()` replacing `defaultReviewStub` (AC: 2)
  - [ ] 3.1 In `Run()` function (runner.go:348-361): change `ReviewFn: defaultReviewStub` → `ReviewFn: realReview`
  - [ ] 3.2 Delete `defaultReviewStub` — after replacing with `realReview` it becomes dead code. Tests use `cleanReviewFn` (test_helpers_test.go:40), which is an independent function. Also remove the TODO(epic-4) comment above it
  - [ ] 3.3 Update `RunReview` doc comment: mark as "Deprecated: use realReview via Run() instead" — RunReview was walking skeleton, realReview replaces it. Do NOT delete RunReview yet (may break integration tests)

- [x] Task 4: Pass current task text to review function (AC: 1, 5)
  - [ ] 4.1 The review prompt needs `__TASK_CONTENT__` — this is the text of the current open task from sprint-tasks.md
  - [ ] 4.2 `RunConfig` already has `TasksFile` — `realReview` can read and scan tasks to get `result.OpenTasks[0].Text`
  - [ ] 4.3 Pass the task text into `__TASK_CONTENT__` replacement in AssemblePrompt
  - [ ] 4.4 Also pass task text to `determineReviewOutcome` for [x] detection

- [x] Task 5: realReview test coverage plan (AC: 1-4)
  - [ ] 5.1 DEFERRED TO STORY 4.8: `realReview` calls `session.Execute` (subprocess) — cannot unit-test without interface-for-testing-only pattern. Story 4.8 integration tests cover realReview end-to-end via MockClaude self-reexec
  - [ ] 5.2 Document test scenarios for Story 4.8 in a code comment above `realReview`:
    - HappyPath_CleanReview: mock session → task [x] + no findings → Clean: true
    - HappyPath_WithFindings: mock session → findings file with content → Clean: false
    - SessionError_ExitError: *exec.ExitError → proceed to file-state check
    - SessionError_Fatal: non-ExitError → return wrapped error
    - FreshSession: verify opts has no Resume field (AC4)
    - UsesModelReview: opts.Model == cfg.ModelReview (NOT ModelExecute)
  - [ ] 5.3 THIS STORY tests realReview ONLY through `determineReviewOutcome` (Task 6) — the pure-function part that is independently testable

- [x] Task 6: Write unit tests for `determineReviewOutcome` (AC: 5)
  - [ ] 6.1 Test name: `TestDetermineReviewOutcome_CleanReview` — task marked [x] + no findings file → Clean: true
  - [ ] 6.2 Test name: `TestDetermineReviewOutcome_CleanReview_EmptyFindings` — task [x] + findings file exists but empty → Clean: true
  - [ ] 6.3 Test name: `TestDetermineReviewOutcome_WithFindings` — task NOT [x] + findings file with content → Clean: false
  - [ ] 6.4 Test name: `TestDetermineReviewOutcome_TaskDoneButFindings` — task [x] + findings non-empty → ambiguous, treat as Clean: false (findings take precedence)
  - [ ] 6.5 Test name: `TestDetermineReviewOutcome_NoChangeNoFindings` — task NOT [x] + no findings → Clean: false (session failed silently)
  - [ ] 6.6 Test name: `TestDetermineReviewOutcome_ReadError` — tasks file unreadable → wrapped error
  - [ ] 6.7 All tests use `t.TempDir()` for file isolation
  - [ ] 6.8 Table-driven where appropriate (merge similar positive/negative cases)

- [x] Task 7: Run full test suite (AC: all)
  - [ ] 7.1 `go test ./runner/` — all tests pass including new determineReviewOutcome tests
  - [ ] 7.2 `go test ./...` — no regressions
  - [ ] 7.3 `go build ./...` — clean build

## Dev Notes

### Architecture Constraints

- **Runner boundary**: All new code goes in `runner/runner.go`. Do NOT create `runner/review.go` — MVP keeps runner as single file until >1000 LOC (currently ~400 LOC, adding ~80-100 LOC)
- **Dependency direction**: `runner → session, config` (unchanged). No new dependencies
- **ReviewResult contract**: `ReviewResult{Clean bool}` — established in Story 3.5. Do NOT add FindingsPath field yet — the AC says it but the actual struct only has `Clean bool` per existing code
- **ReviewFunc signature**: `func(ctx context.Context, rc RunConfig) (ReviewResult, error)` — `realReview` MUST match this exactly
- **Config immutability**: `rc.Cfg` passed by pointer, never mutated
- **Two-stage prompt assembly**: `config.AssemblePrompt(reviewTemplate, TemplateData{}, replacements)` — same pattern as execute prompt

### Key Design Decision: File-State-Based Review Outcome

Ralph does NOT parse Claude's review output. Instead:
1. Claude (inside review session) decides: clean → marks [x] + clears findings; not clean → writes findings
2. Ralph checks file state AFTER session completes:
   - Re-reads sprint-tasks.md → checks if current task now has `[x]`
   - Checks review-findings.md existence and content via `os.Stat` + `os.ReadFile`
3. This decoupling means Ralph doesn't need to understand Claude's output format

**Architecture pattern**: `os.Stat` + `errors.Is(err, os.ErrNotExist)` → absent = empty (same as Story 4.5 AC5)

### Session Model Selection

The review session MUST use `cfg.ModelReview` (not `cfg.ModelExecute`). These are separate config fields:
- `ModelExecute` — for execute sessions (implementation)
- `ModelReview` — for review sessions (code review)

Both default to empty string (Claude CLI default). Users can set different models for execute vs review.

### Current review.md Prompt

```
Review the code changes for the following task.

Task:
__TASK_CONTENT__
```

This is minimal. Stories 4.4, 4.5, 4.6 will expand it with verification, clean handling, and findings write instructions. For Story 4.3, the prompt is sufficient to test the session launch mechanism.

### Task Identification for [x] Check

After review session completes, `determineReviewOutcome` needs to check if the current task was marked `[x]`. Strategy:
1. Before review: capture `result.OpenTasks[0].Text` (the task line text)
2. After review: re-read sprint-tasks.md, find line matching task text, check if now matches `config.TaskDoneRegex`
3. Match by substring (task description text) not by line number (lines may shift if review edits file)

### Error Handling in Review Session

Review session errors need nuanced handling:
- `*exec.ExitError` (non-zero exit) — Claude may have partially written files. PROCEED to file-state check (the review may have succeeded in writing [x] before crashing)
- Other errors (binary not found, context canceled) — ABORT, return error immediately
- This matches execute session error handling pattern from runner.go:193-204

### KISS/DRY/SRP Analysis

**KISS:**
- `realReview` = one function, assembles prompt + calls session + checks file state
- `determineReviewOutcome` = pure function, reads files + returns result. No side effects
- No new types, interfaces, or abstractions

**DRY:**
- Reuses `config.AssemblePrompt` (same as execute prompt assembly)
- Reuses `session.Execute` + `session.ParseResult` (same pattern as RunOnce/Execute)
- Reuses `config.TaskDoneRegex` for [x] detection (same regex as ScanTasks)
- `session.Options` construction same pattern as Execute loop (runner.go:165-173)

**SRP:**
- `realReview` = orchestration (prompt → session → outcome check)
- `determineReviewOutcome` = pure file-state logic (testable independently)
- Separation: prompt content (Stories 4.4-4.6) vs session mechanics (this story) vs loop wiring (Story 4.7)

### Story 4.2 Code Review Learnings (apply to 4.3)

- **Unused param = doc lie**: if a function parameter isn't used, remove it or document why it's there for future use
- **errors.Is convention**: always `errors.Is(err, target)` not type assertions for sentinel detection
- **Signature change cascade**: if you change a function signature, grep ALL callers including integration tests
- **Dead stub removal**: when replacing a stub with real implementation, update doc comments on the stub immediately

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `testConfig(tmpDir, maxIter)` | test_helpers_test.go | Config with standard defaults |
| `cleanReviewFn` | test_helpers_test.go | Clean review stub (for other tests, NOT for testing realReview) |
| `writeTasksFile(t, dir, content)` | test_helpers_test.go | Write sprint-tasks.md |
| `goldenTest(t, name, got)` | prompt_test.go | Golden file comparison |
| `threeOpenTasks` constant | test_helpers_test.go | Three open tasks content |
| `allDoneTasks` constant | test_helpers_test.go | All done tasks content |

### Sentinel Errors (do NOT duplicate)

Existing: `config.ErrMaxRetries`, `config.ErrMaxReviewCycles`, `config.ErrNoTasks`, `runner.ErrNoCommit`, `runner.ErrDirtyTree`, `runner.ErrDetachedHead`, `runner.ErrMergeInProgress`

No new sentinels needed for this story.

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Add `realReview` function, add `determineReviewOutcome` function, update `Run()` to use `realReview`, delete `defaultReviewStub` (dead code), deprecate `RunReview` doc comment |
| `runner/runner_test.go` | Add tests for `determineReviewOutcome` (6+ test cases, table-driven) |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/prompts/review.md` | Review prompt template (has `__TASK_CONTENT__` placeholder) |
| `runner/scan.go` | ScanTasks logic, TaskDoneRegex usage pattern |
| `session/session.go` | session.Execute API, session.Options struct |
| `config/config.go` | Config struct (ModelReview field), CLIFlags |
| `config/constants.go` | TaskDoneRegex, TaskOpenRegex patterns |
| `runner/test_helpers_test.go` | Existing test helpers to reuse |

**Files NOT to create**: No new files. All code goes in existing runner/runner.go and runner/runner_test.go.

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.3] — AC and technical requirements
- [Source: runner/runner.go:43-61] — ReviewResult struct, ReviewFunc type, defaultReviewStub (to be deleted)
- [Source: runner/runner.go:348-361] — Run() function wiring defaultReviewStub
- [Source: runner/runner.go:363-400] — RunReview walking skeleton (to be deprecated)
- [Source: runner/runner.go:165-173] — session.Options construction pattern for execute
- [Source: runner/runner.go:193-204] — ExitError vs fatal error handling pattern
- [Source: runner/scan.go:41-70] — ScanTasks logic, TaskDoneRegex usage
- [Source: runner/prompts/review.md] — Review prompt with __TASK_CONTENT__ placeholder
- [Source: session/session.go:31-40] — session.Options struct
- [Source: config/config.go:17-35] — Config struct with ModelReview, MaxTurns
- [Source: docs/project-context.md#File I/O] — os.Stat + os.ErrNotExist pattern
- [Source: docs/sprint-artifacts/4-2-sub-agent-prompts.md#Dev Notes] — Story 4.2 learnings
- [Source: .claude/rules/code-quality-patterns.md] — Stale doc comments, error wrapping, unused params
- [Source: .claude/rules/test-error-patterns.md] — Error testing patterns (inner error, multi-layer)

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1-4: `realReview` implemented in runner.go with fresh session (ModelReview, no Resume), ExitError → file-state check, fatal → wrapped error
- Task 2: `DetermineReviewOutcome` exported for testing. Uses `taskDescription` helper to extract description from checkbox line and match by substring after re-reading sprint-tasks.md
- Task 3: `defaultReviewStub` deleted, `Run()` wires `realReview`, `RunReview` marked Deprecated
- Task 5: Test scenarios documented in doc comment above `realReview` (6 integration test scenarios for Story 4.8)
- Task 6: 7 table-driven test cases covering all AC5 combinations: clean/dirty task × absent/empty/non-empty findings + read error
- Task 7: `go test ./...` all pass, `go build ./...` clean
- Design decision: `DetermineReviewOutcome` uses `os.ReadFile` for findings (not os.Stat + ReadFile) — simpler, same behavior with `errors.Is(err, os.ErrNotExist)` guard

### File List

- runner/runner.go — Added realReview, DetermineReviewOutcome, taskDescription; deleted defaultReviewStub; wired realReview in Run(); deprecated RunReview
- runner/runner_test.go — Added TestDetermineReviewOutcome_Scenarios (7 table-driven cases)
