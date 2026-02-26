package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

func TestRecoverDirtyState_Scenarios(t *testing.T) {
	tests := []struct {
		name                 string
		mock                 *testutil.MockGitClient
		wantRecovered        bool
		wantErr              bool
		wantErrContains      string
		wantErrContainsInner string
		wantErrIs            error
		wantHealthCheckCount int
		wantRestoreCount     int
	}{
		{
			name:                 "clean repo",
			mock:                 &testutil.MockGitClient{},
			wantRecovered:        false,
			wantErr:              false,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "dirty tree recovery succeeds",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrDirtyTree},
			},
			wantRecovered:        true,
			wantErr:              false,
			wantHealthCheckCount: 1,
			wantRestoreCount:     1,
		},
		{
			name: "dirty tree recovery fails",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrDirtyTree},
				RestoreCleanError: errors.New("restore failed"),
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrContainsInner: "restore failed",
			wantHealthCheckCount: 1,
			wantRestoreCount:     1,
		},
		{
			name: "detached HEAD",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrDetachedHead},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            runner.ErrDetachedHead,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "merge in progress",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrMergeInProgress},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            runner.ErrMergeInProgress,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "context canceled",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{context.Canceled},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            context.Canceled,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "context deadline exceeded",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{context.DeadlineExceeded},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            context.DeadlineExceeded,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recovered, err := runner.RecoverDirtyState(context.Background(), tc.mock)

			if recovered != tc.wantRecovered {
				t.Errorf("recovered: got %v, want %v", recovered, tc.wantRecovered)
			}

			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil error, got %v", err)
			}

			if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error message: want containing %q, got %q", tc.wantErrContains, err.Error())
			}

			if tc.wantErrContainsInner != "" && !strings.Contains(err.Error(), tc.wantErrContainsInner) {
				t.Errorf("inner error: want containing %q, got %q", tc.wantErrContainsInner, err.Error())
			}

			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("errors.Is(err, %v): want true, got false; err = %v", tc.wantErrIs, err)
			}

			if tc.mock.HealthCheckCount != tc.wantHealthCheckCount {
				t.Errorf("HealthCheckCount: got %d, want %d", tc.mock.HealthCheckCount, tc.wantHealthCheckCount)
			}

			if tc.mock.RestoreCleanCount != tc.wantRestoreCount {
				t.Errorf("RestoreCleanCount: got %d, want %d", tc.mock.RestoreCleanCount, tc.wantRestoreCount)
			}
		})
	}
}

// =============================================================================
// Task 6: Execute happy path tests (AC: #1, #2, #3)
// =============================================================================

// TestRunner_Execute_SequentialExecution verifies N sequential sessions execute
// with correct call counts and fresh sessions (AC1, AC2).
func TestRunner_Execute_SequentialExecution(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "execute-sequential",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-002"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-003"},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      10,
		MaxIterations: 3,
		ProjectRoot:   tmpDir,
	}

	reviewCount := 0
	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(ctx context.Context, rc runner.RunConfig) (runner.ReviewResult, error) {
			reviewCount++
			return cleanReviewFn(ctx, rc)
		},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Task 6.1: Verify 3 sessions executed
	if reviewCount != 3 {
		t.Errorf("reviewCount = %d, want 3", reviewCount)
	}

	// Task 6.4: Call counts
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1", mock.HealthCheckCount)
	}
	wantHeadCommit := 3 * 2 // 2 per iteration (before + after)
	if mock.HeadCommitCount != wantHeadCommit {
		t.Errorf("HeadCommitCount = %d, want %d", mock.HeadCommitCount, wantHeadCommit)
	}

	// Task 6.5: Verify fresh sessions (no --resume in any invocation)
	for i := 0; i < 3; i++ {
		args := testutil.ReadInvocationArgs(t, stateDir, i)
		assertArgsFlagAbsent(t, args, "--resume")
		assertArgsContainFlag(t, args, "-p")
	}
}

// TestRunner_Execute_AllTasksDone verifies immediate successful return when no open tasks (AC3).
func TestRunner_Execute_AllTasksDone(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, allDoneTasks)

	mock := &testutil.MockGitClient{}
	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		MaxIterations: 10,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  fatalReviewFn(t),
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil (all done), got: %v", err)
	}

	// HealthCheck at startup, but no iterations → no HeadCommit
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1", mock.HealthCheckCount)
	}
	if mock.HeadCommitCount != 0 {
		t.Errorf("HeadCommitCount = %d, want 0 (no iterations)", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_ErrNoTasks verifies error when tasks file has no markers (AC3 variant).
func TestRunner_Execute_ErrNoTasks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, noMarkersTasks)

	mock := &testutil.MockGitClient{}
	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		MaxIterations: 10,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  fatalReviewFn(t),
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, config.ErrNoTasks) {
		t.Errorf("errors.Is(err, ErrNoTasks): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "runner: scan tasks:") {
		t.Errorf("error prefix: want 'runner: scan tasks:', got %q", err.Error())
	}
}

