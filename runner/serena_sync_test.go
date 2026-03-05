package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/session"
)

// --- Story 8.3: Backup/Rollback/Validate tests ---

// setupMemories creates .serena/memories/ with N .md files containing predictable content.
func setupMemories(t *testing.T, dir string, names []string) {
	t.Helper()
	memDir := filepath.Join(dir, serenaMemoriesDir)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("setup memories dir: %v", err)
	}
	for _, name := range names {
		content := "content of " + name
		if err := os.WriteFile(filepath.Join(memDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("setup memory file %s: %v", name, err)
		}
	}
}

func TestBackupMemories_CopiesFiles(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.md", "b.md", "c.md", "d.md", "e.md"}
	setupMemories(t, dir, files)

	if err := backupMemories(dir); err != nil {
		t.Fatalf("backupMemories: unexpected error: %v", err)
	}

	// Verify all files copied with correct content
	for _, name := range files {
		bakPath := filepath.Join(dir, serenaBackupDir, name)
		data, err := os.ReadFile(bakPath)
		if err != nil {
			t.Errorf("backup file %s: %v", name, err)
			continue
		}
		want := "content of " + name
		if string(data) != want {
			t.Errorf("backup %s content = %q, want %q", name, string(data), want)
		}
	}
}

func TestBackupMemories_RemovesPreviousBackup(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md"})

	// Create old backup with stale file
	oldBak := filepath.Join(dir, serenaBackupDir)
	if err := os.MkdirAll(oldBak, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldBak, "stale.md"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := backupMemories(dir); err != nil {
		t.Fatalf("backupMemories: unexpected error: %v", err)
	}

	// Stale file should be gone
	if _, err := os.Stat(filepath.Join(oldBak, "stale.md")); !os.IsNotExist(err) {
		t.Error("previous backup stale.md should have been removed")
	}
	// New backup should have a.md
	if _, err := os.Stat(filepath.Join(oldBak, "a.md")); err != nil {
		t.Errorf("new backup should have a.md: %v", err)
	}
}

func TestBackupMemories_MissingSource(t *testing.T) {
	dir := t.TempDir()
	// No .serena/memories/ created

	err := backupMemories(dir)
	if err == nil {
		t.Fatal("backupMemories: expected error for missing source, got nil")
	}
	if !strings.Contains(err.Error(), "runner: serena sync: backup:") {
		t.Errorf("error = %q, want containing prefix %q", err.Error(), "runner: serena sync: backup:")
	}
	wantPath := filepath.Join(".serena", "memories")
	if !strings.Contains(err.Error(), wantPath) {
		t.Errorf("error = %q, want containing path %q", err.Error(), wantPath)
	}

	// Verify .bak/ not created
	bakDir := filepath.Join(dir, serenaBackupDir)
	if _, statErr := os.Stat(bakDir); !os.IsNotExist(statErr) {
		t.Error(".bak/ should not be created when source is missing")
	}
}

func TestRollbackMemories_RestoresContent(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.md", "b.md"}
	setupMemories(t, dir, files)

	// Create backup
	if err := backupMemories(dir); err != nil {
		t.Fatalf("backupMemories: %v", err)
	}

	// Modify original memories (simulate sync corruption)
	memDir := filepath.Join(dir, serenaMemoriesDir)
	if err := os.WriteFile(filepath.Join(memDir, "a.md"), []byte("corrupted"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "extra.md"), []byte("extra"), 0644); err != nil {
		t.Fatal(err)
	}

	// Rollback
	if err := rollbackMemories(dir); err != nil {
		t.Fatalf("rollbackMemories: unexpected error: %v", err)
	}

	// Verify original content restored
	data, err := os.ReadFile(filepath.Join(memDir, "a.md"))
	if err != nil {
		t.Fatalf("read a.md after rollback: %v", err)
	}
	if string(data) != "content of a.md" {
		t.Errorf("a.md content = %q, want %q", string(data), "content of a.md")
	}

	// Extra file should be gone (entire dir replaced)
	if _, err := os.Stat(filepath.Join(memDir, "extra.md")); !os.IsNotExist(err) {
		t.Error("extra.md should be removed after rollback")
	}
}

func TestRollbackMemories_PreservesBackup(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md"})

	if err := backupMemories(dir); err != nil {
		t.Fatalf("backupMemories: %v", err)
	}
	if err := rollbackMemories(dir); err != nil {
		t.Fatalf("rollbackMemories: %v", err)
	}

	// .bak/ should still exist
	bakPath := filepath.Join(dir, serenaBackupDir, "a.md")
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf(".bak/ should be preserved after rollback: %v", err)
	}
}

