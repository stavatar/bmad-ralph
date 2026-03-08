# Story 9.5: Filepath Normalization + Graceful Degradation

Status: done

## Story

As a разработчик,
I want чтобы Ralph корректно работал с путями на Windows, WSL и Linux, а некритические файловые операции не прерывали ран при ошибке,
so that Ralph стабилен на всех платформах и resilient к отсутствующим файлам.

## Acceptance Criteria

1. **filepath.Join в knowledge_distill.go (FR70):**
   - All path concatenations use `filepath.Join` (no string + "/")
   - `filepath.Abs` used for normalizing input paths where needed

2. **filepath.Join в knowledge_write.go (FR70):**
   - All path concatenations use `filepath.Join`
   - No manual "/" or "\\" separators in path building

3. **filepath.Join в runner.go (FR70):**
   - All remaining string concatenation paths use `filepath.Join`
   - `DetermineReviewOutcome` already uses `filepath.Join` (verify preserved)

4. **Graceful degradation for AutoDistill (FR71):**
   - When AutoDistill encounters `os.ErrNotExist` reading LEARNINGS.md
   - Function returns nil (graceful skip)
   - Warning logged: "WARN: <path> not found, skipping"
   - Run continues without abort

5. **Graceful degradation for WriteLearnings (FR71):**
   - When WriteLearnings encounters `os.ErrNotExist`
   - Function returns nil (graceful skip)
   - Warning logged

6. **Real errors still propagated (FR71):**
   - When AutoDistill encounters permission error (not `os.ErrNotExist`)
   - Error propagated to caller (not swallowed)
   - Non-NotExist errors are NOT gracefully skipped

7. **Cross-platform path test:**
   - `filepath.Join(projectRoot, "review-findings.md")` produces platform-correct path

## Tasks / Subtasks

