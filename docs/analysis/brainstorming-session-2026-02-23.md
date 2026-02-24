---
stepsCompleted: [1]
inputDocuments:
  - mentorlearnplatform/.claude/skills/sprint-run/SKILL.md
  - mentorlearnplatform/docs/research/intermediate/ralph-bmad/version2.md
  - mentorlearnplatform/docs/research/sprint-run-vs-ralph-playbook-2026-02-23.md
  - mentorlearnplatform/docs/research/ralph-loop-teacher-qa-2026-02-23.md
  - mentorlearnplatform/docs/research/ralph-loop-bmalph-analysis-2026-02-23.md
session_topic: 'Гибрид BMad + Ralph цикл: собственная реализация'
session_goals: 'Определить архитектуру гибридного подхода BMad planning + Ralph execution'
selected_approach: 'interview'
techniques_used: [interview, comparative-analysis]
ideas_generated: [hybrid-architecture, resume-extraction, distillation-pattern, md-state-format]
context_file: '.bmad/bmm/data/project-context-template.md'
---

# Brainstorming Session Results

**Facilitator:** Степан
**Date:** 2026-02-23

---

## Executive Summary

Сессия определила архитектуру гибрида **BMad Method (phases 1-3) + Ralph Loop (phase 4)** для замены текущей skill-based оркестрации sprint-run (~641 LOC, 4 скилла). Цель: масштабируемость, минимальный контекст, итеративность, сохранение лучших практик из обоих подходов.

Проанализированы две эталонные реализации:
- **bmalph** (LarsCowe) — TypeScript CLI, BMad → Ralph bridge, circuit breaker, RALPH_STATUS протокол
- **Clayton Farr Ralph Playbook** — минималистичный loop.sh, AGENTS.md, 999-Series Guardrails, 5 Enhancements

---

## Принятые решения

### 1. Общая архитектура

| Слой | Решение |
|---|---|
| Planning (phases 1-3) | **BMad as-is** — PRD, Architecture, Stories, AC |
| Bridge (BMad → Ralph) | **Код-конвертер** (bmalph-подход) — детерминированный, Smart Merge, + служебные задачи |
| Execution (phase 4) | **Ralph Loop** — bash loop, fresh context per iteration, agent decides task order |
| Review (MVP) | **Task tool sub-agents** — параллельные ревьюеры внутри итерации |
| Review (Production) | **Гибрид A+C** — sub-agents in-loop + тест-фикстуры через `claude -p` для CI quality gate |
| Knowledge extraction | **Farr-стиль** непрерывная запись + meta-task дистилляция + финальная задача спринта |

### 2. Формат хранения состояния

**Только Markdown** для задач (как у bmalph и Farr):

| Файл | Формат | Кто читает | Для чего |
|---|---|---|---|
| `sprint-tasks.md` | MD | Агент (Read tool) + скрипт (grep) | Список задач `[ ]`/`[x]`, human gates |
| `AGENTS.md` | MD | Агент | Операционные знания (~60 строк, строго кратко) |
| `LEARNINGS.md` | MD | Агент + meta-task | Находки, issues, дистилляция паттернов |

JSON — только если понадобится для runtime-метаданных скрипта (circuit breaker в post-MVP).

### 3. Как агент получает задачи

**Farr-подход**: агент сам читает `sprint-tasks.md` целиком через Read tool (видит и `[x]`, и `[ ]` — понимает общий контекст). Скрипт не извлекает задачи в промпт.

### 4. Knowledge extraction — трёхуровневая система

| Уровень | Что | Когда |
|---|---|---|
| **Непрерывная запись** (Farr-стиль) | Агент пишет в AGENTS.md + LEARNINGS.md по ходу каждой итерации | Каждая итерация |
| **Meta-task дистилляция** | Читает AGENTS.md + LEARNINGS.md → сжимает в паттерны | Каждые N итераций |
| **Финальная задача** | Анализ паттернов → решение о создании кастомных скиллов | Конец спринта |

**Дистилляция вместо обрезки**: сырые записи сжимаются в паттерны ("bcrypt несовместим с Node 22 → всегда argon2"), ничего не теряется.

### 5. Resume-extraction при неуспехе

При неуспешной итерации (нет коммита / тесты красные) — `--resume` той же сессии с промптом на извлечение знаний (2-3 turns). Агент помнит весь контекст и может объяснить ПОЧЕМУ не получилось.

- По умолчанию: **только при неуспехе**
- Флаг `--always-extract`: запускать после каждой итерации

### 6. Human Gates

Свойство любой задачи в `sprint-tasks.md`. Если задача помечена как human_gate — loop останавливается и ждёт решения пользователя.

**Точки gate'ов (user-visible milestones):**
- После bridge — review sprint-tasks.md
- Первая задача epic'а — верификация направления
- После задач с usable UI (новая функция/экран/кнопка)
- Перед merge — финальное подтверждение

**Human gate prompt:**
- `[Enter]` approve — продолжить
- `[r]` retry — feedback → fix-задача вперёд (MVP) | rollback (post-MVP)
- `[c]` correct — Claude+BMad правит story → re-bridge (MVP last epics)
- `[s]` skip — пропустить задачу
- `[q]` quit — остановить loop

