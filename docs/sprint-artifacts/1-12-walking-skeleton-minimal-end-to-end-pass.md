# Story 1.12: Walking Skeleton — Minimal End-to-End Pass

Status: done

## Story

As a developer,
I want a minimal integration test proving config → session → execute one task works,
so that the architecture is validated before building features on top.

## Acceptance Criteria

1. A valid config loads with mock Claude command substituted via `config.ClaudeCommand`
2. A hand-crafted sprint-tasks.md fixture exists with one task: `- [ ] Implement hello world`
3. Mock Claude scenario: 1 execute (exit 0, creates commit) + 1 review (exit 0, clean)
4. Mock git: a stub `GitClient` interface with `mockGitClient` that returns `HealthCheck OK` and `HasNewCommit true`
5. The walking skeleton integration test runs end-to-end:
   - Config loads successfully
   - `session.Execute` is called with assembled prompt (via `config.AssemblePrompt`)
   - Mock Claude receives correct flags (`-p`, `--max-turns`, `--output-format json`, `--dangerously-skip-permissions`)
   - `session.ParseResult` parses the mock response correctly
   - Mock git confirms commit exists (stub — real GitClient interface in Story 3.3)
   - Task is logically marked done (output validated, not file mutation yet)
6. The test includes a stub review step: mock Claude returns clean review (validates runner↔review integration point exists)
7. The test uses a hand-crafted sprint-tasks.md fixture matching the shared format contract (Story 2.1 will formalize format)
8. Test file: `runner/runner_integration_test.go` with build tag `//go:build integration`
9. Test uses `t.TempDir()` for isolation

## Tasks / Subtasks

**CRLF REMINDER:** After writing ANY file with Write/Edit tools, run: `sed -i 's/\r$//' <file>` (Windows NTFS creates CRLF)

- [x] Task 1: Create sprint-tasks.md test fixture (AC: 2, 7)
  - [x] 1.1 Create `runner/testdata/sprint-tasks-basic.md` — must use `config.TaskOpen` marker (`- [ ]`):
    ```markdown
    # Sprint Tasks

    ## Epic 1: Foundation

    - [ ] Implement hello world
    ```
  - [x] 1.2 Delete `runner/testdata/.gitkeep` AFTER fixture is created (directory now has real content)

