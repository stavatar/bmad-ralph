# Epic 7: Observability & Metrics — Stories

**Scope:** FR42-FR56, NFR21-NFR25
**Stories:** 10
**Release milestone:** v0.4
**PRD:** [docs/prd/observability-metrics.md](../prd/observability-metrics.md)
**Architecture:** [docs/architecture/observability-metrics.md](../architecture/observability-metrics.md)

**Context:**
Ralph логирует 14 типов событий через RunLogger, но не собирает метрики производительности, стоимости и качества. Epic 7 добавляет полную observability: token/cost tracking, review enrichment, stuck/budget/similarity detection, structured JSON report. Zero new dependencies.

**Dependency structure:**
```
7.1 Metrics Foundation ──┬──→ 7.3 Cost Tracking ──→ 7.7 Budget Alerts
                         ├──→ 7.5 Stuck Detection
                         ├──→ 7.6 Gate Analytics
                         └──→ 7.9 Error Cat. + Latency
7.2 Git Diff Stats ──────────→ 7.8 Similarity Detection
7.4 Review Enrichment (independent)

7.10 Run Summary Report (depends on all above)
```

**Existing scaffold:**
- `runner/log.go` — RunLogger с Info/Warn/Error, kv/kvs helpers
- `runner/runner.go` — Runner struct с injectable fields, ReviewResult{Clean bool}
- `session/result.go` — ParseResult, SessionResult{SessionID, ExitCode, Output, Duration}, jsonResultMessage
- `runner/git.go` — GitClient interface (HealthCheck, HeadCommit, RestoreClean), ExecGitClient
- `config/config.go` — Config struct (18 fields), CLIFlags, defaults cascade
- `gates/gates.go` — Gate struct, Prompt function
- `internal/testutil/mock_git.go` — MockGitClient

---

### Story 7.1: Metrics Foundation — Token Parsing, MetricsCollector, Structured Log Keys

**User Story:**
Как разработчик, я хочу чтобы ralph собирал token usage из Claude Code JSON output, генерировал уникальные Run/Task ID и включал structured keys в каждую запись лога, чтобы метрики были доступны для агрегации и корреляции.

**Acceptance Criteria:**

```gherkin
Scenario: SessionMetrics extraction from Claude Code JSON (FR42)
  Given Claude Code returns JSON with usage data in stdout
  When ParseResult parses the JSON
  Then SessionResult.Metrics contains InputTokens, OutputTokens, CacheReadTokens, NumTurns
  And SessionMetrics struct defined in session/result.go with json tags
  And jsonResultMessage extended with Usage *usageData and NumTurns fields

Scenario: Graceful degradation when usage data absent (FR42)
  Given Claude Code returns JSON without usage fields (old CLI version)
  When ParseResult parses the JSON
  Then SessionResult.Metrics == nil (not error)
  And all existing ParseResult tests pass without modification

Scenario: MetricsCollector struct (Architecture Decision 2)
  Given runner/metrics.go defines MetricsCollector struct
  When NewMetricsCollector(runID, pricing) called
  Then collector initialized with empty tasks slice and zero counters
  And MetricsCollector has methods: StartTask, RecordSession, FinishTask, Finish
  And Finish() returns RunMetrics with all accumulated data

Scenario: MetricsCollector nil safety
  Given Runner.Metrics == nil (no collector configured)
  When execute loop runs normally
  Then no panic — all r.Metrics calls guarded by nil check
  And all existing Runner tests pass without MetricsCollector

Scenario: Run ID generation (FR47)
  Given cmd/ralph/run.go starts ralph run
  When MetricsCollector created
  Then runID is UUID v4 (crypto/rand, 36-char format)
  And runID passed to RunLogger and MetricsCollector

Scenario: RunLogger structured keys (FR47, FR48)
  Given RunLogger has runID field set at creation
  When any log method (Info/Warn/Error) called
  Then output includes run_id=<uuid> as first kv pair
  And caller provides task_id and step_type via kv() helper
  And step_type is one of: execute, review, gate, git_check, retry, distill, resume
  And format: "2026-03-04T10:15:30 INFO [runner] msg run_id=abc task_id=story-5.1 step_type=execute key=val"

Scenario: Runner.Execute returns RunMetrics (Architecture Decision 12)
  Given Runner.Execute(ctx) currently returns error
  When signature changed to (*RunMetrics, error)
  Then *RunMetrics contains all accumulated metrics (nil on early error)
  And cmd/ralph/run.go updated to receive RunMetrics
```

