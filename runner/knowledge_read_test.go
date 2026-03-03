package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Task 9.1: ValidateLearnings valid entries ---

func TestValidateLearnings_ValidEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create files that the citations reference
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	content := "## testing: assertion-quality [review, runner/runner.go:42]\nAlways verify message content\n\n## errors: wrapping [review, runner/runner.go:10]\nUse fmt.Errorf with %%w"

	valid, stale := ValidateLearnings(tmpDir, content)

	if !strings.Contains(valid, "assertion-quality") {
		t.Errorf("valid should contain 'assertion-quality', got %q", valid)
	}
	if !strings.Contains(valid, "wrapping") {
		t.Errorf("valid should contain 'wrapping', got %q", valid)
	}
	if stale != "" {
		t.Errorf("stale should be empty, got %q", stale)
	}
}

// --- Task 9.2: ValidateLearnings stale entries ---

func TestValidateLearnings_StaleEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create only one of the two referenced files
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	content := "## testing: good-entry [review, runner/runner.go:42]\nValid entry\n\n## testing: stale-entry [review, src/old_module.go:10]\nStale entry"

	valid, stale := ValidateLearnings(tmpDir, content)

	if !strings.Contains(valid, "good-entry") {
		t.Errorf("valid should contain 'good-entry', got %q", valid)
	}
	if strings.Contains(valid, "stale-entry") {
		t.Errorf("valid should NOT contain 'stale-entry', got %q", valid)
	}
	if !strings.Contains(stale, "stale-entry") {
		t.Errorf("stale should contain 'stale-entry', got %q", stale)
	}
	if strings.Contains(stale, "good-entry") {
		t.Errorf("stale should NOT contain 'good-entry', got %q", stale)
	}
}

// --- Task 9.3: ValidateLearnings reverse order ---

func TestValidateLearnings_ReverseOrder(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create referenced files
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Entry A (oldest) then Entry B (newest) in file order
	content := "## testing: entry-A [review, runner/runner.go:10]\nOldest entry\n\n## testing: entry-B [review, runner/runner.go:20]\nNewest entry"

	valid, _ := ValidateLearnings(tmpDir, content)

	// After reverse: B should come before A (recency-first)
	idxB := strings.Index(valid, "entry-B")
	idxA := strings.Index(valid, "entry-A")
	if idxB < 0 || idxA < 0 {
		t.Fatalf("both entries should be in valid output, got %q", valid)
	}
	if idxB >= idxA {
		t.Errorf("entry-B (newest) should come BEFORE entry-A (oldest) in recency-first order, B at %d, A at %d", idxB, idxA)
	}
}

// --- Task 9.4: ValidateLearnings empty content ---

func TestValidateLearnings_EmptyContent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	cases := []struct {
		name    string
		content string
	}{
		{"empty string", ""},
		{"whitespace only", "   \n  \n  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			valid, stale := ValidateLearnings(tmpDir, tc.content)
			if valid != "" {
				t.Errorf("valid should be empty, got %q", valid)
			}
			if stale != "" {
				t.Errorf("stale should be empty, got %q", stale)
			}
		})
	}
}

// --- Task 9.5: buildKnowledgeReplacements all files ---

