//go:build integration

package runner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
	"github.com/bmad-ralph/bmad-ralph/session"
)

// =============================================================================
// Serena Sync Integration Tests (Story 8.7)
//
// Full-flow integration tests exercising sync through Execute() with mock Claude
// via self-reexec, real file operations, and t.TempDir.
// =============================================================================

// setupMemoriesIntegration creates .serena/memories/ with named files in tmpDir.
// Mirrors setupMemories from serena_sync_test.go (internal package).
func setupMemoriesIntegration(t *testing.T, dir string, names []string) {
	t.Helper()
	memDir := filepath.Join(dir, ".serena", "memories")
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

// --- Task 2: Batch sync integration tests (AC: #1, #2, #3, #4, #5) ---

// TestRunner_Execute_SerenaSyncIntegration_HappyPath verifies AC#1: happy path
// batch sync after run — SerenaSyncFn called exactly once with populated opts,
// RunMetrics.SerenaSync.Status == "success".
func TestRunner_Execute_SerenaSyncIntegration_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	setupMemoriesIntegration(t, tmpDir, []string{"a.md", "b.md"})
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	scenario := testutil.Scenario{
		Name: "sync-happy",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-exec-1", CreatesCommit: true},
		},
	}

	// HeadCommit sequence for batch sync (trigger="run") with 1 task:
	// 1: initialCommit in Execute()
	// 2: headBefore in execute retry loop
	// 3: headAfter after commit detection
	// 4: FinishTask
	mock := &testutil.MockGitClient{
		HeadCommits:      []string{"aaa", "aaa", "bbb", "bbb"},
		DiffStatsResults: []*runner.DiffStats{{FilesChanged: 5, Insertions: 30, Deletions: 10}},
	}

	syncCalled := 0
	var capturedOpts runner.SerenaSyncOpts

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.SerenaSyncEnabled = true
	r.Cfg.SerenaSyncTrigger = "run"
	r.Cfg.SerenaSyncMaxTurns = 3
	r.Cfg.MaxIterations = 1
	r.CodeIndexer = &mockCodeIndexer{available: true}
	r.SerenaSyncFn = func(_ context.Context, opts runner.SerenaSyncOpts) (*session.SessionResult, error) {
		syncCalled++
		capturedOpts = opts
		return &session.SessionResult{
			Metrics: &session.SessionMetrics{
				InputTokens:  500,
				OutputTokens: 200,
				CostUSD:      0.05,
			},
		}, nil
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-happy-run"
	r.Metrics = runner.NewMetricsCollector("sync-happy-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if syncCalled != 1 {
		t.Errorf("SerenaSyncFn call count = %d, want 1", syncCalled)
	}
	if capturedOpts.MaxTurns != 3 {
		t.Errorf("MaxTurns = %d, want 3", capturedOpts.MaxTurns)
	}
	if capturedOpts.ProjectRoot != tmpDir {
		t.Errorf("ProjectRoot = %q, want %q", capturedOpts.ProjectRoot, tmpDir)
	}
	if capturedOpts.DiffSummary == "" {
		t.Error("DiffSummary is empty, want non-empty")
	}
	if capturedOpts.Learnings == "" {
		t.Error("Learnings is empty, want non-empty")
	}
	if capturedOpts.CompletedTasks == "" {
		t.Error("CompletedTasks is empty, want non-empty")
	}

	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if rm.SerenaSync == nil {
		t.Fatal("RunMetrics.SerenaSync is nil")
	}
	if rm.SerenaSync.Status != "success" {
		t.Errorf("SerenaSync.Status = %q, want %q", rm.SerenaSync.Status, "success")
	}
	if rm.SerenaSync.DurationMs < 0 {
		t.Errorf("SerenaSync.DurationMs = %d, want >= 0", rm.SerenaSync.DurationMs)
	}
	if rm.SerenaSync.TokensIn != 500 {
		t.Errorf("SerenaSync.TokensIn = %d, want 500", rm.SerenaSync.TokensIn)
	}
	if rm.SerenaSync.TokensOut != 200 {
		t.Errorf("SerenaSync.TokensOut = %d, want 200", rm.SerenaSync.TokensOut)
	}
	if rm.SerenaSync.CostUSD != 0.05 {
		t.Errorf("SerenaSync.CostUSD = %f, want 0.05", rm.SerenaSync.CostUSD)
	}
}

