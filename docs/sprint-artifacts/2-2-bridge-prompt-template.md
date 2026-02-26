# Story 2.2: Bridge Prompt Template

Status: done

## Story

As a developer using ralph bridge,
I want a well-structured prompt template that gives Claude clear instructions,
so that story files are accurately converted into sprint-tasks.md format.

## Acceptance Criteria

1. **Bridge prompt template exists at `bridge/prompts/bridge.md`** using text/template syntax with go:embed in the bridge package.

2. **Template includes the sprint-tasks format contract** via `__FORMAT_CONTRACT__` Stage 2 placeholder. At runtime, `bridge.Run` injects `config.SprintTasksFormat()` through the replacements map. The full format spec appears verbatim in the assembled prompt so Claude knows the exact output format. Do NOT hardcode the format text in bridge.md — it MUST come from the single source-of-truth in `config/shared/sprint-tasks-format.md`.

3. **Template includes conversion instructions:**
   - Story-to-tasks conversion: each AC → one or more tasks
   - Test case derivation from objective AC (FR2) with red-green principle reminder
   - Gate marking skeleton: `[GATE]` placement instructions (FR3 — detailed in Story 2.4)
   - Service task generation skeleton: `[SETUP]`, `[VERIFY]`, `[E2E]` instructions (FR5 — detailed in Story 2.4)
   - Source traceability skeleton: `source:` field on every task (FR5a — detailed in Story 2.4)

4. **Template uses two-stage prompt assembly** (architecture mandate):
   - Stage 1 (text/template): `{{if .HasExistingTasks}}` — bool conditional for smart merge mode
   - Stage 2 (strings.Replace) placeholders:
     - `__STORY_CONTENT__` — story file content (user-controlled, MAY contain `{{`)
     - `__FORMAT_CONTRACT__` — sprint-tasks format spec from `config.SprintTasksFormat()`
     - `__EXISTING_TASKS__` — existing sprint-tasks.md content for merge mode (inside `{{if .HasExistingTasks}}` block). Story 2.6 populates this; Story 2.2 creates the placeholder.
   - **CRITICAL:** Story content and existing tasks are user-controlled — MUST use Stage 2 placeholders, NOT `{{.FieldName}}`. This follows the architecture pattern in `config/prompt.go` TemplateData doc comment.

5. **Template includes negative examples** (prohibited formats):
   - "DO NOT use numbered lists"
   - "DO NOT add markdown headers outside the defined structure"
   - "Every task MUST start with exactly `- [ ]`"
   - "source: field MUST be indented under parent task"

6. **Golden file snapshot test with scenario verification** in `bridge/prompt_test.go`:
   - Test assembles prompt via `config.AssemblePrompt` with realistic data
   - Verifies assembled prompt contains key sections (format contract text, conversion instructions, negative examples)
   - Compares output to golden file (`bridge/testdata/TestBridgePrompt_Creation.golden`)
   - Second test variant with `HasExistingTasks=true` for merge mode (`TestBridgePrompt_Merge.golden`)
   - NOT just snapshot — must verify key content via `strings.Contains` assertions

7. **`HasExistingTasks` bool field added to `config.TemplateData`** so bridge prompt can use Stage 1 conditional `{{if .HasExistingTasks}}`.

## Tasks / Subtasks

