# Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**7 категорий, 56+ паттернов** — области, где AI-агенты могли бы принять разные решения при реализации bmad-ralph.

### Naming Patterns

| Паттерн | Стандарт | Пример |
|---------|---------|--------|
| **Exported/unexported** | Go standard: `PascalCase` / `camelCase` | `RunLoop` (exported), `scanTasks` (unexported) |
| **Interfaces** | Noun/verb, в пакете-потребителе | `GitClient` в `runner/`, не в отдельном `git/` package |
| **Mocks** | `Mock` prefix | `MockGitClient`, `MockSessionRunner` |
| **Sentinel errors** | `Err` prefix, package scope | `var ErrNoTasks = errors.New("no tasks")` |
| **Error wrapping** | `package: operation: %w` | `fmt.Errorf("runner: execute task %s: %w", id, err)` |
| **Test functions** | `Test<Type>_<Method>_<Scenario>` | `TestRunner_Execute_WithFindings` |
| **Test files** | Co-located, `_test.go` | `runner_test.go` рядом с `runner.go` |
| **Testdata** | `testdata/` в каждом package | `runner/testdata/TestExecute.golden` |
| **Packages** | Lowercase, short, no underscores | `runner`, `gates`, `config` |

### Structural Patterns

| Паттерн | Стандарт |
|---------|---------|
| **Package entry point** | Каждый package имеет одну главную exported функцию: `bridge.Run(ctx, cfg)`, `runner.Run(ctx, cfg)`, `gates.Prompt(ctx, gate)`, `session.Execute(ctx, opts)`. Минимальная exported API surface — не экспортировать внутренние helpers |
| **Dependency direction** | Строго сверху вниз. `cmd/ralph` → `runner` → `session`, `gates`, `config`. `bridge` → `session`, `config`. `session` и `gates` НЕ зависят друг от друга. `config` — leaf package (не зависит ни от кого). Циклические зависимости запрещены |
| **Config immutability** | `config.Config` парсится один раз при старте и дальше read-only. Передаётся в packages by pointer, не мутируется в runtime. Никаких "подкрутим параметр для сложной задачи" |
| **String constants для маркеров** | Все маркеры sprint-tasks.md как `const` в config package: `TaskOpen = "- [ ]"`, `TaskDone = "- [x]"`, `GateTag = "[GATE]"`, `FeedbackPrefix = "> USER FEEDBACK:"`. Regex patterns для scan тоже как `var` с `regexp.MustCompile` в package scope |

**Dependency diagram:**
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

**Anti-patterns:**
- `gates` импортирует `runner` — circular dependency
- `session` импортирует `config` напрямую для чтения файлов — session принимает options struct, config заполняет
- Package экспортирует 10+ функций — скорее всего, нарушение single responsibility

### Error Handling Patterns

| Паттерн | Стандарт |
|---------|---------|
| **Wrapping** | `fmt.Errorf("package: operation: %w", err)` — всегда package prefix + контекст |
| **Custom error types** | Только для branch logic: `*ExitCodeError` (Claude exit code), `*GateDecision` (user quit/skip) |
| **Sentinel errors** | `ErrNoTasks`, `ErrDirtyTree`, `ErrMaxRetries` — control flow в runner |
| **Type checking** | `errors.Is` / `errors.As` always. Никогда string matching |
| **Panic** | Никогда в runtime. Только `init()` для программных ошибок (missing embed) |
| **Exit code mapping** | Только в `cmd/ralph/` — единственное место error → exit code 0-4. Packages не знают про exit codes |

**Anti-patterns:**
- `if err.Error() == "not found"` — используй `errors.Is(err, ErrNotFound)`
- `os.Exit(1)` в package — возвращай error, пусть cmd решает
- `panic("unexpected")` в runtime — возвращай error

### File I/O Patterns

| Паттерн | Стандарт |
|---------|---------|
| **Encoding** | UTF-8, no BOM |
| **Line endings** | `\n` (Unix). git autocrlf handles cross-platform |
| **Read** | `os.ReadFile` целиком (файлы маленькие) |
| **Write** | `os.WriteFile` с `0644`. Не atomic — git = backup |
| **Scan** | `strings.Split(content, "\n")` + regex match. Без bufio.Scanner |
| **Paths** | Относительно `config.ProjectRoot`. Определяется один раз при старте |
| **File existence** | `os.Stat` + `errors.Is(err, os.ErrNotExist)`. Отсутствующий review-findings.md = пуст |

### Subprocess Patterns

