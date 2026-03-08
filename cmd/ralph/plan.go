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
	planCmd.Flags().String("output", "", "Output file path (overrides config)")
	planCmd.Flags().Bool("no-review", false, "Skip AI review of generated plan")
	planCmd.Flags().Bool("merge", false, "Merge into existing sprint-tasks.md")
	planCmd.Flags().Bool("force", false, "Bypass size warning")
	planCmd.Flags().StringArray("input", nil, "Input file with optional role (file:role), repeatable")
}

// bmadDefaultFiles are the well-known BMad documentation files to autodiscover.
var bmadDefaultFiles = []string{
	"prd.md",
	"architecture.md",
	"ux-design.md",
	"front-end-spec.md",
}

var planCmd = &cobra.Command{
	Use:   "plan [file...]",
	Short: "Generate sprint-tasks.md from input documents",
	Long: `Plan generates a sprint-tasks.md file from input documents using Claude.

Without arguments, autodiscovers BMad files in docs/ directory.
With arguments, uses the specified files as input.`,
	Args: cobra.ArbitraryArgs,
	RunE: runPlan,
}

// runPlan implements the plan subcommand.
func runPlan(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	// Apply --input flag: if set, override args with parsed inputs
	inputFlags, _ := cmd.Flags().GetStringArray("input")
	if len(inputFlags) > 0 {
		var flagInputs []plan.PlanInput
		for _, raw := range inputFlags {
			pi := parseInput(raw)
			data, readErr := os.ReadFile(pi.File)
			if readErr != nil {
				return fmt.Errorf("ralph: plan: read %s: %w", pi.File, readErr)
			}
			pi.Content = data
			pi.File = filepath.Base(pi.File)
			flagInputs = append(flagInputs, pi)
		}
		args = nil // suppress positional args
		// Use flagInputs directly below
		return runPlanWithInputs(cmd, cfg, flagInputs)
	}

	inputs, singleDoc, err := discoverInputs(cfg, args)
	if err != nil {
		return err
	}

	if len(inputs) == 0 {
		return fmt.Errorf("ralph: plan: no input files found")
	}

	// Resolve roles
	for i := range inputs {
		inputs[i].Role = plan.ResolveRole(inputs[i].File, inputs[i].Role, singleDoc)
	}

	return runPlanWithInputs(cmd, cfg, inputs)
}

// runPlanWithInputs runs the plan pipeline with resolved inputs.
func runPlanWithInputs(cmd *cobra.Command, cfg *config.Config, inputs []plan.PlanInput) error {
	force, _ := cmd.Flags().GetBool("force")
	if err := checkSizeGuard(inputs, force); err != nil {
		return err
	}

	outputPath := resolveOutputPath(cmd, cfg)
	noReview := resolveNoReview(cmd)

	merge := cfg.PlanMerge
	if cmd.Flags().Changed("merge") {
		merge, _ = cmd.Flags().GetBool("merge")
	}

	// Read existing content for merge mode (only cmd/ralph reads files, plan/ never does).
	var existingContent []byte
	if merge {
		absPath := filepath.Join(cfg.ProjectRoot, outputPath)
		data, readErr := os.ReadFile(absPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("ralph: plan: read existing: %w", readErr)
		}
		existingContent = data
	}

	opts := plan.PlanOpts{
		Inputs:          inputs,
		OutputPath:      outputPath,
		Merge:           merge,
		NoReview:        noReview,
		MaxRetries:      cfg.PlanMaxRetries,
		ExistingContent: existingContent,
	}

	if _, err := runPlanSession(cmd, cfg, opts, "Генерация плана..."); err != nil {
		return err
	}

	// Summary: read generated file, count tasks
	absPath := filepath.Join(cfg.ProjectRoot, outputPath)
	data, err := os.ReadFile(absPath)
	if err == nil {
		content := string(data)
		taskCount := countTasks(content)
		rn := reviewNote(noReview)
		if merge {
			existingTaskCount := countTasks(string(existingContent))
			newTasks := taskCount - existingTaskCount
			if newTasks < 0 {
				newTasks = 0
			}
			color.Green("sprint-tasks.md обновлён: +%d новых задач%s | %s", newTasks, rn, absPath)
		} else {
			color.Green("sprint-tasks.md готов: %d задач%s | %s", taskCount, rn, absPath)
		}
	} else {
		color.Green("sprint-tasks.md generated successfully")
	}

	return nil
}

// countTasks returns the total number of tasks (open + done) in content.
func countTasks(content string) int {
	return strings.Count(content, config.TaskOpen) + strings.Count(content, config.TaskDone)
}

// parseInput parses a "file:role" string into a PlanInput.
// If no colon is present, role is empty (default lookup applies).
func parseInput(s string) plan.PlanInput {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return plan.PlanInput{File: parts[0], Role: parts[1]}
	}
	return plan.PlanInput{File: parts[0]}
}

// discoverInputs resolves input files from args or autodiscovery.
// Returns the inputs with Content populated and whether single-doc mode is active.
func discoverInputs(cfg *config.Config, args []string) ([]plan.PlanInput, bool, error) {
	var inputs []plan.PlanInput

	if len(args) > 0 {
		// Explicit files from command line
		for _, f := range args {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, false, fmt.Errorf("ralph: plan: read %s: %w", f, err)
			}
			inputs = append(inputs, plan.PlanInput{
				File:    filepath.Base(f),
				Content: data,
			})
		}
	} else if len(cfg.PlanInputs) > 0 {
		// Config-specified inputs
		for _, pi := range cfg.PlanInputs {
			absPath := filepath.Join(cfg.ProjectRoot, pi.File)
			data, err := os.ReadFile(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue // skip missing files
				}
				return nil, false, fmt.Errorf("ralph: plan: read %s: %w", pi.File, err)
			}
			inputs = append(inputs, plan.PlanInput{
				File:    pi.File,
				Role:    pi.Role,
				Content: data,
			})
		}
	} else {
		// BMad autodiscovery
		for _, name := range bmadDefaultFiles {
			absPath := filepath.Join(cfg.ProjectRoot, "docs", name)
			data, err := os.ReadFile(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue // skip missing files
				}
				return nil, false, fmt.Errorf("ralph: plan: read %s: %w", name, err)
			}
			inputs = append(inputs, plan.PlanInput{
				File:    name,
				Content: data,
			})
		}
	}

	// Determine single-doc mode
	singleDoc := false
	switch cfg.PlanMode {
	case "single":
		singleDoc = true
	case "bmad":
		singleDoc = false
	case "auto":
		singleDoc = len(inputs) <= 1
	}

	return inputs, singleDoc, nil
}
