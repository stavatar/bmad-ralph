//go:build integration

package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

// =============================================================================
// Review integration tests (Story 4.8)
//
// These tests use RealReview (exported for testing) with MockClaude file side
// effects to validate the full review pipeline end-to-end.
//
// v0.1 smoke test note (AC7): automated integration tests validate the
// execute-review loop mechanics. Manual smoke test with real Claude CLI
// recommended before v0.1 tag — see runner/testdata/manual_smoke_checklist.md.
// =============================================================================

// TestRunner_Execute_ReviewIntegration_CleanReview verifies a single clean
// review pass: execute (commit) -> review (marks [x], deletes findings) -> done.
// AC1, AC5, AC7.
func TestRunner_Execute_ReviewIntegration_CleanReview(t *testing.T) {
	tmpDir := t.TempDir()

	oneTask := "- [ ] Implement feature X\n"
	markedTask := "- [x] Implement feature X\n"

	scenario := testutil.Scenario{
		Name: "clean-review",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-1"},
			{
				Type:        "review",
				ExitCode:    0,
				SessionID:   "rev-1",
				WriteFiles:  map[string]string{"sprint-tasks.md": markedTask},
				DeleteFiles: []string{"review-findings.md"},
			},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, stateDir := setupReviewIntegration(t, tmpDir, oneTask, scenario, mock)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC1: 1 execute + 1 review = 2 HeadCommit calls (before+after for 1 execute)
	if mock.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2", mock.HeadCommitCount)
	}

	// Verify execute session got -p prompt with execute template content
	execArgs := testutil.ReadInvocationArgs(t, stateDir, 0)
	assertArgsContainFlag(t, execArgs, "-p")
	execPrompt := argValueAfterFlag(execArgs, "-p")
	if !strings.Contains(execPrompt, "sprint-tasks.md") {
		t.Errorf("execute prompt: want containing 'sprint-tasks.md', got %d chars", len(execPrompt))
	}

	// Verify review session got -p prompt with task content (injected via __TASK_CONTENT__)
	revArgs := testutil.ReadInvocationArgs(t, stateDir, 1)
	assertArgsContainFlag(t, revArgs, "-p")
	revPrompt := argValueAfterFlag(revArgs, "-p")
	if !strings.Contains(revPrompt, "Implement feature X") {
		t.Errorf("review prompt: want containing task text, got %d chars", len(revPrompt))
	}

	// AC1: task marked [x]
	finalTasks, err := os.ReadFile(filepath.Join(tmpDir, "sprint-tasks.md"))
	if err != nil {
		t.Fatalf("read final tasks: %v", err)
	}
	if !strings.Contains(string(finalTasks), "[x] Implement feature X") {
		t.Errorf("final tasks: want [x] marked, got %q", string(finalTasks))
	}

	// AC1: review-findings.md absent
	_, err = os.Stat(filepath.Join(tmpDir, "review-findings.md"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("review-findings.md: want absent (ErrNotExist), got err=%v", err)
	}
}

// TestRunner_Execute_ReviewIntegration_FindingsFixClean verifies findings -> fix -> clean:
// execute (commit) -> review (findings) -> execute (commit, with findings) -> review (clean).
// AC2, AC5. Also validates Story 4.7 findings injection.
func TestRunner_Execute_ReviewIntegration_FindingsFixClean(t *testing.T) {
	tmpDir := t.TempDir()

	oneTask := "- [ ] Fix auth bug\n"
	markedTask := "- [x] Fix auth bug\n"
	findingsContent := "## [HIGH] Auth bypass\n- **ЧТО не так** — missing validation\n"

	scenario := testutil.Scenario{
		Name: "findings-fix-clean",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-1"},
			{
				Type:       "review",
				ExitCode:   0,
				SessionID:  "rev-1",
				WriteFiles: map[string]string{"review-findings.md": findingsContent},
			},
			{Type: "execute", ExitCode: 0, SessionID: "exec-2"},
			{
				Type:        "review",
				ExitCode:    0,
				SessionID:   "rev-2",
				WriteFiles:  map[string]string{"sprint-tasks.md": markedTask},
				DeleteFiles: []string{"review-findings.md"},
			},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
		),
	}

	r, stateDir := setupReviewIntegration(t, tmpDir, oneTask, scenario, mock)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC2: 2 execute + 2 review sessions = 4 HeadCommit calls
	if mock.HeadCommitCount != 4 {
		t.Errorf("HeadCommitCount = %d, want 4", mock.HeadCommitCount)
	}

	// Verify fix execute (step 2) prompt contains findings content (Story 4.7 validation)
	fixArgs := testutil.ReadInvocationArgs(t, stateDir, 2)
	fixPrompt := argValueAfterFlag(fixArgs, "-p")
	if !strings.Contains(fixPrompt, "Auth bypass") {
		t.Errorf("fix execute prompt: want containing findings text 'Auth bypass', got %d chars", len(fixPrompt))
	}
	if !strings.Contains(fixPrompt, "MUST FIX FIRST") {
		t.Errorf("fix execute prompt: want containing 'MUST FIX FIRST' section header, got %d chars", len(fixPrompt))
	}

	// AC2: task marked [x] at end
	finalTasks, err := os.ReadFile(filepath.Join(tmpDir, "sprint-tasks.md"))
	if err != nil {
		t.Fatalf("read final tasks: %v", err)
	}
	if !strings.Contains(string(finalTasks), "[x] Fix auth bug") {
		t.Errorf("final tasks: want [x] marked, got %q", string(finalTasks))
	}

	// AC2: review-findings.md absent at end
	_, err = os.Stat(filepath.Join(tmpDir, "review-findings.md"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("review-findings.md: want absent at end, got err=%v", err)
	}
}

// TestRunner_Execute_ReviewIntegration_MaxReviewCycles verifies that review exhaust
// continues to next task instead of aborting the run (BUG-2). Metrics record the task
// as "error" with last known HEAD SHA (MINOR-2).
func TestRunner_Execute_ReviewIntegration_MaxReviewCycles(t *testing.T) {
	tmpDir := t.TempDir()

	oneTask := "- [ ] Task one\n"
	findingsContent := "## [MED] Persistent issue\n"

	// 3 cycles: execute + review(findings) x 3
	scenario := testutil.Scenario{
		Name: "max-review-cycles",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-1"},
			{Type: "review", ExitCode: 0, SessionID: "rev-1",
				WriteFiles: map[string]string{"review-findings.md": findingsContent}},
			{Type: "execute", ExitCode: 0, SessionID: "exec-2"},
			{Type: "review", ExitCode: 0, SessionID: "rev-2",
				WriteFiles: map[string]string{"review-findings.md": findingsContent}},
			{Type: "execute", ExitCode: 0, SessionID: "exec-3"},
			{Type: "review", ExitCode: 0, SessionID: "rev-3",
				WriteFiles: map[string]string{"review-findings.md": findingsContent}},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	r, _ := setupReviewIntegration(t, tmpDir, oneTask, scenario, mock)
	r.Cfg.MaxReviewIterations = 3
	r.Cfg.MaxIterations = 1 // single outer iteration: review exhausts, then exits loop
	mc := runner.NewMetricsCollector("test-review-exhaust-integ", nil)
	r.Metrics = mc

	rm, err := r.Execute(context.Background())

	// BUG-2: Execute no longer returns error on review exhaust
	if err != nil {
		t.Fatalf("Execute: want nil error, got %v", err)
	}
	// Verify task recorded as "error" in metrics
	if rm == nil {
		t.Fatal("RunMetrics = nil, want non-nil")
	}
	if len(rm.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(rm.Tasks))
	}
	if rm.Tasks[0].Status != "error" {
		t.Errorf("Tasks[0].Status = %q, want %q", rm.Tasks[0].Status, "error")
	}
	// MINOR-2: last known SHA recorded (last headAfter = "ddd")
	if rm.Tasks[0].CommitSHA != "ddd" {
		t.Errorf("Tasks[0].CommitSHA = %q, want %q", rm.Tasks[0].CommitSHA, "ddd")
	}
	if rm.TasksFailed != 1 {
		t.Errorf("TasksFailed = %d, want 1", rm.TasksFailed)
	}

	// 3 execute + 3 review = 6 HeadCommit calls
	if mock.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_ReviewIntegration_MultiTaskMixed verifies 3 tasks with mixed outcomes:
// task1 (clean), task2 (1 fix cycle), task3 (clean). AC4.
func TestRunner_Execute_ReviewIntegration_MultiTaskMixed(t *testing.T) {
	tmpDir := t.TempDir()

	threeTasks := "- [ ] Task one\n- [ ] Task two\n- [ ] Task three\n"
	afterT1 := "- [x] Task one\n- [ ] Task two\n- [ ] Task three\n"
	findingsT2 := "## [LOW] Style issue in task two\n"
	afterT2 := "- [x] Task one\n- [x] Task two\n- [ ] Task three\n"
	afterT3 := "- [x] Task one\n- [x] Task two\n- [x] Task three\n"

	scenario := testutil.Scenario{
		Name: "multi-task-mixed",
		Steps: []testutil.ScenarioStep{
			// Task 1: execute + clean review
			{Type: "execute", ExitCode: 0, SessionID: "exec-t1"},
			{Type: "review", ExitCode: 0, SessionID: "rev-t1",
				WriteFiles:  map[string]string{"sprint-tasks.md": afterT1},
				DeleteFiles: []string{"review-findings.md"}},
			// Task 2: execute + findings review
			{Type: "execute", ExitCode: 0, SessionID: "exec-t2a"},
			{Type: "review", ExitCode: 0, SessionID: "rev-t2a",
				WriteFiles: map[string]string{"review-findings.md": findingsT2}},
			// Task 2: fix execute + clean review
			{Type: "execute", ExitCode: 0, SessionID: "exec-t2b"},
			{Type: "review", ExitCode: 0, SessionID: "rev-t2b",
				WriteFiles:  map[string]string{"sprint-tasks.md": afterT2},
				DeleteFiles: []string{"review-findings.md"}},
			// Task 3: execute + clean review
			{Type: "execute", ExitCode: 0, SessionID: "exec-t3"},
			{Type: "review", ExitCode: 0, SessionID: "rev-t3",
				WriteFiles:  map[string]string{"sprint-tasks.md": afterT3},
				DeleteFiles: []string{"review-findings.md"}},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"}, // T1
			[2]string{"bbb", "ccc"}, // T2a
			[2]string{"ccc", "ddd"}, // T2b (fix)
			[2]string{"ddd", "eee"}, // T3
		),
	}

	r, _ := setupReviewIntegration(t, tmpDir, threeTasks, scenario, mock)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC4: all 3 tasks marked [x]
	finalTasks, err := os.ReadFile(filepath.Join(tmpDir, "sprint-tasks.md"))
	if err != nil {
		t.Fatalf("read final tasks: %v", err)
	}
	finalStr := string(finalTasks)
	if !strings.Contains(finalStr, "[x] Task one") {
		t.Errorf("final tasks: want [x] Task one, got %q", finalStr)
	}
	if !strings.Contains(finalStr, "[x] Task two") {
		t.Errorf("final tasks: want [x] Task two, got %q", finalStr)
	}
	if !strings.Contains(finalStr, "[x] Task three") {
		t.Errorf("final tasks: want [x] Task three, got %q", finalStr)
	}

	// AC4: review-findings.md absent at end (last review was clean)
	_, findingsErr := os.Stat(filepath.Join(tmpDir, "review-findings.md"))
	if !errors.Is(findingsErr, os.ErrNotExist) {
		t.Errorf("review-findings.md: want absent (ErrNotExist), got err=%v", findingsErr)
	}

	// AC4: 4 execute sessions x 2 HeadCommit calls = 8
	if mock.HeadCommitCount != 8 {
		t.Errorf("HeadCommitCount = %d, want 8", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_ReviewIntegration_BridgeGoldenFile validates the full pipeline
// using bridge golden file output (Story 2.5) as runner input. AC6.
func TestRunner_Execute_ReviewIntegration_BridgeGoldenFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Read bridge golden file — has 2 [x] + 3 [ ] tasks
	goldenData, err := os.ReadFile(filepath.Join("..", "bridge", "testdata", "TestBridge_MergeWithCompleted.golden"))
	if err != nil {
		t.Fatalf("read bridge golden file: %v", err)
	}
	goldenContent := string(goldenData)

	// Build incremental [x] states for 3 open tasks
	afterTask3 := strings.Replace(goldenContent, "- [ ] Implement password reset flow", "- [x] Implement password reset flow", 1)
	afterTask4 := strings.Replace(afterTask3, "- [ ] Add two-factor authentication", "- [x] Add two-factor authentication", 1)
	afterTask5 := strings.Replace(afterTask4, "- [ ] Create session management service", "- [x] Create session management service", 1)

	// 3 open tasks, each with clean review = 6 steps
	scenario := testutil.Scenario{
		Name: "bridge-golden",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-bg1"},
			{Type: "review", ExitCode: 0, SessionID: "rev-bg1",
				WriteFiles:  map[string]string{"sprint-tasks.md": afterTask3},
				DeleteFiles: []string{"review-findings.md"}},
			{Type: "execute", ExitCode: 0, SessionID: "exec-bg2"},
			{Type: "review", ExitCode: 0, SessionID: "rev-bg2",
				WriteFiles:  map[string]string{"sprint-tasks.md": afterTask4},
				DeleteFiles: []string{"review-findings.md"}},
			{Type: "execute", ExitCode: 0, SessionID: "exec-bg3"},
			{Type: "review", ExitCode: 0, SessionID: "rev-bg3",
				WriteFiles:  map[string]string{"sprint-tasks.md": afterTask5},
				DeleteFiles: []string{"review-findings.md"}},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	r, _ := setupReviewIntegration(t, tmpDir, goldenContent, scenario, mock)

	_, execErr := r.Execute(context.Background())
	if execErr != nil {
		t.Fatalf("Execute: unexpected error: %v", execErr)
	}

	// AC6: validates bridge->runner->review data contract end-to-end
	finalTasks, err := os.ReadFile(filepath.Join(tmpDir, "sprint-tasks.md"))
	if err != nil {
		t.Fatalf("read final tasks: %v", err)
	}
	finalStr := string(finalTasks)
	if strings.Contains(finalStr, "- [ ]") {
		t.Errorf("final tasks: want no open tasks, but found '- [ ]' in %q", finalStr)
	}

	// AC6: review-findings.md absent at end (all reviews were clean)
	_, findingsErr := os.Stat(filepath.Join(tmpDir, "review-findings.md"))
	if !errors.Is(findingsErr, os.ErrNotExist) {
		t.Errorf("review-findings.md: want absent (ErrNotExist), got err=%v", findingsErr)
	}

	// 3 execute sessions x 2 HeadCommit calls = 6
	if mock.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6", mock.HeadCommitCount)
	}
}
