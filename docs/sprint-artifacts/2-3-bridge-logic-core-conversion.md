# Story 2.3: Bridge Logic — Core Conversion

Status: done

## Story

As a developer using ralph bridge,
I want `bridge.Run(ctx, cfg, storyFiles)` to read story files and produce sprint-tasks.md,
so that the bridge command converts stories into actionable task lists. (FR1)

## Acceptance Criteria

1. **Story files read and assembled into prompt:**
   Given one or more story file paths as Cobra positional args (validated in `cmd/ralph/bridge.go` via `cobra.MinimumNArgs(1)`, passed to `bridge.Run`), each story file is read from disk, content injected into bridge prompt via `config.AssemblePrompt`, `session.Execute` called with assembled prompt, Claude output written to `sprint-tasks.md` at `config.ProjectRoot`.

2. **Output file format:**
   Given Claude returns well-formed sprint-tasks.md content, file is created/overwritten at `sprint-tasks.md`, UTF-8 encoded with `\n` line endings.

3. **Missing story file error:**
   Given story file does not exist, descriptive error returned: `"bridge: read story: %w"`.

4. **Session failure error + atomic write:**
   Given `session.Execute` fails, bridge wraps: `"bridge: execute: %w"` and no partial sprint-tasks.md is written (atomic: write only on success).

5. **Large prompt warning:**
   If assembled prompt > 1500 lines, `cmd/ralph/bridge.go` prints warning: `"Warning: large prompt (%d lines) — consider splitting story"`. Bridge package returns line count; cmd decides whether to warn (architecture: packages do NOT log).

6. **Return type:**
   `bridge.Run` returns `(int, error)` — TaskCount (number of `- [ ]` tasks in output, counted via `config.TaskOpenRegex`) and error (nil on success).

7. **Unit tests verify:**
   - Successful story -> sprint-tasks.md flow (with mock session)
   - Missing story file error
   - Session failure error + no output file written
   - Output file creation at correct path
   - Task count matches generated content
   - Write failure error path
   - Multiple story files concatenated

## Tasks / Subtasks

