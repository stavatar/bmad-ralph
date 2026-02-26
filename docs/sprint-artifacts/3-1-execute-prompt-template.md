# Story 3.1: Execute Prompt Template

Status: done

## Story

As a developer,
I want the execute prompt to contain 999-rules guardrails, ATDD instructions, red-green cycle, and self-directing behavior,
so that Claude Code autonomously and safely implements tasks from sprint-tasks.md.

## Acceptance Criteria

1. **Execute prompt assembled with all required sections:**
   Given config with project root and sprint-tasks path, and optional review-findings.md exists, when prompt is assembled via text/template + strings.Replace, then prompt contains: (1) 999-rules guardrail section (FR36), (2) ATDD instruction: every AC must have test (FR37), (3) zero-skip policy: never skip/xfail tests (FR38), (4) red-green cycle instruction, (5) self-directing instruction: read sprint-tasks.md, take first `- [ ]` (FR11), (6) instruction: MUST NOT modify task status markers (Mutation Asymmetry), (7) instruction: commit on green tests only (FR8), (8) sprint-tasks-format.md content injected via strings.Replace (not template).

2. **Execute prompt includes review-findings when present:**
   Given review-findings.md exists with CONFIRMED findings, when prompt is assembled, then review-findings content is injected via strings.Replace, and prompt contains instruction to fix findings before continuing.

3. **Execute prompt without review-findings:**
   Given review-findings.md does not exist, when prompt is assembled, then prompt does not contain findings section, and prompt instructs Claude to proceed with next task.

4. **Golden file snapshots match baseline:**
   Given execute prompt template in runner/prompts/execute.md, when assembled with test fixture data, then output matches golden files in runner/testdata/ (one per scenario: with-findings, without-findings), and golden files are updateable via `go test -update`.

5. **User content cannot break template:**
   Given Stage 2 replacement content contains Go template syntax `{{.Dangerous}}` (e.g., from format contract or future LEARNINGS.md), when prompt is assembled (stage 2: strings.Replace), then template syntax in user content is preserved literally, and no template execution error occurs.

## Tasks / Subtasks

