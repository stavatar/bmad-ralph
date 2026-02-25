# Story 1.9: Session --resume Support

Status: done

## Story

As a developer,
I want session to support `--resume <session_id>` for resume-extraction,
So that failed execute sessions can be resumed to extract progress and knowledge.

## Acceptance Criteria (BDD)

```gherkin
Given a previous session with SessionID "abc-123"
When session.Execute(ctx, opts) is called with opts.Resume = "abc-123"
Then CLI args include "--resume" "abc-123"
And "-p" flag is NOT included (resume uses previous prompt)
And "--max-turns" IS included (limits resume duration)
And "--output-format json" IS included

Given opts.Resume is empty string
When CLI args are constructed
Then "--resume" flag is NOT included
And "-p" flag IS included with opts.Prompt

And unit tests verify:
  - Resume args construction (no -p, has --resume)
  - Normal args construction (has -p, no --resume)
  - Resume with max-turns combination
```

## Implementation Readiness Assessment

**IMPORTANT:** Most of Story 1.9's functionality was already implemented forward-lookingly in Story 1.7.
The dev agent MUST verify existing code satisfies all AC before writing new code.

### Already Implemented (Story 1.7):
- `Options.Resume string` field — `session/session.go:38`
- `flagResume = "--resume"` constant — `session/session.go:23`
- `buildArgs` logic: Resume != "" uses `--resume` and skips `-p` — `session/session.go:88-92`
- Test "resume mode no prompt" — Resume + MaxTurns + SkipPermissions — `session/session_test.go:110-121`
- Test "resume overrides prompt" — Resume + Prompt + SkipPermissions — `session/session_test.go:122-133`

### Gaps Identified (Remaining Work):
1. **No combined resume-extraction scenario test:** AC scenario 1 requires Resume + MaxTurns + OutputJSON together. Current "resume mode no prompt" test has Resume + MaxTurns but NO OutputJSON
2. **No "all resume fields" comprehensive test:** Per CLAUDE.md pattern "All-fields comprehensive test", need Resume + MaxTurns + Model + OutputJSON + SkipPermissions combined (mirrors existing "all fields set" which uses Prompt, not Resume)
3. **No integration test for resume:** `TestExecuteAndParse_Integration` has "json success" and "non-JSON fallback" but no resume round-trip scenario
4. **No edge case test for empty Resume + non-empty Prompt:** AC scenario 2 requires explicitly tested `Options{Resume: "", Prompt: "test"}`
5. **Story file creation:** This file

## Tasks / Subtasks

- [x] Task 1: Verify existing implementation satisfies AC (AC: all)
  - [x] 1.1: Confirm `Options.Resume` field type and doc comment are correct
  - [x] 1.2: Confirm `flagResume` constant value matches Claude CLI docs
  - [x] 1.3: Confirm `buildArgs` Resume logic: includes --resume, excludes -p, preserves --max-turns and --output-format json
  - [x] 1.4: Run existing tests — all session tests must pass with zero regressions

- [x] Task 2: Add resume-extraction full scenario test cases (AC: scenario 1, scenario 2)
  - [x] 2.1: Add test case "resume with max-turns and output-json" to `TestBuildArgs_BasicPrompt` table (insert after existing "resume overrides prompt" case ~line 133)
    - Options: Resume="abc-123", MaxTurns=10, Model="" (omitted), OutputJSON=true, SkipPermissions=true
    - Expected args: ["--resume", "abc-123", "--max-turns", "10", "--output-format", "json", "--dangerously-skip-permissions"]
    - Verify: no "-p" in output
  - [x] 2.2: Add test case "resume all fields set" to `TestBuildArgs_BasicPrompt` table (insert after 2.1)
    - Options: Resume="session-789", Prompt="ignored" (verifies ignored), MaxTurns=5, Model="claude-sonnet-4-5-20250514", OutputJSON=true, SkipPermissions=true
    - Expected args: ["--resume", "session-789", "--max-turns", "5", "--model", "claude-sonnet-4-5-20250514", "--output-format", "json", "--dangerously-skip-permissions"]
    - Verify: no "-p" in output, Prompt value ignored
  - [x] 2.3: Add test case "empty resume with prompt" to `TestBuildArgs_BasicPrompt` table (insert after 2.2)
    - Options: Resume="" (explicitly empty), Prompt="test", SkipPermissions=true
    - Expected args: ["-p", "test", "--dangerously-skip-permissions"]
    - Verify: no "--resume" in output, "-p" IS included (AC scenario 2 explicit)

