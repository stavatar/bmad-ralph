# Story 1.10: Prompt Assembly Utility

Status: done

## Story

As a developer,
I want a two-stage prompt assembly utility (text/template + strings.Replace),
so that all epics use a single mechanism for building Claude prompts. **Interface contract freeze after this story.**

## Acceptance Criteria

1. `AssemblePrompt(tmplContent string, data TemplateData, replacements map[string]string) (string, error)` — exact frozen signature
2. Stage 1: `text/template` processes structural placeholders (`{{.Variable}}`, `{{if .BoolField}}`)
3. Stage 2: `strings.Replace` injects user content safely — template engine does NOT re-process
4. User content containing `{{.Malicious}}` injected via Stage 2 remains literal text (no template injection)
5. `TemplateData` struct fields: `SerenaEnabled bool`, `GatesEnabled bool`, `TaskContent string`, `LearningsContent string`, `ClaudeMdContent string`, `FindingsContent string`
6. Golden file tests verify: simple template, conditional `{{if .SerenaEnabled}}`, user content with `{{`, empty replacements, invalid template syntax (descriptive error)
7. Stage 2 replacements applied in deterministic order (sorted by key)
8. Replacements MUST NOT contain other replacement placeholders (flat, not recursive)
9. Function lives in `config/` package (cross-cutting utility, leaf constraint — MUST NOT import other project packages)
10. Interface contract: `AssemblePrompt()` signature is FROZEN after this story

## Tasks / Subtasks

- [x] Task 1: Create `config/prompt.go` with `TemplateData` struct and `AssemblePrompt()` function (AC: 1, 2, 3, 4, 5, 7, 8, 9)
  - [x] 1.1 Define `TemplateData` struct with all 6 fields (doc comments per field explaining Stage 1 vs Stage 2 role)
  - [x] 1.2 Implement Stage 1: `text/template.New("prompt").Parse(tmplContent)` → `Execute` into `bytes.Buffer` with `data`
  - [x] 1.3 Decide `missingkey` behavior: use default (silent `<no value>`) or `template.Option("missingkey", "error")` for strict mode — document decision in code comment
  - [x] 1.4 Implement Stage 2: sort `replacements` keys, iterate sorted, apply `strings.Replace(result, key, value, -1)`
  - [x] 1.5 Error wrapping: `fmt.Errorf("config: assemble prompt: %w", err)` for template parse/execute errors
  - [x] 1.6 Only stdlib imports: `bytes`, `fmt`, `sort`, `strings`, `text/template`
- [x] Task 2: Create `config/prompt_test.go` with table-driven tests and golden files (AC: 6, 10)
  - [x] 2.1 Create golden file infrastructure (NEW — config package has none yet): `var update = flag.Bool("update", false, "update golden files")` + `func TestMain(m *testing.M) { flag.Parse(); os.Exit(m.Run()) }`
  - [x] 2.2 Test: simple template with one `{{.SerenaEnabled}}` variable → golden file
  - [x] 2.3 Test: conditional template `{{if .SerenaEnabled}}...{{end}}` → golden file
  - [x] 2.4 Test: user content containing `{{.Malicious}}` injected via Stage 2 — verify literal output → golden file
  - [x] 2.5 Test: empty replacements map (Stage 2 is no-op)
  - [x] 2.6 Test: invalid template syntax (parse error) → descriptive error with `strings.Contains` verification
  - [x] 2.7 Test: template.Execute error (e.g., `{{call .BadField}}`) — distinct from parse error
  - [x] 2.8 Test: all fields simultaneously (max complexity — both bools true, all string fields populated, multiple replacements) → golden file
  - [x] 2.9 Test: nil replacements map (edge case — should behave same as empty)
  - [x] 2.10 Test: zero-value — `AssemblePrompt("", TemplateData{}, nil)` → empty string, no error
  - [x] 2.11 Test: replacements with deterministic order verification (keys sorted — output differs if unsorted)
  - [x] 2.12 Golden files: `testdata/TestAssemblePrompt_Simple.golden`, `TestAssemblePrompt_Conditional.golden`, `TestAssemblePrompt_Injection.golden`, `TestAssemblePrompt_AllFields.golden`
- [x] Task 3: Verify all constraints and update story (AC: 9, 10)
  - [x] 3.1 Run tests: `"/mnt/c/Program Files/Go/bin/go.exe" test ./config/ -v`
  - [x] 3.2 Verify no new external dependencies (`go mod tidy` should not change go.mod)
  - [x] 3.3 Verify `config/prompt.go` imports only stdlib packages
  - [x] 3.4 Fix CRLF: `sed -i 's/\r$//' config/prompt.go config/prompt_test.go`
  - [x] 3.5 Update story file with completion notes

## Dev Notes

### Architecture Context

**Two-Stage Prompt Assembly** — critical architectural decision from `docs/architecture/core-architectural-decisions.md`:

