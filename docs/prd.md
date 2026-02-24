---
stepsCompleted: [1, 2, 3, 4, 7, 8, 9, 10, 11]
inputDocuments:
  - docs/analysis/product-brief-bmad-ralph-2026-02-24.md
  - docs/research/llm-as-judge-variants-research-2026-02-24.md
  - docs/research/research-1-serena-ralph-2026-02-24.md
  - docs/research/research-2-tdd-vs-prewritten-tests-2026-02-24.md
  - docs/research/research-3-e2e-in-ralph-2026-02-24.md
  - docs/research/research-4-review-agents-composition-2026-02-24.md
  - docs/research/research-5-human-gates-2026-02-24.md
workflowType: 'prd'
lastStep: 11
project_name: 'bmad-ralph'
user_name: 'Степан'
date: '2026-02-24'
---

# Product Requirements Document - bmad-ralph

**Author:** Степан
**Date:** 2026-02-24

## Executive Summary

**bmad-ralph** — open-source CLI-утилита (single Go binary), которая соединяет структурированное планирование BMad Method v6 с автономным выполнением через Ralph Loop. Утилита берёт BMad stories с acceptance criteria, конвертирует их в задачи через детерминированный bridge, и автономно выполняет в цикле с fresh context на каждой итерации. Встроенные параллельные review-агенты (4 шт.), три механизма извлечения знаний, интеграция с Serena для token economy, и гибкая система human gates — от полной автономии до ручного контроля каждой задачи.

Целевая аудитория: Mid-to-Senior разработчики, использующие Claude Code для автономной разработки и желающие масштабировать AI-driven workflow за пределы одной сессии. По личному опыту автора, без автоматизации 30-50% времени уходит на "обслуживание" AI вместо продуктовой работы.

### What Makes This Special

1. **BMad + Ralph в одном инструменте.** Первое решение, где AI планирует через BMad (PRD → Architecture → Stories) и выполняет через Ralph (fresh context, loop). Ни canonical Ralph, ни ralphex, ни bmalph этого не предлагают.
2. **Guardrails из Farr Playbook.** 999-series правила и промпт-структура для надёжного автономного выполнения — агент не уйдёт в сторону.
3. **Serena интеграция.** Агент читает только нужные символы через semantic code retrieval вместо целых файлов — меньше токенов на контекст, больше полезной работы за итерацию. Ни одна Ralph-реализация этого не имеет.
4. **Гибкий контроль.** От "запустил и ушёл на 100 задач" до "проверяю каждый экран" — одним флагом `--gates`.
5. **Correct flow.** При несогласии на human gate — правка оригинальной BMad story через Claude + автоматический re-bridge. Source of truth сохраняется.
6. **Knowledge extraction.** Три механизма: execute пишет learnings (best effort), review записывает findings-знания, resume-extraction при неуспехе (`claude --resume`). Качество растёт от спринта к спринту.
7. **ATDD enforcement через review-агента.** Четвёртый review-агент `test-coverage` проверяет: (a) каждый AC покрыт тестом, (b) тесты не trivial, (c) тесты действительно запускаются. Review ТОЛЬКО находит проблемы — фиксы выполняет свежая execute-сессия. Тесты (включая e2e) никогда не скипаются — если что-то падает, AI обязан исправить. Если AI определяет что не может справиться — срабатывает экстренный human gate для эскалации.

## Project Classification

**Technical Type:** cli_tool
**Domain:** general (low domain complexity)
**Orchestration Complexity:** medium — Go-оркестратор управляет Claude Code сессиями, параллельные sub-agents через Task tool, Serena incremental indexing, state management через markdown-файлы.

CLI-утилита для разработчиков без регуляторных ограничений. Основные технические вызовы — в оркестрации процессов, а не в доменной сложности.

## Success Criteria

### User Success

| Метрика | Целевое значение | Как измеряем |
|---------|-----------------|--------------|
| **Автономное выполнение** | 10-20 задач за один `ralph run` без вмешательства | Счётчик итераций между human gates |
| **Тесты зелёные** | >90% итераций завершаются с green tests (unit + e2e) | Статистика loop |
| **Экономия времени** | 3-5x по сравнению с ручной оркестрацией | Субъективная оценка пользователя |
| **Review полезность** | >50% замечаний ценные (MVP), >70% (production) | Субъективная оценка |
| **Correct flow** | Курс меняется правильно с первой попытки в >80% случаев | Количество повторных correct |
| **Review → fix** | Critical findings исправляются за 1-2 цикла execute→review | Статистика loop |

**Aha-moment:** Распланировал через BMad, запустил `ralph run` — и оно пошло выполняться. Без лишних телодвижений, настроек, ручного кормления контекста. Одна команда — автономное выполнение всех задач.

**Ключевой критерий:** Не количество задач без остановки, а **качество коррекции курса**. Страшно не то, что human gate сработал, а то, что после feedback система не может перестроиться.

### Business Success

| Метрика | 3 месяца | 12 месяцев |
|---------|----------|------------|
| **GitHub Stars** | 100+ | 1000+ |
| **Активные пользователи** | 10-20 | 100+ |
| **Community вклад** | Первые issues/PRs от сторонних людей | Регулярные контрибьюции, 5+ contributors |
| **Упоминания** | Посты в русскоязычных dev-сообществах | Англоязычные блоги/YouTube |

### Technical Success

