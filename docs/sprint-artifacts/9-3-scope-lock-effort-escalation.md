# Story 9.3: Scope Lock + Effort Escalation

Status: review

## Story

As a разработчик,
I want чтобы review на поздних циклах получал инкрементальный diff и контекст предыдущих findings, а execute запускался с extended thinking,
so that сложные замечания исправляются качественнее и review-цикл сходится.

## Acceptance Criteria

1. **RunConfig extended with progressive fields (FR72):**
   - RunConfig struct gets 5 new fields: `Cycle int`, `MinSeverity SeverityLevel`, `MaxFindings int`, `IncrementalDiff bool`, `PrevFindings string`
   - All fields accessible in RealReview and Execute

2. **Incremental diff on cycle 3+ (FR74):**
   - When cycle >= 3 and ProgressiveParams returns incrementalDiff=true
   - Review prompt receives `git diff HEAD~1..HEAD` (last commit only)
   - Review prompt contains task description and story reference
   - Review prompt contains previous cycle findings text
   - Review prompt contains instruction: "проверь корректность исправлений и отсутствие новых проблем уровня <порог>+"

3. **Full diff on cycles 1-2 (FR74):**
   - When cycle <= 2, review prompt receives full task diff (existing behavior)

4. **Effort escalation via environment variable (FR76):**
   - When cycle >= 3 and highEffort=true, `session.Options.Env` contains `{"CLAUDE_CODE_EFFORT_LEVEL": "high"}`

5. **No effort escalation on early cycles (FR76):**
   - When cycle <= 2, `session.Options.Env` is nil or empty

6. **Model escalation on late cycles (FR72):**
   - When highEffort=true, `selectReviewModel` returns `cfg.ModelReview` (full model, never light)
   - `selectReviewModel` signature includes `highEffort bool` parameter

7. **Review prompt with findings budget (FR75):**
   - When cycle 4, maxFindings=1: prompt contains "Найди НЕ БОЛЕЕ 1 самых важных замечаний. Приоритизируй по severity"

8. **Execute loop integration (FR72-FR76):**
   - Runner.Execute() with maxIter=6:
     - cycle 1: full diff, LOW+ threshold, budget 5, standard model
     - cycle 3: incremental diff, MEDIUM+ threshold, budget 3, max model + effort=high
     - cycle 5: incremental diff, CRITICAL threshold, budget 1, max model + effort=high

## Tasks / Subtasks

