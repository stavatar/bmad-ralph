package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CLI flag constants for Claude CLI invocation.
// Defined as constants (not inline strings) for resilience to Claude CLI
// breaking changes — if a flag name changes, only one place to update.
// These are session-local (not in config/constants.go) because they're
// Claude CLI flags, not project sprint-tasks.md markers.
const (
	flagPrompt             = "-p"
	flagMaxTurns           = "--max-turns"
	flagModel              = "--model"
	flagOutputFormat       = "--output-format"
	flagResume             = "--resume"
	flagSkipPermissions    = "--dangerously-skip-permissions"
	flagAppendSystemPrompt = "--append-system-prompt"
	outputFormatJSON       = "json"

	// maxPromptArgLen is the threshold above which the prompt is delivered
	// via stdin instead of -p flag. Windows CreateProcess has a 32767-char
	// command line limit; we use a conservative threshold.
	maxPromptArgLen = 30000
)

// Options configures a Claude CLI session invocation.
// The caller (runner/bridge) fills this from config.Config values.
// Session package does NOT import config — receives everything via Options.
type Options struct {
	Command                    string  // Claude CLI path (config.ClaudeCommand)
	Dir                        string  // Working directory (config.ProjectRoot)
	Prompt                     string  // -p flag content (stdin for prompts > maxPromptArgLen)
	MaxTurns                   int     // --max-turns value (0 = omit)
	Model                      string  // --model value (empty = omit)
	OutputJSON                 bool    // --output-format json
	Resume                     string  // --resume session_id (empty = omit)
	DangerouslySkipPermissions bool    // --dangerously-skip-permissions
	AppendSystemPrompt         *string // Channel 1 delivery — critical rules via system prompt (nil = omit)
}

// RawResult contains raw output from a Claude CLI invocation.
// TRANSITIONAL: Story 1.8 adds SessionResult with parsed JSON fields
// (SessionID, Output, Duration). RawResult may become unexported or
// embedded — don't over-engineer it, keep it minimal.
type RawResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Execute invokes the Claude CLI with the given options and captures output.
// For prompts exceeding maxPromptArgLen, the prompt is delivered via stdin
// instead of -p flag to avoid the Windows 32K command line length limit.
func Execute(ctx context.Context, opts Options) (*RawResult, error) {
	args := buildArgs(opts)

	cmd := exec.CommandContext(ctx, opts.Command, args...)
	cmd.Dir = opts.Dir
	cmd.Env = os.Environ()

	if opts.Prompt != "" && len(opts.Prompt) > maxPromptArgLen {
		cmd.Stdin = strings.NewReader(opts.Prompt)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	result := &RawResult{
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		ExitCode: 0,
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf("session: claude: exit %d: %w", result.ExitCode, err)
		}
		return result, fmt.Errorf("session: claude: %w", err)
	}

	return result, nil
}

// buildArgs constructs the CLI argument slice from Options.
// Prompt is included as -p flag only when within maxPromptArgLen; longer
// prompts are delivered via stdin by Execute.
// Resume and Prompt are independent (not mutually exclusive) — Claude CLI
// supports both --resume and -p simultaneously for resume-with-prompt workflows.
func buildArgs(opts Options) []string {
	var args []string

	if opts.Resume != "" {
		args = append(args, flagResume, opts.Resume)
	}
	if opts.Prompt != "" {
		if len(opts.Prompt) <= maxPromptArgLen {
			// Short prompt: pass as positional arg after -p flag.
			args = append(args, flagPrompt, opts.Prompt)
		} else {
			// Long prompt: -p flag only (prompt delivered via stdin by Execute).
			args = append(args, flagPrompt)
		}
	}

	if opts.MaxTurns > 0 {
		args = append(args, flagMaxTurns, strconv.Itoa(opts.MaxTurns))
	}

	if opts.Model != "" {
		args = append(args, flagModel, opts.Model)
	}

	if opts.OutputJSON {
		args = append(args, flagOutputFormat, outputFormatJSON)
	}

	if opts.DangerouslySkipPermissions {
		args = append(args, flagSkipPermissions)
	}

	if opts.AppendSystemPrompt != nil {
		args = append(args, flagAppendSystemPrompt, *opts.AppendSystemPrompt)
	}

	return args
}
