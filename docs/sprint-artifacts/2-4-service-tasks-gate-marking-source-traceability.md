# Story 2.4: Service Tasks, Gate Marking, Source Traceability

Status: done

## Story

As a developer using ralph bridge,
I want bridge prompt enhanced with service task generation, gate marking, and source traceability instructions,
so that sprint-tasks.md includes all supporting infrastructure. (FR3, FR5, FR5a)

## Acceptance Criteria

1. **[SETUP] service tasks generated BEFORE implementation tasks:**
   Given a story with dependencies on new frameworks, when bridge generates sprint-tasks.md, then `[SETUP]` service tasks appear BEFORE implementation tasks (e.g., `"- [ ] [SETUP] Install and configure testing framework"`), with `source: stories/<file>.md#SETUP`.

2. **[VERIFY] integration tasks appear after related tasks:**
   Given a story spans multiple parts of a feature, when bridge generates sprint-tasks.md, then `[VERIFY]` integration verification tasks appear AFTER related implementation tasks (e.g., `"- [ ] [VERIFY] Verify login API + frontend integration"`), with `source: stories/<file>.md#VERIFY`.

3. **[GATE] tag on first epic task:**
   Given a story is the first in an epic, when bridge generates sprint-tasks.md, then the first task of the epic has `[GATE]` tag (e.g., `"- [ ] Implement user model [GATE]"`).

4. **[GATE] tag on user-visible milestones:**
   Given user-visible milestones exist in stories, when bridge generates sprint-tasks.md, then milestone tasks have `[GATE]` tag.

5. **Source traceability on EVERY task:**
   EVERY task has `source:` field indented under the task line (e.g., `"  source: stories/auth.md#AC-3"`). Service tasks use `#SETUP`, `#VERIFY`, `#E2E` identifiers.

6. **Golden file tests verify enriched prompt:**
   Updated golden files and assertions verify the prompt contains detailed instructions for:
   - Story with dependencies -> `[SETUP]` task placement rules
   - Multi-part story -> `[VERIFY]` task placement rules
   - First epic task -> `[GATE]` tag placement rules
   - All tasks -> `source:` field scoping rules

## Tasks / Subtasks

