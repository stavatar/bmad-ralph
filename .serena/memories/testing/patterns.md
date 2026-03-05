# Testing Patterns

## Go Binary
All test/build commands use: `"/mnt/c/Program Files/Go/bin/go.exe"`
But gopls uses native Linux go at `~/.local/bin/go` (GOROOT=~/.local/go)

## Test Naming
`Test<Type>_<Method>_<Scenario>` — "Type" = real Go type or exported var name
Example: `TestConfig_Validate_MissingStoriesDir`, `TestRunner_Execute_HappyPath`

## Core Patterns
- Table-driven by default, Go stdlib assertions (no testify)
- `t.TempDir()` for filesystem isolation
- `errors.As(err, &target)` not type assertions
- Error tests verify message via `strings.Contains`
- Golden files: `testdata/TestName.golden` with `-update` flag
- Coverage targets: runner and config >80%

## Mock Claude (self-reexec pattern)
1. TestMain checks env var → dispatches to `testutil.RunMockClaude()`
2. Test sets `config.ClaudeCommand = os.Args[0]` + env var
3. Mock reads scenario JSON, returns canned responses
4. `testutil.SetupMockClaude(t, scenarios)` wires it up
5. `testutil.ReadInvocationArgs(t, dir)` verifies prompt/flags

## Test File Organization
- `*_test.go`: unit tests per file
- `*_integration_test.go`: multi-component tests
- `test_helpers_test.go`: shared mocks and helpers (package-level)
- `testmain_test.go`: TestMain for self-reexec dispatch

## WSL/NTFS Test Issues
- `os.MkdirAll` on nonexistent root succeeds on WSL — use file-as-directory trick
- Broken symlinks may not work on WSL/NTFS — use `t.Skipf`
- File-as-directory errors differ: "is a directory" (Linux) vs "Incorrect function." (Windows)
- Use file path in assertion instead of OS-specific message
