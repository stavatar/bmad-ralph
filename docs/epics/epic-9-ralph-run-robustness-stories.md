# Epic 9: Ralph Run Robustness — Stories

**Scope:** FR67-FR80
**Stories:** 9
**Release milestone:** v0.6
**PRD:** [docs/prd/ralph-run-robustness.md](../prd/ralph-run-robustness.md)
**Architecture:** [docs/architecture/ralph-run-robustness.md](../architecture/ralph-run-robustness.md)

**Context:**
При тестировании Ralph Run на реальном проекте (mentorlearnplatform, 15 задач, 3 рана, $38.44) выявлены 4 системные проблемы: повторное выполнение задач ($1-3 впустую), WSL/Windows path баг (distillation failure), гидра-паттерн в review ($20+ на задачу), scope creep. Epic 9 решает все 4 проблемы через прогрессивную схему review, filepath нормализацию, pre-flight проверки и scope boundary enforcement.

**Dependency structure:**
```
9.1 Progressive types + config ──┐
9.4 Session.Options.Env ─────────┼──→ 9.2 Severity filtering + budget
                                 └──→ 9.3 Scope lock + effort escalation
9.5 Filepath normalization + graceful degradation
9.6 Scope creep prompts
9.7 Pre-flight + TaskHash + LogOneline ──→ 9.8 Smart merge [x]
9.9 Agent stats (depends on 9.1)
```

**Existing scaffold:**
- `runner/runner.go` — Runner struct (15 fields), Execute(), execute(), RealReview(), DetermineReviewOutcome(), selectReviewModel(), RunConfig, ReviewResult, findingSeverityRe
- `runner/metrics.go` — ReviewFinding (4 fields), MetricsCollector, RunMetrics, TaskMetrics, RecordFindingsCycle()
- `runner/scan.go` — TaskEntry, ScanResult, ScanTasks()
- `runner/git.go` — GitClient interface, RealGit struct, HeadCommit(), HealthCheck()
- `runner/knowledge_distill.go` — AutoDistill(), distillation pipeline
- `runner/knowledge_write.go` — WriteLearnings(), WriteRules()
- `session/session.go` — Options struct (9 fields), Execute()
- `config/config.go` — Config struct (25+ fields), defaults cascade, Validate()
- `config/defaults.yaml` — max_review_iterations: 3
- `runner/prompts/execute.md` — execute template с Commit Rules секцией
- `runner/prompts/review.md` — review template с 5 sub-agents, формат findings (4 поля)
- `runner/prompts/agents/implementation.md` — implementation review agent

---

### Story 9.1: Progressive Review Types + Config Default

**User Story:**
Как разработчик, я хочу иметь типы и функции для прогрессивной схемы review, чтобы параметры review (severity порог, бюджет замечаний, scope, effort) определялись номером цикла.

**Acceptance Criteria:**