// =============================================================================
// Task 7: Health check and error paths (AC: #4)
// =============================================================================

// TestRunner_Execute_HealthCheckErrors verifies health check failures stop the loop (AC4).
func TestRunner_Execute_HealthCheckErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		healthErr            error
		wantErrContains      string
		wantErrContainsInner string
		wantErrIs            error
		wantHealthCheckCount int
		wantHeadCommitCount  int
	}{
		{
			name:                 "dirty tree",
			healthErr:            runner.ErrDirtyTree,
			wantErrContains:      "runner: health check:",
			wantErrIs:            runner.ErrDirtyTree,
			wantHealthCheckCount: 1,
			wantHeadCommitCount:  0,
		},
		{
			name:                 "detached HEAD",
			healthErr:            runner.ErrDetachedHead,
			wantErrContains:      "runner: health check:",
			wantErrIs:            runner.ErrDetachedHead,
			wantHealthCheckCount: 1,
			wantHeadCommitCount:  0,
		},
		{
			name:                 "generic git error",
			healthErr:            errors.New("git not found"),
			wantErrContains:      "runner: health check:",
			wantErrContainsInner: "git not found",
			wantHealthCheckCount: 1,
			wantHeadCommitCount:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

			mock := &testutil.MockGitClient{
				HealthCheckErrors: []error{tc.healthErr},
			}

			cfg := &config.Config{
				ClaudeCommand: "unused",
				MaxTurns:      5,
				MaxIterations: 10,
				ProjectRoot:   tmpDir,
			}

			r := &runner.Runner{
				Cfg:       cfg,
				Git:       mock,
				TasksFile: tasksPath,
				ReviewFn:  fatalReviewFn(t),
			}

			err := r.Execute(context.Background())
			if err == nil {
				t.Fatal("Execute: want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error: want containing %q, got %q", tc.wantErrContains, err.Error())
			}
			if tc.wantErrContainsInner != "" && !strings.Contains(err.Error(), tc.wantErrContainsInner) {
				t.Errorf("inner error: want containing %q, got %q", tc.wantErrContainsInner, err.Error())
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("errors.Is(err, %v): want true, got false; err = %v", tc.wantErrIs, err)
			}
			// Task 7.3: HealthCheck called before any HeadCommit/session.Execute
			if mock.HealthCheckCount != tc.wantHealthCheckCount {
				t.Errorf("HealthCheckCount = %d, want %d", mock.HealthCheckCount, tc.wantHealthCheckCount)
			}
			if mock.HeadCommitCount != tc.wantHeadCommitCount {
				t.Errorf("HeadCommitCount = %d, want %d (health check fails before any HeadCommit)", mock.HeadCommitCount, tc.wantHeadCommitCount)
			}
		})
	}
}

// =============================================================================
// Task 8: Session.Options values (AC: #5)
// =============================================================================

// TestRunner_Execute_SessionOptions verifies --max-turns and --model values (AC5).
func TestRunner_Execute_SessionOptions(t *testing.T) {
	tests := []struct {
		name       string
		maxTurns   int
		model      string
		wantModel  bool
		modelValue string
	}{
		{
			name:       "max-turns and model set",
			maxTurns:   25,
			model:      "sonnet",
			wantModel:  true,
			modelValue: "sonnet",
		},
		{
			name:      "model empty omits flag",
			maxTurns:  10,
			model:     "",
			wantModel: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

			scenario := testutil.Scenario{
				Name: "session-options-" + tc.name,
				Steps: []testutil.ScenarioStep{
					{Type: "execute", ExitCode: 0, SessionID: "opts-001"},
				},
			}
			_, stateDir := testutil.SetupMockClaude(t, scenario)

			mock := &testutil.MockGitClient{
				HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
			}

			cfg := &config.Config{
				ClaudeCommand: os.Args[0],
				MaxTurns:      tc.maxTurns,
				MaxIterations: 1,
				ModelExecute:  tc.model,
				ProjectRoot:   tmpDir,
			}

			r := &runner.Runner{
				Cfg:       cfg,
				Git:       mock,
				TasksFile: tasksPath,
				ReviewFn:  cleanReviewFn,
			}

			err := r.Execute(context.Background())
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v", err)
			}

			args := testutil.ReadInvocationArgs(t, stateDir, 0)

			// AC5: --max-turns value
			assertArgsContainFlagValue(t, args, "--max-turns", fmt.Sprintf("%d", tc.maxTurns))

			// AC5: --model presence/absence
			if tc.wantModel {
				assertArgsContainFlagValue(t, args, "--model", tc.modelValue)
			} else {
				assertArgsFlagAbsent(t, args, "--model")
			}
		})
	}
}

// =============================================================================
// Task 9: Review stub tests (AC: #6)
// =============================================================================

// TestReviewResult_ZeroValue verifies ReviewResult zero value behavior.
func TestReviewResult_ZeroValue(t *testing.T) {
	t.Parallel()
	rr := runner.ReviewResult{}
	if rr.Clean {
		t.Errorf("ReviewResult{}.Clean = true, want false (zero value)")
	}
	rr = runner.ReviewResult{Clean: true}
	if !rr.Clean {
		t.Errorf("ReviewResult{Clean: true}.Clean = false")
	}
}

