package runner

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/execute.md
var executeTemplate string

//go:embed prompts/review.md
var reviewTemplate string

// ErrNoCommit indicates that a Claude session completed but HEAD did not change.
// Currently unused in production — Story 3.6 replaced direct return with boolean retry logic.
// Retained as exported sentinel for potential errors.Is detection in future stories.
var ErrNoCommit = errors.New("no commit detected")

// ReviewResult holds the outcome of a review step.
type ReviewResult struct {
	Clean bool // true when review found no actionable findings
}

// ReviewFunc is the signature for the review step called after each successful execute.
// Default: defaultReviewStub. Epic 4 replaces with real review logic.
type ReviewFunc func(ctx context.Context, rc RunConfig) (ReviewResult, error)

// ResumeExtractFunc is the signature for resume-extraction called before retry.
// The sessionID parameter comes from SessionResult.SessionID (empty string if not available).
// Default: closure over ResumeExtraction in Run(). Tests inject custom implementations.
type ResumeExtractFunc func(ctx context.Context, rc RunConfig, sessionID string) error

// defaultReviewStub returns a clean review result.
// TODO(epic-4): replace with real review logic
func defaultReviewStub(_ context.Context, _ RunConfig) (ReviewResult, error) {
	return ReviewResult{Clean: true}, nil
}

// ResumeExtraction invokes claude --resume to capture WIP progress from an
// interrupted execute session. Returns nil when sessionID is empty (nothing to resume).
func ResumeExtraction(ctx context.Context, cfg *config.Config, kw KnowledgeWriter, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	opts := session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Resume:                     sessionID,
		MaxTurns:                   cfg.MaxTurns,
		Model:                      cfg.ModelExecute,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	}

	start := time.Now()
	raw, execErr := session.Execute(ctx, opts)
	elapsed := time.Since(start)

	if execErr != nil {
		return fmt.Errorf("runner: resume extraction: execute: %w", execErr)
	}

	sr, parseErr := session.ParseResult(raw, elapsed)
	if parseErr != nil {
		return fmt.Errorf("runner: resume extraction: parse: %w", parseErr)
	}

	// TaskDescription left empty — no plumbing from caller yet (Epic 6 may add)
	if err := kw.WriteProgress(ctx, ProgressData{SessionID: sr.SessionID}); err != nil {
		return fmt.Errorf("runner: resume extraction: write progress: %w", err)
	}

	return nil
}

// RunConfig passes dependencies to runner functions.
type RunConfig struct {
	Cfg       *config.Config
	Git       GitClient
	TasksFile string // path to sprint-tasks.md
}

// Runner orchestrates the execute-review loop with injectable dependencies.
// Public API: Run() creates a Runner internally. Tests construct Runner directly.
type Runner struct {
	Cfg             *config.Config
	Git             GitClient
	TasksFile       string              // path to sprint-tasks.md
	ReviewFn        ReviewFunc          // called after each successful execute with commit
	ResumeExtractFn ResumeExtractFunc   // called before retry to extract session context
	SleepFn         func(time.Duration) // injectable sleep for testable backoff
	Knowledge       KnowledgeWriter     // records execution progress; no-op in Epic 3
}

