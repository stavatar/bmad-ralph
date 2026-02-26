package runner

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/execute.md
var executeTemplate string

//go:embed prompts/review.md
var reviewTemplate string

// RunConfig passes dependencies to runner functions.
type RunConfig struct {
	Cfg       *config.Config
	Git       GitClient
	TasksFile string // path to sprint-tasks.md
}

// RunOnce executes a single iteration of the task loop.
// It reads the tasks file, scans for task state via ScanTasks, assembles the prompt,
// invokes Claude via session.Execute, parses the result, and retrieves HEAD commit SHA.
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
// Story 3.5 implements the full loop. This stub validates CLI wiring.
func Run(ctx context.Context, cfg *config.Config) error {
	return fmt.Errorf("runner: loop not implemented")
}

// RunReview runs a standalone review step.
// Walking skeleton review is a stub — Story 3.5+ adds session context.
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
