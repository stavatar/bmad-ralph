# Story 1.6: Constants & Regex Patterns

Status: Done

## Story

As a developer,
I want all sprint-tasks.md markers and regex patterns defined as constants,
so that bridge, runner, and scanner use identical patterns without duplication.

## Acceptance Criteria

```gherkin
Given constants need to be defined for sprint-tasks.md parsing
When constants.go is created in config package
Then the following string constants exist:
  | Constant       | Value              |
  | TaskOpen       | "- [ ]"            |
  | TaskDone       | "- [x]"            |
  | GateTag        | "[GATE]"           |
  | FeedbackPrefix | "> USER FEEDBACK:" |

And the following compiled regex patterns exist:
  | Pattern          | Matches                        |
  | TaskOpenRegex    | Lines starting with "- [ ]"    |
  | TaskDoneRegex    | Lines starting with "- [x]"    |
  | GateTagRegex     | Lines containing "[GATE]"      |

And regex patterns are compiled via regexp.MustCompile at package scope
And all patterns have unit tests with positive and negative cases
And edge cases tested: indented tasks, tasks with trailing content, empty lines
```

## Tasks / Subtasks

- [x] Task 1: Create config/constants.go with string constants and regex patterns (AC: all constants and patterns defined)
  - [x] 1.1 Create `config/constants.go` with `package config` declaration
  - [x] 1.2 Define string constants in a `const` block: `TaskOpen`, `TaskDone`, `GateTag`, `FeedbackPrefix`
  - [x] 1.3 Import `regexp` package
  - [x] 1.4 Define compiled regex variables at package scope via `regexp.MustCompile`:
    - `TaskOpenRegex` = `^\s*- \[ \]` — matches lines starting with optional whitespace + "- [ ]" (trailing content optional)
    - `TaskDoneRegex` = `^\s*- \[x\]` — matches lines starting with optional whitespace + "- [x]" (trailing content optional)
    - `GateTagRegex` = `\[GATE\]` — matches "[GATE]" anywhere in a line
  - [x] 1.5 Run `sed -i 's/\r$//' config/constants.go` (CRLF fix)

- [x] Task 2: Create config/constants_test.go with table-driven tests (AC: positive and negative cases, edge cases)
  - [x] 2.1 Create `config/constants_test.go` with `package config` declaration
  - [x] 2.2 Write `TestTaskOpenRegex` — table-driven with cases:
    - Positive: `"- [ ] Implement feature"` (standard task)
    - Positive: `"  - [ ] Subtask indented"` (space-indented task)
    - Positive: `"\t- [ ] Tab indented"` (tab-indented task)
    - Positive: `"- [ ]"` (marker only, no trailing content — still an open task)
    - Negative: `"- [x] Done task"` (done task, not open)
    - Negative: `""` (empty line)
    - Negative: `"Some text - [ ] embedded"` (marker not at line start)
    - Negative: `"- [] Missing space"` (malformed marker)
  - [x] 2.3 Write `TestTaskDoneRegex` — table-driven with cases:
    - Positive: `"- [x] Completed task"` (standard done)
    - Positive: `"  - [x] Indented done"` (space-indented)
    - Positive: `"- [x]"` (marker only, no trailing content)
    - Positive: `"- [x] Task with [GATE] tag"` (done task with gate)
    - Negative: `"- [ ] Open task"` (open, not done)
    - Negative: `""` (empty line)
    - Negative: `"- [X] Uppercase X"` (case-sensitive: convention uses lowercase `x`)
  - [x] 2.4 Write `TestGateTagRegex` — table-driven with cases:
    - Positive: `"- [ ] Setup environment [GATE]"` (gate at end)
    - Positive: `"[GATE] First line"` (gate at start)
    - Positive: `"- [x] Done [GATE] tagged"` (gate in middle of done task)
    - Negative: `"- [ ] Normal task"` (no gate tag)
    - Negative: `""` (empty line)
    - Negative: `"[gate] lowercase"` (case-sensitive)
    - Negative: `"GATE without brackets"` (missing brackets)
  - [x] 2.5 Write `TestTaskOpen_Value`, `TestTaskDone_Value`, `TestGateTag_Value`, `TestFeedbackPrefix_Value` — guard tests verifying exact constant values:
    - `TaskOpen == "- [ ]"`
    - `TaskDone == "- [x]"`
    - `GateTag == "[GATE]"`
    - `FeedbackPrefix == "> USER FEEDBACK:"`
    NOTE: Each test uses the real exported constant name as the "Type" in `Test<Type>_Value` per naming convention. These are simple one-liner guard tests, not table-driven.
  - [x] 2.6 Run `sed -i 's/\r$//' config/constants_test.go` (CRLF fix)

