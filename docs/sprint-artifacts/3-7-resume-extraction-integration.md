# Story 3.7: Resume-Extraction Integration

Status: done

## Story

As a system,
I want to resume interrupted execute sessions via `claude --resume`,
so that WIP progress is saved and execution state is recorded before retry.

## Acceptance Criteria

1. **Resume-extraction invokes --resume with session_id**
   - Given: execute session returned `session_id "abc-123"` but no commit
   - When: resume-extraction triggers
   - Then: invokes `claude --resume abc-123` (FR9); session type is `resume-extraction` (not fresh execute)

2. **Resume-extraction creates WIP commit**
   - Given: resume-extraction session completes
   - When: checking git state
   - Then: WIP commit exists (partial progress saved); commit message indicates WIP status
   - Note: WIP commit is a side effect of the resumed Claude session, not runner Go code. Runner only orchestrates the `--resume` invocation. Verified in integration test (Story 3.11)

3. **Resume-extraction writes progress to sprint-tasks.md**
   - Given: resume-extraction session runs
   - When: it interacts with sprint-tasks.md
   - Then: progress notes added (but NOT task status change); Mutation Asymmetry preserved — no `[x]` marking
   - Note: Progress notes are written by the resumed Claude session, not runner Go code. Runner verifies Mutation Asymmetry via test assertion that `[x]` count is unchanged

4. **KnowledgeWriter interface defined (no-op)**
   - Given: `KnowledgeWriter` interface in `runner/knowledge.go`
   - When: resume-extraction calls `KnowledgeWriter.WriteProgress(ctx, data)`
   - Then: no-op implementation returns `nil`; interface has 1 method now (`WriteProgress`), extensible to max 2 in Epic 6; designed for extension

5. **KnowledgeWriter no-op does not write LEARNINGS.md**
   - Given: no-op `KnowledgeWriter` implementation
   - When: resume-extraction completes
   - Then: LEARNINGS.md is NOT created or modified (FR17 deferred to Epic 6)

6. **Resume-extraction session uses correct flags**
   - Given: `session_id` from previous execute
   - When: resume-extraction invoked
   - Then: uses `--resume` flag via `session.Options.Resume` field; inherits `max_turns`, `Model`, and `DangerouslySkipPermissions` from config (mirrors execute session Options)

## Tasks / Subtasks

- [x] Task 1: Define KnowledgeWriter interface and types (AC: 4, 5)
  - [x] 1.1 Create `runner/knowledge.go` with `KnowledgeWriter` interface — 1 method: `WriteProgress(ctx context.Context, data ProgressData) error`. Doc comment: "Epic 6 adds `WriteLessons` method"
  - [x] 1.2 Define `ProgressData` struct (`SessionID string`, `TaskDescription string`). TaskDescription = first open task text from latest ScanResult, or empty string if unavailable
  - [x] 1.3 Implement `NoOpKnowledgeWriter` struct — `WriteProgress` returns nil
  - [x] 1.4 Compile-time interface check: `var _ KnowledgeWriter = (*NoOpKnowledgeWriter)(nil)`

- [x] Task 2: Wire KnowledgeWriter into Runner (AC: 4)
  - [x] 2.1 Add `Knowledge KnowledgeWriter` field to `Runner` struct (same level as `ReviewFn`, `ResumeExtractFn`, `SleepFn` — injectable dependency, NOT on `RunConfig`)
  - [x] 2.2 Set default `Knowledge: &NoOpKnowledgeWriter{}` in `Run()` function

