# bmalph: Анализ реализации Ralph Loop + BMAD интеграции

> **Дата:** 2026-02-23
> **Версия:** 1.0
> **Объект анализа:** https://github.com/LarsCowe/bmalph
> **Контекст:** Анализ с точки зрения v6-таксономии Ralph Loop и сравнения с Clayton Farr Playbook
> **Предшествующие отчёты:** ralph-loop-v5-final-2026-02-22.md, ralph-loop-v6-taxonomy-2026-02-22.md
> **Языковая политика:** Русский текст, технические термины на английском

---

## 1. Что такое bmalph

**bmalph** — npm-пакет (`npm install -g bmalph`), который объединяет два AI-инструмента:

- **BMAD-METHOD v6.0.0-Beta.8** — система агентов для планирования (Phases 1-3)
- **Ralph v0.11.4** — autonomous implementation loop (Phase 4)

**Репозиторий:** https://github.com/LarsCowe/bmalph
**Язык:** TypeScript (оркестратор) + Bash (Ralph loop)
**Звёзды:** 67 (на 2026-02-23)
**Последний коммит:** 2026-02-21
**Тип реализации:** полный CLI (`bmalph init / upgrade / doctor / status / check-updates`)

Ключевая идея: пользователь работает с BMAD агентами для планирования (фазы 1-3), затем одной командой (`/bmalph-implement`) переходит к Ralph для автономной реализации (фаза 4). Инструмент решает главную проблему стыковки двух систем — что происходит, когда планирование закончено и надо начинать кодить.

---

## 2. Архитектура: Как связаны Ralph и BMAD

### 2.1. Общая схема

```
PHASE 1-3: BMAD PLANNING
─────────────────────────────────────────────────────────────────────────
  Пользователь в чате с Claude Code

  /analyst → Create Brief (MR, DR, TR, CB)
  /pm      → Create PRD (CP, VP)
  /pm      → Create UX (CU)
  /architect → Create Architecture (CA)
  /architect → Create Epics & Stories (CE)
  /architect → Implementation Readiness (IR)

  Выход: _bmad-output/planning-artifacts/
         ├── prd.md
         ├── architecture.md
         ├── epics.md          ← сюда идут stories
         └── stories/
             ├── story-1.md
             └── story-2.md

BRIDGE: /bmalph-implement (slash command = TypeScript src/transition/)
─────────────────────────────────────────────────────────────────────────
  Читает: _bmad-output/planning-artifacts/ (stories)
  Пишет:  .ralph/@fix_plan.md        ← checkbox task list
          .ralph/specs/               ← копии спецификаций
          .ralph/SPECS_CHANGELOG.md   ← diff с прошлого запуска
          .ralph/PROJECT_CONTEXT.md   ← контекст проекта
          .ralph/@AGENT.md            ← build instructions

  Smart Merge: сохраняет [x] выполненные задачи при повторном запуске

PHASE 4: RALPH AUTONOMOUS LOOP
─────────────────────────────────────────────────────────────────────────
  $ bash .ralph/ralph_loop.sh

  Читает: .ralph/PROMPT.md      ← итерационный промпт (шаблон)
          .ralph/@fix_plan.md   ← задачи (state)
          .ralph/specs/         ← спецификации (context)
          .ralph/@AGENT.md      ← build instructions
  Пишет:  git commits (каждая история)
          .ralph/status.json    ← runtime state
          .ralph/logs/          ← логи каждой итерации
```

### 2.2. Механизм связи (src/transition/)

TypeScript-код `src/transition/` реализует трансформацию артефактов BMAD → Ralph:

1. **Чтение stories**: парсит markdown-файлы из `_bmad-output/`, извлекает задачи
2. **Генерация @fix_plan.md**: создаёт markdown с checkbox-ами (`[ ]` незавершённые, `[x]` выполненные)
3. **Smart Merge**: при повторном вызове `/bmalph-implement` — сохраняет `[x]` задачи, добавляет новые `[ ]`
4. **Копирование specs**: `_bmad-output/planning-artifacts/*.md` → `.ralph/specs/` с changelog'ом
5. **SPECS_CHANGELOG.md**: diff между текущими и предыдущими спецификациями — Ralph видит что изменилось