func TestRollbackMemories_MissingBackup(t *testing.T) {
	dir := t.TempDir()
	// Create memories dir but no .bak/
	setupMemories(t, dir, []string{"a.md"})

	err := rollbackMemories(dir)
	if err == nil {
		t.Fatal("rollbackMemories: expected error for missing backup, got nil")
	}
	if !strings.Contains(err.Error(), "runner: serena sync: rollback:") {
		t.Errorf("error = %q, want containing prefix %q", err.Error(), "runner: serena sync: rollback:")
	}
}

func TestCleanupBackup_RemovesBackup(t *testing.T) {
	dir := t.TempDir()
	bakDir := filepath.Join(dir, serenaBackupDir)
	if err := os.MkdirAll(bakDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bakDir, "a.md"), []byte("bak"), 0644); err != nil {
		t.Fatal(err)
	}

	cleanupBackup(dir)

	if _, err := os.Stat(bakDir); !os.IsNotExist(err) {
		t.Error(".bak/ should be removed after cleanup")
	}
}

func TestCleanupBackup_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	// No .bak/ exists — should not panic
	cleanupBackup(dir)
}

func TestCountMemoryFiles_OnlyMdFiles(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, serenaMemoriesDir)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .md files, .txt file, and subdirectory
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if err := os.WriteFile(filepath.Join(memDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(memDir, "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(memDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	count, err := countMemoryFiles(dir)
	if err != nil {
		t.Fatalf("countMemoryFiles: %v", err)
	}
	if count != 3 {
		t.Errorf("countMemoryFiles = %d, want 3 (only .md files)", count)
	}
}

func TestCountMemoryFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, serenaMemoriesDir)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	count, err := countMemoryFiles(dir)
	if err != nil {
		t.Fatalf("countMemoryFiles: %v", err)
	}
	if count != 0 {
		t.Errorf("countMemoryFiles = %d, want 0 (empty dir)", count)
	}
}

