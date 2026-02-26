---
globs: ["*_test.go", "**/*_test.go", "**/prompts/*.md"]
---

# Template Testing & Review Process

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
