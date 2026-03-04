package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// --- Task 9: ParseDistillOutput tests ---

func TestParseDistillOutput_ValidOutput(t *testing.T) {
	raw := `Some preamble text from Claude.

BEGIN_DISTILLED_OUTPUT

## CATEGORY: testing
## testing: assertion-quality [review, runner/runner_test.go:42] [freq:8] [stage:review]
Count assertions: strings.Count >= N, not just strings.Contains.

## CATEGORY: errors
## errors: wrapping-consistency [review, runner/runner.go:85] [freq:12] [stage:both] ANCHOR
Error wrapping: ALL returns must wrap with same prefix.

END_DISTILLED_OUTPUT

Some trailing text.`

	output, err := ParseDistillOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Categories) != 2 {
		t.Errorf("Categories count = %d, want 2", len(output.Categories))
	}

	testingEntries := output.Categories["testing"]
	if len(testingEntries) != 1 {
		t.Fatalf("testing entries = %d, want 1", len(testingEntries))
	}
	if testingEntries[0].Freq != 8 {
		t.Errorf("testing entry freq = %d, want 8", testingEntries[0].Freq)
	}
	if testingEntries[0].Stage != "review" {
		t.Errorf("testing entry stage = %q, want %q", testingEntries[0].Stage, "review")
	}
	if testingEntries[0].IsAnchor {
		t.Error("testing entry should not be ANCHOR")
	}

	errorsEntries := output.Categories["errors"]
	if len(errorsEntries) != 1 {
		t.Fatalf("errors entries = %d, want 1", len(errorsEntries))
	}
	if errorsEntries[0].Freq != 12 {
		t.Errorf("errors entry freq = %d, want 12", errorsEntries[0].Freq)
	}
	if !errorsEntries[0].IsAnchor {
		t.Error("errors entry should be ANCHOR")
	}
	if errorsEntries[0].Stage != "both" {
		t.Errorf("errors entry stage = %q, want %q", errorsEntries[0].Stage, "both")
	}

	// Verify content includes body text
	if !strings.Contains(errorsEntries[0].Content, "ALL returns must wrap") {
		t.Errorf("errors entry content missing body text, got: %q", errorsEntries[0].Content)
	}

	// CompressedLearnings should be the content between markers
	if !strings.Contains(output.CompressedLearnings, "## CATEGORY: testing") {
		t.Error("CompressedLearnings should contain category headers")
	}
}

func TestParseDistillOutput_MissingMarkers(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"no markers at all", "just some text without markers"},
		{"only begin marker", "BEGIN_DISTILLED_OUTPUT\nsome content"},
		{"only end marker", "some content\nEND_DISTILLED_OUTPUT"},
		{"end before begin", "END_DISTILLED_OUTPUT\nBEGIN_DISTILLED_OUTPUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDistillOutput(tt.raw)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "bad format") {
				t.Errorf("error = %q, want containing %q", err.Error(), "bad format")
			}
			if !strings.Contains(err.Error(), "runner: distill: parse:") {
				t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: parse:")
			}
		})
	}
}

func TestParseDistillOutput_EmptyBetweenMarkers(t *testing.T) {
	raw := "BEGIN_DISTILLED_OUTPUT\n\nEND_DISTILLED_OUTPUT"
	_, err := ParseDistillOutput(raw)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("error = %q, want containing %q", err.Error(), "empty content")
	}
	if !strings.Contains(err.Error(), "bad format") {
		t.Errorf("error = %q, want containing %q", err.Error(), "bad format")
	}
}

func TestParseDistillOutput_NewCategory(t *testing.T) {
	raw := `BEGIN_DISTILLED_OUTPUT

NEW_CATEGORY: concurrency

## CATEGORY: concurrency
## concurrency: mutex-guards [review, runner/pool.go:10] [freq:3] [stage:execute]
Always protect shared state with sync.Mutex.

END_DISTILLED_OUTPUT`

	output, err := ParseDistillOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.NewCategories) != 1 {
		t.Fatalf("NewCategories count = %d, want 1", len(output.NewCategories))
	}
	if output.NewCategories[0] != "concurrency" {
		t.Errorf("NewCategories[0] = %q, want %q", output.NewCategories[0], "concurrency")
	}

	entries := output.Categories["concurrency"]
	if len(entries) != 1 {
		t.Fatalf("concurrency entries = %d, want 1", len(entries))
	}
	if entries[0].Freq != 3 {
		t.Errorf("entry freq = %d, want 3", entries[0].Freq)
	}
}