- [x] Task 1: Audit and fix path construction in `runner/knowledge_distill.go` (AC: #1)
  - [x] Replace any string concatenation paths with filepath.Join
  - [x] Add filepath.Abs where normalizing user-provided paths
- [x] Task 2: Audit and fix path construction in `runner/knowledge_write.go` (AC: #2)
  - [x] Replace any string concatenation paths with filepath.Join
- [x] Task 3: Audit and fix path construction in `runner/runner.go` (AC: #3)
  - [x] Verify DetermineReviewOutcome already uses filepath.Join
  - [x] Fix any remaining string concatenation paths
- [x] Task 4: Add graceful degradation guard in AutoDistill (AC: #4, #6)
  - [x] Add `errors.Is(err, os.ErrNotExist)` check at LEARNINGS.md read point
  - [x] Return nil for NotExist, propagate real errors
  - [x] Log WARN for skip
- [x] Task 5: Add graceful degradation guard in WriteLearnings (AC: #5, #6)
  - [x] Add `errors.Is(err, os.ErrNotExist)` check at relevant read points
  - [x] Return nil for NotExist, propagate real errors
- [x] Task 6: Write tests (AC: #1-#7)
  - [x] Test filepath.Join produces correct paths
  - [x] Test AutoDistill graceful skip on missing LEARNINGS.md
  - [x] Test AutoDistill propagates permission errors
  - [x] Test WriteLearnings graceful skip on missing file
  - [x] Test cross-platform path format

## Dev Notes

### Architecture & Design

- **Files:** `runner/knowledge_distill.go`, `runner/knowledge_write.go`, `runner/runner.go`
- **Nature:** Refactoring existing code — no new types or files
- **No new dependencies** — uses `filepath` from stdlib (already imported)
- **No cross-package impact** — all changes within runner/

### Current State Analysis

Based on codebase grep, runner/*.go already uses `filepath.Join` extensively. The task is to:
1. **Audit** all path construction for any remaining string concatenation
2. **Add graceful degradation** guards where not yet present

Key existing patterns:
- `knowledge_distill.go:136` — already has `os.ErrNotExist` check for `os.Stat`
- `knowledge_distill.go:625` — already has `os.ErrNotExist` check for ReadFile
- `knowledge_write.go:87,436` — already has `os.IsNotExist` checks
- `knowledge_read.go:131` — already has `os.IsNotExist` check

**Important:** Many guards already exist. The story is about:
- Ensuring COMPLETENESS — all code paths have guards
- Ensuring CONSISTENCY — same pattern everywhere (return nil for NotExist, propagate others)
- Ensuring log messages use "WARN:" prefix for skips

### Graceful Degradation Pattern

```go
if err != nil {
    if errors.Is(err, os.ErrNotExist) {
        log.Printf("WARN: %s not found, skipping", path)
        return nil
    }
    return fmt.Errorf("runner: op: %w", err)
}
```

Use `errors.Is(err, os.ErrNotExist)` (not `os.IsNotExist`) per project convention (`errors.As`/`errors.Is` only).

### Non-critical Operations List

Per architecture doc, these are best-effort:
- AutoDistill — LEARNINGS.md distillation
- WriteLearnings — writing knowledge
- WriteRules — writing rules
- Serena detection — reading `.claude/settings.json`

### Testing Standards

- Table-driven tests, Go stdlib assertions
- Platform-agnostic error assertions (WSL/Windows produces different error messages)
- Use `t.TempDir()` for isolation
- Test both os.ErrNotExist path AND non-NotExist error path

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.5]
- [Source: docs/architecture/ralph-run-robustness.md#Область 2]
- [Source: docs/prd/ralph-run-robustness.md#FR70, FR71]
- [Source: runner/knowledge_distill.go — existing os.ErrNotExist guards]
- [Source: runner/knowledge_write.go — existing os.IsNotExist guards]

## Dev Agent Record

### Context Reference

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: knowledge_distill.go — all paths already use filepath.Join, all checks already use errors.Is(err, os.ErrNotExist). No changes needed.
- Task 2: knowledge_write.go — paths already use filepath.Join. Fixed os.IsNotExist→errors.Is on lines 87,436. Added log import. Added WARN log on ValidateNewLessons NotExist path.
- Task 3: runner.go — all paths already use filepath.Join, all checks already use errors.Is. No changes needed. DetermineReviewOutcome confirmed using filepath.Join.
- Task 3b: knowledge_read.go — fixed os.IsNotExist→errors.Is on line 131. Added errors import. (Out of AC scope but improves consistency — single-line fix, no risk.)
- Task 3c: progressive_test.go — fixed string concatenation dir+"/review-findings.md"→filepath.Join(dir,"review-findings.md") on lines 390,417. Added path/filepath import.
- Task 4: AutoDistill — added errors.Is(err, os.ErrNotExist) guard with log.Printf WARN and return nil. Added log import. Updated existing test from error-expectation to graceful-skip-expectation.
- Task 5: ValidateNewLessons already had NotExist guard (returns nil). Added WARN log.Printf for consistency with AC#5 pattern. BudgetCheck NotExist guard already existed (returns zero BudgetStatus).
- Task 6: Tests — TestAutoDistill_MissingLearningsGracefulSkip (AC#4), TestAutoDistill_ReadLearningsRealError (AC#6), TestFilepathJoin_CrossPlatform (AC#7). Existing tests cover: ValidateNewLessons missing file (AC#5), ValidateNewLessons read error (AC#6).

### Change Log

- 2026-03-06: Implemented Story 9.5 — filepath normalization audit, os.IsNotExist→errors.Is consistency, AutoDistill graceful degradation, WARN logging, cross-platform path tests

### File List

- runner/knowledge_distill.go (modified — added graceful degradation in AutoDistill, added log import)
- runner/knowledge_write.go (modified — os.IsNotExist→errors.Is, added WARN log, added errors+log imports)
- runner/knowledge_read.go (modified — os.IsNotExist→errors.Is, added errors import)
- runner/progressive_test.go (modified — string concat→filepath.Join, added path/filepath import)
- runner/coverage_external_test.go (modified — renamed+updated AutoDistill missing file test, added real error test)
- runner/knowledge_write_test.go (modified — added TestFilepathJoin_CrossPlatform)
