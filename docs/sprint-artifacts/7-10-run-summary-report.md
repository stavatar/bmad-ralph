# Story 7.10: Run Summary Report + Stdout Summary

Status: done

## Story

As a разработчик,
I want получать structured JSON report и краткую текстовую сводку после каждого run,
so that анализировать паттерны и сравнивать runs.

## Acceptance Criteria

1. **AC1: JSON report generation (FR55)**
   - `cmd/ralph/run.go` получает non-nil `*RunMetrics` из `runner.Run()` (currently discards with `_` — need to capture)
   - `json.MarshalIndent(metrics, "", "  ")` записывается в `<logDir>/ralph-run-<runID>.json`
   - JSON schema = `json.Marshal` of `RunMetrics` struct. Existing json tags (Story 7.1):
     - Top-level: `run_id`, `start_time`, `end_time`, `duration_ms`, `input_tokens`, `output_tokens`, `cache_tokens`, `cost_usd`, `num_turns`, `total_sessions`
     - `tasks[]` (TaskMetrics): `name`, `status`, `commit_sha`, `start_time`, `end_time`, `duration_ms`, `input_tokens`, `output_tokens`, `cache_tokens`, `cost_usd`, `num_turns`, `sessions`
     - `tasks[].diff` (DiffStats, omitempty): `files_changed`, `insertions`, `deletions`
     - `tasks[].findings[]` (ReviewFinding, omitempty): `severity`, `description`, `file`, `line`
     - `tasks[].latency` (LatencyBreakdown, omitempty): `session_ms`, `git_ms`, `gate_ms`, `review_ms`, `distill_ms`
     - `tasks[].gate` (GateStats, omitempty): `total_prompts`, `approvals`, `rejections`, `skips`, `total_wait_ms`, `last_action`
     - `tasks[].errors` (ErrorStats, omitempty): `total_errors`, `categories`
   - **NEW aggregate fields on RunMetrics** (added in this story per architecture Решение 1):
     - `tasks_completed` int `json:"tasks_completed"` — count of Tasks with Status=="completed"
     - `tasks_failed` int `json:"tasks_failed"` — count of Tasks with Status=="failed"
     - `tasks_skipped` int `json:"tasks_skipped"` — count of Tasks with Status=="skipped"
   - These aggregates computed in `MetricsCollector.Finish()` from `Tasks[]` — stored in RunMetrics for JSON serialization
   - NOTE: `Gates` and `Errors` are per-task (nested in TaskMetrics), not duplicated at run level since per-task granularity is more useful for jq queries

2. **AC2: JSON report jq-compatible (NFR22)**
   - Valid JSON, parseable `jq`
   - `jq '.tasks[0].cost_usd'` → valid number
   - `jq '.cost_usd'` → valid number (run-level total; epic uses `total_cost_usd` but actual json tag is `cost_usd`)
   - `jq '.tasks_completed'` → valid number (aggregate field)
   - `jq '.tasks[] | select(.status == "completed")'` → filter by status
   - Все fields accessible via standard jq selectors matching json tags above

3. **AC3: Stdout text summary (FR56)**
   - Format (4 lines per epic):
     ```
     Run complete: N tasks (X completed, Y skipped, Z failed)
     Duration: Xm Ys | Cost: $X.XX | Tokens: XK in / YK out
     Reviews: N cycles, M findings (Xh/Ym/Zl)
     Report: .ralph/logs/ralph-run-<id>.json
     ```
   - Task counts from `RunMetrics.TasksCompleted/Failed/Skipped` (computed in Finish())
   - Review cycles = sum of tasks that had at least one review session (count tasks where `Findings != nil || Gate != nil`)
   - Finding counts computed from `Tasks[].Findings[].Severity` (count HIGH/MEDIUM/LOW)
   - `fatih/color` для formatting (green success, yellow warnings)
   - Token counts с K suffix (125K for 125000, 1.2M for 1200000, exact for <1000)

4. **AC4: Summary with zero metrics**
   - Run без usage data (old CLI): `"Cost: N/A | Tokens: N/A"` вместо `$0.00 / 0K`
   - JSON report: zero values (не null)

5. **AC5: Report path in logDir**
   - Path: `<projectRoot>/<logDir>/ralph-run-<runID>.json`
   - `logDir` created if not exists (`os.MkdirAll`)

6. **AC6: Report write failure non-fatal (NFR24)**
   - При ошибке write (permissions, disk full):
     - `fmt.Fprintf(os.Stderr, "WARNING: failed to write run report: %v\n", err)` (cmd/ralph не имеет structured logger)
     - Stdout summary всё равно printed
     - Ralph exits с normal exit code (report — best-effort)

7. **AC7: Additive-only schema (NFR22)**
   - Новые поля добавляются, существующие never removed/renamed
   - Documented в architecture shard

## Tasks / Subtasks