```gherkin
Scenario: SeverityLevel type and constants (FR72)
  Given runner/progressive.go defines SeverityLevel type (int)
  When constants defined via iota:
    SeverityLow = 0
    SeverityMedium = 1
    SeverityHigh = 2
    SeverityCritical = 3
  Then all 4 severity levels have correct ordering (Low < Medium < High < Critical)

Scenario: ParseSeverity converts strings (FR73)
  Given strings from review-findings.md: "LOW", "MEDIUM", "HIGH", "CRITICAL"
  When ParseSeverity(s) called
  Then "LOW" → SeverityLow
  And "MEDIUM" → SeverityMedium
  And "HIGH" → SeverityHigh
  And "CRITICAL" → SeverityCritical
  And "low" (case-insensitive) → SeverityLow
  And "" or unknown → SeverityLow (fallback)

Scenario: ProgressiveParams for default 6 cycles (FR72)
  Given maxCycles = 6
  When ProgressiveParams(cycle, 6) called for cycles 1-6
  Then cycle 1 → minSeverity=LOW, maxFindings=5, incrementalDiff=false, highEffort=false
  And cycle 2 → minSeverity=LOW, maxFindings=5, incrementalDiff=false, highEffort=false
  And cycle 3 → minSeverity=MEDIUM, maxFindings=3, incrementalDiff=true, highEffort=true
  And cycle 4 → minSeverity=HIGH, maxFindings=1, incrementalDiff=true, highEffort=true
  And cycle 5 → minSeverity=CRITICAL, maxFindings=1, incrementalDiff=true, highEffort=true
  And cycle 6 → minSeverity=CRITICAL, maxFindings=1, incrementalDiff=true, highEffort=true

Scenario: ProgressiveParams scales for non-6 maxCycles (FR72)
  Given maxCycles = 3
  When ProgressiveParams(cycle, 3) called
  Then cycle 1 → minSeverity=LOW, maxFindings=5, incrementalDiff=false, highEffort=false
  And cycle 2 → minSeverity=MEDIUM, maxFindings=3, incrementalDiff=true, highEffort=true
  And cycle 3 → minSeverity=CRITICAL, maxFindings=1, incrementalDiff=true, highEffort=true

Scenario: ProgressiveParams edge cases
  Given various edge cases
  When ProgressiveParams(0, 6) called → same as cycle 1 (clamped)
  And ProgressiveParams(7, 6) called → same as cycle 6 (clamped)
  And ProgressiveParams(1, 1) called → minSeverity=CRITICAL, maxFindings=1

Scenario: Config default change (FR72)
  Given config/defaults.yaml
  When max_review_iterations value checked
  Then value is 6 (was 3)
  And Config.Load() with empty config → MaxReviewIterations == 6
```

**Technical Notes:**
- `runner/progressive.go`: новый файл с SeverityLevel, ParseSeverity, ProgressiveParams
- `config/defaults.yaml`: `max_review_iterations: 6`
- ProgressiveParams возвращает struct `ProgressiveReviewParams{MinSeverity, MaxFindings, IncrementalDiff, HighEffort}` для читаемости
- Масштабирование: первые ~33% циклов = LOW, ~50% = MEDIUM, ~67% = HIGH, остальные = CRITICAL
- Без зависимостей от других stories

**Prerequisites:** Нет

---

### Story 9.2: Severity Filtering + Findings Budget

**User Story:**
Как разработчик, я хочу фильтровать findings по severity и ограничивать их количество, чтобы review на поздних циклах фокусировался на критических проблемах и не генерировал новые мелкие замечания.

**Acceptance Criteria:**

```gherkin
Scenario: FilterBySeverity removes low findings (FR73)
  Given findings = [{CRITICAL, "a"}, {HIGH, "b"}, {MEDIUM, "c"}, {LOW, "d"}]
  When FilterBySeverity(findings, SeverityMedium) called
  Then result = [{CRITICAL, "a"}, {HIGH, "b"}, {MEDIUM, "c"}]
  And LOW finding "d" is excluded

Scenario: FilterBySeverity at CRITICAL threshold (FR73)
  Given findings = [{CRITICAL, "a"}, {HIGH, "b"}, {LOW, "c"}]
  When FilterBySeverity(findings, SeverityCritical) called
  Then result = [{CRITICAL, "a"}]

Scenario: FilterBySeverity at LOW threshold passes all (FR73)
  Given findings = [{HIGH, "a"}, {LOW, "b"}]
  When FilterBySeverity(findings, SeverityLow) called
  Then result = [{HIGH, "a"}, {LOW, "b"}] (all pass)

Scenario: FilterBySeverity with empty input
  Given findings = []
  When FilterBySeverity(findings, SeverityHigh) called
  Then result = [] (empty, no panic)

Scenario: TruncateFindings limits count (FR75)
  Given findings = [{CRITICAL, "a"}, {HIGH, "b"}, {MEDIUM, "c"}, {LOW, "d"}, {LOW, "e"}]
  When TruncateFindings(findings, 3) called
  Then result has exactly 3 findings
  And highest severity findings preserved: CRITICAL, HIGH, MEDIUM

Scenario: TruncateFindings sorts by severity (FR75)
  Given findings = [{LOW, "a"}, {CRITICAL, "b"}, {MEDIUM, "c"}]
  When TruncateFindings(findings, 2) called
  Then result = [{CRITICAL, "b"}, {MEDIUM, "c"}]

Scenario: TruncateFindings when count exceeds budget
  Given findings = [{HIGH, "a"}]
  When TruncateFindings(findings, 5) called
  Then result = [{HIGH, "a"}] (no truncation needed)

Scenario: Integration in Execute cycle (FR73, FR75)
  Given Runner.Execute() processes review cycle 4 (maxIter=6)
  When DetermineReviewOutcome returns 3 findings (CRITICAL, MEDIUM, LOW)
  Then FilterBySeverity with HIGH threshold removes MEDIUM and LOW
  And TruncateFindings with budget=1 keeps only CRITICAL
  And only CRITICAL finding written to review-findings.md for next execute
```

