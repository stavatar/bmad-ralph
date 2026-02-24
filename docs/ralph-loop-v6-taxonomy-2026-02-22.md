# Ralph Loop Taxonomy v6: Полная классификация паттернов автономной итерации

> **Дата:** 2026-02-22
> **Версия:** 6.0 (taxonomy report)
> **Аудитория:** Разработчик, выбирающий тип Ralph Loop для своего проекта
> **Контекст:** Ecosystem survey 2025-2026, 35 sources, 20+ реализаций
> **Источники:** evidence-table.md (T01-T35), v5 evidence-table.md (S01-S40)
> **Цитирование:** inline [Автор/Источник]
> **Языковая политика:** Русский текст, технические термины на английском

---

## 1. Executive Summary

### 1.1. Шесть типов Ralph Loop

За 2025-2026 годы паттерн Ralph Loop (автономная итерация AI-агента с fresh context)
эволюционировал от простого bash while-loop [Huntley, T01] до сложных multi-agent систем
с параллельными worktrees и Judge Agents [Carlini, T11; Vercel, T16]. Анализ 35 источников
и 20+ реализаций выявил **шесть устойчивых типов**, различающихся по session model,
agent count, state mechanism и verification strategy.

| # | Тип | Ключевая идея | Session model | State mechanism | Сложность |
|---|-----|---------------|---------------|-----------------|-----------|
| 1 | **Pure Bash Loop** | `while true; do claude -p "..."; done` | Multi-session | Нет (prompt only) | Минимальная |
| 2 | **Plugin/Hook-Based** | Stop hook внутри одной сессии | Single session | Conversation context | Низкая |
| 3 | **PRD-Driven Sequential** | Spec-файл как state + sequential execution | Multi-session | prd.json / spec.md | Средняя |
| 4 | **State-Machine Multi-Phase** | Все фазы как атомарные задачи, dependency-ordered | Multi-session | JSON state file (DAG) | Средне-высокая |
| 5 | **Multi-Model Review Loop** | Отдельная модель-reviewer, coding+reviewing в loop | Multi-session | Review state files | Средне-высокая |
| 6 | **Multi-Agent Parallel** | Несколько агентов, worktrees, lock-based claiming | Multi-session parallel | Shared task list + locks | Высокая |

### 1.2. Типы НЕ взаимоисключающие

**Большинство production-реализаций комбинируют типы:**
- ralphex (umputun) = Type 4 + Type 5 (state-machine + review-агенты) [T12]
- Carlini C Compiler = Type 4 + Type 6 (state-machine + 16 parallel agents) [T11]
- hamelsmu = Type 2 + Type 5 (stop-hook + Codex review) [T14]
- choo-choo-ralph = Type 3 + Type 4 (PRD-driven + 5-phase pipeline) [T09]
- Tessmann hybrid = Type 4 + Type 6 (Agent Teams creative + Ralph mechanical) [T20]

### 1.3. Рекомендация v5 = Тип 4

Предыдущий отчёт (v5: "Everything is a Ralph Loop") рекомендовал **State-Machine Multi-Phase**
архитектуру: sprint-loop.sh + sprint-state.json с flat list задач, покрывающих ВСЕ фазы
(init, execute, review, finish). Это Тип 4 в нашей таксономии [T13, T33].

**Почему именно Тип 4:**

1. **Brownfield-совместимость.** 1451 тест, 12 спринтов, NestJS + React monorepo -- Тип 1
   (чистый bash) не справляется с inter-module dependencies и multi-phase review [T13, S36].
2. **Atomic decomposition.** ВСЕ фазы (не только coding) декомпозированы в атомарные задачи.
   Review -- не отдельная система, а набор задач в том же flat list [ralphex T12, Carlini T11].
3. **Стоимость.** $0.77-1.50 за итерацию vs $1.50-2.50 с compaction в skill-orchestrator
   [Stringer, T21].
4. **Простота.** ~110 строк (bash + prompt + state) vs ~641 строка orchestration skills [T13].
5. **Knowledge extraction per-iteration.** `discoveries.log` (append-only) + finish-задачи [T33].
6. **Circuit breaker + human pause.** `HUMAN:*` stops loop. 3 no-progress iterations = stop.

### 1.4. Каждый тип имеет "sweet spot"

Нет универсально лучшего типа. Каждый оптимален для определённого сочетания: проект
(greenfield/brownfield), размер (solo/team), бюджет ($10/$1000), верификация
(lint-only/multi-reviewer). Для быстрого выбора: см. **Секцию 9 (Decision Matrix)**.
Для понимания каждого типа: секции 3-8 содержат определение, реализации, архитектурную
диаграмму, плюсы/минусы и рекомендации по применению.

---

## 2. Taxonomy: Оси классификации

### 2.1. Пять осей

Классификация построена на пяти ортогональных осях, каждая из которых описывает
один аспект архитектуры Ralph Loop:

| Ось | Варианты | Что определяет |
|-----|----------|----------------|
| **Session model** | Single session / Multi-session fresh context | Accumulates context vs starts clean |
| **Agent count** | Single agent / Multi-agent sequential / Multi-agent parallel | Сколько агентов работают одновременно |
| **State mechanism** | None / Git-only / PRD JSON / State file + dependencies / Lock files | Как передаётся прогресс между итерациями |
| **Verification** | Exit code / passes:boolean / Judge Agent / SHIP/REVISE / Human gate | Как определяется завершение задачи |
| **Scope** | Single task / PRD items / Sprint (multi-phase) / Full SDLC | Масштаб автоматизации |

### 2.2. Визуальный спектр осей

```
Ось 1: Session Model       Single ................. Multi ............... Parallel
                            Type 2               Types 1,3,4,5           Type 6

Ось 2: Agent Count          1 agent ............... Agent+Reviewer ........ N agents
                            Types 1,2,3,4             Type 5               Type 6

Ось 3: State Mechanism      None ....... Conversation ...... File ......... JSON DAG
                            Type 1       Type 2           Types 3,5      Types 4,6

Ось 4: Verification         None ......... test+lint ....... Judge Agent ... Multi-reviewer
                            Type 1 (basic) Types 2,3,4        Type 5         Type 6

Ось 5: Scope                Task ......... PRD items ....... Sprint ........ Full SDLC
                            Types 1,2       Type 3           Type 4         Types 5,6
```

### 2.3. Матрица типов по осям

| Тип | Session | Agents | State | Verification | Scope |
|-----|---------|--------|-------|--------------|-------|
| 1. Pure Bash | Multi-session | Single | None / Git-only | Exit code | Single task |
| 2. Plugin/Hook | **Single session** | Single | In-memory | Exit code / hook | Single task |
| 3. PRD-Driven | Multi-session | Single | PRD JSON | passes:boolean | PRD items |
| 4. State-Machine | Multi-session | Single/Sequential | State file + deps | passes:boolean + gates | Sprint (multi-phase) |
| 5. Multi-Model Review | Multi-session | Multi sequential | State files | Judge Agent / SHIP/REVISE | PRD items + review |
| 6. Multi-Agent Parallel | Multi-session | Multi parallel | Lock files / shared state | Distributed consensus | Full SDLC |

### 2.4. Обзорная таблица характеристик

| Характеристика | Type 1 | Type 2 | Type 3 | Type 4 | Type 5 | Type 6 |
|----------------|--------|--------|--------|--------|--------|--------|
| Строк кода | ~5-10 | ~0-50 (plugin) | ~30-80 | ~80-150 | ~100-200 | ~200-500+ |
| Setup effort | Minimal | Minimal | Low | Medium | Medium | High |
| Fresh context | Да | **Нет** | Да | Да | Да | Да |
| Resume after crash | Нет (Git only) | Нет | Partial (JSON) | **Full** (JSON + deps) | Partial (state files) | **Full** (locks + state) |
| Review quality | Нет | Нет | test+lint | Multi-task same-model | **Separate model** | Multi-agent |
| Cost per iteration | $0.50-1.50 | $0.50-3.00+ | $0.50-1.50 | $0.80-1.50 | $1.50-3.00 | $0.80-1.50 x N |
| Max project size | 1K LOC | 10K LOC | 50K LOC | 100K+ LOC | 100K+ LOC | **1M+ LOC** |
| Vendor lock-in | None | Claude Code | Low | Low | Multi-vendor | Multi-vendor |

### 2.5. Эволюционная связь

Типы образуют эволюционную цепочку, где каждый следующий добавляет
измерение сложности поверх предыдущего:

```
Type 1 (Pure Bash)
  |
  +---> Type 2 (Plugin)        -- same loop, single session (ТУПИК)
  |
  +---> Type 3 (PRD-Driven)    -- добавляет structured state
          |
          +---> Type 4 (State-Machine) -- добавляет dependencies + phases
          |       |             \
          |       +---> Type 4+5 (+ Multi-Model Review)
          |
          +---> Type 6 (Multi-Agent)   -- добавляет параллельность
                        \
                         +---> Type 4+6 (+ Parallel)
```

Эта эволюция НЕ означает, что более сложный тип всегда лучше.
Каждый тип оптимален для своего класса задач (см. Секцию 9).
Большинство проектов начинают с Type 1 или Type 3, мигрируют на Type 4
при росте complexity. Type 2 -- тупиковая ветка, community recommends against [T03].

---

## 3. Тип 1: Pure Bash Loop

### 3.1. Определение

**Pure Bash Loop** -- оригинальный паттерн Ralph Loop: bash while-цикл,
запускающий AI-агент с fresh context на каждой итерации. Никакого structured state --
прогресс определяется по exit code или наличию определённого файла.
Оригинальный паттерн Geoffrey Huntley (июль 2025) [T01].

> *"Compaction is the devil. Fresh context every iteration."* [Huntley, T01]

### 3.2. Ключевые реализации

| Реализация | Автор | URL | Особенности |
|------------|-------|-----|-------------|
| **ghuntley/ralph** | Geoffrey Huntley | ghuntley.com/ralph | Оригинальная реализация, минимальный overhead |
| **KLIEBHAN/ralph-loop** | KLIEBHAN | github.com/KLIEBHAN/ralph-loop | External loop для Claude и Codex, multi-provider |

### 3.3. Архитектура

```
BASH SHELL                          CLAUDE CODE SESSION
+--------------------------+        +---------------------------+
| while true; do           | -----> | - Reads CLAUDE.md         |
|   claude -p "PROMPT"     |        | - Executes prompt         |
|   check exit code        | <----- | - Commits + exits         |
|   sleep 2                |        +---------------------------+
| done                     |
+--------------------------+
  State: NONE (prompt only, or git diff as implicit state)
  Exit: exit code 0 = done, 1 = retry, or Ctrl+C / iteration limit
```

### 3.4. Типичная реализация (~5-10 строк)

```bash
#!/usr/bin/env bash
while true; do
  claude -p "Continue working on the task. Read CLAUDE.md first." \
    --dangerously-skip-permissions \
    --max-turns 20
  EXIT_CODE=$?
  if [[ $EXIT_CODE -eq 0 ]]; then
    echo "Task complete"
    break
  fi
  sleep 2
done
```

Альтернативный вариант (pipe prompt):

```bash
#!/usr/bin/env bash
while true; do
  cat PROMPT.md | claude --dangerously-skip-permissions --max-turns 20
  sleep 2
done
```

### 3.5. Плюсы и минусы

| Плюсы | Минусы |
|-------|--------|
| Минимальный код (~5-10 строк) | Нет structured state -- непонятно, что завершено |
| Fresh context каждую итерацию [AI Hero, T03] | Exit code -- единственный сигнал завершения |
| Vendor-agnostic (любой AI tool) | Нет dependencies между задачами |
| Zero learning curve, zero setup | Нет review phase |
| Ctrl+C safe (git = state) | Нет circuit breaker (бесконечный loop) |
| Debugging: git log = progress | Не подходит для multi-story sprints |
| Идеальная для обучения паттерну | Prompt bloat = единственный way to convey state |

### 3.6. Когда использовать

**Best for:** Solo greenfield < 1K LOC. One-off refactor. "Хочу попробовать Ralph за 5 минут."
Прототипирование, когда overhead orchestration не оправдан.
Первый опыт с Ralph Loop -- начните с Type 1, усложняйте по необходимости.

**Worst for:** Brownfield с 800+ тестами. Multi-story спринты. Projects с inter-module dependencies.

> *"A for loop is a legitimate orchestration strategy"* [Parsons, T22] --
> для простых задач Type 1 **достаточен**. Не усложняйте без причины.
> Но только если задачи атомарные и независимые.

---

## 4. Тип 2: Plugin/Hook-Based Single Session

### 4.1. Определение

**Plugin/Hook-Based** реализации работают **внутри одной сессии** Claude Code,
используя Stop hook или exit detection для автоматического перезапуска агента
без потери контекстного окна. Принципиальное отличие от Type 1 --
контекст **накапливается** вместо сброса. Continuity и degradation одновременно.

### 4.2. Ключевые реализации

| Реализация | Автор | Особенности |
|------------|-------|-------------|
| **ralph-wiggum plugin** | Boris Cherny / Anthropic | Official, Stop hook, `/ralph-loop` [T02] |
| **ralph-claude-code** | Frank Bria | Exit detection, circuit breaker, rate limiting, 484 теста [T04] |
| **ralph-wiggum-cursor** | Agrim Singh | Token tracking, context rotation при 80K tokens [T25] |
| **opencode-ralph-wiggum** | Th0rgal | Real-time status, mid-loop injection, struggle detection [T26] |

