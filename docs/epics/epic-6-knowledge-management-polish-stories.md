# Epic 6: Knowledge Management & Polish — Stories (v5 — 6-agent review refinements)

**Scope:** FR26, FR27, FR28, FR28a, FR29, FR39
**Stories:** 11
**Release milestone:** v0.3

**Redesign context (2026-02-27/28 → v3, 2026-03-02 → v4, 2026-03-02 → v5):**
Эпик переработан дважды на основе исследований двух команд по 4 агента (2 аналитика + 2 архитектора
с /deep-research каждая), верифицирован командой из 6 агентов (3 пары × /deep-research: retrieval quality,
extraction quality, alternative methods — 91-92% confidence, 0 redesigns), и доработан по результатам
2 challenge-сессий с пользователем. V4 интегрирует результаты 10-agent review (5 пар аналитик+архитектор:
extraction pipeline, injection universality, stories 6.5/6.6, stories 6.7/6.8, consolidated) — 28 решений
пользователя (4 CRITICAL, 8 HIGH, 12 MEDIUM, 4 LOW). V5 интегрирует результаты 6-agent review (3 пары
аналитик+архитектор) — 11 корректировок: line-count guard (C1), distill_gate config (C2), overflow stderr
warning (M6), inject all from LEARNINGS.md (H5), Story 6.5 split на 3 (6.5a/b/c), DistillState в .ralph/
с Version, knowledge.go split на 4 файла, trend tracking вместо A/B (6.9). Ключевые изменения v3→v4:

1. **Write path (C1):** Claude пишет напрямую в LEARNINGS.md через file tools. Go делает snapshot
   LEARNINGS.md перед сессией, diff после сессии, тегирует невалидные entries `[needs-formatting]`.
   НЕТ промежуточного pending файла. Go-side WriteLessons НЕ пишет контент — Go только post-validates.
2. **Distillation failures (C2):** Config-driven: `distill_gate: human|auto` (default: human).
   Human mode: Human GATE на КАЖДОМ failure дистилляции с описанием ошибки + текущий размер файла.
   Опции: retry once, retry 5-10 times, skip. Auto mode: circuit breaker auto-skip после 3
   consecutive failures, продолжаем без дистилляции. `ralph distill` сохранён как manual override.
3. **Serena = MCP (C3):** Serena — MCP server, НЕ CLI. Детекция через `.claude/settings.json` или
   `.mcp.json`. НЕТ `exec.LookPath("serena")`. Ralph не вызывает Serena напрямую. Prompt hint:
   "If Serena MCP tools available, use them for code navigation." Минимальный интерфейс:
   `CodeIndexerDetector{Available(projectRoot) bool, PromptHint() string}`.
4. **Persistence: Multi-file by category** — LEARNINGS.md (hot, raw) + `.claude/rules/ralph-{category}.md`
   (distilled, multi-file). Index: `.claude/rules/ralph-index.md` (auto-generated TOC).
   Ralph **НЕ модифицирует** CLAUDE.md проекта (zero corruption risk, FR26 satisfied vacuously).
   `.claude/rules/` = proven infrastructure (9 файлов, 122 паттерна за 5 эпиков).
5. **Distillation: HUMAN-GATED** — 3-слойная composable архитектура:
   - Layer 1: Go-level semantic dedup при каждой записи (0 tokens)
   - Layer 2: Auto `claude -p` distillation при soft threshold 150 строк (~8K tokens)
   - Layer 3: Safety nets — human gate on failure (retry/skip), cooldown (monotonic counter, ≥5 tasks),
     post-validation quality gate (7+ criteria)
   - `ralph distill` CLI сохранён как manual override (дополняет, не заменяет auto)
   - Без принудительной обрезки: 300+ строк = 3-4% контекста (200K окно), linear decay не cliff
6. **Category system (H2):** Fixed canonical 7 + misc. Claude can propose NEW_CATEGORY at distillation.
   Category list only grows, never shrinks.
7. **Output protocol (H6):** BEGIN_DISTILLED_OUTPUT / END_DISTILLED_OUTPUT markers. Go парсит только
   между маркерами.
8. **Trend tracking (6.9):** Knowledge effectiveness trend tracking — findings_per_task,
   first_clean_review_rate tracked over time в DistillState. Нет A/B mode switching.
9. **--always-extract: Deferred** — флаг scaffold существует в config, wiring в Growth.

**Источники решений (3 раунда исследований + верификация + 10-agent review):**
- R1: `docs/research/knowledge-extraction-in-claude-code-agents.md` (20 источников)
- R2: `docs/research/knowledge-enforcement-in-claude-code-agents.md`
- R3: `docs/research/alternative-knowledge-methods-for-cli-agents.md` (22 источника)
- Verification: 6-agent team (91-92% confidence), RAG/GraphRAG NOT needed for file-based CLI
- V4 review: 10-agent team (5 pairs), 28 user decisions (C1-C4, H1-H8, M1-M12, L1-L6)
- V5 review: 6-agent team (3 pairs), 11 corrections (C1, C2, M6, H5, 6.5 split, DistillState, knowledge.go split, 6.9 simplify)
- Anthropic: Effective Context Engineering — "smallest set of high-signal tokens"
- Chroma: Context Rot (18 моделей) — degradation grows with context
- SFEIR: CLAUDE.md Optimization — ~15 rules = 94% compliance
- CVE-2025-59536, CVE-2026-21852 — programmatic config file editing = confirmed risk class
- GitHub Copilot: JIT citation validation — +7% PR merge rate, self-healing memories
- Live-SWE-agent: step-reflection prompt — +12% quality from single reflection
- SICA: utility function for distillation validation
- DGM: concrete violations >> abstract rules (2.5x effectiveness)
- MemOS: multi-level working/long-term/cold confirms tiered approach
- Nature 2024: Model collapse — self-referencing training degradation risk

**Existing scaffold (from Epics 1-5):**
- `runner/knowledge.go` — KnowledgeWriter interface (WriteProgress only), NoOpKnowledgeWriter, ProgressData
- `config.Config` — AlwaysExtract, SerenaEnabled, SerenaTimeout, LearningsBudget fields
- `config.TemplateData` — LearningsContent, ClaudeMdContent string fields
- `config.CLIFlags` — AlwaysExtract *bool
- `defaults.yaml` — learnings_budget: 200, serena_enabled: true, serena_timeout: 10
- `cmd/ralph/run.go` — --always-extract flag parsed (not wired to runner)
- `session.Options` — Prompt field + flagPrompt="-p" (pipe mode support)

---

### Story 6.1: FileKnowledgeWriter — LEARNINGS.md Post-Validation

**User Story:**
Как система, я хочу Go-side post-validation для LEARNINGS.md записей которые Claude пишет напрямую через file tools, с проверкой бюджета и тегированием невалидных entries, чтобы знания накапливались между сессиями с гарантией качества.

**Acceptance Criteria:**

```gherkin
Scenario: WriteLessons replaced with post-validation model (C1)
  Given KnowledgeWriter from Epic 3 has only WriteProgress
  When Epic 6 extends the interface
  Then ValidateNewLessons(ctx, LessonsData) error added as second method
  And LessonsData struct defined: Source string, Entries []LessonEntry
  And LessonEntry struct defined: Category, Topic, Content, Citation string (M3)
  And NoOpKnowledgeWriter updated with no-op ValidateNewLessons
  And compile-time interface check still passes

Scenario: Snapshot-diff post-validation model (C1)
  Given runner snapshots LEARNINGS.md content before session starts
  And Claude writes directly to LEARNINGS.md via file tools during session
  When session ends (execute or review)
  Then Go diffs LEARNINGS.md (current vs snapshot)
  And line-count guard: if current lines < snapshot lines, log warning "LEARNINGS.md rewrite detected — full revalidation triggered"
  And new entries parsed into []LessonEntry
  And each entry validated through 6 quality gates (G1-G6)
  And invalid entries tagged with [needs-formatting] IN the file (C4)
  And warning logged: "Entry saved with [needs-formatting] — will be fixed at distillation"
  And no entry content removed (append-only, no knowledge loss)

Scenario: Quality gate validates each new entry (6 gates)
  Given new entries detected via diff
  When FileKnowledgeWriter.ValidateNewLessons(ctx, data) called
  Then Go-side quality gate validates each entry:
    - G1 Format check: entry has `## category: topic [citation]` header
    - G2 Citation present: `[source, file:line]` parsed successfully (any file extension)
    - G3 Not duplicate: no existing entry with same citation (semantic dedup)
    - G4 Budget check: total lines < hard limit
    - G5 Entry cap: max 5 new entries per validation call (named constant, L1)
    - G6 Min content: entry content body >= 20 chars (named constant, L1)
  And valid entries left as-is in LEARNINGS.md
  And invalid entries tagged [needs-formatting] in-place
  And optional `VIOLATION:` marker supported in content body (inline examples)

