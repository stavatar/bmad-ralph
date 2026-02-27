# Story 4.2: Sub-Agent Prompts

Status: Ready for Review

## Story

As a review-session,
I want 5 specialized sub-agent prompts with explicit scope boundaries,
so that each agent checks its area of responsibility without overlap.

## Acceptance Criteria

```gherkin
Scenario: Five sub-agent prompts exist with explicit scopes (AC1)
  Given runner/prompts/agents/ directory
  Then contains quality.md, implementation.md, simplification.md, design-principles.md, test-coverage.md
  And each prompt defines explicit SCOPE section (what this agent checks)
  And each prompt defines explicit OUT-OF-SCOPE section (what other agents check)
  And no overlap between 5 agents' scope boundaries

Scenario: Quality agent scope (AC2)
  Given quality.md prompt
  Then scope includes: bugs, security issues, performance problems, error handling
  And out-of-scope: AC compliance (implementation), code simplicity (simplification), DRY/KISS/SRP (design-principles), test coverage (test-coverage)

Scenario: Implementation agent scope (AC3)
  Given implementation.md prompt
  Then scope includes: acceptance criteria compliance, feature completeness, requirement satisfaction
  And out-of-scope: code quality (quality), simplification (simplification), DRY/KISS/SRP (design-principles), test coverage (test-coverage)

Scenario: Simplification agent scope (AC4)
  Given simplification.md prompt
  Then scope includes: code readability, verbose constructs, dead code, simpler alternatives
  And out-of-scope: bugs (quality), AC compliance (implementation), DRY/KISS/SRP (design-principles), test coverage (test-coverage)

Scenario: Design-principles agent scope (AC5)
  Given design-principles.md prompt
  Then scope includes: DRY (code/logic duplication across functions/files), KISS (architectural over-engineering, unnecessary abstractions), SRP (functions/types with multiple responsibilities)
  And out-of-scope: bugs (quality), AC compliance (implementation), expression-level simplicity (simplification), test coverage (test-coverage)

Scenario: Test-coverage agent scope (AC6)
  Given test-coverage.md prompt
  Then scope includes: test coverage for each AC (FR37 ATDD), no skip/xfail (FR38), test quality
  And out-of-scope: code bugs (quality), AC compliance (implementation), code simplicity (simplification), DRY/KISS/SRP (design-principles)

Scenario: Golden file snapshot per agent (AC7)
  Given each sub-agent prompt
  When assembled with test fixture
  Then matches runner/testdata/TestPrompt_Agent_<name>.golden

Scenario: Adversarial test — agent detects planted issue in its scope (AC8)
  Given test fixture code with scope-specific planted issue per agent
  When prompt structure analyzed
  Then each agent's prompt is structured to detect issues in its scope
  And adversarial golden files capture expected detection behavior
```

## Tasks / Subtasks

- [x] Task 1: Create sub-agent prompt directory and 5 prompt files (AC: 1-6)
  - [x] 1.1 Directory `runner/prompts/agents/` already exists (has `.gitkeep`) — no mkdir needed
  - [x] 1.2 Create `quality.md` — SCOPE: bugs, security, performance, error handling. OUT-OF-SCOPE: AC compliance, simplification, DRY/KISS/SRP, test coverage
  - [x] 1.3 Create `implementation.md` — SCOPE: AC compliance, feature completeness, requirement satisfaction. OUT-OF-SCOPE: code quality, simplification, DRY/KISS/SRP, test coverage
  - [x] 1.4 Create `simplification.md` — SCOPE: readability, verbose constructs, dead code, simpler alternatives. OUT-OF-SCOPE: bugs, AC compliance, DRY/KISS/SRP, test coverage
  - [x] 1.5 Create `design-principles.md` — SCOPE: DRY (duplication across functions/files), KISS (over-engineering, unnecessary abstractions), SRP (multiple responsibilities). OUT-OF-SCOPE: bugs, AC compliance, expression-level simplicity, test coverage
  - [x] 1.6 Create `test-coverage.md` — SCOPE: ATDD test coverage per AC (FR37), zero-skip/no-xfail (FR38), test quality. OUT-OF-SCOPE: code bugs, AC compliance, code simplicity, DRY/KISS/SRP
  - [x] 1.7 Each prompt MUST include both SCOPE and OUT-OF-SCOPE sections with explicit cross-references to which other agent handles out-of-scope concerns

