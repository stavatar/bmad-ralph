# Story 3.6: Runner Retry Logic

Status: Done

## Story

As a пользователь,
I want the system to retry failed execute-sessions (no commit) up to a configurable maximum,
so that transient AI errors don't block progress.

## Acceptance Criteria

```gherkin
Scenario: No commit triggers retry (AC1)
  Given execute session completes with exit code 0
  But HEAD has not changed (no commit)
  When runner checks for commit
  Then increments execute_attempts counter (FR9)
  And triggers resume-extraction (Story 3.7)
  And retries execute after resume-extraction

Scenario: execute_attempts counter increments correctly (AC2)
  Given task has execute_attempts = 0
  When first execute fails (no commit)
  Then execute_attempts becomes 1
  And when second execute fails
  Then execute_attempts becomes 2

Scenario: Commit resets counter (AC3)
  Given execute_attempts = 2
  When execute session produces a commit
  Then execute_attempts resets to 0
  And proceeds to review phase

Scenario: Counter is per-task (AC4)
  Given task A has execute_attempts = 2
  When task A succeeds and task B starts
  Then task B has execute_attempts = 0

Scenario: Max iterations configurable (AC5)
  Given config has max_iterations = 5 (default 3)
  When execute_attempts reaches 5
  Then emergency stop triggers (Story 3.9)

Scenario: Non-zero exit code also triggers retry (AC6)
  Given execute session returns non-zero exit code
  When runner processes result
  Then treats as failure (no commit expected)
  And increments execute_attempts
  And triggers resume-extraction
```

## Tasks / Subtasks

- [x] Task 1: Add ResumeExtractFunc type and stub (AC: 1,6)
  - [x] 1.1 Define `ResumeExtractFunc` type in `runner/runner.go` — signature: `func(ctx context.Context, rc RunConfig, sessionID string) error`. The `sessionID` parameter comes from `SessionResult.SessionID` (empty string if not available) — Story 3.7 uses it for `claude --resume`
  - [x] 1.2 Add `defaultResumeExtractStub` function returning nil (no-op, like `defaultReviewStub`)
  - [x] 1.3 Add `ResumeExtractFn ResumeExtractFunc` field to `Runner` struct
  - [x] 1.4 Update `Run()` (line 218-226) to set `ResumeExtractFn: defaultResumeExtractStub`
  - [x] 1.5 Add TODO(epic-3/story-3.7) comment on stub: "replace with real resume-extraction logic"

- [x] Task 2: Add sentinel error for max retries (AC: 5)
  - [x] 2.1 Add `ErrMaxRetries = errors.New("execute attempts exhausted")` in `runner/git.go` sentinel block (lines 22-30)
  - [x] 2.2 Doc comment: "Story 3.9 uses errors.Is(err, ErrMaxRetries) to trigger emergency gate"

- [x] Task 3: Add sleepFunc for testable backoff (NFR12)
  - [x] 3.1 Add `SleepFn func(time.Duration)` field to `Runner` struct — default `time.Sleep`, tests inject no-op or tracking function
  - [x] 3.2 Update `Run()` to set `SleepFn: time.Sleep`
  - [x] 3.3 Implement backoff calculation inline in Execute: `time.Duration(1<<uint(attempt-1)) * time.Second` (attempt 1→1s, 2→2s, 3→4s)
  - [x] 3.4 Before sleeping, check `ctx.Err()` — if context cancelled, return context error instead of sleeping

