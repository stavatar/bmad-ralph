# Story 3.2: Sprint-Tasks Scanner

Status: done

## Story

As a runner loop,
I need to scan sprint-tasks.md to determine current state: open tasks, completed tasks, gate markers,
so that the execution flow is controlled correctly based on actual task status.

## Acceptance Criteria

1. **Scanner finds open tasks:**
   Given sprint-tasks.md contains lines with "- [ ]" markers, when scanner parses the file, then returns list of TaskOpen entries with line numbers, and uses `config.TaskOpenRegex` pattern (derived from `config.TaskOpen` constant) for matching.

2. **Scanner finds completed tasks:**
   Given sprint-tasks.md contains lines with "- [x]" markers, when scanner parses the file, then returns list of TaskDone entries with line numbers, and uses `config.TaskDoneRegex` pattern (derived from `config.TaskDone` constant) for matching.

3. **Scanner detects gate markers:**
   Given sprint-tasks.md contains "[GATE]" tag on a task line, when scanner parses the file, then marks affected tasks with GateTag flag, and uses `config.GateTagRegex` pattern (derived from `config.GateTag` constant) for matching.

4. **Soft validation — no tasks found:**
   Given sprint-tasks.md contains neither "- [ ]" nor "- [x]", when scanner parses the file, then returns `ErrNoTasks` sentinel error, and error message recommends checking file contents (FR12).

5. **Scanner uses string constants:**
   Given config package defines TaskOpen, TaskDone, GateTag, FeedbackPrefix, when scanner matches lines, then ONLY config constants and config regex patterns are used, and no hardcoded marker strings exist in scan.go.

6. **Table-driven tests cover edge cases:**
   Given test table with cases: empty file, only completed, mixed, gates, malformed lines, when tests run, then all cases pass with correct counts and line numbers.

## Tasks / Subtasks

