# Исследование: Форматы документов BMad v6

**Дата:** 2026-03-07
**Источник:** реальный проект learnPracticsCodePlatform (greenfield, medium complexity)
**Версия BMad:** v6-alpha (установлена в `.bmad/bmm/`)

---

## 1. Каталог артефактов BMad v6

### 1.1 Полный список документов

| # | Документ | Файл | Строк | Создаёт workflow | Фаза |
|---|----------|------|-------|------------------|------|
| 1 | Brainstorming Session | `docs/analysis/brainstorming-session-*.md` | ~120 | `1-analysis/research` (brainstorm-project) | 0 - Discovery |
| 2 | Product Brief | `docs/product-brief.md` | ~200-400 | `1-analysis/product-brief` | 0 - Discovery |
| 3 | Research (Domain/Market/Technical) | `docs/research-*.md` | варьируется | `1-analysis/research` | 0 - Discovery |
| 4 | **PRD** | `docs/prd.md` | **~420** | `2-plan-workflows/prd` | 1 - Planning |
| 5 | **UX Design Specification** | `docs/ux-design-specification.md` | **~1530** | `2-plan-workflows/create-ux-design` | 1 - Planning |
| 6 | **Architecture Decision Document** | `docs/architecture.md` | **~1760** | `3-solutioning/architecture` | 2 - Solutioning |
| 7 | **Epics & Stories** | `docs/epics.md` | **~2370** | `3-solutioning/create-epics-and-stories` | 2 - Solutioning |
| 8 | Implementation Readiness Report | `docs/implementation-readiness-report-*.md` | ~230 | `3-solutioning/implementation-readiness` | 2 - Solutioning |
| 9 | BMM Workflow Status | `docs/bmm-workflow-status.yaml` | ~120 | `workflow-status/init` | мета |
| 10 | **Sprint Status** | `docs/sprint-artifacts/sprint-status.yaml` | **~105** | `4-implementation/sprint-planning` | 3 - Implementation |
| 11 | **Story файлы** | `docs/sprint-artifacts/{N}-{M}-{title}.md` | **~125-235** | `4-implementation/create-story` | 3 - Implementation |
| 12 | Validation Reports | `docs/sprint-artifacts/validation-report-epic-{N}.md` | ~190 | `4-implementation/create-story` (validate) | 3 - Implementation |

**Жирным** выделены документы, РЕАЛЬНО используемые в pipeline разработки.

---

## 2. Детальная структура каждого документа

### 2.1 PRD (Product Requirements Document)

**Файл:** `docs/prd.md`
**Размер:** ~420 строк
**Workflow:** `2-plan-workflows/prd` (11 шагов)

**YAML frontmatter:**
```yaml
---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]
inputDocuments:
  - docs/analysis/brainstorming-session-2026-03-05.md
workflowType: 'prd'
lastStep: 11
project_name: 'learnPracticsCodePlatform'
user_name: 'Степан'
date: '2026-03-05'
---
```

**Секции (markdown H2/H3):**
1. `# Product Requirements Document - {project_name}` — заголовок + автор + дата
2. `## Executive Summary` — краткое описание продукта + "What Makes This Special" (буллеты)
3. `## Project Classification` — Technical Type, Domain, Complexity (текстовый блок)
4. `## Success Criteria` — User Success, Business Success, Technical Success, Measurable Outcomes
5. `## Product Scope` — MVP (буллет-список фич), Growth Features, Vision
6. `## User Journeys` — пронумерованные Journey 1..N (нарративные сценарии, 15-25 строк каждый)
   - Заканчивается `### Journey Requirements Summary` (таблица)
7. `## Web Application Specific Requirements` (или другой project-type) — Technical Architecture Considerations, Responsive Design, Performance Targets, Data Distribution, Implementation Considerations
8. `## Project Scoping & Phased Development` — MVP Strategy, MVP Feature Set, Post-MVP Features, Risk Mitigation Strategy
9. `## Functional Requirements` — сгруппированные FR по доменам, формат: `- FR{N}: {описание}`
10. `## Non-Functional Requirements` — Performance, Security, Scalability, Integration, Reliability, Observability