- [x] Task 1: Extend `config.TemplateData` with bridge fields (AC: #7)
  - [x] 1.1 Add to `TemplateData` struct in `config/prompt.go`:
    - `HasExistingTasks bool` — Stage 1 conditional (doc comment: "bridge merge mode")
    - `StoryContent string` — Stage 2 type grouping (doc comment: "injected via `__STORY_CONTENT__` replacement, NOT template")
    - `ExistingTasksContent string` — Stage 2 type grouping (doc comment: "injected via `__EXISTING_TASKS__` replacement, NOT template")
  - [x] 1.2 Update `TestAssemblePrompt_AllFields` in `config/prompt_test.go` to exercise `HasExistingTasks` (add `{{if .HasExistingTasks}}` to template)
  - [x] 1.3 Verify existing config tests still pass (no regressions)

- [x] Task 2: Create `bridge/prompts/bridge.md` prompt template (AC: #1, #2, #3, #4, #5)
  - [x] 2.1 Delete `bridge/prompts/.gitkeep` (replaced by actual file)
  - [x] 2.2 Create `bridge/prompts/bridge.md` following the section outline below:
    1. **Role preamble** — You are a task planner converting user stories to sprint-tasks.md
    2. **Format contract** — `__FORMAT_CONTRACT__` Stage 2 placeholder (injected from `config.SprintTasksFormat()`)
    3. **Story content** — `__STORY_CONTENT__` Stage 2 placeholder (injected story file)
    4. **Conversion instructions** — Each AC → one or more tasks, preserve AC numbering
    5. **Test derivation (FR2)** — Red-green principle: write test task BEFORE implementation task for each objective AC. Subjective AC → mark for review
    6. **Gate marking skeleton (FR3)** — Brief: append `[GATE]` to first task of epic and user-visible milestones. `<!-- Story 2.4 enriches with detailed placement rules -->`
    7. **Service tasks skeleton (FR5)** — Brief: generate `[SETUP]` for framework deps, `[VERIFY]` for integration checks, `[E2E]` for end-to-end flows. `<!-- Story 2.4 enriches with detection criteria -->`
    8. **Source traceability skeleton (FR5a)** — Brief: every task MUST have indented `source:` field. `<!-- Story 2.4 enriches with scoping rules -->`
    9. **Test framework presence check** — If project has test framework → generate test tasks. If not → include `[SETUP]` task for test framework setup
    10. **Negative examples** — Prohibited formats (numbered lists, extra headers, wrong task syntax, unindented source fields)
    11. **Merge mode conditional** — `{{- if .HasExistingTasks}}` block containing `__EXISTING_TASKS__` placeholder and brief merge skeleton (preserve `[x]` status, preserve source fields, preserve order). `<!-- Story 2.6 enriches with detailed merge instructions -->`
    12. **Output instructions** — Output only the sprint-tasks.md content, no explanations
  - [x] 2.3 Use `{{- if -}}` trim markers to prevent blank lines from disabled conditionals (Story 1.10 learning)

- [x] Task 3: Add go:embed and prompt accessor in `bridge/bridge.go` (AC: #1)
  - [x] 3.1 Add `//go:embed prompts/bridge.md` directive and `var bridgePrompt string`
  - [x] 3.2 Add `BridgePrompt() string` exported function for prompt access (mirrors `config.SprintTasksFormat()` pattern)
  - [x] 3.3 Ensure import of `_ "embed"` if needed (check if already imported)

- [x] Task 4: Create golden file tests in `bridge/prompt_test.go` (AC: #6)
  - [x] 4.1 Create `bridge/prompt_test.go` with `TestMain` (flag.Parse for `-update`), golden file helper
  - [x] 4.2 `TestBridgePrompt_Creation` — assemble prompt with `HasExistingTasks=false`, replacements map `{"__STORY_CONTENT__": "test story content", "__FORMAT_CONTRACT__": config.SprintTasksFormat()}`, verify:
    - Assembled prompt contains format contract text (unique marker: `"Sprint Tasks Format Specification"`)
    - Contains "test story content" (story placeholder replaced)
    - Contains conversion instructions (e.g., `"acceptance criter"` substring)
    - Contains negative examples (e.g., `"DO NOT"`)
    - Does NOT contain merge instructions or `__EXISTING_TASKS__`
    - Compare to golden file
  - [x] 4.3 `TestBridgePrompt_Merge` — assemble prompt with `HasExistingTasks=true`, replacements map adds `{"__EXISTING_TASKS__": "- [x] existing task\n  source: stories/test.md#AC-1"}`, verify:
    - Contains all creation-mode content
    - ALSO contains merge-specific text and existing tasks content
    - Compare to golden file
  - [x] 4.4 `TestBridgePrompt_NonEmpty` — basic embed verification (prompt string is not empty)
  - [x] 4.5 `TestBridgePrompt_ContainsFormatContract` — verify `config.SprintTasksFormat()` key markers present in assembled prompt (Structural Rule #8 cross-package verification)

- [x] Task 5: Run all tests and verify (AC: all)
  - [x] 5.1 Run `go test ./config/...` — verify no regressions
  - [x] 5.2 Run `go test -update ./bridge/...` — generate golden files
  - [x] 5.3 Run `go test ./bridge/...` — verify golden files match
  - [x] 5.4 Run `go vet ./...` — verify no vet issues

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Reviewer Model:** Claude Opus 4.6 (same session, adversarial review workflow)
**Review Outcome:** Approve (after fixes)

### Action Items

- [x] [HIGH] Remove `{{- if .GatesEnabled}}` conditional from gate marking section — bridge should always generate [GATE] markers; conditional was extra scope beyond AC + untested false path [bridge/prompts/bridge.md:33]
- [x] [MED] Make Merge test assertions symmetric with Creation test — 5 creation checks expanded to 12+ (matching Creation's coverage) [bridge/prompt_test.go:124-140]
- [x] [MED] Replace hardcoded `if c.name == "..."` negative check pattern with `present bool` struct field in Merge test [bridge/prompt_test.go:152-165]
- [x] [MED] Replace fragile `"acceptance criter"` partial substring with specific `"For each AC, create"` unique to Conversion Instructions section [bridge/prompt_test.go:77]
- [x] [LOW] Replace numbered list (1. 2. 3.) with bullet points in Conversion Instructions to avoid stylistic conflict with Prohibited Formats prohibition [bridge/prompts/bridge.md:18-24]

**Summary:** 5 issues found (1 HIGH, 3 MED, 1 LOW), all auto-fixed in this session. Key finding: dev agent added `GatesEnabled` conditional without AC mandate and without test coverage for the false path — removed in favor of unconditional gate marking which better matches bridge's role (format generation, not execution control).

## Dev Notes

### Placeholder Keys Quick Reference

| Key | Stage | Source | Injected By |
|-----|-------|--------|-------------|
| `{{if .HasExistingTasks}}` | 1 (text/template) | `config.TemplateData` bool | bridge.Run caller |
| `__STORY_CONTENT__` | 2 (strings.Replace) | User story file from disk | bridge.Run (Story 2.3) / test |
| `__FORMAT_CONTRACT__` | 2 (strings.Replace) | `config.SprintTasksFormat()` | bridge.Run (Story 2.3) / test |
| `__EXISTING_TASKS__` | 2 (strings.Replace) | Existing sprint-tasks.md | bridge.Run (Story 2.6) / test |

### Architecture Pattern: Two-Stage Prompt Assembly

The bridge prompt MUST follow the two-stage assembly pattern from `config/prompt.go`:

- **Stage 1 (text/template):** Process bool conditionals like `{{if .HasExistingTasks}}`. Safe because `TemplateData` is code-controlled.
- **Stage 2 (strings.Replace):** Inject content via `__PLACEHOLDER__` syntax. Template engine does NOT re-process Stage 2, so user content with `{{` remains literal.

The epic AC mentions `{{.StoryContent}}` but this would be a **template injection vulnerability** — story files are user-authored and may contain `{{`. Use `__STORY_CONTENT__` instead. Same pattern as `__TASK_CONTENT__` in `runner/prompts/execute.md`. Same applies to existing tasks content (`__EXISTING_TASKS__`) and format contract (`__FORMAT_CONTRACT__` — code-controlled but kept as Stage 2 for consistency and because it's a large text block).

### Story 2.2 = Skeleton, Story 2.4 = Enrichment

This story creates the BASIC prompt structure. Story 2.4 enriches with detailed FR-specific instructions:
- FR3: Detailed gate placement rules (which tasks get `[GATE]`, first-of-epic logic)
- FR5: Detailed service task detection (dependency scanning, `[SETUP]`/`[VERIFY]`/`[E2E]` criteria)
- FR5a: Detailed source field coverage rules (AC numbering, scoping)

Include skeleton sections with brief instructions and TODO markers for Story 2.4.

### Existing Code Patterns

**Prompt file pattern** — see `runner/prompts/execute.md` (line 1-8):
- Simple, directive text
- `{{- if .Field -}}` trim markers for conditionals
- `__PLACEHOLDER__` for Stage 2 content

**go:embed pattern** — see `config/format.go`:
```go
import _ "embed"

//go:embed prompts/bridge.md
var bridgePrompt string

func BridgePrompt() string { return bridgePrompt }
```

**Golden file test pattern** — see `config/prompt_test.go`:
- `var update = flag.Bool("update", false, "update golden files")`
- `TestMain` for flag parsing
- `goldenTest(t, filename, got)` helper
- Both scenario assertions AND golden file comparison

### Template Trim Markers (Story 1.10 Learning)

Disabled `{{if}}` blocks leave blank lines. Always use trim markers:
```
{{- if .HasExistingTasks}}
merge instructions here
{{- end}}
```

### Testing Anti-Pattern

From architecture: "Golden file = достаточно для промптов" is WRONG. Must add scenario test exercising actual data substitution AND content verification via `strings.Contains`.

### Bridge Package TestMain Consideration

`bridge/format_test.go` already exists but has no `TestMain`. Adding `prompt_test.go` with `TestMain` means it will apply to the whole bridge package test suite — this is expected and causes no conflict. The `goldenTest` helper follows the same pattern as `config/prompt_test.go` (expected package-local duplication, not worth extracting to `internal/testutil` for just 2 packages).

### Test Framework Presence Check

The AC requires "Instructions to check test framework presence". In the bridge prompt, this means: instruct Claude to check if the project has a test framework set up (e.g., Go testing, Jest, pytest). If yes → generate test tasks per red-green principle. If no → include a `[SETUP]` task for test framework installation before test tasks.

### Project Structure Notes

- Files align with architecture: `bridge/prompts/bridge.md`, `bridge/prompt_test.go`
- `bridge/testdata/` directory already has `.gitkeep` — will contain golden files
- `config.TemplateData` is the shared struct — adding `HasExistingTasks` is non-breaking
- No new dependencies required

### References

- [Source: docs/architecture/implementation-patterns.md — Naming, Error Handling, File I/O, Testing]
- [Source: docs/architecture/architectural-decisions.md — Two-stage prompt assembly, text/template + strings.Replace]
- [Source: docs/architecture/project-structure.md — bridge/prompts/, bridge/testdata/]
- [Source: docs/prd/functional-requirements.md — FR1, FR2, FR3, FR5, FR5a]
- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md — Story 2.2 AC]
- [Source: config/prompt.go — TemplateData struct, AssemblePrompt function, two-stage doc comments]
- [Source: config/format.go — go:embed pattern, SprintTasksFormat() accessor]
- [Source: runner/prompts/execute.md — Prompt file pattern with trim markers and __PLACEHOLDER__]
- [Source: docs/sprint-artifacts/2-1-shared-sprint-tasks-format-contract.md — Review learnings, completion record]

### Existing Code to Build On

| File | Status | Description |
|------|--------|-------------|
| `config/prompt.go` | modify | Add `HasExistingTasks bool`, `StoryContent string`, `ExistingTasksContent string` to `TemplateData` |
| `config/prompt_test.go` | modify | Update `AllFields` test for new field |
| `bridge/bridge.go` | modify | Add go:embed directive, `bridgePrompt` var, `BridgePrompt()` accessor |
| `bridge/prompts/.gitkeep` | delete | Replaced by `bridge.md` |
| `bridge/prompts/bridge.md` | new | The prompt template file |
| `bridge/prompt_test.go` | new | Golden file + scenario tests |
| `bridge/testdata/TestBridgePrompt_Creation.golden` | new | Golden file (creation mode) |
| `bridge/testdata/TestBridgePrompt_Merge.golden` | new | Golden file (merge mode) |
| `bridge/testdata/.gitkeep` | delete | Replaced by golden files |

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None required — clean implementation without debugging.

### Completion Notes List

- Task 1: Added `HasExistingTasks bool`, `StoryContent string`, `ExistingTasksContent string` to `config.TemplateData`. Updated `TestAssemblePrompt_AllFields` to exercise all new fields including `HasExistingTasks` conditional and Stage 2 placeholders for story/existing content. All 47 config tests pass.
- Task 2: Created `bridge/prompts/bridge.md` with all 12 sections per AC: role preamble, `__FORMAT_CONTRACT__` placeholder, `__STORY_CONTENT__` placeholder, conversion instructions, test derivation (FR2 red-green), gate marking skeleton (FR3), service tasks skeleton (FR5), source traceability skeleton (FR5a), test framework presence check, negative examples, merge mode conditional (`{{- if .HasExistingTasks}}`), output instructions. Applied `{{- if -}}` trim markers per Story 1.10 learning. Skeleton sections include HTML comment markers for Story 2.4/2.6 enrichment.
- Task 3: Added `//go:embed prompts/bridge.md`, `var bridgePrompt string`, and `BridgePrompt() string` accessor to `bridge/bridge.go`. Mirrors `config.SprintTasksFormat()` pattern.
- Task 4: Created `bridge/prompt_test.go` with `TestMain`, `goldenTest` helper, and 5 test functions: `TestBridgePrompt_NonEmpty` (embed check), `TestBridgePrompt_Creation` (17 substring assertions + golden file), `TestBridgePrompt_Merge` (19 symmetric assertions + golden file), `TestBridgePrompt_ContainsFormatContract` (8 marker assertions, Structural Rule #8). Both golden files generated and verified stable.
- Task 5: Full test suite passes (bridge, cmd/ralph, config, session, internal/testutil). `go vet ./...` clean.
- Review fixes: (1) Removed `{{- if .GatesEnabled}}` from gate marking — bridge always generates [GATE] markers. (2) Made Merge test assertions symmetric with Creation (19 checks). (3) Replaced hardcoded `if c.name ==` negative check with `present bool` pattern. (4) Replaced fragile `"acceptance criter"` substring with specific `"For each AC, create"`. (5) Changed numbered list to bullet points in Conversion Instructions.

### Change Log

- 2026-02-26: Implemented Story 2.2 — Bridge prompt template with two-stage assembly, golden file tests, and format contract verification.
- 2026-02-26: Code review fixes — 1 HIGH, 3 MED, 1 LOW resolved. Gate marking unconditional, test symmetry enforced, fragile assertions fixed.

### File List

- `config/prompt.go` — modified (added HasExistingTasks, StoryContent, ExistingTasksContent to TemplateData)
- `config/prompt_test.go` — modified (updated TestAssemblePrompt_AllFields for new fields)
- `config/testdata/TestAssemblePrompt_AllFields.golden` — modified (regenerated for new fields)
- `bridge/bridge.go` — modified (added go:embed, bridgePrompt var, BridgePrompt() accessor)
- `bridge/prompts/bridge.md` — new (the prompt template)
- `bridge/prompt_test.go` — new (golden file + scenario tests)
- `bridge/testdata/TestBridgePrompt_Creation.golden` — new (creation mode golden file)
- `bridge/testdata/TestBridgePrompt_Merge.golden` — new (merge mode golden file)
- `bridge/prompts/.gitkeep` — deleted (replaced by bridge.md)
- `bridge/testdata/.gitkeep` — deleted (replaced by golden files)
- `docs/sprint-artifacts/sprint-status.yaml` — modified (status: ready-for-dev → in-progress → review)
- `docs/sprint-artifacts/2-2-bridge-prompt-template.md` — modified (tasks marked, Dev Agent Record updated)