Это обеспечивает **инкрементальный цикл разработки**:

```
BMAD (Epic 1) → /bmalph-implement → Ralph [x] Epic 1 done
     ↓
BMAD (добавить Epic 2) → /bmalph-implement (smart merge)
     → .ralph/@fix_plan.md: [x] Epic 1, [ ] Epic 2 stories
     → .ralph/SPECS_CHANGELOG.md: "Added: epic-2.md"
     → bash .ralph/ralph_loop.sh → Ralph видит только незавершённые
```

---

## 3. Реализация Ralph Loop: технический анализ

### 3.1. Структура .ralph/ после bmalph init

```
.ralph/
├── ralph_loop.sh          # Main loop (69KB — самый большой файл!)
├── ralph_import.sh        # Импорт requirements (22KB)
├── ralph_monitor.sh       # Мониторинг прогресса (6KB)
├── .ralphrc               # Project config (env vars)
├── RALPH-REFERENCE.md     # Документация Ralph
├── PROMPT.md              # Итерационный промпт (шаблон)
├── @AGENT.md              # Build instructions (от transition)
├── @fix_plan.md           # Task list (от transition)
├── specs/                 # Скопированные из BMAD спецификации
├── logs/                  # Логи каждой итерации
├── PROJECT_CONTEXT.md     # Контекст проекта
├── SPECS_CHANGELOG.md     # Diff спецификаций
├── drivers/
│   ├── claude-code.sh     # Claude Code driver (invoke `claude`)
│   └── codex.sh           # OpenAI Codex driver (invoke `codex exec`)
└── lib/
    ├── circuit_breaker.sh    # Автомат состояний (19KB)
    ├── response_analyzer.sh  # Анализ ответов (38KB — самый сложный!)
    ├── task_sources.sh       # Источники задач (16KB)
    ├── enable_core.sh        # Инициализация (23KB)
    ├── wizard_utils.sh       # Wizard UI (15KB)
    ├── date_utils.sh         # Дата/время (3.6KB)
    └── timeout_utils.sh      # Timeout (4.2KB)
```

### 3.2. Главный цикл (ralph_loop.sh)

```bash
#!/bin/bash
set -e

# Конфигурация (из env > .ralphrc > defaults)
RALPH_DIR=".ralph"
MAX_CALLS_PER_HOUR="${MAX_CALLS_PER_HOUR:-100}"
CLAUDE_TIMEOUT_MINUTES="${CLAUDE_TIMEOUT_MINUTES:-15}"
CLAUDE_OUTPUT_FORMAT="${CLAUDE_OUTPUT_FORMAT:-json}"
PLATFORM_DRIVER="${PLATFORM_DRIVER:-claude-code}"

# Подключить библиотеки
source lib/date_utils.sh
source lib/timeout_utils.sh
source lib/response_analyzer.sh
source lib/circuit_breaker.sh

# Загрузить платформенный драйвер
load_platform_driver() {
    source "$SCRIPT_DIR/drivers/${PLATFORM_DRIVER}.sh"
    driver_valid_tools()
    CLAUDE_CODE_CMD="$(driver_cli_binary)"
}

# Основной цикл
main() {
    validate_setup()          # проверить .ralph/PROMPT.md
    init_session_tracking()   # восстановить или создать сессию

    while true; do
        should_halt_execution()     # circuit breaker check
        can_make_call()             # rate limiter (100/hour)
        should_exit_gracefully()    # completion detection
        build_loop_context()        # извлечь незавершённые задачи из @fix_plan.md
        execute_claude_code()       # запустить через platform driver
    done
}
```

**Последовательность каждой итерации:**

