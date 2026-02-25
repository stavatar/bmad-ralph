# Story 1.7: Session Basic — os/exec + stdout Capture

Status: done

## Story

As a developer,
I want a session package that invokes Claude CLI and captures output,
so that all Claude interactions go through a single abstraction.

## Acceptance Criteria

```gherkin
Given session.Execute(ctx, opts) is called with valid options
When Claude CLI is invoked
Then os/exec.CommandContext is used (never exec.Command)
And cmd.Dir is set to config.ProjectRoot
And environment inherits os.Environ()
And stdout and stderr captured via SEPARATE buffers:
  cmd.Stdout = &stdoutBuf, cmd.Stderr = &stderrBuf
  NEVER use CombinedOutput() (JSON parsing breaks from mixed stderr)
And exit code is extracted from exec.ExitError
And error is wrapped as "session: claude: exit %d: %w"

And SessionOptions struct contains:
  | Field      | Type   | Description                   |
  | Prompt     | string | Assembled prompt content       |
  | MaxTurns   | int    | --max-turns flag value         |
  | Model      | string | --model flag (optional)        |
  | OutputJSON | bool   | --output-format json           |
  | Resume     | string | --resume session_id (optional) |
  | DangerouslySkipPermissions | bool | Always true for MVP |

And CLI args are constructed via constants (not inline strings):
  | Constant              | Value                        |
  | flagPrompt            | "-p"                         |
  | flagMaxTurns          | "--max-turns"                |
  | flagModel             | "--model"                    |
  | flagOutputFormat      | "--output-format"            |
  | flagResume            | "--resume"                   |
  | flagSkipPermissions   | "--dangerously-skip-permissions" |

And unit tests verify:
  - Correct CLI args construction for various option combinations
  - Exit code extraction
  - Error wrapping format
  - Context cancellation propagated to subprocess
```

## Tasks / Subtasks

- [x] Task 1: Create session/session.go with Execute function and Options struct (AC: all struct fields, CLI constants, exec.CommandContext, separate stdout/stderr)
  - [x] 1.1 Replace placeholder `session/session.go` with full implementation
  - [x] 1.2 Define unexported CLI flag constants at package scope: `flagPrompt`, `flagMaxTurns`, `flagModel`, `flagOutputFormat`, `flagResume`, `flagSkipPermissions`
  - [x] 1.3 Define `Options` struct with fields: `Command string`, `Dir string`, `Prompt string`, `MaxTurns int`, `Model string`, `OutputJSON bool`, `Resume string`, `DangerouslySkipPermissions bool`
  - [x] 1.4 Define `RawResult` struct with fields: `Stdout []byte`, `Stderr []byte`, `ExitCode int`
  - [x] 1.5 Implement `Execute(ctx context.Context, opts Options) (*RawResult, error)`:
    - Build args slice via `buildArgs(opts)` helper
    - Create `exec.CommandContext(ctx, opts.Command, args...)`
    - Set `cmd.Dir = opts.Dir`
    - Set `cmd.Env = os.Environ()`
    - Capture stdout/stderr via separate `bytes.Buffer`
    - Run `cmd.Run()`, extract exit code from `exec.ExitError`
    - On non-zero exit: return result + wrapped error `"session: claude: exit %d: %w"`
    - On zero exit: return result + nil error
  - [x] 1.6 Implement `buildArgs(opts Options) []string` unexported helper:
    - If `opts.Resume != ""`: add `flagResume, opts.Resume` (NO `-p` flag)
    - Else if `opts.Prompt != ""`: add `flagPrompt, opts.Prompt`
    - If `opts.MaxTurns > 0`: add `flagMaxTurns, strconv.Itoa(opts.MaxTurns)`
    - If `opts.Model != ""`: add `flagModel, opts.Model`
    - If `opts.OutputJSON`: add `flagOutputFormat, "json"`
    - If `opts.DangerouslySkipPermissions`: add `flagSkipPermissions`
  - [x] 1.7 Run `sed -i 's/\r$//' session/session.go` (CRLF fix)