func TestParseDistillOutput_FreqMonotonicity(t *testing.T) {
	raw := `BEGIN_DISTILLED_OUTPUT

## CATEGORY: testing
## testing: assertion-quality [review, runner_test.go:42] [freq:5] [stage:review]
Count assertions must use strings.Count.

## testing: error-paths [review, runner_test.go:100] [freq:3] [stage:both]
Test all error return paths.

END_DISTILLED_OUTPUT`

	output, err := ParseDistillOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old freqs: assertion-quality was 8, error-paths was 3
	oldFreqs := map[string]int{
		"testing:assertion-quality": 8,
		"testing:error-paths":      3,
	}

	ValidateFreqMonotonicity(output, oldFreqs)

	entries := output.Categories["testing"]
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	// assertion-quality: new freq 5 < old 8 → corrected to 8
	if entries[0].Freq != 8 {
		t.Errorf("assertion-quality freq = %d, want 8 (corrected from 5)", entries[0].Freq)
	}
	if !strings.Contains(entries[0].Content, "[freq:8]") {
		t.Errorf("content not updated with corrected freq, got: %q", entries[0].Content)
	}

	// error-paths: new freq 3 == old 3 → unchanged
	if entries[1].Freq != 3 {
		t.Errorf("error-paths freq = %d, want 3", entries[1].Freq)
	}
}

// --- Task 9: DetectProjectScope tests ---

func TestDetectProjectScope_GoProject(t *testing.T) {
	dir := t.TempDir()
	// Create .go files at top 2 levels
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "util.go"), []byte("package pkg"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectProjectScope(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "**/*.go") {
		t.Errorf("result = %q, want containing %q", result, "**/*.go")
	}
	if !strings.Contains(result, "Relevant globs:") {
		t.Errorf("result = %q, want containing %q", result, "Relevant globs:")
	}
}

func TestDetectProjectScope_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := DetectProjectScope(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "No language-specific patterns detected" {
		t.Errorf("result = %q, want %q", result, "No language-specific patterns detected")
	}
}

// --- Story 6.8 Task 11: Cross-language scope hints ---

func TestDetectProjectScope_CrossLanguage(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string // relative path → content
		wantGlobs []string          // substrings expected in result
		wantMsg   string            // exact match if set
	}{
		{
			name:      "Python project",
			files:     map[string]string{"app.py": "import os", "tests/test_app.py": "import unittest"},
			wantGlobs: []string{"**/*.py"},
		},
		{
			name:      "JS/TS project",
			files:     map[string]string{"index.ts": "const x = 1", "src/App.tsx": "export default"},
			wantGlobs: []string{"**/*.ts", "**/*.tsx"},
		},
		{
			name:      "Java project",
			files:     map[string]string{"src/Main.java": "public class Main {}"},
			wantGlobs: []string{"**/*.java"},
		},
		{
			name:      "Mixed Go+Python",
			files:     map[string]string{"main.go": "package main", "script.py": "print()"},
			wantGlobs: []string{"**/*.go", "**/*.py"},
		},
		{
			name:    "Empty project",
			files:   map[string]string{},
			wantMsg: "No language-specific patterns detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for relPath, content := range tt.files {
				fullPath := filepath.Join(dir, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result, err := DetectProjectScope(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantMsg != "" {
				if result != tt.wantMsg {
					t.Errorf("result = %q, want %q", result, tt.wantMsg)
				}
				return
			}

			for _, glob := range tt.wantGlobs {
				if !strings.Contains(result, glob) {
					t.Errorf("result = %q, want containing %q", result, glob)
				}
			}
			if !strings.Contains(result, "Relevant globs:") {
				t.Errorf("result = %q, want containing %q", result, "Relevant globs:")
			}
		})
	}
}

// --- Task 9: BackupFile tests ---

func TestBackupFile_TwoGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	// Create original file
	if err := os.WriteFile(path, []byte("version 1"), 0644); err != nil {
		t.Fatal(err)
	}

	// First backup: test.md → test.md.bak
	if err := BackupFile(path); err != nil {
		t.Fatalf("first backup: %v", err)
	}

	bakContent, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read .bak: %v", err)
	}
	if string(bakContent) != "version 1" {
		t.Errorf(".bak content = %q, want %q", bakContent, "version 1")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("original should be removed after backup")
	}

	// Create new version
	if err := os.WriteFile(path, []byte("version 2"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second backup: .bak → .bak.1, test.md → .bak
	if err := BackupFile(path); err != nil {
		t.Fatalf("second backup: %v", err)
	}

	bak1Content, err := os.ReadFile(path + ".bak.1")
	if err != nil {
		t.Fatalf("read .bak.1: %v", err)
	}
	if string(bak1Content) != "version 1" {
		t.Errorf(".bak.1 content = %q, want %q", bak1Content, "version 1")
	}

	bakContent2, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read .bak after second: %v", err)
	}
	if string(bakContent2) != "version 2" {
		t.Errorf(".bak content = %q, want %q", bakContent2, "version 2")
	}
}

func TestBackupFile_MissingSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.md")

	err := BackupFile(path)
	if err != nil {
		t.Fatalf("expected no-op for missing file, got: %v", err)
	}
}

