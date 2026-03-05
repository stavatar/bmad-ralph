# Observability & Metrics — PRD Shard

**Epic:** 7 — Observability & Metrics
**Status:** Draft
**Date:** 2026-03-04
**Research:** [docs/research/observability-metrics.md](../research/observability-metrics.md)

---

## Контекст

Ralph логирует 14 типов событий через RunLogger (key=value текст), но не собирает метрики производительности, стоимости и качества. Конкурентный анализ (Aider, SWE-agent, Devin, Cline, Codex CLI) показывает: token/cost tracking — table stakes, structured export — минимальный порог. Review quality tracking — уникальное преимущество ralph, но без гранулярных данных (severity breakdown, findings по категориям) это преимущество не реализовано.

Текущие пробелы:
- Нет token/cost tracking (Claude Code возвращает usage data в JSON, ralph не парсит)
- ReviewResult = `{Clean bool}` — binary signal без severity breakdown
- Нет git diff stats (только SHA before/after)
- Нет structured JSON export — невозможна агрегация между запусками
- Нет run/task ID корреляции в логах
- Нет budget alerts и stuck detection
- Нет latency breakdown по фазам loop

---

## Scope

Полный P0-P2 из research-отчёта. Zero new external dependencies (сохраняется принцип "Only 3 direct deps"). P3 (OTel, Langfuse, TUI dashboard, trend CLI) — отдельный backlog.

---

## Функциональные требования

### Сбор метрик (Metrics Collection)

- **FR42:** Система собирает token usage (input_tokens, output_tokens, cache_read_input_tokens) из JSON output каждой Claude Code сессии (execute и review). Данные извлекаются из `session.ParseResult` — расширение существующего парсера, zero new dependencies. При отсутствии usage data в output (старые версии CLI) — graceful degradation с нулевыми значениями

- **FR43:** Система рассчитывает стоимость каждой сессии на основе token usage и model-aware pricing table. Pricing table — встроенные константы с возможностью override через config. Поддерживаемые модели: claude-sonnet-4-20250514, claude-opus-4-20250514 (расширяемый список). При неизвестной модели — warning + fallback на самую дорогую цену

- **FR44:** Система собирает git diff stats (files_changed, insertions, deletions, packages_touched) после каждого успешного commit. Расширение `GitClient` interface методом `DiffStats(ctx, beforeSHA, afterSHA)`. Логирование: `INFO commit stats files=3 insertions=47 deletions=12 packages=runner,config`

- **FR45:** Система агрегирует стоимость per-task (сумма всех execute + review сессий) и per-run (сумма всех задач). Текущая стоимость задачи отображается в human gate prompt (`Cost so far: $1.23`). Кумулятивная стоимость run — в run summary

- **FR46:** Система расширяет `ReviewResult` с `{Clean bool}` до структуры с findings по severity (CRITICAL/HIGH/MEDIUM/LOW) и category. Review output parsing извлекает severity и описание каждого finding. Метрики: `review.findings.total`, `review.findings.by_severity`, `review.fix_rate` (findings исправленных за один review cycle)

### Корреляция и идентификация (Trace Correlation)

- **FR47:** Система генерирует уникальный Run ID (UUID) при старте `ralph run` и уникальный Task ID при начале каждой задачи. Run ID и Task ID включаются в каждую запись RunLogger как structured keys (`run_id=abc123 task_id=story-5.1`). Session ID из Claude Code output добавляется при доступности

- **FR48:** Каждая запись RunLogger включает `step_type` key, идентифицирующий фазу loop: `execute`, `review`, `gate`, `git_check`, `retry`, `distill`, `resume`. Формат: `2026-03-04T10:15:30 INFO [runner] session complete run_id=abc123 task_id=story-5.1 step_type=execute duration_ms=45000`

### Обнаружение проблем (Problem Detection)

- **FR49:** Система обнаруживает "застревание" (stuck detection): если `headAfter == headBefore` (нет нового commit) в N consecutive execute attempts (configurable `stuck_threshold`, default 2), система inject'ит feedback в следующий execute prompt с указанием на отсутствие прогресса. Не заменяет MaxIterations — дополняет его ранним предупреждением

