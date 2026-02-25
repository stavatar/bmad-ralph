# Epic 6: Knowledge Management & Polish — Stories

**Scope:** FR26, FR27, FR28, FR28a, FR28b, FR29, FR39
**Stories:** 9
**Release milestone:** v0.3

**Context (из Epics Structure Plan + предыдущих эпиков):**
- **KnowledgeWriter interface** определён в Epic 3 Story 3.7: `WriteProgress(ctx, ProgressData) error` + `WriteLessons(ctx, LessonsData) error`
- Epic 3 содержит no-op implementation — Epic 6 заменяет на реальную
- Extensible: добавление полей в structs, не изменение method signatures (Story 3.7 contract)
- FR17 lessons deferred from Epic 4 — реализуются здесь
- `runner/knowledge.go` уже содержит interface + no-op + data structs
- LEARNINGS.md budget: 200 lines hard limit (Architecture)
- Distillation: `claude -p` session (не interactive)
- Serena: best-effort, graceful fallback, modifies `runner.Run()`

---

### Story 6.1: KnowledgeWriter Implementation — LEARNINGS.md

**User Story:**
Как система, я хочу реальную реализацию KnowledgeWriter которая записывает паттерны и выводы в LEARNINGS.md с проверкой бюджета, чтобы знания накапливались между сессиями.

**Acceptance Criteria:**

```gherkin
Scenario: WriteLessons appends to LEARNINGS.md
  Given LEARNINGS.md exists with 50 lines
  And LessonsData contains new lesson content
  When KnowledgeWriter.WriteLessons(ctx, data) called
  Then lesson appended to LEARNINGS.md (FR27)
  And existing content preserved
  And file written via os.WriteFile with 0644

Scenario: WriteLessons creates LEARNINGS.md if absent
  Given LEARNINGS.md does not exist
  When WriteLessons called
  Then creates LEARNINGS.md with lesson content
  And no error

Scenario: Budget check returns line count
  Given LEARNINGS.md has 180 lines
  When BudgetCheck(ctx, learningsPath) called (free function, not interface method)
  Then returns BudgetStatus{Lines: 180, Limit: 200, OverBudget: false}

Scenario: Budget exceeded detection
  Given LEARNINGS.md has 210 lines
  When BudgetCheck(ctx, learningsPath) called
  Then returns BudgetStatus{OverBudget: true}
  And triggers distillation (Story 6.3)

Scenario: Replaces no-op from Epic 3
  Given KnowledgeWriter interface from Story 3.7
  When real implementation provided
  Then same interface methods: WriteProgress + WriteLessons
  And ProgressData/LessonsData structs extended with new fields (not renamed)
  And no-op impl removed or replaced

Scenario: Thread-safe writes
  Given multiple callers could write lessons
  When concurrent writes attempted
  Then writes are serialized (simple mutex or sequential calls)
  And no data corruption
```

**Technical Notes:**
- Architecture: `runner/knowledge.go` — LEARNINGS.md append, budget check (200 lines hard limit)
- Extends Epic 3 structs: `LessonsData` gets `Source string` field (resume/review/distillation)
- Line counting: `strings.Count(content, "\n")` — simple, no parsing
- Budget = 200 lines (Architecture constant, could be in config later)
- File path: `{projectRoot}/LEARNINGS.md` via config
- BudgetCheck is a free function `BudgetCheck(ctx, path) (BudgetStatus, error)`, NOT a KnowledgeWriter method — preserves "max 2 methods" interface contract from Epic 3

**Prerequisites:** Story 3.7 (KnowledgeWriter interface + no-op)

---

### Story 6.2: Distillation Prompt Template

**User Story:**
Как система, я хочу distillation-промпт который инструктирует Claude сжать раздутый LEARNINGS.md, сохраняя ключевые паттерны и удаляя дублирование.

**Acceptance Criteria:**

```gherkin
Scenario: Distillation prompt assembled
  Given LEARNINGS.md content (over budget)
  When distillation prompt assembled
  Then contains: full LEARNINGS.md content for compression
  And instructs: preserve key patterns, remove duplicates, merge similar lessons
  And instructs: output compressed LEARNINGS.md (replace entirely)
  And instructs: target under 200 lines budget
  And instructs: keep chronological grouping where meaningful

Scenario: Golden file snapshot
  Given distillation prompt template in runner/prompts/distillation.md
  When assembled with test fixture
  Then matches runner/testdata/TestPrompt_Distillation.golden
  And updateable via `go test -update`

Scenario: Distillation uses claude -p (non-interactive)
  Given distillation prompt
  When session invoked
  Then uses `claude -p` flag (pipe mode, non-interactive)
  And no --resume, no --max-turns (single-shot)
```

