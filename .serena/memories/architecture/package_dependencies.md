# Package Dependencies & Structure

## Dependency Direction (strictly top-down, cycles forbidden)
```
cmd/ralph → runner → session, gates, config
cmd/ralph → bridge → session, config
```

## Package Roles
- `cmd/ralph/` — CLI entry point (cobra), exit codes, flag wiring. Exit codes ONLY here.
- `runner/` — Core execution loop, orchestrates Claude sessions iteratively
- `bridge/` — Single-shot bridge mode (one Claude session, no loop)
- `session/` — Claude subprocess execution and result parsing
- `gates/` — Human gate prompts (approve/retry/skip/quit)
- `config/` — Leaf package (depends on NOTHING). Config parsing, constants, errors, prompt assembly
- `internal/testutil/` — Mock Claude, mock git, test scenarios

## Key Interfaces (defined in consumer package)
- `runner.GitClient` — HealthCheck, HeadCommit, RestoreClean (impl: ExecGitClient)
- `runner.KnowledgeWriter` — WriteProgress, ValidateNewLessons (impl: FileKnowledgeWriter, NoOpKnowledgeWriter)
- `runner.CodeIndexerDetector` — Available, PromptHint (impl: SerenaMCPDetector, NoOpCodeIndexerDetector)
- `runner.ReviewFunc` — type alias for review function injection
- `runner.GatePromptFunc` — type alias for gate prompt injection
- `runner.DistillFunc` — type alias for distillation function injection

## Critical Rules
- `config.Config` parsed once, passed by pointer, NEVER mutated at runtime
- Packages return errors, never `os.Exit`
- Interfaces in consumer package, not provider