| Шаг | Что происходит |
|-----|----------------|
| 1 | Проверить circuit breaker (CLOSED/HALF_OPEN/OPEN) |
| 2 | Проверить rate limit (≤100 звонков/час) |
| 3 | Проверить завершение (completion_indicators + EXIT_SIGNAL) |
| 4 | Захватить `git rev-parse HEAD` (baseline для progress detection) |
| 5 | Вызвать Claude Code через драйвер |
| 6 | Проанализировать ответ (response_analyzer) |
| 7 | Обновить circuit breaker state |
| 8 | Обновить status.json |
| 9 | Проверить прогресс (файлы OR новые git commits) |

### 3.3. Platform Driver Pattern

bmalph использует **pluggable driver architecture** — любой AI CLI можно добавить как драйвер:

```bash
# drivers/claude-code.sh
driver_cli_binary() { echo "claude"; }
driver_display_name() { echo "Claude Code"; }
driver_min_version() { echo "2.0.76"; }

driver_build_command() {
    local args=(
        "--output-format" "json"           # структурированный вывод
        "--allowedTools" "$ALLOWED_TOOLS"  # whitelist инструментов
        "--resume" "$SESSION_ID"           # возобновить сессию по UUID
        # --system: loop context (номер итерации, незавершённые задачи)
    )
    echo "claude ${args[*]} < $PROMPT_FILE"
}
```

**Критическое дизайн-решение:**
bmalph **не использует** `--dangerously-skip-permissions`. Вместо этого:
- Whitelist через `--allowedTools` в `.ralphrc`
- Permission denial автоматически открывает circuit breaker → loop останавливается
- Пользователь видит сообщение: "добавьте нужный tool в ALLOWED_TOOLS"

Это делает систему **безопаснее** оригинального Ralph, но требует правильной начальной настройки.

### 3.4. Session Management

```
# Файлы состояния сессии:
.ralph/.claude_session_id      # UUID текущей сессии Claude Code
.ralph/.ralph_session          # Статус сессии (JSON)
.ralph/.ralph_session_history  # История последних 50 сессий

# Схема сессии:
{
  "session_id": "uuid-string",
  "created_at": "ISO-timestamp",
  "last_used": "ISO-timestamp",
  "reset_at": null,
  "reset_reason": null
}
```

**Ключевое решение:** `--resume <uuid>` вместо `--continue`

Почему: `--continue` возобновляет **последнюю** сессию Claude Code, что может случайно захватить сессию, открытую пользователем в другом окне. `--resume <uuid>` возобновляет только конкретную ralph-сессию.

Автоматический сброс при: открытии circuit breaker, Ctrl+C, завершении проекта, `ralph --reset-session`.
Истечение: 24 часа (настраивается через `SESSION_EXPIRY_HOURS`).

---

## 4. Circuit Breaker: Детальный разбор

### 4.1. Три состояния

```
       ┌─────────────────────────────┐
       │           CLOSED            │ ← нормальная работа
       │       (loop продолжается)   │
       └──────────────┬──────────────┘
                      │ порог нарушен
                      ▼
       ┌─────────────────────────────┐
       │            OPEN             │ ← loop ОСТАНОВЛЕН
       │  (stagnation detected)      │
       └──────────────┬──────────────┘
                      │ cooldown 30 мин
                      ▼
       ┌─────────────────────────────┐
       │          HALF_OPEN          │ ← тестирование
       │   (проверяем восстановление) │
       └──────────────┬──────────────┘
          прогресс    │    нет прогресса
          ────────────┘    ──────────────────► OPEN
          ────────────► CLOSED
```

### 4.2. Пороги открытия

| Порог | Default | Описание |
|-------|---------|----------|
| `CB_NO_PROGRESS_THRESHOLD` | 3 | Итераций без изменений файлов/git commits |
| `CB_SAME_ERROR_THRESHOLD` | 5 | Итераций с одинаковой ошибкой |
| `CB_OUTPUT_DECLINE_THRESHOLD` | 70% | Снижение объёма вывода |
| `CB_PERMISSION_DENIAL_THRESHOLD` | 2 | Permission denied от `--allowedTools` |
| `CB_COOLDOWN_MINUTES` | 30 | Минут до перехода OPEN→HALF_OPEN |