func TestBackupDistillationFiles_AllFiles(t *testing.T) {
	dir := t.TempDir()

	// Create LEARNINGS.md
	if err := os.WriteFile(filepath.Join(dir, "LEARNINGS.md"), []byte("learnings"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create ralph-*.md files
	rulesDir := filepath.Join(dir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-testing.md"), []byte("testing rules"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create distill-state.json
	ralphDir := filepath.Join(dir, ".ralph")
	if err := os.WriteFile(filepath.Join(ralphDir, "distill-state.json"), []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	err := BackupDistillationFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify LEARNINGS.md.bak exists
	if _, err := os.Stat(filepath.Join(dir, "LEARNINGS.md.bak")); err != nil {
		t.Errorf("LEARNINGS.md.bak should exist: %v", err)
	}

	// Verify ralph-testing.md.bak exists
	if _, err := os.Stat(filepath.Join(rulesDir, "ralph-testing.md.bak")); err != nil {
		t.Errorf("ralph-testing.md.bak should exist: %v", err)
	}

	// Verify distill-state.json.bak exists
	if _, err := os.Stat(filepath.Join(ralphDir, "distill-state.json.bak")); err != nil {
		t.Errorf("distill-state.json.bak should exist: %v", err)
	}
}

// --- Task 9: WriteDistillOutput tests ---

func TestWriteDistillOutput_MultiFile(t *testing.T) {
	dir := t.TempDir()
	state := &DistillState{Version: 1}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed\n\nEntry 1\nEntry 2",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "## testing: a1 [r, f:1] [freq:2] [stage:review]", Freq: 2, Stage: "review"},
				{Content: "## testing: a2 [r, f:2] [freq:3] [stage:review]", Freq: 3, Stage: "review"},
				{Content: "## testing: a3 [r, f:3] [freq:4] [stage:review]", Freq: 4, Stage: "review"},
				{Content: "## testing: a4 [r, f:4] [freq:5] [stage:review]", Freq: 5, Stage: "review"},
				{Content: "## testing: a5 [r, f:5] [freq:6] [stage:review]", Freq: 6, Stage: "review"},
			},
		},
	}

	files, err := WriteDistillOutput(dir, output, state, "Project languages detected. Relevant globs: **/*.go, **/*_test.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("commit pending: %v", err)
	}

	// Verify LEARNINGS.md
	learnings, readErr := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if readErr != nil {
		t.Fatalf("read LEARNINGS.md: %v", readErr)
	}
	if !strings.Contains(string(learnings), "# Compressed") {
		t.Error("LEARNINGS.md should contain compressed content")
	}

	// Verify ralph-testing.md exists (5 entries >= 5 threshold)
	testingPath := filepath.Join(dir, ".ralph", "rules", "ralph-testing.md")
	testingContent, readErr := os.ReadFile(testingPath)
	if readErr != nil {
		t.Fatalf("read ralph-testing.md: %v", readErr)
	}
	if !strings.Contains(string(testingContent), "---\nglobs:") {
		t.Error("ralph-testing.md should have YAML frontmatter with globs")
	}
	if !strings.Contains(string(testingContent), "# testing") {
		t.Error("ralph-testing.md should have category header")
	}
}

func TestWriteDistillOutput_LazyCreation(t *testing.T) {
	dir := t.TempDir()
	state := &DistillState{Version: 1}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"config": {
				{Content: "## config: yaml [r, f:1] [freq:1]", Freq: 1, Stage: "both"},
				{Content: "## config: env [r, f:2] [freq:2]", Freq: 2, Stage: "both"},
			},
		},
	}

	files, err := WriteDistillOutput(dir, output, state, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("commit pending: %v", err)
	}

	// config has only 2 entries (< 5), should go to ralph-misc.md
	configPath := filepath.Join(dir, ".ralph", "rules", "ralph-config.md")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("ralph-config.md should NOT exist (< 5 entries, merged into misc)")
	}

	miscPath := filepath.Join(dir, ".ralph", "rules", "ralph-misc.md")
	miscContent, readErr := os.ReadFile(miscPath)
	if readErr != nil {
		t.Fatalf("read ralph-misc.md: %v", readErr)
	}
	if !strings.Contains(string(miscContent), "config") {
		t.Error("ralph-misc.md should contain config entries")
	}
}

