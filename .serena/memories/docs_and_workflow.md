# Documentation & Workflow

## Documentation Layout
- `docs/project-context.md` — Condensed architecture context (primary reference)
- `docs/prd/` — Product requirements documents
- `docs/architecture/` — Architecture decisions
- `docs/epics/` — Epic definitions: `epic-N-*.md`
- `docs/sprint-artifacts/` — Sprint tracking + story files
  - `sprint-status.yaml` — Sprint status tracker
  - `<key>.md` — Individual story files (e.g., `6-1-fileknowledgewriter-learnings-md.md`)
  - `epic-N-retro-*.md` — Epic retrospectives
- `docs/research/` — Research documents (R1-R7)
- `docs/reviews/` — Code review artifacts
- `docs/analysis/` — Analysis documents

## BMad Workflow (via .bmad/ config)
- Config: `.bmad/bmm/config.yaml` (user: Степан, lang: Русский)
- Workflows: dev-story, code-review, create-story, sprint-planning, retrospective
- 4-agent pipeline: creator → validator → developer → reviewer

## Project Status (as of 2026-03-04)
- Epics 1-6: DONE (all stories implemented, reviewed, retro'd)
- Total: ~226 findings across 40+ stories, 100% fix rate
- Finding trend: 5.7 → 6.27 → 5.0 → 6.2 → 4.2 avg/story
- 132 testing/code quality patterns cataloged in .claude/rules/
