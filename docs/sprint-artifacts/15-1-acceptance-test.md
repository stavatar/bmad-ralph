# Story 15.1: Acceptance test на learnPracticsCodePlatform

Status: ready-for-dev

## Story

As a разработчик,
I want запустить `ralph plan` на реальном BMad-проекте и сравнить output с предыдущим bridge,
so that объективно подтвердить качество.

## Acceptance Criteria

1. **`ralph plan docs/` выполнен** на learnPracticsCodePlatform — `sprint-tasks.md` сгенерирован без ошибок

2. **Количество битых source-ссылок = 0** (было 100% с bridge, FR20)

3. **Задач не меньше** чем в `sprint-tasks.old.md` (предыдущий bridge output, 295 строк)

4. **Результаты задокументированы** в `docs/sprint-artifacts/acceptance-test-results.md`:
   - Количество задач: было X, стало Y
   - Битые ссылки: 0
   - Стоимость запуска: $N
   - Время выполнения: N сек

## Tasks / Subtasks

- [ ] Task 1: Запустить ralph plan на learnPracticsCodePlatform (AC: #1)
- [ ] Task 2: Проверить source-ссылки (AC: #2)
- [ ] Task 3: Сравнить с baseline (AC: #3)
- [ ] Task 4: Документировать результаты (AC: #4)

## Dev Notes

- **Research/manual story** — не требует кода, требует ручного выполнения и анализа
- Сравнение с `sprint-tasks.old.md` — через diff
- Метрика "битые ссылки": grep source-ref на несуществующие файлы

### References

- [Source: docs/epics.md#Story 15.1] — полные AC
- [Source: docs/epics.md#FR20] — typed headers, 0 битых ссылок

## Dev Agent Record

### Context Reference

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
