---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - docs/prd.md
  - docs/research/llm-as-judge-variants-research-2026-02-24.md
  - docs/research/research-1-serena-ralph-2026-02-24.md
  - docs/research/research-2-tdd-vs-prewritten-tests-2026-02-24.md
  - docs/research/research-3-e2e-in-ralph-2026-02-24.md
  - docs/research/research-4-review-agents-composition-2026-02-24.md
  - docs/research/research-5-human-gates-2026-02-24.md
  - docs/research/deep-research-ralph-review/farr-playbook.md
  - docs/research/deep-research-ralph-review/canonical-ralph.md
  - docs/research/deep-research-ralph-review/ralphex.md
  - docs/research/deep-research-ralph-review/community-consensus.md
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-02-24'
project_name: 'bmad-ralph'
user_name: 'Степан'
date: '2026-02-24'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
45 FR в 7 категориях: планирование (bridge), автономное выполнение (run), ревью кода, контроль качества (gates), управление знаниями, конфигурация, guardrails/ATDD. Архитектурно распадается на два принципиально разных компонента: `ralph bridge` (one-shot конвертер) и `ralph run` (long-running orchestrator с десятками Claude-сессий). Ключевая модель: review-сессия отвечает за подтверждение качества и отметку задачи как выполненной (`[x]`). MVP: review после каждой задачи, batch review — Growth.

**Non-Functional Requirements:**
20 NFR в 6 категориях. Ключевые для архитектуры: контекстное окно 40-50% (NFR1), single Go binary с zero runtime deps (NFR16-17), graceful shutdown через signal handling (NFR13), crash recovery через sprint-tasks.md (NFR10), dual-level logging (NFR14), промпты через go:embed + external files fallback (NFR18).

**Scale & Complexity:**
- Primary domain: CLI-утилита, процессная оркестрация
- Complexity level: средний
- Architectural components: ~10 Go packages

### Technology Decision

**Язык: Go** — single binary, zero runtime dependencies, встроенные тесты, нативный subprocess (`os/exec`), отличный markdown/YAML парсинг, signal handling для graceful shutdown. Соответствует NFR16-17 и паттерну CLI-утилит экосистемы (gh, rg, fzf).

**Config format: YAML** (`.ralph/config.yaml`) — нативный парсинг в Go (`gopkg.in/yaml.v3`), без внешних deps типа yq.

**Distribution:** `go install github.com/...` + GitHub Releases через goreleaser.

**Промпты:** defaults встроены в binary через `go:embed`, кастомные файлы в `.ralph/agents/` имеют приоритет. Fallback chain: project → global → embedded.

### Technical Constraints & Dependencies

