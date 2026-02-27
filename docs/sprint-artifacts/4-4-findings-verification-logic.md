# Story 4.4: Findings Verification Logic

Status: Done

## Story

As a review prompt,
I want to instruct Claude to verify each sub-agent finding and classify it,
so that only real confirmed issues are written to review-findings.md.

## Acceptance Criteria

```gherkin
Scenario: Each finding verified independently (AC1)
  Given 5 sub-agents produced findings
  When review session verifies each finding
  Then each classified as CONFIRMED or FALSE POSITIVE (FR16)
  And verification happens BEFORE writing to review-findings.md

Scenario: Severity assigned to confirmed findings (AC2)
  Given finding classified as CONFIRMED
  When severity assigned
  Then exactly one of: CRITICAL, HIGH, MEDIUM, LOW (FR16)
  And severity is mandatory for every CONFIRMED finding

Scenario: False positives excluded from findings file (AC3)
  Given finding classified as FALSE POSITIVE
  When review-findings.md is written
  Then FALSE POSITIVE findings are NOT included
  And only CONFIRMED findings appear in file

Scenario: Finding structure complete (AC4)
  Given CONFIRMED finding
  When written to review-findings.md
  Then contains: description (what is wrong)
  And contains: location (file path + line range)
  And contains: reasoning (why this is a problem)
  And contains: recommendation (how to fix)
```

## Tasks / Subtasks

- [x] Task 1: Expand review.md with sub-agent orchestration and verification section (AC: 1, 2, 3, 4)
  - [x] 1.1 Add sub-agent launch instructions to review.md: instruct Claude to launch 5 sub-agents (quality, implementation, simplification, design-principles, test-coverage) via Task tool, each reading their prompt from the corresponding file in `runner/prompts/agents/{name}.md` (Claude has filesystem access in project root)
  - [x] 1.2 Add verification section: after sub-agents report, Claude MUST verify EACH finding independently by checking if the claimed issue actually exists in the code
  - [x] 1.3 Add classification instructions: CONFIRMED = verified real issue; FALSE POSITIVE = issue does not actually exist or is not a problem
  - [x] 1.4 Add severity assignment instructions: every CONFIRMED finding MUST have exactly one severity: CRITICAL (blocks functionality), HIGH (significant issue), MEDIUM (improvement needed), LOW (minor/style)
  - [x] 1.5 Add false positive exclusion: FALSE POSITIVE findings MUST NOT appear in review-findings.md
  - [x] 1.6 Add finding structure format: each CONFIRMED finding must include 4 fields (description, location, reasoning, recommendation)
  - [x] 1.7 Keep `__TASK_CONTENT__` placeholder for current task injection (already exists)
  - [x] 1.8 Maintain two-stage assembly compatibility: review.md is Go template (text/template first stage), `__TASK_CONTENT__` is strings.Replace (second stage). Do NOT add `{{.Var}}` template vars unless needed
  - [x] 1.9 Add prompt invariant guardrails (from Epic 4 invariants, originally Story 4.1 AC): MUST NOT modify source code (FR17), MUST NOT write LEARNINGS.md or CLAUDE.md (deferred to Epic 6), Mutation Asymmetry (review writes `[x]` and findings ONLY, execute MUST NOT write `[x]`)

- [x] Task 2: Create TestPrompt_Review golden file test (AC: 1, 2, 3, 4)
  - [x] 2.1 Add `TestPrompt_Review` function in `runner/prompt_test.go`
  - [x] 2.2 Assemble review prompt via `config.AssemblePrompt(reviewTemplate, config.TemplateData{}, replacements)` with `__TASK_CONTENT__` replacement
  - [x] 2.3 Golden file: `runner/testdata/TestPrompt_Review.golden`
  - [x] 2.4 Use existing `goldenTest(t, name, got)` helper

