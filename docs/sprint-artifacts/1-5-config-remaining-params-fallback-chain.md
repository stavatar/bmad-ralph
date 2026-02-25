# Story 1.5: Config Remaining Params + Fallback Chain

Status: done

## Story

As a developer,
I want custom prompt file fallback chain (project → global → embedded) working,
so that users can customize agent prompts at project or global level. (FR32, FR33, FR34)

## Acceptance Criteria

```gherkin
Given custom prompt file exists at .ralph/agents/quality.md (project level)
When prompt fallback chain resolves "agents/quality.md"
Then project-level file content is returned
And source is reported as "project"

Given no project-level file exists
And custom prompt file exists at ~/.config/ralph/agents/quality.md (global level)
When prompt fallback chain resolves "agents/quality.md"
Then global-level file content is returned
And source is reported as "global"

Given neither project nor global file exists
When prompt fallback chain resolves "agents/quality.md" with embedded fallback content
Then embedded default content is returned
And source is reported as "embedded"

Given neither project nor global file exists
And no embedded content provided (nil/empty)
When prompt fallback chain resolves "agents/quality.md"
Then a descriptive error is returned

And config.ResolvePath method signature:
  func (c *Config) ResolvePath(name string, embedded []byte) (content []byte, source string, err error)
  DEVIATION FROM EPIC: Epic defines ResolvePath(name string) without embedded parameter.
  Added because go:embed only works for files in same/child directories — config package
  cannot embed files from runner/prompts/ or bridge/prompts/. Caller passes its own embedded
  content, keeping config as leaf package.
  - name: relative path within override hierarchy (e.g., "agents/quality.md", "prompts/execute.md")
  - embedded: fallback content from caller's go:embed (may be nil)
  - Returns: file content, source descriptor ("project"/"global"/"embedded"), error

And per-agent model configuration is supported (FR34):
  | Config Key     | Description                     |
  | model_execute  | Model for execute sessions      |
  | model_review   | Model for review sub-agents     |
And model config maps to --model flag in session invocation
And per-agent model resolution works through existing three-level cascade (CLI > config > embedded)
NOTE: Per-agent model fields (ModelExecute/ModelReview) are already implemented and tested
in Stories 1.3/1.4. No new implementation or tests needed — existing coverage satisfies FR34:
  - TestConfig_Load_ValidFullConfig: verifies config file values
  - TestConfig_Load_CascadeThreeLevels: verifies cascade for ModelExecute
  - TestConfig_Load_AllCLIFlagsOverrideFullConfig: verifies CLI override of both fields
  - TestConfig_Load_DefaultsComplete: verifies embedded defaults

And table-driven tests cover:
  - Project-level override found
  - Global-level fallback
  - Embedded default fallback
  - Project takes priority over global when both exist
  - No file + no embedded → error
  - Empty embedded ([]byte{}) → error (same as nil)
  - Subdirectory name resolution ("agents/quality.md")
  - Global dir not accessible (os.UserHomeDir fails) → skip global, use embedded
  - Unreadable project file (e.g., directory) → falls through to global
```

## Tasks / Subtasks

- [x] Task 1: Implement ResolvePath method on Config (AC: fallback chain resolution)
  - [x] 1.1 Add `ResolvePath(name string, embedded []byte) ([]byte, string, error)` method to `*Config` in `config/config.go`
  - [x] 1.2 Fallback step 1: Check `filepath.Join(c.ProjectRoot, ".ralph", name)` via `os.ReadFile`. If readable, return `(data, "project", nil)`
  - [x] 1.3 Fallback step 2: Call `os.UserHomeDir()`. If success, check `filepath.Join(home, ".config", "ralph", name)` via `os.ReadFile`. If readable, return `(data, "global", nil)`. If `UserHomeDir` fails, skip global level entirely (no error — graceful degradation)
  - [x] 1.4 Fallback step 3: If `len(embedded) > 0`, return `(embedded, "embedded", nil)`
  - [x] 1.5 If no fallback succeeded: return `(nil, "", fmt.Errorf("config: resolve %q: not found in project, global, or embedded", name))`
  - [x] 1.6 Run `sed -i 's/\r$//' config/config.go` after editing (CRLF fix)