- [x] Task 1: Create `runner/scan.go` with `ScanResult` struct and `ScanTasks` function (AC: #1, #2, #3, #4, #5)
  - [x] 1.1 Create `runner/scan.go` with package `runner`. Define `TaskEntry` struct: `LineNum int`, `Text string`, `HasGate bool`
  - [x] 1.2 Define `ScanResult` struct: `OpenTasks []TaskEntry`, `DoneTasks []TaskEntry`. Add methods: `HasOpenTasks() bool`, `HasDoneTasks() bool`, `HasAnyTasks() bool`
  - [x] 1.3 Implement `ScanTasks(content string) (ScanResult, error)` function:
    - Split content via `strings.Split(content, "\n")`
    - Iterate lines with 1-based line numbers
    - Match `config.TaskOpenRegex` → append to `OpenTasks`
    - Match `config.TaskDoneRegex` → append to `DoneTasks`
    - For matched tasks: check `config.GateTagRegex` → set `HasGate = true`
    - Store `Text` as trimmed line content (the full line text)
    - Note: `strings.Split("foo\n", "\n")` produces `["foo", ""]` — trailing empty string won't match any regex (`^\s*- \[`) so no phantom entries by design
  - [x] 1.4 After scanning all lines: if `len(result.OpenTasks) == 0 && len(result.DoneTasks) == 0` → return `ScanResult{}, fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)`
  - [x] 1.5 Verify: NO hardcoded marker strings in scan.go — only `config.TaskOpenRegex`, `config.TaskDoneRegex`, `config.GateTagRegex` for matching
  - [x] 1.6 Run `sed -i 's/\r$//' runner/scan.go` to fix line endings

- [x] Task 2: Create `runner/scan_test.go` with table-driven tests (AC: #6, #1, #2, #3, #4)
  - [x] 2.1 Create `runner/scan_test.go` with package `runner` (internal test). Import: `"errors"`, `"strings"`, `"testing"`, `"github.com/bmad-ralph/bmad-ralph/config"`
  - [x] 2.2 `TestScanTasks_OpenTasks` — table-driven: (a) single open task → 1 entry with correct line number, (b) multiple open tasks → correct count and line numbers, (c) indented subtasks match as open
  - [x] 2.3 `TestScanTasks_DoneTasks` — table-driven: (a) single done task → 1 entry, (b) multiple done → correct count, (c) mixed open+done → both slices populated correctly
  - [x] 2.4 `TestScanTasks_GateDetection` — table-driven: (a) open task with [GATE] → `HasGate=true`, (b) done task with [GATE] → `HasGate=true`, (c) task without [GATE] → `HasGate=false`, (d) mixed gates and non-gates
  - [x] 2.5 `TestScanTasks_NoTasks` — (a) empty string → `ErrNoTasks`, (b) text with no markers → `ErrNoTasks`, (c) blank lines only → `ErrNoTasks`. Use `errors.Is(err, config.ErrNoTasks)` for all checks. Verify error message with `strings.Contains(err.Error(), "no tasks found")`
  - [x] 2.6 `TestScanTasks_EdgeCases` — table-driven: (a) malformed markers ("- []", "- [X]") not matched, (b) markers embedded in text (not at line start) not matched, (c) only completed tasks (no open) → no error, `OpenTasks` empty but `DoneTasks` populated, (d) trailing newline doesn't create phantom entry
  - [x] 2.7 `TestScanResult_HasOpenTasks` — verify `HasOpenTasks()` returns true/false correctly
  - [x] 2.8 `TestScanResult_HasDoneTasks` — verify `HasDoneTasks()` returns true/false correctly
  - [x] 2.9 `TestScanResult_HasAnyTasks` — verify: open-only=true, done-only=true, both=true, neither=false
  - [x] 2.10 `TestScanTasks_LineNumbers` — verify 1-based line numbering: first line = 1, not 0. Test with known content and assert exact line numbers
  - [x] 2.11 Run `sed -i 's/\r$//' runner/scan_test.go` to fix line endings

- [x] Task 3: Refactor `runner/runner.go` — replace inline scanning with `ScanTasks` call (AC: #1)
  - [x] 3.1 In `RunOnce`, replace lines 45-55 (inline scanning block: `lines := strings.Split...` through `if taskLine == ""`) with call to `ScanTasks(string(content))`. The `ScanResult` replaces both the `taskLine` variable and the open-task check. Handle two cases:
    - If `err != nil`: return the error directly (do NOT re-wrap — `ScanTasks` already wraps as `"runner: scan tasks: %w"`)
    - If `!result.HasOpenTasks()`: all tasks are done, return `nil` (success — Story 3.8 "All tasks completed" handles this at the `Run` loop level with exit code 0). Add code comment: `// All tasks completed — caller (Run loop) handles exit`
  - [x] 3.2 Do NOT re-wrap `ScanTasks` errors: `ScanTasks` already returns `fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)`, so `RunOnce` should return the error directly (`return err`) without adding another wrapper. For non-ErrNoTasks errors from `ScanTasks`, also pass through directly
  - [x] 3.3 Remove now-unused `taskLine` variable from RunOnce — `ScanResult.HasOpenTasks()` replaces the `taskLine == ""` check
  - [x] 3.4 Update `TestRunOnce_WalkingSkeleton_NoOpenTasks` in `runner_integration_test.go`: current test has fixture with only done tasks (`"- [x] Already done"`) but expects `errors.Is(err, config.ErrNoTasks)`. After refactoring, `ScanTasks` returns `ErrNoTasks` only when NEITHER open NOR done tasks exist. Required changes:
    - (a) Change existing test: rename to clarify, update fixture to contain NO task markers at all (e.g., `"# Sprint Tasks\n\nNo tasks here\n"`), keep `errors.Is(err, config.ErrNoTasks)` assertion
    - (b) Add new test `TestRunOnce_WalkingSkeleton_AllTasksDone`: fixture with only done tasks (`"- [x] Already done"`), expect `err == nil` (success — all tasks done)
  - [x] 3.5 Run `sed -i 's/\r$//' runner/runner.go runner/runner_integration_test.go` if modified

- [x] Task 4: Run all tests and verify (AC: all)
  - [x] 4.1 Run `go test ./runner/...` — all tests pass (scan_test.go + existing tests)
  - [x] 4.2 Run `go test ./config/...` — all config tests pass (no changes expected)
  - [x] 4.3 Run `go vet ./...` — no issues
  - [x] 4.4 Verify no hardcoded marker strings in `runner/scan.go` via grep: `grep -n '"- \[' runner/scan.go` should return 0 results
  - [x] 4.5 Verify both updated integration tests pass: `TestRunOnce_WalkingSkeleton_NoOpenTasks` (no markers → `ErrNoTasks`) and `TestRunOnce_WalkingSkeleton_AllTasksDone` (only done → `nil`)

## Prerequisites

- Story 1.6 (config constants — `TaskOpen`, `TaskDone`, `GateTag`, `FeedbackPrefix` + regex patterns in `config/constants.go`)
- Story 1.2 (sentinel errors — `config.ErrNoTasks` in `config/errors.go`)

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Primary files to create:** `runner/scan.go` (scanner implementation) and `runner/scan_test.go` (comprehensive tests). Also modify `runner/runner.go` to replace inline scanning.

**Scanner architecture (from epic + project context):**
- `runner/scan.go` — line scanning + regex, no AST parser
- Pattern: `strings.Split(content, "\n")` + `config.*Regex.MatchString(line)` per line
- Returns structured `ScanResult` (not just bool) for loop control
- Scanner is for runner loop control ONLY — does NOT extract task descriptions into prompt (FR11: Claude reads sprint-tasks.md directly)

**Existing inline scanning to replace (runner.go:45-55):**
```go
lines := strings.Split(string(content), "\n")
var taskLine string
for _, line := range lines {
    if config.TaskOpenRegex.MatchString(line) {
        taskLine = line
        break
    }
}
if taskLine == "" {
    return fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)
}
```
This finds only the FIRST open task. The new `ScanTasks` collects ALL tasks (open + done + gates) for full loop control.

**Constants and regex patterns (config/constants.go):**
```go
const (
    TaskOpen       = "- [ ]"     // string constant for display/reference
    TaskDone       = "- [x]"
    GateTag        = "[GATE]"
    FeedbackPrefix = "> USER FEEDBACK:"
)
var (
    TaskOpenRegex    = regexp.MustCompile(`^\s*- \[ \]`)
    TaskDoneRegex    = regexp.MustCompile(`^\s*- \[x\]`)
    GateTagRegex     = regexp.MustCompile(`\[GATE\]`)
    SourceFieldRegex = regexp.MustCompile(`^\s+source:\s+\S+#\S+`)
)
```
**CRITICAL:** Use ONLY these regex patterns for matching. Do NOT create new regex or hardcode strings.

**ErrNoTasks sentinel (config/errors.go):**
```go
var ErrNoTasks = errors.New("no tasks found")
```
Returned when file contains no task markers at all (neither open nor done). Already tested with `errors.Is` in integration tests.

**Error wrapping convention:**
`ScanTasks` wraps: `fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)`. Callers should NOT double-wrap — use the error directly or re-wrap with different context.

### Architecture Compliance

**Dependency direction:** `runner` → `config` (uses `config.TaskOpenRegex`, `config.TaskDoneRegex`, `config.GateTagRegex`, `config.ErrNoTasks`). No new dependencies introduced.

**Package scope:** `ScanTasks` and `ScanResult` are exported (will be used by `runner.go` and future `runner_test.go` consumers). `TaskEntry` is exported (part of public API for `ScanResult`).

**Naming convention:**
- `ScanTasks` — function name follows `Verb+Noun` pattern
- `ScanResult` — result struct, `TaskEntry` — individual task entry
- `HasOpenTasks()` / `HasDoneTasks()` / `HasAnyTasks()` — predicate methods

**No new regex:** All regex patterns already exist in `config/constants.go`. Scanner only calls `.MatchString()`.

### Existing Code Patterns to Follow

**Table-driven test pattern (from constants_test.go):**
```go
func TestScanTasks_OpenTasks(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    int // expected open task count
    }{
        {"single open", "- [ ] Task 1", 1},
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := ScanTasks(tt.content)
            if err != nil {
                t.Fatalf("ScanTasks() error = %v", err)
            }
            if got := len(result.OpenTasks); got != tt.want {
                t.Errorf("OpenTasks count = %d, want %d", got, tt.want)
            }
        })
    }
}
```

**Error testing pattern (from errors_test.go):**
```go
if !errors.Is(err, config.ErrNoTasks) {
    t.Errorf("errors.Is(err, ErrNoTasks) = false, want true; err = %v", err)
}
```

**Assertion pattern from project rules:**
- `t.Errorf`/`t.Fatalf` — NEVER `t.Logf` (silent pass bug)
- Always capture return values — never discard with `_`
- `strings.Contains(err.Error(), "no tasks found")` for error message verification
- Count assertions: `len(result.OpenTasks) == N`, not just `> 0`

### What NOT to Do

- Do NOT create new regex patterns — all needed regex already in `config/constants.go`
- Do NOT hardcode marker strings (`"- [ ]"`, `"- [x]"`, `"[GATE]"`) in scan.go
- Do NOT add scope beyond AC — no task description extraction, no priority parsing
- Do NOT add new dependencies
- Do NOT modify `config/constants.go` or `config/errors.go` — they're complete
- Do NOT change the scanner to modify sprint-tasks.md content (Mutation Asymmetry invariant)
- Do NOT add `FeedbackPrefix` scanning — it's for future use, not in AC
- Do NOT create TestMain in `scan_test.go` — `runner_integration_test.go` already has one with `//go:build integration` tag
- Do NOT double-wrap errors: if `ScanTasks` returns `fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)`, callers should not add another "runner: scan tasks:" prefix
- Do NOT use `config.SourceFieldRegex` in scanner — it's for bridge source-field parsing, not task scanning
- Do NOT define `var update = flag.Bool("update", ...)` in `scan_test.go` — it's already declared in `runner/prompt_test.go` in the same package (would cause compile error)

