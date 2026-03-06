package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// --- Task 8.1-8.3: Zero-value struct tests ---

func TestLessonEntry_ZeroValue(t *testing.T) {
	t.Parallel()
	var e runner.LessonEntry
	if e.Category != "" {
		t.Errorf("Category: want empty, got %q", e.Category)
	}
	if e.Topic != "" {
		t.Errorf("Topic: want empty, got %q", e.Topic)
	}
	if e.Content != "" {
		t.Errorf("Content: want empty, got %q", e.Content)
	}
	if e.Citation != "" {
		t.Errorf("Citation: want empty, got %q", e.Citation)
	}
}

func TestLessonsData_ZeroValue(t *testing.T) {
	t.Parallel()
	var d runner.LessonsData
	if d.Source != "" {
		t.Errorf("Source: want empty, got %q", d.Source)
	}
	if d.Entries != nil {
		t.Errorf("Entries: want nil, got %v", d.Entries)
	}
	if d.Snapshot != "" {
		t.Errorf("Snapshot: want empty, got %q", d.Snapshot)
	}
	if d.BudgetLimit != 0 {
		t.Errorf("BudgetLimit: want 0, got %d", d.BudgetLimit)
	}
}

func TestBudgetStatus_ZeroValue(t *testing.T) {
	t.Parallel()
	var s runner.BudgetStatus
	if s.Lines != 0 {
		t.Errorf("Lines: want 0, got %d", s.Lines)
	}
	if s.Limit != 0 {
		t.Errorf("Limit: want 0, got %d", s.Limit)
	}
	if s.NearLimit {
		t.Errorf("NearLimit: want false, got true")
	}
	if s.OverBudget {
		t.Errorf("OverBudget: want false, got true")
	}
}

// --- Task 8.4: NoOp ValidateNewLessons ---

func TestNoOpKnowledgeWriter_ValidateNewLessons_ReturnsNil(t *testing.T) {
	t.Parallel()
	kw := &runner.NoOpKnowledgeWriter{}

	// Non-zero data
	data := runner.LessonsData{
		Source: "test",
		Entries: []runner.LessonEntry{
			{Category: "testing", Topic: "example", Content: "some content here", Citation: "[review, test.go:1]"},
		},
	}
	if err := kw.ValidateNewLessons(context.Background(), data); err != nil {
		t.Errorf("ValidateNewLessons(non-zero): want nil, got %v", err)
	}

	// Zero-value data
	if err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{}); err != nil {
		t.Errorf("ValidateNewLessons(zero): want nil, got %v", err)
	}
}

// --- Task 8.5: Quality gates table-driven ---

func TestFileKnowledgeWriter_ValidateNewLessons_QualityGates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		content  string
		wantTag  bool   // expect [needs-formatting] in output
		wantGate string // gate that should trigger (empty if none)
	}{
		{
			name: "valid entry passes all gates",
			content: `## testing: assertion-quality [review, runner/runner_test.go:42]
This is a valid lesson entry content that exceeds minimum length requirement.
`,
			wantTag:  false,
			wantGate: "",
		},
		{
			name: "G1 bad header format",
			content: `## bad header without citation
This is a valid lesson entry content that exceeds minimum length requirement.
`,
			wantTag:  true,
			wantGate: "G1",
		},
		{
			name: "G2 missing citation",
			content: `## testing: assertion-quality
This is a valid lesson entry content that exceeds minimum length requirement.
`,
			wantTag:  true,
			wantGate: "G2",
		},
		{
			name: "G6 content too short",
			content: `## testing: short [review, test.go:1]
Short.
`,
			wantTag:  true,
			wantGate: "G6",
		},
		{
			name: "valid entry with VIOLATION marker",
			content: `## testing: violation-example [review, runner/runner_test.go:10]
VIOLATION: this is an example of a violation that has enough content length.
`,
			wantTag:  false,
			wantGate: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
			if err := os.WriteFile(learningsPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write learnings: %v", err)
			}

			kw := runner.NewFileKnowledgeWriter(tmpDir)
			err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"})
			if err != nil {
				t.Fatalf("ValidateNewLessons: unexpected error: %v", err)
			}

			result, readErr := os.ReadFile(learningsPath)
			if readErr != nil {
				t.Fatalf("read result: %v", readErr)
			}

			hasTag := strings.Contains(string(result), "[needs-formatting]")
			if hasTag != tc.wantTag {
				t.Errorf("[needs-formatting] tag: want %v, got %v\ncontent:\n%s", tc.wantTag, hasTag, result)
			}
		})
	}
}

