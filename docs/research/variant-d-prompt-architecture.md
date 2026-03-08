# Вариант D: Prompt Architecture

Исследование промпт-архитектуры для самодостаточного ralph без BMad workflow.

Дата: 2026-03-07

---

## Best Practices (из исследования)

### 1. Как coding agents делают planning/decomposition

#### Devin (Cognition AI)
- **Подход:** Planner-Worker в единой сессии. Получает требование, сканирует codebase, предлагает plan (Interactive Planning в 2.0)
- **Промежуточный артефакт:** НЕТ. Plan живёт в сессии, человек корректирует интерактивно
- **Ключевой принцип:** Агент сам декомпозирует, сам выполняет, сам итерирует. Нет отдельного planning LLM-вызова
- **Для ralph:** Devin-подход = `ralph plan` генерирует plan в той же сессии, что будет выполнять. Но ralph использует ИЗОЛИРОВАННЫЕ сессии (каждая задача = новый Claude процесс), поэтому plan ДОЛЖЕН быть persistent файлом

#### MetaGPT / MGX
- **Подход:** Multi-agent с SOPs (Standard Operating Procedures). Product Manager agent декомпозирует PRD в задачи
- **SOPs:** ПРОГРАММНЫЙ enforcement через structured outputs, не через markdown файлы
- **Специализация:** Каждый agent имеет чёткий input/output формат (PRD --> System Design --> Tasks --> Code --> Tests)
- **Для ralph:** Ближайший аналог текущего bridge. Но MetaGPT использует in-memory structured outputs, не persistent markdown

#### AutoGPT
- **Подход:** Self-prompting loop. Goal --> subtasks --> execution --> reflection --> next step
- **Planning prompt:** `"You are an expert project planner. Given the goal 'X', break it into actionable steps."`
- **Хранение:** Dynamic task list в памяти агента
- **Для ralph:** AutoGPT-style self-prompting не подходит -- ralph нужен persistent plan для изолированных сессий