- [x] Task 3: Validation (AC: all tests pass, config remains leaf)
  - [x] 3.1 `go build ./...` passes
  - [x] 3.2 `go test ./config/...` passes (existing 31 top-level test functions + 7 new = 38 total)
  - [x] 3.3 `go vet ./...` passes
  - [x] 3.4 Verify `config` remains leaf package (only new import: `regexp`)
  - [x] 3.5 Verify no new external dependencies added
  - [x] 3.6 Verify constants and regex patterns are exported (PascalCase) for cross-package use

## Dev Notes

### Scope

This story creates **two new files** in the config package:
1. `config/constants.go` — string constants + compiled regex patterns
2. `config/constants_test.go` — table-driven tests for all patterns

No modifications to existing files. The config package gains `regexp` as a new stdlib import (not an external dependency).

### Implementation Guide

**constants.go structure:**
```go
package config

import "regexp"

// String constants for sprint-tasks.md markers.
// Used by bridge (generation) and runner (scanning) packages.
const (
	TaskOpen       = "- [ ]"
	TaskDone       = "- [x]"
	GateTag        = "[GATE]"
	FeedbackPrefix = "> USER FEEDBACK:"
)

// Compiled regex patterns for sprint-tasks.md line scanning.
// All patterns are compiled at package init via MustCompile.
// Runner scanner uses: strings.Split(content, "\n") + regex match per line.
var (
	TaskOpenRegex = regexp.MustCompile(`^\s*- \[ \]`)
	TaskDoneRegex = regexp.MustCompile(`^\s*- \[x\]`)
	GateTagRegex  = regexp.MustCompile(`\[GATE\]`)
)
```

### Regex Pattern Design Decisions

**TaskOpenRegex: `^\s*- \[ \]`**
- `^` — anchored at line start (scanner processes line-by-line after `strings.Split`)
- `\s*` — optional leading whitespace for indented subtasks (sprint-tasks.md allows nested tasks, both spaces and tabs)
- `- \[ \]` — literal "- [ ]" WITHOUT trailing space requirement
- Brackets escaped: `\[` and `\]` because `[]` is character class in regex
- No trailing space: a marker-only line `"- [ ]"` is still an open task and must match

**TaskDoneRegex: `^\s*- \[x\]`**
- Same structure as TaskOpenRegex but with `x` inside brackets
- Case-sensitive: architecture convention uses lowercase `x` only. Uppercase `X` would not match
- This is intentional: Claude CLI outputs lowercase `[x]` per the execute prompt instructions

**GateTagRegex: `\[GATE\]`**
- NOT anchored — matches `[GATE]` anywhere in a line (FR3: "тегом `[GATE]` в строке задачи")
- Case-sensitive: `[GATE]` is the exact convention from architecture
- Simple: no capturing groups needed — scanner just needs boolean "has gate?"

**FeedbackPrefix usage** (not a regex — string prefix match):
- Runner uses `strings.HasPrefix(line, config.FeedbackPrefix)` to detect feedback lines
- No regex needed: feedback lines always start at column 0 with exact `> USER FEEDBACK:` prefix

### Sprint-Tasks.md Format Context

The scanner in `runner/scan.go` (Story 3.2) will use these patterns to process sprint-tasks.md:
```
- [ ] Setup environment [GATE]
  source: stories/auth.md#AC-1
- [ ] Implement authentication logic
  source: stories/auth.md#AC-2
  > USER FEEDBACK: Use JWT, not session cookies
- [x] Write unit tests
  source: stories/auth.md#AC-3
```

**Scanner algorithm** (from architecture):
1. `content := os.ReadFile("sprint-tasks.md")`
2. `lines := strings.Split(string(content), "\n")`
3. For each line: match against regex patterns
4. `TaskOpenRegex.MatchString(line)` → open task found
5. `GateTagRegex.MatchString(line)` → gate point detected

### Project Structure Notes

- **New file:** `config/constants.go` — aligned with architecture: "config/constants.go — separate file for clarity" [Source: docs/architecture/project-structure-boundaries.md]
- **New file:** `config/constants_test.go` — co-located test file per naming convention
- **No conflicts:** No other package currently defines these constants (Story 1.6 depends only on Story 1.1)
- **Consumer packages:** `bridge` and `runner` will `import "github.com/bmad-ralph/bmad-ralph/config"` and use `config.TaskOpen`, `config.TaskOpenRegex`, etc.

### Previous Story Intelligence (Story 1.5)

**Learnings from Story 1.5 implementation and review:**
- Error test cases MUST verify error message content, not just `err != nil` — not directly relevant here (no error paths in constants), but good to remember
- Cross-platform `HOME`/`USERPROFILE` handling — not needed for this story
- Story File List: NEW files should be labeled "new/added", not "modified"
- Test naming: `Test<Type>_<Method>_<Scenario>` strictly enforced — regex tests use the pattern name as "Type"
- Single table-driven test per logical group — no standalone duplicates

