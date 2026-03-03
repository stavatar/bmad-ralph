package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

var distillCmd = &cobra.Command{
	Use:   "distill",
	Short: "Compress LEARNINGS.md via distillation",
	Long: `Manually trigger distillation of LEARNINGS.md into compressed rule files.
Reuses the same distillation pipeline as auto-distillation during ralph run.

WARNING: Do not run ralph distill concurrently with ralph run.`,
	RunE: runDistill,
}

func runDistill(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	learningsPath := filepath.Join(cfg.ProjectRoot, "LEARNINGS.md")
	if _, err := os.Stat(learningsPath); err != nil {
		if os.IsNotExist(err) {
			return &config.ExitCodeError{Code: 1, Message: "LEARNINGS.md not found — nothing to distill"}
		}
		return fmt.Errorf("ralph: distill: %w", err)
	}

	distillStatePath := filepath.Join(cfg.ProjectRoot, ".ralph", "distill-state.json")
	state, err := runner.LoadDistillState(distillStatePath)
	if err != nil {
		return fmt.Errorf("ralph: distill: %w", err)
	}

	ctx := cmd.Context()
	distillErr := runner.AutoDistill(ctx, cfg, state)

	if distillErr == nil {
		state.LastDistillTask = state.MonotonicTaskCounter
		if saveErr := runner.SaveDistillState(distillStatePath, state); saveErr != nil {
			return fmt.Errorf("ralph: distill: %w", saveErr)
		}
		fmt.Printf("Distillation complete.\n")
		return nil
	}

	// Failure path: display error, prompt for retry
	color.Red("Distillation failed: %v", distillErr)

	lineCount := countFileLines(learningsPath)
	fmt.Fprintf(os.Stderr, "LEARNINGS.md: %d lines\n", lineCount)
	fmt.Fprintf(os.Stderr, "Retry? [y/N]: ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(scanner.Text())
		if answer == "y" || answer == "Y" {
			retryErr := runner.AutoDistill(ctx, cfg, state)
			if retryErr == nil {
				state.LastDistillTask = state.MonotonicTaskCounter
				if saveErr := runner.SaveDistillState(distillStatePath, state); saveErr != nil {
					return fmt.Errorf("ralph: distill: %w", saveErr)
				}
				fmt.Printf("Distillation complete.\n")
				return nil
			}
			// Retry failed — best-effort restore; already returning error to user
			_ = runner.RestoreDistillationBackups(cfg.ProjectRoot)
			return &config.ExitCodeError{Code: 1, Message: retryErr.Error()}
		}
	}

	// Abort: best-effort restore; already returning error to user
	_ = runner.RestoreDistillationBackups(cfg.ProjectRoot)
	return &config.ExitCodeError{Code: 1, Message: distillErr.Error()}
}

// countFileLines returns the line count of a file. Returns 0 on error.
func countFileLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
}
