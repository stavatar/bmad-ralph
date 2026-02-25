package main

import (
	"context"
	"errors"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// Exit code constants matching PRD exit code table.
const (
	exitSuccess     = 0
	exitPartial     = 1
	exitUserQuit    = 2
	exitInterrupted = 3
	exitFatal       = 4
)

// exitCode maps an error to the appropriate process exit code.
// nil → 0, ExitCodeError → its Code, GateDecision(quit) → 2,
// context.Canceled → 3, everything else → 4.
func exitCode(err error) int {
	if err == nil {
		return exitSuccess
	}

	var exitErr *config.ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	var gate *config.GateDecision
	if errors.As(err, &gate) {
		if gate.Action == "quit" {
			return exitUserQuit
		}
	}

	if errors.Is(err, context.Canceled) {
		return exitInterrupted
	}

	return exitFatal
}
