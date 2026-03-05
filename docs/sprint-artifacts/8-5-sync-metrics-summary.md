# Story 8.5: Sync Metrics + Stdout Summary

Status: done

## Story

As a разработчик,
I want видеть результат sync-сессии (статус, стоимость, время) в run summary и JSON report,
so that контролировать стоимость sync.

## Acceptance Criteria

1. **SerenaSyncMetrics struct (FR65):** `runner/metrics.go` определяет `SerenaSyncMetrics` со полями: `Status string json:"status"` (success/skipped/failed/rollback), `DurationMs int64 json:"duration_ms"`, `TokensIn int json:"tokens_input,omitempty"`, `TokensOut int json:"tokens_output,omitempty"`, `CostUSD float64 json:"cost_usd,omitempty"`.
2. **RunMetrics extension (FR65):** `RunMetrics` расширен полем `SerenaSync *SerenaSyncMetrics json:"serena_sync,omitempty"`. Nil когда sync отключён (omitted from JSON). Populated когда sync выполняется (success, failed, rollback).
3. **RecordSerenaSync method (FR65):** `MetricsCollector` имеет метод `RecordSerenaSync(status string, durationMs int64, result *session.SessionResult)`. Создаёт `SerenaSyncMetrics` в RunMetrics. Если result != nil — извлекает tokens и cost. Если result == nil — только status и duration.
4. **Nil safety:** При `Runner.Metrics == nil` — вызов recordSyncMetrics не паникует (nil guard).
5. **Stdout summary line (FR65):** `formatSummary` включает строку sync: `"Serena sync: success ($0.05, 12s)"`, или `"Serena sync: rollback (validation failed, 8s)"`, или `"Serena sync: skipped (disabled)"`. Строка отсутствует когда sync не запускался (no Serena).
6. **JSON report contains sync data (FR65):** `RunMetrics` сериализация включает `"serena_sync"` объект при sync. При отключённом sync — поле отсутствует (`omitempty` на pointer).

## Tasks / Subtasks