- [x] Task 2: Write table-driven ResolvePath tests (AC: all fallback scenarios)
  - [x] 2.1 Add `writeFile(t, path, content)` test helper for creating files at arbitrary paths
  - [x] 2.2 Create single table-driven `TestConfig_ResolvePath` with the following subtests:
    - "project level found" — project file exists, return content with source "project"
    - "global level fallback" — no project file, global exists, return with source "global"
    - "embedded fallback" — neither file exists, embedded provided, return with source "embedded"
    - "project priority over global" — both exist, project content returned
    - "no file no embedded error" — neither exists, embedded nil → descriptive error
    - "empty embedded error" — neither exists, embedded `[]byte{}` → error (same as nil)
    - "subdirectory name" — name "agents/quality.md" resolves correct paths
    - "UserHomeDir failure skips global" — HOME="" causes UserHomeDir error → uses embedded
    - "unreadable project falls through to global" — project path is directory, global file exists → returns global
  - [x] 2.3 Test structure: each case creates `*Config{ProjectRoot: projectDir}`, sets up files via `writeFile`, calls `ResolvePath`, asserts content/source/error
  - [x] 2.4 For global tests: use `t.Setenv("HOME", homeDir)` + `t.Setenv("USERPROFILE", homeDir)` to control `os.UserHomeDir()` cross-platform
  - [x] 2.5 For UserHomeDir failure: clear HOME, USERPROFILE, HOMEDRIVE, HOMEPATH — causes `os.UserHomeDir()` error on both Linux and Windows
  - [x] 2.6 For unreadable project test: create project path as directory (`os.MkdirAll`) so `os.ReadFile` fails with non-NotExist error
  - [x] 2.7 Run `sed -i 's/\r$//' config/config_test.go` after editing (CRLF fix)

- [x] Task 3: Validation (AC: all tests pass, config remains leaf)
  - [x] 3.1 `go build ./...` passes
  - [x] 3.2 `go test ./config/...` passes with ALL tests green (existing 33 + new 9 ResolvePath subtests = 42 total)
  - [x] 3.3 `go vet ./...` passes
  - [x] 3.4 Verify `config` remains leaf package (no new project imports)
  - [x] 3.5 Verify no new external dependencies added
  - [x] 3.6 Verify Config immutability: ResolvePath is read-only method (does not modify *Config)

## Dev Notes

### Scope Clarification

Despite the title referencing "Remaining Params", all 16 Config struct parameters are already implemented (Stories 1.3/1.4). This story focuses solely on:
1. **ResolvePath method** — three-level fallback chain for prompt/agent file resolution
2. **Tests** for ResolvePath

Per-agent model (FR34) is already satisfied by existing ModelExecute/ModelReview fields and cascade tests. No new implementation or tests needed for that.

### Implementation Guide

**This story modifies `config/config.go` and `config/config_test.go` (existing files from Stories 1.3/1.4). NO new files created.**

Required stdlib functions (`os.ReadFile`, `os.UserHomeDir`, `filepath.Join`) are already imported in config/config.go from Story 1.3. No import changes needed.

### ResolvePath Implementation

```go
// ResolvePath resolves a file through the three-level fallback chain:
//   1. Project-level: <ProjectRoot>/.ralph/<name>
//   2. Global-level: ~/.config/ralph/<name>
//   3. Embedded: provided fallback content
//
// Returns the file content, a source description ("project", "global", or "embedded"),
// and any error. If no level provides content, returns a descriptive error.
func (c *Config) ResolvePath(name string, embedded []byte) ([]byte, string, error) {
	// 1. Project-level override
	projectPath := filepath.Join(c.ProjectRoot, ".ralph", name)
	if data, err := os.ReadFile(projectPath); err == nil {
		return data, "project", nil
	}

	// 2. Global-level override
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".config", "ralph", name)
		if data, err := os.ReadFile(globalPath); err == nil {
			return data, "global", nil
		}
	}

	// 3. Embedded fallback
	if len(embedded) > 0 {
		return embedded, "embedded", nil
	}

	return nil, "", fmt.Errorf("config: resolve %q: not found in project, global, or embedded", name)
}
```