Scenario: Semantic dedup merges similar entries
  Given LEARNINGS.md has entry "## testing: assertion-quality [review, tests/test_auth.py:42]"
  And Claude wrote new entry with same heading prefix "## testing: assertion-quality [review, tests/test_api.py:15]"
  When post-validation runs
  Then header normalization applied (strings.ToLower + strings.TrimSpace) before comparison
  And new facts merged under existing heading (not duplicated)
  And both citations preserved in merged entry

Scenario: LEARNINGS.md created by Claude if absent
  Given LEARNINGS.md does not exist
  When Claude writes lessons during session
  Then Claude creates LEARNINGS.md with lesson content
  And post-validation runs on entire file (all entries are "new")

Scenario: BudgetCheck returns status with thresholds
  Given LEARNINGS.md has 160 lines
  When BudgetCheck(ctx, learningsPath) called (free function, not interface method)
  Then returns BudgetStatus{Lines: 160, Limit: 200, NearLimit: true, OverBudget: false}
  And NearLimit true when Lines >= 150 (soft distillation threshold)

Scenario: Budget exceeded detection
  Given LEARNINGS.md has 210 lines
  When BudgetCheck called
  Then returns BudgetStatus{OverBudget: true, Lines: 210}
  And OverBudget is informational only (no forced action — file stays as-is)

Scenario: BudgetCheck handles missing file
  Given LEARNINGS.md does not exist
  When BudgetCheck called
  Then returns BudgetStatus{Lines: 0, OverBudget: false, NearLimit: false}
  And no error

Scenario: FileKnowledgeWriter replaces NoOp in runner
  Given Runner struct has Knowledge KnowledgeWriter field
  When runner.Run initializes
  Then FileKnowledgeWriter created with projectRoot path
  And replaces NoOpKnowledgeWriter as default
  And WriteProgress behavior unchanged (still writes to sprint-tasks context)

Scenario: No mutex needed (L2)
  Given architecture is sequential (single runner goroutine)
  When post-validation runs
  Then no mutex or thread safety needed (YAGNI)
  And documented: "Sequential architecture — no concurrent access"
```

**Technical Notes:**
- Files: `runner/knowledge_write.go` (FileKnowledgeWriter, post-validation, gates),
  `runner/knowledge_read.go` (buildKnowledgeReplacements, ValidateLearnings, file reading)
- **C1 model:** Claude writes → Go snapshots before session, diffs after, tags invalid entries
- **Line-count guard:** if file shrank (lines decreased), Claude may have rewritten instead of appended.
  Log warning, treat entire file content as new entries for full revalidation.
- Go does NOT write lesson content — only reads, validates, and tags `[needs-formatting]`
- **M3 struct:** `LessonEntry{Category, Topic, Content, Citation string}`, `LessonsData{Source string, Entries []LessonEntry}` — internal per-entry validation
- BudgetCheck is a free function (preserves 2-method interface contract from Epic 3)
- BudgetStatus struct: Lines int, Limit int, NearLimit bool (>=150), OverBudget bool (>=200)
- Soft threshold 150 = auto-distillation trigger; OverBudget = informational only (no forced action)
- Line counting: `strings.Count(content, "\n")` (Architecture pattern)
- File path: `{projectRoot}/LEARNINGS.md`
- Entry format: `## category: topic [source, file:line]\nAtomized fact content.\n`
  (file extension is project-dependent: .go, .py, .js, .rs, etc.)
- Optional: `VIOLATION: concrete example` inline in content (DGM research: 2.5x effectiveness)
- **Quality gate (Layer 1):** 6 gates — format, citation, dedup, budget, cap, min content
- **Named constants (L1):** G5 entry cap (5) and G6 min content (20 chars) — named constants
- **Semantic dedup (Layer 1):** `strings.Split(content, "\n## ")` → heading match by prefix after
  `strings.ToLower + strings.TrimSpace` normalization → merge
- **[needs-formatting] tag:** entries failing G1/G2 saved with tag, fixed during distillation
- **Append-only write:** Claude appends at end of file; read reverses for recency at injection
- **L2:** No mutex — sequential architecture, YAGNI. Documented as design decision.

**Prerequisites:** Story 3.7 (KnowledgeWriter interface + no-op)

---

### Story 6.2: Knowledge Injection into Prompts

**User Story:**
Как execute и review сессии, я хочу чтобы LEARNINGS.md и дистиллированные знания из `.claude/rules/ralph-*.md` загружались в prompt assembly, чтобы каждая сессия имела доступ к накопленным знаниям.

**Acceptance Criteria:**

```gherkin
Scenario: Execute prompt includes validated LEARNINGS.md content
  Given LEARNINGS.md exists with lessons
  When execute prompt assembled (Story 3.1)
  Then ValidateLearnings() filters stale entries before injection
  And content reversed: split by "\n## ", reverse section order, rejoin (L3)
  And ALL entries from LEARNINGS.md injected (old knowledge already in ralph-*.md after distillation)
  And only validated content injected via strings.Replace __LEARNINGS_CONTENT__ (FR29)
  And content available to Claude in session context

Scenario: JIT citation validation filters stale entries (M9)
  Given LEARNINGS.md has entry citing "src/old_module:42"
  And src/old_module no longer exists in project
  When ValidateLearnings(projectRoot, content) called
  Then entry excluded from valid output (stale citation)
  And validation is os.Stat file existence check only (no line range validation — M9)
  And entry included in stale output (for removal at distillation)
  And valid entries with existing files preserved

Scenario: Execute prompt includes distilled knowledge (multi-file)
  Given .claude/rules/ralph-testing.md and ralph-errors.md exist with distilled patterns
  When execute prompt assembled
  Then ALL ralph-*.md files loaded from .claude/rules/ (glob pattern)
  And ralph-misc.md always loaded (no globs in frontmatter, L5)
  And combined content injected via strings.Replace __RALPH_KNOWLEDGE__
  And injected alongside LEARNINGS.md (both present)

Scenario: Shared buildKnowledgeReplacements function (H7)
  Given 3 AssemblePrompt call sites in runner.go (initial, retry, review)
  When knowledge replacements built
  Then buildKnowledgeReplacements(projectRoot string) (map[string]string, error) used
  And function defined in runner/knowledge_read.go
  And all 3 call sites use same shared function
  And returns map with __LEARNINGS_CONTENT__ and __RALPH_KNOWLEDGE__ keys

Scenario: HasLearnings template flag (H3)
  Given LEARNINGS.md has validated non-empty content
  When TemplateData assembled
  Then HasLearnings bool set to true in TemplateData
  And execute.md template uses {{- if .HasLearnings}}...self-review section...{{- end}}

Scenario: Primacy zone positioning in prompt
  Given execute prompt template with sections
  When knowledge sections placed
  Then __RALPH_KNOWLEDGE__ placed AFTER "Sprint Tasks Format Reference" section
  And __LEARNINGS_CONTENT__ placed AFTER __RALPH_KNOWLEDGE__
  And both BEFORE "999-Rules Guardrails" section (primacy zone)
  And ordering: distilled (stable) → raw (recent) → guardrails

Scenario: Execute prompt includes self-review step
  Given execute prompt template with HasLearnings = true
  When assembled
  Then contains self-review section AFTER "Review Findings/Proceed" and BEFORE "Gates":
    "Re-read the top 5 most recent learnings. For each modified file, verify
     that the patterns from learnings are applied correctly."
  And self-review content is generic (no language-specific assumptions)
  And self-review conditional on {{- if .HasLearnings}} (H3)

Scenario: Review prompt includes knowledge files
  Given LEARNINGS.md and ralph-*.md files exist
  When review prompt assembled (Story 4.1)
  Then both validated contents injected into review prompt (FR29)
  And same placeholders as execute prompt

Scenario: Review prompt mutation asymmetry updated (M2)
  Given review prompt previously had "MUST NOT write LEARNINGS.md" invariant
  When Epic 6 updates prompt invariants
  Then review.md and execute.md invariants updated to reflect review CAN write LEARNINGS.md
  And documentation matches new behavior

Scenario: Missing knowledge files handled gracefully
  Given LEARNINGS.md does not exist
  And no .claude/rules/ralph-*.md files exist
  When prompts assembled
  Then knowledge placeholders replaced with empty string
  And HasLearnings = false, self-review section omitted
  And no error

Scenario: Golden file update with knowledge injection
  Given execute prompt golden file from Story 3.1
  When knowledge injection added
  Then golden file updated to include knowledge + self-review sections
  And `go test -update` refreshes baseline

Scenario: Knowledge sections use Stage 2 injection
  Given prompt templates contain __LEARNINGS_CONTENT__ and __RALPH_KNOWLEDGE__
  When assembly runs
  Then placeholders replaced in Stage 2 (strings.Replace, NOT text/template)
  And user content with "{{" in LEARNINGS.md does not crash assembly

Scenario: Stderr warning when LEARNINGS.md exceeds budget (M6)
  Given LEARNINGS.md has more lines than learnings_budget config value
  When session starts and prompts assembled
  Then stderr warning printed: "⚠ LEARNINGS.md: {lines}/{budget} lines ({ratio}x budget). Run `ralph distill` or switch to distill_gate: human"
  And warning is informational only (does not block session)
```

