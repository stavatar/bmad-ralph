# Story 9.7: Pre-flight Check + TaskHash + LogOneline

Status: review

## Story

As a разработчик,
I want чтобы Ralph перед запуском execute проверял git log на наличие уже выполненной задачи,
so that не тратятся токены на повторное выполнение.

## Acceptance Criteria

1. **TaskHash computation (FR67):**
   - `TaskHash(text)` returns first 6 hex chars of SHA-256 of task description
   - Description = text after stripping "- [ ] " or "- [x] " prefix
   - Hash is deterministic (same input -> same output)
   - Hash is lowercase hex string of length 6

2. **TaskHash strips prefix (FR67):**
   - "- [ ] Add validation" -> hashes "Add validation"
   - "- [x] Add validation" -> same hash (same description)
   - "Add validation" (no prefix) -> same hash

3. **GitClient.LogOneline (FR68):**
   - New method on GitClient interface: `LogOneline(ctx context.Context, n int) ([]string, error)`
   - Returns last n commits as one-line strings
   - Implementation: `git log --oneline -<n>`
   - Error returned if git not available

4. **PreFlightCheck skip (FR68.3):**
   - Task with hash "a1b2c3", git log contains commit "feat: add validation [task:a1b2c3]"
   - `review-findings.md` does not exist
   - Returns skip=true, reason contains "commit found, no findings"

5. **PreFlightCheck proceed with findings (FR68.4):**
   - Task hash found in git log, but review-findings.md exists with content
   - Returns skip=false, reason contains "commit found but findings exist"

6. **PreFlightCheck proceed no commit (FR68.5):**
   - Task hash NOT in git log
   - Returns skip=false, reason contains "no matching commit"

