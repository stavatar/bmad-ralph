# Story 1.4: Config CLI Override + go:embed Defaults

Status: done

## Story

As a developer,
I want CLI flags to override config file values, with embedded defaults as fallback,
so that the three-level cascade works: CLI > config file > embedded defaults. (FR31)

## Acceptance Criteria

```gherkin
Given embedded defaults exist via go:embed
And a config file exists with some overrides
And CLI flags are provided for some parameters
When config.Load(flags) is called
Then CLI flags take highest priority
And config file values override embedded defaults
And embedded defaults fill remaining gaps

Given CLI flag --max-turns=100 is set
And config file has max_turns: 50
When config is loaded
Then config.MaxTurns == 100

Given CLI flag --max-turns is NOT set
And config file has max_turns: 50
When config is loaded
Then config.MaxTurns == 50

Given CLI flag --max-turns is NOT set
And config file does NOT have max_turns
When config is loaded
Then config.MaxTurns == 50 (embedded default)

And go:embed defaults are defined in config package as embedded YAML
And CLIFlags uses pointer fields for "was set" tracking:
  *int for numeric flags, *string for string flags, *bool for boolean flags
  nil = flag not set (use config/default), non-nil = flag explicitly set
And table-driven tests cover all three cascade levels for at least 3 parameters
```

## Tasks / Subtasks

- [x] Task 1: Create config/defaults.yaml with all default values (AC: go:embed defaults defined)
  - [x] 1.1 Create `config/defaults.yaml` with all 15 YAML-configurable parameters and their default values
  - [x] 1.2 Values MUST match the defaults from Story 1.3 AC table exactly (e.g., max_turns: 50, serena_enabled: true)
  - [x] 1.3 YAML keys MUST match Config struct yaml tags exactly (snake_case)
  - [x] 1.4 Run `sed -i 's/\r$//' config/defaults.yaml` after creating file (CRLF fix)
- [x] Task 2: Embed defaults.yaml and update defaultConfig() (AC: embedded defaults as fallback)
  - [x] 2.1 Add `//go:embed defaults.yaml` variable `var defaultsYAML []byte` in `config/config.go`
  - [x] 2.2 Add blank import `_ "embed"` to import block (REQUIRED by Go for all `//go:embed` usage)
  - [x] 2.3 Replace hardcoded `defaultConfig()` body with `yaml.Unmarshal(defaultsYAML, &cfg)`
  - [x] 2.4 Panic on unmarshal failure: embedded file is compiled-in, parsing failure = programming error. Architecture allows panic for missing/corrupt embed
  - [x] 2.5 Verify existing tests still pass — behavior identical, only source of defaults changed
- [x] Task 3: Add pointer fields to CLIFlags (AC: pointer fields for "was set" tracking)
  - [x] 3.1 Replace empty `CLIFlags struct{}` with pointer fields for CLI-overridable parameters
  - [x] 3.2 Fields to add (matching Story 1.13 run flags + reasonable extras):
    - `MaxTurns *int` (Story 1.13: --max-turns)
    - `MaxIterations *int` (useful for override)
    - `MaxReviewIterations *int` (useful for override)
    - `GatesEnabled *bool` (Story 1.13: --gates)
    - `GatesCheckpoint *int` (Story 1.13: --every)
    - `ReviewEvery *int` (useful for override)
    - `ModelExecute *string` (Story 1.13: --model)
    - `ModelReview *string` (useful for override)
    - `AlwaysExtract *bool` (Story 1.13: --always-extract)
  - [x] 3.3 Verify existing tests compile — `CLIFlags{}` zero value has all nil pointers (no overrides)
- [x] Task 4: Add CLI flag override logic in Load() (AC: CLI flags take highest priority)
  - [x] 4.1 Create unexported `applyCLIFlags(cfg *Config, flags CLIFlags)` function
  - [x] 4.2 For each pointer field in CLIFlags: if non-nil, assign dereferenced value to Config field
  - [x] 4.3 Call `applyCLIFlags(cfg, flags)` in Load() AFTER YAML file parsing, BEFORE return
  - [x] 4.4 Placement in Load(): after yaml.Unmarshal block, before final return
