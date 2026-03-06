package runner

// coverage_internal_test.go — unit tests for unexported functions that cannot be
// tested from runner_test package. Does NOT import testutil (cycle: testutil→runner).
// Covers: readExistingRules, tagEntryInContent, entryTopicKey, extractFrontmatterGlobs,
// WriteIntentFile/DeleteIntentFile/ReadIntentFile error paths, SaveDistillState errors,
// BackupDistillationFiles error, RestoreDistillationBackups error,
// DetectProjectScope branches (NonExistentRoot, DeepDir, UnreadableDir),
// RealReview_ReadTasksError, RealReview_AllDone,
// extractGlobsForCategory (no-prefix path), WriteDistillIndex (index-file skip),
// WriteDistillOutput (pending write error), DeleteIntentFile (non-ErrNotExist),
// BackupFile_RotateError, BackupDistillationFiles rule/state errors,
// restoreFromBackup rename error, RestoreDistillationBackups learnings/state errors,
// WriteDistillOutput category/critical/misc errors, WriteDistillIndex read-continue/write errors,
// ComputeDistillMetrics NeedsFormattingFixed clamp, readExistingRules dir-skip,
// RealReview/RunOnce ScanTasks errors, RunOnce buildKnowledge error.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

// --- readExistingRules ---

func TestReadExistingRules_NoFiles(t *testing.T) {
	dir := t.TempDir()

	result := readExistingRules(dir)

	if result != "No existing rule files." {
		t.Errorf("readExistingRules(emptyDir) = %q, want %q", result, "No existing rule files.")
	}
}

func TestReadExistingRules_WithFilesSkipsIndex(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-testing.md"), []byte("testing rules content"), 0644); err != nil {
		t.Fatal(err)
	}
	// This file must be skipped by readExistingRules
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-index.md"), []byte("index content"), 0644); err != nil {
		t.Fatal(err)
	}

	result := readExistingRules(dir)

	if result == "No existing rule files." {
		t.Error("readExistingRules: want non-empty result when rule files exist")
	}
	if !strings.Contains(result, "### ralph-testing.md") {
		t.Errorf("readExistingRules: want header %q in result %q", "### ralph-testing.md", result)
	}
	if !strings.Contains(result, "testing rules content") {
		t.Errorf("readExistingRules: want content %q in result %q", "testing rules content", result)
	}
	if strings.Contains(result, "ralph-index.md") {
		t.Error("readExistingRules: ralph-index.md must be skipped")
	}
	if strings.Contains(result, "index content") {
		t.Error("readExistingRules: index content must not appear in output")
	}
}

// --- tagEntryInContent ---

func TestTagEntryInContent_AlreadyTagged(t *testing.T) {
	// If header line already contains needsFormattingTag, must not double-tag
	taggedHeader := "## testing: my-entry [r, f:1] [freq:3] [stage:review] " + needsFormattingTag
	lines := []string{taggedHeader, "Entry body content."}
	entry := parsedEntry{rawHeader: taggedHeader, startLine: 0}

	tagEntryInContent(&lines, entry, []string{"G1:format"})

	tagCount := strings.Count(lines[0], needsFormattingTag)
	if tagCount != 1 {
		t.Errorf("tagEntryInContent double-tag: tag count = %d, want 1", tagCount)
	}
}

func TestTagEntryInContent_OutOfBounds(t *testing.T) {
	// startLine >= len(lines) → no-op, no panic
	lines := []string{"line 0"}
	entry := parsedEntry{rawHeader: "## testing: entry", startLine: 5}

	tagEntryInContent(&lines, entry, []string{"G1:format"})

	if lines[0] != "line 0" {
		t.Errorf("tagEntryInContent out-of-bounds: lines[0] = %q, want %q", lines[0], "line 0")
	}
}

// --- entryTopicKey fallback branch ---

func TestEntryTopicKey_NoMatch(t *testing.T) {
	// When content doesn't match entryHeaderRegex, fallback to firstLine
	content := "plain content without header format"
	result := entryTopicKey(content)
	if result != content {
		t.Errorf("entryTopicKey(noMatch) = %q, want %q", result, content)
	}
}

// --- extractFrontmatterGlobs branches ---

func TestExtractFrontmatterGlobs_NoFrontmatter(t *testing.T) {
	content := "# No frontmatter\nJust content here.\n"
	result := extractFrontmatterGlobs(content)
	if result != "-" {
		t.Errorf("extractFrontmatterGlobs(noFrontmatter) = %q, want %q", result, "-")
	}
}

