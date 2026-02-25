# Epic 4: Code Review Pipeline — Stories

**Scope:** FR13, FR14, FR15, FR16, FR17, FR18, FR18a, FR37, FR38
**Stories:** 8
**Release milestone:** v0.1 после завершения этого эпика

**Invariants (из Epics Structure Plan + Epic 3):**
- **Review atomicity:** `[x]` + clear review-findings.md = atomic (write both or neither)
- **Mutation Asymmetry:** Review sessions write `[x]` and findings. Execute sessions MUST NOT write `[x]`
- **FR17 scope Epic 4:** review = `[x]` + findings write/clear ONLY. Lessons (LEARNINGS.md, CLAUDE.md) **deferred to Epic 6**
- **FR16a (severity filtering):** Growth — not in Epic 4. All CONFIRMED findings block pipeline
- **ReviewResult contract (Story 3.5):** `func(ctx, ReviewOpts) (ReviewResult, error)` — `ReviewResult{Clean bool, FindingsPath string}`
- **review_cycles counter:** already built in Epic 3 Story 3.10 — Epic 4 replaces stub with real review
- **Bottleneck:** findings quality determines everything — adversarial tests critical

---

### Story 4.1: Review Prompt Template

**User Story:**
Как система, я хочу review-промпт который инструктирует Claude Code запустить 4 параллельных sub-агента через Task tool, верифицировать их findings, и записать результат, чтобы обеспечить качественное ревью кода.

**Acceptance Criteria:**

```gherkin
Scenario: Review prompt assembled with all required sections
  Given config with project root and sprint-tasks path
  When review prompt is assembled via text/template + strings.Replace
  Then prompt instructs Claude to launch 4 sub-agents via Task tool (FR15)
  And prompt names sub-agents: quality, implementation, simplification, test-coverage
  And prompt instructs verification of sub-agent findings (FR16)
  And prompt instructs CONFIRMED/FALSE POSITIVE classification
  And prompt instructs severity assignment: CRITICAL/HIGH/MEDIUM/LOW
  And prompt instructs atomic [x] + clear on clean review (FR17)
  And prompt instructs findings write on non-clean review (FR17)
  And prompt contains instruction: MUST NOT modify source code (FR17)
  And prompt contains instruction: MUST NOT write LEARNINGS.md or CLAUDE.md (Epic 6)

Scenario: Review prompt includes sprint-tasks-format contract
  Given shared sprint-tasks-format.md (Story 2.1)
  When review prompt is assembled
  Then sprint-tasks-format content injected via strings.Replace
  And review knows exact format for [x] marking

Scenario: Golden file snapshot matches baseline
  Given review prompt template in runner/prompts/review.md
  When assembled with test fixture data
  Then output matches runner/testdata/TestPrompt_Review.golden
  And golden file updateable via `go test -update`

Scenario: Adversarial golden file — planted bug detection
  Given review prompt + test fixture with code containing deliberately planted bug
  When review prompt instructs sub-agents
  Then prompt structure enables detection of planted bugs
  And golden file captures prompt that should trigger finding

Scenario: Adversarial golden file — false positive resistance
  Given review prompt + test fixture with clean, correct code
  When review prompt instructs sub-agents
  Then prompt structure discourages false positives
  And golden file captures prompt that should yield clean review
```

**Technical Notes:**
- Architecture: `runner/prompts/review.md` — Go template file
- Two-stage assembly same as execute prompt (Story 3.1)
- Review prompt is the orchestrator — Claude inside the session uses Task tool to spawn sub-agents
- From Ralph's perspective: one `session.Execute` call with review prompt
- Adversarial golden files validate prompt quality, not Claude behavior
- FR17 lessons scope explicitly excluded — prompt MUST NOT instruct lessons writing

**Prerequisites:** Story 3.1 (execute prompt pattern), Story 2.1 (shared format contract)

---

### Story 4.2: Sub-Agent Prompts

**User Story:**
Как review-сессия, я хочу 4 специализированных sub-agent промпта с чёткими scope boundaries, чтобы каждый агент проверял свою зону ответственности без overlap.

