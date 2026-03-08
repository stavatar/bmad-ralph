# Story 12.1: plan/prompts/review.md — reviewer prompt

Status: Done

## Review Findings (1H/3M/1L)

### HIGH
- [H1] `plan/plan_test.go:358` — `errors.As` используется без `import "errors"`. Весь пакет plan/ не компилируется, тесты не запускаются. **MUST FIX: добавить `"errors"` в import block**

### MEDIUM
- [M1] `ExistingContent []byte` в PlanOpts (строка 73) — dead field, не используется. `Run()` по-прежнему делает `os.ReadFile` напрямую (H1 из Story 11.5 не исправлен) `[plan/plan.go:73,107]`
- [M2] `GenerateReviewPrompt` и `GeneratePrompt` дублируют Stage 1 + Stage 2 assembly логику (20+ строк). Извлечь общий helper `[plan/plan.go:237-278 vs 185-231]`
- [M3] TestPlanPrompt_Review проверяет 5 из 7 checklist items — пропущены "Task Ordering" и "Task Independence" `[plan/plan_test.go:448]`

### LOW
- [L1] `time.Duration(0)` на строках 135, 162 — нестандартная запись, обычно просто `0` `[plan/plan.go:135,162]`

## Story

As a система,
I want иметь промпт для reviewer,
so that Claude в чистой сессии объективно оценивает качество сгенерированного плана.

## Acceptance Criteria

1. **Файл `plan/prompts/review.md` как `text/template`** — промпт инструктирует reviewer проверить:
   - Покрытие всех FR из входных документов
   - Гранулярность задач (не слишком крупные, не слишком мелкие)
   - Корректность source-ссылок (реальные имена файлов из typed headers)
   - Наличие SETUP и GATE задач
   - Отсутствие дублирования и противоречий

2. **Промпт требует ответ в формате:** `OK` или `ISSUES:\n<список проблем>`

3. **`plan/plan_test.go` содержит `TestPlanPrompt_Review`** с golden file `plan/testdata/TestPlanPrompt_Review.golden`

4. **Тест проходит с `-update` и без**

## Tasks / Subtasks

- [x] Task 1: Создать `plan/prompts/review.md` (AC: #1, #2)
  - [x] Инструкции reviewer: FR coverage, гранулярность, source-ссылки, SETUP/GATE, дубли, ordering, independence
  - [x] Формат ответа: `OK` или `ISSUES:\n<список>`
  - [x] Шаблон принимает `templateData` (те же поля что plan.md)
  - [x] Сгенерированный план передаётся через `__PLAN__` placeholder (strings.Replace)
- [x] Task 2: Создать golden file и тест (AC: #3, #4)
  - [x] `TestPlanPrompt_Review` в `plan/plan_test.go`
  - [x] Golden file: `plan/testdata/TestPlanPrompt_Review.golden`
  - [x] `-update` flag pattern — passes both with and without -update

## Dev Notes

### Архитектурные ограничения

- **Двухэтапная Prompt Assembly:** template для структуры, strings.Replace для content [Source: docs/project-context.md#Двухэтапная Prompt Assembly]
- **Reviewer получает сгенерированный план** как часть промпта через strings.Replace [Source: docs/epics.md#Story 12.1 Technical Notes]
- **Формат ответа строго определён** — gate parsing (Story 12.2) зависит от него

### Существующий паттерн

- `plan/prompts/plan.md` (Story 11.4) — generator template, reference для review template
- `runner/prompts/review.md` — существующий review prompt для `ralph run` (reference)

### Формат ответа reviewer

```
OK
```
или
```
ISSUES:
- FR3 не покрыт: нет задачи для single-doc режима
- Задача "настройка БД" слишком крупная — разбить на миграцию и seed
```

Gate parsing в Story 12.2: `strings.HasPrefix(strings.TrimSpace(output), "OK")`

### Тестирование

- Golden file pattern — аналог `TestPlanPrompt_Generate` [Source: CLAUDE.md#Testing Core Rules]
- Template assertions: scope guards, constraint text [Source: .claude/rules/test-assertions-prompt.md]

### Project Structure Notes

- `plan/prompts/review.md` — НОВЫЙ файл (template)
- `plan/plan_test.go` — расширение: `TestPlanPrompt_Review`
- `plan/testdata/TestPlanPrompt_Review.golden` — НОВЫЙ golden file

### References

- [Source: docs/epics.md#Story 12.1] — полные AC и технические заметки
- [Source: docs/epics.md#FR14] — auto review в чистой сессии
- [Source: docs/epics.md#FR15] — reviewer проверяет FR coverage, гранулярность, source-refs
- [Source: docs/project-context.md#Двухэтапная Prompt Assembly] — двухэтапный рендер
- [Source: runner/prompts/review.md] — reference review prompt

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created plan/prompts/review.md with 7-item review checklist
- Review checks: FR Coverage, Granularity, Source References, SETUP/GATE, No Duplication, Task Ordering, Task Independence
- Response format: "OK" or "ISSUES:\n- <list>"
- Added //go:embed for reviewPrompt, ReviewPrompt() accessor
- Added GenerateReviewPrompt() with two-stage assembly (__CONTENT_N__ + __PLAN__)
- TestPlanPrompt_Review: verifies typed headers, content injection, plan injection, all 5 checklist items, response format
- Golden file generated and verified (passes with and without -update)
- Full regression: go test ./... -count=1 — all packages pass, 0 regressions

### File List

- plan/prompts/review.md (new): reviewer prompt template
- plan/plan.go (modified): reviewPrompt embed, ReviewPrompt(), GenerateReviewPrompt()
- plan/plan_test.go (modified): TestPlanPrompt_Review
- plan/testdata/TestPlanPrompt_Review.golden (new): golden file