### 4.3. Детекция прогресса

**Двойная проверка** (новинка по сравнению с другими реализациями):

```bash
# До запуска Claude:
git rev-parse HEAD > .ralph/.loop_start_sha

# После запуска Claude:
# Прогресс = (файлы в git status) OR (HEAD != .loop_start_sha)
```

Это важно: если Claude написал код И сделал коммит (HEAD изменился), но git status чистый — прогресс всё равно засчитывается. Многие реализации проверяют только uncommitted changes и не видят committed работу.

### 4.4. Файл состояния

```json
// .ralph/.circuit_breaker_state
{
    "state": "CLOSED",
    "consecutive_no_progress": 0,
    "consecutive_same_error": 0,
    "consecutive_permission_denials": 0,
    "last_progress_loop": 5,
    "total_opens": 0,
    "reason": null,
    "opened_at": null,
    "current_loop": 10
}
```

---

## 5. Exit Detection: RALPH_STATUS протокол

### 5.1. Двойная верификация

Для выхода из loop необходимо ОДНОВРЕМЕННО:
- `completion_indicators ≥ 2` (Claude написал "done"/"complete" дважды)
- `EXIT_SIGNAL: true` (явный сигнал от Claude в RALPH_STATUS блоке)

Без обоих — loop продолжается. Это предотвращает ложные срабатывания от слова "complete" в середине работы.

### 5.2. RALPH_STATUS блок

Claude Code обязан включать в каждый ответ:

```
---RALPH_STATUS---
STATUS: IN_PROGRESS | COMPLETE | BLOCKED
TASKS_COMPLETED_THIS_LOOP: 2
FILES_MODIFIED: 5
TESTS_STATUS: PASSING | FAILING | NOT_RUN
WORK_TYPE: IMPLEMENTATION | TESTING | DOCUMENTATION | REFACTORING
EXIT_SIGNAL: false | true
RECOMMENDATION: Implement user auth endpoint next
---END_RALPH_STATUS---
```

**EXIT_SIGNAL: true** только когда ВСЕ условия выполнены:
- Все `@fix_plan.md` items отмечены `[x]`
- Все тесты проходят
- Нет ошибок/предупреждений
- Все требования из `specs/` реализованы

### 5.3. Response Analyzer (38KB)

Наиболее сложный компонент системы. Двухуровневая фильтрация:

**Уровень 1 — JSON-field filtering:**
Отфильтровывает `"is_error": false` (содержит слово "error", но не ошибка)

**Уровень 2 — реальная детекция ошибок:**
- Префиксы: `Error:`, `ERROR:`, `error:`, `Exception`, `Fatal`, `FATAL`
- Контекстные: `]: error`, `Link: error`, `Error occurred`, `failed with error`

**Multi-line matching:** все error-строки должны присутствовать во ВСЕХ последних history-файлах (предотвращает false negative при нескольких разных ошибках).

---

## 6. Классификация по v6-таксономии

### 6.1. Итоговая классификация

**bmalph Ralph = Type 3 + Type 4 Hybrid, с доминированием Type 4**

| Ось | Значение в bmalph | Тип (v6) |
|-----|------------------|----------|
| Session model | Multi-session fresh context + session UUID | Type 3/4 |
| Agent count | Single agent (no separate reviewer) | Type 3/4 |
| State mechanism | @fix_plan.md + JSON state files (circuit breaker + session) | **Type 4** |
| Verification | RALPH_STATUS block + EXIT_SIGNAL + completion_indicators | **Type 4** |
| Scope | PRD stories → Sprint (multi-phase: implement + test + commit) | **Type 4** |

### 6.2. Type 3 элементы

bmalph унаследовал от Type 3 (PRD-Driven Sequential):

- `@fix_plan.md` = файл с задачами в markdown-формате (как prd.json у snarktank)
- Истории берутся из BMAD-артефактов (внешний источник, не генерируются агентом)
- Последовательное выполнение story за story