func TestWriteDistillOutput_CriticalFile(t *testing.T) {
	dir := t.TempDir()
	state := &DistillState{Version: 1}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"errors": {
				{Content: "## errors: wrapping [r, f:1] [freq:12] [stage:both]", Freq: 12, Stage: "both", IsAnchor: false},
			},
		},
	}

	files, err := WriteDistillOutput(dir, output, state, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("commit pending: %v", err)
	}

	criticalPath := filepath.Join(dir, ".ralph", "rules", "ralph-critical.md")
	criticalContent, readErr := os.ReadFile(criticalPath)
	if readErr != nil {
		t.Fatalf("read ralph-critical.md: %v", readErr)
	}
	if !strings.Contains(string(criticalContent), "ANCHOR") {
		t.Error("ralph-critical.md entries should have ANCHOR marker")
	}
	if !strings.Contains(string(criticalContent), `globs: ["**"]`) {
		t.Error("ralph-critical.md should have globs: [\"**\"] in frontmatter")
	}

	// AC 7.8a: source category should have reference instead of promoted entry
	miscPath := filepath.Join(dir, ".ralph", "rules", "ralph-misc.md")
	miscContent, readErr := os.ReadFile(miscPath)
	if readErr != nil {
		t.Fatalf("read ralph-misc.md: %v", readErr)
	}
	if !strings.Contains(string(miscContent), "Promoted to ralph-critical.md") {
		t.Error("source category (ralph-misc.md) should have promotion reference for dedup (AC 7.8a)")
	}
	if !strings.Contains(string(miscContent), "freq:12") {
		t.Errorf("promotion reference should include freq value, got: %q", string(miscContent))
	}
}

func TestWriteDistillOutput_MiscNoGlobs(t *testing.T) {
	dir := t.TempDir()
	state := &DistillState{Version: 1}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"misc": {
				{Content: "## misc: general [r, f:1] [freq:1]", Freq: 1, Stage: "both"},
			},
		},
	}

	files, err := WriteDistillOutput(dir, output, state, "Relevant globs: **/*.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("commit pending: %v", err)
	}

	miscPath := filepath.Join(dir, ".ralph", "rules", "ralph-misc.md")
	miscContent, readErr := os.ReadFile(miscPath)
	if readErr != nil {
		t.Fatalf("read ralph-misc.md: %v", readErr)
	}
	// ralph-misc.md should NOT have globs in frontmatter (L5)
	if strings.Contains(string(miscContent), "globs:") {
		t.Error("ralph-misc.md should NOT have globs in frontmatter (always loaded, L5)")
	}
}

func TestWriteDistillOutput_NewCategoriesUpdateState(t *testing.T) {
	dir := t.TempDir()
	state := &DistillState{Version: 1, Categories: []string{"testing"}}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "## testing: a1 [r, f:1] [freq:1]", Freq: 1, Stage: "review"},
			},
		},
		NewCategories: []string{"concurrency", "testing"}, // "testing" already exists — dedup
	}

	files, err := WriteDistillOutput(dir, output, state, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("commit pending: %v", err)
	}

	// Verify new category added
	if !containsString(state.Categories, "concurrency") {
		t.Errorf("state.Categories should contain %q, got: %v", "concurrency", state.Categories)
	}
	// Verify existing category not duplicated
	count := 0
	for _, c := range state.Categories {
		if c == "testing" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("state.Categories has %d copies of %q, want 1", count, "testing")
	}
}

// --- Task 9: WriteDistillIndex tests ---