func TestValidateMemories_CountScenarios(t *testing.T) {
	tests := []struct {
		name        string
		filesBefore int
		filesAfter  int
		wantErr     bool
		errContains string
	}{
		{"count equal", 5, 5, false, ""},
		{"count increased", 3, 5, false, ""},
		{"count decreased", 5, 4, true, "memory count decreased: 5 → 4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			memDir := filepath.Join(dir, serenaMemoriesDir)
			if err := os.MkdirAll(memDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Create "after" files
			for i := 0; i < tt.filesAfter; i++ {
				name := filepath.Join(memDir, strings.Repeat("a", i+1)+".md")
				if err := os.WriteFile(name, []byte("x"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			err := validateMemories(dir, tt.filesBefore)
			if tt.wantErr {
				if err == nil {
					t.Fatal("validateMemories: expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
				}
				if !strings.Contains(err.Error(), "runner: serena sync:") {
					t.Errorf("error = %q, want containing prefix %q", err.Error(), "runner: serena sync:")
				}
			} else {
				if err != nil {
					t.Errorf("validateMemories: unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateMemories_ReadError(t *testing.T) {
	dir := t.TempDir()
	// No .serena/memories/ dir — os.ReadDir will fail

	err := validateMemories(dir, 5)
	if err != nil {
		t.Errorf("validateMemories on read error should return nil (best effort), got: %v", err)
	}
}

func TestCopyDir_RecursiveWithSubdirs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	// Create source with subdirectory
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "top.md"), []byte("top"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "nested.md"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	// Verify top-level file
	data, err := os.ReadFile(filepath.Join(dst, "top.md"))
	if err != nil {
		t.Fatalf("read top.md: %v", err)
	}
	if string(data) != "top" {
		t.Errorf("top.md = %q, want %q", string(data), "top")
	}

	// Verify nested file
	data, err = os.ReadFile(filepath.Join(dst, "sub", "nested.md"))
	if err != nil {
		t.Fatalf("read sub/nested.md: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("sub/nested.md = %q, want %q", string(data), "nested")
	}
}

// --- Story 8.4: runSerenaSync / buildSyncOpts / extractCompletedTasks tests ---

// syncTestGitClient is a minimal GitClient mock for internal runner tests (avoids import cycle with testutil).
type syncTestGitClient struct {
	headCommits      []string
	headCommitIdx    int
	diffStatsResult  *DiffStats
	diffStatsErr     error
}

func (g *syncTestGitClient) HealthCheck(_ context.Context) error { return nil }
func (g *syncTestGitClient) HeadCommit(_ context.Context) (string, error) {
	if g.headCommitIdx < len(g.headCommits) {
		sha := g.headCommits[g.headCommitIdx]
		g.headCommitIdx++
		return sha, nil
	}
	return "", nil
}
func (g *syncTestGitClient) RestoreClean(_ context.Context) error { return nil }
func (g *syncTestGitClient) DiffStats(_ context.Context, _, _ string) (*DiffStats, error) {
	if g.diffStatsErr != nil {
		return nil, g.diffStatsErr
	}
	if g.diffStatsResult != nil {
		return g.diffStatsResult, nil
	}
	return &DiffStats{}, nil
}

// mockCodeIndexerSync is a test mock for CodeIndexerDetector (internal package).
type mockCodeIndexerSync struct {
	available bool
}

func (m *mockCodeIndexerSync) Available(_ string) bool { return m.available }
func (m *mockCodeIndexerSync) PromptHint() string      { return "" }

func TestRunner_runSerenaSync_HappyPath(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md", "b.md", "c.md"})

	var captured SerenaSyncOpts
	syncCalled := 0

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncMaxTurns: 5,
		},
		Git:       &syncTestGitClient{},
		TasksFile: filepath.Join(dir, "sprint-tasks.md"),
		SerenaSyncFn: func(_ context.Context, opts SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			captured = opts
			return nil, nil
		},
	}

	r.runSerenaSync(context.Background(), "", "")

	if syncCalled != 1 {
		t.Errorf("SerenaSyncFn call count = %d, want 1", syncCalled)
	}
	if captured.ProjectRoot != dir {
		t.Errorf("opts.ProjectRoot = %q, want %q", captured.ProjectRoot, dir)
	}

	// Backup should be cleaned up on success
	bakDir := filepath.Join(dir, serenaBackupDir)
	if _, err := os.Stat(bakDir); !os.IsNotExist(err) {
		t.Error(".bak/ should be removed after successful sync")
	}
}

func TestRunner_runSerenaSync_SyncError(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md", "b.md"})

	// Write known content so we can verify rollback
	memDir := filepath.Join(dir, serenaMemoriesDir)
	if err := os.WriteFile(filepath.Join(memDir, "a.md"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncMaxTurns: 5,
		},
		Git:       &syncTestGitClient{},
		TasksFile: filepath.Join(dir, "sprint-tasks.md"),
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			// Corrupt a memory file during sync
			if err := os.WriteFile(filepath.Join(memDir, "a.md"), []byte("corrupted"), 0644); err != nil {
				t.Fatal(err)
			}
			return nil, fmt.Errorf("runner: serena sync: mock failure")
		},
	}

	r.runSerenaSync(context.Background(), "", "")

	// Verify rollback happened: a.md should be restored to "original"
	data, err := os.ReadFile(filepath.Join(memDir, "a.md"))
	if err != nil {
		t.Fatalf("read a.md after rollback: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("a.md content = %q, want %q (rollback should restore)", string(data), "original")
	}
}

func TestRunner_runSerenaSync_ValidationError(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md", "b.md", "c.md"})

	memDir := filepath.Join(dir, serenaMemoriesDir)

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncMaxTurns: 5,
		},
		Git:       &syncTestGitClient{},
		TasksFile: filepath.Join(dir, "sprint-tasks.md"),
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			// Delete a memory file during sync (count decrease)
			if err := os.Remove(filepath.Join(memDir, "a.md")); err != nil {
				t.Fatal(err)
			}
			return nil, nil
		},
	}

	r.runSerenaSync(context.Background(), "", "")

	// Rollback should have restored the deleted file
	if _, err := os.Stat(filepath.Join(memDir, "a.md")); err != nil {
		t.Errorf("a.md should be restored after validation failure: %v", err)
	}
}

func TestRunner_runSerenaSync_BackupError(t *testing.T) {
	dir := t.TempDir()
	// No .serena/memories/ created → backup will fail

	syncCalled := 0
	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncMaxTurns: 5,
		},
		Git:       &syncTestGitClient{},
		TasksFile: filepath.Join(dir, "sprint-tasks.md"),
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			return nil, nil
		},
	}

	r.runSerenaSync(context.Background(), "", "")

	// Sync should not have been called (skipped due to backup error)
	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0 (backup error → skip)", syncCalled)
	}
}

func TestRunner_Execute_SerenaSyncTriggered(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md"})

	// Create tasks file
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done task\n"), 0644); err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	var captured SerenaSyncOpts
	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: true,
			SerenaSyncTrigger: "run",
			SerenaSyncMaxTurns: 5,
		},
		Git:         &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:   tasksFile,
		CodeIndexer: &mockCodeIndexerSync{available: true},
		SerenaSyncFn: func(_ context.Context, opts SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			captured = opts
			return nil, nil
		},
	}

	_, _ = r.Execute(context.Background())

	if syncCalled != 1 {
		t.Errorf("SerenaSyncFn call count = %d, want 1", syncCalled)
	}
	if captured.ProjectRoot != dir {
		t.Errorf("opts.ProjectRoot = %q, want %q", captured.ProjectRoot, dir)
	}
	if captured.MaxTurns != 5 {
		t.Errorf("opts.MaxTurns = %d, want 5", captured.MaxTurns)
	}
}