**Technical Notes:**
- `runner/progressive.go`: добавить FilterBySeverity, TruncateFindings
- FilterBySeverity использует ParseSeverity для сравнения finding.Severity с threshold
- TruncateFindings сортирует по severity (desc), обрезает до maxCount
- Фильтрация вызывается в Execute() ПОСЛЕ DetermineReviewOutcome, НЕ внутри него
- Отфильтрованные findings логируются: `INFO finding below threshold: [SEVERITY] description`

**Prerequisites:** Story 9.1

---

### Story 9.3: Scope Lock + Effort Escalation

**User Story:**
Как разработчик, я хочу чтобы review на поздних циклах получал инкрементальный diff и контекст предыдущих findings, а execute запускался с extended thinking, чтобы сложные замечания исправлялись качественнее.

**Acceptance Criteria:**

```gherkin
Scenario: RunConfig extended with progressive fields (FR72)
  Given RunConfig struct in runner.go
  When new fields added:
    Cycle int
    MinSeverity SeverityLevel
    MaxFindings int
    IncrementalDiff bool
    PrevFindings string
  Then all fields accessible in RealReview and Execute

Scenario: Incremental diff on cycle 3+ (FR74)
  Given cycle >= 3 and ProgressiveParams returns incrementalDiff=true
  When RealReview called
  Then review prompt receives git diff HEAD~1..HEAD (last commit only)
  And review prompt contains task description and story reference
  And review prompt contains previous cycle findings text
  And review prompt contains instruction: "проверь корректность исправлений и отсутствие новых проблем уровня <порог>+"

Scenario: Full diff on cycles 1-2 (FR74)
  Given cycle <= 2 and ProgressiveParams returns incrementalDiff=false
  When RealReview called
  Then review prompt receives full task diff (existing behavior)

Scenario: Effort escalation via environment variable (FR76)
  Given cycle >= 3 and ProgressiveParams returns highEffort=true
  When execute session launched
  Then session.Options.Env contains {"CLAUDE_CODE_EFFORT_LEVEL": "high"}

Scenario: No effort escalation on early cycles (FR76)
  Given cycle <= 2 and ProgressiveParams returns highEffort=false
  When execute session launched
  Then session.Options.Env is nil or empty

Scenario: Model escalation on late cycles (FR72)
  Given cycle >= 3 and highEffort=true
  When selectReviewModel called
  Then returns cfg.ModelReview (full model, never light)
  And selectReviewModel signature includes highEffort bool parameter

Scenario: Review prompt with findings budget (FR75)
  Given cycle 4, maxFindings=1 from ProgressiveParams
  When review prompt assembled
  Then prompt contains instruction: "Найди НЕ БОЛЕЕ 1 самых важных замечаний. Приоритизируй по severity"

Scenario: Execute loop integration (FR72-FR76)
  Given Runner.Execute() with maxIter=6
  When processing task through cycles:
  Then cycle 1: full diff, LOW+ threshold, budget 5, standard model
  And cycle 3: incremental diff, MEDIUM+ threshold, budget 3, max model + effort=high
  And cycle 5: incremental diff, CRITICAL threshold, budget 1, max model + effort=high
```