| Метрика | Порог |
|---------|-------|
| **Task completion rate** | >70% задач выполнено автономно (без retry/correct) за спринт |
| **Correct-to-resolution** | 1-2 итерации после коррекции курса |
| **ATDD enforcement** | Каждый AC покрыт тестом, тесты (включая e2e) никогда не скипаются |
| **Экстренный human gate** | AI вызывает человека когда застрял — для MVP простая эскалация без circuit breaker |
| **Knowledge retention** | LEARNINGS.md содержит actionable паттерны после спринта |
| **Onboarding** | От `git clone` до первого успешного `ralph run` за <15 минут |

### Measurable Outcomes

1. **End-to-end flow работает:** BMad stories → bridge → run → задачи выполнены с green tests
2. **Автономность:** 10+ задач за один `ralph run` без вмешательства
3. **Zero test skip policy:** Ни один тест не пропускается — если падает, AI исправляет или эскалирует через экстренный human gate
4. **Review → action:** >50% замечаний review-агентов приводят к улучшению кода

## Product Scope

### MVP - Minimum Viable Product

1. **`ralph bridge`** — детерминированный конвертер BMad stories → `sprint-tasks.md` с AC-derived test cases, human gates, служебными задачами. Smart Merge при повторном запуске.
2. **`ralph run`** — loop с fresh context (Go single binary). Review после каждой задачи. Serena integration (best effort). ATDD-lite. Режимы: без gates, `--gates`, `--gates --every N`.
3. **4 параллельных review sub-агента** — quality, implementation, simplification, test-coverage. Critical finding = итерация не пройдена. Review сама записывает findings-знания.
4. **Human gates** — approve, retry (feedback → fix-задача), skip, quit. Экстренный human gate при застревании AI.
5. **Knowledge extraction (три механизма)** — (1) execute пишет learnings перед завершением (best effort), (2) review записывает findings-знания при анализе, (3) resume-extraction (`claude --resume`) при неуспехе execute — коммит WIP + прогресс + знания. Дистилляция LEARNINGS.md с hard limit при превышении бюджета.
6. **Guardrails** — 999-series правила из Farr Playbook в execute-промпте.

### MVP Phase 2 (после стабилизации основного loop)

1. **Correct flow** (c на human gate) — правка BMad story → автоматический re-bridge. Требует stable bridge + run + human gates.
2. **Circuit breaker** — автоматическая остановка при серии неуспехов (CLOSED/HALF_OPEN/OPEN).
3. **Lightweight review** — при малом diff (<50 строк) и быстром execute (<5 ходов) — упрощённый review (1 агент вместо 4). Фокус на проверке AC из story-файла.

### Growth Features (Post-MVP)

- Rollback retry: git reset + повтор задачи с feedback
- LLM-as-Judge test fixtures: формальный quality gate через `claude -p`
- Quick start: `ralph run --plan plan.md` для пользователей без BMad
- Notifications: desktop/Slack при human gate, завершении спринта
- Performance и security review agents
- Custom skills из LEARNINGS.md: автоматическое создание переиспользуемых навыков из повторяющихся паттернов (level 3 knowledge extraction)
- Cross-model review: внешний reviewer (Codex, другая модель) для независимой проверки
- Hook-based review: review через Claude Code hooks как альтернатива sub-agent подходу
- Vision-based LLM-as-Judge: скриншот-сравнение UI с макетом для автоматической проверки визуала
- "Fix the neighborhood" (Farr): автоматическое исправление не связанных падающих тестов при обнаружении
- Context budget calculator: подсчёт размера контекста (промпт + файлы) перед сессией, warning при >40% context window
- CLI version check: проверка совместимости версии Claude CLI при старте, compatibility matrix
- Review severity filtering: `review_min_severity` в конфиге, findings ниже порога не блокируют pipeline
- Session adapter (multi-LLM): абстракция вызовов для поддержки Gemini/других LLM-провайдеров
- Structured log format: JSON/tab-separated лог для автоматического анализа и метрик
- goreleaser: автоматизированная сборка и публикация binary через GitHub Releases
- Batch review (`--review-every N`): review после каждых N задач с аннотированным diff и маппингом TASK→AC→tests
- Smart resume: при crash recovery — если последний коммит соответствует текущей задаче, skip execute и сразу review (оптимизация лишнего API-call)
- Contributor guide: документация для контрибьюторов (setup, architecture overview, PR process)
- Safe extraction order: extraction → commit → очистка review-findings.md. При падении extraction — findings не очищаются, но задача не блокируется
- Resilient resume-extraction: retry resume-extraction при сбое, partial commit recovery

### Vision (Future)

- Полная BMad CLI интеграция: `ralph init` → `ralph plan` → `ralph bridge` → `ralph run`
- Multi-agent parallelism: несколько задач одновременно на разных ветках
- Team mode: несколько разработчиков, общий sprint-tasks.md
- Plugin system: кастомные review agents, bridge adapters, notification providers
- Dashboard с метриками спринта
- Интеграция с другими planning-фреймворками

## User Journeys

### Architecture Decision: Two-Phase Iteration Model

На основании deep research (4 отчёта в `docs/research/deep-research-ralph-review/`) принято архитектурное решение:

**Три типа сессий:**

