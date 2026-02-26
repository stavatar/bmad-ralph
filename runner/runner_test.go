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
		name                   string
		sessionID              string
		scenarioSteps          []testutil.ScenarioStep
		knowledgeErr           error
		wantErr                bool
		wantErrContains        string
		wantErrContainsInner   string
		wantWriteProgressCount int
		wantSessionInvoked     bool
		checkArgs              func(t *testing.T, args []string)
		checkData              func(t *testing.T, kw *trackingKnowledgeWriter)
	}{
		{
			name:      "valid session ID",
			sessionID: "abc-123",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "resumed-001"},
			},
			wantErr:                false,
			wantWriteProgressCount: 1,
			wantSessionInvoked:     true,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				assertArgsContainFlagValue(t, args, "--resume", "abc-123")
				assertArgsFlagAbsent(t, args, "-p")
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
			},
		},
		{
			name:                   "empty session ID",
			sessionID:              "",
			wantErr:                false,
			wantWriteProgressCount: 0,
			wantSessionInvoked:     false,
		},
		{
			name:      "session execute error",
			sessionID: "err-session",
			// No scenario steps: use nonexistent binary to trigger exec error
			wantErr:                true,
			wantErrContains:        "runner: resume extraction: execute:",
			wantErrContainsInner:   "/nonexistent/binary",
			wantWriteProgressCount: 0,
			wantSessionInvoked:     false, // binary not found, no invocation logged
		},
		{
			name:      "write progress error",
			sessionID: "wp-err-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "wp-001"},
			},
			knowledgeErr:           errors.New("write failed"),
			wantErr:                true,
			wantErrContains:        "runner: resume extraction: write progress:",
			wantErrContainsInner:   "write failed",
			wantWriteProgressCount: 1,
			wantSessionInvoked:     true,
		},
		{
			name:      "mutation asymmetry preserved",
			sessionID: "mut-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "mut-001"},
			},
			wantErr:                false,
			wantWriteProgressCount: 1,
			wantSessionInvoked:     true,
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

			kw := &trackingKnowledgeWriter{writeProgressErr: tc.knowledgeErr}

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
