# LLM Prompt Templates

## Prompt Assembly Flow
1. Go templates in `runner/prompts/*.md` and `bridge/prompts/*.md`
2. `config.AssemblePrompt(templateStr, TemplateData)` renders with Go `text/template`
3. `TemplateData` struct controls conditional sections (GatesEnabled, HasFindings, etc.)
4. Validation: `unreplacedPlaceholderRe` catches leftover `__PLACEHOLDER__` markers

## Runner Prompts (runner/prompts/)
- `execute.md` — Main execution prompt. Conditionals: GatesEnabled, HasExistingTasks, HasFindings, HasLearnings, SerenaEnabled
- `review.md` — Code review prompt. Conditionals: HasFindings (re-review vs fresh), HasLearnings
- `distill.md` — Knowledge distillation prompt. Used by AutoDistill

## Review Agent Sub-Prompts (runner/prompts/agents/)
- `quality.md` — Code quality agent (DRY, error handling, naming)
- `implementation.md` — Implementation correctness agent
- `simplification.md` — KISS/SRP simplification agent  
- `design-principles.md` — Design principles agent
- `test-coverage.md` — Test coverage agent (DRY+KISS+SRP scope)

## Bridge Prompt (bridge/prompts/)
- `bridge.md` — Single-shot bridge mode prompt

## Template Variables (TemplateData fields)
- Boolean flags: SerenaEnabled, GatesEnabled, HasExistingTasks, HasFindings, HasLearnings
- Content strings: TaskContent, LearningsContent, ClaudeMdContent, FindingsContent, StoryContent, ExistingTasksContent

## Testing Prompts
- Golden file tests in `runner/prompt_test.go`
- Content assertions: scope guards, constraint text, absence checks
- Discriminating assertions for agent scope boundaries