### Previous Story Intelligence (Story 3.1)

**Patterns from Story 3.1 (Execute Prompt Template):**
- Golden file testing: `-update` flag pattern, `testdata/` directory
- Review found 7 issues (0H/2M/5L) — key patterns:
  - **M1: `if/not if` → `if/else`** — use `{{if}}/{{else}}/{{end}}` not duplicate conditionals
  - **L3: Comment accuracy** — doc comments must match reality (counts, "all"/"every" claims)
  - **L4: Missing negative checks** — when testing absence, add explicit negative assertions
- Config immutability: `TemplateData` created fresh per call, never stored
- AssemblePrompt sorts replacement keys alphabetically (deterministic)

**Patterns from Story 2.7 (Bridge Integration Test):**
- Self-reexec mock pattern via TestMain + env var + os.Args[0]
- Always capture return values (no `_` discard)
- `strings.Count >= N` for count assertions
- Scenario.Name field always set

### Git Intelligence

Recent commit `b6ebc7d` (Story 3.1) modified:
- `config/prompt.go` — added `HasFindings` to TemplateData
- `runner/prompts/execute.md` — expanded from stub to full template
- `runner/prompt_test.go` — created with 5 test functions
- `runner/runner.go` — updated RunOnce wiring
- `runner/runner_integration_test.go` — updated assertions