**Technical Notes:**
- `session/result.go`: extend `jsonResultMessage` with `Usage *usageData`, `NumTurns int`. New `usageData` struct. `SessionMetrics` struct with json tags. ParseResult populates `SessionResult.Metrics` when usage present.
- `runner/metrics.go` (NEW): MetricsCollector, TaskMetrics, RunMetrics, DiffStats, LatencyBreakdown, GateStats, ErrorStats structs. All with json tags for FR55.
- `runner/log.go`: add `runID string` field to RunLogger. `OpenRunLogger` gains `runID` param. `write()` prepends `run_id=`.
- `runner/runner.go`: Runner struct gains `Metrics *MetricsCollector`. Execute signature: `(*RunMetrics, error)`. Instrument execute loop with RecordSession calls.
- `cmd/ralph/run.go`: generate UUID, create MetricsCollector, pass to Runner, receive RunMetrics.
- UUID: `crypto/rand` + `fmt.Sprintf("%x-%x-%x-%x-%x", ...)` — no external deps.
- CostUSD NOT calculated here — deferred to Story 7.3 (pricing table needed).

**Prerequisites:** None (foundation story)

---

### Story 7.2: Git Diff Stats

**User Story:**
Как разработчик, я хочу видеть git diff statistics (files changed, insertions, deletions, packages) после каждого commit, чтобы оценивать объём изменений per task.

**Acceptance Criteria:**

```gherkin
Scenario: GitClient interface extension (FR44)
  Given GitClient interface has HealthCheck, HeadCommit, RestoreClean
  When DiffStats(ctx, before, after string) (*DiffStats, error) added
  Then interface has 4 methods
  And DiffStats struct defined: FilesChanged int, Insertions int, Deletions int, Packages []string

Scenario: ExecGitClient.DiffStats implementation
  Given two valid commit SHAs (before, after)
  When DiffStats called
  Then executes "git diff --numstat <before> <after>"
  And parses tab-separated output: insertions<TAB>deletions<TAB>filename
  And FilesChanged = number of lines in output
  And Insertions/Deletions = sum of respective columns
  And Packages = unique parent directories from filenames (sorted)
  And binary files ("-" in numstat) counted as FilesChanged but 0 insertions/deletions

Scenario: DiffStats with identical SHAs
  Given before == after
  When DiffStats called
  Then returns &DiffStats{} with all zeros and empty Packages

Scenario: DiffStats error handling
  Given invalid SHA or git error
  When DiffStats called
  Then returns wrapped error: "runner: diff stats: %w"

Scenario: MockGitClient extension
  Given MockGitClient in internal/testutil/mock_git.go
  When DiffStatsResult and DiffStatsError fields added
  Then DiffStats method returns configured values
  And DiffStatsCount int tracks call count

Scenario: Logging after commit (FR44)
  Given execute loop detects new commit (headAfter != headBefore)
  When DiffStats(ctx, headBefore, headAfter) succeeds
  Then logger.Info("commit stats", kv("files", N), kv("insertions", N), kv("deletions", N), kv("packages", "runner,config"))
  And if DiffStats fails — logger.Warn with error, execution continues (NFR24)
```