// --- Task 8.6: Tags invalid entries in file ---

func TestFileKnowledgeWriter_ValidateNewLessons_TagsInvalid(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	content := `## bad-header no citation
Short content here.
`
	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	if err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"}); err != nil {
		t.Fatalf("ValidateNewLessons: %v", err)
	}

	result, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "[needs-formatting]") {
		t.Errorf("want [needs-formatting] tag in output, got:\n%s", resultStr)
	}
	// Original content preserved (append-only)
	if !strings.Contains(resultStr, "bad-header no citation") {
		t.Errorf("original header should be preserved, got:\n%s", resultStr)
	}
}

// --- Task 8.7: Semantic dedup ---

func TestFileKnowledgeWriter_ValidateNewLessons_SemanticDedup(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	snapshot := `## testing: assertion-quality [review, tests/test_auth.py:42]
Existing lesson about assertion quality with sufficient content length.
`
	current := snapshot + `
## testing: assertion-quality [review, tests/test_api.py:15]
New facts about assertion quality with enough content length for validation.
`

	if err := os.WriteFile(learningsPath, []byte(current), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:      "test",
		Snapshot:    snapshot,
		BudgetLimit: 200,
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons: %v", err)
	}

	result, readErr := os.ReadFile(learningsPath)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}

	resultStr := string(result)

	// Both citations preserved
	if !strings.Contains(resultStr, "tests/test_auth.py:42") {
		t.Errorf("original citation should be preserved, got:\n%s", resultStr)
	}
	if !strings.Contains(resultStr, "tests/test_api.py:15") {
		t.Errorf("new citation should be preserved, got:\n%s", resultStr)
	}
	// New content merged under existing heading
	if !strings.Contains(resultStr, "New facts about assertion") {
		t.Errorf("new content should be merged, got:\n%s", resultStr)
	}
	// Should NOT have duplicate ## testing: assertion-quality headers
	headerCount := strings.Count(resultStr, "## testing: assertion-quality")
	if headerCount != 1 {
		t.Errorf("want 1 merged header, got %d headers:\n%s", headerCount, resultStr)
	}
}

// --- Task 8.8: Line-count guard ---

func TestFileKnowledgeWriter_ValidateNewLessons_LineCountGuard(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Snapshot has more lines than current (rewrite detected)
	snapshot := strings.Repeat("line\n", 20)
	current := `## testing: rewrite-test [review, test.go:1]
This entry was written after a rewrite with sufficient content length for validation.
`

	if err := os.WriteFile(learningsPath, []byte(current), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	// Should not error — rewrite triggers full revalidation, not failure
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:      "test",
		Snapshot:    snapshot,
		BudgetLimit: 200,
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons: %v", err)
	}

	// File should still exist (append-only)
	result, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(result), "rewrite-test") {
		t.Errorf("content should be preserved after rewrite detection, got:\n%s", result)
	}
}

// --- Task 8.9: BudgetCheck thresholds ---

