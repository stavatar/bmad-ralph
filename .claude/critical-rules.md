CRITICAL DEVELOPMENT RULES (injected via SessionStart hook — not subject to framing):

1. Test names: Test<Type>_<Method>_<Scenario> — "Type" = real Go type/exported var name
2. Error tests MUST verify message content via strings.Contains, not bare err != nil
3. errors.As(err, &target) not type assertions — project standard
4. Count assertions: strings.Count >= N, not just strings.Contains
5. No standalone duplicates of table-driven test cases — merge into table
6. Always capture return values for assertion, never discard with _
7. t.Errorf/t.Fatalf in assertions, NEVER t.Logf (silent pass bug)
8. Every exported function needs dedicated error test
9. Don't add scope/conditionals not mandated by AC — extra code = untested risk
10. Doc comment claims must match reality — verify "all"/"every" exhaustively
11. Error wrapping: fmt.Errorf("pkg: op: %w", err) — ALL returns in a function, not just some
12. After Write/Edit on NTFS: sed -i 's/\r$//' <file>
