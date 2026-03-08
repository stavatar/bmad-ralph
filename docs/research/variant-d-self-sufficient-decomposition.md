# Вариант D: Самодостаточная декомпозиция требований в ralph

**Дата:** 2026-03-07
**Контекст:** ralph зависит от BMad workflow (PRD → Architecture → Epics → Stories → Bridge → Tasks). Исследуем возможность встроенной декомпозиции — от требования до sprint-tasks.md без внешних workflow.

---

## 1. Как это делают другие (обзор 8 инструментов)

### 1.1 Devin (Cognition AI)

**Архитектура:** Planner-Worker-Critic — три специализированные модели.

- **Planner** (high-reasoning модель) разбивает задачу на пошаговый план
- **Coder** (fine-tuned на коде) реализует каждый шаг
- **Critic** (adversarial) проверяет безопасность и логику перед выполнением
- **Interactive Planning (v2.0+):** Devin предлагает план, человек корректирует ДО начала работы
- **Dynamic re-planning (v3.0):** при препятствии меняет стратегию автоматически

**Ключевое:** НЕТ промежуточного файла. План живёт в сессии. Devin сам сканирует codebase, предлагает план, получает одобрение, выполняет.

**Входной формат:** Natural language description (Issue, Slack-сообщение, требование в свободной форме).

### 1.2 SWE-Agent / OpenHands

**Архитектура:** ReAct-style (Thought → Action → Observation цикл).

- **SWE-Agent:** Agent-Computer Interface (ACI) — специализированные команды для навигации и редактирования. Получает GitHub Issue, сам решает что делать
- **OpenHands:** Event-sourced stateless архитектура. Composable agents через SDK

**Декомпозиция:** Полностью implicit — агент не создаёт план, а итеративно исследует, локализует, патчит. AutoCodeRover (structured pipeline) достигает результата за 6 шагов vs 29 у OpenHands.

**Ключевое:** GitHub Issue = единственный input. Никакого промежуточного task file.

### 1.3 Claude Code (нативный workflow Anthropic)

**Архитектура:** Plan Mode + Agent Mode.

- **Plan Mode:** Read-only анализ. Claude создаёт plan.md, человек одобряет, затем переключение в execution mode
- **Agent Mode:** Автономное выполнение с полным доступом к файловой системе
- **Agent Teams (2026):** Несколько Claude-инстансов через tmux, team lead координирует, workers выполняют

**Промпт-паттерн PRD → Tasks (из документации Anthropic):**
1. "Read @docs/prd.md and create an implementation plan. Do NOT write any code yet."
2. Человек одобряет план
3. "Convert this plan into a TODO.md with checkboxes. Implement step by step."

**Ключевое:** План создаётся В ТОЙ ЖЕ сессии. Нет отдельного bridge-шага. baby-steps framework: 3-5 задач с verification criteria.

### 1.4 GitHub Copilot Workspace

**Архитектура:** Issue → Specification → Plan → Code → PR.

- Начинается с GitHub Issue
- Автоматически генерирует **спецификацию** (что изменить) из issue + codebase
- Генерирует **план** (какие файлы, какие изменения)
- Два checkpoint'а для человека: корректировка спецификации и плана
- Генерирует код, создаёт PR

**Ключевое:** Встроенная двухфазная декомпозиция (spec + plan), но всё в одном UI/сессии, без внешних файлов.

### 1.5 Cursor Agent / Windsurf Cascade

**Cursor Agent Mode:**
- Описание задачи → план → edits → diff для approval
- Manual review cycle: AI предлагает, человек одобряет каждый diff
- Нет persistent task list

**Windsurf Cascade:**
- Graph-based reasoning: строит граф зависимостей codebase перед планированием
- Hierarchical context: анализирует модули, shared types, cross-cutting concerns
- Multi-step execution без промежуточного файла
- Параллельные агенты через Git worktrees

**Ключевое:** Оба инструмента планируют "на лету" внутри сессии, не создавая отдельный task file.

### 1.6 CCPM (automazeio)

**Архитектура:** PRD → Epic → Task → GitHub Issue → Code → Commit.

- `/pm:prd-new` → создание PRD
- `/pm:prd-parse` → технический план из PRD
- `/pm:epic-decompose` → задачи с acceptance criteria, effort estimates, parallel flags
- `/pm:epic-sync` → синхронизация с GitHub Issues
- `/pm:issue-start` → агент берёт issue и реализует

