# Story 9.6: Scope Creep Protection Prompts

Status: done

## Story

As a разработчик,
I want чтобы Claude не реализовывал соседние задачи из sprint-tasks.md, а review agent обнаруживал scope creep,
so that каждый execute цикл фокусируется на одной задаче.

## Acceptance Criteria

1. **SCOPE BOUNDARY блок в execute.md (FR79):**
   - `runner/prompts/execute.md` contains section "## SCOPE BOUNDARY (MANDATORY)"
   - Contains "Реализуй ТОЛЬКО текущую задачу: __TASK__"
   - Contains "НЕ реализуй другие задачи из sprint-tasks.md"
   - Contains instruction проверить перед коммитом
   - Contains instruction откатить через git checkout

2. **SCOPE BOUNDARY uses __TASK__ placeholder (FR79):**
   - execute.md template uses `__TASK__` placeholder (already exists in execute.md)
   - `buildTemplateData()` replaces `__TASK__` with actual task text
   - Scope boundary references the specific current task

3. **Scope compliance в implementation agent (FR80):**
   - `runner/prompts/agents/implementation.md` contains scope compliance check instruction
   - Contains "Все изменения в diff относятся к AC текущей задачи"
   - Contains "Scope creep" as HIGH severity finding format
   - Finding format: "Scope creep: изменения в <файл> реализуют задачу '<другая>', а не текущую"

4. **Scope check doesn't block unrelated agents (FR80):**
   - Other review agents (quality, simplification, design-principles, test-coverage) do NOT contain scope creep check instructions
   - Only implementation agent performs scope validation

5. **Template test coverage:**
   - Execute template rendered with task text contains "SCOPE BOUNDARY" section
   - Output contains the task text in scope boundary context

## Tasks / Subtasks

- [x] Task 1: Add SCOPE BOUNDARY section to `runner/prompts/execute.md` (AC: #1, #2)
  - [x] Add section after "Commit Rules" and before "Session Completion"
  - [x] Use __TASK_CONTENT__ placeholder (corrected from story's __TASK__)
  - [x] Include prohibition, check instruction, and rollback instruction
- [x] Task 2: Add scope compliance check to `runner/prompts/agents/implementation.md` (AC: #3)
  - [x] Add scope compliance point to Instructions section (point 5, renumbered 5→6, 6→7, 7→8)
  - [x] Include specific finding format for scope creep
  - [x] Specify HIGH severity for scope creep findings
- [x] Task 3: Verify other agents don't have scope check (AC: #4)
  - [x] Confirmed quality.md, simplification.md, design-principles.md, test-coverage.md have no scope creep instructions
- [x] Task 4: Write template tests in `runner/prompt_test.go` (AC: #5)
  - [x] Test execute prompt contains "SCOPE BOUNDARY" section
  - [x] Test execute prompt contains task text in scope boundary
  - [x] Test implementation agent contains scope compliance instructions
  - [x] Negative test: other agents don't contain scope creep check

## Dev Notes

### Architecture & Design

- **Prompt-only changes** — NO new Go code
- **Files:** `runner/prompts/execute.md`, `runner/prompts/agents/implementation.md`
- **Tests:** `runner/prompt_test.go`
- **No dependencies** on other Epic 9 stories

### Implementation Details

**SCOPE BOUNDARY section placement in execute.md:**
Insert after "## Commit Rules" section (line ~66) and before "## Session Completion" (line ~68):

```markdown
## SCOPE BOUNDARY (MANDATORY)

Реализуй ТОЛЬКО текущую задачу: __TASK__
НЕ реализуй другие задачи из sprint-tasks.md, даже если они кажутся связанными.
Если текущая задача зависит от другой — остановись и сообщи, не делай обе.

Перед коммитом проверь: каждый изменённый файл и каждое изменение
напрямую связаны с текущей задачей. Если обнаружишь изменения для другой
задачи — откати их через git checkout.
```

**Note:** `__TASK__` is NOT a new placeholder — it's already used in execute.md via Stage 2 replacement. The scope boundary section reuses it.

**Implementation agent addition:**
Add to Instructions section in implementation.md after point 4 (out-of-scope file modifications):

```markdown
5. Verify ALL changes in the diff relate to the current task's acceptance criteria.
   If changes implement a different task from sprint-tasks.md — this is a scope creep finding:
   - Severity: HIGH
   - Format: "Scope creep: изменения в <файл> реализуют задачу '<другая задача>', а не текущую"
```

### Existing Scaffold Context

- `runner/prompts/execute.md:59-66` — Commit Rules section (SCOPE BOUNDARY goes after)
- `runner/prompts/execute.md:68-73` — Session Completion section
- `runner/prompts/execute.md:75-79` — Task Scope section (already has basic file scope rule)
- `runner/prompts/agents/implementation.md:24-30` — current Instructions (4 points + finding format)
- `__TASK__` placeholder — already defined in `runner/prompt.go` buildTemplateData()

### Testing Standards

- Use discriminating assertions: "SCOPE BOUNDARY" not just "scope"
- Section-specific substring: check for "Реализуй ТОЛЬКО текущую задачу" not generic words
- Negative checks: verify other agents DON'T contain "Scope creep" instructions
- Template rendering test: verify __TASK__ is replaced in scope boundary context

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.6]
- [Source: docs/architecture/ralph-run-robustness.md#Область 4]
- [Source: docs/prd/ralph-run-robustness.md#FR79, FR80]
- [Source: runner/prompts/execute.md:59-79 — current Commit Rules and Task Scope sections]
- [Source: runner/prompts/agents/implementation.md:24-30 — current Instructions]

## Dev Agent Record

### Context Reference

### Agent Model Used

### Debug Log References

### Completion Notes List

- Validator correction: story said `__TASK__` but real placeholder is `__TASK_CONTENT__` (runner.go:165)
- Added `__TASK_CONTENT__` to execute replacements map in Runner.Execute() (line 888) and RunOnce() (line 1684) — required because execute.md now references it in SCOPE BOUNDARY
- Updated golden files for execute and implementation agent prompts (golden files include pre-existing format.md changes captured during regeneration)
- Implementation agent instructions renumbered: new point 5 (scope compliance), existing 5-7 became 6-8

### File List

- runner/prompts/execute.md — SCOPE BOUNDARY section (AC#1, AC#2)
- runner/prompts/agents/implementation.md — scope compliance check point 5 (AC#3)
- runner/prompt_test.go — 3 new test functions (AC#5)
- runner/runner.go — __TASK_CONTENT__ in executeReplacements and onceReplacements
- runner/testdata/TestPrompt_Execute_KnowledgeSections.golden — updated
- runner/testdata/TestPrompt_Execute_WithFindings.golden — updated
- runner/testdata/TestPrompt_Execute_WithoutFindings.golden — updated
- runner/testdata/TestPrompt_Agent_Golden_Implementation.golden — updated
