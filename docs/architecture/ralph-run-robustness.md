# Ralph Run Robustness — Architecture Shard

**Epic:** 9 — Ralph Run Robustness
**PRD:** [docs/prd/ralph-run-robustness.md](../prd/ralph-run-robustness.md)
**Research:** [docs/research/ralph-run-problems-and-solutions.md](../research/ralph-run-problems-and-solutions.md)
**Status:** Draft
**Date:** 2026-03-06

---

## Обзор

Epic 9 решает 4 системные проблемы, выявленные при тестировании Ralph Run на реальном проекте (mentorlearnplatform, 15 задач, 3 рана, $38.44). Все изменения встраиваются в существующую архитектуру без новых пакетов и без новых зависимостей. Ключевой принцип: расширяем существующие точки в runner, не создаём параллельные системы.

### Dependency Direction (без изменений)

```
cmd/ralph
├── runner
│   ├── session (Options.Env — новое поле для env vars)
│   ├── gates (без изменений)
│   └── config (max_review_iterations: 3→6, новые severity constants)
└── bridge (без изменений)
```

Config остаётся leaf. Session получает одно новое поле. Runner реализует всю новую логику. cmd/ralph без изменений.

---

## Область 1: Pre-flight проверка и маркер задачи (FR67-FR69)

### Проблема

Ralph тратит $1-2 на задачу, которая уже закоммичена, потому что sprint-tasks.md рассинхронизирован с git history после перегенерации.

### Компоненты

#### runner/preflight.go — новый файл

```go
// TaskHash returns first 6 hex chars of SHA-256 of the task description text.
// Description is extracted by stripping the leading "- [ ] " or "- [x] " prefix.
func TaskHash(taskText string) string

// PreFlightCheck examines git log for a commit with [task:<hash>] marker.
// Returns (skip bool, reason string).
// skip=true when commit found AND review-findings.md is absent/empty.
// Uses GitClient.HeadCommit equivalent — new method GitClient.LogOneline(n int).
func PreFlightCheck(git GitClient, taskText, projectRoot string) (skip bool, reason string, err error)
```

- `TaskHash`: `crypto/sha256` → `hex.EncodeToString`[:12] → первые 6 символов
- `PreFlightCheck`: вызывает `git log --oneline -20`, ищет `[task:<hash>]`, проверяет `review-findings.md`

#### runner/scan.go — расширение ScanTasks

```go
// SmartMergeStatus preserves [x] marks from old sprint-tasks.md when regenerating.
// Matches tasks by TaskHash. Returns updated content with [x] preserved.
func SmartMergeStatus(oldContent, newContent string) string
```

- Матчинг задач по хэшу текста (тот же `TaskHash`)
- Вызывается из bridge при перегенерации sprint-tasks.md

#### Интеграция в runner/runner.go

Точка вызова: `(*Runner).Execute()` — перед циклом execute→review для каждой задачи.

```
Execute()
  for each task in openTasks:
    hash := TaskHash(task.Text)          // FR67
    skip, reason := PreFlightCheck(...)  // FR68
    if skip:
      markTaskDone(task)                 // FR68.3
      log "INFO pre-flight skip: %s", reason
      continue
    // ... existing execute→review cycle
```

#### Изменения в execute.md промпте

Добавление требования маркера `[task:__TASK_HASH__]` в commit message:

```markdown
## Commit Rules
...
В конце commit message добавь маркер: [task:__TASK_HASH__]
Пример: feat: add user validation [task:a1b2c3]
```

`__TASK_HASH__` — новый placeholder в `buildTemplateData()`, вычисляется через `TaskHash(task.Text)`.

#### GitClient — новый метод

```go
// LogOneline returns last n commits as one-line strings.
LogOneline(n int) ([]string, error)
```

Реализация в `runner/git.go`: `git log --oneline -<n>`.

### Data Flow

```
sprint-tasks.md → ScanTasks() → TaskEntry
                                    ↓
                               TaskHash(Text) → "a1b2c3"
                                    ↓
                        PreFlightCheck(git, text, root)
                           ↓                    ↓
                     git log --oneline    os.ReadFile(review-findings.md)
                           ↓                    ↓
                   contains [task:a1b2c3]?   empty/absent?
                           ↓
                    skip=true / false
```

---

## Область 2: Кросс-платформенные пути (FR70-FR71)

### Проблема

`AutoDistill` и другие файловые операции используют строковую конкатенацию путей, что ломает WSL/Windows.

### Изменения

Рефакторинг существующего кода — **без новых типов и файлов**.

#### runner/runner.go, runner/knowledge_distill.go, runner/knowledge_write.go

Заменить все `projectRoot + "/filename"` на `filepath.Join(projectRoot, "filename")`.

Места для замены (поиск по `+ "/"` и `+ "\\"` и конкатенации с путями):