- **Stage 1 (`text/template`):** Processes structural placeholders — bool conditionals (`{{if .SerenaEnabled}}`). Safe because TemplateData is code-controlled.
- **Stage 2 (`strings.Replace`):** Injects user-controlled content (LEARNINGS.md, CLAUDE.md, review-findings.md, task descriptions). These may contain `{{` characters. Applying `strings.Replace` AFTER template processing means user content is never evaluated as Go templates.

**Why two stages:** User files written by Claude sessions may legitimately contain `{{` syntax. Without separation, `text/template` would crash or execute arbitrary template code.

**Package: `config/`** — Cross-cutting utility used by runner, bridge, and future packages. MUST NOT import other project packages (leaf constraint). If violated in future → extract to `internal/prompt/`.

**Interface freeze:** `AssemblePrompt()` signature FROZEN after this story. `TemplateData` struct IS extensible — adding new fields is not a breaking change (Go struct compatibility). Future epics (bridge needs `HasExistingTasks bool`, etc.) can extend fields freely.

### TemplateData Field Roles (CRITICAL)

| Field type | Stage | Usage | Example |
|------------|-------|-------|---------|
| `bool` fields | Stage 1 | Template conditionals | `{{if .SerenaEnabled}}...{{end}}` |
| `string` fields | Stage 2 | NOT used via `{{.FieldName}}` in templates | Callers put content in replacements map |

**DANGER:** Templates MUST NOT reference string fields via `{{.TaskContent}}` etc. when content is user-controlled. If TaskContent contains `{{`, `text/template.Execute` will fail. String fields exist in the struct for type grouping and caller convenience — actual injection happens via Stage 2 `replacements` map.

**Stage 2 placeholder convention:** Use double-underscore markers in templates: `__TASK_CONTENT__`, `__LEARNINGS__`, `__CLAUDE_MD__`, `__FINDINGS__`. Callers pass `{"__TASK_CONTENT__": actualContent}` in replacements.

### missingkey Behavior Decision

Go `text/template` defaults to replacing unknown fields with `<no value>` (no error). Decide one of:
- **Default (silent):** Unknown fields output `<no value>` — lenient, matches Go stdlib behavior
- **Strict:** `template.Option("missingkey", "error")` — catches typos in templates, fails fast

Document the chosen behavior in a code comment. This is part of the frozen contract — changing it later would alter output for existing templates.

### Implementation Guidance

**Error wrapping** (matches `config/config.go` pattern):
```go
fmt.Errorf("config: assemble prompt: parse: %w", err)   // Stage 1 parse
fmt.Errorf("config: assemble prompt: execute: %w", err)  // Stage 1 execute
```

**Required imports for `config/prompt.go`:**
```go
import (
    "bytes"
    "fmt"
    "sort"
    "strings"
    "text/template"
)
```