// TestRunner_Execute_SerenaSyncIntegration_Disabled verifies AC#2: sync disabled —
// SerenaSyncFn NOT called, RunMetrics.SerenaSync == nil.
func TestRunner_Execute_SerenaSyncIntegration_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	scenario := testutil.Scenario{
		Name: "sync-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-dis-1", CreatesCommit: true},
		},
	}

	// No sync: headBefore + headAfter + FinishTask = 3 slots
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "bbb", "bbb"},
	}

	syncCalled := 0
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.SerenaSyncEnabled = false
	r.Cfg.MaxIterations = 1
	r.SerenaSyncFn = func(_ context.Context, _ runner.SerenaSyncOpts) (*session.SessionResult, error) {
		syncCalled++
		return nil, nil
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-disabled-run"
	r.Metrics = runner.NewMetricsCollector("sync-disabled-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0", syncCalled)
	}
	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if rm.SerenaSync != nil {
		t.Errorf("RunMetrics.SerenaSync = %v, want nil", rm.SerenaSync)
	}
}

// TestRunner_Execute_SerenaSyncIntegration_Unavailable verifies AC#3: Serena
// unavailable — SerenaSyncFn NOT called, RunMetrics.SerenaSync == nil.
// Note: AC#3 also requires "log contains 'serena not available'" — not asserted
// here because no slog capture infrastructure exists in test helpers (coverage gap).
func TestRunner_Execute_SerenaSyncIntegration_Unavailable(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	scenario := testutil.Scenario{
		Name: "sync-unavailable",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-unavail-1", CreatesCommit: true},
		},
	}

	// initialCommit captured even when unavailable: initialCommit + headBefore + headAfter + FinishTask = 4
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "bbb", "bbb"},
	}

	syncCalled := 0
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.SerenaSyncEnabled = true
	r.Cfg.SerenaSyncTrigger = "run"
	r.Cfg.MaxIterations = 1
	r.CodeIndexer = &mockCodeIndexer{available: false}
	r.SerenaSyncFn = func(_ context.Context, _ runner.SerenaSyncOpts) (*session.SessionResult, error) {
		syncCalled++
		return nil, nil
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-unavail-run"
	r.Metrics = runner.NewMetricsCollector("sync-unavail-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if syncCalled != 0 {
		t.Errorf("SerenaSyncFn call count = %d, want 0", syncCalled)
	}

	// Metrics should not have sync data since sync was skipped
	if rm != nil && rm.SerenaSync != nil {
		t.Errorf("RunMetrics.SerenaSync = %v, want nil (sync skipped)", rm.SerenaSync)
	}
}

