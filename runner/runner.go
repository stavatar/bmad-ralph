package runner

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/gates"
	"github.com/bmad-ralph/bmad-ralph/session"
)

// errBudgetSkip is a sentinel for budget emergency gate skip action.
// Callers use errors.Is to detect and set wasSkipped flag.
var errBudgetSkip = errors.New("budget skip")

//go:embed prompts/execute.md
var executeTemplate string

//go:embed prompts/review.md
var reviewTemplate string

//go:embed prompts/agents/quality.md
var agentQualityPrompt string

//go:embed prompts/agents/implementation.md
var agentImplementationPrompt string

//go:embed prompts/agents/simplification.md
var agentSimplificationPrompt string

//go:embed prompts/agents/design-principles.md
var agentDesignPrinciplesPrompt string

//go:embed prompts/agents/test-coverage.md
var agentTestCoveragePrompt string

// resumeExtractionPrompt is the prompt passed via -p to resume-extraction sessions.
// Instructs Claude to extract failure insights and write them to LEARNINGS.md.
const resumeExtractionPrompt = `Extract failure insights from the interrupted session.

Analyze: what was attempted, where the session got stuck, and any extracted insights.

Write findings as atomized facts to LEARNINGS.md using this format:

## category: topic [source, file:line]
Atomized fact content. One insight per entry.

Categories: testing, errors, architecture, performance, tooling, patterns.
Each entry must cite the specific file and line where the issue was observed.
Do NOT remove existing entries — only append new ones at the end of the file.`

// ErrNoCommit indicates that a Claude session completed but HEAD did not change.
// Currently unused in production — Story 3.6 replaced direct return with boolean retry logic.
// Retained as exported sentinel for potential errors.Is detection in future stories.
var ErrNoCommit = errors.New("no commit detected")

// findingSeverityRe parses review findings with severity headers: ### [SEVERITY] Description
var findingSeverityRe = regexp.MustCompile(`(?m)^###\s*\[(\w+)\]\s*(.+)$`)

// ReviewResult holds the outcome of a review step.
type ReviewResult struct {
	Clean          bool                    // true when review found no actionable findings
	Findings       []ReviewFinding         // parsed severity findings from review-findings.md
	SessionMetrics *session.SessionMetrics // token usage from review session (nil if parse failed or ExitError)
	Model          string                  // model used for review session (for cost tracking)
}

// ReviewFunc is the signature for the review step called after each successful execute.
// Production: RealReview (wired by Run). Tests inject custom implementations.
type ReviewFunc func(ctx context.Context, rc RunConfig) (ReviewResult, error)

// GatePromptFunc is the signature for the human gate prompt called after clean review
// when gates are enabled. Fires on [GATE]-tagged tasks (Story 5.2) or checkpoint tasks
// every N completions (Story 5.4). Takes taskText which may be enriched with a checkpoint
// suffix "(checkpoint every N)" — not always raw task text.
// Runner tests inject custom implementations to avoid importing the gates package.
// Production: closure over gates.Prompt (wired by Run).
type GatePromptFunc func(ctx context.Context, taskText string) (*config.GateDecision, error)

// ResumeExtractFunc is the signature for resume-extraction called before retry.
// The sessionID parameter comes from SessionResult.SessionID (empty string if not available).
// Default: closure over ResumeExtraction in Run(). Tests inject custom implementations.
type ResumeExtractFunc func(ctx context.Context, rc RunConfig, sessionID string) error

// selectReviewModel returns the light model if the diff is small enough,
// otherwise the standard review model. If isGate, hydraDetected, or highEffort
// is true, always returns the standard model to ensure thorough review.
// If diffStats is nil (e.g., first review cycle or diff stats failed),
// falls back to the standard model.
func selectReviewModel(cfg *config.Config, ds *DiffStats, isGate bool, hydraDetected bool, highEffort bool) string {
	if isGate || hydraDetected || highEffort || ds == nil || cfg.ModelReviewLight == "" {
		return cfg.ModelReview
	}
	totalLines := ds.Insertions + ds.Deletions
	if ds.FilesChanged <= cfg.ReviewLightMaxFiles && totalLines <= cfg.ReviewLightMaxLines {
		return cfg.ModelReviewLight
	}
	return cfg.ModelReview
}

// RealReview runs a review session and determines the outcome from file state.
// It reads the current task, assembles the review prompt, launches a fresh Claude
// session with the selected review model (standard or light, based on diff size,
// gate flag, and hydra escalation via selectReviewModel), then checks
// sprint-tasks.md and review-findings.md
// to compute ReviewResult. On success, populates SessionMetrics and Model for
// cost tracking (AC5). On ExitError or parse failure, these fields are nil/empty.
// Story 6.4: on non-clean review, post-validates LEARNINGS.md via snapshot-diff
// (rc.Knowledge.ValidateNewLessons). Clean review skips knowledge validation.
// Exported for integration testing (Story 4.8). Production wiring via Run().
//
// Covered by Story 4.8 integration tests (MockClaude + file side effects):
//   - CleanReview: mock session → task [x] + no findings → Clean: true
//   - FindingsFixClean: findings → fix execute → clean review
//   - MaxReviewCycles: emergency stop after max iterations
//   - MultiTaskMixed: 3 tasks with mixed clean/findings outcomes
//   - BridgeGoldenFile: bridge output as runner input end-to-end
//
// Covered by Story 6.4 knowledge tests:
//   - FindingsWriteLessons: ValidateNewLessons called after findings review
//   - CleanNoLessons: ValidateNewLessons NOT called on clean review
//   - SnapshotDiff: snapshot taken before session, diff after
//   - ValidateLessonsError: error propagation from ValidateNewLessons
//
// Not yet covered (future stories or manual testing):
//   - SnapshotReadError: snapshot error path (line 168) is defensive but
//     unreachable — buildKnowledgeReplacements reads same file first
//   - SessionError_ExitError: *exec.ExitError → proceed to file-state check
//   - SessionError_Fatal: non-ExitError → return wrapped error
//   - FreshSession: verify opts has no Resume field
//   - UsesSelectedReviewModel: opts.Model set via selectReviewModel (NOT ModelExecute)
func RealReview(ctx context.Context, rc RunConfig) (ReviewResult, error) {
	content, err := os.ReadFile(rc.TasksFile)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("runner: review: read tasks: %w", err)
	}

	result, scanErr := ScanTasks(string(content))
	if scanErr != nil {
		return ReviewResult{}, scanErr // ScanTasks already wraps with "runner: scan tasks:" prefix
	}
	if !result.HasOpenTasks() {
		return ReviewResult{Clean: true}, nil
	}

	currentTaskText := result.OpenTasks[0].Text

	// Story 6.2: build knowledge for review — override __LEARNINGS_CONTENT__ to empty
	// (self-review loop prevention: review must not see LEARNINGS.md it writes to)
	reviewKnowledge, reviewAppendSys, reviewKnowledgeErr := buildKnowledgeReplacements(rc.Cfg.ProjectRoot)
	if reviewKnowledgeErr != nil {
		return ReviewResult{}, fmt.Errorf("runner: review: build knowledge: %w", reviewKnowledgeErr)
	}
	reviewKnowledge["__LEARNINGS_CONTENT__"] = "" // H7: no self-review of own writes

	reviewReplacements := map[string]string{
		"__TASK_CONTENT__": currentTaskText,
		"__SERENA_HINT__":  rc.SerenaHint,
	}
	// Story 9.3: add previous findings for incremental review context.
	if rc.IncrementalDiff {
		reviewReplacements["__PREV_FINDINGS__"] = rc.PrevFindings
	}
	for k, v := range reviewKnowledge {
		reviewReplacements[k] = v
	}

	// Story 9.3: build template data with progressive review fields.
	td := buildTemplateData(rc.Cfg, rc.SerenaHint, false, false)
	td.IncrementalDiff = rc.IncrementalDiff
	td.Cycle = rc.Cycle
	td.MinSeverityLabel = rc.MinSeverity.String()
	td.MaxFindings = rc.MaxFindings

	prompt, err := config.AssemblePrompt(
		reviewTemplate,
		td,
		reviewReplacements,
	)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("runner: review: assemble prompt: %w", err)
	}

	// DESIGN-4: select review model based on diff size (without mutating Config)
	// GATE tasks, hydra escalation, and high-effort cycles always use standard model.
	reviewModel := selectReviewModel(rc.Cfg, rc.LastDiffStats, rc.IsGate, rc.HydraDetected, rc.HighEffort)

	opts := session.Options{
		Command:                    rc.Cfg.ClaudeCommand,
		Dir:                        rc.Cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   rc.Cfg.MaxTurns,
		Model:                      reviewModel,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
		AppendSystemPrompt:         reviewAppendSys,
	}

	// Story 6.4: snapshot LEARNINGS.md before review session for post-validation diff
	learningsPath := filepath.Join(rc.Cfg.ProjectRoot, "LEARNINGS.md")
	learningsSnapshot := ""
	if snapshotData, snapErr := os.ReadFile(learningsPath); snapErr == nil {
		learningsSnapshot = string(snapshotData)
	} else if !errors.Is(snapErr, os.ErrNotExist) {
		return ReviewResult{}, fmt.Errorf("runner: review: snapshot: %w", snapErr)
	}

	start := time.Now()
	raw, execErr := session.Execute(ctx, opts)
	elapsed := time.Since(start)
	if rc.Logger != nil {
		rc.Logger.SaveSession("review", raw, raw.ExitCode, elapsed)
	}

	var reviewMetrics *session.SessionMetrics
	var resolvedModel string
	if execErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(execErr, &exitErr) {
			return ReviewResult{}, fmt.Errorf("runner: review: execute: %w", execErr)
		}
		// ExitError: proceed to file-state check — review may have partially written
	} else {
		// Parse for session metrics + model (cost tracking, AC5).
		// Error intentionally ignored: review outcome is determined by file state,
		// not by session output parsing.
		if sr, parseErr := session.ParseResult(raw, elapsed); parseErr == nil {
			reviewMetrics = sr.Metrics
			resolvedModel = sr.Model
		}
	}

	outcome, outcomeErr := DetermineReviewOutcome(rc.TasksFile, currentTaskText, rc.Cfg.ProjectRoot)
	if outcomeErr != nil {
		return ReviewResult{}, outcomeErr
	}
	outcome.SessionMetrics = reviewMetrics
	outcome.Model = resolvedModel

	// Story 6.4: post-validate lessons on non-clean review (FR28a)
	// Clean review = no findings = no lessons to validate.
	if !outcome.Clean && rc.Knowledge != nil {
		if err := rc.Knowledge.ValidateNewLessons(ctx, LessonsData{
			Source:      "review",
			Snapshot:    learningsSnapshot,
			BudgetLimit: rc.Cfg.LearningsBudget,
		}); err != nil {
			return ReviewResult{}, fmt.Errorf("runner: review: validate lessons: %w", err)
		}
	}

	return outcome, nil
}