- [x] Task 2: Create minimal runner scaffolding for walking skeleton (AC: 4, 5)
  - [x] 2.1 In `runner/runner.go`, define minimal `GitClient` interface:
    ```go
    type GitClient interface {
        HealthCheck(ctx context.Context) error
        HasNewCommit(ctx context.Context) (bool, error)
    }
    ```
    - Consumer-side interface per naming convention (interfaces in consumer package)
    - Story 3.3 extends to full interface + `ExecGitClient` implementation
    - `HasNewCommit` semantics: walking skeleton mock returns true/false directly. Story 3.3 will implement as "HEAD changed since last check" (compare before/after `session.Execute`)
  - [x] 2.2 Embed prompt templates at package scope using `go:embed`:
    ```go
    import _ "embed"

    //go:embed prompts/execute.md
    var executeTemplate string

    //go:embed prompts/review.md
    var reviewTemplate string
    ```
    Templates are baked into the binary — no filesystem reads needed at runtime. The fallback chain (project → global → embedded) is a `config.ResolvePath` concern (Story 1.5), not runner's concern for now.
  - [x] 2.3 Define `RunConfig` struct to pass dependencies:
    ```go
    type RunConfig struct {
        Cfg       *config.Config
        Git       GitClient
        TasksFile string // path to sprint-tasks.md
    }
    ```
  - [x] 2.4 Implement `RunOnce(ctx context.Context, rc RunConfig) error`:
    - Read `rc.TasksFile` via `os.ReadFile`
    - Scan for first open task: `strings.Split(content, "\n")` → `config.TaskOpenRegex.MatchString(line)` → extract full matched line as `taskLine` (e.g., `"- [ ] Implement hello world"`)
    - If no open task found: `return fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)` — uses existing sentinel from Story 1.2, enabling `errors.Is` checks
    - Call `rc.Git.HealthCheck(ctx)` — fail if error
    - Assemble prompt via `config.AssemblePrompt`:
      ```go
      prompt, err := config.AssemblePrompt(
          executeTemplate,        // go:embed'd from prompts/execute.md
          config.TemplateData{},  // minimal — GatesEnabled=false, SerenaEnabled=false
          map[string]string{
              "__TASK_CONTENT__": taskLine,
          },
      )
      ```
    - Measure elapsed time and call `session.Execute`:
      ```go
      start := time.Now()
      raw, execErr := session.Execute(ctx, opts)
      elapsed := time.Since(start)
      ```
      with Options: `Command: rc.Cfg.ClaudeCommand`, `Dir: rc.Cfg.ProjectRoot`, `Prompt: prompt`, `MaxTurns: rc.Cfg.MaxTurns`, `OutputJSON: true`, `DangerouslySkipPermissions: true`
    - **CRITICAL: `session.Execute` returns `(*RawResult, error)`. On non-zero exit code, BOTH are non-nil** (RawResult has stdout/stderr, error wraps exit code). For walking skeleton, treat any Execute error as fatal: `return fmt.Errorf("runner: execute: %w", execErr)`. Always capture `raw` even on error path (CLAUDE.md learning from Story 1.11). Story 3.5+ will distinguish `ExitCodeError` (findings) from other errors (crash)
    - On success: call `session.ParseResult(raw, elapsed)` to parse response
    - Call `rc.Git.HasNewCommit(ctx)` — check for commit
    - Return nil on success
  - [x] 2.5 Implement `RunReview(ctx context.Context, rc RunConfig) error`:
    - NOTE: No `sessionID` parameter — walking skeleton review is a standalone stub. Story 3.5+ will add session context when real review needs execute session data
    - Assemble review prompt via `config.AssemblePrompt(reviewTemplate, config.TemplateData{}, map[string]string{"__TASK_CONTENT__": "review stub"})`
    - Measure elapsed, call `session.Execute`, call `session.ParseResult`
    - Return nil on success (clean review = no findings)
  - [x] 2.6 Error wrapping: `fmt.Errorf("runner: <operation>: %w", err)` for all errors
  - [x] 2.7 Required imports for `runner/runner.go`:
    ```go
    import (
        _ "embed"
        "context"
        "fmt"
        "os"
        "strings"
        "time"

        "github.com/bmad-ralph/bmad-ralph/config"
        "github.com/bmad-ralph/bmad-ralph/session"
    )
    ```
    Runner MUST NOT import: `bridge`, `gates`, `cmd/ralph`. Only `config`, `session`, and stdlib.

- [x] Task 3: Create MockGitClient for walking skeleton test (AC: 4)
  - [x] 3.1 In `runner/runner_integration_test.go`, define `mockGitClient` (unexported, test-only):
    ```go
    type mockGitClient struct {
        healthCheckErr  error
        hasNewCommit    bool
        hasNewCommitErr error
    }

    func (m *mockGitClient) HealthCheck(ctx context.Context) error {
        return m.healthCheckErr
    }

    func (m *mockGitClient) HasNewCommit(ctx context.Context) (bool, error) {
        return m.hasNewCommit, m.hasNewCommitErr
    }
    ```
  - [x] 3.2 This is a TEST-LOCAL mock, not `internal/testutil/MockGitClient`. Real `MockGitClient` deferred to Story 3.3 when `GitClient` interface is fully defined