**Acceptance Criteria:**

```gherkin
Scenario: Four sub-agent prompts exist with explicit scopes
  Given runner/prompts/agents/ directory
  Then contains quality.md, implementation.md, simplification.md, test-coverage.md
  And each prompt defines explicit SCOPE section (what this agent checks)
  And each prompt defines explicit OUT-OF-SCOPE section (what other agents check)
  And no overlap between 4 agents' scope boundaries

Scenario: Quality agent scope
  Given quality.md prompt
  Then scope includes: bugs, security issues, performance problems, error handling
  And out-of-scope: AC compliance (implementation), code simplicity (simplification), test coverage (test-coverage)

Scenario: Implementation agent scope
  Given implementation.md prompt
  Then scope includes: acceptance criteria compliance, feature completeness, requirement satisfaction
  And out-of-scope: code quality (quality), simplification opportunities (simplification), test coverage (test-coverage)

Scenario: Simplification agent scope
  Given simplification.md prompt
  Then scope includes: code readability, unnecessary complexity, over-engineering, dead code
  And out-of-scope: bugs (quality), AC compliance (implementation), test coverage (test-coverage)

Scenario: Test-coverage agent scope
  Given test-coverage.md prompt
  Then scope includes: test coverage for each AC (FR37 ATDD), no skip/xfail (FR38), test quality
  And out-of-scope: code bugs (quality), AC compliance (implementation), code simplicity (simplification)

Scenario: Golden file snapshot per agent
  Given each sub-agent prompt
  When assembled with test fixture
  Then matches runner/testdata/TestPrompt_Agent_<name>.golden

Scenario: Adversarial test — agent detects planted issue in its scope
  Given test fixture code with scope-specific planted issue per agent
  When prompt structure analyzed
  Then each agent's prompt is structured to detect issues in its scope
  And adversarial golden files capture expected detection behavior
```

**Technical Notes:**
- Architecture: `runner/prompts/agents/{quality,implementation,simplification,test-coverage}.md`
- Sub-agents are Claude Task tool agents INSIDE the review session — not separate Ralph processes
- Explicit scope boundaries = Devil's Advocate finding from epics elicitation (4 sub-agents overlap concern)
- FR37 (ATDD) enforced by test-coverage agent specifically
- FR38 (zero skip) enforced by test-coverage agent specifically
- Each prompt is `go:embed` in runner package

**Prerequisites:** Story 4.1 (review prompt references sub-agents)

---

### Story 4.3: Review Session Logic

**User Story:**
Как runner, мне нужно запускать review session после успешного execute (commit detected), передавая review prompt в fresh Claude Code session.

**Acceptance Criteria:**

```gherkin
Scenario: Review launches after commit detection
  Given execute session completed with new commit (FR8)
  When runner proceeds to review phase
  Then launches fresh Claude Code session with review prompt (FR13, FR14)
  And session uses --max-turns from config
  And session is independent of execute session context

Scenario: Review replaces stub from Story 3.5
  Given runner loop with ReviewResult contract
  When review session completes
  Then returns ReviewResult{Clean, FindingsPath}
  And matches function signature from Story 3.5 contract

Scenario: Review session captures output
  Given review session runs
  When Claude processes review prompt
  Then session.Execute returns SessionResult with session_id
  And runner can determine review outcome from file state

Scenario: Review session uses fresh context
  Given execute session was session abc-123
  When review session launches
  Then review session gets new session_id (not --resume)
  And review has no memory of execute session internals (FR14)

Scenario: Review outcome determined from file state
  Given review session completed
  When runner checks sprint-tasks.md for [x] on current task
  And checks review-findings.md via os.Stat + os.ErrNotExist
  Then computes ReviewResult{Clean: true} if [x] present and findings absent/empty
  Or computes ReviewResult{Clean: false, FindingsPath} if findings file non-empty
  And this is the Go code bridge between Claude behavior and ReviewResult contract
```

