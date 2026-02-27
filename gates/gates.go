package gates

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/fatih/color"
)

// Gate holds the configuration for a single human gate prompt.
// TaskText is displayed in the prompt header.
// Reader and Writer are injectable for testing.
// Emergency: when true, uses emergency styling (red header, no [a]pprove option).
type Gate struct {
	TaskText  string
	Reader    io.Reader
	Writer    io.Writer
	Emergency bool
}

// readResult carries a single line or error from the scanner goroutine.
type readResult struct {
	text string
	err  error
}

// Prompt displays an interactive human gate and waits for user input.
// It returns a GateDecision on valid input (approve/skip/quit/retry),
// or an error for context cancellation, I/O errors, or EOF.
// When retry is selected, Prompt collects multi-line feedback input
// until an empty line is entered. Lines are trimmed and joined with space.
// All errors are wrapped with "gates: prompt:" prefix.
func Prompt(ctx context.Context, gate Gate) (*config.GateDecision, error) {
	scanner := bufio.NewScanner(gate.Reader)
	lineCh := make(chan readResult, 1)

	go func() {
		defer close(lineCh)
		for scanner.Scan() {
			lineCh <- readResult{text: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			lineCh <- readResult{err: err}
		} else {
			lineCh <- readResult{err: io.EOF}
		}
	}()

	errColor := color.New(color.FgRed)

	var header *color.Color
	if gate.Emergency {
		header = color.New(color.FgRed, color.Bold)
	} else {
		header = color.New(color.FgCyan, color.Bold)
	}

	for {
		if gate.Emergency {
			header.Fprintf(gate.Writer, "🚨 EMERGENCY GATE: ")
		} else {
			header.Fprintf(gate.Writer, "🚦 HUMAN GATE: ")
		}
		fmt.Fprintln(gate.Writer, gate.TaskText)
		if gate.Emergency {
			fmt.Fprintln(gate.Writer, "   [r]etry with feedback  [s]kip  [q]uit")
		} else {
			fmt.Fprintln(gate.Writer, "   [a]pprove  [r]etry with feedback  [s]kip  [q]uit")
		}
		fmt.Fprint(gate.Writer, "> ")

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("gates: prompt: %w", ctx.Err())
		case r, ok := <-lineCh:
			if !ok || r.err != nil {
				err := r.err
				if !ok {
					err = io.EOF
				}
				return nil, fmt.Errorf("gates: prompt: %w", err)
			}

			input := strings.TrimSpace(strings.ToLower(r.text))
			switch input {
			case "a":
				if gate.Emergency {
					errColor.Fprintln(gate.Writer, "Approve not available at emergency gate")
					continue
				}
				return &config.GateDecision{Action: config.ActionApprove}, nil
			case "r":
				// Feedback collection flow (Story 5.3)
				fmt.Fprintln(gate.Writer, "Enter feedback (empty line to submit):")
				fmt.Fprint(gate.Writer, "> ")

				var feedbackLines []string
				for {
					select {
					case <-ctx.Done():
						return nil, fmt.Errorf("gates: prompt: %w", ctx.Err())
					case r, ok := <-lineCh:
						if !ok || r.err != nil {
							err := r.err
							if !ok {
								err = io.EOF
							}
							return nil, fmt.Errorf("gates: prompt: %w", err)
						}
						trimmed := strings.TrimSpace(r.text)
						if trimmed == "" {
							// Empty line = submit feedback
							feedback := strings.Join(feedbackLines, " ")
							return &config.GateDecision{
								Action:   config.ActionRetry,
								Feedback: feedback,
							}, nil
						}
						feedbackLines = append(feedbackLines, trimmed)
						fmt.Fprint(gate.Writer, "> ") // prompt for next line
					}
				}
			case "s":
				return &config.GateDecision{Action: config.ActionSkip}, nil
			case "q":
				return &config.GateDecision{Action: config.ActionQuit}, nil
			default:
				if input == "" {
					errColor.Fprintln(gate.Writer, "Unknown option: (empty)")
				} else {
					errColor.Fprintf(gate.Writer, "Unknown option: %s\n", input)
				}
			}
		}
	}
}
