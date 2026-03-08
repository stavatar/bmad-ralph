# Violation Tracker

# Scope: updated after each epic retrospective — tracks violation frequency and enforcement escalation

## Violation Frequency (Epics 2-12, 58 stories, 310 findings)

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
- Epic 7 Story 7.1: 7 findings (0C/0H/4M/3L) — Metrics foundation, 2 new patterns (metrics lifecycle completeness, metrics recording on error paths), 5 instances of existing patterns (return value handling, doc accuracy, assertion quality)
- Epic 7 Story 7.2: 5 findings (0C/0H/3M/2L) — Git Diff Stats, 0 new patterns, all match existing: return value handling (M1: fmt.Sscanf discarded), stub method body empty (M2: RecordGitDiff), test coverage gap (M3: binary file path untested), assertion completeness (L1: Packages field), missing integration test (L2)
- Epic 7 Story 7.4: 5 findings (0C/0H/3M/2L) — Review Enrichment, 0 new patterns, all match existing: standalone duplicate test (M1: BackwardCompat=CleanNoFindings), table missing new struct field assertion (M2: Scenarios wantFindingsNil), incomplete mock data verification (M3: RecordReview Findings[1] skipped), same gap in MultipleReviewCycles (L1), defensive ToUpper untested/YAGNI (L2)
- Epic 7 Story 7.3: 5 findings (0C/1H/3M/1L) — Cost Tracking, 0 new patterns, all fixed, all match existing: AC code path coverage (H1: review session cost untracked→fixed via ReviewResult.SessionMetrics), test coverage gap (M1: no ModelPricing yaml test), symmetric negative check (M2: gate cost absent with nil Metrics), test coverage scope (M3: only 1/4 gate sites tested→documented), doc comment accuracy (L1: RecordSession edge cases)
- Epic 7 Story 7.5: 5 findings (0C/0H/4M/1L) — Stuck Detection, 0 new patterns, all fixed, all match existing: doc comment accuracy (M1: Execute doc missing stuck detection, L1: consecutiveNoCommit undocumented), error path test coverage (M2: InjectFeedback failure in stuck — documented coverage gap), assertion quality (M3: RecordRetry positive path untested→fixed), incomplete Dev Agent Record (M4: empty File List/notes→filled)
- Epic 7 Story 7.8: 6 findings (1C/0H/3M/2L) — Similarity Detection, 0 new patterns, all fixed, all match existing: dead feature in production (C1: Run() missing SimilarityDetector init), doc comment accuracy (M1: "consecutive" misleading, L2: constructor doc claim without validation), error path test coverage (M2: hard+no-gates untested), assertion quality (M3: enriched text incomplete), test naming (L1: misleading boundary test name)
- Epic 7 Story 7.9: 5 findings (0C/1H/3M/1L) — Error Categorization + Latency, 2 new patterns (incremental metrics recording, double Finish()), all fixed: double Finish() on MetricsCollector (H1: tests called Finish() after Execute() already finished), stale doc comment on RecordSession (M1), partial latency lost on error returns (M2→refactored to incremental), story doc count mismatch (M3), dead RecordError message parameter (L1)
- Epic 7 Story 7.7: 5 findings (0C/0H/4M/1L) — Budget Alerts, 0 new patterns, all fixed, all match existing: error format inconsistency %f→%.2f (M1), missing dollar amount assertions in hard error test (M2), missing inner error assertion in gate quit test (M3), imprecise gate count assertion (M4), AC5 doc lists distill but no distill RecordSession (L1)
- Epic 7 Story 7.10: 4 findings (0C/0H/2M/2L) — Run Summary Report, 1 new pattern (enum switch completeness), all fixed: missing nil guard test+false test count claim (M1: assertion quality+doc accuracy), enum switch ignores "error" status in aggregates (M2: new pattern), false completion note about var restoration (L1: doc accuracy), test naming convention violation (L2: test naming)
- Epic 8 Story 8.1: 5 findings (0C/0H/3M/2L) — Serena Sync Config+CLI, 0 new patterns, all fixed, all match existing: stale Validate() doc comment (M1: doc accuracy), incomplete full-config test (M2: assertion quality), missing error prefix assertion (M3: assertion quality), File List count inaccuracy (L1: doc accuracy), no beyond-range test case (L2: assertion quality)
- Epic 8 Story 8.2: 5 findings (0C/0H/3M/2L) — Sync Prompt Template, 0 new patterns, all fixed, all match existing: TemplateParse test missing Parse call (M1: assertion quality), weak max turns assertion (M2: assertion quality), missing both-absent test (M3: code path coverage), weak constraint assertion (L1: assertion quality), go fmt not run (L2: gofmt)
- Epic 8 Story 8.3: 5 findings (0C/0H/3M/2L) — Backup/Rollback Memories, 0 new patterns, all fixed, all match existing: weak validateMemories error assertion (M1: assertion quality), missing inner error check + platform-agnostic path (M2: assertion quality + platform-agnostic error), missing rollback error path test (M3: error path coverage), count self-resolved (L1: doc accuracy), missing empty-dir test (L2: assertion quality)
- Epic 8 Story 8.4: 5 findings (0C/0H/3M/2L) — runSerenaSync Core, 0 new patterns, all fixed, all match existing: task marked done but not implemented (M1: doc accuracy), Execute() doc missing sync mention (M2: doc accuracy), missing empty-trigger test (M3: code path coverage), Execute test no opts capture (L1: assertion quality), test count inaccuracy (L2: doc accuracy)
- Epic 8 Story 8.5: 4 findings (0C/0H/3M/1L) — Sync Metrics+Summary, 0 new patterns, all fixed, all match existing: no test for multi-call accumulation (M1: code path coverage), missing line count assertion (M2: assertion quality), weak JSON nested assertions (M3: assertion quality), incomplete struct field verification (L1: assertion quality)
- Epic 8 Story 8.6: 3 findings (0C/0H/2M/1L) — Per-Task Trigger, 0 new patterns, all fixed, all match existing: duplicate test PartialStatus⊂MultipleCalls (M1: duplicate code), stale runSerenaSync doc "after execute loop" (M2: doc accuracy), missing negative assertion in PerTaskScoping (L1: assertion quality)
- Epic 8 Story 8.7: 3 findings (0C/0H/2M/1L) — Integration Tests, 0 new patterns, all fixed, all match existing: duplicate MetricsNilWhenDisabled⊂Disabled (M1: duplicate code), misleading FormatSummaryWithSync name+subset of HappyPath (M2: duplicate code+test naming), Unavailable doc promises log assertion without infrastructure (L1: doc accuracy)
- Epic 9 Story 9.1: 5 findings (0C/0H/3M/2L) — Progressive Review Types, 0 new patterns, all fixed, all match existing: out-of-scope config additions (M1: SRP/YAGNI), TestSeverityLevel_Values not table-driven (M2: duplicate code), missing maxCycles≤0 edge case tests (M3: assertion quality/code path coverage), missing SeverityLevel.String() (L1: API completeness), doc comment boundary asymmetry unexplained (L2: doc accuracy)
- Epic 11 Story 11.1: 5 findings (0C/0H/3M/2L) — Config plan fields, 2 new patterns (magic string defaults, enum field validation): magic string defaults in applyPlanDefaults (M1: new pattern), PlanMode not validated (M2: new pattern), File List missing project-context.md (M3: doc accuracy), no tests for plan stub (L1: test coverage), struct field grouping (L2: code style)
- Epic 11 Story 11.2: 4 findings (0C/0H/3M/1L) — Session InjectFeedback, 1 new pattern (duplicate CLI flag from multiple struct fields): double --append-system-prompt when both fields set (M1: new pattern), test not isolated from AppendSystemPrompt (M2: assertion quality), empty test false positive with AppendSystemPrompt (M3: assertion quality), stale Options doc comment (L1: doc accuracy)
- Epic 11 Story 11.3: 4 findings (0C/0H/3M/1L) — CheckSize pure function, 0 new patterns, all match existing: missing KB value assertion in above threshold test (M1: assertion quality), no empty slice test case (M2: code path coverage), no Russian text assertion in msg (M3: assertion quality), redundant wantEmpty field (L1: duplicate code)
- Epic 11 Story 11.4: 5 findings (0C/0H/4M/1L) — Plan prompt generator, 0 new patterns, all match existing: AC deviation Existing as placeholder not template var (M1: doc accuracy), no error path tests for GeneratePrompt (M2: error path coverage), missing SETUP tag assertion (M3: assertion quality), merge test without golden file (M4: assertion quality), missingkey=error NO-OP for struct (L1: doc accuracy)
- Epic 11 Story 11.5: 4 findings (0C/1H/3M/0L) — Core generate flow, 0 new patterns: os.ReadFile in plan/ violates invariant (H1: architecture violation), defer os.Remove after Rename (M1: code quality), SetupMockClaude error discarded (M2: return value handling), no OutputPath fallback test (M3: code path coverage)
- Epic 11 Story 11.6: 4 findings (0C/0H/3M/1L) — Role mapping + typed headers, 1 new pattern (case-sensitive map lookup): AC4 closing tag missing in template (M1: doc accuracy), no end-to-end ResolveRole→GeneratePrompt test (M2: assertion quality), case-sensitive defaultRoles lookup (M3: new pattern), no empty filename test (L1: assertion quality)
- Epic 11 Story 11.7: 5 findings (0C/0H/4M/1L) — BMad autodiscovery + single-doc, 0 new patterns, all match existing: File List missing bridge.go (M1: doc accuracy), no PlanInputs branch test (M2: code path coverage), no error path tests for discoverInputs (M3: error path coverage), autodiscovery test incomplete inputs[1] assertion (M4: assertion quality), undocumented bridge bug fix (L1: doc accuracy)
- Epic 11 Story 11.8: 5 findings (0C/0H/4M/1L) — CLI flag wiring, 1 new pattern (colon-based path parsing): discarded GetStringArray error (M1: return value handling), SplitN colon breaks Windows drive letters (M2: new pattern), SizeWarning test missing Message assertion (M3: assertion quality), AC7 gate quit unimplemented (M4: AC code path coverage), runPlanWithInputs doc incomplete (L1: doc accuracy)
- Epic 11 Story 11.9: 5 findings (0C/0H/3M/2L) — Progress output + summary, 0 new patterns, all match existing: color.Yellow bypasses cmd.OutOrStdout (M1: assertion quality), silent os.ReadFile error in summary (M2: silent error swallowing), no stdout capture test for progress/summary (M3: AC code path coverage), duplicate size warning tests (L1: duplicate code), dead args=nil assignment (L2: dead variable)
- Epic 12 Story 12.1: 5 findings (0C/1H/3M/1L) — Review prompt, 0 new patterns, all match existing: missing errors import breaks compilation (H1: build failure), ExistingContent dead field + os.ReadFile still in Run (M1: dead parameter), GenerateReviewPrompt duplicates GeneratePrompt assembly (M2: duplicate code), test checks 5/7 checklist items (M3: assertion quality), time.Duration(0) non-idiomatic (L1: code style)

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