**Ключевые элементы:**
- FR нумерация сквозная (FR1-FR51 в примере)
- Группировка FR по доменным областям (Auth, Каталог, Редактор, etc.)
- User Journeys — развёрнутые текстовые сценарии с именами персонажей
- Чёткое разделение MVP / Growth / Vision

---

### 2.2 UX Design Specification

**Файл:** `docs/ux-design-specification.md`
**Размер:** ~1530 строк
**Workflow:** `2-plan-workflows/create-ux-design` (14 шагов)

**YAML frontmatter:**
```yaml
---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]
inputDocuments:
  - docs/prd.md
workflowType: 'ux-design'
lastStep: 14
status: complete
completedAt: '2026-03-05'
project_name: 'learnPracticsCodePlatform'
user_name: 'Степан'
date: '2026-03-05'
---
```

**Секции:**
1. `## Executive Summary` — Project Vision, Target Users, Key Design Challenges, Design Opportunities
2. `## Core User Experience` — Defining Experience, Platform Strategy, Effortless Interactions, Critical Success Moments
3. `## Visual Foundation` — Typography, Color System (design tokens), Spacing, Borders/Shadows
4. `## Design Directions` — HTML wireframe-варианты (включённый HTML код)
5. `## User Journeys` — детальные UX-флоу с описанием каждого шага
6. `## Component Strategy` — атомарные компоненты, составные, страницы
7. `## UX Patterns` — формы, навигация, обратная связь, состояния загрузки
8. `## Responsive & Accessibility` — breakpoints, a11y требования
9. `## State Management` — state machine диаграммы (Mermaid)
10. `## Tech Notes for Development` — конкретные рекомендации разработчику

**Особенности:**
- Содержит HTML wireframes (inline HTML code)
- Design tokens вместо hardcoded hex-кодов
- Mermaid-диаграммы для state machines
- Самый большой документ по объёму

---

### 2.3 Architecture Decision Document

**Файл:** `docs/architecture.md`
**Размер:** ~1760 строк
**Workflow:** `3-solutioning/architecture` (8 шагов)

**YAML frontmatter:**
```yaml
---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - docs/prd.md
  - docs/ux-design-specification.md
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-03-06'
project_name: 'learnPracticsCodePlatform'
user_name: 'Степан'
date: '2026-03-05'
---
```

**Секции (соответствуют step-ам workflow):**
1. `## Project Context Analysis` — Requirements Overview (таблица FR по группам + NFR)
2. `## Starter Template & Tech Stack` — выбранный стек, версии, обоснование
3. `## Key Architecture Decisions` — пронумерованные ADR (Architecture Decision Records)
4. `## Architecture Patterns` — паттерны кода, enforcement rules (H1-H8 и т.д.)
5. `## Project Structure & Boundaries` — дерево файлов, зависимости модулей
6. `## Validation` — Party Mode review (несколько рецензентов оценивают)
7. `## Data Models` — Prisma schema, API contracts (REST endpoints)
8. `## Testing Strategy` — тестовая пирамида, инструменты

**Особенности:**
- Enforcement rules с кодовыми именами (H1, H2, ..., H8)
- Party Mode — внутренняя peer-review от нескольких "рецензентов" (LLM)
- Конкретные API endpoints с request/response контрактами
- Prisma schema inline
- Таблицы с FR mapping на архитектурные решения

---

### 2.4 Epics & Stories (epics.md)

**Файл:** `docs/epics.md`
**Размер:** ~2370 строк
**Workflow:** `3-solutioning/create-epics-and-stories`

**Формат:** чистый markdown без YAML frontmatter (в отличие от остальных)

