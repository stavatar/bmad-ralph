# Epic 5: Human Gates & Control — Stories

**Scope:** FR20, FR21, FR22, FR25
**Stories:** 6
**Release milestone:** v0.2 (post-initial release)

**Context (из Epics Structure Plan):**
- FR23/FR24 (emergency gates) реализованы как minimal stop в Epic 3 (Stories 3.9, 3.10)
- В Epic 5 emergency stop апгрейдится до interactive gate с approve/retry/feedback/skip/quit
- `gates` package: `gates.Prompt(ctx, gate)` — interactive stdin + `fatih/color`
- `gates` и `session` НЕ зависят друг от друга (Architecture dependency direction)
- Feedback injection: `> USER FEEDBACK: ...` в sprint-tasks.md (config.FeedbackPrefix constant)
- `[GATE]` tag scanning already implemented in Story 3.2 (scanner)

---

### Story 5.1: Basic Gate Prompt

**User Story:**
Как разработчик, я хочу интерактивный gate prompt с цветными опциями approve/skip/quit, чтобы контролировать выполнение на gate-точках.

**Acceptance Criteria:**

```gherkin
Scenario: Gate prompt displays with color
  Given gate triggered for task "TASK-1 — Setup project structure"
  When gates.Prompt(ctx, gate) is called
  Then displays colored prompt via fatih/color:
    🚦 HUMAN GATE: TASK-1 — Setup project structure
       [a]pprove  [r]etry with feedback  [s]kip  [q]uit
    > _
  And waits for user input via io.Reader (not raw fmt.Scan — injectable for testing)

Scenario: Approve action
  Given gate prompt displayed
  When user enters "a"
  Then returns GateDecision{Action: Approve}
  And runner proceeds to next task

Scenario: Skip action
  Given gate prompt displayed
  When user enters "s"
  Then returns GateDecision{Action: Skip}
  And current task marked [x] (skipped)
  And runner proceeds to next task

Scenario: Quit action
  Given gate prompt displayed
  When user enters "q"
  Then returns GateDecision{Action: Quit}
  And runner exits with code 2 (user quit — matches exit code table in Story 1.13)
  And sprint-tasks.md state preserved (incomplete tasks remain [ ])

Scenario: Invalid input re-prompts
  Given gate prompt displayed
  When user enters invalid input (e.g., "x")
  Then displays error message
  And re-displays prompt
  And does not advance or quit

Scenario: Context cancellation during prompt
  Given gate prompt waiting for input
  When ctx is cancelled (Ctrl+C)
  Then returns context.Canceled error
  And runner handles graceful shutdown
```

**Technical Notes:**
- Architecture: `gates/gates.go` — `gates.Prompt(ctx, gate)` entry point
- Structural pattern: one main exported function per package
- `GateDecision` custom error type: `Action` enum (Approve, Retry, Skip, Quit) + optional `Feedback` string
- `fatih/color` for colored output, `io.Reader` + `io.Writer` for testable I/O (not raw `fmt.Scan`/`os.Stdout`)
- `gates.Prompt` accepts `io.Reader` + `io.Writer` parameters (or struct with these fields) for dependency injection
- Gates package does NOT depend on runner or session (Architecture dependency direction)
- Retry action defined here but feedback input handled in Story 5.3

**Prerequisites:** Story 1.5 (fatih/color dependency), Story 1.6 (config constants: FeedbackPrefix, GateTag)

---

### Story 5.2: Gate Detection in Runner

**User Story:**
Как пользователь `ralph run --gates`, я хочу чтобы система останавливалась на задачах с `[GATE]` тегом для моего одобрения перед продолжением.

**Acceptance Criteria:**

