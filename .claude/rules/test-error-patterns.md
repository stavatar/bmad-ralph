---
globs: ["*_test.go", "**/*_test.go"]
---

# Error Testing Patterns

- `errors.As` tests must be table-driven with multiple field combinations `[config/errors_test.go]`
- Always test zero values for custom error types — catches uninitialized field bugs
- Test double-wrapped errors: `fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", err))` `[config/errors.go]`
- Test ALL error paths: non-NotExist os.ReadFile errors (permission, is-a-directory) need tests `[config/config.go]`
- Test `context.DeadlineExceeded` alongside `context.Canceled` — distinct `errors.Is` behavior `[cmd/ralph/]`
- Every exported function needs dedicated error test — not just tested inside HappyPath `[session/]`
- Discarded `_` RawResult kills stderr assertions: ALWAYS capture when testing errors `[session/]`
- When string matching on errors is unavoidable (yaml.v3), add justification comment `[config/config.go]`
- Inner error verification: test BOTH prefix (`"runner: dirty state recovery:"`) AND inner cause (`"restore failed"`) `[runner/runner_test.go]`
- Multi-layer error wrapping: test ALL layers — outer prefix, intermediate prefix, and innermost cause `[runner/runner_test.go:RecoverDirtyStateFails]`
- Platform-agnostic inner error assertions: file-as-directory produces "is a directory" on Linux but "Incorrect function." on Windows/WSL — use file path in assertion instead of OS message `[runner/runner_test.go:FindingsReadError]` (Story 4.7)
