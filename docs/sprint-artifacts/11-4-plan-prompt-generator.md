# Story 11.4: plan/prompts/plan.md — generator prompt

Status: Done

## Review Findings (0H/4M/1L)

### MEDIUM
- [M1] AC1 требует `{{.Existing}}` template var, реализовано как `__EXISTING__` placeholder — AC отклонение `[plan/plan.go:34-38]`
- [M2] Нет тестов на error paths GeneratePrompt (parse, execute, unreplaced) `[plan/plan.go:63,68,86]`
- [M3] Тест не проверяет `[SETUP]` tag — AC2 требует SETUP+GATE `[plan/plan_test.go:81-88]`
- [M4] TestPlanPrompt_Generate_Merge без golden file — inconsistent с non-merge `[plan/plan_test.go:93-125]`

### LOW
- [L1] `missingkey=error` NO-OP для struct data — ложная безопасность `[plan/plan.go:61]`

## Story

As a система,
I want иметь промпт для генератора sprint-tasks.md,
so that Claude получает чёткие инструкции по созданию задач из входных документов.

## Acceptance Criteria

1. **Файл `plan/prompts/plan.md` как `text/template`** с переменными: `{{.Inputs}}`, `{{.OutputPath}}`, `{{.Merge}}`, `{{.Existing}}`

2. **Промпт содержит инструкции:**
   - Как интерпретировать typed headers `<!-- file: <name> | role: <role> -->`
   - Формат задач sprint-tasks.md (SETUP, GATE, обычные задачи)
   - Требование source-ссылок с реальными именами файлов из typed headers
   - Инструкция для merge mode (если `{{.Merge}}`)

3. **`plan/plan_test.go` содержит `TestPlanPrompt_Generate`** с golden file `plan/testdata/TestPlanPrompt_Generate.golden`

4. **Тест проходит с `-update` для регенерации golden и без `-update` для валидации**

## Tasks / Subtasks

- [x] Task 1: Создать `plan/prompts/plan.md` (AC: #1, #2)
  - [x] Определить шаблон с `{{.Inputs}}`, `{{.OutputPath}}`, `{{.Merge}}`, `{{.Existing}}`
  - [x] Секция: интерпретация typed headers `<!-- file: <name> | role: <role> -->`
  - [x] Секция: формат задач (SETUP, GATE, обычные), использование `- [ ]` / `- [x]`
  - [x] Секция: source-ссылки с реальными именами файлов из typed headers
  - [x] Секция: merge mode `{{if .Merge}}...{{end}}` — добавлять задачи, не трогать `[x]`
- [x] Task 2: Создать golden file и тест (AC: #3, #4)
  - [x] Создать `plan/testdata/` директорию
  - [x] Создать `TestPlanPrompt_Generate` в `plan/plan_test.go`
  - [x] Реализовать `-update` flag pattern для golden file
  - [x] Проверить: `go test ./plan/... -count=1 -update` генерирует golden, повторный запуск без `-update` проходит

## Dev Notes

### Архитектурные ограничения

- **Двухэтапная Prompt Assembly (КРИТИЧНО):** [Source: docs/project-context.md#Двухэтапная Prompt Assembly]
  1. Этап 1: `text/template` для структуры промпта (`{{if .Merge}}`)
  2. Этап 2: `strings.Replace` для user content injection (содержимое файлов)
  - User content через `template.Execute` ЗАПРЕЩЕНО — template injection risk
- **Placeholder для content:** `__CONTENT_N__` где N — индекс PlanInput [Source: docs/epics.md#Story 11.4 Technical Notes]
- **Prompt файлы = Go templates, не чистый Markdown** [Source: docs/project-context.md#Prompt файлы]

### Существующий паттерн

- `bridge/prompts/bridge.md` — reference template для промптов (удаляется в Epic 14, но паттерн тот же)
- `runner/prompts/execute.md`, `runner/prompts/review.md` — аналогичные шаблоны
- Golden file pattern: `bridge/testdata/TestBridgePrompt_Creation.golden` — reference

### templateData struct

```go
type templateData struct {
    Inputs     []PlanInput
    OutputPath string
    Merge      bool
    Existing   string // текущий sprint-tasks.md (для merge prompt)
}
```
Определяется в `plan/plan.go` (unexported) [Source: docs/project-context.md#TemplateData struct]

### Content injection

User content (содержимое файлов) вставляется через `strings.Replace` ПОСЛЕ `template.Execute`:
```go
result = strings.Replace(result, "__CONTENT_0__", string(inputs[0].Content), 1)
result = strings.Replace(result, "__CONTENT_1__", string(inputs[1].Content), 1)
```

### Формат sprint-tasks.md

Ссылка на существующий формат: `config/constants.go` содержит `TaskOpen = "- [ ]"`, `TaskDone = "- [x]"`, `GateTag = "[GATE]"` [Source: docs/project-context.md#String Constants]

### Тестирование

- Golden file with `-update` flag [Source: CLAUDE.md#Testing Core Rules]
- Template trim markers `{{- if -}}` должны быть ПРИМЕНЕНЫ [Source: .claude/rules/test-templates-review.md]
- Negative examples (WRONG format) с dedicated assertions [Source: .claude/rules/test-templates-review.md]
- Mutually exclusive conditionals: `{{if}}/{{else}}/{{end}}`, НЕ `{{if}}/{{end}} {{if not}}/{{end}}` [Source: .claude/rules/test-templates-review.md]

### Project Structure Notes

- `plan/prompts/plan.md` — НОВЫЙ файл (template)
- `plan/plan_test.go` — НОВЫЙ тест (или расширение если создан в 11.1)
- `plan/testdata/TestPlanPrompt_Generate.golden` — НОВЫЙ golden file

### References

- [Source: docs/epics.md#Story 11.4] — полные AC и технические заметки
- [Source: docs/project-context.md#Двухэтапная Prompt Assembly] — двухэтапный рендер
- [Source: docs/project-context.md#TemplateData struct] — struct для промптов
- [Source: docs/project-context.md#Plan Package] — архитектура plan/
- [Source: .claude/rules/test-templates-review.md] — паттерны тестирования шаблонов
- [Source: config/constants.go] — TaskOpen, TaskDone, GateTag константы
- [Source: bridge/prompts/bridge.md] — reference шаблон промпта

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created `plan/prompts/plan.md` template with typed headers, task format, source references, merge mode
- Added `templateData` struct and `GeneratePrompt` function to `plan/plan.go` with two-stage assembly
- Created `plan/plan_test.go` with goldenTest helper, `TestPlanPrompt_Generate` and `TestPlanPrompt_Generate_Merge`
- Golden file `plan/testdata/TestPlanPrompt_Generate.golden` generated and validated
- Two-stage prompt assembly: Stage 1 = text/template (structure), Stage 2 = strings.Replace (user content)
- Full regression: `go test ./... -count=1` — all packages pass, 0 regressions

### File List

- plan/plan.go (modified): added templateData struct, GeneratePrompt function, go:embed, PlanPrompt()
- plan/prompts/plan.md (new): plan prompt template
- plan/plan_test.go (new): goldenTest helper, TestPlanPrompt_Generate, TestPlanPrompt_Generate_Merge
- plan/testdata/TestPlanPrompt_Generate.golden (new): golden file