func TestBuildKnowledgeReplacements_AllFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with valid citation
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir runner: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write runner.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"),
		[]byte("## testing: patterns [review, runner/runner.go:1]\nImportant pattern"), 0644); err != nil {
		t.Fatalf("write LEARNINGS.md: %v", err)
	}

	// Create ralph rules
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("mkdir rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-critical.md"), []byte("Critical rule content"), 0644); err != nil {
		t.Fatalf("write ralph-critical.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-testing.md"), []byte("Testing rule content"), 0644); err != nil {
		t.Fatalf("write ralph-testing.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-misc.md"), []byte("Misc rule content"), 0644); err != nil {
		t.Fatalf("write ralph-misc.md: %v", err)
	}

	replacements, appendSys, err := buildKnowledgeReplacements(tmpDir)
	if err != nil {
		t.Fatalf("buildKnowledgeReplacements error: %v", err)
	}

	// LEARNINGS.md content should be present
	learnings := replacements["__LEARNINGS_CONTENT__"]
	if !strings.Contains(learnings, "Important pattern") {
		t.Errorf("__LEARNINGS_CONTENT__ should contain 'Important pattern', got %q", learnings)
	}

	// Channel 1: ralph-critical.md via append-system-prompt
	if appendSys == nil {
		t.Fatal("appendSysPrompt should not be nil when ralph-critical.md exists")
	}
	if !strings.Contains(*appendSys, "Critical rule content") {
		t.Errorf("appendSysPrompt should contain 'Critical rule content', got %q", *appendSys)
	}

	// Channel 2: remaining ralph-*.md
	knowledge := replacements["__RALPH_KNOWLEDGE__"]
	if !strings.Contains(knowledge, "Testing rule content") {
		t.Errorf("__RALPH_KNOWLEDGE__ should contain 'Testing rule content', got %q", knowledge)
	}
	if !strings.Contains(knowledge, "Misc rule content") {
		t.Errorf("__RALPH_KNOWLEDGE__ should contain 'Misc rule content', got %q", knowledge)
	}
	// Channel 2 should NOT contain critical (it goes to Channel 1)
	if strings.Contains(knowledge, "Critical rule content") {
		t.Errorf("__RALPH_KNOWLEDGE__ should NOT contain 'Critical rule content'")
	}
}

// --- Task 9.6: buildKnowledgeReplacements missing files ---

func TestBuildKnowledgeReplacements_MissingFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	replacements, appendSys, err := buildKnowledgeReplacements(tmpDir)
	if err != nil {
		t.Fatalf("buildKnowledgeReplacements error: %v", err)
	}

	if replacements["__LEARNINGS_CONTENT__"] != "" {
		t.Errorf("__LEARNINGS_CONTENT__ should be empty, got %q", replacements["__LEARNINGS_CONTENT__"])
	}
	if replacements["__RALPH_KNOWLEDGE__"] != "" {
		t.Errorf("__RALPH_KNOWLEDGE__ should be empty, got %q", replacements["__RALPH_KNOWLEDGE__"])
	}
	if appendSys != nil {
		t.Errorf("appendSysPrompt should be nil when no ralph-critical.md, got %q", *appendSys)
	}
}

// --- Task 9.7: buildKnowledgeReplacements critical channel ---

func TestBuildKnowledgeReplacements_CriticalChannel(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-critical.md"), []byte("High-frequency rules here"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, appendSys, err := buildKnowledgeReplacements(tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if appendSys == nil {
		t.Fatal("appendSysPrompt should not be nil")
	}
	if *appendSys != "High-frequency rules here" {
		t.Errorf("appendSysPrompt = %q, want %q", *appendSys, "High-frequency rules here")
	}
}

// --- Task 9.8: buildKnowledgeReplacements excludes index ---

func TestBuildKnowledgeReplacements_ExcludesIndex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-index.md"), []byte("Index should be excluded"), 0644); err != nil {
		t.Fatalf("write ralph-index.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-testing.md"), []byte("Testing content"), 0644); err != nil {
		t.Fatalf("write ralph-testing.md: %v", err)
	}

	replacements, appendSys, err := buildKnowledgeReplacements(tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	knowledge := replacements["__RALPH_KNOWLEDGE__"]
	if strings.Contains(knowledge, "Index should be excluded") {
		t.Errorf("__RALPH_KNOWLEDGE__ should NOT contain ralph-index.md content")
	}
	if !strings.Contains(knowledge, "Testing content") {
		t.Errorf("__RALPH_KNOWLEDGE__ should contain 'Testing content', got %q", knowledge)
	}
	if appendSys != nil {
		t.Errorf("appendSysPrompt should be nil (no ralph-critical.md)")
	}
}

// --- Task 9.17: Budget warning over budget ---

func TestBudgetWarning_OverBudget(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a LEARNINGS.md that exceeds budget
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	budgetStatus, err := BudgetCheck(t.Context(), filepath.Join(tmpDir, "LEARNINGS.md"), 200)
	if err != nil {
		t.Fatalf("BudgetCheck error: %v", err)
	}
	if !budgetStatus.OverBudget {
		t.Errorf("OverBudget should be true when lines (%d) >= limit (200)", budgetStatus.Lines)
	}
	if budgetStatus.Lines < 200 {
		t.Errorf("Lines should be >= 200, got %d", budgetStatus.Lines)
	}
}