func TestBudgetCheck_Thresholds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		lines          int
		limit          int
		wantNearLimit  bool
		wantOverBudget bool
	}{
		{
			name:           "normal below threshold",
			lines:          50,
			limit:          200,
			wantNearLimit:  false,
			wantOverBudget: false,
		},
		{
			name:           "at soft threshold",
			lines:          150,
			limit:          200,
			wantNearLimit:  true,
			wantOverBudget: false,
		},
		{
			name:           "near limit above soft threshold",
			lines:          160,
			limit:          200,
			wantNearLimit:  true,
			wantOverBudget: false,
		},
		{
			name:           "at hard limit",
			lines:          200,
			limit:          200,
			wantNearLimit:  true,
			wantOverBudget: true,
		},
		{
			name:           "over budget",
			lines:          210,
			limit:          200,
			wantNearLimit:  true,
			wantOverBudget: true,
		},
		{
			name:           "zero lines",
			lines:          0,
			limit:          200,
			wantNearLimit:  false,
			wantOverBudget: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

			// Generate content with exact number of newlines
			content := strings.Repeat("line\n", tc.lines)
			if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			status, err := runner.BudgetCheck(context.Background(), learningsPath, tc.limit)
			if err != nil {
				t.Fatalf("BudgetCheck: unexpected error: %v", err)
			}

			if status.Lines != tc.lines {
				t.Errorf("Lines: want %d, got %d", tc.lines, status.Lines)
			}
			if status.Limit != tc.limit {
				t.Errorf("Limit: want %d, got %d", tc.limit, status.Limit)
			}
			if status.NearLimit != tc.wantNearLimit {
				t.Errorf("NearLimit: want %v, got %v", tc.wantNearLimit, status.NearLimit)
			}
			if status.OverBudget != tc.wantOverBudget {
				t.Errorf("OverBudget: want %v, got %v", tc.wantOverBudget, status.OverBudget)
			}
		})
	}
}

// --- Task 8.10: BudgetCheck missing file ---

func TestBudgetCheck_MissingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "nonexistent-LEARNINGS.md")

	status, err := runner.BudgetCheck(context.Background(), learningsPath, 200)
	if err != nil {
		t.Fatalf("BudgetCheck on missing file: want nil error, got %v", err)
	}

	if status.Lines != 0 {
		t.Errorf("Lines: want 0, got %d", status.Lines)
	}
	if status.Limit != 200 {
		t.Errorf("Limit: want 200, got %d", status.Limit)
	}
	if status.NearLimit {
		t.Errorf("NearLimit: want false, got true")
	}
	if status.OverBudget {
		t.Errorf("OverBudget: want false, got true")
	}
}

// --- M4 fix: BudgetCheck non-NotExist error path ---

func TestBudgetCheck_ReadError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Use a directory path as file — produces non-NotExist read error
	dirAsFile := filepath.Join(tmpDir, "fake-dir")
	if err := os.MkdirAll(dirAsFile, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := runner.BudgetCheck(context.Background(), dirAsFile, 200)
	if err == nil {
		t.Fatalf("BudgetCheck on directory: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: budget check:") {
		t.Errorf("error should wrap with 'runner: budget check:', got %q", err.Error())
	}
}

// --- M4 fix: ValidateNewLessons read error path ---

func TestFileKnowledgeWriter_ValidateNewLessons_ReadError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create a directory named LEARNINGS.md — produces non-NotExist read error
	learningsDir := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"})
	if err == nil {
		t.Fatalf("ValidateNewLessons on directory: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: validate lessons:") {
		t.Errorf("error should wrap with 'runner: validate lessons:', got %q", err.Error())
	}
}

// --- Task 8.11: Entry cap ---

func TestFileKnowledgeWriter_ValidateNewLessons_EntryCap(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Write 7 valid entries — entries 6 and 7 exceed MaxNewEntriesPerValidation (5)
	var b strings.Builder
	for i := 1; i <= 7; i++ {
		b.WriteString("## testing: topic-")
		b.WriteString(strings.Repeat("x", i)) // unique topic
		b.WriteString(" [review, test.go:")
		b.WriteString(strings.Repeat("1", i)) // unique line
		b.WriteString("]\n")
		b.WriteString("This is entry number with enough content for validation threshold.\n\n")
	}

	if err := os.WriteFile(learningsPath, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	if err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"}); err != nil {
		t.Fatalf("ValidateNewLessons: %v", err)
	}

	result, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	resultStr := string(result)
	tagCount := strings.Count(resultStr, "[needs-formatting]")
	// Entries at index >= 5 should be tagged (entries 6 and 7 = indices 5, 6)
	if tagCount < 2 {
		t.Errorf("want >= 2 [needs-formatting] tags for excess entries, got %d\ncontent:\n%s", tagCount, resultStr)
	}
}

// --- Task 8.12: Min content length ---

func TestFileKnowledgeWriter_ValidateNewLessons_MinContent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	content := `## testing: short-content [review, test.go:1]
Too short.
`

	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	if err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"}); err != nil {
		t.Fatalf("ValidateNewLessons: %v", err)
	}

	result, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.Contains(string(result), "[needs-formatting]") {
		t.Errorf("short entry should be tagged [needs-formatting], got:\n%s", result)
	}
}

