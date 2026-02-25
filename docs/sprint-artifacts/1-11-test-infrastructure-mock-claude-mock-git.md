# Story 1.11: Test Infrastructure — Mock Claude

Status: done

## Story

As a developer,
I want scenario-based mock Claude infrastructure,
so that all subsequent epics can write integration tests without real Claude CLI calls.

## Acceptance Criteria

1. Mock Claude helper exists in `internal/testutil/mock_claude.go` with exported `RunMockClaude() bool` function
2. Mock Claude reads scenario JSON file via `MOCK_CLAUDE_SCENARIO` env var
3. Mock Claude tracks invocation step via counter file in `MOCK_CLAUDE_STATE_DIR`
4. Returns responses in order per scenario steps with JSON matching Claude CLI format:
   - `type`: "execute" or "review" (step metadata, not part of JSON output)
   - `exit_code`: process exit code
   - `session_id`: mock session ID for JSON response
   - `output_file`: file with custom output text (optional, relative to scenario dir)
   - `creates_commit`: bool signal for future MockGitClient (stored, not acted on)
5. Mock Claude is substituted via `config.ClaudeCommand` path using self-reexec pattern (`os.Args[0]`)
6. Mock Claude logs received CLI args to `{state_dir}/invocation_{N}.json` for test verification
7. At least one example scenario JSON file exists: `internal/testutil/scenarios/happy_path.json` (1 execute success + 1 review clean)
8. Standalone binary source exists at `internal/testutil/cmd/mock_claude/main.go` (calls shared `RunMockClaude()`)
9. Table-driven tests verify:
   - Mock Claude returns correct sequence of responses (parsed via `session.ParseResult`)
   - Mock Claude fails on unexpected call (beyond scenario steps)
   - Zero-value `ScenarioStep{}` and `Scenario{}` behavior documented by tests
10. **MockGitClient deferred to Epic 3** (Story 3.3 defines `GitClient` interface — speculative mock is dangerous)

## Tasks / Subtasks

- [x] Task 1: Define scenario types and core logic in `internal/testutil/mock_claude.go` (AC: 1, 2, 3, 4, 5, 6)
  - [x] 1.1 Define `ScenarioStep` struct with JSON tags: `Type string`, `ExitCode int`, `SessionID string`, `OutputFile string`, `CreatesCommit bool`
  - [x] 1.2 Define `Scenario` struct with `Name string` and `Steps []ScenarioStep`
  - [x] 1.3 Implement `RunMockClaude() bool` — self-reexec handler:
    - Read `MOCK_CLAUDE_SCENARIO` env var; if empty, return false
    - Read `MOCK_CLAUDE_STATE_DIR` env var; if empty when scenario IS set → stderr error, `os.Exit(1)`
    - Read scenario JSON from file path; if read/parse fails → stderr error, `os.Exit(1)`
    - Read step counter from `{state_dir}/counter` (0 if file missing; if file contains non-integer → stderr error, `os.Exit(1)`)
    - If step >= len(steps): write error to stderr "mock_claude: step N: beyond scenario (has M steps)", `os.Exit(1)`
    - Get current step from scenario
    - Log received args: write `os.Args[1:]` to `{state_dir}/invocation_{N}.json`
    - Resolve output text: read `output_file` if set (relative to scenario dir), else default `"Mock output for step N"`
    - Write Claude CLI JSON array to stdout (matching `session/testdata/result_success.json` format)
    - Increment counter file
    - `os.Exit(step.ExitCode)` — AFTER writing stdout (Go `os.Stdout` is unbuffered `*os.File`, writes flush immediately)
  - [x] 1.4 Implement `SetupMockClaude(t *testing.T, scenario Scenario) (scenarioPath, stateDir string)` — test helper:
    - Create temp dir via `t.TempDir()`
    - Write scenario JSON to `{dir}/scenario.json`
    - Create state subdirectory `{dir}/state/`
    - Set env vars via `t.Setenv("MOCK_CLAUDE_SCENARIO", scenarioPath)` and `t.Setenv("MOCK_CLAUDE_STATE_DIR", stateDir)`
    - Return paths for assertions
  - [x] 1.5 Implement `ReadInvocationArgs(t *testing.T, stateDir string, step int) []string` — reads args log for test assertions
  - [x] 1.6 Only stdlib imports: `encoding/json`, `fmt`, `os`, `path/filepath`, `strconv`
  - [x] 1.7 Error wrapping: `fmt.Fprintf(os.Stderr, "mock_claude: ...")` for error messages

