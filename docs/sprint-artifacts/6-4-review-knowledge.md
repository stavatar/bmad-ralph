# Story 6.4: Review Knowledge

Status: ready-for-review

## Story

As a review сессия с findings,
I want записывать уроки (типы ошибок, упускаемые паттерны) в LEARNINGS.md,
so that будущие execute сессии не повторяли те же ошибки.

## Acceptance Criteria

```gherkin
Scenario: Review with findings writes lessons via Claude (C1)
  Given review found CONFIRMED findings
  When Claude in review session processes findings (FR28a)
  Then Claude writes lessons to LEARNINGS.md via file tools
  And lessons include: error types, what agent forgets, patterns for future sessions
  And entries formatted as atomized facts with source citations

Scenario: Go post-validates review-written lessons (C1)
  Given Claude wrote lessons during review
  When review session ends
  Then Go diffs LEARNINGS.md (snapshot vs current)
  And validates new entries via FileKnowledgeWriter
  And invalid entries tagged [needs-formatting]

Scenario: Clean review does NOT write lessons
  Given review is clean (no findings)
  When review session completes
  Then no new content added to LEARNINGS.md
  And no knowledge files modified (beyond [x] + clear findings)

Scenario: Review prompt updated with knowledge instructions (M2)
  Given review prompt from Story 4.1
  When Epic 6 integration
  Then prompt includes: write lessons to LEARNINGS.md on findings
  And prompt includes: do NOT write lessons on clean review
  And prompt includes: atomized fact format specification
  And existing "MUST NOT write LEARNINGS.md" invariant REMOVED from review prompt
    (was at runner/prompts/review.md ~line 127, now replaced with knowledge write instructions)
  And prompt invariants documentation updated (M2)

Scenario: FR17 lessons scope now implemented
  Given FR17 lessons deferred from Epic 4
  When Epic 6 review knowledge active
  Then review writes lessons on findings (previously deferred)
  And review writes [x] + clears findings on clean (unchanged from Epic 4)
```

## Tasks / Subtasks

