# Story 9.1: Progressive Review Types + Config Default

Status: Ready for Review

## Story

As a разработчик,
I want иметь типы и функции для прогрессивной схемы review,
so that параметры review (severity порог, бюджет замечаний, scope, effort) определяются номером цикла.

## Acceptance Criteria

1. **SeverityLevel type and constants (FR72):**
   - `runner/progressive.go` defines `SeverityLevel` type (int)
   - Constants via iota: `SeverityLow = 0`, `SeverityMedium = 1`, `SeverityHigh = 2`, `SeverityCritical = 3`
   - All 4 severity levels have correct ordering (Low < Medium < High < Critical)

2. **ParseSeverity converts strings (FR73):**
   - "LOW" -> SeverityLow, "MEDIUM" -> SeverityMedium, "HIGH" -> SeverityHigh, "CRITICAL" -> SeverityCritical
   - Case-insensitive: "low" -> SeverityLow
   - Empty string or unknown -> SeverityLow (fallback)

3. **ProgressiveParams for default 6 cycles (FR72):**
   - `ProgressiveParams(cycle, 6)` returns `ProgressiveReviewParams` struct for each cycle:
     - cycle 1: minSeverity=LOW, maxFindings=5, incrementalDiff=false, highEffort=false
     - cycle 2: minSeverity=LOW, maxFindings=5, incrementalDiff=false, highEffort=false
     - cycle 3: minSeverity=MEDIUM, maxFindings=3, incrementalDiff=true, highEffort=true
     - cycle 4: minSeverity=HIGH, maxFindings=1, incrementalDiff=true, highEffort=true
     - cycle 5: minSeverity=CRITICAL, maxFindings=1, incrementalDiff=true, highEffort=true
     - cycle 6: minSeverity=CRITICAL, maxFindings=1, incrementalDiff=true, highEffort=true

4. **ProgressiveParams scales for non-6 maxCycles (FR72):**
   - maxCycles=3: cycle 1 -> LOW/5/false/false, cycle 2 -> MEDIUM/3/true/true, cycle 3 -> CRITICAL/1/true/true
   - Scaling: first ~33% = LOW, ~50% = MEDIUM, ~67% = HIGH, rest = CRITICAL

5. **ProgressiveParams edge cases:**
   - ProgressiveParams(0, 6) -> same as cycle 1 (clamped)
   - ProgressiveParams(7, 6) -> same as cycle 6 (clamped)
   - ProgressiveParams(1, 1) -> minSeverity=CRITICAL, maxFindings=1

6. **Config default change (FR72):**
   - `config/defaults.yaml`: `max_review_iterations: 6` (was 3)
   - `Config.Load()` with empty config -> `MaxReviewIterations == 6`

## Tasks / Subtasks