**Technical Notes:**
- Modifies: `runner/prompts/execute.md`, `runner/prompts/review.md` — add placeholder sections
- Assembly: Stage 2 replacements map gets 2 new keys
- **H7:** `buildKnowledgeReplacements(projectRoot string) (map[string]string, error)` in runner/knowledge_read.go.
  All 3 AssemblePrompt call sites use this shared function — no per-call-site changes needed
- **H3:** `HasLearnings bool` added to `config.TemplateData`. Runner sets true when validated content
  is non-empty. Template: `{{- if .HasLearnings}}...{{- end}}`
- **LEARNINGS.md = buffer of recent entries not yet distilled.** Old entries already in ralph-*.md.
  Inject ALL entries from LEARNINGS.md — no 20% filtering needed.
- **L3:** Reverse read: `strings.Split` by `\n## `, reverse section order, rejoin
- **L5:** ralph-misc.md always loaded via Stage 2 injection. NO globs in frontmatter.
- **M2:** Update prompt invariants in review.md and execute.md — review CAN write LEARNINGS.md
- **M6:** Budget overflow warning: stderr at session start when lines > budget. Informational only,
  does not block. Format: `"⚠ LEARNINGS.md: {lines}/{budget} lines ({ratio}x budget). Run `ralph distill` or switch to distill_gate: human"`
- **M9:** JIT validation = `os.Stat` only for file existence. No line range validation (Growth phase)
- **ValidateLearnings(projectRoot, content) (string, string)** — returns (valid, stale)
  - Parse by `## ` headers, extract `[file:line]` citation (any file extension)
  - `os.Stat(filepath.Join(projectRoot, file))` — exists? (no line range check)
  - Cost: O(N) stat calls, ~50 entries × ~1ms = 50ms (negligible)
  - Stale entries excluded from prompt, marked for removal at distillation