// TestRunner_Execute_SerenaSyncIntegration_FailureRollback verifies AC#4: sync
// failure triggers rollback — real .serena/memories/ files restored from backup.
func TestRunner_Execute_SerenaSyncIntegration_FailureRollback(t *testing.T) {
	tmpDir := t.TempDir()
	setupMemoriesIntegration(t, tmpDir, []string{"a.md", "b.md", "c.md"})
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	// Capture original content for verification after rollback
	originalA, err := os.ReadFile(filepath.Join(tmpDir, ".serena/memories/a.md"))
	if err != nil {
		t.Fatalf("read original a.md: %v", err)
	}
	originalC, err := os.ReadFile(filepath.Join(tmpDir, ".serena/memories/c.md"))
	if err != nil {
		t.Fatalf("read original c.md: %v", err)
	}

	scenario := testutil.Scenario{
		Name: "sync-fail-rollback",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-fail-1", CreatesCommit: true},
		},
	}

	// initialCommit + headBefore + headAfter + FinishTask = 4
	mock := &testutil.MockGitClient{
		HeadCommits:      []string{"aaa", "aaa", "bbb", "bbb"},
		DiffStatsResults: []*runner.DiffStats{{FilesChanged: 3, Insertions: 10, Deletions: 5}},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.SerenaSyncEnabled = true
	r.Cfg.SerenaSyncTrigger = "run"
	r.Cfg.MaxIterations = 1
	r.CodeIndexer = &mockCodeIndexer{available: true}
	r.SerenaSyncFn = func(_ context.Context, _ runner.SerenaSyncOpts) (*session.SessionResult, error) {
		return nil, fmt.Errorf("serena API error")
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-fail-run"
	r.Metrics = runner.NewMetricsCollector("sync-fail-run", nil)

	rm, err := r.Execute(context.Background())
	// Sync failure should NOT cause Execute to fail
	if err != nil {
		t.Fatalf("Execute: %v (sync failure should not cause Execute failure)", err)
	}

	// Verify rollback restored files
	restoredA, err := os.ReadFile(filepath.Join(tmpDir, ".serena/memories/a.md"))
	if err != nil {
		t.Fatalf("read restored a.md: %v", err)
	}
	if string(restoredA) != string(originalA) {
		t.Errorf("rollback did not restore a.md content: got %q, want %q", string(restoredA), string(originalA))
	}

	restoredC, err := os.ReadFile(filepath.Join(tmpDir, ".serena/memories/c.md"))
	if err != nil {
		t.Fatalf("read restored c.md: %v", err)
	}
	if string(restoredC) != string(originalC) {
		t.Errorf("rollback did not restore c.md content: got %q, want %q", string(restoredC), string(originalC))
	}

	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if rm.SerenaSync == nil {
		t.Fatal("RunMetrics.SerenaSync is nil")
	}
	if rm.SerenaSync.Status != "failed" {
		t.Errorf("SerenaSync.Status = %q, want %q", rm.SerenaSync.Status, "failed")
	}
}

// TestRunner_Execute_SerenaSyncIntegration_ValidationRollback verifies AC#5:
// SerenaSyncFn "succeeds" but deletes a memory file — validation detects count
// decrease and triggers rollback, status == "failed".
func TestRunner_Execute_SerenaSyncIntegration_ValidationRollback(t *testing.T) {
	tmpDir := t.TempDir()
	setupMemoriesIntegration(t, tmpDir, []string{"a.md", "b.md", "c.md"})
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	originalC, err := os.ReadFile(filepath.Join(tmpDir, ".serena/memories/c.md"))
	if err != nil {
		t.Fatalf("read original c.md: %v", err)
	}

	scenario := testutil.Scenario{
		Name: "sync-validate-rollback",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-val-1", CreatesCommit: true},
		},
	}

	// initialCommit + headBefore + headAfter + FinishTask = 4
	mock := &testutil.MockGitClient{
		HeadCommits:      []string{"aaa", "aaa", "bbb", "bbb"},
		DiffStatsResults: []*runner.DiffStats{{FilesChanged: 2, Insertions: 5, Deletions: 1}},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.SerenaSyncEnabled = true
	r.Cfg.SerenaSyncTrigger = "run"
	r.Cfg.MaxIterations = 1
	r.CodeIndexer = &mockCodeIndexer{available: true}
	r.SerenaSyncFn = func(_ context.Context, _ runner.SerenaSyncOpts) (*session.SessionResult, error) {
		// Delete a memory file to trigger validation failure (countBefore > countAfter)
		if err := os.Remove(filepath.Join(tmpDir, ".serena/memories/c.md")); err != nil {
			t.Fatalf("remove c.md in sync fn: %v", err)
		}
		return nil, nil // "success" but validation will fail
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-val-run"
	r.Metrics = runner.NewMetricsCollector("sync-val-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify rollback restored the deleted file
	restoredC, err := os.ReadFile(filepath.Join(tmpDir, ".serena/memories/c.md"))
	if err != nil {
		t.Fatalf("read restored c.md after validation rollback: %v", err)
	}
	if string(restoredC) != string(originalC) {
		t.Errorf("rollback did not restore c.md: got %q, want %q", string(restoredC), string(originalC))
	}

	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if rm.SerenaSync == nil {
		t.Fatal("RunMetrics.SerenaSync is nil")
	}
	// Validation failure records as "rollback" which is derived to "failed" by accumulation
	if rm.SerenaSync.Status != "failed" {
		t.Errorf("SerenaSync.Status = %q, want %q", rm.SerenaSync.Status, "failed")
	}
}

// --- Task 3: Per-task sync integration tests (AC: #6, #7, #8) ---

// TestRunner_Execute_SerenaSyncIntegration_PerTaskTwoTasks verifies AC#6: per-task
// sync with 2 tasks — SerenaSyncFn called twice, each with task-scoped opts.
func TestRunner_Execute_SerenaSyncIntegration_PerTaskTwoTasks(t *testing.T) {
	tmpDir := t.TempDir()
	setupMemoriesIntegration(t, tmpDir, []string{"a.md"})
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	scenario := testutil.Scenario{
		Name: "sync-per-task",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-task-1", CreatesCommit: true},
			{Type: "execute", ExitCode: 0, SessionID: "sync-task-2", CreatesCommit: true},
		},
	}

	// HeadCommit sequence for per-task sync (trigger="task") with 2 tasks:
	// Task 1: taskHeadBefore(1) + headBefore(2) + headAfter(3) + FinishTask(4)
	// Task 2: taskHeadBefore(5) + headBefore(6) + headAfter(7) + FinishTask(8)
	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "aaa", "bbb", "bbb", // task 1
			"bbb", "bbb", "ccc", "ccc", // task 2
		},
		DiffStatsResults: []*runner.DiffStats{{FilesChanged: 3, Insertions: 15, Deletions: 5}},
	}

	syncCalled := 0
	var capturedOpts []runner.SerenaSyncOpts

	r, _ := setupRunnerIntegration(t, tmpDir, twoOpenTasks, scenario, mock)
	r.Cfg.SerenaSyncEnabled = true
	r.Cfg.SerenaSyncTrigger = "task"
	r.Cfg.SerenaSyncMaxTurns = 3
	r.Cfg.MaxIterations = 2
	r.CodeIndexer = &mockCodeIndexer{available: true}
	r.SerenaSyncFn = func(_ context.Context, opts runner.SerenaSyncOpts) (*session.SessionResult, error) {
		syncCalled++
		capturedOpts = append(capturedOpts, opts)
		return &session.SessionResult{
			Metrics: &session.SessionMetrics{
				InputTokens:  300,
				OutputTokens: 100,
				CostUSD:      0.03,
			},
		}, nil
	}
	r.ReviewFn = progressiveReviewFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-pertask-run"
	r.Metrics = runner.NewMetricsCollector("sync-pertask-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if syncCalled != 2 {
		t.Fatalf("SerenaSyncFn call count = %d, want 2", syncCalled)
	}

	// Verify task-scoped CompletedTasks per call
	if !strings.Contains(capturedOpts[0].CompletedTasks, "Task one") {
		t.Errorf("call 0 CompletedTasks = %q, want contains %q", capturedOpts[0].CompletedTasks, "Task one")
	}
	if strings.Contains(capturedOpts[0].CompletedTasks, "Task two") {
		t.Errorf("call 0 CompletedTasks = %q, should NOT contain %q", capturedOpts[0].CompletedTasks, "Task two")
	}
	if !strings.Contains(capturedOpts[1].CompletedTasks, "Task two") {
		t.Errorf("call 1 CompletedTasks = %q, want contains %q", capturedOpts[1].CompletedTasks, "Task two")
	}
	if strings.Contains(capturedOpts[1].CompletedTasks, "Task one") {
		t.Errorf("call 1 CompletedTasks = %q, should NOT contain %q", capturedOpts[1].CompletedTasks, "Task one")
	}

	// Verify aggregated metrics
	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if rm.SerenaSync == nil {
		t.Fatal("RunMetrics.SerenaSync is nil")
	}
	if rm.SerenaSync.Status != "success" {
		t.Errorf("SerenaSync.Status = %q, want %q", rm.SerenaSync.Status, "success")
	}
	// Accumulated: 2 calls × 300 = 600 tokens in
	if rm.SerenaSync.TokensIn != 600 {
		t.Errorf("SerenaSync.TokensIn = %d, want 600", rm.SerenaSync.TokensIn)
	}
	// Accumulated: 2 calls × 100 = 200 tokens out
	if rm.SerenaSync.TokensOut != 200 {
		t.Errorf("SerenaSync.TokensOut = %d, want 200", rm.SerenaSync.TokensOut)
	}
	// Accumulated: 2 calls × 0.03 = 0.06 cost
	if rm.SerenaSync.CostUSD != 0.06 {
		t.Errorf("SerenaSync.CostUSD = %f, want 0.06", rm.SerenaSync.CostUSD)
	}
}