func TestWriteDistillIndex_MarkdownTable(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test rule files
	testingContent := "---\nglobs: [**/*.go]\n---\n# testing\n\n## testing: a1\nContent\n\n## testing: a2\nContent\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-testing.md"), []byte(testingContent), 0644); err != nil {
		t.Fatal(err)
	}
	miscContent := "# Miscellaneous\n\n## misc: general\nContent\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-misc.md"), []byte(miscContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := WriteDistillIndex(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	indexPath := filepath.Join(rulesDir, "ralph-index.md")
	indexContent, readErr := os.ReadFile(indexPath)
	if readErr != nil {
		t.Fatalf("read ralph-index.md: %v", readErr)
	}

	content := string(indexContent)
	if !strings.Contains(content, "| File |") {
		t.Error("index should contain table header")
	}
	if !strings.Contains(content, "ralph-testing.md") {
		t.Error("index should list ralph-testing.md")
	}
	if !strings.Contains(content, "ralph-misc.md") {
		t.Error("index should list ralph-misc.md")
	}
	// Index should NOT list itself
	if strings.Count(content, "ralph-index.md") > 0 {
		t.Error("index should NOT list itself")
	}
}

// --- Task 9: Prompt tests ---

func TestDistillPrompt_Instructions(t *testing.T) {
	required := []string{
		"compression agent",
		"<= 100 lines",
		"50% of budget",
		"stale-cited entries",
		"merge duplicate",
		"[needs-formatting]",
		"Auto-promote",
		">= 5 entries",
		"freq:N",
		"ANCHOR",
		"VIOLATION:",
		"[stage:execute|review|both]",
	}

	for _, keyword := range required {
		if !strings.Contains(distillTemplate, keyword) {
			t.Errorf("distill prompt missing required keyword: %q", keyword)
		}
	}
}

func TestDistillPrompt_OutputProtocol(t *testing.T) {
	if !strings.Contains(distillTemplate, "BEGIN_DISTILLED_OUTPUT") {
		t.Error("distill prompt missing BEGIN_DISTILLED_OUTPUT marker")
	}
	if !strings.Contains(distillTemplate, "END_DISTILLED_OUTPUT") {
		t.Error("distill prompt missing END_DISTILLED_OUTPUT marker")
	}
	if !strings.Contains(distillTemplate, "## CATEGORY:") {
		t.Error("distill prompt missing ## CATEGORY: format")
	}
}

func TestDistillPrompt_Categories(t *testing.T) {
	canonical := []string{
		"testing", "errors", "config", "cli",
		"architecture", "performance", "security", "misc",
	}

	for _, cat := range canonical {
		if !strings.Contains(distillTemplate, cat) {
			t.Errorf("distill prompt missing canonical category: %q", cat)
		}
	}
}

func TestDistillPrompt_Placeholders(t *testing.T) {
	placeholders := []string{
		"__LEARNINGS_CONTENT__",
		"__SCOPE_HINTS__",
		"__EXISTING_RULES__",
	}

	for _, ph := range placeholders {
		if !strings.Contains(distillTemplate, ph) {
			t.Errorf("distill prompt missing placeholder: %q", ph)
		}
	}
}

// --- Story 6.5c: Validation tests ---

func TestValidateDistillation_AllPass(t *testing.T) {
	oldContent := "## testing: a1 [review, runner/test.go:1]\nSome content\n## errors: wrapping [review, runner/err.go:2]\nMore content\n"
	output := &DistillOutput{
		CompressedLearnings: "## testing: a1 [review, runner/test.go:1]\nSome compressed\n## errors: wrapping [review, runner/err.go:2]\nMore compressed\n",
		Categories:          map[string][]DistilledEntry{"testing": {{Content: "entry", Freq: 2}}},
	}

	err := ValidateDistillation(output, oldContent, 200)
	if err != nil {
		t.Fatalf("ValidateDistillation: want nil, got %v", err)
	}
}

func TestValidateDistillation_BudgetExceeded(t *testing.T) {
	// Create output with > 10 lines, budget = 10
	lines := strings.Repeat("line\n", 15)
	output := &DistillOutput{CompressedLearnings: lines}

	err := ValidateDistillation(output, "", 10)
	if err == nil {
		t.Fatal("ValidateDistillation: want error for budget exceeded, got nil")
	}
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("error should wrap ErrValidationFailed, got: %v", err)
	}
	if !strings.Contains(err.Error(), "criterion 1") {
		t.Errorf("error should mention criterion 1, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("error should mention budget exceeded, got: %q", err.Error())
	}
}

func TestValidateDistillation_CitationLoss(t *testing.T) {
	// Old content with 10 citations, new output preserves only 5 (50% < 80%)
	var oldLines []string
	for i := 0; i < 10; i++ {
		oldLines = append(oldLines, fmt.Sprintf("## cat: topic%d [review, file%d.go:%d]", i, i, i))
	}
	oldContent := strings.Join(oldLines, "\n")

	// New output preserves only citations 0-4
	var newLines []string
	for i := 0; i < 5; i++ {
		newLines = append(newLines, fmt.Sprintf("## cat: topic%d [review, file%d.go:%d]", i, i, i))
	}
	output := &DistillOutput{CompressedLearnings: strings.Join(newLines, "\n")}

	err := ValidateDistillation(output, oldContent, 200)
	if err == nil {
		t.Fatal("ValidateDistillation: want error for citation loss, got nil")
	}
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("error should wrap ErrValidationFailed, got: %v", err)
	}
	if !strings.Contains(err.Error(), "criterion 2") {
		t.Errorf("error should mention criterion 2, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "citation preservation") {
		t.Errorf("error should mention citation preservation, got: %q", err.Error())
	}
}

func TestValidateDistillation_NeedsFormattingDropped(t *testing.T) {
	oldContent := "## testing: bad-format [review, f:1] [needs-formatting]\nBroken entry\n"
	// New output preserves citation but drops the NF topic "bad-format" entirely
	output := &DistillOutput{CompressedLearnings: "## testing: good [review, f:1]\nGood entry\n"}

	err := ValidateDistillation(output, oldContent, 200)
	if err == nil {
		t.Fatal("ValidateDistillation: want error for dropped needs-formatting, got nil")
	}
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("error should wrap ErrValidationFailed, got: %v", err)
	}
	if !strings.Contains(err.Error(), "criterion 3") {
		t.Errorf("error should mention criterion 3, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "silently dropped") {
		t.Errorf("error should mention silently dropped, got: %q", err.Error())
	}
}

func TestValidateDistillation_NeedsFormattingFixed(t *testing.T) {
	oldContent := "## testing: bad-format [review, f:1] [needs-formatting]\nBroken entry\n"
	// Topic "bad-format" present WITHOUT [needs-formatting] tag — fixed
	output := &DistillOutput{CompressedLearnings: "## testing: bad-format [review, f:1]\nFixed entry\n"}

	err := ValidateDistillation(output, oldContent, 200)
	if err != nil {
		t.Fatalf("ValidateDistillation: want nil (fixed), got %v", err)
	}
}

func TestValidateDistillation_NeedsFormattingPreserved(t *testing.T) {
	oldContent := "## testing: bad-format [review, f:1] [needs-formatting]\nBroken entry\n"
	// Topic still present with [needs-formatting] — preserved, not dropped
	output := &DistillOutput{CompressedLearnings: "## testing: bad-format [review, f:1] [needs-formatting]\nStill broken\n"}

	err := ValidateDistillation(output, oldContent, 200)
	if err != nil {
		t.Fatalf("ValidateDistillation: want nil (preserved), got %v", err)
	}
}

// --- Story 6.5c: Metrics tests ---

func TestComputeDistillMetrics_AllFields(t *testing.T) {
	oldContent := "## testing: a1 [r, f:1]\ncontent\n## errors: b1 [r, f:2]\ncontent\n## testing: a2 [r, f:3] [needs-formatting]\ncontent\n"
	output := &DistillOutput{
		CompressedLearnings: "## testing: a1 [r, f:1]\ncompressed\n",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "## testing: a1 [r, f:1] [freq:2]", Freq: 2},
				{Content: "## testing: a3 [r, f:4] [freq:12]", Freq: 12},
			},
		},
	}

	metrics := ComputeDistillMetrics(oldContent, output)

	if metrics.EntriesBefore != 3 {
		t.Errorf("EntriesBefore = %d, want 3", metrics.EntriesBefore)
	}
	if metrics.EntriesAfter != 2 {
		t.Errorf("EntriesAfter = %d, want 2", metrics.EntriesAfter)
	}
	if metrics.StaleRemoved != 1 {
		t.Errorf("StaleRemoved = %d, want 1", metrics.StaleRemoved)
	}
	if metrics.CategoriesTotal != 1 {
		t.Errorf("CategoriesTotal = %d, want 1", metrics.CategoriesTotal)
	}
	if metrics.CategoriesPreserved != 1 {
		t.Errorf("CategoriesPreserved = %d, want 1", metrics.CategoriesPreserved)
	}
	if metrics.NeedsFormattingFixed != 1 {
		t.Errorf("NeedsFormattingFixed = %d, want 1", metrics.NeedsFormattingFixed)
	}
	if metrics.T1Promotions != 1 {
		t.Errorf("T1Promotions = %d, want 1", metrics.T1Promotions)
	}
	if metrics.LastDistillTime == "" {
		t.Error("LastDistillTime should be set")
	}
}

