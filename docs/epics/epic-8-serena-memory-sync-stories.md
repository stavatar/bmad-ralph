# Epic 8: Serena Memory Sync — Stories

**Scope:** FR57-FR66, NFR26-NFR29
**Stories:** 7
**Release milestone:** v0.5
**PRD:** [docs/prd/serena-memory-sync.md](../prd/serena-memory-sync.md)
**Architecture:** [docs/architecture/serena-memory-sync.md](../architecture/serena-memory-sync.md)

**Context:**
Ralph детектит Serena MCP и использует его для token economy, но не обновляет Serena memories после прогона. Epic 8 добавляет отдельную sync-сессию (Claude subprocess), которая обновляет memories на основе git diff, LEARNINGS.md и завершённых задач. Backup/rollback защищает от порчи. Zero new dependencies.

**Dependency structure:**
```
8.1 Config + CLI flags ───┐
8.2 Sync prompt template ─┼──→ 8.4 runSerenaSync core ──→ 8.5 Sync metrics
8.3 Backup/Rollback ──────┘                             └──→ 8.6 Per-task trigger (Growth)

8.7 Integration tests (depends on 8.4 + 8.5 + 8.6)
```

**Existing scaffold:**
- `runner/serena.go` — CodeIndexerDetector interface (Available, PromptHint), SerenaMCPDetector, DetectSerena()
- `runner/runner.go` — Runner struct (14 fields), Execute() wraps execute(), buildTemplateData()
- `config/config.go` — Config struct (25+ fields), CLIFlags, defaults cascade, Validate()
- `config/prompt.go` — TemplateData struct, AssemblePrompt() with two-stage assembly
- `session/result.go` — Execute(), ParseResult(), SessionResult
- `runner/metrics.go` — MetricsCollector, RunMetrics, TaskMetrics
- `runner/prompts/` — go:embed templates (execute.md, review.md, serena-sync.md TBD)

---

### Story 8.1: Sync Config + CLI Flag

**User Story:**
Как разработчик, я хочу настроить Serena sync через config файл и CLI флаг, чтобы включать автоматическую синхронизацию memories без правки кода.

**Acceptance Criteria:**

```gherkin
Scenario: Config fields (FR63)
  Given Config struct extended with 3 new fields:
    SerenaSyncEnabled bool yaml:"serena_sync_enabled"    (default false)
    SerenaSyncMaxTurns int yaml:"serena_sync_max_turns"  (default 5)
    SerenaSyncTrigger string yaml:"serena_sync_trigger"  (default "task")
  When defaults.yaml has serena_sync_enabled: false, serena_sync_max_turns: 5, serena_sync_trigger: "task"
  Then all fields parsed from .ralph/config.yaml with fallback to defaults
  And SerenaSyncEnabled == false by default (sync disabled)

Scenario: CLI flag --serena-sync (FR63)
  Given cmd/ralph/run.go defines --serena-sync flag
  When ralph run --serena-sync executed
  Then CLIFlags.SerenaSyncEnabled == true
  And CLI override applied: serena_sync_enabled = true in resolved Config
  And existing CLI cascade works: CLI > config > defaults

Scenario: Validate trigger values (FR63)
  Given Config.SerenaSyncTrigger set to arbitrary string
  When Config.Validate() called
  Then "run" → valid
  And "task" → valid
  And "" → valid (treated as "task" default)
  And "invalid" → error: "config: validate: invalid serena_sync_trigger \"invalid\" (must be \"run\" or \"task\")"

Scenario: Validate max turns (FR63)
  Given Config.SerenaSyncMaxTurns set to various values
  When Config.Validate() called
  Then 0 → corrected to 5 (enforce minimum)
  And negative → corrected to 5
  And 1..100 → valid
  And existing validation rules unchanged

Scenario: Config round-trip
  Given config.yaml with all 3 serena_sync fields set
  When config.Load() called
  Then all fields populated from YAML
  And config.yaml without serena_sync fields → defaults applied
```

