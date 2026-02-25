# Project Context Analysis

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