- [x] Task 2: Create standalone binary source `internal/testutil/cmd/mock_claude/main.go` (AC: 8)
  - [x] 2.1 `package main` with single `main()` function
  - [x] 2.2 Call `testutil.RunMockClaude()` — if returns false, print error and `os.Exit(1)`
  - [x] 2.3 Import `github.com/bmad-ralph/bmad-ralph/internal/testutil`

- [x] Task 3: Create example scenario files (AC: 7)
  - [x] 3.1 Create `internal/testutil/scenarios/happy_path.json`:
    - Step 1: `type: "execute"`, `exit_code: 0`, `session_id: "mock-exec-001"`, `creates_commit: true`
    - Step 2: `type: "review"`, `exit_code: 0`, `session_id: "mock-review-001"`, `output_file: "review_clean.txt"`, `creates_commit: false`
  - [x] 3.2 Create `internal/testutil/scenarios/review_clean.txt` — output content for review step ("Review complete. No findings.")
  - [x] 3.3 Validate JSON manually (proper formatting, correct field types)

- [x] Task 4: Create tests `internal/testutil/mock_claude_test.go` (AC: 9)
  - [x] 4.1 `TestMain` with `RunMockClaude()` self-reexec guard (BEFORE `m.Run()`)
  - [x] 4.2 Table-driven `TestRunMockClaude_SequentialResponses`: verify correct sequence
    - Set up scenario with 2 steps (happy_path pattern)
    - Call `session.Execute` twice with `Command: os.Args[0]`
    - Parse each response with `session.ParseResult`
    - Verify SessionID, Output, ExitCode match expected step values
  - [x] 4.3 Test `TestRunMockClaude_BeyondScenarioSteps`: verify failure on extra call
    - Set up scenario with 1 step
    - Call `session.Execute` once (success)
    - Call `session.Execute` again — expect non-zero exit code
    - Verify error message contains "beyond scenario"
  - [x] 4.4 Test `TestRunMockClaude_CustomOutputFile`: verify output_file content used
    - Create temp file with custom output text
    - Set up scenario step with `output_file` pointing to it
    - Verify response output matches file content
  - [x] 4.5 Test `TestRunMockClaude_ArgsLogging`: verify invocation args logged
    - Execute with specific flags (prompt, max-turns, etc.)
    - Read `invocation_0.json` from state dir via `ReadInvocationArgs`
    - Verify logged args contain expected flags
  - [x] 4.6 Test `TestRunMockClaude_NonZeroExitCode`: verify exit code propagation
    - Set up scenario step with `exit_code: 2`
    - Verify Execute returns error with exit code 2
    - Verify error message contains "session: claude: exit 2:"
  - [x] 4.7 Test `TestRunMockClaude_MissingScenarioFile`: verify graceful error
    - Set `MOCK_CLAUDE_SCENARIO` to nonexistent path, set valid `MOCK_CLAUDE_STATE_DIR`
    - Verify non-zero exit and descriptive error
  - [x] 4.8 Test `TestRunMockClaude_MissingStateDir`: verify graceful error when state dir env var not set
    - Set `MOCK_CLAUDE_SCENARIO` to valid path, do NOT set `MOCK_CLAUDE_STATE_DIR`
    - Verify non-zero exit and stderr contains "MOCK_CLAUDE_STATE_DIR"
  - [x] 4.9 Test `TestRunMockClaude_EmptyScenario`: verify failure when scenario has zero steps
    - Set up `Scenario{Name: "empty", Steps: nil}`
    - First call triggers "beyond scenario (has 0 steps)" error
  - [x] 4.10 Test `TestScenarioStep_ZeroValue`: verify zero-value struct behavior
    - `ScenarioStep{}` should produce valid JSON response with ExitCode=0, empty SessionID, default output
    - `Scenario{}` should have empty Name and nil Steps

- [x] Task 5: Verify and finalize (AC: all)
  - [x] 5.1 Run tests: `"/mnt/c/Program Files/Go/bin/go.exe" test ./internal/testutil/ -v`
  - [x] 5.2 Verify no new external dependencies (`go mod tidy` should not change go.mod)
  - [x] 5.3 Verify only stdlib imports in mock_claude.go (except testutil->session import in test)
  - [x] 5.4 CRLF fix: `sed -i 's/\r$//'` on all new files
  - [x] 5.5 Update story file with completion notes and file list

