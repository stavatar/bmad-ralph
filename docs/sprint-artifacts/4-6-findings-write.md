# Story 4.6: Findings Write

Status: Ready for Review

## Story

As a review session,
I want to overwrite review-findings.md with structured CONFIRMED findings when issues are detected,
so that the next execute session knows exactly what to fix.

## Acceptance Criteria

```gherkin
Scenario: Findings written to review-findings.md (AC1)
  Given review found 2 CONFIRMED findings
  When review session writes findings
  Then review-findings.md contains structured findings (FR17)
  And previous content fully replaced (overwrite, not append)
  And findings are for current task only (no task ID in file)

Scenario: Finding format matches contract (AC2)
  Given CONFIRMED finding with severity HIGH
  When written to review-findings.md
  Then format: ЧТО/ГДЕ/ПОЧЕМУ/КАК structure (Story 4.4)
  And severity clearly indicated
  And file is self-contained (execute session needs no other context)

Scenario: Only current task findings in file (AC3)
  Given review for task 3
  When findings written
  Then review-findings.md contains ONLY task 3 findings
  And no historical findings from previous tasks
  And file is transient — represents current state only

Scenario: Runner detects findings via file state (AC4)
  Given review session wrote findings
  When runner checks review outcome
  Then review-findings.md exists and is non-empty
  And ReviewResult.Clean = false
  And ReviewResult.FindingsPath points to review-findings.md
```

## Tasks / Subtasks

- [x] Task 1: Add "Findings Write" section to review.md (AC: 1, 2, 3)
  - [x] 1.1 Add new `## Findings Write` section to `runner/prompts/review.md` between "Clean Review Handling" and "Prompt Invariants". Current structure: Clean Review ends at ~line 90, `---` at line 92, Prompt Invariants at line 94. Replace the single `---` separator with: `---` + new Findings Write section + `---` (each major section is separated by `---`)
  - [x] 1.2 Instruct Claude: when CONFIRMED findings exist (non-clean review), write ALL confirmed findings to review-findings.md
  - [x] 1.3 Instruct Claude: OVERWRITE review-findings.md completely (not append) — previous content fully replaced
  - [x] 1.4 Instruct Claude: use ЧТО/ГДЕ/ПОЧЕМУ/КАК as output headers when writing findings to review-findings.md. These map to the 4 fields already defined in the Finding Structure section (Story 4.4): ЧТО=Description, ГДЕ=Location, ПОЧЕМУ=Reasoning, КАК=Recommendation. Do NOT re-define the fields — REFERENCE the Finding Structure section and specify that review-findings.md uses Russian headers:
    - ЧТО не так (description)
    - ГДЕ в коде (file path + line range)
    - ПОЧЕМУ это проблема (reasoning/impact)
    - КАК исправить (actionable recommendation)
  - [x] 1.5 Instruct Claude: include severity level for each finding (CRITICAL/HIGH/MEDIUM/LOW from Severity Assignment section)
  - [x] 1.6 Instruct Claude: file contains ONLY current task findings — no historical data, no task ID needed (file is always about the current task)
  - [x] 1.7 Instruct Claude: file must be self-contained — the execute session reads ONLY this file for fix context (via `__FINDINGS_CONTENT__` injection in execute prompt)
  - [x] 1.8 Instruct Claude: do NOT mark [x] in sprint-tasks.md when findings exist — that is Clean Review Handling only (Story 4.5)
  - [x] 1.9 Maintain two-stage assembly compatibility: no `{{.Var}}` template variables needed — plain text instructions only
  - [x] 1.10 Verify section integrates naturally with existing review.md flow: sub-agents -> verification -> severity -> exclusion -> structure -> clean handling -> **findings write** -> invariants

- [x] Task 2: Update TestPrompt_Review structural assertions (AC: 1, 2, 3, 4)
  - [x] 2.1 In `runner/prompt_test.go` function `TestPrompt_Review`, flip the scope-creep guard:
    - Change `{"no overwrite findings instructions", "overwrite review-findings", false}` -> `{"overwrite findings instructions", "overwrite review-findings", true}`
  - [x] 2.2 Flip the ЧТО format guard:
    - Change `{"no Story 4.6 format keywords", "ЧТО", false}` -> `{"findings format keyword", "ЧТО", true}`
  - [x] 2.3 Add new structural assertions for findings write section:
    - `"ГДЕ"` keyword present (AC2: location field in Russian)
    - `"ПОЧЕМУ"` keyword present (AC2: reasoning field in Russian)
    - `"КАК"` keyword present (AC2: recommendation field in Russian)
    - `"self-contained"` or `"no other context"` keyword present (AC2: execute session autonomy)
    - `"ONLY current task findings"` keyword present (AC3: no historical data) — MUST use this precise phrase, NOT generic `"current task"` which already exists in Clean Review Handling section (line 83: "Mark current task done") and would match there instead
    - `"do NOT mark"` or similar — no [x] marking when findings exist (AC1 boundary with Story 4.5)
    - NOTE: do NOT add a generic `"overwrite"` check — Task 2.1 already asserts `"overwrite review-findings"` which is more precise (per Story 4.5 learning: absence checks use precise phrases)
  - [x] 2.4 Use `assertContains` helper for any assertions outside the table-driven pattern
  - [x] 2.5 Verify discriminating keywords: ensure new assertions don't accidentally match other sections (e.g. "ЧТО" is unique to Russian format, "overwrite review-findings" is unique to findings write)