| Файл | Текущий код | Замена |
|------|------------|--------|
| runner.go | `filepath.Join` уже используется в `DetermineReviewOutcome` | Проверить остальные места |
| knowledge_distill.go | Конструирование путей к LEARNINGS.md | `filepath.Join` |
| knowledge_write.go | Конструирование путей к `.ralph/rules/` | `filepath.Join` |

#### Graceful degradation (FR71)

Некритические операции при ошибке → skip + WARN:

```go
// В AutoDistill, WriteLearnings и аналогичных:
if err != nil {
    if errors.Is(err, os.ErrNotExist) {
        log.Printf("WARN: %s not found, skipping: %v", path, err)
        return nil // graceful skip
    }
    return err // реальная ошибка — пробрасываем
}
```

Список некритических операций:
- `AutoDistill` — дистилляция LEARNINGS.md
- `WriteLearnings` — запись knowledge
- `WriteRules` — запись rules
- Чтение `.claude/settings.json` для Serena detection

### Принцип

Нет нового кода — только замена строковых операций на `filepath.Join`/`filepath.Abs` и добавление `os.IsNotExist` guard в best-effort операциях.

---

## Область 3: Прогрессивная схема review — Анти-гидра (FR72-FR76)

### Проблема

Review цикл не сходится: замечания 3→2→4→1→0, стоимость одной задачи $20+ при 5 циклах. LLM недетерминирован — каждый раз находит «новые» замечания.

### Компоненты

#### config/defaults.yaml — изменение дефолта

```yaml
max_review_iterations: 6  # было 3
```

#### runner/progressive.go — новый файл

```go
// SeverityLevel represents finding severity for progressive filtering.
type SeverityLevel int

const (
    SeverityLow SeverityLevel = iota
    SeverityMedium
    SeverityHigh
    SeverityCritical
)

// ParseSeverity converts string severity (from review-findings.md) to SeverityLevel.
func ParseSeverity(s string) SeverityLevel

// ProgressiveParams returns review parameters for the given cycle number.
// cycle is 1-based. maxCycles is total allowed cycles.
// Returns: minSeverity, maxFindings, useIncrementalDiff, useHighEffort.
func ProgressiveParams(cycle, maxCycles int) (minSeverity SeverityLevel, maxFindings int, incrementalDiff bool, highEffort bool)

// FilterBySeverity removes findings below minSeverity threshold.
// Filtered findings are logged but not returned.
func FilterBySeverity(findings []ReviewFinding, minSeverity SeverityLevel) []ReviewFinding

// TruncateFindings limits findings to maxCount, keeping highest severity first.
func TruncateFindings(findings []ReviewFinding, maxCount int) []ReviewFinding
```

Таблица прогрессии (дефолт для `maxCycles=6`):

| Цикл | minSeverity | maxFindings | incrementalDiff | highEffort |
|------|-------------|-------------|-----------------|------------|
| 1 | LOW | 5 | false | false |
| 2 | LOW | 5 | false | false |
| 3 | MEDIUM | 3 | true | true |
| 4 | HIGH | 1 | true | true |
| 5 | CRITICAL | 1 | true | true |
| 6 | CRITICAL | 1 | true | true |

При `maxCycles != 6` пропорции масштабируются: первые ~33% = LOW, ~50% = MEDIUM, ~67% = HIGH, остальные = CRITICAL.

#### Интеграция в runner/runner.go

**Точка 1: `(*Runner).Execute()` — цикл execute→review**

```
Execute()
  for cycle := 1; cycle <= maxIter; cycle++:
    params := ProgressiveParams(cycle, maxIter)  // FR72

    // FR76: high effort на поздних циклах
    env := map[string]string{}
    if params.highEffort:
      env["CLAUDE_CODE_EFFORT_LEVEL"] = "high"

    // FR74: scope lock — выбор diff
    diff := fullTaskDiff()
    if params.incrementalDiff:
      diff = incrementalDiff()  // git diff HEAD~1..HEAD

    // execute session с env
    raw := session.Execute(opts)  // opts.Env = env

    // review session
    reviewResult := ReviewFn(...)

    // FR73: severity filtering
    filtered := FilterBySeverity(reviewResult.Findings, params.minSeverity)

    // FR75: budget truncation
    truncated := TruncateFindings(filtered, params.maxFindings)

    if len(truncated) == 0:
      break  // clean
```

**Точка 2: `RealReview()` — передача diff и контекста**

На циклах 3+ review получает:
- Инкрементальный diff (`git diff HEAD~1..HEAD`)
- Описание задачи и ссылку на story
- Findings предыдущего цикла
- Инструкцию: «проверь корректность исправлений и отсутствие новых проблем уровня <порог>+»

Это реализуется через новые поля в `RunConfig`:

```go
type RunConfig struct {
    // ... existing fields
    Cycle           int            // текущий номер цикла (1-based)
    MinSeverity     SeverityLevel  // порог severity для текущего цикла
    MaxFindings     int            // бюджет замечаний
    IncrementalDiff bool           // использовать инкрементальный diff
    PrevFindings    string         // текст предыдущих findings (для контекста)
}
```

**Точка 3: `selectReviewModel()` — эскалация модели**

```go
func selectReviewModel(cfg *config.Config, ds *DiffStats, isGate bool, hydraDetected bool, highEffort bool) string {
    if isGate || hydraDetected || highEffort || ds == nil || cfg.ModelReviewLight == "" {
        return cfg.ModelReview
    }
    // ... existing light model logic
}
```

Параметр `highEffort` добавляется — на циклах 3+ всегда полная модель.

#### session/session.go — новое поле Env

```go
type Options struct {
    // ... existing fields
    Env map[string]string // дополнительные переменные окружения для процесса
}
```

В `session.Execute()` при создании `exec.Cmd`:
```go
if len(opts.Env) > 0 {
    cmd.Env = append(os.Environ(), envToSlice(opts.Env)...)
}
```

Это единственное изменение в пакете `session`. Используется для `CLAUDE_CODE_EFFORT_LEVEL=high`.

**Точка 4: `DetermineReviewOutcome()` — severity filtering**

Текущий `DetermineReviewOutcome` парсит findings через `findingSeverityRe`. После парсинга вызываются `FilterBySeverity` и `TruncateFindings`:

```go
// В DetermineReviewOutcome или в caller (Execute):
findings = FilterBySeverity(findings, params.minSeverity)
findings = TruncateFindings(findings, params.maxFindings)
```

**Решение:** Фильтрация в caller (`Execute`), не в `DetermineReviewOutcome`. Это сохраняет DetermineReviewOutcome чистым (парсинг без бизнес-логики).

### Data Flow прогрессивной схемы

```
Execute() loop
  ↓
ProgressiveParams(cycle, maxCycles)
  → minSeverity, maxFindings, incrementalDiff, highEffort
  ↓
execute session (+ CLAUDE_CODE_EFFORT_LEVEL env)
  ↓
RealReview (incrementalDiff → git diff HEAD~1..HEAD / full diff)
  ↓
DetermineReviewOutcome → raw findings []ReviewFinding
  ↓
FilterBySeverity(findings, minSeverity) → filtered
  ↓
TruncateFindings(filtered, maxFindings) → truncated
  ↓
len(truncated) == 0 → clean / continue cycle
```

---

## Область 4: Защита от scope creep (FR79-FR80)

### Проблема

Claude реализует соседнюю задачу из sprint-tasks.md вместо текущей.

### Изменения — только промпты

#### runner/prompts/execute.md — новый блок

```markdown
## SCOPE BOUNDARY (MANDATORY)

Реализуй ТОЛЬКО текущую задачу: __TASK__
НЕ реализуй другие задачи из sprint-tasks.md, даже если они кажутся связанными.
Если текущая задача зависит от другой — остановись и сообщи, не делай обе.

Перед коммитом проверь: каждый изменённый файл и каждое изменение
напрямую связаны с текущей задачей. Если обнаружишь изменения для другой
задачи — откати их через git checkout.
```

#### runner/prompts/agents/implementation.md — расширение

Добавить пункт в scope проверки:

```markdown
- Проверить что ВСЕ изменения в diff относятся к AC текущей задачи
- Если обнаружены изменения, реализующие другую задачу из sprint-tasks.md — это finding severity HIGH
- Формулировка: "Scope creep: изменения в <файл> реализуют задачу '<другая задача>', а не текущую"
```

### Без изменений в Go-коде

Двойная защита: инструкция в execute + проверка в review agent. Обе реализуются исключительно через промпты, без нового Go-кода.

---

## Область 5: Статистика по review агентам (FR77-FR78)

### Проблема

Нет данных о том, какие review sub-agents генерируют больше всего замечаний.

### Компоненты

#### runner/metrics.go — расширение ReviewFinding

```go
type ReviewFinding struct {
    Severity    string `json:"severity"`
    Description string `json:"description"`
    File        string `json:"file"`
    Line        int    `json:"line"`
    Agent       string `json:"agent,omitempty"` // FR77: quality|implementation|simplification|design-principles|test-coverage
}
```

#### runner/metrics.go — AgentStats в RunMetrics

```go
// AgentFindingStats holds per-severity counts for a single review agent.
type AgentFindingStats struct {
    Critical int `json:"critical"`
    High     int `json:"high"`
    Medium   int `json:"medium"`
    Low      int `json:"low"`
}

type RunMetrics struct {
    // ... existing fields
    AgentStats map[string]*AgentFindingStats `json:"agent_stats,omitempty"` // FR78
}
```

#### runner/runner.go — парсинг поля Agent

