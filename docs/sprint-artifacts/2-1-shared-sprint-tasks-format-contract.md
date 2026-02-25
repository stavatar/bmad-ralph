# Story 2.1: Shared Sprint-Tasks Format Contract

Status: done

## Story

As a developer of ralph bridge and ralph run,
I want the sprint-tasks.md format defined once as a shared contract,
so that bridge output and runner parsing are always in sync.

## Acceptance Criteria

1. **Format document exists** at `config/shared/sprint-tasks-format.md` defining the complete sprint-tasks.md format:
   - Task syntax: `- [ ] Task description [GATE]`
   - Done syntax: `- [x] Task description`
   - Source field: `  source: stories/file.md#AC-N` (indented under task line)
   - Feedback syntax: `> USER FEEDBACK: text`
   - Section headers: `## Epic Name` grouping
   - Service task prefixes: `[SETUP]`, `[VERIFY]`, `[E2E]`

2. **Source field regex** defined: `^\s+source:\s+\S+#\S+`

3. **Embedded via go:embed** in config package — `config.SprintTasksFormat()` returns embedded content as string

4. **Bridge imports format** for inclusion in bridge prompt (Story 2.2 will use it)

5. **Runner uses same constants** (TaskOpen, TaskDone from Story 1.6) to parse — no new constants needed for runner

6. **Tests in BOTH config and bridge packages** verify:
   - Format content is non-empty
   - Format contains key markers (TaskOpen, TaskDone, GateTag)
   - Structural Rule #8: shared contract tested from both consumer sides

## Tasks / Subtasks

- [x] Task 1: Create format contract document (AC: 1, 2)
  - [x] 1.1 Create `config/shared/sprint-tasks-format.md` — this file is embedded VERBATIM into bridge and execute prompts for Claude AI, so write it as clear LLM-consumable instructions with examples, not as human reference docs
  - [x] 1.2 Include task syntax, done syntax, source field, feedback, headers, service prefixes
  - [x] 1.3 Include source field regex pattern: `^\s+source:\s+\S+#\S+`
  - [x] 1.4 Include concrete examples for each syntax element (Claude needs exact patterns to follow)
  - [x] 1.5 Remove `.gitkeep` from `config/shared/` (replaced by real file)

- [x] Task 2: Add go:embed and SprintTasksFormat() function (AC: 3)
  - [x] 2.1 Create `config/format.go` with `import _ "embed"` (required for go:embed) and `//go:embed shared/sprint-tasks-format.md` — use `var sprintTasksFormat string` (string type, NOT []byte — prompts use string throughout, see AssemblePrompt signature)
  - [x] 2.2 Implement `func SprintTasksFormat() string` returning embedded content
  - [x] 2.3 Add `SourceFieldRegex` compiled pattern to `config/constants.go` — note: uses `^\s+` (mandatory indent) unlike TaskOpenRegex `^\s*` (optional), because source field is always indented under its parent task line

