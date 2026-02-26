---
globs: ["*.go", "**/*.go"]
---

# Code Quality Patterns

- Doc comment claims must match reality: "all" = verify exhaustively (recurring: 1.8, 1.10, 2.5, 3.1, 3.2, 3.7)
- Stale doc comments after refactoring: when function behavior changes, update doc comment immediately `[runner/runner.go, cmd/ralph/exit.go]` (recurring: 3.2, 3.3, 3.8, 3.9, 3.10)
- Edge case tests must verify ALL struct fields, not just counts `[runner/scan_test.go]`
- Stale API surface comments: update "ONLY entry point" when adding new exports
- Comment counts must match AC: "7 sections" when AC lists 8 = misleading `[runner/prompt_test.go]`
- After removing a placeholder from a code path, update doc comments referencing it `[config/prompt.go]`
- `strings.ReplaceAll` over `strings.Replace(s, old, new, -1)` since Go 1.12
- Group related constants into `const (...)` block, not separate `const` lines
- Remove unused test struct fields immediately (copy-paste remnants)
- Error wrapping consistency: ALL error returns in a function must wrap with same prefix pattern `[runner/runner.go]` (recurring: 3.4)
- Sentinel errors for future flow control: when an error will need `errors.Is` detection later, define sentinel NOW `[runner/git.go]`
- SRP for sentinels: place sentinel errors in the file that owns the concern `[runner/]`
- No duplicate sentinels: check `config/errors.go` before adding new sentinels `[runner/git.go→runner.go]`
- Run `go fmt` after editing Go files: Edit tool doesn't auto-format `[runner/runner.go]` (Story 3.9)
- When enhancing error messages, don't drop existing inner error text assertions `[runner/runner_test.go]` (Story 3.9)
- New sentinels must be added to existing sentinel unwrap test table in `config/errors_test.go` (Story 3.10)
- AC references in code comments must match the actual AC they describe `[runner/runner.go]` (Story 3.10)
