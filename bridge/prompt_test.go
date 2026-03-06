package bridge

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	// Self-reexec dispatch: exit 0 with empty stdout for parse result error testing
	if os.Getenv("BRIDGE_TEST_EMPTY_OUTPUT") == "1" {
		os.Exit(0)
	}
	if testutil.RunMockClaude() {
		return
	}
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

func TestBridgePrompt_NonEmpty(t *testing.T) {
	prompt := BridgePrompt()
	if prompt == "" {
		t.Error("BridgePrompt() returned empty string")
	}
}

func TestBridgePrompt_Creation(t *testing.T) {
	replacements := map[string]string{
		"__STORY_CONTENT__":   "## Story: User Authentication\n\n### AC-1\nUser can log in with email and password.",
		"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
		"__EXISTING_TASKS__":  "should not appear in creation mode",
	}

	data := config.TemplateData{
		HasExistingTasks: false,
	}

	got, err := config.AssemblePrompt(BridgePrompt(), data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// Verify key content via strings.Contains
	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		{"format contract title", "Sprint Tasks Format Specification", true},
		{"story content injected", "User Authentication", true},
		{"story AC detail", "email and password", true},
		{"conversion instructions", "Group related ACs into tasks", true},
		{"AC classification", "AC Classification", true},
		{"granularity rule", "Task Granularity Rule", true},
		{"unit of work", "unit of work", true},
		{"testing within tasks", "Testing Within Tasks", true},
		{"negative examples DO NOT", "DO NOT", true},
		{"task syntax requirement", "- [ ]", true},
		{"source traceability", "source:", true},
		{"gate marking", "[GATE]", true},
		{"service tasks SETUP", "[SETUP]", true},
		{"service tasks E2E", "[E2E]", true},
		{"output instructions", "sprint-tasks.md", true},
		// Enriched content assertions (Story 2.4)
		{"gate first-of-epic rule", "first task of each epic", true},
		{"gate milestone rule", "user-visible milestone", true},
		{"source multi-AC format", "#AC-1,AC-2,AC-3", true},
		{"source identifier SETUP", "#SETUP", true},
		{"source identifier E2E", "#E2E", true},
		{"negative source example", "Task source: file.md#AC-1", true},
		{"no merge content", "Merge Mode", false},
		{"no existing tasks placeholder", "__EXISTING_TASKS__", false},
		{"no story placeholder", "__STORY_CONTENT__", false},
		{"no format placeholder", "__FORMAT_CONTRACT__", false},
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

	goldenTest(t, "TestBridgePrompt_Creation.golden", got)
}

func TestBridgePrompt_Merge(t *testing.T) {
	existingTasks := "- [x] existing task\n  source: stories/test.md#AC-1"
	replacements := map[string]string{
		"__STORY_CONTENT__":   "## Story: Add Payment\n\n### AC-1\nUser can pay with credit card.",
		"__FORMAT_CONTRACT__": config.SprintTasksFormat(),
		"__EXISTING_TASKS__":  existingTasks,
	}

	data := config.TemplateData{
		HasExistingTasks: true,
	}

	got, err := config.AssemblePrompt(BridgePrompt(), data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// Symmetric with TestBridgePrompt_Creation: verify all content sections
	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// Creation-mode content must still be present
		{"format contract title", "Sprint Tasks Format Specification", true},
		{"story content injected", "Add Payment", true},
		{"conversion instructions", "Group related ACs into tasks", true},
		{"AC classification", "AC Classification", true},
		{"granularity rule", "Task Granularity Rule", true},
		{"unit of work", "unit of work", true},
		{"testing within tasks", "Testing Within Tasks", true},
		{"negative examples", "DO NOT", true},
		{"task syntax requirement", "- [ ]", true},
		{"source traceability", "source:", true},
		{"gate marking", "[GATE]", true},
		{"service tasks SETUP", "[SETUP]", true},
		{"service tasks E2E", "[E2E]", true},
		{"output instructions", "sprint-tasks.md", true},
		// Enriched content assertions (Story 2.4)
		{"gate first-of-epic rule", "first task of each epic", true},
		{"gate milestone rule", "user-visible milestone", true},
		{"source multi-AC format", "#AC-1,AC-2,AC-3", true},
		{"source identifier SETUP", "#SETUP", true},
		{"source identifier E2E", "#E2E", true},
		{"negative source example", "Task source: file.md#AC-1", true},
		// Merge-specific content
		{"merge mode header", "Merge Mode", true},
		{"existing tasks injected", "existing task", true},
		{"existing source field", "stories/test.md#AC-1", true},
		{"preserve status instruction", "Preserve", true},
		// AC #2 mandated merge instructions (Story 2.6)
		{"merge MUST NOT change x status", "MUST NOT change [x] status", true},
		{"merge PRESERVE original task order", "PRESERVE original task order", true},
		{"merge Modified tasks instruction", "Modified tasks", true},
		// Placeholders must be replaced
		{"no existing tasks placeholder", "__EXISTING_TASKS__", false},
		{"no story placeholder", "__STORY_CONTENT__", false},
		{"no format placeholder", "__FORMAT_CONTRACT__", false},
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

	goldenTest(t, "TestBridgePrompt_Merge.golden", got)
}

func TestBridgePrompt_ContainsFormatContract(t *testing.T) {
	// Structural Rule #8: cross-package verification of format contract markers
	formatContent := config.SprintTasksFormat()

	replacements := map[string]string{
		"__STORY_CONTENT__":   "test story",
		"__FORMAT_CONTRACT__": formatContent,
		"__EXISTING_TASKS__":  "",
	}

	got, err := config.AssemblePrompt(BridgePrompt(), config.TemplateData{}, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	markers := []struct {
		name   string
		marker string
	}{
		{"TaskOpen marker", config.TaskOpen},
		{"TaskDone marker", config.TaskDone},
		{"GateTag marker", config.GateTag},
		{"FeedbackPrefix marker", config.FeedbackPrefix},
		{"source field syntax", "source:"},
		{"SETUP service prefix", "[SETUP]"},
		{"E2E service prefix", "[E2E]"},
	}
	for _, m := range markers {
		t.Run(m.name, func(t *testing.T) {
			if !strings.Contains(got, m.marker) {
				t.Errorf("assembled prompt does not contain format contract marker %q", m.marker)
			}
		})
	}
}