Расширение `findingSeverityRe` или отдельный regex для парсинга `- **Агент**: <name>`:

```go
// Текущий regex: (?m)^###\s*\[(\w+)\]\s*(.+)$
// Дополнительный regex для агента:
findingAgentRe = regexp.MustCompile(`(?m)^\s*-\s*\*\*Агент\*\*:\s*(\S+)`)
```

Парсинг в `DetermineReviewOutcome`: после нахождения `### [SEVERITY] Title` ищем `- **Агент**:` в следующих строках до следующего `###`.

#### MetricsCollector — аккумуляция agent stats

```go
// RecordAgentFinding accumulates per-agent severity counts.
func (mc *MetricsCollector) RecordAgentFinding(agent, severity string)
```

Вызывается из `Execute()` после парсинга findings.

#### Изменения в промптах

**runner/prompts/review.md** — расширение формата findings:

```markdown
### [SEVERITY] Finding title
- **Описание**: ...
- **Файл**: ...
- **Строка**: ...
- **Агент**: quality|implementation|simplification|design-principles|test-coverage
```

**runner/prompts/agents/*.md** — каждый sub-agent добавляет `- **Агент**: <name>` в свои findings.

### Data Flow

```
review sub-agents → review-findings.md (с полем Агент)
  ↓
DetermineReviewOutcome → []ReviewFinding (с Agent field)
  ↓
MetricsCollector.RecordAgentFinding(agent, severity)
  ↓
RunMetrics.AgentStats → JSON log (.ralph/logs/ralph-run-<id>.json)
```

---

## Сводка изменений по файлам

| Файл | Изменения | FR |
|------|-----------|-----|
| **Новые файлы** | | |
| `runner/preflight.go` | TaskHash, PreFlightCheck, SmartMergeStatus | FR67, FR68, FR69 |
| `runner/progressive.go` | SeverityLevel, ProgressiveParams, FilterBySeverity, TruncateFindings | FR72, FR73, FR75 |
| **Существующие файлы** | | |
| `config/defaults.yaml` | `max_review_iterations: 6` | FR72 |
| `session/session.go` | `Options.Env map[string]string` | FR76 |
| `runner/runner.go` | Pre-flight в Execute(), progressive params в цикле, agent parsing в DetermineReviewOutcome, RunConfig расширение | FR68, FR72-FR76, FR78 |
| `runner/metrics.go` | `ReviewFinding.Agent`, `AgentFindingStats`, `RunMetrics.AgentStats`, `RecordAgentFinding` | FR77, FR78 |
| `runner/git.go` | `LogOneline(n int)` | FR68 |
| `runner/knowledge_distill.go` | `filepath.Join`, graceful degradation | FR70, FR71 |
| `runner/knowledge_write.go` | `filepath.Join`, graceful degradation | FR70, FR71 |
| `runner/prompts/execute.md` | SCOPE BOUNDARY блок, `[task:__TASK_HASH__]` маркер | FR67, FR79 |
| `runner/prompts/review.md` | Формат findings с полем Агент, инкрементальный diff контекст | FR74, FR77 |
| `runner/prompts/agents/implementation.md` | Scope compliance проверка | FR80 |
| `runner/prompts/agents/*.md` | Добавление `- **Агент**: <name>` в findings | FR77 |

---

## Риски реализации

| Риск | Митигация |
|------|-----------|
| `session.Options.Env` ломает существующие тесты | Env = nil → без изменений поведения (zero-value safe) |
| `ProgressiveParams` пропорциональное масштабирование при maxCycles≠6 | Покрыть тестами: maxCycles=3, 6, 10 |
| `findingAgentRe` не находит поле при отсутствии | Agent = "unknown" по умолчанию (FR78) |
| `LogOneline` зависит от наличия git | Pre-flight = best-effort, ошибка → proceed (не skip) |
| `FilterBySeverity` пропускает LOW findings на поздних циклах | LOW findings логируются (INFO), доступны для ручного review |

---

## Порядок реализации (stories)

| Приоритет | Stories | Область | Зависимости |
|-----------|---------|---------|-------------|
| P0 | 9.1, 9.2 | Progressive review + severity filtering | config (defaults.yaml) |
| P0 | 9.3 | Scope lock (инкрементальный diff) + effort escalation | 9.1 |
| P0 | 9.4 | session.Options.Env для CLAUDE_CODE_EFFORT_LEVEL | Нет |
| P1 | 9.5 | filepath normalization + graceful degradation | Нет |
| P1 | 9.6 | Scope creep защита (промпты) | Нет |
| P2 | 9.7 | Pre-flight + TaskHash + LogOneline | Нет |
| P2 | 9.8 | Smart merge [x] preservation | 9.7 |
| P3 | 9.9 | Agent stats (ReviewFinding.Agent, parsing, metrics) | 9.1 |
