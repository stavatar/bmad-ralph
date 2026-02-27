---
globs: ["*_test.go", "**/*_test.go"]
---

# Mock & Test Infrastructure

# Scope: mock infrastructure, test helpers, fixture management, golden files, CLI flag testing

- Mock JSON fidelity: expose `is_error`/`subtype` control, not hardcoded values `[testutil/]`
- Test helper `default` case required in TestMain switch — prevents silent typo pass
- Self-reexec dispatch: env var checks BEFORE `RunMockClaude()` for non-mock modes `[bridge/]`
- Fixture copy boilerplate → extract helper on 2nd occurrence: `copyFixtureToScenario(t, name)`
- DRY test closures: when a closure pattern appears 3+ times, extract to package-level var or `func(t) Type` helper `[runner/test_helpers_test.go]`
- DRY test config: extract `testConfig(tmpDir, maxIter)` helper when `config.Config{}` boilerplate repeats 3+ times `[runner/test_helpers_test.go]`
- Initialize ALL injectable function fields on test Runner structs (even if nil won't be hit) `[runner/runner_test.go]`
- `Scenario.Name` field: always set on `testutil.Scenario` structs for debugging
- No dead golden files: every testdata fixture must be loaded by at least one test `[session/]`
- No vacuous tests: if a test creates a temp resource but code under test never references it, assertion is unfalsifiable `[runner/knowledge_test.go]`
- Extract `runGit(t, dir, args...)` helper for real-git tests — avoids 3+ copies of closure `[runner/git_test.go]`
- Never hardcode default branch name ("master"): use `git rev-parse --abbrev-ref HEAD` after init `[runner/git_test.go]`
- Test ALL indicator file paths: if code checks MERGE_HEAD + rebase-merge + rebase-apply, test at least 2 `[runner/git_test.go]`
- Beyond-length behavior symmetry: if HeadCommit tests beyond-length, HealthCheck must too `[testutil/mock_git_test.go]`
- Track ALL injectable fns exercised in scenario: use `trackingSleep` not `noopSleepFn` — verify call count AND computed values `[runner/runner_integration_test.go]`

## CLI Testing

- Flag wiring: test `buildCLIFlags` maps to CORRECT struct fields, not just flag existence `[cmd/ralph/]`
- Flag default values: when AC specifies defaults, add `DefValue` assertion `[cmd/ralph/]`
- CLI arg values need constants: flag names AND fixed values should be const `[cmd/ralph/]`