- [x] Task 3: Config package tests in `config/format_test.go` (AC: 6)
  - [x] 3.1 `TestSprintTasksFormat_NonEmpty` — format content is non-empty string
  - [x] 3.2 `TestSprintTasksFormat_ContainsMarkers` — ONE table-driven test verifying ALL required markers: TaskOpen, TaskDone, GateTag, FeedbackPrefix, source field syntax, service prefixes [SETUP]/[VERIFY]/[E2E]. Do NOT create separate functions per marker — that is the "standalone tests duplicating table cases" anti-pattern from Epic 1 retro
  - [x] 3.3 `TestSourceFieldRegex` — table-driven regex test with cases:
    - valid: `"  source: stories/auth.md#AC-3"` (two-space indent)
    - valid: `"    source: stories/api.md#AC-1"` (four-space indent)
    - valid: `"\tsource: story.md#SETUP"` (tab indent)
    - invalid: `"source: stories/auth.md#AC-3"` (no indent — MUST fail, source is always under parent task)
    - invalid: `"  source: stories/auth.md"` (missing # separator)
    - invalid: `"  source: stories/auth.md#"` (empty identifier after #)
    - invalid: `""` (empty line)
    - invalid: `"  no source here"` (no source: keyword)

- [x] Task 4: Bridge package contract tests (AC: 4, 6)
  - [x] 4.1 Create `bridge/format_test.go` — first test file in bridge package, establishes Epic 2 testing pattern
  - [x] 4.2 `TestSprintTasksFormat_BridgeConsumer_NonEmpty` — import config, call `config.SprintTasksFormat()`, verify non-empty
  - [x] 4.3 `TestSprintTasksFormat_BridgeConsumer_ContainsMarkers` — table-driven: verify TaskOpen, TaskDone, GateTag, FeedbackPrefix all present (full contract, not partial)
  - [x] 4.4 `TestSprintTasksFormat_BridgeConsumer_RegexAccessible` — verify `config.SourceFieldRegex` compiles and is usable from bridge package (cross-package export check)

## Dev Notes

### Architecture Context

- **config = leaf package**: depends on nothing, imported by bridge and runner
- **Dependency direction**: `bridge → config`, `runner → config` (never reverse)
- **Shared contract pattern** (Structural Rule #8): format defined once in config, tested from both consumer packages
- **This is the "hub node"** identified in Graph of Thoughts — 5 writers/readers depend on this contract
- The format document is NOT a Go file — it's a markdown file embedded via `go:embed` and included VERBATIM in bridge prompt (Story 2.2) and execute prompt (Story 3.1) for Claude AI. Write it as clear LLM instructions with exact format examples, not as human reference documentation

### Existing Code to Build On

| File | What exists | What to do |
|------|------------|------------|
| `config/constants.go` | TaskOpen, TaskDone, GateTag, FeedbackPrefix consts + TaskOpenRegex, TaskDoneRegex, GateTagRegex patterns | Add SourceFieldRegex pattern |
| `config/constants_test.go` | Tests for all 4 constants + 3 regex patterns | Add SourceFieldRegex tests |
| `config/shared/.gitkeep` | Empty placeholder | Replace with sprint-tasks-format.md |
| `config/config.go` | go:embed for defaults.yaml | New file `format.go` for format embed (separate concern) |
| `bridge/bridge.go` | Stub `Run()` returning "not implemented" | No changes — bridge tests import config |

### Implementation Constraints

- **go:embed directive**: `//go:embed shared/sprint-tasks-format.md` — embed path is relative to Go source file
- **Function signature**: `func SprintTasksFormat() string` — returns string (not []byte) because prompts use string type throughout
- **Regex pattern var**: `var SourceFieldRegex = regexp.MustCompile(...)` in `constants.go` at package scope. Uses `^\s+` (mandatory indent), NOT `^\s*` (optional) like TaskOpenRegex — source field is always indented under parent task
- **No new dependencies**: only Go stdlib + existing config package patterns
- **UTF-8, LF line endings**: `sed -i 's/\r$//'` after creating files on NTFS/WSL

### Naming & Testing Standards

- **Files**: `config/format.go` (embed + accessor), `config/format_test.go`, `bridge/format_test.go`
- **Function**: `SprintTasksFormat` (exported PascalCase). Regex var: `SourceFieldRegex` (matches `TaskOpenRegex` pattern)
- **Test names**: `Test<FunctionName>_<Scenario>` — e.g., `TestSprintTasksFormat_NonEmpty`, `TestSourceFieldRegex`
- **Table-driven** by default, Go stdlib `if got != want { t.Errorf(...) }` assertions (no testify)
- **strings.Contains** for marker checks — more resilient than exact match
- **SourceFieldRegex tests**: table-driven, symmetric cases (indent variants, missing hash, no indent, empty)
- **Bridge tests import config directly**: proves cross-package contract accessibility — Structural Rule #8
- **ANTI-PATTERN**: Do NOT create separate test functions for each marker (e.g., `TestContainsTaskOpen`, `TestContainsTaskDone`...). Use ONE table-driven test. This was caught 3 times in Epic 1

### Previous Story Patterns to Apply

- **go:embed pattern** (Story 1.4): use `string` type for prompt-compatible embed, `import _ "embed"` required. See `config/config.go:13-14` for existing `[]byte` pattern
- **Regex pattern** (Story 1.6): `var X = regexp.MustCompile(...)` at package scope, symmetric table-driven tests (indent variants, malformed cases)
- **Anti-pattern** (Epic 1 retro, caught 3x): do NOT create separate test functions when a single table-driven test covers all cases
- **WSL/NTFS**: always `sed -i 's/\r$//'` after Write tool creates files

### Project Structure Notes

- `config/shared/` directory already exists (with `.gitkeep`) — architecture pre-planned this location
- `config/format.go` is a new file — keeps embed+accessor separate from main config.go
- `bridge/format_test.go` is first test file in bridge package — establishes testing pattern for Epic 2

### References

- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md#Story 2.1]
- [Source: docs/architecture/project-structure-boundaries.md — config/shared/ directory]
- [Source: docs/architecture/implementation-patterns.md — go:embed patterns]
- [Source: docs/project-context.md#String Constants]
- [Source: config/constants.go — existing marker constants and regex patterns]
- [Source: config/config.go:13-14 — go:embed pattern for defaults.yaml]
- [Source: docs/sprint-artifacts/epic-1-retro-2026-02-26.md — Epic 1 learnings]

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Reviewer Model:** Claude Opus 4.6
**Review Outcome:** Approve with fixes applied

### Findings Summary

| # | Severity | Description | Status |
|---|----------|-------------|--------|
| M1 | Medium | SourceFieldRegex test in format_test.go breaks constants_test.go convention | [x] Fixed |
| M2 | Medium | Regex pattern duplication in doc and code without synchronization test | [x] Fixed |
| M3 | Medium | Bridge tests check only 4/8 markers — incomplete Structural Rule #8 | [x] Fixed |
| L1 | Low | Missing SourceFieldRegex edge cases: no-space-after-colon, capital-S | [x] Fixed |
| L2 | Low | No init() guard for empty embedded content in format.go | Accepted — go:embed guarantees file at build time |
| L3 | Low | Bare test function name TestSourceFieldRegex | Accepted — consistent with existing regex test names |

### Fixes Applied

- Moved `TestSourceFieldRegex` from `format_test.go` to `constants_test.go` alongside other regex tests (M1)
- Added regex pattern string sync test case to `TestSprintTasksFormat_ContainsMarkers` (M2)
- Expanded bridge `ContainsMarkers` test from 4 to 8 markers (source:, [SETUP], [VERIFY], [E2E]) (M3)
- Added 2 edge cases: no-space-after-colon, capital-S (L1)

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow 2026-02-26 -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No debug issues encountered.

### Completion Notes List

- Task 1: Created `config/shared/sprint-tasks-format.md` as LLM-consumable format specification with exact syntax examples for task/done/gate/source/feedback/service-prefix elements. Removed `.gitkeep` placeholder.
- Task 2: Created `config/format.go` with `//go:embed shared/sprint-tasks-format.md` using `string` type (matches prompt assembly signature). Added `SourceFieldRegex` to `config/constants.go` with `^\s+` mandatory indent (distinct from TaskOpenRegex `^\s*`).
- Task 3: Created `config/format_test.go` with 2 test functions: NonEmpty, ContainsMarkers (single table-driven, 9 markers incl regex sync).
- Task 4: Created `bridge/format_test.go` (first test file in bridge package) with 3 tests verifying cross-package contract accessibility per Structural Rule #8, all 8 markers.
- All 6 ACs satisfied. Full regression suite passes (0 failures).
- Code review (Claude Opus 4.6): 6 findings (3M, 3L) — all 3M and 1L fixed, 2L accepted as-is.

### Change Log

- 2026-02-26: Implemented Story 2.1 — shared sprint-tasks format contract with go:embed, SourceFieldRegex, and cross-package tests
- 2026-02-26: Code review fixes — moved SourceFieldRegex test to constants_test.go, added regex sync test, expanded bridge markers to 8/8, added 2 edge cases

### File List

- config/shared/sprint-tasks-format.md (new)
- config/format.go (new)
- config/format_test.go (new)
- config/constants.go (modified — added SourceFieldRegex)
- config/constants_test.go (modified — added TestSourceFieldRegex with 10 cases)
- config/shared/.gitkeep (deleted)
- bridge/format_test.go (new)
- docs/sprint-artifacts/2-1-shared-sprint-tasks-format-contract.md (modified)
- docs/sprint-artifacts/sprint-status.yaml (modified)