### 4.3. Архитектура

```
SINGLE CLAUDE CODE SESSION
+-------------------------------------------------------------+
| Iter 1: Execute task                                         |
|    |-> [Stop Hook] -> Iter 2 (context: 1+2)                 |
|         |-> [Stop Hook] -> Iter 3 (context: 1+2+3)          |
|              |-> ... context grows linearly ...               |
|                   |-> Iter N (context: ALL) --> DEGRADATION  |
+-------------------------------------------------------------+

Альтернативная визуализация:

+----------------------------------------------+
|           Single Claude Code Session          |
|                                               |
|  +----------+    +----------+    +----------+ |
|  | Iter 1   |--->| Iter 2   |--->| Iter 3   | |
|  | (40% ctx)|    | (60% ctx)|    | (80% ctx)| |
|  +----------+    +----------+    +----------+ |
|       ^                               |       |
|       |      Stop Hook / Plugin       |       |
|       +-------------------------------+       |
|                                               |
|  Context: ACCUMULATES (compaction risk!)      |
+----------------------------------------------+
```

### 4.4. Проблема: Context Degradation

AI Hero [T03] провёл детальный анализ производительности агента в зависимости от
заполненности контекстного окна:

```
Context usage:   0%----[OPTIMAL]----40%----[DEGRADED]----70%----[DUMB ZONE]----100%
Quality:         *****              ****                ***              **
```

> *"Plugin fills context window, bash loop preserves quality.
> Performance zones: first 40% = optimal."* [AI Hero, T03]

При работе в single session контекст неизбежно растёт (~5-15K tokens/iteration).
После ~40% начинается деградация reasoning quality. После 10+ итераций -- "dumb zone".
Compaction (сжатие контекста) -- не решение, а mitigation: сжатые факты теряются [Mickel, S10].

> *"Compaction is the devil."* [Huntley, T01]

**Ключевые проблемы:**
1. **Context accumulation:** ~5-15K tokens/iteration, линейный рост
2. **Compaction degradation:** lossy operation, теряются нюансы [Mickel, S10]
3. **No resume:** crash = потеря всего контекста
4. **Cost grows linearly:** $0.50 в начале, $3+ в конце сессии

### 4.5. Плюсы и минусы

| Плюсы | Минусы |
|-------|--------|
| Простая установка (npm install / plugin) | Context degradation после ~40% / 5+ итераций [AI Hero, T03] |
| Не нужен bash/terminal знание | Compaction lossy -- теряются детали [Mickel, S10] |
| Работает на Windows без WSL | Нет structured state между сессиями |
| Видим весь контекст в одном чате | Дороже: оплата за compacted context, cost grows linearly |
| Context continuity между итерациями | Vendor lock-in (Claude Code / Cursor specific) |
| frankbria: circuit breaker + rate limit [T04] | Нет resume after crash |
| Lowest barrier to entry | Community actively discourages [T03] |

### 4.6. Когда использовать

**Best for:** Quick debugging (3-5 iterations). Exploratory coding. Newcomers.
**Короткие задачи** (1-3 итерации), где контекст не успевает деградировать.
Когда **bash недоступен** (Windows без WSL, restricted environments).

**Worst for:** Sprints из 5+ stories. Tasks > 10 итераций. Brownfield. Production.

### 4.7. Почему community рекомендует против

Сообщество сходится на том, что Plugin/Hook подход **масштабируется плохо**:

> *"Code is cheap, rerunning loops is cheaper than rebasing."* [Horthy/HumanLayer, S02]

Fresh context per iteration (Type 1, 3, 4) обеспечивает стабильное качество reasoning
независимо от количества итераций. Single session неизбежно деградирует.
Для задач длиннее 3-5 итераций Type 1 или Type 3 предпочтительнее.

**Type 2 -- тупиковая ветка:** нет upgrade path к Types 3-6. При росте проекта
нужно переписывать на Type 3/4 с нуля.

---

## 5. Тип 3: PRD-Driven Sequential

### 5.1. Определение

**PRD-Driven Sequential** реализации используют structured document (PRD, spec, task list)
в формате JSON или Markdown как state file. Агент последовательно берёт следующий
незавершённый item из PRD и выполняет его с fresh context. Прогресс отслеживается
через `passes: boolean` или аналогичный маркер в файле.
**Самый популярный тип** (5+ реализаций, тысячи stars) [T05, T06, T07, T08, T28].

### 5.2. Ключевые реализации

| Реализация | Автор | Особенности |
|------------|-------|-------------|
| **snarktank/ralph** | snarktank | prd.json + circuit breaker + progress.txt (append-only journal) [T05] |
| **ralph-playbook** | Clayton Farr | 3 фазы (Requirements/Planning/Building), 5 Enhancements, AGENTS.md, JTBD→SLC, LLM-as-Judge [T07] |
| **fstandhartinger/ralph-wiggum** | Florian Standhartinger | SpecKit + Ralph = Autonomous Stack [T08, T29] |
| **choo-choo-ralph** | MJ Meyer | Beads-powered, 5-phase (Plan/Spec/Pour/Ralph/Harvest) [T09] |
| **iannuttall/ralph** | Ian Nuttall | Minimal, multi-agent support (Claude, Codex, Droid, OpenCode) [T06] |
| **nitodeco/ralph** | nitodeco | CLI, PRD-driven, sequential task execution [T28] |

### 5.3. Архитектура

```
BASH LOOP                          CLAUDE CODE SESSION
+----------------------------+     +---------------------------+
| while incomplete(prd.json) | --> | 1. Read prd.json          |
|   claude -p "do next task" |     | 2. Find passes==false     |
|   sleep 2                  | <-- | 3. Execute, test, lint    |
| done                       |     | 4. Set passes=true, exit  |
+----------------------------+     +---------------------------+
                                          |
                                          v
                                   prd.json:
                                   { tasks: [
                                     { name: "Auth", passes: true },
                                     { name: "Dashboard", passes: false }
                                   ]}

                                   progress.txt (append-only journal)
```

### 5.4. Паттерн prd.json (snarktank)

```json
{
  "name": "My Feature",
  "tasks": [
    { "name": "Setup project", "passes": true, "description": "..." },
    { "name": "Implement JWT auth", "passes": false, "description": "..." },
    { "name": "Add role-based access", "passes": false, "description": "..." }
  ],
  "circuit_breaker": { "max_iterations": 50, "max_no_progress": 3 }
}
```

Агент на каждой итерации:
1. Читает prd.json
2. Находит первый item с `passes: false`
3. Выполняет задачу
4. Если успех -- устанавливает `passes: true`
5. Если неудача -- оставляет `passes: false`, добавляет notes
6. Loop повторяет с fresh context

### 5.5. Вариации

- **snarktank [T05]:** prd.json + `progress.txt` (append-only codebase patterns memory). Самая зрелая реализация.
- **ClaytonFarr [T07]:** Самая проработанная Type 3 реализация. 3 фазы (Define Requirements → Planning → Building), 4 принципа ("Context is Everything", "Backpressure > Direction", "Let Ralph Ralph", "Move Outside the Loop"), 5 Enhancements (Acceptance-Driven Backpressure, LLM-as-Judge, Work Branches, User Interview, JTBD→SLC). С расширениями фактически выходит за рамки Type 3 → мост к Type 4 и Type 5. Подробный анализ — секция 5.6.
- **fstandhartinger [T08]:** SpecKit генерирует спецификацию, Ralph выполняет. *"SpecKit generates what Ralph executes"* [T29]. Философия: "SpecKit + Ralph = Autonomous Stack".
- **choo-choo-ralph [T09]:** 5-phase (Plan/Spec/Pour/Ralph/Harvest), compounding knowledge -- каждая фаза обогащает контекст для следующей. Ближе к Type 4.

### 5.6. Deep Dive: Clayton Farr Ralph Playbook

Среди всех реализаций Type 3 работа Clayton Farr [T07] заслуживает отдельного глубокого
разбора по нескольким причинам. Во-первых, это **не просто ещё один скрипт с prd.json** —
это полноценная методология разработки с AI-агентом, охватывающая весь жизненный цикл
от сбора требований до релиза. Во-вторых, Farr — единственный автор, который
систематизировал не только инструменты (файлы, промпты, loop.sh), но и **принципы**
работы с LLM-агентами: как управлять контекстным окном, что такое backpressure,
почему план должен быть одноразовым. В-третьих, его 5 Enhancements превращают базовый
Type 3 playbook в нечто, что функционально приближается к Type 4 и Type 5, сохраняя
при этом простоту файловой архитектуры — без JSON state machine и без отдельного
orchestration layer [T07].

По сути, Farr Playbook — это **учебник по autonomous AI development**, который можно
использовать как самостоятельный фреймворк или как набор идей для усиления любой
другой реализации.

#### 5.6.1. Архитектура и файлы

```
project-root/
├── loop.sh                    # Ralph loop (plan/build/plan-work modes)
├── PROMPT_build.md            # Build mode: 10-step lifecycle
├── PROMPT_plan.md             # Plan mode: gap analysis → task list
├── PROMPT_plan_work.md        # Scoped plan for feature branches [Enh.4]
├── PROMPT_plan_slc.md         # SLC release planning [Enh.5]
├── AGENTS.md                  # Operational guide (~60 строк, "heart of the loop")
├── IMPLEMENTATION_PLAN.md     # Prioritized task list (disposable!)
├── AUDIENCE_JTBD.md           # Jobs to Be Done [Enh.5]
├── specs/                     # Requirement specs per topic
│   ├── [topic-a].md
│   └── [topic-b].md
└── src/lib/
    ├── llm-review.ts          # LLM-as-Judge fixture [Enh.3]
    └── llm-review.test.ts     # Reference examples для Ralph
```

**Описание каждого файла:**

