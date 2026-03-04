package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/bridge"
	"github.com/bmad-ralph/bmad-ralph/config"
)

const (
	largePromptThreshold = 1500
	// maxBatchBytes limits total story content per Claude call (~80KB).
	// Claude context includes system prompt + CLAUDE.md + tools (~120K tokens),
	// leaving ~80K tokens for story content. At ~3.5 chars/token, ~80KB is safe.
	maxBatchBytes = 80000
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge [story-files...]",
	Short: "Convert story files to sprint-tasks.md",
	Long: `Bridge converts BMad story files into a structured sprint-tasks.md
for execution by the run command.

If no story files are provided, bridge auto-discovers *.md files in
StoriesDir (default: docs/sprint-artifacts) relative to the project root.`,
	Args: cobra.ArbitraryArgs,
	RunE: runBridge,
}

func runBridge(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	files := args
	if len(files) == 0 {
		pattern := filepath.Join(cfg.ProjectRoot, cfg.StoriesDir, "*.md")
		matches, globErr := filepath.Glob(pattern)
		if globErr != nil {
			return fmt.Errorf("ralph: bridge: glob stories: %w", globErr)
		}
		if len(matches) == 0 {
			return fmt.Errorf("ralph: bridge: no story files found in %q and no files specified; use: ralph bridge <file.md>", filepath.Join(cfg.ProjectRoot, cfg.StoriesDir))
		}
		sort.Strings(matches)
		files = matches
	}

	batches := splitBySize(files, maxBatchBytes)

	var totalTasks int
	for i, batch := range batches {
		taskCount, promptLines, runErr := bridge.Run(cmd.Context(), cfg, batch)
		if runErr != nil {
			return runErr
		}
		if promptLines > largePromptThreshold {
			color.Yellow("Warning: large prompt (%d lines) in batch %d/%d", promptLines, i+1, len(batches))
		}
		totalTasks = taskCount
	}

	fmt.Printf("Generated %d tasks in sprint-tasks.md\n", totalTasks)

	return nil
}

// splitBySize groups files into batches where total file size per batch
// stays under maxBytes. Files are added sequentially; a file that would
// exceed the limit starts a new batch.
func splitBySize(files []string, maxBytes int64) [][]string {
	if len(files) == 0 {
		return nil
	}
	var batches [][]string
	var current []string
	var currentSize int64

	for _, f := range files {
		info, err := os.Stat(f)
		size := int64(0)
		if err == nil {
			size = info.Size()
		}
		if len(current) > 0 && currentSize+size > maxBytes {
			batches = append(batches, current)
			current = nil
			currentSize = 0
		}
		current = append(current, f)
		currentSize += size
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}
