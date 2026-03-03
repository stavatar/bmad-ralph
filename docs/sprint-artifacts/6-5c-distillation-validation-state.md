# Story 6.5c: Distillation Validation & State

Status: review

## Story

As a runner,
I want post-validation of distillation output and reliable state persistence with crash recovery,
so that distillation is safe and recoverable.

## Acceptance Criteria

```gherkin
Scenario: Post-validation rejects bad distillation (ValidateDistillation, v6 simplified)
  Given auto-distillation produced output
  When ValidateDistillation(old, new) runs
  Then checks 3 criteria (v6 — simplified from 7):
    1. Output <= 200 lines (budget guard)
    2. Citation preservation >= 80% (no mass knowledge loss)
    3. All [needs-formatting] entries either fixed or preserved (none silently dropped)
  And if any check fails: treated as distillation failure (triggers human gate per Story 6.5a)

Scenario: DistillState extended with Metrics struct
  Given distillation state needs persistence
  When DistillState serialized
  Then stored at `{projectRoot}/.ralph/distill-state.json`
  And contains: Version int, MonotonicTaskCounter int, LastDistillTask int, Categories []string, Metrics struct
  And Version field provides forward compatibility

Scenario: DistillState includes Version field for forward compatibility
  Given DistillState JSON structure
  When deserialized
  Then Version int field present (current: 1)
  And future versions can migrate from older formats
  And missing Version treated as Version: 0 (pre-versioning)

Scenario: DistillState included in backup rotation
  Given distillation about to run
  When backups created (2-generation)
  Then .ralph/distill-state.json backed up alongside LEARNINGS.md and ralph-*.md
  And backup rotation: .bak + .bak.1 (same as other files)

Scenario: Atomic multi-file distillation via intent file (CR1)
  Given distillation about to write multiple files (LEARNINGS.md, ralph-*.md, distill-state.json)
  When write sequence starts
  Then Phase 1: create backups → write .pending files → write `.ralph/distill-intent.json`
  And Phase 2: rename .pending → target → update distill-state.json → delete intent file
  And `.ralph/distill-intent.json` contains: timestamp, list of target files, phase (backup|write|commit)
  And if any rename fails in Phase 2: remaining files left as .pending (recoverable)

Scenario: Crash recovery at startup (M7)
  Given runner starts
  When startup check runs
  Then if `.ralph/distill-intent.json` exists: interrupted distillation detected
  And Phase 2 incomplete → complete pending renames OR rollback from .bak (based on phase field)
  And if no intent file but .bak files exist → normal state (previous successful distillation)
  And log warning: "Recovered from interrupted distillation" (only when intent file found)
  And clean up: delete intent file + .pending files after recovery

Scenario: Effectiveness metrics after distillation
  Given auto-distillation completed
  When metrics computed
  Then log includes: entries before/after, stale removed count, categories preserved/total,
    [needs-formatting] fixed count, T1 promotions count
  And metrics written to DistillState for trend tracking
```

## Tasks / Subtasks

