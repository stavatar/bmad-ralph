# Story 2.7: Bridge Integration Test

Status: Done

## Story

As a developer,
I want a full bridge integration test with mock Claude,
so that the complete bridge flow is validated end-to-end.

## Acceptance Criteria

1. **Full create flow integration test:**
   Given a mock Claude scenario for bridge flow and test story files in `t.TempDir()`, when bridge integration test runs, then the full flow executes: (1) Story files read from disk, (2) Bridge prompt assembled with format contract, (3) Mock Claude invoked with correct flags, (4) Output parsed and written to sprint-tasks.md, (5) Sprint-tasks.md contains expected tasks.

2. **Prompt content validation:**
   Mock Claude validates received prompt contains: sprint-tasks format contract text, story content, FR instructions (test cases, gates, service tasks, source traceability).

3. **Smart merge integration flow:**
   A second test covers the smart merge flow: (1) First bridge creates sprint-tasks.md, (2) Test modifies file marking some tasks `[x]`, (3) Second bridge with same story, (4) Merged output preserves `[x]` tasks, (5) Backup file exists and matches pre-merge content.

4. **Prompt-to-parser contract validation:**
   Scenario-based test validates prompt↔parser contract: mock Claude returns predefined bridge output → bridge parses correctly → output is consumable by config regex patterns (TaskOpenRegex, SourceFieldRegex, GateTagRegex).

5. **CLI-level test via compiled binary:**
   `exec.Command("ralph", "bridge", storyFile)` with mock Claude. Validates full path: Cobra arg parsing → config.Load → bridge.Run() → output. Tests exit code mapping: success → 0, failure → 4 (exitFatal).

6. **Test file location and isolation:**
   Test is in `bridge/bridge_integration_test.go` with `//go:build integration` tag. Uses `t.TempDir()` for isolation. Shares TestMain with existing `prompt_test.go` (compiled together under integration tag).

## Tasks / Subtasks