**Technical Notes:**
- `runner/git.go`: add DiffStats to GitClient interface, implement in ExecGitClient. Parse `git diff --numstat` output. Extract packages via `filepath.Dir()` on filenames, deduplicate.
- `internal/testutil/mock_git.go`: extend MockGitClient with DiffStats fields.
- `runner/runner.go`: after commit detection in execute(), call git.DiffStats, log results. Pass to MetricsCollector.RecordGitDiff if available (nil guard).
- Real git tests: use `runGit` helper pattern from `git_test.go`.

**Prerequisites:** None (independent of 7.1)

---

### Story 7.3: Cost Tracking — Pricing Table + Per-Task Aggregation

**User Story:**
Как разработчик, я хочу видеть стоимость каждой задачи и кумулятивную стоимость run в реальном времени (на gate prompt), чтобы контролировать бюджет.

**Acceptance Criteria:**

```gherkin
Scenario: Pricing struct and defaults (FR43)
  Given config/pricing.go (NEW) defines Pricing struct
  Then Pricing has InputPer1M, OutputPer1M, CachePer1M float64 with yaml+json tags
  And DefaultPricing map[string]Pricing contains at least:
    - "claude-sonnet-4-20250514"
    - "claude-opus-4-20250514"
  And prices match Anthropic public pricing at time of implementation

Scenario: Config pricing override (FR43)
  Given Config struct gains ModelPricing map[string]Pricing yaml:"model_pricing"
  When config.yaml contains model_pricing with custom prices
  Then custom prices override DefaultPricing per-model
  And models not in override use DefaultPricing

Scenario: Unknown model warning (FR43)
  Given session uses a model not in DefaultPricing or ModelPricing
  When cost calculation attempted
  Then logger.Warn("unknown model pricing", kv("model", name))
  And fallback to most expensive model in DefaultPricing (Opus)

Scenario: SessionMetrics.CostUSD calculation (FR43)
  Given SessionMetrics has token counts from Story 7.1
  When MetricsCollector.RecordSession called with model name
  Then CostUSD = (InputTokens * InputPer1M + OutputTokens * OutputPer1M + CacheReadTokens * CachePer1M) / 1_000_000
  And CostUSD accumulated in taskAccumulator

Scenario: Per-task cost aggregation (FR45)
  Given task has multiple sessions (execute + review + retry)
  When each session recorded via RecordSession
  Then TaskMetrics.CostUSD = sum of all session costs for this task
  And TaskMetrics.TokensInput/Output/Cache = sum of all session tokens

Scenario: Cumulative cost on gate prompt (FR45)
  Given gates enabled and task reaches gate checkpoint
  When gate prompt displayed
  Then includes "Cost so far: $X.XX" (cumulative run cost)
  And Gate struct or GatePromptFunc signature extended to receive cost info
  And formatting: 2 decimal places, USD symbol

Scenario: Cost precision
  Given float64 arithmetic for cost
  When many sessions accumulated
  Then no precision loss visible at 2 decimal places
  And test verifies: 1000 sessions * $0.001 = $1.00 (not $0.999...)
```

**Technical Notes:**
- `config/pricing.go` (NEW): Pricing struct, DefaultPricing var, `MergePricing(defaults, overrides)` function.
- `config/config.go`: add `ModelPricing map[string]Pricing` field.
- `runner/metrics.go`: extend MetricsCollector with pricing map. RecordSession calculates cost per session. `CumulativeCost() float64` method.
- `runner/runner.go`: pass model name to RecordSession. Before gate prompt, get CumulativeCost() for display.
- Gate display: extend gate prompt text with cost line. Minimal change — append string to taskText or add field to Gate struct.

**Prerequisites:** Story 7.1 (MetricsCollector, SessionMetrics)

---

### Story 7.4: Review Enrichment — Severity Findings

**User Story:**
Как разработчик, я хочу видеть breakdown review findings по severity (CRITICAL/HIGH/MEDIUM/LOW), чтобы понимать качество генерируемого кода и отслеживать тренды.

**Acceptance Criteria:**