**Ключевое:** Вся декомпозиция через Claude Code slash-commands. GitHub Issues = source of truth. Без внешних workflow-инструментов. Архитектурно ОЧЕНЬ близок к тому, что ralph мог бы делать.

### 1.7 claude-code-skills (levnikolaevich)

**Архитектура:** 4-уровневая Orchestrator-Worker иерархия (L0-L3).

- **ln-200-scope-decomposer:** PRD → 3-7 Epics + Stories
- **ln-300-task-coordinator:** Story → 1-6 задач
- **ln-310-story-validator:** 20 критериев, penalty points
- **ln-400-story-executor:** Task Execution → Review Loop → Test → Quality Gate
- **ln-500-story-quality-gate:** PASS/CONCERNS/REWORK/FAIL

**Fallback:** Без Linear работает с `kanban_board.md` (local markdown).

**Ключевое:** Полный pipeline "PRD → код" через Claude Code skills. Фактически — BMad workflow, встроенный в Claude Code.

### 1.8 MetaGPT / MGX

**Архитектура:** Multi-agent с SOPs (Standard Operating Procedures).

- Product Manager agent → System Design → Tasks
- SOPs = программный enforcement, не prompt enforcement
- Structured outputs между agents (in-memory, не через файлы)

**Ключевое:** Единственный инструмент (кроме ralph) с persistent task file. Но task file — structured JSON между agents, не markdown.

### Сводная таблица

| Инструмент | Промежуточный task file? | Кто декомпозирует? | Входной формат | Внешние зависимости |
|------------|------------------------|-------------------|---------------|-------------------|
| Devin | Нет (in-session plan) | Сам агент (3 модели) | Natural language | Нет |
| SWE-Agent | Нет | Сам агент (ReAct) | GitHub Issue | Нет |
| Claude Code | plan.md (optional) | Сам агент | Natural language / PRD | Нет |
| Copilot Workspace | Нет (UI state) | Встроенный pipeline | GitHub Issue | GitHub |
| Cursor/Windsurf | Нет | Сам агент | Natural language | Нет |
| CCPM | GitHub Issues | Claude Code skills | PRD | GitHub Issues |
| claude-code-skills | kanban_board.md | Claude Code skills (L0-L3) | PRD | Linear (optional) |
| MetaGPT | Structured JSON | Product Manager agent | Natural language | Нет |
| **bmad-ralph** | **sprint-tasks.md** | **Отдельный LLM bridge** | **BMad Stories** | **BMad workflow** |

**Вывод:** ralph — единственный инструмент из 9 рассмотренных, который:
1. Требует ВНЕШНИЙ workflow для создания входных данных (stories)
2. Использует ОТДЕЛЬНЫЙ LLM-вызов (bridge) для создания промежуточного task file
3. Имеет BATCHING проблему из-за разделения planning и execution

---

## 2. Архитектура `ralph plan`

### 2.1 Концепция

Новая команда `ralph plan` заменяет связку "BMad stories + ralph bridge" одним шагом:

```
БЫЛО:
  BMad AI (PRD → Architecture → Epics → Stories) → ralph bridge (Stories → Tasks) → ralph run (Tasks → Code)
                     ^^^^^^^^^^^^^^^^^^^                    ^^^^^^^^^^^^
                     Внешняя зависимость                   LLM bridge

СТАЛО:
  ralph plan (PRD → Tasks) → ralph run (Tasks → Code)
       ^^^^^^^^^^^^^^^^^^^^
       Встроенная декомпозиция
```

### 2.2 Три режима работы

#### Режим A: `ralph plan <prd.md>`

Основной режим. Принимает PRD (или любой requirement document) и генерирует sprint-tasks.md.

```bash
ralph plan docs/prd/feature-x.md
ralph plan docs/prd/feature-x.md --architecture docs/architecture.md
ralph plan docs/prd/feature-x.md --output sprint-tasks.md
```

**Алгоритм:**
1. Читает PRD, извлекает Functional Requirements (FR)
2. Если указана архитектура — читает для технического контекста
3. Отправляет Claude промпт-планировщик (см. раздел 3)
4. Claude генерирует структурированный план
5. ralph парсит ответ, форматирует в sprint-tasks.md
6. Показывает план пользователю для одобрения

#### Режим B: `ralph plan --from-issue <URL>`

Из GitHub Issue или группы issues.

