package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

func TestScanTasks_OpenTasks(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
		wantLines []int // expected 1-based line numbers
	}{
		{
			"single open task",
			"- [ ] Task 1",
			1,
			[]int{1},
		},
		{
			"multiple open tasks",
			"- [ ] Task 1\n- [ ] Task 2\n- [ ] Task 3",
			3,
			[]int{1, 2, 3},
		},
		{
			"indented subtasks match as open",
			"- [ ] Parent task\n  - [ ] Subtask 1\n    - [ ] Subtask 2",
			3,
			[]int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScanTasks(tt.content)
			if err != nil {
				t.Fatalf("ScanTasks() error = %v", err)
			}
			if got := len(result.OpenTasks); got != tt.wantCount {
				t.Errorf("OpenTasks count = %d, want %d", got, tt.wantCount)
			}
			if result.DoneTasks != nil {
				t.Errorf("DoneTasks = %v, want nil (open-only input)", result.DoneTasks)
			}
			for i, wantLine := range tt.wantLines {
				if i >= len(result.OpenTasks) {
					break
				}
				if result.OpenTasks[i].LineNum != wantLine {
					t.Errorf("OpenTasks[%d].LineNum = %d, want %d", i, result.OpenTasks[i].LineNum, wantLine)
				}
			}
		})
	}
}

func TestScanTasks_DoneTasks(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantDone    int
		wantOpen    int
		wantDoneNil bool
		wantOpenNil bool
	}{
		{
			"single done task",
			"- [x] Completed",
			1, 0, false, true,
		},
		{
			"multiple done tasks",
			"- [x] Done 1\n- [x] Done 2\n- [x] Done 3",
			3, 0, false, true,
		},
		{
			"mixed open and done",
			"- [ ] Open 1\n- [x] Done 1\n- [ ] Open 2\n- [x] Done 2",
			2, 2, false, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScanTasks(tt.content)
			if err != nil {
				t.Fatalf("ScanTasks() error = %v", err)
			}
			if got := len(result.DoneTasks); got != tt.wantDone {
				t.Errorf("DoneTasks count = %d, want %d", got, tt.wantDone)
			}
			if got := len(result.OpenTasks); got != tt.wantOpen {
				t.Errorf("OpenTasks count = %d, want %d", got, tt.wantOpen)
			}
			if tt.wantOpenNil && result.OpenTasks != nil {
				t.Errorf("OpenTasks = %v, want nil", result.OpenTasks)
			}
			if tt.wantDoneNil && result.DoneTasks != nil {
				t.Errorf("DoneTasks = %v, want nil", result.DoneTasks)
			}
		})
	}
}

func TestScanTasks_GateDetection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		checkIdx int // which task to check
		inOpen   bool
		wantGate bool
	}{
		{
			"open task with GATE",
			"- [ ] Task with [GATE] marker",
			0, true, true,
		},
		{
			"done task with GATE",
			"- [x] Completed [GATE] task",
			0, false, true,
		},
		{
			"task without GATE",
			"- [ ] Normal task",
			0, true, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScanTasks(tt.content)
			if err != nil {
				t.Fatalf("ScanTasks() error = %v", err)
			}

			var entry TaskEntry
			if tt.inOpen {
				if tt.checkIdx >= len(result.OpenTasks) {
					t.Fatalf("OpenTasks has %d entries, want index %d", len(result.OpenTasks), tt.checkIdx)
				}
				entry = result.OpenTasks[tt.checkIdx]
			} else {
				if tt.checkIdx >= len(result.DoneTasks) {
					t.Fatalf("DoneTasks has %d entries, want index %d", len(result.DoneTasks), tt.checkIdx)
				}
				entry = result.DoneTasks[tt.checkIdx]
			}

			if entry.HasGate != tt.wantGate {
				t.Errorf("HasGate = %v, want %v", entry.HasGate, tt.wantGate)
			}
		})
	}

	// Verify the "mixed gates" case has correct gate flags for all entries
	t.Run("mixed gates all entries verified", func(t *testing.T) {
		content := "- [ ] Normal\n- [ ] With [GATE]\n- [x] Done [GATE]\n- [x] Done normal"
		result, err := ScanTasks(content)
		if err != nil {
			t.Fatalf("ScanTasks() error = %v", err)
		}

		if len(result.OpenTasks) != 2 {
			t.Fatalf("OpenTasks count = %d, want 2", len(result.OpenTasks))
		}
		if result.OpenTasks[0].HasGate {
			t.Errorf("OpenTasks[0].HasGate = true, want false (Normal)")
		}
		if !result.OpenTasks[1].HasGate {
			t.Errorf("OpenTasks[1].HasGate = false, want true (With [GATE])")
		}

		if len(result.DoneTasks) != 2 {
			t.Fatalf("DoneTasks count = %d, want 2", len(result.DoneTasks))
		}
		if !result.DoneTasks[0].HasGate {
			t.Errorf("DoneTasks[0].HasGate = false, want true (Done [GATE])")
		}
		if result.DoneTasks[1].HasGate {
			t.Errorf("DoneTasks[1].HasGate = true, want false (Done normal)")
		}
	})
}

