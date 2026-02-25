# Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Git interaction: subprocess via `os/exec` через `GitClient` interface
- Claude CLI output: `--output-format json` для structured parsing
- Prompt assembly: `text/template` + двухэтапная сборка (template → user content injection)
- Go version: 1.25 в `go.mod`

**Important Decisions (Shape Architecture):**
- CLI output: `fatih/color` для цветного вывода
- Progress: статичные строки + spinner при интерактивном терминале (isatty)
- Markdown parsing: line scanning + regex (без AST)
- Линтинг: `golangci-lint`
- CI: GitHub Actions
- Distribution: goreleaser с первого релиза

**Deferred Decisions (Post-MVP):**
- Structured log format (Growth — сейчас простой text log)
- Session adapter для multi-LLM (Growth)
- Smoke test с real Claude CLI для JSON format validation (Growth — первый smoke test)
- TUI dashboard с метриками (Vision)

### Subprocess & Git

| Решение | Выбор | Обоснование |
|---------|-------|-------------|
| **Git interaction** | `git` subprocess (`os/exec`) через `GitClient` interface | git уже hard dep, Ralph использует только базовые команды. go-git — overkill (+15MB). Interface (`GitClient` → `ExecGitClient` prod, `MockGitClient` test) обеспечивает тестируемость |
| **Claude CLI output** | `--output-format json` | Надёжный capture session ID для `--resume`, structured exit info. Golden file тесты на JSON response — контракт с Claude CLI |
| **Claude CLI invocation** | `os/exec.Command` через session package | Единая точка вызова. Абстракция изолирует от изменений Claude CLI формата. Мониторинг Claude CLI changelog — операционная задача |

### File I/O & Prompt Assembly

| Решение | Выбор | Обоснование |
|---------|-------|-------------|
| **Prompt assembly** | `text/template` (Go stdlib) + двухэтапная сборка | Этап 1: `text/template` для структуры промпта (`{{if .SerenaEnabled}}`). Этап 2: `strings.Replace` для user content injection (LEARNINGS.md, CLAUDE.md, review-findings.md). Двухэтапность защищает от `{{` в user-контролируемых файлах |
| **Markdown parsing** | Line scanning + regex | Ralph парсит только `- [ ]`, `- [x]`, `[GATE]`. Goldmark/AST — лишняя сложность |
| **File writes** | Простой `os.WriteFile` | Atomic writes не нужны — git = backup. Crash recovery через `git checkout` |
| **Prompt файлы** | `.md` файлы — Go templates, не чистый Markdown | Файлы в `*/prompts/` содержат `{{.Var}}` синтаксис. Не предназначены для GitHub preview — это рабочие файлы `text/template` |

### CLI UX & Output

| Решение | Выбор | Обоснование |
|---------|-------|-------------|
| **Цветной вывод** | `fatih/color` | Простой API, auto-detect terminal (`color.NoColor` в pipe/CI). lipgloss — overkill для однострочных статусов |
| **Progress indication** | Статичные строки + spinner при isatty | Статичные строки работают в pipe/CI. Spinner через `\r` только в интерактивном терминале — явный `mattn/go-isatty` check (транзитивный dep от `fatih/color`, не новый import) |
| **Human gate prompts** | `fmt.Scan` + `fatih/color` | Простой интерактивный ввод, цветные опции |

### Build, Tooling & CI

| Решение | Выбор | Версия | Обоснование |
|---------|-------|--------|-------------|
| **Go version** | 1.25 в `go.mod` | 1.25.x | Совместимость с обоими поддерживаемыми версиями (1.25, 1.26). slog доступен с 1.21 |
| **Линтинг** | `golangci-lint` | latest | Industry standard, `.golangci.yml` в репозитории, `make lint` |
| **CI** | GitHub Actions | — | Бесплатно для public repos. Workflow: test + lint + build на push/PR |
| **Distribution** | goreleaser (MVP) | latest | Кросс-компиляция, GitHub Releases. Критично для onboarding — binary без Go toolchain |

### External Dependencies (Final)

| Dependency | Назначение | Обоснование |
|------------|------------|-------------|
| `github.com/spf13/cobra` | CLI framework | Subcommands, flags, auto help, shell completion |
| `gopkg.in/yaml.v3` | Config parsing | YAML config, без Viper |
| `github.com/fatih/color` | Цветной вывод | Статусы, warnings, human gate prompts |

**Всего 3 direct dependencies.** `mattn/go-isatty` — транзитивная (от `fatih/color`), используется напрямую для spinner isatty check. `text/template`, `os/exec`, `regexp` — stdlib.

### Testing Implications of Decisions

| Решение | Тестовый подход |
|---------|----------------|
| Git subprocess + `GitClient` interface | `MockGitClient` в `internal/testutil/` — предопределённые ответы по сценарию |
| Claude CLI JSON output | Golden file с example JSON response. Парсинг session_id, result, exit_code. Обновление golden file при изменении Claude CLI |
| `text/template` промпты | Snapshot/golden file тесты: собранный промпт = baseline. `go test -update` для обновления |
| Spinner при isatty | Не тестируется (визуальный feedback). Логика isatty check тривиальная |
| Real Claude CLI smoke test | Growth: integration test с реальным `claude` CLI для валидации JSON формата |

### Decision Impact Analysis

**Implementation Sequence:**
1. `config` — YAML parsing + defaults cascade (фундамент для всего)
2. `session` — Claude CLI invocation с `--output-format json` + `--resume` (зависит от config)
3. `gates` — interactive stdin prompts с `fatih/color`
4. `bridge` — story → sprint-tasks.md (через session)
5. `runner` — основной loop (объединяет session + gates + config)
6. `cmd/ralph` — Cobra wiring

**Cross-Component Dependencies:**
- `session` зависит от `config` (claude_command, max_turns, model)
- `runner` зависит от `session`, `gates`, `config`
- `bridge` зависит от `session`, `config`
- `cmd/ralph` зависит от всех packages (wiring only)
- `text/template` используется в `runner` и `bridge` для prompt assembly
- `GitClient` interface определён в `runner`, реализация в `runner` (prod) и `internal/testutil` (mock)
