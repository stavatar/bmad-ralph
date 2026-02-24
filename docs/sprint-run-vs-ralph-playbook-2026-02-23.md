# /sprint-run vs Farr Playbook vs bmalph: трёхстороннее сравнение

> **Дата:** 2026-02-23 (v2 — с учётом BMAD-pipeline и bmalph)
> **Контекст:** Полный цикл проекта = `/create-sprint` (BMAD planning) + `/sprint-run` (execution).
> Источники: реальные SKILL.md скиллов + `ralph-loop-bmalph-analysis-2026-02-23.md`

---

## Ключевой инсайт: три системы делают одно и то же

Все три системы реализуют **один и тот же полный SDLC-цикл**:

```
BMAD-планирование → конвертация артефактов → автономная реализация → качество → коммит
```

Разница — в степени автономии, жёсткости gating'а и философии обнаружения ошибок.

---

## Полный pipeline — все три системы рядом

```
ЭТАП               /create-sprint + /sprint-run    bmalph                       Farr Playbook
═══════════════════════════════════════════════════════════════════════════════════════════════

PLANNING
───────────────────────────────────────────────────────────────────────────────────────────────
Phase 0            /analyst brainstorm              ✗ нет                        ✗ нет
Discovery          /analyst research
                   /analyst product-brief

Phase 1-2          /pm → PRD + UX                   /pm → PRD + UX               specs/*.md
Требования         /architect → Architecture         /architect → Architecture     (один файл = одна capability)
                   docs/prd.md, ux-design.md         _bmad-output/prd.md

Phase 3            /create-sprint:                   /architect → Epics & Stories  loop.sh plan:
Epics & Stories    1. create-epics-and-stories        _bmad-output/stories/*.md    агент сам делает gap analysis
                   2. implementation-readiness                                      → IMPLEMENTATION_PLAN.md
                   3. sprint-planning
                   4. create-story × N
                   → docs/sprint-artifacts/stories/{id}.md (rich-context)

BRIDGE
───────────────────────────────────────────────────────────────────────────────────────────────
Трансформация      /sprint-run читает               /bmalph-implement             IMPLEMENTATION_PLAN.md
артефактов         sprint-status.yaml +              TypeScript src/transition/:   уже готов (агент создал)
                   stories/{id}.md                   stories → @fix_plan.md
                   (📄 rich-context mode)             specs/ копируются
                                                      SPECS_CHANGELOG.md
                                                      Smart Merge: [x] сохраняются

EXECUTION
───────────────────────────────────────────────────────────────────────────────────────────────
Pre-phase setup    /sprint-preflight:                ✗ нет отдельного preflight   ✗ нет
                   Gate 1: /continue
                   Gate 2: /reindex (Serena)
                   Gate 3: git checkout -b
                   Gate 4: TDD failing tests (RED)

Реализация         Sub-agents:                       ralph_loop.sh:               loop.sh build:
                   backend-dev / frontend-dev          claude subprocess            claude subprocess
                   per-story (batch by phase)          --max-turns 15min            --max-turns N (50-100)
                   📄 rich-context промпт              --resume <uuid>              fresh context per iteration
                                                        RALPH_STATUS block          AGENTS.md per iteration

Iteration state    review-state.json                 .ralph/status.json            IMPLEMENTATION_PLAN.md
(между итерациями) sprint-status.yaml                .ralph/.circuit_breaker_state AGENTS.md
                   MEMORY.md                         .ralph/.claude_session_id

Memory             /compact между фазами             --resume <uuid>               Fresh context
mechanism          → review-state.json сохраняет     Session context накапливается + AGENTS.md (60 строк)
                   последний шаг                     SESSION_EXPIRY_HOURS=24

Stagnation guard   3 попытки → СТОП                  Circuit breaker:              ✗ нет механизма
                   (спросить пользователя)            CLOSED→OPEN→HALF_OPEN
                                                      CB_NO_PROGRESS_THRESHOLD=3

QUALITY GATES
───────────────────────────────────────────────────────────────────────────────────────────────
Phase gate         node test-summary.js              RALPH_STATUS: TESTS_STATUS   Guardrail 999:
(per phase/iter)   0 failures → review               EXIT_SIGNAL: false           «required tests must pass»
                   иначе СТОП                        → loop продолжается          до коммита

TDD                Gate 4: тесты ПАДАЮТ до кода      ✗ тесты пишутся вместе       ✗ тесты пишутся вместе
                   (red phase, hard gate)             с кодом (~20% усилий)        с кодом
                                                      backpressure, не TDD         backpressure, не TDD

Review             /sprint-review:                    Self-reporting               Self-reporting
(что проверяет)    2 параллельных агента:             RALPH_STATUS block           failing tests + typecheck
                   • AC reviewer (haiku)              ✗ нет внешнего ревьюера      Enhancement 3: LLM-as-Judge
                   • Code quality (sonnet)                                          (опционально)
                   findings confidence ≥ 80

E2E                АБСОЛЮТНЫЙ ПРИОРИТЕТ:              PROMPT.md: «тесты ~20%»      PROMPT_build.md: «run tests
                   Docker down → СТОП                 ✗ нет hard requirement       for improved code»
                   localhost перед каждым коммитом    ✗ нет E2E enforcement        ✗ нет hard requirement

Pre-commit gate    tests + typecheck + lint (HARD)    RALPH_STATUS: всё PASSING    Guardrail 999 (soft)
                   ВСЕ = 0 failures                   EXIT_SIGNAL: true            зависит от LLM

FINAL
───────────────────────────────────────────────────────────────────────────────────────────────
Финальное ревью    /sprint-finish:                    ✗ нет финального ревью       ✗ нет финального ревью
                   6 PR-агентов параллельно:          loop просто останавливается  loop останавливается
                   code-reviewer, simplifier,         по EXIT_SIGNAL: true         по completion_indicators
                   comment-analyzer, test-analyzer,
                   silent-failure-hunter,
                   type-design-analyzer

Knowledge          /skill-propagator:                 --resume: session context    AGENTS.md:
extraction         новые SKILL.md файлы               ✗ нет систематического       операционные знания
                   MEMORY.md обновление               извлечения в файлы           ~60 строк, вручную
                   CLAUDE.md реестр
                   Sprint history

Human control      STOP: пользователь смотрит diff    ✗ полностью автономно        «Move Outside the Loop»
                   и решает мёржить                   до EXIT_SIGNAL               пользователь наблюдает
                                                                                    за паттернами
```