func TestScanTasks_NoTasks(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty string", ""},
		{"text with no markers", "# Sprint Tasks\n\nSome text without checkboxes\n"},
		{"blank lines only", "\n\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScanTasks(tt.content)
			if err == nil {
				t.Fatal("ScanTasks() expected error, got nil")
			}
			if !errors.Is(err, config.ErrNoTasks) {
				t.Errorf("errors.Is(err, ErrNoTasks) = false, want true; err = %v", err)
			}
			if !strings.Contains(err.Error(), "no tasks found") {
				t.Errorf("error message: want 'no tasks found', got %q", err.Error())
			}
			if !strings.Contains(err.Error(), "runner:") {
				t.Errorf("error prefix: want 'runner:', got %q", err.Error())
			}
			if result.HasAnyTasks() {
				t.Errorf("HasAnyTasks() = true, want false")
			}
		})
	}
}

func TestScanTasks_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantOpen int
		wantDone int
		wantText string // expected Text of first matched open task (if wantOpen > 0)
	}{
		{
			"malformed marker missing space",
			"- [] Not a task\n- [ ] Real task",
			1, 0,
			"- [ ] Real task",
		},
		{
			"uppercase X not matched",
			"- [X] Not matched\n- [x] Matched",
			0, 1,
			"",
		},
		{
			"marker not at line start embedded in text",
			"Some text - [ ] not a task\n- [ ] Real task",
			1, 0,
			"- [ ] Real task",
		},
		{
			"only completed tasks no error",
			"- [x] Done 1\n- [x] Done 2",
			0, 2,
			"",
		},
		{
			"trailing newline no phantom entry",
			"- [ ] Task 1\n",
			1, 0,
			"- [ ] Task 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScanTasks(tt.content)
			if err != nil {
				t.Fatalf("ScanTasks() unexpected error = %v", err)
			}
			if got := len(result.OpenTasks); got != tt.wantOpen {
				t.Errorf("OpenTasks count = %d, want %d", got, tt.wantOpen)
			}
			if got := len(result.DoneTasks); got != tt.wantDone {
				t.Errorf("DoneTasks count = %d, want %d", got, tt.wantDone)
			}
			if tt.wantText != "" && len(result.OpenTasks) > 0 {
				if result.OpenTasks[0].Text != tt.wantText {
					t.Errorf("OpenTasks[0].Text = %q, want %q", result.OpenTasks[0].Text, tt.wantText)
				}
			}
		})
	}
}

func TestScanResult_HasOpenTasks(t *testing.T) {
	tests := []struct {
		name string
		r    ScanResult
		want bool
	}{
		{"with open tasks", ScanResult{OpenTasks: []TaskEntry{{LineNum: 1}}}, true},
		{"no open tasks", ScanResult{}, false},
		{"only done tasks", ScanResult{DoneTasks: []TaskEntry{{LineNum: 1}}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasOpenTasks(); got != tt.want {
				t.Errorf("HasOpenTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanResult_HasDoneTasks(t *testing.T) {
	tests := []struct {
		name string
		r    ScanResult
		want bool
	}{
		{"with done tasks", ScanResult{DoneTasks: []TaskEntry{{LineNum: 1}}}, true},
		{"no done tasks", ScanResult{}, false},
		{"only open tasks", ScanResult{OpenTasks: []TaskEntry{{LineNum: 1}}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasDoneTasks(); got != tt.want {
				t.Errorf("HasDoneTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanResult_HasAnyTasks(t *testing.T) {
	tests := []struct {
		name string
		r    ScanResult
		want bool
	}{
		{"open only", ScanResult{OpenTasks: []TaskEntry{{LineNum: 1}}}, true},
		{"done only", ScanResult{DoneTasks: []TaskEntry{{LineNum: 1}}}, true},
		{"both open and done", ScanResult{
			OpenTasks: []TaskEntry{{LineNum: 1}},
			DoneTasks: []TaskEntry{{LineNum: 2}},
		}, true},
		{"neither", ScanResult{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasAnyTasks(); got != tt.want {
				t.Errorf("HasAnyTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanTasks_LineNumbers(t *testing.T) {
	content := "# Header\n\n- [ ] First task\n- [x] Done task\n\n- [ ] Third task"
	result, err := ScanTasks(content)
	if err != nil {
		t.Fatalf("ScanTasks() error = %v", err)
	}

	// "# Header" = line 1, "" = line 2, "- [ ] First task" = line 3
	// "- [x] Done task" = line 4, "" = line 5, "- [ ] Third task" = line 6
	if len(result.OpenTasks) != 2 {
		t.Fatalf("OpenTasks count = %d, want 2", len(result.OpenTasks))
	}
	if result.OpenTasks[0].LineNum != 3 {
		t.Errorf("OpenTasks[0].LineNum = %d, want 3", result.OpenTasks[0].LineNum)
	}
	if result.OpenTasks[1].LineNum != 6 {
		t.Errorf("OpenTasks[1].LineNum = %d, want 6", result.OpenTasks[1].LineNum)
	}

	if len(result.DoneTasks) != 1 {
		t.Fatalf("DoneTasks count = %d, want 1", len(result.DoneTasks))
	}
	if result.DoneTasks[0].LineNum != 4 {
		t.Errorf("DoneTasks[0].LineNum = %d, want 4", result.DoneTasks[0].LineNum)
	}

	// Verify text is stored as the full line
	if result.OpenTasks[0].Text != "- [ ] First task" {
		t.Errorf("OpenTasks[0].Text = %q, want %q", result.OpenTasks[0].Text, "- [ ] First task")
	}
}
