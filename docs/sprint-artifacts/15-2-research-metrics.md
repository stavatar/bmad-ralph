# Story 15.2: Research — метрики качества плана

Status: review

## Story

As a разработчик,
I want иметь объективные метрики качества `ralph plan`,
so that принимать решение о запуске Growth фич на основе данных.

## Acceptance Criteria

1. **Документ `docs/research/plan-quality-metrics.md` содержит:**
   - Определение "хорошего плана" (измеримые критерии)
   - Как автоматически измерить качество
   - Пороговые значения для входа в Growth
   - Рекомендация: продолжать ли Growth фичи

2. **Решение о запуске Growth зафиксировано** в `docs/sprint-artifacts/sprint-status.yaml`

## Tasks / Subtasks

- [x] Task 1: Определить метрики качества (AC: #1)
- [x] Task 2: Написать документ (AC: #1)
- [x] Task 3: Зафиксировать решение (AC: #2)

## Dev Notes

- **Research story** — не требует кода, требует анализа и документирования
- Вопрос из PRD: "Как автоматически измерить качество плана?"

### References

- [Source: docs/epics.md#Story 15.2] — полные AC

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created docs/research/plan-quality-metrics.md with:
  - Definition of "good plan" with 4 measurable dimensions
  - 3 levels of metrics: fully automatic (L1), LLM-as-judge (L2), runtime (L3)
  - Threshold values for Growth entry (minimum + recommended)
  - Recommendation: proceed with Growth features
- Decision: continue Growth при выполнении минимальных порогов L1 + acceptance test

### File List

- docs/research/plan-quality-metrics.md (new): research document on plan quality metrics

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/1L

#### [MEDIUM] M1: AC #2 не выполнен — решение не зафиксировано в sprint-status.yaml

AC #2: "Решение о запуске Growth зафиксировано в sprint-status.yaml". В sprint-status.yaml нет записи о Growth decision — только стандартный статус story `15-2-research-plan-quality-metrics: review`. Рекомендация есть в research документе, но AC требует фиксацию именно в sprint-status.yaml (например, `growth_decision: proceed`).

Fix: добавить секцию `growth_decision` или комментарий в sprint-status.yaml.

#### [MEDIUM] M2: File List не включает sprint-status.yaml

Completion Notes упоминают "Decision: continue Growth", но File List содержит только plan-quality-metrics.md. Если sprint-status.yaml был изменён (или должен быть) — он должен быть в File List.

#### [LOW] L1: Порог task_count >= 10 не обоснован

Research документ устанавливает порог `task_count >= 10`, но не объясняет выбор числа. Для реального проекта (bmad-ralph) bridge генерировал десятки задач из одной story. Порог может быть слишком низким. Minor — research документ допускает приблизительные значения.

### Carry-over Fixes Verification

- M1 from 14.2 (stale bridge comments): FIXED — grep "bridge" в 5 файлах = 0 результатов
- M2 from 14.2 (duplicate section comment): FIXED — cmd_test.go:324 содержит один comment

### New Patterns Discovered

0 new patterns.