**Секции:**
1. `# {project_name} - Epic Breakdown` — заголовок + Author, Date, Project Level, Target Scale
2. `## Overview` — ссылки на входные документы
3. `## Functional Requirements Inventory` — полная таблица FR (FR1-FR51 с описаниями и группами)
4. `## Context Validation (Step 0)` — Party Mode Review таблица + выявленные пробелы (G1..G7)
5. `## Epic Structure Plan` — граф зависимостей (ASCII art), сводная таблица эпиков
6. `## FR Coverage Map` — полная таблица: FR -> Epic -> Stories -> описание
7. `## Epic N: {title}` (повторяется) — для каждого эпика:
   - User Value, FRs, Dependencies
   - `### Story N.M: {title}` — для каждой story:
     - User story (As a / I want / So that)
     - Acceptance Criteria в BDD формате (Given/When/Then/And)
     - Prerequisites
     - Technical Notes

**Особенности:**
- Самый большой документ (2370 строк)
- Stories внутри epics.md — КРАТКИЕ (7-30 строк каждая)
- AC в BDD формате: Given/When/Then
- FR Coverage Map обеспечивает 100% покрытие всех FR
- Граф зависимостей эпиков (ASCII)

---

### 2.5 Story файлы (индивидуальные)

**Файл:** `docs/sprint-artifacts/{N}-{M}-{slug}.md`
**Размер:** 125-235 строк
**Workflow:** `4-implementation/create-story`

**Формат:** чистый markdown, без YAML frontmatter

**Секции:**
```markdown
# Story N.M: {Title}

Status: ready-for-dev

## Story
As a {role},
I want {action},
So that {benefit}.

## Acceptance Criteria (BDD)
**Given** ...
**When** ...
**Then** ...
**And** ...

## Tasks / Subtasks
### Task 1: {title} (AC: {ref})
1.1. {subtask}
1.2. {subtask}

## Dev Notes
- {архитектурные паттерны и ограничения}
- {технические детали: версии, библиотеки}

### Project Structure Notes
- {пути файлов, модули}

### References
- docs/epics.md#Story N.M
- docs/architecture.md#{Section}
```

**Отличия от story в epics.md:**
- Story файл = РАСШИРЕННАЯ версия (~150 строк vs ~15 строк в epics.md)
- Добавлены Tasks/Subtasks с конкретными шагами реализации
- Добавлены Dev Notes с точными версиями, путями, паттернами
- Добавлены References на исходные документы
- Добавлена секция `## Dev Agent Record` (заполняется при разработке)

---

### 2.6 Sprint Status

**Файл:** `docs/sprint-artifacts/sprint-status.yaml`
**Размер:** ~100 строк
**Workflow:** `4-implementation/sprint-planning`

**Формат:** чистый YAML

```yaml
generated: 2026-03-06
project: learnPracticsCodePlatform
tracking_system: file-system
story_location: docs/sprint-artifacts

development_status:
  epic-0: in-progress
  0-1-turborepo-monorepo-scaffold: ready-for-dev
  0-2-prisma-schema-postgresql-setup: ready-for-dev
  ...
  epic-0-retrospective: optional
```

**Статусы:**
- Epic: `backlog` -> `in-progress` / `contexted` -> `done`
- Story: `backlog` -> `drafted` -> `ready-for-dev` -> `in-progress` -> `review` -> `done`
- Retrospective: `optional` -> `completed`

---

### 2.7 BMM Workflow Status

**Файл:** `docs/bmm-workflow-status.yaml`
**Размер:** ~120 строк
**Workflow:** `workflow-status/init`

**Формат:** YAML — отслеживает прогресс по фазам BMad Method

```yaml
generated: "2026-03-05"
project: "learnPracticsCodePlatform"
selected_track: "bmad-method"
field_type: "greenfield"

workflow_status:
  - id: "brainstorm-project"
    phase: 0
    phase_name: "Discovery"
    agent: "analyst"
    command: "brainstorm-project"
    status: "docs/analysis/brainstorming-session-*.md"
    output: "docs/analysis/..."
    note: "Completed..."
```

**Отслеживает:** какие workflow завершены, на каком этапе проект

---

### 2.8 Validation Reports

**Файл:** `docs/sprint-artifacts/validation-report-epic-{N}.md`
**Размер:** ~190 строк
**Workflow:** `4-implementation/create-story` (validate подпроцесс)

**Формат:** markdown

