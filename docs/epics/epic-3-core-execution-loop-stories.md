# Epic 3: Core Execution Loop — Stories

**Scope:** FR6, FR7, FR8, FR9, FR10, FR11, FR12, FR23, FR24, FR36, FR37, FR38
**Stories:** 11
**Threshold check:** 11 < 13 → split не требуется

**Invariants (из Epics Structure Plan):**
- **Mutation Asymmetry:** Execute sessions MUST NOT modify sprint-tasks.md task status
- **Review atomicity:** `[x]` + clear findings = atomic operation (Epic 4, но runner готовит контракт)
- **KnowledgeWriter:** minimal interface (1-2 метода), no-op impl в Epic 3, реализация в Epic 6
- **Emergency gates:** minimal stop с exit code + informative message (не interactive UI — это Epic 5)
- **FR17 lessons deferred:** resume-extraction НЕ пишет LEARNINGS.md в Epic 3, только WIP commit + progress

---

### Story 3.1: Execute Prompt Template

**User Story:**
Как разработчик, я хочу чтобы execute-промпт содержал 999-rules guardrails, ATDD-инструкции, red-green cycle и self-directing поведение, чтобы Claude Code автономно и безопасно выполнял задачи.

**Acceptance Criteria:**

```gherkin
Scenario: Execute prompt assembled with all required sections
  Given config with project root and sprint-tasks path
  And optional review-findings.md exists
  When prompt is assembled via text/template + strings.Replace
  Then prompt contains 999-rules guardrail section (FR36)
  And prompt contains ATDD instruction: every AC must have test (FR37)
  And prompt contains zero-skip policy: never skip/xfail tests (FR38)
  And prompt contains red-green cycle instruction
  And prompt contains self-directing instruction: read sprint-tasks.md, take first `- [ ]` (FR11)
  And prompt contains instruction: MUST NOT modify task status markers (Mutation Asymmetry)
  And prompt contains instruction: commit on green tests only (FR8)
  And sprint-tasks-format.md content is injected via strings.Replace (not template)

Scenario: Execute prompt includes review-findings when present
  Given review-findings.md exists with CONFIRMED findings
  When prompt is assembled
  Then review-findings content is injected via strings.Replace
  And prompt contains instruction to fix findings before continuing

Scenario: Execute prompt without review-findings
  Given review-findings.md does not exist
  When prompt is assembled
  Then prompt does not contain findings section
  And prompt instructs Claude to proceed with next task

Scenario: Golden file snapshot matches baseline
  Given execute prompt template in runner/prompts/execute.md
  When assembled with test fixture data
  Then output matches runner/testdata/TestPrompt_Execute.golden
  And golden file is updateable via `go test -update`

Scenario: User content cannot break template
  Given LEARNINGS.md contains Go template syntax `{{.Dangerous}}`
  When prompt is assembled (stage 2: strings.Replace)
  Then template syntax in user content is preserved literally
  And no template execution error occurs
```

**Technical Notes:**
- Architecture: `runner/prompts/execute.md` — Go template file, NOT pure Markdown
- Two-stage assembly: (1) `text/template` for structure, (2) `strings.Replace` for user content (LEARNINGS, findings)
- Deterministic assembly order: template variables sorted or explicitly ordered (Graph of Thoughts finding)
- `config/shared/sprint-tasks-format.md` injected via strings.Replace (shared contract from Story 2.1)
- 999-rules = hard guardrails from Farr Playbook — execute refuses dangerous actions even from review-findings
- Architecture pattern: "Prompt файлы = .md файлы — Go templates, не чистый Markdown"

**Prerequisites:** Story 1.10 (prompt assembly), Story 2.1 (shared format contract)

---

### Story 3.2: Sprint-Tasks Scanner

**User Story:**
Как runner loop, мне нужно сканировать sprint-tasks.md для определения текущего состояния: есть ли открытые задачи, закрытые задачи, gate-маркеры, чтобы управлять потоком выполнения.

**Acceptance Criteria:**

