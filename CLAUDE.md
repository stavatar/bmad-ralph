# CLAUDE.md — bmad-ralph

## MANDATORY: Knowledge Extraction After Story Review

**After every code-review workflow completes, you MUST extract learnings into this file.**

This is non-negotiable. Before marking the review session complete:
1. Identify patterns, conventions, or pitfalls discovered during review
2. Add them to the relevant section of this CLAUDE.md (Testing, Architecture Rules, Naming, etc.)
3. Update `memory/MEMORY.md` project status (completed stories, next story)
4. Remove or update any outdated entries

This prevents the same review findings from recurring in future stories.

## Project Overview

Go CLI tool orchestrating Claude Code sessions for autonomous development (Ralph Loop).
See `docs/project-context.md` for full architecture context.

## Environment (WSL on Windows)

- **Go binary**: `/mnt/c/Program Files/Go/bin/go.exe` (Windows Go 1.26.0, NOT in WSL PATH)
- Always use full path: `"/mnt/c/Program Files/Go/bin/go.exe"` for all go commands
- Filesystem is Windows NTFS mounted via WSL — affects line endings and file permissions

## Critical: Line Endings (CRLF Problem)

- **Write tool on Windows NTFS creates CRLF files.** This BREAKS Makefile and can break shell scripts.
- After creating/editing ANY file with Write/Edit tools, run: `sed -i 's/\r$//' <file>`
- `.gitattributes` enforces LF on `git add`, but files on disk remain CRLF until converted
- Verify with: `file <filename>` — should say "ASCII text" (not "with CRLF line terminators")
- Architecture requirement: UTF-8, no BOM, `\n` line endings (project-context.md)

## Critical: .gitignore Patterns

- **Always use leading `/` for root-anchored patterns.** Without `/`, pattern matches at ANY depth.
- Example: `/ralph` ignores only root binary. `ralph` would also ignore `cmd/ralph/` directory.
- Before committing, verify critical files aren't ignored: `git check-ignore -v <path>`

## Go Module Management

- `go mod tidy` removes dependencies without imports — during scaffold phase, blank imports (`_ "pkg"`) in main.go retain deps
- Module path: `github.com/bmad-ralph/bmad-ralph`
- go.mod uses `go 1.25` (no patch version)
- Only 3 direct deps allowed: cobra, yaml.v3, fatih/color. New deps require justification.

## Architecture Rules (from project-context.md)

- **Dependency direction** (strictly top-down, cycles forbidden):
  ```
  cmd/ralph → runner → session, gates, config
  cmd/ralph → bridge → session, config
  ```
- `config` = leaf package (depends on nothing)
- `session` and `gates` do NOT depend on each other
- Exit codes ONLY in `cmd/ralph/`. Packages return errors, never call `os.Exit`
- `config.Config` parsed once at startup, passed by pointer, NEVER mutated at runtime

## Naming Conventions

- Interfaces in consumer package (e.g., `GitClient` in `runner/`, not in `git/`)
- Sentinel errors: `var ErrNoTasks = errors.New("no tasks")`
- Error wrapping: `fmt.Errorf("pkg: op: %w", err)`
- Tests: `Test<Type>_<Method>_<Scenario>`
- Golden files: `testdata/TestName.golden`

## Testing

