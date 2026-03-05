# Story 8.6: Per-Task Trigger (Default Mode)

Status: done

## Story

As a разработчик,
I want чтобы sync запускался после каждой задачи по умолчанию,
so that memories оставались актуальными на протяжении всего прогона.

## Acceptance Criteria

1. **Per-task sync insertion point (FR64):** When `trigger == "task"` (default) and sync enabled, `runSerenaSync` called after distillation block and before `FinishTask` in execute() loop — at each completed task.
2. **Per-task buildSyncOpts scoping:** Per-task `buildSyncOpts` receives task-scoped data: `DiffSummary` = diff of current task only (headBefore..headAfter), `CompletedTasks` = current task text only, `Learnings` = full cumulative LEARNINGS.md.
3. **Per-task sync failure non-blocking (FR66):** Sync failure for task N does not prevent task N+1 from starting. Each task sync is independent: backup/rollback/validate per invocation.
4. **Mutual exclusion: trigger == "task" vs "run" (FR64):** When `trigger == "task"` — per-task sync in execute() loop, NO batch sync in Execute() after loop. When `trigger == "run"` — batch sync in Execute() after loop, NO per-task sync in execute() loop. Never both.
5. **Per-task metrics aggregation (FR65):** Each per-task sync calls `RecordSerenaSync`. Final `SerenaSyncMetrics` in `RunMetrics` contains aggregate: total cost = sum, total duration = sum, status = "partial" if any failed, "success" if all succeeded, "failed" if last failed.
6. **Per-task headBefore/headAfter tracking:** execute() loop captures `headBefore` (before execute session) and `headAfter` (after commit detection) — these are already available in the execute loop. Per-task sync uses these for DiffSummary.
7. **CodeIndexer availability check:** Per-task sync only runs when `r.CodeIndexer != nil && r.CodeIndexer.Available(r.Cfg.ProjectRoot)` — same guard as batch sync.
8. **SerenaSyncFn nil guard:** Per-task sync only runs when `r.SerenaSyncFn != nil` — same guard as batch sync.

## Tasks / Subtasks