- [x] Task 1: SerenaSyncMetrics struct (AC: #1)
  - [x] 1.1 Define `SerenaSyncMetrics` struct in `runner/metrics.go` with 5 fields
  - [x] 1.2 JSON tags: `status`, `duration_ms`, `tokens_input,omitempty`, `tokens_output,omitempty`, `cost_usd,omitempty`

- [x] Task 2: Extend RunMetrics (AC: #2)
  - [x] 2.1 Add `SerenaSync *SerenaSyncMetrics json:"serena_sync,omitempty"` field to RunMetrics

- [x] Task 3: RecordSerenaSync method (AC: #3, #4)
  - [x] 3.1 Add `RecordSerenaSync(status string, durationMs int64, result *session.SessionResult)` method to MetricsCollector
  - [x] 3.2 Nil receiver guard: if mc == nil → return (no panic)
  - [x] 3.3 Create SerenaSyncMetrics with status and duration
  - [x] 3.4 If result != nil AND result.Metrics != nil: extract InputTokens, OutputTokens, CostUSD from result.Metrics (SessionMetrics)

- [x] Task 4: Integrate recording into runSerenaSync (AC: #3)
  - [x] 4.1 Update `runSerenaSync` in `runner/serena.go` to call `r.Metrics.RecordSerenaSync(...)` at each exit point
  - [x] 4.2 Pass session result when available (success case), nil when unavailable (skip/backup-error cases)

- [x] Task 5: Update formatSummary for sync line (AC: #5)
  - [x] 5.1 Add sync line to `formatSummary` in `cmd/ralph/run.go` after reviews line
  - [x] 5.2 Conditional on `m.SerenaSync != nil`
  - [x] 5.3 Format: `"Serena sync: <status> ($<cost>, <duration>s)"` for success/rollback
  - [x] 5.4 Format: absent line when SerenaSync == nil

- [x] Task 6: Tests (AC: #1-#6)
  - [x] 6.1 Test SerenaSyncMetrics JSON serialization: verify field names and omitempty
  - [x] 6.2 Test RunMetrics JSON with SerenaSync: verify `serena_sync` present
  - [x] 6.3 Test RunMetrics JSON without SerenaSync: verify `serena_sync` omitted
  - [x] 6.4 Test RecordSerenaSync with result: verify tokens and cost populated
  - [x] 6.5 Test RecordSerenaSync without result (nil): verify only status and duration
  - [x] 6.6 Test RecordSerenaSync nil receiver: no panic
  - [x] 6.7 Test formatSummary with SerenaSync success: verify summary line present
  - [x] 6.8 Test formatSummary without SerenaSync: verify no sync line
  - [x] 6.9 Test Finish() preserves SerenaSync data in RunMetrics

## Dev Notes

### Architecture Compliance

- **Metrics pattern:** `SerenaSyncMetrics` follows same approach as `TaskMetrics`, `DiffStats`, `ReviewFinding` — separate struct, JSON-serializable.
- **Pointer field with omitempty:** `*SerenaSyncMetrics` + `omitempty` → nil = absent from JSON. Same pattern used for optional struct fields.
- **Nil receiver pattern:** `MetricsCollector` methods are nil-safe (existing pattern — all methods check `mc == nil`).
- **No new packages:** session package already imported in runner for `SessionResult`.

### Implementation Patterns (from existing code)

**RunMetrics** (`runner/metrics.go:74-89`):
- Add `SerenaSync *SerenaSyncMetrics` after `TasksSkipped` field (line 88).
- Pointer + omitempty for optional JSON field.

**MetricsCollector methods** — nil receiver pattern:
```go
func (mc *MetricsCollector) RecordSerenaSync(status string, durationMs int64, result *session.SessionResult) {
    if mc == nil { return }
    sm := &SerenaSyncMetrics{
        Status:     status,
        DurationMs: durationMs,
    }
    if result != nil && result.Metrics != nil {
        sm.TokensIn = result.Metrics.InputTokens
        sm.TokensOut = result.Metrics.OutputTokens
        sm.CostUSD = result.Metrics.CostUSD
    }
    mc.serenaSync = sm // store for Finish()
}
```
Note: Need `serenaSync *SerenaSyncMetrics` field in MetricsCollector struct (internal state, not exported).

**Finish()** (`runner/metrics.go:221+`):
- Must include `SerenaSync: mc.serenaSync` in returned RunMetrics.

**SessionResult tokens** — fields via `result.Metrics`:
- `session.SessionResult.Metrics` is `*SessionMetrics` — nil when parsing fails or no metrics available.
- `SessionMetrics` has `InputTokens int`, `OutputTokens int`, `CacheReadTokens int`, `CostUSD float64`, `NumTurns int`.
- CostUSD is directly available on SessionMetrics — no pricing calculation needed.
- **IMPORTANT:** Always check `result.Metrics != nil` before accessing — prevents nil dereference on failed sessions.

**formatSummary** (`cmd/ralph/run.go:77-127`):
- Currently 4 lines: tasks, duration/cost/tokens, reviews, report path.
- Add line 5 (conditional) between reviews and report.
- Pattern: `if m.SerenaSync != nil { ... }`.
- Format: `"Serena sync: success ($0.05, 12s)"` → `fmt.Sprintf("Serena sync: %s ($%.2f, %ds)", sm.Status, sm.CostUSD, sm.DurationMs/1000)`.

### Critical Constraints

- **RecordSerenaSync called from runSerenaSync** — at every exit point (success, rollback, skipped). Must be in Story 8.4 code (update runSerenaSync to call it). This means Task 4 modifies runner/serena.go runSerenaSync.
- **Finish() must propagate SerenaSync:** The internal `mc.serenaSync` field must be included in RunMetrics returned by Finish().
- **Cost calculation:** CostUSD available directly on SessionMetrics — no separate pricing calculation needed. Check `result.Metrics != nil` to avoid nil dereference.
- **No double Finish():** Tests must use RunMetrics from Execute(), not call Finish() again (per code-quality-patterns.md).
- **omitempty on pointer:** Go json omits nil pointers with `omitempty`. Verified correct.

### Testing Standards

- **Table-driven** for RecordSerenaSync (with/without result).
- **JSON marshaling tests:** `json.Marshal(rm)` + `strings.Contains` for field presence/absence.
- **Test naming:** `TestMetricsCollector_RecordSerenaSync_WithResult`, `TestMetricsCollector_RecordSerenaSync_NilResult`, `TestMetricsCollector_RecordSerenaSync_NilReceiver`, `TestFormatSummary_WithSerenaSync`.
- **Nil receiver:** Verify `var mc *MetricsCollector; mc.RecordSerenaSync(...)` doesn't panic.
- **Verify ALL struct fields** in SerenaSyncMetrics assertions, not just primary fields.

### Project Structure Notes

- `runner/metrics.go` — SerenaSyncMetrics struct, RunMetrics extension, RecordSerenaSync, Finish() update
- `runner/serena.go` — Update runSerenaSync to call RecordSerenaSync at each exit
- `cmd/ralph/run.go` — formatSummary sync line addition
- `runner/metrics_test.go` — RecordSerenaSync tests, JSON serialization tests
- `cmd/ralph/run_test.go` — formatSummary tests

### References

- [Source: docs/epics/epic-8-serena-memory-sync-stories.md#Story 8.5] — AC and technical notes
- [Source: docs/prd/serena-memory-sync.md#FR65] — Sync metrics requirements
- [Source: docs/architecture/serena-memory-sync.md#Decision 6] — Metrics architecture
- [Source: runner/metrics.go:74-89] — RunMetrics struct
- [Source: runner/metrics.go:110-124] — MetricsCollector struct (internal state)
- [Source: runner/metrics.go:221+] — Finish() method
- [Source: cmd/ralph/run.go:77-127] — formatSummary function

## Dev Agent Record

### Context Reference
Story 8.4 completed prior. Signature change: SerenaSyncFn now returns (*session.SessionResult, error) to propagate token/cost metrics.

### Agent Model Used
Claude Opus 4.6

### Debug Log References
N/A

### Completion Notes List
- Task 1: SerenaSyncMetrics struct with 5 JSON fields, omitempty on optional fields
- Task 2: RunMetrics extended with `SerenaSync *SerenaSyncMetrics` pointer + omitempty
- Task 3: RecordSerenaSync method with nil receiver guard, internal serenaSync field on MetricsCollector, Finish() propagation
- Task 4: runSerenaSync updated with time tracking and RecordSerenaSync at all 4 exit points (2x skipped, 1x failed, 1x rollback, 1x success). SerenaSyncFn signature changed to return (*session.SessionResult, error)
- Task 5: formatSummary conditional sync line between reviews and report path
- Task 6: 9 tests — all pass. 7 in runner/metrics_test.go, 2 in cmd/ralph/run_test.go
- Existing 8.4 tests updated for new SerenaSyncFn signature (return nil, nil / nil, error)

### File List
- runner/metrics.go — SerenaSyncMetrics struct, RunMetrics.SerenaSync field, MetricsCollector.serenaSync field, RecordSerenaSync method, Finish() update
- runner/serena.go — runSerenaSync metrics recording at all exits, RealSerenaSync returns (*session.SessionResult, error)
- runner/runner.go — SerenaSyncFn signature change, Run() factory lambda update
- runner/serena_sync_test.go — SerenaSyncFn lambda signature updates (10 instances)
- runner/metrics_test.go — 7 new tests (JSON serialization, RecordSerenaSync with/without result, nil receiver, Finish preserves)
- cmd/ralph/run.go — formatSummary sync line, doc comment update
- cmd/ralph/run_test.go — 2 new tests (formatSummary with/without SerenaSync)

## Review Record

### Findings (4 total: 0C/0H/3M/1L)

| # | Severity | Description | File | Fix |
|---|----------|-------------|------|-----|
| M1 | MEDIUM | No test for RecordSerenaSync multi-call accumulation (ternary status, duration/tokens/cost summing) | runner/metrics_test.go | Added TestMetricsCollector_RecordSerenaSync_MultipleCalls |
| M2 | MEDIUM | TestFormatSummary_WithSerenaSync missing line count assertion (4→5 lines) | cmd/ralph/run_test.go | Added lines split + len(lines) != 5 assertion |
| M3 | MEDIUM | TestRunMetrics_JSON_WithSerenaSync weak nested assertions (only key+status) | runner/metrics_test.go | Added all 5 nested field name + value assertions |
| L1 | LOW | TestMetricsCollector_Finish_PreservesSerenaSync checks 2/5 fields | runner/metrics_test.go | Added DurationMs, TokensOut, CostUSD assertions |

All findings fixed, all tests pass (runner: 5.9s, cmd/ralph: 0.03s).