- [x] Task 1: Update review.md prompt with knowledge write instructions (AC: #4, #1)
  - [x] 1.1 Add "Knowledge Extraction" section to `runner/prompts/review.md` — AFTER Findings Write, BEFORE Prompt Invariants
  - [x] 1.2 Instructions: "When CONFIRMED findings exist, write lessons to LEARNINGS.md"
  - [x] 1.3 Instructions: "Each lesson as atomized fact: `## category: topic [source, file:line]\nContent.\n`"
  - [x] 1.4 Instructions: "Include: error type, what agent forgets/misses, pattern for future sessions"
  - [x] 1.5 Instructions: "Do NOT write lessons on clean review (no findings = no lessons)"
  - [x] 1.6 LEARNINGS.md format specification section

- [x] Task 2: Remove stale LEARNINGS.md invariant (AC: #4, M2)
  - [x] 2.1 **REMOVE** "MUST NOT write LEARNINGS.md or CLAUDE.md" from review.md Prompt Invariants (~line 127)
  - [x] 2.2 **REPLACE** with: "review sessions MAY write to LEARNINGS.md for knowledge extraction (FR28a)"
  - [x] 2.3 Verify execute.md does NOT have conflicting invariant about LEARNINGS.md
  - [x] 2.4 Update golden file: `runner/testdata/TestPrompt_Review.golden`

- [x] Task 3: Wire snapshot-diff into RealReview (AC: #2)
  - [x] 3.1 In `RealReview` (runner/runner.go:85): before session, snapshot LEARNINGS.md content
  - [x] 3.2 After session ends (after file-state check): call `kw.ValidateNewLessons(ctx, data)`
  - [x] 3.3 Note: RealReview doesn't currently have KnowledgeWriter access — need to add it
  - [x] 3.4 Option A: Add KnowledgeWriter to RunConfig struct
  - [x] 3.5 Option B: Add KnowledgeWriter parameter to ReviewFunc signature
  - [x] 3.6 Set `LessonsData.Source = "review"`, `LessonsData.BudgetLimit = cfg.LearningsBudget`
  - [x] 3.7 Missing LEARNINGS.md before session → empty snapshot (all entries new)

- [x] Task 4: Handle clean review — no knowledge write (AC: #3)
  - [x] 4.1 When clean review detected (no findings, task marked [x]): skip ValidateNewLessons
  - [x] 4.2 LEARNINGS.md unchanged on clean review
  - [x] 4.3 No knowledge files modified (only [x] + clear findings per existing Epic 4 logic)

- [x] Task 5: Tests (AC: all)
  - [x] 5.1 `TestPrompt_Review_KnowledgeInstructions` — review prompt contains knowledge write instructions (merged into TestPrompt_Review table)
  - [x] 5.2 `TestPrompt_Review_InvariantRemoved` — "MUST NOT write LEARNINGS.md" absent (TestPrompt_Review_InvariantUpdated)
  - [x] 5.3 `TestPrompt_Review_InvariantReplaced` — "MAY write to LEARNINGS.md" present (TestPrompt_Review_InvariantUpdated)
  - [x] 5.4 `TestPrompt_Review_FormatSpecification` — atomized fact format in prompt (merged into TestPrompt_Review table)
  - [x] 5.5 `TestRealReview_FindingsWriteLessons` — ValidateNewLessons called after findings review
  - [x] 5.6 `TestRealReview_CleanNoLessons` — ValidateNewLessons NOT called on clean review
  - [x] 5.7 `TestRealReview_SnapshotDiff` — snapshot taken before session, diff after
  - [x] 5.8 Update golden file via `go test -update`

## Dev Notes

### Architecture & Design Decisions

- **C1 Model:** Claude inside review session does the writing to LEARNINGS.md via file tools. Go post-validates after session ends (snapshot-diff model from Story 6.1).
- **FR28a:** Review-сессия при наличии findings сама записывает уроки. Clean review = no lessons.
- **M2 Invariant Update (Critical):** Line 127 of review.md currently says `"MUST NOT write LEARNINGS.md or CLAUDE.md"`. This was correct for Epic 4 (no knowledge system) but MUST be reversed for Epic 6. The golden file `runner/testdata/TestPrompt_Review.golden` line 120 also has this text — must update.
- **FR17 Completion:** FR17 lessons deferred from Epic 4 now implemented.

### RealReview KnowledgeWriter Access

RealReview currently takes `RunConfig` which has:
```go
type RunConfig struct {
    Cfg        *config.Config
    Git        GitClient
    TasksFile  string
    SerenaHint string
}
```

KnowledgeWriter is on `Runner` struct (runner.go:334), not on `RunConfig`. Two options:
1. **Add to RunConfig** — cleaner, passes through naturally
2. **Add to ReviewFunc signature** — breaks existing interface

Recommendation: Add `Knowledge KnowledgeWriter` to `RunConfig`. This keeps ReviewFunc signature clean and is consistent with how Cfg and Git are passed. All 3 call sites that create RunConfig already have access to Runner.Knowledge.

### Current Review Prompt (runner/prompts/review.md)

Key sections to modify:
- Line 117-122: `{{- if .SerenaEnabled}}` Code Navigation section — AFTER this section
- Line 124-128: Prompt Invariants — **MUST MODIFY**: remove "MUST NOT write LEARNINGS.md"

Current invariants (line 126-128):
```
- **MUST NOT modify source code** (FR17): this is a review session — you read and analyze code, you do NOT change it
- **MUST NOT write LEARNINGS.md or CLAUDE.md**: knowledge extraction is deferred to Epic 6
- **Mutation Asymmetry**: review sessions write task markers and findings ONLY; execute sessions MUST NOT write task markers
```

After update:
```
- **MUST NOT modify source code** (FR17): this is a review session — you read and analyze code, you do NOT change it
- **MAY write to LEARNINGS.md**: review sessions write lessons on findings (FR28a), do NOT write on clean review
- **MUST NOT write to CLAUDE.md or .claude/ directory**: Ralph controls its own files only
- **Mutation Asymmetry**: review sessions write task markers, findings, and lessons ONLY; execute sessions MUST NOT write task markers
```

### File Layout

| File | Purpose |
|------|---------|
| `runner/prompts/review.md` | MODIFY: add Knowledge Extraction section, update invariants (M2) |
| `runner/runner.go` | MODIFY: add Knowledge to RunConfig, wire snapshot-diff into RealReview |
| `runner/runner_test.go` | MODIFY: add review knowledge tests |
| `runner/prompt_test.go` | MODIFY: update invariant assertion tests |
| `runner/testdata/TestPrompt_Review.golden` | UPDATE: via `go test -update` |

### Existing Test References

- `runner/prompt_test.go:322`: `{"invariant no write learnings", "MUST NOT write LEARNINGS", true}` — this assertion MUST change: now should check "MAY write to LEARNINGS.md"
- `runner/runner_test.go` — RealReview tests use `cleanReviewFn`, `findingsReviewFn` closures
- `runner/test_helpers_test.go` — `trackingKnowledgeWriter` already tracks ValidateNewLessons calls

### Error Wrapping Convention

```go
fmt.Errorf("runner: review: validate lessons: %w", err)
// Existing: "runner: review: read tasks:", "runner: review: execute:", "runner: review: assemble prompt:"
```

### Dependency Direction

No new packages. RunConfig gets new field but no new imports.

### Testing Standards

- Table-driven, Go stdlib assertions, `t.TempDir()`
- Golden files: `go test -update` for review prompt
- Symmetric assertion: if FindingsWriteLessons checks ValidateNewLessons called, CleanNoLessons must check NOT called
- Prompt tests: section-specific substrings, invariant change verification
- Mock: `trackingKnowledgeWriter.ValidateNewLessonsCount` for call tracking

### Code Review Learnings from Story 6.1

- Non-interface methods break testability — ensure new methods are accessible through existing interfaces
- Doc comment accuracy: verify "MUST NOT" → "MAY" change reflected in ALL comments
- Dead parameters: don't add fields to RunConfig that won't be used by all call sites

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.4]
- [Source: runner/prompts/review.md:126-128 — current Prompt Invariants to modify]
- [Source: runner/testdata/TestPrompt_Review.golden:120 — golden file invariant text]
- [Source: runner/prompt_test.go:322 — invariant assertion test to update]
- [Source: runner/runner.go:85-139 — RealReview function]
- [Source: runner/runner.go:310-319 — RunConfig struct to extend]
- [Source: runner/runner.go:334 — Runner.Knowledge field]
- [Source: runner/knowledge_write.go:82 — ValidateNewLessons signature]

## Dev Agent Record

### Context Reference
- Story 6.1 (FileKnowledgeWriter, ValidateNewLessons, LessonsData, snapshot-diff model)
- Story 6.2 (knowledge injection, --append-system-prompt, M2 invariant already updated)
- Story 6.3 (ResumeExtraction snapshot-diff pattern, reused for RealReview)

### Agent Model Used
claude-opus-4-6

### Debug Log References
- Task 2 M2 invariant: already done in Story 6.2, verified and enhanced with CLAUDE.md protection
- Task 3: chose Option A (RunConfig) per Dev Notes recommendation
- 5.1-5.4 prompt tests merged into existing TestPrompt_Review table (no standalone duplicates)
- Added TestRealReview_ValidateLessonsError for error path coverage (beyond AC)

### Completion Notes List
- All 5 tasks (23 subtasks) implemented and passing
- Full regression: `go test ./...` — all 8 packages PASS (0 failures)
- Knowledge Extraction section added to review.md with atomized fact format
- Invariants updated: MAY write LEARNINGS.md, MUST NOT write CLAUDE.md/.claude/
- RunConfig.Knowledge field wired into rc in Execute()
- Snapshot-diff in RealReview: os.ReadFile before session, ValidateNewLessons after (non-clean only)
- Clean review skips ValidateNewLessons (AC #3)
- 8 new prompt assertions + 4 new standalone RealReview tests
- Golden file updated via `go test -update`

### File List
| File | Action | Purpose |
|------|--------|---------|
| `runner/prompts/review.md` | MODIFIED | Added Knowledge Extraction section, updated Prompt Invariants (M2) |
| `runner/runner.go` | MODIFIED | Added Knowledge to RunConfig, snapshot-diff + ValidateNewLessons in RealReview, updated doc comment |
| `runner/prompt_test.go` | MODIFIED | Added 8 knowledge assertions to TestPrompt_Review table |
| `runner/runner_test.go` | MODIFIED | Added 4 new RealReview knowledge tests (findings, clean, snapshot, error) |
| `runner/testdata/TestPrompt_Review.golden` | UPDATED | Regenerated via `go test -update` |