- **FR50:** Система предупреждает при приближении к cost budget: `budget_warn_pct` (default 80%) от `budget_max_usd` — WARN в лог + hint в prompt. При 100% — emergency human gate. `budget_max_usd` = 0 означает "без лимита" (default). Config fields: `budget_max_usd float64`, `budget_warn_pct int`

- **FR51:** Система обнаруживает повторяющиеся паттерны в последовательных diff'ах через similarity detection. Jaccard similarity между consecutive diffs (window size N=3, configurable). Warning threshold 0.85 → WARN + hint injection. Hard threshold 0.95 → emergency human gate. Config fields: `similarity_window int`, `similarity_warn float64`, `similarity_hard float64`

- **FR52:** Система классифицирует ошибки по категориям через parsing error wrapping prefixes (`"runner: scan tasks:"`, `"session: execute:"`). Категории: `transient` (rate limit, timeout, API error), `persistent` (bad config, missing file, impossible task), `unknown`. Классификация включается в RunMetrics для анализа паттернов recovery

### Gate Analytics

- **FR53:** Система логирует каждое gate decision с timing: `gate_action` (approve/retry/skip/quit), `gate_wait_ms` (wall-clock от prompt до ответа), `gate_task` (текущая задача). В run summary — distribution: `{approve: 12, retry: 3, skip: 1, quit: 0}` и средний `gate_wait_ms`

### Latency Breakdown

- **FR54:** Система инструментирует каждую фазу ralph loop для latency breakdown: `prompt_build_ms` (template assembly), `session_ms` (Claude Code execution), `git_check_ms` (HeadCommit + HealthCheck), `review_ms` (review session), `gate_wait_ms` (human input), `distill_ms` (distillation session), `backoff_ms` (sleep between retries). Метрики собираются per-task и агрегируются в run summary

### Run Summary Report

- **FR55:** По завершении `ralph run` система генерирует JSON summary report в `<logDir>/ralph-run-<runID>.json`. Report содержит:
  - Run metadata: run_id, start_time, end_time, total_duration_ms, ralph version
  - Per-task metrics: name, iterations, duration, commit_sha, diff_stats, review_cycles, review_findings_by_severity, gate_decision, retries, status (completed/skipped/reverted/failed), cost_usd, tokens (input/output/cache)
  - Run totals: tasks_completed, tasks_failed, tasks_skipped, total_tokens, total_cost_usd, total_duration_ms, avg_cost_per_task, avg_iterations_per_task
  - Gate distribution: {approve: N, retry: N, skip: N, quit: N}
  - Latency breakdown: per-phase totals and percentages
  - Error summary: by category (transient/persistent/unknown), count per category

- **FR56:** Система выводит краткую текстовую сводку в stdout по завершении run:
  ```
  Run complete: 8 tasks (7 completed, 1 skipped)
  Duration: 45m 23s | Cost: $3.47 | Tokens: 125K in / 42K out
  Reviews: 12 cycles, 23 findings (2H/15M/6L), 100% fix rate
  Report: .ralph/logs/ralph-run-abc123.json
  ```

---

## Нефункциональные требования

- **NFR21:** Overhead сбора метрик не превышает 100ms per iteration (за исключением `git diff --numstat` для крупных diff). Метрики собираются in-process, без сетевых вызовов

- **NFR22:** JSON summary report совместим с `jq` для post-processing. Schema стабильна: новые поля добавляются, существующие не удаляются и не переименовываются (additive-only evolution)

- **NFR23:** Все новые config fields имеют sensible defaults и не требуют настройки для базового использования. `ralph run` без изменений config получает полную метрику автоматически. Budget/similarity detection отключены по умолчанию (`budget_max_usd: 0`, `similarity_window: 0`)

- **NFR24:** При сбое сбора метрик (неожиданный JSON format, ошибка git diff) ralph продолжает выполнение с partial metrics. Метрики — best effort, не блокируют основной pipeline

