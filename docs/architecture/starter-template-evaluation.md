# Starter Template Evaluation

### Primary Technology Domain

**Go CLI tool** — 2 команды (`bridge`, `run`), ~10 CLI-флагов, YAML config, subprocess management.

### Starter Options Considered

| Вариант | Оценка | Обоснование |
|---------|--------|-------------|
| **Cobra** (spf13/cobra) | **Выбран** | Industry standard (37K+ stars, Dec 2025). Используется gh CLI, Kubernetes, Docker, Hugo. Subcommands, auto help, shell completion, тестируемость из коробки |
| urfave/cli | Отклонён | Проще, но меньше экосистема. Нет преимущества перед Cobra для нашего случая |
| Kong | Отклонён | Struct-based, но маленькое community (2K stars). Риск для open-source проекта |
| Standard `flag` | Отклонён | Zero deps, но ручной routing, ручной help, нет shell completion. Не масштабируется на Growth (новые команды) |

### Selected Starter: Cobra + yaml.v3

**External dependencies (всего 2):**
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML config parsing

**Viper НЕ используется** — свой каскад config (CLI > config file > go:embed defaults) через yaml.v3. Viper — лишний dep.

**Initialization Command:**
```bash
go mod init github.com/user/bmad-ralph
go get github.com/spf13/cobra@latest
go get gopkg.in/yaml.v3
```

### Project Structure

> **Примечание:** Каноническая структура проекта — в секции "Project Structure & Boundaries" (Step 6). Дерево ниже — краткая версия для контекста Starter Evaluation.

```
bmad-ralph/
├── cmd/ralph/
│   ├── main.go              # Cobra root command setup
│   ├── bridge.go            # bridge subcommand (wiring only, логика в bridge/)
│   └── run.go               # run subcommand (wiring only, логика в runner/)
├── bridge/
│   ├── bridge.go            # Bridge logic
│   ├── bridge_test.go
│   ├── prompts/             # go:embed bridge prompts
│   │   └── bridge.md
│   └── testdata/            # Golden files: input stories → expected sprint-tasks.md
├── runner/
│   ├── runner.go            # Main loop (execute→review)
│   ├── runner_test.go
│   └── prompts/             # go:embed execute/review prompts
│       ├── execute.md
│       ├── review.md
│       ├── distillation.md
│       └── agents/
│           ├── quality.md
│           ├── implementation.md
│           ├── simplification.md
│           └── test-coverage.md
├── session/
│   ├── session.go           # Claude CLI invocation (os/exec)
│   └── session_test.go
├── gates/
│   ├── gates.go             # Human gate logic, interactive stdin
│   └── gates_test.go
├── config/
│   ├── config.go            # YAML + CLI flags + go:embed defaults cascade
│   ├── config_test.go
│   ├── shared/              # go:embed shared contracts
│   │   └── sprint-tasks-format.md   # Format contract (shared between bridge & execute)
│   └── testdata/            # Config parsing test fixtures
├── internal/
│   └── testutil/
│       └── mock_claude.go   # Mock Claude CLI for integration tests
├── Makefile                 # make build, make test, make lint
├── go.mod
└── go.sum
```

**Архитектурные правила структуры:**
- `cmd/ralph/*.go` — только Cobra wiring (flag definition, arg validation, вызов package). Бизнес-логика — в packages
- `go:embed` промпты в package который их использует (`runner/prompts/`, `bridge/prompts/`). Каждый package embed-ит свои файлы
- `testdata/` внутри каждого package (Go convention). `go test ./...` находит fixtures по относительному пути
- `internal/testutil/` — shared test utilities (mock Claude), доступны только внутри проекта
- `.ralph/` НЕ является частью исходников bmad-ralph. Это runtime directory в проекте пользователя
- `config/shared/sprint-tasks-format.md` — shared format contract, импортируется bridge и runner через config package

### Architectural Decisions Provided by Starter

- **CLI framework:** Cobra — subcommand routing, flag parsing, auto help, shell completion
- **Config parsing:** yaml.v3 — YAML config, без Viper (свой каскад)
- **Prompts:** go:embed per-package — каждый package embed-ит свои промпты
- **Testing:** Go built-in `testing` + mock Claude в `internal/testutil/`
- **Build:** `go build` + Makefile для contributor convenience (`make test`, `make build`, `make lint`)
- **Project layout:** `cmd/` + flat packages + `internal/` (Go best practice — shallow hierarchy)
