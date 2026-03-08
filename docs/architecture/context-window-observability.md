# Context Window Observability — Architecture Shard

**Epic:** 10 — Context Window Observability
**PRD:** [docs/prd/context-window-observability.md](../prd/context-window-observability.md)
**Status:** Draft
**Date:** 2026-03-06

---

## Обзор

Context Window Observability добавляет два измерения к существующей подсистеме метрик: **точный подсчёт compactions** (через PreCompact hook) и **приблизительный context fill %** (через формулу из кумулятивных token данных). Архитектурно — расширение существующих `MetricsCollector`, `RunLogger`, `Config`, `SessionMetrics` без нового пакета, без нового dependency.

### Dependency Direction (без изменений)

```
cmd/ralph
├── runner (MetricsCollector ← новые поля, EstimateMaxContextFill ← новая функция)
│   ├── session (SessionMetrics ← +ContextWindow field)
│   ├── gates (без изменений)
│   └── config (← +context_warn_pct, +context_critical_pct)
└── bridge (без изменений)
```

Session расширяет `SessionMetrics` полем `ContextWindow` (парсит из `modelUsage` в JSON) и добавляет internal struct `modelUsageEntry`. Runner владеет формулой и hook lifecycle. Config добавляет два порога. `cmd/ralph` форматирует summary.

---

## Решение 1: PreCompact Hook — Lifecycle и Counter

### Проблема

Claude Code не предоставляет compaction count в result JSON. Единственный способ узнать о compaction: PreCompact hook — bash-скрипт, вызываемый Claude Code перед каждым auto-compact.

### Архитектура

```
runner (before session.Execute)
  │ os.CreateTemp("", "ralph-compact-*") → tempFile
  │ opts.Env["RALPH_COMPACT_COUNTER"] = tempFile.Name()
  ▼
session.Execute(ctx, opts)
  │ Claude Code runs...
  │   └── PreCompact hook fires (0..N times)
  │       └── count-compact.sh: echo 1 >> $RALPH_COMPACT_COUNTER
  ▼
runner (after session.Execute)
  │ countCompactions(tempFile) → int
  │ os.Remove(tempFile)
  │ MetricsCollector.RecordSession(..., compactions)
```

### Новые функции в `runner/context.go` (новый файл)

```go
// runner/context.go — context window observability functions

// CreateCompactCounter creates a temp file for counting PreCompact hook events.
// Returns the file path and a cleanup function. Caller must defer cleanup.
// On error, returns empty path and no-op cleanup (graceful degradation).
func CreateCompactCounter() (path string, cleanup func())

// CountCompactions reads the compact counter file and returns the number
// of compaction events (non-empty lines). Returns 0 on any error.
func CountCompactions(path string) int

// EstimateMaxContextFill calculates approximate max context window fill
// percentage from cumulative session metrics.
//
// Formula: 2 × (cache_read + cache_creation + input) / max(num_turns, 2) / context_window × 100
//
// Returns 0.0 when numTurns == 0, contextWindow == 0, or metrics is nil.
// Accuracy: ±10-15%, gives upper bound (safer for monitoring).
// Uses metrics.ContextWindow if > 0, otherwise falls back to fallbackContextWindow.
func EstimateMaxContextFill(metrics *session.SessionMetrics, fallbackContextWindow int) float64
```

```go
// DefaultContextWindow is the fallback context window size (Claude Code default: 200k tokens).
const DefaultContextWindow = 200000
```

**Обоснование отдельного файла:** context.go группирует 3 функции + 1 константу единой подсистемы. Не раздувает metrics.go (уже ~300 LOC). Аналогично similarity.go.

### Temp-файл lifecycle

| Момент | Действие | Ошибка → |
|--------|----------|----------|
| Перед `session.Execute` | `CreateCompactCounter()` → path, cleanup | path = "", cleanup = no-op |
| `session.Execute` | Claude Code sets `RALPH_COMPACT_COUNTER` env | Если path пуст — env не задаётся |
| После `session.Execute` | `CountCompactions(path)` → int | return 0 |
| defer cleanup | `os.Remove(tempFile)` | Warning в лог, не блокирует |

Temp-файл создаётся через `os.CreateTemp("", "ralph-compact-*")` — OS temp dir, уникальное имя. При crash процесса — OS cleanup.

### Hook Setup: EnsureCompactHook

```go
// runner/context.go

// EnsureCompactHook ensures PreCompact hook script and settings.json registration exist.
// Called once at runner start. Errors are warnings, not fatal.
func EnsureCompactHook(projectRoot string) error
```

