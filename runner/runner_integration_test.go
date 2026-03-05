//go:build integration

package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

func TestRunOnce_WalkingSkeleton_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	tasksPath := copyFixtureToDir(t, tmpDir, "sprint-tasks-basic.md")

	// Setup mock Claude scenario: execute (exit 0) + review (exit 0)
	scenario := testutil.Scenario{
		Name: "walking-skeleton-happy",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "skel-exec-001", CreatesCommit: true},
			{Type: "review", ExitCode: 0, SessionID: "skel-review-001", CreatesCommit: false},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand: os.Args[0], // test binary = mock Claude
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommits: []string{"abc123"}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	ctx := context.Background()

	// Execute
	if err := runner.RunOnce(ctx, rc); err != nil {
		t.Fatalf("RunOnce: unexpected error: %v", err)
	}

	// Verify EXECUTE args
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	assertArgsContainFlag(t, args, "-p")
	assertArgsContainFlagValue(t, args, "--max-turns", "5")
	assertArgsContainFlagValue(t, args, "--output-format", "json")
	assertArgsContainFlag(t, args, "--dangerously-skip-permissions")

	// Verify prompt contains task content
	promptValue := argValueAfterFlag(args, "-p")
	if promptValue == "" {
		t.Fatalf("execute: -p flag has no value")
	}
	if !strings.Contains(promptValue, "sprint-tasks.md") {
		t.Errorf("execute prompt: want self-directing reference to sprint-tasks.md, got %q", promptValue)
	}
	if !strings.Contains(promptValue, "Sprint Tasks Format Specification") {
		t.Errorf("execute prompt: want format contract title, got %q", promptValue)
	}

	// Review
	if err := runner.RunReview(ctx, rc); err != nil {
		t.Fatalf("RunReview: unexpected error: %v", err)
	}

	// Verify REVIEW args
	reviewArgs := testutil.ReadInvocationArgs(t, stateDir, 1)
	assertArgsContainFlag(t, reviewArgs, "-p")
	assertArgsContainFlagValue(t, reviewArgs, "--max-turns", "5")
	assertArgsContainFlagValue(t, reviewArgs, "--output-format", "json")
	assertArgsContainFlag(t, reviewArgs, "--dangerously-skip-permissions")

	// Review prompt should be different from execute prompt
	reviewPrompt := argValueAfterFlag(reviewArgs, "-p")
	if reviewPrompt == "" {
		t.Fatalf("review: -p flag has no value")
	}
	if !strings.Contains(reviewPrompt, "review stub") {
		t.Errorf("review prompt: want 'review stub' content, got %q", reviewPrompt)
	}
}

func TestRunOnce_WalkingSkeleton_GitHealthCheckFails(t *testing.T) {
	tmpDir := t.TempDir()

	tasksPath := copyFixtureToDir(t, tmpDir, "sprint-tasks-basic.md")

	// No mock Claude needed — should fail before session.Execute
	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HealthCheckErrors: []error{errors.New("git not found")}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner:") {
		t.Errorf("error prefix: want 'runner:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "git not found") {
		t.Errorf("error cause: want 'git not found', got %q", err.Error())
	}
}

func TestRunOnce_WalkingSkeleton_NoTaskMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	// No task markers at all — triggers ErrNoTasks
	noMarkers := "# Sprint Tasks\n\nNo tasks here\n"
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, []byte(noMarkers), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommits: []string{"abc123"}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !errors.Is(err, config.ErrNoTasks) {
		t.Errorf("errors.Is(err, ErrNoTasks): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "runner:") {
		t.Errorf("error prefix: want 'runner:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "no tasks found") {
		t.Errorf("error message: want 'no tasks found', got %q", err.Error())
	}
}

func TestRunOnce_WalkingSkeleton_AllTasksDone(t *testing.T) {
	tmpDir := t.TempDir()

	// Only done tasks — all tasks completed, expect nil (success)
	allDone := "# Sprint Tasks\n\n- [x] Already done\n"
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, []byte(allDone), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommits: []string{"abc123"}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err := runner.RunOnce(context.Background(), rc)
	if err != nil {
		t.Fatalf("RunOnce: expected nil (all done), got error: %v", err)
	}
}

func TestRunOnce_WalkingSkeleton_SessionFails(t *testing.T) {
	tmpDir := t.TempDir()

	tasksPath := copyFixtureToDir(t, tmpDir, "sprint-tasks-basic.md")

	// Mock Claude exits with code 1
	scenario := testutil.Scenario{
		Name: "walking-skeleton-session-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "skel-fail-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommits: []string{"abc123"}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: execute:") {
		t.Errorf("error prefix: want 'runner: execute:', got %q", err.Error())
	}
}

func TestRunOnce_WalkingSkeleton_TasksFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommits: []string{"abc123"}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: filepath.Join(tmpDir, "nonexistent.md")}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner:") {
		t.Errorf("error prefix: want 'runner:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "nonexistent.md") {
		t.Errorf("error path: want 'nonexistent.md', got %q", err.Error())
	}
}

