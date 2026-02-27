# Story 5.5: Emergency Gate Upgrade

Status: done

## Story

As a developer running `ralph run --gates`,
I want emergency stops (execute attempts and review cycles exhausted) to become interactive gates with retry/feedback/skip/quit options,
so that I can decide how to proceed instead of the system automatically stopping.

## Acceptance Criteria

### AC1: Emergency gate replaces stop for execute attempts exhaustion

```gherkin
Scenario: Emergency gate for execute attempts when gates enabled
  Given gates_enabled = true
  And execute_attempts reaches max_iterations (FR23)
  When emergency triggers
  Then shows interactive emergency gate via EmergencyGatePromptFn
  And prompt text includes: task text, attempts count (e.g., "3/3"), "execute attempts exhausted"
  And does NOT return ErrMaxRetries (interactive decision instead)
```

### AC2: Emergency gate replaces stop for review cycles exhaustion

```gherkin
Scenario: Emergency gate for review cycles when gates enabled
  Given gates_enabled = true
  And review_cycles reaches max_review_iterations (FR24)
  When emergency triggers
  Then shows interactive emergency gate via EmergencyGatePromptFn
  And prompt text includes: task text, cycles count (e.g., "3/3"), "review cycles exhausted"
  And does NOT return ErrMaxReviewCycles (interactive decision instead)
```

### AC3: Non-interactive stop preserved when gates disabled

```gherkin
Scenario: Original behavior when gates disabled (execute attempts)
  Given gates_enabled = false
  And execute_attempts reaches max_iterations
  When emergency triggers
  Then returns ErrMaxRetries error (original Epic 3 behavior)
  And no interactive prompt displayed
  And exit code 1 (partial) via exit.go

Scenario: Original behavior when gates disabled (review cycles)
  Given gates_enabled = false
  And review_cycles reaches max_review_iterations
  When emergency triggers
  Then returns ErrMaxReviewCycles error (original Epic 3 behavior)
  And no interactive prompt displayed
  And exit code 1 (partial) via exit.go
```

### AC4: Emergency gate styling — different from normal gate

```gherkin
Scenario: Emergency gate uses distinct styling
  Given emergency gate triggered
  When gates.Prompt displays
  Then header uses 🚨 instead of 🚦:
    🚨 EMERGENCY GATE: <task text with context>
  And header color is bold red (color.FgRed) instead of bold cyan
  And options show: [r]etry with feedback  [s]kip  [q]uit
  And [a]pprove is NOT available (nothing to approve — task failed)
  And "a" input at emergency gate is treated as invalid
```

### AC5: Retry at emergency gate resets counters and injects feedback

```gherkin
Scenario: Retry at execute attempts emergency
  Given emergency gate for execute_attempts
  When developer chooses [r]etry with feedback "fix the build"
  Then execute_attempts resets to 0
  And feedback injected into sprint-tasks.md via InjectFeedback (Story 5.3)
  And execute retry loop continues (fresh attempt)
  And does NOT run resume extraction or backoff (user-initiated retry)

Scenario: Retry at review cycles emergency
  Given emergency gate for review_cycles
  When developer chooses [r]etry with feedback "check test coverage"
  Then review_cycles resets to 0
  And feedback injected into sprint-tasks.md via InjectFeedback (Story 5.3)
  And review cycle loop continues (fresh execute+review)
```

### AC6: Skip at emergency gate marks task done and advances

```gherkin
Scenario: Skip at execute attempts emergency
  Given emergency gate for execute_attempts
  When developer chooses [s]kip
  Then current task marked [x] in sprint-tasks.md via SkipTask function
  And runner exits both execute retry loop and review cycle loop
  And proceeds to next task in outer loop
  And counters reset for next task (natural loop re-initialization)

Scenario: Skip at review cycles emergency
  Given emergency gate for review_cycles
  When developer chooses [s]kip
  Then current task marked [x] in sprint-tasks.md via SkipTask function
  And runner exits review cycle loop
  And proceeds to next task in outer loop
```

### AC7: Quit at emergency gate returns error

```gherkin
Scenario: Quit at emergency gate
  Given emergency gate displayed (execute or review)
  When developer chooses [q]uit
  Then returns error wrapping GateDecision with "runner: emergency gate:" prefix
  And exit.go maps GateDecision{quit} to exit code 2 (user quit)
  And sprint-tasks.md state preserved (same as normal gate quit)
```

## Tasks / Subtasks

