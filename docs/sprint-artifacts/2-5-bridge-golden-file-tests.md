# Story 2.5: Bridge Golden File Tests

Status: done

## Story

As a developer,
I want comprehensive golden file tests for bridge output,
so that any regression in task generation is caught immediately.

## Acceptance Criteria

1. **Test story fixtures exist in bridge/testdata/:**
   Given bridge golden file tests need realistic input, test story files are created in `bridge/testdata/` for each scenario (simple 3-AC, multi-story, dependencies, first-of-epic, 5-AC traceability).

2. **Mock Claude scenarios return deterministic output per test:**
   Given each golden file test needs predictable bridge output, mock Claude scenarios use dedicated output fixtures in `bridge/testdata/` with realistic sprint-tasks.md content.

3. **Golden files verify expected output for 5 test cases:**
   | Test Case | Input | Validates |
   |-----------|-------|-----------|
   | SingleStory | Simple 3-AC story | Basic conversion: 3 tasks with source fields |
   | MultiStory | 2 stories, 1 epic | Multi-file handling: separator, both stories' tasks |
   | WithDependencies | Story needing framework | `[SETUP]` tasks before implementation tasks |
   | GateMarking | First-of-epic story | `[GATE]` on first task and milestone task |
   | SourceTraceability | Story with 5 ACs | `source:` field on every task, AC-1 through AC-5 |

4. **EVERY golden file output validated by regex:**
   After golden file comparison, programmatic validation runs on each golden file:
   - `config.TaskOpenRegex` scan returns > 0 tasks (output is parseable)
   - `config.SourceFieldRegex` matches on every task's source line (no missing source fields)

5. **Golden files use standard pattern:**
   - Files in `bridge/testdata/TestBridge_<Case>.golden`
   - Tests use `go test -update` for golden file refresh (reuse `goldenTest` helper from prompt_test.go)
   - CI without `-update` = mismatch fail

6. **Cross-validation: bridge output → config regex scanner:**
   At least one test feeds bridge golden file output as input for config regex patterns (Story 2.1 cross-validation): scans all lines, extracts task lines via `config.TaskOpenRegex`, verifies each has a subsequent `config.SourceFieldRegex` match. Proves bridge output is parseable by runner's scanning logic.

## Tasks / Subtasks

