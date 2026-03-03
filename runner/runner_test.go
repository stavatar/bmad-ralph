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
	"time"

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
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
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
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil (all done), got: %v", err)
	}

	// RecoverDirtyState at startup (calls HealthCheck internally), no iterations → no HeadCommit
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
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
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
// Story 3.8: Startup errors (AC3 — dirty recovery failure + non-dirty health errors)
// =============================================================================

// TestRunner_Execute_StartupErrors verifies startup failures stop the loop.
// RecoverDirtyState wraps HealthCheck errors with "runner: dirty state recovery:" prefix,
// Execute wraps with "runner: startup:" — multi-layer error chain.
func TestRunner_Execute_StartupErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                        string
		healthErr                   error
		restoreErr                  error
		wantErrContains             string
		wantErrContainsIntermediate string
		wantErrContainsInner        string
		wantErrIs                   error
		wantHealthCheckCount        int
		wantHeadCommitCount         int
		wantRestoreCount            int
	}{
		{
			name:                        "detached HEAD",
			healthErr:                   runner.ErrDetachedHead,
			wantErrContains:             "runner: startup:",
			wantErrContainsIntermediate: "runner: dirty state recovery:",
			wantErrIs:                   runner.ErrDetachedHead,
			wantHealthCheckCount:        1,
			wantHeadCommitCount:         0,
			wantRestoreCount:            0,
		},
		{
			name:                        "generic git error",
			healthErr:                   errors.New("git not found"),
			wantErrContains:             "runner: startup:",
			wantErrContainsIntermediate: "runner: dirty state recovery:",
			wantErrContainsInner:        "git not found",
			wantHealthCheckCount:        1,
			wantHeadCommitCount:         0,
			wantRestoreCount:            0,
		},
		{
			name:                        "dirty tree restore fails",
			healthErr:                   runner.ErrDirtyTree,
			restoreErr:                  errors.New("restore failed"),
			wantErrContains:             "runner: startup:",
			wantErrContainsIntermediate: "runner: dirty state recovery:",
			wantErrContainsInner:        "restore failed",
			wantHealthCheckCount:        1,
			wantHeadCommitCount:         0,
			wantRestoreCount:            1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

			mock := &testutil.MockGitClient{
				HealthCheckErrors: []error{tc.healthErr},
				RestoreCleanError: tc.restoreErr,
			}

			cfg := &config.Config{
				ClaudeCommand: "unused",
				MaxTurns:      5,
				MaxIterations: 10,
				ProjectRoot:   tmpDir,
			}

			r := &runner.Runner{
				Cfg:             cfg,
				Git:             mock,
				TasksFile:       tasksPath,
				ReviewFn:        fatalReviewFn(t),
				ResumeExtractFn: noopResumeExtractFn,
				SleepFn:         noopSleepFn,
				Knowledge:       &runner.NoOpKnowledgeWriter{},
			}

			err := r.Execute(context.Background())
			if err == nil {
				t.Fatal("Execute: want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error: want containing %q, got %q", tc.wantErrContains, err.Error())
			}
			if tc.wantErrContainsIntermediate != "" && !strings.Contains(err.Error(), tc.wantErrContainsIntermediate) {
				t.Errorf("intermediate error: want containing %q, got %q", tc.wantErrContainsIntermediate, err.Error())
			}
			if tc.wantErrContainsInner != "" && !strings.Contains(err.Error(), tc.wantErrContainsInner) {
				t.Errorf("inner error: want containing %q, got %q", tc.wantErrContainsInner, err.Error())
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("errors.Is(err, %v): want true, got false; err = %v", tc.wantErrIs, err)
			}
			if mock.HealthCheckCount != tc.wantHealthCheckCount {
				t.Errorf("HealthCheckCount = %d, want %d", mock.HealthCheckCount, tc.wantHealthCheckCount)
			}
			if mock.HeadCommitCount != tc.wantHeadCommitCount {
				t.Errorf("HeadCommitCount = %d, want %d (startup error before loop)", mock.HeadCommitCount, tc.wantHeadCommitCount)
			}
			if mock.RestoreCleanCount != tc.wantRestoreCount {
				t.Errorf("RestoreCleanCount = %d, want %d", mock.RestoreCleanCount, tc.wantRestoreCount)
			}
		})
	}
}

// TestRunner_Execute_DirtyTreeRecoveryAtStartup verifies that a dirty working
// tree at startup is recovered (not errored), and execution proceeds to scan tasks.
// Covers AC3 (dirty recovery) + AC2 (all done) intersection.
func TestRunner_Execute_DirtyTreeRecoveryAtStartup(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, allDoneTasks)

	mock := &testutil.MockGitClient{
		HealthCheckErrors: []error{runner.ErrDirtyTree},
	}

	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		MaxIterations: 10,
		ProjectRoot:   tmpDir,
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil (recovery + all done), got: %v", err)
	}

	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1 (RecoverDirtyState calls HealthCheck once)", mock.HealthCheckCount)
	}
	if mock.RestoreCleanCount != 1 {
		t.Errorf("RestoreCleanCount = %d, want 1 (recovery performed)", mock.RestoreCleanCount)
	}
	if mock.HeadCommitCount != 0 {
		t.Errorf("HeadCommitCount = %d, want 0 (no open tasks, no session executed)", mock.HeadCommitCount)
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
				Cfg:             cfg,
				Git:             mock,
				TasksFile:       tasksPath,
				ReviewFn:        cleanReviewFn,
				ResumeExtractFn: noopResumeExtractFn,
				SleepFn:         noopSleepFn,
				Knowledge:       &runner.NoOpKnowledgeWriter{},
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

	cfg := testConfig(tmpDir, 1)

	customCalled := false
	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			customCalled = true
			return runner.ReviewResult{Clean: true}, nil
		},
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if !customCalled {
		t.Error("custom ReviewFn was not called")
	}
}

// TestRunner_Execute_ReviewFuncSequence verifies review cycle loop: findings 2 times then clean (AC1, AC4).
// Review cycle loop: non-clean increments reviewCycles, clean breaks loop and task completes.
// 1 task × 3 review cycles = 3 execute steps (each produces commit).
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

	cfg := testConfig(tmpDir, 1)
	cfg.MaxReviewIterations = 5 // won't hit max

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
			// Clean review — maxIterations=1 exits outer loop after this task
			return runner.ReviewResult{Clean: true}, nil
		},
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("review callCount = %d, want 3 (2 non-clean + 1 clean)", callCount)
	}
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1 (startup only)", mock.HealthCheckCount)
	}
	if mock.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6 (3 pairs)", mock.HeadCommitCount)
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

	cfg := testConfig(tmpDir, 1)

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			return runner.ReviewResult{}, errors.New("review crashed")
		},
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
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

// TestRunner_Execute_MaxReviewCyclesExhausted verifies ErrMaxReviewCycles with informative message
// when all reviews return non-clean. Table-driven to prove configurable max (AC2, AC5).
func TestRunner_Execute_MaxReviewCyclesExhausted(t *testing.T) {
	tests := []struct {
		name                 string
		maxReviewIter        int
		wantReviewCount      int
		wantResumeCount      int
		wantSleepCount       int
		wantHeadCommitCount  int
		wantHealthCheckCount int
		wantCountFormat      string
	}{
		{
			name:                 "max 3 default",
			maxReviewIter:        3,
			wantReviewCount:      3,
			wantResumeCount:      0,
			wantSleepCount:       0,
			wantHeadCommitCount:  6,
			wantHealthCheckCount: 1, // startup only
			wantCountFormat:      "3/3",
		},
		{
			name:                 "max 5 configurable",
			maxReviewIter:        5,
			wantReviewCount:      5,
			wantResumeCount:      0,
			wantSleepCount:       0,
			wantHeadCommitCount:  10,
			wantHealthCheckCount: 1, // startup only
			wantCountFormat:      "5/5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

			steps := make([]testutil.ScenarioStep, tt.maxReviewIter)
			for i := range steps {
				steps[i] = testutil.ScenarioStep{Type: "execute", ExitCode: 0, SessionID: fmt.Sprintf("rev-%03d", i+1)}
			}
			scenario := testutil.Scenario{Name: "max-review-cycles", Steps: steps}
			testutil.SetupMockClaude(t, scenario)

			// Each execute produces a commit (different SHAs)
			pairs := make([][2]string, tt.maxReviewIter)
			for i := range pairs {
				pairs[i] = [2]string{fmt.Sprintf("%03d", i), fmt.Sprintf("%03d", i+1)}
			}
			mock := &testutil.MockGitClient{HeadCommits: headCommitPairs(pairs...)}

			cfg := testConfig(tmpDir, 1) // single task
			cfg.MaxReviewIterations = tt.maxReviewIter

			re := &trackingResumeExtract{}
			ts := &trackingSleep{}
			reviewCount := 0

			r := &runner.Runner{
				Cfg:       cfg,
				Git:       mock,
				TasksFile: tasksPath,
				ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
					reviewCount++
					return runner.ReviewResult{Clean: false}, nil // always non-clean
				},
				ResumeExtractFn: re.fn,
				SleepFn:         ts.fn,
				Knowledge:       &runner.NoOpKnowledgeWriter{},
			}

			err := r.Execute(context.Background())
			if err == nil {
				t.Fatal("Execute: want error, got nil")
			}
			if reviewCount != tt.wantReviewCount {
				t.Errorf("reviewCount = %d, want %d", reviewCount, tt.wantReviewCount)
			}
			if !errors.Is(err, config.ErrMaxReviewCycles) {
				t.Errorf("errors.Is(err, ErrMaxReviewCycles): want true, got false; err = %v", err)
			}
			if !strings.Contains(err.Error(), "review cycles exhausted") {
				t.Errorf("error message: want 'review cycles exhausted', got %q", err.Error())
			}
			if !strings.Contains(err.Error(), "Task one") {
				t.Errorf("error message: want 'Task one' task name, got %q", err.Error())
			}
			if !strings.Contains(err.Error(), "check logs") {
				t.Errorf("error message: want 'check logs' suggestion, got %q", err.Error())
			}
			if !strings.Contains(err.Error(), tt.wantCountFormat) {
				t.Errorf("error message: want %q count format, got %q", tt.wantCountFormat, err.Error())
			}
			if !strings.Contains(err.Error(), "max review cycles exceeded") {
				t.Errorf("inner error: want 'max review cycles exceeded' sentinel text, got %q", err.Error())
			}
			if re.count != tt.wantResumeCount {
				t.Errorf("ResumeExtractFn count = %d, want %d", re.count, tt.wantResumeCount)
			}
			if ts.count != tt.wantSleepCount {
				t.Errorf("SleepFn count = %d, want %d", ts.count, tt.wantSleepCount)
			}
			if mock.HeadCommitCount != tt.wantHeadCommitCount {
				t.Errorf("HeadCommitCount = %d, want %d", mock.HeadCommitCount, tt.wantHeadCommitCount)
			}
			if mock.HealthCheckCount != tt.wantHealthCheckCount {
				t.Errorf("HealthCheckCount = %d, want %d", mock.HealthCheckCount, tt.wantHealthCheckCount)
			}
		})
	}
}

// TestRunner_Execute_ReviewCyclesPerTask verifies review_cycles counter is per-task (AC3).
// If counter persisted across tasks, task B's first non-clean would hit max (2 >= 2) → error.
// Test proves NO error: counter resets when task A completes with clean review.
func TestRunner_Execute_ReviewCyclesPerTask(t *testing.T) {
	twoOpenTasks := "# Sprint Tasks\n\n- [ ] Task one\n- [ ] Task two\n"
	taskOneDone := "# Sprint Tasks\n\n- [x] Task one\n- [ ] Task two\n"

	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, twoOpenTasks)

	// 4 execute steps: 2 per task (each produces commit)
	scenario := testutil.Scenario{
		Name: "review-per-task",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "pt-001"},
			{Type: "execute", ExitCode: 0, SessionID: "pt-002"},
			{Type: "execute", ExitCode: 0, SessionID: "pt-003"},
			{Type: "execute", ExitCode: 0, SessionID: "pt-004"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
			[2]string{"ddd", "eee"},
		),
	}

	cfg := testConfig(tmpDir, 3) // generous for 2 tasks
	cfg.MaxReviewIterations = 2  // low threshold — proves per-task reset

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}
	reviewCount := 0

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			reviewCount++
			switch reviewCount {
			case 1: // task A, review 1: non-clean
				return runner.ReviewResult{Clean: false}, nil
			case 2: // task A, review 2: clean → task A done
				os.WriteFile(tasksPath, []byte(taskOneDone), 0644)
				return runner.ReviewResult{Clean: true}, nil
			case 3: // task B, review 1: non-clean
				return runner.ReviewResult{Clean: false}, nil
			default: // task B, review 2: clean → all done
				os.WriteFile(tasksPath, []byte(allDoneTasks), 0644)
				return runner.ReviewResult{Clean: true}, nil
			}
		},
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil (per-task counter reset), got: %v", err)
	}
	if reviewCount != 4 {
		t.Errorf("reviewCount = %d, want 4 (2 per task)", reviewCount)
	}
	if mock.HeadCommitCount != 8 {
		t.Errorf("HeadCommitCount = %d, want 8 (4 pairs)", mock.HeadCommitCount)
	}
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1 (startup only)", mock.HealthCheckCount)
	}
	if re.count != 0 {
		t.Errorf("ResumeExtractFn count = %d, want 0 (no retries)", re.count)
	}
	if ts.count != 0 {
		t.Errorf("SleepFn count = %d, want 0 (no backoff)", ts.count)
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

	cfg := testConfig(tmpDir, 2)

	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        cleanReviewFn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
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

// TestRunner_Execute_NoCommitDetected verifies that no-commit with MaxIterations=1
// exhausts retries immediately and returns ErrMaxRetries with informative message.
// At MaxIterations=1, emergency stop fires before resume extract or backoff sleep.
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

	cfg := testConfig(tmpDir, 1)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
	}

	// Message assertions (AC2: informative stop message)
	if !strings.Contains(err.Error(), "execute attempts exhausted") {
		t.Errorf("error message: want 'execute attempts exhausted', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "1/1") {
		t.Errorf("error message: want '1/1' count format, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Task one") {
		t.Errorf("error message: want 'Task one' task name, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "check logs") {
		t.Errorf("error message: want 'check logs' suggestion, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("inner error: want 'max retries exceeded' sentinel text, got %q", err.Error())
	}

	// Tracking assertions: emergency stop fires before resume extract at MaxIterations=1
	if re.count != 0 {
		t.Errorf("ResumeExtractFn count = %d, want 0 (emergency stop before RE)", re.count)
	}
	if ts.count != 0 {
		t.Errorf("SleepFn count = %d, want 0 (no backoff sleep)", ts.count)
	}
	if mock.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2 (before+after)", mock.HeadCommitCount)
	}
	// 1 startup HealthCheck only
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1 (startup only)", mock.HealthCheckCount)
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
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
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

	cfg := testConfig(tmpDir, 1)

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
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
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       filepath.Join(tmpDir, "nonexistent.md"),
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: read tasks:") {
		t.Errorf("error prefix: want 'runner: read tasks:', got %q", err.Error())
	}
}

// =============================================================================
// Story 3.6: Retry logic tests (AC1-AC6, NFR12)
// =============================================================================

// TestRunner_Execute_RetryOnNoCommit verifies that no-commit triggers retry
// with resume-extraction and backoff, then commit triggers review (AC1, AC3).
func TestRunner_Execute_RetryOnNoCommit(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "retry-no-commit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "retry-001"},
			{Type: "execute", ExitCode: 0, SessionID: "retry-002"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// Attempt 1: same HEAD (no commit). Attempt 2: different HEAD (commit).
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "bbb"},
	}

	cfg := testConfig(tmpDir, 3)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}
	reviewCount := 0

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewAndMarkDoneFn(tasksPath, &reviewCount),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC1: ResumeExtractFn called on no-commit
	if re.count != 1 {
		t.Fatalf("ResumeExtractFn count = %d, want 1", re.count)
	}
	if re.sessionIDs[0] != "retry-001" {
		t.Errorf("ResumeExtractFn sessionID = %q, want %q", re.sessionIDs[0], "retry-001")
	}

	// AC1: SleepFn called for backoff
	if ts.count != 1 {
		t.Errorf("SleepFn count = %d, want 1", ts.count)
	}

	// AC3: commit triggers review (counter resets implicitly)
	if reviewCount != 1 {
		t.Errorf("reviewCount = %d, want 1", reviewCount)
	}

	// Call count assertions
	if mock.HeadCommitCount != 4 {
		t.Errorf("HeadCommitCount = %d, want 4", mock.HeadCommitCount)
	}
	if mock.HealthCheckCount != 2 {
		t.Errorf("HealthCheckCount = %d, want 2 (initial + RecoverDirtyState)", mock.HealthCheckCount)
	}
}

