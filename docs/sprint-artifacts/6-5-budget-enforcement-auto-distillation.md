# Story 6.5a: Budget Check & Distillation Trigger

Status: ready-for-review

## Story

As a runner after clean review,
I want to automatically check LEARNINGS.md size and trigger distillation when soft threshold exceeded with cooldown control and human gate on failure,
so that distillation runs at the right time with user control.

## Acceptance Criteria

```gherkin
Scenario: Budget check after clean review — under limit
  Given clean review completed (task marked [x])
  And LEARNINGS.md has 100 lines
  When runner checks budget
  Then no action taken
  And runner proceeds to next task

Scenario: Auto-distillation trigger at soft threshold 150 lines
  Given clean review completed
  And LEARNINGS.md has 160 lines (exceeds soft threshold 150)
  And cooldown check passes: MonotonicTaskCounter - LastDistillTask >= 5 (H1)
  When runner triggers auto-distillation
  Then distillation pipeline invoked (Story 6.5b)

Scenario: Cooldown via MonotonicTaskCounter (H1)
  Given MonotonicTaskCounter in DistillState = 15
  And LastDistillTask = 12
  And LEARNINGS.md exceeds 150 lines
  When runner checks budget
  Then cooldown check: 15 - 12 = 3 < 5 → cooldown NOT met
  And no distillation triggered
  And runner continues

Scenario: Human GATE on distillation failure (v6: human only)
  Given auto-distillation failed (crash, timeout >2min, bad format, validation reject, I/O error)
  When failure detected
  Then human GATE presented with error description + current file size status
  And gate options: skip, retry once, retry 5 times
  And if retry: re-run distillation (up to selected count)
  And if skip: restore all backups, log warning, continue
  And runner continues normally after gate resolution

Scenario: Bad format gets free retry with reinforced prompt (H4)
  Given distillation output is unparseable (missing BEGIN/END markers or bad structure)
  When failure type = bad_format
  Then ONE automatic retry with reinforced prompt instructions (no gate yet)
  And if retry also fails: human gate (skip/retry 1/retry 5)

Scenario: Missing LEARNINGS.md — no action
  Given LEARNINGS.md does not exist
  When runner checks budget
  Then no distillation triggered
  And runner proceeds normally

Scenario: DistillFunc injectable for testing
  Given Runner struct has DistillFn field (like ReviewFn, GatePromptFn)
  When runner.Run initializes
  Then DistillFn wired to AutoDistill closure
  And tests can inject custom DistillFunc implementations
```

## Tasks / Subtasks

