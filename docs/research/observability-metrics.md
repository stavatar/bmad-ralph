# Исследование: Метрики и наблюдаемость для AI-оркестраторов разработки

**Дата:** 2026-03-04
**Проект:** bmad-ralph (Go CLI, оркестратор Claude Code сессий)
**Методология:** Deep research с UNION merge двух независимых версий отчёта

---

## 1. Введение

Ralph -- Go CLI-инструмент, оркестрирующий автономные сессии Claude Code для разработки ПО. Основной цикл: чтение задач → формирование prompt → вызов Claude Code (`session.Execute`) → проверка git commit (`HeadCommit`) → code review (`RunReview`/`RealReview`) → gate check → переход к следующей задаче.

AI-оркестраторы обладают уникальными свойствами, требующими специализированной observability:

- **Недетерминизм**: LLM-вызовы дают разные результаты при одинаковых входах — требуется статистический анализ качества.
- **Стоимость**: каждый вызов потребляет tokens с реальной денежной стоимостью; без cost tracking невозможно оптимизировать бюджет.
- **Loop-риски**: автономный агент может зациклиться, тратя ресурсы без прогресса.
- **Многоуровневая обратная связь**: результат зависит от цепочки (prompt → code → test → review → gate), сбой на любом уровне требует точной диагностики.

