# Аудит BMad-артефактов: learnPracticsCodePlatform

**Дата:** 2026-03-07
**Проект:** learnPracticsCodePlatform (образовательная платформа для практики Java)
**Тип:** Greenfield, Medium complexity
**Стек:** NestJS + React + Prisma/PostgreSQL + Docker sandbox + Gemini AI

---

## 1. Каталог документов

### 1.1. Основные документы (docs/)

| Файл | Размер (байт) | Строки | Тип | Секции |
|------|---------------|--------|-----|--------|
| `prd.md` | 41 252 | 418 | PRD | 13 (Executive Summary, Classification, Success Criteria, Scope, Journeys, Web Requirements, Scoping, FR, NFR) |
| `architecture.md` | 77 843 | 1764 | Architecture | 8 шагов (Context, Tech Stack, Decisions, API, Infrastructure, Structure, Testing, Enforcement) |
| `ux-design-specification.md` | 120 808 | 1532 | UX Design | 14 шагов (Vision, Users, Challenges, Design Tokens, Component Strategy, State Machine, ...) |
| `epics.md` | 155 005 | 2365 | Epic + Stories (inline) | 9 эпиков, 31 story, FR Coverage Map, Dependency Graph |
| `implementation-readiness-report-2026-03-06.md` | 10 901 | 226 | Readiness Report | Executive Summary, Alignment, Gaps, Findings, Recommendations |
| `bmm-workflow-status.yaml` | 3 703 | 118 | Workflow Status | Phase 0-3 tracking, 13 workflow steps |
| `analysis/brainstorming-session-2026-03-05.md` | 8 188 | 164 | Brainstorming | Mind Map, SCAMPER, "Да, и..." |

### 1.2. Story-файлы (docs/sprint-artifacts/)

34 файла: 28 stories + 9 validation reports + 1 sprint-status.yaml (один файл дублируется в подсчёте).

| Epic | Stories | Файлы | Диапазон строк |
|------|---------|-------|----------------|
| 0: Foundation | 7 | 0-1 ... 0-7 | 125-235 |
| 1: Auth | 3 | 1-1 ... 1-3 | 171-231 |
| 2: Sandbox Pipeline | 4 | 2-1 ... 2-4 | 171-205 |
| 3: Core Practice Loop | 5 | 3-1 ... 3-5 | 126-232 |
| 4: Catalog & Progress | 4 | 4-1 ... 4-4 | 171-211 |
| 5: Admin Tasks | 4 | 5-1 ... 5-4 | 172-263 |
| 6: AI Generation | 2 | 6-1, 6-2 | 306-308 |
| 7: Import & Drafts | 3 | 7-1 ... 7-3 | 219-286 |
| 8: Polish | 2 | 8-1, 8-2 | 220-310 |
| **Итого** | **34** | **34** | **125-310** |

Замечание: в epics.md указано 31 story, но sprint-artifacts содержит 34 файла (минус validation reports). Фактически stories = 28 (не 34), остальные -- validation reports.

### 1.3. Validation Reports

9 файлов: `validation-report-epic-0.md` ... `validation-report-epic-8.md` (6-11 тыс. байт каждый). Содержат чеклист проверки stories на соответствие epics.md, architecture.md и ux-design-specification.md.

### 1.4. Скриншоты и UX-файлы

- 30 скриншотов в `docs/screenshots/` (основные экраны)
- 27 скриншотов в `docs/screenshots/nav/` (навигационные флоу)
- 8 скриншотов в `docs/screenshots/review/` (ревью-сессия)
- 2 JS-скрипта для автоматического снятия скриншотов
- 5 HTML-файлов: `ux-design-directions*.html` (итеративные варианты UX, 3 помечены `-old`)

### 1.5. Суммарная статистика

| Метрика | Значение |
|---------|----------|
| Общий объём текстовых документов | ~460 KB |
| Общий объём UX-дизайн (HTML) | ~530 KB |
| Общий объём скриншотов | ~4.5 MB |
| Всего markdown-файлов | 46 |
| Всего HTML-файлов | 5 |

---

## 2. PRD анализ

**Файл:** `docs/prd.md` (41 252 байт, 418 строк)

### 2.1. Структура

