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
