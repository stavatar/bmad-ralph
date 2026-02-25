# Story 1.13: CLI Wiring — Cobra, Signal Handling, Exit Codes

Status: done

## Story

As a developer,
I want `ralph --help`, `ralph bridge`, and `ralph run` commands wired with Cobra,
so that the CLI is usable with proper help, flags, signal handling, and exit codes. (FR35)

## Acceptance Criteria

1. `ralph --help` shows available commands (`bridge`, `run`), global flags, and version (`var version = "dev"`)
2. `ralph bridge --help` shows bridge subcommand usage (positional args: story files)
3. `ralph run --help` shows run-specific flags:
   | Flag               | Type   | Default | Description                  |
   |--------------------|--------|---------|------------------------------|
   | `--max-turns`      | int    | 0       | Max turns per session (0=config) |
   | `--gates`          | bool   | false   | Enable human gates           |
   | `--every`          | int    | 0       | Checkpoint gate interval     |
   | `--model`          | string | ""      | Override model               |
   | `--always-extract` | bool   | false   | Extract after every execute  |
4. Signal handling:
   - `signal.NotifyContext(ctx, os.Interrupt)` in main()
   - First Ctrl+C propagates cancellation to all subprocesses with graceful message
   - Second Ctrl+C: force kill via `os.Exit` (NFR13 double signal handler)
5. Exit code mapping:
   | Exit Code | Condition                          | Error Type                          |
   |-----------|------------------------------------|-------------------------------------|
   | 0         | All tasks completed                | `err == nil`                        |
   | 1         | Partial success (limits, gates off) | `ExitCodeError{Code: 1}`           |
   | 2         | User quit (at gate)                | `GateDecision{Action: "quit"}`     |
   | 3         | Interrupted (Ctrl+C)               | `context.Canceled`                 |
   | 4         | Fatal error                        | Any other error                    |
6. Log directory `.ralph/logs/` created; log file `run-{timestamp}.log` written for `run` subcommand
7. Colored output uses `fatih/color` (auto-disabled in non-TTY)
8. `cmd/ralph/*.go` contains ONLY wiring — no business logic:
   - `bridge.go` calls `bridge.Run(ctx, cfg, args)`
   - `run.go` calls `runner.Run(ctx, cfg)`
9. Tests verify:
   - Exit code mapping from all error types to correct codes (table-driven)
   - Wrapped errors (double-wrapped) resolve correctly
   - Zero-value error types behave correctly
   - Run subcommand has correct flags defined with correct types
   - Root command has `bridge` and `run` subcommands
   - Version is set

## Tasks / Subtasks

**CRLF REMINDER:** After writing ANY file with Write/Edit tools, run: `sed -i 's/\r$//' <file>` (Windows NTFS creates CRLF)

- [x] Task 1: Add `runner.Run` entry point stub (AC: 8)
  - [x] 1.1 Add to `runner/runner.go`:
    ```go
    // Run is the main entry point for the execute-review loop.
    // Story 3.5 implements the full loop. This stub validates CLI wiring.
    func Run(ctx context.Context, cfg *config.Config) error {
        return fmt.Errorf("runner: loop not implemented")
    }
    ```
  - [x] 1.2 No new imports needed (`context`, `fmt`, `config` already imported)
  - [x] 1.3 This is a STUB — full implementation is Story 3.5 (runner loop skeleton). Signature matches architecture entry point: `runner.Run(ctx, cfg)`

- [x] Task 2: Add `bridge.Run` entry point stub (AC: 8)
  - [x] 2.1 Replace `bridge/bridge.go` content:
    ```go
    package bridge

    import (
        "context"
        "fmt"

        "github.com/bmad-ralph/bmad-ralph/config"
    )

    // Run converts story files to sprint-tasks.md.
    // Epic 2 (Story 2.3) implements the full bridge logic.
    func Run(ctx context.Context, cfg *config.Config, storyFiles []string) error {
        return fmt.Errorf("bridge: not implemented")
    }
    ```
  - [x] 2.2 This is a STUB — full implementation is Epic 2. Accepts `storyFiles []string` for positional args from CLI
  - [x] 2.3 **Signature deviation:** Architecture entry point table says `bridge.Run(ctx, cfg)` without args. Adding `storyFiles []string` is necessary because `config.Config` has no StoryFiles field. This is an acceptable deviation — Epic 2 may reconcile

