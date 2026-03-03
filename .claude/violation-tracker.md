# Violation Tracker

# Scope: updated after each epic retrospective — tracks violation frequency and enforcement escalation

## Violation Frequency (Epics 2-6, 42 stories, 232 findings)

| Category | E2 (7s) | E3 (11s) | E4 (8s) | E5 (6s) | E6 (3s) | Aggregate | Enforcement Tier |
|----------|---------|----------|---------|---------|---------|-----------|-----------------|
| Assertion quality | 7/7 | 11/11 | 8/8 | 6/6 | 5/5 | 37/37 (100%) | T1 SessionStart #2,#4 + T2 rules |
| Doc comment accuracy | 3/7 | 6/11 | 2/8 | 3/6 | 5/5 | 19/37 (51%) | T1 SessionStart #10 |
| Duplicate code | 5/7 | 6/11 | — | 3/5 | 1/1 | 15/24 (63%)* | T1 SessionStart #5,#12 |
| Error wrapping/paths | 4/7 | 4/11 | — | 4/5 | 6/6 | 18/29 (62%)* | T1 SessionStart #11 |
| Return value handling | — | — | 2/8 | — | 1/1 | 3/9 (33%)** | T1 SessionStart #6 |
| SRP/YAGNI | — | 3/11 | — | — | 1/1 | 4/12 (33%)** | T1 SessionStart #9 |
| gofmt after Edit | — | 2/11 | — | — | — | 2/11 (18%)** | T2.5 PreToolUse checklist |
| Prompt scope coverage | — | — | 2/8 | — | — | 2/8 (25%)** | T1 SessionStart #14 |
| Dead parameter/API design | — | — | — | — | 1/1 | 1/1 (new) | T2 code-quality-patterns |
| filepath.Join vs concat | — | — | — | — | 1/1 | 1/1 (new) | T2 code-quality-patterns |
| Variable shadowing std pkg | — | — | — | — | 1/1 | 1/1 (new) | T2 code-quality-patterns |
| Unexported testable helpers | — | — | — | — | 1/1 | 1/1 (new) | T2 code-quality-patterns |

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
- Epic 6 Story 6.1: 8 findings (1C/2H/4M/1L) — FileKnowledgeWriter knowledge_write.go, 4 new patterns (dead param, non-interface method, filepath.Join, silent error swallow)
- Epic 6 Story 6.7: 5 findings (0C/0H/3M/2L) — Serena MCP integration serena.go, 2 new patterns (variable shadowing, unexported helper testability)
- Epic 6 Story 6.2: 5 findings (0C/2H/2M/1L) — Knowledge injection knowledge_read.go, all match existing patterns (error test coverage, doc accuracy, KISS)
- Epic 6 Story 6.3: 4 findings (0C/1H/2M/1L) — Resume extraction knowledge runner.go, all match existing patterns (error path coverage, prompt assertion, table field completeness)
- Epic 6 Story 6.4: 2 findings fixed + 1 downgraded (0C/1H/0M/1L) — Review knowledge review.md+runner.go, patterns: contradictory prompt constraint (H1), stale doc comment (L1), unreachable error path (M1 downgraded)
- Epic 6 Story 6.5a: 4 findings (0C/1H/2M/1L) — Budget check + distillation trigger, patterns: doc comment accuracy (H1), AC code path coverage (M1), fixture location consistency (M2), error path test coverage (L1)
- Epic 6 Story 6.5b: 5 findings (0C/0H/2M/3L) — Distillation session + output parsing, patterns: double error wrapping (M1), T1 promotion dedup test gap (M2), doc comment format mismatch (L1), glob validation separator bug (L2), missing state mutation test (L3)
- Epic 6 Story 6.5c: 3 findings (0C/0H/1M/2L) — Distillation validation + state, patterns: dead variable with misleading comment (M1), log timing before action (L1), error path test coverage for recovery (L2)
- Epic 6 Story 6.6: 4 findings (0C/0H/2M/2L) — Distillation CLI ralph distill, patterns: missing exported doc comment (M1), silent os.Stat non-NotExist error (M2), undocumented error discard (L1×2), misleading log on no-op (L2 noted/no-fix)
- Epic 6 Story 6.8: 2 findings (0C/0H/1M/1L) — Final integration test, patterns: missing table case per AC (M1: empty project), undocumented error discard in test (L1). +1 coverage gap noted: CrashRecovery stderr not asserted (Task 10.6)

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