func TestRunner_Execute_SerenaSyncDisabled(t *testing.T) {
	dir := t.TempDir()
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: false,
			SerenaSyncTrigger: "run",
		},
		Git:         &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:   tasksFile,
		CodeIndexer: &mockCodeIndexerSync{available: true},
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			return nil, nil
		},
	}

	_, _ = r.Execute(context.Background())

	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0 (sync disabled)", syncCalled)
	}
}

func TestRunner_Execute_SerenaSyncUnavailable(t *testing.T) {
	dir := t.TempDir()
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: true,
			SerenaSyncTrigger: "run",
		},
		Git:         &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:   tasksFile,
		CodeIndexer: &mockCodeIndexerSync{available: false},
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			return nil, nil
		},
	}

	_, _ = r.Execute(context.Background())

	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0 (Serena unavailable)", syncCalled)
	}
}

func TestRunner_Execute_SerenaSyncNilFn(t *testing.T) {
	dir := t.TempDir()
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: true,
			SerenaSyncTrigger: "run",
		},
		Git:          &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:    tasksFile,
		CodeIndexer:  &mockCodeIndexerSync{available: true},
		SerenaSyncFn: nil, // nil — no panic expected
	}

	// Should not panic
	_, _ = r.Execute(context.Background())
}

func TestRunner_buildSyncOpts_PopulatesFields(t *testing.T) {
	dir := t.TempDir()

	// Create LEARNINGS.md
	learningsContent := "# Learnings\n- lesson 1\n"
	if err := os.WriteFile(filepath.Join(dir, "LEARNINGS.md"), []byte(learningsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tasks file with completed + open tasks
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	tasksContent := "- [x] Done task 1\n- [ ] Open task 2\n- [x] Done task 3\n"
	if err := os.WriteFile(tasksFile, []byte(tasksContent), 0644); err != nil {
		t.Fatal(err)
	}

	mock := &syncTestGitClient{
		diffStatsResult: &DiffStats{FilesChanged: 10, Insertions: 50, Deletions: 20, Packages: []string{"pkg"}},
	}

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncMaxTurns: 7,
		},
		Git:       mock,
		TasksFile: tasksFile,
	}

	opts := r.buildSyncOpts(context.Background(), "abc123", "")

	if opts.MaxTurns != 7 {
		t.Errorf("MaxTurns = %d, want 7", opts.MaxTurns)
	}
	if opts.ProjectRoot != dir {
		t.Errorf("ProjectRoot = %q, want %q", opts.ProjectRoot, dir)
	}
	if !strings.Contains(opts.DiffSummary, "10 files changed") {
		t.Errorf("DiffSummary = %q, want containing %q", opts.DiffSummary, "10 files changed")
	}
	if !strings.Contains(opts.DiffSummary, "+50/-20") {
		t.Errorf("DiffSummary = %q, want containing %q", opts.DiffSummary, "+50/-20")
	}
	if opts.Learnings != learningsContent {
		t.Errorf("Learnings = %q, want %q", opts.Learnings, learningsContent)
	}
	if !strings.Contains(opts.CompletedTasks, "Done task 1") {
		t.Errorf("CompletedTasks = %q, want containing %q", opts.CompletedTasks, "Done task 1")
	}
	if !strings.Contains(opts.CompletedTasks, "Done task 3") {
		t.Errorf("CompletedTasks = %q, want containing %q", opts.CompletedTasks, "Done task 3")
	}
	if strings.Contains(opts.CompletedTasks, "Open task 2") {
		t.Errorf("CompletedTasks = %q, should not contain open task", opts.CompletedTasks)
	}
}

