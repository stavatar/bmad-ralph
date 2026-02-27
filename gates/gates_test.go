package gates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
)

func TestPrompt_ValidActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantAction string
	}{
		{"approve lowercase", "a\n", config.ActionApprove},
		{"skip lowercase", "s\n", config.ActionSkip},
		{"quit lowercase", "q\n", config.ActionQuit},
		{"retry lowercase", "r\n\n", config.ActionRetry},
		{"approve uppercase", "A\n", config.ActionApprove},
		{"approve with whitespace", "  a  \n", config.ActionApprove},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			gate := Gate{
				TaskText: "TASK-1 — Setup project structure",
				Reader:   strings.NewReader(tc.input),
				Writer:   &out,
			}

			decision, err := Prompt(context.Background(), gate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision == nil {
				t.Fatal("expected non-nil decision")
			}
			if decision.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", decision.Action, tc.wantAction)
			}
			if decision.Feedback != "" {
				t.Errorf("Feedback = %q, want empty", decision.Feedback)
			}
		})
	}
}

func TestPrompt_InvalidThenValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		input          string
		wantAction     string
		wantErrStrings []string
		wantErrCount   int // expected count of "Unknown option:" in output
	}{
		{
			name:           "single invalid then approve",
			input:          "x\na\n",
			wantAction:     config.ActionApprove,
			wantErrStrings: []string{"Unknown option: x"},
			wantErrCount:   1,
		},
		{
			name:           "multiple invalid then skip",
			input:          "z\n\nhello\ns\n",
			wantAction:     config.ActionSkip,
			wantErrStrings: []string{"Unknown option: z", "Unknown option: (empty)", "Unknown option: hello"},
			wantErrCount:   3, // z, empty line, hello
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			gate := Gate{
				TaskText: "TASK-2 — Build feature",
				Reader:   strings.NewReader(tc.input),
				Writer:   &out,
			}

			decision, err := Prompt(context.Background(), gate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision == nil {
				t.Fatal("expected non-nil decision")
			}
			if decision.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", decision.Action, tc.wantAction)
			}
			if decision.Feedback != "" {
				t.Errorf("Feedback = %q, want empty", decision.Feedback)
			}

			output := out.String()
			for _, want := range tc.wantErrStrings {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\ngot: %s", want, output)
				}
			}
			if got := strings.Count(output, "Unknown option:"); got < tc.wantErrCount {
				t.Errorf("Unknown option count = %d, want >= %d\ngot: %s", got, tc.wantErrCount, output)
			}
		})
	}
}

func TestPrompt_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Use io.Pipe so the reader blocks forever — ensures ctx.Done() wins the select race.
	pr, pw := io.Pipe()
	defer pw.Close()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-3 — Blocked task",
		Reader:   pr,
		Writer:   &out,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	decision, err := Prompt(ctx, gate)
	if decision != nil {
		t.Errorf("expected nil decision, got %+v", decision)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if !strings.Contains(err.Error(), "gates: prompt:") {
		t.Errorf("error missing prefix, got %v", err)
	}
}

func TestPrompt_OutputFormat(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-42 — Important task",
		Reader:   strings.NewReader("a\n"),
		Writer:   &out,
	}

	decision, err := Prompt(context.Background(), gate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}

	output := out.String()
	required := []string{
		"TASK-42 — Important task",
		"[a]pprove",
		"[s]kip",
		"[q]uit",
		"[r]etry",
		"HUMAN GATE",
		"> ",
	}
	for _, want := range required {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\ngot: %s", want, output)
		}
	}
}

func TestPrompt_EOF(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-5 — EOF task",
		Reader:   strings.NewReader(""),
		Writer:   &out,
	}

	decision, err := Prompt(context.Background(), gate)
	if decision != nil {
		t.Errorf("expected nil decision, got %+v", decision)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF, got %v", err)
	}
	if !strings.Contains(err.Error(), "gates: prompt:") {
		t.Errorf("error missing prefix, got %v", err)
	}
}