- [x] Task 1: Create `runner/progressive.go` with SeverityLevel type and constants (AC: #1)
  - [x] Define `SeverityLevel` as `type SeverityLevel int`
  - [x] Define iota constants: SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical
- [x] Task 2: Implement `ParseSeverity` function (AC: #2)
  - [x] Case-insensitive `strings.ToUpper` switch
  - [x] Default fallback to SeverityLow for unknown/empty input
- [x] Task 3: Define `ProgressiveReviewParams` struct (AC: #3)
  - [x] Fields: MinSeverity SeverityLevel, MaxFindings int, IncrementalDiff bool, HighEffort bool
- [x] Task 4: Implement `ProgressiveParams` function (AC: #3, #4, #5)
  - [x] Clamp cycle to [1, maxCycles] range
  - [x] Calculate proportional thresholds based on cycle position
  - [x] Return correct params for all cycle positions
- [x] Task 5: Update `config/defaults.yaml` (AC: #6)
  - [x] Change `max_review_iterations: 3` to `max_review_iterations: 6`
- [x] Task 6: Write comprehensive tests in `runner/progressive_test.go` (AC: #1-#5)
  - [x] Table-driven tests for ParseSeverity (all named values + case-insensitive + unknown + empty)
  - [x] Table-driven tests for ProgressiveParams with maxCycles=6 (all 6 cycles)
  - [x] Table-driven tests for ProgressiveParams with maxCycles=3
  - [x] Edge case tests: cycle=0, cycle>maxCycles, maxCycles=1
  - [x] SeverityLevel ordering test: Low < Medium < High < Critical
- [x] Task 7: Update config test for new default (AC: #6)
  - [x] Verify `Config.Load()` returns MaxReviewIterations=6 with empty config

## Dev Notes

### Architecture & Design

- **New file:** `runner/progressive.go` — contains all progressive review types and functions
- **Existing file modification:** `config/defaults.yaml` — single line change
- **No new dependencies** — uses only Go stdlib (`strings`)
- **Package:** `runner` — types will be consumed by runner.go Execute() loop (Story 9.2/9.3)
- **No cross-package impact** — SeverityLevel and functions are internal to runner for now

### Implementation Details

- `ProgressiveReviewParams` is a plain struct (not method receiver), returned by `ProgressiveParams`
- Scaling algorithm: compute `position = float64(cycle-1) / float64(maxCycles-1)` for maxCycles > 1
  - position < 0.33 → LOW
  - position < 0.50 → MEDIUM
  - position < 0.67 → HIGH
  - position >= 0.67 → CRITICAL
- For maxCycles=1: always return CRITICAL/1/true/true (single cycle = strictest)
- `ParseSeverity` uses `strings.ToUpper` + switch, not map lookup (4 cases is clearer)

### Existing Scaffold Context

- `runner/metrics.go:20-25` — `ReviewFinding` struct already has `Severity string` field. Story 9.1 creates typed `SeverityLevel` for progressive params, NOT for ReviewFinding (that's Story 9.9)
- `config/defaults.yaml:4` — current `max_review_iterations: 3`
- `config/config.go` — `MaxReviewIterations int` field in Config struct, populated by defaults cascade
- `runner/runner.go` — `selectReviewModel` exists with `isGate, hydraDetected bool` params (will be extended in Story 9.3)

### Testing Standards

- Table-driven tests, Go stdlib assertions (no testify)
- Test naming: `TestSeverityLevel_Ordering`, `TestParseSeverity_AllValues`, `TestProgressiveParams_DefaultSixCycles`, etc.
- Golden files NOT needed (no template output)
- Coverage target: >80% for runner package
- Error tests: ParseSeverity has no error return (fallback to SeverityLow), so no error path tests needed
- Edge case tests MUST verify ALL struct fields of ProgressiveReviewParams, not just one

### Project Structure Notes

- `runner/progressive.go` follows existing pattern of domain files in runner: `runner/metrics.go`, `runner/similarity.go`, `runner/git.go`
- Constants use iota pattern consistent with Go conventions
- No impact on dependency graph — `runner` package, no new imports beyond stdlib

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.1]
- [Source: docs/architecture/ralph-run-robustness.md#Область 3]
- [Source: docs/prd/ralph-run-robustness.md#FR72, FR73]
- [Source: config/defaults.yaml — current max_review_iterations: 3]
- [Source: runner/metrics.go:20-25 — existing ReviewFinding struct]
- [Source: docs/project-context.md#Naming Conventions — error/type naming patterns]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1-4: Created `runner/progressive.go` with SeverityLevel type (iota: Low=0, Medium=1, High=2, Critical=3), ParseSeverity function (case-insensitive, fallback to Low), ProgressiveReviewParams struct, and ProgressiveParams function with proportional scaling and clamping
- Task 5: Updated `config/defaults.yaml` max_review_iterations from 3 to 6
- Task 6: Comprehensive table-driven tests in `runner/progressive_test.go`: ordering test, value test, ParseSeverity (15 cases), ProgressiveParams for 6 cycles, 3 cycles, and edge cases (5 cases)
- Task 7: Updated existing config default test to expect MaxReviewIterations=6
- All tests pass, no regressions

### Change Log

- 2026-03-06: Implemented Story 9.1 — progressive review types, config default change, comprehensive tests

### File List

- runner/progressive.go (new)
- runner/progressive_test.go (new)
- config/defaults.yaml (modified)
- config/config_test.go (modified)
