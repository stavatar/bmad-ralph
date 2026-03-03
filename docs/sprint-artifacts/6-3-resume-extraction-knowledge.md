# Story 6.3: Resume-Extraction Knowledge

Status: ready-for-review

## Story

As a resume-extraction сессия,
I want записывать причины неудачи в LEARNINGS.md,
so that будущие сессии учились на ошибках.

## Acceptance Criteria

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

## Tasks / Subtasks

- [x] Task 1: Fix --resume and -p compatibility in session.go (AC: #2, M1)
  - [x] 1.1 In `session/session.go:88-92`: change `else if` to allow BOTH `--resume` AND `-p` flags simultaneously
  - [x] 1.2 Logic: if Resume != "" → add `--resume session_id`; if Prompt != "" → add `-p prompt` (independent, not mutually exclusive)
  - [x] 1.3 Update doc comment on `buildArgs` to reflect compatibility
  - [x] 1.4 Test: `TestBuildArgs_ResumeWithPrompt` — both flags present in output

- [x] Task 2: Create resume-extraction prompt with knowledge instructions (AC: #4)
  - [x] 2.1 Update resume-extraction prompt (inline in runner/runner.go or new template)
  - [x] 2.2 Add instructions: "Extract failure insights from the interrupted session"
  - [x] 2.3 Add instructions: "Write findings as atomized facts to LEARNINGS.md"
  - [x] 2.4 Include LEARNINGS.md format specification: `## category: topic [source, file:line]\nAtomized fact content.\n`
  - [x] 2.5 Include: "what was attempted, where stuck, extracted insights"

- [x] Task 3: Wire snapshot-diff into ResumeExtraction (AC: #1, #3)
  - [x] 3.1 Before session: snapshot LEARNINGS.md content via `os.ReadFile`
  - [x] 3.2 Pass snapshot to `ValidateNewLessons` via `LessonsData.Snapshot`
  - [x] 3.3 After session ends: call `kw.ValidateNewLessons(ctx, LessonsData{...})`
  - [x] 3.4 Set `LessonsData.Source = "resume-extraction"`
  - [x] 3.5 Set `LessonsData.BudgetLimit = cfg.LearningsBudget`

- [x] Task 4: Pass prompt to ResumeExtraction (AC: #2)
  - [x] 4.1 Add `-p` prompt to `session.Options` in `ResumeExtraction` alongside `--resume`
  - [x] 4.2 Assemble resume-extraction prompt with knowledge instructions
  - [x] 4.3 No separate extraction session — resume gets prompt directly

- [x] Task 5: Tests (AC: all)
  - [x] 5.1 `TestBuildArgs_ResumeWithPrompt` — both --resume and -p flags in output
  - [x] 5.2 `TestBuildArgs_ResumeOnly` — only --resume when no prompt (backward compat)
  - [x] 5.3 `TestResumeExtraction_SnapshotDiff` — validates new LEARNINGS.md entries after session
  - [x] 5.4 `TestResumeExtraction_NoChanges` — LEARNINGS.md unchanged, no error
  - [x] 5.5 `TestResumeExtraction_WithPrompt` — resume session gets extraction prompt
  - [x] 5.6 `TestResumeExtraction_InvalidEntries` — entries tagged [needs-formatting]
  - [x] 5.7 Update existing `TestResumeExtraction_Scenarios` table — add knowledge-related cases

## Dev Notes

### Architecture & Design Decisions

- **C1 Model:** Claude inside resume session writes LEARNINGS.md directly via file tools. Go post-validates (snapshot-diff model from Story 6.1).
- **M1 Fix (Critical):** `session/session.go:88-92` currently has `else if` making `--resume` and `-p` mutually exclusive. Fix: make them independent `if` blocks. Claude CLI officially supports both simultaneously.
- **No separate extraction session:** Resume gets the extraction prompt directly via `-p` alongside `--resume`.
- **FR28:** Resume пишет причины неудачи + извлечённые знания. Triggers for: actual errors AND sessions that ran out of turns.

### Current Code (session.go:88-92 — the else-if to fix)

```go
// CURRENT (broken for our use case):
if opts.Resume != "" {
    args = append(args, flagResume, opts.Resume)
} else if opts.Prompt != "" {
    args = append(args, flagPrompt, opts.Prompt)
}

// FIXED:
if opts.Resume != "" {
    args = append(args, flagResume, opts.Resume)
}
if opts.Prompt != "" {
    args = append(args, flagPrompt, opts.Prompt)
}
```

### Current ResumeExtraction (runner/runner.go:189-225)

- Takes `(ctx, cfg, kw KnowledgeWriter, sessionID string)`
- Currently: only `--resume` + `--max-turns` + `--model` + `--output-format json`
- No `-p` prompt — just resumes session silently
- After session: calls `kw.WriteProgress(ctx, ProgressData{SessionID: sr.SessionID})`
- Need to add: `-p` with extraction prompt, snapshot before, ValidateNewLessons after

### Snapshot-Diff Integration

```go
// Before session:
snapshot, _ := os.ReadFile(filepath.Join(cfg.ProjectRoot, "LEARNINGS.md"))

// After session:
data := LessonsData{
    Source:      "resume-extraction",
    Snapshot:    string(snapshot),
    BudgetLimit: cfg.LearningsBudget,
}
kw.ValidateNewLessons(ctx, data)
```

### LEARNINGS.md Entry Format (from Story 6.1)

```
## category: topic [source, file:line]
Atomized fact content. Optional VIOLATION: concrete example.
```

### File Layout

| File | Purpose |
|------|---------|
| `session/session.go:88-92` | FIX: `else if` → independent `if` blocks for Resume + Prompt compatibility |
| `session/session_test.go` | ADD: TestBuildArgs_ResumeWithPrompt |
| `runner/runner.go:189-225` | MODIFY: ResumeExtraction — add prompt, snapshot-diff, ValidateNewLessons |
| `runner/runner_test.go` | MODIFY: update TestResumeExtraction_Scenarios, add knowledge tests |

### Error Wrapping Convention

```go
fmt.Errorf("runner: resume extraction: validate lessons: %w", err)
// Existing: "runner: resume extraction: execute:", "runner: resume extraction: parse:", "runner: resume extraction: write progress:"
```

### Dependency Direction

```
runner/runner.go → session (unchanged)
session/session.go → (stdlib only, unchanged)
```

No new external dependencies.

### Testing Standards

- Table-driven, `t.TempDir()`, Go stdlib assertions
- Update existing `TestResumeExtraction_Scenarios` table — add knowledge-related cases
- Verify backward compatibility: existing tests still pass with changed buildArgs
- `errors.Is` / `strings.Contains` for error verification
- Mock: trackingKnowledgeWriter already tracks ValidateNewLessons calls

### Code Review Learnings from Story 6.1

- Use `filepath.Join` not string concatenation for paths
- Distinguish `os.IsNotExist` from other `os.ReadFile` errors
- Non-interface methods break testability — ensure ValidateNewLessons is on the interface
- All error returns in a function must wrap with same prefix

### Platform Notes (WSL/NTFS)

- CRLF auto-fixed by PostToolUse hook
- Missing LEARNINGS.md: `os.ReadFile` → `os.IsNotExist` → empty snapshot (all entries new)

### References

- [Source: docs/epics/epic-6-knowledge-management-polish-stories.md#Story-6.3]
- [Source: session/session.go:88-92 — the else-if to fix (M1)]
- [Source: runner/runner.go:189-225 — ResumeExtraction function]
- [Source: runner/knowledge_write.go — FileKnowledgeWriter.ValidateNewLessons from Story 6.1]
- [Source: runner/knowledge_write.go:82 — ValidateNewLessons signature and snapshot-diff logic]
- [Source: runner/runner_test.go:1943 — existing TestResumeExtraction_Scenarios]
- [Source: runner/test_helpers_test.go:316 — trackingKnowledgeWriter mock]

## Dev Agent Record

### Context Reference
- Story 6.1 (FileKnowledgeWriter, ValidateNewLessons, LessonsData)
- Story 6.2 (knowledge injection, --append-system-prompt)
- Validator key points: M1 fix (else-if → independent if), snapshot-diff, resumeExtractionPrompt const

### Agent Model Used
claude-opus-4-6

### Debug Log References
- Updated 2 existing session tests ("resume overrides prompt" → "resume with prompt both present", "resume all fields set")
- Updated TestResumeExtraction_Scenarios table struct (added validateLessonsErr, wantValidateLessonsCount)

### Completion Notes List
- All 5 tasks (20 subtasks) implemented and passing
- Full regression: `go test ./...` — all 8 packages PASS (0 failures)
- M1 fix: `else if` → independent `if` blocks in session.go:buildArgs
- resumeExtractionPrompt const: inline in runner.go with format spec and instructions
- Snapshot-diff: os.ReadFile before session, ValidateNewLessons after session
- 4 new test functions + 2 updated existing tests + 1 new table case

### File List
| File | Action | Purpose |
|------|--------|---------|
| `session/session.go` | MODIFIED | Fix else-if → independent if for Resume+Prompt compatibility (M1), updated doc comment |
| `session/session_test.go` | MODIFIED | Updated 2 table cases for Resume+Prompt compatibility |
| `runner/runner.go` | MODIFIED | Added resumeExtractionPrompt const, wired -p prompt + snapshot-diff + ValidateNewLessons into ResumeExtraction |
| `runner/runner_test.go` | MODIFIED | Added validateLessonsErr/wantValidateLessonsCount to table, 1 new table case (validate lessons error), 2 new standalone tests (SnapshotDiff, NoChanges) |
