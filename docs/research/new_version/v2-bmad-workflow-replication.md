# Исследование: Встраивание BMad Workflows в Ralph v2

Дата: 2026-03-07
Контекст: Анализ всех BMad workflow файлов для оценки целесообразности репликации в ralph.

---

## 1. Полный каталог BMad Workflows

### 1.1 Фаза 1 — Анализ (pre-implementation)

| Workflow | Описание | Инструкции (строк) | Интерактивность | Зависимости |
|----------|---------|---------------------|-----------------|-------------|
| **product-brief** | Создание Product Brief через 6 step-файлов | ~1200 (6 steps) | Высокая (диалог на каждом шаге) | Нет |
| **research** (domain/market/technical) | 3 трека исследований по 6 шагов каждый | ~3700 (18 step-файлов) | Высокая (web search + диалог) | Нет |

### 1.2 Фаза 2 — Планирование (pre-implementation)

| Workflow | Описание | Инструкции (строк) | Интерактивность | Зависимости |
|----------|---------|---------------------|-----------------|-------------|
| **prd** | Создание PRD через 11 шагов | ~2573 (11 steps) | Высокая (checkpoint на каждом шаге) | product-brief |
| **create-ux-design** | UX дизайн через 14 шагов | ~2702 (14 steps) | Высокая | PRD |

### 1.3 Фаза 3 — Solutioning (граница pre-impl / implementation)

| Workflow | Описание | Инструкции (строк) | Интерактивность | Зависимости |
|----------|---------|---------------------|-----------------|-------------|
| **architecture** | Architecture doc через 8 шагов | ~2362 (8 steps) | Высокая | PRD |
| **create-epics-and-stories** | Декомпозиция PRD+Architecture в эпики/стори | 387 + 80 template | Средняя (checkpoints) | PRD + Architecture |
| **implementation-readiness** | Валидация готовности к Phase 4 | 332 + 169 checklist | Средняя | PRD + Architecture + Epics |

### 1.4 Фаза 4 — Implementation (ядро разработки)

| Workflow | Описание | Инструкции (строк) | Checklist (строк) | Интерактивность | Зависимости |
|----------|---------|---------------------|-------------------|-----------------|-------------|
| **sprint-planning** | Генерация sprint-status.yaml из эпиков | 234 + 33 + 56 template | 33 | Низкая | Epics |
| **create-story** | Создание файла стори с полным контекстом | 323 + 358 checklist + 51 template | 358 | Низкая (автоматизируема) | sprint-status + Epics |
| **dev-story** | Реализация стори (red-green-refactor) | 405 + 80 checklist | 80 | Низкая (автономна) | Story file |
| **code-review** | Adversarial code review | 237 | 0 (встроено) | Низкая | Story file + Git |
| **correct-course** | Управление изменениями в спринте | 206 + 279 checklist | 279 | Высокая (диалог) | PRD + Epics + Architecture |
| **retrospective** | Ретроспектива по эпику | 1443 | 0 | Средняя (party mode) | Completed epic + Stories |

### 1.5 Вспомогательные workflows

