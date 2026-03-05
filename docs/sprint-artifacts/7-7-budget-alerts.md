# Story 7.7: Budget Alerts

Status: done

## Story

As a разработчик,
I want задать максимальный бюджет в долларах и получать предупреждения при приближении к лимиту, с экстренной остановкой при превышении,
so that не тратить ресурсы сверх запланированного.

## Acceptance Criteria

1. **AC1: Config fields (FR50)**
   - `BudgetMaxUSD float64` yaml:"budget_max_usd" (default 0 = unlimited)
   - `BudgetWarnPct int` yaml:"budget_warn_pct" (default 80)
   - Defaults в `defaults.yaml`: `budget_max_usd: 0`, `budget_warn_pct: 80`
   - Budget alerts отключены по умолчанию

2. **AC2: Budget warning at threshold (FR50)**
   - При `BudgetMaxUSD == 10.0` и `BudgetWarnPct == 80`:
     - Cumulative cost reaches $8.00 (80%):
       - `logger.Warn("budget warning", kv("cost", 8.00), kv("warn_at", 8.00), kv("budget", 10.00))`
       - InjectFeedback с budget warning hint (аналогично stuck detection в Story 7.5)
   - Warning логируется один раз per task (budgetWarned flag, reset per task)

3. **AC3: Emergency gate at budget exceeded (FR50)**
   - Cumulative cost reaches $10.00 (100%):
     - `logger.Error("budget exceeded", kv("cost", 10.00), kv("budget", 10.00))`
     - `EmergencyGatePromptFn` вызывается с budget exceeded message (emergency gate — approve NOT available)
     - User может retry (continue despite exceeded budget), skip (skip current task), или quit (abort run)
     - При отсутствии `EmergencyGatePromptFn` или `!GatesEnabled` — `return fmt.Errorf("runner: budget exceeded: ...")` (hard stop)

4. **AC4: Budget disabled (default)**
   - `BudgetMaxUSD == 0` → никаких budget warnings или emergency gates
   - Любое количество accumulated cost — без реакции

5. **AC5: Budget check timing**
   - Budget check выполняется после КАЖДОГО `RecordSession` (execute, review, distill)
   - Check происходит ПЕРЕД следующей фазой (early exit)

6. **AC6: Config validation**
   - `BudgetWarnPct` за пределами 1-99 (при BudgetMaxUSD > 0) → `"budget_warn_pct must be 1-99"`
   - `BudgetMaxUSD < 0` → validation error
   - Validation в `Config.Validate()`

## Tasks / Subtasks