```gherkin
Scenario: Scanner finds open tasks
  Given sprint-tasks.md contains lines with "- [ ]" markers
  When scanner parses the file
  Then returns list of TaskOpen entries with line numbers
  And uses config.TaskOpen constant for matching

Scenario: Scanner finds completed tasks
  Given sprint-tasks.md contains lines with "- [x]" markers
  When scanner parses the file
  Then returns list of TaskDone entries with line numbers
  And uses config.TaskDone constant for matching

Scenario: Scanner detects gate markers
  Given sprint-tasks.md contains "[GATE]" tag on a task line
  When scanner parses the file
  Then marks affected tasks with GateTag flag
  And uses config.GateTag constant for matching

Scenario: Soft validation — no tasks found
  Given sprint-tasks.md contains neither "- [ ]" nor "- [x]"
  When scanner parses the file
  Then returns ErrNoTasks sentinel error
  And error message recommends checking file contents (FR12)

Scenario: Scanner uses string constants
  Given config package defines TaskOpen, TaskDone, GateTag, FeedbackPrefix
  When scanner matches lines
  Then ONLY config constants and config regex patterns are used
  And no hardcoded marker strings exist in scan.go

Scenario: Table-driven tests cover edge cases
  Given test table with cases: empty file, only completed, mixed, gates, malformed lines
  When tests run
  Then all cases pass with correct counts and line numbers
```

**Technical Notes:**
- Architecture: `runner/scan.go` — line scanning + regex, no AST parser
- Pattern: `strings.Split(content, "\n")` + regex match
- Constants in `config/constants.go`: `TaskOpen`, `TaskDone`, `GateTag`, `FeedbackPrefix`
- Regex patterns as `var` with `regexp.MustCompile` in package scope
- Scanner returns structured result (not just bool) for loop control
- Ralph scans ONLY for loop control — does NOT extract task descriptions into prompt (FR11)

**Prerequisites:** Story 1.6 (config constants)

---

### Story 3.3: GitClient Interface + ExecGitClient

**User Story:**
Как runner, мне нужен GitClient interface с реализацией ExecGitClient для проверки здоровья git-репозитория при старте и обнаружения новых коммитов после execute-сессии.

**Acceptance Criteria:**

```gherkin
Scenario: Git health check passes on clean repo
  Given project directory is a git repo
  And working tree is clean (no uncommitted changes)
  And HEAD is not detached
  And not in merge/rebase state
  When GitClient.HealthCheck(ctx) is called
  Then returns nil error

Scenario: Git health check fails on dirty tree
  Given working tree has uncommitted changes
  When GitClient.HealthCheck(ctx) is called
  Then returns ErrDirtyTree sentinel error

Scenario: Git health check fails on detached HEAD
  Given HEAD is detached
  When GitClient.HealthCheck(ctx) is called
  Then returns error indicating detached HEAD

Scenario: Git health check fails during merge/rebase
  Given repo is in merge or rebase state
  When GitClient.HealthCheck(ctx) is called
  Then returns error indicating merge/rebase in progress

Scenario: Commit detection via HEAD comparison
  Given initial HEAD is "abc123"
  When execute session completes
  And GitClient.HeadCommit(ctx) returns "def456"
  Then commit is detected (new HEAD != old HEAD)

Scenario: No commit detection
  Given initial HEAD is "abc123"
  When execute session completes
  And GitClient.HeadCommit(ctx) still returns "abc123"
  Then no commit detected (trigger retry/resume-extraction)

Scenario: ExecGitClient uses exec.CommandContext
  Given ExecGitClient implementation
  When any git command is executed
  Then uses exec.CommandContext(ctx, "git", ...) with context propagation
  And cmd.Dir is set to config.ProjectRoot
```

**Technical Notes:**
- Architecture: `runner/git.go` — `GitClient` interface + `ExecGitClient`
- Interface defined in consumer package (`runner/`), NOT separate `git/` package
- Methods: `HealthCheck(ctx) error`, `HeadCommit(ctx) (string, error)` (2 methods only; `HealthCheck` returns `ErrDirtyTree` for dirty state — no separate `IsClean` method needed)
- Sentinel errors: `ErrDirtyTree`, `ErrDetachedHead` — в runner package scope
- Pattern: `cmd.Output()` for git, extract via `strings.TrimSpace`
- Health check runs at `ralph run` startup (FR6)
- No wall-clock timeout for git — cancellation only via ctx (Architecture decision)

**Prerequisites:** Story 1.8 (session package с exec.CommandContext pattern)

---

### Story 3.4: MockGitClient + Dirty State Recovery

**User Story:**
Как runner, мне нужно восстанавливать чистое состояние git при обнаружении dirty working tree (прерванная сессия), а как разработчик тестов, мне нужен MockGitClient для unit-тестирования runner без реального git.

