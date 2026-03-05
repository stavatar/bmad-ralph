# Observability & Metrics — Architecture Shard

**Epic:** 7 — Observability & Metrics
**PRD:** [docs/prd/observability-metrics.md](../prd/observability-metrics.md)
**Status:** Draft
**Date:** 2026-03-04

---

## Обзор

Метрики встраиваются в существующую архитектуру без новых пакетов и без новых зависимостей. Ключевой принцип: метрики — побочный продукт существующего data flow, а не параллельная система. Каждая точка сбора уже имеет данные — нужно только их захватить и агрегировать.

### Dependency Direction (без изменений)

```
cmd/ralph
├── runner (MetricsCollector, RunMetrics, TaskMetrics)
│   ├── session (SessionMetrics — extends ParseResult)
│   ├── gates (без изменений, runner инструментирует снаружи)
│   └── config (новые config fields: budget, thresholds, pricing)
└── bridge (без изменений)
```

Config остаётся leaf. Session расширяет свой ParseResult. Runner владеет агрегацией. cmd/ralph записывает JSON report и stdout summary.

---

## Решение 1: Типы метрик — размещение по пакетам

### session/result.go — SessionMetrics

```go
// SessionMetrics contains token usage and cost data extracted from Claude Code JSON output.
// Nil-safe: all consumers must handle nil (graceful degradation per FR42).
type SessionMetrics struct {
    InputTokens      int     `json:"input_tokens"`
    OutputTokens     int     `json:"output_tokens"`
    CacheReadTokens  int     `json:"cache_read_input_tokens"`
    CostUSD          float64 `json:"cost_usd"`
    NumTurns         int     `json:"num_turns"`
}
```

- Добавляется как `Metrics *SessionMetrics` field в `SessionResult`
- Извлекается из ParseResult — тот же JSON, дополнительные поля
- При отсутствии usage data в JSON: `Metrics == nil` (не ошибка)
- Расчёт `CostUSD` — в session, т.к. session знает модель (из Options)

**Обоснование:** Session парсит Claude CLI JSON — он единственный, кто видит raw output. Извлечение usage data — естественное расширение ParseResult, а не отдельный этап.

### runner/metrics.go — MetricsCollector, TaskMetrics, RunMetrics

```go
// DiffStats holds git diff statistics between two commits.
type DiffStats struct {
    FilesChanged int      `json:"files_changed"`
    Insertions   int      `json:"insertions"`
    Deletions    int      `json:"deletions"`
    Packages     []string `json:"packages"`
}

// ReviewFinding represents a single finding from code review.
type ReviewFinding struct {
    Severity string `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW
    Text     string `json:"text"`
}

// TaskMetrics holds aggregated metrics for a single task execution.
type TaskMetrics struct {
    Name           string            `json:"name"`
    Iterations     int               `json:"iterations"`
    DurationMs     int64             `json:"duration_ms"`
    CommitSHA      string            `json:"commit_sha,omitempty"`
    DiffStats      *DiffStats        `json:"diff_stats,omitempty"`
    ReviewCycles   int               `json:"review_cycles"`
    ReviewFindings []ReviewFinding   `json:"review_findings,omitempty"`
    GateDecision   string            `json:"gate_decision,omitempty"`
    GateWaitMs     int64             `json:"gate_wait_ms,omitempty"`
    Retries        int               `json:"retries"`
    Status         string            `json:"status"` // completed, skipped, reverted, failed
    TokensInput    int               `json:"tokens_input"`
    TokensOutput   int               `json:"tokens_output"`
    TokensCache    int               `json:"tokens_cache"`
    CostUSD        float64           `json:"cost_usd"`
    LatencyBreakdown *LatencyBreakdown `json:"latency,omitempty"`
}

// LatencyBreakdown holds per-phase timing for a task.
type LatencyBreakdown struct {
    PromptBuildMs  int64 `json:"prompt_build_ms"`
    SessionMs      int64 `json:"session_ms"`      // all execute sessions
    GitCheckMs     int64 `json:"git_check_ms"`
    ReviewMs       int64 `json:"review_ms"`        // all review sessions
    GateWaitMs     int64 `json:"gate_wait_ms"`
    DistillMs      int64 `json:"distill_ms"`
    BackoffMs      int64 `json:"backoff_ms"`
}

// GateStats holds aggregate gate decision statistics for a run.
type GateStats struct {
    Approve int   `json:"approve"`
    Retry   int   `json:"retry"`
    Skip    int   `json:"skip"`
    Quit    int   `json:"quit"`
    AvgWaitMs int64 `json:"avg_wait_ms"`
}