**Technical Notes:**
- From Ralph's perspective: `session.Execute(ctx, reviewOpts)` — one call
- Claude inside session orchestrates 4 sub-agents via Task tool (not Ralph's concern)
- Review outcome determined by checking: (1) sprint-tasks.md for `[x]`, (2) review-findings.md existence/content
- **File-state-to-ReviewResult logic** lives in this story as `determineReviewOutcome(projectRoot, taskLine) (ReviewResult, error)` function in runner
- **Current task identification:** `determineCurrentTask(sprintTasksPath) (taskLine, error)` parses sprint-tasks.md, finds first `- [ ]` (same logic as scanner Story 3.2 but returns the task line string for `[x]` check after review)
- This story replaces the configurable review stub from Story 3.5 with real implementation
- Session flags: same pattern as execute (SessionFlagMaxTurns, SessionFlagModel constants)

**Prerequisites:** Story 4.1 (review prompt), Story 3.5 (ReviewResult contract)

---

### Story 4.4: Findings Verification Logic

**User Story:**
Как review prompt, я хочу инструктировать Claude верифицировать каждый sub-agent finding и классифицировать его, чтобы только реальные проблемы попадали в review-findings.md.

**Acceptance Criteria:**

```gherkin
Scenario: Each finding verified independently
  Given 4 sub-agents produced findings
  When review session verifies each finding
  Then each classified as CONFIRMED or FALSE POSITIVE (FR16)
  And verification happens BEFORE writing to review-findings.md

Scenario: Severity assigned to confirmed findings
  Given finding classified as CONFIRMED
  When severity assigned
  Then exactly one of: CRITICAL, HIGH, MEDIUM, LOW (FR16)
  And severity is mandatory for every CONFIRMED finding

Scenario: False positives excluded from findings file
  Given finding classified as FALSE POSITIVE
  When review-findings.md is written
  Then FALSE POSITIVE findings are NOT included
  And only CONFIRMED findings appear in file

Scenario: Finding structure complete
  Given CONFIRMED finding
  When written to review-findings.md
  Then contains: ЧТО не так (description)
  And contains: ГДЕ в коде (file path + line range)
  And contains: ПОЧЕМУ это проблема (reasoning)
  And contains: КАК исправить (actionable recommendation)
```

**Technical Notes:**
- Verification is Claude's responsibility inside the review session — Ralph does not parse findings
- Review prompt (Story 4.1) contains verification instructions
- This story defines the finding format contract that review-findings.md must follow
- Finding format = ЧТО/ГДЕ/ПОЧЕМУ/КАК structure for execute session consumption
- FR16a (severity filtering) is Growth — in Epic 4 ALL confirmed findings block
- **Deliverable:** dedicated verification section in `runner/prompts/review.md`. Golden file `TestPrompt_Review.golden` updated to include verification instructions. PR = review.md diff + golden file update

**Prerequisites:** Story 4.3 (review session runs, sub-agents produce findings via 4.1+4.2)

---

### Story 4.5: Clean Review Handling

**User Story:**
Как review session, при отсутствии findings я должен атомарно отметить задачу `[x]` в sprint-tasks.md и очистить review-findings.md, чтобы runner мог перейти к следующей задаче.

**Acceptance Criteria:**

```gherkin
Scenario: Atomic [x] marking + findings clear on clean review
  Given review found no CONFIRMED findings (clean review)
  When review session writes results
  Then marks current task [x] in sprint-tasks.md (FR17)
  And clears review-findings.md content (FR17)
  And both operations happen together (atomic: both or neither)

Scenario: Review MUST NOT modify git working tree
  Given clean review handling
  When review session runs
  Then MUST NOT run git commands
  And MUST NOT modify source code files
  And ONLY modifies sprint-tasks.md and review-findings.md

Scenario: Runner detects clean review via file state
  Given review session completed
  When runner checks review outcome
  Then detects [x] on current task in sprint-tasks.md
  And review-findings.md is empty or absent
  And ReviewResult.Clean = true

Scenario: review_cycles resets on clean review
  Given review_cycles = 2 for current task
  When clean review detected
  Then review_cycles resets to 0 (Story 3.10 counter)
  And runner proceeds to next task

Scenario: review-findings.md absence equals clean
  Given review-findings.md does not exist after review
  When runner checks
  Then treats as clean review (Architecture: "Отсутствующий review-findings.md = пуст")
```

**Technical Notes:**
- Architecture: "review-findings.md: транзиентный (перезапись при findings, clear при clean)"
- Atomicity: review prompt instructs Claude to do both operations in sequence — not Ralph's enforcement
- Architecture pattern: `os.Stat` + `errors.Is(err, os.ErrNotExist)` — absent = empty
- Review does NOT create commit — only modifies sprint-tasks.md and review-findings.md
- The [x] marking is the ONLY place task status changes — Mutation Asymmetry enforced
- **Deliverable:** dedicated clean-handling section in `runner/prompts/review.md`. Golden file `TestPrompt_Review.golden` updated. PR = review.md diff + golden file update

**Prerequisites:** Story 4.3 (review session logic), Story 3.10 (review_cycles counter)

---

### Story 4.6: Findings Write

**User Story:**
Как review session, при обнаружении CONFIRMED findings я должен перезаписать review-findings.md со структурированными findings, чтобы следующая execute-сессия знала что исправлять.

**Acceptance Criteria:**

```gherkin
Scenario: Findings written to review-findings.md
  Given review found 2 CONFIRMED findings
  When review session writes findings
  Then review-findings.md contains structured findings (FR17)
  And previous content fully replaced (overwrite, not append)
  And findings are for current task only (no task ID in file)

Scenario: Finding format matches contract
  Given CONFIRMED finding with severity HIGH
  When written to review-findings.md
  Then format: ЧТО/ГДЕ/ПОЧЕМУ/КАК structure (Story 4.4)
  And severity clearly indicated
  And file is self-contained (execute session needs no other context)

Scenario: Only current task findings in file
  Given review for task 3
  When findings written
  Then review-findings.md contains ONLY task 3 findings
  And no historical findings from previous tasks
  And file is transient — represents current state only

Scenario: Runner detects findings via file state
  Given review session wrote findings
  When runner checks review outcome
  Then review-findings.md exists and is non-empty
  And ReviewResult.Clean = false
  And ReviewResult.FindingsPath points to review-findings.md
```

**Technical Notes:**
- Architecture: review-findings.md is transient — overwrite on findings, clear on clean
- File path: `{projectRoot}/review-findings.md` (resolved via config.ProjectRoot)
- Execute prompt (Story 3.1) already handles non-empty review-findings.md case
- File format designed for Claude consumption, not human — structured enough for AI to parse
- No task ID in file because it's always about the current task
- **Deliverable:** dedicated findings-write section in `runner/prompts/review.md`. Golden file `TestPrompt_Review.golden` updated. PR = review.md diff + golden file update

**Prerequisites:** Story 4.4 (finding format), Story 4.3 (review session)

---

### Story 4.7: Execute→Review Loop Integration

**User Story:**
Как runner, мне нужно интегрировать реальную review фазу в основной loop, заменив stub на настоящий review с review_cycles counter.

**Acceptance Criteria:**

```gherkin
Scenario: Review replaces stub in runner loop
  Given runner loop from Story 3.5 with configurable review stub
  When Epic 4 integration complete
  Then real review session replaces stub
  And ReviewResult contract preserved (Story 3.5)
  And runner loop code changes are minimal (seam design)

Scenario: Execute→review cycle with review_cycles counter
  Given execute completed with commit
  When review finds 1 CONFIRMED finding
  Then review_cycles increments to 1 (Story 3.10)
  And next execute launched (reads review-findings.md)
  And after fix commit → review again (cycle 2)

Scenario: Clean review after fix cycle
  Given review_cycles = 1 (one previous findings cycle)
  When second review is clean
  Then [x] marked, review-findings.md cleared
  And review_cycles resets to 0
  And runner proceeds to next task

Scenario: Max review iterations triggers emergency stop
  Given max_review_iterations = 3 (config)
  And review_cycles reaches 3 without clean review
  When runner checks counter (Story 3.10 logic)
  Then triggers emergency stop with exit code 1 (FR24)
  And message includes: task name, cycles completed, remaining findings

Scenario: Full task lifecycle in runner loop
  Given sprint-tasks.md with 1 open task
  When runner executes full cycle:
    execute → commit → review → findings → execute → commit → review → clean
  Then task marked [x]
  And 2 execute sessions + 2 review sessions launched
  And review_cycles = 0 at end

Scenario: Execute sees findings and fixes them (FR18)
  Given review-findings.md contains 2 CONFIRMED findings
  When next execute session launches
  Then execute prompt includes review-findings content (Story 3.1)
  And Claude addresses findings instead of implementing from scratch
  And runs tests after fix
  And commits on green tests

Scenario: Ralph does not distinguish first execute from fix execute
  Given review-findings.md is non-empty
  When ralph launches execute
  Then uses same execute prompt template (Story 3.1)
  And same session type (fresh, not --resume)
  And ralph makes no distinction between "first" and "fix" (FR18)

Scenario: Each fix iteration creates separate commit
  Given execute→review→execute→review cycle
  When each execute fixes issues
  Then each fix execute creates its own commit (FR18)
  And commits are separate from initial implementation commit
```

**Technical Notes:**
- This story wires together: Story 3.5 (loop), Story 3.10 (review_cycles), Story 4.3 (review session)
- Minimal runner.go changes — review stub already has correct interface (ReviewResult)
- Emergency stop logic already exists from Story 3.9/3.10 — just activated with real review
- Runner determines review outcome by checking file state, not parsing Claude output
- Architecture: "Ralph НЕ различает первый execute и fix execute — это одна и та же сессия по типу"
- Execute prompt (Story 3.1) already handles review-findings.md injection
- The intelligence is in the prompt — Claude decides whether to implement fresh or fix based on review-findings.md
- Commit detection same as Story 3.5 — HEAD comparison via GitClient

**Prerequisites:** Story 4.3 (review session), Story 4.5 (clean handling), Story 4.6 (findings write), Story 3.10 (review_cycles), Story 3.1 (execute prompt with findings injection)

---

### Story 4.8: Review Integration Test

**User Story:**
Как разработчик, я хочу комплексный integration test review pipeline, покрывающий clean review, findings cycle, fix cycle и emergency stop, чтобы гарантировать корректность перед v0.1 release.

**Acceptance Criteria:**

```gherkin
Scenario: Clean review — single pass
  Given scenario JSON: execute (commit) → review (clean)
  And MockGitClient returns commit after execute
  When runner.Run executes single task
  Then 1 execute + 1 review session launched
  And task marked [x]
  And review-findings.md absent or empty
  And exit code 0

Scenario: Findings → fix → clean review
  Given scenario JSON: execute (commit) → review (findings) → execute (commit) → review (clean)
  And review-findings.md created after first review
  When runner.Run executes
  Then 2 execute + 2 review sessions
  And review_cycles = 1 after first findings, 0 after clean
  And task marked [x] at end

Scenario: Emergency stop on max review cycles
  Given scenario JSON: 3 cycles of execute (commit) → review (findings)
  And max_review_iterations = 3
  When runner.Run executes
  Then review_cycles reaches 3
  And runner returns ErrMaxReviewCycles
  And error contains task info + cycles count

Scenario: Multi-task with mixed outcomes
  Given sprint-tasks.md with 3 tasks
  And scenario JSON: task1 (clean), task2 (1 fix cycle), task3 (clean)
  When runner.Run executes
  Then all 3 tasks marked [x]
  And review_cycles properly reset between tasks

Scenario: Review determines outcome via file state
  Given mock review session that writes [x] to sprint-tasks.md
  And mock review session that writes review-findings.md
  When runner checks outcome
  Then correctly determines clean vs findings from file state
  And does not parse Claude session output

Scenario: Bridge golden file as input
  Given sprint-tasks.md from bridge golden file testdata (Story 2.5)
  When used as runner test input
  Then validates full pipeline: bridge output → execute → review

Scenario: v0.1 smoke test note
  Given all integration tests pass
  When considering release readiness
  Then manual smoke test with real Claude CLI recommended before v0.1 tag
  And documented in test file comments

Scenario: Manual prompt validation checklist documented
  Given review prompt from 4.1 and sub-agent prompts from 4.2
  When manual testing before v0.1 tag
  Then checklist covers: (1) planted bug detected by correct sub-agent
  And (2) clean code yields no false positives
  And (3) findings contain all 4 fields (ЧТО/ГДЕ/ПОЧЕМУ/КАК)
  And (4) clean review produces [x] + empty findings
  And (5) findings review does NOT mark [x]
  And checklist is in runner/testdata/manual_smoke_checklist.md
```

**Technical Notes:**
- Uses MockClaude (Story 1.11) + MockGitClient (Story 3.4) + t.TempDir()
- Scenario JSON extended: `{type: "review", clean: true/false, creates_findings: bool}`
- Build tag: `//go:build integration`
- Test file: `runner/runner_review_integration_test.go` (separate from Story 3.11 runner_integration_test.go)
- v0.1 manual smoke test = Chaos Monkey finding from epics elicitation
- Bridge golden file input validates end-to-end contract
- Manual smoke checklist = formal artifact for v0.1 release gate (Murat TEA recommendation)

**Prerequisites:** Story 4.7 (execute→review loop + fix cycle), Story 3.11 (runner integration test pattern), Story 1.11 (MockClaude), Story 3.4 (MockGitClient)

---

### Epic 4 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 4.1 | Review Prompt Template | FR13,FR15,FR16,FR17 | 2 + testdata | 5 |
| 4.2 | Sub-Agent Prompts | FR15,FR37,FR38 | 4 + testdata | 7 |
| 4.3 | Review Session Logic | FR13,FR14 | 1 | 5 |
| 4.4 | Findings Verification Logic | FR16 | review.md section | 4 |
| 4.5 | Clean Review Handling | FR17 | review.md section | 5 |
| 4.6 | Findings Write | FR17 | review.md section | 4 |
| 4.7 | Execute→Review Loop + Fix Cycle | FR18,FR18a,FR24 | 1 | 8 |
| 4.8 | Review Integration Test | — | 1 + scenarios + checklist | 8 |
| | **Total** | **FR13-FR18a,FR37,FR38** | | **~46** |

**FR Coverage:** FR13 (4.1, 4.3), FR14 (4.3), FR15 (4.1, 4.2), FR16 (4.1, 4.4), FR17 (4.1, 4.5, 4.6), FR18 (4.7), FR18a (4.7), FR37 (4.2), FR38 (4.2)

**Architecture Sections Referenced:** Runner package (prompts/review.md, prompts/agents/), Package Boundaries (review-findings.md writer/reader), Testing Patterns (scenario-based mock, golden files), Data Flow (execute→review cycle)

**Dependency Graph:**
```
3.1 ──→ 4.1 ──→ 4.2
2.1 ──→ 4.1     │
3.5 ──→ 4.3 ←── 4.1 + 4.2
             4.3 ──→ 4.4 ──→ 4.6 ─┐
                │                   │
                ├──→ 4.5 ──────────┤
                                    └──→ 4.7 ──→ 4.8
                            3.10 ──→ 4.7
                            3.1 ───→ 4.7
                            1.11, 3.4 ──→ 4.8
                            2.5 ──→ 4.8
```
Note: 4.4 и 4.5 parallel-capable (prompt-driven, зависят от 4.3 но не друг от друга). 4.6 logically depends on 4.4 (findings must be verified before written). v0.1 release после 4.8. Story 4.8 = бывшая 4.9 (Story 4.8 "Review Fix Cycle" merged в Story 4.7)

---