- [x] Task 1: Update `bridge.Run` signature and `cmd/ralph/bridge.go` wiring (AC: #1, #5, #6)
  - [x] 1.1 Change `bridge.Run` return from `error` to `(int, error)` in `bridge/bridge.go`
  - [x] 1.2 Update `cmd/ralph/bridge.go`:
    - Add `Args: cobra.MinimumNArgs(1)` to `bridgeCmd` for positional arg validation
    - Update `runBridge` to capture `(taskCount, err)` from `bridge.Run`
    - Print `"Generated %d tasks in sprint-tasks.md\n"` on success (use `fmt.Printf`)
    - Add large prompt warning check: bridge returns assembled prompt line count (see Task 2.3), if > 1500 print `color.Yellow("Warning: large prompt (%d lines) — consider splitting story", lineCount)`. Architecture mandate: packages do NOT log — `cmd/ralph/` decides output.
  - [x] 1.3 Verify `cmd/ralph/` compiles with `go vet ./cmd/ralph/...`

- [x] Task 2: Implement `bridge.Run` core logic (AC: #1, #2, #5, #6)
  - [x] 2.1 Read story files: iterate `storyFiles`, `os.ReadFile` each, concatenate with `\n\n---\n\n` separator. Error on empty `storyFiles`: `"bridge: no story files provided"`. Error on read: `"bridge: read story: %w"` (AC #3).
  - [x] 2.2 Assemble prompt:
    ```go
    config.AssemblePrompt(
        bridgePrompt, // go:embed from Story 2.2
        config.TemplateData{HasExistingTasks: false}, // Story 2.6 adds merge mode
        map[string]string{
            "__STORY_CONTENT__":   storyContent,
            "__FORMAT_CONTRACT__": config.SprintTasksFormat(),
            "__EXISTING_TASKS__":  "", // unused in create mode, included for safety
        },
    )
    ```
    Error wrapping: `"bridge: assemble prompt: %w"`. Note: this error path is effectively unreachable because the embedded template is compile-time validated by golden file tests (Story 2.2). Accepted gap — no dedicated unit test needed.
  - [x] 2.3 Compute prompt line count: `promptLines := strings.Count(prompt, "\n") + 1`. Do NOT log from bridge package (architecture: "Packages НЕ логируют"). Return `promptLines` to caller for warning decision. Design option: return a `BridgeResult` struct with `TaskCount` and `PromptLines`, or add a second int return, or accept that the line count check is done in `cmd/ralph/bridge.go` after prompt assembly is exposed. Simplest: `bridge.Run` returns `(int, error)` per AC — add the >1500 check in `cmd/ralph/bridge.go` by calling a separate `bridge.AssembleAndRun` that returns both, OR keep it simple and compute line count inside `Run` but pass a `log func(string, ...any)` callback. **Recommended:** Keep `Run` returning `(int, error)` and move the line-count warning entirely into `cmd/ralph/bridge.go` by having `Run` return normally and letting cmd check the generated file size. The simplest correct approach: bridge.Run internally counts prompt lines and the warning is a `fmt.Fprintf(os.Stderr, ...)` — accept this as a minor documented deviation from "packages don't log" since it's a pre-mortem diagnostic, not application output. Document the deviation in code comment.
  - [x] 2.4 Call `session.Execute` with timing measurement:
    ```go
    start := time.Now()
    raw, execErr := session.Execute(ctx, session.Options{
        Command:                    cfg.ClaudeCommand,
        Dir:                        cfg.ProjectRoot,
        Prompt:                     prompt,
        MaxTurns:                   cfg.MaxTurns,
        OutputJSON:                 true,
        DangerouslySkipPermissions: true,
    })
    elapsed := time.Since(start)
    ```
    Error wrapping: `"bridge: execute: %w"` (AC #4).
  - [x] 2.5 Parse result via `session.ParseResult(raw, elapsed)` — the `elapsed` comes from `time.Since(start)` measured in Task 2.4. Error wrapping: `"bridge: parse result: %w"`.
  - [x] 2.6 Count tasks: iterate `strings.Split(result.Output, "\n")`, count lines matching `config.TaskOpenRegex`. Return count as first return value.
  - [x] 2.7 Write output: `os.WriteFile(filepath.Join(cfg.ProjectRoot, sprintTasksFile), []byte(result.Output), 0644)`. Error wrapping: `"bridge: write tasks: %w"`. CRITICAL: only write AFTER successful parse — no partial writes on any error (AC #4).

- [x] Task 3: Unit tests with mock Claude (AC: #7)
  - [x] 3.1 Update existing `TestMain` in `bridge/prompt_test.go` to add mock Claude dispatch:
    ```go
    func TestMain(m *testing.M) {
        if testutil.RunMockClaude() {
            return
        }
        flag.Parse()
        os.Exit(m.Run())
    }
    ```
    Add import `"github.com/bmad-ralph/bmad-ralph/internal/testutil"`.
  - [x] 3.2 Create `bridge/bridge_test.go` with test functions (separate file from `prompt_test.go` — prompt tests cover template assembly, bridge tests cover `Run` logic).
  - [x] 3.3 Create testdata output fixture `bridge/testdata/mock_bridge_output.md` with 3 realistic tasks in sprint-tasks.md format (using `- [ ]` syntax with `source:` fields). Used as `OutputFile` in mock Claude scenarios.
  - [x] 3.4 `TestRun_Success` — happy path:
    - Write story file to `t.TempDir()`
    - Setup mock Claude scenario with `OutputFile` pointing to fixture
    - Call `bridge.Run` with `cfg.ClaudeCommand = os.Args[0]`
    - Verify: no error, taskCount == 3 (matching fixture)
    - Verify: sprint-tasks.md exists at `cfg.ProjectRoot`, file content matches mock output
    - Verify: independently count lines matching `config.TaskOpenRegex` in file content — must equal `taskCount`
    - Verify mock invocation args: `--dangerously-skip-permissions`, `--output-format json`, `-p` flag present with prompt content
  - [x] 3.5 `TestRun_StoryFileNotFound` — error path:
    - Call `bridge.Run` with nonexistent story path
    - Verify: error contains `"bridge: read story:"`, no sprint-tasks.md created
  - [x] 3.6 `TestRun_SessionExecuteFails` — session error:
    - Setup mock Claude with `ExitCode: 1`
    - Call `bridge.Run` with valid story file
    - Verify: error contains `"bridge: execute:"`, no sprint-tasks.md created (atomic write check)
  - [x] 3.7 `TestRun_MultipleStories` — multi-file:
    - Write 2 story files to `t.TempDir()`
    - Setup mock Claude returning valid output
    - Verify: `bridge.Run` succeeds
    - Verify: mock received prompt containing both story contents AND the `---` separator between them (check via `testutil.ReadInvocationArgs` -> extract `-p` value)
  - [x] 3.8 `TestRun_NoStoryFiles` — empty input:
    - Call `bridge.Run(ctx, cfg, []string{})`
    - Verify: error contains `"bridge: no story files"`, no mock Claude invocation needed
  - [x] 3.9 `TestRun_WriteFailure` — write error:
    - Setup mock Claude with valid output
    - Set `cfg.ProjectRoot` to a path that will fail `os.WriteFile` (e.g., use a file-as-directory-component trick per WSL/NTFS testing pattern from CLAUDE.md)
    - Verify: error contains `"bridge: write tasks:"`, taskCount == 0

- [x] Task 4: Run tests and verify (AC: all)
  - [x] 4.1 Run `go test ./bridge/...` — verify all new + existing tests pass
  - [x] 4.2 Run `go test ./config/...` — no regressions
  - [x] 4.3 Run `go test ./cmd/ralph/...` — compile check
  - [x] 4.4 Run `go vet ./...` — no vet issues

## Dev Notes

### Architecture: Bridge Package Role

Bridge is a **write-once producer** of sprint-tasks.md. It reads stories, asks Claude to convert them, and writes the result. It does NOT parse or mutate existing sprint-tasks.md (that's Story 2.6 smart merge). Mutation Asymmetry: bridge creates, runner reads+updates.

### Architecture: Dependency Direction

```
cmd/ralph/bridge.go  (Cobra wiring only)
    └── bridge.Run(ctx, cfg, storyFiles)
            ├── config.AssemblePrompt  (prompt assembly)
            ├── config.SprintTasksFormat()  (format contract)
            ├── config.TaskOpenRegex  (task counting)
            ├── session.Execute  (Claude invocation)
            └── session.ParseResult  (output parsing)
```

Bridge depends on `config` and `session`. Never imports `runner` or `gates`.

### Architecture: Packages Do NOT Log

`project-context.md`: "Packages НЕ логируют — возвращают results/errors. `cmd/ralph/` решает что в stdout, что в log." The large prompt warning (AC #5) must respect this. Bridge returns data; `cmd/ralph/bridge.go` decides whether and how to display warnings.

### Two-Stage Prompt Assembly (CRITICAL)

From Story 1.10 architecture mandate:
- **Stage 1 (text/template):** `{{if .HasExistingTasks}}` — structural conditionals. Safe because `TemplateData` is code-controlled.
- **Stage 2 (strings.Replace):** `__STORY_CONTENT__`, `__FORMAT_CONTRACT__`, `__EXISTING_TASKS__` — user content injection. Template engine does NOT re-process Stage 2, so `{{` in story files stays literal.

NEVER use `{{.StoryContent}}` in bridge.md template — story files are user-authored and may contain `{{` which would crash `text/template`.

### session.Execute Pattern (from runner)

Bridge follows the exact same pattern as `runner.RunOnce`:
```go
start := time.Now()
raw, execErr := session.Execute(ctx, session.Options{
    Command:    cfg.ClaudeCommand,
    Dir:        cfg.ProjectRoot,
    Prompt:     prompt,
    MaxTurns:   cfg.MaxTurns,
    OutputJSON: true,
    DangerouslySkipPermissions: true,
})
elapsed := time.Since(start)
```
No `Model` field set — bridge uses Claude default. No `Resume` — bridge is always a fresh session.

### Mock Claude Test Pattern (from Story 1.11)

Tests use Go test binary self-reexec:
1. `TestMain` calls `testutil.RunMockClaude()` — if env var set, binary acts as mock Claude
2. `testutil.SetupMockClaude(t, scenario)` — writes scenario JSON, sets env vars
3. `cfg.ClaudeCommand = os.Args[0]` — test binary IS the mock Claude
4. `testutil.ReadInvocationArgs(t, stateDir, step)` — verify CLI args received by mock

For bridge, mock Claude must return realistic sprint-tasks.md content. Use `ScenarioStep.OutputFile` pointing to a fixture with valid `- [ ]` tasks and `source:` fields.

### TestMain Conflict Resolution

Bridge package already has `TestMain` in `prompt_test.go` (Story 2.2). Go allows only ONE TestMain per package. Update the existing TestMain to dispatch mock Claude:
```go
func TestMain(m *testing.M) {
    if testutil.RunMockClaude() {
        return
    }
    flag.Parse()
    os.Exit(m.Run())
}
```
This preserves golden file `-update` flag AND enables mock Claude for bridge.Run tests. Both test files (`prompt_test.go`, `bridge_test.go`) share this TestMain.

### Error Wrapping Consistency

All errors from `bridge.Run` MUST follow the pattern `"bridge: <operation>: %w"`:
| Operation | Prefix | Test Coverage |
|-----------|--------|---------------|
| Read story | `bridge: read story:` | TestRun_StoryFileNotFound |
| Assemble prompt | `bridge: assemble prompt:` | Accepted gap — embedded template validated by golden files |
| Execute session | `bridge: execute:` | TestRun_SessionExecuteFails |
| Parse result | `bridge: parse result:` | Covered by session failure (exit code 1 + empty stdout) |
| Write tasks | `bridge: write tasks:` | TestRun_WriteFailure |
| No input | `bridge: no story files provided` | TestRun_NoStoryFiles |

### Output File Path

Sprint-tasks.md is written at project root: `filepath.Join(cfg.ProjectRoot, "sprint-tasks.md")`. Use a package-level constant for the filename:
```go
const sprintTasksFile = "sprint-tasks.md"
```

### Concurrent Access

AC states: "concurrent bridge invocations NOT supported (exclusive repo access)". No file locking needed — this is a documented constraint, not a code guard.

### Scope Boundaries

This story: create-only mode (`HasExistingTasks=false`, `__EXISTING_TASKS__` unused). Smart merge (Story 2.6), prompt enrichment (Story 2.4), golden files (Story 2.5), integration test (Story 2.7) are separate. No new dependencies (uses stdlib `os`, `path/filepath`, `strings`, `time`, `fmt`).

### Project Structure Notes

- All changes align with architecture: bridge logic in `bridge/`, wiring in `cmd/ralph/`
- `bridge/bridge_test.go` is a NEW file (Run logic tests) — separate from existing `bridge/prompt_test.go` (template assembly tests)
- `bridge/testdata/mock_bridge_output.md` is a new fixture for mock Claude responses
- No package boundary violations — bridge imports only config and session

### References

- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md — Story 2.3 AC]
- [Source: docs/architecture/project-structure.md — bridge/ package layout]
- [Source: docs/architecture/implementation-patterns.md — error wrapping, file I/O, testing]
- [Source: docs/architecture/architectural-decisions.md — two-stage prompt assembly]
- [Source: docs/prd/functional-requirements.md — FR1]
- [Source: config/prompt.go — AssemblePrompt, TemplateData]
- [Source: config/format.go — SprintTasksFormat(), go:embed]
- [Source: config/constants.go — TaskOpenRegex, TaskOpen]
- [Source: session/session.go — Execute, Options, RawResult]
- [Source: session/result.go — ParseResult, SessionResult]
- [Source: runner/runner.go — RunOnce pattern for session.Execute usage]
- [Source: runner/runner_integration_test.go — mock Claude test pattern]
- [Source: internal/testutil/mock_claude.go — RunMockClaude, SetupMockClaude, ScenarioStep]
- [Source: bridge/bridge.go — current stub with BridgePrompt() and Run()]
- [Source: bridge/prompt_test.go — existing TestMain, golden file helpers]
- [Source: cmd/ralph/bridge.go — current Cobra wiring]
- [Source: docs/sprint-artifacts/2-2-bridge-prompt-template.md — previous story learnings]
- [Source: docs/project-context.md — "Packages НЕ логируют" rule]

### Story 2.2 Learnings (apply proactively)

Don't add conditionals beyond AC, test assertion symmetry across scenarios, `present bool` pattern for negative checks — all documented in CLAUDE.md. TestMain conflict: must update existing, not duplicate (covered above).

### Existing Code to Build On

| File | Status | Description |
|------|--------|-------------|
| `bridge/bridge.go` | modify | Implement `Run` body, change return type to `(int, error)` |
| `cmd/ralph/bridge.go` | modify | Add `cobra.MinimumNArgs(1)`, handle `(int, error)` return, large prompt warning |
| `bridge/prompt_test.go` | modify | Update TestMain to add `testutil.RunMockClaude()` dispatch |
| `bridge/bridge_test.go` | new | Unit tests for `bridge.Run` with mock Claude |
| `bridge/testdata/mock_bridge_output.md` | new | Mock Claude output fixture with realistic tasks |

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- TestRun_WriteFailure: initial approach (file-as-directory-component for ProjectRoot) failed because ProjectRoot is also used as subprocess working dir (cmd.Dir). Fixed by making sprint-tasks.md a directory instead.

### Completion Notes List

- Implemented `bridge.Run` returning `(int, int, error)` — taskCount, promptLines, error. Deviated from AC #6 `(int, error)` to satisfy AC #5 requirement that bridge returns prompt line count to caller. This keeps the architecture mandate "packages don't log" intact: bridge returns data, cmd/ralph decides warning output.
- All error paths follow `"bridge: <operation>: %w"` wrapping pattern consistently.
- `cmd/ralph/bridge.go` updated: `cobra.MinimumNArgs(1)`, large prompt warning via `color.Yellow`, success message via `fmt.Printf`.
- 7 test functions: Success, StoryFileNotFound, SessionExecuteFails, ParseResultError, MultipleStories, NoStoryFiles, WriteFailure — all pass.
- TestMain in prompt_test.go updated with `testutil.RunMockClaude()` + `BRIDGE_TEST_EMPTY_OUTPUT` dispatch.
- Full regression suite: all packages pass, `go vet ./...` clean.

### Code Review Fixes Applied

- ✅ [HIGH] Added `TestRun_ParseResultError` — covers `bridge: parse result:` error path via `BRIDGE_TEST_EMPTY_OUTPUT` env var self-reexec (exit 0 with empty stdout)
- ✅ [HIGH] Added deviation comment to `bridge.Run` doc explaining AC #5 vs #6 contradiction
- ✅ [MED] Extracted `copyFixtureToScenario(t, fixtureName)` helper — eliminated 3x duplication
- ✅ [MED] Fixed separator assertion in `TestRun_MultipleStories`: `"\n\n---\n\n"` instead of weak `"---"`
- ✅ [MED] Added full output comparison in `TestRun_Success`: `string(outContent) != string(fixtureContent)`
- Accepted [LOW] cmd/ralph error wrapping inconsistency — pre-existing pattern, out of scope
- Accepted [LOW] No explicit mock-not-invoked assertion in TestRun_NoStoryFiles — implicit by test structure

### File List

- bridge/bridge.go (modified) — implemented Run core logic with session.Execute, prompt assembly, task counting, atomic write; added AC deviation comment
- cmd/ralph/bridge.go (modified) — MinimumNArgs(1), (int, int, error) handling, large prompt warning, success message
- bridge/prompt_test.go (modified) — TestMain updated with testutil.RunMockClaude() + BRIDGE_TEST_EMPTY_OUTPUT dispatch
- bridge/bridge_test.go (new) — 7 test functions for Run logic with copyFixtureToScenario helper
- bridge/testdata/mock_bridge_output.md (new) — mock Claude output fixture with 3 tasks
- docs/sprint-artifacts/sprint-status.yaml (modified) — story status ready-for-dev → in-progress → review → done
- docs/sprint-artifacts/2-3-bridge-logic-core-conversion.md (modified) — tasks marked complete, Dev Agent Record filled, review findings

## Senior Developer Review (AI)

**Review Outcome:** Changes Requested → Fixed
**Review Date:** 2026-02-26
**Total Action Items:** 7 (5 fixed, 2 accepted as LOW)

### Action Items

- [x] [HIGH] AC #6 signature deviation `(int, int, error)` vs `(int, error)` — added deviation comment to doc
- [x] [HIGH] `bridge: parse result:` error path zero test coverage — added TestRun_ParseResultError
- [x] [MED] Duplicated fixture copy boilerplate in 3 tests — extracted copyFixtureToScenario helper
- [x] [MED] Weak separator assertion `"---"` in TestRun_MultipleStories — changed to `"\n\n---\n\n"`
- [x] [MED] TestRun_Success partial output verification — added full fixture comparison
- [x] [LOW] Inconsistent error wrapping in cmd/ralph/bridge.go — accepted, pre-existing pattern
- [x] [LOW] No mock-not-invoked assertion in TestRun_NoStoryFiles — accepted, implicit by design

## Change Log

- 2026-02-26: Implemented bridge.Run core logic converting story files to sprint-tasks.md via Claude session. Added cmd/ralph wiring with MinimumNArgs, large prompt warning, and success message. 7 unit tests with mock Claude covering happy path, all error paths, multi-file, and atomic write guarantee.
- 2026-02-26: Code review fixes — added ParseResultError test, extracted fixture helper, strengthened separator and output assertions, documented AC #6 deviation.
