---
project_name: 'bmad-ralph'
user_name: 'Степан'
date: '2026-03-07'
source: 'docs/architecture.md'
status: 'complete (Epics 1-7); v2 architecture ready'
optimized_for_llm: true
---

# Project Context for AI Agents

_Критические правила bmad-ralph. Только неочевидное — стандартные Go conventions не дублируются._

---

## Technology Stack

- **Go 1.25** в go.mod (совместимость с 1.25 + 1.26)
- **Cobra** — CLI framework (subcommands `run`, `plan`; `bridge` удаляется в v2)
- **yaml.v3** — config parsing (без Viper, свой каскад)
- **fatih/color** — цветной вывод (mattn/go-isatty — транзитивная, используется для spinner)
- **text/template** + **strings.Replace** — двухэтапная prompt assembly
- **golangci-lint** — линтинг, **goreleaser** — distribution
- **Всего 3 direct deps.** Новые зависимости требуют обоснования

## Architecture — Ключевые решения

### Dependency Direction (строго сверху вниз, циклы запрещены)

```
cmd/ralph
├── runner
│   ├── session
│   ├── gates
│   └── config
├── plan          ← [NEW v2]
│   ├── session
│   └── config
└── bridge        ← [DELETE v2]
    ├── session
    └── config
```

- `config` = leaf package, не зависит ни от кого
- `session` и `gates` НЕ зависят друг от друга
- `cmd/ralph/` — единственное место для exit code mapping (0-4) и output decisions

### Package Entry Points (минимальная exported API surface)

| Package | Entry Point |
|---------|-------------|
| `plan` | `plan.Run(ctx, cfg, opts)` ← [NEW v2] |
| `bridge` | `bridge.Run(ctx, cfg)` ← [DELETE v2] |
| `runner` | `runner.Run(ctx, cfg)` |
| `session` | `session.Execute(ctx, opts)` |
| `gates` | `gates.Prompt(ctx, gate)` |
| `config` | `config.Load(flags)` |

Не экспортировать внутренние helpers. Если package экспортирует >10 функций — нарушение single responsibility.

### Config Immutability

`config.Config` парсится **один раз** при старте, передаётся by pointer, **никогда не мутируется в runtime**.

### Двухэтапная Prompt Assembly (КРИТИЧНО)

1. **Этап 1:** `text/template` для структуры промпта (`{{if .SerenaEnabled}}`)
2. **Этап 2:** `strings.Replace` для user content injection (LEARNINGS.md, CLAUDE.md, review-findings.md)

**ПОЧЕМУ:** User-controlled файлы могут содержать `{{` — text/template crashed бы. Двухэтапность защищает от template injection.

### Prompt файлы = Go templates, не чистый Markdown

Файлы в `*/prompts/` содержат `{{.Var}}` синтаксис. Не предназначены для GitHub preview.

## Naming Conventions

| Что | Паттерн | Пример |
|-----|---------|--------|
| Interfaces | В пакете-потребителе | `GitClient` в `runner/`, не в `git/` |
| Mocks | `Mock` prefix | `MockGitClient` |
| Sentinel errors | `Err` prefix, package scope | `var ErrNoTasks = errors.New("no tasks")` |
| Error wrapping | `package: operation: %w` | `fmt.Errorf("runner: execute: %w", err)` |
| Tests | `Test<Type>_<Method>_<Scenario>` | `TestRunner_Execute_WithFindings` |
| Golden files | `testdata/TestName.golden` | `runner/testdata/TestPrompt_Execute.golden` |

## Error Handling

- **Wrapping:** всегда `fmt.Errorf("pkg: op: %w", err)` — package prefix + контекст
- **Type checking:** только `errors.Is` / `errors.As`. НИКОГДА string matching
- **Panic:** НИКОГДА в runtime. Только `init()` для missing embed
- **Exit codes:** только в `cmd/ralph/`. Packages возвращают errors, не вызывают `os.Exit`
- **Custom error types:** только для branch logic — `*ExitCodeError`, `*GateDecision`

## Subprocess

- **Context:** `main()` создаёт root ctx через `signal.NotifyContext`. Прокидывает через параметры
- **Все subprocess:** через `exec.CommandContext(ctx)`. Без `exec.Command`, без `context.TODO()` в prod
- **Working dir:** `cmd.Dir = config.ProjectRoot` — всегда явно
- **Таймауты:** без wall-clock timeout. Git — быстрый, Claude — `--max-turns`. Cancellation только через ctx
- **Git:** через `GitClient` interface → `ExecGitClient` (prod) + `MockGitClient` (test)
- **Claude:** через `session.Execute` с `--output-format json`