**Двухэтапный setup:**

**Этап 1: Script**
- Путь: `<projectRoot>/.ralph/hooks/count-compact.sh`
- Содержимое: `#!/bin/bash\n[ -n "$RALPH_COMPACT_COUNTER" ] && echo 1 >> "$RALPH_COMPACT_COUNTER"\n`
- Создать если не существует. Перезаписать если содержимое не совпадает (обновление Ralph). `chmod +x`

**Этап 2: settings.json (аддитивный merge)**
- Путь: `<projectRoot>/.claude/settings.json`
- Логика:
  1. Если файл не существует → создать с `{"hooks":{"PreCompact":[...]}}`
  2. Если существует → `json.Unmarshal` в `map[string]any`
  3. Навигация: `hooks` → `PreCompact` → массив
  4. Проверить есть ли запись с `command` содержащим `count-compact.sh`
  5. Если нет → append в массив
  6. Если есть → не трогать
  7. `json.MarshalIndent` → запись обратно
  8. **Никогда не удалять** другие записи пользователя

**Структура hook записи:**
```json
{
  "matcher": "auto",
  "hooks": [
    {
      "type": "command",
      "command": ".ralph/hooks/count-compact.sh"
    }
  ]
}
```

**matcher: "auto"** — hook срабатывает только при auto-compact, не при ручном `/compact`.

### settings.json Mutation — обоснование и риски

**Concern (party-mode critique):** Ralph модифицирует `.claude/settings.json` — конфигурацию чужого инструмента. Это:
- Может сломать пользовательские настройки при ошибке merge
- Создаёт неожиданный side effect от `ralph run`
- Аналогичный паттерн: `--append-system-prompt` — Ralph уже инструментирует Claude Code

**Решение: safe additive merge с двумя guardrails:**
1. **Идемпотентность:** проверка `command` содержит `count-compact.sh` → не дублируем
2. **Никогда не удаляем:** только append в массив `PreCompact`, никогда не удаляем/модифицируем другие hooks
3. **JSON round-trip:** `json.Unmarshal` → modify → `json.MarshalIndent` с `"  "` indent — сохраняет читаемость
4. **Backup:** `settings.json.bak` перед первой модификацией (не перед каждой — только если бэкап не существует)
5. **Non-fatal:** ошибка merge → warning + `compactions=0` (degraded, не broken)
6. **Revert:** hook script в `.ralph/hooks/` (не в `.claude/`) — удаление Ralph удаляет скрипт, settings.json остаётся с dangling reference (harmless — Claude Code ignores missing hook scripts)

### Интеграция в Runner

`EnsureCompactHook` вызывается в `Runner.Execute()` перед основным циклом — один раз на run. Ошибка логируется как warning, не блокирует execution.

**Execute path** (runner.go, внутри цикла итераций):
```go
counterPath, counterCleanup := CreateCompactCounter()
defer counterCleanup()

if counterPath != "" {
    opts.Env["RALPH_COMPACT_COUNTER"] = counterPath
}

raw, execErr := session.Execute(ctx, opts)
// ... parse result → sr ...

compactions := CountCompactions(counterPath)
fillPct := EstimateMaxContextFill(sr.Metrics, DefaultContextWindow)
resolved := r.Metrics.RecordSession(sr.Metrics, sr.Model, "execute", elapsed.Milliseconds(), compactions, fillPct)
LogContextWarnings(log, fillPct, compactions, r.Cfg.MaxTurns, r.Cfg.ContextWarnPct, r.Cfg.ContextCriticalPct)
```

**Review path** — две точки интеграции:

1. **В `RealReview`** — counter через `RunConfig.Env` (новое поле):
```go
// runner/runner.go — RunConfig extension
type RunConfig struct {
    // ... existing fields ...
    Env map[string]string // extra env vars for session (e.g., RALPH_COMPACT_COUNTER)
}
```

В `RealReview`:
```go
opts := session.Options{
    // ... existing ...
}
// Pass through extra env vars (compact counter, etc.)
for k, v := range rc.Env {
    if opts.Env == nil {
        opts.Env = make(map[string]string)
    }
    opts.Env[k] = v
}
```