```gherkin
Scenario: Gates enabled via --gates flag
  Given ralph run invoked with --gates flag (FR20)
  When runner starts
  Then gates_enabled = true in config
  And runner will check for gate tags during execution

Scenario: Gates disabled by default
  Given ralph run invoked without --gates flag
  When runner encounters [GATE] tagged task
  Then skips gate prompt
  And executes task normally without stopping

Scenario: Stop at GATE-tagged task
  Given gates_enabled = true
  And current task has [GATE] tag (detected by scanner Story 3.2)
  When task completes review (marked [x])
  Then runner calls gates.Prompt AFTER task completion (FR21)
  And waits for developer input before next task

Scenario: Gate prompt shows after task completion, not before
  Given gates_enabled and task with [GATE]
  When runner reaches gate
  Then task is already executed and reviewed
  And gate prompt shows AFTER [x] marking
  And developer approves the completed work, not pre-approves

Scenario: Approve continues to next task
  Given gate prompt returns Approve
  When runner processes decision
  Then proceeds to next task in sprint-tasks.md

Scenario: Quit at gate preserves state
  Given gate prompt returns Quit
  When runner processes decision
  Then exits with code 2 (user quit — matches exit code table in Story 1.13)
  And all completed tasks remain [x]
  And incomplete tasks remain [ ]
  And re-run continues from first [ ] (FR12)
```

**Technical Notes:**
- Architecture: runner calls `gates.Prompt` when `cfg.GatesEnabled && task.HasGateTag`
- Scanner (Story 3.2) already detects `[GATE]` tags — this story wires detection to gates.Prompt
- Gate triggers AFTER task completion (execute + review + [x]) — developer approves finished work
- `--gates` flag wired in `cmd/ralph/run.go` (Story 1.3 Cobra structure)
- Skip at gate: runner marks [x] and continues (task already completed by review)

**Prerequisites:** Story 5.1 (gate prompt), Story 3.2 (scanner with GateTag), Story 4.7 (execute→review loop)

---

### Story 5.3: Retry with Feedback

**User Story:**
Как разработчик, я хочу на gate-точке выбрать retry с обратной связью, чтобы AI учёл мои комментарии при повторной реализации задачи.

**Acceptance Criteria:**

```gherkin
Scenario: Retry action prompts for feedback
  Given gate prompt displayed
  When user enters "r"
  Then system prompts for feedback text input
  And user types multi-line feedback (Enter twice to submit)

Scenario: Feedback injected into sprint-tasks.md
  Given user provided feedback "Need to add validation for email field"
  When feedback injection runs
  Then sprint-tasks.md updated with indented line under current task:
    > USER FEEDBACK: Need to add validation for email field
  And uses config.FeedbackPrefix constant (FR22)

Scenario: Execute sees feedback on retry
  Given feedback injected into sprint-tasks.md
  When next execute session launches (fresh session)
  Then Claude reads sprint-tasks.md and sees feedback line
  And addresses feedback in implementation (self-directing model)

Scenario: Retry resets task for re-execution
  Given retry with feedback selected
  When runner processes decision
  Then current task [x] reverted to [ ] in sprint-tasks.md
  And execute_attempts reset to 0
  And review_cycles reset to 0
  And fresh execute cycle starts for this task

Scenario: Ralph writes feedback programmatically
  Given feedback text from user
  When ralph injects feedback
  Then ralph (not Claude) writes the feedback line via os.WriteFile
  And feedback line is indented under the task
  And existing feedback lines preserved (append, not overwrite)

Scenario: GateDecision includes feedback
  Given user chose retry with feedback
  When gates.Prompt returns
  Then GateDecision{Action: Retry, Feedback: "user text"}
  And runner uses Feedback field for injection
```

**Technical Notes:**
- Architecture: "Ralph программно добавляет feedback в sprint-tasks.md"
- Feedback format: `> USER FEEDBACK: <text>` — config.FeedbackPrefix constant
- Ralph writes this line (not Claude) — one of the few places Ralph modifies sprint-tasks.md content
- This does NOT violate Mutation Asymmetry: feedback is content injection, not task status change
- Execute prompt (Story 3.1) already handles self-directing model — Claude reads sprint-tasks.md
- Retry reverts [x] → [ ]: this is the ONLY place Ralph changes task status markers (exception to Mutation Asymmetry, documented)

**Prerequisites:** Story 5.1 (gate prompt with retry action), Story 5.2 (gate detection in runner)

---

### Story 5.4: Checkpoint Gates

**User Story:**
Как разработчик, я хочу периодические checkpoint gates каждые N задач (`--gates --every N`), чтобы регулярно проверять прогресс AI даже без `[GATE]` разметки.

**Acceptance Criteria:**