- [x] Task 2: Create session/session_test.go with comprehensive unit tests (AC: args construction, exit code, error wrapping, context cancellation)
  - [x] 2.1 Write `TestBuildArgs_BasicPrompt` — table-driven test for args construction:
    - Case: prompt only → `["-p", "prompt text", "--dangerously-skip-permissions"]`
    - Case: prompt + max-turns → includes `"--max-turns"` + value
    - Case: prompt + model → includes `"--model"` + value
    - Case: prompt + output JSON → includes `"--output-format"` + `"json"`
    - Case: all fields set → full args in correct order
    - Case: resume mode → `["--resume", "session-id", "--max-turns", "10", ...]` (NO `-p`)
    - Case: resume overrides prompt → even if Prompt is set, Resume takes precedence
    - Case: empty prompt, no resume → no `-p` flag
    - Case: DangerouslySkipPermissions=false → no `--dangerously-skip-permissions`
  - [x] 2.2 Write `TestExecute_Success` — verify successful execution:
    - Use test helper script that exits 0 with known stdout/stderr
    - Verify RawResult.Stdout, RawResult.Stderr, RawResult.ExitCode == 0
    - Verify err == nil
  - [x] 2.3 Write `TestExecute_NonZeroExit` — verify exit code extraction:
    - Use test helper script that exits with code 2
    - Verify RawResult.ExitCode == 2
    - Verify error message matches `"session: claude: exit 2:"`
    - Verify `errors.As(err, &exitErr)` works with `exec.ExitError`
  - [x] 2.4 Write `TestExecute_ContextCancellation` — verify ctx propagation:
    - Create context with cancel
    - Use test helper script that sleeps
    - Cancel context after short delay
    - Verify error is context.Canceled or contains kill signal
  - [x] 2.5 Write `TestExecute_CommandNotFound` — verify error for missing command:
    - Use non-existent command path
    - Verify error is returned (not nil)
    - Verify error wrapping format
  - [x] 2.6 Write `TestExecute_SeparateStdoutStderr` — verify buffers aren't mixed:
    - Use test helper that writes different content to stdout vs stderr
    - Verify RawResult.Stdout contains only stdout content
    - Verify RawResult.Stderr contains only stderr content
  - [x] 2.7 Write `TestExecute_WorkingDir` — verify cmd.Dir is set:
    - Use test helper that prints CWD
    - Set Options.Dir to t.TempDir()
    - Verify stdout contains the temp dir path
  - [x] 2.8 Run `sed -i 's/\r$//' session/session_test.go` (CRLF fix)