// DetermineReviewOutcome computes ReviewResult from file state after a review session.
// It checks two conditions for Clean:
//  1. Current task marked [x] in sprint-tasks.md (re-read after review)
//  2. review-findings.md absent or empty (whitespace-only counts as empty)
//
// Clean = taskMarkedDone AND (findingsAbsent OR findingsEmpty).
// If task not marked done but no findings: NOT clean (review session may have failed).
//
// When findings file has content, parses severity headers via findingSeverityRe
// (### [SEVERITY] Description) into ReviewResult.Findings. Returns nil Findings
// when clean or when content has no parseable severity headers.
func DetermineReviewOutcome(tasksFile, currentTaskText, projectRoot string) (ReviewResult, error) {
	content, err := os.ReadFile(tasksFile)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("runner: determine review outcome: %w", err)
	}

	desc := taskDescription(currentTaskText)
	taskMarkedDone := false
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, desc) && config.TaskDoneRegex.MatchString(line) {
			taskMarkedDone = true
			break
		}
	}

	findingsPath := filepath.Join(projectRoot, "review-findings.md")
	findingsData, findingsErr := os.ReadFile(findingsPath)
	if findingsErr != nil && !errors.Is(findingsErr, os.ErrNotExist) {
		return ReviewResult{}, fmt.Errorf("runner: determine review outcome: %w", findingsErr)
	}
	findingsNonEmpty := findingsErr == nil && len(strings.TrimSpace(string(findingsData))) > 0

	clean := taskMarkedDone && !findingsNonEmpty

	// Parse severity findings when findings file has content and review is not clean.
	var findings []ReviewFinding
	if findingsNonEmpty {
		matches := findingSeverityRe.FindAllStringSubmatch(string(findingsData), -1)
		for _, m := range matches {
			findings = append(findings, ReviewFinding{
				Severity:    m[1],
				Description: strings.TrimSpace(m[2]),
			})
		}
	}

	return ReviewResult{Clean: clean, Findings: findings}, nil
}

// taskDescription extracts the description from a task line, stripping the
// checkbox prefix ("- [ ] " or "- [x] ") and leading whitespace.
func taskDescription(taskLine string) string {
	trimmed := strings.TrimSpace(taskLine)
	if idx := strings.Index(trimmed, "] "); idx >= 0 {
		return strings.TrimSpace(trimmed[idx+2:])
	}
	return trimmed
}

// checkBudget checks cumulative cost against BudgetMaxUSD after each RecordSession.
// Returns nil if budget is disabled, under threshold, or user chose to continue (retry).
// Returns errBudgetSkip if user chose to skip (SkipTask already called).
// Returns other error for quit, gate errors, or hard stop (no gates).
// budgetWarned is a per-task flag; when set, warning is not repeated.
func (r *Runner) checkBudget(ctx context.Context, taskText string, budgetWarned *bool) error {
	if r.Cfg.BudgetMaxUSD <= 0 || r.Metrics == nil {
		return nil
	}
	log := r.logger()
	cumCost := r.Metrics.CumulativeCost()
	budgetMax := r.Cfg.BudgetMaxUSD
	warnAt := budgetMax * float64(r.Cfg.BudgetWarnPct) / 100

	if cumCost >= budgetMax {
		log.Error("budget exceeded",
			"cost", fmt.Sprintf("%.2f", cumCost),
			"budget", fmt.Sprintf("%.2f", budgetMax),
		)
		if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
			emergencyText := fmt.Sprintf("budget exceeded ($%.2f/$%.2f) for %q", cumCost, budgetMax, taskText)
			decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
			if gateErr != nil {
				return fmt.Errorf("runner: budget gate: %w", gateErr)
			}
			if decision.Action == config.ActionQuit {
				return fmt.Errorf("runner: budget exceeded ($%.2f/$%.2f): %w", cumCost, budgetMax, decision)
			}
			if decision.Action == config.ActionSkip {
				taskDesc := taskDescription(taskText)
				if err := SkipTask(r.TasksFile, taskDesc); err != nil {
					return err
				}
				return errBudgetSkip
			}
			// retry: continue execution despite exceeded budget
			return nil
		}
		return fmt.Errorf("runner: budget exceeded ($%.2f/$%.2f): cost limit reached", cumCost, budgetMax)
	}

	if cumCost >= warnAt && !*budgetWarned {
		*budgetWarned = true
		log.Warn("budget warning",
			"cost", fmt.Sprintf("%.2f", cumCost),
			"warn_at", fmt.Sprintf("%.2f", warnAt),
			"budget", fmt.Sprintf("%.2f", budgetMax),
		)
		taskDesc := taskDescription(taskText)
		msg := fmt.Sprintf("Budget warning: $%.2f of $%.2f (%.0f%%). Consider wrapping up.", cumCost, budgetMax, cumCost/budgetMax*100)
		if err := InjectFeedback(r.TasksFile, taskDesc, msg); err != nil {
			log.Warn("budget feedback injection failed", "error", err.Error())
		}
	}
	return nil
}

// checkTaskBudget checks current task cost against TaskBudgetMaxUSD after each RecordSession.
// Returns nil if task budget is disabled (0), under threshold, or user chose to continue (retry).
// Returns errBudgetSkip if user chose to skip or no gates available (SkipTask already called).
// Returns other error for quit or gate errors.
func (r *Runner) checkTaskBudget(ctx context.Context, taskText string) error {
	if r.Cfg.TaskBudgetMaxUSD <= 0 || r.Metrics == nil {
		return nil
	}
	taskCost := r.Metrics.CurrentTaskCost()
	taskMax := r.Cfg.TaskBudgetMaxUSD
	if taskCost < taskMax {
		return nil
	}
	log := r.logger()
	log.Error("task budget exceeded",
		"task", taskText,
		"cost", fmt.Sprintf("%.2f", taskCost),
		"task_budget", fmt.Sprintf("%.2f", taskMax),
	)
	if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
		emergencyText := fmt.Sprintf("task budget exceeded ($%.2f/$%.2f) for %q", taskCost, taskMax, taskText)
		decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
		if gateErr != nil {
			return fmt.Errorf("runner: task budget gate: %w", gateErr)
		}
		if decision.Action == config.ActionQuit {
			return fmt.Errorf("runner: task budget exceeded ($%.2f/$%.2f): %w", taskCost, taskMax, decision)
		}
		if decision.Action == config.ActionSkip {
			taskDesc := taskDescription(taskText)
			if err := SkipTask(r.TasksFile, taskDesc); err != nil {
				return err
			}
			return errBudgetSkip
		}
		// retry: continue execution despite exceeded task budget
		return nil
	}
	// No gates: auto-skip the task
	taskDesc := taskDescription(taskText)
	if err := SkipTask(r.TasksFile, taskDesc); err != nil {
		return err
	}
	return errBudgetSkip
}

// recordGateDecision logs a structured "gate decision" event and records GateStats
// in the MetricsCollector. Called from all 4 gate locations (execute emergency,
// review emergency, normal gate, distillation gate).
func (r *Runner) recordGateDecision(action string, waitMs int64, taskText string) {
	log := r.logger()
	log.Info("gate decision",
		"step_type", "gate",
		"action", action,
		"wait_ms", fmt.Sprintf("%d", waitMs),
		"task", taskText,
	)
	if r.Metrics != nil {
		stats := GateStats{
			TotalPrompts: 1,
			TotalWaitMs:  waitMs,
			LastAction:   action,
		}
		switch action {
		case config.ActionApprove:
			stats.Approvals = 1
		case config.ActionQuit:
			stats.Rejections = 1
		case config.ActionSkip:
			stats.Skips = 1
		}
		r.Metrics.RecordGate(stats)
	}
}