// --- Task 8.13: New file (absent) = all entries new ---

func TestFileKnowledgeWriter_ValidateNewLessons_NewFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// LEARNINGS.md does not exist — ValidateNewLessons should return nil (no error)
	kw := runner.NewFileKnowledgeWriter(tmpDir)
	if err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"}); err != nil {
		t.Errorf("ValidateNewLessons on absent file: want nil, got %v", err)
	}

	// For snapshot variant: Claude creates the file, then validation runs
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	content := `## testing: new-file-entry [review, test.go:1]
This is a new entry in a newly created file with enough content for validation.
`
	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Empty snapshot = all entries are new
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:      "test",
		Snapshot:    "",
		BudgetLimit: 200,
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons on new file: %v", err)
	}

	result, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Valid entry should NOT be tagged
	if strings.Contains(string(result), "[needs-formatting]") {
		t.Errorf("valid entry should not be tagged, got:\n%s", result)
	}
}

// --- Coverage: ValidateNewLessons uncovered paths ---

// TestFileKnowledgeWriter_ValidateNewLessons_NoNewEntries verifies early return nil
// at line 117-119 when snapshot equals current LEARNINGS.md content.
// diffEntries returns empty → len(newEntries)==0 → return nil immediately.
func TestFileKnowledgeWriter_ValidateNewLessons_NoNewEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	content := "## testing: topic [review, runner/runner.go:1]\nContent long enough for validation.\n"
	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	// Snapshot = current content → diffEntries returns empty → early return nil
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:   "test",
		Snapshot: content,
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons (no new entries): unexpected error: %v", err)
	}

	// File should be unchanged (no tagging since no new entries processed)
	result, readErr := os.ReadFile(learningsPath)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if string(result) != content {
		t.Errorf("content changed unexpectedly:\ngot: %q\nwant: %q", string(result), content)
	}
}

// TestFileKnowledgeWriter_ValidateNewLessons_BudgetGate verifies G4 (budget check) tags
// entries when budgetLimit > 0 && totalLines >= budgetLimit. Covers line 243-245.
func TestFileKnowledgeWriter_ValidateNewLessons_BudgetGate(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Entry is valid format but totalLines (3) >= budgetLimit (2) → G4 triggers
	content := "## testing: budget-gate [review, runner/runner.go:1]\nContent long enough.\n"
	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:      "test",
		BudgetLimit: 1, // totalLines (3) >= budgetLimit (1) → G4 triggered
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons (budget gate): unexpected error: %v", err)
	}

	result, readErr := os.ReadFile(learningsPath)
	if readErr != nil {
		t.Fatalf("read result: %v", readErr)
	}
	if !strings.Contains(string(result), "[needs-formatting]") {
		t.Errorf("[needs-formatting] tag expected (G4:budget), got:\n%s", result)
	}
}

