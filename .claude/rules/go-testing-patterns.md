---
globs: ["*_test.go", "**/*_test.go"]
---

# Go Testing Patterns — bmad-ralph

Detailed testing patterns from code reviews (Epics 1-2, 40 findings across 20 stories).
For core rules, see CLAUDE.md `## Testing Core Rules`.

## Test Naming

- `Test<Type>_<Method>_<Scenario>` — "Type" must be real Go type or exported var name `[config/errors_test.go]`
- Zero-value tests: one function per type, split `TestFoo_ZeroValue` + `TestBar_ZeroValue` `[config/errors_test.go]`
- Table-driven function names need scenario suffix: `TestFoo_EdgeCases` not bare `TestFoo`
- Test case names within a function: consistent style (all spaces OR all hyphens, not mixed)

## Error Testing

- `errors.As` tests must be table-driven with multiple field combinations `[config/errors_test.go]`
- Always test zero values for custom error types — catches uninitialized field bugs
- Test double-wrapped errors: `fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", err))` `[config/errors.go]`
- Test ALL error paths: non-NotExist os.ReadFile errors (permission, is-a-directory) need tests `[config/config.go]`
- Test `context.DeadlineExceeded` alongside `context.Canceled` — distinct `errors.Is` behavior `[cmd/ralph/]`
- Every exported function needs dedicated error test — not just tested inside HappyPath `[session/]`
- Discarded `_` RawResult kills stderr assertions: ALWAYS capture when testing errors `[session/]`
- When string matching on errors is unavoidable (yaml.v3), add justification comment `[config/config.go]`

## Assertions

- Count assertions: `strings.Count >= N`, not just `strings.Contains` for multiple instances `[bridge/]`
- Substring assertions must be section-specific: `"acceptance criter"` is ambiguous, use unique substring
- Assertion uniqueness: verify substring doesn't exist in base content before claiming unique to enrichment
- ExtraCheck must cover ALL scenario-specific markers, not just one of many `[bridge/bridge_test.go]`
- Full output comparison when mock returns deterministic content — substrings miss corruption
- Separator assertions: `"\n\n---\n\n"` not generic `"---"` `[bridge/bridge_test.go]`
- Cross-validate related counts: `sourceCount == taskOpenCount` not just `> 0`
- Guard between-steps mutations: `if modified == original { t.Fatal }` prevents silent no-ops
- Verify flag values AND presence: `--max-turns` needs value check ("5"), not just exists

## Test Structure

- No standalone duplicates of table cases — merge into existing table (recurring: Stories 1.3, 1.4, 2.3)
- All-fields comprehensive test when testing multi-field override patterns `[config/config_test.go]`
- Test ALL code path combinations: don't leave diagonal gaps in branch matrices `[config/]`
- Parallel regex test symmetry: paired patterns need symmetric test cases `[config/constants_test.go]`
- Consistent negative check patterns: `present bool` struct field, not `if c.name == "..."` matching
- Integration test coverage: Load() must cover all detectProjectRoot paths `[config/]`

## Mock & Test Infrastructure

- Mock JSON fidelity: expose `is_error`/`subtype` control, not hardcoded values `[testutil/]`
- Test helper `default` case required in TestMain switch — prevents silent typo pass
- Self-reexec dispatch: env var checks BEFORE `RunMockClaude()` for non-mock modes `[bridge/]`
- Fixture copy boilerplate → extract helper on 2nd occurrence: `copyFixtureToScenario(t, name)`
- `Scenario.Name` field: always set on `testutil.Scenario` structs for debugging
- No dead golden files: every testdata fixture must be loaded by at least one test `[session/]`

## CLI Testing

- Flag wiring: test `buildCLIFlags` maps to CORRECT struct fields, not just flag existence `[cmd/ralph/]`
- Flag default values: when AC specifies defaults, add `DefValue` assertion `[cmd/ralph/]`
- CLI arg values need constants: flag names AND fixed values should be const `[cmd/ralph/]`

## Code Quality

- Doc comment claims must match reality: "all" = verify exhaustively (recurring: 1.8, 1.10, 2.5)
- Stale API surface comments: update "ONLY entry point" when adding new exports
- `strings.ReplaceAll` over `strings.Replace(s, old, new, -1)` since Go 1.12
- Group related constants into `const (...)` block, not separate `const` lines
- Remove unused test struct fields immediately (copy-paste remnants)

## Template Testing

- `text/template` `missingkey=error`: NO-OP for struct data, only maps `[config/prompt.go]`
- `template.Option("missingkey=error")` format: single string with `=`, not two args (panic)
- Template trim markers `{{- if -}}` must be APPLIED, not just documented `[bridge/prompts/]`
- Negative examples (WRONG format) need dedicated test assertions `[bridge/prompt_test.go]`

## Review Process

- Dev Notes error path claims: trace actual code path to verify coverage `[bridge/bridge.go]`
- yaml.v3 #395 guard: `map[string]any` probe before struct unmarshal `[config/config.go]`
- Generator vs parser spec: separate MUST-requirement from guidance in same prompt
- Continuous bullet lists in LLM prompts: no blank lines between related instructions
- Don't add conditionals not in the AC — extra scope = untested risk
- New regex/constant tests go next to existing ones in same file `[config/constants_test.go]`
- Duplicated content between code and docs needs sync test via `strings.Contains`
- Structural Rule #8 symmetry: both consumer test suites verify same marker set