```gherkin
Scenario: ReviewResult extension (FR46)
  Given ReviewResult currently has only Clean bool
  When extended with Findings []ReviewFinding
  Then ReviewFinding struct has Severity string and Text string
  And Clean==true implies Findings==nil or len(Findings)==0
  And all existing code checking result.Clean continues to work

Scenario: DetermineReviewOutcome parses findings (FR46)
  Given review-findings.md contains formatted findings:
    "### [HIGH] Missing error assertion\nDescription...\n\n### [MEDIUM] Doc comment\nDescription..."
  When DetermineReviewOutcome called with findingsNonEmpty==true
  Then Findings populated with parsed entries
  And severity extracted via regex: (?m)^###\s*\[(\w+)\]\s*(.+)$
  And Findings[0].Severity == "HIGH", Findings[0].Text == "Missing error assertion"
  And Findings[1].Severity == "MEDIUM", Findings[1].Text == "Doc comment"

Scenario: No findings parsed when clean
  Given review-findings.md is empty or absent
  When DetermineReviewOutcome returns Clean==true
  Then Findings == nil

Scenario: Malformed findings graceful handling
  Given review-findings.md has content but no ### [SEVERITY] headers
  When DetermineReviewOutcome parses
  Then Clean == false (file non-empty, task not marked done)
  And Findings == nil (no parseable findings — not an error)

Scenario: Findings logged with severity counts
  Given review session completed with findings
  When runner logs review result
  Then logger.Info("review findings", kv("total", N), kv("critical", N), kv("high", N), kv("medium", N), kv("low", N))

Scenario: Findings recorded in MetricsCollector
  Given MetricsCollector.RecordReview(result, durationMs) called
  When result has Findings
  Then TaskMetrics.ReviewFindings populated with findings list
  And severity counts aggregated for run totals
```

**Technical Notes:**
- `runner/runner.go`: extend ReviewResult with `Findings []ReviewFinding`. ReviewFinding struct: `{Severity, Text string}`.
- `runner/runner.go` DetermineReviewOutcome: after detecting `findingsNonEmpty`, parse content with `regexp.MustCompile("(?m)^###\\s*\\[(\\w+)\\]\\s*(.+)$")`. Package-scope compiled regex.
- Backward compatible: all `if result.Clean` checks unchanged. Findings is additive enrichment.
- Test fixtures: add `testdata/review-findings-with-severity.md` with mixed severity entries.

**Prerequisites:** None (independent — ReviewResult in runner, no dependency on MetricsCollector)

---

### Story 7.5: Stuck Detection

**User Story:**
Как разработчик, я хочу чтобы ralph обнаруживал застревание (нет commit за N попыток подряд) и подкидывал hint в prompt, чтобы AI попробовал другой подход раньше, чем исчерпаются лимиты.

**Acceptance Criteria:**

```gherkin
Scenario: Config field (FR49)
  Given Config struct extended with StuckThreshold int yaml:"stuck_threshold"
  When defaults.yaml has stuck_threshold: 2
  Then default StuckThreshold == 2
  And StuckThreshold == 0 disables stuck detection

Scenario: Stuck detection triggers feedback injection (FR49)
  Given StuckThreshold == 2
  And task has had 2 consecutive execute attempts with headAfter == headBefore
  When third execute attempt begins
  Then InjectFeedback called with message containing "no commit" and attempt count
  And logger.Warn("stuck detected", kv("task", taskText), kv("no_commit_count", 2))
  And MetricsCollector records stuck event if available

Scenario: Counter resets on successful commit
  Given consecutiveNoCommit == 2 (stuck threshold reached)
  When next execute produces a commit (headAfter != headBefore)
  Then consecutiveNoCommit resets to 0
  And no further stuck feedback injected

Scenario: Stuck detection disabled
  Given StuckThreshold == 0
  When headAfter == headBefore multiple times
  Then no stuck feedback injected
  And no stuck warning logged

Scenario: Stuck detection does NOT replace MaxIterations
  Given StuckThreshold == 2 and MaxIterations == 3
  When stuck detected at attempt 2
  Then feedback injected but loop continues
  And MaxIterations still enforced at attempt 3 (emergency gate or stop)

Scenario: Feedback message content
  Given stuck detected at consecutiveNoCommit == N
  When InjectFeedback called
  Then message includes: "No commit in last N attempts. Consider a different approach."
  And message format matches existing FeedbackPrefix pattern
```

