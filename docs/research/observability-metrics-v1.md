# Исследование: Метрики и наблюдаемость для AI-оркестраторов разработки

**Версия:** v1 (обновлено)
**Дата:** 2026-03-04
**Проект:** bmad-ralph (Go CLI, оркестратор Claude Code сессий)

---

## 1. Введение

Ralph -- Go CLI-инструмент, оркестрирующий автономные сессии Claude Code для разработки программного обеспечения. Основной цикл работы: чтение задач из sprint-артефактов, формирование prompt, вызов Claude Code (`session.Execute`), проверка git commit (`HeadCommit`), code review (`RunReview`/`RealReview`), gate check (`gates.Evaluate`), переход к следующей задаче. На текущий момент ralph располагает базовым логированием через `RunLogger` (пакет `runner/log.go`) с уровнями INFO/WARN/ERROR, записью в файл и stderr. Логируются: старт/финиш запуска, начало/завершение задач, длительность сессии, результаты commit check, retry-попытки и циклы review.

Рынок автономных AI-агентов для разработки стремительно растет. Наблюдаемость (observability) становится критическим дифференциатором -- без полноценных метрик невозможно ответить на фундаментальные вопросы: "Сколько стоит одна задача?", "Где агент теряет время?", "Когда нужно вмешательство человека?", "Не зациклился ли агент?".

AI-оркестраторы обладают уникальными свойствами, требующими специализированной observability:

- **Недетерминизм**: LLM-вызовы дают разные результаты при одинаковых входах, что требует статистического анализа качества.
- **Стоимость**: каждый вызов Claude Code потребляет token-ы с реальной денежной стоимостью; без cost tracking невозможно оптимизировать бюджет.
- **Loop-риски**: автономный агент может зациклиться, тратя ресурсы без прогресса -- нужны guardrail-метрики.
- **Многоуровневая обратная связь**: результат зависит от цепочки шагов (prompt -> code -> test -> review -> gate), и сбой на любом уровне требует точной диагностики.