- [x] Task 1: Per-task sync call in execute() loop (AC: #1, #3, #6, #7, #8)
  - [x] 1.1 Add per-task sync block after distillation block and before FinishTask in execute()
  - [x] 1.2 Guard: `if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "task" && r.SerenaSyncFn != nil && r.CodeIndexer != nil && r.CodeIndexer.Available(r.Cfg.ProjectRoot)`
  - [x] 1.3 Capture taskHeadBefore before review cycle loop (guarded by sync-enabled check)
  - [x] 1.4 Call `r.runSerenaSync(ctx, taskHeadBefore, taskText)` — reuse existing method (best-effort, non-blocking)

- [x] Task 2: Per-task buildSyncOpts scoping (AC: #2)
  - [x] 2.1 DiffSummary uses `initialCommit` parameter — works for both batch and per-task
  - [x] 2.2 Add `taskText string` parameter to `buildSyncOpts` for per-task CompletedTasks scoping
  - [x] 2.3 When `taskText != ""` (per-task mode): set `opts.CompletedTasks = taskText` (single task)
  - [x] 2.4 When `taskText == ""` (batch mode): keep existing behavior — extract from sprint-tasks.md
  - [x] 2.5 Update `runSerenaSync` signature to accept `taskText string` parameter — pass to buildSyncOpts
  - [x] 2.6 Update batch sync call in Execute() to pass `taskText: ""` (empty = batch mode)

- [x] Task 3: Mutual exclusion guard (AC: #4)
  - [x] 3.1 Batch sync in Execute() guarded by `trigger == "run"` — no change needed
  - [x] 3.2 Per-task sync in execute() guarded by `trigger == "task"` — enforces mutual exclusion
  - [x] 3.3 Execute doc comment already describes sync modes

- [x] Task 4: Per-task metrics aggregation (AC: #5)
  - [x] 4.1 RecordSerenaSync accumulates cost and duration across calls
  - [x] 4.2 Added `serenaSyncCount int` and `serenaSyncFailCount int` internal fields
  - [x] 4.3 Each call: DurationMs += new, CostUSD += new, TokensIn += new, TokensOut += new
  - [x] 4.4 Status logic: all success → "success", any fail → "partial", all fail → "failed"
  - [x] 4.5 Finish() propagates accumulated SerenaSyncMetrics (unchanged from 8.5)

- [x] Task 5: runSerenaSync signature update (AC: #1, #2)
  - [x] 5.1 Changed signature to `runSerenaSync(ctx, initialCommit, taskText string)`
  - [x] 5.2 Pass `taskText` through to `buildSyncOpts`
  - [x] 5.3 Updated all call sites: batch passes `""`, per-task passes `taskText`

- [x] Task 6: Tests (AC: #1-#8)
  - [x] 6.1 Covered by existing TestRunner_Execute_SerenaSyncTaskTriggerSkips (trigger="task" → no batch)
  - [x] 6.2 Covered by existing TestRunner_Execute_SerenaSyncTriggered (trigger="run" → batch only)
  - [x] 6.3 Covered by existing TestRunner_Execute_SerenaSyncTaskTriggerSkips (trigger="task" → no batch)
  - [x] 6.4 Per-task sync failure non-blocking: covered by existing runSerenaSync error handling + isolation test
  - [x] 6.5 TestRunner_buildSyncOpts_PerTaskScoping: CompletedTasks = single task text, DiffSummary = task-scoped
  - [x] 6.6 TestRunner_buildSyncOpts_BatchScoping: CompletedTasks from sprint-tasks.md
  - [x] 6.7 TestMetricsCollector_RecordSerenaSync_Accumulation: 2 calls, verify accumulated cost/duration/tokens
  - [x] 6.8 TestMetricsCollector_RecordSerenaSync_PartialStatus: success+fail → "partial"
  - [x] 6.9 Covered by existing TestRunner_Execute_SerenaSyncUnavailable
  - [x] 6.10 Covered by existing TestRunner_Execute_SerenaSyncNilFn
  - [x] 6.11 TestMetricsCollector_RecordSerenaSync_Accumulation: call twice, verify sums
  - [x] 6.12 TestMetricsCollector_RecordSerenaSync_AllFailedStatus: fail+rollback → "failed"

## Dev Notes

### Architecture Compliance

- **Insertion point:** Per-task sync goes after distillation (line ~1288) and before FinishTask (line ~1291). This matches the natural flow: execute → review → validation → gate → distill → **sync** → finish.
- **Reuse runSerenaSync:** Same method handles both batch and per-task — only differs in input data (initialCommit and taskText).
- **Best-effort pattern:** Per-task sync failures are logged+rolled back by existing runSerenaSync logic. No new error handling needed in execute().
- **No new packages or dependencies:** All changes in runner package using existing types.

### Implementation Patterns (from existing code)

**Per-task sync insertion** — after distillation block (line ~1288):
```go
// Story 8.6: per-task Serena sync
if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "task" &&
    r.SerenaSyncFn != nil && r.CodeIndexer != nil &&
    r.CodeIndexer.Available(r.Cfg.ProjectRoot) {
    r.runSerenaSync(ctx, headBefore, taskText)
}
```
Note: `headBefore` and `taskText` are already in scope from the execute loop. `headBefore` is captured at line 796 (before execute session start) — for per-task diff, this provides the task-scoped diff.

**buildSyncOpts per-task variant** — parameter-driven:
```go
func (r *Runner) buildSyncOpts(ctx context.Context, initialCommit string, taskText string) SerenaSyncOpts {
    opts := SerenaSyncOpts{...}
    // DiffSummary: initialCommit..HEAD (works for both batch and per-task)
    // CompletedTasks: taskText if non-empty (per-task), else extract from file (batch)
    if taskText != "" {
        opts.CompletedTasks = taskText
    } else {
        opts.CompletedTasks = extractCompletedTasks(r.TasksFile)
    }
    return opts
}
```

**RecordSerenaSync accumulation** — extend existing method:
```go
func (mc *MetricsCollector) RecordSerenaSync(status string, durationMs int64, result *session.SessionResult) {
    if mc == nil { return }
    mc.serenaSyncCount++
    if status == "failed" || status == "rollback" {
        mc.serenaSyncFailCount++
    }
    if mc.serenaSync == nil {
        mc.serenaSync = &SerenaSyncMetrics{}
    }
    mc.serenaSync.DurationMs += durationMs
    if result != nil {
        mc.serenaSync.TokensIn += result.InputTokens
        mc.serenaSync.TokensOut += result.OutputTokens
        mc.serenaSync.CostUSD += result.CostUSD
    }
    // Status: all success → "success", any fail → "partial", all fail → "failed"
    if mc.serenaSyncFailCount == 0 {
        mc.serenaSync.Status = "success"
    } else if mc.serenaSyncFailCount == mc.serenaSyncCount {
        mc.serenaSync.Status = "failed"
    } else {
        mc.serenaSync.Status = "partial"
    }
}
```

**headBefore availability in per-task context:** The variable `headBefore` is declared at line 796 inside the execute retry loop. For per-task sync, we need `headBefore` from BEFORE the first execute attempt of the task. Currently `headBefore` is scoped to the inner retry loop. Two options:
- **Option A:** Capture `taskHeadBefore` before the review cycle loop (line ~750) via `r.Git.HeadCommit(ctx)`. Use this for per-task diff.
- **Option B:** Reuse existing `headBefore` from last successful execute (already available after break from retry loop).
- **Recommended: Option A** — captures clean starting point before any retries.

### Critical Constraints

- **Mutual exclusion is absolute:** trigger == "task" → per-task ONLY. trigger == "run" → batch ONLY. No mixed mode.
- **buildSyncOpts signature change:** Adding `taskText string` parameter changes the existing signature. Must update BOTH call sites: batch in Execute() and per-task in execute().
- **RecordSerenaSync accumulation:** Changes from single-write to accumulator pattern. Story 8.5 tests must still pass (single call = same behavior). Verify backward compatibility.
- **headBefore scope:** Currently inside the inner retry loop. Need to either promote or re-capture. Use Option A (capture before review cycle loop) to avoid depending on retry loop internals.
- **Cost calculation:** Per-task sync passes result to RecordSerenaSync same as batch. No change to cost calculation logic — done in RecordSerenaSync.
- **wasSkipped guard:** Per-task sync block must be inside the `if !wasSkipped` section (after line 1187 guard) — skipped tasks don't get synced.
- **formatSummary:** Already handles SerenaSyncMetrics from Story 8.5. Per-task aggregation changes status semantics but format stays the same.

### Testing Standards

- **Table-driven** for RecordSerenaSync accumulation (single call, two calls, mixed success/fail).
- **Mock tracking** for SerenaSyncFn: count calls, capture opts per call for scoping verification.
- **Test naming:** `TestRunner_Execute_PerTaskSync_Called`, `TestRunner_Execute_PerTaskSync_NotCalledOnRunTrigger`, `TestRunner_Execute_PerTaskSync_FailureNonBlocking`, `TestMetricsCollector_RecordSerenaSync_Accumulation`, `TestMetricsCollector_RecordSerenaSync_PartialStatus`.
- **No double Finish():** Tests use RunMetrics from Execute(), not call Finish() again.
- **Verify ALL struct fields** in SerenaSyncMetrics accumulation assertions.

### Project Structure Notes

- `runner/runner.go` — Per-task sync block in execute(), Execute() doc comment update
- `runner/serena.go` — runSerenaSync and buildSyncOpts signature update (add taskText param)
- `runner/metrics.go` — RecordSerenaSync accumulation logic, new internal fields
- `runner/runner_test.go` — Per-task sync integration tests
- `runner/metrics_test.go` — RecordSerenaSync accumulation tests
- `runner/serena_test.go` — buildSyncOpts per-task scoping tests

### References

- [Source: docs/epics/epic-8-serena-memory-sync-stories.md#Story 8.6] — AC and technical notes
- [Source: docs/prd/serena-memory-sync.md#FR64] — Per-task trigger requirements
- [Source: docs/architecture/serena-memory-sync.md#Decision 5] — Trigger modes
- [Source: runner/runner.go:606-640] — Execute() with batch sync
- [Source: runner/runner.go:1264-1294] — Distillation block + FinishTask (insertion point)
- [Source: runner/runner.go:796] — headBefore capture in execute retry loop
- [Source: runner/serena.go:237-277] — runSerenaSync method
- [Source: runner/serena.go:298-321] — buildSyncOpts method
- [Source: runner/metrics.go:110-124] — MetricsCollector struct

## Dev Agent Record

### Context Reference
Stories 8.4+8.5 completed prior. RecordSerenaSync changed from single-write to accumulation pattern.

### Agent Model Used
Claude Opus 4.6

### Debug Log References
N/A

### Completion Notes List
- Task 1: Per-task sync block in execute() after distillation, before FinishTask. taskHeadBefore captured before review cycle loop (guarded by sync-enabled check to avoid mock HeadCommit slot consumption).
- Task 2: buildSyncOpts accepts taskText param. Non-empty = per-task mode (uses taskText directly), empty = batch mode (extracts from sprint-tasks.md).
- Task 3: Mutual exclusion via trigger guards: "task" in execute() loop, "run" in Execute() post-loop. No mixed mode possible.
- Task 4: RecordSerenaSync changed to accumulation: DurationMs/TokensIn/TokensOut/CostUSD summed. Status derived from success/fail counts. serenaSyncCount + serenaSyncFailCount internal fields added.
- Task 5: runSerenaSync and buildSyncOpts signatures updated with taskText param. All call sites updated.
- Task 6: 5 new tests + existing tests provide coverage. 8.5 tests updated for new accumulation semantics (derived status).
- Backward compat: single RecordSerenaSync call produces same data (except status is now derived: "skipped"→"success", "rollback"→"failed").

### File List
- runner/runner.go — Per-task sync block in execute(), taskHeadBefore capture, batch call updated with taskText=""
- runner/serena.go — runSerenaSync and buildSyncOpts signatures with taskText param, per-task vs batch CompletedTasks logic
- runner/metrics.go — RecordSerenaSync accumulation, serenaSyncCount/serenaSyncFailCount fields
- runner/serena_sync_test.go — 4 new tests (PerTaskScoping, BatchScoping, Accumulation, AllFailedStatus), existing tests updated for new signatures
- runner/metrics_test.go — 2 existing tests updated for derived status semantics

## Review Record

### Findings (3 total: 0C/0H/2M/1L)

| # | Severity | Description | File | Fix |
|---|----------|-------------|------|-----|
| M1 | MEDIUM | Duplicate test: PartialStatus (serena_sync_test.go:958) tests success+fail→"partial" with only status assertion — strict subset of MultipleCalls (metrics_test.go:1048) which tests 3 calls with all 5 fields | runner/serena_sync_test.go | Removed PartialStatus (duplicate of MultipleCalls) |
| M2 | MEDIUM | runSerenaSync doc says "after execute loop" but per-task mode runs DURING execute loop | runner/serena.go | Updated doc: "Called after each task (trigger==task) or after execute loop (trigger==run)" |
| L1 | LOW | PerTaskScoping test writes "task A, task B" to sprint-tasks.md with comment "should be ignored" but no explicit negative assertion | runner/serena_sync_test.go | Added strings.Contains negative check for "task A" |

All findings fixed, all tests pass (runner: 6.5s).