// errReader returns a fixed error on every Read call.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestPrompt_ScannerError(t *testing.T) {
	t.Parallel()

	readErr := fmt.Errorf("device failure")
	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-6 — Scanner error",
		Reader:   errReader{err: readErr},
		Writer:   &out,
	}

	decision, err := Prompt(context.Background(), gate)
	if decision != nil {
		t.Errorf("expected nil decision, got %+v", decision)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gates: prompt:") {
		t.Errorf("error missing prefix, got %v", err)
	}
	if !strings.Contains(err.Error(), "device failure") {
		t.Errorf("error missing inner cause, got %v", err)
	}
}

// --- Story 5.3: Retry with feedback tests ---

// TestPrompt_RetryWithFeedback verifies feedback collection on retry (AC1, AC2, AC3).
func TestPrompt_RetryWithFeedback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		input             string
		wantFeedback      string
		wantMinPromptCount int // main "> " + initial feedback "> " + per-line "> " prompts
	}{
		{
			name:              "single line feedback",
			input:             "r\nfix validation\n\n",
			wantFeedback:      "fix validation",
			wantMinPromptCount: 3, // main + initial feedback + 1 per-line
		},
		{
			name:              "multi line feedback joined with space",
			input:             "r\nline1\nline2\n\n",
			wantFeedback:      "line1 line2",
			wantMinPromptCount: 4, // main + initial feedback + 2 per-line
		},
		{
			name:              "whitespace trimmed",
			input:             "r\n  spaced  \n\n",
			wantFeedback:      "spaced",
			wantMinPromptCount: 3, // main + initial feedback + 1 per-line
		},
		{
			name:              "three lines joined",
			input:             "r\nadd tests\nfix lint\nupdate docs\n\n",
			wantFeedback:      "add tests fix lint update docs",
			wantMinPromptCount: 5, // main + initial feedback + 3 per-line
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			gate := Gate{
				TaskText: "TASK-10 — Retry feedback",
				Reader:   strings.NewReader(tc.input),
				Writer:   &out,
			}

			decision, err := Prompt(context.Background(), gate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision == nil {
				t.Fatal("expected non-nil decision")
			}
			if decision.Action != config.ActionRetry {
				t.Errorf("Action = %q, want %q", decision.Action, config.ActionRetry)
			}
			if decision.Feedback != tc.wantFeedback {
				t.Errorf("Feedback = %q, want %q", decision.Feedback, tc.wantFeedback)
			}

			output := out.String()
			if !strings.Contains(output, "Enter feedback (empty line to submit):") {
				t.Errorf("output missing feedback prompt\ngot: %s", output)
			}
			// Verify per-line "> " prompts match expected count
			promptCount := strings.Count(output, "> ")
			if promptCount < tc.wantMinPromptCount {
				t.Errorf("prompt count = %d, want >= %d\ngot: %s", promptCount, tc.wantMinPromptCount, output)
			}
		})
	}
}

// TestPrompt_RetryEmptyFeedback verifies immediate empty line returns empty feedback (AC2).
func TestPrompt_RetryEmptyFeedback(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-11 — Retry empty feedback",
		Reader:   strings.NewReader("r\n\n"),
		Writer:   &out,
	}

	decision, err := Prompt(context.Background(), gate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if decision.Action != config.ActionRetry {
		t.Errorf("Action = %q, want %q", decision.Action, config.ActionRetry)
	}
	if decision.Feedback != "" {
		t.Errorf("Feedback = %q, want empty", decision.Feedback)
	}
}

// TestPrompt_RetryFeedbackEOF verifies EOF during feedback returns error (AC8).
func TestPrompt_RetryFeedbackEOF(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-12 — Retry feedback EOF",
		Reader:   strings.NewReader("r\npartial"),
		Writer:   &out,
	}

	decision, err := Prompt(context.Background(), gate)
	if decision != nil {
		t.Errorf("expected nil decision, got %+v", decision)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF, got %v", err)
	}
	if !strings.Contains(err.Error(), "gates: prompt:") {
		t.Errorf("error missing prefix, got %v", err)
	}
}

