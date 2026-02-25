package config

import "regexp"

// String constants for sprint-tasks.md markers.
// Used by bridge (generation) and runner (scanning) packages.
const (
	TaskOpen       = "- [ ]"
	TaskDone       = "- [x]"
	GateTag        = "[GATE]"
	FeedbackPrefix = "> USER FEEDBACK:"
)

// Compiled regex patterns for sprint-tasks.md line scanning.
// All patterns are compiled at package scope via MustCompile.
// Runner scanner uses: strings.Split(content, "\n") + regex match per line.
var (
	TaskOpenRegex = regexp.MustCompile(`^\s*- \[ \]`)
	TaskDoneRegex = regexp.MustCompile(`^\s*- \[x\]`)
	GateTagRegex  = regexp.MustCompile(`\[GATE\]`)
)