**Retry flow (MVP):** всегда спрашивает feedback → создаёт fix-задачу вперёд.
**Retry rollback:** post-MVP (git reset + повтор задачи с feedback).

**Correct flow:** скрипт определяет source story из sprint-tasks.md → запускает Claude+BMad analyst с контекстом (какая story, что сделано, что не понравилось) → после выхода автоматический re-bridge (Smart Merge).

### 7. Circuit Breaker

**Post-MVP.** В MVP — простой `MAX_ITER`. Позже — bmalph-стиль (CLOSED/HALF_OPEN/OPEN, пороги, cooldown).

### 8. Мост BMad → Ralph

**Код-конвертер** (bmalph-подход): детерминированный скрипт читает BMad story-файлы → генерирует `sprint-tasks.md` + добавляет служебные задачи (reindex, review, meta-analysis, finish). Smart Merge при повторном запуске. Конкретный алгоритм — отдельная задача.

### 9. LLM-as-Judge

**MVP:** Вариант A — Task tool sub-agents внутри Ralph-итерации. Параллельный запуск 2-5 ревьюеров (состав — Research 4). Минимальная реализация (~20-50 LOC промпт). Самый распространённый паттерн в сообществе (HAMY: 9 агентов, ralphex: 5 агентов).

**Production:** Гибрид A+C:
- **A** — быстрое in-loop ревью sub-agents после каждой задачи
- **C** — тест-фикстура `createReview()` вызывает `claude -p --output-format json` как subprocess, binary pass/fail в vitest. Формальный quality gate в CI.

**Без внешних API** — все ревью через Claude Code подписку. Подход `claude -p` с очисткой `ANTHROPIC_API_KEY` проверен на практике (Voicetest).

**Исследование:** см. `docs/research/llm-as-judge-variants-research-2026-02-24.md`

---

## Backlog: Deep Research

Вопросы, требующие отдельного исследования перед реализацией:

### Research 1: Serena + Ralph
Как интегрировать Serena с философией Ralph цикла: паттерны reindexing, оптимальная частота, подходы из community. Агент активно использует Serena для экономии токенов — индекс должен быть актуальным.

### Research 2: TDD vs Pre-written Test Cases
Как стыкуются TDD и Ralph цикл. Имеет ли смысл отдельные stories на написание тестов, или достаточно детально расписать test-кейсы и AC в story-файлах.

### Research 3: E2E тесты в Ralph цикле
Насколько активно использовать browser E2E: частота запуска, стоимость vs ценность, когда запускать в рамках цикла.

### Research 4: Состав review-агентов
Определить набор параллельных sub-агентов для review: что взять из PR Toolkit, что из ralphex подхода, возможно универсальный набор.

### Research 5: Конкретные Human Gate точки
Определить дефолтные задачи с human_gate (merge — точно, остальные — решить).

---

## Backlog: Design & Implementation

### Design 1: Алгоритм код-конвертера
Конкретный алгоритм трансформации BMad stories → sprint-tasks.md + служебные задачи.

### Design 2: PROMPT_build.md
Основной промпт для итерации: инструкции агенту, guardrails, правила записи в AGENTS.md / LEARNINGS.md.

### Design 3: PROMPT_extract.md
Промпт для resume-extraction: что извлекать, куда писать, формат.

### Design 4: loop.sh
Структура bash-скрипта: main loop, completion check, human gate detection, resume-extraction trigger, MAX_ITER.

### Design 5: Meta-task промпт и частота
Как часто запускать дистилляцию, промпт для анализа LEARNINGS.md → паттерны.

---

## Ключевые источники

| Источник | Роль в решениях |
|---|---|
| Clayton Farr Ralph Playbook | AGENTS.md, LEARNINGS модель, агент читает задачи сам, 999-Series Guardrails |
| bmalph (LarsCowe) | Код-конвертер, Smart Merge, circuit breaker (post-MVP), RALPH_STATUS |
| sprint-run (MentorLearnPlatform) | Исходная система, E2E правила, typed agents, rich-context stories |
| ralph-bmad research v2 | 3-Layer Autonomous Stack, "BMad thinks, Ralph executes", Decision Matrix |
| ralph-loop-teacher-qa | Механика turns, AGENTS.md bloat protection, финальный шаг проблема |

---

## Что НЕ вошло в гибрид (осознанно)

| Компонент | Почему убрано |
|---|---|
| Skill-based оркестрация (sprint-run, sprint-preflight, sprint-review, sprint-finish) | Не масштабируется, жёсткая структура, контекст переполняется |
| --resume для основной работы (bmalph) | Против философии Ralph (fresh context). Resume только для extraction |
| Disposable plan (Farr loop.sh plan) | Мозг = BMad, не агент. Story-файлы не disposable |
| TDD Gate 4 (sprint-preflight) | Под вопросом — Research 2 |
| 6 PR-агентов в финале (sprint-finish) | Заменяется на параллельных review sub-агентов — Research 4 |
| Compaction recovery | Не нужна — fresh context per iteration |