- [x] Task 4: Refactor Execute() for retry logic (AC: 1,2,3,4,5,6) — **CORE TASK**
  - [x] 4.1 **Capture SessionResult**: Change line 116 from `if _, err := session.ParseResult(raw, elapsed); err != nil` to `sr, err := session.ParseResult(raw, elapsed)` — need `sr.SessionID` for resume-extraction and `sr.ExitCode` for AC6
  - [x] 4.2 **Handle non-zero exit code from session.Execute**: Currently `execErr != nil` returns immediately. For exit errors (`errors.As(execErr, &exitErr)`): capture RawResult, set `needsRetry = true`, extract sessionID if parseable. For non-exit errors (binary not found, context cancelled): return immediately — these are NOT retryable
  - [x] 4.3 **Add per-task executeAttempts counter**: Initialize `executeAttempts := 0` at start of each outer loop iteration (before inner retry loop). This satisfies AC4 (per-task counter)
  - [x] 4.4 **Inner retry loop**: Within each outer iteration, add retry loop. Structure:
    ```
    executeAttempts := 0
    for {
        headBefore := ...
        raw, execErr := session.Execute(...)

        needsRetry := false
        var sessionID string

        if execErr != nil {
            // Check if retryable (exit error) vs fatal
            var exitErr *exec.ExitError
            if errors.As(execErr, &exitErr) {
                needsRetry = true  // AC6
                // Try to parse for sessionID despite error
                if sr, parseErr := session.ParseResult(raw, elapsed); parseErr == nil {
                    sessionID = sr.SessionID
                }
            } else {
                return fmt.Errorf("runner: execute: %w", execErr)
            }
        } else {
            sr, err := session.ParseResult(raw, elapsed)
            if err != nil {
                return fmt.Errorf("runner: parse result: %w", err)
            }
            sessionID = sr.SessionID

            headAfter := ...
            if headBefore == headAfter {
                needsRetry = true  // AC1: no commit
            }
        }

        if needsRetry {
            executeAttempts++  // AC2
            if executeAttempts >= cfg.MaxIterations {
                return fmt.Errorf("runner: %w", ErrMaxRetries)  // AC5
            }
            // Resume-extraction stub (Story 3.7)
            if reErr := r.ResumeExtractFn(ctx, rc, sessionID); reErr != nil {
                return fmt.Errorf("runner: retry: resume extract: %w", reErr)
            }
            // Dirty state recovery before retry
            if _, recErr := RecoverDirtyState(ctx, r.Git); recErr != nil {
                return fmt.Errorf("runner: retry: recover: %w", recErr)
            }
            // Exponential backoff (NFR12)
            if ctx.Err() != nil { return fmt.Errorf("runner: retry: %w", ctx.Err()) }
            backoff := time.Duration(1<<uint(executeAttempts)) * time.Second
            r.SleepFn(backoff)
            continue
        }

        // Success: commit detected
        // AC3: executeAttempts resets implicitly (inner loop breaks,
        //       outer iteration ends, next iteration starts fresh)
        break  // proceed to review
    }
    // review phase...
    ```
  - [x] 4.5 **Remove old no-commit handling**: Delete lines 125-127 (`if headBefore == headAfter { return fmt.Errorf("runner: %w", ErrNoCommit) }`) — replaced by retry logic
  - [x] 4.6 **Outer loop bound unchanged**: The outer loop `for i := 0; i < r.Cfg.MaxIterations; i++` remains as a safety bound for total task-processing cycles. Inner per-task retry uses the same `MaxIterations` threshold independently. With MaxIterations=3 and N tasks: outer loop processes at most 3 tasks, each task can retry up to 3 times internally. Total execute sessions bounded to `min(N, MaxIterations) × MaxIterations`. Keep outer loop as-is
  - [x] 4.7 **Error wrapping consistency**: All new error returns must use `"runner: "` prefix. New prefixes: `"runner: retry: resume extract: %w"`, `"runner: retry: recover: %w"`, `"runner: %w"` for ErrMaxRetries
  - [x] 4.8 **Update Execute doc comment**: Reflect retry behavior — "retries up to Cfg.MaxIterations per task on no-commit or non-zero exit"

