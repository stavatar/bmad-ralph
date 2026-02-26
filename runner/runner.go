package runner

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/execute.md
var executeTemplate string

//go:embed prompts/review.md
var reviewTemplate string

// GitClient abstracts git operations for the runner.
// Consumer-side interface per naming convention (interfaces in consumer package).
// Story 3.3 extends to full interface + ExecGitClient implementation.
type GitClient interface {
	HealthCheck(ctx context.Context) error
	HasNewCommit(ctx context.Context) (bool, error)
}

// RunConfig passes dependencies to runner functions.
type RunConfig struct {
	Cfg       *config.Config
	Git       GitClient
	TasksFile string // path to sprint-tasks.md
}

// RunOnce executes a single task from the sprint-tasks file.
// It reads the tasks file, finds the first open task, assembles the prompt,
// invokes Claude via session.Execute, parses the result, and checks for commits.
func RunOnce(ctx context.Context, rc RunConfig) error {
	content, err := os.ReadFile(rc.TasksFile)
	if err != nil {
		return fmt.Errorf("runner: read tasks: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var taskLine string
	for _, line := range lines {
		if config.TaskOpenRegex.MatchString(line) {
			taskLine = line
			break
		}
	}
	if taskLine == "" {
		return fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)
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

	if _, err := rc.Git.HasNewCommit(ctx); err != nil {
		return fmt.Errorf("runner: check commit: %w", err)
	}

	return nil
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