// TestRunner_Execute_RetryCounterIncrements verifies counter increments on
// consecutive no-commit failures before success (AC2).
func TestRunner_Execute_RetryCounterIncrements(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "retry-counter",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "cnt-001"},
			{Type: "execute", ExitCode: 0, SessionID: "cnt-002"},
			{Type: "execute", ExitCode: 0, SessionID: "cnt-003"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// 2 no-commit attempts, then commit on 3rd
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "bbb"},
	}

	cfg := testConfig(tmpDir, 5)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}
	reviewCount := 0

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewAndMarkDoneFn(tasksPath, &reviewCount),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC2: counter incremented twice (2 no-commit failures)
	if re.count != 2 {
		t.Errorf("ResumeExtractFn count = %d, want 2", re.count)
	}
	if ts.count != 2 {
		t.Errorf("SleepFn count = %d, want 2", ts.count)
	}
	if reviewCount != 1 {
		t.Errorf("reviewCount = %d, want 1", reviewCount)
	}
	if mock.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6", mock.HeadCommitCount)
	}
	// 1 initial + 2 RecoverDirtyState
	if mock.HealthCheckCount != 3 {
		t.Errorf("HealthCheckCount = %d, want 3", mock.HealthCheckCount)
	}
}

// TestRunner_Execute_CounterPerTask verifies independent retry counters
// per task — task B starts fresh after task A retried (AC4).
func TestRunner_Execute_CounterPerTask(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "counter-per-task",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "cpt-001"}, // task A attempt 1 (no commit)
			{Type: "execute", ExitCode: 0, SessionID: "cpt-002"}, // task A attempt 2 (commit)
			{Type: "execute", ExitCode: 0, SessionID: "cpt-003"}, // task B attempt 1 (commit)
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// Task A: fail once [aaa,aaa], succeed [aaa,bbb]. Task B: succeed [bbb,ccc].
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "bbb", "bbb", "ccc"},
	}

	cfg := testConfig(tmpDir, 3)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}
	reviewCount := 0

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			reviewCount++
			if reviewCount >= 2 {
				os.WriteFile(tasksPath, []byte(allDoneTasks), 0644)
			}
			return runner.ReviewResult{Clean: true}, nil
		},
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC4: only task A retried (1 resume extract), task B succeeded first try
	if re.count != 1 {
		t.Errorf("ResumeExtractFn count = %d, want 1 (only task A retry)", re.count)
	}
	if ts.count != 1 {
		t.Errorf("SleepFn count = %d, want 1", ts.count)
	}
	if reviewCount != 2 {
		t.Errorf("reviewCount = %d, want 2 (one per task)", reviewCount)
	}
	if mock.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6", mock.HeadCommitCount)
	}
	// 1 initial + 1 RecoverDirtyState (task A retry only)
	if mock.HealthCheckCount != 2 {
		t.Errorf("HealthCheckCount = %d, want 2", mock.HealthCheckCount)
	}
}

// TestRunner_Execute_MaxRetriesExhausted verifies ErrMaxRetries with informative message
// when all attempts produce no-commit. Table-driven to prove configurable max (AC3).
func TestRunner_Execute_MaxRetriesExhausted(t *testing.T) {
	tests := []struct {
		name                 string
		maxIter              int
		wantResumeCount      int
		wantSleepCount       int
		wantHeadCommitCount  int
		wantHealthCheckCount int
		wantCountFormat      string
	}{
		{
			name:                 "max 3 default",
			maxIter:              3,
			wantResumeCount:      2,
			wantSleepCount:       2,
			wantHeadCommitCount:  6,
			wantHealthCheckCount: 3, // 1 startup + 2 retry RecoverDirtyState
			wantCountFormat:      "3/3",
		},
		{
			// AC3: proves stop does NOT trigger at 3 or 4 (4 resume extracts = loop ran through attempts 1-4)
			name:                 "max 5 configurable",
			maxIter:              5,
			wantResumeCount:      4,
			wantSleepCount:       4,
			wantHeadCommitCount:  10,
			wantHealthCheckCount: 5, // 1 startup + 4 retry RecoverDirtyState
			wantCountFormat:      "5/5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

			steps := make([]testutil.ScenarioStep, tt.maxIter)
			for i := range steps {
				steps[i] = testutil.ScenarioStep{Type: "execute", ExitCode: 0, SessionID: fmt.Sprintf("max-%03d", i+1)}
			}
			scenario := testutil.Scenario{Name: "max-retries", Steps: steps}
			testutil.SetupMockClaude(t, scenario)

			// 2 HeadCommits (before+after) per attempt, all same SHA (no commit)
			headCommits := make([]string, tt.maxIter*2)
			for i := range headCommits {
				headCommits[i] = "aaa"
			}
			mock := &testutil.MockGitClient{HeadCommits: headCommits}

			cfg := testConfig(tmpDir, tt.maxIter)

			re := &trackingResumeExtract{}
			ts := &trackingSleep{}

			r := &runner.Runner{
				Cfg:             cfg,
				Git:             mock,
				TasksFile:       tasksPath,
				ReviewFn:        fatalReviewFn(t),
				ResumeExtractFn: re.fn,
				SleepFn:         ts.fn,
				Knowledge:       &runner.NoOpKnowledgeWriter{},
			}

			err := r.Execute(context.Background())
			if err == nil {
				t.Fatal("Execute: want error, got nil")
			}
			if !errors.Is(err, config.ErrMaxRetries) {
				t.Errorf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
			}
			if !strings.Contains(err.Error(), "execute attempts exhausted") {
				t.Errorf("error message: want 'execute attempts exhausted', got %q", err.Error())
			}
			if !strings.Contains(err.Error(), "Task one") {
				t.Errorf("error message: want 'Task one' task name, got %q", err.Error())
			}
			if !strings.Contains(err.Error(), "check logs") {
				t.Errorf("error message: want 'check logs' suggestion, got %q", err.Error())
			}
			if !strings.Contains(err.Error(), tt.wantCountFormat) {
				t.Errorf("error message: want %q count format, got %q", tt.wantCountFormat, err.Error())
			}
			if !strings.Contains(err.Error(), "max retries exceeded") {
				t.Errorf("inner error: want 'max retries exceeded' sentinel text, got %q", err.Error())
			}
			if re.count != tt.wantResumeCount {
				t.Errorf("ResumeExtractFn count = %d, want %d", re.count, tt.wantResumeCount)
			}
			if ts.count != tt.wantSleepCount {
				t.Errorf("SleepFn count = %d, want %d", ts.count, tt.wantSleepCount)
			}
			if mock.HeadCommitCount != tt.wantHeadCommitCount {
				t.Errorf("HeadCommitCount = %d, want %d", mock.HeadCommitCount, tt.wantHeadCommitCount)
			}
			if mock.HealthCheckCount != tt.wantHealthCheckCount {
				t.Errorf("HealthCheckCount = %d, want %d", mock.HealthCheckCount, tt.wantHealthCheckCount)
			}
		})
	}
}

// TestRunner_Execute_NonZeroExitRetry verifies non-zero exit code triggers
// retry, not immediate failure (AC6).
func TestRunner_Execute_NonZeroExitRetry(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "exit-error-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "exit-001"}, // non-zero exit
			{Type: "execute", ExitCode: 0, SessionID: "exit-002"}, // success
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// Attempt 1: headBefore only (exit error, no headAfter).
	// Attempt 2: headBefore + headAfter (different = commit).
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "bbb"},
	}

	cfg := testConfig(tmpDir, 3)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}
	reviewCount := 0

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewAndMarkDoneFn(tasksPath, &reviewCount),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC6: exit error triggered retry, then success
	if re.count != 1 {
		t.Fatalf("ResumeExtractFn count = %d, want 1", re.count)
	}
	// SessionID parsed from exit error response
	if re.sessionIDs[0] != "exit-001" {
		t.Errorf("ResumeExtractFn sessionID = %q, want %q", re.sessionIDs[0], "exit-001")
	}
	if ts.count != 1 {
		t.Errorf("SleepFn count = %d, want 1", ts.count)
	}
	if reviewCount != 1 {
		t.Errorf("reviewCount = %d, want 1", reviewCount)
	}
	// headBefore×2 + headAfter×1 (no headAfter on exit error attempt)
	if mock.HeadCommitCount != 3 {
		t.Errorf("HeadCommitCount = %d, want 3", mock.HeadCommitCount)
	}
	if mock.HealthCheckCount != 2 {
		t.Errorf("HealthCheckCount = %d, want 2", mock.HealthCheckCount)
	}
}

// TestRunner_Execute_FatalExecErrorNoRetry verifies non-exit errors
// (binary not found) cause immediate return without retry.
func TestRunner_Execute_FatalExecErrorNoRetry(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa"},
	}

	cfg := &config.Config{
		ClaudeCommand: "/nonexistent/binary",
		MaxTurns:      5,
		MaxIterations: 3,
		ProjectRoot:   tmpDir,
	}

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: execute:") {
		t.Errorf("error prefix: want 'runner: execute:', got %q", err.Error())
	}

	// No retry attempted
	if re.count != 0 {
		t.Errorf("ResumeExtractFn count = %d, want 0 (fatal, no retry)", re.count)
	}
	if ts.count != 0 {
		t.Errorf("SleepFn count = %d, want 0", ts.count)
	}
	// Only headBefore called, then execute fails
	if mock.HeadCommitCount != 1 {
		t.Errorf("HeadCommitCount = %d, want 1", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_BackoffTiming verifies exponential backoff durations
// on consecutive retries: 1s, 2s, 4s (NFR12).
func TestRunner_Execute_BackoffTiming(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "backoff-timing",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bo-001"},
			{Type: "execute", ExitCode: 0, SessionID: "bo-002"},
			{Type: "execute", ExitCode: 0, SessionID: "bo-003"},
			{Type: "execute", ExitCode: 0, SessionID: "bo-004"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// 3 no-commit attempts, then commit on 4th
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "aaa", "aaa", "bbb"},
	}

	cfg := testConfig(tmpDir, 5)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewAndMarkDoneFn(tasksPath, nil),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// NFR12: exponential backoff 1s, 2s, 4s
	if ts.count != 3 {
		t.Fatalf("SleepFn count = %d, want 3", ts.count)
	}
	wantDurations := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for i, want := range wantDurations {
		if ts.durations[i] != want {
			t.Errorf("SleepFn duration[%d] = %v, want %v", i, ts.durations[i], want)
		}
	}

	if re.count != 3 {
		t.Errorf("ResumeExtractFn count = %d, want 3", re.count)
	}
	if mock.HeadCommitCount != 8 {
		t.Errorf("HeadCommitCount = %d, want 8", mock.HeadCommitCount)
	}
	// 1 initial + 3 RecoverDirtyState
	if mock.HealthCheckCount != 4 {
		t.Errorf("HealthCheckCount = %d, want 4", mock.HealthCheckCount)
	}
}

// TestRunner_Execute_ContextCancelDuringRetry verifies that context
// cancellation during retry produces context error.
// Both context.Canceled and context.DeadlineExceeded flow through the same
// ctx.Err() check — testing Canceled via cancel() in ResumeExtractFn.
func TestRunner_Execute_ContextCancelDuringRetry(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "ctx-cancel-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "ctx-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa"},
	}

	cfg := testConfig(tmpDir, 3)

	ctx, cancel := context.WithCancel(context.Background())
	ts := &trackingSleep{}
	reCount := 0

	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn:  fatalReviewFn(t),
		ResumeExtractFn: func(_ context.Context, _ runner.RunConfig, _ string) error {
			reCount++
			cancel() // cancel context before backoff sleep
			return nil
		},
		SleepFn:   ts.fn,
		Knowledge: &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(ctx)
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "runner: retry:") {
		t.Errorf("error prefix: want 'runner: retry:', got %q", err.Error())
	}

	// Context cancelled after ResumeExtractFn, before sleep
	if reCount != 1 {
		t.Errorf("ResumeExtractFn count = %d, want 1", reCount)
	}
	if ts.count != 0 {
		t.Errorf("SleepFn count = %d, want 0 (context cancelled before sleep)", ts.count)
	}
	if mock.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_ResumeExtractFnError verifies error propagation