- [x] Task 1: Add Emergency field to Gate struct and update Prompt (AC: #4)
  - [x] 1.1: In `gates/gates.go`, add `Emergency bool` field to `Gate` struct — when true, use emergency styling
  - [x] 1.2: In `Prompt`, select header based on `gate.Emergency`: `"🚨 EMERGENCY GATE: "` with `color.FgRed, color.Bold` instead of `"🚦 HUMAN GATE: "` with `color.FgCyan, color.Bold`
  - [x] 1.3: In `Prompt`, select options line: emergency shows `"   [r]etry with feedback  [s]kip  [q]uit"` (no [a]pprove), normal shows all 4 options
  - [x] 1.4: In `Prompt` input parsing, "a" at emergency gate: display error `"Approve not available at emergency gate"`, loop back (invalid input)
  - [x] 1.5: Non-emergency behavior unchanged — all existing tests must pass without modification
- [x] Task 2: Add EmergencyGatePromptFn to Runner struct and wire in Run() (AC: #1, #2)
  - [x] 2.1: In `runner/runner.go`, add `EmergencyGatePromptFn GatePromptFunc` field to `Runner` struct — reuses same `GatePromptFunc` type as `GatePromptFn`
  - [x] 2.2: Update `Runner` struct doc comment to mention EmergencyGatePromptFn
  - [x] 2.3: In `Run()`, inside the `if cfg.GatesEnabled` block, wire EmergencyGatePromptFn to closure calling `gates.Prompt(ctx, gates.Gate{TaskText: taskText, Reader: os.Stdin, Writer: os.Stdout, Emergency: true})`
- [x] Task 3: Add SkipTask function (AC: #6)
  - [x] 3.1: In `runner/` package (same file as InjectFeedback/RevertTask from Story 5.3), define `func SkipTask(tasksFile, taskDesc string) error`
  - [x] 3.2: Read `tasksFile` via `os.ReadFile`
  - [x] 3.3: Find line containing `taskDesc` AND matching `config.TaskOpenRegex` (must be `[ ]` line)
  - [x] 3.4: Replace `config.TaskOpen` ("- [ ]") with `config.TaskDone` ("- [x]") on that line via `strings.Replace(line, config.TaskOpen, config.TaskDone, 1)`
  - [x] 3.5: Write back to file. Error wrapping: `fmt.Errorf("runner: skip task: %w", err)`
  - [x] 3.6: Task not found: `fmt.Errorf("runner: skip task: task not found: %s", taskDesc)` — distinct error
- [x] Task 4: Wire emergency gate at execute attempts exhaustion (AC: #1, #3, #5, #6, #7)
  - [x] 4.1: In `Runner.Execute`, at `if executeAttempts >= r.Cfg.MaxIterations` (line ~365), wrap existing return with conditional: `if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil { ... } else { return existing error }`
  - [x] 4.2: Build emergency text: `fmt.Sprintf("execute attempts exhausted (%d/%d) for %q", executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text)`
  - [x] 4.3: Call `r.EmergencyGatePromptFn(ctx, emergencyText)`. Error: `return fmt.Errorf("runner: emergency gate: %w", gateErr)`
  - [x] 4.4: Quit: `return fmt.Errorf("runner: emergency gate: %w", decision)` — GateDecision wraps to exit code 2
  - [x] 4.5: Skip: call `SkipTask(r.TasksFile, taskDesc)` — pass through SkipTask error directly (already wraps with "runner: skip task:" prefix, avoid double "runner:" wrap per ScanTasks pattern). Set `skipTask = true` flag, `break` execute retry loop
  - [x] 4.6: Retry: set `executeAttempts = 0`, call `InjectFeedback` if Feedback non-empty — pass through InjectFeedback error directly (already wraps with "runner: inject feedback:" prefix, avoid double "runner:" wrap per ScanTasks pattern). `continue` execute retry loop (skip resume extraction/backoff — user-initiated retry)
  - [x] 4.7: Declare `skipTask := false` before execute retry loop. After retry loop: `if skipTask { break }` to also exit review cycle loop
- [x] Task 5: Wire emergency gate at review cycles exhaustion (AC: #2, #3, #5, #6, #7)
  - [x] 5.1: In `Runner.Execute`, at `if reviewCycles >= r.Cfg.MaxReviewIterations` (line ~398), wrap existing return with conditional: `if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil { ... } else { return existing error }`
  - [x] 5.2: Build emergency text: `fmt.Sprintf("review cycles exhausted (%d/%d) for %q", reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text)`
  - [x] 5.3: Call `r.EmergencyGatePromptFn(ctx, emergencyText)`. Error: `return fmt.Errorf("runner: emergency gate: %w", gateErr)`
  - [x] 5.4: Quit: `return fmt.Errorf("runner: emergency gate: %w", decision)`
  - [x] 5.5: Skip: call `SkipTask(r.TasksFile, taskDesc)` — pass through SkipTask error directly (already wraps with "runner: skip task:" prefix). `break` review cycle loop (one level — no flag needed)
  - [x] 5.6: Retry: set `reviewCycles = 0`, call `InjectFeedback` if Feedback non-empty — pass through InjectFeedback error directly (already wraps with "runner: inject feedback:" prefix). `continue` review cycle loop
- [x] Task 6: Write gates tests (AC: #4)
  - [x] 6.1: `TestPrompt_EmergencyStyle` — Gate{Emergency: true}, input "a\ns\n". Verify: output contains "🚨 EMERGENCY GATE:", output contains "[s]kip", output does NOT contain "[a]pprove" in options line, output contains "Approve not available", decision.Action == ActionSkip
  - [x] 6.2: `TestPrompt_EmergencyValidActions` — table-driven: "s"→ActionSkip, "r"→ActionRetry, "q"→ActionQuit. Verify: all work at emergency gate, same as normal
  - [x] 6.3: `TestPrompt_EmergencyNoApprove` — input "a\nq\n". Verify: "a" rejected with error message, then "q" accepted. Verify error message in output contains "not available"
  - [x] 6.4: `TestPrompt_NormalUnchanged` — Gate{Emergency: false}, input "a\n". Verify: output contains "🚦 HUMAN GATE:", [a]pprove present, decision.Action == ActionApprove
- [x] Task 7: Write runner tests (AC: #1-#3, #5-#7)
  - [x] 7.1: `TestSkipTask_Scenarios` — table-driven: basic skip `[ ]→[x]` (verify text preserved, other tasks unchanged), task not found (no [ ] line matching), task already [x] (not found), os.ReadFile error (non-existent path → wraps with "runner: skip task:" prefix)
  - [x] 7.2: `TestRunner_Execute_EmergencyGateExecuteRetry` — GatesEnabled=true, EmergencyGatePromptFn returns retry+feedback, MockClaude always fails (needsRetry=true). Verify: EmergencyGatePromptFn called after MaxIterations attempts, executeAttempts reset, feedback injected, runner retries
  - [x] 7.3: `TestRunner_Execute_EmergencyGateExecuteSkip` — EmergencyGatePromptFn returns skip. Verify: task marked [x] via SkipTask, runner proceeds to next task, does NOT return ErrMaxRetries
  - [x] 7.4: `TestRunner_Execute_EmergencyGateExecuteQuit` — EmergencyGatePromptFn returns quit. Verify: error wraps GateDecision, `errors.As` extracts Action=="quit", "runner: emergency gate:" prefix
  - [x] 7.5: `TestRunner_Execute_EmergencyGateReviewRetry` — GatesEnabled=true, ReviewFn returns non-clean review MaxReviewIterations times, then EmergencyGatePromptFn returns retry. Verify: reviewCycles reset, feedback injected, runner retries review cycle
  - [x] 7.6: `TestRunner_Execute_EmergencyGateReviewSkip` — ReviewFn returns non-clean, EmergencyGatePromptFn returns skip. Verify: task marked [x], runner proceeds
  - [x] 7.7: `TestRunner_Execute_EmergencyGateDisabled` — GatesEnabled=false, executeAttempts exhausted. Verify: EmergencyGatePromptFn NOT called, returns ErrMaxRetries (original behavior)
  - [x] 7.8: Use existing test infrastructure: MockClaude (always fail scenario for retries), MockGitClient (same SHA for no-commit), trackingResumeExtract, trackingSleep, writeTasksFile

## Dev Notes

### Architecture Constraints

- **Dependency direction preserved:** `runner → gates` — same as normal gate flow
- **GatePromptFunc reused:** EmergencyGatePromptFn has same type as GatePromptFn — no new function type needed
- **Gates package only adds Emergency flag:** Prompt function checks Emergency bool for styling/options. No new exported functions
- **No os.Exit in runner:** emergency quit propagates as error, `cmd/ralph/exit.go` maps GateDecision{quit} → exit code 2 (same as normal gate quit)
- **Mutation Asymmetry exception:** SkipTask marks [ ] → [x] — second exception alongside RevertTask's [x] → [ ] from Story 5.3. Both documented

### What Already Exists (DO NOT RECREATE)

| Component | Location | Status |
|-----------|----------|--------|
| `GateDecision` type | `config/errors.go:28-37` | Action + Feedback, implements error |
| `config.ErrMaxRetries` | `config/errors.go:11` | Sentinel for execute exhaustion |
| `config.ErrMaxReviewCycles` | `config/errors.go:12` | Sentinel for review exhaustion |
| Execute exhaustion return | `runner/runner.go:365-368` | Current: return ErrMaxRetries |
| Review exhaustion return | `runner/runner.go:398-401` | Current: return ErrMaxReviewCycles |
| `GatePromptFunc` type | `runner/runner.go:54-58` | Reuse for EmergencyGatePromptFn |
| `GatePromptFn` field | `runner/runner.go:236` | On Runner struct — add EmergencyGatePromptFn alongside |
| Gate check block | `runner/runner.go:404-417` | After clean review (Story 5.2) |
| `InjectFeedback` | Story 5.3 adds | Reuse for emergency retry feedback |
| `RevertTask` | Story 5.3 adds | Inverse of SkipTask |
| `taskDescription()` | `runner/runner.go:169-175` | Reuse for task matching |
| `config.TaskOpen/TaskDone` | `config/constants.go:8-9` | For SkipTask marker replacement |
| `config.TaskOpenRegex` | `config/constants.go:27` | For SkipTask line matching |
| Exit code mapping | `cmd/ralph/exit.go:32-37` | GateDecision{quit} → code 2 (works for emergency too) |
| `gates.Gate` struct | `gates/gates.go:17-21` | Add Emergency bool field |
| `gates.Prompt` function | `gates/gates.go:33-89` | Add emergency styling conditionals |
| `fatih/color` | `gates/gates.go:11` | Already imported for styling |

### Story 5.2-5.4 Create (Prerequisites — Assumed Complete)

| Component | Location | Expected |
|-----------|----------|----------|
| `GatePromptFunc` type | `runner/runner.go` | Injectable function type |
| `GatePromptFn` field | `Runner` struct | Normal gate prompt function |
| Gate check block | `Runner.Execute` | After clean review, with checkpoint logic (5.4) |
| `InjectFeedback` | `runner/` | Feedback injection into sprint-tasks.md |
| `RevertTask` | `runner/` | [x] → [ ] revert on retry |
| `completedTasks` counter | `Runner.Execute` | For checkpoint gates (5.4) |

### Critical: Execute Attempts Emergency Gate Placement

Current code (line ~363-368):
```go
if needsRetry {
    executeAttempts++
    if executeAttempts >= r.Cfg.MaxIterations {
        return fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q ...: %w",
            executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text, config.ErrMaxRetries)
    }
    // Resume extraction, dirty recovery, backoff, continue
}
```

After Story 5.5:
```go
if needsRetry {
    executeAttempts++
    if executeAttempts >= r.Cfg.MaxIterations {
        if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
            emergencyText := fmt.Sprintf("execute attempts exhausted (%d/%d) for %q",
                executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text)
            decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
            if gateErr != nil {
                return fmt.Errorf("runner: emergency gate: %w", gateErr)
            }
            if decision.Action == config.ActionQuit {
                return fmt.Errorf("runner: emergency gate: %w", decision)
            }
            if decision.Action == config.ActionSkip {
                taskDesc := taskDescription(result.OpenTasks[0].Text)
                if err := SkipTask(r.TasksFile, taskDesc); err != nil {
                    return err // SkipTask wraps with "runner: skip task:" prefix
                }
                skipTask = true
                break // exit execute retry loop
            }
            if decision.Action == config.ActionRetry {
                executeAttempts = 0
                if decision.Feedback != "" {
                    taskDesc := taskDescription(result.OpenTasks[0].Text)
                    if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
                        return err // InjectFeedback wraps with "runner: inject feedback:" prefix
                    }
                }
                continue // restart execute retry loop — skip resume/backoff
            }
        } else {
            return fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q (check logs for details): %w",
                executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text, config.ErrMaxRetries)
        }
    }
    // Normal retry: resume extraction, dirty recovery, backoff
    ...
}
```

**Key: `skipTask` flag** — declared `skipTask := false` before execute retry loop. After the retry loop: `if skipTask { break }` exits review cycle loop too.

### Critical: Review Cycles Emergency Gate Placement

Current code (line ~397-401):
```go
reviewCycles++
if reviewCycles >= r.Cfg.MaxReviewIterations {
    return fmt.Errorf("runner: review cycles exhausted (%d/%d) for %q ...: %w",
        reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text, config.ErrMaxReviewCycles)
}
```

After Story 5.5:
```go
reviewCycles++
if reviewCycles >= r.Cfg.MaxReviewIterations {
    if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
        emergencyText := fmt.Sprintf("review cycles exhausted (%d/%d) for %q",
            reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text)
        decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
        if gateErr != nil {
            return fmt.Errorf("runner: emergency gate: %w", gateErr)
        }
        if decision.Action == config.ActionQuit {
            return fmt.Errorf("runner: emergency gate: %w", decision)
        }
        if decision.Action == config.ActionSkip {
            taskDesc := taskDescription(result.OpenTasks[0].Text)
            if err := SkipTask(r.TasksFile, taskDesc); err != nil {
                return err // SkipTask wraps with "runner: skip task:" prefix
            }
            break // exit review cycle loop (one level — no flag needed)
        }
        if decision.Action == config.ActionRetry {
            reviewCycles = 0
            if decision.Feedback != "" {
                taskDesc := taskDescription(result.OpenTasks[0].Text)
                if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
                    return err // InjectFeedback wraps with "runner: inject feedback:" prefix
                }
            }
            continue // restart review cycle loop
        }
    } else {
        return fmt.Errorf("runner: review cycles exhausted (%d/%d) for %q (check logs for details): %w",
            reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text, config.ErrMaxReviewCycles)
    }
}
```

### SkipTask vs RevertTask — Symmetry

| Function | Direction | Use Case | Regex | Constants |
|----------|-----------|----------|-------|-----------|
| `RevertTask` (5.3) | [x] → [ ] | Retry at normal gate | `TaskDoneRegex` | `TaskDone → TaskOpen` |
| `SkipTask` (5.5) | [ ] → [x] | Skip at emergency gate | `TaskOpenRegex` | `TaskOpen → TaskDone` |

Both follow the same pattern: read file, find matching line, replace marker, write back. Identical structure, opposite direction.

### Emergency Retry: No Resume/Backoff

When user chooses retry at emergency gate, the `continue` restarts the inner loop immediately:
- **Execute retry:** `continue` restarts execute retry loop — new HeadCommit, new session.Execute. Skips resume extraction, dirty recovery, and exponential backoff. Rationale: user-initiated retry is intentional, no need for automated delay
- **Review retry:** `continue` restarts review cycle loop — new findings read, new prompt assembly, new execute+review. Fresh cycle

Normal automated retries still use resume extraction + backoff (code below the emergency check).

### Emergency Gate: No `completedTasks` Counter Interaction

Emergency gates fire at exhaustion points, BEFORE task completion. They don't interact with the `completedTasks` checkpoint counter (Story 5.4):
- Skip: SkipTask marks [x], outer loop re-reads and finds next [ ] task. Counter increments as usual on next task's clean review
- Retry: inner loop restarts, eventually reaches clean review, normal counter increment
- Quit: runner exits, no counter change

### Testing Approach

**gates tests (gates/gates_test.go):**
- **Emergency bool** flag on Gate: verify output contains 🚨 header, no [a]pprove option
- **"a" rejection** at emergency: verify error message, then valid input accepted
- **Non-emergency unchanged:** verify existing tests still pass, 🚦 header preserved

**runner tests (runner/runner_test.go):**
- **SkipTask:** table-driven, t.TempDir(), file content verification
- **Execute attempts emergency:** MockClaude always returns same SHA (no commit → needsRetry). After MaxIterations, EmergencyGatePromptFn fires. Track call counts
- **Review cycles emergency:** ReviewFn always returns non-clean. After MaxReviewIterations, emergency fires
- **Gates disabled:** verify original ErrMaxRetries/ErrMaxReviewCycles behavior unchanged
- **Mock EmergencyGatePromptFn:** simple closure returning fixed GateDecision, track call count + passed text

### Naming Conventions

- Field: `EmergencyGatePromptFn` — follows `GatePromptFn` pattern with "Emergency" prefix
- Function: `SkipTask` — imperative, matches `RevertTask` (Story 5.3) and `InjectFeedback`
- Error prefix: `"runner: emergency gate:"` — distinct from `"runner: gate:"` (normal) for error chain disambiguation
- Test names: `TestPrompt_Emergency<Scenario>`, `TestRunner_Execute_EmergencyGate<Location><Decision>`, `TestSkipTask_Scenarios`

### What This Story Does NOT Include

- GateType enum — KISS: `Emergency bool` is sufficient (only two types: normal and emergency)
- Emergency gate at other exhaustion points — only execute attempts and review cycles
- Custom emergency-specific timeout or retry limits — reuses existing MaxIterations/MaxReviewIterations
- Integration tests (Story 5.6 covers end-to-end emergency gate scenarios)

### Project Structure Notes

- **Modified file:** `gates/gates.go` — add Emergency field to Gate, update Prompt styling/options
- **Modified file:** `gates/gates_test.go` — new emergency-related test cases (3-4 functions)
- **Modified file:** `runner/runner.go` — add EmergencyGatePromptFn field, SkipTask function, emergency conditionals at both exhaustion points, wire in Run()
- **Modified file:** `runner/runner_test.go` — new emergency gate test cases (6-7 functions)
- **No new files** — all changes in existing packages

### References

- [Source: docs/epics/epic-5-human-gates-control-stories.md#Story 5.5]
- [Source: runner/runner.go:363-368] — Execute attempts exhaustion (modify)
- [Source: runner/runner.go:397-401] — Review cycles exhaustion (modify)
- [Source: runner/runner.go:54-58] — GatePromptFunc type (reuse for EmergencyGatePromptFn)
- [Source: runner/runner.go:231-240] — Runner struct (add EmergencyGatePromptFn field)
- [Source: runner/runner.go:502-523] — Run() function (wire EmergencyGatePromptFn)
- [Source: gates/gates.go:17-21] — Gate struct (add Emergency field)
- [Source: gates/gates.go:52-56] — Prompt header + options display (add conditional)
- [Source: config/errors.go:11-12] — ErrMaxRetries, ErrMaxReviewCycles sentinels
- [Source: cmd/ralph/exit.go:32-37] — GateDecision quit → exit code 2

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

N/A

### Completion Notes List

- All 7 tasks implemented and tested
- 4 new gates tests (emergency styling, valid actions, no-approve, normal unchanged)
- 7 new runner tests (SkipTask scenarios, execute retry/skip/quit, review retry/skip, disabled)
- Emergency gate at execute attempts exhaustion: retry resets executeAttempts=0, skip marks [x] via SkipTask + breaks both loops, quit wraps GateDecision
- Emergency gate at review cycles exhaustion: same pattern, skip breaks one loop (no flag needed)
- SkipTask function symmetric to RevertTask ([ ] → [x] vs [x] → [ ])
- EmergencyGatePromptFn reuses GatePromptFunc type
- Emergency retry skips resume extraction and backoff (user-initiated)
- Intermediate state capture in tests: ReviewFn callback captures feedback before overwrite
- Edit tool had trouble matching tab-indented Go — used Python workaround for complex replacements

### File List

- `gates/gates.go` — Added Emergency field to Gate struct, conditional Prompt styling/options
- `gates/gates_test.go` — 4 new test functions for emergency gate behavior
- `runner/runner.go` — Added EmergencyGatePromptFn field, SkipTask function, emergency gate wiring at both exhaustion points, Run() wiring
- `runner/runner_test.go` — 7 new test functions for emergency gate and SkipTask
- `runner/test_helpers_test.go` — No new helpers added; existing helpers used for emergency gate tests
- `docs/sprint-artifacts/sprint-status.yaml` — Status update to review
- `docs/sprint-artifacts/5-5-emergency-gate-upgrade.md` — Tasks marked complete, Dev Agent Record

### Change Log

- `gates/gates.go`: Added `Emergency bool` field to Gate struct; Prompt now conditionally renders 🚨 EMERGENCY GATE header (red, bold) vs 🚦 HUMAN GATE, omits [a]pprove for emergency, rejects "a" input at emergency gate
- `gates/gates_test.go`: Added TestPrompt_EmergencyStyle, TestPrompt_EmergencyValidActions, TestPrompt_EmergencyNoApprove, TestPrompt_NormalUnchanged
- `runner/runner.go`: Added `EmergencyGatePromptFn GatePromptFunc` to Runner struct; Added `SkipTask(tasksFile, taskDesc)` function; Wrapped execute attempts exhaustion and review cycles exhaustion with emergency gate conditionals; Updated Execute doc comment; Wired EmergencyGatePromptFn in Run()
- `runner/runner_test.go`: Added TestSkipTask_Scenarios (5 cases), TestRunner_Execute_EmergencyGateExecuteRetry, ExecuteSkip, ExecuteQuit, ReviewRetry, ReviewSkip, Disabled
