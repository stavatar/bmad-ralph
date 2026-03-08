# Оптимальный размер задач для LLM-генератора планов

**Дата:** 2026-03-08
**Контекст:** ralph — Go CLI, запускает Claude Code сессии (max_turns=15) для автономной разработки.
Промпт-генератор (LLM) читает PRD + architecture и создаёт список задач в sprint-tasks.md.
Каждая задача выполняется в **изолированной** сессии без памяти о предыдущих сессиях.

---

## 1. Метрики размера задачи

### 1.1 Эмпирические данные из SWE-Bench Pro

Исследование Scale AI на 2025 показывает чёткую обратную зависимость между размером задачи и успехом агента:

| Метрика сложности | Успех агента (среднее) |
|-------------------|----------------------|
| 1-2 файла, 1-40 LOC изменений | ~70% (SWE-Bench Verified) |
| 3-5 файлов, 40-100 LOC | ~40-50% |
| 5+ файлов, 100+ LOC изменений | ~23% (SWE-Bench Pro) |

Среднее в SWE-Bench Pro: 107 LOC, 4.1 файла — успех топ-моделей 23%.
Среднее в SWE-Bench Verified: 41 LOC, 1.5 файла — успех 70%+.

**Вывод:** задача с >4 файлами и >100 LOC изменений — в зоне высокого риска отказа.

### 1.2 Рабочие метрики для промпта

На основе данных SWE-Bench Pro и практики agentic coding 2025-2026:

| Метрика | Зелёная зона (задача выполнима) | Жёлтая зона (риск) | Красная зона (разбить) |
|---------|--------------------------------|-------------------|----------------------|
| Затрагиваемые файлы | 1-3 | 3-5 | 5+ |
| Изменений LOC | до 100 | 100-200 | 200+ |
| Новых функций/типов | 1-3 | 3-5 | 5+ |
| Acceptance criteria | 1-4 | 4-7 | 7+ |
| "Concerns" (областей ответственности) | 1 | 1-2 | 2+ независимых |

**Важно:** Метрики комплементарны — нарушение одной метрики не автоматически красная зона.
Нарушение двух и более — сигнал к декомпозиции.

### 1.3 Правило "2-5 минут"

Согласно практике 2025-2026 (Superpowers plugin framework для Claude Code): задача оптимального размера = та, что человек-разработчик выполнил бы за **2-5 минут** в автономном режиме. Это субъективная метрика, но она интуитивно понятна для промпта.

Для LLM-генератора она лучше переводится как:
- **1-3 файла** с изменениями
- **Одна логическая функция** (авторизация, парсер, форматтер)
- **Тесты включены** в ту же задачу

---

## 2. Правильный уровень декомпозиции: не слишком атомарный, не слишком крупный

### 2.1 Антипаттерн "Слишком мелкие задачи" (Over-decomposed / Micro-tasks)

**Симптомы:**
- Задача реализует одну функцию без тестов
- Задача только пишет тесты без реализации
- Задача только добавляет константы/конфиги без логики
- Задачи B, C, D все зависят от задачи A и делают по 3-4 строки

**Проблема:** Overhead на изоляцию сессии. Каждая сессия = новый Claude процесс, который должен:
1. Прочитать sprint-tasks.md (~500 токенов)
2. Прочитать релевантные файлы (~2000 токенов)
3. Понять контекст проекта

При задаче на 10 строк это ~80% overhead. Кроме того, сессия без тестов нарушает ATDD и усложняет верификацию.

**WRONG примеры:**
```
WRONG: - [ ] Добавить константу DefaultMaxTurns = 15 в config/constants.go
WRONG: - [ ] Написать unit-тест для функции ParseConfig
WRONG: - [ ] Добавить поле MaxTurns в структуру Config
WRONG: - [ ] Добавить валидацию поля MaxTurns
```

**CORRECT:** Всё вышеперечисленное = одна задача:
```
CORRECT: - [ ] Добавить поле MaxTurns в Config с константой DefaultMaxTurns=15,
             валидацией (1-100, error если вне диапазона) и тестами:
             TestConfig_MaxTurns_Valid, TestConfig_MaxTurns_TooLarge,
             TestConfig_MaxTurns_Default
             source: prd.md#FR-5
```