- [x] Task 3: Implement ResumeExtraction function (AC: 1, 6)
  - [x] 3.1 Create `ResumeExtraction(ctx context.Context, cfg *config.Config, kw KnowledgeWriter, sessionID string) error` in `runner/runner.go`
  - [x] 3.2 Early return nil if `sessionID == ""` (nothing to resume)
  - [x] 3.3 Build `session.Options{Resume: sessionID, Command: cfg.ClaudeCommand, Dir: cfg.ProjectRoot, MaxTurns: cfg.MaxTurns, Model: cfg.ModelExecute, OutputJSON: true, DangerouslySkipPermissions: true}` — mirror execute session Options
  - [x] 3.4 Call `session.Execute(ctx, opts)`, measure elapsed, `session.ParseResult(raw, elapsed)`
  - [x] 3.5 Call `kw.WriteProgress(ctx, ProgressData{SessionID: sr.SessionID, TaskDescription: taskDesc})` where taskDesc passed from caller or empty
  - [x] 3.6 Wrap all errors consistently: `"runner: resume extraction: <op>: %w"` (execute / parse / write progress)

- [x] Task 4: Replace stub and update docs (AC: 1)
  - [x] 4.1 In `Run()`: replace `defaultResumeExtractStub` with closure calling `ResumeExtraction(ctx, r.Cfg, r.Knowledge, sessionID)`
  - [x] 4.2 Remove `defaultResumeExtractStub` function
  - [x] 4.3 Update doc comments on `ResumeExtractFunc` type and `Runner` struct

- [x] Task 5: Tests for KnowledgeWriter (AC: 4, 5)
  - [x] 5.1 Create `runner/knowledge_test.go`
  - [x] 5.2 `TestNoOpKnowledgeWriter_WriteProgress_ReturnsNil` — verify nil return with non-zero ProgressData
  - [x] 5.3 `TestNoOpKnowledgeWriter_WriteProgress_NoLearningsFile` — call WriteProgress, verify LEARNINGS.md not created (AC5)

- [x] Task 6: Table-driven tests for ResumeExtraction (AC: 1, 3, 6)
  - [x] 6.1 Create `TestResumeExtraction_Scenarios` table-driven test in `runner/runner_test.go`
  - [x] 6.2 Table struct fields: `name`, `sessionID`, `scenarioSteps`, `knowledgeErr`, `wantErr`, `wantErrContains`, `wantErrContainsInner`, `wantWriteProgressCount`, `wantSessionInvoked bool`, `checkArgs func(t, args)`
  - [x] 6.3 Test cases:
    - `"valid session ID"` — verifies `--resume` flag + sessionID in mock args, max_turns + Model inherited, WriteProgress called once, `session.Options.Prompt` is empty
    - `"empty session ID"` — returns nil, session NOT invoked, WriteProgress NOT called
    - `"session execute error"` — wraps with outer `"runner: resume extraction:"` + inner cause
    - `"parse result error"` — wraps with `"runner: resume extraction: parse:"` + inner cause (separate TestResumeExtraction_ParseError)
    - `"write progress error"` — wraps with `"runner: resume extraction: write progress:"` + inner cause
    - `"mutation asymmetry preserved"` — sprint-tasks.md `[x]` count unchanged after resume
  - [x] 6.4 Each error case verifies BOTH outer prefix (from ResumeExtraction) AND inner cause (from dependency). When tested through Execute() retry path, also verify caller prefix `"runner: retry: resume extract:"`

- [x] Task 7: Update test infrastructure (AC: 4)
  - [x] 7.1 Add `Knowledge: &NoOpKnowledgeWriter{}` to ALL existing test Runner structs in `runner_test.go` (nil-pointer safety, M3 pattern)
  - [x] 7.2 Add `trackingKnowledgeWriter` mock type to `test_helpers_test.go` with fields: `writeProgressCount int`, `writeProgressData []ProgressData`, `writeProgressErr error`
  - [x] 7.3 Verify all pre-existing tests pass with new Knowledge field

### Review Follow-ups (AI)

