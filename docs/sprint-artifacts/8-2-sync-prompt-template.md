# Story 8.2: Sync Prompt Template

Status: done

## Story

As a разработчик,
I want иметь специализированный промпт для sync-сессии,
so that Claude фокусировался исключительно на обновлении Serena memories.

## Acceptance Criteria

1. **Prompt file exists (FR59):** `runner/prompts/serena-sync.md` создан как Go-шаблон. Embedded via `go:embed`. Доступен через `embed.FS` наряду с execute.md и review.md. Template compiles без ошибок через `text/template.Parse`.
2. **Two-stage assembly (FR59):** Промпт использует двухэтапную сборку: Stage 1 — `{{if .HasLearnings}}` и `{{if .HasCompletedTasks}}` conditional blocks; Stage 2 — `__DIFF_SUMMARY__`, `__LEARNINGS_CONTENT__`, `__COMPLETED_TASKS__` placeholders через `strings.Replace`. Финальный output не содержит `{{.Var}}` и `__PLACEHOLDER__`.
3. **TemplateData extension (Architecture Decision 3):** `config/prompt.go` TemplateData расширен полем `HasCompletedTasks bool`. Поле используется serena-sync.md. execute.md и review.md не затронуты (HasCompletedTasks=false). Existing TemplateData tests pass.
4. **Prompt content — instructions:** Промпт инструктирует Claude: `list_memories` -> `read_memory` -> `edit_memory`/`write_memory`. ЗАПРЕЩАЕТ: удаление memories, создание без необходимости. ПРЕДПОЧИТАЕТ: `edit_memory` над `write_memory`. Содержит секции: context, diff summary, learnings, tasks, instructions.
5. **Prompt content — conditional sections:** При `HasLearnings == false` — секция "Извлечённые уроки" отсутствует, `__LEARNINGS_CONTENT__` не в output. При `HasCompletedTasks == false` — секция "Завершённые задачи" отсутствует.
6. **assembleSyncPrompt function:** `runner/serena.go` определяет `assembleSyncPrompt(opts SerenaSyncOpts) (string, error)`. Возвращает собранный prompt string. Error при template parse failure.

## Tasks / Subtasks

