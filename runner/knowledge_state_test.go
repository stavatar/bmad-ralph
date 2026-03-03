package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// TestDistillState_LoadSave verifies round-trip: save → load → verify fields.
func TestDistillState_LoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ralph", "distill-state.json")

	state := &runner.DistillState{
		Version:              1,
		MonotonicTaskCounter: 15,
		LastDistillTask:      10,
	}

	if err := runner.SaveDistillState(path, state); err != nil {
		t.Fatalf("SaveDistillState: %v", err)
	}

	loaded, err := runner.LoadDistillState(path)
	if err != nil {
		t.Fatalf("LoadDistillState: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	if loaded.MonotonicTaskCounter != 15 {
		t.Errorf("MonotonicTaskCounter = %d, want 15", loaded.MonotonicTaskCounter)
	}
	if loaded.LastDistillTask != 10 {
		t.Errorf("LastDistillTask = %d, want 10", loaded.LastDistillTask)
	}
}

// TestDistillState_LoadNotExist verifies missing file returns default {Version:1}.
func TestDistillState_LoadNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "state.json")

	state, err := runner.LoadDistillState(path)
	if err != nil {
		t.Fatalf("LoadDistillState: want nil error for missing file, got %v", err)
	}
	if state.Version != 1 {
		t.Errorf("Version = %d, want 1", state.Version)
	}
	if state.MonotonicTaskCounter != 0 {
		t.Errorf("MonotonicTaskCounter = %d, want 0", state.MonotonicTaskCounter)
	}
	if state.LastDistillTask != 0 {
		t.Errorf("LastDistillTask = %d, want 0", state.LastDistillTask)
	}
}

// TestDistillState_LoadInvalid verifies corrupt JSON returns error with prefix.
func TestDistillState_LoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	_, err := runner.LoadDistillState(path)
	if err == nil {
		t.Fatal("LoadDistillState: want error for corrupt JSON, got nil")
	}
	if !strings.Contains(err.Error(), "runner: distill state: load:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill state: load:")
	}
}

// TestDistillState_SaveCreatesDir verifies .ralph/ directory is created if not exists.
func TestDistillState_SaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ralph", "distill-state.json")

	// .ralph/ does not exist yet
	state := &runner.DistillState{Version: 1, MonotonicTaskCounter: 5}
	if err := runner.SaveDistillState(path, state); err != nil {
		t.Fatalf("SaveDistillState: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(filepath.Join(dir, ".ralph"))
	if err != nil {
		t.Fatalf("stat .ralph dir: %v", err)
	}
	if !info.IsDir() {
		t.Error(".ralph should be a directory")
	}

	// Verify file contents
	loaded, err := runner.LoadDistillState(path)
	if err != nil {
		t.Fatalf("LoadDistillState: %v", err)
	}
	if loaded.MonotonicTaskCounter != 5 {
		t.Errorf("MonotonicTaskCounter = %d, want 5", loaded.MonotonicTaskCounter)
	}
}

// TestDistillState_LoadReadError verifies non-NotExist read errors are wrapped.
func TestDistillState_LoadReadError(t *testing.T) {
	// Use a directory as the file path to trigger a read error
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("create dir-as-file: %v", err)
	}

	_, err := runner.LoadDistillState(path)
	if err == nil {
		t.Fatal("LoadDistillState: want error for directory-as-file, got nil")
	}
	if !strings.Contains(err.Error(), "runner: distill state: load:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill state: load:")
	}
}

// --- Story 6.5c: Metrics serialization ---