- [x] [AI-Review][Medium] M1: "valid session ID" test verifies ProgressData.SessionID value via checkData callback `[runner/runner_test.go]`
- [x] [AI-Review][Medium] M2: "session execute error" wantErrContainsInner changed to "/nonexistent/binary" for unique inner cause verification `[runner/runner_test.go]`
- [x] [AI-Review][Medium] M3: NoLearningsFile test now uses projectRoot via chdir for meaningful LEARNINGS.md check `[runner/knowledge_test.go]`
- [x] [AI-Review][Medium] M4: Added TODO comment documenting TaskDescription as unreachable until Epic 6 plumbing `[runner/runner.go]`
- [x] [AI-Review][Low] L1: NoOpKnowledgeWriter doc comment fixed from "all methods" to "WriteProgress" `[runner/knowledge.go]`
- [x] [AI-Review][Low] L2: Added zero-value ProgressData test case `[runner/knowledge_test.go]`
- [x] [AI-Review][Low] L3: wantErrContains made specific: "runner: resume extraction: execute:" for execute error case `[runner/runner_test.go]`

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Review Outcome:** Changes Requested (all auto-fixed)
**Reviewer Model:** Claude Opus 4.6

**Findings:** 7 total — 0 High, 4 Medium, 3 Low
**All 7 action items resolved in this session.**

### Action Items

- [x] M1: ProgressData.SessionID not verified in "valid session ID" test
- [x] M2: Inner error assertion matches outer prefix, not inner cause
- [x] M3: NoLearningsFile test vacuous — tmpDir never linked to KnowledgeWriter
- [x] M4: TaskDescription field unreachable in current architecture
- [x] L1: Doc comment "all methods" misleading with 1 method
- [x] L2: ProgressData zero-value not tested
- [x] L3: Nonspecific wantErrContains in error cases

## Dev Notes

### Architecture Constraints

- **Dependency direction**: `runner → session, config` (no reverse imports, no cycles)
- **Mutation Asymmetry** (Epic 3 invariant): execute/resume sessions may write files, but runner Go code NEVER writes `[x]`/`[ ]` task markers. Only review phase (Epic 4) marks tasks done
- **AC2/AC3 are Claude-side behaviors**: WIP commit creation and progress note writing happen INSIDE the resumed Claude CLI session, not in runner Go code. Runner orchestrates the `--resume` invocation; Claude does the actual work. These are verified in integration test (Story 3.11) where mock Claude simulates commit creation
- **KnowledgeWriter scope in Epic 3**: 1 method (`WriteProgress`), no-op implementation. Epic 6 adds `WriteLessons` method and real implementation. FR17 (LEARNINGS.md) explicitly deferred
- **Session Resume/Prompt mutual exclusivity**: `session.Options.Resume` and `session.Options.Prompt` are mutually exclusive in `buildArgs()`. When Resume is set, Prompt is NOT sent. Claude continues from previous session context

### Code Patterns from Previous Stories

**Injectable dependencies on Runner struct (Story 3.5/3.6 pattern):**
```go
type Runner struct {
    ReviewFn        ReviewFunc          // Story 3.5
    ResumeExtractFn ResumeExtractFunc   // Story 3.6 — type already defined
    SleepFn         func(time.Duration) // Story 3.6
    Knowledge       KnowledgeWriter     // Story 3.7 — NEW (on Runner, NOT RunConfig)
}
```
`Knowledge` lives on `Runner` (injectable dependency) not `RunConfig` (static parameters). Same pattern as `ReviewFn`, `ResumeExtractFn`, `SleepFn`. Defaults set in `Run()`, overridden in tests. ALL fields must be initialized in test Runner structs (M3 pattern).

**Error wrapping (ALL returns in a function):**
```go
return fmt.Errorf("runner: resume extraction: execute: %w", err)
return fmt.Errorf("runner: resume extraction: parse: %w", err)
return fmt.Errorf("runner: resume extraction: write progress: %w", err)
```
Tests verify BOTH outer prefix AND inner cause via `strings.Contains`. When tested through Execute() retry path, also check caller prefix `"runner: retry: resume extract:"`.

**ResumeExtractFunc type (already defined in runner.go):**
```go
type ResumeExtractFunc func(ctx context.Context, rc RunConfig, sessionID string) error
```
Called at retry point in Execute(): `r.ResumeExtractFn(ctx, rc, sessionID)`.
Real implementation is a closure in Run() that calls `ResumeExtraction(ctx, r.Cfg, r.Knowledge, sessionID)`.

