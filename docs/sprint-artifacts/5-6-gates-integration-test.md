# Story 5.6: Gates Integration Test

Status: Done

## Story

As a developer,
I want comprehensive integration tests for the gates system covering all gate types and actions,
so that I can guarantee the correctness of interactive control across normal gates, checkpoint gates, and emergency gates.

## Acceptance Criteria

### AC1: Approve at GATE-tagged task

```gherkin
Scenario: Approve at GATE tag
  Given sprint-tasks.md with [GATE] tagged task
  And gates_enabled = true
  And mock stdin returns "a"
  When runner.Execute runs
  Then gate prompt fires after task completion (clean review)
  And approve continues to next task
  And task remains [x] in sprint-tasks.md
  And no error returned
```

### AC2: Quit at gate preserves state

```gherkin
Scenario: Quit at gate
  Given gate triggered on [GATE] task
  And mock stdin returns "q"
  When runner processes quit
  Then returns error wrapping GateDecision{Action: "quit"}
  And errors.As extracts GateDecision with Action == config.ActionQuit
  And error message contains "runner: gate:" prefix
  And completed tasks remain [x] in sprint-tasks.md
  And incomplete tasks remain [ ]
```

### AC3: Retry with feedback at gate

```gherkin
Scenario: Retry with feedback
  Given gate triggered on [GATE] task
  And mock stdin returns "r" then "fix validation" then empty line
  When runner processes retry
  Then feedback injected into sprint-tasks.md with config.FeedbackPrefix
  And task reverted from [x] to [ ]
  And task re-executed with feedback visible in file
  And on second pass: approve to complete
```

### AC4: Skip at gate

```gherkin
Scenario: Skip at gate
  Given gate triggered on [GATE] task
  And mock stdin returns "s"
  When runner processes skip
  Then task remains [x] (already marked done by review)
  And runner proceeds to next task or completes
  And no error returned
```

### AC5: Checkpoint gate fires every N tasks

```gherkin
Scenario: Checkpoint gate fires every N
  Given --every 2 configured (GatesCheckpoint=2)
  And 3+ tasks in sprint-tasks.md (no [GATE] tags)
  And mock stdin returns "a" for all gates
  When runner.Execute runs all tasks
  Then checkpoint fires after task 2 (completedTasks % 2 == 0)
  And gate text contains "(checkpoint every 2)"
  And non-checkpoint tasks (1, 3) do NOT trigger gate
```

### AC6: Emergency gate upgrade — skip at exhaustion

```gherkin
Scenario: Emergency gate for execute exhaustion
  Given gates_enabled = true
  And execute always fails (no commit) max_iterations times
  And mock EmergencyGatePromptFn returns skip
  When emergency gate fires
  Then shows emergency prompt (not regular exit with ErrMaxRetries)
  And skip via SkipTask marks task [x]
  And runner proceeds to next task or completes

Scenario: Emergency gate for review exhaustion
  Given gates_enabled = true
  And review always returns non-clean max_review_iterations times
  And mock EmergencyGatePromptFn returns skip
  When emergency gate fires
  Then shows emergency prompt (not regular exit with ErrMaxReviewCycles)
  And skip advances to next task
```

### AC7: Gates disabled — no prompts fire

```gherkin
Scenario: Gates disabled — no prompts
  Given gates_enabled = false
  And sprint-tasks.md with [GATE] tagged tasks
  And GatesCheckpoint > 0
  When runner.Execute runs
  Then no gate prompts fire (GatePromptFn call count == 0)
  And no emergency gate prompts fire (EmergencyGatePromptFn call count == 0)
  And all tasks executed normally
```

### AC8: Combined GATE + checkpoint — single prompt

```gherkin
Scenario: Combined GATE + checkpoint
  Given GatesCheckpoint=2
  And task 2 has [GATE] tag
  And mock stdin returns "a"
  When task 2 completes clean review
  Then ONE gate prompt fires (not two)
  And gate text contains both "[GATE]" and "(checkpoint every 2)"
```

### AC9: gates package integration — real Prompt with mock I/O

