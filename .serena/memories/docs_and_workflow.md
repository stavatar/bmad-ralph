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

## Project Status (as of 2026-03-05)
- ALL 7 EPICS COMPLETE (FR1-FR56 delivered, roadmap finished)
- Epics 1-7: DONE (62 stories implemented, reviewed)
- Epic 7: Observability & Metrics (10 stories, 49 findings, 0 HIGH, 100% fix rate)
- Total: ~275 findings across 52+ stories, 100% fix rate
- Finding trend: 5.7 → 6.27 → 5.0 → 6.2 → 4.2 → 4.9 avg/story
- ~137 testing/code quality patterns cataloged in .claude/rules/