// when ResumeExtractFn fails during retry.
func TestRunner_Execute_ResumeExtractFnError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "resume-extract-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "re-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa"},
	}

	cfg := testConfig(tmpDir, 3)

	re := &trackingResumeExtract{err: errors.New("resume extraction failed")}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: retry: resume extract:") {
		t.Errorf("error prefix: want 'runner: retry: resume extract:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "resume extraction failed") {
		t.Errorf("inner error: want 'resume extraction failed', got %q", err.Error())
	}

	if re.count != 1 {
		t.Errorf("ResumeExtractFn count = %d, want 1", re.count)
	}
	if ts.count != 0 {
		t.Errorf("SleepFn count = %d, want 0 (error before sleep)", ts.count)
	}
	if mock.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_RecoverDirtyStateFails verifies error propagation
// when RecoverDirtyState fails during retry.
func TestRunner_Execute_RecoverDirtyStateFails(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "recover-dirty-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "rdf-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// HealthCheck: call 1 (initial) = nil, call 2 (RecoverDirtyState) = error
	mock := &testutil.MockGitClient{
		HealthCheckErrors: []error{nil, errors.New("git network error")},
		HeadCommits:       []string{"aaa", "aaa"},
	}

	cfg := testConfig(tmpDir, 3)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: retry: recover:") {
		t.Errorf("error prefix: want 'runner: retry: recover:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "runner: dirty state recovery:") {
		t.Errorf("intermediate error: want 'runner: dirty state recovery:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "git network error") {
		t.Errorf("inner error: want 'git network error', got %q", err.Error())
	}

	// ResumeExtractFn called before RecoverDirtyState
	if re.count != 1 {
		t.Errorf("ResumeExtractFn count = %d, want 1", re.count)
	}
	if ts.count != 0 {
		t.Errorf("SleepFn count = %d, want 0 (error before sleep)", ts.count)
	}
	if mock.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2", mock.HeadCommitCount)
	}
	// Initial + RecoverDirtyState
	if mock.HealthCheckCount != 2 {
		t.Errorf("HealthCheckCount = %d, want 2", mock.HealthCheckCount)
	}
}

// TestRunner_Execute_ExitErrorWithParseFailure verifies retry proceeds even when
// session.Execute returns exit error AND ParseResult fails (empty stdout).
// ResumeExtractFn receives empty sessionID "".
func TestRunner_Execute_ExitErrorWithParseFailure(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// MOCK_EXIT_EMPTY: test binary exits with code 1 and empty stdout
	t.Setenv("MOCK_EXIT_EMPTY", "1")

	// headBefore only per attempt (no headAfter on exit error)
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa"},
	}

	cfg := testConfig(tmpDir, 2)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	// Eventually exhausts retries
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
	}
	// Informative message present even on exit-error retry path
	if !strings.Contains(err.Error(), "execute attempts exhausted") {
		t.Errorf("error message: want 'execute attempts exhausted', got %q", err.Error())
	}

	// Retry still proceeded despite parse failure
	if re.count != 1 {
		t.Fatalf("ResumeExtractFn count = %d, want 1 (retry proceeded)", re.count)
	}
	// Empty sessionID because ParseResult failed on empty stdout
	if re.sessionIDs[0] != "" {
		t.Errorf("ResumeExtractFn sessionID = %q, want empty string", re.sessionIDs[0])
	}
	if ts.count != 1 {
		t.Errorf("SleepFn count = %d, want 1", ts.count)
	}
	if mock.HeadCommitCount != 2 {
		t.Errorf("HeadCommitCount = %d, want 2", mock.HeadCommitCount)
	}
}

// TestRunner_Execute_EmptySessionIDField verifies ResumeExtractFn receives ""
// when scenario produces empty SessionID field (ParseResult succeeds, field empty).
func TestRunner_Execute_EmptySessionIDField(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "empty-session-id",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: ""},
			{Type: "execute", ExitCode: 0, SessionID: ""},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// Both attempts: same HEAD (no commit)
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa"},
	}

	cfg := testConfig(tmpDir, 2)

	re := &trackingResumeExtract{}
	ts := &trackingSleep{}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: re.fn,
		SleepFn:         ts.fn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
	}

	if re.count != 1 {
		t.Fatalf("ResumeExtractFn count = %d, want 1", re.count)
	}
	// Empty sessionID propagated from scenario
	if re.sessionIDs[0] != "" {
		t.Errorf("ResumeExtractFn sessionID = %q, want empty string", re.sessionIDs[0])
	}
	if ts.count != 1 {
		t.Errorf("SleepFn count = %d, want 1", ts.count)
	}
	if mock.HeadCommitCount != 4 {
		t.Errorf("HeadCommitCount = %d, want 4", mock.HeadCommitCount)
	}
}

// =============================================================================
// Story 3.7: ResumeExtraction tests (AC1, AC3, AC6)
// =============================================================================

// TestResumeExtraction_Scenarios verifies ResumeExtraction behavior via table-driven tests.
func TestResumeExtraction_Scenarios(t *testing.T) {
	tests := []struct {
		name                     string
		sessionID                string
		scenarioSteps            []testutil.ScenarioStep
		knowledgeErr             error
		validateLessonsErr       error
		wantErr                  bool
		wantErrContains          string
		wantErrContainsInner     string
		wantWriteProgressCount   int
		wantValidateLessonsCount int
		wantSessionInvoked       bool
		checkArgs                func(t *testing.T, args []string)
		checkData                func(t *testing.T, kw *trackingKnowledgeWriter)
	}{
		{
			name:      "valid session ID with extraction prompt",
			sessionID: "abc-123",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "resumed-001"},
			},
			wantErr:                  false,
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 1,
			wantSessionInvoked:       true,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				assertArgsContainFlagValue(t, args, "--resume", "abc-123")
				assertArgsContainFlag(t, args, "-p")
				// Verify extraction prompt contains key instruction
				promptValue := argValueAfterFlag(args, "-p")
				if !strings.Contains(promptValue, "failure insights") {
					t.Errorf("-p value should contain 'failure insights', got %q", promptValue)
				}
				if !strings.Contains(promptValue, "LEARNINGS.md") {
					t.Errorf("-p value should contain 'LEARNINGS.md', got %q", promptValue)
				}
				if !strings.Contains(promptValue, "atomized facts") {
					t.Errorf("-p value should contain 'atomized facts', got %q", promptValue)
				}
				assertArgsContainFlagValue(t, args, "--max-turns", "5")
				assertArgsContainFlagValue(t, args, "--model", "opus")
			},
			checkData: func(t *testing.T, kw *trackingKnowledgeWriter) {
				t.Helper()
				if len(kw.writeProgressData) != 1 {
					t.Fatalf("writeProgressData len = %d, want 1", len(kw.writeProgressData))
				}
				if kw.writeProgressData[0].SessionID != "resumed-001" {
					t.Errorf("ProgressData.SessionID = %q, want %q", kw.writeProgressData[0].SessionID, "resumed-001")
				}
				// Verify ValidateNewLessons called with correct source
				if len(kw.validateLessonsData) != 1 {
					t.Fatalf("validateLessonsData len = %d, want 1", len(kw.validateLessonsData))
				}
				if kw.validateLessonsData[0].Source != "resume-extraction" {
					t.Errorf("LessonsData.Source = %q, want %q", kw.validateLessonsData[0].Source, "resume-extraction")
				}
			},
		},
		{
			name:                     "empty session ID",
			sessionID:                "",
			wantErr:                  false,
			wantWriteProgressCount:   0,
			wantValidateLessonsCount: 0,
			wantSessionInvoked:       false,
		},
		{
			name:      "session execute error",
			sessionID: "err-session",
			// No scenario steps: use nonexistent binary to trigger exec error
			wantErr:                  true,
			wantErrContains:          "runner: resume extraction: execute:",
			wantErrContainsInner:     "/nonexistent/binary",
			wantWriteProgressCount:   0,
			wantValidateLessonsCount: 0,
			wantSessionInvoked:       false, // binary not found, no invocation logged
		},
		{
			name:      "write progress error",
			sessionID: "wp-err-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "wp-001"},
			},
			knowledgeErr:             errors.New("write failed"),
			wantErr:                  true,
			wantErrContains:          "runner: resume extraction: write progress:",
			wantErrContainsInner:     "write failed",
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 0, // write progress fails before validate
			wantSessionInvoked:       true,
		},
		{
			name:      "mutation asymmetry preserved",
			sessionID: "mut-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "mut-001"},
			},
			wantErr:                  false,
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 1,
			wantSessionInvoked:       true,
		},
		{
			name:      "validate lessons error",
			sessionID: "vl-err-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "vl-001"},
			},
			validateLessonsErr:       errors.New("validation failed"),
			wantErr:                  true,
			wantErrContains:          "runner: resume extraction: validate lessons:",
			wantErrContainsInner:     "validation failed",
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 1,
			wantSessionInvoked:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Setup sprint-tasks.md for mutation asymmetry check
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)
			beforeContent, err := os.ReadFile(tasksPath)
			if err != nil {
				t.Fatalf("read tasks before: %v", err)
			}
			beforeCheckCount := strings.Count(string(beforeContent), "[x]")

			cfg := testConfig(tmpDir, 1)
			cfg.ModelExecute = "opus"

			var stateDir string
			if len(tc.scenarioSteps) > 0 {
				scenario := testutil.Scenario{
					Name:  "resume-" + tc.name,
					Steps: tc.scenarioSteps,
				}
				_, stateDir = testutil.SetupMockClaude(t, scenario)
			} else {
				// No scenario: use nonexistent binary to trigger exec error
				cfg.ClaudeCommand = "/nonexistent/binary"
			}

			kw := &trackingKnowledgeWriter{
				writeProgressErr:   tc.knowledgeErr,
				validateLessonsErr: tc.validateLessonsErr,
			}

			err = runner.ResumeExtraction(context.Background(), cfg, kw, tc.sessionID)

			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil error, got %v", err)
			}

			if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error: want containing %q, got %q", tc.wantErrContains, err.Error())
			}
			if tc.wantErrContainsInner != "" && !strings.Contains(err.Error(), tc.wantErrContainsInner) {
				t.Errorf("inner error: want containing %q, got %q", tc.wantErrContainsInner, err.Error())
			}

			if kw.writeProgressCount != tc.wantWriteProgressCount {
				t.Errorf("WriteProgress count = %d, want %d", kw.writeProgressCount, tc.wantWriteProgressCount)
			}
			if kw.validateLessonsCount != tc.wantValidateLessonsCount {
				t.Errorf("ValidateNewLessons count = %d, want %d", kw.validateLessonsCount, tc.wantValidateLessonsCount)
			}

			if tc.wantSessionInvoked && stateDir != "" {
				args := testutil.ReadInvocationArgs(t, stateDir, 0)
				if tc.checkArgs != nil {
					tc.checkArgs(t, args)
				}
			}

			if tc.checkData != nil {
				tc.checkData(t, kw)
			}

			// Mutation asymmetry: [x] count must be unchanged
			afterContent, readErr := os.ReadFile(tasksPath)
			if readErr != nil {
				t.Fatalf("read tasks after: %v", readErr)
			}
			afterCheckCount := strings.Count(string(afterContent), "[x]")
			if afterCheckCount != beforeCheckCount {
				t.Errorf("[x] count changed: before=%d, after=%d — mutation asymmetry violated", beforeCheckCount, afterCheckCount)
			}
		})
	}
}

// TestResumeExtraction_ParseError verifies parse error wrapping when mock returns
// empty stdout with exit code 0 (valid execution but unparseable output).
func TestResumeExtraction_ParseError(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig(tmpDir, 1)
	cfg.ModelExecute = "opus"

	// MOCK_EXIT_EMPTY causes the test binary to exit with code 0 and empty stdout,
	// which triggers a parse error in session.ParseResult.
	t.Setenv("MOCK_EXIT_EMPTY", "0")

	kw := &trackingKnowledgeWriter{}

	err := runner.ResumeExtraction(context.Background(), cfg, kw, "parse-err-session")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: resume extraction: parse:") {
		t.Errorf("error: want containing %q, got %q", "runner: resume extraction: parse:", err.Error())
	}
	if kw.writeProgressCount != 0 {
		t.Errorf("WriteProgress count = %d, want 0 (parse failed before write)", kw.writeProgressCount)
	}
}

// TestResumeExtraction_SnapshotDiff verifies that pre-existing LEARNINGS.md content
// is passed as snapshot to ValidateNewLessons for diff-based validation (AC #3).
func TestResumeExtraction_SnapshotDiff(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pre-existing LEARNINGS.md
	existingContent := "## testing: patterns [review, runner/runner.go:1]\nExisting lesson\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(existingContent), 0644); err != nil {
		t.Fatalf("write LEARNINGS.md: %v", err)
	}

	cfg := testConfig(tmpDir, 1)
	cfg.ModelExecute = "opus"
	cfg.LearningsBudget = 200

	scenario := testutil.Scenario{
		Name: "resume-snapshot",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "snap-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	kw := &trackingKnowledgeWriter{}

	err := runner.ResumeExtraction(context.Background(), cfg, kw, "snap-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if kw.validateLessonsCount != 1 {
		t.Fatalf("ValidateNewLessons count = %d, want 1", kw.validateLessonsCount)
	}
	data := kw.validateLessonsData[0]
	if data.Source != "resume-extraction" {
		t.Errorf("Source = %q, want %q", data.Source, "resume-extraction")
	}
	if data.Snapshot != existingContent {
		t.Errorf("Snapshot = %q, want %q", data.Snapshot, existingContent)
	}
	if data.BudgetLimit != 200 {
		t.Errorf("BudgetLimit = %d, want 200", data.BudgetLimit)
	}
}