// --- Task 9.18: TemplateData HasLearnings ---

func TestTemplateData_HasLearnings(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with valid content
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"),
		[]byte("## testing: patterns [review, runner/runner.go:1]\nImportant pattern"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	replacements, _, err := buildKnowledgeReplacements(tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	hasLearnings := replacements["__LEARNINGS_CONTENT__"] != ""
	if !hasLearnings {
		t.Error("HasLearnings should be true when LEARNINGS.md has validated content")
	}

	// Without LEARNINGS.md
	emptyDir := t.TempDir()
	emptyReplacements, _, err := buildKnowledgeReplacements(emptyDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	hasLearningsEmpty := emptyReplacements["__LEARNINGS_CONTENT__"] != ""
	if hasLearningsEmpty {
		t.Error("HasLearnings should be false when no LEARNINGS.md")
	}
}

// --- Task 9.19: Template syntax safety ---

func TestBuildKnowledgeReplacements_TemplateSyntaxSafe(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with Go template syntax that would crash text/template
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	dangerousContent := "## testing: templates [review, runner/runner.go:1]\nUse {{.Field}} and {{range .Items}}{{end}} in templates"
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(dangerousContent), 0644); err != nil {
		t.Fatalf("write LEARNINGS.md: %v", err)
	}

	replacements, _, err := buildKnowledgeReplacements(tmpDir)
	if err != nil {
		t.Fatalf("buildKnowledgeReplacements error: %v", err)
	}

	// Stage 2 injection: content with {{ should be preserved literally
	learnings := replacements["__LEARNINGS_CONTENT__"]
	if !strings.Contains(learnings, "{{.Field}}") {
		t.Errorf("template syntax should be preserved literally, got %q", learnings)
	}
	if !strings.Contains(learnings, "{{range .Items}}") {
		t.Errorf("template range syntax should be preserved literally, got %q", learnings)
	}
}

// --- Error path tests for buildKnowledgeReplacements ---

func TestBuildKnowledgeReplacements_LearningsReadError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create LEARNINGS.md as a directory — os.ReadFile returns non-NotExist error
	if err := os.MkdirAll(filepath.Join(tmpDir, "LEARNINGS.md"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, _, err := buildKnowledgeReplacements(tmpDir)
	if err == nil {
		t.Fatal("expected error when LEARNINGS.md is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "runner: build knowledge: read learnings:") {
		t.Errorf("error should contain 'runner: build knowledge: read learnings:', got %q", err.Error())
	}
}

func TestBuildKnowledgeReplacements_RuleReadError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create ralph rules dir with a file that is actually a directory
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create ralph-broken.md as a directory — os.ReadFile will fail
	if err := os.MkdirAll(filepath.Join(rulesDir, "ralph-broken.md"), 0755); err != nil {
		t.Fatalf("mkdir broken: %v", err)
	}

	_, _, err := buildKnowledgeReplacements(tmpDir)
	if err == nil {
		t.Fatal("expected error when ralph-broken.md is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "runner: build knowledge: read rule ralph-broken.md:") {
		t.Errorf("error should contain 'runner: build knowledge: read rule ralph-broken.md:', got %q", err.Error())
	}
}

// --- Test for ValidateLearnings entry without citation ---

func TestValidateLearnings_NoCitation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a referenced file for the second entry
	if err := os.MkdirAll(filepath.Join(tmpDir, "runner"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner", "runner.go"), []byte("package runner"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// First entry has no citation, second has valid citation
	content := "## general: patterns\nGeneral pattern without file citation\n\n## testing: specific [review, runner/runner.go:10]\nSpecific with citation"

	valid, stale := ValidateLearnings(tmpDir, content)

	// No-citation entry should be treated as valid
	if !strings.Contains(valid, "General pattern without file citation") {
		t.Errorf("valid should contain no-citation entry, got %q", valid)
	}
	if !strings.Contains(valid, "Specific with citation") {
		t.Errorf("valid should contain cited entry, got %q", valid)
	}
	if stale != "" {
		t.Errorf("stale should be empty (no stale entries), got %q", stale)
	}
}