- [x] Task 3: Update golden file (AC: all)
  - [x] 3.1 Run `go test ./runner/ -run TestPrompt_Review -update` to regenerate `runner/testdata/TestPrompt_Review.golden`
  - [x] 3.2 Verify golden file contains the new findings write section

- [x] Task 4: Run full test suite (AC: all)
  - [x] 4.1 `go test ./runner/` — all tests pass including updated TestPrompt_Review
  - [x] 4.2 `go test ./...` — no regressions
  - [x] 4.3 `go build ./...` — clean build

## Dev Notes

### Architecture Constraints

- **Prompt-only story**: NO Go logic changes except test assertion updates. All deliverables are in `runner/prompts/review.md` and `runner/prompt_test.go`
- **Two-stage assembly**: `text/template` (stage 1) + `strings.Replace` (stage 2). Review.md uses `__TASK_CONTENT__` placeholder (stage 2). No `{{.Var}}` template variables needed for this story
- **go:embed**: `reviewTemplate` in `runner/runner.go:22` — already embeds `runner/prompts/review.md`. Any changes to review.md are automatically reflected
- **File-state detection**: `DetermineReviewOutcome` in `runner/runner.go:134-159` already implements the Go logic for detecting findings from file state (os.ReadFile + non-empty check). This story adds PROMPT instructions that tell Claude WHAT to write — the runner code that READS the result already exists
- **FindingsPath not in ReviewResult yet**: AC4 references `ReviewResult.FindingsPath` but the current struct only has `Clean bool` (FindingsPath deferred per Story 4.3 Dev Notes). For this prompt-only story, AC4 is satisfied by: (1) prompt instructs Claude to write to review-findings.md, (2) `DetermineReviewOutcome` detects non-empty file → `Clean: false`. FindingsPath field will be added when needed (Story 4.7 or later)

### Key Design Decision: Overwrite Semantics

review-findings.md is **transient** (Architecture decision):
- **Overwrite on findings** — Claude replaces entire file content, never appends
- **Clear on clean** — Story 4.5 already handles clearing
- No task ID in file — it's always about the current task (context-free design)
- File consumed by execute prompt via `__FINDINGS_CONTENT__` injection (Story 3.1)

### Current review.md Structure (after Story 4.5, ~99 lines)

| Section | Lines | Story |
|---------|-------|-------|
| Intro + task | 1-6 | 4.1 |
| Sub-Agent Orchestration | 10-23 | 4.4 |
| Verification | 25-39 | 4.4 |
| Severity Assignment | 41-53 | 4.4 |
| False Positive Exclusion | 55-60 | 4.4 |
| Finding Structure | 63-73 | 4.4 |
| Clean Review Handling | 77-90 | 4.5 |
| Prompt Invariants | 94-98 | 4.4 |

Story 4.6 adds: **Findings Write** section between "Clean Review Handling" and "Prompt Invariants".

### Review Prompt Flow After Story 4.6

1. Launch 5 sub-agents -> collect findings
2. Verify each finding -> CONFIRMED / FALSE POSITIVE
3. Assign severity to CONFIRMED
4. Exclude FALSE POSITIVE from output
5. Format CONFIRMED findings (4-field structure)
6. If NO confirmed findings -> clean review: mark [x] + clear findings (Story 4.5)
7. **If confirmed findings exist -> overwrite review-findings.md with structured findings (THIS STORY)**
8. Invariants always apply

### Scope Boundary: Story 4.5 vs 4.6

| Concern | Story 4.5 | Story 4.6 (this) |
|---------|-----------|-------------------|
| When | No confirmed findings (clean) | Has confirmed findings |
| Action on sprint-tasks.md | Mark current task [x] | Do NOT mark [x] |
| Action on review-findings.md | Clear / delete | Overwrite with findings |
| Format instructions | N/A (no content to write) | ЧТО/ГДЕ/ПОЧЕМУ/КАК structure |

Do NOT modify clean handling instructions — those are Story 4.5 (already done).

### Story 4.5 Code Review Learnings (apply to 4.6)

- **Constraint instruction assertions**: when AC says "mandatory" or "exactly one", assert the constraint TEXT (e.g., "Severity is mandatory") not just the keyword list
- **Ordering/completeness constraint assertions**: when AC requires temporal ordering (e.g., "before writing"), assert the ordering instruction text
- **Absence checks use precise phrases**: "overwrite review-findings" not generic "overwrite" — single common words cause false failures
- **Discriminating cross-agent assertion keywords**: use unique substrings that won't match other sections