**Секции:**
- Заголовок + Validator + Date + Source Documents
- Checklist Legend (PASS / FAIL (FIXED) / FAIL (NOTED) / N/A)
- Для каждой story в эпике: таблица из 10 проверок
- Summary: таблица результатов + Cross-Story Consistency Notes

---

### 2.9 Implementation Readiness Report

**Файл:** `docs/implementation-readiness-report-*.md`
**Размер:** ~230 строк
**Workflow:** `3-solutioning/implementation-readiness`

**Секции:**
- Executive Summary (READY / NOT READY + оценка X/10)
- Project Context
- Document Inventory (таблица документов с путями и строками)
- Alignment Validation Results (PRD <-> Architecture, PRD <-> Stories, etc.)
- Gap and Risk Analysis
- UX and Special Concerns
- Detailed Findings (Critical / High / Medium / Low)
- Recommendations

---

## 3. YAML Frontmatter — общая структура

Документы фаз 0-2 (analysis, planning, solutioning) имеют YAML frontmatter:

```yaml
---
stepsCompleted: [1, 2, ...]     # какие шаги workflow пройдены
inputDocuments:                  # какие документы были входными
  - docs/prd.md
workflowType: 'prd'             # тип workflow
lastStep: 11                    # последний шаг
status: 'complete'              # опционально
completedAt: '2026-03-06'       # опционально
project_name: '{name}'
user_name: '{user}'
date: '{date}'
---
```

Документы фазы 3+ (stories, sprint-status) **НЕ** имеют YAML frontmatter.

---

## 4. Workflow генерации — полный pipeline

### 4.1 Граф зависимостей

```
Phase 0: Discovery (опциональная)
  brainstorm-project → product-brief → research
                           ↓
Phase 1: Planning
  PRD ←──────────── UX Design
   ↓                    ↓
Phase 2: Solutioning
  Architecture ←──── PRD + UX
       ↓
  Epics & Stories ←── PRD + Architecture + UX
       ↓
  Implementation Readiness ←── все 4 документа
       ↓
Phase 3: Implementation
  Sprint Planning → sprint-status.yaml
       ↓
  create-story (для каждой story) → {N}-{M}-{slug}.md
       ↓
  [optional] validate-create-story → validation-report
       ↓
  dev-story → код
       ↓
  code-review → findings
```

### 4.2 Какой workflow создаёт какой документ

| Workflow | Агент | Вход | Выход |
|----------|-------|------|-------|
| `brainstorm-project` | analyst | пользовательский ввод | `docs/analysis/brainstorming-*.md` |
| `product-brief` | analyst | brainstorm | `docs/product-brief.md` |
| `research` | analyst | product-brief | `docs/research-*.md` |
| `prd` | pm | brainstorm/brief | **`docs/prd.md`** |
| `create-ux-design` | ux-designer | prd.md | **`docs/ux-design-specification.md`** |
| `create-architecture` | architect | prd.md + ux.md | **`docs/architecture.md`** |
| `create-epics-and-stories` | pm | prd + arch + ux | **`docs/epics.md`** |
| `implementation-readiness` | architect | все 4 документа | `docs/implementation-readiness-*.md` |
| `sprint-planning` | sm | epics.md | **`docs/sprint-artifacts/sprint-status.yaml`** |
| `create-story` | sm | epics + arch + prd + ux + sprint-status | **`docs/sprint-artifacts/{N}-{M}-*.md`** |
| `validate-create-story` | sm | story файл + все документы | `docs/sprint-artifacts/validation-report-*.md` |
| `dev-story` | dev | story файл | код проекта |
| `code-review` | reviewer | story + git diff | findings в story файле |

### 4.3 Фазы и порядок

1. **Phase 0 — Discovery** (опциональная): brainstorm, product-brief, research
2. **Phase 1 — Planning**: PRD, UX Design
3. **Phase 2 — Solutioning**: Architecture, Epics & Stories, Implementation Readiness
4. **Phase 3 — Implementation**: Sprint Planning, затем цикл (create-story -> dev-story -> code-review) для каждой story

---

## 5. Зависимости между документами