- [x] Task 1: Add HasCompletedTasks to TemplateData (AC: #3)
  - [x] 1.1 Add `HasCompletedTasks bool` field to TemplateData in `config/prompt.go`
  - [x] 1.2 Verify `buildTemplateData` unchanged — `HasCompletedTasks=false` via zero-value. `assembleSyncPrompt` constructs TemplateData directly (Option B from Dev Notes)

- [x] Task 2: Create serena-sync.md prompt template (AC: #1, #4, #5)
  - [x] 2.1 Create `runner/prompts/serena-sync.md` with Go template syntax
  - [x] 2.2 Template contains: Role section, Context section with `__DIFF_SUMMARY__`, conditional `{{if .HasLearnings}}` block with `__LEARNINGS_CONTENT__`, conditional `{{if .HasCompletedTasks}}` block with `__COMPLETED_TASKS__`, Instructions section, Constraints section
  - [x] 2.3 Add `go:embed prompts/serena-sync.md` var in `runner/serena.go`

- [x] Task 3: Implement assembleSyncPrompt function (AC: #2, #6)
  - [x] 3.1 Define `SerenaSyncOpts` struct in `runner/serena.go` (DiffSummary, Learnings, CompletedTasks, MaxTurns, ProjectRoot strings/ints)
  - [x] 3.2 Implement `assembleSyncPrompt(opts SerenaSyncOpts) (string, error)` in `runner/serena.go`
  - [x] 3.3 Stage 1: build TemplateData with HasLearnings/HasCompletedTasks from opts, call `config.AssemblePrompt`
  - [x] 3.4 Stage 2: pass replacements map with `__DIFF_SUMMARY__`, `__LEARNINGS_CONTENT__`, `__COMPLETED_TASKS__`

- [x] Task 4: Tests (AC: #1-#6)
  - [x] 4.1 Template compilation test: verify `text/template.Parse` succeeds on serena-sync.md
  - [x] 4.2 Full assembly test with all conditionals true: verify no `{{` and no `__` in output
  - [x] 4.3 Conditional test: HasLearnings=false → learnings section absent
  - [x] 4.4 Conditional test: HasCompletedTasks=false → completed tasks section absent
  - [x] 4.5 Prompt content assertions: verify key instructions (list_memories, edit_memory, forbid delete)
  - [x] 4.6 Verify existing buildTemplateData tests still pass (HasCompletedTasks=false by zero-value, no regression)
  - [x] 4.7 Existing TemplateData tests still pass (no regression)

## Dev Notes

### Architecture Compliance

- **Two-stage assembly pattern:** CRITICAL — same as execute.md and review.md. Stage 1 = `text/template` for `{{if}}` blocks. Stage 2 = `strings.Replace` for user content (`__PLACEHOLDER__`). User content NEVER goes through `{{.Field}}` — template injection protection.
- **Dependency direction:** Changes in `config/` (TemplateData field) and `runner/` (template + assembly). No cmd/ralph changes.
- **Config = leaf package.** Adding `HasCompletedTasks` to TemplateData is safe — no new imports.

### Implementation Patterns (from existing code)

**go:embed pattern** (`runner/runner.go:24-43`):
```go
//go:embed prompts/serena-sync.md
var serenaSyncTemplate string
```
Place in `runner/serena.go` since it owns sync logic (same as `distillTemplate` in `runner/knowledge_distill.go:20-21`).

**TemplateData** (`config/prompt.go:29-45`):
- Bool fields in Stage 1 section: `HasCompletedTasks bool` next to `HasLearnings bool` (line 35).
- Comment style: `// true when completed tasks text is non-empty`.

**buildTemplateData** (`runner/runner.go:516-524`):
- Currently: `func buildTemplateData(cfg *config.Config, serenaHint string, hasFindings, hasLearnings bool) config.TemplateData`
- Needs new parameter or sync-specific variant. Options:
  - A) Add `hasCompletedTasks bool` parameter — changes all call sites (3+ locations). Clean but more churn.
  - B) Sync builds its own TemplateData directly — simpler, no churn to existing calls. Architecture doc says "Вариант A: расширение TemplateData" but for the `buildTemplateData` function, direct construction in assembleSyncPrompt is cleaner.
- **Recommended: Option B** — assembleSyncPrompt constructs TemplateData directly (like `RunReview` at line 1520 does `config.TemplateData{}`). This avoids changing buildTemplateData signature and all existing call sites. HasCompletedTasks=false at other call sites automatically via zero-value.

**AssemblePrompt** (`config/prompt.go:66-97`):
- `config.AssemblePrompt(tmplContent, data, replacements)` — returns `(string, error)`.
- Validates no unreplaced `__PLACEHOLDER__` in output.
- Used by distillTemplate too (`knowledge_distill.go:821`).

**Prompt template authoring** — existing patterns:
- Use `{{- if .Field -}}` trim markers to avoid blank lines from disabled sections.
- Sections separated by `---` or `##` headers.
- Russian language for prompt content (per PRD).

**SerenaSyncOpts struct** — per architecture doc:
```go
type SerenaSyncOpts struct {
    DiffSummary    string
    Learnings      string
    CompletedTasks string
    MaxTurns       int
    ProjectRoot    string
}
```
Note: This struct is also used by Story 8.4. Defining it here with assembleSyncPrompt is correct — 8.4 will use it.

### Critical Constraints

- **Template injection protection:** `__DIFF_SUMMARY__`, `__LEARNINGS_CONTENT__`, `__COMPLETED_TASKS__` are Stage 2 placeholders. Content may contain `{{` from user files — MUST be strings.Replace, never `{{.Field}}`.
- **Trim markers required:** `{{- if .HasLearnings -}}` — prevents blank lines in output when condition is false.
- **Prompt language:** Russian (per PRD/Architecture — "Извлечённые уроки", "Завершённые задачи").
- **~50 lines:** Architecture specifies `runner/prompts/serena-sync.md` ~50 lines. Keep concise.
- **No `__PLACEHOLDER__` in final output:** `config.AssemblePrompt` validates this (line 92-94). If a conditional block is disabled but its placeholder is in the replacements map, that's OK — the placeholder text is gone with the conditional block, and the replacement key simply doesn't match anything (no error).
- **assembleSyncPrompt needs conditional placeholder handling:** When HasLearnings=false, the `__LEARNINGS_CONTENT__` is inside a `{{if}}` block that gets removed. The replacement map still has the key but it won't match — `AssemblePrompt` checks for UNREPLACED placeholders, so if the placeholder text is removed by Stage 1, it won't be found. This is the correct behavior.

### Testing Standards

- **Table-driven** with Go stdlib assertions (no testify).
- **Test naming:** `TestAssembleSyncPrompt_AllSections`, `TestAssembleSyncPrompt_NoLearnings`, `TestAssembleSyncPrompt_NoCompletedTasks`.
- **Prompt content assertions:** Use discriminating substrings — `list_memories` (unique to sync prompt), `edit_memory` (unique to sync prompt), section headers in Russian.
- **Absence checks:** Use precise phrases — not generic words. E.g., "Извлечённые уроки" for learnings section absence.
- **Template parse test:** Verify `text/template.New("sync").Parse(serenaSyncTemplate)` succeeds.
- **No `{{` in output:** `strings.Contains(got, "{{")` must be false after assembly.
- **No `__` in output:** `unreplacedPlaceholderRe.FindAllString(got, -1)` must be empty (or delegate to AssemblePrompt which checks this).

### Project Structure Notes

- `config/prompt.go` — TemplateData struct (add HasCompletedTasks)
- `runner/serena.go` — SerenaSyncOpts, assembleSyncPrompt, go:embed var
- `runner/prompts/serena-sync.md` — NEW Go template file
- `runner/prompt_test.go` — Prompt assembly tests (add sync template tests)
- `runner/runner_run_test.go` — buildTemplateData tests (verify no regression)

### References

- [Source: docs/epics/epic-8-serena-memory-sync-stories.md#Story 8.2] — AC and technical notes
- [Source: docs/prd/serena-memory-sync.md#FR59] — Sync prompt template requirements
- [Source: docs/architecture/serena-memory-sync.md#Decision 3] — Prompt template architecture
- [Source: config/prompt.go:15-45] — TemplateData struct and doc comments
- [Source: config/prompt.go:66-97] — AssemblePrompt two-stage implementation
- [Source: runner/runner.go:24-43] — Existing go:embed patterns
- [Source: runner/runner.go:516-524] — buildTemplateData function
- [Source: runner/serena.go] — Existing Serena code (CodeIndexerDetector etc.)
- [Source: runner/knowledge_distill.go:20-21] — distillTemplate go:embed pattern
- [Source: runner/prompt_test.go] — Existing prompt test patterns

## Dev Agent Record

### Context Reference

### Agent Model Used
Claude Opus 4.6

### Debug Log References
N/A

### Completion Notes List
- All 4 tasks completed, all subtasks done
- Two-stage assembly pattern followed (Stage 1 template + Stage 2 string replace)
- Option B chosen: assembleSyncPrompt builds TemplateData directly, no buildTemplateData changes
- Full test suite passes: `go test ./...` — 0 failures, no regressions
- Prompt ~35 lines, Russian language, all 5 placeholders

### File List
- `config/prompt.go` — Added HasCompletedTasks bool to TemplateData
- `runner/prompts/serena-sync.md` — NEW: sync prompt template (~35 lines)
- `runner/serena.go` — Added go:embed, SerenaSyncOpts, assembleSyncPrompt
- `runner/prompt_test.go` — Added 6 test functions for sync prompt (5 original + 1 both-absent)

### Review Record
- **Reviewer:** Claude Opus 4.6
- **Findings:** 0H / 3M / 2L (5 total)
- **All fixed:** M1 (template.Parse test), M2 (max turns discriminating assertion), M3 (both-sections-absent test), L1 (delete_memory prohibition context), L2 (go fmt alignment)