// ErrorStats holds aggregate error categorization for a run.
type ErrorStats struct {
    Transient  int `json:"transient"`
    Persistent int `json:"persistent"`
    Unknown    int `json:"unknown"`
}

// RunMetrics is the top-level metrics structure returned by Runner.Execute.
type RunMetrics struct {
    RunID           string        `json:"run_id"`
    StartTime       time.Time     `json:"start_time"`
    EndTime         time.Time     `json:"end_time"`
    TotalDurationMs int64         `json:"total_duration_ms"`
    Tasks           []TaskMetrics `json:"tasks"`
    TotalTokensIn   int           `json:"total_tokens_input"`
    TotalTokensOut  int           `json:"total_tokens_output"`
    TotalCostUSD    float64       `json:"total_cost_usd"`
    TasksCompleted  int           `json:"tasks_completed"`
    TasksFailed     int           `json:"tasks_failed"`
    TasksSkipped    int           `json:"tasks_skipped"`
    Gates           GateStats     `json:"gates"`
    Errors          ErrorStats    `json:"errors"`
}
```

### config/config.go — новые поля

```go
// Additions to Config struct:
BudgetMaxUSD       float64            `yaml:"budget_max_usd"`       // 0 = unlimited (default)
BudgetWarnPct      int                `yaml:"budget_warn_pct"`      // default 80
StuckThreshold     int                `yaml:"stuck_threshold"`      // default 2, 0 = disabled
SimilarityWindow   int                `yaml:"similarity_window"`    // default 0 (disabled)
SimilarityWarn     float64            `yaml:"similarity_warn"`      // default 0.85
SimilarityHard     float64            `yaml:"similarity_hard"`      // default 0.95
ModelPricing       map[string]Pricing `yaml:"model_pricing"`        // override built-in prices
```

```go
// config/pricing.go (new file, leaf — no deps)
type Pricing struct {
    InputPer1M  float64 `yaml:"input_per_1m"  json:"input_per_1m"`
    OutputPer1M float64 `yaml:"output_per_1m" json:"output_per_1m"`
    CachePer1M  float64 `yaml:"cache_per_1m"  json:"cache_per_1m"`
}
```

Встроенные цены — `var DefaultPricing map[string]Pricing` в `config/pricing.go`. Config merge: user pricing override встроенные per-model.

---

## Решение 2: MetricsCollector — injectable struct

```go
// runner/metrics.go

// MetricsCollector accumulates metrics during a run.
// Injectable into Runner for testability (analogous to KnowledgeWriter).
type MetricsCollector struct {
    runID     string
    startTime time.Time
    tasks     []TaskMetrics
    current   *taskAccumulator // mutable state for current task
    gates     GateStats
    errors    ErrorStats
    pricing   map[string]Pricing
}

// taskAccumulator is mutable state for the task currently being executed.
type taskAccumulator struct {
    name      string
    startTime time.Time
    tokens    [3]int    // input, output, cache — sum across sessions
    cost      float64
    latency   LatencyBreakdown
    // ... other accumulators
}
```

**Методы:**
- `NewMetricsCollector(runID string, pricing map[string]Pricing) *MetricsCollector`
- `StartTask(name string)` — начинает новую taskAccumulator
- `RecordSession(metrics *SessionMetrics, stepType string, durationMs int64)` — агрегирует tokens/cost
- `RecordGitDiff(stats *DiffStats)`
- `RecordReview(result ReviewResult, durationMs int64)`
- `RecordGate(action string, waitMs int64)`
- `RecordRetry(reason string)`
- `RecordError(category string)` — transient/persistent/unknown
- `RecordLatency(phase string, ms int64)` — добавляет к текущему task latency
- `FinishTask(status string, commitSHA string)` — финализирует TaskMetrics
- `Finish() RunMetrics` — возвращает итоговую структуру

**В Runner struct:**
```go
Runner struct {
    // ... existing fields ...
    Metrics *MetricsCollector // nil = no-op (analogous to NopLogger)
}
```

Nil-safe: все вызовы `r.Metrics.RecordSession(...)` обёрнуты в `if r.Metrics != nil`. Альтернативно — `NoOpMetricsCollector`, но nil-check проще и не требует interface.

**Обоснование:** Concrete struct, не interface. MetricsCollector не имеет внешних зависимостей, не делает I/O. Тестируемость через direct inspection (`collector.Finish()` в тесте). Interface не нужен — нет alternative implementations (в отличие от KnowledgeWriter с FileKnowledgeWriter vs NoOp).

---

## Решение 3: Расширение ParseResult

Текущий `jsonResultMessage`:
```go
type jsonResultMessage struct {
    Type      string `json:"type"`
    SessionID string `json:"session_id"`
    Result    string `json:"result"`
    IsError   bool   `json:"is_error"`
}
```

Claude Code JSON output (при `--output-format json`) может содержать usage data. Расширение:

```go
type jsonResultMessage struct {
    Type      string       `json:"type"`
    SessionID string       `json:"session_id"`
    Result    string       `json:"result"`
    IsError   bool         `json:"is_error"`
    // New fields for metrics extraction:
    Usage     *usageData   `json:"usage,omitempty"`
    Model     string       `json:"model,omitempty"`
    NumTurns  int          `json:"num_turns,omitempty"`
}