- **Go single binary** — кросс-компиляция через `GOOS/GOARCH`
- **Hard deps для пользователя:** `git`, `claude` CLI
- **Claude Code CLI** — execution engine через `--dangerously-skip-permissions`. MVP: прямой вызов `os/exec`. При ошибке запуска — понятное сообщение. Version check и session adapter — Growth
- **Файловая система = state store.** sprint-tasks.md (прогресс задач), review-findings.md (транзиентный — findings текущей задачи), LEARNINGS.md (долгосрочные знания), CLAUDE.md (операционный контекст), .ralph/config.yaml
- **Windows только через WSL**
- **sprint-tasks.md ownership: Claude читает/пишет, review пишет `[x]`, ralph — loop control.** Execute-сессия Claude сама читает sprint-tasks.md и берёт первую `- [ ]` задачу сверху вниз (модель Playbook — self-directing). Review-сессия отмечает задачу `[x]` после подтверждения качества (clean review). Resume-extraction пишет прогресс под текущей задачей при незавершённом execute. Execute НЕ изменяет статус задач. Ralph сканирует (grep `- [ ]`) только для контроля loop (есть ли ещё задачи?) — не извлекает описание, не передаёт в промпт. Формат — open format (bridge создаёт структуру, Claude пишет свободно). Ralph парсит только `- [ ]`, `- [x]`, `[GATE]`. Защита от порчи: каждый успешный цикл коммитит в git, sprint-tasks.md восстановим через `git checkout`
- **sprint-tasks.md format contract:** Формат определяется один раз. Пример формата включается и в bridge prompt, и в execute prompt — единый source of truth через go:embed shared file. Ralph сканирует два паттерна: (1) `- [ ]` / `- [x]` для статуса задач, (2) `[GATE]` тег для определения остановочных точек human gate. Формат остального — забота Claude. Мягкая валидация при scan: если файл не содержит ни `- [ ]`, ни `- [x]` — warning "файл повреждён или пуст"
- **Exit codes** (0-4) — cross-component contract: 0=успех, 1=частичный успех (лимиты, gates off), 2=user quit (на gate), 3=Ctrl+C, 4=fatal error. Каждый компонент должен корректно возвращать и обрабатывать коды завершения
- **Git health check:** при старте `ralph run` — проверка: clean working tree, не detached HEAD, не в merge/rebase. Предотвращает каскадные сбои (commit fails, diff fails, dirty recovery fails)
- **Dirty state recovery:** при resume после прерывания (Ctrl+C, crash) — `git checkout -- .` для восстановления чистого состояния. При незавершённом execute (max-turns) — resume-extraction коммитит WIP, следующий execute продолжает с WIP-состояния
- **Exclusive repo access:** ralph ожидает эксклюзивный доступ к репозиторию во время `ralph run`. Ручные правки кода между сессиями могут привести к конфликтам. Это ограничение, не баг — single developer workflow
- **999-правила в execute промпте:** Guardrail 999-rules включаются в execute-промпт. Когда execute видит review-findings.md — 999-правила служат последним барьером: даже если finding предлагает опасное действие, execute откажется. Review-сессии 999-правила нужны только для валидации (проверить что execute не нарушил)
- **Red-green principle в execute промпте:** Execute промпт должен включать правило: "Тест должен падать при удалении реализации". Защита от trivial tests (`assert(true)`), которые LLM-as-Judge (review) может не поймать из-за общих blind spots. Дополнительная митигация: review agents на разных моделях (sonnet/haiku — разные bias), cross-model review в Growth
- **Bridge: проверка test framework:** Bridge промпт должен проверять наличие test framework в проекте. Если нет — первая задача = настройка тестов (часть FR5 project setup). Без этого execute упадёт на первой же задаче с тестами
- **MVP = Claude Code CLI only:** Все вызовы LLM через `claude` CLI с флагами `-p`, `--max-turns`, `--resume`, `--dangerously-skip-permissions`. Task tool для sub-агентов — специфика Claude Code. `--resume` используется для resume-extraction. Поддержка других LLM (GPT-4, Gemini) — Growth через session adapter
- **Кастомные промпты — ручная совместимость:** При обновлении ralph кастомные промпты в `.ralph/agents/` могут потребовать ручной адаптации. Механизм version check для промптов отсутствует. Известное ограничение MVP
- **Resume-extraction failure — потеря прогресса:** Если resume-extraction упадёт (API timeout, rate limit), WIP не закоммичен, прогресс не записан. Ralph fallback: `git checkout -- .` → retry задачи с чистого листа. Потеря одной попытки, не блокировка. Resilient resume-extraction — Growth

### Cross-Cutting Concerns

| Concern | Влияние |
|---------|---------|
| **Context window 40-50%** | Определяет структуру промптов, --max-turns, объём контекста. MVP: компактные промпты + --max-turns. Context budget calculator — Growth |
| **Fresh session principle** | Каждый компонент (bridge, execute, review, distillation) — изолированный вызов Claude. Resume-extraction — единственное исключение (`claude --resume` execute-сессии при неуспехе). Execute и "fix" — один тип сессии (Claude смотрит: review-findings.md пуст → реализовать, не пуст → исправить). review-findings.md — транзиентный: перезаписывается review при findings, очищается при clean review. Review записывает findings-знания в LEARNINGS.md + CLAUDE.md (без отдельной extraction-сессии). Distillation — отдельная лёгкая сессия (`claude -p`), запускается ralph при превышении бюджета LEARNINGS.md |
| **State consistency** | sprint-tasks.md — single source of state. Review-сессия пишет `[x]` при clean review (execute НЕ трогает статус). Ralph сканирует. Crash recovery: dirty tree → git checkout → retry. При Ctrl+C незавершённая задача остаётся `[ ]` — review не подтвердил качество |
| **Graceful failure** | Retry с backoff, emergency gate, resume — пронизывает все компоненты. Ctrl+C → signal.Notify + context cancellation. sprint-tasks.md не требует обновления при Ctrl+C: незавершённая задача остаётся `[ ]`, при resume — git checkout + retry |
| **Knowledge lifecycle (critical path)** | Три механизма extraction: (1) execute пишет learnings (best effort, инструкция в промпте), (2) review записывает findings-знания при анализе, (3) resume-extraction при неуспехе execute (`claude --resume` — коммит WIP + прогресс + знания). LEARNINGS.md append с hard limit **200 строк** (≈3,500 токенов, <2% context window) + distillation-сессия (`claude -p`) при превышении бюджета — запускается ralph после clean review. Distillation target: ~100 строк (50% бюджета). Бюджет — hardcoded constant в `runner/knowledge.go` (MVP), configurable `learnings_budget` в Growth. CLAUDE.md секция ralph — обновляется review и resume-extraction. ОПАСНЫЙ FEEDBACK LOOP: больше ошибок → больше LEARNINGS → меньше места → больше ошибок. Hard limit + distillation разрывают цикл. **5 Whys insight:** LEARNINGS.md — главный leverage point для снижения числа review→fix циклов |
| **Serena (high impact, dual value)** | Detect -> full index (timeout 60s) -> incremental (timeout 10s configurable) -> fallback с progress output. Двойная ценность: (1) token economy в execute — без Serena Claude читает файлы целиком, теряя до 30% контекста; (2) review accuracy — sub-агенты с Serena проверяют related code и интерфейсы, без неё судят только по diff (больше false positives). Рекомендуется для проектов >50 файлов |
| **Logging** | MVP: stdout цветной human-friendly (одна строка на событие, live-строка через `\r`) + простой text log (append `timestamp event details\n`). Structured log format — Growth |
| **Real-time feedback** | Статусные переходы задач в stdout. Без streaming Claude output — только результаты и тайминги |
| **Review quality data** | review-findings.md транзиентный (текущая задача). Review сама сохраняет паттерны в LEARNINGS.md при findings. Лог findings (agent, severity, file) в .ralph/logs/ — данные для будущего анализа. Severity filtering и формальные метрики — Growth |