### 5.1 Граф зависимостей (входы -> выходы)

```
brainstorm-session ──→ PRD
                        ↓
                    UX Design ──→ Architecture
                        ↓              ↓
                        └──────→ Epics & Stories ←── Architecture
                                       ↓
                                Implementation Readiness ←── PRD + UX + Architecture + Epics
                                       ↓
                                Sprint Status ←── Epics
                                       ↓
                                Story файлы ←── Epics + Architecture + UX + PRD + предыдущие stories + git log
```

### 5.2 Что ссылается на что

| Документ | Ссылается на |
|----------|-------------|
| PRD | brainstorm (inputDocuments в frontmatter) |
| UX Design | PRD (inputDocuments) |
| Architecture | PRD + UX (inputDocuments) |
| Epics | PRD + Architecture + UX (Overview section) |
| Impl. Readiness | все 4 (Document Inventory) |
| Sprint Status | Epics (список stories) |
| Story файл | Epics + Architecture (References section) |
| Validation Report | Story файлы + Epics + Architecture |

---

## 6. Что РЕАЛЬНО используется runner/bridge в bmad-ralph

### 6.1 Документы, используемые runner'ом

Из анализа Go-кода (`config/config.go`, `cmd/ralph/bridge.go`, `runner/`):

| Документ | Использование |
|----------|--------------|
| **Sprint Status** (`sprint-status.yaml`) | Сканируется для обнаружения story с нужным статусом. `runner/scan.go` читает для определения следующей задачи |
| **Story файлы** (`{N}-{M}-*.md`) | Основной рабочий артефакт. `runner/scan.go` сканирует директорию `StoriesDir`, фильтрует non-story файлы (validation reports, retrospectives, sprint-status). Bridge собирает промпты на основе story контента |
| **`StoriesDir` config** | `config.Config.StoriesDir` (default: `docs/sprint-artifacts`) — путь к директории с stories |

### 6.2 Документы, НЕ используемые runner'ом

| Документ | Статус |
|----------|--------|
| PRD | Не читается кодом. Используется только workflow'ами BMad |
| UX Design | Не читается кодом |
| Architecture | Не читается кодом |
| Epics | Не читается кодом |
| Impl. Readiness | Не читается кодом |
| BMM Workflow Status | Не читается кодом |
| Validation Reports | Фильтруются bridge'ом как non-story файлы |
| Brainstorm | Не читается кодом |

### 6.3 Вывод

Ralph runner работает ТОЛЬКО с двумя типами файлов:
1. `sprint-status.yaml` — для определения статуса stories
2. `{N}-{M}-*.md` story файлы — как контекст для Claude Code сессий

Все остальные документы (PRD, Architecture, UX, Epics) используются исключительно BMad workflow'ами для ГЕНЕРАЦИИ story файлов. Runner не знает о них и не нуждается в них.

---

## 7. Воспроизводимость формата без BMad

### 7.1 Что нужно для встраивания в ralph

Для генерации story файлов без BMad workflow'ов нужно воспроизвести:

**Минимально необходимое (для runner):**
1. **Sprint Status YAML** — простой YAML с ключами `{N}-{M}-{slug}: {status}`
2. **Story файлы** — markdown с секциями:
   - `Status: {status}` (первая строка после заголовка)
   - `## Story` (As a / I want / So that)
   - `## Acceptance Criteria` (BDD: Given/When/Then)
   - `## Tasks / Subtasks` (чеклисты)
   - `## Dev Notes` (техконтекст для разработчика)

**Необязательное для runner, но полезное:**
3. Epics файл (для create-story workflow)
4. Architecture/PRD (для обогащения story контекста)

### 7.2 Сложность воспроизведения

| Аспект | Сложность | Комментарий |
|--------|-----------|-------------|
| Story формат | Низкая | Простой markdown, без frontmatter |
| Sprint Status | Низкая | Простой YAML, 2-3 поля |
| PRD генерация | Высокая | 11-шаговый интерактивный workflow с Party Mode |
| Architecture | Высокая | 8-шаговый workflow с ADR + Party Mode validation |
| UX Design | Очень высокая | 14-шаговый workflow, самый сложный |
| Epics генерация | Средняя | Зависит от PRD+Architecture, но формат простой |
| Create-story | Средняя | Обогащение из всех документов, но формат story простой |