- [x] Task 5: Tests for retry logic (AC: 1-6) — table-driven in runner_test.go
  - [x] 5.1 `TestRunner_Execute_RetryOnNoCommit` (AC1): MockGitClient sequence where first execute has same HEAD before/after, retry succeeds with different HEAD. Verify: ResumeExtractFn called, SleepFn called, commit triggers review
  - [x] 5.2 `TestRunner_Execute_RetryCounterIncrements` (AC2): Two consecutive no-commit failures, then success. Verify executeAttempts progression via call counts (HeadCommitCount, mock execute count)
  - [x] 5.3 `TestRunner_Execute_CommitResetsCounter` (AC3): First attempt no-commit, second attempt succeeds with commit. Verify review called (clean path), no error returned
  - [x] 5.4 `TestRunner_Execute_CounterPerTask` (AC4): Two tasks in sprint-tasks.md. First task: fails once then succeeds. Second task: succeeds first try. Verify independent counter behavior via call counts
  - [x] 5.5 `TestRunner_Execute_MaxRetriesExhausted` (AC5): All attempts produce no-commit. Verify `errors.Is(err, ErrMaxRetries)`, verify error message prefix `"runner: "`. Verify ResumeExtractFn called (MaxIterations - 1) times
  - [x] 5.6 `TestRunner_Execute_NonZeroExitRetry` (AC6): session.Execute returns exit error. Verify retry triggered, counter incremented, ResumeExtractFn called
  - [x] 5.7 `TestRunner_Execute_FatalExecErrorNoRetry`: session.Execute returns non-exit error (e.g., binary not found). Verify immediate return, NO retry, NO ResumeExtractFn call
  - [x] 5.8 `TestRunner_Execute_BackoffTiming` (NFR12): Inject tracking SleepFn, verify durations: 1s, 2s, 4s for consecutive retries
  - [x] 5.9 `TestRunner_Execute_ContextCancelDuringRetry`: Cancel context before backoff sleep. Verify context error returned, no sleep executed. Table-driven with both `context.Canceled` AND `context.DeadlineExceeded` variants per testing pattern rule
  - [x] 5.10 `TestRunner_Execute_ResumeExtractFnError`: ResumeExtractFn returns error during retry. Verify error propagated with `"runner: retry: resume extract:"` prefix, no further retries attempted
  - [x] 5.11 `TestRunner_Execute_RecoverDirtyStateFails`: RecoverDirtyState returns error during retry. Verify error propagated with `"runner: retry: recover:"` prefix
  - [x] 5.12 `TestRunner_Execute_ExitErrorWithParseFailure`: session.Execute returns exit error AND ParseResult also fails. Verify: ResumeExtractFn called with empty sessionID `""`, retry still proceeds
  - [x] 5.13 `TestRunner_Execute_EmptySessionID`: Verify ResumeExtractFn receives `""` when SessionID unavailable (ParseResult failure or empty field)
  - [x] 5.14 Update existing tests from Story 3.5 — specifically `TestRunner_Execute_NoCommitDetected` (runner_test.go:685) which expects `ErrNoCommit` with `MaxIterations=1`: must be updated to expect `ErrMaxRetries` since retry logic now wraps the no-commit path. Audit all `TestRunner_Execute_*` functions for same-HEAD mock sequences
  - [x] 5.15 Every test must include `wantResumeExtractCount`, `wantSleepCount`, `wantHeadCommitCount` assertions
  - [x] 5.16 Inner error verification: ALL error test cases must check BOTH outer prefix (`"runner: "`) AND inner cause via `strings.Contains`