### Testing Strategy (ralph itself)

| Уровень | Инструмент | Что тестирует |
|---------|-----------|---------------|
| **Unit tests** | Go built-in (`testing`) | Config loading, state scanning, prompt assembly |
| **Integration tests** | Go + mock Claude (скрипт-заглушка) | Полные сценарии: execute→review loop, human gates, retry logic, graceful shutdown, dirty state recovery |
| **Prompt snapshot tests** | Go + golden files | Diff промптов с baseline. Изменение промпта = осознанное обновление snapshot |
| **Golden file tests (bridge)** | Go + testdata/ | Input story → ожидаемый sprint-tasks.md. Регрессия в конвертации = сломанный тест |

Mock Claude: Go-скрипт (или shell wrapper), возвращающий предопределённые ответы по сценарию. Позволяет тестировать ralph без реальных API-вызовов. Smoke tests с real Claude — Growth (CI с API key).

**Критические промпты (требуют golden file / snapshot тестов):**
- **Review findings prompt** — качество findings = bottleneck всей системы. Плохой finding → плохой fix → лишний цикл → трата денег. Каждый finding должен содержать ЧТО/ГДЕ/ПОЧЕМУ/КАК. Sub-агенты возвращают свободный текст (MVP); структурированный формат — Growth (для severity filtering)
- **Distillation prompt** — определяет что "ценное" в LEARNINGS.md при сжатии. Ошибка = потеря важного паттерна или засорение бесполезным
- **Bridge merge prompt** — при smart merge (FR4) не должен сбросить `[x]` у выполненных задач. Регрессия = потеря прогресса спринта

### Go Package Structure (MVP)

| Package | Ответственность |
|---------|----------------|
| `cmd/ralph` | CLI entry point, flag parsing, colored output, log file writer |
| `bridge` | Конвертер stories → sprint-tasks.md (через Claude-сессию) |
| `runner` | Основной loop (execute → [resume-extraction] → review), state scanning (grep), git health check, knowledge append + distillation-сессия (отдельный `claude -p` при превышении бюджета LEARNINGS.md), dirty state recovery. Два счётчика на задачу: `execute_attempts` (нет коммита → resume-extraction → WIP commit → retry) и `review_cycles` (review нашёл findings → повторный execute→review). Resume-extraction через `claude --resume` при неуспешном execute |
| `session` | Запуск Claude-сессий (os/exec), stdout capture, error handling, `--resume` support |
| `gates` | Human gate logic, interactive stdin prompts |
| `config` | Каскадный config (YAML + CLI flags + go:embed defaults) |

**Growth packages** (выделяются при необходимости):
- `review` — из runner, когда severity filtering и custom agents усложняют логику
- `knowledge` — из runner, когда smart update CLAUDE.md и distillation вырастут
- `state` — из runner, если state management станет сложнее
- `logger` — из cmd, когда structured log и analytics потребуют отдельной логики

### Competitive Position

**Сравнение с экосистемой Ralph (weighted total по 10 критериям, max 200):**

