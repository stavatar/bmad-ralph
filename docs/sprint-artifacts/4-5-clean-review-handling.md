# Story 4.5: Clean Review Handling

Status: done

## Story

As a review session,
I want to atomically mark the current task `[x]` in sprint-tasks.md and clear review-findings.md when no confirmed findings exist,
so that the runner can detect a clean review via file state and proceed to the next task.

## Acceptance Criteria

```gherkin
Scenario: Atomic [x] marking + findings clear on clean review (AC1)
  Given review found no CONFIRMED findings (clean review)
  When review session writes results
  Then marks current task [x] in sprint-tasks.md (FR17)
  And clears review-findings.md content (FR17)
  And both operations happen together (atomic: both or neither)

Scenario: Review MUST NOT modify git working tree (AC2)
  Given clean review handling
  When review session runs
  Then MUST NOT run git commands
  And MUST NOT modify source code files
  And ONLY modifies sprint-tasks.md and review-findings.md

Scenario: Runner detects clean review via file state (AC3)
  Given review session completed
  When runner checks review outcome
  Then detects [x] on current task in sprint-tasks.md
  And review-findings.md is empty or absent
  And ReviewResult.Clean = true

Scenario: review_cycles resets on clean review (AC4)
  Given review_cycles = 2 for current task
  When clean review detected
  Then review_cycles resets to 0 (Story 3.10 counter)
  And runner proceeds to next task

Scenario: review-findings.md absence equals clean (AC5)
  Given review-findings.md does not exist after review
  When runner checks
  Then treats as clean review (Architecture: "absent = empty")
```

## Tasks / Subtasks

- [ ] Task 1: Add "Clean Review Handling" section to review.md (AC: 1, 2, 5)
  - [ ] 1.1 Add new `## Clean Review Handling` section to `runner/prompts/review.md` after the "Finding Structure" section and before "Prompt Invariants"
  - [ ] 1.2 Instruct Claude: when ALL findings are FALSE POSITIVE or no findings reported by any sub-agent, this is a CLEAN REVIEW
  - [ ] 1.3 Instruct Claude: on clean review, perform TWO operations atomically (both or neither):
    - Mark current task `[x]` in sprint-tasks.md (change `- [ ]` to `- [x]` for the reviewed task)
    - Clear review-findings.md (write empty content or delete the file)
  - [ ] 1.4 Instruct Claude: MUST NOT run any git commands (no `git add`, `git commit`, etc.) — review only modifies sprint-tasks.md and review-findings.md
  - [ ] 1.5 Instruct Claude: MUST NOT modify any source code files — review reads code, does not change it
  - [ ] 1.6 Instruct Claude: if review-findings.md does not exist, treat as already clean (no need to create then clear it)
  - [ ] 1.7 Maintain two-stage assembly compatibility: no `{{.Var}}` template variables needed — plain text instructions only
  - [ ] 1.8 Verify section integrates naturally with existing review.md flow (sub-agents → verification → severity → exclusion → structure → **clean handling** → invariants)

- [ ] Task 2: Update TestPrompt_Review structural assertions (AC: 1, 2, 3, 5)
  - [ ] 2.1 In `runner/prompt_test.go` function `TestPrompt_Review`, update the existing scope-creep guard for `[x]` marking:
    - Change `{"no mark [x] instructions", "mark [x]", false}` → `{"clean review mark [x]", "mark [x]", true}` (Story 4.5 adds this)
  - [ ] 2.2 Keep `{"no overwrite findings instructions", "overwrite review-findings", false}` unchanged — Story 4.6 adds overwrite instructions
  - [ ] 2.3 Add new structural assertions for clean review handling keywords:
    - `"clean review"` or `"CLEAN REVIEW"` keyword present
    - `"clear review-findings"` or similar clearing instruction present
    - `"sprint-tasks.md"` reference in clean handling context (already present from task content, need discriminating assertion)
    - `"- [ ]"` to `"- [x]"` transformation instruction present
    - Atomicity keyword: prefer `"atomic"` over `"both"` — `"both"` is too generic and may match other sections
    - No git commands: `"MUST NOT run"` + `"git"` — discriminating keyword for AC2
  - [ ] 2.4 Use `assertContains` helper for any assertions outside the table-driven pattern
  - [ ] 2.5 Add absence check: Story 4.6 scope-creep guard — verify `"overwrite review-findings"` guard (Task 2.2) still present:false AND add new absence for `"ЧТО"` keyword (Story 4.6 ЧТО/ГДЕ/ПОЧЕМУ/КАК format not yet added)

