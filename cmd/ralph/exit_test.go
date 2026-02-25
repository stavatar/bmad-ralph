package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

func TestExitCode_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, exitSuccess},
		{"ExitCodeError partial", &config.ExitCodeError{Code: 1, Message: "partial"}, exitPartial},
		{"ExitCodeError fatal", &config.ExitCodeError{Code: 4, Message: "crash"}, exitFatal},
		{"GateDecision quit", &config.GateDecision{Action: "quit", Feedback: ""}, exitUserQuit},
		{"GateDecision skip", &config.GateDecision{Action: "skip"}, exitFatal},
		{"context.Canceled", context.Canceled, exitInterrupted},
		{"generic error", fmt.Errorf("something broke"), exitFatal},
		// Wrapped errors — realistic multi-layer call stack
		{"wrapped ExitCodeError", fmt.Errorf("runner: %w", &config.ExitCodeError{Code: 1, Message: "limits"}), exitPartial},
		{"wrapped GateDecision quit", fmt.Errorf("gates: %w", &config.GateDecision{Action: "quit"}), exitUserQuit},
		{"wrapped context.Canceled", fmt.Errorf("runner: execute: %w", context.Canceled), exitInterrupted},
		{"double wrapped Canceled", fmt.Errorf("ralph: %w", fmt.Errorf("runner: %w", context.Canceled)), exitInterrupted},
		{"double wrapped ExitCodeError", fmt.Errorf("ralph: %w", fmt.Errorf("runner: %w", &config.ExitCodeError{Code: 2, Message: "quit"})), exitUserQuit},
		// context.DeadlineExceeded — architecture says no timeouts, maps to exitFatal defensively
		{"context.DeadlineExceeded", context.DeadlineExceeded, exitFatal},
		// Sentinel errors — map to exitFatal
		{"ErrMaxRetries", config.ErrMaxRetries, exitFatal},
		{"ErrNoTasks", config.ErrNoTasks, exitFatal},
		{"wrapped ErrMaxRetries", fmt.Errorf("runner: %w", config.ErrMaxRetries), exitFatal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitCode(tt.err)
			if got != tt.want {
				t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestExitCode_ZeroValueExitCodeError(t *testing.T) {
	// Zero-value ExitCodeError has Code=0 → exitSuccess
	err := &config.ExitCodeError{}
	if got := exitCode(err); got != exitSuccess {
		t.Errorf("exitCode(ExitCodeError{}) = %d, want %d", got, exitSuccess)
	}
}

func TestExitCode_ZeroValueGateDecision(t *testing.T) {
	// Zero-value GateDecision has Action="" → not "quit" → falls through to exitFatal
	err := &config.GateDecision{}
	if got := exitCode(err); got != exitFatal {
		t.Errorf("exitCode(GateDecision{}) = %d, want %d", got, exitFatal)
	}
}
