# Story 7.3: Cost Tracking — Pricing Table + Per-Task Aggregation

Status: done

## Story

As a разработчик,
I want видеть стоимость каждой задачи и кумулятивную стоимость run в реальном времени (на gate prompt),
so that контролировать бюджет.

## Acceptance Criteria

1. **AC1: Pricing struct and defaults (FR43)**
   - `config/pricing.go` (НОВЫЙ файл) определяет `Pricing` struct: `InputPer1M`, `OutputPer1M`, `CachePer1M float64` с yaml+json tags
   - `DefaultPricing map[string]Pricing` содержит как минимум:
     - `"claude-sonnet-4-20250514"` (Input: $3, Output: $15, Cache: $0.30)
     - `"claude-opus-4-20250514"` (Input: $15, Output: $75, Cache: $1.50)
   - Цены hardcoded как initial defaults (обновляются при необходимости)
   - `MostExpensiveModel(pricing map[string]Pricing) string` — helper, возвращает model с max OutputPer1M (для fallback)

2. **AC2: Config pricing override (FR43)**
   - `Config` struct расширен: `ModelPricing map[string]Pricing` yaml:"model_pricing"
   - Когда config.yaml содержит `model_pricing` с custom ценами — override per-model
   - Модели не в override используют `DefaultPricing`
   - `MergePricing(defaults, overrides map[string]Pricing) map[string]Pricing` function

3. **AC3: Unknown model warning (FR43)**
   - При session с моделью не в merged pricing map:
     - Runner логирует `logger.Warn("unknown model pricing", kv("model", name))`
     - Fallback на самую дорогую модель в merged pricing (MostExpensiveModel)
   - Warning логируется Runner'ом (caller), НЕ MetricsCollector'ом — collector только возвращает данные

4. **AC4: SessionMetrics.CostUSD calculation (FR43)**
   - CostUSD = `(InputTokens * InputPer1M + OutputTokens * OutputPer1M + CacheReadTokens * CachePer1M) / 1_000_000`
   - CostUSD accumulated в `taskAccumulator`
   - `RecordSession` в MetricsCollector рассчитывает cost используя pricing map и model name

5. **AC5: Per-task cost aggregation (FR45)**
   - Task с multiple sessions (execute + review + retry) — `TaskMetrics.CostUSD = sum` всех session costs
   - `TaskMetrics.TokensInput/Output/Cache = sum` всех session tokens

6. **AC6: Cumulative cost on gate prompt (FR45)**
   - При gate checkpoint: prompt включает `"Cost so far: $X.XX"` (cumulative run cost)
   - Formatting: 2 decimal places, USD symbol
   - GatePromptFunc signature НЕ МЕНЯТЬ — cost string append'ится к `taskText` перед вызовом

7. **AC7: Cost precision**
   - `float64` arithmetic для cost — no precision loss видимый at 2 decimal places
   - Тест: 1000 sessions * $0.003 = $3.00 (не $2.999...)
   - Тест: accumulated cost from mixed models matches manual calculation

## Tasks / Subtasks