- [x] Task 2: Add go:embed declarations for sub-agent prompts (AC: 1, 7)
  - [x] 2.1 Add 5 `//go:embed prompts/agents/<name>.md` variables in `runner/runner.go` (385 lines — well within budget, add after existing embeds at line 21)
  - [x] 2.2 Variable naming: `agentQualityPrompt`, `agentImplementationPrompt`, `agentSimplificationPrompt`, `agentDesignPrinciplesPrompt`, `agentTestCoveragePrompt` — intentionally "Prompt" not "Template" because these are static text (no `text/template` processing), unlike `executeTemplate`/`reviewTemplate` which are Go templates
  - [x] 2.3 Verify `go build ./...` succeeds with all embeds

- [x] Task 3: Write tests for scope boundary validation (AC: 1-6)
  - [x] 3.1 Test name pattern: `TestPrompt_Agent_<Name>_Scope` in `runner/prompt_test.go`
  - [x] 3.2 For each agent: verify SCOPE section contains required keywords
  - [x] 3.3 For each agent: verify OUT-OF-SCOPE section contains required cross-references
  - [x] 3.4 Verify no overlap: each domain keyword appears in exactly ONE agent's SCOPE (table-driven cross-check test)
  - [x] 3.5 Use `strings.Contains` with section-specific substrings (not ambiguous fragments)

- [x] Task 4: Write golden file tests per agent (AC: 7)
  - [x] 4.1 Test name pattern: `TestPrompt_Agent_<Name>_Golden` in `runner/prompt_test.go`
  - [x] 4.2 Sub-agent prompts are static (no `text/template` conditionals, no `strings.Replace` placeholders) — golden test = direct string comparison via `goldenTest(t, name, promptContent)`
  - [x] 4.3 Generate golden files: `runner/testdata/TestPrompt_Agent_Quality.golden`, `TestPrompt_Agent_Implementation.golden`, `TestPrompt_Agent_Simplification.golden`, `TestPrompt_Agent_DesignPrinciples.golden`, `TestPrompt_Agent_TestCoverage.golden`
  - [x] 4.4 Run with `-update` first: `go test ./runner/ -run TestPrompt_Agent -update`

- [x] Task 5: Write adversarial structure tests (AC: 8)
  - [x] 5.1 Test name: `TestPrompt_Agent_ScopeExclusivity` — table-driven, verify each scope domain is assigned to exactly one agent
  - [x] 5.2 Test name: `TestPrompt_Agent_OutOfScopeCompleteness` — verify each agent's OUT-OF-SCOPE mentions all 4 other agents' domain areas
  - [x] 5.3 Test name: `TestPrompt_Agent_DetectionStructure` — verify each prompt contains instructions that would enable detection of scope-specific issues (e.g., quality agent has "security" instruction, test-coverage agent has "ATDD" instruction). NOTE: "adversarial" here means testing PROMPT STRUCTURE (static analysis of text), NOT testing LLM runtime behavior against planted bugs. Assertions should use domain-specific keywords AND agent names together (Story 4.1 review learning: discriminating assertions)

- [x] Task 6: Run full test suite (AC: all)
  - [x] 6.1 `go test ./runner/` — all prompt tests pass including new agent tests
  - [x] 6.2 `go test ./...` — no regressions across all packages
  - [x] 6.3 `go build ./...` — clean build with new embeds

## Dev Notes

### Architecture Constraints

- **File location**: `runner/prompts/agents/{quality,implementation,simplification,design-principles,test-coverage}.md`
- **Embedding**: `go:embed` in runner package — sub-agent prompts are static strings, loaded at compile time
- **No template processing**: Sub-agent prompts are plain text with NO `{{.Var}}` syntax and NO `__PLACEHOLDER__` markers. They are instruction-only files consumed by Claude Task tool inside the review session
- **Scope**: Sub-agents are Claude Task tool agents INSIDE the review session (Story 4.1). They are NOT separate Ralph processes. Ralph's Go code doesn't orchestrate sub-agents — the review prompt (Story 4.1) instructs Claude to spawn them
- **Dependency direction**: sub-agent prompts are static data in `runner/` package. No new dependencies needed
- **Two-stage assembly NOT needed**: Unlike execute.md and review.md which use `config.AssemblePrompt()`, sub-agent prompts are static text. Golden tests compare embedded string directly via `goldenTest(t, name, promptContent)`

### Story 4.1 Context (Previous Story)