**Technical Notes:**
- `runner/runner.go`: расширить RunConfig 5 полями, обновить Execute() цикл для вызова ProgressiveParams и передачи в RealReview
- `runner/runner.go`: обновить selectReviewModel — добавить параметр `highEffort bool`
- `runner/prompts/review.md`: добавить conditional секцию для инкрементального режима (prev findings, budget instruction)
- `config/prompt.go`: расширить TemplateData для review промпта (Cycle, MinSeverityLabel, MaxFindings, PrevFindings, IncrementalDiff)
- Инкрементальный diff: `git diff HEAD~1..HEAD` через GitClient

**Prerequisites:** Story 9.1, Story 9.4

---

### Story 9.4: Session Options.Env

**User Story:**
Как разработчик, я хочу передавать дополнительные переменные окружения в Claude Code сессию, чтобы управлять extended thinking через `CLAUDE_CODE_EFFORT_LEVEL`.

**Acceptance Criteria:**

```gherkin
Scenario: Options.Env field (FR76)
  Given session.Options struct
  When Env field added: Env map[string]string
  Then field is optional (nil = no extra env vars)

Scenario: Env passed to subprocess
  Given Options.Env = {"CLAUDE_CODE_EFFORT_LEVEL": "high"}
  When session.Execute() creates exec.Cmd
  Then cmd.Env includes all os.Environ() entries
  And cmd.Env includes CLAUDE_CODE_EFFORT_LEVEL=high

Scenario: Nil Env preserves existing behavior
  Given Options.Env = nil
  When session.Execute() creates exec.Cmd
  Then cmd.Env is nil (inherits parent environment, existing behavior)

Scenario: Multiple env vars
  Given Options.Env = {"KEY1": "val1", "KEY2": "val2"}
  When session.Execute() creates exec.Cmd
  Then cmd.Env includes both KEY1=val1 and KEY2=val2

Scenario: Env overrides existing var
  Given os.Environ() contains CLAUDE_CODE_EFFORT_LEVEL=low
  And Options.Env = {"CLAUDE_CODE_EFFORT_LEVEL": "high"}
  When session.Execute() creates exec.Cmd
  Then cmd.Env has CLAUDE_CODE_EFFORT_LEVEL=high (last wins)
```

**Technical Notes:**
- `session/session.go`: добавить `Env map[string]string` в Options struct
- `session/session.go`: в Execute(), если len(opts.Env) > 0, установить cmd.Env = append(os.Environ(), envToSlice(opts.Env)...)
- Вспомогательная функция `envToSlice(m map[string]string) []string` — конвертирует map в `KEY=VALUE` слайс
- Zero-value safe: nil Env = nil cmd.Env = inherit parent (без изменения поведения)
- Единственное изменение в пакете session

**Prerequisites:** Нет

---

### Story 9.5: Filepath Normalization + Graceful Degradation

**User Story:**
Как разработчик, я хочу чтобы Ralph корректно работал с путями на Windows, WSL и Linux, а некритические файловые операции не прерывали ран при ошибке.

**Acceptance Criteria:**

```gherkin
Scenario: filepath.Join в knowledge_distill.go (FR70)
  Given knowledge_distill.go constructs paths to LEARNINGS.md
  When path construction code reviewed
  Then all path concatenations use filepath.Join (no string + "/")
  And filepath.Abs used for normalizing input paths where needed

Scenario: filepath.Join в knowledge_write.go (FR70)
  Given knowledge_write.go constructs paths to .ralph/rules/
  When path construction code reviewed
  Then all path concatenations use filepath.Join
  And no manual "/" or "\\" separators in path building

Scenario: filepath.Join в runner.go (FR70)
  Given runner.go constructs paths (review-findings.md, sprint-tasks.md)
  When path construction code reviewed
  Then all remaining string concatenation paths use filepath.Join
  And DetermineReviewOutcome already uses filepath.Join (verify preserved)

Scenario: Graceful degradation for AutoDistill (FR71)
  Given AutoDistill encounters os.ErrNotExist reading LEARNINGS.md
  When error is os.IsNotExist
  Then function returns nil (graceful skip)
  And warning logged: "WARN: <path> not found, skipping"
  And run continues without abort

Scenario: Graceful degradation for WriteLearnings (FR71)
  Given WriteLearnings encounters os.ErrNotExist
  When error is os.IsNotExist
  Then function returns nil (graceful skip)
  And warning logged

Scenario: Real errors still propagated (FR71)
  Given AutoDistill encounters permission error (not os.ErrNotExist)
  When error checked
  Then error propagated to caller (not swallowed)
  And non-NotExist errors are NOT gracefully skipped

Scenario: Cross-platform path test
  Given projectRoot = "/mnt/e/Projects/test"
  When filepath.Join(projectRoot, "review-findings.md") called
  Then result is platform-correct path (no double slashes, correct separator)
```

