//go:build integration

package gates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// --- Task 1.2: Real gates.Prompt integration with all actions ---

func TestPrompt_Integration_AllActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		input        string
		wantAction   string
		wantFeedback string
	}{
		{"approve", "a\n", config.ActionApprove, ""},
		{"skip", "s\n", config.ActionSkip, ""},
		{"quit", "q\n", config.ActionQuit, ""},
		{"retry with feedback", "r\nfeedback\n\n", config.ActionRetry, "feedback"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			gate := Gate{
				TaskText: "Setup project [GATE]",
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
			if decision.Feedback != tc.wantFeedback {
				t.Errorf("Feedback = %q, want %q", decision.Feedback, tc.wantFeedback)
			}

			output := out.String()
			if !strings.Contains(output, "HUMAN GATE") {
				t.Errorf("output missing HUMAN GATE header\ngot: %s", output)
			}
			if !strings.Contains(output, "Setup project [GATE]") {
				t.Errorf("output missing task text\ngot: %s", output)
			}
		})
	}
}

// --- Task 1.3: Emergency gate actions ---

func TestPrompt_Integration_EmergencyActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantAction string
	}{
		{"skip", "s\n", config.ActionSkip},
		{"retry", "r\nfix\n\n", config.ActionRetry},
		{"quit", "q\n", config.ActionQuit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			gate := Gate{
				TaskText:  "execute attempts exhausted (3/3)",
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

			output := out.String()
			if !strings.Contains(output, "EMERGENCY GATE") {
				t.Errorf("output missing EMERGENCY GATE header\ngot: %s", output)
			}
			if strings.Contains(output, "[a]pprove") {
				t.Errorf("output should NOT contain [a]pprove at emergency gate\ngot: %s", output)
			}
		})
	}
}

// TestPrompt_Integration_EmergencyApproveRejected verifies "a" rejected then "s" accepted.
func TestPrompt_Integration_EmergencyApproveRejected(t *testing.T) {
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
	if !strings.Contains(output, "not available") {
		t.Errorf("output missing 'not available' rejection message\ngot: %s", output)
	}
}

// --- Task 1.4: Invalid then valid input ---

func TestPrompt_Integration_InvalidThenValid(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-1 — Integration test",
		Reader:   strings.NewReader("x\na\n"),
		Writer:   &out,
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
	if !strings.Contains(output, "Unknown option: x") {
		t.Errorf("output missing 'Unknown option: x'\ngot: %s", output)
	}
}

// --- Task 1.5: Retry with multiline feedback ---

func TestPrompt_Integration_RetryMultilineFeedback(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	gate := Gate{
		TaskText: "TASK-2 — Multiline feedback",
		Reader:   strings.NewReader("r\nline1\nline2\n\n"),
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
	if decision.Feedback != "line1 line2" {
		t.Errorf("Feedback = %q, want %q", decision.Feedback, "line1 line2")
	}
}