type usageData struct {
    InputTokens     int `json:"input_tokens"`
    OutputTokens    int `json:"output_tokens"`
    CacheReadTokens int `json:"cache_read_input_tokens"`
}
```

ParseResult расширяется: после извлечения SessionResult, если `msg.Usage != nil`, создаёт `SessionMetrics` и привязывает к result.

**Расчёт стоимости:** ParseResult принимает model name (из Options, прокидывается caller'ом) и pricing table для калькуляции `CostUSD`. Signature change:

```go
func ParseResult(raw *RawResult, elapsed time.Duration, pricing map[string]Pricing) (*SessionResult, error)
```

Либо: стоимость считает caller (runner) после получения SessionResult. **Выбор:** caller считает — ParseResult не знает про pricing, сохраняется чистота session пакета. `SessionMetrics.CostUSD` заполняется runner'ом.

---

## Решение 4: GitClient расширение

```go
// runner/git.go — расширение GitClient interface
type GitClient interface {
    HealthCheck(ctx context.Context) error
    HeadCommit(ctx context.Context) (string, error)
    RestoreClean(ctx context.Context) error
    DiffStats(ctx context.Context, before, after string) (*DiffStats, error) // NEW
}
```

**ExecGitClient.DiffStats:** выполняет `git diff --numstat <before> <after>`, парсит output (tab-separated: `insertions\tdeletions\tfilename`). Packages — unique parent dirs из filenames.

**MockGitClient:** расширяется полем `DiffStatsResult *DiffStats` и `DiffStatsError error`.

---

## Решение 5: ReviewResult enrichment

```go
// runner/runner.go — расширение ReviewResult
type ReviewResult struct {
    Clean    bool
    Findings []ReviewFinding // nil when Clean==true
}
```

`DetermineReviewOutcome` расширяется: когда `findingsNonEmpty == true`, парсит content для извлечения findings. Формат review-findings.md (установлен review prompt):

```markdown
### [HIGH] Missing error assertion
Description of the finding...

### [MEDIUM] Doc comment outdated
Description...
```

Regex: `(?m)^###\s*\[(\w+)\]\s*(.+)$` — извлекает severity и title.

**Обратная совместимость:** `Clean` остаётся primary signal. `Findings` — enrichment. Код, проверяющий `result.Clean`, продолжает работать.

---

## Решение 6: Stuck Detection (FR49)

Реализуется в `Runner.execute()` — внутренний loop уже отслеживает `headBefore`/`headAfter`:

```go
// Pseudocode in execute():
if headAfter == headBefore {
    consecutiveNoCommit++
    if r.Cfg.StuckThreshold > 0 && consecutiveNoCommit >= r.Cfg.StuckThreshold {
        r.Logger.Warn("stuck detected", kvs("task", taskText, "no_commit_count", consecutiveNoCommit)...)
        InjectFeedback(r.TasksFile, "No commit in last "+strconv.Itoa(consecutiveNoCommit)+" attempts. Try a different approach.")
        r.Metrics.RecordLatency("stuck_hint", 0)
    }
} else {
    consecutiveNoCommit = 0 // reset on success
}
```

Не заменяет MaxIterations. Дополняет ранним сигналом.

---

## Решение 7: Budget Alerts (FR50)

Проверка — после каждого `RecordSession`:

```go
// In Runner.execute(), after session completes:
if r.Cfg.BudgetMaxUSD > 0 && r.Metrics != nil {
    cumCost := r.Metrics.CumulativeCost()
    warnAt := r.Cfg.BudgetMaxUSD * float64(r.Cfg.BudgetWarnPct) / 100
    if cumCost >= r.Cfg.BudgetMaxUSD {
        // Emergency gate: budget exceeded
        r.Logger.Error("budget exceeded", kvs("cost", cumCost, "budget", r.Cfg.BudgetMaxUSD)...)
        // trigger emergency gate
    } else if cumCost >= warnAt {
        r.Logger.Warn("budget warning", kvs("cost", cumCost, "warn_at", warnAt)...)
    }
}
```

