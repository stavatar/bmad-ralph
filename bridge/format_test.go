package bridge

import (
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

func TestSprintTasksFormat_BridgeConsumer_NonEmpty(t *testing.T) {
	content := config.SprintTasksFormat()
	if content == "" {
		t.Error("config.SprintTasksFormat() returned empty string from bridge package")
	}
}

func TestSprintTasksFormat_BridgeConsumer_ContainsMarkers(t *testing.T) {
	content := config.SprintTasksFormat()
	tests := []struct {
		name   string
		marker string
	}{
		{"TaskOpen marker", config.TaskOpen},
		{"TaskDone marker", config.TaskDone},
		{"GateTag marker", config.GateTag},
		{"FeedbackPrefix marker", config.FeedbackPrefix},
		{"source field syntax", "source:"},
		{"SETUP service prefix", "[SETUP]"},
		{"VERIFY service prefix", "[VERIFY]"},
		{"E2E service prefix", "[E2E]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(content, tt.marker) {
				t.Errorf("config.SprintTasksFormat() does not contain %q", tt.marker)
			}
		})
	}
}

func TestSprintTasksFormat_BridgeConsumer_RegexAccessible(t *testing.T) {
	// Verify SourceFieldRegex is exported and usable from bridge package.
	// Cross-package export check for Structural Rule #8 (shared contract).
	input := "  source: stories/auth.md#AC-3"
	if !config.SourceFieldRegex.MatchString(input) {
		t.Errorf("config.SourceFieldRegex.MatchString(%q) = false, want true", input)
	}
}
