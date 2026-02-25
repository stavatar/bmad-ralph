package config

import "testing"

func TestTaskOpenRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"standard task", "- [ ] Implement feature", true},
		{"space-indented subtask", "  - [ ] Subtask indented", true},
		{"tab-indented subtask", "\t- [ ] Tab indented", true},
		{"deep indentation", "    - [ ] Deep subtask", true},
		{"marker only no trailing", "- [ ]", true},
		{"done task not open", "- [x] Done task", false},
		{"empty line", "", false},
		{"marker not at line start", "Some text - [ ] embedded", false},
		{"malformed missing space", "- [] Missing space", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskOpenRegex.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("TaskOpenRegex.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTaskDoneRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"standard done", "- [x] Completed task", true},
		{"space-indented done", "  - [x] Indented done", true},
		{"tab-indented done", "\t- [x] Tab indented", true},
		{"deep indentation done", "    - [x] Deep subtask", true},
		{"marker only no trailing", "- [x]", true},
		{"done with gate tag", "- [x] Task with [GATE] tag", true},
		{"open task not done", "- [ ] Open task", false},
		{"empty line", "", false},
		{"uppercase X", "- [X] Uppercase X", false},
		{"marker not at line start", "Some text - [x] embedded", false},
		{"malformed extra space", "- [x ] Extra space", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskDoneRegex.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("TaskDoneRegex.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGateTagRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"gate at end", "- [ ] Setup environment [GATE]", true},
		{"gate at start", "[GATE] First line", true},
		{"gate in middle of done task", "- [x] Done [GATE] tagged", true},
		{"no gate tag", "- [ ] Normal task", false},
		{"empty line", "", false},
		{"lowercase gate", "[gate] lowercase", false},
		{"missing brackets", "GATE without brackets", false},
		{"missing closing bracket", "[GATE", false},
		{"missing opening bracket", "GATE]", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GateTagRegex.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("GateTagRegex.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTaskOpen_Value(t *testing.T) {
	if TaskOpen != "- [ ]" {
		t.Errorf("TaskOpen = %q, want %q", TaskOpen, "- [ ]")
	}
}

func TestTaskDone_Value(t *testing.T) {
	if TaskDone != "- [x]" {
		t.Errorf("TaskDone = %q, want %q", TaskDone, "- [x]")
	}
}

func TestGateTag_Value(t *testing.T) {
	if GateTag != "[GATE]" {
		t.Errorf("GateTag = %q, want %q", GateTag, "[GATE]")
	}
}

func TestFeedbackPrefix_Value(t *testing.T) {
	if FeedbackPrefix != "> USER FEEDBACK:" {
		t.Errorf("FeedbackPrefix = %q, want %q", FeedbackPrefix, "> USER FEEDBACK:")
	}
}

func TestSourceFieldRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"two-space indent", "  source: stories/auth.md#AC-3", true},
		{"four-space indent", "    source: stories/api.md#AC-1", true},
		{"tab indent", "\tsource: story.md#SETUP", true},
		{"no indent", "source: stories/auth.md#AC-3", false},
		{"missing hash separator", "  source: stories/auth.md", false},
		{"empty identifier after hash", "  source: stories/auth.md#", false},
		{"empty line", "", false},
		{"no source keyword", "  no source here", false},
		{"no space after colon", "  source:stories/auth.md#AC-3", false},
		{"capital S", "  Source: stories/auth.md#AC-3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SourceFieldRegex.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("SourceFieldRegex.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