- [x] Task 4: Create integration test `runner/runner_integration_test.go` (AC: 1, 3, 5, 6, 8, 9)
  - [x] 4.1 Add build tag `//go:build integration` as FIRST line
  - [x] 4.2 `package runner` (internal test — access to unexported if needed)
  - [x] 4.3 Add `TestMain` with `testutil.RunMockClaude()` self-reexec guard:
    ```go
    func TestMain(m *testing.M) {
        if testutil.RunMockClaude() {
            return
        }
        os.Exit(m.Run())
    }
    ```
    NOTE: Uses `testutil.RunMockClaude()` (env: `MOCK_CLAUDE_SCENARIO`), NOT the `SESSION_TEST_HELPER` pattern from `session_test.go`. They are different self-reexec mechanisms: `RunMockClaude` handles multi-step scenarios with state tracking; `SESSION_TEST_HELPER` handles simple single-response helpers.
  - [x] 4.4 Implement `TestRunOnce_WalkingSkeleton_HappyPath`:
    - Create `t.TempDir()` as project root
    - Copy sprint-tasks-basic.md fixture to temp dir (read from `testdata/`, write to temp)
    - Set up mock Claude scenario with 2 steps:
      - Step 1: `{Type: "execute", ExitCode: 0, SessionID: "skel-exec-001", CreatesCommit: true}`
      - Step 2: `{Type: "review", ExitCode: 0, SessionID: "skel-review-001", CreatesCommit: false}`
      - NOTE: `IsError: false` (zero value) is correct for success. `OutputFile: ""` uses default mock output
    - Call `testutil.SetupMockClaude(t, scenario)` — returns `(scenarioPath, stateDir)`
    - Create `config.Config` directly (bypass `config.Load()` — fields are exported, Load tested in Stories 1.3-1.5):
      ```go
      cfg := &config.Config{
          ClaudeCommand: os.Args[0],  // test binary = mock Claude
          MaxTurns:      5,
          ProjectRoot:   tmpDir,
      }
      ```
      Only 3 fields needed. All others use Go zero values: `GatesEnabled=false` (no gate section in prompt), `Model=""` (no `--model` flag), `MaxIterations=0` (not used by `RunOnce`)
    - Create `mockGitClient{hasNewCommit: true}`
    - Create `RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}`
    - Call `RunOnce(ctx, rc)` — expect nil error
    - NOTE: `RunOnce` returns only `error`, not `SessionResult`. Validate end-to-end via: (a) no error, (b) mock Claude args verification
    - Verify EXECUTE args via `testutil.ReadInvocationArgs(t, stateDir, 0)`:
      ```go
      args := testutil.ReadInvocationArgs(t, stateDir, 0)
      // Iterate args slice to find flag+value pairs:
      // "-p" followed by prompt content
      // "--max-turns" followed by "5" (string, not int)
      // "--output-format" followed by "json"
      // "--dangerously-skip-permissions" (standalone flag)
      ```
    - Call `RunReview(ctx, rc)` — expect nil error
    - Verify REVIEW args via `testutil.ReadInvocationArgs(t, stateDir, 1)`:
      - Contains `-p` with review prompt content (different from execute prompt)
      - Contains `--max-turns`, `--output-format json`, `--dangerously-skip-permissions`
  - [x] 4.5 Implement `TestRunOnce_WalkingSkeleton_GitHealthCheckFails`:
    - Same setup but `mockGitClient{healthCheckErr: errors.New("git not found")}`
    - Call `RunOnce(ctx, rc)` — expect error
    - Verify: `strings.Contains(err.Error(), "runner:")` and `strings.Contains(err.Error(), "git not found")`
  - [x] 4.6 Implement `TestRunOnce_WalkingSkeleton_NoOpenTasks`:
    - Create sprint-tasks.md with only completed tasks: `- [x] Already done`
    - Call `RunOnce(ctx, rc)` — expect error
    - Verify: `errors.Is(err, config.ErrNoTasks)` is true
    - Verify: `strings.Contains(err.Error(), "runner:")` is true
  - [x] 4.7 Implement `TestRunOnce_WalkingSkeleton_SessionFails`:
    - Mock Claude scenario with `ExitCode: 1`
    - Call `RunOnce(ctx, rc)` — expect error wrapping session error
    - Verify: `strings.Contains(err.Error(), "runner: execute:")` is true
  - [x] 4.8 Implement `TestRunOnce_WalkingSkeleton_TasksFileNotFound`:
    - Set `TasksFile` to non-existent path
    - Call `RunOnce(ctx, rc)` — expect error
    - Verify: `strings.Contains(err.Error(), "runner:")` and error relates to file path
  - [x] 4.9 Test naming: `TestRunOnce_<Scenario>` is acceptable for package-level functions (matches `TestBuildArgs` pattern from `session_test.go`). If `RunOnce` becomes a method on a `Runner` type (Story 3.5+), rename to `TestRunner_RunOnce_<Scenario>`

