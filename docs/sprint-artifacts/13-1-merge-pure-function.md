# Story 13.1: plan/merge.go — MergeInto pure function

Status: review

## Story

As a система,
I want иметь функцию для слияния нового плана с существующим,
so that атомарно добавляются только новые задачи без потери выполненных.

## Acceptance Criteria

1. **`MergeInto(existing, generated []byte) ([]byte, error)`** в `plan/merge.go`

2. **Дедупликация:** если `existing` и `generated` содержат `## Story 1.1` — результат содержит ровно один раз (FR5)

3. **Новая story:** `## Story 2.1` из `generated` отсутствует в `existing` → добавляется в конец

4. **Выполненные задачи `- [x]`** из `existing` не модифицируются (FR4)

5. **Порядок:** весь `existing` → новые stories из `generated` (append-only)

6. **Тесты `plan/merge_test.go`:**
   - Дедупликация story: дубль пропускается
   - Новая story: добавляется в конец
   - Пустой existing: результат = generated
   - Пустой generated: результат = existing
   - Выполненные [x] задачи остаются нетронутыми

7. **`go test ./plan/... -count=1` проходит**

## Tasks / Subtasks

- [x] Task 1: Создать `plan/merge.go` (AC: #1-#5)
  - [x] Pure function: без I/O, зависимости: regexp, strings
  - [x] Дедупликация по regex `^## Story \d+\.\d+`
  - [x] Append-only: никогда не удалять строки из existing
- [x] Task 2: Тесты (AC: #6, #7)
  - [x] Table-driven 5 сценариев + 2 дополнительных теста

## Dev Notes

### Архитектурные ограничения

- **Pure function:** без I/O [Source: docs/project-context.md#Plan Package]
- **Без `os.WriteFile` в merge.go** — запись через writeAtomic в plan.go [Source: docs/project-context.md#Plan Package Инварианты]
- **Reference:** `bridge/bridge.go` Smart Merge (Epic 2, Story 2.6) — аналогичная логика

### References

- [Source: docs/epics.md#Story 13.1] — полные AC
- [Source: docs/epics.md#FR4] — merge не трогает [x]
- [Source: docs/epics.md#FR5] — пропуск существующих stories

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created plan/merge.go with MergeInto(existing, generated []byte) ([]byte, error)
- Pure function: no I/O, uses regexp + strings only
- Deduplication via storyHeaderRe matching "## Story N.N" headers
- Existing content preserved including [x] completed tasks
- New stories from generated appended after existing content
- splitBySections helper splits content by ## Story headers
- Edge cases: empty existing returns generated, empty generated returns existing
- TestMergeInto_Scenarios: 5 table-driven cases (empty existing, empty generated, dedup, new story, completed preserved)
- TestMergeInto_StoryCountDedup: verifies exact count = 1 for duplicate story
- TestMergeInto_OrderPreserved: verifies append order via string index comparison
- Full regression: go test ./... -count=1 — all packages pass

### File List

- plan/merge.go (new): MergeInto(), splitBySections(), storyHeaderRe
- plan/merge_test.go (new): TestMergeInto_Scenarios, TestMergeInto_StoryCountDedup, TestMergeInto_OrderPreserved

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/2L

#### [MEDIUM] M1: MergeInto never returns error — dead error path

`merge.go:16`: signature `([]byte, error)` но все пути возвращают `nil` error. Нет теста для error path (`wantErr` field exists but all false). Либо error return нужно оправдать будущим использованием (документировать), либо упростить signature до `[]byte`.

Fix: если error предусмотрен для валидации (e.g., malformed headers), добавить хотя бы один error case. Если нет — оставить signature для forward compatibility, но добавить doc comment объясняющий это.

#### [MEDIUM] M2: splitBySections пропускает preamble content

`merge.go:40-42`: content до первого `## Story` header отбрасывается (`if header == "" { continue }`). Если generated начинается с `# Sprint Tasks\n\n## Story 1.1...`, title `# Sprint Tasks` потеряется. AC не определяет поведение для preamble, но при merge mode preamble из generated может содержать важную info.

Fix: добавить тест для preamble behavior (even if it documents current "skip" behavior). Или сохранять preamble из existing.

#### [LOW] L1: storyHeaderRe partial match на nested versions

`merge.go:9`: `^## Story \d+\.\d+` совпадёт с `## Story 1.1.2` (partial match на `1.1`). Edge case, но regex точнее с `$` или `(?:\s|$)` после version.

#### [LOW] L2: SetupMockClaude строка 210 без комментария

`plan_test.go:210`: `_, _ = testutil.SetupMockClaude(t, scenario)` — единственное место без пояснительного комментария (6 других мест получили комментарий в carry-over fix).

### Carry-over Fixes Verification

- M1 from 12.3 (ErrReviewIssues dead type): FIXED — удалён из plan.go
- M2 from 12.3 (promptGate hardcodes os.Stderr): FIXED — принимает io.Writer parameter
- M3 from 12.3 (SetupMockClaude comments): PARTIALLY FIXED — 6/7 мест получили комментарий, строка 210 пропущена

### New Patterns Discovered

0 new patterns.