- [x] Task 5: Write cascade tests (AC: table-driven tests for all three cascade levels, at least 3 parameters)
  - [x] 5.1 Create `TestConfig_Load_CLIOverridesConfigFile` — CLI flag overrides config file value
  - [x] 5.2 Create `TestConfig_Load_ConfigOverridesEmbedded` — config file overrides embedded default (this is already tested by existing tests, but add explicit test naming the cascade)
  - [x] 5.3 Create `TestConfig_Load_EmbeddedDefaultUsed` — no CLI, no config → embedded default used
  - [x] 5.4 Create `TestConfig_Load_CascadeThreeLevels` — table-driven with at least 3 parameters testing all three cascade levels:
    - int parameter (MaxTurns): CLI=100, config=75, embedded=50
    - bool parameter (GatesEnabled): CLI=true, config=false, embedded=false
    - string parameter (ModelExecute): CLI="haiku", config="sonnet", embedded=""
  - [x] 5.5 Test: CLI flag bool explicitly false overrides config true (e.g., GatesEnabled=false via CLI, true in config → false)
  - [x] 5.6 Test: CLI flag int zero overrides non-zero config (e.g., GatesCheckpoint=0 via CLI, 5 in config → 0)
  - [x] 5.7 Test: CLI flag + no config file → CLI overrides embedded default (e.g., MaxTurns=100 via CLI, no config file → 100, not embedded 50)
  - [x] 5.8 Test: nil CLIFlags (no flags set) → config/embedded values unchanged (existing tests cover this, verify they still pass)
  - [x] 5.9 Create `TestConfig_Load_DefaultsFromEmbed` — verify defaults come from defaults.yaml, not hardcoded Go (modify defaults.yaml in test? No — use existing default verification tests)
- [x] Task 6: Validation (AC: all existing tests still pass)
  - [x] 6.1 `go build ./...` passes
  - [x] 6.2 `go test ./config/...` passes with ALL tests green (existing + new)
  - [x] 6.3 `go vet ./...` passes
  - [x] 6.4 Verify Config immutability: no mutation of *Config after Load returns
  - [x] 6.5 Verify `config` remains leaf package (no new project imports)

## Dev Notes

### Implementation Guide

**This story modifies `config/config.go` and `config/config_test.go` (existing files from Story 1.3) and creates one new file `config/defaults.yaml`.**

The key change: defaults are now sourced from an embedded YAML file instead of hardcoded Go, and CLI flags can override any level of the cascade.

### defaults.yaml Content (MUST CREATE)

Create `config/defaults.yaml` with EXACTLY these values (matching Story 1.3 AC table):

```yaml
claude_command: "claude"
max_turns: 50
max_iterations: 3
max_review_iterations: 3
gates_enabled: false
gates_checkpoint: 0
review_every: 1
model_execute: ""
model_review: ""
review_min_severity: "LOW"
always_extract: false
serena_enabled: true
serena_timeout: 10
learnings_budget: 200
log_dir: ".ralph/logs"
```

**CRITICAL:** YAML keys MUST match the Config struct `yaml:"..."` tags exactly. If they don't, yaml.v3 will silently ignore mismatched keys and Go zero values will be used (0, false, "") instead of the intended defaults.

### go:embed Pattern

```go
import (
    _ "embed" // Required for //go:embed directive
    // ... other imports
)

//go:embed defaults.yaml
var defaultsYAML []byte
```

**Go requires `import _ "embed"`** (blank import) for ALL `//go:embed` usage, including `string` and `[]byte` types. The `//go:embed` comment MUST appear on the line immediately before the `var` declaration with no blank line between them.

### Updated defaultConfig()

```go
func defaultConfig() *Config {
    var cfg Config
    // Embedded defaults are compiled into the binary.
    // Parsing failure indicates corrupt defaults.yaml — programming error.
    if err := yaml.Unmarshal(defaultsYAML, &cfg); err != nil {
        panic("config: embedded defaults.yaml: " + err.Error())
    }
    return &cfg
}
```

