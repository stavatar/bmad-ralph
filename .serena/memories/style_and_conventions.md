# Code Style & Conventions

## Naming
- **Interfaces**: defined in consumer package (`GitClient` in `runner/`, not `git/`)
- **Sentinel errors**: `var ErrNoTasks = errors.New("no tasks")`
- **Error wrapping**: `fmt.Errorf("pkg: op: %w", err)` — ALL returns in a function
- **Tests**: `Test<Type>_<Method>_<Scenario>` — "Type" = real Go type or exported var name
- **Golden files**: `testdata/TestName.golden`

## Testing
- Table-driven by default, Go stdlib assertions (no testify)
- `t.TempDir()` for filesystem isolation
- `errors.As(err, &target)` not type assertions
- Error tests MUST verify message content via `strings.Contains`
- Coverage targets: runner and config >80%
- Mock Claude: scenario-based JSON via `config.ClaudeCommand`
- Self-reexec pattern for subprocess mocking (TestMain + env var)

## Code Quality
- No unnecessary dependencies (only 3 direct deps, new deps require justification)
- `strings.ReplaceAll` over `strings.Replace(s, old, new, -1)`
- Group related constants into `const (...)` block
- `filepath.Join` not string concatenation for paths
- Run `go fmt` after editing Go files

## Error Handling
- Packages return errors, never `os.Exit` (exit codes only in `cmd/ralph/`)
- `errors.As` for type checking, `errors.Is` for sentinel checking
- Multi-layer wrapping with distinct prefixes per layer

## Doc Comments
- Must match code after EVERY change
- Verify "all"/"every" assertions exhaustively
- Update immediately when function behavior changes