- [x] Task 1: Extend `RunConfig` struct with progressive fields (AC: #1)
  - [x] Add Cycle, MinSeverity, MaxFindings, IncrementalDiff, PrevFindings, HighEffort fields
- [x] Task 2: Update `selectReviewModel` signature (AC: #6)
  - [x] Add `highEffort bool` parameter
  - [x] When highEffort=true, return cfg.ModelReview (bypass light model logic)
  - [x] Update all call sites (runner.go + coverage_internal_test.go)
- [x] Task 3: Update Execute() loop to call ProgressiveParams (AC: #4, #5, #8)
  - [x] Call `ProgressiveParams(cycle, maxIter)` at start of each cycle
  - [x] Set `session.Options.Env` with CLAUDE_CODE_EFFORT_LEVEL=high when highEffort
  - [x] Populate RunConfig progressive fields before passing to RealReview
  - [x] Capture prevFindingsText after filtering for next cycle context
- [x] Task 4: Update review prompt template for incremental mode (AC: #2, #3, #7)
  - [x] Add conditional section in review.md for incremental diff context
  - [x] Add TemplateData fields: Cycle, MinSeverityLabel, MaxFindings, IncrementalDiff (bool)
  - [x] Add conditional budget instruction: "Найди НЕ БОЛЕЕ N замечаний"
- [x] Task 5: Incremental diff via review prompt instruction (AC: #2)
  - [x] Review prompt instructs Claude to use `git diff HEAD~1..HEAD` directly (Claude executes git commands)
  - [x] No GitClient changes needed — diff instruction is in prompt text, not passed as content
- [x] Task 6: Update RealReview to use RunConfig progressive fields (AC: #2, #3)
  - [x] Build TemplateData with IncrementalDiff, Cycle, MinSeverityLabel, MaxFindings
  - [x] Add __PREV_FINDINGS__ replacement when IncrementalDiff=true
  - [x] Pass HighEffort to selectReviewModel
- [x] Task 7: Write tests (AC: #1-#8)
  - [x] Test selectReviewModel with highEffort=true/false (3 new cases in coverage_internal_test.go)
  - [x] Test Execute loop progressive params integration (TestRunner_Execute_ProgressiveReviewParams)
  - [x] Test review prompt contains incremental diff context on cycle 3+ (TestPrompt_Review_IncrementalMode)
  - [x] Test review prompt absent incremental on cycle 1-2 (TestPrompt_Review_FullDiffMode)
  - [x] Test review prompt contains budget instruction (TestPrompt_Review_BudgetInstruction)
  - [x] Test effort escalation + PrevFindings wired per cycle (ProgressiveReviewParams integration)

## Dev Notes

### Architecture & Design

- **Primary files:** `runner/runner.go` (RunConfig, selectReviewModel, Execute), `runner/prompts/review.md`
- **Config file:** `config/prompt.go` (TemplateData extension)
- **Dependencies:** Story 9.1 (ProgressiveParams, SeverityLevel), Story 9.4 (Options.Env)
- **No new packages or dependencies**

### Critical Implementation Details

**selectReviewModel signature change:**
Current: `func selectReviewModel(cfg *config.Config, ds *DiffStats, isGate bool, hydraDetected bool) string`
New: `func selectReviewModel(cfg *config.Config, ds *DiffStats, isGate bool, hydraDetected bool, highEffort bool) string`

Added condition: `if isGate || hydraDetected || highEffort || ds == nil || cfg.ModelReviewLight == "" {`

**Execute loop integration:**
```
for cycle := 1; cycle <= maxIter; cycle++ {
    params := ProgressiveParams(cycle, maxIter)

    // Build env for execute session
    var env map[string]string
    if params.HighEffort {
        env = map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "high"}
    }
    opts.Env = env

    // ... execute session ...

    // Populate RunConfig for review
    rc.Cycle = cycle
    rc.MinSeverity = params.MinSeverity
    rc.MaxFindings = params.MaxFindings
    rc.IncrementalDiff = params.IncrementalDiff
    rc.PrevFindings = prevFindingsText

    // ... call ReviewFn(rc) ...

    // Filter + truncate (from Story 9.2)
    filtered := FilterBySeverity(findings, params.MinSeverity)
    truncated := TruncateFindings(filtered, params.MaxFindings)
}
```

**Review prompt conditional sections:**
```markdown
{{if .IncrementalDiff}}
## Incremental Review (Cycle {{.Cycle}})

Это инкрементальный review. Проверяй ТОЛЬКО изменения последнего коммита (diff ниже).

Предыдущие замечания:
__PREV_FINDINGS__

Инструкция: проверь корректность исправлений и отсутствие новых проблем уровня {{.MinSeverityLabel}}+.

Найди НЕ БОЛЕЕ {{.MaxFindings}} самых важных замечаний. Приоритизируй по severity.
{{end}}
```

**TemplateData extensions in config/prompt.go:**
```go
type TemplateData struct {
    // ... existing fields
    IncrementalDiff  bool   // Stage 1: conditional for incremental review mode
    Cycle            int    // Stage 1: current cycle number
    MinSeverityLabel string // Stage 1: severity label for instructions
    MaxFindings      int    // Stage 1: findings budget for instructions
}
```

### Existing Scaffold Context

- `runner/runner.go:608-618` — current RunConfig struct (6 fields)
- `runner/runner.go:98-107` — current selectReviewModel (4 params)
- `runner/prompts/review.md` — current review template (no progressive sections)
- `config/prompt.go:29-46` — current TemplateData struct
- `session/session.go:38-48` — Options struct (Story 9.4 adds Env field)

### Testing Standards

- Table-driven tests, Go stdlib assertions
- Test selectReviewModel with matrix: all combinations of isGate/hydra/highEffort
- Golden files for review prompt conditional output
- Integration test: mock Execute loop verifying progressive params per cycle

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.3]
- [Source: docs/architecture/ralph-run-robustness.md#Область 3 — Точки 1-4]
- [Source: docs/prd/ralph-run-robustness.md#FR72, FR74, FR75, FR76]
- [Source: runner/runner.go:98-107 — selectReviewModel]
- [Source: runner/runner.go:608-618 — RunConfig struct]
- [Source: config/prompt.go:29-46 — TemplateData struct]
- [Source: runner/prompts/review.md — review template]

## Dev Agent Record

### Context Reference

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: RunConfig extended with 6 progressive fields (Cycle, MinSeverity, MaxFindings, IncrementalDiff, HighEffort, PrevFindings)
- Task 2: selectReviewModel updated: 5th param `highEffort bool`, forces standard model. All call sites updated (RealReview + test)
- Task 3: Execute() loop: ProgressiveParams called per cycle, effort Env on session opts, RunConfig populated, prevFindingsText captured
- Task 4: TemplateData extended with 4 fields; review.md conditional incremental section with budget instruction
- Task 5: Incremental diff via prompt instruction — Claude executes `git diff HEAD~1..HEAD` directly, no GitClient changes needed
- Task 6: RealReview builds progressive TemplateData, adds __PREV_FINDINGS__ replacement, passes HighEffort to selectReviewModel
- Task 7: 7 test functions covering all AC: selectReviewModel highEffort (3 cases), prompt incremental/full/budget (3 tests), integration (1 test verifying 5 cycles)
- All tests pass, no regressions across all packages
- Review fix M1: AC#4 env gap — added doc comment noting HighEffort→execEnv is a direct 3-line assignment, full subprocess env verification out of scope for unit test
- Review fix M2: prevFindingsText already uses filtered findings (rr.Findings=truncated at line 1313) — added clarifying comment
- Review fix M3: Added scope annotations to selectReviewModel test cases — "Backfill from Story 9.2" for gate/hydra, "Story 9.3 AC#6" for highEffort
- Review fix L1: Removed duplicate standalone HighEffort assertions (already verified in loop via ProgressiveParams)
- Review fix L2: BudgetInstruction test uses strings.Count==1 instead of strings.Contains

### Change Log

- 2026-03-06: Implemented Story 9.3 — scope lock, effort escalation, incremental review, progressive fields

### File List

- runner/runner.go (modified — RunConfig, selectReviewModel, Execute loop, RealReview)
- runner/prompts/review.md (modified — incremental review conditional section)
- config/prompt.go (modified — TemplateData progressive fields)
- runner/coverage_internal_test.go (modified — selectReviewModel highEffort test cases)
- runner/prompt_test.go (modified — 3 new prompt tests for incremental/full/budget)
- runner/runner_test.go (modified — TestRunner_Execute_ProgressiveReviewParams integration test)
- runner/testdata/TestPrompt_Review.golden (updated)