// ResumeExtraction invokes claude --resume with -p extraction prompt to capture
// WIP progress and failure insights from an interrupted execute session.
// Returns nil when sessionID is empty (nothing to resume).
// When mc is non-nil, merges resume session metrics (tokens, cost) into the current task (BUG-11).
// After session: snapshot-diffs LEARNINGS.md and validates new entries via KnowledgeWriter.
func ResumeExtraction(ctx context.Context, cfg *config.Config, kw KnowledgeWriter, logger *RunLogger, mc *MetricsCollector, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	// Snapshot LEARNINGS.md before session for post-validation diff
	learningsPath := filepath.Join(cfg.ProjectRoot, "LEARNINGS.md")
	snapshot, readErr := os.ReadFile(learningsPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("runner: resume extraction: snapshot: %w", readErr)
	}

	opts := session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Resume:                     sessionID,
		Prompt:                     resumeExtractionPrompt,
		MaxTurns:                   cfg.MaxTurns,
		Model:                      cfg.ModelExecute,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	}

	start := time.Now()
	raw, execErr := session.Execute(ctx, opts)
	elapsed := time.Since(start)
	if logger != nil {
		logger.SaveSession("resume", raw, raw.ExitCode, elapsed)
	}

	if execErr != nil {
		return fmt.Errorf("runner: resume extraction: execute: %w", execErr)
	}

	sr, parseErr := session.ParseResult(raw, elapsed)
	if parseErr != nil {
		return fmt.Errorf("runner: resume extraction: parse: %w", parseErr)
	}

	// BUG-11: merge resume session metrics into the current task
	if mc != nil {
		mc.RecordSession(sr.Metrics, cfg.ModelExecute, "resume", elapsed.Milliseconds())
	}

	if err := kw.WriteProgress(ctx, ProgressData{SessionID: sr.SessionID}); err != nil {
		return fmt.Errorf("runner: resume extraction: write progress: %w", err)
	}

	// Post-validate new LEARNINGS.md entries via snapshot-diff
	if err := kw.ValidateNewLessons(ctx, LessonsData{
		Source:      "resume-extraction",
		Snapshot:    string(snapshot),
		BudgetLimit: cfg.LearningsBudget,
	}); err != nil {
		return fmt.Errorf("runner: resume extraction: validate lessons: %w", err)
	}

	return nil
}