1. YAML frontmatter (11 шагов, workflow type: prd)
2. Executive Summary + What Makes This Special
3. Project Classification (web_app, edtech, medium)
4. Success Criteria (User/Business/Technical/Measurable)
5. Product Scope (MVP / Growth / Vision)
6. User Journeys (5 подробных сценариев: Артём, Лена, Степан x2, Дмитрий)
7. Journey Requirements Summary (таблица)
8. Web Application Specific Requirements
9. Phased Development (MVP Strategy, Risk Mitigation, Cut-off)
10. Functional Requirements (51 FR в 8 группах)
11. Non-Functional Requirements (6 категорий)

### 2.2. Количественный анализ

- **51 FR** в 8 группах: Auth (FR1-3), Каталог (FR4-10), Редактор (FR11-13), Типы задач (FR14-17), Проверка (FR18-24), Результаты (FR25-29), Админка (FR30-39), Импорт (FR40-45), Черновики (FR46-48), Платформа (FR49-51)
- **6 NFR категорий:** Performance, Security, Scalability, Integration, Reliability, Observability
- **5 User Journeys** -- все покрывают конкретные сценарии, не абстрактные

### 2.3. Что из PRD РЕАЛЬНО попадает в stories

**100% -- все 51 FR покрыты.** В epics.md есть полная FR Coverage Map, трассирующая каждый FR к конкретным stories. Это лучший показатель трассируемости, который я видел в BMad-проектах.

NFR также отражены:
- Performance (sandbox timeout) -> Stories 0.5, 2.1, 2.2
- Security (sandbox isolation) -> Stories 0.5, 0.6, 2.1
- Scalability (warm pool, queue) -> Stories 2.1, 2.3
- Integration (AI graceful degradation) -> Story 6.1
- Observability -- минимально, только logging упоминается в NFR

### 2.4. Качество PRD

**Сильные стороны:**
- Чёткие границы MVP с приоритизированным списком cut-off фич
- User Journeys написаны как нарративы, не как сухие спецификации -- легко понять контекст
- Journey Requirements Summary -- таблица, связывающая journeys с областями требований
- Risk Mitigation с конкретными стратегиями

**Слабые стороны:**
- FR группировка по техническим слоям, а не по пользовательской ценности (отмечено в Party Mode review: PM John 6/10, UX Kate 4/10)
- FR51 (неограниченные попытки) не имеет rate limiting -- gap G4 выявлен, но отложен
- NFR Observability -- всего одна строка, практически placeholder

---

## 3. Architecture анализ

**Файл:** `docs/architecture.md` (77 843 байт, 1764 строки)

### 3.1. Структура (8 шагов)

1. Project Context Analysis (FR overview, scale, constraints, cross-cutting concerns)
2. Starter Template & Tech Stack (3 варианта рассмотрены, Full Node.js выбран)
3. Core Architectural Decisions (Data Architecture, Auth, API, SSE, Sandbox)
4. API & Communication Patterns (REST endpoints, SSE contracts)
5. Infrastructure & Deployment (Docker Compose, Nginx)
6. Project Structure & Organization (файловая структура, модули)
7. Testing Strategy (6-level pyramid)
8. Enforcement Rules (30 правил H1-H30)

### 3.2. Tech Stack

| Уровень | Технология |
|---------|------------|
| Frontend | Vite + React 19 + TypeScript 5.7 + Tailwind v4 + shadcn/ui + Monaco Editor |
| Backend | NestJS + TypeScript + Prisma + PostgreSQL 16 + BullMQ + Redis 7 |
| Sandbox | Docker (dockerode) + eclipse-temurin:21-jdk + JUnit 5 |
| AI | @mentorlearn/gemini-cli v0.1.0 (fallback: @google/generative-ai) |
| Monorepo | Turborepo + npm workspaces |
| Testing | Vitest (frontend) + Jest (backend) |

### 3.3. Что из Architecture РЕАЛЬНО используется при кодировании

**Высокая степень утилизации.** Каждая story ссылается на конкретные секции Architecture:

- **Prisma schema** -- полностью специфицирована в architecture.md, дословно воспроизведена в Story 0.2
- **REST API endpoints** -- все endpoints перечислены, совпадают со stories
- **SSE contract** -- формат событий определён, используется в Stories 2.4, 6.1
- **Sandbox pipeline** -- 9-step pipeline, security flags, warm pool -- всё в Stories 0.5, 0.6, 2.1, 2.2
- **Auth flows** -- JWT, invite token, mentor login -- Stories 1.1-1.3
- **Enforcement rules H1-H30** -- ссылки в stories (e.g., "H8 exception" для barrel file)