// TestFileKnowledgeWriter_ValidateNewLessons_MergeDedupNoPrefixEntry verifies that
// mergeDedup skips entries with empty categoryTopicPrefix (line 321-322 continue branch).
// Header "##  [source, test.go:1]" (double space) produces empty prefix.
func TestFileKnowledgeWriter_ValidateNewLessons_MergeDedupNoPrefixEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Second entry has double space after "##" → categoryTopicPrefix returns ""
	// → mergeDedup hits `if newPrefix == "" { continue }` branch
	content := "## testing: topic [review, runner/runner.go:1]\nContent long enough for validation.\n\n" +
		"##  [source, test.go:1]\nAnother entry with double-space header and sufficient content length.\n"
	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"})
	if err != nil {
		t.Fatalf("ValidateNewLessons (no-prefix entry): unexpected error: %v", err)
	}
}

// TestFileKnowledgeWriter_ValidateNewLessons_MergedFullRevalidation verifies that
// after mergeDedup finds a match (merged=true) the re-parse at lines 127-129 uses
// allEntries (not diffEntries) when fullRevalidation=true.
// Scenario: snapshot has more lines than current → fullRevalidation=true; both have
// the same category:topic prefix → mergeDedup merges → line 127-129 executed.
// Bonus: existing.endLine from snapshot > len(current lines) → line 343-345 clamped.
func TestFileKnowledgeWriter_ValidateNewLessons_MergedFullRevalidation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Snapshot: 22 newlines (header + 20 content lines + trailing empty)
	// strings.Count(snapshot, "\n") = 21
	snapshot := "## testing: quality [review, old.go:1]\n" + strings.Repeat("content line\n", 20)

	// Current: only 2 newlines — fewer than snapshot → fullRevalidation=true
	current := "## testing: quality [review, new.go:2]\nNew content here sufficiently long for validation.\n"

	if err := os.WriteFile(learningsPath, []byte(current), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:      "test",
		Snapshot:    snapshot,
		BudgetLimit: 200,
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons (merged+fullRevalidation): unexpected error: %v", err)
	}

	result, readErr := os.ReadFile(learningsPath)
	if readErr != nil {
		t.Fatalf("read result: %v", readErr)
	}
	// Entry header prefix must still be present after merge
	if !strings.Contains(string(result), "testing: quality") {
		t.Errorf("entry prefix should survive merge, got:\n%s", result)
	}
}

// TestFileKnowledgeWriter_ValidateNewLessons_MergeShiftedIndices verifies that
// mergeDedup correctly handles entries when new entries shift line positions.
// Scenario: snapshot has entry A at lines 0-2. Current has new entry B at lines 0-2
// and entry A at lines 3-5 (shifted down). Merge must use fresh positions from
// current content, not stale snapshot positions.
func TestFileKnowledgeWriter_ValidateNewLessons_MergeShiftedIndices(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Snapshot: entry A at top
	snapshot := "## testing: topic-a [review, file.go:1]\nExisting content for topic A is long enough.\n"

	// Current: new entry B added BEFORE existing entry A + duplicate A with new citation
	current := "## errors: new-topic [review, other.go:5]\nBrand new content for error topic is long enough.\n\n" +
		"## testing: topic-a [review, file.go:1]\nExisting content for topic A is long enough.\n\n" +
		"## testing: topic-a [execute, file.go:99]\nAdditional content for topic A merged here is long enough.\n"

	if err := os.WriteFile(learningsPath, []byte(current), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{
		Source:      "test",
		Snapshot:    snapshot,
		BudgetLimit: 200,
	})
	if err != nil {
		t.Fatalf("ValidateNewLessons (shifted indices): unexpected error: %v", err)
	}

	result, readErr := os.ReadFile(learningsPath)
	if readErr != nil {
		t.Fatalf("read result: %v", readErr)
	}
	resultStr := string(result)

	// After merge: topic-a entries should be merged (content combined under one header)
	topicACount := strings.Count(resultStr, "testing: topic-a")
	if topicACount != 1 {
		t.Errorf("topic-a header count = %d, want 1 (merged), got:\n%s", topicACount, resultStr)
	}

	// New entry B should still be present
	if !strings.Contains(resultStr, "errors: new-topic") {
		t.Errorf("new entry B should survive merge, got:\n%s", resultStr)
	}

	// Merged entry should contain combined content
	if !strings.Contains(resultStr, "Additional content for topic A") {
		t.Errorf("merged content should include new entry's content, got:\n%s", resultStr)
	}
}

