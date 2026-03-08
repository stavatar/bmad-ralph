# Story 9.2: Severity Filtering + Findings Budget

Status: done

## Story

As a разработчик,
I want фильтровать findings по severity и ограничивать их количество,
so that review на поздних циклах фокусируется на критических проблемах и не генерирует новые мелкие замечания.

## Acceptance Criteria

1. **FilterBySeverity removes low findings (FR73):**
   - `FilterBySeverity(findings, SeverityMedium)` returns only findings with severity >= MEDIUM
   - LOW findings excluded, CRITICAL/HIGH/MEDIUM preserved

2. **FilterBySeverity at CRITICAL threshold (FR73):**
   - Only CRITICAL findings pass through

3. **FilterBySeverity at LOW threshold passes all (FR73):**
   - All findings preserved when threshold is LOW

4. **FilterBySeverity with empty input:**
   - Returns empty slice (no panic)

5. **TruncateFindings limits count (FR75):**
   - `TruncateFindings(findings, 3)` returns exactly 3 findings
   - Highest severity findings preserved (CRITICAL > HIGH > MEDIUM > LOW)

6. **TruncateFindings sorts by severity (FR75):**
   - Output sorted by severity descending
   - `TruncateFindings([LOW, CRITICAL, MEDIUM], 2)` returns [CRITICAL, MEDIUM]

7. **TruncateFindings when count exceeds budget:**
   - No truncation when findings count <= budget

8. **Integration in Execute cycle (FR73, FR75):**
   - `Runner.Execute()` on cycle 4 (maxIter=6): DetermineReviewOutcome returns 3 findings (CRITICAL, MEDIUM, LOW)
   - FilterBySeverity with HIGH threshold removes MEDIUM and LOW
   - TruncateFindings with budget=1 keeps only CRITICAL
   - Only CRITICAL finding written to review-findings.md for next execute

## Tasks / Subtasks

