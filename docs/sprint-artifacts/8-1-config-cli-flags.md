# Story 8.1: Sync Config + CLI Flag

Status: done

## Story

As a разработчик,
I want настроить Serena sync через config файл и CLI флаг,
so that включать автоматическую синхронизацию memories без правки кода.

## Acceptance Criteria

1. **Config fields (FR63):** Config struct расширен 3 полями: `SerenaSyncEnabled bool yaml:"serena_sync_enabled"` (default false), `SerenaSyncMaxTurns int yaml:"serena_sync_max_turns"` (default 5), `SerenaSyncTrigger string yaml:"serena_sync_trigger"` (default "task"). Все поля парсятся из `.ralph/config.yaml` с fallback на defaults.
2. **CLI flag --serena-sync (FR63):** `cmd/ralph/run.go` определяет `--serena-sync` boolean flag. `CLIFlags.SerenaSyncEnabled` — pointer `*bool`. При `ralph run --serena-sync` → `SerenaSyncEnabled = true`. Стандартный каскад CLI > config > defaults.
3. **Validate trigger values (FR63):** `Config.Validate()` проверяет `SerenaSyncTrigger`: "run" → valid, "task" → valid, "" → valid (treated as "task" default), "invalid" → error: `config: validate: invalid serena_sync_trigger "invalid" (must be "run" or "task")`.
4. **Validate max turns (FR63):** `Config.Validate()` проверяет `SerenaSyncMaxTurns`: 0 → corrected to 5, negative → corrected to 5, 1..100 → valid. Existing validation rules unchanged.
5. **Config round-trip:** `config.Load()` с YAML файлом, содержащим все 3 `serena_sync_*` поля → все поля populated. YAML без этих полей → defaults applied.

## Tasks / Subtasks