- [x] Task 1: Enrich `## Gate Marking` section in bridge.md (AC: #3, #4)
  - [x] 1.1 In `bridge/prompts/bridge.md`, locate the `## Gate Marking` section. PRESERVE existing skeleton text and ADD detailed FR3 placement rules below it:
    - First task of each epic MUST have `[GATE]` appended
    - User-visible milestone tasks MUST have `[GATE]` appended
    - Define "user-visible milestone": deployments to production/staging, feature completions visible to end users, security-critical changes, new UI screens/workflows
    - `[GATE]` placement: end of task line (e.g., `- [ ] Task description [GATE]`)
    - Gate purpose: pauses automated execution for human approval
    - Detection guidance for Claude: instruct to infer "first in epic" from story numbering (e.g., "Story 1.1" = first), epic context, or explicit story metadata. Instruct to identify milestones from keywords like "deploy", "release", "production", "visible to user"
  - [x] 1.2 Remove the `<!-- Story 2.4 enriches with detailed gate placement rules (FR3) -->` HTML comment

- [x] Task 2: Enrich `## Service Tasks` section in bridge.md (AC: #1, #2)
  - [x] 2.1 In `bridge/prompts/bridge.md`, locate the `## Service Tasks` section. PRESERVE existing skeleton text (including test framework presence check on lines 45-46) and ADD detailed FR5 detection criteria:
    - `[SETUP]` tasks: detect story dependencies on NEW frameworks, libraries, or infrastructure not yet in the project. Generate `[SETUP]` tasks BEFORE any implementation tasks that depend on them. Examples: install testing framework, configure database, initialize project tooling.
    - `[VERIFY]` tasks: detect stories spanning multiple components or integration points. Generate `[VERIFY]` tasks after related implementation tasks they verify. Examples: verify API + frontend integration, validate contract compliance, check cross-service data flow.
    - `[E2E]` tasks: detect critical user flows or multi-step workflows. Generate `[E2E]` tasks at the end of the epic section covering the complete flow. Examples: end-to-end registration flow, full checkout pipeline.
    - Ordering rule: `[SETUP]` -> implementation tasks -> `[VERIFY]` -> `[E2E]`
  - [x] 2.2 Remove the `<!-- Story 2.4 enriches with detailed service task detection criteria (FR5) -->` HTML comment

- [x] Task 3: Enrich `## Source Traceability` section in bridge.md (AC: #5)
  - [x] 3.1 In `bridge/prompts/bridge.md`, locate the `## Source Traceability` section. PRESERVE existing skeleton text and ADD detailed FR5a scoping rules:
    - EVERY task (implementation, test, service) MUST have an indented `source:` field on the next line
    - Format for implementation/test tasks: `source: stories/<filename>.md#AC-<N>` where N is the AC number
    - Format for service tasks: `source: stories/<filename>.md#SETUP`, `#VERIFY`, or `#E2E`
    - If a task satisfies multiple ACs, use the PRIMARY AC number
    - The `source:` line MUST be indented (2 spaces) under its parent task — never on the same line
    - The path and identifier are separated by `#`, identifier MUST be non-empty
    - Add negative example: `- [ ] Task source: file.md#AC-1` is WRONG — source MUST be on a separate indented line
  - [x] 3.2 Remove the `<!-- Story 2.4 enriches with detailed source field scoping rules (FR5a) -->` HTML comment
  - [x] 3.3 Verify ALL examples in enriched sections match `SourceFieldRegex = ^\s+source:\s+\S+#\S+` — no spaces in path, non-empty identifier after `#`

- [x] Task 4: Update prompt golden file tests (AC: #6)
  - [x] 4.1 Add enriched content assertions to `TestBridgePrompt_Creation` in `bridge/prompt_test.go`. Use COMPOUND substrings unique to enriched content (avoid generic words that collide with existing skeleton — "BEFORE" already appears in red-green principle line 28):
    - `"first task of each epic"` — gate placement rule (unique compound)
    - `"user-visible milestone"` — gate milestone rule
    - `"BEFORE any implementation tasks"` — SETUP ordering (NOT bare "BEFORE" which collides with red-green "BEFORE the implementation task")
    - `"after related implementation"` — VERIFY ordering (lowercase, unique compound)
    - `"stories/<filename>.md#AC-<N>"` — source AC format template
    - `"#SETUP"` — service source identifier
    - `"#VERIFY"` — service source identifier
    - `"#E2E"` — service source identifier
  - [x] 4.2 Add same enriched assertions to `TestBridgePrompt_Merge` (maintain assertion symmetry per Story 2.2 review learning)
  - [x] 4.3 Regenerate golden files: `go test -update ./bridge/...`
  - [x] 4.4 Verify golden files stable: `go test ./bridge/...`

- [x] Task 5: Run all tests and verify (AC: all)
  - [x] 5.1 Run `go test ./bridge/...` — all bridge tests pass (prompt + Run tests)
  - [x] 5.2 Run `go test ./config/...` — no regressions
  - [x] 5.3 Run `go test ./cmd/ralph/...` — compile check
  - [x] 5.4 Run `go vet ./...` — no vet issues

## Dev Notes

### Scope: Prompt Enrichment Only

This story modifies ONLY the bridge prompt template (`bridge/prompts/bridge.md`) and its tests. No Go code changes to `bridge/bridge.go` or `bridge_test.go`. The epic tech notes confirm: "These are prompt instructions, not bridge Go logic — Claude generates these based on prompt."

Story 2.2 created skeleton sections with HTML comment markers `<!-- Story 2.4 enriches... -->`. This story ENRICHES those sections by adding detailed instructions below existing correct skeleton text, then removes the HTML comments. Do NOT delete the existing skeleton content — it is correct and aligns with the format contract.

### Architecture: Bridge Prompt = Format Concern, Runner = Execution Concern

From Story 2.2 review learning: "Bridge always generates [GATE] markers (format concern); runner controls gate behavior (execution concern)." The bridge prompt instructs Claude to ALWAYS generate [GATE] markers on appropriate tasks. The runner decides whether to pause execution at gates.

Do NOT add template conditionals like `{{- if .GatesEnabled}}` — this was a Story 2.2 review finding (HIGH priority). Gate marking is unconditional in the prompt.

### Current Bridge Prompt Structure (Sections to Enrich)

The current `bridge/prompts/bridge.md` has three sections with Story 2.4 HTML comments. Locate by `## Section Header`, NOT by line number (line numbers shift after edits):

1. **`## Gate Marking`**: Brief skeleton — "Append `[GATE]` to the FIRST task of each epic and to user-visible milestone tasks." Has `<!-- Story 2.4 enriches ... (FR3) -->`.

2. **`## Service Tasks`**: Brief skeleton with [SETUP], [VERIFY], [E2E] prefixes and test framework check (2 lines about test framework — PRESERVE these). Has `<!-- Story 2.4 enriches ... (FR5) -->`.

3. **`## Source Traceability`**: Brief skeleton — "Every task MUST have an indented `source:` field." Has `<!-- Story 2.4 enriches ... (FR5a) -->`.

### Enrichment Strategy: ADD, Do NOT Replace

The existing skeleton content is CORRECT — it aligns with `config/shared/sprint-tasks-format.md`. For each section:
1. KEEP existing skeleton text intact
2. ADD detailed rules, detection criteria, ordering rules, and examples BELOW the existing text
3. REMOVE the `<!-- Story 2.4 enriches... -->` HTML comment
4. PRESERVE test framework presence check lines in Service Tasks section

### "First in Epic" Detection Guidance

Bridge prompt receives story content but no explicit metadata like `epic_num` or `story_num`. Claude must INFER "first in epic" from:
- Story numbering in content (e.g., "Story 1.1: ..." = first story)
- Epic grouping context in story headers
- Explicit mentions like "This is the first story in Epic X"

The enriched Gate Marking section must instruct Claude on these heuristics.

### Sprint-Tasks Format Contract Alignment

The enriched sections must remain consistent with `config/shared/sprint-tasks-format.md` (Story 2.1). Key constants from `config/constants.go`:
- `TaskOpen = "- [ ]"`, `TaskDone = "- [x]"`, `GateTag = "[GATE]"`
- `SourceFieldRegex = ^\s+source:\s+\S+#\S+`
- Service prefixes: `[SETUP]`, `[VERIFY]`, `[E2E]`

**CRITICAL: Regex alignment.** ALL `source:` field examples added to the enriched prompt MUST match `SourceFieldRegex`. This means: indented with spaces/tabs, `source:` keyword, then `<path>#<identifier>` with NO spaces in path or identifier. Malformed examples (spaces in path, missing `#`, empty identifier) would teach Claude to generate output the runner scanner can't parse.

### Test Assertions: Enriched Content Verification

The existing `TestBridgePrompt_Creation` and `TestBridgePrompt_Merge` tests have 17 and 19 assertions respectively. Story 2.4 adds ~8 new assertions per test to verify the enriched content.

**CRITICAL: Assertion substring specificity.** The word "BEFORE" already appears in bridge.md line 28 (red-green principle: "Create a TEST task BEFORE the implementation task"). Using bare "BEFORE" as assertion would match the WRONG section. Use compound substrings unique to enriched content:

| Assertion Purpose | Substring | Why Unique |
|---|---|---|
| Gate first-of-epic | `"first task of each epic"` | Not in skeleton (skeleton says "FIRST task" without "of each epic") |
| Gate milestone | `"user-visible milestone"` | Same text exists in skeleton — OK, verifies it's preserved |
| SETUP ordering | `"BEFORE any implementation tasks"` | Skeleton has "BEFORE the implementation task" (red-green) — the "any" makes it unique |
| VERIFY ordering | `"after related implementation"` | Not in skeleton at all |
| Source AC format | `"stories/<filename>.md#AC-<N>"` | Template format, not in skeleton |
| Service source #SETUP | `"#SETUP"` | In skeleton briefly, enriched expands |
| Service source #VERIFY | `"#VERIFY"` | In skeleton briefly, enriched expands |
| Service source #E2E | `"#E2E"` | In skeleton briefly, enriched expands |

Per Story 2.2 review learning: maintain assertion symmetry between Creation and Merge tests. Both must check the same set of assertions.

### Golden File Update Process

Golden files will change because prompt content changes. Process:
1. Edit `bridge/prompts/bridge.md`
2. Run `go test -update ./bridge/...` to regenerate golden files
3. Run `go test ./bridge/...` to verify stability
4. Review golden file diffs for correctness

### Review Learnings to Apply Proactively

**From Story 2.2 (directly relevant):**
- Don't add template conditionals not mandated by AC (no extra `{{- if}}` blocks)
- Test assertion symmetry: Creation and Merge tests must have same base assertions
- Consistent negative check patterns: use `present bool` struct field
- Substring assertions must be section-specific (unique to target section)
- When prompts say "DO NOT use numbered lists", the prompt itself uses bullet points

**From Story 2.1:**
- Duplicated content between code and docs needs sync test (already handled by `TestBridgePrompt_ContainsFormatContract`)
- Structural Rule #8: shared contract tested from both consumer packages (already covered)

**From Story 2.3:**
- Separator assertions must use full pattern `"\n\n---\n\n"` (not relevant to prompt tests)
- Full output comparison when source is deterministic (golden file does this)

### Project Structure Notes

- `bridge/prompts/bridge.md` — modify (enrich 3 sections with detailed rules, remove 3 HTML comments, preserve skeleton text)
- `bridge/prompt_test.go` — modify (add ~8 assertions per test function with compound substrings)
- `bridge/testdata/TestBridgePrompt_Creation.golden` — regenerated (via -update)
- `bridge/testdata/TestBridgePrompt_Merge.golden` — regenerated (via -update)
- No new files, no new dependencies, no package boundary changes

### References

- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md — Story 2.4 AC, lines 159-207]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md — Prompt = instructions to Claude]
- [Source: docs/architecture/project-structure-boundaries.md — bridge/prompts/ layout]
- [Source: docs/project-context.md — Two-stage prompt assembly, bridge role]
- [Source: config/constants.go — TaskOpen, TaskDone, GateTag, SourceFieldRegex]
- [Source: config/shared/sprint-tasks-format.md — Format contract with examples]
- [Source: bridge/prompts/bridge.md — Current prompt with skeleton sections]
- [Source: bridge/prompt_test.go — Existing assertions to extend]
- [Source: docs/sprint-artifacts/2-2-bridge-prompt-template.md — Review learnings, skeleton design]
- [Source: docs/sprint-artifacts/2-3-bridge-logic-core-conversion.md — bridge.Run implementation, error patterns]

### Existing Code to Build On

| File | Status | Description |
|------|--------|-------------|
| `bridge/prompts/bridge.md` | modify | Enrich 3 sections with detailed rules, remove HTML comments, preserve skeleton |
| `bridge/prompt_test.go` | modify | Add ~8 enriched content assertions per test function (compound substrings) |
| `bridge/testdata/TestBridgePrompt_Creation.golden` | regenerated | Via `go test -update` |
| `bridge/testdata/TestBridgePrompt_Merge.golden` | regenerated | Via `go test -update` |

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

None — clean implementation, no debugging needed.

### Implementation Plan

Prompt-only enrichment story. Three sections in `bridge/prompts/bridge.md` enriched with detailed rules per FR3 (gate marking), FR5 (service tasks), FR5a (source traceability). Skeleton content preserved, HTML comment markers removed. Golden files regenerated, 8 new test assertions added per test function (Creation + Merge symmetric).

### Completion Notes List

- Enriched Gate Marking section with placement rules, "first in epic" detection heuristics, and user-visible milestone criteria (AC #3, #4)
- Enriched Service Tasks section with SETUP/VERIFY/E2E detection criteria, examples, and ordering rule (AC #1, #2)
- Enriched Source Traceability section with scoping rules, format templates, and negative example (AC #5)
- Added 9 enriched content assertions to both TestBridgePrompt_Creation and TestBridgePrompt_Merge (assertion symmetry maintained per Story 2.2 review learning) (AC #6)
- All source field examples verified against SourceFieldRegex
- Golden files regenerated and stable
- All tests pass: bridge, config, cmd/ralph, go vet

### Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Review Outcome:** Changes Requested (3 MEDIUM, 4 LOW)
**Reviewer Model:** Claude Opus 4.6 (same session, adversarial review)

#### Action Items

- [x] [M1] Replace non-unique assertion `"stories/<filename>.md#AC-<N>"` with `"PRIMARY AC number"` (unique to enriched content) — `bridge/prompt_test.go`
- [x] [M2] Fix conflicting MUST precision in source indentation: separate requirement from guidance — `bridge/prompts/bridge.md:115`
- [x] [M3] Add missing negative source example assertion `"Task source: file.md#AC-1"` to both test functions — `bridge/prompt_test.go`
- [ ] [L1] Redundant restatement in Gate Marking enrichment (first two bullets repeat skeleton) — cosmetic, low priority
- [ ] [L2] 5 of 8 enriched assertions overlap with skeleton content — golden file covers, low priority
- [ ] [L3] File List labels untracked files as "modified" — multi-story uncommitted development nuance
- [ ] [L4] Enriched `###` headings add structural complexity to prompt — acceptable trade-off for clarity

### Change Log

- 2026-02-26: Enriched bridge prompt with gate marking, service tasks, and source traceability rules (Story 2.4)
- 2026-02-26: Code review fixes — replaced non-unique assertion, fixed source indent phrasing, added negative example assertion (3 MEDIUM resolved)

### File List

- `bridge/prompts/bridge.md` — new/added (created Story 2.2, enriched Story 2.4, review fix M2)
- `bridge/prompt_test.go` — new/added (created Story 2.2, enriched Story 2.4, review fixes M1+M3)
- `bridge/testdata/TestBridgePrompt_Creation.golden` — new/added (regenerated)
- `bridge/testdata/TestBridgePrompt_Merge.golden` — new/added (regenerated)
- `docs/sprint-artifacts/sprint-status.yaml` — modified (status: ready-for-dev → in-progress → review → done)
- `docs/sprint-artifacts/2-4-service-tasks-gate-marking-source-traceability.md` — new/added (tasks checked, Dev Agent Record, review section)
