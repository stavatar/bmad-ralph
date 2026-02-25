package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/bridge"
	"github.com/bmad-ralph/bmad-ralph/config"
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge [story-files...]",
	Short: "Convert story files to sprint-tasks.md",
	Long: `Bridge converts BMad story files into a structured sprint-tasks.md
for execution by the run command.`,
	RunE: runBridge,
}

func runBridge(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	return bridge.Run(cmd.Context(), cfg, args)
}