**Technical Notes:**
- `config/config.go`: add 3 fields to Config struct. Extend Validate() with trigger enum check and MaxTurns minimum.
- `config/defaults.yaml`: add 3 default values.
- `cmd/ralph/run.go`: add `--serena-sync` boolean flag. Extend `applyCLIFlags()`.
- No runtime behavior changes — config only. Sync logic in Story 8.4.

**Prerequisites:** None (foundation story)

---

### Story 8.2: Sync Prompt Template

**User Story:**
Как разработчик, я хочу иметь специализированный промпт для sync-сессии, чтобы Claude фокусировался исключительно на обновлении Serena memories.

**Acceptance Criteria:**

```gherkin
Scenario: Prompt file exists (FR59)
  Given runner/prompts/serena-sync.md created as Go-шаблон
  When embedded via go:embed
  Then file accessible through embed.FS alongside execute.md and review.md
  And template compiles without error via text/template.Parse

Scenario: Two-stage assembly (FR59)
  Given prompt template contains:
    - Stage 1: {{if .HasLearnings}} conditional block
    - Stage 1: {{if .HasCompletedTasks}} conditional block
    - Stage 2: __DIFF_SUMMARY__ placeholder
    - Stage 2: __LEARNINGS_CONTENT__ placeholder
    - Stage 2: __COMPLETED_TASKS__ placeholder
  When assembleSyncPrompt(opts) called
  Then Stage 1: text/template renders conditionals
  And Stage 2: strings.Replace injects user content
  And no {{.Var}} in final output (all resolved)
  And no __PLACEHOLDER__ in final output (all replaced)

Scenario: TemplateData extension (Architecture Decision 3)
  Given config/prompt.go TemplateData struct
  When HasCompletedTasks bool field added
  Then field used by serena-sync.md template
  And execute.md and review.md unaffected (HasCompletedTasks=false)
  And existing TemplateData tests pass

Scenario: Prompt content — instructions
  Given sync prompt rendered
  When Claude receives it
  Then prompt instructs to use list_memories → read_memory → edit_memory/write_memory
  And prompt FORBIDS: deleting memories, creating without necessity
  And prompt PREFERS: edit_memory over write_memory (точечные обновления)
  And prompt contains sections: context, diff summary, learnings, tasks, instructions

Scenario: Prompt content — conditional sections
  Given HasLearnings == false (empty LEARNINGS.md)
  When prompt rendered
  Then "Извлечённые уроки" section absent
  And __LEARNINGS_CONTENT__ not in output

Scenario: assembleSyncPrompt function
  Given runner/serena.go defines assembleSyncPrompt(opts SerenaSyncOpts) (string, error)
  When called with populated opts
  Then returns assembled prompt string
  And error on template parse failure
```

**Technical Notes:**
- `runner/prompts/serena-sync.md` (NEW): Go-шаблон ~50 lines. Sections: Роль, Контекст изменений (diff), Уроки (learnings), Завершённые задачи, Инструкции обновления, Ограничения.
- `runner/serena.go`: `assembleSyncPrompt()` function. Two-stage: template.Execute + strings.Replace.
- `config/prompt.go`: add `HasCompletedTasks bool` to TemplateData.
- User-controlled content (__DIFF_SUMMARY__ etc.) injected via strings.Replace, NOT via {{.Field}} — protection from template injection (existing pattern).

**Prerequisites:** None (independent, template only)

---

### Story 8.3: Backup/Rollback Memories

**User Story:**
Как разработчик, я хочу чтобы sync создавал backup memories перед обновлением и автоматически восстанавливал при ошибке, чтобы мои данные были защищены.

**Acceptance Criteria:**