Фреймворк [MELT (Metrics, Events, Logs, Traces)](https://opentelemetry.io/docs/) задает стандартную структуру наблюдаемости для агентных систем. Ralph уже покрывает букву "L" (Logs) через `RunLogger`, но "M", "E" и "T" остаются неохваченными. Цель этого исследования -- систематизировать лучшие практики индустрии (2024-2026) и выработать приоритизированные рекомендации для ralph с учетом его текущей архитектуры.

---

## 2. Обзор конкурентов и аналогов

Ниже представлен сравнительный анализ ключевых инструментов по критерию наблюдаемости и метрик.

| Инструмент | Тип | Token tracking | Cost tracking | Loop detection | Quality metrics | Observability API/Export |
|---|---|---|---|---|---|---|
| **[Aider](https://aider.chat/)** | CLI, open-source | Да (input/output per model) | Да ($/model, cumulative) | Нет (ручной лимит) | Blame-based stats | Analytics opt-in, локальные метрики |
| **[SWE-agent](https://github.com/princeton-nlp/SWE-agent)** | CLI, open-source | Да (per step) | Частично (через API) | Да (max_steps) | SWE-bench score | Trajectory logging, JSON export |
| **[Devin](https://devin.ai/)** (Cognition) | Cloud IDE | Скрытый | ACU (compute units) | Внутренний | Внутренний | Cloud dashboard, нет export API |
| **[Cline](https://github.com/cline/cline)** | VS Code extension | Да (per request) | Да (running total) | Нет | Нет | VS Code UI, нет structured export |
| **[Antigravity](https://blog.google/technology/google-labs/project-mariner-antigravity/)** (Google) | Agent IDE | Внутренний | Внутренний | Manager view | Внутренний | Manager orchestration dashboard |
| **[OpenAI Codex](https://github.com/openai/codex)** | CLI, open-source | Запрошен ([issue #5085](https://github.com/openai/codex/issues/5085)) | Запрошен | Нет | Нет | Минимальный (stdout) |
| **[Claude Code](https://docs.anthropic.com/en/docs/claude-code)** | CLI | Да (per session via API) | Через API pricing | Нет (max_turns) | 80.8% SWE-bench | Hooks system, JSON output mode |

Источники: [Tembo: Coding CLI Tools Comparison](https://www.tembo.io/blog/coding-cli-tools-comparison), [Cline: Top 6 Claude Code Alternatives](https://cline.bot/blog/top-6-claude-code-alternatives-for-agentic-coding-workflows-in-2025), [AIMultiple: Agentic CLI](https://aimultiple.com/agentic-cli), [DigitalOcean: Claude Code Alternatives](https://www.digitalocean.com/resources/articles/claude-code-alternatives).

### Ключевые наблюдения

**Aider** наиболее близок к ralph по архитектуре (CLI, git-aware, loop). Он отслеживает token usage по моделям с кумулятивной стоимостью и предоставляет blame-based contribution statistics -- метрику, показывающую какой процент кода в репозитории написан AI. Однако loop detection и quality metrics отсутствуют. Analytics opt-in позволяет агрегировать анонимные данные по benchmark-задачам.

**SWE-agent** выделяется trajectory logging -- полным журналом действий агента в JSON-формате с поддержкой parallel tool calls и context management, что позволяет детальный offline-анализ. Подход с `max_steps` как единственным ограничителем loop примитивен, но функционален для benchmark-сценариев.

**Devin** (Cognition) -- cloud-based AI Software Engineer, недавно приобретший Windsurf за $250M, представляет собой hybrid chatbot/IDE. Имеет внутренние dashboards, но не предоставляет API для export метрик -- закрытая экосистема. Для CLI-инструмента вроде ralph это anti-pattern: пользователь должен владеть своими данными.

**Claude Code** -- runtime, который ralph оркестрирует -- предоставляет JSON output mode (`--output-format json`), что ralph уже использует через `session.Options.OutputJSON`. Это основной канал получения structured data из сессий. [Hooks system](https://docs.anthropic.com/en/docs/claude-code/hooks) позволяет инжектировать pre/post обработку. Claude Code достиг 80.8% на SWE-bench Verified и поддерживает Agent Teams.

**OpenAI Codex CLI** -- примечателен тем, что [issue #5085](https://github.com/openai/codex/issues/5085) с запросом cost tracking набрал значительную поддержку сообщества, подтверждая рыночный спрос на эту функцию в CLI-агентах.

**Ни один инструмент не предоставляет полной observability-системы.** Комплексная observability с метриками качества, стоимости и loop health остается незанятой нишей. Ralph, уже имеющий `MaxIterations`, gate-систему и review pipeline, находится в выгодной позиции для заполнения этой ниши.

---

## 3. Категории метрик

### 3.1 Runtime-метрики (производительность)

Runtime-метрики отвечают на вопрос "как быстро и эффективно работает агент?".

| Метрика | Описание | Источник в ralph | Статус |
|---|---|---|---|
| `session.duration_ms` | Wall-clock время одной Claude Code сессии | `session.Execute` return | Логируется (INFO) |
| `session.tokens.input` | Input tokens per session | Claude Code JSON output | **Не собирается** |
| `session.tokens.output` | Output tokens per session | Claude Code JSON output | **Не собирается** |
| `task.duration_ms` | Полное время обработки задачи (sessions + reviews + gates) | `Runner.execute` | Логируется частично |
| `run.duration_ms` | Полное время `Runner.Execute` (все задачи) | `Runner.Execute` | Логируется (INFO) |
| `retry.count` | Количество retry per task | Execute loop | Логируется (WARN) |
| `retry.backoff_ms` | Время ожидания между retry | `SleepFn` | **Не собирается** |
| `review.cycles` | Количество review-циклов per task | Execute loop | Логируется (INFO) |
| `git.diff.files_changed` | Файлов изменено в commit | `git diff --stat` | **Не собирается** |
| `git.diff.insertions` | Строк добавлено | `git diff --numstat` | **Не собирается** |
| `git.diff.deletions` | Строк удалено | `git diff --numstat` | **Не собирается** |
| `time_to_first_commit` | От начала задачи до первого успешного commit | Execute loop timing | **Не собирается** |

**Token usage** -- наиболее критичная недостающая метрика. Claude Code в JSON output mode возвращает structured response, из которой можно извлечь token counts. Для ralph это означает parsing `session.RawResult.Stdout` для извлечения usage metadata.

**Git diff stats** доступны через существующий `GitClient` interface в ralph (`runner.Runner.Git`). Добавление метода `DiffStats(from, to string) (DiffStats, error)` позволит собирать данные о размере изменений. Token efficiency ratio (`useful_output_tokens / total_tokens`) покажет, какая доля token-ов привела к принятому коду.

### 3.2 Метрики качества кода

Метрики качества отвечают на вопрос "насколько хорош результат работы агента?".

| Метрика | Описание | Источник | Статус |
|---|---|---|---|
| `review.findings.total` | Общее число findings per review | `ReviewResult` | **Не собирается** (Clean bool только) |
| `review.findings.by_severity` | Findings по severity (C/H/M/L) | Review output parsing | **Не собирается** |
| `review.fix_rate` | Доля findings, исправленных за один цикл | Review delta | **Не собирается** |
| `review.cycles_per_task` | Сколько раз review потребовал переделки | Execute loop | Логируется |
| `build.success` | Успешность `go build` after commit | Post-commit check | **Не реализовано** |
| `test.pass_rate` | Доля проходящих тестов after commit | Post-commit check | **Не реализовано** |
| `test.new_added` | Новые тесты добавленные за задачу | Diff analysis | **Не реализовано** |
| `lint.warnings_delta` | Изменение числа lint warnings | Pre/post comparison | **Не реализовано** |
| `coverage.delta` | Изменение code coverage | Pre/post comparison | **Не реализовано** |
| `complexity.delta` | Изменение cyclomatic complexity | Pre/post comparison | **Не реализовано** |

Текущая архитектура ralph использует `ReviewResult` со структурой `{Clean bool}` -- binary signal. Расширение до `{Clean bool; TotalFindings int; BySeverity map[string]int}` даст гранулярные данные без изменения review flow.

Из опыта ralph Epics 1-6 (226 findings across 40 stories), top recurring категории findings: assertion quality, doc comment accuracy, duplicate code, error wrapping. Классификация findings по этим категориям позволит отслеживать тренды улучшения качества генерации кода.

### 3.3 Метрики стоимости

Метрики стоимости отвечают на вопрос "сколько стоит работа агента?".

| Метрика | Описание | Формула | Статус |
|---|---|---|---|
| `cost.per_session` | Стоимость одной Claude Code сессии | `tokens * price_per_token` | **Не собирается** |
| `cost.per_task` | Полная стоимость задачи (все sessions + reviews) | `sum(sessions + reviews)` | **Не собирается** |
| `cost.per_review_cycle` | Стоимость одного review-цикла | `review_tokens * price` | **Не собирается** |
| `cost.per_retry` | Стоимость retry-попытки (waste metric) | `retry_tokens * price` | **Не собирается** |
| `cost.cumulative` | Нарастающий итог за run | `sum(all_sessions)` | **Не собирается** |
| `cost.efficiency` | Стоимость на строку кода | `cost / (insertions + deletions)` | **Не собирается** |
| `cost.per_accepted_loc` | Стоимость принятой строки (post-review) | `cost / accepted_loc` | **Не собирается** |
| `cost.waste_ratio` | Доля затрат на неуспешные попытки | `retry_cost / total_cost` | **Не собирается** |
| `cost.budget_remaining` | Остаток бюджета (если задан лимит) | `budget - cumulative` | **Не реализовано** |
| `cost.burn_rate` | Скорость расходования бюджета | `cumulative / elapsed_time` | **Не реализовано** |

Стоимость вычислима из token counts при известных ценах модели. [Anthropic pricing](https://www.anthropic.com/pricing) для Claude моделей публичен и обновляется нечасто. Ralph может хранить pricing table в `config.Config` или отдельном YAML-файле.

Подходы к cost tracking из индустрии: [Traceloop](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user) реализует per-user token/cost tracking через OpenTelemetry, [Portkey](https://portkey.ai/blog/tracking-llm-token-usage-across-providers-teams-and-workloads/) обеспечивает cross-provider token tracking. Для ralph как CLI-инструмента оптимален простой подход: pricing table + token extraction из JSON output.

**Cost budget** -- мощный guardrail: пользователь задает `--max-cost 5.00`, ralph останавливается при достижении лимита. Это аналог `--max-turns` в Claude Code, но в денежном выражении.

### 3.4 Метрики здоровья loop (петель)

Метрики здоровья loop отвечают на вопрос "не застрял ли агент?".

| Метрика | Описание | Механизм | Статус |
|---|---|---|---|
| `loop.iteration` | Текущий номер итерации | Execute counter | Логируется |
| `loop.max_iterations` | Лимит итераций | `config.Config.MaxIterations` | Реализовано |
| `loop.similarity_score` | Similarity текущего output с предыдущими N | Window-based comparison | **Не реализовано** |
| `loop.stuck_detected` | Детекция зацикливания | Threshold crossing | **Не реализовано** |
| `loop.circuit_breaker` | Принудительная остановка при repeated patterns | Pattern matcher | **Не реализовано** |
| `loop.error_repetition` | Один тип ошибки повторяется N раз подряд | Error pattern tracker | **Не реализовано** |
| `gate.action` | Решение gate (approve/retry/skip/abort) | `GateDecision.Action` | Логируется |
| `gate.feedback_injected` | Был ли injected feedback от человека | `GateDecision.Feedback` | Логируется |
| `gate.consecutive_rejections` | Количество последовательных отклонений gate | Counter | **Не собирается** |

**Loop detection** -- ключевая проблема автономных агентов. Индустриальный подход использует dual-threshold систему:

1. **Warning threshold** (soft): при similarity > 0.85 за последние 3 итерации -- WARN, hint injection в prompt.
2. **Hard threshold** (force): при similarity > 0.95 за последние 3 итерации -- принудительная остановка или human gate trigger.

Window-based similarity detection использует скользящее окно последних N outputs. Для ralph достаточно простого подхода: сравнение git diff output между итерациями. Если diff идентичен или пуст в N последовательных итерациях, агент застрял.

Ralph уже имеет `MaxIterations` как hard stop и `GatePromptFn`/`EmergencyGatePromptFn` для human intervention. Добавление similarity check между этими уровнями создаст трехуровневую защиту: similarity warning -> human gate -> hard stop.

**Circuit breakers** применяются к нескольким измерениям: max iterations per task, max total cost per run, max consecutive failures (одинакового типа), max review cycles (ralph уже имеет review retry logic), emergency stop conditions (критическая ошибка, потеря данных).

### 3.5 Метрики самовосстановления

Метрики самовосстановления отвечают на вопрос "насколько устойчив агент к ошибкам?".

| Метрика | Описание | Источник в ralph | Статус |
|---|---|---|---|
| `recovery.dirty_state` | Успешность recovery из dirty state | `RecoverDirtyState` | Логируется (INFO/ERROR) |
| `recovery.resume_extract` | Успешность извлечения resume context | `ResumeExtraction` | Логируется частично |
| `recovery.revert` | Количество revert операций | `RevertTask` | Логируется |
| `recovery.skip` | Количество skip операций | `SkipTask` | Логируется |
| `error.category` | Классификация (transient/persistent) | Error wrapping prefix | **Не собирается** |
| `error.auto_recovery_rate` | % автоматически восстановленных ошибок | `recovery.* / error.*` | **Не собирается** |
| `error.escalation_rate` | % ошибок, потребовавших human intervention | Gate trigger count | **Не собирается** |
| `recovery.time_ms` | Время от ошибки до успешного продолжения | Timing delta | **Не собирается** |
| `debug.self_correction_cycles` | Сколько раз агент исправил собственную ошибку | Review retry success | **Не собирается** |

Ralph уже имеет развитую систему error recovery: `RecoverDirtyState` для восстановления из грязного git state, `ResumeExtraction` для извлечения контекста из прерванных сессий, `RevertTask`/`SkipTask` для управления задачами при ошибках. `RunLogger` логирует эти события, но не агрегирует их в метрики.

**Error categorization** может быть реализована через parsing error wrapping prefixes, которые ralph использует единообразно (e.g., `"runner: scan tasks:"`, `"runner: dirty state recovery:"`). Transient errors (rate limit, timeout) отличаются от persistent errors (bad prompt, impossible task) -- стратегия recovery должна зависеть от типа.

---

## 4. Стандарты наблюдаемости

### OpenTelemetry GenAI Semantic Conventions

[OpenTelemetry](https://opentelemetry.io/) выпустил [GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) (статус: experimental, активно развиваются), определяющие стандартные атрибуты для LLM spans:

- `gen_ai.system` -- провайдер (e.g., `"anthropic"`)
- `gen_ai.request.model` -- модель (e.g., `"claude-sonnet-4-20250514"`)
- `gen_ai.request.max_tokens` -- лимит output tokens
- `gen_ai.usage.input_tokens` -- фактическое потребление input tokens
- `gen_ai.usage.output_tokens` -- фактическое потребление output tokens
- `gen_ai.response.finish_reason` -- причина завершения (stop, max_tokens, error)

Для ralph применение полного OpenTelemetry SDK было бы избыточным (overhead на single-threaded CLI). Однако следование naming conventions в метриках обеспечит совместимость с экосистемой при будущей интеграции с observability-платформами.

### MELT Framework

Фреймворк MELT (Metrics, Events, Logs, Traces) определяет четыре столпа наблюдаемости:

| Столп | Описание | Текущее покрытие в ralph | Рекомендация |
|---|---|---|---|
| **Metrics** | Числовые агрегаты: token count, latency percentiles, cost totals | Нет (implicit в логах) | RunMetrics struct, JSON summary |
| **Events** | Дискретные события с metadata: task_started, commit_created | Частично (log lines) | Structured events с типами |
| **Logs** | Текстовый поток выполнения | Да (`RunLogger`, INFO/WARN/ERROR) | Расширить key-value набор |
| **Traces** | Распределенные цепочки вызовов | Нет | Не нужно для single-process CLI |

**Traces** предназначены для distributed systems и для ralph избыточны. Однако концепция span hierarchy полезна: Run -> Task -> Session/Review/Gate -- это естественная иерархия, которую можно отразить в structured log entries через `run_id`, `task_id`, `step_type` key-value pairs.

### Span Types для Agent Systems

Платформы [Langfuse](https://langfuse.com/docs/observability/overview), [LangSmith](https://www.langchain.com/langsmith/observability) и [Braintrust](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026) определяют типичные span types для агентных систем:

- **Generation** -- вызов LLM (в ralph: `session.Execute`)
- **Tool** -- вызов инструмента (в ralph: git operations, file writes)
- **Retrieval** -- поиск контекста (в ralph: task scanning, knowledge loading)
- **Agent** -- высокоуровневый агент (в ralph: execute loop iteration)

Каждый span несет атрибуты: duration, status (ok/error), token usage (для generation spans), error message. Ralph может адаптировать эту таксономию для structured logging через `step_type=generation|tool|retrieval|agent`.

Рекомендуемая иерархия span-ов для ralph:

```
run (top-level)
  task (per story/task)
    session (Claude Code invocation)
      prompt_build
      claude_execution
      result_parse
    git_check
    review (optional)
      review_session
      review_parse
    gate_check (optional)
      gate_prompt
      gate_decision
    retry (if applicable)
```

Источники: [Datadog LLM Observability](https://www.datadoghq.com/product/llm-observability/), [SigNoz: LLM Observability Tools](https://signoz.io/comparisons/llm-observability-tools/).

---

## 5. Обнаружение и исправление проблем в реальном времени

### Runtime Guardrails

Современные AI-оркестраторы используют многоуровневую систему guardrails:

**Уровень 1 -- Passive monitoring** (текущий ralph):
- Логирование событий через `RunLogger`
- Подсчет итераций (`MaxIterations`)
- Запись длительности сессий

**Уровень 2 -- Active detection** (рекомендация для ralph):
- Similarity detection между последовательными outputs
- Token budget tracking с предупреждениями на 70%/90%
- Повторяющиеся ошибки одного типа (>3 одинаковых за run)
- Progress indicators: задача не продвигается (нет commits, тесты не проходят, findings не уменьшаются)

**Уровень 3 -- Automated response** (частично в ralph):
- Human gate trigger при аномалиях (ralph: `EmergencyGatePromptFn`)
- Автоматический revert при broken builds (ralph: `RevertTask`)
- Resume extraction при crashed sessions (ralph: `ResumeExtraction`)

### Dual-Threshold система

Подход, зарекомендовавший себя в индустрии:

| Метрика | Warning threshold | Hard threshold | Действие при Warning | Действие при Hard |
|---|---|---|---|---|
| Iteration count | `MaxIterations * 0.75` | `MaxIterations` | WARN log | Force stop |
| Token cost | `budget * 0.80` | `budget * 1.0` | WARN log | Force stop |
| Output similarity | 0.85 (3-window) | 0.95 (3-window) | Hint injection | Human gate |
| Review cycles | 2 per task | `maxReviewRetries` | WARN log | Skip task |
| Retry count | 3 per session | 5 per session | WARN log | Force stop |
| Time per task | 30 min | 60 min | WARN log | Human gate |
| Consecutive failures | 3 | 5 | WARN log | Force stop |

Warning threshold: логирование WARN, injection дополнительного контекста в prompt ("Ты приближаешься к лимиту, сосредоточься на завершении"). Hard threshold: принудительное прерывание текущего шага, переход к gate или skip.

### HITL (Human-in-the-Loop) Triggers

Ralph уже имеет gate-систему (`GatePromptFn`, `EmergencyGatePromptFn`). Рекомендуемые автоматические HITL triggers:

1. **Loop detected** -- similarity score превысил warning threshold. Контекст для human: "Агент выполнил 3 итерации с similarity > 85%. Продолжить/изменить подход/остановить?"
2. **Conflicting signals** -- тесты проходят, но review находит critical findings. Или: review says "not clean", но diff пуст.
3. **High-value action** -- удаление файлов, изменение конфигурации CI, модификация security-критичного кода.
4. **Budget approaching** -- cost run приближается к hard limit.
5. **Cost spike** -- стоимость текущей задачи > 2x median: "Задача стоит $X, что в 2 раза выше среднего."
6. **Unprecedented error** -- ошибка, не встречавшаяся в предыдущих runs.

### Self-Debugging Patterns

Архитектура самоисправления для AI-оркестраторов следует паттерну Task Planner -> Coder -> Executor -> Debugger, описанному в [исследованиях InspectCoder/VulDebugger](https://www.emergentmind.com/topics/self-debugging-agent):

1. **Error classification**: при ошибке определить тип -- compilation, test failure, runtime error, review rejection. Каждый тип требует своей стратегии recovery.
2. **Context enrichment**: добавить в prompt информацию об ошибке, stack trace, предыдущие попытки исправления. Ralph уже делает это через `InjectFeedback` при review retry.
3. **Strategy selection**: для каждого типа ошибки -- своя стратегия (recompile, fix test, revert and retry, escalate). Формализация через error -> strategy mapping.
4. **Meta-observation**: отслеживать эффективность debugging -- если три попытки исправления не помогли, эскалация. Meta-agents наблюдают за debugging effectiveness.
5. **Resume manifests**: сохранение точного состояния при сбое для продолжения с точки отказа (ralph: `ResumeExtraction`).

[Google CodeMender](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/) демонстрирует паттерн self-correction на основе LLM judge feedback -- агент генерирует код, LLM-judge оценивает его, агент корректирует. Ralph реализует аналогичный паттерн через review -> feedback injection -> retry.

[Self-healing CI pipelines](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents) показывают, как AI-агенты могут автоматически исправлять CI-сбои. Для ralph аналогичный подход: при build failure после commit -- автоматический retry с build error в prompt.

---

## 6. Рекомендации для ralph

Рекомендации приоритизированы от P0 (критично, минимальные усилия, максимальная отдача) до P3 (nice-to-have, значительные усилия).

### P0 -- Немедленная реализация (1-2 stories)

**P0.1: RunMetrics struct и JSON summary**

Создать `RunMetrics` struct в пакете `runner`, агрегирующий данные за весь run. В `Runner.Execute`: инкрементировать счетчики при каждом событии. В конце run: записать JSON summary.

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

Место хранения: `<logDir>/ralph-metrics-<runID>.json`. Все события уже логируются -- нужна только агрегация.

**Усилия**: ~200 LOC, затрагивает `runner/runner.go` (сбор), `runner/metrics.go` (новый файл).

**P0.2: Расширение key-value пар в RunLogger**

Добавить `run_id` (UUID per run), `task_id` (index or name), `step_type` (session/review/gate/recovery) в каждую log entry:

```
2026-03-04T10:15:30 INFO  [runner] session complete run_id=abc123 task_id=story-5.1 step_type=session duration_ms=45000
```

`RunLogger.write` уже принимает `kvs ...string`, нужно расширить вызовы.

**Усилия**: ~80 LOC, только `runner/log.go` и вызовы в `runner/runner.go`.

### P1 -- Следующий epic (3-5 stories)

**P1.1: Token usage extraction из Claude Code JSON output**

Парсить `session.RawResult.Stdout` для извлечения `usage.input_tokens`, `usage.output_tokens`. Defensive parsing с fallback при отсутствии полей.

```go
type SessionMetrics struct {
    InputTokens  int    `json:"input_tokens"`
    OutputTokens int    `json:"output_tokens"`
    DurationMs   int64  `json:"duration_ms"`
    Model        string `json:"model,omitempty"`
}
```

**P1.2: Cost calculation**

Pricing table в config (model -> price per input/output token). Вычисление `cost.per_session`, `cost.per_task`, `cost.cumulative`:

```
cost_usd=0.0142 cumulative_cost_usd=0.3580 budget_remaining_usd=4.65
```

**P1.3: ReviewResult enrichment**

Расширить `ReviewResult` с binary `{Clean bool}` до гранулярной структуры:

```go
type ReviewResult struct {
    Clean         bool
    TotalFindings int
    BySeverity    map[string]int // "critical", "high", "medium", "low"
}
```

Review prompt (`runner/prompts/review.md`) уже определяет формат findings с severity -- нужен парсер.

**P1.4: Git diff stats**

Добавить метод в `GitClient` interface:

```go
DiffStatsSinceCommit(commit string) (DiffStats, error)
```

Реализация через `git diff --numstat`. Вызов после успешного commit check.

### P2 -- Observability epic (5-8 stories)

**P2.1: Loop detection (similarity-based)**

Window-based similarity detection: хранить последние N (default 3) git diff outputs. Вычислять Jaccard similarity. При crossing warning threshold (0.85) -- inject hint в prompt. При crossing hard threshold (0.95) -- trigger human gate.

Реализация: `runner/loop_detect.go`, новый field в `Runner` struct. Не требует внешних зависимостей.

```go
func outputSimilarity(prev, curr string) float64 {
    prevLines := toLineSet(prev)
    currLines := toLineSet(curr)
    intersection := setIntersection(prevLines, currLines)
    union := setUnion(prevLines, currLines)
    if len(union) == 0 { return 1.0 } // both empty = identical
    return float64(len(intersection)) / float64(len(union))
}
```

**P2.2: Cost budget guardrail**

Новый config parameter `MaxCostUSD float64`, CLI flag `--max-cost`. Проверка в Execute loop с dual-threshold:

```go
if r.metrics.CumulativeCost > r.Cfg.MaxCostUSD * 0.8 {
    r.Logger.Warn("approaching cost budget", "remaining_pct", "20%")
}
if r.metrics.CumulativeCost > r.Cfg.MaxCostUSD {
    return ErrCostBudgetExhausted
}
```

**P2.3: Metrics JSON export**

По завершении run: полный `RunMetrics` как JSON в `<logDir>/ralph-metrics-<runID>.json`. Позволяет пост-анализ через `jq`, trend analysis, dashboard интеграцию.

**P2.4: Error categorization**

Parsing error wrapping prefixes для классификации: `runner: git:` -> git, `runner: session:` -> model, `runner: scan:` -> filesystem. Агрегация в `RunMetrics.ErrorsByCategory`.

### P3 -- Будущие улучшения

**P3.1: OpenTelemetry-compatible naming**

Переименовать key-value keys для совместимости с GenAI conventions: `gen_ai.usage.input_tokens`. Не добавляет функциональности, но упрощает будущую интеграцию.

**P3.2: Langfuse integration (optional)**

Опциональная интеграция через HTTP API (не SDK). Feature flag: `observability.langfuse.enabled`. Минимальный payload: traces с spans. Добавляет только `net/http` (stdlib).

**P3.3: Terminal dashboard**

Terminal UI (через `charmbracelet/bubbletea`) с live метриками: текущая задача, cost, tokens, health indicators. Значительные усилия, но высокая UX-ценность.

**P3.4: Trend analysis CLI command**

Новая команда `ralph metrics --last 10` для анализа последних N runs: средняя стоимость, время, findings, тренды.

### Сводная таблица

| Приоритет | Рекомендация | LOC | Новые deps | Затрагиваемые пакеты |
|---|---|---|---|---|
| P0.1 | RunMetrics struct + JSON | ~200 | Нет | `runner` |
| P0.2 | Structured log keys | ~80 | Нет | `runner` |
| P1.1 | Token extraction | ~120 | Нет | `session`, `runner` |
| P1.2 | Cost calculation | ~100 | Нет | `runner`, `config` |
| P1.3 | ReviewResult enrichment | ~150 | Нет | `runner` |
| P1.4 | Git diff stats | ~100 | Нет | `runner` (GitClient) |
| P2.1 | Loop detection | ~200 | Нет | `runner` (new file) |
| P2.2 | Cost budget guardrail | ~80 | Нет | `runner`, `config` |
| P2.3 | Metrics JSON export | ~100 | Нет | `runner` |
| P2.4 | Error categorization | ~100 | Нет | `runner`, `config` |
| P3.1 | OTel naming | ~50 | Нет | `runner` |
| P3.2 | Langfuse integration | ~300 | `net/http` (stdlib) | `runner` (new file) |
| P3.3 | Terminal dashboard | ~500 | `bubbletea` | Новый пакет |
| P3.4 | Trend CLI command | ~200 | Нет | `cmd/ralph` |

**Суммарный объем P0:** ~280 LOC -- реализуемо за 1-2 stories.
**Суммарный объем P0+P1:** ~750 LOC -- один epic из 4-6 stories.
**P0-P2 не требуют новых внешних зависимостей**, что соответствует принципу ralph "Only 3 direct deps, new deps require justification".

---

## 7. Источники

### Стандарты и фреймворки
- [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) -- стандартные атрибуты для LLM spans
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/) -- основной фреймворк distributed tracing

### Observability-платформы
- [Langfuse -- Open Source LLM Observability](https://langfuse.com/docs/observability/overview) -- traces, scores, cost tracking
- [LangSmith Observability](https://www.langchain.com/langsmith/observability) -- LangChain tracing и мониторинг
- [Braintrust: Best LLM Monitoring Tools 2026](https://www.braintrust.dev/articles/best-llm-monitoring-tools-2026) -- обзор инструментов
- [Datadog LLM Observability](https://www.datadoghq.com/product/llm-observability/) -- enterprise LLM monitoring
- [SigNoz: LLM Observability Tools Comparison](https://signoz.io/comparisons/llm-observability-tools/) -- сравнение open-source решений

### Cost и token tracking
- [Traceloop: From Bills to Budgets -- LLM Token Usage and Cost Per User](https://www.traceloop.com/blog/from-bills-to-budgets-how-to-track-llm-token-usage-and-cost-per-user) -- per-user cost tracking через OTel
- [Portkey: Tracking LLM Token Usage Across Providers, Teams, and Workloads](https://portkey.ai/blog/tracking-llm-token-usage-across-providers-teams-and-workloads/) -- cross-provider tracking

### Конкуренты и аналоги
- [Tembo: Coding CLI Tools Comparison](https://www.tembo.io/blog/coding-cli-tools-comparison) -- сравнение CLI coding tools
- [Cline: Top 6 Claude Code Alternatives](https://cline.bot/blog/top-6-claude-code-alternatives-for-agentic-coding-workflows-in-2025) -- обзор альтернатив
- [AIMultiple: Agentic CLI](https://aimultiple.com/agentic-cli) -- аналитика рынка agentic CLI
- [DigitalOcean: Claude Code Alternatives](https://www.digitalocean.com/resources/articles/claude-code-alternatives) -- обзор альтернатив
- [OpenAI Codex Issue #5085 -- Cost Tracking](https://github.com/openai/codex/issues/5085) -- community request

### Self-healing и debugging
- [Self-Debugging Agent Architecture](https://medium.com/@atnoforaimldl/we-coded-an-ai-agent-that-can-debug-its-own-errors-heres-the-architecture-bdf5e72f87ce) -- Task Planner -> Coder -> Executor -> Debugger
- [Google CodeMender: AI Agent for Code Security](https://deepmind.google/blog/introducing-codemender-an-ai-agent-for-code-security/) -- self-correction на основе LLM judge
- [Dagger: Self-Healing CI Pipelines with AI Agents](https://dagger.io/blog/automate-your-ci-fixes-self-healing-pipelines-with-ai-agents) -- автоматическое исправление CI
- [EmergentMind: Self-Debugging Agent Topics](https://www.emergentmind.com/topics/self-debugging-agent) -- обзор подходов к self-debugging

### Модели и pricing
- [Anthropic Pricing](https://www.anthropic.com/pricing) -- цены Claude моделей
- [Claude Code Documentation](https://docs.anthropic.com/en/docs/claude-code) -- hooks, JSON output, agent teams