---

## Три принципиальных отличия

### 1. TDD — строгий vs backpressure

**sprint-run:** Тесты написаны и ПАДАЮТ до первой строчки реализации. Gate 4 не пройден без реально красных тестов. Фаза исполнения = green phase по определению.

**bmalph + Farr:** Тесты **известны заранее** (выведены из AC на этапе planning), но **написаны одновременно с кодом**. Нет red phase. Коммит блокируется без них (Guardrail 999 / RALPH_STATUS: FAILING), но это не TDD — это test-verified completion.

```
sprint-run:  Gate 4 red → [исполнение] → green ← тест был написан ДО кода
bmalph/Farr: plan (AC→required_tests) → [исполнение + написание тестов вместе] → коммит
```

### 2. Степень автономии

**sprint-run** — supervised automation:
- Execution plan показывается пользователю, ждёт подтверждения
- После каждой фазы: sprint-review показывает findings → человек читает
- После всего: sprint-finish → **СТОП, пользователь мёржит**
- 3 попытки → **СТОП, спросить пользователя**

**bmalph** — autonomous loop with safety:
- Запускается один раз: `bash .ralph/ralph_loop.sh`
- Работает до EXIT_SIGNAL: true — человек не участвует между итерациями
- Circuit breaker останавливает при стагнации (не человек)
- Человек включается только при OPEN circuit breaker

