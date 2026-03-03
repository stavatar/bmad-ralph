# Story 6.7: Serena MCP Integration

Status: Ready for Review

## Story

As a runner,
I want обнаруживать Serena MCP server и добавлять prompt hint для использования её tools,
so that улучшить code navigation при наличии Serena.

## Acceptance Criteria

```gherkin
Scenario: Serena MCP detected via config files (C3)
  Given .claude/settings.json or .mcp.json contains Serena MCP config
  When ralph run starts
  Then CodeIndexerDetector.Available(projectRoot) returns true
  And logs "Serena MCP detected"

Scenario: Detection reads config files only (C3)
  Given Serena MCP detection needed
  When Available(projectRoot) called
  Then reads .claude/settings.json or .mcp.json for Serena MCP config
  And NO exec.LookPath("serena") used
  And NO serena index --full called
  And Ralph does NOT call Serena directly

Scenario: Prompt hint injected when available (C3)
  Given Serena MCP detected
  When prompt assembled
  Then CodeIndexerDetector.PromptHint() returns hint string
  And hint: "If Serena MCP tools available, use them for code navigation"
  And hint injected into execute and review prompts

Scenario: Serena unavailable graceful fallback
  Given .claude/settings.json has no Serena config
  And .mcp.json does not exist
  When ralph run starts
  Then CodeIndexerDetector.Available() returns false
  And no Serena prompt hint injected
  And runner operates normally
  And no error

Scenario: Serena configurable
  Given config with serena_enabled: false
  When ralph run starts
  Then skips Serena detection entirely
  And no Serena-related output

Scenario: Minimal CodeIndexerDetector interface (M5/C3)
  Given CodeIndexerDetector interface
  When implemented
  Then only 2 methods: Available(projectRoot string) bool, PromptHint() string
  And no index commands, no timeout management, no progress output
```

## Tasks / Subtasks

