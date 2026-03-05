# Исследование: Метрики и наблюдаемость для AI-оркестраторов разработки

**Дата**: 2026-03-04
**Контекст**: ralph — Go CLI-инструмент для оркестрации Claude Code сессий в автономном цикле разработки (Ralph Loop)
**Статус**: Финальный отчёт (UNION merge v1+v2)

---

## 1. Введение

Наблюдаемость (observability) — критический фактор успеха AI-оркестраторов разработки. В отличие от традиционных CI/CD-систем, где пайплайн детерминирован, AI-оркестратор работает в стохастической среде: LLM может зациклиться, превысить бюджет токенов, сгенерировать код с регрессиями или застрять на одной задаче. Без метрик и инструментирования оператор слеп: он не знает, почему задача заняла 40 минут вместо 5, сколько стоил каждый retry, и достаточно ли качественен сгенерированный код.

Рынок автономных AI-агентов для разработки оценивается в ~$8.5B к 2026 году (Grand View Research), и наблюдаемость становится критическим дифференциатором.

### Текущее состояние ralph

Ralph располагает `RunLogger` — структурированным логгером (пакет `runner/log.go`) с уровнями INFO/WARN/ERROR и key=value форматом. Логи пишутся в ежедневный файл (`ralph-YYYY-MM-DD.log`) и дублируются в stderr. Логируемые события:

- `run started/finished` (с `status=ok|error`)
- `task started/completed` (с `attempt=N/M`, `task=...`, `duration`)
- `execute session started/finished` (с `duration`, `exit` code, `review_cycle`, `execute_attempt`)
- `commit check` (с `before`/`after` SHA, `changed=true/false`)
- `retry scheduled` (с `attempt`, `backoff`, `reason`)
- `review session started/finished` (с `clean=true/false`)
- `dirty state detected/recovered`
- `learnings budget check` (с `lines=N/M`, `near_limit`)
- `distillation triggered` (с `counter`, `cooldown`)

Система `DistillMetrics` отслеживает метрики дистилляции: `entries_before/after`, `stale_removed`, `categories_preserved/total`, `t1_promotions`.

**Что НЕ отслеживается**: token consumption, стоимость вызовов, diff-метрики (lines changed), test pass rate delta, review findings breakdown по severity, gate decision distribution, session-level tool call counts, latency breakdown по этапам.