### 2.2 Антипаттерн "Слишком крупная задача" (Bundle / God-task)

**Симптомы:**
- Задача затрагивает несколько независимых concerns
- Описание содержит "и" в смысле "а также другое"
- Задача меняет и production-код, и тесты, и документацию, и миграции
- Задача реализует несколько независимых модулей

**Проблема:** При max_turns=15 сессия исчерпывает лимит итераций до завершения. Агент делает частичную работу, что хуже чем ничего (broken state).

**WRONG примеры:**
```
WRONG: - [ ] Реализовать систему аутентификации: JWT генерация, валидация токена,
             refresh токены, middleware для защищённых роутов, OAuth2 интеграция,
             rate limiting на login endpoint, логирование попыток входа
             source: prd.md#FR-12

WRONG: - [ ] Рефакторить runner.go (убрать дублирование, улучшить error handling)
             и добавить новую функцию SmartMerge для tasks
             source: prd.md#FR-8,FR-9
```

**CORRECT:** Разбить по concerns:
```
CORRECT:
- [ ] Реализовать JWT генерацию и валидацию (jwt.go): GenerateToken(userID, secret),
     ValidateToken(token, secret) с тестами valid/expired/invalid-signature
     source: prd.md#FR-12a
- [ ] Добавить refresh token ротацию (refresh.go): RotateToken, InvalidateToken
     с тестами и хранением в памяти (in-memory store)
     source: prd.md#FR-12b
- [ ] [GATE] Добавить auth middleware (middleware.go): RequireAuth handler wrapper,
     rate limiting 5 req/min/IP с интеграционным тестом
     source: prd.md#FR-12c
```

### 2.3 Правило "Один module с тестами"

**Золотое правило декомпозиции:** одна задача = один Go-файл (или пара файл+тест) с одной логической ответственностью.

Нормально объединять в одну задачу:
- Реализацию и тесты к ней (всегда)
- Несколько тесно связанных функций одного модуля
- Константы + структуру + валидацию одного домена
- Правки в 2-3 файлах если они реализуют одну feature

Нельзя объединять в одну задачу:
- Независимые features ("и JWT, и OAuth2")
- Рефакторинг + новый функционал
- Несколько пакетов с разными concerns
- Любое "и" если оба пункта самодостаточны

---

## 3. Hydra Pattern: задача, которую невозможно завершить за N итераций

### 3.1 Определение

**Hydra Pattern** (в контексте AI coding) — это задача, у которой при каждой попытке исправить одну проблему возникает одна или несколько новых. Агент никогда не может завершить её за фиксированное число итераций (max_turns).

Назван по аналогии с мифологической гидрой: отрубаешь одну голову — вырастают две.

### 3.2 Как возникает Hydra в ralph

ralph использует max_turns=15. Если задача содержит:
1. **Скрытые зависимости:** реализация A требует B, которого ещё нет
2. **Cascading failures:** изменение одного файла ломает другой
3. **Scope creep в задаче:** "улучши систему X" без чётких границ
4. **Circular reasoning:** компилятор требует интерфейс, который требует тип, который требует интерфейс

Тогда каждое исправление агента создаёт новую ошибку. Сессия исчерпывает max_turns в полузавершённом состоянии.

### 3.3 Признаки Hydra-задачи на этапе планирования

Промпт-генератор должен детектировать эти признаки и **разбивать** задачу:

| Признак | Диагностика | Решение |
|---------|-------------|---------|
| Нет зависимого кода | Задача использует типы/функции из другой задачи | Разбить: сначала зависимость |
| "Улучши/рефакторь" без критериев | Нет конкретного acceptance criteria | Добавить конкретные требования или убрать |
| Меняет публичный API | Все потребители тоже нужно обновить | Либо всё в одну задачу, либо 2 задачи (API + consumers) |
| Cross-cutting concern | Затрагивает все пакеты (logging, errors) | Либо foundation-задача первой, либо inlined everywhere |
| Нет deterministic "done" | Нельзя проверить завершённость тестами | Переформулировать с тестируемыми критериями |

### 3.4 Пример Hydra-задачи и как её исправить

