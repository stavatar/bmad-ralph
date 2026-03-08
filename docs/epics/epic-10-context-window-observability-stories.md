# Epic 10: Context Window Observability — Stories

**Scope:** FR81-FR92, NFR30-NFR35
**Stories:** 7
**Release milestone:** v0.7
**PRD:** [docs/prd/context-window-observability.md](../prd/context-window-observability.md)
**Architecture:** [docs/architecture/context-window-observability.md](../architecture/context-window-observability.md)

**Context:**
Исследования 2024-2026 подтверждают: деградация качества LLM при заполнении контекстного окна — доказанный факт ("context rot"). При `max_turns: 50` все 11 реальных сессий mentorlearnplatform оказались в жёлтой/красной зоне (52-88% fill). Epic 10 добавляет два измерения к существующей подсистеме метрик: точный подсчёт compactions (через PreCompact hook) и приблизительный context fill % (через формулу из кумулятивных token данных). Также снижает дефолтный `max_turns` с 50 до 15, удерживая контекст в зелёной зоне (~30-35%).

**Dependency structure:**
```
10.1 Config + defaults ─────────────────────┐
10.2 SessionMetrics.ContextWindow parsing ──┬┤
10.3 context.go core functions (←10.2) ─────┼┼──→ 10.5 MetricsCollector extension ──→ 10.6 Runner integration ──→ 10.7 Warning + summary
10.4 EnsureCompactHook ────────────────────┘
```
10.3 зависит от 10.2 (использует `metrics.ContextWindow` field).

**Existing scaffold:**
- `runner/metrics.go` — MetricsCollector, RecordSession(metrics, model, stepType, durationMs) string, TaskMetrics, RunMetrics, taskAccumulator, FinishTask, Finish
- `runner/runner.go` — Runner.Execute(), execute(), RealReview(), RunConfig (15+ fields), 6 RecordSession call sites
- `runner/log.go` — RunLogger, SessionLogInfo (4 fields), SaveSessionLog(), SaveSession()
- `runner/serena.go` — RealSerenaSync(), 1 RecordSession call site
- `session/result.go` — SessionMetrics (6 fields), jsonResultMessage, usageData, resultFromMessage(), ParseResult()
- `session/session.go` — Options.Env map[string]string
- `config/config.go` — Config struct (25+ fields), Validate(), BudgetWarnPct pattern
- `config/defaults.yaml` — max_turns: 50

---

### Story 10.1: Config Fields + max_turns Default

**User Story:**
Как разработчик, я хочу иметь настраиваемые пороги context fill % и безопасный дефолт max_turns, чтобы Ralph по умолчанию работал в зелёной зоне контекста.

**Acceptance Criteria:**

```gherkin
Scenario: Config fields (FR91)
  Given config/config.go defines Config struct
  When new fields added:
    ContextWarnPct    int `yaml:"context_warn_pct"`
    ContextCriticalPct int `yaml:"context_critical_pct"`
  Then fields parse from ralph.yaml correctly
  And zero values before defaults applied

Scenario: Defaults (FR81, FR91)
  Given config/defaults.yaml
  When defaults loaded:
    max_turns: 15
    context_warn_pct: 55
    context_critical_pct: 65
  Then Config.MaxTurns == 15
  And Config.ContextWarnPct == 55
  And Config.ContextCriticalPct == 65

Scenario: Validation — range (FR91)
  Given Config with ContextWarnPct = 0
  When Validate() called
  Then error: "config: context_warn_pct must be 1-99, got 0"

  Given Config with ContextCriticalPct = 100
  When Validate() called
  Then error: "config: context_critical_pct must be 1-99, got 100"

Scenario: Validation — ordering (FR91)
  Given Config with ContextWarnPct = 65, ContextCriticalPct = 55
  When Validate() called
  Then error: "config: context_critical_pct (55) must be > context_warn_pct (65)"

  Given Config with ContextWarnPct = 55, ContextCriticalPct = 55
  When Validate() called
  Then error contains "must be >"

Scenario: Validation — happy path (FR91)
  Given Config with ContextWarnPct = 55, ContextCriticalPct = 65
  When Validate() called
  Then no error from these fields

Scenario: max_turns default change (FR81)
  Given config/defaults.yaml has max_turns: 15
  When Config loaded without user override
  Then Config.MaxTurns == 15
  And existing tests with explicit max_turns override are not affected
```