1. **EXECUTE** — свежая сессия: Claude читает sprint-tasks.md, берёт первую `- [ ]` задачу. Если review-findings.md не пуст — сначала исправляет findings. Если пуст — реализует задачу с нуля. Запускает тесты (unit + e2e), коммитит при green. Execute-промпт включает инструкцию записать learnings в LEARNINGS.md перед завершением (best effort)
2. **REVIEW** — свежая сессия: 4 параллельных sub-агента через Task tool, верификация находок. **Review ТОЛЬКО анализирует — ничего не фиксит.** При findings — перезаписывает review-findings.md (только текущие проблемы, без task ID) + записывает уроки в LEARNINGS.md и обновляет секцию ralph в CLAUDE.md. При clean review — ставит `[x]`, очищает review-findings.md, distillation LEARNINGS.md если превышен бюджет
3. **RESUME-EXTRACTION** (`claude --resume <session-id>`) — краткое возобновление execute-сессии при неуспехе (нет коммита). Execute имеет полный контекст — знает что пыталась, где остановилась. Задачи: (1) коммитит WIP-состояние, (2) пишет прогресс в sprint-tasks.md (под текущей задачей), (3) записывает знания в LEARNINGS.md + CLAUDE.md секцию. По умолчанию: при неуспехе. `--always-extract`: после каждого execute

**Loop:** execute → [resume-extraction при неуспехе] → review (только при наличии коммита). Если review нашёл проблемы → перезаписывает review-findings.md + записывает уроки → следующий execute адресует findings → review проверяет фиксы (max `max_review_iterations` циклов, default 3). После лимита — emergency human gate.

**Ключевые упрощения:**
- Ralph не различает "первый execute" и "fix после review" — одна и та же сессия. Execute смотрит: review-findings.md пуст? → реализовать. Не пуст? → исправить
- review-findings.md — транзиентный файл для текущей задачи. Перезаписывается review при findings (только актуальные проблемы). Не требует task ID
- sprint-tasks.md — open format. Bridge создаёт структуру, Claude пишет в ней свободно. Ralph парсит только `- [ ]`, `- [x]`, `[GATE]`. Resume-extraction пишет прогресс прямо в sprint-tasks.md под текущей задачей
- Review сама записывает findings-знания в LEARNINGS.md + CLAUDE.md (без отдельной extraction-сессии)
- После clean review: review-findings.md очищается → distillation LEARNINGS.md при превышении бюджета → следующая задача

**Принцип:** "One context window, one activity, one goal" (Huntley). Review = только ревью. Реализация/фиксы = только execute.

**Обоснование:**
- Канонический Ralph (snarktank, ghuntley): ревью отсутствует, только backpressure
- Farr Playbook: ревью = backpressure внутри итерации, нет sub-агентов
- Ralphex (umputun): свежие сессии между фазами — прецедент. Мы идём дальше: review НЕ фиксит (чистое разделение ответственности)
- Community consensus: resume для ревью нарушает философию Ralph; свежий контекст + параллельные sub-агенты = best practice

**Частота ревью (MVP):** review после каждой задачи (review-every 1). Batch review (`--review-every N`) — Growth feature

### Journey 1: "Запустил и ушёл" (Happy Path)

Алексей — mid-level фронтенд, первый раз пробует bmad-ralph. У него BMad stories для CRUD-модуля (8 задач, 1 epic).

```
$ ralph bridge stories/crud-module.md
✓ Parsed 8 acceptance criteria
✓ Generated sprint-tasks.md (8 tasks + 2 service tasks)
✓ Human gates: TASK-1 (first in epic), TASK-6 (new UI screen)

🚦 HUMAN GATE: Review sprint-tasks.md before run
   [a]pprove  [r]etry with feedback  [s]kip  [q]uit
> a

$ ralph run
⏳ TASK-1: Setup project structure
  → EXECUTE: fresh session... ✓ (unit green, committed)
  → REVIEW: 4 sub-agents... ✓ clean (0 findings)
🚦 HUMAN GATE: First task in epic — verify direction
> a

⏳ TASK-2: Implement data model
  → EXECUTE: fresh session... ✓ (unit green, committed)
  → REVIEW: 4 sub-agents... found 2 issues → review-findings.md
  → EXECUTE: fresh session, sees findings... fixes, tests green ✓ committed
  → REVIEW: 4 sub-agents... ✓ clean, marks [x]
⏳ TASK-3: API endpoints
  → EXECUTE: fresh session... ✓
  → REVIEW: 4 sub-agents... ✓ clean
⏳ TASK-4..5: Validation + edge case tests
  → EXECUTE + REVIEW: ✓ (2 tasks, no issues)
⏳ TASK-6: List + Detail UI  [UI task]
  → EXECUTE: fresh session... ✓ (unit + e2e green — UI task triggers e2e)
  → REVIEW: 4 sub-agents... ✓ clean
🚦 HUMAN GATE: New UI screen — check in browser
> a

⏳ TASK-7..8: Form UI + E2E tests
  → EXECUTE + REVIEW: ✓

✅ Sprint complete! 8/8 tasks done.
   Commits: 10 (8 tasks + 2 fix commits)
   LEARNINGS.md updated with 3 new patterns
```

**Aha-moment:** Одна команда — 8 задач выполнены автономно. Проверил направление на первой задаче, глянул UI на шестой, и всё.

### Journey 2: "Ревью нашёл критический баг" (Execute → Review Loop)

Марина — senior backend, auth-модуль. Review находит race condition.