**Technical Notes:**
- Architecture: `runner/prompts/distillation.md` — Go template
- `claude -p` = pipe mode: stdin prompt → stdout response → exit
- Distillation is a compression task — input = full LEARNINGS.md, output = compressed version
- session.Execute with pipe mode option (new session type variant)
- Template is simpler than execute/review — no sub-agents, no sprint-tasks

**Prerequisites:** Story 1.10 (prompt assembly pattern)

---

### Story 6.3: Distillation Trigger

**User Story:**
Как runner, после clean review я хочу проверить размер LEARNINGS.md и при превышении бюджета запустить distillation session с backup, чтобы файл оставался компактным и полезным.

**Acceptance Criteria:**

```gherkin
Scenario: Budget check after clean review triggers distillation
  Given clean review completed (task marked [x])
  And LEARNINGS.md has 220 lines (exceeds 200 budget)
  When runner checks budget (FR28a)
  Then triggers distillation session

Scenario: Backup before distillation
  Given distillation about to start
  When backup created
  Then copies LEARNINGS.md to LEARNINGS.md.bak (byte-for-byte)
  And backup exists before distillation session runs
  And distillation backup preserved until next successful distillation

Scenario: Distillation replaces LEARNINGS.md
  Given distillation session completes successfully
  When output received
  Then LEARNINGS.md replaced with compressed content
  And new line count under 200 budget
  And backup remains as safety net

Scenario: Distillation failure preserves original
  Given distillation session fails (non-zero exit)
  When error handled
  Then original LEARNINGS.md.bak restored
  And warning logged
  And runner continues (distillation failure is non-fatal)

Scenario: No distillation when under budget
  Given clean review completed
  And LEARNINGS.md has 150 lines
  When runner checks budget
  Then no distillation triggered
  And runner proceeds to next task

Scenario: First clean review without prior LEARNINGS — no distillation
  Given clean review completed
  And LEARNINGS.md does not exist
  When runner checks budget
  Then no distillation needed
  And runner proceeds normally
```

**Technical Notes:**
- Architecture: "Distillation: отдельная `claude -p` сессия при превышении бюджета"
- Backup = distillation backup requirement (Graph of Thoughts finding from epics elicitation)
- Distillation is non-fatal: failure → restore backup, log warning, continue
- Trigger point: after clean review, before advancing to next task
- Budget check uses `BudgetCheck()` free function from Story 6.1 (not a KnowledgeWriter method)

**Prerequisites:** Story 6.1 (budget check), Story 6.2 (distillation prompt)

---

### Story 6.4: CLAUDE.md Section Management

**User Story:**
Как система, я хочу безопасно читать и обновлять ТОЛЬКО секцию `## Ralph operational context` в CLAUDE.md, не затрагивая остальное содержимое проекта.

**Acceptance Criteria:**

```gherkin
Scenario: Read ralph section from CLAUDE.md
  Given CLAUDE.md exists with multiple sections including "## Ralph operational context"
  When ReadRalphSection(ctx) called
  Then returns content between "## Ralph operational context" and next ## heading (or EOF)
  And does not return other sections

Scenario: Write ralph section preserves other content
  Given CLAUDE.md has: ## Project setup, ## Ralph operational context, ## Guidelines
  When WriteRalphSection(ctx, newContent) called
  Then "## Ralph operational context" section replaced with newContent
  And "## Project setup" section unchanged
  And "## Guidelines" section unchanged

Scenario: Create section if missing
  Given CLAUDE.md exists but has no "## Ralph operational context" section
  When WriteRalphSection called
  Then appends "## Ralph operational context" + content at end of file
  And existing content preserved

Scenario: Create CLAUDE.md if missing
  Given CLAUDE.md does not exist
  When WriteRalphSection called
  Then creates CLAUDE.md with "## Ralph operational context" section
  And content written correctly

Scenario: Section boundary detection
  Given "## Ralph operational context" followed by "## Another section"
  When reading ralph section
  Then stops at "## Another section" heading
  And does not include content from other sections
```

**Technical Notes:**
- Architecture: "CLAUDE.md: обновление ТОЛЬКО секции `## Ralph operational context`"
- Section detection: line scanning for `## Ralph operational context` heading
- End detection: next `## ` heading or EOF
- File path: `{projectRoot}/CLAUDE.md` via config (paths.claude_md)
- This is utility code used by both resume-extraction knowledge (Story 6.6) and review knowledge (Story 6.7)
- FR26: "Существующий контент проекта вне секции ralph не затрагивается"

