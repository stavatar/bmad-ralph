# V2 Prompt Chain Design: plan.md + replan.md

Дата: 2026-03-07

Проектирование промпт-цепочки для команд `ralph plan` и `ralph replan`.

---

## Содержание

1. [Исследование и обоснование](#1-исследование-и-обоснование)
2. [Single-pass vs Multi-pass анализ](#2-single-pass-vs-multi-pass-анализ)
3. [JSON Schema для output](#3-json-schema-для-output)
4. [Переиспользование из bridge.md](#4-переиспользование-из-bridgemd)
5. [Draft промпта plan.md](#5-draft-промпта-planmd)
6. [Draft промпта replan.md](#6-draft-промпта-replanmd)
7. [Источники](#7-источники)

---

## 1. Исследование и обоснование

### 1.1 Промптовые стратегии декомпозиции в 2025-2026

Ключевые находки из исследования:

**Skeleton-of-Thought (SoT)** — двухфазный подход: сначала генерируется скелет (outline), затем детали расширяются параллельно. Снижает latency, улучшает качество. Прямо применим к `ralph plan`: сначала анализ требований (скелет epics), затем декомпозиция в задачи.

**Plan-and-Solve (PS)** — вводит промежуточную фазу планирования перед решением. Модель НЕ пропускает критические шаги. Для ralph: поле `analysis` в JSON output играет роль промежуточной фазы reasoning перед генерацией задач.

**Blueprint First, Model Second** (2025) — декомпозиция workflow на детерминистические шаги + LLM ТОЛЬКО для bounded sub-tasks. +10.1 п.п. на tau-bench, -81.8% tool calls. Это ТОЧНО наш подход: Go = детерминистический scaffold, LLM = семантический анализ.

**ADaPT** (Allen AI, 2024) — as-needed decomposition превосходит upfront planning на 28-33%. Но для ralph нужен persistent plan (изолированные сессии), поэтому ADaPT применяется к replan, а не к plan.

**Исследование длины промпта (2025)**: reasoning деградирует после ~3000 tokens. Практический sweet spot: 150-300 слов инструкций. Текущий bridge.md (244 строки) значительно превышает этот порог. Целевой plan.md: ~100 строк (~250 слов инструкций).

### 1.2 Structured Output: техники для Claude

**Anthropic Structured Outputs API** — constrained decoding на уровне API через `output_config.format`. 100% schema compliance, zero retries. НО: ralph использует Claude CLI (не API), поэтому API structured outputs недоступны.

**Reasoning + JSON паттерн** — два подхода:
1. Поле `"analysis"` в начале JSON как CoT-area внутри structured output
2. Два вызова: reasoning → structured output (дороже, но чище)

Google Cloud research подтверждает: constrained decoding может "derail" reasoning. Решение: CoT в промежуточных шагах, structured output в финальном.

**Для ralph (CLI)**: JSON schema описана в промпте + Go-валидация через `json.Unmarshal`. Поле `analysis` служит CoT-зоной, не ограничивающей reasoning. При невалидном JSON — retry с error feedback.

### 1.3 Devin: подход к планированию

Из утечки системного промпта Devin (апрель 2025, 400+ строк):
- Три режима: planning, standard, edit
- Planning mode: собирает информацию, НЕ выполняет
- Subtask management: определи SUBTASKS, помечай STATUS, сообщай на checkpoints
- Full tool-calling JSON API с Linux shell, browser, filesystem

Ключевая разница с ralph: Devin работает в единой сессии (plan + execute), ralph требует persistent файл для изолированных сессий. Но идея "modes" (plan vs execute) та же.

### 1.4 MetaGPT: SOP-driven декомпозиция

MetaGPT кодирует Standard Operating Procedures (SOPs) в промпт-последовательности:
- Product Manager agent: PRD → structured task list
- SOPs — программный enforcement через structured outputs, не markdown
- Publish-subscribe коммуникация через global message pool

Для ralph: подход MetaGPT с SOP-кодированием в промпте — ЭТО то, что делает bridge.md. Наша эволюция: перенести SOP enforcement из промпта (хрупко) в Go-код (детерминистично).

---

## 2. Single-pass vs Multi-pass анализ

### Три варианта

| Вариант | Описание | Latency | Качество | Стоимость |
|---------|----------|---------|----------|-----------|
| **A: Single-pass** | Один LLM-вызов: анализ + генерация JSON | 1x | Хорошее для <10 требований | 1x |
| **B: Two-step** | LLM1: classify ACs → LLM2: generate tasks | 2x | Лучше для 10-20 требований | 1.5-2x |
| **C: CoT-in-JSON** | Один вызов, поле `analysis` как CoT-зона | 1x | Как B, стоимость как A | 1x + ~200 tokens |

### Рекомендация: Вариант C (CoT-in-JSON, single-pass)

**Обоснование:**

1. **Latency = 1 вызов**: ralph plan — интерактивная команда, пользователь ждёт результат. Два вызова удваивают время ожидания без пропорционального прироста качества.

2. **CoT без ограничения reasoning**: поле `analysis` в JSON позволяет LLM "подумать" перед генерацией задач. Google Cloud research показывает, что reasoning В СОСТАВЕ structured output (как первое поле) не страдает от constrained decoding, потому что к моменту reasoning ограничений на значение поля нет.

3. **Стоимость**: ~200 extra tokens на `analysis` vs удвоение полного контекста при two-step.

4. **Исследования подтверждают**: "reasoning first, then structured answer" — рекомендация OpenAI и подтверждена на практике (Plan-and-Solve, Skeleton-of-Thought).

5. **Для complex PRD (20+ требований)**: Go-код может split по секциям PRD и сделать несколько single-pass вызовов. Это программная декомпозиция (Blueprint First), а не промптовая.

**Исключение для two-step**: если JSON parsing fails (невалидный JSON), retry с error feedback — фактически two-step, но только в fallback path.

---

## 3. JSON Schema для output

### 3.1 Schema определение

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["analysis", "epics"],
  "additionalProperties": false,
  "properties": {
    "analysis": {
      "type": "string",
      "description": "1-3 предложения: краткий анализ требований, ключевые решения по группировке"
    },
    "epics": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "tasks"],
        "additionalProperties": false,
        "properties": {
          "name": {
            "type": "string",
            "description": "Название epic-группы (напр. 'Authentication', 'API Endpoints')"
          },
          "tasks": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["title", "requirement_refs"],
              "additionalProperties": false,
              "properties": {
                "title": {
                  "type": "string",
                  "description": "Описание задачи в императивной форме, до 500 символов",
                  "maxLength": 500
                },
                "test_scenarios": {
                  "type": "array",
                  "items": {"type": "string"},
                  "description": "Ключевые тест-сценарии (пустой массив если инфра)"
                },
                "requirement_refs": {
                  "type": "array",
                  "items": {"type": "string"},
                  "description": "Ссылки на требования: AC-1, FR-3, REQ-5 и т.д."
                },
                "depends_on": {
                  "type": "array",
                  "items": {"type": "string"},
                  "description": "Titles задач-зависимостей (пустой массив если нет)"
                },
                "tags": {
                  "type": "array",
                  "items": {
                    "type": "string",
                    "enum": ["SETUP", "E2E", "GATE"]
                  },
                  "description": "Сервисные теги задачи"
                },
                "size": {
                  "type": "string",
                  "enum": ["S", "M", "L"],
                  "description": "Оценка размера: S(<1ч), M(1-3ч), L(3-8ч)"
                }
              }
            }
          }
        }
      }
    }
  }
}
```

### 3.2 Что Go добавляет программно (НЕ LLM)

| Поле в sprint-tasks.md | Источник | Как |
|------------------------|----------|-----|
| `- [ ]` / `- [x]` | Go | Все новые = `[ ]`, existing сохраняются |
| `source:` | Go | Из `requirement_refs` + имя входного файла |
| `[GATE]` (первый в epic) | Go | Первая задача каждого epic |
| `[GATE]` (deploy/security) | Go | Keyword scan по title |
| `[SETUP]` / `[E2E]` | Go + LLM | LLM ставит tag, Go форматирует |
| Ordering | Go + LLM | LLM задаёт `depends_on`, Go делает topological sort |
| Merge | Go | Детерминистический diff по title+refs |

### 3.3 Почему `tags` вместо `gate: bool`

В ранних draft'ах (variant-d-prompt-architecture.md) gate/setup/e2e были отдельными boolean полями. Теперь — массив `tags`:
- **Extensibility**: добавление нового типа задачи — добавление enum value, без изменения schema
- **GATE управляется Go**: LLM НЕ решает где ставить GATE (первый в epic = программно). Но LLM МОЖЕТ пометить deploy/security задачу тегом GATE
- **Compact**: один массив вместо трёх boolean полей

---

## 4. Переиспользование из bridge.md

### Что берём (ценное ядро)

| Секция bridge.md | Строки | Действие в plan.md |
|------------------|--------|--------------------|
| AC Classification (4 типа) | 28-41 | Упростить до 1 абзаца "skip if..." |
| Task Granularity Rule | 43-98 | Сохранить ядро: unit of work + signals to keep/split |
| Complexity Ceiling | 78-84 | Сохранить 4 эвристики |
| Minimum Decomposition | 86-90 | Сохранить (5+ AC → 3+ tasks) |
| Task Ordering | 100-102 | Сохранить (dependency ordering) |
| Testing Within Tasks | 112-116 | Сохранить (тесты = часть описания задачи) |

**Итого переиспользуется:** ~45 строк из 244 (18%) — но это ценные 18%.

### Что убираем (Go делает)

| Секция bridge.md | Строки | Причина удаления |
|------------------|--------|------------------|
| Format Contract placeholder | 4-8 | JSON schema заменяет |
| Source Traceability | 173-213 | Go добавляет `source:` из `requirement_refs` |
| Merge Mode | 222-239 | Go делает детерминистический merge |
| Gate Marking details | 119-145 | Go ставит GATE программно |
| Prohibited Formats | 216-220 | JSON schema контролирует формат |
| Service Tasks ordering | 168-171 | Go сортирует по tags |
| Negative examples format | 209-213 | Не нужны для JSON |

**Итого убираем:** ~100 строк (41%).

### Что остаётся неизменным (примеры)

Примеры Correct/WRONG из bridge.md (строки 51-110) — самая ценная часть для обучения LLM правильной гранулярности. В plan.md: сократить до 2 компактных примеров (один correct, один wrong), встроенных в секцию Granularity.

---

## 5. Draft промпта plan.md

```markdown
You are a software project planner. Your job is to decompose project requirements
into an ordered list of implementation tasks for an autonomous coding agent.

Each task will be executed in an isolated session — the agent has NO memory of
previous tasks. Therefore every task must be self-contained.

## Project Context

__PROJECT_CONTEXT__

## Requirements

Analyze the following requirements and decompose them into tasks:

__REQUIREMENTS_CONTENT__

## Decomposition Rules

### Requirement Classification

Before creating tasks, classify each requirement:

- **Implementation** — requires writing or modifying code. Creates a task.
- **Behavioral** — a test case of an implementation requirement. Does NOT create
  a separate task — merge into the implementation task as test scenarios.
- **Verification** — confirms existing behavior, zero code changes. Skip entirely
  or mention as a verification step within a related task.
- **Manual** — requires actions the agent cannot perform (browser testing, Docker,
  SSH, UI checks). Skip entirely.

Only Implementation requirements produce tasks.

### Task Granularity

Group requirements by **unit of work**, not by requirement count.

A unit of work is a set of changes that:
- Touch the same file or closely related files (e.g., one module + its test)
- Implement one logical feature (even if multiple requirements)
- Cannot be meaningfully split without breaking compilation or tests

**Split when:**
- Concerns are independent (refactoring + new feature = 2 tasks)
- Task touches 4+ unrelated files across different concerns
- Task combines refactoring existing code with writing new code
- Description exceeds ~300 characters (likely over-packed)

**Keep as one task when:**
- All changes are in the same file or file+test pair
- Splitting would produce a task with no independently testable outcome
- Multiple behavioral requirements test the same implementation

### Minimum / Maximum Decomposition

- 5+ requirements → at least 3 tasks (never one monolithic task)
- 1-2 simple requirements → exactly 1 task (do not pad)
- 6 validators in one file → 1 task (do not over-decompose)

### Testing

Include test scenarios in the task title:
- "...with tests for valid input, invalid values, and edge cases"
- Tests are part of the task, NOT separate entries

### Dependency Ordering

If task B depends on task A (uses code or artifacts created by A),
A must appear before B. Express this via the `depends_on` field.

### Size Estimation

- **S** — changes to 1-2 files, <1 hour of agent work
- **M** — changes to 2-4 files, 1-3 hours (typical task)
- **L** — changes to 4+ files, 3-8 hours (should be rare, consider splitting)
{{- if .HasExistingTasks}}

## Existing Tasks

The project already has these completed and open tasks.
Do NOT regenerate tasks that already exist.
Generate ONLY tasks for requirements not yet covered.

__EXISTING_TASKS_SUMMARY__
{{- end}}

## Output Format

Respond with ONLY a JSON object. No explanations, no markdown fences, no preamble.

{
  "analysis": "1-3 sentences: summary of requirements, key grouping decisions",
  "epics": [
    {
      "name": "Epic Name",
      "tasks": [
        {
          "title": "Imperative description of task (under 500 chars)",
          "test_scenarios": ["scenario 1", "scenario 2"],
          "requirement_refs": ["AC-1", "AC-2"],
          "depends_on": [],
          "tags": [],
          "size": "M"
        }
      ]
    }
  ]
}

Field rules:
- "analysis": reasoning about the requirements BEFORE generating tasks
- "title": imperative sentence, include test scenarios inline
- "test_scenarios": key verification points (empty array for pure infra tasks)
- "requirement_refs": which input requirements this task covers
- "depends_on": titles of prerequisite tasks (empty array if none)
- "tags": ["SETUP"] for new framework/tooling, ["E2E"] for cross-component tests,
  ["GATE"] for deploy/security tasks needing human approval. Usually empty.
- "size": S/M/L estimate. Most tasks should be S or M.
```

### Обоснование дизайна plan.md

1. **~95 строк** вместо 244 в bridge.md — потому что format contract, merge mode, source traceability, gate marking, prohibited formats — всё в Go-коде.

2. **Requirement Classification** сохранена из bridge.md (4 типа AC), но сжата в 10 строк вместо 14. Убраны примеры — они перегружали промпт при JSON output.

3. **Task Granularity** — ядро bridge.md. Сохранены критерии split/keep, убраны 6 полноразмерных примеров (Correct/WRONG). При JSON output LLM реже ошибается с гранулярностью, т.к. формат task'а компактнее.

4. **`analysis` поле** — CoT-зона. LLM рассуждает о группировке ДО генерации задач. Google Cloud research подтверждает: reasoning в первом поле JSON не страдает от format constraints.

5. **Zero-shot** (без few-shot примеров) — JSON schema достаточно чётко определяет формат. Few-shot примеры нужны были bridge.md для markdown (хрупкий формат). JSON — self-describing.

6. **`depends_on` через titles** (не через индексы) — позволяет Go-коду делать topological sort и менять порядок при merge. Titles стабильнее индексов при replan.

7. **`tags` вместо отдельных boolean** — extensible, compact.

8. **Existing tasks summary** — Go инжектирует только titles+status существующих задач (не полный файл). Экономия tokens + предотвращение дублирования.

---

## 6. Draft промпта replan.md

```markdown
You are a software project planner performing a plan correction.

You have an existing task plan and new information (feedback, changed requirements,
or completed task results). Your job is to update the plan with MINIMAL changes.

## Correction Principles

1. **Preserve completed tasks**: tasks marked [DONE] are immutable history.
   NEVER modify, reorder, or remove them.
2. **Minimal diff**: make the smallest changes that address the feedback.
   Do NOT reorganize or rename tasks that are working fine.
3. **Consistency**: new tasks must follow the same granularity and style
   as existing ones.
4. **Dependency awareness**: if you add task C that depends on existing task B,
   include it in `depends_on`. If you remove a task, check nothing depends on it.

## Project Context

__PROJECT_CONTEXT__

## Current Plan

Tasks marked [DONE] are completed. Tasks marked [OPEN] are pending.

__CURRENT_TASKS__

## Feedback / Changes

__FEEDBACK_CONTENT__

## What You Can Do

- **ADD** new tasks to cover uncovered requirements or feedback items
- **MODIFY** the title or test_scenarios of [OPEN] tasks (not [DONE] ones)
- **REMOVE** [OPEN] tasks that are no longer needed (set action to "remove")
- **REORDER** [OPEN] tasks by changing depends_on relationships

You CANNOT modify [DONE] tasks. If completed work needs rework, create a NEW task.

## Output Format

Respond with ONLY a JSON object. No explanations, no markdown fences, no preamble.

{
  "analysis": "1-3 sentences: what changed, why, impact on plan",
  "changes": [
    {
      "action": "add",
      "epic": "Epic Name",
      "task": {
        "title": "New task description",
        "test_scenarios": ["scenario 1"],
        "requirement_refs": ["AC-7"],
        "depends_on": [],
        "tags": [],
        "size": "M"
      },
      "insert_after": "Title of task after which to insert"
    },
    {
      "action": "modify",
      "original_title": "Old task title",
      "task": {
        "title": "Updated task title (may be same)",
        "test_scenarios": ["updated scenarios"],
        "requirement_refs": ["AC-3"],
        "depends_on": [],
        "tags": [],
        "size": "M"
      }
    },
    {
      "action": "remove",
      "original_title": "Task to remove",
      "reason": "Why this task is no longer needed"
    }
  ]
}

Field rules:
- "analysis": reasoning about changes BEFORE generating the diff
- "action": one of "add", "modify", "remove"
- For "add": full task object + "insert_after" (title of predecessor, or "" for first)
- For "modify": "original_title" to identify + updated task object
- For "remove": "original_title" + "reason"
- ONLY include tasks that CHANGE. Do NOT echo unchanged tasks.
- [DONE] tasks MUST NOT appear in changes with "modify" or "remove" action.
```

### Обоснование дизайна replan.md

1. **Diff-формат вместо полного плана** — replan выдаёт только ИЗМЕНЕНИЯ, а не весь план заново. Это критично:
   - Предотвращает случайное изменение/удаление существующих задач
   - Минимизирует output tokens (типичный replan: 2-5 изменений, не 20+ задач)
   - Go-код применяет diff к существующему sprint-tasks.md — детерминистически

2. **`[DONE]` immutability** — явный запрет на модификацию completed задач. Если нужна доработка — новая задача. Это повторяет паттерн merge mode из bridge.md, но проще: вместо "preserve [x] markers" — "DONE tasks are immutable".

3. **`insert_after`** — позиционирование новых задач относительно существующих. Go-код вставляет после указанной задачи, сохраняя порядок.

4. **`reason` для remove** — принуждает LLM обосновать удаление. Снижает вероятность случайного удаления нужной задачи.

5. **Feedback injection** — `__FEEDBACK_CONTENT__` может быть:
   - Текст от пользователя ("добавь валидацию email")
   - Review findings ("тесты для AC-3 не покрывают edge case")
   - Обновлённый PRD section
   - Результаты выполнения ("AC-5 оказался сложнее, нужно split")

6. **~65 строк** — ещё компактнее чем plan.md. Replan — более узкая задача, требует меньше инструкций. Correction principles + what you can do + schema = достаточно.

---

## 7. Источники

### Академические исследования

- [Blueprint First, Model Second](https://arxiv.org/abs/2508.02721) — детерминистические workflow с LLM как bounded sub-task executor. +10.1 п.п. на tau-bench, -81.8% tool calls
- [ADaPT: As-Needed Decomposition and Planning](https://arxiv.org/abs/2311.05772) — as-needed decomposition превосходит upfront planning на 28-33%
- [MetaGPT: Meta Programming for Multi-Agent Framework](https://arxiv.org/html/2308.00352v6) — SOPs как программный enforcement в multi-agent системах
- [Chain-of-Thought Prompting Elicits Reasoning](https://arxiv.org/abs/2201.11903) — оригинальная работа по CoT
- [Teaching LLMs to Plan: Logical Chain-of-Thought](https://arxiv.org/pdf/2509.13351) — CoL для планирования
- [Generating reliable software project task flows using LLMs](https://www.nature.com/articles/s41598-025-19170-9) — task flow генерация через промпт-инжиниринг

### Промпт-инжиниринг и best practices

- [Claude Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs) — constrained decoding для JSON schema compliance
- [Claude Prompting Best Practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices) — официальные рекомендации Anthropic
- [Prompt Engineering Best Practices 2026](https://thomas-wiegold.com/blog/prompt-engineering-best-practices-2026/) — sweet spot 150-300 слов
- [Advanced Decomposition Techniques](https://learnprompting.org/docs/advanced/decomposition/introduction) — SoT, PS, ToT, CoC
- [Hierarchical Task Decomposition Prompt Strategies](https://apxml.com/courses/prompt-engineering-agentic-workflows/chapter-4-prompts-agent-planning-task-management/prompt-strategies-hierarchical-tasks)
- [Claude Prompt Engineering Checklist 2026](https://promptbuilder.cc/blog/claude-prompt-engineering-best-practices-2026/)

### Утечки системных промптов

- [Devin AI System Prompt (aug 2025)](https://github.com/EliFuzz/awesome-system-prompts/blob/main/leaks/devin/archived/2025-08-09_prompt_system.md) — 400+ строк, planning/standard/edit modes
- [System Prompts Collection](https://github.com/x1xhlol/system-prompts-and-models-of-ai-tools) — Cursor, Devin, Windsurf, Claude Code

### Conflict: Reasoning vs Structured Output

- [The Conflict Between LLM Reasoning and Structured Output](https://medium.com/google-cloud/the-conflict-between-llm-reasoning-and-structured-output-fluid-thinking-vs-rigid-rules-e64fb0509d40) — constrained decoding может ломать reasoning
- [Structured-chain-of-thought breaks language-use principles](https://gist.github.com/yoavg/5b106275e38f4ccc796bc8ba7919060b) — критика s-CoT формата

### Инструменты и frameworks

- [MetaGPT PRD automation with DeepSeek](https://www.ibm.com/think/tutorials/multi-agent-prd-ai-automation-metagpt-ollama-deepseek) — практическое применение MetaGPT
- [LLMs are bad at returning code in JSON](https://aider.chat/2024/08/14/code-in-json.html) — markdown лучше для code, JSON для structured data
- [Kovyrin PRD-Tasklist Process](https://kovyrin.net/2025/06/20/prd-tasklist-process/) — PRD + Task List workflow

### Внутренние исследования проекта

- [variant-d-prompt-architecture.md](variant-d-prompt-architecture.md) — предыдущий анализ (сводная таблица, переиспользование bridge.md)
- [bridge-concept-analysis.md](bridge-concept-analysis.md) — критический анализ текущего bridge
- [bridge-performance-analysis.md](bridge-performance-analysis.md) — performance анализ bridge (batching, merge mode)