**Design decisions:**
- **Method on `*Config`** (not free function): needs `ProjectRoot`, cleaner API (`cfg.ResolvePath("agents/quality.md", embeddedContent)`)
- **`embedded []byte` parameter:** config package CANNOT embed files from `runner/prompts/` (go:embed limitation — same or child directories only). Caller passes its own embedded content. Keeps config as leaf package
- **`os.UserHomeDir()` failure:** graceful skip — global level is optional. Some environments (containers, CI) may not have HOME set
- **Read errors on files:** treated as "not found" — if a file exists but is unreadable, fallback continues to next level. Matches "override" semantics
- **Empty embedded (`[]byte{}`):** treated same as nil — prevents returning empty useless content
- **Global path `~/.config/ralph/`:** follows XDG Base Directory convention. Uses hardcoded `~/.config/ralph/` (not `$XDG_CONFIG_HOME`). Simplicity over full XDG compliance — acceptable for MVP
- **Path traversal:** `name` is caller-controlled (not user input in MVP). `filepath.Join` resolves `..` naturally. No validation needed in MVP

### Test Helper

```go
// writeFile creates a file at the given path with the specified content,
// creating parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
```

### Table-Driven Test Structure

```go
func TestConfig_ResolvePath(t *testing.T) {
	embedded := []byte("embedded content")

	tests := []struct {
		name        string
		setup       func(t *testing.T, projectDir, homeDir string)
		setHome     string // override HOME env var ("" = use homeDir, "EMPTY" = empty string)
		embedded    []byte
		resolveName string
		wantContent string
		wantSource  string
		wantErr     bool
	}{
		{
			name: "project level found",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(projectDir, ".ralph", "agents", "quality.md"), "project content")
			},
			resolveName: "agents/quality.md",
			wantContent: "project content",
			wantSource:  "project",
		},
		{
			name: "global level fallback",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global content")
			},
			resolveName: "agents/quality.md",
			wantContent: "global content",
			wantSource:  "global",
		},
		{
			name:        "embedded fallback",
			setup:       func(t *testing.T, projectDir, homeDir string) {},
			embedded:    embedded,
			resolveName: "agents/quality.md",
			wantContent: "embedded content",
			wantSource:  "embedded",
		},
		{
			name: "project priority over global",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(projectDir, ".ralph", "agents", "quality.md"), "project wins")
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global loses")
			},
			resolveName: "agents/quality.md",
			wantContent: "project wins",
			wantSource:  "project",
		},
		{
			name:        "no file no embedded error",
			setup:       func(t *testing.T, projectDir, homeDir string) {},
			resolveName: "agents/missing.md",
			wantErr:     true,
		},
		{
			name:        "empty embedded error",
			setup:       func(t *testing.T, projectDir, homeDir string) {},
			embedded:    []byte{},
			resolveName: "agents/quality.md",
			wantErr:     true,
		},
		{
			name: "UserHomeDir failure skips global uses embedded",
			setup: func(t *testing.T, projectDir, homeDir string) {
				// Global file exists but HOME is invalid so UserHomeDir fails
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global content")
			},
			setHome:     "EMPTY", // t.Setenv("HOME", "") → UserHomeDir error
			embedded:    embedded,
			resolveName: "agents/quality.md",
			wantContent: "embedded content",
			wantSource:  "embedded",
		},
		{
			name: "unreadable project falls through to global",
			setup: func(t *testing.T, projectDir, homeDir string) {
				// Create project path as DIRECTORY (os.ReadFile fails with non-NotExist)
				os.MkdirAll(filepath.Join(projectDir, ".ralph", "agents", "quality.md"), 0755)
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global content")
			},
			resolveName: "agents/quality.md",
			wantContent: "global content",
			wantSource:  "global",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			homeDir := t.TempDir()

			if tt.setHome == "EMPTY" {
				t.Setenv("HOME", "")
			} else {
				t.Setenv("HOME", homeDir)
			}

			tt.setup(t, projectDir, homeDir)

			cfg := &Config{ProjectRoot: projectDir}
			content, source, err := cfg.ResolvePath(tt.resolveName, tt.embedded)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("content = %q, want %q", string(content), tt.wantContent)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
		})
	}
}
```

