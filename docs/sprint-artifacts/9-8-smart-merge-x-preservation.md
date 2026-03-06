# Story 9.8: Smart Merge [x] Preservation

Status: review

## Story

As a разработчик,
I want чтобы при перегенерации sprint-tasks.md через bridge статусы `[x]` из предыдущей версии сохранялись,
so that не теряется информация о выполненных задачах.

## Acceptance Criteria

1. **SmartMergeStatus preserves [x] (FR69):**
   - Old: `"- [x] Add validation"`, `"- [ ] Fix bug"`
   - New: `"- [ ] Add validation"`, `"- [ ] Fix bug"`, `"- [ ] New task"`
   - Result: `"- [x] Add validation"` (preserved), `"- [ ] Fix bug"` (unchanged), `"- [ ] New task"` (new, stays [ ])

2. **SmartMergeStatus matches by hash (FR69):**
   - Old "- [x] Add user validation with tests" and new "- [ ] Add user validation with tests"
   - TaskHash matches for both → [x] transferred from old to new

3. **SmartMergeStatus ignores reordering (FR69):**
   - Old: "- [x] Task A", "- [ ] Task B"
   - New (reordered): "- [ ] Task B", "- [ ] Task A"
   - Result: "- [ ] Task B", "- [x] Task A" (preserved despite reordering)

4. **SmartMergeStatus with removed tasks (FR69):**
   - Old: "- [x] Task A", "- [x] Task B"
   - New: "- [ ] Task A" (Task B removed)
   - Result: "- [x] Task A" — Task B silently dropped (not in new)

5. **SmartMergeStatus with non-task lines (FR69):**
   - Headers "## Section" and empty lines preserved from new content unchanged
   - Only task lines with matching hash get [x] transfer

6. **SmartMergeStatus with empty old content:**
   - `SmartMergeStatus("", newContent)` returns newContent (no changes)

## Tasks / Subtasks

- [x] Task 1: Implement `SmartMergeStatus` in `runner/preflight.go` (AC: #1-#6)
  - [x] Parse old content → build `map[string]bool` (hash → done status)
  - [x] Iterate new content lines → if task line and hash in map and done → replace `[ ]` with `[x]`
  - [x] Preserve non-task lines unchanged from new content
  - [x] Handle empty old content
- [x] Task 2: Write comprehensive tests (AC: #1-#6)
  - [x] Table-driven tests covering all scenarios
  - [x] Verify exact output content (not just [x] count)
  - [x] Test with headers, empty lines, mixed content
  - [x] Test empty old content
  - [x] Test reordering preservation

## Dev Notes

### Architecture & Design

- **File:** `runner/preflight.go` — alongside TaskHash (from Story 9.7)
- **Uses:** `TaskHash` for matching tasks between old and new content
- **No new dependencies**
- **Integration point:** Called from bridge when regenerating sprint-tasks.md (integration TBD)

### Algorithm

```go
func SmartMergeStatus(oldContent, newContent string) string {
    if oldContent == "" {
        return newContent
    }

    // 1. Parse old: build map[hash]bool for done tasks
    doneHashes := map[string]bool{}
    for _, line := range strings.Split(oldContent, "\n") {
        if strings.HasPrefix(line, "- [x] ") {
            h := TaskHash(line)
            doneHashes[h] = true
        }
    }

    // 2. Process new: transfer [x] where hash matches
    var result []string
    for _, line := range strings.Split(newContent, "\n") {
        if strings.HasPrefix(line, "- [ ] ") {
            h := TaskHash(line)
            if doneHashes[h] {
                line = strings.Replace(line, "- [ ] ", "- [x] ", 1)
            }
        }
        result = append(result, line)
    }

    return strings.Join(result, "\n")
}
```

### Key Points

- Uses `config.TaskOpen` ("- [ ]") and `config.TaskDone` ("- [x]") constants for prefix matching
- Note: `runner/preflight.go` imports `config` only for constants — check if this violates dependency direction (runner → config is allowed)
- Does NOT modify task description text — only checkbox status
- Non-task lines (headers, empty lines, comments) pass through unchanged

### Existing Scaffold Context

- `config/constants.go` — `TaskOpen = "- [ ]"`, `TaskDone = "- [x]"`
- `runner/scan.go` — ScanTasks already parses task lines
- `bridge/bridge.go` — bridge generates sprint-tasks.md (potential integration point)
- Story 9.7 creates `runner/preflight.go` with TaskHash — SmartMergeStatus is added to same file

### Testing Standards

- Table-driven tests with exact string comparison of output
- Verify non-task lines preserved verbatim
- Test ordering: verify [x] transfers even when tasks reordered
- Empty content edge case

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.8]
- [Source: docs/architecture/ralph-run-robustness.md#Область 1 — SmartMergeStatus]
- [Source: docs/prd/ralph-run-robustness.md#FR69]
- [Source: config/constants.go — TaskOpen, TaskDone]
- [Source: runner/scan.go — ScanTasks parsing]

## Dev Agent Record

### Context Reference

### Agent Model Used

### Debug Log References

### Completion Notes List

- SmartMergeStatus uses config.TaskOpen/TaskDone constants for prefix matching (not hardcoded strings)
- Uses TaskHash from Story 9.7 for hash-based matching between old and new content
- Added config import to preflight.go (runner → config is allowed per dependency direction)
- 9 table-driven test cases covering all 6 ACs plus edge cases (both empty, no done, multiple done)
- Exact string comparison in all tests (not substring or count assertions)

### File List

- runner/preflight.go — SmartMergeStatus function + config import (AC#1-#6)
- runner/preflight_test.go — TestSmartMergeStatus_Scenarios, 9 table cases (AC#1-#6)