### 7.3 Что можно встроить в ralph v2

1. **Sprint Status управление** — уже частично есть (scan stories). Можно добавить автоматическое обновление статусов
2. **Story генерация** — можно встроить упрощённый create-story (промпт Claude для генерации story из epics.md)
3. **PRD/Architecture/UX** — НЕ стоит встраивать. Это интерактивные многошаговые workflow'ы, требующие человеческого участия. Лучше оставить как внешний процесс

### 7.4 Формат story файла — спецификация для ralph

Минимальный формат story файла, который ralph может обрабатывать:

```markdown
# Story {N}.{M}: {Title}

Status: {ready-for-dev|in-progress|review|done}

## Story

As a {role},
I want {action},
So that {benefit}.

## Acceptance Criteria (BDD)

**AC-1: {название}**
**Given** {предусловие}
**When** {действие}
**Then** {ожидаемый результат}
**And** {дополнительные критерии}

## Tasks / Subtasks

### Task 1: {заголовок} (AC: {ссылка})
1.1. {подзадача}
1.2. {подзадача}

## Dev Notes

- {архитектурные паттерны}
- {технические ограничения}
- {версии библиотек}

### References
- {ссылки на исходные документы}
```

---

## 8. Количественный анализ

### 8.1 Объём документации для одного проекта

| Метрика | Значение |
|---------|----------|
| Общий объём docs/ | ~6530 строк |
| PRD | 418 строк |
| Architecture | 1764 строки |
| UX Design | 1532 строки |
| Epics | 2365 строк |
| Impl. Readiness | 226 строк |
| Sprint Status | 105 строк |
| BMM Workflow Status | 118 строк |
| Story файлов | 31 штук |
| Средний размер story | ~170 строк |
| Общий объём stories | ~5300 строк |
| Validation Reports | 9 штук, ~190 строк каждый |
| **ИТОГО документации** | **~13500 строк** |

### 8.2 Соотношение "документация vs код"

На момент анализа проект learnPracticsCodePlatform имеет ~13500 строк документации и 0 строк кода (все stories в статусе ready-for-dev). Это нормально для BMad Method — документация создаётся ДО кода.

### 8.3 Коэффициент расширения epics -> stories

- Epics.md: ~15 строк на story (краткие AC + Technical Notes)
- Story файл: ~170 строк (полные Tasks/Subtasks + Dev Notes + References)
- **Коэффициент расширения: ~11x**

---

## 9. Ключевые выводы

1. **BMad v6 генерирует 12 типов документов**, но runner использует только 2 (sprint-status + story файлы)

2. **Документы фаз 0-2 имеют YAML frontmatter** с метаданными workflow (stepsCompleted, inputDocuments). Документы фазы 3 (stories) — чистый markdown

3. **Pipeline линейный**: brainstorm -> PRD -> UX -> Architecture -> Epics -> Stories. Каждый следующий документ зависит от предыдущих

4. **Story файл — единственный "мост"** между BMad документацией и ralph runner. Весь контекст из PRD/Architecture/UX должен быть "впитан" в story при создании

5. **Формат story файла прост и стабилен**: markdown без frontmatter, фиксированные секции (Story, AC, Tasks, Dev Notes). Легко воспроизводим

6. **Sprint Status — простой YAML реестр** с парами ключ:статус. Легко воспроизводим

7. **Сложные workflow (PRD, Architecture, UX)** — интерактивные многошаговые процессы с Party Mode review. Встраивать в ralph нецелесообразно — лучше оставить как внешний инструмент

8. **Create-story workflow** — единственный кандидат на встраивание в ralph. Берёт данные из epics.md + architecture.md и генерирует обогащённый story файл. Можно реализовать как промпт для Claude Code

9. **Validation Reports** — побочный продукт validate-create-story. Фильтруются runner'ом как non-story. Полезны для качества, но не для автоматизации
