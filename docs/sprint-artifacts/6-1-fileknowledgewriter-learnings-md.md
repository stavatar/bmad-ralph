# Story 6.1: FileKnowledgeWriter — LEARNINGS.md Post-Validation

Status: Ready for Review

## Story

As a система,
I want Go-side post-validation для LEARNINGS.md записей которые Claude пишет напрямую через file tools, с проверкой бюджета и тегированием невалидных entries,
so that знания накапливались между сессиями с гарантией качества.

## Acceptance Criteria

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

## Tasks / Subtasks

- [x] Task 1: Define data structs and extend KnowledgeWriter interface (AC: #1)
  - [x] 1.1 Define `LessonEntry` struct in `runner/knowledge_write.go`: `Category, Topic, Content, Citation string`
  - [x] 1.2 Define `LessonsData` struct: `Source string, Entries []LessonEntry`
  - [x] 1.3 Define `BudgetStatus` struct: `Lines, Limit int`, `NearLimit, OverBudget bool`
  - [x] 1.4 Add `ValidateNewLessons(ctx context.Context, data LessonsData) error` to `KnowledgeWriter` interface in `runner/knowledge.go`
  - [x] 1.5 Update `NoOpKnowledgeWriter` with no-op `ValidateNewLessons` returning nil
  - [x] 1.6 Verify compile-time interface check: `var _ KnowledgeWriter = (*NoOpKnowledgeWriter)(nil)`
  - [x] 1.7 Define named constants (L1): `MaxNewEntriesPerValidation = 5`, `MinEntryContentLength = 20`, `SoftDistillationThreshold = 150`

- [x] Task 2: Implement FileKnowledgeWriter struct (AC: #9)
  - [x] 2.1 Create `FileKnowledgeWriter` struct in `runner/knowledge_write.go`: `projectRoot string`
  - [x] 2.2 Implement `WriteProgress` on `FileKnowledgeWriter` — same behavior as current (write to sprint-tasks context)
  - [x] 2.3 Add compile-time check: `var _ KnowledgeWriter = (*FileKnowledgeWriter)(nil)`
  - [x] 2.4 Doc comment: "Sequential architecture — no concurrent access" (AC: #10 L2)

- [x] Task 3: Implement 6 quality gates (AC: #3)
  - [x] 3.1 G1 Format check: entry has `## category: topic [citation]` header — regex pattern in package scope
  - [x] 3.2 G2 Citation present: `[source, file:line]` parsed — regex for `\[.*,\s*\S+:\d+\]`
  - [x] 3.3 G3 Semantic dedup: header normalization via `strings.ToLower + strings.TrimSpace`, prefix match (AC: #4)
  - [x] 3.4 G4 Budget check: total lines vs `config.LearningsBudget` hard limit
  - [x] 3.5 G5 Entry cap: `len(entries) <= MaxNewEntriesPerValidation`
  - [x] 3.6 G6 Min content: `len(strings.TrimSpace(entry.Content)) >= MinEntryContentLength`

- [x] Task 4: Implement ValidateNewLessons with snapshot-diff model (AC: #2, #3)
  - [x] 4.1 Parse LEARNINGS.md into entries: `strings.Split(content, "\n## ")` → `[]LessonEntry`
  - [x] 4.2 Diff new vs snapshot: identify entries added since snapshot
  - [x] 4.3 Line-count guard: if current lines < snapshot lines, log warning and full revalidation
  - [x] 4.4 Run 6 quality gates on each new entry
  - [x] 4.5 Tag invalid entries with `[needs-formatting]` in-place (write back to file)
  - [x] 4.6 Log warning for each tagged entry: "Entry saved with [needs-formatting] — will be fixed at distillation"
  - [x] 4.7 Support optional `VIOLATION:` marker in content body

- [x] Task 5: Implement semantic dedup merge (AC: #4)
  - [x] 5.1 Normalize headers: `strings.ToLower(strings.TrimSpace(header))` before comparison
  - [x] 5.2 Detect same `category: topic` prefix between existing and new entries
  - [x] 5.3 Merge new facts under existing heading, preserve both citations
  - [x] 5.4 Write merged content back to LEARNINGS.md

- [x] Task 6: Implement BudgetCheck free function (AC: #6, #7, #8)
  - [x] 6.1 `BudgetCheck(ctx context.Context, learningsPath string, limit int) BudgetStatus`
  - [x] 6.2 Count lines via `strings.Count(content, "\n")` (Architecture pattern)
  - [x] 6.3 NearLimit: `lines >= SoftDistillationThreshold` (150)
  - [x] 6.4 OverBudget: `lines >= limit`
  - [x] 6.5 Missing file: return `BudgetStatus{Lines: 0}`, no error

- [x] Task 7: Wire FileKnowledgeWriter into runner (AC: #9)
  - [x] 7.1 In `runner.Run` initialization, create `FileKnowledgeWriter{projectRoot: cfg.ProjectRoot}`
  - [x] 7.2 Set `r.Knowledge = &FileKnowledgeWriter{...}` instead of `&NoOpKnowledgeWriter{}`
  - [x] 7.3 Call snapshot before session: read LEARNINGS.md content and store
  - [x] 7.4 Call `ValidateNewLessons` after session ends (execute or review)

- [x] Task 8: Tests (AC: all)
  - [x] 8.1 `TestLessonEntry_ZeroValue` — verify zero-value struct
  - [x] 8.2 `TestLessonsData_ZeroValue` — verify zero-value struct
  - [x] 8.3 `TestBudgetStatus_ZeroValue` — verify zero-value struct
  - [x] 8.4 `TestNoOpKnowledgeWriter_ValidateNewLessons_ReturnsNil` — no-op returns nil
  - [x] 8.5 `TestFileKnowledgeWriter_ValidateNewLessons_QualityGates` — table-driven: each gate pass/fail
  - [x] 8.6 `TestFileKnowledgeWriter_ValidateNewLessons_TagsInvalid` — [needs-formatting] tag written to file
  - [x] 8.7 `TestFileKnowledgeWriter_ValidateNewLessons_SemanticDedup` — merge + citation preservation
  - [x] 8.8 `TestFileKnowledgeWriter_ValidateNewLessons_LineCountGuard` — rewrite detection warning
  - [x] 8.9 `TestBudgetCheck_Thresholds` — table-driven: NearLimit, OverBudget, normal, zero
  - [x] 8.10 `TestBudgetCheck_MissingFile` — returns zero status, no error
  - [x] 8.11 `TestFileKnowledgeWriter_ValidateNewLessons_EntryCap` — > MaxNewEntriesPerValidation tagged
  - [x] 8.12 `TestFileKnowledgeWriter_ValidateNewLessons_MinContent` — short entries tagged
  - [x] 8.13 `TestFileKnowledgeWriter_ValidateNewLessons_NewFile` — absent file = all entries new
  - [x] 8.14 `TestFileKnowledgeWriter_WriteProgress_Unchanged` — same as NoOp behavior

## Dev Notes

### Architecture & Design Decisions

- **C1 Model (Critical):** Claude пишет LEARNINGS.md напрямую через file tools. Go НЕ пишет контент — только читает, валидирует и тегирует `[needs-formatting]`. Snapshot-diff модель: Go делает snapshot перед сессией, diff после.
- **Sequential Architecture (L2):** Без mutex — single runner goroutine. Документировано как design decision. YAGNI.
- **BudgetCheck = free function:** НЕ метод интерфейса. Сохраняет 2-method interface contract (WriteProgress + ValidateNewLessons).
- **Named constants (L1):** `MaxNewEntriesPerValidation = 5`, `MinEntryContentLength = 20`, `SoftDistillationThreshold = 150` — в `runner/knowledge_write.go`.
- **Soft threshold 150:** триггер для auto-distillation (Story 6.5). OverBudget (>=200) = informational only, файл НЕ обрезается.
- **Append-only:** никогда не удалять контент из LEARNINGS.md. Invalid entries тегируются, не удаляются.

### Existing Scaffold (from Epics 1-5)

- `runner/knowledge.go` — `KnowledgeWriter` interface (1 method: `WriteProgress`), `NoOpKnowledgeWriter`, `ProgressData` struct
- `runner/knowledge_test.go` — 2 теста: `TestNoOpKnowledgeWriter_WriteProgress_ReturnsNil`, `TestNoOpKnowledgeWriter_WriteProgress_NoLearningsFile`
- `runner/test_helpers_test.go:316` — `trackingKnowledgeWriter` mock для test assertions
- `runner/runner.go:330` — `Runner.Knowledge KnowledgeWriter` field, initialized to `&NoOpKnowledgeWriter{}` at line 710
- `config.Config.LearningsBudget` = 200 (default in `config/defaults.yaml`)
- `config.TemplateData.LearningsContent` string — exists but not yet wired to actual file read (Story 6.2)

### File Layout

| File | Purpose |
|------|---------|
| `runner/knowledge.go` | KnowledgeWriter interface — ADD `ValidateNewLessons` method |
| `runner/knowledge_write.go` | **NEW:** FileKnowledgeWriter, LessonEntry, LessonsData, BudgetStatus, BudgetCheck, quality gates, snapshot-diff, semantic dedup |
| `runner/knowledge_write_test.go` | **NEW:** All tests for Task 8 |
| `runner/runner.go:710` | Change `&NoOpKnowledgeWriter{}` → `&FileKnowledgeWriter{projectRoot: cfg.ProjectRoot}` |
| `runner/test_helpers_test.go` | Update `trackingKnowledgeWriter` to implement new `ValidateNewLessons` method |

### Entry Format

```
## category: topic [source, file:line]
Atomized fact content. Optional VIOLATION: concrete example inline.
```

- Category: one of 7 canonical categories (testing, errors, architecture, naming, security, performance, misc) + extensible
- file extension is project-dependent (.go, .py, .js, .rs, etc.)
- Parsing: `strings.Split(content, "\n## ")` → iterate sections
- Header regex: `^## (\w[\w-]*): (.+?) \[(.+)\]$` — captures category, topic, citation

### Quality Gates Detail

| Gate | Check | Action on Fail |
|------|-------|---------------|
| G1 | `## category: topic [citation]` header format | Tag `[needs-formatting]` |
| G2 | Citation `[source, file:line]` parseable | Tag `[needs-formatting]` |
| G3 | No duplicate heading prefix (semantic dedup) | Merge under existing |
| G4 | Total lines < `config.LearningsBudget` | Tag `[needs-formatting]` |
| G5 | `len(newEntries) <= MaxNewEntriesPerValidation` (5) | Tag excess entries |
| G6 | Content body >= `MinEntryContentLength` chars (20) | Tag `[needs-formatting]` |

### Line Counting

- Pattern: `strings.Count(content, "\n")` — matches Architecture doc
- Empty file = 0 lines
- Missing file = `BudgetStatus{Lines: 0}`, no error

### Error Wrapping Convention

```go
fmt.Errorf("runner: validate lessons: %w", err)
fmt.Errorf("runner: budget check: %w", err)
fmt.Errorf("runner: snapshot diff: %w", err)
```

### Dependency Direction

```
runner/knowledge_write.go → config (for LearningsBudget)
runner/knowledge_write.go → context, os, strings, fmt, regexp (stdlib)
runner/knowledge.go → context (unchanged)
```

No new external dependencies. Строго top-down: runner → config.

### Testing Standards

- Table-driven по умолчанию, Go stdlib assertions (без testify)
- `t.TempDir()` для файловой изоляции
- `errors.As(err, &target)` для type assertions
- Error tests MUST verify message content via `strings.Contains`
- Naming: `Test<Type>_<Method>_<Scenario>` — "Type" = real Go type
- Coverage: runner >80%
- Golden files with `-update` flag if needed
- Zero-value tests для каждого нового struct

### Platform Notes (WSL/NTFS)

- CRLF auto-fixed by PostToolUse hook — no manual handling needed
- `os.ReadFile` / `os.WriteFile` with `0644`
- File paths: relative to `config.ProjectRoot`
- Missing file: `errors.Is(err, os.ErrNotExist)` → empty, not error

### Project Structure Notes

- `runner/knowledge_write.go` — новый файл, следует паттерну split из project-context.md (runner/ = loop + git + scan + knowledge)
- Runner split boundary: ~600-800 LOC в MVP. Knowledge write + read добавит ~200-300 LOC — в рамках лимита
- Naming: `FileKnowledgeWriter` в consumer package (`runner/`), не в отдельном `knowledge/` package

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.1]
- [Source: docs/project-context.md#Двухэтапная-Prompt-Assembly]
- [Source: docs/project-context.md#File-IO]
- [Source: docs/project-context.md#LEARNINGS.md-Budget]
- [Source: runner/knowledge.go — existing KnowledgeWriter interface]
- [Source: runner/runner.go:330 — Runner.Knowledge field]
- [Source: runner/runner.go:710 — NoOpKnowledgeWriter default initialization]
- [Source: config/defaults.yaml — learnings_budget: 200]
- [Source: config/config.go:32 — LearningsBudget field]
- [Source: runner/test_helpers_test.go:316 — trackingKnowledgeWriter mock]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6

### Debug Log References

- All 14 test functions pass (0 failures)
- Full regression: all 8 packages pass, 0 regressions
- Build: `go build ./...` clean

### Completion Notes List

- Task 1: Defined LessonEntry, LessonsData, BudgetStatus structs; extended KnowledgeWriter interface with ValidateNewLessons; updated NoOpKnowledgeWriter; added 3 named constants (L1)
- Task 2: FileKnowledgeWriter struct with projectRoot, NewFileKnowledgeWriter constructor, WriteProgress no-op, compile-time check, L2 doc comment
- Task 3: 6 quality gates (G1-G6) in validateEntry function: format regex, citation regex, dedup, budget, entry cap, min content
- Task 4: ValidateNewLessons reads file and validates entries; ValidateNewLessonsWithSnapshot implements snapshot-diff model with line-count guard, full revalidation on rewrite, [needs-formatting] tagging, stderr warnings, VIOLATION: marker support
- Task 5: Semantic dedup via categoryTopicPrefix normalization, mergeDedup detects same category:topic prefix, mergeEntryContent combines citations and content under existing heading
- Task 6: BudgetCheck free function returns BudgetStatus with NearLimit (>=150) and OverBudget (>=limit) thresholds; missing file returns zero status
- Task 7: Wired FileKnowledgeWriter into runner.Run replacing NoOpKnowledgeWriter; updated Knowledge field doc comment
- Task 8: All 14 tests implemented and passing (3 zero-value, 1 NoOp, 5 quality gates table-driven, 1 tag verification, 1 semantic dedup, 1 line-count guard, 6 budget thresholds table-driven, 1 missing file, 1 entry cap, 1 min content, 1 new file, 1 WriteProgress unchanged)
- Validator fix: updated stale comment in knowledge.go:13 "WriteLessons" → "ValidateNewLessons"

### File List

- `runner/knowledge.go` — modified: added ValidateNewLessons to KnowledgeWriter interface, NoOpKnowledgeWriter implementation, fixed stale comment
- `runner/knowledge_write.go` — **new**: FileKnowledgeWriter, LessonEntry, LessonsData, BudgetStatus structs, quality gates, snapshot-diff validation, semantic dedup, BudgetCheck
- `runner/knowledge_write_test.go` — **new**: 14 test functions covering all ACs
- `runner/runner.go` — modified: wired FileKnowledgeWriter in Run(), updated Knowledge field doc comment
- `runner/test_helpers_test.go` — modified: added ValidateNewLessons to trackingKnowledgeWriter mock
- `docs/sprint-artifacts/sprint-status.yaml` — modified: 6-1 status ready-for-dev → in-progress → review

## Change Log

- 2026-03-02: Implemented Story 6.1 — FileKnowledgeWriter with LEARNINGS.md post-validation, 6 quality gates, snapshot-diff model, semantic dedup, BudgetCheck, 14 tests
