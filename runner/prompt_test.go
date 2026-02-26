package runner

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

var update = flag.Bool("update", false, "update golden files")

// goldenTest compares got against a golden file, updating it if -update flag is set.
func goldenTest(t *testing.T, name, got string) {
	t.Helper()
	golden := filepath.Join("testdata", name+".golden")
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
		t.Errorf("output mismatch (run with -update to refresh)\ngot:\n%s\nwant:\n%s", got, string(want))
	}
}

func TestPrompt_Execute_WithFindings(t *testing.T) {
	findingsText := "- [HIGH] Missing input validation in handler.go:42\n- [MED] Unused import in utils.go:3"
	data := config.TemplateData{
		HasFindings: true,
	}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__":  config.SprintTasksFormat(),
		"__FINDINGS_CONTENT__": findingsText,
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// Verify all 8 required items from AC #1 (7 sections + format contract injection)
	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// AC #1 required sections
		{"999-rules guardrail section", "999-Rules Guardrails", true},
		{"ATDD instruction", "Acceptance Test-Driven Development", true},
		{"zero-skip policy", "Zero-Skip Policy", true},
		{"red-green cycle", "Red-Green Cycle", true},
		{"self-directing instruction", "sprint-tasks.md", true},
		{"mutation asymmetry", "Mutation Asymmetry", true},
		{"commit on green only", "Commit ONLY when ALL tests pass", true},
		// Format contract injection (AC #1 item 8)
		{"format contract title", "Sprint Tasks Format Specification", true},
		// Findings section present (AC #2)
		{"findings header", "Review Findings", true},
		{"findings instruction", "MUST FIX FIRST", true},
		{"findings content injected", "Missing input validation", true},
		{"findings content severity", "[HIGH]", true},
		// No-findings proceed absent (AC #3)
		{"no proceed instruction", "No pending review findings", false},
		// Placeholders replaced
		{"no format placeholder", "__FORMAT_CONTRACT__", false},
		{"no findings placeholder", "__FINDINGS_CONTENT__", false},
		// 999-rules specific items (FR36)
		{"rule no delete files", "Do NOT delete files outside", true},
		{"rule no destructive commands", "destructive commands", true},
		{"rule no skip tests", "Do NOT skip", true},
		{"rule no disable linters", "Do NOT disable linters", true},
		// ATDD specific (FR37)
		{"ATDD every AC must have test", "Every acceptance criterion", true},
		// Zero-skip specific (FR38)
		{"zero-skip never xfail", "NEVER use xfail", true},
		// Red-green specific
		{"red phase", "failing test", true},
		{"green phase", "minimal code", true},
		// Self-directing specific (FR11)
		{"self-directing scan", "FIRST task marked", true},
		// Mutation asymmetry specific
		{"mutation no modify markers", "MUST NOT modify task status markers", true},
		// Commit rules (FR8)
		{"commit never with failing", "NEVER commit with failing tests", true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	goldenTest(t, "TestPrompt_Execute_WithFindings", got)
}

func TestPrompt_Execute_WithoutFindings(t *testing.T) {
	data := config.TemplateData{
		HasFindings: false,
	}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// All 8 required items still present (AC #1)
		{"999-rules guardrail section", "999-Rules Guardrails", true},
		{"ATDD instruction", "Acceptance Test-Driven Development", true},
		{"zero-skip policy", "Zero-Skip Policy", true},
		{"red-green cycle", "Red-Green Cycle", true},
		{"self-directing instruction", "sprint-tasks.md", true},
		{"mutation asymmetry", "Mutation Asymmetry", true},
		{"commit on green only", "Commit ONLY when ALL tests pass", true},
		{"format contract title", "Sprint Tasks Format Specification", true},
		// No findings section (AC #3)
		{"no findings header", "Review Findings", false},
		{"no findings instruction", "MUST FIX FIRST", false},
		// Proceed instruction present (AC #3)
		{"proceed instruction", "No pending review findings", true},
		// Placeholders replaced / absent
		{"no format placeholder", "__FORMAT_CONTRACT__", false},
		{"no findings placeholder", "__FINDINGS_CONTENT__", false},
		// Gates absent by default
		{"no gates section", "GATES ARE ENABLED", false},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	goldenTest(t, "TestPrompt_Execute_WithoutFindings", got)
}

func TestPrompt_Execute_FormatContract(t *testing.T) {
	data := config.TemplateData{}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	if !strings.Contains(got, "Sprint Tasks Format Specification") {
		t.Errorf("expected prompt to contain format contract title 'Sprint Tasks Format Specification'")
	}

	// Verify format contract markers from config/constants.go are present
	markers := []struct {
		name   string
		marker string
	}{
		{"TaskOpen marker", config.TaskOpen},
		{"TaskDone marker", config.TaskDone},
		{"GateTag marker", config.GateTag},
		{"FeedbackPrefix marker", config.FeedbackPrefix},
		{"source field syntax", "source:"},
	}
	for _, m := range markers {
		t.Run(m.name, func(t *testing.T) {
			if !strings.Contains(got, m.marker) {
				t.Errorf("assembled prompt does not contain format contract marker %q", m.marker)
			}
		})
	}
}

func TestPrompt_Execute_WithGates(t *testing.T) {
	data := config.TemplateData{
		GatesEnabled: true,
	}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		{"gates section present", "GATES ARE ENABLED", true},
		{"gates pause instruction", "pause execution and report status", true},
		{"gates GATE tag reference", "[GATE]", true},
		// Core sections still present with gates
		{"999-rules still present", "999-Rules Guardrails", true},
		{"self-directing still present", "FIRST task marked", true},
		{"proceed section present", "No pending review findings", true},
		// No findings section (HasFindings=false)
		{"no findings header", "Review Findings", false},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}
}

func TestPrompt_Execute_Injection(t *testing.T) {
	// AC #5: user content containing Go template syntax must be preserved literally
	dangerousContent := "This has {{.Dangerous}} template syntax and {{range .Items}}bad{{end}}"
	data := config.TemplateData{
		HasFindings: true,
	}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__":  dangerousContent,
		"__FINDINGS_CONTENT__": "finding with {{.Exploit}} attempt",
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v (template injection should not cause error)", err)
	}

	// Verify dangerous content preserved literally (not executed by template engine)
	if !strings.Contains(got, "{{.Dangerous}}") {
		t.Errorf("expected {{.Dangerous}} to be preserved literally in output")
	}
	if !strings.Contains(got, "{{range .Items}}bad{{end}}") {
		t.Errorf("expected {{range .Items}} to be preserved literally in output")
	}
	if !strings.Contains(got, "{{.Exploit}}") {
		t.Errorf("expected {{.Exploit}} to be preserved literally in output")
	}
}
