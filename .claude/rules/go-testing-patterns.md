---
globs: ["*_test.go", "**/*_test.go"]
---

# Go Testing Patterns — bmad-ralph

Detailed testing patterns from code reviews (Epics 1-3, 93 findings across 28 stories).
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
- Inner error verification: when wrapping errors, test BOTH prefix (`"runner: dirty state recovery:"`) AND inner cause (`"restore failed"`) `[runner/runner_test.go]`
- Multi-layer error wrapping: when function A wraps function B's error, test ALL layers — outer prefix, intermediate prefix, and innermost cause `[runner/runner_test.go:RecoverDirtyStateFails]`

## Assertions

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
- Inner error in ALL table cases: every case with non-sentinel error must have `wantErrContainsInner` — not just sentinel cases `[runner/runner_test.go]`
- Intermediate error in ALL table cases: when table struct has `wantErrContainsIntermediate`, set it in EVERY case where the intermediate layer exists — partial coverage across cases is inconsistent `[runner/runner_test.go:StartupErrors]`
- Disambiguate same-function error prefixes: when a function is called twice in a flow (e.g., HeadCommit before/after), use distinct error prefixes `[runner/runner.go]`
- Verify mock data contents, not just counts: when tracking mock captures `data []T`, assert field values (e.g., `data[0].SessionID == "expected"`), not just `len(data) == 1` `[runner/runner_test.go]`
- Inner error assertion must NOT match outer prefix: if wantErrContainsInner matches the wrapping prefix, it proves nothing about the inner cause — use a unique substring from the actual inner error `[runner/runner_test.go]`

## Test Structure

- No standalone duplicates of table cases — merge into existing table (recurring: Stories 1.3, 1.4, 2.3, 3.2)
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
- DRY test closures: when a closure pattern appears 3+ times, extract to package-level var (stateless) or `func(t) Type` helper (t-dependent) `[runner/test_helpers_test.go]`
- DRY test config: extract `testConfig(tmpDir, maxIter)` helper when `config.Config{}` boilerplate repeats 3+ times with same defaults `[runner/test_helpers_test.go]`
- Initialize ALL injectable function fields on test Runner structs (even if nil won't be hit) — prevents latent nil-pointer panics on mock data changes `[runner/runner_test.go]`
- `Scenario.Name` field: always set on `testutil.Scenario` structs for debugging
- No dead golden files: every testdata fixture must be loaded by at least one test `[session/]`
- No vacuous tests: if a test creates a temp resource but the code under test never references it, the assertion is unfalsifiable — link the test context to the code path `[runner/knowledge_test.go]`
- Extract `runGit(t, dir, args...)` helper for real-git tests — avoids 3+ copies of closure `[runner/git_test.go]`
- Never hardcode default branch name ("master"): use `git rev-parse --abbrev-ref HEAD` after init `[runner/git_test.go]`
- Test ALL indicator file paths: if code checks MERGE_HEAD + rebase-merge + rebase-apply, test at least 2 `[runner/git_test.go]`
- Beyond-length behavior symmetry: if HeadCommit tests beyond-length, HealthCheck must too — test all mock sequence edge cases `[testutil/mock_git_test.go]`

## CLI Testing

- Flag wiring: test `buildCLIFlags` maps to CORRECT struct fields, not just flag existence `[cmd/ralph/]`
- Flag default values: when AC specifies defaults, add `DefValue` assertion `[cmd/ralph/]`
- CLI arg values need constants: flag names AND fixed values should be const `[cmd/ralph/]`

## Code Quality

- Doc comment claims must match reality: "all" = verify exhaustively (recurring: 1.8, 1.10, 2.5, 3.1, 3.2, 3.7)
- Stale doc comments after refactoring: when function behavior changes, update doc comment immediately — includes inline test comments referencing old behavior `[runner/runner.go, runner/runner_test.go]` (recurring: 3.2, 3.3, 3.8)
- Edge case tests must verify ALL struct fields, not just counts — e.g., `Text` field on matched entries `[runner/scan_test.go]`
- Stale API surface comments: update "ONLY entry point" when adding new exports
- Comment counts must match AC: "7 sections" when AC lists 8 = misleading `[runner/prompt_test.go]`
- After removing a placeholder from a code path, update doc comments referencing it `[config/prompt.go]`
- `strings.ReplaceAll` over `strings.Replace(s, old, new, -1)` since Go 1.12
- Group related constants into `const (...)` block, not separate `const` lines
- Remove unused test struct fields immediately (copy-paste remnants)
- Error wrapping consistency: ALL error returns in a function must wrap with same prefix pattern `[runner/runner.go]` (recurring: 3.4)
- Sentinel errors for future flow control: when an error will need `errors.Is` detection in a later story, define sentinel NOW `[runner/git.go]`
- SRP for sentinels: place sentinel errors in the file that owns the concern — git errors in git.go, retry errors in runner.go, cross-package sentinels in config/errors.go `[runner/]`
- No duplicate sentinels: check `config/errors.go` before adding new sentinels — reuse existing cross-package sentinels (e.g., `config.ErrMaxRetries`) instead of defining package-local copies `[runner/git.go→runner.go]`
- Discarded `_` return value in production: document in test comment that related tests verify mock capability, not code path differentiation `[runner/runner_test.go]`

## Template Testing

- `text/template` `missingkey=error`: NO-OP for struct data, only maps `[config/prompt.go]`
- `template.Option("missingkey=error")` format: single string with `=`, not two args (panic)
- Template trim markers `{{- if -}}` must be APPLIED, not just documented `[bridge/prompts/]`
- Negative examples (WRONG format) need dedicated test assertions `[bridge/prompt_test.go]`
- Mutually exclusive conditionals: use `{{if}}/{{else}}/{{end}}`, NOT `{{if}}/{{end}} {{if not}}/{{end}}` `[runner/prompts/execute.md]`
- Full template rewrite = test ALL conditional paths (incl. pre-existing ones like GatesEnabled) `[runner/prompt_test.go]`

## Review Process

- Dev Notes error path claims: trace actual code path to verify coverage `[bridge/bridge.go]`
- yaml.v3 #395 guard: `map[string]any` probe before struct unmarshal `[config/config.go]`
- Generator vs parser spec: separate MUST-requirement from guidance in same prompt
- Continuous bullet lists in LLM prompts: no blank lines between related instructions
- Don't add conditionals not in the AC — extra scope = untested risk
- New regex/constant tests go next to existing ones in same file `[config/constants_test.go]`
- Duplicated content between code and docs needs sync test via `strings.Contains`
- Structural Rule #8 symmetry: both consumer test suites verify same marker set