**Code review findings from Story 1.5:**
- [M1] Error message verification is mandatory — incorporated in test design
- [L1] Test variety in naming patterns (flat vs subdirectory) — applied to regex test cases (varied input formats)

### Git Intelligence

**Recent commits (5 total):**
- `8d8df51` — Story 1.4: CLI override, go:embed defaults, three-level config cascade
- `bfa30c2` — Story 1.3: Config struct, YAML parsing, project root detection
- `dccde3b` — Stories 1.1 + 1.2: scaffold + error types

**IMPORTANT: Story 1.5 uncommitted changes exist.** `config/config.go` and `config/config_test.go` contain Story 1.5 ResolvePath implementation (done, not yet committed). These files will be in modified state when you start. Do NOT revert them — they are completed work.

**Patterns:**
- Single commit per story
- Table-driven tests with `t.Run`
- 31 top-level test functions currently passing (including Story 1.5 ResolvePath with 10 subtests)
- Go binary at `"/mnt/c/Program Files/Go/bin/go.exe"`
- CRLF fix required after every file creation: `sed -i 's/\r$//'`

### Architecture Compliance Checklist

- [ ] `config` remains leaf package (no imports of other project packages)
- [ ] Constants are exported (PascalCase) for cross-package use
- [ ] Regex patterns use `regexp.MustCompile` at package scope (NEVER inline)
- [ ] No `var` block for constants (use `const` block)
- [ ] `var` block for regex patterns (cannot be `const` — runtime initialization)
- [ ] No new external dependencies (only `regexp` stdlib)
- [ ] Test naming follows `Test<Pattern>` convention
- [ ] Table-driven tests for all regex patterns
- [ ] Edge cases: indented tasks, trailing content, empty lines, malformed markers
- [ ] No `os.Exit` calls
- [ ] No logging/printing from config package
- [ ] Existing tests still pass with no changes

### Anti-Patterns (FORBIDDEN)

- Defining constants in `runner/` or `bridge/` (MUST be in `config/` for shared use)
- Inline `regexp.Compile()` in function bodies (MUST be `MustCompile` at package scope)
- Using `regexp.Compile` with error return (use `MustCompile` — patterns are static, panics indicate programming error)
- Adding constants that aren't in the architecture spec (scope creep)
- Creating separate test files per regex pattern (one `constants_test.go` file)
- String matching on regex objects (test via `MatchString` with concrete inputs)
- Adding dependencies beyond `regexp` stdlib
- Modifying existing config files (this story creates NEW files only)
- Case-insensitive matching for `[x]` (architecture convention is lowercase only)
- Creating `FeedbackPrefixRegex` — not in AC, FeedbackPrefix is used as string prefix match by runner

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.6]
- [Source: docs/project-context.md#String Constants (config/constants.go)]
- [Source: docs/project-context.md#Anti-Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Structural Patterns — "String constants для маркеров"]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Structural Patterns — "Regex patterns для scan"]
- [Source: docs/architecture/project-structure-boundaries.md — "config/constants.go — separate file for clarity"]
- [Source: docs/prd/functional-requirements.md#FR3 — GATE tag in sprint-tasks.md]
- [Source: docs/prd/functional-requirements.md#FR11 — scanner grep `- [ ]`]
- [Source: docs/prd/functional-requirements.md#FR22 — USER FEEDBACK prefix]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No issues encountered during implementation.

### Completion Notes List

- Created `config/constants.go` with 4 string constants (`TaskOpen`, `TaskDone`, `GateTag`, `FeedbackPrefix`) and 3 compiled regex patterns (`TaskOpenRegex`, `TaskDoneRegex`, `GateTagRegex`)
- Created `config/constants_test.go` with 7 top-level test functions: 3 table-driven regex tests (29 subtests total covering positive/negative/edge cases) + 4 guard value tests
- All 38 config package tests pass (31 existing + 7 new), no regressions
- `config` remains leaf package — only `regexp` stdlib added as new import
- No external dependencies added
- CRLF fix applied to both new files

### File List

- `config/constants.go` (new/added) — string constants and compiled regex patterns for sprint-tasks.md parsing
- `config/constants_test.go` (new/added) — table-driven tests for all regex patterns and guard tests for constant values
- `docs/sprint-artifacts/sprint-status.yaml` (modified) — story status: ready-for-dev → in-progress → review
- `docs/sprint-artifacts/1-6-constants-and-regex-patterns.md` (modified) — tasks marked complete, Dev Agent Record filled

### Change Log

- 2026-02-25: Implemented Story 1.6 — constants and regex patterns for sprint-tasks.md markers in config package
- 2026-02-25: Code review fixes — added 7 test cases (M1: tab-indented done, M2: embedded done marker, M3: malformed done marker, L1: partial bracket gate tests, L3: deep indentation tests), fixed comment precision (L2)