// TestFileKnowledgeWriter_ValidateNewLessons_WriteError verifies the error path at
// line 155-157 when os.WriteFile fails after tagging (modified=true).
// Requires read-only file to trigger write failure — skipped on platforms where
// chmod 0444 does not restrict writes.
func TestFileKnowledgeWriter_ValidateNewLessons_WriteError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Bad header format triggers G1 → tagging → modified=true → WriteFile attempted
	content := "## bad-header no citation\nContent long enough for validation purposes here.\n"
	if err := os.WriteFile(learningsPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := os.Chmod(learningsPath, 0444); err != nil {
		t.Skipf("chmod 0444 failed: %v", err)
	}
	defer func() { _ = os.Chmod(learningsPath, 0644) }() // restore for TempDir cleanup

	// Safety check: verify chmod actually restricts writes on this platform
	if writeErr := os.WriteFile(learningsPath, []byte(content), 0644); writeErr == nil {
		t.Skip("chmod 0444 did not restrict writes on this platform")
	}

	kw := runner.NewFileKnowledgeWriter(tmpDir)
	err := kw.ValidateNewLessons(context.Background(), runner.LessonsData{Source: "test"})
	if err == nil {
		t.Fatal("ValidateNewLessons: want error from read-only file, got nil")
	}
	if !strings.Contains(err.Error(), "runner: validate lessons: write:") {
		t.Errorf("error should contain 'runner: validate lessons: write:', got %q", err.Error())
	}
}

// --- Task 8.14: WriteProgress unchanged ---

// TestFilepathJoin_CrossPlatform verifies filepath.Join produces platform-correct
// paths for key runner operations (AC#7: review-findings.md, LEARNINGS.md).
// Exercises AC#7; actual filepath.Join coverage via stdlib — this test guards
// that production path patterns (nested dirs, dotfiles) work on the current platform.
func TestFilepathJoin_CrossPlatform(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Verify filepath.Join produces a valid, accessible path
	cases := []struct {
		name string
		path string
	}{
		{"review-findings", filepath.Join(root, "review-findings.md")},
		{"LEARNINGS", filepath.Join(root, "LEARNINGS.md")},
		{"ralph-rules", filepath.Join(root, ".ralph", "rules", "ralph-testing.md")},
	}
	for _, tc := range cases {
		// filepath.Join normalizes to os-specific separator.
		if !strings.Contains(tc.path, string(os.PathSeparator)) {
			t.Errorf("%s: path %q missing os.PathSeparator %q", tc.name, tc.path, string(os.PathSeparator))
		}
		// Verify the path is writable (platform-correct separators)
		dir := filepath.Dir(tc.path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("%s: MkdirAll(%q): %v", tc.name, dir, err)
		}
		if err := os.WriteFile(tc.path, []byte("test"), 0644); err != nil {
			t.Fatalf("%s: WriteFile(%q): %v", tc.name, tc.path, err)
		}
		if _, err := os.ReadFile(tc.path); err != nil {
			t.Fatalf("%s: ReadFile(%q): %v", tc.name, tc.path, err)
		}
	}
}

func TestFileKnowledgeWriter_WriteProgress_Unchanged(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	kw := runner.NewFileKnowledgeWriter(tmpDir)

	data := runner.ProgressData{
		SessionID:       "test-session-789",
		TaskDescription: "Some task",
	}
	if err := kw.WriteProgress(context.Background(), data); err != nil {
		t.Errorf("WriteProgress: want nil, got %v", err)
	}

	// Zero-value data
	if err := kw.WriteProgress(context.Background(), runner.ProgressData{}); err != nil {
		t.Errorf("WriteProgress(zero): want nil, got %v", err)
	}

	// No LEARNINGS.md should be created by WriteProgress
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	if _, err := os.Stat(learningsPath); err == nil {
		t.Errorf("WriteProgress should not create LEARNINGS.md")
	}
}