```bash
ralph plan --from-issue https://github.com/org/repo/issues/42
ralph plan --from-issue https://github.com/org/repo/issues/42,43,44
ralph plan --from-label "sprint-3"
```

**Алгоритм:**
1. `gh issue view` → получает title + body + comments
2. Контекст из CLAUDE.md и codebase scan
3. Claude планирует задачи с учётом issue context
4. Результат → sprint-tasks.md

#### Режим C: `ralph plan --interactive`

Интерактивная сессия (аналог Claude Code Plan Mode).

```bash
ralph plan --interactive
# > Опишите что нужно сделать:
# > Добавить систему аутентификации с OAuth2
# > Какие провайдеры? Google, GitHub
# > [Генерация плана...]
# > Одобрить план? [y/n/edit]
```

**Алгоритм:**
1. ralph задаёт уточняющие вопросы (через Claude)
2. Собирает ответы в requirement document (in-memory)
3. Сканирует codebase для контекста
4. Генерирует план, показывает пользователю
5. Итерация до одобрения

### 2.3 Архитектурные решения

#### Что сохраняется от текущего ralph

| Компонент | Решение | Обоснование |
|-----------|---------|-------------|
| `sprint-tasks.md` | Сохраняется как output формат | Runner уже умеет с ним работать, 42 story отлажены на нём |
| `- [ ]` / `- [x]` формат | Сохраняется | Детерминистический парсинг regex'ами, отлаженный |
| `source:` поля | Модифицируются: `source: prd.md#FR-3` | Traceability к FR вместо AC stories |
| `[GATE]` маркеры | Сохраняются | Human checkpoints — proven value |
| `runner.Execute()` | Без изменений | Runner не знает КАК задачи создались |

#### Что меняется

| Было | Стало | Причина |
|------|-------|---------|
| BMad создаёт stories | ralph plan создаёт tasks напрямую | Устранение внешней зависимости |
| bridge.Run() = LLM call | plan.Run() = LLM call, НО с другим промптом | Один LLM-шаг вместо трёх (BMad + bridge + runner) |
| Stories = промежуточный артефакт | Нет промежуточного артефакта | "Испорченный телефон" устранён |
| 6 batch'ей для 34 stories | 1-2 вызова для PRD | PRD компактнее чем набор stories |
| AC classification в bridge промпте | FR classification в plan промпте | Прямая работа с FR, не с AC |

#### Новый пакет `planner/`

```
cmd/ralph/
├── plan.go          // cobra command для ralph plan
├── ...
planner/
├── planner.go       // Plan(ctx, cfg, opts) → sprint-tasks.md
├── parser.go        // ParsePRD(content) → []FR (программный парсинг FR)
├── formatter.go     // FormatTasks([]Task) → string (sprint-tasks.md)
├── prompts/
│   └── plan.md      // Go template для промпта планировщика
└── planner_test.go
```

**Dependency direction:**
```
cmd/ralph → planner → session, config
cmd/ralph → runner  → session, config   // без изменений
```

Пакет `planner` параллелен `runner` и `bridge`. `bridge` deprecated но не удалён.

### 2.4 ADaPT-паттерн: декомпозиция по необходимости

Вместо upfront decomposition всех FR в задачи, ralph plan может использовать ADaPT-подход:

1. **Простые FR (1-2 файла, одна concern):** Программная генерация задачи без LLM
2. **Средние FR (3-5 файлов, одна concern):** Одна задача, полный FR-контекст в промпте
3. **Сложные FR (5+ файлов, multiple concerns):** LLM декомпозирует на 2-4 задачи

Это гибрид — 80% задач генерируются программно (детерминизм), 20% через LLM (когда нужна семантика).

---

## 3. Промпт-стратегия (без BMad stories)

### 3.1 Ключевое изменение: FR вместо AC

**Было (bridge.md — 244 строки):**
- Классификация AC по 4 типам (Implementation, Behavioral, Verification, Manual)
- Группировка AC по "unit of work"
- 100+ строк правил granularity
- Merge mode для batch'ей