**Why panic:** Architecture says "Panic: Никогда в runtime. Только init() для missing embed". An embedded YAML file that fails to parse is the same category — a programming error that should never happen in a correctly built binary. This panic is acceptable.

**Alternative considered:** Return error from defaultConfig() → would require changing Load() signature and all callers. Not worth it for a "should never happen" case.

### Updated CLIFlags Struct

```go
// CLIFlags holds command-line flag values for the three-level cascade:
// CLI flags > config file > embedded defaults.
// Pointer fields: nil = flag not set (use config/default), non-nil = explicitly set.
type CLIFlags struct {
    MaxTurns            *int
    MaxIterations       *int
    MaxReviewIterations *int
    GatesEnabled        *bool
    GatesCheckpoint     *int
    ReviewEvery         *int
    ModelExecute        *string
    ModelReview         *string
    AlwaysExtract       *bool
}
```

**Why these 9 fields:** These are the parameters that Story 1.13 (CLI Wiring) and future stories will expose as CLI flags. Parameters like `claude_command`, `serena_enabled`, `serena_timeout`, `learnings_budget`, `log_dir`, `review_min_severity` are config-file-only settings — no CLI flags planned.

**Backward compatibility:** `CLIFlags{}` zero value has all nil pointers → no overrides → existing tests pass unchanged.

### applyCLIFlags Helper

```go
func applyCLIFlags(cfg *Config, flags CLIFlags) {
    if flags.MaxTurns != nil {
        cfg.MaxTurns = *flags.MaxTurns
    }
    if flags.MaxIterations != nil {
        cfg.MaxIterations = *flags.MaxIterations
    }
    if flags.MaxReviewIterations != nil {
        cfg.MaxReviewIterations = *flags.MaxReviewIterations
    }
    if flags.GatesEnabled != nil {
        cfg.GatesEnabled = *flags.GatesEnabled
    }
    if flags.GatesCheckpoint != nil {
        cfg.GatesCheckpoint = *flags.GatesCheckpoint
    }
    if flags.ReviewEvery != nil {
        cfg.ReviewEvery = *flags.ReviewEvery
    }
    if flags.ModelExecute != nil {
        cfg.ModelExecute = *flags.ModelExecute
    }
    if flags.ModelReview != nil {
        cfg.ModelReview = *flags.ModelReview
    }
    if flags.AlwaysExtract != nil {
        cfg.AlwaysExtract = *flags.AlwaysExtract
    }
}
```

**Why not reflection/generics:** Simple, explicit, readable. 9 if-statements are clearer than reflection magic. Architecture favors simplicity.

### Updated Load() Function

The ONLY change in Load() is adding one line after the YAML parsing block:

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
            applyCLIFlags(cfg, flags) // Apply CLI overrides even without config file
            return cfg, nil
        }
        return nil, fmt.Errorf("config: read: %w", err)
    }

    var probe map[string]any
    if err := yaml.Unmarshal(data, &probe); err != nil {
        return nil, fmt.Errorf("config: parse yaml: %w", err)
    }
    if len(probe) == 0 {
        applyCLIFlags(cfg, flags) // Apply CLI overrides on empty config
        return cfg, nil
    }

    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("config: parse yaml: %w", err)
    }

    applyCLIFlags(cfg, flags) // CLI flags override everything

    return cfg, nil
}
```

**CRITICAL:** `applyCLIFlags` must be called on ALL return paths that return a valid config (3 places): after missing file, after empty document, and after successful YAML parse. This ensures CLI flags ALWAYS override, regardless of config file state.

### Test Helpers for CLI Flags

```go
// intPtr returns a pointer to the given int value.
func intPtr(v int) *int { return &v }

// boolPtr returns a pointer to the given bool value.
func boolPtr(v bool) *bool { return &v }

