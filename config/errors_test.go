package config

import (
	"errors"
	"fmt"
	"testing"
)

// M1 fix: Renamed to follow Test<Type>_<Method>_<Scenario> convention (ref: Dev Notes)
// L2 fix: Added double-wrapped error unwrapping cases
func TestErrNoTasks_Is_WrappedUnwraps(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{"ErrNoTasks unwraps", fmt.Errorf("config: scan: %w", ErrNoTasks), ErrNoTasks, true},
		{"ErrMaxRetries unwraps", fmt.Errorf("runner: loop: %w", ErrMaxRetries), ErrMaxRetries, true},
		{"ErrNoTasks wrapping pattern", fmt.Errorf("config: load: %w", ErrNoTasks), ErrNoTasks, true},
		{"ErrNoTasks is not ErrMaxRetries", ErrNoTasks, ErrMaxRetries, false},
		{"wrapped sentinel not other sentinel", fmt.Errorf("wrap: %w", ErrNoTasks), ErrMaxRetries, false},
		{"ErrNoTasks double-wrapped", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", ErrNoTasks)), ErrNoTasks, true},
		{"ErrMaxRetries double-wrapped", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", ErrMaxRetries)), ErrMaxRetries, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, tt.target); got != tt.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		})
	}
}

// M2 fix: Converted to table-driven
// L1 fix: Added zero value test case
func TestExitCodeError_As_Extraction(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    int
		wantMessage string
	}{
		{"code 1 partial", fmt.Errorf("session: claude: %w", &ExitCodeError{Code: 1, Message: "partial"}), 1, "partial"},
		{"code 2 user quit", fmt.Errorf("session: claude: %w", &ExitCodeError{Code: 2, Message: "user quit"}), 2, "user quit"},
		{"code 4 fatal", fmt.Errorf("runner: execute: %w", &ExitCodeError{Code: 4, Message: "fatal"}), 4, "fatal"},
		{"zero value", fmt.Errorf("session: %w", &ExitCodeError{}), 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target *ExitCodeError
			if !errors.As(tt.err, &target) {
				t.Fatal("errors.As failed to extract *ExitCodeError")
			}
			if target.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", target.Code, tt.wantCode)
			}
			if target.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", target.Message, tt.wantMessage)
			}
		})
	}
}

// M2 fix: Converted to table-driven
// L1 fix: Added zero value test case
func TestGateDecision_As_Extraction(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantAction   string
		wantFeedback string
	}{
		{"quit with feedback", fmt.Errorf("gates: prompt: %w", &GateDecision{Action: "quit", Feedback: "done"}), "quit", "done"},
		{"approve no feedback", fmt.Errorf("gates: prompt: %w", &GateDecision{Action: "approve"}), "approve", ""},
		{"retry with feedback", fmt.Errorf("runner: gate: %w", &GateDecision{Action: "retry", Feedback: "fix auth"}), "retry", "fix auth"},
		{"zero value", fmt.Errorf("gates: %w", &GateDecision{}), "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target *GateDecision
			if !errors.As(tt.err, &target) {
				t.Fatal("errors.As failed to extract *GateDecision")
			}
			if target.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", target.Action, tt.wantAction)
			}
			if target.Feedback != tt.wantFeedback {
				t.Errorf("Feedback = %q, want %q", target.Feedback, tt.wantFeedback)
			}
		})
	}
}

func TestExitCodeError_Error_Format(t *testing.T) {
	tests := []struct {
		name string
		err  *ExitCodeError
		want string
	}{
		{"code 0", &ExitCodeError{Code: 0, Message: "success"}, "exit code 0: success"},
		{"code 1", &ExitCodeError{Code: 1, Message: "partial"}, "exit code 1: partial"},
		{"code 4", &ExitCodeError{Code: 4, Message: "fatal error"}, "exit code 4: fatal error"},
		{"zero value", &ExitCodeError{}, "exit code 0: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGateDecision_Error_Format(t *testing.T) {
	tests := []struct {
		name string
		err  *GateDecision
		want string
	}{
		{"approve", &GateDecision{Action: "approve"}, "gate: approve"},
		{"quit with feedback", &GateDecision{Action: "quit", Feedback: "user done"}, "gate: quit"},
		{"retry", &GateDecision{Action: "retry", Feedback: "fix this"}, "gate: retry"},
		{"zero value", &GateDecision{}, "gate: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// M1 fix: Renamed to follow Test<Type>_<Method>_<Scenario> convention
func TestErrNoTasks_As_NegativeCrossType(t *testing.T) {
	wrapped := fmt.Errorf("config: scan: %w", ErrNoTasks)
	var exitErr *ExitCodeError
	if errors.As(wrapped, &exitErr) {
		t.Error("errors.As(wrappedSentinel, &ExitCodeError) = true, want false")
	}
	var gateErr *GateDecision
	if errors.As(wrapped, &gateErr) {
		t.Error("errors.As(wrappedSentinel, &GateDecision) = true, want false")
	}
}
