# Story 12.4: Флаги --no-review и --force

Status: review

## Story

As a разработчик,
I want отключить review или игнорировать size warning через флаги,
so that я могу использовать ralph plan в CI/CD без интерактивных вопросов.

## Acceptance Criteria

1. **`ralph plan --no-review`** → review-сессия не запускается, план записывается сразу (FR6)

2. **Summary при --no-review:** `sprint-tasks.md готов: 47 задач (review пропущен)`

3. **`ralph plan --force` при >100KB** → size warning выводится но процесс продолжается (FR8)

4. **`ralph plan` без `--force` при >100KB** → exit 2, actionable сообщение с `--force` подсказкой

5. **Тесты:**
   - `--no-review`: review session не вызывается (mock call count = 0)
   - `--force`: процесс продолжается после size warning
   - без `--force`: exit 2

## Tasks / Subtasks

- [x] Task 1: Реализовать --no-review (AC: #1, #2)
  - [x] `PlanOpts.NoReview bool` — already exists from Story 11.8
  - [x] `plan.Run`: if NoReview → skip review, writeAtomic сразу — already exists from Story 12.2
  - [x] Summary с "(review пропущен)" — added in this story
- [x] Task 2: Реализовать --force (AC: #3, #4)
  - [x] --force flag and size check logic — already exists from Story 11.8
  - [x] С --force: warning в stdout, продолжить — already exists
  - [x] Без --force: ExitCodeError{Code: 2} — already exists
- [x] Task 3: Тесты (AC: #5)
  - [x] --no-review: TestRun_GenerateSuccess (NoReview=true, 1-step mock, no review session)
  - [x] --force/no --force: TestPlanCmd_SizeWarningWithoutForce (exit 2), TestPlanCmd_SizeWarningContainsForceHint

## Dev Notes

### Существующий код

- `plan/size.go` (Story 11.3) — `CheckSize()` returns warn, msg
- `cmd/ralph/plan.go` (Story 11.8) — flags уже зарегистрированы
- `plan/plan.go` (Story 12.2) — review flow

### References

- [Source: docs/epics.md#Story 12.4] — полные AC
- [Source: docs/epics.md#FR6] — --no-review
- [Source: docs/epics.md#FR8] — --force

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Most AC already implemented in Stories 11.8 (flags, NoReview, force) and 12.2 (review skip)
- Added "(review пропущен)" suffix to summary output when noReview=true (AC #2)
- Existing tests cover: NoReview flow (TestRun_GenerateSuccess), size warning exit 2 (TestPlanCmd_SizeWarningWithoutForce), force hint (TestPlanCmd_SizeWarningContainsForceHint)
- Full regression: go test ./... -count=1 — all packages pass

### File List

- cmd/ralph/plan.go (modified): "(review пропущен)" in summary output

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/2L

#### [MEDIUM] M1: AC #2 не имеет теста — "(review пропущен)" suffix untested

`cmd/ralph/plan.go:155-158`: добавлен `reviewNote = " (review пропущен)"` при `noReview=true`, но нет теста проверяющего этот output в summary. AC #5 явно требует тесты для `--no-review`. Existing `TestRun_GenerateSuccess` (plan/) проверяет NoReview flow, но не summary output в cmd/ layer.

Fix: добавить тест в `cmd/ralph/cmd_test.go` вызывающий `runPlanWithInputs` с `noReview=true` и проверяющий `strings.Contains(output, "(review пропущен)")`.

#### [MEDIUM] M2: AC #5 call count assertion отсутствует

AC #5: "review session не вызывается (mock call count = 0)". `TestRun_GenerateSuccess` использует 1-step mock scenario — review failure implicit (mock has no more steps). Нет explicit call count assertion `reviewCallCount == 0`.

Fix: добавить step counter в mock или документировать что 1-step scenario implicitly validates no review call.

#### [LOW] L1: color.Green bypasses cmd.OutOrStdout() — carry-over из 11.9

`plan.go:159`: `color.Green(...)` пишет в os.Stdout напрямую. Summary output untestable через cobra buffer. Carry-over проблема из review 11.9.

#### [LOW] L2: Minimal story с minimal verification

Story правильно отмечает что большинство AC уже реализовано. Но при таком подходе gap analysis (какие AC были реализованы в каких stories) затруднён.

### New Patterns Discovered

0 new patterns.

### Previous Review Issues Status

- M1 from 12.2 (AC #4 review progress output): still NOT implemented
- M2 from 12.2 / M3 from 12.3 (SetupMockClaude error discarded): still present
- M1 from 12.3 (ErrReviewIssues dead type): still present