// strPtr returns a pointer to the given string value.
func strPtr(v string) *string { return &v }
```

These helpers make test code readable: `CLIFlags{MaxTurns: intPtr(100)}` vs `CLIFlags{MaxTurns: &hundred}`.

### Example Test: Three-Level Cascade

```go
func TestConfig_Load_CascadeThreeLevels(t *testing.T) {
    tests := []struct {
        name       string
        yaml       string          // config file content ("" = no config file)
        flags      CLIFlags        // CLI flags
        checkField string          // which field to verify
        wantInt    int             // expected int value (0 if not checking int)
        wantBool   bool            // expected bool value
        wantStr    string          // expected string value
    }{
        // MaxTurns: int cascade
        {
            name:       "int/CLI overrides config",
            yaml:       "max_turns: 75\n",
            flags:      CLIFlags{MaxTurns: intPtr(100)},
            checkField: "MaxTurns",
            wantInt:    100,
        },
        {
            name:       "int/config overrides embedded",
            yaml:       "max_turns: 75\n",
            flags:      CLIFlags{},
            checkField: "MaxTurns",
            wantInt:    75,
        },
        {
            name:       "int/embedded default used",
            yaml:       "",  // no config file
            flags:      CLIFlags{},
            checkField: "MaxTurns",
            wantInt:    50,  // embedded default
        },
        {
            name:       "int/CLI overrides embedded (no config file)",
            yaml:       "",  // no config file
            flags:      CLIFlags{MaxTurns: intPtr(100)},
            checkField: "MaxTurns",
            wantInt:    100,  // CLI wins over embedded default of 50
        },
        // GatesEnabled: bool cascade
        {
            name:       "bool/CLI true overrides config false",
            yaml:       "gates_enabled: false\n",
            flags:      CLIFlags{GatesEnabled: boolPtr(true)},
            checkField: "GatesEnabled",
            wantBool:   true,
        },
        {
            name:       "bool/CLI false overrides config true",
            yaml:       "gates_enabled: true\n",
            flags:      CLIFlags{GatesEnabled: boolPtr(false)},
            checkField: "GatesEnabled",
            wantBool:   false,
        },
        // ModelExecute: string cascade
        {
            name:       "string/CLI overrides config",
            yaml:       "model_execute: sonnet\n",
            flags:      CLIFlags{ModelExecute: strPtr("haiku")},
            checkField: "ModelExecute",
            wantStr:    "haiku",
        },
        {
            name:       "string/config overrides embedded",
            yaml:       "model_execute: sonnet\n",
            flags:      CLIFlags{},
            checkField: "ModelExecute",
            wantStr:    "sonnet",
        },
        {
            name:       "string/embedded default used",
            yaml:       "",
            flags:      CLIFlags{},
            checkField: "ModelExecute",
            wantStr:    "",  // embedded default
        },
    }
    // ... t.Run loop with field checking
}
```

### Edge Case Tests

```go
// CLI flag int=0 overrides non-zero config
func TestConfig_Load_CLIZeroOverrides(t *testing.T) {
    dir := t.TempDir()
    writeConfigYAML(t, dir, "gates_checkpoint: 5\n")
    t.Chdir(dir)

    cfg, err := Load(CLIFlags{GatesCheckpoint: intPtr(0)})
    if err != nil { t.Fatal(err) }
    if cfg.GatesCheckpoint != 0 {
        t.Errorf("GatesCheckpoint = %d, want 0 (CLI override)", cfg.GatesCheckpoint)
    }
}
```

### Project Structure Notes

- `config` remains leaf package (no imports of other project packages)
- Only modification: `config/config.go` (embed, CLIFlags, applyCLIFlags, Load update)
- Only addition: `config/defaults.yaml` (new file, embedded)
- Tests: `config/config_test.go` (add cascade tests, add helper functions)
- No changes to `config/errors.go` or `config/errors_test.go`
- No changes outside `config/` package

### Previous Story Intelligence (Story 1.3)

**Learnings from Story 1.3 implementation and review:**
- yaml.v3 #395 guard uses `map[string]any` probe (not `bytes.TrimSpace`) — already implemented, keep as-is
- `t.Chdir(dir)` pattern works well for Load() tests — reuse for cascade tests
- `writeConfigYAML` helper exists — reuse for new tests
- All 22 config tests pass currently — new tests must not break existing ones
- CRLF fix required after creating files with Write tool: `sed -i 's/\r$//'`
- Test naming convention: `Test<Type>_<Method>_<Scenario>` — strictly enforced

**Code review findings from Story 1.3:**
- M1: Always test non-happy-path os.ReadFile errors (permission denied, is-a-directory)
- M2: yaml.v3 #395 guard must use `map[string]any` probe, not `bytes.TrimSpace`
- M3: Integration tests should cover all detectProjectRoot paths
- M4: String matching on errors needs justification comments
- L1: Merge standalone tests into table-driven where appropriate

### Git Intelligence

**Recent commits (4 total):**
- `bfa30c2` — Story 1.3: Config struct, YAML parsing, project root detection
- `dccde3b` — Stories 1.1 + 1.2: scaffold + error types
- `5271d1b` — Epic breakdown: 6 epics, 54 stories
- `0f0102a` — Initial commit: discovery, PRD, architecture

**Patterns established:**
- Single commit per story
- `config/config.go` is the main file being modified (struct + Load + helpers)
- `config/config_test.go` co-located test file
- Table-driven tests with `t.Run` and `t.Chdir`
- Go binary at `"/mnt/c/Program Files/Go/bin/go.exe"` (Windows Go via WSL)
- CRLF fix with `sed -i 's/\r$//'` after Write tool

**Files created/modified in Story 1.3:**
- `config/config.go` — Config struct, CLIFlags, defaultConfig, Load, detectProjectRoot
- `config/config_test.go` — 16 test functions covering all Load and detection scenarios

### Architecture Compliance Checklist

- [ ] `config` remains leaf package (no imports of other project packages)
- [ ] `config/defaults.yaml` contains correct default values matching AC table
- [ ] `//go:embed defaults.yaml` correctly loads the file into `[]byte`
- [ ] `defaultConfig()` parses embedded YAML (panic on failure = programming error)
- [ ] `CLIFlags` uses pointer fields (*int, *string, *bool) for nil-vs-set tracking
- [ ] `applyCLIFlags()` applies non-nil flag values after YAML parsing
- [ ] `applyCLIFlags()` called on ALL successful return paths in Load()
- [ ] Config struct passed by pointer, never mutated after Load
- [ ] Error wrapping follows `fmt.Errorf("config: operation: %w", err)` pattern
- [ ] No `os.Exit` calls in config package
- [ ] No logging/printing from config package
- [ ] Existing tests still pass with no changes
- [ ] Only uses: `gopkg.in/yaml.v3` + Go stdlib. No new dependencies

