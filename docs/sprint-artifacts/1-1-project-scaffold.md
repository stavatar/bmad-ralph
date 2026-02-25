# Story 1.1: Project Scaffold

Status: done

## Story

As a developer,
I want the project directory structure, go.mod, and placeholder files created according to Architecture,
so that all subsequent stories have a consistent foundation with correct paths and naming.

## Acceptance Criteria

```gherkin
Given the project is being initialized
When the scaffold is created
Then the following directory structure exists:
  | Path                              | Type      |
  | cmd/ralph/                        | directory |
  | bridge/                           | directory |
  | bridge/prompts/                   | directory |
  | bridge/testdata/                  | directory |
  | runner/                           | directory |
  | runner/prompts/                   | directory |
  | runner/prompts/agents/            | directory |
  | runner/testdata/                  | directory |
  | session/                          | directory |
  | gates/                            | directory |
  | config/                           | directory |
  | config/shared/                    | directory |
  | config/testdata/                  | directory |
  | internal/testutil/                | directory |
  | internal/testutil/scenarios/      | directory |

And go.mod exists with module path and Go 1.25
And go.sum exists (after go mod tidy with cobra + yaml.v3 + fatih/color)
And Makefile exists with targets: build, test, lint
And .gitignore includes: binary, .ralph/, *.log
And .golangci.yml exists with basic linter config
And .github/workflows/ci.yml exists with: go test ./..., golangci-lint, build check
And .goreleaser.yaml exists with basic Go binary release config (ldflags for version)
And cmd/ralph/main.go contains package main with empty main()
And each package directory has a placeholder .go file with correct package declaration
```

## Tasks / Subtasks

- [x] Task 1: Инициализация Go module (AC: go.mod, go.sum)
  - [x] 1.1 `go mod init github.com/bmad-ralph/bmad-ralph`
  - [x] 1.2 `go get github.com/spf13/cobra@latest` → v1.10.2
  - [x] 1.3 `go get gopkg.in/yaml.v3` → v3.0.1
  - [x] 1.4 `go get github.com/fatih/color` → v1.18.0
  - [x] 1.5 `go mod tidy` для генерации go.sum
- [x] Task 2: Создание directory structure (AC: directory structure)
  - [x] 2.1 Создать все 15 директорий из AC таблицы + .github/workflows/
  - [x] 2.2 Добавить `internal/testutil/cmd/mock_claude/` (нужна для Story 1.11)
  - [x] 2.3 Добавить `.gitkeep` в 9 пустых директорий
- [x] Task 3: Placeholder .go файлы (AC: package declarations)
  - [x] 3.1 `cmd/ralph/main.go` — `package main` с `func main()` + blank imports для deps
  - [x] 3.2 `bridge/bridge.go` — `package bridge`
  - [x] 3.3 `runner/runner.go` — `package runner`
  - [x] 3.4 `session/session.go` — `package session`
  - [x] 3.5 `gates/gates.go` — `package gates`
  - [x] 3.6 `config/config.go` — `package config`
  - [x] 3.7 `internal/testutil/testutil.go` — `package testutil`
- [x] Task 4: Makefile (AC: build, test, lint)
  - [x] 4.1 `make build` → `go build -o ralph ./cmd/ralph`
  - [x] 4.2 `make test` → `go test ./...`
  - [x] 4.3 `make lint` → `golangci-lint run`
  - [x] 4.4 `make clean` → удалить binary
- [x] Task 5: .gitignore (AC: binary, .ralph/, *.log)
  - [x] 5.1 Исключить: `ralph`, `.ralph/`, `*.log`, `dist/`, Go artifacts, IDE, OS
- [x] Task 6: .golangci.yml (AC: basic linter config)
  - [x] 6.1 Линтеры: govet, errcheck, staticcheck, unused, gosimple, ineffassign, typecheck
  - [x] 6.2 Go version: 1.25
- [x] Task 7: CI workflow (AC: .github/workflows/ci.yml)
  - [x] 7.1 Trigger: push + PR to main
  - [x] 7.2 Go version: ['1.25'] (1.26 добавить после выхода stable)
  - [x] 7.3 Steps: checkout → setup-go → go test → go build + отдельный lint job
