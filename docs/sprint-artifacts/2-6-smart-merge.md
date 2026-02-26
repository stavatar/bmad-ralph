# Story 2.6: Smart Merge

Status: Done

## Story

As a developer,
I want bridge to merge new tasks into existing sprint-tasks.md without losing progress,
so that I can re-bridge after story updates without losing completed work.

## Acceptance Criteria

1. **Merge mode auto-detection and backup:**
   Given sprint-tasks.md exists at `cfg.ProjectRoot` with some tasks marked `[x]` (completed) and story files have been updated with new/changed requirements, when `ralph bridge` is run again, then existing sprint-tasks.md is BACKED UP to `sprint-tasks.md.bak` and bridge prompt includes existing sprint-tasks.md content (merge mode) and Claude merges: new tasks added, completed `[x]` tasks preserved and modified tasks updated but completion status preserved.

2. **Merge prompt assembly with explicit instructions:**
   Given the merge prompt is assembled and `{{if .HasExistingTasks}}` evaluates to true, then prompt includes existing sprint-tasks.md content and explicit instruction: "MUST NOT change [x] status of completed tasks" and explicit instruction: "MUST preserve source: fields of existing tasks" and explicit instruction: "PRESERVE original task order. New tasks insert at logical position. NEVER reorder existing tasks".

3. **Merge failure handling — backup remains intact:**
   Given merge fails (session error or output malformed), when error occurs, then backup file `sprint-tasks.md.bak` remains intact and backup file content equals original sprint-tasks.md byte-for-byte and original sprint-tasks.md is NOT modified and descriptive error returned with `"bridge: merge:"` prefix.

4. **Golden file tests for merge scenarios:**
   Golden file tests cover: merge with 2 completed + 3 new tasks; merge where story changed an existing task description; merge that adds `[GATE]` to previously un-gated task. Verify `[x]` status preserved in all cases. Verify existing task order unchanged after merge.

5. **Mandatory AC (Risk Heatmap):**
   Backup sprint-tasks.md before merge. Golden file tests for merge scenarios.

## Tasks / Subtasks