**Prerequisites:** None (utility code, no epic dependencies)

---

### Story 6.5: Knowledge Loading in Session Context

**User Story:**
Как execute и review сессии, я хочу чтобы LEARNINGS.md и ralph section из CLAUDE.md загружались в prompt assembly, чтобы каждая сессия имела доступ к накопленным знаниям.

**Acceptance Criteria:**

```gherkin
Scenario: Execute prompt includes LEARNINGS.md content
  Given LEARNINGS.md exists with lessons
  When execute prompt assembled (Story 3.1)
  Then LEARNINGS.md content injected via strings.Replace (FR29)
  And content available to Claude in session context

Scenario: Execute prompt includes ralph section from CLAUDE.md
  Given CLAUDE.md has "## Ralph operational context" section
  When execute prompt assembled
  Then ralph section content injected via strings.Replace (FR29)

Scenario: Review prompt includes knowledge files
  Given LEARNINGS.md and CLAUDE.md ralph section exist
  When review prompt assembled (Story 4.1)
  Then both contents injected into review prompt (FR29)

Scenario: Missing knowledge files handled gracefully
  Given LEARNINGS.md does not exist
  And CLAUDE.md has no ralph section
  When prompts assembled
  Then knowledge placeholders replaced with empty string
  And no error

Scenario: Golden file update with knowledge injection
  Given execute prompt golden file from Story 3.1
  When knowledge injection added
  Then golden file updated to include knowledge sections
  And `go test -update` refreshes baseline
```

**Technical Notes:**
- Architecture: "Knowledge files загружаются в prompt assembly (strings.Replace)"
- Modifies execute prompt template (Story 3.1) and review prompt template (Story 4.1) — adds knowledge placeholders
- Uses strings.Replace stage 2 (user content injection) — safe from template injection
- ReadRalphSection from Story 6.4 used to extract CLAUDE.md section
- LEARNINGS.md read via os.ReadFile (whole file, small)
- Config paths: `paths.learnings_md`, `paths.claude_md`
- NFR14: log file events should include: session type (execute/review/resume), task name, duration, outcome (commit/findings/error)

**Prerequisites:** Story 6.4 (CLAUDE.md section reader), Story 3.1 (execute prompt), Story 4.1 (review prompt)

---

### Story 6.6: Resume-Extraction Knowledge

**User Story:**
Как resume-extraction сессия, я хочу записывать причины неудачи в LEARNINGS.md и обновлять ralph section в CLAUDE.md, чтобы будущие сессии учились на ошибках.

**Acceptance Criteria:**

```gherkin
Scenario: Resume-extraction writes to LEARNINGS.md
  Given resume-extraction completed (Story 3.7)
  When KnowledgeWriter.WriteLessons called with source="resume"
  Then failure reasons appended to LEARNINGS.md (FR28)
  And lessons include: what was attempted, where stuck, extracted insights

Scenario: Resume-extraction updates CLAUDE.md ralph section
  Given resume-extraction completed
  When WriteRalphSection called
  Then ralph operational context updated with failure insights (FR26, FR28)
  And existing project content preserved

Scenario: Replaces no-op behavior from Epic 3
  Given Epic 3 KnowledgeWriter no-op returned nil
  When Epic 6 real implementation active
  Then WriteLessons actually writes to LEARNINGS.md
  And WriteProgress still writes progress to sprint-tasks.md (unchanged from Epic 3)

Scenario: Resume-extraction prompt updated with knowledge instructions
  Given resume-extraction invoked via --resume
  When prompt assembled
  Then includes instructions to extract failure insights
  And includes instructions to write to LEARNINGS.md
  And includes instructions to update CLAUDE.md ralph section
```

**Technical Notes:**
- This story replaces no-op from Epic 3 Story 3.7 with real KnowledgeWriter
- FR28: resume-extraction пишет причины неудачи + извлечённые знания
- Resume-extraction prompt needs update to include knowledge-writing instructions
- Claude inside resume-extraction session reads/writes LEARNINGS.md and CLAUDE.md directly
- Ralph's KnowledgeWriter provides Go-side budget check; Claude does the actual writing

**Prerequisites:** Story 6.1 (KnowledgeWriter impl), Story 6.4 (CLAUDE.md section), Story 3.7 (resume-extraction)

---

### Story 6.7: Review Knowledge

**User Story:**
Как review сессия с findings, я хочу записывать уроки (типы ошибок, упускаемые паттерны) в LEARNINGS.md и обновлять CLAUDE.md, чтобы будущие execute сессии не повторяли те же ошибки.