// InjectFeedback appends a user feedback line into tasksFile under the task
// matching taskDesc. The feedback line is indented with 2 spaces and prefixed
// with config.FeedbackPrefix. Existing indented lines (previous feedback) are
// preserved — the new line is inserted after them.
// Error wrapping: "runner: inject feedback:" prefix for all error returns.
func InjectFeedback(tasksFile, taskDesc, feedback string) error {
	content, err := os.ReadFile(tasksFile)
	if err != nil {
		return fmt.Errorf("runner: inject feedback: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	taskIdx := -1
	for i, line := range lines {
		if strings.Contains(line, taskDesc) {
			taskIdx = i
			break
		}
	}
	if taskIdx < 0 {
		return fmt.Errorf("runner: inject feedback: task not found: %s", taskDesc)
	}

	// Find insertion point: after task line + consecutive indented lines
	insertIdx := taskIdx + 1
	for insertIdx < len(lines) && len(lines[insertIdx]) > 0 && (lines[insertIdx][0] == ' ' || lines[insertIdx][0] == '\t') {
		insertIdx++
	}

	feedbackLine := "  " + config.FeedbackPrefix + " " + feedback
	// Insert at insertIdx
	lines = append(lines[:insertIdx], append([]string{feedbackLine}, lines[insertIdx:]...)...)

	return os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")), 0644)
}

// RevertTask changes a completed task ([x]) back to open ([ ]) in tasksFile.
// The task must match taskDesc AND be marked done (config.TaskDoneRegex).
// Error wrapping: "runner: revert task:" prefix for all error returns.
func RevertTask(tasksFile, taskDesc string) error {
	content, err := os.ReadFile(tasksFile)
	if err != nil {
		return fmt.Errorf("runner: revert task: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, taskDesc) && config.TaskDoneRegex.MatchString(line) {
			lines[i] = strings.Replace(line, config.TaskDone, config.TaskOpen, 1)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("runner: revert task: task not found: %s", taskDesc)
	}

	return os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")), 0644)
}

// SkipTask changes an open task ([ ]) to done ([x]) in tasksFile.
// The task must match taskDesc AND be marked open (config.TaskOpenRegex).
// Error wrapping: "runner: skip task:" prefix for all error returns.
func SkipTask(tasksFile, taskDesc string) error {
	content, err := os.ReadFile(tasksFile)
	if err != nil {
		return fmt.Errorf("runner: skip task: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, taskDesc) && config.TaskOpenRegex.MatchString(line) {
			lines[i] = strings.Replace(line, config.TaskOpen, config.TaskDone, 1)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("runner: skip task: task not found: %s", taskDesc)
	}

	return os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")), 0644)
}

// buildTemplateData creates config.TemplateData consistently across all call sites.
// Centralizes field assignment so new template fields are added in one place.
func buildTemplateData(cfg *config.Config, serenaHint string, hasFindings, hasLearnings bool) config.TemplateData {
	return config.TemplateData{
		GatesEnabled:  cfg.GatesEnabled,
		SerenaEnabled: serenaHint != "",
		HasFindings:   hasFindings,
		HasLearnings:  hasLearnings,
	}
}

// RunConfig passes dependencies to runner functions.
type RunConfig struct {
	Cfg           *config.Config
	Git           GitClient
	TasksFile     string          // path to sprint-tasks.md
	SerenaHint    string          // Serena MCP prompt hint (empty if unavailable)
	Knowledge     KnowledgeWriter // records execution progress and validates lessons (nil = skip)
	Logger        *RunLogger      // session log writer (nil = skip session logging)
	LastDiffStats  *DiffStats // diff stats from last execute cycle (DESIGN-4: review model routing)
	IsGate         bool       // true when current task has [GATE] tag (DESIGN-4: forces standard review model)
	HydraDetected  bool       // true when hydra pattern detected (DESIGN-4: escalates to standard review model)
	// Story 9.3: progressive review fields (FR72-FR76)
	Cycle           int           // current review cycle (1-based)
	MinSeverity     SeverityLevel // minimum severity threshold for this cycle
	MaxFindings     int           // findings budget for this cycle
	IncrementalDiff bool          // true when review should use incremental diff (cycle 3+)
	HighEffort      bool          // true when extended thinking should be used (cycle 3+)
	PrevFindings    string        // previous cycle findings text for incremental review context
}

// Runner orchestrates the execute-review loop with injectable dependencies.
// Public API: Run() creates a Runner internally. Tests construct Runner directly.
// EmergencyGatePromptFn: called at execute/review exhaustion when gates enabled.
// Uses same GatePromptFunc type as GatePromptFn but creates Gate{Emergency: true}.
// DistillFn: called after clean review when budget check and cooldown pass (Story 6.5a).
// SerenaSyncFn: called after execute loop when sync enabled and Serena available. Nil = no sync capability.
// Logger: structured log writer; nil falls back to NopLogger (no-op, no file I/O).
type Runner struct {
	Cfg                   *config.Config
	Git                   GitClient
	TasksFile             string              // path to sprint-tasks.md
	ReviewFn              ReviewFunc          // called after each successful execute with commit
	GatePromptFn          GatePromptFunc      // called after clean review on [GATE] or checkpoint tasks when gates enabled
	EmergencyGatePromptFn GatePromptFunc      // called at execute/review exhaustion when gates enabled (Story 5.5)
	ResumeExtractFn       ResumeExtractFunc   // called before retry to extract session context
	DistillFn             DistillFunc         // called after clean review when budget+cooldown checks pass (Story 6.5a)
	SerenaSyncFn          func(ctx context.Context, opts SerenaSyncOpts) (*session.SessionResult, error) // called after execute loop when sync enabled and Serena available
	SleepFn               func(time.Duration) // injectable sleep for testable backoff
	Knowledge             KnowledgeWriter     // records execution progress and validates lessons
	CodeIndexer           CodeIndexerDetector // detects code indexing tools (Serena MCP)
	Logger                *RunLogger          // structured log writer; nil = NopLogger
	Metrics               *MetricsCollector   // accumulates session metrics; nil = no collection
	Similarity            *SimilarityDetector // detects repeating diff patterns; nil = disabled (Story 7.8)
}

// logger returns the Runner's logger, falling back to NopLogger when Logger is nil.
// All Execute internals call logger() rather than accessing Logger directly so that
// tests that do not set Logger never see a nil-pointer panic.
func (r *Runner) logger() *RunLogger {
	if r.Logger == nil {
		return NopLogger()
	}
	return r.Logger
}

// Execute runs the main task loop: startup recovery, then iterate over tasks.
// Startup: recovers dirty working tree (RecoverDirtyState), non-dirty health errors abort.
// Each iteration: read tasks → scan → review cycle loop
// (read findings → assemble prompt → execute retry → review → check Clean,
// stop at MaxReviewIterations (FR24)).
// Prompt is assembled inside the review cycle loop so each execute iteration
// gets fresh findings content from review-findings.md.
// Execute retry: session.Execute → check commit → retry on no-commit or non-zero
// exit up to Cfg.MaxIterations per review cycle.
// Stuck detection (Story 7.5): tracks consecutive no-commit attempts across tasks.
// When consecutiveNoCommit >= StuckThreshold (and threshold > 0), injects feedback
// via InjectFeedback and logs warning. Counter resets to 0 on successful commit.
// StuckThreshold == 0 disables detection. Stuck does NOT terminate the loop.
// At exhaustion points (execute attempts or review cycles), when GatesEnabled and
// EmergencyGatePromptFn is set, shows emergency gate instead of returning error.
// Emergency gate: quit returns wrapped GateDecision (exit code 2), skip marks task
// done via SkipTask and proceeds to next task (bypasses completedTasks increment
// and normal gate check), retry resets counter and continues.
// After clean review (not emergency skip), increments completedTasks counter and checks gate triggers:
// [GATE] tag (Story 5.2) or checkpoint every N tasks (Story 5.4, GatesCheckpoint > 0).
// When GatesEnabled and either trigger fires (OR logic) and GatePromptFn is set,
// calls GatePromptFn with enriched text (checkpoint appends "(checkpoint every N)").
// Quit returns wrapped GateDecision error (exit code 2).
// Retry decrements completedTasks, injects feedback into sprint-tasks.md, reverts
// task [x]→[ ], and continues the outer loop. Retries count toward MaxIterations.
// After gate: increments MonotonicTaskCounter in DistillState, checks budget (NearLimit)
// and cooldown (counter - lastDistill >= DistillCooldown). If both pass, calls DistillFn.
// Distillation failures are non-fatal: ErrBadFormat gets one free retry, then human gate.
// Loops up to Cfg.MaxIterations task-processing cycles in the outer loop.
// After the execute loop (before Metrics.Finish), runs Serena memory sync when
// SerenaSyncEnabled && trigger == "run" && SerenaSyncFn != nil && CodeIndexer.Available.
// Sync is best-effort: failure does not affect runErr.
// Structured progress is written to Logger (file + stderr) at every key decision point.
// Returns RunMetrics (nil on early error before collection starts) and error.
// Returns nil error when all tasks are complete. Returns error on any failure.
func (r *Runner) Execute(ctx context.Context) (*RunMetrics, error) {
	log := r.logger()
	tasksBase := filepath.Base(r.TasksFile)
	log.Info("run started", "tasks_file", tasksBase)

	// Capture initial HEAD SHA for diff summary in sync (best-effort).
	// Only call when batch sync is active to avoid consuming mock HeadCommit slots.
	var initialCommit string
	if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "run" {
		initialCommit, _ = r.Git.HeadCommit(ctx)
	}

	runErr := r.execute(ctx)
	if runErr != nil {
		log.Error("run finished", "status", "error", "error", runErr.Error())
	} else {
		log.Info("run finished", "status", "ok")
	}

	// Serena sync: batch mode only when trigger == "run" (AC #2, #3, #4).
	if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "run" &&
		r.SerenaSyncFn != nil && r.CodeIndexer != nil &&
		r.CodeIndexer.Available(r.Cfg.ProjectRoot) {
		r.runSerenaSync(ctx, initialCommit, "")
	} else if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "run" &&
		(r.CodeIndexer == nil || !r.CodeIndexer.Available(r.Cfg.ProjectRoot)) {
		log.Info("serena sync skipped", "reason", "serena not available")
	}

	if r.Metrics != nil {
		rm := r.Metrics.Finish()
		return &rm, runErr
	}
	return nil, runErr
}

// execute is the internal implementation of Execute.
// Execute wraps this with run-level log lines; execute contains the actual loop.
func (r *Runner) execute(ctx context.Context) error {
	log := r.logger()

	// M7: Recover interrupted distillation BEFORE git state recovery
	if err := RecoverDistillation(r.Cfg.ProjectRoot); err != nil {
		return fmt.Errorf("runner: startup: %w", err)
	}

	recovered, err := RecoverDirtyState(ctx, r.Git)
	if err != nil {
		return fmt.Errorf("runner: startup: %w", err)
	}
	if recovered {
		log.Warn("dirty state detected", "recovering", "true")
		log.Info("dirty state recovered")
	}

	// Story 6.7: Serena MCP detection at startup
	serenaHint := ""
	if r.CodeIndexer != nil {
		serenaHint = DetectSerena(r.CodeIndexer, r.Cfg.ProjectRoot)
	}

	// Story 6.2: build knowledge replacements for prompt injection
	knowledgeReplacements, appendSysPrompt, knowledgeErr := buildKnowledgeReplacements(r.Cfg.ProjectRoot)
	if knowledgeErr != nil {
		return fmt.Errorf("runner: startup: %w", knowledgeErr)
	}
	hasLearnings := knowledgeReplacements["__LEARNINGS_CONTENT__"] != ""

	// Story 6.2: budget warning (M6)
	if hasLearnings {
		learningsPathForBudget := filepath.Join(r.Cfg.ProjectRoot, "LEARNINGS.md")
		budgetStatus, budgetErr := BudgetCheck(ctx, learningsPathForBudget, r.Cfg.LearningsBudget)
		if budgetErr == nil {
			log.Info("learnings budget check",
				"lines", fmt.Sprintf("%d/%d", budgetStatus.Lines, budgetStatus.Limit),
				"near_limit", fmt.Sprintf("%v", budgetStatus.NearLimit),
			)
			if budgetStatus.OverBudget {
				ratio := 0
				if budgetStatus.Limit > 0 {
					ratio = budgetStatus.Lines / budgetStatus.Limit
				}
				fmt.Fprintf(os.Stderr, "⚠ LEARNINGS.md: %d/%d lines (%dx budget). Run `ralph distill` to compress.\n",
					budgetStatus.Lines, budgetStatus.Limit, ratio)
			}
		}
	}

	rc := RunConfig{
		Cfg:        r.Cfg,
		Git:        r.Git,
		TasksFile:  r.TasksFile,
		SerenaHint: serenaHint,
		Knowledge:  r.Knowledge,
		Logger:     r.Logger,
	}

	// Story 6.1: snapshot LEARNINGS.md before task execution for post-validation diff
	learningsPath := filepath.Join(r.Cfg.ProjectRoot, "LEARNINGS.md")
	learningsSnapshot := ""
	if snapshotData, snapErr := os.ReadFile(learningsPath); snapErr == nil {
		learningsSnapshot = string(snapshotData)
	}

	// Story 6.5a: load distillation state for budget check + cooldown
	distillStatePath := filepath.Join(r.Cfg.ProjectRoot, ".ralph", "distill-state.json")
	distillState, distillLoadErr := LoadDistillState(distillStatePath)
	if distillLoadErr != nil {
		return fmt.Errorf("runner: startup: %w", distillLoadErr)
	}

	// completedTasks: in-memory counter for gate checkpoint logic (resets each Execute call).
	// Distinct from distillState.MonotonicTaskCounter which persists across runs for cooldown.
	completedTasks := 0
	// consecutiveNoCommit: cross-task counter for stuck detection (Story 7.5).
	// Increments on no-commit, resets to 0 ONLY on successful commit. Not reset between tasks.
	consecutiveNoCommit := 0
	taskStart := time.Now()
	for i := 0; i < r.Cfg.MaxIterations; i++ {
		content, err := os.ReadFile(r.TasksFile)
		if err != nil {
			return fmt.Errorf("runner: read tasks: %w", err)
		}

		result, scanErr := ScanTasks(string(content))
		if scanErr != nil {
			return scanErr // ScanTasks already wraps with "runner: scan tasks:" prefix
		}
		if !result.HasOpenTasks() {
			return nil // Execute() wrapper logs "run finished" on return
		}

		taskText := result.OpenTasks[0].Text
		taskStart = time.Now()
		budgetWarned := false  // Story 7.7: budget warning logged once per task
		hydraDetected := false // DESIGN-4: hydra escalation flag, reset per task (AC2)
		if r.Metrics != nil {
			r.Metrics.StartTask(taskText)
		}
		log.Info("task started",
			"attempt", fmt.Sprintf("%d/%d", i+1, r.Cfg.MaxIterations),
			"task", taskText,
		)

		// Story 9.7: pre-flight check — skip task if commit with [task:<hash>] found and no findings.
		if skip, reason := PreFlightCheck(ctx, r.Git, taskText, r.Cfg.ProjectRoot); skip {
			log.Info("pre-flight skip", "task", taskText, "reason", reason)
			taskDesc := taskDescription(taskText)
			if err := SkipTask(r.TasksFile, taskDesc); err != nil {
				return fmt.Errorf("runner: pre-flight skip task: %w", err)
			}
			if r.Metrics != nil {
				r.Metrics.FinishTask("skipped", "")
			}
			continue
		}

		// Story 8.6: capture HEAD before task for per-task diff scope.
		// Only call when per-task sync is active to avoid consuming mock HeadCommit slots.
		var taskHeadBefore string
		if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "task" &&
			r.SerenaSyncFn != nil && r.CodeIndexer != nil &&
			r.CodeIndexer.Available(r.Cfg.ProjectRoot) {
			taskHeadBefore, _ = r.Git.HeadCommit(ctx)
		}

		// Review cycle loop: per-task counter, resets when clean (AC3, AC4)
		reviewCycles := 0
		prevFindingsText := "" // Story 9.3: previous cycle findings for incremental review context
		wasSkipped := false          // Story 5.5: emergency skip exits without completedTasks++ or gate check
		reviewExhausted := false     // BUG-2: review exhaust continues to next task instead of aborting
		lastKnownSHA := ""           // Track last HEAD SHA for FinishTask on error paths (MINOR-2)
		var lastDiffStats *DiffStats // DESIGN-4: last execute diff stats for review model selection
		for {
			cycleStart := time.Now() // MINOR-7: track full cycle duration

			// Story 9.3: compute progressive params for this cycle (FR72-FR76).
			progParams := ProgressiveParams(reviewCycles+1, r.Cfg.MaxReviewIterations)

			// Read findings file: absent = empty (normal first-execute case)
			findingsContent := ""
			findingsPath := filepath.Join(r.Cfg.ProjectRoot, "review-findings.md")
			findingsData, findingsErr := os.ReadFile(findingsPath)
			if findingsErr != nil && !errors.Is(findingsErr, os.ErrNotExist) {
				return fmt.Errorf("runner: read findings: %w", findingsErr)
			}
			if findingsErr == nil {
				findingsContent = string(findingsData)
			}

			executeReplacements := map[string]string{
				"__FORMAT_CONTRACT__":  config.SprintTasksFormat(),
				"__FINDINGS_CONTENT__": findingsContent,
				"__SERENA_HINT__":      serenaHint,
				"__TASK_CONTENT__":     taskText,
				"__TASK_HASH__":        TaskHash(taskText),
			}
			for k, v := range knowledgeReplacements {
				executeReplacements[k] = v
			}

			prompt, err := config.AssemblePrompt(
				executeTemplate,
				buildTemplateData(r.Cfg, serenaHint, len(strings.TrimSpace(findingsContent)) > 0, hasLearnings),
				executeReplacements,
			)
			if err != nil {
				return fmt.Errorf("runner: assemble prompt: %w", err)
			}

			// Story 9.3: effort escalation via CLAUDE_CODE_EFFORT_LEVEL env (FR76).
			var execEnv map[string]string
			if progParams.HighEffort {
				execEnv = map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "high"}
			}

			opts := session.Options{
				Command:                    r.Cfg.ClaudeCommand,
				Dir:                        r.Cfg.ProjectRoot,
				Prompt:                     prompt,
				MaxTurns:                   r.Cfg.MaxTurns,
				Model:                      r.Cfg.ModelExecute,
				OutputJSON:                 true,
				DangerouslySkipPermissions: true,
				AppendSystemPrompt:         appendSysPrompt,
				Env:                        execEnv,
			}
			// Per-review-cycle retry loop: executeAttempts resets each cycle
			executeAttempts := 0
			skipTask := false // Story 5.5: set by emergency gate skip to break out of review cycle loop
			for {
				gitT0 := time.Now()
				headBefore, err := r.Git.HeadCommit(ctx)
				if r.Metrics != nil {
					r.Metrics.RecordLatency(LatencyBreakdown{GitMs: time.Since(gitT0).Milliseconds()})
				}
				if err != nil {
					if r.Metrics != nil {
						r.Metrics.RecordError(CategorizeError(err), err.Error())
					}
					return fmt.Errorf("runner: head commit before: %w", err)
				}

				log.Info("execute session started",
					"task", taskText,
					"review_cycle", fmt.Sprintf("%d", reviewCycles+1),
					"execute_attempt", fmt.Sprintf("%d", executeAttempts+1),
				)

				start := time.Now()
				raw, execErr := session.Execute(ctx, opts)
				elapsed := time.Since(start)
				if r.Logger != nil {
					r.Logger.SaveSession("execute", raw, raw.ExitCode, elapsed)
				}
				if r.Metrics != nil {
					r.Metrics.RecordLatency(LatencyBreakdown{SessionMs: elapsed.Milliseconds()})
				}

				needsRetry := false
				var sessionID string

				if execErr != nil {
					if r.Metrics != nil {
						r.Metrics.RecordError(CategorizeError(execErr), execErr.Error())
					}
					// Distinguish retryable (exit error) from fatal (binary not found, ctx cancel)
					var exitErr *exec.ExitError
					if errors.As(execErr, &exitErr) {
						log.Info("execute session finished",
							"duration", fmt.Sprintf("%ds", int(elapsed.Seconds())),
							"exit", fmt.Sprintf("%d", exitErr.ExitCode()),
						)
						needsRetry = true // AC6: non-zero exit triggers retry
						// Try to parse for sessionID and metrics despite error
						if sr, parseErr := session.ParseResult(raw, elapsed); parseErr == nil {
							sessionID = sr.SessionID
							if r.Metrics != nil {
								resolved := r.Metrics.RecordSession(sr.Metrics, sr.Model, "execute", elapsed.Milliseconds())
								if resolved != sr.Model {
									log.Warn("unknown model pricing", "model", sr.Model, "fallback", resolved)
								}
							}
						}
						// Story 7.7: budget check after execute session (error path)
						if budgetErr := r.checkBudget(ctx, taskText, &budgetWarned); budgetErr != nil {
							if errors.Is(budgetErr, errBudgetSkip) {
								skipTask = true
								wasSkipped = true
								break
							}
							return budgetErr
						}
						// DESIGN-4: per-task budget check after execute session (error path)
						if budgetErr := r.checkTaskBudget(ctx, taskText); budgetErr != nil {
							if errors.Is(budgetErr, errBudgetSkip) {
								skipTask = true
								wasSkipped = true
								break
							}
							return budgetErr
						}
					} else {
						return fmt.Errorf("runner: execute: %w", execErr)
					}
				} else {
					log.Info("execute session finished",
						"duration", fmt.Sprintf("%ds", int(elapsed.Seconds())),
						"exit", "0",
					)

					sr, parseErr := session.ParseResult(raw, elapsed)
					if parseErr != nil {
						return fmt.Errorf("runner: parse result: %w", parseErr)
					}
					sessionID = sr.SessionID
					if r.Metrics != nil {
						resolved := r.Metrics.RecordSession(sr.Metrics, sr.Model, "execute", elapsed.Milliseconds())
						if resolved != sr.Model {
							log.Warn("unknown model pricing", "model", sr.Model, "fallback", resolved)
						}
					}
					// Story 7.7: budget check after execute session (success path)
					if budgetErr := r.checkBudget(ctx, taskText, &budgetWarned); budgetErr != nil {
						if errors.Is(budgetErr, errBudgetSkip) {
							skipTask = true
							wasSkipped = true
							break
						}
						return budgetErr
					}
					// DESIGN-4: per-task budget check after execute session (success path)
					if budgetErr := r.checkTaskBudget(ctx, taskText); budgetErr != nil {
						if errors.Is(budgetErr, errBudgetSkip) {
							skipTask = true
							wasSkipped = true
							break
						}
						return budgetErr
					}

					gitT1 := time.Now()
					headAfter, err := r.Git.HeadCommit(ctx)
					if r.Metrics != nil {
						r.Metrics.RecordLatency(LatencyBreakdown{GitMs: time.Since(gitT1).Milliseconds()})
					}
					if err != nil {
						if r.Metrics != nil {
							r.Metrics.RecordError(CategorizeError(err), err.Error())
						}
						return fmt.Errorf("runner: head commit after: %w", err)
					}
					lastKnownSHA = headAfter

					if headBefore == headAfter {
						log.Info("commit check",
							"before", headBefore,
							"after", headAfter,
							"changed", "false",
							"reason", "retry",
						)
						consecutiveNoCommit++
						if r.Cfg.StuckThreshold > 0 && consecutiveNoCommit >= r.Cfg.StuckThreshold {
							stuckMsg := fmt.Sprintf("No commit in last %d attempts. Consider a different approach.", consecutiveNoCommit)
							log.Warn("stuck detected", "task", taskText, "no_commit_count", fmt.Sprintf("%d", consecutiveNoCommit))
							// Non-fatal: feedback injection failure is logged but does not abort the loop.
							// InjectFeedback error paths covered by TestInjectFeedback_Scenarios.
							if err := InjectFeedback(r.TasksFile, taskDescription(taskText), stuckMsg); err != nil {
								log.Warn("stuck feedback injection failed", "error", err.Error())
							}
							if r.Metrics != nil {
								r.Metrics.RecordRetry("stuck")
							}
						}
						needsRetry = true // AC1: no commit
					} else {
						consecutiveNoCommit = 0
						log.Info("commit check",
							"before", headBefore,
							"after", headAfter,
							"changed", "true",
						)
						// Story 7.2: collect git diff stats after commit detection (NFR24 best-effort)
						diffStats, diffErr := r.Git.DiffStats(ctx, headBefore, headAfter)
						if diffErr != nil {
							log.Warn("diff stats failed", "error", diffErr.Error())
						} else {
							log.Info("commit stats",
								"files", fmt.Sprintf("%d", diffStats.FilesChanged),
								"insertions", fmt.Sprintf("%d", diffStats.Insertions),
								"deletions", fmt.Sprintf("%d", diffStats.Deletions),
								"packages", strings.Join(diffStats.Packages, ","),
							)
							lastDiffStats = diffStats // DESIGN-4: save for review model selection
							if r.Metrics != nil {
								r.Metrics.RecordGitDiff(*diffStats)
							}
							// Story 7.8: similarity detection using Packages from DiffStats
							if r.Similarity != nil {
								r.Similarity.Push(diffStats.Packages)
								simLevel, simScore := r.Similarity.Check()
								switch simLevel {
								case "hard":
									log.Error("similarity loop detected",
										"score", fmt.Sprintf("%.2f", simScore),
										"threshold", fmt.Sprintf("%.2f", r.Cfg.SimilarityHard),
									)
									if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
										emergencyText := fmt.Sprintf("%s\nSimilarity loop detected (score: %.2f). Diffs are repeating.",
											taskText, simScore)
										decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
										if gateErr != nil {
											return fmt.Errorf("runner: similarity gate: %w", gateErr)
										}
										if decision.Action == config.ActionQuit {
											return fmt.Errorf("runner: similarity gate: %w", decision)
										}
									}
								case "warn":
									log.Warn("similarity warning",
										"score", fmt.Sprintf("%.2f", simScore),
										"threshold", fmt.Sprintf("%.2f", r.Cfg.SimilarityWarn),
									)
									if feedbackErr := InjectFeedback(r.TasksFile, taskText, "Recent changes are very similar. Try a fundamentally different approach."); feedbackErr != nil {
										log.Warn("similarity feedback injection failed", "error", feedbackErr.Error())
									}
								}
							}
						}
					}
				}

				if needsRetry {
					executeAttempts++ // AC2: increment counter
					if executeAttempts >= r.Cfg.MaxIterations {
						log.Error("execute attempts exhausted",
							"task", taskText,
							"attempts", fmt.Sprintf("%d", executeAttempts),
						)
						if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
							emergencyText := fmt.Sprintf("execute attempts exhausted (%d/%d) for %q",
								executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text)
							if r.Metrics != nil {
								emergencyText += fmt.Sprintf("\nCost so far: $%.2f", r.Metrics.CumulativeCost())
							}
							gateT0 := time.Now()
							decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
							gateElapsed := time.Since(gateT0)
							if r.Metrics != nil {
								r.Metrics.RecordLatency(LatencyBreakdown{GateMs: gateElapsed.Milliseconds()})
							}
							if gateErr != nil {
								if r.Metrics != nil {
									r.Metrics.RecordError(CategorizeError(gateErr), gateErr.Error())
								}
								return fmt.Errorf("runner: emergency gate: %w", gateErr)
							}
							r.recordGateDecision(decision.Action, gateElapsed.Milliseconds(), taskText)
							if decision.Action == config.ActionQuit {
								return fmt.Errorf("runner: emergency gate: %w", decision)
							}
							if decision.Action == config.ActionSkip {
								taskDesc := taskDescription(result.OpenTasks[0].Text)
								if err := SkipTask(r.TasksFile, taskDesc); err != nil {
									return err // SkipTask wraps with "runner: skip task:" prefix
								}
								skipTask = true
								wasSkipped = true
								break // exit execute retry loop
							}
							if decision.Action == config.ActionRetry {
								executeAttempts = 0
								if decision.Feedback != "" {
									taskDesc := taskDescription(result.OpenTasks[0].Text)
									if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
										return err // InjectFeedback wraps with "runner: inject feedback:" prefix
									}
								}
								continue // restart execute retry loop — skip resume/backoff
							}
						} else {
							return fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q (check logs for details): %w",
								executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text, config.ErrMaxRetries)
						}
					}
					// Exponential backoff (NFR12): 1s, 2s, 4s...
					backoff := time.Duration(1<<uint(executeAttempts-1)) * time.Second

					var retryReason string
					if execErr != nil {
						retryReason = "exit_error"
					} else {
						retryReason = "no_commit"
					}
					log.Info("retry scheduled",
						"attempt", fmt.Sprintf("%d/%d", executeAttempts+1, r.Cfg.MaxIterations),
						"backoff", fmt.Sprintf("%ds", int(backoff.Seconds())),
						"reason", retryReason,
					)

					// Resume-extraction: capture WIP state before retry
					if reErr := r.ResumeExtractFn(ctx, rc, sessionID); reErr != nil {
						return fmt.Errorf("runner: retry: resume extract: %w", reErr)
					}
					// Dirty state recovery before retry
					if _, recErr := RecoverDirtyState(ctx, r.Git); recErr != nil {
						return fmt.Errorf("runner: retry: recover: %w", recErr)
					}
					if ctx.Err() != nil {
						return fmt.Errorf("runner: retry: %w", ctx.Err())
					}
					r.SleepFn(backoff)
					continue
				}

				// Success: commit detected — exit retry loop
				break
			}

			// Story 5.5: emergency skip at execute exhaustion exits review cycle loop too
			if skipTask {
				break
			}

			// DESIGN-4: pass diff stats, gate flag, and hydra state to ReviewFn via RunConfig for model routing
			// selectReviewModel is called once inside RealReview; we log the inputs here.
			rc.LastDiffStats = lastDiffStats
			rc.IsGate = result.OpenTasks[0].HasGate
			rc.HydraDetected = hydraDetected
			// Story 9.3: progressive review fields (FR72-FR76)
			rc.Cycle = reviewCycles + 1
			rc.MinSeverity = progParams.MinSeverity
			rc.MaxFindings = progParams.MaxFindings
			rc.IncrementalDiff = progParams.IncrementalDiff
			rc.HighEffort = progParams.HighEffort
			rc.PrevFindings = prevFindingsText

			log.Info("review session started", "task", taskText, "isGate", fmt.Sprintf("%v", rc.IsGate), "hydra", fmt.Sprintf("%v", hydraDetected))
			reviewStart := time.Now()
			rr, err := r.ReviewFn(ctx, rc)
			reviewElapsed := time.Since(reviewStart)
			if r.Metrics != nil {
				r.Metrics.RecordLatency(LatencyBreakdown{ReviewMs: reviewElapsed.Milliseconds()})
			}
			if err != nil {
				if r.Metrics != nil {
					r.Metrics.RecordError(CategorizeError(err), err.Error())
				}
				return fmt.Errorf("runner: review: %w", err)
			}
			// AC5: record review session cost (tokens + model from RealReview)
			if r.Metrics != nil && rr.SessionMetrics != nil {
				resolved := r.Metrics.RecordSession(rr.SessionMetrics, rr.Model, "review", reviewElapsed.Milliseconds())
				if resolved != rr.Model {
					log.Warn("unknown model pricing", "model", rr.Model, "fallback", resolved)
				}
			}
			// Story 7.7: budget check after review session
			if budgetErr := r.checkBudget(ctx, taskText, &budgetWarned); budgetErr != nil {
				if errors.Is(budgetErr, errBudgetSkip) {
					wasSkipped = true
					break
				}
				return budgetErr
			}
			// DESIGN-4: per-task budget check after review session
			if budgetErr := r.checkTaskBudget(ctx, taskText); budgetErr != nil {
				if errors.Is(budgetErr, errBudgetSkip) {
					wasSkipped = true
					break
				}
				return budgetErr
			}
			if rr.Clean {
				if r.Metrics != nil {
					r.Metrics.RecordFindingsCycle(0)
					r.Metrics.RecordCycleDuration(time.Since(cycleStart).Milliseconds())
				}
				log.Info("review session finished",
					"duration", fmt.Sprintf("%ds", int(reviewElapsed.Seconds())),
					"clean", "true",
				)
				break // AC4: clean review exits review cycle loop
			}
			log.Info("review session finished",
				"duration", fmt.Sprintf("%ds", int(reviewElapsed.Seconds())),
				"clean", "false",
				"findings", "true",
			)

			// Story 9.2: progressive severity filtering + findings budget.
			// Apply BEFORE logging/metrics so only filtered findings are counted.
			if len(rr.Findings) > 0 {
				params := ProgressiveParams(reviewCycles+1, r.Cfg.MaxReviewIterations)
				beforeCount := len(rr.Findings)
				filtered := FilterBySeverity(rr.Findings, params.MinSeverity)
				truncated := TruncateFindings(filtered, params.MaxFindings)

				if len(truncated) < beforeCount {
					// Log filtered-out findings for traceability.
					for _, f := range rr.Findings {
						if ParseSeverity(f.Severity) < params.MinSeverity {
							log.Info("finding below threshold",
								"severity", f.Severity,
								"description", f.Description,
							)
						}
					}
					// Rewrite review-findings.md with only filtered findings.
					rewritePath := filepath.Join(r.Cfg.ProjectRoot, "review-findings.md")
					if err := writeFilteredFindings(rewritePath, truncated); err != nil {
						return fmt.Errorf("runner: write filtered findings: %w", err)
					}
					log.Info("findings filtered",
						"before", fmt.Sprintf("%d", beforeCount),
						"after", fmt.Sprintf("%d", len(truncated)),
						"min_severity", params.MinSeverity.String(),
						"max_budget", fmt.Sprintf("%d", params.MaxFindings),
					)
				}
				rr.Findings = truncated
			}

			// Story 7.4: log severity breakdown and record findings in metrics.
			if len(rr.Findings) > 0 {
				var critical, high, medium, low int
				for _, f := range rr.Findings {
					switch f.Severity {
					case "CRITICAL":
						critical++
					case "HIGH":
						high++
					case "MEDIUM":
						medium++
					case "LOW":
						low++
					}
				}
				log.Info("review findings",
					"total", fmt.Sprintf("%d", len(rr.Findings)),
					"critical", fmt.Sprintf("%d", critical),
					"high", fmt.Sprintf("%d", high),
					"medium", fmt.Sprintf("%d", medium),
					"low", fmt.Sprintf("%d", low),
				)
				if r.Metrics != nil {
					r.Metrics.RecordReview(rr.Findings)
				}
			}

			// DESIGN-6: record findings count per cycle and detect hydra pattern
			findingsCount := len(rr.Findings)
			if r.Metrics != nil {
				hydra := r.Metrics.RecordFindingsCycle(findingsCount)
				if hydra {
					log.Warn("hydra pattern detected: findings not decreasing",
						"task", taskText,
						"count", fmt.Sprintf("%d", findingsCount),
						"cycle", fmt.Sprintf("%d", reviewCycles+1),
					)
					// DESIGN-4: escalate to standard review model on hydra detection
					if !hydraDetected {
						hydraDetected = true
						log.Warn("hydra escalation: switching to full review model",
							"task", taskText,
						)
					}
				}
			}

			// MINOR-7: record full cycle duration
			if r.Metrics != nil {
				r.Metrics.RecordCycleDuration(time.Since(cycleStart).Milliseconds())
			}

			// Story 9.3: capture current findings text for next cycle's incremental review context.
			// Note: rr.Findings is already filtered by FilterBySeverity+TruncateFindings (line 1313).
			if len(rr.Findings) > 0 {
				var fb strings.Builder
				for _, f := range rr.Findings {
					fmt.Fprintf(&fb, "### [%s] %s\n", f.Severity, f.Description)
				}
				prevFindingsText = fb.String()
			}

			reviewCycles++
			if reviewCycles >= r.Cfg.MaxReviewIterations {
				log.Error("review cycles exhausted",
					"task", taskText,
					"cycles", fmt.Sprintf("%d", reviewCycles),
				)
				if r.Cfg.GatesEnabled && r.EmergencyGatePromptFn != nil {
					emergencyText := fmt.Sprintf("review cycles exhausted (%d/%d) for %q",
						reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text)
					if r.Metrics != nil {
						emergencyText += fmt.Sprintf("\nCost so far: $%.2f", r.Metrics.CumulativeCost())
					}
					gateT0 := time.Now()
					decision, gateErr := r.EmergencyGatePromptFn(ctx, emergencyText)
					gateElapsed := time.Since(gateT0)
					if r.Metrics != nil {
						r.Metrics.RecordLatency(LatencyBreakdown{GateMs: gateElapsed.Milliseconds()})
					}
					if gateErr != nil {
						if r.Metrics != nil {
							r.Metrics.RecordError(CategorizeError(gateErr), gateErr.Error())
						}
						return fmt.Errorf("runner: emergency gate: %w", gateErr)
					}
					r.recordGateDecision(decision.Action, gateElapsed.Milliseconds(), taskText)
					if decision.Action == config.ActionQuit {
						return fmt.Errorf("runner: emergency gate: %w", decision)
					}
					if decision.Action == config.ActionSkip {
						taskDesc := taskDescription(result.OpenTasks[0].Text)
						if err := SkipTask(r.TasksFile, taskDesc); err != nil {
							return err // SkipTask wraps with "runner: skip task:" prefix
						}
						wasSkipped = true
						break // exit review cycle loop
					}
					if decision.Action == config.ActionRetry {
						reviewCycles = 0
						if decision.Feedback != "" {
							taskDesc := taskDescription(result.OpenTasks[0].Text)
							if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
								return err // InjectFeedback wraps with "runner: inject feedback:" prefix
							}
						}
						continue // restart review cycle loop
					}
				} else {
					// BUG-2: continue to next task instead of aborting entire run.
					log.Warn("review cycles exhausted, skipping task",
						"task", taskText,
						"cycles", fmt.Sprintf("%d/%d", reviewCycles, r.Cfg.MaxReviewIterations),
					)
					if r.Metrics != nil {
						r.Metrics.RecordError("review_exhaust", fmt.Sprintf("review cycles exhausted (%d/%d) for %q",
							reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text))
						r.Metrics.FinishTask("error", lastKnownSHA)
					}
					reviewExhausted = true
					break // exit review cycle loop, continue to next task
				}
			}
		}

		// BUG-2: review exhaust already called FinishTask("error"), skip to next task
		if reviewExhausted {
			continue
		}

		// Story 5.5: emergency skip bypasses validation, completion counter, and gate check
		if wasSkipped {
			if r.Metrics != nil {
				r.Metrics.FinishTask("skipped", "")
			}
			continue
		}

		// Story 6.1: post-validate LEARNINGS.md entries after session ends
		if r.Knowledge != nil {
			if err := r.Knowledge.ValidateNewLessons(ctx, LessonsData{
				Source:      "execute",
				Snapshot:    learningsSnapshot,
				BudgetLimit: r.Cfg.LearningsBudget,
			}); err != nil {
				return fmt.Errorf("runner: post-validate lessons: %w", err)
			}
		}
		// Update snapshot for next task iteration
		if snapshotData, snapErr := os.ReadFile(learningsPath); snapErr == nil {
			learningsSnapshot = string(snapshotData)
		}

		// Story 5.4: increment completion counter after clean review, before gate check.
		completedTasks++
		log.Info("task completed",
			"task", taskText,
			"iterations", fmt.Sprintf("%d", reviewCycles+1),
			"duration", fmt.Sprintf("%ds", int(time.Since(taskStart).Seconds())),
		)

		// Gate check: after clean review, before outer loop continues to next task.
		// Story 5.2: [GATE] tag trigger. Story 5.4: checkpoint trigger (every N tasks).
		// Combined: single prompt when both triggers fire simultaneously (AC5).
		isGateTask := result.OpenTasks[0].HasGate
		isCheckpoint := r.Cfg.GatesCheckpoint > 0 && completedTasks%r.Cfg.GatesCheckpoint == 0

		if r.Cfg.GatesEnabled && (isGateTask || isCheckpoint) && r.GatePromptFn != nil {
			gateText := result.OpenTasks[0].Text
			if isCheckpoint {
				gateText += fmt.Sprintf(" (checkpoint every %d)", r.Cfg.GatesCheckpoint)
			}
			if r.Metrics != nil {
				gateText += fmt.Sprintf("\nCost so far: $%.2f", r.Metrics.CumulativeCost())
			}

			gateT0 := time.Now()
			decision, gateErr := r.GatePromptFn(ctx, gateText)
			gateElapsed := time.Since(gateT0)
			if r.Metrics != nil {
				r.Metrics.RecordLatency(LatencyBreakdown{GateMs: gateElapsed.Milliseconds()})
			}
			if gateErr != nil {
				if r.Metrics != nil {
					r.Metrics.RecordError(CategorizeError(gateErr), gateErr.Error())
				}
				return fmt.Errorf("runner: gate: %w", gateErr)
			}
			r.recordGateDecision(decision.Action, gateElapsed.Milliseconds(), taskText)
			if decision.Action == config.ActionQuit {
				return fmt.Errorf("runner: gate: %w", decision)
			}
			if decision.Action == config.ActionRetry {
				completedTasks-- // Story 5.4 AC8: undo increment — task not truly completed
				taskDesc := taskDescription(result.OpenTasks[0].Text)
				if decision.Feedback != "" {
					if err := InjectFeedback(r.TasksFile, taskDesc, decision.Feedback); err != nil {
						return err // InjectFeedback wraps with "runner: inject feedback:" prefix
					}
				}
				if err := RevertTask(r.TasksFile, taskDesc); err != nil {
					return err // RevertTask wraps with "runner: revert task:" prefix
				}
				continue // outer for-loop: re-reads tasks, reviewCycles/executeAttempts re-initialized
			}
			// approve, skip → continue to next task (fall through)
		}

		// Story 6.5a: distillation trigger — after gate, before next iteration.
		// Increment monotonic counter (persists across runs, unlike completedTasks).
		distillState.MonotonicTaskCounter++
		if saveErr := SaveDistillState(distillStatePath, distillState); saveErr != nil {
			distillState.MonotonicTaskCounter-- // rollback on save failure
			fmt.Fprintf(os.Stderr, "WARNING: save distill state: %v\n", saveErr)
		}

		if r.DistillFn != nil {
			budgetStatus, budgetErr := BudgetCheck(ctx, learningsPath, r.Cfg.LearningsBudget)
			if budgetErr == nil && budgetStatus.NearLimit {
				cooldownMet := distillState.MonotonicTaskCounter-distillState.LastDistillTask >= r.Cfg.DistillCooldown
				if cooldownMet {
					log.Info("distillation triggered",
						"counter", fmt.Sprintf("%d", distillState.MonotonicTaskCounter),
						"cooldown", fmt.Sprintf("%d", r.Cfg.DistillCooldown),
					)
					distillT0 := time.Now()
					r.runDistillation(ctx, distillState, distillStatePath, budgetStatus)
					if r.Metrics != nil {
						r.Metrics.RecordLatency(LatencyBreakdown{DistillMs: time.Since(distillT0).Milliseconds()})
					}
				}
			}
		}

		// Story 8.6: per-task Serena sync — after distillation, before FinishTask.
		if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "task" &&
			r.SerenaSyncFn != nil && r.CodeIndexer != nil &&
			r.CodeIndexer.Available(r.Cfg.ProjectRoot) {
			r.runSerenaSync(ctx, taskHeadBefore, taskText)
		}

		// Finalize task metrics after all phases (latency already recorded incrementally).
		if r.Metrics != nil {
			headSHA, _ := r.Git.HeadCommit(ctx) // best-effort: SHA is optional for metrics
			r.Metrics.FinishTask("completed", headSHA)
		}
	}

	return nil
}

