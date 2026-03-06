# bmad-ralph — Project Overview

## Purpose
Go CLI tool that orchestrates Claude Code sessions for autonomous development ("Ralph Loop").
It manages iterative AI-driven development cycles with code review, human gates, and knowledge management.

## Tech Stack
- **Language**: Go 1.25
- **Module**: `github.com/bmad-ralph/bmad-ralph`
- **Dependencies** (only 3 direct): cobra (CLI), yaml.v3, fatih/color
- **Build**: Makefile + goreleaser v2
- **CI**: GitHub Actions, golangci-lint v2

## Environment
- **Platform**: WSL on Windows NTFS
- **Go binary**: `"/mnt/c/Program Files/Go/bin/go.exe"` (Windows Go 1.26.0, NOT in WSL PATH)
- CRLF auto-fixed by PostToolUse hook (`.claude/hooks/fix-crlf.sh`)
- `.gitattributes` enforces LF on git add

## Project Status
All 8 epics complete (FR1-FR66): Foundation, Bridge, Core Loop, Code Review, Human Gates, Knowledge Management, Observability & Metrics, Serena Memory Sync.
Total: 69 stories, ~400+ acceptance criteria, ~137 testing/code quality patterns.
Post-epic: Session log saving added — all Claude sessions (execute, review, resume, sync) write stdout/stderr to `.ralph/logs/sessions/<runID>/`.

## Package Structure (dependency direction: top-down, cycles forbidden)
```
cmd/ralph/     → CLI entry point, exit codes, cobra commands (run, bridge, distill)
runner/        → Core execution loop, prompts, git, scanning, knowledge mgmt, metrics, similarity, session logging
bridge/        → Bridge mode (single-shot sessions)
session/       → Claude session management, result parsing
gates/         → Human gate prompts and decisions
config/        → Configuration (leaf package, depends on nothing), pricing
internal/testutil/ → Test infrastructure, mock Claude, mock git, scenarios
```

## Key Architectural Rules
- `cmd/ralph → runner → session, gates, config` / `cmd/ralph → bridge → session, config`
- `config` = leaf package (depends on nothing)
- `session` and `gates` do NOT depend on each other
- Exit codes ONLY in `cmd/ralph/`. Packages return errors, never `os.Exit`
- `config.Config` parsed once, passed by pointer, NEVER mutated at runtime
- `MetricsCollector` nil-safe injectable: nil receiver = no-op, no nil checks in callers