### 6.3. Type 4 элементы (доминирующие)

bmalph реализует Type 4 (State-Machine Multi-Phase):

- **Circuit breaker state machine** (CLOSED/HALF_OPEN/OPEN) — центральный механизм
- **JSON state files** для circuit breaker, сессии, статуса — персистентное состояние
- **Git SHA tracking** — progress detection на основе коммитов, не только файлов
- **Rate limiter** (100 calls/hour) — управление потреблением API
- **Session UUID persistence** — контекст сохраняется между итерациями
- **Multi-threshold detection** — несколько независимых причин остановки

### 6.4. Что выходит за рамки обоих типов

bmalph добавляет функциональность, которая не вписывается в чистый Type 3 или Type 4:

- **Pluggable driver architecture** — поддержка claude-code + codex (и потенциально других)
- **Live streaming + tmux monitoring** (3-pane layout)
- **Multi-source tasks** (local / beads / github) — для enterprise-команд
- **npm CLI** (`bmalph init/upgrade/doctor`) — управление как продуктом
- **Incremental BMAD→Ralph cycles** (smart merge) — не одноразовое, а непрерывное

### 6.5. Позиция в эволюционной цепочке v6

```
Type 1 (Pure Bash, ~10 строк)
  ↓
Type 3 (PRD-Driven, ~30-80 строк)
  ↓
Type 4 (State-Machine, ~80-150 строк)
  ↓
bmalph Ralph (State-Machine Extended, ~69KB!)
   = Type 4 + driver abstraction + npm packaging + BMAD integration layer
```

69KB bash против 80-150 строк у "типичного" Type 4 — свидетельство production-grade зрелости и ориентации на широкую аудиторию (не DIY, а готовый продукт).

---

## 7. Сравнение с Clayton Farr Ralph Playbook

### 7.1. Краткий профиль Farr Playbook

Clayton Farr [github.com/ClaytonFarr/ralph-playbook] — наиболее проработанная Type 3 реализация. По нашей v6-классификации: **Type 3 с расширениями, мост к Type 4 и Type 5**.

| Компонент | Описание |
|-----------|----------|
| `loop.sh` | 3 режима: build / plan / plan-work |
| `PROMPT_build.md` | 10-step lifecycle, 999-Series Guardrails |
| `PROMPT_plan.md` | Gap analysis mode (агент анализирует, не кодит) |
| `AGENTS.md` | "сердце цикла" (~60 строк operational learnings) |
| `IMPLEMENTATION_PLAN.md` | Disposable task list (пересоздаётся agentom) |
| `specs/*.md` | Требования по темам (один файл = один topic) |
| `llm-review.ts` | LLM-as-Judge fixture (Enhancement 3) |
| 5 Enhancements | Acceptance-Driven Backpressure, LLM-as-Judge, Work Branches, User Interview, JTBD→SLC |

4 принципа: *"Context is Everything"*, *"Backpressure > Direction"*, *"Let Ralph Ralph"*, *"Move Outside the Loop"*

### 7.2. Сравнительная таблица

