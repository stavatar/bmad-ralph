package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/plan"
)

func init() {
	replanCmd.Flags().String("output", "", "Output file path (overrides config)")
	replanCmd.Flags().Bool("no-review", false, "Skip AI review of generated plan")
	replanCmd.Flags().Bool("force", false, "Bypass size warning")
	replanCmd.Flags().StringArray("input", nil, "Input file with optional role (file:role), repeatable")
}

var replanCmd = &cobra.Command{
	Use:   "replan [file...]",
	Short: "Regenerate sprint-tasks.md preserving completed tasks",
	Long: `Replan regenerates sprint-tasks.md from input documents, preserving
completed [x] tasks from the existing file. Incomplete [ ] tasks are
replaced with new ones based on current input documents.`,
	Args: cobra.ArbitraryArgs,
	RunE: runReplan,
}

// runReplan implements the replan subcommand.
func runReplan(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	// Apply --input flag
	inputFlags, _ := cmd.Flags().GetStringArray("input")
	if len(inputFlags) > 0 {
		var flagInputs []plan.PlanInput
		for _, raw := range inputFlags {
			pi := parseInput(raw)
			data, readErr := os.ReadFile(pi.File)
			if readErr != nil {
				return fmt.Errorf("ralph: replan: read %s: %w", pi.File, readErr)
			}
			pi.Content = data
			pi.File = filepath.Base(pi.File)
			flagInputs = append(flagInputs, pi)
		}
		args = nil
		return runReplanWithInputs(cmd, cfg, flagInputs)
	}

	inputs, singleDoc, err := discoverInputs(cfg, args)
	if err != nil {
		return err
	}

	if len(inputs) == 0 {
		return fmt.Errorf("ralph: replan: no input files found")
	}

	for i := range inputs {
		inputs[i].Role = plan.ResolveRole(inputs[i].File, inputs[i].Role, singleDoc)
	}

	return runReplanWithInputs(cmd, cfg, inputs)
}

// runReplanWithInputs runs the replan pipeline with resolved inputs.
func runReplanWithInputs(cmd *cobra.Command, cfg *config.Config, inputs []plan.PlanInput) error {
	force, _ := cmd.Flags().GetBool("force")
	if err := checkSizeGuard(inputs, force); err != nil {
		return err
	}

	outputPath := resolveOutputPath(cmd, cfg)
	noReview := resolveNoReview(cmd)

	// Read existing sprint-tasks.md and extract completed tasks
	absPath := filepath.Join(cfg.ProjectRoot, outputPath)
	existingData, readErr := os.ReadFile(absPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return fmt.Errorf("ralph: replan: %s not found (use 'ralph plan' for initial generation)", outputPath)
		}
		return fmt.Errorf("ralph: replan: read existing: %w", readErr)
	}

	completedTasks := extractCompletedTasks(string(existingData))

	opts := plan.PlanOpts{
		Inputs:         inputs,
		OutputPath:     outputPath,
		Merge:          false, // AC #3: Merge = false
		NoReview:       noReview,
		MaxRetries:     cfg.PlanMaxRetries,
		CompletedTasks: completedTasks,
	}

	if _, err := runPlanSession(cmd, cfg, opts, "Replan..."); err != nil {
		return err
	}

	// Summary
	data, err := os.ReadFile(absPath)
	if err == nil {
		content := string(data)
		taskCount := countTasks(content)
		completedCount := strings.Count(content, config.TaskDone)
		openCount := strings.Count(content, config.TaskOpen)
		rn := reviewNote(noReview)
		color.Green("sprint-tasks.md обновлён: %d задач (%d выполнено, %d новых)%s | %s",
			taskCount, completedCount, openCount, rn, absPath)
	} else {
		color.Green("sprint-tasks.md regenerated successfully")
	}

	return nil
}

// extractCompletedTasks extracts lines with [x] markers and their context from sprint-tasks.md.
func extractCompletedTasks(content string) string {
	lines := strings.Split(content, "\n")
	var completed []string
	for _, line := range lines {
		if strings.Contains(line, config.TaskDone) {
			completed = append(completed, line)
		}
	}
	if len(completed) == 0 {
		return ""
	}
	return strings.Join(completed, "\n")
}
