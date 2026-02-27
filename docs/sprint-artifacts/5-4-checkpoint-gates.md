# Story 5.4: Checkpoint Gates

Status: done

## Story

As a developer running `ralph run --gates --every N`,
I want periodic checkpoint gates every N completed tasks,
so that I can regularly review AI progress even without `[GATE]` markup on tasks.

## Acceptance Criteria

### AC1: Checkpoint gate fires every N completed tasks

```gherkin
Scenario: Checkpoint gate fires every N tasks
  Given ralph run --gates --every 3
  And 5 tasks in sprint-tasks.md (none with [GATE] tag)
  When tasks complete in sequence
  Then checkpoint gate fires after task 3
  And checkpoint gate fires after task 6 (if reached)
  And no checkpoint after tasks 1, 2, 4, 5
  And prompt indicates checkpoint: task text + "(checkpoint every 3)" (FR25)
```

### AC2: Counter increments on each task completion

```gherkin
Scenario: Counter increments on clean review
  Given --every 2 configured
  When task completes clean review (marked [x])
  Then completedTasks counter increments by 1
  And counter is cumulative across all tasks (never resets mid-run)
  And counter declared before outer for-loop in Execute
```

### AC3: Skipped tasks count toward checkpoint

```gherkin
Scenario: Skipped tasks count toward checkpoint
  Given --every 3 configured
  And task 2 has [GATE] tag, user chooses skip
  When counting for checkpoint
  Then skipped task still counts (counter was incremented before gate check)
  And checkpoint fires after task 3 (not deferred)
```

### AC4: Checkpoint independent of [GATE] tags

```gherkin
Scenario: Checkpoint fires on non-GATE tasks
  Given --every 2 configured
  And no tasks have [GATE] tag
  When task 2 completes clean review
  Then checkpoint gate fires (even without [GATE])
  And checkpoint counter continues independently of [GATE] presence
```

### AC5: Combined GATE + checkpoint = single prompt

```gherkin
Scenario: Combined GATE and checkpoint in single prompt
  Given --every 3 configured
  And task 3 has [GATE] tag
  When task 3 completes clean review
  Then ONE gate prompt fires (not two separate prompts)
  And prompt text includes both indicators:
    task text already contains [GATE], plus "(checkpoint every 3)" appended
  And same decision options apply (approve/skip/quit/retry)
```

### AC6: --every 0 disables checkpoint gates

```gherkin
Scenario: Checkpoint disabled with --every 0
  Given --gates --every 0 (default)
  And GatesEnabled = true
  When tasks complete
  Then no checkpoint gates fire
  And only [GATE]-tagged tasks trigger gate prompt
  And completedTasks counter still maintained (no-op check)
```

### AC7: Checkpoint requires --gates flag

```gherkin
Scenario: Checkpoint inactive without --gates
  Given --every 3 configured but --gates NOT set (GatesEnabled = false)
  When task 3 completes
  Then no checkpoint gate fires
  And no [GATE] gate fires either
  And GatePromptFn is NOT called
```

### AC8: Counter unaffected by retry

```gherkin
Scenario: Counter adjusted on retry
  Given --every 3 configured
  And task 2 has [GATE], user chooses retry
  When retry reverts task to [ ] (Story 5.3)
  Then completedTasks counter decremented by 1 (undo: task not truly completed)
  And on re-completion of same task, counter increments again
  And checkpoint still fires at correct cumulative count
```

## Tasks / Subtasks