- [x] Task 1: Create input story fixture files in bridge/testdata/ (AC: #1)
  - [x] 1.1 Create `bridge/testdata/story_single_3ac.md` — simple story with 3 acceptance criteria: login endpoint, input validation, JWT response. Include "As a... I want... so that..." format, numbered ACs in gherkin-style
  - [x] 1.2 Create `bridge/testdata/story_multi_a.md` — first story (user registration, 2 ACs) for multi-story test
  - [x] 1.3 Create `bridge/testdata/story_multi_b.md` — second story (user profile, 2 ACs) for multi-story test
  - [x] 1.4 Create `bridge/testdata/story_with_deps.md` — story that explicitly depends on a NEW testing framework not yet installed. 2 ACs, dependency context triggers [SETUP] generation
  - [x] 1.5 Create `bridge/testdata/story_first_of_epic.md` — story explicitly numbered as "Story 1.1" (first in epic) with 4 ACs including a deployment milestone. Triggers [GATE] on first task and milestone
  - [x] 1.6 Create `bridge/testdata/story_5ac_traceability.md` — story with exactly 5 ACs covering different aspects (data ingestion, transformation, validation, export, monitoring). Exercises full source traceability

- [x] Task 2: Create mock output fixture files in bridge/testdata/ (AC: #2)
  - [x] 2.1 Create `bridge/testdata/mock_single_story.md` — 3 open tasks with source fields (`stories/auth.md#AC-1`, `#AC-2`, `#AC-3`). All tasks use `- [ ]` syntax. Source lines indented 2 spaces. Grouped under `## Epic:` header
  - [x] 2.2 Create `bridge/testdata/mock_multi_story.md` — tasks from both stories under same epic header. 4+ tasks total, each with valid source field referencing correct story file
  - [x] 2.3 Create `bridge/testdata/mock_with_deps.md` — includes `[SETUP]` task BEFORE implementation tasks with `source: stories/api-testing.md#SETUP`. Also `[VERIFY]` task after implementation. Ordering: SETUP → implementation → VERIFY
  - [x] 2.4 Create `bridge/testdata/mock_gate_marking.md` — first task has `[GATE]` appended. Milestone task (deploy to staging) also has `[GATE]`. 4 tasks total, each with source field
  - [x] 2.5 Create `bridge/testdata/mock_source_traceability.md` — 5 implementation tasks mapping to AC-1 through AC-5 (first task has `[GATE]` for cross-validation coverage), plus `[SETUP]`, `[VERIFY]`, `[E2E]` service tasks with respective source identifiers. Total 8 tasks. EVERY task has valid `source:` field matching `SourceFieldRegex`. This is the richest fixture — used by cross-validation test to verify ALL config regex patterns
  - [x] 2.6 **Validation**: Verify ALL mock output files pass manual regex check — every line starting with `- [ ]` has a subsequent indented `source:` line matching `^\s+source:\s+\S+#\S+`

- [x] Task 3: Implement table-driven golden file test function (AC: #3, #4, #5)
  - [x] 3.1 In `bridge/bridge_test.go`, add `TestRun_GoldenFiles` — table-driven test with 5 subcases: `SingleStory`, `MultiStory`, `WithDependencies`, `GateMarking`, `SourceTraceability`
  - [x] 3.2 Test case struct fields: `name string`, `storyFiles []string` (fixture names), `mockOutput string` (mock fixture name), `goldenFile string`, `wantTaskCount int` (expected task count — see Quick Reference table), `extraCheck func(t *testing.T, output string)` (per-scenario assertions from 3.6-3.9, nil if none)
  - [x] 3.3 For each subcase: create TWO t.TempDir() (projectDir for output, storyDir for story fixtures), copy story fixture(s) to storyDir, set up mock Claude scenario with `OutputFile` pointing to mock output fixture, call `Run(ctx, cfg, storyPaths)` (package-internal call, NOT bridge.Run), verify no error, verify returned taskCount matches `wantTaskCount`
  - [x] 3.4 Read output sprint-tasks.md, compare to golden file using `goldenTest(t, goldenFile, got)` helper (already in prompt_test.go, accessible within same package)
  - [x] 3.5 After golden comparison, run regex validation on output via extracted helper `validateTaskSourcePairs(t *testing.T, output string) int` (returns task count):
    - Split output by `\n`, count lines matching `config.TaskOpenRegex` → must be > 0
    - For each task line found, verify the NEXT line matches `config.SourceFieldRegex`
    - If any task lacks valid source line → `t.Errorf("task at line %d has no valid source field", lineNum)`
    - Helper is reused by ALL 5 subcases AND by TestRun_CrossValidation_BridgeToScanner
  - [x] 3.6 `MultiStory` subcase: additionally verify both story names appear somewhere in source fields (cross-file validation)
  - [x] 3.7 `WithDependencies` subcase: additionally verify at least one task line contains `[SETUP]` prefix
  - [x] 3.8 `GateMarking` subcase: additionally verify at least one task line contains `[GATE]` suffix
  - [x] 3.9 `SourceTraceability` subcase: verify EACH of `#AC-1`, `#AC-2`, `#AC-3`, `#AC-4`, `#AC-5` individually via `strings.Contains(output, "#AC-N")` — all 5 must be present. Additionally verify at least one service identifier (`#SETUP`, `#VERIFY`, or `#E2E`)

- [x] Task 4: Add cross-validation test function (AC: #6)
  - [x] 4.1 Add `TestRun_CrossValidation_BridgeToScanner` in `bridge/bridge_test.go`
  - [x] 4.2 Load the `TestBridge_SourceTraceability.golden` file (richest golden file with most task types)
  - [x] 4.3 Scan all lines: extract tasks via `config.TaskOpenRegex`, extract source fields via `config.SourceFieldRegex`
  - [x] 4.4 Verify: number of task lines == number of source lines (1:1 mapping)
  - [x] 4.5 Verify: `config.GateTagRegex` finds at least one match (SourceTraceability fixture includes [GATE] task)
  - [x] 4.6 Verify: output contains `[SETUP]`, `[VERIFY]`, or `[E2E]` service prefixes (at least one)
  - [x] 4.7 This test documents the bridge→runner contract: bridge output is consumable by config regex patterns

- [x] Task 5: Generate golden files and run all tests (AC: all)
  - [x] 5.1 Run `go test -update ./bridge/...` to generate golden files from mock output
  - [x] 5.2 Run `go test ./bridge/...` — all bridge tests pass (new + existing)
  - [x] 5.3 Run `go test ./config/...` — no regressions
  - [x] 5.4 Run `go vet ./...` — no vet issues
  - [x] 5.5 Verify golden files are non-empty and contain expected content (spot-check)

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Package declaration:** `package bridge` (internal test package, same as bridge.go). Call `Run(...)` directly, NOT `bridge.Run(...)`. All unexported helpers (`goldenTest`, `copyFixtureToScenario`, `-update` flag var) are accessible across test files.

**Config setup (required for every test):**
```go
projectDir := t.TempDir()  // cfg.ProjectRoot — sprint-tasks.md output goes here
storyDir := t.TempDir()    // story fixtures copied here (SEPARATE from output!)
cfg := &config.Config{
    ClaudeCommand: os.Args[0],  // REQUIRED: self-reexec test binary for mock
    ProjectRoot:   projectDir,  // REQUIRED: where sprint-tasks.md is written
    MaxTurns:      5,           // Optional: standard value for tests
}
```

**Output path:** `filepath.Join(cfg.ProjectRoot, "sprint-tasks.md")` (constant `sprintTasksFile` in bridge.go).

**Expected task counts per fixture:**
| Fixture | wantTaskCount |
|---------|---------------|
| mock_single_story.md | 3 |
| mock_multi_story.md | 4 |
| mock_with_deps.md | 4 |
| mock_gate_marking.md | 4 |
| mock_source_traceability.md | 8 |

**Story fixture content is IRRELEVANT to mock output.** Mock Claude returns canned fixture content regardless of prompt. Story fixtures exist for: (a) bridge.Run input validation (file must exist and be readable), (b) documentation of test scenario intent. Keep them minimal — a few lines each is sufficient.

### Scope: Test-Only Story

This story adds ONLY test files and test fixtures. No production code changes to `bridge/bridge.go`, `bridge/prompts/bridge.md`, or any config package files. The bridge.Run logic and prompt template are complete from Stories 2.2-2.4.

### Current Test Coverage to Build On

Existing `bridge/bridge_test.go` already has 7 test functions covering bridge.Run:
- `TestRun_Success` — happy path with full output comparison to `mock_bridge_output.md` fixture
- `TestRun_StoryFileNotFound`, `TestRun_SessionExecuteFails`, `TestRun_ParseResultError` — error paths
- `TestRun_MultipleStories` — multi-file with separator verification
- `TestRun_NoStoryFiles`, `TestRun_WriteFailure` — edge cases

**What Story 2.5 adds beyond existing tests:**
- **Scenario variety** — 5 specific scenarios exercising [SETUP], [GATE], [VERIFY], [E2E], multi-story, full traceability (existing tests only verify basic conversion)
- **Golden file pattern with -update** — existing tests compare to fixture directly; golden file adds the standard `-update` refresh mechanism
- **Per-task source validation** — existing tests count tasks but don't verify every task has valid source
- **Cross-validation** — explicitly proves bridge output is parseable by runner's regex patterns

### Test Infrastructure: Reuse Established Patterns

**Mock Claude setup** — use inline Go struct scenarios (established pattern from Story 2.3), NOT JSON files on disk. The AC's mention of `internal/testutil/scenarios/bridge_*.json` is aspirational; the actual codebase uses:
```go
scenario := testutil.Scenario{
    Name: "bridge-golden-single",
    Steps: []testutil.ScenarioStep{{
        Type: "execute", ExitCode: 0, SessionID: "mock-golden-1",
        OutputFile: "mock_single_story.md",
    }},
}
_, stateDir := testutil.SetupMockClaude(t, scenario)
```

**Fixture copy helper** — reuse `copyFixtureToScenario(t, fixtureName)` from bridge_test.go (extracted in Story 2.3). Copies fixture from testdata/ to mock scenario dir so OutputFile can find it.

**Golden file helper** — reuse `goldenTest(t, goldenFile, got)` from prompt_test.go. Both files are in the same test package, so the helper is accessible. It handles `-update` flag for golden file refresh.

**Config reference** — use `cfg.ClaudeCommand = os.Args[0]` for self-reexec pattern (standard from Story 1.11).

### Golden File Content = Mock Output Fixture

Since `bridge.Run` passes Claude output directly to sprint-tasks.md (via `session.ParseResult` → `os.WriteFile`), the golden file content is IDENTICAL to the mock output fixture content. The test validates:
1. The full pipeline executes without error
2. Output file matches golden file exactly (no corruption, truncation, or transformation)
3. Output content is valid sprint-tasks.md format (regex validation)

### Mock Output Fixture Requirements (CRITICAL)

ALL mock output fixtures MUST be valid sprint-tasks.md format per `config/shared/sprint-tasks-format.md`:
- Tasks: `- [ ] Task description` (matches `config.TaskOpenRegex = ^\s*- \[ \]`)
- Source: `  source: stories/<file>.md#<identifier>` on next line (matches `config.SourceFieldRegex = ^\s+source:\s+\S+#\S+`)
- Service tasks: `[SETUP]`, `[VERIFY]`, `[E2E]` prefixes in task line
- Gates: `[GATE]` suffix on task line (matches `config.GateTagRegex = \[GATE\]`)
- Grouping: `## Epic: <name>` headers
- No spaces in path or identifier after `#`

**Pre-commit check:** Run `grep -P '^\s*- \[ \]' fixture.md` and `grep -P '^\s+source:\s+\S+#\S+' fixture.md` — counts MUST be equal (1:1 task:source mapping).

### Test Naming Convention

Per project convention `Test<Type>_<Method>_<Scenario>`:
- Main function: `TestRun_GoldenFiles` (table-driven, 5 subcases via `t.Run`)
- Subcases: `SingleStory`, `MultiStory`, `WithDependencies`, `GateMarking`, `SourceTraceability`
- Cross-validation: `TestRun_CrossValidation_BridgeToScanner`

The AC names (`TestBridge_*`) are subtest names accessed via `TestRun_GoldenFiles/SingleStory`, etc.

### Regex Validation Helper (Task 3.5)

Extract as `validateTaskSourcePairs(t *testing.T, output string) int` — returns task count. Reused by all 5 subcases and by cross-validation test:
```go
func validateTaskSourcePairs(t *testing.T, output string) int {
    t.Helper()
    lines := strings.Split(output, "\n")
    taskCount := 0
    for i, line := range lines {
        if config.TaskOpenRegex.MatchString(line) {
            taskCount++
            if i+1 >= len(lines) || !config.SourceFieldRegex.MatchString(lines[i+1]) {
                t.Errorf("task at line %d has no valid source field on next line", i+1)
            }
        }
    }
    if taskCount == 0 {
        t.Errorf("golden file has no parseable tasks")
    }
    return taskCount
}
```

### Cross-Validation Purpose

The `TestRun_CrossValidation_BridgeToScanner` test explicitly documents the contract between bridge (producer) and runner (consumer). It loads a golden file (not running bridge.Run) and validates it with all config regex patterns. If the format contract changes, this test breaks — preventing silent format drift between bridge and runner.

### Review Learnings Applied Proactively

**From Story 2.3 (directly relevant):**
- Fixture copy boilerplate → use `copyFixtureToScenario(t, name)` helper (already extracted)
- Full output comparison when source is deterministic — golden file does this
- Separator assertions use full `"\n\n---\n\n"` pattern for MultiStory

**From Story 2.2:**
- Test assertion symmetry — ALL 5 golden file tests run the SAME regex validation code
- Consistent assertion patterns — table-driven with struct fields, not hardcoded per-case logic

**From Story 2.1:**
- Structural Rule #8 — cross-validation test proves bridge output compatible with config regex
- No dead golden files — every `.golden` file loaded by a test case

**From Story 2.4:**
- Assertion uniqueness — per-scenario assertions (gate, setup, source coverage) use specific checks

**From Story 1.11:**
- Self-reexec TestMain already handles mock Claude dispatch — no changes needed to TestMain

### What NOT to Do

- Do NOT modify bridge.go, bridge/prompts/bridge.md, or config/ code
- Do NOT create JSON scenario files in internal/testutil/scenarios/ — use inline Go structs
- Do NOT add build tags — these are standard unit tests, not integration tests
- Do NOT add new dependencies
- Do NOT merge golden file tests into existing TestRun_Success — keep separate for scenario clarity

### Project Structure Notes

- `bridge/testdata/story_single_3ac.md` — new (input story fixture)
- `bridge/testdata/story_multi_a.md` — new (input story fixture)
- `bridge/testdata/story_multi_b.md` — new (input story fixture)
- `bridge/testdata/story_with_deps.md` — new (input story fixture)
- `bridge/testdata/story_first_of_epic.md` — new (input story fixture)
- `bridge/testdata/story_5ac_traceability.md` — new (input story fixture)
- `bridge/testdata/mock_single_story.md` — new (mock output fixture)
- `bridge/testdata/mock_multi_story.md` — new (mock output fixture)
- `bridge/testdata/mock_with_deps.md` — new (mock output fixture)
- `bridge/testdata/mock_gate_marking.md` — new (mock output fixture)
- `bridge/testdata/mock_source_traceability.md` — new (mock output fixture)
- `bridge/testdata/TestBridge_SingleStory.golden` — new (generated via -update)
- `bridge/testdata/TestBridge_MultiStory.golden` — new (generated via -update)
- `bridge/testdata/TestBridge_WithDependencies.golden` — new (generated via -update)
- `bridge/testdata/TestBridge_GateMarking.golden` — new (generated via -update)
- `bridge/testdata/TestBridge_SourceTraceability.golden` — new (generated via -update)
- `bridge/bridge_test.go` — modified (add 2 test functions: TestRun_GoldenFiles, TestRun_CrossValidation_BridgeToScanner)

### References

- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md — Story 2.5 AC, lines 210-249]
- [Source: docs/project-context.md — Two-stage prompt assembly, bridge role, testing patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md — Golden file pattern, -update flag, CI behavior]
- [Source: docs/architecture/project-structure-boundaries.md — bridge/testdata/ layout, cross-validation]
- [Source: config/constants.go — TaskOpenRegex, SourceFieldRegex, GateTagRegex]
- [Source: config/shared/sprint-tasks-format.md — Format contract with task/source syntax]
- [Source: bridge/bridge.go — Run function pipeline, atomic write, 3-value return]
- [Source: bridge/bridge_test.go — Existing TestRun_* tests, copyFixtureToScenario helper]
- [Source: bridge/prompt_test.go — goldenTest helper, TestMain dispatch]
- [Source: internal/testutil/mock_claude.go — SetupMockClaude, ScenarioStep, RunMockClaude]
- [Source: docs/sprint-artifacts/2-3-bridge-logic-core-conversion.md — copyFixtureToScenario extraction, full output comparison]
- [Source: docs/sprint-artifacts/2-4-service-tasks-gate-marking-source-traceability.md — Enriched prompt, assertion symmetry]

### Existing Code to Build On

| File | Status | Description |
|------|--------|-------------|
| `bridge/bridge_test.go` | modify | Add TestRun_GoldenFiles (5 subcases) + TestRun_CrossValidation_BridgeToScanner |
| `bridge/prompt_test.go` | read-only | Reuse goldenTest helper and TestMain dispatch |
| `bridge/bridge.go` | read-only | Run function, BridgePrompt accessor |
| `internal/testutil/mock_claude.go` | read-only | SetupMockClaude, ScenarioStep, RunMockClaude |
| `config/constants.go` | read-only | TaskOpenRegex, SourceFieldRegex, GateTagRegex |
| `bridge/testdata/mock_bridge_output.md` | read-only | Existing fixture pattern reference |

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- All 5 golden file test subcases pass: SingleStory (3 tasks), MultiStory (4 tasks), WithDependencies (4 tasks), GateMarking (4 tasks), SourceTraceability (8 tasks)
- Cross-validation test passes: bridge golden file output is parseable by config regex patterns (TaskOpenRegex, SourceFieldRegex, GateTagRegex)
- validateTaskSourcePairs helper validates 1:1 task:source mapping across all golden files
- Per-scenario extra checks: MultiStory verifies cross-file source references, WithDependencies verifies [SETUP], GateMarking verifies [GATE], SourceTraceability verifies all 5 ACs and service identifiers
- No production code changes — test-only story
- All existing bridge and config tests pass with no regressions
- go vet clean

### Implementation Plan

Test-only story: created 6 story fixtures, 5 mock output fixtures, 1 shared validation helper, 1 table-driven golden file test (5 subcases), and 1 cross-validation test. Reused existing copyFixtureToScenario and goldenTest helpers. All mock fixtures validated for 1:1 task:source regex mapping before test implementation.

### File List

**New files (added):**
- bridge/testdata/story_single_3ac.md — input story fixture (3 ACs, login scenario)
- bridge/testdata/story_multi_a.md — input story fixture (user registration, 2 ACs)
- bridge/testdata/story_multi_b.md — input story fixture (user profile, 2 ACs)
- bridge/testdata/story_with_deps.md — input story fixture (API testing with deps, 2 ACs)
- bridge/testdata/story_first_of_epic.md — input story fixture (first-of-epic, 4 ACs with milestone)
- bridge/testdata/story_5ac_traceability.md — input story fixture (data pipeline, 5 ACs)
- bridge/testdata/mock_single_story.md — mock output fixture (3 tasks)
- bridge/testdata/mock_multi_story.md — mock output fixture (4 tasks, 2 stories)
- bridge/testdata/mock_with_deps.md — mock output fixture (4 tasks, SETUP+VERIFY)
- bridge/testdata/mock_gate_marking.md — mock output fixture (4 tasks, 2 GATEs)
- bridge/testdata/mock_source_traceability.md — mock output fixture (8 tasks, full coverage)
- bridge/testdata/TestBridge_SingleStory.golden — golden file (generated)
- bridge/testdata/TestBridge_MultiStory.golden — golden file (generated)
- bridge/testdata/TestBridge_WithDependencies.golden — golden file (generated)
- bridge/testdata/TestBridge_GateMarking.golden — golden file (generated)
- bridge/testdata/TestBridge_SourceTraceability.golden — golden file (generated)

**Modified files:**
- bridge/bridge_test.go — added validateTaskSourcePairs helper, TestRun_GoldenFiles (5 subcases), TestRun_CrossValidation_BridgeToScanner

## Change Log

- 2026-02-26: Implemented Story 2.5 — comprehensive golden file tests for bridge output with 5 scenario variants and cross-validation test proving bridge→runner contract compatibility
- 2026-02-26: Code review fixes — 3 MED + 1 LOW: cross-validation comment precision, GateMarking count >= 2 assertion, WithDependencies [VERIFY] assertion, SourceTraceability individual service identifier checks

## Senior Developer Review (AI)

**Review Date:** 2026-02-26
**Reviewer Model:** Claude Opus 4.6
**Review Outcome:** Approve (with fixes applied)

### Action Items

- [x] [MED] Cross-validation doc comment overclaims "all config regex patterns" — fixed to "bridge-relevant" with explicit list
- [x] [MED] GateMarking extraCheck verifies only existence, not count >= 2 — fixed with strings.Count
- [x] [MED] WithDependencies extraCheck doesn't verify [VERIFY] presence — added assertion
- [x] [LOW] SourceTraceability extraCheck uses || instead of verifying all 3 service identifiers — fixed with individual loop
- [ ] [LOW] Sprint-status.yaml modified but not in story File List — standard workflow artifact, no fix needed
- [ ] [LOW] validateTaskSourcePairs line error could include golden file context — minor debuggability, deferred
