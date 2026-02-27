# Story 5.1: Basic Gate Prompt

Status: Done

## Story

As a developer running `ralph run --gates`,
I want an interactive gate prompt with colored options (approve/skip/quit/retry),
so that I can control execution at gate points and decide whether to proceed, skip, retry, or stop.

## Acceptance Criteria

### AC1: Gate prompt displays with color

```gherkin
Scenario: Gate prompt displays with color
  Given gate triggered for task "TASK-1 — Setup project structure"
  When gates.Prompt(ctx, gate) is called
  Then displays colored prompt via fatih/color:
    🚦 HUMAN GATE: TASK-1 — Setup project structure
       [a]pprove  [r]etry with feedback  [s]kip  [q]uit
    > _
  And waits for user input via io.Reader (injectable for testing)
```

### AC2: Approve action

```gherkin
Scenario: Approve action
  Given gate prompt displayed
  When user enters "a"
  Then returns GateDecision{Action: "approve"}
  And runner proceeds to next task
```

### AC3: Skip action

```gherkin
Scenario: Skip action
  Given gate prompt displayed
  When user enters "s"
  Then returns GateDecision{Action: "skip"}
  And current task marked [x] (skipped)
  And runner proceeds to next task
```

### AC4: Quit action

```gherkin
Scenario: Quit action
  Given gate prompt displayed
  When user enters "q"
  Then returns GateDecision{Action: "quit"}
  And runner exits with code 2 (user quit — exit code table in cmd/ralph/exit.go)
  And sprint-tasks.md state preserved (incomplete tasks remain [ ])
```

### AC5: Retry action (feedback deferred to Story 5.3)

```gherkin
Scenario: Retry action
  Given gate prompt displayed
  When user enters "r"
  Then returns GateDecision{Action: "retry", Feedback: ""}
  And feedback text input is NOT prompted here (Story 5.3 adds feedback input)
```

### AC6: Invalid input re-prompts

```gherkin
Scenario: Invalid input re-prompts
  Given gate prompt displayed
  When user enters invalid input (e.g., "x", "hello", empty line)
  Then displays error message (e.g., "Unknown option: x")
  And re-displays prompt options
  And does not advance or quit
```

### AC7: Context cancellation during prompt

```gherkin
Scenario: Context cancellation during prompt
  Given gate prompt waiting for input
  When ctx is cancelled (Ctrl+C / signal)
  Then returns context.Canceled error (not GateDecision)
  And runner handles graceful shutdown via existing signal handling
```

### AC8: EOF on reader

```gherkin
Scenario: Input exhausted (EOF)
  Given gate prompt waiting for input
  When io.Reader reaches EOF (piped input ends, empty reader)
  Then returns io.EOF error (wrapped with "gates: prompt:" prefix)
  And does not loop infinitely
```

## Tasks / Subtasks