// TestResumeExtraction_NoChanges verifies that when LEARNINGS.md does not exist,
// ValidateNewLessons is still called with empty snapshot and no error (AC #5).
func TestResumeExtraction_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := testConfig(tmpDir, 1)
	cfg.ModelExecute = "opus"

	scenario := testutil.Scenario{
		Name: "resume-no-changes",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nc-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	kw := &trackingKnowledgeWriter{}

	err := runner.ResumeExtraction(context.Background(), cfg, kw, "nc-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if kw.validateLessonsCount != 1 {
		t.Fatalf("ValidateNewLessons count = %d, want 1", kw.validateLessonsCount)
	}
	if kw.validateLessonsData[0].Snapshot != "" {
		t.Errorf("Snapshot should be empty when LEARNINGS.md missing, got %q", kw.validateLessonsData[0].Snapshot)
	}
}

// TestResumeExtraction_SnapshotReadError verifies that a non-NotExist error on
// LEARNINGS.md read returns a wrapped error with "runner: resume extraction: snapshot:" prefix.
func TestResumeExtraction_SnapshotReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create LEARNINGS.md as a directory to trigger non-NotExist read error
	learningsDir := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("create LEARNINGS.md dir: %v", err)
	}

	cfg := testConfig(tmpDir, 1)
	cfg.ModelExecute = "opus"

	kw := &trackingKnowledgeWriter{}

	err := runner.ResumeExtraction(context.Background(), cfg, kw, "snap-read-err")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: resume extraction: snapshot:") {
		t.Errorf("error: want containing %q, got %q", "runner: resume extraction: snapshot:", err.Error())
	}
	if !strings.Contains(err.Error(), "LEARNINGS.md") {
		t.Errorf("error: want containing file path, got %q", err.Error())
	}
	if kw.writeProgressCount != 0 {
		t.Errorf("WriteProgress count = %d, want 0 (snapshot failed before session)", kw.writeProgressCount)
	}
	if kw.validateLessonsCount != 0 {
		t.Errorf("ValidateNewLessons count = %d, want 0", kw.validateLessonsCount)
	}
}

// =============================================================================
// Findings injection tests (Story 4.7, AC2, AC6, AC7)
// =============================================================================

// TestRunner_Execute_FindingsInjection verifies the execute-review cycle flow
// when review-findings.md contains findings: first execute runs, review returns
// not-clean, second execute runs (with findings file present), second review
// returns clean → loop exits. Validates AC2, AC6, AC7.
func TestRunner_Execute_FindingsInjection(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// Write review-findings.md with sample findings content
	findingsPath := filepath.Join(tmpDir, "review-findings.md")
	findingsContent := "## [HIGH] Test finding\n- **ЧТО не так** — test issue\n"
	if err := os.WriteFile(findingsPath, []byte(findingsContent), 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	// 2 execute iterations → need 2 scenario steps
	scenario := testutil.Scenario{
		Name: "findings-injection",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-findings-1"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-findings-2"},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"}, // first execute: commit detected
			[2]string{"bbb", "ccc"}, // second execute: commit detected
		),
	}

	cfg := testConfig(tmpDir, 3)

	reviewCount := 0
	r := &runner.Runner{
		Cfg:       cfg,
		Git:       mock,
		TasksFile: tasksPath,
		ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
			reviewCount++
			if reviewCount == 1 {
				// First review: not clean (findings cycle continues)
				return runner.ReviewResult{Clean: false}, nil
			}
			// Second review: write all-done tasks + clean
			if wErr := os.WriteFile(tasksPath, []byte(allDoneTasks), 0644); wErr != nil {
				t.Errorf("write all-done tasks: %v", wErr)
			}
			return runner.ReviewResult{Clean: true}, nil
		},
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC2: ReviewFn called exactly 2 times (2 review cycles)
	if reviewCount != 2 {
		t.Errorf("reviewCount = %d, want 2", reviewCount)
	}

	// 2 execute iterations → 2 HeadCommit before + 2 HeadCommit after = 4
	if mock.HeadCommitCount != 4 {
		t.Errorf("HeadCommitCount = %d, want 4", mock.HeadCommitCount)
	}

	// Verify both mock Claude invocations consumed (2 execute sessions)
	testutil.ReadInvocationArgs(t, stateDir, 0) // first execute
	testutil.ReadInvocationArgs(t, stateDir, 1) // second execute
}

// TestRunner_Execute_FindingsReadError verifies that a non-ErrNotExist error
// reading review-findings.md returns a wrapped error with "runner: read findings:" prefix.
func TestRunner_Execute_FindingsReadError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// Make review-findings.md a directory → os.ReadFile fails with non-ErrNotExist
	findingsPath := filepath.Join(tmpDir, "review-findings.md")
	if err := os.MkdirAll(findingsPath, 0755); err != nil {
		t.Fatalf("create findings dir: %v", err)
	}

	mock := &testutil.MockGitClient{}
	cfg := testConfig(tmpDir, 3)

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: read findings:") {
		t.Errorf("error: want containing %q, got %q", "runner: read findings:", err.Error())
	}
	// Inner error: verify wrapping includes OS path (platform-specific message varies:
	// Linux="is a directory", Windows="Incorrect function.")
	if !strings.Contains(err.Error(), "review-findings.md") {
		t.Errorf("error: want inner cause containing file path, got %q", err.Error())
	}
	// Guard: error occurs before execute loop — no HeadCommit calls
	if mock.HeadCommitCount != 0 {
		t.Errorf("HeadCommitCount = %d, want 0 (error before execute)", mock.HeadCommitCount)
	}
	// Guard: startup RecoverDirtyState called HealthCheck once
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount = %d, want 1 (startup only)", mock.HealthCheckCount)
	}
}

// =============================================================================
// DetermineReviewOutcome tests (Story 4.3, AC5)
// =============================================================================

// TestDetermineReviewOutcome_Scenarios verifies file-state-based review outcome logic.
// Clean = taskMarkedDone AND (findingsAbsent OR findingsEmpty).
func TestDetermineReviewOutcome_Scenarios(t *testing.T) {
	emptyFindings := ""
	whitespaceFindings := "  \n  \n"
	realFindings := "## Finding 1\nSome issue found\n"

	tests := []struct {
		name            string
		tasksContent    string
		currentTaskText string
		findingsContent *string // nil = no findings file
		findingsIsDir   bool    // true = create directory instead of file (triggers non-NotExist error)
		wantClean       bool
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "clean review - task done no findings",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n- [ ] Task two\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: nil,
			wantClean:       true,
		},
		{
			name:            "clean review - empty findings file",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &emptyFindings,
			wantClean:       true,
		},
		{
			name:            "clean review - whitespace-only findings",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &whitespaceFindings,
			wantClean:       true,
		},
		{
			name:            "with findings - task not done",
			tasksContent:    "# Sprint Tasks\n\n- [ ] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &realFindings,
			wantClean:       false,
		},
		{
			name:            "task done but findings non-empty",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &realFindings,
			wantClean:       false,
		},
		{
			name:            "no change no findings - session failed silently",
			tasksContent:    "# Sprint Tasks\n\n- [ ] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: nil,
			wantClean:       false,
		},
		{
			name:            "bare task text without checkbox prefix",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "Task one",
			findingsContent: nil,
			wantClean:       true,
		},
		{
			name:            "read error - tasks file missing",
			tasksContent:    "", // will not create file
			currentTaskText: "- [ ] Task one",
			wantErr:         true,
			wantErrContains: "runner: determine review outcome:",
		},
		{
			name:            "read error - findings file is directory",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsIsDir:   true,
			wantErr:         true,
			wantErrContains: "runner: determine review outcome:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			var tasksFile string
			if tt.tasksContent != "" {
				tasksFile = writeTasksFile(t, tmpDir, tt.tasksContent)
			} else {
				// Non-existent file for error test
				tasksFile = filepath.Join(tmpDir, "sprint-tasks.md")
			}

			if tt.findingsIsDir {
				findingsPath := filepath.Join(tmpDir, "review-findings.md")
				if err := os.MkdirAll(findingsPath, 0755); err != nil {
					t.Fatalf("create findings dir: %v", err)
				}
			} else if tt.findingsContent != nil {
				findingsPath := filepath.Join(tmpDir, "review-findings.md")
				if err := os.WriteFile(findingsPath, []byte(*tt.findingsContent), 0644); err != nil {
					t.Fatalf("write findings: %v", err)
				}
			}

			rr, err := runner.DetermineReviewOutcome(tasksFile, tt.currentTaskText, tmpDir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error: want containing %q, got %q", tt.wantErrContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rr.Clean != tt.wantClean {
				t.Errorf("Clean = %v, want %v", rr.Clean, tt.wantClean)
			}
		})
	}
}

// =============================================================================
// Story 5.2: Gate Detection in Runner (AC: #1-#8)
// =============================================================================

// TestRunner_Execute_GateContinueActions verifies gate prompt called on [GATE] task
// with approve/skip/retry decisions — all continue to next task (AC3, AC5, AC7).
// Table-driven: approve, skip, retry all produce same outcome (proceed).
func TestRunner_Execute_GateContinueActions(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{name: "approve", action: config.ActionApprove},
		{name: "skip", action: config.ActionSkip},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, gp := setupGateTest(t, gateOpenTask, true)
			gp.decision = &config.GateDecision{Action: tc.action}

			err := r.Execute(context.Background())
			if err != nil {
				t.Fatalf("Execute: want nil, got: %v", err)
			}

			// AC3: GatePromptFn called with task text
			if gp.count != 1 {
				t.Errorf("GatePromptFn count = %d, want 1", gp.count)
			}
			if !strings.Contains(gp.taskText, "Setup project") {
				t.Errorf("GatePromptFn taskText = %q, want containing 'Setup project'", gp.taskText)
			}
			if !strings.Contains(gp.taskText, "[GATE]") {
				t.Errorf("GatePromptFn taskText = %q, want containing '[GATE]'", gp.taskText)
			}
		})
	}
}

// TestRunner_Execute_GateQuit verifies quit at gate returns wrapped GateDecision error (AC6).
func TestRunner_Execute_GateQuit(t *testing.T) {
	r, gp := setupGateTest(t, gateOpenTask, true)
	gp.decision = &config.GateDecision{Action: config.ActionQuit}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}

	// AC6: error wraps GateDecision
	if !strings.Contains(err.Error(), "runner: gate:") {
		t.Errorf("error prefix: want 'runner: gate:', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "gate: quit") {
		t.Errorf("inner error: want 'gate: quit' (GateDecision.Error()), got %q", err.Error())
	}

	// AC6: errors.As extracts GateDecision with Action == "quit"
	var gd *config.GateDecision
	if !errors.As(err, &gd) {
		t.Fatal("errors.As(err, &GateDecision): want true, got false")
	}
	if gd.Action != config.ActionQuit {
		t.Errorf("GateDecision.Action = %q, want %q", gd.Action, config.ActionQuit)
	}

	// GatePromptFn was called
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
}

// TestRunner_Execute_GatesDisabled verifies GatePromptFn NOT called when gates disabled (AC2).
func TestRunner_Execute_GatesDisabled(t *testing.T) {
	r, gp := setupGateTest(t, gateOpenTask, false)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC2: GatePromptFn NOT called
	if gp.count != 0 {
		t.Errorf("GatePromptFn count = %d, want 0 (gates disabled)", gp.count)
	}
}

// TestRunner_Execute_NoGateTag verifies GatePromptFn NOT called for tasks without [GATE] (AC8).
func TestRunner_Execute_NoGateTag(t *testing.T) {
	r, gp := setupGateTest(t, nonGateOpenTask, true)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC8: GatePromptFn NOT called (no [GATE] tag)
	if gp.count != 0 {
		t.Errorf("GatePromptFn count = %d, want 0 (no [GATE] tag)", gp.count)
	}
}

// TestRunner_Execute_GatePromptFnNil verifies no panic when GatesEnabled=true, task has [GATE],
// but GatePromptFn is nil (defensive nil-guard in condition at runner.go:408).
func TestRunner_Execute_GatePromptFnNil(t *testing.T) {
	r, _ := setupGateTest(t, gateOpenTask, true)
	r.GatePromptFn = nil // override: nil guard path

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil (nil guard skips gate), got: %v", err)
	}
}

// TestRunner_Execute_GatePromptError verifies gate prompt error propagation (AC3 error path).
func TestRunner_Execute_GatePromptError(t *testing.T) {
	r, gp := setupGateTest(t, gateOpenTask, true)
	gp.decision = nil
	gp.err = context.Canceled

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}

	// Error wrapping: "runner: gate:" prefix
	if !strings.Contains(err.Error(), "runner: gate:") {
		t.Errorf("error prefix: want 'runner: gate:', got %q", err.Error())
	}

	// errors.Is unwraps to context.Canceled
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled): want true, got false; err = %v", err)
	}

	// GatePromptFn was called
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
}

// --- Story 5.3: Retry with feedback tests ---