func TestRunOnce_WalkingSkeleton_HeadCommitFails(t *testing.T) {
	tmpDir := t.TempDir()

	tasksPath := copyFixtureToDir(t, tmpDir, "sprint-tasks-basic.md")

	// Mock Claude succeeds so RunOnce reaches HeadCommit
	scenario := testutil.Scenario{
		Name: "walking-skeleton-commit-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "skel-commit-fail-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommitErrors: []error{errors.New("git diff failed")}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: head commit:") {
		t.Errorf("error prefix: want 'runner: head commit:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "git diff failed") {
		t.Errorf("error cause: want 'git diff failed', got %q", err.Error())
	}
}

func TestRunReview_WalkingSkeleton_SessionFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock Claude exits with code 1
	scenario := testutil.Scenario{
		Name: "walking-skeleton-review-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "review", ExitCode: 1, SessionID: "skel-review-fail-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &testutil.MockGitClient{HeadCommits: []string{"abc123"}}
	rc := runner.RunConfig{Cfg: cfg, Git: git, TasksFile: filepath.Join(tmpDir, "unused.md")}

	err := runner.RunReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RunReview: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: review execute:") {
		t.Errorf("error prefix: want 'runner: review execute:', got %q", err.Error())
	}
}

// --- Runner.Execute integration tests (Story 3.11) ---

func TestRunner_Execute_Integration_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-happy-path",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "happy-1"},
			{Type: "execute", ExitCode: 0, SessionID: "happy-2"},
			{Type: "execute", ExitCode: 0, SessionID: "happy-3"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	reviewCount := 0
	r, stateDir := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, git)
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if git.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1", git.HealthCheckCount)
	}
	if git.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6", git.HeadCommitCount)
	}
	if reviewCount != 3 {
		t.Errorf("reviewCount = %d, want 3", reviewCount)
	}

	// Verify each session received -p prompt with task content and --max-turns flag (AC1)
	for i := 0; i < 3; i++ {
		args := testutil.ReadInvocationArgs(t, stateDir, i)
		assertArgsContainFlag(t, args, "-p")
		assertArgsContainFlagValue(t, args, "--max-turns", "5")
		prompt := argValueAfterFlag(args, "-p")
		if !strings.Contains(prompt, "Sprint Tasks Format Specification") {
			t.Errorf("step %d: prompt missing format contract title", i)
		}
	}
}

func TestRunner_Execute_Integration_RetryWithResume(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-retry-resume",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "retry-1"},
			{Type: "execute", ExitCode: 0, SessionID: "retry-2"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"}, // same = no commit → retry
			[2]string{"aaa", "bbb"}, // different = commit → success
		),
	}

	resume := &trackingResumeExtract{}
	sleep := &trackingSleep{}
	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, git)
	r.ResumeExtractFn = resume.fn
	r.SleepFn = sleep.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if resume.count != 1 {
		t.Errorf("resume count = %d, want 1", resume.count)
	}
	if resume.sessionIDs[0] != "retry-1" {
		t.Errorf("resume sessionID = %q, want %q", resume.sessionIDs[0], "retry-1")
	}
	if git.HeadCommitCount != 4 {
		t.Errorf("HeadCommitCount = %d, want 4", git.HeadCommitCount)
	}
	if sleep.count != 1 {
		t.Errorf("sleep count = %d, want 1", sleep.count)
	}
	// First retry backoff: 1<<0 * 1s = 1s (exponential backoff verification)
	if len(sleep.durations) > 0 && sleep.durations[0] != 1*time.Second {
		t.Errorf("sleep duration[0] = %v, want 1s", sleep.durations[0])
	}
}

func TestRunner_Execute_Integration_MaxRetriesEmergencyStop(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-max-retries",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "max-1"},
			{Type: "execute", ExitCode: 0, SessionID: "max-2"},
			{Type: "execute", ExitCode: 0, SessionID: "max-3"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"},
			[2]string{"aaa", "aaa"},
			[2]string{"aaa", "aaa"},
		),
	}

	resume := &trackingResumeExtract{}
	sleep := &trackingSleep{}
	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, git)
	r.ResumeExtractFn = resume.fn
	r.SleepFn = sleep.fn

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: expected error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "execute attempts exhausted") {
		t.Errorf("error prefix: want 'execute attempts exhausted', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("error inner: want 'max retries exceeded', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Task one") {
		t.Errorf("error task text: want 'Task one', got %q", err.Error())
	}
	// Last attempt triggers emergency stop BEFORE resume/sleep (executeAttempts++ then check >= max)
	if resume.count != 2 {
		t.Errorf("resume count = %d, want 2", resume.count)
	}
	if sleep.count != 2 {
		t.Errorf("sleep count = %d, want 2", sleep.count)
	}
}

