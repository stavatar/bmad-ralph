# Синтез архитектурных исследований (8 отчётов)

**Дата:** 2026-03-07
**Статус:** Промежуточный синтез
**Источники:** 8 архитектурных отчётов из `docs/research/new_version/`
**Контекст:** bmad-ralph v1 завершён (10 эпиков, FR1-FR92, 82 story). Планирование v2 — переход от BMad-зависимости к самодостаточности.

---

## Оглавление

1. [Варианты решения — полное сравнение](#1-варианты-решения--полное-сравнение)
2. [Как устроены конкуренты](#2-как-устроены-конкуренты)
3. [BMad v6 — что ценно, что избыточно](#3-bmad-v6--что-ценно-что-избыточно)
4. [Архитектура ralph plan](#4-архитектура-ralph-plan)
5. [ralph init — быстрый старт](#5-ralph-init--быстрый-старт)
6. [ralph replan — коррекция курса](#6-ralph-replan--коррекция-курса)
7. [YAML формат задач](#7-yaml-формат-задач)
8. [Промпты plan.md и replan.md](#8-промпты-planmd-и-replanmd)
9. [Общие выводы и рекомендации](#9-общие-выводы-и-рекомендации)

---

## 1. Варианты решения — полное сравнение

**Источник:** `v2-variants-comparison.md`

### 1.1 Четыре варианта эволюции

Исследованы 4 принципиально разных подхода к устранению bottleneck'а в pipeline планирования ralph.

#### Вариант A: Bridge упрощается (программный парсинг)

**Суть:** Убрать LLM из bridge, заменить Go-парсером. Bridge остаётся, но парсит AC программно.

| Аспект | Значение |
|--------|----------|
| Архитектура | `bridge/` рефакторинг, `runner/` без изменений |
| Входные данные | BMad story файлы (строго `docs/sprint-artifacts/*.md`) |
| Выходные данные | `sprint-tasks.md` (идентичный текущему формату) |
| Зависимость от BMad | **Полная** — stories обязательны |
| Зависимость от LLM (planning) | **Нулевая** — программный парсинг |
| LOC нового кода | ~300 + ~600 тестов |
| LOC удаления | ~400 (bridge.md + session-код) |
| Stories | 2-3 |
| Дни | 1-2 |
| Качество декомпозиции | 6/10 |
| Time-to-first-task | 55-135 мин (bottleneck = BMad) |
| Стоимость planning | $0 (программный) |
| Гибкость (новые форматы) | 1/10 |
| Backward compatibility | Полная |

**Плюсы:**
- Детерминизм — одинаковый результат при каждом запуске
- Быстрота — мгновенное выполнение, нет Claude call
- Полная backward compatibility
- Минимальный объём работ

**Минусы:**
- Грубая группировка: keyword-эвристики покрывают ~60% случаев
- Regex привязан к конкретному формату story — хрупкость
- Не устраняет BMad-зависимость (стратегическая проблема)
- Нулевая гибкость для новых форматов

#### Вариант B: Runner работает со stories напрямую

**Суть:** Убрать bridge целиком. Runner напрямую читает story файлы, каждая story = scope Claude-сессии.

| Аспект | Значение |
|--------|----------|
| Архитектура | `runner/` значительно переработан, `bridge/` удалён |
| Входные данные | BMad story файлы |
| Выходные данные | `.ralph-state.yaml` (внутренний state) |
| Зависимость от BMad | **Полная** |
| LOC нового кода | ~800 + ~1500 тестов |
| LOC удаления | ~2800 (bridge/ целиком) |
| Stories | 4-6 |
| Дни | 3-5 |
| Качество декомпозиции | **8/10** (лучший результат) |
| Гибкость | 1/10 (ещё хуже — runner привязан к story формату) |
| Backward compatibility | **Нет** |

**Плюсы:**
- Лучшее качество декомпозиции — Claude видит ВСЮ story целиком
- 2 AI-слоя вместо 3 (BMad + Runner)
- Story = единственный source of truth

**Минусы:**
- Нет checkpoint'ов внутри story (8 AC = одна сессия)
- Большие stories могут не поместиться в контекст
- Ломает backward compatibility (sprint-tasks.md удаляется)
- 20+ тестов runner'а потенциально ломаются

#### Вариант C: Ralph работает с epics

**Суть:** Убрать промежуточный слой stories. Ralph обрабатывает epic целиком.

| Аспект | Значение |
|--------|----------|
| Архитектура | `runner/` полная переработка, `bridge/` удалён |
| Зависимость от BMad | **Полная** — epics обязательны |
| LOC нового кода | ~1200 + ~2000 тестов |
| Stories | 6-8 |
| Дни | 5-7 |
| Качество декомпозиции | **4/10** (худший результат) |
| Backward compatibility | **Нет** |

**Плюсы:**
- Полный контекст связей между stories

**Минусы:**
- **Context window overflow:** epic (200 строк) + architecture + codebase = 500+ строк метаданных
- Потеря Dev Notes (они в stories, stories удалены)
- Декомпозиция 1 epic = 10-30 tasks — на порядок сложнее для LLM
- Полный редизайн workflow, максимальный объём работы
- **Худший вариант по всем критериям**

#### Вариант D: ralph plan — самодостаточная команда

**Суть:** Новая команда `ralph plan` принимает любой текстовый input и генерирует sprint-tasks.md. BMad опционален.

| Аспект | Значение |
|--------|----------|
| Архитектура | `planner/` новый пакет, `bridge/` deprecated, `runner/` без изменений |
| Входные данные | **Любой markdown/text** (PRD, Issues, plain text, BMad stories) |
| Выходные данные | `sprint-tasks.md` (тот же формат) |
| Зависимость от BMad | **Нулевая** — BMad опционален |
| Зависимость от LLM | Частичная (для сложных FR, простые — программно) |
| LOC нового кода | ~120 Go + ~150 промпт + ~1250 тестов |
| LOC удаления (Phase 3) | -2844 (bridge/ целиком) |
| Нетто-эффект | **-1324 строки** (упрощение codebase) |
| Stories | 3-5 |
| Дни | 2-3 |
| Качество декомпозиции | 7/10 |
| Time-to-first-task (без BMad) | **11-33 мин** (в 3-5x быстрее текущего) |
| Стоимость planning | $0.10-0.30 (90-97% экономия) |
| Гибкость | **9/10** |
| Backward compatibility | **Полная** |

**Плюсы:**
- Единственный самодостаточный вариант
- Минимальный риск при максимальном эффекте
- ADaPT-совместимость (as-needed decomposition)
- Нетто-упрощение codebase (-1324 строки после удаления bridge)
- Полная backward compatibility (runner не трогается)
- Максимальный потенциал community adoption

**Минусы:**
- LLM недетерминизм (менее критично — промпт на 40% короче bridge.md)
- Качество зависит от input (plain text "добавь auth" < формализованный PRD)
- Нет BMad-уровня валидации

### 1.2 Сводная таблица сравнения

```
Критерий               Вес    A      B      C      D
────────────────────────────────────────────────────────
Качество декомпозиции   20%    6      8*     4      7
Самодостаточность       15%    1      1      1     10*
Time-to-first-task      15%    3      3      3      8*
Объём работы (обратный) 10%    9*     5      3      8
Backward compat         10%   10*     2      1     10*
Гибкость форматов       10%    1      1      1      9*
Риск (обратный)         10%    9*     6      3      9*
Стоимость (обратная)     5%    7      6      7      9*
Adoption потенциал       5%    2      2      2      9*
────────────────────────────────────────────────────────
ИТОГО (взвешенный)            4.85   3.95   2.45   8.50*
```

**Вариант D побеждает с отрывом 3.65 балла** от ближайшего конкурента (A).

### 1.3 Почему выбран вариант D

5 ключевых аргументов:

1. **Конкурентный паритет.** Из 11 рассмотренных инструментов (Devin, Claude Code, Aider, SWE-Agent, OpenHands, AutoCodeRover, Cursor, Windsurf, MetaGPT, Kiro, Gastown) **ни один** не требует внешнего workflow для создания входных данных. ralph с BMad-зависимостью — аномалия рынка.

2. **Минимальный риск при максимальном эффекте.** `planner/` — новый пакет параллельно `bridge/`. Runner не трогается. 0 тестов ломается. Bridge deprecated, не удалён немедленно.

3. **ADaPT-совместимость.** As-needed decomposition на 28-33% эффективнее upfront decomposition (Allen AI, NAACL 2024). Bridge делает upfront (все AC -> все tasks). Plan может использовать ADaPT: простые FR — программно, сложные — LLM.

4. **Нетто-упрощение.** После Phase 3: -1324 строки кода.

5. **Backward compatibility.** Единственный вариант (кроме A), полностью backward-compatible. Миграция = замена одного слова: `ralph bridge story.md` -> `ralph plan story.md`.

### 1.4 Возможная комбинация D + элементы A

Для BMad stories — программный парсинг AC без LLM (0 cost). Для plain text/PRD — LLM-декомпозиция. Это ADaPT-подход: программный парсинг для structured input, LLM для unstructured.

---

## 2. Как устроены конкуренты

**Источник:** `v2-agent-orchestrators.md`

### 2.1 Обзор 11 инструментов

| Инструмент | Входные данные | Persistent task file | Single/Multi-agent | Upfront/Incremental |
|---|---|---|---|---|
| **Devin** | Plain text / Slack / Issue | notes.txt + knowledge entries | Multi-agent (2.0) | Upfront + итеративный |
| **SWE-Agent** | GitHub issue | Нет | Single-agent | Incremental (ReAct) |
| **OpenHands** | Issue / plain text | Event stream (replay-capable) | Multi-agent (AgentHub) | Incremental + делегирование |
| **AutoCodeRover** | GitHub issue | Нет | Single-agent | 2-фазный (context + patch) |
| **MetaGPT** | Однострочное требование | Структурированные артефакты по SOP | Multi-agent (5+ ролей) | Upfront (SOP pipeline) |
| **Gastown** | Задача от Mayor | Beads (git-backed issues) | Multi-agent (20-30 параллельных) | Upfront + Mayor |
| **Claude Code** | Plain text / CLAUDE.md | In-context tasks (не persistent) | Single + sub-agents | Гибридный |
| **Cursor** | Natural language | Diff preview | Multi-agent (до 8) | Upfront plan |
| **Windsurf** | Natural language | Semantic model | Single-agent | Upfront plan mode |
| **Copilot Workspace** | GitHub issue / prompt | Specification + Plan (editable) | Single-agent | Upfront 3-phase |
| **Kiro** | Natural language | **tasks.md** (persistent, trackable) | Single-agent + hooks | Upfront 3-phase |

### 2.2 Ключевые паттерны (что работает)

#### Паттерн 1: Spec-first pipeline (Copilot Workspace, Kiro, MetaGPT)

Явная спецификация "что есть" vs "что нужно" перед планированием. Промежуточные документы снижают ambiguity. Пользователь может steering на каждом этапе.

- **Copilot Workspace:** Specification (current vs desired state) -> Plan (files to modify) -> Implementation
- **Kiro:** requirements.md -> design.md -> tasks.md (3-фазный workflow с EARS нотацией)
- **MetaGPT:** PRD -> User Stories -> Design -> Implementation (SOP pipeline с 5+ ролями)

**Применимость к ralph:** `ralph plan` реализует сжатую версию — requirements -> tasks. Промежуточная spec-фаза возможна через `ralph init --interactive`.

#### Паттерн 2: Git-backed state (Gastown, Kiro)

Состояние задач в Git, а не in-memory. Version control для task state даёт rollback. Естественная интеграция с development workflow.

- **Gastown Beads:** ID (prefix + 5-char alphanumeric), описание, статус, assignee — всё в Git
- **Kiro tasks.md:** Markdown с checkbox'ами, обновляемый в real-time

**Применимость к ralph:** `sprint-tasks.yaml` (YAML формат) — git-tracked, version-controlled. Уже реализован аналогичный паттерн через `sprint-tasks.md`.

#### Паттерн 3: ACI-optimized tools (SWE-Agent, Princeton)

Дизайн интерфейса агента важнее сложности планирования:
- Linter-gates на edit commands предотвращают синтаксические ошибки
- Feedback format оптимизирован для LLM
- Результат: 12.29% на SWE-bench, $0.43 средняя стоимость решения

**Применимость к ralph:** Оптимизация промптов и feedback'ов для Claude. Linter-gates перед отправкой кода.

#### Паттерн 4: Role-based SOP (MetaGPT, Gastown)

Фиксированные роли с определёнными ответственностями. Стандарты для промежуточных выходов. Каждая роль работает только в своей зоне.

- **MetaGPT:** Product Manager, Architect, Project Manager, Engineer
- **Gastown:** Mayor (orchestrator), Polecats (workers), Witness+Deacon (мониторинг), Refinery (merge)

**Применимость к ralph:** BMad team workflow (creator/validator/developer/reviewer) — уже реализован через manual orchestration. Автоматизация role dispatch — следующий шаг.

#### Паттерн 5: Editable intermediate artifacts (Copilot Workspace, Devin, Kiro)

Все промежуточные документы редактируемые. Двунаправленная коммуникация human<->agent. Kiro: двунаправленная синхронизация spec<->code.

#### Паттерн 6: Persistent knowledge (Devin, Kiro)

Knowledge entries переживают сессии. "This repo uses Tailwind", "API responds with XML". ralph уже имеет knowledge management (Epic 6), но можно улучшить targeted entries.

#### Паттерн 7: As-needed decomposition (ADaPT, Claude Code)

**ADaPT (Allen AI, NAACL 2024):**
- Separate Planner и Executor модули
- Executor пробует выполнить задачу
- При failure — Planner рекурсивно декомпозирует failed sub-task
- Результаты: +28.3% в ALFWorld, +27% в WebShop, +33% в TextCraft

**Применимость к ralph:** ADaPT совместим с retry-механизмом ralph. При failure — декомпозиция текущей задачи на sub-tasks.

### 2.3 Антипаттерны (что не работает)

| Антипаттерн | Пример | Проблема |
|---|---|---|
| Overplanning | Детальный upfront план для single-issue fix | SWE-Agent показывает: для простых задач план не нужен |
| Планы без steering points | Нет возможности вмешаться между spec и impl | Ошибки amplify'ятся |
| In-memory-only state | Claude Code без persistent tasks | Потеря контекста при перезапуске |
| Trust without validation | Слепое доверие агентам | Gastown: validation gates вместо доверия |
| Monolithic agent для сложных задач | Один агент для всего | Не масштабируется (MetaGPT, Gastown) |
| Fidelity loss между ролями | Неструктурированные промежуточные артефакты | Информация теряется при передаче |

### 2.4 Что ralph может заимствовать

**Высокий приоритет:**

1. **Persistent task file (Kiro pattern)** — runtime task tracking. `sprint-tasks.yaml` обновляется в real-time. Уже реализован аналог через markdown.

2. **Spec-first workflow (Copilot Workspace)** — "current state -> desired state" для каждой задачи. `ralph plan` генерирует мини-spec в task description.

3. **As-needed decomposition (ADaPT)** — пробовать выполнить task целиком, при failure декомпозировать на sub-tasks. Совместимо с retry-механизмом.

**Средний приоритет:**

4. **Knowledge entries (Devin)** — targeted entries из review results
5. **Git-backed task state (Gastown)** — version-controlled task tracking (уже есть)
6. **ACI improvements (SWE-Agent)** — linter-gates перед Claude

**Низкий приоритет:**

7. **Multi-agent с ролями (MetaGPT/Gastown)** — автоматизация role dispatch
8. **Bi-directional spec sync (Kiro)** — обновление specs на основе кода

### 2.5 Ключевые тренды 2025-2026

1. **Тренд:** Движение от single-agent ReAct loops к structured multi-phase pipelines с persistent state
2. **Spec-driven > prompt-driven:** Все наиболее успешные инструменты создают explicit промежуточные документы
3. **Hybrid decomposition побеждает:** Ни чистый upfront, ни чистый incremental не оптимальны
4. **Persistent state критичен:** Без persistent state масштабирование невозможно
5. **Steering points обязательны:** Copilot Workspace с двумя explicit steering points — золотой стандарт

---

## 3. BMad v6 — что ценно, что избыточно

**Источник:** `v2-bmad-document-formats.md`

### 3.1 Каталог документов BMad v6

Полный pipeline BMad v6 генерирует **12 типов документов** в 4 фазах:

| # | Документ | Файл | Строк | Фаза |
|---|----------|------|-------|------|
| 1 | Brainstorming Session | `docs/analysis/brainstorming-*.md` | ~120 | 0 - Discovery |
| 2 | Product Brief | `docs/product-brief.md` | ~200-400 | 0 - Discovery |
| 3 | Research | `docs/research-*.md` | варьируется | 0 - Discovery |
| 4 | **PRD** | `docs/prd.md` | **~420** | 1 - Planning |
| 5 | **UX Design Spec** | `docs/ux-design-specification.md` | **~1530** | 1 - Planning |
| 6 | **Architecture** | `docs/architecture.md` | **~1760** | 2 - Solutioning |
| 7 | **Epics & Stories** | `docs/epics.md` | **~2370** | 2 - Solutioning |
| 8 | Impl. Readiness Report | `docs/implementation-readiness-*.md` | ~230 | 2 - Solutioning |
| 9 | BMM Workflow Status | `docs/bmm-workflow-status.yaml` | ~120 | мета |
| 10 | **Sprint Status** | `docs/sprint-artifacts/sprint-status.yaml` | **~105** | 3 - Implementation |
| 11 | **Story файлы** | `docs/sprint-artifacts/{N}-{M}-*.md` | **~125-235** | 3 - Implementation |
| 12 | Validation Reports | `docs/sprint-artifacts/validation-report-*.md` | ~190 | 3 - Implementation |

**Жирным** — документы, реально используемые в pipeline разработки.

### 3.2 Что runner РЕАЛЬНО использует

Из анализа Go-кода (`config/config.go`, `runner/scan.go`, `bridge/bridge.go`):

**Используется runner'ом (2 типа из 12):**

| Документ | Как используется |
|----------|-----------------|
| **Sprint Status** (`sprint-status.yaml`) | Определение статуса stories, сканирование `StoriesDir` |
| **Story файлы** (`{N}-{M}-*.md`) | Контекст для Claude Code сессий |

**НЕ используется runner'ом (10 типов из 12):**

- PRD — только BMad workflow'ы
- UX Design — только BMad workflow'ы
- Architecture — только BMad workflow'ы
- Epics — только BMad workflow'ы
- Implementation Readiness — только BMad workflow'ы
- BMM Workflow Status — только BMad workflow'ы
- Validation Reports — фильтруются как non-story
- Brainstorming — только BMad workflow'ы
- Product Brief — только BMad workflow'ы
- Research — только BMad workflow'ы

**Вывод:** Ralph runner работает ТОЛЬКО с 2 типами файлов. Все остальные 10 используются исключительно BMad workflow'ами для ГЕНЕРАЦИИ story файлов.

### 3.3 Количественный анализ

| Метрика | Значение |
|---------|----------|
| Общий объём docs/ | ~6530 строк |
| PRD | 418 строк |
| Architecture | 1764 строки |
| UX Design | 1532 строки |
| Epics | 2365 строк (самый большой) |
| Story файлов | 31 штука |
| Средний размер story | ~170 строк |
| Общий объём stories | ~5300 строк |
| **ИТОГО документации** | **~13500 строк** |
| **Код проекта** | 0 строк (все stories в ready-for-dev) |

### 3.4 Коэффициент расширения

```
Epics.md:     ~15 строк на story (краткие AC + Technical Notes)
Story файл:   ~170 строк (полные Tasks/Subtasks + Dev Notes + References)
Коэффициент расширения: ~11x
```

Это означает, что create-story workflow добавляет ~155 строк контекста на каждую story. Основной объём — Tasks/Subtasks и Dev Notes.

### 3.5 Граф зависимостей документов

```
Phase 0: brainstorm-session
              ↓
Phase 1: PRD ←──── UX Design
              ↓         ↓
Phase 2: Architecture ──┘
              ↓
         Epics & Stories ←── PRD + Architecture + UX
              ↓
         Implementation Readiness ←── все 4 документа
              ↓
Phase 3: Sprint Status ←── Epics
              ↓
         Story файлы ←── Epics + Architecture + UX + PRD
```

### 3.6 Что ценно vs что избыточно для ralph v2

**Ценно (встраивать в ralph):**

| Аспект | Сложность | Обоснование |
|--------|-----------|-------------|
| Sprint Status YAML | Низкая | Простой YAML с key:status, уже используется |
| Story формат (markdown) | Низкая | Простой markdown без frontmatter, фиксированные секции |
| Формат FR нумерации | Низкая | FR1-FRN, сквозная нумерация по группам |
| Группировка FR по доменам | Низкая | Auth, API, UI — естественная декомпозиция |

**Избыточно (НЕ встраивать):**

| Аспект | Причина |
|--------|---------|
| PRD генерация (11 шагов) | Интерактивный workflow с Party Mode review |
| Architecture (8 шагов) | ADR + Party Mode validation |
| UX Design (14 шагов) | Самый сложный workflow, inline HTML wireframes |
| Epics генерация | Зависит от PRD+Architecture+UX |
| Validation Reports | Побочный продукт validate-create-story |
| BMM Workflow Status | Мета-tracking для BMad Method |
| YAML frontmatter документов | stepsCompleted, inputDocuments — только для BMad |

**Единственный кандидат на встраивание:** Create-story workflow — берёт данные из epics.md + architecture.md и генерирует обогащённый story файл. Можно реализовать как промпт для Claude Code (упрощённый).

### 3.7 Минимальный формат story для ralph

```markdown
# Story {N}.{M}: {Title}

Status: {ready-for-dev|in-progress|review|done}

## Story
As a {role}, I want {action}, So that {benefit}.

## Acceptance Criteria (BDD)
**AC-1: {название}**
**Given** {предусловие}
**When** {действие}
**Then** {ожидаемый результат}

## Tasks / Subtasks
### Task 1: {заголовок} (AC: {ссылка})
1.1. {подзадача}

## Dev Notes
- {архитектурные паттерны}
- {технические ограничения}
```

Этот формат легко воспроизводим без BMad. Story файл — единственный "мост" между BMad документацией и ralph runner.

---

## 4. Архитектура ralph plan

**Источник:** `v2-ralph-plan-architecture.md`

### 4.1 Обзор

Команда `ralph plan` заменяет связку "BMad stories + ralph bridge" одним шагом:

```
БЫЛО:
  BMad AI (PRD -> Arch -> Epics -> Stories)
    -> ralph bridge (Stories -> sprint-tasks.md)
      -> ralph run (Tasks -> Code)

СТАЛО:
  ralph plan (PRD + Arch -> sprint-tasks.md)
    -> ralph run (Tasks -> Code)
```

### 4.2 Граф зависимостей пакетов

```
                     cmd/ralph/
                    /    |     \
                   /     |      \
            planner/  runner/  bridge/ (deprecated)
              |    \    |   \     |
              |     \   |    \    |
           session  config  gates session

Направление зависимостей (строго top-down):

  cmd/ralph
    -> planner   (NEW)
    -> runner    (без изменений)
    -> bridge    (deprecated)
    -> config    (leaf)

  planner
    -> session   (вызов Claude CLI)
    -> config    (Config, AssemblePrompt, constants)

  planner НЕ зависит от:
    -> runner    (параллельный пакет)
    -> bridge    (deprecated)
    -> gates     (planner не нуждается в gate prompt)
```

Архитектурное правило проекта (`cmd/ralph → runner → session, gates, config`) соблюдено: `planner` — параллельный пакет на том же уровне, что и `runner`.

### 4.3 Структура пакета planner/

```
planner/
  planner.go           // Plan() — основная точка входа
  discover.go          // DiscoverDocs() — автодискавери документов
  context.go           // CollectContext() — сбор codebase context
  format.go            // FormatTasks(), ParseLLMOutput() — JSON -> sprint-tasks.md
  merge.go             // MergeTasks() — детерминистический merge
  prompts/
    plan.md            // Go template — промпт планировщика (embed)
  planner_test.go
  discover_test.go
  format_test.go
  merge_test.go
```

### 4.4 Ключевые типы данных

```go
// PlanOptions — входные параметры
type PlanOptions struct {
    PRDFiles  []string // --prd flag(s), или автодискавери
    ArchFiles []string // --arch flag(s), или автодискавери
    UXFiles   []string // --ux flag(s), или автодискавери
    Merge     bool     // объединить с existing sprint-tasks.md
    DryRun    bool     // не записывать файл
    Output    string   // путь к output файлу
}

// LLMPlanOutput — структура JSON output от LLM
type LLMPlanOutput struct {
    Analysis string     `json:"analysis"`
    Epics    []LLMEpic  `json:"epics"`
}

// PlanResult — результат работы Plan()
type PlanResult struct {
    TaskCount   int
    EpicCount   int
    OutputPath  string
    Duration    time.Duration
    Analysis    string // краткий анализ от LLM
}
```

### 4.5 Основная логика Plan()

```
Plan(ctx, cfg, opts) выполняет 7 шагов:

1. Автодискавери документов (если не указаны явно)
2. Сбор контекста проекта (Go, программно):
   - go.mod / package.json (tech stack)
   - Структура каталогов (max 3 уровня)
   - Existing sprint-tasks.md (для merge mode)
   - CLAUDE.md (правила проекта)
3. Сборка промпта через config.AssemblePrompt()
4. Вызов Claude CLI через session (LLM)
5. Парсинг JSON output, валидация (Go, программно)
6. Форматирование в sprint-tasks.md (Go, программно)
7. Merge с existing tasks (Go, программно) + запись файла
```

### 4.6 Автодискавери документов

Стратегия классификации по 3 приоритетам:

```
Приоритет 1: Имя файла (regex match, score: 0.9)
  prd*.md, requirements*.md         -> DocPRD
  arch*.md, *-architecture.md       -> DocArchitecture
  ux*.md, ui-spec*.md               -> DocUX
  epic*.md, *-epics.md              -> DocEpics

Приоритет 2: Имя каталога (score: 0.8)
  docs/prd/*, docs/requirements/*   -> DocPRD
  docs/architecture/*, docs/design/ -> DocArchitecture

Приоритет 3: Содержимое первых 1000 байт (score: 0.6)
  "functional requirement", "FR-"   -> DocPRD
  "architecture", "system design"   -> DocArchitecture
  "wireframe", "mockup"             -> DocUX

  Score < 0.5 -> DocUnknown (файл игнорируется)
```

Конфигурация через `ralph.yaml`:
```yaml
plan:
  prd: ["docs/prd/feature-x.md"]
  architecture: ["docs/architecture/system.md"]
  output: sprint-tasks.md
  model: claude-sonnet-4-20250514
  max_turns: 5
```

### 4.7 CLI команда

```
ralph plan [requirement-files...]

Флаги:
  --prd           []string  PRD файл(ы)
  --arch          []string  Architecture файл(ы)
  --ux            []string  UX спецификации
  --from-stories  []string  Story файлы (backward compat)
  -o, --output    string    Output файл (default: sprint-tasks.md)
  --dry-run       bool      Показать без записи
  --no-merge      bool      Перезаписать вместо merge
  --model         string    Модель для plan сессии
  --max-turns     int       Максимум turns для Claude
```

### 4.8 Разделение ответственностей Go vs LLM

| Ответственность | Кто делает | Как |
|-----------------|-----------|-----|
| Автодискавери документов | Go | Regex по имени, содержимому |
| Сбор контекста (tree, go.mod) | Go | Программное чтение файлов |
| Сборка промпта | Go | config.AssemblePrompt() |
| Декомпозиция требований в задачи | **LLM** | JSON output |
| Парсинг JSON output | Go | json.Unmarshal + валидация |
| Форматирование sprint-tasks.md | Go | Программный формат |
| Source traceability (source: lines) | Go | Из requirement_refs + имя файла |
| Gate marking | Go | Первый в epic + keyword scan |
| Merge с existing tasks | Go | Детерминистический diff по title+refs |
| Topological sort | Go | Из depends_on |

**Ключевой принцип:** "Blueprint First, Model Second" — Go = детерминистический scaffold, LLM = семантический анализ. Исследование показывает +10.1 п.п. на tau-bench, -81.8% tool calls.

### 4.9 Реализация в 9 stories (3 фазы)

**Phase 1 (Stories 1-5): Core planner** — 5 stories, ~3 дня

| Story | Scope | LOC (оценка) |
|-------|-------|---|
| 1 | PlanOptions + PlanResult structs, PlanConfig в config | ~100 |
| 2 | DiscoverDocs() + ClassifyDoc() | ~200 |
| 3 | CollectContext() + BuildDirTree() | ~150 |
| 4 | ParseLLMOutput() + FormatTasks() + FormatTaskLine() | ~200 |
| 5 | Plan() orchestrator + plan.md prompt + CLI command | ~200 |

**Phase 2 (Stories 6-7): Merge + backward compat** — 2 stories, ~1 день

| Story | Scope |
|-------|-------|
| 6 | MergeTasks() — детерминистический merge |
| 7 | `--from-stories` backward compat (программный парсинг BMad stories) |

**Phase 3 (Stories 8-9): Bridge deprecation** — 2 stories, ~1 день

| Story | Scope |
|-------|-------|
| 8 | `ralph bridge` deprecated с warning |
| 9 | `bridge/` удалён (-2844 LOC) |

---

## 5. ralph init — быстрый старт

**Источник:** `v2-ralph-init-design.md`

### 5.1 Проблема

Ralph сейчас требует 5 шагов до первого коммита:
1. Установить BMad Method
2. 4 workflow (PRD, Architecture, Epics, Stories) — 45-100 мин
3. `ralph bridge` — 10-30 мин
4. Получить `sprint-tasks.md`
5. `ralph run`

**Time-to-first-task: 55-130 минут.** Конкуренты: 1-5 минут.

### 5.2 Позиционирование на спектре инструментов

```
Минимум структуры                                    Максимум структуры
     |                                                        |
  Cursor    Aider    Claude Code    Devin    ralph init    ralph+BMad
  "делай"   "делай"  "CLAUDE.md"   "Issue"  "требования"  "PRD->Stories"
  1 мин     1 мин    1-3 мин       2-5 мин  2-5 мин       55-130 мин
```

**Ниша ralph:** между Devin и полным BMad. Больше структуры, чем "просто делай" (review, gates, knowledge), меньше церемонии, чем BMad.

**Ключевой инсайт:** ralph init не конкурирует с "напиши функцию" (территория Cursor/Aider). ralph init для проектов, где нужно 10-100+ задач с code review и quality control.

### 5.3 Три flow

#### Flow 1: Минимальный (one-liner)

```bash
ralph init "Платформа для обучения Java с sandbox, JWT auth, Monaco editor"
```

- Создаёт `docs/requirements.md` через один LLM-вызов
- Стоимость: ~$0.10-0.30
- Время: 30-90 секунд
- Выход: "Создан docs/requirements.md. Проверьте, затем: ralph plan"

#### Flow 2: Интерактивный

```bash
ralph init --interactive
```

- 5-7 фиксированных вопросов (НЕ через LLM)
- Генерирует `docs/prd.md` + `docs/architecture.md` (раздельно)
- Стоимость: ~$0.30-0.80
- Время: 2-5 минут (включая ввод ответов)

Вопросы:
```
Проект: ___
Опиши проект в 1-3 предложениях: ___
Tech stack (языки, фреймворки): ___
Основные фичи (через запятую): ___
Есть ли внешние зависимости (DB, API, очереди)? ___
Масштаб (сколько пользователей, данных)? ___
Особые требования (безопасность, performance, compliance)? ___
```

#### Flow 3: Brownfield (существующий проект)

```bash
ralph init --scan
```

- Сканирует проект программно (без LLM):
  - `go.mod` / `package.json` / `Cargo.toml` — стек
  - `Dockerfile` / `docker-compose.yml` — инфраструктура
  - `README.md` (первые 50 строк)
  - Структура директорий (глубина 2)
  - `CLAUDE.md` / `.cursor/` — существующие AI-конфиги
- Генерирует `docs/project-context.md`
- Стоимость: ~$0.10-0.30
- Время: 30-90 секунд

### 5.4 Формат requirements.md vs BMad PRD

| Аспект | requirements.md | BMad PRD |
|--------|----------------|----------|
| **Объём** | 30-80 строк | 200-500 строк |
| User Journeys | Нет | 3-5 детальных |
| Success Criteria | Нет (или 2-3 строки) | Секция с KPI |
| Growth/Vision | Нет | Да |
| FR нумерация | Свободная (FR1-FRN) | Структурированная по группам |
| NFR | 2-5 пунктов | 10-15 с таблицей |
| Scope | MVP only | MVP + Growth + Vision |
| UI/UX | Нет | Отдельный документ |

**Ключевое различие:** requirements.md — "достаточно для начала работы". BMad PRD — "полная спецификация для передачи команде".

### 5.5 Полные pipeline'ы

**Greenfield минимальный (2-5 мин):**
```bash
mkdir my-project && cd my-project && git init
ralph init "REST API для задач на Go с PostgreSQL и JWT"
# → docs/requirements.md (30 сек)
# Проверка/правка (1-2 мин)
ralph plan
# → sprint-tasks.yaml (30 сек)
ralph run
```

**Greenfield интерактивный (4-7 мин):**
```bash
ralph init --interactive  # 5-7 вопросов (2 мин) → prd.md + architecture.md
ralph plan                # → sprint-tasks.yaml
ralph run
```

**Brownfield (3-5 мин):**
```bash
cd existing-project
ralph init --scan         # → project-context.md (30 сек)
# Добавить требования (2-3 мин)
ralph plan docs/project-context.md
ralph run
```

**BMad-совместимый:**
```bash
ralph plan --stories docs/sprint-artifacts/  # Программный парсинг (без LLM!)
ralph run
```

**Прямой запуск (без init/plan):**
```bash
ralph run "Добавь JWT авторизацию с refresh-токенами"
# → ralph plan автоматически → sprint-tasks.yaml → выполнение
```

### 5.6 Оценка реализации

| Компонент | LOC (оценка) | Сложность |
|-----------|--------------|-----------|
| `cmd/ralph/init.go` | 80-120 | Низкая |
| `cmd/ralph/plan.go` | 100-150 | Низкая |
| `planner/scanner.go` | 150-250 | Средняя |
| `planner/questions.go` | 80-120 | Низкая |
| `planner/tasks.go` | 100-150 | Средняя |
| `planner/prompt.go` | 50-80 | Низкая |
| `planner/planner.go` | 100-150 | Средняя |
| Промпты (init + plan) | 60-100 | Средняя |
| Миграция runner на YAML | 200-300 | Средняя |
| Тесты | 500-800 | Средняя |
| **ИТОГО** | **1400-2200** | |

**Новых внешних зависимостей: 0** (yaml.v3 и cobra уже есть).

---

## 6. ralph replan — коррекция курса

**Источник:** `v2-replan-correct-course.md`

### 6.1 Три уровня коррекции

```
Уровень 1: Тактический (retry)    — УЖЕ РЕАЛИЗОВАН
  Scope: одна текущая задача
  Механизм: [r] → InjectFeedback() → RevertTask() → повтор
  Примеры: "добавь обработку ошибок", "используй другой подход"

Уровень 2: Структурный (replan)   — НОВЫЙ [c]
  Scope: sprint-tasks.md целиком
  Механизм: [c] → описание → Claude replan → diff → подтверждение
  Примеры: "убери basic auth, добавь OAuth", "сначала API, потом UI"

Уровень 3: Стратегический (reinit) — ОТЛОЖЕН (v3+)
  Scope: requirements + sprint-tasks.md
  Механизм: пересоздание requirements → новый sprint plan
  Примеры: полная смена стека
```

### 6.2 Сравнение с BMad correct-course

BMad correct-course — 6-шаговый тяжеловесный процесс для межкомандной координации (PM, PO, SM, архитектор):
1. Инициализация — сбор контекста, загрузка документов
2. Чеклист из 20 пунктов — Impact Analysis по 6 секциям
3. Change Proposals (old -> new для каждого артефакта)
4. Sprint Change Proposal документ (5 секций)
5. Финализация и роутинг (Minor/Moderate/Major)
6. Handoff и completion

**Что применимо к ralph CLI:**

| BMad элемент | Применимость | Обоснование |
|---|---|---|
| Сбор описания проблемы | Да | Одна строка ввода |
| Классификация scope | Да, упрощённо | 3 уровня |
| Diff-показ изменений | Да | Критично для UX |
| Подтверждение/отклонение | Да | Rollback если отклонено |
| Impact Analysis (20 пунктов) | **Нет** | Overkill для CLI |
| Sprint Change Proposal документ | **Нет** | Некому читать в solo-dev CLI |
| Agent routing по scope | **Нет** | ralph — один агент |
| PRD/Architecture review | **Нет** | ralph не модифицирует эти файлы |

**Вывод:** из 6 шагов BMad correct-course нужна суть шагов 1 и 3 — сбор описания и показ конкретных изменений.

### 6.3 Gate action [c] Correct Course

**Новое меню обычного gate:**
```
🚦 HUMAN GATE: - [ ] Implement user login [GATE]
   [a]pprove  [r]etry with feedback  [c]orrect course  [s]kip  [q]uit
>
```

При выборе `[c]`:
```
Опишите изменения в плане (пустая строка = отправить):
> Убрать задачи по basic auth, добавить OAuth flow
> Добавить задачу на refresh tokens
>
```

### 6.4 Replan flow

```go
type ReplanFunc func(ctx context.Context, opts ReplanOpts) (*ReplanResult, error)

type ReplanOpts struct {
    TasksFile   string   // путь к sprint-tasks.md
    Feedback    string   // описание от пользователя
    ProjectRoot string
    ClaudeCmd   string
}

type ReplanResult struct {
    OriginalContent string
    NewContent      string
    Added           []string   // новые задачи
    Removed         []string   // убранные задачи
    Modified        []string   // изменённые задачи
    Preserved       []string   // сохранённые [x] задачи
}
```

**Алгоритм:**
1. Прочитать текущий sprint-tasks.md
2. Извлечь выполненные [x] задачи — **НЕПРИКОСНОВЕННЫ**
3. Сформировать промпт (текущие задачи + описание изменений + инструкция)
4. Вызвать Claude через session.RunClaude
5. Распарсить ответ -> новый sprint-tasks.md
6. Вычислить diff (Added/Removed/Modified)
7. Вернуть ReplanResult

### 6.5 Diff display

```
📋 Plan changes:
  ✚ Added (3):
    + - [ ] Implement OAuth2 authorization flow
    + - [ ] Add refresh token rotation
    + - [ ] Add OAuth scopes configuration

  ✖ Removed (2):
    - - [ ] Implement basic auth middleware
    - - [ ] Add password hashing

  ≡ Modified (1):
    ~ - [ ] Add login endpoint → - [ ] Add OAuth login endpoint

  ✓ Preserved (5 completed tasks unchanged)

Apply changes? [a]pply  [e]dit description  [c]ancel
>
```

**Три действия:**
- **[a]pply** — записать новый sprint-tasks.md, продолжить execute loop
- **[e]dit** — повторить ввод описания, re-run Claude
- **[c]ancel** — sprint-tasks.md не изменён, вернуться в gate prompt

### 6.6 Защита [x] задач

**Валидация (post-processing):**
```go
func ValidateReplan(original, replanned string) error
// Все [x] из оригинала должны присутствовать в replanned
// При потере — ошибка, не применяем
```

**Fallback (принудительная инъекция):**
```go
func ForcePreserveDoneTasks(original, replanned string) string
// Убрать из replanned любые [x] (Claude мог испортить)
// Вставить оригинальные [x] в начало (грубо, но гарантирует)
```

**Рекомендация:** ValidateReplan() как guard. Если провалился — сообщить пользователю и предложить повторить. ForcePreserveDoneTasks() — крайний fallback.

### 6.7 Boundary conditions

| Edge case | Поведение |
|---|---|
| Все задачи выполнены | Replan добавляет новые `[ ]` задачи |
| Claude возвращает невалидный markdown | ValidateReplan() + ScanTasks() проверка |
| Пользователь отменяет несколько раз | Cancel возвращает в gate prompt, зацикливание невозможно |
| Budget exceeded | [c] недоступен в emergency gate |
| Стоимость replan | Входит в общий бюджет |

### 6.8 Оценка работ

| Компонент | Оценка |
|---|---|
| `config/constants.go` + test | Тривиально |
| `gates/gates.go` + test | ~60 строк |
| `runner/replan.go` + test | ~550 строк |
| `runner/prompts/replan.md` + prompt test | ~60 строк |
| `runner/runner.go` + test (case ActionCorrectCourse) | ~110 строк |
| `runner/metrics.go` (Replans counter) | ~5 строк |
| **ИТОГО** | **~800 строк, 3-4 story, ~2 дня** |

---

## 7. YAML формат задач

**Источник:** `v2-yaml-task-format.md`

### 7.1 Мотивация перехода с markdown

3 фундаментальные проблемы текущего `sprint-tasks.md`:

1. **Хрупкий regex-парсинг** — 4 regex'а в `config/constants.go` (TaskOpenRegex, TaskDoneRegex, GateTagRegex, SourceFieldRegex), ломаются от `- []` (без пробела), `* [ ]`, `[X]` (заглавная)

2. **LLM merge mode** — 244-строчный промпт bridge.md, 12 правил merge, LLM теряет `[x]` статусы (~85% надёжность)

3. **Бедные метаданные** — нет ID, зависимостей, оценки сложности, подсказок по файлам

### 7.2 YAML Schema (sprint-tasks.yaml)

```yaml
version: 1
generated: "2026-03-07T12:00:00Z"

source_docs:
  - path: "docs/stories/1-1-auth.md"
    hash: "abc123def"

epics:
  - name: "Authentication & Security"
    tasks:
      - id: 1
        description: "Implement password hashing with bcrypt"
        status: done           # open | done | skipped
        gate: true
        tags: []               # SETUP | E2E
        source: "1-1-auth.md#AC-1,AC-2"
        depends_on: []         # список id задач-блокеров
        files_hint: []         # подсказка по файлам
        size: M                # S | M | L
        feedback: ""           # USER FEEDBACK от gate review

      - id: 2
        description: "Add JWT token generation and validation"
        status: open
        gate: false
        tags: []
        source: "1-2-jwt.md#AC-1,AC-2,AC-3"
        depends_on: [1]
        files_hint:
          - "internal/auth/jwt.go"
          - "internal/auth/jwt_test.go"
        size: L
        feedback: ""
```

### 7.3 Go Structs

```go
// SprintTasks — корневая структура sprint-tasks.yaml
type SprintTasks struct {
    Version    int         `yaml:"version"`
    Generated  time.Time   `yaml:"generated"`
    SourceDocs []SourceDoc `yaml:"source_docs,omitempty"`
    Epics      []Epic      `yaml:"epics"`
}

// Task — единица работы в спринте
type Task struct {
    ID          int      `yaml:"id"`
    Description string   `yaml:"description"`
    Status      string   `yaml:"status"`      // open | done | skipped
    Gate        bool     `yaml:"gate"`
    Tags        []string `yaml:"tags"`
    Source      string   `yaml:"source"`
    DependsOn   []int    `yaml:"depends_on"`
    FilesHint   []string `yaml:"files_hint,omitempty"`
    Size        string   `yaml:"size,omitempty"`    // S | M | L
    Feedback    string   `yaml:"feedback,omitempty"`
}

// IsBlocked — зависимости не выполнены
func (t Task) IsBlocked(index map[int]*Task) bool {
    for _, depID := range t.DependsOn {
        dep, ok := index[depID]
        if !ok || !dep.IsDone() {
            return true
        }
    }
    return false
}
```

### 7.4 Правила схемы

| Поле | Тип | Обязательно | Ограничения |
|------|-----|-------------|-------------|
| `version` | int | да | Всегда `1` |
| `generated` | string | да | ISO 8601 |
| `source_docs` | []SourceDoc | нет | Опционально |
| `epics` | []Epic | да | Минимум 1 |
| `task.id` | int | да | Уникальный, монотонно растёт |
| `task.description` | string | да | 1-500 символов |
| `task.status` | string | да | Enum: open, done, skipped |
| `task.gate` | bool | да | — |
| `task.tags` | []string | да | Enum: SETUP, E2E |
| `task.source` | string | да | Формат: `<filename>#<anchor>` |
| `task.depends_on` | []int | да | Ссылки на id задач |
| `task.size` | string | нет | Enum: S, M, L (default M) |

### 7.5 Почему целочисленные ID

Ранние draft'ы предлагали строковые ID (`auth-1`, `api-2`). Целочисленные лучше:
- **Автоинкремент** — `maxID + 1` при merge, нет коллизий
- **depends_on читаемость** — `[1, 3]` vs `["auth-1", "api-3"]`
- **Нет проблемы именования** — не нужен префикс
- **YAML совместимость** — целые числа не конфликтуют с YAML спецсимволами

### 7.6 Программный Merge

**Ключ дедупликации:** пара `(epicName, source)`.

**Алгоритм MergeTasks:**
1. Построить индексы existing (по ID и по ключу epicName+source)
2. Для каждой incoming задачи:
   - Если дубликат найден И status=done → **сохранить** existing. Если description изменился → создать НОВУЮ open задачу
   - Если дубликат найден И status=open → **обновить** description, не трогать status/gate/feedback
   - Если дубликат не найден → **добавить** с maxID++
3. Переписать depends_on через маппинг incomingID -> mergedID
4. Обновить generated timestamp

**Инварианты:**
- done задачи **НИКОГДА** не меняют status
- done задачи **НИКОГДА** не меняют description
- ID уникальны в пределах файла
- depends_on ссылки корректно переписаны

### 7.7 Сравнение markdown vs YAML

**Парсинг:**

| Аспект | Markdown | YAML |
|--------|----------|------|
| Парсер | 4 regex, 70 строк | yaml.Unmarshal, ~30 строк |
| Edge cases | `- []`, `* [ ]`, `[X]`, табы vs пробелы | Нет |
| Валидация | Нет (regex матчит или нет) | Полная: типы, enum, зависимости, циклы |
| Ошибки | Молча пропускает невалидные строки | Явная ошибка с позицией |

**Merge:**

| Аспект | LLM Merge | Программный Merge |
|--------|-----------|-------------------|
| Надёжность | ~85% (LLM теряет [x]) | **100%** (Go-код, детерминистичный) |
| Скорость | 30-60 сек (Claude API) | **<10 мс** |
| Стоимость | ~$0.01-0.05 за merge | **$0** |
| Diff | Нет (LLM возвращает весь файл) | MergeDiff struct |
| Тестируемость | Невозможно unit-test | Полное покрытие |

**Метаданные:**

| Аспект | Markdown | YAML |
|--------|----------|------|
| ID задачи | Нет (номер строки) | Уникальный int |
| Зависимости | Нет (неявный порядок) | depends_on: [id, ...] |
| Сложность | Нет | size: S/M/L |
| Файлы | Нет | files_hint: [...] |
| Feedback | `> USER FEEDBACK:` blockquote | feedback: "..." поле |

### 7.8 Dual-format миграция

```go
// ScanTasksAuto — автодетект формата
func ScanTasksAuto(projectRoot string) (ScanResult, error) {
    yamlPath := filepath.Join(projectRoot, "sprint-tasks.yaml")
    mdPath := filepath.Join(projectRoot, "sprint-tasks.md")

    // YAML имеет приоритет
    if _, err := os.Stat(yamlPath); err == nil {
        return ScanTasksYAML(yamlPath)
    }
    // Fallback на markdown (legacy)
    if _, err := os.Stat(mdPath); err == nil {
        return ScanTasks(string(data))  // текущий regex-парсер
    }
    return ScanResult{}, config.ErrNoTasks
}
```

**Фазы перехода:**

| Фаза | Что | Backward compat |
|------|-----|-----------------|
| 0 | sprint-tasks.md + regex (текущее) | — |
| 1 | Добавить YAML structs + парсер | Читает оба формата |
| 2 | Bridge генерирует JSON -> YAML | Merge через Go-код |
| 3 | `ralph migrate-tasks` CLI | md -> yaml конвертация |
| 4 | Удалить regex-парсер | Только YAML |

**Критерий удаления markdown:** если за 30 дней ни одного md-файла не прочитано (метрика в логах).

### 7.9 Размер файла

10 эпиков x 10 задач x ~150 байт/задача = ~15 KB YAML. Это **меньше** типичного sprint-tasks.md (>20 KB). Проблем с размером нет.

---

## 8. Промпты plan.md и replan.md

**Источник:** `v2-prompt-chain-design.md`

### 8.1 Исследование промптовых стратегий

| Стратегия | Суть | Применимость к ralph |
|-----------|------|---------------------|
| **Skeleton-of-Thought (SoT)** | Сначала скелет, затем детали параллельно | Скелет epics -> задачи |
| **Plan-and-Solve (PS)** | Промежуточная фаза планирования перед решением | Поле `analysis` в JSON = фаза reasoning |
| **Blueprint First, Model Second** | Go = детерминистический scaffold, LLM = семантический анализ | **Точно наш подход.** +10.1 п.п., -81.8% tool calls |
| **ADaPT** | As-needed decomposition (+28-33%) | Применяется к replan, не к plan |
| **Длина промпта (2025)** | Reasoning деградирует после ~3000 tokens. Sweet spot: 150-300 слов | bridge.md (244 строки) превышает. Целевой plan.md: ~100 строк |

### 8.2 Single-pass vs Multi-pass

| Вариант | Описание | Latency | Качество | Стоимость |
|---------|----------|---------|----------|-----------|
| A: Single-pass | Один LLM-вызов | 1x | Хорошее для <10 FR | 1x |
| B: Two-step | LLM1: classify -> LLM2: generate | 2x | Лучше для 10-20 FR | 1.5-2x |
| **C: CoT-in-JSON** | Один вызов, поле `analysis` как CoT-зона | **1x** | **Как B, стоимость как A** | **1x + ~200 tokens** |

**Выбрано: Вариант C (CoT-in-JSON, single-pass).**

Обоснование:
1. Latency = 1 вызов (пользователь ждёт)
2. CoT без ограничения reasoning: поле `analysis` НЕ ограничивает reasoning
3. ~200 extra tokens vs удвоение контекста при two-step
4. Исследования подтверждают: "reasoning first, then structured answer"
5. Для complex PRD (20+ FR): Go-код split по секциям (Blueprint First)

**Исключение для two-step:** если JSON parsing fails, retry с error feedback — фактически two-step, но только в fallback path.

### 8.3 JSON Schema для output

```json
{
  "analysis": "1-3 предложения: анализ требований, ключевые решения по группировке",
  "epics": [
    {
      "name": "Epic Name",
      "tasks": [
        {
          "title": "Описание задачи (до 500 символов)",
          "test_scenarios": ["scenario 1", "scenario 2"],
          "requirement_refs": ["AC-1", "FR-3"],
          "depends_on": [],
          "tags": [],          // SETUP | E2E | GATE
          "size": "M"          // S | M | L
        }
      ]
    }
  ]
}
```

**Что Go добавляет программно (НЕ LLM):**

| Поле | Кто | Как |
|------|-----|-----|
| `- [ ]` / `- [x]` | Go | Все новые = `[ ]`, existing сохраняются |
| `source:` | Go | Из requirement_refs + имя файла |
| `[GATE]` (первый в epic) | Go | Программно по позиции |
| `[GATE]` (deploy/security) | Go | Keyword scan по title |
| Ordering | Go + LLM | LLM задаёт depends_on, Go делает topological sort |
| Merge | Go | Детерминистический diff |

### 8.4 plan.md — draft промпта (~95 строк)

**Ключевые секции:**

1. **Role + isolation constraint** (2 строки): "Each task will be executed in an isolated session — the agent has NO memory of previous tasks."

2. **Project Context** (placeholder): `__PROJECT_CONTEXT__` — Go инжектирует tree, go.mod, CLAUDE.md

3. **Requirements** (placeholder): `__REQUIREMENTS_CONTENT__` — содержимое PRD/requirements

4. **Requirement Classification** (10 строк): 4 типа (Implementation, Behavioral, Verification, Manual). Только Implementation создаёт задачи.

5. **Task Granularity** (20 строк): Unit of work, критерии split/keep, минимум/максимум декомпозиции

6. **Testing** (3 строки): Тесты = часть задачи, не отдельная задача

7. **Dependency Ordering + Size** (6 строк): depends_on, S/M/L

8. **Existing Tasks** (условный блок): Только если merge mode

9. **Output Format** (~20 строк): JSON schema + field rules

### 8.5 Сравнение plan.md vs bridge.md

| Метрика | plan.md | bridge.md | Разница |
|---------|---------|-----------|---------|
| **Строки** | ~95 | 244 | **-61%** |
| Requirement Classification | 10 строк | 14 строк | -4 строки |
| Task Granularity | 20 строк | 58 строк (+ примеры) | -38 строк |
| Format Contract | 0 (JSON schema в Go) | 8 строк | -8 строк |
| Source Traceability | 0 (Go добавляет) | 41 строк | -41 строк |
| Merge Mode | 0 (Go делает) | 18 строк | -18 строк |
| Gate Marking | 0 (Go ставит) | 27 строк | -27 строк |
| Service Tasks ordering | 0 (Go сортирует) | 4 строки | -4 строки |
| Prohibited Formats | 0 (JSON schema) | 5 строк | -5 строк |
| Примеры (Correct/WRONG) | 0 (zero-shot) | 60 строк | -60 строк |

### 8.6 Что переиспользуется из bridge.md (18%)

| Секция bridge.md | Строки | Действие в plan.md |
|------------------|--------|--------------------|
| AC Classification (4 типа) | 28-41 | Упрощено до 1 абзаца |
| Task Granularity Rule | 43-98 | Сохранено ядро: unit of work + signals |
| Complexity Ceiling | 78-84 | Сохранены 4 эвристики |
| Minimum Decomposition | 86-90 | Сохранено (5+ AC -> 3+ tasks) |
| Task Ordering | 100-102 | Сохранено |
| Testing Within Tasks | 112-116 | Сохранено |
| **Итого** | **~45 строк из 244** | **18%** |

**Что убирается (41%):** Format Contract, Source Traceability, Merge Mode, Gate Marking details, Prohibited Formats, Service Tasks ordering, Negative examples — всё это Go делает программно.

**Что остаётся неизменным (примеры):** Примеры Correct/WRONG — ценны для обучения LLM, но в plan.md убраны (zero-shot достаточен для JSON output).

### 8.7 replan.md — draft промпта (~65 строк)

**Ключевые секции:**

1. **Role + correction framing** (3 строки): "performing a plan correction"

2. **Correction Principles** (4 правила):
   - Preserve completed tasks (DONE = immutable)
   - Minimal diff
   - Consistency (стиль существующих задач)
   - Dependency awareness

3. **Current Plan** (placeholder): `__CURRENT_TASKS__` — задачи с пометками [DONE]/[OPEN]

4. **Feedback/Changes** (placeholder): `__FEEDBACK_CONTENT__` — текст от пользователя

5. **What You Can Do** (4 действия): ADD, MODIFY, REMOVE, REORDER

6. **Output Format** (~20 строк): JSON с `"changes"` массивом

**Ключевое отличие от plan.md:** replan выдаёт **только изменения** (diff-формат), а не весь план заново:

```json
{
  "analysis": "что изменилось, почему, impact",
  "changes": [
    {"action": "add", "epic": "...", "task": {...}, "insert_after": "..."},
    {"action": "modify", "original_title": "...", "task": {...}},
    {"action": "remove", "original_title": "...", "reason": "..."}
  ]
}
```

**Преимущества diff-формата:**
- Предотвращает случайное изменение/удаление
- Минимизирует output tokens (2-5 изменений vs 20+ задач)
- Go применяет diff детерминистически
- `reason` для remove принуждает обосновать удаление

---

## 9. Общие выводы и рекомендации

### 9.1 Стратегическое решение

**Вариант D (ralph plan)** — единственный вариант, устраняющий стратегическую проблему BMad-зависимости при минимальном риске. Взвешенная оценка 8.50 vs 4.85 у ближайшего конкурента.

### 9.2 Архитектурные принципы v2

1. **Blueprint First, Model Second** — Go = детерминистический scaffold, LLM = семантический анализ. Подтверждено исследованиями (+10.1 п.п.).

2. **Программный merge вместо LLM** — 100% надёжность vs ~85%. $0 vs $0.01-0.05. <10 мс vs 30-60 сек.

3. **CoT-in-JSON** — single-pass с полем `analysis` как CoT-зоной. Latency = 1 вызов.

4. **YAML вместо markdown** для task storage — устраняет 4 regex'а, добавляет ID/зависимости/метаданные.

5. **Diff-based replan** — только изменения, не весь план. Минимизирует риск потери данных.

6. **Backward compatibility через dual-format** — YAML имеет приоритет, markdown fallback.

### 9.3 Количественные показатели

| Метрика | Текущий (v1) | Целевой (v2) | Изменение |
|---------|-------------|-------------|-----------|
| Time-to-first-task | 55-130 мин | 11-33 мин | **3-5x быстрее** |
| Стоимость planning | $2.50-9.80 | $0.10-0.30 | **90-97% экономия** |
| BMad-зависимость | Полная | Нулевая | **Устранена** |
| Промпт planning | 244 строки | 95 строк | **-61%** |
| Merge надёжность | ~85% | 100% | **Детерминистичный** |
| LOC нового кода | — | ~120 Go + ~150 prompt | **Минимальный** |
| LOC удаления (Phase 3) | — | -2844 (bridge/) | **Упрощение** |
| Нетто-эффект | — | -1324 строки | **Codebase проще** |
| Гибкость форматов | 1/10 | 9/10 | **9x** |
| Community adoption potential | Низкий | Высокий | **Self-contained CLI** |

### 9.4 Общий объём работ

| Компонент | LOC | Stories | Дни |
|-----------|-----|---------|-----|
| ralph plan (core planner) | ~800 + ~1250 тесты | 5 | 3 |
| ralph plan (merge + compat) | ~300 + ~400 тесты | 2 | 1 |
| ralph init | ~700 + ~800 тесты | 3-4 | 2 |
| ralph replan ([c] action) | ~800 (код + тесты) | 3-4 | 2 |
| YAML формат задач | ~500 + ~600 тесты | 2-3 | 1.5 |
| Bridge deprecation + removal | ~50 нового, -2844 удаления | 2 | 1 |
| **ИТОГО** | **~3150 + ~3050 тесты** | **17-20** | **~10.5** |

### 9.5 Рекомендуемый порядок реализации

1. **Epic 11: ralph plan** (Phase 1-2) — 7 stories, ~4 дня
   - Цель: `ralph plan docs/prd.md` -> sprint-tasks.md
   - Включает YAML формат, dual-format поддержку

2. **Epic 12: ralph init** — 3-4 stories, ~2 дня
   - Цель: `ralph init "описание"` -> docs/requirements.md
   - 3 flow: one-liner, interactive, brownfield

3. **Epic 13: ralph replan** — 3-4 stories, ~2 дня
   - Цель: gate action [c] -> Claude replan -> diff -> apply
   - Включает ValidateReplan, ComputeTaskDiff

4. **Epic 14: Bridge deprecation** — 2 stories, ~1 день
   - Phase 2: deprecated с warning
   - Phase 3: bridge/ удалён (-2844 LOC)

### 9.6 Риски и mitigation

| Риск | Вероятность | Mitigation |
|------|-------------|------------|
| Claude генерирует плохие задачи | Средняя | CoT-in-JSON + Go-валидация + human review |
| YAML-миграция ломает существующих пользователей | Низкая | Dual-format с автодетектом |
| replan теряет [x] задачи | Средняя | ValidateReplan() + ForcePreserveDoneTasks() fallback |
| Автодискавери неправильно классифицирует файлы | Средняя | 3 уровня (имя, каталог, содержимое) + CLI override |
| Промпт plan.md слишком короткий для complex PRD | Низкая | Go-код split по секциям (Blueprint First) |

### 9.7 Ключевые заимствования из конкурентов

| Что заимствуем | Откуда | Приоритет |
|----------------|--------|-----------|
| Persistent task file (tasks.md/yaml) | Kiro | Высокий (реализуется) |
| Spec-first pipeline | Copilot Workspace | Высокий (ralph plan) |
| As-needed decomposition | ADaPT (Allen AI) | Высокий (ralph replan) |
| Git-backed task state | Gastown | Высокий (sprint-tasks.yaml) |
| Blueprint First, Model Second | Академические исследования | Высокий (основной принцип) |
| Editable intermediate artifacts | Devin, Kiro | Средний (requirements.md editable) |
| Knowledge entries | Devin | Средний (уже есть в Epic 6) |
| Multi-agent orchestration | MetaGPT, Gastown | Низкий (v3+) |

---

## Приложение A: Источники исследований

### Академические работы
- ADaPT: As-Needed Decomposition and Planning (NAACL 2024, Allen AI)
- Requirements are All You Need (2024)
- MetaGPT: Meta Programming for Multi-Agent Framework (ICLR 2024)
- SWE-Agent: Agent-Computer Interfaces (NeurIPS 2024, Princeton)
- AutoCodeRover (ISSTA 2024, NUS)
- OpenHands (ICLR 2025)
- Blueprint First, Model Second (2025)
- Chain-of-Thought Prompting Elicits Reasoning (2022)

### Инструменты (11 проанализированных)
- Devin (Cognition), SWE-Agent (Princeton), OpenHands, AutoCodeRover (NUS)
- MetaGPT, Gastown (Steve Yegge), Claude Code (Anthropic)
- Cursor Composer, Windsurf Cascade, GitHub Copilot Workspace, Amazon Kiro

### Промпт-инжиниринг
- Claude Structured Outputs (Anthropic)
- Prompt Engineering Best Practices 2026
- Conflict Between LLM Reasoning and Structured Output (Google Cloud)
- Devin AI System Prompt (утечка, август 2025)

### Внутренние исследования проекта
- bridge-concept-analysis.md
- variant-d-self-sufficient-decomposition.md
- variant-d-cost-benefit.md
- variant-d-migration-path.md
- variant-d-task-format.md
- variant-d-prompt-architecture.md