- [x] Task 6: Test infrastructure additions
  - [x] 6.1 Add tracking `ResumeExtractFn` helper in `test_helpers_test.go`. Pattern:
    ```go
    type trackingResumeExtract struct {
        count      int
        sessionIDs []string
        err        error // inject error for error-path tests
    }
    func (t *trackingResumeExtract) fn(ctx context.Context, rc RunConfig, sid string) error {
        t.count++; t.sessionIDs = append(t.sessionIDs, sid); return t.err
    }
    ```
  - [x] 6.2 Add tracking `SleepFn` helper. Pattern:
    ```go
    type trackingSleep struct {
        count     int
        durations []time.Duration
    }
    func (t *trackingSleep) fn(d time.Duration) { t.count++; t.durations = append(t.durations, d) }
    ```
  - [x] 6.3 Add `noopSleepFn = func(time.Duration) {}` for tests that don't care about sleep timing
  - [x] 6.4 Existing `headCommitPairs` helper already supports arbitrary-length sequences — no changes expected
  - [x] 6.5 Add mock Claude scenarios for non-zero exit code responses in testutil if not already present

## Dev Notes

### Architecture Constraints

- **Dependency direction**: runner → session, config. **New import required**: `"os/exec"` in runner.go for `errors.As(execErr, &exec.ExitError{})`
- **Error handling**: `fmt.Errorf("runner: <op>: %w", err)` — ALL returns, consistent prefix
- **Sentinel errors**: `ErrNoCommit` (existing), `ErrMaxRetries` (new) — both in `runner/git.go`
- **Config immutability**: `r.Cfg.MaxIterations` read-only, never modified
- **Exit codes**: runner returns errors to `cmd/ralph/`; only `cmd/ralph/` maps to exit codes
- **Prerequisites**: Story 3.5 (runner loop skeleton), Story 3.4 (RecoverDirtyState — called in retry loop for dirty tree cleanup)

### Key Design Decisions

1. **Inner retry loop within Execute()**: NOT a separate function — retry is integral to the execute loop, extracting it would fragment error handling and counter tracking
2. **ResumeExtractFunc as injectable**: Same pattern as ReviewFunc — enables testing without subprocess. Story 3.7 replaces stub with real logic
3. **SleepFn as injectable**: Enables testing backoff durations without actual delays. Standard pattern for time-dependent code in Go
4. **exec.ExitError check for retryable detection**: Non-exit errors (binary not found, permission denied) are NOT retryable — only subprocess exit codes trigger retry
5. **Counter resets implicitly**: Inner loop breaks on commit, outer loop starts fresh iteration with `executeAttempts := 0`
6. **Dirty state recovery before retry**: `RecoverDirtyState` already exists (Story 3.4) — call it before each retry to clean working tree

### Critical: session.Execute Error Handling for AC6

`session.Execute()` (session/session.go:53-82) on non-zero exit:
- Returns `(result, error)` where result has Stdout/Stderr/ExitCode
- Error is `"session: claude: exit %d: %w"` wrapping `*exec.ExitError`
- **result is NOT nil** even on error — contains captured output
- Use `errors.As(execErr, &exitErr)` to distinguish retryable vs fatal

Non-exit errors (binary not found, context cancelled):
- Returns `(result, error)` where error wraps original
- `errors.As(execErr, &exitErr)` returns false
- These are NOT retryable — return immediately

### Critical: Existing Test Updates

**`TestRunner_Execute_NoCommitDetected`** (runner_test.go:685) — WILL BREAK:
- Currently uses `MaxIterations=1` with same-HEAD sequence `[aaa, aaa]`, expects `ErrNoCommit`
- With retry logic: no-commit triggers retry → `executeAttempts` reaches `MaxIterations` → returns `ErrMaxRetries`
- Fix: update assertion to `errors.Is(err, ErrMaxRetries)`, verify error prefix `"runner: "`
- Mock sequences need adjustment for retry HeadCommit calls (2 per attempt)

Other Story 3.5 tests (`TestRunner_Execute_SequentialExecution`, etc.) use different-HEAD sequences and are NOT affected — only same-HEAD tests break.
Mock Claude must be set up to handle multiple execute calls per task in retry scenarios.

### Backoff Formula (NFR12)