// runDistillation calls DistillFn with failure handling (Story 6.5a).
// ErrBadFormat: ONE free retry before gate. Other errors: immediate gate.
// All failures are non-fatal: human gate or log warning, then continue.
// On success: updates LastDistillTask and saves state.
func (r *Runner) runDistillation(ctx context.Context, state *DistillState, statePath string, budget BudgetStatus) {
	err := r.DistillFn(ctx, state)

	// ErrBadFormat: one free retry (H4)
	if err != nil && errors.Is(err, ErrBadFormat) {
		err = r.DistillFn(ctx, state)
	}

	if err != nil {
		// Human gate on failure — non-fatal
		r.handleDistillFailure(ctx, err, state, statePath, budget)
		return
	}

	// Success: update state
	state.LastDistillTask = state.MonotonicTaskCounter
	if saveErr := SaveDistillState(statePath, state); saveErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: save distill state after success: %v\n", saveErr)
	}
}

// handleDistillFailure presents human gate on distillation failure.
// Gate options: skip (log + continue), retry once, retry 5 times, quit.
// All failures are non-fatal (Task 5.12): quit logs warning and continues.
func (r *Runner) handleDistillFailure(ctx context.Context, distillErr error, state *DistillState, statePath string, budget BudgetStatus) {
	if r.GatePromptFn == nil || !r.Cfg.GatesEnabled {
		fmt.Fprintf(os.Stderr, "WARNING: distillation failed: %v\n", distillErr)
		return
	}

	gateText := fmt.Sprintf("distillation failed: %v. LEARNINGS.md: %d/%d lines",
		distillErr, budget.Lines, budget.Limit)
	if r.Metrics != nil {
		gateText += fmt.Sprintf("\nCost so far: $%.2f", r.Metrics.CumulativeCost())
	}

	gateT0 := time.Now()
	decision, gateErr := r.GatePromptFn(ctx, gateText)
	gateElapsed := time.Since(gateT0)
	if gateErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: distillation gate error: %v\n", gateErr)
		return
	}
	r.recordGateDecision(decision.Action, gateElapsed.Milliseconds(), gateText)
	if decision.Action == config.ActionQuit {
		// Quit is the only fatal path — but distillation is inside the loop,
		// so we log and return (non-fatal). Caller continues to next iteration.
		fmt.Fprintf(os.Stderr, "WARNING: distillation quit requested, continuing\n")
		return
	}

	retryCount := 0
	switch decision.Action {
	case config.ActionRetry:
		retryCount = 1
	case config.ActionSkip:
		fmt.Fprintf(os.Stderr, "WARNING: distillation skipped by user\n")
		return
	default:
		// approve or unknown → treat as skip
		return
	}

	// Check feedback for retry count override: "retry 5" pattern
	if decision.Feedback == "5" {
		retryCount = 5
	}

	for i := 0; i < retryCount; i++ {
		if retryErr := r.DistillFn(ctx, state); retryErr == nil {
			state.LastDistillTask = state.MonotonicTaskCounter
			if saveErr := SaveDistillState(statePath, state); saveErr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: save distill state after retry success: %v\n", saveErr)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "WARNING: distillation retries exhausted, continuing\n")
}

// RunOnce executes a single standalone iteration of the task loop.
// It reads the tasks file, scans for task state, assembles the prompt,
// invokes Claude via session.Execute, parses the result, and retrieves HEAD commit SHA.
// RunOnce is a standalone utility — Execute does NOT delegate to RunOnce.
func RunOnce(ctx context.Context, rc RunConfig) error {
	content, err := os.ReadFile(rc.TasksFile)
	if err != nil {
		return fmt.Errorf("runner: read tasks: %w", err)
	}

	result, scanErr := ScanTasks(string(content))
	if scanErr != nil {
		return scanErr
	}
	if !result.HasOpenTasks() {
		// All tasks completed — caller (Run loop) handles exit
		return nil
	}

	if err := rc.Git.HealthCheck(ctx); err != nil {
		return fmt.Errorf("runner: git health: %w", err)
	}

	// Story 6.2: knowledge injection for walking skeleton
	onceKnowledge, onceAppendSys, onceKnowledgeErr := buildKnowledgeReplacements(rc.Cfg.ProjectRoot)
	if onceKnowledgeErr != nil {
		return fmt.Errorf("runner: build knowledge: %w", onceKnowledgeErr)
	}
	onceHasLearnings := onceKnowledge["__LEARNINGS_CONTENT__"] != ""

	taskText := result.OpenTasks[0].Text
	onceReplacements := map[string]string{
		"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
		"__TASK_CONTENT__":    taskText,
		"__TASK_HASH__":       TaskHash(taskText),
	}
	for k, v := range onceKnowledge {
		onceReplacements[k] = v
	}

	prompt, err := config.AssemblePrompt(
		executeTemplate,
		buildTemplateData(rc.Cfg, "", false, onceHasLearnings),
		onceReplacements,
	)
	if err != nil {
		return fmt.Errorf("runner: assemble prompt: %w", err)
	}

	opts := session.Options{
		Command:                    rc.Cfg.ClaudeCommand,
		Dir:                        rc.Cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   rc.Cfg.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
		AppendSystemPrompt:         onceAppendSys,
	}

	start := time.Now()
	raw, execErr := session.Execute(ctx, opts)
	elapsed := time.Since(start)
	if rc.Logger != nil {
		rc.Logger.SaveSession("once", raw, raw.ExitCode, elapsed)
	}

	if execErr != nil {
		return fmt.Errorf("runner: execute: %w", execErr)
	}

	if _, err := session.ParseResult(raw, elapsed); err != nil {
		return fmt.Errorf("runner: parse result: %w", err)
	}

	if _, err := rc.Git.HeadCommit(ctx); err != nil {
		return fmt.Errorf("runner: head commit: %w", err)
	}

	return nil
}

// RecoverDirtyState checks git health and attempts recovery if dirty tree detected.
// Returns true if recovery was performed, false if repo was clean.
// Returns error only if health check fails for non-dirty reasons or recovery fails.
func RecoverDirtyState(ctx context.Context, git GitClient) (bool, error) {
	err := git.HealthCheck(ctx)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, ErrDirtyTree) {
		return false, fmt.Errorf("runner: dirty state recovery: %w", err)
	}
	if restoreErr := git.RestoreClean(ctx); restoreErr != nil {
		return false, fmt.Errorf("runner: dirty state recovery: %w", restoreErr)
	}
	return true, nil
}

