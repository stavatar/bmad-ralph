# Story 7.6: Gate Analytics

Status: review

## Story

As a разработчик,
I want чтобы ralph логировал каждое gate decision с timing (время от prompt до ответа),
so that анализировать паттерны принятия решений.

## Acceptance Criteria

1. **AC1: Gate timing measurement (FR53)**
   - Wall-clock time от gate prompt до user response измеряется в миллисекундах
   - `logger.Info("gate decision", kv("step_type", "gate"), kv("action", action), kv("wait_ms", N), kv("task", taskText))`

2. **AC2: Gate decision recorded in MetricsCollector (FR53)**
   - `MetricsCollector.RecordGate(stats GateStats)` вызывается (signature из Story 7.1 stub)
   - GateStats counters инкрементируются: `Approvals` (approve), `Rejections` (quit), `Skips` (skip). Retry не инкрементирует counter (retry = продолжение, не финальное решение)
   - `TotalWaitMs` accumulated, `TotalPrompts` incremented, `LastAction` set
   - `TaskMetrics.Gate` populated (тип `*GateStats`, уже определён в TaskMetrics)

3. **AC3: Emergency gate tracked separately**
   - Emergency gate (execute/review exhaustion) — same recording as normal gate
   - `step_type` в log == `"gate"` (одинаково для normal и emergency)

4. **AC4: Gate timing in Runner**
   - `t0 = time.Now()` перед вызовом `GatePromptFn`/`EmergencyGatePromptFn`
   - `elapsed = time.Since(t0)` после
   - `elapsed.Milliseconds()` передаётся в `RecordGate`

## Tasks / Subtasks

