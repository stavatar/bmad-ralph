package config

import (
	"strings"
	"testing"
)

func TestSprintTasksFormat_NonEmpty(t *testing.T) {
	content := SprintTasksFormat()
	if content == "" {
		t.Error("SprintTasksFormat() returned empty string")
	}
}

func TestSprintTasksFormat_ContainsMarkers(t *testing.T) {
	content := SprintTasksFormat()
	tests := []struct {
		name   string
		marker string
	}{
		{"TaskOpen marker", TaskOpen},
		{"TaskDone marker", TaskDone},
		{"GateTag marker", GateTag},
		{"FeedbackPrefix marker", FeedbackPrefix},
		{"source field syntax", "source:"},
		{"SETUP service prefix", "[SETUP]"},
		{"E2E service prefix", "[E2E]"},
		{"source field regex pattern", `^\s+source:\s+\S+#\S+`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(content, tt.marker) {
				t.Errorf("SprintTasksFormat() does not contain %q", tt.marker)
			}
		})
	}
}
