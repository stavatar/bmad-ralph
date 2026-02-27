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
- Out-of-scope checks need domain keywords + agent names: "acceptance criteria" alone is ambiguous across agents, combine with scope-specific terms `[runner/prompt_test.go]` (Story 4.2)
- Symmetric scope boundary clarification: when 2 agents share a keyword (e.g., "acceptance criteria"), use discriminating co-keywords unique to each agent `[runner/prompt_test.go]` (Story 4.2)
- Constraint instruction assertions: when AC says "mandatory" or "exactly one", assert the constraint TEXT (e.g., "Severity is mandatory") not just the keyword list `[runner/prompt_test.go]` (Story 4.4)
- Ordering/completeness constraint assertions: when AC requires temporal ordering (e.g., "before writing"), assert the ordering instruction text (e.g., "before proceeding to verification") `[runner/prompt_test.go]` (Story 4.4)
- Absence checks use precise phrases: "overwrite review-findings" not generic "overwrite" — single common words cause false failures `[runner/prompt_test.go]` (Story 4.4)
- Scope-creep guard completeness: implement ALL absence guards listed in task spec — don't silently drop items from a multi-item requirement `[runner/prompt_test.go]` (Story 4.5)
- AC "ONLY modifies" constraint assertions: when AC says "ONLY modifies X and Y", assert the exact constraint statement text, not just the file names `[runner/prompt_test.go]` (Story 4.5)
- Test name prefix consistency: all assertions in a story group should use same prefix for grepability (e.g., all "clean*" not mixed "clean/clear") `[runner/prompt_test.go]` (Story 4.5)
- Negative constraint assertions: when AC says "not append" or "never X", assert the negative constraint text (e.g., "never appended") not just the positive ("overwrite") `[runner/prompt_test.go]` (Story 4.6)
- Output format template assertions: when prompt defines an output template (e.g., `### [SEVERITY] Finding title`), assert the template markers that show structure `[runner/prompt_test.go]` (Story 4.6)
- Symmetric file-absent assertions: if CleanReview test checks `review-findings.md` absent at end, ALL tests ending with clean review must too `[runner/runner_review_integration_test.go]` (Story 4.8)
- Error message count assertions: when AC says "cycles count", assert the numeric format (e.g., "3/3") not just the sentinel and task name `[runner/runner_review_integration_test.go]` (Story 4.8)