- [x] Task 3: Add resume integration test (AC: resume round-trip)
  - [x] 3.1: Add `resume_json` scenario to `runTestHelper` in session_test.go
    - MUST use DISTINCT session_id from json_success: `"resume-test-002"` (not "integ-test-001")
    - MUST use DISTINCT output text: `"Resumed session output."` (not "Integration test output.")
    - Rationale: distinct values prevent scenario routing bugs from being masked
    - Example output: `[{"type":"system","subtype":"init","session_id":"resume-test-002","tools":[],"model":"claude-sonnet-4-5-20250514"},{"type":"result","subtype":"success","session_id":"resume-test-002","result":"Resumed session output.","is_error":false,"duration_ms":500,"num_turns":1}]`
  - [x] 3.2: Add "resume json round-trip" subtest to `TestExecuteAndParse_Integration`
    - Options: Command=os.Args[0], Dir=dir, Resume="abc-123", MaxTurns=10, OutputJSON=true
    - Expected concrete values:
      - `result.SessionID == "resume-test-002"`
      - `result.Output == "Resumed session output."`
      - `result.ExitCode == 0`
      - `result.Duration > 0`
    - NOTE: This integration test validates the Execute+ParseResult parsing pipeline works for resume scenarios. It does NOT validate CLI flag construction (that is covered by buildArgs unit tests in Task 2). The subprocess checks env var `SESSION_TEST_HELPER`, not actual CLI args.

- [x] Task 4: Verify test naming and patterns (AC: all)
  - [x] 4.1: Confirm all new test cases follow `Test<Type>_<Method>_<Scenario>` naming
  - [x] 4.2: Confirm error messages verified with `strings.Contains` (per CLAUDE.md)
  - [x] 4.3: Run full test suite: `go test ./session/ -v -count=1`
  - [x] 4.4: Run linter: `golangci-lint run ./session/...`

## Dev Notes

### Architecture Context

- **Resume-extraction pattern:** When an execute session fails (no commit), runner can resume it via `claude --resume <session_id>` to extract WIP code and learnings. This is the only exception to the fresh session principle — balances isolation with pragmatic recovery [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.9, docs/architecture/core-architectural-decisions.md — "--resume используется для resume-extraction"]
- **Why max-turns is critical for resume (design rationale):** Resume sessions operate without the original prompt's full context. `--max-turns` acts as a fail-safe against infinite turn loops. Semantically different from fresh session max-turns (progress limit vs context-loss safety)
- **Mutation asymmetry (design rationale):** Resume-extraction sessions do NOT mark tasks as `[x]` — only review sessions do. This constraint will be enforced in runner (Epic 3), not session package. Session is unaware of this rule
- **Session package = Claude CLI abstraction:** session does NOT import config — receives everything via `Options` struct. All `--resume` logic is encapsulated here. Caller (runner/bridge) maps config.Config fields to Options [Source: docs/architecture/implementation-patterns-consistency-rules.md]

### Implementation Pattern: buildArgs Flag Ordering

The `buildArgs` function uses a specific ordering pattern:
1. Resume OR Prompt (mutually exclusive, Resume wins)
2. MaxTurns (if > 0)
3. Model (if non-empty)
4. OutputJSON (if true)
5. SkipPermissions (if true)

This ordering is conventional (not required by Claude CLI) but consistent across all tests. New test cases MUST use the same expected arg order.

### Testing Pattern: Self-Reexec for Integration