- [ ] Task 3: Update golden file (AC: all)
  - [ ] 3.1 Run `go test ./runner/ -run TestPrompt_Review -update` to regenerate `runner/testdata/TestPrompt_Review.golden`
  - [ ] 3.2 Verify golden file contains the new clean handling section

- [ ] Task 4: Run full test suite (AC: all)
  - [ ] 4.1 `go test ./runner/` — all tests pass including updated TestPrompt_Review
  - [ ] 4.2 `go test ./...` — no regressions
  - [ ] 4.3 `go build ./...` — clean build

## Dev Notes

### Architecture Constraints

- **Prompt-only story**: NO Go logic changes except test assertion updates. All deliverables are in `runner/prompts/review.md` and `runner/prompt_test.go`
- **Two-stage assembly**: `text/template` (stage 1) + `strings.Replace` (stage 2). Review.md uses `__TASK_CONTENT__` placeholder (stage 2). No `{{.Var}}` template variables needed for this story
- **go:embed**: `reviewTemplate` in `runner/runner.go:22` — already embeds `runner/prompts/review.md`. Any changes to review.md are automatically reflected
- **File-state detection**: `DetermineReviewOutcome` in `runner/runner.go:134-159` already implements the Go logic for detecting clean reviews from file state. This story adds the PROMPT instructions that tell Claude WHAT to do — the runner code that READS the result already exists

### Key Design Decision: Claude Responsibility for Atomicity

Ralph does NOT enforce atomicity of [x] marking + findings clearing. The review.md prompt instructs Claude to:
1. Determine review outcome (clean or findings)
2. If clean: mark [x] in sprint-tasks.md AND clear review-findings.md
3. Claude performs both operations in sequence within the same session

From Ralph's perspective: after `session.Execute` completes, `DetermineReviewOutcome` checks file state — it does not know or care HOW Claude wrote the files.

### Current review.md Structure (after Story 4.4, 82 lines)

| Section | Lines | Story |
|---------|-------|-------|
| Intro + task | 1-7 | 4.1 |
| Sub-Agent Orchestration | 10-23 | 4.4 |
| Verification | 25-39 | 4.4 |
| Severity Assignment | 41-53 | 4.4 |
| False Positive Exclusion | 55-61 | 4.4 |
| Finding Structure | 63-74 | 4.4 |
| Prompt Invariants | 76-82 | 4.4 |

Story 4.5 adds: **Clean Review Handling** section between "Finding Structure" and "Prompt Invariants".

### Review Prompt Flow After Story 4.5

1. Launch 5 sub-agents → collect findings
2. Verify each finding → CONFIRMED / FALSE POSITIVE
3. Assign severity to CONFIRMED
4. Exclude FALSE POSITIVE from output
5. Format CONFIRMED findings (4-field structure)
6. **If NO confirmed findings → clean review: mark [x] + clear findings (THIS STORY)**
7. If confirmed findings exist → write to review-findings.md (Story 4.6)
8. Invariants always apply

### Scope Boundary: Story 4.5 vs 4.6

| Concern | Story 4.5 (this) | Story 4.6 |
|---------|-------------------|-----------|
| When | No confirmed findings (clean) | Has confirmed findings |
| Action on sprint-tasks.md | Mark current task [x] | Do NOT mark [x] |
| Action on review-findings.md | Clear / delete | Write confirmed findings |
| Format instructions | N/A (no content to write) | ЧТО/ГДЕ/ПОЧЕМУ/КАК structure |

Do NOT add findings-write instructions — those are Story 4.6.

### Story 4.3 Code Review Learnings (apply to 4.5)