- [x] Task 5: Create minimal prompt templates for walking skeleton (AC: 5)
  - [x] 5.1 Create `runner/prompts/execute.md`:
    ```
    You are a developer. Complete the following task.

    {{if .GatesEnabled}}
    GATES ARE ENABLED - pause at checkpoints.
    {{end}}

    Task:
    __TASK_CONTENT__
    ```
  - [x] 5.2 Create `runner/prompts/review.md`:
    ```
    Review the code changes for the following task.

    Task:
    __TASK_CONTENT__
    ```
    Review template is intentionally static for walking skeleton. Production template (Story 4.1) will have conditionals. Walking skeleton validates `AssemblePrompt` integration regardless.
  - [x] 5.3 Delete `runner/prompts/.gitkeep` after real files are created. Keep `runner/prompts/agents/.gitkeep` (no agent prompts yet)
  - [x] 5.4 These are MINIMAL templates. Epics 3-4 create production prompts. Point here is validating `config.AssemblePrompt` integration with runner

- [x] Task 6: Run tests and verify (AC: all)
  - [x] 6.1 Run integration test: `"/mnt/c/Program Files/Go/bin/go.exe" test ./runner/ -tags=integration -v -run TestRunOnce`
  - [x] 6.2 Run full unit tests for regressions: `"/mnt/c/Program Files/Go/bin/go.exe" test ./... -v`
  - [x] 6.3 Verify no new external dependencies: `"/mnt/c/Program Files/Go/bin/go.exe" mod tidy` should not change go.mod
  - [x] 6.4 Verify dependency direction: `runner/` imports only `session`, `config`, `embed`, and stdlib
  - [x] 6.5 Update story file with completion notes and file list

## Dev Notes

### Architecture Context

**Walking skeleton purpose:** Validate that the architecture works end-to-end before building features. This is NOT the full runner loop — just proving config → prompt assembly → session execute → parse result → mock git verification connects correctly. Story 3.5+ builds the real runner loop.

**Structural Rule #1 (from architecture):** "Walking skeleton в Epic 1 — минимальный e2e pass" — the CRITICAL architectural validation proving all packages compose together.

**What this story DOES NOT do:**
- No real `runner.Run(ctx, cfg)` loop — Story 3.5
- No sprint-tasks.md mutation (marking `[x]`) — part of the runner loop
- No real `GitClient` — Story 3.3 (`ExecGitClient` + full interface)
- No real prompt templates — Stories 3.1 (execute), 4.1 (review)
- No bridge — Epic 2
- No CLI wiring — Story 1.13

### Sprint-tasks.md Format (Hand-Crafted Fixture)

Markers from `config/constants.go`:
```
- [ ]  → open task (config.TaskOpen / config.TaskOpenRegex)
- [x]  → done task (config.TaskDone / config.TaskDoneRegex)
[GATE] → human gate marker (config.GateTag / config.GateTagRegex)
```

Scanning: `os.ReadFile` → `strings.Split(content, "\n")` → `config.TaskOpenRegex.MatchString(line)`. Extract full matched line as task content for prompt injection.

Story 2.1 formalizes full format contract. Walking skeleton fixture is intentionally simple.

### Existing Code Patterns to Follow

- **`session/session.go:53-82`** — `Execute()` returns `(*RawResult, error)`. On non-zero exit: both non-nil
- **`session/result.go:34-74`** — `ParseResult(raw, elapsed)` extracts `SessionResult`. Needs `time.Duration`
- **`config/prompt.go`** — `AssemblePrompt(tmplContent, data, replacements)` — frozen interface
- **`config/constants.go`** — `TaskOpenRegex`, `TaskDoneRegex`, `ErrNoTasks` sentinel
- **`internal/testutil/mock_claude.go`** — `RunMockClaude()`, `SetupMockClaude()`, `ReadInvocationArgs()`
- **`session/session_test.go:17-23`** — TestMain self-reexec pattern
- **`internal/testutil/scenarios/happy_path.json`** — scenario format reference

### Previous Story Intelligence

**From Story 1.11 (mock Claude):**
- Self-reexec via `os.Args[0]` as Command — proven working
- `SetupMockClaude()` uses `t.Setenv` → blocks `t.Parallel()` (Go 1.24+)
- Always capture `RawResult` (not `_`) when asserting on subprocess output
- Counter file tracks step number across invocations