**Technical Notes:**
- `runner/knowledge_distill.go`: заменить строковые конкатенации путей на filepath.Join, добавить os.IsNotExist guard
- `runner/knowledge_write.go`: аналогично
- `runner/runner.go`: проверить и заменить оставшиеся строковые конкатенации
- Guard pattern: `if errors.Is(err, os.ErrNotExist) { log.Printf("WARN: ..."); return nil }`
- Не затрагивает session/, config/, gates/, bridge/

**Prerequisites:** Нет

---

### Story 9.6: Scope Creep Protection Prompts

**User Story:**
Как разработчик, я хочу чтобы Claude не реализовывал соседние задачи из sprint-tasks.md, а review agent обнаруживал scope creep, чтобы каждый execute цикл фокусировался на одной задаче.

**Acceptance Criteria:**

```gherkin
Scenario: SCOPE BOUNDARY блок в execute.md (FR79)
  Given runner/prompts/execute.md
  When prompt content checked
  Then contains section "## SCOPE BOUNDARY (MANDATORY)"
  And contains "Реализуй ТОЛЬКО текущую задачу: __TASK__"
  And contains "НЕ реализуй другие задачи из sprint-tasks.md"
  And contains instruction проверить перед коммитом
  And contains instruction откатить через git checkout

Scenario: SCOPE BOUNDARY uses __TASK__ placeholder (FR79)
  Given execute.md template with __TASK__ placeholder
  When buildTemplateData() assembles prompt
  Then __TASK__ replaced with actual task text
  And scope boundary references the specific current task

Scenario: Scope compliance в implementation agent (FR80)
  Given runner/prompts/agents/implementation.md
  When prompt content checked
  Then contains scope compliance check instruction
  And contains "Все изменения в diff относятся к AC текущей задачи"
  And contains "Scope creep" as HIGH severity finding format
  And finding format: "Scope creep: изменения в <файл> реализуют задачу '<другая>', а не текущую"

Scenario: Scope check doesn't block unrelated agents (FR80)
  Given other review agents (quality, simplification, design-principles, test-coverage)
  When their prompts checked
  Then they do NOT contain scope creep check instructions
  And only implementation agent performs scope validation

Scenario: Template test coverage
  Given runner/prompt_test.go
  When execute template rendered with task text
  Then output contains "SCOPE BOUNDARY" section
  And output contains the task text in scope boundary context
```

**Technical Notes:**
- `runner/prompts/execute.md`: добавить блок SCOPE BOUNDARY после Commit Rules
- `runner/prompts/agents/implementation.md`: добавить пункт scope compliance check
- Только промпт-изменения, без нового Go-кода
- Тесты: проверить наличие SCOPE BOUNDARY в rendered execute prompt, проверить наличие scope check в implementation agent prompt
- __TASK__ placeholder уже существует в execute.md — scope boundary его переиспользует

**Prerequisites:** Нет

---

### Story 9.7: Pre-flight Check + TaskHash + LogOneline

**User Story:**
Как разработчик, я хочу чтобы Ralph перед запуском execute проверял git log на наличие уже выполненной задачи, чтобы не тратить токены на повторное выполнение.

**Acceptance Criteria:**