- **`loop.sh`** — центральный bash-скрипт, запускающий Ralph Loop. Поддерживает **три
  режима запуска**:
  - `./loop.sh` (без аргументов) — **Build mode**. Агент читает IMPLEMENTATION_PLAN.md,
    выбирает следующую задачу, реализует, тестирует, коммитит. Это основной рабочий режим.
  - `./loop.sh plan` — **Plan mode**. Агент анализирует разрыв между спецификациями
    (specs/*.md) и текущим кодом (src/), генерирует или обновляет IMPLEMENTATION_PLAN.md.
    Используется перед началом работы или когда план устарел.
  - `./loop.sh plan-work "описание"` — **Scoped Plan mode** (Enhancement 4). Создаёт план
    только для конкретной фичи, отфильтровывая нерелевантные задачи ещё на этапе
    планирования, а не на этапе выбора задачи. Используется в feature-ветках.
  Каждый режим передаёт соответствующий PROMPT-файл как системный промпт для Claude.

- **`PROMPT_build.md`** — промпт для Build mode. Содержит 10-шаговый lifecycle (orient →
  select → implement → validate → commit), 999-Series Guardrails (приоритизированные
  правила поведения агента) и инструкции по работе с subagents. Это **самый длинный и
  детальный промпт** — он определяет поведение агента при каждой итерации цикла.

- **`PROMPT_plan.md`** — промпт для Plan mode. Содержит инструкции по gap analysis:
  агент сравнивает спецификации с текущим состоянием кода и генерирует список задач.
  Отличается от PROMPT_build.md тем, что агент **не пишет код** — только анализирует и
  планирует. Это разделение критически важно: планирование требует обзорного контекста
  (много файлов одновременно), а реализация — глубокого погружения в конкретный модуль.

- **`AGENTS.md`** — *"сердце цикла"*, как его называет Farr. Несмотря на маленький размер
  (~60 строк), это **самый критичный файл** для стабильности loop. В нём хранится:
  операционная информация (какие команды запускать для тестов, как устроен проект,
  какие соглашения приняты), а также **learnings** — знания, извлечённые агентом за
  предыдущие итерации (например: "не использовать relative imports в src/lib/",
  "запускать typecheck перед commit"). Каждая итерация Build mode обновляет AGENTS.md
  новыми operational learnings (шаг 8 из 10). Почему ~60 строк? Потому что AGENTS.md
  читается на **каждой** итерации и съедает контекстное окно. Чем он короче, тем больше
  места для кода и спецификаций. При этом содержимое должно быть строго operational —
  никаких объяснений "как работает система", только "что делать и чего не делать".

- **`IMPLEMENTATION_PLAN.md`** — приоритизированный список задач. Ключевая характеристика:
  он **одноразовый** (disposable). Если план устарел, рассинхронизировался с кодом или
  стал слишком длинным — его не нужно "чинить". Достаточно запустить `./loop.sh plan`,
  и агент сгенерирует новый план с нуля, проведя свежий gap analysis. Пересоздание
  стоит **одну итерацию Planning** (1-3 минуты), что ничтожно мало по сравнению с
  попытками агента работать по устаревшему плану. Это прямое следствие принципа
  "Let Ralph Ralph" (см. 5.6.3).

- **`specs/`** — директория со спецификациями требований. Каждый файл описывает одну
  **тему** (topic), привязанную к конкретному JTBD (Job to Be Done). Один файл = одна
  тема = один чётко определённый scope. Примеры: `specs/color-extraction.md`,
  `specs/user-authentication.md`, `specs/export-pipeline.md`. Файлы пишутся
  на Phase 1 (Define Requirements) в разговоре с LLM и содержат: описание задачи,
  acceptance criteria, ограничения, примеры использования. Разделение на отдельные файлы
  позволяет subagents читать **только релевантные** спецификации, экономя контекст.

- **`src/lib/llm-review.ts`** — тестовая фикстура для Enhancement 3 (LLM-as-Judge).
  Экспортирует функцию `createReview()`, которая отправляет артефакт (текст или скриншот)
  на оценку второму LLM и возвращает бинарный результат (pass/fail). Размещён как
  обычный модуль в `src/lib/`, а не как отдельный инструмент — это позволяет Ralph
  использовать его в тестах через стандартный тестовый фреймворк (Vitest/Jest).

- **`src/lib/llm-review.test.ts`** — **не просто тесты**, а **reference examples** для
  Ralph. Содержит конкретные примеры вызова `createReview()` с разными criteria и
  артефактами. Агент читает этот файл чтобы понять API и паттерн использования, после
  чего применяет аналогичные вызовы для субъективной валидации в своих задачах.

#### 5.6.2. Три фазы

| Фаза | Режим loop.sh | Что происходит | Выход |
|------|---------------|----------------|-------|
| **Define Requirements** | Conversation (вне loop) | JTBD → topics → specs/*.md | specs/*.md |
| **Planning** | `./loop.sh plan` | Gap analysis: specs vs src → IMPLEMENTATION_PLAN.md | IMPLEMENTATION_PLAN.md |
| **Building** | `./loop.sh` | 10-step lifecycle: orient→select→implement→test→commit | Код + обновлённый план |

**Phase 1: Define Requirements**

Эта фаза проходит **вне Ralph Loop** — в обычном разговоре с Claude Code. Разработчик
описывает, что хочет построить, используя концепцию JTBD (Jobs to Be Done): не "какие
фичи нужны", а "какие задачи пользователь хочет решить". LLM помогает декомпозировать
JTBD на отдельные **topics** — каждый topic = одна спецификация в specs/. Декомпозиция
использует Topic Scope Test (см. 5.6.5): если topic нельзя описать одним предложением
без "и" — его нужно разбить.

Результат Phase 1 — набор файлов `specs/*.md`, каждый из которых содержит:
требования (что делать), acceptance criteria (как проверить), ограничения (чего не делать),
примеры использования. Эти файлы становятся **upstream backpressure** — детерминированным
источником истины, формирующим поведение агента на всех последующих фазах.

**Phase 2: Planning**

Запускается командой `./loop.sh plan`. Агент получает PROMPT_plan.md как системный
промпт и выполняет **gap analysis** — сравнивает спецификации (`specs/*.md`) с текущим
состоянием кодовой базы (`src/`). На основе разрыва между "что должно быть" и "что есть"
агент формирует IMPLEMENTATION_PLAN.md — приоритизированный список задач.

Gap analysis — ключевая инновация Farr по сравнению с другими Type 3 реализациями.
В snarktank [T05] план (prd.json) пишется человеком вручную. У Farr планирование
**делегировано агенту**, но ограничено промптом: агент может только планировать, не кодить.
Это позволяет LLM потратить всё контекстное окно на обзорный анализ (прочитать много
файлов одновременно), а не разрываться между пониманием кода и его изменением.

Farr отмечает, что Phase 2 может породить **до 500 параллельных subagents** [T07] для
чтения кодовой базы. Каждый subagent читает свою группу файлов и возвращает summary
главному агенту. Это решает проблему контекстного окна: вместо того чтобы загружать
все файлы в один контекст (200K токенов быстро заканчиваются), агент делегирует чтение
subagents и собирает только summary.

**Phase 3: Building**

Запускается командой `./loop.sh` (без аргументов). Каждая итерация цикла следует
**10-step lifecycle**, определённому в PROMPT_build.md:

1. **Orient** — subagents читают `specs/*` (спецификации требований), чтобы агент
   понимал конечную цель. Не весь набор — только те спецификации, которые релевантны
   текущему состоянию плана.

2. **Read plan** — агент читает IMPLEMENTATION_PLAN.md, чтобы увидеть полную картину:
   что уже сделано, что осталось, какие задачи приоритетнее.

3. **Select** — агент выбирает **наиболее важную** задачу (*"pick the most important task"*).
   Важно: не первую по списку, а наиболее важную по совокупности факторов (зависимости,
   критичность, сложность). LLM сам определяет приоритет — это проявление принципа
   "Let Ralph Ralph".

4. **Investigate** — subagents изучают текущий код (`src/`), связанный с выбранной задачей.
   Критический момент: промпт явно указывает **"don't assume not implemented"** — агент
   обязан проверить, не реализована ли задача (или её часть) на предыдущих итерациях.
   Без этой инструкции агент часто переписывает уже работающий код или создаёт дубли.

5. **Implement** — N subagents выполняют файловые операции (создание, редактирование
   файлов). Главный агент координирует, subagents пишут код. Количество subagents
   определяется объёмом работы — от 1 для простых задач до 5-10 для комплексных.

6. **Validate** — **1 subagent** запускает build и тесты. Именно один, а не несколько —
   это **backpressure**: если тесты не прошли, результат возвращается главному агенту,
   который решает: исправить сейчас или отложить. Ограничение до одного subagent
   предотвращает ситуацию, когда несколько параллельных валидаций дают противоречивые
   результаты. Subagent запускает ровно те команды, которые указаны в AGENTS.md
   (например, `npm run test`, `npm run typecheck`, `npm run lint`).

7. **Update IMPLEMENTATION_PLAN.md** — агент отмечает задачу как выполненную, добавляет
   notes о найденных проблемах или открытиях. Если в процессе работы обнаружились новые
   задачи — они добавляются в план. Это делает план **живым документом**, а не статичным
   списком.

8. **Update AGENTS.md** — агент записывает operational learnings, полученные на этой
   итерации. Например: "модуль X использует barrel exports — при импорте указывать
   конкретный файл, не индекс" или "тесты для auth-модуля требуют мок JwtService".
   Эти learnings будут прочитаны на следующей итерации (шаг 1-2), что предотвращает
   повторение ошибок.

9. **Commit + push** — агент коммитит все изменения и пушит в remote. Push после **каждого**
   коммита важен: если агент "сломается" на следующей итерации (timeout, crash, context
   overflow), работа не будет потеряна. Кроме того, push позволяет другим участникам
   (людям или агентам) видеть прогресс в реальном времени.

10. **Loop ends → context cleared → fresh iteration** — скрипт `loop.sh` завершает сессию
    Claude и запускает новую. Весь conversation context сбрасывается, но состояние сохранено
    в файлах: IMPLEMENTATION_PLAN.md (что сделано), AGENTS.md (что узнали), specs/* (что
    нужно). Следующая итерация начинает с шага 1 с чистым контекстом.

#### 5.6.3. Четыре принципа

Farr формулирует четыре принципа, которые определяют **почему** playbook устроен именно
так, а не иначе. Это не просто guidelines — это инженерные решения, основанные на
понимании ограничений LLM [T07].

**1. Context Is Everything**

Контекстное окно LLM — ограниченный ресурс, и его использование определяет качество
работы агента. Claude Sonnet 4 advertises 200K токенов, но реально доступно ~176K
(остальное занимает системный промпт, tool definitions, conversation overhead). Farr
вводит понятие **"smart zone"** — диапазон утилизации контекста **40-60%**, в котором
агент работает наиболее эффективно. Ниже 40% — агент "не видит" достаточно кода чтобы
принимать хорошие решения. Выше 60% — начинается деградация: LLM "забывает" ранние
части контекста, путает файлы, теряет нить рассуждений.

Как playbook управляет контекстом:

- **Главный агент = scheduler, не worker.** Он не читает файлы и не пишет код напрямую —
  он делегирует subagents. Каждый subagent получает свою порцию контекста, работает в ней,
  возвращает результат и **утилизируется**. Farr оценивает "garbage collection" от одного
  subagent как ~156KB — это объём контекста, который высвобождается после завершения
  работы subagent.

- **Markdown вместо JSON для планов.** IMPLEMENTATION_PLAN.md написан в Markdown, а не
  в JSON (как prd.json у snarktank). Причина: Markdown занимает меньше токенов при
  эквивалентном содержании (нет скобок, кавычек, ключей), а LLM лучше "понимают"
  структурированный текст, чем машинный формат. Для state machine это ограничение
  (Markdown нельзя парсить детерминировано), но для Type 3 — преимущество.

- **Краткость = детерминизм.** *"Simplicity and brevity win; verbose inputs degrade
  determinism."* Чем лаконичнее промпт и файлы контекста, тем предсказуемее поведение
  агента. Многословные инструкции создают пространство для "интерпретации", а LLM
  склонны интерпретировать в непредсказуемую сторону.

**2. Backpressure > Direction**

Backpressure — это механизм, при котором система **отвергает плохой результат**, вынуждая
агента повторить работу. Farr противопоставляет два подхода к управлению поведением агента:

- **Upstream steering (детерминированное формирование):** файлы PROMPT.md, AGENTS.md,
  specs/*.md и существующие паттерны кода **задают ожидания** до того, как агент начал
  работу. Это "направляющие рельсы" — они не гарантируют результат, но сужают пространство
  решений. Пример: если AGENTS.md содержит "используй barrel exports из src/lib/index.ts",
  агент с высокой вероятностью будет следовать этому паттерну. Пример из кода: если в проекте
  все модули используют dependency injection, агент скорее всего продолжит этот паттерн.

- **Downstream steering (отвержение плохого результата):** тесты, typecheck, lint и
  LLM-as-Judge проверяют результат **после** реализации. Если проверка не прошла —
  это backpressure: результат отвергается, и на следующей итерации агент попробует снова.
  Пример: агент написал функцию без обработки ошибок → тест упал → на следующей итерации
  агент видит failing test → добавляет обработку ошибок.

Ключевая цитата: *"Wire validation... prompt says 'run tests' generically; AGENTS.md
specifies commands."* Промпт (PROMPT_build.md) говорит "запусти тесты" абстрактно, а
AGENTS.md указывает конкретные команды (`npm run test -- --bail`, `npx tsc --noEmit`).
Это разделение позволяет менять конкретику (AGENTS.md) без перезаписи промпта.

Backpressure сильнее direction, потому что direction — это **надежда** ("я сказал агенту
сделать X, надеюсь он сделает X"), а backpressure — это **гарантия** ("если агент сделал
не X, тесты упадут и он переделает").

**3. Let Ralph Ralph**

Этот принцип означает: **не пытайся контролировать каждое решение агента — позволь ему
ошибаться и исправляться через итерацию.** LLM — не детерминированный исполнитель команд.
Он будет делать неожиданные выборы, менять приоритеты, предлагать альтернативные решения.
Вместо борьбы с этим поведением, Farr предлагает принять его и построить систему,
устойчивую к ошибкам отдельной итерации.

Ключевое понятие: **eventual consistency через итерацию.** Агент может ошибиться на
итерации 3 (например, выбрать неправильный подход к реализации), но это обнаружится
на итерации 4-5 (тесты упадут или plan покажет нерешённую задачу) и будет исправлено
на итерации 6-7. Не нужно предотвращать каждую ошибку — нужно обеспечить, чтобы ошибки
**обнаруживались и исправлялись**.

Прямое следствие: **план одноразовый** (*"The plan is disposable"*). Если
IMPLEMENTATION_PLAN.md устарел, рассинхронизировался с кодом или содержит ошибки —
не нужно его чинить вручную. Достаточно запустить `./loop.sh plan`, и агент сгенерирует
новый план с нуля, потратив одну итерацию Planning (1-3 минуты). Стоимость пересоздания
плана ничтожна по сравнению со стоимостью работы по неактуальному плану.

**4. Move Outside the Loop**

*"Your job is now to sit on the loop, not in it."* Роль разработчика в Farr Playbook —
**не писать код и не направлять агента**, а наблюдать за паттернами его поведения и
корректировать окружение (промпты, guardrails, AGENTS.md, спецификации).

Что значит "наблюдать паттерны":
- Агент раз за разом забывает запустить typecheck → добавить в AGENTS.md
- Агент создаёт файлы не в той директории → уточнить структуру проекта в AGENTS.md
- Агент игнорирует определённый acceptance criterion → усилить формулировку в specs/
- Агент тратит 3 итерации на одну задачу → разбить задачу в IMPLEMENTATION_PLAN.md

Ключевой принцип: **"настраивай реактивно, не предписывай заранее"** (tune reactively,
not prescriptively). Не нужно пытаться предугадать все возможные проблемы и написать
исчерпывающий промпт на 10 000 слов. Лучше начать с минимального промпта, запустить loop,
увидеть конкретные проблемы и адресовать именно их. Каждая итерация наблюдения →
корректировки → наблюдения делает playbook всё более устойчивым к ошибкам конкретного
проекта.

#### 5.6.4. 999-Series Guardrails

Numbered guardrails — это приоритизированные правила поведения агента, встроенные
в PROMPT_build.md. Каждый guardrail имеет числовой приоритет, и **чем длиннее число —
тем критичнее правило**. Это не произвольный выбор: Farr обнаружил, что LLM лучше
воспринимают **длину числа** как индикатор важности, чем абсолютное значение. Число
из 15 цифр визуально и "семантически" (с точки зрения LLM) кажется важнее числа
из 5 цифр. Это трюк для приоритизации в промпте: вместо "ВАЖНО!!!" (к чему LLM
привыкают и начинают игнорировать) используется числовая шкала [T07].

| Приоритет | Guardrail | Для чего нужен |
|-----------|-----------|----------------|
| 99999 | Документируй "why" | Передача знаний: агент должен объяснять причины решений в комментариях и коммитах, чтобы следующая итерация (или человек) понимала контекст |
| 999999 | Single source of truth, no migrations | Системная согласованность: одна сущность определяется в одном месте. Нет миграций, нет копий. Предотвращает рассинхронизацию между файлами |
| 9999999 | Auto-tag releases (0.0.0+) | Отслеживание релизов: каждый значимый коммит автоматически тегируется, чтобы можно было откатиться к конкретной версии |
| 999999999 | Keep IMPLEMENTATION_PLAN current | Предотвращение дублирования работы: если план не обновлён, следующая итерация может повторить уже сделанную задачу, потратив целую итерацию впустую |
| 9999999999 | Update AGENTS.md с learnings | Повышение эффективности loop: каждая итерация должна оставлять "след" — что узнали, что не работает, какие команды использовать |
| 99999999999 | Resolve/document unrelated bugs | Здоровье системы: если агент наткнулся на баг, не связанный с текущей задачей — не игнорировать, а зафиксировать или исправить |
| 999999999999 | No placeholders, no stubs | Качество работы: каждая реализация должна быть полной. Stub (`// TODO: implement`) отравляет кодовую базу и обманывает валидацию |
| 999999999999999 | AGENTS.md = operational only | Предотвращение загрязнения контекста: AGENTS.md читается на КАЖДОЙ итерации. Если туда попадёт "объяснительный" текст (архитектурные описания, обоснования решений), это будет съедать контекстное окно без пользы |

**Как guardrails работают на практике:**

Пример 1: Агент реализовал функцию парсинга CSV, но оставил stub для обработки ошибок
(`// TODO: handle malformed rows`). Guardrail 999999999999 (No placeholders) с высшим
числовым приоритетом вынуждает агента реализовать обработку ошибок прямо сейчас, а не
откладывать.

Пример 2: Агент на итерации 5 обнаружил, что `npm run test` требует флаг `--bail`
для быстрого прерывания при первом failure. Guardrail 9999999999 (Update AGENTS.md)
вынуждает его записать это в AGENTS.md, и на итерации 6+ все subagents автоматически
используют `--bail`.

Пример 3: Агент при работе над модулем авторизации заметил, что модуль уведомлений
бросает unhandled exception при отсутствии TG_BOT_TOKEN. Guardrail 99999999999
(Resolve/document unrelated bugs) вынуждает его либо исправить баг, либо задокументировать
в IMPLEMENTATION_PLAN.md как отдельную задачу.

#### 5.6.5. Topic Scope Test

Topic Scope Test — правило **"One Sentence Without 'And'"**: может ли topic быть описан
одним предложением без соединения несвязанных capabilities через "и"? Это критически важно
для качества спецификаций и, как следствие, для качества работы агента.

**Почему это нужно:** если topic слишком широкий, его спецификация раздувается, содержит
множество разнородных требований, и LLM **теряет фокус**. Агент начинает "прыгать" между
подзадачами, путать acceptance criteria разных capabilities, и в итоге ни одну из них
не реализует полностью. Маленький, чётко определённый topic → короткая спецификация →
агент полностью понимает задачу и реализует её за одну-две итерации.

Примеры:

- **Хорошо:** *"Color extraction analyzes images to identify dominant colors"* — один topic,
  одна capability, одна спецификация.
- **Плохо:** *"User system handles auth, profiles, and billing"* — три разных topic:
  аутентификация (JWT, OAuth, sessions), профили пользователей (CRUD, аватары, настройки),
  биллинг (подписки, платежи, инвойсы). Каждый из них заслуживает отдельного файла в specs/.
- **Плохо:** *"Dashboard shows analytics and manages notifications"* — два topic: аналитика
  (графики, метрики, фильтры) и управление уведомлениями (создание, отправка, шаблоны).
  Если объединить в одну спецификацию, агент может при реализации аналитики случайно
  задеть уведомления или наоборот.
- **Хорошо:** *"Email notification service sends templated emails via SMTP"* — один topic,
  чёткий scope, одна спецификация.

### 5.7. Пять Enhancements Clayton Farr

Пять Enhancements — это **опциональные расширения** базового playbook, каждое из которых
добавляет конкретную capability. Они не являются обязательными компонентами: базовый
playbook (loop.sh + PROMPT_build.md + PROMPT_plan.md + AGENTS.md + specs/) работает
и без них. Enhancements предназначены для **постепенного наращивания** функциональности
по мере роста проекта и накопления опыта работы с AI-агентом [T07].

Farr рекомендует начать с нуля (базовый playbook) и добавлять Enhancements **по мере
обнаружения конкретных проблем**: спецификации размыты → Enhancement 1 (User Interview);
агент "жульничает" при проверке → Enhancement 2 (Acceptance-Driven Backpressure);
субъективные критерии не поддаются автотестам → Enhancement 3 (LLM-as-Judge);
агент берётся за чужие задачи → Enhancement 4 (Work Branches); продукт растёт
хаотично → Enhancement 5 (JTBD → SLC).

Именно эти Enhancements превращают Farr из "ещё одного Type 3" в нечто, функционально
сравнимое с Type 4 и Type 5.

#### Enhancement 1: User Interview (AskUserQuestionTool)

**Когда нужен:** в начале проекта, когда требования размыты, или когда для одной задачи
существует несколько равноценных подходов и нужно выбрать правильный. Также полезен
при работе с domain expert, который не может (или не хочет) писать спецификации сам,
но может ответить на конкретные вопросы.

**Как работает:** вместо того чтобы разработчик сам писал specs/*.md, он запускает Claude
Code и просит: *"Interview me using AskUserQuestion to understand [JTBD/topic/acceptance
criteria/...]"*. Claude задаёт серию уточняющих вопросов через встроенный инструмент
AskUserQuestion, который останавливает выполнение и ждёт ответа пользователя. Диалог
может выглядеть так:

1. Claude: *"Кто основной пользователь этой функции?"* → Ответ: "Студент"
2. Claude: *"Какой результат студент ожидает получить?"* → Ответ: "AI-сгенерированную обратную связь по домашке"
3. Claude: *"Какие ограничения по времени ответа?"* → Ответ: "До 30 секунд"
4. Claude: *"Может ли студент отклонить AI-ответ?"* → Ответ: "Да, и запросить ревью от ментора"
5. Claude генерирует `specs/ai-homework-feedback.md` с acceptance criteria

**Не требует изменений в коде** — используются нативные capabilities Claude Code.
Enhancement 1 улучшает Phase 1 (Define Requirements): вместо "разработчик пишет
спецификацию из головы" получается "structured interview → спецификация с acceptance
criteria, выведенными из конкретных ответов пользователя". Это снижает риск пропуска
требований и повышает качество specs/*.md.

#### Enhancement 2: Acceptance-Driven Backpressure

**Проблема, которую решает:** без явных тестовых требований агент может "сжульничать" —
пометить задачу как done, не проведя реальной проверки, или написать тесты, которые
всегда проходят (тесты-заглушки). Стандартный Type 3 (snarktank) проверяет только
`passes: boolean` — агент сам решает, прошла ли задача. Это **self-grading** — проверяющий
совпадает с исполнителем, что ненадёжно.

**Решение: вывод тестовых требований из acceptance criteria на этапе Planning, не Runtime.**

```
Phase 1: specs/*.md + Acceptance Criteria
    ↓ (агент выводит WHAT to verify)
Phase 2: IMPLEMENTATION_PLAN.md + Required Tests (derived from AC)
    ↓ (агент реализует код И тесты)
Phase 3: Implementation + Tests → Backpressure (тесты отвергают плохой результат)
```

**Связь трёх фаз:** на Phase 1 разработчик формулирует acceptance criteria в спецификациях
("изображения <5MB обрабатываются за <100ms"). На Phase 2 (Planning) агент анализирует
каждый acceptance criterion и **выводит** из него конкретное тестовое требование: "нужен
performance test: обработка 5MB изображения за <100ms". Эти тестовые требования включаются
в IMPLEMENTATION_PLAN.md рядом с задачей реализации. На Phase 3 (Building) агент обязан
реализовать **и код, и тесты** — guardrail блокирует коммит без прохождения required tests.

**Ключевой принцип: WHAT, а не HOW.** Acceptance criteria определяют **что проверять**
(outcomes), но **не как реализовать** (approach). Это даёт агенту свободу выбора
архитектурных решений:

- AC: *"Извлекает доминантные цвета из изображения за <100ms"*
- Required test: *"Performance test: <100ms для 5MB JPEG"*
- Реализация: агент сам выбирает алгоритм — K-means? Median-cut? Color quantization?
  Это его решение, и оно не регламентируется.

Ещё примеры разделения WHAT/HOW:
- AC: *"Пользователь видит уведомление в течение 5 секунд после события"*
- Required test: *"E2E: уведомление появляется за <5s после триггера"*
- Реализация: WebSocket? SSE? Polling? — выбор агента.

**Как это приближает к TDD:** тестовые требования определяются **до кода** (на этапе
Planning), но формулируются как acceptance criteria, а не как конкретные unit-тесты. Это
"TDD через планирование" — тесты known before implementation, но их конкретная форма
(Vitest? Playwright? Integration test?) остаётся на усмотрение агента.

**Добавленный guardrail:** *"999. Required tests from AC must exist and pass before
committing."* — это downstream backpressure: коммит блокируется, если тесты из acceptance
criteria не существуют или не проходят.

#### Enhancement 3: Non-Deterministic Backpressure (LLM-as-Judge)

**Проблема:** некоторые критерии качества принципиально **не поддаются программной
проверке**. Тон текста (формальный? дружелюбный? снисходительный?), визуальная гармония
интерфейса, соответствие brand guidelines, UX-чувство "правильности" — всё это невозможно
проверить автотестами. В стандартном Type 3 такие критерии просто игнорируются: если тесты
прошли и lint чистый — задача done. Но для продуктовых проектов субъективное качество
часто важнее технического.

**Решение: LLM-as-Judge — второй AI-агент оценивает результат работы первого.**

**Почему бинарный pass/fail, а не оценка 1-10?** Потому что бинарный результат = **backpressure**:
loop либо повторяется (fail), либо идёт дальше (pass). Оценка 1-10 требует порога ("выше 7 —
хорошо"), а выбор порога сам по себе субъективен и создаёт ложное чувство точности.
Бинарный вердикт проще, детерминистичнее и напрямую интегрируется в Ralph Loop: pass = next
task, fail = retry [T07].

**API (`src/lib/llm-review.ts`):**

```typescript
interface ReviewResult {
  pass: boolean;
  feedback?: string; // Только при pass=false — объяснение, что не так
}

function createReview(config: {
  criteria: string;     // Что оценивать (текстовое описание критерия)
  artifact: string;     // Текстовый контент ИЛИ путь к скриншоту (.png/.jpg)
  intelligence?: "fast" | "smart";
}): Promise<ReviewResult>;
```

**Два уровня intelligence:**
- **`"fast"`** (по умолчанию) — использует быструю модель (Claude Haiku / GPT-4o-mini).
  Подходит для простых проверок: тон текста, наличие обязательных элементов, формат ответа.
  Дёшево и быстро — можно запускать на каждой итерации без ощутимых затрат.
- **`"smart"`** — использует полноразмерную модель (Claude Sonnet/Opus / GPT-4). Для
  сложных оценок: визуальная гармония интерфейса (по скриншоту), соответствие дизайн-системе,
  quality of writing. Дороже и медленнее, используется для критически важных проверок.

**Multimodal — текст и скриншоты:**

Тип артефакта определяется автоматически по расширению файла: если `artifact` заканчивается
на `.png`, `.jpg`, `.jpeg` — функция загружает изображение и отправляет на vision evaluation.
Иначе — текстовый анализ.

```typescript
// Текстовая оценка — проверка тона сообщения
const result = await createReview({
  criteria: "Message uses warm, conversational tone without being patronizing",
  artifact: generatedMessage,
});

// Визуальная оценка — проверка UI по скриншоту
await page.screenshot({ path: "./tmp/dashboard.png" });
const result = await createReview({
  criteria: "Clear visual hierarchy with obvious primary action button",
  artifact: "./tmp/dashboard.png",
  intelligence: "smart",  // Визуальный анализ требует "умной" модели
});
```

**Eventual consistency — почему недетерминизм нормален:**

LLM-as-Judge по определению недетерминистичен: один и тот же артефакт при разных вызовах
может получить разные оценки. Farr принимает это как данность и полагается на
**eventual consistency через Ralph Loop**: если на итерации 5 review вернул `pass: false`,
агент переделывает работу; на итерации 6 результат улучшается; на итерации 7 review
возвращает `pass: true`. Даже если бы на итерации 5 review ошибочно вернул `pass: true`
для плохого результата, на следующих итерациях другие проверки (тесты, typecheck) могли бы
поймать проблему. Ключевое: система **converges** к хорошему результату через несколько
итераций, даже если отдельные проверки неидеальны.

**Чем отличается от полноценного Type 5:**

Type 5 (Multi-Model Review Loop) предполагает отдельный orchestration layer — специальный
скрипт, который запускает coding agent, потом review agent, парсит результат, принимает
решение о retry. У Farr Judge **встроен как тестовая фикстура**: `createReview()` вызывается
из обычного теста (Vitest/Jest), результат проверяется assert-ом. Для Ralph Loop это
**обычный failing test**, который запускает backpressure — никакого дополнительного
orchestration layer не нужно. Это делает Enhancement 3 значительно проще в настройке,
но ограничивает гибкость: нельзя, например, запустить review-агент с другим системным
промптом или использовать цепочку из нескольких judges.

> **Примечание:** Несмотря на эти ограничения, Enhancement 3 выводит Farr's playbook
> в территорию **Type 5** (Multi-Model Review Loop) — второй AI-агент выступает как Judge.

#### Enhancement 4: Ralph-Friendly Work Branches

**Проблема (подробнее):** допустим, в IMPLEMENTATION_PLAN.md 30 задач, охватывающих разные
области проекта: auth, UI, API, тесты, инфраструктура. Разработчик хочет, чтобы агент
работал **только над auth**. Наивный подход: добавить в промпт "работай только над задачами
auth". Проблема: LLM **в 20-30% случаев** начинает делать другие задачи [T07]. Причины:
модель "видит" весь план, "замечает" высокоприоритетную задачу из другой области и решает,
что она "тоже важна"; или модель неправильно интерпретирует scope и считает, что "API
для auth" включает рефакторинг всего API-слоя.

**Почему runtime-фильтрация ненадёжна:** когда агент на шаге 3 (Select) должен выбрать
задачу из 30 и при этом отфильтровать 24 "нерелевантные" — это **probabilistic filtering**.
LLM делает выбор на основе prompt + context, и результат недетерминирован. Чем больше
задач в плане, тем выше вероятность ошибки.

**Решение: deterministic scoping на этапе создания плана, а не выбора задачи.**

Вместо "отфильтруй нерелевантное" агент получает план, в котором **изначально только
релевантные задачи**. Для этого используются feature-ветки Git:

```bash
# Шаг 1: Полное планирование на main-ветке
./loop.sh plan
# → IMPLEMENTATION_PLAN.md с 30 задачами по всему проекту

# Шаг 2: Создание feature-ветки и scoped планирование
git checkout -b ralph/user-auth-oauth
./loop.sh plan-work "user authentication system with OAuth"
# → Агент получает PROMPT_plan_work.md, который указывает:
#   "создай IMPLEMENTATION_PLAN.md ТОЛЬКО с задачами для user auth + OAuth"
# → IMPLEMENTATION_PLAN.md содержит 6 задач вместо 30

# Шаг 3: Build на feature-ветке
./loop.sh
# → Агент на шаге 3 (Select) выбирает из 6 задач — ВСЕ релевантны
# → Вероятность "ухода в сторону" = 0% (нет куда уходить)

# Шаг 4: PR и merge
git push && gh pr create
```

**Почему deterministic scoping лучше probabilistic filtering:** при scoping решение
о том, какие задачи включить, принимается **один раз** на этапе планирования (Phase 2).
Агент анализирует описание scope ("user authentication system with OAuth"), кодовую базу
и спецификации, и генерирует план только с релевантными задачами. Если план получился
неточным (включил лишнее или пропустил нужное) — он disposable, можно перегенерировать
за одну итерацию. При probabilistic filtering решение принимается **на каждой итерации**
Build phase, и каждый раз есть шанс ошибки.

**Связь с git workflow:** каждая feature-ветка = один scope, один
IMPLEMENTATION_PLAN.md, один PR. Это естественная интеграция с привычным Git workflow:
main-ветка содержит полный план (обзор всего проекта), feature-ветки содержат scoped
планы (конкретные фичи). После merge feature-ветки можно запустить `./loop.sh plan`
на main чтобы обновить полный план.

#### Enhancement 5: JTBD → Story Map → SLC Release

**Что такое SLC (Simple/Lovable/Complete):** концепция Jason Cohen (основателя WP Engine).
Вместо MVP (Minimum Viable Product), где "minimum" часто означает "урезанный и неприятный",
SLC предлагает: Simple (простой в использовании — не "мало фич", а "мало сложности"),
Lovable (пользователь хочет вернуться), Complete (решает одну задачу полностью, без
"coming soon" заглушек). SLC-релиз — это **тонкий горизонтальный срез** через все
активности пользователя, который даёт реальную ценность с первого дня.

**Что такое Story Map:** визуализация, где горизонтальная ось — **активности пользователя**
(последовательность шагов от начала до конца), а вертикальная ось — **глубина реализации**
(от простого к сложному). Каждый горизонтальный "слой" — один релиз:

```
              UPLOAD   →  EXTRACT   →  ARRANGE    →  SHARE
Release 1:    basic       auto                        export
Release 2:                palette      manual
Release 3:    batch       AI themes    templates      embed
```

Release 1 — простейший, но **полный** путь: загрузить изображение → автоматически
извлечь цвета → экспортировать результат. Нет Arrange (ручной расстановки) — но SLC
не требует "всего", он требует **завершённости** одного пути. Пользователь может решить
свою задачу с Release 1, даже если не все фичи реализованы.

**Как AUDIENCE_JTBD.md служит single source of truth:**

Новый файл `AUDIENCE_JTBD.md` содержит описание целевой аудитории и их Jobs to Be Done.
Этот файл становится **корнем дерева** всех спецификаций: specs/*.md ссылаются на
конкретные JTBD из AUDIENCE_JTBD.md, а IMPLEMENTATION_PLAN.md ссылается на specs/*.md.
Это предотвращает **drift** — ситуацию, когда спецификации постепенно расходятся
с реальными потребностями аудитории. Если требование нельзя привязать к конкретному JTBD
из AUDIENCE_JTBD.md — это сигнал, что требование либо лишнее, либо JTBD неполон.

**PROMPT_plan_slc.md — что он делает:**

Новый промпт для Planning phase, который вместо обычного gap analysis выполняет
последовательность:
1. Читает AUDIENCE_JTBD.md → понимает, кто пользователь и что ему нужно
2. Строит **journey map** — последовательность активностей пользователя от начала до конца
3. Для каждой активности определяет уровни глубины реализации
4. **Рекомендует следующий SLC-релиз** — тонкий горизонтальный срез, дающий максимальную
   ценность при минимальной сложности
5. Генерирует IMPLEMENTATION_PLAN.md только с задачами для этого релиза

**Пример тонкого горизонтального среза для LMS-платформы:**

Активности: Регистрация → Выбор курса → Прохождение урока → Сдача задания → Получение фидбека

- Release 1: email-регистрация → один курс (hardcoded) → текстовые уроки → загрузка файла → ручной фидбек от ментора
- Release 2: OAuth → каталог курсов → видео-уроки → код в IDE → AI-фидбек
- Release 3: social login → персональные рекомендации → интерактивные уроки → автопроверка → AI + ментор вместе

Release 1 простой, но **полный**: студент проходит весь путь от регистрации до получения
фидбека. Нет OAuth, нет AI, нет видео — но задача "пройти курс и получить обратную связь"
решается полностью.

### 5.8. Farr как мост Type 3 → Type 4 → Type 5

Классифицировать Farr Playbook как "чистый Type 3" — значит упустить его главную ценность.
Формально — да: PRD-driven sequential, flat task list (IMPLEMENTATION_PLAN.md — Markdown,
не JSON DAG), passes через текстовые пометки в плане. Но пять Enhancements **фактически
реализуют ключевые свойства**, которые определяют Type 4 и Type 5 в нашей таксономии.
Farr находится на **границе между типами** и представляет собой уникальный случай —
Type 3 реализация, которая органически выросла в нечто более сложное без перехода
на формальный state machine [T07].

| Свойство | Type 3 | Farr Playbook | Type 4 | Type 5 |
|----------|--------|---------------|--------|--------|
| Phase separation | Нет | ✅ 3 фазы (Requirements/Planning/Building) | ✅ init/exec/review/finish | Varies |
| Dependency ordering | Нет | Частично (priority-sorted, not DAG) | ✅ JSON DAG | Varies |
| Review as tasks | Нет | ✅ Enh.2 (AC tests) + Enh.3 (LLM-Judge) | ✅ review tasks in state | ✅ Multi-model |
| HUMAN gates | Нет | Частично (interview, branch control) | ✅ `HUMAN:*` tasks | Optional |
| Knowledge extraction | progress.txt | ✅ AGENTS.md + IMPLEMENTATION_PLAN.md | ✅ discoveries.log | Varies |
| Multi-model review | Нет | ✅ Enh.3 (LLM-as-Judge) | Нет (same-model) | ✅ Core feature |
| Scoped execution | Нет | ✅ Enh.4 (work branches) | Zone metadata | N/A |
| Product-driven planning | Нет | ✅ Enh.5 (SLC releases) | Sprint stories | N/A |

**Комментарии к таблице:**

- **Phase separation:** Farr имеет полноценное разделение на 3 фазы с разными режимами
  loop.sh. Type 4 использует 4 фазы (init/exec/review/finish), что формальнее: каждая
  фаза — отдельный блок в JSON state file. У Farr фазы разделены **процедурно** (разные
  промпты, разные режимы запуска), но не **структурно** (нет state machine, управляющего
  переходами).

- **Dependency ordering:** "Частично" у Farr, потому что IMPLEMENTATION_PLAN.md содержит
  задачи, отсортированные по приоритету, и агент сам учитывает зависимости при выборе
  (шаг 3 — Select). Но это **probabilistic** ordering: агент может неправильно оценить
  зависимость. В Type 4 зависимости выражены явно в JSON DAG (`"blockedBy": ["task-1"]`),
  что **deterministic** — задача не может быть выбрана, пока блокирующая не завершена.

- **HUMAN gates:** "Частично" у Farr, потому что человеческое вмешательство возможно
  (Enhancement 1 — User Interview, Enhancement 4 — ветки создаёт человек), но не
  формализовано. В Type 4 задачи `HUMAN:*` явно останавливают loop до получения
  подтверждения от человека. У Farr loop не останавливается автоматически — человек
  должен сам решить, когда вмешаться (Ctrl+C → корректировка → перезапуск).

- **Knowledge extraction:** Farr использует два файла для накопления знаний (AGENTS.md +
  IMPLEMENTATION_PLAN.md notes), что богаче чем progress.txt стандартного Type 3, но
  менее структурировано чем discoveries.log в Type 4 (JSON-формат, привязка к задачам,
  timestamp).

- **Multi-model review:** у Farr есть полноценный LLM-as-Judge (Enhancement 3), чего
  нет в Type 4 (который использует ту же модель для review). Это единственное свойство,
  где Farr **опережает** Type 4 и соответствует Type 5.

**Вывод: когда выбрать Farr, а когда Type 4**

Farr Playbook с Enhancements — наиболее продвинутая Type 3 реализация, которая закрывает
разрыв между Type 3 и Types 4-5. Выбор между Farr и полноценным Type 4 (sprint-state.json
с DAG) зависит от характеристик проекта:

**Farr Playbook лучше подходит для:**
- **Greenfield-проектов** — когда нет legacy-кода и зависимостей между модулями минимальны.
  Flat task list достаточен, DAG dependencies избыточны.
- **Solo-разработки** — когда один человек + один агент. Нет необходимости в формальной
  координации между несколькими агентами или разработчиками.
- **Продуктовых проектов** — где субъективное качество (UX, tone, brand) важнее технической
  корректности. LLM-as-Judge (Enhancement 3) и SLC-планирование (Enhancement 5) —
  уникальные capabilities Farr, отсутствующие в типичном Type 4.
- **Organic evolution** — когда хочется начать просто (базовый playbook) и добавлять
  сложность по мере роста. Enhancements добавляются независимо друг от друга.

**Type 4 (state-machine) лучше подходит для:**
- **Brownfield-проектов** — с 800+ тестами, shared packages, Prisma schema,
  cross-module dependencies. JSON DAG гарантирует правильный порядок выполнения задач,
  что критически важно, когда task 3 зависит от task 1.
- **Командной работы** — когда несколько агентов или разработчиков работают параллельно.
  Формальные HUMAN gates, task claiming, lock-based coordination.
- **CI/CD-интеграции** — JSON state file легко парсится скриптами, GitHub Actions,
  dashboards. IMPLEMENTATION_PLAN.md в Markdown требует NLP для анализа.
- **Regulated environments** — где нужен audit trail: кто сделал, когда, почему.
  JSON state file предоставляет это "из коробки".

**Путь миграции:** проект может начать с Farr Playbook, вырасти до точки, где flat
task list создаёт проблемы (задачи выполняются в неправильном порядке, регрессии из-за
пропущенных зависимостей), и мигрировать на Type 4 — перенеся принципы Farr (backpressure,
LLM-as-Judge, AGENTS.md как operational memory) в новую архитектуру. Это **не замена**,
а **эволюционный путь**: Farr → Farr + Enhancements → Type 4 с Farr-принципами.

### 5.9. Почему ломается для brownfield

Type 3 предполагает **линейную** последовательность без dependencies [T33]:

1. **Нет dependency ordering.** prd.json -- flat list. Task 3 не зависит от task 1. Brownfield с shared packages, Prisma schema -- критично.
2. **Review = test+lint.** Для brownfield с 800+ тестами нужен regression pass, AC review, quality review [T33].
3. **Нет phase grouping.** Init, execute, review, finish -- всё в одном flat list без разделения.
4. **Нет human pause.** Loop бежит до конца или circuit breaker.

### 5.10. Плюсы и минусы

| Плюсы | Минусы |
|-------|--------|
| Самый популярный, rich ecosystem [T31] | Нет dependency ordering |
| Structured progress tracking (JSON) | Все items -- flat list, нет фаз |
| Git-trackable progress | Нет review phase (coding only, test+lint) |
| Circuit breaker (snarktank) [T05] | Нет multi-model verification |
| Cross-tool compatible, vendor-agnostic | Sequential only -- нет параллельности |
| progress.txt = audit trail | PRD granularity = story, не atomic task |
| Resume by design | Нет HUMAN gates |
| Low setup (~30 строк bash) | Ломается для brownfield + complex review |
| Fresh context per iteration | Prompt bloat при большом prd.json |

### 5.11. Когда использовать

**Best for:** Greenfield до 50K LOC. Linear feature development. Solo dev. Budget $10-50.
PRD из 5-15 items. Когда **review не нужен** (personal project, prototype).
Как **upgrade path** с Type 1 -- добавить prd.json к bash loop.

**Worst for:** Brownfield с 800+ тестами. Multi-phase review. Human approval gates. Team > 1.

> *"Backpressure > direction"* [ClaytonFarr, T07] -- структура PRD важнее
> детальных инструкций. AI-агент лучше работает с constraints, чем с prescriptions.

---

## 6. Тип 4: State-Machine Multi-Phase

### 6.1. Определение

**State-Machine Multi-Phase** реализации расширяют PRD-driven подход тремя
ключевыми свойствами:

1. **Dependencies** между задачами (DAG, не flat list)
2. **Phases** (init, execute, review, finish) с различными prompts и verification
3. **State transitions** с deterministic progress tracking

Это тип, рекомендованный в v5 отчёте для brownfield-проектов [T13, T33].
Ключевое отличие от Type 3: review, init, finish -- тоже задачи в state file,
не только coding tasks. **Anthropic Harness pattern** (Initializer + Coding Agent) [T10].

### 6.2. Ключевые реализации

| Реализация | Автор | Scale | Особенности |
|------------|-------|-------|-------------|
| **Anthropic Harness** | Anthropic | Reference pattern | Initializer + Coding Agent, feature_list.json [T10] |
| **Carlini C Compiler** | Nicholas Carlini | 100K LOC, $20K, 16 agents | Flat task list + lock-based claiming [T11] |
| **ralphex** | umputun | Production, 5+2 review agents | 4-phase pipeline, fresh session per task [T12] |
| **v5 sprint-loop** | Our v5 report | 1451 tests, brownfield | sprint-loop.sh + sprint-state.json [T13, T33] |
| **Levi Stringer** | Levi Stringer | Cost benchmark | $0.77 decomposed vs $1.82 one-shot [T21] |

### 6.3. Архитектура

```
sprint-loop.sh (BASH)                  CLAUDE CODE SESSION
+-------------------------------+      +------------------------------+
| while incomplete(state.json)  | ---> | 1. Read state.json           |
|   next = first(passes==false  |      | 2. Find task (deps satisfied)|
|          AND deps satisfied)  |      | 3. Read CLAUDE.md + MEMORY.md|
|   if HUMAN:* -> PAUSE         |      | 4. Execute (code/review/init)|
|   claude -p PROMPT_sprint.md  |      | 5. Set passes=true           |
|   if no_progress x3 -> STOP  | <--- | 6. Commit + exit             |
|   sleep 2                     |      +------------------------------+
| done                          |
+-------------------------------+

sprint-state.json:
+----------------------------------------------------------------+
| sprint: "NF-10"                                                |
| tasks:                                                         |
|   +--------+   +--------+   +---------+   +---------+         |
|   | init-1 |-->| init-2 |-->| exec-1  |-->| exec-2  |         |
|   | passes:|   | passes:|   | passes: |   | passes: |         |
|   | true   |   | true   |   | true    |   | false   |         |
|   +--------+   +--------+   +---------+   +---------+         |
|                                               |                |
|   +-----------+   +-----------+   +----------+|                |
|   | review-1  |<--| review-2  |   | review-3 ||  (blocked)    |
|   | passes: F |   | passes: F |   | passes:F ||                |
|   +-----------+   +-----------+   +----------+|                |
|                                               |                |
|   +-----------+   +-----------+   +-----------+                |
|   | finish-1  |   | finish-2  |   | finish-3  |  (blocked)    |
|   | passes: F |   | passes: F |   | passes: F |               |
|   +-----------+   +-----------+   +-----------+                |
+----------------------------------------------------------------+
```

### 6.4. Пример sprint-state.json

```json
{
  "sprint": "NF-10", "branch": "sprint-nf-10",
  "tasks": [
    { "id": "init-01", "phase": "init", "name": "Read MEMORY + create branch",
      "passes": false, "dependencies": [] },
    { "id": "exec-01", "phase": "execute", "name": "Story nf-10-1: Add preferences",
      "passes": false, "dependencies": ["init-01"], "zone": "backend", "sp": 3 },
    { "id": "review-01", "phase": "review", "name": "Full regression suite",
      "passes": false, "dependencies": ["exec-01"] },
    { "id": "review-02", "phase": "review", "name": "AC review",
      "passes": false, "dependencies": ["review-01"] },
    { "id": "HUMAN: Review UI", "phase": "review", "name": "HUMAN: Review UI",
      "passes": false, "dependencies": ["review-02"] },
    { "id": "finish-01", "phase": "finish", "name": "Extract discoveries",
      "passes": false, "dependencies": ["review-02"] }
  ]
}
```

### 6.5. Ключевой паттерн: "Everything is a Ralph Loop"

Главный инсайт v5 [T13, T33]: не нужно оборачивать в Ralph Loop только фазу кодирования.
ВСЕ фазы декомпозируются в атомарные задачи:

```
INIT:     Read MEMORY.md, create branch, reindex       <- Ralph iterations
EXECUTE:  Implement story-1, story-2, story-3          <- Ralph iterations
REVIEW:   Run tests, check AC, check quality, fix      <- Ralph iterations
FINISH:   Extract discoveries, update docs, summary    <- Ralph iterations
```

**Evidence:**
- Carlini [T11]: ВСЕ задачи (code, test, review, merge) -- flat list
- ralphex [T12]: Review agents = Task tool calls в свежих сессиях
- hamelsmu [T14]: Review -- часть итеративного цикла
- Stringer [T21]: Atomic decomposition = $0.77 vs $1.82 one-shot

### 6.6. Sprint-state.json vs prd.json

| Свойство | prd.json (Type 3) | sprint-state.json (Type 4) |
|----------|-------------------|---------------------------|
| Dependencies | Нет | Да (`"dependencies": ["init-01"]`) |
| Phases | Нет | Да (`"phase": "review"`) |
| Granularity | Story/feature | Atomic task |
| HUMAN gates | Нет | Да (`"name": "HUMAN: approve PR"`) |
| Review tasks | Нет (coding only) | Да (review = tasks в том же списке) |
| Circuit breaker | Отдельный код | Встроен в loop (md5sum state) |
| Zone/SP metadata | Нет | Да (`"zone": "backend", "sp": 3`) |
| Knowledge extraction | Нет | discoveries.log + finish tasks |

### 6.7. Почему Type 4 для brownfield

Brownfield (existing codebase, 800+ тестов, inter-module deps) требует [T33]:

1. **Dependency ordering.** `"dependencies": ["exec-01"]` -- prd.json (Type 3) этого не умеет.
2. **Review как отдельные задачи.** Не test+lint (Type 3), а dedicated: regression, AC review, quality, fix findings, E2E. Каждая -- fresh session с focused prompt [T12, ralphex].
3. **Knowledge extraction per-iteration.** `discoveries.log` (append-only) + finish-задачи [T33].
4. **Circuit breaker + human pause.** `HUMAN:*` stops loop. 3 no-progress iterations = stop.
5. **Full resume.** sprint-state.json на диске. Crash -> loop продолжает с первой `passes:false`.

### 6.8. Carlini: масштаб Type 4

Nicholas Carlini [T11]: 100K строк, C-компилятор (Linux 6.9, x86/ARM/RISC-V).
16 агентов, ~2000 сессий, $20K. Flat task list с lock-based claiming.
Хотя Carlini использует parallel agents (Type 6), его **state management** --
чистый Type 4: feature_list.json с `passes: boolean`, dependencies через lock files.
Доказательство: Type 4 масштабируется до 100K+ LOC.

> *"Most of his effort went into designing the environment around Claude --
> the tests, the environment, the feedback."* [Anthropic, T10/T11]

### 6.9. Плюсы и минусы

| Плюсы | Минусы |
|-------|--------|
| ВСЕ фазы в одном flat list | Сложнее prd.json -- нужно проектировать dependencies |
| Dependencies = правильный порядок (DAG) | Требует upfront decomposition (sprint planning) |
| HUMAN gates для approval | State file растёт с количеством задач |
| Circuit breaker встроен (md5sum) | Нет native multi-model review (нужен Type 5) |
| Fresh context = no compaction | Sequential execution (нет параллельности) |
| Resume by design (Ctrl+C safe) | Bash зависимость (Windows нужен WSL/Git Bash) |
| Git-trackable progress | Нужен jq для JSON parsing |
| -83% orchestration code vs skills [T13] | Нет встроенного rate limiting |
| Cost-efficient ($0.80-1.50/iter) [T21] | Overhead для trivial projects |
| Proven at scale (Carlini, 100K LOC) | State file corruption risk |

### 6.10. Когда использовать

**Best for:** Brownfield 10K-100K+ LOC. Multi-story sprints. 100+ тестов + regression risk.
Solo/small team. Sprint-based workflow (agile/scrum).
Когда нужен **audit trail** и **reproducibility**.

**Worst for:** One-off tasks (Type 1). Greenfield hello-world (Type 3). Projects without tests.

### 6.11. Почему это рекомендация v5

Type 4 -- оптимальный баланс между сложностью и возможностями для brownfield:

1. **Type 1-3 недостаточны:** нет review, нет dependencies, coding-only scope
2. **Type 5-6 избыточны:** multi-model и parallel agents добавляют cost и complexity
   без пропорционального увеличения качества для одного разработчика
3. **Type 4 закрывает gap:** review как задачи, dependencies, HUMAN gates --
   всё, что нужно для production-grade sprints

> *"Each step = 1 iteration. Atomic decomposition: $0.77 vs $1.82"* [Stringer, T21]

---

## 7. Тип 5: Multi-Model Review Loop

### 7.1. Определение

**Multi-Model Review Loop** добавляет к итеративному циклу **второго AI-агента**
(другую модель или тот же model с другой ролью), который выступает reviewer/judge.
Coding agent и review agent работают **последовательно**: code -> review -> fix -> review.
Два "мозга" с разными objectives, часто разными моделями (Claude + Codex, Claude + Gemini)
[T14, T15, T16].

Ключевое отличие от Type 4: в Type 4 review -- задача для того же агента с другим prompt.
В Type 5 review -- **отдельная модель** (Codex, GPT-4, второй Claude) или агент
с read-only доступом (Judge Agent pattern).

### 7.2. Ключевые реализации

| Реализация | Автор | Models | Pattern |
|------------|-------|--------|---------|
| **claude-review-loop** | Hamel Husain | Claude + Codex | Stop hook -> review -> fix [T14] |
| **Goose ralph-loop** | Block | Worker + Reviewer | SHIP/REVISE [T15] |
| **ralph-loop-agent** | Vercel Labs | Coder + Judge | verifyCompletion callback [T16] |
| **continuous-claude** | Anand Chowdhary | Claude + CI | PR lifecycle, worktrees [T17] |

**Примечание:** Clayton Farr Ralph Playbook [T07] реализует Non-Deterministic Backpressure
(Enhancement 3) через `llm-review.ts` — LLM-as-Judge fixture в тестовом фреймворке.
Формально Type 3, но этот Enhancement привносит Type 5 multi-model review capability
внутрь Ralph loop через binary pass/fail API с multimodal support (text + screenshots).

### 7.3. Архитектура

```
OUTER LOOP
+----------------------------------------------------------+
| while review != APPROVE:                                  |
|   CODING AGENT -> executes task, writes work-summary      |
|   REVIEWER     -> reads diff+summary -> APPROVE/REVISE    |
|   if REVISE: feedback -> next coding iteration            |
+----------------------------------------------------------+

CODING AGENT              REVIEWER AGENT
+-----------------+       +------------------+
| Model: Claude   |       | Model: Codex/    |
| Role: Implement |       |   Gemini/Claude  |
| Access: R+W     |       | Access: Read-only|
| Output: code    |       | Output: APPROVE  |
|   + summary     |       |   or feedback    |
+-----------------+       +------------------+

State files:
  task.md             <- task description
  work-summary.txt    <- coder output
  review-result.txt   <- SHIP or REVISE + feedback
```

### 7.4. Паттерны верификации

Три паттерна верификации в Type 5:

**1. SHIP/REVISE (Goose)** [T15]
```
Reviewer -> "SHIP" (binary yes/no)
         -> "REVISE: [feedback]"
```
Простой, deterministic. Подходит для задач с чётким AC.
State files: `task.md`, `work-summary.txt`, `review-result.txt`.

**2. Judge Agent (Vercel)** [T16]
```
Judge -> approveTask(summary)
      -> requestChanges(feedback, failedChecks[])
```
Structured feedback с конкретными check failures. Judge имеет read-only доступ --
не может модифицировать код, только оценивать. Это гарантирует **separation of concerns**.
Nested loops: Ralph (outer) wraps AI SDK Tool Loop (inner) [T30].

```typescript
// Vercel pattern (simplified)
const result = await runJudge({ summary, diff, acceptanceCriteria });
result.approved ? markComplete(task) : injectFeedback(result.feedback);
```

**3. Cross-Model Review (hamelsmu)** [T14]
```
Claude (coder) -> Codex (reviewer) -> Claude (fixer)
                      |
               Different model =
               different blind spots
```
"Fresh eyes" эффект: разные модели имеют разные bias patterns,
перекрёстная проверка выявляет больше проблем.
State: `.claude/review-loop.local.md`.

**4. CI-as-Reviewer (continuous-claude)** [T17]
```
Claude -> PR -> CI (automated reviewer) -> fix failures -> repeat
```
`SHARED_TASK_NOTES.md` = inter-session memory. Parallel worktrees для
одновременной работы над несколькими PR. CI results как review signal.

### 7.5. Зачем два разных "мозга"

Использование **другой модели** для review [T14, T20]:

1. **Different reasoning** -- Codex видит ошибки, к которым Claude "привык"
2. **No confirmation bias** -- reviewer не помнит design decisions
3. **Cost optimization** -- reviewer может быть дешевле (Codex vs Opus)
4. **Separation of concerns** -- coder: "make it work"; reviewer: "make it right"

> *"Fresh eyes from a different model catch what the coder missed."* [Husain, T14]
> *"Opus for judgment, Sonnet for iteration."* [Tessmann, T20]

### 7.6. Плюсы и минусы

| Плюсы | Минусы |
|-------|--------|
| Separation of concerns (coder != reviewer) | 2x API cost (два агента на задачу) |
| Cross-model catches different bugs [T14] | Сложнее debug (два контекста) |
| SHIP/REVISE -- deterministic exit [T15] | Review agent может быть overly strict |
| Judge = read-only = safe [T16] | Needs prompt engineering для обоих agents |
| Nested loops for complex tasks [T30] | Sequential bottleneck (code -> review -> fix) |
| CI integration (continuous-claude) [T17] | Rate limiting для двух providers |
| No confirmation bias (другая модель) | Feedback loop может зациклиться |
| Catches design issues, не только bugs | Reviewer context = только diff+summary |
| Proven (hamelsmu, Vercel, Goose) | Setup complexity |

### 7.7. Когда использовать

**Best for:** High quality bar (fintech, security). Brownfield + complex business logic.
Test coverage < 60%. Budget $50-200/sprint.
**Production code** с высокими quality requirements.
Когда **единственный разработчик** -- нет human reviewer.
Когда **один model** систематически пропускает определённый класс ошибок.

**Worst for:** Prototyping/MVP. Solo dev, budget < $20. Coverage 90%+ (tests catch most issues).

### 7.8. Совмещение с Type 4

Type 5 **не исключает** Type 4. В v5-архитектуре review tasks в sprint-state.json
могут использовать multi-model verification:

```json
{
  "id": "review-02",
  "name": "AC review with Judge Agent",
  "description": "Use Codex as reviewer. SHIP/REVISE protocol.",
  "model": "codex",
  "passes": false
}
```

Это hybrid: Type 4 state management + Type 5 verification.

---

## 8. Тип 6: Multi-Agent Parallel

### 8.1. Определение

**Multi-Agent Parallel** реализации запускают несколько AI-агентов **одновременно**,
каждый в своём git worktree или docker container. Координация через shared state
(lock files, task claiming, message passing). Масштабирование -- горизонтальное:
больше агентов = больше throughput. 2-16-100+ агентов [T11, T18, T19].

### 8.2. Ключевые реализации

| Реализация | Автор | Agents | Tech |
|------------|-------|--------|------|
| **Carlini C Compiler** | Nicholas Carlini | 16 | Claude Code + Docker + git worktrees [T11] |
| **multi-agent-ralph-loop** | alfredolopez80 | N | Agent Teams + Memory MCP, 3 слоя memory [T18] |
| **ralph-orchestrator** | Mikey O'Brien | N | Rust, 7 backends, Hat System, TUI [T19] |
| **Tessmann hybrid** | Meag Tessmann | Teams + Ralph | Agent Teams creative + Ralph mechanical [T20] |
| **Anthropic Agent Teams** | Anthropic | N | Native multi-agent в Claude Code (Opus 4.6) [S18] |

### 8.3. Архитектура

```
ORCHESTRATOR (shared task list + locks)
       |              |              |              |
       v              v              v              v
+------------+ +------------+ +------------+ +------------+
| AGENT 1    | | AGENT 2    | | AGENT 3    | | AGENT N    |
| worktree-1 | | worktree-2 | | worktree-3 | | worktree-N |
| Lock task  | | Lock task  | | Lock task  | | Lock task  |
| Execute    | | Execute    | | Execute    | | Execute    |
| Test+Commit| | Test+Commit| | Test+Commit| | Test+Commit|
| Unlock     | | Unlock     | | Unlock     | | Unlock     |
+------------+ +------------+ +------------+ +------------+
       |              |              |              |
       v              v              v              v
+-------------------------------------------+
|         Shared Git Repository              |
|  +------+  +------+  +------+  +------+   |
|  |lock-A|  |lock-B|  |lock-C|  |lock-D|   |
|  +------+  +------+  +------+  +------+   |
|                                            |
|  feature_list.json (shared state)          |
+-------------------------------------------+
```

### 8.4. Lock-based Task Claiming (Carlini pattern)

```
# Pseudo-code (16 agents, Docker + git worktrees)
while has_unclaimed_tasks(); do
  task = pick_random_unclaimed()
  if create_lock_file("locks/$task.lock", agent_id); then
    git pull; execute(task); test(); commit+push; mark_complete(task)
  else continue  # Another agent claimed it
  fi
done
```

Lock файлы в git -- простой, но достаточный mechanism.
Конфликты решаются через git merge. Если два агента создают lock одновременно --
первый коммит wins (optimistic locking).
16 agents + ~2000 sessions + $20K = C compiler [T11].

> *"Lock-based claiming via text files -- analogue of database locks, but in .txt"*
> [Carlini/Anthropic, T11]

### 8.5. Hat System (O'Brien)

ralph-orchestrator [T19] использует "Hat System" -- каждый агент получает
специализированную "шляпу" (persona) для текущей итерации:

| Hat | Persona | Когда |
|-----|---------|-------|
| Coder | Implementation specialist | Реализация feature |
| Reviewer | Quality inspector | Code review |
| Architect | System designer | API design, schema changes |
| Tester | QA specialist | Test writing, E2E |
| Documenter | Technical writer | Docs, README |

"Шляпа" -- это prompt modifier, не отдельная модель. Один агент может менять шляпы
между итерациями. Backpressure gates контролируют, когда агент должен сменить шляпу
(например, после 3 coding итераций -- обязательный review).

Дополнительно: 7 AI backends, TUI dashboard, git checkpointing at 70% context usage.

### 8.6. Agent Teams (Anthropic native)

Claude Code Opus 4.6 поддерживает нативные Agent Teams [S18]:
- Shared task list (TaskCreate, TaskUpdate, TaskList)
- Teammate messaging (SendMessage)
- Parallel execution с координацией
- Team config: `~/.claude/teams/{team-name}/config.json`

Это **первый official multi-agent runtime** от Anthropic. Experimental,
но указывает направление эволюции: от bash orchestration к native multi-agent.

alfredolopez80 [T18]: Memory-driven planning + multi-agent execution + Agent Teams integration.
Три слоя memory: persistent (claude-mem MCP), local JSON (state), ledgers (transaction logs).

Tessmann [T20]: Agent Teams для creative work (architecture, UX),
Ralph Loop для mechanical (coding, testing). Opus для judgment, Sonnet для iteration.

### 8.7. Когда параллелизм оправдан

Только при **независимых задачах** [T11, T22]:

| Parallel-safe | Must be sequential |
|---------------|-------------------|
| Backend story 1 + Frontend story 2 | Story 2 depends on shared pkg from Story 1 |
| Docs + Test infra | Schema migration affects multiple modules |
| Module A tests + Module B tests | Shared util refactor |

**Правило:** если >50% задач в спринте имеют dependencies на общие файлы --
используйте Type 4 (sequential), не Type 6 (parallel).
Tasks в разных worktrees ДОЛЖНЫ модифицировать разные файлы.
Merge conflicts > speedup = net loss [T11].

### 8.8. Пример: parallel worktrees (minimal)

```bash
#!/usr/bin/env bash
git worktree add ../worker-backend sprint-nf-10
git worktree add ../worker-frontend sprint-nf-10

jq '.tasks |= map(select(.zone == "backend"))' state.json > ../worker-backend/state.json
jq '.tasks |= map(select(.zone == "frontend"))' state.json > ../worker-frontend/state.json

(cd ../worker-backend && ./sprint-loop.sh) &
(cd ../worker-frontend && ./sprint-loop.sh) &
wait

git merge worker-backend worker-frontend --no-edit
npm run test  # Full regression
```

### 8.9. Плюсы и минусы

| Плюсы | Минусы |
|-------|--------|
| N agents = ~N throughput (horizontal scaling) | N agents = ~N cost |
| Independent worktrees = no conflicts (if tasks are independent) | Merge conflicts between agents on shared files |
| Scales to 100K+ LOC [Carlini, T11] | Complex orchestration (Rust/Node, not bash) |
| Hat System = specialized agents [T19] | Debugging N agents simultaneously |
| Agent Teams = native support [S18] | Rate limiting * N |
| Backpressure gates control quality [T19] | Requires independent tasks (no deps) |
| TUI dashboard for monitoring [T19] | Lock contention at scale |
| Natural for frontend+backend split | Complex setup (worktrees, locks) |

### 8.10. Когда использовать

**Best for:** Large projects 100K+ LOC. Clear module boundaries. Team 2+. 5+ independent stories.
Budget $100-500+. Жёсткий deadline -- нужен throughput.
Есть навыки DevOps для настройки worktrees, lock files, monitoring.

**Worst for:** Solo dev (overhead > speedup). < 10K LOC. Heavy inter-dependencies.
Budget < $50. Stories с heavy shared deps.

### 8.11. Предупреждение: Premature Parallelization

> *"Agent Teams for creative work, Ralph Loop for mechanical."* [Tessmann, T20]

Параллельность **не всегда** улучшает результат. Для задач с high inter-dependency
(brownfield refactoring, schema migration) последовательное выполнение (Type 4)
может быть **быстрее**, потому что merge conflicts от параллельных агентов
обходятся дороже, чем ожидание.

---

## 9. Decision Matrix

### 9.1. Быстрый выбор по проекту

| Если ваш проект... | Используйте | Тип |
|---------------------|-------------|-----|
| Greenfield, 1 feature, прототип | Pure Bash Loop | 1 |
| Быстрая задача, Windows, no bash | Plugin/Hook | 2 |
| Greenfield, PRD из 5-15 items | PRD-Driven Sequential | 3 |
| Brownfield, 500+ тестов, sprints | State-Machine Multi-Phase | **4** |
| Production code, нет human reviewer | Multi-Model Review Loop | 5 |
| 50K+ LOC, 10+ независимых задач, бюджет не ограничен | Multi-Agent Parallel | 6 |

### 9.2. Быстрый выбор по ресурсам

| Если у вас... | Используйте | Тип |
|---------------|-------------|-----|
| 10 минут на настройку | Pure Bash Loop | 1 |
| npm install и 5 минут | Plugin/Hook | 2 |
| 30 минут + PRD документ | PRD-Driven Sequential | 3 |
| 1 час + sprint planning | State-Machine Multi-Phase | **4** |
| 2 часа + два API key | Multi-Model Review Loop | 5 |
| 1 день + DevOps навыки | Multi-Agent Parallel | 6 |

### 9.3. Полная decision matrix по осям

| Project Complexity | Team | Budget | Verification | Recommended Type |
|---|---|---|---|---|
| Trivial (one-off refactor) | Solo | < $5 | None | **Type 1** |
| Simple greenfield < 5K LOC | Solo | < $20 | test+lint | **Type 3** |
| Greenfield 5-50K LOC | Solo | $20-50 | test+lint+AC | **Type 3** or **Type 4** |
| Brownfield 10-100K LOC | Solo | $20-100 | Regression+AC+quality | **Type 4** |
| Brownfield, complex review | Solo/Small | $50-200 | Multi-reviewer+E2E | **Type 4+5** |
| Large project 100K+ LOC | Team 2-5 | $100-500 | Full CI/CD | **Type 4+6** |
| Enterprise 1M+ LOC | Team 5+ | $500+ | All of above | **Type 4+5+6** |
| Quick debug (3-5 iterations) | Solo | < $5 | None | **Type 2** |

### 9.4. Полная таблица характеристик

| Критерий | Type 1 | Type 2 | Type 3 | Type 4 | Type 5 | Type 6 |
|----------|--------|--------|--------|--------|--------|--------|
| **Setup time** | 5 мин | 5 мин | 30 мин | 1 час | 2 часа | 1 день |
| **LOC overhead** | ~5-10 | ~0-50 (plugin) | ~30-80 | ~80-150 | ~100-200 | ~200-500+ |
| **API cost/iter** | $0.5-1.5 | $0.5-3.0* | $0.5-1.5 | $0.8-1.5 | $1.5-3.0 | $0.8-1.5 x N |
| **Context quality** | Fresh (optimal) | Degrades** | Fresh | Fresh | Fresh | Fresh |
| **Review** | None | None | None (test+lint) | Same-model tasks | Cross-model | Per-agent |
| **Dependencies** | None | None | None | DAG | Limited | Lock-based |
| **Parallel** | No | No | No | No | No | Yes |
| **HUMAN gates** | Manual | Manual | Manual | Built-in | Optional | Optional |
| **Resume** | Git only | No | JSON (partial) | JSON + deps (full) | State files (partial) | Locks + state (full) |
| **Vendor lock-in** | None | Claude Code | Low | Low | Multi-vendor | Multi-vendor |
| **Max project size** | 1K LOC | 10K LOC | 50K LOC | 100K+ LOC | 100K+ LOC | 1M+ LOC |
| **Best for** | Solo, simple | Quick, interactive | Greenfield PRD | Brownfield sprint | Quality-critical | Large scale |

*\* Plugin cost higher due to compaction overhead*
*\*\* Degrades after ~40% context usage [AI Hero, T03]*

### 9.5. Upgrade paths

Типы образуют natural upgrade paths. Начните с простого, усложняйте по необходимости:

```
Type 1 -> Type 3:  +prd.json с passes:boolean        (+30 min)
                    Trigger: "Нужно помнить прогресс"

Type 3 -> Type 4:  +dependencies, phase grouping      (+2-4 hours)
                    Trigger: "test+lint недостаточно для review"

Type 3 -> Type 3+: +Farr Enhancements (AC, LLM-Judge) (+1-2 hours)
                    Trigger: "Нужен structured review без полного Type 4"

Type 4 -> Type 4+5: +Judge Agent / second model       (+4-8 hours)
                     Trigger: "Bugs в production после merge"

Type 4 -> Type 4+6: +worktrees + parallel execution   (+1 day)
                     Trigger: "8 independent stories, need speed"

Type 2 -> ???:       ТУПИК. Переписать на Type 3/4.
                     Trigger: "Context degradation после 10 iter"
```

**Рекомендованный upgrade path для нового проекта:**

1. Начните с **Type 1** (bash loop) для первых задач
2. Когда PRD > 5 items, перейдите на **Type 3** (добавьте prd.json)
3. Когда проект достигнет 500+ тестов и multi-phase sprints, перейдите на **Type 4**
4. Если нет human reviewer и quality критична, добавьте **Type 5** review
5. Если задач > 10 и они независимы, масштабируйтесь через **Type 6**

### 9.6. Anti-patterns

| Тип | Anti-pattern | Последствие |
|-----|-------------|-------------|
| Type 1 | Brownfield с 800+ тестами | Regression bugs, нет review |
| Type 2 | Sprint 20+ итераций | "Dumb zone", cost explosion [T03] |
| Type 3 | Brownfield + inter-module deps | Broken builds, нет dep ordering |
| Type 4 | One-off trivial task | Overhead > task itself |
| Type 5 | Project с 95% coverage | Reviewer adds cost, no value |
| Type 6 | Stories с heavy shared deps | Git conflicts > speedup |

### 9.7. v5 рекомендация в контексте таксономии

Наш проект (MentorLearnPlatform):
- **1451 тест**, 12 спринтов, NestJS + React monorepo
- Brownfield: inter-module dependencies, Prisma schema, 800+ backend тестов
- Solo developer + AI agent
- Sprint-based workflow

**Рекомендация v5 = Type 4** с optional Type 5 review:

```
sprint-loop.sh (Type 4)
  +-- init tasks
  +-- execute tasks (stories)
  +-- review tasks
  |     +-- review-02: "AC review" [optional: Type 5 Judge Agent]
  |     +-- review-03: "Quality review" [optional: Type 5 Codex review]
  +-- finish tasks
```

| v5 Requirement | Type 4 Solution |
|----------------|-----------------|
| Dependency ordering | `dependencies` field в JSON DAG |
| Review как задачи | review-01..05 в state file |
| Knowledge extraction | discoveries.log + finish tasks |
| Circuit breaker | md5sum check (3 no-progress = stop) |
| Human pauses | `HUMAN:*` task names |
| Resume after crash | `passes: boolean` в JSON |
| Cost efficiency | Fresh context = $0.80-1.50/iter |

Type 4 покрывает 95% потребностей. Type 5 добавляется опционально для review tasks,
когда human reviewer недоступен. Type 6 не нужен для solo developer.

---

## 10. Sources

### Primary Sources (Tier A)

| ID | Источник | Автор | Тип | Релевантность |
|----|----------|-------|-----|---------------|
| T01 | ghuntley.com/ralph | Geoffrey Huntley | 1 | Оригинальный паттерн, "compaction is the devil" |
| T02 | anthropics/claude-code/plugins/ralph-wiggum | Boris Cherny / Anthropic | 2 | Официальный plugin, Stop hook |
| T10 | anthropic.com/engineering/effective-harnesses | Anthropic | 4 | Initializer + Coding Agent |
| T11 | anthropic.com/engineering/building-c-compiler | Nicholas Carlini | 4+6 | 16 agents, ~2000 sessions, $20K, flat list |
| T13 | v5 sprint-loop.sh + sprint-state.json | Наш v5 отчёт | 4 | Рекомендация: all phases = atomic tasks |
| T33 | ralph-loop-v5-final-2026-02-22.md | Наш v5 отчёт | 4 | "Everything is a Ralph Loop" |

### Secondary Sources (Tier A-B)

| ID | Источник | Автор | Тип |
|----|----------|-------|-----|
| T03 | aihero.dev | AI Hero | 2 critique -- context degradation, "dumb zone" after 40% |
| T04 | frankbria/ralph-claude-code | Frank Bria | 2 enhanced -- circuit breaker, 484 tests |
| T05 | snarktank/ralph | snarktank | 3 -- prd.json, passes:boolean, progress.txt |
| T06 | iannuttall/ralph | Ian Nuttall | 3 -- minimal, multi-provider |
| T07 | ClaytonFarr/ralph-playbook | Clayton Farr | 3 (→4/5 bridge) -- 3 phases, 4 principles, 5 Enhancements, LLM-as-Judge, JTBD→SLC |
| T08 | fstandhartinger/ralph-wiggum | Florian Standhartinger | 3 -- spec-driven, SpecKit |
| T09 | choo-choo-ralph | MJ Meyer | 3+4 -- Beads-powered, 5-phase, compounding knowledge |
| T12 | umputun/ralphex | umputun | 4+5 -- 4-phase pipeline, 5+2 review agents |
| T14 | hamelsmu/claude-review-loop | Hamel Husain | 5 -- Claude + Codex review loop |
| T15 | Goose ralph-loop | Block | 5 -- SHIP/REVISE pattern |
| T16 | vercel-labs/ralph-loop-agent | Vercel Labs | 5 -- Judge Agent, nested loops |
| T17 | AnandChowdhary/continuous-claude | Anand Chowdhary | 5 -- PR lifecycle, parallel worktrees |
| T18 | alfredolopez80/multi-agent-ralph-loop | alfredolopez80 | 6 -- Agent Teams, Memory MCP |
| T19 | mikeyobrien/ralph-orchestrator | Mikey O'Brien | 6 -- Rust, 7 backends, Hat System, TUI |
| T20 | medium.com/@himeag | Meag Tessmann | 4+6 -- Agent Teams creative + Ralph mechanical |
| T21 | medium.com/@levi_stringer | Levi Stringer | 4 -- atomic decomposition, $0.77 vs $1.82 |
| T22 | chrismdp.com | Chris Parsons | 3/4 -- "for loop is legitimate orchestration" |
| T23 | paddo.dev | paddo.dev | 3/4 -- SDLC collapse, harness != decisions |
| T24 | bytesizedbrainwaves.substack.com | Gordon Mickel | 1/3 -- "malloc orchestrator", state in files |

### Tertiary Sources

| ID | Источник | Автор | Тип |
|----|----------|-------|-----|
| T25 | agrimsingh/ralph-wiggum-cursor | Agrim Singh | 2 (Cursor) -- token tracking, rotation at 80K |
| T26 | Th0rgal/opencode-ralph-wiggum | Th0rgal | 2 (OpenCode) -- struggle detection |
| T27 | KLIEBHAN/ralph-loop | KLIEBHAN | 1 -- external loop for Claude/Codex |
| T28 | nitodeco/ralph | nitodeco | 3 -- CLI, PRD-driven |
| T29 | ralph-wiggum.ai | fstandhartinger | 3 -- SpecKit docs |
| T30 | deepwiki.com/vercel-labs/ralph-loop-agent | DeepWiki | 5 -- nested loops analysis |
| T31 | awesome-ralph/README.md | snwfdhmp | Catalog -- 20+ implementations |
| T32 | linearb.io/ralph-loop | LinearB | Meta -- industry adoption |
| S35 | ralph-loop-v3-2026-02-21.md | Наш v3 отчёт | 16+ implementations survey |
| S36 | ralph-loop-gaps-2026-02-21.md | Наш v4 отчёт | Gap analysis, 5 unsolved problems |

### Key URLs

- Huntley: https://ghuntley.com/ralph/
- Anthropic Harness: https://anthropic.com/engineering/effective-harnesses-for-long-running-agents
- Carlini: https://anthropic.com/engineering/building-c-compiler
- Parsons: https://chrismdp.com/your-agent-orchestrator-is-too-clever/
- AI Hero: https://aihero.dev/why-the-anthropic-ralph-plugin-sucks
- snarktank: https://github.com/snarktank/ralph
- hamelsmu: https://github.com/hamelsmu/claude-review-loop
- Vercel: https://github.com/vercel-labs/ralph-loop-agent
- continuous-claude: https://github.com/AnandChowdhary/continuous-claude
- ralphex: https://ralphex.umputun.dev
- ralph-orchestrator: https://mikeyobrien.github.io/ralph-orchestrator
- ClaytonFarr: https://claytonfarr.github.io/ralph-playbook/
- frankbria: https://github.com/frankbria/ralph-claude-code
- awesome-ralph: https://github.com/snwfdhmp/awesome-ralph

---

## Appendix A: Glossary

| Термин | Определение |
|--------|-------------|
| **Ralph Loop** | Паттерн автономной итерации AI-агента: loop -> task -> verify -> repeat |
| **Fresh context** | Каждая итерация начинается с чистого контекстного окна (нет carry-over) |
| **Compaction** | Сжатие контекста LLM при приближении к лимиту (lossy operation) |
| **Circuit breaker** | Механизм остановки loop при отсутствии прогресса (N итераций без изменений) |
| **HUMAN gate** | Точка в workflow, требующая human approval перед продолжением |
| **passes: boolean** | Binary маркер завершения задачи в state file |
| **Atomic task** | Задача, выполнимая за одну сессию одним агентом с чётким exit condition |
| **Judge Agent** | AI-агент с read-only доступом, оценивающий работу coding agent |
| **SHIP/REVISE** | Binary protocol: reviewer возвращает SHIP (accept) или REVISE (reject + feedback) |
| **Worktree** | Git worktree -- независимая рабочая копия репозитория для параллельной работы |
| **Hat System** | Persona-switching: один агент получает разные "шляпы" (coder, reviewer, architect) |
| **Backpressure** | Механизм замедления/остановки при накоплении необработанных задач |
| **DAG** | Directed Acyclic Graph -- граф зависимостей между задачами без циклов |
| **Nested loops** | Ralph (outer loop) wraps AI SDK Tool Loop (inner), двухуровневая итерация |

## Appendix B: Quick Reference Card

```
ВЫБЕРИ ТИП:

  Один файл / quick fix?           --> Type 1 (Bash Loop)
  Windows, нет terminal?            --> Type 2 (Plugin)
  PRD с 5-15 фичами, greenfield?   --> Type 3 (PRD-Driven)
  Brownfield, sprints, review?      --> Type 4 (State-Machine) <-- v5 рекомендация
  Production, нет human reviewer?   --> Type 5 (Multi-Model Review)
  50K+ LOC, 10+ параллельных задач? --> Type 6 (Multi-Agent Parallel)

  Не уверен? Начни с Type 1. Усложняй когда больно.
```