- [x] Task 3: Add structural assertion tests for verification keywords (AC: 1, 2, 3, 4)
  - [x] 3.1 Table-driven `checks` in `TestPrompt_Review` (same pattern as `TestPrompt_Execute_WithFindings`)
  - [x] 3.2 Assert presence of verification keywords: "CONFIRMED", "FALSE POSITIVE", "verify" or "verification"
  - [x] 3.3 Assert presence of severity keywords: "CRITICAL", "HIGH", "MEDIUM", "LOW"
  - [x] 3.4 Assert presence of finding structure keywords: "description" or "what", "location" or "file path", "reasoning" or "why", "recommendation" or "how to fix"
  - [x] 3.5 Assert presence of sub-agent names: "quality", "implementation", "simplification", "design-principles", "test-coverage"
  - [x] 3.6 Assert `__TASK_CONTENT__` placeholder is replaced (not present in output)
  - [x] 3.7 Assert task content IS present in output (injection worked)
  - [x] 3.8 Assert presence of invariant guardrails: "MUST NOT modify source code", "MUST NOT write LEARNINGS", "Mutation Asymmetry"
  - [x] 3.9 Assert absence (present: false) of Story 4.5/4.6 scope keywords that should NOT be in review.md yet: specific `[x]` marking logic instructions (e.g. "mark.*\\[x\\]" type instructions), "overwrite" findings write instructions — prevents scope creep

- [x] Task 4: Run full test suite (AC: all)
  - [x] 4.1 `go test ./runner/` — all tests pass including new TestPrompt_Review
  - [x] 4.2 `go test ./...` — no regressions
  - [x] 4.3 `go build ./...` — clean build

## Dev Notes

### Architecture Constraints

- **Prompt-only story**: NO Go logic changes except test additions. All deliverables are in `runner/prompts/review.md` and `runner/prompt_test.go`
- **Two-stage assembly**: `text/template` (stage 1) + `strings.Replace` (stage 2). Review.md uses `__TASK_CONTENT__` placeholder (stage 2). No `{{.Var}}` template variables needed for this story — review prompt does not use TemplateData fields (unlike execute prompt which uses `{{if .HasFindings}}`)
- **go:embed**: `reviewTemplate` in `runner/runner.go:22` — already embeds `runner/prompts/review.md`. Any changes to review.md are automatically reflected
- **Sub-agent prompts**: Already created in `runner/prompts/agents/` (Story 4.2). They are `go:embed`ded as `agentQualityPrompt`, `agentImplementationPrompt`, etc. The review prompt references them by name — the actual prompt content injection is Claude's Task tool mechanism (Ralph only passes the review.md prompt to the session)

### Key Design Decision: Ralph vs Claude Responsibility Boundary

Ralph does NOT verify findings. The review.md prompt instructs Claude (inside the review session) to:
1. Launch 5 sub-agents via Task tool
2. Collect findings from all sub-agents
3. Verify each finding by checking the actual code
4. Classify: CONFIRMED or FALSE POSITIVE
5. Assign severity to CONFIRMED
6. Write ONLY CONFIRMED findings (Stories 4.5 clean handling / 4.6 findings write)

From Ralph's perspective: `session.Execute(ctx, opts)` with the review prompt — one call. The verification logic is IN the prompt text, executed by Claude.

### Finding Structure: 4-Field Format

Each CONFIRMED finding must have (from AC4):
1. **Description** (ЧТО не так) — what is the issue
2. **Location** (ГДЕ) — file path + line range
3. **Reasoning** (ПОЧЕМУ) — why this is a problem
4. **Recommendation** (КАК исправить) — actionable fix suggestion