func TestComputeDistillMetrics_NoStale(t *testing.T) {
	oldContent := "## testing: a1 [r, f:1]\ncontent\n"
	output := &DistillOutput{
		CompressedLearnings: "## testing: a1 [r, f:1]\ncompressed\n",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "entry1", Freq: 2},
				{Content: "entry2", Freq: 3},
			},
		},
	}

	metrics := ComputeDistillMetrics(oldContent, output)
	if metrics.StaleRemoved != 0 {
		t.Errorf("StaleRemoved = %d, want 0 (more entries after than before)", metrics.StaleRemoved)
	}
}

// --- Story 6.5c: Intent file tests ---

func TestWriteIntentFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	intent := &DistillIntent{
		Timestamp: "2026-03-02T14:30:00Z",
		Files:     []string{"LEARNINGS.md", ".ralph/rules/ralph-testing.md"},
		Phase:     "write",
	}

	if err := WriteIntentFile(dir, intent); err != nil {
		t.Fatalf("WriteIntentFile: %v", err)
	}

	loaded, err := ReadIntentFile(dir)
	if err != nil {
		t.Fatalf("ReadIntentFile: %v", err)
	}
	if loaded == nil {
		t.Fatal("ReadIntentFile: want non-nil, got nil")
	}
	if loaded.Timestamp != "2026-03-02T14:30:00Z" {
		t.Errorf("Timestamp = %q, want %q", loaded.Timestamp, "2026-03-02T14:30:00Z")
	}
	if len(loaded.Files) != 2 {
		t.Fatalf("Files count = %d, want 2", len(loaded.Files))
	}
	if loaded.Files[0] != "LEARNINGS.md" {
		t.Errorf("Files[0] = %q, want %q", loaded.Files[0], "LEARNINGS.md")
	}
	if loaded.Phase != "write" {
		t.Errorf("Phase = %q, want %q", loaded.Phase, "write")
	}
}