**Acceptance Criteria:**

```gherkin
Scenario: Dirty state recovery via git checkout
  Given working tree is dirty (interrupted session)
  When runner detects dirty state at startup (FR12)
  Then executes `git checkout -- .` to restore clean state
  And logs warning about recovery action
  And proceeds with execution after recovery

Scenario: MockGitClient implements GitClient interface
  Given MockGitClient in internal/testutil/mock_git.go
  When used in tests
  Then implements all GitClient methods
  And returns preconfigured responses per test scenario
  And tracks method call counts for assertions

Scenario: MockGitClient simulates commit detection
  Given MockGitClient configured with HEAD sequence ["abc", "abc", "def"]
  When HeadCommit called three times
  Then returns "abc", "abc", "def" in order
  And test can verify commit detection logic

Scenario: MockGitClient simulates health check failure
  Given MockGitClient configured with HealthCheck returning ErrDirtyTree
  When runner calls HealthCheck at startup
  Then receives ErrDirtyTree
  And test can verify dirty state recovery path

Scenario: Recovery only runs when dirty tree detected
  Given working tree is clean
  When runner starts
  Then git checkout is NOT executed
  And execution proceeds normally
```

**Technical Notes:**
- Architecture: `internal/testutil/mock_git.go` — `MockGitClient`
- Deferred from Epic 1 Story 1.11 (Bob SM decision: MockGitClient belongs with GitClient in Epic 3)
- Pattern: `Mock` prefix naming convention
- Dirty recovery: `git checkout -- .` — discards uncommitted changes (Architecture: "Crash recovery через git checkout")
- MockGitClient stores call sequence for scenario-based testing
- Recovery logged as WARN level (Architecture: packages return errors, cmd/ logs)

**Prerequisites:** Story 3.3 (GitClient interface), Story 1.11 (mock Claude infra pattern)

---

### Story 3.5: Runner Loop Skeleton — Happy Path

**User Story:**
Как пользователь `ralph run`, я хочу чтобы система последовательно выполняла задачи из sprint-tasks.md, запуская свежую Claude Code сессию на каждую задачу и коммитя при green тестах.

**Acceptance Criteria:**

```gherkin
Scenario: Runner executes tasks sequentially
  Given sprint-tasks.md contains 3 open tasks (- [ ])
  And MockGitClient returns health check OK
  And MockClaude returns exit 0 with new commit for each execute
  When runner.Run(ctx, cfg) is called
  Then executes 3 sequential Claude sessions (FR6)
  And each session is fresh (new invocation, not --resume) (FR7)
  And passes --max-turns from config (FR10)

Scenario: Runner detects commit after execute
  Given execute session completes
  And HEAD changed from "abc" to "def"
  When runner checks for commit
  Then detects new commit (FR8)
  And proceeds to next phase (review in Epic 4, stub for now)

Scenario: Runner stops when all tasks complete
  Given sprint-tasks.md has no remaining "- [ ]" tasks
  When runner scans for next task
  Then returns successfully (exit code 0)
  And logs completion message

Scenario: Runner runs health check at startup
  Given ralph run invoked
  When runner starts
  Then calls GitClient.HealthCheck first (FR6)
  And fails with informative error if health check fails

Scenario: Runner uses session CLI flag constants
  Given config specifies max_turns=25 and model="sonnet"
  When runner invokes session.Execute
  Then passes --max-turns 25 (uses SessionFlagMaxTurns constant)
  And passes --model sonnet (uses SessionFlagModel constant)

Scenario: Runner stub for review step (configurable)
  Given execute completed with commit
  When review phase would run
  Then stub returns "clean" by default (no findings)
  And task advances to next
  And stub is clearly marked as placeholder for Epic 4
  And stub accepts configurable response sequence for testing (e.g., findings N times then clean)
  And this enables Story 3.10 review_cycles counter to be tested within Epic 3

Scenario: Mutation Asymmetry enforced
  Given runner loop completes a task
  When checking sprint-tasks.md modifications
  Then runner process itself NEVER writes task status markers
  And only Claude sessions (review in Epic 4) modify task status
```