- [x] Task 1: Add aggregate fields to RunMetrics + JSON report writing (AC: #1, #2, #5, #6)
  - [x] 1.1 Extend `RunMetrics` struct with 3 aggregate fields: `TasksCompleted int json:"tasks_completed"`, `TasksFailed int json:"tasks_failed"`, `TasksSkipped int json:"tasks_skipped"`
  - [x] 1.2 Update `MetricsCollector.Finish()`: compute aggregate counts from `mc.tasks[].Status` and populate new RunMetrics fields
  - [x] 1.3 Change `_, runErr := runner.Run(...)` → `metrics, runErr := runner.Run(...)` (capture RunMetrics) in `cmd/ralph/run.go`
  - [x] 1.4 После runner.Run: если metrics != nil → `json.MarshalIndent(metrics, "", "  ")`
  - [x] 1.5 Compute report path: `filepath.Join(cfg.ProjectRoot, cfg.LogDir, fmt.Sprintf("ralph-run-%s.json", cfg.RunID))`
  - [x] 1.6 `os.MkdirAll` для logDir path
  - [x] 1.7 `os.WriteFile(path, jsonBytes, 0644)`
  - [x] 1.8 При ошибке MkdirAll или WriteFile: `fmt.Fprintf(os.Stderr, "WARNING: ...")`, continue (не return error)
  - [x] 1.9 Тесты: JSON valid, contains actual json tag keys incl. `tasks_completed`/`tasks_failed`/`tasks_skipped`, jq-parseable

- [x] Task 2: Stdout text summary (AC: #3, #4)
  - [x] 2.1 `formatSummary(m *runner.RunMetrics) string` — helper в cmd/ralph/run.go (import `runner` package)
  - [x] 2.2 `formatTokens(n int) string` — "125K", "1.2M", exact for <1000
  - [x] 2.3 Task counts from `m.TasksCompleted`/`m.TasksFailed`/`m.TasksSkipped` (pre-computed in Finish())
  - [x] 2.4 Compute review cycles = count tasks where `Findings != nil` (tasks that went through review)
  - [x] 2.5 Compute findings from `m.Tasks[].Findings[].Severity`: count HIGH/MEDIUM/LOW
  - [x] 2.6 4 строки summary с `fatih/color` (green for success line, yellow for warnings): line 3 = "Reviews: N cycles, M findings (Xh/Ym/Zl)"
  - [x] 2.7 Zero metrics: `m.CostUSD == 0 && m.InputTokens == 0` → "Cost: N/A | Tokens: N/A"
  - [x] 2.8 Тесты: formatTokens edge cases, formatSummary output with various RunMetrics states, review line format

- [x] Task 3: Integration (AC: #1-#7)
  - [x] 3.1 Тест: RunMetrics with populated Tasks → JSON file created in logDir with correct path pattern
  - [x] 3.2 Тест: RunMetrics nil → no file written, no summary printed (guard)
  - [x] 3.3 Тест: write failure (bad logDir path) → warning to stderr, summary still printed, no error return
  - [x] 3.4 Тест: JSON round-trip (marshal RunMetrics → unmarshal to map → verify json tag keys: `run_id`, `duration_ms`, `tasks`, `cost_usd`, `tasks_completed`, `tasks_failed`, `tasks_skipped`)
  - [x] 3.5 Тест: summary output contains task counts matching TasksCompleted/Failed/Skipped
  - [x] 3.6 Тест: Finish() correctly computes aggregate fields from Tasks[].Status
  - [x] 3.7 Тест: summary "Reviews:" line shows correct cycles and finding severity counts

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `runner/metrics.go` | Extend: 3 aggregate fields on RunMetrics, compute in Finish() | ~15 |
| `runner/metrics_test.go` | Add: тесты aggregate fields in Finish() | ~20 |
| `cmd/ralph/run.go` | Extend: JSON write, text summary, formatSummary, formatTokens | ~80 |
| `cmd/ralph/run_test.go` | Add: тесты summary formatting, JSON report | ~70 |

### Architecture Compliance

- **cmd/ralph/ decides output** — JSON write and stdout summary in cmd, not in runner
- **runner returns data** — `*RunMetrics` struct with aggregate fields, cmd serializes
- **fatih/color** — already a dependency, used for stdout formatting
- **RunMetrics aggregate fields** — architecture Решение 1 defines `TasksCompleted`, `TasksFailed`, `TasksSkipped` on RunMetrics — this story implements them via `Finish()` computation
- **Per-task Gates/Errors** — already on TaskMetrics as nested structs; NOT duplicated at run level (architecture had run-level `Gates`/`Errors` but per-task is more useful)

### Key Technical Decisions

1. **formatTokens** — `"125K"` not `"125,000"` — concise terminal output
2. **N/A for zero metrics** — distinguishes "no data" from "zero cost" clearly
3. **Best-effort report** — write failure doesn't change exit code (NFR24)
4. **Report in logDir** — same dir as RunLogger log files, consistent location
5. **JSON schema = direct marshal of RunMetrics** — aggregate task counts (`tasks_completed/failed/skipped`) computed in `Finish()` and stored on RunMetrics for JSON serialization (per architecture Решение 1). Finding severity counts computed only for stdout summary
6. **No logger in cmd/ralph for report errors** — use `fmt.Fprintf(os.Stderr, "WARNING: ...")` for report write failures (cmd/ralph doesn't have structured logger; epic uses `logger.Error` but that's in runner scope, not cmd)
7. **Epic json tag deviation** — epic says `total_cost_usd`, `total_tokens_input/output`; actual code (Story 7.1) uses `cost_usd`, `input_tokens`, `output_tokens`. Story follows actual code.

### Existing Code Context

- `runRun()` в `cmd/ralph/run.go` (line 29) — currently `_, runErr := runner.Run(cmd.Context(), cfg)` at line 38 — DISCARDS RunMetrics with `_`
- `runner.Run()` (line 1143 runner.go) returns `(*RunMetrics, error)` — public API, internally calls `r.Execute(ctx)`
- `Config.LogDir` — configured log directory (e.g., `.ralph/logs`)
- `Config.ProjectRoot` — project root for path resolution
- `Config.RunID` — UUID v4 set by cmd/ralph before Run call
- `fatih/color` — already a dependency, used in cmd/ralph
- `RunMetrics` struct (lines 73-85 metrics.go) — json tags: `run_id`, `start_time`, `end_time`, `duration_ms`, `tasks`, `input_tokens`, `output_tokens`, `cache_tokens`, `cost_usd`, `num_turns`, `total_sessions`. **This story adds:** `tasks_completed`, `tasks_failed`, `tasks_skipped`
- `TaskMetrics` struct (lines 52-70 metrics.go) — json tags: `name`, `status`, `commit_sha`, `start_time`, `end_time`, `duration_ms`, `input_tokens`, `output_tokens`, `cache_tokens`, `cost_usd`, `num_turns`, `sessions`, `diff`, `findings`, `latency`, `gate`, `errors`
- `ReviewFinding` struct (lines 19-24 metrics.go) — json tags: `severity`, `description`, `file`, `line`
- `MetricsCollector.Finish()` (line 184 metrics.go) — returns RunMetrics. **This story extends** Finish() to compute TasksCompleted/Failed/Skipped from `mc.tasks[].Status`
- **Capstone story:** depends on ALL prior stories (7.1-7.9) populating RunMetrics fields

### Token formatting examples

| Input | Output |
|-------|--------|
| 500 | "500" |
| 1000 | "1.0K" |
| 125000 | "125K" |
| 1200000 | "1.2M" |
| 0 | "0" |

### References

- [Source: docs/architecture/observability-metrics.md#Решение 12] — JSON Report и stdout Summary
- [Source: docs/prd/observability-metrics.md#FR55] — JSON report
- [Source: docs/prd/observability-metrics.md#FR56] — text summary
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.10] — полное описание

## Dev Agent Record

### Context Reference
- Story 7.1 (RunMetrics, MetricsCollector, Finish()), Story 7.4 (ReviewFinding), Story 7.9 (LatencyBreakdown, ErrorStats)

### Agent Model Used
- claude-opus-4-6

### Debug Log References
- All tests pass: 8 packages, 0 failures

### Completion Notes List
- Task 1: Added TasksCompleted/TasksFailed/TasksSkipped aggregate fields to RunMetrics, computed in Finish() from Tasks[].Status. Captured RunMetrics in runRun(), writeRunReport() writes JSON to logDir (best-effort, non-fatal errors via stderr WARNING).
- Task 2: formatSummary() produces 4-line stdout summary with fatih/color (green success line). formatTokens() handles <1K exact, K suffix, M suffix. Zero metrics → "Cost: N/A | Tokens: N/A". Review line counts cycles and finding severity breakdown.
- Task 3: 8 tests in run_test.go (formatTokens table-driven, formatSummary with metrics, zero metrics, JSON report creation, bad path non-fatal, nil metrics guard, review line, JSON round-trip). 1 test in metrics_test.go (Finish aggregate counts). FinishEmpty extended with aggregate zero assertions. Review fix: Finish() now counts "error" status as "failed" in aggregates.
- Fixed: inline LatencyBreakdown literals in runner.go RecordLatency calls (prior session had dropped these calls)

### File List
- runner/metrics.go — +3 aggregate fields on RunMetrics, Finish() computes counts
- runner/metrics_test.go — +Finish_AggregateTaskCounts test, +zero assertions in FinishEmpty
- cmd/ralph/run.go — capture RunMetrics, writeRunReport, formatSummary, formatTokens
- cmd/ralph/run_test.go (NEW) — 8 tests covering AC1-AC7
- runner/runner.go — restored `var breakdown LatencyBreakdown` declaration

## Change Log
- 2026-03-05: Implemented Story 7.10 — Run Summary Report (all 3 tasks, 7 ACs satisfied)
