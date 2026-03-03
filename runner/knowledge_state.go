package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrBadFormat indicates distillation output is unparseable (missing markers or bad structure).
// ONE automatic retry is attempted before escalating to human gate (H4).
var ErrBadFormat = errors.New("bad format")

// ErrValidationFailed indicates distillation output is parseable but fails validation criteria.
// Distinct from ErrBadFormat: NO free retry — direct human gate escalation.
var ErrValidationFailed = errors.New("validation failed")

// DistillFunc is the injectable function type for distillation.
// Same pattern as ReviewFunc, GatePromptFunc, ResumeExtractFunc.
type DistillFunc func(ctx context.Context, state *DistillState) error

// DistillMetrics tracks effectiveness metrics after distillation (AC #7).
// Stored in DistillState.Metrics for trend tracking.
type DistillMetrics struct {
	EntriesBefore        int    `json:"entries_before"`
	EntriesAfter         int    `json:"entries_after"`
	StaleRemoved         int    `json:"stale_removed"`
	CategoriesPreserved  int    `json:"categories_preserved"`
	CategoriesTotal      int    `json:"categories_total"`
	NeedsFormattingFixed int    `json:"needs_formatting_fixed"`
	T1Promotions         int    `json:"t1_promotions"`
	LastDistillTime      string `json:"last_distill_time"`
}

// DistillState tracks distillation timing across runs.
// Persisted as JSON at {projectRoot}/.ralph/distill-state.json.
type DistillState struct {
	Version              int              `json:"version"`
	MonotonicTaskCounter int              `json:"monotonic_task_counter"`
	LastDistillTask      int              `json:"last_distill_task"`
	Categories           []string         `json:"categories,omitempty"`
	Metrics              *DistillMetrics  `json:"metrics,omitempty"`
}

// LoadDistillState reads DistillState from path.
// Returns default {Version:1} on NotExist (first run).
// Error wrapping: "runner: distill state: load:" prefix.
func LoadDistillState(path string) (*DistillState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &DistillState{Version: 1}, nil
		}
		return nil, fmt.Errorf("runner: distill state: load: %w", err)
	}

	var state DistillState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("runner: distill state: load: %w", err)
	}
	return &state, nil
}

// SaveDistillState writes DistillState as JSON to path.
// Creates parent directory (.ralph/) with os.MkdirAll if not exists.
// Error wrapping: "runner: distill state: save:" prefix.
func SaveDistillState(path string, state *DistillState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("runner: distill state: save: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("runner: distill state: save: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("runner: distill state: save: %w", err)
	}
	return nil
}

// RecoverDistillation checks for interrupted distillation at startup (M7).
// If no intent file exists: returns nil (normal state).
// If intent file with phase="backup": rollback — delete .pending files, delete intent.
// If intent file with phase="write" or "commit": complete pending renames, delete intent.
// Logs warning when recovery occurs. Called BEFORE RecoverDirtyState in Execute().
func RecoverDistillation(projectRoot string) error {
	intent, err := ReadIntentFile(projectRoot)
	if err != nil {
		return fmt.Errorf("runner: distill: recovery: %w", err)
	}
	if intent == nil {
		return nil // no intent file = normal state
	}

	switch intent.Phase {
	case "backup":
		// Rollback: delete any .pending files
		for _, target := range intent.Files {
			_ = os.Remove(target + ".pending") // best-effort cleanup
		}
	case "write", "commit":
		// Complete: rename .pending → target
		if commitErr := CommitPendingFiles(intent.Files); commitErr != nil {
			return fmt.Errorf("runner: distill: recovery: %w", commitErr)
		}
	}

	fmt.Fprintf(os.Stderr, "Recovered from interrupted distillation\n")

	// Clean up: delete intent file + any remaining .pending files
	for _, target := range intent.Files {
		_ = os.Remove(target + ".pending") // best-effort cleanup
	}
	if delErr := DeleteIntentFile(projectRoot); delErr != nil {
		return fmt.Errorf("runner: distill: recovery: %w", delErr)
	}

	return nil
}