**Technical Notes:**
- Architecture: `runner/runner.go` — `runner.Run(ctx, cfg)` main entry point
- Structural pattern: "Каждый package имеет одну главную exported функцию"
- Review step is a configurable stub returning "clean" by default — Epic 4 replaces with real review
- Review stub follows defined function signature: `func(ctx, ReviewOpts) (ReviewResult, error)`. `ReviewResult` contains: `Clean bool`, `FindingsPath string`. This establishes the seam for Epic 4 replacement
- Runner passes `config.ProjectRoot` as `cmd.Dir` for all sessions
- Context propagation: `exec.CommandContext(ctx)` for cancellation
- Happy path only — no retry, no resume, no emergency stops (Stories 3.6-3.10)
- Log file opened by `cmd/ralph`, runner returns results/errors

**Prerequisites:** Story 3.1 (execute prompt), Story 3.2 (scanner), Story 3.3 (GitClient), Story 1.8 (session)

---

### Story 3.6: Runner Retry Logic

**User Story:**
Как пользователь, я хочу чтобы система повторяла неудачные execute-сессии (нет коммита) до настраиваемого максимума, чтобы транзиентные ошибки AI не блокировали прогресс.

**Acceptance Criteria:**

```gherkin
Scenario: No commit triggers retry
  Given execute session completes with exit code 0
  But HEAD has not changed (no commit)
  When runner checks for commit
  Then increments execute_attempts counter (FR9)
  And triggers resume-extraction (Story 3.7)
  And retries execute after resume-extraction

Scenario: execute_attempts counter increments correctly
  Given task has execute_attempts = 0
  When first execute fails (no commit)
  Then execute_attempts becomes 1
  And when second execute fails
  Then execute_attempts becomes 2

Scenario: Commit resets counter
  Given execute_attempts = 2
  When execute session produces a commit
  Then execute_attempts resets to 0
  And proceeds to review phase

Scenario: Counter is per-task
  Given task A has execute_attempts = 2
  When task A succeeds and task B starts
  Then task B has execute_attempts = 0

Scenario: Max iterations configurable
  Given config has max_iterations = 5 (default 3)
  When execute_attempts reaches 5
  Then emergency stop triggers (Story 3.9)

Scenario: Non-zero exit code also triggers retry
  Given execute session returns non-zero exit code
  When runner processes result
  Then treats as failure (no commit expected)
  And increments execute_attempts
  And triggers resume-extraction
```

**Technical Notes:**
- Architecture: два независимых счётчика — `execute_attempts` (max_iterations) и `review_cycles` (max_review_iterations)
- `execute_attempts` counter: per-task, resets on commit
- Default `max_iterations` = 3 (from config)
- No-commit detection: compare HEAD before/after via GitClient.HeadCommit
- Resume-extraction trigger is the link to Story 3.7
- Emergency stop trigger is the link to Story 3.9
- NFR12: retry timing uses exponential backoff between attempts (e.g., 1s, 2s, 4s) to handle transient Claude CLI failures

**Prerequisites:** Story 3.5 (runner loop skeleton)

---

### Story 3.7: Resume-Extraction Integration

**User Story:**
Как система, я хочу возобновлять прерванные execute-сессии через `claude --resume`, чтобы сохранить WIP-прогресс и записать состояние выполнения перед retry.

**Acceptance Criteria:**

```gherkin
Scenario: Resume-extraction invokes --resume with session_id
  Given execute session returned session_id "abc-123" but no commit
  When resume-extraction triggers
  Then invokes claude --resume abc-123 (FR9)
  And session type is resume-extraction (not fresh execute)

Scenario: Resume-extraction creates WIP commit
  Given resume-extraction session completes
  When checking git state
  Then WIP commit exists (partial progress saved)
  And commit message indicates WIP status

Scenario: Resume-extraction writes progress to sprint-tasks.md
  Given resume-extraction session runs
  When it interacts with sprint-tasks.md
  Then progress notes are added (but NOT task status change)
  And Mutation Asymmetry preserved: no `[x]` marking

Scenario: KnowledgeWriter interface defined (no-op)
  Given KnowledgeWriter interface in runner/knowledge.go
  When resume-extraction calls KnowledgeWriter.WriteProgress(ctx, data)
  Then no-op implementation returns nil
  And interface has maximum 2 methods
  And interface is designed for extension in Epic 6

Scenario: KnowledgeWriter no-op does not write LEARNINGS.md
  Given no-op KnowledgeWriter implementation
  When resume-extraction completes
  Then LEARNINGS.md is NOT created or modified
  And FR17 lessons deferred to Epic 6

Scenario: Resume-extraction session uses correct flags
  Given session_id from previous execute
  When resume-extraction invoked
  Then uses --resume flag (uses SessionFlagResume constant)
  And inherits max_turns from config
```