| Решение | Score | Главная сила | Главная слабость |
|---------|:-----:|-------------|-----------------|
| **bmad-ralph** | **156** | Quality assurance (5/5) — 4 review-агента + ATDD, разрыв +2 от ближайшего | Onboarding (3/5) — привязка к BMad, порог входа |
| Farr Playbook | 155 | Автономность (5/5) — одна сессия на задачу, минимум overhead | Quality assurance (3/5) — только backpressure, нет dedicated review |
| Ralphex | 137 | Context management (5/5) — минимальный контекст между фазами | Knowledge retention (2/5) — нет системы знаний |
| Canonical Ralph | 132 | Onboarding (5/5) — один bash-файл, 2 минуты до результата | Quality assurance (1/5) — ноль review, ноль ATDD |

**Наши лидерские позиции:**
- Quality assurance: 5 vs 3 (Farr) — единственное решение с полноценным review pipeline
- Knowledge retention: 5 vs 4 (Farr) — hard limit + автоматическая дистилляция
- Resume / crash recovery: 5 vs 4 (Farr) — git health check + dirty state recovery

**Осознанные trade-offs:**
- Onboarding (3 vs 5 Canonical) — плата за BMad-интеграцию. Митигация: quick start в Growth
- Technology risk (3 vs 5 Canonical/Farr) — Go вместо bash. Плата за тестируемость и maintainability
- Community (1 vs 5 Canonical) — новый продукт. Митигация: quality docs + example project

## Starter Template Evaluation

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

## Core Architectural Decisions

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

## Implementation Patterns & Consistency Rules

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

## Project Structure & Boundaries

### Complete Project Directory Structure

```
bmad-ralph/
├── .github/
│   └── workflows/
│       └── ci.yml                  # test + lint + build (Go 1.25, 1.26 matrix)
├── .goreleaser.yml                 # Cross-platform binary builds + GitHub Releases
├── .golangci.yml                   # Linter configuration
├── .gitignore
├── Makefile                        # make build, make test, make lint, make release
├── go.mod                          # Go 1.25
├── go.sum
├── LICENSE
├── cmd/ralph/
│   ├── main.go                     # Cobra root, signal.NotifyContext, log file, var version = "dev"
│   ├── bridge.go                   # bridge subcommand (flag wiring → bridge.Run)
│   └── run.go                      # run subcommand (flag wiring → runner.Run)
├── bridge/
│   ├── bridge.go                   # bridge.Run(ctx, cfg) — story → sprint-tasks.md
│   ├── bridge_test.go
│   ├── prompts/                    # go:embed
│   │   └── bridge.md               # Bridge prompt template (text/template)
│   └── testdata/                   # Golden files: input stories → expected sprint-tasks.md
│       ├── TestBridge_SingleStory.golden
│       └── TestBridge_MultiStory.golden
├── runner/
│   ├── runner.go                   # runner.Run(ctx, cfg) — main loop
│   ├── runner_test.go
│   ├── git.go                      # GitClient interface + ExecGitClient implementation
│   ├── git_test.go
│   ├── scan.go                     # sprint-tasks.md scanning (TaskOpen, TaskDone, GateTag)
│   ├── scan_test.go
│   ├── knowledge.go                # LEARNINGS.md append, budget check (200 lines hard limit), distillation trigger
│   ├── knowledge_test.go
│   ├── prompts/                    # go:embed
│   │   ├── execute.md              # Execute prompt template
│   │   ├── review.md               # Review prompt template
│   │   ├── distillation.md         # LEARNINGS.md compression prompt
│   │   └── agents/
│   │       ├── quality.md
│   │       ├── implementation.md
│   │       ├── simplification.md
│   │       └── test-coverage.md
│   └── testdata/                   # Fixtures for runner integration tests
│       ├── sprint-tasks-basic.md       # Example sprint-tasks.md
│       ├── sprint-tasks-with-gate.md   # sprint-tasks.md with [GATE] tags
│       ├── review-findings-sample.md   # Example review-findings.md
│       ├── learnings-over-budget.md    # LEARNINGS.md exceeding budget
│       ├── TestPrompt_Execute.golden   # Assembled execute prompt snapshot
│       ├── TestPrompt_Review.golden    # Assembled review prompt snapshot
│       └── TestRunner_HappyPath.golden
├── session/
│   ├── session.go                  # session.Execute(ctx, opts) — Claude CLI invocation
│   ├── session_test.go
│   └── result.go                   # SessionResult struct (session_id, exit_code, output)
├── gates/
│   ├── gates.go                    # gates.Prompt(ctx, gate) — interactive stdin
│   └── gates_test.go
├── config/
│   ├── config.go                   # config.Load(flags) — YAML + CLI + defaults cascade
│   ├── config_test.go
│   ├── constants.go                # TaskOpen, TaskDone, GateTag, FeedbackPrefix + regex patterns
│   ├── shared/                     # go:embed
│   │   └── sprint-tasks-format.md  # Format contract (shared between bridge & execute)
│   └── testdata/
│       ├── TestConfig_Default.golden
│       └── TestConfig_Override.golden
├── internal/
│   └── testutil/
│       ├── mock_claude.go          # Scenario-based mock Claude CLI
│       ├── mock_git.go             # MockGitClient implementation
│       └── scenarios/              # JSON scenario files for integration tests
│           ├── happy_path.json
│           ├── review_findings.json
│           └── max_retries.json
└── docs/                           # Project documentation (не embed, не runtime)
```