**Technical Notes:**
- Файлы: `config/config.go` (+2 fields, +validation), `config/defaults.yaml` (max_turns: 15, +2 fields)
- Паттерн валидации — аналогично `BudgetWarnPct`: проверка range + ordering
- `max_turns: 50 → 15` — breaking default, но resume компенсирует (задачи продолжаются в следующей итерации)
- Тесты: `config/config_test.go` — table-driven validation cases для range, ordering, happy path

**Prerequisites:** Нет

---

### Story 10.2: SessionMetrics.ContextWindow Parsing

**User Story:**
Как разработчик, я хочу парсить `contextWindow` из JSON result Claude Code, чтобы формула context fill использовала реальный размер контекстного окна модели.

**Acceptance Criteria:**

```gherkin
Scenario: modelUsageEntry struct (FR86)
  Given session/result.go
  When new unexported struct modelUsageEntry defined:
    InputTokens             int     `json:"inputTokens"`
    OutputTokens            int     `json:"outputTokens"`
    CacheReadInputTokens    int     `json:"cacheReadInputTokens"`
    CacheCreationInputTokens int    `json:"cacheCreationInputTokens"`
    ContextWindow           int     `json:"contextWindow"`
    MaxOutputTokens         int     `json:"maxOutputTokens"`
    CostUSD                 float64 `json:"costUSD"`
  Then struct correctly parses camelCase JSON fields from modelUsage

Scenario: jsonResultMessage extension (FR86)
  Given jsonResultMessage struct in session/result.go
  When new field added:
    ModelUsage map[string]modelUsageEntry `json:"modelUsage"`
  Then field parses modelUsage map from JSON result

Scenario: SessionMetrics ContextWindow field (FR86)
  Given SessionMetrics struct in session/result.go
  When new field added:
    ContextWindow int `json:"context_window"`
  Then field stores parsed context window size

Scenario: resultFromMessage extraction (FR86)
  Given jsonResultMessage with modelUsage:
    {"claude-sonnet-4-6": {"contextWindow": 200000, ...}}
  When resultFromMessage() called
  Then SessionMetrics.ContextWindow == 200000

Scenario: Multiple models in modelUsage (FR86)
  Given jsonResultMessage with modelUsage containing 2 models:
    {"claude-sonnet-4-6": {"contextWindow": 200000}, "claude-haiku-4-5": {"contextWindow": 200000}}
  When resultFromMessage() called
  Then SessionMetrics.ContextWindow == 200000 (from first entry)

Scenario: Missing modelUsage — backward compat (FR86)
  Given JSON result without modelUsage field (old Claude Code)
  When ParseResult() called
  Then SessionMetrics.ContextWindow == 0
  And all other fields parsed correctly (existing behavior preserved)

Scenario: Empty modelUsage (FR86)
  Given JSON result with modelUsage: {}
  When ParseResult() called
  Then SessionMetrics.ContextWindow == 0

Scenario: Existing golden files pass (backward compat)
  Given testdata/result_success.json, result_success_object.json, etc.
  When ParseResult() called on each
  Then all existing assertions pass
  And SessionMetrics.ContextWindow == 0 (no modelUsage in golden files)
```

**Technical Notes:**
- Файлы: `session/result.go` (+modelUsageEntry struct, +ModelUsage field in jsonResultMessage, +ContextWindow in SessionMetrics, extraction in resultFromMessage)
- Тесты: `session/result_test.go` — новые golden files с modelUsage, table-driven
- **camelCase** в modelUsage json tags (не snake_case как в usage) — верифицировано в исходниках Claude Code v2.1.56
- Итерация по `msg.ModelUsage` с `break` после первой записи — обычно одна модель

**Prerequisites:** Нет

---

### Story 10.3: Core Context Functions

**User Story:**
Как разработчик, я хочу иметь функции для создания compaction counter и расчёта context fill %, чтобы интегрировать их в runner pipeline.

**Acceptance Criteria:**