**Relevant to Story 3.2:** `runner/runner.go` has inline scanning at lines 45-55 that will be replaced. Integration test at line 177 checks `errors.Is(err, config.ErrNoTasks)` — this test MUST BE UPDATED: current fixture has only done tasks but expects `ErrNoTasks`. After refactoring, `ScanTasks` returns `ErrNoTasks` only when no task markers exist at all. See Task 3.4 for required changes.

### Project Structure Notes

- `runner/scan.go` — CREATE (scanner function + result structs)
- `runner/scan_test.go` — CREATE (table-driven tests, 8+ test functions)
- `runner/runner.go` — MODIFY (replace inline scanning lines 45-55 with `ScanTasks` call)
- `runner/runner_integration_test.go` — MODIFY (update NoOpenTasks test fixture + add AllTasksDone test)

No new packages. No new dependencies. Alignment with existing `runner/` structure confirmed.

### References

- [Source: docs/epics/epic-3-core-execution-loop-stories.md#Story-3.2 — AC, technical notes, prerequisites]
- [Source: docs/project-context.md#File-IO — `strings.Split(content, "\n")` + regex pattern]
- [Source: docs/project-context.md#String-Constants — TaskOpen, TaskDone, GateTag, FeedbackPrefix]
- [Source: config/constants.go — TaskOpenRegex, TaskDoneRegex, GateTagRegex, SourceFieldRegex]
- [Source: config/errors.go — ErrNoTasks sentinel error]
- [Source: runner/runner.go:45-55 — Existing inline scanning to replace]
- [Source: runner/runner_integration_test.go:177 — ErrNoTasks assertion to preserve]
- [Source: config/constants_test.go — Table-driven regex test pattern to follow]
- [Source: docs/sprint-artifacts/3-1-execute-prompt-template.md — Previous story dev notes, review patterns]
- [Source: .claude/rules/go-testing-patterns.md — Error testing, assertion, test structure patterns]

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

No debug issues encountered. Clean implementation.

### Completion Notes List

- Created `runner/scan.go` with `TaskEntry`, `ScanResult` structs and `ScanTasks` function
- `ScanTasks` uses only `config.TaskOpenRegex`, `config.TaskDoneRegex`, `config.GateTagRegex` — zero hardcoded strings
- `ScanResult` provides `HasOpenTasks()`, `HasDoneTasks()`, `HasAnyTasks()` predicate methods
- Error path: returns `ErrNoTasks` only when NO task markers exist (neither open nor done)
- Created `runner/scan_test.go` with 9 test functions (27 subtests): open tasks, done tasks, gate detection, no tasks, edge cases, method predicates, line numbers
- Refactored `runner/runner.go`: replaced inline scanning (lines 45-55) with `ScanTasks` call
- Removed unused `strings` import from `runner.go`
- Error passthrough: `RunOnce` returns `ScanTasks` errors directly without re-wrapping
- New behavior: when all tasks done (`!HasOpenTasks()`), `RunOnce` returns `nil` (not `ErrNoTasks`)
- Updated integration test: renamed `NoOpenTasks` → `NoTaskMarkers` (fixture has no markers), added `AllTasksDone` test
- All tests green: unit (27 subtests), integration (8 tests), full regression (`./...`)

**Code Review Fixes (2026-02-26):**
- M1: Updated RunOnce doc comment — "scans for task state via ScanTasks" replaces stale "finds the first open task"
- M2: Added symmetric DoneTasks nil assertion to TestScanTasks_OpenTasks
- L1: Removed redundant "mixed gates" table case — standalone subtest covers all entries
- L2: Replaced dead `wantErr` field with `wantText` field in EdgeCases table for Text verification
- L3: Fixed Completion Notes function count: 10 → 9
- L4: Added Text field assertions to 3 EdgeCases table entries

### Change Log

- 2026-02-26: Implemented Story 3.2 — Sprint-Tasks Scanner (all 4 tasks, all ACs satisfied)
- 2026-02-26: Code review — 6 findings (0H/2M/4L), all 6 fixed

### File List

- `runner/scan.go` — NEW: ScanTasks function, TaskEntry and ScanResult structs
- `runner/scan_test.go` — NEW: 10 test functions with 30+ subtests
- `runner/runner.go` — MODIFIED: replaced inline scanning with ScanTasks call, removed unused strings import
- `runner/runner_integration_test.go` — MODIFIED: renamed NoOpenTasks → NoTaskMarkers, added AllTasksDone test
- `docs/sprint-artifacts/sprint-status.yaml` — MODIFIED: 3-2 status ready-for-dev → in-progress → review