7. **PreFlightCheck graceful on git error:**
   - Git log returns error -> skip=false (proceed, don't skip)
   - Error logged but not propagated (best-effort)

8. **Execute.md маркер requirement (FR67):**
   - `runner/prompts/execute.md` Commit Rules contains instruction: add `[task:__TASK_HASH__]` to commit message
   - `__TASK_HASH__` is a new placeholder in `buildTemplateData()`

9. **Integration in Execute() (FR68):**
   - Pre-flight check called before execute cycle for each open task
   - skip=true -> task marked [x], execute cycle skipped, log: "INFO pre-flight skip: <reason>"
   - MetricsCollector records task as skipped

## Tasks / Subtasks

- [x] Task 1: Create `runner/preflight.go` with TaskHash function (AC: #1, #2)
  - [x] `crypto/sha256` + `encoding/hex` -> first 6 chars
  - [x] Strip "- [ ] " / "- [x] " prefix before hashing
  - [x] Handle input with and without prefix
- [x] Task 2: Add `LogOneline` to GitClient interface and ExecGitClient (AC: #3)
  - [x] Add method to interface in `runner/git.go`
  - [x] Implement in ExecGitClient: `git log --oneline -<n>`
  - [x] Add to MockGitClient in test helpers
- [x] Task 3: Implement `PreFlightCheck` in `runner/preflight.go` (AC: #4-#7)
  - [x] Call `LogOneline(ctx, 20)`, search for `[task:<hash>]`
  - [x] Check `review-findings.md` existence and content
  - [x] Return (skip bool, reason string)
  - [x] Graceful on git error: proceed, don't skip
- [x] Task 4: Add `[task:__TASK_HASH__]` to execute.md commit rules (AC: #8)
  - [x] Add instruction and example in Commit Rules section
  - [x] Add `__TASK_HASH__` to executeReplacements in Execute() and RunOnce()
  - [x] Note: TemplateData not modified — __TASK_HASH__ is Stage 2 replacement
- [x] Task 5: Integrate PreFlightCheck in Execute() loop (AC: #9)
  - [x] Call before execute cycle for each task
  - [x] On skip: SkipTask [x], FinishTask("skipped"), log, continue
- [x] Task 6: Write comprehensive tests (AC: #1-#9)
  - [x] TaskHash: table-driven with prefix stripping, determinism, length
  - [x] PreFlightCheck: skip, proceed-with-findings, proceed-no-commit, git-error
  - [x] Template test: verify __TASK_HASH__ replaced in execute prompt

## Dev Notes

### Architecture & Design

- **New file:** `runner/preflight.go` — TaskHash, PreFlightCheck
- **Modified files:** `runner/git.go` (LogOneline), `runner/runner.go` (Execute integration), `runner/prompts/execute.md`, `config/prompt.go`
- **No new dependencies** — uses `crypto/sha256`, `encoding/hex` from stdlib

### Critical Implementation Details

**TaskHash:**
```go
func TaskHash(taskText string) string {
    desc := taskText
    desc = strings.TrimPrefix(desc, "- [ ] ")
    desc = strings.TrimPrefix(desc, "- [x] ")
    h := sha256.Sum256([]byte(desc))
    return hex.EncodeToString(h[:])[:6]
}
```

**PreFlightCheck signature:**
```go
func PreFlightCheck(ctx context.Context, git GitClient, taskText, projectRoot string) (skip bool, reason string)
```

Note: returns (bool, string) — NO error return. Git errors are logged internally, treated as "proceed".

**LogOneline on GitClient interface:**
```go
type GitClient interface {
    HealthCheck(ctx context.Context) error
    HeadCommit(ctx context.Context) (string, error)
    RestoreClean(ctx context.Context) error
    DiffStats(ctx context.Context, before, after string) (*DiffStats, error)
    LogOneline(ctx context.Context, n int) ([]string, error)  // NEW
}
```

**Execute.md commit rules addition:**
```markdown
- В конце commit message добавь маркер: [task:__TASK_HASH__]
- Пример: feat: add user validation [task:a1b2c3]
```

### MockGitClient Extension

Must add `LogOneline` to MockGitClient in test helpers:
```go
type MockGitClient struct {
    // ... existing fields
    LogOnelineResponses [][]string
    LogOnelineCount     int
}
```

### Existing Scaffold Context

- `runner/git.go:17-22` — current GitClient interface (4 methods)
- `runner/git.go:31+` — ExecGitClient implementation
- `runner/runner.go` — Execute() loop iterates over openTasks
- `runner/prompts/execute.md:59-66` — Commit Rules section
- `config/prompt.go:29-46` — TemplateData struct
- `config/constants.go` — TaskOpen, TaskDone constants for prefix matching

### Testing Standards

- Table-driven tests for TaskHash
- Pre-flight tests: use mock GitClient with scripted LogOneline responses
- Use `t.TempDir()` for review-findings.md presence/absence tests
- Verify hash determinism: same input = same output across runs

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.7]
- [Source: docs/architecture/ralph-run-robustness.md#Область 1]
- [Source: docs/prd/ralph-run-robustness.md#FR67, FR68]
- [Source: runner/git.go:17-22 — current GitClient interface]
- [Source: runner/runner.go — Execute() loop]
- [Source: runner/prompts/execute.md:59-66 — Commit Rules]
- [Source: config/prompt.go:29-46 — TemplateData]

## Dev Agent Record

### Context Reference

### Agent Model Used

### Debug Log References

### Completion Notes List

- TaskHash uses Stage 2 replacement (__TASK_HASH__), NOT TemplateData field — no config/prompt.go changes needed
- LogOneline added to GitClient interface — required updating nopGitClient, syncTestGitClient, MockGitClient
- PreFlightCheck returns (bool, string) not error — best-effort design per AC#7
- Execute() integration: PreFlightCheck called after StartTask, before review cycle loop
- RunReview also gets __TASK_HASH__ in replacements for consistency

### File List

- runner/preflight.go (new — TaskHash, PreFlightCheck)
- runner/preflight_test.go (new — 6 test functions)
- runner/git.go (modified — LogOneline on interface + ExecGitClient)
- runner/runner.go (modified — __TASK_HASH__ in replacements, PreFlightCheck in Execute)
- runner/prompts/execute.md (modified — [task:__TASK_HASH__] in Commit Rules)
- runner/prompt_test.go (modified — __TASK_HASH__ in helpers, TaskHashMarker test)
- internal/testutil/mock_git.go (modified — LogOneline on MockGitClient)
- runner/coverage_internal_test.go (modified — LogOneline on nopGitClient)
- runner/serena_sync_test.go (modified — LogOneline on syncTestGitClient)
- runner/testdata/*.golden (updated)