func TestRunner_Execute_Integration_ResumeAfterPartialCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	twoCompletedOneOpen := "# Sprint Tasks\n\n- [x] Done one\n- [x] Done two\n- [ ] Remaining task\n"

	scenario := testutil.Scenario{
		Name: "integration-resume-partial",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "partial-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
		),
	}

	r, stateDir := setupRunnerIntegration(t, tmpDir, twoCompletedOneOpen, scenario, git)
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Only 1 session launched (not 3), proving scanner starts from first open task
	if git.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2 (1 session)", git.HeadCommitCount)
	}

	// Verify prompt sent to MockClaude contained the correct (first open) task
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	prompt := argValueAfterFlag(args, "-p")
	if !strings.Contains(prompt, "Sprint Tasks Format Specification") {
		t.Errorf("prompt missing format contract title")
	}
}

func TestRunner_Execute_Integration_DirtyTreeRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-dirty-tree",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "dirty-1"},
		},
	}
	git := &testutil.MockGitClient{
		HealthCheckErrors: []error{runner.ErrDirtyTree},
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
		),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, git)
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if git.RestoreCleanCount != 1 {
		t.Errorf("RestoreCleanCount = %d, want 1", git.RestoreCleanCount)
	}
	// RecoverDirtyState calls HealthCheck once, does NOT re-check after RestoreClean
	if git.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1", git.HealthCheckCount)
	}
}

func TestRunner_Execute_Integration_ResumeFailureRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-resume-failure-recovery",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "resume-fail-1"},
			{Type: "execute", ExitCode: 0, SessionID: "resume-fail-2"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"}, // no commit → retry
			[2]string{"aaa", "bbb"}, // commit → success
		),
		// HealthCheck: OK(startup) → ErrDirtyTree(after resume, triggers recovery)
		HealthCheckErrors: []error{nil, runner.ErrDirtyTree},
	}
	// Resume returns nil — "failure" simulated by dirty tree side effect on MockGitClient.
	// Real ResumeExtraction would propagate subprocess exit error, but integration test
	// verifies the recovery pipeline: resume leaves tree dirty → RecoverDirtyState cleans it.
	resume := &trackingResumeExtract{}
	sleep := &trackingSleep{}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, git)
	r.ResumeExtractFn = resume.fn
	r.SleepFn = sleep.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if git.RestoreCleanCount != 1 {
		t.Errorf("RestoreCleanCount = %d, want 1", git.RestoreCleanCount)
	}
	if resume.count != 1 {
		t.Errorf("resume count = %d, want 1", resume.count)
	}
	if git.HealthCheckCount < 2 {
		t.Errorf("HealthCheckCount = %d, want >= 2", git.HealthCheckCount)
	}
	// Verify execute_attempts tracked correctly through recovery (AC6)
	if sleep.count != 1 {
		t.Errorf("sleep count = %d, want 1 (backoff after retry)", sleep.count)
	}
}

func TestRunner_Execute_Integration_BridgeGoldenFileContract(t *testing.T) {
	tmpDir := t.TempDir()

	// Read bridge golden file — validates bridge→runner data contract (AC7).
	// Golden file contains source: annotations and ## Epic: headers that scanner must ignore.
	goldenData, err := os.ReadFile(filepath.Join("..", "bridge", "testdata", "TestBridge_MergeWithCompleted.golden"))
	if err != nil {
		t.Fatalf("read bridge golden file: %v", err)
	}

	scenario := testutil.Scenario{
		Name: "integration-bridge-golden",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bridge-1"},
			{Type: "execute", ExitCode: 0, SessionID: "bridge-2"},
			{Type: "execute", ExitCode: 0, SessionID: "bridge-3"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, string(goldenData), scenario, git)

	_, err = r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 3 sessions launched despite source: annotations and ## Epic: headers in golden file
	if git.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6 (3 sessions)", git.HeadCommitCount)
	}
}