**Technical Notes:**
- `config/config.go`: add StuckThreshold field. `defaults.yaml`: `stuck_threshold: 2`.
- `runner/runner.go` execute(): add `consecutiveNoCommit int` local counter. After headBefore/headAfter comparison: increment if same, reset if different. Check against cfg.StuckThreshold.
- Uses existing `InjectFeedback(tasksFile, message)` — no new function needed.

**Prerequisites:** Story 7.1 (MetricsCollector for recording, but stuck detection works without it)

---

### Story 7.6: Gate Analytics

**User Story:**
Как разработчик, я хочу чтобы ralph логировал каждое gate decision с timing (время от prompt до ответа), чтобы анализировать паттерны принятия решений.

**Acceptance Criteria:**

```gherkin
Scenario: Gate timing measurement (FR53)
  Given gate prompt displayed to user
  When user responds with decision (approve/retry/skip/quit)
  Then wall-clock time from prompt to response measured in milliseconds
  And logger.Info("gate decision", kv("action", action), kv("wait_ms", N), kv("task", taskText))

Scenario: Gate decision recorded in MetricsCollector (FR53)
  Given MetricsCollector.RecordGate(action string, waitMs int64) called
  When gate decision recorded
  Then GateStats counters incremented (approve/retry/skip/quit)
  And waitMs accumulated for average calculation
  And TaskMetrics.GateDecision and GateWaitMs populated

Scenario: Emergency gate tracked separately
  Given emergency gate triggered (execute exhaustion or review exhaustion)
  When decision recorded
  Then same recording as normal gate
  And step_type in log == "gate" (same for both normal and emergency)

Scenario: Gate timing in Runner
  Given Runner calls GatePromptFn or EmergencyGatePromptFn
  When measuring time
  Then t0 = time.Now() before call, elapsed = time.Since(t0) after
  And elapsed.Milliseconds() passed to RecordGate
```

**Technical Notes:**
- `runner/runner.go`: wrap GatePromptFn/EmergencyGatePromptFn calls with `time.Now()`/`time.Since()`. After gate returns, call `r.Metrics.RecordGate(decision.Action, elapsed.Milliseconds())`.
- `runner/metrics.go`: RecordGate increments GateStats counters, accumulates wait time. AvgWaitMs computed in Finish().
- No changes to `gates/` package — timing measured in runner (caller), not gates (provider).

**Prerequisites:** Story 7.1 (MetricsCollector)

---

### Story 7.7: Budget Alerts

**User Story:**
Как разработчик, я хочу задать максимальный бюджет в долларах и получать предупреждения при приближении к лимиту, с экстренной остановкой при превышении.

**Acceptance Criteria:**