func TestExtractFrontmatterGlobs_NoEndDelimiter(t *testing.T) {
	// Has "---\n" prefix but no closing "\n---\n"
	content := "---\nglobs: *.go\nno closing delimiter\n"
	result := extractFrontmatterGlobs(content)
	if result != "-" {
		t.Errorf("extractFrontmatterGlobs(noEnd) = %q, want %q", result, "-")
	}
}

func TestExtractFrontmatterGlobs_NoGlobsField(t *testing.T) {
	// Valid frontmatter but no "globs:" line
	content := "---\ntitle: my file\nauthor: test\n---\ncontent"
	result := extractFrontmatterGlobs(content)
	if result != "-" {
		t.Errorf("extractFrontmatterGlobs(noGlobs) = %q, want %q", result, "-")
	}
}

func TestExtractFrontmatterGlobs_WithGlobs(t *testing.T) {
	// Valid frontmatter with globs field
	content := "---\ntitle: my file\nglobs: *.go\n---\ncontent"
	result := extractFrontmatterGlobs(content)
	if result != "*.go" {
		t.Errorf("extractFrontmatterGlobs(withGlobs) = %q, want %q", result, "*.go")
	}
}

// --- WriteIntentFile error path ---

func TestWriteIntentFile_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a file as projectRoot so MkdirAll(projectRoot/.ralph) fails
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err := WriteIntentFile(blocker, &DistillIntent{Phase: "write"})
	if err == nil {
		t.Fatal("WriteIntentFile: expected error when MkdirAll fails")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("WriteIntentFile error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

// --- DeleteIntentFile error path ---

func TestDeleteIntentFile_RemoveError(t *testing.T) {
	tmpDir := t.TempDir()
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create intentFileName as a directory so os.Remove (file-only) fails on WSL/Linux
	intentDir := filepath.Join(ralphDir, intentFileName)
	if err := os.MkdirAll(intentDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := DeleteIntentFile(tmpDir)
	if err == nil {
		// On NTFS/WSL, os.Remove of a non-empty dir fails; empty dir may succeed
		// Recreate to ensure it's non-empty
		_ = os.WriteFile(filepath.Join(intentDir, "dummy"), []byte("x"), 0644)
		err = DeleteIntentFile(tmpDir)
	}
	if err == nil {
		t.Skip("filesystem allows Remove on directory — error path not reproducible")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("DeleteIntentFile error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

// --- ReadIntentFile JSON error path ---

func TestReadIntentFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(ralphDir, intentFileName)
	if err := os.WriteFile(intentPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadIntentFile(tmpDir)
	if err == nil {
		t.Fatal("ReadIntentFile: expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("ReadIntentFile error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

// --- SaveDistillState error path ---

func TestSaveDistillState_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	// path = blocker/subdir/distill-state.json → MkdirAll(blocker/subdir) fails (blocker is a file)
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(blocker, "subdir", "distill-state.json")

	err := SaveDistillState(badPath, &DistillState{Version: 1})
	if err == nil {
		t.Fatal("SaveDistillState: expected error when MkdirAll fails")
	}
	if !strings.Contains(err.Error(), "runner: distill state: save:") {
		t.Errorf("SaveDistillState error = %q, want containing %q", err.Error(), "runner: distill state: save:")
	}
}

// --- BackupDistillationFiles error path ---

func TestBackupDistillationFiles_LearningsRenameError(t *testing.T) {
	tmpDir := t.TempDir()
	// Write LEARNINGS.md
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Block rename: create LEARNINGS.md.bak as a non-empty directory
	bakDir := filepath.Join(tmpDir, "LEARNINGS.md.bak")
	if err := os.MkdirAll(bakDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bakDir, "dummy"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err := BackupDistillationFiles(tmpDir)
	if err == nil {
		t.Skip("filesystem allows rename over non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: distill: backup:") {
		t.Errorf("BackupDistillationFiles error = %q, want containing %q", err.Error(), "runner: distill: backup:")
	}
}

// --- RestoreDistillationBackups error path ---

func TestRestoreDistillationBackups_RulesRenameError(t *testing.T) {
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create ralph-testing.md.bak
	bakFile := filepath.Join(rulesDir, "ralph-testing.md.bak")
	if err := os.WriteFile(bakFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make target a non-empty directory so rename fails
	targetDir := filepath.Join(rulesDir, "ralph-testing.md")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "dummy"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err := RestoreDistillationBackups(tmpDir)
	if err == nil {
		t.Skip("filesystem allows rename over non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: distill: restore:") {
		t.Errorf("RestoreDistillationBackups error = %q, want containing %q", err.Error(), "runner: distill: restore:")
	}
}

// --- RealReview error paths (no session.Execute needed) ---

func TestRealReview_ReadTasksError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   tmpDir,
		MaxTurns:      5,
	}
	rc := RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: filepath.Join(tmpDir, "nonexistent-tasks.md"),
	}

	_, err := RealReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RealReview: expected error for missing tasks file")
	}
	if !strings.Contains(err.Error(), "runner: review: read tasks:") {
		t.Errorf("RealReview error = %q, want containing %q", err.Error(), "runner: review: read tasks:")
	}
}

// --- DetectProjectScope branches ---

func TestDetectProjectScope_NonExistentRoot(t *testing.T) {
	// Non-existent root: WalkDir callback receives err != nil → returns nil (skip unreadable).
	// WalkDir itself returns nil since callback never propagates errors.
	// Result: no error, no extensions found → "No language-specific patterns detected".
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	result, err := DetectProjectScope(nonExistent)
	if err != nil {
		t.Errorf("DetectProjectScope: unexpected error for non-existent root: %v", err)
	}
	if result != "No language-specific patterns detected" {
		t.Errorf("DetectProjectScope result = %q, want %q", result, "No language-specific patterns detected")
	}
}

func TestDetectProjectScope_DeepDir(t *testing.T) {
	// 3-level dir: WalkDir callback receives dir at depth=2 → SkipDir branch executed.
	// SkipDir is not an error for WalkDir, so result is nil error.
	root := t.TempDir()
	deepDir := filepath.Join(root, "level1", "level2", "level3")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := DetectProjectScope(root)
	if err != nil {
		t.Errorf("DetectProjectScope: unexpected error with deep dirs: %v", err)
	}
}

func TestDetectProjectScope_UnreadableDir(t *testing.T) {
	// Unreadable subdir: WalkDir callback receives err != nil → return nil (skip branch).
	// Requires chmod, which works on Linux/ext4 (/tmp) but not NTFS.
	root := t.TempDir()
	subDir := filepath.Join(root, "protected")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(subDir, 0000); err != nil {
		t.Skipf("chmod not supported on this filesystem: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(subDir, 0755) })

	// WalkDir triggers callback err != nil for subDir → return nil → no WalkDir error
	_, err := DetectProjectScope(root)
	if err != nil {
		t.Errorf("DetectProjectScope: unexpected error with unreadable dir: %v", err)
	}
}

// --- SaveDistillState WriteFile error path ---

func TestSaveDistillState_WriteFileError(t *testing.T) {
	// Make path an existing directory so os.WriteFile fails with EISDIR.
	// filepath.Dir(path) == tmpDir → MkdirAll succeeds, then WriteFile(dir) fails.
	dirPath := filepath.Join(t.TempDir(), "statefile-is-dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatal(err)
	}

	err := SaveDistillState(dirPath, &DistillState{Version: 1})
	if err == nil {
		t.Fatal("SaveDistillState: expected error when path is a directory")
	}
	if !strings.Contains(err.Error(), "runner: distill state: save:") {
		t.Errorf("SaveDistillState error = %q, want containing %q", err.Error(), "runner: distill state: save:")
	}
}

// --- WriteIntentFile WriteFile error path ---

func TestWriteIntentFile_WriteFileError(t *testing.T) {
	// Create intentFileName as a directory so os.WriteFile fails.
	tmpDir := t.TempDir()
	ralphDir := filepath.Join(tmpDir, ".ralph")
	intentDir := filepath.Join(ralphDir, intentFileName)
	if err := os.MkdirAll(intentDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := WriteIntentFile(tmpDir, &DistillIntent{Phase: "write"})
	if err == nil {
		t.Fatal("WriteIntentFile: expected error when intentFileName is a directory")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("WriteIntentFile error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

// --- ReadIntentFile non-NotExist ReadFile error path ---

func TestReadIntentFile_DirError(t *testing.T) {
	// Make intentFileName a directory so os.ReadFile fails with EISDIR (not ErrNotExist).
	tmpDir := t.TempDir()
	ralphDir := filepath.Join(tmpDir, ".ralph")
	intentDir := filepath.Join(ralphDir, intentFileName)
	if err := os.MkdirAll(intentDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := ReadIntentFile(tmpDir)
	if err == nil {
		t.Fatal("ReadIntentFile: expected error when intentFileName is a directory")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("ReadIntentFile error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

func TestRealReview_AllDone(t *testing.T) {
	tmpDir := t.TempDir()
	tasksFile := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("# Sprint\n\n- [x] Done one\n- [x] Done two\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   tmpDir,
		MaxTurns:      5,
	}
	rc := RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: tasksFile,
	}

	result, err := RealReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RealReview: unexpected error: %v", err)
	}
	if !result.Clean {
		t.Error("RealReview: want Clean=true when all tasks done")
	}
}

func TestExtractGlobsForCategory_NoPrefix(t *testing.T) {
	// scopeHints without "Relevant globs: " prefix → return nil branch (line 521).
	result := extractGlobsForCategory("testing", "No language-specific patterns detected")
	if result != nil {
		t.Errorf("extractGlobsForCategory: want nil for no-prefix scopeHints, got %v", result)
	}
}

func TestWriteDistillIndex_SkipsIndexFile(t *testing.T) {
	// When ralph-index.md exists alongside ralph-<cat>.md files, the index file
	// itself must be skipped (continue branch at line 540 in WriteDistillIndex).
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a real category file and the index file itself.
	catContent := "---\nglobs: [\"**/*.go\"]\n---\n# Category\n\n## Rule one\n\ncontent\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-testing.md"), []byte(catContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-index.md"), []byte("# Old Index\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := WriteDistillIndex(tmpDir); err != nil {
		t.Fatalf("WriteDistillIndex: unexpected error: %v", err)
	}

	// The written index must contain the category file but not reference itself.
	data, err := os.ReadFile(filepath.Join(rulesDir, "ralph-index.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ralph-testing.md") {
		t.Errorf("index: want to contain %q, got:\n%s", "ralph-testing.md", content)
	}
	// ralph-index.md should not appear as a table row (only as the output file).
	rows := strings.Count(content, "| ralph-index.md |")
	if rows != 0 {
		t.Errorf("index: want 0 rows for ralph-index.md, got %d", rows)
	}
}

func TestWriteDistillOutput_PendingWriteError(t *testing.T) {
	// LEARNINGS.md.pending exists as a directory → WriteFile fails with EISDIR.
	// Covers the pending-write error path at line 344-346.
	tmpDir := t.TempDir()

	// Pre-create LEARNINGS.md.pending as a directory to cause EISDIR on WriteFile.
	if err := os.MkdirAll(filepath.Join(tmpDir, "LEARNINGS.md.pending"), 0755); err != nil {
		t.Fatal(err)
	}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories:          map[string][]DistilledEntry{},
	}
	state := &DistillState{Version: 1}

	_, err := WriteDistillOutput(tmpDir, output, state, "")
	if err == nil {
		t.Fatal("WriteDistillOutput: expected error when LEARNINGS.md.pending is a dir")
	}
	if !strings.Contains(err.Error(), "runner: distill: write:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: write:")
	}
}

func TestDeleteIntentFile_NonExistError(t *testing.T) {
	// distill-intent.json is a directory → os.Remove returns EISDIR (not ErrNotExist)
	// → DeleteIntentFile returns "runner: distill: intent:" error (line 636).
	tmpDir := t.TempDir()
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create intent file path as a non-empty directory so os.Remove fails.
	intentDir := filepath.Join(ralphDir, intentFileName)
	if err := os.MkdirAll(filepath.Join(intentDir, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := DeleteIntentFile(tmpDir)
	if err == nil {
		t.Fatal("DeleteIntentFile: expected error when intent path is a non-empty directory")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

// --- BackupFile rotate error ---

func TestBackupFile_RotateError(t *testing.T) {
	// LEARNINGS.md.bak exists as a regular file; LEARNINGS.md.bak.1 is a non-empty dir.
	// os.Rename(regular_file, non_empty_dir) returns EISDIR on Linux → line 146 hit.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "LEARNINGS.md")
	bakPath := path + ".bak"
	bak1Path := path + ".bak.1"

	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bakPath, []byte("old backup"), 0644); err != nil {
		t.Fatal(err)
	}
	// bak.1 as non-empty dir: rename(bak_file, bak1_dir) → EISDIR on Linux
	if err := os.MkdirAll(filepath.Join(bak1Path, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := BackupFile(path)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: backup:") {
		t.Errorf("BackupFile rotate error = %q, want containing %q", err.Error(), "runner: backup:")
	}
}

// --- BackupDistillationFiles rule file error ---

func TestBackupDistillationFiles_RuleFileError(t *testing.T) {
	// LEARNINGS.md has no .bak conflict so it backs up OK.
	// ralph-test.md.bak is a regular file; .bak.1 is a non-empty dir → rotate EISDIR → line 170 hit.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	ruleFile := filepath.Join(rulesDir, "ralph-test.md")
	if err := os.WriteFile(ruleFile, []byte("rules"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruleFile+".bak", []byte("old bak"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ruleFile+".bak.1", "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := BackupDistillationFiles(tmpDir)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: distill: backup:") {
		t.Errorf("BackupDistillationFiles rule error = %q, want containing %q", err.Error(), "runner: distill: backup:")
	}
}

// --- BackupDistillationFiles state file error ---

func TestBackupDistillationFiles_StateFileError(t *testing.T) {
	// LEARNINGS.md and ralph files have no .bak conflicts so they back up OK.
	// distill-state.json.bak is a regular file; .bak.1 is a non-empty dir → rotate EISDIR → line 176 hit.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(ralphDir, "distill-state.json")
	if err := os.WriteFile(statePath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath+".bak", []byte("old bak"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(statePath+".bak.1", "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := BackupDistillationFiles(tmpDir)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: distill: backup:") {
		t.Errorf("BackupDistillationFiles state error = %q, want containing %q", err.Error(), "runner: distill: backup:")
	}
}

// --- restoreFromBackup rename error ---

func TestRestoreFromBackup_RenameError(t *testing.T) {
	// bakPath exists as a regular file; origPath exists as a non-empty directory.
	// os.Rename(file, non_empty_dir) returns EISDIR on Linux → line 220-221 hit.
	tmpDir := t.TempDir()
	origPath := filepath.Join(tmpDir, "LEARNINGS.md")
	bakPath := origPath + ".bak"

	if err := os.WriteFile(bakPath, []byte("backup content"), 0644); err != nil {
		t.Fatal(err)
	}
	// origPath as non-empty dir so rename fails
	if err := os.MkdirAll(filepath.Join(origPath, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := restoreFromBackup(origPath)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — not reproducible on this FS")
	}
	// restoreFromBackup returns the raw OS error (no wrapping at line 221)
	if err.Error() == "" {
		t.Error("restoreFromBackup: expected non-empty error message")
	}
}

// --- RestoreDistillationBackups learnings restore error ---

func TestRestoreDistillationBackups_LearningsRestoreError(t *testing.T) {
	// LEARNINGS.md.bak exists as regular file; LEARNINGS.md is a non-empty directory.
	// restoreFromBackup(learningsPath) returns EISDIR → line 192 hit.
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	if err := os.WriteFile(learningsPath+".bak", []byte("backup content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(learningsPath, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := RestoreDistillationBackups(tmpDir)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: distill: restore:") {
		t.Errorf("RestoreDistillationBackups learnings error = %q, want containing %q", err.Error(), "runner: distill: restore:")
	}
}

// --- RestoreDistillationBackups state restore error ---

func TestRestoreDistillationBackups_StateRestoreError(t *testing.T) {
	// LEARNINGS.md.bak missing → skipped. distill-state.json.bak is a regular file;
	// distill-state.json is a non-empty directory → restoreFromBackup fails → line 207 hit.
	tmpDir := t.TempDir()
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(ralphDir, "distill-state.json")
	if err := os.WriteFile(statePath+".bak", []byte("backup state"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(statePath, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := RestoreDistillationBackups(tmpDir)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — not reproducible on this FS")
	}
	if !strings.Contains(err.Error(), "runner: distill: restore:") {
		t.Errorf("RestoreDistillationBackups state error = %q, want containing %q", err.Error(), "runner: distill: restore:")
	}
}

// --- WriteDistillOutput category write error ---

func TestWriteDistillOutput_CategoryWriteError(t *testing.T) {
	// Category "testing" has >=5 entries → writeCategoryFilePending is called.
	// ralph-testing.md.pending pre-created as non-empty dir → WriteFile EISDIR → lines 385,463 hit.
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	catPending := filepath.Join(rulesDir, "ralph-testing.md.pending")
	if err := os.MkdirAll(filepath.Join(catPending, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "## testing: rule1\nBody 1", Freq: 1},
				{Content: "## testing: rule2\nBody 2", Freq: 2},
				{Content: "## testing: rule3\nBody 3", Freq: 3},
				{Content: "## testing: rule4\nBody 4", Freq: 4},
				{Content: "## testing: rule5\nBody 5", Freq: 5},
			},
		},
	}
	state := &DistillState{Version: 1}

	_, err := WriteDistillOutput(tmpDir, output, state, "")
	if err == nil {
		t.Fatal("WriteDistillOutput: expected category write error")
	}
	if !strings.Contains(err.Error(), "runner: distill: write:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: write:")
	}
}

// --- WriteDistillOutput critical write error ---

func TestWriteDistillOutput_CriticalWriteError(t *testing.T) {
	// Entry with freq >= 10 → criticalEntries non-empty → writeCriticalFilePending called.
	// ralph-critical.md.pending pre-created as non-empty dir → WriteFile EISDIR → lines 398,480 hit.
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	critPending := filepath.Join(rulesDir, "ralph-critical.md.pending")
	if err := os.MkdirAll(filepath.Join(critPending, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "## testing: high-freq [freq:10] ANCHOR\nBody", Freq: 10, IsAnchor: true},
			},
		},
	}
	state := &DistillState{Version: 1}

	_, err := WriteDistillOutput(tmpDir, output, state, "")
	if err == nil {
		t.Fatal("WriteDistillOutput: expected critical write error")
	}
	if !strings.Contains(err.Error(), "runner: distill: write:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: write:")
	}
}

// --- WriteDistillOutput misc write error ---

func TestWriteDistillOutput_MiscWriteError(t *testing.T) {
	// Category "testing" has only 1 entry (< 5) → goes to miscEntries → writeMiscFilePending called.
	// ralph-misc.md.pending pre-created as non-empty dir → WriteFile EISDIR → lines 407,499 hit.
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	miscPending := filepath.Join(rulesDir, "ralph-misc.md.pending")
	if err := os.MkdirAll(filepath.Join(miscPending, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	output := &DistillOutput{
		CompressedLearnings: "# Compressed",
		Categories: map[string][]DistilledEntry{
			"testing": {
				{Content: "## testing: rule1\nBody 1", Freq: 1},
			},
		},
	}
	state := &DistillState{Version: 1}

	_, err := WriteDistillOutput(tmpDir, output, state, "")
	if err == nil {
		t.Fatal("WriteDistillOutput: expected misc write error")
	}
	if !strings.Contains(err.Error(), "runner: distill: write:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: write:")
	}
}

// --- WriteDistillIndex ReadFile continue ---

func TestWriteDistillIndex_ReadFileContinue(t *testing.T) {
	// ralph-foo.md exists as a non-empty directory → os.ReadFile(dir) returns EISDIR
	// → the continue branch at line 544-545 is hit (file skipped, no error returned).
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	fooDir := filepath.Join(rulesDir, "ralph-foo.md")
	if err := os.MkdirAll(filepath.Join(fooDir, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	// Should not error — ReadFile failure on dir → continue (skipped)
	if err := WriteDistillIndex(tmpDir); err != nil {
		t.Fatalf("WriteDistillIndex: unexpected error: %v", err)
	}

	indexPath := filepath.Join(rulesDir, "ralph-index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile index: %v", err)
	}
	if strings.Contains(string(data), "ralph-foo.md") {
		t.Error("WriteDistillIndex: ralph-foo.md (dir) should be skipped from index")
	}
}

// --- WriteDistillIndex WriteFile error ---

func TestWriteDistillIndex_WriteFileError(t *testing.T) {
	// ralph-index.md pre-created as a non-empty directory → WriteFile fails with EISDIR → line 561 hit.
	tmpDir := t.TempDir()
	rulesDir := filepath.Join(tmpDir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	indexDir := filepath.Join(rulesDir, "ralph-index.md")
	if err := os.MkdirAll(filepath.Join(indexDir, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	err := WriteDistillIndex(tmpDir)
	if err == nil {
		t.Fatal("WriteDistillIndex: expected error when ralph-index.md is a directory")
	}
	if !strings.Contains(err.Error(), "runner: distill: index:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: index:")
	}
}

// --- ComputeDistillMetrics NeedsFormattingFixed < 0 clamp ---

func TestComputeDistillMetrics_NeedsFormattingFixed(t *testing.T) {
	// output has more [needs-formatting] tags than oldContent → needsFormattingFixed < 0
	// → clamped to 0 at line 764.
	oldContent := "# Compressed LEARNINGS\n\nNo formatting issues.\n"
	output := &DistillOutput{
		CompressedLearnings: "## rule: example " + needsFormattingTag + "\nBody\n",
		Categories:          map[string][]DistilledEntry{},
	}

	metrics := ComputeDistillMetrics(oldContent, output)
	if metrics.NeedsFormattingFixed != 0 {
		t.Errorf("NeedsFormattingFixed = %d, want 0 (clamped from negative)", metrics.NeedsFormattingFixed)
	}
}

// --- readExistingRules dir entry skip ---

func TestReadExistingRules_DirEntrySkip(t *testing.T) {
	// ralph-foo.md exists as a non-empty directory → os.ReadFile returns EISDIR
	// → the continue branch at line 913 is hit; other files still included.
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".ralph", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(rulesDir, "ralph-good.md"), []byte("good rules"), 0644); err != nil {
		t.Fatal(err)
	}
	fooDir := filepath.Join(rulesDir, "ralph-foo.md")
	if err := os.MkdirAll(filepath.Join(fooDir, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	result := readExistingRules(dir)
	if !strings.Contains(result, "ralph-good.md") {
		t.Errorf("readExistingRules: want %q in result, got: %s", "ralph-good.md", result)
	}
	if strings.Contains(result, "ralph-foo.md") {
		t.Error("readExistingRules: ralph-foo.md (dir) should be skipped")
	}
}

// --- RealReview ScanTasks error ---

func TestRealReview_ScanTasksError(t *testing.T) {
	// Tasks file exists but has no open or done task markers → ScanTasks returns ErrNoTasks.
	// Line 117-118: if scanErr != nil { return ReviewResult{}, scanErr }
	tmpDir := t.TempDir()
	tasksFile := filepath.Join(tmpDir, "tasks.md")
	if err := os.WriteFile(tasksFile, []byte("# Sprint\n\nNo task markers here.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   tmpDir,
		MaxTurns:      5,
	}
	rc := RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: tasksFile,
	}

	_, err := RealReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RealReview: expected error for tasks file with no task lines")
	}
	if !strings.Contains(err.Error(), "runner: scan tasks:") {
		t.Errorf("RealReview error = %q, want containing %q", err.Error(), "runner: scan tasks:")
	}
}

// nopGitClient is a minimal GitClient stub that returns success for all operations.
// Used by tests that require Git.HealthCheck to pass but do not exercise git behaviour.
type nopGitClient struct{}

func (nopGitClient) HealthCheck(_ context.Context) error          { return nil }
func (nopGitClient) HeadCommit(_ context.Context) (string, error) { return "abc123", nil }
func (nopGitClient) RestoreClean(_ context.Context) error         { return nil }
func (nopGitClient) DiffStats(_ context.Context, _, _ string) (*DiffStats, error) {
	return &DiffStats{}, nil
}

// --- RunOnce ScanTasks error ---

func TestRunOnce_ScanTasksError(t *testing.T) {
	// Tasks file exists but has no task markers → ScanTasks returns ErrNoTasks.
	// Line 891-892: if scanErr != nil { return scanErr }
	tmpDir := t.TempDir()
	tasksFile := filepath.Join(tmpDir, "tasks.md")
	if err := os.WriteFile(tasksFile, []byte("# Sprint\n\nJust a header.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   tmpDir,
		MaxTurns:      5,
	}
	rc := RunConfig{
		Cfg:       cfg,
		Git:       nopGitClient{},
		TasksFile: tasksFile,
	}

	err := RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error for tasks file with no task lines")
	}
	if !strings.Contains(err.Error(), "runner: scan tasks:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: scan tasks:")
	}
}

// --- RunOnce buildKnowledge error ---

func TestRunOnce_BuildKnowledgeError(t *testing.T) {
	// Tasks file has open tasks; HealthCheck passes; LEARNINGS.md is a non-empty directory
	// → buildKnowledgeReplacements returns EISDIR → RunOnce wraps "runner: build knowledge:".
	// Line 905-906: if onceKnowledgeErr != nil { return fmt.Errorf("runner: build knowledge: %w", ...) }
	tmpDir := t.TempDir()
	tasksFile := filepath.Join(tmpDir, "tasks.md")
	if err := os.WriteFile(tasksFile, []byte("# Sprint\n\n- [ ] Open task\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// LEARNINGS.md as non-empty dir → ReadFile returns EISDIR (not ErrNotExist)
	if err := os.MkdirAll(filepath.Join(tmpDir, "LEARNINGS.md", "child"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   tmpDir,
		MaxTurns:      5,
	}
	rc := RunConfig{
		Cfg:       cfg,
		Git:       nopGitClient{},
		TasksFile: tasksFile,
	}

	err := RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error when buildKnowledgeReplacements fails")
	}
	if !strings.Contains(err.Error(), "runner: build knowledge:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: build knowledge:")
	}
	if !strings.Contains(err.Error(), "runner: build knowledge: read learnings:") {
		t.Errorf("RunOnce inner error = %q, want containing %q", err.Error(), "runner: build knowledge: read learnings:")
	}
}

// --- DESIGN-4: selectReviewModel tests ---

func TestSelectReviewModel_Scenarios(t *testing.T) {
	baseCfg := func() *config.Config {
		return &config.Config{
			ModelReview:         "claude-sonnet-4-6",
			ModelReviewLight:    "claude-haiku-4-5-20251001",
			ReviewLightMaxFiles: 5,
			ReviewLightMaxLines: 50,
		}
	}

	cases := []struct {
		name          string
		cfg           *config.Config
		ds            *DiffStats
		isGate        bool
		hydraDetected bool
		highEffort    bool
		wantModel     string
	}{
		{
			name:      "nil diff stats falls back to standard",
			cfg:       baseCfg(),
			ds:        nil,
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "small diff uses light model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 3, Insertions: 20, Deletions: 10},
			wantModel: "claude-haiku-4-5-20251001",
		},
		{
			name:      "exact threshold uses light model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 5, Insertions: 30, Deletions: 20},
			wantModel: "claude-haiku-4-5-20251001",
		},
		{
			name:      "too many files uses standard model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 6, Insertions: 10, Deletions: 5},
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "too many lines uses standard model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 2, Insertions: 40, Deletions: 15},
			wantModel: "claude-sonnet-4-6",
		},
		{
			name: "empty light model falls back to standard",
			cfg: &config.Config{
				ModelReview:         "claude-sonnet-4-6",
				ModelReviewLight:    "",
				ReviewLightMaxFiles: 5,
				ReviewLightMaxLines: 50,
			},
			ds:        &DiffStats{FilesChanged: 1, Insertions: 5, Deletions: 2},
			wantModel: "claude-sonnet-4-6",
		},
		// Backfill from Story 9.2: isGate/hydraDetected coverage
		{
			name:      "gate task with small diff forces standard model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 1, Insertions: 5, Deletions: 2},
			isGate:    true,
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "gate task with large diff uses standard model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 10, Insertions: 200, Deletions: 100},
			isGate:    true,
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "gate task with nil diff stats uses standard model",
			cfg:       baseCfg(),
			ds:        nil,
			isGate:    true,
			wantModel: "claude-sonnet-4-6",
		},
		{
			name:      "non-gate task with small diff uses light model",
			cfg:       baseCfg(),
			ds:        &DiffStats{FilesChanged: 2, Insertions: 10, Deletions: 5},
			isGate:    false,
			wantModel: "claude-haiku-4-5-20251001",
		},
		// DESIGN-4: hydra escalation cases
		{
			name:          "hydra with small diff forces standard model",
			cfg:           baseCfg(),
			ds:            &DiffStats{FilesChanged: 1, Insertions: 5, Deletions: 2},
			hydraDetected: true,
			wantModel:     "claude-sonnet-4-6",
		},
		{
			name:          "hydra with large diff uses standard model",
			cfg:           baseCfg(),
			ds:            &DiffStats{FilesChanged: 10, Insertions: 200, Deletions: 100},
			hydraDetected: true,
			wantModel:     "claude-sonnet-4-6",
		},
		{
			name:          "hydra with nil diff stats uses standard model",
			cfg:           baseCfg(),
			ds:            nil,
			hydraDetected: true,
			wantModel:     "claude-sonnet-4-6",
		},
		{
			name:          "hydra and gate both true uses standard model",
			cfg:           baseCfg(),
			ds:            &DiffStats{FilesChanged: 1, Insertions: 5, Deletions: 2},
			isGate:        true,
			hydraDetected: true,
			wantModel:     "claude-sonnet-4-6",
		},
		{
			name:          "no hydra no gate small diff uses light model",
			cfg:           baseCfg(),
			ds:            &DiffStats{FilesChanged: 2, Insertions: 10, Deletions: 5},
			hydraDetected: false,
			wantModel:     "claude-haiku-4-5-20251001",
		},
		// Story 9.3 AC#6: highEffort escalation cases
		{
			name:       "highEffort with small diff forces standard model",
			cfg:        baseCfg(),
			ds:         &DiffStats{FilesChanged: 1, Insertions: 5, Deletions: 2},
			highEffort: true,
			wantModel:  "claude-sonnet-4-6",
		},
		{
			name:       "highEffort false with small diff uses light model",
			cfg:        baseCfg(),
			ds:         &DiffStats{FilesChanged: 2, Insertions: 10, Deletions: 5},
			highEffort: false,
			wantModel:  "claude-haiku-4-5-20251001",
		},
		{
			name:          "highEffort and hydra and gate all true uses standard",
			cfg:           baseCfg(),
			ds:            &DiffStats{FilesChanged: 1, Insertions: 5, Deletions: 2},
			isGate:        true,
			hydraDetected: true,
			highEffort:    true,
			wantModel:     "claude-sonnet-4-6",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectReviewModel(tc.cfg, tc.ds, tc.isGate, tc.hydraDetected, tc.highEffort)
			if got != tc.wantModel {
				t.Errorf("selectReviewModel() = %q, want %q", got, tc.wantModel)
			}
		})
	}
}