```gherkin
Scenario: Checkpoint every N tasks
  Given ralph run --gates --every 5
  When 5th task completes (marked [x])
  Then checkpoint gate prompt fires
  And prompt indicates "checkpoint every 5"
  And same options: approve/retry/skip/quit (FR25)

Scenario: Checkpoint counter counts completed tasks
  Given --every 3 configured
  And tasks 1,2 completed, task 3 skipped via [s]kip
  When counting for checkpoint
  Then skipped tasks count toward checkpoint (3 tasks processed)
  And checkpoint fires after task 3

Scenario: Checkpoint independent of [GATE] tags
  Given --every 5 configured
  And task 3 has [GATE] tag
  When task 3 completes
  Then [GATE] gate fires at task 3 (Story 5.2)
  And checkpoint counter continues (3/5)
  And next checkpoint at task 5 (not reset by GATE)

Scenario: Combined GATE + checkpoint — single prompt
  Given --every 5 configured
  And task 5 has [GATE] tag
  When task 5 completes
  Then ONE combined gate prompt (not two)
  And prompt indicates both: "[GATE] + checkpoint every 5"

Scenario: Config file support
  Given config.yaml has gates_checkpoint: 5
  And no --every flag on CLI
  When runner starts
  Then checkpoint every 5 tasks active
  And CLI --every overrides config value

Scenario: --every 0 disables checkpoints
  Given --gates --every 0
  When tasks complete
  Then no checkpoint gates fire
  And only [GATE] tagged tasks trigger gates
```

**Technical Notes:**
- Architecture: "Checkpoint gates: каждые N [x] задач (считает completed, не attempts)"
- CLI: `--gates --every N` — `--every` only valid with `--gates`
- Config: `gates_checkpoint` in YAML, default 0 (off)
- Counter: increment on each task completion (including skip), reset never (cumulative)
- Combined gate: if task has [GATE] AND hits checkpoint, merge into single prompt
- `--every 1` ≈ gate after every task

**Prerequisites:** Story 5.2 (gate detection in runner), Story 1.13 (Cobra flag wiring for --every)

---

### Story 5.5: Emergency Gate Upgrade

**User Story:**
Как разработчик, я хочу чтобы emergency stops (исчерпание попыток execute/review) стали интерактивными gates с опциями retry/feedback/skip/quit, чтобы я мог решить как поступить вместо автоматического прекращения.

**Acceptance Criteria:**

```gherkin
Scenario: Emergency gate replaces stop when gates enabled
  Given gates_enabled = true
  And execute_attempts reaches max_iterations (FR23)
  When emergency triggers
  Then shows interactive emergency gate (not just exit)
  And prompt includes: task info, attempts count, failure context
  And options: [r]etry with feedback, [s]kip task, [q]uit

Scenario: Emergency gate for review cycles when gates enabled
  Given gates_enabled = true
  And review_cycles reaches max_review_iterations (FR24)
  When emergency triggers
  Then shows interactive emergency gate
  And prompt includes: task info, review cycles count, remaining findings
  And options: [r]etry with feedback, [s]kip task, [q]uit

Scenario: Non-interactive stop preserved when gates disabled
  Given gates_enabled = false
  And execute_attempts reaches max_iterations
  When emergency triggers
  Then original behavior: exit code 1 + informative message (Epic 3)
  And no interactive prompt

Scenario: Retry at emergency gate resets counters
  Given emergency gate for execute_attempts
  When developer chooses [r]etry with feedback
  Then execute_attempts resets to 0
  And feedback injected (Story 5.3)
  And fresh execute cycle starts

Scenario: Skip at emergency gate advances to next task
  Given emergency gate displayed
  When developer chooses [s]kip
  Then current task marked [x] (skipped)
  And runner proceeds to next task
  And counters reset for next task
```

**Technical Notes:**
- This story modifies runner loop logic from Stories 3.9/3.10 — adds conditional: `if gatesEnabled { gates.Prompt(emergency) } else { return ErrMaxRetries }`
- Emergency gate prompt has different styling: 🚨 instead of 🚦
- Approve option NOT available at emergency gate (nothing to approve — task failed)
- Feedback from retry goes through same injection as Story 5.3
- Emergency gates use same `gates.Prompt` function — different `GateType` enum value