**WRONG (Hydra):**
```
- [ ] Рефакторить систему ошибок: заменить string errors на sentinel errors
     во всём проекте, обновить все тесты, добавить errors.Is/As
     source: prd.md#FR-15
```
*Проблема:* Каждый пакет требует изменений, CI падает на несвязанных тестах,
агент застрянет в цикле "исправляю тут — ломается там".

**CORRECT (разбито на foundation + adoption):**
```
- [ ] [GATE] Определить sentinel errors в config/errors.go:
     ErrNoTasks, ErrInvalidConfig, ErrSessionFailed с документацией
     и тестами TestSentinelErrors_Unwrap
     source: prd.md#FR-15a

- [ ] Обновить runner/ для использования sentinel errors:
     заменить fmt.Errorf("runner: ...") на fmt.Errorf("runner: %w", ErrXxx),
     обновить тесты на errors.Is/As проверки
     source: prd.md#FR-15b

- [ ] Обновить config/ и session/ для sentinel errors (аналогично)
     source: prd.md#FR-15c
```

---

## 4. Исследовательская база: ADaPT, HyperAgent, другие системы

### 4.1 ADaPT (Allen AI, NAACL 2024)

**Ключевой принцип:** Декомпозировать задачу **только тогда, когда агент не может её выполнить** — не upfront.

Результаты: +28.3% на ALFWorld, +27% на WebShop, +33% на TextCraft по сравнению с plan-and-execute.

**Применение для ralph plan prompt:**
- Промпт не должен разбивать всё на максимально атомарные задачи заранее
- Разбивать нужно по "complexity ceiling": если задача явно превышает порог — декомпозировать
- Простые FR (1-2 файла) → 1 задача без декомпозиции
- Сложные FR (5+ файлов, multiple concerns) → ADaPT-декомпозиция на 2-4 задачи

### 4.2 HyperAgent (FPT Software AI4Code, 2024)

**Архитектура:** 4 специализированных агента — Planner, Navigator, Code Editor, Executor.

Planner декомпозирует задачу в подзадачи, Navigator находит релевантный код, Editor меняет, Executor проверяет. Успех 33% на SWE-Bench Verified.

**Применение:** Модель разделения ответственности. В ralph plan:
- **Планировщик (LLM):** определяет что делать (декомпозиция)
- **Навигатор (Go-код):** сканирует codebase, собирает контекст
- **Форматировщик (Go-код):** создаёт sprint-tasks.md

### 4.3 Blueprint First, Model Second (arxiv 2025)

**Принцип:** Использовать LLM только для "bounded sub-tasks" в детерминистическом workflow.

Результаты: +10.1 п.п. на tau-bench, -81.8% tool calls.

**Применение для ralph:** LLM отвечает за содержание задач (что делать), Go-код за формат (sprint-tasks.md). Промпт не должен содержать format rules — только decomposition rules.

### 4.4 Kovyrin PRD-Tasklist Process (практика 2025)

**Паттерн:**
1. PRD с детальными требованиями
2. Task List с мульти-уровневым планом + "persistent storage" (файлы, findings)
3. Каждая сессия: spawn clean agent, дать PRD + Task List, выполнить задачу, обновить Task List

**Ключевая цитата:** "Each iteration is isolated — the agent is spawned fresh each time, so you truly wipe the slate clean but feed it the necessary context anew each time."

**Применение:** ralph уже следует этому паттерну. Промпт должен генерировать задачи, самодостаточные для изолированной сессии — с достаточным контекстом внутри описания.

### 4.5 Corpus данных: что происходит с "слишком большими" задачами

Из исследований траекторий SWE-Agent (ASE 2025):
- Агенты тратят 60-70% итераций на "exploration" при нечётких границах задачи
- Задачи с >5 файлами требуют в среднем 2.3x больше итераций
- Наиболее частая причина отказа: "task requires understanding of component not in context"

---

## 5. Конкретные правила для промпта plan.md

### 5.1 Структура секции "Task Granularity" в промпте