```gherkin
Scenario: TaskHash computation (FR67)
  Given task text "Add user validation with tests [GATE]"
  When TaskHash(text) called
  Then returns first 6 hex chars of SHA-256 of task description
  And description = text after "- [ ] " or "- [x] " prefix stripping
  And hash is deterministic (same input → same output)
  And hash is lowercase hex string of length 6

Scenario: TaskHash strips prefix (FR67)
  Given text "- [ ] Add validation"
  When TaskHash called
  Then hashes "Add validation" (stripped "- [ ] ")
  And text "- [x] Add validation" → same hash (same description)
  And text "Add validation" (no prefix) → same hash

Scenario: GitClient.LogOneline (FR68)
  Given GitClient interface
  When LogOneline(n int) added
  Then returns last n commits as one-line strings
  And implementation: git log --oneline -<n>
  And error returned if git not available

Scenario: PreFlightCheck skip (FR68.3)
  Given task with hash "a1b2c3"
  And git log contains commit "feat: add validation [task:a1b2c3]"
  And review-findings.md does not exist
  When PreFlightCheck(git, taskText, projectRoot) called
  Then skip=true, reason contains "commit found, no findings"

Scenario: PreFlightCheck proceed with findings (FR68.4)
  Given task with hash "a1b2c3"
  And git log contains commit with [task:a1b2c3]
  And review-findings.md exists with content
  When PreFlightCheck called
  Then skip=false, reason contains "commit found but findings exist"

Scenario: PreFlightCheck proceed no commit (FR68.5)
  Given task with hash "a1b2c3"
  And git log does NOT contain [task:a1b2c3]
  When PreFlightCheck called
  Then skip=false, reason contains "no matching commit"

Scenario: PreFlightCheck graceful on git error
  Given git log returns error
  When PreFlightCheck called
  Then skip=false (proceed, don't skip)
  And error logged but not propagated (best-effort)

Scenario: Execute.md маркер requirement (FR67)
  Given runner/prompts/execute.md
  When commit rules section checked
  Then contains instruction: add [task:__TASK_HASH__] to commit message
  And __TASK_HASH__ is a new placeholder in buildTemplateData()

Scenario: Integration in Execute() (FR68)
  Given Runner.Execute() processing open tasks
  When pre-flight check returns skip=true
  Then task marked [x] in sprint-tasks.md
  And execute cycle skipped for this task
  And log: "INFO pre-flight skip: <reason>"
  And MetricsCollector records task as skipped
```

**Technical Notes:**
- `runner/preflight.go`: новый файл с TaskHash, PreFlightCheck
- `runner/git.go`: добавить LogOneline к GitClient interface и RealGit
- `runner/runner.go`: вызов PreFlightCheck в Execute() перед циклом, добавить TaskHash в buildTemplateData()
- `runner/prompts/execute.md`: добавить маркер `[task:__TASK_HASH__]` в Commit Rules
- `config/prompt.go`: добавить TaskHash в TemplateData struct
- TaskHash: `crypto/sha256`, `encoding/hex`, первые 6 символов
- Pre-flight = best-effort: ошибка git → proceed (не skip)

**Prerequisites:** Нет

---

### Story 9.8: Smart Merge [x] Preservation

**User Story:**
Как разработчик, я хочу чтобы при перегенерации sprint-tasks.md через bridge статусы `[x]` из предыдущей версии сохранялись, чтобы не терять информацию о выполненных задачах.

**Acceptance Criteria:**