- [x] Task 1: Add timing around gate calls in runner (AC: #1, #4)
  - [x] 1.1 В runner.go: перед каждым `GatePromptFn`/`EmergencyGatePromptFn` call → `t0 := time.Now()`
  - [x] 1.2 После: `gateElapsed := time.Since(t0)`
  - [x] 1.3 Log: `r.logger().Info("gate decision", kv("step_type", "gate"), kv("action", decision.Action), kv("wait_ms", gateElapsed.Milliseconds()), kv("task", taskText))`
  - [x] 1.4 Gate call locations (4 шт): execute emergency (~line 723), review emergency (~line 821), normal gate (~line 897), distillation gate (~line 983 в `handleDistillFailure`)
  - [x] 1.5 ПРИМЕЧАНИЕ: budget exceeded gate — это Story 7.7 scope, НЕ 7.6. Когда 7.7 добавит budget gate, он должен добавить timing самостоятельно

- [x] Task 2: MetricsCollector integration (AC: #2, #3)
  - [x] 2.1 Construct `GateStats{TotalPrompts: 1, TotalWaitMs: gateElapsed.Milliseconds(), LastAction: decision.Action}` + set action counter (Approvals/Rejections/Skips)
  - [x] 2.2 `r.Metrics.RecordGate(stats)` с nil guard — signature: `RecordGate(stats GateStats)` (stub из Story 7.1)
  - [x] 2.3 RecordGate impl: merge stats в taskAccumulator.Gate — increment counters, accumulate TotalWaitMs, update LastAction
  - [x] 2.4 Action mapping: `approve` → Approvals++, `quit` → Rejections++, `skip` → Skips++, `retry` → TotalPrompts only (retry не финальное)
  - [x] 2.5 ПРИМЕЧАНИЕ: `GateStats` struct уже определён в `runner/metrics.go` (Story 7.1) — НЕ создавать дубликат. AvgWaitMs не нужен как field — можно вычислить из `TotalWaitMs/TotalPrompts` в reporting

- [x] Task 3: Тесты (AC: #1-#4)
  - [x] 3.1 Тест: normal gate call → GateStats recorded with approve counter (TestRunner_Execute_GateAnalytics_NormalGateRecordsMetrics)
  - [x] 3.2 Тест: MetricsCollector.RecordGate → correct GateStats counters (TestMetricsCollector_RecordGate)
  - [x] 3.3 Тест: emergency gate → same tracking pattern (TestRunner_Execute_GateAnalytics_EmergencyGateRecordsMetrics)
  - [x] 3.4 Тест: multiple gate calls → counters accumulate correctly (TestMetricsCollector_RecordGate_MultipleCallsAccumulate)
  - [x] 3.5 Тест: nil Metrics → no panic (TestRunner_Execute_GateAnalytics_NilMetricsNoPanic + TestMetricsCollector_RecordGate_NoTask)
  - [x] 3.6 Тест: retry action → TotalPrompts incremented, but Approvals/Rejections/Skips NOT incremented (TestMetricsCollector_RecordGate_RetryOnlyIncrementsPrompts)
  - [x] 3.7 Тест: distillation gate — covered by existing TestRunner_Execute_DistillationFailureHumanGate (distillation gate uses GatePromptFn, timing added)
  - [x] 3.8 Тест: quit records Rejections counter (TestRunner_Execute_GateAnalytics_QuitRecordsRejection)

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `runner/runner.go` | Extend: timing around gate calls, logging | ~20 |
| `runner/metrics.go` | Extend: RecordGate logic, AvgWaitMs in Finish | ~15 |
| `runner/runner_test.go` | Add: gate timing tests | ~40 |
| `runner/metrics_test.go` | Add: RecordGate + GateStats tests | ~30 |
| `runner/runner_gates_integration_test.go` | Extend: gate timing verification | ~20 |

### Architecture Compliance

- **No changes to `gates/` package** — timing measured in runner (caller), not gates (provider)
- **Existing infrastructure:** GatePromptFn, EmergencyGatePromptFn — только обёртка time.Now()/Since()

### Existing Code Context

- `GatePromptFn` и `EmergencyGatePromptFn` — injectable `GatePromptFunc` fields в Runner struct (line 425-426)
- `GateDecision` struct в `config/errors.go` (НЕ в gates/) — `Action string`, `Feedback string`
- Action constants в `config/constants.go`: `ActionApprove = "approve"`, `ActionRetry = "retry"`, `ActionSkip = "skip"`, `ActionQuit = "quit"`
- Gate call locations (4 шт в runner.go):
  1. Execute emergency gate: `r.EmergencyGatePromptFn(ctx, emergencyText)` (~line 723)
  2. Review emergency gate: `r.EmergencyGatePromptFn(ctx, emergencyText)` (~line 821)
  3. Normal gate: `r.GatePromptFn(ctx, gateText)` (~line 897)
  4. Distillation gate: `r.GatePromptFn(ctx, gateText)` (~line 983 в `handleDistillFailure`)
- `GateStats` struct уже определён в `runner/metrics.go` (Story 7.1): `TotalPrompts`, `Approvals`, `Rejections`, `Skips`, `TotalWaitMs`, `LastAction`
- `RecordGate(stats GateStats)` stub уже определён в `runner/metrics.go` (Story 7.1)
- `TaskMetrics.Gate *GateStats` field уже определён (Story 7.1)
- `handleDistillFailure` — separate method (line 974), uses `r.GatePromptFn` NOT `EmergencyGatePromptFn`

### References

- [Source: docs/architecture/observability-metrics.md#Data Flow] — RecordGate placement
- [Source: docs/prd/observability-metrics.md#FR53] — gate analytics
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.6] — полное описание

## Dev Agent Record

### Context Reference

- docs/architecture/observability-metrics.md (Data Flow)
- docs/prd/observability-metrics.md (FR53)

### Agent Model Used

Claude Opus 4.6 (code-review fixes)

### Debug Log References

### Completion Notes List

- All 4 gate locations wrapped with time.Now()/time.Since() and recordGateDecision helper
- RecordGate merges GateStats into taskAccumulator (lazy init)
- Action mapping: approve→Approvals, quit→Rejections, skip→Skips, retry→TotalPrompts only
- Code-review: added doc comment to recordGateDecision, log assertion, distillation gate metrics coverage

### File List

- `runner/runner.go` — recordGateDecision helper, timing wrappers at 4 gate locations
- `runner/metrics.go` — RecordGate implementation (merge counters into taskAccumulator.gate)
- `runner/runner_test.go` — 4 GateAnalytics tests + distillation gate metrics assertion
- `runner/metrics_test.go` — 4 RecordGate unit tests (approve, accumulate, no-task, retry)
