package runner

import (
	"fmt"
	"strings"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// TaskEntry represents a single task line from sprint-tasks.md.
type TaskEntry struct {
	LineNum int
	Text    string
	HasGate bool
}

// ScanResult holds the parsed task state from sprint-tasks.md.
type ScanResult struct {
	OpenTasks []TaskEntry
	DoneTasks []TaskEntry
}

// HasOpenTasks reports whether any open tasks were found.
func (r ScanResult) HasOpenTasks() bool {
	return len(r.OpenTasks) > 0
}

// HasDoneTasks reports whether any completed tasks were found.
func (r ScanResult) HasDoneTasks() bool {
	return len(r.DoneTasks) > 0
}

// HasAnyTasks reports whether any tasks (open or done) were found.
func (r ScanResult) HasAnyTasks() bool {
	return r.HasOpenTasks() || r.HasDoneTasks()
}

// ScanTasks parses sprint-tasks.md content and returns structured task state.
// It uses config.TaskOpenRegex, config.TaskDoneRegex, and config.GateTagRegex
// for matching — no hardcoded marker strings.
func ScanTasks(content string) (ScanResult, error) {
	var result ScanResult

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNum := i + 1 // 1-based line numbers

		if config.TaskOpenRegex.MatchString(line) {
			entry := TaskEntry{
				LineNum: lineNum,
				Text:    line,
				HasGate: config.GateTagRegex.MatchString(line),
			}
			result.OpenTasks = append(result.OpenTasks, entry)
		} else if config.TaskDoneRegex.MatchString(line) {
			entry := TaskEntry{
				LineNum: lineNum,
				Text:    line,
				HasGate: config.GateTagRegex.MatchString(line),
			}
			result.DoneTasks = append(result.DoneTasks, entry)
		}
	}

	if !result.HasAnyTasks() {
		return ScanResult{}, fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)
	}

	return result, nil
}
