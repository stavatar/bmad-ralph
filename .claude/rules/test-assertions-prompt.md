---
globs: ["*_test.go", "**/*_test.go", "**/prompts/*.md"]
---

# Assertion Patterns — Prompt & Template Tests

# Scope: assertion patterns specific to LLM prompt testing — scope guards, constraints, discriminating assertions

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