Фреймворк [MELT (Metrics, Events, Logs, Traces)](https://www.splunk.com/en_us/data-insider/melt-metrics-events-logs-traces.html) задаёт стандартную структуру наблюдаемости. Ralph покрывает "L" (Logs) и частично "E" (Events), но "M" (Metrics) и "T" (Traces) остаются неохваченными. Цель исследования — определить, какие метрики и механизмы дадут наибольшую отдачу при минимальном усложнении архитектуры.

---

## 2. Обзор конкурентов и аналогов

| Инструмент | Тип | Token tracking | Cost tracking | Git metrics | Loop detection | Review/Quality | Export |
|---|---|---|---|---|---|---|---|
| [Aider](https://aider.chat/) | CLI, OSS | Да (in/out/cache) | Да (--analytics, $/model) | Да (blame attribution) | Нет (ручной лимит) | Нет | Markdown reports |
| [SWE-agent](https://swe-agent.com) | Benchmark CLI | Да (per step) | Частично (API) | Нет | Да (max_steps, context mgmt) | SWE-bench resolve rate | JSON trajectory |
| [Devin](https://devin.ai) | Cloud IDE | Внутренний | ACU (compute units) | Внутренний | Да (manager view) | Внутренний | Проприетарный dashboard |
| [Cline](https://github.com/cline/cline) | VS Code ext | Да (per request) | Да (per-task budget alerts) | Нет | Нет | Нет | VS Code output panel |
| [Antigravity](https://blog.google/technology/google-labs/project-mariner-antigravity/) (Google) | Agent IDE | Внутренний | Внутренний | Внутренний | Manager agent view | Внутренний | Проприетарный |
| [OpenAI Codex](https://github.com/openai/codex) | CLI, OSS | Запрошен ([#5085](https://github.com/openai/codex/issues/5085)) | Запрошен | Нет | Нет | Нет | stdout |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | CLI | Да (json output) | Через API billing | Нет | Нет (max_turns) | 80.8% SWE-bench | JSON, hooks |
| **ralph** | CLI orchestrator | **Нет** | **Нет** | Частично (HEAD SHA) | Частично (MaxIterations) | Частично (clean/dirty) | Structured log kv |

### Ключевые наблюдения

**[Aider](https://aider.chat/docs/more/analytics.html)** — золотой стандарт для CLI token tracking. Логирует `prompt_tokens`, `completion_tokens`, `cached_tokens`, стоимость по модели, и git-attribution через blame. Analytics opt-in позволяет benchmark-агрегацию.

**[Cline](https://github.com/cline/cline)** реализует per-task budget alerts: пользователь задаёт лимит в долларах, Cline останавливается при приближении. Прямой аналог ralph's `LearningsBudget`, но для финансов.

**[Claude Code](https://docs.anthropic.com/en/docs/claude-code)** через `--output-format json` предоставляет structured output, включая `session_id`, `num_turns`, `total_cost_usd` (когда доступно). Ralph уже использует JSON mode (`session.Options.OutputJSON`). [Hooks system](https://docs.anthropic.com/en/docs/claude-code/hooks) позволяет инжектировать pre/post обработку.

**[SWE-agent](https://swe-agent.com)** выделяется trajectory logging — полным журналом действий в JSON, что позволяет offline-анализ. `max_steps` как ограничитель loop примитивен, но функционален.

**[Langfuse](https://langfuse.com/)** как open-source observability platform задаёт стандарт для structured tracing: traces → spans (generations, tools, retrievals), каждый span несёт token counts, latency, cost, quality scores. Интеграция через [OpenTelemetry export](https://langfuse.com/docs/integrations/opentelemetry).

**[OpenAI Codex](https://github.com/openai/codex/issues/5085)** — cost tracking как open feature request с значительной поддержкой, подтверждая рыночный спрос на эту функцию в CLI-агентах.

**Devin и Antigravity** — закрытые экосистемы с внутренними dashboards без API для export метрик. Для CLI-инструмента вроде ralph это anti-pattern: пользователь должен владеть своими данными.

---

## 3. Таксономия метрик

### 3.1 Производительность и runtime

Runtime-метрики отвечают на вопрос "как быстро и эффективно работает агент?".

| Метрика | Описание | Источник в ralph | Статус |
|---|---|---|---|
| `tokens_input` | Input tokens per session | Claude Code JSON output | **Не собирается** |
| `tokens_output` | Output tokens per session | Claude Code JSON output | **Не собирается** |
| `tokens_cached` | Prefix cache hits per session | Claude Code JSON output | **Не собирается** |
| `num_turns` | Количество tool-use turns в сессии | Claude Code JSON output | **Не собирается** |
| `session_duration_ms` | Wall-clock время Claude Code сессии | `session.Execute` return + `time.Since` | Логируется (INFO) |
| `task_duration_ms` | Полное время задачи (sessions+reviews+gates) | Execute loop | Логируется частично |
| `run_duration_ms` | Полное время `Execute` (все задачи) | `Runner.Execute` | Логируется |
| `retry_count` | Количество retry per task | Execute loop | Логируется (WARN) |
| `retry_backoff_ms` | Время ожидания между retry | `SleepFn` | **Не собирается** |
| `review_cycles` | Количество review-циклов per task | Execute loop | Логируется |
| `git_diff_files_changed` | Файлов изменено в commit | `git diff --stat` | **Не собирается** |
| `git_diff_insertions` | Строк добавлено | `git diff --numstat` | **Не собирается** |
| `git_diff_deletions` | Строк удалено | `git diff --numstat` | **Не собирается** |
| `prompt_assembly_ms` | Время сборки промпта | `config.AssemblePrompt` | **Не собирается** |
| `git_ops_ms` | Время git-операций | `HeadCommit`, `RestoreClean` | **Не собирается** |
| `gate_wait_ms` | Время ожидания human input | Gate prompt duration | **Не собирается** |

**Token usage** — наиболее критичная недостающая метрика. Claude Code в JSON output mode возвращает usage data. [OpenTelemetry GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) определяют `gen_ai.usage.input_tokens` и `gen_ai.usage.output_tokens` как стандартные атрибуты.

**Latency breakdown** позволит обнаружить bottlenecks: медленный git на больших репозиториях, долгие review, ожидание human gates. Сейчас ralph логирует только общий `duration` сессии.

**Git diff stats** доступны через существующий `GitClient` interface. Метод `DiffStatsSinceCommit(commit string) (DiffStats, error)` через `git diff --numstat` — zero-dependency addition.

### 3.2 Качество и корректность

Метрики качества — ключевое отличие AI-оркестратора от chatbot-обёртки.

| Метрика | Описание | Источник | Статус |
|---|---|---|---|
| `review_findings_total` | Общее число findings per review | ReviewResult parsing | **Нет** (Clean bool только) |
| `review_findings_by_severity` | Findings по C/H/M/L | Review output parsing | **Нет** |
| `review_findings_types` | Bug, style, performance, security | Review output parsing | **Нет** |
| `review_recurrence_rate` | Повторяющиеся паттерны | Cross-task analysis | **Нет** |
| `build_success` | go build after commit | Post-commit check | **Не реализовано** |
| `test_pass_rate_delta` | Тесты до/после задачи | `go test ./...` pre/post | **Не реализовано** |
| `lint_warnings_delta` | Lint warnings до/после | Pre/post comparison | **Не реализовано** |
| `coverage_delta` | Code coverage до/после | `-coverprofile` pre/post | **Не реализовано** |
| `complexity_delta` | Cyclomatic complexity до/после | `gocyclo` pre/post | **Не реализовано** |

Текущий `ReviewResult` имеет структуру `{Clean bool}` — binary signal. Расширение до `{Clean bool; Findings []Finding}` с severity parsing даст гранулярные данные без изменения review flow.

**Test pass rate delta**: [Anthropic eval methodology](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) и SWE-bench используют fail-to-pass + pass-to-pass подход. Для ralph — `exec.Command(goExe, "test", "./...")` до и после commit.

**Build success** — бинарный, но критически важен для обнаружения syntax errors, import cycles, type mismatches. Потенциально отдельный post-commit gate.

### 3.3 Экономика и стоимость

Финансовая наблюдаемость — одна из самых востребованных фич. [Braintrust](https://www.braintrust.dev/docs/guides/traces) предоставляет per-request cost breakdowns с tag-based attribution.

| Метрика | Описание | Формула | Статус |
|---|---|---|---|
| `cost_per_session` | Стоимость одной Claude Code сессии | `tokens × price_per_token` | **Нет** |
| `cost_per_task` | Полная стоимость задачи | `sum(sessions + reviews)` | **Нет** |
| `cost_per_review` | Стоимость одного review-цикла | `review_tokens × price` | **Нет** |
| `cost_per_retry` | Стоимость retry-попытки | `retry_tokens × price` | **Нет** |
| `cost_cumulative` | Нарастающий итог за run | `sum(all_sessions)` | **Нет** |
| `cost_efficiency` | $/строку изменённого кода | `cost / (insertions + deletions)` | **Нет** |
| `token_efficiency` | Tokens/строку изменённого кода | `tokens_total / lines_changed` | **Нет** |
| `cost_budget_remaining` | Остаток бюджета (если задан) | `budget - cumulative` | **Нет** |

Стоимость вычислима из token counts при известных ценах модели. Ralph может хранить pricing table в `config.Config`. [Anthropic pricing](https://www.anthropic.com/pricing) для Claude обновляется нечасто. Claude Sonnet 4 и Claude Opus 4 имеют разные тарифы — смешанное использование (`ModelExecute` vs `ModelReview`) требует per-session tracking.

**Cost budget** — мощный guardrail: `--max-cost 5.00`, ralph останавливается при достижении. Аналог `--max-turns` в Claude Code, но в денежном выражении. [Cline](https://github.com/cline/cline) уже реализует per-task budget alerts.

**Model cost comparison**: при A/B данных по моделям можно оптимизировать model selection по cost/quality tradeoff.

### 3.4 Здоровье цикла (loop health)

Обнаружение зацикливания и деградации — критическая проблема для autonomous agents. [Langfuse](https://langfuse.com/docs/tracing) предоставляет end-to-end tracing с визуализацией agent loops.

| Метрика | Описание | Механизм | Статус |
|---|---|---|---|
| `loop_iteration` | Текущий номер итерации | Execute counter | Логируется |
| `loop_max` | Лимит итераций | `Config.MaxIterations` | Реализовано |
| `loop_similarity_score` | Similarity с предыдущими N outputs | Window-based comparison | **Нет** |
| `loop_stuck_detected` | Детекция зацикливания | Threshold crossing | **Нет** |
| `loop_circuit_breaker` | Принудительная остановка | Pattern matcher | **Нет** |
| `gate_action` | Решение gate (approve/retry/skip/quit) | `GateDecision.Action` | **Не агрегируется** |
| `gate_feedback_injected` | Был ли injected feedback | `GateDecision.Feedback` | **Не агрегируется** |
| `emergency_gate_count` | Срабатывания emergency gate | `EmergencyGatePromptFn` | **Не агрегируется** |
| `circuit_breaker_triggers` | MaxIterations/MaxReviewIterations hit | Hard stop reached | **Не агрегируется** |

**Loop detection** — ключевая проблема. Dual-threshold система зарекомендовала себя в индустрии ([Arize](https://arize.com/blog/), [Braintrust](https://www.braintrust.dev/)):

1. **Warning threshold** (soft): при similarity > 0.85 за последние 3 итерации — WARN + hint injection: "You seem stuck. Try a different approach." Ralph уже имеет `InjectFeedback`.
2. **Hard threshold** (force): при similarity > 0.95 за 3 итерации — trigger human gate через `EmergencyGatePromptFn`.

Ralph может использовать простой подход: сравнение git diff output между итерациями. Если diff идентичен или пуст в N последовательных попытках — агент застрял.

**Gate decision distribution**: высокий % retry указывает на систематические проблемы; частый skip — на неадекватные задачи. Emergency gate frequency — индикатор проблем в промптах/конфигурации.

### 3.5 Восстановление и устойчивость

| Метрика | Описание | Источник | Статус |
|---|---|---|---|
| `recovery_dirty_state` | Успешность из dirty state | `RecoverDirtyState` | Логируется |
| `recovery_resume_extract` | Успешность извлечения resume | `ResumeExtraction` | Частично |
| `recovery_distillation` | Восстановление прерванной distillation | `RecoverDistillation` | Логируется |
| `recovery_revert_count` | Количество revert операций | `RevertTask` | Логируется |
| `recovery_skip_count` | Количество skip операций | `SkipTask` | Логируется |
| `error_category` | Классификация (network/model/git/fs) | Error prefix parsing | **Нет** |
| `error_auto_recovery_rate` | % восстановлений без human | `recovery.* / error.*` | **Нет** |
| `distill_duration_ms` | Время дистилляции | `AutoDistill` timing | **Нет** |
| `distill_validation_failures` | Ошибки валидации дистилляции | `ValidateDistillation` | **Нет** |

**Error categorization**: ralph оборачивает ошибки с единообразными префиксами (`"runner: startup:"`, `"runner: execute:"`, `"runner: scan tasks:"`). Parsing prefix → category: `"runner: git:"` → git, `"runner: execute:"` → model, `"runner: read tasks:"` → filesystem.

**Resume extraction effectiveness**: `ResumeExtraction` вызывает Claude для извлечения insights из прерванной сессии. Метрика: количество извлечённых entries в LEARNINGS.md, их quality (validated vs. rejected при дистилляции).

---

## 4. Индустриальные стандарты

### OpenTelemetry GenAI Semantic Conventions

[OpenTelemetry](https://opentelemetry.io/) выпустил [GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) (experimental, активно развиваются):

- **`gen_ai.client` spans** — обёртка вокруг одного LLM вызова:
  - `gen_ai.system` — провайдер ("anthropic")
  - `gen_ai.request.model` — модель ("claude-sonnet-4-20250514")
  - `gen_ai.request.max_tokens` — лимит output tokens
  - `gen_ai.usage.input_tokens` — фактическое потребление input tokens
  - `gen_ai.usage.output_tokens` — output tokens
  - `gen_ai.response.finish_reason` — причина завершения (stop, max_tokens, error)

- **`gen_ai.agent` spans** (в разработке) — обёртка вокруг agent loop iteration. Для ralph: один span per execute session + один per review session.

[Datadog LLM Observability](https://docs.datadoghq.com/llm_observability/) реализует нативную поддержку этих conventions.

**Рекомендация для ralph**: полный OpenTelemetry SDK избыточен для single-threaded CLI. Однако следование naming conventions в key-value парах `RunLogger` обеспечит совместимость с экосистемой.

### MELT Framework

| Столп | Текущее покрытие | Рекомендация |
|---|---|---|
| **Metrics** | Нет (implicit в логах) | `RunMetrics` struct, JSON summary |
| **Events** | Частично (log lines = events) | Structured events с типами (`step_type=session\|review\|gate`) |
| **Logs** | Да (`RunLogger`, INFO/WARN/ERROR) | Расширить key-value набор |
| **Traces** | Нет | `run_id` + `task_id` + `session_id` correlation (без distributed tracing) |

Traces предназначены для distributed systems — для ralph избыточны. Однако концепция span hierarchy полезна: Run → Task → Session/Review/Gate. Отражение через `run_id`, `task_id`, `step_type` key-value pairs.

### Span Types для Agent Systems

Платформы [Langfuse](https://langfuse.com/docs/tracing) и [LangSmith](https://docs.smith.langchain.com/observability/concepts#traces) определяют типичные span types:

- **Generation** — вызов LLM (в ralph: `session.Execute`)
- **Tool** — вызов инструмента (в ralph: git operations, file writes)
- **Retrieval** — поиск контекста (в ralph: task scanning, knowledge loading)
- **Agent** — высокоуровневый agent (в ralph: execute loop iteration)

Ralph может адаптировать таксономию: `step_type=generation|tool|retrieval|agent` без зависимости от конкретной платформы.

### Arize и LLM-Specific Observability

[Arize](https://arize.com/blog/the-complete-guide-to-llm-observability/) специализируется на drift detection и hallucination monitoring:

- Embedding-based similarity между input prompt и output — обнаружение off-topic responses
- Response quality scoring через reference-free evaluation
- Для ralph релевантно: мониторинг деградации review quality (если review prompt даёт менее полезные findings со временем)

---

## 5. Обнаружение и исправление проблем в реальном времени

### Runtime Guardrails (3 уровня)

**Уровень 1 — Passive monitoring** (текущий ralph):
- Логирование через `RunLogger`
- Подсчёт итераций (`MaxIterations`)
- Запись длительности сессий

**Уровень 2 — Active detection** (рекомендация):
- Similarity detection между последовательными outputs
- Token/cost budget tracking с предупреждениями на 70%/90%
- Повторяющиеся ошибки одного типа (>3 одинаковых за run)
- Stuck detection: HEAD не меняется N attempts подряд

**Уровень 3 — Automated response** (частично реализовано):
- Human gate trigger при аномалиях (`EmergencyGatePromptFn`)
- Автоматический revert при broken builds (`RevertTask`)
- Resume extraction при crashed sessions (`ResumeExtraction`)
- Feedback injection при stuck detection (`InjectFeedback`)

### Dual-Threshold система

| Метрика | Warning threshold | Hard threshold | Действие |
|---|---|---|---|
| Iteration count | `MaxIterations × 0.7` | `MaxIterations` | WARN → Force stop |
| Token/cost budget | `budget × 0.7` | `budget × 1.0` | WARN → Force stop |
| Output similarity | 0.85 (3-window) | 0.95 (3-window) | Hint injection → Human gate |
| Review cycles | `MaxReviewIter × 0.6` | `MaxReviewIter` | WARN → Emergency gate |
| Execute retries | 3 per session | `MaxIterations` | WARN → Emergency gate |

### HITL (Human-in-the-Loop) Triggers

Ralph уже имеет gate system. Рекомендуемые дополнительные triggers:

1. **Loop detection** — при similarity > 85%: "Агент выполнил 3 итерации с одинаковым diff. Продолжить/изменить подход/остановить?"
2. **Cost spike** — при cost per task > 2× median: "Текущая задача стоит $X, что в 2 раза выше среднего."
3. **Conflicting signals** — review "not clean" но git diff пуст: "Review нашёл проблемы, но нет изменений. Возможна ошибка в review."

### Self-Healing Patterns

Исследования описывают несколько архитектур самовосстановления для AI-оркестраторов:

- **Task Planner → Coder → Executor → Debugger loop**: четырёхфазная архитектура, где Debugger анализирует failures и передаёт insights обратно в Planner. Ralph's `ResumeExtraction` — аналог Debugger phase.

- [**Google CodeMender**](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/): self-correction на основе LLM judge feedback. Код проверяется вторым LLM (reviewer), findings инжектируются обратно. Ralph's review cycle — прямая реализация этого паттерна.

- **DoVer pattern**: iterative failure hypothesis → minimal intervention → validation. Ralph: findings (hypothesis) → inject feedback (intervention) → next execute + review (validation).

- [**InspectCoder**](https://arxiv.org/abs/2404.00681): interactive debugger API с breakpoints и runtime state inspection. Для ralph применим через Claude Code's `--resume` flag с checkpoint-like semantics.

- **Error pattern learning**: если одна и та же ошибка повторяется в разных задачах, добавлять в prompt: "В предыдущих задачах возникала ошибка X. Избегай подхода Y." Ralph's LEARNINGS.md + distillation — реализация этого паттерна.

### CI Integration

- **Pre-merge validation**: `go test ./...` и `go vet ./...` после каждого задания. Failure = automatic retry с test output в feedback.
- **Post-run report**: structured JSON report для consumption CI pipeline (GitHub Actions).
- [Anthropic eval approach](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents): "failures become test cases, test cases prevent regressions" — ralph может конвертировать review findings в regression tests.

---

## 6. Рекомендации для ralph

Приоритизированные рекомендации, привязанные к архитектуре: `cmd/ralph → runner → session, gates, config`.

### P0: Критично — следующий sprint (~2.5 дня, zero new deps)

**P0.1: Token & cost parsing из Claude Code JSON output.**
`session.Execute` возвращает `RawResult.Stdout`. При `OutputJSON=true`, JSON содержит usage data. Расширить `session.SessionResult`:
```go
type SessionResult struct {
    // existing fields...
    InputTokens  int
    OutputTokens int
    CachedTokens int
    CostUSD      float64
    NumTurns     int
}
```
Логировать: `tokens_in=N tokens_out=M cached=K cost_usd=X.XX turns=T`. **Effort: S (1 day).**

**P0.2: Per-task cost aggregation + RunMetrics struct.**
`RunMetrics` struct в пакете `runner`, агрегирующий данные за run:
```go
type RunMetrics struct {
    RunID           string
    StartTime       time.Time
    EndTime         time.Time
    TasksTotal      int
    TasksCompleted  int
    TasksSkipped    int
    TasksReverted   int
    TotalRetries    int
    TotalReviews    int
    TotalTokensIn   int
    TotalTokensOut  int
    TotalCostUSD    float64
    DirtyRecoveries int
}
```
При gate prompt показывать: "Task cost so far: $X.XX (N sessions)". При run finish — summary. **Effort: S (1 day).**

**P0.3: Git diff metrics after commit.**
После подтверждения нового коммита (`headAfter != headBefore`):
```go
type DiffStats struct {
    FilesChanged int
    Insertions   int
    Deletions    int
}
// GitClient interface extension:
DiffStatsSinceCommit(ctx, fromCommit string) (DiffStats, error)
```
Реализация через `git diff --numstat`. Логировать: `commit_stat files=N insertions=M deletions=K`. **Effort: S (0.5 day).**

### P1: Высокий приоритет — 1-2 sprints

**P1.1: Structured run report (JSON).** По завершении `Execute` записать `ralph-run-<runID>.json` в `LogDir` со всеми метриками: tasks, cost, per-task breakdown, gate decisions, reviews, retries, distillation. Foundation для dashboard и CI. **Effort: M (2-3 days). Depends: P0.1, P0.3.**

**P1.2: Review findings breakdown.** Модифицировать `DetermineReviewOutcome` для парсинга severity из review output:
```go
type ReviewResult struct {
    Clean    bool
    Findings []Finding // severity, category, file, description
}
```
Логировать: `review_result clean=false findings=5 critical=0 high=1 medium=3 low=1`. **Effort: M (2-3 days).**

**P1.3: Stuck detection (soft circuit breaker).** Per-task progress tracking: если `headAfter == headBefore` в N consecutive execute attempts (default N=3), inject feedback: "No progress detected. Break the task down or try a different approach." Использовать `InjectFeedback`. Новый config field: `StuckThreshold int`. **Effort: S (1 day).**

**P1.4: Gate decision aggregation.** Логировать каждое gate decision: `gate_decision action=approve task="..."`. В run report — distribution: `{approve: 12, retry: 3, skip: 1, quit: 0}`. **Effort: S (0.5 day).**

**P1.5: Structured log keys (run_id, task_id, step_type).** Добавить `run_id` (UUID), `task_id` (index), `step_type` (session/review/gate/recovery) в каждую log entry:
```
2026-03-04T10:15:30 INFO  [runner] session complete run_id=abc123 task_id=2 step_type=session duration_ms=45000
```
Превращает flat log в queryable data. **Effort: S (1 day).**

### P2: Средний приоритет — 2-3 sprints

**P2.1: Test pass rate delta.** `TestCommand string` в Config. Запускать тесты до первого session и после каждого commit. `test_delta pass_before=142 pass_after=147 fail_before=0 fail_after=0`. При fail_after > 0 — automatic retry с test output. **Effort: M (2 days).**

**P2.2: Budget alerts.** `BudgetMaxUSD float64` и `BudgetWarnPct float64` в Config. При warn% — WARN + stderr. При max — emergency gate. **Effort: S (1 day). Depends: P0.1.**

**P2.3: Latency breakdown.** Инструментировать: `prompt_assembly_ms`, `session_ms`, `git_ops_ms`, `review_ms`, `gate_wait_ms`. В run report для performance profiling. **Effort: M (2 days).**

**P2.4: Error categorization.** Parsing error prefixes → categories. Mapping в constants. Агрегация `ErrorsByCategory map[string]int` в `RunMetrics`. **Effort: S (1 day).**

**P2.5: Loop detection (similarity-based).** Window-based: хранить последние N (default 3) git diff outputs. Jaccard similarity. Warning → hint. Hard → gate. Файл `runner/loop_detect.go`. **Effort: M (2-3 days).**

### P3: Backlog

**P3.1: OpenTelemetry-compatible naming.** Переименовать keys: `gen_ai.usage.input_tokens`. Не добавляет функциональности, упрощает интеграцию. **Effort: S.**

**P3.2: Langfuse integration (optional).** HTTP API (не SDK) для отправки trace data. Feature flag `observability.langfuse.enabled`. **Effort: M.**

**P3.3: Terminal UI dashboard.** Live метрики через `charmbracelet/bubbletea`. Текущая задача, cost, tokens, health. Высокая UX-ценность для long-running sessions. **Effort: L. Requires new dependency.**

**P3.4: Trend analysis CLI.** `ralph metrics --last 10` — агрегация по JSON run reports. Средняя стоимость, время, findings, тренды. **Effort: L (3-5 days). Depends: P1.1.**

**P3.5: Code complexity delta.** Интеграция с `gocyclo` до/после. Предупреждение при значительном росте. **Effort: S. Requires external tool.**

**P3.6: Coverage delta.** `go test -coverprofile` до/после. Diff coverage report. **Effort: M. Two test runs per task = expensive.**

### Матрица: пакеты и зависимости

| ID | Затрагиваемые пакеты | Новые зависимости | Изменение interface |
|---|---|---|---|
| P0.1 | `session`, `runner` | Нет | `SessionResult` parsing |
| P0.2 | `runner` | Нет | Нет |
| P0.3 | `runner` | Нет | `GitClient` interface |
| P1.1 | `runner` | Нет | Нет |
| P1.2 | `runner` | Нет | `ReviewResult` struct |
| P1.3 | `runner`, `config` | Нет | `Config` extension |
| P1.4 | `runner` | Нет | Нет |
| P1.5 | `runner` (log.go) | Нет | Нет |
| P2.1 | `runner`, `config` | Нет | `Config` extension |
| P2.2 | `runner`, `config` | Нет | `Config` extension |
| P2.3 | `runner` | Нет | Нет |
| P2.4 | `runner`, `config` | Нет | Constants |
| P2.5 | `runner` (new file) | Нет | `Runner` struct field |
| P3.2 | `runner` (new file) | `net/http` (stdlib) | Optional export |
| P3.3 | Новый пакет | `bubbletea` | Новая команда |
| P3.4 | `cmd/ralph` | Нет | Новая команда |

**Важно**: P0-P2 не требуют новых внешних зависимостей — соответствует правилу "Only 3 direct deps, new deps require justification". P3.3 — единственная рекомендация с новой зависимостью.

---

## 7. Источники

### Стандарты и фреймворки
- [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/)
- [OpenTelemetry GenAI Agent Spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/)
- [OpenTelemetry GenAI Metrics](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-metrics/)
- [AI Agent Observability — OTel Blog 2025](https://opentelemetry.io/blog/2025/ai-agent-observability/)
- [MELT Framework (Splunk)](https://www.splunk.com/en_us/data-insider/melt-metrics-events-logs-traces.html)

### Observability платформы
- [Langfuse — Open Source LLM Observability](https://langfuse.com/)
- [Langfuse Token/Cost Tracking](https://langfuse.com/docs/observability/features/token-and-cost-tracking)
- [LangSmith Observability](https://www.langchain.com/langsmith/observability)
- [Arize — AI Agent Observability 2026](https://arize.com/blog/best-ai-observability-tools-for-autonomous-agents-in-2026/)
- [Braintrust — AI Observability](https://www.braintrust.dev/articles/best-ai-observability-tools-2026)
- [Datadog LLM Observability + OTel](https://www.datadoghq.com/blog/llm-otel-semantic-convention/)
- [SigNoz LLM Observability Tools](https://signoz.io/comparisons/llm-observability-tools/)
- [Traceloop — Token Usage & Cost](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user)

### Конкуренты и аналоги
- [Aider — AI Pair Programming](https://aider.chat/)
- [Aider Analytics](https://aider.chat/docs/more/analytics.html)
- [SWE-bench](https://www.swebench.com/) — resolve rate benchmark
- [SWE-bench Verified (OpenAI)](https://openai.com/index/introducing-swe-bench-verified/)
- [Devin / Cognition](https://devin.ai/)
- [Cline — VS Code AI Agent](https://github.com/cline/cline)
- [Antigravity (Google)](https://blog.google/technology/google-labs/project-mariner-antigravity/)
- [Claude Code Documentation](https://docs.anthropic.com/en/docs/claude-code)
- [Claude Code Hooks](https://docs.anthropic.com/en/docs/claude-code/hooks)
- [OpenAI Codex CLI](https://github.com/openai/codex)
- [OpenAI Codex Issue #5085 — Cost Tracking](https://github.com/openai/codex/issues/5085)
- [2026 Guide to Coding CLI Tools Compared](https://www.tembo.io/blog/coding-cli-tools-comparison)
- [Top 5 AI Agent Observability Platforms](https://o-mega.ai/articles/top-5-ai-agent-observability-platforms-the-ultimate-2026-guide)
- [15 AI Agent Observability Tools](https://research.aimultiple.com/agentic-monitoring/)

### Self-healing и guardrails
- [Google CodeMender](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/)
- [InspectCoder (arXiv:2404.00681)](https://arxiv.org/abs/2404.00681)
- [Real-Time Guardrails for Agentic Systems](https://www.akira.ai/blog/real-time-guardrails-agentic-systems)
- [Fixing Infinite Loop AI Agent](https://techbytes.app/posts/fixing-infinite-loop-ai-agent-refuses-stop-coding/)
- [Agentic Guardrails (GitHub)](https://github.com/FareedKhan-dev/agentic-guardrails)
- [Self-Debugging AI Agent Architecture](https://medium.com/@atnoforaimldl/we-coded-an-ai-agent-that-can-debug-its-own-errors-heres-the-architecture-bdf5e72f87ce)
- [Self-Healing Software Systems (arXiv)](https://arxiv.org/pdf/2504.20093)
- [Dagger — Self-Healing CI Pipelines](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents)

### Методологии оценки
- [Anthropic — Demystifying Evals for AI Agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
- [Microsoft — Agent Observability Best Practices](https://azure.microsoft.com/en-us/blog/agent-factory-top-5-agent-observability-best-practices-for-reliable-ai/)
- [Deloitte — AI Agent Orchestration 2026](https://www.deloitte.com/us/en/insights/industry/technology/technology-media-and-telecom-predictions/2026/ai-agent-orchestration.html)
- [Code Quality Metrics 2026 (Qodo)](https://www.qodo.ai/blog/code-quality-metrics-2026/)