**Stage 1 sketch** (adapt for edge cases, don't copy blindly):
```go
tmpl, err := template.New("prompt").Parse(tmplContent)
// ... error handling ...
var buf bytes.Buffer
err = tmpl.Execute(&buf, data)
// ... error handling ...
```

**Stage 2 sketch:**
```go
result := buf.String()
keys := make([]string, 0, len(replacements))
for k := range replacements { keys = append(keys, k) }
sort.Strings(keys)
for _, k := range keys { result = strings.Replace(result, k, replacements[k], -1) }
```

### Golden File Infrastructure (NEW — first in config package)

`config/testdata/` currently has only `.gitkeep`. This story creates golden file testing from scratch.

**Required setup in `config/prompt_test.go`:**
```go
var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
    flag.Parse()
    os.Exit(m.Run())
}
```

**Golden file read/compare pattern:**
```go
golden := filepath.Join("testdata", tt.goldenFile)
if *update {
    if err := os.WriteFile(golden, []byte(got), 0644); err != nil {
        t.Fatal(err)
    }
    return
}
want, err := os.ReadFile(golden)
if err != nil {
    t.Fatalf("read golden: %v (run with -update to create)", err)
}
if got != string(want) {
    t.Errorf("output mismatch:\ngot:\n%s\nwant:\n%s", got, string(want))
}
```

**Note:** Existing `config/config_test.go` does NOT have golden files or `-update` flag. This is new infrastructure.

### Testing Standards

- **Table-driven** with `t.Run` for all test cases
- **Golden files** in `config/testdata/TestAssemblePrompt_*.golden`
- **Error tests** MUST verify message content: `strings.Contains(err.Error(), "config: assemble prompt")`
- **Two error categories:** parse errors (bad syntax) AND execute errors (template execution failure) — test both
- **No testify** — stdlib assertions: `if got != want { t.Errorf(...) }`
- **Test naming:** `TestAssemblePrompt_<Scenario>` — function name as type
- **Test name style:** use spaces consistently (not hyphens) in case names
- **CRLF fix** after creating files: `sed -i 's/\r$//' config/prompt.go config/prompt_test.go`
- **Coverage:** config package must stay >80%
- **Windows Go path:** `"/mnt/c/Program Files/Go/bin/go.exe"` for all go commands

### Previous Story Intelligence (Story 1.9)

- Test naming consistency: all case names within one function must use same style — use spaces
- Completion note counts: verify actual count via `go test -v` — don't estimate
- TestMain pattern IS needed here (for `-update` golden flag), unlike 1.9

### Existing Code to Reference

- **`config/config.go`** — error wrapping pattern (`"config: <op>: %w"`), doc comment style
- **`config/constants.go`** — package-scope constants/vars pattern
- **`config/errors.go`** — sentinel errors pattern
- **`config/config_test.go`** — table-driven tests, stdlib assertions, helper functions (`intPtr`, `strPtr`)

### Project Structure Notes

- **New files (added):** `config/prompt.go`, `config/prompt_test.go`, `config/testdata/TestAssemblePrompt_*.golden`
- **No modified production files** — new utility, no changes to existing code
- **Alignment:** follows `config/` package structure (`config.go`, `constants.go`, `errors.go` pattern)

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.10] — AC, user story, prerequisites
- [Source: docs/architecture/core-architectural-decisions.md#File I/O & Prompt Assembly] — two-stage decision, security rationale
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Naming Patterns] — test naming, error wrapping
- [Source: docs/architecture/project-structure-boundaries.md#Complete Project Directory Structure] — file locations, golden file locations
- [Source: docs/project-context.md#Двухэтапная Prompt Assembly] — critical architecture summary
- [Source: docs/prd/functional-requirements.md#FR29] — prompt files loaded into session context
- [Source: docs/epics/epic-2-story-to-tasks-bridge-stories.md] — bridge caller expectations
- [Source: docs/epics/epic-3-core-execution-loop-stories.md] — runner caller expectations (execute/review prompts)

## Senior Developer Review (AI)

**Review Date:** 2026-02-25
**Reviewer:** Claude Opus 4.6 (adversarial code review)
**Outcome:** Approve (all issues fixed)

### Action Items

- [x] [HIGH] `missingkey=error` doc comment misleading — option is no-op for struct data, corrected documentation
- [x] [MED] Missing test for `{{.NonexistentField}}` — added `execute error unknown struct field` test case
- [x] [MED] Story File List: story md listed as "(modified)" but git shows `??` (new) — fixed to "(new/added)"
- [x] [MED] Bare `TestAssemblePrompt` lacks scenario suffix — renamed to `TestAssemblePrompt_EdgeCases`
- [x] [LOW] `strings.Replace(s, old, new, -1)` → `strings.ReplaceAll` — more idiomatic
- [x] [LOW] Template whitespace behavior undocumented — added note about `{{- if -}}` trim markers
- [x] [LOW] `TestAssemblePrompt_AllFields` string fields — noted as intentional completeness test (no change)

**Summary:** 7 findings total. 6 code/doc fixes applied, 1 accepted as-is. All tests pass, 95.5% coverage maintained.

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- Initial `template.Option("missingkey", "error")` caused panic — Go requires `"missingkey=error"` format (single string with `=`). Fixed immediately.

### Completion Notes List

- Implemented `TemplateData` struct with 6 fields (2 bool for Stage 1, 4 string for Stage 2) with comprehensive doc comments explaining two-stage roles
- Implemented `AssemblePrompt()` with exact frozen signature per AC 1
- Stage 1: `text/template` with `missingkey=error` strict mode (documented in code comment as part of frozen contract)
- Stage 2: deterministic `strings.Replace` with sorted keys per AC 7
- Error wrapping follows `config:` package pattern: `config: assemble prompt: parse:` and `config: assemble prompt: execute:`
- Only stdlib imports: `bytes`, `fmt`, `sort`, `strings`, `text/template` — verified per AC 9
- Golden file infrastructure created: `TestMain` + `-update` flag + `goldenTest` helper
- 11 test scenarios across 5 test functions covering all ACs: simple, conditional, injection safety, all fields, empty/nil replacements, zero value, parse error, execute error, deterministic order
- 4 golden files generated: Simple, Conditional, Injection, AllFields
- Config package coverage: 95.5% (well above >80% requirement)
- Full regression suite passes: `./...` all green
- `go mod tidy` no changes — no new dependencies
- CRLF fixed on all new files
- Total tests: 12 new test cases (4 standalone golden + 8 table-driven) — updated after review

### Change Log

- 2026-02-25: Implemented Story 1.10 — two-stage prompt assembly utility (`config/prompt.go`, `config/prompt_test.go`, 4 golden files)
- 2026-02-25: Code review — 7 findings (1H/3M/3L), all fixed automatically

### File List

- config/prompt.go (new/added)
- config/prompt_test.go (new/added)
- config/testdata/TestAssemblePrompt_Simple.golden (new/added)
- config/testdata/TestAssemblePrompt_Conditional.golden (new/added)
- config/testdata/TestAssemblePrompt_Injection.golden (new/added)
- config/testdata/TestAssemblePrompt_AllFields.golden (new/added)
- docs/sprint-artifacts/sprint-status.yaml (modified)
- docs/sprint-artifacts/1-10-prompt-assembly-utility.md (new/added)
