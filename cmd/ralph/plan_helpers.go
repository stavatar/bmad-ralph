package main

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/plan"
)

// checkSizeGuard checks input size and returns an error if --force is not set.
func checkSizeGuard(inputs []plan.PlanInput, force bool) error {
	if warn, msg := plan.CheckSize(inputs); warn {
		if !force {
			color.Yellow("%s", msg)
			color.Yellow("  Используйте --force для продолжения")
			return &config.ExitCodeError{Code: 2, Message: msg}
		}
		color.Yellow("WARNING: %s", msg)
	}
	return nil
}

// resolveOutputPath returns the output path from --output flag or config default.
func resolveOutputPath(cmd *cobra.Command, cfg *config.Config) string {
	outputPath := cfg.PlanOutputPath
	if cmd.Flags().Changed("output") {
		outputPath, _ = cmd.Flags().GetString("output")
	}
	return outputPath
}

// resolveNoReview returns the --no-review flag value.
func resolveNoReview(cmd *cobra.Command) bool {
	noReview := false
	if cmd.Flags().Changed("no-review") {
		noReview, _ = cmd.Flags().GetBool("no-review")
	}
	return noReview
}

// runPlanSession runs plan.Run with progress output and timing.
// Returns elapsed duration on success.
func runPlanSession(cmd *cobra.Command, cfg *config.Config, opts plan.PlanOpts, progressMsg string) (time.Duration, error) {
	fmt.Fprint(cmd.OutOrStdout(), progressMsg)
	start := time.Now()

	if err := plan.Run(cmd.Context(), cfg, opts); err != nil {
		fmt.Fprintln(cmd.OutOrStdout())
		return 0, err
	}

	elapsed := time.Since(start).Round(time.Second)
	fmt.Fprintf(cmd.OutOrStdout(), " (%s)\n", elapsed)
	return elapsed, nil
}

// reviewNote returns " (review пропущен)" if noReview is true.
func reviewNote(noReview bool) string {
	if noReview {
		return " (review пропущен)"
	}
	return ""
}