// Execute runs the main task loop: startup recovery, then iterate over tasks.
// Startup: recovers dirty working tree (RecoverDirtyState), non-dirty health errors abort.
// Each iteration: read tasks → scan → assemble prompt → review cycle loop
// (execute retry → review → check Clean, stop at MaxReviewIterations (FR24)).
// Execute retry: session.Execute → check commit → retry on no-commit or non-zero
// exit up to Cfg.MaxIterations per review cycle.
// Loops up to Cfg.MaxIterations task-processing cycles in the outer loop.
// Returns nil when all tasks are complete. Returns error on any failure.
func (r *Runner) Execute(ctx context.Context) error {
	// recovered bool unused — no startup logging plumbing yet
	if _, err := RecoverDirtyState(ctx, r.Git); err != nil {
		return fmt.Errorf("runner: startup: %w", err)
	}

	rc := RunConfig{
		Cfg:       r.Cfg,
		Git:       r.Git,
		TasksFile: r.TasksFile,
	}

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
			return nil
		}

		prompt, err := config.AssemblePrompt(
			executeTemplate,
			config.TemplateData{GatesEnabled: r.Cfg.GatesEnabled},
			map[string]string{
				"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
			},
		)
		if err != nil {
			return fmt.Errorf("runner: assemble prompt: %w", err)
		}

		opts := session.Options{
			Command:                    r.Cfg.ClaudeCommand,
			Dir:                        r.Cfg.ProjectRoot,
			Prompt:                     prompt,
			MaxTurns:                   r.Cfg.MaxTurns,
			Model:                      r.Cfg.ModelExecute,
			OutputJSON:                 true,
			DangerouslySkipPermissions: true,
		}

		// Review cycle loop: per-task counter, resets when clean (AC3, AC4)
		reviewCycles := 0
		for {
			// Per-review-cycle retry loop: executeAttempts resets each cycle
			executeAttempts := 0
			for {
				headBefore, err := r.Git.HeadCommit(ctx)
				if err != nil {
					return fmt.Errorf("runner: head commit before: %w", err)
				}

				start := time.Now()
				raw, execErr := session.Execute(ctx, opts)
				elapsed := time.Since(start)

				needsRetry := false
				var sessionID string

				if execErr != nil {
					// Distinguish retryable (exit error) from fatal (binary not found, ctx cancel)
					var exitErr *exec.ExitError
					if errors.As(execErr, &exitErr) {
						needsRetry = true // AC6: non-zero exit triggers retry
						// Try to parse for sessionID despite error
						if sr, parseErr := session.ParseResult(raw, elapsed); parseErr == nil {
							sessionID = sr.SessionID
						}
					} else {
						return fmt.Errorf("runner: execute: %w", execErr)
					}
				} else {
					sr, parseErr := session.ParseResult(raw, elapsed)
					if parseErr != nil {
						return fmt.Errorf("runner: parse result: %w", parseErr)
					}
					sessionID = sr.SessionID

					headAfter, err := r.Git.HeadCommit(ctx)
					if err != nil {
						return fmt.Errorf("runner: head commit after: %w", err)
					}

					if headBefore == headAfter {
						needsRetry = true // AC1: no commit
					}
				}

				if needsRetry {
					executeAttempts++ // AC2: increment counter
					if executeAttempts >= r.Cfg.MaxIterations {
						return fmt.Errorf("runner: execute attempts exhausted (%d/%d) for %q (check logs for details): %w",
							executeAttempts, r.Cfg.MaxIterations, result.OpenTasks[0].Text, config.ErrMaxRetries)
					}
					// Resume-extraction: capture WIP state before retry
					if reErr := r.ResumeExtractFn(ctx, rc, sessionID); reErr != nil {
						return fmt.Errorf("runner: retry: resume extract: %w", reErr)
					}
					// Dirty state recovery before retry
					if _, recErr := RecoverDirtyState(ctx, r.Git); recErr != nil {
						return fmt.Errorf("runner: retry: recover: %w", recErr)
					}
					// Exponential backoff (NFR12): 1s, 2s, 4s...
					if ctx.Err() != nil {
						return fmt.Errorf("runner: retry: %w", ctx.Err())
					}
					backoff := time.Duration(1<<uint(executeAttempts-1)) * time.Second
					r.SleepFn(backoff)
					continue
				}

				// Success: commit detected — exit retry loop
				break
			}

			rr, err := r.ReviewFn(ctx, rc)
			if err != nil {
				return fmt.Errorf("runner: review: %w", err)
			}
			if rr.Clean {
				break // AC4: clean review exits review cycle loop
			}
			reviewCycles++
			if reviewCycles >= r.Cfg.MaxReviewIterations {
				return fmt.Errorf("runner: review cycles exhausted (%d/%d) for %q (check logs for details): %w",
					reviewCycles, r.Cfg.MaxReviewIterations, result.OpenTasks[0].Text, config.ErrMaxReviewCycles)
			}
		}
	}

	return nil
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

	prompt, err := config.AssemblePrompt(
		executeTemplate,
		config.TemplateData{GatesEnabled: rc.Cfg.GatesEnabled},
		map[string]string{
			"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
		},
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
	}

	start := time.Now()
	raw, execErr := session.Execute(ctx, opts)
	elapsed := time.Since(start)

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
func Run(ctx context.Context, cfg *config.Config) error {
	r := &Runner{
		Cfg:       cfg,
		Git:       &ExecGitClient{Dir: cfg.ProjectRoot},
		TasksFile: filepath.Join(cfg.ProjectRoot, "sprint-tasks.md"),
		ReviewFn:  defaultReviewStub,
		SleepFn:   time.Sleep,
		Knowledge: &NoOpKnowledgeWriter{},
	}
	r.ResumeExtractFn = func(_ context.Context, _ RunConfig, sid string) error {
		return ResumeExtraction(ctx, cfg, r.Knowledge, sid)
	}
	return r.Execute(ctx)
}

// RunReview runs a standalone review step using a real Claude session.
// Walking skeleton function — Execute uses ReviewFunc instead (not RunReview).
// Retained for potential standalone use; may be retired in Epic 4.
func RunReview(ctx context.Context, rc RunConfig) error {
	prompt, err := config.AssemblePrompt(
		reviewTemplate,
		config.TemplateData{},
		map[string]string{
			"__TASK_CONTENT__": "review stub",
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

	if execErr != nil {
		return fmt.Errorf("runner: review execute: %w", execErr)
	}

	if _, err := session.ParseResult(raw, elapsed); err != nil {
		return fmt.Errorf("runner: review parse: %w", err)
	}

	return nil
}