2. **В caller (Runner.Execute review section, line ~1260)** — создать counter, передать через RunConfig.Env, считать после:
```go
reviewCounterPath, reviewCounterCleanup := CreateCompactCounter()
defer reviewCounterCleanup()

reviewEnv := map[string]string{}
if reviewCounterPath != "" {
    reviewEnv["RALPH_COMPACT_COUNTER"] = reviewCounterPath
}
rc := RunConfig{
    // ... existing fields ...
    Env: reviewEnv,
}

rr, err := r.ReviewFn(ctx, rc)
// ... existing error handling ...

reviewCompactions := CountCompactions(reviewCounterPath)
reviewFillPct := EstimateMaxContextFill(rr.SessionMetrics, DefaultContextWindow)
resolved := r.Metrics.RecordSession(rr.SessionMetrics, rr.Model, "review", reviewElapsed.Milliseconds(), reviewCompactions, reviewFillPct)
LogContextWarnings(log, reviewFillPct, reviewCompactions, r.Cfg.MaxTurns, r.Cfg.ContextWarnPct, r.Cfg.ContextCriticalPct)
```

> **Критика party-mode** подтверждена и исправлена: review сессии теперь получают свой counter через `RunConfig.Env`.

**Resume path** (line ~523): аналогично execute — counter создаётся в вызывающем коде `ResumeExtraction`, передаётся через opts.Env.

**SerenaSync path** (serena.go): аналогично — counter через opts.Env.

---

## Решение 2: EstimateMaxContextFill — Формула

### Математическая модель

Контекст Claude Code растёт линейно (подтверждено документацией Anthropic). Кумулятивные token данные из result JSON:

```
cumulative_total = cache_read + cache_creation + input_tokens
```

Для N turns с линейным ростом от C₀ до C_max:
```
cumulative_total = N × (C₀ + C_max) / 2
```

Решая для C_max (упрощённо, без C₀):
```
estimated_max = 2 × cumulative_total / N
fill_pct = estimated_max / context_window × 100
```

### Guard: max(numTurns, 2)

При `numTurns == 1`: весь cumulative total — один запрос, нет "роста". Формула `2 × total / 1 = 2 × total` — двойной переcчёт. Guard `max(numTurns, 2)` устраняет артефакт.

### Числовой пример (реальные данные)

Сессия #4 mentorlearnplatform:
```
cache_read = 1,456,521    cache_creation = 57,388    input = 2,700
num_turns = 25            context_window = 200,000

total = 1,516,609
estimated_max = 2 × 1,516,609 / 25 = 121,329
fill_pct = 121,329 / 200,000 × 100 = 60.7%
```

### Поведение при compaction

Compaction ломает линейную модель — после сжатия контекст резко падает. Формула **занижает** реальный пик. Это допустимо: PreCompact hook детектирует сам факт compaction, warning выдаётся по `compactions > 0`.

### Сигнатура

```go
func EstimateMaxContextFill(metrics *session.SessionMetrics, fallbackContextWindow int) float64
```

