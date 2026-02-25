package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ralph",
	Version: version,
	Short:   "Orchestrate Claude Code sessions for autonomous development",
	Long: `Ralph orchestrates Claude Code sessions in an execute → review loop
for autonomous software development. It reads sprint-tasks.md and executes
each task through Claude Code, then reviews the result.`,
}

func main() {
	os.Exit(run())
}

// run is the real entry point. Separated from main() so defers execute
// before os.Exit and for testability.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Double-signal handler: second Ctrl+C force-exits (NFR13).
	go func() {
		<-ctx.Done()
		// Context canceled by first signal. Listen for second.
		stop()
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		color.Red("\nForce exit (second interrupt)")
		os.Exit(exitInterrupted)
	}()

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	err := rootCmd.ExecuteContext(ctx)
	code := exitCode(err)

	if err != nil {
		switch code {
		case exitInterrupted:
			color.Yellow("\nInterrupted")
		default:
			color.Red("Error: %v", err)
		}
	}

	return code
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(bridgeCmd)
}
