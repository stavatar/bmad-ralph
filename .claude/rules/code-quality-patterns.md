---
globs: ["*.go", "**/*.go"]
---

# Code Quality Patterns

# Scope: production Go code quality — doc comments, error wrapping, DRY, sentinels, return values

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
- Prompt Instructions must cover ALL SCOPE areas: if SCOPE defines 3 areas, Instructions must guide for all 3 `[runner/prompts/agents/test-coverage.md]` (Story 4.2)
- Detection structure tests must check ALL scope dimensions: if agent covers DRY+KISS+SRP, verify all 3 in Instructions `[runner/prompt_test.go]` (Story 4.2)
- Never silently discard return values: `session.ParseResult(raw, elapsed)` with no assignment = error swallowed. Use `_, _ =` and comment why `[runner/runner.go]` (Story 4.3)
- Double-wrap consistency: if callee already wraps with package prefix (e.g., `ScanTasks` → `"runner: scan tasks:"`), caller should pass through without re-wrapping, not add second `"runner:"` prefix `[runner/runner.go]` (Story 4.3)
- Test ALL error return paths: when a function has N error returns, need N test cases — don't skip file-system error paths like non-NotExist on os.ReadFile `[runner/runner_test.go]` (Story 4.3)
- DRY in prompts: keep constraint statements (e.g., "MUST NOT modify source code") in ONE canonical section (Invariants), not duplicated across multiple sections `[runner/prompts/review.md]` (Story 4.5)
- Test helpers that write files must check errors: `os.WriteFile` in ReviewFn closures needs `_ =` with comment or explicit check, not bare discard `[runner/test_helpers_test.go]` (Story 4.7)
- Error-path tests need mock call count guards: verify `HeadCommitCount == 0` and `HealthCheckCount == 1` to detect code path drift `[runner/runner_test.go]` (Story 4.7)
- Rename = update ALL doc comment references: `realReview` → `RealReview` must be updated in ReviewFunc doc and RunReview doc `[runner/runner.go]` (recurring: 3.2, 3.3, 3.8, 3.9, 3.10, 4.8)
- Doc test scenario lists must match actual tests: when listing "covered by" scenarios, list actual test names, separate "not yet covered" clearly `[runner/runner.go:RealReview]` (Story 4.8)
- Post-loop processing guard: when multiple break paths exit a loop with different semantics (clean completion vs emergency skip), guard post-loop code with flags to prevent running completion logic after non-completion exits `[runner/runner.go:Execute]` (Story 5.5)
