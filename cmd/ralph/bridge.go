package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/bridge"
	"github.com/bmad-ralph/bmad-ralph/config"
)

const largePromptThreshold = 1500

var bridgeCmd = &cobra.Command{
	Use:   "bridge [story-files...]",
	Short: "Convert story files to sprint-tasks.md",
	Long: `Bridge converts BMad story files into a structured sprint-tasks.md
for execution by the run command.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runBridge,
}

func runBridge(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	taskCount, promptLines, err := bridge.Run(cmd.Context(), cfg, args)
	if err != nil {
		return err
	}

	if promptLines > largePromptThreshold {
		color.Yellow("Warning: large prompt (%d lines) — consider splitting story", promptLines)
	}

	fmt.Printf("Generated %d tasks in sprint-tasks.md\n", taskCount)

	return nil
}