// Run is the main entry point for the execute-review loop.
// It creates a Runner with production defaults and delegates to Runner.Execute.
// Returns RunMetrics (nil on early error) and error.
// When cfg.GatesEnabled is true, wires GatePromptFn to gates.Prompt with os.Stdin/os.Stdout.
// When cfg.SimilarityWindow > 0, creates SimilarityDetector for diff loop detection.
// When cfg.SerenaEnabled is true, wires CodeIndexer to SerenaMCPDetector for detection.
// Opens a RunLogger writing to <ProjectRoot>/<LogDir>/ralph-YYYY-MM-DD.log (appended)
// and duplicating to stderr. Logger failure is non-fatal: falls back to NopLogger.
func Run(ctx context.Context, cfg *config.Config) (*RunMetrics, error) {
	var indexer CodeIndexerDetector = &NoOpCodeIndexerDetector{}
	if cfg.SerenaEnabled {
		indexer = &SerenaMCPDetector{}
	}

	var runLog *RunLogger
	if l, err := OpenRunLogger(cfg.ProjectRoot, cfg.LogDir, cfg.RunID); err == nil {
		runLog = l
		defer l.Close() //nolint:errcheck
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: could not open run log: %v\n", err)
		runLog = NopLogger()
	}

	var mc *MetricsCollector
	if cfg.RunID != "" {
		pricing := config.MergePricing(config.DefaultPricing, cfg.ModelPricing)
		mc = NewMetricsCollector(cfg.RunID, pricing)
	}

	r := &Runner{
		Cfg:         cfg,
		Git:         &ExecGitClient{Dir: cfg.ProjectRoot},
		TasksFile:   filepath.Join(cfg.ProjectRoot, "sprint-tasks.md"),
		ReviewFn:    RealReview,
		SleepFn:     time.Sleep,
		Knowledge:   &FileKnowledgeWriter{projectRoot: cfg.ProjectRoot},
		CodeIndexer: indexer,
		Logger:      runLog,
		Metrics:     mc,
	}
	if cfg.SimilarityWindow > 0 {
		r.Similarity = NewSimilarityDetector(cfg.SimilarityWindow, cfg.SimilarityWarn, cfg.SimilarityHard)
	}
	if cfg.GatesEnabled {
		r.GatePromptFn = func(ctx context.Context, taskText string) (*config.GateDecision, error) {
			return gates.Prompt(ctx, gates.Gate{TaskText: taskText, Reader: os.Stdin, Writer: os.Stdout})
		}
		r.EmergencyGatePromptFn = func(ctx context.Context, taskText string) (*config.GateDecision, error) {
			return gates.Prompt(ctx, gates.Gate{TaskText: taskText, Reader: os.Stdin, Writer: os.Stdout, Emergency: true})
		}
	}
	r.ResumeExtractFn = func(_ context.Context, _ RunConfig, sid string) error {
		return ResumeExtraction(ctx, cfg, r.Knowledge, runLog, r.Metrics, sid)
	}
	r.DistillFn = func(ctx context.Context, state *DistillState) error {
		return AutoDistill(ctx, cfg, state)
	}
	r.SerenaSyncFn = func(ctx context.Context, opts SerenaSyncOpts) (*session.SessionResult, error) {
		return RealSerenaSync(ctx, cfg, opts, runLog)
	}
	return r.Execute(ctx)
}