## Dev Notes

### Architecture Context

**Mock Claude purpose:** Integration tests for runner (Stories 1.12, 3.5-3.11) and review pipeline (Epic 4) need Claude CLI responses without real API calls. Mock Claude provides scenario-based, deterministic responses matching the real Claude CLI `--output-format json` format.

**Self-reexec pattern** (proven in Stories 1.7-1.9): The test binary itself acts as mock Claude when `MOCK_CLAUDE_SCENARIO` env var is set. This avoids the Windows Go cross-compilation issue (`go.exe` at `/mnt/c/Program Files/Go/bin/go.exe` — not in WSL PATH) that would complicate building a separate binary in tests. Each test package that needs mock Claude adds `testutil.RunMockClaude()` to its `TestMain`.

**Why NOT a separate binary (for now):** Session tests already use this exact pattern with `SESSION_TEST_HELPER`. The standalone binary source EXISTS at `internal/testutil/cmd/mock_claude/main.go` for future use (CI, external testing), but tests use `os.Args[0]` for cross-platform reliability.

**MockGitClient deferred:** `GitClient` interface is defined in Story 3.3. Creating a speculative mock before the interface exists risks API mismatch. Story 3.3 will create `MockGitClient` alongside `ExecGitClient`.

### Claude CLI JSON Output Format (CRITICAL)

Mock Claude MUST output JSON matching the real Claude CLI `--output-format json` format. Reference: `session/testdata/result_success.json`:

```json
[
  {"type":"system","subtype":"init","session_id":"<session_id>","tools":[],"model":"mock-claude"},
  {"type":"result","subtype":"success","session_id":"<session_id>","result":"<output_text>","is_error":false,"duration_ms":100,"num_turns":1}
]
```

**CRITICAL JSON fidelity rules:**
- `session.ParseResult` looks for the LAST element with `type == "result"` and extracts `session_id` and `result` fields
- Mock must produce at least a system init message AND a result message
- Do NOT use `omitempty` on `is_error`, `duration_ms`, `num_turns` — real Claude CLI outputs `"is_error":false` explicitly. `omitempty` on bool omits `false`, breaking fidelity. Use plain tags: `json:"is_error"`

### `RunMockClaude()` Return Behavior

`RunMockClaude() bool` returns `false` when `MOCK_CLAUDE_SCENARIO` env var is empty (normal test execution). When env var IS set, the function executes mock logic and calls `os.Exit()` — it **never returns `true`**. The `return true` path is unreachable. Callers use the idiomatic guard pattern:

```go
if testutil.RunMockClaude() {
    return // dead code, but documents intent for readers
}
```

Doc comment MUST explain this: "Returns false if not acting as mock. When acting as mock, calls os.Exit and never returns."

### State Tracking Design

Mock Claude is invoked as a NEW process for each `session.Execute` call. Step tracking across invocations uses a counter file:

```
{state_dir}/
├── counter           # Plain text: "0", "1", "2"... incremented each invocation
├── invocation_0.json # CLI args received: ["--resume","abc","-p","prompt text",...]
├── invocation_1.json
└── ...
```

**Counter robustness:** Tests are sequential within a scenario (`t.Setenv` blocks `t.Parallel()` — Go panics since 1.24). Simple `os.ReadFile` / `os.WriteFile` is sufficient. If counter file is missing → start at 0. If counter file contains non-integer text → stderr error, exit 1 (corrupted state, fail fast).

### Self-Reexec Integration Pattern

For packages needing mock Claude in their tests (e.g., `runner/`):

```go
// runner/runner_test.go
func TestMain(m *testing.M) {
    if testutil.RunMockClaude() {
        return // acted as mock Claude subprocess
    }
    os.Exit(m.Run())
}

func TestRunner_Integration(t *testing.T) {
    scenario := testutil.Scenario{
        Name: "happy_path",
        Steps: []testutil.ScenarioStep{
            {Type: "execute", ExitCode: 0, SessionID: "test-001", CreatesCommit: true},
        },
    }
    _, _ = testutil.SetupMockClaude(t, scenario)

    result, err := session.Execute(ctx, session.Options{
        Command:    os.Args[0],  // test binary = mock Claude
        Dir:        t.TempDir(),
        Prompt:     "test prompt",
        OutputJSON: true,
    })
    // ... assertions ...
}
```