## File I/O

- UTF-8, no BOM, `\n` line endings
- `os.ReadFile` / `os.WriteFile` с `0644` (файлы маленькие, atomic не нужен — git = backup)
- Scan: `strings.Split(content, "\n")` + regex. Без bufio.Scanner
- Paths: относительно `config.ProjectRoot` (определяется один раз при старте)
- Отсутствующий файл: `os.Stat` + `errors.Is(err, os.ErrNotExist)` → пуст, не ошибка

## String Constants (config/constants.go)

```go
const (
    TaskOpen       = "- [ ]"
    TaskDone       = "- [x]"
    GateTag        = "[GATE]"
    FeedbackPrefix = "> USER FEEDBACK:"
)
```

Regex patterns для scan — `var` с `regexp.MustCompile` в package scope. НИКОГДА inline compile.

## Logging & Output

- **stdout** = user-facing (через `fatih/color`). **stderr** не используем
- **Log file** = `fmt.Fprintf(logFile, "%s %s %s\n", ts, level, msg)`. Append-only
- **Packages НЕ логируют** — возвращают results/errors. `cmd/ralph/` решает что в stdout, что в log
- **Claude output:** не логируем целиком. Только session_id, exit code, duration, наличие коммита

## Testing

- **Table-driven** по умолчанию (`[]struct{name; ...}` + `t.Run`)
- **Golden files:** `go test -update` для обновления, CI без `-update` = mismatch fail
- **Assertions:** Go stdlib `if got != want { t.Errorf }`. Без testify — zero test deps
- **Isolation:** `t.TempDir()` для каждого теста. Без shared state
- **Mock Claude:** scenario-based JSON (ordered responses). Подставляется через `config.ClaudeCommand`
- **Mock Git:** `MockGitClient` implements `GitClient` interface
- **Coverage:** `runner` и `config` >80%, `cmd/ralph/` minimal

## LEARNINGS.md Budget

- **Hard limit: 200 строк.** При превышении → distillation session (`claude -p`)
- **Distillation target:** ~100 строк (50% бюджета)
- Hardcoded const в `runner/knowledge.go` (MVP). Configurable в Growth
- **ОПАСНЫЙ FEEDBACK LOOP:** больше ошибок → больше learnings → меньше context → больше ошибок. Budget = circuit breaker

## Anti-Patterns (ЗАПРЕЩЕНО)

- `gates` импортирует `runner` — circular dependency
- `session` читает config files напрямую — принимает options struct, config заполняет
- `os.Exit` в package — возвращай error
- `exec.Command` без context — всегда `exec.CommandContext(ctx)`
- `if err.Error() == "..."` — используй `errors.Is`/`errors.As`
- Мутировать config в runtime
- Inline `regexp.Compile` — только `regexp.MustCompile` в package scope
- Новые зависимости без обоснования
- `os.ReadFile` внутри `plan/` — только `cmd/ralph/plan.go` читает файлы
- `os.WriteFile` в `plan/merge.go` или `plan/size.go` — только через `writeAtomic()` в `plan/plan.go`
- User content через `template.Execute` — только через `strings.Replace` после рендера шаблона

## Observability & Metrics

### MetricsCollector (nil-safe injectable)

`MetricsCollector` инжектируется через `Runner.Metrics`. Если `nil` — все методы no-op (nil receiver pattern). Это reference architecture для optional подсистем.

```
StartTask → RecordSession/RecordDiff/RecordFindings/RecordGate/RecordError → FinishTask
→ Finish() → RunMetrics (JSON-сериализуемые)
```

Правила:
- `StartTask` всегда имеет парный `FinishTask` на **всех** code paths (включая error paths)
- Сессии с parseable output записывают метрики **до** retry (не теряем данные)
- `Finish()` вызывается **один раз** — двойной вызов в тестах = баг

### Pricing (config/pricing.go)

`Pricing{InputPer1M, OutputPer1M, CachePer1M}` — стоимость за 1M токенов.
- `DefaultPricing` — встроенная таблица моделей
- `MergePricing(base, override)` — пользовательские цены поверх default
- `MostExpensiveModel` — conservative estimate для неизвестных моделей

### SimilarityDetector (runner/similarity.go)

Jaccard similarity на множествах токенов (whitespace-split). Скользящее окно из `similarity_window` последних промптов. Два порога:
- `similarity_warn` (0.85) — логирует предупреждение
- `similarity_hard` (0.95) — пропуск задачи (abort)

При `similarity_window: 0` — отключён.

### Budget Alerts

Два порога на `budget_max_usd`:
- `budget_warn_pct` (80%) — предупреждение в лог
- 100% — emergency gate (если gates включены) или пропуск задач