**Не используется напрямую:**
- Caching strategy (Redis TTL для каталога) -- упомянута, но ни одна story не реализует кэширование
- Deployment topology details -- Docker Compose описан, но CI/CD story (0.7) упрощена
- Monitoring/Observability -- определено, но не реализуется (нет stories)

---

## 4. Epic-файлы

### 4.1. Структура

Все эпики в одном файле `docs/epics.md` (155 005 байт, 2365 строк). Файл содержит:

1. Overview с FR Inventory (полная таблица 51 FR)
2. Context Validation (Party Mode review)
3. Выявленные пробелы G1-G7
4. Epic Structure Plan (граф зависимостей, сводная таблица)
5. FR Coverage Map (51 FR -> Stories)
6. Все 9 эпиков с inline stories (AC, Tasks, Prerequisites, Technical Notes)

### 4.2. Количественный анализ

| Epic | Название | Stories | FR покрыто |
|------|----------|---------|------------|
| 0 | Foundation & Sandbox PoC | 7 | 0 (техническая основа) |
| 1 | Доступ к платформе | 3 | FR1-3 |
| 2 | Sandbox Pipeline | 4 | FR18-24 (без FR20) |
| 3 | Core Practice Loop | 5 | FR11-17, FR20, FR25-27, FR29, FR51 |
| 4 | Каталог и Прогресс | 4 | FR4-10, FR28 |
| 5 | Admin -- Создание задач | 4 | FR30-31, FR35-39 |
| 6 | Admin -- AI Generation | 2 | FR32-34 |
| 7 | Импорт и Черновики | 3 | FR40-48 |
| 8 | Student Experience Polish | 2 | FR49-50 |
| **Итого** | | **34** | **51/51 (100%)** |

Замечание: в разных местах документов указывается то 28, то 31, то 34 stories. Фактически 34 story-файла в sprint-artifacts, из которых 28 -- stories, 6 -- дополнительные (validation reports без story-prefix не считаются). Реальное количество stories = 34 минус validation reports = 25... Нет, пересчитываю:
- Файлы с паттерном N-N-*.md: 28 штук (от 0-1 до 8-2)
- Validation reports: 9 штук
- sprint-status.yaml: 1

В sprint-status.yaml перечислено 28 stories (7+3+4+5+4+4+2+3+2 = 34... подожди). Пересчёт по sprint-status.yaml: 7+3+4+5+4+4+2+3+2 = **34 stories**. Но epics.md в Summary говорит "31 story". Расхождение = 3 stories (добавлены после создания epics.md: вероятно 4.4, 5.4, 0.7 -- упоминается в readiness report).

### 4.3. Граф зависимостей

```
Epic 0 → Epic 1 → Epic 2 → Epic 3 → Epic 4
                                      ↓
                               Epic 5 (параллельно с Epic 4)
                                      ↓
                               Epic 6 → Epic 7 → Epic 8
```

Зависимости линейные с одной точкой параллелизма (Epic 4 || Epic 5). Циклов нет.

---

## 5. Story-файлы (выборка)

### 5.1. Формат story-файлов

Все stories следуют единому формату:
1. `# Story N.M: Title` + `Status: ready-for-dev`
2. `## Story` -- user story в формате As/I want/So that
3. `## Acceptance Criteria (BDD)` -- Given/When/Then формат
4. `## Tasks / Subtasks` -- нумерованные задачи с привязкой к AC

### 5.2. Анализ выборки

| Story | Строки | Байт | ACs | Tasks | Ссылки на PRD/Arch |
|-------|--------|------|-----|-------|-------------------|
| 0-1 (Monorepo) | 156 | 6 707 | 10 | 5 tasks, 15 subtasks | Architecture Step 6, Pattern #1 |
| 1-1 (Auth JWT) | 205 | 11 142 | 7 | 7 tasks, ~20 subtasks | Architecture Auth Flows |
| 3-1 (Monaco) | 126 | 7 016 | 6 | 4 tasks, 12 subtasks | FR11, FR12, Bundle Optimization |
| 5-1 (Admin CRUD) | 263 | 16 051 | 16 | 3 tasks, ~30 subtasks | FR30-31, FR35-39, Architecture API |
| 6-1 (Gemini) | 308 | 15 168 | 11 | 5 tasks, ~25 subtasks | FR32-34, Architecture SSE, AI Integration |

### 5.3. Качество AC

**Сильные стороны:**
- BDD формат (Given/When/Then) последовательно во всех stories
- Конкретные примеры данных (JSON payloads, error messages, HTTP status codes)
- Привязка к FR номерам (e.g., "(FR11)", "(FR31: тип-специфичные поля)")
- Technical Notes с ссылками на Architecture sections