- [x] Task 1: Implement ValidateDistillation — replace stub (AC: #1)
  - [x] 1.1 Replace `ValidateDistillation` stub in `runner/knowledge_distill.go:531-533`
  - [x] 1.2 Signature: `func ValidateDistillation(output *DistillOutput, oldContent string, budgetLimit int) error`
  - [x] 1.3 Update call site in `AutoDistill` (knowledge_distill.go:595): pass oldContent and budgetLimit
  - [x] 1.4 Criterion 1 — Budget guard: count total lines in output.CompressedLearnings, reject if > budgetLimit (200)
  - [x] 1.5 Criterion 2 — Citation preservation >= 80%: parse citations from old content (citationRegex from knowledge_write.go), count preserved in new output, reject if preserved/total < 0.80
  - [x] 1.6 Criterion 3 — [needs-formatting] handling: count `[needs-formatting]` in old content, verify none silently dropped (each must be either fixed or still present in new output)
  - [x] 1.7 On ANY criterion failure: return descriptive error (NOT ErrBadFormat — this is validation failure, not parse failure)
  - [x] 1.8 Define `ErrValidationFailed` sentinel: `var ErrValidationFailed = errors.New("validation failed")`
  - [x] 1.9 Error wrapping: `"runner: distill: validate: criterion N: %w"` prefix with criterion number
  - [x] 1.10 Return nil if all 3 criteria pass

- [x] Task 2: Extend DistillState with Metrics (AC: #2, #7)
  - [x] 2.1 Add `DistillMetrics` struct to knowledge_state.go:
    - `EntriesBefore int`, `EntriesAfter int`, `StaleRemoved int`
    - `CategoriesPreserved int`, `CategoriesTotal int`
    - `NeedsFormattingFixed int`, `T1Promotions int`
    - `LastDistillTime string` (ISO 8601 timestamp)
  - [x] 2.2 Add `Metrics *DistillMetrics \`json:"metrics,omitempty"\`` to DistillState struct
  - [x] 2.3 Existing tests must still pass (omitempty means nil = not serialized)

- [x] Task 3: Compute and store effectiveness metrics (AC: #7)
  - [x] 3.1 `func ComputeDistillMetrics(oldContent string, output *DistillOutput) *DistillMetrics`
  - [x] 3.2 EntriesBefore: count `## ` headers in old content (parseEntries from knowledge_write.go)
  - [x] 3.3 EntriesAfter: count total entries across all categories in output
  - [x] 3.4 StaleRemoved: EntriesBefore - (entries matched in new output)
  - [x] 3.5 CategoriesPreserved/Total: from output.Categories
  - [x] 3.6 NeedsFormattingFixed: count `[needs-formatting]` in old, count in new, diff
  - [x] 3.7 T1Promotions: count entries with Freq >= 10 in output
  - [x] 3.8 LastDistillTime: `time.Now().UTC().Format(time.RFC3339)`
  - [x] 3.9 Wire into AutoDistill: compute metrics AFTER write, store in state.Metrics, save state
  - [x] 3.10 Log metrics: `fmt.Fprintf(os.Stderr, "Distillation metrics: %d→%d entries, %d stale removed, %d [needs-formatting] fixed, %d T1 promotions\n", ...)`

- [x] Task 4: Implement intent file for atomic multi-file write (AC: #5, CR1)
  - [x] 4.1 Define `DistillIntent` struct: `Timestamp string`, `Files []string`, `Phase string` (backup|write|commit)
  - [x] 4.2 Intent file path: `{projectRoot}/.ralph/distill-intent.json`
  - [x] 4.3 Modify `AutoDistill` write sequence:
    - Phase "backup": create backups (already done in Step 1)
    - Phase "write": write all output files as `.pending` suffix first (e.g., `LEARNINGS.md.pending`)
    - Write intent file with phase="write" and list of target files
    - Phase "commit": rename `.pending` → target for each file
    - Update distill-state.json
    - Delete intent file
  - [x] 4.4 `func WriteIntentFile(projectRoot string, intent *DistillIntent) error`
  - [x] 4.5 `func DeleteIntentFile(projectRoot string) error`
  - [x] 4.6 `func ReadIntentFile(projectRoot string) (*DistillIntent, error)` — returns nil, nil on NotExist
  - [x] 4.7 Modify `WriteDistillOutput`: write to `.pending` paths, return list of target files
  - [x] 4.8 New `func CommitPendingFiles(files []string) error` — rename `.pending` → target
  - [x] 4.9 If any rename fails in Phase 2: remaining files left as `.pending` (recoverable by crash recovery)
  - [x] 4.10 Error wrapping: `"runner: distill: intent:"` prefix

- [x] Task 5: Implement crash recovery at startup (AC: #6, M7)
  - [x] 5.1 `func RecoverDistillation(projectRoot string) error` in knowledge_state.go
  - [x] 5.2 Read intent file: if NotExist → return nil (normal state, no recovery needed)
  - [x] 5.3 If intent file exists with phase="backup": rollback — delete any .pending files, done
  - [x] 5.4 If intent file exists with phase="write": complete pending renames (CommitPendingFiles)
  - [x] 5.5 If intent file exists with phase="commit": same as "write" — complete remaining renames
  - [x] 5.6 Clean up: delete intent file + any remaining .pending files after recovery
  - [x] 5.7 Log: `fmt.Fprintf(os.Stderr, "Recovered from interrupted distillation\n")`
  - [x] 5.8 Wire into runner startup: call `RecoverDistillation(cfg.ProjectRoot)` in Execute() BEFORE RecoverDirtyState
  - [x] 5.9 Error wrapping: `"runner: distill: recovery:"` prefix

- [x] Task 6: DistillState backup in rotation (AC: #4)
  - [x] 6.1 Verify `BackupDistillationFiles` (from Story 6.5b) already backs up distill-state.json — CONFIRMED (knowledge_distill.go:173-176)
  - [x] 6.2 This task is already implemented in Story 6.5b — test coverage exists in TestBackupDistillationFiles_AllFiles
  - [x] 6.3 Verified: TestBackupDistillationFiles_AllFiles covers distill-state.json backup

- [x] Task 7: Tests (AC: all) — 19 tests written, all passing
  - [x] 7.1 `TestValidateDistillation_AllPass` — all 3 criteria pass, returns nil
  - [x] 7.2 `TestValidateDistillation_BudgetExceeded` — >200 lines → error with "criterion 1"
  - [x] 7.3 `TestValidateDistillation_CitationLoss` — <80% citations preserved → error with "criterion 2"
  - [x] 7.4 `TestValidateDistillation_NeedsFormattingDropped` — [needs-formatting] entry silently dropped → error with "criterion 3"
  - [x] 7.5 `TestValidateDistillation_NeedsFormattingFixed` — [needs-formatting] removed (fixed) → passes
  - [x] 7.6 `TestValidateDistillation_NeedsFormattingPreserved` — [needs-formatting] still present → passes
  - [x] 7.7 `TestComputeDistillMetrics_AllFields` — verify all metric fields computed correctly
  - [x] 7.8 `TestComputeDistillMetrics_NoStale` — all entries preserved → StaleRemoved=0
  - [x] 7.9 `TestDistillState_MetricsSerialization` — round-trip with Metrics field (save → load → verify)
  - [x] 7.10 `TestDistillState_MetricsOmitEmpty` — nil Metrics → no "metrics" key in JSON
  - [x] 7.11 `TestDistillState_VersionZero` — missing Version → treated as 0 (pre-versioning)
  - [x] 7.12 `TestWriteIntentFile_RoundTrip` — write → read → verify fields
  - [x] 7.13 `TestReadIntentFile_NotExist` — returns nil, nil
  - [x] 7.14 `TestCommitPendingFiles_AllRenamed` — .pending → target for all files
  - [x] 7.15 `TestCommitPendingFiles_SkipsMissing` — missing .pending skipped (idempotent for crash recovery)
  - [x] 7.16 `TestRecoverDistillation_NoIntent` — no intent file → no-op
  - [x] 7.17 `TestRecoverDistillation_WritePhase` — completes pending renames
  - [x] 7.18 `TestRecoverDistillation_BackupPhase` — deletes .pending files (rollback)
  - [x] 7.19 `TestRecoverDistillation_CleansUp` — intent file + .pending deleted after recovery
  - [x] 7.20 Covered by unit tests: ValidateDistillation returns ErrValidationFailed (tests 7.1-7.6)
  - [x] 7.21 Covered by unit tests: ComputeDistillMetrics populates all fields (test 7.7)

## Dev Notes

### Architecture & Design Decisions

- **ValidateDistillation replaces stub** at knowledge_distill.go:531-533. Signature changes: adds `oldContent string` and `budgetLimit int` parameters.
- **3 criteria only (v6 simplification):** Budget guard, citation preservation, [needs-formatting] handling. Removed from original v3/v4: topic headers, duplicates, category count, YAML frontmatter checks — overengineered for MVP.
- **ErrValidationFailed sentinel:** Distinct from `ErrBadFormat`. BadFormat = parse failure (free retry). ValidationFailed = output is parseable but wrong (direct gate).
- **CR1 — Atomic multi-file write:** Intent file `.ralph/distill-intent.json` marks in-flight distillation. Two-phase: (1) write .pending files + intent, (2) rename .pending → target + delete intent. If crash between phases: recovery reads intent and completes or rollbacks.
- **M7 — Crash recovery:** Called at runner startup, BEFORE git RecoverDirtyState. Checks for intent file → if present, interrupted distillation → recover. No intent file = normal. .bak files without intent = previous successful distillation (normal).
- **Metrics:** Computed AFTER successful write, stored in DistillState.Metrics, logged to stderr. Trend tracking via JSON persistence.
- **DistillState.Metrics omitempty:** First distillation run has no metrics. After first distillation: Metrics populated. JSON stays clean before first distillation.
- **Version field:** Already Version:1 from Story 6.5a. Version:0 = pre-versioning (missing field). Future migrations keyed on version number.

### Current Code State (after Story 6.5b)

**ValidateDistillation stub (knowledge_distill.go:531-533):**
```go
func ValidateDistillation(_ *DistillOutput) error {
    return nil
}
```

**AutoDistill call site (knowledge_distill.go:595-597):**
```go
// Step 8: Validate (stub until 6.5c)
if err := ValidateDistillation(output); err != nil {
    return fmt.Errorf("runner: distill: validate: %w", err)
}
```
→ Must update to pass `oldContent` and `budgetLimit`.

**DistillState struct (knowledge_state.go:22-27):**
```go
type DistillState struct {
    Version              int      `json:"version"`
    MonotonicTaskCounter int      `json:"monotonic_task_counter"`
    LastDistillTask      int      `json:"last_distill_task"`
    Categories           []string `json:"categories,omitempty"`
    // ADD: Metrics *DistillMetrics `json:"metrics,omitempty"`
}
```

**Execute() startup (runner.go:450-454):**
```go
func (r *Runner) Execute(ctx context.Context) error {
    if _, err := RecoverDirtyState(ctx, r.Git); err != nil {
        return fmt.Errorf("runner: startup: %w", err)
    }
    // ADD: RecoverDistillation(r.Cfg.ProjectRoot) BEFORE RecoverDirtyState
```

**BackupDistillationFiles (knowledge_distill.go:159-179):**
Already backs up distill-state.json (line 173-176) — AC #4 partially implemented in 6.5b.

### Intent File JSON Format

```json
{
  "timestamp": "2026-03-02T14:30:00Z",
  "files": [
    "LEARNINGS.md",
    ".ralph/rules/ralph-testing.md",
    ".ralph/rules/ralph-errors.md",
    ".ralph/rules/ralph-misc.md",
    ".ralph/rules/ralph-critical.md",
    ".ralph/rules/ralph-index.md"
  ],
  "phase": "write"
}
```

### Two-Phase Write Sequence

```
Phase 1 (backup):
  BackupDistillationFiles()  ← already done in AutoDistill Step 1

Phase 2 (write):
  WriteDistillOutput() → writes .pending files (LEARNINGS.md.pending, ralph-*.md.pending)
  WriteIntentFile(phase="write", files=[...])

Phase 3 (commit):
  CommitPendingFiles(files) → rename .pending → target
  SaveDistillState()
  ComputeDistillMetrics() → state.Metrics
  SaveDistillState() again (with metrics)
  DeleteIntentFile()
```

### Crash Recovery Decision Tree

```
startup → ReadIntentFile()
  ↓ nil (not exist) → normal, no recovery
  ↓ phase="backup" → cleanup: delete any .pending files, delete intent file
  ↓ phase="write" → complete: CommitPendingFiles(intent.Files), delete intent
  ↓ phase="commit" → complete: CommitPendingFiles(intent.Files), delete intent
```

### File Layout

| File | Purpose |
|------|---------|
| `runner/knowledge_distill.go` | MODIFY: replace ValidateDistillation stub, add ErrValidationFailed, WriteIntentFile/ReadIntentFile/DeleteIntentFile, CommitPendingFiles, ComputeDistillMetrics, update AutoDistill with 2-phase write |
| `runner/knowledge_distill_test.go` | MODIFY: add validation, intent, commit, recovery, metrics tests |
| `runner/knowledge_state.go` | MODIFY: add DistillMetrics struct and Metrics field to DistillState, add RecoverDistillation |
| `runner/knowledge_state_test.go` | MODIFY: add Metrics serialization tests, Version:0 test, RecoverDistillation tests |
| `runner/runner.go` | MODIFY: add RecoverDistillation call at Execute() startup |
| `runner/runner_test.go` | MODIFY: add crash recovery integration test |

### Error Wrapping Convention

```go
fmt.Errorf("runner: distill: validate: criterion 1: budget exceeded (%d > %d lines): %w", lines, limit, ErrValidationFailed)
fmt.Errorf("runner: distill: validate: criterion 2: citation preservation %.0f%% < 80%%: %w", pct, ErrValidationFailed)
fmt.Errorf("runner: distill: validate: criterion 3: %d [needs-formatting] entries silently dropped: %w", count, ErrValidationFailed)
fmt.Errorf("runner: distill: intent: %w", err)
fmt.Errorf("runner: distill: commit: %w", err)
fmt.Errorf("runner: distill: recovery: %w", err)
```

### Dependency Direction

```
runner/knowledge_distill.go → knowledge_state.go (DistillState, DistillMetrics — same package)
runner/knowledge_state.go → (stdlib only)
runner/runner.go → knowledge_state.go (RecoverDistillation)
```

No new external packages.

### Testing Standards

- Table-driven, Go stdlib assertions, `t.TempDir()` for isolation
- ValidateDistillation: deterministic — direct input/output testing, no mocks
- Intent file: round-trip (write → read), NotExist, partial failure
- CommitPendingFiles: create .pending files in t.TempDir, verify renamed
- RecoverDistillation: create intent file + .pending files, verify recovery actions
- Metrics: verify ALL fields computed (table-driven return value completeness rule)
- `errors.Is(err, ErrValidationFailed)` for sentinel assertions (distinct from ErrBadFormat)
- Symmetric assertions: AllPass checks nil, each criterion failure checks specific error text

### Code Review Learnings

- Distinct sentinels for distinct failure modes: ErrBadFormat (parse) vs ErrValidationFailed (validation) — Story 6.5a gate logic uses `errors.Is(err, ErrBadFormat)` for free retry
- Stub replacement: update ALL call sites when signature changes (AutoDistill line 595)
- Recovery at startup BEFORE other startup steps — distillation recovery is independent of git state
- omitempty for optional struct fields — prevents cluttering JSON on first run
- No forced truncation — user decides via human gate, not code

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.5c (lines 707-789)]
- [Source: runner/knowledge_distill.go:531-533 — ValidateDistillation stub to replace]
- [Source: runner/knowledge_distill.go:594-597 — AutoDistill call site to update]
- [Source: runner/knowledge_distill.go:284-374 — WriteDistillOutput to modify for .pending writes]
- [Source: runner/knowledge_distill.go:538-610 — AutoDistill pipeline]
- [Source: runner/knowledge_state.go:22-27 — DistillState struct to extend]
- [Source: runner/runner.go:450-454 — Execute() startup for crash recovery insertion]
- [Source: runner/knowledge_write.go:47-50 — citationRegex, headerRegex for validation reuse]

## Dev Agent Record

### Context Reference
- Story 6.5b (predecessor): runner/knowledge_distill.go, runner/knowledge_state.go
- Story 6.5a: runner/runner.go runDistillation/handleDistillFailure
- knowledge_write.go: citationRegex, needsFormattingTag reused for validation

### Agent Model Used
claude-opus-4-6

### Debug Log References
- TestValidateDistillation_NeedsFormattingDropped: initial fail because criterion 2 (citation) fired before criterion 3. Fixed by ensuring test data preserves citations in new output.

### Completion Notes List
- 7 tasks completed, 19 tests written, all passing
- Full regression green across all packages
- CommitPendingFiles handles missing .pending files (idempotent) per validator note
- WriteDistillOutput now returns ([]string, error) — existing 6.5b tests updated to call CommitPendingFiles
- ErrValidationFailed distinct from ErrBadFormat — no free retry, direct gate
- RecoverDistillation wired BEFORE RecoverDirtyState in Execute()
- AutoDistill now uses 2-phase write: .pending → intent → commit → state → delete intent → index

### File List
- runner/knowledge_distill.go — MODIFIED: ValidateDistillation (3 criteria), ComputeDistillMetrics, DistillIntent, WriteIntentFile, ReadIntentFile, DeleteIntentFile, CommitPendingFiles, extractCitations, extractNeedsFormattingTopics, WriteDistillOutput returns ([]string, error) with .pending, writeCategoryFilePending/writeCriticalFilePending/writeMiscFilePending, AutoDistill 2-phase write
- runner/knowledge_distill_test.go — MODIFIED: 13 new tests (validation, metrics, intent, commit), 5 existing tests updated for new WriteDistillOutput signature
- runner/knowledge_state.go — MODIFIED: ErrValidationFailed sentinel, DistillMetrics struct, Metrics field on DistillState, RecoverDistillation function
- runner/knowledge_state_test.go — MODIFIED: 7 new tests (Metrics serialization/omitempty, Version:0, Recovery: NoIntent/WritePhase/BackupPhase/CleansUp)
- runner/runner.go — MODIFIED: RecoverDistillation call at Execute() startup before RecoverDirtyState
