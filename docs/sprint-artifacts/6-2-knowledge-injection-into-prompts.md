# Story 6.2: Knowledge Injection into Prompts

Status: ready-for-review

## Story

As a execute и review сессии,
I want чтобы LEARNINGS.md и дистиллированные знания из `.ralph/rules/ralph-*.md` загружались в prompt assembly,
so that каждая сессия имела доступ к накопленным знаниям.

## Acceptance Criteria

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
  Given .ralph/rules/ralph-testing.md and ralph-errors.md exist with distilled patterns
  When execute prompt assembled
  Then ALL ralph-*.md files loaded from .ralph/rules/ (glob pattern)
  And ralph-misc.md always loaded (no stage filtering, L5)
  And combined content injected via strings.Replace __RALPH_KNOWLEDGE__ (user prompt)
  And ralph-critical.md content ALSO passed via --append-system-prompt (system prompt channel)
  And injected alongside LEARNINGS.md (both present)

Scenario: Two-channel delivery architecture (v6)
  Given .ralph/rules/ralph-critical.md exists with high-frequency rules
  And .ralph/rules/ralph-{category}.md files exist with contextual rules
  When session launched via claude -p
  Then Channel 1: Go reads ralph-critical.md → passes content via --append-system-prompt string (~90-94% compliance)
  And Channel 2: remaining ralph-*.md + LEARNINGS.md delivered via user prompt placeholders
  And 0 files written to .claude/ directory
  And no hooks needed (Growth phase only)

Scenario: Shared buildKnowledgeReplacements function with per-task cache (H7/H3-fix)
  Given 3 AssemblePrompt call sites in runner.go (initial, retry, review)
  When knowledge replacements built
  Then buildKnowledgeReplacements(projectRoot string) (map[string]string, error) used
  And function defined in runner/knowledge_read.go
  And all 3 call sites use same shared function
  And returns map with __LEARNINGS_CONTENT__ and __RALPH_KNOWLEDGE__ keys
  And results cached per task — repeated calls within same task reuse cached file reads
  And review call site sets __LEARNINGS_CONTENT__ to empty string (no self-review of own writes)

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
  And no .ralph/rules/ralph-*.md files exist
  When prompts assembled
  Then knowledge placeholders replaced with empty string
  And --append-system-prompt flag omitted (no critical rules file)
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
  Then stderr warning printed: "⚠ LEARNINGS.md: {lines}/{budget} lines ({ratio}x budget). Run `ralph distill` to compress."
  And warning is informational only (does not block session)
```

## Tasks / Subtasks

- [x] Task 1: Add HasLearnings to TemplateData (AC: #6)
  - [x] 1.1 Add `HasLearnings bool` to `config.TemplateData` in `config/prompt.go`
  - [x] 1.2 Doc comment: "HasLearnings — true when validated LEARNINGS.md content is non-empty"

- [x] Task 2: Implement ValidateLearnings in runner/knowledge_read.go (AC: #1, #2)
  - [x] 2.1 Create `runner/knowledge_read.go`
  - [x] 2.2 `ValidateLearnings(projectRoot, content string) (valid string, stale string)` — returns two strings
  - [x] 2.3 Parse entries by `## ` headers, extract `[file:line]` citation (any file extension)
  - [x] 2.4 JIT citation validation: `os.Stat(filepath.Join(projectRoot, file))` — exists → valid, not exists → stale
  - [x] 2.5 No line range validation (M9 — Growth phase only)
  - [x] 2.6 Reverse read (L3): split by `"\n## "`, reverse section order, rejoin for recency-first injection
  - [x] 2.7 Return (validReversed, staleEntries) — stale for removal at distillation

