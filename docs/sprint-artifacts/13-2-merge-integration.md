# Story 13.2: Merge mode интеграция

Status: review

## Story

As a разработчик,
I want запустить `ralph plan --merge` и получить обновлённый `sprint-tasks.md` с новыми задачами,
so that выполненные задачи не теряются.

## Acceptance Criteria

1. **`ralph plan --merge` при существующем `sprint-tasks.md`** → `cmd/ralph/plan.go` читает существующий файл → передаёт как `PlanInput` с ролью `existing_plan`

2. **`plan.Run` вызывает `MergeInto(existing, generated)`** вместо прямой записи

3. **Результат записывается через `writeAtomic`** (атомарно)

4. **При `--merge` без существующего файла** → merge как создание нового (не ошибка)

5. **Summary:** `sprint-tasks.md обновлён: +12 новых задач | path/to/sprint-tasks.md`

6. **Тест через mock сценарий `merge_mode`:** проверяет что `MergeInto` вызван, [x] задачи не модифицированы

## Tasks / Subtasks

- [x] Task 1: Интеграция merge в plan.Run (AC: #1-#4)
  - [x] При opts.Merge + ExistingContent: MergeInto → writeAtomic
  - [x] Missing file: handled by cmd/ralph/plan.go (os.IsNotExist → nil ExistingContent)
  - [x] Empty existing: MergeInto returns generated as-is
- [x] Task 2: Summary для merge (AC: #5)
  - [x] countTasks helper, diff existing vs result for "+N новых задач"
- [x] Task 3: Тесты (AC: #6)
  - [x] TestRun_MergeMode: verifies dedup, new story appended, [x] preserved

## Dev Notes

### Архитектурные ограничения

- **`plan.Run` при merge — единственное исключение** из правила "plan/ не читает файлы": `os.ReadFile(opts.OutputPath)` [Source: docs/epics.md#Story 13.2 Technical Notes]
- **Подсчёт новых задач:** `strings.Count` разница между existing и result

### References

- [Source: docs/epics.md#Story 13.2] — полные AC
- [Source: docs/epics.md#FR4] — merge не трогает [x]
- [Source: docs/epics.md#FR5] — пропуск существующих stories
- [Source: plan/merge.go] — MergeInto (Story 13.1)

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added merge path in plan.Run(): if opts.Merge && ExistingContent → MergeInto(existing, generated)
- cmd/ralph/plan.go: merge summary "sprint-tasks.md обновлён: +N новых задач"
- countTasks() helper for task count diff
- Existing content read in cmd/ralph/plan.go (already from Story 11.8 reviewer)
- Missing file handled: os.IsNotExist → nil ExistingContent → MergeInto returns generated
- TestRun_MergeMode: mock scenario, verifies dedup (Story 1.1 count=1), new story appended, [x] preserved, dup task absent
- Full regression: go test ./... -count=1 — all packages pass

### File List

- plan/plan.go (modified): merge call before writeAtomic
- cmd/ralph/plan.go (modified): merge summary, countTasks helper
- plan/plan_test.go (modified): TestRun_MergeMode

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/2L

#### [MEDIUM] M1: countTasks helper не используется в строке 154

`cmd/ralph/plan.go:154`: `taskCount := strings.Count(content, config.TaskOpen) + strings.Count(content, config.TaskDone)` — дублирует `countTasks()` helper (строка 174). DRY violation: helper создан для merge branch (строка 160), но non-merge branch (строка 154) дублирует ту же логику inline.

Fix: `taskCount := countTasks(content)` на строке 154.

#### [MEDIUM] M2: newTasks может быть отрицательным

`cmd/ralph/plan.go:161`: `newTasks := taskCount - existingTaskCount`. Если merge bug или edge case приводит к уменьшению задач, output будет `+%-3 новых задач`. Нет guard `if newTasks < 0 { newTasks = 0 }` или альтернативного сообщения.

Fix: добавить guard для negative diff или показывать "sprint-tasks.md обновлён: N задач" без `+`.

#### [LOW] L1: Dev Notes содержат stale claim

Dev Notes строка 40: "plan.Run при merge — единственное исключение из правила 'plan/ не читает файлы': os.ReadFile(opts.OutputPath)". Неверно — plan.go:167 использует `opts.ExistingContent`, НЕ `os.ReadFile`. Dev Notes отражают ранний design, не финальную реализацию.

#### [LOW] L2: AC #1 формулировка расходится с реализацией

AC #1: "передаёт как PlanInput с ролью existing_plan". Реализация: ExistingContent []byte поле. Технически правильный подход (проще), но AC буквально не выполнен. Cosmetic — AC нуждается в обновлении.

### New Patterns Discovered

0 new patterns.
