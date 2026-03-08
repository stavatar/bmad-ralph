# Синтез аналитических исследований (8 отчётов)

**Дата:** 2026-03-07
**Контекст:** bmad-ralph — Go CLI, оркестрирующий Claude Code сессии для автономной разработки.
Исследуется переход от зависимости от BMad к самодостаточности (Вариант D: ralph plan).

**Источники:**
1. `v2-learnplatform-audit.md` — Аудит learnPlatform (реальный проект)
2. `v2-bmad-story-generation.md` — Pipeline генерации stories в BMad
3. `v2-gastown-orchestrators.md` — Gastown + фреймворки (LangGraph, CrewAI, etc.)
4. `v2-quality-comparison.md` — Качество Bridge vs Stories vs PRD
5. `v2-risk-analysis.md` — Анализ рисков всех вариантов
6. `v2-migration-roadmap.md` — Roadmap миграции (эпики, stories)
7. `v2-competitive-landscape.md` — Конкурентный ландшафт (20 инструментов)
8. `v2-bmad-workflow-replication.md` — Репликация BMad workflows

---

## Оглавление

1. [Аудит реального проекта (learnPlatform)](#1-аудит-реального-проекта-learnplatform)
2. [Качество генерации — Bridge vs Stories vs PRD](#2-качество-генерации--bridge-vs-stories-vs-prd)
3. [Конкурентный ландшафт — 20 инструментов](#3-конкурентный-ландшафт--20-инструментов)
4. [Gastown и мульти-агентные фреймворки](#4-gastown-и-мульти-агентные-фреймворки)
5. [BMad pipeline и workflow replication](#5-bmad-pipeline-и-workflow-replication)
6. [Анализ рисков](#6-анализ-рисков)
7. [Roadmap миграции](#7-roadmap-миграции)
8. [Точки расхождения между агентами](#8-точки-расхождения-между-агентами)
9. [Консолидированные выводы и рекомендации](#9-консолидированные-выводы-и-рекомендации)

---

## 1. Аудит реального проекта (learnPlatform)

**Источник:** `v2-learnplatform-audit.md`

### 1.1 Характеристика проекта

learnPracticsCodePlatform — образовательная платформа для практики Java.
Greenfield, medium complexity, стек: NestJS + React + Prisma/PostgreSQL + Docker sandbox + Gemini AI.

| Метрика | Значение |
|---------|----------|
| Документов (MD) | 46 |
| FR (функциональных требований) | 51 |
| NFR | 6 категорий |
| User Journeys | 5 |
| Эпиков | 9 (0-8) |
| Stories | 28 (в sprint-artifacts) |
| Задач в sprint-tasks.old | 93 |
| Validation Reports | 9 |
| Скриншотов | 65 |
| Общий объём документации | ~1.06 MB (MD+YAML+HTML) |
| Readiness Score | 8.1/10 (READY) |

### 1.2 Сильные стороны BMad-артефактов

1. **100% FR трассируемость:** все 51 FR привязаны к конкретным stories через FR Coverage Map в epics.md. Ни один FR не потерян. Это лучший показатель, зафиксированный в BMad-проектах.

2. **Консистентность между документами:** PRD <-> Architecture <-> Epics <-> Stories согласованы, подтверждено 3 раундами readiness review.

3. **BDD Acceptance Criteria:** единый формат Given/When/Then во всех stories с конкретными примерами данных (JSON payloads, error messages, HTTP status codes).

4. **Детальные Dev Notes** (Technical Notes) во всех stories: ссылки на Architecture секции, конкретные команды (`npm create vite@latest`, `npx prisma migrate dev`), предупреждения о зависимостях.

5. **Party Mode review:** multi-reviewer валидация выявила ~45 проблем, все исправлены.

6. **Граф зависимостей эпиков:** линейный с одной точкой параллелизма (Epic 4 || Epic 5), без циклов.

### 1.3 Системные проблемы

#### 1.3.1 sprint-tasks.old.md — 100% битых ссылок

**Все 93 source-ссылки используют несуществующий путь:**

```
source: stories/0-1-monorepo-scaffold.md#build
Реальный путь: docs/sprint-artifacts/0-1-turborepo-monorepo-scaffold.md
```

Три уровня проблемы:
- Директория `stories/` не существует — реальные файлы в `docs/sprint-artifacts/`
- 23 из 34 имён файлов сокращены/искажены (68% неточных имён, 32% совпадений)
- Якоря (`#fragment`) произвольные, не соответствуют заголовкам Tasks или AC-номерам

**Вывод:** Bridge генерирует source-ссылки на основе внутреннего маппинга, а не реальной файловой системы. Это фундаментальный дефект: source-ссылки не верифицируемы ни программно, ни вручную.

#### 1.3.2 Расхождение в количестве stories

| Источник | Количество |
|----------|-----------|
| epics.md Summary | 31 story |
| sprint-status.yaml (реальные файлы) | 34 story |
| sprint-artifacts/ директория | 28 story-файлов + 9 validation reports |

Расхождение = 3 stories добавлены после создания epics.md (вероятно 4.4, 5.4, 0.7). Мета-данные не синхронизированы.

#### 1.3.3 Однострочный формат задач — антипаттерн

93 задачи в sprint-tasks.old.md — каждая в одну строку. Средний размер: 633 символа. 57% задач (53 из 93) превышают 500 символов.

Примеры мегазадач:
- OnboardingOverlay: 1636 символов — по сути 5-7 отдельных подзадач
- useAdminStats: 1420 символов — полный хук с 4 API-вызовами
- DraftsPage: 1293 символов — целая страница с роутингом, фильтрами, группировкой

#### 1.3.4 Документы без ссылок

~530 KB HTML-файлов UX-дизайна (5 файлов, из них 3 с `-old` суффиксом) не referenced ни из stories, ни из architecture. 65 скриншотов не referenced из MD-документов. Validation reports (~75 KB) — одноразовые артефакты, после валидации не перечитываются.

#### 1.3.5 Монолитный epics.md

155 KB / 2365 строк в одном файле — плохо для контекстного окна любой LLM. Сравнение: bmad-ralph использует sharded подход (отдельные epic-N-*.md файлы).

### 1.4 Сравнение с bmad-ralph

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

### 1.5 Выводы из аудита

Аудит learnPlatform подтверждает как ценность BMad (100% FR трассируемость, BDD AC, consistent documents), так и системные проблемы Bridge-подхода (выдуманные source-пути, мегазадачи, потеря Dev Notes). Эти проблемы не специфичны для одного проекта — они структурные и воспроизводятся при любом использовании Bridge.

---

## 2. Качество генерации — Bridge vs Stories vs PRD

**Источник:** `v2-quality-comparison.md`

### 2.1 Распределение задач по размеру

```
<50:      0  |
50-99:    1  |#
100-199:  5  |#####
200-299: 11  |###########
300-499: 23  |#######################
>500:    53  |#####################################################
```

| Метрика | Значение |
|---------|----------|
| Минимум | 61 символов |
| Максимум | 1636 символов |
| Среднее | 633 символа |
| Медиана | ~500 символов |
| Оптимальный диапазон (200-400 симв.) | 37% задач |
| Перегруженные (>400 симв.) | 57% задач |

**37% задач в оптимальном диапазоне, 57% перегружены.** Bridge агрессивно мержит задачи из stories, теряя атомарность.

### 2.2 Конкретный пример потери гранулярности

**Story 1-1 (Auth Module): 12 Tasks -> 2 Tasks**

Экстремальная компрессия. 12 детальных задач сжаты в 2 мегазадачи:
- Задача 1: PrismaModule (корректно, 1 из 12)
- Задача 2: ВСЁ ОСТАЛЬНОЕ — AuthModule, DTO, Service, Strategy, Guards, Controller, env validation, тесты — в одной гигантской задаче на ~800 символов

Потери:
- Разделение на unit tests и E2E tests — утеряно (слиты)
- JWT Strategy как отдельный компонент — утерян
- CORS configuration — утеряно полностью
- Env validation — упомянуто, но не как отдельный шаг

### 2.3 Шаблон потерь при Bridge-компрессии (83% потеря гранулярности)

1. **Тестовые задачи** — unit и E2E тесты либо сливаются, либо исчезают
2. **Verification/verify задачи** — проверочные шаги теряются (Story 0-1: Task 6 "Verify full build" полностью исчезла)
3. **Env/config задачи** — мелкие конфигурационные задачи поглощаются
4. **Architectural constraints** (Dev Notes) — паттерны вроде "Named exports" не попадают в задачи
5. **Dependencies to install** — секция Dev Notes полностью игнорируется
6. **Project structure notes** — не попадают в задачи

### 2.4 Dev Notes blindness — ключевая проблема

**Bridge не извлекает Dev Notes из stories.** Он работает только с AC и Tasks, игнорируя самую ценную часть story — конкретные технические решения.

Примеры потерянных Dev Notes из Story 0-1:
- Tailwind CSS v4 (не упомянут в задачах)
- shadcn/ui (не упомянут)
- Jest для NestJS (не упомянут)
- Named exports Architecture Pattern #1 (не упомянут)

Покрытие Dev Notes -> Tasks: 11/15 = 73%. 4 элемента потеряны.

### 2.5 Оценки по 4 метрикам

| Метрика | Bridge | Stories | PRD Direct |
|---------|--------|---------|------------|
| Task Clarity (понятно ли что делать) | 5/10 | 9/10 | 7/10 |
| Task Atomicity (одна задача = одна сессия) | 3/10 | 8/10 | 6/10 |
| Task Completeness (все requirements покрыты) | 6/10 | 9/10 | 7/10 |
| Source Accuracy (source-ссылки корректны) | 2/10 | N/A | 8/10 |
| **Среднее** | **4.0** | **8.7** | **7.0** |

**Bridge (4.0) уступает обоим альтернативам:**
- Stories (8.7) дают наилучшее качество, но требуют отдельного шага создания
- PRD Direct (7.0) — золотая середина при условии включения "Dev Hints"

### 2.6 Рекомендация по оптимальной стратегии

**PRD + Architecture -> Tasks напрямую, но с "Dev Hints"** — компактными техническими решениями, которые сейчас живут в Dev Notes stories. Это позволяет:
- Избежать потерь при Bridge-компрессии
- Сохранить implementation details
- Получить атомарные задачи правильного размера
- Иметь точные source-ссылки (на PRD FR, Architecture sections)

---

## 3. Конкурентный ландшафт — 20 инструментов

**Источник:** `v2-competitive-landscape.md`

### 3.1 Пять тиров инструментов

#### Tier 1: Автономные агенты (5 инструментов)

| Инструмент | Цена | Планирование | Multi-agent | Ключевая фича |
|-----------|------|-------------|-------------|---------------|
| **Devin** (Cognition) | $20+/мес + ACU | Автономное | Параллельные сессии | Облачная IDE, 83% улучшение Devin 2.0 |
| **Codex** (OpenAI) | $20-200/мес | Sandbox/задачу | Да (март 2026) | Skills, GPT-5.2-Codex, облако + CLI |
| **Claude Code** (Anthropic) | $20-200/мес | Инкрементальное | Agent Teams (эксп.) | 1M контекст, $1B ARR за 6 мес |
| **Amazon Q** | $19/мес | Автономное | Нет | SWE-bench 66%, Java миграции |
| **Jules** (Google) | $0-125/мес | Асинхронное | Нет | Suggested Tasks (проактивное сканирование) |

#### Tier 2: IDE-интегрированные агенты (5 инструментов)

| Инструмент | Цена | Ключевая фича |
|-----------|------|---------------|
| **Cursor** | $0-20+/мес | До 8 параллельных агентов, завершение < 30 сек |
| **Windsurf** (Cognition) | $15/мес | Cascade, persistent knowledge layer, #1 AI Dev Tool Rankings |
| **GitHub Copilot** | $10-39/мес | Git-native (draft PR), security scanning |
| **Kiro** (AWS) | Preview (бесплатно) | **Spec-driven development** — промпт -> user stories + design doc + task list |
| **Junie** (JetBrains) | $100-300/год | Глубокая интеграция с JetBrains IDE |

#### Tier 3: CLI/Framework агенты (4 инструмента)

| Инструмент | Цена | Ключевая фича |
|-----------|------|---------------|
| **Aider** | Бесплатно + API | Architect mode (2-модельный: планирование + выполнение) |
| **SWE-Agent** (Princeton/Stanford) | Бесплатно + API | Custom ACI, Mini-SWE-Agent 100 строк Python |
| **OpenHands** (ex-OpenDevin) | Бесплатно + API | Расширяемая платформа, Docker sandbox |
| **AutoCodeRover** (-> SonarSource) | Часть Sonar | Fix bugs found by static analysis |

#### Tier 4: Мульти-агентные фреймворки (3 инструмента)

| Инструмент | Stars | Ключевая фича |
|-----------|-------|---------------|
| **MetaGPT** | ~ICLR 2025 | SOP-based, Code = SOP(Team), MGX — первая AI-команда |
| **CrewAI** | ~26k | 1.4B agentic automations, PwC/IBM/NVIDIA |
| **Claude Code Agent Teams** | встроен | Peer-to-peer (не hub-and-spoke), ~$7.80/задачу |

#### Tier 5: Методологии и Workflow (3 инструмента)

| Инструмент | Ключевая фича |
|-----------|---------------|
| **BMad Method** | YAML workflows, 8 ролей, 2-фазный lifecycle |
| **Taskmaster AI** | PRD -> tasks с зависимостями, 90% снижение ошибок с Cursor |
| **Claude Code Skills** | Marketplace, organization-level deployment, авто-matching |

### 3.2 Уникальная ниша ralph

Ralph находится на пересечении нескольких категорий, но ни одна не покрывает его полностью:

```
                    Автономность
                        ^
                        |
        Devin --------- | --------- Codex
                        |
        Jules --------- | --------- Claude Code (raw)
                        |
                    ralph <--- тут
                        |
        Kiro ---------- | --------- Cursor
                        |
        Taskmaster ---- | --------- Aider
                        |
                        +--------------------------------> Структурированность
```

**ralph = "structured autonomous loop":**
- Более структурирован, чем raw Claude Code (формализованные tasks, gates, reviews)
- Менее "облачный", чем Devin/Codex (CLI, локальный контроль)
- Более автономный, чем Kiro (полный loop: task -> execute -> review -> learn)
- Более методологичный, чем Aider/Taskmaster (BMad Method интеграция)

**Формула позиционирования:**
> ralph = structured task loop + adversarial code review + knowledge lifecycle + context observability

### 3.3 Фичи без аналогов

| Фича ralph | Ближайший конкурент | Разница |
|-----------|-------------------|---------|
| Context window observability (FR75-FR92) | Нет прямых аналогов | **Уникальная фича** |
| Knowledge extraction + distillation | Claude Code (.claude/ + memory) | ralph формализует цикл обучения |
| Per-task human gates (approve/reject/modify) | Copilot (review PR) | ralph гранулярнее |
| Adversarial code review (5 агентов) | Copilot (security scanning) | ralph по code quality, не security |

### 3.4 Конкурентные угрозы

**Критические (могут "убить" ralph):**

1. **Claude Code Agent Teams** — нативная мульти-агентная координация от Anthropic. Если добавят task persistence и review loop, ralph теряет значительную часть value proposition. Статус: экспериментальный (февраль 2026), API может измениться.

2. **Kiro** (AWS) — spec-driven подход максимально близок к ralph. Промпт -> user stories + acceptance criteria + design doc + task list. Если Kiro добавит review pipeline и knowledge extraction, пересечение будет критическим.

**Среднесрочные:**

3. **Taskmaster AI** — task decomposition + context management. Фокус на тех же проблемах (потеря контекста в больших проектах), другой подход (плагин к IDE, не CLI).

4. **OpenAI Codex Skills** — если skills ecosystem разовьёт BMad-подобные workflows, ralph станет менее нужен.

### 3.5 Стратегические рекомендации из конкурентного анализа

1. **Усилить Context window observability** — единственная фича без конкурентов. Развивать в направлении cost optimization.

2. **Knowledge extraction loop** — формализованное обучение (extraction -> distillation -> injection) не имеет аналогов. Это конкурентное преимущество.

3. **Niche positioning:** ralph — не "ещё один AI coding agent". ralph — **autonomous development loop orchestrator** со встроенным quality assurance, knowledge management и observability. Ни один конкурент не покрывает все три аспекта.

4. **Самодостаточность = survival:** все конкуренты (Devin, Codex, Claude Code, Kiro, Aider) самодостаточны. Зависимость ralph от BMad = adoption barrier = конкурентный недостаток.

---

## 4. Gastown и мульти-агентные фреймворки

**Источник:** `v2-gastown-orchestrators.md`

### 4.1 Gas Town (Steve Yegge)

**Характеристика:** Система оркестрации 20-30 параллельных Claude Code агентов. Go, ~11k stars, экспериментальный. 75k строк, 2000 коммитов, "100% vibecoded" за ~17 дней.

**Ключевые концепции:**

| Концепция | Описание | Релевантность для ralph |
|-----------|---------|------------------------|
| **Beads** | Атомарные единицы работы, JSON в Git | Аналог story files, но с формализованным ID |
| **Convoys** | Группировка задач для агентов | Аналог sprint-tasks.md |
| **Mayor** | Координатор, НЕ пишет код | Нет аналога (ralph сам является координатором) |
| **Polecats** | Временные рабочие, "умирают" после задачи | Аналог одиночных Claude Code сессий в ralph |
| **Witness** | Супервизор, мониторит и подталкивает | Нет аналога, потенциально полезный паттерн |
| **Refinery** | Merge queue, разрешение конфликтов | Нет аналога, потенциально полезный |
| **Seancing** | Воскрешение знаний предыдущих сессий | Аналог knowledge distillation в Epic 6 |

**Стоимость:** $2,000-$5,000/мес, ранний пользователь сообщил о $100/час.

**Критика (Maggie Appleton):**
- "Количество перекрывающихся и ad hoc концепций ошеломляет"
- Компоненты произвольны (polecats, convoys, molecules, deacons, witnesses, protomolecules)
- Vibecoding-подход жертвует связностью ради экспериментов

**Фундаментальное наблюдение:** когда агенты берут на себя реализацию, **дизайн становится узким местом** — скорость ограничена человеческой способностью к планированию, а не скоростью кодирования.

### 4.2 Goosetown (Block/Square)

Надстройка над Goose (~27k stars), вдохновлена Gas Town. Более лёгкий подход: каждый subagent работает в чистом контексте. **Town Wall** — append-only лог координации.

### 4.3 Универсальные фреймворки — что релевантно для ralph

| Фреймворк | Stars | Что полезно для ralph | Что НЕ нужно |
|-----------|-------|-----------------------|-------------|
| **CrewAI** | ~26k | Planning agent (автопланирование), Role-based teams | Python-only, general-purpose |
| **LangGraph** | ~12k | Checkpointing (durable execution), interrupt() для HITL | Зависимость от LangChain, DAG-сложность |
| **MS Agent Framework** | ~40k | Event-driven архитектура, Observability | Перманентная миграция API, enterprise-overhead |
| **Agency Swarm** | ~4k | Multi-LLM через LiteLLM | Привязка к OpenAI SDK |
| **PraisonAI** | ~5.5k | A2A Protocol | Meta-framework, менее зрелый |

### 4.4 Паттерны оркестрации

| Паттерн | Кто использует | Применимость к ralph |
|---------|---------------|---------------------|
| Sequential Pipeline | CrewAI, OMC, **bmad-ralph** | **Текущий паттерн ralph** — предсказуемость, простота |
| DAG (графы) | LangGraph, MS Agent Framework | Избыточен для ralph |
| Event-Driven | AutoGen v0.4, MS Agent Framework | Сложно предсказать поведение |
| Hierarchical Supervision | Gas Town, Claude Code Teams | Bottleneck на Mayor, стоимость supervisor-агентов |
| Swarm / Task Pool | OMC, OpenAI Swarm | Merge conflicts, координация доступа |

### 4.5 Почему massive parallelism НЕ для ralph

1. **Стоимость:** Gas Town $2-5k/мес vs ralph ~$20-200/мес (API costs). 10-25x разница.
2. **Merge conflicts:** 20-30 параллельных агентов создают больше конфликтов, чем решают задач.
3. **Design bottleneck:** скорость ограничена планированием, не кодированием.
4. **Предсказуемость:** sequential pipeline ralph предсказуемее и проще в отладке.
5. **Vibecoding-подход Gas Town** жертвует связностью — ralph ценит тестируемость и 100% fix rate.

### 4.6 Идеи для заимствования

- **Beads-подобная система:** атомарные единицы работы с persistent ID. ralph уже использует story files, но без формализованного ID-tracking.
- **Seancing (Gas Town):** восстановление контекста предыдущих сессий — аналог knowledge distillation, уже реализовано в Epic 6.
- **LangGraph checkpointing:** durable execution с точным восстановлением при крэше mid-task. ralph теряет состояние при крэше.
- **OMC Ecomode:** smart model routing для экономии (30-50% экономия токенов). ralph использует одну модель.

### 4.7 Чего НЕ заимствовать

- Сложность Gas Town (polecats, molecules, protomolecules) — избыточная терминология
- 20-30 параллельных агентов — $100/час неоправдано
- Привязка к одному LLM-провайдеру
- Event-driven архитектура — непредсказуемость поведения

### 4.8 Task persistence — сравнение подходов

| Подход | Реализация | Плюсы | Минусы |
|--------|-----------|-------|--------|
| Git-backed JSON | Gas Town beads | Версионирование, persistent | Merge conflicts |
| Database checkpoints | LangGraph + Postgres | Надёжно, queryable | Требует инфраструктуру |
| In-memory + memory tiers | CrewAI | Простота | Потеря при крэше |
| **File-based state** | **bmad-ralph** | **Простота, git-trackable** | **Нет rich querying** |
| Worktree isolation | Claude Code Teams | Git-native изоляция | Сложный merge |

ralph's file-based approach (story files + sprint-status.yaml) находится в оптимальной точке между простотой и функциональностью для CLI-инструмента.

### 4.9 Тренд 2026: два лагеря

1. **General-purpose frameworks** (CrewAI, LangGraph, MS Agent Framework) — широкий охват, не заточены под код
2. **Code-specific orchestrators** (Gas Town, Claude Code Teams, Goosetown, OMC) — оптимизированы для кодовых workflow

ralph уверенно в лагере code-specific orchestrators с уникальным фокусом на quality assurance + knowledge lifecycle.

---

## 5. BMad pipeline и workflow replication

**Источники:** `v2-bmad-story-generation.md`, `v2-bmad-workflow-replication.md`

### 5.1 Полный каталог BMad Workflows

#### Суммарный объём промптов: ~29600 строк

| Категория | Строк инструкций | Строк чеклистов | Итого |
|-----------|-----------------|-----------------|-------|
| Фаза 1 (анализ) | ~4900 | 0 | ~4900 |
| Фаза 2 (планирование) | ~5275 | 0 | ~5275 |
| Фаза 3 (solutioning) | ~3131 | 169 | ~3300 |
| Фаза 4 (implementation) | ~2848 | 750 | ~3598 |
| Вспомогательные | ~9246 | ~3283 | ~12529 |
| **ИТОГО** | **~25400** | **~4202** | **~29600** |

#### Детализация по workflows (Фаза 4 — самая релевантная)

| Workflow | Инструкции (строк) | Checklist | Интерактивность | Нужен ralph? |
|----------|-------------------|-----------|-----------------|-------------|
| sprint-planning | 234+33+56 | 33 | Низкая | Частично (уже есть ScanTasks) |
| create-story | 323+358+51 | 358 | Низкая | Рассмотреть (для plan mode) |
| dev-story | 405+80 | 80 | Низкая | **Уже есть** (ralph execute) |
| code-review | 237 | 0 | Низкая | **Уже есть** (ralph review) |
| correct-course | 206+279 | 279 | Высокая | Рассмотреть (ralph replan) |
| retrospective | 1443 | 0 | Средняя | Опционально |

### 5.2 Pipeline генерации stories — полная цепочка

```
PRD.md + Architecture.md + [UX.md]
        |
        v
  create-epics-and-stories (Phase 3)
  Промпт: 387 строк instructions + 80 строк template = 467 строк
  4 шага: Валидация -> Структура эпиков -> Stories -> Финальная валидация
        |
        v
  sprint-planning (Phase 4)
  Промпт: ~285 строк
  Выход: sprint-status.yaml
        |
        v
  create-story (Phase 4, повторяется)
  Промпт: 323 + 358 + 51 = 732 строки
  6 шагов: Target -> Artifacts -> Architecture -> Web -> Create -> Update
        |
        v
  validate-create-story (опционально)
  LLM-as-judge: свежий LLM проверяет output другого LLM
        |
        v
  dev-story -> code-review (Phase 4)
```

### 5.3 89% промптов НЕ нужны ralph

| Категория | Строк | Обоснование |
|-----------|-------|-------------|
| Pre-implementation (product-brief, research, prd, UX, architecture) | ~13475 | Domain пользователя, требует интерактивный диалог |
| Визуализация (diagrams x4) | ~645 | Excalidraw-специфичны |
| BMad infrastructure (workflow-status, workflow-init) | ~741 | Мета-координация BMad engine |
| Test Architecture (testarch x8) | ~9610 | QA планирование, domain пользователя |
| Quick Flow | ~245 | ralph сам является "quick flow" |
| Document Project | ~1626 | Разовая задача |
| **Итого НЕ встраивать** | **~26342** | **89% всех промптов BMad** |

### 5.4 Оставшиеся 11% — уже покрыты runner/review

| BMad Workflow | Ralph эквивалент | Покрытие |
|--------------|-----------------|----------|
| dev-story (405 строк) | ralph execute (runner + execute.md 130 строк) | **~90%** |
| code-review (237 строк) | ralph review (review.md 176 строк + 5 agents ~182 строки) | **~95%** |
| create-story (732 строки) | ScanTasks + execute prompt context | **~80%** |
| sprint-planning (285 строк) | ScanTasks + tasks.md management | **~70%** |

**ralph review даже мощнее BMad:** 5 специализированных review agents vs 1 monolithic BMad reviewer.

### 5.5 Коэффициент сжатия: 9-12x

BMad промпты содержат огромное количество formatting, emoji, party mode, checkpoint протокола. При портировании в ralph:

| Для чего | Строк из BMad | Строк в ralph (после сжатия) |
|----------|--------------|------------------------------|
| plan mode (декомпозиция) | 387 | ~60-80 |
| replan mode (correct-course) | 485 | ~60-80 |
| retro mode (ретроспектива) | 1443 | ~50-80 |
| knowledge extraction | 60 | ~20-30 |
| **Итого** | **2375** | **~190-270** |

### 5.6 Почему НЕ встраивать BMad workflows

1. **89% промптов не нужны ralph** — pre-implementation, визуализация, QA planning, infrastructure.

2. **Оставшиеся 11% уже покрыты** — ralph execute/review покрывают dev-story/code-review на 90-95%.

3. **Промпты BMad не переносимы напрямую** — оптимизированы под BMad workflow engine (checkpoints, party mode, template-output). Ralph использует другую модель: один промпт на сессию Claude Code.

4. **Ralph и BMad — complementary tools:**
   - BMad: планирование, design, story creation (высокоинтерактивные)
   - Ralph: автономное выполнение задач (низкоинтерактивный loop)
   - Граница чёткая: BMad создает артефакты -> Ralph их исполняет

5. **3 потенциальных новых режима (plan/replan/retro) — маргинальная ценность:** можно создать tasks.md вручную, отредактировать tasks.md, просмотреть метрики через session logs.

### 5.7 Единственная фича с ROI > 1

`ralph plan` как lightweight декомпозитор задач из краткого описания в tasks.md:
- ~70 строк промпта
- ~250 LOC Go-кода
- Заменяет как BMad create-epics-and-stories (для простых случаев), так и bridge

---

## 6. Анализ рисков

**Источник:** `v2-risk-analysis.md`

### 6.1 Варианты эволюции

| ID | Название | Суть |
|----|----------|------|
| **A** | Bridge упрощается | Программный парсинг AC, LLM убирается из bridge |
| **B** | Bridge убирается, runner со stories | Runner читает story файлы напрямую |
| **C** | Stories убираются, ralph с epics | Epic файл = единица работы |
| **D** | ralph plan (самодостаточный) | Новая команда: PRD/текст/issues -> sprint-tasks.md -> runner |
| **D+** | ralph plan + init + replan | Полный вариант D с инициализацией и перепланированием |

### 6.2 Ранжирование по общему риску

| Ранг | Вариант | Средний score | Оценка |
|------|---------|---------------|--------|
| 1 | **A** (программный парсинг) | **4.1** | НИЗКИЙ |
| 2 | **D** (ralph plan) | **5.4** | СРЕДНИЙ |
| 3 | **B** (runner со stories) | **7.6** | ВЫСОКИЙ |
| 4 | **D+** (plan + init + replan) | **8.5** | СРЕДНИЙ-ВЫСОКИЙ |
| 5 | **Status Quo** | **16.7** | ВЫСОКИЙ |
| 6 | **C** (epics) | **18.4** | КРИТИЧЕСКИЙ |

### 6.3 Детализация по категориям

| Категория | A | B | C | D | D+ | Status Quo |
|-----------|---|---|---|---|----|----|
| Технические | 5.3 | 9.8 | 19.3 | 5.6 | 8.6 | — |
| Продуктовые | 3.3 | 5.0 | 16.7 | 6.0 | 11.3 | — |
| Реализация | 4.0 | 12.0 | 20.0 | 5.8 | 8.3 | — |
| **Общий средний** | **4.1** | **7.6** | **18.4** | **5.4** | **8.5** | **16.7** |

### 6.4 Самые опасные риски варианта D и митигации

| Риск | Score | Описание | Митигация |
|------|-------|---------|-----------|
| **D-R2** | **12** | Промпт quality для plan.md: произвольный текст -> задачи, Claude может "додумывать" | Итерации: v1 -> тест на 5+ реальных inputs -> v2. `--review` флаг для human approval |
| **D-P3** | **12** | Scope creep: plan -> stories -> architecture -> ralph = BMad | Жёсткий scope: ralph plan = input -> sprint-tasks.md. НЕ генерировать stories, architecture |
| D-T2 | 9 | Новая точка отказа: LLM-генерация задач из произвольного текста менее предсказуема | Golden file тесты, `--dry-run` для preview |
| D-T5 | 8 | Backward compatibility sprint-tasks.md | Format contract тот же, runner не знает кто создал файл |
| D-T1 | 6 | Регрессия качества: plan.md без BMad-правил (AC Classification) | Granularity rules переиспользуются, format contract сохранён |

### 6.5 Риски НЕ делать ничего (Status Quo: score 16.7)

| Риск | Score | Описание |
|------|-------|---------|
| **SQ-7** | **25** | ralph без BMad неработоспособен — нет альтернативного способа создать sprint-tasks.md |
| **SQ-1** | **20** | Bridge остаётся bottleneck: 55-130 мин от идеи до первого коммита vs 2-5 мин у конкурентов |
| **SQ-4** | **20** | Adoption barrier: новый пользователь должен освоить BMad + ralph (7 концепций, 15 файлов) |
| SQ-3 | 16 | Конкуренты обгоняют: Devin, Kiro, Aider — все самодостаточны |
| SQ-6 | 12 | "Испорченный телефон" (3 AI-слоя): BMad -> bridge -> runner |
| SQ-2 | 12 | Зависимость от BMad: изменения story формата ломают bridge промпт |
| SQ-5 | 12 | Batching антипаттерн: 6 изолированных Claude-сессий для planning |

**Вывод: status quo неприемлем долгосрочно.** 5 из 7 рисков не митигируются без архитектурных изменений.

### 6.6 Что ИСКЛЮЧИТЬ

- **Вариант C (epics) — ИСКЛЮЧИТЬ.** Критический риск (18.4) по всем категориям. Context window overflow + потеря human control. Любая попытка сделать C работоспособным приводит к воссозданию stories.

- **Вариант B напрямую — НЕ делать.** D даёт ту же ценность с меньшим риском. B ломает scan.go + execute.md (score 12 каждый), объём недооценён (score 12).

- **Вариант D+ до успеха D — НЕ делать.** Scope creep увеличивает риск без доказанной ценности.

### 6.7 Risk-adjusted рекомендация

**Вариант A** — минимальный риск (4.1), минимальная ценность. Решает только "испорченный телефон". НЕ решает зависимость от BMad, adoption barrier, time-to-first-task. Оптимизация, не эволюция.

**Вариант D** — оптимальный баланс риск/ценность (5.4). Решает ВСЕ ключевые проблемы status quo:
- Adoption barrier (plain text input)
- BMad зависимость (опциональна)
- Time-to-first-task (2-5 мин вместо 55-130 мин)

Главный риск (D-R2, score 12) — качество plan.md промпта — митигируется итеративно.

---

## 7. Roadmap миграции

**Источник:** `v2-migration-roadmap.md`

### 7.1 Целевой pipeline

**Текущий:**
```
BMad AI -> stories (*.md) -> ralph bridge -> sprint-tasks.md -> ralph run -> код
```

**Целевой:**
```
Требования (любой формат) -> ralph plan -> sprint-tasks.md -> ralph run -> код
```

### 7.2 Пять эпиков, 19 stories

| Epic | Stories | Размер | Зависимости | Приоритет | Оценка (дни) |
|------|---------|--------|-------------|-----------|-------------|
| **11: ralph plan** | 6 | L | нет | КРИТИЧЕСКИЙ | 2 |
| **12: ralph replan** | 4 | M | Epic 11 | ВЫСОКИЙ | 1 |
| **13: YAML format** | 4 | M | нет (13.3 -> Epic 11) | НИЗКИЙ | 1.5 |
| **14: ralph init** | 3 | M | нет | СРЕДНИЙ | 1 |
| **15: Bridge removal** | 2 | S | Epic 11 | ВЫСОКИЙ | 0.5 |
| **Итого** | **19** | — | — | — | **6 дней** |

### 7.3 Граф зависимостей эпиков

```
Epic 11 (ralph plan)  <-- КРИТИЧЕСКИЙ ПУТЬ
     |
     +-- Epic 12 (ralph replan) <-- зависит от 11, параллелится с 13
     |
     +-- Epic 13 (YAML формат) <-- ОПЦИОНАЛЕН, независим от 12
     |
     +-- Epic 15 (Bridge Deprecation) <-- зависит от 11

Epic 14 (ralph init)  <-- ПОЛНОСТЬЮ НЕЗАВИСИМ, параллельно с 11
```

### 7.4 Детализация Epic 11: ralph plan (КРИТИЧЕСКИЙ ПУТЬ)

| Story | Размер | Описание |
|-------|--------|---------|
| 11.1: Пакет planner/ scaffold | S | Заглушка Plan(), PlanPrompt(), embedded plan.md |
| 11.2: Document autodiscovery | M | DiscoverInputs(): файлы, stdin, autodiscovery |
| 11.3: Промпт plan.md | M | Go template, granularity rules, format contract |
| 11.4: Core planner.Plan() | M | Сборка промпта, вызов session, парсинг, merge mode |
| 11.5: CLI cmd/ralph/plan.go | S | Cobra команда, --stdin, exit codes |
| 11.6: Integration тесты | M | Mock Claude, merge mode, stdin mode |

### 7.5 MVP path (минимум для самодостаточности)

```
Epic 11 (2 дня) -> Epic 15 (0.5 дня) = 2.5 дня, 8 stories
```

Результат: `ralph plan` заменяет `ralph bridge`, bridge удалён.

### 7.6 Рекомендуемый path

```
Epic 11 (2 дня) -> Epic 15 (0.5 дня) -> Epic 12 (1 день) = 3.5 дня, 12 stories
        |                                                     + 3 stories (Epic 14)
        +-- параллельно ---> Epic 14 (1 день)                = 15 stories итого
```

Результат: `ralph plan` + `ralph replan` + `ralph init` + bridge удалён.

### 7.7 Что отложить и почему

**Epic 13 (YAML Task Format) — отложить:**
- Текущий markdown формат работает стабильно 10 эпиков, ~500 задач
- Regex-хрупкость не является реальной проблемой на практике
- GO/NO-GO решение через 1-2 месяца использования `ralph plan`

**Epic 14 (ralph init) — опционально:**
- Отложить если целевая аудитория — опытные разработчики
- Приоритет повышается при публичном релизе

### 7.8 Архитектурные решения roadmap

#### Почему новый пакет planner/, а не расширение bridge/

1. **Семантика:** bridge = "story -> task" (BMad-специфичный), planner = "requirements -> task" (универсальный)
2. **Чистое удаление:** bridge удаляется целиком в Epic 15
3. **SRP:** runner/ уже ~1200 LOC, планирование в runner нарушает single responsibility
4. **Dependency tree:** `cmd/ralph -> planner -> session, config` — параллельно bridge

#### Почему LLM для plan, а не программный парсинг

1. **Input непредсказуем:** plain text, PRD, markdown, GitHub Issues
2. **Декомпозиция = семантическая задача:** regex-парсером не разбить "добавить авторизацию" на атомарные задачи
3. **ADaPT research:** as-needed decomposition через LLM эффективнее upfront программного разбора
4. **Bridge = антипаттерн не из-за LLM**, а из-за лишнего AI-слоя. Plan убирает слой (BMad stories), не добавляет

#### Формат sprint-tasks.md сохраняется

1. **Runner не меняется:** 0 изменений в scan.go
2. **Человекочитаемость:** markdown чеклист легко читать и редактировать
3. **Проверено:** 10 эпиков, 80+ stories, ~500 задач — формат стабилен

### 7.9 Метрики успеха

1. ralph plan генерирует >= 90% задач, которые бы создал bridge (ручная проверка на 3 stories)
2. 0 тестов runner/ сломано после удаления bridge
3. Нетто-эффект: >= -1000 строк кода (упрощение)
4. Среднее quality findings <= 4.0/story (ниже текущего 3.3)
5. ralph plan работает с stdin (GitHub Issues, pipe content)

---

## 8. Точки расхождения между агентами

### 8.1 Где аналитики расходятся между собой

#### 8.1.1 Ценность BMad workflows для ralph

**Аналитик workflow-replication** занимает жёсткую позицию:
> "НЕ встраивать BMad workflows в ralph. Ralph и BMad — complementary tools."

**Аналитик story-generation** более нюансирован:
> "Суммарный объём для полного встраивания: ~900-1000 строк промптов. Минимальный MVP: ~400-500 строк."

Расхождение: первый считает встраивание ненужным, второй — технически выполнимым с умеренным объёмом. Оба согласны, что `ralph plan` — единственная фича с ROI > 1.

#### 8.1.2 Оценка покрытия create-story

**Аналитик workflow-replication** оценивает покрытие в 80%:
> "ralph уже покрывает create-story на ~80% через execute prompt context"

**Аналитик story-generation** детализирует:
> "Промпт create-story содержит 6 шагов: artifacts, architecture, git, web, compose, validate. ralph покрывает 4 из 6."

Потенциально непокрытые шаги: web research (актуальные версии библиотек) и validate (LLM-as-judge).

#### 8.1.3 Нужен ли ralph replan?

**Аналитик risks** осторожен:
> "Replan score 8.5 (D+), сначала D, потом D+ если D успешен"

**Аналитик roadmap** включает replan в рекомендуемый path:
> "3.5 дня, 15 stories, включая Epic 12 (replan)"

**Аналитик workflow-replication** скептичен:
> "replan — маргинальная ценность, можно просто отредактировать tasks.md"

Расхождение: от "обязательно" до "не нужно". Компромисс: включить в рекомендуемый, но не в MVP.

### 8.2 Где аналитики расходятся с архитекторами (из предыдущих исследований)

#### 8.2.1 Формат задач: Markdown vs YAML

**Аналитик roadmap** предлагает YAML как опцию (Epic 13):
> "YAML Task Format — машиночитаемый формат, устраняющий regex-хрупкость"

**Архитектурное решение проекта** (10 эпиков на markdown):
> "Формат sprint-tasks.md стабилен, 500+ задач без проблем"

**Аналитик risks** поддерживает архитектора:
> "YAML = migration burden. GO/NO-GO через 1-2 месяца"

Консенсус: отложить YAML. Но аналитик roadmap создал полный Epic 13 (4 stories), что может привести к scope creep при реализации.

#### 8.2.2 Вариант A vs D

**Аналитик risks** ранжирует A (score 4.1) выше D (score 5.4) по риску, но рекомендует D:
> "A — минимальный риск, минимальная ценность. D — оптимальный баланс риск/ценность."

Это нетривиальное решение: выбрать вариант с БОЛЕЕ ВЫСОКИМ риском ради стратегической ценности. Аналитики фактически приняли product-decision, не только аналитическое.

#### 8.2.3 Bridge как антипаттерн

Все аналитики согласны, что bridge — антипаттерн. Но расходятся в диагнозе:

| Аналитик | Диагноз |
|----------|---------|
| quality-comparison | "Dev Notes blindness" — bridge не видит Dev Notes |
| learnplatform-audit | "100% битых source-ссылок" — bridge генерирует выдуманные пути |
| risk-analysis | "Испорченный телефон" — 3 AI-слоя вместо 2 |
| workflow-replication | "Коэффициент сжатия 9-12x" — BMad промпты избыточны для ralph |

Все четыре диагноза верны и дополняют друг друга. Они описывают разные аспекты одной системной проблемы.

### 8.3 Спорные решения

#### 8.3.1 Самодостаточность vs каннибализация BMad

**Проблема:** ralph plan делает часть работы BMad (task decomposition). Это сознательная каннибализация?

**Аналитик risks:**
> "Каннибализация BMad: ralph plan делает часть работы BMad, но НЕ делает PRD/Architecture/Epics. Разные уровни абстракции."

**Аналитик competitive:**
> "Самодостаточность = survival. Все конкуренты самодостаточны."

**Компромисс:** ralph plan принимает stories как input (`ralph plan story1.md`). BMad workflow не ломается, появляется альтернатива.

#### 8.3.2 LLM для планирования vs программный парсинг

**За LLM (вариант D):**
- Input непредсказуем (plain text, PRD, GitHub Issues)
- Декомпозиция = семантическая задача
- ADaPT research: as-needed decomposition через LLM эффективнее

**За программный парсинг (вариант A):**
- Предсказуемость (regex vs LLM hallucinations)
- Тестируемость (golden files для regex, а не для LLM output)
- Score 4.1 vs 5.4 — программный парсинг объективно менее рискован

**Решение аналитиков:** D (LLM), потому что A не решает ключевые проблемы (adoption barrier, BMad зависимость).

#### 8.3.3 Количество новых эпиков

**Аналитик roadmap** предлагает 5 эпиков (11-15), 19 stories, 6 дней.
**Аналитик risks** рекомендует MVP (2 эпика, 8 stories, 2.5 дня).

Разрыв: 19 vs 8 stories. Аналитик roadmap создал "полный" roadmap с YAML и init, которые аналитик risks явно помечает как scope creep.

### 8.4 Открытые вопросы

1. **Качество plan.md промпта** — ни один аналитик не тестировал промпт на реальных данных. Весь анализ теоретический. Risk score D-R2 = 12 (самый высокий для D), но нет эмпирической базы.

2. **LLM-agnostic абстракция** — аналитик competitive рекомендует "рассмотреть LLM-agnostic абстракцию" для снижения vendor lock-in. Но ни один аналитик не оценил стоимость и реалистичность этой рекомендации. ralph жёстко привязан к Claude Code (session.go вызывает claude binary), абстракция потребует масштабного рефакторинга.

3. **Timing deprecation bridge** — roadmap предлагает "быстрый переход Phase 1->3 (2.5 дня)". Но если plan.md промпт окажется некачественным (risk D-R2), bridge придётся поддерживать параллельно неопределённое время.

4. **Тестирование plan.md на learnPlatform** — learnPlatform audit дал детальные данные о качестве BMad-артефактов. Идеальный test case: подать PRD learnPlatform в ralph plan и сравнить output с реальными 93 задачами. Ни один аналитик не предложил этот эксперимент.

5. **Claude Code Agent Teams vs ralph** — если Anthropic добавит task persistence и review loop в Agent Teams, ralph теряет значительную часть value. Нет contingency plan на этот сценарий.

6. **Context window observability как продукт** — все аналитики отмечают уникальность FR75-FR92, но никто не предлагает конкретный план монетизации или development roadmap для этой фичи.

7. **Реальная стоимость ralph vs конкурентов** — конкурентный анализ содержит цены ($20/мес Devin, $20-200/мес Claude Code), но нет оценки стоимости ralph pipeline (BMad сессии + bridge + run + review = N токенов = $X). Без этого сравнение неполно.

---

## 9. Консолидированные выводы и рекомендации

### 9.1 Факты, подтверждённые всеми 8 отчётами

1. **Bridge — антипаттерн** (подтверждено 4 отчётами из разных углов):
   - 100% битых source-ссылок (audit)
   - 57% мегазадач, 83% потеря гранулярности (quality)
   - Dev Notes blindness (quality)
   - "Испорченный телефон" — 3 AI-слоя (risk)
   - Коэффициент сжатия промптов 9-12x (workflow)

2. **Status quo неприемлем** (risk score 16.7):
   - ralph без BMad неработоспособен (SQ-7, score 25)
   - Adoption barrier (SQ-4, score 20)
   - Bottleneck bridge: 55-130 мин vs 2-5 мин у конкурентов (SQ-1, score 20)

3. **Вариант D (ralph plan) — оптимальный баланс** (risk score 5.4):
   - Решает все 3 ключевые проблемы status quo
   - Сохраняет sprint-tasks.md формат (0 изменений в runner)
   - Новый пакет planner/ параллельно bridge (чистое удаление потом)
   - MVP: 2.5 дня, 8 stories

4. **89% BMad промптов НЕ нужны ralph** — pre-implementation, визуализация, QA, infrastructure.

5. **Конкурентное преимущество ralph** — не в одной фиче, а в комбинации: structured task loop + adversarial code review + knowledge lifecycle + context observability. Ни один из 20 конкурентов не покрывает все 4 аспекта.

### 9.2 Рекомендуемый план действий

**Фаза 1 (MVP, 2.5 дня):**
- Epic 11: ralph plan (6 stories) — новый пакет planner/, промпт plan.md, CLI
- Epic 15: Bridge removal (2 stories) — deprecation + deletion

**Фаза 2 (рекомендуемая, +1.5 дня):**
- Epic 12: ralph replan (4 stories) — mid-sprint коррекция
- Epic 14: ralph init (3 stories) — quick start для новых пользователей

**Фаза 3 (отложить):**
- Epic 13: YAML format — GO/NO-GO через 1-2 месяца

**Что НЕ делать:**
- Вариант C (epics as work unit) — критический риск, score 18.4
- Вариант B (runner со stories) — высокий риск, score 7.6, D даёт то же дешевле
- Встраивание BMad workflows — 89% не нужны, остальные покрыты
- Massive parallelism (Gas Town style) — $100/час, непредсказуемо
- YAML формат до доказательства проблемы с markdown

### 9.3 Ключевые метрики для отслеживания

| Метрика | Порог успеха |
|---------|-------------|
| Качество plan output vs bridge | >= 90% покрытие задач |
| Сломанных runner тестов | 0 |
| Нетто-эффект строк кода | >= -1000 (упрощение) |
| Quality findings per story | <= 4.0 |
| Time-to-first-task | <= 5 мин (vs 55-130 мин сейчас) |

### 9.4 Риски для мониторинга

| Риск | Score | Триггер для эскалации |
|------|-------|----------------------|
| Качество plan.md промпта (D-R2) | 12 | Plan output значительно хуже bridge на 3+ test cases |
| Scope creep к BMad (D-P3) | 12 | Запросы на генерацию stories/architecture |
| Claude Code Agent Teams | — | Anthropic добавляет task persistence + review |
| Kiro spec-driven | — | AWS добавляет review pipeline + knowledge |
