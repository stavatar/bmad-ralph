# AgentPlane vs bmad-ralph: Сравнение и идеи для заимствования

**Дата:** 2026-03-06
**Источник:** [AgentPlane Documentation](https://agentplane.org/docs), [GitHub](https://github.com/basilisk-labs/agentplane)

---

## Ключевое различие двух подходов

| | AgentPlane | bmad-ralph |
|---|---|---|
| **Парадигма** | Пассивный — агент сам вызывает CLI изнутри сессии | Активный — оркестратор запускает агента снаружи |
| **Управление** | Policy file (AGENTS.md), агент следует правилам добровольно | Go-код формирует промпт, парсит результат, принимает решения |
| **LLM-интеграция** | Нет — не вызывает LLM, не парсит результаты | Глубокая — session.Execute, JSON parsing, cost tracking |
| **Мульти-модель** | Нет — не управляет моделями | Да — разные модели для execute/review (будущее: разные backend) |
| **Расширяемость** | Recipes (ZIP-архивы с manifest) | Go interfaces + injectable functions |

AgentPlane = "git + jira для AI-агентов". bmad-ralph = "автопилот для Claude Code".

---

## Идеи для заимствования

### Приоритет 1: Бери сейчас (без мульти-backend рефакторинга)

#### 1.1. Task lifecycle: DOING/BLOCKED статусы

**Что у AgentPlane:**
> "TODO -> DOING, TODO -> BLOCKED, DOING -> DONE, DOING -> BLOCKED, BLOCKED -> TODO, BLOCKED -> DOING"
> -- [Workflow](https://agentplane.org/docs/user/workflow)

**Проблема ralph:** задачи бинарные `- [ ]` / `- [x]`. Если Claude упал на середине —
задача остаётся `- [ ]`, ralph не знает, была ли попытка.

**Предложение:** добавить третий маркер `- [~]` (DOING) и `- [!]` (BLOCKED).
При старте задачи ralph ставит `- [~]`, при сбое — `- [!]` с причиной.
Это помогает при recovery: DOING = продолжить, BLOCKED = пропустить, TODO = начать.

**Сложность:** низкая (изменения в scan + execute loop).
**Ценность:** высокая (recovery, visibility).

#### 1.2. Task ID в коммитах

**Что у AgentPlane:**
> "commits are tied to task IDs and include changelog detail"
> -- [Design Principles](https://agentplane.org/docs/developer/design-principles)

**Предложение:** добавить инструкцию в execute prompt — коммиты с суффиксом `[TASK-N]`.
`git log --grep="TASK-3"` даст все коммиты по задаче. Без изменений в Go-коде — только prompt template.

**Сложность:** тривиальная (prompt change).
**Ценность:** средняя (traceability).

#### 1.3. Append-only verification records

**Что у AgentPlane:**
> "Verify Steps — ex-ante verification contract (criteria for verifier)"
> "Verification — ex-post append-only record from `agentplane verify`"
> -- [Task Lifecycle](https://agentplane.org/docs/user/task-lifecycle)

**Проблема ralph:** `review-findings.md` перезаписывается на каждой итерации.
История потеряна — нельзя увидеть прогресс (3H -> 1M -> 0).

**Предложение:** append-only findings log (по итерациям):
```
## Iteration 1 (2026-03-06T14:30)
3 HIGH, 2 MEDIUM findings
## Iteration 2 (2026-03-06T14:45)
0 HIGH, 1 MEDIUM findings
## Iteration 3 (2026-03-06T14:55)
Clean review
```

Помогает: (1) решение "продолжать или стоп", (2) метрики качества, (3) аудит.

**Сложность:** низкая (WriteProgress уже есть).
**Ценность:** высокая (observability, decision-making).

---

### Приоритет 2: Делать вместе с мульти-backend

#### 2.1. Execution profiles

**Что у AgentPlane:**
> "Light: Minimal enforcement, maximum flexibility"
> "Normal (default): Balanced approvals and checks"
> "Full Harness: Strict guardrails with comprehensive questionnaire"
> -- [Setup](https://agentplane.org/docs/user/setup)

**Предложение:** один ключ в конфиге определяет пакет настроек:

```yaml
execution_profile: strict  # quick | normal | strict
```

| Профиль | Gates | Review | Max iter | Модель по умолчанию |
|---------|-------|--------|----------|---------------------|
| quick | нет | нет | 1 | haiku |
| normal | да | light | 3 | sonnet |
| strict | да + emergency | full | 5 | opus |

**Сложность:** средняя.
**Ценность:** высокая (UX, onboarding).

#### 2.2. Config-driven approvals

**Что у AgentPlane:**
> `agents.approvals`: `require_plan`, `require_network`, `require_verify`, `require_force`
> -- [Configuration](https://agentplane.org/docs/user/configuration)

**Предложение:** декларативные правила вместо hardcoded gates:

```yaml
approvals:
  require_gate_before_execute: true
  require_review_after_execute: true
  auto_approve_clean_review: true    # 0 findings -> skip gate
  require_gate_before_distill: false
```

Позволяет автоматизировать happy path и ужесточить проблемные сценарии.

**Сложность:** средняя.
**Ценность:** средняя (гибкость, автоматизация).

#### 2.3. Primary tags -> pipeline routing

**Что у AgentPlane:**
> "resolves exactly one primary tag per task" -- allowlist: code, data, research, docs, ops
> "require_verification_for_primary" -- задачи с тегом `code` требуют review, `docs` -- нет
> -- [Configuration](https://agentplane.org/docs/user/configuration)

**Предложение:** тег задачи определяет pipeline:

```yaml
pipelines:
  code:
    backend: claude
    model: opus
    review: full
    gates: true
  docs:
    backend: claude
    model: sonnet
    review: none
    gates: false
  research:
    backend: codex
    model: o3
    review: light
    gates: false
```

Прямая связка с мульти-backend архитектурой: тег -> backend -> модель -> pipeline.

**Сложность:** средняя.
**Ценность:** высокая (мульти-backend, гибкость).

---

### Приоритет 3: Будущие идеи

#### 3.1. Task dependencies

**Что у AgentPlane:**
> "PLANNER creates the task, assigns ownership, and sets dependencies"
> -- [Task Lifecycle](https://agentplane.org/docs/user/task-lifecycle)

**Идея:** если задача 5 зависит от задачи 3, а задача 4 независима — ralph мог бы
параллелить выполнение (запустить 4 пока ждёт 3). Требует значительного рефакторинга
execute loop. Архитектурно заложить, реализовать позже.

**Сложность:** высокая.
**Ценность:** средняя.

#### 3.2. Per-task artifact directory

**Что у AgentPlane:**
> "Task docs live in `.agentplane/tasks/<task-id>/README.md`"
> PR artifacts: `meta.json`, `diffstat.txt`, `verify.log`, `review.md`
> -- [Architecture](https://agentplane.org/docs/developer/architecture), [Branching](https://agentplane.org/docs/user/branching-and-pr-artifacts)

**Идея:** `.ralph/tasks/TASK-42/` с артефактами:
- `prompt.md` — промпт, отправленный агенту
- `response.json` — сырой JSON-ответ
- `findings.md` — результаты review
- `metrics.json` — токены, стоимость, latency

Полная воспроизводимость и аудит. Частично пересекается с существующим session logging.

**Сложность:** средняя.
**Ценность:** средняя.

#### 3.3. Entropy management как явная фаза

**Что у AgentPlane:**
> "Routine cleanup, early drift detection, and synchronized docs prevent repository chaos from agent output."
> -- [Design Principles](https://agentplane.org/docs/developer/design-principles)

**Идея:** после execute, перед commit — фаза cleanup:
- Удаление артефактов, не относящихся к задаче
- Проверка на drift (изменения вне scope задачи)
- Обновление docs, если код изменился

**Сложность:** высокая (scope detection непросто).
**Ценность:** средняя.

---

### Не подходит для ralph (и почему)

| Идея AgentPlane | Почему не подходит |
|---|---|
| AGENTS.md как policy file | Ralph сам формирует промпт — policy в Go-коде, не в markdown |
| 9 ролей агентов (ORCHESTRATOR, PLANNER...) | Оркестрация в Go-коде, не в промптах для LLM |
| Recipe system (ZIP + manifest) | Go interfaces мощнее и типобезопаснее |
| Offline-first network approvals | Ralph уже offline (LLM — единственная сеть) |
| Status commit policy (off/warn/confirm) | Ralph управляет git снаружи, не через policy |

---

## Связь с мульти-backend архитектурой

Идеи 2.1-2.3 (profiles, config-driven approvals, primary tags) образуют **единую систему** с мульти-backend:

```
Задача с тегом "code"
  -> pipeline "code" (из конфига)
    -> backend: claude, model: opus
    -> review: full (backend: claude, model: sonnet)
    -> gates: require_before_execute + auto_approve_clean
    -> execution_profile: strict (max_iter=5)
```

Это переход от "один pipeline для всех" к "routing по типу задачи" — значительно более
гибкая архитектура, которую AgentPlane реализует через primary tags, а ralph может
реализовать через конфиг pipelines.