- [x] Task 1: Define CodeIndexerDetector interface (AC: #6)
  - [x] 1.1 Define `CodeIndexerDetector` interface in `runner/serena.go`: `Available(projectRoot string) bool`, `PromptHint() string`
  - [x] 1.2 Doc comment: minimal interface per M5/C3, no index commands, no timeout management
  - [x] 1.3 Define `NoOpCodeIndexerDetector` struct with `Available() = false`, `PromptHint() = ""`

- [x] Task 2: Implement SerenaMCPDetector (AC: #1, #2)
  - [x] 2.1 Create `SerenaMCPDetector` struct in `runner/serena.go`
  - [x] 2.2 Implement `Available(projectRoot string) bool`:
    - Read `.claude/settings.json` via `os.ReadFile` → `json.Unmarshal` to `map[string]any`
    - Check for Serena MCP config in `mcpServers` key (case-insensitive "serena" substring match)
    - If not found, try `.mcp.json` same approach
    - Any read/parse error → return false (best-effort, no error propagation)
  - [x] 2.3 Implement `PromptHint() string`: return `"If Serena MCP tools available, use them for code navigation"`
  - [x] 2.4 NO `exec.LookPath`, NO subprocess calls — config file detection ONLY
  - [x] 2.5 Compile-time check: `var _ CodeIndexerDetector = (*SerenaMCPDetector)(nil)`

- [x] Task 3: Wire into Runner (AC: #1, #4, #5)
  - [x] 3.1 Add `CodeIndexer CodeIndexerDetector` field to Runner struct
  - [x] 3.2 In `runner.Run` init: if `cfg.SerenaEnabled` → create `&SerenaMCPDetector{}`, else `&NoOpCodeIndexerDetector{}`
  - [x] 3.3 Call `r.CodeIndexer.Available(cfg.ProjectRoot)` at startup, store result; if true, log "Serena MCP detected" (AC: #1)
  - [x] 3.4 If `cfg.SerenaEnabled == false`, skip detection entirely (AC: #5)
  - [x] 3.5 Pass `PromptHint()` result to prompt assembly via `TemplateData.SerenaEnabled` bool (already exists) + Stage 2 replacement for hint text

- [x] Task 4: Inject prompt hint (AC: #3)
  - [x] 4.1 Add `__SERENA_HINT__` placeholder to execute and review prompt templates
  - [x] 4.2 In prompt assembly, if Serena available: replace `__SERENA_HINT__` with `PromptHint()` text
  - [x] 4.3 If Serena unavailable: replace `__SERENA_HINT__` with empty string
  - [x] 4.4 `SerenaEnabled` bool already in `TemplateData` — use for conditional sections in templates

- [x] Task 5: Remove SerenaTimeout (AC: technical notes)
  - [x] 5.1 Remove `SerenaTimeout int` field and `yaml:"serena_timeout"` tag from `config/config.go`
  - [x] 5.2 Remove `serena_timeout: 10` from `config/defaults.yaml`
  - [x] 5.3 Remove any `SerenaTimeout` references in tests (`config/config_test.go`)
  - [x] 5.4 No timeout needed — detection is file-based only (v6 architecture decision)

- [x] Task 6: Tests (AC: all)
  - [x] 6.1 `TestSerenaMCPDetector_Available_SettingsJson` — table-driven: Serena present/absent in .claude/settings.json
  - [x] 6.2 `TestSerenaMCPDetector_Available_McpJson` — fallback to .mcp.json when settings.json missing
  - [x] 6.3 `TestSerenaMCPDetector_Available_BothMissing` — returns false, no error
  - [x] 6.4 `TestSerenaMCPDetector_Available_MalformedJson` — returns false (best-effort)
  - [x] 6.5 `TestSerenaMCPDetector_PromptHint` — returns expected hint string
  - [x] 6.6 `TestNoOpCodeIndexerDetector_Available` — returns false
  - [x] 6.7 `TestNoOpCodeIndexerDetector_PromptHint` — returns empty string
  - [x] 6.8 `TestRunner_SerenaDisabled_SkipsDetection` — verify no file reads when serena_enabled: false
  - [x] 6.9 `TestRunner_SerenaEnabled_InjectsHint` — verify hint in assembled prompt

## Dev Notes

### Architecture & Design Decisions

- **C3 Model (Critical):** Serena — MCP server, НЕ CLI. Ralph НЕ вызывает Serena напрямую. Только детекция через конфигурационные файлы и prompt hint.
- **Минимальный интерфейс (M5):** `CodeIndexerDetector{Available(projectRoot string) bool, PromptHint() string}` — 2 метода, больше ничего.
- **Best-effort detection:** Любая ошибка чтения/парсинга → `Available() = false`. Без propagation ошибок.
- **SerenaTimeout REMOVED:** Поле и default удалены из config — detection is file-based only, timeout не нужен (v6 architecture decision).

### Existing Scaffold

- `config.Config.SerenaEnabled` bool (default: `true`) — уже в `config/config.go:30`
- `config.Config.SerenaTimeout` int (default: `10`) — в `config/config.go:31`, **УДАЛИТЬ** (Task 5)
- `config.TemplateData.SerenaEnabled` bool — уже в `config/prompt.go:27`
- `config/defaults.yaml` — `serena_enabled: true`, `serena_timeout: 10` (**удалить serena_timeout**, Task 5)
- Prompt templates: `{{if .SerenaEnabled}}...{{end}}` conditional already tested in `config/prompt_test.go`
- Runner struct: NO CodeIndexer field yet — нужно добавить

### File Layout

| File | Purpose |
|------|---------|
| `runner/serena.go` | **NEW:** CodeIndexerDetector interface, SerenaMCPDetector, NoOpCodeIndexerDetector |
| `runner/serena_test.go` | **NEW:** All tests for Task 6 |
| `runner/runner.go` | Add `CodeIndexer CodeIndexerDetector` field to Runner struct, wire in Run |
| `runner/prompts/execute.md` | Add `__SERENA_HINT__` placeholder |
| `runner/prompts/review.md` | Add `__SERENA_HINT__` placeholder |

### Detection Logic

```
Available(projectRoot):
  1. path = filepath.Join(projectRoot, ".claude", "settings.json")
  2. os.ReadFile(path) → json.Unmarshal → map[string]any
  3. Check "mcpServers" key for case-insensitive "serena" in any server name
  4. If found → return true
  5. Fallback: path = filepath.Join(projectRoot, ".mcp.json")
  6. Same parse + check
  7. Any error at any step → return false
```

### JSON Config Format Examples

`.claude/settings.json`:
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--project", "."]
    }
  }
}
```

`.mcp.json`:
```json
{
  "mcpServers": {
    "serena-mcp": {
      "command": "npx",
      "args": ["-y", "serena-mcp"]
    }
  }
}
```

Detection: iterate `mcpServers` keys, `strings.Contains(strings.ToLower(key), "serena")`.

### Interface Placement Convention

- `CodeIndexerDetector` in `runner/serena.go` (consumer package = runner)
- NOT in separate `serena/` package (project convention: interfaces in consumer)

### Error Wrapping Convention

```go
// No error wrapping needed — best-effort detection returns bool only
// Internal errors silently return false
```

### Dependency Direction

```
runner/serena.go → os, encoding/json, filepath, strings (stdlib only)
```

No new external dependencies. No config package dependency (detection reads files directly, doesn't need config struct).

### Testing Standards

- Table-driven по умолчанию, Go stdlib assertions
- `t.TempDir()` с .claude/settings.json и .mcp.json test fixtures
- Test JSON files created in tmpDir for isolation
- No golden files needed (bool + string returns)
- Naming: `Test<Type>_<Method>_<Scenario>`
- Coverage: all 6 AC scenarios covered

### Platform Notes (WSL/NTFS)

- JSON file reads via `os.ReadFile` — standard pattern
- Path construction via `filepath.Join` — cross-platform
- Missing file: `os.ReadFile` error → return false (no `os.Stat` pre-check needed)

### Project Structure Notes

- `runner/serena.go` — new file, minimal (~60-80 LOC)
- Interface in consumer package per naming convention
- `NoOpCodeIndexerDetector` for disabled/test scenarios

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.7]
- [Source: docs/project-context.md#Architecture-Ключевые-решения]
- [Source: config/config.go:30-31 — SerenaEnabled, SerenaTimeout fields]
- [Source: config/prompt.go:27 — TemplateData.SerenaEnabled]
- [Source: config/defaults.yaml — serena_enabled: true]
- [Source: runner/runner.go:321-331 — Runner struct definition]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- All 9 Serena test functions pass (0 failures)
- Full regression: all 8 packages pass, 0 regressions
- Build: `go build ./...` clean

### Completion Notes List

- Task 1: CodeIndexerDetector interface with Available(projectRoot) bool + PromptHint() string; NoOpCodeIndexerDetector returns false/empty
- Task 2: SerenaMCPDetector reads .claude/settings.json and .mcp.json, checks mcpServers keys for case-insensitive "serena" substring; best-effort (errors → false); no exec.LookPath
- Task 3: CodeIndexer field on Runner; Run() wires SerenaMCPDetector when SerenaEnabled, NoOp otherwise; detectSerena() at startup logs "Serena MCP detected"; SerenaHint added to RunConfig for review prompt injection
- Task 4: __SERENA_HINT__ placeholder in execute.md and review.md templates; conditional {{if .SerenaEnabled}} blocks; Stage 2 replacement in both Execute and RealReview
- Task 5: Removed SerenaTimeout field from config.Config, serena_timeout from defaults.yaml, 4 test assertions updated in config_test.go
- Task 6: 9 tests: 6-case table-driven settings.json detection, .mcp.json fallback, both missing, malformed JSON, PromptHint value, NoOp Available/PromptHint, disabled skips detection, enabled injects hint

### File List

- `runner/serena.go` — **new**: CodeIndexerDetector interface, SerenaMCPDetector, NoOpCodeIndexerDetector, detectSerena helper
- `runner/serena_test.go` — **new**: 9 test functions covering all ACs
- `runner/runner.go` — modified: CodeIndexer field on Runner, SerenaHint on RunConfig, wired in Run() and Execute(), hint injection in prompt assembly for both execute and review
- `runner/prompts/execute.md` — modified: added {{if .SerenaEnabled}} Code Navigation section with __SERENA_HINT__
- `runner/prompts/review.md` — modified: added {{if .SerenaEnabled}} Code Navigation section with __SERENA_HINT__
- `config/config.go` — modified: removed SerenaTimeout field
- `config/defaults.yaml` — modified: removed serena_timeout: 10
- `config/config_test.go` — modified: removed 4 SerenaTimeout assertions, updated "several fields set" table case
- `docs/sprint-artifacts/sprint-status.yaml` — modified: 6-7 status ready-for-dev → in-progress → review

## Change Log

- 2026-03-02: Implemented Story 6.7 — Serena MCP Integration: CodeIndexerDetector interface, file-based detection, prompt hint injection, SerenaTimeout removal, 9 tests