- Table-driven by default, Go stdlib assertions (no testify)
- Golden files with `-update` flag
- `t.TempDir()` for isolation
- Mock Claude: scenario-based JSON via `config.ClaudeCommand`
- Coverage: runner and config >80%
- **Naming strictly enforced:** `Test<Type>_<Method>_<Scenario>` — "Type" must be a real Go type or exported var name (e.g., `TestErrNoTasks_Is_...`, NOT `TestSentinelErrors_Is_...`)
- **Always test zero values** for custom error types (e.g., `ExitCodeError{}`, `GateDecision{}`) — catches uninitialized field bugs
- **Test double-wrapped errors** (`fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", err))`) — realistic multi-layer call stack scenario
- **errors.As tests must be table-driven** with multiple field combinations, not single inline assertions
- **Test ALL error paths:** non-happy-path os.ReadFile errors (permission denied, is-a-directory) need explicit tests — don't assume only NotExist matters
- **yaml.v3 #395 guard:** Use `map[string]any` probe before struct unmarshal — `bytes.TrimSpace == "---"` is fragile, misses comments-only and multi-document YAML
- **Integration test coverage:** Load() tests must cover all detectProjectRoot paths (.ralph/, .git/ fallback, CWD), not just the primary path
- **String matching on errors:** When unavoidable (yaml.v3 doesn't export syntax error type), add justification comment explaining why errors.Is/As isn't possible
- **No standalone duplicates of table cases:** If a standalone test is a subset of an existing table-driven test, merge it — recurring issue caught in Stories 1.3 and 1.4
- **All-fields comprehensive test:** When testing multi-field override/cascade patterns (e.g., CLI flags), always include a test exercising ALL fields simultaneously
- **Error tests must verify message content:** Bare `err != nil` is insufficient — always verify error wrapping prefix and key identifiers with `strings.Contains`. Matches codebase pattern from Load error tests
- **Test ALL code path combinations:** When function has multiple branch points (e.g., UserHomeDir success/fail x embedded present/absent), cover the full matrix — don't leave diagonal gaps
- **Windows Go `os.UserHomeDir()`:** Uses `USERPROFILE` (not `HOME`). Tests must `t.Setenv` both `HOME` and `USERPROFILE`. For failure: also clear `HOMEDRIVE`/`HOMEPATH`
- **Story File List accuracy:** NEW files = "new/added", not "modified". Check `git status` — `??` = untracked = new
- **Parallel regex test symmetry:** When testing parallel patterns (e.g., TaskOpenRegex/TaskDoneRegex), ensure both have symmetric test cases — tab-indented, embedded marker, malformed marker. Asymmetry = coverage gap
- **Use `errors.As` not type assertions:** Project standard requires `errors.As(err, &target)` instead of `err.(*Type)` — type assertion breaks if error wrapping changes in future Go versions. Caught in Story 1.7 review
- **CLI arg values need constants too:** Not just flag names (`--output-format`) but also fixed values (`"json"`) should be const. AC says "constructed via constants (not inline strings)" — applies to both flag names and their fixed values
- **Test helper `default` case required:** When using TestMain self-reexec pattern with `switch scenario`, always add `default: os.Exit(1)` — silent success on scenario typos masks test bugs
- **Windows Go exec testing:** `go.exe` cannot execute bash scripts (`%1 is not a valid Win32 application`). Use Go test binary self-reexec pattern: `TestMain` + env var (`SESSION_TEST_HELPER`) + `os.Args[0]` as Command. Standard Go stdlib approach
- **Windows path comparison:** Use `os.SameFile()` instead of string equality for path comparison — Windows 8.3 short names (e.g., `4689~1`) differ from long names
- **Doc comments must match actual behavior:** If implementation deviates from spec (e.g., truncated JSON becomes fallback not error), update doc comments AND document deviation in story. Caught in Story 1.8 review
- **No dead golden files:** Every testdata fixture must be loaded by at least one test. Dead fixtures create false confidence. Caught in Story 1.8 — result_empty.json was created but unused
- **Remove unused test struct fields:** Orphan fields like `wantNilOK bool` in test structs indicate copy-paste remnants. Clean immediately
- **Test `is_error: true` from Claude CLI:** When parsing JSON with boolean error flags, add explicit test case documenting behavior when flag is true, even if current code ignores it. Documents a design decision
- **json.Unmarshal cannot distinguish truncated JSON from non-JSON:** Both fail the same way. If spec says "truncated JSON = error", that requires heuristic detection (starts with `[`). Accept fallback behavior and document the deviation rather than adding fragile heuristics
- **Stale API surface comments:** When adding new exported functions to a package, check existing comments that claim "ONLY entry point" or similar exclusivity. Update or remove them
- **`text/template` `missingkey` is map-only:** `missingkey=error` option has NO effect on struct data — struct field resolution always errors on unknown fields regardless. Don't misattribute behavior in doc comments
- **`template.Option` format:** Use `"missingkey=error"` (single string with `=`), NOT `"missingkey", "error"` (two args → panic)
- **`strings.ReplaceAll` over `strings.Replace`:** Prefer `strings.ReplaceAll(s, old, new)` over `strings.Replace(s, old, new, -1)` — more idiomatic since Go 1.12
- **Table-driven test function naming:** Bare function names like `TestFoo` need scenario suffix — `TestFoo_EdgeCases` or `TestFoo_TableDriven`. Standalone tests (`TestFoo_Simple`) already follow convention
- **Discarded `_` RawResult breaks error assertions:** When testing subprocess errors, ALWAYS capture the RawResult — stderr message is in the RawResult, not in `err.Error()`. Discarding with `_` makes stderr content unverifiable. Caught in Story 1.11 review
- **`t.Logf` vs `t.Errorf` in assertions:** `t.Logf` only logs, it does NOT fail the test. Always use `t.Errorf` or `t.Fatalf` inside assertion blocks. `t.Logf` inside an `if !condition` is a silent pass. Caught in Story 1.11 review
- **Zero-value test naming must reflect ALL tested types:** If `TestFoo_ZeroValue` tests both `Foo` and `Bar`, split into `TestFoo_ZeroValue` + `TestBar_ZeroValue`. One test function per type per naming convention
- **Mock JSON fidelity: include `is_error`/`subtype` control:** When mocking CLI tools that distinguish error vs success in JSON fields, expose control in the scenario struct. Hardcoded `"success"` breaks fidelity for error-path tests
- **Every exported function needs dedicated error test:** If `RunReview` is exported, it needs `TestRunReview_<Scenario>` — testing it only inside another function's HappyPath leaves error paths uncovered. Caught in Story 1.12 review
- **Duplicate test helpers are still duplication:** Identical function bodies with different names (e.g., `assertArgsContainFlag` vs `assertArgsContainStandaloneFlag`) must be merged. Caught in Story 1.12 review
- **Template trim markers: APPLY, don't just document:** Story 1.10 documented `{{- if -}}` trim markers for blank line prevention. Story 1.12 repeated the same mistake in `execute.md`. When a learning exists, apply it proactively in new code
- **CLI flag wiring needs dedicated test:** `TestRunCmd_Flags` checking flag existence is insufficient — must also test `buildCLIFlags` maps each flag to the CORRECT CLIFlags struct field. Field swap bugs are invisible to name-only tests. Caught in Story 1.13 review
- **Flag default values must be tested:** When AC specifies exact defaults, add `DefValue` assertion alongside type check. Untested defaults can drift silently
- **Error wrapping consistency in helper functions:** ALL error returns in a function must wrap consistently with `fmt.Errorf("pkg: op: %w", err)` — don't leave the last return bare. Caught in Story 1.13 `ensureLogDir`
- **Test `context.DeadlineExceeded` alongside `context.Canceled`:** They are distinct errors with different `errors.Is` behavior. Always test both even if architecture says one shouldn't occur — defensive coverage
- **WSL/NTFS: `os.MkdirAll` on nonexistent root paths succeeds:** Use file-as-directory-component trick for guaranteed MkdirAll failure in tests, not `/nonexistent/...` paths
- **New regex/constant tests go next to existing ones:** When adding a new regex pattern to `constants.go`, its test belongs in `constants_test.go` alongside the other regex tests — not in a separate file. Breaks discoverability otherwise. Caught in Story 2.1 review
- **Duplicated content between code and docs needs sync test:** When a pattern string (like a regex) appears in both Go code and an embedded markdown doc, add a `strings.Contains` test case verifying the doc contains the exact pattern string. Prevents stale documentation
- **Structural Rule #8 symmetry:** When testing a shared contract from both consumer packages, both test suites should verify the SAME set of markers. Asymmetric marker coverage (e.g., 8 in config, 4 in bridge) defeats the purpose of cross-package validation

## Build & CI

- `make build` → `go build -o ralph ./cmd/ralph`
- CI: GitHub Actions, Go 1.25 matrix (1.26 added when stable)
- golangci-lint v2 with 7 linters (govet, errcheck, staticcheck, unused, gosimple, ineffassign, typecheck)
- goreleaser v2: `.goreleaser.yaml` (not `.yml`), linux/darwin, amd64/arm64, CGO_ENABLED=0

## BMad Workflow Notes

- Sprint tracking: `docs/sprint-artifacts/sprint-status.yaml`
- Story files: `docs/sprint-artifacts/<story-key>.md`
- Epics sharded in `docs/epics/`, PRD in `docs/prd/`, Architecture in `docs/architecture/`
- Communication language: Русский
