# Story 5.3: Retry with Feedback

Status: ready-for-dev

## Story

As a developer running `ralph run --gates`,
I want to select retry with feedback at a gate point,
so that AI can address my comments when re-executing the task.

## Acceptance Criteria

### AC1: Retry prompts for feedback text

```gherkin
Scenario: Retry prompts for feedback text
  Given gate prompt displayed
  When user enters "r"
  Then system displays feedback prompt on gate.Writer:
    Enter feedback (empty line to submit):
    > _
  And waits for user input via existing lineCh (same scanner goroutine)
```

### AC2: Multi-line feedback collected until empty line

```gherkin
Scenario: Multi-line feedback collected until empty line
  Given feedback prompt displayed
  When user types "fix validation" then Enter, "add tests" then Enter, then empty Enter
  Then lines collected: ["fix validation", "add tests"]
  And joined with space into single string: "fix validation add tests"
  And returned in GateDecision.Feedback
```

### AC3: GateDecision includes feedback

```gherkin
Scenario: GateDecision includes feedback
  Given user chose retry with feedback "Need to add validation"
  When gates.Prompt returns
  Then returns GateDecision{Action: "retry", Feedback: "Need to add validation"}
  And error is nil (valid input, not an error)
```

### AC4: Feedback injected into sprint-tasks.md

```gherkin
Scenario: Feedback injected into sprint-tasks.md
  Given retry decision with Feedback "Need validation"
  When runner processes retry
  Then sprint-tasks.md updated with indented line under current task:
    - [ ] Setup project [GATE]
      > USER FEEDBACK: Need validation
  And uses config.FeedbackPrefix constant (">" + " USER FEEDBACK:") (FR22)
  And Ralph writes programmatically via os.WriteFile (not Claude)
```

### AC5: Existing feedback lines preserved

```gherkin
Scenario: Existing feedback lines preserved
  Given sprint-tasks.md already has feedback under task:
    - [x] Setup project [GATE]
      > USER FEEDBACK: First attempt feedback
  When retry with new feedback "Second attempt feedback"
  Then new feedback appended AFTER existing feedback lines:
    - [ ] Setup project [GATE]
      > USER FEEDBACK: First attempt feedback
      > USER FEEDBACK: Second attempt feedback
  And existing feedback NOT overwritten or removed
```

### AC6: Task reverted [x] to [ ] on retry

```gherkin
Scenario: Task reverted on retry
  Given task "Setup project [GATE]" marked [x] after clean review
  When runner processes retry decision
  Then task line changed from "- [x]" to "- [ ]" in sprint-tasks.md
  And task description and [GATE] tag preserved
  And other tasks in file unchanged
```

### AC7: Counters reset on retry

```gherkin
Scenario: Counters reset on retry
  Given retry decision processed
  When runner continues to re-execute task
  Then execute_attempts starts from 0 (fresh review cycle)
  And review_cycles starts from 0 (outer loop re-initializes)
  And achieved via outer for-loop continue (local vars re-initialized)
```

### AC8: Error handling during feedback input

```gherkin
Scenario: EOF during feedback input
  Given feedback prompt waiting for input
  When io.Reader reaches EOF before empty line
  Then returns error wrapped with "gates: prompt:" prefix
  And errors.Is(err, io.EOF) is true
  And decision is nil

Scenario: Context cancel during feedback input
  Given feedback prompt waiting for input
  When ctx is cancelled
  Then returns error wrapped with "gates: prompt:" prefix
  And errors.Is(err, context.Canceled) is true
  And decision is nil
```

## Tasks / Subtasks

