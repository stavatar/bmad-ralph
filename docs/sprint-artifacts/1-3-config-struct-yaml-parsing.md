# Story 1.3: Config Struct + YAML Parsing

Status: Done

## Story

As a developer,
I want config loaded from `.ralph/config.yaml` with strongly-typed struct,
so that all parameters are available as typed Go values. (FR30)

## Acceptance Criteria

```gherkin
Given a .ralph/config.yaml file exists in the project root
When config.Load() is called
Then Config struct is populated with all 16 parameters:
  | Parameter             | Type   | Default            |
  | claude_command        | string | "claude"           |
  | max_turns             | int    | 50                 |
  | max_iterations        | int    | 3                  |
  | max_review_iterations | int    | 3                  |
  | gates_enabled         | bool   | false              |
  | gates_checkpoint      | int    | 0                  |
  | review_every          | int    | 1                  |
  | model_execute         | string | ""                 |
  | model_review          | string | ""                 |
  | review_min_severity   | string | "LOW"              |
  | always_extract        | bool   | false              |
  | serena_enabled        | bool   | true               |
  | serena_timeout        | int    | 10                 |
  | learnings_budget      | int    | 200                |
  | log_dir               | string | ".ralph/logs"      |
  | project_root          | string | (auto-detected)    |

And YAML field tags MUST match PRD config table names exactly (snake_case):
  e.g. MaxTurns -> yaml:"max_turns", NOT yaml:"maxTurns"

And missing config file results in all defaults applied (no error)
And malformed YAML returns descriptive error with line number
And unknown fields are silently ignored (forward compatibility)
And Config struct is immutable after Load() (passed by pointer, never mutated)

And project root auto-detection follows priority:
  - Walk up from CWD looking for .ralph/ directory
  - If .ralph/ not found, fall back to .git/ directory
  - If neither found, use CWD with warning
  - .ralph/ takes priority over .git/ if both exist at different levels

And table-driven tests cover:
  - Valid config with all fields
  - Partial config (missing fields -> defaults)
  - Empty file (all defaults)
  - Malformed YAML (error case)
  - Unknown fields (ignored)
  - Project root: .ralph/ found
  - Project root: only .git/ found (fallback)
  - Project root: neither found (CWD + warning)
```

## Tasks / Subtasks

- [x] Task 1: Define Config struct with all 16 parameters (AC: Config struct populated)
  - [x] 1.1 Create `Config` struct in `config/config.go` with 16 fields and yaml tags
  - [x] 1.2 Define `CLIFlags` struct as empty placeholder (fields added in Story 1.4)
  - [x] 1.3 Create unexported `defaultConfig()` returning `*Config` with all defaults pre-set
- [x] Task 2: Implement project root auto-detection (AC: project root auto-detection)
  - [x] 2.1 Create unexported `detectProjectRoot()` with two-pass walk-up algorithm
  - [x] 2.2 Pass 1: walk up from CWD looking for `.ralph/` directory
  - [x] 2.3 Pass 2: walk up from CWD looking for `.git/` directory (fallback)
  - [x] 2.4 Pass 3: return CWD if neither found (caller handles warning)