**From Story 1.10 (prompt assembly):**
- `AssemblePrompt` signature FROZEN: `(tmplContent string, data TemplateData, replacements map[string]string) (string, error)`
- Stage 2 placeholder convention: `__TASK_CONTENT__`, `__LEARNINGS__`, etc.

**From CLAUDE.md review learnings:**
- Use `t.Errorf`/`t.Fatalf`, NEVER `t.Logf` in assertion blocks
- Error tests MUST verify message content with `strings.Contains`
- Test ALL error paths: missing files, permission denied, not just happy path
- Windows Go path: `"/mnt/c/Program Files/Go/bin/go.exe"` for all go commands

### Project Structure Notes

- **New files (added):**
  - `runner/runner.go` — expanded from placeholder (RunOnce, RunReview, GitClient, RunConfig, go:embed templates)
  - `runner/runner_integration_test.go` — integration test with build tag
  - `runner/testdata/sprint-tasks-basic.md` — hand-crafted fixture
  - `runner/prompts/execute.md` — minimal execute prompt template
  - `runner/prompts/review.md` — minimal review prompt template
- **Deleted files:**
  - `runner/testdata/.gitkeep` — replaced by real fixture
  - `runner/prompts/.gitkeep` — replaced by real files
- **Kept:**
  - `runner/prompts/agents/.gitkeep` — no agent prompts yet
- **No modified production files** outside `runner/` package

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.12] — AC, prerequisites
- [Source: session/session.go] — Execute() signature, Options struct
- [Source: session/result.go] — ParseResult(), SessionResult, RawResult
- [Source: config/prompt.go] — AssemblePrompt() frozen interface
- [Source: config/constants.go] — TaskOpen, TaskOpenRegex, ErrNoTasks
- [Source: internal/testutil/mock_claude.go] — RunMockClaude(), SetupMockClaude(), ReadInvocationArgs()
- [Source: docs/sprint-artifacts/1-11-test-infrastructure-mock-claude-mock-git.md] — self-reexec pattern

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No debug issues encountered. All tests passed on first run.

### Completion Notes List

- Implemented `runner/runner.go` with `GitClient` interface, `RunConfig` struct, `RunOnce()` and `RunReview()` functions
- `RunOnce` reads sprint-tasks.md, scans for first open task via `config.TaskOpenRegex`, assembles prompt via `config.AssemblePrompt`, executes via `session.Execute`, parses via `session.ParseResult`, checks commit via `GitClient.HasNewCommit`
- `RunReview` assembles review prompt and executes standalone review stub
- Both functions use go:embed'd minimal prompt templates (execute.md, review.md)
- Error wrapping follows `fmt.Errorf("runner: <op>: %w", err)` convention
- Created 7 integration tests: HappyPath (full e2e with execute+review), GitHealthCheckFails, NoOpenTasks (ErrNoTasks sentinel), SessionFails (exit code 1), TasksFileNotFound, HasNewCommitFails, RunReview_SessionFails
- All tests use `t.TempDir()` for isolation, mock Claude via `testutil.RunMockClaude()` self-reexec pattern
- HappyPath test verifies CLI args passed to mock Claude: `-p`, `--max-turns 5`, `--output-format json`, `--dangerously-skip-permissions`
- No new dependencies added. Runner imports only `config`, `session`, `embed`, and stdlib
- All 7 integration tests PASS, full regression suite (70+ tests) PASS, no regressions

### Change Log

- 2026-02-25: Implemented walking skeleton — runner scaffolding with RunOnce/RunReview, GitClient interface, prompt templates, 5 integration tests
- 2026-02-25: Code review fixes — added 2 missing error path tests (HasNewCommitFails, RunReview_SessionFails), removed duplicate test helper, fixed execute.md template trim markers

### File List

- `runner/runner.go` — modified (expanded from placeholder to full implementation)
- `runner/runner_integration_test.go` — new/added
- `runner/testdata/sprint-tasks-basic.md` — new/added
- `runner/prompts/execute.md` — new/added
- `runner/prompts/review.md` — new/added
- `runner/testdata/.gitkeep` — deleted
- `runner/prompts/.gitkeep` — deleted
- `docs/sprint-artifacts/sprint-status.yaml` — modified (story status updated)
- `docs/sprint-artifacts/1-12-walking-skeleton-minimal-end-to-end-pass.md` — modified (task checkboxes, dev agent record)