**Technical Notes:**
- Architecture: `claude --resume <session_id>` — resume previous session
- FR17 lessons writing deferred to Epic 6 (Chaos Monkey finding) — no LEARNINGS.md in Epic 3
- KnowledgeWriter interface: `WriteProgress(ctx, ProgressData) error` + `WriteLessons(ctx, LessonsData) error` (max 2)
- `ProgressData` struct: `SessionID string`, `TaskDescription string` (minimum fields, establishes contract for Epic 6)
- `LessonsData` struct: `SessionID string`, `Content string` (minimum fields)
- No-op implementation: both methods return nil, structs defined but unused until Epic 6
- Interface in `runner/knowledge.go`, no-op impl + data structs in same file
- Extensible in Epic 6: add struct fields (not change method signatures), or wrap with richer impl
- Resume-extraction can write progress notes into sprint-tasks under the task but NOT change `- [ ]` / `- [x]`

**Prerequisites:** Story 3.6 (retry logic triggers resume-extraction), Story 1.9 (session --resume support)

---

### Story 3.8: Runner Resume on Re-run

**User Story:**
Как пользователь, я хочу чтобы при повторном запуске `ralph run` система продолжала с первой незавершённой задачи, восстанавливая dirty state если нужно.

**Acceptance Criteria:**

```gherkin
Scenario: Resume from first incomplete task
  Given sprint-tasks.md has tasks: [x] task1, [x] task2, [ ] task3, [ ] task4
  When ralph run starts
  Then scanner finds first "- [ ]" = task3
  And execution begins from task3 (FR12)

Scenario: All tasks completed
  Given sprint-tasks.md has only "- [x]" tasks
  When ralph run starts
  Then reports "all tasks completed"
  And exits with code 0

Scenario: Dirty tree recovery on re-run
  Given working tree is dirty (interrupted previous run)
  When ralph run starts
  And GitClient.HealthCheck returns ErrDirtyTree
  Then executes git checkout -- . for recovery (FR12)
  And logs warning about recovery
  And proceeds with first incomplete task

Scenario: Soft validation warning
  Given sprint-tasks.md contains no "- [ ]" and no "- [x]" markers
  When scanner parses file
  Then outputs warning recommending file check (FR12)
  And exits (no tasks to process)

Scenario: Re-run after partial completion
  Given 5 tasks total, 3 completed in previous run
  When ralph run re-invoked
  Then starts from task 4 (first "- [ ]")
  And does not re-execute completed tasks
```

**Technical Notes:**
- Architecture: "При повторном запуске ralph run продолжает с первой незавершённой задачи"
- Scanner already finds first `- [ ]` (Story 3.2) — this story wires it into runner startup
- Dirty recovery calls GitClient methods from Story 3.3/3.4
- Soft validation: ErrNoTasks from scanner → formatted warning
- No special "resume state" file — sprint-tasks.md IS the state (hub node)

**Prerequisites:** Story 3.5 (runner loop), Story 3.4 (dirty state recovery), Story 3.2 (scanner)

---

### Story 3.9: Emergency Stop — Execute Attempts Exhausted

**User Story:**
Как пользователь, я хочу чтобы система автоматически останавливалась когда AI исчерпал максимум попыток выполнения задачи, чтобы я не тратил ресурсы впустую.

**Acceptance Criteria:**

```gherkin
Scenario: Emergency stop when execute_attempts reaches max
  Given max_iterations = 3 (config default)
  And current task has execute_attempts = 3
  When runner checks counter before next attempt
  Then triggers emergency stop (FR23)
  And exits with code 1

Scenario: Informative stop message
  Given emergency stop triggered by execute_attempts
  When stop message is generated
  Then includes task name/description that failed
  And includes number of attempts made
  And includes suggestion to check logs
  And message uses fatih/color for visibility (via cmd/)

Scenario: Configurable max_iterations
  Given config has max_iterations = 5
  When execute_attempts reaches 5
  Then emergency stop triggers
  And does not trigger at 3 or 4

Scenario: Emergency stop is non-interactive
  Given emergency stop triggers
  When system stops
  Then does NOT prompt for user input (не interactive gate)
  And simply exits with error code + message
  And interactive gate upgrade is in Epic 5
```