// TestPrompt_RetryFeedbackContextCancel verifies context cancellation during feedback (AC8).
func TestPrompt_RetryFeedbackContextCancel(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	defer pw.Close()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-13 — Retry feedback cancel",
		Reader:   pr,
		Writer:   &out,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Goroutine writes "r\n" to enter feedback mode, then cancels context.
	go func() {
		// Write "r\n" so scanner delivers the "r" line
		_, _ = pw.Write([]byte("r\n"))
		// Wait for feedback prompt to appear before cancelling
		for i := 0; i < 200; i++ {
			if strings.Contains(out.String(), "Enter feedback") {
				break
			}
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()

	decision, err := Prompt(ctx, gate)
	if decision != nil {
		t.Errorf("expected nil decision, got %+v", decision)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if !strings.Contains(err.Error(), "gates: prompt:") {
		t.Errorf("error missing prefix, got %v", err)
	}
}

// --- Story 5.5: Emergency gate tests ---

// TestPrompt_EmergencyStyle verifies emergency gate uses distinct styling (AC4):
// 🚨 header, no [a]pprove option, "a" rejected with error message.
func TestPrompt_EmergencyStyle(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText:  "execute attempts exhausted (3/3)",
		Reader:    strings.NewReader("a\ns\n"),
		Writer:    &out,
		Emergency: true,
	}

	decision, err := Prompt(context.Background(), gate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if decision.Action != config.ActionSkip {
		t.Errorf("Action = %q, want %q", decision.Action, config.ActionSkip)
	}

	output := out.String()
	// AC4: emergency header
	if !strings.Contains(output, "EMERGENCY GATE:") {
		t.Errorf("output missing EMERGENCY GATE header\ngot: %s", output)
	}
	// AC4: no [a]pprove option in the options line
	if strings.Contains(output, "[a]pprove") {
		t.Errorf("output should NOT contain [a]pprove at emergency gate\ngot: %s", output)
	}
	// AC4: [s]kip present
	if !strings.Contains(output, "[s]kip") {
		t.Errorf("output missing [s]kip\ngot: %s", output)
	}
	// AC4: "a" rejected with error message
	if !strings.Contains(output, "Approve not available") {
		t.Errorf("output missing 'Approve not available' error message\ngot: %s", output)
	}
}

// TestPrompt_EmergencyValidActions verifies s/r/q all work at emergency gate (AC4).
func TestPrompt_EmergencyValidActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantAction string
	}{
		{"skip", "s\n", config.ActionSkip},
		{"retry", "r\n\n", config.ActionRetry},
		{"quit", "q\n", config.ActionQuit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			gate := Gate{
				TaskText:  "review cycles exhausted (3/3)",
				Reader:    strings.NewReader(tc.input),
				Writer:    &out,
				Emergency: true,
			}

			decision, err := Prompt(context.Background(), gate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision == nil {
				t.Fatal("expected non-nil decision")
			}
			if decision.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", decision.Action, tc.wantAction)
			}
		})
	}
}

// TestPrompt_EmergencyNoApprove verifies "a" rejected then "q" accepted at emergency gate (AC4).
func TestPrompt_EmergencyNoApprove(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText:  "execute attempts exhausted (5/5)",
		Reader:    strings.NewReader("a\nq\n"),
		Writer:    &out,
		Emergency: true,
	}

	decision, err := Prompt(context.Background(), gate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if decision.Action != config.ActionQuit {
		t.Errorf("Action = %q, want %q", decision.Action, config.ActionQuit)
	}

	output := out.String()
	if !strings.Contains(output, "not available") {
		t.Errorf("output missing 'not available' error for 'a' input\ngot: %s", output)
	}
}

// TestPrompt_NormalUnchanged verifies non-emergency gate unchanged: 🚦 header, [a]pprove works (AC4).
func TestPrompt_NormalUnchanged(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText:  "TASK-99 — Normal task",
		Reader:    strings.NewReader("a\n"),
		Writer:    &out,
		Emergency: false,
	}

	decision, err := Prompt(context.Background(), gate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision == nil {
		t.Fatal("expected non-nil decision")
	}
	if decision.Action != config.ActionApprove {
		t.Errorf("Action = %q, want %q", decision.Action, config.ActionApprove)
	}

	output := out.String()
	if !strings.Contains(output, "HUMAN GATE:") {
		t.Errorf("output missing HUMAN GATE header\ngot: %s", output)
	}
	if !strings.Contains(output, "[a]pprove") {
		t.Errorf("output missing [a]pprove\ngot: %s", output)
	}
}