- [x] Task 1: Create integration test file with build tag and helpers (AC: #6)
  - [x] 1.1 Create `bridge/bridge_integration_test.go` with `//go:build integration` build constraint. Package `bridge` (same as bridge_test.go — internal test package). Import: `context`, `errors`, `fmt`, `os`, `os/exec`, `path/filepath`, `runtime`, `strings`, `sync`, `testing`, `config`, `testutil`
  - [x] 1.2 Add `extractPromptFromArgs(args []string) string` helper — iterates args to find `-p` flag and return the next arg (the prompt text). Returns empty string if not found
  - [x] 1.3 Add `verifyMockFlags(t, args []string)` helper — checks args contain `--dangerously-skip-permissions`, `--output-format json`, `-p`. Uses the same pattern as TestRun_Success in bridge_test.go
  - [x] 1.4 Add Go tool and binary build helpers for CLI tests:
    - `findGoTool() (string, error)` — checks GOEXE env, then exec.LookPath("go"), then runtime.GOROOT()+"/bin/go" and "/bin/go.exe" fallbacks
    - Package-level `var (buildRalphOnce sync.Once; ralphBinPath string; ralphBuildErr error)`
    - `ensureRalphBin(t) string` — uses sync.Once to build ralph binary once per test run. Uses `os.MkdirTemp` (not t.TempDir — shared across tests, temp dir NOT auto-cleaned — OS responsibility, acceptable for integration tests). Finds module root via `filepath.Dir(os.Getwd())` (one level up from bridge/). Runs `exec.Command(goTool, "build", "-o", bin, "./cmd/ralph")` from module root. Returns bin path or `t.Skipf` on failure
  - [x] 1.5 Add `setupCLIProject(t, mockCfg string) string` helper — creates TempDir with `.ralph/config.yaml` containing mockCfg, returns projectDir. Used by CLI tests to set up a valid project root for config.Load detection

- [x] Task 2: Full create flow integration test (AC: #1, #2)
  - [x] 2.1 `TestBridge_Integration_CreateFlow` — table-driven is unnecessary (single complex scenario), use standalone test
  - [x] 2.2 Setup: projectDir = TempDir, copy `story_single_3ac.md` to storyDir, setup 1-step mock scenario with `mock_bridge_output.md` fixture
  - [x] 2.3 Execute: `Run(context.Background(), cfg, []string{storyPath})`
  - [x] 2.4 Verify flow results: taskCount == 3, promptLines > 0, output file exists, output matches fixture byte-for-byte (deterministic mock)
  - [x] 2.5 Verify prompt content via `testutil.ReadInvocationArgs(t, stateDir, 0)`:
    - Format contract: contains "Sprint Tasks Format Specification" (from config.SprintTasksFormat() title)
    - Story content: contains "User Login Authentication" (from story_single_3ac.md title)
    - FR instructions: contains "[GATE]", "[SETUP]", "[VERIFY]", "[E2E]", "source:", "red-green"
    - Conversion instructions: contains "For each AC, create"
    - Negative examples: contains "DO NOT"
  - [x] 2.6 Verify mock invocation flags: `--dangerously-skip-permissions`, `--output-format json`, `-p`, `--max-turns`

- [x] Task 3: Smart merge integration flow (AC: #3)
  - [x] 3.1 `TestBridge_Integration_MergeFlow` — multi-step test
  - [x] 3.2 Setup: projectDir = TempDir, copy `story_single_3ac.md` to storyDir, setup 2-step mock scenario: step 0 → `mock_single_story.md` (create), step 1 → `mock_merge_completed.md` (merge)
  - [x] 3.3 Step 1 — Create: `Run()` → verify sprint-tasks.md created with open tasks
  - [x] 3.4 Between steps: read sprint-tasks.md, replace `"- [ ] Implement login endpoint"` → `"- [x] Implement login endpoint"` and `"- [ ] Add request validation"` → `"- [x] Add request validation"`, write back. Save modified content for later backup comparison
  - [x] 3.5 Step 2 — Merge: `Run()` again → auto-detects existing file → merge mode
  - [x] 3.6 Verify merge results:
    - Output contains `"- [x]"` (completed tasks preserved)
    - Output contains `"- [ ]"` (new/open tasks present)
    - `.bak` file exists at `sprint-tasks.md.bak`
    - `.bak` content matches pre-merge modified content byte-for-byte
  - [x] 3.7 Verify merge prompt: read step 1 args, check prompt contains "Merge Mode" and `"[x]"` (existing tasks were injected)

- [x] Task 4: Prompt-to-parser contract validation (AC: #4)
  - [x] 4.1 `TestBridge_Integration_PromptParserContract` — uses richest fixture `mock_source_traceability.md` for maximum coverage
  - [x] 4.2 Setup: projectDir = TempDir, copy `story_5ac_traceability.md`, setup mock with `mock_source_traceability.md`
  - [x] 4.3 Execute Run(), read output sprint-tasks.md
  - [x] 4.4 Cross-validate: `validateTaskSourcePairs(t, output)` confirms every task has source field
  - [x] 4.5 Regex validation: scan output lines with `config.TaskOpenRegex`, `config.SourceFieldRegex`, `config.GateTagRegex` — all must find at least one match
  - [x] 4.6 Verify task count from Run matches validated count from regex scan

- [x] Task 5: CLI-level success test (AC: #5)
  - [x] 5.1 `TestBridge_CLI_Success` — builds ralph binary, runs via exec.Command
  - [x] 5.2 Setup: `ensureRalphBin(t)` to get compiled binary path. Use `os.Executable()` to get test binary path (will be mock Claude)
  - [x] 5.3 Create project dir: `setupCLIProject(t, fmt.Sprintf("claude_command: %q\nmax_turns: 5\n", testBin))` — creates `.ralph/config.yaml` with test binary as claude_command
  - [x] 5.4 Create story file in project dir: write simple story content
  - [x] 5.5 Setup mock: `testutil.SetupMockClaude(t, scenario)` with 1-step success scenario using `mock_bridge_output.md`. Copy fixture via `copyFixtureToScenario`
  - [x] 5.6 Run: `cmd := exec.Command(ralphBin, "bridge", storyPath)`, `cmd.Dir = projectDir`. Get `cmd.CombinedOutput()`
  - [x] 5.7 Verify success: no error, exit code 0, stdout contains "Generated" and "tasks" substring
  - [x] 5.8 Verify sprint-tasks.md created in project dir

- [x] Task 6: CLI-level failure test (AC: #5)
  - [x] 6.1 `TestBridge_CLI_Failure` — same setup as success but mock returns exit 1
  - [x] 6.2 Setup: same as 5.2-5.4 but scenario step has `ExitCode: 1`
  - [x] 6.3 Run ralph binary, expect non-zero exit
  - [x] 6.4 Verify: `errors.As(err, &exitErr)` succeeds, `exitErr.ExitCode() == 4` (exitFatal mapping for bridge errors)
  - [x] 6.5 Verify: stdout contains "Error:" (from main.go color.Red output)
  - [x] 6.6 Verify: no sprint-tasks.md created

- [x] Task 7: Run all tests and verify (AC: all)
  - [x] 7.1 Run `go test ./bridge/...` — all existing tests still pass (no integration tag = existing behavior)
  - [x] 7.2 Run `go test -tags integration ./bridge/...` — integration tests pass alongside existing tests
  - [x] 7.3 Run `go vet ./...` — no issues
  - [x] 7.4 Verify no production code changes (this is test-only story)

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Build tag:** `//go:build integration` (Go 1.17+ syntax, no legacy `// +build` line needed) at top of file. This file is ONLY compiled when `-tags integration` is passed. All existing tests continue to run without this tag.

**Package declaration:** `package bridge` (internal test package, same as bridge_test.go). Shares TestMain from prompt_test.go — no new TestMain needed. All helpers from bridge_test.go are accessible: `copyFixtureToScenario`, `validateTaskSourcePairs`, `validateMergeTaskSourcePairs`, `goldenTest`, `-update` flag.

**Config setup pattern:**
```go
projectDir := t.TempDir()
storyDir := t.TempDir()
cfg := &config.Config{
    ClaudeCommand: os.Args[0],  // self-reexec test binary = mock Claude
    ProjectRoot:   projectDir,
    MaxTurns:      5,
}
```

**Mock scenario setup pattern:**
```go
scenario := testutil.Scenario{
    Steps: []testutil.ScenarioStep{{
        Type: "execute", ExitCode: 0, SessionID: "integ-xxx",
        OutputFile: "mock_fixture.md",
    }},
}
_, stateDir := testutil.SetupMockClaude(t, scenario)
copyFixtureToScenario(t, "mock_fixture.md")
```

**Multi-step scenario for merge:**
```go
scenario := testutil.Scenario{
    Steps: []testutil.ScenarioStep{
        {Type: "execute", ExitCode: 0, SessionID: "integ-create", OutputFile: "mock_single_story.md"},
        {Type: "execute", ExitCode: 0, SessionID: "integ-merge", OutputFile: "mock_merge_completed.md"},
    },
}
```
Mock Claude auto-increments step counter per invocation. First Run() gets step 0, second gets step 1.

### Architecture: Integration Tests vs Unit Tests

**Unit tests** (bridge_test.go, existing): test individual functions, mock at function level, fast, always run.

**Integration tests** (bridge_integration_test.go, new): test complete flows end-to-end, validate multi-step sequences, test CLI binary, slower, opt-in via build tag.

The integration tests do NOT duplicate unit test assertions — they verify the FLOW works correctly when all pieces connect.

### CLI Test Architecture

The CLI test requires a compiled `ralph` binary. Architecture:

1. **Build ralph binary** once per test run via `sync.Once`
2. **Find Go tool** via GOEXE env → exec.LookPath → runtime.GOROOT fallback → `t.Skipf` if unavailable
3. **Module root** = `filepath.Dir(os.Getwd())` — tests run from `bridge/`, one level up is project root
4. **Mock Claude in ralph subprocess**: The test binary (`os.Executable()`, NOT `os.Args[0]` — absolute path required because it's written to config.yaml and used by ralph in a different working directory) serves as mock Claude:
   - Test creates `.ralph/config.yaml` with `claude_command: "<test-binary-path>"`
   - ralph loads config, gets test binary as ClaudeCommand
   - ralph invokes test binary via session.Execute (which uses `os.Environ()`)
   - Test binary's TestMain dispatches to RunMockClaude (MOCK_CLAUDE_SCENARIO env var)
   - Mock returns canned JSON → ralph processes it
5. **Environment propagation chain**: test → `t.Setenv(MOCK_CLAUDE_SCENARIO, ...)` → `os.Environ()` inherited by ralph subprocess → `session.Execute` uses `cmd.Env = os.Environ()` → mock claude subprocess inherits vars
6. **Project root detection**: `cmd.Dir = projectDir` → ralph's `os.Getwd()` returns projectDir → `detectProjectRoot` finds `.ralph/` there

```
Test process          ralph binary            mock claude (test binary)
    |                     |                        |
    |--- exec.Command --->|                        |
    | (env: MOCK_*)       |                        |
    |                     |--- session.Execute ---->|
    |                     | (env: MOCK_* inherited) |
    |                     |                        |--- RunMockClaude()
    |                     |<--- JSON stdout -------|
    |                     |                        |
    |<--- exit code ------|                        |
```

### Exit Code Mapping (for CLI tests)

From `cmd/ralph/exit.go`:
| Error Type | Exit Code | Constant |
|-----------|-----------|----------|
| nil | 0 | exitSuccess |
| ExitCodeError | .Code | — |
| GateDecision(quit) | 2 | exitUserQuit |
| context.Canceled | 3 | exitInterrupted |
| everything else | 4 | exitFatal |

Bridge errors (`bridge: execute:` wrapping `session: claude: exit 1`) are "everything else" → **exit code 4**.

**CLI output note:** `color.Red("Error: %v", err)` and `color.Yellow("Warning: ...")` in main.go write to `os.Stdout` (fatih/color default). Use `cmd.CombinedOutput()` to capture both stdout and stderr; error messages will be in the combined output.

### Test Fixtures — All Reused, No New Files

| Test | Story Fixture | Mock Output Fixture |
|------|--------------|-------------------|
| CreateFlow | story_single_3ac.md | mock_bridge_output.md |
| MergeFlow step 1 | story_single_3ac.md | mock_single_story.md |
| MergeFlow step 2 | story_single_3ac.md | mock_merge_completed.md |
| PromptParserContract | story_5ac_traceability.md | mock_source_traceability.md |
| CLI_Success | inline story content | mock_bridge_output.md |
| CLI_Failure | inline story content | (exit 1, no output) |

**No new test fixtures needed.** All fixtures are from Stories 2.3-2.6.

### Merge Flow Test — Between-Steps Modification

After step 1 (create), the test must modify sprint-tasks.md to simulate completed tasks:
```go
// mock_single_story.md has these tasks:
// - [ ] Implement login endpoint with email/password validation
// - [ ] Add request validation middleware for login form fields
// - [ ] Generate and return JWT token with user claims on success

// Mark first two as done (prefix match — strings.Replace finds substring):
modified := strings.Replace(content, "- [ ] Implement login", "- [x] Implement login", 1)
modified = strings.Replace(modified, "- [ ] Add request validation", "- [x] Add request validation", 1)
```

**Fixture content mismatch is intentional:** `mock_merge_completed.md` (step 2 mock output) has different task DESCRIPTIONS than `mock_single_story.md` (step 1 output) — e.g., "Implement user login endpoint" vs "Implement login endpoint with email/password validation". This is acceptable because integration merge test validates the MECHANICS (prompt assembly with existing content, backup creation, output write), NOT the quality of Claude's merge (which is prompt-driven). Content correctness is already verified by unit golden file tests in Story 2.5/2.6.

Step 2's mock returns canned output with `[x]` tasks. The merge test verifies:
1. Output has `[x]` lines (mock returned merged content with completions)
2. Backup file content matches the MODIFIED version (not the original step 1 output)
3. Merge prompt includes "Merge Mode" section and existing `[x]` content (verifies prompt assembly worked)

### findGoTool Priority Chain

```go
func findGoTool() (string, error) {
    // 1. GOEXE env (explicit override for non-standard paths like WSL)
    // 2. exec.LookPath("go") (standard PATH lookup)
    // 3. runtime.GOROOT()/bin/go (compiled-in GOROOT)
    // 4. runtime.GOROOT()/bin/go.exe (Windows variant)
    // 5. error → t.Skipf
}
```

**WSL WARNING:** `runtime.GOROOT()` fallback will NOT work on WSL — it returns a Windows path like `C:\Program Files\Go` which is not a valid WSL path. The only reliable options on WSL are:
1. Set `GOEXE="/mnt/c/Program Files/Go/bin/go.exe"` before running integration tests
2. Add Go to WSL PATH
3. Run integration tests directly via `"/mnt/c/Program Files/Go/bin/go.exe" test -tags integration ./bridge/...`

If none of these work, CLI tests `t.Skipf` gracefully — non-CLI integration tests still run (they don't need the compiled binary).

### extractPromptFromArgs Pattern

The mock Claude logs all received CLI args to `invocation_N.json`. To validate prompt content:
```go
args := testutil.ReadInvocationArgs(t, stateDir, stepNum)
prompt := extractPromptFromArgs(args) // finds "-p" flag, returns next arg
// Now validate: strings.Contains(prompt, "expected content")
```

### What NOT to Do

- Do NOT define a new TestMain — existing one in prompt_test.go handles dispatch
- Do NOT create new test fixtures — reuse existing from Stories 2.3-2.6
- Do NOT modify production code — this is test-only
- Do NOT modify existing test files (bridge_test.go, prompt_test.go)
- Do NOT use `t.TempDir()` for the ralph binary (shared via sync.Once) — use `os.MkdirTemp`
- Do NOT hardcode the Go binary path in tests — use findGoTool with fallback chain
- Do NOT test content correctness in integration tests — unit/golden tests cover that
- Do NOT add new dependencies

### Project Structure Notes

- `bridge/bridge_integration_test.go` — new (only file in this story)
- No other files created or modified
- Build tag `//go:build integration` ensures zero impact on existing test suite

### References

- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md — Story 2.7 AC]
- [Source: docs/project-context.md — Two-stage prompt assembly, bridge role, config immutability, exit codes]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md — Testing patterns, scenario-based mock]
- [Source: docs/architecture/project-structure-boundaries.md — Test scenario format, bridge file structure]
- [Source: bridge/bridge.go — Run() implementation, sprintTasksFile constant, merge detection]
- [Source: bridge/bridge_test.go — Existing test patterns, helpers (copyFixtureToScenario, validateTaskSourcePairs, validateMergeTaskSourcePairs)]
- [Source: bridge/prompt_test.go — TestMain dispatch, goldenTest, BridgePrompt assertion patterns]
- [Source: internal/testutil/mock_claude.go — SetupMockClaude, RunMockClaude, ReadInvocationArgs, multi-step scenarios]
- [Source: session/session.go — Execute(), buildArgs(), env propagation via os.Environ()]
- [Source: session/result.go — ParseResult, JSON array format]
- [Source: config/config.go — Config struct, Load(), detectProjectRoot(), CLIFlags]
- [Source: config/constants.go — TaskOpenRegex, TaskDoneRegex, SourceFieldRegex, GateTagRegex]
- [Source: config/prompt.go — AssemblePrompt, TemplateData]
- [Source: config/defaults.yaml — claude_command default: "claude"]
- [Source: cmd/ralph/main.go — run(), exitCode mapping, signal handling]
- [Source: cmd/ralph/exit.go — Exit code constants (0-4), exitCode() function]
- [Source: cmd/ralph/bridge.go — runBridge(), Cobra arg parsing, config.Load call]
- [Source: docs/sprint-artifacts/2-6-smart-merge.md — Previous story dev notes, merge patterns, fixture patterns]
- [Source: docs/sprint-artifacts/2-5-bridge-golden-file-tests.md — Golden file test patterns]

### Existing Code to Build On

| File | Status | Description |
|------|--------|-------------|
| `bridge/bridge.go` | read-only | Run() entry point — target of integration testing |
| `bridge/bridge_test.go` | read-only | Existing helpers: copyFixtureToScenario, validateTaskSourcePairs, validateMergeTaskSourcePairs |
| `bridge/prompt_test.go` | read-only | TestMain (shared), goldenTest, -update flag |
| `config/config.go` | read-only | Config struct, Load(), detectProjectRoot |
| `config/constants.go` | read-only | Regex patterns for cross-validation |
| `config/prompt.go` | read-only | AssemblePrompt, TemplateData |
| `session/session.go` | read-only | Execute(), env propagation |
| `cmd/ralph/main.go` | read-only | Exit code mapping target |
| `cmd/ralph/bridge.go` | read-only | CLI wiring target |
| `cmd/ralph/exit.go` | read-only | exitCode() function, exit constants |
| `internal/testutil/mock_claude.go` | read-only | SetupMockClaude, RunMockClaude, ReadInvocationArgs |
| `bridge/testdata/*.md` | read-only | 18 existing fixtures — all reused, none modified |

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- CLI tests initially failed: Windows Go (`go.exe`) builds a Windows PE binary but without `.exe` extension, Windows `CreateProcess` couldn't execute it. Fixed by using `runtime.GOOS == "windows"` check to append `.exe` suffix to build output name.

### Completion Notes List

- Created `bridge/bridge_integration_test.go` with `//go:build integration` build tag
- 5 integration test functions: CreateFlow, MergeFlow, PromptParserContract, CLI_Success, CLI_Failure
- 5 helper functions: extractPromptFromArgs, verifyMockFlags, findGoTool, ensureRalphBin, setupCLIProject
- All 5 integration tests pass alongside all existing tests (zero regressions)
- No production code modified — test-only story
- No new test fixtures created — all reused from Stories 2.3-2.6
- CLI tests build ralph binary via sync.Once, use GOEXE env var for WSL compatibility
- findGoTool fallback chain: GOEXE env → exec.LookPath → runtime.GOROOT, with t.Skipf if unavailable

### File List

- `bridge/bridge_integration_test.go` — new (integration test file with build tag)
- `docs/sprint-artifacts/sprint-status.yaml` — modified (status tracking)
- `docs/sprint-artifacts/2-7-bridge-integration-test.md` — modified (story file updates)

### Review Findings & Resolution

Code review found 6 issues (0 High, 3 Medium, 3 Low). All fixed automatically:

**M1 — Story fixture copy boilerplate duplicated 3x:** Extracted `copyStoryFixture(t, fixtureName, destDir)` helper to deduplicate 3 identical fixture copy blocks (CreateFlow, MergeFlow, PromptParserContract).

**M2 — MergeFlow discards step 2 taskCount:** Changed `_, _, err = Run(...)` to `mergeTaskCount, _, err := Run(...)` with assertion `mergeTaskCount != 3` — mock_merge_completed.md has 3 open tasks.

**M3 — MergeFlow doesn't verify between-steps modification:** Added `if modified == string(content) { t.Fatal("...") }` to guard against replacement having no effect (e.g., if fixture changes).

**L1 — Missing Name field in Scenario structs:** Added descriptive `Name` field to all 5 Scenario struct literals: "create-flow", "merge-flow", "parser-contract", "cli-success", "cli-failure".

**L2 — verifyMockFlags doesn't check --max-turns value:** Added `maxTurnsValue` capture and assertion `maxTurnsValue != "5"`.

**L3 — PromptParserContract missing sourceCount==taskOpenCount cross-validation:** Added `sourceCount != taskOpenCount` assertion to verify 1:1 task-to-source mapping.

### Change Log

- 2026-02-26: Implemented Story 2.7 — Bridge integration tests covering full create flow, smart merge flow, prompt-to-parser contract validation, and CLI-level success/failure tests with compiled ralph binary
- 2026-02-26: Fixed 6 code review issues (3 Medium, 3 Low) — extracted copyStoryFixture helper, captured merge taskCount, added between-steps guard, added Scenario Names, enhanced verifyMockFlags and PromptParserContract assertions