| Аспект | Farr Playbook | bmalph Ralph |
|--------|---------------|--------------|
| **Тип (v6)** | Type 3 + Enhancements (→Type 4/5) | **Type 3 + Type 4 Hybrid** |
| **Планирование** | LLM делает gap analysis (PROMPT_plan.md) | BMAD агенты создают артефакты, TypeScript конвертирует |
| **Task list** | IMPLEMENTATION_PLAN.md (disposable, перегенерируется) | @fix_plan.md (persistent, smart merge) |
| **Спецификации** | specs/*.md (по topics, JTBD-driven) | .ralph/specs/ (копии из BMAD) |
| **Context accumulation** | **AGENTS.md** (operational learnings per iteration) | **Отсутствует** |
| **Circuit breaker** | Нет (базовый Farr) | **Есть: CLOSED/HALF_OPEN/OPEN (19KB lib)** |
| **Session management** | `--continue` или новая сессия | **`--resume <uuid>`** (explicit session ID) |
| **Progress detection** | Нет dedicated mechanism | **Git SHA + file changes** |
| **Rate limiting** | Нет | **100 calls/hour** |
| **Review/validation** | LLM-as-Judge (Enhancement 3) | RALPH_STATUS block (self-reporting) |
| **Multi-mode loop** | plan / build / plan-work | Нет (один режим) |
| **Pluggable drivers** | Нет (claude только) | **claude-code + codex** |
| **Monitoring** | Нет | **tmux 3-pane + live streaming** |
| **Packages** | Manual install | **`npm install -g bmalph`** |
| **Permission model** | `--dangerously-skip-permissions` (типично для Type 3) | `--allowedTools` whitelist |
| **Строк bash** | ~50-100 (loop.sh) | ~69KB (ralph_loop.sh) |

### 7.3. Принципиальные различия

#### Различие 1: Кто планирует

**Farr:** Агент сам делает gap analysis (Phase 2 = `loop.sh plan`). LLM читает specs/ и src/, понимает что реализовано, что нет, генерирует план. Plan — динамический, "disposable", регенерируется при устаревании. *"Let Ralph Ralph"* — агент автономен в планировании.

**bmalph:** BMAD агенты создают план (PRD, Architecture, Epics), TypeScript переводит в @fix_plan.md. План — статический (только человек через BMAD добавляет новые stories). Smart merge сохраняет прогресс при обновлении. LLM не переосмысливает план сам.

**Вывод:** Farr = более автономный агент (планирует сам). bmalph = более управляемый (человек через BMAD контролирует план).

#### Различие 2: Накопление знаний

**Farr:** `AGENTS.md` (~60 строк) содержит operational learnings — каждая итерация добавляет знания: "не использовать relative imports", "typecheck перед commit". Эти знания читаются на каждой следующей итерации. Механизм против повторения ошибок.

**bmalph:** Аналога AGENTS.md нет. Знания живут только в session context (через `--resume`). При сбросе сессии — всё теряется. Specs/ только о требованиях, не об операционных деталях.

**Вывод:** Farr решает context rot через explicit knowledge accumulation. bmalph решает через session persistence. Оба подхода работают, но по-разному.

#### Различие 3: Backpressure реализация

Farr's *"Backpressure > Direction"*: строй среду, которая автоматически отвергает неправильный output. Реализация: тесты + typecheck + lint в шаге Validate. Если не прошли — агент видит FAIL, сам понимает что делать.

bmalph реализует backpressure другим способом:
1. `--allowedTools` whitelist → permission denial → circuit breaker open
2. RALPH_STATUS block → EXIT_SIGNAL: false → loop продолжается
3. Circuit breaker thresholds → принудительная остановка при стагнации

Оба инструментируют backpressure, но bmalph делает это на уровне инфраструктуры (loop system), Farr — на уровне агентных промптов (шаг Validate).

#### Различие 4: Scope

**Farr:** Ориентирован на greenfield проекты. 3 фазы: Requirements → Planning → Building. Нет явного multi-sprint, нет BMAD-подобной multi-agent planning.

**bmalph:** Ориентирован на полный SDLC с multi-phase planning. Поддерживает iterative cycles: BMAD (Epic 1) → implement → BMAD (Epic 2) → implement. Brownfield-friendly (явно поддерживается в README).

### 7.4. Что bmalph перенял из Farr-идей (без явного цитирования)

| Идея Farr | Реализация в bmalph |
|-----------|---------------------|
| Structured task list (IMPLEMENTATION_PLAN.md) | @fix_plan.md (checkbox markdown) |
| Fresh context per iteration | Multi-session model (default) |
| Specs as truth source | .ralph/specs/ directory |
| Commit after each story | PROMPT.md instructed (per Ralph convention) |
| *"Let Ralph Ralph"* | RALPH_STATUS block (агент сам сигнализирует завершение) |

### 7.5. Что bmalph добавляет к идеям Farr

| Дополнение bmalph | Что решает |
|-------------------|-----------|
| Circuit breaker (3 states, JSON) | Runaway loops, stagnation detection |
| Session UUID management | Hijacking active sessions, context persistence |
| Rate limiter | API cost control |
| Response analyzer (38KB) | False positive error detection |
| Permission denial as breaker trigger | Security + safe tool usage |
| Pluggable drivers | Multi-platform (claude + codex) |
| Smart merge | Incremental BMAD→Ralph cycles |
| tmux monitoring | Production observability |
| npm packaging | Distribution, upgrades, doctor |

---

## 8. Итоговые выводы

### 8.1. На вопрос А: Как работает связь Ralph-BMAD в bmalph

bmalph реализует **фазовый переход** через TypeScript CLI:

1. **BMAD Phase 1-3** генерирует planning artifacts в `_bmad-output/`
2. **`/bmalph-implement`** (TypeScript `src/transition/`) читает stories, генерирует `@fix_plan.md` + specs/
3. **Smart merge** обеспечивает инкрементальность: старые `[x]` сохраняются, новые `[ ]` добавляются
4. **SPECS_CHANGELOG.md** — Ralph видит что изменилось с прошлого запуска
5. **`bash .ralph/ralph_loop.sh`** автономно реализует stories по одной

Это не простая "склейка двух инструментов" — это **продуманный lifecycle management** с сохранением прогресса, обнаружением изменений и incremental execution.

### 8.2. На вопрос Б: Классификация Ralph и соотношение с Farr Playbook

**Классификация:** bmalph Ralph = **Type 3 + Type 4 Hybrid (v6 таксономия), с доминированием Type 4**

По ключевым осям:
- PRD-driven task list → Type 3
- Circuit breaker state machine → **Type 4**
- JSON persistent state → **Type 4**
- Session UUID management → **Type 4**
- Git SHA progress detection → **Type 4**

**Соотношение с Farr Playbook:**

bmalph и Farr решают похожие задачи (автономный AI-агент с structured state), но с разных углов:

- **Farr** = *"autonomous agent первичен, план — disposable, агент сам планирует"*. Минималистичная файловая архитектура, глубокая проработка принципов и Enhancements, AGENTS.md как накопитель знаний.

- **bmalph** = *"BMAD первично, Ralph — исполнитель"*. Production-grade infra (circuit breaker, session management, rate limiting), multi-platform, npm distribution, инкрементальные cycles.

Farr отвечает на вопрос *"как научить агента планировать самому"*. bmalph отвечает *"как safely запустить автономный loop в production-проекте"*.

Если Farr — это методология с философией, то bmalph — это infrastructure product с safety mechanisms.

---

## 9. Источники

| # | Источник | Что |
|---|----------|-----|
| 1 | github.com/LarsCowe/bmalph | Основной объект анализа |
| 2 | github.com/LarsCowe/bmalph/blob/main/README.md | README (19.8KB) |
| 3 | github.com/LarsCowe/bmalph/blob/main/ralph/RALPH-REFERENCE.md | Документация Ralph (13.3KB) |
| 4 | github.com/LarsCowe/bmalph/blob/main/ralph/ralph_loop.sh | Main loop (69KB) |
| 5 | github.com/LarsCowe/bmalph/blob/main/ralph/drivers/claude-code.sh | Claude Code driver |
| 6 | github.com/LarsCowe/bmalph/blob/main/ralph/templates/PROMPT.md | Prompt template (8.2KB) |
| 7 | github.com/LarsCowe/bmalph/blob/main/CHANGELOG.md | Version history |
| 8 | docs/research/ralph-loop-v6-taxonomy-2026-02-22.md | v6 taxonomy (наш отчёт) |
| 9 | docs/research/ralph-loop-v5-final-2026-02-22.md | v5 final (наш отчёт) |
| 10 | github.com/ClaytonFarr/ralph-playbook | Clayton Farr Playbook |