// TestInjectFeedback_Scenarios verifies feedback injection into sprint-tasks.md (AC4, AC5).
// Coverage gap: os.WriteFile error path (line 255 in runner.go) is not testable without DI —
// ReadFile and WriteFile use the same path, so we cannot make Read succeed but Write fail.
// The error wrapping pattern is verified by the ReadFile error case.
func TestInjectFeedback_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		content         string
		taskDesc        string
		feedback        string
		wantErr         bool
		wantErrContains string
		wantContains    []string
	}{
		{
			name:     "basic injection",
			content:  "- [ ] Setup project [GATE]\n- [ ] Write tests\n",
			taskDesc: "Setup project [GATE]",
			feedback: "Need validation",
			wantContains: []string{
				"- [ ] Setup project [GATE]",
				"  " + config.FeedbackPrefix + " Need validation",
				"- [ ] Write tests",
			},
		},
		{
			name: "preserve existing feedback",
			content: "- [ ] Setup project [GATE]\n" +
				"  " + config.FeedbackPrefix + " First attempt feedback\n" +
				"- [ ] Write tests\n",
			taskDesc: "Setup project [GATE]",
			feedback: "Second attempt feedback",
			wantContains: []string{
				config.FeedbackPrefix + " First attempt feedback",
				config.FeedbackPrefix + " Second attempt feedback",
			},
		},
		{
			name:     "multiple tasks inject on correct one",
			content:  "- [ ] Task alpha\n- [ ] Task beta [GATE]\n- [ ] Task gamma\n",
			taskDesc: "Task beta [GATE]",
			feedback: "fix beta",
			wantContains: []string{
				"- [ ] Task beta [GATE]",
				config.FeedbackPrefix + " fix beta",
				"- [ ] Task gamma",
			},
		},
		{
			name:            "task not found",
			content:         "- [ ] Setup project [GATE]\n",
			taskDesc:        "Nonexistent task",
			feedback:        "feedback",
			wantErr:         true,
			wantErrContains: "runner: inject feedback: task not found: Nonexistent task",
		},
		{
			name:            "read error non-existent file",
			content:         "", // sentinel: uses non-existent path
			taskDesc:        "any",
			feedback:        "any",
			wantErr:         true,
			wantErrContains: "runner: inject feedback:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			var tasksPath string
			if tc.name == "read error non-existent file" {
				tasksPath = filepath.Join(tmpDir, "nonexistent", "sprint-tasks.md")
			} else {
				tasksPath = writeTasksFile(t, tmpDir, tc.content)
			}

			err := runner.InjectFeedback(tasksPath, tc.taskDesc, tc.feedback)

			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, readErr := os.ReadFile(tasksPath)
			if readErr != nil {
				t.Fatalf("read result file: %v", readErr)
			}
			result := string(data)
			for _, want := range tc.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

// TestRevertTask_Scenarios verifies task revert [x]→[ ] (AC6).
// Coverage gap: os.WriteFile error path (line 280 in runner.go) is not testable without DI —
// ReadFile and WriteFile use the same path, so we cannot make Read succeed but Write fail.
// The error wrapping pattern is verified by the ReadFile error case.
func TestRevertTask_Scenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		content         string
		taskDesc        string
		wantErr         bool
		wantErrContains string
		wantContains    []string
	}{
		{
			name:     "basic revert",
			content:  "- [x] Setup project [GATE]\n- [ ] Write tests\n",
			taskDesc: "Setup project [GATE]",
			wantContains: []string{
				"- [ ] Setup project [GATE]",
				"- [ ] Write tests",
			},
		},
		{
			name:     "preserves other done tasks",
			content:  "- [x] Task alpha\n- [x] Task beta [GATE]\n- [ ] Task gamma\n",
			taskDesc: "Task beta [GATE]",
			wantContains: []string{
				"- [x] Task alpha",       // unchanged
				"- [ ] Task beta [GATE]", // reverted
				"- [ ] Task gamma",       // unchanged
			},
		},
		{
			name:            "task not found as done",
			content:         "- [ ] Setup project [GATE]\n",
			taskDesc:        "Setup project [GATE]",
			wantErr:         true,
			wantErrContains: "runner: revert task: task not found: Setup project [GATE]",
		},
		{
			name:            "task not found at all",
			content:         "- [x] Setup project [GATE]\n",
			taskDesc:        "Nonexistent task",
			wantErr:         true,
			wantErrContains: "runner: revert task: task not found: Nonexistent task",
		},
		{
			name:            "read error non-existent file",
			content:         "",
			taskDesc:        "any",
			wantErr:         true,
			wantErrContains: "runner: revert task:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			var tasksPath string
			if tc.name == "read error non-existent file" {
				tasksPath = filepath.Join(tmpDir, "nonexistent", "sprint-tasks.md")
			} else {
				tasksPath = writeTasksFile(t, tmpDir, tc.content)
			}

			err := runner.RevertTask(tasksPath, tc.taskDesc)

			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, readErr := os.ReadFile(tasksPath)
			if readErr != nil {
				t.Fatalf("read result file: %v", readErr)
			}
			result := string(data)
			for _, want := range tc.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

// TestRunner_Execute_GateRetry verifies retry injects feedback, reverts task, re-processes (AC4, AC6, AC7).
func TestRunner_Execute_GateRetry(t *testing.T) {
	tmpDir := t.TempDir()

	// Start with open task — ReviewFn will mark [x] (simulating Claude execution)
	tasksPath := writeTasksFile(t, tmpDir, gateOpenTask)

	// MockClaude: 2 executions (1st before retry, 2nd after retry)
	scenario := testutil.Scenario{
		Name: "gate-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "retry-001"},
			{Type: "execute", ExitCode: 0, SessionID: "retry-002"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	// 2 iterations: before1/after1 (commit detected) + before2/after2
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}, [2]string{"ccc", "ddd"}),
	}

	cfg := testConfig(tmpDir, 3) // MaxIterations=3 to allow retry + re-process
	cfg.GatesEnabled = true

	// ReviewFn: each call marks task [x] (simulating Claude marking done) then returns clean.
	// On 2nd call, captures file content (to verify feedback injection) then marks all done.
	reviewCount := 0
	var capturedAfterRetry string // file content captured at start of 2nd review
	reviewFn := func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		if reviewCount == 1 {
			// 1st review: mark task [x] — gate retry will revert to [ ]
			// Error ignored: test helper in controlled tmpDir
			_ = os.WriteFile(tasksPath, []byte(gateOpenTaskDone), 0644)
		} else {
			// 2nd review: capture file state BEFORE overwriting (verify inject+revert)
			data, _ := os.ReadFile(tasksPath)
			capturedAfterRetry = string(data)
			// Mark all done so outer loop exits
			// Error ignored: test helper in controlled tmpDir
			_ = os.WriteFile(tasksPath, []byte(allDoneTasks), 0644)
		}
		return runner.ReviewResult{Clean: true}, nil
	}

	// Gate sequence: 1st call = retry with feedback, 2nd call = approve
	gp := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionRetry, Feedback: "Need validation"},
			{Action: config.ActionApprove},
		},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// Verify gate was called twice (retry then approve)
	if gp.calls != 2 {
		t.Errorf("GatePromptFn calls = %d, want 2", gp.calls)
	}

	// Verify review was called twice
	if reviewCount != 2 {
		t.Errorf("ReviewFn count = %d, want 2", reviewCount)
	}

	// Verify task text passed to gate prompt
	if len(gp.taskTexts) < 1 {
		t.Fatal("expected at least 1 taskText recorded")
	}
	if !strings.Contains(gp.taskTexts[0], "Setup project") {
		t.Errorf("1st gate taskText = %q, want containing 'Setup project'", gp.taskTexts[0])
	}

	// M1: Verify feedback was actually injected into file (AC4 end-to-end)
	if !strings.Contains(capturedAfterRetry, config.FeedbackPrefix+" Need validation") {
		t.Errorf("after retry, file missing feedback line\ngot:\n%s", capturedAfterRetry)
	}
	// Verify task was reverted to [ ] (AC6 end-to-end)
	if !strings.Contains(capturedAfterRetry, "- [ ] Setup project [GATE]") {
		t.Errorf("after retry, task not reverted to [ ]\ngot:\n%s", capturedAfterRetry)
	}
}

// TestRunner_Execute_GateRetryEmptyFeedback verifies retry with empty feedback skips injection (AC4).
func TestRunner_Execute_GateRetryEmptyFeedback(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, gateOpenTask)

	scenario := testutil.Scenario{
		Name: "gate-retry-empty",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "empty-001"},
			{Type: "execute", ExitCode: 0, SessionID: "empty-002"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}, [2]string{"ccc", "ddd"}),
	}

	cfg := testConfig(tmpDir, 3)
	cfg.GatesEnabled = true

	// ReviewFn: 1st marks [x] (gate retry reverts), 2nd captures file then marks all done
	reviewCount := 0
	var capturedAfterRetry string
	reviewFn := func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		if reviewCount == 1 {
			// Error ignored: test helper in controlled tmpDir
			_ = os.WriteFile(tasksPath, []byte(gateOpenTaskDone), 0644)
		} else {
			// Capture file state to verify no feedback was injected
			data, _ := os.ReadFile(tasksPath)
			capturedAfterRetry = string(data)
			// Error ignored: test helper in controlled tmpDir
			_ = os.WriteFile(tasksPath, []byte(allDoneTasks), 0644)
		}
		return runner.ReviewResult{Clean: true}, nil
	}

	gp := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionRetry, Feedback: ""}, // empty feedback
			{Action: config.ActionApprove},
		},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	if gp.calls != 2 {
		t.Errorf("GatePromptFn calls = %d, want 2", gp.calls)
	}
	if reviewCount != 2 {
		t.Errorf("ReviewFn count = %d, want 2", reviewCount)
	}

	// M4: Verify empty feedback was NOT injected (negative assertion)
	if strings.Contains(capturedAfterRetry, config.FeedbackPrefix) {
		t.Errorf("after empty-feedback retry, file should NOT contain %q\ngot:\n%s", config.FeedbackPrefix, capturedAfterRetry)
	}
	// Task should still be reverted to [ ] even without feedback
	if !strings.Contains(capturedAfterRetry, "- [ ] Setup project [GATE]") {
		t.Errorf("after retry, task not reverted to [ ]\ngot:\n%s", capturedAfterRetry)
	}
}

// TestRunner_Execute_GateRetryInjectError verifies inject failure propagation (AC4 error path).
func TestRunner_Execute_GateRetryInjectError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, gateOpenTask)

	scenario := testutil.Scenario{
		Name: "gate-retry-inject-err",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "injerr-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	cfg := testConfig(tmpDir, 3)
	cfg.GatesEnabled = true

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionRetry, Feedback: "trigger inject"},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        cleanReviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	// ReviewFn marks task [x] then swaps TasksFile to bad path so InjectFeedback fails
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(tasksPath, []byte(gateOpenTaskDone), 0644)
		r.TasksFile = filepath.Join(tmpDir, "nonexistent", "sprint-tasks.md")
		return runner.ReviewResult{Clean: true}, nil
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: inject feedback:") {
		t.Errorf("error = %q, want containing 'runner: inject feedback:'", err.Error())
	}
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
}

// =============================================================================
// Story 5.4: Checkpoint gates tests (AC: #1-#8)
// =============================================================================

// TestRunner_Execute_CheckpointFires verifies checkpoint gate fires every N tasks (AC1, AC2, AC4).
func TestRunner_Execute_CheckpointFires(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, fourOpenTasks)

	// 4 tasks = 4 execute steps
	scenario := testutil.Scenario{
		Name: "checkpoint-fires",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "cp-001"},
			{Type: "execute", ExitCode: 0, SessionID: "cp-002"},
			{Type: "execute", ExitCode: 0, SessionID: "cp-003"},
			{Type: "execute", ExitCode: 0, SessionID: "cp-004"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"a1", "a2"}, [2]string{"b1", "b2"},
			[2]string{"c1", "c2"}, [2]string{"d1", "d2"},
		),
	}

	cfg := testConfig(tmpDir, 5)
	cfg.GatesEnabled = true
	cfg.GatesCheckpoint = 2

	reviewCount := 0
	gp := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionApprove},
			{Action: config.ActionApprove},
		},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        progressiveReviewFn(tasksPath, &reviewCount),
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC1: checkpoint fires after task 2 and task 4 (2 calls total)
	if gp.calls != 2 {
		t.Errorf("GatePromptFn calls = %d, want 2", gp.calls)
	}

	// AC2: counter cumulative — review called 4 times (one per task)
	if reviewCount != 4 {
		t.Errorf("ReviewFn count = %d, want 4", reviewCount)
	}

	// AC1: gate text contains checkpoint indicator
	for i, text := range gp.taskTexts {
		if !strings.Contains(text, "(checkpoint every 2)") {
			t.Errorf("gate text[%d] = %q, want containing '(checkpoint every 2)'", i, text)
		}
	}

	// AC4: checkpoint fires on non-GATE tasks — verify no [GATE] in text
	for i, text := range gp.taskTexts {
		if strings.Contains(text, "[GATE]") {
			t.Errorf("gate text[%d] = %q, should NOT contain '[GATE]' (checkpoint-only)", i, text)
		}
	}

	// Verify task identity: checkpoint at task 2 and task 4
	if len(gp.taskTexts) >= 2 {
		if !strings.Contains(gp.taskTexts[0], "Task two") {
			t.Errorf("gate text[0] = %q, want containing 'Task two' (checkpoint at task 2)", gp.taskTexts[0])
		}
		if !strings.Contains(gp.taskTexts[1], "Task four") {
			t.Errorf("gate text[1] = %q, want containing 'Task four' (checkpoint at task 4)", gp.taskTexts[1])
		}
	}
}