- [x] Task 1: Add `HasFindings` bool to `config.TemplateData` (AC: #2, #3)
  - [x] 1.1 Add `HasFindings bool` field to `TemplateData` in `config/prompt.go` alongside other Stage 1 booleans (`SerenaEnabled`, `GatesEnabled`, `HasExistingTasks`). Add doc comment: `// execute findings mode`
  - [x] 1.2 Verify existing tests still pass — `HasFindings` is zero-valued `false` by default, so no existing behavior changes

- [x] Task 2: Expand `runner/prompts/execute.md` to full prompt template (AC: #1, #2, #3)
  - [x] 2.1 Replace stub content with comprehensive execute prompt template containing all sections listed in AC #1
  - [x] 2.2 **999-rules guardrail section (FR36):** Hard rules Claude MUST obey even if review-findings suggest otherwise. Include: do not delete files outside project, do not run destructive commands, do not modify config files not related to the task, do not skip tests, do not disable linters
  - [x] 2.3 **ATDD instruction (FR37):** Every acceptance criterion must have a corresponding test. Write tests BEFORE implementation (red-green-refactor)
  - [x] 2.4 **Zero-skip policy (FR38):** NEVER skip, xfail, or comment out tests. Failing tests must be fixed or escalated, never suppressed
  - [x] 2.5 **Red-green cycle:** Write failing test first (red), implement until test passes (green), refactor if needed. Commit only when all tests pass
  - [x] 2.6 **Self-directing instruction (FR11):** Read `sprint-tasks.md` in the project root. Find the FIRST task with `- [ ]` marker (top-to-bottom scan). Implement that task. Do NOT skip ahead to other tasks
  - [x] 2.7 **Mutation Asymmetry:** MUST NOT modify task status markers (`- [ ]` → `- [x]`) in sprint-tasks.md. Only review sessions change task status. This is inviolable
  - [x] 2.8 **Commit on green only (FR8):** Commit code ONLY after all tests pass. Never commit with failing tests
  - [x] 2.9 **Format contract injection:** Include `__FORMAT_CONTRACT__` placeholder for Stage 2 injection of `config.SprintTasksFormat()` content
  - [x] 2.10 **Conditional findings section:** Use `{{- if .HasFindings}}...{{- end}}` (Stage 1 conditional) with `__FINDINGS_CONTENT__` placeholder (Stage 2 injection). Include instruction to fix findings BEFORE continuing with task
  - [x] 2.11 **No-findings path:** When findings section is absent, include instruction to proceed with next task from sprint-tasks.md
  - [x] 2.12 Use `{{- if -}}` trim markers on all conditionals to avoid blank lines (per bridge.md pattern and `.claude/rules/go-testing-patterns.md` Template Testing section)

- [x] Task 3: Create `runner/prompt_test.go` with golden file tests (AC: #4, #5)
  - [x] 3.1 Create `runner/prompt_test.go` with package `runner` (internal test). Import: `os`, `path/filepath`, `strings`, `testing`, `config`
  - [x] 3.2 Add `-update` flag: `var update = flag.Bool("update", false, "update golden files")`
  - [x] 3.3 Add `goldenTest(t *testing.T, name, got string)` helper following exact pattern from `bridge/prompt_test.go`: read golden file, if `-update` write and return, else compare
  - [x] 3.4 `TestPrompt_Execute_WithFindings` — assemble execute template with `HasFindings: true` and `__FINDINGS_CONTENT__` replacement containing test findings text. Verify golden file match. Verify prompt contains all 7 required sections from AC #1. Verify findings instruction present
  - [x] 3.5 `TestPrompt_Execute_WithoutFindings` — assemble execute template with `HasFindings: false` and no findings replacement. Verify golden file match. Verify prompt does NOT contain findings section. Verify proceed instruction present
  - [x] 3.6 `TestPrompt_Execute_FormatContract` — assemble with `__FORMAT_CONTRACT__` → `config.SprintTasksFormat()`. Verify output contains "Sprint Tasks Format Specification" (title from sprint-tasks-format.md)
  - [x] 3.7 `TestPrompt_Execute_Injection` — assemble with `__FORMAT_CONTRACT__` replaced by content containing `{{.Dangerous}}` Go template syntax. Verify no error and `{{.Dangerous}}` preserved literally in output (AC #5)
  - [x] 3.8 Run `go test -update` to generate golden files, then run without `-update` to verify match

- [x] Task 4: Update `runner/runner.go` RunOnce to pass correct TemplateData and replacements (AC: #1)
  - [x] 4.1 Update `RunOnce` AssemblePrompt call: set `TemplateData{GatesEnabled: rc.Cfg.GatesEnabled}` (already used in stub) plus format contract replacement `"__FORMAT_CONTRACT__": config.SprintTasksFormat()`
  - [x] 4.2 NOTE: Full findings loading (reading review-findings.md, setting `HasFindings`, passing `__FINDINGS_CONTENT__`) is wired in Story 3.5 (runner loop), NOT here. RunOnce is walking skeleton — it keeps simple wiring
  - [x] 4.3 Remove `__TASK_CONTENT__` replacement from RunOnce — execute prompt is self-directing (FR11), Claude reads sprint-tasks.md directly. Instead ensure sprint-tasks.md path is accessible in the working directory (cmd.Dir = ProjectRoot)
  - [x] 4.4 Update `runner/runner_integration_test.go` assertion at line ~91: replace `strings.Contains(promptValue, "Implement hello world")` with assertion on new execute template content (e.g., `"sprint-tasks.md"` or `"Sprint Tasks Format Specification"`). The old assertion checks for task content injection which no longer happens with self-directing prompt

- [x] Task 5: Run all tests and verify (AC: all)
  - [x] 5.1 Run `go test ./runner/...` — all tests pass
  - [x] 5.2 Run `go test ./config/...` — all tests pass (TemplateData change backward-compatible)
  - [x] 5.3 Run `go vet ./...` — no issues
  - [x] 5.4 Verify golden files exist: `runner/testdata/TestPrompt_Execute_WithFindings.golden`, `runner/testdata/TestPrompt_Execute_WithoutFindings.golden`

## Prerequisites

- Story 1.10 (prompt assembly utility — `config.AssemblePrompt`, `config.TemplateData`)
- Story 2.1 (shared sprint-tasks format contract — `config/shared/sprint-tasks-format.md`, `config.SprintTasksFormat()`)

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Primary file to create/modify:** `runner/prompts/execute.md` — this is the main deliverable. The current file is a 9-line stub. Replace entirely with comprehensive prompt.

**Two-stage assembly (CRITICAL — understand before writing):**
- **Stage 1:** `text/template` processes `{{if .HasFindings}}`, `{{if .GatesEnabled}}`, `{{if .SerenaEnabled}}` — only BOOL fields from `config.TemplateData`. NO string fields in templates!
- **Stage 2:** `strings.ReplaceAll` injects user content: `__FORMAT_CONTRACT__` → format spec, `__FINDINGS_CONTENT__` → review findings (if present). User content may contain `{{` — this is safe because Stage 2 runs AFTER template execution.
- **Reference implementation:** `bridge/prompts/bridge.md` — uses exact same pattern. Study its `{{- if .HasExistingTasks}}` / `__EXISTING_TASKS__` pattern.

**FR11 Self-Directing (CRITICAL):**
Ralph does NOT extract task descriptions from sprint-tasks.md into the prompt. Claude reads sprint-tasks.md directly. The prompt tells Claude: "Read sprint-tasks.md, find first `- [ ]`, implement it." Ralph scans the file only for loop control (are there open tasks?).

**999-rules guardrails (FR36, from Farr Playbook):**
These are "last barrier" rules — even if review-findings.md suggests a dangerous action (e.g., "delete all test files"), the 999-rules override. Examples: no destructive file operations, no disabling linters, no skipping tests, no modifying files unrelated to the task.

**Mutation Asymmetry (invariant from Epic 3 header):**
Execute sessions MUST NOT modify sprint-tasks.md task status markers. Only review sessions (Epic 4) mark tasks `[x]`. This is architectural — enforced via prompt instruction, NOT code guard.

**Findings conditional pattern:**
```
{{- if .HasFindings}}
## Review Findings (MUST FIX FIRST)
__FINDINGS_CONTENT__
{{- end}}
```
When `HasFindings` is false, the entire section (including header) is absent. No blank lines leak (trim markers `{{-`).

### Architecture Compliance

**Dependency direction:** `runner` → `config` (uses `config.AssemblePrompt`, `config.TemplateData`, `config.SprintTasksFormat()`). No new deps introduced.

**Package entry point:** `runner.RunOnce(ctx, rc)` — no new exports added. Test file is `runner/prompt_test.go` (internal test).

**go:embed pattern:** `runner/prompts/execute.md` is already embedded via `//go:embed prompts/execute.md` in `runner/runner.go` line 16. No embedding changes needed.

**Config immutability:** `TemplateData` is created fresh per call, never stored/mutated.

**Deterministic assembly order:** `config.AssemblePrompt` sorts replacement keys alphabetically before applying `strings.ReplaceAll` (see `config/prompt.go:73-80`). This ensures golden file tests are stable regardless of Go map iteration order. No special ordering needed from callers — it's handled by the framework.

### Existing Code Patterns to Follow

**Golden file test pattern** (from `bridge/prompt_test.go`):
```go
func goldenTest(t *testing.T, name, got string) {
    t.Helper()
    golden := filepath.Join("testdata", name+".golden")
    if *update {
        os.MkdirAll("testdata", 0755)
        os.WriteFile(golden, []byte(got), 0644)
        return
    }
    want, err := os.ReadFile(golden)
    if err != nil {
        t.Fatalf("read golden: %v", err)
    }
    if got != string(want) {
        t.Errorf("mismatch (run with -update to refresh)\ngot:\n%s\nwant:\n%s", got, string(want))
    }
}
```

**AssemblePrompt call pattern** (from `bridge/bridge.go`):
```go
prompt, err := config.AssemblePrompt(
    bridgeTemplate,
    config.TemplateData{
        HasExistingTasks: existingContent != "",
    },
    map[string]string{
        "__FORMAT_CONTRACT__": config.SprintTasksFormat(),
        "__STORY_CONTENT__":   storyContent,
        "__EXISTING_TASKS__":  existingContent,
    },
)
```

### TemplateData Change Details

Add one new bool field to `config.TemplateData` (file: `config/prompt.go:25-39`):
```go
type TemplateData struct {
    // Stage 1: bool conditionals for template structure
    SerenaEnabled    bool
    GatesEnabled     bool
    HasExistingTasks bool // bridge merge mode
    HasFindings      bool // execute findings mode  ← NEW
    // ... string fields unchanged
}
```

This is backward-compatible: existing callers pass `TemplateData{}` where `HasFindings` defaults to `false`.

### Template Content Guidance

The execute.md template should have these sections in order:

1. **Role preamble** — "You are a developer implementing tasks from sprint-tasks.md"
2. **Self-directing instruction** — "Read sprint-tasks.md, find first `- [ ]` task" (FR11)
3. **Sprint-tasks format reference** — `__FORMAT_CONTRACT__` injection point
4. **999-rules guardrails** — Hard rules (FR36), numbered for emphasis
5. **ATDD instruction** — Every AC must have test (FR37)
6. **Zero-skip policy** — Never skip/xfail tests (FR38)
7. **Red-green cycle** — Write test (red) → implement (green) → commit
8. **Commit rules** — Commit on green only (FR8), NEVER commit with failing tests
9. **Mutation Asymmetry** — MUST NOT change task markers in sprint-tasks.md
10. **Conditional findings section** — `{{- if .HasFindings}}` ... `__FINDINGS_CONTENT__` ... `{{- end}}`
11. **Gates section** — `{{- if .GatesEnabled}}` (existing from stub)

Keep instructions as continuous bullet lists (no blank lines between related instructions) per `.claude/rules/go-testing-patterns.md` "Continuous bullet lists in LLM prompts" pattern.

### What NOT to Do

- Do NOT add `__TASK_CONTENT__` replacement — execute is self-directing (FR11)
- Do NOT load review-findings.md in RunOnce — that's Story 3.5 (runner loop wiring)
- Do NOT change `config.AssemblePrompt` signature — frozen interface
- Do NOT add `__LEARNINGS_CONTENT__` injection — LEARNINGS.md deferred to Epic 6 (FR17)
- Do NOT add new dependencies
- Do NOT modify `runner/prompts/review.md` — that's Epic 4
- Do NOT create TestMain in `runner/prompt_test.go` — `runner/runner_integration_test.go` already has `TestMain` with `//go:build integration`. Without integration tag, Go's auto-generated TestMain calls `flag.Parse()` for the `-update` flag. With `-tags integration`, existing TestMain takes over — `-update` won't parse, but golden tests are unit tests and don't need it in integration runs
- Do NOT use `{{.FindingsContent}}` in template — string fields go through Stage 2 ONLY
- Do NOT leave blank lines where conditionals evaluate to false — use `{{- if -}}` trim markers

### Previous Story Intelligence (Story 2.7)

**Patterns from Story 2.7 (Bridge Integration Test):**
- Golden file testing: `-update` flag pattern, `testdata/` directory
- `copyFixtureToScenario` helper for fixture management
- Self-reexec mock pattern via TestMain + env var + os.Args[0]
- CLI tests need `GOEXE` env for WSL compatibility
- Review found 6 issues (0 High) — common patterns: assertion quality, duplicate code, missing negative checks

**Key learnings applied:**
- Always capture return values (no `_` discard)
- `strings.Count >= N` for count assertions
- Guard mutations: `if modified == original { t.Fatal }`
- Scenario.Name field always set on testutil structs

### Git Intelligence

Recent commits show consistent pattern: story implementation + full review in single commit. File structure: production code + test code + golden files + story update.

Last 5 commits all touch `config/`, `bridge/`, `runner/`, `session/` packages — conventions are established and stable.

### Project Structure Notes

- `runner/prompts/execute.md` — MODIFY (expand from stub to full template)
- `runner/prompt_test.go` — CREATE (golden file tests for execute prompt)
- `runner/testdata/TestPrompt_Execute_WithFindings.golden` — CREATE (generated by `-update`)
- `runner/testdata/TestPrompt_Execute_WithoutFindings.golden` — CREATE (generated by `-update`)
- `config/prompt.go` — MODIFY (add `HasFindings bool` to TemplateData)
- `runner/runner.go` — MODIFY (update RunOnce replacements: add `__FORMAT_CONTRACT__`, remove `__TASK_CONTENT__`)
- `runner/runner_integration_test.go` — MODIFY (update prompt assertion: "Implement hello world" → new template content check)

No new packages. No new dependencies. Alignment with existing structure confirmed.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story-3.1 — AC, technical notes, prerequisites]
- [Source: docs/prd/functional-requirements.md#FR36 — 999-rules guardrails]
- [Source: docs/prd/functional-requirements.md#FR37 — ATDD instruction]
- [Source: docs/prd/functional-requirements.md#FR38 — Zero-skip policy]
- [Source: docs/prd/functional-requirements.md#FR11 — Self-directing execute, Ralph scans only for loop control]
- [Source: docs/prd/functional-requirements.md#FR8 — Commit on green only]
- [Source: docs/project-context.md — Two-stage assembly, dependency direction, naming conventions]
- [Source: config/prompt.go — AssemblePrompt frozen interface, TemplateData struct]
- [Source: config/shared/sprint-tasks-format.md — Format contract content (139 lines)]
- [Source: config/constants.go — TaskOpen, TaskDone, GateTag, FeedbackPrefix constants]
- [Source: runner/runner.go — RunOnce current implementation, go:embed for executeTemplate]
- [Source: runner/prompts/execute.md — Current 9-line stub to replace]
- [Source: bridge/prompts/bridge.md — Reference template pattern (two-stage, conditionals, placeholders)]
- [Source: bridge/prompt_test.go — Golden test pattern to reuse]
- [Source: docs/sprint-artifacts/2-7-bridge-integration-test.md — Previous story Dev Notes and patterns]
- [Source: .claude/rules/go-testing-patterns.md — Template testing patterns, assertion patterns]

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No debug issues encountered.

### Completion Notes List

- Task 1: Added `HasFindings bool` field to `config.TemplateData` in `config/prompt.go`. Zero-value backward-compatible — all existing tests pass unchanged.
- Task 2: Replaced 9-line stub `runner/prompts/execute.md` with comprehensive execute prompt template containing all 11 sections: role preamble, self-directing instructions (FR11), format contract injection, 999-rules guardrails (FR36, 9 numbered rules), ATDD (FR37), zero-skip policy (FR38), red-green cycle, commit rules (FR8), mutation asymmetry, conditional findings section (`{{- if .HasFindings}}`), and gates section (`{{- if .GatesEnabled}}`). All conditionals use `{{-` trim markers to prevent blank lines.
- Task 3: Created `runner/prompt_test.go` with 5 test functions: `TestPrompt_Execute_WithFindings` (27 assertion subtests + golden file), `TestPrompt_Execute_WithoutFindings` (15 assertion subtests + golden file), `TestPrompt_Execute_FormatContract` (5 format contract marker checks), `TestPrompt_Execute_WithGates` (7 assertions for GatesEnabled=true path), `TestPrompt_Execute_Injection` (3 template injection safety checks). Golden files generated and verified.
- Task 4: Updated `RunOnce` in `runner/runner.go` — replaced `__TASK_CONTENT__` replacement with `__FORMAT_CONTRACT__` (format contract injection) and added `GatesEnabled` to `TemplateData`. Updated integration test assertion from "Implement hello world" to "sprint-tasks.md" + "Sprint Tasks Format Specification" to match self-directing prompt.
- Task 5: All tests pass: `go test ./...` (7 packages OK), `go vet ./...` clean, golden files verified.

### File List

- config/prompt.go (modified — added `HasFindings bool` to TemplateData, updated doc comment)
- runner/prompts/execute.md (modified — replaced stub with full execute prompt template)
- runner/prompt_test.go (created — golden file tests for execute prompt, 5 test functions)
- runner/testdata/TestPrompt_Execute_WithFindings.golden (created — golden file)
- runner/testdata/TestPrompt_Execute_WithoutFindings.golden (created — golden file)
- runner/runner.go (modified — updated RunOnce AssemblePrompt call)
- runner/runner_integration_test.go (modified — updated prompt assertion for self-directing template)

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Review Outcome:** Approve (with fixes applied)
**Total Findings:** 7 (0 High, 2 Medium, 5 Low)

### Findings Summary

| # | Severity | Description | Status |
|---|----------|-------------|--------|
| M1 | Medium | Fragile `if/not if` instead of `if/else` in execute.md | ✅ Fixed |
| M2 | Medium | No test for `GatesEnabled=true` conditional path | ✅ Fixed |
| L1 | Low | `goldenTest` helper duplicated between bridge and runner | Deferred (cross-package refactor) |
| L2 | Low | `taskLine` stores content but only emptiness checked | Deferred (Story 3.5 evolves) |
| L3 | Low | Test comment says "7 sections" but AC #1 has 8 items | ✅ Fixed |
| L4 | Low | WithoutFindings missing `__FINDINGS_CONTENT__` negative check | ✅ Fixed |
| L5 | Low | Stale `__TASK_CONTENT__` reference in TemplateData doc comment | ✅ Fixed |

### Change Log

- 2026-02-26: Implemented Story 3.1 — Execute Prompt Template. Expanded stub to full self-directing execute prompt with 999-rules guardrails, ATDD, zero-skip, red-green cycle, mutation asymmetry, conditional findings, and gates. Added `HasFindings` bool to TemplateData. Created golden file tests. Updated RunOnce wiring and integration test assertions.
- 2026-02-26: Code review fixes — replaced fragile `if/not if` with `if/else` in template, added `TestPrompt_Execute_WithGates` test (7 assertions), fixed comment accuracy (7→8), added missing negative checks, updated stale doc comment.