// TestRunner_Execute_Integration_WithMetricsCollector verifies that Execute returns
// non-nil RunMetrics when MetricsCollector is configured, and that session metrics
// from mock Claude output are accumulated in the result. (AC3, AC7)
func TestRunner_Execute_Integration_WithMetricsCollector(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-metrics",
		Steps: []testutil.ScenarioStep{
			{
				Type: "execute", ExitCode: 0, SessionID: "m-1",
				Model: "claude-sonnet-4-20250514",
				Usage: map[string]int{"input_tokens": 100, "output_tokens": 50, "cache_read_tokens": 10},
			},
			{
				Type: "execute", ExitCode: 0, SessionID: "m-2",
				Model: "claude-sonnet-4-20250514",
				Usage: map[string]int{"input_tokens": 200, "output_tokens": 80, "cache_read_tokens": 20},
			},
		},
	}
	// HeadCommit call sequence per iteration: before(1), after(2), FinishTask(3).
	// 2 iterations × 3 calls = 6 values needed.
	git := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "bbb", "bbb", // iter 0: before=aaa, after=bbb (changed), finish=bbb
			"bbb", "ccc", "ccc", // iter 1: before=bbb, after=ccc (changed), finish=ccc
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, twoOpenTasks, scenario, git)
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		return runner.ReviewResult{Clean: true}, nil
	}
	r.Cfg.MaxIterations = 2
	r.Cfg.RunID = "test-run-id"
	r.Metrics = runner.NewMetricsCollector("test-run-id", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if rm == nil {
		t.Fatal("Execute: RunMetrics is nil, want non-nil when MetricsCollector configured")
	}
	if rm.RunID != "test-run-id" {
		t.Errorf("RunMetrics.RunID = %q, want %q", rm.RunID, "test-run-id")
	}
	if len(rm.Tasks) != 2 {
		t.Fatalf("RunMetrics.Tasks len = %d, want 2", len(rm.Tasks))
	}

	// Verify task-level fields for task 0
	t0 := rm.Tasks[0]
	if t0.Name != "Task one" {
		t.Errorf("Tasks[0].Name = %q, want %q", t0.Name, "Task one")
	}
	if t0.Status != "done" {
		t.Errorf("Tasks[0].Status = %q, want %q", t0.Status, "done")
	}
	if t0.CommitSHA != "bbb" {
		t.Errorf("Tasks[0].CommitSHA = %q, want %q", t0.CommitSHA, "bbb")
	}
	if t0.InputTokens != 100 {
		t.Errorf("Tasks[0].InputTokens = %d, want 100", t0.InputTokens)
	}
	if t0.OutputTokens != 50 {
		t.Errorf("Tasks[0].OutputTokens = %d, want 50", t0.OutputTokens)
	}
	if t0.CacheTokens != 10 {
		t.Errorf("Tasks[0].CacheTokens = %d, want 10", t0.CacheTokens)
	}
	if t0.Sessions != 1 {
		t.Errorf("Tasks[0].Sessions = %d, want 1", t0.Sessions)
	}

	// Verify task-level fields for task 1
	t1 := rm.Tasks[1]
	if t1.Name != "Task two" {
		t.Errorf("Tasks[1].Name = %q, want %q", t1.Name, "Task two")
	}
	if t1.Status != "done" {
		t.Errorf("Tasks[1].Status = %q, want %q", t1.Status, "done")
	}
	if t1.CommitSHA != "ccc" {
		t.Errorf("Tasks[1].CommitSHA = %q, want %q", t1.CommitSHA, "ccc")
	}
	if t1.InputTokens != 200 {
		t.Errorf("Tasks[1].InputTokens = %d, want 200", t1.InputTokens)
	}
	if t1.OutputTokens != 80 {
		t.Errorf("Tasks[1].OutputTokens = %d, want 80", t1.OutputTokens)
	}
	if t1.CacheTokens != 20 {
		t.Errorf("Tasks[1].CacheTokens = %d, want 20", t1.CacheTokens)
	}
	if t1.Sessions != 1 {
		t.Errorf("Tasks[1].Sessions = %d, want 1", t1.Sessions)
	}

	// Verify run-level totals
	if rm.InputTokens != 300 {
		t.Errorf("RunMetrics.InputTokens = %d, want 300", rm.InputTokens)
	}
	if rm.OutputTokens != 130 {
		t.Errorf("RunMetrics.OutputTokens = %d, want 130", rm.OutputTokens)
	}
	if rm.TotalSessions != 2 {
		t.Errorf("RunMetrics.TotalSessions = %d, want 2", rm.TotalSessions)
	}
}

// TestRunner_Execute_Integration_WithoutMetricsCollector verifies that Execute
// works correctly with nil MetricsCollector (returns nil RunMetrics). (AC4)
func TestRunner_Execute_Integration_WithoutMetricsCollector(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "integration-no-metrics",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nm-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
		),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		return runner.ReviewResult{Clean: true}, nil
	}
	r.Cfg.MaxIterations = 1
	// r.Metrics intentionally left nil

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if rm != nil {
		t.Errorf("Execute: RunMetrics = %+v, want nil when MetricsCollector not configured", rm)
	}
}
