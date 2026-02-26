---
globs: ["*_test.go", "**/*_test.go"]
---

# Assertion Patterns

- Count assertions: `strings.Count >= N`, not just `strings.Contains` for multiple instances `[bridge/]`
- Substring assertions must be section-specific: `"acceptance criter"` is ambiguous, use unique substring
- Assertion uniqueness: verify substring doesn't exist in base content before claiming unique to enrichment
- Symmetric negative checks: if TestA checks `"__X__" absent`, TestB (opposite scenario) must too `[runner/prompt_test.go]`
- Symmetric slice nil checks: open-only tests must verify `DoneTasks == nil`, done-only must verify `OpenTasks == nil` `[runner/scan_test.go]`
- ExtraCheck must cover ALL scenario-specific markers, not just one of many `[bridge/bridge_test.go]`
- Full output comparison when mock returns deterministic content — substrings miss corruption
- Separator assertions: `"\n\n---\n\n"` not generic `"---"` `[bridge/bridge_test.go]`
- Cross-validate related counts: `sourceCount == taskOpenCount` not just `> 0`
- Guard between-steps mutations: `if modified == original { t.Fatal }` prevents silent no-ops
- Verify flag values AND presence: `--max-turns` needs value check ("5"), not just exists
- Call count assertions: table-driven tests should include `wantXxxCount` fields for mock call tracking `[runner/runner_test.go]`
- Inner error in ALL table cases: every case with non-sentinel error must have `wantErrContainsInner` `[runner/runner_test.go]`
- Intermediate error in ALL table cases: when table struct has `wantErrContainsIntermediate`, set it in EVERY case `[runner/runner_test.go:StartupErrors]`
- Disambiguate same-function error prefixes: when a function is called twice in a flow, use distinct error prefixes `[runner/runner.go]`
- Verify mock data contents, not just counts: assert field values (e.g., `data[0].SessionID == "expected"`), not just `len(data) == 1` `[runner/runner_test.go]`
- Inner error assertion must NOT match outer prefix: use a unique substring from the actual inner error `[runner/runner_test.go]`
- Integration tests: verify ALL error message layers (sentinel via `errors.Is` AND message content via `strings.Contains`) `[runner/runner_integration_test.go]`
- Integration tests: verify subprocess invocation args (prompt content, flags) via `ReadInvocationArgs` `[runner/runner_integration_test.go]`