Budget check вставляется в существующий loop без изменения его структуры.

---

## Решение 8: Similarity Detection (FR51)

Новый файл `runner/similarity.go`:

```go
// JaccardSimilarity computes similarity between two sets of changed lines.
func JaccardSimilarity(a, b []string) float64

// SimilarityDetector tracks consecutive diffs for loop detection.
type SimilarityDetector struct {
    window    int       // from config.SimilarityWindow
    warnAt    float64   // from config.SimilarityWarn
    hardAt    float64   // from config.SimilarityHard
    history   [][]string // sliding window of diff lines
}
```

- `Push(diffLines []string)` — добавляет в sliding window
- `Check() (level string, score float64)` — `""` / `"warn"` / `"hard"`

Diff lines получаем из `git diff <before> <after>` (не numstat, а полный diff). Хранятся в памяти (window=3 — незначительный footprint).

Runner создаёт SimilarityDetector при `cfg.SimilarityWindow > 0`. Проверяет после каждого commit.

---

## Решение 9: Error Categorization (FR52)

Простой подход через error prefix matching:

```go
// runner/metrics.go
func CategorizeError(err error) string {
    msg := err.Error()
    switch {
    case strings.Contains(msg, "rate limit"),
         strings.Contains(msg, "timeout"),
         strings.Contains(msg, "API error"),
         strings.Contains(msg, "connection"):
        return "transient"
    case strings.Contains(msg, "config"),
         strings.Contains(msg, "not found"),
         strings.Contains(msg, "permission"):
        return "persistent"
    default:
        return "unknown"
    }
}
```

Расширяемый — новые patterns добавляются без структурных изменений.

---

## Решение 10: Run ID и Trace Correlation (FR47-FR48)

- **Run ID:** UUID v4, генерируется в `cmd/ralph/run.go` при старте, передаётся в Runner через Config или MetricsCollector
- **Task ID:** Извлекается из task text (task description) — уже уникально
- **Session ID:** Из `SessionResult.SessionID` — Claude Code присваивает
- **step_type:** Строковая константа (`"execute"`, `"review"`, `"gate"`, etc.)

RunLogger расширяется: `Info(msg, kvs...)` автоматически добавляет `run_id` если установлен. Task ID добавляется caller'ом через `kv("task_id", ...)`.

**Реализация:** `RunLogger` получает field `runID string` при создании. Каждый `write()` prepend'ит `run_id=...`.

---

## Решение 11: Latency Breakdown (FR54)

Инструментация через simple `time.Now()` / `time.Since()` pairs в существующих точках:

```go
// Example in Runner.execute():
t0 := time.Now()
prompt, err := buildTemplateData(...)
r.Metrics.RecordLatency("prompt_build", time.Since(t0).Milliseconds())

t1 := time.Now()
raw, err := session.Execute(ctx, opts)
r.Metrics.RecordLatency("session", time.Since(t1).Milliseconds())
```

Нет отдельного timer framework. Простые замеры в точках, которые уже выделены в коде.

---

## Решение 12: JSON Report и stdout Summary (FR55-FR56)

**Runner.Execute() signature change:**

```go
// Before:
func (r *Runner) Execute(ctx context.Context) error

// After:
func (r *Runner) Execute(ctx context.Context) (*RunMetrics, error)
```

Runner возвращает `*RunMetrics` (nil при ошибке до начала сбора). `cmd/ralph/run.go`:
1. Получает `RunMetrics` из `Runner.Execute()`
2. Сериализует в JSON → `<logDir>/ralph-run-<runID>.json`
3. Печатает text summary в stdout (через `fatih/color`)

Это соответствует принципу: packages return data, `cmd/` decides output.

---

## Data Flow с метриками