- [x] Task 1: Define DistillFunc type and DistillState struct (AC: #7, #3)
  - [x] 1.1 Define `DistillFunc` type in runner.go: `type DistillFunc func(ctx context.Context, state *DistillState) error`
  - [x] 1.2 Define `DistillState` struct in new file `runner/knowledge_state.go`:
    - `MonotonicTaskCounter int` — incremented at each clean review, never resets (H1)
    - `LastDistillTask int` — MonotonicTaskCounter value at last successful distillation
    - `Version int` — forward compatibility (default: 1)
  - [x] 1.3 `LoadDistillState(path string) (*DistillState, error)` — reads JSON, returns default `{Version:1}` on NotExist
  - [x] 1.4 `SaveDistillState(path string, state *DistillState) error` — writes JSON with 0644 perms
  - [x] 1.5 Error wrapping: `"runner: distill state: load:"`, `"runner: distill state: save:"` prefixes
  - [x] 1.6 Path: `{projectRoot}/.ralph/distill-state.json`
  - [x] 1.7 Create `.ralph/` dir with `os.MkdirAll` if not exists (before save)

- [x] Task 2: Add DistillFn to Runner struct and wire in Run() (AC: #7)
  - [x] 2.1 Add `DistillFn DistillFunc` field to Runner struct (after ResumeExtractFn)
  - [x] 2.2 Wire in `Run()`: `r.DistillFn = func(ctx context.Context, state *DistillState) error { return AutoDistill(ctx, cfg, state) }` — placeholder that returns nil until Story 6.5b implements it
  - [x] 2.3 `AutoDistill` stub: `func AutoDistill(ctx context.Context, cfg *config.Config, state *DistillState) error { return nil }` in knowledge_state.go
  - [x] 2.4 Update Runner doc comment to include DistillFn description

- [x] Task 3: Add DistillCooldown to Config (AC: #3)
  - [x] 3.1 Add `DistillCooldown int \`yaml:"distill_cooldown"\`` to Config struct (after LearningsBudget)
  - [x] 3.2 Add `distill_cooldown: 5` to defaults.yaml
  - [x] 3.3 No CLI flag needed — config file or default only

- [x] Task 4: Wire budget check + distillation trigger in Execute() (AC: #1, #2, #3, #6)
  - [x] 4.1 Trigger point: AFTER gate check (line ~754), BEFORE `continue` to next iteration
  - [x] 4.2 Load DistillState at Execute() startup (after Serena detection, before outer loop)
  - [x] 4.3 Increment `distillState.MonotonicTaskCounter` after each clean review (not emergency skip)
  - [x] 4.4 Save DistillState after increment (persists counter across runs)
  - [x] 4.5 Budget check: call `BudgetCheck(ctx, learningsPath, r.Cfg.LearningsBudget)`
  - [x] 4.6 If `!budgetStatus.NearLimit` → no action, continue
  - [x] 4.7 Cooldown check: `distillState.MonotonicTaskCounter - distillState.LastDistillTask >= r.Cfg.DistillCooldown`
  - [x] 4.8 If cooldown not met → no action, continue
  - [x] 4.9 Both checks pass → call `r.DistillFn(ctx, distillState)`
  - [x] 4.10 On success: update `distillState.LastDistillTask = distillState.MonotonicTaskCounter`, save state
  - [x] 4.11 Missing LEARNINGS.md: BudgetCheck returns `{NearLimit: false}` → no action (existing behavior from Story 6.1)

- [x] Task 5: Distillation failure handling with human gate (AC: #4, #5)
  - [x] 5.1 Define `ErrBadFormat` sentinel: `var ErrBadFormat = errors.New("bad format")` in knowledge_state.go
  - [x] 5.2 On DistillFn error with `errors.Is(err, ErrBadFormat)`: ONE automatic retry (no gate)
  - [x] 5.3 If retry also fails (any error): fall through to human gate
  - [x] 5.4 On any other DistillFn error: immediate human gate
  - [x] 5.5 Human gate: use `r.GatePromptFn(ctx, gateText)` with error description + file size info
  - [x] 5.6 Gate text format: `"distillation failed: <error>. LEARNINGS.md: <lines>/<limit> lines"`
  - [x] 5.7 Gate option "skip": log warning via `fmt.Fprintf(os.Stderr, ...)`, continue to next task
  - [x] 5.8 Gate option "retry once": call DistillFn again once
  - [x] 5.9 Gate option "retry 5": call DistillFn up to 5 more times (break on success)
  - [x] 5.10 Gate option "quit": return wrapped GateDecision error (exit code 2)
  - [x] 5.11 After ALL retries exhausted without success → log warning, continue (non-fatal)
  - [x] 5.12 All distillation failures are non-fatal: NEVER interrupt task loop

- [x] Task 6: Tests (AC: all)
  - [x] 6.1 `TestDistillState_LoadSave` — round-trip: save → load → verify fields
  - [x] 6.2 `TestDistillState_LoadNotExist` — missing file returns default `{Version:1}`
  - [x] 6.3 `TestDistillState_LoadInvalid` — corrupt JSON returns error
  - [x] 6.4 `TestDistillState_SaveCreatesDir` — `.ralph/` created if not exists
  - [x] 6.5 `TestRunner_Execute_BudgetUnderLimit` — 100 lines, no distillation triggered (wantDistillCount=0)
  - [x] 6.6 `TestRunner_Execute_DistillationTrigger` — 160 lines, cooldown met → DistillFn called (wantDistillCount=1)
  - [x] 6.7 `TestRunner_Execute_CooldownNotMet` — 160 lines, cooldown 3<5 → no distillation (wantDistillCount=0)
  - [x] 6.8 `TestRunner_Execute_DistillationBadFormatRetry` — ErrBadFormat → 1 free retry → success (wantDistillCount=2)
  - [x] 6.9 `TestRunner_Execute_DistillationBadFormatRetryFails` — ErrBadFormat → retry also fails → human gate
  - [x] 6.10 `TestRunner_Execute_DistillationFailureHumanGate` — non-format error → gate → skip → continue
  - [x] 6.11 `TestRunner_Execute_MissingLearnings` — no LEARNINGS.md → no distillation
  - [x] 6.12 `TestRunner_Execute_MonotonicCounterPersist` — counter incremented and saved after clean review
  - [x] 6.13 `TestRunner_Execute_DistillSuccess_UpdatesLastDistillTask` — LastDistillTask updated after success
  - [x] 6.14 Add `trackingDistillFunc` to test_helpers_test.go: tracks call count, returns configurable error
  - [x] 6.15 Config test: `TestConfig_DistillCooldown_Default` — verifies default=5

## Dev Notes

### Architecture & Design Decisions

- **Trigger point:** runner.go Execute(), AFTER gate check (~line 754), BEFORE outer loop continues to next task. Clean review completed, gate passed — now check distillation.
- **Non-fatal:** ALL distillation failures → human gate or log warning → continue. NEVER return error from distillation path — task loop MUST continue.
- **H1 — MonotonicTaskCounter:** Persisted in DistillState JSON, never resets. Incremented at each clean review. Cooldown: `MonotonicTaskCounter - LastDistillTask >= DistillCooldown`. Solves cross-session problem (completedTasks resets on restart, MonotonicTaskCounter does not).
- **H4 — Bad format free retry:** `ErrBadFormat` sentinel. ONE automatic retry before gate. Other errors go directly to gate.
- **v6 simplification:** Human gate ONLY (no auto/circuit breaker). Options: skip, retry 1, retry 5.
- **DistillFunc injectable:** Same pattern as ReviewFn, GatePromptFn, ResumeExtractFn. Tests inject tracking mock.
- **DistillState location:** `{projectRoot}/.ralph/distill-state.json` (v6 architecture: `.ralph/` for Ralph state files).
- **Config:** Only `distill_cooldown: 5` added. No distill_gate config needed — always human.
- **AutoDistill stub:** Returns nil until Story 6.5b implements actual distillation pipeline. This story only wires the trigger mechanism.

### File Layout

| File | Purpose |
|------|---------|
| `runner/knowledge_state.go` | NEW: DistillState, DistillFunc type, LoadDistillState, SaveDistillState, AutoDistill stub, ErrBadFormat |
| `runner/knowledge_state_test.go` | NEW: DistillState load/save tests |
| `runner/runner.go` | MODIFY: add DistillFn to Runner, wire trigger in Execute(), wire in Run() |
| `runner/runner_test.go` | MODIFY: add distillation trigger tests |
| `runner/test_helpers_test.go` | MODIFY: add trackingDistillFunc |
| `config/config.go` | MODIFY: add DistillCooldown field |
| `config/defaults.yaml` | MODIFY: add distill_cooldown: 5 |

### Existing Code References

**BudgetCheck (knowledge_write.go:396):**
```go
func BudgetCheck(_ context.Context, learningsPath string, limit int) (BudgetStatus, error)
// Returns BudgetStatus{NearLimit: lines >= 150, OverBudget: lines >= limit}
// Missing file → {NearLimit: false}, nil
```

**Runner struct (runner.go:401-412):**
```go
type Runner struct {
    Cfg                   *config.Config
    Git                   GitClient
    TasksFile             string
    ReviewFn              ReviewFunc
    GatePromptFn          GatePromptFunc
    EmergencyGatePromptFn GatePromptFunc
    ResumeExtractFn       ResumeExtractFunc
    SleepFn               func(time.Duration)
    Knowledge             KnowledgeWriter
    CodeIndexer           CodeIndexerDetector
    // ADD: DistillFn DistillFunc
}
```

**Execute() trigger insertion point (runner.go:~754):**
After gate check `approve/skip → continue` falls through to end of for-loop body.
New distillation check goes here — after gate, before closing `}` of outer loop.

**Run() wiring (runner.go:859-886):**
```go
r := &Runner{...}
// ADD: r.DistillFn = func(ctx context.Context, state *DistillState) error { ... }
```

### DistillState JSON Format

```json
{
  "version": 1,
  "monotonic_task_counter": 15,
  "last_distill_task": 10
}
```

### Distillation Trigger Flow

```
clean review → post-validate LEARNINGS.md → (skip if emergency) → increment completedTasks → gate check
                                                                                              ↓
                                                                              increment MonotonicTaskCounter → save state
                                                                                              ↓
                                                                              BudgetCheck: NearLimit? (≥150 lines)
                                                                                              ↓ yes
                                                                              Cooldown: counter - lastDistill ≥ 5?
                                                                                              ↓ yes
                                                                              DistillFn(ctx, state)
                                                                                              ↓ error?
                                                                              ErrBadFormat? → free retry → still error? → gate
                                                                              other error? → gate (skip/retry 1/retry 5)
                                                                                              ↓ success
                                                                              lastDistillTask = counter → save state → continue
```

### Error Wrapping Convention

```go
// New:
fmt.Errorf("runner: distill state: load: %w", err)
fmt.Errorf("runner: distill state: save: %w", err)
// Distillation errors are logged/gated, NOT returned (non-fatal)
fmt.Fprintf(os.Stderr, "WARNING: distillation failed: %v\n", err)
```

### Dependency Direction

```
runner/runner.go → runner/knowledge_state.go (new, same package)
config/config.go (DistillCooldown field — leaf, no new imports)
```

No new external packages.

### Testing Standards

- Table-driven, Go stdlib assertions, `t.TempDir()` for isolation
- `trackingDistillFunc`: counts calls, returns configurable error sequence (e.g., `[ErrBadFormat, nil]` for retry success)
- DistillState tests: isolated via t.TempDir, test all paths (load/save/missing/corrupt)
- Runner integration: mock DistillFn, verify call counts + state mutations
- `errors.Is(err, ErrBadFormat)` for sentinel check
- Non-fatal: ALL distillation test cases must verify Execute() returns nil (except quit gate)

### Code Review Learnings

- Dead parameters: don't add DistillState fields that won't be used until Story 6.5b/6.5c
- Non-interface methods: DistillFunc is a func type (not interface) — follows existing pattern
- Error wrapping consistency: ALL error returns in Load/Save must use same prefix
- Post-loop processing guard: distillation runs inside loop body, not post-loop — no guard needed

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.5a (lines 492-567)]
- [Source: runner/knowledge_write.go:396-411 — BudgetCheck function]
- [Source: runner/knowledge_write.go:13-19 — SoftDistillationThreshold=150]
- [Source: runner/runner.go:401-412 — Runner struct to extend]
- [Source: runner/runner.go:700-754 — Execute trigger insertion area]
- [Source: runner/runner.go:859-886 — Run() wiring]
- [Source: config/config.go:18-34 — Config struct to extend]
- [Source: config/defaults.yaml — add distill_cooldown]

## Dev Agent Record

### Agent Model Used
claude-opus-4-6

### Completion Notes List
- Task 1: Created runner/knowledge_state.go with DistillState, LoadDistillState, SaveDistillState, AutoDistill stub, ErrBadFormat sentinel
- Task 2: Added DistillFn to Runner struct, wired in Run() with AutoDistill closure
- Task 3: Added DistillCooldown to config.Config + defaults.yaml (default: 5)
- Task 4: Wired budget check + distillation trigger in Execute() after gate check: MonotonicTaskCounter increment, BudgetCheck NearLimit, cooldown check
- Task 5: runDistillation (ErrBadFormat free retry) + handleDistillFailure (human gate: skip/retry/quit). All failures non-fatal.
- Task 6: 15 tests total — 5 DistillState unit tests, 9 Execute integration tests, 1 Config default test (added to existing TestConfig_Load_DefaultsComplete)
- Fixed pre-existing TestRealReview_SnapshotReadError → renamed to TestRealReview_LearningsReadError (buildKnowledgeReplacements reads LEARNINGS.md before snapshot)
- Full regression: all packages pass

### File List
- `runner/knowledge_state.go` — NEW: DistillState, DistillFunc, Load/Save, AutoDistill stub, ErrBadFormat
- `runner/knowledge_state_test.go` — NEW: 5 DistillState tests
- `runner/runner.go` — MODIFIED: DistillFn in Runner, distillation trigger in Execute(), runDistillation, handleDistillFailure, Run() wiring
- `runner/runner_test.go` — MODIFIED: 9 Execute distillation tests + TestRealReview_LearningsReadError
- `runner/test_helpers_test.go` — MODIFIED: trackingDistillFunc, noopDistillFn, writeLearningsFile, writeDistillState helpers
- `config/config.go` — MODIFIED: DistillCooldown field
- `config/defaults.yaml` — MODIFIED: distill_cooldown: 5
- `config/config_test.go` — MODIFIED: DistillCooldown default assertion added