This format is consumed by the execute session in the next iteration (execute reads review-findings.md via Story 3.1's `__FINDINGS_CONTENT__` injection).

### Severity Levels

Exactly 4 levels (FR16):
- **CRITICAL** — blocks core functionality, data loss risk
- **HIGH** — significant bug or security issue
- **MEDIUM** — improvement needed, doesn't block
- **LOW** — minor style, readability

FR16a (severity filtering) is Growth — in Epic 4 ALL confirmed findings block pipeline regardless of severity.

### Current review.md State

```
Review the code changes for the following task.

Task:
__TASK_CONTENT__
```

Story 4.4 expands this to include sub-agent orchestration and verification logic. Stories 4.5 and 4.6 will add clean handling and findings write sections respectively.

### Review Prompt Scope Boundaries (Stories 4.4-4.6)

| Section | Story | Content |
|---------|-------|---------|
| Sub-agent orchestration | 4.4 (this story) | Launch 5 agents, collect findings |
| Verification & classification | 4.4 (this story) | CONFIRMED/FALSE POSITIVE, severity |
| Finding format | 4.4 (this story) | 4-field structure |
| Clean review handling | 4.5 | Atomic [x] + clear findings |
| Findings write | 4.6 | Write CONFIRMED to review-findings.md |

Do NOT add clean handling or findings write instructions — those are Stories 4.5 and 4.6.

### Prompt Invariants (MUST include in review.md)

From Epic 4 invariants:
- **MUST NOT modify source code** (FR17) — review reads code, does NOT change it
- **MUST NOT write LEARNINGS.md or CLAUDE.md** (deferred to Epic 6)
- **Mutation Asymmetry**: Review sessions write `[x]` and findings. Execute sessions MUST NOT write `[x]`

### Story 4.3 Code Review Learnings (apply to 4.4)

- **Prompt Instructions must cover ALL SCOPE areas**: if defining verification for 5 agents, instructions must guide verification for all 5
- **Detection structure tests must check ALL scope dimensions**: test each severity level and each finding field
- **Discriminating cross-agent assertion keywords**: use unique substrings that won't match other prompts

### Existing Test Helpers Available

| Helper | Source | Used for |
|--------|--------|----------|
| `goldenTest(t, name, got)` | prompt_test.go:16 | Golden file comparison |
| `assertContains(t, text, substr, msg)` | prompt_test.go:278 | Substring assertion |
| `config.AssemblePrompt` | config/prompt.go | Two-stage assembly |
| `config.TemplateData{}` | config/prompt.go | Empty template data (no fields needed) |

### Existing Test Patterns to Follow

Follow `TestPrompt_Execute_WithFindings` pattern (prompt_test.go:37):
1. Assemble prompt with test data
2. Table-driven `checks` with `name`, `substr`, `present` fields
3. Golden file comparison via `goldenTest`

### KISS/DRY/SRP Analysis

**KISS:**
- Review.md = plain text instructions for Claude. No complex logic
- Test = golden file + structural substring checks (established pattern)

**DRY:**
- Reuses `goldenTest` helper (same as execute prompt tests)
- Reuses `assertContains` helper (same as agent prompt tests)
- Reuses `config.AssemblePrompt` (same assembly pipeline)

**SRP:**
- Story 4.4 = verification logic section ONLY
- Story 4.5 = clean handling section (separate story)
- Story 4.6 = findings write section (separate story)

### Project Structure Notes

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/prompts/review.md` | Expand from 4 lines to full verification section with sub-agent orchestration, classification, severity, finding format |
| `runner/prompt_test.go` | Add `TestPrompt_Review` with golden file test + structural assertions |

**Files to CREATE:**
| File | Content |
|------|---------|
| `runner/testdata/TestPrompt_Review.golden` | Generated by `go test -update` |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/runner.go:22` | `reviewTemplate` embed — verify no changes needed |
| `runner/prompts/agents/*.md` | Agent names for cross-reference in review.md |
| `config/prompt.go` | AssemblePrompt API, TemplateData struct |

**Files NOT to create**: No new Go files. All test code goes in existing `runner/prompt_test.go`.

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.4] — AC and technical requirements
- [Source: runner/prompts/review.md] — Current minimal review prompt (4 lines)
- [Source: runner/runner.go:22] — `reviewTemplate` go:embed declaration
- [Source: runner/runner.go:70-125] — `realReview` function using reviewTemplate
- [Source: runner/prompt_test.go:37-110] — TestPrompt_Execute pattern to follow
- [Source: runner/prompt_test.go:278-283] — `assertContains` helper
- [Source: runner/prompt_test.go:16-35] — `goldenTest` helper
- [Source: config/prompt.go] — AssemblePrompt function, TemplateData struct
- [Source: docs/sprint-artifacts/4-3-review-session-logic.md#Dev Notes] — Story 4.3 learnings
- [Source: docs/sprint-artifacts/4-2-sub-agent-prompts.md] — Story 4.2 learnings
- [Source: .claude/rules/code-quality-patterns.md] — Prompt instruction coverage rules
- [Source: .claude/rules/test-assertions.md] — Assertion patterns

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Expanded `runner/prompts/review.md` from 4 lines to 81-line review prompt with 7 sections: intro, sub-agent orchestration, verification, severity, false positive exclusion, finding structure, invariants
- Added `TestPrompt_Review` with 28 table-driven structural assertions covering all 4 ACs + golden file comparison
- Removed duplicate TestPrompt_Review from previous incomplete session
- Generated golden file `runner/testdata/TestPrompt_Review.golden`
- All tests pass: `go test ./...` clean, `go build ./...` clean

### File List

- `runner/prompts/review.md` — expanded (4 → 81 lines)
- `runner/prompt_test.go` — added `TestPrompt_Review` function (24 structural checks + golden)
- `runner/testdata/TestPrompt_Review.golden` — new golden file