```gherkin
Scenario: Config fields (FR50)
  Given Config struct extended with:
    BudgetMaxUSD float64 yaml:"budget_max_usd"     (default 0 = unlimited)
    BudgetWarnPct int yaml:"budget_warn_pct"         (default 80)
  When defaults.yaml has budget_max_usd: 0, budget_warn_pct: 80
  Then budget alerts disabled by default

Scenario: Budget warning at threshold (FR50)
  Given BudgetMaxUSD == 10.0 and BudgetWarnPct == 80
  And cumulative cost reaches $8.00 (80%)
  When next session recorded
  Then logger.Warn("budget warning", kv("cost", 8.00), kv("warn_at", 8.00), kv("budget", 10.00))
  And hint injected in next prompt or log visible to user

Scenario: Emergency gate at budget exceeded (FR50)
  Given BudgetMaxUSD == 10.0
  And cumulative cost reaches $10.00 (100%)
  When next session recorded
  Then logger.Error("budget exceeded", kv("cost", 10.00), kv("budget", 10.00))
  And EmergencyGatePromptFn called with budget exceeded message
  And user can approve (continue), skip, or quit

Scenario: Budget disabled (default)
  Given BudgetMaxUSD == 0
  When any amount of cost accumulated
  Then no budget warnings or emergency gates triggered

Scenario: Budget check timing
  Given budget check runs after each RecordSession
  When session completes
  Then budget evaluated against CumulativeCost()
  And check happens BEFORE next execute starts (early exit)

Scenario: Config validation
  Given BudgetWarnPct set to 150 (invalid)
  When Config.Validate called
  Then error: "budget_warn_pct must be 1-99"
  And BudgetMaxUSD < 0 also validation error
```

**Technical Notes:**
- `config/config.go`: add BudgetMaxUSD, BudgetWarnPct fields. Validate: 0 < WarnPct < 100 when BudgetMaxUSD > 0.
- `runner/runner.go`: after each RecordSession, if BudgetMaxUSD > 0: check CumulativeCost() against thresholds. Warn at WarnPct%, emergency gate at 100%.
- Emergency gate reuses existing EmergencyGatePromptFn infrastructure from Epic 5.

**Prerequisites:** Story 7.3 (cost tracking needed for CumulativeCost)

---

### Story 7.8: Similarity Detection

**User Story:**
Как разработчик, я хочу чтобы ralph обнаруживал повторяющиеся diff-паттерны (зацикливание), чтобы не тратить ресурсы на бесплодные попытки.

**Acceptance Criteria:**

```gherkin
Scenario: SimilarityDetector struct (FR51)
  Given runner/similarity.go (NEW) defines SimilarityDetector
  When NewSimilarityDetector(window, warnAt, hardAt) called
  Then detector initialized with empty history slice
  And window, warnAt, hardAt stored

Scenario: JaccardSimilarity function (FR51)
  Given two slices of strings (diff lines)
  When JaccardSimilarity(a, b) called
  Then returns |intersection| / |union| as float64
  And empty slices → 0.0 (not NaN or panic)
  And identical slices → 1.0

Scenario: Push and Check cycle (FR51)
  Given SimilarityDetector with window=3, warnAt=0.85, hardAt=0.95
  When Push(diffLines) called 3 times with similar diffs
  And Check() returns ("warn", 0.87) when avg similarity > warnAt
  And Check() returns ("hard", 0.96) when avg similarity > hardAt
  And Check() returns ("", 0.5) when diffs are diverse

Scenario: Warning threshold action (FR51)
  Given similarity Check returns "warn"
  When runner processes result
  Then logger.Warn("similarity warning", kv("score", 0.87), kv("threshold", 0.85))
  And hint injected: "Recent changes are very similar. Try a fundamentally different approach."

Scenario: Hard threshold action (FR51)
  Given similarity Check returns "hard"
  When runner processes result
  Then logger.Error("similarity loop detected", kv("score", 0.96), kv("threshold", 0.95))
  And EmergencyGatePromptFn called with loop detection message

Scenario: Similarity disabled by default (NFR23)
  Given Config SimilarityWindow == 0 (default)
  When runner starts
  Then no SimilarityDetector created
  And no similarity checks performed

Scenario: Config fields (FR51)
  Given Config extended with:
    SimilarityWindow int yaml:"similarity_window"     (default 0 = disabled)
    SimilarityWarn float64 yaml:"similarity_warn"     (default 0.85)
    SimilarityHard float64 yaml:"similarity_hard"     (default 0.95)
  When SimilarityWindow > 0
  Then detector created and used after each commit

Scenario: Diff lines source
  Given commit detected (headAfter != headBefore)
  When similarity tracking active
  Then diff lines obtained via "git diff <before> <after>" (full diff, not numstat)
  And GitClient extended with DiffLines(ctx, before, after) ([]string, error) method
  Or alternatively: use DiffStats output + file list as similarity signal
```