// TestRunner_Execute_CustomReviewFunc verifies custom ReviewFunc is called (AC6).
func TestRunner_Execute_CustomReviewFunc(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "custom-review",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "review-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		MaxIterations: 1,
		ProjectRoot:   tmpDir,
	}

	customCalled := false
	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			customCalled = true
			return runner.ReviewResult{Clean: true}, nil
		},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if !customCalled {
		t.Error("custom ReviewFn was not called")
	}
}

// TestRunner_Execute_ReviewFuncSequence verifies configurable sequence (findings N then clean) (AC6).
// NOTE: Execute currently discards ReviewResult.Clean (Story 3.10 adds review_cycles counter).
// This test verifies the ReviewFn CAN return varied sequences and is called the correct number of times.
func TestRunner_Execute_ReviewFuncSequence(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "review-sequence",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "seq-001"},
			{Type: "execute", ExitCode: 0, SessionID: "seq-002"},
			{Type: "execute", ExitCode: 0, SessionID: "seq-003"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		MaxIterations: 3,
		ProjectRoot:   tmpDir,
	}

	// Configurable sequence: findings 2 times, then clean
	callCount := 0
	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			callCount++
			if callCount <= 2 {
				return runner.ReviewResult{Clean: false}, nil
			}
			return runner.ReviewResult{Clean: true}, nil
		},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("review callCount = %d, want 3", callCount)
	}
}

// TestRunner_Execute_ReviewFuncError verifies review error propagation (AC6).
func TestRunner_Execute_ReviewFuncError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "review-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "err-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		MaxIterations: 1,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			return runner.ReviewResult{}, errors.New("review crashed")
		},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: review:") {
		t.Errorf("error prefix: want 'runner: review:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "review crashed") {
		t.Errorf("inner error: want 'review crashed', got %q", err.Error())
	}
}

// =============================================================================
// Task 10: Mutation asymmetry (AC: #7)
// =============================================================================

// TestRunner_Execute_MutationAsymmetry verifies runner never writes to tasks file (AC7).
func TestRunner_Execute_MutationAsymmetry(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "mutation-check",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "mut-001"},
			{Type: "execute", ExitCode: 0, SessionID: "mut-002"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
		),
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		MaxIterations: 2,
		ProjectRoot:   tmpDir,
	}

	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  cleanReviewFn,
	}

	err = r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}

	if !bytes.Equal(before, after) {
		t.Errorf("tasks file was modified by runner — mutation asymmetry violated\nbefore: %q\nafter:  %q", before, after)
	}
}

// =============================================================================
// Additional error path tests
// =============================================================================

// TestRunner_Execute_NoCommitDetected verifies error when HEAD unchanged (AC2 negative).
func TestRunner_Execute_NoCommitDetected(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "no-commit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nc-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// Same SHA before and after — no commit
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa"},
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		MaxIterations: 1,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  fatalReviewFn(t),
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, runner.ErrNoCommit) {
		t.Errorf("errors.Is(err, ErrNoCommit): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "runner:") {
		t.Errorf("error: want 'runner: no commit detected', got %q", err.Error())
	}
}

// TestRunner_Execute_HeadCommitBeforeFails verifies error on pre-execute HeadCommit failure.
func TestRunner_Execute_HeadCommitBeforeFails(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	mock := &testutil.MockGitClient{
		HeadCommitErrors: []error{errors.New("git broken")},
	}

	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		MaxIterations: 1,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  fatalReviewFn(t),
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: head commit before:") {
		t.Errorf("error prefix: want 'runner: head commit before:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "git broken") {
		t.Errorf("inner error: want 'git broken', got %q", err.Error())
	}
}

// TestRunner_Execute_HeadCommitAfterFails verifies error on post-execute HeadCommit failure.
// Exercises runner.go:120-123 — distinct path from HeadCommitBeforeFails (runner.go:103-106).
func TestRunner_Execute_HeadCommitAfterFails(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "head-commit-after-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "hca-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits:      []string{"aaa"},
		HeadCommitErrors: []error{nil, errors.New("git broken after")},
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		MaxTurns:      5,
		MaxIterations: 1,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  fatalReviewFn(t),
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: head commit after:") {
		t.Errorf("error prefix: want 'runner: head commit after:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "git broken after") {
		t.Errorf("inner error: want 'git broken after', got %q", err.Error())
	}
}

// TestRunner_Execute_ReadTasksFails verifies error when tasks file is missing.
func TestRunner_Execute_ReadTasksFails(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	mock := &testutil.MockGitClient{}
	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		MaxIterations: 1,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: filepath.Join(tmpDir, "nonexistent.md"),
		ReviewFn:  fatalReviewFn(t),
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: read tasks:") {
		t.Errorf("error prefix: want 'runner: read tasks:', got %q", err.Error())
	}
}