**Env var propagation:** `t.Setenv` sets env vars in the current process. `session.Execute` calls `cmd.Env = os.Environ()` which captures these vars. The subprocess (test binary re-invoked) sees them and triggers `RunMockClaude()`.

### `output_file` Resolution

When `ScenarioStep.OutputFile` is non-empty:
- Path is relative to the scenario JSON file's directory (`filepath.Dir(scenarioPath)`)
- Mock Claude reads the file and uses content as the `result` field in JSON output
- If file doesn't exist → write error to stderr, exit 1

When `OutputFile` is empty:
- Use default message: `"Mock output for step N"` (N = step index)

### `creates_commit` Field

Present in `ScenarioStep` for forward-compatibility with MockGitClient (Epic 3). Mock Claude does NOT act on this field — it's metadata for the test harness. Tests that need git commit information can read the scenario JSON directly.

### Implementation Guidance

**JSON generation** — use `json.Marshal`, not string concatenation. Do NOT use `omitempty` on fields that real Claude CLI always includes:

```go
type mockMessage struct {
    Type      string `json:"type"`
    Subtype   string `json:"subtype,omitempty"`
    SessionID string `json:"session_id"`
    Tools     []any  `json:"tools,omitempty"`
    Model     string `json:"model,omitempty"`
    Result    string `json:"result,omitempty"`
    IsError   bool   `json:"is_error"`
    Duration  int    `json:"duration_ms"`
    NumTurns  int    `json:"num_turns"`
}
```

**Error output** — mock_claude writes errors to stderr, not stdout:
```go
fmt.Fprintf(os.Stderr, "mock_claude: step %d: %s\n", stepNum, errMsg)
os.Exit(1)
```

### Existing Code Patterns to Follow

- **`session/session_test.go:17-23`** — TestMain self-reexec pattern (exact model)
- **`session/session_test.go:26-51`** — `runTestHelper` switch/case with `default: os.Exit(1)`
- **`session/result.go:34-74`** — `ParseResult` function (used by tests to verify mock output)
- **`session/testdata/result_success.json`** — Claude CLI JSON format reference
- **`session/testdata/result_is_error.json`** — `is_error: true` reference (verify mock can produce this too)
- **`config/config.go:19`** — `ClaudeCommand` field for mock substitution
- **`session/result_test.go:239-349`** — `Execute` -> `ParseResult` round-trip integration test pattern
- **`session/result_test.go:351-367`** — zero-value struct test pattern (apply to ScenarioStep/Scenario)

### Project Structure Notes

- **New files (added):**
  - `internal/testutil/mock_claude.go` — core mock logic + helper functions
  - `internal/testutil/mock_claude_test.go` — tests
  - `internal/testutil/cmd/mock_claude/main.go` — standalone binary source
  - `internal/testutil/scenarios/happy_path.json` — example scenario
  - `internal/testutil/scenarios/review_clean.txt` — review output fixture
- **Modified files:**
  - `internal/testutil/testutil.go` — may remain as-is (placeholder package declaration)
- **No new external dependencies** — all stdlib

### Previous Story Intelligence (Stories 1.7-1.9)