// TestRunner_Execute_CheckpointNotTriggered verifies checkpoint does not fire when
// disabled (AC6: --every 0) or when gates flag is off (AC7: GatesEnabled=false).
func TestRunner_Execute_CheckpointNotTriggered(t *testing.T) {
	cases := []struct {
		name            string
		gatesEnabled    bool
		gatesCheckpoint int
	}{
		{"every 0 disables checkpoint AC6", true, 0},
		{"no gates flag disables checkpoint AC7", false, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

			scenario := testutil.Scenario{
				Name: "checkpoint-not-triggered",
				Steps: []testutil.ScenarioStep{
					{Type: "execute", ExitCode: 0, SessionID: "nt-001"},
					{Type: "execute", ExitCode: 0, SessionID: "nt-002"},
					{Type: "execute", ExitCode: 0, SessionID: "nt-003"},
				},
			}
			testutil.SetupMockClaude(t, scenario)

			mock := &testutil.MockGitClient{
				HeadCommits: headCommitPairs(
					[2]string{"a1", "a2"}, [2]string{"b1", "b2"}, [2]string{"c1", "c2"},
				),
			}

			cfg := testConfig(tmpDir, 5)
			cfg.GatesEnabled = c.gatesEnabled
			cfg.GatesCheckpoint = c.gatesCheckpoint

			gp := &trackingGatePrompt{
				decision: &config.GateDecision{Action: config.ActionApprove},
			}

			reviewCount := 0
			r := &runner.Runner{
				Cfg:             cfg,
				Git:             mock,
				TasksFile:       tasksPath,
				ReviewFn:        progressiveReviewFn(tasksPath, &reviewCount),
				GatePromptFn:    gp.fn,
				ResumeExtractFn: noopResumeExtractFn,
				SleepFn:         noopSleepFn,
				Knowledge:       &runner.NoOpKnowledgeWriter{},
			}

			err := r.Execute(context.Background())
			if err != nil {
				t.Fatalf("Execute: want nil, got: %v", err)
			}

			if gp.count != 0 {
				t.Errorf("GatePromptFn count = %d, want 0", gp.count)
			}
			if reviewCount != 3 {
				t.Errorf("ReviewFn count = %d, want 3", reviewCount)
			}
		})
	}
}

// TestRunner_Execute_CheckpointCombinedWithGate verifies combined GATE + checkpoint = single prompt (AC5).
func TestRunner_Execute_CheckpointCombinedWithGate(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, fourOpenTasksWithGate)

	// 4 tasks
	scenario := testutil.Scenario{
		Name: "checkpoint-combined",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "comb-001"},
			{Type: "execute", ExitCode: 0, SessionID: "comb-002"},
			{Type: "execute", ExitCode: 0, SessionID: "comb-003"},
			{Type: "execute", ExitCode: 0, SessionID: "comb-004"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"a1", "a2"}, [2]string{"b1", "b2"},
			[2]string{"c1", "c2"}, [2]string{"d1", "d2"},
		),
	}

	cfg := testConfig(tmpDir, 5)
	cfg.GatesEnabled = true
	cfg.GatesCheckpoint = 2

	gp := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionApprove}, // task 2: combined GATE + checkpoint
			{Action: config.ActionApprove}, // task 4: checkpoint only
		},
	}

	reviewCount := 0
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        progressiveReviewFn(tasksPath, &reviewCount),
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC5: ONE prompt for task 2 (GATE + checkpoint combined), ONE for task 4 (checkpoint)
	if gp.calls != 2 {
		t.Errorf("GatePromptFn calls = %d, want 2", gp.calls)
	}

	// AC5: task 2 text has both [GATE] and checkpoint indicator
	if len(gp.taskTexts) < 1 {
		t.Fatal("expected at least 1 taskText recorded")
	}
	if !strings.Contains(gp.taskTexts[0], "[GATE]") {
		t.Errorf("task 2 gate text = %q, want containing '[GATE]'", gp.taskTexts[0])
	}
	if !strings.Contains(gp.taskTexts[0], "(checkpoint every 2)") {
		t.Errorf("task 2 gate text = %q, want containing '(checkpoint every 2)'", gp.taskTexts[0])
	}

	// Task 4: checkpoint only (no [GATE])
	if len(gp.taskTexts) >= 2 {
		if strings.Contains(gp.taskTexts[1], "[GATE]") {
			t.Errorf("task 4 gate text = %q, should NOT contain '[GATE]'", gp.taskTexts[1])
		}
		if !strings.Contains(gp.taskTexts[1], "(checkpoint every 2)") {
			t.Errorf("task 4 gate text = %q, want containing '(checkpoint every 2)'", gp.taskTexts[1])
		}
	}

	if reviewCount != 4 {
		t.Errorf("ReviewFn count = %d, want 4", reviewCount)
	}
}

// TestRunner_Execute_CheckpointGateOnly verifies GATE fires without checkpoint when N not reached (AC1, AC4).
func TestRunner_Execute_CheckpointGateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	// Task 1 has [GATE], checkpoint=5 (won't fire for 1 task)
	tasksPath := writeTasksFile(t, tmpDir, gateOpenTask)

	scenario := testutil.Scenario{
		Name: "checkpoint-gate-only",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "go-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	cfg := testConfig(tmpDir, 1)
	cfg.GatesEnabled = true
	cfg.GatesCheckpoint = 5 // high — checkpoint won't fire on task 1

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        cleanReviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC1: gate fires for [GATE] tag, NOT checkpoint
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1 (GATE tag trigger)", gp.count)
	}

	// Verify gate text has [GATE] but NOT checkpoint indicator
	if !strings.Contains(gp.taskText, "[GATE]") {
		t.Errorf("gate text = %q, want containing '[GATE]'", gp.taskText)
	}
	if strings.Contains(gp.taskText, "(checkpoint every") {
		t.Errorf("gate text = %q, should NOT contain checkpoint (N not reached)", gp.taskText)
	}
}

// TestRunner_Execute_CheckpointSkipCounts verifies skipped tasks count toward checkpoint (AC3).
// When user skips at a [GATE] task, completedTasks is NOT decremented (unlike retry),
// so checkpoint still fires at the correct cumulative count.
func TestRunner_Execute_CheckpointSkipCounts(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeTasksWithGate)

	scenario := testutil.Scenario{
		Name: "checkpoint-skip-counts",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sk-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sk-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sk-003"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"a1", "a2"}, [2]string{"b1", "b2"}, [2]string{"c1", "c2"},
		),
	}

	cfg := testConfig(tmpDir, 5)
	cfg.GatesEnabled = true
	cfg.GatesCheckpoint = 2

	// Gate sequence: 1st call (task 1, GATE, completedTasks=1) = skip,
	// 2nd call (task 2, checkpoint, completedTasks=2) = approve
	gp := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionSkip},
			{Action: config.ActionApprove},
		},
	}

	reviewCount := 0
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        progressiveReviewFn(tasksPath, &reviewCount),
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC3: skip does NOT decrement counter → checkpoint fires at task 2 (completedTasks=2)
	if gp.calls != 2 {
		t.Errorf("GatePromptFn calls = %d, want 2 (GATE skip + checkpoint)", gp.calls)
	}

	// 1st call: GATE only (completedTasks=1, not divisible by 2)
	if len(gp.taskTexts) >= 1 {
		if !strings.Contains(gp.taskTexts[0], "[GATE]") {
			t.Errorf("call 1 text = %q, want containing '[GATE]'", gp.taskTexts[0])
		}
		if strings.Contains(gp.taskTexts[0], "(checkpoint every") {
			t.Errorf("call 1 text = %q, should NOT have checkpoint (count=1)", gp.taskTexts[0])
		}
	}

	// 2nd call: checkpoint (completedTasks=2 — skip did not decrement)
	if len(gp.taskTexts) >= 2 {
		if !strings.Contains(gp.taskTexts[1], "(checkpoint every 2)") {
			t.Errorf("call 2 text = %q, want containing '(checkpoint every 2)'", gp.taskTexts[1])
		}
	}

	if reviewCount != 3 {
		t.Errorf("ReviewFn count = %d, want 3", reviewCount)
	}
}

// TestRunner_Execute_CheckpointRetryAdjusts verifies counter adjusted on retry (AC8).
func TestRunner_Execute_CheckpointRetryAdjusts(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, twoTasksWithGate)

	// 3 executions: task1 first attempt, task1 retry, task2
	scenario := testutil.Scenario{
		Name: "checkpoint-retry-adjust",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "cra-001"},
			{Type: "execute", ExitCode: 0, SessionID: "cra-002"},
			{Type: "execute", ExitCode: 0, SessionID: "cra-003"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"a1", "a2"}, [2]string{"b1", "b2"}, [2]string{"c1", "c2"},
		),
	}

	cfg := testConfig(tmpDir, 5)
	cfg.GatesEnabled = true
	cfg.GatesCheckpoint = 2

	// ReviewFn: progressively marks each open task as [x]
	reviewCount := 0
	reviewFn := progressiveReviewFn(tasksPath, &reviewCount)

	// Gate sequence: 1st call (task 1, GATE trigger) = retry, 2nd call (task 1 re-done, GATE) = approve
	// 3rd call should be checkpoint at completedTasks=2 for task 2
	gp := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionRetry, Feedback: ""},
			{Action: config.ActionApprove},
			{Action: config.ActionApprove},
		},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        reviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil, got: %v", err)
	}

	// AC8: gate called 3 times:
	// 1. task 1 (GATE, completedTasks=1) → retry → completedTasks back to 0
	// 2. task 1 re-done (GATE, completedTasks=1) → approve
	// 3. task 2 (checkpoint, completedTasks=2) → approve
	if gp.calls != 3 {
		t.Errorf("GatePromptFn calls = %d, want 3", gp.calls)
	}

	// 1st call: GATE only (completedTasks=1, not divisible by 2)
	if len(gp.taskTexts) >= 1 {
		if !strings.Contains(gp.taskTexts[0], "[GATE]") {
			t.Errorf("call 1 text = %q, want containing '[GATE]'", gp.taskTexts[0])
		}
		if strings.Contains(gp.taskTexts[0], "(checkpoint every") {
			t.Errorf("call 1 text = %q, should NOT have checkpoint (count=1)", gp.taskTexts[0])
		}
	}

	// 2nd call: GATE only again (completedTasks=1 after decrement+re-increment)
	if len(gp.taskTexts) >= 2 {
		if !strings.Contains(gp.taskTexts[1], "[GATE]") {
			t.Errorf("call 2 text = %q, want containing '[GATE]'", gp.taskTexts[1])
		}
		if strings.Contains(gp.taskTexts[1], "(checkpoint every") {
			t.Errorf("call 2 text = %q, should NOT have checkpoint (count=1)", gp.taskTexts[1])
		}
	}

	// 3rd call: checkpoint (completedTasks=2, divisible by 2)
	if len(gp.taskTexts) >= 3 {
		if !strings.Contains(gp.taskTexts[2], "(checkpoint every 2)") {
			t.Errorf("call 3 text = %q, want containing '(checkpoint every 2)'", gp.taskTexts[2])
		}
	}

	// Review called 3 times (task1, task1-retry, task2)
	if reviewCount != 3 {
		t.Errorf("ReviewFn count = %d, want 3", reviewCount)
	}
}

// TestRunner_Execute_GateRetryRevertError verifies revert failure propagation (AC6 error path).
func TestRunner_Execute_GateRetryRevertError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, gateOpenTask)

	scenario := testutil.Scenario{
		Name: "gate-retry-revert-err",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "reverterr-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	cfg := testConfig(tmpDir, 3)
	cfg.GatesEnabled = true

	// Empty feedback: skips InjectFeedback, goes straight to RevertTask
	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionRetry, Feedback: ""},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        cleanReviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	// ReviewFn marks task [x] then swaps TasksFile to bad path so RevertTask fails
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(tasksPath, []byte(gateOpenTaskDone), 0644)
		r.TasksFile = filepath.Join(tmpDir, "nonexistent", "sprint-tasks.md")
		return runner.ReviewResult{Clean: true}, nil
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: revert task:") {
		t.Errorf("error = %q, want containing 'runner: revert task:'", err.Error())
	}
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
}

// =============================================================================
// Story 5.5: Emergency gate tests
// =============================================================================

// TestSkipTask_Scenarios verifies SkipTask marks [ ]→[x] (AC6).
func TestSkipTask_Scenarios(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		taskDesc        string
		skipCreate      bool // true = don't create file (test read error)
		wantErr         bool
		wantErrContains string
		wantContent     string // expected file content after skip (empty = no check)
	}{
		{
			name:        "basic skip marks open task done",
			content:     "- [ ] Setup project\n- [ ] Write tests\n",
			taskDesc:    "Setup project",
			wantErr:     false,
			wantContent: "- [x] Setup project\n- [ ] Write tests\n",
		},
		{
			name:        "skip preserves other tasks unchanged",
			content:     "- [x] Done task\n- [ ] Open task\n- [ ] Another open\n",
			taskDesc:    "Open task",
			wantErr:     false,
			wantContent: "- [x] Done task\n- [x] Open task\n- [ ] Another open\n",
		},
		{
			name:            "task not found returns error",
			content:         "- [ ] Task one\n- [ ] Task two\n",
			taskDesc:        "Nonexistent task",
			wantErr:         true,
			wantErrContains: "runner: skip task: task not found: Nonexistent task",
		},
		{
			name:            "already done task not found as open",
			content:         "- [x] Already done\n",
			taskDesc:        "Already done",
			wantErr:         true,
			wantErrContains: "runner: skip task: task not found:",
		},
		{
			name:            "read error on nonexistent file",
			content:         "",
			taskDesc:        "Any task",
			skipCreate:      true,
			wantErr:         true,
			wantErrContains: "runner: skip task:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")

			if !tt.skipCreate {
				if err := os.WriteFile(tasksPath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("write tasks: %v", err)
				}
			}

			err := runner.SkipTask(tasksPath, tt.taskDesc)

			if tt.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := os.ReadFile(tasksPath)
			if err != nil {
				t.Fatalf("read result: %v", err)
			}
			if string(got) != tt.wantContent {
				t.Errorf("content:\ngot:  %q\nwant: %q", string(got), tt.wantContent)
			}
		})
	}
}

