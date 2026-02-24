# Ralphex: Глубокий анализ мульти-агентной системы ревью кода

> **Репозиторий**: [github.com/umputun/ralphex](https://github.com/umputun/ralphex)
> **Сайт**: [ralphex.com](https://ralphex.com/)
> **Документация**: [ralphex.com/docs](https://ralphex.com/docs/)
> **Автор**: umputun
> **Язык реализации**: Go
> **Лицензия**: MIT

## 1. Общая архитектура

Ralphex -- наиболее зрелая реализация Ralph Loop. Это CLI-инструмент, написанный на Go, который оркестрирует Claude Code для автономного выполнения планов реализации. Ключевое отличие от оригинального Ralph -- четырёхфазная система с мульти-агентным ревью кода.

### Принцип работы

```
Plan (markdown) --> Phase 1: Task Execution --> Phase 2: First Review (5 agents)
    --> Phase 3: External Review (Codex) --> Phase 4: Final Review (2 agents)
    --> [Optional] Finalize (rebase/squash)
```

Каждая фаза запускается в **свежей сессии Claude Code**. Это фундаментальное архитектурное решение -- контекст не переносится между фазами, что предотвращает деградацию качества из-за переполнения контекстного окна.

### Пакетная структура Go

```
ralphex/
├── cmd/ralphex/           # CLI точка входа
├── pkg/
│   ├── config/            # Конфигурация, загрузка промптов и агентов
│   │   ├── defaults/      # Встроенные дефолты (embed.FS)
│   │   │   ├── agents/    # 5 файлов агентов (.txt)
│   │   │   ├── prompts/   # 8 файлов промптов (.txt)
│   │   │   └── config     # Дефолтный конфиг
│   │   ├── agents.go      # Загрузка кастомных агентов
│   │   ├── config.go      # Основная конфигурация
│   │   ├── defaults.go    # Система встраивания дефолтов (embed)
│   │   ├── frontmatter.go # Парсинг YAML-фронтматтера агентов
│   │   ├── prompts.go     # Загрузка промптов с fallback-цепочкой
│   │   └── values.go      # Парсинг значений конфига
│   ├── executor/          # Исполнители CLI-команд
│   │   ├── executor.go    # ClaudeExecutor -- запуск Claude Code
│   │   ├── codex.go       # CodexExecutor -- запуск Codex
│   │   └── custom.go      # CustomExecutor -- кастомный скрипт ревью
│   ├── processor/         # Оркестрация фаз
│   │   ├── runner.go      # Главный оркестратор (Runner)
│   │   ├── prompts.go     # Построение промптов с подстановкой переменных
│   │   └── signals.go     # Сигналы завершения фаз
│   ├── git/               # Git-операции
│   ├── plan/              # Работа с планами
│   ├── progress/          # Логирование прогресса
│   ├── status/            # Общие типы и сигнальные константы
│   ├── notify/            # Уведомления
│   └── web/               # Веб-дашборд
```

## 2. Четырёхфазная система ревью

### Phase 1: Task Execution (Выполнение задач)

Каждая задача из плана выполняется в **отдельной свежей сессии** Claude Code. После каждой задачи запускаются команды валидации (тесты, линтеры), чекбоксы обновляются, изменения коммитятся.

Ключевое ограничение промпта задачи:

> "Complete ONE Task section per iteration... Do NOT continue to the next section - the external loop will call you again."

Это означает, что внешний Go-цикл (`Runner.runFull()`) вызывает Claude Code повторно для каждой задачи, а не полагается на внутренний цикл Claude.

### Phase 2: First Code Review (Первое ревью -- 5 агентов)

После завершения всех задач запускается комплексное ревью. **5 специализированных агентов** запускаются **параллельно** через механизм Task tool Claude Code.

Промпт `review_first.txt` содержит следующую структуру:

```
Code review of: {{GOAL}}
Progress log: {{PROGRESS_FILE}}

## Step 1: Get Branch Context
- git log {{DEFAULT_BRANCH}}..HEAD --oneline
- git diff {{DEFAULT_BRANCH}}...HEAD

## Step 2: Launch ALL 5 Review Agents IN PARALLEL
All Task tool calls MUST be in the same message for parallel foreground execution.
Do NOT use run_in_background.

Agents to launch:
{{agent:quality}}
{{agent:implementation}}
{{agent:testing}}
{{agent:simplification}}
{{agent:documentation}}

## Step 3: Collect, Verify, and Fix Findings
### 3.1 Collect and Deduplicate
### 3.2 Verify EVERY Finding (CRITICAL)
### 3.3 Fix All Confirmed Issues

## Step 4: Signal Completion
Path A - NO confirmed issues: <<<RALPHEX:REVIEW_DONE>>>
Path B - Issues found AND fixed: STOP (no signal, loop re-runs)
Path C - Cannot fix: <<<RALPHEX:TASK_FAILED>>>
```

**Механизм параллельности**: промпт явно указывает, что все Task tool вызовы должны быть в одном сообщении. Claude Code нативно поддерживает параллельный запуск Task tool -- foreground агенты работают одновременно и блокируют до завершения всех. Не используется `run_in_background`.

**Итерационная логика**: реализована в `runClaudeReviewLoop()`:

```
maxReviewIterations = max(3, MaxIterations/10)
for i := 0; i < maxReviewIterations; i++ {
    headBefore = git HEAD hash
    result = runClaudeReview(prompt)
    if IsReviewDone(result.Signal) → break  // агенты ничего не нашли
    if headAfter == headBefore → break       // ничего не изменилось
    sleep(iterationDelay)
}
```

Если Claude нашёл и исправил проблемы (Path B), он не отправляет сигнал -- внешний цикл запускает новую итерацию ревью для верификации исправлений. Цикл завершается когда:
- Все агенты рапортуют "чисто" (сигнал `REVIEW_DONE`)
- Не произошло изменений (сравнение хеша HEAD до и после)
- Достигнут лимит итераций

### Phase 3: External Review (Codex)

Независимый внешний инструмент анализирует код. По умолчанию это Codex (GPT-5), но можно настроить кастомный скрипт.

Конфигурация:
```
external_review_tool = codex|custom|none
codex_model = gpt-5.3-codex
codex_reasoning_effort = xhigh
codex_sandbox = read-only
codex_timeout_ms = 3600000
```

**Цикл codex-ревью** реализован в `runExternalReviewLoop()`:

```
maxIterations = max(3, MaxIterations/5)
for i := 0; i < maxIterations; i++ {
    codexResult = runExternalTool(prompt)     // Codex анализирует diff
    displayFindings(codexResult)               // показать находки
    claudeResult = runClaudeEval(codexResult)  // Claude оценивает
    if IsCodexDone(claudeResult.Signal) → break
    sleep(iterationDelay)
}
```

Промпт `codex.txt` для Claude-оценки находок Codex:

> "For EACH Codex finding: Read the reported code location, trace execution flow, check the plan file for intentional design decisions. Classify as valid or invalid. Fix valid issues. Only output CODEX_REVIEW_DONE when Codex itself reports no actionable issues."

Ключевая идея -- **две AI-перспективы**: Codex находит проблемы, Claude верифицирует и исправляет. Итерации продолжаются пока Codex находит что-то новое.

### Phase 4: Second Code Review (Финальное ревью -- 2 агента)

Запускаются только 2 агента: `quality` и `implementation`. Фокус исключительно на критических и мажорных проблемах.

Промпт `review_second.txt`:

```
## Step 2: Launch 2 Review Agents IN PARALLEL
{{agent:quality}}
{{agent:implementation}}

Focus EXCLUSIVELY on critical and major issues.
Ignore: style, minor improvements, preferences.
```

Итерационная логика аналогична Phase 2 (`runClaudeReviewLoop`), но с меньшим фокусом -- это предотвращает "усталость от итераций" при сохранении отлова критических проблем.

### (Optional) Finalize

Опциональный шаг (по умолчанию отключён). Выполняет rebase на последний default branch, squash коммитов, финальный запуск тестов.

## 3. Пять агентов ревью -- детальный анализ

### Архитектура агентов

Агенты -- это текстовые файлы (`.txt`) в директории `~/.config/ralphex/agents/` (или встроенные дефолты). Каждый агент определяет инструкции для sub-agent, который запускается через Task tool.

Система экспансии `{{agent:name}}` преобразует ссылку в инструкцию для Task tool:

```go
// из prompts.go
func (r *Runner) formatAgentExpansion(prompt string, opts config.Options) string {
    subagent := "general-purpose"
    if opts.AgentType != "" {
        subagent = opts.AgentType
    }
    var modelClause string
    if opts.Model != "" {
        modelClause = " with model=" + opts.Model
    }
    return fmt.Sprintf(`Use the Task tool%s to launch a %s agent with this prompt:
"%s"
Report findings only - no positive observations.`, modelClause, subagent, prompt)
}
```

Каждый агент поддерживает YAML-фронтматтер для per-agent настроек:

```yaml
---
model: haiku
agent: code-reviewer
---
```

### Agent 1: Quality (Качество)

Анализирует четыре аспекта:

1. **Correctness Review**: логические ошибки, off-by-one, неправильные условия, операторы, edge cases, обработка ошибок, управление ресурсами, проблемы конкурентности, целостность данных
2. **Security Analysis**: валидация ввода, аутентификация/авторизация, injection-уязвимости, хардкоженные секреты, утечка информации
3. **Simplicity Assessment**: обоснованность сложных паттернов (фабрики, билдеры), преждевременная оптимизация, расползание scope
4. **Reporting Format**: файл:строка, описание, импакт, конкретное исправление

### Agent 2: Implementation (Реализация)

Пять доменов анализа:

1. **Requirement coverage**: покрытие всех требований, edge cases
2. **Approach correctness**: методология решения, устойчивость к разным условиям
3. **Integration quality**: связность компонентов, маршруты, конфигурация
4. **Completeness**: отсутствующие части для функциональной работы
5. **Logic and flow**: трансформация данных, управление состоянием

Приоритет -- корректность над стилем.

### Agent 3: Testing (Тестирование)

1. **Coverage Analysis**: отсутствующие тесты, непротестированные пути ошибок, интеграционное тестирование
2. **Quality Evaluation**: тесты проверяют поведение, а не имплементацию; независимость; описательные имена
3. **Fake Test Detection**: тесты с хардкоженными значениями вместо реального вывода, игнорирование ошибок
4. **Independence Verification**: отсутствие shared state, setup/teardown, зависимости от порядка
5. **Edge Case Assessment**: пустые входы, null-значения, граничные условия, конкурентный доступ, таймауты

### Agent 4: Simplification (Упрощение)

Шесть категорий over-engineering:

1. **Excessive Abstraction Layers**: wrapper-методы с идентичной сигнатурой, фабрики с одной реализацией, "layer cake" паттерны
2. **Premature Generalization**: event bus для одного типа событий, конфиг-объекты для 2-3 опций
3. **Unnecessary Indirection**: pass-through обёртки, избыточный chaining, оборачивание примитивов
4. **Future-Proofing Excess**: неиспользуемые точки расширения, версионированные API с одной версией
5. **Unnecessary Fallbacks**: fallback-и которые никогда не срабатывают, disabled legacy-пути
6. **Premature Optimization**: кэширование редко используемых данных, кастомные структуры данных

### Agent 5: Documentation (Документация)

Два уровня документации:

1. **README.md** (User-Facing): новые фичи, CLI-флаги, API-эндпоинты, конфигурация, breaking changes
2. **CLAUDE.md** (Developer Knowledge): архитектурные паттерны, конвенции, команды сборки/тестирования, интегрированные инструменты

Пропускает: внутренний рефакторинг, баг-фиксы восстанавливающие документированное поведение, добавление тестов, стилевые изменения.

## 4. Механизм запуска агентов: Task Tool, а не отдельные сессии

**Критически важная архитектурная деталь**: агенты ревью запускаются НЕ как отдельные сессии Claude Code и НЕ через resume. Они используют **Task tool** -- нативный механизм Claude Code для запуска sub-agent внутри текущей сессии.

```
┌─────────────────────────────────────────────────┐
│  Claude Code Session (review_first prompt)       │
│                                                   │
│  1. git log / git diff (понять контекст)         │
│                                                   │
│  2. Task tool x5 (ПАРАЛЛЕЛЬНО):                 │
│     ├── quality agent (sub-agent)                │
│     ├── implementation agent (sub-agent)         │
│     ├── testing agent (sub-agent)                │
│     ├── simplification agent (sub-agent)         │
│     └── documentation agent (sub-agent)          │
│                                                   │
│  3. Собрать, верифицировать, исправить            │
│  4. Сигнал завершения                            │
└─────────────────────────────────────────────────┘
```

Каждый Task tool вызов создаёт sub-agent с собственным контекстом, но в рамках одной сессии. Агенты не видят выход друг друга -- они работают изолированно и параллельно. Главная сессия собирает результаты всех агентов и принимает решения.

### Процесс экспансии `{{agent:name}}`

1. Промпт `review_first.txt` содержит `{{agent:quality}}`
2. Go-код `expandAgentReferences()` находит паттерн через regexp `\{\{agent:([a-zA-Z0-9_-]+)\}\}`
3. Загружает содержимое файла `quality.txt` из агентов
4. Подставляет переменные в промпт агента (`replaceBaseVariables`)
5. Оборачивает в Task tool инструкцию через `formatAgentExpansion()`
6. Итоговый текст: `Use the Task tool to launch a general-purpose agent with this prompt: "..."`

## 5. Как обрабатываются находки ревью

### Принцип "Тот же агент исправляет"

Находки обрабатываются **той же сессией Claude**, которая проводила ревью. Нет отдельного "fixer-агента". Процесс:

1. Sub-агенты ревью находят проблемы
2. Главная сессия собирает и дедуплицирует находки
3. Главная сессия **верифицирует каждую находку** -- читает реальный код, проверяет контекст
4. Классифицирует: CONFIRMED или FALSE POSITIVE
5. Исправляет все CONFIRMED проблемы
6. Запускает тесты/линтер для верификации
7. Коммитит: `git commit -m "fix: address code review findings"`
8. **Не отправляет сигнал завершения** (Path B)
9. Внешний Go-цикл запускает НОВУЮ сессию ревью для проверки исправлений

### Итерационный цикл

```
Iteration 1: 5 агентов → 12 находок → 8 confirmed → исправить → коммит → нет сигнала
Iteration 2: 5 агентов → 3 находки (новые из-за фиксов) → 2 confirmed → исправить → коммит
Iteration 3: 5 агентов → 0 находок → сигнал REVIEW_DONE → выход из цикла
```

Максимальное количество итераций: `max(3, MaxIterations/10)` для ревью, `max(3, MaxIterations/5)` для codex.

### Обработка pre-existing проблем

Промпт явно указывает:

> "Pre-existing issues (linter errors, failed tests) should also be fixed. Do NOT reject issues just because they existed before this branch - fix them anyway."

Это означает, что ревью исправляет не только проблемы текущей ветки, но и существующие баги, обнаруженные в процессе.

## 6. External Review (Codex) -- детали

### Codex Executor

```go
// Конфигурация по умолчанию:
// Command: "codex"
// Model: "gpt-5.3-codex"
// Reasoning: "xhigh"
// Sandbox: "read-only" (или "danger-full-access" в Docker)
```

Codex запускается как отдельный процесс. Stderr стримится для индикации прогресса, stdout захватывается как финальный ответ.

### Двухшаговый цикл

```
┌──────────────────┐     ┌─────────────────────┐
│ Codex анализирует │────>│ Claude оценивает     │
│ git diff          │     │ находки Codex        │
│ (GPT-5)           │     │ (Claude Code)        │
└──────────────────┘     └──────┬──────────────┘
                                │
                    ┌───────────┴───────────┐
                    │ Valid? → Fix + commit  │
                    │ Invalid? → Explain why │
                    │ Empty? → CODEX_DONE    │
                    └───────────────────────┘
```

### Custom Review Tool

Вместо Codex можно подключить любой скрипт. Интерфейс:
- Скрипт получает файл с промптом
- Выводит находки в формате: `file:line - description of issue`
- Claude оценивает и исправляет

Промпт для кастомного скрипта (`custom_review.txt`):

```
You are reviewing code changes for: {{GOAL}}

## Get the Diff
Run this command to see the changes:
{{DIFF_INSTRUCTION}}

## Review Focus
1. Bugs and logic errors
2. Security issues
3. Race conditions
4. Error handling
5. Test coverage
6. Code quality

## Output Format
- file:line - description of issue
If no issues found, output: NO ISSUES FOUND
```

## 7. Система конфигурации

### Иерархия конфигурации

```
Приоритет: CLI flags > Local (.ralphex/config) > Global (~/.config/ralphex/config) > Embedded defaults
```

### Загрузка агентов -- "Replace Entire Set"

Агенты загружаются как **цельный набор**, а не per-file fallback:

1. Есть локальные агенты (`.ralphex/agents/`)? → Использовать ТОЛЬКО их
2. Есть глобальные (`~/.config/ralphex/agents/`)? → Использовать ТОЛЬКО их
3. Иначе → встроенные дефолты

Обоснование из кода:

> "agents define the review strategy as a cohesive set, so partial mixing would create unpredictable review behavior."

### Загрузка промптов -- per-file fallback

Промпты, в отличие от агентов, используют per-file fallback цепочку: local -> global -> embedded.

### Ключевые настройки конфигурации

```ini
# Claude Executor
claude_command = claude
claude_args = --dangerously-skip-permissions --output-format stream-json --verbose

# Codex Executor
codex_enabled = true
codex_command = codex
codex_model = gpt-5.3-codex
codex_reasoning_effort = xhigh
codex_timeout_ms = 3600000
codex_sandbox = read-only

# External Review Tool
external_review_tool = codex    # codex | custom | none
custom_review_script =          # path to custom script

# Iteration Control
iteration_delay_ms = 2000
task_retry_count = 1

# Paths
plans_dir = docs/plans
default_branch =                # auto-detect

# Post-completion
finalize_enabled = false

# Notifications
notify_channels =               # telegram, slack, email, webhook, custom
```

### Шаблонные переменные в промптах

```
{{PLAN_FILE}}        - путь к файлу плана
{{PROGRESS_FILE}}    - файл лога прогресса
{{GOAL}}             - описание цели
{{DEFAULT_BRANCH}}   - ветка по умолчанию
{{PLANS_DIR}}        - директория планов
{{DIFF_INSTRUCTION}} - команда git diff (зависит от итерации)
{{agent:name}}       - экспансия агента в Task tool инструкцию
{{CODEX_OUTPUT}}     - вывод Codex для оценки
{{PLAN_DESCRIPTION}} - описание для создания плана
{{CUSTOM_OUTPUT}}    - вывод кастомного скрипта
```

### YAML-фронтматтер агентов

```yaml
---
model: haiku          # переопределить модель для этого агента
agent: code-reviewer  # тип sub-agent
---
Тело промпта агента...
```

Парсинг фронтматтера из `frontmatter.go`:

```go
// parseOptions ожидает разделители ---
// Поддерживается только YAML с '---' разделителями
// Модель нормализуется: "claude-sonnet-4-5-20250929" → "sonnet"
func normalizeModel(model string) string {
    // извлекает ключевые слова: haiku, sonnet, opus
}
```

## 8. Сигнальная система

Ralphex использует текстовые сигналы в формате `<<<RALPHEX:...>>>` для обнаружения состояния фаз:

```go
// pkg/status/status.go
const (
    Completed  = "<<<RALPHEX:ALL_TASKS_DONE>>>"
    Failed     = "<<<RALPHEX:TASK_FAILED>>>"
    ReviewDone = "<<<RALPHEX:REVIEW_DONE>>>"
    CodexDone  = "<<<RALPHEX:CODEX_REVIEW_DONE>>>"
    Question   = "<<<RALPHEX:QUESTION>>>"
    PlanReady  = "<<<RALPHEX:PLAN_READY>>>"
    PlanDraft  = "<<<RALPHEX:PLAN_DRAFT>>>"
)
```

Фазы для цветовой кодировки:

```go
const (
    PhaseTask       Phase = "task"        // зелёный
    PhaseReview     Phase = "review"      // циан
    PhaseCodex      Phase = "codex"       // маджента
    PhaseClaudeEval Phase = "claude-eval" // яркий циан
    PhasePlan       Phase = "plan"        // инфо
    PhaseFinalize   Phase = "finalize"    // зелёный
)
```

## 9. ClaudeExecutor -- как запускается Claude Code

```go
// pkg/executor/executor.go
type ClaudeExecutor struct {
    Command       string   // "claude"
    Args          string   // доп. аргументы
    OutputHandler func(text string)
    Debug         bool
    ErrorPatterns []string // паттерны ошибок (rate limit и т.д.)
}

func (e *ClaudeExecutor) Run(ctx context.Context, prompt string) Result {
    args := []string{
        "--dangerously-skip-permissions",
        "--output-format", "stream-json",
        "--verbose",
    }
    args = append(args, "-p", prompt)
    // запускает claude через os/exec
    // парсит JSON-стрим
    // детектит сигналы в выходе
}
```

Ключевые детали:
- Используется `--output-format stream-json` для парсинга событий
- `ANTHROPIC_API_KEY` явно фильтруется из окружения (Claude Code использует свою аутентификацию)
- Создаётся отдельная process group для корректного kill при отмене
- Поддерживаются error patterns для обнаружения rate limit и подобных проблем

## 10. Режимы работы

```go
type Mode int
const (
    ModeFull      // Tasks + все фазы ревью
    ModeReview    // Только ревью (Phase 2 → 3 → 4)
    ModeCodexOnly // Только внешнее ревью (Phase 3 → 4)
    ModeTasksOnly // Только выполнение задач (Phase 1)
    ModePlan      // Интерактивное создание плана
)
```

**Review-only mode** (`--review`) полезен когда изменения сделаны вне ralphex -- через Claude Code plan mode, ручные правки, другие AI-агенты.

## 11. Сводная таблица архитектуры ревью

| Аспект | Реализация |
|--------|-----------|
| Количество агентов | 5 (Phase 2) + 2 (Phase 4) |
| Запуск агентов | Task tool Claude Code (sub-agents) |
| Параллельность | Да, все агенты в одном сообщении |
| Свежие сессии | Да, между фазами. Sub-агенты внутри одной сессии |
| Кто исправляет | Та же сессия Claude, что проводила ревью |
| Итерации ревью | max(3, MaxIterations/10) для Claude, max(3, MaxIterations/5) для Codex |
| Сигнализация | Текстовые маркеры <<<RALPHEX:...>>> |
| Внешнее ревью | Codex (GPT-5) или кастомный скрипт |
| Кастомизация | Полная -- агенты, промпты, конфиг |
| Fallback стратегия | Агенты: replace entire set. Промпты: per-file fallback |

## 12. Ключевые выводы для реализации

### Что делает ralphex правильно

1. **Свежие сессии между фазами** -- предотвращает контекстную деградацию
2. **Task tool для sub-agents** -- нативный механизм Claude, не хак
3. **Явная верификация находок** -- агенты находят, главная сессия проверяет каждую
4. **Итерационный цикл с commit-detection** -- элегантное условие выхода
5. **Две AI-перспективы** (Claude + Codex) -- разные модели ловят разные баги
6. **Фокусированное финальное ревью** (2 агента, только critical/major) -- предотвращает бесконечные итерации
7. **Полная кастомизация** -- можно заменить любой агент или промпт

### Архитектурные решения для адаптации

1. Агенты как текстовые файлы с YAML-фронтматтером -- простой и расширяемый формат
2. Шаблонные переменные (`{{agent:name}}`, `{{DIFF_INSTRUCTION}}`) -- мощная система подстановки
3. Сигнальная система через текстовые маркеры -- надёжное обнаружение состояния без парсинга сложных структур
4. Разделение "strategy as cohesive set" для агентов vs per-file fallback для промптов
5. Process group management для корректной очистки при отмене

---

*Исследование проведено 2026-02-24 на основе анализа исходного кода [umputun/ralphex](https://github.com/umputun/ralphex), документации [ralphex.com](https://ralphex.com/) и [ralphex.com/docs](https://ralphex.com/docs/).*