- [x] Task 3: Implement config.Load function (AC: YAML parsing, defaults, missing file, malformed YAML, unknown fields)
  - [x] 3.1 Define `Load(flags CLIFlags) (*Config, error)` as exported entry point
  - [x] 3.2 Call `defaultConfig()` then `detectProjectRoot()` for project root
  - [x] 3.3 Build config path: `filepath.Join(root, ".ralph", "config.yaml")`
  - [x] 3.4 Read file with `os.ReadFile` — on `os.ErrNotExist` return defaults (no error)
  - [x] 3.5 Guard against empty YAML document `---` (yaml.v3 zeros struct on `---` — GitHub #395): `bytes.TrimSpace` check before Unmarshal
  - [x] 3.6 Parse YAML with `yaml.Unmarshal(data, cfg)` — overwrites only present fields, defaults remain for absent fields
  - [x] 3.7 Return descriptive error on malformed YAML (yaml.v3 includes line/column automatically)
  - [x] 3.8 Unknown YAML fields silently ignored (yaml.v3 default behavior — do NOT set KnownFields)
- [x] Task 4: Write table-driven tests for Config.Load (AC: all test scenarios)
  - [x] 4.1 Create `config/config_test.go` with `writeConfigYAML` test helper
  - [x] 4.2 Test: valid config with ALL 16 fields explicitly set — verify each field parsed correctly
  - [x] 4.3 Test: partial config (3-4 fields set) — unset fields have defaults, including `SerenaEnabled == true`
  - [x] 4.4 Test: empty YAML file (0 bytes) — all fields have defaults
  - [x] 4.5 Test: YAML with only `---` marker — all defaults preserved (yaml.v3 zeros struct on `---`, guard must catch)
  - [x] 4.6 Test: missing config file (no `.ralph/config.yaml`) — all defaults, no error
  - [x] 4.7 Test: malformed YAML (syntax error) — descriptive error returned
  - [x] 4.8 Test: unknown fields in YAML — silently ignored, known fields parsed correctly
  - [x] 4.9 Test: bool fields explicit false vs absent (gates_enabled: false vs not set, SerenaEnabled absent = true)
  - [x] 4.10 Test: ALL defaults complete — verify every non-obvious default explicitly (SerenaEnabled=true, LearningsBudget=200, ReviewMinSeverity="LOW")
  - [x] 4.11 Use `t.Chdir(dir)` (Go 1.24+) for safe CWD manipulation in Load tests
- [x] Task 5: Write table-driven tests for project root detection (AC: project root tests)
  - [x] 5.1 Test: `.ralph/` directory exists in CWD — returns CWD
  - [x] 5.2 Test: `.ralph/` in parent directory — returns parent
  - [x] 5.3 Test: only `.git/` found (no `.ralph/`) — returns dir with `.git/`
  - [x] 5.4 Test: neither `.ralph/` nor `.git/` found — returns CWD
  - [x] 5.5 Test: `.ralph/` in grandparent, `.git/` in parent — returns grandparent (`.ralph/` priority)
  - [x] 5.6 All tests use `t.TempDir()` for isolation
- [x] Task 6: Validation
  - [x] 6.1 `go build ./...` passes
  - [x] 6.2 `go test ./config/...` passes with all tests green
  - [x] 6.3 `go vet ./...` passes
  - [x] 6.4 Verify Config immutability: no mutation of *Config after Load returns

## Dev Notes

### Config Struct Definition

Create in `config/config.go` (currently just `package config` — add to same file):

```go
package config

import (
    "bytes"
    "errors"
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Config holds all ralph configuration parameters.
// Parsed once at startup, passed by pointer, NEVER mutated at runtime.
type Config struct {
    ClaudeCommand       string `yaml:"claude_command"`
    MaxTurns            int    `yaml:"max_turns"`
    MaxIterations       int    `yaml:"max_iterations"`
    MaxReviewIterations int    `yaml:"max_review_iterations"`
    GatesEnabled        bool   `yaml:"gates_enabled"`
    GatesCheckpoint     int    `yaml:"gates_checkpoint"`
    ReviewEvery         int    `yaml:"review_every"`
    ModelExecute        string `yaml:"model_execute"`
    ModelReview         string `yaml:"model_review"`
    ReviewMinSeverity   string `yaml:"review_min_severity"`
    AlwaysExtract       bool   `yaml:"always_extract"`
    SerenaEnabled       bool   `yaml:"serena_enabled"`
    SerenaTimeout       int    `yaml:"serena_timeout"`
    LearningsBudget     int    `yaml:"learnings_budget"`
    LogDir              string `yaml:"log_dir"`
    ProjectRoot         string `yaml:"-"` // auto-detected, NOT from YAML
}

// CLIFlags holds command-line flag values.
// Pointer fields for "was set" tracking added in Story 1.4.
type CLIFlags struct{}
```

**IMPORTANT:** This is reference code. The developer should use this as the implementation guide, not a blind copy-paste target.

### YAML Tag Rules (CRITICAL)

Every struct field MUST have a `yaml:"snake_case"` tag matching the config key names from the AC parameter table:
- `MaxTurns` -> `yaml:"max_turns"` (NOT `yaml:"maxTurns"`)
- `MaxReviewIterations` -> `yaml:"max_review_iterations"` (NOT `yaml:"reviewMaxIterations"`)
- `ProjectRoot` -> `yaml:"-"` (excluded from YAML — auto-detected)

### Default Values Pattern

Pre-initialize Config with defaults, then unmarshal YAML over it. yaml.v3 only overwrites fields present in YAML — absent fields keep their default values:

```go
func defaultConfig() *Config {
    return &Config{
        ClaudeCommand:       "claude",
        MaxTurns:            50,
        MaxIterations:       3,
        MaxReviewIterations: 3,
        GatesEnabled:        false,
        GatesCheckpoint:     0,
        ReviewEvery:         1,
        ModelExecute:        "",
        ModelReview:         "",
        ReviewMinSeverity:   "LOW",
        AlwaysExtract:       false,
        SerenaEnabled:       true,
        SerenaTimeout:       10,
        LearningsBudget:     200,
        LogDir:              ".ralph/logs",
    }
}
```

**Note on defaults vs PRD:** There are minor discrepancies between PRD and Epic AC defaults. Follow the **Epic AC table** (authoritative):
- `max_turns`: Epic=50, PRD table says 30 (Epic is authoritative)
- `review_min_severity`: Epic="LOW", PRD FR16a says "HIGH" (Epic is authoritative)

### Project Root Auto-Detection Algorithm

Two-pass walk-up from CWD. `.ralph/` always takes priority over `.git/` even at different directory levels.

Use a thin exported wrapper + testable unexported core to avoid CWD dependency in tests:

```go
// detectProjectRoot — production entry point (calls os.Getwd)
func detectProjectRoot() (string, error) {
    cwd, err := os.Getwd()
    if err != nil {
        return "", fmt.Errorf("config: getwd: %w", err)
    }
    return detectProjectRootFrom(cwd), nil
}

// detectProjectRootFrom — testable core (no CWD dependency)
func detectProjectRootFrom(start string) string {
    // Pass 1: Walk up looking for .ralph/ (highest priority)
    if root := walkUpFor(start, ".ralph"); root != "" {
        return root
    }
    // Pass 2: Walk up looking for .git/ (fallback)
    if root := walkUpFor(start, ".git"); root != "" {
        return root
    }
    // Pass 3: start dir as fallback (warning handled by caller)
    return start
}

func walkUpFor(start, target string) string {
    dir := start
    for {
        path := filepath.Join(dir, target)
        if info, err := os.Stat(path); err == nil && info.IsDir() {
            return dir
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return "" // filesystem root reached
        }
        dir = parent
    }
}
```

**Warning on CWD fallback:** Architecture says packages don't log. The "CWD with warning" AC is satisfied by returning CWD — the warning message is a CLI-layer concern (Story 1.13). Do NOT add fmt.Println or log calls inside config package.

### Load Function Structure

```go
func Load(flags CLIFlags) (*Config, error) {
    cfg := defaultConfig()

    root, err := detectProjectRoot()
    if err != nil {
        return nil, err
    }
    cfg.ProjectRoot = root

    configPath := filepath.Join(root, ".ralph", "config.yaml")
    data, err := os.ReadFile(configPath)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return cfg, nil // Missing file = all defaults, NOT an error
        }
        return nil, fmt.Errorf("config: read: %w", err)
    }

    // CRITICAL: yaml.v3 ZEROS the struct on empty document "---"
    // (GitHub issue #395). Guard against this to preserve defaults.
    trimmed := bytes.TrimSpace(data)
    if len(trimmed) == 0 || string(trimmed) == "---" {
        return cfg, nil // treat as empty file, keep defaults
    }

    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("config: parse yaml: %w", err)
    }

    // Story 1.4 will add: CLI flags override logic here

    return cfg, nil
}
```

**Key behaviors:**
- Missing file -> all defaults, nil error (AC requirement)
- Empty file (0 bytes) -> guard returns defaults (yaml.v3 would leave struct unchanged, but guard is cleaner)
- File with only `---` -> guard returns defaults (**CRITICAL**: yaml.v3 ZEROS the struct on `---` document — GitHub issue #395)
- Malformed YAML -> yaml.v3 returns error with line/column info automatically
- Unknown fields -> yaml.v3 ignores by default (do NOT call `KnownFields(true)`)
- `os.ErrNotExist` check via `errors.Is` (NOT string matching — project convention)
- yaml.v3 Unmarshal into pre-initialized struct: overwrites ONLY fields present in YAML, absent fields keep defaults (confirmed behavior)

### Testing Pattern (MANDATORY)

Follow established patterns from `config/errors_test.go`:
- Table-driven by default: `[]struct{name; ...}` + `t.Run`
- Test naming: `Test<Type>_<Method>_<Scenario>` (e.g., `TestConfig_Load_ValidFullConfig`)
- Go stdlib assertions: `if got != want { t.Errorf }` — NO testify
- `t.TempDir()` for all file-system tests

**Testing detectProjectRootFrom:** Call the unexported `detectProjectRootFrom(startDir)` directly with `t.TempDir()`. Create `.ralph/` or `.git/` subdirs as needed. No CWD manipulation required.

**Testing Load() — use `t.Chdir` (Go 1.24+, available in go 1.25):**

`testing.T.Chdir(dir)` safely changes CWD for a single test and auto-reverts on cleanup. This is the recommended approach for testing `Load()`:

```go
func TestConfig_Load_ValidFullConfig(t *testing.T) {
    dir := t.TempDir()
    // Create .ralph/config.yaml in temp dir
    ralphDir := filepath.Join(dir, ".ralph")
    os.MkdirAll(ralphDir, 0755)
    os.WriteFile(filepath.Join(ralphDir, "config.yaml"), []byte(yamlContent), 0644)

    t.Chdir(dir) // CWD = temp dir (auto-reverted after test)
    cfg, err := Load(CLIFlags{})
    // assertions...
}
```

**Fixture files:** YAML test fixtures can be stored in `config/testdata/` (directory exists from Story 1.1) for complex configs, or inlined as strings for simple cases. Use `testdata/` for golden files and reusable fixtures; inline strings for one-off test cases.

### Example Test Structure

```go
// --- Load tests: use t.Chdir for CWD control ---

func TestConfig_Load_ValidFullConfig(t *testing.T) {
    dir := t.TempDir()
    writeConfigYAML(t, dir, fullConfigYAML) // helper creates .ralph/config.yaml
    t.Chdir(dir)

    cfg, err := Load(CLIFlags{})
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if cfg.MaxTurns != 100 { t.Errorf("MaxTurns = %d, want 100", cfg.MaxTurns) }
    // ... verify all 16 fields
}

func TestConfig_Load_PartialConfig(t *testing.T) {
    tests := []struct {
        name    string
        yaml    string
        check   func(t *testing.T, cfg *Config)
    }{
        {
            "only max_turns set",
            "max_turns: 100\n",
            func(t *testing.T, cfg *Config) {
                if cfg.MaxTurns != 100 { t.Errorf("MaxTurns = %d, want 100", cfg.MaxTurns) }
                if cfg.MaxIterations != 3 { t.Errorf("default MaxIterations = %d, want 3", cfg.MaxIterations) }
                // CRITICAL: verify non-obvious defaults preserved
                if !cfg.SerenaEnabled { t.Error("SerenaEnabled default should be true") }
            },
        },
        {
            "bool explicit false vs absent",
            "gates_enabled: false\n",
            func(t *testing.T, cfg *Config) {
                if cfg.GatesEnabled { t.Error("GatesEnabled should be false") }
                if !cfg.SerenaEnabled { t.Error("SerenaEnabled default should be true (absent)") }
            },
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dir := t.TempDir()
            writeConfigYAML(t, dir, tt.yaml)
            t.Chdir(dir)
            cfg, err := Load(CLIFlags{})
            if err != nil { t.Fatalf("unexpected error: %v", err) }
            tt.check(t, cfg)
        })
    }
}

func TestConfig_Load_EmptyDocumentMarker(t *testing.T) {
    // CRITICAL: yaml.v3 ZEROS struct on "---" (GitHub #395)
    // Guard must preserve defaults
    dir := t.TempDir()
    writeConfigYAML(t, dir, "---\n")
    t.Chdir(dir)

    cfg, err := Load(CLIFlags{})
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if cfg.MaxTurns != 50 { t.Errorf("defaults lost: MaxTurns = %d, want 50", cfg.MaxTurns) }
    if !cfg.SerenaEnabled { t.Error("defaults lost: SerenaEnabled should be true") }
}

func TestConfig_Load_MissingFile(t *testing.T) {
    dir := t.TempDir()
    os.MkdirAll(filepath.Join(dir, ".ralph"), 0755) // .ralph/ exists but no config.yaml
    t.Chdir(dir)

    cfg, err := Load(CLIFlags{})
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if cfg.MaxTurns != 50 { t.Errorf("default MaxTurns = %d, want 50", cfg.MaxTurns) }
}

func TestConfig_Load_DefaultsComplete(t *testing.T) {
    // Verify ALL non-obvious defaults explicitly
    dir := t.TempDir()
    os.MkdirAll(filepath.Join(dir, ".ralph"), 0755)
    t.Chdir(dir)
    cfg, _ := Load(CLIFlags{})

    if cfg.ClaudeCommand != "claude" { t.Errorf("ClaudeCommand = %q", cfg.ClaudeCommand) }
    if cfg.SerenaEnabled != true { t.Error("SerenaEnabled should default to true") }
    if cfg.LearningsBudget != 200 { t.Errorf("LearningsBudget = %d", cfg.LearningsBudget) }
    if cfg.ReviewMinSeverity != "LOW" { t.Errorf("ReviewMinSeverity = %q", cfg.ReviewMinSeverity) }
    if cfg.LogDir != ".ralph/logs" { t.Errorf("LogDir = %q", cfg.LogDir) }
    // ... all 16 fields
}

// --- detectProjectRootFrom tests: no CWD dependency ---

func TestConfig_DetectProjectRootFrom_RalphDir(t *testing.T) {
    dir := t.TempDir()
    os.MkdirAll(filepath.Join(dir, ".ralph"), 0755)
    got := detectProjectRootFrom(dir)
    if got != dir { t.Errorf("got %q, want %q", got, dir) }
}

func TestConfig_DetectProjectRootFrom_RalphPriority(t *testing.T) {
    // .ralph/ in grandparent, .git/ in parent -> returns grandparent
    grandparent := t.TempDir()
    parent := filepath.Join(grandparent, "sub")
    child := filepath.Join(parent, "deep")
    os.MkdirAll(filepath.Join(grandparent, ".ralph"), 0755)
    os.MkdirAll(filepath.Join(parent, ".git"), 0755)
    os.MkdirAll(child, 0755)

    got := detectProjectRootFrom(child)
    if got != grandparent { t.Errorf("got %q, want %q (grandparent with .ralph/)", got, grandparent) }
}

// Helper: creates .ralph/config.yaml with given content
func writeConfigYAML(t *testing.T, dir, content string) {
    t.Helper()
    ralphDir := filepath.Join(dir, ".ralph")
    if err := os.MkdirAll(ralphDir, 0755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(ralphDir, "config.yaml"), []byte(content), 0644); err != nil { t.Fatal(err) }
}
```

**Optional golden file test:** Create `config/testdata/TestConfig_Load_Defaults.golden` with serialized default Config. Useful for regression detection if default values change unintentionally. Not in AC but recommended.

### Previous Story Intelligence (Story 1.2)

**Learnings from Story 1.2 implementation and review:**
- Write tool creates CRLF on Windows NTFS — **always run `sed -i 's/\r$//' <file>`** after creating/editing files
- `.gitattributes` with `* text=auto eol=lf` enforces LF on `git add`
- Go binary path: `"/mnt/c/Program Files/Go/bin/go.exe"` (Windows Go via WSL)
- Test function naming STRICTLY enforced: `Test<Type>_<Method>_<Scenario>` — "Type" must be a real Go type name
- `errors.As` tests must be **table-driven** with multiple scenarios (different field values, zero values)
- Always test **zero-value behavior** of structs
- Always test **double-wrapped errors** for unwrapping
- golangci-lint not installed in WSL — only CI catches lint issues
- `config/config.go` currently contains only `package config` — this is where Config struct goes
- `config/errors.go` already exists with ErrNoTasks, ErrMaxRetries, ExitCodeError, GateDecision

**Code review found in Story 1.2:**
- CRITICAL `.gitignore` bug (fixed): pattern without `/` matched directories at any depth
- CRLF issue (fixed): all files converted to LF
- Test naming convention violations caught and fixed (M1, M2 findings)

### Git Intelligence

**Recent commits (3 total):**
- `dccde3b` — Implement Stories 1.1 (scaffold) and 1.2 (error types) with full review
- `5271d1b` — Add epic breakdown: 6 epics, 54 stories, 42/42 MVP FR coverage
- `0f0102a` — Initial commit: bmad-ralph project with completed discovery, PRD, and architecture

**Patterns established:**
- Single commit per story (or per story batch)
- Files follow exact architecture structure
- Test files co-located with source (`config/errors_test.go` next to `config/errors.go`)
- No dependencies added beyond the 3 approved ones

### Architecture Compliance Checklist

- [ ] `config` remains leaf package (no imports of other project packages)
- [ ] Config struct passed by pointer, never mutated after Load
- [ ] Error wrapping follows `fmt.Errorf("config: operation: %w", err)` pattern
- [ ] No `os.Exit` calls in config package
- [ ] No `panic()` in production code paths
- [ ] No logging/printing (packages don't log — architecture rule)
- [ ] `errors.Is(err, os.ErrNotExist)` for file existence (not string matching)
- [ ] `os.ReadFile` for reading (files are small)
- [ ] Only uses: `gopkg.in/yaml.v3` + Go stdlib. No new dependencies

### Library/Framework Notes

- **gopkg.in/yaml.v3** (v3.0.1 in go.mod): Stable, well-established. Key behaviors:
  - `Unmarshal` into pre-initialized struct: only overwrites fields present in YAML, absent fields keep pre-set values
  - **GOTCHA: `---` (empty document) ZEROS the struct** — must guard against this (GitHub issue #395). Use `bytes.TrimSpace` check before Unmarshal
  - `null` YAML value for a field: yaml.v3 preserves the pre-set value (good for our defaults pattern)
  - Empty byte input `[]byte{}`: leaves struct unchanged when target is `*T` (can't nil the pointer)
  - Unknown fields silently ignored by default (no `KnownFields` needed)
  - Malformed YAML errors include line/column information
  - Struct tags: `yaml:"snake_case"`, `yaml:"-"` to exclude
- **Go 1.24+ testing features** available (go.mod says 1.25):
  - `t.Chdir(dir)` — safely changes CWD for a single test, auto-reverts on cleanup
- No new dependencies required for this story

### File Structure

Files to create/modify:
- `config/config.go` (MODIFY — currently just `package config`, add Config struct + Load + helpers)
- `config/config_test.go` (CREATE — table-driven tests for Load and project root detection)

Files NOT to touch:
- `config/errors.go` — already complete from Story 1.2
- `config/errors_test.go` — already complete from Story 1.2
- Any files outside `config/` package

### Project Structure Notes

- Alignment with architecture: `config.Load(flags)` matches entry point table in project-context.md
- `config` package remains a leaf — imports only yaml.v3 + Go stdlib
- `CLIFlags` defined here but empty — Story 1.4 adds pointer fields
- `ProjectRoot` has `yaml:"-"` tag — never read from YAML, always auto-detected

### Anti-Patterns (FORBIDDEN)

- `viper` or any config framework — use yaml.v3 directly (architecture decision)
- `yaml.Decoder.KnownFields(true)` — unknown fields MUST be silently ignored
- Logging/printing from config package — packages don't log
- `os.Exit` in config package — return errors
- `context.TODO()` — not applicable to this story (no subprocess calls)
- Mutating `*Config` after `Load()` returns
- Hard-coding config file path without using ProjectRoot
- `exec.Command` without context — not applicable but remember for future
- Using `bufio.Scanner` for file reading — use `os.ReadFile`
- String matching on errors — use `errors.Is`
- Adding any new external dependencies

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.3]
- [Source: docs/project-context.md#Architecture -- Key Decisions]
- [Source: docs/project-context.md#Testing]
- [Source: docs/architecture/core-architectural-decisions.md#External Dependencies]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Structural Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Error Handling Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#File I/O Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Testing Patterns]
- [Source: docs/architecture/project-structure-boundaries.md#Complete Project Directory Structure]
- [Source: docs/prd/functional-requirements.md#FR30]
- [Source: docs/prd/cli-tool-specific-requirements.md#Configuration]
- [Source: docs/sprint-artifacts/1-2-error-types-and-context-pattern.md#Completion Notes]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

No debug issues encountered. All tests passed on first run.

### Completion Notes List

- Implemented Config struct with 16 fields, all yaml tags matching PRD snake_case names
- CLIFlags defined as empty placeholder for Story 1.4
- defaultConfig() returns all defaults per AC table (SerenaEnabled=true, MaxTurns=50, etc.)
- detectProjectRoot/detectProjectRootFrom: two-pass walk-up (.ralph/ priority over .git/)
- walkUpFor helper: generic directory walk-up with filesystem root termination
- Load() with full error handling: missing file=defaults, empty doc guard (yaml.v3 #395), malformed YAML error, unknown fields ignored
- 16 test functions + 1 helper covering all AC scenarios (11 Load tests, 5 detectProjectRootFrom tests)
- All 22 config package tests pass (16 new + 6 existing from errors_test.go)
- go build, go test, go vet all pass
- CRLF fixed with sed after file creation
- No new dependencies added — uses only yaml.v3 + Go stdlib
- Config immutability verified: no mutation of *Config after Load returns

### Code Review Fixes Applied

- **M1 (test gap):** Added `TestConfig_Load_UnreadableConfig` — covers non-NotExist os.ReadFile error path
- **M2 (fragile guard):** Replaced `bytes.TrimSpace` string check with `map[string]any` probe — handles all yaml.v3 struct-zeroing edge cases (comments-only, multi-document, end markers). Added `TestConfig_Load_CommentsOnlyYAML` to verify. Removed `"bytes"` import.
- **M3 (test gap):** Added `TestConfig_Load_GitFallbackRoot` — verifies Load() when project root detected via .git/ fallback
- **M4 (string matching):** Added justification comments to string-matching assertions in MalformedYAML and UnreadableConfig tests (yaml.v3/os don't export relevant error types)
- **L1 (duplication):** Merged standalone BoolExplicitFalseVsAbsent test into PartialConfig table

### Change Log

- 2026-02-25: Implemented Config struct, Load(), project root detection, and comprehensive tests (Story 1.3)

### File List

- config/config.go (MODIFIED — added Config struct, CLIFlags, defaultConfig, Load, detectProjectRoot, detectProjectRootFrom, walkUpFor)
- config/config_test.go (CREATED — 15 test functions for Load and detectProjectRootFrom)
- docs/sprint-artifacts/sprint-status.yaml (MODIFIED — story status: ready-for-dev → in-progress → review)