// TestRunner_Execute_SerenaSyncIntegration_PerTaskFailureNonBlocking verifies AC#7:
// per-task sync failure for task 1 does not prevent task 2 — status == "partial".
func TestRunner_Execute_SerenaSyncIntegration_PerTaskFailureNonBlocking(t *testing.T) {
	tmpDir := t.TempDir()
	setupMemoriesIntegration(t, tmpDir, []string{"a.md"})
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	scenario := testutil.Scenario{
		Name: "sync-pertask-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sync-pf-1", CreatesCommit: true},
			{Type: "execute", ExitCode: 0, SessionID: "sync-pf-2", CreatesCommit: true},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "aaa", "bbb", "bbb", // task 1
			"bbb", "bbb", "ccc", "ccc", // task 2
		},
		DiffStatsResults: []*runner.DiffStats{{FilesChanged: 2, Insertions: 10, Deletions: 3}},
	}

	syncCalled := 0

	r, _ := setupRunnerIntegration(t, tmpDir, twoOpenTasks, scenario, mock)
	r.Cfg.SerenaSyncEnabled = true
	r.Cfg.SerenaSyncTrigger = "task"
	r.Cfg.MaxIterations = 2
	r.CodeIndexer = &mockCodeIndexer{available: true}
	r.SerenaSyncFn = func(_ context.Context, _ runner.SerenaSyncOpts) (*session.SessionResult, error) {
		syncCalled++
		if syncCalled == 1 {
			return nil, fmt.Errorf("sync API error for task 1")
		}
		return &session.SessionResult{
			Metrics: &session.SessionMetrics{
				InputTokens:  200,
				OutputTokens: 80,
				CostUSD:      0.02,
			},
		}, nil
	}
	r.ReviewFn = progressiveReviewFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn
	r.Cfg.RunID = "sync-pf-run"
	r.Metrics = runner.NewMetricsCollector("sync-pf-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Both tasks should complete despite task 1 sync failure
	if syncCalled != 2 {
		t.Errorf("SerenaSyncFn call count = %d, want 2", syncCalled)
	}

	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if rm.SerenaSync == nil {
		t.Fatal("RunMetrics.SerenaSync is nil")
	}
	if rm.SerenaSync.Status != "partial" {
		t.Errorf("SerenaSync.Status = %q, want %q", rm.SerenaSync.Status, "partial")
	}
}