**Farr** — максимальная автономия:
- «Let Ralph Ralph» — агент сам планирует и реализует
- Нет circuit breaker (в базовой версии)
- Человек: «Move Outside the Loop» — наблюдает за паттернами

### 3. Rich context vs минимальный контекст

**sprint-run** с rich-context режимом: каждая story имеет файл `docs/sprint-artifacts/stories/{id}.md` с техническим контекстом, детальными AC, implementation notes, зависимостями. Sub-agent получает это всё целиком в промпте.

**bmalph:** BMAD создаёт story-файлы в `_bmad-output/stories/`. `/bmalph-implement` копирует их в `.ralph/specs/`. Ralph читает specs/ как reference, но `@fix_plan.md` — это checkbox-список, не детальный контекст.

**Farr:** `specs/*.md` — один файл на capability. Нет story-файлов с техническим контекстом. Агент сам делает gap analysis (`loop.sh plan`) и решает как реализовать.

---

## Соответствие конкретных компонентов

| Компонент sprint-run | bmalph | Farr Playbook |
|---|---|---|
| `/create-sprint` → `create-epics-and-stories` | `/analyst` + `/pm` + `/architect` → `_bmad-output/` | `specs/*.md` (вручную или через диалог) |
| `create-story × N` → `stories/{id}.md` | `_bmad-output/stories/*.md` (BMAD создаёт) | ✗ нет story-файлов |
| `sprint-status.yaml` (трекинг) | `@fix_plan.md` (checkbox list) | `IMPLEMENTATION_PLAN.md` (disposable) |
| `/sprint-run` анализ → execution plan | ✗ нет (агент сам читает @fix_plan.md) | ✗ нет (агент сам из IMPLEMENTATION_PLAN.md) |
| `Gate 1` `/continue` + MEMORY.md | `--resume <uuid>` (session memory) | `AGENTS.md` (per-iteration read) |
| `Gate 2` `/reindex` Serena | ✗ нет | ✗ нет |
| `Gate 3` `git checkout -b` | Enhancement 4: Work Branches (опц.) | Enhancement 4: Work Branches (опц.) |
| `Gate 4` TDD failing tests | ✗ нет red phase | ✗ нет red phase |
| Sub-agent `backend-dev` / `frontend-dev` | `claude --resume <uuid>` subprocess | `claude -p PROMPT_build.md` subprocess |
| `node test-summary.js` phase gate | `RALPH_STATUS: TESTS_STATUS` | `npm test + typecheck` (Validate step) |
| `review-state.json` | `.ralph/status.json` | `IMPLEMENTATION_PLAN.md` |
| AC reviewer (haiku) | ✗ нет | ✗ нет (Enhancement 3 LLM-as-Judge — опц.) |
| Code quality reviewer (sonnet) | ✗ нет | ✗ нет |
| `/commit-commands:commit` | `git commit` (per iteration) | `git commit` (rule 4, when tests pass) |
| `/sprint-finish` PR toolkit × 6 | ✗ нет | ✗ нет |
| `/skill-propagator` | ✗ нет систематического | `AGENTS.md` (вручную, ~60 строк) |
| `MEMORY.md` | ✗ нет | ✗ нет |
| `/compact` между фазами | `SESSION_EXPIRY_HOURS=24` | Fresh context per iteration |

---

## Что лучше в каждой системе

### sprint-run лучше:
- **Строгий TDD** — Gate 4 создаёт реально падающие тесты до реализации
- **Независимый review** — два отдельных агента (реализатор ≠ ревьюер)
- **E2E как hard gate** — Docker down = СТОП, нельзя обойти
- **Rich story context** — sub-agents получают детальный tech context, AC, зависимости
- **6 PR-агентов в финале** — pr-review-toolkit охватывает code/comments/tests/types/silent-failures/simplicity
- **Систематическое извлечение знаний** — skill-propagator создаёт SKILL.md файлы, обновляет CLAUDE.md
- **Compaction recovery** — review-state.json сохраняет конкретный шаг, не теряет прогресс