func TestRunner_buildSyncOpts_EmptyRun(t *testing.T) {
	dir := t.TempDir()

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncMaxTurns: 5,
		},
		Git:       &syncTestGitClient{},
		TasksFile: filepath.Join(dir, "sprint-tasks.md"),
	}

	// Empty initialCommit → no DiffSummary
	opts := r.buildSyncOpts(context.Background(), "", "")

	if opts.DiffSummary != "" {
		t.Errorf("DiffSummary = %q, want empty (no commits)", opts.DiffSummary)
	}
	if opts.Learnings != "" {
		t.Errorf("Learnings = %q, want empty (no LEARNINGS.md)", opts.Learnings)
	}
	if opts.CompletedTasks != "" {
		t.Errorf("CompletedTasks = %q, want empty (no tasks file)", opts.CompletedTasks)
	}
}

func TestExtractCompletedTasks_MixedLines(t *testing.T) {
	dir := t.TempDir()
	tasksFile := filepath.Join(dir, "tasks.md")
	content := "# Sprint Tasks\n- [x] Task A done\n- [ ] Task B open\n- [x] Task C done\n- [ ] Task D open\n"
	if err := os.WriteFile(tasksFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := extractCompletedTasks(tasksFile)

	if strings.Count(result, "[x]") != 2 {
		t.Errorf("completed task count = %d, want 2", strings.Count(result, "[x]"))
	}
	if !strings.Contains(result, "Task A done") {
		t.Errorf("result = %q, want containing %q", result, "Task A done")
	}
	if !strings.Contains(result, "Task C done") {
		t.Errorf("result = %q, want containing %q", result, "Task C done")
	}
	if strings.Contains(result, "Task B open") {
		t.Errorf("result = %q, should not contain open task", result)
	}
}

func TestExtractCompletedTasks_MissingFile(t *testing.T) {
	result := extractCompletedTasks("/nonexistent/path/tasks.md")
	if result != "" {
		t.Errorf("extractCompletedTasks on missing file = %q, want empty string", result)
	}
}

func TestRunner_Execute_SerenaSyncIsolation(t *testing.T) {
	dir := t.TempDir()
	setupMemories(t, dir, []string{"a.md"})

	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: true,
			SerenaSyncTrigger: "run",
			SerenaSyncMaxTurns: 5,
		},
		Git:         &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:   tasksFile,
		CodeIndexer: &mockCodeIndexerSync{available: true},
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			return nil, fmt.Errorf("runner: serena sync: mock failure")
		},
	}

	_, runErr := r.Execute(context.Background())

	// Sync failure should NOT affect runErr — execute() returns ErrNoTasks
	// which is the error from the task loop, not from sync.
	if runErr == nil {
		// runErr may be nil if execute() completed successfully,
		// or non-nil from execute() — either way, sync error must not change it.
		// The key assertion is that sync error is swallowed.
		return
	}
	// If there IS a runErr, it must not be from sync
	if strings.Contains(runErr.Error(), "serena sync") {
		t.Errorf("runErr = %v, should not contain serena sync error (best-effort isolation)", runErr)
	}
}