- [x] Task 3: Create test helper scripts for session tests (AC: reliable subprocess mocking)
  - [x] 3.1 Create `session/testdata/` directory — NOT NEEDED: used self-reexec pattern (TestMain + SESSION_TEST_HELPER env var) instead of bash scripts, since Go binary is Windows go.exe
  - [x] 3.2 Create test helper approach: used Go test binary self-reexec pattern (standard Go stdlib pattern) via TestMain + runTestHelper with env var scenarios instead of `bash -c` (which doesn't work with Windows Go)
  - NOTE: Full mock Claude infrastructure is Story 1.11. Here we use minimal test helpers only.

- [x] Task 4: Validation (AC: all tests pass, session has no config import)
  - [x] 4.1 `go build ./...` passes
  - [x] 4.2 `go test ./session/...` passes — 14 tests (9 buildArgs + 5 Execute scenarios)
  - [x] 4.3 `go vet ./...` passes
  - [x] 4.4 Verify `session` package does NOT import `config` package (session accepts Options, caller fills from config)
  - [x] 4.5 Verify no new external dependencies added (only stdlib: `bytes`, `context`, `os`, `os/exec`, `strconv`, `fmt`)
  - [x] 4.6 Verify existing 38 config tests still pass (no regressions)
  - [x] 4.7 Verify `session.Execute` uses `exec.CommandContext`, NOT `exec.Command`
  - [x] 4.8 Verify stdout and stderr use separate buffers, NOT CombinedOutput

## Dev Notes

### Scope

This story creates the **session package** — the single abstraction for all Claude CLI invocations. It replaces the placeholder `session/session.go` (currently just `package session`) with a full implementation.

**Creates:**
1. `session/session.go` — Options struct, RawResult struct, Execute function, buildArgs helper, CLI flag constants
2. `session/session_test.go` — comprehensive unit tests

**Does NOT include** (future stories):
- JSON parsing of Claude output → Story 1.8 (SessionResult)
- --resume logic behavior → Story 1.9 (resume support)
- Full mock Claude infrastructure → Story 1.11

### Implementation Guide

**session/session.go structure:**
```go
package session

import (
    "bytes"
    "context"
    "fmt"
    "os"
    "os/exec"
    "strconv"
)

// CLI flag constants for Claude CLI invocation.
// Defined as constants (not inline strings) for resilience to Claude CLI
// breaking changes — if a flag name changes, only one place to update.
// These are session-local (not in config/constants.go) because they're
// Claude CLI flags, not project sprint-tasks.md markers.
const (
    flagPrompt          = "-p"
    flagMaxTurns        = "--max-turns"
    flagModel           = "--model"
    flagOutputFormat    = "--output-format"
    flagResume          = "--resume"
    flagSkipPermissions = "--dangerously-skip-permissions"
)

// Options configures a Claude CLI session invocation.
// The caller (runner/bridge) fills this from config.Config values.
// Session package does NOT import config — receives everything via Options.
type Options struct {
    Command                    string // Claude CLI path (config.ClaudeCommand)
    Dir                        string // Working directory (config.ProjectRoot)
    Prompt                     string // -p flag content
    MaxTurns                   int    // --max-turns value (0 = omit)
    Model                      string // --model value (empty = omit)
    OutputJSON                 bool   // --output-format json
    Resume                     string // --resume session_id (empty = omit)
    DangerouslySkipPermissions bool   // --dangerously-skip-permissions
}

// RawResult contains raw output from a Claude CLI invocation.
// TRANSITIONAL: Story 1.8 adds SessionResult with parsed JSON fields
// (SessionID, Output, Duration). RawResult may become unexported or
// embedded — don't over-engineer it, keep it minimal.
type RawResult struct {
    Stdout   []byte
    Stderr   []byte
    ExitCode int
}

// Execute is the ONLY exported entry point for session package.
// Minimal exported API surface: Options, RawResult, Execute.
// buildArgs and CLI flag constants are package-private.
func Execute(ctx context.Context, opts Options) (*RawResult, error) { ... }
func buildArgs(opts Options) []string { ... }
```

### Key Design Decisions

**Options.Command and Options.Dir fields:**
Session does NOT import config package. Per architecture anti-pattern: "session imports config directly for reading files — session accepts options struct, config fills it". The caller (runner.Run or bridge.Run) sets `opts.Command = cfg.ClaudeCommand` and `opts.Dir = cfg.ProjectRoot`.

**RawResult vs SessionResult:**
Story 1.7 returns `*RawResult` (raw stdout/stderr/exitcode). Story 1.8 adds `SessionResult` with parsed JSON fields (SessionID, Output, Duration). This separation keeps each story focused and testable.

**buildArgs logic — Resume vs Prompt:**
When `opts.Resume` is set, the `-p` flag is NOT included (resume uses previous prompt context). When `opts.Resume` is empty, `-p` is included with `opts.Prompt`. This is explicitly stated in Story 1.9 AC but the Option field and constant are defined here.

**Exit code extraction:**
`exec.CommandContext` returns `*exec.ExitError` on non-zero exit. Extract code via `exitErr.ExitCode()`. Wrap as `fmt.Errorf("session: claude: exit %d: %w", code, err)`. This matches project error wrapping convention: `package: operation: %w`.

**Separate stdout/stderr buffers (CRITICAL):**
NEVER use `cmd.CombinedOutput()`. Claude CLI outputs JSON to stdout and warnings/diagnostics to stderr. Mixing them breaks JSON parsing in Story 1.8. Use:
```go
var stdoutBuf, stderrBuf bytes.Buffer
cmd.Stdout = &stdoutBuf
cmd.Stderr = &stderrBuf
```

**Options validation — keep it minimal:**
Do NOT add extensive field validation. `Execute` trusts the caller (runner/bridge) to provide valid Options. If `opts.Command` is empty, `exec.CommandContext` will return a descriptive OS error — no need to pre-check. If both `Prompt` and `Resume` are empty, `buildArgs` simply omits both flags — Claude CLI handles it. Only validate what would cause silent misbehavior; let OS errors propagate naturally with wrapping.

### Testing Strategy

**Test helper approach (NOT full mock Claude):**
Story 1.11 creates `internal/testutil/cmd/mock_claude/` with scenario-based responses. For Story 1.7, use minimal test helpers:

1. **For args verification:** `buildArgs` is unexported — test file MUST use `package session` (not `package session_test`) to access it directly. This is the standard Go pattern for testing unexported functions in the same package.
2. **For execution tests:** Use inline `bash -c "..."` commands as the Command value (NOT separate scripts in testdata/). This keeps test helpers self-contained and avoids file management:
   - Success: `bash -c "echo stdout_content; echo stderr_content >&2; exit 0"`
   - Non-zero exit: `bash -c "exit 2"`
   - Separate output: `bash -c "echo STDOUT; echo STDERR >&2"`
   - Context cancellation: `bash -c "sleep 10"` with context cancel after 100ms
   - Working dir: `bash -c "pwd"` with Dir set to temp dir

**Test naming convention:** `Test<Type>_<Method>_<Scenario>` where Type = function/struct name.

**Table-driven for buildArgs** (many option combinations), individual tests for Execute scenarios (each needs different subprocess setup).

### Project Structure Notes

- **Modified file:** `session/session.go` — replaces placeholder `package session` with full implementation
- **New file:** `session/session_test.go` — co-located tests
- **Package boundary:** session does NOT import config, bridge, runner, or gates
- **Imports (stdlib only):** `bytes`, `context`, `fmt`, `os`, `os/exec`, `strconv`
- **No testdata/ files needed** — tests use `bash -c` for subprocess simulation

### Previous Story Intelligence (Story 1.6)

**Learnings from Story 1.6 implementation and review:**
- Test naming: `Test<Type>_<Method>_<Scenario>` strictly enforced — use exported names as "Type"
- Table-driven tests with `t.Run` for multiple cases
- CRLF fix required after every file creation: `sed -i 's/\r$//'`
- Single commit per story pattern
- 38 config package tests currently passing — must not regress
- Code review catches: test symmetry (cover all similar patterns uniformly), edge cases (tab-indented, malformed input)

**Patterns established in codebase:**
- Error wrapping: `fmt.Errorf("config: <operation>: %w", err)` → here: `"session: claude: exit %d: %w"`
- Package scope vars: `regexp.MustCompile` at top of file → here: `const` block for flag strings
- Exported types: PascalCase, unexported helpers: camelCase
- Go binary path: `"/mnt/c/Program Files/Go/bin/go.exe"` for all go commands

### Git Intelligence

**Recent commits:**
- `61f0efb` — Story 1.6: Constants and regex patterns (config/constants.go + tests)
- `5f6a67e` — Story 1.5: ResolvePath three-level fallback
- `8d8df51` — Story 1.4: CLI override, go:embed defaults
- `bfa30c2` — Story 1.3: Config struct, YAML parsing
- `dccde3b` — Stories 1.1 + 1.2: scaffold + error types

**Patterns from git:**
- Single commit per story with descriptive message
- All code changes + test changes in one commit
- Sprint status updated in same commit

### Architecture Compliance Checklist

- [ ] `session` does NOT import `config` (accepts Options struct)
- [ ] `session` does NOT import `runner`, `bridge`, or `gates`
- [ ] Uses `exec.CommandContext(ctx)`, NEVER `exec.Command`
- [ ] Separate stdout/stderr buffers, NEVER `CombinedOutput()`
- [ ] `cmd.Dir` set explicitly to `opts.Dir`
- [ ] `cmd.Env` set to `os.Environ()` (inherit environment)
- [ ] Exit code extracted via `exec.ExitError.ExitCode()`
- [ ] Error wrapping: `"session: claude: exit %d: %w"` format
- [ ] CLI flag constants as `const` block at package scope (not inline strings)
- [ ] No `os.Exit` calls in session package
- [ ] No logging/printing from session package (returns errors)
- [ ] No new external dependencies (stdlib only)
- [ ] `session.Execute` is the ONLY exported function (buildArgs, flag constants are unexported)
- [ ] Minimal exported API: only `Options`, `RawResult`, `Execute` are exported
- [ ] Test file uses `package session` (not `package session_test`) for access to `buildArgs`
- [ ] Test naming: `Test<Type>_<Method>_<Scenario>`
- [ ] Table-driven tests for buildArgs with multiple option combinations

### Anti-Patterns (FORBIDDEN)

- `exec.Command` without context (MUST use `exec.CommandContext`)
- `cmd.CombinedOutput()` (MUST use separate stdout/stderr buffers)
- Importing `config` package (session receives everything via Options)
- Inline string literals for CLI flags (MUST use constants)
- `os.Exit` or `log.Fatal` in session package
- `context.TODO()` or `context.Background()` in Execute (use provided ctx)
- Parsing JSON output (Story 1.8 scope)
- Implementing resume behavior logic (Story 1.9 scope)
- Creating mock Claude infrastructure (Story 1.11 scope)
- Hardcoding "claude" as command — use `opts.Command`

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.7]
- [Source: docs/project-context.md#Subprocess]
- [Source: docs/project-context.md#Architecture — Ключевые решения]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Subprocess Patterns]
- [Source: docs/architecture/core-architectural-decisions.md#Subprocess & Git]
- [Source: docs/architecture/project-structure-boundaries.md — session/ directory]
- [Source: docs/prd/functional-requirements.md#FR7 — fresh session per task]
- [Source: docs/prd/functional-requirements.md#FR9 — retry with resume]
- [Source: docs/prd/functional-requirements.md#FR10 — max turns limit]
- [Source: docs/prd/functional-requirements.md#FR14 — separate review session]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

- Initial attempt with bash test scripts failed: Windows Go (`go.exe`) cannot execute bash scripts directly (`%1 is not a valid Win32 application`). Switched to Go test binary self-reexec pattern (TestMain + SESSION_TEST_HELPER env var), which is the standard Go stdlib approach for testing exec.Command.

### Completion Notes List

- Replaced placeholder `session/session.go` (just `package session`) with full implementation: Options struct, RawResult struct, Execute function, buildArgs helper, 6 CLI flag constants
- All 6 CLI flag constants defined as unexported `const` block at package scope
- Execute uses `exec.CommandContext` (never `exec.Command`), separate stdout/stderr buffers (never `CombinedOutput`)
- Error wrapping follows project convention: `"session: claude: exit %d: %w"` for exit errors, `"session: claude: %w"` for other errors (e.g., command not found)
- Test strategy: self-reexec pattern via TestMain (standard Go pattern for exec testing on Windows). TestMain checks SESSION_TEST_HELPER env var; if set, runs the corresponding subprocess scenario and exits. Tests use `t.Setenv` + `os.Args[0]` as Command to re-invoke the test binary as a controlled subprocess.
- 16 total tests: 10 table-driven buildArgs cases (incl. zero-value) + 6 Execute scenario tests (success, non-zero exit, context cancellation, command not found, separate stdout/stderr, working dir)
- WorkingDir test uses `os.SameFile` for path comparison to handle Windows 8.3 short name differences
- All 38 existing config tests pass (no regressions)
- No new external dependencies (stdlib only: bytes, context, fmt, os, os/exec, strconv)
- session package does NOT import config (architecture boundary maintained)

### File List

- `session/session.go` — modified (replaced placeholder with full implementation)
- `session/session_test.go` — new (comprehensive unit tests)
- `docs/sprint-artifacts/sprint-status.yaml` — modified (1-7 status: ready-for-dev → in-progress → review)
- `docs/sprint-artifacts/1-7-session-basic-os-exec-stdout-capture.md` — new (story file created by create-story, updated with task completions and dev agent record)

## Senior Developer Review (AI)

**Review Date:** 2026-02-25
**Review Outcome:** Changes Requested (auto-fixed)
**Reviewer Model:** Claude Opus 4.6

### Action Items

- [x] [HIGH] Replace type assertion `err.(*exec.ExitError)` with `errors.As` in session.go:72 — project coding standard violation (session.go:72)
- [x] [MEDIUM] Extract inline string `"json"` to `outputFormatJSON` constant — AC requires constants for CLI args (session.go:101)
- [x] [MEDIUM] Add `default` case to `runTestHelper` — unknown scenarios silently succeed (session_test.go:41)
- [x] [LOW] Fix completion notes test count: "14 (9+5)" → "16 (10+6)" (story file)
- [x] [LOW] Add zero-value `Options{}` test case to buildArgs table (session_test.go)
- [x] [LOW] Fix File List: `1-7-*.md` is "new" (git ??), not "modified" (story file)

**Summary:** 6 issues found (1 High, 2 Medium, 3 Low), all auto-fixed in this session.

## Change Log

- 2026-02-25: Implemented Story 1.7 — session package with Execute function, Options/RawResult structs, CLI flag constants, buildArgs helper. 16 comprehensive tests covering args construction, exit code extraction, error wrapping, context cancellation, separate stdout/stderr buffers, and working directory propagation. Used Go test binary self-reexec pattern instead of bash scripts for Windows Go compatibility.
- 2026-02-25: Code review fixes — replaced type assertion with errors.As (project standard), extracted "json" to outputFormatJSON constant, added default case to runTestHelper, added zero-value Options test, fixed documentation inaccuracies.