### bmalph лучше:
- **Полная автономия** — запустил и не трогаешь. Circuit breaker останавливает при проблемах
- **Инкрементальные BMAD-циклы** — Smart merge: добавил Epic 2 → `/bmalph-implement` → `[x] Epic 1` сохраняется
- **Структурный финальный шаг** — RALPH_STATUS обязателен в каждом ответе (hard, не soft)
- **Production-grade safety** — rate limiter (100 calls/hr), session UUID, response analyzer (38KB)
- **Multi-platform** — драйверы для claude-code + codex

### Farr Playbook лучше:
- **Минимализм** — `loop.sh` ~100 строк vs sprint-run ~5 скиллов + sub-agents + state files
- **Агент планирует сам** — `loop.sh plan` делает gap analysis. sprint-run требует /create-sprint вручную
- **AGENTS.md как живая память** — накапливается итерационно внутри одного проекта
- **«Let Ralph Ralph»** — агент находит баги вне scope текущей story

---

## Итоговая позиция каждой системы

```
Максимальная     ←────────────────────────────────────→     Максимальный
автономия                                                      контроль
│                                                                       │
│  Farr        bmalph                      sprint-run                  │
│  (агент      (автономный loop            (supervised:                 │
│   планирует   + circuit breaker           человек одобряет            │
│   сам,        + BMAD artifacts,           каждую фазу,                │
│   ~100        но без человека             6 PR-агентов,               │
│   строк)      между итерациями)           skill-propagator)           │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘

Меньше        ←──────────────────────────────────→       Больше
overhead                                                   качество
│                                                                 │
│  Farr        bmalph              sprint-run                    │
│  (soft        (hard RALPH_STATUS  (hard gates:                  │
│   guardrails  circuit breaker     TDD + E2E +                   │
│   промпт)     rate limiter)       reviewers +                   │
│                                   PR toolkit)                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Гипотетический гибрид

Если взять лучшее из всех трёх:

```
PLANNING (как есть):
  /create-sprint → BMAD phases 0-3 → rich story context files ✅

BRIDGE (добавить из bmalph):
  Smart Merge при повторном /create-sprint → не терять [x] выполненные stories
  SPECS_CHANGELOG.md → sub-agents видят что изменилось с прошлого спринта

PREFLIGHT (как есть + улучшение):
  Gate 4 TDD → добавить required_tests в sprint-status.yaml (трассировка AC→тест) ✅
  ← из Farr Enhancement 2: Acceptance-Driven Backpressure

EXECUTION (как есть + улучшение из bmalph):
  Sub-agents как сейчас (rich-context) ✅
  + story-level mini-loop: 3 попытки → не СТОП, а cooldown → retry
  ← из bmalph circuit breaker

REVIEW (как есть):
  2 reviewer агента + E2E + pre-commit gate ✅

MEMORY (добавить из Farr):
  AGENTS.md per-sprint: операционные знания накапливаются внутри спринта
  ← то что сейчас идёт в MEMORY.md, но структурированнее

FINISH (как есть):
  6 PR-агентов + skill-propagator ✅
```

---

## Источники

| Источник | Что |
|---|---|
| `.claude/skills/create-sprint/SKILL.md` | BMAD → sprint tracking → rich-context story files |
| `.claude/skills/sprint-run/SKILL.md` | Orchestrator: preflight → execution → review → finish |
| `.claude/skills/sprint-preflight/SKILL.md` | TDD Gates 1-4 |
| `.claude/skills/sprint-review/SKILL.md` | Reviewers + E2E + pre-commit |
| `.claude/skills/sprint-finish/SKILL.md` | PR toolkit × 6 + skill-propagator |
| `ralph-loop-bmalph-analysis-2026-02-23.md` | bmalph: архитектура, circuit breaker, BMAD интеграция |
| `ralph-loop-teacher-qa-2026-02-23.md` | Q&A: turns, AGENTS.md, IMPLEMENTATION_PLAN.md, коммиты |
| `ralph-tdd-concerns-2026-02-23.md` | TDD vs acceptance-driven backpressure |