Принимает `*session.SessionMetrics` целиком (не 5 отдельных int'ов) — расширяемо, DRY. `contextWindow` теперь доступен как `metrics.ContextWindow` (из `modelUsage` в JSON). Функция внутренне применяет fallback: если `metrics.ContextWindow > 0` — использует его, иначе `fallbackContextWindow`. Caller передаёт `DefaultContextWindow` (200000) как fallback.

---

## Решение 3: SessionMetrics — ContextWindow field

### Проблема

PRD (FR86) требует `context_window` из result JSON. Текущий `SessionMetrics` не содержит этого поля.

### Источник данных (верифицировано)

Анализ минифицированного исходного кода Claude Code v2.1.56 показал:

**Поле `usage`** в result JSON — тип `unknown` (не типизирован), реально содержит:
```json
{
  "input_tokens": 0,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 0,
  "output_tokens": 0,
  "server_tool_use": {"web_search_requests": 0, "web_fetch_requests": 0}
}
```
**НЕ содержит `context_window`.**

**Поле `modelUsage`** — типизировано как `Record<string, ModelUsageEntry>`:
```json
{
  "claude-sonnet-4-6-20250514": {
    "inputTokens": 12345,
    "outputTokens": 6789,
    "cacheReadInputTokens": 500,
    "cacheCreationInputTokens": 100,
    "webSearchRequests": 0,
    "costUSD": 0.03,
    "contextWindow": 200000,
    "maxOutputTokens": 16384
  }
}
```
**`contextWindow` находится здесь**, ключ — имя модели.

> **Внимание:** camelCase в `modelUsage` vs snake_case в `usage` — Claude Code использует разные стили для разных полей.

### Изменение

**Новый тип `modelUsageEntry`:**
```go
// session/result.go
type modelUsageEntry struct {
    InputTokens             int     `json:"inputTokens"`
    OutputTokens            int     `json:"outputTokens"`
    CacheReadInputTokens    int     `json:"cacheReadInputTokens"`
    CacheCreationInputTokens int    `json:"cacheCreationInputTokens"`
    ContextWindow           int     `json:"contextWindow"`
    MaxOutputTokens         int     `json:"maxOutputTokens"`
    CostUSD                 float64 `json:"costUSD"`
}
```

**Расширение `jsonResultMessage`:**
```go
type jsonResultMessage struct {
    // ... existing fields ...
    ModelUsage map[string]modelUsageEntry `json:"modelUsage"` // NEW
}
```

**Расширение `SessionMetrics`:**
```go
type SessionMetrics struct {
    InputTokens         int     `json:"input_tokens"`
    OutputTokens        int     `json:"output_tokens"`
    CacheReadTokens     int     `json:"cache_read_input_tokens"`
    CacheCreationTokens int     `json:"cache_creation_input_tokens"`
    CostUSD             float64 `json:"cost_usd"`
    NumTurns            int     `json:"num_turns"`
    ContextWindow       int     `json:"context_window"` // NEW: from modelUsage[*].contextWindow
}
```

**Извлечение в `resultFromMessage`:**
```go
// After setting existing metrics fields:
if len(msg.ModelUsage) > 0 {
    // Take contextWindow from first (usually only) model entry
    for _, entry := range msg.ModelUsage {
        r.Metrics.ContextWindow = entry.ContextWindow
        break
    }
}
```

При отсутствии `modelUsage` в JSON (старые версии Claude Code) — `ContextWindow == 0`, caller использует fallback 200000.

### Обратная совместимость

Zero value (0) — семантически "неизвестно". Все потребители проверяют `if contextWindow == 0 { contextWindow = 200000 }`. Существующие тесты не ломаются — `modelUsage` отсутствует в текущих golden files → парсится как nil map → ContextWindow = 0.

---

## Решение 4: MetricsCollector Extension

### Новые поля в taskAccumulator

```go
type taskAccumulator struct {
    // ... existing fields ...
    totalCompactions  int     // sum of compactions across sessions in this task
    maxContextFillPct float64 // max fill% among sessions in this task
}
```

### RecordSession — расширение сигнатуры

Текущая сигнатура:
```go
func (mc *MetricsCollector) RecordSession(metrics *session.SessionMetrics, model, stepType string, durationMs int64) string
```

> **Возвращает `string`** — resolved model name (fallback на MostExpensiveModel если model неизвестен). Все 6 call sites используют этот return value для warning логирования.

Новая:
```go
func (mc *MetricsCollector) RecordSession(metrics *session.SessionMetrics, model, stepType string, durationMs int64, compactions int, contextFillPct float64) string
```

Return value не меняется — по-прежнему `string` (resolved model).

Внутри (добавить после existing logic, перед `return resolvedModel`):
```go
mc.current.totalCompactions += compactions
if contextFillPct > mc.current.maxContextFillPct {
    mc.current.maxContextFillPct = contextFillPct
}
```

**Обновление всех call sites** (6 мест в runner.go + 1 в serena.go):
- Все вызовы `RecordSession` получают +2 аргумента: `compactions int, contextFillPct float64`
- Для путей, где compaction counter не создавался (fallback): `0, 0.0`

### Новые поля в TaskMetrics

```go
type TaskMetrics struct {
    // ... existing fields ...
    TotalCompactions  int     `json:"total_compactions"`
    MaxContextFillPct float64 `json:"max_context_fill_pct"`
}
```

### Новые поля в RunMetrics

```go
type RunMetrics struct {
    // ... existing fields ...
    TotalCompactions  int     `json:"total_compactions"`
    MaxContextFillPct float64 `json:"max_context_fill_pct"`
}
```

### FinishTask и Finish — агрегация

`FinishTask`: копирует `current.totalCompactions` и `current.maxContextFillPct` в `TaskMetrics`.

`Finish`: агрегирует по всем tasks:
- `TotalCompactions` = sum всех task compactions
- `MaxContextFillPct` = max всех task fill%

---

## Решение 5: Warning System

### Уровни (тексты из PRD FR89)

| Условие | Уровень | Сообщение |
|---------|---------|-----------|
| fill ≤ warn threshold | — (тихо) | Только в session log header |
| fill > warn, ≤ critical | WARN | `"context fill NN.N%% — consider reducing max_turns (current: N) or splitting task into smaller pieces"` |
| fill > critical | ERROR | `"context fill NN.N%% exceeds critical threshold — quality degradation likely, reduce max_turns (current: N)"` |
| compactions > 0 | ERROR | `"N compaction(s) detected — context was compressed, quality degraded. Reduce max_turns (current: N)"` |

> `%%` — Go fmt literal `%` escape.

### Реализация

Новая функция в `runner/context.go`:

```go
// LogContextWarnings logs context fill and compaction warnings to the RunLogger.
// maxTurns is included in messages for actionable guidance.
func LogContextWarnings(log *RunLogger, fillPct float64, compactions int, maxTurns int, warnPct int, criticalPct int)
```

Вызывается после каждого `RecordSession` в execute/review path.

### Summary Line

`cmd/ralph/run.go` `formatSummary`:
```
Context: max 42.7% fill, 0 compactions
```

При `compactions > 0` — строка дополняется `[!]` маркером и окрашивается жёлтым. При `fill > criticalPct` — `[!]` маркер и красный цвет (через `fatih/color`).

Данные берутся из `RunMetrics.MaxContextFillPct` и `RunMetrics.TotalCompactions`.

---

## Решение 6: Config Fields

### Новые поля

```go
// config/config.go — дополнение Config struct
type Config struct {
    // ... existing ...
    ContextWarnPct    int `yaml:"context_warn_pct"`    // default: 55
    ContextCriticalPct int `yaml:"context_critical_pct"` // default: 65
}
```

### defaults.yaml

```yaml
# ... existing ...
context_warn_pct: 55
context_critical_pct: 65
```

### Валидация в Config.Validate()

```go
if c.ContextWarnPct < 1 || c.ContextWarnPct > 99 {
    return fmt.Errorf("config: context_warn_pct must be 1-99, got %d", c.ContextWarnPct)
}
if c.ContextCriticalPct < 1 || c.ContextCriticalPct > 99 {
    return fmt.Errorf("config: context_critical_pct must be 1-99, got %d", c.ContextCriticalPct)
}
if c.ContextCriticalPct <= c.ContextWarnPct {
    return fmt.Errorf("config: context_critical_pct (%d) must be > context_warn_pct (%d)", c.ContextCriticalPct, c.ContextWarnPct)
}
```

### max_turns default change

```yaml
# defaults.yaml — CHANGE
max_turns: 15  # was: 50
```

**Breaking change analysis:**

Реальные данные mentorlearnplatform (11 execute-сессий при `max_turns: 50`):
- Минимум: 18 turns, максимум: 41 turn
- Все 11 сессий превысили бы `max_turns: 15`

**Это допустимо**, потому что Ralph поддерживает **resume**: задача, не завершённая за 15 turns, продолжается в следующей итерации со свежим контекстом. При `max_iterations: 3` и `max_turns: 15` агент получает до 45 turns на задачу (3 × 15), но каждый раз с чистым контекстом.

**Митигация:**
1. Пользователь может вернуть через `max_turns: 50` в ralph.yaml
2. CLI override: `--max-turns 30`
3. Документировать в CHANGELOG: *"Default max_turns changed from 50 to 15. Ralph uses resume to continue tasks across iterations with fresh context. Override with `max_turns` in config."*
4. Warning в summary при resume: `"Task continued via resume (iteration N) — context refreshed"`

---

## Решение 7: Session Log Extension

### SessionLogInfo — новые поля

```go
type SessionLogInfo struct {
    SessionType string
    Seq         int
    ExitCode    int
    Elapsed     time.Duration
    Compactions int     // NEW: compaction count (0 if unknown)
    MaxFillPct  float64 // NEW: estimated max context fill % (0 if unknown)
}
```

### SaveSessionLog — расширение header

Текущий формат:
```
=== SESSION execute seq=3 exit_code=0 elapsed=45.2s ===
```

Новый формат:
```
=== SESSION execute seq=3 exit_code=0 elapsed=45.2s compactions=0 max_fill=42.7% ===
```

Изменение в `fmt.Fprintf` строке — добавление двух полей в конец header line.

---

## FR → Package/File Mapping

| FR | Package | File(s) | Тип изменения |
|----|---------|---------|---------------|
| FR81 | config | defaults.yaml | `max_turns: 50` → `max_turns: 15` |
| FR82 | runner | context.go (new) | `CreateCompactCounter`, `CountCompactions` |
| FR83 | runner | context.go (new) | `EnsureCompactHook` |
| FR84 | runner | runner.go | Интеграция: env var, post-session read, передача в RecordSession |
| FR85 | runner | context.go (new) | `EstimateMaxContextFill` |
| FR86 | session | result.go | `SessionMetrics.ContextWindow` field, `modelUsageEntry` struct, parse from `modelUsage` in JSON |
| FR87 | runner | metrics.go | `TaskMetrics.TotalCompactions`, `TaskMetrics.MaxContextFillPct` |
| FR88 | runner | metrics.go | `RunMetrics.TotalCompactions`, `RunMetrics.MaxContextFillPct` |
| FR89 | runner | context.go (new) | `LogContextWarnings` |
| FR90 | cmd/ralph | run.go | Context line в formatSummary |
| FR91 | config | config.go, defaults.yaml | `ContextWarnPct`, `ContextCriticalPct` |
| FR92 | runner | log.go | `SessionLogInfo` extension, header format |

---

## Новые файлы

| File | Package | LOC est. | Content |
|------|---------|----------|---------|
| `runner/context.go` | runner | ~150 | CreateCompactCounter, CountCompactions, EstimateMaxContextFill, EnsureCompactHook, LogContextWarnings |
| `runner/context_test.go` | runner | ~200 | Unit tests for all functions |

---

## Изменения в существующих файлах

| File | Changes |
|------|---------|
| `session/result.go` | +`modelUsageEntry` struct, +`ModelUsage` in `jsonResultMessage`, +`ContextWindow` in `SessionMetrics`, extraction in `resultFromMessage` |
| `runner/metrics.go` | +TotalCompactions/MaxContextFillPct в TaskMetrics, RunMetrics, taskAccumulator. RecordSession +2 params. FinishTask/Finish агрегация |
| `runner/runner.go` | EnsureCompactHook вызов при старте. CreateCompactCounter/CountCompactions/EstimateMaxContextFill в execute/review paths. LogContextWarnings вызов |
| `runner/log.go` | SessionLogInfo +Compactions/MaxFillPct. SaveSessionLog header format |
| `config/config.go` | +ContextWarnPct, +ContextCriticalPct fields. Validate() extension |
| `config/defaults.yaml` | max_turns: 15, +context_warn_pct: 55, +context_critical_pct: 65 |
| `cmd/ralph/run.go` | Context line в formatSummary, цветной вывод |

---

## Риски и mitigation

| Риск | Mitigation |
|------|------------|
| Hook script не executable на WSL/NTFS | `os.Chmod(path, 0755)` — работает на WSL, проверить |
| settings.json corrupt после merge | JSON re-serialize через `json.MarshalIndent`. Тест: round-trip с существующим settings |
| RecordSession signature change ломает callers | 6 call sites в runner.go + 1 в serena.go. Обновить все одновременно. Return type `string` сохраняется |
| max_turns:15 — breaking default | Документировать в CHANGELOG. CLI --max-turns override. Resume компенсирует |
| Review сессия без counter | Решено: counter передаётся через `RunConfig.Env` → `session.Options.Env` |
| ContextWindow absent в старых Claude Code | Graceful: modelUsage=nil → ContextWindow=0 → fallback 200000. Тест с JSON без modelUsage |
| modelUsage camelCase vs usage snake_case | Отдельный struct `modelUsageEntry` с camelCase json tags |
| Формула ±10-15% | Пороги 55/65 учитывают погрешность. PreCompact hook = точный backup |

---

## Testing Strategy

- **Unit (context_test.go):** CreateCompactCounter (happy/error), CountCompactions (0/1/N/missing/corrupt), EstimateMaxContextFill (happy/edge/zero-turns/one-turn/nil-metrics), EnsureCompactHook (fresh/exists/outdated/corrupt-settings)
- **Unit (metrics_test.go):** RecordSession с compactions/fillPct, FinishTask агрегация, Finish агрегация
- **Unit (config_test.go):** Validate для context_warn_pct/context_critical_pct (range, ordering)
- **Unit (result_test.go):** ParseResult с/без `modelUsage` в JSON, с camelCase полями, с несколькими моделями, с пустым modelUsage
- **Integration:** Runner execute с mock binary → verify compactions=0, fill% в RunMetrics
- **Backward compat:** Все существующие тесты проходят без изменений (RecordSession callers обновляются)
