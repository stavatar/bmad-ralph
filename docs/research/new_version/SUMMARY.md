# Эволюция Ralph v2 — Консолидированный итоговый отчёт

**Дата:** 2026-03-07
**Статус:** Финальный отчёт для принятия решения
**Основа:** 16 исследований (8 архитектурных + 8 аналитических отчётов)
**Контекст:** bmad-ralph v1 завершён (10 эпиков, 82 story, FR1-FR92). Планирование v2 — переход от BMad-зависимости к самодостаточности.

---

## Оглавление

1. [Executive Summary](#1-executive-summary)
2. [Текущая ситуация — что не так](#2-текущая-ситуация--что-не-так)
3. [Исследование индустрии](#3-исследование-индустрии)
4. [Анализ BMad Method](#4-анализ-bmad-method)
5. [Варианты решения — полное сравнение](#5-варианты-решения--полное-сравнение)
6. [Детальный дизайн Варианта D](#6-детальный-дизайн-варианта-d)
7. [Анализ рисков](#7-анализ-рисков)
8. [План реализации](#8-план-реализации)
9. [Что НЕ нужно делать](#9-что-не-нужно-делать)
10. [Точки расхождения между агентами](#10-точки-расхождения-между-агентами)
11. [Итоговый вердикт и рекомендации](#11-итоговый-вердикт-и-рекомендации)

---

## 1. Executive Summary

### Проблема

ralph v1 не может работать без BMad Method. Для запуска `ralph run` требуется:
1. Установить BMad Method (workflow engine, 8 ролей, 29600 строк промптов)
2. Пройти 4 workflow (PRD, Architecture, Epics, Stories) — 45-100 минут
3. Запустить `ralph bridge` для конвертации stories в задачи — 10-30 минут
4. Только после этого `ralph run` может начать писать код

**Time-to-first-task: 55-130 минут.** У конкурентов (Devin, Kiro, Aider, Claude Code): 1-5 минут. Разница: 10-100x.

Bridge при этом теряет 83% гранулярности задач, генерирует 100% битых source-ссылок, и полностью игнорирует Dev Notes — самую ценную часть stories.

### Решение

**Вариант D: `ralph plan`** — новая команда, принимающая любой текстовый input (PRD, plain text, GitHub Issues, BMad stories) и генерирующая `sprint-tasks.md` напрямую. BMad становится опциональным.

### Стоимость

- MVP (ralph plan + bridge removal): **2.5 дня, 8 stories**
- Рекомендуемый (+ ralph replan + ralph init): **3.5 дня, 15 stories**
- Нетто-эффект на codebase: **-1324 строки** (упрощение)
- Стоимость planning: $0.10-0.30 вместо $2.50-9.80 (**90-97% экономия**)

### Риски

Главный риск — качество plan.md промпта (risk score 12 по шкале вероятность*impact, максимум 25). Митигация: итеративная доработка, human review (`--dry-run`), golden file тесты. Status quo (ничего не делать) имеет средний risk score 16.7 — значительно хуже любого варианта действий.

### Вердикт

**Делать Вариант D.** Начать с MVP (Epic 11 + Epic 15), затем Epic 12 (replan) и Epic 14 (init). YAML формат задач (Epic 13) — отложить минимум на 1-2 месяца.

---

## 2. Текущая ситуация — что не так

### 2.1 Архитектура Bridge и её ограничения

#### Текущий pipeline

```
BMad AI (PRD -> Architecture -> Epics -> Stories)   [45-100 мин, $2-9]
  -> ralph bridge (Stories -> sprint-tasks.md)       [10-30 мин, $0.50-1.80]
    -> ralph run (Tasks -> Code)                     [5-50 мин/задача]
```

Bridge — это промежуточный слой между BMad stories и ralph runner. Он принимает story файлы (markdown с AC, Tasks, Dev Notes) и генерирует sprint-tasks.md — плоский список задач для `ralph run`.

#### Архитектура bridge в коде

```
bridge/
  bridge.go          // Bridge() — точка входа
  bridge_test.go
  prompts/
    bridge.md        // 244-строчный LLM промпт (embed)
```

Bridge зависит от пакетов `session` (вызов Claude CLI) и `config`. Он параллелен `runner` — оба вызываются из `cmd/ralph/`.

#### Пять фундаментальных проблем Bridge

**1. "Испорченный телефон" — 3 AI-слоя вместо 2.**

```
Пользователь -> BMad AI (слой 1) -> stories
stories -> ralph bridge (слой 2, LLM) -> sprint-tasks.md
sprint-tasks.md -> ralph run (слой 3, Claude Code) -> код
```

Каждый AI-слой вносит потери: BMad AI интерпретирует требования, bridge интерпретирует stories, runner интерпретирует задачи. Три последовательных интерпретации = накопление ошибок.

**2. Dev Notes blindness — bridge не видит Dev Notes.**

Bridge работает только с Acceptance Criteria и Tasks из stories, полностью игнорируя Dev Notes — секцию с конкретными техническими решениями. Dev Notes содержат:
- Архитектурные паттерны ("Named exports", "Repository pattern")
- Конкретные библиотеки и версии ("Tailwind CSS v4", "shadcn/ui")
- Команды установки ("npm create vite@latest", "npx prisma migrate dev")
- Предупреждения о зависимостях

Покрытие Dev Notes в output bridge: 73% — 4 из 15 элементов полностью потеряны (измерено на learnPlatform).

**3. Агрессивная компрессия — 83% потеря гранулярности.**

Bridge мержит несколько task'ов из story в одну мегазадачу. Пример из Story 1-1 (Auth Module):

```
Story: 12 Tasks (PrismaModule, AuthModule, DTO, Service, Strategy, Guards,
       Controller, env validation, unit tests, E2E tests, CORS, JWT)
Bridge: 2 Tasks (PrismaModule + ВСЁ ОСТАЛЬНОЕ)
```

Потери: разделение unit/E2E тестов — утеряно, JWT Strategy как компонент — утерян, CORS configuration — утеряно полностью.

**4. 100% битых source-ссылок.**

Все 93 source-ссылки в sprint-tasks.old.md проекта learnPlatform используют несуществующие пути:

```
source: stories/0-1-monorepo-scaffold.md#build
Реальный путь: docs/sprint-artifacts/0-1-turborepo-monorepo-scaffold.md
```

Три уровня проблемы:
- Директория `stories/` не существует — реальные файлы в `docs/sprint-artifacts/`
- 23 из 34 имён файлов сокращены/искажены (68% неточных имён)
- Якоря (`#fragment`) произвольные, не соответствуют реальным заголовкам

Bridge генерирует source-ссылки на основе внутреннего маппинга LLM, а не реальной файловой системы. Это фундаментальный дефект: ссылки не верифицируемы ни программно, ни вручную.

**5. Однострочный формат задач — антипаттерн.**

57% задач (53 из 93) превышают 500 символов — это "мегазадачи", содержащие по сути 3-7 подзадач в одной строке. Максимум: 1636 символов (OnboardingOverlay — 5-7 подзадач в одной строке).

#### Промпт bridge.md — 244 строки

Промпт bridge.md содержит 12 правил merge, format contract, source traceability инструкции, gate marking rules, prohibited formats, примеры Correct/WRONG — и всё это обрабатывается одним LLM вызовом. Исследования 2025 года показывают, что reasoning деградирует после ~3000 tokens промпта. bridge.md превышает этот порог.

### 2.2 Аудит реального проекта (learnPlatform) — конкретные цифры

learnPracticsCodePlatform — образовательная платформа для практики Java. Greenfield, medium complexity, стек: NestJS + React + Prisma/PostgreSQL + Docker sandbox + Gemini AI.

#### Масштаб проекта

| Метрика | Значение |
|---------|----------|
| Документов (MD) | 46 |
| FR (функциональных требований) | 51 |
| NFR | 6 категорий |
| User Journeys | 5 |
| Эпиков | 9 (0-8) |
| Stories | 28 (в sprint-artifacts) |
| Задач в sprint-tasks.old | 93 |
| Общий объём документации | ~1.06 MB |
| Readiness Score | 8.1/10 (READY) |

#### Сильные стороны BMad-артефактов

1. **100% FR трассируемость:** все 51 FR привязаны к stories через FR Coverage Map. Ни один FR не потерян.
2. **Консистентность между документами:** PRD <-> Architecture <-> Epics <-> Stories согласованы, подтверждено 3 раундами readiness review.
3. **BDD Acceptance Criteria:** единый формат Given/When/Then с конкретными примерами (JSON payloads, HTTP status codes).
4. **Детальные Dev Notes:** ссылки на Architecture секции, конкретные команды, предупреждения о зависимостях.
5. **Party Mode review:** multi-reviewer валидация выявила ~45 проблем, все исправлены.

#### Системные проблемы (обнаруженные при аудите)

**100% битых source-ссылок в sprint-tasks.old.md:**
- Директория `stories/` не существует
- 68% имён файлов искажены
- Якоря произвольные

**Расхождение в количестве stories:**

| Источник | Количество |
|----------|-----------|
| epics.md Summary | 31 story |
| sprint-status.yaml (реальные файлы) | 34 story |
| sprint-artifacts/ директория | 28 story-файлов + 9 validation reports |

3 stories добавлены после создания epics.md — мета-данные не синхронизированы.

**Монолитный epics.md:** 155 KB / 2365 строк в одном файле. bmad-ralph использует sharded подход (отдельные epic-N-*.md файлы) — значительно лучше для контекстного окна LLM.

**~530 KB HTML-файлов UX-дизайна** не referenced ни из stories, ни из architecture. 65 скриншотов не referenced из MD-документов. Validation reports (~75 KB) — одноразовые артефакты.

#### Сравнение с bmad-ralph

| Аспект | bmad-ralph | learnPlatform |
|--------|-----------|---------------|
| FR count | 92 (10 эпиков) | 51 (9 эпиков) |
| Stories per epic | 7-10 | 2-7 |
| Story format | отдельные файлы | отдельные файлы (+ inline в epics.md) |
| Validation | code-review workflow | validation reports (чеклисты) |
| sprint-tasks | sprint-status.yaml only | sprint-tasks.old.md (93 задачи) |
| Монолит epics | несколько файлов | один файл 155KB |

### 2.3 Качество генерации Bridge — метрики

#### Распределение задач по размеру

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

**37% задач в оптимальном диапазоне, 57% перегружены.** Bridge агрессивно мержит задачи, теряя атомарность.

#### Шаблон потерь при Bridge-компрессии

1. **Тестовые задачи** — unit и E2E тесты либо сливаются, либо исчезают
2. **Verification задачи** — проверочные шаги теряются (Story 0-1: Task 6 "Verify full build" полностью исчезла)
3. **Env/config задачи** — мелкие конфигурационные задачи поглощаются более крупными
4. **Architectural constraints** (Dev Notes) — паттерны вроде "Named exports" не попадают в задачи
5. **Dependencies to install** — секция Dev Notes полностью игнорируется
6. **Project structure notes** — не попадают в задачи

#### Оценки по 4 метрикам

| Метрика | Bridge | Stories напрямую | PRD Direct |
|---------|--------|-----------------|------------|
| Task Clarity (понятно ли что делать) | 5/10 | 9/10 | 7/10 |
| Task Atomicity (одна задача = одна сессия) | 3/10 | 8/10 | 6/10 |
| Task Completeness (все requirements покрыты) | 6/10 | 9/10 | 7/10 |
| Source Accuracy (source-ссылки корректны) | 2/10 | N/A | 8/10 |
| **Среднее** | **4.0** | **8.7** | **7.0** |

**Bridge (4.0) уступает обоим альтернативам.** Stories (8.7) дают лучшее качество, но требуют отдельного шага создания. PRD Direct (7.0) — золотая середина при условии включения "Dev Hints".

### 2.4 Зависимость от BMad — почему это проблема

#### Adoption barrier

Новый пользователь ralph должен:
1. Понять BMad Method (8 ролей, 12 типов документов, 4 фазы)
2. Установить и настроить BMad workflow engine
3. Создать 4-6 документов (PRD, Architecture, Epics, Stories)
4. Только после этого запустить ralph

Это **7 концепций и 15 файлов** до первого полезного результата. У Devin: "напиши в чат". У Aider: "запусти CLI". У Kiro: "опиши требования".

#### Конкурентная аномалия

Из 20 исследованных инструментов (Devin, Claude Code, Aider, SWE-Agent, OpenHands, AutoCodeRover, Cursor, Windsurf, MetaGPT, Kiro, Gastown, GitHub Copilot, Jules, Amazon Q, Codex, Junie, CrewAI, Taskmaster AI, Claude Code Agent Teams, BMad Method) **ни один** не требует внешнего workflow для создания входных данных, кроме ralph.

ralph с BMad-зависимостью — аномалия рынка.

#### Технический долг

Bridge — единственный пакет, который:
- Зависит от конкретного формата story файлов (BMad-специфичный markdown)
- Использует LLM для задачи, которую можно решить программно (парсинг + форматирование)
- Генерирует невалидные данные (100% битых ссылок)
- Имеет reliability ~85% при merge (LLM теряет [x] статусы)

### 2.5 Количественная сводка проблем текущей ситуации

| Проблема | Количественная оценка | Источник |
|----------|----------------------|----------|
| Битые source-ссылки | 93 из 93 (100%) | learnplatform-audit |
| Потеря гранулярности | 83% задач потеряны при компрессии | quality-comparison |
| Перегруженные задачи (>400 симв.) | 57% (53 из 93) | quality-comparison |
| Потеря Dev Notes | 27% полностью утеряны (4 из 15) | quality-comparison |
| Merge reliability | ~85% (LLM теряет [x] статусы) | variants-comparison |
| Time overhead | 55-130 мин (vs 2-5 у конкурентов) | competitive-landscape |
| Cost overhead | $2.50-9.80 на planning (vs $0.10-0.30 у plan) | variants-comparison |
| Bridge промпт | 244 строки (превышает ~3000 token порог) | prompt-chain-design |
| Документов BMad | 10 из 12 типов не используются runner | bmad-document-formats |
| BMad промптов | 89% (~26342 строк) не нужны ralph | workflow-replication |

### 2.6 Выводы по текущей ситуации

Четыре независимых исследования подтверждают с разных сторон, что bridge — антипаттерн:

| Аналитик | Диагноз | Источник |
|----------|---------|----------|
| quality-comparison | "Dev Notes blindness" — bridge не видит Dev Notes | v2-quality-comparison.md |
| learnplatform-audit | "100% битых source-ссылок" — bridge генерирует выдуманные пути | v2-learnplatform-audit.md |
| risk-analysis | "Испорченный телефон" — 3 AI-слоя вместо 2 | v2-risk-analysis.md |
| workflow-replication | "Коэффициент сжатия 9-12x" — BMad промпты избыточны для ralph | v2-bmad-workflow-replication.md |

Все четыре диагноза верны и дополняют друг друга. Они описывают разные аспекты одной системной проблемы: **bridge создаёт больше проблем, чем решает**.

Status quo неприемлем:
- ralph без BMad неработоспособен (risk score 25 — максимальный)
- Adoption barrier блокирует рост пользовательской базы (risk score 20)
- Bridge bottleneck: 55-130 мин vs 2-5 мин у конкурентов (risk score 20)

---

## 3. Исследование индустрии

### 3.1 Конкурентный ландшафт — 20 инструментов

Исследованы 20 инструментов в 5 тирах, от полностью автономных агентов до методологий и workflow.

#### Tier 1: Автономные агенты (5 инструментов)

| Инструмент | Цена | Планирование | Multi-agent | Ключевая фича |
|-----------|------|-------------|-------------|---------------|
| **Devin** (Cognition) | $20+/мес + ACU | Автономное | Параллельные сессии | Облачная IDE, Devin 2.0 |
| **Codex** (OpenAI) | $20-200/мес | Sandbox/задачу | Да (март 2026) | Skills, GPT-5.2-Codex |
| **Claude Code** (Anthropic) | $20-200/мес | Инкрементальное | Agent Teams (эксп.) | 1M контекст, $1B ARR за 6 мес |
| **Amazon Q** | $19/мес | Автономное | Нет | SWE-bench 66%, Java миграции |
| **Jules** (Google) | $0-125/мес | Асинхронное | Нет | Suggested Tasks (проактивное сканирование) |

**Общая черта:** все принимают plain text / issues / описание. Ни один не требует внешнего workflow engine.

#### Tier 2: IDE-интегрированные агенты (5 инструментов)

| Инструмент | Цена | Ключевая фича |
|-----------|------|---------------|
| **Cursor** | $0-20+/мес | До 8 параллельных агентов, завершение < 30 сек |
| **Windsurf** (Cognition) | $15/мес | Cascade, persistent knowledge layer, #1 AI Dev Tool Rankings |
| **GitHub Copilot** | $10-39/мес | Git-native (draft PR), security scanning |
| **Kiro** (AWS) | Preview (бесплатно) | **Spec-driven development** — промпт -> user stories + design doc + task list |
| **Junie** (JetBrains) | $100-300/год | Глубокая интеграция с JetBrains IDE |

**Kiro — ближайший конкурент ralph по подходу.** Оба создают промежуточные спецификации перед кодированием. Kiro: prompt -> requirements.md -> design.md -> tasks.md. ralph plan: requirements -> sprint-tasks.md. Разница: Kiro встроен в IDE, ralph — CLI.

#### Tier 3: CLI/Framework агенты (4 инструмента)

| Инструмент | Цена | Ключевая фича |
|-----------|------|---------------|
| **Aider** | Бесплатно + API | Architect mode (2-модельный: планирование + выполнение) |
| **SWE-Agent** (Princeton/Stanford) | Бесплатно + API | Custom ACI, $0.43 средняя стоимость решения |
| **OpenHands** (ex-OpenDevin) | Бесплатно + API | Расширяемая платформа, Docker sandbox |
| **AutoCodeRover** (-> SonarSource) | Часть Sonar | Fix bugs found by static analysis |

**Aider — самый близкий CLI-конкурент.** Architect mode: одна модель планирует (cheap), другая кодирует (capable). ralph plan аналогичен: plan = планирование, run = кодирование.

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

### 3.2 Паттерны оркестраторов (11 инструментов, лучшие практики)

Из детального анализа 11 инструментов (Devin, SWE-Agent, OpenHands, AutoCodeRover, MetaGPT, Gastown, Claude Code, Cursor, Windsurf, Copilot Workspace, Kiro) выделены 7 работающих паттернов и 6 антипаттернов.

#### 7 работающих паттернов

**Паттерн 1: Spec-first pipeline (Copilot Workspace, Kiro, MetaGPT)**

Явная спецификация "что есть" vs "что нужно" перед планированием. Промежуточные документы снижают ambiguity. Пользователь может steering на каждом этапе.

- Copilot Workspace: Specification (current vs desired state) -> Plan (files to modify) -> Implementation
- Kiro: requirements.md -> design.md -> tasks.md (3-фазный workflow с EARS нотацией)
- MetaGPT: PRD -> User Stories -> Design -> Implementation (SOP pipeline с 5+ ролями)

Применимость к ralph: `ralph plan` реализует сжатую версию — requirements -> tasks. Промежуточная spec-фаза возможна через `ralph init --interactive`.

**Паттерн 2: Git-backed state (Gastown, Kiro)**

Состояние задач в Git, а не in-memory. Version control для task state даёт rollback. ralph уже использует файловый подход — sprint-tasks.md в git.

**Паттерн 3: ACI-optimized tools (SWE-Agent, Princeton)**

Дизайн интерфейса агента важнее сложности планирования. Linter-gates на edit commands предотвращают синтаксические ошибки. Feedback format оптимизирован для LLM. Результат: 12.29% на SWE-bench, $0.43 средняя стоимость решения.

**Паттерн 4: Role-based SOP (MetaGPT, Gastown)**

Фиксированные роли с определёнными ответственностями. Стандарты для промежуточных выходов. ralph уже использует 4-агентную модель (creator/validator/developer/reviewer) через manual orchestration.

**Паттерн 5: Editable intermediate artifacts (Copilot Workspace, Devin, Kiro)**

Все промежуточные документы редактируемые. Двунаправленная коммуникация human<->agent. sprint-tasks.md уже редактируемый — это преимущество ralph.

**Паттерн 6: Persistent knowledge (Devin, Kiro)**

Knowledge entries переживают сессии. ralph уже имеет knowledge management (Epic 6) — extraction, distillation, injection.

**Паттерн 7: As-needed decomposition (ADaPT, Claude Code)**

ADaPT (Allen AI, NAACL 2024): Executor пробует задачу, при failure — Planner рекурсивно декомпозирует failed sub-task. Результаты: +28.3% в ALFWorld, +27% в WebShop, +33% в TextCraft. Совместимо с retry-механизмом ralph.

#### 6 антипаттернов

| Антипаттерн | Пример | Проблема |
|---|---|---|
| Overplanning | Детальный plan для single-issue fix | SWE-Agent: для простых задач план не нужен |
| Планы без steering points | Нет возможности вмешаться между spec и impl | Ошибки amplify'ятся |
| In-memory-only state | Claude Code без persistent tasks | Потеря контекста при перезапуске |
| Trust without validation | Слепое доверие агентам | Gastown: validation gates |
| Monolithic agent | Один агент для всего | Не масштабируется |
| Fidelity loss между ролями | Неструктурированные промежуточные артефакты | Информация теряется |

**Bridge попадает под 2 антипаттерна:** fidelity loss (83% потеря гранулярности) и plans without steering points (нет возможности вмешаться между bridge и runner).

#### Паттерны оркестрации — сравнение

| Паттерн | Кто использует | Применимость к ralph |
|---------|---------------|---------------------|
| Sequential Pipeline | CrewAI, OMC, **bmad-ralph** | **Текущий паттерн ralph** — предсказуемость, простота |
| DAG (графы) | LangGraph, MS Agent Framework | Избыточен для ralph |
| Event-Driven | AutoGen v0.4, MS Agent Framework | Сложно предсказать поведение |
| Hierarchical Supervision | Gas Town, Claude Code Teams | Bottleneck на Mayor, стоимость supervisor-агентов |
| Swarm / Task Pool | OMC, OpenAI Swarm | Merge conflicts, координация доступа |

ralph использует Sequential Pipeline — самый предсказуемый и простой паттерн. Нет причин усложнять.

**Тренд 2026: два лагеря** — general-purpose frameworks (CrewAI, LangGraph, MS Agent Framework) и code-specific orchestrators (Gas Town, Claude Code Teams, Goosetown). ralph уверенно в лагере code-specific orchestrators с уникальным фокусом на quality assurance + knowledge lifecycle.

#### Ключевые тренды 2025-2026

1. **Spec-driven > prompt-driven:** наиболее успешные инструменты создают explicit промежуточные документы (Kiro, Copilot Workspace, MetaGPT). ralph plan следует этому тренду.
2. **Hybrid decomposition побеждает:** ни чистый upfront, ни чистый incremental планирование не оптимальны. ADaPT (+28-33%) показывает: декомпозируй по мере необходимости.
3. **Persistent state критичен:** без persistent task state масштабирование невозможно. ralph уже имеет sprint-tasks.md в git.
4. **Steering points обязательны:** Copilot Workspace с двумя explicit steering points — золотой стандарт. ralph gates реализуют этот паттерн.

### 3.3 Gastown и мульти-агентные фреймворки

#### Gas Town (Steve Yegge)

Система оркестрации 20-30 параллельных Claude Code агентов. Go, ~11k stars, "100% vibecoded" за ~17 дней.

| Концепция | Описание | Релевантность для ralph |
|-----------|---------|------------------------|
| **Beads** | Атомарные единицы работы, JSON в Git | Аналог story files, но с формализованным ID |
| **Convoys** | Группировка задач для агентов | Аналог sprint-tasks.md |
| **Mayor** | Координатор, НЕ пишет код | Нет аналога (ralph сам координатор) |
| **Polecats** | Временные рабочие, "умирают" после задачи | Аналог одиночных Claude Code сессий |
| **Witness** | Супервизор, мониторит и подталкивает | Потенциально полезный паттерн |
| **Refinery** | Merge queue, разрешение конфликтов | Потенциально полезный |
| **Seancing** | Воскрешение знаний предыдущих сессий | Аналог knowledge distillation (Epic 6) |

**Стоимость Gas Town:** $2,000-$5,000/мес, ранний пользователь: $100/час. ralph: $20-200/мес. Разница: 10-25x.

**Критика (Maggie Appleton):** "Количество перекрывающихся и ad hoc концепций ошеломляет". Компоненты произвольны, vibecoding-подход жертвует связностью.

**Фундаментальное наблюдение из Gas Town:** когда агенты берут на себя реализацию, **дизайн становится узким местом** — скорость ограничена человеческой способностью к планированию, а не скоростью кодирования. Это напрямую подтверждает необходимость `ralph plan`.

#### Goosetown (Block/Square)

Надстройка над Goose (~27k stars), вдохновлена Gas Town. Более лёгкий подход: каждый subagent работает в чистом контексте. **Town Wall** — append-only лог координации.

#### Почему massive parallelism НЕ для ralph

1. **Стоимость:** Gas Town $2-5k/мес vs ralph ~$20-200/мес. 10-25x разница.
2. **Merge conflicts:** 20-30 параллельных агентов создают больше конфликтов, чем решают задач.
3. **Design bottleneck:** скорость ограничена планированием, не кодированием.
4. **Предсказуемость:** sequential pipeline ralph предсказуемее и проще в отладке.
5. **Vibecoding-подход Gas Town** жертвует связностью — ralph ценит тестируемость и 100% fix rate.

#### Идеи для заимствования (из всех фреймворков)

| Что заимствовать | Откуда | Приоритет |
|-----------------|--------|-----------|
| Persistent task file (tasks.md/yaml) | Kiro | Высокий (реализуется через ralph plan) |
| Spec-first pipeline | Copilot Workspace | Высокий (ralph plan) |
| As-needed decomposition | ADaPT (Allen AI) | Высокий (ralph replan / retry) |
| Git-backed task state | Gastown | Высокий (sprint-tasks в git) |
| Blueprint First, Model Second | Академические исследования | Высокий (основной принцип) |
| Knowledge entries | Devin | Средний (уже есть в Epic 6) |
| ACI improvements (linter-gates) | SWE-Agent | Средний |
| Multi-agent orchestration | MetaGPT, Gastown | Низкий (v3+) |

#### Task persistence — сравнение подходов

| Подход | Реализация | Плюсы | Минусы |
|--------|-----------|-------|--------|
| Git-backed JSON | Gas Town beads | Версионирование | Merge conflicts |
| Database checkpoints | LangGraph + Postgres | Надёжно, queryable | Требует инфраструктуру |
| In-memory + memory tiers | CrewAI | Простота | Потеря при крэше |
| **File-based state** | **bmad-ralph** | **Простота, git-trackable** | **Нет rich querying** |
| Worktree isolation | Claude Code Teams | Git-native изоляция | Сложный merge |

ralph's file-based approach оптимален для CLI-инструмента: простота + git-trackable.

### 3.4 Позиционирование ralph среди конкурентов

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
- Более методологичный, чем Aider/Taskmaster

**Формула позиционирования:**
> ralph = structured task loop + adversarial code review + knowledge lifecycle + context observability

#### Фичи без аналогов

| Фича ralph | Ближайший конкурент | Разница |
|-----------|-------------------|---------|
| Context window observability (FR75-FR92) | Нет прямых аналогов | **Уникальная фича** |
| Knowledge extraction + distillation | Claude Code (.claude/ + memory) | ralph формализует цикл обучения |
| Per-task human gates (approve/reject/modify) | Copilot (review PR) | ralph гранулярнее |
| Adversarial code review (5 агентов) | Copilot (security scanning) | ralph по code quality, не security |

### 3.5 Конкурентные угрозы и возможности

#### Критические угрозы

**1. Claude Code Agent Teams** — нативная мульти-агентная координация от Anthropic. Если добавят task persistence и review loop, ralph теряет значительную часть value proposition. Статус: экспериментальный (февраль 2026), API может измениться.

**2. Kiro** (AWS) — spec-driven подход максимально близок к ralph. Промпт -> user stories + acceptance criteria + design doc + task list. Если Kiro добавит review pipeline и knowledge extraction, пересечение будет критическим.

#### Среднесрочные угрозы

**3. Taskmaster AI** — task decomposition + context management. Фокус на тех же проблемах (потеря контекста в больших проектах), другой подход (плагин к IDE).

**4. OpenAI Codex Skills** — если skills ecosystem разовьёт BMad-подобные workflows, ralph станет менее нужен.

#### Возможности

1. **Усилить Context window observability** — единственная фича без конкурентов. Развивать в направлении cost optimization.
2. **Knowledge extraction loop** — формализованное обучение (extraction -> distillation -> injection) не имеет аналогов.
3. **Самодостаточность = survival:** все конкуренты самодостаточны. Устранение BMad-зависимости — вопрос выживания, а не оптимизации.

---

## 4. Анализ BMad Method

### 4.1 Структура BMad v6 — 12 типов документов

Полный pipeline BMad v6 генерирует 12 типов документов в 4 фазах:

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

**Жирным** выделены документы, реально используемые в pipeline разработки.

#### Что runner РЕАЛЬНО использует

Из анализа Go-кода (`config/config.go`, `runner/scan.go`, `bridge/bridge.go`):

**Используется runner'ом (2 типа из 12):**

| Документ | Как используется |
|----------|-----------------|
| Sprint Status (`sprint-status.yaml`) | Определение статуса stories, сканирование StoriesDir |
| Story файлы (`{N}-{M}-*.md`) | Контекст для Claude Code сессий |

**НЕ используется runner'ом (10 типов из 12):** PRD, UX Design, Architecture, Epics, Implementation Readiness, BMM Workflow Status, Validation Reports, Brainstorming, Product Brief, Research — все используются исключительно BMad workflow'ами для ГЕНЕРАЦИИ story файлов.

**Вывод:** Ralph runner работает ТОЛЬКО с 2 типами файлов из 12. Все остальные — промежуточные артефакты BMad pipeline, не нужные ralph напрямую.

### 4.2 Pipeline генерации stories

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

#### Коэффициент расширения

```
Epics.md:     ~15 строк на story (краткие AC + Technical Notes)
Story файл:   ~170 строк (полные Tasks/Subtasks + Dev Notes + References)
Коэффициент расширения: ~11x
```

create-story workflow добавляет ~155 строк контекста на каждую story. Основной объём — Tasks/Subtasks и Dev Notes.

### 4.3 Что из BMad workflows нужно ralph (11% из 29600 строк)

#### Суммарный объём промптов: ~29600 строк

| Категория | Строк инструкций | Строк чеклистов | Итого |
|-----------|-----------------|-----------------|-------|
| Фаза 1 (анализ) | ~4900 | 0 | ~4900 |
| Фаза 2 (планирование) | ~5275 | 0 | ~5275 |
| Фаза 3 (solutioning) | ~3131 | 169 | ~3300 |
| Фаза 4 (implementation) | ~2848 | 750 | ~3598 |
| Вспомогательные | ~9246 | ~3283 | ~12529 |
| **ИТОГО** | **~25400** | **~4202** | **~29600** |

#### 89% промптов НЕ нужны ralph

| Категория | Строк | Обоснование |
|-----------|-------|-------------|
| Pre-implementation (product-brief, research, prd, UX, architecture) | ~13475 | Domain пользователя, требует интерактивный диалог |
| Визуализация (diagrams x4) | ~645 | Excalidraw-специфичны |
| BMad infrastructure (workflow-status, workflow-init) | ~741 | Мета-координация BMad engine |
| Test Architecture (testarch x8) | ~9610 | QA планирование, domain пользователя |
| Quick Flow | ~245 | ralph сам является "quick flow" |
| Document Project | ~1626 | Разовая задача |
| **Итого НЕ встраивать** | **~26342** | **89% всех промптов BMad** |

#### Оставшиеся 11% — уже покрыты runner/review

| BMad Workflow | Ralph эквивалент | Покрытие |
|--------------|-----------------|----------|
| dev-story (405 строк) | ralph execute (runner + execute.md 130 строк) | **~90%** |
| code-review (237 строк) | ralph review (review.md 176 строк + 5 agents ~182 строки) | **~95%** |
| create-story (732 строки) | ScanTasks + execute prompt context | **~80%** |
| sprint-planning (285 строк) | ScanTasks + tasks.md management | **~70%** |

**ralph review даже мощнее BMad:** 5 специализированных review agents vs 1 monolithic BMad reviewer.

#### Коэффициент сжатия: 9-12x

BMad промпты содержат огромное количество formatting, emoji, party mode, checkpoint протокола. При портировании в ralph:

| Для чего | Строк из BMad | Строк в ralph (после сжатия) |
|----------|--------------|------------------------------|
| plan mode (декомпозиция) | 387 | ~60-80 |
| replan mode (correct-course) | 485 | ~60-80 |
| retro mode (ретроспектива) | 1443 | ~50-80 |
| knowledge extraction | 60 | ~20-30 |
| **Итого** | **2375** | **~190-270** |

### 4.4 Что ralph уже покрывает

Ralph v1 уже реализует эквиваленты ключевых BMad workflow:

| BMad Workflow | ralph v1 эквивалент | Что отличается |
|---|---|---|
| dev-story | `ralph run` (execute loop) | ralph добавляет retry, gates, metrics |
| code-review | `ralph review` (5 agents) | ralph глубже (5 agents vs 1) |
| sprint-planning | ScanTasks + sprint-status.yaml | ralph не генерирует stories |
| knowledge-extraction | Epic 6 (extraction + distillation) | ralph формализует lifecycle |
| correct-course | Gate actions [r] retry | ralph не имеет [c] correct course (пока) |

### 4.5 Вердикт: встраивать BMad или нет

**НЕ встраивать BMad workflows в ralph.** Обоснование:

1. **89% промптов не нужны ralph** — pre-implementation, визуализация, QA planning, infrastructure.

2. **Оставшиеся 11% уже покрыты** — ralph execute/review покрывают dev-story/code-review на 90-95%.

3. **Промпты BMad не переносимы напрямую** — оптимизированы под BMad workflow engine (checkpoints, party mode, template-output). Ralph использует другую модель: один промпт на сессию Claude Code.

4. **Ralph и BMad — complementary tools:**
   - BMad: планирование, design, story creation (высокоинтерактивные)
   - Ralph: автономное выполнение задач (низкоинтерактивный loop)
   - Граница чёткая: BMad создает артефакты -> Ralph их исполняет

5. **Единственная фича с ROI > 1:** `ralph plan` как lightweight декомпозитор задач. ~70 строк промпта, ~250 LOC Go-кода. Это НЕ встраивание BMad — это создание альтернативы для простых случаев.

---

## 5. Варианты решения — полное сравнение

### 5.1 Вариант A: Программный парсинг bridge

**Суть:** Убрать LLM из bridge, заменить Go-парсером. Bridge остаётся, но парсит AC программно.

| Аспект | Значение |
|--------|----------|
| Архитектура | `bridge/` рефакторинг, `runner/` без изменений |
| Входные данные | BMad story файлы (строго `docs/sprint-artifacts/*.md`) |
| Выходные данные | `sprint-tasks.md` (идентичный формат) |
| Зависимость от BMad | **Полная** — stories обязательны |
| Зависимость от LLM (planning) | **Нулевая** — программный парсинг |
| LOC нового кода | ~300 + ~600 тестов |
| LOC удаления | ~400 (bridge.md + session-код) |
| Stories | 2-3 |
| Дни | 1-2 |
| Качество декомпозиции | 6/10 |
| Time-to-first-task | 55-130 мин (bottleneck = BMad) |
| Стоимость planning | $0 (программный) |
| Гибкость | 1/10 |
| Backward compatibility | Полная |

**Плюсы:**
- Детерминизм — одинаковый результат при каждом запуске
- Быстрота — мгновенное выполнение, нет Claude call
- Минимальный объём работ

**Минусы:**
- Грубая группировка: keyword-эвристики покрывают ~60% случаев
- Regex привязан к формату story — хрупкость
- Не устраняет BMad-зависимость (стратегическая проблема)
- Нулевая гибкость для новых форматов

### 5.2 Вариант B: Runner со stories напрямую

**Суть:** Убрать bridge целиком. Runner напрямую читает story файлы, каждая story = scope Claude-сессии.

| Аспект | Значение |
|--------|----------|
| Архитектура | `runner/` значительно переработан, `bridge/` удалён |
| Зависимость от BMad | **Полная** |
| LOC нового кода | ~800 + ~1500 тестов |
| LOC удаления | ~2800 (bridge/ целиком) |
| Stories | 4-6 |
| Дни | 3-5 |
| Качество декомпозиции | **8/10** (лучший) |
| Гибкость | 1/10 (runner привязан к story формату) |
| Backward compatibility | **Нет** |

**Плюсы:**
- Лучшее качество — Claude видит ВСЮ story целиком
- 2 AI-слоя вместо 3

**Минусы:**
- Нет checkpoint'ов внутри story (8 AC = одна сессия)
- Ломает backward compatibility (sprint-tasks.md удаляется)
- 20+ тестов runner'а потенциально ломаются
- Не устраняет BMad-зависимость

### 5.3 Вариант C: Epics напрямую

**Суть:** Убрать промежуточный слой stories. Ralph обрабатывает epic целиком.

| Аспект | Значение |
|--------|----------|
| LOC нового кода | ~1200 + ~2000 тестов |
| Stories | 6-8 |
| Дни | 5-7 |
| Качество декомпозиции | **4/10** (худший) |
| Backward compatibility | **Нет** |

**Минусы (критические):**
- Context window overflow: epic (200 строк) + architecture + codebase = 500+ строк метаданных
- Потеря Dev Notes (они в stories, stories удалены)
- Декомпозиция 1 epic = 10-30 tasks — на порядок сложнее для LLM
- Полный редизайн workflow, максимальный объём работы
- **Худший вариант по всем критериям**

### 5.4 Вариант D: ralph plan (самодостаточный)

**Суть:** Новая команда `ralph plan` принимает любой текстовый input и генерирует sprint-tasks.md. BMad опционален.

| Аспект | Значение |
|--------|----------|
| Архитектура | `planner/` новый пакет, `bridge/` deprecated, `runner/` без изменений |
| Входные данные | **Любой markdown/text** (PRD, Issues, plain text, BMad stories) |
| Выходные данные | `sprint-tasks.md` (тот же формат) |
| Зависимость от BMad | **Нулевая** — BMad опционален |
| LOC нового кода | ~120 Go + ~150 промпт + ~1250 тестов |
| LOC удаления (Phase 3) | -2844 (bridge/ целиком) |
| Нетто-эффект | **-1324 строки** (упрощение codebase) |
| Stories | 3-5 |
| Дни | 2-3 |
| Качество декомпозиции | 7/10 |
| Time-to-first-task (без BMad) | **11-33 мин** (в 3-5x быстрее) |
| Стоимость planning | $0.10-0.30 (90-97% экономия) |
| Гибкость | **9/10** |
| Backward compatibility | **Полная** |

**Плюсы:**
- Единственный самодостаточный вариант
- Минимальный риск при максимальном эффекте
- ADaPT-совместимость (as-needed decomposition)
- Нетто-упрощение codebase (-1324 строки)
- Полная backward compatibility (runner не трогается)
- Максимальный потенциал community adoption

**Минусы:**
- LLM недетерминизм (менее критично — промпт на 61% короче bridge.md)
- Качество зависит от input (plain text "добавь auth" < формализованный PRD)
- Нет BMad-уровня валидации

### 5.5 Сводная таблица сравнения (10 критериев)

```
Критерий               Вес    A      B      C      D
------------------------------------------------------------
Качество декомпозиции   20%    6      8*     4      7
Самодостаточность       15%    1      1      1     10*
Time-to-first-task      15%    3      3      3      8*
Объём работы (обратный) 10%    9*     5      3      8
Backward compat         10%   10*     2      1     10*
Гибкость форматов       10%    1      1      1      9*
Риск (обратный)         10%    9*     6      3      9*
Стоимость (обратная)     5%    7      6      7      9*
Adoption потенциал       5%    2      2      2      9*
------------------------------------------------------------
ИТОГО (взвешенный)            4.85   3.95   2.45   8.50*
```

*Примечание: итоговые взвешенные оценки взяты из первоисточника (`v2-variants-comparison.md`), где оценки критериев содержат дробные значения. Целые числа в таблице выше — округление для читаемости.*

### 5.6 Взвешенные оценки

| Ранг | Вариант | Взвешенная оценка | Разница от лидера |
|------|---------|-------------------|-------------------|
| **1** | **D (ralph plan)** | **8.50** | — |
| 2 | A (программный парсинг) | 4.85 | -3.65 |
| 3 | B (runner со stories) | 3.95 | -4.55 |
| 4 | C (epics) | 2.45 | -6.05 |

**Вариант D побеждает с отрывом 3.65 балла** от ближайшего конкурента.

### 5.7 Почему выбран Вариант D

5 ключевых аргументов:

**1. Конкурентный паритет.** Из 20 инструментов ни один не требует внешнего workflow. ralph с BMad-зависимостью — аномалия.

**2. Минимальный риск при максимальном эффекте.** `planner/` — новый пакет параллельно `bridge/`. Runner не трогается. 0 тестов ломается. Bridge deprecated, не удалён немедленно.

**3. ADaPT-совместимость.** As-needed decomposition на 28-33% эффективнее upfront decomposition (Allen AI, NAACL 2024). Bridge делает upfront (все AC -> все tasks). Plan может использовать ADaPT.

**4. Нетто-упрощение.** После Phase 3: -1324 строки кода. Codebase становится проще.

**5. Backward compatibility.** Единственный вариант (кроме A), полностью backward-compatible. Миграция = замена одного слова: `ralph bridge story.md` -> `ralph plan story.md`.

#### Возможная комбинация D + элементы A

Для BMad stories — программный парсинг AC без LLM (0 cost). Для plain text/PRD — LLM-декомпозиция. Это ADaPT-подход: программный парсинг для structured input, LLM для unstructured.

---

## 6. Детальный дизайн Варианта D

### 6.1 ralph plan — архитектура

#### Пакет planner/, зависимости, структура

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

Архитектурное правило проекта (`cmd/ralph -> runner -> session, gates, config`) соблюдено: `planner` — параллельный пакет на том же уровне, что и `runner`.

Структура пакета:

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

#### Ключевые типы данных

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

#### JSON от LLM, Go форматирует

Ключевой архитектурный принцип: **"Blueprint First, Model Second"** — Go выполняет всю детерминистическую работу, LLM только семантический анализ.

| Ответственность | Кто | Как |
|-----------------|-----|-----|
| Автодискавери документов | Go | Regex по имени, содержимому |
| Сбор контекста (tree, go.mod) | Go | Программное чтение файлов |
| Сборка промпта | Go | config.AssemblePrompt() |
| **Декомпозиция требований в задачи** | **LLM** | **JSON output** |
| Парсинг JSON output | Go | json.Unmarshal + валидация |
| Форматирование sprint-tasks.md | Go | Программный формат |
| Source traceability | Go | Из requirement_refs + имя файла |
| Gate marking | Go | Первый в epic + keyword scan |
| Merge с existing tasks | Go | Детерминистический diff |
| Topological sort | Go | Из depends_on |

Исследование показывает +10.1 п.п. на tau-bench, -81.8% tool calls при "Blueprint First" подходе.

#### Автодискавери документов (3 уровня)

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

#### CLI флаги

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

#### Основная логика Plan() — 7 шагов

```
Plan(ctx, cfg, opts):
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

#### 9 stories, 3 фазы

**Phase 1 (Stories 1-5): Core planner — 5 stories, ~3 дня**

| Story | Scope | LOC (оценка) |
|-------|-------|---|
| 1 | PlanOptions + PlanResult structs, PlanConfig в config | ~100 |
| 2 | DiscoverDocs() + ClassifyDoc() | ~200 |
| 3 | CollectContext() + BuildDirTree() | ~150 |
| 4 | ParseLLMOutput() + FormatTasks() + FormatTaskLine() | ~200 |
| 5 | Plan() orchestrator + plan.md prompt + CLI command | ~200 |

**Phase 2 (Stories 6-7): Merge + backward compat — 2 stories, ~1 день**

| Story | Scope |
|-------|-------|
| 6 | MergeTasks() — детерминистический merge |
| 7 | `--from-stories` backward compat (программный парсинг BMad stories) |

**Phase 3 (Stories 8-9): Bridge deprecation — 2 stories, ~1 день**

| Story | Scope |
|-------|-------|
| 8 | `ralph bridge` deprecated с warning |
| 9 | `bridge/` удалён (-2844 LOC) |

### 6.2 ralph init — быстрый старт

#### Проблема

Ralph сейчас требует 5 шагов до первого коммита. Time-to-first-task: 55-130 минут. Конкуренты: 1-5 минут.

#### Позиционирование на спектре инструментов

```
Минимум структуры                                    Максимум структуры
     |                                                        |
  Cursor    Aider    Claude Code    Devin    ralph init    ralph+BMad
  "делай"   "делай"  "CLAUDE.md"   "Issue"  "требования"  "PRD->Stories"
  1 мин     1 мин    1-3 мин       2-5 мин  2-5 мин       55-130 мин
```

**Ниша ralph:** между Devin и полным BMad. Больше структуры, чем "просто делай", меньше церемонии, чем BMad.

**Ключевой инсайт:** ralph init не конкурирует с "напиши функцию" (территория Cursor/Aider). ralph init для проектов, где нужно 10-100+ задач с code review и quality control.

#### 3 flow (one-liner, interactive, brownfield)

**Flow 1: Минимальный (one-liner)**

```bash
ralph init "Платформа для обучения Java с sandbox, JWT auth, Monaco editor"
```

- Создаёт `docs/requirements.md` через один LLM-вызов
- Стоимость: ~$0.10-0.30
- Время: 30-90 секунд
- Выход: "Создан docs/requirements.md. Проверьте, затем: ralph plan"

**Flow 2: Интерактивный**

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

**Flow 3: Brownfield (существующий проект)**

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

#### Сравнение requirements.md vs BMad PRD

| Аспект | requirements.md | BMad PRD |
|--------|----------------|----------|
| Объём | 30-80 строк | 200-500 строк |
| User Journeys | Нет | 3-5 детальных |
| Success Criteria | Нет (или 2-3 строки) | Секция с KPI |
| FR нумерация | Свободная | Структурированная по группам |
| NFR | 2-5 пунктов | 10-15 с таблицей |
| Scope | MVP only | MVP + Growth + Vision |
| UI/UX | Нет | Отдельный документ |

**Ключевое различие:** requirements.md — "достаточно для начала работы". BMad PRD — "полная спецификация для передачи команде".

#### Полные pipeline'ы

**Greenfield минимальный (2-5 мин):**
```bash
mkdir my-project && cd my-project && git init
ralph init "REST API для задач на Go с PostgreSQL и JWT"
# -> docs/requirements.md (30 сек)
# Проверка/правка (1-2 мин)
ralph plan
# -> sprint-tasks.md (30 сек)
ralph run
```

**BMad-совместимый:**
```bash
ralph plan --from-stories docs/sprint-artifacts/  # Программный парсинг (без LLM!)
ralph run
```

### 6.3 ralph replan — коррекция курса

#### 3 уровня коррекции

```
Уровень 1: Тактический (retry)    — УЖЕ РЕАЛИЗОВАН
  Scope: одна текущая задача
  Механизм: [r] -> InjectFeedback() -> RevertTask() -> повтор

Уровень 2: Структурный (replan)   — НОВЫЙ [c]
  Scope: sprint-tasks.md целиком
  Механизм: [c] -> описание -> Claude replan -> diff -> подтверждение

Уровень 3: Стратегический (reinit) — ОТЛОЖЕН (v3+)
  Scope: requirements + sprint-tasks.md
  Механизм: пересоздание requirements -> новый sprint plan
```

#### Gate action [c] Correct Course

Новое меню обычного gate:
```
HUMAN GATE: - [ ] Implement user login [GATE]
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

#### Replan flow

```go
type ReplanFunc func(ctx context.Context, opts ReplanOpts) (*ReplanResult, error)

type ReplanResult struct {
    OriginalContent string
    NewContent      string
    Added           []string   // новые задачи
    Removed         []string   // убранные задачи
    Modified        []string   // изменённые задачи
    Preserved       []string   // сохранённые [x] задачи
}
```

Алгоритм:
1. Прочитать текущий sprint-tasks.md
2. Извлечь выполненные [x] задачи — **НЕПРИКОСНОВЕННЫ**
3. Сформировать промпт (текущие задачи + описание изменений)
4. Вызвать Claude через session.RunClaude
5. Распарсить ответ -> новый sprint-tasks.md
6. Вычислить diff (Added/Removed/Modified)
7. Вернуть ReplanResult

#### Diff display

```
Plan changes:
  + Added (3):
    + - [ ] Implement OAuth2 authorization flow
    + - [ ] Add refresh token rotation
    + - [ ] Add OAuth scopes configuration

  - Removed (2):
    - - [ ] Implement basic auth middleware
    - - [ ] Add password hashing

  ~ Modified (1):
    ~ - [ ] Add login endpoint -> - [ ] Add OAuth login endpoint

  = Preserved (5 completed tasks unchanged)

Apply changes? [a]pply  [e]dit description  [c]ancel
>
```

#### Защита [x] задач

Два уровня защиты:

1. **ValidateReplan()** — проверяет, что все [x] из оригинала присутствуют в replanned. При потере — ошибка, не применяем.

2. **ForcePreserveDoneTasks()** — крайний fallback: убрать из replanned любые [x] (Claude мог испортить), вставить оригинальные [x] в начало.

#### Сравнение с BMad correct-course

BMad correct-course — 6-шаговый тяжеловесный процесс (Impact Analysis 20 пунктов, Sprint Change Proposal, Agent routing). Из 6 шагов ralph нужна суть только шагов 1 и 3 — сбор описания и показ конкретных изменений.

| BMad элемент | Нужен ralph? | Обоснование |
|---|---|---|
| Сбор описания проблемы | Да | Одна строка ввода |
| Diff-показ изменений | Да | Критично для UX |
| Подтверждение/отклонение | Да | Rollback если отклонено |
| Impact Analysis (20 пунктов) | **Нет** | Overkill для CLI |
| Sprint Change Proposal | **Нет** | Некому читать в solo-dev CLI |
| Agent routing | **Нет** | ralph — один агент |

#### Оценка работ

| Компонент | Оценка |
|---|---|
| `config/constants.go` + test | Тривиально |
| `gates/gates.go` + test | ~60 строк |
| `runner/replan.go` + test | ~550 строк |
| `runner/prompts/replan.md` + prompt test | ~60 строк |
| `runner/runner.go` + test (case ActionCorrectCourse) | ~110 строк |
| `runner/metrics.go` (Replans counter) | ~5 строк |
| **ИТОГО** | **~800 строк, 3-4 story, ~2 дня** |

### 6.4 YAML формат задач (отложено)

#### Мотивация

3 проблемы текущего sprint-tasks.md:
1. **Хрупкий regex-парсинг** — 4 regex'а в config/constants.go, ломаются от `- []` (без пробела), `* [ ]`, `[X]`
2. **LLM merge mode** — 244-строчный промпт, 12 правил merge, ~85% надёжность
3. **Бедные метаданные** — нет ID, зависимостей, оценки сложности, подсказок по файлам

#### YAML Schema

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
        depends_on: []
        files_hint: []
        size: M                # S | M | L
        feedback: ""
```

#### Markdown vs YAML — сравнение

**Парсинг:**

| Аспект | Markdown | YAML |
|--------|----------|------|
| Парсер | 4 regex, 70 строк | yaml.Unmarshal, ~30 строк |
| Edge cases | `- []`, `* [ ]`, `[X]`, табы vs пробелы | Нет |
| Валидация | Нет (regex матчит или нет) | Полная: типы, enum, зависимости |
| Ошибки | Молча пропускает невалидные строки | Явная ошибка с позицией |

**Merge:**

| Аспект | LLM Merge | Программный Merge |
|--------|-----------|-------------------|
| Надёжность | ~85% | **100%** (детерминистичный) |
| Скорость | 30-60 сек | **<10 мс** |
| Стоимость | ~$0.01-0.05 | **$0** |
| Тестируемость | Невозможно unit-test | Полное покрытие |

#### Почему отложить

- Текущий markdown формат работает стабильно 10 эпиков, ~500 задач
- Regex-хрупкость не является реальной проблемой на практике
- GO/NO-GO решение через 1-2 месяца использования `ralph plan`
- YAML = migration burden: нужен dual-format период, тесты, документация
- Критерий удаления markdown: если за 30 дней ни одного md-файла не прочитано

### 6.5 Промпты plan.md и replan.md

#### Исследование промптовых стратегий

| Стратегия | Суть | Применимость к ralph |
|-----------|------|---------------------|
| **Skeleton-of-Thought** | Сначала скелет, затем детали | Скелет epics -> задачи |
| **Plan-and-Solve** | Промежуточная фаза планирования | Поле `analysis` в JSON = фаза reasoning |
| **Blueprint First, Model Second** | Go = scaffold, LLM = семантика | **Точно наш подход.** +10.1 п.п. |
| **ADaPT** | As-needed decomposition (+28-33%) | Применяется к replan |
| **Длина промпта** | Reasoning деградирует после ~3000 tokens | bridge.md превышает. plan.md: ~100 строк |

#### Выбор стратегии: CoT-in-JSON, single-pass

| Вариант | Описание | Latency | Качество | Стоимость |
|---------|----------|---------|----------|-----------|
| A: Single-pass | Один LLM-вызов | 1x | Хорошее для <10 FR | 1x |
| B: Two-step | LLM1: classify -> LLM2: generate | 2x | Лучше для 10-20 FR | 1.5-2x |
| **C: CoT-in-JSON** | Один вызов, поле `analysis` как CoT-зона | **1x** | **Как B, стоимость как A** | **1x + ~200 tokens** |

**Выбрано: Вариант C (CoT-in-JSON, single-pass).**

Обоснование:
1. Latency = 1 вызов (пользователь ждёт)
2. CoT не ограничивает reasoning: поле `analysis` — свободная зона
3. ~200 extra tokens vs удвоение контекста при two-step
4. Исследования: "reasoning first, then structured answer"
5. Для complex PRD (20+ FR): Go-код split по секциям (Blueprint First)

#### JSON Schema для output

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
          "tags": [],
          "size": "M"
        }
      ]
    }
  ]
}
```

Что Go добавляет программно (НЕ LLM):

| Поле | Кто | Как |
|------|-----|-----|
| `- [ ]` / `- [x]` | Go | Все новые = `[ ]`, existing сохраняются |
| `source:` | Go | Из requirement_refs + имя файла |
| `[GATE]` (первый в epic) | Go | Программно по позиции |
| `[GATE]` (deploy/security) | Go | Keyword scan по title |
| Ordering | Go + LLM | LLM задаёт depends_on, Go делает topological sort |
| Merge | Go | Детерминистический diff |

#### plan.md — 95 строк vs bridge.md — 244 строки (-61%)

| Метрика | plan.md | bridge.md | Разница |
|---------|---------|-----------|---------|
| **Строки** | ~95 | 244 | **-61%** |
| Requirement Classification | 10 строк | 14 строк | -4 строки |
| Task Granularity | 20 строк | 58 строк | -38 строк |
| Format Contract | 0 (JSON schema в Go) | 8 строк | -8 строк |
| Source Traceability | 0 (Go добавляет) | 41 строк | -41 строк |
| Merge Mode | 0 (Go делает) | 18 строк | -18 строк |
| Gate Marking | 0 (Go ставит) | 27 строк | -27 строк |
| Prohibited Formats | 0 (JSON schema) | 5 строк | -5 строк |
| Примеры (Correct/WRONG) | 0 (zero-shot) | 60 строк | -60 строк |

#### Что переиспользуется из bridge.md (18%)

| Секция bridge.md | Строки | Действие в plan.md |
|------------------|--------|--------------------|
| AC Classification (4 типа) | 28-41 | Упрощено до 1 абзаца |
| Task Granularity Rule | 43-98 | Сохранено ядро: unit of work + signals |
| Complexity Ceiling | 78-84 | Сохранены 4 эвристики |
| Minimum Decomposition | 86-90 | Сохранено (5+ AC -> 3+ tasks) |
| Task Ordering | 100-102 | Сохранено |
| Testing Within Tasks | 112-116 | Сохранено |
| **Итого** | **~45 строк из 244** | **18%** |

Что убирается (41%): Format Contract, Source Traceability, Merge Mode, Gate Marking, Prohibited Formats, Service Tasks, Negative examples — всё делает Go программно.

#### replan.md — ~65 строк (diff-based)

Ключевое отличие от plan.md: replan выдаёт **только изменения** (diff-формат), а не весь план:

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

Преимущества diff-формата:
- Предотвращает случайное изменение/удаление
- Минимизирует output tokens (2-5 изменений vs 20+ задач)
- Go применяет diff детерминистически
- `reason` для remove принуждает обосновать удаление

---

## 7. Анализ рисков

### 7.1 Риски Варианта D

| ID | Риск | Вероятность | Impact | Score | Описание |
|----|------|-------------|--------|-------|---------|
| **D-R2** | Промпт quality | Средняя | Высокий | **12** | Произвольный текст -> задачи, Claude может "додумывать" |
| **D-P3** | Scope creep | Средняя | Высокий | **12** | plan -> stories -> architecture -> ralph = BMad |
| D-T2 | Новая точка отказа | Средняя | Средний | 9 | LLM-генерация из произвольного текста менее предсказуема |
| D-T5 | Backward compatibility | Низкая | Высокий | 8 | Формат sprint-tasks.md |
| D-T1 | Регрессия качества | Низкая | Средний | 6 | plan.md без BMad-правил (AC Classification) |

**Средний score Варианта D: 5.4 (СРЕДНИЙ)**

Два самых опасных риска (оба score 12):

1. **D-R2: Качество промпта plan.md.** Ни один аналитик не тестировал промпт на реальных данных. Весь анализ теоретический. Митигация: итеративная разработка (v1 -> тест на 5+ реальных inputs -> v2), `--dry-run` для preview, `--review` для human approval, golden file тесты.

2. **D-P3: Scope creep к BMad.** При реализации ralph plan может возникнуть соблазн добавить генерацию stories, architecture и т.д. Митигация: жёсткий scope — ralph plan = input -> sprint-tasks.md. НЕ генерировать stories, architecture. Один промпт, один вызов, один файл.

### 7.2 Риски НЕ делать ничего (Status Quo)

| ID | Риск | Score | Описание |
|----|------|-------|---------|
| **SQ-7** | ralph без BMad неработоспособен | **25** | Нет альтернативного способа создать sprint-tasks.md |
| **SQ-1** | Bridge bottleneck | **20** | 55-130 мин от идеи до первого коммита vs 2-5 мин у конкурентов |
| **SQ-4** | Adoption barrier | **20** | Новый пользователь должен освоить BMad + ralph (7 концепций, 15 файлов) |
| SQ-3 | Конкуренты обгоняют | 16 | Devin, Kiro, Aider — все самодостаточны |
| SQ-6 | "Испорченный телефон" | 12 | 3 AI-слоя вместо 2 |
| SQ-2 | Зависимость от BMad | 12 | Изменения story формата ломают bridge промпт |
| SQ-5 | Batching антипаттерн | 12 | 6 изолированных Claude-сессий для planning |

**Средний score Status Quo: 16.7 (ВЫСОКИЙ)**

5 из 7 рисков не митигируются без архитектурных изменений. Status quo неприемлем долгосрочно.

### 7.3 Митигации для Варианта D

| Риск | Митигация |
|------|-----------|
| Промпт quality (D-R2) | Итеративная разработка: v1 -> тест на 5+ inputs -> v2. `--dry-run`, `--review` флаги. Golden file тесты. |
| Scope creep (D-P3) | Жёсткий scope: ralph plan = input -> sprint-tasks.md. НЕ генерировать stories, architecture. |
| Новая точка отказа (D-T2) | Golden file тесты, `--dry-run` для preview, детерминистическое форматирование Go. |
| Backward compat (D-T5) | Format contract сохранён. Runner не знает кто создал файл. `--from-stories` для BMad users. |
| Регрессия качества (D-T1) | Granularity rules переиспользуются из bridge.md (18%). Format contract идентичен. |

### 7.4 Ранжирование рисков — все варианты (таблица)

| Ранг | Вариант | Средний score | Оценка |
|------|---------|---------------|--------|
| 1 | **A** (программный парсинг) | **4.1** | НИЗКИЙ |
| 2 | **D** (ralph plan) | **5.4** | СРЕДНИЙ |
| 3 | **B** (runner со stories) | **7.6** | ВЫСОКИЙ |
| 4 | **D+** (plan + init + replan) | **8.5** | СРЕДНИЙ-ВЫСОКИЙ |
| 5 | **Status Quo** | **16.7** | ВЫСОКИЙ |
| 6 | **C** (epics) | **18.4** | КРИТИЧЕСКИЙ |

**Нетривиальное решение:** мы выбираем вариант D (score 5.4) вместо A (score 4.1), хотя A формально менее рискован. Причина: A не решает ключевые проблемы (adoption barrier, BMad зависимость, time-to-first-task). A — это оптимизация, не эволюция.

Детализация по категориям:

| Категория | A | B | C | D | D+ | Status Quo |
|-----------|---|---|---|---|----|----|
| Технические | 5.3 | 9.8 | 19.3 | 5.6 | 8.6 | — |
| Продуктовые | 3.3 | 5.0 | 16.7 | 6.0 | 11.3 | — |
| Реализация | 4.0 | 12.0 | 20.0 | 5.8 | 8.3 | — |
| **Общий средний** | **4.1** | **7.6** | **18.4** | **5.4** | **8.5** | **16.7** |

#### Что ИСКЛЮЧИТЬ

- **Вариант C (epics) — ИСКЛЮЧИТЬ.** Критический риск (18.4) по всем категориям. Context window overflow + потеря human control. Любая попытка сделать C работоспособным приводит к воссозданию stories.

- **Вариант B — НЕ делать.** D даёт ту же ценность с меньшим риском. B ломает scan.go + execute.md (score 12 каждый).

- **Вариант D+ до успеха D — НЕ делать.** Scope creep увеличивает риск без доказанной ценности.

---

## 8. План реализации

### 8.1 MVP (Epic 11 + Epic 15): 2.5 дня, 8 stories

```
Epic 11: ralph plan (6 stories, 2 дня)
  -> Epic 15: Bridge removal (2 stories, 0.5 дня)
```

**Результат:** `ralph plan` заменяет `ralph bridge`, bridge удалён.

**Epic 11: ralph plan — КРИТИЧЕСКИЙ ПУТЬ**

| Story | Размер | Описание |
|-------|--------|---------|
| 11.1 | S | Пакет planner/ scaffold: Plan(), PlanPrompt(), embedded plan.md |
| 11.2 | M | Document autodiscovery: DiscoverInputs(), файлы, stdin |
| 11.3 | M | Промпт plan.md: Go template, granularity rules, format contract |
| 11.4 | M | Core planner.Plan(): сборка промпта, вызов session, парсинг, merge mode |
| 11.5 | S | CLI cmd/ralph/plan.go: Cobra команда, --stdin, exit codes |
| 11.6 | M | Integration тесты: mock Claude, merge mode, stdin mode |

**Epic 15: Bridge removal**

| Story | Размер | Описание |
|-------|--------|---------|
| 15.1 | S | `ralph bridge` deprecated с warning |
| 15.2 | S | `bridge/` удалён (-2844 LOC) |

**Метрики успеха MVP:**
1. ralph plan генерирует >= 90% задач, которые бы создал bridge (проверка на 3 stories)
2. 0 тестов runner/ сломано после удаления bridge
3. Нетто-эффект: >= -1000 строк кода
4. Time-to-first-task <= 33 мин без ralph init, <= 5 мин с ralph init (vs 55-130 мин сейчас)

### 8.2 Рекомендуемый (+ Epic 12, 14): 3.5 дня, 15 stories

```
Epic 11 (2 дня) -> Epic 15 (0.5 дня) -> Epic 12 (1 день) = 3.5 дня
        |                                                     + 3 stories (Epic 14)
        +-- параллельно --> Epic 14 (1 день)                = 15 stories итого
```

**Epic 12: ralph replan (4 stories, 1 день)**

| Story | Размер | Описание |
|-------|--------|---------|
| 12.1 | S | Gate action [c] в gates/gates.go |
| 12.2 | M | runner/replan.go: ReplanFunc, промпт replan.md |
| 12.3 | M | Diff display + apply/edit/cancel flow |
| 12.4 | S | ValidateReplan + ForcePreserveDoneTasks |

**Epic 14: ralph init (3 stories, 1 день)**

| Story | Размер | Описание |
|-------|--------|---------|
| 14.1 | S | Flow 1: one-liner (`ralph init "описание"`) |
| 14.2 | M | Flow 2: interactive (`ralph init --interactive`) |
| 14.3 | M | Flow 3: brownfield (`ralph init --scan`) |

**Результат:** `ralph plan` + `ralph replan` + `ralph init` + bridge удалён. Полная самодостаточность.

### 8.3 Полный (+ Epic 13): 5 дней по критическому пути, 19 stories

Добавляется Epic 13: YAML Task Format (4 stories, 1.5 дня). Суммарный объём работ — 6 дней, но Epic 14 (init) параллелен Epic 11 (plan), поэтому критический путь = 5 дней.

| Story | Размер | Описание |
|-------|--------|---------|
| 13.1 | M | YAML structs + парсер (SprintTasks, Task types) |
| 13.2 | M | ScanTasksYAML + dual-format автодетект |
| 13.3 | M | planner генерирует YAML (зависит от Epic 11) |
| 13.4 | S | `ralph migrate-tasks` CLI (md -> yaml) |

**Рекомендация: НЕ включать Epic 13 в ближайший план.** GO/NO-GO через 1-2 месяца.

### 8.4 Зависимости между эпиками

```
Epic 11 (ralph plan)  <-- КРИТИЧЕСКИЙ ПУТЬ
     |
     +-- Epic 12 (ralph replan) <-- зависит от 11
     |
     +-- Epic 13 (YAML формат) <-- 13.3 зависит от 11, остальные независимы
     |
     +-- Epic 15 (Bridge Deprecation) <-- зависит от 11

Epic 14 (ralph init)  <-- ПОЛНОСТЬЮ НЕЗАВИСИМ, параллельно с 11
```

Epic 14 (init) можно делать параллельно с Epic 11 (plan) — полная независимость.

### 8.5 Что отложить и почему

**Epic 13 (YAML Task Format) — отложить:**
- Текущий markdown формат работает стабильно: 10 эпиков, ~500 задач, 0 regex-failures
- Regex-хрупкость — теоретическая проблема, не практическая
- GO/NO-GO решение через 1-2 месяца: если `ralph plan` генерирует задачи с edge cases, пересмотреть
- Migration burden: dual-format период, тесты, документация

**Epic 14 (ralph init) — по ситуации:**
- Если целевая аудитория — опытные разработчики: отложить
- Если целевая аудитория — массовый adoption: включить
- При публичном релизе: обязательно

**Стратегический reinit (Уровень 3 коррекции) — v3+:**
- Пересоздание requirements + sprint-tasks.md
- Слишком разрушительно для текущего формата
- Подождать feedback от ralph plan/replan

---

## 9. Что НЕ нужно делать

### 9.1 YAML формат — отложить

**Аргументы за YAML:**
- Устраняет 4 regex'а
- Добавляет ID, зависимости, метаданные
- Программный merge (100% vs ~85%)

**Аргументы за отложение (перевешивают):**
- 10 эпиков, ~500 задач — ни одного regex failure
- Migration burden > текущая боль
- ralph plan может генерировать markdown (формат сохранён) — YAML не обязателен для MVP
- Решение принимать через 1-2 месяца на основе реального опыта с ralph plan

**Вердикт:** GO/NO-GO через 1-2 месяца. Если regex ломается на output ralph plan — делать. Если нет — не нужно.

### 9.2 Встраивание BMad workflows — не нужно (89% избыточно)

Из 29600 строк BMad промптов:
- 89% (~26342 строк) — pre-implementation, визуализация, QA, infrastructure — не нужны ralph
- 11% (~3258 строк) — уже покрыты ralph execute/review на 80-95%
- Единственная фича с ROI > 1 — `ralph plan` (которая реализуется как отдельная команда, а не встраивание BMad)

ralph и BMad — **complementary tools**, а не конкуренты:
- BMad: планирование, design, story creation (высокоинтерактивные)
- Ralph: автономное выполнение задач (низкоинтерактивный loop)
- Граница чёткая: BMad создает артефакты -> Ralph их исполняет (если пользователь хочет BMad-уровень планирования)
- ralph plan — альтернатива для случаев, когда BMad-уровень планирования не нужен

### 9.3 Мульти-агентная архитектура — не сейчас

**Аргументы за мульти-агентность:**
- MetaGPT: 5+ ролей, SOP pipeline
- Gas Town: 20-30 параллельных агентов
- Claude Code Agent Teams: peer-to-peer координация

**Аргументы против (для ralph):**
- Стоимость: Gas Town $2-5k/мес vs ralph $20-200/мес (10-25x разница)
- Merge conflicts: 20-30 агентов создают больше конфликтов, чем решают задач
- Design bottleneck: скорость ограничена планированием, не кодированием
- Sequential pipeline ralph предсказуемее и проще в отладке
- ralph уже использует 4-агентную модель через manual orchestration (creator/validator/developer/reviewer)

**Вердикт:** v3+. Только если Claude Code Agent Teams станет стабильным API и ralph сможет его использовать как runtime.

### 9.4 Другие отвергнутые идеи

**1. LLM-agnostic абстракция (поддержка разных LLM)**
- ralph жёстко привязан к Claude Code (session.go вызывает claude binary)
- Абстракция потребует масштабного рефакторинга
- Нет доказательства спроса от пользователей
- Вердикт: не делать без явного запроса

**2. Прямой запуск без init/plan (ralph run "описание")**
- Идея: `ralph run "Добавь JWT авторизацию"` -> ralph plan автоматически -> sprint-tasks -> выполнение
- Риск: скрытый planning step может создать неожиданные задачи
- Вердикт: потенциально интересно, но после стабилизации ralph plan

**3. Event-driven архитектура**
- MS Agent Framework, AutoGen v0.4 используют event-driven
- Непредсказуемость поведения
- ralph ценит предсказуемость и тестируемость
- Вердикт: не подходит для ralph

**4. Database-backed state (вместо файлов)**
- LangGraph использует Postgres для checkpoints
- ralph — CLI-инструмент, не сервер
- Файловый подход + git = оптимальная точка для CLI
- Вердикт: overengineering

**5. Bi-directional spec sync (Kiro-style)**
- Kiro: обновление specs на основе кода
- Требует постоянного мониторинга файловой системы
- Сложность реализации не оправдана для CLI
- Вердикт: потенциально v3+

---

## 10. Точки расхождения между агентами

### 10.1 Replan: обязательно vs не нужно

**Позиция "обязательно" (аналитик roadmap):**
> "3.5 дня, 15 stories, включая Epic 12 (replan). Replan — часть рекомендуемого path."

**Позиция "осторожно" (аналитик risks):**
> "Replan score 8.5 (D+). Сначала D, потом D+ если D успешен."

**Позиция "не нужно" (аналитик workflow-replication):**
> "replan — маргинальная ценность, можно просто отредактировать tasks.md"

**Разрешение:** replan в рекомендуемый path, но НЕ в MVP. Пользователь может редактировать sprint-tasks.md вручную до реализации replan. Это working solution, пусть и менее удобное.

### 10.2 YAML vs Markdown

**Позиция "YAML" (аналитик roadmap):**
> "YAML Task Format — машиночитаемый формат, устраняет regex-хрупкость. Epic 13, 4 stories."

**Позиция "Markdown" (архитектурное решение проекта + аналитик risks):**
> "Формат sprint-tasks.md стабилен, 500+ задач без проблем. YAML = migration burden."

**Разрешение:** отложить YAML. Аналитик roadmap создал полный Epic 13 (4 stories), что может привести к scope creep. GO/NO-GO через 1-2 месяца.

### 10.3 Самодостаточность vs каннибализация BMad

**Проблема:** ralph plan делает часть работы BMad (task decomposition). Это сознательная каннибализация?

**Аналитик risks:** "ralph plan делает часть работы BMad, но НЕ делает PRD/Architecture/Epics. Разные уровни абстракции."

**Аналитик competitive:** "Самодостаточность = survival. Все конкуренты самодостаточны."

**Компромисс:** ralph plan принимает stories как input (`ralph plan story1.md`). BMad workflow не ломается, появляется альтернатива для случаев, когда BMad-уровень планирования не нужен.

### 10.4 Ценность BMad workflows

**Жёсткая позиция (аналитик workflow-replication):**
> "НЕ встраивать BMad workflows. Ralph и BMad — complementary tools."

**Нюансированная позиция (аналитик story-generation):**
> "Суммарный объём для полного встраивания: ~900-1000 строк промптов. Минимальный MVP: ~400-500 строк."

**Оценка покрытия create-story:**

| Аналитик | Оценка покрытия |
|----------|----------------|
| workflow-replication | 80% |
| story-generation | 67% (4 из 6 шагов) |

Расхождение: первый считает встраивание ненужным, второй — технически выполнимым. Оба согласны: `ralph plan` — единственная фича с ROI > 1.

**Непокрытые шаги create-story:** web research (актуальные версии библиотек) и validate (LLM-as-judge). Оба низкоприоритетны для ralph.

### 10.5 Декомпозиция Epic 11: 9 stories (архитекторы) vs 6 stories (roadmap)

Архитектурный отчёт (`v2-ralph-plan-architecture.md`) описывает реализацию ralph plan в 9 stories (3 фазы: 5 core + 2 merge/compat + 2 bridge deprecation). Аналитический отчёт roadmap (`v2-migration-roadmap.md`) описывает Epic 11 как 6 stories (core planner), а bridge deprecation вынесен в отдельный Epic 15 (2 stories).

Итоговый результат одинаков (8 stories на MVP path), но границы эпиков разные. SUMMARY использует roadmap-декомпозицию (Epic 11 = 6 stories + Epic 15 = 2 stories) как более модульную: bridge deprecation независима от core planner и может быть отложена.

### 10.6 Открытые вопросы (7 штук)

**1. Качество plan.md промпта — нет эмпирических данных.**

Ни один аналитик не тестировал промпт на реальных данных. Risk score D-R2 = 12 (самый высокий для D), но нет эмпирической базы. Идеальный test case: подать PRD learnPlatform в ralph plan и сравнить output с реальными 93 задачами.

**2. LLM-agnostic абстракция — стоимость неизвестна.**

Аналитик competitive рекомендует "рассмотреть LLM-agnostic абстракцию" для снижения vendor lock-in. Но ralph жёстко привязан к Claude Code (session.go вызывает claude binary), абстракция потребует масштабного рефакторинга. Стоимость никто не оценил.

**3. Timing deprecation bridge — неопределённость.**

Roadmap предлагает "быстрый переход Phase 1->3 (2.5 дня)". Но если plan.md промпт окажется некачественным (risk D-R2), bridge придётся поддерживать параллельно неопределённое время.

**Рекомендация:** deprecation с warning в Phase 2, удаление в Phase 3 ТОЛЬКО после подтверждения качества ralph plan на 3+ реальных проектах.

**4. Тестирование plan.md на learnPlatform — никто не предложил.**

learnPlatform audit дал детальные данные о качестве BMad-артефактов. Идеальный тест: подать PRD learnPlatform в ralph plan и сравнить с реальными 93 задачами. Этот эксперимент критически важен, но ни один из 16 аналитиков его не предложил.

**5. Claude Code Agent Teams vs ralph — нет contingency plan.**

Если Anthropic добавит task persistence и review loop в Agent Teams, ralph теряет значительную часть value. Нет плана на этот сценарий. Рекомендация: мониторить announcements Anthropic, быть готовым к pivot (ralph как "smart layer" поверх Agent Teams).

**6. Context window observability как продукт — нет development roadmap.**

Все аналитики отмечают уникальность FR75-FR92 (context window observability), но никто не предлагает конкретный план монетизации или дальнейшего развития. Это упущение: уникальная фича без конкурентов заслуживает отдельного roadmap.

**7. Реальная стоимость ralph vs конкурентов — сравнение неполно.**

Конкурентный анализ содержит цены конкурентов ($20/мес Devin, $20-200/мес Claude Code), но нет оценки полной стоимости ralph pipeline (BMad сессии + bridge + run + review = N токенов = $X). Без этого сравнение с конкурентами неточно.

---

## 11. Итоговый вердикт и рекомендации

### Что делать

**Вариант D: ralph plan.** Новая команда, новый пакет `planner/`, принимающий любой текстовый input и генерирующий `sprint-tasks.md` напрямую.

### В каком порядке

**Фаза 1 — MVP (2.5 дня, обязательно):**
1. Epic 11: ralph plan (6 stories) — planner/, промпт plan.md, CLI
2. Epic 15: Bridge removal (2 stories) — deprecation + deletion

**Фаза 2 — Рекомендуемая (+1.5 дня):**
3. Epic 12: ralph replan (4 stories) — gate action [c], diff display
4. Epic 14: ralph init (3 stories) — quick start (one-liner, interactive, brownfield)

**Фаза 3 — Отложить (GO/NO-GO через 1-2 месяца):**
5. Epic 13: YAML format (4 stories) — только если markdown ломается на practice

### С какими ожиданиями

**Реалистичные ожидания:**

| Метрика | Текущий (v1) | Целевой (v2 MVP) | Изменение |
|---------|-------------|-------------------|-----------|
| Time-to-first-task (plan only) | 55-130 мин | 11-33 мин | **3-5x быстрее** |
| Time-to-first-task (с init) | 55-130 мин | 2-5 мин | **20-60x быстрее** |
| Стоимость planning | $2.50-9.80 | $0.10-0.30 | **90-97% экономия** |
| BMad-зависимость | Полная | Нулевая | **Устранена** |
| Промпт planning | 244 строки | 95 строк | **-61%** |
| Merge надёжность | ~85% | 100% | **Детерминистичный** |
| LOC codebase | +2844 (bridge/) | -2844 bridge + ~270 planner | **-1324 нетто** |
| Гибкость форматов | 1/10 | 9/10 | **Любой текст** |

**Что НЕ ожидать:**
- Качество декомпозиции НЕ будет равно BMad stories (7/10 vs 8.7/10). Но 7/10 достаточно для продуктивной работы.
- ralph plan НЕ заменяет BMad для complex enterprise projects. BMad остаётся опциональным для тех, кому нужен полный PMO-уровень планирования.
- Первая версия промпта plan.md НЕ будет идеальной. Планировать 2-3 итерации на доработку промпта по результатам реальных test cases.

### Ключевые метрики успеха

| Метрика | Порог успеха | Как измерить |
|---------|-------------|-------------|
| Качество plan output vs bridge | >= 90% покрытие задач | Ручная проверка на 3 stories |
| Сломанных runner тестов | 0 | `go test ./runner/...` |
| Нетто-эффект строк кода | >= -1000 (упрощение) | `wc -l` до и после |
| Quality findings per story | <= 4.0 | code-review workflow |
| Time-to-first-task (MVP, без init) | <= 33 мин (vs 55-130 мин сейчас) | Измерение от создания requirements.md до первого `ralph run` |
| Time-to-first-task (с ralph init) | <= 5 мин (vs 55-130 мин) | Измерение от `ralph init` до первого `ralph run` |
| ralph plan работает с stdin | Да | `cat issues.md | ralph plan --stdin` |

### Риски для мониторинга

| Риск | Score | Триггер эскалации |
|------|-------|-------------------|
| Качество plan.md (D-R2) | 12 | Plan output значительно хуже bridge на 3+ cases |
| Scope creep к BMad (D-P3) | 12 | Запросы на генерацию stories/architecture |
| Claude Code Agent Teams | — | Anthropic добавляет task persistence + review |
| Kiro spec-driven | — | AWS добавляет review pipeline + knowledge |

### Действия сразу после MVP

После успешной реализации MVP (Epic 11 + Epic 15) — 3 обязательных шага:

**1. Эмпирическая валидация промпта plan.md:**
- Подать PRD learnPlatform в ralph plan
- Сравнить output с реальными 93 задачами sprint-tasks.old.md
- Измерить: покрытие FR, гранулярность, точность source-ссылок
- Минимум 3 test case из разных доменов (не только Go)

**2. Замер baseline-метрик:**
- Time-to-first-task с ralph plan (включая правку requirements.md)
- Стоимость в токенах одного вызова ralph plan
- Сравнение с bridge по тем же проектам

**3. Feedback loop:**
- Использовать ralph plan для собственной разработки (Epic 12, 14)
- Фиксировать edge cases промпта
- Итерация: v1 -> feedback -> v2 (не позднее 5 дней после MVP)

### Contingency plan

**Если plan.md промпт не даёт 90% покрытия задач:**
- Не удалять bridge (Phase 3 откладывается)
- Итерация промпта: добавить few-shot examples, усилить granularity rules
- Рассмотреть two-step fallback (classify -> generate)
- Worst case: вернуться к Варианту A (программный парсинг) как interim

**Если Claude Code Agent Teams станет production-ready:**
- ralph plan остаётся ценным (planning не зависит от runtime)
- ralph run может использовать Agent Teams как runtime вместо одиночных сессий
- Knowledge management и observability остаются unique value
- Pivot: ralph как "smart orchestration layer" поверх Agent Teams

**Если Kiro добавит review pipeline:**
- ralph сохраняет преимущество в knowledge lifecycle (extraction -> distillation -> injection)
- Context window observability остаётся уникальной
- CLI-подход vs IDE — разные аудитории
- Усилить то, что Kiro не покрывает: adversarial review с 5 agents

### Финальное резюме

16 агентов (8 архитекторов + 8 аналитиков) исследовали эволюцию ralph v2 с разных сторон: конкурентный ландшафт, аудит реального проекта, качество генерации, BMad workflows, риски, промптовые стратегии, архитектурные паттерны.

**Консенсус по 5 ключевым пунктам:**

1. **Bridge — антипаттерн** (подтверждено 4 независимыми отчётами из разных углов)
2. **Status quo неприемлем** (risk score 16.7 — хуже любого варианта действий)
3. **Вариант D оптимален** (взвешенная оценка 8.50, отрыв 3.65 от ближайшего конкурента)
4. **89% BMad не нужно** ralph (только 2 из 12 типов документов используются runner'ом)
5. **Конкурентное преимущество** ralph — комбинация structured task loop + adversarial code review + knowledge lifecycle + context observability (ни один конкурент не покрывает все 4)

**Единственный реальный спор:** timing и scope дополнительных фич (replan, init, YAML). MVP бесспорен.

Рекомендация: **начинать с MVP (Epic 11 + Epic 15), 2.5 дня. Это минимальное вложение для устранения стратегической проблемы BMad-зависимости.**

---

## Приложение A: Источники исследований

### Архитектурные отчёты (8)

1. `v2-variants-comparison.md` — Сравнение 4 вариантов эволюции
2. `v2-agent-orchestrators.md` — 11 инструментов-оркестраторов
3. `v2-bmad-document-formats.md` — BMad v6 документы (12 типов)
4. `v2-ralph-plan-architecture.md` — Архитектура ralph plan
5. `v2-ralph-init-design.md` — Дизайн ralph init
6. `v2-replan-correct-course.md` — Дизайн ralph replan
7. `v2-yaml-task-format.md` — YAML формат задач
8. `v2-prompt-chain-design.md` — Промпты plan.md и replan.md

### Аналитические отчёты (8)

1. `v2-learnplatform-audit.md` — Аудит learnPlatform
2. `v2-bmad-story-generation.md` — Pipeline генерации stories
3. `v2-gastown-orchestrators.md` — Gastown + фреймворки
4. `v2-quality-comparison.md` — Качество Bridge vs Stories vs PRD
5. `v2-risk-analysis.md` — Анализ рисков
6. `v2-migration-roadmap.md` — Roadmap миграции
7. `v2-competitive-landscape.md` — Конкурентный ландшафт (20 инструментов)
8. `v2-bmad-workflow-replication.md` — Репликация BMad workflows

### Промежуточные синтезы (2)

1. `SYNTHESIS-ARCH.md` — Синтез 8 архитектурных отчётов
2. `SYNTHESIS-ANALYSIS.md` — Синтез 8 аналитических отчётов

### Академические работы (цитируемые в отчётах)

- ADaPT: As-Needed Decomposition and Planning (NAACL 2024, Allen AI)
- Requirements are All You Need (2024)
- MetaGPT: Meta Programming for Multi-Agent Framework (ICLR 2024)
- SWE-Agent: Agent-Computer Interfaces (NeurIPS 2024, Princeton)
- AutoCodeRover (ISSTA 2024, NUS)
- OpenHands (ICLR 2025)
- Blueprint First, Model Second (2025)
- Chain-of-Thought Prompting Elicits Reasoning (2022)

### Инструменты (20 исследованных)

Devin, Codex, Claude Code, Amazon Q, Jules, Cursor, Windsurf, GitHub Copilot, Kiro, Junie, Aider, SWE-Agent, OpenHands, AutoCodeRover, MetaGPT, CrewAI, Claude Code Agent Teams, BMad Method, Taskmaster AI, Claude Code Skills.

---

## Приложение B: Глоссарий

| Термин | Определение |
|--------|------------|
| **Bridge** | Пакет `bridge/` в ralph v1. Конвертирует BMad story файлы в sprint-tasks.md через LLM |
| **Runner** | Пакет `runner/` в ralph. Оркестрирует Claude Code сессии для последовательного выполнения задач |
| **Sprint-tasks.md** | Файл с задачами в формате markdown checklist. Входные данные для `ralph run` |
| **Planner** | Предлагаемый новый пакет `planner/` для ralph v2. Заменяет bridge |
| **Gate** | Точка контроля: ralph приостанавливается и ждёт решения человека (approve/retry/skip/quit) |
| **BMad Method** | YAML-based workflow engine для structured development. 8 ролей, 4 фазы, 12 типов документов |
| **ADaPT** | As-Needed Decomposition and Planning with Tasks (Allen AI, NAACL 2024). Рекурсивная декомпозиция при неудаче |
| **CoT-in-JSON** | Промптовая стратегия: поле `analysis` в JSON output как Chain-of-Thought зона |
| **Blueprint First** | Архитектурный принцип: Go код как детерминистический scaffold, LLM только для семантического анализа |
| **ACI** | Agent-Computer Interface (Princeton). Оптимизация интерфейса взаимодействия агента с инструментами |
| **Spec-driven** | Подход к разработке: сначала спецификация, потом код (Kiro, Copilot Workspace) |
| **Beads** | Атомарные единицы работы в Gas Town (git-backed JSON) |
| **SOP** | Standard Operating Procedure. Формализованные процедуры для каждой роли (MetaGPT) |

---

## Приложение C: Перекрёстные ссылки между секциями

Ключевые тезисы отчёта подтверждаются данными из нескольких секций:

**"Bridge — антипаттерн":**
- Секция 2.1: 5 фундаментальных проблем (архитектура)
- Секция 2.2: аудит learnPlatform (эмпирические данные)
- Секция 2.3: метрики качества (количественный анализ)
- Секция 3.2: антипаттерны оркестраторов (fidelity loss, no steering)
- Секция 7.2: risk score Status Quo = 16.7

**"Вариант D оптимален":**
- Секция 5.5: взвешенная оценка 8.50 (отрыв 3.65)
- Секция 5.7: 5 ключевых аргументов
- Секция 7.1: risk score D = 5.4 (второй после A = 4.1)
- Секция 7.4: нетривиальное решение — D выше по ценности при чуть большем риске
- Секция 3.4: конкурентный паритет (все самодостаточны)

**"89% BMad не нужно":**
- Секция 4.1: 12 типов документов, 2 используются runner
- Секция 4.3: 26342 строки из 29600 не нужны
- Секция 4.4: оставшиеся 11% уже покрыты ralph execute/review
- Секция 4.5: вердикт — complementary tools

**"Самодостаточность = survival":**
- Секция 3.1: все 20 конкурентов самодостаточны
- Секция 3.5: 2 критические угрозы (Agent Teams, Kiro)
- Секция 7.2: SQ-7 (ralph без BMad неработоспособен) = score 25
- Секция 2.4: adoption barrier, конкурентная аномалия

---

## Changelog ревью

**Ревьюер:** независимый ревьюер-редактор
**Дата:** 2026-03-07
**Проверено:** SUMMARY.md (итоговый отчёт) сверен с SYNTHESIS-ARCH.md и SYNTHESIS-ANALYSIS.md

### Исправленные фактические ошибки

1. **Time-to-first-task: несогласованность 55-135 vs 55-130 мин (строка 722).** В таблице Варианта A стояло "55-135 мин", во всех остальных местах документа -- "55-130 мин". Унифицировано на 55-130 мин.

2. **Time-to-first-task v2: завышенные ожидания в итоговой таблице.** В секции 11 (вердикт) целевой показатель "2-5 мин" предполагает наличие `ralph init` (Epic 14), который НЕ входит в MVP. MVP без init даёт 11-33 мин. Разбито на две строки: "plan only" (11-33 мин) и "с init" (2-5 мин).

3. **Метрики успеха MVP: Time-to-first-task <= 5 мин (строка 1562).** Уточнено: "<= 33 мин без ralph init, <= 5 мин с ralph init" -- различие между MVP (без init) и рекомендуемым path (с init).

4. **Финальная таблица метрик успеха (строка 1844).** Аналогично разбита на MVP (без init) и полный вариант (с init).

5. **Risk score "12 из 25" в Executive Summary (строка 53).** Формулировка вводила в заблуждение -- не шкала 0-25, а конкретный risk score. Пояснено: "risk score 12 по шкале вероятность*impact, максимум 25".

6. **Взвешенные оценки не воспроизводятся из таблицы (4.85, 3.95, 2.45, 8.50).** Пересчёт по целочисленным оценкам из таблицы даёт другие значения (5.15, 4.00, 2.65, 8.60). Причина: оценки в первоисточнике содержат дробные значения, в таблице округлены. Добавлено примечание.

7. **"Полный вариант: 5 дней, 19 stories" (строка 1602).** Уточнено: "5 дней по критическому пути" с пояснением, что суммарный объём = 6 дней (Epic 14 параллелен Epic 11).

### Добавленный контент (пропуски из синтезов)

8. **Тренды 2025-2026 (из SYNTHESIS-ARCH 2.5).** 4 ключевых тренда (spec-driven > prompt-driven, hybrid decomposition, persistent state, steering points) отсутствовали в SUMMARY. Добавлены в секцию 3.2.

9. **"Два лагеря" фреймворков (из SYNTHESIS-ANALYSIS 4.9).** General-purpose vs code-specific orchestrators -- добавлено в секцию 3.2.

10. **Каннибализация BMad (из SYNTHESIS-ANALYSIS 8.3.1).** Спорный вопрос -- делает ли ralph plan часть работы BMad? -- не попал в секцию точек расхождения. Добавлен как подраздел 10.3.

11. **Декомпозиция Epic 11: расхождение архитекторов и roadmap.** Архитектурный отчёт описывает 9 stories (3 фазы), roadmap -- 6 stories + Epic 15 (2 stories). Итог одинаков (8 stories MVP), но границы эпиков разные. Добавлен подраздел 10.5.

12. **Windsurf: #1 AI Dev Tool Rankings.** Пропущена характеристика из SYNTHESIS-ANALYSIS. Восстановлена в таблице Tier 2.

### Проверено и подтверждено (без ошибок)

- Взвешенные оценки вариантов (A=4.85, B=3.95, C=2.45, D=8.50) -- совпадают во всех файлах
- Risk scores (A=4.1, D=5.4, B=7.6, D+=8.5, StatusQuo=16.7, C=18.4) -- совпадают
- Количество конкурентов (20 инструментов) -- корректно, все перечислены
- 7 открытых вопросов -- совпадают с SYNTHESIS-ANALYSIS
- Все 4 диагноза bridge как антипаттерна -- корректно отражены
- Стоимостные оценки ($0.10-0.30 vs $2.50-9.80) -- совпадают
- Процент BMad промптов "89% не нужны" = 26342/29600 = 89% -- корректно
- Нетто-эффект -1324 строки: формула = (120 Go + 150 промпт + 1250 тестов) - 2844 bridge = 1520 - 2844 = -1324. Корректно (тесты включены в подсчёт).

### Отмеченные, но не исправленные

- **82 story в заголовке:** подсчёт из MEMORY.md даёт 88 stories (13+7+11+8+6+10+10+7+9+7), sprint-artifacts содержит 86 story-файлов. Число "82" взято из первоисточника (SYNTHESIS-ARCH), оставлено как есть -- требует уточнения у автора исследований.
- **Повторы между разделами:** разделы 2, 5, 7, 9, 11 содержат пересекающийся контент (таблицы, аргументы). Это допустимо для документа принятия решений, где каждый раздел должен быть самодостаточным. Приложение C обеспечивает перекрёстные ссылки.