- **Unused param = doc lie**: if a function parameter isn't used, remove it or document why
- **errors.Is convention**: always `errors.Is(err, target)` not type assertions
- **Dead stub removal**: when replacing a stub with real implementation, update doc comments immediately
- **Prompt Instructions must cover ALL SCOPE areas**: all 5 ACs must be addressed in prompt text

### Story 4.4 Code Review Learnings (apply to 4.5)

- **Never silently discard return values**: capture and assert on all returns
- **Test ALL error return paths**: when a function has N error returns, need N test cases
- **Discriminating cross-agent assertion keywords**: use unique substrings that won't match other sections

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `goldenTest(t, name, got)` | prompt_test.go:16 | Golden file comparison |
| `assertContains(t, text, substr, msg)` | prompt_test.go:352 | Substring assertion |
| `config.AssemblePrompt` | config/prompt.go | Two-stage assembly |
| `config.TemplateData{}` | config/prompt.go | Empty template data |

### Existing Test to Modify

`TestPrompt_Review` in `runner/prompt_test.go:271-341`:
- Currently has 29 structural checks (Story 4.4)
- Has scope-creep guards: `"mark [x]"` (present: false) and `"overwrite review-findings"` (present: false)
- Story 4.5 flips the `"mark [x]"` guard to `present: true` and adds new clean handling assertions
- Story 4.6 will flip the `"overwrite review-findings"` guard to `present: true`

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
- Story 4.5 = clean handling section ONLY
- Story 4.6 = findings write section (separate story, separate section)
- Each story adds one section to review.md in sequence

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/prompts/review.md` | Add "Clean Review Handling" section (~15-20 lines) between "Finding Structure" and "Prompt Invariants" |
| `runner/prompt_test.go` | Update `TestPrompt_Review`: flip `[x]` guard, add ~6-8 new structural assertions |

**Files to UPDATE (auto-generated):**
| File | Content |
|------|---------|
| `runner/testdata/TestPrompt_Review.golden` | Regenerated by `go test -update` |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/runner.go:134-159` | `DetermineReviewOutcome` — verify clean detection logic exists |
| `runner/runner.go:70-125` | `realReview` — verify it delegates to DetermineReviewOutcome |
| `config/constants.go` | `TaskOpen`, `TaskDone` constants for [x] marking reference |
| `runner/scan.go` | ScanTasks — understand task checkbox format |

**Files NOT to create**: No new Go files. All changes go in existing files.

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.5] — AC and technical requirements
- [Source: runner/prompts/review.md] — Current review prompt (82 lines, 7 sections from Story 4.4)
- [Source: runner/runner.go:134-159] — `DetermineReviewOutcome` function (file-state detection logic)
- [Source: runner/runner.go:70-125] — `realReview` function (review session orchestration)
- [Source: runner/prompt_test.go:271-341] — `TestPrompt_Review` function to update
- [Source: config/prompt.go] — AssemblePrompt function, TemplateData struct
- [Source: config/constants.go] — TaskOpen `"- [ ]"`, TaskDone `"- [x]"` constants
- [Source: docs/sprint-artifacts/4-4-findings-verification-logic.md#Dev Notes] — Story 4.4 learnings
- [Source: docs/sprint-artifacts/4-3-review-session-logic.md#Dev Notes] — Story 4.3 learnings
- [Source: .claude/rules/code-quality-patterns.md] — Prompt instruction coverage rules
- [Source: .claude/rules/test-assertions.md] — Assertion patterns, discriminating keywords

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- All 4 tasks completed: review.md section added, test assertions updated (7 new + 1 flipped), golden file regenerated, full suite green
- Prompt-only story: no Go logic changes, only review.md + prompt_test.go + golden file
- Clean section covers all 5 ACs: atomicity (AC1), no git (AC2), file-state detection (AC3), counter reset referenced (AC4), absent=clean (AC5)
- Story 4.6 scope-creep guard maintained: `"overwrite review-findings"` still absent:false

### File List

- runner/prompts/review.md — added "Clean Review Handling" section (~16 lines)
- runner/prompt_test.go — updated TestPrompt_Review: 7 new assertions + 1 flipped guard
- runner/testdata/TestPrompt_Review.golden — regenerated