```
cmd/ralph/run.go
  │ creates MetricsCollector(runID, pricing)
  │ passes to Runner
  ▼
Runner.Execute(ctx)
  │
  ├── StartTask(taskText)
  │     │
  │     ├── time prompt_build → RecordLatency("prompt_build", ms)
  │     │
  │     ├── session.Execute(ctx, opts)
  │     │     └── ParseResult → SessionResult{..., Metrics: &SessionMetrics{...}}
  │     │
  │     ├── RecordSession(sessionMetrics, "execute", durationMs)
  │     │     └── accumulates tokens, cost (via pricing table)
  │     │
  │     ├── time git_check → RecordLatency("git_check", ms)
  │     ├── git.HeadCommit → commit changed?
  │     │     YES: git.DiffStats → RecordGitDiff(stats)
  │     │     NO:  stuck check → maybe InjectFeedback
  │     │
  │     ├── session.Execute (review)
  │     │     └── ParseResult → SessionMetrics
  │     ├── RecordSession(sessionMetrics, "review", durationMs)
  │     ├── DetermineReviewOutcome → ReviewResult{Clean, Findings}
  │     ├── RecordReview(result, durationMs)
  │     │
  │     ├── budget check → maybe emergency gate
  │     ├── similarity check → maybe warn/gate
  │     │
  │     ├── gates.Prompt → GateDecision
  │     ├── RecordGate(action, waitMs)
  │     │
  │     └── FinishTask(status, commitSHA)
  │
  ├── Finish() → RunMetrics
  │
  └── return &RunMetrics, nil

cmd/ralph/run.go
  │ receives RunMetrics
  ├── json.MarshalIndent → write to logDir/ralph-run-<runID>.json
  └── print text summary to stdout
```

---

## FR → Package/File Mapping

| FR | Package | File(s) | Тип изменения |
|----|---------|---------|---------------|
| FR42 | session | result.go | Extend ParseResult, add SessionMetrics |
| FR43 | config | pricing.go (new) | DefaultPricing, Pricing struct |
| FR44 | runner | git.go | DiffStats method + struct |
| FR45 | runner | metrics.go | RecordSession accumulates cost |
| FR46 | runner | runner.go | ReviewResult, DetermineReviewOutcome enrichment |
| FR47 | runner | log.go, metrics.go | RunLogger.runID, MetricsCollector.runID |
| FR48 | runner | log.go | step_type in write() |
| FR49 | runner | runner.go | stuck detection in execute() |
| FR50 | runner, config | runner.go, config.go | budget check in loop, config fields |
| FR51 | runner | similarity.go (new) | SimilarityDetector |
| FR52 | runner | metrics.go | CategorizeError function |
| FR53 | runner | runner.go, metrics.go | RecordGate in gate paths |
| FR54 | runner | runner.go | time.Now/Since pairs + RecordLatency |
| FR55 | cmd/ralph | run.go | JSON write after Execute returns |
| FR56 | cmd/ralph | run.go | stdout summary via fatih/color |

---

## Новые файлы

| File | Package | LOC est. | Content |
|------|---------|----------|---------|
| `config/pricing.go` | config | ~60 | Pricing struct, DefaultPricing map |
| `config/pricing_test.go` | config | ~40 | Default values, merge logic |
| `runner/metrics.go` | runner | ~250 | MetricsCollector, types, CategorizeError |
| `runner/metrics_test.go` | runner | ~200 | Unit tests for collector |
| `runner/similarity.go` | runner | ~80 | JaccardSimilarity, SimilarityDetector |
| `runner/similarity_test.go` | runner | ~100 | Similarity edge cases |

---

## Изменения в существующих файлах

| File | Changes |
|------|---------|
| `session/result.go` | +SessionMetrics, +usageData, extend ParseResult, extend jsonResultMessage |
| `runner/runner.go` | +ReviewFinding, extend ReviewResult, extend Runner struct (+Metrics), instrument execute(), stuck detection, budget check |
| `runner/git.go` | +DiffStats to GitClient, implement in ExecGitClient |
| `runner/log.go` | +runID field, extend write() with run_id, task_id |
| `config/config.go` | +7 config fields (budget, stuck, similarity, pricing) |
| `cmd/ralph/run.go` | Create MetricsCollector, receive RunMetrics, write JSON, print summary |
| `internal/testutil/mock_git.go` | +DiffStats mock method |

---

## Риски и mitigation

| Риск | Mitigation |
|------|------------|
| Claude Code JSON format change (usage fields) | `Metrics == nil` graceful degradation. Golden file тесты на JSON contract |
| Runner.Execute signature change (returns RunMetrics) | Все callers — в cmd/ralph (контролируем). Единственный breaking change |
| ParseResult signature change (pricing param) | Отказ: caller считает cost, ParseResult не меняет signature |
| GitClient interface expansion | MockGitClient обновляется. Единственный consumer — runner |
| MetricsCollector nil safety | Все вызовы guarded by `if r.Metrics != nil`. Тесты без collector работают без изменений |

---

## Testing Strategy

- **Unit:** MetricsCollector methods, JaccardSimilarity, CategorizeError, Pricing merge, DiffStats parse
- **Integration:** Runner с MetricsCollector — verify RunMetrics structure after happy path / error path
- **Golden files:** JSON report schema validation
- **ParseResult:** Extend existing golden files with usage data fields
- **Backward compat:** All existing tests pass without MetricsCollector (nil guard)