**Technical Notes:**
- Architecture: "Emergency gate: minimal stop + message при execute_attempts >= max_iterations (FR23) — не interactive UI, просто stop с exit code + info"
- Returns sentinel error `ErrMaxRetries` — `cmd/ralph` maps to exit code 1
- Pattern: packages return errors, `cmd/ralph/` decides output format and exit codes
- Interactive upgrade (approve/retry/skip/quit) deferred to Epic 5
- Runner returns structured error with task info + attempt count for cmd/ formatting

**Prerequisites:** Story 3.6 (execute_attempts counter)

---

### Story 3.10: Emergency Stop — Review Cycles Trigger Point

**User Story:**
Как runner, мне нужно отслеживать счётчик review_cycles и останавливаться при превышении максимума, чтобы предотвратить бесконечные циклы execute→review.

**Acceptance Criteria:**

```gherkin
Scenario: review_cycles counter increments via configurable stub
  Given task enters review phase (configurable stub from Story 3.5)
  And review stub configured to return findings 2 times then clean
  When execute→review cycle repeats
  Then review_cycles counter increments to 2
  And on third review (clean) counter resets to 0

Scenario: Emergency stop at max_review_iterations
  Given max_review_iterations = 3 (config default)
  And review_cycles reaches 3
  When runner checks counter
  Then triggers emergency stop (FR24)
  And exits with code 1
  And message indicates review cycle exhaustion

Scenario: Counter is per-task
  Given task A had review_cycles = 2
  When task A completes (clean review) and task B starts
  Then task B has review_cycles = 0

Scenario: Counter resets on clean review
  Given review_cycles = 1
  When review returns clean (no findings)
  Then review_cycles resets to 0
  And task marked complete (by review in Epic 4)

Scenario: Configurable max_review_iterations
  Given config has max_review_iterations = 5
  When review_cycles reaches 5
  Then emergency stop triggers

Scenario: Trigger point prepared for Epic 4
  Given review stub returns "clean" in Epic 3
  When review is replaced with real review in Epic 4
  Then review_cycles counter already integrated
  And no runner loop changes needed for FR24
```

**Technical Notes:**
- Architecture: два независимых счётчика — `execute_attempts` (FR23) и `review_cycles` (FR24)
- In Epic 3: review is a configurable stub (Story 3.5), review_cycles counter and max check are real code tested via stub sequences
- In Epic 4: real review replaces stub, counter logic already works
- Actual interactive gate for FR24 in Epic 5
- Same pattern as Story 3.9: returns structured error, cmd/ formats output
- Declares `ErrMaxReviewCycles` sentinel error in runner package (alongside ErrDirtyTree, ErrDetachedHead from Story 3.3)

**Prerequisites:** Story 3.5 (runner loop with review stub), Story 3.9 (emergency stop pattern)

---

### Story 3.11: Runner Integration Test

**User Story:**
Как разработчик, я хочу комплексный integration test runner-а, который проверяет full flow через scenario-based mock Claude и MockGitClient, чтобы гарантировать корректную работу всех компонентов вместе.

**Acceptance Criteria:**

```gherkin
Scenario: Happy path — all tasks complete
  Given scenario JSON with 3 execute steps (all produce commits)
  And MockGitClient returns health OK + commit sequence
  And sprint-tasks.md fixture with 3 open tasks
  When runner.Run executes
  Then 3 sessions launched sequentially
  And runner exits with code 0

Scenario: Retry + resume-extraction flow
  Given scenario JSON: execute (no commit) → resume-extraction → execute (commit)
  And MockGitClient returns no-commit then commit
  When runner.Run executes
  Then execute_attempts increments to 1
  And resume-extraction invoked with session_id
  And retry succeeds with commit

Scenario: Emergency stop on max retries
  Given scenario JSON: 3 executes all without commit
  And max_iterations = 3
  When runner.Run executes
  Then execute_attempts reaches 3
  And runner returns ErrMaxRetries
  And error contains task info

Scenario: Resume on re-run after partial completion
  Given sprint-tasks.md with 2 completed + 1 open task
  And scenario JSON for 1 execute (commit)
  When runner.Run executes
  Then starts from task 3 (first open)
  And completes successfully

Scenario: Dirty tree recovery at startup
  Given MockGitClient.HealthCheck returns ErrDirtyTree
  When runner starts
  Then recovery executed (git checkout -- .)
  And then proceeds normally

Scenario: Resume-extraction failure triggers recovery
  Given scenario JSON: execute (no commit) → resume-extraction (exit code non-zero)
  And MockGitClient shows dirty tree after failed resume
  When runner processes resume-extraction failure
  Then triggers dirty state recovery (git checkout -- .)
  And retries execute from clean state
  And execute_attempts is correctly tracked through the recovery

Scenario: Uses bridge golden file output as input
  Given sprint-tasks.md from bridge golden file testdata (not hand-crafted)
  When used as runner test input
  Then validates bridge→runner data contract
  And proves end-to-end compatibility

Scenario: Test isolation
  Given integration test
  When test runs
  Then uses t.TempDir() for all file operations
  And no shared state between test cases
  And build tag //go:build integration
```