- [x] Task 1: Add completedTasks counter to Runner.Execute (AC: #1-#4, #6-#8)
  - [x] 1.1: In `runner/runner.go` Execute method, declare `completedTasks := 0` before outer for-loop (line ~263), persists across all task iterations
  - [x] 1.2: Increment `completedTasks++` after review cycle loop exits on clean review (after the `break` from `if rr.Clean`) — BEFORE gate check block
  - [x] 1.3: Placement: between review cycle loop exit and gate check. Current structure (after Story 5.2): review cycle loop → break on clean → gate check → outer loop continues. New: review cycle loop → break on clean → `completedTasks++` → gate check
- [x] Task 2: Expand gate trigger condition for checkpoint (AC: #1, #4, #5, #6, #7)
  - [x] 2.1: Compute `isGateTask := result.OpenTasks[0].HasGate`
  - [x] 2.2: Compute `isCheckpoint := r.Cfg.GatesCheckpoint > 0 && completedTasks%r.Cfg.GatesCheckpoint == 0`
  - [x] 2.3: Replace Story 5.2's single condition `r.Cfg.GatesEnabled && result.OpenTasks[0].HasGate && r.GatePromptFn != nil` with combined: `r.Cfg.GatesEnabled && (isGateTask || isCheckpoint) && r.GatePromptFn != nil`
  - [x] 2.4: This naturally handles: GATE-only (isGateTask=true, isCheckpoint=false), checkpoint-only (isGateTask=false, isCheckpoint=true), combined (both true), neither (falls through)
- [x] Task 3: Build enriched gate text with checkpoint context (AC: #1, #5)
  - [x] 3.1: Initialize `gateText := result.OpenTasks[0].Text` (same as Story 5.2)
  - [x] 3.2: If `isCheckpoint`, append: `gateText += fmt.Sprintf(" (checkpoint every %d)", r.Cfg.GatesCheckpoint)`
  - [x] 3.3: Pass `gateText` to `r.GatePromptFn(ctx, gateText)` instead of raw task text
  - [x] 3.4: For combined GATE + checkpoint: task text already contains `[GATE]`, plus `(checkpoint every N)` appended — naturally shows both
- [x] Task 4: Adjust counter on retry (AC: #8)
  - [x] 4.1: In retry handling block (added by Story 5.3), add `completedTasks--` BEFORE the `continue` statement
  - [x] 4.2: This undoes the increment from step 1.2 — task not truly completed on retry
  - [x] 4.3: On re-completion (next outer loop iteration clean review), counter re-increments naturally
- [x] Task 5: Write unit tests (AC: #1-#8)
  - [x] 5.1: `TestRunner_Execute_CheckpointFires` — GatesEnabled=true, GatesCheckpoint=2, 4 tasks (no [GATE]). Mock GatePromptFn tracks calls. Verify: GatePromptFn called after task 2 and task 4 (2 calls total), not after task 1 or 3. Verify gateText contains "(checkpoint every 2)"
  - [x] 5.2: `TestRunner_Execute_CheckpointDisabled` — GatesCheckpoint=0, GatesEnabled=true, 3 tasks (no [GATE]). Verify: GatePromptFn NOT called (0 calls)
  - [x] 5.3: `TestRunner_Execute_CheckpointWithoutGatesFlag` — GatesCheckpoint=2, GatesEnabled=false. Verify: GatePromptFn NOT called
  - [x] 5.4: `TestRunner_Execute_CheckpointCombinedWithGate` — GatesCheckpoint=2, task 2 has [GATE]. Verify: GatePromptFn called exactly once for task 2 (not twice). Verify gateText contains both "[GATE]" and "(checkpoint every 2)"
  - [x] 5.5: `TestRunner_Execute_CheckpointGateOnly` — GatesCheckpoint=5 (high), task 1 has [GATE]. Verify: GatePromptFn called after task 1 (GATE trigger), checkpoint NOT reached yet
  - [x] 5.6: `TestRunner_Execute_CheckpointRetryAdjusts` — GatesCheckpoint=2, task 1 has [GATE], user retries then approves on second attempt. Verify: counter adjusted — checkpoint fires at correct task. Use mock sequence: first gate call returns retry, second gate call returns approve
  - [x] 5.7: Use existing test infrastructure: MockClaude + MockGitClient, writeTasksFile helper, cleanReviewFn (or reviewAndMarkDoneFn for multi-task), noopSleepFn, noopResumeExtractFn
  - [x] 5.8: Test fixture for multi-task scenarios: `"- [ ] Task one\n- [ ] Task two\n- [ ] Task three\n- [ ] Task four\n"` — tasks without [GATE] for checkpoint-only tests. Separate fixture with [GATE] for combined tests

## Dev Notes

### Architecture Constraints

- **No changes to gates package** — checkpoint logic is entirely in runner. The gate prompt function receives enriched text, no interface change needed
- **GatePromptFunc signature unchanged** — `func(ctx, taskText) (*GateDecision, error)` — checkpoint context passed via taskText string
- **Config infrastructure already exists** — `config.GatesCheckpoint`, `CLIFlags.GatesCheckpoint`, `--every` flag, `applyCLIFlags` all wired in Epics 1-2

### What Already Exists (DO NOT RECREATE)

| Component | Location | Status |
|-----------|----------|--------|
| `config.GatesCheckpoint` | `config/config.go:24` | `int` field, YAML + CLI cascade |
| `CLIFlags.GatesCheckpoint` | `config/config.go:45` | Pointer field for three-level cascade |
| `--every` CLI flag | `cmd/ralph/run.go:28` | `Int` flag, default 0 |
| `buildCLIFlags` wiring | `cmd/ralph/run.go:61-64` | Maps --every → CLIFlags.GatesCheckpoint |
| `applyCLIFlags` | `config/config.go:77-79` | Applies GatesCheckpoint override |
| `defaults.yaml` | `config/defaults.yaml:6` | `gates_checkpoint: 0` (disabled by default) |
| `config.GatesEnabled` | `config/config.go:23` | Bool field, gates master switch |
| Gate check block | Story 5.2 adds | In Execute, after clean review |
| Retry handling | Story 5.3 adds | InjectFeedback + RevertTask + continue |
| `GatePromptFunc` type | Story 5.2 adds | Injectable gate prompt function |
| `GatePromptFn` field | Story 5.2 adds | On Runner struct |

### Story 5.2 and 5.3 Create (Prerequisites — Assumed Complete)

| Component | Location | Expected |
|-----------|----------|----------|
| `GatePromptFunc` type | `runner/runner.go` | `type GatePromptFunc func(ctx context.Context, taskText string) (*config.GateDecision, error)` |
| `GatePromptFn` field | `Runner` struct | Injectable gate prompt function |
| Gate check block | `Runner.Execute` | After clean review, checks HasGate + GatesEnabled |
| Retry handling | `Runner.Execute` gate check | InjectFeedback + RevertTask + `continue` outer loop |

### Critical: Counter Placement in Runner.Execute

After Story 5.2/5.3, the Execute method has this structure after the review cycle loop:

```go
// review cycle loop exits here (rr.Clean == true)

// Story 5.4: increment completion counter
completedTasks++

// Gate check (Story 5.2, expanded by 5.4)
isGateTask := result.OpenTasks[0].HasGate
isCheckpoint := r.Cfg.GatesCheckpoint > 0 && completedTasks%r.Cfg.GatesCheckpoint == 0

if r.Cfg.GatesEnabled && (isGateTask || isCheckpoint) && r.GatePromptFn != nil {
    gateText := result.OpenTasks[0].Text
    if isCheckpoint {
        gateText += fmt.Sprintf(" (checkpoint every %d)", r.Cfg.GatesCheckpoint)
    }

    decision, gateErr := r.GatePromptFn(ctx, gateText)
    if gateErr != nil {
        return fmt.Errorf("runner: gate: %w", gateErr)
    }
    if decision.Action == config.ActionQuit {
        return fmt.Errorf("runner: gate: %w", decision)
    }
    if decision.Action == config.ActionRetry {
        completedTasks-- // undo: task not truly completed
        // Story 5.3: inject feedback, revert task
        taskDesc := taskDescription(result.OpenTasks[0].Text)
        if decision.Feedback != "" {
            if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
                return err // InjectFeedback wraps with "runner: inject feedback:" prefix
            }
        }
        if err := RevertTask(r.TasksFile, taskDesc); err != nil {
            return err // RevertTask wraps with "runner: revert task:" prefix
        }
        continue // outer for-loop
    }
    // approve, skip → fall through to next task
}
// Outer loop continues: re-read tasks, scan, process next open task
```

Key design decisions:
- **Counter before gate check:** `completedTasks++` happens BEFORE the checkpoint condition is evaluated. This ensures the modulo check uses the current completion count
- **Decrement on retry:** `completedTasks--` undoes the increment when a task is retried. The task will re-increment on its next completion
- **Counter scope:** declared before outer `for` loop, persistent across all iterations
- **No counter reset:** cumulative across entire run (per epic: "reset never")

### Checkpoint Trigger Formula

```go
isCheckpoint := r.Cfg.GatesCheckpoint > 0 && completedTasks%r.Cfg.GatesCheckpoint == 0
```

- `GatesCheckpoint == 0`: disables checkpoints (modulo by zero would panic — guarded by `> 0` check)
- `GatesCheckpoint == 1`: gate after every task (every completion % 1 == 0)
- `GatesCheckpoint == 3`: gate after tasks 3, 6, 9, ...
- Combined with `isGateTask`: OR logic — either trigger activates gate

### Gate Text Enrichment

| Trigger | Gate Text |
|---------|-----------|
| [GATE] only | `"- [ ] Setup project [GATE]"` (raw task text) |
| Checkpoint only | `"- [ ] Write tests (checkpoint every 3)"` |
| Combined | `"- [ ] Setup project [GATE] (checkpoint every 3)"` |

The gates.Prompt function displays the text as-is in the header. No changes to gates package needed.

### Testing Approach

- **Multi-task test fixtures** — need 3-5 tasks to verify checkpoint counting:
  ```go
  const fourOpenTasks = "- [ ] Task one\n- [ ] Task two\n- [ ] Task three\n- [ ] Task four\n"
  const fourOpenTasksWithGate = "- [ ] Task one\n- [ ] Task two [GATE]\n- [ ] Task three\n- [ ] Task four\n"
  ```
- **Mock GatePromptFn with tracking** — counter + received text args:
  ```go
  type trackingGatePrompt struct {
      count    int
      texts    []string
      decision *config.GateDecision
  }
  ```
- **ReviewFn that marks tasks done** — need ReviewFn that writes progressive task completion to sprint-tasks.md. Use `reviewAndMarkDoneFn` pattern or custom closure that marks current task [x]
- **Multi-step MockClaude** — each task execution needs separate scenario step. Use `testutil.Scenario` with multiple invocations
- **Table-driven where possible** — checkpoint vs no-checkpoint vs combined cases
- **Verify call count AND text content** — assert both GatePromptFn call count and the texts passed to it

### Naming Conventions

- Variable: `completedTasks` — descriptive, matches epic language ("counts completed tasks")
- Variables: `isGateTask`, `isCheckpoint` — boolean predicates for gate trigger
- Error prefix: unchanged — `"runner: gate:"` for all gate-related errors
- Test names: `TestRunner_Execute_Checkpoint<Scenario>`
- Test fixture: `fourOpenTasks` (follows `threeOpenTasks` pattern in test_helpers_test.go)

### What This Story Does NOT Include

- Gates package changes (no changes to gates.Prompt or Gate struct)
- New config fields (GatesCheckpoint already exists)
- New CLI flags (--every already wired)
- Emergency gate upgrade (Story 5.5)
- Integration tests (Story 5.6)
- Checkpoint counter persistence across runs (counter lives in Execute, resets each `ralph run`)

### Project Structure Notes

- **Modified file:** `runner/runner.go` — add completedTasks counter, expand gate condition, enrich gate text, adjust counter on retry
- **Modified file:** `runner/runner_test.go` — new checkpoint test cases (6 test functions)
- **Potentially modified:** `runner/test_helpers_test.go` — new multi-task fixture constants, tracking gate prompt struct
- **No new files** — all changes in existing runner package
- **No gates package changes** — checkpoint is entirely runner-side logic

### References

- [Source: docs/epics/epic-5-human-gates-control-stories.md#Story 5.4]
- [Source: config/config.go:24] — GatesCheckpoint field
- [Source: config/config.go:45] — CLIFlags.GatesCheckpoint
- [Source: cmd/ralph/run.go:28] — --every CLI flag
- [Source: cmd/ralph/run.go:61-64] — buildCLIFlags wiring for --every
- [Source: config/defaults.yaml:6] — gates_checkpoint: 0 default
- [Source: runner/runner.go:263-396] — Execute method (insertion points)
- [Source: runner/test_helpers_test.go:17-24] — threeOpenTasks fixture pattern

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

claude-opus-4-6

### Debug Log References

None

### Completion Notes List

- Task 1-4: Added `completedTasks` counter before outer for-loop, `completedTasks++` after clean review break, expanded gate condition with `isGateTask || isCheckpoint` OR logic, enriched gate text with `(checkpoint every N)` suffix, `completedTasks--` on retry
- Task 5: 6 tests covering all 8 ACs — checkpoint fires (AC1,2,4), disabled (AC6), no gates flag (AC7), combined with [GATE] (AC5), gate-only (AC1,4), retry adjustment (AC8)
- Added `progressiveReviewFn` helper and `fourOpenTasks`/`fourOpenTasksWithGate` fixtures for multi-task scenarios
- All existing gate tests (Story 5.2, 5.3) pass with zero regressions
- Full `./...` test suite passes

### File List

- `runner/runner.go` — completedTasks counter, expanded gate trigger condition, enriched gate text, retry counter adjustment, updated Execute doc comment
- `runner/runner_test.go` — 6 new checkpoint test functions (CheckpointFires, CheckpointDisabled, CheckpointWithoutGatesFlag, CheckpointCombinedWithGate, CheckpointGateOnly, CheckpointRetryAdjusts)
- `runner/test_helpers_test.go` — `fourOpenTasks`, `fourOpenTasksWithGate` fixtures, `progressiveReviewFn` helper
- `docs/sprint-artifacts/5-4-checkpoint-gates.md` — tasks marked [x], dev record, status → review
- `docs/sprint-artifacts/sprint-status.yaml` — 5-4 status → in-progress (will be review after completion)