```gherkin
Scenario: backupMemories copies directory (FR61)
  Given .serena/memories/ contains 5 .md files
  When backupMemories(projectRoot) called
  Then .serena/memories.bak/ created with exact copies of all 5 files
  And previous .bak/ directory removed before copy (clean backup)
  And function returns nil on success

Scenario: backupMemories error handling
  Given .serena/memories/ does not exist
  When backupMemories called
  Then returns error: "runner: serena sync: backup: ..."
  And .serena/memories.bak/ not created

Scenario: rollbackMemories restores from backup (FR61)
  Given .serena/memories.bak/ contains original 5 files
  And .serena/memories/ was modified by sync (different content)
  When rollbackMemories(projectRoot) called
  Then .serena/memories/ restored to match .bak/ contents exactly
  And .bak/ preserved (not deleted by rollback)

Scenario: cleanupBackup removes backup (FR61)
  Given .serena/memories.bak/ exists after successful sync
  When cleanupBackup(projectRoot) called
  Then .serena/memories.bak/ directory removed
  And no error on missing .bak/ (idempotent)

Scenario: validateMemories count check (FR62)
  Given countBefore == 5 (memories before sync)
  When validateMemories(projectRoot, countBefore) called
  And countAfter == 5 or more
  Then returns nil (valid)

Scenario: validateMemories detects deletion (FR62)
  Given countBefore == 5
  When validateMemories(projectRoot, countBefore) called
  And countAfter == 4 (one memory deleted)
  Then returns error: "runner: serena sync: memory count decreased: 5 → 4"

Scenario: validateMemories graceful on read error (FR62)
  Given .serena/memories/ unreadable (permissions)
  When validateMemories called
  Then returns nil (skip validation, best effort)
  And no panic

Scenario: countMemoryFiles counts .md files
  Given .serena/memories/ contains 3 .md files and 1 .txt file and 1 subdirectory
  When countMemoryFiles(projectRoot) called
  Then returns 3 (only .md files, no dirs, no other extensions)

Scenario: NTFS compatibility (NFR27)
  Given WSL/NTFS filesystem
  When backup/rollback operations execute
  Then filepath.Walk used (no symlinks)
  And os.MkdirAll creates target directories
  And explicit file copy via os.ReadFile + os.WriteFile (no os.Link)
```

**Technical Notes:**
- `runner/serena.go`: constants `serenaMemoriesDir`, `serenaBackupDir`. Functions: `backupMemories`, `rollbackMemories`, `cleanupBackup`, `validateMemories`, `countMemoryFiles`, helper `copyDir` (recursive file copy via filepath.Walk).
- No symlinks — NTFS/WSL safe. Explicit copy only.
- `os.RemoveAll` before backup (clean) and before rollback (replace).
- Count validation: only `.md` files (Serena memory format), no dirs.

**Prerequisites:** None (independent, file ops only)

---

### Story 8.4: runSerenaSync — Core Sync Session + Integration

**User Story:**
Как разработчик использующий Serena, я хочу чтобы ralph автоматически запускал sync-сессию после прогона, чтобы Serena memories обновлялись без ручной работы.

**Acceptance Criteria:**

