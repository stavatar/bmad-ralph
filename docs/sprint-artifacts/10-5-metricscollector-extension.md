# Story 10.5: MetricsCollector Extension

Status: Ready for Review

## Story

As a разработчик,
I want чтобы MetricsCollector накапливал compactions и fill%,
so that эти данные были доступны в TaskMetrics и RunMetrics.

## Acceptance Criteria

### AC1: taskAccumulator new fields (FR87)
- `runner/metrics.go` — `taskAccumulator` struct получает:
  - `totalCompactions int`
  - `maxContextFillPct float64`
- Инициализируются в 0/0.0 на `StartTask`

### AC2: RecordSession — new signature (FR87, FR88)
- Текущая сигнатура:
  ```go
  func (mc *MetricsCollector) RecordSession(metrics, model, stepType, durationMs) string
  ```
- Новая сигнатура:
  ```go
  func (mc *MetricsCollector) RecordSession(metrics *session.SessionMetrics, model, stepType string, durationMs int64, compactions int, contextFillPct float64) string
  ```
- Return type по-прежнему `string` (resolved model)
- Existing accumulation logic preserved

### AC3: RecordSession — accumulates compactions (FR87)
- Задача начата, 3 сессии:
  - session 1: compactions=0, fillPct=30.0
  - session 2: compactions=1, fillPct=55.0
  - session 3: compactions=0, fillPct=42.0
- `current.totalCompactions == 1` (sum)
- `current.maxContextFillPct == 55.0` (max)

### AC4: RecordSession — nil collector (FR87)
- `mc.current == nil`
- RecordSession с compactions=2, fillPct=50.0
- Returns model (no panic, graceful)

### AC5: FinishTask — copies to TaskMetrics (FR87)
- `taskAccumulator` с `totalCompactions=2, maxContextFillPct=65.3`
- `FinishTask("done", "abc123")` →
  - `TaskMetrics.TotalCompactions == 2`
  - `TaskMetrics.MaxContextFillPct == 65.3`

### AC6: TaskMetrics new fields (FR87)
- `TaskMetrics` struct получает:
  ```go
  TotalCompactions  int     `json:"total_compactions"`
  MaxContextFillPct float64 `json:"max_context_fill_pct"`
  ```
- Поля сериализуются в JSON корректно

### AC7: RunMetrics new fields (FR88)
- `RunMetrics` struct получает:
  ```go
  TotalCompactions  int     `json:"total_compactions"`
  MaxContextFillPct float64 `json:"max_context_fill_pct"`
  ```
- Поля сериализуются в JSON корректно

### AC8: Finish — aggregates across tasks (FR88)
- 2 задачи:
  - task 1: compactions=1, maxFillPct=55.0
  - task 2: compactions=3, maxFillPct=42.0
- `Finish()` →
  - `RunMetrics.TotalCompactions == 4` (sum)
  - `RunMetrics.MaxContextFillPct == 55.0` (max)

### AC9: All existing RecordSession callers updated
- 6 call sites в `runner.go` + 1 в `serena.go`
- Где compactions/fillPct not available: pass `0, 0.0`
- Existing behavior preserved

## Tasks / Subtasks

