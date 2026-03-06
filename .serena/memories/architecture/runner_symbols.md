# Runner Package — Symbol Map

## runner/runner.go — Core Loop
- `Runner` struct: main orchestrator with injectable fields (Git, ReviewFn, GatePromptFn, SleepFn, Knowledge, etc.)
- `Runner.Execute()` — public entry, runs full iteration loop
- `Runner.execute()` — single iteration (session + review)
- `Runner.runDistillation()` — triggers knowledge distillation
- `RunConfig` — config passed to Run/RunReview constructors (Cfg, Git, TasksFile, SerenaHint, Knowledge, Logger)
- `Run(cfg RunConfig) *Runner` — factory for execute mode
- `RunReview(cfg RunConfig) *Runner` — factory for review mode
- `RealReview(cfg, tasksFile, git, knowledge, logger) ReviewResult` — actual review implementation
- `DetermineReviewOutcome(result SessionResult) ReviewResult` — parses review output
- `buildTemplateData(cfg, tasksFile, serenaHint, knowledge) TemplateData` — assembles prompt data
- `InjectFeedback(file, feedback)` — writes gate feedback into tasks file
- `RevertTask/SkipTask` — task manipulation helpers
- `RecoverDirtyState(git, logger)` — recovery on startup
- `ResumeExtraction(ctx, cfg, kw, logger, sessionID)` — extracts knowledge from session, saves session log

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

## runner/serena.go — Code Indexer Detection & Serena Sync
- `CodeIndexerDetector` interface
- `DetectSerena(projectRoot) CodeIndexerDetector`
- `SerenaMCPDetector` — checks .mcp.json for serena config
- `RealSerenaSync(ctx, cfg, opts, logger)` — executes Serena memory sync session, saves session log
- `SerenaSyncOpts` struct, `buildSyncOpts`, `extractCompletedTasks`, `assembleSyncPrompt`

## runner/metrics.go — Observability
- `DiffStats` struct: FilesChanged, Insertions, Deletions, Packages
- `ReviewFinding` struct: parsed review finding (severity, description)
- `LatencyBreakdown` struct: timing per phase (session, git, gate, review, distill)
- `GateStats` struct: gate analytics (prompts, approvals, rejections, skips, wait time)
- `ErrorStats` struct: error categorization (timeout, parse, git, session, config, unknown)
- `TaskMetrics` struct: per-task metrics (tokens, cost, diff, findings, latency, gate, errors)
- `RunMetrics` struct: aggregate run metrics (JSON-serializable)
- `MetricsCollector` struct: nil-safe injectable collector
  - `NewMetricsCollector(runID, pricing)` → constructor
  - `StartTask/FinishTask` → lifecycle pair (MUST be matched on all code paths)
  - `RecordSession/RecordDiff/RecordFindings/RecordGate/RecordError` → incremental recording
  - `Finish() → RunMetrics` → aggregation (call ONCE)
- `PrintRunSummary(w, metrics)` — colored terminal summary

## runner/similarity.go — Duplicate Detection
- `SimilarityDetector` struct: window, warnAt, hardAt, history
- `NewSimilarityDetector(window, warn, hard)` → constructor
- `Check(prompt) (score, action)` — Jaccard similarity check
- `jaccardSimilarity(a, b)` — set intersection / union on whitespace tokens

## runner/log.go — Structured Logging & Session Logs
- `RunLogger` struct: file + stderr writer with Info/Warn/Error levels + session log saving
  - Fields: file, stderr, runID, sessDir (session log directory), sessSeq (sequence counter)
  - `NextSeq()` — returns monotonic session sequence number
  - `SaveSession(type, raw, exitCode, elapsed)` — writes session log file, non-fatal (warns on error)
- `SessionLogInfo` struct: SessionType, Seq, ExitCode, Elapsed
- `SaveSessionLog(sessDir, info, raw)` — writes `<type>-<seq>-<timestamp>.log` with header + stdout + stderr
- `OpenRunLogger(projectRoot, logDir, runID)` — creates logger, sets sessDir to `<logDir>/sessions/<runID>`
- `NopLogger()` — discards all output, sessDir="" (session logging disabled)
- Session log files: `<logDir>/sessions/<runID>/<type>-<seq>-<timestamp>.log`