func TestRunner_Execute_SerenaSyncEmptyTriggerSkips(t *testing.T) {
	dir := t.TempDir()
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: true,
			SerenaSyncTrigger: "", // empty default — batch sync should NOT fire
		},
		Git:         &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:   tasksFile,
		CodeIndexer: &mockCodeIndexerSync{available: true},
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			return nil, nil
		},
	}

	_, _ = r.Execute(context.Background())

	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0 (empty trigger → no batch sync)", syncCalled)
	}
}

func TestRunner_Execute_SerenaSyncTaskTriggerSkips(t *testing.T) {
	dir := t.TempDir()
	tasksFile := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:       dir,
			SerenaSyncEnabled: true,
			SerenaSyncTrigger: "task", // per-task mode — batch sync should NOT fire
		},
		Git:         &syncTestGitClient{headCommits: []string{"abc123"}},
		TasksFile:   tasksFile,
		CodeIndexer: &mockCodeIndexerSync{available: true},
		SerenaSyncFn: func(_ context.Context, _ SerenaSyncOpts) (*session.SessionResult, error) {
			syncCalled++
			return nil, nil
		},
	}

	_, _ = r.Execute(context.Background())

	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0 (task trigger → no batch sync)", syncCalled)
	}
}

// --- Story 8.6: Per-task trigger tests ---