- [x] Task 1: Create config/pricing.go (AC: #1, #2)
  - [ ] 1.1 Определить `Pricing` struct с yaml+json tags: `InputPer1M`, `OutputPer1M`, `CachePer1M float64`
  - [ ] 1.2 Определить `var DefaultPricing = map[string]Pricing{...}` с Sonnet ($3/$15/$0.30) и Opus ($15/$75/$1.50) ценами
  - [ ] 1.3 Реализовать `MergePricing(defaults, overrides map[string]Pricing) map[string]Pricing` — копирует defaults, перезаписывает overrides
  - [ ] 1.4 Реализовать `MostExpensiveModel(pricing map[string]Pricing) string` — возвращает key с max OutputPer1M
  - [ ] 1.5 Написать тесты: default values, merge override, merge empty, MostExpensiveModel

- [x] Task 2: Extend Config struct (AC: #2)
  - [ ] 2.1 Добавить `ModelPricing map[string]Pricing` yaml:"model_pricing" в Config struct (config/config.go:L17-L37)
  - [ ] 2.2 Import `Pricing` type не нужен — Config в том же package. Но yaml parsing `map[string]Pricing` потребует тип в том же package
  - [ ] 2.3 Тесты: config с model_pricing override, config без override (nil map)

- [x] Task 3: Cost calculation in MetricsCollector (AC: #4, #5, #7) — depends on Task 1
  - [ ] 3.1 Добавить `pricing map[string]config.Pricing` field в `MetricsCollector` struct (runner/metrics.go:L99-L113). Добавить `import "github.com/bmad-ralph/bmad-ralph/config"` в metrics.go
  - [ ] 3.2 Изменить конструктор: `NewMetricsCollector(runID string, pricing map[string]config.Pricing) *MetricsCollector` — pricing сохраняется в struct
  - [ ] 3.3 Изменить `RecordSession` signature: `RecordSession(metrics *session.SessionMetrics, model, stepType string, durationMs int64)` — добавить `model string` 2-м параметром
  - [ ] 3.4 В RecordSession: lookup `mc.pricing[model]` → рассчитать CostUSD по формуле → accumulate. Если model не в map — использовать `MostExpensiveModel(mc.pricing)` для fallback, вернуть resolved model name через return value: `RecordSession(...) (resolvedModel string)`
  - [ ] 3.5 Обновить `CumulativeCost() float64`: `return mc.totalCost + currentTaskCost()` где currentTaskCost() = `mc.current.costUSD` если current != nil, иначе 0. Текущая реализация (L221-223) возвращает только `mc.totalCost` — НЕ включает in-progress task
  - [ ] 3.6 Обновить ВСЕ callers `RecordSession` в runner.go — добавить `model` argument (берётся из `result.Model` где result = `*session.SessionResult`). Callers: execute session, review session, distill session — найти по `r.Metrics.RecordSession` в runner.go
  - [ ] 3.7 Обновить `cmd/ralph/run.go`: в месте создания MetricsCollector: `pricing := config.MergePricing(config.DefaultPricing, cfg.ModelPricing)` → `runner.NewMetricsCollector(cfg.RunID, pricing)`
  - [ ] 3.8 Обновить `runner/metrics_test.go`: ВСЕ существующие тесты NewMetricsCollector/RecordSession — добавить pricing param
  - [ ] 3.9 Новые тесты: single session cost calculation, multi-session aggregation per task, CumulativeCost includes current task, precision тест (1000 sessions * $0.003)

- [x] Task 4: Unknown model handling (AC: #3) — depends on Task 3
  - [ ] 4.1 В RecordSession: если model не в `mc.pricing` map → использовать `config.MostExpensiveModel(mc.pricing)` для fallback pricing. Вернуть resolved model name
  - [ ] 4.2 RecordSession returns `(resolvedModel string)` — если resolvedModel != переданный model, Runner логирует warning: `r.logger().Warn("unknown model pricing", kv("model", original), kv("fallback", resolved))`
  - [ ] 4.3 НЕ добавлять logger в MetricsCollector — он остаётся чистым data collector. Warning ответственность Runner'а
  - [ ] 4.4 Тест: unknown model → RecordSession returns fallback model name, cost calculated by Opus pricing

- [x] Task 5: Gate prompt cost display (AC: #6)
  - [ ] 5.1 В runner.go: перед КАЖДЫМ вызовом GatePromptFn и EmergencyGatePromptFn — получить `r.Metrics.CumulativeCost()` (nil guard: `if r.Metrics != nil`)
  - [ ] 5.2 Append `fmt.Sprintf("\nCost so far: $%.2f", cost)` к `gateText` string перед вызовом. Найти ВСЕ gate вызовы по grep `GatePromptFn(` и `EmergencyGatePromptFn(` в runner.go
  - [ ] 5.3 Тест: gate prompt содержит "Cost so far: $" string; без MetricsCollector — нет cost string

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `config/pricing.go` (NEW) | Create: Pricing struct, DefaultPricing, MergePricing, MostExpensiveModel | ~70 |
| `config/pricing_test.go` (NEW) | Create: тесты pricing | ~50 |
| `config/config.go` | Extend: +ModelPricing field | ~3 |
| `runner/metrics.go` | Extend: +pricing field, +config import, RecordSession cost calc, CumulativeCost fix, NewMetricsCollector signature | ~40 |
| `runner/metrics_test.go` | Update: existing tests (new signature) + Add: cost calculation tests | ~70 |
| `runner/runner.go` | Extend: pass model to RecordSession, unknown model warning, cost in gate prompt | ~25 |
| `runner/runner_test.go` | Update: NewMetricsCollector calls with pricing param | ~10 |
| `cmd/ralph/run.go` | Extend: MergePricing call, pass pricing to NewMetricsCollector | ~5 |

### Architecture Compliance

- **config = leaf:** `config/pricing.go` не зависит ни от кого — leaf package сохранён
- **Pricing in config, cost calc in runner:** session не знает о pricing, MetricsCollector (runner package) считает cost
- **runner → config:** допустимая dependency direction. `runner/metrics.go` добавляет import `config` для `config.Pricing` type
- **No new deps:** только Go stdlib

### Key Technical Decisions

1. **Pricing в config package** — leaf, используется runner для cost calc
2. **Model name** передаётся в RecordSession — берётся из `SessionResult.Model` (Story 7.1 парсит из Claude JSON)
3. **Fallback на MostExpensiveModel** при unknown model — консервативная оценка (дороже = безопаснее). Helper function в config/pricing.go
4. **Unknown model warning в Runner, НЕ в MetricsCollector** — collector остаётся чистым data aggregator. RecordSession возвращает resolvedModel, Runner сравнивает с original и логирует warning
5. **Gate prompt extension** — append cost string к `gateText` перед вызовом GatePromptFn. GatePromptFunc signature НЕ МЕНЯТЬ — cost передаётся через taskText string
6. **Backward compatibility** — все callers RecordSession (execute, review, distill sessions) обновляются с model param. Все callers NewMetricsCollector обновляются с pricing param

### Existing Code Context

**MetricsCollector** (runner/metrics.go):
- Struct (L99-113): fields `runID`, `startTime`, `tasks`, `current`, `totalInput/Output/Cache/Cost/Turns/Sessions`. НЕ имеет `pricing` field — добавить
- `NewMetricsCollector(runID string)` (L117-121) — добавить `pricing map[string]config.Pricing` param
- `RecordSession(metrics *session.SessionMetrics, stepType string, durationMs int64)` (L134-147) — добавить `model string` param, добавить return `string` (resolvedModel). Текущая реализация просто прокидывает `metrics.CostUSD` (всегда 0) — заменить на cost calculation по формуле
- `CumulativeCost() float64` (L221-223) — возвращает только `mc.totalCost` (finished tasks). ИСПРАВИТЬ: добавить `mc.current.costUSD` если current != nil
- Comment L116: "Pricing is not configured here; added in Story 7.3" — удалить/обновить
- Import (L4-8): `time` и `session` — добавить `config`

**Config** (config/config.go:L17-L37):
- 20 fields, последний `RunID string yaml:"-"`. Добавить `ModelPricing map[string]Pricing yaml:"model_pricing"` рядом с другими config fields

**SessionResult.Model** (session/result.go):
- `Model string` field — Story 7.1 парсит из Claude JSON. Использовать как ключ для pricing lookup

**GatePromptFunc** type (runner/runner.go:L74):
- `func(ctx context.Context, taskText string) (*config.GateDecision, error)` — НЕ менять signature

**Gate prompt call sites** в runner.go:
- Найти по grep `GatePromptFn(` и `EmergencyGatePromptFn(` — НЕ по line numbers (они сдвигаются после Story 7.1 изменений)
- Каждый call site: добавить cost string append к gateText перед вызовом

**cmd/ralph/run.go**:
- Место создания MetricsCollector: `runner.NewMetricsCollector(cfg.RunID)` — изменить на `runner.NewMetricsCollector(cfg.RunID, pricing)` где `pricing = config.MergePricing(config.DefaultPricing, cfg.ModelPricing)`

### Default Anthropic Pricing (hardcoded)

Цены hardcoded в `DefaultPricing` как baseline. Пользователь может override через `model_pricing` в config.yaml.

```go
var DefaultPricing = map[string]Pricing{
    "claude-sonnet-4-20250514": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
    "claude-opus-4-20250514":   {InputPer1M: 15.0, OutputPer1M: 75.0, CachePer1M: 1.50},
}
```

Model ID формат: `claude-{family}-{version}-{date}`. При несовпадении с реальными model ID — пользователь добавляет правильные через config override.

### References

- [Source: docs/architecture/observability-metrics.md#Решение 1] — Pricing struct placement
- [Source: docs/architecture/observability-metrics.md#Решение 2] — MetricsCollector injectable struct
- [Source: docs/prd/observability-metrics.md#FR43] — cost calculation
- [Source: docs/prd/observability-metrics.md#FR45] — per-task aggregation, gate display
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.3] — полное описание

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- All 5 tasks completed, all AC verified
- config/pricing.go: Pricing struct, DefaultPricing (Sonnet/Opus), MergePricing, MostExpensiveModel
- Config extended with ModelPricing field (yaml:"model_pricing")
- MetricsCollector: +pricing field, NewMetricsCollector takes pricing param
- RecordSession: cost calculation via pricing table, unknown model fallback returns resolved model name
- CumulativeCost includes in-progress task (mc.current.costUSD)
- Gate prompt: "Cost so far: $X.XX" appended at all 4 gate sites (checkpoint, 2 emergency, distill)
- Runner: unknown model warning logged by caller (not MetricsCollector)
- MergePricing called in Run() (runner.go), not cmd/ralph/run.go
- generateRunID moved from runner to cmd/ralph/run.go
- Review session cost NOW tracked via ReviewResult.SessionMetrics/Model (code-review fix H1)
- 7 new config tests + 8 new metrics tests, all pass

### Code Review Fixes Applied

- H1: Review session cost tracking — added SessionMetrics/Model to ReviewResult, RealReview populates them, Execute records via RecordSession
- M1: Added ModelPricing yaml parsing test in config_test.go (ValidFullConfig + DefaultsComplete)
- M2: Added gate prompt cost negative test (NilMetricsNoPanic asserts "Cost so far" absent)
- M3: Documented checkpoint gate test covers all 4 identical gate sites by code inspection
- L1: RecordSession doc comment updated for nil pricing and no-current-task edge cases

### File List

| File | Action | Description |
|------|--------|-------------|
| `config/pricing.go` | Created | Pricing struct, DefaultPricing, MergePricing, MostExpensiveModel |
| `config/pricing_test.go` | Created | 7 tests: defaults, merge override/empty, MostExpensiveModel variants |
| `config/config.go` | Modified | +ModelPricing field in Config struct |
| `config/config_test.go` | Modified | +ModelPricing yaml parsing test, +nil default assertion |
| `runner/metrics.go` | Modified | +pricing field, NewMetricsCollector signature, RecordSession cost calc, CumulativeCost fix, doc comment update |
| `runner/metrics_test.go` | Modified | +8 cost tests: calculation, multi-session, cumulative, precision, unknown model, empty pricing |
| `runner/runner.go` | Modified | +ReviewResult.SessionMetrics/Model, +RealReview cost capture, +Execute review cost recording, +cost in 4 gate sites, MergePricing in Run() |
| `runner/runner_test.go` | Modified | +gate prompt cost test, +nil Metrics cost absence assertion |
| `cmd/ralph/run.go` | Modified | +generateRunID, cfg.RunID assignment, Run() returns (*RunMetrics, error) |
| `cmd/ralph/cmd_test.go` | Modified | +TestGenerateRunID_Format |