**Technical Notes:**
- `runner/similarity.go` (NEW): JaccardSimilarity function, SimilarityDetector struct with Push/Check methods. Sliding window of recent diff line sets.
- `config/config.go`: add SimilarityWindow, SimilarityWarn, SimilarityHard fields.
- `runner/runner.go`: if cfg.SimilarityWindow > 0, create detector. After each commit, push diff lines, check thresholds.
- Diff lines: simplest approach — reuse `git diff --numstat` file list as similarity signal (filenames, not content). Cheaper than full diff. Or add `DiffLines` to GitClient.

**Prerequisites:** Story 7.2 (GitClient DiffStats infrastructure)

---

### Story 7.9: Error Categorization + Latency Breakdown

**User Story:**
Как разработчик, я хочу видеть классификацию ошибок (transient/persistent/unknown) и разбивку времени по фазам loop, чтобы диагностировать проблемы и находить bottlenecks.

**Acceptance Criteria:**

```gherkin
Scenario: CategorizeError function (FR52)
  Given runner/metrics.go defines CategorizeError(err error) string
  When called with rate limit / timeout / API error → returns "transient"
  And called with config / not found / permission error → returns "persistent"
  And called with other errors → returns "unknown"
  And pattern matching via strings.Contains on err.Error()

Scenario: Error categorization recorded (FR52)
  Given error occurs during execute/review/gate
  When MetricsCollector.RecordError(CategorizeError(err)) called
  Then ErrorStats counter incremented for appropriate category
  And error category included in TaskMetrics

Scenario: Latency measurement points (FR54)
  Given execute loop has distinct phases
  When each phase timed with time.Now()/time.Since()
  Then following phases measured:
    - prompt_build (buildTemplateData)
    - session (session.Execute for execute)
    - git_check (HeadCommit + optional HealthCheck)
    - review (session.Execute for review)
    - gate_wait (GatePromptFn / EmergencyGatePromptFn)
    - distill (distillation session)
    - backoff (SleepFn between retries)

Scenario: Latency recorded per-task (FR54)
  Given MetricsCollector.RecordLatency(phase string, ms int64) called
  When task finishes
  Then TaskMetrics.LatencyBreakdown populated with per-phase totals
  And each phase is sum of all occurrences (e.g., 3 retries = 3x backoff)

Scenario: Latency not blocking
  Given time.Now() call overhead ~50ns
  When 7 measurement points per iteration
  Then total overhead < 1ms (within NFR21 100ms budget)
```

**Technical Notes:**
- `runner/metrics.go`: CategorizeError function with string pattern matching. RecordError method on MetricsCollector. RecordLatency accumulates per phase in LatencyBreakdown.
- `runner/runner.go`: wrap each phase with `t := time.Now()` / `r.Metrics.RecordLatency("phase", time.Since(t).Milliseconds())`. 7 measurement points in execute loop.
- Simple implementation — no timer framework, just time.Now pairs.

**Prerequisites:** Story 7.1 (MetricsCollector)

---

### Story 7.10: Run Summary Report + Stdout Summary

**User Story:**
Как разработчик, я хочу получать structured JSON report и краткую текстовую сводку после каждого run, чтобы анализировать паттерны и сравнивать runs.

**Acceptance Criteria:**