**Acceptance Criteria:**

```gherkin
Scenario: Review with findings writes lessons
  Given review found CONFIRMED findings
  When review session processes findings (FR28a)
  Then lessons appended to LEARNINGS.md
  And lessons include: error types, what agent forgets, patterns for future sessions
  And ralph section in CLAUDE.md updated

Scenario: Clean review does NOT write lessons
  Given review is clean (no findings)
  When review session completes
  Then no new content added to LEARNINGS.md
  And CLAUDE.md not modified (beyond [x] + clear findings)

Scenario: Review prompt updated with knowledge instructions
  Given review prompt from Story 4.1
  When Epic 6 integration
  Then prompt includes: write lessons to LEARNINGS.md on findings
  And prompt includes: update ralph section in CLAUDE.md on findings
  And prompt includes: do NOT write lessons on clean review

Scenario: FR17 lessons scope now implemented
  Given FR17 lessons deferred from Epic 4
  When Epic 6 review knowledge active
  Then review writes lessons on findings (previously deferred)
  And review writes [x] + clears findings on clean (unchanged from Epic 4)
```

**Technical Notes:**
- FR28a: "Review-сессия при наличии findings сама записывает уроки в LEARNINGS.md"
- This completes the FR17 deferred scope from Epic 4
- Review prompt (Story 4.1) gets additional instructions for knowledge writing
- Claude inside review session does the actual writing — not Ralph's Go code
- Budget check + distillation trigger after clean review (Story 6.3)

**Prerequisites:** Story 6.1 (KnowledgeWriter), Story 6.4 (CLAUDE.md section), Story 4.1 (review prompt)

---

### Story 6.8: Serena Integration

**User Story:**
Как runner, я хочу обнаруживать Serena MCP и использовать её для индексации проекта перед execute сессиями, чтобы улучшить token economy и review accuracy.

**Acceptance Criteria:**

```gherkin
Scenario: Serena detected at startup
  Given Serena MCP available in environment
  When ralph run starts
  Then detects Serena availability
  And logs "Serena MCP detected"

Scenario: Full index at ralph run startup
  Given Serena detected
  When runner initializes
  Then triggers Serena full index (FR39)
  And timeout: 60 seconds
  And progress output displayed

Scenario: Incremental index before each execute
  Given Serena available
  When execute session about to launch
  Then triggers Serena incremental index
  And timeout: configurable (default 10s from config)
  And progress output displayed

Scenario: Timeout graceful fallback
  Given Serena full index running
  When 60 second timeout exceeded
  Then cancels index operation
  And outputs "Serena timeout — falling back to standard file reading"
  And runner continues without Serena index

Scenario: Serena unavailable graceful fallback
  Given Serena MCP not available in environment
  When ralph run starts
  Then outputs "Serena MCP not available — using standard file reading"
  And runner operates normally without Serena
  And no error

Scenario: Serena configurable
  Given config with serena_enabled: false
  When ralph run starts
  Then skips Serena detection entirely
  And no Serena-related output

Scenario: Serena timeout configurable
  Given config with serena_timeout: 20
  When incremental index runs
  Then uses 20s timeout instead of default 10s
```

**Technical Notes:**
- Architecture: "Serena: detect → full index (60s timeout) → incremental (10s) → graceful fallback"
- Modifies `runner.Run()` from Epic 3 — adds Serena calls before execute
- Detection: check if `serena` CLI available via `exec.LookPath` or similar
- Full index: `serena index --full` (or equivalent CLI command)
- Incremental: `serena index` (or equivalent)
- All Serena calls via `exec.CommandContext(ctx)` with timeout context
- Config: `serena_enabled` (default true), `serena_timeout` (default 10s)
- Best-effort: any Serena failure = log warning + continue

**Prerequisites:** Story 3.5 (runner loop — insertion point for Serena calls)

---

### Story 6.9: --always-extract Flag + Final Integration Test

**User Story:**
Как разработчик, я хочу `--always-extract` для извлечения знаний из каждого execute, и финальный end-to-end integration test всего продукта, чтобы все 6 эпиков работали вместе.

**Acceptance Criteria:**