**Version embedding:** `var version = "dev"` в `cmd/ralph/main.go`. goreleaser overrides через `-ldflags "-X main.version=v1.0.0"`. Cobra `rootCmd.Version = version`.

**Runner split boundary:** В MVP `runner/` содержит loop + git + scan + knowledge (~600-800 LOC est.). Когда превышает ~1000 LOC — выделять `review`, `knowledge`, `state` как отдельные packages (Growth packages list). Агенты НЕ должны split'ить runner в MVP.

### Runtime `.ralph/` Structure (в проекте пользователя)

```
user-project/
├── .ralph/
│   ├── config.yaml                 # User config
│   ├── agents/                     # Custom review agent overrides (.md)
│   ├── prompts/
│   │   └── execute.md              # Custom execute prompt override
│   └── logs/
│       ├── run-2026-02-24-143022.log
│       └── run-2026-02-24-160515.log
├── sprint-tasks.md                 # Bridge output, main state file
├── review-findings.md              # Transient — current task findings
├── LEARNINGS.md                    # Accumulated knowledge
└── CLAUDE.md                       # Operational context (ralph section)
```

### FR → Package Mapping

| FR Category | Package | Ключевые FR |
|-------------|---------|-------------|
| **Bridge** | `bridge` | FR1-FR5a: story → sprint-tasks.md, AC-derived tests, human gates, smart merge |
| **Execute loop** | `runner` | FR6-FR12: sequential execution, fresh sessions, retry, resume-extraction |
| **Review** | `runner` | FR13-FR19: 4 sub-agents, verification, findings → execute fix cycle |
| **Human gates** | `gates` | FR20-FR25: approve/retry/skip/quit, emergency gate, checkpoint |
| **Knowledge** | `runner` (knowledge.go) | FR26-FR29: LEARNINGS.md, CLAUDE.md section, distillation |
| **Config** | `config` | FR30-FR35: YAML + CLI cascade, agent files fallback |
| **Guardrails** | `runner` (prompts) | FR36-FR41: 999-rules in execute prompt, ATDD, Serena |
| **CLI** | `cmd/ralph` | Exit codes, flag parsing, colored output, log file |
| **Claude CLI** | `session` | All session invocations, --output-format json, --resume |

**Принцип:** Architecture doc маппит FR → packages. Story files при реализации указывают конкретные файлы. Это level of detail для stories, не для architecture.

### Package Boundaries — Кто что читает/пишет

| File | Creator | Reader | Writer |
|------|---------|--------|--------|
| `sprint-tasks.md` | `bridge` (через Claude) | `runner` (scan), Claude (execute/review) | Claude (execute: progress, review: `[x]`), ralph (feedback) |
| `review-findings.md` | Claude (review) | Claude (execute) | Claude (review: overwrite/clear) |
| `LEARNINGS.md` | Claude (execute, review, resume-extraction) | `runner` (budget check), Claude (all sessions) | Claude sessions (append), ralph → distillation session (rewrite при превышении бюджета) |
| `CLAUDE.md` section | Claude (review, resume-extraction) | Claude (all sessions) | Claude (update ralph section) |
| `.ralph/config.yaml` | User | `config` | Never (read-only) |
| `.ralph/logs/*.log` | `cmd/ralph` | User (post-mortem) | `cmd/ralph` (append) |
| Git state | Claude (execute: commit) | `runner` (HEAD check, health check) | Claude (commit), resume-extraction (WIP commit) |

### Data Flow

```
Stories (.md) ──→ bridge ──→ sprint-tasks.md
                                  │
                    ┌─────────────┘
                    ▼
              runner.Run loop:
                    │
              Serena incremental index (best effort, timeout)
                    │
              ┌─────┴──────┐
              ▼             │
         session.Execute    │
         (execute prompt)   │
              │             │
         commit? ──NO──→ session.Execute(--resume)
              │           resume-extraction
              │             │ WIP commit
              │             │ progress → sprint-tasks.md
              │             │ knowledge → LEARNINGS.md
              │             └──→ retry execute
              │YES
              ▼
         session.Execute
         (review prompt)
         4 sub-agents (+ Serena MCP inside sessions)
              │
         findings? ──YES──→ review-findings.md
              │              knowledge → LEARNINGS.md
              │              retry execute (fix)
              │NO (clean)
              ▼
         mark [x], clear review-findings.md
         budget check LEARNINGS.md
              │
         over budget? ──YES──→ session.Execute(distillation)
              │
              ▼
         next task (or GATE → gates.Prompt)
```

