package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute tasks from sprint-tasks.md",
	Long: `Run the execute → review loop on sprint-tasks.md.
Each task is executed in a fresh Claude Code session, reviewed,
and retried if findings are found.`,
	RunE: runRun,
}

func init() {
	runCmd.Flags().Int("max-turns", 0, "Max turns per Claude session (0 = use config/default)")
	runCmd.Flags().Bool("gates", false, "Enable human gates")
	runCmd.Flags().Int("every", 0, "Checkpoint gate every N tasks (0 = off)")
	runCmd.Flags().String("model", "", "Override Claude model for execute")
	runCmd.Flags().Bool("always-extract", false, "Extract knowledge after every execute")
}

func runRun(cmd *cobra.Command, args []string) error {
	flags := buildCLIFlags(cmd)

	cfg, err := config.Load(flags)
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	return runner.Run(cmd.Context(), cfg)
}

// buildCLIFlags converts Cobra flag values to config.CLIFlags.
// Only flags explicitly set by the user are populated (pointer non-nil).
func buildCLIFlags(cmd *cobra.Command) config.CLIFlags {
	var flags config.CLIFlags

	if cmd.Flags().Changed("max-turns") {
		v, _ := cmd.Flags().GetInt("max-turns")
		flags.MaxTurns = &v
	}
	if cmd.Flags().Changed("gates") {
		v, _ := cmd.Flags().GetBool("gates")
		flags.GatesEnabled = &v
	}
	if cmd.Flags().Changed("every") {
		v, _ := cmd.Flags().GetInt("every")
		flags.GatesCheckpoint = &v
	}
	if cmd.Flags().Changed("model") {
		v, _ := cmd.Flags().GetString("model")
		flags.ModelExecute = &v
	}
	if cmd.Flags().Changed("always-extract") {
		v, _ := cmd.Flags().GetBool("always-extract")
		flags.AlwaysExtract = &v
	}

	return flags
}
