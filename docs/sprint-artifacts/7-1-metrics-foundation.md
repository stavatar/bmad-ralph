# Story 7.1: Metrics Foundation — Token Parsing, MetricsCollector, Structured Log Keys

Status: done

## Story

As a разработчик,
I want чтобы ralph собирал token usage из Claude Code JSON output, генерировал уникальные Run/Task ID и включал structured keys в каждую запись лога,
so that метрики были доступны для агрегации и корреляции.

## Acceptance Criteria

1. **AC1: SessionMetrics extraction from Claude Code JSON (FR42)**
   - `SessionResult.Metrics` содержит `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CostUSD` (=0, заполняется runner в 7.3), `NumTurns`
   - `SessionMetrics` struct определён в `session/result.go` с json tags
   - `jsonResultMessage` расширен полями `Usage *usageData`, `Model string` и `NumTurns int`
   - `SessionResult` расширен полем `Model string` (для forward-compatibility с Story 7.3 cost calculation)
   - `ParseResult` заполняет `SessionResult.Metrics` когда usage data присутствует в JSON; `SessionResult.Model` заполняется из `msg.Model`

2. **AC2: Graceful degradation when usage data absent (FR42)**
   - Если Claude Code JSON не содержит usage полей (старая версия CLI) — `SessionResult.Metrics == nil` (не ошибка)
   - ВСЕ существующие тесты `ParseResult` проходят без модификации

3. **AC3: MetricsCollector struct (Architecture Decision 2)**
   - `runner/metrics.go` (НОВЫЙ файл) содержит `MetricsCollector` struct
   - `NewMetricsCollector(runID string) *MetricsCollector` — конструктор (pricing добавляется в Story 7.3)
   - Методы: `StartTask(name)`, `RecordSession(metrics *SessionMetrics, stepType string, durationMs int64)`, `FinishTask(status, commitSHA string)`, `Finish() RunMetrics`
   - `Finish()` возвращает `RunMetrics` с accumulated данными
   - Structs `TaskMetrics`, `RunMetrics`, `DiffStats`, `LatencyBreakdown`, `GateStats`, `ErrorStats` определены с json tags (для FR55 позже)

4. **AC4: MetricsCollector nil safety**
   - `Runner.Metrics == nil` (no collector configured) — никаких panic
   - Все вызовы `r.Metrics.*` обёрнуты `if r.Metrics != nil`
   - ВСЕ существующие тесты Runner проходят без MetricsCollector

5. **AC5: Run ID generation (FR47)**
   - `cmd/ralph/run.go` генерирует UUID v4 через `crypto/rand`
   - Формат: 36-char UUID (`xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`)
   - `runID` передаётся через `Config.RunID string` → `Run()` → `MetricsCollector` + `OpenRunLogger`

6. **AC6: RunLogger structured keys (FR47, FR48)**
   - `RunLogger` имеет `runID string` field, устанавливается при создании
   - `OpenRunLogger` получает `runID` параметр
   - Каждый `write()` включает `run_id=<uuid>` как первый kv pair
   - Caller предоставляет `task_id` и `step_type` через `kv()` helper
   - `step_type` — одно из: `execute`, `review`, `gate`, `git_check`, `retry`, `distill`, `resume`
   - Формат: `"2026-03-04T10:15:30 INFO [runner] msg run_id=abc task_id=story-5.1 step_type=execute key=val"`

7. **AC7: Runner.Execute and Run return RunMetrics (Architecture Decision 12)**
   - `Execute` signature: `func (r *Runner) Execute(ctx context.Context) (*RunMetrics, error)`
   - `Run` signature: `func Run(ctx context.Context, cfg *config.Config) (*RunMetrics, error)`
   - `*RunMetrics` содержит все accumulated metrics (nil при ранней ошибке до начала сбора)
   - `cmd/ralph/run.go` (`runRun`) обновлён для приёма RunMetrics через `Run()`

## Tasks / Subtasks