```
⏳ TASK-3: JWT token validation
  → EXECUTE: fresh session... ✓ (unit green, committed)
  → REVIEW: 4 sub-agents...
    quality:        ⚠ CRITICAL — race condition in token refresh
    implementation: ✓ AC met
    simplification: ✓ clean
    test-coverage:  ⚠ HIGH — missing e2e test for concurrent refresh
  → Verifying findings... 2 confirmed → review-findings.md

  → EXECUTE: fresh session, sees review-findings.md
    Fixing: race condition + adding e2e test... tests green ✓ committed

  → REVIEW (cycle 2/3): 4 sub-agents...
    quality:        ✓ race condition fixed
    test-coverage:  ✓ e2e concurrent test added, all e2e green
  → ✓ clean, marks [x]

⏳ TASK-4: ...continues
```

**Ключевое:** Review ТОЛЬКО нашёл и верифицировал проблемы — записал в файл. Следующий execute (та же сессия по типу, свежий контекст) увидел findings и исправил. Повторный review проверил фиксы (cycle 2 из max 3). Ralph не различает "первый execute" и "fix" — это одна и та же сессия.

### Journey 3: "AI застрял" (Emergency Human Gate)

Дмитрий — mid-level, интеграция со сторонним API. API вернул неожиданный формат.

```
⏳ TASK-4: Parse partner API response
  → EXECUTE: fresh session... ✗ (tests red — unexpected XML instead of JSON)
  → EXECUTE (retry 1): fresh session with failure context... ✗
  → EXECUTE (retry 2): fresh session... ✗

🚨 EMERGENCY HUMAN GATE: AI stuck after 3 retries
   Task: TASK-4 — Parse partner API response
   Error: Expected JSON, got XML. Test: test_parse_response

   [f]eedback (add context, retry)
   [s]kip task
   [q]uit sprint
> f
> "API changed to XML. Use xml2js, schema in docs/partner-api-v2.xsd"

⏳ TASK-4 (retry with feedback):
  → EXECUTE: fresh session + user feedback... ✓ (unit green)
  → REVIEW: 4 sub-agents... ✓ clean
```

**Ключевое:** AI честно признал что застрял. Человек дал контекст — одна итерация и задача решена.

### Journey 4 (Growth): "Batch Review" (Review Every N)

Олег — senior full-stack, рефакторинг (20 задач). Хочет экономить на ревью.

```
$ ralph run --review-every 5
⏳ TASK-1..4: Execute only (unit tests, no review yet)
⏳ TASK-5: Execute ✓ (unit green)
  → e2e checkpoint (review-every 5)... e2e green ✓
  → REVIEW (batch: TASK-1..5):
    Аннотированный diff по задачам + маппинг TASK → AC → tests
    4 sub-agents review cumulative diff...
    quality:        ⚠ HIGH — circular import (TASK-3)
    implementation: ✓
    simplification: ⚠ MEDIUM — duplicate util (TASK-2 & TASK-3)
    test-coverage:  ✓ all AC covered
  → 2 confirmed findings → review-findings.md

  → EXECUTE: fresh session, sees findings... fixes both issues, unit green ✓ committed
  → REVIEW (cycle 2/3): ✓ clean, marks [x] for TASK-1..5

⏳ TASK-6..10: next batch, review + e2e after TASK-10
```

**Ключевое:** При batch review, review-сессия получает аннотированный diff (разбит по задачам) и маппинг TASK → AC → expected tests, чтобы sub-агенты не потеряли контекст.

### Journey 5 (Phase 2): "Correct Flow" — курс поменялся

Степан использует bmad-ralph на своём проекте. На human gate понимает, что story нуждается в правке.

```
🚦 HUMAN GATE: TASK-6 — Payment form
   [a]pprove  [r]etry  [c]orrect  [s]kip  [q]uit
> c
> "Нужен не Stripe, а Tinkoff Pay. Переделай story."

→ Claude правит оригинальную BMad story (source of truth)
→ Автоматический re-bridge: sprint-tasks.md обновлён
→ TASK-6 перегенерирован с новыми AC
→ Продолжение ralph run с обновлённой задачей
```

**Ключевое (Phase 2):** Source of truth — BMad story. Correct flow правит story, а не sprint-tasks.md напрямую.

### Requirements Summary

| Требование | Journey | Приоритет |
|-----------|---------|-----------|
| Двухфазная итерация: execute → review (свежие сессии, единый тип execute) | 1, 2 | MVP |
| 4 параллельных review sub-агента через Task tool | 1, 2 | MVP |
| Execute→review loop с max_review_iterations (default 3) | 2 | MVP |
| Emergency human gate на max_review_iterations exceeded | 2 | MVP |
| Emergency human gate при N execute retry failures | 3 | MVP |
| User feedback → retry с контекстом | 3 | MVP |
| Review после каждой задачи (review-every 1) | 1, 2 | MVP |
| Review-сессия ставит `[x]` при clean review | 1, 2 | MVP |
| `--review-every N` (batch review) + аннотированный diff | 4 | Growth |
| Human gates на first-in-epic и user-visible milestones | 1 | MVP |
| E2e тесты запускаются в execute-фазе, test-coverage агент верифицирует покрытие | 1, 2 | MVP |
| Knowledge extraction (LEARNINGS.md) | 1 | MVP |
| Correct flow: правка BMad story → re-bridge | 5 | MVP Phase 2 |