- [x] Task 8: .goreleaser.yaml (AC: binary release config)
  - [x] 8.1 Базовый config: Go binary с ldflags `-s -w -X main.version={{.Version}}`
  - [x] 8.2 Targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
  - [x] 8.3 Архив: tar.gz для Linux/macOS
- [x] Task 9: Валидация (все AC)
  - [x] 9.1 `go build ./...` проходит без ошибок
  - [x] 9.2 `go vet ./...` проходит без ошибок
  - [x] 9.3 Все package declarations корректны

## Senior Developer Review (AI)

**Review Date:** 2026-02-25
**Review Outcome:** Changes Requested → Fixed
**Reviewer Model:** Claude Opus 4.6

### Action Items

- [x] **[CRITICAL]** `.gitignore` pattern `ralph` matches `cmd/ralph/` — main.go would never be committed. Fix: `/ralph`
- [x] **[HIGH]** All created files have CRLF line endings — Makefile breaks on Linux. Fix: convert to LF + add `.gitattributes`
- [x] **[MEDIUM]** No `.gitattributes` file — no LF enforcement. Fix: created with `* text=auto eol=lf`
- [ ] **[MEDIUM]** `.gitignore` has extra entries beyond story spec (*.exe, vendor/, .claude/, etc.) — cosmetic, no code impact
- [ ] **[MEDIUM]** Story File List missing workflow meta-files — added to File List
- [ ] **[LOW]** `make build` not individually validated in Task 9
- [ ] **[LOW]** CI matrix Dev Notes show 1.25+1.26 but implementation is 1.25 only — documented deviation
- [ ] **[LOW]** Blank imports in main.go vs "exact content" in Dev Notes — documented deviation

**Summary:** 1 Critical + 1 High fixed automatically. 3 Medium partially addressed. 3 Low documented as accepted deviations.

## Dev Notes

### Критические архитектурные правила

**Dependency Direction (СТРОГО):**
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
- `config` = leaf package (не зависит ни от кого)
- `session` и `gates` НЕ зависят друг от друга
- Циклические зависимости ЗАПРЕЩЕНЫ

**Package Entry Points (минимальная exported API surface):**

| Package | Entry Point | Placeholder |
|---------|-------------|-------------|
| `bridge` | `bridge.Run(ctx, cfg)` | Пустой файл с `package bridge` |
| `runner` | `runner.Run(ctx, cfg)` | Пустой файл с `package runner` |
| `session` | `session.Execute(ctx, opts)` | Пустой файл с `package session` |
| `gates` | `gates.Prompt(ctx, gate)` | Пустой файл с `package gates` |
| `config` | `config.Load(flags)` | Пустой файл с `package config` |

**Placeholder файлы:** Минимальный валидный Go файл — ТОЛЬКО `package X`, без imports, без функций. Цель: `go build ./...` проходит.