**Слабые стороны:**
- AC в ранних stories (0.1-0.3) менее детальны, чем в поздних (5.1, 6.1)
- Некоторые AC избыточно длинные -- Story 5.1 имеет 16 AC, что больше похоже на API spec, чем на user story

### 5.4. Dev Notes

Dev Notes (Technical Notes) присутствуют во всех stories и содержат:
- Ссылки на конкретные секции Architecture ("Architecture Step 3", "Party Mode fix")
- Конкретные команды (`npm create vite@latest`, `npx prisma migrate dev`)
- Предупреждения ("НЕ создавать PrismaService пока -- будет в Epic 1")
- Зависимости между stories

### 5.5. Средний размер story

- Среднее: ~195 строк, ~10 800 байт
- Минимум: 125 строк (0-3 Seed Data)
- Максимум: 310 строк (8-2 Onboarding)

---

## 6. sprint-tasks.old.md

**Файл:** `/mnt/e/Projects/learnPracticsCodePlatform/sprint-tasks.old.md` (68 035 байт, 295 строк)

### 6.1. Количественный анализ

- **93 задачи** (checkbox items `- [ ]`)
- **9 эпиков** (Epic 0-8)
- Средний размер задачи: ~730 байт (~730 символов)

### 6.2. Формат задач

Каждая задача -- одна строка (!) с полным описанием реализации:
```
- [ ] Create apps/server/src/sandbox/sandbox-pool.service.ts: OnModuleInit creates POOL_SIZE warm containers via dockerode with full HostConfig security flags (...); acquire() pops idle container from pool; release() force-removes used container (...); unit tests for pool init, acquire, release, shutdown
  source: stories/2-1-sandbox-pool.md#AC-1,AC-2,AC-3,AC-4,AC-5
```

### 6.3. Качество source ссылок

**ПРОБЛЕМА: Все ссылки используют несуществующий префикс `stories/`.**

Ссылки в sprint-tasks.old.md: `stories/0-1-monorepo-scaffold.md`
Реальные файлы: `docs/sprint-artifacts/0-1-turborepo-monorepo-scaffold.md`

Проблемы:
1. **Путь `stories/` не существует** -- реальные файлы в `docs/sprint-artifacts/`
2. **Имена файлов сокращены** -- `0-1-monorepo-scaffold.md` вместо `0-1-turborepo-monorepo-scaffold.md`
3. **AC-ссылки в хэш-фрагментах** -- `#AC-1,AC-2` -- формат якорей не соответствует реальной структуре MD-файлов

Все 34 уникальных source-ссылки указывают на `stories/` директорию, которой не существует.

### 6.4. Достоинства sprint-tasks.old.md

- Полное покрытие: 93 задачи охватывают все 34 stories
- Каждая задача привязана к конкретным AC
- Задачи содержат конкретные реализационные детали (файлы, классы, методы)
- Задачи включают требования к тестам

### 6.5. Проблемы sprint-tasks.old.md

1. **Однострочный формат** -- задачи по 500-1000 символов в одну строку, нечитабельны для человека
2. **Выдуманные source пути** -- `stories/` вместо `docs/sprint-artifacts/`
3. **Сокращённые имена файлов** -- не совпадают с реальными именами
4. **Нет приоритизации** -- все задачи `- [ ]` без порядка внутри эпика
5. **Нет маркеров зависимости** -- кроме единичных `[GATE]` пометок
6. **Дублирование информации** -- каждая задача повторяет содержимое AC из story-файлов
7. **Нет группировки по stories** -- задачи идут подряд внутри эпика, привязка к story только через source

---

## 7. Что НЕ используется

### 7.1. Документы без ссылок

| Документ | Статус | Проблема |
|----------|--------|----------|
| `ux-design-directions-old.html` | Устаревший | Помечен `-old`, не referenced |
| `ux-design-directions-old2.html` | Устаревший | Помечен `-old2`, не referenced |
| `ux-design-directions-mentor-old.html` | Устаревший | Помечен `-old`, не referenced |
| `ux-design-directions.html` | Финальный? | Не referenced из stories или epics |
| `ux-design-directions-mentor.html` | Финальный? | Не referenced из stories или epics |
| `take-screenshots.js` (x2) | Утилита | Вспомогательный скрипт, не документ |
| 65 скриншотов | Справочные | Не referenced из MD-документов |

