CRITICAL DEVELOPMENT RULES (injected via SessionStart hook — not subject to framing):

1. Test names: Test<Type>_<Method>_<Scenario> — "Type" = real Go type/exported var name
2. Error tests MUST verify message content via strings.Contains, not bare err != nil
3. errors.As(err, &target) not type assertions — project standard
4. Count assertions: strings.Count >= N, not just strings.Contains
5. No standalone duplicates of table-driven test cases — merge into table
6. NEVER leave return values uncaptured — covers BOTH `_ =` discard AND missing assignment. VIOLATION: Story 3.9 dropped inner error text assertion with `_`. Story 4.3: `session.ParseResult(raw, elapsed)` with no LHS = error silently swallowed. FIX: capture BOTH (result, error) and assert on BOTH. Use `_, _ =` with comment if intentionally ignoring.
7. t.Errorf/t.Fatalf in assertions, NEVER t.Logf (silent pass bug)
8. Every exported function needs dedicated error test
9. Don't add scope/conditionals not mandated by AC — extra code = untested risk
10. Doc comments MUST match code after EVERY change — VIOLATIONS: Story 3.10: comment said "FR24" but code was FR25. Story 3.9: doc said "returns nil on clean state" but function was refactored to return ErrNoRecovery. Story 3.8: RecoverDirtyState doc referenced old behavior. Story 4.8: rename realReview→RealReview but doc still said realReview. FIX: After ANY code change, re-read EVERY doc comment on modified functions and verify each claim.
11. Error wrapping: fmt.Errorf("pkg: op: %w", err) — ALL returns in a function, not just some. VIOLATION: Story 3.4: 3 of 5 returns wrapped but 2 missed. FIX: grep function for ALL return err lines, verify each wraps.
12. Test ALL error return paths: when a function has N error returns, need N test cases. VIOLATION: Story 4.3: file-system error path on os.ReadFile skipped. FIX: count return statements with err, write test for each.
13. DRY threshold: extract helper on 2nd occurrence, not 3rd. VIOLATION: Stories 3.6-3.8 had 3+ copies of test closures before extraction. FIX: when you copy-paste a block, immediately extract to helper.
14. Prompt scope completeness: N SCOPE items = N Instruction sections. VIOLATION: Story 4.2: SCOPE defined DRY+KISS+SRP but Instructions only covered DRY+KISS. FIX: enumerate SCOPE items, verify each has corresponding Instructions.
15. After code-review: MUST update .claude/rules/*.md (new patterns) + memory/MEMORY.md (status)
