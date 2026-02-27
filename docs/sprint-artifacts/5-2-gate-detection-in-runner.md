# Story 5.2: Gate Detection in Runner

Status: Done

## Story

As a user running `ralph run --gates`,
I want the system to stop at tasks with `[GATE]` tag for my approval before continuing,
so that I can review completed work at gate points and decide whether to proceed or stop.

## Acceptance Criteria

### AC1: Gates enabled via --gates flag

```gherkin
Scenario: Gates enabled via --gates flag
  Given ralph run invoked with --gates flag (FR20)
  When runner starts
  Then gates_enabled = true in config (already wired: config.GatesEnabled)
  And runner checks for gate tags during execution
```

### AC2: Gates disabled by default

```gherkin
Scenario: Gates disabled by default
  Given ralph run invoked without --gates flag
  When runner encounters [GATE] tagged task
  Then skips gate prompt entirely
  And executes task normally without stopping
  And GatePromptFn is NOT called
```

### AC3: Stop at GATE-tagged task

```gherkin
Scenario: Stop at GATE-tagged task
  Given gates_enabled = true
  And current task has [GATE] tag (detected by scanner — HasGate field on TaskEntry)
  When task completes clean review (marked [x])
  Then runner calls GatePromptFn AFTER task completion (FR21)
  And waits for developer input before processing next task
```

### AC4: Gate prompt shows after task completion, not before

```gherkin
Scenario: Gate prompt shows after task completion, not before
  Given gates_enabled and task with [GATE]
  When runner reaches gate point
  Then task is already executed and reviewed (clean review)
  And gate prompt shows AFTER [x] marking
  And developer approves the completed work, not pre-approves
```

### AC5: Approve continues to next task

```gherkin
Scenario: Approve continues to next task
  Given gate prompt returns GateDecision{Action: "approve"}
  When runner processes decision
  Then proceeds to next task in sprint-tasks.md
  And outer loop continues normally
```

### AC6: Quit at gate preserves state

```gherkin
Scenario: Quit at gate preserves state
  Given gate prompt returns GateDecision{Action: "quit"}
  When runner processes decision
  Then returns error wrapping GateDecision (for exit code mapping)
  And cmd/ralph/exit.go maps to exit code 2 (user quit)
  And all completed tasks remain [x] in sprint-tasks.md
  And incomplete tasks remain [ ]
  And re-run continues from first [ ] (FR12)
```

### AC7: Skip at gate continues

```gherkin
Scenario: Skip at gate continues
  Given gate prompt returns GateDecision{Action: "skip"}
  When runner processes decision
  Then task remains [x] (already marked done by review)
  And runner proceeds to next task
```

### AC8: No gate for non-GATE tasks

```gherkin
Scenario: No gate for tasks without [GATE] tag
  Given gates_enabled = true
  And current task does NOT have [GATE] tag
  When task completes clean review
  Then GatePromptFn is NOT called
  And runner proceeds directly to next task
```

## Tasks / Subtasks