- [x] Task 1: Enhance Prompt for feedback input after "r" (AC: #1, #2, #3, #8)
  - [x] 1.1: In `gates/gates.go` Prompt function, replace immediate return on "r" case with feedback collection flow
  - [x] 1.2: Display feedback prompt to gate.Writer: `"Enter feedback (empty line to submit):\n> "` — use same errColor or plain fmt
  - [x] 1.3: Read lines from existing `lineCh` in loop. For each line: `strings.TrimSpace(r.text)` — empty string terminates collection
  - [x] 1.4: Collect non-empty trimmed lines into `[]string` slice, then `strings.Join(lines, " ")` into single feedback string
  - [x] 1.5: Handle EOF during feedback: same as main loop — return `fmt.Errorf("gates: prompt: %w", r.err)` or `io.EOF` if channel closed
  - [x] 1.6: Handle ctx.Done() during feedback: `select` on `ctx.Done()` vs `lineCh` — return `fmt.Errorf("gates: prompt: %w", ctx.Err())`
  - [x] 1.7: Return `&config.GateDecision{Action: config.ActionRetry, Feedback: joined}` — Feedback may be empty if user immediately enters empty line
- [x] Task 2: Add InjectFeedback function (AC: #4, #5)
  - [x] 2.1: In `runner/` package (runner.go or new file e.g. feedback.go), define `func InjectFeedback(tasksFile, taskDesc, feedback string) error`
  - [x] 2.2: Read `tasksFile` via `os.ReadFile`
  - [x] 2.3: Split content into lines, find line index containing `taskDesc` (use `strings.Contains(line, taskDesc)` — same pattern as `DetermineReviewOutcome`)
  - [x] 2.4: Walk forward from task line: skip consecutive lines starting with whitespace (existing feedback/metadata lines under task). This is the insertion point
  - [x] 2.5: Build feedback line: `"  " + config.FeedbackPrefix + " " + feedback` (2-space indent + prefix + space + text)
  - [x] 2.6: Insert feedback line at insertion point via slice operations
  - [x] 2.7: Write back to file via `os.WriteFile` with joined lines + trailing newline
  - [x] 2.8: Error wrapping: `fmt.Errorf("runner: inject feedback: %w", err)` for read/write errors
  - [x] 2.9: Task not found: return `fmt.Errorf("runner: inject feedback: task not found: %s", taskDesc)` — distinct error for debugging
- [x] Task 3: Add RevertTask function (AC: #6)
  - [x] 3.1: In `runner/` package (same file as InjectFeedback), define `func RevertTask(tasksFile, taskDesc string) error`
  - [x] 3.2: Read `tasksFile` via `os.ReadFile`
  - [x] 3.3: Split content into lines, find line containing `taskDesc` AND matching `config.TaskDoneRegex` (must be `[x]` line)
  - [x] 3.4: Replace `config.TaskDone` ("- [x]") with `config.TaskOpen` ("- [ ]") on that line via `strings.Replace(line, config.TaskDone, config.TaskOpen, 1)` — first occurrence only
  - [x] 3.5: Write back to file via `os.WriteFile`
  - [x] 3.6: Error wrapping: `fmt.Errorf("runner: revert task: %w", err)` for read/write errors
  - [x] 3.7: Task not found: return `fmt.Errorf("runner: revert task: task not found: %s", taskDesc)` — distinct error message
- [x] Task 4: Wire retry handling in Runner.Execute gate check (AC: #4, #6, #7)
  - [x] 4.1: In `runner/runner.go`, modify gate check block added by Story 5.2: replace no-op retry handling with proper retry logic
  - [x] 4.2: After `decision.Action == config.ActionRetry` check, extract taskDesc via `taskDescription(result.OpenTasks[0].Text)`
  - [x] 4.3: If `decision.Feedback != ""`, call `InjectFeedback(r.TasksFile, taskDesc, decision.Feedback)` — skip injection when feedback empty
  - [x] 4.4: Call `RevertTask(r.TasksFile, taskDesc)` — always revert [x] → [ ] on retry
  - [x] 4.5: `continue` outer for-loop — natural reset of `executeAttempts` (line ~310) and `reviewCycles` (line ~272) via local variable initialization at loop start
  - [x] 4.6: Error propagation for inject: pass through InjectFeedback error directly (already wraps with "runner: inject feedback:" prefix) — avoid double "runner:" wrap per ScanTasks pass-through pattern
  - [x] 4.7: Error propagation for revert: pass through RevertTask error directly (already wraps with "runner: revert task:" prefix) — avoid double "runner:" wrap per ScanTasks pass-through pattern
- [x] Task 5: Write gates tests (AC: #1, #2, #3, #8)
  - [x] 5.1: `TestPrompt_RetryWithFeedback` — table-driven: `"r\nfix validation\n\n"` → Action=retry, Feedback="fix validation"; `"r\nline1\nline2\n\n"` → Feedback="line1 line2"; `"r\n  spaced  \n\n"` → Feedback="spaced" (trimmed). Verify: decision.Action, decision.Feedback, err==nil
  - [x] 5.2: `TestPrompt_RetryEmptyFeedback` — `"r\n\n"` → Action=retry, Feedback="" (immediate empty line). Verify: valid decision, not error
  - [x] 5.3: `TestPrompt_RetryFeedbackEOF` — `"r\npartial"` (no empty line terminator, EOF). Verify: `errors.Is(err, io.EOF)`, decision==nil
  - [x] 5.4: `TestPrompt_RetryFeedbackContextCancel` — use `io.Pipe()` reader, pre-cancel context. Type "r\n" then block on feedback. Verify: `errors.Is(err, context.Canceled)`, decision==nil. Note: need to deliver "r" line before ctx cancels — use goroutine writing to pipe with timing
- [x] Task 6: Write runner tests (AC: #4, #5, #6, #7)
  - [x] 6.1: `TestInjectFeedback_Scenarios` — table-driven: basic injection (verify line appears after task, indented, with prefix), preserve existing feedback (new line after existing), task not found error, multiple tasks (inject on correct one), os.ReadFile error (non-existent path → wraps with "runner: inject feedback:" prefix). Use `t.TempDir()` + `writeTasksFile` helper
  - [x] 6.2: `TestRevertTask_Scenarios` — table-driven: basic revert `[x]→[ ]` (verify task text preserved, other tasks unchanged), task not found error (no [x] line matching), task already [ ] (error — not found as [x]), os.ReadFile error (non-existent path → wraps with "runner: revert task:" prefix). Use `t.TempDir()` + `writeTasksFile` helper
  - [x] 6.3: `TestRunner_Execute_GateRetry` — construct Runner with mock GatePromptFn returning retry+feedback. Verify: GatePromptFn called, sprint-tasks.md contains feedback line, task reverted to [ ], runner re-processes same task (mock sequence: 1st run→[x]+gate retry, 2nd run→[x]+gate approve). Use existing test infra: MockClaude, MockGitClient, writeTasksFile
  - [x] 6.4: `TestRunner_Execute_GateRetryEmptyFeedback` — retry with Feedback="". Verify: no feedback line injected, task still reverted, runner re-processes
  - [x] 6.5: `TestRunner_Execute_GateRetryInjectError` — InjectFeedback fails (e.g., non-existent tasksFile path). Verify: error propagated with "runner: inject feedback:" prefix (InjectFeedback wraps, caller passes through per ScanTasks pattern). Note: requires error injection via file-system trick (e.g., set Runner.TasksFile to non-existent path after initial scan succeeds)

## Dev Notes

### Architecture Constraints

- **Dependency direction:** `runner → gates` is valid, `gates → config` only. `gates` MUST NOT import `runner`
- **Mutation Asymmetry exception:** Retry is the ONLY place Ralph changes task status markers `[x] → [ ]`. This is documented and intentional — not a violation of the normal pattern where only Claude modifies task markers
- **Feedback is content injection, not task status change:** adding `> USER FEEDBACK:` lines does NOT violate Mutation Asymmetry — it's metadata injection alongside the task
- **No `os.Exit` in runner** — errors propagate up, `cmd/ralph/exit.go` handles exit codes

### What Already Exists (DO NOT RECREATE)

| Component | Location | Status |
|-----------|----------|--------|
| `GateDecision` type | `config/errors.go:28-37` | Action + Feedback fields, implements error |
| `GateDecision.Feedback` field | `config/errors.go:32` | string field, ready for use |
| `config.FeedbackPrefix` | `config/constants.go:11` | `"> USER FEEDBACK:"` |
| `config.ActionRetry` | `config/constants.go:18` | `"retry"` |
| `config.TaskOpen` / `config.TaskDone` | `config/constants.go:8-9` | `"- [ ]"` / `"- [x]"` |
| `config.TaskDoneRegex` | `config/constants.go:28` | Regex for [x] lines |
| `gates.Prompt` function | `gates/gates.go:33-85` | Returns GateDecision — enhance for feedback |
| `readResult` type | `gates/gates.go:24-27` | Used by scanner goroutine — reuse for feedback |
| `lineCh` channel | `gates/gates.go:35` | Same channel for main prompt AND feedback reads |
| `taskDescription()` | `runner/runner.go:169-175` | Strips checkbox prefix — reuse for matching |
| `GatePromptFunc` type | Story 5.2 adds | `func(ctx, taskText) (*GateDecision, error)` |
| Gate check block | Story 5.2 adds | After clean review, before outer loop continue |
| `writeTasksFile` helper | `runner/test_helpers_test.go:56-63` | Test helper for fixture setup |
| `cleanReviewFn` | `runner/test_helpers_test.go:40-42` | Shared clean review mock |

### Story 5.2 Creates (Prerequisites — Assumed Complete)

| Component | Location | Expected |
|-----------|----------|----------|
| `GatePromptFunc` type | `runner/runner.go` | `type GatePromptFunc func(ctx context.Context, taskText string) (*config.GateDecision, error)` |
| `GatePromptFn` field | `Runner` struct | Injectable gate prompt function |
| Gate check block | `Runner.Execute` | After clean review break, before outer loop continues |
| Retry no-op | Gate check block | `ActionRetry` treated as approve — Story 5.3 replaces with proper handling |

### Critical: Feedback Collection in gates.Prompt

Current code returns immediately on "r":
```go
case "r":
    return &config.GateDecision{Action: config.ActionRetry}, nil
```

Story 5.3 replaces this with feedback collection flow:
```go
case "r":
    // Display feedback prompt
    fmt.Fprintln(gate.Writer, "Enter feedback (empty line to submit):")
    fmt.Fprint(gate.Writer, "> ")

    var feedbackLines []string
    for {
        select {
        case <-ctx.Done():
            return nil, fmt.Errorf("gates: prompt: %w", ctx.Err())
        case r, ok := <-lineCh:
            if !ok || r.err != nil {
                err := r.err
                if !ok {
                    err = io.EOF
                }
                return nil, fmt.Errorf("gates: prompt: %w", err)
            }
            trimmed := strings.TrimSpace(r.text)
            if trimmed == "" {
                // Empty line = submit
                feedback := strings.Join(feedbackLines, " ")
                return &config.GateDecision{
                    Action:   config.ActionRetry,
                    Feedback: feedback,
                }, nil
            }
            feedbackLines = append(feedbackLines, trimmed)
            fmt.Fprint(gate.Writer, "> ") // prompt for next line
        }
    }
```

Key design decisions:
- **Same lineCh channel** — no new goroutine, reuses existing scanner goroutine from Prompt
- **Trimmed lines joined with space** — single-line feedback format for sprint-tasks.md
- **Empty trimmed line = submit** — whitespace-only counts as empty
- **Empty feedback valid** — `"r\n\n"` returns Feedback="" (runner skips injection)

### Critical: Retry Handling in Runner.Execute

Story 5.2 gate check block (with retry no-op) becomes:
```go
// Gate check — ONLY for tasks with [GATE] tag and gates enabled
if r.Cfg.GatesEnabled && result.OpenTasks[0].HasGate && r.GatePromptFn != nil {
    decision, gateErr := r.GatePromptFn(ctx, result.OpenTasks[0].Text)
    if gateErr != nil {
        return fmt.Errorf("runner: gate: %w", gateErr)
    }
    if decision.Action == config.ActionQuit {
        return fmt.Errorf("runner: gate: %w", decision)
    }
    if decision.Action == config.ActionRetry {
        taskDesc := taskDescription(result.OpenTasks[0].Text)
        if decision.Feedback != "" {
            if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
                return err // InjectFeedback wraps with "runner: inject feedback:" prefix
            }
        }
        if err := RevertTask(r.TasksFile, taskDesc); err != nil {
            return err // RevertTask wraps with "runner: revert task:" prefix
        }
        continue // outer for-loop: re-reads tasks, finds [ ], starts fresh
    }
    // approve, skip → continue to next task (fall through)
}
```

**Counter reset mechanism:** `continue` restarts the outer `for i := 0; i < r.Cfg.MaxIterations; i++` loop body. This naturally re-initializes:
- `reviewCycles := 0` (line ~272)
- `executeAttempts := 0` (line ~310)

No explicit reset code needed — the loop structure handles it.

**Note:** The outer loop `i` counter increments on retry. This means retries count toward MaxIterations. This is intentional — prevents infinite retry loops. If the user keeps retrying, eventually MaxIterations is reached and runner stops.

### InjectFeedback Implementation Detail

```go
func InjectFeedback(tasksFile, taskDesc, feedback string) error {
    content, err := os.ReadFile(tasksFile)
    if err != nil {
        return fmt.Errorf("runner: inject feedback: %w", err)
    }

    lines := strings.Split(string(content), "\n")
    taskIdx := -1
    for i, line := range lines {
        if strings.Contains(line, taskDesc) {
            taskIdx = i
            break
        }
    }
    if taskIdx < 0 {
        return fmt.Errorf("runner: inject feedback: task not found: %s", taskDesc)
    }

    // Find insertion point: after task line + consecutive indented lines
    insertIdx := taskIdx + 1
    for insertIdx < len(lines) && len(lines[insertIdx]) > 0 && (lines[insertIdx][0] == ' ' || lines[insertIdx][0] == '\t') {
        insertIdx++
    }

    feedbackLine := "  " + config.FeedbackPrefix + " " + feedback
    // Insert at insertIdx
    lines = append(lines[:insertIdx], append([]string{feedbackLine}, lines[insertIdx:]...)...)

    return os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")), 0644)
}
```

### RevertTask Implementation Detail

```go
func RevertTask(tasksFile, taskDesc string) error {
    content, err := os.ReadFile(tasksFile)
    if err != nil {
        return fmt.Errorf("runner: revert task: %w", err)
    }

    lines := strings.Split(string(content), "\n")
    found := false
    for i, line := range lines {
        if strings.Contains(line, taskDesc) && config.TaskDoneRegex.MatchString(line) {
            lines[i] = strings.Replace(line, config.TaskDone, config.TaskOpen, 1)
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("runner: revert task: task not found: %s", taskDesc)
    }

    return os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")), 0644)
}
```

### Feedback Line Format

```
  > USER FEEDBACK: actual feedback text
^^                 ^^^^^^^^^^^^^^^^^^^^^
2 spaces indent    user's feedback (joined if multi-line)
```

- Prefix: `config.FeedbackPrefix` = `"> USER FEEDBACK:"`
- Full line: `"  " + config.FeedbackPrefix + " " + feedback`
- Multi-line user input joined with space: `["line1", "line2"]` → `"line1 line2"`
- Empty feedback (immediate Enter): Feedback="" — runner skips InjectFeedback call

### Testing Approach

**gates tests (gates/gates_test.go):**
- **Table-driven** for feedback scenarios (single line, multi-line, trimmed whitespace)
- **io.Reader injection** — `strings.NewReader("r\nfeedback\n\n")` for input
- **bytes.Buffer** for output verification (feedback prompt text displayed)
- **io.Pipe()** for context cancel test — deliver "r\n" via goroutine, then cancel ctx
- **errors.Is** for EOF and context.Canceled checks

**runner tests (runner/runner_test.go):**
- **Table-driven** for InjectFeedback and RevertTask scenarios
- **t.TempDir()** for file isolation
- **writeTasksFile** helper for fixture setup
- **File content verification** via `os.ReadFile` + `strings.Contains`
- **Runner.Execute retry test** — mock GatePromptFn returning retry first, approve second. MockClaude + MockGitClient infrastructure. Verify: feedback injected, task reverted, re-processed

### Naming Conventions

- Functions: `InjectFeedback`, `RevertTask` — imperative, descriptive
- Error prefixes: `"runner: inject feedback:"`, `"runner: revert task:"` — package: operation
- Test names: `TestPrompt_RetryWithFeedback`, `TestInjectFeedback_Scenarios`, `TestRevertTask_Scenarios`, `TestRunner_Execute_GateRetry`
- File placement: `InjectFeedback` and `RevertTask` in `runner/runner.go` or new `runner/feedback.go` — developer's choice

### What This Story Does NOT Include

- Checkpoint gates every N tasks (Story 5.4)
- Emergency gate upgrade to interactive (Story 5.5)
- Integration tests with real gates.Prompt (Story 5.6)
- Feedback display styling/color (KISS — plain text prompt)
- Feedback persistence across sessions (feedback is in sprint-tasks.md, naturally persisted)

### Project Structure Notes

- **Modified file:** `gates/gates.go` — enhance Prompt "r" case with feedback collection
- **Modified file:** `gates/gates_test.go` — new feedback test cases (4 test functions)
- **Modified file:** `runner/runner.go` — add InjectFeedback, RevertTask functions; modify gate check for retry. OR:
- **New file (optional):** `runner/feedback.go` — InjectFeedback + RevertTask if developer prefers SRP separation
- **Modified file:** `runner/runner_test.go` — new test cases for InjectFeedback, RevertTask, Runner.Execute retry

### References

- [Source: docs/epics/epic-5-human-gates-control-stories.md#Story 5.3]
- [Source: docs/project-context.md#Dependency Direction] — runner → gates valid
- [Source: docs/project-context.md#Package Entry Points] — gates.Prompt(ctx, gate)
- [Source: gates/gates.go:33-85] — Prompt function (enhance "r" case)
- [Source: gates/gates.go:74-75] — Current "r" case (replace with feedback flow)
- [Source: runner/runner.go:169-175] — taskDescription() helper (reuse for matching)
- [Source: runner/runner.go:245-396] — Execute method (modify gate check for retry)
- [Source: config/constants.go:11] — FeedbackPrefix constant
- [Source: config/constants.go:8-9] — TaskOpen, TaskDone constants
- [Source: config/errors.go:28-37] — GateDecision type with Feedback field

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

claude-opus-4-6

### Debug Log References

- Existing Story 5.1 "retry lowercase" test updated: input `"r\n"` → `"r\n\n"` (feedback needs empty line terminator)
- Story 5.2 "retry treated as approve" table case removed: retry no longer falls through
- Gate retry integration tests: needed ReviewFn to mark task `[x]` before gate check (simulating Claude execution marking task done)

### Completion Notes List

- Task 1: Enhanced `gates.Prompt` "r" case with feedback collection loop. Same `lineCh` channel, `select` on ctx.Done(). Lines trimmed and joined with space. Updated doc comment.
- Task 2: Added `InjectFeedback` in `runner/runner.go`. Finds task by description, walks past existing indented lines, inserts feedback line with 2-space indent + `config.FeedbackPrefix`.
- Task 3: Added `RevertTask` in `runner/runner.go`. Finds task matching description AND `TaskDoneRegex`, replaces `TaskDone` with `TaskOpen`.
- Task 4: Wired retry handling in `Runner.Execute` gate check block. Retry → extract taskDesc → inject feedback (if non-empty) → revert task → `continue` outer loop (counters re-init naturally). Error pass-through (no double-wrap).
- Task 5: Added 4 gates test functions: `TestPrompt_RetryWithFeedback` (table-driven, 4 cases), `TestPrompt_RetryEmptyFeedback`, `TestPrompt_RetryFeedbackEOF`, `TestPrompt_RetryFeedbackContextCancel`.
- Task 6: Added `TestInjectFeedback_Scenarios` (5 cases), `TestRevertTask_Scenarios` (5 cases), `TestRunner_Execute_GateRetry`, `TestRunner_Execute_GateRetryEmptyFeedback`, `TestRunner_Execute_GateRetryInjectError`. Added `sequenceGatePrompt` helper for multi-decision gate mocking.
- All tests pass: `./...` green, zero regressions.

### File List

- `gates/gates.go` — modified: feedback collection flow in Prompt "r" case, updated doc comment
- `gates/gates_test.go` — modified: updated "retry lowercase" input, added 4 new test functions (Story 5.3), added `time` import, per-case `wantMinPromptCount`, `time.Sleep` in busy-wait
- `runner/runner.go` — modified: added InjectFeedback, RevertTask functions; wired retry in Execute gate check; updated Execute doc comment
- `runner/runner_test.go` — modified: removed "retry treated as approve" case, added InjectFeedback/RevertTask/GateRetry tests, added TestRunner_Execute_GateRetryRevertError, feedback content verification in integration tests, coverage gap comments
- `runner/test_helpers_test.go` — modified: moved `sequenceGatePrompt` type and `gateOpenTaskDone` const from runner_test.go for DRY consistency

## Change Log

- 2026-02-27: Story 5.3 implementation — retry with feedback collection in gates.Prompt, InjectFeedback/RevertTask in runner, full retry wiring in Runner.Execute gate check
- 2026-02-27: Code review fixes — M1: feedback content verification in GateRetry test, M2: WriteFile coverage gap documented, M3: moved sequenceGatePrompt/gateOpenTaskDone to test_helpers, M4: negative feedback assertion in empty-feedback test, M5: added GateRetryRevertError test, L1: per-case wantMinPromptCount, L2: time.Sleep in busy-wait

## Status

done
