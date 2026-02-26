# Story 3.5: Runner Loop Skeleton — Happy Path

Status: Done

## Story

As a пользователь `ralph run`,
I want система последовательно выполняла задачи из sprint-tasks.md, запуская свежую Claude Code сессию на каждую задачу,
so that задачи выполняются автоматически с детекцией коммитов и ревью-стабом.

## Acceptance Criteria (BDD)

### AC1: Runner executes tasks sequentially

**Given:**
- sprint-tasks.md contains 3 open tasks (`- [ ]`)
- MockGitClient returns health check OK
- MockClaude returns exit 0 with new commit for each execute

**When:** `runner.Run(ctx, cfg)` is called

**Then:**
- Executes 3 sequential Claude sessions (FR6)
- Each session is fresh (new invocation, not `--resume`) (FR7)
- Passes `--max-turns` from config (FR10)

### AC2: Runner detects commit after execute

**Given:**
- Execute session completes
- HEAD changed from "abc" to "def"

**When:** runner checks for commit

**Then:**
- Detects new commit (FR8)
- Proceeds to next phase (review stub)

### AC3: Runner stops when all tasks complete

**Given:** sprint-tasks.md has no remaining `- [ ]` tasks

**When:** runner scans for next task

**Then:**
- Returns successfully (nil error → exit code 0)
- All tasks were processed

### AC4: Runner runs health check at startup

**Given:** `ralph run` invoked

**When:** runner starts

**Then:**
- Calls `GitClient.HealthCheck` first (FR6)
- Fails with informative error if health check fails

### AC5: Runner passes config values via session.Options

**Given:** config specifies `max_turns=25` and `model="sonnet"`

**When:** runner builds `session.Options` for execute

**Then:**
- Sets `Options.MaxTurns = 25` from `cfg.MaxTurns` (session internally translates to `--max-turns`)
- Sets `Options.Model = "sonnet"` from `cfg.ModelExecute` (session internally translates to `--model`)
- Session flag constants (`flagMaxTurns`, `flagModel`) are **unexported** in session package — runner passes values via `session.Options` struct fields, NOT raw CLI flag strings

### AC6: Runner stub for review step (configurable)

**Given:** execute completed with commit

**When:** review phase would run

**Then:**
- Stub returns "clean" by default (no findings)
- Task advances to next iteration
- Stub is clearly marked as placeholder for Epic 4
- Stub accepts configurable response sequence for testing (e.g., findings N times then clean)
- This enables Story 3.10 review_cycles counter to be tested within Epic 3

### AC7: Mutation Asymmetry enforced

**Given:** runner loop completes a task

**When:** checking sprint-tasks.md modifications

**Then:**
- Runner process itself NEVER writes task status markers
- Only Claude sessions (review in Epic 4) modify task status

## Tasks / Subtasks