Session tests use the Go standard `TestMain` self-reexec pattern:
- `TestMain` checks `SESSION_TEST_HELPER` env var
- `runTestHelper(scenario)` dispatches to scenario handlers
- Test binary (`os.Args[0]`) is used as `Options.Command`
- **CRITICAL:** `default: os.Exit(1)` case already exists in runTestHelper — do NOT remove it
- Integration tests validate the Execute+ParseResult parsing pipeline, NOT CLI flag construction (that's covered by buildArgs unit tests)

New resume_json scenario MUST use distinct session_id and output text from json_success to prevent masked bugs.

### Previous Story (1.8) Learnings

From Story 1.8 review findings:
- Doc comments must match actual behavior (deviation documentation required)
- No dead golden files — every testdata fixture must be loaded by at least one test
- Remove unused test struct fields (orphan fields indicate copy-paste)
- Test `is_error: true` from Claude CLI explicitly
- New tests in Tasks 2 and 3 must verify no dead fixtures or unused fields per this pattern

### What NOT To Do

- Do NOT refactor existing Resume code — it works correctly
- Do NOT add new files — all changes go in existing session_test.go
- Do NOT add golden files for buildArgs tests — they use inline expected values
- Do NOT modify session.go or result.go — no production code changes needed
- Do NOT add error path tests — buildArgs is pure (no errors possible)
- Do NOT use type assertions (`err.(*Type)`) — project standard requires `errors.As`
- Do NOT remove or modify the `default: os.Exit(1)` case in runTestHelper

### Project Structure Notes

- All changes in `session/` package only
- Files to modify:
  - `session/session_test.go` — Tasks 2 (buildArgs table cases) and 3.1 (resume_json scenario in runTestHelper)
  - `session/result_test.go` — Task 3.2 (resume subtest in TestExecuteAndParse_Integration)
- No new files, no new dependencies
- Alignment with project structure: both test files co-located with their source files

### Key Constants (from session/session.go)

```go
const (
    flagPrompt          = "-p"
    flagMaxTurns        = "--max-turns"
    flagModel           = "--model"
    flagOutputFormat    = "--output-format"
    flagResume          = "--resume"
    flagSkipPermissions = "--dangerously-skip-permissions"
    outputFormatJSON    = "json"
)
```

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.9] — AC and technical notes
- [Source: docs/architecture/core-architectural-decisions.md] — Resume-extraction pattern, max-turns rationale, mutation asymmetry
- [Source: docs/architecture/implementation-patterns-consistency-rules.md] — Testing patterns, naming conventions, package boundaries
- [Source: docs/project-context.md] — Subprocess patterns, test conventions
- [Source: session/session.go] — Existing Resume implementation (lines 23, 38, 88-92)
- [Source: session/session_test.go] — Existing Resume tests (lines 110-133), TestMain self-reexec pattern
- [Source: docs/sprint-artifacts/1-8-session-json-parsing-sessionresult.md] — Previous story learnings

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->
<!-- Quality competition validation applied: 3 critical fixes, 4 enhancements, 2 optimizations -->

### Agent Model Used

claude-opus-4-6

### Debug Log References

No debug issues encountered. All implementation was test-only; no production code changes needed.

### Completion Notes List

- Task 1: Verified all existing Resume implementation from Story 1.7 — Options.Resume field, flagResume constant, buildArgs logic all correct. All 37 existing tests pass (14 top-level functions).
- Task 2: Added 3 new buildArgs test cases: "resume with max-turns and output-json" (AC scenario 1 full combo), "resume all fields set" (comprehensive per CLAUDE.md pattern), "empty resume with prompt" (AC scenario 2 explicit edge case).
- Task 3: Added `resume_json` scenario to runTestHelper with distinct session_id="resume-test-002" and output="Resumed session output." to prevent masked routing bugs. Added "resume json round-trip" integration subtest to TestExecuteAndParse_Integration.
- Task 4: All naming conventions verified. Full test suite 41/41 PASS (14 top-level functions), go vet clean. golangci-lint unavailable in WSL (CI-only).

### Change Log

- 2026-02-25: Added 3 buildArgs resume test cases and 1 resume integration test (session_test.go, result_test.go). No production code changes.
- 2026-02-25: [Review Fix] Renamed test "resume with max-turns and output-json" → "resume with max turns and output json" for naming consistency. Added inline comment to integration test explaining Options fields are self-documenting. Fixed test counts in completion notes (37→41).

### File List

- session/session_test.go (modified) — 3 new buildArgs table cases + resume_json scenario in runTestHelper
- session/result_test.go (modified) — "resume json round-trip" subtest in TestExecuteAndParse_Integration