```gherkin
Scenario: SmartMergeStatus preserves [x] (FR69)
  Given old sprint-tasks.md:
    "- [x] Add validation"
    "- [ ] Fix bug"
  And new sprint-tasks.md:
    "- [ ] Add validation"
    "- [ ] Fix bug"
    "- [ ] New task"
  When SmartMergeStatus(oldContent, newContent) called
  Then result:
    "- [x] Add validation"  (preserved from old)
    "- [ ] Fix bug"          (unchanged)
    "- [ ] New task"         (new, stays [ ])

Scenario: SmartMergeStatus matches by hash (FR69)
  Given old task "- [x] Add user validation with tests"
  And new task "- [ ] Add user validation with tests"
  When TaskHash computed for both
  Then hashes match (same description text)
  And [x] status transferred from old to new

Scenario: SmartMergeStatus ignores reordering (FR69)
  Given old:
    "- [x] Task A"
    "- [ ] Task B"
  And new (reordered):
    "- [ ] Task B"
    "- [ ] Task A"
  When SmartMergeStatus called
  Then result:
    "- [ ] Task B"
    "- [x] Task A"  (preserved despite reordering)

Scenario: SmartMergeStatus with removed tasks (FR69)
  Given old: "- [x] Task A", "- [x] Task B"
  And new: "- [ ] Task A" (Task B removed)
  When SmartMergeStatus called
  Then result: "- [x] Task A"
  And Task B silently dropped (not in new)

Scenario: SmartMergeStatus with non-task lines (FR69)
  Given old and new contain headers "## Section" and empty lines
  When SmartMergeStatus called
  Then non-task lines preserved from new content unchanged
  And only task lines with matching hash get [x] transfer

Scenario: SmartMergeStatus with empty old content
  Given old content is empty
  When SmartMergeStatus("", newContent) called
  Then result = newContent (no changes)
```

**Technical Notes:**
- `runner/preflight.go`: добавить SmartMergeStatus (рядом с TaskHash)
- Алгоритм: parse old → build map[hash]bool (done?), iterate new lines → if task line and hash in map and done → replace `[ ]` with `[x]`
- Вызывается из bridge при перегенерации sprint-tasks.md (точка интеграции TBD, может быть отложена если bridge не перегенерирует)
- Использует TaskHash для матчинга
- Не модифицирует описание задачи, только checkbox статус

**Prerequisites:** Story 9.7 (TaskHash)

---

### Story 9.9: Agent Stats in Review Findings

**User Story:**
Как разработчик, я хочу знать какие review sub-agents генерируют замечания, чтобы анализировать эффективность review pipeline и настраивать агентов.

**Acceptance Criteria:**

```gherkin
Scenario: ReviewFinding.Agent field (FR77)
  Given ReviewFinding struct in runner/metrics.go
  When Agent field added: Agent string `json:"agent,omitempty"`
  Then JSON serialization includes agent when non-empty
  And existing findings without Agent field → Agent="" (backward compatible)

Scenario: Agent field в review prompt (FR77)
  Given runner/prompts/review.md
  When findings format checked
  Then format includes 5th field:
    ### [SEVERITY] Finding title
    - **Описание**: ...
    - **Файл**: ...
    - **Строка**: ...
    - **Агент**: <agent_name>

Scenario: Sub-agent prompts include agent name (FR77)
  Given runner/prompts/agents/*.md (5 agents)
  When each agent prompt checked
  Then quality.md → "- **Агент**: quality"
  And implementation.md → "- **Агент**: implementation"
  And simplification.md → "- **Агент**: simplification"
  And design-principles.md → "- **Агент**: design-principles"
  And test-coverage.md → "- **Агент**: test-coverage"

Scenario: DetermineReviewOutcome parses Agent field (FR78)
  Given review-findings.md with Agent field:
    "### [HIGH] Bug found"
    "- **Описание**: desc"
    "- **Файл**: foo.go"
    "- **Строка**: 42"
    "- **Агент**: implementation"
  When DetermineReviewOutcome called
  Then ReviewFinding.Agent == "implementation"

Scenario: DetermineReviewOutcome без Agent field (FR78)
  Given review-findings.md without Agent field (old format):
    "### [HIGH] Bug found"
    "- **Описание**: desc"
  When DetermineReviewOutcome called
  Then ReviewFinding.Agent == "" (empty, not "unknown")

Scenario: AgentFindingStats type (FR78)
  Given runner/metrics.go
  When AgentFindingStats struct defined
  Then has fields: Critical, High, Medium, Low (all int, json tagged)

Scenario: RunMetrics.AgentStats (FR78)
  Given RunMetrics struct
  When AgentStats field added: map[string]*AgentFindingStats
  Then JSON key "agent_stats", omitempty
  And aggregated across all tasks and cycles in the run

Scenario: RecordAgentFinding accumulation (FR78)
  Given MetricsCollector
  When RecordAgentFinding("implementation", "HIGH") called 3 times
  And RecordAgentFinding("quality", "MEDIUM") called 2 times
  Then Finish() returns RunMetrics with:
    AgentStats["implementation"] = {High: 3}
    AgentStats["quality"] = {Medium: 2}

Scenario: Unknown agent handling (FR78)
  Given finding without Agent field (Agent="")
  When RecordAgentFinding("", "HIGH") called
  Then counted under AgentStats["unknown"]

Scenario: Agent regex parsing (FR78)
  Given findingAgentRe regex
  When matched against "- **Агент**: implementation"
  Then captures "implementation"
  And matched against "- **Агент**: design-principles"
  Then captures "design-principles"
```