```gherkin
Scenario: SerenaSyncFn injectable field (Architecture Decision 2)
  Given Runner struct in runner/runner.go
  When SerenaSyncFn func(ctx, SerenaSyncOpts) error field added
  Then field follows pattern of DistillFn, ReviewFn, ResumeExtractFn
  And nil SerenaSyncFn means no sync capability

Scenario: SerenaSyncOpts struct (FR58)
  Given runner/serena.go defines SerenaSyncOpts
  Then struct contains:
    DiffSummary string, Learnings string, CompletedTasks string,
    MaxTurns int, ProjectRoot string
  And all fields populated from run context

Scenario: Sync triggered after run — batch mode (FR57, Architecture Decision 1)
  Given Config.SerenaSyncEnabled == true
  And Config.SerenaSyncTrigger == "run" (batch mode)
  And CodeIndexer.Available(projectRoot) == true
  And SerenaSyncFn != nil
  When Runner.Execute(ctx) completes execute() loop
  Then runSerenaSync(ctx) called BEFORE Metrics.Finish()
  And sync failure does NOT affect runErr (best-effort)
  And batch sync NOT called when trigger == "task" (per-task already ran)

Scenario: Sync skipped when disabled (FR57)
  Given Config.SerenaSyncEnabled == false
  When Runner.Execute completes
  Then runSerenaSync NOT called
  And no backup/rollback operations

Scenario: Sync skipped when Serena unavailable (FR66)
  Given Config.SerenaSyncEnabled == true
  But CodeIndexer.Available(projectRoot) == false (or CodeIndexer == nil)
  When Runner.Execute completes
  Then runSerenaSync NOT called
  And logger.Info with reason "serena not available"

Scenario: buildSyncOpts gathers context (FR58)
  Given run completed with 3 tasks, 2 commits, non-empty LEARNINGS.md
  When buildSyncOpts() called
  Then DiffSummary contains git diff --stat of run (first commit..HEAD)
  And Learnings contains LEARNINGS.md content
  And CompletedTasks contains lines with [x] from sprint-tasks.md
  And MaxTurns == Config.SerenaSyncMaxTurns

Scenario: buildSyncOpts with empty run
  Given run completed with 0 commits (all tasks skipped)
  When buildSyncOpts() called
  Then DiffSummary == "" (no diff to show)
  And Learnings may be empty or non-empty
  And CompletedTasks == "" (no newly completed tasks)

Scenario: RealSerenaSync implementation (FR57, FR60)
  Given RealSerenaSync(ctx, opts) called
  When session.Execute invoked with sync prompt
  Then MaxTurns from opts used in session.Options
  And session runs with --output-format json
  And ProjectRoot set as working directory
  And result parsed via session.ParseResult

Scenario: RealSerenaSync error handling
  Given sync session returns error or IsError
  When RealSerenaSync returns error
  Then error wrapped: "runner: serena sync: ..."
  And caller (runSerenaSync) triggers rollback

Scenario: Full sync flow (FR57 + FR61 + FR62)
  Given sync enabled and Serena available
  When runSerenaSync(ctx) executes
  Then sequence: countMemoryFiles → backupMemories → buildSyncOpts → SerenaSyncFn → validateMemories → cleanupBackup
  And on sync error: rollback + log warning
  And on validation error: rollback + log warning
  And on backup error: skip sync + log warning
  And on count error: skip sync + log warning

Scenario: extractCompletedTasks function
  Given sprint-tasks.md contains:
    "- [x] Task A\n- [ ] Task B\n- [x] Task C"
  When extractCompletedTasks(tasksFile) called
  Then returns "- [x] Task A\n- [x] Task C" (only completed)
```

**Technical Notes:**
- `runner/runner.go`: add `SerenaSyncFn` field to Runner. In Execute(): between `r.execute(ctx)` and `r.Metrics.Finish()`, call `r.runSerenaSync(ctx)` with 3-condition guard.
- `runner/serena.go`: `SerenaSyncOpts` struct, `RealSerenaSync`, `runSerenaSync` method, `buildSyncOpts`, `extractCompletedTasks`.
- `cmd/ralph/run.go`: wire `RealSerenaSync` as `Runner.SerenaSyncFn`.
- Git diff stat: reuse existing `GitClient.DiffStat` or run `git diff --stat <sha>..HEAD` directly.
- Session: reuse `session.Execute` — same subprocess pattern as distill.

**Prerequisites:** Stories 8.1, 8.2, 8.3

---

### Story 8.5: Sync Metrics + Stdout Summary

**User Story:**
Как разработчик, я хочу видеть результат sync-сессии (статус, стоимость, время) в run summary и JSON report, чтобы контролировать стоимость sync.

**Acceptance Criteria:**

