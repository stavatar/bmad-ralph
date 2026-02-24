---
project_name: 'bmad-ralph'
user_name: 'Степан'
date: '2026-02-24'
source: 'docs/architecture.md'
status: 'complete'
optimized_for_llm: true
---

# Project Context for AI Agents

_Критические правила bmad-ralph. Только неочевидное — стандартные Go conventions не дублируются._

---

## Technology Stack

- **Go 1.25** в go.mod (совместимость с 1.25 + 1.26)
- **Cobra** — CLI framework (subcommands `bridge`, `run`)
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
└── bridge
    ├── session
    └── config
```

- `config` = leaf package, не зависит ни от кого
- `session` и `gates` НЕ зависят друг от друга
- `cmd/ralph/` — единственное место для exit code mapping (0-4) и output decisions

### Package Entry Points (минимальная exported API surface)

| Package | Entry Point |
|---------|-------------|
| `bridge` | `bridge.Run(ctx, cfg)` |
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

## Runner Split Boundary

В MVP `runner/` = loop + git + scan + knowledge (~600-800 LOC). Когда >1000 LOC → выделять `review`, `knowledge`, `state`. **НЕ split'ить в MVP.**