### Resume Session Flow

```
execute session (exit 0, no commit)
    ↓ needsRetry = true, executeAttempts++
    ↓
ResumeExtractFn(ctx, rc, sessionID)
    ├─ closure calls ResumeExtraction(ctx, cfg, kw, sessionID)
    ├─ sessionID == "" → return nil (skip)
    ├─ session.Execute(ctx, opts{Resume: sessionID, MaxTurns, Model, ...})
    │   └─ Claude continues previous conversation
    │   └─ Claude may create WIP commit + write progress notes (AC2/AC3)
    ├─ session.ParseResult(raw, elapsed)
    ├─ kw.WriteProgress(ctx, ProgressData{...})  // no-op in Epic 3
    └─ return nil
    ↓
RecoverDirtyState(ctx, r.Git)
    ↓
exponential backoff + SleepFn
    ↓
continue → fresh execute (new prompt, NOT resume)
```

Key: resume-extraction is a one-time invocation to capture WIP state. Next retry uses fresh execute prompt. The new SessionID from resume is NOT reused.

### Session Package Integration

**session.Options already supports Resume (Story 1.9):**
```go
type Options struct {
    Command                    string
    Dir                        string
    Prompt                     string  // -p flag (empty when resuming)
    Resume                     string  // --resume session_id
    MaxTurns                   int
    Model                      string
    OutputJSON                 bool
    DangerouslySkipPermissions bool
}
```
`buildArgs()` at `session/session.go:88-92`: if `Resume != ""`, adds `--resume <id>`; else if `Prompt != ""`, adds `-p <prompt>`.
Resume Options MUST mirror execute Options (MaxTurns, Model, DangerouslySkipPermissions, OutputJSON) for consistency.

**SessionResult from session/result.go:**
```go
type SessionResult struct {
    SessionID string        // from JSON "session_id"
    ExitCode  int
    Output    string        // from JSON "result"
    Duration  time.Duration
}
```
Resume session returns a NEW SessionResult with potentially different SessionID.

### KnowledgeWriter Design (Minimal Contract)

```go
// runner/knowledge.go

// ProgressData carries resume session outcome for knowledge tracking.
// Epic 6 may add fields (backward-compatible).
type ProgressData struct {
    SessionID       string // from resumed session's SessionResult
    TaskDescription string // first open task text from ScanResult, or ""
}

// KnowledgeWriter records execution progress and lessons.
// Epic 3: 1 method (WriteProgress). Epic 6 adds WriteLessons.
type KnowledgeWriter interface {
    WriteProgress(ctx context.Context, data ProgressData) error
}

type NoOpKnowledgeWriter struct{}

func (n *NoOpKnowledgeWriter) WriteProgress(_ context.Context, _ ProgressData) error {
    return nil
}
```
- 1 method in Epic 3. Epic 6 adds `WriteLessons(ctx, LessonsData) error` when FR17 is implemented
- `ProgressData` has minimum fields. Epic 6 may add fields (backward-compatible, no signature change)
- `NoOpKnowledgeWriter` is the default in `Run()` — real implementation in Epic 6

### Testing Patterns

**Self-reexec mock for session.Execute:**
- Test binary (`os.Args[0]`) used as `ClaudeCommand`
- `testutil.SetupMockClaude(t, scenario)` returns executable path + state dir
- `testutil.ReadInvocationArgs(t, stateDir, index)` verifies CLI args
- Verify `--resume` and sessionID appear in invocation args

**Mock KnowledgeWriter for call tracking:**
```go
type trackingKnowledgeWriter struct {
    writeProgressCount int
    writeProgressData  []ProgressData
    writeProgressErr   error
}
```
Add to `test_helpers_test.go` alongside existing `trackingResumeExtract`.