- [x] Task 1: Define gate action constants and Gate struct (AC: #1-#5)
  - [x] 1.1: Add action constants to existing `config/constants.go` const block: `ActionApprove = "approve"`, `ActionRetry = "retry"`, `ActionSkip = "skip"`, `ActionQuit = "quit"` — alongside GateTag/FeedbackPrefix, accessible to both `gates` and `cmd/ralph` without new imports
  - [x] 1.2: Define `Gate` struct in `gates/gates.go` with fields: `TaskText string` (displayed in prompt header), `Reader io.Reader` (input source), `Writer io.Writer` (output target)
  - [x] 1.3: No GateType enum yet — KISS. Story 5.5 adds emergency type when needed
- [x] Task 2: Implement `Prompt` function in `gates/gates.go` (AC: #1-#8)
  - [x] 2.1: Signature: `func Prompt(ctx context.Context, gate Gate) (*config.GateDecision, error)` — returns decision as first value on valid input, error only for ctx cancel/I/O/EOF (see GateDecision Return Contract in Dev Notes)
  - [x] 2.2: Display colored header via `fatih/color`: bold cyan "🚦 HUMAN GATE:" + gate.TaskText
  - [x] 2.3: Display options line: `[a]pprove  [r]etry with feedback  [s]kip  [q]uit`
  - [x] 2.4: Display `> ` prompt marker, read line via single `bufio.Scanner` goroutine sending lines to a channel (see Implementation Pattern in Dev Notes — single goroutine for all reads, not one per loop iteration)
  - [x] 2.5: Parse trimmed lowercase input: "a" → `config.ActionApprove`, "r" → `config.ActionRetry`, "s" → `config.ActionSkip`, "q" → `config.ActionQuit`
  - [x] 2.6: Invalid input → write error message to gate.Writer (e.g., "Unknown option: x"), loop back to re-display options
  - [x] 2.7: EOF → return `fmt.Errorf("gates: prompt: %w", io.EOF)` — prevents infinite loop on exhausted/piped input
  - [x] 2.8: Context cancellation: select on `ctx.Done()` vs input channel — return `fmt.Errorf("gates: prompt: %w", ctx.Err())`
  - [x] 2.9: Return `&config.GateDecision{Action: matched_action}` — Feedback field empty (Story 5.3)
- [x] Task 3: Write unit tests in `gates/gates_test.go` (AC: #1-#8)
  - [x] 3.1: `TestPrompt_ValidActions` — table-driven: cases for "a"→ActionApprove, "s"→ActionSkip, "q"→ActionQuit, "r"→ActionRetry, "A"→ActionApprove (uppercase), "  a  \n"→ActionApprove (whitespace trimmed). Each: `strings.NewReader(input)`, verify `decision.Action`, `decision.Feedback == ""`, `err == nil`
  - [x] 3.2: `TestPrompt_InvalidThenValid` — table-driven: "x\na\n"→Approve (verify writer contains "Unknown option"), "z\n\nhello\ns\n"→Skip (multiple invalid before valid). Verify final decision AND error messages in output buffer
  - [x] 3.3: `TestPrompt_ContextCancelled` — use `io.Pipe()` reader (blocks forever) with pre-cancelled context. Verify `errors.Is(err, context.Canceled)` and `decision == nil`. Do NOT use `strings.NewReader` — goroutine may read before ctx check, causing race
  - [x] 3.4: `TestPrompt_OutputFormat` — verify `bytes.Buffer` output contains: gate.TaskText, "[a]pprove", "[s]kip", "[q]uit", "[r]etry", "HUMAN GATE"
  - [x] 3.5: `TestPrompt_EOF` — `strings.NewReader("")` (immediate EOF), verify `errors.Is(err, io.EOF)` and `decision == nil`

## Dev Notes

### Architecture Constraints

- **Dependency direction:** `gates` depends only on `config` (for GateDecision type + action constants) and stdlib. MUST NOT import `runner`, `session`, or `bridge`
- **Package entry point:** `gates.Prompt(ctx, gate)` — single exported function per architecture table in `docs/project-context.md`
- **Action constants in `config/constants.go`:** follows project pattern where cross-package constants live alongside GateTag/FeedbackPrefix. Avoids `cmd/ralph/exit.go` needing to import `gates`. Note: `exit.go:34` currently hardcodes `"quit"` — Story 5.2 can update to `config.ActionQuit`

### GateDecision Return Contract

`Prompt` returns `(*config.GateDecision, error)`:
- **First return** (decision): populated on valid user input — this is a VALUE, not an error
- **Second return** (error): only for ctx cancel, I/O error, or EOF
- **Never** return GateDecision in both positions. `GateDecision` implements `error` interface for runner's exit-code propagation (via `errors.As` in `cmd/ralph/exit.go`), NOT for Prompt's return contract
- Runner (Story 5.2) wraps quit decisions: `fmt.Errorf("runner: gate: %w", decision)` to propagate through error chain

### What Already Exists (DO NOT RECREATE)

| Component | Location | Status |
|-----------|----------|--------|
| `GateDecision` type | `config/errors.go:28-37` | Action + Feedback fields, implements error |
| `GateTag` constant | `config/constants.go:10` | `"[GATE]"` |
| `FeedbackPrefix` constant | `config/constants.go:11` | `"> USER FEEDBACK:"` |
| `GateTagRegex` | `config/constants.go:20` | Compiled regex |
| `GatesEnabled` config | `config/config.go:23` | YAML + CLI flag + cascade |
| `GatesCheckpoint` config | `config/config.go:24` | YAML + CLI flag + cascade |
| `--gates` CLI flag | `cmd/ralph/run.go:27` | Bool flag, wired to config |
| Scanner `HasGate` | `runner/scan.go:14` | On TaskEntry struct |
| Exit code mapping | `cmd/ralph/exit.go:32-37` | GateDecision quit → code 2 |
| `fatih/color` dep | `go.mod` | Already in go.mod |
| `gates` package | `gates/gates.go` | Empty — only `package gates` |

### Implementation Pattern: Context-Aware Input Loop

Single scanner goroutine reads lines into a channel. Main loop selects between ctx.Done() and input. One goroutine for ALL reads — not one per loop iteration:

```go
type readResult struct {
    text string
    err  error
}

scanner := bufio.NewScanner(gate.Reader)
lineCh := make(chan readResult, 1)

go func() {
    defer close(lineCh)
    for scanner.Scan() {
        lineCh <- readResult{text: scanner.Text()}
    }
    if err := scanner.Err(); err != nil {
        lineCh <- readResult{err: err}
    } else {
        lineCh <- readResult{err: io.EOF}
    }
}()

for {
    // display prompt to gate.Writer
    select {
    case <-ctx.Done():
        return nil, fmt.Errorf("gates: prompt: %w", ctx.Err())
    case r, ok := <-lineCh:
        if !ok || r.err != nil {
            return nil, fmt.Errorf("gates: prompt: %w", r.err)
        }
        // parse r.text → action
    }
}
```

Notes:
- Single goroutine — no goroutine-per-iteration leak on invalid input
- Goroutine may block if ctx cancels while real stdin blocks — acceptable for CLI (process exits soon)
- `close(lineCh)` ensures channel cleanup on reader exhaustion
- EOF sends `io.EOF` so loop returns error instead of looping infinitely

### Naming Conventions

- Error wrapping: `fmt.Errorf("gates: prompt: %w", err)` — package: function: inner
- Test names: `TestPrompt_<Scenario>` — Prompt is the only exported function
- Action constants: `config.Action*` prefix in `config/constants.go`

### Testing Approach

- **Table-driven** for valid actions (6 cases in one function) and invalid-then-valid scenarios
- **io.Reader/io.Writer injection** — `strings.NewReader` for input, `bytes.Buffer` for output
- **io.Pipe() for context cancel test** — blocks forever, ensures ctx.Done() wins the select race
- **Go stdlib assertions** — no testify, `t.Errorf`/`t.Fatalf` for failures
- **`t.TempDir()` not needed** — no file I/O in gates package
- **`errors.Is`** for context.Canceled and io.EOF checks — project standard

### Color Usage

- `fatih/color` already a project dependency
- Gate header: `color.New(color.FgCyan, color.Bold)` for "🚦 HUMAN GATE:" prefix
- Error messages: `color.New(color.FgRed)` for "Unknown option" text
- Options line: regular output — keep color usage minimal (KISS)

### What This Story Does NOT Include

- Gate detection in runner loop (Story 5.2)
- Feedback text input after retry selection (Story 5.3)
- Checkpoint gate counting (Story 5.4)
- Emergency gate styling/type (Story 5.5)
- Integration tests with runner (Story 5.6)

### References

- [Source: docs/epics/epic-5-human-gates-control-stories.md#Story 5.1]
- [Source: docs/project-context.md#Package Entry Points] — `gates.Prompt(ctx, gate)`
- [Source: docs/project-context.md#Dependency Direction] — gates independent of runner/session
- [Source: docs/project-context.md#Error Handling] — GateDecision as custom error type
- [Source: config/errors.go:28-37] — GateDecision type definition
- [Source: config/constants.go:7-12] — GateTag, FeedbackPrefix constants
- [Source: cmd/ralph/exit.go:32-37] — GateDecision quit → exit code 2
- [Source: runner/scan.go:11-15] — TaskEntry.HasGate field

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No issues encountered during implementation.

### Completion Notes List

- Task 1: Added `ActionApprove`, `ActionRetry`, `ActionSkip`, `ActionQuit` constants to `config/constants.go` const block. Defined `Gate` struct in `gates/gates.go` with `TaskText`, `Reader`, `Writer` fields. No GateType enum (KISS).
- Task 2: Implemented `Prompt(ctx, gate)` function with context-aware input loop using single scanner goroutine pattern. Colored output via `fatih/color` (bold cyan header, red error). Returns `*config.GateDecision` on valid input, error for ctx cancel/EOF/I/O. All error returns wrapped with `"gates: prompt:"` prefix.
- Task 3: 6 test functions (15 sub-cases total) covering all ACs: valid actions (table-driven, 6 cases), invalid-then-valid (table-driven, 2 cases), context cancellation (io.Pipe for blocking), output format verification, EOF handling, scanner I/O error. All use `errors.Is` + `strings.Contains` for error assertions.

### File List

- config/constants.go (modified — added ActionApprove/ActionRetry/ActionSkip/ActionQuit constants, fixed doc comment)
- config/constants_test.go (modified — added 4 Action*_Value tests)
- gates/gates.go (modified — added Gate struct, readResult type, Prompt function, empty input UX)
- gates/gates_test.go (new — 6 test functions, 15 sub-cases)
- docs/sprint-artifacts/sprint-status.yaml (modified — 5-1 status: ready-for-dev → in-progress → review → done)
- docs/sprint-artifacts/5-1-basic-gate-prompt.md (modified — tasks marked, Dev Agent Record filled, review applied)

### Change Log

- 2026-02-27: Implemented Story 5.1 — Basic Gate Prompt. Added gate action constants to config, Gate struct and Prompt function to gates package with full test coverage.
- 2026-02-27: Code review — 7 findings (5M/2L), all fixed: doc comment accuracy (M1), count assertion (M2), scanner error test (M3), Feedback symmetry (M4), Action value tests (M5), prompt marker assertion (L1), empty input UX (L2).