func TestReadIntentFile_NotExist(t *testing.T) {
	dir := t.TempDir()

	intent, err := ReadIntentFile(dir)
	if err != nil {
		t.Fatalf("ReadIntentFile: want nil error for missing file, got %v", err)
	}
	if intent != nil {
		t.Errorf("ReadIntentFile: want nil intent for missing file, got %+v", intent)
	}
}

func TestCommitPendingFiles_AllRenamed(t *testing.T) {
	dir := t.TempDir()

	// Create .pending files
	file1 := filepath.Join(dir, "LEARNINGS.md")
	file2 := filepath.Join(dir, "ralph-testing.md")
	if err := os.WriteFile(file1+".pending", []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2+".pending", []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	err := CommitPendingFiles([]string{file1, file2})
	if err != nil {
		t.Fatalf("CommitPendingFiles: %v", err)
	}

	// Verify renamed
	content1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("read target file1: %v", err)
	}
	if string(content1) != "content1" {
		t.Errorf("file1 content = %q, want %q", string(content1), "content1")
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("read target file2: %v", err)
	}
	if string(content2) != "content2" {
		t.Errorf("file2 content = %q, want %q", string(content2), "content2")
	}

	// Verify .pending files gone
	if _, err := os.Stat(file1 + ".pending"); !os.IsNotExist(err) {
		t.Error("file1.pending should not exist after commit")
	}
	if _, err := os.Stat(file2 + ".pending"); !os.IsNotExist(err) {
		t.Error("file2.pending should not exist after commit")
	}
}

func TestCommitPendingFiles_SkipsMissing(t *testing.T) {
	dir := t.TempDir()

	// file1 has .pending, file2 does not (already committed or never written)
	file1 := filepath.Join(dir, "LEARNINGS.md")
	file2 := filepath.Join(dir, "ralph-testing.md")
	if err := os.WriteFile(file1+".pending", []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}

	err := CommitPendingFiles([]string{file1, file2})
	if err != nil {
		t.Fatalf("CommitPendingFiles: want no error when .pending missing, got %v", err)
	}

	// file1 should be renamed
	if _, err := os.Stat(file1); err != nil {
		t.Errorf("file1 should exist after commit: %v", err)
	}
}

func TestRestoreDistillationBackups_RestoresFiles(t *testing.T) {
	dir := t.TempDir()

	// Create .ralph/rules directory
	rulesDir := filepath.Join(dir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create backup files
	learningsPath := filepath.Join(dir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath+".bak", []byte("original learnings"), 0644); err != nil {
		t.Fatal(err)
	}
	// Write a "corrupt" current file to verify it gets overwritten
	if err := os.WriteFile(learningsPath, []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}

	rulePath := filepath.Join(rulesDir, "ralph-testing.md")
	if err := os.WriteFile(rulePath+".bak", []byte("original rule"), 0644); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(dir, ".ralph", "distill-state.json")
	if err := os.WriteFile(statePath+".bak", []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	err := RestoreDistillationBackups(dir)
	if err != nil {
		t.Fatalf("RestoreDistillationBackups: unexpected error: %v", err)
	}

	// Verify LEARNINGS.md restored
	data, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("LEARNINGS.md not found after restore: %v", err)
	}
	if string(data) != "original learnings" {
		t.Errorf("LEARNINGS.md content = %q, want %q", string(data), "original learnings")
	}

	// Verify rule file restored
	data, err = os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("ralph-testing.md not found after restore: %v", err)
	}
	if string(data) != "original rule" {
		t.Errorf("ralph-testing.md content = %q, want %q", string(data), "original rule")
	}

	// Verify state restored
	data, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("distill-state.json not found after restore: %v", err)
	}
	if string(data) != `{"version":1}` {
		t.Errorf("distill-state.json content = %q, want %q", string(data), `{"version":1}`)
	}

	// Verify .bak files removed
	if _, err := os.Stat(learningsPath + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Error("LEARNINGS.md.bak should be removed after restore")
	}
}

func TestRestoreDistillationBackups_MissingBak(t *testing.T) {
	dir := t.TempDir()

	// Create .ralph/rules directory (empty — no .bak files)
	rulesDir := filepath.Join(dir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create .ralph dir for state path check
	if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}

	// No .bak files exist — should succeed silently
	err := RestoreDistillationBackups(dir)
	if err != nil {
		t.Fatalf("RestoreDistillationBackups with no .bak files: unexpected error: %v", err)
	}
}

// --- WriteDistillOutput additional branch coverage ---