**CRITICAL:** Use `t.Setenv` (Go 1.17+), NOT `os.Setenv`. `t.Setenv` automatically restores the original value after the test.

### Fallback Path Resolution Examples

For `cfg.ResolvePath("agents/quality.md", embeddedQualityMD)`:
1. Checks: `<ProjectRoot>/.ralph/agents/quality.md`
2. Checks: `~/.config/ralph/agents/quality.md`
3. Returns: `embeddedQualityMD` content

Consumer pattern (in future runner code):
```go
// Runner loads review agent prompt with fallback
content, source, err := cfg.ResolvePath("agents/quality.md", embeddedQualityMD)
```

### Project Structure Notes

- `config` remains leaf package (no imports of other project packages)
- Only modification: `config/config.go` (add ResolvePath method ~15 lines)
- Tests: `config/config_test.go` (add writeFile helper + table-driven TestConfig_ResolvePath)
- No changes to `config/errors.go`, `config/errors_test.go`, `config/defaults.yaml`
- No changes outside `config/` package
- No new files created

### Previous Story Intelligence (Story 1.4)

**Learnings from Story 1.4 implementation and review:**
- `t.Chdir(dir)` pattern works for Load() tests but NOT needed for ResolvePath (uses `c.ProjectRoot`, not CWD)
- `writeConfigYAML` helper exists — not needed for ResolvePath (creates config.yaml, not arbitrary files)
- intPtr/boolPtr/strPtr helpers available if needed
- CRLF fix required after creating files with Write tool: `sed -i 's/\r$//'`
- Test naming: `Test<Type>_<Method>_<Scenario>` — strictly enforced
- **Recurring issue from Stories 1.3/1.4:** Don't create standalone tests that are subsets of table-driven tests. All ResolvePath scenarios MUST be in single table-driven TestConfig_ResolvePath

**Code review findings from Story 1.4:**
- M1/M2: Removed 3 duplicate standalone tests — prevented in this story by table-driven-only approach
- M3: Added comprehensive all-fields test — good pattern
- L2: Clean up orphan files from workflow

### Git Intelligence

**Recent commits (5 total):**
- `8d8df51` — Story 1.4: CLI override, go:embed defaults, three-level config cascade
- `bfa30c2` — Story 1.3: Config struct, YAML parsing, project root detection
- `dccde3b` — Stories 1.1 + 1.2: scaffold + error types

**Patterns:** Single commit per story. Table-driven tests with `t.Run`. 33 tests currently passing. Go binary at `"/mnt/c/Program Files/Go/bin/go.exe"`.

### Architecture Compliance Checklist

- [ ] `config` remains leaf package (no imports of other project packages)
- [ ] ResolvePath is a read-only method on `*Config` (no mutation)
- [ ] ResolvePath uses `os.ReadFile` for file access (architecture pattern)
- [ ] Error wrapping follows `fmt.Errorf("config: resolve %q: ...", name)` pattern
- [ ] No `os.Exit` calls in config package
- [ ] No logging/printing from config package
- [ ] Existing 33 tests still pass with no changes
- [ ] Only uses Go stdlib (`os`, `path/filepath`). No new dependencies or imports
- [ ] Test naming: `TestConfig_ResolvePath` (single table-driven function)
- [ ] Tests use `t.TempDir()` for isolation
- [ ] Tests use `t.Setenv` (not `os.Setenv`) for HOME override
- [ ] No standalone tests duplicating table cases (recurring review finding)

### Anti-Patterns (FORBIDDEN)