### Library/Framework Notes

- **go:embed** (Go 1.16+, available in Go 1.25): Embeds files into Go binary at compile time. `//go:embed defaults.yaml` + `var defaultsYAML []byte` — file content available as byte slice. File must be in same package directory or subdirectory. **REQUIRES `import _ "embed"`** (blank import) — mandatory for all `//go:embed` usage including `string` and `[]byte`. The `//go:embed` comment must be immediately before the `var` declaration (no blank line).
- **gopkg.in/yaml.v3** (v3.0.1): Same version as Story 1.3. Key behavior for this story: `yaml.Unmarshal` into pre-initialized struct overwrites ONLY fields present in YAML. This means: (1) embedded defaults fill base Config, (2) config file overwrites present fields, (3) applyCLIFlags overwrites non-nil fields. The cascade works naturally with yaml.v3's partial-override behavior.
- No new dependencies required.

### File Structure

Files to create:
- `config/defaults.yaml` (NEW — embedded YAML with all default values)

Files to modify:
- `config/config.go` (MODIFY — add embed, update CLIFlags, update defaultConfig, add applyCLIFlags, update Load)
- `config/config_test.go` (MODIFY — add cascade tests, add intPtr/boolPtr/strPtr helpers)

Files NOT to touch:
- `config/errors.go` — complete from Story 1.2
- `config/errors_test.go` — complete from Story 1.2
- Any files outside `config/` package

### Anti-Patterns (FORBIDDEN)