**Стало (plan.md — ~120 строк):**
- Прямая работа с Functional Requirements из PRD
- Группировка FR по зависимостям и файлам
- Одна сессия = один PRD (нет batch'ей)

### 3.2 Структура промпта планировщика

```markdown
You are a task planner for an autonomous coding agent (Claude Code).

## Input

### Product Requirements
__PRD_CONTENT__

### Architecture Context (if available)
__ARCHITECTURE_CONTENT__

### Existing Codebase Structure
__CODEBASE_TREE__

### Existing Tasks (for merge mode)
__EXISTING_TASKS__

## Task Generation Rules

1. Each task = ONE atomic code change completable in a single Claude Code session
2. Sessions are ISOLATED — no memory between tasks. Each must be self-contained.
3. Task description must include:
   - What to implement (specific files, functions, types)
   - What tests to write (test scenarios inline)
   - What FR(s) this satisfies
4. Order tasks by dependency: if B imports from A, A comes first
5. First task of each logical group gets [GATE] for human review

## Granularity Heuristics

- 1 FR touching 1-2 files → 1 task
- 1 FR touching 3-5 related files → 1-2 tasks
- 1 FR touching 5+ files across concerns → 2-4 tasks (split by concern)
- Never collapse all FRs into 1 mega-task
- Never split 1 FR into 5+ micro-tasks

## Output Format

__FORMAT_CONTRACT__
```

### 3.3 Почему промпт короче

| Причина | Детали |
|---------|--------|
| Нет AC classification | FR из PRD = implementation by definition. Нет "already implemented" или "manual" |
| Нет merge mode | Один PRD = одна сессия. Нет batch'ей |
| Нет batching | PRD (~10-30KB) влезает в контекст целиком с архитектурой |
| Нет "испорченного телефона" | PRD → Tasks напрямую, не PRD → Stories → Tasks |
| FR = atomic requirements | BMad stories группируют FR в user stories, потом bridge разгруппирует обратно. ralph plan пропускает эту стадию |

### 3.4 Codebase awareness

Критическое преимущество перед bridge: ralph plan может сканировать codebase:

```go
func (p *Planner) collectCodebaseContext(root string) string {
    // tree output (имена файлов/директорий, max 2 уровня)
    // go.mod (зависимости)
    // CLAUDE.md (правила проекта)
    // существующие sprint-tasks.md (merge mode)
}
```

Bridge не знал о codebase — только о stories. Plan видит и требования, и код, что даёт лучшую декомпозицию.

### 3.5 Итеративная стратегия (ADaPT в промпте)

Для сложных PRD (30+ FR) — двухфазный подход:

**Фаза 1 — Группировка:**
```
Read the PRD and group FRs into 3-7 logical epics by domain concern.
Output: list of epic names with FR numbers assigned to each.
Do NOT generate tasks yet.
```

**Фаза 2 — Декомпозиция по эпику:**
```
For Epic "{{.EpicName}}" containing FR-{{.FRList}}:
Generate sprint tasks following the format contract.
```

Это устраняет batching: вместо 6 параллельных Claude-вызовов, 1 вызов для группировки + N вызовов по эпикам (где N = 3-7). Каждый вызов компактный, контекст не теряется.

---

## 4. Сравнение с текущим подходом

| Критерий | Текущий (BMad + Bridge) | Вариант D (ralph plan) | Победитель |
|----------|------------------------|----------------------|------------|
| **Самодостаточность** | Нет — нужен BMad для stories | Да — PRD = единственный input | **D** |
| **Количество LLM-слоёв** | 3 (BMad → Bridge → Runner) | 2 (Plan → Runner) | **D** |
| **Потеря информации** | Высокая (story → 2 строки → source file) | Низкая (FR → task с полным контекстом) | **D** |
| **Детерминизм** | Низкий (LLM на каждом шаге) | Средний (LLM только для декомпозиции) | **D** |
| **Стоимость планирования** | ~$1-3 (bridge batches) | ~$0.10-0.30 (1-2 вызова) | **D** |
| **Время планирования** | 10-30 мин (bridge batches последовательно) | 1-3 мин (1-2 вызова) | **D** |
| **Human control** | Высокий (stories = review points) | Средний (план показывается, но нет story-level review) | **Текущий** |
| **Качество stories** | Высокое (BMad = специализированный workflow) | Нет stories (задачи = единственный уровень) | **Текущий** |
| **Traceability** | Stories → AC → Tasks | FR → Tasks | **Паритет** |
| **Codebase awareness** | Нет (bridge не знает код) | Да (plan сканирует проект) | **D** |
| **Scalability** | Проблема (batching) | Нет проблемы (PRD компактен) | **D** |
| **Incremental planning** | Merge mode (хрупкий) | Append mode (новые FR → новые tasks) | **D** |
| **Зрелость** | 10 epics, 92 FR, battle-tested | Концепция (нужна реализация) | **Текущий** |
| **Онбординг новых проектов** | Сложный (установить BMad, создать PRD, Architecture, Epics, Stories) | Простой (`ralph plan feature.md`) | **D** |

### Итого по баллам

- **Вариант D побеждает:** Самодостаточность, скорость, стоимость, codebase awareness, scalability, онбординг (7 из 13)
- **Текущий побеждает:** Human control, качество stories, зрелость (3 из 13)
- **Паритет:** Traceability, количество LLM-слоёв, incremental planning (3 из 13)

---

## 5. Варианты реализации (от минимального к максимальному)

### 5.1 Минимальный: `ralph plan` = модифицированный bridge

**Объём:** ~200 LOC, 1-2 дня.

Заменить `bridge.md` промпт на `plan.md` (работает с PRD вместо stories). Переиспользовать `bridge.Run()` с новым промптом. Не трогать runner.

```go
// planner/planner.go
func Plan(ctx context.Context, cfg *config.Config) error {
    prdContent := readPRD(cfg.PlanInput)
    archContent := readOptional(cfg.ArchitectureFile)
    codeTree := scanCodebase(cfg.ProjectRoot)

    prompt := assemblePrompt(prdContent, archContent, codeTree)
    result, err := session.Execute(ctx, session.Options{
        Prompt:    prompt,
        MaxTurns:  3,
    })

    return writeSprintTasks(result.Output, cfg.ProjectRoot)
}
```

**Плюсы:** Минимальные изменения, backward-compatible.
**Минусы:** Всё ещё LLM для форматирования, нет программного парсинга FR.

### 5.2 Средний: Программный парсинг FR + LLM для группировки

**Объём:** ~500 LOC, 3-5 дней.

1. Go-код парсит PRD: извлекает FR по regex/structure
2. Простые FR → задачи без LLM
3. Сложные FR → LLM группирует и декомпозирует
4. Программный форматировщик → sprint-tasks.md

```go
// planner/parser.go
type FR struct {
    ID          string   // "FR-42"
    Title       string
    Description string
    Complexity  int      // estimated by file count heuristic
}

func ParsePRD(content string) ([]FR, error) { ... }

// planner/planner.go
func Plan(ctx context.Context, cfg *config.Config) error {
    frs := ParsePRD(readPRD(cfg.PlanInput))

    var tasks []Task
    var complexFRs []FR

    for _, fr := range frs {
        if fr.Complexity <= 2 {
            tasks = append(tasks, simpleTask(fr))
        } else {
            complexFRs = append(complexFRs, fr)
        }
    }

    if len(complexFRs) > 0 {
        llmTasks := decomposeWithLLM(ctx, complexFRs, cfg)
        tasks = append(tasks, llmTasks...)
    }

    return writeSprintTasks(FormatTasks(tasks), cfg.ProjectRoot)
}
```

**Плюсы:** 80% задач без LLM, детерминизм, быстро.
**Минусы:** Нужен стабильный формат PRD для парсинга.

### 5.3 Максимальный: Полная pipeline-замена

**Объём:** ~1000 LOC, 1-2 недели.

1. `ralph plan` с тремя режимами (PRD / Issue / Interactive)
2. Программный парсинг + LLM декомпозиция
3. GitHub Issues интеграция (вдохновлено CCPM)
4. Interactive mode с уточняющими вопросами
5. `ralph plan --validate` для проверки плана без выполнения

---

## 6. Рекомендация

### Фаза 1: `ralph plan` (минимальный → средний вариант)

1. **Создать `planner/` пакет** с новым промптом, работающим с PRD напрямую
2. **Добавить `ralph plan <file>` команду** в cobra
3. **Переиспользовать** `session.Execute()` для Claude-вызова
4. **Сохранить** sprint-tasks.md формат и runner без изменений
5. **bridge** пометить как deprecated, не удалять

**Входной формат:** Любой markdown с требованиями. Не обязательно формальный PRD. Файл описания фичи, GitHub Issue body, даже bullet list.

### Фаза 2: Программный парсинг (когда формат PRD устаканится)

1. **ParsePRD()** для извлечения FR программно
2. **ADaPT-стратегия:** LLM только для сложных FR
3. **Codebase scan** для контекстной декомпозиции

### Фаза 3: GitHub Issues интеграция (Growth)

1. `ralph plan --from-issue` для работы с GitHub
2. `ralph plan --interactive` для discovery-сессий
3. Bidirectional sync: задачи ↔ issues

### Что НЕ делать

- **НЕ** пытаться встроить полный BMad workflow (create-epics, create-story, validate-story) в ralph — это overcomplicated. CCPM и claude-code-skills показывают, что 109 skills — это excess complexity
- **НЕ** убирать sprint-tasks.md — runner отлажен на нём, замена формата = переписывание runner
- **НЕ** делать interactive mode первым — PRD-to-tasks даёт 80% value за 20% effort
- **НЕ** делать "stories внутри ralph" — stories = промежуточный артефакт, который не нужен если есть PRD → Tasks

### Обоснование

Исследование ADaPT (Allen AI, NAACL 2024) показывает: as-needed decomposition на 28% эффективнее upfront planning. Текущий ralph делает максимальный upfront: PRD → Architecture → Epics → Stories → Tasks. Вариант D сокращает цепочку до PRD → Tasks.

Из 8 рассмотренных инструментов ни один не использует трёхслойную LLM-цепочку. Devin и Claude Code работают с одним уровнем планирования. CCPM и claude-code-skills (ближайшие аналоги) — тоже one-step от PRD к задачам.

Критический аргумент из bridge-concept-analysis: "Bridge создаёт ОГЛАВЛЕНИЕ для stories, а не новую информацию. Runner всё равно открывает source file для деталей." Если ralph plan включает FR-контекст прямо в задачу, runner не нуждается в промежуточном файле.

---

## 7. Источники

### Исследования
- [ADaPT: As-Needed Decomposition and Planning with Language Models](https://arxiv.org/abs/2311.05772) — Allen AI, NAACL 2024
- [Requirements are All You Need: From Requirements to Code with LLMs](https://arxiv.org/pdf/2406.10101) — structured requirements vs plain text
- [AI Agentic Programming: A Survey](https://arxiv.org/html/2508.11126v1) — обзор архитектур агентных систем
- [The Agentic Telephone Game: Cautionary Tale](https://www.christopheryee.org/blog/agentic-telephone-game-cautionary-tale/) — потеря информации в LLM-цепочках
- [Long-Running AI Agents and Task Decomposition 2026](https://zylos.ai/research/2026-01-16-long-running-ai-agents) — декомпозиция для долгих агентов

### Инструменты и документация
- [Devin 2.0 Interactive Planning](https://docs.devin.ai/work-with-devin/interactive-planning)
- [Devin 2.0 Technical Design](https://medium.com/@takafumi.endo/agent-native-development-a-deep-dive-into-devin-2-0s-technical-design-3451587d23c0)
- [Devin Performance Review 2025](https://cognition.ai/blog/devin-annual-performance-review-2025)
- [Claude Code Common Workflows](https://code.claude.com/docs/en/common-workflows)
- [Claude Code Task Management](https://claudefa.st/blog/guide/development/task-management)
- [Claude Code Plan Mode](https://claudelog.com/mechanics/plan-mode/)
- [PRD to Plan to Todo Workflow](https://developertoolkit.ai/en/claude-code/quick-start/prd-workflow/)
- [Claude Code for Product Managers](https://www.builder.io/blog/claude-code-for-product-managers)
- [GitHub Copilot Workspace](https://githubnext.com/projects/copilot-workspace)
- [GitHub Implementation Planner](https://docs.github.com/en/copilot/tutorials/customization-library/custom-agents/implementation-planner)
- [CCPM — Claude Code Project Manager](https://github.com/automazeio/ccpm)
- [claude-code-skills (levnikolaevich)](https://github.com/levnikolaevich/claude-code-skills)
- [Cursor vs Windsurf vs Claude Code 2026](https://dev.to/pockit_tools/cursor-vs-windsurf-vs-claude-code-in-2026-the-honest-comparison-after-using-all-three-3gof)
- [Windsurf Cascade Documentation](https://docs.windsurf.com/windsurf/cascade/cascade)
- [SWE-Agent](https://github.com/SWE-agent/SWE-agent)
- [OpenHands Agent SDK](https://docs.openhands.dev/sdk)
- [Agentic Coding — MIT Missing Semester 2026](https://missing.csail.mit.edu/2026/agentic-coding/)
- [My LLM Coding Workflow 2026 — Addy Osmani](https://addyosmani.com/blog/ai-coding-workflow/)
- [Augment Code — Devin Alternatives 2026](https://www.augmentcode.com/tools/best-devin-alternatives)