**Table-driven ResumeExtraction tests:**
- Single `TestResumeExtraction_Scenarios` function with struct fields:
  `name`, `sessionID`, `scenarioSteps`, `knowledgeErr`, `wantErr`, `wantErrContains`, `wantErrContainsInner`, `wantWriteProgressCount`, `wantSessionInvoked`, `checkArgs`
- Each error case: verify outer prefix (`"runner: resume extraction: ..."`) AND inner cause
- Call count assertions: `wantWriteProgressCount` in every case
- Mutation Asymmetry case: verify `[x]` count unchanged in sprint-tasks.md

**Multi-layer error testing:**
- Inside ResumeExtraction: `"runner: resume extraction: execute: %w"` (inner)
- From caller in Execute(): `"runner: retry: resume extract: %w"` (outer)
- Tests through Execute() verify BOTH layers; unit tests of ResumeExtraction verify inner layer only

**Nil-pointer safety (M3 from Story 3.6):**
- ALL existing test Runner structs must initialize `Knowledge` field
- Use `&NoOpKnowledgeWriter{}` as default in tests that don't check knowledge behavior

### Previous Story Intelligence (Story 3.6)

**Key learnings to apply:**
- M1: No duplicate sentinels — check `config/errors.go` before adding new ones
- M2: DRY test closures — extract helpers on 2nd occurrence
- M3: Initialize ALL injectable fields on test Runner structs
- M5: SRP for sentinels — place in owning file, not cross-package
- M6: DRY test config — use existing `testConfig(tmpDir, maxIter)` helper

**Existing test helpers available:**
- `testConfig(tmpDir, maxIter)` — config builder (17x dedup)
- `cleanReviewFn` — ReviewFunc that returns clean result
- `noopResumeExtractFn` — no-op ResumeExtractFunc
- `noopSleepFn` — no-op sleep
- `trackingResumeExtract` — tracks call count + sessionIDs
- `trackingSleep` — tracks call count + durations
- `writeTasksFile(t, dir, content)` — creates sprint-tasks.md
- `headCommitPairs(pairs...)` — generates MockGitClient.HeadCommits sequences

**Existing sentinel errors (do NOT duplicate):**
- `config.ErrMaxRetries` — cross-package sentinel for retry exhaustion
- `config.ErrNoTasks` — no tasks found
- `runner.ErrNoCommit` — no commit detected (in runner.go)
- `runner.ErrDirtyTree`, `ErrDetachedHead`, `ErrMergeInProgress` — git errors (in git.go)

### Git Intelligence (Recent Commits)

Last 5 commits all in runner/ package:
- `3584c6b`: Story 3.5 — Runner loop skeleton with ReviewFn, Execute() method, RunConfig struct
- `16fc58f`: Stories 3.3/3.4 — GitClient, ExecGitClient, MockGitClient, RecoverDirtyState
- `9248eb8`: Story 3.2 — ScanTasks, TaskEntry, ScanResult
- `b6ebc7d`: Story 3.1 — Execute prompt template

Story 3.6 changes are in working tree (unstaged) with:
- `ResumeExtractFunc` type + `defaultResumeExtractStub` added
- `SleepFn` field added to Runner
- Inner retry loop with exponential backoff
- 12 new retry tests + tracking helpers

### Project Structure Notes

**Files to CREATE:**
| File | Purpose |
|------|---------|
| `runner/knowledge.go` | KnowledgeWriter interface, ProgressData struct, NoOpKnowledgeWriter |
| `runner/knowledge_test.go` | Tests for KnowledgeWriter and no-op implementation |

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Add Knowledge to Runner, implement ResumeExtraction, replace stub, update docs |
| `runner/runner_test.go` | Add table-driven ResumeExtraction tests, update existing tests with Knowledge field |
| `runner/test_helpers_test.go` | Add trackingKnowledgeWriter mock |

**Files NOT to modify:**
- `session/session.go` — Resume support already exists
- `session/result.go` — SessionResult already has SessionID
- `config/` — No new config fields or sentinels needed
- `runner/git.go` — No git changes needed
- `runner/scan.go` — No scanning changes needed