### Integration Points

| Point | From | To | Mechanism |
|-------|------|----|-----------|
| **CLI → Packages** | `cmd/ralph` | `runner.Run`, `bridge.Run` | Direct function call |
| **Runner → Claude** | `runner` | Claude CLI | `session.Execute` (os/exec) |
| **Runner → Git** | `runner` | git CLI | `GitClient` interface (os/exec) |
| **Runner → Gates** | `runner` | `gates` | `gates.Prompt` (stdin/stdout) |
| **Runner → Config** | `runner` | `config` | `config.Config` struct (read-only) |
| **Runner → Serena** | `runner` | Serena CLI | `os/exec` incremental index перед execute (best effort, timeout) |
| **Claude → Serena** | Claude sessions | Serena MCP | Внутри sessions: semantic code retrieval (Claude manages) |
| **Claude → Files** | Claude sessions | Filesystem | Direct read/write (sprint-tasks.md, LEARNINGS.md, etc.) |
| **Ralph → Files** | `runner`, `cmd/ralph` | Filesystem | `os.ReadFile`/`os.WriteFile` (scan, log, feedback) |

### Test Scenario Format

Integration тесты runner используют scenario-based mock Claude. Принцип: scenario = ordered sequence of mock responses, каждый response соответствует одному вызову `session.Execute`:

```json
{
  "name": "happy_path",
  "steps": [
    {"type": "execute", "exit_code": 0, "session_id": "abc-123",
     "creates_commit": true},
    {"type": "review", "exit_code": 0, "session_id": "def-456",
     "output_file": "review_clean.txt", "creates_commit": false}
  ]
}
```

Mock Claude script читает scenario JSON, возвращает responses по порядку. `creates_commit` — mock git отвечает что HEAD изменился. Exact schema определяется при реализации; принцип фиксируется здесь.

**Golden files для промптов:** собранный промпт (после text/template + strings.Replace) = golden file в `testdata/` того package, который собирает промпт. `runner/testdata/TestPrompt_Execute.golden`, `bridge/testdata/TestPrompt_Bridge.golden`.

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
Все 11 ключевых решений работают вместе без конфликтов:
- Go 1.25 + Cobra + yaml.v3 + fatih/color — стабильная, протестированная комбинация
- `os/exec` для git и Claude CLI + `exec.CommandContext` для cancellation — единый subprocess паттерн
- `text/template` + `strings.Replace` двухэтапная сборка совместима с `go:embed` per-package промптами
- `--output-format json` Claude CLI + golden file тесты — контрактная валидация
- goreleaser + GitHub Actions CI + golangci-lint — стандартный Go CI/CD pipeline
- GitClient interface в runner + MockGitClient в testutil — testability без изменения prod кода

**Pattern Consistency:**
- Naming: все 9 паттернов следуют Go conventions (PascalCase/camelCase, Err prefix, Test naming)
- Error handling: единая цепочка `fmt.Errorf("pkg: op: %w")` → `errors.Is/As` → exit code mapping только в `cmd/ralph`
- File I/O: единый паттерн `os.ReadFile` / `os.WriteFile` + `strings.Split` scan — без bufio
- Subprocess: все через `exec.CommandContext(ctx)` — единый cancellation path
- Testing: table-driven по умолчанию, golden files для промптов, scenario mock для integration
- Logging: packages не логируют → возвращают results/errors → cmd/ralph решает

**Structure Alignment:**
- Package structure поддерживает все decisions: каждый package embed-ит свои промпты, config = leaf
- Dependency direction строго сверху вниз, нет циклических зависимостей
- `internal/testutil/` содержит shared test infrastructure (mock Claude, mock git, scenarios)
- Runtime `.ralph/` structure чётко отделена от исходников bmad-ralph

### Requirements Coverage Validation ✅

**Functional Requirements Coverage (41/41):**

