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
- Dead parameters: if a function accepts a struct but ignores its fields, the API is misleading — either use the data or restructure the signature `[runner/knowledge_write.go:ValidateNewLessons]` (Story 6.1)
- Non-interface methods break testability: if a method needs to be called through an interface field, it MUST be on the interface — concrete-only methods can't be mocked or injected `[runner/knowledge_write.go:ValidateNewLessonsWithSnapshot]` (Story 6.1)
- Use `filepath.Join` not string concatenation for path construction — mixed separators on Windows/WSL `[runner/knowledge_write.go:learningsPath]` (Story 6.1)
- Silent error swallowing: `os.ReadFile` errors should distinguish `os.IsNotExist` (expected) from other errors (permission, is-a-directory) — don't return zero-value for all errors `[runner/knowledge_write.go:BudgetCheck]` (Story 6.1)
- Variable names must not shadow standard packages: `var config map[string]any` shadows `config` package — use `parsed`, `data`, or domain-specific name `[runner/serena.go:hasSerenamcp]` (Story 6.7)
- Export helpers that need cross-package testing: unexported `detectSerena` couldn't be tested from `runner_test` — export as `DetectSerena` when function is part of public API flow `[runner/serena.go]` (Story 6.7)
- Double-wrap prevention: when callee wraps with full prefix (e.g., `"runner: distill: backup:"`), caller must NOT re-wrap with same prefix — pass through directly `[runner/knowledge_distill.go:AutoDistill]` (Story 6.5b)
- Filter-then-join pattern for loop output: when building separator-delimited output with skip conditions, filter valid items first then join — `continue` in loop body creates malformed separators `[runner/knowledge_distill.go:writeCategoryFile]` (Story 6.5b)
- Log timing: log "Recovered from X" AFTER recovery action succeeds, not before — premature log misleads if recovery fails `[runner/knowledge_state.go:RecoverDistillation]` (Story 6.5c)
- Dead variables with misleading discard: `_ = newNF // used implicitly` is worse than no variable — if value is unused, don't compute it `[runner/knowledge_distill.go:ValidateDistillation]` (Story 6.5c)
- Metrics lifecycle completeness: StartTask must always have a matching FinishTask on ALL code paths (including skip/error exits) — orphaned accumulator silently loses partial data `[runner/runner.go:Execute]` (Story 7.1)
- Metrics recording on error paths: when session fails (ExitError) but produces parseable output with usage data, record metrics before retry — tokens consumed in failed sessions must be tracked `[runner/runner.go:Execute]` (Story 7.1)
- Incremental metrics recording: when accumulating per-phase data (latency, costs) in a loop with early error returns, record incrementally after each measurement rather than collecting into a local struct and recording once at the end — local struct data is lost on error returns `[runner/runner.go:execute]` (Story 7.9)
- Double Finish() on metrics collector: tests must use the RunMetrics returned by Execute(), not call Finish() again — second call only works because internal state isn't cleared, making tests fragile `[runner/runner_test.go:LatencyRecorded]` (Story 7.9)
- Enum switch completeness: when a switch categorizes values (e.g., task status → aggregate counts), all possible values must be handled — auto-generated statuses like "error" falling through silently causes incorrect totals `[runner/metrics.go:Finish]` (Story 7.10)
- Magic string defaults: when a function sets default values for config fields, use exported constants instead of inline string/int literals — enables test assertions and consumer code to reference the same source of truth `[config/config.go:applyPlanDefaults]` (Story 11.1)
- Enum field validation: when config field has a finite set of valid values (e.g., PlanMode: "bmad"|"single"|"auto"), validate in Validate() — invalid values cause unpredictable runtime behavior `[config/config.go:Validate]` (Story 11.1)
- Duplicate CLI flag from multiple struct fields: when two Options fields map to the same CLI flag (e.g., AppendSystemPrompt and InjectFeedback both use `--append-system-prompt`), document mutual exclusion or merge values — duplicate flags have undefined CLI behavior `[session/session.go:buildArgs]` (Story 11.2)
- Case-sensitive map lookup on user-provided paths: when map keys are lowercase filenames but lookup uses `filepath.Base` (preserves case), mismatches occur on case-sensitive FS — normalize with `strings.ToLower` before lookup `[plan/plan.go:defaultRoles]` (Story 11.6)
- Colon-based string parsing on paths: `strings.SplitN(s, ":", 2)` breaks on Windows drive letters (`C:\path:role` → File=`C`). Use `strings.LastIndex` or document Unix-only constraint `[cmd/ralph/plan.go:parseInput]` (Story 11.8)
- Dead type after refactoring: when a custom error type is introduced in story N and the flow is refactored in story N+1 (e.g., ErrReviewIssues no longer returned after retry logic added), verify the type is still used or remove it — stale types with misleading doc comments create confusion `[plan/plan.go:ErrReviewIssues]` (Story 12.3)