```gherkin
Scenario: JSON report generation (FR55)
  Given Runner.Execute returns *RunMetrics
  When cmd/ralph/run.go receives non-nil RunMetrics
  Then json.MarshalIndent(metrics, "", "  ") written to <logDir>/ralph-run-<runID>.json
  And file contains all RunMetrics fields:
    - run_id, start_time, end_time, total_duration_ms
    - tasks[] with per-task: name, iterations, duration_ms, commit_sha, diff_stats,
      review_cycles, review_findings, gate_decision, gate_wait_ms, retries, status,
      tokens_input, tokens_output, tokens_cache, cost_usd, latency
    - total_tokens_input, total_tokens_output, total_cost_usd
    - tasks_completed, tasks_failed, tasks_skipped
    - gates: {approve, retry, skip, quit, avg_wait_ms}
    - errors: {transient, persistent, unknown}

Scenario: JSON report jq-compatible (NFR22)
  Given report written as valid JSON
  When processed with jq
  Then jq '.tasks[0].cost_usd' returns valid number
  And jq '.total_cost_usd' returns valid number
  And all fields accessible via standard jq selectors

Scenario: Stdout text summary (FR56)
  Given run completes
  When cmd/ralph/run.go prints summary
  Then output format:
    "Run complete: N tasks (X completed, Y skipped, Z failed)"
    "Duration: Xm Ys | Cost: $X.XX | Tokens: XK in / YK out"
    "Reviews: N cycles, M findings (Xh/Ym/Zl), R% fix rate"
    "Report: .ralph/logs/ralph-run-<id>.json"
  And uses fatih/color for formatting (green for success, yellow for warnings)
  And token counts formatted with K suffix (e.g., "125K")

Scenario: Summary with zero metrics
  Given run completes but no sessions had usage data (old CLI)
  When summary printed
  Then "Cost: N/A | Tokens: N/A" instead of $0.00 / 0K
  And JSON report has zero values (not null)

Scenario: Report path in logDir
  Given Config.LogDir configured
  When report written
  Then path is <projectRoot>/<logDir>/ralph-run-<runID>.json
  And logDir created if not exists (os.MkdirAll)

Scenario: Report write failure non-fatal (NFR24)
  Given write to logDir fails (permissions, disk full)
  When error occurs
  Then logger.Error("failed to write run report", kv("error", err))
  And stdout summary still printed
  And ralph exits with normal exit code (report is best-effort)

Scenario: Additive-only schema (NFR22)
  Given JSON report schema
  When future stories add new metrics
  Then new fields added, existing fields never removed or renamed
  And documented in architecture shard
```

**Technical Notes:**
- `cmd/ralph/run.go`: after Runner.Execute returns, marshal RunMetrics to JSON. Write to logDir. Print text summary to stdout with fatih/color.
- Text summary: helper function `formatSummary(m *RunMetrics) string` in `cmd/ralph/run.go`.
- Token formatting: `formatTokens(n int) string` — "125K" for 125000, "1.2M" for 1200000, exact for < 1000.
- This story is the capstone — it assembles all data from stories 7.1-7.9 into final output.

**Prerequisites:** Stories 7.1-7.9 (all metrics must be collected to report them)

---

## FR Coverage Matrix

| FR | Story | Description |
|----|-------|-------------|
| FR42 | 7.1 | Token usage parsing from Claude Code JSON |
| FR43 | 7.3 | Cost calculation with model-aware pricing |
| FR44 | 7.2 | Git diff stats after commit |
| FR45 | 7.3 | Per-task cost aggregation, gate display |
| FR46 | 7.4 | ReviewResult enrichment with severity findings |
| FR47 | 7.1 | Run ID + Task ID generation and correlation |
| FR48 | 7.1 | step_type key in RunLogger |
| FR49 | 7.5 | Stuck detection (no commit N attempts) |
| FR50 | 7.7 | Budget alerts (warn + emergency gate) |
| FR51 | 7.8 | Similarity detection (Jaccard, dual threshold) |
| FR52 | 7.9 | Error categorization (transient/persistent) |
| FR53 | 7.6 | Gate decision aggregation with timing |
| FR54 | 7.9 | Latency breakdown per phase |
| FR55 | 7.10 | JSON run summary report |
| FR56 | 7.10 | Text summary in stdout |

**Coverage: 15/15 FRs → 10 stories. 100% FR coverage.**
