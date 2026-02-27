# CLAUDE.md — bmad-ralph

## Knowledge Extraction Protocol (MANDATORY after every code-review)

After every code-review workflow completes, you MUST update these four locations:
1. **`.claude/rules/<topic>.md`** — new testing/code quality patterns (see index in `go-testing-patterns.md`)
2. **`.claude/rules/wsl-ntfs.md`** — new WSL/NTFS-specific patterns (if any)
3. **`memory/MEMORY.md`** — project status (completed story, next story, metrics update)
4. **`.claude/violation-tracker.md`** — update violation counts after retro (see escalation thresholds)

New learnings go to the appropriate rules file, NOT this CLAUDE.md. This file stays under ~80 lines.

## Project Overview

Go CLI tool orchestrating Claude Code sessions for autonomous development (Ralph Loop).
See `docs/project-context.md` for full architecture context.

## Environment (WSL on Windows)

- **Go binary**: `"/mnt/c/Program Files/Go/bin/go.exe"` (Windows Go 1.26.0, NOT in WSL PATH)
- Filesystem is Windows NTFS mounted via WSL — see `.claude/rules/wsl-ntfs.md` for patterns

## Critical: Line Endings

- CRLF auto-fixed by PostToolUse hook (`.claude/hooks/fix-crlf.sh`) — no manual sed needed
- `.gitattributes` enforces LF on git add, but disk files remain CRLF until hook fixes them
- Verify: `file <filename>` — must say "ASCII text" not "with CRLF line terminators"

## Critical: .gitignore

- Always use leading `/` for root-anchored patterns — `/ralph` not `ralph`
- Verify before commit: `git check-ignore -v <path>`

## Go Module Management

- Module: `github.com/bmad-ralph/bmad-ralph`, go.mod uses `go 1.25`
- Only 3 direct deps: cobra, yaml.v3, fatih/color. New deps require justification
- `go mod tidy` removes deps without imports — blank imports retain during scaffold

## Architecture Rules

- **Dependency direction** (strictly top-down, cycles forbidden):
  `cmd/ralph → runner → session, gates, config` / `cmd/ralph → bridge → session, config`
- `config` = leaf package (depends on nothing)
- `session` and `gates` do NOT depend on each other
- Exit codes ONLY in `cmd/ralph/`. Packages return errors, never `os.Exit`
- `config.Config` parsed once, passed by pointer, NEVER mutated at runtime

## Naming Conventions

- Interfaces in consumer package (`GitClient` in `runner/`, not `git/`)
- Sentinel errors: `var ErrNoTasks = errors.New("no tasks")`
- Error wrapping: `fmt.Errorf("pkg: op: %w", err)` — ALL returns in a function, not just some
- Tests: `Test<Type>_<Method>_<Scenario>` — "Type" = real Go type or exported var name
- Golden files: `testdata/TestName.golden`

## Testing Core Rules

- Table-driven by default, Go stdlib assertions (no testify), `t.TempDir()` for isolation
- Golden files with `-update` flag. Mock Claude: scenario-based JSON via `config.ClaudeCommand`
- Coverage: runner and config >80%
- `errors.As(err, &target)` not type assertions — project standard
- Error tests MUST verify message content via `strings.Contains`, not bare `err != nil`
- Count assertions: `strings.Count >= N` when AC specifies multiple instances
- No standalone duplicates of table-driven test cases — merge into table
- Always capture return values, never discard with `_` — breaks assertion coverage
- `t.Errorf`/`t.Fatalf` in assertions, NEVER `t.Logf` (silent pass bug)
- Every exported function needs dedicated `Test<Func>_<Scenario>` error test
- Don't add scope/conditionals not mandated by AC — extra code = untested risk
- Doc comment claims must match reality — verify "all"/"every" assertions
- See `.claude/rules/go-testing-patterns.md` for index of ~111 detailed patterns (7 topic files)

## Build & CI

- `make build` → `go build -o ralph ./cmd/ralph`
- CI: GitHub Actions, Go 1.25 matrix, golangci-lint v2 (7 linters)
- goreleaser v2: `.goreleaser.yaml`, linux/darwin, amd64/arm64, CGO_ENABLED=0

## BMad Workflow Notes

- Sprint tracking: `docs/sprint-artifacts/sprint-status.yaml`
- Story/epic files: `docs/sprint-artifacts/<key>.md`, `docs/epics/epic-N-*.md`
- Communication language: Русский
