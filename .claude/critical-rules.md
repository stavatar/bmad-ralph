CRITICAL DEVELOPMENT RULES (injected via SessionStart hook — not subject to framing):

1. Test names: Test<Type>_<Method>_<Scenario> — "Type" = real Go type/exported var name
2. Error tests MUST verify message content via strings.Contains, not bare err != nil
3. errors.As(err, &target) not type assertions — project standard
4. Count assertions: strings.Count >= N, not just strings.Contains
5. No standalone duplicates of table-driven test cases — merge into table
6. NEVER discard return values with _ — VIOLATION: Story 3.9 dropped inner error text assertion when enhancing error message, broke error wrapping verification. FIX: capture BOTH (result, error) and assert on BOTH.
7. t.Errorf/t.Fatalf in assertions, NEVER t.Logf (silent pass bug)
8. Every exported function needs dedicated error test
9. Don't add scope/conditionals not mandated by AC — extra code = untested risk
10. Doc comments MUST match code after EVERY change — VIOLATIONS: Story 3.10: comment said "FR24" but code was FR25. Story 3.9: doc said "returns nil on clean state" but function was refactored to return ErrNoRecovery. Story 3.8: RecoverDirtyState doc referenced old behavior. FIX: After ANY code change, re-read EVERY doc comment on modified functions and verify each claim.
11. Error wrapping: fmt.Errorf("pkg: op: %w", err) — ALL returns in a function, not just some. VIOLATION: Story 3.4: 3 of 5 returns wrapped but 2 missed. FIX: grep function for ALL return err lines, verify each wraps.
12. After code-review: MUST update .claude/rules/*.md (new patterns) + memory/MEMORY.md (status)