- [x] Task 1: Add Config fields + validation (AC: #1, #6)
  - [x]1.1 Добавить `BudgetMaxUSD float64` и `BudgetWarnPct int` в Config struct (config/config.go, после field `RunID`)
  - [x]1.2 Defaults: `budget_max_usd: 0`, `budget_warn_pct: 80` в defaults.yaml
  - [x]1.3 Validate: BudgetMaxUSD < 0 → error; WarnPct не 1-99 при BudgetMaxUSD > 0 → error. Prefix: `"config: validate: "` (match existing Validate pattern)
  - [x]1.4 Тесты: default values, validation errors, valid overrides

- [x] Task 2: Budget check in execute loop (AC: #2, #3, #4, #5)
  - [x]2.1 Создать helper method `checkBudget(ctx, log, taskText) error` в Runner — вызывается после каждого RecordSession
  - [x]2.2 Guard: `if r.Cfg.BudgetMaxUSD <= 0 || r.Metrics == nil { return nil }` — budget disabled
  - [x]2.3 `cumCost := r.Metrics.CumulativeCost()` — ВАЖНО: CumulativeCost() после Story 7.3 включает in-progress task cost
  - [x]2.4 `warnAt := r.Cfg.BudgetMaxUSD * float64(r.Cfg.BudgetWarnPct) / 100`
  - [x]2.5 Если `cumCost >= r.Cfg.BudgetMaxUSD` → logger.Error + EmergencyGatePromptFn (если available) или hard error (если no gates). Emergency gate actions: quit → return error, skip → SkipTask + break, retry → continue execution despite exceeded budget (cost already incurred, no reset)
  - [x]2.6 Иначе если `cumCost >= warnAt && !budgetWarned` → logger.Warn + InjectFeedback с budget warning hint (аналог stuck detection). `budgetWarned` — bool field per-task, reset в StartTask или при новом task
  - [x]2.7 Вставить вызов checkBudget после КАЖДОГО RecordSession call site в runner.go. Найти по grep `r.Metrics.RecordSession(` — есть 3 call sites: execute session, review session, distill session. Каждый → budget check после
  - [x]2.8 ВАЖНО: emergency gates НЕ поддерживают `approve` — только retry/skip/quit (per Story 5.5 AC4, `gates.Gate{Emergency: true}` не показывает `[a]pprove`)
  - [x]2.9 Emergency gate pattern: скопировать существующий pattern из execute exhaustion handler (grep `"execute attempts exhausted"` в runner.go) — тот же формат: construct text, call EmergencyGatePromptFn, handle quit/skip/retry

- [x] Task 3: Тесты (AC: #1-#6)
  - [x]3.1 Тест: cost at 80% → warning logged (once per task) + InjectFeedback called
  - [x]3.2 Тест: cost at 100% → emergency gate triggered
  - [x]3.3 Тест: BudgetMaxUSD == 0 → no checks
  - [x]3.4 Тест: budget exceeded without gates → hard error return
  - [x]3.5 Тест: budget emergency gate actions: quit → error, skip → skip task, retry → continue
  - [x]3.6 Тест: validation errors for invalid config values (BudgetMaxUSD < 0, WarnPct out of range)
  - [x]3.7 Тест: nil Metrics → no budget checks (no panic)
  - [x]3.8 Тест: budget exceeded during review session → same emergency gate behavior
  - [x]3.9 Тест: budget warning NOT repeated for same task (budgetWarned flag)

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `config/config.go` | Extend: +BudgetMaxUSD, +BudgetWarnPct fields, Validate() | ~15 |
| `config/defaults.yaml` | Extend: +budget fields | ~2 |
| `config/config_test.go` | Add: validation tests | ~30 |
| `runner/runner.go` | Extend: checkBudget helper, budget check after all RecordSession sites | ~35 |
| `runner/runner_test.go` | Add: budget alert tests | ~70 |

### Architecture Compliance

- **Reuses EmergencyGatePromptFn** from Epic 5 — no new gate infrastructure
- **Config validation** in existing `Config.Validate()` method with `"config: validate: "` prefix
- **Budget check placement:** after every RecordSession, before next phase — fits existing loop structure
- **InjectFeedback for warnings** — matches stuck detection pattern (Story 7.5)

### Existing Code Context

- `EmergencyGatePromptFn GatePromptFunc` — injectable field в Runner, used by execute/review exhaustion. Emergency gate pattern: construct text → call fn → check quit/skip/retry. Find existing pattern via grep `EmergencyGatePromptFn(ctx` in runner.go
- Emergency gate actions: retry/skip/quit (НЕТ approve — per Story 5.5 AC4, `gates.Gate{Emergency: true}` не показывает `[a]pprove`)
- `CumulativeCost() float64` — MetricsCollector method. После Story 7.3 включает in-progress task cost (totalCost + current.costUSD)
- `Config.Validate()` — existing validation method, uses `"config: validate:"` prefix. Find via grep `func.*Validate` in config/config.go
- RecordSession call sites в runner.go — find via grep `r.Metrics.RecordSession(`. Есть 3 call sites: execute session, review session, distill session
- Existing emergency gate pattern — find via grep `"execute attempts exhausted"` or `EmergencyGatePromptFn(ctx` in runner.go
- InjectFeedback — `InjectFeedback(tasksFile, taskDesc, message)` — existing function, used by stuck detection. Find via grep `InjectFeedback(` in runner.go
- `SkipTask(tasksFile, taskDesc)` — existing function for skip action. Find via grep `SkipTask(` in runner.go

### Prerequisites

- Story 7.3 (CumulativeCost must return actual values including in-progress task, not just totalCost)
- Story 7.1 (MetricsCollector, RecordSession infrastructure)

### Key Design Decisions

1. **checkBudget helper method** — single method called after every RecordSession, avoids 3x copy-paste in execute/review/distill paths
2. **budgetWarned per task** — warning logged once per task to avoid noise; reset when new task starts
3. **InjectFeedback for warning** — AI agent sees the warning in next prompt (logger.Warn alone not visible to AI)
4. **Emergency gate retry = continue despite exceeded** — budget cost can't be undone; retry simply means "I accept the overspend"

### References

- [Source: docs/architecture/observability-metrics.md#Решение 7] — Budget Alerts
- [Source: docs/prd/observability-metrics.md#FR50] — budget alerts
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.7] — полное описание

## Dev Agent Record

### Context Reference
- Story 7.3 (CumulativeCost), Story 5.5 (emergency gate pattern), Story 7.5 (InjectFeedback pattern)

### Agent Model Used
- claude-opus-4-6

### Debug Log References
- All 10 runner budget tests pass, full suite green
- Review 7.7 fixes applied: M1 gate.count != 1 (Quit/Skip), M2 dollar amount assertions in warning test
- Review 7.7 R2 fixes: M1 %.2f validation format, M2 dollar assertions in hard error, M3 inner error in gate quit, M4 exact gate retry count
- Note: AC5 lists "distill" as RecordSession site but AutoDistill has no RecordSession (pre-existing gap, not 7.7 scope)

### Completion Notes List
- Config fields (BudgetMaxUSD, BudgetWarnPct) added with defaults and validation
- checkBudget helper method on Runner: budget disabled guard, exceeded (emergency gate or hard error), warning (once per task via InjectFeedback)
- errBudgetSkip sentinel for skip action propagation through nested loops
- Budget check wired after all 3 RecordSession call sites (2 execute + 1 review)
- budgetWarned per-task flag declared in execute loop, reset per task iteration
- 10 tests: disabled, warning, hard error, gate quit/retry/skip, gate error, nil metrics, review-exceeded, warning-not-repeated

### File List
- config/config.go — BudgetMaxUSD + BudgetWarnPct fields, Validate() rules
- config/defaults.yaml — budget_max_usd: 0, budget_warn_pct: 80
- config/config_test.go — DefaultsComplete, ValidFullConfig, 3 validation error cases
- runner/runner.go — errBudgetSkip sentinel, checkBudget helper, budget check at 3 RecordSession sites, budgetWarned per-task
- runner/runner_test.go — 10 budget alert integration tests