```gherkin
Scenario: SerenaSyncMetrics struct (FR65)
  Given runner/metrics.go defines SerenaSyncMetrics
  Then struct contains:
    Status string json:"status"           (success/skipped/failed/rollback)
    DurationMs int64 json:"duration_ms"
    TokensIn int json:"tokens_input,omitempty"
    TokensOut int json:"tokens_output,omitempty"
    CostUSD float64 json:"cost_usd,omitempty"

Scenario: RunMetrics extension (FR65)
  Given RunMetrics struct in runner/metrics.go
  When SerenaSync *SerenaSyncMetrics json:"serena_sync,omitempty" field added
  Then field nil when sync disabled (omitted from JSON)
  And field populated when sync runs (success, failed, or rollback)

Scenario: RecordSerenaSync method (FR65)
  Given MetricsCollector has RecordSerenaSync(status, durationMs, sessionMetrics)
  When called after sync completes
  Then SerenaSyncMetrics populated in RunMetrics
  And if sessionMetrics != nil: tokens and cost extracted
  And if sessionMetrics == nil: only status and duration set

Scenario: recordSyncMetrics nil safety
  Given Runner.Metrics == nil (no collector)
  When recordSyncMetrics called
  Then no panic (nil guard)

Scenario: Stdout summary line (FR65)
  Given run completes with sync
  When text summary printed
  Then includes line: "Serena sync: success ($0.05, 12s)"
  Or: "Serena sync: rollback (validation failed, 8s)"
  Or: "Serena sync: skipped (disabled)"
  And line absent when sync never attempted (no Serena)

Scenario: JSON report contains sync data (FR65)
  Given RunMetrics serialized to JSON
  When sync was successful
  Then json contains "serena_sync" object with all fields
  And jq '.serena_sync.status' returns "success"
  And jq '.serena_sync.cost_usd' returns number
```

**Technical Notes:**
- `runner/metrics.go`: add `SerenaSyncMetrics` struct, extend `RunMetrics` with `SerenaSync *SerenaSyncMetrics` field. Add `RecordSerenaSync` method to MetricsCollector.
- `runner/serena.go`: in `runSerenaSync`, parse session result for tokens/cost, pass to `recordSyncMetrics`.
- `cmd/ralph/run.go`: extend `formatSummary` to include sync line. Conditional on `m.SerenaSync != nil`.
- `omitempty` on pointer field — JSON omits when nil.

**Prerequisites:** Story 8.4 (runSerenaSync must exist to record metrics)

---

### Story 8.6: Per-Task Trigger (Default Mode)

**User Story:**
Как разработчик, я хочу чтобы sync запускался после каждой задачи по умолчанию, чтобы memories оставались актуальными на протяжении всего прогона.

**Acceptance Criteria:**

```gherkin
Scenario: Per-task trigger activation — default mode (FR64)
  Given Config.SerenaSyncTrigger == "task" (default)
  And Config.SerenaSyncEnabled == true
  And Serena available
  When task N completes (after knowledge extraction, before gate)
  Then runSerenaSync(ctx) called for task N
  And backup/rollback executed per task

Scenario: Per-task buildSyncOpts scoped to task
  Given trigger == "task" and task N just completed
  When buildSyncOpts() called
  Then DiffSummary contains diff of THIS task only (commitBefore..commitAfter)
  And CompletedTasks contains only task N text
  And Learnings contains full LEARNINGS.md (cumulative)

Scenario: Per-task sync failure non-blocking
  Given sync for task N fails
  When rollback executed
  Then next task N+1 proceeds normally
  And sync attempted again for N+1 (independent)

Scenario: Per-task vs run trigger mutual exclusion
  Given trigger == "task"
  When all tasks complete
  Then NO additional batch sync in Execute() (per-task already ran)
  And trigger == "run" → only batch sync, no per-task sync

Scenario: Per-task metrics aggregation
  Given 5 tasks each trigger sync
  When RunMetrics reported
  Then SerenaSyncMetrics contains aggregate: total cost, total duration, final status
  And individual sync results logged per task

Scenario: Mixed success/failure across tasks
  Given task 1 sync succeeds, task 2 sync fails (rollback), task 3 sync succeeds
  When RunMetrics reported
  Then SerenaSyncMetrics.Status reflects overall: "partial" or last status
  And each task sync logged individually
```

**Technical Notes:**
- `runner/runner.go` execute(): after knowledge extraction block, if `trigger == "task"`: call `r.runSerenaSync(ctx)` with task-scoped opts.
- `buildSyncOpts` needs per-task variant: diff of single task (commitBefore..commitAfter), single completed task text.
- In `Execute()`: skip batch sync if `trigger == "task"` (already synced per task).
- Metrics: accumulate across tasks. Final SerenaSyncMetrics = aggregate.