```gherkin
Scenario: CreateCompactCounter happy path (FR82)
  Given runner/context.go (new file)
  When CreateCompactCounter() called
  Then returns non-empty path to temp file in os.TempDir()
  And file name matches "ralph-compact-*" pattern
  And cleanup function removes the file when called

Scenario: CreateCompactCounter — error (NFR30)
  Given os.CreateTemp fails (e.g., invalid TempDir)
  When CreateCompactCounter() called
  Then returns ("", no-op cleanup)
  And no panic

Scenario: CreateCompactCounter — cleanup idempotent (FR82, NFR34)
  Given CreateCompactCounter() returned (path, cleanup)
  When cleanup() called twice
  Then no error (second call is no-op)

Scenario: CountCompactions — empty file (FR84)
  Given temp file exists but is empty (0 bytes)
  When CountCompactions(path) called
  Then returns 0

Scenario: CountCompactions — 1 compaction (FR84)
  Given temp file with content "1\n"
  When CountCompactions(path) called
  Then returns 1

Scenario: CountCompactions — 3 compactions (FR84)
  Given temp file with content "1\n1\n1\n"
  When CountCompactions(path) called
  Then returns 3

Scenario: CountCompactions — missing file (FR84, NFR30)
  Given path to non-existent file
  When CountCompactions(path) called
  Then returns 0 (graceful degradation)

Scenario: CountCompactions — empty path (FR84, NFR30)
  Given path = ""
  When CountCompactions("") called
  Then returns 0

Scenario: CountCompactions — corrupt file with blank lines (FR84)
  Given temp file with content "1\n\n1\n\n"
  When CountCompactions(path) called
  Then returns 2 (only non-empty lines counted)

Scenario: EstimateMaxContextFill — happy path (FR85)
  Given SessionMetrics: cache_read=1456521, cache_creation=57388, input=2700, numTurns=25
  And contextWindow = 200000
  When EstimateMaxContextFill(metrics, 200000) called
  Then returns approximately 60.7 (±0.1)

Scenario: EstimateMaxContextFill — uses metrics.ContextWindow (FR86)
  Given SessionMetrics: cache_read=1456521, cache_creation=57388, input=2700, numTurns=25, contextWindow=200000
  When EstimateMaxContextFill(metrics, 100000) called
  Then returns approximately 60.7 (uses metrics.ContextWindow=200000, not fallback=100000)

Scenario: EstimateMaxContextFill — fallback when ContextWindow=0 (FR86)
  Given SessionMetrics with ContextWindow=0
  When EstimateMaxContextFill(metrics, 200000) called
  Then uses fallbackContextWindow=200000 for calculation

Scenario: EstimateMaxContextFill — guard max(numTurns, 2) (FR85)
  Given SessionMetrics: cache_read=20000, cache_creation=5000, input=500, numTurns=1
  And contextWindow = 200000
  When EstimateMaxContextFill(metrics, 200000) called
  Then returns approximately 12.8 (uses effective_turns=2, not 1)

Scenario: EstimateMaxContextFill — zero turns (FR85)
  Given SessionMetrics with numTurns=0
  When EstimateMaxContextFill(metrics, 200000) called
  Then returns 0.0

Scenario: EstimateMaxContextFill — nil metrics (FR85)
  Given metrics = nil
  When EstimateMaxContextFill(nil, 200000) called
  Then returns 0.0

Scenario: EstimateMaxContextFill — zero context window both (FR85)
  Given SessionMetrics with ContextWindow=0
  When EstimateMaxContextFill(metrics, 0) called
  Then returns 0.0 (no division by zero)
```

**Technical Notes:**
- Файл: `runner/context.go` (новый) — CreateCompactCounter, CountCompactions, EstimateMaxContextFill
- Тесты: `runner/context_test.go` (новый) — table-driven для всех сценариев
- `CreateCompactCounter` использует `os.CreateTemp("", "ralph-compact-*")` — OS temp dir
- `EstimateMaxContextFill` приоритет: `metrics.ContextWindow > 0` → использовать, иначе `fallbackContextWindow`
- Формула: `2 × (cache_read + cache_creation + input) / max(numTurns, 2) / contextWindow × 100`

**Prerequisites:** 10.2 (для ContextWindow field в SessionMetrics)

---

### Story 10.4: EnsureCompactHook

**User Story:**
Как разработчик, я хочу чтобы Ralph автоматически устанавливал PreCompact hook при запуске, чтобы compaction events записывались в counter file.

**Acceptance Criteria:**

