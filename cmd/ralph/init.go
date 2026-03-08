package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

//go:embed prompts/init.md
var initPrompt string

// fileSeparator is the delimiter between prd.md and architecture.md in Claude output.
const fileSeparator = "===FILE_SEPARATOR==="

var initCmd = &cobra.Command{
	Use:   "init <description>",
	Short: "Generate minimal project docs from a description",
	Long: `Init generates docs/prd.md and docs/architecture.md from a brief project
description, enabling immediate use of 'ralph plan docs/'.`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

// runInit implements the init subcommand.
func runInit(cmd *cobra.Command, args []string) error {
	description := args[0]

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("ralph: load config: %w", err)
	}

	// Assemble prompt
	prompt := strings.ReplaceAll(initPrompt, "__DESCRIPTION__", description)

	fmt.Fprint(cmd.OutOrStdout(), "Генерация документов...")
	start := time.Now()

	// Run Claude session
	raw, execErr := session.Execute(cmd.Context(), session.Options{
		Command:                    cfg.ClaudeCommand,
		Dir:                        cfg.ProjectRoot,
		Prompt:                     prompt,
		MaxTurns:                   cfg.MaxTurns,
		OutputJSON:                 true,
		DangerouslySkipPermissions: true,
	})
	if execErr != nil {
		fmt.Fprintln(cmd.OutOrStdout())
		return fmt.Errorf("ralph: init: %w", execErr)
	}

	result, parseErr := session.ParseResult(raw, 0)
	if parseErr != nil {
		fmt.Fprintln(cmd.OutOrStdout())
		return fmt.Errorf("ralph: init: parse: %w", parseErr)
	}

	elapsed := time.Since(start).Round(time.Second)
	fmt.Fprintf(cmd.OutOrStdout(), " (%s)\n", elapsed)

	// Split output into two files
	prdContent, archContent, splitErr := splitInitOutput(result.Output)
	if splitErr != nil {
		return fmt.Errorf("ralph: init: %w", splitErr)
	}

	// Ensure docs/ directory exists
	docsDir := filepath.Join(cfg.ProjectRoot, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return fmt.Errorf("ralph: init: create docs dir: %w", err)
	}

	// Write files
	prdPath := filepath.Join(docsDir, "prd.md")
	archPath := filepath.Join(docsDir, "architecture.md")

	if err := os.WriteFile(prdPath, []byte(prdContent), 0o644); err != nil {
		return fmt.Errorf("ralph: init: write prd.md: %w", err)
	}
	if err := os.WriteFile(archPath, []byte(archContent), 0o644); err != nil {
		return fmt.Errorf("ralph: init: write architecture.md: %w", err)
	}

	// AC #4: instruction message
	color.Green("Документы готовы. Запустите: ralph plan docs/")

	return nil
}

// splitInitOutput splits Claude output into prd and architecture content
// using the FILE_SEPARATOR delimiter.
func splitInitOutput(output string) (prd, arch string, err error) {
	parts := strings.SplitN(output, fileSeparator, 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("split output: separator %q not found", fileSeparator)
	}
	prd = strings.TrimSpace(parts[0])
	arch = strings.TrimSpace(parts[1])
	if prd == "" {
		return "", "", fmt.Errorf("split output: prd.md content is empty")
	}
	if arch == "" {
		return "", "", fmt.Errorf("split output: architecture.md content is empty")
	}
	return prd, arch, nil
}