**Prerequisites:** Story 5.3 (retry with feedback), Story 3.9 (execute emergency stop), Story 3.10 (review emergency stop)

---

### Story 5.6: Gates Integration Test

**User Story:**
Как разработчик, я хочу комплексный integration test gates system, покрывающий все gate types и actions, чтобы гарантировать корректность интерактивного контроля.

**Acceptance Criteria:**

```gherkin
Scenario: Approve at GATE tag
  Given sprint-tasks.md with [GATE] tagged task
  And gates_enabled = true
  And mock stdin returns "a"
  When runner.Run executes
  Then gate prompt fires after task completion
  And approve continues to next task

Scenario: Quit at gate
  Given gate triggered
  And mock stdin returns "q"
  When runner processes quit
  Then exits with code 2 (user quit)
  And state preserved in sprint-tasks.md

Scenario: Retry with feedback
  Given gate triggered
  And mock stdin returns "r" then "fix validation"
  When runner processes retry
  Then feedback injected into sprint-tasks.md
  And task re-executed with feedback visible

Scenario: Skip at gate
  Given gate triggered
  And mock stdin returns "s"
  When runner processes skip
  Then task remains [x] and runner continues

Scenario: Checkpoint gate fires every N
  Given --every 3 and 5 tasks
  And mock stdin returns "a" for all gates
  When runner.Run executes
  Then checkpoint fires after task 3
  And checkpoint fires after task 5 (if > 5 tasks, after 6...)

Scenario: Emergency gate upgrade
  Given gates_enabled = true
  And execute fails max_iterations times
  And mock stdin returns "s" (skip)
  When emergency gate fires
  Then shows emergency prompt (not regular exit)
  And skip advances to next task

Scenario: Gates disabled — no prompts
  Given gates_enabled = false
  And sprint-tasks.md with [GATE] tagged tasks
  When runner.Run executes
  Then no gate prompts fire
  And all tasks executed normally
```

**Technical Notes:**
- Mock stdin: `io.Reader` injection into gates package for testing (not os.Stdin directly)
- gates package should accept `io.Reader` + `io.Writer` for testability
- Build tag: `//go:build integration`
- Test file: `gates/gates_integration_test.go` + runner-level gate scenarios in `runner/runner_gates_integration_test.go`
- Combined GATE + checkpoint scenario covers edge case from Story 5.4

**Prerequisites:** Story 5.1-5.5 (all gate stories), Story 1.11 (MockClaude), Story 3.4 (MockGitClient)

---

### Epic 5 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 5.1 | Basic Gate Prompt | FR21,FR22 | 2 | 6 |
| 5.2 | Gate Detection in Runner | FR20,FR21 | 1 | 6 |
| 5.3 | Retry with Feedback | FR22 | 1 | 6 |
| 5.4 | Checkpoint Gates | FR25 | 1 | 6 |
| 5.5 | Emergency Gate Upgrade | FR23,FR24 | 1 | 5 |
| 5.6 | Gates Integration Test | — | 2 | 7 |
| | **Total** | **FR20-FR22,FR25 + FR23/FR24 upgrade** | | **~36** |

**FR Coverage:** FR20 (5.2), FR21 (5.1, 5.2), FR22 (5.1, 5.3), FR23 upgrade (5.5), FR24 upgrade (5.5), FR25 (5.4)

**Architecture Sections Referenced:** Gates package (gates.go), Subprocess Patterns, CLI UX & Output (fatih/color, io.Reader/io.Writer), Dependency Direction (gates independent of session)

**Dependency Graph:**
```
1.5, 1.6 ──→ 5.1 ──→ 5.2 ──→ 5.3 ──→ 5.5
3.2 ────────→ 5.2     │              ↗
4.7 ────────→ 5.2     ├──→ 5.4    3.9, 3.10
1.3 ────────→ 5.4     │
                       └──→ 5.5 ──→ 5.6
                       5.1-5.5 ──→ 5.6
                       1.11, 3.4 ──→ 5.6
```
Note: 5.3 и 5.4 partially parallel-capable (оба зависят от 5.2, но 5.5 зависит от 5.3)

---