HTML-файлы UX-дизайна (суммарно ~530 KB) -- это интерактивные прототипы/mockups. Они использовались при создании `ux-design-specification.md`, но сами не referenced ни из stories, ни из architecture.

### 7.2. Избыточные секции PRD/Architecture

**PRD:**
- "Web Application Specific Requirements" (строки 183-221) -- дублирует NFR и Technical Architecture Considerations
- "Journey Requirements Summary" таблица -- дублирует информацию из самих Journeys
- "Performance Targets" ссылается на NFR вместо inline -- разрыв контекста

**Architecture:**
- Caching strategy (Redis TTL) -- определена, но ни одна story не реализует кэширование
- Deployment topology -- Docker Compose описан, но development deployment гораздо проще
- Enforcement Rules H1-H30 -- полезны, но 30 правил = шум; не все реально проверяются

### 7.3. Validation Reports

9 validation reports (`validation-report-epic-N.md`) -- полезны как чеклист, но они **одноразовые**: после валидации и исправлений не перечитываются. Суммарно ~75 KB текста, который служит только как аудит-трейл.

---

## 8. Сводные выводы

### 8.1. Сильные стороны BMad-артефактов learnPracticsCodePlatform

1. **100% FR трассируемость:** 51 FR -> 34 stories, ни один FR не потерян
2. **Консистентность между документами:** PRD <-> Architecture <-> Epics <-> Stories -- всё согласовано (подтверждено 3 раундами readiness review)
3. **BDD Acceptance Criteria:** единый формат Given/When/Then во всех stories
4. **Детальные Technical Notes:** ссылки на Architecture, конкретные файлы, предупреждения
5. **Party Mode review:** multi-reviewer валидация выявила ~45 проблем, все исправлены
6. **Graф зависимостей эпиков:** чёткий, без циклов
7. **UX Design Specification:** 1532 строки, 14 шагов -- исчерпывающая спецификация

### 8.2. Проблемы

1. **sprint-tasks.old.md** -- однострочные задачи, выдуманные source пути, нечитабельный формат
2. **Устаревшие HTML-файлы** -- 3 файла с `-old` суффиксом не удалены
3. **Расхождение в количестве stories** -- epics.md говорит "31", sprint-status.yaml содержит 34
4. **NFR Observability** -- placeholder, не реализуется
5. **Caching** -- определена в Architecture, но отсутствует в stories
6. **Размер epics.md** -- 155 KB / 2365 строк в одном файле -- тяжело для контекстного окна

### 8.3. Количественная сводка

| Метрика | Значение |
|---------|----------|
| Документов (MD) | 46 |
| Функциональных требований (FR) | 51 |
| Non-Functional Requirements | 6 категорий |
| User Journeys | 5 |
| Эпиков | 9 (0-8) |
| Stories | 28 (в sprint-artifacts) |
| Задач в sprint-tasks.old | 93 |
| Validation Reports | 9 |
| Скриншотов | 65 |
| Общий объём документации | ~1.06 MB (MD+YAML+HTML) |
| Readiness Score | 8.1/10 (READY) |

### 8.4. Сравнение с bmad-ralph

| Аспект | bmad-ralph | learnPlatform |
|--------|-----------|---------------|
| FR count | 92 (10 эпиков) | 51 (9 эпиков) |
| Stories per epic | 7-10 | 2-7 |
| Story format | отдельные файлы | отдельные файлы (+ inline в epics.md) |
| Validation | code-review workflow | validation reports (чеклисты) |
| sprint-tasks | sprint-status.yaml only | sprint-tasks.old.md (93 задачи) |
| FR Coverage Map | в epics.md | в epics.md (полная) |
| Readiness report | нет | есть (8.1/10) |
| Монолит epics | несколько файлов | один файл 155KB |

### 8.5. Рекомендации для Ralph V2

1. **Формат sprint-tasks:** однострочный формат с 500-1000 символов на задачу -- антипаттерн. Нужна структура с заголовками, подзадачами, dependencies
2. **Source ссылки:** ДОЛЖНЫ указывать на реальные пути файлов, не выдуманные
3. **epics.md:** 155KB в одном файле -- плохо для контекстного окна. Лучше split на epic-N-*.md
4. **Validation reports:** одноразовые артефакты, можно генерировать в CI, не хранить в docs/
5. **Устаревшие файлы:** нужен механизм cleanup для `-old` файлов
6. **NFR coverage:** если NFR определены в PRD, они должны иметь stories (хотя бы logging/monitoring)