- Creating standalone test functions for individual ResolvePath scenarios (MUST be single table-driven test)
- Creating a separate `config/resolve.go` file for ~15 lines (over-engineering)
- Using `os.Stat` + `os.ReadFile` (double syscall — just `os.ReadFile` and check error)
- Embedding files from `runner/prompts/` in config package (go:embed can't cross package boundary)
- Making ResolvePath a free function (method on *Config is cleaner)
- Failing loudly when `os.UserHomeDir()` returns error (skip global gracefully)
- Using `os.Setenv` in tests instead of `t.Setenv` (test pollution)
- Adding validation for file content (not this story's concern)
- Creating new per-agent model tests (already fully covered by existing cascade tests)
- Mutating `*Config` in ResolvePath
- Adding new external dependencies
- `os.Exit` or logging/printing from config package

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.5]
- [Source: docs/prd/functional-requirements.md#FR32]
- [Source: docs/prd/functional-requirements.md#FR33]
- [Source: docs/prd/functional-requirements.md#FR34]
- [Source: docs/project-context.md#Architecture -- Key Decisions]
- [Source: docs/project-context.md#Config Immutability]
- [Source: docs/architecture/core-architectural-decisions.md#External Dependencies]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Structural Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Error Handling Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#File I/O Patterns]
- [Source: docs/architecture/project-structure-boundaries.md#Complete Project Directory Structure]
- [Source: docs/architecture/project-structure-boundaries.md#Runtime .ralph/ Structure]
- [Source: docs/sprint-artifacts/1-4-config-cli-override-go-embed-defaults.md#Completion Notes]
- [Source: docs/sprint-artifacts/1-4-config-cli-override-go-embed-defaults.md#Code Review Fixes Applied]

## Senior Developer Review (AI)

**Review Date:** 2026-02-25
**Review Outcome:** Changes Requested
**Reviewer Model:** Claude Opus 4.6

### Action Items

- [x] [M1] Error test cases lack message content verification — added `wantErrContain` field and `strings.Contains` checks
- [x] [M2] Missing test case: UserHomeDir failure + no embedded → error — added "UserHomeDir failure no embedded error" subtest
- [x] [M3] Story File List mislabels story file as "modified" (actually new) — fixed to "new"
- [x] [L1] "subdirectory name" test redundant — replaced with "flat name without subdirectory" testing `"quality.md"`
- [x] [L2] Completion Notes test count ambiguous — clarified wording

### Summary

3 Medium, 2 Low findings. All fixed and verified. Implementation is correct — issues were in test coverage gaps and documentation accuracy.

## Change Log

- 2026-02-25: Implemented ResolvePath three-level fallback chain (project → global → embedded) with 9 table-driven test cases. Cross-platform HOME env handling for Windows Go.
- 2026-02-25: Code review fixes — added error message verification, missing UserHomeDir+no-embedded error test, flat name test, fixed File List labeling.

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- Initial test run: 2 failures (global_level_fallback, unreadable_project_falls_through_to_global) — caused by Windows Go using `USERPROFILE` instead of `HOME` for `os.UserHomeDir()`. Fixed by setting both `HOME` and `USERPROFILE` in tests, and clearing `HOMEDRIVE`/`HOMEPATH` for failure case.

### Completion Notes List

- Implemented `ResolvePath` method on `*Config` (~15 lines) — three-level fallback chain: project (.ralph/) → global (~/.config/ralph/) → embedded
- Added `writeFile` test helper for creating arbitrary files with parent directories
- Created single table-driven `TestConfig_ResolvePath` with 10 subtests covering all AC scenarios + review additions
- Cross-platform fix: tests set both `HOME` (Linux) and `USERPROFILE` (Windows) env vars for `os.UserHomeDir()` compatibility
- All tests pass (33 existing top-level functions + 1 new with 10 subtests), zero regressions
- `go build`, `go vet` clean; `go mod tidy` shows no new dependencies
- config remains leaf package (no project imports)
- ResolvePath is read-only method (no `*Config` mutation)
- No new files created; only modified `config/config.go` and `config/config_test.go`

### File List

- config/config.go (modified — added ResolvePath method)
- config/config_test.go (modified — added writeFile helper + TestConfig_ResolvePath with 10 subtests)
- docs/sprint-artifacts/sprint-status.yaml (modified — story status update)
- docs/sprint-artifacts/1-5-config-remaining-params-fallback-chain.md (new — story file with task checkboxes, status, dev agent record)