// TestRunner_Execute_SerenaSyncIntegration_MutualExclusion verifies AC#8:
// trigger="task" → no batch sync; trigger="run" → no per-task sync.
func TestRunner_Execute_SerenaSyncIntegration_MutualExclusion(t *testing.T) {
	tests := []struct {
		name       string
		trigger    string
		wantCalls  int // expected total sync calls for 2 tasks
		wantStatus string
	}{
		{
			name:       "task trigger - per-task only",
			trigger:    "task",
			wantCalls:  2, // one per task
			wantStatus: "success",
		},
		{
			name:       "run trigger - batch only",
			trigger:    "run",
			wantCalls:  1, // single batch call
			wantStatus: "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			setupMemoriesIntegration(t, tmpDir, []string{"a.md"})
			writeLearningsFile(t, tmpDir, 10)
			writeDistillState(t, tmpDir, 1, 1)

			scenario := testutil.Scenario{
				Name: "sync-mutex-" + tt.trigger,
				Steps: []testutil.ScenarioStep{
					{Type: "execute", ExitCode: 0, SessionID: "sync-mx-1", CreatesCommit: true},
					{Type: "execute", ExitCode: 0, SessionID: "sync-mx-2", CreatesCommit: true},
				},
			}

			var commits []string
			if tt.trigger == "task" {
				// Per-task: taskHeadBefore + headBefore + headAfter + FinishTask × 2
				commits = []string{
					"aaa", "aaa", "bbb", "bbb",
					"bbb", "bbb", "ccc", "ccc",
				}
			} else {
				// Batch: initialCommit + (headBefore + headAfter + FinishTask) × 2
				commits = []string{
					"aaa",                       // initialCommit
					"aaa", "bbb", "bbb",         // task 1
					"bbb", "ccc", "ccc",         // task 2
				}
			}

			mock := &testutil.MockGitClient{
				HeadCommits:     commits,
				DiffStatsResults: []*runner.DiffStats{{FilesChanged: 2, Insertions: 5, Deletions: 1}},
			}

			syncCalled := 0

			r, _ := setupRunnerIntegration(t, tmpDir, twoOpenTasks, scenario, mock)
			r.Cfg.SerenaSyncEnabled = true
			r.Cfg.SerenaSyncTrigger = tt.trigger
			r.Cfg.MaxIterations = 2
			r.CodeIndexer = &mockCodeIndexer{available: true}
			r.SerenaSyncFn = func(_ context.Context, _ runner.SerenaSyncOpts) (*session.SessionResult, error) {
				syncCalled++
				return nil, nil
			}
			r.ReviewFn = progressiveReviewFn(r.TasksFile, nil)
			r.DistillFn = noopDistillFn
			r.Cfg.RunID = "sync-mx-run"
			r.Metrics = runner.NewMetricsCollector("sync-mx-run", nil)

			rm, err := r.Execute(context.Background())
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}

			if syncCalled != tt.wantCalls {
				t.Errorf("SerenaSyncFn call count = %d, want %d", syncCalled, tt.wantCalls)
			}

			if rm != nil && rm.SerenaSync != nil {
				if rm.SerenaSync.Status != tt.wantStatus {
					t.Errorf("SerenaSync.Status = %q, want %q", rm.SerenaSync.Status, tt.wantStatus)
				}
			}
		})
	}
}

// --- Task 5: Metrics integration tests (AC: #1, #6, #7) ---

// TestRunner_Execute_SerenaSyncIntegration_MetricsNilWhenDisabled removed — duplicate
// of Disabled test which already verifies disabled→SerenaSync==nil with stronger
// assertions (also checks syncCalled==0).

// TestRunner_Execute_SerenaSyncIntegration_FormatSummaryWithSync removed — misleading
// name (formatSummary is in cmd/ralph, inaccessible from runner_test) and strict subset
// of HappyPath assertions (Status+CostUSD). formatSummary tested in cmd/ralph/run_test.go.