// Deprecated: RunReview is a walking skeleton from Story 1.12.
// Production review logic uses RealReview via Run(). RunReview is retained
// to avoid breaking integration tests; may be removed in a future story.
func RunReview(ctx context.Context, rc RunConfig) error {
	prompt, err := config.AssemblePrompt(
		reviewTemplate,
		config.TemplateData{},
		map[string]string{
			"__TASK_CONTENT__":      "review stub",
			"__TASK_HASH__":         TaskHash("review stub"),
			"__RALPH_KNOWLEDGE__":   "",
			"__LEARNINGS_CONTENT__": "",
			"__SERENA_HINT__":       "",
		},
	)
	if err != nil {
		return fmt.Errorf("runner: assemble review prompt: %w", err)
	}

	opts := session.Options{
		Command:                    rc.Cfg.ClaudeCommand,
		Dir:                        rc.Cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   rc.Cfg.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	}

	start := time.Now()
	raw, execErr := session.Execute(ctx, opts)
	elapsed := time.Since(start)
	if rc.Logger != nil {
		rc.Logger.SaveSession("once-review", raw, raw.ExitCode, elapsed)
	}

	if execErr != nil {
		return fmt.Errorf("runner: review execute: %w", execErr)
	}

	if _, err := session.ParseResult(raw, elapsed); err != nil {
		return fmt.Errorf("runner: review parse: %w", err)
	}

	return nil
}