- [x] Task 1: Implement `FilterBySeverity` in `runner/progressive.go` (AC: #1-#4)
  - [x] Use `ParseSeverity` to compare finding.Severity with threshold
  - [x] Return new slice (not modify input)
  - [x] Handle empty input gracefully
- [x] Task 2: Implement `TruncateFindings` in `runner/progressive.go` (AC: #5-#7)
  - [x] Sort by severity descending using `sort.SliceStable`
  - [x] Truncate to maxCount
  - [x] Return unchanged slice if len <= maxCount
- [x] Task 3: Integrate filtering in `runner/runner.go` Execute() loop (AC: #8)
  - [x] After ReviewFn call, apply `FilterBySeverity` and `TruncateFindings`
  - [x] Use params from `ProgressiveParams(cycle, maxIter)`
  - [x] Log filtered findings: `INFO finding below threshold: [SEVERITY] description`
  - [x] Write only filtered+truncated findings to review-findings.md via `writeFilteredFindings`
- [x] Task 4: Write comprehensive tests in `runner/progressive_test.go` (AC: #1-#7)
  - [x] Table-driven tests for FilterBySeverity: all threshold levels + empty input + DoesNotModifyInput
  - [x] Table-driven tests for TruncateFindings: SortBySeverity, NoTruncation, ExactBudget, Empty, BudgetOne
  - [x] Verify ALL fields of returned findings (Severity, Description, File, Line)
  - [x] writeFilteredFindings tests: Format (content verification) + Empty
- [x] Task 5: Write integration test for Execute cycle with filtering (AC: #8)
  - [x] Mock ReviewFn returns CRITICAL+MEDIUM+LOW findings with maxReviewIterations=1
  - [x] Verify only CRITICAL finding written to review-findings.md

## Dev Notes

### Architecture & Design

- **File:** `runner/progressive.go` — extends the file created in Story 9.1
- **Integration point:** `runner/runner.go` Execute() loop — after DetermineReviewOutcome
- **Decision:** Filtering in caller (`Execute`), NOT in `DetermineReviewOutcome` — keeps parsing clean
- **No new dependencies** — uses `sort` from stdlib

### Critical Implementation Details

`FilterBySeverity` uses `ParseSeverity` (from Story 9.1) to convert `finding.Severity` string to `SeverityLevel` for comparison:
```go
func FilterBySeverity(findings []ReviewFinding, minSeverity SeverityLevel) []ReviewFinding {
    var result []ReviewFinding
    for _, f := range findings {
        if ParseSeverity(f.Severity) >= minSeverity {
            result = append(result, f)
        }
    }
    return result
}
```

`TruncateFindings` sorts by severity descending, then truncates:
```go
func TruncateFindings(findings []ReviewFinding, maxCount int) []ReviewFinding {
    if len(findings) <= maxCount {
        return findings
    }
    sort.SliceStable(findings, func(i, j int) bool {
        return ParseSeverity(findings[i].Severity) > ParseSeverity(findings[j].Severity)
    })
    return findings[:maxCount]
}
```

### Integration Point in Execute()

Current flow in `runner/runner.go` Execute():
```
reviewResult := ReviewFn(...)
findings := reviewResult.Findings
// ... write findings to review-findings.md
```

New flow:
```
reviewResult := ReviewFn(...)
findings := reviewResult.Findings
params := ProgressiveParams(cycle, maxIter)
filtered := FilterBySeverity(findings, params.MinSeverity)
truncated := TruncateFindings(filtered, params.MaxFindings)
// ... write truncated to review-findings.md
```

### Dependencies

- **Depends on Story 9.1:** SeverityLevel type, ParseSeverity function, ProgressiveParams, ProgressiveReviewParams
- **Type dependency:** Uses `ReviewFinding` struct from `runner/metrics.go` (existing, has Severity string field)

### Testing Standards

- Table-driven tests, Go stdlib assertions (no testify)
- Test naming: `TestFilterBySeverity_MediumThreshold`, `TestTruncateFindings_SortBySeverity`, etc.
- Edge case tests MUST verify ALL struct fields of returned findings
- Integration test verifies review-findings.md file content after filtering

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.2]
- [Source: docs/architecture/ralph-run-robustness.md#Область 3 — FilterBySeverity, TruncateFindings]
- [Source: docs/prd/ralph-run-robustness.md#FR73, FR75]
- [Source: runner/metrics.go:20-25 — ReviewFinding struct with Severity string]
- [Source: runner/runner.go — Execute() loop, DetermineReviewOutcome]

## Dev Agent Record

### Context Reference

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: FilterBySeverity — uses ParseSeverity for comparison, returns new slice, nil/empty safe
- Task 2: TruncateFindings — sort.SliceStable descending by severity, truncate to maxCount, no-op when under budget; copy-before-sort (review fix M1)
- Task 3: Integration in Execute() — applies ProgressiveParams after ReviewFn returns, logs below-threshold findings, rewrites review-findings.md with writeFilteredFindings, updates rr.Findings
- Task 4: 9+2 test functions covering all ACs: FilterBySeverity (AllThresholds 4 cases, EmptyInput, DoesNotModifyInput), TruncateFindings (SortBySeverity, NoTruncation, ExactBudget, Empty, BudgetOne, DoesNotModifyInput, MaxCountZero), writeFilteredFindings (Format, Empty)
- Task 5: Integration test TestRunner_Execute_FindingsFiltered — maxReviewIterations=1 triggers CRITICAL threshold, verifies review-findings.md contains only CRITICAL finding
- All tests pass, no regressions across all packages
- Review fix H1: runner.go commit included pre-existing scaffold code (selectReviewModel gate/hydra, checkTaskBudget, RunConfig.IsGate/HydraDetected) from DESIGN-4/Story 9.3 prep — staging scope issue, not a code bug. These features are now covered by their respective stories (9.3 committed as ff19112)
- Review fix M2: writeFilteredFindings doc comment explains intentional File/Line omission
- Review fix L1: integration test comment clarifies AC#8 mapping

### Change Log

- 2026-03-06: Implemented Story 9.2 — FilterBySeverity, TruncateFindings, writeFilteredFindings, Execute() integration, comprehensive tests

### File List

- runner/progressive.go (modified — added FilterBySeverity, TruncateFindings, writeFilteredFindings)
- runner/progressive_test.go (modified — added 9 test functions)
- runner/runner.go (modified — integrated filtering after ReviewFn)
- runner/runner_test.go (modified — added TestRunner_Execute_FindingsFiltered)
