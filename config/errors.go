package config

import (
	"errors"
	"fmt"
)

// Sentinel errors for control flow.
var (
	ErrNoTasks         = errors.New("no tasks found")
	ErrMaxRetries      = errors.New("max retries exceeded")
	ErrMaxReviewCycles = errors.New("max review cycles exceeded")
)

// ExitCodeError represents a Claude CLI exit with a specific code.
// Used in cmd/ralph for exit code mapping:
//
//	0=success, 1=partial, 2=user quit, 3=interrupted, 4=fatal
type ExitCodeError struct {
	Code    int
	Message string
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d: %s", e.Code, e.Message)
}

// GateDecision represents a user decision at a human gate.
// Action values: "approve", "skip", "quit", "retry".
type GateDecision struct {
	Action   string
	Feedback string
}

func (e *GateDecision) Error() string {
	return fmt.Sprintf("gate: %s", e.Action)
}