- [x] Task 1: Extend session/result.go — SessionMetrics + usage parsing (AC: #1, #2)
  - [x] 1.1 Добавить `usageData` struct с json tags в `session/result.go`
  - [x] 1.2 Добавить `SessionMetrics` struct с полями `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CostUSD` (=0.0), `NumTurns` + json tags
  - [x] 1.3 Расширить `jsonResultMessage` полями `Usage *usageData`, `Model string` и `NumTurns int`
  - [x] 1.4 Расширить `SessionResult` полями `Metrics *SessionMetrics` и `Model string`
  - [x] 1.5 В `ParseResult`: после parse JSON, если `msg.Usage != nil` — создать `SessionMetrics` и привязать к result; всегда заполнять `result.Model = msg.Model`
  - [x] 1.6 Написать тесты: JSON с usage → Metrics populated; JSON без usage → Metrics == nil
  - [x] 1.7 Убедиться что ВСЕ существующие `ParseResult` тесты проходят без изменений

- [x] Task 2: Create runner/metrics.go — MetricsCollector + all metric types (AC: #3)
  - [x] 2.1 Определить structs: `DiffStats`, `ReviewFinding`, `LatencyBreakdown`, `GateStats`, `ErrorStats`, `TaskMetrics`, `RunMetrics` с json tags
  - [x] 2.2 Определить `taskAccumulator` (internal mutable state)
  - [x] 2.3 Реализовать `NewMetricsCollector(runID string) *MetricsCollector` конструктор (без pricing — добавляется в Story 7.3)
  - [x] 2.4 Реализовать `StartTask(name)`, `FinishTask(status, commitSHA)`, `Finish() RunMetrics`
  - [x] 2.5 Реализовать `RecordSession(metrics, stepType, durationMs)` — агрегация tokens (CostUSD пока 0, Story 7.3 добавит)
  - [x] 2.6 Реализовать stub-методы: `RecordGitDiff`, `RecordReview`, `RecordGate`, `RecordRetry`, `RecordError`, `RecordLatency`, `CumulativeCost`
  - [x] 2.7 Написать unit тесты: StartTask → RecordSession → FinishTask → Finish; zero-value checks; nil metrics input

- [x] Task 3: Extend runner/log.go — RunLogger с runID (AC: #6)
  - [x] 3.1 Добавить `runID string` field в `RunLogger` struct
  - [x] 3.2 Изменить `OpenRunLogger` сигнатуру: добавить `runID string` параметр
  - [x] 3.3 В `write()`: prepend `run_id=<runID>` как первый kv pair (если runID не пуст)
  - [x] 3.4 Обновить все вызовы `OpenRunLogger` в codebase (runner, cmd/ralph)
  - [x] 3.5 `NopLogger()` НЕ менять — runID="" по умолчанию, write() корректно пропускает пустой run_id
  - [x] 3.6 Написать тесты: log output содержит run_id; пустой runID — нет run_id в output

- [x] Task 4: Extend runner/runner.go — Runner.Metrics + Execute signature (AC: #4, #7)
  - [x] 4.1 Добавить `Metrics *MetricsCollector` field в `Runner` struct
  - [x] 4.2 Изменить `Execute` signature: `(ctx context.Context) (*RunMetrics, error)`
  - [x] 4.3 В `Execute`: если `r.Metrics != nil` — вызывать `StartTask` / `FinishTask` / `Finish` в нужных точках
  - [x] 4.4 В `Execute`: после session.Execute — вызвать `r.Metrics.RecordSession(result.Metrics, "execute", elapsed)` с nil guard
  - [x] 4.5 Обновить все return paths в Execute: возвращать `nil, err` для ранних ошибок; `&metrics, err` после Finish
  - [x] 4.6 Обновить `Run()` signature: `func Run(ctx, cfg) (*RunMetrics, error)` — единственный caller `r.Execute(ctx)`
  - [x] 4.7 `RunOnce()` и `RunReview()` — standalone функции, вызывают `session.Execute`, НЕ `Runner.Execute` — НЕ ТРОГАТЬ

- [x] Task 5: UUID generation + Config.RunID (AC: #5)
  - [x] 5.1 Добавить `RunID string` field в `config.Config` struct (NO yaml tag — runtime only)
  - [x] 5.2 Реализовать `generateRunID() string` в `cmd/ralph/run.go` — UUID v4 через `crypto/rand` + `fmt.Sprintf`
  - [x] 5.3 В `runRun`: вызвать `generateRunID()`, установить `cfg.RunID = runID` перед `runner.Run()`
  - [x] 5.4 Обновить `runRun` для приёма `(*RunMetrics, error)` из `runner.Run()` (не Runner.Execute напрямую — runRun вызывает Run())
  - [x] 5.5 Написать тест: UUID формат 36 chars, содержит 4 дефиса, version nibble = 4

- [x] Task 6: Integration tests (AC: #1-#7)
  - [x] 6.1 Тест: Runner с MetricsCollector → Execute возвращает non-nil RunMetrics
  - [x] 6.2 Тест: Runner без MetricsCollector (nil) → Execute работает как раньше
  - [x] 6.3 Тест: SessionMetrics populated в RunMetrics tasks
  - [x] 6.4 Проверить: ВСЕ существующие тесты runner, session, cmd/ralph проходят

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `session/result.go` | Extend: +SessionMetrics, +usageData, +Model, extend jsonResultMessage, extend ParseResult | ~45 |
| `session/result_test.go` | Add: тесты usage parsing + graceful degradation | ~60 |
| `runner/metrics.go` (NEW) | Create: MetricsCollector, all metric structs, methods | ~250 |
| `runner/metrics_test.go` (NEW) | Create: unit тесты MetricsCollector | ~200 |
| `runner/log.go` | Extend: +runID field, OpenRunLogger param, write() prepend | ~15 |
| `runner/log_test.go` | Add: тесты runID в log output | ~30 |
| `runner/runner.go` | Extend: +Metrics field, Execute signature change, nil-guarded calls | ~40 |
| `runner/runner_test.go` | Update: callers Execute signature, add MetricsCollector tests | ~50 |
| `config/config.go` | Extend: +RunID string field (no yaml tag) | ~2 |
| `cmd/ralph/run.go` | Extend: UUID gen, set cfg.RunID, receive RunMetrics | ~30 |

### Architecture Compliance

- **Dependency direction:** `runner/metrics.go` зависит только от `session` (для `SessionMetrics`) — top-down соблюдён. НЕ зависит от `config` в этой стори (pricing добавляется в 7.3)
- **config = leaf:** `config/pricing.go` НЕ создаётся в этой стори (Story 7.3). MetricsCollector НЕ принимает pricing — конструктор `NewMetricsCollector(runID string)`
- **Exit codes:** не затрагиваются. Execute возвращает error, `cmd/ralph/` конвертирует
- **Config immutability:** Config не изменяется. MetricsCollector — mutable runtime state, не часть Config

### Key Technical Decisions

1. **ParseResult signature НЕ МЕНЯЕТСЯ** — Pricing не передаётся в session. CostUSD заполняется runner'ом (Story 7.3). ParseResult извлекает raw token counts и model name
2. **Concrete struct, не interface** для MetricsCollector — нет alternative implementations, тестируемость через `collector.Finish()` inspection
3. **UUID через `crypto/rand`** — `fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])` с version 4 bit manipulation. Без external deps
4. **Nil-guard pattern** (не NoOp): `if r.Metrics != nil { r.Metrics.RecordSession(...) }` — проще, чем interface + NoOp implementation
5. **runID propagation:** `runRun` генерирует UUID → устанавливает `cfg.RunID` → `runner.Run()` берёт `cfg.RunID` → передаёт в `NewMetricsCollector(cfg.RunID)` и `OpenRunLogger(root, logDir, cfg.RunID)`. Config.RunID — NOT a config file field (no yaml tag), только runtime
6. **Model name forward-compatibility:** `jsonResultMessage.Model` и `SessionResult.Model` парсятся сейчас (zero-cost), чтобы Story 7.3 не модифицировала ParseResult повторно

### Config Changes

- Добавить `RunID string` field в `Config` struct (NO yaml tag — runtime only, не из config file)
- `runRun()` устанавливает `cfg.RunID = generateRunID()` после `config.Load()`
- `Run()` использует `cfg.RunID` для MetricsCollector и OpenRunLogger

### Existing Code Context

- `ParseResult` сигнатура: `func ParseResult(raw *RawResult, elapsed time.Duration) (*SessionResult, error)` — **не менять** в этой стори
- `SessionResult` fields: `SessionID string`, `ExitCode int`, `Output string`, `Duration time.Duration` — добавить `Metrics *SessionMetrics`, `Model string`
- `jsonResultMessage` fields: `Type`, `SessionID`, `Result`, `IsError` — добавить `Usage *usageData`, `Model string`, `NumTurns int`
- `RunLogger` fields: `file io.WriteCloser`, `stderr io.Writer` — добавить `runID string`
- `OpenRunLogger(projectRoot, logDir)` — добавить `runID string` третьим параметром
- `Runner` struct fields: `Cfg`, `Git`, `TasksFile`, `ReviewFn`, `GatePromptFn`, `EmergencyGatePromptFn`, `ResumeExtractFn`, `DistillFn`, `SleepFn`, `Knowledge`, `CodeIndexer`, `Logger` — добавить `Metrics *MetricsCollector`
- `Runner.Execute(ctx)` currently returns `error` — изменить на `(*RunMetrics, error)`
- **Caller chain:** `cmd/ralph/run.go:runRun()` → `runner.Run(ctx, cfg)` → `r.Execute(ctx)`. Только `Run()` вызывает `r.Execute()`. `RunOnce()` и `RunReview()` — standalone, вызывают `session.Execute()`, НЕ `Runner.Execute()`

### CostUSD Deferred

CostUSD **НЕ** рассчитывается в этой стори. Story 7.3 добавит pricing table и расчёт. В MetricsCollector `CostUSD` fields = 0.0 до Story 7.3.

### Testing Standards

- Table-driven тесты, Go stdlib assertions (без testify)
- `t.TempDir()` для file isolation
- Golden files для JSON output с `-update` flag
- `errors.As(err, &target)` — project standard
- Error тесты MUST verify message content через `strings.Contains`
- Каждая exported function — dedicated error test

### Project Structure Notes

- Alignment: `runner/metrics.go` рядом с `runner/log.go`, `runner/runner.go` — в том же package
- `config/pricing.go` НЕ создаётся (deferred to 7.3). MetricsCollector НЕ принимает pricing в этой стори
- Новые structs в `runner/metrics.go` (DiffStats, ReviewFinding, etc.) — позже stories 7.2-7.9 будут использовать

### References

- [Source: docs/architecture/observability-metrics.md#Решение 1] — типы метрик по пакетам
- [Source: docs/architecture/observability-metrics.md#Решение 2] — MetricsCollector injectable struct
- [Source: docs/architecture/observability-metrics.md#Решение 3] — расширение ParseResult
- [Source: docs/architecture/observability-metrics.md#Решение 10] — Run ID и Trace Correlation
- [Source: docs/architecture/observability-metrics.md#Решение 12] — Runner.Execute signature change
- [Source: docs/prd/observability-metrics.md#FR42] — token usage parsing
- [Source: docs/prd/observability-metrics.md#FR47] — Run ID + Task ID
- [Source: docs/prd/observability-metrics.md#FR48] — step_type in RunLogger
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.1] — полное описание стори

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- All 6 tasks completed, all AC verified
- Focus: DRY (resultFromMessage helper), KISS (nil-guard over NoOp interface), SRP (metrics in own file)
- Mock infrastructure extended with Model/Usage fields for metrics integration tests
- All existing tests pass without modification (AC2, AC4)

### File List

| File | Action | Description |
|------|--------|-------------|
| `session/result.go` | Modified | +SessionMetrics, +usageData, extended jsonResultMessage/SessionResult, resultFromMessage helper |
| `session/result_test.go` | Modified | +TestParseResult_UsageMetrics (6 cases), +TestSessionMetrics_ZeroValue, updated TestSessionResult_ZeroValue |
| `runner/metrics.go` | Created | MetricsCollector, all metric structs (RunMetrics, TaskMetrics, DiffStats, etc.), methods |
| `runner/metrics_test.go` | Created | 8 test functions covering lifecycle, nil input, zero values, cumulative cost |
| `runner/log.go` | Modified | +runID field, OpenRunLogger 3rd param, write() prepends run_id |
| `runner/log_test.go` | Modified | Updated OpenRunLogger calls, +TestRunLogger_RunID_IncludedInOutput |
| `runner/runner.go` | Modified | +Metrics field, Execute/Run return (*RunMetrics, error), nil-guarded metrics calls |
| `runner/runner_test.go` | Modified | All Execute calls updated to capture (*RunMetrics, error) |
| `runner/runner_integration_test.go` | Modified | +2 integration tests (WithMetrics/WithoutMetrics), all Execute calls updated |
| `runner/runner_gates_integration_test.go` | Modified | All Execute calls updated |
| `runner/runner_review_integration_test.go` | Modified | All Execute calls updated |
| `runner/runner_final_integration_test.go` | Modified | All Execute calls updated |
| `runner/runner_run_test.go` | Modified | All Run() calls updated to capture (*RunMetrics, error) |
| `runner/test_helpers_test.go` | Modified | +oneOpenTask, +twoOpenTasks fixtures |
| `config/config.go` | Modified | +RunID string field (yaml:"-") |
| `cmd/ralph/run.go` | Modified | +generateRunID(), cfg.RunID assignment, Run() captures RunMetrics |
| `cmd/ralph/cmd_test.go` | Modified | +TestGenerateRunID_Format |
| `internal/testutil/mock_claude.go` | Modified | +Model/Usage fields on ScenarioStep and mockResultMessage |
| `runner/git.go` | Modified | +DiffStats method on ExecGitClient, +DiffStats on GitClient interface (Story 7.2 scope, included early) |
| `runner/git_test.go` | Modified | +TestExecGitClient_DiffStats_HappyPath, _IdenticalSHAs, _EmptyParams |
| `internal/testutil/mock_git.go` | Modified | +DiffStats mock method, +DiffStatsResults/DiffStatsErrors/DiffStatsCount |
| `internal/testutil/mock_git_test.go` | Modified | +TestMockGitClient_DiffStats tests |
| `runner/coverage_internal_test.go` | Modified | +nopGitClient.DiffStats for interface compliance |