```gherkin
Scenario: Create hook script — fresh (FR83)
  Given projectRoot without .ralph/hooks/ directory
  When EnsureCompactHook(projectRoot) called
  Then .ralph/hooks/count-compact.sh created with correct content:
    #!/bin/bash
    [ -n "$RALPH_COMPACT_COUNTER" ] && echo 1 >> "$RALPH_COMPACT_COUNTER"
  And file is executable (chmod +x)

Scenario: Hook script exists — same content (FR83)
  Given .ralph/hooks/count-compact.sh already exists with correct content
  When EnsureCompactHook(projectRoot) called
  Then file not modified (no unnecessary write)

Scenario: Hook script exists — outdated content (FR83)
  Given .ralph/hooks/count-compact.sh exists with old/different content
  When EnsureCompactHook(projectRoot) called
  Then file overwritten with current version
  And file is executable

Scenario: Create settings.json — fresh (FR83, NFR35)
  Given projectRoot without .claude/settings.json
  When EnsureCompactHook(projectRoot) called
  Then .claude/settings.json created with:
    {"hooks":{"PreCompact":[{"matcher":"auto","hooks":[{"type":"command","command":".ralph/hooks/count-compact.sh"}]}]}}
  And file formatted with json.MarshalIndent ("  " indent)

Scenario: Additive merge — existing settings.json without PreCompact (FR83, NFR35)
  Given .claude/settings.json with other settings (e.g., {"permissions":{"allow":["Read"]}})
  When EnsureCompactHook(projectRoot) called
  Then PreCompact hook entry added to settings
  And existing "permissions" preserved unchanged
  And file formatted with json.MarshalIndent

Scenario: Additive merge — hooks key exists without PreCompact (FR83, NFR35)
  Given .claude/settings.json with {"hooks":{}}
  When EnsureCompactHook(projectRoot) called
  Then PreCompact array created inside hooks object
  And Ralph's hook entry added

Scenario: Additive merge — PreCompact exists as empty array (FR83, NFR35)
  Given .claude/settings.json with {"hooks":{"PreCompact":[]}}
  When EnsureCompactHook(projectRoot) called
  Then Ralph's hook entry appended to empty PreCompact array

Scenario: Additive merge — PreCompact exists with other hooks (FR83, NFR35)
  Given .claude/settings.json with PreCompact containing user's own hook
  When EnsureCompactHook(projectRoot) called
  Then Ralph's hook entry appended to PreCompact array
  And user's existing hook preserved unchanged

Scenario: Idempotent — hook already registered (FR83, NFR35)
  Given .claude/settings.json with PreCompact containing Ralph's count-compact.sh hook
  When EnsureCompactHook(projectRoot) called
  Then no changes to settings.json (idempotent)

Scenario: Backup before first modification (FR83)
  Given .claude/settings.json exists and .claude/settings.json.bak does NOT exist
  When EnsureCompactHook modifies settings.json
  Then .claude/settings.json.bak created with original content before modification

Scenario: Backup already exists (FR83)
  Given .claude/settings.json.bak already exists
  When EnsureCompactHook modifies settings.json
  Then .claude/settings.json.bak NOT overwritten (preserve original backup)

Scenario: Corrupt settings.json — graceful (NFR30)
  Given .claude/settings.json with invalid JSON content
  When EnsureCompactHook(projectRoot) called
  Then returns error (non-fatal, caller logs as warning)
  And settings.json not modified (don't corrupt further)

Scenario: Error return is non-fatal (NFR30)
  Given any error from EnsureCompactHook
  When caller (Runner.Execute) receives error
  Then logs warning, continues execution (compactions=0 fallback)
```

**Technical Notes:**
- Файл: `runner/context.go` — EnsureCompactHook function
- Тесты: `runner/context_test.go` — все сценарии, `t.TempDir()` для изоляции
- settings.json navigation: `map[string]any` → `hooks` → `PreCompact` → `[]any` → check for `count-compact.sh` in command
- Backup: `settings.json.bak` — только если бэкап не существует (одноразовый)
- `os.Chmod(path, 0755)` для hook script

**Prerequisites:** Нет

---

### Story 10.5: MetricsCollector Extension

**User Story:**
Как разработчик, я хочу чтобы MetricsCollector накапливал compactions и fill%, чтобы эти данные были доступны в TaskMetrics и RunMetrics.

**Acceptance Criteria:**