- [x] Task 1: Add GatePromptFunc type and field to Runner struct (AC: #1-#8)
  - [x] 1.1: Define `GatePromptFunc` type in `runner/runner.go`: `type GatePromptFunc func(ctx context.Context, taskText string) (*config.GateDecision, error)` — follows ReviewFunc pattern. Takes taskText string (not gates.Gate struct) so runner tests don't need to import gates package
  - [x] 1.2: Add `GatePromptFn GatePromptFunc` field to `Runner` struct alongside ReviewFn, ResumeExtractFn, SleepFn
- [x] Task 2: Wire gate check into Runner.Execute after clean review (AC: #3, #4, #5, #6, #7, #8)
  - [x] 2.1: In `Runner.Execute`, after `if rr.Clean { break }` (line ~384), before outer loop continues to next iteration, add gate check block
  - [x] 2.2: Gate condition: `if r.Cfg.GatesEnabled && result.OpenTasks[0].HasGate && r.GatePromptFn != nil`
  - [x] 2.3: Call `decision, err := r.GatePromptFn(ctx, result.OpenTasks[0].Text)`
  - [x] 2.4: Handle error (ctx cancel, I/O): return `fmt.Errorf("runner: gate: %w", err)`
  - [x] 2.5: Handle quit: return `fmt.Errorf("runner: gate: %w", decision)` — GateDecision implements error, exit.go maps to code 2
  - [x] 2.6: Handle approve/skip: no-op (break already happened, outer loop continues to next task)
  - [x] 2.7: Handle retry: treat as approve for now (Story 5.3 adds proper retry with feedback injection)
  - [x] 2.8: Gate check placement: AFTER the review cycle `break` but BEFORE the outer for-loop iteration continues. Restructure: move `if rr.Clean { break }` to set a flag, then after review cycle loop, check gate
- [x] Task 3: Wire production GatePromptFn in Run() (AC: #1)
  - [x] 3.1: In `Run()` function, add import `"github.com/bmad-ralph/bmad-ralph/gates"` and `"os"`
  - [x] 3.2: Set `r.GatePromptFn` to closure calling `gates.Prompt(ctx, gates.Gate{TaskText: taskText, Reader: os.Stdin, Writer: os.Stdout})`
  - [x] 3.3: Only set GatePromptFn when `cfg.GatesEnabled` — nil field means gate skipped (avoids constructing gate objects unnecessarily)
- [x] Task 4: Write unit tests in `runner/runner_test.go` (AC: #2-#8)
  - [x] 4.1: `TestRunner_Execute_GateApprove` — table-driven: GatesEnabled=true, mock GatePromptFn returns approve, task with [GATE] tag. Verify: GatePromptFn called with task text, runner proceeds to completion. Mock ReviewFn returns Clean, MockClaude succeeds
  - [x] 4.2: `TestRunner_Execute_GateQuit` — GatesEnabled=true, GatePromptFn returns quit. Verify: error wraps GateDecision, `errors.As` extracts Action=="quit", task [x] preserved
  - [x] 4.3: `TestRunner_Execute_GateSkip` — GatesEnabled=true, GatePromptFn returns skip. Verify: runner continues, task [x] preserved, proceeds to next task
  - [x] 4.4: `TestRunner_Execute_GatesDisabled` — GatesEnabled=false, task has [GATE]. Verify: GatePromptFn NOT called (nil or track call count == 0)
  - [x] 4.5: `TestRunner_Execute_NoGateTag` — GatesEnabled=true, task without [GATE]. Verify: GatePromptFn NOT called
  - [x] 4.6: `TestRunner_Execute_GatePromptError` — GatePromptFn returns context.Canceled. Verify: error propagated with "runner: gate:" prefix, `errors.Is(err, context.Canceled)`
  - [x] 4.7: Use existing `Runner` struct construction pattern with mock fields — follow existing test patterns in `runner/runner_test.go`

## Dev Notes

### Architecture Constraints

- **Dependency direction:** `runner → gates` is valid per architecture diagram in `docs/project-context.md`
- **runner MUST NOT pass `io.Reader`/`io.Writer` through RunConfig** — I/O injection is handled via `GatePromptFn` closure, keeping RunConfig clean
- **Gates package remains independent** — runner calls gates through the injectable function, gates knows nothing about runner
- **No `os.Exit` in runner** — quit propagates as error, `cmd/ralph/exit.go` handles exit code mapping

### What Already Exists (DO NOT RECREATE)

| Component | Location | Status |
|-----------|----------|--------|
| `GateDecision` type | `config/errors.go:28-37` | Action + Feedback, implements error |
| `config.GatesEnabled` | `config/config.go:23` | Bool field, YAML + CLI cascade |
| `--gates` CLI flag | `cmd/ralph/run.go:27` | Wired to config.GatesEnabled |
| Scanner `HasGate` | `runner/scan.go:14` | TaskEntry.HasGate bool field |
| `GateTagRegex` | `config/constants.go:29` | Used by ScanTasks |
| Exit code mapping | `cmd/ralph/exit.go:32-37` | GateDecision{quit} → code 2 |
| `ReviewFunc` pattern | `runner/runner.go:51` | Injectable function type — follow same pattern |
| `ResumeExtractFunc` pattern | `runner/runner.go:56` | Injectable closure in Run() — follow same pattern |
| `Runner` struct | `runner/runner.go:224-232` | Add GatePromptFn field here |
| `Run()` function | `runner/runner.go:479-492` | Wire GatePromptFn closure here |

### Story 5.1 Creates (Prerequisites — Assumed Complete)

| Component | Location | Expected |
|-----------|----------|----------|
| `gates.Gate` struct | `gates/gates.go` | `{TaskText string, Reader io.Reader, Writer io.Writer}` |
| `gates.Prompt` | `gates/gates.go` | `func Prompt(ctx, gate) (*config.GateDecision, error)` |
| Action constants | `config/constants.go` | `ActionApprove`, `ActionSkip`, `ActionQuit`, `ActionRetry` |

### Critical: Gate Check Placement in Runner.Execute

The gate check must happen **after clean review, before next task iteration**. Current code structure:

```go
// Current (lines 380-392 in runner.go):
rr, err := r.ReviewFn(ctx, rc)
if err != nil {
    return fmt.Errorf("runner: review: %w", err)
}
if rr.Clean {
    break  // exits review cycle loop → outer for-loop continues to next task
}
reviewCycles++
// ...
```

**Required restructure:** The `break` exits the review cycle inner loop, then the outer loop continues to next iteration (next task). Gate check must go between review cycle exit and next task processing:

```go
// After review cycle loop exits (rr.Clean == true):
// Gate check — ONLY for tasks with [GATE] tag and gates enabled
if r.Cfg.GatesEnabled && result.OpenTasks[0].HasGate && r.GatePromptFn != nil {
    decision, gateErr := r.GatePromptFn(ctx, result.OpenTasks[0].Text)
    if gateErr != nil {
        return fmt.Errorf("runner: gate: %w", gateErr)
    }
    if decision.Action == config.ActionQuit {
        return fmt.Errorf("runner: gate: %w", decision)
    }
    // approve, skip, retry (future) → continue to next task
}
// Outer loop continues: re-read tasks, scan, process next open task
```

Note: `result.OpenTasks[0]` is safe here because we checked `result.HasOpenTasks()` earlier and haven't modified the scan result.

### Decision Handling Matrix

| Decision | Action in 5.2 | Exit Code | Notes |
|----------|---------------|-----------|-------|
| approve | continue to next task | — | No-op after clean review |
| skip | continue to next task | — | Task already [x] from review |
| quit | return wrapped error | 2 | `cmd/ralph/exit.go` handles mapping |
| retry | continue (like approve) | — | Story 5.3 adds proper retry logic |

### Error Wrapping Pattern

- Gate prompt error: `fmt.Errorf("runner: gate: %w", err)` — for ctx cancel, I/O errors
- Gate quit: `fmt.Errorf("runner: gate: %w", decision)` — GateDecision implements error, unwrappable via errors.As in exit.go
- Consistent with existing runner error prefixes: "runner: execute:", "runner: review:", "runner: gate:"

### Testing Approach

- **Runner.Execute tests** — construct Runner with mock fields (ReviewFn, GatePromptFn, etc.)
- **Mock GatePromptFn** — simple closure returning fixed GateDecision or error
- **Track call counts** — use counter variable in closure to verify GatePromptFn called/not called
- **Task fixtures** — sprint-tasks.md content with `[GATE]` tags: `"- [ ] Setup project [GATE]\n- [ ] Write tests"`
- **MockClaude + MockGitClient** — existing test infrastructure from testutil package
- **Table-driven** where possible (approve/skip/quit as table cases)
- **Verify taskText** passed to GatePromptFn matches the scanned task text

### Naming Conventions

- Type: `GatePromptFunc` (not `GateFunc` or `PromptFunc` — matches domain + function name)
- Field: `GatePromptFn` (matches `ReviewFn`, `ResumeExtractFn`, `SleepFn` pattern)
- Test names: `TestRunner_Execute_Gate<Scenario>` (follows `TestRunner_Execute_<Scenario>` pattern)
- Error prefix: `"runner: gate:"` (new prefix, joins existing "runner: execute:", "runner: review:")

### What This Story Does NOT Include

- Feedback text input after retry (Story 5.3)
- Checkpoint gates every N tasks (Story 5.4 — adds counter + checkpoint gate logic)
- Emergency gate upgrade (Story 5.5 — replaces ErrMaxRetries/ErrMaxReviewCycles returns)
- Integration tests with real gates.Prompt (Story 5.6)

### Project Structure Notes

- **Modified file:** `runner/runner.go` — add GatePromptFunc type, Runner field, gate check in Execute, wire in Run()
- **Modified file:** `runner/runner_test.go` — new gate detection test cases
- **No new files** — all changes in existing runner package
- **New import in runner.go:** `"github.com/bmad-ralph/bmad-ralph/gates"` (only in Run() production wiring)

### References

- [Source: docs/epics/epic-5-human-gates-control-stories.md#Story 5.2]
- [Source: docs/project-context.md#Dependency Direction] — runner → gates valid
- [Source: docs/project-context.md#Package Entry Points] — gates.Prompt(ctx, gate)
- [Source: runner/runner.go:224-232] — Runner struct (add GatePromptFn field)
- [Source: runner/runner.go:380-392] — Review cycle loop exit (insertion point for gate check)
- [Source: runner/runner.go:479-492] — Run() function (wire GatePromptFn)
- [Source: runner/scan.go:11-15] — TaskEntry.HasGate field
- [Source: config/errors.go:28-37] — GateDecision type
- [Source: cmd/ralph/exit.go:32-37] — GateDecision quit → exit code 2

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None — all tests passed on first run.

### Completion Notes List

- Task 1: Defined `GatePromptFunc` type following `ReviewFunc` pattern. Added `GatePromptFn` field to `Runner` struct between `ReviewFn` and `ResumeExtractFn`.
- Task 2: Wired gate check AFTER review cycle loop exits (clean review), BEFORE outer for-loop continues to next task. Three-way condition: `GatesEnabled && HasGate && GatePromptFn != nil`. Quit returns wrapped `GateDecision` error (exit code 2 via `cmd/ralph/exit.go`). Approve/skip/retry are no-ops (continue to next task).
- Task 3: Wired production `GatePromptFn` closure in `Run()` only when `cfg.GatesEnabled` — nil field means gate check is skipped. Added `gates` import to runner package.
- Task 4: 5 test functions covering 7 scenarios: approve/skip/retry (table-driven), quit (errors.As unwrap), disabled (count==0), no-tag (count==0), prompt error (context.Canceled propagation). All use `trackingGatePrompt` helper for call-count verification.

### Change Log

- 2026-02-27: Story 5.2 implementation — gate detection in runner (Tasks 1-4)

### File List

- runner/runner.go (modified: GatePromptFunc type, Runner.GatePromptFn field, gate check in Execute, gates.Prompt wiring in Run)
- runner/runner_test.go (modified: 5 new gate detection test functions)
- runner/test_helpers_test.go (modified: gate task fixtures, trackingGatePrompt helper)
- docs/sprint-artifacts/sprint-status.yaml (modified: 5-2 status → in-progress)
- docs/sprint-artifacts/5-2-gate-detection-in-runner.md (modified: tasks marked [x], Dev Agent Record)