- [x] Task 1: Add 3 fields to Config struct (AC: #1)
  - [x] 1.1 Add `SerenaSyncEnabled bool yaml:"serena_sync_enabled"` to Config in `config/config.go`
  - [x] 1.2 Add `SerenaSyncMaxTurns int yaml:"serena_sync_max_turns"` to Config in `config/config.go`
  - [x] 1.3 Add `SerenaSyncTrigger string yaml:"serena_sync_trigger"` to Config in `config/config.go`
  - [x] 1.4 Add 3 default values to `config/defaults.yaml`: `serena_sync_enabled: false`, `serena_sync_max_turns: 5`, `serena_sync_trigger: "task"`

- [x] Task 2: Add CLIFlags field and --serena-sync flag (AC: #2)
  - [x] 2.1 Add `SerenaSyncEnabled *bool` to CLIFlags struct in `config/config.go`
  - [x] 2.2 Add `--serena-sync` flag definition in `cmd/ralph/run.go` init()
  - [x] 2.3 Add `SerenaSyncEnabled` handling in `buildCLIFlags()` in `cmd/ralph/run.go`
  - [x] 2.4 Add `SerenaSyncEnabled` handling in `applyCLIFlags()` in `config/config.go`

- [x] Task 3: Extend Validate() with trigger and max turns checks (AC: #3, #4)
  - [x] 3.1 Add trigger enum validation in `Config.Validate()`: "run", "task", "" → valid; others → error
  - [x] 3.2 Add max turns minimum enforcement in `Config.Validate()`: < 1 → correct to 5

- [x] Task 4: Tests (AC: #1-#5)
  - [x] 4.1 Config round-trip tests in `config/config_test.go`: Load() with/without serena_sync fields
  - [x] 4.2 Validate trigger tests in `config/config_test.go`: table-driven with valid/invalid trigger values
  - [x] 4.3 Validate max turns tests: 0, negative, 1, 100 → correct behavior
  - [x] 4.4 CLIFlags tests in `cmd/ralph/run_test.go`: --serena-sync flag wiring, buildCLIFlags mapping
  - [x] 4.5 Defaults test: verify default values (false, 5, "task") from embedded defaults.yaml
  - [x] 4.6 applyCLIFlags test: SerenaSyncEnabled override from CLI

## Dev Notes

### Architecture Compliance

- **Config = leaf package.** No new imports needed. Fields follow existing yaml tag pattern.
- **Config immutability:** These 3 fields parsed once at startup, never mutated in runtime. Validate() may correct `SerenaSyncMaxTurns` (same pattern as existing: it's pre-run initialization, not runtime mutation).
- **Dependency direction:** Changes in `config/` and `cmd/ralph/` only. No runner changes in this story.
- **CLI cascade:** CLI > config > defaults. Same pattern as `GatesEnabled`, `MaxTurns`, etc.

### Implementation Patterns (from existing code)

**Config struct** (`config/config.go:19-46`):
- Fields grouped logically. Add serena_sync fields after existing `SerenaEnabled` field (line 31).
- YAML tags use snake_case: `yaml:"serena_sync_enabled"`.

**CLIFlags** (`config/config.go:51-61`):
- Pointer fields: `*bool` for `SerenaSyncEnabled`. nil = not set by user.
- Only `SerenaSyncEnabled` needs CLI flag. `MaxTurns` and `Trigger` are config-only (no CLI override per AC).

**applyCLIFlags** (`config/config.go:75+`):
- Pattern: `if flags.Field != nil { cfg.Field = *flags.Field }`.
- Add at end of function.

**Validate()** (`config/config.go:160-200`):
- Error format: `fmt.Errorf("config: validate: ...")`.
- Trigger validation: switch on value, error for unknown. Pattern: same as `ReviewMinSeverity` validation (line 170-175).
- MaxTurns correction: silent fix to 5 (not error). AC says "corrected to 5", not "error". This differs from `MaxTurns` which errors on <=0.

**defaults.yaml** (`config/defaults.yaml`):
- Add 3 lines at end: `serena_sync_enabled: false`, `serena_sync_max_turns: 5`, `serena_sync_trigger: "task"`.

**CLI flag** (`cmd/ralph/run.go:28-34`):
- Pattern: `runCmd.Flags().Bool("serena-sync", false, "Enable Serena memory sync after run")`.
- In `buildCLIFlags()`: `if cmd.Flags().Changed("serena-sync") { v, _ := cmd.Flags().GetBool("serena-sync"); flags.SerenaSyncEnabled = &v }`.

### Critical Constraints

- **No runtime behavior changes.** This story is config-only. Sync logic is in Story 8.4.
- **Existing tests must pass.** No changes to existing validation behavior.
- **Error format exact match:** AC specifies exact error message for invalid trigger: `config: validate: invalid serena_sync_trigger "invalid" (must be "run" or "task")`.
- **MaxTurns correction is silent:** No error — just correct to 5 when < 1. Different from validation errors.
- **Empty trigger is valid:** Treated as "task" default (no correction needed in Validate — runtime code in 8.4 will handle empty as "task").

### Testing Standards

- **Table-driven** with Go stdlib assertions (no testify). `t.TempDir()` for config file tests.
- **Test naming:** `TestConfig_Validate_SerenaSyncTrigger`, `TestConfig_Validate_SerenaSyncMaxTurns`, `TestConfig_Load_SerenaSyncFields`.
- **Error message assertions:** `strings.Contains(err.Error(), "serena_sync_trigger")` — verify message content, not bare `err != nil`.
- **Golden files:** Not needed for this story (no prompt templates).
- **CLI flag tests:** `TestBuildCLIFlags_SerenaSyncFlag` — verify flag maps to correct CLIFlags field, verify DefValue.
- **Coverage target:** config >80%.

### Project Structure Notes

- `config/config.go` — Config struct, CLIFlags, Validate(), applyCLIFlags()
- `config/defaults.yaml` — Embedded defaults
- `cmd/ralph/run.go` — CLI flag definitions, buildCLIFlags()
- `config/config_test.go` — Config and Validate tests
- `cmd/ralph/run_test.go` — CLI flag wiring tests

### References

- [Source: docs/epics/epic-8-serena-memory-sync-stories.md#Story 8.1] — AC and technical notes
- [Source: docs/prd/serena-memory-sync.md#FR63] — Config fields + CLI flag + validation
- [Source: docs/architecture/serena-memory-sync.md#Decision 5] — Config extension details
- [Source: config/config.go:19-46] — Existing Config struct (25+ fields)
- [Source: config/config.go:51-61] — Existing CLIFlags struct (pointer pattern)
- [Source: config/config.go:75-100] — applyCLIFlags pattern
- [Source: config/config.go:160-200] — Validate() pattern
- [Source: config/defaults.yaml] — Existing defaults (22 lines)
- [Source: cmd/ralph/run.go:28-34] — Existing CLI flag definitions
- [Source: cmd/ralph/run.go:156-181] — buildCLIFlags pattern

## Dev Agent Record

### Context Reference

<!-- Story context created by create-story workflow -->

### Agent Model Used
Claude Opus 4.6

### Debug Log References
N/A

### Completion Notes List
- All 4 tasks completed, all 16 subtasks done
- All AC verified: config fields, CLI flag, trigger validation, max turns correction, round-trip
- Full test suite passes: `go test ./...` — 0 failures
- No new dependencies added

### File List
- `config/config.go` — Added 3 Config fields, 1 CLIFlags field, applyCLIFlags handler, Validate() trigger+maxturns
- `config/defaults.yaml` — Added 3 default values
- `cmd/ralph/run.go` — Added --serena-sync flag definition and buildCLIFlags handler
- `config/config_test.go` — Added 4 new test functions + updated 3 existing tests
- `cmd/ralph/run_test.go` — Added TestBuildCLIFlags_SerenaSyncFlag

### Review Record
- **Reviewer:** Claude Opus 4.6
- **Findings:** 0H / 3M / 2L (5 total)
- **All fixed:** M1 (Validate doc comment), M2 (ValidFullConfig test updated), M3 (error prefix assertion), L1 (File List count), L2 (beyond-range test case)
