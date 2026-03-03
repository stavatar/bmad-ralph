# Story 6.8: Final Integration Test

Status: review

## Story

As a developer,
I want a final end-to-end integration test of the entire product,
so that I can confirm all 6 epics work together — including knowledge pipeline with human-gated distillation and multi-file output.

## Acceptance Criteria

```gherkin
Scenario: FINAL — full end-to-end flow with auto-knowledge (C1)
  Given scenario JSON covering full flow:
    bridge -> execute (commit) -> review (findings) -> execute fix (commit) -> review (clean)
    -> budget check -> knowledge post-validated -> Serena hint injected
  And MockClaude + MockGitClient + mock Serena detection + mock DistillFn
  And sprint-tasks.md from bridge golden file
  When runner.Run executes with all features
  Then bridge output feeds runner
  And execute sessions launch with knowledge context (__LEARNINGS_CONTENT__ injected)
  And review finds and verifies findings
  And fix cycle produces clean review
  And [x] marked + review-findings cleared
  And Claude writes lessons to LEARNINGS.md (on findings review)
  And Go post-validates lessons after review session (snapshot-diff model)
  And budget check runs after clean review (no distill — under limit)
  And Serena prompt hint present when detected
  And all 6 epics work together

Scenario: FINAL — gates + knowledge + emergency
  Given gates_enabled = true, --every 2
  And scenario with 3 tasks: task1 (clean), task2 (emergency->skip), task3 (clean)
  And mock stdin for gate actions
  When runner.Run executes
  Then checkpoint gate fires after task 2
  And emergency gate fires for task 2 (max retries)
  And skip advances to task 3
  And knowledge written throughout (LEARNINGS.md has entries from both resume-extraction and review)

Scenario: FINAL — auto-distillation multi-file output with human gate
  Given LEARNINGS.md starts with 140 lines
  And review writes ~20 lines of lessons (total >150 soft threshold)
  And MonotonicTaskCounter - LastDistillTask >= 5 (cooldown met)
  And mock DistillFn returns compressed content with 2 categories
  When clean review completes and budget check runs
  Then auto-distillation triggered (DistillFn called)
  And LEARNINGS.md replaced with compressed output
  And ralph-{category}.md files created in .ralph/rules/
  And ralph-index.md generated
  And log contains "Auto-distilled LEARNINGS.md"
  And next execute session gets distilled knowledge context from ralph-*.md files

Scenario: FINAL — distillation failure triggers human gate (v6: human only)
  Given auto-distillation fails (mock returns error)
  When failure detected
  Then human gate presented with error description + options: skip / retry 1 / retry 5
  And if skip chosen: all backups restored, LEARNINGS.md unchanged
  And runner continues normally

Scenario: FINAL — JIT citation validation filters stale
  Given LEARNINGS.md has 5 entries, 2 citing deleted files
  When execute prompt assembled
  Then ValidateLearnings filters 2 stale entries (os.Stat only, M9)
  And only 3 valid entries injected into prompt (__LEARNINGS_CONTENT__)
  And HasLearnings = true, self-review step present (H3)

Scenario: FINAL — resume + knowledge flow (M1)
  Given scenario: execute (no commit) -> resume-extraction -> retry -> execute (commit)
  When runner.Run executes
  Then resume-extraction runs with --resume + -p (compatible, M1)
  And Claude writes knowledge to LEARNINGS.md during resume
  And Go post-validates after resume session
  And LEARNINGS.md accumulates from resume source
  And retry execute includes knowledge context from previous failure

Scenario: FINAL — Serena MCP detection fallback (C3)
  Given .claude/settings.json has no Serena config
  When runner.Run executes
  Then CodeIndexerDetector.Available() returns false
  And no Serena prompt hint injected
  And runner completes full flow without Serena
  And no errors from Serena absence

Scenario: FINAL — [needs-formatting] tag and fix cycle
  Given LEARNINGS.md has 2 entries with [needs-formatting] tag
  And LEARNINGS.md exceeds 150 lines (triggers distillation)
  And mock DistillFn returns output with [needs-formatting] entries fixed
  When auto-distillation runs
  Then [needs-formatting] entries present in distillation input
  And output has properly formatted entries (tags removed, format fixed)
  And ValidateDistillation criterion #3 passes (all [needs-formatting] handled)

Scenario: FINAL — crash recovery at startup via intent file (M7)
  Given `.ralph/distill-intent.json` exists from interrupted distillation
  And LEARNINGS.md.bak and .ralph/distill-state.json.bak exist
  When runner starts
  Then intent file detected → recovery triggered
  And Phase 2 completed or rolled back from .bak (based on phase field)
  And intent file + .pending files cleaned up
  And log warning: "Recovered from interrupted distillation"
  And runner proceeds normally

Scenario: FINAL — cross-language scope hints (M12)
  Given multiple project stacks: Go, Python, JS/TS, Java, mixed
  When distillation generates scope hints for each stack
  Then scope hints match language conventions (M4)
  And categories appropriate for each language
  And citations valid for each file type
```