- [x] Task 1: Extend taskAccumulator (AC: #1)
  - [x] 1.1 Добавить `totalCompactions int` и `maxContextFillPct float64`

- [x] Task 2: Extend RecordSession signature (AC: #2-#4)
  - [x] 2.1 Добавить `compactions int, contextFillPct float64` параметры
  - [x] 2.2 Accumulate: `mc.current.totalCompactions += compactions`
  - [x] 2.3 Max tracking: `if contextFillPct > mc.current.maxContextFillPct`
  - [x] 2.4 Nil guard: existing `mc.current == nil` check covers new params

- [x] Task 3: Extend TaskMetrics + FinishTask (AC: #5, #6)
  - [x] 3.1 Добавить поля в `TaskMetrics`
  - [x] 3.2 Скопировать в `FinishTask`

- [x] Task 4: Extend RunMetrics + Finish (AC: #7, #8)
  - [x] 4.1 Добавить поля в `RunMetrics`
  - [x] 4.2 Агрегировать в `Finish()`: sum compactions, max fillPct

- [x] Task 5: Update all callers (AC: #9)
  - [x] 5.1 Найти все `RecordSession(` call sites
  - [x] 5.2 Добавить `, 0, 0.0` к каждому вызову
  - [x] 5.3 Verify compilation

- [x] Task 6: Тесты (AC: #1-#9)
  - [x] 6.1 RecordSession accumulation test (sum compactions, max fillPct)
  - [x] 6.2 FinishTask copies test
  - [x] 6.3 Finish aggregation across 2+ tasks
  - [x] 6.4 Nil collector test
  - [x] 6.5 Verify existing tests pass with updated signature

## Dev Notes

### RecordSession — добавить после existing logic
```go
// Context window metrics
mc.current.totalCompactions += compactions
if contextFillPct > mc.current.maxContextFillPct {
    mc.current.maxContextFillPct = contextFillPct
}
```

### FinishTask — добавить в TaskMetrics construction
```go
tm := TaskMetrics{
    // ... existing fields ...
    TotalCompactions:  mc.current.totalCompactions,
    MaxContextFillPct: mc.current.maxContextFillPct,
}
```

### Finish — агрегация в RunMetrics
```go
var totalCompactions int
var maxFillPct float64
for _, t := range mc.tasks {
    totalCompactions += t.TotalCompactions
    if t.MaxContextFillPct > maxFillPct {
        maxFillPct = t.MaxContextFillPct
    }
}
// В RunMetrics:
TotalCompactions:  totalCompactions,
MaxContextFillPct: maxFillPct,
```

### 7 call sites для обновления
Все в `runner/runner.go` и `runner/serena.go`. Найти через grep `RecordSession(`. Добавить `, 0, 0.0` — в Story 10.6 эти нули заменятся на реальные данные.

### Existing tests
Все тесты в `runner/metrics_test.go` нужно обновить: добавить `, 0, 0.0` к `RecordSession` calls.

### Project Structure Notes

- Файл: `runner/metrics.go`
- Тесты: `runner/metrics_test.go`
- Callers: `runner/runner.go` (6 мест), `runner/serena.go` (1 место)
- Dependency direction: без изменений

### References

- [Source: docs/prd/context-window-observability.md#FR87] — TaskMetrics fields
- [Source: docs/prd/context-window-observability.md#FR88] — RunMetrics fields
- [Source: docs/architecture/context-window-observability.md#Решение 4] — MetricsCollector Extension
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.5] — AC, technical notes
- [Source: runner/metrics.go:62-94] — existing TaskMetrics struct
- [Source: runner/metrics.go:96-116] — existing RunMetrics struct
- [Source: runner/metrics.go:117-132] — existing taskAccumulator struct
- [Source: runner/metrics.go:187-221] — existing RecordSession function

## Testing Standards

- Table-driven tests с `[]struct{name string; ...}` + `t.Run`
- Go stdlib assertions — без testify
- Verify ALL struct fields in TaskMetrics/RunMetrics (not just counts)
- Float comparison: exact equality OK here (max tracking, not arithmetic)
- Call count assertions: verify sessions count matches expected
- Naming: `TestMetricsCollector_RecordSession_ContextMetrics`, `TestMetricsCollector_Finish_ContextAggregation`

## Dev Agent Record

### Context Reference
- Story: docs/sprint-artifacts/10-5-metricscollector-extension.md
- Source: runner/metrics.go (taskAccumulator, RecordSession, FinishTask, Finish, TaskMetrics, RunMetrics)

### Agent Model Used
claude-opus-4-6

### Debug Log References
N/A

### Completion Notes List
- Extended taskAccumulator with totalCompactions/maxContextFillPct fields
- Extended RecordSession signature: added compactions int, contextFillPct float64
- Accumulation: sum compactions, max fillPct per task
- Extended TaskMetrics and RunMetrics with new JSON-tagged fields
- FinishTask copies accumulator → TaskMetrics
- Finish() aggregates across tasks: sum compactions, max fillPct
- Updated 4 callers in runner.go with `, 0, 0.0`
- Updated 21 existing RecordSession calls in metrics_test.go
- 3 new test functions: accumulation, nil collector, cross-task aggregation

### File List
- runner/metrics.go
- runner/runner.go
- runner/metrics_test.go