| Workflow | Описание | Инструкции (строк) | Цель |
|----------|---------|---------------------|------|
| **bmad-quick-flow/create-tech-spec** | Упрощенная Tech Spec | 115 | Быстрый старт без PRD |
| **bmad-quick-flow/quick-dev** | Быстрая разработка по tech-spec | 105 + 25 | Быстрый старт |
| **diagrams/** (4 шт) | Excalidraw диаграммы | 645 (все вместе) | Визуализация |
| **document-project** | Документирование brownfield проекта | 222 + 1106 (full-scan) + 298 (deep-dive) | Brownfield |
| **generate-project-context** | project-context.md | ~787 (3 steps) | Контекст |
| **testarch/** (8 шт) | Test Architecture workflows | ~6327 instructions + ~3283 checklists | QA |
| **workflow-status** | Трекинг прогресса workflow | 346 + 395 | Мета-координация |

### 1.6 Суммарный объём промптов

| Категория | Строк инструкций | Строк чеклистов | Итого |
|-----------|-----------------|-----------------|-------|
| Фаза 1 (анализ) | ~4900 | 0 | ~4900 |
| Фаза 2 (планирование) | ~5275 | 0 | ~5275 |
| Фаза 3 (solutioning) | ~3131 | 169 | ~3300 |
| Фаза 4 (implementation) | ~2848 | 750 | ~3598 |
| Вспомогательные | ~9246 | ~3283 | ~12529 |
| **ИТОГО** | **~25400** | **~4202** | **~29600** |

---

## 2. Оценка встраиваемости в Ralph

### 2.1 Классификация workflows

| Workflow | Нужен ralph? | Обоснование |
|----------|-------------|-------------|
| product-brief | **Нет** | Pre-implementation, domain пользователя |
| research (3 трека) | **Нет** | Pre-implementation, domain пользователя |
| prd | **Нет** | Pre-implementation, domain пользователя |
| create-ux-design | **Нет** | Pre-implementation, domain пользователя |
| architecture | **Нет** | Pre-implementation, domain пользователя |
| diagrams (4 шт) | **Нет** | Визуализация, domain пользователя |
| document-project | **Нет** | Brownfield onboarding, domain пользователя |
| testarch (8 шт) | **Нет** | QA планирование, domain пользователя |
| workflow-status | **Нет** | Мета-координация BMad, не нужна ralph |
| generate-project-context | **Нет** | Разовая задача, CLI уже генерит project-context.md |
| bmad-quick-flow (2 шт) | **Нет** | Упрощенный трек, ralph заменяет полностью |
| --- | --- | --- |
| **create-epics-and-stories** | **Рассмотреть** | `ralph plan` — декомпозиция задач |
| **implementation-readiness** | **Нет** | Валидация артефактов, не автоматизируема |
| **sprint-planning** | **Частично** | ralph уже управляет задачами через tasks.md |
| **create-story** | **Рассмотреть** | Подготовка контекста для сессии |
| **dev-story** | **Уже есть** | ralph execute — это dev-story |
| **code-review** | **Уже есть** | ralph review — это code-review |
| **correct-course** | **Рассмотреть** | `ralph replan` — пересмотр плана |
| **retrospective** | **Опционально** | `ralph retro` — анализ метрик |

### 2.2 Детальная оценка кандидатов

#### A. create-epics-and-stories -> `ralph plan`

**Что делает BMad:**
- Читает PRD + Architecture + UX
- Декомпозирует функциональные требования в эпики/стори
- Проверяет FR coverage
- Генерирует epics.md

**Что нужно ralph:**
- Читает описание задачи (issue / brief / PRD)
- Декомпозирует в tasks.md (уже существует формат)
- Не нужны эпики — ralph работает на уровне отдельных задач

**Оценка:**
- **Размер промпта:** Из 387 строк BMad ~60-80 строк релевантны для ralph (принципы декомпозиции, sizing, зависимости)
- **Go-код:** ~200-300 LOC (чтение input, генерация tasks.md, валидация)
- **Ценность:** Средняя. Ralph уже имеет ScanTasks для разбора задач. Plan mode — это новая функция, но промпт можно сделать проще
- **Сложность:** M (средняя)

#### B. create-story -> контекстное обогащение задачи

**Что делает BMad:**
- Анализирует sprint-status.yaml, находит следующую backlog стори
- Загружает эпик, архитектуру, предыдущую стори, git историю
- Создает "ultimate developer guide" с полным контекстом
- Web research для актуальных версий библиотек

**Что нужно ralph:**
- Ralph уже строит промпт execute с контекстом задачи, проекта, предыдущих результатов
- 80% функциональности create-story уже реализовано в ralph через:
  - `runner/prompts/execute.md` (130 строк)
  - `config.Config` (project context)
  - `runner/scan.go` (task discovery)
  - `session/session.go` (контекстное обогащение)

**Оценка:**
- **Размер промпта:** 0 дополнительных строк — ralph уже делает это
- **Go-код:** 0 LOC — уже реализовано
- **Ценность:** Нулевая (дублирование)
- **Вывод:** НЕ встраивать. Ralph уже покрывает эту функцию нативно

#### C. dev-story -> `ralph execute`

**Что делает BMad:**
- 10-шаговый workflow: discovery -> context -> review continuation -> mark in-progress -> implement (red-green-refactor) -> test -> validate -> mark complete -> communicate
- Sprint-status.yaml обновления
- Review follow-up обработка

**Что уже делает ralph:**
- `runner.Execute()` — полный цикл выполнения задач
- `runner/prompts/execute.md` — промпт с task/subtask чеклистом
- `gates/` — human gates (approve/skip/retry)
- `session/` — управление Claude Code сессиями
- `runner/metrics.go` — метрики выполнения

**Оценка:**
- **Уже реализовано на ~90%**
- BMad dev-story = ralph execute + review continuation
- Из 405 строк BMad инструкций, ralph использует суть в 130 строках execute.md (более компактно)
- **Ценные идеи из BMad для заимствования:**
  - Review continuation (step 3): обнаружение review follow-ups и приоритетная обработка. Ralph уже имеет это через gate retry mechanism
  - Definition of Done checklist (80 строк): ralph имеет аналог через gate checks

#### D. code-review -> `ralph review`

**Что делает BMad:**
- Adversarial review: AC validation, task audit, code quality, test quality
- Git vs story discrepancies
- Auto-fix with user approval
- Knowledge extraction (step 6): обновление rules, violation-tracker, memory

**Что уже делает ralph:**
- `runner/prompts/review.md` (176 строк) — adversarial review с 5 review agents
- `runner/prompts/agents/` (5 agent промптов, ~182 строки)
- Progressive review types (full/patch/architecture)
- Severity-based filtering
- Scope-creep protection

**Оценка:**
- **Уже реализовано на ~95%**
- Ralph review даже мощнее BMad: 5 специализированных review agents vs 1 monolithic BMad reviewer
- Knowledge extraction (step 6, 60 строк) — уникальная для BMad фича, но ralph имеет `runner/knowledge_*.go` для управления знаниями
- **Ценные идеи из BMad:** Step 6 (knowledge extraction) — систематическое обновление rules файлов после каждого review. Ralph делает distill, но не обновляет rules автоматически

#### E. correct-course -> `ralph replan`

**Что делает BMad:**
- Анализ триггера изменений
- Impact assessment по эпикам и артефактам
- 3 опции: Direct Adjustment / Rollback / MVP Review
- Sprint Change Proposal документ
- Agent handoff план

**Что нужно ralph:**
- Пересмотр tasks.md когда задача заблокирована или требования изменились
- Анализ what went wrong и корректировка плана
- Не нужен Sprint Change Proposal — ralph работает проще

**Оценка:**
- **Размер промпта:** Из 206+279 строк BMad, для ralph нужно ~60-80 строк (анализ блокера, пересмотр задач, обновление tasks.md)
- **Go-код:** ~150-200 LOC (новый replan mode в runner, обновление tasks.md)
- **Ценность:** Средняя. Полезно для долгих проектов с множеством задач
- **Сложность:** M

#### F. sprint-planning -> управление задачами в ralph

**Что делает BMad:**
- Парсит эпики и генерирует sprint-status.yaml
- Status state machine: backlog -> drafted -> ready-for-dev -> in-progress -> review -> done

**Что уже делает ralph:**
- `runner/scan.go` — парсит tasks.md и определяет open/done задачи
- Статусы задач управляются через чеклисты в tasks.md

**Оценка:**
- Ralph уже управляет задачами, но проще (open/done vs 6 состояний BMad)
- 6-state machine BMad избыточна для ralph — ralph не различает "drafted" vs "ready-for-dev"
- **Вывод:** НЕ встраивать. Текущая система задач ralph достаточна

#### G. retrospective -> `ralph retro`

**Что делает BMad:**
- Epic Discovery по sprint-status
- Party mode (ролевая дискуссия между "агентами")
- Количественные метрики: velocity, story points, cycle time
- Lessons learned extraction
- Next epic preparation

**Что может делать ralph:**
- У ralph уже есть RunMetrics (latency, token costs, task completion)
- Может анализировать session logs
- Может генерировать отчёт по метрикам

**Оценка:**
- **Размер промпта:** Из 1443 строк BMad (самый большой!), для ralph нужно ~50-80 строк
- **Go-код:** ~100-150 LOC (сбор метрик из RunMetrics, форматирование отчёта)
- **Ценность:** Низкая-Средняя. Метрики уже собираются, но нет UI для их просмотра
- **Сложность:** S (маленькая)
- 1443 строки BMad retrospective — это party mode с ролевыми диалогами, что абсолютно не нужно CLI утилите

---

## 3. Анализ ценности промптов BMad для Ralph

### 3.1 Промпты с ценными правилами

| Источник BMad | Что ценно | Уже есть в ralph? | Портировать? |
|--------------|-----------|-------------------|-------------|
| create-story step 5 | Принципы sizing задач | Частично (ScanTasks) | Нет |
| dev-story step 5 | Red-green-refactor cycle | Да (execute.md) | Нет |
| dev-story step 8 | Validation gates | Да (gates/) | Нет |
| code-review step 3 | Adversarial review techniques | Да (review.md + agents/) | Нет |
| code-review step 6 | Knowledge extraction protocol | Частично (distill.md) | Рассмотреть |
| correct-course checklist | Impact analysis структура | Нет | Рассмотреть |
| create-epics instructions | Epic decomposition principles | Нет | Для plan mode |
| retrospective | Metrics reporting format | Частично (metrics.go) | Нет |

### 3.2 Суммарный объём промптов к портированию

| Для чего | Строк из BMad | Строк в ralph (после сжатия) |
|----------|--------------|------------------------------|
| plan mode (декомпозиция) | 387 | ~60-80 |
| replan mode (correct-course) | 485 | ~60-80 |
| retro mode (ретроспектива) | 1443 | ~50-80 |
| knowledge extraction | 60 | ~20-30 |
| **Итого** | **2375** | **~190-270** |

Коэффициент сжатия: **~9-12x**. BMad промпты содержат огромное количество formatting, emoji, party mode, и checkpoint протокола, который ralph не использует.

---

## 4. Что НЕ встраивать (с обоснованием)

### 4.1 Pre-implementation workflows (Фазы 1-2, частично 3)

- **product-brief, research, prd, create-ux-design, architecture** — это domain пользователя. Ralph не создает PRD и не проектирует архитектуру. Эти workflows требуют глубокого интерактивного диалога, что противоречит модели ralph (автономное выполнение).
- **Объём:** ~13475 строк инструкций. Портирование не имеет смысла.

### 4.2 Visualization workflows

- **diagrams/** (4 шт) — Excalidraw-специфичны. Ralph не генерирует диаграммы.
- **Объём:** 645 строк.

### 4.3 BMad infrastructure

- **workflow-status, workflow-init** — мета-координация BMad workflow engine. Ralph имеет свою систему состояний (tasks.md + session state).
- **Объём:** 741 строк.

### 4.4 Test Architecture workflows

- **testarch/** (8 шт) — специализированные QA workflows. Ralph не занимается test architecture planning.
- **Объём:** ~9610 строк.

### 4.5 Quick Flow

- **bmad-quick-flow/** — упрощенный трек для малых проектов. Ralph уже является "quick flow" по своей природе.
- **Объём:** 245 строк.

### 4.6 Document Project

- **document-project** — документирование brownfield проекта. Разовая задача, не часть цикла разработки ralph.
- **Объём:** 1626 строк.

**Итого НЕ встраивать: ~26342 строки (~89% всех промптов BMad)**

---

## 5. Итоговая оценка

### 5.1 Что ralph уже покрывает

| BMad Workflow | Ralph эквивалент | Покрытие |
|--------------|-----------------|----------|
| dev-story | `ralph execute` (runner + execute.md) | ~90% |
| code-review | `ralph review` (runner + review.md + 5 agents) | ~95% |
| create-story | ScanTasks + execute prompt context | ~80% |
| sprint-planning | ScanTasks + tasks.md management | ~70% |

### 5.2 Потенциальные новые режимы

| Режим | Промпт (строк) | Go-код (LOC) | Ценность | Приоритет |
|-------|----------------|-------------|----------|-----------|
| `ralph plan` | ~70 | ~250 | Средняя | P2 |
| `ralph replan` | ~70 | ~180 | Средняя | P2 |
| `ralph retro` | ~60 | ~130 | Низкая | P3 |

### 5.3 ROI анализ

**Встраивание в ralph:**
- Промптов к написанию: ~200 строк (из 29600 строк BMad = 0.7%)
- Go-кода к написанию: ~560 LOC
- Результат: 3 новых режима (plan, replan, retro)
- Общая трудоёмкость: ~3-5 стори

**Использование BMad externally:**
- 0 строк кода
- BMad уже работает через Claude Code slash commands
- Пользователь запускает BMad workflow отдельно, ralph получает готовый tasks.md
- Текущая интеграция: ralph читает tasks.md, BMad создает tasks.md

### 5.4 Рекомендация

**Короткий ответ: НЕ встраивать BMad workflows в ralph.**

Обоснование:

1. **89% BMad промптов не нужны ralph** — это pre-implementation workflows, визуализация, QA planning, infrastructure координация.

2. **Оставшиеся 11% уже покрыты** — ralph execute/review покрывают dev-story/code-review на 90-95%.

3. **3 потенциальных новых режима (plan/replan/retro) дают маргинальную ценность:**
   - `plan` — пользователь уже может создать tasks.md вручную или через BMad
   - `replan` — можно просто отредактировать tasks.md
   - `retro` — метрики уже собираются, просмотр через session logs

4. **Ralph и BMad — complementary tools, а не конкуренты:**
   - BMad: планирование, design, story creation (высокоинтерактивные)
   - Ralph: автономное выполнение задач (низкоинтерактивный loop)
   - Граница чёткая: BMad создает артефакты -> Ralph их исполняет

5. **Промпты BMad не переносимы напрямую:**
   - Коэффициент сжатия 9-12x показывает, что BMad промпты оптимизированы под BMad workflow engine (checkpoints, party mode, template-output). Ralph использует другую модель — один промпт на сессию Claude Code.

**Если делать что-то одно:**
Единственная фича с ROI > 1 — это `ralph plan` как lightweight декомпозитор задач из краткого описания в tasks.md. Это ~70 строк промпта + ~250 LOC Go-кода. Но даже это можно заменить ручным созданием tasks.md или использованием BMad create-epics-and-stories externally.