func TestWriteDistillOutput_CriticalMultilineContent(t *testing.T) {
	// Entry with freq>=10, !IsAnchor, no "ANCHOR" in Content, AND "\n" in Content.
	// Covers the rest = e.Content[idx:] branch (knowledge_distill.go:364-365).
	dir := t.TempDir()
	state := &DistillState{Version: 1}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{
					Content:  "## testing: multiline [r, f:1] [freq:12] [stage:review]\nBody content here.",
					Freq:     12,
					Stage:    "review",
					IsAnchor: false,
				},
			},
		},
	}

	files, err := WriteDistillOutput(dir, output, state, "")
	if err != nil {
		t.Fatalf("WriteDistillOutput: unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("CommitPendingFiles: unexpected error: %v", err)
	}

	criticalPath := filepath.Join(dir, ".ralph", "rules", "ralph-critical.md")
	content, readErr := os.ReadFile(criticalPath)
	if readErr != nil {
		t.Fatalf("read ralph-critical.md: %v", readErr)
	}
	// ANCHOR inserted after first line, before newline
	if !strings.Contains(string(content), "ANCHOR") {
		t.Error("ralph-critical.md: ANCHOR marker must be added to multiline entry")
	}
	// Body content preserved after ANCHOR insertion
	if !strings.Contains(string(content), "Body content here.") {
		t.Error("ralph-critical.md: body content must be preserved after ANCHOR insertion")
	}
}

func TestWriteDistillOutput_InvalidGlobWarning(t *testing.T) {
	// scopeHints with syntactically invalid glob: filepath.Match returns error
	// → WARNING printed + continue (invalid glob skipped, no frontmatter written).
	dir := t.TempDir()
	state := &DistillState{Version: 1}

	// 5 entries to trigger category file (not misc)
	entries := make([]DistilledEntry, 5)
	for i := range entries {
		entries[i] = DistilledEntry{Content: fmt.Sprintf("## testing: entry%d [freq:1]", i), Freq: 1}
	}
	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories:          map[string][]DistilledEntry{"testing": entries},
	}

	// "[" is syntactically invalid: unclosed bracket → filepath.Match returns ErrBadPattern
	files, err := WriteDistillOutput(dir, output, state, "Relevant globs: [")
	if err != nil {
		t.Fatalf("WriteDistillOutput with invalid glob: unexpected error: %v", err)
	}
	if err := CommitPendingFiles(files); err != nil {
		t.Fatalf("CommitPendingFiles: unexpected error: %v", err)
	}

	// With invalid glob skipped, no frontmatter should be written
	catPath := filepath.Join(dir, ".ralph", "rules", "ralph-testing.md")
	content, readErr := os.ReadFile(catPath)
	if readErr != nil {
		t.Fatalf("read ralph-testing.md: %v", readErr)
	}
	if strings.Contains(string(content), "globs:") {
		t.Error("invalid glob must be skipped — no frontmatter should be written")
	}
}

func TestWriteDistillOutput_MkdirAllError(t *testing.T) {
	// Block .ralph/rules creation by making .ralph a file → MkdirAll returns error.
	dir := t.TempDir()
	// Create .ralph as a regular file (not directory) to block MkdirAll(.ralph/rules)
	if err := os.WriteFile(filepath.Join(dir, ".ralph"), []byte("blocker"), 0644); err != nil {
		t.Fatal(err)
	}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories:          map[string][]DistilledEntry{},
	}
	_, err := WriteDistillOutput(dir, output, &DistillState{Version: 1}, "")
	if err == nil {
		t.Fatal("WriteDistillOutput: expected error when MkdirAll fails")
	}
	if !strings.Contains(err.Error(), "runner: distill: write:") {
		t.Errorf("WriteDistillOutput error = %q, want containing %q", err.Error(), "runner: distill: write:")
	}
	if !strings.Contains(err.Error(), ".ralph") {
		t.Errorf("WriteDistillOutput inner error = %q, want containing path %q", err.Error(), ".ralph")
	}
}

// --- AutoDistill timeout path (knowledge_distill.go:836-838) ---

// TestAutoDistill_TimeoutError verifies AutoDistill returns a "timeout after Ns" error
// when cfg.DistillTimeout=0 (context.WithTimeout(ctx, 0) is immediately expired).
// Covers the DeadlineExceeded branch at line 836-838.
func TestAutoDistill_TimeoutError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Step 1 of AutoDistill reads LEARNINGS.md — must exist.
	learningsPath := filepath.Join(dir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("## testing: topic [source, file.go:1]\nSome content.\n"), 0644); err != nil {
		t.Fatalf("write LEARNINGS.md: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot:    dir,
		ClaudeCommand:  "/nonexistent/binary",
		DistillTimeout: 0, // context.WithTimeout(ctx, 0) → already expired
	}

	err := AutoDistill(context.Background(), cfg, &DistillState{Version: 1})
	if err == nil {
		t.Fatal("AutoDistill() expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: distill: execute: timeout after") {
		t.Errorf("AutoDistill() error = %q, want to contain %q", err.Error(), "runner: distill: execute: timeout after")
	}
}