| FR Category | FRs | Package | Покрытие |
|-------------|-----|---------|----------|
| Bridge | FR1-FR5a | `bridge` | ✅ story→sprint-tasks.md, AC tests, smart merge, test framework check |
| Execute | FR6-FR12 | `runner` | ✅ sequential loop, fresh sessions, --max-turns, retry, resume-extraction |
| Review | FR13-FR19 | `runner` | ✅ 4 sub-agents (Task tool), verification, findings→fix cycle, clean→[x] |
| Gates | FR20-FR25 | `gates` | ✅ approve/retry/skip/quit, emergency gate, checkpoint |
| Knowledge | FR26-FR29 | `runner` (knowledge.go) | ✅ LEARNINGS.md (200 lines budget), CLAUDE.md section, distillation, --always-extract |
| Config | FR30-FR35 | `config` | ✅ YAML + CLI cascade, go:embed defaults, agent files fallback |
| Guardrails | FR36-FR41 | `runner` (prompts), `session` | ✅ 999-rules, ATDD, Serena detect+fallback |

**Growth FRs acknowledged (not blocked):** FR16a (severity filter), FR19 (batch review), FR40 (version check + session adapter), FR41 (context budget calculator — будет использовать 200-line budget как вход).

**Non-Functional Requirements Coverage (20/20):**

| NFR | Concern | Architectural Support |
|-----|---------|----------------------|
| NFR1 | Context 40-50% | Fresh sessions + --max-turns + компактные промпты + LEARNINGS.md 200 lines (≈3,500 токенов <2%) |
| NFR2 | Overhead <5s | Go native performance, scan с regex — мгновенно |
| NFR3 | Batch diff check | Runner проверяет diff size, warning |
| NFR4 | No destructive git | 999-rules в execute prompt |
| NFR5 | No API keys | Claude CLI own auth |
| NFR6 | --dangerously-skip-permissions | session.Execute flags |
| NFR7 | Documented CLI params | session package — только -p, --max-turns, --resume, --allowedTools |
| NFR8 | Serena best effort | detect → full index (60s) → incremental (10s) → fallback |
| NFR9 | Any git repo | Нет assumptions о языке/фреймворке |
| NFR10 | Crash recovery | sprint-tasks.md = state, git checkout → retry |
| NFR11 | Atomic commits | Green tests → commit, WIP only via resume-extraction |
| NFR12 | Claude CLI retry | Exponential backoff, configurable max |
| NFR13 | Graceful shutdown | signal.NotifyContext → ctx cancellation → double Ctrl+C kill |
| NFR14 | Log file | .ralph/logs/run-*.log, append-only |
| NFR15 | Knowledge limits | LEARNINGS.md 200 lines hard limit + distillation. CLAUDE.md section — review/resume update |
| NFR16 | Single binary | Go cross-compile, goreleaser |
| NFR17 | Zero runtime deps | git + claude CLI only |
| NFR18 | Prompt files | go:embed + .ralph/agents/ fallback chain |
| NFR19 | Add review agent | Просто добавить .md файл |
| NFR20 | Isolated components | 6 packages, minimal deps, config = leaf |

### Implementation Readiness Validation ✅

**Decision Completeness:**
- Все critical decisions с конкретными версиями и обоснованиями
- Implementation sequence определён: config → session → gates → bridge → runner → cmd/ralph
- Cross-component dependencies явно задокументированы
- External dependencies финализированы: Cobra, yaml.v3, fatih/color (3 total)

**Structure Completeness:**
- Полное дерево проекта до файлов с описанием каждого файла (Step 6)
- Runtime `.ralph/` structure отдельно
- FR → Package mapping полный
- Package boundaries (кто что читает/пишет) — таблица
- Data flow diagram — ASCII

**Pattern Completeness:**
- 56+ паттернов в 7 категориях с примерами и anti-patterns
- Enforcement guidelines (8 правил для AI-агентов)
- Structural patterns включают dependency diagram
- Testing patterns включают golden file workflow и scenario mock format с JSON примером

### Gap Analysis Results

**Critical Gaps: 0** — все blocking decisions приняты.

**Important Gaps (resolved during validation):**

| # | Gap | Решение |
|---|-----|---------|
| 1 | LEARNINGS.md budget threshold не был конкретизирован | **200 строк** (≈3,500 токенов, <2% context window). Farr AGENTS.md = 60 строк × 3.3 (broader scope). Distillation target: ~100 строк. Hardcoded const в knowledge.go (MVP), configurable в Growth. Circuit breaker для feedback loop |
| 2 | Step 3 и Step 6 оба содержат project tree | Step 3 tree = краткая версия для Starter Evaluation context. Step 6 tree = authoritative (superset). Добавлено примечание |

**Nice-to-Have Gaps:**

| # | Gap | Приоритет |
|---|-----|-----------|
| 1 | Distillation prompt не описан в деталях (что считать "ценным" при сжатии) | Story-level — деталь промпта, не архитектуры |
| 2 | Context budget calculator (FR41, Growth) — формула подсчёта | Growth feature, формула: сумма промпт + LEARNINGS.md + CLAUDE.md section + findings ≈ tokens |