Фреймворк [MELT (Metrics, Events, Logs, Traces)](https://opentelemetry.io/docs/) задает стандартную структуру наблюдаемости. Ralph покрывает букву "L" (Logs), "M", "E" и "T" остаются неохваченными.

### Текущее состояние ralph (RunLogger)

Ralph использует `RunLogger` — структурированный логгер (INFO/WARN/ERROR, key=value, файл + stderr). Текущие 14 точек инструментации:

- **Run lifecycle**: `run started` (tasks_file), `run finished` (status, error)
- **Task lifecycle**: `task started` (attempt, task), `task completed` (task, iterations, duration)
- **Session tracking**: `execute session started/finished` (task, review_cycle, execute_attempt, duration, exit_code)
- **Review tracking**: `review session started/finished` (task, clean/dirty, duration)
- **Git checks**: `commit check` (before, after, changed, reason)
- **Retry analytics**: `retry scheduled` (attempt, backoff, reason), `execute attempts exhausted`
- **Recovery**: `dirty state detected`, `dirty state recovered`
- **Distillation**: `distillation triggered` (counter, cooldown), `learnings budget check` (lines, near_limit)

**Ключевые пробелы**: нет token tracking, нет cost tracking, нет latency breakdown по фазам, нет structured JSON export, нет aggrеgation между запусками, нет review findings severity breakdown, нет gate decision distribution, нет budget enforcement.

---

## 2. Обзор конкурентов и аналогов

### Сравнительная таблица метрик

| Метрика | [Aider](https://aider.chat) | [SWE-agent](https://swe-agent.com) | [Devin](https://devin.ai) | [Cline](https://cline.bot) | [Codex CLI](https://github.com/openai/codex) | [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | **ralph** |
|---|---|---|---|---|---|---|---|
| Token usage (in/out) | Да, per-model | Да, per-turn | Да (cloud) | Да, VS Code | Запрошено ([#5085](https://github.com/openai/codex/issues/5085)) | Да (JSON output) | **Нет** |
| Cost tracking | Да, cumulative | Нет | Да (ACU) | Да, real-time | Нет | Через API pricing | **Нет** |
| Git diff stats | Да, blame-based | Да, per-patch | Да | Нет | Нет | Нет | **Частично** (SHA only) |
| Latency breakdown | Нет | Нет | Да (timeline) | Нет | Нет | Нет | **Частично** (session duration) |
| Retry/loop metrics | Нет | Да (step count) | Да | Нет | Нет | max_turns | **Да** (attempt, reason) |
| Review quality | Нет | Нет | Нет | Нет | Нет | Нет | **Да** (clean/dirty, cycles) |
| Loop detection | Нет (ручной лимит) | Да (max_steps) | Внутренний | Нет | Нет | Нет (max_turns) | **Частично** (MaxIterations) |
| Multi-model comparison | Да (benchmark) | Да (SWE-bench) | Нет | Нет | Нет | Нет | **Нет** |
| Export format | Text, --analytics | JSON trajectories | REST API | JSON | JSON | Hooks, JSON | **Text logs** |
| Budget alerts | Нет | Нет | Да | Да (configurable) | Нет | Нет | **Нет** |
| Gate/approval tracking | Нет | Нет | Да | Нет | Нет | Нет | **Частично** (ad-hoc) |

Источники: [Tembo: Coding CLI Tools Comparison](https://www.tembo.io/blog/coding-cli-tools-comparison), [Cline: Top 6 Claude Code Alternatives](https://cline.bot/blog/top-6-claude-code-alternatives-for-agentic-coding-workflows-in-2025), [AIMultiple: Agentic CLI](https://aimultiple.com/agentic-cli), [DigitalOcean: Claude Code Alternatives](https://www.digitalocean.com/resources/articles/claude-code-alternatives), [Patrick Hulce: AI Code Comparison](https://blog.patrickhulce.com/blog/2025/ai-code-comparison).

### Ключевые наблюдения

1. **Token tracking — table stakes.** Aider — золотой стандарт для CLI token tracking: каждый запрос логирует `prompt_tokens`, `completion_tokens`, `cached_tokens`, стоимость по модели, и git-attribution через blame. OpenAI Codex CLI получил [feature request #5085](https://github.com/openai/codex/issues/5085) именно на cost tracking — подтверждая рыночный спрос.

2. **Review quality tracking — уникальное преимущество ralph.** Ни один конкурент не отслеживает clean/dirty review cycles, severity distribution, recurrence patterns. Devin (после [приобретения Windsurf/Codeium за $250M](https://www.cognition.ai/blog/cognition-acquires-windsurf)) строит проприетарную observability, но без review feedback loop. Это дифференциатор.

3. **Structured export — минимальный порог.** Все серьезные инструменты предлагают JSON export. RunLogger пишет key=value текст, что затрудняет агрегацию.

4. **Ни один инструмент не предоставляет полной observability-системы.** Комплексная наблюдаемость с метриками качества, стоимости и loop health — незанятая ниша. Ralph, уже имеющий MaxIterations, gate-систему и review pipeline, в выгодной позиции.

5. **[Antigravity](https://blog.google/technology/google-labs/project-mariner-antigravity/) (Google)** — agent-first IDE с manager view для multi-agent orchestration — показывает направление рынка к координации множества агентов.

---

## 3. Категории метрик

### 3.1 Runtime-метрики (производительность)

| Метрика | Описание | Источник в ralph | Статус |
|---|---|---|---|
| `session.duration_ms` | Wall-clock время Claude Code сессии | `session.Execute` return | Логируется (INFO) |
| `session.tokens.input` | Input tokens per session | Claude Code JSON output | **Не собирается** |
| `session.tokens.output` | Output tokens per session | Claude Code JSON output | **Не собирается** |
| `session.tokens.cache_read` | Cached input tokens (prompt caching) | Claude Code JSON output | **Не собирается** |
| `session.num_turns` | Tool calls внутри сессии | Claude Code JSON `num_turns` | **Не собирается** |
| `task.duration_ms` | Полное время задачи (sessions + reviews + gates) | `Runner.execute` | Логируется частично |
| `run.duration_ms` | Полное время `Runner.Execute` | `Runner.Execute` | Логируется (INFO) |
| `retry.count` | Retry per task | Execute loop | Логируется (WARN) |
| `retry.backoff_ms` | Время ожидания между retry | `SleepFn` | **Не собирается** |
| `review.cycles` | Review-циклов per task | Execute loop | Логируется (INFO) |
| `git.diff.files_changed` | Файлов изменено в commit | `git diff --stat` | **Не собирается** |
| `git.diff.insertions` | Строк добавлено | `git diff --numstat` | **Не собирается** |
| `git.diff.deletions` | Строк удалено | `git diff --numstat` | **Не собирается** |
| `git.diff.packages` | Затронутые пакеты | `git diff --name-only` | **Не собирается** |
| `time_to_first_commit` | От начала задачи до первого commit | Execute loop timing | **Не собирается** |

**Latency breakdown** — где тратится время в ralph loop:

| Фаза | Текущее измерение | Предлагаемое |
|---|---|---|
| Prompt assembly (`buildTemplateData`) | Нет | `prompt_build_ms` |
| LLM session (`session.Execute`) | Да (duration) | Сохранить, добавить `first_token_ms` |
| Git operations (`HeadCommit`, `HealthCheck`) | Нет | `git_check_ms` |
| Review session (`RunReview`) | Да (duration) | Добавить `review_prompt_ms` + `review_llm_ms` |
| Gate wait (human input) | Нет | `gate_wait_ms` |
| Distillation | Нет | `distill_ms` |
| Sleep/backoff (`SleepFn`) | Нет | `backoff_total_ms` |

**Token usage** — наиболее критичная недостающая метрика. Claude Code при `--output-format json` возвращает usage data в stdout. Ralph уже парсит этот JSON через `session.ParseResult` — расширение парсера даст token counts.

**Cache hit ratio** (`cache_read_input_tokens / total_input_tokens`) — показывает эффективность prompt caching. Высокий ratio = хорошо структурированные промпты.

### 3.2 Метрики качества кода

| Метрика | Описание | Источник | Статус |
|---|---|---|---|
| `review.findings.total` | Общее число findings per review | `ReviewResult` | **Не собирается** (только Clean bool) |
| `review.findings.by_severity` | Findings по severity (C/H/M/L) | Review output parsing | **Не собирается** |
| `review.findings.by_category` | По категории (assertion, doc-comment, error-wrapping...) | Review output parsing | **Не собирается** |
| `review.fix_rate` | Доля findings, исправленных за один цикл | Review delta | **Не собирается** |
| `review.recurrence_rate` | Процент findings, повторяющих ранее зафиксированный паттерн | Cross-reference с `.claude/rules/` | **Не собирается** |
| `review.cycles_per_task` | Сколько раз review потребовал переделки | Execute loop | Логируется |
| `build.success` | Успешность `go build` after commit | Post-commit check | **Не реализовано** |
| `test.pass_rate_delta` | Изменение pass rate до/после задачи | `go test` before/after | **Не реализовано** |
| `test.new_added` | Новые тесты за задачу | Diff analysis | **Не реализовано** |
| `lint.warnings_delta` | Изменение lint warnings | Pre/post comparison | **Не реализовано** |
| `coverage.delta` | Изменение code coverage | `go test -coverprofile` | **Не реализовано** |
| `complexity.delta` | Cyclomatic complexity delta | `gocyclo` before/after | **Не реализовано** |

Текущая архитектура использует `ReviewResult{Clean bool}` — binary signal. Расширение:

```go
type ReviewResult struct {
    Clean    bool
    Findings []Finding
}
type Finding struct {
    Severity string // CRITICAL, HIGH, MEDIUM, LOW
    Category string // assertion, doc-comment, error-wrapping, etc.
    File     string
    Text     string
}
```

Из опыта ralph Epics 1-6 (226 findings across 40 stories), top recurring: assertion quality, doc comment accuracy, duplicate code, error wrapping. Классификация позволит отслеживать тренды.

### 3.3 Метрики стоимости

| Метрика | Описание | Формула | Статус |
|---|---|---|---|
| `cost.per_session` | Стоимость одной Claude Code сессии | `tokens * price_per_token` | **Не собирается** |
| `cost.per_task` | Полная стоимость задачи | `sum(sessions + reviews)` | **Не собирается** |
| `cost.per_review_cycle` | Стоимость одного review-цикла | `review_tokens * price` | **Не собирается** |
| `cost.per_retry` | Стоимость retry-попытки (waste metric) | `retry_tokens * price` | **Не собирается** |
| `cost.cumulative` | Нарастающий итог за run | `sum(all_sessions)` | **Не собирается** |
| `cost.efficiency` | Стоимость на строку кода | `cost / (insertions + deletions)` | **Не собирается** |
| `cost.per_accepted_loc` | Стоимость принятой строки (post-review) | `cost / accepted_loc` | **Не собирается** |
| `cost.waste_ratio` | Доля затрат на неуспешные попытки | `retry_cost / total_cost` | **Не собирается** |
| `cost.budget_remaining` | Остаток бюджета | `budget - cumulative` | **Не реализовано** |
| `cost.burn_rate` | Скорость расходования | `cumulative / elapsed_time` | **Не реализовано** |

Подходы из индустрии: [Traceloop](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user) — per-user token/cost через OpenTelemetry, [Portkey](https://portkey.ai/blog/tracking-llm-token-usage-across-providers-teams-and-workloads/) — cross-provider tracking, [Braintrust](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026) — per-request cost breakdowns с tag-based attribution и budget alerts.

Ralph поддерживает разные модели для execute (`ModelExecute`) и review (`ModelReview`) — стоимость разная, требуется model-aware pricing table.

**Cost budget** — мощный guardrail: `--max-cost 5.00`, ralph останавливается при достижении лимита. Аналог `--max-turns`, но в денежном выражении.

### 3.4 Метрики здоровья loop (петель)

| Метрика | Описание | Механизм | Статус |
|---|---|---|---|
| `loop.iteration` | Текущий номер итерации | Execute counter | Логируется |
| `loop.max_iterations` | Лимит итераций | `config.Config.MaxIterations` | Реализовано |
| `loop.similarity_score` | Similarity с предыдущими N outputs | Window-based comparison | **Не реализовано** |
| `loop.stuck_detected` | Детекция зацикливания | Threshold crossing | **Не реализовано** |
| `loop.circuit_breaker` | Принудительная остановка | Pattern matcher | **Не реализовано** |
| `loop.error_repetition` | Один тип ошибки N раз подряд | Error pattern tracker | **Не реализовано** |
| `loop.monotonicity` | Тест pass count не растет 3+ итераций | Test result delta | **Не реализовано** |
| `gate.action` | Решение gate (approve/retry/skip/quit) | `GateDecision.Action` | Логируется (ad-hoc) |
| `gate.distribution` | Распределение решений | Counter per action type | **Не собирается** |
| `gate.time_to_decision` | Wall-clock от prompt до ответа | Gate timing | **Не собирается** |
| `gate.approve_rate_trend` | Тренд approve rate | Running average | **Не собирается** |
| `gate.emergency_frequency` | Частота emergency gates | `Gate.Emergency` counter | **Не собирается** |
| `gate.consecutive_rejections` | Последовательные отклонения | Counter | **Не собирается** |

**Loop detection** — dual-threshold система:

1. **Warning threshold** (soft): similarity > 0.85 за последние 3 итерации → WARN, hint injection в prompt.
2. **Hard threshold** (force): similarity > 0.95 за последние 3 итерации → human gate trigger.

Ralph уже имеет `MaxIterations` как hard stop и `EmergencyGatePromptFn`. Similarity check между ними создаст трехуровневую защиту: similarity warning → human gate → hard stop.

**Gate decision distribution** (`{approve: 73%, retry: 15%, skip: 8%, quit: 4%}`) — показывает доверие оператора к AI. Растущий approve rate = улучшение качества.

**Emergency gate frequency** > 5% = системная проблема в промптах или конфигурации.

### 3.5 Метрики самовосстановления

| Метрика | Описание | Источник в ralph | Статус |
|---|---|---|---|
| `recovery.dirty_state` | Успешность dirty state recovery | `RecoverDirtyState` | Логируется |
| `recovery.resume_extract` | Успешность resume extraction | `ResumeExtraction` | Логируется частично |
| `recovery.revert` | Количество revert операций | `RevertTask` | Логируется |
| `recovery.skip` | Количество skip операций | `SkipTask` | Логируется |
| `recovery.distill_success` | Distillation success/failure | `DistillFn` | Частично |
| `error.category` | Классификация (transient/persistent) | Error wrapping prefix | **Не собирается** |
| `error.auto_recovery_rate` | % автоматически восстановленных | `recovery.* / error.*` | **Не собирается** |
| `error.escalation_rate` | % потребовавших human intervention | Gate trigger count | **Не собирается** |
| `recovery.time_ms` (MTTR) | Время от ошибки до успешного продолжения | Timing delta | **Не собирается** |
| `debug.self_correction_cycles` | Сколько раз агент исправил свою ошибку | Review retry success | **Не собирается** |

**Error categorization** через parsing error wrapping prefixes (`"runner: scan tasks:"`, `"runner: dirty state recovery:"`). Transient errors (rate limit, timeout) требуют иной стратегии recovery, чем persistent (bad prompt, impossible task).

**MTTR (Mean Time To Recovery)** — от ошибки до successful resume. Ключевая SRE-метрика, адаптированная для AI loop.

---

## 4. Стандарты наблюдаемости

### OpenTelemetry GenAI Semantic Conventions

[OpenTelemetry GenAI](https://opentelemetry.io/docs/specs/semconv/gen-ai/) (experimental, активно развиваются) определяет:

- `gen_ai.system` — провайдер (`"anthropic"`)
- `gen_ai.request.model` — модель
- `gen_ai.request.max_tokens` — лимит output tokens
- `gen_ai.usage.input_tokens` / `gen_ai.usage.output_tokens` — потребление
- `gen_ai.response.finish_reason` — причина завершения

Типы spans:
- **`gen_ai.client` spans** — один LLM вызов → `session.Execute()` в ralph
- **`gen_ai.agent` spans** (development) — агентный workflow → task cycle в ralph
- **Events**: `gen_ai.choice`, `gen_ai.tool.message`

### Маппинг на ralph

| OTel Concept | ralph Equivalent | Реализация |
|---|---|---|
| `gen_ai.client` span | `session.Execute()` call | Log entry, нет span |
| `gen_ai.agent` span | Task cycle (execute+review+gate) | Log entries, нет span |
| `gen_ai.usage.input_tokens` | Нет доступа | Требует JSON parse `RawResult.Stdout` |
| Trace context propagation | Нет | RunID есть, trace ID нет |

### MELT Framework

| Столп | Описание | Покрытие в ralph | Рекомендация |
|---|---|---|---|
| **Metrics** | Числовые агрегаты | Нет (implicit в логах) | RunMetrics struct, JSON summary |
| **Events** | Дискретные структурированные факты | Частично (log lines) | Structured events с типами |
| **Logs** | Текстовый поток | Да (RunLogger) | JSON parallel output |
| **Traces** | Причинно-связанные spans | Нет | Не нужно для single-process CLI (run_id/task_id достаточно) |

### Рекомендуемая иерархия spans для ralph

```
run (top-level, run_id)
  task (per story/task, task_id)
    session (Claude Code invocation)
      prompt_build
      claude_execution (tokens, cost)
      result_parse
    git_check (before/after SHA, diff_stats)
    review (optional)
      review_session (tokens, cost)
      review_parse (findings)
    gate_check (optional)
      gate_prompt
      gate_decision (action, wait_ms)
    retry (if applicable, reason, backoff)
  distillation (optional)
```

Источники: [Langfuse](https://langfuse.com/docs/observability/overview), [LangSmith](https://www.langchain.com/langsmith/observability), [Datadog LLM Obs](https://www.datadoghq.com/product/llm-observability/), [SigNoz](https://signoz.io/comparisons/llm-observability-tools/), [Comet: LLM Observability Tools](https://www.comet.com/site/blog/llm-observability-tools/).

---

## 5. Обнаружение и исправление проблем в реальном времени

### Runtime Guardrails (три уровня)

**Уровень 1 — Passive monitoring** (текущий ralph):
- Логирование событий через RunLogger
- Подсчет итераций (MaxIterations)
- Запись длительности сессий

**Уровень 2 — Active detection** (рекомендация):
- Similarity detection между последовательными outputs
- Token budget tracking с предупреждениями на 80%/100%
- Повторяющиеся ошибки одного типа (>3 одинаковых за run)
- Progress indicators: нет commits, тесты не проходят, findings не уменьшаются

**Уровень 3 — Automated response** (частично в ralph):
- Human gate trigger при аномалиях (EmergencyGatePromptFn)
- Автоматический revert при broken builds (RevertTask)
- Resume extraction при crashed sessions (ResumeExtraction)

### Dual-Threshold система

| Метрика | Warning threshold | Hard threshold | Действие при Warning | Действие при Hard |
|---|---|---|---|---|
| Iteration count | `MaxIterations * 0.75` | `MaxIterations` | WARN log | Force stop |
| Token cost | `budget * 0.80` | `budget * 1.0` | WARN + hint inject | Emergency gate |
| Output similarity | 0.85 (3-window) | 0.95 (3-window) | Hint injection | Human gate |
| Review cycles | 2 per task | `maxReviewRetries` | WARN log | Skip/emergency gate |
| Retry count | 3 per session | 5 per session | WARN log | Force stop |
| Time per task | 30 min | 60 min | WARN log | Human gate |
| Consecutive failures | 3 | 5 | WARN log | Force stop |

### Anomaly Detection

- **Duration anomaly**: текущая сессия > 3x median duration → предупреждение (требует исторические данные)
- **Token spike**: input tokens текущего вызова > 2x предыдущего → возможен context blowup
- **Retry storm**: > 3 retry за < 60 секунд → системная проблема (сломанный тест, permission error)

### HITL (Human-in-the-Loop) Triggers

Ralph уже имеет gate-систему. Рекомендуемые автоматические HITL triggers:

1. **Loop detected** — similarity > warning threshold 3 итерации подряд
2. **Conflicting signals** — тесты проходят, но review находит critical findings
3. **High-value action** — удаление файлов, изменение CI конфигурации
4. **Budget approaching** — cost приближается к hard limit
5. **Cost spike** — стоимость задачи > 2x median
6. **Unprecedented error** — ошибка, не встречавшаяся ранее

### Self-Healing Patterns (индустрия)

- **Task Planner → Coder → Executor → Debugger** ([архитектурный паттерн](https://medium.com/@atnoforaimldl/we-coded-an-ai-agent-that-can-debug-its-own-errors-heres-the-architecture-bdf5e72f87ce)): ralph реализует аналог через execute → commit check → review → retry. Debugger phase = review agent + InjectFeedback.
- **[Google CodeMender](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/)**: self-correction на основе LLM judge feedback. Ralph's review cycle с 5 специализированными sub-agents (quality, implementation, simplification, design-principles, test-coverage) — более продвинутая реализация.
- **[InspectCoder](https://www.emergentmind.com/topics/self-debugging-agent)**: interactive debugger API с breakpoints и runtime state inspection. Для ralph — inject test output и stack traces в retry prompt.
- **DoVer pattern**: iterative failure hypothesis → minimal intervention → validation. Ralph: review findings (hypothesis) → inject feedback (minimal intervention) → next execute + review (validation).
- **[Self-healing CI](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents)**: AI-агент автоматически исправляет CI failures. Ralph может интегрировать CI feedback через `--append-system-prompt`.
- **[Sentry Seer](https://sentry.io/product/seer/)**: AI-powered root cause analysis по stack trace. Интеграция: передать structured error report в retry prompt.

---

## 6. Рекомендации для ralph

Привязаны к текущей архитектуре (`cmd/ralph → runner → session, gates, config`). Dependency direction строго top-down.

### P0: Немедленная реализация (~2.5 дня, zero new deps)

**P0.1: RunMetrics struct и JSON summary**

```go
// runner/metrics.go
type TaskMetrics struct {
    Name           string         `json:"name"`
    Iterations     int            `json:"iterations"`
    DurationMs     int64          `json:"duration_ms"`
    CommitSHA      string         `json:"commit_sha,omitempty"`
    DiffStats      *DiffStats     `json:"diff_stats,omitempty"`
    ReviewCycles   int            `json:"review_cycles"`
    ReviewFindings map[string]int `json:"review_findings,omitempty"`
    GateDecision   string         `json:"gate_decision,omitempty"`
    Retries        int            `json:"retries"`
    Status         string         `json:"status"` // completed, skipped, reverted, failed
}

type RunMetrics struct {
    RunID           string        `json:"run_id"`
    StartTime       time.Time     `json:"start_time"`
    EndTime         time.Time     `json:"end_time"`
    TotalDurationMs int64         `json:"total_duration_ms"`
    Tasks           []TaskMetrics `json:"tasks"`
    Totals          RunTotals     `json:"totals"`
}
```

Хранение: `<logDir>/ralph-metrics-<runID>.json`. Усилия: ~200 LOC.

**P0.2: Token & cost parsing из Claude Code JSON output**

Расширить `session.ParseResult` для извлечения usage data:

```go
type SessionMetrics struct {
    InputTokens  int     `json:"input_tokens"`
    OutputTokens int     `json:"output_tokens"`
    CacheHits    int     `json:"cache_read_input_tokens"`
    CostUSD      float64 `json:"cost_usd"`
    NumTurns     int     `json:"num_turns"`
}
```

Zero-dependency change — данные уже в stdout. Усилия: ~120 LOC.

**P0.3: Git diff metrics after commit**

Расширить `GitClient` interface:

```go
DiffStats(beforeSHA, afterSHA string) (DiffStat, error)
```

Логировать: `INFO commit stats files=3 insertions=47 deletions=12 packages=runner,config`. Усилия: ~100 LOC.

**P0.4: Structured log keys (run_id, task_id, step_type)**

```
2026-03-04T10:15:30 INFO  [runner] session complete run_id=abc123 task_id=story-5.1 step_type=session duration_ms=45000
```

Усилия: ~80 LOC.

### P1: Высокий приоритет (~3.5 дня)

**P1.1: Per-task cost aggregation** — аккумулятор в Runner, при gate prompt показывать стоимость. Requires P0.2. ~100 LOC.

**P1.2: ReviewResult enrichment** — расширить `{Clean bool}` до гранулярной структуры с findings по severity и category. ~150 LOC.

**P1.3: Stuck detection (soft circuit breaker)** — если `headAfter == headBefore` в N consecutive attempts, inject feedback. Новое поле `StuckThreshold int` в Config. ~100 LOC.

**P1.4: Gate decision aggregation** — логировать с timing (`gate_wait_ms`), в run report — distribution `{approve: 12, retry: 3, skip: 1, quit: 0}`. ~80 LOC.

### P2: Средний приоритет (~8 дней)

**P2.1: Run summary report (JSON)** — по завершении Run() emit `ralph-run-summary.json`. Foundation для historical analytics. ~200 LOC.

**P2.2: Run ID и trace correlation** — UUID per run, task_id и session_id в каждый log entry. ~100 LOC.

**P2.3: Budget alerts** — `BudgetMaxUSD` + `BudgetWarnPct` в Config, WARN при 80%, emergency gate при 100%. ~80 LOC.

**P2.4: Latency breakdown** — инструментировать `prompt_assembly_ms`, `git_ops_ms`, `gate_wait_ms`. ~150 LOC.

**P2.5: Loop detection (similarity-based)** — Jaccard similarity между consecutive diffs (N=5, threshold=0.9). ~200 LOC.

**P2.6: Error categorization** — parsing error wrapping prefixes для классификации. ~100 LOC.

### P3: Backlog

**P3.1: OpenTelemetry export** — optional OTLP exporter. Build tag `otlp`. ~300 LOC.

**P3.2: Langfuse integration** — optional HTTP API (не SDK). Feature flag. ~300 LOC.

**P3.3: Terminal dashboard** — live метрики через `charmbracelet/bubbletea`. ~500 LOC.

**P3.4: Trend analysis CLI** — `ralph metrics --last 10` для анализа runs. ~200 LOC.

**P3.5: Test pass rate delta** — `go test` before/after. ~150 LOC.

**P3.6: Complexity delta** — `gocyclo` before/after. External tool dependency. ~100 LOC.

### Сводная матрица

| ID | Название | Effort | Impact | Deps | LOC |
|---|---|---|---|---|---|
| P0.1 | RunMetrics struct + JSON | 1d | Critical | RunLogger | ~200 |
| P0.2 | Token/cost parsing | 1d | Critical | session.ParseResult | ~120 |
| P0.3 | Git diff metrics | 0.5d | High | GitClient | ~100 |
| P0.4 | Structured log keys | — | Medium | RunLogger | ~80 |
| P1.1 | Per-task cost aggregation | 1d | Critical | P0.2 | ~100 |
| P1.2 | ReviewResult enrichment | 1.5d | High | DetermineReviewOutcome | ~150 |
| P1.3 | Stuck detection | 0.5d | High | InjectFeedback | ~100 |
| P1.4 | Gate decision aggregation | 0.5d | Medium | gates.Prompt | ~80 |
| P2.1 | Run summary report | 2d | High | P0.*, P1.* | ~200 |
| P2.2 | Trace correlation | 1d | Medium | RunLogger | ~100 |
| P2.3 | Budget alerts | 1d | Medium | P0.2, Config | ~80 |
| P2.4 | Latency breakdown | 2d | Medium | RunLogger | ~150 |
| P2.5 | Similarity detection | 2d | Medium | Diff storage | ~200 |
| P2.6 | Error categorization | 1d | Low | runner, config | ~100 |
| P3.1 | OpenTelemetry export | 5d | Low | P2.2 | ~300 |
| P3.2 | Langfuse integration | 3d | Low | P2.2 | ~300 |
| P3.3 | Terminal dashboard | 5d | Medium | bubbletea | ~500 |
| P3.4 | Trend analysis CLI | 3d | Medium | P2.1 | ~200 |
| P3.5 | Test pass rate delta | 2d | High | Config | ~150 |
| P3.6 | Complexity delta | 1d | Low | External tool | ~100 |

**P0 total: ~500 LOC, ~2.5 дня, zero new dependencies.**
**P0+P1: ~930 LOC, ~6 дней, zero new dependencies, закрывают ~80% gaps.**
**Все рекомендации P0-P2 не требуют новых внешних зависимостей** — соответствует принципу ralph "Only 3 direct deps".

---

## 7. Источники

### Стандарты и фреймворки
- [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/)
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)

### Observability-платформы
- [Langfuse — Open Source LLM Observability](https://langfuse.com/docs/observability/overview)
- [LangSmith Observability](https://www.langchain.com/langsmith/observability)
- [Braintrust: Best LLM Monitoring Tools 2026](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026)
- [Datadog LLM Observability](https://www.datadoghq.com/product/llm-observability/)
- [SigNoz: LLM Observability Tools Comparison](https://signoz.io/comparisons/llm-observability-tools/)
- [Comet: LLM Observability Tools](https://www.comet.com/site/blog/llm-observability-tools/)

### Cost и token tracking
- [Traceloop: From Bills to Budgets](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user)
- [Traceloop: Granular LLM Monitoring](https://www.traceloop.com/blog/granular-llm-monitoring-for-tracking-token-usage-and-latency-per-user-and-feature)
- [Portkey: Tracking LLM Token Usage](https://portkey.ai/blog/tracking-llm-token-usage-across-providers-teams-and-workloads/)

### Конкуренты и сравнения
- [Tembo: Coding CLI Tools Comparison](https://www.tembo.io/blog/coding-cli-tools-comparison)
- [Cline: Top 6 Claude Code Alternatives](https://cline.bot/blog/top-6-claude-code-alternatives-for-agentic-coding-workflows-in-2025)
- [AIMultiple: Agentic CLI](https://aimultiple.com/agentic-cli)
- [DigitalOcean: Claude Code Alternatives](https://www.digitalocean.com/resources/articles/claude-code-alternatives)
- [Patrick Hulce: AI Code Comparison](https://blog.patrickhulce.com/blog/2025/ai-code-comparison)
- [OpenAI Codex Issue #5085 — Cost Tracking](https://github.com/openai/codex/issues/5085)

### Self-healing и debugging
- [Self-Debugging Agent Architecture](https://medium.com/@atnoforaimldl/we-coded-an-ai-agent-that-can-debug-its-own-errors-heres-the-architecture-bdf5e72f87ce)
- [Google CodeMender](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/)
- [InspectCoder: Self-Debugging Agent](https://www.emergentmind.com/topics/self-debugging-agent)
- [Dagger: Self-Healing CI Pipelines](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents)
- [Sentry Seer: AI-Powered Debugging](https://sentry.io/product/seer/)

### Модели и pricing
- [Anthropic Pricing](https://www.anthropic.com/pricing)
- [Claude Code Documentation](https://docs.anthropic.com/en/docs/claude-code)
