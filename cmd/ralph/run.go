package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
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

	cfg.RunID = generateRunID()
	metrics, runErr := runner.Run(cmd.Context(), cfg)

	if metrics != nil {
		writeRunReport(cfg, metrics)
		fmt.Fprintln(cmd.OutOrStdout(), formatSummary(metrics, cfg))
	}

	return runErr
}

// writeRunReport writes the JSON run report to logDir. Errors are non-fatal (AC6).
func writeRunReport(cfg *config.Config, m *runner.RunMetrics) {
	jsonBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to marshal run report: %v\n", err)
		return
	}

	dir := filepath.Join(cfg.ProjectRoot, cfg.LogDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to create log directory: %v\n", err)
		return
	}

	path := filepath.Join(dir, fmt.Sprintf("ralph-run-%s.json", cfg.RunID))
	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to write run report: %v\n", err)
	}
}

// formatSummary returns a 4-line text summary of the run for stdout (AC3, AC4).
func formatSummary(m *runner.RunMetrics, cfg *config.Config) string {
	totalTasks := m.TasksCompleted + m.TasksFailed + m.TasksSkipped
	dur := time.Duration(m.DurationMs) * time.Millisecond
	minutes := int(dur.Minutes())
	seconds := int(dur.Seconds()) % 60

	// Line 1: task counts
	green := color.New(color.FgGreen).SprintFunc()
	line1 := green(fmt.Sprintf("Run complete: %d tasks (%d completed, %d skipped, %d failed)",
		totalTasks, m.TasksCompleted, m.TasksSkipped, m.TasksFailed))

	// Line 2: duration, cost, tokens
	var costStr, tokenStr string
	if m.CostUSD == 0 && m.InputTokens == 0 {
		costStr = "N/A"
		tokenStr = "N/A"
	} else {
		costStr = fmt.Sprintf("$%.2f", m.CostUSD)
		tokenStr = fmt.Sprintf("%s in / %s out", formatTokens(m.InputTokens), formatTokens(m.OutputTokens))
	}
	line2 := fmt.Sprintf("Duration: %dm %ds | Cost: %s | Tokens: %s", minutes, seconds, costStr, tokenStr)

	// Line 3: reviews
	var reviewCycles, totalFindings int
	var high, medium, low int
	for _, t := range m.Tasks {
		if len(t.Findings) > 0 || t.Gate != nil {
			reviewCycles++
			if len(t.Findings) > 0 {
				for _, f := range t.Findings {
					totalFindings++
					switch strings.ToUpper(f.Severity) {
					case "HIGH":
						high++
					case "MEDIUM":
						medium++
					case "LOW":
						low++
					}
				}
			}
		}
	}
	line3 := fmt.Sprintf("Reviews: %d cycles, %d findings (%dh/%dm/%dl)", reviewCycles, totalFindings, high, medium, low)

	// Line 4: report path
	reportPath := filepath.Join(cfg.LogDir, fmt.Sprintf("ralph-run-%s.json", cfg.RunID))
	line4 := fmt.Sprintf("Report: %s", reportPath)

	return fmt.Sprintf("%s\n%s\n%s\n%s", line1, line2, line3, line4)
}

// formatTokens formats a token count for display: exact for <1000, "XK" for thousands, "X.XM" for millions.
func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1000:
		k := float64(n) / 1000
		if k == float64(int(k)) && k >= 10 {
			return fmt.Sprintf("%dK", int(k))
		}
		return fmt.Sprintf("%.1fK", k)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// generateRunID returns a UUID v4 string (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx).
// Uses crypto/rand for secure random bytes.
func generateRunID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read always returns len(p), nil on supported platforms
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
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