#### SWE-Agent / OpenHands
- **Подход:** Issue --> agent напрямую работает с кодом. НЕТ промежуточного task файла
- **Planning:** HyperAgent использует Planner Agent (интерпретирует промпт, координирует worker'ов)
- **OpenHands SDK:** Обсуждается TODO.md-подобный planning tool, вдохновлённый Claude Code
- **Для ralph:** Подход "issue = единственный input" слишком ограничен для PRD с десятками требований

#### Claude Code (рекомендации Anthropic)
- **Подход:** CLAUDE.md + plan mode + direct execution
- **Plan mode:** Read-only режим -- анализ codebase, вопросы, детальный plan БЕЗ модификации файлов
- **Task management:** TodoWrite tool для внутреннего tracking
- **Best practice:** "Никогда не позволяй Claude писать код, пока не одобришь план"
- **Для ralph:** Наиболее релевантный паттерн. `ralph plan` = plan mode, `ralph run` = execution mode

#### Kovyrin PRD-Tasklist Process
- **Подход:** PRD + отдельный Task List, каждая задача выполняется чистым агентом
- **Task List:** Детальный multi-level план + "persistent storage" (затронутые файлы, findings, ссылки)
- **Execution:** Одна задача за раз, агент знает о PRD и task list
- **Для ralph:** ОЧЕНЬ близкий паттерн к тому, что ralph уже делает. Разница: Kovyrin генерирует task list вручную или через LLM в одной сессии

### Сводная таблица подходов

| Agent | Persistent plan? | Кто декомпозирует? | Planning = отдельный LLM call? |
|-------|-----------------|-------------------|-------------------------------|
| Devin | Нет (in-session) | Сам агент | Нет |
| MetaGPT | Да (in-memory) | Product Manager agent | Да (специализированный agent) |
| AutoGPT | Да (in-memory) | Self-prompting loop | Да (тот же LLM) |
| SWE-Agent | Нет | Сам агент | Нет |
| Claude Code | Нет (plan mode) | Сам агент | Нет |
| Kovyrin | Да (файл) | Человек + LLM | Да (ручной + LLM) |
| **ralph bridge** | **Да (файл)** | **Отдельный LLM** | **Да** |
| **ralph plan (D)** | **Да (файл)** | **LLM в plan mode** | **Да** |

**Вывод:** Persistent plan file нужен ralph из-за изолированных сессий. Но generating его через отдельный LLM-вызов с 244 строками правил -- антипаттерн. Оптимально: LLM генерирует СОДЕРЖАНИЕ задач, Go-код форматирует OUTPUT.

### 2. Chain-of-Thought vs Structured Output для task generation

**Фундаментальное противоречие:**

| CoT (chain-of-thought) | Structured Output |
|------------------------|-------------------|
| Вероятностное рассуждение | Детерминистический формат |
| Гибкость, нюансы | Жёсткая схема |
| Лучше для сложных решений | Лучше для машинного парсинга |
| Недетерминистичен | Может ограничить reasoning |

Исследование "Blueprint First, Model Second" (2025) показывает: **декомпозировать workflow на детерминистические шаги + использовать LLM ТОЛЬКО для bounded sub-tasks** -- даёт +10.1 п.п. улучшение на tau-bench и сокращение tool calls на 81.8%.

**Google Cloud research** подтверждает: constrained decoding (JSON mode) маскирует токены на каждом шаге генерации, что может "derail" reasoning. Решение: **CoT reasoning В ПРОМЕЖУТОЧНЫХ шагах, structured output В ФИНАЛЬНОМ шаге**.

**Рекомендация для ralph plan:**
```
Шаг 1 (CoT): LLM анализирует PRD, рассуждает о группировке требований
Шаг 2 (Structured): LLM выдаёт задачи в жёстком формате
ИЛИ:
Шаг 1: LLM анализирует и выдаёт structured JSON
Шаг 2: Go-код форматирует JSON в sprint-tasks.md
```

Второй вариант предпочтительнее -- Go-код ГАРАНТИРУЕТ формат, LLM отвечает только за содержание.

### 3. Few-shot vs Zero-shot для format compliance

| Критерий | Zero-shot | Few-shot (2-3 примера) |
|----------|-----------|----------------------|
| Format compliance | Средняя | Высокая |
| Token cost | Ниже | Выше (~500-1000 tokens) |
| Гибкость | Выше | Может "залипнуть" на примере |
| Нужно при | Простых форматах | Сложных/нестандартных форматах |

**Исследования показывают:** Few-shot критичен для structured extraction и domain-specific форматов. Zero-shot достаточен когда формат тривиален (JSON с ясной schema).

**Рекомендация для ralph plan:**
- Если OUTPUT = JSON (Go парсит и форматирует): **zero-shot** + JSON schema description
- Если OUTPUT = markdown (sprint-tasks.md напрямую): **few-shot** обязателен (2 примера: простой и сложный)
- Текущий bridge.md использует few-shot (Correct/WRONG примеры) -- это правильно для markdown output

### 4. Детерминизм regex-parseable output

**Современные подходы (ранжированы по надёжности):**

1. **API-level structured outputs** (OpenAI JSON Schema, Anthropic tool use) -- 100% compliance, но ограничивает reasoning
2. **Constrained decoding** (Outlines, vLLM grammar) -- 100% compliance для self-hosted моделей
3. **Two-step: LLM reasoning + Go formatting** -- 99%+ compliance, не ограничивает reasoning
4. **Few-shot + format contract в промпте** -- 95-98% compliance (текущий подход bridge)
5. **Zero-shot с schema description** -- 85-95% compliance

**Для ralph (использует Claude CLI, не API):**
- Constrained decoding и API structured outputs недоступны
- Оптимальный подход: **two-step** -- LLM выдаёт JSON, Go форматирует в sprint-tasks.md
- Fallback: few-shot с валидацией + retry (текущий подход, но надёжнее)

**Конкретная техника:**
```
Промпт: "Output a JSON array of task objects..."
Go-код: json.Unmarshal → validate → format sprint-tasks.md
Retry: если JSON невалиден, повторить с error feedback
```

### 5. Multi-step vs Single-pass decomposition

**ADaPT research** (Allen AI, 2024): as-needed decomposition превосходит upfront planning на 28-33%.

**Systematic Decomposition (2025):** Для сложных задач multi-step даёт лучшие результаты, но для простых -- overhead.

| Подход | Когда использовать | Для ralph plan |
|--------|-------------------|----------------|
| **Single-pass** | PRD < 5 требований, одна concern | Простые фичи |
| **Two-step** (analyze + generate) | 5-15 требований | Типичный use case |
| **Multi-step** (classify + group + format) | 15+ требований, multiple concerns | Крупные PRD |
| **ADaPT** (decompose as-needed) | Неизвестная сложность | Идеально для ralph |

**Рекомендация:** Two-step по умолчанию (анализ + генерация), с возможностью ADaPT-style повторной декомпозиции если задача слишком сложная для одной сессии.

### 6. JSON vs Markdown для task list output

| Критерий | JSON | Markdown |
|----------|------|----------|
| Machine parsing | Нативный `json.Unmarshal` | Regex (хрупко) |
| Token efficiency | Больше tokens (скобки, кавычки) | ~16% экономия |
| Code quality | LLM пишет ХУЖЕ код в JSON обёртке | Лучше в markdown |
| Human readability | Средняя | Высокая |
| Go parsing | `encoding/json` -- надёжно | Regex -- хрупко |
| Nested structures | Нативная поддержка | Ограниченная |

**Aider research:** LLM пишут хуже код когда output обёрнут в JSON. Для CODE output -- markdown лучше.

**Для TASK LIST (не код):** JSON предпочтительнее:
- Задачи -- структурированные данные (title, source, gate, dependencies)
- Go парсит JSON нативно, без regex
- LLM не пишет код в этом промпте, а описывает задачи

**Рекомендация для ralph plan:**
- LLM output: JSON (задачи как structured data)
- Финальный артефакт: sprint-tasks.md (Go форматирует из JSON)
- Человек читает sprint-tasks.md, Go парсит sprint-tasks.md regex'ами (как сейчас)

### 7. Project context injection в planning prompt

**Aider repository map:** Граф зависимостей исходного кода, ранжированный по релевантности к текущей задаче. Классы + сигнатуры функций без тел.

**Repomix:** Весь codebase в один файл для LLM.

**Addy Osmani рекомендации:**
- Spec = Purpose + Requirements + Inputs/Outputs + Constraints + APIs + Milestones + Coding Conventions
- Adjust detail to complexity -- не over-spec простые задачи
- Include verification criteria (highest-leverage practice)

**Рекомендация для ralph plan:**
- **Минимум:** tech stack (язык, фреймворки, зависимости) + directory structure
- **Оптимум:** repo map (packages + exported symbols) + existing sprint-tasks.md (если есть)
- **Максимум:** полный tree + key files content -- слишком дорого по tokens

Конкретно для ralph:
```go
type PlanContext struct {
    TechStack   string   // go.mod content
    DirTree     string   // ls -R output (filtered)
    RepoMap     string   // exported symbols per package
    ExistingTasks string // current sprint-tasks.md if exists
    Knowledge   string   // distilled knowledge
}
```

---

## Новая промпт-архитектура

### Общий дизайн

```
ralph plan flow:

[Input: PRD/requirements/issue файл]
        |
        v
[Go: читает файл, собирает project context]
        |
        v
[LLM: plan.md промпт → JSON массив задач]
        |
        v
[Go: валидация JSON, форматирование → sprint-tasks.md]
        |
        v
[ralph run: выполняет задачи как обычно]
```

### Промпт plan.md -- архитектура

**Философия:** LLM отвечает за СОДЕРЖАНИЕ (анализ, декомпозиция, группировка). Go отвечает за ФОРМАТ (sprint-tasks.md).

**Sections промпта:**

1. **Role** -- "You are a software project planner"
2. **Project Context** -- tech stack, directory structure, existing tasks (injected)
3. **Input** -- PRD/requirements content (injected)
4. **Decomposition Rules** -- правила группировки (из bridge.md, упрощённые)
5. **Output Schema** -- JSON schema для задач
6. **Examples** -- 1-2 few-shot примера (optional, для сложных случаев)

### Сравнение с текущим bridge.md

| Аспект | bridge.md (текущий) | plan.md (новый) |
|--------|-------------------|-----------------|
| Строк правил | 244 | ~80-100 |
| Format enforcement | В промпте (хрупко) | В Go-коде (надёжно) |
| Output | Markdown sprint-tasks.md | JSON массив задач |
| Input | Story файлы | PRD/requirements/issue |
| Context | Только story content | Tech stack + repo map + existing tasks |
| Few-shot | 6+ примеров inline | 1-2 примера (или zero-shot с JSON schema) |
| Merge mode | 30 строк LLM-правил | Go-код (детерминистический merge) |
| Gate marking | LLM-эвристика | Go-код (программные правила) |
| Source traceability | LLM генерирует `source:` | Go-код добавляет `source:` |
| Batching | 6+ batch'ей последовательно | 1 вызов (JSON compact) |

### Ключевые решения

**1. JSON output вместо markdown:**
- LLM выдаёт `[{"title": "...", "acs": ["AC-1", "AC-2"], "dependencies": [], "gate": false}]`
- Go конвертирует в sprint-tasks.md с гарантированным форматом
- Убирает 50+ строк format contract из промпта

**2. Go-code для merge, gates, source:**
- Merge mode: Go читает existing sprint-tasks.md, сравнивает, добавляет новые задачи
- Gates: Go ставит `[GATE]` по программным правилам (первая задача epic'а, deploy keyword)
- Source: Go добавляет `source:` по mapping (PRD section --> task)

**3. Project context injection:**
- `go.mod` (tech stack)
- Directory tree (структура проекта)
- Exported symbols per package (repo map) -- опционально
- Существующий sprint-tasks.md (для merge)

**4. ADaPT-style complexity detection:**
- Если PRD < 5 требований: single-pass (один LLM-вызов)
- Если PRD 5-20 требований: two-step (анализ + генерация)
- Если PRD > 20 требований: split по sections, каждая section = LLM-вызов

---

## Переиспользование из текущих промптов

### Что взять из bridge.md

| Секция bridge.md | Ценность | Действие для plan.md |
|------------------|----------|---------------------|
| **AC Classification** (4 типа) | ВЫСОКАЯ | Упростить: 2 типа (Implementation, Skip). Behavioral/Verification/Manual --> "skip" c пояснением |
| **Task Granularity Rule** | ВЫСОКАЯ | Взять ядро: "unit of work" + complexity ceiling. Убрать 50% примеров |
| **Gate Marking** | СРЕДНЯЯ | Перенести в Go-код. В промпте: "mark tasks that need human approval" |
| **Format Contract** | НИЗКАЯ | Убрать полностью. JSON schema заменяет |
| **Source Traceability** | НИЗКАЯ | Убрать полностью. Go-код добавляет |
| **Service Tasks** | СРЕДНЯЯ | Упростить: `[SETUP]` и `[E2E]` как boolean в JSON |
| **Merge Mode** | НИЗКАЯ | Убрать полностью. Go-код делает merge |
| **Prohibited Formats** | НИЗКАЯ | Не нужно -- JSON schema контролирует |
| **Testing Within Tasks** | ВЫСОКАЯ | Сохранить: "include test scenarios in task description" |
| **Task Ordering** | ВЫСОКАЯ | Сохранить: dependency-aware ordering |

### Что взять из execute.md

| Секция | Ценность для plan.md | Действие |
|--------|---------------------|----------|
| Self-Directing Instructions | НИЗКАЯ | Не нужно для planning |
| 999-Rules Guardrails | СРЕДНЯЯ | Адаптировать: "do not include tasks that require external access" |
| ATDD | ВЫСОКАЯ | Интегрировать: "each task must have testable acceptance criteria" |
| Scope Boundary | НИЗКАЯ | Не нужно для planning |

### Что взять из review.md

| Секция | Ценность | Действие |
|--------|----------|----------|
| Sub-Agent Orchestration | НИЗКАЯ | Не применимо к planning |
| Severity Assignment | СРЕДНЯЯ | Адаптировать для task priority |
| Finding Structure | НИЗКАЯ | Не применимо |

### Итоговая оценка переиспользования

- ~30% bridge.md переиспользуется (AC Classification ядро, Granularity Rule ядро, Testing, Ordering)
- ~70% bridge.md заменяется Go-кодом или JSON schema
- execute.md/review.md дают мало для planning промпта

---

## Prompt Chain Design

### Полный flow `ralph plan`

```
Step 1: Collect context          [ПРОГРАММНЫЙ, Go]
Step 2: Analyze input            [LLM]
Step 3: Generate task list       [LLM (может быть тот же вызов)]
Step 4: Validate & format        [ПРОГРАММНЫЙ, Go]
Step 5: Write sprint-tasks.md    [ПРОГРАММНЫЙ, Go]
```

### Детализация каждого шага

#### Step 1: Collect context (Go)

```go
func CollectPlanContext(cfg *config.Config) (*PlanContext, error) {
    // 1. Прочитать go.mod (или package.json, requirements.txt)
    techStack := readTechStack(cfg.ProjectRoot)

    // 2. Directory tree (отфильтрованный)
    dirTree := buildDirTree(cfg.ProjectRoot, maxDepth=3)

    // 3. Existing sprint-tasks.md (если есть)
    existingTasks := readExistingTasks(cfg.ProjectRoot)

    // 4. Distilled knowledge (если есть)
    knowledge := readKnowledge(cfg.ProjectRoot)

    return &PlanContext{techStack, dirTree, existingTasks, knowledge}, nil
}
```

**Почему Go:** Детерминистическая операция. Не нужен LLM для чтения файлов.

#### Step 2+3: Analyze + Generate (LLM, один вызов)

**Промпт:** plan.md с injected context и input.

**LLM получает:**
- Project context (tech stack, dir tree)
- Requirements content (PRD, issue, или свободная форма)
- Decomposition rules (упрощённые из bridge.md)
- JSON output schema

**LLM выдаёт:**
```json
{
  "analysis": "краткий анализ требований",
  "tasks": [
    {
      "title": "Implement user authentication endpoint",
      "description": "Add POST /api/auth/login with JWT token generation",
      "test_scenarios": ["valid credentials → 200 + token", "invalid → 401"],
      "acs": ["FR-1", "FR-2"],
      "epic": "Authentication",
      "dependencies": [],
      "gate": false,
      "setup": false,
      "e2e": false,
      "complexity": "medium"
    }
  ]
}
```

**Почему LLM:** Семантический анализ требований, группировка по "unit of work", определение зависимостей -- требует "понимания".

#### Step 4: Validate & Format (Go)

```go
func ValidateAndFormat(tasks []Task, existing []Task) (string, error) {
    // 1. Валидация JSON (обязательные поля, формат)
    if err := validateTasks(tasks); err != nil {
        return "", fmt.Errorf("invalid tasks: %w", err)
    }

    // 2. Gate marking (программные правила)
    applyGateRules(tasks)

    // 3. Merge с existing tasks (если есть)
    merged := mergeTasks(existing, tasks)

    // 4. Format в sprint-tasks.md
    return formatSprintTasks(merged), nil
}
```

**Почему Go:**
- Gate marking: "первая задача epic'а" + "deploy/security keywords" -- regex/keyword match
- Merge: детерминистический diff по task title + source
- Format: строковая операция, 100% compliance гарантирован

#### Step 5: Write (Go)

Записать sprint-tasks.md. Тривиально.

### Comparison: что программное, что LLM

| Операция | Текущий bridge | Новый plan |
|----------|---------------|------------|
| Чтение input | Go | Go |
| AC Classification | LLM (в промпте) | LLM (упрощённые правила) |
| Task Grouping | LLM | LLM |
| Dependency ordering | LLM | LLM + Go (topological sort) |
| Gate marking | LLM | **Go** (программно) |
| Source traceability | LLM | **Go** (программно) |
| Format contract | LLM (244 строки правил) | **Go** (json.Unmarshal + template) |
| Merge mode | LLM (30 строк) | **Go** (детерминистический diff) |
| Batching | Нужно (LLM output = markdown) | **Не нужно** (LLM output = compact JSON) |

**Результат:** 5 из 8 операций переносятся из LLM в Go. Промпт сокращается с 244 до ~80-100 строк.

---

## Пример промпта plan.md (draft)

```markdown
You are a software project planner. Your job is to decompose requirements into
an ordered list of implementation tasks for an autonomous coding agent.

## Project Context

__PROJECT_CONTEXT__

## Requirements

Analyze the following requirements and decompose them into tasks:

__REQUIREMENTS_CONTENT__

## Decomposition Rules

### Task Granularity

A task is a single unit of work that an autonomous coding agent can complete
in one isolated session (no memory of previous tasks).

Group requirements by **unit of work**:
- Changes to the same file or closely related files = one task
- One logical feature = one task (even if multiple requirements)
- Split when concerns are independent (refactoring + new feature = 2 tasks)

### Complexity Ceiling

Each task should be completable in 1-3 code review cycles:
- If a task touches 4+ unrelated files → split by concern
- If description exceeds ~300 characters → likely over-packed
- If a task combines refactoring with new code → split

### Minimum Decomposition

- 5+ requirements → at least 3 tasks
- Never collapse everything into one monolithic task
- But do NOT over-decompose: 6 validators in one file = 1 task

### Testing

Include test scenarios in the task description:
- "...with tests for valid input, invalid values, and edge cases"
- Tests are part of the task, not separate entries

### Dependency Ordering

If task B depends on task A (uses code created by A), task A must come first.

### Classification

Skip requirements that:
- Say "already implemented" or "verify existing behavior"
- Require manual actions (browser testing, Docker, SSH)
- Are purely behavioral tests of another requirement (merge into that task)

## Output Format

Respond with ONLY a JSON object. No explanations, no code fences, no preamble.

Schema:
{
  "analysis": "1-2 sentence summary of the requirements",
  "epics": [
    {
      "name": "Epic Name",
      "tasks": [
        {
          "title": "Task description (under 500 chars)",
          "test_scenarios": ["scenario 1", "scenario 2"],
          "requirement_refs": ["REQ-1", "REQ-2"],
          "dependencies": [],
          "needs_human_approval": false,
          "is_setup": false,
          "is_e2e": false
        }
      ]
    }
  ]
}

Rules:
- "title": imperative sentence describing the implementation work
- "test_scenarios": key test cases (empty array if purely infra)
- "requirement_refs": which requirements this task covers
- "dependencies": titles of tasks that must complete first
- "needs_human_approval": true for deployments or security changes
- "is_setup": true for new framework/tooling installation
- "is_e2e": true for cross-component integration tests
{{- if .HasExistingTasks}}

## Existing Tasks

The project already has these tasks. Do NOT re-generate them.
Generate ONLY tasks for requirements not yet covered:

__EXISTING_TASKS_SUMMARY__
{{- end}}
```

### Обоснование дизайна промпта

1. **~80 строк** вместо 244 -- потому что формат (sprint-tasks.md), merge, gates, source -- в Go
2. **JSON output** -- Go парсит `json.Unmarshal`, 100% format compliance
3. **Decomposition Rules** сохранены из bridge.md (ценное ядро)
4. **Classification** упрощена: 4 типа --> 1 абзац "skip if..."
5. **Few-shot примеры убраны** -- JSON schema достаточно для Claude
6. **Existing tasks** -- список titles (не полный файл), чтобы не дублировать
7. **Project context** -- injected Go-кодом, адаптируется к любому проекту

---

## Источники

### Исследования и статьи

- [Blueprint First, Model Second: A Framework for Deterministic LLM Workflow](https://arxiv.org/abs/2508.02721) -- детерминистические workflow с LLM как bounded sub-task executor. +10 п.п. на tau-bench
- [ADaPT: As-Needed Decomposition and Planning with Language Models](https://arxiv.org/abs/2311.05772) -- as-needed decomposition превосходит upfront planning на 28-33%
- [MetaGPT: Meta Programming for a Multi-Agent Collaborative Framework](https://arxiv.org/html/2308.00352v6) -- SOPs как программный enforcement в multi-agent системах
- [The Conflict Between LLM Reasoning and Structured Output](https://medium.com/google-cloud/the-conflict-between-llm-reasoning-and-structured-output-fluid-thinking-vs-rigid-rules-e64fb0509d40) -- constrained decoding может ломать reasoning
- [An Approach for Systematic Decomposition of Complex LLM Tasks](https://arxiv.org/html/2510.07772v1) -- multi-step decomposition для сложных задач
- [LLMs are bad at returning code in JSON](https://aider.chat/2024/08/14/code-in-json.html) -- markdown лучше для code output, JSON -- для structured data
- [Does Output Format Actually Matter?](https://checksum.ai/blog/does-output-format-actually-matter-an-experiment-comparing-json-xml-and-markdown-for-llm-tasks) -- сравнение JSON, XML, Markdown для LLM tasks
- [Markdown is 15% more token efficient than JSON](https://community.openai.com/t/markdown-is-15-more-token-efficient-than-json/841742) -- token efficiency разных форматов
- [Fast, High-Fidelity LLM Decoding with Regex Constraints](https://huggingface.co/blog/vivien/llm-decoding-with-regex-constraints) -- constrained decoding через DFA
- [Understanding Software Engineering Agents: Trajectories Analysis](https://software-lab.org/publications/ase2025_trajectories.pdf) -- анализ траекторий SWE-Agent и OpenHands
- [The OpenHands Software Agent SDK](https://arxiv.org/html/2511.03690v1) -- composable architecture для coding agents

### Best Practices и руководства

- [How to write a good spec for AI agents](https://addyosmani.com/blog/good-spec/) -- Addy Osmani о спецификациях для AI агентов
- [Align, Plan, Ship: PRD-Driven AI Agents](https://kovyrin.net/2025/06/20/prd-tasklist-process/) -- Kovyrin о PRD + Task List workflow
- [Claude Code Best Practices](https://code.claude.com/docs/en/best-practices) -- официальные рекомендации Anthropic
- [Plan Mode in Claude Code](https://codewithmukesh.com/blog/plan-mode-claude-code/) -- plan mode для planning перед execution
- [Prompting Best Practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices) -- Claude API prompting guide
- [How to write PRDs for AI Coding Agents](https://medium.com/@haberlah/how-to-write-prds-for-ai-coding-agents-d60d72efb797) -- PRD для coding agents
- [Devin Agents 101](https://devin.ai/agents101) -- Devin подход к autonomous coding
- [Repository Map (Aider)](https://aider.chat/docs/repomap.html) -- граф-based repo map для codebase awareness
- [Few-Shot Prompting Guide](https://www.promptingguide.ai/techniques/fewshot) -- когда few-shot vs zero-shot
- [Zero-Shot vs Few-Shot: A Guide](https://www.vellum.ai/blog/zero-shot-vs-few-shot-prompting-a-guide-with-examples) -- сравнение подходов

### Внутренние исследования проекта

- [bridge-concept-analysis.md](bridge-concept-analysis.md) -- критический анализ текущего bridge (варианты A-E)
- [bridge-performance-analysis.md](bridge-performance-analysis.md) -- анализ performance bridge (batching, merge mode)