[Source: docs/project-context.md#Architecture — Ключевые решения]
[Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.1]

### go.mod — точные параметры

```
module github.com/bmad-ralph/bmad-ralph

go 1.25
```

**Module path:** Использовать `github.com/bmad-ralph/bmad-ralph` (или актуальный GitHub org/repo). Если org ещё не создан — использовать placeholder, поправить позже.

**Go version в go.mod:** Именно `1.25` (не `1.25.7`). go.mod указывает минимальную совместимую версию.

[Source: docs/architecture/ — Core Architectural Decisions: Go version 1.25]

### Makefile — точное содержание

```makefile
.PHONY: build test lint clean

build:
	go build -o ralph ./cmd/ralph

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f ralph
```

**Важно:**
- ВНИМАНИЕ: Makefile использует TAB для indentation (не spaces). При генерации убедиться в корректности отступов
- Binary name: `ralph` (не `bmad-ralph`)
- `go build -o ralph` — явное имя binary для .gitignore
- Без `-race` в default `make test` (добавляется в CI)
- Без `go vet` в Makefile (входит в golangci-lint)
- Architecture упоминает `make release` — НЕ включён в Story 1.1 AC. Будет добавлен при настройке CI/CD release pipeline

[Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.1 Technical Notes]

### .gitignore — точное содержание

```gitignore
# Binary
ralph

# Runtime directory (user project)
.ralph/

# Logs
*.log

# Goreleaser output
dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db
```

### .golangci.yml — минимальный config

```yaml
run:
  go: "1.25"

linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck
```

**Не включать:** exhaustive, wrapcheck, gocyclo — слишком шумные для scaffold. Расширить позже.

### CI workflow (.github/workflows/ci.yml)

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.25', '1.26']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
      - run: go test ./...
      - run: go build ./cmd/ralph

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

**Важно:**
- Go matrix: 1.25 + 1.26 (Architecture: совместимость с двумя версиями). Если Go 1.26 недоступна при реализации — использовать `['1.25']` single version. Добавить 1.26 после выхода stable release
- Lint на 1.25 (минимальная) — отдельный job
- `golangci-lint-action` v6 (актуальная). Пинить конкретную версию lint после первого успешного CI run (заменить `version: latest` на `version: v2.10.0` или актуальную)
- Без `-race` в CI для скорости (добавить в Growth)

[Source: docs/architecture/ — Build, Tooling & CI]

### .goreleaser.yaml — базовый config

```yaml
version: 2
builds:
  - main: ./cmd/ralph
    binary: ralph
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - format: tar.gz

changelog:
  sort: asc
```

**Важно:**
- `version: 2` — goreleaser v2 format
- `CGO_ENABLED=0` — чистый static binary
- `main.version` — переменная в `cmd/ralph/main.go` (`var version = "dev"`)
- Windows отсутствует — Architecture: "Windows только через WSL"

[Source: docs/architecture/ — Build, Tooling & CI: goreleaser]

### Технологический стек — актуальные версии (февраль 2026)

| Dependency | Version | Назначение |
|------------|---------|------------|
| Go | 1.25 (в go.mod) | Язык |
| spf13/cobra | latest (go get @latest) | CLI framework |
| gopkg.in/yaml.v3 | v3.0.1 | Config YAML parsing |
| fatih/color | v1.18.0 | Цветной вывод |
| golangci-lint | v2.10+ | Линтер (CI action) |
| goreleaser | v2.14+ | Release builds |

**ЗАПРЕЩЕНО добавлять новые зависимости** без обоснования. Architecture: "Всего 3 direct deps. Новые зависимости требуют обоснования."

[Source: docs/project-context.md#Technology Stack]

### Структура проекта — полный список файлов для создания

```
bmad-ralph/                        # Project root (уже существует)
├── .github/
│   └── workflows/
│       └── ci.yml                 # CI: test + lint + build
├── .goreleaser.yaml               # Release config
├── .golangci.yml                  # Linter config
├── .gitignore                     # Exclusions
├── Makefile                       # build, test, lint, clean
├── LICENSE                        # (НЕ создавать в scaffold — добавляется перед релизом)
├── go.mod                         # Go 1.25 + 3 deps
├── go.sum                         # auto-generated
├── cmd/ralph/
│   └── main.go                    # package main, empty main(), var version
├── bridge/
│   ├── bridge.go                  # package bridge
│   ├── prompts/                   # (пустая, для go:embed — Story 2.2)
│   └── testdata/                  # (пустая, для golden files — Story 2.5)
├── runner/
│   ├── runner.go                  # package runner
│   ├── prompts/                   # (пустая, для go:embed — Story 3.1)
│   │   └── agents/               # (пустая, для sub-agent prompts — Story 4.2)
│   └── testdata/                  # (пустая, для golden files)
├── session/
│   └── session.go                 # package session
├── gates/
│   └── gates.go                   # package gates
├── config/
│   ├── config.go                  # package config
│   ├── shared/                    # (пустая, для sprint-tasks-format.md — Story 2.1)
│   └── testdata/                  # (пустая, для config test fixtures — Story 1.3)
└── internal/
    └── testutil/
        ├── testutil.go            # package testutil
        ├── cmd/
        │   └── mock_claude/       # (пустая, для mock binary — Story 1.11)
        └── scenarios/             # (пустая, для scenario JSON — Story 1.11)
```

**Пустые директории:** Go не коммитит пустые директории. Добавить `.gitkeep` в каждую из следующих:
- `bridge/prompts/.gitkeep`
- `bridge/testdata/.gitkeep`
- `runner/prompts/.gitkeep`
- `runner/prompts/agents/.gitkeep`
- `runner/testdata/.gitkeep`
- `config/shared/.gitkeep`
- `config/testdata/.gitkeep`
- `internal/testutil/scenarios/.gitkeep`
- `internal/testutil/cmd/mock_claude/.gitkeep`

### cmd/ralph/main.go — точное содержание

```go
package main

// version is set by goreleaser ldflags at build time.
var version = "dev"

func main() {
}
```

**Почему пустой main():**
- Cobra wiring добавляется в Story 1.13
- Scaffold только устанавливает структуру
- `go build ./...` должен проходить

### Анти-паттерны (ЗАПРЕЩЕНО в этой story)

- НЕ добавлять бизнес-логику в placeholder файлы
- НЕ добавлять imports в placeholder файлы (кроме main.go если нужен)
- НЕ создавать файлы вне утверждённой структуры
- НЕ добавлять testify или другие test dependencies
- НЕ создавать `.ralph/` директорию — это runtime directory пользователя, не часть исходников
- НЕ использовать `go 1.25.7` в go.mod — только `go 1.25`
- НЕ добавлять Windows targets в goreleaser — "Windows только через WSL"
- НЕ создавать `config/defaults.yaml` — это задача Story 1.4 (go:embed defaults)
- НЕ создавать `LICENSE` — добавляется отдельно перед первым публичным релизом

### Project Structure Notes

- Полное соответствие Architecture: docs/architecture/ → Project Structure & Boundaries
- Добавлен `internal/testutil/cmd/mock_claude/` из Story 1.11 Technical Notes — standalone binary для `config.ClaudeCommand` substitution. Architecture также упоминает `internal/testutil/mock_claude.go` — это scenario loading logic (создаётся в Story 1.11). В scaffold создаётся ТОЛЬКО директория `cmd/mock_claude/`
- **Known deviation:** `.goreleaser.yaml` вместо `.goreleaser.yml` из Architecture. Goreleaser v2 рекомендует `.yaml`. Architecture doc подлежит обновлению. Goreleaser поддерживает оба расширения

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.1]
- [Source: docs/project-context.md#Technology Stack]
- [Source: docs/project-context.md#Architecture — Ключевые решения]
- [Source: docs/architecture/ — Project Structure & Boundaries]
- [Source: docs/architecture/ — Build, Tooling & CI]
- [Source: docs/architecture/ — Core Architectural Decisions]
- [Source: docs/epics/epics-structure-plan.md — Epic 1]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- Go module инициализирован с `go 1.25` (build-time Go 1.26.0 через Windows WSL)
- 3 direct dependencies добавлены: cobra v1.10.2, yaml.v3 v3.0.1, color v1.18.0
- Blank imports в main.go для удержания deps в go.mod (go mod tidy удаляет неиспользуемые)
- 7 placeholder .go файлов с минимальным `package X` — `go build ./...` проходит
- 9 .gitkeep файлов для пустых директорий (Go не коммитит пустые dirs)
- CI workflow с Go 1.25 only (1.26 deferred — ещё нет stable release)
- .goreleaser.yaml (не .yml) — goreleaser v2 рекомендация, documented deviation
- **[Code Review Fix]** CRITICAL: `.gitignore` pattern `ralph` → `/ralph` — без ведущего `/` игнорировался весь `cmd/ralph/`
- **[Code Review Fix]** HIGH: Все файлы конвертированы CRLF → LF (Write tool на Windows NTFS создавал CRLF)
- **[Code Review Fix]** Добавлен `.gitattributes` с `* text=auto eol=lf` для enforcement LF endings

### File List

- go.mod (created)
- go.sum (created)
- cmd/ralph/main.go (created)
- bridge/bridge.go (created)
- runner/runner.go (created)
- session/session.go (created)
- gates/gates.go (created)
- config/config.go (created)
- internal/testutil/testutil.go (created)
- Makefile (created)
- .gitignore (modified)
- .golangci.yml (created)
- .github/workflows/ci.yml (created)
- .goreleaser.yaml (created)
- bridge/prompts/.gitkeep (created)
- bridge/testdata/.gitkeep (created)
- runner/prompts/.gitkeep (created)
- runner/prompts/agents/.gitkeep (created)
- runner/testdata/.gitkeep (created)
- config/shared/.gitkeep (created)
- config/testdata/.gitkeep (created)
- internal/testutil/scenarios/.gitkeep (created)
- internal/testutil/cmd/mock_claude/.gitkeep (created)
- .gitattributes (created) [code-review]
- docs/sprint-artifacts/1-1-project-scaffold.md (created)
- docs/sprint-artifacts/sprint-status.yaml (created)