- [x] Task 3: Create exit code mapping (AC: 5, 9)
  - [x] 3.1 Create `cmd/ralph/exit.go`:
    ```go
    package main

    import (
        "context"
        "errors"

        "github.com/bmad-ralph/bmad-ralph/config"
    )

    // Exit code constants matching PRD exit code table.
    const (
        exitSuccess     = 0
        exitPartial     = 1
        exitUserQuit    = 2
        exitInterrupted = 3
        exitFatal       = 4
    )

    // exitCode maps an error to the appropriate process exit code.
    // nil → 0, ExitCodeError → its Code, GateDecision(quit) → 2,
    // context.Canceled → 3, everything else → 4.
    func exitCode(err error) int {
        if err == nil {
            return exitSuccess
        }

        var exitErr *config.ExitCodeError
        if errors.As(err, &exitErr) {
            return exitErr.Code
        }

        var gate *config.GateDecision
        if errors.As(err, &gate) {
            if gate.Action == "quit" {
                return exitUserQuit
            }
        }

        if errors.Is(err, context.Canceled) {
            return exitInterrupted
        }

        return exitFatal
    }
    ```
  - [x] 3.2 **Order matters:** `ExitCodeError` checked BEFORE `GateDecision` — if an error wraps both (shouldn't happen), ExitCodeError takes priority
  - [x] 3.3 `errors.As` with pointer receiver — requires `*config.ExitCodeError` and `*config.GateDecision` (both use pointer receiver for `Error()` method)

- [x] Task 4: Create Cobra root command + signal handling (AC: 1, 4, 7)
  - [x] 4.1 Rewrite `cmd/ralph/main.go`:
    ```go
    package main

    import (
        "context"
        "os"
        "os/signal"

        "github.com/fatih/color"
        "github.com/spf13/cobra"
    )

    var version = "dev"

    var rootCmd = &cobra.Command{
        Use:     "ralph",
        Version: version,
        Short:   "Orchestrate Claude Code sessions for autonomous development",
        Long: `Ralph orchestrates Claude Code sessions in an execute → review loop
    for autonomous software development. It reads sprint-tasks.md and executes
    each task through Claude Code, then reviews the result.`,
    }

    func main() {
        os.Exit(run())
    }

    // run is the real entry point. Separated from main() so defers execute
    // before os.Exit and for testability.
    func run() int {
        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
        defer stop()

        // Double-signal handler: second Ctrl+C force-exits (NFR13).
        go func() {
            <-ctx.Done()
            // Context canceled by first signal. Listen for second.
            stop()
            sigCh := make(chan os.Signal, 1)
            signal.Notify(sigCh, os.Interrupt)
            <-sigCh
            color.Red("\nForce exit (second interrupt)")
            os.Exit(exitInterrupted)
        }()

        rootCmd.SilenceErrors = true
        rootCmd.SilenceUsage = true

        err := rootCmd.ExecuteContext(ctx)
        code := exitCode(err)

        if err != nil {
            switch code {
            case exitInterrupted:
                color.Yellow("\nInterrupted")
            default:
                color.Red("Error: %v", err)
            }
        }

        return code
    }

    func init() {
        rootCmd.AddCommand(runCmd)
        rootCmd.AddCommand(bridgeCmd)
    }
    ```
  - [x] 4.2 `run()` separated from `main()` so defers execute before `os.Exit` (common Go pattern)
  - [x] 4.3 `SilenceErrors = true` — we handle error display ourselves with colored output
  - [x] 4.4 `SilenceUsage = true` — don't print usage on RunE errors (noisy)
  - [x] 4.5 `rootCmd.ExecuteContext(ctx)` — passes signal-aware context to all subcommands
  - [x] 4.6 Double-signal goroutine: after first signal cancels ctx, `stop()` resets signal handling, then manually listens for second signal → `os.Exit(3)`
  - [x] 4.7 Remove blank imports (`_ "gopkg.in/yaml.v3"`) — no longer needed since bridge.go now imports config which imports yaml.v3. Verify with `go mod tidy` that all deps are retained

- [x] Task 5: Create run subcommand (AC: 2, 3, 6, 8)
  - [x] 5.1 Create `cmd/ralph/run.go`:
    ```go
    package main

    import (
        "fmt"
        "os"
        "path/filepath"
        "time"

        "github.com/fatih/color"
        "github.com/spf13/cobra"

        "github.com/bmad-ralph/bmad-ralph/config"
        "github.com/bmad-ralph/bmad-ralph/runner"
    )

    var runCmd = &cobra.Command{
        Use:   "run",
        Short: "Execute tasks from sprint-tasks.md",
        Long: `Run the execute → review loop on sprint-tasks.md.
    Each task is executed in a fresh Claude Code session, reviewed,
    and retried if findings are found.`,
        RunE: runRun,
    }

    func init() {
        runCmd.Flags().Int("max-turns", 0, "Max turns per Claude session (0 = use config/default)")
        runCmd.Flags().Bool("gates", false, "Enable human gates")
        runCmd.Flags().Int("every", 0, "Checkpoint gate every N tasks (0 = off)")
        runCmd.Flags().String("model", "", "Override Claude model for execute")
        runCmd.Flags().Bool("always-extract", false, "Extract knowledge after every execute")
    }

    func runRun(cmd *cobra.Command, args []string) error {
        flags := buildCLIFlags(cmd)

        cfg, err := config.Load(flags)
        if err != nil {
            return fmt.Errorf("ralph: load config: %w", err)
        }

        if err := ensureLogDir(cfg); err != nil {
            color.Yellow("Warning: could not create log directory: %v", err)
        }

        return runner.Run(cmd.Context(), cfg)
    }

    // buildCLIFlags converts Cobra flag values to config.CLIFlags.
    // Only flags explicitly set by the user are populated (pointer non-nil).
    func buildCLIFlags(cmd *cobra.Command) config.CLIFlags {
        var flags config.CLIFlags

        if cmd.Flags().Changed("max-turns") {
            v, _ := cmd.Flags().GetInt("max-turns")
            flags.MaxTurns = &v
        }
        if cmd.Flags().Changed("gates") {
            v, _ := cmd.Flags().GetBool("gates")
            flags.GatesEnabled = &v
        }
        if cmd.Flags().Changed("every") {
            v, _ := cmd.Flags().GetInt("every")
            flags.GatesCheckpoint = &v
        }
        if cmd.Flags().Changed("model") {
            v, _ := cmd.Flags().GetString("model")
            flags.ModelExecute = &v
        }
        if cmd.Flags().Changed("always-extract") {
            v, _ := cmd.Flags().GetBool("always-extract")
            flags.AlwaysExtract = &v
        }

        return flags
    }

    // ensureLogDir creates the log directory for run logs.
    func ensureLogDir(cfg *config.Config) error {
        logDir := filepath.Join(cfg.ProjectRoot, cfg.LogDir)
        if err := os.MkdirAll(logDir, 0755); err != nil {
            return fmt.Errorf("ralph: create log dir: %w", err)
        }

        ts := time.Now().Format("2006-01-02-150405")
        logPath := filepath.Join(logDir, fmt.Sprintf("run-%s.log", ts))

        f, err := os.Create(logPath)
        if err != nil {
            return fmt.Errorf("ralph: create log file: %w", err)
        }
        defer f.Close()

        _, err = fmt.Fprintf(f, "%s INFO ralph run started\n", time.Now().Format("2006-01-02T15:04:05"))
        return err
    }
    ```
  - [x] 5.2 `buildCLIFlags` uses `cmd.Flags().Changed()` to detect explicitly-set flags — unset flags → nil pointer → config cascade uses file/default. This is the correct pattern for CLIFlags pointer fields
  - [x] 5.6 **DO NOT ADD extra CLI flags.** `config.CLIFlags` has 9 fields but Story 1.13 exposes ONLY the 5 listed above per AC. Fields `MaxIterations`, `MaxReviewIterations`, `ReviewEvery`, `ModelReview` exist in CLIFlags for future stories but MUST NOT be wired as flags here
  - [x] 5.3 `ensureLogDir` creates `.ralph/logs/` and writes a start entry. Full logging (append during run) is Story 3.5's concern. Story 1.13 ensures the directory and file creation works. Story 3.5 will refactor to return `*os.File` handle for append-only logging throughout the run
  - [x] 5.4 Error from `ensureLogDir` is a WARNING, not fatal — don't block `ralph run` due to log directory issues
  - [x] 5.5 `runner.Run` receives `cmd.Context()` which carries the signal-aware context from main

- [x] Task 6: Create bridge subcommand (AC: 2, 8)
  - [x] 6.1 Create `cmd/ralph/bridge.go`:
    ```go
    package main

    import (
        "fmt"

        "github.com/spf13/cobra"

        "github.com/bmad-ralph/bmad-ralph/bridge"
        "github.com/bmad-ralph/bmad-ralph/config"
    )

    var bridgeCmd = &cobra.Command{
        Use:   "bridge [story-files...]",
        Short: "Convert story files to sprint-tasks.md",
        Long: `Bridge converts BMad story files into a structured sprint-tasks.md
    for execution by the run command.`,
        RunE: runBridge,
    }

    func runBridge(cmd *cobra.Command, args []string) error {
        cfg, err := config.Load(config.CLIFlags{})
        if err != nil {
            return fmt.Errorf("ralph: load config: %w", err)
        }

        return bridge.Run(cmd.Context(), cfg, args)
    }
    ```
  - [x] 6.2 Bridge takes positional args (story file paths). No custom flags for Story 1.13 — `--merge` flag is Story 2.6
  - [x] 6.3 `config.CLIFlags{}` — bridge has no CLI flags that override config in Epic 1

- [x] Task 7: Create tests (AC: 9)
  - [x] 7.1 Create `cmd/ralph/exit_test.go` — exit code mapping tests (table-driven):
    ```go
    package main

    import (
        "context"
        "fmt"
        "testing"

        "github.com/bmad-ralph/bmad-ralph/config"
    )

    func TestExitCode_TableDriven(t *testing.T) {
        tests := []struct {
            name string
            err  error
            want int
        }{
            {"nil error", nil, exitSuccess},
            {"ExitCodeError partial", &config.ExitCodeError{Code: 1, Message: "partial"}, exitPartial},
            {"ExitCodeError fatal", &config.ExitCodeError{Code: 4, Message: "crash"}, exitFatal},
            {"GateDecision quit", &config.GateDecision{Action: "quit", Feedback: ""}, exitUserQuit},
            {"GateDecision skip", &config.GateDecision{Action: "skip"}, exitFatal},
            {"context.Canceled", context.Canceled, exitInterrupted},
            {"generic error", fmt.Errorf("something broke"), exitFatal},
            // Wrapped errors — realistic multi-layer call stack
            {"wrapped ExitCodeError", fmt.Errorf("runner: %w", &config.ExitCodeError{Code: 1, Message: "limits"}), exitPartial},
            {"wrapped GateDecision quit", fmt.Errorf("gates: %w", &config.GateDecision{Action: "quit"}), exitUserQuit},
            {"wrapped context.Canceled", fmt.Errorf("runner: execute: %w", context.Canceled), exitInterrupted},
            {"double-wrapped Canceled", fmt.Errorf("ralph: %w", fmt.Errorf("runner: %w", context.Canceled)), exitInterrupted},
            {"double-wrapped ExitCodeError", fmt.Errorf("ralph: %w", fmt.Errorf("runner: %w", &config.ExitCodeError{Code: 2, Message: "quit"})), exitUserQuit},
            // Sentinel errors — map to exitFatal. Runner should wrap in ExitCodeError{Code:1} for partial success
            {"ErrMaxRetries", config.ErrMaxRetries, exitFatal},
            {"ErrNoTasks", config.ErrNoTasks, exitFatal},
            {"wrapped ErrMaxRetries", fmt.Errorf("runner: %w", config.ErrMaxRetries), exitFatal},
        }

        for _, tt := range tests {
            t.Run(tt.name, func(t *testing.T) {
                got := exitCode(tt.err)
                if got != tt.want {
                    t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
                }
            })
        }
    }

    func TestExitCode_ZeroValueExitCodeError(t *testing.T) {
        // Zero-value ExitCodeError has Code=0 → exitSuccess
        err := &config.ExitCodeError{}
        if got := exitCode(err); got != exitSuccess {
            t.Errorf("exitCode(ExitCodeError{}) = %d, want %d", got, exitSuccess)
        }
    }

    func TestExitCode_ZeroValueGateDecision(t *testing.T) {
        // Zero-value GateDecision has Action="" → not "quit" → falls through to exitFatal
        err := &config.GateDecision{}
        if got := exitCode(err); got != exitFatal {
            t.Errorf("exitCode(GateDecision{}) = %d, want %d", got, exitFatal)
        }
    }
    ```
  - [x] 7.2 Create `cmd/ralph/cmd_test.go` — command structure and flag tests:
    ```go
    package main

    import "testing"

    func TestRootCmd_HasSubcommands(t *testing.T) {
        want := map[string]bool{"bridge": false, "run": false}

        for _, cmd := range rootCmd.Commands() {
            if _, ok := want[cmd.Name()]; ok {
                want[cmd.Name()] = true
            }
        }

        for name, found := range want {
            if !found {
                t.Errorf("subcommand %q not registered on root command", name)
            }
        }
    }

    func TestRootCmd_Version(t *testing.T) {
        if rootCmd.Version == "" {
            t.Error("rootCmd.Version is empty")
        }
        if rootCmd.Version != version {
            t.Errorf("rootCmd.Version = %q, want %q", rootCmd.Version, version)
        }
    }

    func TestRunCmd_Flags(t *testing.T) {
        tests := []struct {
            flag     string
            flagType string
        }{
            {"max-turns", "int"},
            {"gates", "bool"},
            {"every", "int"},
            {"model", "string"},
            {"always-extract", "bool"},
        }

        for _, tt := range tests {
            t.Run(tt.flag, func(t *testing.T) {
                f := runCmd.Flags().Lookup(tt.flag)
                if f == nil {
                    t.Fatalf("flag --%s not defined on run command", tt.flag)
                }
                if f.Value.Type() != tt.flagType {
                    t.Errorf("flag --%s type = %q, want %q", tt.flag, f.Value.Type(), tt.flagType)
                }
            })
        }
    }

    func TestBridgeCmd_Usage(t *testing.T) {
        if bridgeCmd.Use == "" {
            t.Error("bridgeCmd.Use is empty")
        }
        // Bridge accepts positional args
        if bridgeCmd.Use != "bridge [story-files...]" {
            t.Errorf("bridgeCmd.Use = %q, want %q", bridgeCmd.Use, "bridge [story-files...]")
        }
    }
    ```
  - [x] 7.3 Test naming follows convention: `TestExitCode_TableDriven`, `TestExitCode_ZeroValueExitCodeError`, `TestRootCmd_HasSubcommands`, etc.
  - [x] 7.4 Always use `t.Errorf`/`t.Fatalf`, NEVER `t.Logf` in assertion blocks (CLAUDE.md learning)
  - [x] 7.5 Error tests verify both exit code value AND that wrapped errors resolve correctly via `errors.As`/`errors.Is`

- [x] Task 8: Run tests and verify (AC: all)
  - [x] 8.1 Run unit tests: `"/mnt/c/Program Files/Go/bin/go.exe" test ./cmd/ralph/ -v`
  - [x] 8.2 Run full test suite for regressions: `"/mnt/c/Program Files/Go/bin/go.exe" test ./... -v`
  - [x] 8.3 Run integration tests (should still pass): `"/mnt/c/Program Files/Go/bin/go.exe" test ./runner/ -tags=integration -v`
  - [x] 8.4 Build binary and test help output:
    ```bash
    "/mnt/c/Program Files/Go/bin/go.exe" build -o ralph ./cmd/ralph
    ./ralph --help
    ./ralph run --help
    ./ralph bridge --help
    ./ralph --version
    ```
  - [x] 8.5 Verify no new external dependencies: `"/mnt/c/Program Files/Go/bin/go.exe" mod tidy` should not change go.mod
  - [x] 8.6 Verify dependency direction: `cmd/ralph/` imports `runner`, `bridge`, `config`, `cobra`, `color`, and stdlib ONLY. No `session`, `gates` imports in cmd/ralph/
  - [x] 8.7 Verify blank imports removed from main.go: `_ "gopkg.in/yaml.v3"` no longer needed. Confirm all 3 deps retained after `go mod tidy` (cobra, yaml.v3, color used transitively)
  - [x] 8.8 Update story file with completion notes and file list

## Dev Notes

### Architecture Context

**CLI Structure (from architecture):**
- `cmd/ralph/` = ONLY wiring, no business logic
- `cmd/ralph/` = ONLY place for exit code mapping (0-4) and output decisions
- Packages return errors, NEVER call `os.Exit`
- Cobra root command + subcommands (`bridge`, `run`)
- `var version = "dev"` overridden by goreleaser ldflags: `-X main.version=v1.0.0`

**Signal Handling (from architecture):**
- `main()` creates root ctx via `signal.NotifyContext(ctx, os.Interrupt)`
- Context propagated through all function parameters
- No `context.TODO()` in production code
- All subprocess use `exec.CommandContext(ctx)` (already implemented in session package)

**Exit Code Table (from PRD/Architecture):**
| Code | Condition | Maps From |
|------|-----------|-----------|
| 0 | Success | `err == nil` |
| 1 | Partial success | `ExitCodeError{Code: 1}` |
| 2 | User quit at gate | `GateDecision{Action: "quit"}` |
| 3 | Interrupted | `context.Canceled` |
| 4 | Fatal error | Everything else |

**Logging (from architecture):**
- `fmt.Fprintf(logFile, "%s %s %s\n", ts, level, msg)` — append-only
- Packages DO NOT log — return results/errors
- `cmd/ralph/` decides what goes to stdout vs log
- Claude output: don't log entirely, only session_id, exit code, duration

### Existing Code Patterns to Follow

- **`config/config.go:CLIFlags`** — Pointer fields (`*int`, `*string`, `*bool`) for "was set" tracking. `nil` = not set by CLI, use config/default cascade
- **`config/errors.go:ExitCodeError`** — `Code int` + `Message string`, implements `error`. Use `errors.As(err, &exitErr)` with `*config.ExitCodeError`
- **`config/errors.go:GateDecision`** — `Action string` + `Feedback string`, implements `error`. Use `errors.As(err, &gate)` with `*config.GateDecision`
- **`runner/runner.go:RunOnce`** — Returns `error`, wraps as `fmt.Errorf("runner: <op>: %w", err)`
- **`session/session.go:Execute`** — Uses `exec.CommandContext(ctx)` — signal cancellation already propagated
- **`config/config.go:Load`** — `Load(flags CLIFlags) (*Config, error)` — cascade: CLI > file > embedded
- **`config/config.go:Config.LogDir`** — `string`, default `".ralph/logs"` — relative to ProjectRoot

### Context Cancellation and Exit Codes (Note for Story 3.5)

When Ctrl+C kills a running subprocess, the error chain from `session.Execute` may contain `exec.ExitError` (signal: killed) rather than `context.Canceled`. For Story 1.13 stubs this doesn't matter — the stub returns immediately. Story 3.5 (runner loop) should check `ctx.Err() == context.Canceled` alongside the returned error to correctly detect interruption vs subprocess failure.

### Why `runner.Run` and `bridge.Run` are stubs

- `runner.Run(ctx, cfg)` — Full runner loop is Story 3.5. Requires: `ExecGitClient` (Story 3.3), sprint-tasks scanner (Story 3.2), execute prompt (Story 3.1). Walking skeleton (Story 1.12) validated RunOnce via integration tests
- `bridge.Run(ctx, cfg, args)` — Full bridge is Epic 2. Requires: bridge prompt (Story 2.2), bridge logic (Story 2.3), bridge golden files (Story 2.5)
- Story 1.13 focus: CLI wiring, signal handling, exit codes. Stubs are sufficient to validate the wiring end-to-end

### Previous Story Intelligence

**From Story 1.12 (walking skeleton):**
- `RunOnce` and `RunReview` proven working end-to-end with mock Claude
- go:embed templates work for prompt assembly
- Error wrapping consistently uses `fmt.Errorf("runner: <op>: %w", err)`
- Integration tests use self-reexec pattern (`testutil.RunMockClaude()`)
- `config.Config` fields are exported — can be constructed directly in tests

**From Story 1.4 (CLI override):**
- `CLIFlags` pointer fields tested and working
- `config.Load(flags)` handles nil pointers gracefully
- Three-level cascade validated: CLI > file > embedded defaults

**From Story 1.2 (error types):**
- `ExitCodeError` and `GateDecision` implement `error` interface with pointer receivers
- `errors.As` tested with table-driven multi-field scenarios and zero values
- Double-wrapped error unwrapping works correctly

**From CLAUDE.md review learnings:**
- Zero-value test for custom error types (ExitCodeError{}, GateDecision{})
- Error tests MUST verify message content with `strings.Contains`
- `t.Errorf`/`t.Fatalf`, NEVER `t.Logf` in assertion blocks
- Duplicate test helpers must be merged
- Windows Go path: `"/mnt/c/Program Files/Go/bin/go.exe"` for all go commands
- After Write/Edit, always `sed -i 's/\r$//' <file>` for CRLF fix

### Project Structure Notes

- **New files (to be created):**
  - `cmd/ralph/exit.go` — exit code constants + mapping function
  - `cmd/ralph/run.go` — Cobra run subcommand + flag wiring
  - `cmd/ralph/bridge.go` — Cobra bridge subcommand
  - `cmd/ralph/exit_test.go` — exit code mapping tests
  - `cmd/ralph/cmd_test.go` — command structure and flag tests
- **Modified files:**
  - `cmd/ralph/main.go` — complete rewrite (Cobra root, signal handling)
  - `runner/runner.go` — add `Run(ctx, cfg)` stub entry point
  - `bridge/bridge.go` — add `Run(ctx, cfg, args)` stub entry point
- **No deleted files**
- **Dependency direction verified:**
  - `cmd/ralph/` → `runner`, `bridge`, `config` (direct)
  - `cmd/ralph/` → `cobra`, `color` (external)
  - `cmd/ralph/` does NOT import `session` or `gates`

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.13] — AC, prerequisites, technical notes
- [Source: docs/prd/cli-tool-specific-requirements.md] — Command structure, flags table, exit codes
- [Source: docs/prd/functional-requirements.md#FR35] — Informative exit codes
- [Source: docs/architecture/core-architectural-decisions.md] — Cobra, fatih/color, signal handling decisions
- [Source: docs/architecture/project-structure-boundaries.md] — cmd/ralph/ file layout, .ralph/logs/ structure
- [Source: docs/project-context.md] — Architecture summary, dependency direction, naming conventions
- [Source: config/config.go] — CLIFlags struct, Load() signature, Config fields
- [Source: config/errors.go] — ExitCodeError, GateDecision types
- [Source: runner/runner.go] — RunOnce, RunReview, GitClient, RunConfig
- [Source: docs/sprint-artifacts/1-12-walking-skeleton-minimal-end-to-end-pass.md] — Previous story implementation details

## Senior Developer Review (AI)

**Review Date:** 2026-02-25
**Review Outcome:** Changes Requested → All Fixed
**Reviewer Model:** Claude Opus 4.6

### Action Items

- [x] [HIGH] buildCLIFlags wiring completely untested — flag→CLIFlags field mapping has zero coverage
- [x] [HIGH] Flag default values not tested — AC 3 specifies exact defaults, no test verifies them
- [x] [MED] Inconsistent error wrapping in ensureLogDir — Fprintf error returned bare, violates project convention
- [x] [MED] Missing test case for context.DeadlineExceeded — defensive coverage gap
- [x] [MED] No test coverage for ensureLogDir — 3 error paths, creates real files, zero tests
- [x] [LOW] Test case naming inconsistency — "double-wrapped" uses hyphens vs spaces elsewhere
- [x] [LOW] Completion notes test count inaccurate — fixed from "22/22" to actual counts

### Summary

7 issues found (2 High, 3 Medium, 2 Low). All resolved in same session.
Key additions: TestBuildCLIFlags_WiringCorrectness (verifies flag→struct mapping),
TestBuildCLIFlags_NoFlagsChanged (nil verification), flag DefValue checks,
TestEnsureLogDir_* (3 tests: happy path, invalid path, read-only dir),
context.DeadlineExceeded defensive test case.

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No debug issues encountered.

### Completion Notes List

- Task 1: Added `runner.Run(ctx, cfg)` stub to `runner/runner.go` — returns "loop not implemented" error. Signature matches architecture entry point table
- Task 2: Replaced `bridge/bridge.go` with `bridge.Run(ctx, cfg, storyFiles)` stub. Added `storyFiles []string` param as noted deviation (config.Config has no StoryFiles field)
- Task 3: Created `cmd/ralph/exit.go` with exit code constants (0-4) and `exitCode(err)` mapping function. Priority order: ExitCodeError > GateDecision > context.Canceled > fatal
- Task 4: Rewrote `cmd/ralph/main.go` with Cobra root command, signal.NotifyContext, double-signal handler (NFR13), colored error output. Removed blank imports (`_ "gopkg.in/yaml.v3"`)
- Task 5: Created `cmd/ralph/run.go` with 5 CLI flags (max-turns, gates, every, model, always-extract), buildCLIFlags using Changed() pattern, ensureLogDir for log file creation
- Task 6: Created `cmd/ralph/bridge.go` with positional args, no custom flags
- Task 7: Created exit_test.go (16 table-driven cases + 2 zero-value tests) and cmd_test.go (subcommand, version, flag type+default, usage, buildCLIFlags wiring, ensureLogDir tests)
- Task 8: All tests pass (cmd/ralph 13 top-level tests, config cached, session cached, runner integration 7/7). Binary builds and shows correct --help, --version, run --help, bridge --help. go mod tidy = no changes. Dependency direction verified (no session/gates imports)

### Change Log

- 2026-02-25: Story 1.13 implemented — CLI wiring with Cobra, signal handling, exit code mapping
- 2026-02-25: Code review fixes — 7 issues (2H, 3M, 2L) addressed: added buildCLIFlags wiring test, flag default value tests, ensureLogDir tests, context.DeadlineExceeded test case, fixed error wrapping consistency, fixed test naming

### File List

- cmd/ralph/main.go (modified — complete rewrite with Cobra root + signal handling)
- cmd/ralph/exit.go (new — exit code constants + mapping function)
- cmd/ralph/run.go (new — run subcommand with 5 CLI flags)
- cmd/ralph/bridge.go (new — bridge subcommand with positional args)
- cmd/ralph/exit_test.go (new — exit code mapping tests, 17 test cases)
- cmd/ralph/cmd_test.go (new — command structure and flag tests)
- runner/runner.go (modified — added Run stub entry point)
- bridge/bridge.go (modified — added Run stub entry point)
- docs/sprint-artifacts/sprint-status.yaml (modified — story status updated)
- docs/sprint-artifacts/1-13-cli-wiring-cobra-signal-handling-exit-codes.md (modified — task completion, dev record)
