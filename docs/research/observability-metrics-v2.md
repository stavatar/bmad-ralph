# Исследование: Метрики и наблюдаемость для AI-оркестраторов разработки (v2)

> Дата: 2026-03-04 | Автор: Claude Opus 4.6 | Проект: bmad-ralph

## 1. Введение

AI-оркестраторы разработки -- класс инструментов, автономно выполняющих задачи кодирования через LLM-сессии -- находятся на критическом этапе зрелости. Когда CLI-инструмент самостоятельно читает задачи, генерирует код, делает коммиты и проводит ревью, **отсутствие наблюдаемости превращает автономность в непредсказуемость**. Оператор не может ответить на базовые вопросы: почему задача заняла 47 минут? сколько tokens израсходовано на ревью vs. написание кода? какой процент retry привел к успеху?

### Текущее состояние ralph

Ralph использует `RunLogger` -- структурированный логгер с тремя уровнями (`INFO`, `WARN`, `ERROR`) и key=value форматом, пишущий в файл и stderr одновременно. Текущие точки инструментации покрывают основные lifecycle events:

- **Run lifecycle**: `run started` (tasks_file), `run finished` (status, error)
- **Task lifecycle**: `task started` (task, index, total), `task completed` (task, duration)
- **Session tracking**: `execute session started/finished` (task, attempt, duration, exit_code)
- **Review tracking**: `review session started/finished` (task, clean/dirty, duration)
- **Git checks**: `commit check` (found/not_found, before/after SHA)
- **Retry analytics**: `retry scheduled` (task, attempt, reason), `execute attempts exhausted`
- **Recovery**: `dirty state detected`, `dirty state recovered`
- **Distillation**: `distillation triggered`, `learnings budget check` (lines, near_limit)

Помимо `RunLogger`, система `DistillMetrics` отслеживает метрики дистилляции знаний: `entries_before/after`, `stale_removed`, `categories_preserved/total`, `t1_promotions`.

**Ключевые пробелы**: нет token tracking (Claude Code не экспортирует напрямую в key=value), нет latency breakdown по фазам, нет агрегации метрик между запусками, нет alerting/budget enforcement, нет structured export для внешних систем (только текстовые логи), нет gate decision distribution, нет review findings severity breakdown.

## 2. Ландшафт конкурентов

### Сравнительная таблица метрик