// TestRunner_Execute_EmergencyGateExecuteRetry verifies retry at execute exhaustion
// resets counter, injects feedback, runner retries (AC1, AC5).
func TestRunner_Execute_EmergencyGateExecuteRetry(t *testing.T) {
	tmpDir := t.TempDir()
	tasksContent := threeOpenTasks

	// Need enough steps: MaxIterations=1 means executeAttempts=1 triggers emergency.
	// Emergency retry resets to 0, then next attempt succeeds (commit detected).
	// 2 attempts total: attempt 1 (no commit, emergency, retry), attempt 2 (commit).
	// After retry, review clean → task 1 done. Then tasks 2,3 complete normally.
	scenario := testutil.Scenario{
		Name: "emergency-execute-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "em-001"}, // attempt 1: no commit
			{Type: "execute", ExitCode: 0, SessionID: "em-002"}, // attempt 2: commit (after retry)
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, tasksContent, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs(
				[2]string{"aaa", "aaa"}, // attempt 1: same SHA → no commit → emergency
				[2]string{"aaa", "bbb"}, // attempt 2: different SHA → commit
			),
		})
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true

	emergencyCount := 0
	emergencyText := ""
	r.EmergencyGatePromptFn = func(_ context.Context, text string) (*config.GateDecision, error) {
		emergencyCount++
		emergencyText = text
		return &config.GateDecision{
			Action:   config.ActionRetry,
			Feedback: "fix the build",
		}, nil
	}

	// ReviewFn captures intermediate file state to verify feedback injection,
	// then marks all done for loop exit.
	reviewCount := 0
	var feedbackCaptured string
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		// Capture file state on first review call (after emergency retry + feedback inject)
		if reviewCount == 1 {
			data, readErr := os.ReadFile(r.TasksFile)
			if readErr == nil {
				feedbackCaptured = string(data)
			}
		}
		// Mark all done for loop exit
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(r.TasksFile, []byte(allDoneTasks), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if emergencyCount != 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 1", emergencyCount)
	}
	if !strings.Contains(emergencyText, "execute attempts exhausted") {
		t.Errorf("emergency text = %q, want containing 'execute attempts exhausted'", emergencyText)
	}
	if !strings.Contains(emergencyText, "1/1") {
		t.Errorf("emergency text = %q, want containing '1/1'", emergencyText)
	}
	if !strings.Contains(emergencyText, "Task one") {
		t.Errorf("emergency text = %q, want containing 'Task one'", emergencyText)
	}
	// Verify feedback was injected with correct format (captured before review overwrite)
	if !strings.Contains(feedbackCaptured, config.FeedbackPrefix) {
		t.Errorf("tasks file missing FeedbackPrefix %q\ncaptured: %s", config.FeedbackPrefix, feedbackCaptured)
	}
	if !strings.Contains(feedbackCaptured, "fix the build") {
		t.Errorf("tasks file missing injected feedback 'fix the build'\ncaptured: %s", feedbackCaptured)
	}
	if reviewCount < 1 {
		t.Errorf("reviewCount = %d, want >= 1", reviewCount)
	}
}

// TestRunner_Execute_EmergencyGateExecuteSkip verifies skip at execute exhaustion
// marks task [x] and proceeds to next task (AC1, AC6).
func TestRunner_Execute_EmergencyGateExecuteSkip(t *testing.T) {
	tmpDir := t.TempDir()

	// 2 tasks: task 1 always fails (no commit), task 2 succeeds.
	scenario := testutil.Scenario{
		Name: "emergency-execute-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "skip-001"}, // task 1: no commit
			{Type: "execute", ExitCode: 0, SessionID: "skip-002"}, // task 2: commit
		},
	}

	twoTasks := "# Sprint Tasks\n\n- [ ] Task one\n- [ ] Task two\n"
	r, _ := setupRunnerIntegration(t, tmpDir, twoTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs(
				[2]string{"aaa", "aaa"}, // task 1: no commit → emergency
				[2]string{"aaa", "bbb"}, // task 2: commit
			),
		})
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true

	emergencyCount := 0
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		emergencyCount++
		return &config.GateDecision{Action: config.ActionSkip}, nil
	}

	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if emergencyCount != 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 1", emergencyCount)
	}
	// Verify task 1 was marked [x] via SkipTask
	content, err := os.ReadFile(r.TasksFile)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	if !strings.Contains(string(content), "- [x] Task one") {
		t.Errorf("task 1 not marked done via SkipTask\ngot: %s", string(content))
	}
}

// TestRunner_Execute_EmergencyGateExecuteQuit verifies quit at execute exhaustion
// returns wrapped GateDecision error (AC1, AC7).
func TestRunner_Execute_EmergencyGateExecuteQuit(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "emergency-execute-quit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "quit-001"},
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs([2]string{"aaa", "aaa"}), // no commit
		})
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true

	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionQuit}, nil
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: emergency gate:") {
		t.Errorf("error = %q, want containing 'runner: emergency gate:'", err.Error())
	}
	// Verify GateDecision can be extracted
	var gd *config.GateDecision
	if !errors.As(err, &gd) {
		t.Fatalf("errors.As(err, *GateDecision): want true, got false; err = %v", err)
	}
	if gd.Action != config.ActionQuit {
		t.Errorf("GateDecision.Action = %q, want %q", gd.Action, config.ActionQuit)
	}
}

// TestRunner_Execute_EmergencyGateReviewRetry verifies retry at review cycles exhaustion
// resets counter, injects feedback, runner retries (AC2, AC5).
func TestRunner_Execute_EmergencyGateReviewRetry(t *testing.T) {
	tmpDir := t.TempDir()

	// MaxReviewIterations=1: after 1 non-clean review, emergency fires.
	// Retry resets reviewCycles to 0. Next iteration: execute succeeds, review clean.
	scenario := testutil.Scenario{
		Name: "emergency-review-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "rev-001"}, // execute 1
			{Type: "execute", ExitCode: 0, SessionID: "rev-002"}, // execute 2 (after review retry)
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs(
				[2]string{"aaa", "bbb"}, // execute 1: commit
				[2]string{"bbb", "ccc"}, // execute 2: commit
			),
		})
	r.Cfg.MaxReviewIterations = 1
	r.Cfg.GatesEnabled = true

	emergencyCount := 0
	emergencyText := ""
	r.EmergencyGatePromptFn = func(_ context.Context, text string) (*config.GateDecision, error) {
		emergencyCount++
		emergencyText = text
		return &config.GateDecision{
			Action:   config.ActionRetry,
			Feedback: "check test coverage",
		}, nil
	}

	reviewCount := 0
	var feedbackCaptured string
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		if reviewCount == 1 {
			return runner.ReviewResult{Clean: false}, nil // non-clean → triggers emergency
		}
		// Capture file state on second review call (after emergency retry + feedback inject)
		if reviewCount == 2 {
			data, readErr := os.ReadFile(r.TasksFile)
			if readErr == nil {
				feedbackCaptured = string(data)
			}
		}
		// After retry: mark all done and return clean
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(r.TasksFile, []byte(allDoneTasks), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if emergencyCount != 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 1", emergencyCount)
	}
	if !strings.Contains(emergencyText, "review cycles exhausted") {
		t.Errorf("emergency text = %q, want containing 'review cycles exhausted'", emergencyText)
	}
	if !strings.Contains(emergencyText, "1/1") {
		t.Errorf("emergency text = %q, want containing '1/1'", emergencyText)
	}
	if !strings.Contains(emergencyText, "Task one") {
		t.Errorf("emergency text = %q, want containing 'Task one'", emergencyText)
	}
	// Verify feedback was injected (captured before review overwrite)
	if !strings.Contains(feedbackCaptured, config.FeedbackPrefix) {
		t.Errorf("tasks file missing FeedbackPrefix %q\ncaptured: %s", config.FeedbackPrefix, feedbackCaptured)
	}
	if !strings.Contains(feedbackCaptured, "check test coverage") {
		t.Errorf("tasks file missing injected feedback 'check test coverage'\ncaptured: %s", feedbackCaptured)
	}
	if reviewCount < 2 {
		t.Errorf("reviewCount = %d, want >= 2 (1 non-clean + 1 clean after retry)", reviewCount)
	}
}

// TestRunner_Execute_EmergencyGateReviewSkip verifies skip at review cycles exhaustion
// marks task [x] and proceeds to next task (AC2, AC6).
func TestRunner_Execute_EmergencyGateReviewSkip(t *testing.T) {
	tmpDir := t.TempDir()

	twoTasks := "# Sprint Tasks\n\n- [ ] Task one\n- [ ] Task two\n"

	// Task 1: execute succeeds, review non-clean, emergency skip.
	// Task 2: execute succeeds, review clean → all done.
	scenario := testutil.Scenario{
		Name: "emergency-review-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "rskip-001"}, // task 1
			{Type: "execute", ExitCode: 0, SessionID: "rskip-002"}, // task 2
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, twoTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs(
				[2]string{"aaa", "bbb"}, // task 1: commit
				[2]string{"bbb", "ccc"}, // task 2: commit
			),
		})
	r.Cfg.MaxReviewIterations = 1
	r.Cfg.GatesEnabled = true

	emergencyCount := 0
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		emergencyCount++
		return &config.GateDecision{Action: config.ActionSkip}, nil
	}

	reviewCount := 0
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		if reviewCount == 1 {
			return runner.ReviewResult{Clean: false}, nil // task 1: non-clean → emergency
		}
		// task 2: mark all done, clean
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(r.TasksFile, []byte("# Sprint Tasks\n\n- [x] Task one\n- [x] Task two\n"), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if emergencyCount != 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 1", emergencyCount)
	}
	// Verify task 1 was marked [x] via SkipTask
	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	if !strings.Contains(string(content), "- [x] Task one") {
		t.Errorf("task 1 not marked done via SkipTask\ngot: %s", string(content))
	}
}

// TestRunner_Execute_EmergencyGateReviewQuit verifies quit at review cycles exhaustion
// returns wrapped GateDecision error (AC2, AC7).
func TestRunner_Execute_EmergencyGateReviewQuit(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "emergency-review-quit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "rquit-001"},
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}), // commit
		})
	r.Cfg.MaxReviewIterations = 1
	r.Cfg.GatesEnabled = true

	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionQuit}, nil
	}

	reviewCount := 0
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		return runner.ReviewResult{Clean: false}, nil // non-clean → triggers emergency
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: emergency gate:") {
		t.Errorf("error = %q, want containing 'runner: emergency gate:'", err.Error())
	}
	// Verify GateDecision can be extracted
	var gd *config.GateDecision
	if !errors.As(err, &gd) {
		t.Fatalf("errors.As(err, *GateDecision): want true, got false; err = %v", err)
	}
	if gd.Action != config.ActionQuit {
		t.Errorf("GateDecision.Action = %q, want %q", gd.Action, config.ActionQuit)
	}
	if reviewCount != 1 {
		t.Errorf("reviewCount = %d, want 1", reviewCount)
	}
}

// TestRunner_Execute_EmergencyGateError verifies EmergencyGatePromptFn error propagation (AC1, AC2).
func TestRunner_Execute_EmergencyGateError(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "emergency-gate-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gerr-001"},
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs([2]string{"aaa", "aaa"}), // no commit → emergency
		})
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true

	gateErr := fmt.Errorf("stdin closed")
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return nil, gateErr
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: emergency gate:") {
		t.Errorf("error = %q, want containing 'runner: emergency gate:'", err.Error())
	}
	if !strings.Contains(err.Error(), "stdin closed") {
		t.Errorf("error = %q, want containing inner cause 'stdin closed'", err.Error())
	}
}

// TestRunner_Execute_EmergencyGateDisabled verifies original behavior when gates disabled (AC3).
func TestRunner_Execute_EmergencyGateDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "emergency-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "dis-001"},
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario,
		&testutil.MockGitClient{
			HeadCommits: headCommitPairs([2]string{"aaa", "aaa"}), // no commit
		})
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = false // gates disabled

	emergencyCalled := false
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		emergencyCalled = true
		return &config.GateDecision{Action: config.ActionSkip}, nil
	}

	err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "execute attempts exhausted") {
		t.Errorf("error = %q, want containing 'execute attempts exhausted'", err.Error())
	}
	if emergencyCalled {
		t.Error("EmergencyGatePromptFn should NOT be called when gates disabled")
	}
}

// =============================================================================
// Story 6.4: RealReview Knowledge Tests
// =============================================================================

// TestRealReview_FindingsWriteLessons verifies ValidateNewLessons is called
// after a review that produces findings (non-clean review).
// AC: #2 — Go post-validates review-written lessons.
func TestRealReview_FindingsWriteLessons(t *testing.T) {
	tmpDir := t.TempDir()

	oneTask := "- [ ] Fix auth bug\n"
	findingsContent := "## [HIGH] Auth bypass\n- **ЧТО не так** — missing validation\n"

	scenario := testutil.Scenario{
		Name: "findings-review",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "review",
				ExitCode:   0,
				SessionID:  "rev-1",
				WriteFiles: map[string]string{"review-findings.md": findingsContent},
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)

	tasksPath := writeTasksFile(t, tmpDir, oneTask)

	kw := &trackingKnowledgeWriter{}
	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		TasksFile: tasksPath,
		Knowledge: kw,
	}

	result, err := runner.RealReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RealReview: unexpected error: %v", err)
	}
	if result.Clean {
		t.Error("RealReview: want non-clean result, got clean")
	}

	// AC: ValidateNewLessons called exactly once
	if kw.validateLessonsCount != 1 {
		t.Errorf("ValidateNewLessons count = %d, want 1", kw.validateLessonsCount)
	}
	// Source must be "review"
	if len(kw.validateLessonsData) < 1 {
		t.Fatal("ValidateNewLessons: no data recorded")
	}
	if kw.validateLessonsData[0].Source != "review" {
		t.Errorf("ValidateNewLessons Source = %q, want %q", kw.validateLessonsData[0].Source, "review")
	}
	// BudgetLimit passed through
	if kw.validateLessonsData[0].BudgetLimit != cfg.LearningsBudget {
		t.Errorf("ValidateNewLessons BudgetLimit = %d, want %d", kw.validateLessonsData[0].BudgetLimit, cfg.LearningsBudget)
	}
}