```gherkin
Scenario: taskAccumulator new fields (FR87)
  Given runner/metrics.go taskAccumulator struct
  When new fields added:
    totalCompactions  int
    maxContextFillPct float64
  Then fields initialized at 0/0.0 on StartTask

Scenario: RecordSession — new signature (FR87, FR88)
  Given RecordSession current signature:
    func (mc *MetricsCollector) RecordSession(metrics, model, stepType, durationMs) string
  When signature extended to:
    func (mc *MetricsCollector) RecordSession(metrics, model, stepType, durationMs, compactions int, contextFillPct float64) string
  Then return type still string (resolved model)
  And existing accumulation logic preserved

Scenario: RecordSession — accumulates compactions (FR87)
  Given task started, 3 sessions:
    session 1: compactions=0, fillPct=30.0
    session 2: compactions=1, fillPct=55.0
    session 3: compactions=0, fillPct=42.0
  When RecordSession called for each
  Then current.totalCompactions == 1 (sum)
  And current.maxContextFillPct == 55.0 (max)

Scenario: RecordSession — nil collector (FR87)
  Given mc.current == nil
  When RecordSession called with compactions=2, fillPct=50.0
  Then returns model (no panic, graceful)

Scenario: FinishTask — copies to TaskMetrics (FR87)
  Given taskAccumulator with totalCompactions=2, maxContextFillPct=65.3
  When FinishTask("done", "abc123") called
  Then TaskMetrics.TotalCompactions == 2
  And TaskMetrics.MaxContextFillPct == 65.3

Scenario: TaskMetrics new fields (FR87)
  Given TaskMetrics struct
  When new fields:
    TotalCompactions  int     `json:"total_compactions"`
    MaxContextFillPct float64 `json:"max_context_fill_pct"`
  Then fields serialize to JSON correctly

Scenario: RunMetrics new fields (FR88)
  Given RunMetrics struct
  When new fields:
    TotalCompactions  int     `json:"total_compactions"`
    MaxContextFillPct float64 `json:"max_context_fill_pct"`
  Then fields serialize to JSON correctly

Scenario: Finish — aggregates across tasks (FR88)
  Given 2 tasks:
    task 1: compactions=1, maxFillPct=55.0
    task 2: compactions=3, maxFillPct=42.0
  When Finish() called
  Then RunMetrics.TotalCompactions == 4 (sum)
  And RunMetrics.MaxContextFillPct == 55.0 (max)

Scenario: All existing RecordSession callers updated
  Given 6 call sites in runner.go + 1 in serena.go
  When compactions/fillPct not available at call site
  Then pass 0, 0.0 as default values
  And existing behavior preserved
```

**Technical Notes:**
- Файл: `runner/metrics.go` — taskAccumulator +2 fields, RecordSession +2 params, TaskMetrics +2 fields, RunMetrics +2 fields, FinishTask extension, Finish extension
- Тесты: `runner/metrics_test.go` — table-driven для accumulation, aggregation
- **7 call sites** обновляются: runner.go (lines ~525, ~1009, ~1048, ~1283) + serena.go
- Для call sites без counter: `0, 0.0` — backward compat

**Prerequisites:** Compile-independent (struct fields + signature change). Логическая зависимость: 10.1-10.4 создают данные, которые 10.5 накапливает; реальная интеграция в 10.6.

---

### Story 10.6: Runner Integration

**User Story:**
Как разработчик, я хочу чтобы runner создавал compaction counter, передавал env в сессии, считывал результат и записывал метрики, чтобы context observability работала end-to-end.

**Acceptance Criteria:**