```gherkin
Scenario: --always-extract runs resume-extraction after every execute
  Given ralph run --always-extract
  And execute session completes with commit (success)
  When runner processes successful execute
  Then resume-extraction still runs (FR28b)
  And extracts knowledge from successful execution process
  And writes to LEARNINGS.md + CLAUDE.md

Scenario: Without --always-extract — only on failure
  Given ralph run without --always-extract
  And execute session completes with commit
  When runner processes result
  Then NO resume-extraction (standard behavior)
  And proceeds directly to review

Scenario: Config file support for always-extract
  Given config.yaml has always_extract: true
  And no --always-extract flag
  When runner starts
  Then always-extract enabled
  And CLI flag overrides config value

Scenario: FINAL — full end-to-end flow
  Given scenario JSON covering full flow:
    bridge → execute (commit) → review (findings) → execute fix (commit) → review (clean) → knowledge written → Serena indexed
  And MockClaude + MockGitClient + mock Serena
  And sprint-tasks.md from bridge golden file
  When runner.Run executes with all features
  Then bridge output feeds runner
  And execute sessions launch with knowledge context
  And review finds and verifies findings
  And fix cycle produces clean review
  And [x] marked + review-findings cleared
  And LEARNINGS.md written with lessons
  And CLAUDE.md ralph section updated
  And Serena incremental index called before each execute
  And budget check runs after clean review
  And all 6 epics work together

Scenario: FINAL — gates + knowledge + emergency
  Given gates_enabled = true, --every 2
  And scenario with 3 tasks: task1 (clean), task2 (emergency→skip), task3 (clean)
  And mock stdin for gate actions
  When runner.Run executes
  Then checkpoint gate fires after task 2
  And emergency gate fires for task 2 (max retries)
  And skip advances to task 3
  And knowledge written throughout

Scenario: FINAL — resume + always-extract
  Given --always-extract enabled
  And scenario: execute (no commit) → resume-extraction → retry → execute (commit) → always-extract
  When runner.Run executes
  Then resume-extraction runs on failure (writes knowledge)
  And always-extract runs on success (extracts positive knowledge)
  And LEARNINGS.md accumulates from both sources
```

**Technical Notes:**
- `--always-extract` flag: `cmd/ralph/run.go` Cobra flag, config key `always_extract` (default false)
- Modifies runner loop: after successful commit detection, if always_extract → run resume-extraction
- FINAL integration test: most comprehensive test in the project — covers all 6 epics
- Test file: `runner/runner_final_integration_test.go`
- Build tag: `//go:build integration`
- Mock Serena: mock binary that exits 0 (or test without Serena for simplicity)
- This is the "Bob's Final Integration Test" from epics structure plan

**Prerequisites:** Story 6.1-6.8 (all Epic 6 stories), Story 5.6 (gates integration), Story 4.8 (review integration), Story 3.11 (runner integration)

---

### Epic 6 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 6.1 | KnowledgeWriter — LEARNINGS.md | FR27 | 1 | 6 |
| 6.2 | Distillation Prompt Template | FR28a | 2 + testdata | 3 |
| 6.3 | Distillation Trigger | FR28a | 1 | 6 |
| 6.4 | CLAUDE.md Section Management | FR26 | 1 | 5 |
| 6.5 | Knowledge Loading in Context | FR29 | 2 (prompt updates) | 5 |
| 6.6 | Resume-Extraction Knowledge | FR28 | 1 (prompt update) | 4 |
| 6.7 | Review Knowledge | FR28a | 1 (prompt update) | 4 |
| 6.8 | Serena Integration | FR39 | 1 | 7 |
| 6.9 | --always-extract + Final Test | FR28b | 2 | 6 |
| | **Total** | **FR26-FR29,FR28a,FR28b,FR39** | | **~46** |

**FR Coverage:** FR26 (6.4, 6.6, 6.7), FR27 (6.1), FR28 (6.6), FR28a (6.3, 6.7), FR28b (6.9), FR29 (6.5), FR39 (6.8)

**Architecture Sections Referenced:** Runner package (knowledge.go, prompts/distillation.md), Package Boundaries (LEARNINGS.md, CLAUDE.md writers), Subprocess Patterns (Serena CLI), File I/O Patterns, Data Flow

**Dependency Graph:**
```
3.7 ────→ 6.1 ──→ 6.3
1.10 ───→ 6.2 ──→ 6.3
                    │
6.4 (independent) ──┤
                    │
3.1 ──→ 6.5 ←── 6.4
4.1 ──→ 6.5
          │
6.1, 6.4 → 6.6 ←── 3.7
6.1, 6.4 → 6.7 ←── 4.1
          │
3.5 ────→ 6.8 (independent of knowledge)
          │
6.1-6.8 → 6.9
```
Note: 6.4 полностью independent (нет epic dependencies). 6.2 и 6.4 parallel-capable. 6.6 и 6.7 parallel-capable. 6.8 independent от knowledge stories

---