**Technical Notes:**
- Architecture: "scenario = ordered sequence of mock responses"
- Scenario JSON format: `[{type, exit_code, session_id, creates_commit, output_file}]`
- Uses MockClaude from Story 1.11 + MockGitClient from Story 3.4
- Bridge golden file output (Story 2.5) used as sprint-tasks.md input — validates contract
- Build tag: `//go:build integration`
- Test file: `runner/runner_integration_test.go`
- Review step uses stub returning "clean" — real review integration in Epic 4
- Scenarios cover: happy path, retry, resume, emergency stop, re-run, dirty recovery

**Prerequisites:** Story 3.5-3.10 (all runner stories), Story 3.4 (MockGitClient), Story 1.11 (MockClaude), Story 2.5 (bridge golden files)

---

### Epic 3 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 3.1 | Execute Prompt Template | FR8,FR11,FR36,FR37,FR38 | 2 + testdata | 5 |
| 3.2 | Sprint-Tasks Scanner | FR6,FR11,FR12 | 2 | 6 |
| 3.3 | GitClient Interface + ExecGitClient | FR6 | 2 | 7 |
| 3.4 | MockGitClient + Dirty State Recovery | FR12 | 2 | 5 |
| 3.5 | Runner Loop Skeleton | FR6,FR7,FR8,FR10,FR11 | 2 | 8 |
| 3.6 | Runner Retry Logic | FR9 | 1 | 6 |
| 3.7 | Resume-Extraction Integration | FR9 | 2 | 6 |
| 3.8 | Runner Resume on Re-run | FR12 | 1 | 5 |
| 3.9 | Emergency Stop — Execute | FR23 | 1 | 4 |
| 3.10 | Emergency Stop — Review Cycles | FR24 | 1 | 6 |
| 3.11 | Runner Integration Test | — | 1 + scenarios | 8 |
| | **Total** | **FR6-FR12,FR23,FR24,FR36-FR38** | | **~66** |

**FR Coverage:** FR6 (3.2, 3.3, 3.5), FR7 (3.5), FR8 (3.1, 3.5), FR9 (3.6, 3.7), FR10 (3.5), FR11 (3.1, 3.2, 3.5), FR12 (3.2, 3.4, 3.8), FR23 (3.9), FR24 (3.10), FR36 (3.1), FR37 (3.1), FR38 (3.1)

**Architecture Sections Referenced:** Runner package (runner.go, git.go, scan.go, knowledge.go), Subprocess Patterns, Testing Patterns, Error Handling, File I/O, Project Structure

**Dependency Graph:**
```
1.6 ─────→ 3.2
1.8 ─────→ 3.3 ──→ 3.4
1.10 ────→ 3.1
2.1 ─────→ 3.1
1.8 ─────→ 3.5 ←── 3.1, 3.2, 3.3
             │
             ├──→ 3.6 ──→ 3.7
             │           ↗
             │     1.8 ─┘
             ├──→ 3.8 ←── 3.4
             └──→ 3.9 ←── 3.6
                  3.10 ←── 3.5, 3.9 (pattern)
1.11 ────→ 3.4, 3.11
2.5 ─────→ 3.11
3.5-3.10 → 3.11
```
Note: 3.3 и 3.1 parallel-capable (оба зависят от разных Epic 1 stories). 3.9 и 3.10 partially parallel (3.10 reuses pattern from 3.9)

---
