---
globs: ["*_test.go", "**/*_test.go"]
---

# Test Naming & Structure

# Scope: test function naming, test structure, table-driven organization

## Naming

- `Test<Type>_<Method>_<Scenario>` — "Type" must be real Go type or exported var name `[config/errors_test.go]`
- Zero-value tests: one function per type, split `TestFoo_ZeroValue` + `TestBar_ZeroValue` `[config/errors_test.go]`
- Table-driven function names need scenario suffix: `TestFoo_EdgeCases` not bare `TestFoo`
- Test case names within a function: consistent style (all spaces OR all hyphens, not mixed)

## Structure

- No standalone duplicates of table cases — merge into existing table (recurring: Stories 1.3, 1.4, 2.3, 3.2)
- All-fields comprehensive test when testing multi-field override patterns `[config/config_test.go]`
- Test ALL code path combinations: don't leave diagonal gaps in branch matrices `[config/]`
- Parallel regex test symmetry: paired patterns need symmetric test cases `[config/constants_test.go]`
- Consistent negative check patterns: `present bool` struct field, not `if c.name == "..."` matching
- Integration test coverage: Load() must cover all detectProjectRoot paths `[config/]`