- [x] Task 1: Define ReviewResult and ReviewFunc types (AC: #6)
  - [x] 1.1 Add `ReviewResult` struct in `runner/runner.go`: `Clean bool` (FindingsPath deferred to Epic 4)
  - [x] 1.2 Add `ReviewFunc` type: `func(ctx context.Context, rc RunConfig) (ReviewResult, error)`
  - [x] 1.3 Implement `defaultReviewStub` function returning `ReviewResult{Clean: true}` — mark with `// TODO(epic-4): replace with real review logic` comment

- [x] Task 2: Define Runner struct for testable orchestration (AC: #1, #6)
  - [x] 2.1 Add exported `Runner` struct in `runner/runner.go`: `Cfg *config.Config`, `Git GitClient`, `TasksFile string`, `ReviewFn ReviewFunc`
  - [x] 2.2 Implement `(*Runner).Execute(ctx context.Context) error` as the loop method — this is the main loop, does NOT delegate to `RunOnce` (see Dev Notes: Execute vs RunOnce)

- [x] Task 3: Implement `Run()` public function (AC: #1, #4)
  - [x] 3.1 Create `ExecGitClient{Dir: cfg.ProjectRoot}`
  - [x] 3.2 Resolve tasks file path: `filepath.Join(cfg.ProjectRoot, "sprint-tasks.md")`
  - [x] 3.3 Create `Runner` with defaults (defaultReviewStub), call `r.Execute(ctx)`
  - [x] 3.4 Update `Run` doc comment (currently says "This stub validates CLI wiring" — must reflect real implementation)

- [x] Task 4: Implement main loop in `(*Runner).Execute()` (AC: #1, #2, #3, #4, #5, #7)
  - [x] 4.1 Health check at startup via `r.Git.HealthCheck(ctx)` — return wrapped error on failure
  - [x] 4.2 Main loop (up to `r.Cfg.MaxIterations`):
    - Read tasks file via `os.ReadFile(r.TasksFile)`
    - Call `ScanTasks(content)`:
      - If returns error (including `config.ErrNoTasks` for empty files) → return wrapped error
      - If succeeds but `result.HasOpenTasks() == false` → return nil (all tasks completed successfully)
  - [x] 4.3 Record HEAD before execute via `r.Git.HeadCommit(ctx)`
  - [x] 4.4 Assemble execute prompt via `config.AssemblePrompt(executeTemplate, ...)` — reuse same pattern as RunOnce
  - [x] 4.5 Build `session.Options` with `Model: r.Cfg.ModelExecute` (NOTE: existing RunOnce omits Model field — Execute must set it)
  - [x] 4.6 Call `session.Execute(ctx, opts)` → `session.ParseResult(raw, elapsed)` with timing
  - [x] 4.7 Record HEAD after execute via `r.Git.HeadCommit(ctx)`, compare before/after SHAs
  - [x] 4.8 If commit detected (before != after) → call `r.ReviewFn(ctx, rc)`
  - [x] 4.9 If no commit (before == after) → return error (Story 3.6 adds retry for this case)
  - [x] 4.10 Continue loop for next iteration
  - [x] 4.11 Ensure runner NEVER writes to sprint-tasks.md (mutation asymmetry)

- [x] Task 5: Update doc comments on modified functions (AC: quality)
  - [x] 5.1 Update `Run` doc comment — remove "stub" language, describe orchestration role
  - [x] 5.2 Update `RunOnce` doc comment — clarify it's a standalone single-iteration utility, NOT used by Execute loop
  - [x] 5.3 Update `RunReview` doc comment — clarify relationship: Execute uses `ReviewFunc`, RunReview is a walking skeleton function not used by the loop (retained for potential standalone use, may be retired in Epic 4)

- [x] Task 6: Write table-driven tests for Execute happy path (AC: #1, #2, #3)
  - [x] 6.1 Test sequential execution: MockClaude returns success, MockGit returns changing HEADs → verify N sessions executed
  - [x] 6.2 Test completion: all tasks done (HasOpenTasks false) → immediate return nil
  - [x] 6.3 Test ErrNoTasks: empty/invalid tasks file → wrapped error returned (NOT nil)
  - [x] 6.4 Verify call counts: HealthCheckCount == 1, HeadCommitCount == 2 per iteration (before + after)
  - [x] 6.5 Verify fresh sessions (no `--resume` flag in any execute call)

- [x] Task 7: Write tests for health check and error paths (AC: #4)
  - [x] 7.1 Health check fails (ErrDirtyTree) → error returned with "runner: health check:" prefix
  - [x] 7.2 Health check passes → loop starts (verify HealthCheckCount == 1)
  - [x] 7.3 Verify HealthCheck called before any session.Execute

- [x] Task 8: Write tests for session.Options values (AC: #5)
  - [x] 8.1 Verify `--max-turns` value in mock Claude args matches `cfg.MaxTurns`
  - [x] 8.2 Verify `--model` value in mock Claude args matches `cfg.ModelExecute` when set
  - [x] 8.3 Verify `--model` absent when `cfg.ModelExecute` is empty

- [x] Task 9: Write tests for review stub (AC: #6)
  - [x] 9.1 Default `defaultReviewStub` returns `ReviewResult{Clean: true}`
  - [x] 9.2 Custom `ReviewFunc` injected via `Runner.ReviewFn` field
  - [x] 9.3 Configurable sequence: findings N times then clean (via closure with counter)

- [x] Task 10: Write test for mutation asymmetry (AC: #7)
  - [x] 10.1 Capture sprint-tasks.md content before `Execute()`
  - [x] 10.2 After `Execute()` completes, verify file content unchanged via `bytes.Equal`

## Dev Notes

### Architecture: Runner as Orchestrator

Story 3.5 integrates building blocks from Stories 3.1-3.4 into the main execution loop. **`Run()` is the ONLY new public function.** New types: `Runner`, `ReviewResult`, `ReviewFunc`.

**Integration Map:**
| Component | Source Story | Used By Execute? |
|-----------|-------------|:---:|
| `ScanTasks()` | 3.2 | Yes — loop control (check open tasks) |
| `config.AssemblePrompt()` | 3.1/1.10 | Yes — prompt assembly (same pattern as RunOnce) |
| `session.Execute()` + `ParseResult()` | 1.8 | Yes — Claude session invocation |
| `GitClient.HealthCheck()` / `HeadCommit()` | 3.3 | Yes — startup check + commit detection |
| `RecoverDirtyState()` | 3.4 | No — deferred to Story 3.8 |
| `RunOnce()` | 3.1 | **No** — Execute implements its own per-iteration logic (see below) |
| `RunReview()` | 1.12 | **No** — replaced by `ReviewFunc` pattern |

### Key Design Decisions

1. **Execute does NOT delegate to RunOnce (CRITICAL).** `RunOnce` (runner.go:31-87) internally calls `ScanTasks`, `HealthCheck`, `AssemblePrompt`, `session.Execute`, `ParseResult`, AND `HeadCommit`. If Execute called RunOnce, there would be 3 `HeadCommit` calls per iteration (before by Execute, inside RunOnce at line 82, after by Execute), creating mock setup confusion. Instead, Execute **inlines its own per-iteration logic** reusing the same building blocks but with clean HEAD tracking (exactly 2 `HeadCommit` calls: before and after). `RunOnce` remains as a standalone utility for walking skeleton / single-shot mode — it is NOT removed.

2. **RunReview vs ReviewFunc.** `RunReview` (runner.go:114-148) is an existing walking skeleton function that runs a real Claude session for review. Execute does NOT use it. Instead, Execute calls `r.ReviewFn` — a configurable function field on `Runner` struct. Default: `defaultReviewStub` returns `ReviewResult{Clean: true}`. In Epic 4, ReviewFunc will be replaced with real review logic (possibly wrapping RunReview). RunReview is retained for now but not used by the loop.

3. **Runner struct for testability:** Public API stays `Run(ctx, cfg) error`. Internally creates `Runner` struct with `ReviewFn` field. Tests construct `Runner` directly with `MockGitClient` and custom `ReviewFn`.

4. **ReviewFunc as function type:** Enables Story 3.10 to inject sequence-based review stubs (findings N times then clean). Default returns `ReviewResult{Clean: true}`.

5. **Loop re-scans tasks each iteration:** After execute + review, re-read sprint-tasks.md and re-scan. This is correct for the real flow (Epic 4 review marks tasks `[x]`). In Story 3.5 tests, use **Option A: set MaxIterations = N** matching open task count. The runner will select the same first open task each iteration — this is expected behavior for the stub review. Real task progression happens when review (Epic 4) marks tasks `[x]`.

6. **No retry logic:** If no commit detected after execute, return error. Story 3.6 adds retry. Story 3.5 = happy path ONLY.

7. **MaxIterations as safety bound:** Loop runs at most `cfg.MaxIterations` times. Each iteration = one execute + review cycle.

8. **HEAD tracking: 2 calls per iteration (clean).** Execute calls `HeadCommit` before session.Execute and after. Compare SHAs. No redundant calls from RunOnce.

9. **Model field in session.Options.** Existing `RunOnce` (line 61-68) does NOT set `Model` in `session.Options`. Execute MUST set `Model: r.Cfg.ModelExecute`. This is a gap in RunOnce that Execute corrects.

### Design Consideration: Task Progression in Tests

**Problem:** Review stub doesn't mark tasks `[x]`, so re-scanning after each iteration finds the same first open task.

**Recommended approach (Option A):** Test with MaxIterations = N matching open task count. Verifies N sessions execute correctly. Loop mechanics verified even if same task selected each iteration. This is correct because **mutation asymmetry is about the runner process code itself** — test/mock infrastructure modifying files is fine.

For integration tests (Story 3.11), Options B/C may apply:
- **Option B:** MockClaude side-effect modifies tasks file during execute
- **Option C:** Custom ReviewFn modifies tasks file as side-effect

### Existing Code to Reuse

**RunOnce (runner/runner.go:31-87) — NOT called by Execute, reference only:**
```go
func RunOnce(ctx context.Context, rc RunConfig) error
```
Full flow: read tasks → ScanTasks → HealthCheck → AssemblePrompt → session.Execute → ParseResult → HeadCommit. Note: does NOT set `Model` in session.Options (line 61-68). Execute implements its own loop with Model support.

**RunConfig (runner/runner.go:22-26):**
```go
type RunConfig struct {
    Cfg       *config.Config
    Git       GitClient
    TasksFile string
}
```

**ScanTasks (runner/scan.go):**
```go
func ScanTasks(content string) (ScanResult, error)
```
Returns `ScanResult{OpenTasks, DoneTasks}` on success. Returns error wrapping `config.ErrNoTasks` (as `fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)`) when no task markers found. **Important distinction:** `ScanTasks` succeeds with `result.HasOpenTasks() == false` when all tasks are `[x]` done — this is the "all completed" case, NOT an error.

**GitClient (runner/git.go):**
```go
type GitClient interface {
    HealthCheck(ctx context.Context) error
    HeadCommit(ctx context.Context) (string, error)
    RestoreClean(ctx context.Context) error
}
```

**MockGitClient (internal/testutil/mock_git.go):**
```go
type MockGitClient struct {
    HealthCheckErrors []error    // sequence-based responses
    HeadCommits       []string   // sequence-based commit SHAs
    HeadCommitErrors  []error    // parallel error sequence
    RestoreCleanError error
    // Counters: HealthCheckCount, HeadCommitCount, RestoreCleanCount
}
```
- Beyond-sequence: returns nil (HealthCheck) or last element (HeadCommit)
- Call counters incremented atomically

### Error Wrapping Convention

ALL error returns from `Execute()` and `Run()` must wrap with `"runner: "` prefix:
```go
return fmt.Errorf("runner: health check: %w", err)
return fmt.Errorf("runner: read tasks: %w", err)
return fmt.Errorf("runner: scan tasks: %w", err)
return fmt.Errorf("runner: assemble prompt: %w", err)
return fmt.Errorf("runner: execute: %w", err)
return fmt.Errorf("runner: parse result: %w", err)
return fmt.Errorf("runner: head commit: %w", err)
return fmt.Errorf("runner: review: %w", err)
return fmt.Errorf("runner: no commit detected")
```
Note: Execute calls `session.ParseResult(raw, elapsed)` after `session.Execute` — requires measuring `time.Since(start)` for `elapsed` duration. Import `time` package.

### Testing Strategy

**Test package:** `package runner_test` (external test package — avoids import cycles with testutil). This is the established pattern from Story 3.4.

**Mock setup pattern (3 iterations example):**
```go
mock := &testutil.MockGitClient{
    HealthCheckErrors: []error{nil},                                    // startup health check
    HeadCommits:       []string{"aaa", "bbb", "bbb", "ccc", "ccc", "ddd"}, // 2 per iteration × 3
    // Layout: before1, after1, before2, after2, before3, after3
    // Each pair: before != after means commit detected
}
r := &runner.Runner{
    Cfg:       testCfg,  // with MaxIterations = 3
    Git:       mock,
    TasksFile: tasksFilePath,
    ReviewFn:  func(ctx context.Context, rc runner.RunConfig) (runner.ReviewResult, error) {
        return runner.ReviewResult{Clean: true}, nil
    },
}
```
Consider a `headCommitPairs(pairs ...struct{before, after string}) []string` test helper to make sequences more readable and reduce off-by-one errors.

**Call count verification (Execute does NOT call RunOnce, so counts are clean):**
```go
if mock.HealthCheckCount != 1 {
    t.Errorf("HealthCheckCount = %d, want 1", mock.HealthCheckCount)
}
// HeadCommit called exactly 2× per iteration (before + after) — no extra call from RunOnce
wantHeadCommitCount := iterations * 2
if mock.HeadCommitCount != wantHeadCommitCount {
    t.Errorf("HeadCommitCount = %d, want %d", mock.HeadCommitCount, wantHeadCommitCount)
}
```

**t.Parallel():** Tests that use `t.TempDir()` and `MockGitClient` are safe for parallel execution. Add `t.Parallel()` to table-driven subtests where applicable.

**Mutation asymmetry test pattern:**
```go
before, _ := os.ReadFile(tasksFile)
err := r.Execute(ctx)
after, _ := os.ReadFile(tasksFile)
if !bytes.Equal(before, after) {
    t.Errorf("tasks file was modified by runner — mutation asymmetry violated")
}
```

### Previous Story Learnings (from Story 3.4 review)

1. **External test package required:** `package runner_test` to avoid circular imports (testutil → runner → testutil)
2. **Error wrapping consistency:** ALL error returns in a function must use same prefix — review caught inconsistency in 3.4 (4 Medium findings)
3. **Call count assertions mandatory:** Every table-driven test should include `want*Count` fields for mock call tracking
4. **Inner error verification:** Test BOTH outer prefix and inner cause message via `strings.Contains`
5. **Beyond-sequence behavior:** MockGitClient returns nil (HealthCheck) or last element (HeadCommit) when sequence exhausted
6. **CRLF on WSL/NTFS:** Always `sed -i 's/\r$//'` after Write tool; use `strings.TrimSpace()` for cross-platform assertions

### What NOT To Do

- Do NOT use `exec.Command` without context — always `exec.CommandContext`
- Do NOT use `context.TODO()` in production code
- Do NOT call `os.Exit()` from runner package — return errors only
- Do NOT mutate `config.Config` — immutable after `Load()`
- Do NOT write to sprint-tasks.md from runner code — mutation asymmetry
- Do NOT add retry logic — that's Story 3.6
- Do NOT add resume logic — that's Stories 3.7/3.8
- Do NOT add emergency stops — that's Stories 3.9/3.10
- Do NOT add real review logic — that's Epic 4
- Do NOT inline regex compilation — use config package patterns
- Do NOT add new dependencies — stdlib only
- Do NOT log from runner package — return errors/results, let `cmd/ralph` decide output

### Project Structure Notes

- **Modify:** `runner/runner.go` — implement `Run()` body, add `Runner` struct, `ReviewResult`, `ReviewFunc`, `defaultReviewStub`, `(*Runner).Execute()`. Update doc comments on `Run`, `RunOnce`, `RunReview`.
- **Extend:** `runner/runner_test.go` — add `TestRunner_Execute_*` tests (package `runner_test`)
- **No new files needed** — all infrastructure (mock Claude, MockGitClient, prompt assembly) exists
- **Do NOT modify/remove:** `RunOnce` and `RunReview` — they remain as standalone utilities, just update their doc comments
- **Package boundary:** `runner/` depends on `config/`, `session/` — NOT vice versa
- **Test helpers:** `internal/testutil/` (MockGitClient, MockClaude scenario-based)
- **Embedded templates:** `executeTemplate` and `reviewTemplate` are `//go:embed` vars in runner.go — Execute uses `executeTemplate` for prompt assembly
- Alignment with `cmd/ralph/run.go:45` call site: `runner.Run(cmd.Context(), cfg)`

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.5] — all AC, technical notes, FR coverage
- [Source: docs/architecture/core-architectural-decisions.md] — dependency rules, context patterns, error handling
- [Source: docs/architecture/project-structure-boundaries.md] — package boundaries, data flow
- [Source: docs/architecture/implementation-patterns-consistency-rules.md] — naming conventions, testing standards
- [Source: docs/project-context.md] — condensed architecture reference
- [Source: docs/sprint-artifacts/3-4-mockgitclient-dirty-state-recovery.md] — previous story learnings, established patterns
- [Source: runner/runner.go] — RunOnce, RecoverDirtyState, RunConfig, RunReview
- [Source: runner/scan.go] — ScanTasks, ScanResult, TaskEntry
- [Source: runner/git.go] — GitClient interface, ExecGitClient, sentinel errors
- [Source: internal/testutil/mock_git.go] — MockGitClient with sequence-based responses
- [Source: .claude/rules/go-testing-patterns.md] — 50+ testing patterns from code reviews

## Dev Agent Record

### Context Reference

<!-- Generated by create-story workflow 2026-02-26 -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None — clean implementation, all tests passed on first run.

### Completion Notes List

- Implemented `ReviewResult`, `ReviewFunc`, `defaultReviewStub` types in runner/runner.go
- Implemented `Runner` struct with `Execute()` method — full task loop: health check → iterate (read tasks → scan → assemble prompt → session.Execute → HEAD check → review)
- Replaced `Run()` stub with real implementation that creates `Runner` with production defaults
- Updated doc comments on `Run`, `RunOnce`, `RunReview` to reflect new architecture
- `Execute()` does NOT delegate to `RunOnce` — avoids redundant HeadCommit calls (2 per iteration vs 3)
- `ScanTasks` error returned directly (not double-wrapped) — consistent with `RunOnce` pattern
- Extracted `TestMain` to shared `testmain_test.go` (no build tags) so both unit and integration tests use mock Claude dispatch
- Moved shared helpers (`assertArgsContainFlag`, `assertArgsContainFlagValue`, `argValueAfterFlag`, `copyFixtureToDir`) to `test_helpers_test.go` — DRY across unit and integration tests
- Added `headCommitPairs()` helper to make HeadCommits mock setup readable
- 14 test functions covering all 7 ACs: sequential execution, all-done, ErrNoTasks, health check errors, session options (max-turns + model), custom ReviewFunc, sequence-based ReviewFunc, review error propagation, mutation asymmetry, no-commit detection, HeadCommit-before failure, HeadCommit-after failure, read-tasks failure
- All tests pass (0 regressions), full suite: 4.7s

### Code Review Fixes (2026-02-26)

Review: 8 findings (0H/4M/4L), all 8 fixed:
- F1 (M): Added `ErrNoCommit` sentinel error in git.go for Story 3.6 `errors.Is` compatibility
- F2 (M): Added `TestRunner_Execute_HeadCommitAfterFails` test for post-execute HeadCommit failure path
- F3 (M): Added `wantErrContainsInner` to HealthCheckErrors table — "generic git error" now verifies inner cause
- F4 (M): Extracted `cleanReviewFn` var and `fatalReviewFn(t)` helper — replaced 9 inline closures (DRY)
- F5 (L): ErrNoTasks prefix assertion tightened: `"runner: scan tasks:"` instead of `"runner:"`
- F6 (L): Disambiguated head commit errors: `"runner: head commit before:"` / `"runner: head commit after:"`
- F7 (L): Added NOTE comment to ReviewFuncSequence explaining Clean not yet consumed (Story 3.10)
- F8 (L): Known gap documented: ParseResult error path untestable without mock infra changes

### Change Log

- 2026-02-26: Implemented Story 3.5 — Runner loop skeleton happy path (all 10 tasks, 7 ACs)
- 2026-02-26: Code review fixes — 8 findings (0H/4M/4L), all addressed

### File List

- runner/runner.go (modified) — added ReviewResult, ReviewFunc, defaultReviewStub, Runner, Execute; updated Run, doc comments; disambiguated head commit error prefixes; use ErrNoCommit sentinel
- runner/runner_test.go (modified) — 14 Execute test functions covering AC1-AC7; DRY ReviewFn helpers; inner error assertions; HeadCommitAfterFails test
- runner/git.go (modified) — added ErrNoCommit sentinel error
- runner/testmain_test.go (new) — shared TestMain with mock Claude dispatch
- runner/test_helpers_test.go (new) — shared test helpers, data constants, cleanReviewFn, fatalReviewFn
- runner/runner_integration_test.go (modified) — removed TestMain, copyFixtureToDir, and arg helper functions (moved to shared files)
- docs/sprint-artifacts/sprint-status.yaml (modified) — status: review → done
- docs/sprint-artifacts/3-5-runner-loop-skeleton-happy-path.md (modified) — task checkboxes, Dev Agent Record, review fixes, status
