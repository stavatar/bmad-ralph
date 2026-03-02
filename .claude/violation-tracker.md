# Violation Tracker

# Scope: updated after each epic retrospective — tracks violation frequency and enforcement escalation

## Violation Frequency (Epics 2-5, 32 stories, 190 findings)

| Category | E2 (7s) | E3 (11s) | E4 (8s) | E5 (6s) | Aggregate | Enforcement Tier |
|----------|---------|----------|---------|---------|-----------|-----------------|
| Assertion quality | 7/7 | 11/11 | 8/8 | 6/6 | 32/32 (100%) | T1 SessionStart #2,#4 + T2 rules |
| Doc comment accuracy | 3/7 | 6/11 | 2/8 | 3/6 | 14/32 (44%) | T1 SessionStart #10 |
| Duplicate code | 5/7 | 6/11 | — | 3/5 | 14/23 (61%)* | T1 SessionStart #5,#12 |
| Error wrapping/paths | 4/7 | 4/11 | — | 4/5 | 12/23 (52%)* | T1 SessionStart #11 |
| Return value handling | — | — | 2/8 | — | 2/8 (25%)** | T1 SessionStart #6 |
| SRP/YAGNI | — | 3/11 | — | — | 3/11 (27%)** | T1 SessionStart #9 |
| gofmt after Edit | — | 2/11 | — | — | 2/11 (18%)** | T2.5 PreToolUse checklist |
| Prompt scope coverage | — | — | 2/8 | — | 2/8 (25%)** | T1 SessionStart #14 |

\* Tracked in E2-E3 only. \** Tracked in subset of epics.

## Escalation Thresholds

| Frequency | Tier | Mechanism |
|-----------|------|-----------|
| 1-2 occurrences | T2 (Topic) | `.claude/rules/<topic>.md` |
| 3-5 occurrences | T1.5 (Core) | `CLAUDE.md` core rules |
| 6+ occurrences | T1 (Critical) | `.claude/critical-rules.md` (SessionStart) |
| Deterministic | T2.5 (Active) | PostToolUse / PreToolUse hook |

## Enforcement Tiers

| Tier | Mechanism | Survives Compaction? | Count |
|------|-----------|---------------------|-------|
| 1 (Critical) | SessionStart → critical-rules.md | Yes | ~15 rules |
| 1.5 (Core) | CLAUDE.md | Partially | ~25 rules |
| 2 (Topic) | .claude/rules/*.md (glob-scoped) | No | ~112 rules |
| 2.5 (Active) | PreToolUse + PostToolUse hooks | Yes | 6 checks + CRLF fix |
| 3 (Review) | Code review workflow | N/A | All knowledge |

## Trend

- Epic 2: 5.7 avg findings/story (3H/19M/18L)
- Epic 3: 6.27 avg (0H/36M/33L) — R1-R4 applied after retro, eliminated HIGH
- Epic 4: 5.0 avg (0H/22M/18L) — R1-R4 hooks delivered measurable improvement
- Epic 5: 6.2 avg FINAL (0H/28M/13L across 6 stories) — gates package + runner integration, 9 new pattern categories (all new territory, 0 repeats from E4)

## Update Process

After each epic retrospective:
1. Count violation frequency per category from review findings
2. Update the frequency table above
3. Check if any category crossed an escalation threshold
4. If threshold crossed: promote rule to higher enforcement tier
5. If category reaches 0% for 2+ epics: consider demoting to lower tier

## Resolved

| Category | Resolution | Since |
|----------|-----------|-------|
| CRLF line endings | PostToolUse hook auto-fix (deterministic) | E3 retro |