| Паттерн | Стандарт |
|---------|---------|
| **Context creation** | `main()` создаёт root ctx через `signal.NotifyContext(context.Background(), os.Interrupt)`. Прокидывает в runner/bridge через параметры функций. Никаких global context, никаких `context.TODO()` в prod коде |
| **Context propagation** | Все subprocess через `exec.CommandContext(ctx, ...)`. ctx несёт cancellation от Ctrl+C |
| **Git timeout** | Без wall-clock timeout. Git — быстрый. Cancellation только через ctx |
| **Claude timeout** | Без wall-clock timeout. `--max-turns` как логический лимит. Cancellation через ctx |
| **stdout/stderr** | Git: `cmd.Output()`. Claude: разделённый capture (JSON stdout + stderr для warnings) |
| **Exit code** | `exec.ExitError` → извлечение exit code. Wrap: `"session: claude: exit %d: %w"` |
| **Working dir** | `cmd.Dir = config.ProjectRoot` — всегда явно |
| **Environment** | `os.Environ()` — наследуем. Не добавляем переменных |

### Logging & Output Patterns

| Паттерн | Стандарт |
|---------|---------|
| **stdout** | Только user-facing: статусы, gates, warnings. Через `fatih/color` |
| **stderr** | Не используем для вывода. Ralph пишет только в stdout и log file |
| **Log file** | `fmt.Fprintf(logFile, "%s %s %s\n", timestamp, level, msg)`. Append-only |
| **Timestamp** | `2006-01-02T15:04:05` (Go reference time, ISO 8601) |
| **Log levels** | `INFO`, `WARN`, `ERROR`. Без DEBUG в MVP |
| **Что логируется** | Subprocess вызовы (cmd, exit code, duration), gate решения, findings summary, retries, file writes |
| **Что НЕ логируется** | Claude stdout целиком. Только session_id, exit code, duration, наличие коммита |
| **Log path** | `.ralph/logs/run-2006-01-02-150405.log` — один файл на `ralph run` |
| **Architecture** | Packages не логируют — возвращают results/errors. `cmd/ralph/` решает что в stdout, что в log |

### Testing Patterns

| Паттерн | Стандарт |
|---------|---------|
| **Table-driven** | По умолчанию для >2 cases. `[]struct{name; ...}` + `t.Run` |
| **Individual** | Только для сложных integration-like сценариев |
| **Golden files** | `testdata/TestName.golden`. Update: `go test -update` |
| **Golden file workflow** | (1) `-update` обновляет `.golden` файлы, (2) CI НЕ имеет `-update` — mismatch = fail, (3) после обновления промпта: `go test -update ./... && git diff` для review изменений |
| **Scenario-based mock** | Mock Claude получает scenario file (JSON) с sequence of responses: `[{input_match, output, exit_code}, ...]`. Полные integration тесты runner через scenario sequence |
| **Shared helpers** | `internal/testutil/` (mock Claude, mock git) |
| **Mock Claude** | Go script, scenario-based. Подставляется через `config.ClaudeCommand` |
| **Mock Git** | `MockGitClient` implements `GitClient` interface |
| **Assertions** | Go stdlib `if got != want { t.Errorf }`. Без testify — zero test deps |
| **Isolation** | `t.TempDir()` для каждого теста. Без shared state |
| **Coverage target** | `runner` и `config` — critical path, >80%. `cmd/ralph/` — minimal. `gates` — acceptance через scenario mock |

### Enforcement Guidelines

**Все AI-агенты MUST:**
1. Следовать naming conventions. `golangci-lint` ловит часть нарушений автоматически
2. Возвращать errors, не логировать из packages. Только `cmd/ralph/` управляет output
3. Использовать `exec.CommandContext(ctx)` для всех subprocess. Без `exec.Command`
4. Писать table-driven тесты с `t.Run`. Golden files для промптов и bridge output
5. Определять interfaces в пакете-потребителе
6. Соблюдать dependency direction (config = leaf, нет циклов)
7. Не мутировать config в runtime
8. Использовать string constants для маркеров sprint-tasks.md

**Pattern Enforcement:**
- `golangci-lint` — автоматическая проверка naming и error handling
- Golden file / snapshot тесты — ловят drift в промптах и bridge output
- Code review (review agents) — проверка паттернов, которые линтер не ловит
- **project-context.md** — ключевые паттерны (naming, error handling, dependency direction, config immutability) компилируются в project-context.md после завершения архитектуры. Этот файл загружается в CLAUDE.md и виден каждому AI-агенту при реализации