Story 4.1 created `runner/prompts/review.md` — currently a minimal stub:
```
Review the code changes for the following task.

Task:
__TASK_CONTENT__
```

The review prompt references 5 sub-agents by name. Story 4.2 creates the actual prompt files these sub-agents will use. The review.md will be expanded in later stories (4.4, 4.5, 4.6) to include verification, clean handling, and findings write instructions.

**Story 4.1 Code Review Learnings (apply to 4.2):**
- **Defensive prompt guards**: prompts must contain explicit defensive instructions, not just keyword lists — test assertions should verify instructional sentences exist
- **Discriminating adversarial assertions**: assertions must check domain-specific keywords AND agent names together (e.g., "security" in quality agent's SCOPE, not just "security" anywhere) — use section-splitting to isolate SCOPE vs OUT-OF-SCOPE before asserting
- **DRY substring check helper**: when >3 tests check substrings against prompts, extract a reusable helper (pattern: `assertContains(t, section, substr, msg)`)

### Scope Boundary Design

The 5 agents have strict non-overlapping scopes (Devil's Advocate finding from epics elicitation):

| Agent | SCOPE | OUT-OF-SCOPE |
|-------|-------|--------------|
| quality | bugs, security, performance, error handling | AC compliance, simplification, DRY/KISS/SRP, tests |
| implementation | AC compliance, completeness, requirements | code quality, simplification, DRY/KISS/SRP, tests |
| simplification | readability, verbose code, dead code, simpler alternatives | bugs, AC compliance, DRY/KISS/SRP, tests |
| design-principles | DRY (duplication), KISS (over-engineering), SRP (responsibilities) | bugs, AC compliance, expression simplicity, tests |
| test-coverage | ATDD per AC (FR37), zero-skip (FR38), test quality | bugs, AC compliance, simplification, DRY/KISS/SRP |

**Key boundary**: simplification vs design-principles:
- simplification = expression-level / within-function (verbose constructs, dead code, simpler API usage)
- design-principles = cross-function / cross-file (duplication across modules, architectural abstractions, responsibilities)

### KISS/DRY/SRP Analysis

- **KISS**: Plain text prompts, table-driven `strings.Contains` tests, existing `goldenTest` helper
- **DRY**: Reuse `goldenTest` from `runner/prompt_test.go:16`; table-driven scope validation (not 5 separate functions); single `TestPrompt_Agent_ScopeExclusivity` for all overlap checks
- **SRP**: One prompt file per agent; scope tests separate from golden tests separate from adversarial tests

### Testing Strategy

Sub-agent prompts are **static text** (unlike execute.md which uses `text/template`). This means:
1. **No `config.AssemblePrompt` needed** — prompts are used as-is
2. **Golden file = direct comparison** — `goldenTest(t, "TestPrompt_Agent_Quality", agentQualityPrompt)`
3. **Scope tests use `strings.Contains`** — verify required keywords present per AC
4. **Adversarial = cross-agent exclusivity** — verify each domain keyword appears in exactly one agent's SCOPE section, not in others

**Test file**: All new tests go in existing `runner/prompt_test.go` (alongside execute prompt tests).

### Prompt Content Guidelines

Each sub-agent prompt should include:
1. **Role statement**: What this agent is
2. **SCOPE section**: Explicit list of what this agent checks (with examples)
3. **OUT-OF-SCOPE section**: Explicit list of what OTHER agents check (with agent name cross-references)
4. **Instructions**: How to report findings in this scope
5. **Finding format guidance**: Each finding should include ЧТО (what is wrong), ГДЕ (file:line), ПОЧЕМУ (why it matters), КАК (how to fix). This is preliminary guidance — Story 4.4 will formalize the verification contract. Include this structure in prompts now so sub-agents produce consistently formatted output from the start

### Sentinel Errors (do NOT duplicate)

Existing sentinels in `config/errors.go`: ErrMaxRetries, ErrMaxReviewCycles, ErrNoTasks
Existing sentinels in `runner/`: ErrNoCommit, ErrDirtyTree, ErrDetachedHead, ErrMergeInProgress

No new sentinels needed for this story.

### Project Structure Notes

**Files to CREATE:**
| File | Content |
|------|---------|
| `runner/prompts/agents/quality.md` | Quality agent prompt with SCOPE/OUT-OF-SCOPE |
| `runner/prompts/agents/implementation.md` | Implementation agent prompt with SCOPE/OUT-OF-SCOPE |
| `runner/prompts/agents/simplification.md` | Simplification agent prompt with SCOPE/OUT-OF-SCOPE |
| `runner/prompts/agents/design-principles.md` | Design-principles agent prompt with SCOPE/OUT-OF-SCOPE |
| `runner/prompts/agents/test-coverage.md` | Test-coverage agent prompt with SCOPE/OUT-OF-SCOPE |
| `runner/testdata/TestPrompt_Agent_Quality.golden` | Golden file (generated via -update) |
| `runner/testdata/TestPrompt_Agent_Implementation.golden` | Golden file (generated via -update) |
| `runner/testdata/TestPrompt_Agent_Simplification.golden` | Golden file (generated via -update) |
| `runner/testdata/TestPrompt_Agent_DesignPrinciples.golden` | Golden file (generated via -update) |
| `runner/testdata/TestPrompt_Agent_TestCoverage.golden` | Golden file (generated via -update) |

**Files to MODIFY:**
| File | Change |
|------|--------|
| `runner/runner.go` | Add 5 `//go:embed prompts/agents/<name>.md` declarations |
| `runner/prompt_test.go` | Add scope validation, golden file, and adversarial tests |

**Files to READ (not modify):**
| File | Purpose |
|------|---------|
| `runner/prompts/execute.md` | Reference for prompt structure pattern |
| `runner/prompts/review.md` | Verify review prompt references sub-agents |
| `config/prompt.go` | AssemblePrompt API (NOT used for sub-agents, but for reference) |

### References

- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.2] — AC and technical requirements
- [Source: docs/epics/epic-4-code-review-pipeline-stories.md#Story 4.1] — Review prompt that references sub-agents
- [Source: runner/prompts/execute.md] — Existing prompt pattern reference
- [Source: runner/prompts/review.md] — Current review prompt stub
- [Source: runner/prompt_test.go] — Existing prompt test patterns and goldenTest helper
- [Source: runner/runner.go:17-21] — Existing go:embed declarations for prompts
- [Source: config/prompt.go:25-40] — TemplateData struct (NOT used for sub-agents)
- [Source: docs/project-context.md#Prompt files] — "Prompt files = Go templates, not pure Markdown" (sub-agents are exception: plain text)
- [Source: .claude/rules/go-testing-patterns.md] — Testing pattern index (88 patterns)
- [Source: .claude/rules/test-assertions.md] — Assertion patterns (substring specificity, section-specific assertions)
- [Source: .claude/rules/code-quality-patterns.md] — Code quality patterns (doc comment accuracy, go fmt)

## Dev Agent Record

### Context Reference

<!-- This story was created by the create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- All 5 sub-agent prompts created with strict SCOPE/OUT-OF-SCOPE boundaries and cross-references
- 5 go:embed declarations added to runner/runner.go using "Prompt" naming (static text, not templates)
- 13 new tests: 5 scope validation, 5 golden file, 3 adversarial structure
- DRY helpers extracted: `assertContains`, `splitAgentSections`, `verifyAgentScope`, `scopeRef`
- ScopeExclusivity test covers 17 domain keywords across all 5 agents
- OutOfScopeCompleteness verifies each agent references all 4 other agents
- DetectionStructure checks Instructions section for domain-specific detection guidance
- All tests pass, no regressions, clean build

### File List

**Created:**
- `runner/prompts/agents/quality.md`
- `runner/prompts/agents/implementation.md`
- `runner/prompts/agents/simplification.md`
- `runner/prompts/agents/design-principles.md`
- `runner/prompts/agents/test-coverage.md`
- `runner/testdata/TestPrompt_Agent_Quality.golden`
- `runner/testdata/TestPrompt_Agent_Implementation.golden`
- `runner/testdata/TestPrompt_Agent_Simplification.golden`
- `runner/testdata/TestPrompt_Agent_DesignPrinciples.golden`
- `runner/testdata/TestPrompt_Agent_TestCoverage.golden`

**Modified:**
- `runner/runner.go` — 5 go:embed declarations for sub-agent prompts
- `runner/prompt_test.go` — 13 new tests + 4 helper functions
- `docs/sprint-artifacts/sprint-status.yaml` — 4-2 status: in-progress → review

## Change Log

- 2026-02-27: Implemented Story 4.2 — created 5 sub-agent prompts with strict scope boundaries, go:embed declarations, scope validation tests, golden file tests, and adversarial structure tests