// TestDistillState_MetricsSerialization verifies round-trip with Metrics field.
func TestDistillState_MetricsSerialization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ralph", "distill-state.json")

	state := &runner.DistillState{
		Version:              1,
		MonotonicTaskCounter: 20,
		LastDistillTask:      15,
		Metrics: &runner.DistillMetrics{
			EntriesBefore:        30,
			EntriesAfter:         25,
			StaleRemoved:         5,
			CategoriesPreserved:  3,
			CategoriesTotal:      3,
			NeedsFormattingFixed: 2,
			T1Promotions:         1,
			LastDistillTime:      "2026-03-02T14:30:00Z",
		},
	}

	if err := runner.SaveDistillState(path, state); err != nil {
		t.Fatalf("SaveDistillState: %v", err)
	}

	loaded, err := runner.LoadDistillState(path)
	if err != nil {
		t.Fatalf("LoadDistillState: %v", err)
	}

	if loaded.Metrics == nil {
		t.Fatal("Metrics should not be nil after round-trip")
	}
	if loaded.Metrics.EntriesBefore != 30 {
		t.Errorf("EntriesBefore = %d, want 30", loaded.Metrics.EntriesBefore)
	}
	if loaded.Metrics.EntriesAfter != 25 {
		t.Errorf("EntriesAfter = %d, want 25", loaded.Metrics.EntriesAfter)
	}
	if loaded.Metrics.StaleRemoved != 5 {
		t.Errorf("StaleRemoved = %d, want 5", loaded.Metrics.StaleRemoved)
	}
	if loaded.Metrics.CategoriesPreserved != 3 {
		t.Errorf("CategoriesPreserved = %d, want 3", loaded.Metrics.CategoriesPreserved)
	}
	if loaded.Metrics.CategoriesTotal != 3 {
		t.Errorf("CategoriesTotal = %d, want 3", loaded.Metrics.CategoriesTotal)
	}
	if loaded.Metrics.NeedsFormattingFixed != 2 {
		t.Errorf("NeedsFormattingFixed = %d, want 2", loaded.Metrics.NeedsFormattingFixed)
	}
	if loaded.Metrics.T1Promotions != 1 {
		t.Errorf("T1Promotions = %d, want 1", loaded.Metrics.T1Promotions)
	}
	if loaded.Metrics.LastDistillTime != "2026-03-02T14:30:00Z" {
		t.Errorf("LastDistillTime = %q, want %q", loaded.Metrics.LastDistillTime, "2026-03-02T14:30:00Z")
	}
}

// TestDistillState_MetricsOmitEmpty verifies nil Metrics produces no "metrics" key in JSON.
func TestDistillState_MetricsOmitEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ralph", "distill-state.json")

	state := &runner.DistillState{
		Version:              1,
		MonotonicTaskCounter: 5,
	}

	if err := runner.SaveDistillState(path, state); err != nil {
		t.Fatalf("SaveDistillState: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.Contains(string(data), `"metrics"`) {
		t.Errorf("JSON should not contain \"metrics\" key when Metrics is nil, got: %s", string(data))
	}
}

