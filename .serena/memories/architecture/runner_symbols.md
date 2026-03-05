# Runner Package — Symbol Map

## runner/runner.go — Core Loop
- `Runner` struct: main orchestrator with injectable fields (Git, ReviewFn, GatePromptFn, SleepFn, Knowledge, etc.)
- `Runner.Execute()` — public entry, runs full iteration loop
- `Runner.execute()` — single iteration (session + review)
- `Runner.runDistillation()` — triggers knowledge distillation
- `RunConfig` — config passed to Run/RunReview constructors
- `Run(cfg RunConfig) *Runner` — factory for execute mode
- `RunReview(cfg RunConfig) *Runner` — factory for review mode
- `RealReview(cfg, tasksFile, git, knowledge, logger) ReviewResult` — actual review implementation
- `DetermineReviewOutcome(result SessionResult) ReviewResult` — parses review output
- `buildTemplateData(cfg, tasksFile, serenaHint, knowledge) TemplateData` — assembles prompt data
- `InjectFeedback(file, feedback)` — writes gate feedback into tasks file
- `RevertTask/SkipTask` — task manipulation helpers
- `RecoverDirtyState(git, logger)` — recovery on startup
- `ResumeExtraction(cfg, sessionID, logger)` — extracts knowledge from session

## runner/git.go — Git Operations
- `GitClient` interface: HealthCheck, HeadCommit, RestoreClean
- `ExecGitClient` — real implementation using git subprocess
- Sentinels: ErrDirtyTree, ErrDetachedHead, ErrMergeInProgress, ErrNoCommit

## runner/scan.go — Task Scanning
- `ScanTasks(path) (ScanResult, error)` — parses sprint-tasks.md
- `ScanResult` — OpenTasks, DoneTasks slices of TaskEntry
- `TaskEntry` — LineNum, Text, HasGate

## runner/knowledge*.go — Knowledge Management
- `knowledge.go` — KnowledgeWriter interface, NoOpKnowledgeWriter, ProgressData
- `knowledge_write.go` — FileKnowledgeWriter, BudgetCheck, lesson parsing/validation
- `knowledge_read.go` — ValidateLearnings, buildKnowledgeReplacements
- `knowledge_distill.go` — AutoDistill, ParseDistillOutput, WriteDistillOutput, intent files
- `knowledge_state.go` — DistillState, DistillMetrics, LoadDistillState, SaveDistillState

## runner/serena.go — Code Indexer Detection
- `CodeIndexerDetector` interface
- `DetectSerena(projectRoot) CodeIndexerDetector`
- `SerenaMCPDetector` — checks .mcp.json for serena config

## runner/log.go — Structured Logging
- `RunLogger` — file + stderr writer with Info/Warn/Error levels