### Story 4.4 Code Review Learnings (apply to 4.6)

- **Never silently discard return values**: capture and assert on all returns
- **Test ALL error return paths**: when a function has N error returns, need N test cases
- **Prompt Instructions must cover ALL SCOPE areas**: all 4 ACs must be addressed in prompt text

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `goldenTest(t, name, got)` | prompt_test.go:16 | Golden file comparison |
| `assertContains(t, text, substr, msg)` | prompt_test.go:362 | Substring assertion |
| `config.AssemblePrompt` | config/prompt.go | Two-stage assembly |
| `config.TemplateData{}` | config/prompt.go | Empty template data |

### Existing Test to Modify

`TestPrompt_Review` in `runner/prompt_test.go:271-351`:
- Currently has 37 structural checks (Story 4.5)
- Has scope-creep guards: `"overwrite review-findings"` (present: false) and `"ЧТО"` (present: false)
- Story 4.6 flips BOTH guards to `present: true` and adds new findings write assertions

### KISS/DRY/SRP Analysis

**KISS:**
- review.md = plain text instructions for Claude. No complex logic
- Test = update existing table-driven assertions + golden file refresh (established pattern)

**DRY:**
- Reuses `goldenTest` helper (same as all prompt tests)
- Reuses `assertContains` helper (same as agent prompt tests)
- Reuses `config.AssemblePrompt` (same assembly pipeline)
- Updates EXISTING test function — no new test functions needed

**SRP:**
- Story 4.6 = findings write section ONLY
- Story 4.5 = clean handling section (separate story, already done)
- Story 4.7 = execute->review loop wiring (separate story, next)

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/prompts/review.md` | Add "Findings Write" section (~15-20 lines) between "Clean Review Handling" and "Prompt Invariants" |
| `runner/prompt_test.go` | Update `TestPrompt_Review`: flip 2 guards, add ~7 new structural assertions |

**Files to UPDATE (auto-generated):**
| File | Content |
|------|---------|
| `runner/testdata/TestPrompt_Review.golden` | Regenerated by `go test -update` |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/runner.go:134-159` | `DetermineReviewOutcome` — verify findings detection logic exists |
| `runner/runner.go:70-125` | `realReview` — verify it delegates to DetermineReviewOutcome |
| `config/constants.go` | `TaskOpen`, `TaskDone` constants for reference |
| `runner/prompts/execute.md` | Verify `__FINDINGS_CONTENT__` injection reads review-findings.md |

**Files NOT to create**: No new Go files. All changes go in existing files.

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.6] — AC and technical requirements
- [Source: runner/prompts/review.md] — Current review prompt (99 lines, 8 sections from Story 4.5)
- [Source: runner/runner.go:134-159] — `DetermineReviewOutcome` function (file-state detection logic)
- [Source: runner/runner.go:70-125] — `realReview` function (review session orchestration)
- [Source: runner/prompt_test.go:271-351] — `TestPrompt_Review` function to update
- [Source: config/prompt.go] — AssemblePrompt function, TemplateData struct
- [Source: docs/sprint-artifacts/4-5-clean-review-handling.md] — Story 4.5 scope boundary + learnings
- [Source: docs/sprint-artifacts/4-4-findings-verification-logic.md] — Story 4.4 finding structure
- [Source: .claude/rules/code-quality-patterns.md] — Prompt instruction coverage rules
- [Source: .claude/rules/test-assertions.md] — Assertion patterns, discriminating keywords

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Added "Findings Write" section (~20 lines) to review.md between Clean Review Handling and Prompt Invariants
- Section instructs Claude to overwrite review-findings.md with ЧТО/ГДЕ/ПОЧЕМУ/КАК structured format, severity, self-contained content
- Flipped 2 scope-creep guards from false→true in TestPrompt_Review
- Added 6 new structural assertions covering all 4 Russian headers, self-contained, only current task, and no-mark boundary
- Code review added 2 more assertions (never appended, [SEVERITY] format) + precision fixes
- Total: 46 assertions in TestPrompt_Review (36 existing + 10 Story 4.6)
- All tests pass, no regressions, clean build

### File List

- `runner/prompts/review.md` — added Findings Write section (lines 94-113)
- `runner/prompt_test.go` — updated TestPrompt_Review: flipped 2 guards, added 6 assertions
- `runner/testdata/TestPrompt_Review.golden` — regenerated

### Change Log

- 2026-02-27: Story 4.6 — Added Findings Write section to review prompt with ЧТО/ГДЕ/ПОЧЕМУ/КАК format, overwrite semantics, self-contained output, and scope boundary with Story 4.5. Updated 43 structural test assertions.
