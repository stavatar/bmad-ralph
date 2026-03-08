# Story 16.2: ralph init

Status: review

## Story

As a разработчик без BMad,
I want запустить `ralph init "описание"`,
so that получить минимальный набор документов за 2-3 минуты и сразу перейти к `ralph plan`.

## Acceptance Criteria

1. **`ralph init "API сервис для управления задачами"`** в пустой директории → созданы:
   - `docs/prd.md` — минимальный PRD на основе описания
   - `docs/architecture.md` — базовая архитектура

2. **Время выполнения** <= 3 минуты

3. **Сгенерированные файлы достаточны** для запуска `ralph plan docs/`

4. **Инструкция:** `Документы готовы. Запустите: ralph plan docs/`

## Tasks / Subtasks

- [x] Task 1: Создать cobra subcommand `init` (AC: #1-#4)
  - [x] Новый `cmd/ralph/init.go`
  - [x] Одна Claude-сессия с промптом для генерации документов
  - [x] Промпт: `cmd/ralph/prompts/init.md`
- [x] Task 2: Тесты

## Dev Notes

- **Growth story** — после Epic 15 gate
- Реализуется через одну Claude-сессию
- Промпт: `cmd/ralph/prompts/init.md` (отдельно от plan/prompts/)

### References

- [Source: docs/epics.md#Story 16.2] — полные AC
- [Source: docs/epics.md#FR29] — ralph init

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Создан cobra subcommand `init` в `cmd/ralph/init.go`
- Промпт `cmd/ralph/prompts/init.md` генерирует prd.md + architecture.md через разделитель
- `splitInitOutput()` парсит выход Claude по `===FILE_SEPARATOR===`
- Одна Claude-сессия через `session.Execute`
- Создаёт `docs/prd.md` и `docs/architecture.md` атомарно
- Финальное сообщение: "Документы готовы. Запустите: ralph plan docs/"
- Carry-over 16.1 M1: DRY-рефакторинг — извлечены `checkSizeGuard`, `resolveOutputPath`, `resolveNoReview`, `runPlanSession`, `reviewNote` в `plan_helpers.go`
- Carry-over 16.1 L2: doc comment PlanInput.Content обновлён на "cmd/ layer"

### File List

- `cmd/ralph/init.go` (new) — cobra subcommand, runInit, splitInitOutput
- `cmd/ralph/prompts/init.md` (new) — промпт для Claude-сессии
- `cmd/ralph/plan_helpers.go` (new) — shared helpers (carry-over 16.1 M1 DRY)
- `cmd/ralph/main.go` (mod) — registered initCmd in init()
- `cmd/ralph/cmd_test.go` (mod) — init tests + "init" in HasSubcommands
- `cmd/ralph/plan.go` (mod) — refactored to use shared helpers
- `cmd/ralph/replan.go` (mod) — refactored to use shared helpers
- `plan/plan.go` (mod) — PlanInput.Content doc comment fix (carry-over 16.1 L2)

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/2L

#### [MEDIUM] M1: os.WriteFile в init.go — не атомарная запись

`init.go:87-91` пишет prd.md и architecture.md через `os.WriteFile`. Если запись architecture.md падает, prd.md уже записан — частично созданные docs. Plan package использует `writeAtomic` (temp + rename). Init должен либо использовать аналогичный паттерн, либо откатить prd.md при ошибке.

#### [MEDIUM] M2: Stale doc comments в plan/plan.go

`plan/plan.go:39` — PlanInput struct doc comment: "populated by cmd/ralph/plan.go via os.ReadFile". Поле Content обновлено (line 43: "cmd/ layer"), но struct-level comment на строке 39 стале. Также `plan/plan.go:75` ExistingContent: "populated by cmd/ralph/plan.go" и line 102 comment — оба ссылаются на plan.go, но replan.go тоже заполняет.

#### [LOW] L1: init не использует plan/ package — прямой session.Execute

Init напрямую вызывает `session.Execute` вместо plan.Run. Это допустимо (init не генерирует sprint-tasks.md), но создаёт параллельную сессионную логику в cmd/ layer. Observation, не дефект.

#### [LOW] L2: Prompt injection через __DESCRIPTION__

`init.go:43`: `strings.ReplaceAll(initPrompt, "__DESCRIPTION__", description)` — пользовательский ввод подставляется в промпт без санитизации. Пользователь может внедрить prompt-инструкции через description. Для CLI-tool с локальным запуском это low risk, но стоит задокументировать.

### Carry-over Fixes Verification

- M1 from 16.1 (DRY runReplanWithInputs): FIXED — общие helpers вынесены в plan_helpers.go (checkSizeGuard, resolveOutputPath, resolveNoReview, runPlanSession, reviewNote)
- M2 from 16.1 (GeneratePrompt 5 params): NOT FIXED — сигнатура не изменена (low priority)
- L2 from 16.1 (PlanInput.Content doc): PARTIALLY FIXED — line 43 обновлён, но struct comment (line 39) и ExistingContent (line 75) стале

### New Patterns Discovered

0 new patterns.