```gherkin
Scenario: EnsureCompactHook at startup (FR83)
  Given Runner.Execute() starts
  When EnsureCompactHook(cfg.ProjectRoot) called before main loop
  Then hook setup runs once per ralph run
  And error logged as warning, execution continues

Scenario: DefaultContextWindow constant (FR86)
  Given runner/context.go
  When constant defined: DefaultContextWindow = 200000
  Then used as fallback in all EstimateMaxContextFill calls

Scenario: Execute path — counter lifecycle (FR82, FR84)
  Given execute iteration in Runner.Execute()
  When CreateCompactCounter() called before session.Execute
  Then counterPath set in opts.Env["RALPH_COMPACT_COUNTER"]
  And after session: CountCompactions(counterPath) returns count
  And EstimateMaxContextFill(sr.Metrics, DefaultContextWindow) returns fillPct
  And RecordSession receives compactions and fillPct
  And cleanup removes temp file

Scenario: Execute path — counter empty (no compaction) (FR82, FR84)
  Given session completes without triggering PreCompact hook
  When CountCompactions called on empty file
  Then compactions == 0
  And fillPct computed from session metrics

Scenario: Review path — counter via RunConfig.Env (FR82, FR84)
  Given RunConfig struct
  When new field: Env map[string]string
  Then review counter created in caller (Runner.Execute review section)
  And passed to RealReview via RunConfig.Env
  And RealReview copies Env to session.Options.Env
  And after review: CountCompactions returns count
  And RecordSession receives review compactions and fillPct

Scenario: RealReview — env passthrough (FR82)
  Given RunConfig.Env = {"RALPH_COMPACT_COUNTER": "/tmp/ralph-compact-xyz"}
  When RealReview builds session.Options
  Then opts.Env["RALPH_COMPACT_COUNTER"] == "/tmp/ralph-compact-xyz"

Scenario: Resume path — counter (FR82, FR84)
  Given ResumeExtraction flow
  When counter created before session.Execute
  Then env var set, compactions counted after session
  And RecordSession receives compactions and fillPct

Scenario: SerenaSync path — counter (FR82, FR84)
  Given RealSerenaSync flow
  When counter created before session.Execute
  Then env var set, compactions counted after session
  And RecordSession receives compactions and fillPct

Scenario: Counter not created — graceful (NFR30)
  Given CreateCompactCounter returns ("", no-op cleanup)
  When execute path runs
  Then opts.Env does NOT get RALPH_COMPACT_COUNTER
  And CountCompactions("") returns 0
  And RecordSession receives compactions=0
```

**Technical Notes:**
- Файлы: `runner/runner.go` (EnsureCompactHook call, counter lifecycle in execute/review paths, RunConfig.Env field), `runner/serena.go` (counter in sync path)
- `RunConfig.Env` — новое поле, RealReview копирует в `session.Options.Env`
- 4 paths: execute, review, resume, serena sync — каждый с counter lifecycle
- Тесты: integration tests с mock binary → verify RecordSession called with correct compactions/fillPct

**Prerequisites:** 10.3 (functions), 10.4 (hook), 10.5 (RecordSession signature)

---

### Story 10.7: Warning System + Summary + Session Log

**User Story:**
Как пользователь Ralph, я хочу видеть предупреждения о context fill и compactions в логе и summary, чтобы принимать обоснованные решения о настройке max_turns.

**Acceptance Criteria:**

```gherkin
Scenario: LogContextWarnings — silent when below warn threshold (FR89)
  Given fillPct=40.0, compactions=0, warnPct=55, criticalPct=65
  When LogContextWarnings called
  Then no warning logged (silent — data only in session log header)

Scenario: LogContextWarnings — WARN when above warn, below critical (FR89)
  Given fillPct=58.0, compactions=0, maxTurns=15, warnPct=55, criticalPct=65
  When LogContextWarnings called
  Then WARN logged: "context fill 58.0%% — consider reducing max_turns (current: 15) or splitting task into smaller pieces"

Scenario: LogContextWarnings — ERROR when above critical (FR89)
  Given fillPct=70.0, compactions=0, maxTurns=15, warnPct=55, criticalPct=65
  When LogContextWarnings called
  Then ERROR logged: "context fill 70.0%% exceeds critical threshold — quality degradation likely, reduce max_turns (current: 15)"

Scenario: LogContextWarnings — ERROR on compaction (FR89)
  Given fillPct=30.0, compactions=2, maxTurns=15
  When LogContextWarnings called
  Then ERROR logged: "2 compaction(s) detected — context was compressed, quality degraded. Reduce max_turns (current: 15)"

Scenario: LogContextWarnings — both fill and compaction (FR89)
  Given fillPct=70.0, compactions=1, maxTurns=15, warnPct=55, criticalPct=65
  When LogContextWarnings called
  Then TWO messages logged: fill ERROR + compaction ERROR

Scenario: Summary line — normal (FR90)
  Given RunMetrics: MaxContextFillPct=42.7, TotalCompactions=0
  When formatSummary generates Context line
  Then output contains "Context: max 42.7% fill, 0 compactions"

Scenario: Summary line — with compactions (FR90)
  Given RunMetrics: MaxContextFillPct=65.0, TotalCompactions=2
  When formatSummary generates Context line
  Then output contains "Context: max 65.0% fill, 2 compactions [!]"
  And line colored yellow (fatih/color)

Scenario: Summary line — critical fill (FR90)
  Given RunMetrics: MaxContextFillPct=72.0, TotalCompactions=0
  And criticalPct=65
  When formatSummary generates Context line
  Then output contains "Context: max 72.0% fill, 0 compactions [!]"
  And line colored red (fatih/color)

Scenario: Session log header — extended (FR92)
  Given SessionLogInfo with Compactions=1, MaxFillPct=58.3
  When SaveSessionLog writes header
  Then header: "=== SESSION execute seq=3 exit_code=0 elapsed=45.2s compactions=1 max_fill=58.3% ==="

Scenario: Session log header — zero values (FR92)
  Given SessionLogInfo with Compactions=0, MaxFillPct=0.0
  When SaveSessionLog writes header
  Then header contains "compactions=0 max_fill=0.0%"

Scenario: SessionLogInfo new fields (FR92)
  Given SessionLogInfo struct in runner/log.go
  When new fields:
    Compactions int
    MaxFillPct  float64
  Then fields included in header format string
```

