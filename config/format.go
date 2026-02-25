package config

import _ "embed" // Required for //go:embed directive

//go:embed shared/sprint-tasks-format.md
var sprintTasksFormat string

// SprintTasksFormat returns the embedded sprint-tasks format specification.
// Used by bridge (prompt generation) and runner (parsing reference).
func SprintTasksFormat() string {
	return sprintTasksFormat
}