## CLI Tool Specific Requirements

### Command Structure

| Команда | Тип | Описание |
|---------|-----|----------|
| `ralph bridge <story-files>` | One-shot | Конвертация BMad stories → sprint-tasks.md |
| `ralph run` | Long-running | Основной loop с execute → review фазами |

#### `ralph bridge`

```
ralph bridge stories/auth.md stories/crud.md
ralph bridge stories/              # все .md файлы в директории
ralph bridge --merge               # Smart Merge с существующим sprint-tasks.md
```

Output: `sprint-tasks.md` с задачами, AC-derived tests, human gates, служебными задачами.

#### `ralph run`

```
ralph run                          # review после каждой задачи
ralph run --gates                  # human gates включены (по разметке из bridge)
ralph run --gates --every 3        # + checkpoint каждые 3 задачи
ralph run --max-iterations 5       # max 5 попыток на задачу
ralph run --max-turns 50           # max 50 ходов Claude за execute-сессию
ralph run --always-extract         # extraction знаний после каждой итерации (не только failure)
```

### Configuration

Двухуровневая: **config file + CLI flags override**.

**Config file** (`.ralph/config.yaml`, формат YAML) в корне проекта:

**Приоритет:** CLI flags > `.ralph/config.yaml` > embedded defaults

#### Полная таблица параметров

| # | Параметр | CLI flag | Config key | Default | Описание |
|---|----------|----------|------------|---------|----------|
| 1 | Max turns per execute | `--max-turns N` | `max_turns` | 30 | Лимит ходов Claude Code за одну execute-сессию (review не ограничивается) |
| 2 | Max iterations per task | `--max-iterations N` | `max_iterations` | 3 | Попытки execute на задачу до emergency gate |
| 3 | Review frequency | — | — | 1 | Review после каждой задачи (MVP: всегда 1, `--review-every N` — Growth) |
| 4 | Review max iterations | — | `review_max_iterations` | 3 | Max циклов execute→review на задачу |
| 5 | Gates enabled | `--gates` | `gates_enabled` | false | Включить human gates |
| 6 | Gates checkpoint | `--every N` | `gates_checkpoint` | 0 | Checkpoint каждые N задач (0 = off) |
| 7 | Execute model | — | `model` | opus | Модель Claude для execute-фазы |
| 8 | Review agent models | — | `review_agents.*` | sonnet/haiku | Модель для каждого review-агента |
| 9 | Serena enabled | — | `serena_enabled` | true | Best effort Serena integration |
| 10 | Default branch | — | `default_branch` | auto-detect | База для git diff в review |
| 11 | Agent files dir | — | `agents_dir` | `.ralph/agents/` | Директория кастомных review agent `.md` файлов |
| 12 | Prompt file | — | `prompt_file` | `.ralph/prompts/execute.md` | Промпт для execute-фазы (override embedded default) |
| 13 | Claude command | — | `claude_command` | `claude` | Путь к Claude CLI |
| 14 | Paths | — | `paths.*` | defaults | sprint-tasks.md, LEARNINGS.md, CLAUDE.md |
| 15 | Always extract | `--always-extract` | `always_extract` | false | Extraction знаний после каждой итерации (не только failure/review) |
| 16 | Serena timeout | — | `serena_timeout` | 10 | Таймаут Serena incremental index (секунды). Full index при старте: 60s |

#### Agent files fallback chain

`.ralph/agents/` (project) > `~/.config/ralph/agents/` (global) > embedded defaults

### Output

| Канал | Формат | Назначение |
|-------|--------|------------|
| Terminal (stdout) | Цветной текст с progress indicators | Статус, human gates |
| `sprint-tasks.md` | Markdown | Задачи, статусы, AC |
| `LEARNINGS.md` | Markdown | Накопленные знания |
| `CLAUDE.md` | Markdown | Операционный контекст |
| `.ralph/logs/` | Text log | Полная история run для post-mortem |
| Git commits | Conventional commits | Результат задач/fixes |

### Exit Codes

| Code | Значение | Когда |
|------|----------|-------|
| 0 | Успех | Все задачи `[x]` |
| 1 | Частичный успех | Часть задач сделана, остановился по лимитам (gates off) |
| 2 | User quit | Пользователь выбрал quit на любом gate (обычном или emergency) |
| 3 | User interrupted | Ctrl+C (graceful shutdown) |
| 4 | Fatal error | Инфраструктурный сбой (нет git/claude, config, crash) |

### Dependencies

- `git`, `claude` CLI
- Ralph распространяется как single Go binary (zero runtime dependencies)

### Platform

- Linux, macOS — нативные бинарники
- Windows через WSL
- Распространение: `go install github.com/...` + GitHub Releases (goreleaser)

## Project Scoping & Risk

### MVP Strategy

**Approach: Problem-Solving MVP** — решить core problem (ручная оркестрация AI = 30-50% overhead) минимальным набором features.

**MVP = 7 компонентов:**
1. `ralph bridge` — story → tasks с AC-derived tests и human gates
2. `ralph run` — loop, execute → review (свежие сессии)
3. 4 review sub-агента — quality, implementation, simplification, test-coverage
4. Human gates — approve, retry, skip, quit + emergency gate
5. Knowledge extraction — LEARNINGS.md + CLAUDE.md
6. Guardrails — 999-series правила в execute-промпте
7. Configuration system — `.ralph/config` + CLI flags override, agent files fallback chain

