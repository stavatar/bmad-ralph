---
globs: ["*_test.go", "**/*_test.go"]
---

# Go Testing Patterns — bmad-ralph (Index)

# Scope: entry point for all testing pattern files — loaded with any test file

Detailed testing patterns split into topic files for focused loading (~15 rules each).
For core rules, see CLAUDE.md `## Testing Core Rules`.

- **test-naming-structure.md** — Test naming conventions + test structure (12 rules)
- **test-error-patterns.md** — Error testing patterns (11 rules)
- **test-assertions-base.md** — Core assertion patterns: counts, substrings, symmetric checks, integration (23 rules)
- **test-assertions-prompt.md** — Prompt/template test assertions: scope guards, constraints, discriminating (12 rules)
- **test-mocks-infra.md** — Mock & test infrastructure + CLI testing (22 rules)
- **code-quality-patterns.md** — Code quality patterns for all Go files (48 rules)
- **test-templates-review.md** — Template testing + review process (14 rules)

Total: ~142 patterns from code reviews (Epics 1-11, 251 findings across 45 stories).
