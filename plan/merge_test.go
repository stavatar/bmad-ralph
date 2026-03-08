package plan

import (
	"strings"
	"testing"
)

func TestMergeInto_Scenarios(t *testing.T) {
	tests := []struct {
		name         string
		existing     string
		generated    string
		wantContains []string
		wantAbsent   []string
		wantErr      bool
	}{
		{
			name:     "empty existing returns generated",
			existing: "",
			generated: "## Story 1.1\n\n- [ ] Task A\n  source: prd.md#FR1\n",
			wantContains: []string{"## Story 1.1", "Task A"},
		},
		{
			name:     "empty generated returns existing",
			existing: "## Story 1.1\n\n- [x] Done task\n  source: prd.md#FR1\n",
			generated: "",
			wantContains: []string{"## Story 1.1", "- [x] Done task"},
		},
		{
			name: "deduplication skips existing story",
			existing: "## Story 1.1\n\n- [x] Done task\n  source: prd.md#FR1\n",
			generated: "## Story 1.1\n\n- [ ] New version of task\n  source: prd.md#FR1\n",
			wantContains: []string{"## Story 1.1", "- [x] Done task"},
			wantAbsent:   []string{"New version of task"},
		},
		{
			name: "new story appended",
			existing: "## Story 1.1\n\n- [x] Done task\n  source: prd.md#FR1\n",
			generated: "## Story 1.1\n\n- [ ] Dup\n\n## Story 2.1\n\n- [ ] New task\n  source: prd.md#FR2\n",
			wantContains: []string{"## Story 1.1", "- [x] Done task", "## Story 2.1", "New task"},
			wantAbsent:   []string{"Dup"},
		},
		{
			name: "completed tasks preserved",
			existing: "## Story 1.1\n\n- [x] Completed A\n  source: prd.md#FR1\n- [ ] Open B\n  source: prd.md#FR2\n",
			generated: "## Story 2.1\n\n- [ ] New C\n  source: prd.md#FR3\n",
			wantContains: []string{"- [x] Completed A", "- [ ] Open B", "## Story 2.1", "New C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeInto([]byte(tt.existing), []byte(tt.generated))
			if (err != nil) != tt.wantErr {
				t.Fatalf("MergeInto() error = %v, wantErr %v", err, tt.wantErr)
			}
			result := string(got)
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\ngot:\n%s", want, result)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(result, absent) {
					t.Errorf("result should not contain %q\ngot:\n%s", absent, result)
				}
			}
		})
	}
}

func TestMergeInto_StoryCountDedup(t *testing.T) {
	existing := "## Story 1.1\n\n- [x] Done\n"
	generated := "## Story 1.1\n\n- [ ] Dup\n\n## Story 1.1\n\n- [ ] Another dup\n"

	got, err := MergeInto([]byte(existing), []byte(generated))
	if err != nil {
		t.Fatalf("MergeInto() error: %v", err)
	}

	count := strings.Count(string(got), "## Story 1.1")
	if count != 1 {
		t.Errorf("## Story 1.1 appears %d times, want 1", count)
	}
}

func TestMergeInto_OrderPreserved(t *testing.T) {
	existing := "## Story 1.1\n\n- [x] First\n\n## Story 1.2\n\n- [ ] Second\n"
	generated := "## Story 2.1\n\n- [ ] Third\n"

	got, err := MergeInto([]byte(existing), []byte(generated))
	if err != nil {
		t.Fatalf("MergeInto() error: %v", err)
	}

	result := string(got)
	idx11 := strings.Index(result, "## Story 1.1")
	idx12 := strings.Index(result, "## Story 1.2")
	idx21 := strings.Index(result, "## Story 2.1")

	if idx11 >= idx12 {
		t.Errorf("Story 1.1 (pos %d) should come before Story 1.2 (pos %d)", idx11, idx12)
	}
	if idx12 >= idx21 {
		t.Errorf("Story 1.2 (pos %d) should come before Story 2.1 (pos %d)", idx12, idx21)
	}
}