- [x] Task 1: Enhance bridge.md merge instructions (AC: #2)
  - [x] 1.1 In `bridge/prompts/bridge.md`, replace the placeholder comment `<!-- Story 2.6 enriches with detailed merge instructions -->` with explicit AC-mandated instructions:
    - Add: `MUST NOT change [x] status of completed tasks — this is the most critical merge requirement.`
    - Add: `Modified tasks: if a task description changed in the updated story, update the description text but PRESERVE the completion status ([x] or [ ]).`
    - Change "append new tasks at the end of the relevant epic section" to: `PRESERVE original task order. New tasks insert at logical position within the relevant epic section. NEVER reorder existing tasks.`
  - [x] 1.2 In `bridge/prompt_test.go`, add assertions to `TestBridgePrompt_Merge` for AC #2 mandated instructions. Add these substring checks alongside existing assertions:
    - `"MUST NOT change [x] status"` (or unique substring from the new instruction)
    - `"PRESERVE original task order"` (or unique substring)
    - `"Modified tasks"` (unique to the description-update instruction)
    These prevent accidental removal of critical merge instructions.
  - [x] 1.3 Run `go test -args -update ./bridge/...` to regenerate prompt golden files (TestBridgePrompt_Creation.golden and TestBridgePrompt_Merge.golden will be updated)
  - [x] 1.4 Run `go test ./bridge/...` — all existing bridge tests pass

- [x] Task 2: Add merge detection, backup, and data passing to bridge.go (AC: #1, #3)
  - [x] 2.1 Add constant: `const sprintTasksBakSuffix = ".bak"` next to existing `sprintTasksFile` constant
  - [x] 2.2 After story content assembly (step 2.1 in current code) and BEFORE prompt assembly (step 2.2), add merge detection. Store raw bytes for byte-for-byte backup (AC #3), convert to string for prompt:
    ```go
    existingPath := filepath.Join(cfg.ProjectRoot, sprintTasksFile)
    var existingData []byte    // raw bytes for backup (byte-for-byte guarantee)
    var existingContent string // string for prompt injection
    mergeMode := false
    if info, err := os.Stat(existingPath); err == nil && !info.IsDir() {
        data, readErr := os.ReadFile(existingPath)
        if readErr != nil {
            return 0, 0, fmt.Errorf("bridge: merge: read existing: %w", readErr)
        }
        existingData = data
        existingContent = string(data)
        mergeMode = true
    }
    ```
  - [x] 2.3 In merge mode, create backup BEFORE any mutation. Use `existingData` (raw bytes) for byte-for-byte fidelity:
    ```go
    if mergeMode {
        bakPath := existingPath + sprintTasksBakSuffix
        if err := os.WriteFile(bakPath, existingData, 0644); err != nil {
            return 0, 0, fmt.Errorf("bridge: merge: backup: %w", err)
        }
    }
    ```
  - [x] 2.4 Update the EXISTING AssemblePrompt call (bridge.go lines 49-57) to use mergeMode. Replace the full block:
    ```go
    // BEFORE (create-only):
    // config.TemplateData{HasExistingTasks: false},
    // "__EXISTING_TASKS__":  "",

    // AFTER (merge-aware):
    prompt, err := config.AssemblePrompt(
        bridgePrompt,
        config.TemplateData{HasExistingTasks: mergeMode},
        map[string]string{
            "__STORY_CONTENT__":   storyContent,
            "__FORMAT_CONTRACT__": config.SprintTasksFormat(),
            "__EXISTING_TASKS__":  existingContent,
        },
    )
    ```
    Only TWO lines change: `false` → `mergeMode`, and `""` → `existingContent`. No new AssemblePrompt call — modify existing one.
  - [x] 2.5 No new imports needed: `os`, `filepath`, `fmt` already in bridge.go imports. `errors` is NOT needed — detection uses `os.Stat` + `info.IsDir()`, not `errors.Is`

- [x] Task 3: Create merge test fixtures (AC: #4)
  - [x] 3.1 Create `bridge/testdata/existing_completed.md` — existing sprint-tasks.md with 2 completed `[x]` tasks and 3 open `[ ]` tasks, all with valid `source:` fields. Include `## Epic: Authentication` header. Use realistic task descriptions matching story_single_3ac.md context
  - [x] 3.2 Create `bridge/testdata/existing_no_gate.md` — existing sprint-tasks.md with 3 open tasks, NO `[GATE]` tags. Will be used for the "add gate" merge scenario
  - [x] 3.3 Create `bridge/testdata/existing_changed_desc.md` — existing sprint-tasks.md with 2 completed `[x]` tasks where one has an outdated description that the merge will update
  - [x] 3.4 Reuse existing `bridge/testdata/story_single_3ac.md` (from Story 2.5) as the story input for all merge test scenarios — no new story fixture needed. Mock Claude returns canned output regardless of prompt content, so the same story works for all merge cases
  - [x] 3.5 Create `bridge/testdata/mock_merge_completed.md` — mock merged output: existing 2 `[x]` tasks preserved exactly, 3 new `[ ]` tasks added, all with `source:` fields. Total 5 tasks (2 done + 3 open)
  - [x] 3.6 Create `bridge/testdata/mock_merge_description.md` — mock merged output: 2 `[x]` tasks preserved (one with updated description, `[x]` status kept), other tasks unchanged
  - [x] 3.7 Create `bridge/testdata/mock_merge_gate.md` — mock merged output: `[GATE]` added to first task (was un-gated), existing `[x]` tasks preserved
  - [x] 3.8 Validate ALL mock fixtures: every `- [ ]` and `- [x]` task line has a subsequent indented `source:` field matching `config.SourceFieldRegex`
  - [x] 3.9 **CRLF fix (WSL/NTFS):** After creating ALL fixture files with Write tool, run `sed -i 's/\r$//' bridge/testdata/existing_*.md bridge/testdata/mock_merge_*.md` — Write tool on NTFS creates CRLF which breaks test comparisons

- [x] Task 4: Add merge golden file tests (AC: #4, #5)
  - [x] 4.1 In `bridge/bridge_test.go`, add `TestRun_MergeGoldenFiles` — table-driven test with 3 subcases: `MergeWithCompleted`, `MergeDescriptionChange`, `MergeAddGate`
  - [x] 4.2 Test case struct fields: `name string`, `storyFiles []string`, `existingFile string` (fixture name for existing sprint-tasks.md), `mockOutput string`, `goldenFile string`, `wantTaskCount int`, `wantDoneCount int` (count of `[x]` lines), `extraCheck func(t *testing.T, output string)`
  - [x] 4.3 For each subcase: create projectDir with TempDir, copy existing fixture to `filepath.Join(projectDir, "sprint-tasks.md")`, copy story fixture to storyDir, set up mock Claude, call Run, verify no error
  - [x] 4.4 After Run: verify `.bak` file exists at `filepath.Join(projectDir, "sprint-tasks.md.bak")` and its content matches original existing fixture byte-for-byte
  - [x] 4.5 Read output sprint-tasks.md, compare to golden file with `goldenTest(t, goldenFile, got)`
  - [x] 4.6 Run `validateMergeTaskSourcePairs(t, got)` on output — this NEW helper checks source: for BOTH `[ ]` and `[x]` lines (unlike `validateTaskSourcePairs` which only checks `[ ]`). Returns `(openCount, doneCount)`. Verify `openCount == tc.wantTaskCount` and `doneCount == tc.wantDoneCount`. See Dev Notes "validateTaskSourcePairs Limitation for Merge" for helper implementation
  - [x] 4.7 `MergeWithCompleted` extraCheck: verify `strings.Count(output, "- [x]") >= 2` (2 completed tasks preserved), verify `strings.Count(output, "- [ ]") >= 3` (3 new tasks added). Additionally verify task order preservation: extract the first 2 `[x]` task description substrings from the `existing_completed.md` fixture and assert `strings.Index(output, task1Desc) < strings.Index(output, task2Desc)` (existing tasks appear in original order before new tasks)
  - [x] 4.8 `MergeDescriptionChange` extraCheck: verify `[x]` count preserved, verify updated description substring present
  - [x] 4.9 `MergeAddGate` extraCheck: verify `[GATE]` present in output (was absent in existing), verify `[x]` status preserved
  - [x] 4.10 Run `go test -args -update ./bridge/...` to generate merge golden files

- [x] Task 5: Add merge error path tests (AC: #3)
  - [x] 5.1 `TestRun_MergeSessionFailure` — create existing sprint-tasks.md in projectDir, set up mock Claude to return exit 1, call Run, verify error contains `"bridge: execute:"`, verify `.bak` file exists and matches original, verify sprint-tasks.md unchanged (content equals original)
  - [x] 5.2 `TestRun_MergeBackupFailure` — setup order is critical:
    ```go
    projectDir := t.TempDir()
    // 1. Create existing file FIRST (triggers merge mode)
    existingContent := "## Epic: Auth\n\n- [x] Existing task\n  source: stories/auth.md#AC-1\n"
    existingPath := filepath.Join(projectDir, "sprint-tasks.md")
    os.WriteFile(existingPath, []byte(existingContent), 0644)
    // 2. Block backup path with directory
    bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
    os.MkdirAll(bakPath, 0755)
    ```
    Call Run, verify error contains `"bridge: merge: backup:"`. Verify original sprint-tasks.md content unchanged:
    ```go
    afterContent, _ := os.ReadFile(existingPath)
    if string(afterContent) != existingContent {
        t.Errorf("original sprint-tasks.md was modified during backup failure")
    }
    ```
  - [x] 5.3 `TestRun_MergeParseError` — create existing sprint-tasks.md in projectDir (triggers merge + backup), then `t.Setenv("BRIDGE_TEST_EMPTY_OUTPUT", "1")` — NO `SetupMockClaude` needed (subprocess exits before mock dispatch, returning exit 0 with empty stdout → ParseResult fails). Call Run, verify error contains `"bridge: parse result:"`, verify `.bak` file exists and matches original byte-for-byte, verify sprint-tasks.md content equals original (unchanged)

- [x] Task 5b: Add merge read error test (AC: #3, CLAUDE.md: "test ALL error paths")
  - [x] 5b.1 `TestRun_MergeReadExistingFailure` — trigger the `"bridge: merge: read existing:"` error path. This occurs when `os.Stat` succeeds (file exists, not a dir) but `os.ReadFile` fails. On WSL/NTFS, permission tricks may not work. Use a symlink to a nonexistent target:
    ```go
    projectDir := t.TempDir()
    existingPath := filepath.Join(projectDir, "sprint-tasks.md")
    os.Symlink("/nonexistent/target", existingPath) // Stat succeeds, ReadFile fails
    ```
    Verify error contains `"bridge: merge: read existing:"`. Verify no `.bak` file created (backup happens AFTER successful read). If symlink approach doesn't work on WSL/NTFS, document as accepted gap with comment explaining why.

- [x] Task 6: Verify backward compatibility and run all tests (AC: all)
  - [x] 6.1 Run `go test ./bridge/...` — ALL existing create-mode tests pass unchanged (TestRun_Success, TestRun_WriteFailure, TestRun_GoldenFiles, etc.)
  - [x] 6.2 Run `go test ./config/...` — no regressions
  - [x] 6.3 Run `go vet ./...` — no issues
  - [x] 6.4 Verify TestRun_WriteFailure still works: directory at sprint-tasks.md path → `os.Stat` returns non-nil info but `info.IsDir() == true` → skips merge mode → create mode → WriteFile fails on directory → same error behavior as before

## Dev Notes

### Quick Reference (CRITICAL — read first)

**Package declaration:** `package bridge` (internal test package, same as bridge.go). Call `Run(...)` directly, NOT `bridge.Run(...)`. All unexported helpers (`goldenTest`, `copyFixtureToScenario`, `validateTaskSourcePairs`, `-update` flag var) are accessible across test files.

**Config setup (required for every test):**
```go
projectDir := t.TempDir()  // cfg.ProjectRoot — sprint-tasks.md output goes here
storyDir := t.TempDir()    // story fixtures copied here (SEPARATE from output!)
cfg := &config.Config{
    ClaudeCommand: os.Args[0],  // REQUIRED: self-reexec test binary for mock
    ProjectRoot:   projectDir,  // REQUIRED: where sprint-tasks.md is written
    MaxTurns:      5,
}
```

**Merge mode trigger:** Place an existing sprint-tasks.md file at `filepath.Join(projectDir, "sprint-tasks.md")` BEFORE calling `Run()`. This triggers merge detection automatically.

**Output path:** `filepath.Join(cfg.ProjectRoot, "sprint-tasks.md")` (constant `sprintTasksFile` in bridge.go).
**Backup path:** `filepath.Join(cfg.ProjectRoot, "sprint-tasks.md.bak")`.

### Scope: bridge.go Logic + Prompt Enhancement + Tests

This story modifies production code in TWO files:
1. `bridge/bridge.go` — merge detection, backup, data passing to AssemblePrompt
2. `bridge/prompts/bridge.md` — enhance merge instructions per AC #2

And adds test files/fixtures in `bridge/bridge_test.go` and `bridge/testdata/`.

No changes to `config/`, `session/`, `cmd/ralph/`, or any other package.

### Architecture: Merge is Prompt-Driven, Not Go Logic

**Critical design decision from epic:** "Merge is prompt-driven (Claude does the merge), not Go code merge." Go code responsibilities:
1. Detect existing sprint-tasks.md (auto-detect, no CLI flag)
2. Read and backup existing content
3. Pass existing content to Claude via prompt (`__EXISTING_TASKS__` placeholder)
4. Claude produces the COMPLETE merged sprint-tasks.md
5. Go writes the merged result (same as create mode)

Go does NOT perform structural merge (diffing tasks, matching descriptions). Claude handles all merge intelligence via prompt instructions.

### Merge Detection: os.Stat + IsDir Guard

Use `os.Stat` followed by `!info.IsDir()` check, NOT `os.ReadFile` directly. Reason: `TestRun_WriteFailure` creates a DIRECTORY at `sprint-tasks.md` path. If we used `os.ReadFile` directly, the directory would cause a "bridge: merge: read existing:" error instead of the expected "bridge: write tasks:" error. The `IsDir()` guard ensures directories are treated as "not existing" for merge purposes, preserving backward compatibility.

```go
if info, err := os.Stat(existingPath); err == nil && !info.IsDir() {
    // Regular file exists → merge mode
}
// If err != nil (not found) OR info.IsDir() → create mode (unchanged behavior)
```

### Backup Timing: BEFORE Session Execute

Backup is created BEFORE calling `session.Execute`. This means:
- On session error: original sprint-tasks.md untouched (haven't written yet), .bak exists
- On parse error: same as above
- On write error: .bak exists, original may be partially corrupted but .bak is safe
- On success: sprint-tasks.md overwritten with merged content, .bak has original

The AC requirement "original sprint-tasks.md is NOT modified" on failure is satisfied naturally — we only call `os.WriteFile(outPath, ...)` on success (after all validation).

### Error Wrapping Convention

All merge-specific errors use `"bridge: merge: <op>: %w"` prefix:
- `"bridge: merge: read existing: %w"` — existing file exists but unreadable
- `"bridge: merge: backup: %w"` — backup creation failed

Non-merge errors keep existing prefixes:
- `"bridge: execute: %w"` — session failure (same prefix for create and merge)
- `"bridge: parse result: %w"` — parse failure (same prefix)
- `"bridge: write tasks: %w"` — write failure (same prefix)

### Prompt Template: Existing Merge Block

The `bridge/prompts/bridge.md` already contains a merge mode conditional block (added in Story 2.2, enriched in 2.4). The current content:
```
{{- if .HasExistingTasks}}

## Merge Mode

An existing sprint-tasks.md is provided below. You MUST merge the new story tasks into it:
- Preserve ALL existing tasks and their `- [x]` / `- [ ]` completion status.
- Preserve ALL existing `source:` fields exactly as they are.
- Preserve the existing task order — append new tasks at the end of the relevant epic section.
...
<!-- Story 2.6 enriches with detailed merge instructions -->
...
__EXISTING_TASKS__
{{- end}}
```

**Story 2.6 changes — before/after diff:**

BEFORE (line 159):
```
- Preserve the existing task order — append new tasks at the end of the relevant epic section.
```
AFTER:
```
- PRESERVE original task order. New tasks insert at logical position within the relevant epic section. NEVER reorder existing tasks.
```

BEFORE (line 163):
```
<!-- Story 2.6 enriches with detailed merge instructions -->
```
AFTER (replace comment with new bullet points):
```
- MUST NOT change [x] status of completed tasks — this is the most critical merge requirement.
- Modified tasks: if a task description changed in the updated story, update the description text but PRESERVE the completion status ([x] or [ ]).
```

Do NOT restructure or remove the other existing bullet points (Preserve ALL existing tasks, Preserve ALL source: fields, epic section handling, deduplication).

### Two-Stage Prompt Assembly Reminder

Stage 1 (text/template) processes `{{if .HasExistingTasks}}` — safe because it's a bool from code.
Stage 2 (strings.Replace) injects `__EXISTING_TASKS__` — safe because template engine doesn't re-process.

The existing content from sprint-tasks.md may contain `{{` or other template-like syntax. Since it's injected via Stage 2, this is safe. Do NOT inject existing content via a template variable.

### Test Fixtures: Realistic sprint-tasks.md Format

ALL existing sprint-tasks.md fixtures MUST follow the format from `config/shared/sprint-tasks-format.md`:
- Header: `## Epic: <Name>`
- Tasks: `- [ ] Task description` or `- [x] Completed task description`
- Source: `  source: stories/<file>.md#<identifier>` (indented 2 spaces, on next line)
- Service tasks: `[SETUP]`, `[VERIFY]`, `[E2E]` prefixes
- Gates: `[GATE]` suffix

**Example `existing_completed.md` fixture:**
```markdown
## Epic: Authentication

- [x] Implement user login endpoint
  source: stories/auth.md#AC-1
- [x] Add input validation for login
  source: stories/auth.md#AC-2
- [ ] Return JWT token on success
  source: stories/auth.md#AC-3
- [ ] Add rate limiting middleware
  source: stories/auth.md#AC-4
- [ ] Log failed login attempts
  source: stories/auth.md#AC-5
```
2 completed `[x]` + 3 open `[ ]`, all with valid source: fields.

**Pre-commit check for fixtures:** Every `- [ ]` or `- [x]` line MUST have a matching `source:` line below it (validate manually or with `validateMergeTaskSourcePairs`).

### Expected Task Counts per Merge Fixture (CRITICAL)

`Run()` returns ONLY open task count (via `TaskOpenRegex`). `wantDoneCount` is verified separately in extraCheck via `TaskDoneRegex`.

| Fixture | wantTaskCount (open `[ ]`) | wantDoneCount (`[x]`) | Total tasks |
|---------|---------------------------|-----------------------|-------------|
| mock_merge_completed.md | 3 | 2 | 5 |
| mock_merge_description.md | 3 | 2 | 5 |
| mock_merge_gate.md | 3 | 2 | 5 |

Note: exact counts depend on fixture content. Dev agent must match these when creating fixtures. All fixtures should have 2 preserved `[x]` tasks + 3 open `[ ]` tasks for consistency across scenarios.

### Mock Output Fixtures for Merge

Mock Claude returns a COMPLETE sprint-tasks.md, not a diff. The merged output includes ALL tasks (old + new). The dev agent creates fixtures where:
- `[x]` tasks from existing file appear with same `[x]` status
- New tasks appear with `[ ]` status
- Modified task descriptions are updated but `[x]` status preserved
- Task order matches existing order + new tasks at logical positions

### validateTaskSourcePairs Limitation for Merge

The existing `validateTaskSourcePairs` helper only counts `config.TaskOpenRegex` (`- [ ]`) matches. For merge tests with `[x]` tasks, also count `config.TaskDoneRegex` matches separately:
```go
doneCount := 0
for _, line := range strings.Split(output, "\n") {
    if config.TaskDoneRegex.MatchString(line) {
        doneCount++
    }
}
```

Do NOT modify `validateTaskSourcePairs` — it's used by all existing tests. Create a new `validateMergeTaskSourcePairs(t, output) (open, done int)` helper that checks source: for BOTH `TaskOpenRegex` AND `TaskDoneRegex` lines:

```go
func validateMergeTaskSourcePairs(t *testing.T, output string) (int, int) {
    t.Helper()
    lines := strings.Split(output, "\n")
    openCount, doneCount := 0, 0
    for i, line := range lines {
        isOpen := config.TaskOpenRegex.MatchString(line)
        isDone := config.TaskDoneRegex.MatchString(line)
        if isOpen || isDone {
            if isOpen {
                openCount++
            } else {
                doneCount++
            }
            if i+1 >= len(lines) || !config.SourceFieldRegex.MatchString(lines[i+1]) {
                t.Errorf("task at line %d has no valid source field on next line", i+1)
            }
        }
    }
    if openCount+doneCount == 0 {
        t.Errorf("merge output has no parseable tasks")
    }
    return openCount, doneCount
}
```

Use `validateMergeTaskSourcePairs` in ALL merge golden file tests instead of `validateTaskSourcePairs`. This ensures `[x]` tasks also have valid source: fields per format contract.

### Backward Compatibility: TestRun_WriteFailure

`TestRun_WriteFailure` creates a directory at `sprint-tasks.md` path to make `os.WriteFile` fail. With the `os.Stat + IsDir()` guard, this directory is treated as "not existing" → create mode → WriteFile fails on directory → same error as before. No changes to this test needed.

### Review Learnings Applied Proactively

**From Story 2.5:**
- Golden file tests use table-driven structure with `extraCheck func(t *testing.T, output string)` for per-scenario assertions
- `validateTaskSourcePairs` reused for regex validation across all tests
- Count assertions (`strings.Count >= N`), not just existence checks
- ExtraCheck verifies ALL scenario-specific markers individually

**From Story 2.3:**
- `copyFixtureToScenario(t, name)` for fixture injection into mock scenario dir
- Full output comparison when mock returns deterministic content
- Separator assertions use full `"\n\n---\n\n"` pattern

**From Story 2.2:**
- Don't add template conditionals not mandated by AC — merge block already exists, only enhance instructions
- Test assertion symmetry across merge scenarios

**From CLAUDE.md testing rules:**
- Test naming: `TestRun_MergeGoldenFiles`, `TestRun_MergeSessionFailure`, etc.
- Error tests verify message content with `strings.Contains`
- Every exported function error path needs dedicated test

### What NOT to Do

- Do NOT add a CLI flag for merge mode — auto-detection is the design
- Do NOT modify `config.TemplateData` struct — `HasExistingTasks` already exists
- Do NOT modify `config/prompt.go` or `config/constants.go`
- Do NOT add `[x]` tasks to existing CREATE-mode golden files
- Do NOT delete .bak on success (AC doesn't require it, keep it simple)
- Do NOT add merge-specific success message to `cmd/ralph/bridge.go` (out of scope)
- Do NOT implement Go-level structural merge logic — merge is prompt-driven
- Do NOT add new dependencies

### Project Structure Notes

- `bridge/bridge.go` — modified (add merge detection, backup, data passing)
- `bridge/prompts/bridge.md` — modified (enhance merge instructions)
- `bridge/bridge_test.go` — modified (add 5 test functions: TestRun_MergeGoldenFiles, TestRun_MergeSessionFailure, TestRun_MergeBackupFailure, TestRun_MergeParseError, TestRun_MergeReadExistingFailure + validateMergeTaskSourcePairs helper)
- `bridge/prompt_test.go` — modified (add 3 assertions for AC #2 merge instructions in TestBridgePrompt_Merge)
- `bridge/testdata/existing_completed.md` — new (existing sprint-tasks.md with [x] tasks)
- `bridge/testdata/existing_no_gate.md` — new (existing sprint-tasks.md without [GATE])
- `bridge/testdata/existing_changed_desc.md` — new (existing sprint-tasks.md for description change)
- `bridge/testdata/mock_merge_completed.md` — new (mock output: preserved [x] + new tasks)
- `bridge/testdata/mock_merge_description.md` — new (mock output: updated description, [x] preserved)
- `bridge/testdata/mock_merge_gate.md` — new (mock output: [GATE] added, [x] preserved)
- `bridge/testdata/TestBridge_MergeWithCompleted.golden` — new (generated via -update)
- `bridge/testdata/TestBridge_MergeDescriptionChange.golden` — new (generated via -update)
- `bridge/testdata/TestBridge_MergeAddGate.golden` — new (generated via -update)
- `bridge/testdata/TestBridgePrompt_Creation.golden` — modified (regenerated after prompt changes)
- `bridge/testdata/TestBridgePrompt_Merge.golden` — modified (regenerated after prompt changes)

### References

- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md — Story 2.6 AC, lines 253-303]
- [Source: docs/project-context.md — Two-stage prompt assembly, bridge role, config immutability]
- [Source: bridge/bridge.go — Current Run() implementation, sprintTasksFile constant, atomic write pattern]
- [Source: bridge/prompts/bridge.md — Existing merge mode conditional block, lines 152-168]
- [Source: config/prompt.go — AssemblePrompt function, TemplateData struct with HasExistingTasks]
- [Source: config/constants.go — TaskOpenRegex, TaskDoneRegex, SourceFieldRegex, GateTagRegex]
- [Source: config/shared/sprint-tasks-format.md — Format contract defining valid sprint-tasks.md]
- [Source: bridge/bridge_test.go — Existing test patterns, copyFixtureToScenario, validateTaskSourcePairs]
- [Source: bridge/prompt_test.go — goldenTest helper, TestMain dispatch]
- [Source: internal/testutil/mock_claude.go — SetupMockClaude, ScenarioStep, RunMockClaude]
- [Source: docs/sprint-artifacts/2-5-bridge-golden-file-tests.md — Previous story dev notes, fixture patterns]
- [Source: docs/sprint-artifacts/2-3-bridge-logic-core-conversion.md — copyFixtureToScenario, error wrapping]

### Existing Code to Build On

| File | Status | Description |
|------|--------|-------------|
| `bridge/bridge.go` | modify | Add merge detection after story read, before prompt assembly |
| `bridge/prompts/bridge.md` | modify | Enhance merge instructions (replace placeholder comment) |
| `bridge/bridge_test.go` | modify | Add 4 merge test functions, reuse existing helpers |
| `bridge/prompt_test.go` | read-only | goldenTest helper, TestMain dispatch |
| `config/prompt.go` | read-only | AssemblePrompt, TemplateData (HasExistingTasks already exists) |
| `config/constants.go` | read-only | TaskOpenRegex, TaskDoneRegex, SourceFieldRegex, GateTagRegex |
| `config/shared/sprint-tasks-format.md` | read-only | Format contract for fixture validation |
| `internal/testutil/mock_claude.go` | read-only | SetupMockClaude, ScenarioStep, RunMockClaude |
| `bridge/testdata/mock_bridge_output.md` | read-only | Existing fixture pattern reference |

## Dev Agent Record

### Context Reference

<!-- Story created by create-story workflow with full artifact analysis -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: Enhanced bridge.md merge instructions — replaced placeholder comment with 3 AC-mandated instructions (MUST NOT change [x], PRESERVE task order, Modified tasks). Added 3 assertions to TestBridgePrompt_Merge. Golden files regenerated.
- Task 2: Added merge detection (os.Stat + IsDir guard), backup creation (raw bytes for byte-for-byte fidelity), and merge-aware AssemblePrompt call. Only 2 lines changed in existing code (false→mergeMode, ""→existingContent). No new imports needed.
- Task 3: Created 6 test fixtures (3 existing + 3 mock merge output). All validated for task/source pairs. CRLF fixed.
- Task 4: Added TestRun_MergeGoldenFiles with 3 subcases (MergeWithCompleted, MergeDescriptionChange, MergeAddGate). Added validateMergeTaskSourcePairs helper for [x] task validation. All golden files generated and verified.
- Task 5: Added 3 merge error path tests (MergeSessionFailure, MergeBackupFailure, MergeParseError). All verify backup integrity, original file unchanged, and correct error wrapping.
- Task 5b: Added TestRun_MergeReadExistingFailure — t.Skip on WSL/NTFS (symlink privileges unavailable). Accepted coverage gap documented per Dev Notes.
- Task 6: All bridge tests pass (19 PASS + 1 SKIP), config tests pass, go vet clean. TestRun_WriteFailure backward compatibility verified.

### File List

**Modified files:**
- bridge/bridge.go — added merge detection (os.Stat+IsDir), backup creation, merge-aware AssemblePrompt
- bridge/prompts/bridge.md — replaced placeholder comment with 3 AC-mandated merge instructions
- bridge/bridge_test.go — added 5 test functions + validateMergeTaskSourcePairs helper (~200 lines)
- bridge/prompt_test.go — added 3 AC #2 merge instruction assertions to TestBridgePrompt_Merge
- bridge/testdata/TestBridgePrompt_Creation.golden — regenerated (prompt content changed)
- bridge/testdata/TestBridgePrompt_Merge.golden — regenerated (merge instructions enhanced)
- docs/sprint-artifacts/sprint-status.yaml — story status updated

**New files (test fixtures):**
- bridge/testdata/existing_completed.md — existing sprint-tasks.md with 2 [x] + 3 [ ] tasks
- bridge/testdata/existing_no_gate.md — existing sprint-tasks.md with 2 [x] + 1 [ ], no [GATE]
- bridge/testdata/existing_changed_desc.md — existing sprint-tasks.md with outdated descriptions
- bridge/testdata/mock_merge_completed.md — mock output: 2 [x] preserved + 3 new [ ] tasks
- bridge/testdata/mock_merge_description.md — mock output: updated descriptions, [x] preserved
- bridge/testdata/mock_merge_gate.md — mock output: [GATE] added, [x] preserved

**New files (golden files, generated via -update):**
- bridge/testdata/TestBridge_MergeWithCompleted.golden
- bridge/testdata/TestBridge_MergeDescriptionChange.golden
- bridge/testdata/TestBridge_MergeAddGate.golden

### Change Log

- 2026-02-26: Implemented smart merge — merge detection, backup, prompt enhancement, 5 test functions + 9 fixtures (Date: 2026-02-26)
- 2026-02-26: Code review fixes — grouped const block (M1), removed blank line in merge instructions (L1), improved MergeReadExistingFailure test docs + t.Skipf (M2)