## Tasks / Subtasks

- [x] Task 1: Create test file and common test infrastructure (AC: all)
  - [x]1.1 Create `runner/runner_final_integration_test.go` with `//go:build integration` tag
  - [x]1.2 Package `runner_test` (external test package — same as other integration tests)
  - [x]1.3 Shared helper: `setupFinalIntegration(t, opts)` — creates tmpDir, writes tasks, LEARNINGS.md, config, mock git, returns Runner
  - [x]1.4 `finalIntegrationOpts` struct: tasksContent, learningsContent, learningsLines, scenario, gitMock, gates, checkpoint, serenaAvailable, distillFn, knowledgeWriter, distillStatePath
  - [x]1.5 Reuse existing: `setupRunnerIntegration`, `testConfig`, `trackingKnowledgeWriter`, `trackingDistillFunc`, `trackingGatePrompt`, `headCommitPairs`

- [x]Task 2: FINAL — full end-to-end flow with auto-knowledge (AC: #1)
  - [x]2.1 `TestRunner_Execute_FinalIntegration_FullFlowWithKnowledge`
  - [x]2.2 Setup: LEARNINGS.md with 50 lines (under threshold), sprint-tasks.md with 1 task
  - [x]2.3 Scenario: execute (commit) → review (findings) → execute fix (commit) → review (clean)
  - [x]2.4 MockClaude with 4 steps: exec1, review1 (findings), exec2, review2 (clean)
  - [x]2.5 Mock Serena: CodeIndexer returns Available=true, PromptHint="If Serena MCP..."
  - [x]2.6 trackingKnowledgeWriter: verify ValidateNewLessons called (once for execute, once for review)
  - [x]2.7 trackingDistillFunc: set on Runner (non-nil DistillFn enables BudgetCheck path at runner.go:788), verify count==0 (budget under limit → no distillation)
  - [x]2.8 Pre-populate distill-state.json (LoadDistillState needs it for MonotonicTaskCounter increment)
  - [x]2.9 Verify prompt contains `__LEARNINGS_CONTENT__` replacement (knowledge injected)
  - [x]2.10 Verify Serena hint in prompt via ReadInvocationArgs
  - [x]2.11 Verify task marked [x] after clean review
  - [x]2.12 Verify Execute() returns nil (success)

- [x]Task 3: FINAL — gates + knowledge + emergency (AC: #2)
  - [x]3.1 `TestRunner_Execute_FinalIntegration_GatesKnowledgeEmergency`
  - [x]3.2 Setup: 3 tasks, gates_enabled=true, checkpoint=2
  - [x]3.3 Task1: clean review → passes
  - [x]3.4 Task2: all execute attempts fail → emergency gate → skip
  - [x]3.5 Task3: clean review → checkpoint fires (completedTasks=2 mod 2=0) → approve
  - [x]3.6 Mock gate: sequenceGatePrompt with [skip, approve] actions
  - [x]3.7 trackingKnowledgeWriter: verify ValidateNewLessons called for each session
  - [x]3.8 Verify Execute() returns nil (all tasks processed)

- [x]Task 4: FINAL — auto-distillation multi-file output (AC: #3)
  - [x]4.1 `TestRunner_Execute_FinalIntegration_AutoDistillation`
  - [x]4.2 Setup: LEARNINGS.md with 140 lines, 1 task
  - [x]4.3 Review writes ~20 lines to LEARNINGS.md (mock writes via file side effect)
  - [x]4.4 DistillState: MonotonicTaskCounter=10, LastDistillTask=3 (cooldown 10-3=7 >= 5)
  - [x]4.5 trackingDistillFunc: verify called once (auto-distillation triggered)
  - [x]4.6 Verify distillState.LastDistillTask updated after success
  - [x]4.7 Verify distillState.MonotonicTaskCounter incremented
  - [x]4.8 Write pre-populated distill-state.json before test

- [x]Task 5: FINAL — distillation failure triggers human gate (AC: #4)
  - [x]5.1 `TestRunner_Execute_FinalIntegration_DistillFailureGate`
  - [x]5.2 Setup: LEARNINGS.md with 160 lines (over threshold), cooldown met
  - [x]5.3 trackingDistillFunc: returns error on first call
  - [x]5.4 Mock gate: returns skip action
  - [x]5.5 Verify human gate called with distillation error text
  - [x]5.6 Verify runner continues normally (Execute returns nil)
  - [x]5.7 Verify LEARNINGS.md unchanged (skip = no modifications)

- [x]Task 6: FINAL — JIT citation validation filters stale (AC: #5)
  - [x]6.1 `TestRunner_Execute_FinalIntegration_JITCitationValidation`
  - [x]6.2 Setup: LEARNINGS.md with 5 entries, 2 citing files that don't exist
  - [x]6.3 Create 3 cited files in tmpDir, leave 2 missing
  - [x]6.4 Execute → verify prompt via ReadInvocationArgs
  - [x]6.5 Verify only 3 valid entries in `__LEARNINGS_CONTENT__` (stale filtered by ValidateLearnings)
  - [x]6.6 Verify HasLearnings = true (self-review instructions present in prompt)

- [x]Task 7: FINAL — resume + knowledge flow (AC: #6)
  - [x]7.1 `TestRunner_Execute_FinalIntegration_ResumeKnowledge`
  - [x]7.2 Setup: 1 task, execute fails (no commit) → resume-extraction → retry → execute succeeds
  - [x]7.3 Scenario: exec1 (fail), resume-extract (session), exec2 (commit), review (clean)
  - [x]7.4 Mock ResumeExtractFn: tracking function that records calls
  - [x]7.5 Verify resume-extraction called with session ID from failed execute
  - [x]7.6 trackingKnowledgeWriter: verify ValidateNewLessons called for resume-extraction source
  - [x]7.7 Verify retry execute succeeds

- [x]Task 8: FINAL — Serena MCP detection fallback (AC: #7)
  - [x]8.1 `TestRunner_Execute_FinalIntegration_SerenaFallback`
  - [x]8.2 Setup: no .claude/settings.json, no .mcp.json, CodeIndexer=NoOpCodeIndexerDetector
  - [x]8.3 Execute → verify no Serena hint in prompt (ReadInvocationArgs)
  - [x]8.4 Verify Execute() returns nil (no errors from Serena absence)

- [x]Task 9: FINAL — [needs-formatting] tag and fix cycle (AC: #8)
  - [x]9.1 `TestRunner_Execute_FinalIntegration_NeedsFormattingCycle`
  - [x]9.2 Setup: LEARNINGS.md with 160 lines, 2 entries with [needs-formatting]
  - [x]9.3 Cooldown met, DistillFn triggers
  - [x]9.4 Verify LEARNINGS.md content (before distillation) has [needs-formatting] tags
  - [x]9.5 trackingDistillFunc: capture state, verify called

- [x]Task 10: FINAL — crash recovery at startup (AC: #9)
  - [x]10.1 `TestRunner_Execute_FinalIntegration_CrashRecovery`
  - [x]10.2 Setup: create .ralph/distill-intent.json with phase="write" and file list
  - [x]10.3 Create corresponding .pending files in tmpDir
  - [x]10.4 Execute Runner → verify RecoverDistillation runs at startup
  - [x]10.5 Verify: intent file deleted, .pending files committed (renamed to targets)
  - [x]10.6 Verify: stderr contains "Recovered from interrupted distillation"
  - [x]10.7 Verify: runner proceeds normally after recovery

- [x]Task 11: FINAL — cross-language scope hints (AC: #10)
  - [x]11.1 `TestDetectProjectScope_CrossLanguage` (unit test, not integration — deterministic)
  - [x]11.2 Table-driven: Go project (.go files → **/*.go), Python (.py → **/*.py), JS/TS (.ts/.tsx → **/*.ts, **/*.tsx), Java (.java → **/*.java)
  - [x]11.3 Mixed stack: create .go + .py files → both globs in output
  - [x]11.4 Empty project → "No language-specific patterns detected"
  - [x]11.5 Verify scope hints format string content

## Dev Notes

### Architecture & Design Decisions

- **Single test file:** `runner/runner_final_integration_test.go` with `//go:build integration` tag. Follows existing pattern from Story 3.11, 4.8, 5.6.
- **No build tag for unit tests:** Cross-language scope tests (Task 11) are unit tests in `runner/knowledge_distill_test.go` — no build tag needed.
- **Reuse existing infrastructure:** `setupRunnerIntegration`, `testConfig`, `trackingKnowledgeWriter`, `trackingDistillFunc`, `trackingGatePrompt`, `sequenceGatePrompt` — all from test_helpers_test.go.
- **MockClaude pattern:** scenario-based JSON via `testutil.SetupMockClaude`. Test binary self-reexec for Claude process simulation.
- **Mock DistillFn:** `trackingDistillFunc` from test_helpers_test.go. Records calls, returns configurable error sequence.
- **No new mocks needed:** All existing mocks sufficient. trackingKnowledgeWriter tracks ValidateNewLessons calls, trackingDistillFunc tracks DistillFn calls.
- **Crash recovery test:** Creates intent file + .pending files before Runner.Execute(). Recovery happens at startup. Verifies file cleanup and log output.
- **Capture stderr:** Use `os.Pipe()` or redirect `os.Stderr` temporarily to verify log messages (e.g., "Recovered from interrupted distillation", "Auto-distilled LEARNINGS.md").
- **--always-extract NOT tested here:** Deferred to Growth (per epic notes).

### Test Interaction Matrix

| Test | Knowledge | Distill | Gates | Serena | Recovery | Resume |
|------|-----------|---------|-------|--------|----------|--------|
| #1 FullFlow | ValidateNewLessons ×2 | no (under limit) | no | yes | no | no |
| #2 GatesEmergency | ValidateNewLessons | no | yes (emergency+checkpoint) | no | no | no |
| #3 AutoDistill | ValidateNewLessons | yes (triggered) | no | no | no | no |
| #4 DistillFailGate | ValidateNewLessons | yes (fails→gate) | yes (distill gate) | no | no | no |
| #5 JITCitation | ValidateLearnings | no | no | no | no | no |
| #6 ResumeKnowledge | ValidateNewLessons | no | no | no | no | yes |
| #7 SerenaFallback | no | no | no | no (absent) | no | no |
| #8 NeedsFormatting | ValidateNewLessons | yes | no | no | no | no |
| #9 CrashRecovery | no | no | no | no | yes | no |
| #10 CrossLanguage | no | scope hints | no | no | no | no |

### File Layout

| File | Purpose |
|------|---------|
| `runner/runner_final_integration_test.go` | NEW: all final integration tests (10 test functions) |
| `runner/knowledge_distill_test.go` | MODIFY: add cross-language scope hint tests (Task 11, unit tests) |
| `runner/test_helpers_test.go` | MODIFY: add `setupFinalIntegration` helper if needed (or reuse `setupRunnerIntegration`) |

### Existing Test Infrastructure References

**setupRunnerIntegration (test_helpers_test.go:342-357):**
```go
func setupRunnerIntegration(t, tmpDir, tasksContent, scenario, git) (*Runner, stateDir)
// Defaults: cleanReviewFn, noopResumeExtractFn, noopSleepFn, NoOpKnowledgeWriter
```

**trackingKnowledgeWriter (test_helpers_test.go:316-336):**
- `writeProgressCount`, `writeProgressData`
- `validateLessonsCount`, `validateLessonsData`
- Injectable errors via `writeProgressErr`, `validateLessonsErr`

**trackingDistillFunc (test_helpers_test.go:415-431):**
- `count`, `states`, `errs` (error sequence)
- `fn(ctx, state)` method

**trackingGatePrompt / sequenceGatePrompt (test_helpers_test.go):**
- `taskTexts []string` for capture
- Sequential actions for multi-gate tests

**headCommitPairs (test_helpers_test.go):**
- Creates paired commit hashes for execute+review cycles

**testutil.SetupMockClaude (internal/testutil/):**
- Scenario-based mock with ordered steps
- Returns mock binary path and state directory

### LEARNINGS.md Test Content

For tests requiring specific line counts:
```go
// Generate N-line LEARNINGS.md
func generateLearnings(t *testing.T, tmpDir string, lineCount int) {
    var sb strings.Builder
    sb.WriteString("# LEARNINGS.md\n\n")
    for i := 0; i < lineCount-2; i++ {
        sb.WriteString(fmt.Sprintf("## testing: pattern-%d [review, test.go:%d]\nFact %d.\n\n", i, i, i))
    }
    os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(sb.String()), 0644)
}
```

### Error Wrapping Convention

Tests don't wrap errors — they use `t.Fatalf`/`t.Errorf` for assertion failures.

### Dependency Direction

```
runner/runner_final_integration_test.go → runner (Execute, RunConfig, etc.)
runner/runner_final_integration_test.go → config (Config, GateDecision, etc.)
runner/runner_final_integration_test.go → internal/testutil (MockGitClient, SetupMockClaude)
```

No new production code. Test-only file.

### Testing Standards

- `//go:build integration` tag — not run by default `go test ./...`, needs `-tags integration`
- Table-driven where applicable (cross-language scope tests)
- Each test function independent — own t.TempDir, own mocks, own scenario
- Assertions: `strings.Contains` for prompt content, `errors.Is` for sentinels, count checks for mock calls
- Verify intermediate state: capture LEARNINGS.md content at key points, verify file existence
- No time-dependent tests — all mocks return immediately
- `t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)` for tests that need MockClaude file side effects

### Code Review Learnings

- Integration tests: verify ALL error message layers (sentinel + content) per test-assertions-base.md
- Symmetric assertion depth: all tests that verify knowledge injection must check both prompt content and call counts
- Table-driven return value tests: assert ALL struct fields (not just primary)
- No vacuous tests: every assertion must be falsifiable — create conditions that would fail if code regresses

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.8 (lines 930-1053)]
- [Source: runner/runner_integration_test.go — existing integration test pattern]
- [Source: runner/runner_gates_integration_test.go — gate integration test pattern]
- [Source: runner/runner_review_integration_test.go — review integration test pattern]
- [Source: runner/test_helpers_test.go:316-431 — trackingKnowledgeWriter, trackingDistillFunc, setupRunnerIntegration]
- [Source: runner/knowledge_distill.go — AutoDistill, DetectProjectScope, ValidateDistillation]
- [Source: runner/knowledge_state.go — RecoverDistillation, LoadDistillState, DistillState]
- [Source: runner/knowledge_write.go — ValidateNewLessons, BudgetCheck]
- [Source: runner/runner.go:450-757 — Execute() full flow]

## Dev Agent Record

### Context Reference
- Story 6.8: Final Integration Test
- Epic 6: Knowledge Management & Polish

### Agent Model Used
claude-opus-4-6

### Debug Log References
- Assertion fix: `"self-review"` (lowercase) → `"Self-Review"` (matching template execute.md case)

### Completion Notes List
- Tasks 1-11: All implemented and passing
- Task 1: Reused existing `setupRunnerIntegration` + helpers (no new setupFinalIntegration needed)
- Tasks 2-10: 9 integration tests in `runner/runner_final_integration_test.go`
- Task 11: 1 unit test (table-driven, 4 cases) in `runner/knowledge_distill_test.go`
- All 9 integration tests PASS, 1 unit test (4 sub-cases) PASS
- Full regression: all packages PASS (bridge, cmd/ralph, config, gates, testutil, runner, session)
- mockCodeIndexer struct added for Serena testing (Available/PromptHint configurable)

### File List
- `runner/runner_final_integration_test.go` — NEW: 9 integration tests (Tasks 2-10)
- `runner/knowledge_distill_test.go` — MODIFIED: added TestDetectProjectScope_CrossLanguage (Task 11)