**Module: `github.com/bmad-ralph/bmad-ralph`**
- No new external dependencies
- Only stdlib imports: `context`, `fmt`, `time`

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story 3.7] — AC and technical requirements
- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Key Invariants] — Mutation Asymmetry, KnowledgeWriter max 2 methods
- [Source: session/session.go:31-40] — Options struct with Resume field
- [Source: session/session.go:88-92] — buildArgs() Resume/Prompt mutual exclusivity
- [Source: session/result.go:10-17] — SessionResult struct
- [Source: runner/runner.go:37-52] — ResumeExtractFunc type and stub
- [Source: runner/runner.go:114-122] — Execute session Options pattern (mirror for resume)
- [Source: runner/runner.go:168-188] — Retry loop calling ResumeExtractFn
- [Source: runner/test_helpers_test.go:151-181] — trackingResumeExtract helper
- [Source: .claude/rules/go-testing-patterns.md] — 50+ testing patterns
- [Source: docs/sprint-artifacts/3-6-runner-retry-logic.md] — Previous story dev notes

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis and independent quality review -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- Story spec used `cfg.Model` but config struct has `ModelExecute` — corrected to `cfg.ModelExecute` to match execute session pattern
- Split init pattern in `Run()` to avoid chicken-and-egg reference of `r.Knowledge` in closure before `r` exists

### Completion Notes List

- Task 1: Created `runner/knowledge.go` — KnowledgeWriter interface (1 method), ProgressData struct, NoOpKnowledgeWriter with compile-time check
- Task 2: Added `Knowledge KnowledgeWriter` field to Runner struct, default NoOpKnowledgeWriter in Run()
- Task 3: Implemented `ResumeExtraction()` — early return on empty sessionID, builds resume Options mirroring execute, wraps all 3 error paths consistently
- Task 4: Replaced `defaultResumeExtractStub` with closure calling `ResumeExtraction(ctx, cfg, r.Knowledge, sid)`. Split init pattern. Updated doc comments
- Task 5: Created `runner/knowledge_test.go` — 2 tests: ReturnsNil + NoLearningsFile
- Task 6: Created `TestResumeExtraction_Scenarios` (5 table cases) + separate `TestResumeExtraction_ParseError` for parse error via MOCK_EXIT_EMPTY. All cases verify error wrapping + mutation asymmetry
- Task 7: Added `Knowledge: &runner.NoOpKnowledgeWriter{}` to all 25 existing Runner test structs. Added `trackingKnowledgeWriter` mock. All pre-existing tests pass

### File List

- `runner/knowledge.go` — NEW: KnowledgeWriter interface, ProgressData, NoOpKnowledgeWriter
- `runner/knowledge_test.go` — NEW: Tests for NoOpKnowledgeWriter
- `runner/runner.go` — MODIFIED: Added Knowledge field to Runner, ResumeExtraction function, replaced stub with closure, updated comments
- `runner/runner_test.go` — MODIFIED: Added Knowledge field to all 25 Runner structs, added TestResumeExtraction_Scenarios + TestResumeExtraction_ParseError
- `runner/test_helpers_test.go` — MODIFIED: Added trackingKnowledgeWriter mock type
- `docs/sprint-artifacts/sprint-status.yaml` — MODIFIED: 3-7 status updated
- `docs/sprint-artifacts/3-7-resume-extraction-integration.md` — MODIFIED: Tasks marked, Dev Agent Record, File List, Change Log, Status

### Change Log

- 2026-02-26: Implemented Story 3.7 — Resume-extraction integration with KnowledgeWriter interface, ResumeExtraction function, and comprehensive tests (8 new tests, 0 regressions)
- 2026-02-26: Addressed code review findings — 7 items resolved (0H/4M/3L): ProgressData assertion, inner error specificity, vacuous test fix, doc comment accuracy, zero-value test