```markdown
## Task Granularity Rules

### What is ONE task

A task is a single unit of work that a Claude Code session (max 15 iterations,
no memory of previous sessions) can complete end-to-end:

- One logical concern: authentication, parsing, formatting, validation
- Implementation + tests in the SAME task (never separate them)
- Self-contained: if B uses code from A, A must be a SEPARATE task listed first

### Size ceiling (HARD LIMITS — split if exceeded)

A task MUST be split when it:
- Touches 5+ files across different packages
- Combines REFACTORING with NEW FEATURE (different concerns)
- Has 7+ acceptance criteria
- Uses the word "and" to describe TWO independent features

### Size floor (do NOT over-decompose)

Do NOT create tasks that:
- Implement less than one complete testable behavior
- Are purely "add a constant" or "add a test" without the feature
- Depend on 3+ other tasks to be useful at all
- Mirror exactly what another task already does (duplicate concern)

### Cohesive vs Bundle

COHESIVE (one task): changes that share the same module, struct, or concern
  - Config struct + its fields + Validate() + tests → ONE task
  - Parser + its edge cases + tests → ONE task
  - Middleware + its 3 test scenarios → ONE task

BUNDLE (split required): changes that span independent concerns
  - JWT generation AND OAuth2 integration → TWO tasks
  - Refactor error handling AND add new endpoint → TWO tasks
  - Database migration AND business logic → TWO tasks (migration first)
```

### 5.2 Секция "Session Isolation Requirements" в промпте

```markdown
## Session Isolation Requirements

Each task runs in an ISOLATED Claude Code session with no memory of other tasks.
Therefore each task description MUST include:

1. WHAT to implement: specific files, function names, types
2. WHAT tests to write: at minimum 2-3 scenario names
3. WHAT FR(s) this satisfies: source reference
4. Any prerequisite that must exist (reference earlier task by description)

If a task requires context that is not in the codebase yet (new types, interfaces,
constants), either:
- Include creating them in this task, OR
- List the preceding task as a dependency

A task description that says "add it to the system" without specifying
WHICH file and WHICH function is NOT self-contained.
```

### 5.3 Секция "Anti-patterns" (WRONG/CORRECT примеры)

```markdown
## Common Mistakes — WRONG vs CORRECT

### WRONG: Splitting tests from implementation
- [ ] Add UserService.Create() method in user_service.go
- [ ] Write tests for UserService.Create()
CORRECT: Both in ONE task — "Implement UserService.Create() with tests for
valid input, duplicate email (ErrEmailExists), and invalid data"

### WRONG: One God-task combining independent concerns
- [ ] Implement full authentication system: JWT, refresh tokens, OAuth2,
     rate limiting, audit logging
CORRECT: Split into 3-4 tasks by concern with [GATE] before OAuth2

### WRONG: Trivial micro-task
- [ ] Add DefaultMaxTurns = 15 constant to config/constants.go
CORRECT: Merge into the task that needs this constant

### WRONG: Vague scope (Hydra risk)
- [ ] Refactor error handling across the codebase
CORRECT: "Update runner/ errors to use sentinel errors (ErrNoTasks,
ErrSessionFailed) with fmt.Errorf wrapping, update runner_test.go
to use errors.Is assertions"

### WRONG: Missing dependency order
- [ ] Implement SessionManager using the SessionPool interface
- [ ] Define SessionPool interface in session/pool.go  ← should be FIRST
CORRECT: Define interface first (task 1), implement (task 2)

### WRONG: Task that requires browsing/external access
- [ ] Test the OAuth2 flow manually in browser
CORRECT: Skip — manual verification tasks cannot be automated
```

### 5.4 Эвристика granularity для промпта (if-then rules)

```markdown
## Granularity Decision Tree

1 FR → 1-2 source files → 1 task
1 FR → 3-5 related files (same domain) → 1-2 tasks
1 FR → 5+ files OR 2+ independent concerns → 2-4 tasks (split by concern)

Multiple FRs → same file/module → 1 task (group by module, not by FR)
Multiple FRs → sequential dependency (A enables B) → separate tasks, A first

Never: collapse 5+ independent FRs into 1 "implement everything" task
Never: explode 1 FR with 3 validators into 3 "add validator X" tasks
```

---

## 6. Детектирование "Hydra Pattern" на этапе планирования

Промпт должен содержать явные проверки. Добавить в секцию Classification:

```markdown
## Hydra Detection — Skip or Restructure

A requirement creates a Hydra task if it:

1. SCOPE: Says "across the codebase", "all packages", "everywhere"
   → Split into: foundation layer (first, with [GATE]) + adoption tasks per package

2. DEPENDENCY LOOP: Requires type T from package A, but A doesn't exist yet
   → Create a SETUP task for package A first (mark [SETUP])

3. VAGUE "DONE" CRITERION: "improve", "clean up", "make better"
   → Either reformulate with testable criteria OR skip entirely
   → A task is valid only if it can be verified by running tests

4. API CHANGE WITH CONSUMERS: Changes a public interface used by 3+ places
   → Task 1: new interface definition + one reference implementation
   → Task 2: update all consumers (separate concern)

5. COMPOUND REFACTOR+FEATURE: "Fix X and also add Y"
   → Always split: refactor first (standalone), feature second
```

---

## 7. Резюме: правила для включения в промпт plan.md

### Минимальный набор правил (приоритет 1)

1. **Одна задача = одна logical concern** — файлы одного модуля, одна ответственность
2. **Тесты включены** в ту же задачу что и реализация — никогда отдельно
3. **Self-contained**: задача должна быть выполнима без контекста из других задач
4. **Порядок по зависимостям**: если B использует код A, A идёт первой
5. **Конкретность**: имена файлов, функций, минимум 2 тест-сценария в описании

### Правила разбиения (приоритет 2)

6. **Разбивать при 5+ файлах** из разных пакетов
7. **Разбивать при "и"** если оба пункта независимы
8. **Разбивать рефакторинг от new feature** — всегда отдельные задачи
9. **SETUP-задача первой** если нет базовых типов/интерфейсов

### Правила против микро-задач (приоритет 3)

10. **Не создавать задачи только с константами** без использующей их логики
11. **Не создавать задачи только с тестами** без реализации
12. **Группировать по модулю** а не по FR если несколько FR касаются одного файла

### Защита от Hydra (приоритет 4)

13. **"across the codebase"** → foundation-задача + N adoption-задач
14. **Vague criteria** → reformulate OR skip
15. **API change** → сначала новый API, потом consumers

---

## 8. Источники

### Исследования
- [ADaPT: As-Needed Decomposition and Planning with Language Models](https://arxiv.org/abs/2311.05772) — Allen AI, NAACL 2024. +28% vs plan-and-execute
- [SWE-Bench Pro: Can AI Agents Solve Long-Horizon Software Engineering Tasks?](https://arxiv.org/html/2509.16941) — Scale AI 2025. Данные о корреляции размера задачи и успеха
- [Blueprint First, Model Second](https://arxiv.org/abs/2508.02721) — детерминистические workflow с LLM как bounded executor
- [HyperAgent: Generalist Software Engineering Agents](https://github.com/FSoft-AI4Code/HyperAgent) — FSoft 2024. Planner-Navigator-Editor-Executor
- [Understanding SE Agent Trajectories](https://software-lab.org/publications/ase2025_trajectories.pdf) — ASE 2025

### Инструменты и практика
- [Align, Plan, Ship: PRD-Driven AI Agents](https://kovyrin.net/2025/06/20/prd-tasklist-process/) — Kovyrin паттерн изолированных сессий
- [Claude Code Best Practices](https://code.claude.com/docs/en/best-practices) — Anthropic официальные рекомендации
- [LLM Agent Task Decomposition Strategies](https://apxml.com/courses/agentic-llm-memory-architectures/chapter-4-complex-planning-tool-integration/task-decomposition-strategies) — granularity trade-offs
- [Breaking Down Tasks for AI Agents](https://mbrenndoerfer.com/writing/breaking-down-tasks-task-decomposition-ai-agents) — Brenndoerfer 2025
- [Fixing Infinite Loop: When AI Agent Refuses to Stop Coding](https://techbytes.app/posts/fixing-infinite-loop-ai-agent-refuses-stop-coding/) — loop guardrails

### Внутренние исследования проекта
- [variant-d-self-sufficient-decomposition.md](variant-d-self-sufficient-decomposition.md) — ADaPT-паттерн для ralph plan
- [variant-d-prompt-architecture.md](variant-d-prompt-architecture.md) — LLM vs Go разделение ответственности
- [variant-d-task-format.md](variant-d-task-format.md) — форматы задач, YAML vs markdown
