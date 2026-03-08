# Story 11.3: plan/size.go — CheckSize pure function

Status: Done

## Review Findings (0H/3M/1L)

### MEDIUM
- [M1] Тест "above threshold" не проверяет KB-значение в msg `[plan/size_test.go:39-47]`
- [M2] Нет кейса empty slice `[]PlanInput{}` — только nil `[plan/size_test.go:16-21]`
- [M3] Тест не проверяет русский текст в msg ("суммарный размер", "лимит") `[plan/size_test.go]`

### LOW
- [L1] `wantEmpty bool` field избыточен — выводится из `!wantWarn` `[plan/size_test.go:13]`

## Story

As a система,
I want проверять суммарный размер входных документов,
so that пользователь получает предупреждение до отправки слишком больших данных в Claude.

## Acceptance Criteria

1. **Функция `CheckSize(inputs []PlanInput) (warn bool, msg string)`** в `plan/size.go`

2. **Сумма `len(inp.Content)` ≤ `MaxInputBytes` (100_000)** → `warn == false`, `msg == ""`

3. **Сумма > `MaxInputBytes`** → `warn == true`, `msg` содержит суммарный размер в KB и рекомендацию `bmad shard-doc`

4. **`plan/size.go` содержит `const MaxInputBytes = 100_000`**

5. **`plan/size_test.go` покрывает:** below threshold, exactly threshold, above threshold, empty inputs

6. **`go test ./plan/... -count=1` проходит**

## Tasks / Subtasks

- [x] Task 1: Создать `plan/size.go` (AC: #1, #4)
  - [x] Определить `const MaxInputBytes = 100_000`
  - [x] Реализовать `CheckSize(inputs []PlanInput) (warn bool, msg string)`
  - [x] При сумме > MaxInputBytes: формат msg `"суммарный размер входных документов: %dKB (лимит ~100KB). Рекомендуется разбить через 'bmad shard-doc'"`
- [x] Task 2: Создать `plan/size_test.go` (AC: #2, #3, #5, #6)
  - [x] Тест: below threshold → warn=false, msg=""
  - [x] Тест: exactly threshold (100_000 bytes) → warn=false, msg=""
  - [x] Тест: above threshold (100_001+) → warn=true, msg содержит размер в KB и "shard-doc"
  - [x] Тест: empty inputs (nil/empty slice) → warn=false, msg=""
  - [x] Table-driven формат

## Dev Notes

### Архитектурные ограничения

- **Pure function:** без I/O, без зависимостей кроме stdlib [Source: docs/epics.md#Story 11.3 Technical Notes]
- **Без `os.WriteFile` в `plan/size.go`:** запись только через `writeAtomic()` в `plan/plan.go` [Source: docs/project-context.md#Plan Package Инварианты]
- **Threshold по `len(inp.Content)`**, не по размеру файла на диске [Source: docs/epics.md#Story 11.3]

### Существующий код

- `plan/plan.go` (создаётся в Story 11.1) содержит тип `PlanInput` с полем `Content []byte`
- `CheckSize` использует `PlanInput` из того же пакета `plan`

### Формат сообщения

```go
msg = fmt.Sprintf("суммарный размер входных документов: %dKB (лимит ~100KB). Рекомендуется разбить через 'bmad shard-doc'", totalBytes/1024)
```

### Тестирование

- Table-driven, Go stdlib assertions [Source: CLAUDE.md#Testing Core Rules]
- Test naming: `TestCheckSize_BelowThreshold`, `TestCheckSize_ExactlyThreshold`, `TestCheckSize_AboveThreshold`, `TestCheckSize_EmptyInputs` или единый table-driven `TestCheckSize_Scenarios`
- Пограничный случай: ровно 100_000 bytes = NOT warning (≤)

### Project Structure Notes

- `plan/size.go` — НОВЫЙ файл в пакете `plan` (создан в Story 11.1)
- `plan/size_test.go` — НОВЫЙ тест-файл

### References

- [Source: docs/epics.md#Story 11.3] — полные AC и технические заметки
- [Source: docs/project-context.md#Plan Package] — `CheckSize` архитектура, `MaxInputBytes` константа
- [Source: docs/project-context.md#Инварианты] — запрет `os.WriteFile` в size.go
- [Source: CLAUDE.md#Testing Core Rules] — table-driven, stdlib, coverage

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created `plan/size.go` with `MaxInputBytes = 100_000` constant and `CheckSize` pure function
- Created `plan/size_test.go` with table-driven `TestCheckSize_Scenarios` (5 cases: empty, below, exactly, above, multiple above)
- Full regression: `go test ./... -count=1` — all packages pass, 0 regressions

### File List

- plan/size.go (new): MaxInputBytes constant, CheckSize function
- plan/size_test.go (new): TestCheckSize_Scenarios table-driven test