func TestRunner_buildSyncOpts_PerTaskScoping(t *testing.T) {
	dir := t.TempDir()

	// Write LEARNINGS.md
	learningsPath := filepath.Join(dir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("lesson 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write sprint-tasks.md (should be ignored in per-task mode)
	tasksPath := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, []byte("- [x] task A\n- [x] task B"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:        dir,
			SerenaSyncMaxTurns: 5,
		},
		Git: &syncTestGitClient{
			headCommits:     []string{"abc"},
			diffStatsResult: &DiffStats{FilesChanged: 2, Insertions: 10, Deletions: 3},
		},
		TasksFile: tasksPath,
	}

	opts := r.buildSyncOpts(context.Background(), "abc", "- [x] current task only")

	// Per-task mode: CompletedTasks = taskText, not from file
	if opts.CompletedTasks != "- [x] current task only" {
		t.Errorf("CompletedTasks = %q, want per-task text", opts.CompletedTasks)
	}
	// Negative: file-based tasks must NOT leak into per-task CompletedTasks
	if strings.Contains(opts.CompletedTasks, "task A") {
		t.Errorf("CompletedTasks contains file-based 'task A' — should be per-task only")
	}
	// DiffSummary still populated from git
	if !strings.Contains(opts.DiffSummary, "2 files changed") {
		t.Errorf("DiffSummary = %q, want git diff stats", opts.DiffSummary)
	}
	// Learnings still populated
	if opts.Learnings != "lesson 1" {
		t.Errorf("Learnings = %q, want %q", opts.Learnings, "lesson 1")
	}
}

func TestRunner_buildSyncOpts_BatchScoping(t *testing.T) {
	dir := t.TempDir()

	tasksPath := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, []byte("- [x] done A\n- [ ] open B\n- [x] done C"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		Cfg: &config.Config{
			ProjectRoot:        dir,
			SerenaSyncMaxTurns: 5,
		},
		Git:       &syncTestGitClient{},
		TasksFile: tasksPath,
	}

	opts := r.buildSyncOpts(context.Background(), "", "")

	// Batch mode: CompletedTasks from file (only [x] lines)
	if !strings.Contains(opts.CompletedTasks, "done A") {
		t.Errorf("CompletedTasks should contain 'done A', got %q", opts.CompletedTasks)
	}
	if !strings.Contains(opts.CompletedTasks, "done C") {
		t.Errorf("CompletedTasks should contain 'done C', got %q", opts.CompletedTasks)
	}
	if strings.Contains(opts.CompletedTasks, "open B") {
		t.Errorf("CompletedTasks should not contain 'open B', got %q", opts.CompletedTasks)
	}
}

func TestMetricsCollector_RecordSerenaSync_Accumulation(t *testing.T) {
	mc := NewMetricsCollector("run-accum", nil)

	mc.RecordSerenaSync("success", 5000, &session.SessionResult{
		Metrics: &session.SessionMetrics{InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
	})
	mc.RecordSerenaSync("success", 3000, &session.SessionResult{
		Metrics: &session.SessionMetrics{InputTokens: 200, OutputTokens: 80, CostUSD: 0.02},
	})

	rm := mc.Finish()
	if rm.SerenaSync == nil {
		t.Fatal("SerenaSync should be populated")
	}
	sm := rm.SerenaSync
	if sm.DurationMs != 8000 {
		t.Errorf("DurationMs = %d, want 8000 (accumulated)", sm.DurationMs)
	}
	if sm.TokensIn != 300 {
		t.Errorf("TokensIn = %d, want 300 (accumulated)", sm.TokensIn)
	}
	if sm.TokensOut != 130 {
		t.Errorf("TokensOut = %d, want 130 (accumulated)", sm.TokensOut)
	}
	if sm.CostUSD != 0.03 {
		t.Errorf("CostUSD = %f, want 0.03 (accumulated)", sm.CostUSD)
	}
	if sm.Status != "success" {
		t.Errorf("Status = %q, want %q (all success)", sm.Status, "success")
	}
}

// TestMetricsCollector_RecordSerenaSync_PartialStatus removed — duplicate of
// TestMetricsCollector_RecordSerenaSync_MultipleCalls in metrics_test.go which
// tests success+failed+success→"partial" with all 5 field assertions.
// Keeping only the comprehensive version per "no standalone duplicates" rule.

func TestMetricsCollector_RecordSerenaSync_AllFailedStatus(t *testing.T) {
	mc := NewMetricsCollector("run-allfail", nil)

	mc.RecordSerenaSync("failed", 1000, nil)
	mc.RecordSerenaSync("rollback", 2000, nil)

	rm := mc.Finish()
	if rm.SerenaSync == nil {
		t.Fatal("SerenaSync should be populated")
	}
	if rm.SerenaSync.Status != "failed" {
		t.Errorf("Status = %q, want %q (all fail)", rm.SerenaSync.Status, "failed")
	}
	if rm.SerenaSync.DurationMs != 3000 {
		t.Errorf("DurationMs = %d, want 3000", rm.SerenaSync.DurationMs)
	}
}