**Prerequisites:** Story 8.4 (runSerenaSync)

---

### Story 8.7: Integration Tests

**User Story:**
Как разработчик, я хочу иметь интеграционные тесты для полного sync flow, чтобы убедиться что все компоненты работают вместе.

**Acceptance Criteria:**

```gherkin
Scenario: Happy path — sync after run
  Given Runner configured with:
    SerenaSyncEnabled: true, SerenaSyncFn: mock that returns nil
    CodeIndexer: mock with Available() → true
    Mock sprint-tasks.md with 2 tasks
    Mock LEARNINGS.md with content
  When Runner.Execute(ctx) completes
  Then SerenaSyncFn called exactly once
  And SerenaSyncOpts populated: DiffSummary non-empty, Learnings non-empty, CompletedTasks non-empty
  And RunMetrics.SerenaSync.Status == "success"

Scenario: Sync disabled — no sync call
  Given Runner configured with SerenaSyncEnabled: false
  When Runner.Execute(ctx) completes
  Then SerenaSyncFn NOT called
  And RunMetrics.SerenaSync == nil

Scenario: Serena unavailable — graceful skip
  Given Runner configured with SerenaSyncEnabled: true
  But CodeIndexer.Available() → false
  When Runner.Execute(ctx) completes
  Then SerenaSyncFn NOT called
  And log contains "serena not available"

Scenario: Sync failure triggers rollback
  Given Runner configured with SerenaSyncFn: mock that returns error
  And t.TempDir with .serena/memories/ containing 3 files
  When Runner.Execute(ctx) completes
  Then backup created (.serena/memories.bak/)
  And rollback executed (memories restored from .bak/)
  And RunMetrics.SerenaSync.Status == "rollback" or "failed"
  And runner exit code unaffected by sync failure

Scenario: Validation failure triggers rollback
  Given SerenaSyncFn succeeds but deletes a memory file during sync
  And countBefore == 5, countAfter == 4
  When validateMemories detects decrease
  Then rollback from backup
  And RunMetrics.SerenaSync.Status == "rollback"
  And log contains "validation failed"

Scenario: Config round-trip integration
  Given config.yaml with serena_sync_enabled: true, serena_sync_max_turns: 3
  When config.Load() + Runner wiring
  Then Config fields correctly populated
  And MaxTurns == 3 in SerenaSyncOpts

Scenario: CLI flag integration
  Given ralph run --serena-sync
  When CLI flags parsed
  Then Config.SerenaSyncEnabled == true
  And sync triggered if Serena available
```

**Technical Notes:**
- `runner/runner_serena_sync_integration_test.go` (NEW): integration tests using mock SerenaSyncFn, mock CodeIndexer, t.TempDir for file operations.
- Pattern: same as `runner_integration_test.go` — mock claude via config.ClaudeCommand, real file operations for backup/rollback.
- Mock SerenaSyncFn: `func(ctx, opts) error { ... }` captures opts for assertion.
- Backup/rollback tests: create real .serena/memories/ in t.TempDir, verify file contents after rollback.
- Config tests: extend existing config_test.go with new fields.

**Prerequisites:** Stories 8.4, 8.5 (core + metrics)

---

## FR Coverage Matrix

| FR | Story | Description |
|----|-------|-------------|
| FR57 | 8.4 | Sync session after run (conditional, CodeIndexer + config) |
| FR58 | 8.4 | Context gathering: git diff, LEARNINGS.md, completed tasks |
| FR59 | 8.2 | Sync prompt template (Go template, two-stage assembly) |
| FR60 | 8.4 | Max turns limit for sync session |
| FR61 | 8.3 | Backup memories before sync, rollback on failure |
| FR62 | 8.3 | Validate memory count after sync |
| FR63 | 8.1 | Config fields + CLI flag + validation |
| FR64 | 8.6 | Per-task trigger (default mode) |
| FR65 | 8.5 | Sync metrics in RunMetrics + stdout summary |
| FR66 | 8.4 | Graceful skip on MCP unavailable |

**Coverage: 10/10 FRs → 7 stories. 100% FR coverage.**