**Technical Notes:**
- Файлы: `runner/context.go` (LogContextWarnings), `cmd/ralph/run.go` (formatSummary context line), `runner/log.go` (SessionLogInfo +2 fields, header format)
- Тесты: `runner/context_test.go` (LogContextWarnings), `cmd/ralph/run_test.go` (summary line), `runner/log_test.go` (header format)
- Warning тексты дословно из PRD FR89
- `fatih/color` — уже зависимость проекта, используется в summary
- `%%` — Go fmt escape для literal `%`

**Prerequisites:** 10.1 (config thresholds), 10.5 (RunMetrics fields), 10.6 (integration provides data)

---

## FR Coverage Matrix

| FR | Описание | Story |
|----|----------|-------|
| FR81 | max_turns default 50→15 | 10.1 |
| FR82 | CreateCompactCounter + temp file | 10.3, 10.6 |
| FR83 | EnsureCompactHook (script + settings.json) | 10.4, 10.6 |
| FR84 | Integration — env var, post-session read | 10.3, 10.6 |
| FR85 | EstimateMaxContextFill formula | 10.3 |
| FR86 | SessionMetrics.ContextWindow from modelUsage | 10.2 |
| FR87 | TaskMetrics.TotalCompactions, MaxContextFillPct | 10.5 |
| FR88 | RunMetrics.TotalCompactions, MaxContextFillPct | 10.5 |
| FR89 | LogContextWarnings | 10.7 |
| FR90 | Context line в formatSummary | 10.7 |
| FR91 | Config fields (ContextWarnPct, ContextCriticalPct) | 10.1 |
| FR92 | Session log header extension | 10.7 |

| NFR | Описание | Stories |
|-----|----------|---------|
| NFR30 | Best effort — ошибки не блокируют pipeline | 10.3, 10.4, 10.6 |
| NFR31 | Формула из существующих данных | 10.3 |
| NFR32 | Sensible defaults | 10.1 |
| NFR33 | max_turns не ломает workflow | 10.1 |
| NFR34 | Temp file в OS temp dir | 10.3 |
| NFR35 | Hook auto-generated, settings additive merge | 10.4 |

**Полное покрытие:** все 12 FR и 6 NFR mapped to stories.

---

## Summary

| Story | Scope | LOC est. | Files |
|-------|-------|----------|-------|
| 10.1 | Config + defaults | ~50 | config/config.go, config/defaults.yaml |
| 10.2 | ContextWindow parsing | ~60 | session/result.go |
| 10.3 | Core functions | ~100 | runner/context.go (new) |
| 10.4 | EnsureCompactHook | ~120 | runner/context.go |
| 10.5 | MetricsCollector | ~80 | runner/metrics.go |
| 10.6 | Runner integration | ~100 | runner/runner.go, runner/serena.go |
| 10.7 | Warning + summary + log | ~80 | runner/context.go, cmd/ralph/run.go, runner/log.go |
| **Total** | **7 stories** | **~590 LOC** | **8 files** |