// TestRealReview_CleanNoLessons verifies ValidateNewLessons is NOT called
// on a clean review (no findings, task marked [x]).
// AC: #3 — Clean review does NOT write lessons.
func TestRealReview_CleanNoLessons(t *testing.T) {
	tmpDir := t.TempDir()

	oneTask := "- [ ] Implement feature X\n"
	markedTask := "- [x] Implement feature X\n"

	scenario := testutil.Scenario{
		Name: "clean-review",
		Steps: []testutil.ScenarioStep{
			{
				Type:        "review",
				ExitCode:    0,
				SessionID:   "rev-1",
				WriteFiles:  map[string]string{"sprint-tasks.md": markedTask},
				DeleteFiles: []string{"review-findings.md"},
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)

	tasksPath := writeTasksFile(t, tmpDir, oneTask)

	kw := &trackingKnowledgeWriter{}
	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		TasksFile: tasksPath,
		Knowledge: kw,
	}

	result, err := runner.RealReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RealReview: unexpected error: %v", err)
	}
	if !result.Clean {
		t.Error("RealReview: want clean result, got non-clean")
	}

	// AC: ValidateNewLessons NOT called on clean review
	if kw.validateLessonsCount != 0 {
		t.Errorf("ValidateNewLessons count = %d, want 0 (clean review)", kw.validateLessonsCount)
	}
}

// TestRealReview_SnapshotDiff verifies LEARNINGS.md snapshot is taken before
// session and passed to ValidateNewLessons with correct content.
// AC: #2 — Go diffs LEARNINGS.md (snapshot vs current).
func TestRealReview_SnapshotDiff(t *testing.T) {
	tmpDir := t.TempDir()

	existingLearnings := "## testing: assertion-quality [review, runner/runner_test.go:42]\nExisting lesson content here.\n"
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte(existingLearnings), 0644); err != nil {
		t.Fatalf("write LEARNINGS.md: %v", err)
	}

	oneTask := "- [ ] Fix auth bug\n"
	findingsContent := "## [HIGH] Auth bypass\n- **ЧТО не так** — missing validation\n"

	scenario := testutil.Scenario{
		Name: "snapshot-review",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "review",
				ExitCode:   0,
				SessionID:  "rev-1",
				WriteFiles: map[string]string{"review-findings.md": findingsContent},
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)

	tasksPath := writeTasksFile(t, tmpDir, oneTask)

	kw := &trackingKnowledgeWriter{}
	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		TasksFile: tasksPath,
		Knowledge: kw,
	}

	result, err := runner.RealReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RealReview: unexpected error: %v", err)
	}
	if result.Clean {
		t.Error("RealReview: want non-clean result, got clean")
	}

	// Verify snapshot content matches pre-session LEARNINGS.md
	if kw.validateLessonsCount != 1 {
		t.Fatalf("ValidateNewLessons count = %d, want 1", kw.validateLessonsCount)
	}
	data := kw.validateLessonsData[0]
	if data.Snapshot != existingLearnings {
		t.Errorf("ValidateNewLessons Snapshot = %q, want %q", data.Snapshot, existingLearnings)
	}
	if data.Source != "review" {
		t.Errorf("ValidateNewLessons Source = %q, want %q", data.Source, "review")
	}
}

// TestRealReview_ValidateLessonsError verifies error propagation when
// ValidateNewLessons fails after a non-clean review.
func TestRealReview_ValidateLessonsError(t *testing.T) {
	tmpDir := t.TempDir()

	oneTask := "- [ ] Fix auth bug\n"
	findingsContent := "## [HIGH] Auth bypass\n- **ЧТО не так** — missing validation\n"

	scenario := testutil.Scenario{
		Name: "validate-error-review",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "review",
				ExitCode:   0,
				SessionID:  "rev-1",
				WriteFiles: map[string]string{"review-findings.md": findingsContent},
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)

	tasksPath := writeTasksFile(t, tmpDir, oneTask)

	kw := &trackingKnowledgeWriter{
		validateLessonsErr: errors.New("validation failed"),
	}
	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		TasksFile: tasksPath,
		Knowledge: kw,
	}

	_, err := runner.RealReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RealReview: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: review: validate lessons:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: review: validate lessons:")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error = %q, want containing inner error %q", err.Error(), "validation failed")
	}
}

// TestRealReview_LearningsReadError verifies that a non-NotExist error on
// LEARNINGS.md read returns a wrapped error. buildKnowledgeReplacements reads
// LEARNINGS.md before the snapshot code, so the error surfaces with
// "runner: review: build knowledge:" prefix.
func TestRealReview_LearningsReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create LEARNINGS.md as a directory to trigger non-NotExist read error
	learningsDir := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("create LEARNINGS.md dir: %v", err)
	}

	oneTask := "- [ ] Fix auth bug\n"
	tasksPath := writeTasksFile(t, tmpDir, oneTask)

	kw := &trackingKnowledgeWriter{}
	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		TasksFile: tasksPath,
		Knowledge: kw,
	}

	_, err := runner.RealReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RealReview: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: review: build knowledge:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: review: build knowledge:")
	}
	if !strings.Contains(err.Error(), "read learnings:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "read learnings:")
	}
	if kw.validateLessonsCount != 0 {
		t.Errorf("ValidateNewLessons count = %d, want 0 (error before session)", kw.validateLessonsCount)
	}
}

// --- Story 6.5a: Distillation Trigger Tests ---

// TestRunner_Execute_BudgetUnderLimit verifies no distillation when LEARNINGS.md is under
// soft threshold (100 lines < 150). (AC: Scenario 1)
func TestRunner_Execute_BudgetUnderLimit(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 100)
	// Pre-seed state: cooldown met (counter=10, lastDistill=0)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-under-limit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	td := &trackingDistillFunc{}
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if td.count != 0 {
		t.Errorf("DistillFn count = %d, want 0 (budget under limit)", td.count)
	}
}

// TestRunner_Execute_DistillationTrigger verifies distillation is called when budget
// exceeds soft threshold and cooldown is met. (AC: Scenario 2)
func TestRunner_Execute_DistillationTrigger(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	// Pre-seed state: cooldown met (counter=10, lastDistill=0, diff=10 >= 5)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-trigger",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	td := &trackingDistillFunc{}
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1 (budget over limit, cooldown met)", td.count)
	}
}

// TestRunner_Execute_CooldownNotMet verifies no distillation when cooldown check fails.
// counter=15, lastDistill=12, diff=3 < 5. (AC: Scenario 3)
func TestRunner_Execute_CooldownNotMet(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	// Pre-seed state: cooldown NOT met (counter=15, lastDistill=12, diff=3 < 5)
	writeDistillState(t, tmpDir, 15, 12)

	scenario := testutil.Scenario{
		Name: "distill-cooldown-not-met",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	td := &trackingDistillFunc{}
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	// Counter increments: 15+1=16, lastDistill=12, diff=4 < 5 → still not met
	if td.count != 0 {
		t.Errorf("DistillFn count = %d, want 0 (cooldown not met)", td.count)
	}
}

// TestRunner_Execute_DistillationBadFormatRetry verifies ErrBadFormat gets one free retry,
// and if retry succeeds, no gate. (AC: Scenario 5, H4)
func TestRunner_Execute_DistillationBadFormatRetry(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-bad-format-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	// First call: ErrBadFormat, second call: success (nil)
	td := &trackingDistillFunc{errs: []error{fmt.Errorf("parse: %w", runner.ErrBadFormat)}}
	r.DistillFn = td.fn

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	r.GatePromptFn = gp.fn
	r.Cfg.GatesEnabled = true

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if td.count != 2 {
		t.Errorf("DistillFn count = %d, want 2 (first=ErrBadFormat, second=success)", td.count)
	}
	// Gate should NOT be called (free retry succeeded)
	if gp.count != 0 {
		t.Errorf("GatePromptFn count = %d, want 0 (no gate on successful retry)", gp.count)
	}
}

// TestRunner_Execute_DistillationBadFormatRetryFails verifies ErrBadFormat retry that also
// fails triggers human gate. (AC: Scenario 5)
func TestRunner_Execute_DistillationBadFormatRetryFails(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-bad-format-retry-fails",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	// Both calls fail: first ErrBadFormat, retry also ErrBadFormat
	td := &trackingDistillFunc{errs: []error{
		fmt.Errorf("parse: %w", runner.ErrBadFormat),
		fmt.Errorf("parse again: %w", runner.ErrBadFormat),
	}}
	r.DistillFn = td.fn

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	r.GatePromptFn = gp.fn
	r.Cfg.GatesEnabled = true

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if td.count != 2 {
		t.Errorf("DistillFn count = %d, want 2 (first=ErrBadFormat, retry=ErrBadFormat)", td.count)
	}
	// Gate should be called (retry failed)
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1 (gate on retry failure)", gp.count)
	}
	if gp.taskText == "" || !strings.Contains(gp.taskText, "distillation failed:") {
		t.Errorf("gate text = %q, want containing %q", gp.taskText, "distillation failed:")
	}
}

// TestRunner_Execute_DistillationFailureHumanGate verifies non-format error triggers
// immediate human gate with skip action. (AC: Scenario 4)
func TestRunner_Execute_DistillationFailureHumanGate(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-failure-gate",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	// Non-format error → immediate gate (no free retry)
	td := &trackingDistillFunc{errs: []error{errors.New("I/O timeout")}}
	r.DistillFn = td.fn

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	r.GatePromptFn = gp.fn
	r.Cfg.GatesEnabled = true

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1 (no free retry for non-format error)", td.count)
	}
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1 (gate on non-format error)", gp.count)
	}
	if !strings.Contains(gp.taskText, "I/O timeout") {
		t.Errorf("gate text = %q, want containing error %q", gp.taskText, "I/O timeout")
	}
	if !strings.Contains(gp.taskText, "160/200") {
		t.Errorf("gate text = %q, want containing budget info %q", gp.taskText, "160/200")
	}
}

// TestRunner_Execute_MissingLearnings verifies no distillation when LEARNINGS.md absent.
// BudgetCheck returns NearLimit=false for missing files. (AC: Scenario 6)
func TestRunner_Execute_MissingLearnings(t *testing.T) {
	tmpDir := t.TempDir()
	// No LEARNINGS.md file
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-no-learnings",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	td := &trackingDistillFunc{}
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if td.count != 0 {
		t.Errorf("DistillFn count = %d, want 0 (no LEARNINGS.md)", td.count)
	}
}

// TestRunner_Execute_MonotonicCounterPersist verifies MonotonicTaskCounter is incremented
// and persisted after clean review. (AC: H1)
func TestRunner_Execute_MonotonicCounterPersist(t *testing.T) {
	tmpDir := t.TempDir()
	// Start with counter=5, lastDistill=5 (cooldown not met, so distill won't trigger)
	writeDistillState(t, tmpDir, 5, 5)

	scenario := testutil.Scenario{
		Name: "distill-counter-persist",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	td := &trackingDistillFunc{}
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Verify counter was incremented and persisted
	statePath := filepath.Join(tmpDir, ".ralph", "distill-state.json")
	loaded, loadErr := runner.LoadDistillState(statePath)
	if loadErr != nil {
		t.Fatalf("LoadDistillState: %v", loadErr)
	}
	if loaded.MonotonicTaskCounter != 6 {
		t.Errorf("MonotonicTaskCounter = %d, want 6 (5+1)", loaded.MonotonicTaskCounter)
	}
	if loaded.LastDistillTask != 5 {
		t.Errorf("LastDistillTask = %d, want 5 (unchanged, no distill)", loaded.LastDistillTask)
	}
}

// TestRunner_Execute_DistillSuccess_UpdatesLastDistillTask verifies LastDistillTask is
// updated to MonotonicTaskCounter after successful distillation. (AC: Scenario 2)
func TestRunner_Execute_DistillSuccess_UpdatesLastDistillTask(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	// Pre-seed: counter=10, lastDistill=0 → cooldown met
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-success-updates-state",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	td := &trackingDistillFunc{} // returns nil (success)
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Verify LastDistillTask updated
	statePath := filepath.Join(tmpDir, ".ralph", "distill-state.json")
	loaded, loadErr := runner.LoadDistillState(statePath)
	if loadErr != nil {
		t.Fatalf("LoadDistillState: %v", loadErr)
	}
	// Counter was 10, incremented to 11 → LastDistillTask should be 11
	if loaded.MonotonicTaskCounter != 11 {
		t.Errorf("MonotonicTaskCounter = %d, want 11 (10+1)", loaded.MonotonicTaskCounter)
	}
	if loaded.LastDistillTask != 11 {
		t.Errorf("LastDistillTask = %d, want 11 (updated on success)", loaded.LastDistillTask)
	}
}

// TestRunner_Execute_DistillationRetry5 verifies gate "retry 5" path via
// Feedback=="5" triggers up to 5 retries. (AC: Scenario 4, retry 5 times)
func TestRunner_Execute_DistillationRetry5(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-retry-5",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	// All calls fail: initial + free retry + 5 gate retries = 7 calls total
	td := &trackingDistillFunc{errs: []error{
		errors.New("crash"),   // initial call → immediate gate (non-format)
		errors.New("crash 2"), // gate retry 1
		errors.New("crash 3"), // gate retry 2
		errors.New("crash 4"), // gate retry 3
		errors.New("crash 5"), // gate retry 4
		errors.New("crash 6"), // gate retry 5
	}}
	r.DistillFn = td.fn

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionRetry, Feedback: "5"},
	}
	r.GatePromptFn = gp.fn
	r.Cfg.GatesEnabled = true

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	// 1 initial call + 5 gate retries = 6 total (non-format error, no free retry)
	if td.count != 6 {
		t.Errorf("DistillFn count = %d, want 6 (1 initial + 5 gate retries)", td.count)
	}
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
}

// TestRunner_Execute_DistillationFailureGatesDisabled verifies distillation failure
// with gates disabled logs warning and continues (non-fatal). (AC: Scenario 4 edge case)
func TestRunner_Execute_DistillationFailureGatesDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "distill-failure-no-gates",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.Cfg.GatesEnabled = false // gates disabled
	td := &trackingDistillFunc{errs: []error{errors.New("crash")}}
	r.DistillFn = td.fn

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (distillation should be non-fatal)", err)
	}
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1", td.count)
	}
}