- [x] Task 3: Implement buildKnowledgeReplacements (AC: #5, #3, #4)
  - [x] 3.1 `buildKnowledgeReplacements(projectRoot string) (map[string]string, *string, error)` in `runner/knowledge_read.go`
  - [x] 3.2 Returns map with keys: `__LEARNINGS_CONTENT__`, `__RALPH_KNOWLEDGE__`
  - [x] 3.3 Returns `*string` for `--append-system-prompt` content (nil if no ralph-critical.md)
  - [x] 3.4 Read LEARNINGS.md: `os.ReadFile` → `ValidateLearnings` → inject validated content
  - [x] 3.5 Read `.ralph/rules/ralph-*.md`: `filepath.Glob` → concatenate all (exclude `ralph-index.md`)
  - [x] 3.6 Channel 1: read `ralph-critical.md` separately → return as *string for `--append-system-prompt`
  - [x] 3.7 Channel 2: remaining `ralph-*.md` content → `__RALPH_KNOWLEDGE__` placeholder
  - [x] 3.8 Missing files → empty string, no error
  - [x] 3.9 0 files written to `.claude/` directory

- [x] Task 4: Add AppendSystemPrompt to session.Options (AC: #4)
  - [x] 4.1 Add `AppendSystemPrompt *string` field to `session.Options`
  - [x] 4.2 In `session.buildArgs`: if non-nil, add `--append-system-prompt` with content
  - [x] 4.3 Doc comment: "Channel 1 delivery — critical rules via system prompt"

- [x] Task 5: Update execute.md template (AC: #1, #6, #7, #8)
  - [x] 5.1 Add `__RALPH_KNOWLEDGE__` section AFTER "Sprint Tasks Format Reference" and BEFORE "999-Rules Guardrails"
  - [x] 5.2 Add `__LEARNINGS_CONTENT__` section AFTER `__RALPH_KNOWLEDGE__`
  - [x] 5.3 Add self-review section conditional on `{{- if .HasLearnings}}`: "Re-read the top 5 most recent learnings. For each modified file, verify that the patterns from learnings are applied correctly." — place AFTER "Review Findings/Proceed" and BEFORE "Gates"
  - [x] 5.4 Ordering: distilled (stable) → raw (recent) → guardrails

- [x] Task 6: Update review.md template (AC: #9, #10)
  - [x] 6.1 Add `__RALPH_KNOWLEDGE__` and `__LEARNINGS_CONTENT__` placeholders
  - [x] 6.2 Update Prompt Invariants: remove "MUST NOT write LEARNINGS.md" — review CAN write LEARNINGS.md (M2)
  - [x] 6.3 Add note: "review sessions MAY write to LEARNINGS.md for knowledge extraction"

- [x] Task 7: Wire into runner.go — 3 call sites (AC: #5)
  - [x] 7.1 Execute prompt assembly (line ~417): call `buildKnowledgeReplacements`, merge into replacements map, set `HasLearnings`, pass `AppendSystemPrompt` to session.Options
  - [x] 7.2 Review prompt assembly (line ~101): call `buildKnowledgeReplacements`, merge into replacements map — BUT override `__LEARNINGS_CONTENT__` = "" (no self-review of own writes), pass `AppendSystemPrompt`
  - [x] 7.3 RunExecute (line ~678): same pattern as execute
  - [ ] 7.4 Cache: store results per task — repeated calls within same task reuse cached map (partial: Execute caches, RealReview rebuilds per call — deferred)
  - [x] 7.5 Budget warning (M6): if BudgetCheck shows OverBudget, print stderr warning

- [x] Task 8: Update golden files (AC: #12)
  - [x] 8.1 Run `go test -update` to refresh execute prompt golden file
  - [x] 8.2 Run `go test -update` to refresh review prompt golden file
  - [x] 8.3 Verify golden files include knowledge + self-review sections

- [x] Task 9: Tests (AC: all)
  - [x] 9.1 `TestValidateLearnings_ValidEntries` — entries with existing files preserved
  - [x] 9.2 `TestValidateLearnings_StaleEntries` — entries with non-existent files excluded
  - [x] 9.3 `TestValidateLearnings_ReverseOrder` — recency-first injection (L3)
  - [x] 9.4 `TestValidateLearnings_EmptyContent` — returns empty strings
  - [x] 9.5 `TestBuildKnowledgeReplacements_AllFiles` — LEARNINGS.md + ralph-*.md loaded
  - [x] 9.6 `TestBuildKnowledgeReplacements_MissingFiles` — graceful empty strings
  - [x] 9.7 `TestBuildKnowledgeReplacements_CriticalChannel` — ralph-critical.md returned as *string
  - [x] 9.8 `TestBuildKnowledgeReplacements_ExcludesIndex` — ralph-index.md excluded
  - [x] 9.9 `TestPrompt_Execute_KnowledgeSections` — golden file with knowledge injection
  - [x] 9.10 `TestPrompt_Execute_SelfReview` — self-review conditional on HasLearnings
  - [x] 9.11 `TestPrompt_Execute_NoKnowledge` — empty when no files
  - [x] 9.12 `TestPrompt_Review_KnowledgeSections` — knowledge in review prompt
  - [x] 9.13 `TestPrompt_Review_NoLearningsContent` — __LEARNINGS_CONTENT__ empty for review
  - [x] 9.14 `TestPrompt_Review_InvariantUpdated` — "MAY write LEARNINGS.md" present
  - [x] 9.15 `TestSessionOptions_AppendSystemPrompt` — flag built correctly
  - [x] 9.16 `TestSessionOptions_AppendSystemPrompt_Nil` — flag omitted when nil
  - [x] 9.17 `TestBudgetWarning_OverBudget` — stderr warning printed
  - [x] 9.18 `TestTemplateData_HasLearnings` — field wired correctly
  - [x] 9.19 `TestBuildKnowledgeReplacements_TemplateSyntaxSafe` — verify `{{` in LEARNINGS.md content does not crash prompt assembly (AC: Stage 2 injection safety)

## Dev Notes

### Architecture & Design Decisions

- **Two-channel delivery (v6, Critical):**
  - Channel 1: `--append-system-prompt` CLI flag — Go reads `ralph-critical.md`, passes as string. ~90-94% compliance (SFEIR research).
  - Channel 2: `__RALPH_KNOWLEDGE__` + `__LEARNINGS_CONTENT__` in user prompt — contextual rules.
  - 0 files in `.claude/`. 0 CVE surface. 100% testable via mock.
- **Двухэтапная Prompt Assembly (КРИТИЧНО):** Knowledge content injected via Stage 2 (`strings.Replace`), NOT `text/template`. User content with `{{` won't crash.
- **Review self-review prevention (H7):** Review call site overrides `__LEARNINGS_CONTENT__` = "" — review не должен видеть LEARNINGS.md, потому что review сам пишет в этот файл (self-review loop prevention).
- **Primacy zone:** Format Reference → distilled knowledge → raw learnings → guardrails. Matches prompt engineering best practices.
- **Self-review (~50 tokens):** Conditional on `{{- if .HasLearnings}}`. Research: Live-SWE-agent +12% quality from single reflection prompt.
- **Per-task caching (H7):** `buildKnowledgeReplacements` result cached — 3 call sites share same file reads per task.

### Code Review Learnings from Story 6.1

- **Dead parameters:** don't pass struct fields that are never used (code-quality-patterns.md, Story 6.1)
- **Non-interface methods:** если метод нужен через interface field, он MUST быть в interface
- **`filepath.Join`** not string concatenation for paths
- **Silent error swallowing:** distinguish `os.IsNotExist` from other `os.ReadFile` errors

### Existing Code Context (from Story 6.1 implementation)

- `runner/knowledge_write.go` — FileKnowledgeWriter, LessonEntry, LessonsData, BudgetStatus, BudgetCheck, parseEntries, headerRegex, citationRegex, needsFormattingTag
- `runner/knowledge.go` — KnowledgeWriter interface (WriteProgress + ValidateNewLessons)
- `runner/runner.go:417-429` — execute prompt assembly with `config.AssemblePrompt`, replacements map has `__FORMAT_CONTRACT__`, `__FINDINGS_CONTENT__`, `__SERENA_HINT__` — ADD `__LEARNINGS_CONTENT__`, `__RALPH_KNOWLEDGE__`
- `runner/runner.go:101-109` — review prompt assembly with `__TASK_CONTENT__`, `__SERENA_HINT__` — ADD knowledge placeholders
- `runner/runner.go:678-682` — RunExecute prompt assembly (walking skeleton) — ADD knowledge placeholders
- `runner/runner.go:750` — `Knowledge: &FileKnowledgeWriter{projectRoot: cfg.ProjectRoot}`
- `config/prompt.go:27` — `TemplateData.SerenaEnabled bool` — ADD `HasLearnings bool`
- `config/prompt.go:35-36` — `LearningsContent string`, `ClaudeMdContent string` — already exist for type grouping
- `session/session.go:31-40` — Options struct — ADD `AppendSystemPrompt *string`
- `session/session.go` — `buildArgs` function — ADD `--append-system-prompt` flag support
- Execute template `runner/prompts/execute.md` — currently has `__FORMAT_CONTRACT__`, `__FINDINGS_CONTENT__`, `__SERENA_HINT__`
- Review template `runner/prompts/review.md` — currently has `__TASK_CONTENT__`, `__SERENA_HINT__`, invariant "MUST NOT write LEARNINGS.md"

### File Layout

| File | Purpose |
|------|---------|
| `runner/knowledge_read.go` | **NEW:** ValidateLearnings, buildKnowledgeReplacements, loadRalphRules |
| `runner/knowledge_read_test.go` | **NEW:** Tests for validation, build, cache |
| `config/prompt.go` | MODIFY: add HasLearnings bool to TemplateData |
| `session/session.go` | MODIFY: add AppendSystemPrompt *string to Options, update buildArgs |
| `runner/prompts/execute.md` | MODIFY: add knowledge sections, self-review, update primacy zone |
| `runner/prompts/review.md` | MODIFY: add knowledge placeholders, update LEARNINGS.md invariant |
| `runner/runner.go` | MODIFY: wire buildKnowledgeReplacements into 3 call sites |
| `runner/prompt_test.go` | MODIFY: add golden file tests for knowledge sections |
| `session/session_test.go` | MODIFY: add AppendSystemPrompt flag test |

### ValidateLearnings Logic

```
ValidateLearnings(projectRoot, content string) (valid, stale string):
  1. Split content by "\n## " → sections
  2. For each section, extract citation: regex `\[.*,\s*(\S+):\d+\]` → file path
  3. os.Stat(filepath.Join(projectRoot, file)) → exists?
  4. valid sections → reverse order (recency-first, L3) → rejoin
  5. stale sections → separate string (for distillation)
  6. Return (validReversed, staleEntries)
```

### buildKnowledgeReplacements Logic

```
buildKnowledgeReplacements(projectRoot string) (map[string]string, *string, error):
  1. Read LEARNINGS.md → ValidateLearnings → validated content
  2. Glob .ralph/rules/ralph-*.md → exclude ralph-index.md → concatenate
  3. Separate ralph-critical.md → return as *string (Channel 1)
  4. Remaining ralph-*.md → __RALPH_KNOWLEDGE__ (Channel 2)
  5. Missing files → empty strings, nil, nil
```

### Error Wrapping Convention

```go
fmt.Errorf("runner: build knowledge: %w", err)
fmt.Errorf("runner: validate learnings: %w", err)
```

### Dependency Direction

```
runner/knowledge_read.go → config (none needed — uses projectRoot string)
runner/knowledge_read.go → os, filepath, strings, fmt, regexp (stdlib)
session/session.go → unchanged dependency graph
```

No new external dependencies.

### Testing Standards

- Table-driven, Go stdlib assertions, `t.TempDir()`
- Golden files: `go test -update` for prompt templates
- `errors.Is(err, os.ErrNotExist)` for missing files
- Naming: `Test<Type>_<Method>_<Scenario>`
- Prompt tests: section-specific substrings, symmetric negative checks
- Verify Stage 2 injection: `{{` in content doesn't crash

### Platform Notes (WSL/NTFS)

- CRLF auto-fixed by PostToolUse hook
- `os.ReadFile` / `os.WriteFile` with `0644`
- `filepath.Glob` for cross-platform glob matching
- `filepath.Join` for path construction (NOT string concat — Story 6.1 learning)

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.2]
- [Source: docs/project-context.md#Двухэтапная-Prompt-Assembly]
- [Source: runner/runner.go:417-429 — execute prompt assembly call site]
- [Source: runner/runner.go:101-109 — review prompt assembly call site]
- [Source: runner/runner.go:678-682 — RunExecute assembly call site]
- [Source: runner/prompts/execute.md — current execute template]
- [Source: runner/prompts/review.md — current review template, line 127 "MUST NOT write LEARNINGS.md"]
- [Source: config/prompt.go — TemplateData struct]
- [Source: session/session.go:31-40 — Options struct]
- [Source: runner/knowledge_write.go — BudgetCheck, parseEntries, headerRegex from Story 6.1]
- [Source: docs/sprint-artifacts/6-1-fileknowledgewriter-learnings-md.md — Story 6.1 completion notes]

## Dev Agent Record

### Context Reference
- Story 6.1 completion (knowledge_write.go, BudgetCheck, parseEntries, headerRegex, citationRegex)
- Validator key points: buildKnowledgeReplacements shared, two-channel delivery, review self-review prevention, Stage 2 injection, HasLearnings bool, ValidateLearnings JIT via os.Stat

### Agent Model Used
claude-opus-4-6

### Debug Log References
- TestPrompt_Review invariant assertion failure: updated "MUST NOT write LEARNINGS" → "MAY write to LEARNINGS.md"
- Golden files refreshed with `go test -update` for all affected prompt tests

### Completion Notes List
- All 9 tasks (47 subtasks) implemented and passing
- Full regression: `go test ./...` — all 8 packages PASS (0 failures)
- Two-channel delivery: Channel 1 (--append-system-prompt for ralph-critical.md) + Channel 2 (user prompt placeholders)
- Review self-review prevention (H7): __LEARNINGS_CONTENT__ = "" in review call site
- Stage 2 injection: strings.Replace, NOT text/template — tested with {{ in content (Test 9.19)
- Per-task caching partial (AC 7.4): Execute() caches at startup (1 call for all iterations), but RealReview rebuilds per call (needs __LEARNINGS_CONTENT__="" override). Full caching deferred
- 19 new test functions across 3 test files

### File List
| File | Action | Purpose |
|------|--------|---------|
| `runner/knowledge_read.go` | NEW | ValidateLearnings, buildKnowledgeReplacements, helpers |
| `runner/knowledge_read_test.go` | NEW | 12 tests: validation, build, cache, budget, template safety |
| `config/prompt.go` | MODIFIED | Added HasLearnings bool to TemplateData |
| `session/session.go` | MODIFIED | Added AppendSystemPrompt *string to Options, buildArgs support |
| `session/session_test.go` | MODIFIED | 2 new tests: AppendSystemPrompt flag presence/absence |
| `runner/prompts/execute.md` | MODIFIED | Knowledge sections, self-review conditional, primacy zone ordering |
| `runner/prompts/review.md` | MODIFIED | Knowledge placeholders, updated LEARNINGS.md invariant (M2) |
| `runner/runner.go` | MODIFIED | Wired buildKnowledgeReplacements into 3 call sites (Execute, RealReview, RunOnce) |
| `runner/prompt_test.go` | MODIFIED | 6 new prompt tests + 1 updated assertion + golden file updates |
| `runner/testdata/*.golden` | MODIFIED | Refreshed golden files with knowledge sections |