- Forgetting `import _ "embed"` — Go REQUIRES blank import for all `//go:embed` usage, including `[]byte`
- Using reflection or generics for applyCLIFlags — simple if-statements are correct
- Returning error from defaultConfig() — panic is appropriate for embedded file corruption
- Adding validation logic in applyCLIFlags (e.g., MaxTurns > 0) — validation is a separate concern, not in this story
- Hardcoding defaults in BOTH Go code AND defaults.yaml — single source of truth is defaults.yaml
- Mutating `*Config` after `Load()` returns
- Adding new external dependencies
- Using `context.TODO()` — not applicable
- `os.Exit` in config package
- Logging/printing from config package

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.4]
- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.13] (CLI flags that will wire up to CLIFlags)
- [Source: docs/project-context.md#Architecture -- Key Decisions]
- [Source: docs/project-context.md#Config Immutability]
- [Source: docs/architecture/core-architectural-decisions.md#External Dependencies]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Structural Patterns]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Error Handling Patterns]
- [Source: docs/architecture/project-structure-boundaries.md#Complete Project Directory Structure]
- [Source: docs/prd/functional-requirements.md#FR31]
- [Source: docs/sprint-artifacts/1-3-config-struct-yaml-parsing.md#Completion Notes]
- [Source: docs/sprint-artifacts/1-3-config-struct-yaml-parsing.md#Code Review Fixes Applied]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

No issues encountered. All tasks completed in a single pass.

### Completion Notes List

- Created `config/defaults.yaml` with all 15 default parameters matching Story 1.3 AC table exactly
- Replaced hardcoded `defaultConfig()` with `go:embed` + `yaml.Unmarshal` (panic on corrupt embed per architecture)
- Added `_ "embed"` blank import and `//go:embed defaults.yaml` directive
- Updated `CLIFlags` from empty struct to 9 pointer fields (*int, *string, *bool) for nil-vs-set tracking
- Created `applyCLIFlags()` with explicit if-nil checks for all 9 fields
- Updated `Load()` to call `applyCLIFlags` on all 3 successful return paths (missing file, empty config, valid config)
- Added 7 new test functions covering three-level cascade (int, bool, string), edge cases (CLI zero overrides non-zero), CLI+no-config-file, CLI+empty-config, and comprehensive all-9-flags override
- Table-driven `TestConfig_Load_CascadeThreeLevels` with 11 subtests covering all cascade combinations for 3 parameter types
- All 33 tests pass (16 existing config + 10 existing error + 7 new cascade tests)
- `go build`, `go vet` pass; `config` remains leaf package; no new dependencies

### Code Review Fixes Applied

- M1: Removed duplicate `TestConfig_Load_CLIFalseOverridesConfigTrue` (exact copy of cascade table case)
- M2: Removed duplicate `TestConfig_Load_DefaultsFromEmbed` (strict subset of existing `TestConfig_Load_DefaultsComplete`)
- M3: Added `TestConfig_Load_AllCLIFlagsOverrideFullConfig` — all 9 CLI flags vs full config with all 15 params, verifying CLI-overridable fields AND config-only fields remain untouched
- L1: Removed overlapping `TestConfig_Load_NilCLIFlagsNoOverride` (covered by cascade table)
- L2: Deleted orphan `docs/sprint-artifacts/validation-report-1-4-2026-02-25.md` (leftover from create-story validation)
- L3: Accepted — `[]byte` for `defaultsYAML` follows Go convention; mutation risk minimal (unexported, read-only usage)

### File List

- `config/defaults.yaml` (NEW) — embedded YAML with all 15 default configuration values
- `config/config.go` (MODIFIED) — added go:embed, updated CLIFlags with pointer fields, new applyCLIFlags(), updated defaultConfig() and Load()
- `config/config_test.go` (MODIFIED) — added intPtr/boolPtr/strPtr helpers and 7 cascade test functions (after review cleanup)

## Change Log

- 2026-02-25: Implemented Story 1.4 — three-level config cascade (CLI > config file > embedded defaults) with go:embed, CLIFlags pointer fields, and comprehensive cascade tests
- 2026-02-25: Code review fixes — removed 3 duplicate tests, added comprehensive all-9-flags override test, cleaned up orphan file
