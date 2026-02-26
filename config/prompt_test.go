package config

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// goldenTest compares got against a golden file, updating it if -update flag is set.
func goldenTest(t *testing.T, goldenFile, got string) {
	t.Helper()
	golden := filepath.Join("testdata", goldenFile)
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	if got != string(want) {
		t.Errorf("output mismatch:\ngot:\n%s\nwant:\n%s", got, string(want))
	}
}

func TestAssemblePrompt_Simple(t *testing.T) {
	tmpl := `Serena is {{if .SerenaEnabled}}enabled{{else}}disabled{{end}}.`
	got, err := AssemblePrompt(tmpl, TemplateData{SerenaEnabled: true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	goldenTest(t, "TestAssemblePrompt_Simple.golden", got)
}

func TestAssemblePrompt_Conditional(t *testing.T) {
	tmpl := `Base prompt.
{{if .SerenaEnabled}}Use Serena MCP tools.{{end}}
{{if .GatesEnabled}}Gates are active.{{end}}
Done.`
	got, err := AssemblePrompt(tmpl, TemplateData{SerenaEnabled: true, GatesEnabled: false}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	goldenTest(t, "TestAssemblePrompt_Conditional.golden", got)
}

func TestAssemblePrompt_Injection(t *testing.T) {
	tmpl := `Prompt: __TASK_CONTENT__`
	malicious := `{{.SerenaEnabled}} and {{if true}}injected{{end}}`
	replacements := map[string]string{
		"__TASK_CONTENT__": malicious,
	}
	got, err := AssemblePrompt(tmpl, TemplateData{}, replacements)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Stage 2 content must remain literal — no template processing
	if !strings.Contains(got, "{{.SerenaEnabled}}") {
		t.Error("template syntax in user content was processed instead of remaining literal")
	}
	goldenTest(t, "TestAssemblePrompt_Injection.golden", got)
}

func TestAssemblePrompt_AllFields(t *testing.T) {
	tmpl := `{{if .SerenaEnabled}}[SERENA] {{end}}{{if .GatesEnabled}}[GATES] {{end -}}
{{- if .HasExistingTasks}}[MERGE] {{end}}Task: __TASK__
Learnings: __LEARNINGS__
Claude: __CLAUDE_MD__
Findings: __FINDINGS__
Story: __STORY__
Existing: __EXISTING__`
	data := TemplateData{
		SerenaEnabled:        true,
		GatesEnabled:         true,
		HasExistingTasks:     true,
		TaskContent:          "task data",
		LearningsContent:     "learnings data",
		ClaudeMdContent:      "claude md data",
		FindingsContent:      "findings data",
		StoryContent:         "story data",
		ExistingTasksContent: "existing tasks data",
	}
	replacements := map[string]string{
		"__TASK__":      "implement feature X",
		"__LEARNINGS__": "avoid pattern Y",
		"__CLAUDE_MD__": "project rules here",
		"__FINDINGS__":  "no issues found",
		"__STORY__":     "story content here",
		"__EXISTING__":  "existing tasks here",
	}
	got, err := AssemblePrompt(tmpl, data, replacements)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	goldenTest(t, "TestAssemblePrompt_AllFields.golden", got)
}

func TestAssemblePrompt_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		tmpl         string
		data         TemplateData
		replacements map[string]string
		want         string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "empty replacements map",
			tmpl:         `Hello {{if .SerenaEnabled}}Serena{{else}}World{{end}}.`,
			data:         TemplateData{SerenaEnabled: false},
			replacements: map[string]string{},
			want:         "Hello World.",
		},
		{
			name:         "nil replacements map",
			tmpl:         `Hello {{if .GatesEnabled}}Gates{{else}}World{{end}}.`,
			data:         TemplateData{GatesEnabled: false},
			replacements: nil,
			want:         "Hello World.",
		},
		{
			name:         "zero value all empty",
			tmpl:         "",
			data:         TemplateData{},
			replacements: nil,
			want:         "",
		},
		{
			name:    "invalid template syntax",
			tmpl:    `{{if .SerenaEnabled}`,
			data:    TemplateData{},
			wantErr: true,
			errContains: "config: assemble prompt: parse:",
		},
		{
			name:        "execute error bad function call",
			tmpl:        `{{call .SerenaEnabled}}`,
			data:        TemplateData{},
			wantErr:     true,
			errContains: "config: assemble prompt: execute:",
		},
		{
			name:        "execute error unknown struct field",
			tmpl:        `{{.NonexistentField}}`,
			data:        TemplateData{},
			wantErr:     true,
			errContains: "config: assemble prompt: execute:",
		},
		{
			name: "deterministic replacement order",
			tmpl: `__B____A__`,
			data: TemplateData{},
			replacements: map[string]string{
				"__A__": "1",
				"__B__": "2",
			},
			// Sorted order: __A__ first, then __B__
			// "__B____A__" → replace __A__ first → "__B__1" → replace __B__ → "21"
			want: "21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssemblePrompt(tt.tmpl, tt.data, tt.replacements)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