- **Multi-file read:** `filepath.Glob("{projectRoot}/.claude/rules/ralph-*.md")` → read + concatenate
  - Exclude `ralph-index.md` from concatenation (it's metadata, not rules)
- **Self-review step:** Added to execute.md after Proceed/Findings, before Gates (~50 tokens)
  - Conditional: `{{- if .HasLearnings}}` (H3)
  - Research: Live-SWE-agent +12% quality from single reflection prompt
- **Primacy zone:** knowledge after Format Reference, before Guardrails — matches prompt engineering
  best practices (format context → domain knowledge → constraints → instructions)
- Read functions: `os.ReadFile` for all files, `errors.Is(err, os.ErrNotExist)` → empty string
- TemplateData.LearningsContent already exists — wire it to actual file read
- No new dependencies

**Prerequisites:** Story 6.1 (LEARNINGS.md exists), Story 3.1 (execute prompt), Story 4.1 (review prompt)

---

### Story 6.3: Resume-Extraction Knowledge

**User Story:**
Как resume-extraction сессия, я хочу записывать причины неудачи в LEARNINGS.md, чтобы будущие сессии учились на ошибках.

**Acceptance Criteria:**

```gherkin
Scenario: Resume-extraction writes to LEARNINGS.md via Claude (C1)
  Given resume-extraction completed (Story 3.7)
  When Claude inside resume session writes lessons
  Then failure reasons written to LEARNINGS.md via file tools (FR28)
  And lessons include: what was attempted, where stuck, extracted insights
  And entry has source citation format

Scenario: Resume uses --resume with -p prompt (M1)
  Given resume-extraction invoked via --resume
  When session launched
  Then --resume and -p are compatible (fix else-if in session.go)
  And resume session gets extraction prompt directly
  And NO separate extraction session needed

Scenario: Go post-validates resume-written lessons (C1)
  Given Claude wrote lessons during resume-extraction
  When session ends
  Then Go diffs LEARNINGS.md (snapshot vs current)
  And validates new entries via FileKnowledgeWriter
  And invalid entries tagged [needs-formatting]

Scenario: Resume-extraction prompt updated with knowledge instructions
  Given resume-extraction invoked via --resume
  When prompt assembled
  Then includes instructions to extract failure insights
  And includes instructions to write findings as atomized facts to LEARNINGS.md
  And includes LEARNINGS.md format specification

Scenario: Resume-extraction with empty session context
  Given resume-extraction session has no useful failure data
  When Claude decides no lessons to write
  Then LEARNINGS.md unchanged (no diff detected)
  And no error
```

**Technical Notes:**
- Modifies: resume-extraction prompt in runner (instruction update)
- **C1:** Claude inside resume-extraction session writes to LEARNINGS.md directly via file tools.
  Go post-validates after session ends (snapshot-diff model).
- **M1:** Fix `else if` in session.go — `--resume` and `-p` are officially compatible in Claude CLI.
  Remove mutual exclusivity. Resume session gets extraction prompt directly.
- FR28: resume-extraction пишет причины неудачи + извлечённые знания
- Also triggers for sessions that simply ran out of turns (no error, just incomplete work)

**Prerequisites:** Story 6.1 (FileKnowledgeWriter), Story 3.7 (resume-extraction)

---

### Story 6.4: Review Knowledge

**User Story:**
Как review сессия с findings, я хочу записывать уроки (типы ошибок, упускаемые паттерны) в LEARNINGS.md, чтобы будущие execute сессии не повторяли те же ошибки.

**Acceptance Criteria:**

```gherkin
Scenario: Review with findings writes lessons via Claude (C1)
  Given review found CONFIRMED findings
  When Claude in review session processes findings (FR28a)
  Then Claude writes lessons to LEARNINGS.md via file tools
  And lessons include: error types, what agent forgets, patterns for future sessions
  And entries formatted as atomized facts with source citations

Scenario: Go post-validates review-written lessons (C1)
  Given Claude wrote lessons during review
  When review session ends
  Then Go diffs LEARNINGS.md (snapshot vs current)
  And validates new entries via FileKnowledgeWriter
  And invalid entries tagged [needs-formatting]

Scenario: Clean review does NOT write lessons
  Given review is clean (no findings)
  When review session completes
  Then no new content added to LEARNINGS.md
  And no knowledge files modified (beyond [x] + clear findings)

Scenario: Review prompt updated with knowledge instructions (M2)
  Given review prompt from Story 4.1
  When Epic 6 integration
  Then prompt includes: write lessons to LEARNINGS.md on findings
  And prompt includes: do NOT write lessons on clean review
  And prompt includes: atomized fact format specification
  And existing "MUST NOT write LEARNINGS.md" invariant REMOVED from review prompt
    (was at runner/prompts/review.md ~line 120, now replaced with knowledge write instructions)
  And prompt invariants documentation updated (M2)

Scenario: FR17 lessons scope now implemented
  Given FR17 lessons deferred from Epic 4
  When Epic 6 review knowledge active
  Then review writes lessons on findings (previously deferred)
  And review writes [x] + clears findings on clean (unchanged from Epic 4)
```

**Technical Notes:**
- FR28a: "Review-сессия при наличии findings сама записывает уроки в LEARNINGS.md"
- Completes FR17 deferred scope from Epic 4
- **C1:** Claude inside review session does the actual writing via file tools — not Ralph's Go code.
  Go post-validates after session ends (snapshot-diff model).
- **M2:** Update prompt invariants — review.md and execute.md now reflect that review CAN write LEARNINGS.md
- Review prompt (Story 4.1) gets additional instructions for knowledge writing
- **CRITICAL:** Remove "MUST NOT write LEARNINGS.md" from review prompt invariants
  (runner/prompts/review.md ~line 120-121) — this was correct for Epic 4 (no knowledge system)
  but must be reversed for Epic 6

**Prerequisites:** Story 6.1 (FileKnowledgeWriter), Story 4.1 (review prompt)

---

### Story 6.5a: Budget Check & Distillation Trigger

**User Story:**
Как runner, после clean review я хочу автоматически проверять размер LEARNINGS.md и при превышении
soft threshold решать о запуске дистилляции с учётом cooldown и config-driven failure handling,
чтобы дистилляция запускалась в правильное время с контролем пользователя.

**Acceptance Criteria:**

```gherkin
Scenario: Budget check after clean review — under limit
  Given clean review completed (task marked [x])
  And LEARNINGS.md has 100 lines
  When runner checks budget
  Then no action taken
  And runner proceeds to next task

Scenario: Auto-distillation trigger at soft threshold 150 lines
  Given clean review completed
  And LEARNINGS.md has 160 lines (exceeds soft threshold 150)
  And cooldown check passes: MonotonicTaskCounter - LastDistillTask >= 5 (H1)
  When runner triggers auto-distillation
  Then distillation pipeline invoked (Story 6.5b)

Scenario: Cooldown via MonotonicTaskCounter (H1)
  Given MonotonicTaskCounter in DistillState = 15
  And LastDistillTask = 12
  And LEARNINGS.md exceeds 150 lines
  When runner checks budget
  Then cooldown check: 15 - 12 = 3 < 5 → cooldown NOT met
  And no distillation triggered
  And runner continues

Scenario: Human GATE on distillation failure when distill_gate = human (C2, default)
  Given distill_gate config = "human" (default)
  And auto-distillation failed (crash, timeout >2min, bad format, validation reject, I/O error)
  When failure detected
  Then human GATE presented with error description + current file size status
  And gate options: retry once, retry 5-10 times, or skip
  And if retry: re-run distillation (up to selected count)
  And if skip: restore all backups, log warning, continue
  And runner continues normally after gate resolution

Scenario: Auto-skip distillation failure (distill_gate: auto)
  Given distill_gate config = "auto"
  And auto-distillation failed
  When failure detected
  Then circuit breaker increments consecutive failure counter
  And if consecutive failures < 3: retry once automatically
  And if consecutive failures >= 3: auto-skip distillation, restore backups, log warning
  And runner continues without human interaction
  And consecutive failure counter resets on successful distillation

Scenario: Bad format gets free retry with reinforced prompt (H4)
  Given distillation output is unparseable (missing BEGIN/END markers or bad structure)
  When failure type = bad_format
  Then ONE automatic retry with reinforced prompt instructions (no gate yet)
  And if retry also fails: gate as per distill_gate config

Scenario: Missing LEARNINGS.md — no action
  Given LEARNINGS.md does not exist
  When runner checks budget
  Then no distillation triggered
  And runner proceeds normally

Scenario: DistillFunc injectable for testing
  Given Runner struct has DistillFn field (like ReviewFn, GatePromptFn)
  When runner.Run initializes
  Then DistillFn wired to AutoDistill closure
  And tests can inject custom DistillFunc implementations
```

**Technical Notes:**
- Trigger point: runner.go Execute(), AFTER gate check, BEFORE next iteration
  (задача полностью завершена — review чистый, gate пройден)
- **distill_gate config:** `distill_gate: human|auto` (default: human). Human mode: interactive gate
  as before. Auto mode: CB auto-skip after 3 consecutive failures.
- Config additions: `distill_gate: "human"` (default), `distill_cooldown: 5`
- **H1 — MonotonicTaskCounter:** persisted in DistillState JSON, never resets. Incremented at each
  clean review. Cooldown: MonotonicTaskCounter - LastDistillTask >= 5.
- **H4 — Failure types:** crash (non-zero exit), hang (timeout >2min), bad format (unparseable output —
  free retry with reinforced prompt first), validation reject, I/O error.
- **DistillFunc injectable** — follows ReviewFn/GatePromptFn/ResumeExtractFn pattern
- BudgetCheck() from Story 6.1
- All non-fatal: any failure → gate/auto-skip → continue, NEVER interrupt task loop

**Prerequisites:** Story 6.1 (BudgetCheck + LEARNINGS.md format), Story 6.2 (knowledge injection)

---

### Story 6.5b: Distillation Session & Output Parsing

**User Story:**
Как runner, я хочу запустить `claude -p` distillation session с правильным промптом и распарсить
выходной формат в multi-file структуру, чтобы знания были компрессированы и категоризированы.

**Acceptance Criteria:**

```gherkin
Scenario: Distillation session with backup and timeout
  Given auto-distillation triggered (Story 6.5a)
  When distillation starts
  Then backup created: LEARNINGS.md.bak + .bak.1 (2-generation, L4)
  And all existing ralph-*.md files backed up with 2-generation rotation
  And distillation prompt assembled with LEARNINGS.md content
  And distillation prompt includes project scope hints (M4)
  And `claude -p` session runs with context.WithTimeout(ctx, 2*time.Minute) (H8)

Scenario: Distillation timeout (H8)
  Given distillation session running
  When 2 minutes elapsed
  Then context.WithTimeout cancels session
  And treated as distillation failure (triggers gate per Story 6.5a)
  And timeout configurable via distill_timeout config field (default: 120)

Scenario: Distillation prompt instructions
  Given distillation prompt assembled
  When Claude processes LEARNINGS.md content
  Then distillation prompt instructs: compress to <=100 lines (50% budget)
  And distillation prompt instructs: remove stale-cited entries
  And distillation prompt instructs: merge duplicate categories
  And distillation prompt instructs: fix all [needs-formatting] entries
  And distillation prompt instructs: output grouped by category for multi-file split
  And distillation prompt instructs: auto-promote categories with >=6 entries -> ralph-{category}.md
  And distillation prompt instructs: promote [freq:N>=10] entries -> ralph-critical.md
  And distillation prompt instructs: add ANCHOR marker to entries with freq >= 10 (L4)
  And distillation prompt instructs: preserve ANCHOR entries unchanged
  And distillation prompt instructs: preserve `VIOLATION:` markers for high-frequency patterns
  And distillation prompt instructs: assign freq:N to entries (M11)
  And distillation prompt instructs: use output protocol BEGIN_DISTILLED_OUTPUT/END_DISTILLED_OUTPUT (H6)
  And distillation prompt instructs: use ## CATEGORY: <name> sections (H6)
  And distillation prompt instructs: use NEW_CATEGORY: <name> for new categories (H2)
  And distillation prompt instructs: use only canonical categories: testing, errors, config, cli, architecture, performance, security + misc (H2)

Scenario: Output parsing with BEGIN/END markers (H6)
  Given distillation session completed
  When Go parses output
  Then Go parses only between BEGIN_DISTILLED_OUTPUT / END_DISTILLED_OUTPUT markers (H6)
  And category sections parsed by ## CATEGORY: <name> headers
  And NEW_CATEGORY: <name> markers detected and processed

Scenario: Multi-file category output with scope hints
  Given auto-distillation output parsed successfully
  When output written to files
  Then entries grouped by category -> separate ralph-{category}.md files
  And each file has YAML frontmatter with scope hints: `globs: [<patterns>]`
  And scope hints auto-detected from project file types (M4)
  And Go scans top 2 levels of project, collects file extensions, maps to known language globs
  And Claude uses scope info to create globs, Go validates glob syntax with filepath.Match
  And minimum 3 rules per file (smaller categories merged into ralph-misc.md)
  And ralph-misc.md has NO globs in frontmatter — always loaded (L5)
  And high-frequency rules (freq:N>=10) written to ralph-critical.md with globs: ["**"]
  And ANCHOR marker automatically added to entries with freq >= 10 (L4)

Scenario: LEARNINGS.md replaced with compressed output
  Given distillation output valid (passes ValidateDistillation from Story 6.5c)
  When output written
  Then LEARNINGS.md replaced with compressed output
  And auto-promoted categories written to .claude/rules/ralph-{category}.md with scope hints
  And log: "Auto-distilled LEARNINGS.md (160->N lines, K categories)"

Scenario: Index file auto-generation
  Given auto-distillation completed successfully
  When ralph-*.md files written
  Then ralph-index.md generated automatically
  And lists all ralph-*.md files with: category, entry count, scope hints, last updated
  And format: markdown table for human readability

Scenario: T1 promotion via ralph-critical.md
  Given distillation detects entries with [freq:N] where N >= 10
  When entry promoted to T1
  Then written to .claude/rules/ralph-critical.md with globs: ["**"] (always loaded)
  And ANCHOR marker added (L4)
  And original entry in ralph-{category}.md replaced with reference
  And log: "T1 promoted: <topic> (freq:N)"

Scenario: Freq:N validation (M11)
  Given distillation output contains [freq:N] markers
  When Go validates output
  Then checks monotonicity: new freq >= old freq for same entry
  And corrects Claude's arithmetic errors if detected
  And validated freq values written to output

Scenario: NEW_CATEGORY proposal (H2)
  Given distillation output contains NEW_CATEGORY: <name> marker
  When Go parses output
  Then new category added to canonical list in DistillState
  And ralph-index.md updated with new category
  And category list only grows, never shrinks
```

**Technical Notes:**
- File: `runner/knowledge_distill.go` — AutoDistill, distillation prompt, output parsing, multi-file write
- **3-layer architecture:**
  - Layer 1 (Go, 0 tokens): semantic dedup in post-validation (Story 6.1)
  - Layer 2 (LLM, ~8K tokens): auto `claude -p` at 150-line soft threshold
  - Layer 3 (Go, 0 tokens): gate/auto-skip on failure, cooldown, post-validation
- **H6 — Output protocol:** BEGIN_DISTILLED_OUTPUT / END_DISTILLED_OUTPUT markers. Category sections:
  `## CATEGORY: <name>`. New category proposal: `NEW_CATEGORY: <name>`. Go parses only between markers.
- **H2 — Canonical categories:** testing, errors, config, cli, architecture, performance, security + misc.
  Claude can propose NEW_CATEGORY when misc is too large. Go adds to list in DistillState. List only grows.
- **H8 — Timeout:** `context.WithTimeout(ctx, 2*time.Minute)`. Config: `distill_timeout: 120`
- **M4 — Scope hints:** Go scans top 2 levels of project, collects file extensions, maps to known
  language globs. Passes scope info to distillation prompt. Claude uses it. Go validates with filepath.Match.
- **M11 — freq:N:** Claude assigns during distillation. Go validates monotonicity (new >= old).
  Go corrects arithmetic errors.
- **L4 — 2-generation backups:** .bak + .bak.1. ANCHOR marker on freq >= 10 entries. Claude MUST preserve.
- **L5 — ralph-misc.md:** always loaded, no globs. H2 NEW_CATEGORY prevents unbounded growth.
- **Multi-file output:** distillation prompt outputs `## CATEGORY: <name>` sections, Go code
  splits into ralph-{category}.md files with auto-generated YAML frontmatter (scope hints)
- **Backup:** ALL ralph-*.md + LEARNINGS.md backed up with 2-generation rotation before distillation
- **T1 promotion:** high-frequency (>=10 occurrences) → ralph-critical.md (globs: ["**"])
  Safe: writes to .claude/rules/ only (no config editing, no hook modification, zero CVE risk)
- **Violation frequency:** `[freq:N]` marker in distilled entries, incremented by distillation prompt
- Config additions: `distill_target_pct: 50`, `distill_timeout: 120`
- Token cost: ~8K per distillation (~30K/100 tasks, 0.015× one execute session)
- File paths: `{projectRoot}/LEARNINGS.md`, `{projectRoot}/.claude/rules/ralph-*.md`

**Prerequisites:** Story 6.5a (trigger + gate logic)

---

### Story 6.5c: Distillation Validation & State

**User Story:**
Как runner, я хочу post-validation выходных данных дистилляции и надёжное хранение state
с crash recovery, чтобы дистилляция была безопасной и восстанавливаемой.

**Acceptance Criteria:**

```gherkin
Scenario: Post-validation rejects bad distillation (ValidateDistillation)
  Given auto-distillation produced output
  When ValidateDistillation(old, new) runs
  Then checks 8 criteria:
    1. Output <= 200 lines
    2. Topic headers preserved (no category loss)
    3. Entries from last 20% of original file preserved (H5 — distillation safety net)
    4. Citation preservation >= 80%
    5. No duplicate entries
    6. Category count preserved >= 80% of original categories
    7. All [needs-formatting] entries either fixed or preserved (none silently dropped)
    8. All ralph-*.md have valid YAML frontmatter with globs: field (M8)
  And if any check fails: treated as distillation failure (triggers gate per Story 6.5a)

Scenario: DistillState persisted in .ralph/distill-state.json
  Given distillation state needs persistence
  When DistillState serialized
  Then stored at `{projectRoot}/.ralph/distill-state.json`
  And contains: Version int, MonotonicTaskCounter int, LastDistillTask int, Categories []string, Metrics struct
  And Version field provides forward compatibility

Scenario: DistillState includes Version field for forward compatibility
  Given DistillState JSON structure
  When deserialized
  Then Version int field present (current: 1)
  And future versions can migrate from older formats
  And missing Version treated as Version: 0 (pre-versioning)

Scenario: DistillState included in backup rotation
  Given distillation about to run
  When backups created (2-generation)
  Then .ralph/distill-state.json backed up alongside LEARNINGS.md and ralph-*.md
  And backup rotation: .bak + .bak.1 (same as other files)

Scenario: Crash recovery at startup (M7)
  Given runner starts and finds .bak files for LEARNINGS.md or ralph-*.md
  When startup check runs
  Then .bak files restored to original paths
  And .ralph/distill-state.json.bak restored if present
  And log warning: "Recovered from interrupted distillation"

Scenario: Effectiveness metrics after distillation
  Given auto-distillation completed
  When metrics computed
  Then log includes: entries before/after, stale removed count, categories preserved/total,
    [needs-formatting] fixed count, T1 promotions count
  And metrics written to DistillState for trend tracking
```

**Technical Notes:**
- File: `runner/knowledge_state.go` — DistillState, serialization, crash recovery
- **DistillState** persisted in `{projectRoot}/.ralph/distill-state.json` (JSON):
  Version int, MonotonicTaskCounter int, LastDistillTask int, Categories []string, Metrics struct
- **Version field:** forward compatibility — allows migration from older formats. Current version: 1.
- **Backup rotation:** DistillState backed up with 2-generation rotation alongside LEARNINGS.md and ralph-*.md
- **M7 — Crash recovery:** At startup, check for .bak files → restore → log warning.
  Covers LEARNINGS.md, ralph-*.md, and .ralph/distill-state.json
- **Post-validation (ValidateDistillation):** Go code, deterministic, checks 8 criteria
- **H5 — criterion #3:** "last 20% of entries" in ValidateDistillation is a safety net for distillation —
  ensures recent entries are not lost during compression. This is about preservation during distillation,
  not about injection filtering.
- **Effectiveness metrics:** logged + stored in DistillState for trend tracking
- **No forced truncation:** user decision — 300+ lines = 3-4% of 200K context, linear decay.
  If human skips distillation, file stays as-is. No FIFO. No archive.

**Prerequisites:** Story 6.5b (distillation output to validate)

---

### Story 6.6: Distillation CLI — ralph distill (Manual Override)

**User Story:**
Как разработчик, я хочу команду `ralph distill` как ручной override для принудительной дистилляции
LEARNINGS.md, чтобы при необходимости можно было запустить сжатие вне автоматического цикла (Story 6.5a/b/c).

**Context:** Auto-distillation (Story 6.5a/b/c) — основной механизм. `ralph distill` — дополнительный manual
override для случаев когда пользователь хочет принудительную дистилляцию. CLI reuses тот же DistillFunc
и distillation prompt из Story 6.5b.

**Acceptance Criteria:**

```gherkin
Scenario: ralph distill compresses LEARNINGS.md
  Given LEARNINGS.md has 180 lines of raw learnings
  When `ralph distill` executed
  Then reads LEARNINGS.md content
  And assembles distillation prompt (same as Story 6.5b auto-distill)
  And runs `claude -p` (pipe mode, non-interactive)
  And post-validation via ValidateDistillation (same as Story 6.5c)
  And if valid: LEARNINGS.md replaced with compressed output
  And auto-promoted categories written to .claude/rules/ralph-{category}.md
  And ralph-index.md regenerated

Scenario: Backup before distillation (L4)
  Given .claude/rules/ralph-*.md files exist with previous content
  When ralph distill runs
  Then creates LEARNINGS.md.bak + .bak.1 (2-generation rotation)
  And backs up all ralph-*.md with 2-generation rotation
  And backs up .ralph/distill-state.json with 2-generation rotation
  And backups preserved until next distill run

Scenario: Distillation failure — interactive retry
  Given distillation session fails (non-zero exit or validation fails)
  When error handled
  Then error message displayed to user with current file size
  And user prompted: retry or abort
  And if abort: all backups restored
  And exit code 1

Scenario: Missing source file
  Given LEARNINGS.md does not exist
  When `ralph distill` executed
  Then error: "LEARNINGS.md not found — nothing to distill"
  And exit code 1

Scenario: Cobra subcommand wiring
  Given `ralph` CLI
  When `ralph distill` invoked
  Then Cobra dispatches to distill subcommand
  And uses config.ProjectRoot for file paths

Scenario: Advisory concurrent run note (L6)
  Given ralph distill documentation
  When user reads help text
  Then advisory note warns: do not run `ralph distill` + `ralph run` concurrently
  And no file lock code (L6 — advisory only)
```

**Technical Notes:**
- New file: `cmd/ralph/distill.go` — Cobra subcommand
- **Reuses:** `runner/prompts/distillation.md` from Story 6.5b (same prompt, same validation)
- **Reuses:** `AutoDistill()` from Story 6.5b / `ValidateDistillation()` from Story 6.5c — CLI wraps same logic
- Distillation session: `session.Execute` with pipe mode (`-p` flag via Options.Prompt)
- Output target: `{projectRoot}/.claude/rules/ralph-{category}.md` (auto-loaded by Claude Code)
- **L4:** 2-generation backups (.bak + .bak.1), including .ralph/distill-state.json
- **L6:** Advisory note in help text about not running concurrently. No file lock code.
- Exit code mapping: 0=success, 1=error (reuses cmd/ralph exit patterns)
- Manual override — дополняет автоматический процесс, не заменяет

**Prerequisites:** Story 6.5b (AutoDistill, distillation prompt), Story 6.5c (ValidateDistillation), Story 1.10 (prompt assembly)

---

### Story 6.7: Serena MCP Integration

**User Story:**
Как runner, я хочу обнаруживать Serena MCP server и добавлять prompt hint для использования её tools, чтобы улучшить code navigation при наличии Serena.

**Acceptance Criteria:**

```gherkin
Scenario: Serena MCP detected via config files (C3)
  Given .claude/settings.json or .mcp.json contains Serena MCP config
  When ralph run starts
  Then CodeIndexerDetector.Available(projectRoot) returns true
  And logs "Serena MCP detected"

Scenario: Detection reads config files only (C3)
  Given Serena MCP detection needed
  When Available(projectRoot) called
  Then reads .claude/settings.json or .mcp.json for Serena MCP config
  And NO exec.LookPath("serena") used
  And NO serena index --full called
  And Ralph does NOT call Serena directly

Scenario: Prompt hint injected when available (C3)
  Given Serena MCP detected
  When prompt assembled
  Then CodeIndexerDetector.PromptHint() returns hint string
  And hint: "If Serena MCP tools available, use them for code navigation"
  And hint injected into execute and review prompts

Scenario: Serena unavailable graceful fallback
  Given .claude/settings.json has no Serena config
  And .mcp.json does not exist
  When ralph run starts
  Then CodeIndexerDetector.Available() returns false
  And no Serena prompt hint injected
  And runner operates normally
  And no error

Scenario: Serena configurable
  Given config with serena_enabled: false
  When ralph run starts
  Then skips Serena detection entirely
  And no Serena-related output

Scenario: Minimal CodeIndexerDetector interface (M5/C3)
  Given CodeIndexerDetector interface
  When implemented
  Then only 2 methods: Available(projectRoot string) bool, PromptHint() string
  And no index commands, no timeout management, no progress output
```

**Technical Notes:**
- **C3:** Serena is MCP server, NOT CLI. Detection via `.claude/settings.json` or `.mcp.json`.
  Ralph does NOT call Serena directly. Only provides prompt hint.
- **M5:** Minimal interface: `CodeIndexerDetector{Available(projectRoot) bool, PromptHint() string}`
- Config: `serena_enabled` (default true) — already in config.go
- `serena_timeout` config field can be removed or kept as no-op (no longer used for index calls)
- Detection: `os.ReadFile` + JSON parse for `.claude/settings.json` / `.mcp.json`
- Best-effort: any detection failure = Available() returns false

**Prerequisites:** Story 3.5 (runner loop — insertion point)

---

### Story 6.8: Final Integration Test

**User Story:**
Как разработчик, я хочу финальный end-to-end integration test всего продукта, чтобы убедиться что все
6 эпиков работают вместе — включая knowledge pipeline с human-gated distillation и multi-file output.

**Acceptance Criteria:**

```gherkin
Scenario: FINAL — full end-to-end flow with auto-knowledge (C1)
  Given scenario JSON covering full flow:
    bridge -> execute (commit) -> review (findings) -> execute fix (commit) -> review (clean)
    -> budget check -> knowledge post-validated -> Serena hint injected
  And MockClaude + MockGitClient + mock Serena detection + mock DistillFn
  And sprint-tasks.md from bridge golden file
  When runner.Run executes with all features
  Then bridge output feeds runner
  And execute sessions launch with knowledge context (__LEARNINGS_CONTENT__ injected)
  And review finds and verifies findings
  And fix cycle produces clean review
  And [x] marked + review-findings cleared
  And Claude writes lessons to LEARNINGS.md (on findings review)
  And Go post-validates lessons after review session (snapshot-diff model)
  And budget check runs after clean review (no distill — under limit)
  And Serena prompt hint present when detected
  And all 6 epics work together

Scenario: FINAL — gates + knowledge + emergency
  Given gates_enabled = true, --every 2
  And scenario with 3 tasks: task1 (clean), task2 (emergency->skip), task3 (clean)
  And mock stdin for gate actions
  When runner.Run executes
  Then checkpoint gate fires after task 2
  And emergency gate fires for task 2 (max retries)
  And skip advances to task 3
  And knowledge written throughout (LEARNINGS.md has entries from both resume-extraction and review)

Scenario: FINAL — auto-distillation multi-file output with human gate
  Given LEARNINGS.md starts with 140 lines
  And review writes ~20 lines of lessons (total >150 soft threshold)
  And MonotonicTaskCounter - LastDistillTask >= 5 (cooldown met)
  And mock DistillFn returns compressed content with 2 categories
  When clean review completes and budget check runs
  Then auto-distillation triggered (DistillFn called)
  And LEARNINGS.md replaced with compressed output
  And ralph-{category}.md files created in .claude/rules/
  And ralph-index.md generated
  And log contains "Auto-distilled LEARNINGS.md"
  And next execute session gets distilled knowledge context from ralph-*.md files

Scenario: FINAL — distillation failure triggers human gate (C2)
  Given auto-distillation fails (mock returns error)
  And distill_gate = "human" (default)
  When failure detected
  Then human gate presented with error description
  And if skip chosen: all backups restored, LEARNINGS.md unchanged
  And runner continues normally
  And NO circuit breaker state checked

Scenario: FINAL — distillation failure auto-skip (distill_gate: auto)
  Given auto-distillation fails 3 times consecutively (mock returns errors)
  And distill_gate = "auto"
  When failure detected
  Then auto-skip after 3rd failure, backups restored
  And runner continues without human interaction
  And log warning about auto-skipped distillation

Scenario: FINAL — JIT citation validation filters stale
  Given LEARNINGS.md has 5 entries, 2 citing deleted files
  When execute prompt assembled
  Then ValidateLearnings filters 2 stale entries (os.Stat only, M9)
  And only 3 valid entries injected into prompt (__LEARNINGS_CONTENT__)
  And HasLearnings = true, self-review step present (H3)

Scenario: FINAL — resume + knowledge flow (M1)
  Given scenario: execute (no commit) -> resume-extraction -> retry -> execute (commit)
  When runner.Run executes
  Then resume-extraction runs with --resume + -p (compatible, M1)
  And Claude writes knowledge to LEARNINGS.md during resume
  And Go post-validates after resume session
  And LEARNINGS.md accumulates from resume source
  And retry execute includes knowledge context from previous failure

Scenario: FINAL — Serena MCP detection fallback (C3)
  Given .claude/settings.json has no Serena config
  When runner.Run executes
  Then CodeIndexerDetector.Available() returns false
  And no Serena prompt hint injected
  And runner completes full flow without Serena
  And no errors from Serena absence

Scenario: FINAL — [needs-formatting] tag and fix cycle
  Given LEARNINGS.md has 2 entries with [needs-formatting] tag
  And LEARNINGS.md exceeds 150 lines (triggers distillation)
  And mock DistillFn returns output with [needs-formatting] entries fixed
  When auto-distillation runs
  Then [needs-formatting] entries present in distillation input
  And output has properly formatted entries (tags removed, format fixed)
  And ValidateDistillation criterion #7 passes (all [needs-formatting] handled)

Scenario: FINAL — crash recovery at startup (M7)
  Given LEARNINGS.md.bak exists from interrupted distillation
  And .ralph/distill-state.json.bak exists
  When runner starts
  Then .bak files restored to originals
  And log warning: "Recovered from interrupted distillation"
  And runner proceeds normally

Scenario: FINAL — cross-language scope hints (M12)
  Given multiple project stacks: Go, Python, JS/TS, Java, mixed (fullstack: JS+Python, Java+Go)
  When distillation generates scope hints for each stack
  Then scope hints match language conventions (M4)
  And categories appropriate for each language
  And citations valid for each file type
```

**Technical Notes:**
- Test file: `runner/runner_final_integration_test.go`
- Build tag: `//go:build integration`
- Most comprehensive test in the project — covers all 6 epics
- MockClaude scenario-based JSON (ordered responses, via config.ClaudeCommand)
- Mock DistillFn: injectable (same pattern as ReviewFn, GatePromptFn)
- Mock Serena detection: CodeIndexerDetector mock (returns true/false)
- Reuses test helpers from runner/test_helpers_test.go
- Auto-distillation scenarios verify: trigger threshold, human gate, auto-skip, multi-file output
- Citation validation scenario: create temp files, delete some, verify filtering
- [needs-formatting] scenario: verify tag → fix → validation cycle
- **M12 — Cross-language tests:** Go, Python, JS/TS, Java, mixed stacks
- --always-extract NOT tested here (deferred to Growth)

**Prerequisites:** Story 6.1-6.7, 6.5a-6.5c (all prior Epic 6 stories), Story 5.6 (gates integration), Story 4.8 (review integration), Story 3.11 (runner integration)

---

### Story 6.9: Knowledge Effectiveness Trend Tracking

**User Story:**
Как разработчик, я хочу отслеживать тренды эффективности knowledge injection через метрики
findings_per_task и first_clean_review_rate, чтобы понимать улучшает ли knowledge система качество
кода со временем.

**Acceptance Criteria:**

```gherkin
Scenario: Findings per task metric tracked
  Given task completed with review
  When metrics updated
  Then findings_per_task computed: total findings / total tasks (rolling window)
  And metric stored in DistillState.Metrics
  And updated after each review completion

Scenario: First clean review rate tracked
  Given task completed with review
  When metrics updated
  Then first_clean_review_rate computed: tasks with clean first review / total tasks (rolling window)
  And metric stored in DistillState.Metrics
  And updated after each review completion

Scenario: Trend data persisted across sessions
  Given DistillState with existing metrics
  When new session completes tasks
  Then metrics accumulated (not reset)
  And trend visible over time via DistillState JSON
  And log: "Knowledge metrics: findings/task={N}, first-clean-rate={P}%"

Scenario: Metrics available in DistillState JSON
  Given multiple sessions completed
  When user inspects .ralph/distill-state.json
  Then Metrics struct contains: FindingsPerTask float64, FirstCleanReviewRate float64, TotalTasks int, TotalFindings int, CleanFirstReviews int
  And data sufficient for external trend analysis
```

**Technical Notes:**
- Metrics stored in DistillState.Metrics (in `.ralph/distill-state.json`)
- No A/B mode switching — single injection mode (scoped via ralph-*.md)
- No config flag for injection mode — always scoped
- Minimal implementation: update counters in runner.go after review completion
- Rolling window not needed for v1 — simple cumulative counters sufficient
- External tools can read .ralph/distill-state.json for trend analysis
- findings_per_task = TotalFindings / TotalTasks
- first_clean_review_rate = CleanFirstReviews / TotalTasks * 100

**Prerequisites:** Story 6.2 (knowledge injection), Story 6.5c (DistillState)

---

### Epic 6 Summary

| Story | Title | FRs | Key Files | AC Count |
|:-----:|-------|:---:|:---------:|:--------:|
| 6.1 | FileKnowledgeWriter — LEARNINGS.md Post-Validation | FR27 | runner/knowledge_write.go, runner/knowledge_read.go | 10 |
| 6.2 | Knowledge Injection into Prompts | FR29 | runner/prompts/*.md, config/prompt.go | 13 |
| 6.3 | Resume-Extraction Knowledge | FR28 | runner/ (prompt update), session/session.go | 5 |
| 6.4 | Review Knowledge | FR28a | runner/ (prompt update) | 5 |
| 6.5a | Budget Check & Distillation Trigger | FR27,FR28a | runner/knowledge_write.go, runner/runner.go | 8 |
| 6.5b | Distillation Session & Output Parsing | FR27,FR28a | runner/knowledge_distill.go | 11 |
| 6.5c | Distillation Validation & State | FR27 | runner/knowledge_state.go | 6 |
| 6.6 | Distillation CLI — ralph distill (Manual Override) | FR27 | cmd/ralph/distill.go | 6 |
| 6.7 | Serena MCP Integration | FR39 | runner/runner.go | 6 |
| 6.8 | Final Integration Test | ALL | runner/runner_final_integration_test.go | 12 |
| 6.9 | Knowledge Effectiveness Trend Tracking | FR29 | runner/knowledge_state.go, runner/runner.go | 4 |
| | **Total** | **FR26-FR29,FR28a,FR39** | | **86** |

**FR Coverage:**
- FR26: Satisfied vacuously — ralph does NOT modify project CLAUDE.md (zero corruption risk)
- FR27: 6.1 (post-validation), 6.5a/b/c (auto-distillation → ralph-{category}.md), 6.6 (manual override)
- FR28: 6.3 (resume-extraction writes lessons via Claude)
- FR28a: 6.4 (review writes lessons via Claude on findings), 6.5a/b (budget enforcement + auto-distill)
- FR29: 6.2 (knowledge loaded into execute/review prompts, JIT citation validation), 6.9 (trend tracking)
- FR39: 6.7 (Serena MCP detection + prompt hint)
- FR28b (--always-extract): Deferred to Growth (config scaffold exists)

**Architecture: Two-Tier Knowledge System + Progressive Disclosure (Human-Gated Distillation)**
```
Tier 1 — Hot (LEARNINGS.md, soft threshold 150 lines)
  Claude writes directly via file tools (C1)
  Go snapshots before session, diffs after, tags invalid [needs-formatting] (C1)
  Line-count guard: if lines decreased, log warning + full revalidation (C1)
  Append-only write, reverse read: split by "\n## ", reverse, rejoin (L3)
  All entries injected (LEARNINGS.md = recent buffer, old knowledge in ralph-*.md)
  Quality gate on post-validation: 6 gates — format, citation, dedup, budget, cap(5), min(20) (L1)
  Invalid entries saved with [needs-formatting] tag (no knowledge loss)
  Optional VIOLATION: markers for concrete examples (DGM: 2.5x effectiveness)
  JIT citation validation on read: os.Stat only (M9), stale entries filtered out (Story 6.2)
  Injected into ralph prompts via Stage 2 (__LEARNINGS_CONTENT__)
  Auto-distilled at 150 lines via claude -p with 2-min timeout (H8)
  Budget overflow: stderr warning at session start when lines > budget (M6)
  No forced truncation: 300+ lines = 3-4% context, linear decay (user decision)

Tier 2 — Distilled (.claude/rules/ralph-*.md, multi-file by category)
  Auto-promoted from Tier 1 during auto-distillation (categories >=6 entries)
  Canonical categories: testing, errors, config, cli, architecture, performance, security + misc (H2)
  NEW_CATEGORY proposal via marker (H2) — list only grows, never shrinks
  Output protocol: BEGIN_DISTILLED_OUTPUT / END_DISTILLED_OUTPUT markers (H6)
  Each file has scope hints (globs) for contextual loading by Claude Code
  ralph-misc.md: always loaded, no globs (L5)
  Manual override via `ralph distill` CLI (Story 6.6)
  Auto-loaded by Claude Code for ALL sessions (bonus visibility)
  Injected into ralph prompts via Stage 2 (__RALPH_KNOWLEDGE__)
  Trend tracking: findings_per_task, first_clean_review_rate (Story 6.9)
  Three sub-tiers (Progressive Disclosure T1-T3, R2-R6 pattern):
    T1: ralph-critical.md — globs: ["**"] — always loaded, freq:N>=10 + ANCHOR (L4)
    T2: ralph-{category}.md — globs: [<specific>] — contextual loading
    T3: LEARNINGS.md — injected via Stage 2 — full raw knowledge
  Index: ralph-index.md — auto-generated TOC with category, count, scope, updated

R1-R7 Knowledge Enforcement (proven patterns from 5 epics):
  R2-R1: Multi-file by category (ralph-{category}.md, min 3 rules/file)
  R2-R2: T1 promotion to ralph-critical.md (safe: .claude/rules/ only, zero CVE risk)
  R2-R5: Violation frequency tracking — [freq:N] markers, Claude assigns, Go validates monotonicity (M11)
  R2-R6: Progressive disclosure T1-T3 (critical -> scoped -> raw)
  R2-R7: Scope hints in YAML frontmatter (globs for contextual loading)
  R6:    Effectiveness metrics (entries before/after, stale %, categories, fixes)

Safety nets (human-gated or auto-skip, configurable via distill_gate):
  - Layer 1: Go-level semantic dedup on every post-validation (0 tokens)
  - Layer 2: Auto claude -p distillation at 150 lines (~8K tokens, 2-min timeout H8)
  - Layer 3: distill_gate: human → Human GATE on every failure (C2) — retry/skip options
  - Layer 3: distill_gate: auto → CB auto-skip after 3 consecutive failures (C2)
  - Layer 3: Cooldown via MonotonicTaskCounter (H1) — >=5 tasks between distillations
  - Post-validation: 8 criteria check on distillation output (including YAML frontmatter M8)
  - Self-review step in execute prompt via HasLearnings flag (H3, +12% quality)
  - [needs-formatting] tag preserves knowledge (fix at distillation, no loss)
  - 2-generation backups: .bak + .bak.1 (L4)
  - ANCHOR marker on freq>=10 entries — must preserve at distillation (L4)
  - Crash recovery: restore .bak files at startup (M7)
  - No FIFO, no archive — file stays as-is on skip (safe degradation)
  - Advisory note: don't run ralph distill + ralph run concurrently (L6)

DistillState: {projectRoot}/.ralph/distill-state.json
  - Version int (forward compatibility)
  - MonotonicTaskCounter, LastDistillTask, Categories, Metrics
  - Backed up with 2-generation rotation
  - Crash recovery restores from .bak

Knowledge source files (knowledge.go split):
  - runner/knowledge_write.go — FileKnowledgeWriter, post-validation, gates
  - runner/knowledge_read.go — buildKnowledgeReplacements, ValidateLearnings, file reading
  - runner/knowledge_distill.go — AutoDistill, distillation prompt, output parsing, multi-file write
  - runner/knowledge_state.go — DistillState, serialization, crash recovery
```

**Dependency Graph:**
```
3.7 ────→ 6.1 ──→ 6.2 ──→ 6.8
               ├──→ 6.3 ──→ 6.8
               ├──→ 6.4 ──→ 6.8
               ├──→ 6.5a ──→ 6.5b ──→ 6.5c ──→ 6.6 ──→ 6.8
               │                  ╰──→ 6.9 ──→ 6.8
3.5 ────→ 6.7 ──────────────────→ 6.8
6.2 ─────────────→ 6.9 ──→ 6.8
```

**Parallelism opportunities:**
- 6.2 и 6.7 — independent after 6.1 (prompt injection vs Serena MCP)
- 6.3 и 6.4 — parallel-capable (resume vs review knowledge)
- 6.5a depends on 6.1+6.2; 6.5b depends on 6.5a; 6.5c depends on 6.5b
- 6.6 depends on 6.5b+6.5c (reuses distill logic + validation)
- 6.9 depends on 6.2+6.5c (injection + distill state)
- 6.8 — strictly last (depends on all)

**Changes vs v1:**
- Story 6.4 (CLAUDE.md Section Management) → REMOVED. Ralph does not touch CLAUDE.md.
- Story 6.2+6.3 (Distillation Prompt + Auto-Trigger) → REPLACED by 6.5 (fully automatic distillation)
- Story 6.5 (Knowledge Loading) → Moved to 6.2 (earlier), expanded with JIT validation + self-review
- Story 6.6+6.7 (Resume/Review Knowledge) → Renumbered to 6.3+6.4
- Story 6.6 (manual CLI) now depends on 6.5 (reuses auto-distill infrastructure)
- Story 6.8 (Serena) → Renumbered to 6.7, kept as-is
- Story 6.9 (--always-extract + Final Test) → Split: flag deferred, test kept as 6.8
- v2→v3: Manual distillation replaced with fully automatic 3-layer architecture
- v2→v3: Added quality gates (G1-G6), citation validation, self-review, circuit breaker
- v3 post-verification refinements (6-agent consensus + user challenge sessions):
  - Removed emergency FIFO (300+ lines safe at 3-4% context, linear decay)
  - Removed auto-archive (LEARNINGS.archive.md eliminated — no forced knowledge removal)
  - Multi-file by category (ralph-{category}.md) instead of single ralph-learnings.md
  - [needs-formatting] tag instead of entry rejection (zero knowledge loss)
  - R1-R7 knowledge enforcement patterns applied (multi-file, T1-T3, freq tracking, metrics)
  - Index file auto-generation (ralph-index.md)
  - Header normalization for semantic dedup (ToLower+TrimSpace)
  - Inline violation examples support (VIOLATION: marker, DGM 2.5x)
  - Append-only write + reverse read (newest-first at injection)
  - Primacy zone positioning (knowledge after Format Reference, before Guardrails)
- v3→v4: 10-agent review (28 decisions: C1-C4, H1-H8, M1-M12, L1-L6):
  - C1: Claude writes directly to LEARNINGS.md, Go post-validates (snapshot-diff)
  - C2: Human GATE replaces circuit breaker on distillation failures
  - C3: Serena = MCP server, not CLI. Detection via config files, prompt hint only
  - H1: MonotonicTaskCounter for cooldown (never resets)
  - H2: Fixed canonical 7+misc categories, NEW_CATEGORY proposal mechanism
  - H3: HasLearnings bool in TemplateData for conditional self-review
  - H5: "Last 20% of entries" replaces "last 3 sessions"
  - H6: BEGIN/END markers output protocol for distillation
  - H7: Shared buildKnowledgeReplacements function for all 3 call sites
  - H8: 2-minute timeout for distillation sessions
  - M1: --resume + -p compatible (fix else-if in session.go)
  - M2: Prompt invariants updated — review CAN write LEARNINGS.md
  - M3: LessonEntry struct for per-entry validation
  - M4: Go-side project scope hint detection for distillation
  - M7: Crash recovery — restore .bak files at startup
  - M8: YAML frontmatter validation + new Story 6.9 (A/B testing)
  - M9: JIT validation = os.Stat only (no line range)
  - M11: freq:N monotonicity validation by Go
  - M12: Cross-language test scenarios (Go, Python, JS/TS, Java, mixed)
  - L1: Named constants for G5 (cap=5) and G6 (min=20 chars)
  - L2: No mutex (YAGNI, sequential architecture)
  - L3: Reverse read via split/reverse/rejoin
  - L4: 2-generation backups + ANCHOR marker for freq>=10
  - L5: ralph-misc.md always loaded, no globs
  - L6: Advisory note about concurrent runs (no file lock)
  - New Story 6.9: A/B testing — scoped vs flat injection modes
- v4→v5: 6-agent review (3 pairs analytic+architect, 11 corrections):
  - C1: Line-count guard in snapshot-diff — detect LEARNINGS.md rewrite (lines decreased → full revalidation)
  - C2: distill_gate config: human|auto (default: human). Auto mode: CB auto-skip after 3 consecutive failures
  - M6: Stderr warning when LEARNINGS.md exceeds budget at session start (informational, non-blocking)
  - H5: Inject ALL entries from LEARNINGS.md (not 20%) — LEARNINGS.md is recent buffer, old knowledge in ralph-*.md
  - Story 6.5 split into 6.5a (trigger+gate), 6.5b (session+parsing), 6.5c (validation+state) — manageable scope
  - DistillState moved to .ralph/distill-state.json with Version field for forward compatibility
  - DistillState included in backup rotation (2-generation)
  - knowledge.go split into 4 files: knowledge_write.go, knowledge_read.go, knowledge_distill.go, knowledge_state.go
  - Story 6.9 simplified: A/B testing → trend tracking (findings_per_task, first_clean_review_rate)
  - All LEARNINGS.md.state references updated to .ralph/distill-state.json
  - Total: 11 stories (was 9 in v4), 86 AC (was 78 in v4)
