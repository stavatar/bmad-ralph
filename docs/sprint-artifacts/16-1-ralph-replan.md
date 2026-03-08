# Story 16.1: ralph replan

Status: review

## Story

As a разработчик,
I want запустить `ralph replan` в середине спринта,
so that пересчитать незавершённые задачи без потери выполненных.

## Acceptance Criteria

1. **`ralph replan` при существующем `sprint-tasks.md`** с [x] и [ ] задачами → выполненные [x] сохранены в начале файла

2. **Незавершённые задачи [ ]** заменены новыми на основе текущих входных документов

3. **Реализуется через `plan.Run`** с `PlanOpts.Merge = false` и передачей списка выполненных задач в промпт

4. **Exit codes соответствуют `ralph plan`** (0/1/2/3)

## Tasks / Subtasks

- [x] Task 1: Создать cobra subcommand `replan` (AC: #1-#4)
  - [x] Новый `cmd/ralph/replan.go`
  - [x] Переиспользует `plan/` пакет без рефакторинга
  - [x] Промпт: условный блок Replan Mode в `plan/prompts/plan.md`
- [x] Task 2: Тесты

## Dev Notes

- **Growth story** — после Epic 15 gate
- `ralph replan` = новый cobra subcommand, переиспользует plan/ без рефакторинга
- Промпт для replan: отдельный `plan/prompts/replan.md` или параметр в plan.md

### References

- [Source: docs/epics.md#Story 16.1] — полные AC
- [Source: docs/epics.md#FR28] — ralph replan

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Создан cobra subcommand `replan` в `cmd/ralph/replan.go`
- Добавлено поле `CompletedTasks` в `PlanOpts` и `templateData`
- `GeneratePrompt` расширен 5-м параметром `completedTasks`
- Условный блок `Replan Mode` в `plan/prompts/plan.md`
- `extractCompletedTasks()` фильтрует `[x]` строки из существующего файла
- Completed tasks prepend к выходу через `plan.Run`
- Exit codes соответствуют `ralph plan` (переиспользуется тот же `exitCode()`)

### File List

- `cmd/ralph/replan.go` (new) — cobra subcommand, extractCompletedTasks, runReplan
- `cmd/ralph/main.go` (mod) — registered replanCmd in init()
- `cmd/ralph/cmd_test.go` (mod) — TestReplanCmd, TestExtractCompletedTasks, TestRunReplan tests
- `plan/plan.go` (mod) — CompletedTasks field, GeneratePrompt 5th param, prepend logic
- `plan/plan_test.go` (mod) — updated GeneratePrompt calls, TestPlanPrompt_Generate_Replan
- `plan/prompts/plan.md` (mod) — Replan Mode conditional block

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/2L

#### [MEDIUM] M1: runReplanWithInputs DRY violation — ~70 строк копируют runPlanWithInputs

`runReplanWithInputs` (replan.go:76-148) почти полностью дублирует `runPlanWithInputs` (plan.go:88-174): size check, output path resolution, noReview flag, progress output, summary. Отличия: нет --merge, чтение existing + extractCompletedTasks, и формат summary. Fix: извлечь общий helper `runWithOpts(cmd, cfg, inputs, replanMode)` или передавать различия через параметры.

#### [MEDIUM] M2: GeneratePrompt сигнатура с 5 positional string/bool params

`GeneratePrompt(inputs, outputPath, merge, existing, completedTasks)` — 5 параметров, из которых `existing` и `completedTasks` оба string, легко перепутать. Fix: struct `GenerateOpts` с named fields.

#### [LOW] L1: extractCompletedTasks теряет section headers

`extractCompletedTasks` извлекает только `[x]` строки, без заголовков `## Story N.N`. Completed tasks в prompt будут flat list без структуры. Для MVP acceptable, но при сложных sprint-tasks.md потеряется контекст.

#### [LOW] L2: Doc comment PlanInput.Content stale после replan

`plan/plan.go:43`: "populated by cmd/ralph/plan.go via os.ReadFile". Теперь также заполняется из `cmd/ralph/replan.go`. Fix: "populated by cmd/ layer via os.ReadFile".

### Carry-over Fixes Verification

- M1 from 15.2 (Growth decision not in sprint-status.yaml): FIXED — `growth_decision: proceed` добавлен на строке 205
- M2 from 15.2 (File List missing sprint-status.yaml): N/A — другая story

### New Patterns Discovered

0 new patterns.