**Aha-test:** Запустил `ralph run` — 10 задач выполнились автономно с quality review. Без ручного кормления контекста.

### Risk Mitigation

| Риск | Вероятность | Импакт | Митигация |
|------|:-----------:|:------:|-----------|
| **Orchestration complexity** — оркестрация 3 типов сессий, config parsing, human gates, subprocess management | Средняя | Средний | Go single binary: встроенный парсинг, типизация, тесты |
| **Нишевая аудитория** — нужны и BMad, и Ralph Loop | Средняя | Высокий | Quick start (`ralph run --plan`) в Growth снижает порог входа |
| **Claude Code API changes** — CLI flags, Task tool могут измениться | Средняя | Высокий | Абстракция вызовов через config (`claude_command`). Мониторинг changelog |
| **Review quality** — false positives от sub-агентов | Средняя | Средний | Верификация находок перед записью в findings. Настраиваемые agent models |
| **Solo developer** | Высокая | Средний | Lean MVP. Open source с первого дня |

## Функциональные требования

### Планирование задач (Bridge)

- **FR1:** Разработчик может конвертировать BMad story-файлы в структурированный sprint-tasks.md
- **FR2:** Система выводит тест-кейсы из объективных acceptance criteria в stories. Субъективные AC помечаются для ручной или LLM-as-Judge верификации (post-MVP)
- **FR3:** Система определяет и размечает точки human gate тегом `[GATE]` в строке задачи sprint-tasks.md (первая задача epic'а, user-visible milestones). Ralph сканирует `[GATE]` для определения остановочных точек
- **FR4:** Разработчик может повторно запустить bridge с Smart Merge для обновления существующего sprint-tasks.md
- **FR5:** Система генерирует служебные задачи на основе контекста stories. Типовые категории: (a) **project setup** — когда story подразумевает новые зависимости, фреймворки или инфраструктуру (install deps, test config, scaffold); (b) **integration verification** — когда несколько задач работают над разными частями одной фичи (backend + frontend); (c) **e2e checkpoint** (Growth, вместе с batch review) — перед batch review (review-every N) для раннего обнаружения регрессий. Bridge определяет необходимость по сигналам из stories
- **FR5a:** Каждая задача в sprint-tasks.md содержит поле `source:` со ссылкой на оригинальную story и AC (например `source: stories/auth.md#AC-3`). Обеспечивает трассировку задачи → story для correct flow, review и аудита

### Автономное выполнение (Run)

- **FR6:** Система последовательно выполняет задачи из sprint-tasks.md в цикле. При старте `ralph run` — проверка git health (clean state, не detached HEAD, не в merge/rebase)
- **FR7:** Каждое выполнение задачи происходит в свежей сессии Claude Code
- **FR8:** Execute-сессия читает задачу, реализует код, запускает unit-тесты, коммитит при green. e2e запускается только для UI-задач и на checkpoint-ах `review-every N`
- **FR9:** Система повторяет неудачные задачи до настраиваемого максимума итераций. Ralph определяет успешность execute по наличию нового git коммита: есть коммит → переход к review, нет коммита → resume-extraction → retry (execute_attempts++). Resume-extraction возобновляет execute-сессию (`claude --resume`), коммитит WIP, пишет прогресс в sprint-tasks.md и знания в LEARNINGS.md. Два независимых счётчика: `execute_attempts` (max_iterations, default 3) и `review_cycles` (max_review_iterations, default 3)
- **FR10:** Разработчик может ограничить количество ходов Claude Code за одну execute-сессию
- **FR11:** Execute-сессия Claude сама читает sprint-tasks.md и берёт первую невыполненную задачу (`- [ ]`) сверху вниз (модель Playbook — Claude self-directing). Review-сессия отмечает задачу выполненной (`[x]`) после подтверждения качества (clean review без critical findings). Execute-сессии НЕ изменяют статус задач. Ralph сканирует файл (grep `- [ ]`) только для контроля loop — есть ли ещё задачи. Ralph не извлекает описание задач и не передаёт их в промпт
- **FR12:** При повторном запуске `ralph run` система продолжает с первой незавершённой задачи в sprint-tasks.md. При обнаружении dirty working tree (прерванная сессия) — `git checkout -- .` для восстановления чистого состояния перед retry. Мягкая валидация: если sprint-tasks.md не содержит ни `- [ ]`, ни `- [x]` — warning с рекомендацией проверить файл

### Ревью кода

- **FR13:** Система запускает фазу ревью после каждой выполненной задачи
- **FR14:** Ревью выполняется в свежей сессии Claude Code, отдельной от execute
- **FR15:** Review-сессия запускает 4 параллельных sub-агента через Task tool (quality, implementation, simplification, test-coverage)
- **FR16:** Review-сессия верифицирует каждую находку sub-агентов и классифицирует как CONFIRMED или FALSE POSITIVE. Каждый finding получает severity (CRITICAL/HIGH/MEDIUM/LOW)
- **FR16a (Growth):** Findings с severity ниже настроенного порога (`review_min_severity`, default HIGH) записываются в лог, но не блокируют pipeline и не попадают в review-findings.md
- **FR17:** Review-сессия ТОЛЬКО анализирует — при clean review ставит `[x]` и очищает review-findings.md, при findings перезаписывает `review-findings.md` с confirmed findings (только актуальные проблемы текущей задачи, без task ID), записывает уроки в LEARNINGS.md и обновляет секцию ralph в CLAUDE.md, но НЕ вносит изменения в код. Каждый finding должен содержать достаточно информации чтобы следующая execute-сессия без дополнительного контекста могла понять: ЧТО не так, ГДЕ в коде, ПОЧЕМУ это проблема и КАК предлагается исправить
- **FR18:** При наличии findings система запускает следующую execute-сессию (тот же тип сессии — ralph не различает "первый execute" и "fix"). Execute видит непустой review-findings.md, адресует findings, запускает тесты, коммитит при green
- **FR18a:** После execute система запускает повторный review для верификации фиксов (цикл execute→review, до максимума `max_review_iterations` итераций, default 3)
- **FR19 (Growth):** При batch-ревью (`--review-every N`) система предоставляет аннотированный diff с маппингом TASK→AC→тесты

### Контроль качества (Gates)

- **FR20:** Разработчик может включить human gates через CLI-флаг
- **FR21:** Система останавливается на размеченных точках human gate для ввода разработчика
- **FR22:** Разработчик может одобрить, повторить с обратной связью, пропустить или выйти на gate. При retry с feedback ralph программно добавляет feedback в sprint-tasks.md под текущей задачей (индентированная строка `> USER FEEDBACK: ...`). Следующий execute читает sprint-tasks.md и видит feedback
- **FR23:** Система вызывает экстренный human gate когда AI исчерпал максимум попыток execute
- **FR24:** Система вызывает экстренный human gate когда цикл execute→review превысил максимум итераций
- **FR25:** Разработчик может установить периодические checkpoint gates каждые N задач

### Управление знаниями

- **FR26:** Система пишет операционные знания в секцию `## Ralph operational context` файла CLAUDE.md. Обновление — через review-сессию (при findings) и resume-extraction (при неуспехе execute): добавить новое, переформулировать для краткости, убрать дублирование (по модели Farr Playbook). Существующий контент проекта вне секции ralph не затрагивается
- **FR27:** Система записывает паттерны и выводы в LEARNINGS.md
- **FR28:** При неудачном execute (нет коммита) система возобновляет execute-сессию через `claude --resume` (resume-extraction). Execute-сессия имеет полный контекст — знает что пыталась, где застряла. Resume-extraction: (1) коммитит текущее WIP-состояние, (2) пишет прогресс под текущей задачей в sprint-tasks.md, (3) записывает причины неудачи и извлечённые знания в LEARNINGS.md + обновляет секцию ralph в CLAUDE.md
- **FR28a:** Review-сессия при наличии findings сама записывает уроки в LEARNINGS.md (какие типы ошибок, что агент забывает, паттерны для будущих сессий) и обновляет секцию ralph в CLAUDE.md — без отдельной extraction-сессии. Review имеет полный контекст анализа. При clean review — ставит `[x]`, очищает review-findings.md. После clean review ralph проверяет размер LEARNINGS.md и при превышении бюджета запускает отдельную distillation-сессию (`claude -p`). Если первый review сразу clean и LEARNINGS.md в пределах бюджета — distillation не нужна
- **FR28b:** Разработчик может включить флаг `--always-extract` для запуска resume-extraction после КАЖДОГО execute, включая успешные. Resume-extraction возобновляет execute-сессию и извлекает знания из **процесса выполнения** (какие решения принял Claude, что пошло хорошо, какие подходы сработали). Больше знаний, но дороже по токенам (дополнительный `claude --resume` на каждую задачу)
- **FR29:** Файлы знаний загружаются в контекст каждой новой сессии

### Конфигурация и кастомизация

- **FR30:** Разработчик может настраивать поведение через config-файл в корне проекта
- **FR31:** Разработчик может переопределять настройки config-файла через CLI-флаги
- **FR32:** Разработчик может кастомизировать промпты review-агентов через текстовые файлы
- **FR33:** Система использует fallback-цепочку: проектные → глобальные → встроенные конфигурации агентов
- **FR34:** Разработчик может настроить модель Claude для каждого review-агента
- **FR35:** Система возвращает информативные exit-коды для интеграции со скриптами

### Guardrails и ATDD

- **FR36:** Система применяет 999-series guardrail-правила в execute-промпте. 999-правила — последний барьер: даже если review-findings.md предлагает опасное действие, execute откажется
- **FR37:** Система обеспечивает ATDD: каждый acceptance criterion должен иметь соответствующий тест
- **FR38:** Система никогда не пропускает тесты — unit на каждый execute, e2e на UI-задачах и review-every checkpoint-ах. Падения исправляются или эскалируются
- **FR39:** Система обнаруживает наличие Serena MCP и использует его для чтения кода. Двойная ценность: (1) token economy в execute — semantic code retrieval вместо чтения целых файлов; (2) review accuracy — sub-агенты проверяют related code и интерфейсы, снижая false positives. При старте `ralph run` — полная индексация проекта (Serena full index, timeout 60s). Перед каждой execute-сессией — incremental index (timeout configurable, default 10s). При таймауте или недоступности Serena — graceful fallback на стандартное чтение файлов с progress output
- **FR40 (Growth):** При старте `ralph run` система проверяет версию Claude CLI (`claude --version`) и предупреждает при несовместимой или неизвестной версии. Все вызовы CLI абстрагированы через session adapter для поддержки будущих LLM-провайдеров
- **FR41 (Growth):** Перед запуском каждой execute-сессии система подсчитывает примерный размер контекста (промпт + контекстные файлы: CLAUDE.md секция, LEARNINGS.md, task description). При превышении порога 40% от context window — warning с рекомендацией сократить контекстные файлы

## Non-Functional Requirements

### Performance

- **NFR1:** Общая утилизация context window за execute-сессию не должна превышать 40-50%. После 50% модель теряет начальные инструкции — промпт, guardrails, 999-правила (Liu et al. 2024 "Lost in the Middle", NoLiMa/Adobe 2025, ClaudeLog 2025). Обеспечивается через: (a) fresh context на каждую задачу, (b) `--max-turns` как hard limit, (c) Serena для token-efficient чтения кода, (d) компактные контекстные файлы. При превышении суммарного размера контекстных файлов порога — ralph выводит warning
- **NFR2:** Overhead самого ralph (loop, парсинг sprint-tasks.md, запуск Claude) — не более 5 секунд между итерациями. Узкое место — Claude API, не ralph
- **NFR3:** При batch review ralph проверяет размер cumulative diff. Если превышает порог (~120K символов ≈ ~30K токенов), выводит warning с рекомендацией уменьшить review-every. Автоматическое разбиение diff — Growth feature

### Security

- **NFR4:** Ralph никогда не удаляет файлы пользователя и не выполняет `git reset --hard` или `git push --force` без явного human gate. Деструктивные git-операции запрещены в execute-промпте через 999-правила
- **NFR5:** Config-файл и промпты не содержат и не передают API-ключи. Claude CLI использует свой собственный auth
- **NFR6:** Ralph вызывает Claude Code через CLI с флагом `--dangerously-skip-permissions` для автономного выполнения. Безопасность обеспечивается не permission-системой Claude, а guardrails в промпте (999-правила), ATDD (тесты как backpressure), review-агентами и human gates

### Integration

- **NFR7:** Ralph использует документированные CLI-параметры Claude Code: `-p` (prompt), `--max-turns`, `--allowedTools`, `--dangerously-skip-permissions`. Никаких внутренних API или недокументированных флагов
- **NFR8:** Serena интеграция — best effort. Любой сбой Serena (таймаут, индексация не завершена, MCP недоступен) приводит к graceful fallback на стандартное чтение файлов, а не к ошибке
- **NFR9:** Ralph работает с любым git-репозиторием. Никаких предположений о структуре проекта, языке программирования или фреймворке

### Reliability

- **NFR10:** При аварийном завершении (kill, crash, потеря питания) ralph корректно возобновляет работу через `ralph run` — продолжая с первой незавершённой задачи. sprint-tasks.md = single source of state
- **NFR11:** Успешная итерация атомарна: commit происходит при green tests. WIP-коммиты допускаются только через resume-extraction при незавершённом execute (для сохранения прогресса). Промежуточное состояние (dirty working tree без commit) допустимо только внутри active execute-сессии
- **NFR12:** При сбое Claude CLI (API таймаут, rate limit, exit code != 0) ralph делает retry с exponential backoff. После N неудач (configurable, default 3) — останавливается с информативным сообщением и exit code
- **NFR13:** Graceful shutdown: при Ctrl+C ralph дожидается завершения текущей Claude-сессии (или убивает её по второму Ctrl+C) и выходит чисто. sprint-tasks.md не требует обновления — незавершённая задача остаётся `[ ]` (review ещё не подтвердил качество), при resume ralph подхватит её заново
- **NFR14:** Лог-файл `.ralph/logs/run-YYYY-MM-DD-HHMMSS.log` — append-only запись всех событий: старт/стоп задач, результаты тестов, review findings, human gate решения, ошибки. Для post-mortem анализа после длительных запусков
- **NFR15:** Knowledge files: LEARNINGS.md — append-only с hard limit (при превышении бюджета — distillation-сессия сжимает: оставляет ценное, убирает дублирование и устаревшее). CLAUDE.md секция ralph — обновляется review (при findings) и resume-extraction (при неуспехе): add/rewrite/deduplicate. review-findings.md — транзиентный файл для текущей задачи: перезаписывается review при findings, очищается при clean review

### Portability

- **NFR16:** Ralph распространяется как single Go binary. Поддержка: Linux, macOS (нативно), Windows (через WSL). Кросс-компиляция через `GOOS/GOARCH`
- **NFR17:** Единственные hard dependencies для пользователя: `git`, `claude` CLI. Ralph — single Go binary, zero runtime dependencies. Установка: `go install` или скачать binary из GitHub Releases

### Maintainability

- **NFR18:** Все промпты для Claude (execute, review agents, distillation) хранятся как отдельные текстовые файлы. Defaults встроены в binary через `go:embed`, кастомные файлы в `.ralph/agents/` имеют приоритет (fallback chain: project → global → embedded). Изменение промпта не требует пересборки
- **NFR19:** Добавление нового review-агента = добавление `.md` файла в директорию агентов. Не требует изменения кода ralph
- **NFR20:** Конкретная файловая структура компонентов определяется на этапе архитектуры. NFR-критерий: каждый компонент (bridge, runner, session, gates, config) — изолированная единица с минимальными зависимостями между ними