// TestDistillState_VersionZero verifies missing Version treated as 0 (pre-versioning).
func TestDistillState_VersionZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write JSON without version field
	content := `{"monotonic_task_counter": 10, "last_distill_task": 5}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := runner.LoadDistillState(path)
	if err != nil {
		t.Fatalf("LoadDistillState: %v", err)
	}
	if loaded.Version != 0 {
		t.Errorf("Version = %d, want 0 (pre-versioning)", loaded.Version)
	}
	if loaded.MonotonicTaskCounter != 10 {
		t.Errorf("MonotonicTaskCounter = %d, want 10", loaded.MonotonicTaskCounter)
	}
}

// --- Story 6.5c: Recovery tests ---

// TestRecoverDistillation_NoIntent verifies no intent file = no-op.
func TestRecoverDistillation_NoIntent(t *testing.T) {
	dir := t.TempDir()

	err := runner.RecoverDistillation(dir)
	if err != nil {
		t.Fatalf("RecoverDistillation: want nil for no intent file, got %v", err)
	}
}

// TestRecoverDistillation_WritePhase verifies pending renames completed.
func TestRecoverDistillation_WritePhase(t *testing.T) {
	dir := t.TempDir()

	// Create .pending files
	file1 := filepath.Join(dir, "LEARNINGS.md")
	file2 := filepath.Join(dir, "ralph-testing.md")
	if err := os.WriteFile(file1+".pending", []byte("recovered1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2+".pending", []byte("recovered2"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create intent file with phase=write
	intent := &runner.DistillIntent{
		Timestamp: "2026-03-02T14:30:00Z",
		Files:     []string{file1, file2},
		Phase:     "write",
	}
	if err := runner.WriteIntentFile(dir, intent); err != nil {
		t.Fatalf("WriteIntentFile: %v", err)
	}

	err := runner.RecoverDistillation(dir)
	if err != nil {
		t.Fatalf("RecoverDistillation: %v", err)
	}

	// Verify files renamed
	content1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("read file1 after recovery: %v", err)
	}
	if string(content1) != "recovered1" {
		t.Errorf("file1 content = %q, want %q", string(content1), "recovered1")
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("read file2 after recovery: %v", err)
	}
	if string(content2) != "recovered2" {
		t.Errorf("file2 content = %q, want %q", string(content2), "recovered2")
	}

	// Verify intent file deleted
	loaded, err := runner.ReadIntentFile(dir)
	if err != nil {
		t.Fatalf("ReadIntentFile after recovery: %v", err)
	}
	if loaded != nil {
		t.Error("intent file should be deleted after recovery")
	}
}

// TestRecoverDistillation_BackupPhase verifies .pending files deleted (rollback).
func TestRecoverDistillation_BackupPhase(t *testing.T) {
	dir := t.TempDir()

	// Create .pending files
	file1 := filepath.Join(dir, "LEARNINGS.md")
	if err := os.WriteFile(file1+".pending", []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create intent file with phase=backup
	intent := &runner.DistillIntent{
		Timestamp: "2026-03-02T14:30:00Z",
		Files:     []string{file1},
		Phase:     "backup",
	}
	if err := runner.WriteIntentFile(dir, intent); err != nil {
		t.Fatalf("WriteIntentFile: %v", err)
	}

	err := runner.RecoverDistillation(dir)
	if err != nil {
		t.Fatalf("RecoverDistillation: %v", err)
	}

	// Verify .pending deleted (rollback)
	if _, err := os.Stat(file1 + ".pending"); !os.IsNotExist(err) {
		t.Error("file1.pending should be deleted during backup rollback")
	}

	// Verify target NOT created (rollback = no rename)
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Error("file1 should NOT exist after backup rollback")
	}
}

// TestRecoverDistillation_CommitError verifies error wrapping when CommitPendingFiles fails.
func TestRecoverDistillation_CommitError(t *testing.T) {
	dir := t.TempDir()

	// Create .pending as regular file, but target as directory — os.Rename fails
	file1 := filepath.Join(dir, "LEARNINGS.md")
	if err := os.MkdirAll(file1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file1+".pending", []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	intent := &runner.DistillIntent{
		Timestamp: "2026-03-02T14:30:00Z",
		Files:     []string{file1},
		Phase:     "write",
	}
	if err := runner.WriteIntentFile(dir, intent); err != nil {
		t.Fatalf("WriteIntentFile: %v", err)
	}

	err := runner.RecoverDistillation(dir)
	if err == nil {
		t.Fatal("RecoverDistillation: want error for commit failure, got nil")
	}
	if !strings.Contains(err.Error(), "runner: distill: recovery:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: recovery:")
	}
	if !strings.Contains(err.Error(), "runner: distill: commit:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: commit:")
	}
}

// TestRecoverDistillation_CleansUp verifies intent + .pending files deleted after recovery.
func TestRecoverDistillation_CleansUp(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "LEARNINGS.md")
	if err := os.WriteFile(file1+".pending", []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	intent := &runner.DistillIntent{
		Timestamp: "2026-03-02T14:30:00Z",
		Files:     []string{file1},
		Phase:     "commit",
	}
	if err := runner.WriteIntentFile(dir, intent); err != nil {
		t.Fatalf("WriteIntentFile: %v", err)
	}

	err := runner.RecoverDistillation(dir)
	if err != nil {
		t.Fatalf("RecoverDistillation: %v", err)
	}

	// Intent file deleted
	loaded, err := runner.ReadIntentFile(dir)
	if err != nil {
		t.Fatalf("ReadIntentFile: %v", err)
	}
	if loaded != nil {
		t.Error("intent file should be deleted after cleanup")
	}

	// .pending file deleted
	if _, err := os.Stat(file1 + ".pending"); !os.IsNotExist(err) {
		t.Error("file1.pending should be deleted after cleanup")
	}
}