- **Windows Go `os/exec` testing:** Self-reexec via `TestMain` + env var + `os.Args[0]` is the standard pattern. Proven working. [Source: CLAUDE.md, Story 1.7 review]
- **Test helper `default` case required:** Always add `default:` with `os.Exit(1)` — silent success on unknown scenario masks test bugs. [Source: CLAUDE.md, Story 1.7 review]
- **Integration test round-trip pattern:** `Execute` -> `ParseResult` -> verify fields. Clean pattern from `session/result_test.go:239-349`. [Source: Story 1.8]
- **Test name consistency:** All case names within one function must use same style (spaces, not hyphens). [Source: Story 1.9 review]
- **Zero-value tests required:** Always test zero-value behavior of custom types (`ScenarioStep{}`, `Scenario{}`). [Source: Story 1.2 review]
- **Error tests verify message content:** Bare `err != nil` insufficient — always `strings.Contains`. [Source: Story 1.5 review]

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.11] — AC, user story, prerequisites, MockGitClient deferral note
- [Source: docs/architecture/project-structure-boundaries.md#Complete Project Directory Structure] — file locations (`internal/testutil/`, scenarios)
- [Source: docs/architecture/project-structure-boundaries.md#Test Scenario Format] — scenario JSON format, mock Claude principle
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Testing Patterns] — scenario-based mock, assertions, golden files
- [Source: docs/architecture/core-architectural-decisions.md#Testing Implications] — MockGitClient in testutil, scenario-based testing
- [Source: docs/project-context.md#Testing] — mock Claude via config.ClaudeCommand, scenario JSON
- [Source: session/session_test.go] — self-reexec TestMain pattern, proven working
- [Source: session/testdata/result_success.json] — Claude CLI JSON output format

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

None — clean implementation, all tests passed on first run.

### Completion Notes List

- Implemented `ScenarioStep` and `Scenario` structs with proper JSON tags (no `omitempty` on `is_error`, `duration_ms`, `num_turns` per Dev Notes)
- Implemented `RunMockClaude() bool` self-reexec handler with full error handling: missing env vars, scenario file read/parse, counter corruption, beyond-scenario bounds, output_file resolution
- Implemented `SetupMockClaude()` test helper creating temp scenario environment with env vars via `t.Setenv`
- Implemented `ReadInvocationArgs()` for test assertions on logged CLI args
- Created standalone binary source at `internal/testutil/cmd/mock_claude/main.go`
- Created `happy_path.json` scenario (1 execute + 1 review) and `review_clean.txt` fixture
- Created 10 tests covering: sequential responses, beyond-scenario error, custom output file, args logging, non-zero exit code, missing scenario file, missing state dir, empty scenario, zero-value struct behavior, zero-value mock response
- Test package uses `_test` external test convention (`package testutil_test`) to verify exported API
- All 10 tests pass, full regression suite clean (config, session, testutil packages)
- No new external dependencies — mock_claude.go uses only stdlib (`encoding/json`, `errors`, `fmt`, `os`, `path/filepath`, `strconv`, `testing`)
- Removed `.gitkeep` placeholders from `scenarios/` and `cmd/mock_claude/` (directories now contain real files)
- CRLF fixed on all new files
- **[Code Review]** Fixed 8 issues (1 High, 4 Medium, 3 Low):
  - Fixed broken BeyondScenarioSteps test assertion (was using raw from wrong call + t.Logf instead of t.Errorf)
  - Added counter file NotExist vs other error distinction (permission-denied no longer silently ignored)
  - Added error message content verification to MissingScenarioFile test
  - Added `IsError` field to ScenarioStep for real CLI fidelity (subtype:"error" + is_error:true)
  - Added CorruptedCounter test for non-integer counter file content
  - Split TestScenarioStep_ZeroValue into separate TestScenarioStep_ZeroValue + TestScenario_ZeroValue
  - Replaced `fmt.Fprint(os.Stdout, string(output))` with `os.Stdout.Write(output)`
  - 13 tests now (was 10), all pass

### File List

- `internal/testutil/mock_claude.go` — new (core mock logic: types, RunMockClaude, SetupMockClaude, ReadInvocationArgs)
- `internal/testutil/mock_claude_test.go` — new (13 tests covering all ACs)
- `internal/testutil/cmd/mock_claude/main.go` — new (standalone binary source)
- `internal/testutil/scenarios/happy_path.json` — new (example 2-step scenario)
- `internal/testutil/scenarios/review_clean.txt` — new (review output fixture)
- `internal/testutil/scenarios/.gitkeep` — deleted (replaced by real files)
- `internal/testutil/cmd/mock_claude/.gitkeep` — deleted (replaced by real files)
- `docs/sprint-artifacts/sprint-status.yaml` — modified (1-11 status: ready-for-dev → in-progress → review)
- `docs/sprint-artifacts/1-11-test-infrastructure-mock-claude-mock-git.md` — new (story file with tasks marked, Dev Agent Record filled)

### Change Log

- 2026-02-25: Implemented Story 1.11 — Mock Claude test infrastructure with scenario-based responses, self-reexec pattern, args logging, and 10 comprehensive tests
- 2026-02-25: Code review — fixed 8 issues (1 High, 4 Medium, 3 Low), added 3 tests (total 13), improved error handling and JSON fidelity