```gherkin
Scenario: Real gates.Prompt with approve
  Given Gate{TaskText, Reader: strings.NewReader("a\n"), Writer: bytes.Buffer}
  When gates.Prompt(ctx, gate) called
  Then returns GateDecision{Action: approve}
  And output buffer contains "HUMAN GATE" and task text

Scenario: Real gates.Prompt with emergency flag
  Given Gate{TaskText, Reader, Writer, Emergency: true}
  When gates.Prompt(ctx, gate) called
  Then output contains "EMERGENCY GATE"
  And [a]pprove NOT in options
  And "s" input returns ActionSkip
```

## Tasks / Subtasks

- [x] Task 1: Create gates package integration tests (AC: #9)
  - [x] 1.1: File: `gates/gates_integration_test.go` with `//go:build integration` tag
  - [x] 1.2: `TestPrompt_Integration_AllActions` — table-driven: real `gates.Prompt` with `strings.NewReader` for input, `bytes.Buffer` for output. Cases: "a\n"→approve, "s\n"→skip, "q\n"→quit, "r\nfeedback\n\n"→retry+feedback. Verify decision.Action, output contains "HUMAN GATE", output contains task text
  - [x] 1.3: `TestPrompt_Integration_EmergencyActions` — table-driven: Gate{Emergency: true}. Cases: "s\n"→skip, "r\nfix\n\n"→retry, "q\n"→quit. Verify output contains "EMERGENCY GATE", output does NOT contain "[a]pprove". Input "a\ns\n" → verify "a" rejected (output contains "not available"), then "s" accepted
  - [x] 1.4: `TestPrompt_Integration_InvalidThenValid` — "x\na\n". Verify: output contains "Unknown option: x", decision is approve
  - [x] 1.5: `TestPrompt_Integration_RetryMultilineFeedback` — "r\nline1\nline2\n\n". Verify: decision.Feedback == "line1 line2"
- [x] Task 2: Create runner-level gate integration tests — normal gate scenarios (AC: #1, #2, #3, #4, #8)
  - [x] 2.1: File: `runner/runner_gates_integration_test.go` with `//go:build integration` tag
  - [x] 2.2: `TestRunner_Execute_GateIntegration_Approve` — GatesEnabled=true, single [GATE] task, MockClaude succeeds, cleanReviewFn, GatePromptFn returns approve. Verify: no error, task [x], GatePromptFn called once, taskText contains "[GATE]"
  - [x] 2.3: `TestRunner_Execute_GateIntegration_Quit` — GatePromptFn returns quit. Verify: error wraps GateDecision, errors.As extracts ActionQuit, error contains "runner: gate:", task [x] preserved
  - [x] 2.4: `TestRunner_Execute_GateIntegration_RetryWithFeedback` — GatePromptFn: 1st call returns retry+feedback, 2nd call returns approve. MockClaude with 2 execute steps. MockGitClient with 2 SHA pairs. Verify: feedback line in sprint-tasks.md contains FeedbackPrefix, task re-executed (2 executions), final task [x]
  - [x] 2.5: `TestRunner_Execute_GateIntegration_Skip` — GatePromptFn returns skip. Verify: no error, task [x], runner completes
  - [x] 2.6: `TestRunner_Execute_GateIntegration_CombinedGateCheckpoint` — GatesCheckpoint=1, single [GATE] task. Verify: GatePromptFn called once (not twice), taskText contains both "[GATE]" and "(checkpoint every 1)"
- [x] Task 3: Create runner-level gate integration tests — checkpoint scenarios (AC: #5, #8)
  - [x] 3.1: `TestRunner_Execute_GateIntegration_CheckpointEveryN` — GatesCheckpoint=2, 3 tasks (no [GATE]), cleanReviewFn. MockClaude with 3 execute steps, MockGitClient with 3 SHA pairs. Verify: GatePromptFn called once (after task 2 only), taskText for task 2 contains "(checkpoint every 2)", no gate after task 1 or 3
  - [x] 3.2: Use `trackingGatePrompt` with enhanced tracking — record ALL taskTexts received (not just last). Add `texts []string` field to trackingGatePrompt if not already present
- [x] Task 4: Create runner-level gate integration tests — emergency gate scenarios (AC: #6)
  - [x] 4.1: `TestRunner_Execute_GateIntegration_EmergencyExecuteSkip` — GatesEnabled=true, MaxIterations=2, MockClaude always returns same SHA (no commit → needsRetry). EmergencyGatePromptFn returns skip. Verify: EmergencyGatePromptFn called with text containing "execute attempts exhausted" and "2/2", task marked [x] via SkipTask, no ErrMaxRetries error, runner completes
  - [x] 4.2: `TestRunner_Execute_GateIntegration_EmergencyReviewSkip` — GatesEnabled=true, MaxReviewIterations=2, ReviewFn always returns non-clean. EmergencyGatePromptFn returns skip. Verify: EmergencyGatePromptFn called with text containing "review cycles exhausted" and "2/2", task marked [x], no ErrMaxReviewCycles error
  - [x] 4.3: For emergency tests, use separate trackingEmergencyGatePrompt struct to track calls independently from normal GatePromptFn
- [x] Task 5: Create runner-level gate integration test — gates disabled (AC: #7)
  - [x] 5.1: `TestRunner_Execute_GateIntegration_GatesDisabled` — GatesEnabled=false, GatesCheckpoint=2, task with [GATE], MockClaude exhausts MaxIterations. Verify: GatePromptFn call count == 0, EmergencyGatePromptFn call count == 0, returns ErrMaxRetries (original behavior), not emergency gate
- [x] Task 6: Multi-task end-to-end integration test (AC: #1, #5, #8)
  - [x] 6.1: `TestRunner_Execute_GateIntegration_MultiTaskScenario` — 4 tasks: task 1 plain, task 2 [GATE], task 3 plain, task 4 plain. GatesCheckpoint=2. Mock for all 4 task iterations. Verify: gate fires after task 2 (both [GATE] + checkpoint), no gate after task 1, checkpoint gate after task 4 (checkpoint only). Assert GatePromptFn call count == 2, task texts correct for each call
  - [x] 6.2: Create `progressiveReviewFn(tasksPath)` helper — reads file, finds first `[ ]` task, marks it `[x]`, writes back. Do NOT use `reviewAndMarkDoneFn` (it writes `allDoneTasks` which marks everything done at once, breaking multi-task progression)
  - [x] 6.3: Multi-task fixture: `"- [ ] Task one\n- [ ] Task two [GATE]\n- [ ] Task three\n- [ ] Task four\n"`
- [x] Task 7: Verify test infrastructure and run all tests
  - [x] 7.1: Verify `trackingGatePrompt` records all texts (not just last) — if not, add `texts []string` field
  - [x] 7.2: Ensure all integration tests use `//go:build integration` tag
  - [x] 7.3: Run unit tests: verify no regressions in existing gates and runner tests
  - [x] 7.4: Run integration tests with `-tags integration` flag
  - [x] 7.5: Verify coverage of all gate actions (approve, skip, quit, retry) across normal, checkpoint, and emergency contexts

## Dev Notes

### Architecture Constraints

- **Build tag:** `//go:build integration` — integration tests run separately from unit tests
- **Dependency direction preserved:** test files can import both `gates` and `runner` packages
- **No production code changes** — this story only adds test files
- **Mock I/O for gates:** `strings.NewReader` for stdin, `bytes.Buffer` for stdout — no real terminal I/O
- **MockClaude for runner:** existing `testutil.SetupMockClaude` + scenario steps pattern
- **No `os.Exit` in tests** — runner returns errors, tests assert on error values

### What Already Exists (DO NOT RECREATE)

| Component | Location | Status |
|-----------|----------|--------|
| `gates.Prompt` | `gates/gates.go:35-120` | Full implementation with feedback collection + emergency styling (5.1, 5.3, 5.5) |
| `gates.Gate` struct | `gates/gates.go:17-21` | TaskText + Reader + Writer + Emergency fields |
| `gates_test.go` unit tests | `gates/gates_test.go` | ~14 test functions after 5.1+5.3+5.5: 6 base + 4 feedback + 4 emergency |
| `Runner.Execute` | `runner/runner.go` | Full gate check + checkpoint + emergency gate logic (5.2-5.5) |
| `GatePromptFn` field | `Runner` struct | Injectable normal gate function |
| `EmergencyGatePromptFn` field | `Runner` struct | Injectable emergency gate function (5.5) |
| `InjectFeedback` | `runner/runner.go:227-256` | Feedback injection into sprint-tasks.md |
| `RevertTask` | `runner/runner.go:261-281` | Task [x]→[ ] revert |
| `SkipTask` | `runner/runner.go` | Task [ ]→[x] skip (5.5) |
| `trackingGatePrompt` | `runner/test_helpers_test.go:166-177` | Records count + last taskText |
| `setupGateTest` | `runner/test_helpers_test.go:182-217` | Standard gate test setup |
| `setupRunnerIntegration` | `runner/test_helpers_test.go:272-287` | Integration test setup |
| `headCommitPairs` | `runner/test_helpers_test.go:79-85` | SHA pair generator |
| `testConfig` | `runner/test_helpers_test.go:153-161` | Standard test config |
| `writeTasksFile` | `runner/test_helpers_test.go:68-75` | File setup helper |
| `cleanReviewFn` | `runner/test_helpers_test.go:52-54` | Clean review mock |
| `reviewAndMarkDoneFn` | `runner/test_helpers_test.go:314-323` | Review that marks task done |
| `noopSleepFn` | `runner/test_helpers_test.go:246` | No-op sleep |
| `noopResumeExtractFn` | `runner/test_helpers_test.go:249-251` | No-op resume extract |
| `config.ActionApprove/Skip/Quit/Retry` | `config/constants.go` | Action constants |
| `config.GateDecision` | `config/errors.go:28-37` | Action + Feedback, implements error |
| `config.FeedbackPrefix` | `config/constants.go:11` | `"> USER FEEDBACK:"` |
| `config.ErrMaxRetries` | `config/errors.go:11` | Execute exhaustion sentinel |
| `config.ErrMaxReviewCycles` | `config/errors.go:12` | Review exhaustion sentinel |
| `testutil.SetupMockClaude` | `internal/testutil/` | Mock Claude with scenario steps |
| `testutil.MockGitClient` | `internal/testutil/` | Mock git with configurable responses |
| `testutil.ReadInvocationArgs` | `internal/testutil/` | Read subprocess args for verification |

### Stories 5.2-5.5 Creates (Prerequisites — Assumed Complete)

| Component | Location | Expected |
|-----------|----------|----------|
| `GatePromptFunc` type | `runner/runner.go` | `func(ctx, taskText) (*GateDecision, error)` |
| `GatePromptFn` field | Runner struct | Normal gate prompt function |
| `EmergencyGatePromptFn` field | Runner struct | Emergency gate prompt function (5.5) |
| Gate check block | Runner.Execute | After clean review, with checkpoint logic (5.4) |
| Emergency gate at execute exhaustion | Runner.Execute | Conditional on GatesEnabled (5.5) |
| Emergency gate at review exhaustion | Runner.Execute | Conditional on GatesEnabled (5.5) |
| `completedTasks` counter | Runner.Execute | For checkpoint gates (5.4) |
| `InjectFeedback` | runner package | Feedback injection (5.3) |
| `RevertTask` | runner package | Task revert on retry (5.3) |
| `SkipTask` | runner package | Task skip at emergency gate (5.5) |
| `Emergency` field on Gate struct | gates package | Emergency styling flag (5.5) |

### Test File Organization

| File | Package | Purpose |
|------|---------|---------|
| `gates/gates_integration_test.go` | gates | Real Prompt function with mock I/O |
| `runner/runner_gates_integration_test.go` | runner_test | Runner.Execute with mock gate functions |

### Multi-Task Test Pattern

For checkpoint and multi-task scenarios, the test needs:
1. **MockClaude with N steps** — one execute step per task iteration
2. **MockGitClient with N SHA pairs** — different SHAs per iteration (commit detected)
3. **ReviewFn that progresses tasks** — marks current task [x] so scanner finds next [ ]
4. **trackingGatePrompt recording ALL calls** — not just count but taskTexts for verification

Example multi-task setup:
```go
fourTasks := "- [ ] Task one\n- [ ] Task two [GATE]\n- [ ] Task three\n- [ ] Task four\n"

scenario := testutil.Scenario{
    Name: "multi-task-gates",
    Steps: []testutil.ScenarioStep{
        {Type: "execute", ExitCode: 0, SessionID: "exec-1"},
        {Type: "execute", ExitCode: 0, SessionID: "exec-2"},
        {Type: "execute", ExitCode: 0, SessionID: "exec-3"},
        {Type: "execute", ExitCode: 0, SessionID: "exec-4"},
    },
}

mock := &testutil.MockGitClient{
    HeadCommits: headCommitPairs(
        [2]string{"a1", "b1"},
        [2]string{"a2", "b2"},
        [2]string{"a3", "b3"},
        [2]string{"a4", "b4"},
    ),
}
```

The ReviewFn must read current tasks, find first [ ] task, mark it [x], and write back. This simulates Claude completing the task during review. Do NOT use `reviewAndMarkDoneFn` — it writes `allDoneTasks` (all done at once). Create a `progressiveReviewFn(tasksPath)` that uses `config.TaskOpenRegex`/`strings.Replace` to mark one task at a time.

### Emergency Gate Test Pattern

Emergency gates fire when execute retries or review cycles are exhausted. Test setup:
- **Execute exhaustion:** MockGitClient returns SAME SHA (no commit) for all calls → needsRetry=true. After MaxIterations, emergency fires
- **Review exhaustion:** ReviewFn returns non-clean for MaxReviewIterations cycles. Execute succeeds (commit detected) but review finds issues
- **EmergencyGatePromptFn** — separate tracking struct from GatePromptFn for independent call-count assertions

```go
trackingEmergency := &trackingGatePrompt{
    decision: &config.GateDecision{Action: config.ActionSkip},
}
r.EmergencyGatePromptFn = trackingEmergency.fn
```

### trackingGatePrompt Enhancement

Current `trackingGatePrompt` only records last `taskText`. For multi-task and checkpoint tests, need to record ALL texts:

```go
type trackingGatePrompt struct {
    count    int
    taskText string               // last taskText received
    texts    []string             // ALL taskTexts received (for multi-call verification)
    decision *config.GateDecision
    err      error
}

func (tg *trackingGatePrompt) fn(_ context.Context, taskText string) (*config.GateDecision, error) {
    tg.count++
    tg.taskText = taskText
    tg.texts = append(tg.texts, taskText)
    return tg.decision, tg.err
}
```

If `texts` field is not yet present, add it. If already present (from Story 5.4), reuse.

### Retry Integration Test: Two-Phase Mock

The retry test needs GatePromptFn that returns different decisions on different calls:

```go
// sequenceGatePrompt returns different decisions per call.
type sequenceGatePrompt struct {
    calls     int
    decisions []*config.GateDecision
    texts     []string
}

func (sg *sequenceGatePrompt) fn(_ context.Context, taskText string) (*config.GateDecision, error) {
    idx := sg.calls
    sg.calls++
    sg.texts = append(sg.texts, taskText)
    if idx < len(sg.decisions) {
        return sg.decisions[idx], nil
    }
    return &config.GateDecision{Action: config.ActionApprove}, nil // fallback
}
```

### Naming Conventions

- Integration test file: `gates_integration_test.go`, `runner_gates_integration_test.go`
- Test function names: `TestPrompt_Integration_<Scenario>` (Prompt = function name, Integration = scenario prefix), `TestRunner_Execute_GateIntegration_<Scenario>` (Runner = type, Execute = method)
- Helper structs: `sequenceGatePrompt` (multi-decision), `trackingGatePrompt` (single-decision tracking), `progressiveReviewFn` (marks one task [x] per call)
- Error prefixes: unchanged — `"runner: gate:"`, `"runner: emergency gate:"`

### What This Story Does NOT Include

- Production code changes — only test files
- Performance or load testing
- Real stdin testing (all tests use mock I/O)
- Manual smoke test (documented separately in testdata/manual_smoke_checklist.md)
- New test infrastructure shared across packages (all helpers local to test files)

### Project Structure Notes

- **New file:** `gates/gates_integration_test.go` — gates.Prompt integration tests with real function
- **New file:** `runner/runner_gates_integration_test.go` — Runner.Execute gate integration tests
- **Potentially modified:** `runner/test_helpers_test.go` — add `texts []string` to trackingGatePrompt, add sequenceGatePrompt helper, add multi-task fixture constants
- **No production code modified** — this is a test-only story

### References

- [Source: docs/epics/epic-5-human-gates-control-stories.md#Story 5.6]
- [Source: gates/gates.go:35-120] — Prompt function (test target)
- [Source: runner/runner.go] — Runner.Execute with gate check, checkpoint, emergency gates
- [Source: runner/test_helpers_test.go:166-177] — trackingGatePrompt (enhance)
- [Source: runner/test_helpers_test.go:182-217] — setupGateTest helper
- [Source: runner/test_helpers_test.go:272-287] — setupRunnerIntegration pattern
- [Source: runner/runner_integration_test.go] — Existing integration test patterns
- [Source: runner/runner_review_integration_test.go] — Review integration test patterns
- [Source: config/errors.go:28-37] — GateDecision type
- [Source: config/errors.go:11-12] — ErrMaxRetries, ErrMaxReviewCycles sentinels
- [Source: internal/testutil/] — MockClaude, MockGitClient infrastructure

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created `gates/gates_integration_test.go` with 5 test functions (AC9): AllActions (table-driven, 4 cases), EmergencyActions (table-driven, 3 cases), EmergencyApproveRejected, InvalidThenValid, RetryMultilineFeedback. All use real `gates.Prompt` with `strings.NewReader`/`bytes.Buffer` mock I/O.
- Created `runner/runner_gates_integration_test.go` with 10 test functions covering all ACs:
  - AC1: Approve at [GATE] task
  - AC2: Quit preserves state, errors.As extracts GateDecision
  - AC3: Retry with feedback — sequenceGatePrompt for 2-phase mock, feedback injection verified
  - AC4: Skip at gate — task remains [x]
  - AC5: Checkpoint every N — fires after task 2 only (of 3)
  - AC6: Emergency execute skip + emergency review skip — separate tracking for normal/emergency
  - AC7: Gates disabled — zero gate calls, ErrMaxRetries returned
  - AC8: Combined [GATE] + checkpoint — single prompt with both markers
  - Multi-task scenario: 4 tasks, 2 gate fires at correct positions
- No production code changes — test-only story
- All existing helpers reused: `trackingGatePrompt` (already had `taskTexts`), `sequenceGatePrompt`, `progressiveReviewFn`, `reviewAndMarkDoneFn`, `setupRunnerIntegration`, `headCommitPairs`, fixtures `gateOpenTask`, `threeOpenTasks`, `fourOpenTasksWithGate`
- Task 4.3: Used separate `trackingGatePrompt` instances for normal vs emergency tracking (not a new type — same struct, different instances)
- All unit tests pass: `go test ./...` — zero regressions
- All integration tests pass: `go test -tags integration ./...` — 15 new test functions total

### File List

- `gates/gates_integration_test.go` — NEW: gates.Prompt integration tests (5 test functions)
- `runner/runner_gates_integration_test.go` — NEW: Runner.Execute gate integration tests (10 test functions)
- `docs/sprint-artifacts/5-6-gates-integration-test.md` — NEW: story file created by workflow
- `docs/sprint-artifacts/sprint-status.yaml` — MODIFIED: 5-6 status → in-progress → review