- **NFR25:** Pricing table обновляема без пересборки — override через config field `model_pricing`. Встроенные значения актуальны на дату релиза

---

## User Stories (высокоуровневые)

### US1: Анализ стоимости run
**Как** разработчик, **я хочу** видеть стоимость каждой задачи и всего run, **чтобы** оптимизировать бюджет на AI-assisted development.
- **AC1:** После run stdout показывает total cost и tokens
- **AC2:** JSON report содержит per-task cost breakdown
- **AC3:** На gate prompt отображается текущая стоимость задачи
- **AC4:** При превышении budget — emergency gate

### US2: Диагностика review quality
**Как** разработчик, **я хочу** видеть breakdown findings по severity и тренд fix rate, **чтобы** понимать качество генерируемого кода и эффективность review pipeline.
- **AC1:** ReviewResult содержит findings с severity и category
- **AC2:** JSON report содержит review findings по severity per task
- **AC3:** Run summary показывает total findings, fix rate

### US3: Post-run analysis
**Как** разработчик, **я хочу** получать structured JSON report после каждого run, **чтобы** анализировать паттерны, сравнивать runs и находить bottlenecks.
- **AC1:** JSON report генерируется автоматически в logDir
- **AC2:** Report содержит all metrics (tokens, cost, diff stats, review findings, gate decisions, latency breakdown)
- **AC3:** Report parseable через `jq`

### US4: Раннее обнаружение проблем
**Как** разработчик, **я хочу** чтобы ralph обнаруживал зацикливание и чрезмерные затраты до исчерпания лимитов, **чтобы** не тратить ресурсы на безнадёжные задачи.
- **AC1:** Stuck detection при отсутствии commit N попыток подряд
- **AC2:** Budget warning при 80% и emergency gate при 100%
- **AC3:** Similarity detection при повторяющихся diff'ах

### US5: Корреляция событий
**Как** разработчик, **я хочу** чтобы каждая запись лога содержала run_id, task_id и step_type, **чтобы** при анализе логов я мог фильтровать по задаче и фазе.
- **AC1:** Каждый log entry содержит run_id и task_id
- **AC2:** step_type корректно идентифицирует фазу loop
- **AC3:** Session ID из Claude Code включается при доступности

---

## Зависимости

- **Внутренние:** Расширение `session.ParseResult` (FR42), `GitClient` interface (FR44), `ReviewResult` struct (FR46), `RunLogger` (FR47-FR48), `Config` struct (FR50-FR51), `gates.Prompt` (FR53)
- **Внешние:** Zero new dependencies. Используется только Go stdlib (`encoding/json`, `time`, `crypto/rand` для UUID)
- **Claude Code:** Требуется `--output-format json` с usage data (поддерживается в текущих версиях)

---

## Риски

| Риск | Вероятность | Impact | Mitigation |
|------|-------------|--------|------------|
| Claude Code меняет JSON format usage data | Средняя | High | Graceful degradation (FR42), golden file тесты на JSON contract |
| Pricing table устаревает | Высокая | Low | Override через config (NFR25), warning при unknown model (FR43) |
| Similarity detection false positives | Средняя | Medium | Conservative defaults (отключён), configurable thresholds (FR51) |
| Overhead git diff --numstat на крупных репо | Низкая | Low | Timeout + partial metrics (NFR24) |
| Run summary JSON schema evolution | Средняя | Medium | Additive-only policy (NFR22) |

---

## Приоритеты реализации

| Priority | Items | LOC est. | Dependencies |
|----------|-------|----------|--------------|
| P0 (foundation) | FR42, FR44, FR47, FR48 | ~500 | None (extends existing) |
| P1 (analytics) | FR43, FR45, FR46, FR49, FR53 | ~430 | P0 (token data, run_id) |
| P2 (advanced) | FR50, FR51, FR52, FR54, FR55, FR56 | ~580 | P0+P1 (cost data, review data) |
| **Total** | **15 FRs, 5 NFRs** | **~1510** | **Zero new deps** |