### Architecture Completeness Checklist

**✅ Requirements Analysis**

- [x] Project context thoroughly analyzed (PRD, 9 research documents)
- [x] Scale and complexity assessed (~10 Go packages, medium complexity)
- [x] Technical constraints identified (13 constraints + 2 ограничения)
- [x] Cross-cutting concerns mapped (9 concerns, detailed)

**✅ Architectural Decisions**

- [x] Critical decisions documented with versions (Go 1.25, Cobra, yaml.v3, fatih/color)
- [x] Technology stack fully specified (3 direct deps, stdlib for rest)
- [x] Integration patterns defined (9 integration points)
- [x] Performance considerations addressed (context 40-50%, overhead <5s)

**✅ Implementation Patterns**

- [x] Naming conventions established (9 patterns)
- [x] Structural patterns defined (4 patterns + dependency diagram)
- [x] Error handling patterns specified (6 patterns + anti-patterns)
- [x] Process patterns documented (subprocess, file I/O, logging, testing)

**✅ Project Structure**

- [x] Complete directory structure defined (every file described)
- [x] Component boundaries established (read/write table)
- [x] Integration points mapped (9 points)
- [x] Requirements to structure mapping complete (FR → Package)

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** HIGH

Обоснование: 41/41 FR покрыты, 20/20 NFR покрыты, 0 critical gaps, все decisions с конкретными versions, 22 Party Mode insight'а интегрированы (3 раунда review), 2 раунда Self-Consistency Validation (13 inconsistencies найдены и исправлены).

**Key Strengths:**
- Minimal dependency footprint (3 external deps) снижает supply chain risk
- Двухэтапная prompt assembly защищает от template injection в user-controlled files
- GitClient interface обеспечивает полную тестируемость без real git
- LEARNINGS.md 200-line budget с distillation — circuit breaker для feedback loop
- Sprint-tasks.md format contract через shared go:embed — единый source of truth
- Fresh session principle исключает context pollution между задачами
- 999-rules как последний барьер даже при плохих review findings

**Areas for Future Enhancement (Growth):**
- Context budget calculator (FR41) — подсчёт полного context usage до запуска сессии
- Structured log format — для автоматического анализа
- Session adapter — поддержка других LLM-провайдеров
- Severity filtering в review (FR16a) — приоритизация findings
- Smart CLAUDE.md update (knowledge package) — вынос из runner при росте
- Cross-model review — sub-агенты на разных моделях для снижения shared blind spots

### Implementation Handoff

**AI Agent Guidelines:**

- Строго следовать architectural decisions (решения, версии, обоснования)
- Использовать implementation patterns consistently (56+ паттернов, 8 enforcement rules)
- Соблюдать project structure и package boundaries (dependency direction, entry points)
- Обращаться к этому документу при любых архитектурных вопросах
- Ключевые паттерны будут скомпилированы в `project-context.md` для загрузки в CLAUDE.md

**Implementation Sequence:**
1. `config` — YAML parsing + defaults cascade + constants (фундамент)
2. `session` — Claude CLI invocation + JSON output + --resume
3. `gates` — interactive prompts + fatih/color
4. `bridge` — story → sprint-tasks.md (через session)
5. `runner` — main loop (execute → resume-extraction → review → distillation)
6. `cmd/ralph` — Cobra wiring, signal handling, log file, version

## Architecture Completion Summary

### Workflow Completion

**Architecture Decision Workflow:** COMPLETED ✅
**Total Steps Completed:** 8
**Date Completed:** 2026-02-24
**Document Location:** docs/architecture.md

### Final Architecture Deliverables

**Complete Architecture Document:**
- Все architectural decisions с конкретными версиями и обоснованиями
- 56+ implementation patterns для AI agent consistency
- Полная project structure до файлов с описанием каждого
- FR → Package mapping (41 FR, 6 packages)
- Validation: coherence, coverage, readiness — всё ✅

**Implementation Ready Foundation:**
- 15 architectural decisions
- 56+ implementation patterns в 7 категориях
- 6 architectural packages + cmd entry point
- 41 FR + 20 NFR = 61 requirements fully supported

**Quality Assurance Summary:**
- 3 раунда Party Mode review (22 insights интегрированы)
- 2 раунда Self-Consistency Validation (13 inconsistencies найдены и исправлены)
- Comprehensive gap analysis: 0 critical gaps

---

**Architecture Status:** READY FOR IMPLEMENTATION ✅

**Next Phase:** Story preparation → implementation using the architectural decisions and patterns documented herein.

**Document Maintenance:** Update this architecture when major technical decisions are made during implementation.