| Метрика | [Aider](https://aider.chat) | [SWE-agent](https://swe-agent.com) | [Devin](https://devin.ai) | [Cline](https://cline.bot) | [Codex CLI](https://github.com/openai/codex) | **ralph** |
|---|---|---|---|---|---|---|
| Token usage (in/out) | Да, per-model | Да, per-turn | Да (cloud dashboard) | Да, VS Code panel | Запрошено ([#5085](https://github.com/openai/codex/issues/5085)) | **Нет** |
| Cost tracking | Да, cumulative | Нет | Да, per-session | Да, real-time | Нет | **Нет** |
| Git diff stats | Да, blame-based | Да, per-patch | Да | Нет | Нет | **Частично** (SHA before/after) |
| Latency breakdown | Нет | Нет | Да (timeline) | Нет | Нет | **Частично** (session duration) |
| Retry/loop metrics | Нет | Да (step count) | Да | Нет | Нет | **Да** (attempt count, reason) |
| Review quality | Нет | Нет | Нет | Нет | Нет | **Да** (clean/dirty, cycle count) |
| Multi-model comparison | Да (benchmark suite) | Да (SWE-bench) | Нет | Нет | Нет | **Нет** |
| Export format | Text, `--analytics` | JSON, trajectories | REST API | JSON | JSON (`--output-format`) | **Text logs** |
| Budget alerts | Нет | Нет | Да | Да (configurable) | Нет | **Нет** |
| Gate/approval tracking | Нет | Нет | Да (approval flow) | Нет | Нет | **Частично** (ad-hoc) |

### Ключевые наблюдения

1. **Token tracking -- table stakes**. [Aider](https://aider.chat) -- золотой стандарт для CLI token tracking: каждый запрос логирует `prompt_tokens`, `completion_tokens`, `cached_tokens`, стоимость по модели, и git-attribution через blame. [OpenAI Codex CLI](https://github.com/openai/codex) получил [feature request #5085](https://github.com/openai/codex/issues/5085) именно на cost tracking. [Cline](https://cline.bot/blog/top-6-claude-code-alternatives-for-agentic-coding-workflows-in-2025) предоставляет per-task budget alerts в VS Code. Ralph не может получить tokens напрямую из key=value stderr, но может парсить `--output-format json` output.

2. **Review quality tracking -- уникальное преимущество ralph**. Ни один конкурент не отслеживает clean/dirty review cycles, severity distribution, recurrence patterns. [Devin](https://devin.ai) (после [приобретения Windsurf/Codeium за $250M](https://www.cognition.ai/blog/cognition-acquires-windsurf)) строит проприетарную observability, но без review feedback loop. Это дифференциатор, который стоит развивать.

3. **Structured export -- минимальный порог**. Все серьезные инструменты предлагают JSON export. `RunLogger` пишет key=value текст, что затрудняет агрегацию и визуализацию. Claude Code поддерживает `--output-format json` с usage data, `session_id`, `num_turns`.

4. **Git integration depth**. Aider лидирует с blame-based contribution stats (процент кода, написанного AI). [Antigravity](https://blog.google/technology/google-labs/project-mariner-antigravity/) (Google) предлагает manager view для multi-agent orchestration. Ralph отслеживает SHA before/after, но не diff stats (+/- lines, files changed, affected packages).

## 3. Таксономия метрик

### 3.1 Производительность и runtime

**Token consumption** -- фундаментальная метрика стоимости и эффективности AI-оркестратора. [OpenTelemetry GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) определяют атрибуты `gen_ai.usage.input_tokens` и `gen_ai.usage.output_tokens` как индустриальный стандарт.

- **Input/output/cache tokens per LLM call**: Claude Code поддерживает `--output-format json`, где response содержит usage statistics. Ralph вызывает Claude через `session.Execute()` с `Options{OutputJSON: true}` и получает `RawResult{Stdout, Stderr, ExitCode}`. Парсинг JSON stdout дает token counts без модификации Claude Code.
- **Tokens per phase**: разделение на execute-tokens vs. review-tokens vs. resume-extraction-tokens. Каждый вызов `session.Execute()` -- отдельная точка измерения. Это позволяет ответить на вопрос "сколько стоит ревью vs. написание кода?"
- **Cache hit ratio**: Claude поддерживает prompt caching; отношение `cache_read_input_tokens / total_input_tokens` показывает эффективность повторного использования контекста. Высокий ratio = хорошо структурированные промпты.
- **Tool call count per session**: сколько раз Claude использовал Read/Write/Bash внутри сессии (доступно в JSON output `num_turns`).

**Latency breakdown** -- где тратится время в ralph loop. Текущий `RunLogger` логирует только общий `duration` сессии, что недостаточно для диагностики:

| Фаза | Текущее измерение | Предлагаемое |
|---|---|---|
| Prompt assembly (`buildTemplateData`) | Нет | `prompt_build_ms` |
| LLM session (`session.Execute`) | Да (duration) | Сохранить, добавить `first_token_ms` |
| Git operations (`HeadCommit`, `HealthCheck`) | Нет | `git_check_ms` |
| Review session (`RunReview`) | Да (duration) | Добавить `review_prompt_ms` + `review_llm_ms` |
| Gate wait (human input) | Нет | `gate_wait_ms` (wall-clock до ответа) |
| Distillation | Нет | `distill_ms` |
| Sleep/backoff (`SleepFn`) | Нет | `backoff_total_ms` |

**Git diff metrics**: `git diff --stat HEAD~1..HEAD` после коммита дает files changed, insertions, deletions. `git diff --name-only` дает affected packages. Текущий `runner.Git` interface имеет `HeadCommit()` и `HealthCheck()` -- добавление `DiffStat()` расширит без нарушения dependency direction (config остается leaf package).

**Retry analytics**: ralph уже логирует attempt count и reason. Дополнительно полезны: backoff distribution (histogram задержек между retry), success-after-retry rate (процент retry, завершившихся успехом), причины по категориям (no commit vs. dirty review vs. gate retry).

### 3.2 Качество и корректность

**Test pass rate delta** -- метрика, которую ralph может собирать через post-commit hook или отдельный вызов. При запуске `go test ./...` до и после задачи: delta = pass_after - pass_before. [Anthropic's eval methodology](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) использует SWE-bench подход: fail-to-pass (новые тесты проходят) + pass-to-pass (существующие не сломаны).

**Build success/failure tracking**: ralph проверяет exit code сессии (`RawResult.ExitCode`). Дополнительно: tracking `go build` success rate, compilation errors по категориям (type mismatch, undefined, import cycle).

**Review findings distribution**: ralph уже отслеживает clean/dirty через `DetermineReviewOutcome`. Расширение:
- **Count by severity**: CRITICAL/HIGH/MEDIUM/LOW per review cycle -- ключевая quality metric
- **Finding types**: assertion quality, doc comment accuracy, error wrapping (top recurring паттерны из Epic 2-6 retros: 226 findings across 40 stories)
- **Recurrence rate**: процент findings, повторяющих ранее зафиксированный паттерн (из `.claude/rules/`)
- **Fix rate**: процент findings, исправленных в том же run (текущий rate: 100% across Epics 4-6)

**Code complexity delta**: cyclomatic complexity до и после задачи (через `gocyclo`). [SigNoz](https://signoz.io/comparisons/llm-observability-tools/) рекомендует tracking complexity trends для обнаружения code degradation.

**Coverage delta**: `go test -coverprofile` до и после. Разница показывает, добавляет ли AI-сгенерированный код тесты пропорционально production code. Текущий target: runner и config > 80%.

### 3.3 Экономика и стоимость

Экономические метрики -- ключевой драйвер adoption для enterprise-пользователей. [Braintrust](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026) предоставляет per-request cost breakdowns с tag-based attribution, позволяя группировать расходы по задачам и проектам. [Traceloop](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user) реализует per-user token/cost tracking через OpenTelemetry spans. [Portkey](https://portkey.ai/blog/tracking-llm-token-usage-across-providers-teams-and-workloads/) обеспечивает cross-provider tracking.

- **Cost per task**: `(input_tokens * input_price + output_tokens * output_price)` суммарно за все сессии задачи (execute + review + resume extraction). Ralph поддерживает разные модели для execute (`ModelExecute`) и review (`ModelReview`) в `config.Config` -- стоимость разная.
- **Cost per review cycle**: изолированная стоимость ревью; при среднем 2-3 review cycles на задачу это существенная доля общей стоимости
- **Cost per retry**: стоимость каждого повторного execute после failed commit check. Высокая стоимость retry = неэффективный промпт
- **Token efficiency**: `total_tokens / lines_changed` -- нормализованная метрика для сравнения задач разной сложности
- **Budget tracking**: running total с configurable threshold и alert. Реализация: аккумулятор в `Runner` struct, проверка после каждого `session.Execute()`, WARN в RunLogger при достижении 80%
- **Model cost comparison**: при поддержке нескольких моделей (Sonnet vs Opus) -- сравнение cost/quality tradeoff на одном типе задач

### 3.4 Здоровье цикла (loop health)

Loop health метрики критичны для **автономной** работы ralph, когда оператор не следит за процессом в реальном времени. [Langfuse](https://langfuse.com/docs/observability/overview) предоставляет end-to-end tracing с визуализацией agent loops.

**Loop/stuck detection**. Текущий механизм: ralph использует `MaxIterations` и `MaxReviewIterations` как жесткие лимиты. Индустриальные подходы:
- **Window-based similarity**: сравнение последних N=5 diffs на similarity > 0.95 (одинаковые изменения = застревание). [SWE-agent](https://swe-agent.com) использует аналогичный подход с лимитом 8 parallel tool calls per turn
- **Output hash ring**: хранение hash последних K outputs; совпадение = loop. Дешевле similarity, но пропускает near-duplicates
- **Monotonicity check**: если test pass count не растет 3+ итерации подряд, сигнал стагнации

**Circuit breaker triggers and outcomes**: dual-threshold model -- warning threshold (log + inject hint в prompt) и hard threshold (force stop + save state). Метрика: сколько раз лимит достигнут vs. задача завершена нормально.

**Gate decision distribution**: ralph имеет `gates.Prompt()` с `Gate{TaskText, Reader, Writer, Emergency}` и actions approve/retry/skip/quit. Метрики:
- Distribution: `{approve: 73%, retry: 15%, skip: 8%, quit: 4%}` -- показывает доверие оператора к AI
- **Approve rate trend**: растущий approve rate = улучшение качества; падающий = деградация
- **Time-to-decision**: wall-clock от prompt до ответа (proxy для сложности решения)

**Emergency gate frequency**: `Gate.Emergency = true` -- аварийное прерывание loop (review exhaustion, execute exhaustion). Частота > 5% = системная проблема в промптах или конфигурации.

### 3.5 Восстановление и устойчивость

Ralph имеет развитую систему восстановления: `RecoverDirtyState`, `ResumeExtraction`, distillation state management с `RecoverDistillation`. Метрики устойчивости:

- **Dirty state recovery rate**: `successful_recoveries / dirty_state_detections`. RunLogger логирует оба события, но не агрегирует
- **Resume extraction effectiveness**: процент случаев, когда extracted resume привел к успешному завершению задачи. Требует correlation между resume event и последующим task completion
- **Error categorization**: ralph оборачивает ошибки с prefix pattern (`runner: startup:`, `runner: execute:`, `runner: read tasks:`). Агрегация по prefix дает top error categories
- **Auto-recovery success rate**: процент ошибок, восстановленных без оператора
- **Distillation metrics**: `DistillMetrics` уже содержит `entries_before/after`, `stale_removed`, `t1_promotions`. Дополнительно: distillation duration, validation failures, backup/restore events
- **Mean time to recovery (MTTR)**: от ошибки до successful resume -- ключевой SRE-метрика, адаптированная для AI loop

## 4. Индустриальные стандарты

### OpenTelemetry GenAI Semantic Conventions

[OpenTelemetry GenAI](https://opentelemetry.io/docs/specs/semconv/gen-ai/) определяет стандартные span types и атрибуты для AI-систем:

- **`gen_ai.client` spans**: оборачивают один LLM вызов. Атрибуты: `gen_ai.system` ("anthropic"), `gen_ai.request.model`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.response.finish_reason`. Прямо маппятся на `session.Execute()` в ralph.
- **`gen_ai.agent` spans** (development status): оборачивают агентный workflow из нескольких tool calls и LLM invocations. Маппятся на полный цикл task в ralph (execute + commit check + review + gate).
- **Events**: `gen_ai.choice`, `gen_ai.tool.message` -- можно эмитить из `RunLogger` при переходе на OTLP export.

### MELT Framework (Metrics, Events, Logs, Traces)

| Pillar | Описание | Статус в ralph | Gap |
|---|---|---|---|
| **Metrics** | Counters, gauges, histograms | Нет | Нет aggreagated counters (total_tokens, total_tasks), нет histograms (session_duration distribution) |
| **Events** | Дискретные структурированные факты | Частично | RunLogger key=value = events, но нет structured event bus, нет event schema |
| **Logs** | Текстовые записи | Да | RunLogger с key=value -- хорошая основа, но нет JSON export |
| **Traces** | Причинно-связанные spans | Нет | Нет trace ID, нет parent-child spans (run → task → session → git → review) |

### Маппинг на ralph architecture

| OTel Concept | ralph Equivalent | Реализация |
|---|---|---|
| `gen_ai.client` span | `session.Execute()` call | Нет span, есть log entry |
| `gen_ai.agent` span | Task cycle (execute+review+gate) | Нет span, есть log entries |
| `gen_ai.usage.input_tokens` | Нет доступа | Требует JSON parse из `RawResult.Stdout` |
| Trace context propagation | Нет | RunID генерируется, trace ID нет |
| Metric export (OTLP/Prometheus) | Нет | RunLogger only |

[Datadog LLM Observability](https://www.datadoghq.com/product/llm-observability/) нативно поддерживает OTel GenAI conventions. [LangSmith](https://www.langchain.com/langsmith/observability) предоставляет визуальный граф tool invocations. [SigNoz](https://signoz.io/comparisons/llm-observability-tools/) -- open-source альтернатива с custom dashboards. [Comet](https://www.comet.com/site/blog/llm-observability-tools/) предлагает обзор доступных LLM observability tools.

Для ralph как CLI-инструмента (не cloud service) полноценный OTLP export избыточен на текущем этапе. Рекомендация: начать с **structured JSON log format** (совместимый с OTLP event schema), затем добавить optional OTLP exporter для пользователей с существующей observability infrastructure.

## 5. Обнаружение проблем в реальном времени

### Runtime Guardrails

Ralph уже имеет guardrails: `MaxIterations`, `MaxReviewIterations`, emergency gate. Индустриальные практики для усиления:

**Dual-threshold circuit breaker** (паттерн из [Braintrust](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026)):

1. **Soft threshold** (warning): после N consecutive retries без прогресса (HEAD не изменился), inject feedback через `InjectFeedback`: "You appear stuck. Consider breaking the task into smaller steps." Ralph уже имеет `InjectFeedback` -- достаточно добавить stuck detection logic.
2. **Hard threshold** (stop): после 2N retries -- emergency gate или автоматический skip с `SkipTask`. Ralph имеет `MaxIterations`, но они глобальны на run, не per-task.
3. **Per-session model call limits**: [SWE-agent](https://swe-agent.com) использует 8 parallel tool calls per turn. Ralph передает `MaxTurns` в `session.Options` -- аналог.
4. **HITL для ~2% рискованных решений**: loops, conflicting signals, policy violations. Ralph gates -- прямая реализация.

**Similarity-based loop detection**: после каждого `session.Execute()`, сравнить git diff с предыдущими N diffs. Если Jaccard similarity > 0.9 для 3+ consecutive diffs -- circuit breaker. Ловит случаи, когда Claude делает и откатывает одни и те же изменения.

### Anomaly Detection

- **Duration anomaly**: если текущая сессия > 3x median duration для данного task type -- предупреждение. Требует исторические данные (JSON log aggregation)
- **Token spike detection**: если input tokens текущего вызова > 2x предыдущего -- возможен context blowup (Claude включил слишком много файлов)
- **Retry storm**: > 3 retry за < 60 секунд -- системная проблема (сломанный тест, permission error), не transient failure

### Self-Healing Patterns

Индустрия активно развивает self-healing в AI-агентах:

- **Task Planner -> Coder -> Executor -> Debugger loop** ([архитектурный паттерн](https://medium.com/@atnoforaimldl/we-coded-an-ai-agent-that-can-debug-its-own-errors-heres-the-architecture-bdf5e72f87ce)): ralph реализует аналог через execute -> commit check -> review -> retry цикл. Debugger phase = review agent + `InjectFeedback`.
- **[Google CodeMender](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/)**: self-correction на основе LLM judge feedback. Ralph's review cycle с adversarial agent prompts (quality, implementation, simplification, design-principles, test-coverage) -- более продвинутая реализация с 5 специализированными агентами.
- **[InspectCoder](https://www.emergentmind.com/topics/self-debugging-agent)**: interactive debugger API с breakpoints и runtime state inspection. Для ralph -- inject test output и stack traces в retry prompt для targeted исправления.
- **DoVer pattern** (iterative failure hypothesis -> minimal intervention -> validation): ralph's review findings (hypothesis) -> inject feedback (minimal intervention) -> next execute + review (validation).
- **[Self-healing CI](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents)**: AI-агент автоматически исправляет CI failures. Ralph может интегрировать CI feedback через `--append-system-prompt` или review findings injection.
- **[Sentry Seer](https://sentry.io/product/seer/)**: AI-powered debugging -- автоматическая root cause analysis по stack trace. Интеграция с ralph: при test failure, передать structured error report в retry prompt.

### Quality & Process Benchmarks

Из внешних источников -- метрики качества AI-сгенерированного кода:

- **SWE-bench resolve rate**: Claude Code достигает 80.8% SWE-bench Verified. Метрика: fail-to-pass tests pass + pass-to-pass maintained
- **Test coverage delta**: новый код должен поддерживать или увеличивать coverage (ralph target: > 80% для runner и config)
- **Lint warnings delta**: `golangci-lint` before/after -- ralph использует v2 с 7 linters
- **Build success rate**: процент задач, завершившихся compilable кодом

## 6. Практические рекомендации для ralph

Приоритизированные рекомендации, привязанные к текущей архитектуре (`cmd/ralph -> runner -> session, gates, config`). Dependency direction строго top-down.

### P0: Критично (следующий sprint)

**6.1. Structured JSON log output**

Текущий `RunLogger` пишет `2026-03-04T10:00:00Z INFO  run started tasks_file=TASKS.md`. Добавить parallel JSON writer:

```go
type MetricEvent struct {
    Timestamp time.Time      `json:"ts"`
    Level     string         `json:"level"`
    Event     string         `json:"event"`
    Fields    map[string]any `json:"fields"`
    RunID     string         `json:"run_id"`
    TaskIndex int            `json:"task_idx,omitempty"`
}
```

Реализация: расширить `RunLogger` опциональным `jsonFile io.Writer`. Каждый `Info/Warn/Error` вызов эмитит JSON line в дополнение к текстовому формату. Обратная совместимость: текстовый формат сохраняется, JSON -- opt-in через config. Effort: S (1 день). Impact: foundation для всех остальных метрик.

**6.2. Token & cost parsing из Claude Code JSON output**

`session.Execute` возвращает `RawResult` со stdout. При `OutputJSON=true`, Claude Code JSON содержит usage data. Расширить `session.ParseResult`:

```go
type SessionMetrics struct {
    InputTokens  int     `json:"input_tokens"`
    OutputTokens int     `json:"output_tokens"`
    CacheHits    int     `json:"cache_read_input_tokens"`
    CostUSD      float64 `json:"cost_usd"`
    NumTurns     int     `json:"num_turns"`
}
```

Логировать в `RunLogger`: `tokens_in=N tokens_out=M cost_usd=X.XX`. Zero-dependency change -- данные уже в stdout. Effort: S (1 день).

**6.3. Git diff metrics after commit**

После подтверждения нового коммита (`headAfter != headBefore`), выполнить `git diff --stat HEAD~1..HEAD`. Расширить `GitClient` interface:

```go
type DiffStat struct {
    FilesChanged int
    Insertions   int
    Deletions    int
    Packages     []string
}
DiffStats(beforeSHA, afterSHA string) (DiffStat, error)
```

Логировать: `INFO commit stats files=3 insertions=47 deletions=12 packages=runner,config`. Effort: S (0.5 дня). Dependency: остается в runner package, не нарушает architecture rules.

### P1: Высокий приоритет (1-2 sprint)

**6.4. Per-task cost aggregation**

Аккумулятор в `Runner.execute()`:

```go
type CostAccumulator struct {
    TotalInputTokens  int
    TotalOutputTokens int
    EstimatedCostUSD  float64
    BudgetLimitUSD    float64  // from Config
}
```

При gate prompt показывать: "Task cost: $X.XX (N sessions)". При run finish логировать `total_cost_usd`. Requires: P0.2. Effort: S (1 день).

**6.5. Review findings breakdown**

Модифицировать `DetermineReviewOutcome` для парсинга severity из review output:

```go
type ReviewResult struct {
    Clean    bool
    Findings []Finding // severity, category, description
}
type Finding struct {
    Severity string // CRITICAL, HIGH, MEDIUM, LOW
    Category string // assertion, doc-comment, error-wrapping, etc.
    File     string
    Text     string
}
```

Логировать: `review_result clean=false findings=5 critical=0 high=1 medium=3 low=1`. Effort: M (2-3 дня).

**6.6. Stuck detection (soft circuit breaker)**

Per-task progress tracking: если `headAfter == headBefore` в N consecutive execute attempts (default N=3), inject feedback через `InjectFeedback`: "No progress detected in N attempts. Break the task down or try a different approach."

Новое поле в `Config`: `StuckThreshold int`. Использует существующий `InjectFeedback` механизм -- никаких новых dependencies. Effort: S (1 день).

**6.7. Gate decision aggregation**

Логировать каждое gate decision с timing:

```
INFO gate decided action=approve task="implement parser" wait_ms=12340
INFO gate decided action=retry task="implement parser" wait_ms=45230 feedback="fix test assertions"
```

`wait_ms` = wall-clock от начала gate prompt до ответа. В run report -- distribution: `{approve: 12, retry: 3, skip: 1, quit: 0}`. Effort: S (0.5 дня).

### P2: Средний приоритет (2-3 sprint)

**6.8. Run summary report (JSON)**

По завершении `Run()`, emit summary файл `ralph-run-YYYYMMDD-HHMMSS-summary.json`:

```json
{
    "run_id": "abc123",
    "tasks_total": 5, "tasks_completed": 4, "tasks_skipped": 1,
    "total_duration_min": 23.4,
    "total_tokens": 48200, "total_cost_usd": 1.92,
    "avg_task_duration_min": 4.7,
    "retry_total": 3, "retry_success_rate": 0.67,
    "reviews_clean": 3, "reviews_dirty": 2,
    "gate_decisions": {"approve": 4, "retry": 1, "skip": 0, "quit": 0},
    "distillations": 1, "dirty_recoveries": 0
}
```

Foundation для historical analytics и CI consumption. Effort: M (2-3 дня).

**6.9. Run ID и trace correlation**

Генерировать UUID при старте `Execute`, пропагировать как `run_id` во все log entries. Добавить `task_id` и `session_id`:

```
2026-03-04T14:23:01 INFO  execute session started run_id=abc123 task_id=3 session_id=def456
```

Превращает плоские логи в traceable events, совместимые с [OpenTelemetry](https://opentelemetry.io/docs/specs/semconv/gen-ai/) концепциями. Effort: M (1-2 дня).

**6.10. Budget alerts**

Добавить `BudgetMaxUSD float64` и `BudgetWarnPct float64` (default 0.8) в `Config`. При превышении warn -- `WARN budget threshold cost_usd=4.80 budget_usd=6.00`. При превышении max -- emergency gate: "Run budget exceeded ($X.XX / $Y.YY). Approve to continue, quit to stop." Effort: S (1 день). Requires: P0.2.

**6.11. Latency breakdown**

Инструментировать ключевые этапы в `Runner.execute()` и `Runner.Execute()`:

- `prompt_assembly_ms` (buildTemplateData)
- `session_ms` (session.Execute -- уже есть elapsed)
- `git_ops_ms` (HeadCommit + HealthCheck)
- `review_ms` (RunReview total)
- `gate_wait_ms` (wall-clock ожидания human input)

Effort: M (2 дня).

### P3: Backlog

**6.12. OpenTelemetry export** -- optional OTLP exporter для [Langfuse](https://langfuse.com/docs/observability/overview), [Datadog](https://www.datadoghq.com/product/llm-observability/), [SigNoz](https://signoz.io/comparisons/llm-observability-tools/). Build tag `otlp` с отдельным `runner/telemetry_otlp.go`. Effort: L (5+ дней).

**6.13. Window-based similarity detection** -- Jaccard similarity между consecutive diffs (N=5, threshold=0.9). При repetitive pattern -- warn + circuit breaker. Effort: M (2-3 дня).

**6.14. Historical trend analysis** -- CLI command `ralph stats --last 30d` для агрегации run reports: avg cost/task, success rate, review findings trend. Effort: L (3-5 дней).

**6.15. Test pass rate delta** -- `go test ./...` before/after с delta tracking. Предупреждение при regression. Effort: M (2 дня).

**6.16. Code complexity delta** -- `gocyclo` before/after, lint warnings delta. External tool dependency. Effort: S (1 день).

### Матрица приоритетов

| ID | Название | Effort | Impact | Dependencies |
|---|---|---|---|---|
| P0.1 (6.1) | JSON log output | S (1d) | Critical | RunLogger |
| P0.2 (6.2) | Token/cost parsing | S (1d) | Critical | session.ParseResult |
| P0.3 (6.3) | Git diff metrics | S (0.5d) | High | GitClient |
| P1.1 (6.4) | Per-task cost aggregation | S (1d) | Critical | P0.2 |
| P1.2 (6.5) | Review findings breakdown | M (2-3d) | High | DetermineReviewOutcome |
| P1.3 (6.6) | Stuck detection | S (1d) | High | InjectFeedback |
| P1.4 (6.7) | Gate decision aggregation | S (0.5d) | Medium | gates.Prompt |
| P2.1 (6.8) | Run summary report | M (2-3d) | High | P0.1-P0.3, P1.1-P1.4 |
| P2.2 (6.9) | Trace correlation | M (1-2d) | Medium | RunLogger |
| P2.3 (6.10) | Budget alerts | S (1d) | Medium | P0.2, Config |
| P2.4 (6.11) | Latency breakdown | M (2d) | Medium | RunLogger |
| P3.1 (6.12) | OpenTelemetry export | L (5+d) | Low | P2.2 |
| P3.2 (6.13) | Similarity detection | M (2-3d) | Medium | Diff storage |
| P3.3 (6.14) | Historical trends | L (3-5d) | Medium | P2.1 |
| P3.4 (6.15) | Test pass rate delta | M (2d) | High | Config extension |
| P3.5 (6.16) | Complexity delta | S (1d) | Low | External tool |

**Итого P0**: ~2.5 дня, zero new dependencies, максимальный impact. P0 + P1: ~6 дней, закрывают 80% observability gaps.

## 7. Источники

### Конкуренты и сравнительные обзоры
- [Tembo: Coding CLI Tools Comparison](https://www.tembo.io/blog/coding-cli-tools-comparison)
- [Cline: Top 6 Claude Code Alternatives](https://cline.bot/blog/top-6-claude-code-alternatives-for-agentic-coding-workflows-in-2025)
- [AI Multiple: Agentic CLI](https://aimultiple.com/agentic-cli)
- [DigitalOcean: Claude Code Alternatives](https://www.digitalocean.com/resources/articles/claude-code-alternatives)
- [Patrick Hulce: AI Code Comparison](https://blog.patrickhulce.com/blog/2025/ai-code-comparison)
- [OpenAI Codex CLI Issue #5085: Cost Tracking](https://github.com/openai/codex/issues/5085)

### Observability платформы
- [Langfuse: Observability Overview](https://langfuse.com/docs/observability/overview)
- [LangSmith: Observability](https://www.langchain.com/langsmith/observability)
- [Braintrust: Best LLM Monitoring Tools 2026](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026)
- [Datadog LLM Observability](https://www.datadoghq.com/product/llm-observability/)
- [SigNoz: LLM Observability Tools Comparison](https://signoz.io/comparisons/llm-observability-tools/)
- [Traceloop: Token Usage and Cost Per User](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user)
- [Traceloop: Granular LLM Monitoring](https://www.traceloop.com/blog/granular-llm-monitoring-for-tracking-token-usage-and-latency-per-user-and-feature)
- [Portkey: Tracking LLM Token Usage](https://portkey.ai/blog/tracking-llm-token-usage-across-providers-teams-and-workloads/)
- [Comet: LLM Observability Tools](https://www.comet.com/site/blog/llm-observability-tools/)

### Стандарты и спецификации
- [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/)

### Self-Healing и Guardrails
- [Self-Debugging AI Agent Architecture](https://medium.com/@atnoforaimldl/we-coded-an-ai-agent-that-can-debug-its-own-errors-heres-the-architecture-bdf5e72f87ce)
- [Google CodeMender: AI Agent for Code Security](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/)
- [InspectCoder: Self-Debugging Agent](https://www.emergentmind.com/topics/self-debugging-agent)
- [Dagger: Self-Healing CI Pipelines](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents)
- [Sentry Seer: AI-Powered Debugging](https://sentry.io/product/seer/)