При `budget_max_usd: 0` — контроль бюджета отключён.

### Stuck Detection

Счётчик `consecutiveNoCommit` в execute loop. При `stuck_threshold` итераций без нового коммита — feedback injection в следующую попытку. Сбрасывается при обнаружении нового коммита.

### Latency & Error Categorization

`LatencyBreakdown` — тайминг фаз: session, git, gate, review, distill.
`ErrorStats` — классификация ошибок: timeout, parse, git, session, config, unknown.
Записываются инкрементально после каждого измерения.

## Plan Package (v2) — Критические правила

### Архитектура plan/

```
plan/plan.go      — Run(ctx, cfg, PlanOpts); writeAtomic(); единственная точка записи
plan/merge.go     — MergeInto(existing, generated []byte) — pure fn, без I/O
plan/size.go      — CheckSize(inputs []PlanInput) — pure fn, без I/O
plan/prompts/     — plan.md (generator), review.md (reviewer) — text/template файлы
```

### Типы

```go
type PlanInput struct {
    File    string // путь к файлу (читает cmd/ralph/plan.go, НЕ plan/)
    Role    string // семантическая роль: "requirements", "architecture", etc.
    Content []byte // содержимое файла — заполняется до вызова plan.Run()
}

type PlanOpts struct {
    Inputs     []PlanInput
    OutputPath string // default: "docs/sprint-tasks.md"
    Merge      bool
    MaxRetries int    // default: 1
}

// В session/session.go — добавить в v2:
// InjectFeedback string — reviewer feedback для retry сессии (пустая = no retry)
```

### Config поля (добавляются в v2)

```go
PlanInputs     []PlanInputConfig `yaml:"plan_inputs"`       // список входных документов
PlanOutputPath string            `yaml:"plan_output_path"`  // default: "docs/sprint-tasks.md"
PlanMaxRetries int               `yaml:"plan_max_retries"`  // default: 1
PlanMerge      bool              `yaml:"plan_merge"`         // default: false
```

### CLI формат

```bash
ralph plan --input "docs/prd.md:requirements" --input "docs/arch.md:architecture"
# Формат: "filepath:role" — cobra.StringArrayVar — парсится в cmd/ralph/plan.go
# CLI значения переопределяют config.PlanInputs
```

### Инварианты (ОБЯЗАТЕЛЬНО для plan/)

1. **Файлы читает только `cmd/ralph/plan.go`** — внутри `plan/` нет `os.ReadFile`
2. **Запись только через `writeAtomic()`** — нет `os.WriteFile` в merge.go / size.go
3. **User content через `strings.Replace`** — не через `template.Execute` (injection guard)
4. **Максимум 3 сессии**: generate → review → retry (если gate `e`)
5. **`os.Exit` запрещён в plan/** — только `return error`
6. **`plan/` не импортирует** `bridge`, `runner`, `gates`, `cmd/ralph`

### Review Flow

```
generate (сессия 1)
  → review в чистой сессии (сессия 2)
    → gate: "p" → writeAtomic → return nil
    → gate: "q" → return error
    → gate: "e" → retry с InjectFeedback (сессия 3)
      → review (сессия 4 — НЕТ, max reached)
      → return error "max retries exceeded"
```

### Size Warning

```go
const MaxInputBytes = 100_000 // ~100KB суммарно по всем входным документам
// CheckSize возвращает warn=true + сообщение, не блокирует выполнение
```

### TemplateData struct (для prompts)

```go
type templateData struct {
    Inputs     []PlanInput
    OutputPath string
    Merge      bool
    Existing   string // текущий sprint-tasks.md (для merge prompt)
}
// User content (содержимое файлов) вставляется через strings.Replace ПОСЛЕ template.Execute
```

### Golden Files

`plan/testdata/TestPlanPrompt_Generate.golden` и `TestPlanPrompt_Review.golden` — полный текст промпта (аналог `bridge/testdata/`). Флаг `-update` регенерирует.

### Bridge Removal Order

1. `plan/` полностью покрыт тестами (≥80%), CI зелёный
2. **Отдельный коммит**: удалить `bridge/`, `cmd/ralph/bridge.go`, регистрацию в `root.go`
3. `go build ./...` + полный test suite — 0 broken

## Runner Split Boundary

В MVP `runner/` = loop + git + scan + knowledge + metrics (~1200 LOC). Текущее разделение: `runner.go` (основной цикл), `metrics.go`, `similarity.go`, `git.go`, `scan.go`, `prompt.go`, `log.go`, `knowledge*.go`, `serena.go`.