**Technical Notes:**
- `runner/metrics.go`: добавить Agent field в ReviewFinding, AgentFindingStats struct, AgentStats в RunMetrics, RecordAgentFinding на MetricsCollector
- `runner/runner.go`: добавить findingAgentRe regex, расширить парсинг в DetermineReviewOutcome для извлечения Agent
- `runner/prompts/review.md`: добавить 5-е поле Агент в формат findings
- `runner/prompts/agents/*.md`: каждый agent добавляет `- **Агент**: <name>`
- RecordAgentFinding вызывается из Execute() после парсинга findings
- Backward compatible: отсутствие Agent field → пустая строка → "unknown" в stats

**Prerequisites:** Story 9.1 (SeverityLevel для ParseSeverity)

---

## FR Coverage Matrix

| FR | Описание | Story |
|----|----------|-------|
| FR67 | Маркер `[task:<хэш>]` в commit message | 9.7 |
| FR68 | Pre-flight проверка перед execute | 9.7 |
| FR69 | Smart merge сохранение `[x]` | 9.8 |
| FR70 | filepath.Join/Abs для путей | 9.5 |
| FR71 | Graceful degradation (os.IsNotExist) | 9.5 |
| FR72 | Прогрессивная схема review (6 циклов) | 9.1, 9.3 |
| FR73 | Severity filtering | 9.1, 9.2 |
| FR74 | Scope lock (инкрементальный diff) | 9.3 |
| FR75 | Бюджет замечаний | 9.2, 9.3 |
| FR76 | CLAUDE_CODE_EFFORT_LEVEL=high | 9.3, 9.4 |
| FR77 | Поле `Агент` в findings | 9.9 |
| FR78 | Агрегация agent stats | 9.9 |
| FR79 | SCOPE BOUNDARY в execute.md | 9.6 |
| FR80 | Scope compliance в implementation agent | 9.6 |

**Coverage:** 14/14 FR → 9 stories, 100% coverage.

---

## Summary

**Epic 9: Ralph Run Robustness** — 9 stories, FR67-FR80.

| Story | Область | FR | Приоритет |
|-------|---------|-----|-----------|
| 9.1 | Progressive review types + config | FR72, FR73 | P0 |
| 9.2 | Severity filtering + findings budget | FR73, FR75 | P0 |
| 9.3 | Scope lock + effort escalation | FR72, FR74, FR75, FR76 | P0 |
| 9.4 | Session.Options.Env | FR76 | P0 |
| 9.5 | Filepath normalization + graceful degradation | FR70, FR71 | P1 |
| 9.6 | Scope creep prompts | FR79, FR80 | P1 |
| 9.7 | Pre-flight + TaskHash + LogOneline | FR67, FR68 | P2 |
| 9.8 | Smart merge [x] | FR69 | P2 |
| 9.9 | Agent stats | FR77, FR78 | P3 |

**Recommended execution order:** 9.1 → 9.4 → 9.2 → 9.3 → 9.5 → 9.6 → 9.7 → 9.8 → 9.9