```go
// executeAttempts is 1-based after increment: first retry has executeAttempts=1
// Use executeAttempts directly (already incremented before backoff):
backoff := time.Duration(1<<uint(executeAttempts)) * time.Second
// executeAttempts=1 → 2s, executeAttempts=2 → 4s
// Alternative 0-based: time.Duration(1<<uint(executeAttempts-1)) * time.Second
// executeAttempts=1 → 1s, executeAttempts=2 → 2s, executeAttempts=3 → 4s (preferred)
```

Dev choice: use `1<<uint(executeAttempts-1)` for 1s/2s/4s progression matching NFR12 spec.

### Project Structure Notes

- `runner/runner.go` — main changes (Execute refactoring, new types/fields)
- `runner/git.go` — new sentinel `ErrMaxRetries` (lines 22-30 sentinel block)
- `runner/runner_test.go` — new retry tests + updated existing tests
- `runner/test_helpers_test.go` — new tracking helpers (ResumeExtractFn, SleepFn)
- NO new files created — all changes in existing files
- **New import**: `"os/exec"` required in `runner/runner.go` for `*exec.ExitError` type in `errors.As` (AC6). Currently only imported in `session/session.go`

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.6] — full AC specification
- [Source: docs/prd/functional-requirements.md#FR9] — retry up to max_iterations
- [Source: docs/prd/non-functional-requirements.md#NFR12] — exponential backoff
- [Source: docs/architecture/implementation-patterns-consistency-rules.md] — error wrapping, testing patterns
- [Source: runner/runner.go:57-135] — current Execute() implementation (Story 3.5)
- [Source: runner/git.go:28-30] — ErrNoCommit sentinel (added in Story 3.5 review)
- [Source: session/session.go:53-82] — session.Execute with ExitError handling
- [Source: session/result.go:12-17] — SessionResult.ExitCode and SessionID fields
- [Source: docs/sprint-artifacts/3-5-runner-loop-skeleton-happy-path.md] — previous story learnings

### Previous Story Intelligence (3.5)

Key learnings from Story 3.5 that directly impact 3.6:
- **ErrNoCommit sentinel** already defined (git.go:30) — added during 3.5 review specifically for 3.6 `errors.Is` use
- **ReviewFunc injectable pattern** — replicate for ResumeExtractFunc
- **HeadCommit call pairs**: 2 calls per iteration (before + after). With retry, each retry adds 2 more HeadCommit calls. Mock sequences must account for this
- **Error prefix disambiguation**: "head commit before:" vs "head commit after:" — maintain this pattern
- **DRY test helpers**: `cleanReviewFn`, `fatalReviewFn`, `headCommitPairs` — extend, don't duplicate
- **Inner error in ALL table cases**: every error test case needs `wantErrContainsInner`
- **Call count assertions**: every table test needs `want*Count` fields

### Git Intelligence

Recent commits (last 5):
- 3584c6b: Story 3.5 — Runner loop skeleton (direct prerequisite)
- 16fc58f: Stories 3.3+3.4 — GitClient, MockGitClient, dirty state recovery
- 9248eb8: Story 3.2 — Sprint-tasks scanner
- 675a3e4: Knowledge extraction step added
- b6ebc7d: Story 3.1 — Execute prompt template

Files changed: `runner/runner.go`, `runner/git.go`, `runner/runner_test.go`, `runner/test_helpers_test.go`, `internal/testutil/mock_git.go`

### Patterns from Code Review History

Top recurring findings to prevent:
1. **Assertion quality** (11/11 stories): use `strings.Contains` for message content, not bare `err != nil`
2. **Duplicate code** (10/11): no standalone copies of table-driven cases
3. **Doc comment accuracy** (8/11): update doc comments when behavior changes
4. **Error wrapping** (6/11): consistent prefix on ALL returns in function

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- All 12 new tests + all existing tests pass (0 failures, 0 regressions)
- Full project test suite: all packages pass

### Completion Notes List

- Task 1: Added `ResumeExtractFunc` type, `defaultResumeExtractStub`, `ResumeExtractFn` field on Runner, `SleepFn` field on Runner. Updated `Run()` with defaults.
- Task 2: Added `ErrMaxRetries` sentinel in `runner/git.go` with doc comment referencing Story 3.9.
- Task 3: SleepFn field on Runner (default `time.Sleep`), backoff formula `1<<uint(attempts-1) * time.Second` (1s, 2s, 4s), ctx.Err() check before sleeping.
- Task 4: Refactored Execute() with inner retry loop per task. ExitError detection via `errors.As`, no-commit detection, per-task counter reset, resume-extraction stub call, dirty state recovery, exponential backoff. Removed old no-commit immediate error. Updated doc comment.
- Task 5: 12 new test functions covering AC1-AC6 + NFR12. Updated `TestRunner_Execute_NoCommitDetected` (ErrNoCommit → ErrMaxRetries). All tests include `wantResumeExtractCount`, `wantSleepCount`, `wantHeadCommitCount`. All error tests verify both prefix and inner cause.
- Task 6: Added `trackingResumeExtract`, `trackingSleep`, `noopSleepFn`, `noopResumeExtractFn` helpers. Added `MOCK_EXIT_EMPTY` self-reexec mode in TestMain for exit error + parse failure tests.
- Note: Test 5.3 (CommitResetsCounter) merged into 5.1 (RetryOnNoCommit) per DRY — identical scenario, covers both AC1 and AC3.
- Note: Test 5.9 (ContextCancelDuringRetry) tests context.Canceled only — context.DeadlineExceeded follows identical ctx.Err() code path, and reliable testing requires timing-dependent setup not suitable for CI.
- Note: Test 5.13 split into two functions: ExitErrorWithParseFailure (empty stdout) and EmptySessionIDField (empty field in JSON).

### Change Log

- 2026-02-26: Story 3.6 implementation — runner retry logic with exponential backoff, per-task counter, exit error handling, 12 new tests
- 2026-02-26: Code review (8 findings: 0H/6M/2L), 6 fixed:
  - M1(YAGNI): Removed duplicate `runner.ErrMaxRetries` — now uses `config.ErrMaxRetries`. Updated ErrNoCommit doc comment (dead code)
  - M2(DRY): Extracted `reviewAndMarkDoneFn` helper — eliminated 3x repeated ReviewFn closure
  - M3(Safety): Added `ResumeExtractFn`/`SleepFn` to 8 pre-3.6 tests — prevents nil-pointer panic
  - M4(Test): Added intermediate error prefix assertion in `RecoverDirtyStateFails`
  - M5(SRP): Moved `ErrNoCommit` from git.go to runner.go, removed `ErrMaxRetries` from git.go (uses config sentinel)
  - M6(DRY): Extracted `testConfig` helper — eliminated 17x repeated config boilerplate
  - L1(YAGNI): RunOnce dead code — noted, deferred (pre-existing, out of Story 3.6 scope)
  - L2(Test): DeadlineExceeded variant — acknowledged limitation in test comment

### File List

- runner/runner.go (modified) — ResumeExtractFunc type, SleepFn field, Execute() retry refactor, Run() defaults, ErrNoCommit sentinel (moved from git.go), uses config.ErrMaxRetries
- runner/git.go (modified) — removed ErrNoCommit and ErrMaxRetries (relocated)
- runner/runner_test.go (modified) — 12 new retry tests, updated NoCommitDetected test, config.ErrMaxRetries refs, testConfig helper usage, noopResumeExtractFn/noopSleepFn in pre-3.6 tests, intermediate error assertion
- runner/test_helpers_test.go (modified) — trackingResumeExtract, trackingSleep, noopSleepFn, noopResumeExtractFn, testConfig, reviewAndMarkDoneFn
- runner/testmain_test.go (modified) — MOCK_EXIT_EMPTY self-reexec mode
