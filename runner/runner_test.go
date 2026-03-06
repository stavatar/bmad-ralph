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
	"github.com/bmad-ralph/bmad-ralph/session"
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

			_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

			_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

// TestRunner_Execute_MaxReviewCyclesExhausted verifies that review exhaust continues
// to next task instead of aborting the entire run (BUG-2). Metrics record the task
// as "error" with the last known HEAD SHA (MINOR-2).
// Table-driven to prove configurable max (AC2, AC5).
func TestRunner_Execute_MaxReviewCyclesExhausted(t *testing.T) {
	tests := []struct {
		name                 string
		maxReviewIter        int
		wantReviewCount      int
		wantHeadCommitCount  int
		wantHealthCheckCount int
	}{
		{
			name:                 "max 3 default",
			maxReviewIter:        3,
			wantReviewCount:      3,
			wantHeadCommitCount:  6,
			wantHealthCheckCount: 1, // startup only
		},
		{
			name:                 "max 5 configurable",
			maxReviewIter:        5,
			wantReviewCount:      5,
			wantHeadCommitCount:  10,
			wantHealthCheckCount: 1, // startup only
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

			cfg := testConfig(tmpDir, 1) // single iteration
			cfg.MaxReviewIterations = tt.maxReviewIter

			re := &trackingResumeExtract{}
			ts := &trackingSleep{}
			reviewCount := 0
			mc := runner.NewMetricsCollector("test-review-exhaust", nil)

			r := &runner.Runner{
				Cfg:       cfg,
				Git:       mock,
				TasksFile: tasksPath,
				Metrics:   mc,
				ReviewFn: func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
					reviewCount++
					return runner.ReviewResult{Clean: false}, nil // always non-clean
				},
				ResumeExtractFn: re.fn,
				SleepFn:         ts.fn,
				Knowledge:       &runner.NoOpKnowledgeWriter{},
			}

			rm, err := r.Execute(context.Background())
			// BUG-2: Execute no longer returns error on review exhaust
			if err != nil {
				t.Fatalf("Execute: want nil error, got %v", err)
			}
			if reviewCount != tt.wantReviewCount {
				t.Errorf("reviewCount = %d, want %d", reviewCount, tt.wantReviewCount)
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
			// MINOR-2: last known SHA recorded
			lastSHA := fmt.Sprintf("%03d", tt.maxReviewIter)
			if rm.Tasks[0].CommitSHA != lastSHA {
				t.Errorf("Tasks[0].CommitSHA = %q, want %q (last known HEAD)", rm.Tasks[0].CommitSHA, lastSHA)
			}
			// Verify error recorded in metrics
			if rm.Tasks[0].Errors == nil {
				t.Fatal("Tasks[0].Errors = nil, want non-nil")
			}
			if rm.Tasks[0].Errors.TotalErrors != 1 {
				t.Errorf("Tasks[0].Errors.TotalErrors = %d, want 1", rm.Tasks[0].Errors.TotalErrors)
			}
			if len(rm.Tasks[0].Errors.Categories) < 1 || rm.Tasks[0].Errors.Categories[0] != "review_exhaust" {
				t.Errorf("Tasks[0].Errors.Categories = %v, want [review_exhaust]", rm.Tasks[0].Errors.Categories)
			}
			if rm.TasksFailed != 1 {
				t.Errorf("TasksFailed = %d, want 1", rm.TasksFailed)
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

	_, err := r.Execute(context.Background())
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

	_, err = r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

			_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(ctx)
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

// TestResumeExtraction_HappyPath verifies ResumeExtraction success cases:
// valid session (args, data, counts), empty sessionID (no-op), and mutation asymmetry.
func TestResumeExtraction_HappyPath(t *testing.T) {
	tests := []struct {
		name                     string
		sessionID                string
		scenarioSteps            []testutil.ScenarioStep
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
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 1,
			wantSessionInvoked:       true,
			checkArgs: func(t *testing.T, args []string) {
				t.Helper()
				assertArgsContainFlagValue(t, args, "--resume", "abc-123")
				assertArgsContainFlag(t, args, "-p")
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
				if len(kw.validateLessonsData) != 1 {
					t.Fatalf("validateLessonsData len = %d, want 1", len(kw.validateLessonsData))
				}
				if kw.validateLessonsData[0].Source != "resume-extraction" {
					t.Errorf("LessonsData.Source = %q, want %q", kw.validateLessonsData[0].Source, "resume-extraction")
				}
			},
		},
		{
			name:                     "empty session ID is a no-op",
			sessionID:                "",
			wantWriteProgressCount:   0,
			wantValidateLessonsCount: 0,
			wantSessionInvoked:       false,
		},
		{
			name:      "mutation asymmetry preserved",
			sessionID: "mut-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "mut-001"},
			},
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 1,
			wantSessionInvoked:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
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
				_, stateDir = testutil.SetupMockClaude(t, testutil.Scenario{
					Name:  "resume-" + tc.name,
					Steps: tc.scenarioSteps,
				})
			}

			kw := &trackingKnowledgeWriter{}
			err = runner.ResumeExtraction(context.Background(), cfg, kw, nil, nil, tc.sessionID)
			if err != nil {
				t.Fatalf("ResumeExtraction: unexpected error: %v", err)
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

			afterContent, readErr := os.ReadFile(tasksPath)
			if readErr != nil {
				t.Fatalf("read tasks after: %v", readErr)
			}
			if strings.Count(string(afterContent), "[x]") != beforeCheckCount {
				t.Errorf("mutation asymmetry violated: [x] count changed after ResumeExtraction")
			}
		})
	}
}

// TestResumeExtraction_ErrorPaths verifies ResumeExtraction error returns:
// session exec failure, WriteProgress failure, and ValidateNewLessons failure.
func TestResumeExtraction_ErrorPaths(t *testing.T) {
	tests := []struct {
		name                     string
		sessionID                string
		scenarioSteps            []testutil.ScenarioStep
		knowledgeErr             error
		validateLessonsErr       error
		wantErrContains          string
		wantErrContainsInner     string
		wantWriteProgressCount   int
		wantValidateLessonsCount int
	}{
		{
			name:      "session execute error",
			sessionID: "err-session",
			// No scenario steps: nonexistent binary triggers exec error.
			wantErrContains:          "runner: resume extraction: execute:",
			wantErrContainsInner:     "/nonexistent/binary",
			wantWriteProgressCount:   0,
			wantValidateLessonsCount: 0,
		},
		{
			name:      "write progress error",
			sessionID: "wp-err-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "wp-001"},
			},
			knowledgeErr:             errors.New("write failed"),
			wantErrContains:          "runner: resume extraction: write progress:",
			wantErrContainsInner:     "write failed",
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 0, // fails before validate
		},
		{
			name:      "validate lessons error",
			sessionID: "vl-err-session",
			scenarioSteps: []testutil.ScenarioStep{
				{Type: "execute", ExitCode: 0, SessionID: "vl-001"},
			},
			validateLessonsErr:       errors.New("validation failed"),
			wantErrContains:          "runner: resume extraction: validate lessons:",
			wantErrContainsInner:     "validation failed",
			wantWriteProgressCount:   1,
			wantValidateLessonsCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := testConfig(tmpDir, 1)
			cfg.ModelExecute = "opus"

			if len(tc.scenarioSteps) > 0 {
				testutil.SetupMockClaude(t, testutil.Scenario{
					Name:  "resume-err-" + tc.name,
					Steps: tc.scenarioSteps,
				})
			} else {
				cfg.ClaudeCommand = "/nonexistent/binary"
			}

			kw := &trackingKnowledgeWriter{
				writeProgressErr:   tc.knowledgeErr,
				validateLessonsErr: tc.validateLessonsErr,
			}

			err := runner.ResumeExtraction(context.Background(), cfg, kw, nil, nil, tc.sessionID)
			if err == nil {
				t.Fatal("ResumeExtraction: want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error = %q, want containing %q", err.Error(), tc.wantErrContains)
			}
			if !strings.Contains(err.Error(), tc.wantErrContainsInner) {
				t.Errorf("inner error = %q, want containing %q", err.Error(), tc.wantErrContainsInner)
			}
			if kw.writeProgressCount != tc.wantWriteProgressCount {
				t.Errorf("WriteProgress count = %d, want %d", kw.writeProgressCount, tc.wantWriteProgressCount)
			}
			if kw.validateLessonsCount != tc.wantValidateLessonsCount {
				t.Errorf("ValidateNewLessons count = %d, want %d", kw.validateLessonsCount, tc.wantValidateLessonsCount)
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

	err := runner.ResumeExtraction(context.Background(), cfg, kw, nil, nil, "parse-err-session")
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

	err := runner.ResumeExtraction(context.Background(), cfg, kw, nil, nil, "snap-session")
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

	err := runner.ResumeExtraction(context.Background(), cfg, kw, nil, nil, "nc-session")
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

	err := runner.ResumeExtraction(context.Background(), cfg, kw, nil, nil, "snap-read-err")
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

// TestResumeExtraction_RecordsMetrics verifies BUG-11: resume session metrics
// (tokens, cost) are merged into the current task via MetricsCollector.
func TestResumeExtraction_RecordsMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)
	_ = tasksPath

	cfg := testConfig(tmpDir, 1)
	cfg.ModelExecute = "opus"

	_, _ = testutil.SetupMockClaude(t, testutil.Scenario{
		Name:  "resume-metrics",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "rm-001"},
		},
	})

	pricing := map[string]config.Pricing{
		"opus": {InputPer1M: 15.0, OutputPer1M: 75.0, CachePer1M: 1.5},
	}
	mc := runner.NewMetricsCollector("run-resume-metrics", pricing)
	mc.StartTask("task-with-resume")

	kw := &trackingKnowledgeWriter{}
	err := runner.ResumeExtraction(context.Background(), cfg, kw, nil, mc, "resume-session")
	if err != nil {
		t.Fatalf("ResumeExtraction: unexpected error: %v", err)
	}

	mc.FinishTask("completed", "abc123")
	rm := mc.Finish()

	if len(rm.Tasks) != 1 {
		t.Fatalf("Tasks count = %d, want 1", len(rm.Tasks))
	}
	// Resume session should have been recorded — Sessions >= 1
	if rm.Tasks[0].Sessions < 1 {
		t.Errorf("Tasks[0].Sessions = %d, want >= 1 (resume session recorded)", rm.Tasks[0].Sessions)
	}
	// Run-level sessions should also reflect the resume
	if rm.TotalSessions < 1 {
		t.Errorf("TotalSessions = %d, want >= 1", rm.TotalSessions)
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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
		wantFindingsNil bool // true = expect Findings == nil
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "clean review - task done no findings",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n- [ ] Task two\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: nil,
			wantClean:       true,
			wantFindingsNil: true,
		},
		{
			name:            "clean review - empty findings file",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &emptyFindings,
			wantClean:       true,
			wantFindingsNil: true,
		},
		{
			name:            "clean review - whitespace-only findings",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &whitespaceFindings,
			wantClean:       true,
			wantFindingsNil: true,
		},
		{
			name:            "with findings - task not done",
			tasksContent:    "# Sprint Tasks\n\n- [ ] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &realFindings,
			wantClean:       false,
			wantFindingsNil: true, // realFindings has ## not ### — no severity headers match
		},
		{
			name:            "task done but findings non-empty",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: &realFindings,
			wantClean:       false,
			wantFindingsNil: true, // realFindings has ## not ### — no severity headers match
		},
		{
			name:            "no change no findings - session failed silently",
			tasksContent:    "# Sprint Tasks\n\n- [ ] Task one\n",
			currentTaskText: "- [ ] Task one",
			findingsContent: nil,
			wantClean:       false,
			wantFindingsNil: true,
		},
		{
			name:            "bare task text without checkbox prefix",
			tasksContent:    "# Sprint Tasks\n\n- [x] Task one\n",
			currentTaskText: "Task one",
			findingsContent: nil,
			wantClean:       true,
			wantFindingsNil: true,
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
			if tt.wantFindingsNil && rr.Findings != nil {
				t.Errorf("Findings = %v, want nil", rr.Findings)
			}
		})
	}
}

// =============================================================================
// Story 7.4: Review Enrichment — Severity Findings (AC: #1-#6)
// =============================================================================

// TestDetermineReviewOutcome_FindingsParsing verifies severity parsing from review-findings.md.
func TestDetermineReviewOutcome_FindingsParsing(t *testing.T) {
	tmpDir := t.TempDir()
	tasksFile := writeTasksFile(t, tmpDir, "# Sprint Tasks\n\n- [ ] Task one\n")

	// Use the test fixture with mixed severities
	fixtureData, err := os.ReadFile("testdata/review-findings-with-severity.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	findingsPath := filepath.Join(tmpDir, "review-findings.md")
	if err := os.WriteFile(findingsPath, fixtureData, 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	rr, err := runner.DetermineReviewOutcome(tasksFile, "- [ ] Task one", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.Clean {
		t.Errorf("Clean = true, want false")
	}
	if len(rr.Findings) != 5 {
		t.Fatalf("len(Findings) = %d, want 5", len(rr.Findings))
	}

	// Verify all 5 findings: severity and description
	wantFindings := []struct {
		severity    string
		description string
	}{
		{"HIGH", "Missing error assertion"},
		{"MEDIUM", "Stale doc comment"},
		{"LOW", "Unused test fixture"},
		{"CRITICAL", "SQL injection in query builder"},
		{"MEDIUM", "DRY violation in test helpers"},
	}
	for i, wf := range wantFindings {
		if rr.Findings[i].Severity != wf.severity {
			t.Errorf("Findings[%d].Severity = %q, want %q", i, rr.Findings[i].Severity, wf.severity)
		}
		if rr.Findings[i].Description != wf.description {
			t.Errorf("Findings[%d].Description = %q, want %q", i, rr.Findings[i].Description, wf.description)
		}
	}

	// Verify severity counts: 1 CRITICAL, 1 HIGH, 2 MEDIUM, 1 LOW
	counts := map[string]int{}
	for _, f := range rr.Findings {
		counts[f.Severity]++
	}
	wantCounts := map[string]int{"CRITICAL": 1, "HIGH": 1, "MEDIUM": 2, "LOW": 1}
	for sev, want := range wantCounts {
		if got := counts[sev]; got != want {
			t.Errorf("count(%s) = %d, want %d", sev, got, want)
		}
	}

	// Verify File and Line remain zero-value (not extracted by regex)
	if rr.Findings[0].File != "" {
		t.Errorf("Findings[0].File = %q, want empty", rr.Findings[0].File)
	}
	if rr.Findings[0].Line != 0 {
		t.Errorf("Findings[0].Line = %d, want 0", rr.Findings[0].Line)
	}
}

// TestDetermineReviewOutcome_CleanNoFindings verifies clean review has nil Findings.
func TestDetermineReviewOutcome_CleanNoFindings(t *testing.T) {
	tmpDir := t.TempDir()
	tasksFile := writeTasksFile(t, tmpDir, "# Sprint Tasks\n\n- [x] Task one\n")

	rr, err := runner.DetermineReviewOutcome(tasksFile, "- [ ] Task one", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rr.Clean {
		t.Errorf("Clean = false, want true")
	}
	if rr.Findings != nil {
		t.Errorf("Findings = %v, want nil", rr.Findings)
	}
}

// TestDetermineReviewOutcome_MalformedFindings verifies content without severity headers
// results in Clean=false and Findings=nil (AC4).
func TestDetermineReviewOutcome_MalformedFindings(t *testing.T) {
	tmpDir := t.TempDir()
	tasksFile := writeTasksFile(t, tmpDir, "# Sprint Tasks\n\n- [ ] Task one\n")

	// Content without ### [SEVERITY] headers
	malformed := "Some review content\nwithout severity headers\nJust plain text.\n"
	findingsPath := filepath.Join(tmpDir, "review-findings.md")
	if err := os.WriteFile(findingsPath, []byte(malformed), 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	rr, err := runner.DetermineReviewOutcome(tasksFile, "- [ ] Task one", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.Clean {
		t.Errorf("Clean = true, want false (malformed findings file)")
	}
	if rr.Findings != nil {
		t.Errorf("Findings = %v, want nil (no parseable severity headers)", rr.Findings)
	}
}

// BackwardCompat canary removed: identical to CleanNoFindings (M1 review finding).

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

			_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil (nil guard skips gate), got: %v", err)
	}
}

// TestRunner_Execute_GatePromptError verifies gate prompt error propagation (AC3 error path).
func TestRunner_Execute_GatePromptError(t *testing.T) {
	r, gp := setupGateTest(t, gateOpenTask, true)
	gp.decision = nil
	gp.err = context.Canceled

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

			_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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
	r.Cfg.RunID = "distill-gate-run"
	r.Metrics = runner.NewMetricsCollector("distill-gate-run", nil)
	// Non-format error → immediate gate (no free retry)
	td := &trackingDistillFunc{errs: []error{errors.New("I/O timeout")}}
	r.DistillFn = td.fn

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	r.GatePromptFn = gp.fn
	r.Cfg.GatesEnabled = true

	_, err := r.Execute(context.Background())
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

	// Story 7.6 L1: verify distillation gate records GateStats in MetricsCollector.
	rm := r.Metrics.Finish()
	if len(rm.Tasks) < 1 {
		t.Fatalf("len(Tasks) = %d, want >= 1", len(rm.Tasks))
	}
	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil after distillation gate, want non-nil")
	}
	if g.TotalPrompts != 1 {
		t.Errorf("TotalPrompts = %d, want 1", g.TotalPrompts)
	}
	if g.Skips != 1 {
		t.Errorf("Skips = %d, want 1 (distillation skip)", g.Skips)
	}
	if g.LastAction != "skip" {
		t.Errorf("LastAction = %q, want %q", g.LastAction, "skip")
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

	_, err := r.Execute(context.Background())
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

// TestRunner_Execute_DistillationGateError verifies that when GatePromptFn itself
// returns an error during distillation failure handling, execution continues (non-fatal).
func TestRunner_Execute_DistillationGateError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name:  "distill-gate-error",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "exec-001"}},
	}
	mock := &testutil.MockGitClient{HeadCommits: headCommitPairs([2]string{"aaa", "bbb"})}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.Cfg.GatesEnabled = true
	r.DistillFn = (&trackingDistillFunc{errs: []error{errors.New("distill failed")}}).fn
	r.GatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return nil, errors.New("gate prompt error")
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (gate error should be non-fatal)", err)
	}
}

// TestRunner_Execute_DistillationGateQuit verifies that ActionQuit from the
// distillation gate does not propagate as a fatal error — execution continues.
func TestRunner_Execute_DistillationGateQuit(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name:  "distill-gate-quit",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "exec-001"}},
	}
	mock := &testutil.MockGitClient{HeadCommits: headCommitPairs([2]string{"aaa", "bbb"})}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.Cfg.GatesEnabled = true
	r.DistillFn = (&trackingDistillFunc{errs: []error{errors.New("distill failed")}}).fn
	r.GatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionQuit}, nil
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (distillation quit should be non-fatal)", err)
	}
}

// TestRunner_Execute_DistillationGateApprove verifies that ActionApprove from the
// distillation gate (treated as skip — default branch) continues without retrying.
func TestRunner_Execute_DistillationGateApprove(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name:  "distill-gate-approve",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "exec-001"}},
	}
	mock := &testutil.MockGitClient{HeadCommits: headCommitPairs([2]string{"aaa", "bbb"})}

	td := &trackingDistillFunc{errs: []error{errors.New("distill failed")}}
	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.Cfg.GatesEnabled = true
	r.DistillFn = td.fn
	r.GatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionApprove}, nil
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	// ActionApprove → default branch → no retry; DistillFn called only once (initial)
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1 (no retries on approve)", td.count)
	}
}

// TestRunner_Execute_DistillationRetrySucceeds verifies that when DistillFn
// fails on the first call but succeeds on the gate retry, state is persisted
// and execution continues normally.
func TestRunner_Execute_DistillationRetrySucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name:  "distill-retry-succeeds",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "exec-001"}},
	}
	mock := &testutil.MockGitClient{HeadCommits: headCommitPairs([2]string{"aaa", "bbb"})}

	// First call fails, second (retry-1) succeeds.
	td := &trackingDistillFunc{errs: []error{errors.New("first attempt failed")}}
	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.Cfg.GatesEnabled = true
	r.DistillFn = td.fn
	r.GatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionRetry, Feedback: ""}, nil
	}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	// 1 initial failure + 1 successful retry = 2 total calls
	if td.count != 2 {
		t.Errorf("DistillFn count = %d, want 2 (1 fail + 1 retry success)", td.count)
	}

	// State should be persisted with updated LastDistillTask
	statePath := filepath.Join(tmpDir, ".ralph", "distill-state.json")
	loaded, loadErr := runner.LoadDistillState(statePath)
	if loadErr != nil {
		t.Fatalf("LoadDistillState: %v", loadErr)
	}
	if loaded.LastDistillTask != loaded.MonotonicTaskCounter {
		t.Errorf("LastDistillTask = %d, want == MonotonicTaskCounter (%d) after retry success",
			loaded.LastDistillTask, loaded.MonotonicTaskCounter)
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

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (distillation should be non-fatal)", err)
	}
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1", td.count)
	}
}

func TestRunner_Execute_RecoverDistillationError(t *testing.T) {
	t.Parallel()
	// .ralph/distill-intent.json as a non-empty directory → os.ReadFile returns EISDIR
	// → ReadIntentFile → RecoverDistillation returns error → Execute returns "runner: startup:".
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	ralphDir := filepath.Join(tmpDir, ".ralph")
	// Create distill-intent.json as a non-empty directory (child prevents removal).
	intentDir := filepath.Join(ralphDir, "distill-intent.json")
	if err := os.MkdirAll(filepath.Join(intentDir, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(tmpDir, 1)
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             &testutil.MockGitClient{},
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error for RecoverDistillation failure, got nil")
	}
	if !strings.Contains(err.Error(), "runner: startup:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: startup:")
	}
	if !strings.Contains(err.Error(), "runner: distill: recovery:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill: recovery:")
	}
}

func TestRunner_Execute_BuildKnowledgeError(t *testing.T) {
	t.Parallel()
	// LEARNINGS.md as directory → buildKnowledgeReplacements returns non-ErrNotExist error
	// → Execute returns "runner: startup: runner: build knowledge: read learnings:".
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// Create LEARNINGS.md as a directory so os.ReadFile returns EISDIR.
	if err := os.MkdirAll(filepath.Join(tmpDir, "LEARNINGS.md"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(tmpDir, 1)
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             &testutil.MockGitClient{},
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error for buildKnowledgeReplacements failure, got nil")
	}
	if !strings.Contains(err.Error(), "runner: startup:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: startup:")
	}
	if !strings.Contains(err.Error(), "runner: build knowledge: read learnings:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: build knowledge: read learnings:")
	}
}

func TestRunner_Execute_DistillStateLoadError(t *testing.T) {
	t.Parallel()
	// Invalid distill-state.json → LoadDistillState json.Unmarshal fails
	// → Execute returns "runner: startup: runner: distill state: load:".
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ralphDir, "distill-state.json"), []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(tmpDir, 1)
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             &testutil.MockGitClient{},
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error for LoadDistillState failure, got nil")
	}
	if !strings.Contains(err.Error(), "runner: startup:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: startup:")
	}
	if !strings.Contains(err.Error(), "runner: distill state: load:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: distill state: load:")
	}
}

func TestRunner_Execute_BudgetWarning(t *testing.T) {
	t.Parallel()
	// LEARNINGS.md with 5 lines, LearningsBudget=3 → OverBudget=true → warning printed.
	// allDoneTasks → no open tasks → Execute returns nil after budget warning.
	// Covers lines 478-484 (ratio calculation and fprintf).
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, allDoneTasks)

	learnings := strings.Repeat("line\n", 5) // 5 newlines → lines=5 >= budget=3
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(learnings), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(tmpDir, 1)
	cfg.LearningsBudget = 3 // 5 lines >= 3 → OverBudget=true, Limit=3>0 → ratio=1

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             &testutil.MockGitClient{},
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

func TestRunner_Execute_CodeIndexerPresent(t *testing.T) {
	t.Parallel()
	// r.CodeIndexer != nil → enters DetectSerena block (line 463-465).
	// allDoneTasks → no open tasks → Execute returns nil.
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, allDoneTasks)

	cfg := testConfig(tmpDir, 1)
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             &testutil.MockGitClient{},
		TasksFile:       tasksPath,
		ReviewFn:        fatalReviewFn(t),
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
		CodeIndexer:     &runner.NoOpCodeIndexerDetector{},
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

// TestRunner_Execute_NilKnowledge_NoPanic verifies that Execute does not panic
// when Knowledge is nil (no KnowledgeWriter injected).
func TestRunner_Execute_NilKnowledge_NoPanic(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "nil-knowledge",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nk-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Knowledge = nil // explicitly nil — must not panic
	r.Cfg.MaxIterations = 1

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

// TestRunner_Execute_SkippedTask_NoCounterIncrement verifies that emergency-skipped
// tasks do not increment MonotonicTaskCounter (Bug #2 fix).
func TestRunner_Execute_SkippedTask_NoCounterIncrement(t *testing.T) {
	tmpDir := t.TempDir()
	// Pre-seed counter=5
	writeDistillState(t, tmpDir, 5, 5)

	scenario := testutil.Scenario{
		Name: "skip-no-counter",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "skip-cnt-001"}, // task 1: no commit → emergency skip
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "aaa"}), // no commit → emergency
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200

	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionSkip}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Verify counter was NOT incremented (still 5, not 6)
	statePath := filepath.Join(tmpDir, ".ralph", "distill-state.json")
	loaded, loadErr := runner.LoadDistillState(statePath)
	if loadErr != nil {
		t.Fatalf("LoadDistillState: %v", loadErr)
	}
	if loaded.MonotonicTaskCounter != 5 {
		t.Errorf("MonotonicTaskCounter = %d, want 5 (unchanged after skip)", loaded.MonotonicTaskCounter)
	}
}

// TestRunner_Execute_SkippedTask_NoValidation verifies that emergency-skipped
// tasks do not call ValidateNewLessons (Bug #1 fix — wasSkipped bypasses validation).
func TestRunner_Execute_SkippedTask_NoValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// 2 tasks: task 1 fails and gets skipped, task 2 succeeds.
	scenario := testutil.Scenario{
		Name: "skip-no-validate",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "snv-001"}, // task 1: no commit → emergency skip
			{Type: "execute", ExitCode: 0, SessionID: "snv-002"}, // task 2: commit → success
		},
	}

	twoTasks := "# Sprint Tasks\n\n- [ ] Task one\n- [ ] Task two\n"

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"}, // task 1: no commit → emergency
			[2]string{"aaa", "bbb"}, // task 2: commit
		),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, twoTasks, scenario, mock)
	r.Cfg.MaxIterations = 2 // need 2 iterations: skip task 1 + process task 2
	r.Cfg.GatesEnabled = true

	kw := &trackingKnowledgeWriter{}
	r.Knowledge = kw

	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionSkip}, nil
	}

	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// ValidateNewLessons should be called only for task 2 (not skipped task 1)
	if kw.validateLessonsCount != 1 {
		t.Errorf("ValidateNewLessons count = %d, want 1 (only non-skipped task)", kw.validateLessonsCount)
	}
}

// TestRunner_Execute_NormalTask_IncrementsCounter verifies that a normal (non-skipped)
// task still increments MonotonicTaskCounter and calls ValidateNewLessons.
func TestRunner_Execute_NormalTask_IncrementsCounter(t *testing.T) {
	tmpDir := t.TempDir()
	writeDistillState(t, tmpDir, 5, 5)

	scenario := testutil.Scenario{
		Name: "normal-counter",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nc-001"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200

	kw := &trackingKnowledgeWriter{}
	r.Knowledge = kw

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Counter should be incremented (5 → 6)
	statePath := filepath.Join(tmpDir, ".ralph", "distill-state.json")
	loaded, loadErr := runner.LoadDistillState(statePath)
	if loadErr != nil {
		t.Fatalf("LoadDistillState: %v", loadErr)
	}
	if loaded.MonotonicTaskCounter != 6 {
		t.Errorf("MonotonicTaskCounter = %d, want 6 (5+1)", loaded.MonotonicTaskCounter)
	}

	// ValidateNewLessons should be called for the normal task
	if kw.validateLessonsCount != 1 {
		t.Errorf("ValidateNewLessons count = %d, want 1", kw.validateLessonsCount)
	}
}

// TestRunner_Execute_DiffStatsIntegration verifies DiffStats is called on commit detection (AC6),
// results are logged, and MetricsCollector.RecordGitDiff populates TaskMetrics.Diff.
func TestRunner_Execute_DiffStatsIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "diff-stats-integration",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "ds-1"},
		},
	}
	diffResult := &runner.DiffStats{
		FilesChanged: 3,
		Insertions:   42,
		Deletions:    7,
		Packages:     []string{"config", "runner"},
	}
	git := &testutil.MockGitClient{
		HeadCommits:      headCommitPairs([2]string{"aaa", "bbb"}),
		DiffStatsResults: []*runner.DiffStats{diffResult},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.RunID = "ds-run"
	r.Metrics = runner.NewMetricsCollector("ds-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Verify DiffStats was called
	if git.DiffStatsCount != 1 {
		t.Errorf("DiffStatsCount = %d, want 1", git.DiffStatsCount)
	}

	// Verify MetricsCollector received diff stats via RecordGitDiff
	if rm == nil {
		t.Fatal("RunMetrics is nil")
	}
	if len(rm.Tasks) < 1 {
		t.Fatalf("Tasks len = %d, want >= 1", len(rm.Tasks))
	}
	diff := rm.Tasks[0].Diff
	if diff == nil {
		t.Fatal("Tasks[0].Diff is nil, want non-nil from RecordGitDiff")
	}
	if diff.FilesChanged != 3 {
		t.Errorf("Diff.FilesChanged = %d, want 3", diff.FilesChanged)
	}
	if diff.Insertions != 42 {
		t.Errorf("Diff.Insertions = %d, want 42", diff.Insertions)
	}
	if diff.Deletions != 7 {
		t.Errorf("Diff.Deletions = %d, want 7", diff.Deletions)
	}
	if len(diff.Packages) != 2 || diff.Packages[0] != "config" || diff.Packages[1] != "runner" {
		t.Errorf("Diff.Packages = %v, want [config runner]", diff.Packages)
	}
}

// TestRunner_Execute_DiffStatsError verifies that DiffStats error is best-effort (NFR24):
// error is logged as warning but execution continues successfully.
func TestRunner_Execute_DiffStatsError(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "diff-stats-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "dse-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits:     headCommitPairs([2]string{"aaa", "bbb"}),
		DiffStatsErrors: []error{fmt.Errorf("mock diff error")},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: want nil error (best-effort DiffStats), got: %v", err)
	}

	// Verify DiffStats was attempted
	if git.DiffStatsCount != 1 {
		t.Errorf("DiffStatsCount = %d, want 1", git.DiffStatsCount)
	}
}

// TestRunner_Execute_DiffStatsWithoutMetrics verifies DiffStats works when Metrics is nil.
func TestRunner_Execute_DiffStatsWithoutMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "diff-stats-no-metrics",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "dsn-1"},
		},
	}
	diffResult := &runner.DiffStats{
		FilesChanged: 1,
		Insertions:   5,
		Deletions:    0,
		Packages:     []string{"."},
	}
	git := &testutil.MockGitClient{
		HeadCommits:      headCommitPairs([2]string{"aaa", "bbb"}),
		DiffStatsResults: []*runner.DiffStats{diffResult},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	// r.Metrics intentionally left nil

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// DiffStats should still be called even without MetricsCollector
	if git.DiffStatsCount != 1 {
		t.Errorf("DiffStatsCount = %d, want 1", git.DiffStatsCount)
	}
}

// TestRunner_Execute_GatePromptIncludesCost verifies gate prompt text contains
// "Cost so far: $X.XX" when MetricsCollector has pricing configured (AC6).
// Covers checkpoint gate. Emergency gates (execute/review exhaustion) and distill gate
// use identical pattern — covered by code inspection, not dedicated integration tests.
func TestRunner_Execute_GatePromptIncludesCost(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "gate-cost-display",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gc-1",
				Model: "claude-sonnet-4-20250514",
				Usage: map[string]int{"input_tokens": 1000, "output_tokens": 500, "cache_read_input_tokens": 200}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 1
	r.Cfg.RunID = "gate-cost-run"

	pricing := map[string]config.Pricing{
		"claude-sonnet-4-20250514": {InputPer1M: 3.0, OutputPer1M: 15.0, CachePer1M: 0.30},
	}
	r.Metrics = runner.NewMetricsCollector("gate-cost-run", pricing)

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	r.GatePromptFn = gp.fn

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if gp.count < 1 {
		t.Fatalf("gate prompt not called, want >= 1")
	}
	if !strings.Contains(gp.taskText, "Cost so far: $") {
		t.Errorf("gate prompt text missing cost string, got: %q", gp.taskText)
	}
}

// =============================================================================
// Story 7.5: Stuck Detection tests (AC1-AC6)
// =============================================================================

// TestRunner_Execute_StuckDetectionFeedbackInjected verifies that 2 consecutive no-commit
// attempts trigger InjectFeedback with stuck message (AC2, AC6).
func TestRunner_Execute_StuckDetectionFeedbackInjected(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "stuck-feedback",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "stuck-001"},
			{Type: "execute", ExitCode: 0, SessionID: "stuck-002"},
			{Type: "execute", ExitCode: 0, SessionID: "stuck-003"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "aaa"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.StuckThreshold = 2
	r.Metrics = runner.NewMetricsCollector("stuck-run", nil) // exercise RecordRetry("stuck") positive path

	rm, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true; err = %v", err)
	}
	if rm == nil {
		t.Fatal("RunMetrics: want non-nil when Metrics set, got nil")
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	taskContent := string(content)
	if !strings.Contains(taskContent, "No commit in last 2 attempts") {
		t.Errorf("tasks file missing 'No commit in last 2 attempts', got:\n%s", taskContent)
	}
	if !strings.Contains(taskContent, "Consider a different approach") {
		t.Errorf("tasks file missing 'Consider a different approach', got:\n%s", taskContent)
	}
	if !strings.Contains(taskContent, config.FeedbackPrefix) {
		t.Errorf("tasks file missing FeedbackPrefix %q", config.FeedbackPrefix)
	}
}

// TestRunner_Execute_StuckCounterResetsOnCommit verifies that consecutive no-commit counter
// resets to 0 on successful commit (AC3). Cross-task scope: task 1 has 1 no-commit then commit
// (reset), task 2 needs 2 fresh no-commits to trigger stuck — proves counter was reset.
func TestRunner_Execute_StuckCounterResetsOnCommit(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "stuck-reset",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sr-001"}, // task 1: no commit
			{Type: "execute", ExitCode: 0, SessionID: "sr-002"}, // task 1: commit
			{Type: "execute", ExitCode: 0, SessionID: "sr-003"}, // task 2: no commit
			{Type: "execute", ExitCode: 0, SessionID: "sr-004"}, // task 2: no commit (stuck)
			{Type: "execute", ExitCode: 0, SessionID: "sr-005"}, // task 2: no commit (maxiter)
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "aaa", // task 1 attempt 1: no commit (count=1)
			"aaa", "bbb", // task 1 attempt 2: commit (count→0)
			"bbb", "bbb", // task 2 attempt 1: no commit (count=1)
			"bbb", "bbb", // task 2 attempt 2: no commit (count=2 → stuck)
			"bbb", "bbb", // task 2 attempt 3: no commit (count=3 → maxiter)
		},
	}
	reviewCount := 0
	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.StuckThreshold = 2

	// Custom reviewFn: marks only "Task one" done, leaving "Task two" open
	oneDoneContent := "# Sprint Tasks\n\n## Epic 1: Foundation\n\n- [x] Task one\n- [ ] Task two\n- [ ] Task three\n"
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(r.TasksFile, []byte(oneDoneContent), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("errors.Is(err, ErrMaxRetries): want true; err = %v", err)
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	taskContent := string(content)
	// "2 attempts" not "3" — proves counter reset at commit
	if !strings.Contains(taskContent, "No commit in last 2 attempts") {
		t.Errorf("want 'No commit in last 2 attempts' (counter reset at commit), got:\n%s", taskContent)
	}
	// Exactly 1 occurrence of threshold feedback — counter was reset by commit, so
	// stuck fires once at task 2 attempt 2, not earlier from task 1.
	if cnt := strings.Count(taskContent, "No commit in last 2 attempts"); cnt != 1 {
		t.Errorf("strings.Count('No commit in last 2 attempts') = %d, want 1 (counter reset proof)", cnt)
	}
	if reviewCount < 1 {
		t.Errorf("reviewCount = %d, want >= 1 (task 1 committed)", reviewCount)
	}
}

// TestRunner_Execute_StuckDisabledWhenThresholdZero verifies stuck detection is disabled
// when StuckThreshold == 0 (AC4). No feedback injected despite multiple no-commits.
func TestRunner_Execute_StuckDisabledWhenThresholdZero(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "stuck-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sd-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sd-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sd-003"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "aaa"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.StuckThreshold = 0 // disabled

	_, err := r.Execute(context.Background())
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Fatalf("errors.Is(err, ErrMaxRetries): want true; err = %v", err)
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	if strings.Contains(string(content), "No commit in last") {
		t.Errorf("stuck disabled (threshold=0) but feedback was injected:\n%s", string(content))
	}
}

// TestRunner_Execute_StuckDoesNotTerminateLoop verifies stuck detection does NOT replace
// MaxIterations — loop continues after stuck feedback until MaxIterations exhausted (AC5).
func TestRunner_Execute_StuckDoesNotTerminateLoop(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "stuck-continues",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sc-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sc-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sc-003"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "aaa"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.StuckThreshold = 2

	_, err := r.Execute(context.Background())
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Fatalf("errors.Is(err, ErrMaxRetries): want true; err = %v", err)
	}

	// All 3 HeadCommit pairs consumed — stuck didn't stop loop
	if mock.HeadCommitCount != 6 {
		t.Errorf("HeadCommitCount = %d, want 6 (3 before + 3 after)", mock.HeadCommitCount)
	}
	if !strings.Contains(err.Error(), "3/3") {
		t.Errorf("error message: want '3/3' (MaxIterations enforced), got %q", err.Error())
	}
}

// TestRunner_Execute_StuckNilMetricsNoPanic verifies stuck detection works when Metrics is nil
// — no panic from RecordRetry nil guard (AC2).
func TestRunner_Execute_StuckNilMetricsNoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "stuck-nil-metrics",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "snm-001"},
			{Type: "execute", ExitCode: 0, SessionID: "snm-002"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 2
	r.Cfg.StuckThreshold = 2
	r.Metrics = nil

	_, err := r.Execute(context.Background())
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Fatalf("errors.Is(err, ErrMaxRetries): want true; err = %v", err)
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	if !strings.Contains(string(content), "No commit in last 2 attempts") {
		t.Errorf("stuck feedback should be injected even with nil Metrics")
	}
}

// TestRunner_Execute_StuckExecErrorDoesNotAffectCounter verifies that exec errors (non-zero exit)
// do NOT increment or reset the consecutive no-commit counter. Exec-error path skips headAfter
// check entirely, so counter stays unchanged.
func TestRunner_Execute_StuckExecErrorDoesNotAffectCounter(t *testing.T) {
	tmpDir := t.TempDir()
	// Sequence: no-commit → exec-error → no-commit → stuck fires (count=2)
	scenario := testutil.Scenario{
		Name: "stuck-exec-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "see-001"}, // no commit
			{Type: "execute", ExitCode: 1, SessionID: "see-002"}, // exec error
			{Type: "execute", ExitCode: 0, SessionID: "see-003"}, // no commit
			{Type: "execute", ExitCode: 0, SessionID: "see-004"}, // no commit (maxiter)
		},
	}
	// HeadCommit calls: step 1 (before+after), step 2 (before only),
	// step 3 (before+after), step 4 (before+after)
	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "aaa", // step 1: before+after, no commit (count=1)
			"aaa",        // step 2: before only (exec error skips after)
			"aaa", "aaa", // step 3: before+after, no commit (count=2 → stuck)
			"aaa", "aaa", // step 4: before+after, no commit (count=3 → maxiter)
		},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 4
	r.Cfg.StuckThreshold = 2

	_, err := r.Execute(context.Background())
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Fatalf("errors.Is(err, ErrMaxRetries): want true; err = %v", err)
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	// "2 attempts" proves exec-error didn't increment counter
	if !strings.Contains(string(content), "No commit in last 2 attempts") {
		t.Errorf("want 'No commit in last 2 attempts' (exec-error doesn't affect counter), got:\n%s", string(content))
	}
}

// TestRunner_Execute_StuckFeedbackMessageContent verifies exact feedback message format (AC6).
func TestRunner_Execute_StuckFeedbackMessageContent(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "stuck-msg",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sm-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sm-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sm-003"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "aaa"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.StuckThreshold = 2

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: expected error, got nil")
	}
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Errorf("Execute: want ErrMaxRetries, got %v", err)
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	taskContent := string(content)

	// Full feedback line format
	wantLine := config.FeedbackPrefix + " No commit in last 2 attempts. Consider a different approach."
	if !strings.Contains(taskContent, wantLine) {
		t.Errorf("feedback line mismatch\nwant substring: %q\ngot:\n%s", wantLine, taskContent)
	}

	// At step 3, counter=3, second stuck feedback should say "3 attempts"
	if !strings.Contains(taskContent, "No commit in last 3 attempts") {
		t.Errorf("want second stuck feedback 'No commit in last 3 attempts', got:\n%s", taskContent)
	}
}

// =============================================================================
// Story 7.6: Gate Analytics tests (AC1-AC4)
// =============================================================================

// TestRunner_Execute_GateAnalytics_NormalGateRecordsMetrics verifies normal gate records GateStats (AC1,AC2)
// and writes structured log with step_type=gate, action, wait_ms, task fields (AC1).
func TestRunner_Execute_GateAnalytics_NormalGateRecordsMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "gate-analytics-normal",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "ga-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 1
	r.Cfg.RunID = "gate-analytics-run"
	r.Metrics = runner.NewMetricsCollector("gate-analytics-run", nil)

	// Set up logger so we can verify structured log output (AC1).
	logr, logErr := runner.OpenRunLogger(tmpDir, "logs", "gate-analytics-run")
	if logErr != nil {
		t.Fatalf("OpenRunLogger: %v", logErr)
	}
	defer logr.Close() //nolint:errcheck
	r.Logger = logr

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	r.GatePromptFn = gp.fn

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC1: Verify structured log contains gate decision fields.
	logFiles, _ := filepath.Glob(filepath.Join(tmpDir, "logs", "ralph-*.log"))
	if len(logFiles) == 0 {
		t.Fatal("no log file found")
	}
	logData, readErr := os.ReadFile(logFiles[0])
	if readErr != nil {
		t.Fatalf("read log: %v", readErr)
	}
	logContent := string(logData)
	for _, want := range []string{"gate decision", "step_type=gate", "action=approve", "wait_ms=", "task="} {
		if !strings.Contains(logContent, want) {
			t.Errorf("log missing %q in:\n%s", want, logContent)
		}
	}

	// AC2: Verify GateStats counters.
	rm := r.Metrics.Finish()
	if len(rm.Tasks) < 1 {
		t.Fatalf("len(Tasks) = %d, want >= 1", len(rm.Tasks))
	}
	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil, want non-nil")
	}
	if g.TotalPrompts != 1 {
		t.Errorf("TotalPrompts = %d, want 1", g.TotalPrompts)
	}
	if g.Approvals != 1 {
		t.Errorf("Approvals = %d, want 1", g.Approvals)
	}
	if g.Rejections != 0 {
		t.Errorf("Rejections = %d, want 0", g.Rejections)
	}
	if g.Skips != 0 {
		t.Errorf("Skips = %d, want 0", g.Skips)
	}
	if g.TotalWaitMs < 0 {
		t.Errorf("TotalWaitMs = %d, want >= 0", g.TotalWaitMs)
	}
	if g.LastAction != "approve" {
		t.Errorf("LastAction = %q, want %q", g.LastAction, "approve")
	}
}

// TestRunner_Execute_GateAnalytics_EmergencyGateRecordsMetrics verifies emergency gate records same GateStats (AC3).
func TestRunner_Execute_GateAnalytics_EmergencyGateRecordsMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "gate-analytics-emergency",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "ga-em-1"},
			{Type: "execute", ExitCode: 0, SessionID: "ga-em-2"},
		},
	}
	// All no-commit → triggers emergency at MaxIterations
	git := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "aaa", "aaa", "aaa", "aaa"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 2
	r.Cfg.GatesEnabled = true
	r.Cfg.StuckThreshold = 0 // disable stuck to isolate emergency
	r.Cfg.RunID = "gate-em-run"
	r.Metrics = runner.NewMetricsCollector("gate-em-run", nil)

	ep := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	r.EmergencyGatePromptFn = ep.fn

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	rm := r.Metrics.Finish()
	if len(rm.Tasks) < 1 {
		t.Fatalf("len(Tasks) = %d, want >= 1", len(rm.Tasks))
	}
	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil, want non-nil after emergency gate")
	}
	if g.TotalPrompts != 1 {
		t.Errorf("TotalPrompts = %d, want 1", g.TotalPrompts)
	}
	if g.Skips != 1 {
		t.Errorf("Skips = %d, want 1 (emergency skip)", g.Skips)
	}
	if g.Approvals != 0 {
		t.Errorf("Approvals = %d, want 0", g.Approvals)
	}
	if g.Rejections != 0 {
		t.Errorf("Rejections = %d, want 0", g.Rejections)
	}
	if g.LastAction != "skip" {
		t.Errorf("LastAction = %q, want %q", g.LastAction, "skip")
	}
}

// TestRunner_Execute_GateAnalytics_NilMetricsNoPanic verifies gate recording is safe with nil Metrics (AC2 nil guard).
func TestRunner_Execute_GateAnalytics_NilMetricsNoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "gate-analytics-nil",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "ga-nil-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 1
	r.Metrics = nil // explicitly nil

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	r.GatePromptFn = gp.fn

	// Must not panic
	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if gp.count < 1 {
		t.Errorf("gate prompt not called, want >= 1")
	}
	// AC6 negative: no cost string when Metrics is nil
	if strings.Contains(gp.taskText, "Cost so far") {
		t.Errorf("gate prompt should NOT contain cost string with nil Metrics, got: %q", gp.taskText)
	}
}

// TestRunner_Execute_GateAnalytics_QuitRecordsRejection verifies quit action records Rejections counter.
func TestRunner_Execute_GateAnalytics_QuitRecordsRejection(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "gate-analytics-quit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "ga-quit-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 1
	r.Cfg.RunID = "gate-quit-run"
	r.Metrics = runner.NewMetricsCollector("gate-quit-run", nil)

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionQuit},
	}
	r.GatePromptFn = gp.fn

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: expected error for quit, got nil")
	}

	rm := r.Metrics.Finish()
	if len(rm.Tasks) < 1 {
		t.Fatalf("len(Tasks) = %d, want >= 1", len(rm.Tasks))
	}
	g := rm.Tasks[0].Gate
	if g == nil {
		t.Fatal("Gate is nil, want non-nil after quit")
	}
	if g.Rejections != 1 {
		t.Errorf("Rejections = %d, want 1", g.Rejections)
	}
	if g.Approvals != 0 {
		t.Errorf("Approvals = %d, want 0", g.Approvals)
	}
	if g.Skips != 0 {
		t.Errorf("Skips = %d, want 0", g.Skips)
	}
	if g.LastAction != "quit" {
		t.Errorf("LastAction = %q, want %q", g.LastAction, "quit")
	}
}

// --- Story 7.9: Latency + Error integration ---

// TestRunner_Execute_LatencyRecorded verifies latency breakdown is populated after normal execution (AC3,AC4).
func TestRunner_Execute_LatencyRecorded(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "latency-record",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "lat-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.RunID = "latency-run"
	r.Metrics = runner.NewMetricsCollector("latency-run", nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if rm == nil {
		t.Fatal("RunMetrics is nil, want non-nil")
	}
	if len(rm.Tasks) < 1 {
		t.Fatalf("len(Tasks) = %d, want >= 1", len(rm.Tasks))
	}
	lat := rm.Tasks[0].Latency
	if lat == nil {
		t.Fatal("Latency is nil, want non-nil")
	}
	// SessionMs should be > 0 since session.Execute was called
	if lat.SessionMs <= 0 {
		t.Errorf("SessionMs = %d, want > 0", lat.SessionMs)
	}
	// GitMs >= 0 (MockGitClient returns instantly; sub-millisecond rounds to 0)
	if lat.GitMs < 0 {
		t.Errorf("GitMs = %d, want >= 0", lat.GitMs)
	}
}

// TestRunner_Execute_NilMetricsNoPanicLatency verifies nil Metrics doesn't panic on latency/error instrumentation (AC5).
func TestRunner_Execute_NilMetricsNoPanicLatency(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "nil-metrics-latency",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nml-1"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Metrics = nil // explicitly nil

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	// No panic = pass
}

// TestRunner_Execute_ErrorRecorded verifies that execute session errors are recorded
// in metrics via RecordError (AC3, AC4).
func TestRunner_Execute_ErrorRecorded(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "error-record",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "err-1"},
			{Type: "execute", ExitCode: 0, SessionID: "err-2"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "aaa", "bbb"},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 3
	r.Cfg.RunID = "error-run"
	r.Metrics = runner.NewMetricsCollector("error-run", nil)
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	rm, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if rm == nil {
		t.Fatal("RunMetrics is nil, want non-nil")
	}
	if len(rm.Tasks) < 1 {
		t.Fatalf("len(Tasks) = %d, want >= 1", len(rm.Tasks))
	}
	errs := rm.Tasks[0].Errors
	if errs == nil {
		t.Fatal("Errors is nil, want non-nil after exit code 1")
	}
	if errs.TotalErrors < 1 {
		t.Errorf("TotalErrors = %d, want >= 1", errs.TotalErrors)
	}
	if len(errs.Categories) < 1 {
		t.Fatalf("Categories len = %d, want >= 1", len(errs.Categories))
	}
	if errs.Categories[0] == "" {
		t.Error("Categories[0] is empty, want non-empty category")
	}
}

// TestRunner_Execute_SimilarityWarnInjectsFeedback verifies that when consecutive commits
// touch similar packages, the similarity detector triggers warn and injects feedback (AC4).
func TestRunner_Execute_SimilarityWarnInjectsFeedback(t *testing.T) {
	tmpDir := t.TempDir()
	// 3 execute steps: each produces a commit with overlapping packages
	scenario := testutil.Scenario{
		Name: "sim-warn",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sw-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sw-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sw-003"},
		},
	}
	// All 3 iterations detect a commit (different hashes)
	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "bbb", // iter 1: commit detected
			"bbb", "ccc", // iter 2: commit detected
			"ccc", "ddd", // iter 3: commit detected
		},
		DiffStatsResults: []*runner.DiffStats{
			{Packages: []string{"pkg/a", "pkg/b", "pkg/c"}},
			{Packages: []string{"pkg/a", "pkg/b", "pkg/d"}},
			{Packages: []string{"pkg/a", "pkg/b", "pkg/e"}},
		},
	}

	oneDoneContent := "# Sprint Tasks\n\n## Epic 1: Foundation\n\n- [x] Task one\n- [ ] Task two\n- [ ] Task three\n"
	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.SimilarityWindow = 3
	r.Cfg.SimilarityWarn = 0.3 // low threshold to trigger warn with partial overlap
	r.Cfg.SimilarityHard = 0.95
	r.Similarity = runner.NewSimilarityDetector(3, 0.3, 0.95)

	// Capture tasks file content before ReviewFn overwrites it
	var feedbackSeen bool
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		data, _ := os.ReadFile(r.TasksFile) // Error ignored: test helper reads controlled tmpDir
		if strings.Contains(string(data), "Recent changes are very similar") {
			feedbackSeen = true
		}
		_ = os.WriteFile(r.TasksFile, []byte(oneDoneContent), 0644) // Error ignored: test helper
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	_ = err // May complete or exhaust iterations

	if !feedbackSeen {
		t.Error("similarity warn feedback was never injected into tasks file")
	}
}

// TestRunner_Execute_SimilarityHardTriggersEmergencyGate verifies that when diff packages
// are identical across window, the hard threshold triggers EmergencyGatePromptFn (AC5).
func TestRunner_Execute_SimilarityHardTriggersEmergencyGate(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "sim-hard",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sh-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sh-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sh-003"},
		},
	}
	// All commits, identical packages each time
	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "bbb",
			"bbb", "ccc",
			"ccc", "ddd",
		},
		DiffStatsResults: []*runner.DiffStats{
			{Packages: []string{"pkg/a", "pkg/b"}},
			{Packages: []string{"pkg/a", "pkg/b"}},
			{Packages: []string{"pkg/a", "pkg/b"}},
		},
	}

	oneDoneContent := "# Sprint Tasks\n\n## Epic 1: Foundation\n\n- [x] Task one\n- [ ] Task two\n- [ ] Task three\n"
	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.GatesEnabled = true
	r.Cfg.SimilarityWindow = 3
	r.Cfg.SimilarityWarn = 0.5
	r.Cfg.SimilarityHard = 0.8
	r.Similarity = runner.NewSimilarityDetector(3, 0.5, 0.8)

	gate := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionQuit},
	}
	r.EmergencyGatePromptFn = gate.fn
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		_ = os.WriteFile(r.TasksFile, []byte(oneDoneContent), 0644) // Error ignored: test helper
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error from similarity hard gate quit, got nil")
	}
	if !strings.Contains(err.Error(), "similarity gate") {
		t.Errorf("error = %q, want containing 'similarity gate'", err.Error())
	}
	if gate.count == 0 {
		t.Error("EmergencyGatePromptFn was not called for similarity hard threshold")
	}
	if gate.count > 0 {
		if !strings.Contains(gate.taskText, "Similarity loop detected") {
			t.Errorf("gate taskText = %q, want containing 'Similarity loop detected'", gate.taskText)
		}
		if !strings.Contains(gate.taskText, "score:") {
			t.Errorf("gate taskText = %q, want containing 'score:'", gate.taskText)
		}
		if !strings.Contains(gate.taskText, "Diffs are repeating") {
			t.Errorf("gate taskText = %q, want containing 'Diffs are repeating'", gate.taskText)
		}
	}
}

// TestRunner_Execute_SimilarityHardNoGateContinues verifies that when hard threshold
// triggers but GatesEnabled=false, execution continues without calling EmergencyGatePromptFn.
func TestRunner_Execute_SimilarityHardNoGateContinues(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "sim-hard-no-gate",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sng-001"},
			{Type: "execute", ExitCode: 0, SessionID: "sng-002"},
			{Type: "execute", ExitCode: 0, SessionID: "sng-003"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "bbb",
			"bbb", "ccc",
			"ccc", "ddd",
		},
		DiffStatsResults: []*runner.DiffStats{
			{Packages: []string{"pkg/same"}},
			{Packages: []string{"pkg/same"}},
			{Packages: []string{"pkg/same"}},
		},
	}

	oneDoneContent := "# Sprint Tasks\n\n## Epic 1: Foundation\n\n- [x] Task one\n- [ ] Task two\n- [ ] Task three\n"
	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.GatesEnabled = false // gates disabled
	r.Cfg.SimilarityWindow = 3
	r.Cfg.SimilarityWarn = 0.5
	r.Cfg.SimilarityHard = 0.8
	r.Similarity = runner.NewSimilarityDetector(3, 0.5, 0.8)

	gate := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionQuit},
	}
	r.EmergencyGatePromptFn = gate.fn // set but should not be called (GatesEnabled=false)
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		_ = os.WriteFile(r.TasksFile, []byte(oneDoneContent), 0644) // Error ignored: test helper
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	_ = err // May complete or exhaust iterations — no gate quit expected

	if gate.count != 0 {
		t.Errorf("EmergencyGatePromptFn called %d times, want 0 (gates disabled)", gate.count)
	}
}

// TestRunner_Execute_SimilarityDisabledWhenWindowZero verifies no similarity detection
// when SimilarityWindow == 0 (AC6). Identical packages but no detector → no feedback.
func TestRunner_Execute_SimilarityDisabledWhenWindowZero(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "sim-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "sd-001"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: []string{"aaa", "bbb"},
		DiffStatsResults: []*runner.DiffStats{
			{Packages: []string{"pkg/a", "pkg/b"}},
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.SimilarityWindow = 0 // disabled
	// Similarity field stays nil (no detector created)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	if strings.Contains(string(content), "similar") {
		t.Error("tasks file should not contain similarity feedback when disabled")
	}
}

// =============================================================================
// Story 7.7: Budget Alerts tests (AC1-AC6)
// =============================================================================

// budgetPricing returns pricing that yields $1.00 per input token for easy cost math.
func budgetPricing() map[string]config.Pricing {
	return map[string]config.Pricing{
		"test-model": {InputPer1M: 1_000_000, OutputPer1M: 0, CachePer1M: 0},
	}
}

// TestRunner_Execute_BudgetAlertDisabled verifies that BudgetMaxUSD==0 (default)
// produces no budget warnings or errors even with high cost (AC4).
func TestRunner_Execute_BudgetAlertDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "budget-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bd-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 100}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 0 // disabled (default)
	r.Cfg.RunID = "budget-disabled-run"
	r.Metrics = runner.NewMetricsCollector("budget-disabled-run", budgetPricing())

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	// No budget warning in tasks file
	content, _ := os.ReadFile(r.TasksFile)
	if strings.Contains(string(content), "Budget warning") {
		t.Error("tasks file should not contain budget warning when disabled")
	}
}

// TestRunner_Execute_BudgetAlertWarning verifies that cost at warn threshold triggers
// InjectFeedback with budget warning message, logged once per task (AC2).
func TestRunner_Execute_BudgetAlertWarning(t *testing.T) {
	tmpDir := t.TempDir()
	// Session costs $5 (5 input tokens * $1/token).
	// BudgetMaxUSD=100, BudgetWarnPct=5 → warnAt=$5.
	// After execute: $5 >= $5 warnAt → warning injected into tasks file.
	// Review captures tasks file state (with warning), then marks task done.
	scenario := testutil.Scenario{
		Name: "budget-warn",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bw-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 5}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 100.0
	r.Cfg.BudgetWarnPct = 5
	r.Cfg.RunID = "budget-warn-run"
	r.Metrics = runner.NewMetricsCollector("budget-warn-run", budgetPricing())

	// Capture tasks file content during review (before overwrite) to verify warning.
	var capturedTasksContent string
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		data, _ := os.ReadFile(r.TasksFile)
		capturedTasksContent = string(data)
		// Error ignored: test helper in controlled tmpDir
		_ = os.WriteFile(r.TasksFile, []byte(allDoneTasks), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if !strings.Contains(capturedTasksContent, "Budget warning") {
		t.Errorf("tasks file at review time should contain budget warning feedback, got: %s", capturedTasksContent)
	}
	// Verify concrete dollar amounts in warning message
	if !strings.Contains(capturedTasksContent, "$5.00") {
		t.Errorf("budget warning should contain current cost '$5.00', got: %s", capturedTasksContent)
	}
	if !strings.Contains(capturedTasksContent, "$100.00") {
		t.Errorf("budget warning should contain budget limit '$100.00', got: %s", capturedTasksContent)
	}
	// Warning injected only once per task (AC2)
	if count := strings.Count(capturedTasksContent, "Budget warning"); count != 1 {
		t.Errorf("budget warning count = %d, want 1 (once per task)", count)
	}
}

// TestRunner_Execute_BudgetExceededHardError verifies that cost exceeding budget
// without gates enabled returns a hard error (AC3).
func TestRunner_Execute_BudgetExceededHardError(t *testing.T) {
	tmpDir := t.TempDir()
	// Session costs $10 (10 input tokens). BudgetMaxUSD=5 → exceeded after first session.
	scenario := testutil.Scenario{
		Name: "budget-exceeded-hard",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "be-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	r.Cfg.GatesEnabled = false
	r.Cfg.RunID = "budget-exceeded-run"
	r.Metrics = runner.NewMetricsCollector("budget-exceeded-run", budgetPricing())

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("error = %q, want containing 'budget exceeded'", err.Error())
	}
	if !strings.Contains(err.Error(), "cost limit reached") {
		t.Errorf("error = %q, want containing 'cost limit reached'", err.Error())
	}
	if !strings.Contains(err.Error(), "$10.00") {
		t.Errorf("error = %q, want containing actual cost '$10.00'", err.Error())
	}
	if !strings.Contains(err.Error(), "$5.00") {
		t.Errorf("error = %q, want containing budget limit '$5.00'", err.Error())
	}
}

// TestRunner_Execute_BudgetExceededEmergencyGateQuit verifies budget exceeded with
// gates enabled triggers emergency gate, and quit action returns error (AC3).
func TestRunner_Execute_BudgetExceededEmergencyGateQuit(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "budget-gate-quit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bgq-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "budget-gate-quit-run"
	r.Metrics = runner.NewMetricsCollector("budget-gate-quit-run", budgetPricing())

	gate := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionQuit},
	}
	r.EmergencyGatePromptFn = gate.fn

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("error = %q, want containing 'budget exceeded'", err.Error())
	}
	if gate.count != 1 {
		t.Errorf("emergency gate count = %d, want 1", gate.count)
	}
	if !strings.Contains(gate.taskText, "budget exceeded") {
		t.Errorf("gate taskText = %q, want containing 'budget exceeded'", gate.taskText)
	}
	if !strings.Contains(err.Error(), "gate: quit") {
		t.Errorf("error = %q, want containing inner error 'gate: quit'", err.Error())
	}
}

// TestRunner_Execute_BudgetExceededEmergencyGateRetry verifies that retry action
// continues execution despite exceeded budget (AC3).
func TestRunner_Execute_BudgetExceededEmergencyGateRetry(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "budget-gate-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bgr-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "budget-gate-retry-run"
	r.Metrics = runner.NewMetricsCollector("budget-gate-retry-run", budgetPricing())

	gate := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionRetry},
	}
	r.EmergencyGatePromptFn = gate.fn

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if gate.count != 2 {
		t.Errorf("emergency gate count = %d, want 2 (once after execute, once after review)", gate.count)
	}
}

// TestRunner_Execute_BudgetExceededEmergencyGateSkip verifies that skip action
// skips the current task and continues to next (AC3).
func TestRunner_Execute_BudgetExceededEmergencyGateSkip(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "budget-gate-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bgs-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "budget-gate-skip-run"
	r.Metrics = runner.NewMetricsCollector("budget-gate-skip-run", budgetPricing())

	gate := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	r.EmergencyGatePromptFn = gate.fn

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if gate.count != 1 {
		t.Errorf("emergency gate count = %d, want 1", gate.count)
	}
	// Task should be marked as skipped
	content, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("ReadFile tasks: %v", readErr)
	}
	if !strings.Contains(string(content), "[x] Task one") {
		t.Errorf("tasks file should contain '[x] Task one' after budget skip, got: %s", content)
	}
}

// TestRunner_Execute_BudgetExceededGateError verifies that gate prompt error
// propagates as error (AC3).
func TestRunner_Execute_BudgetExceededGateError(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "budget-gate-err",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bge-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "budget-gate-err-run"
	r.Metrics = runner.NewMetricsCollector("budget-gate-err-run", budgetPricing())

	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return nil, fmt.Errorf("gate broken")
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "budget gate") {
		t.Errorf("error = %q, want containing 'budget gate'", err.Error())
	}
	if !strings.Contains(err.Error(), "gate broken") {
		t.Errorf("error = %q, want containing inner error 'gate broken'", err.Error())
	}
}

// TestRunner_Execute_BudgetNilMetricsNoPanic verifies that nil Metrics
// produces no budget checks and no panic (AC4 edge case).
func TestRunner_Execute_BudgetNilMetricsNoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "budget-nil-metrics",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bnm-001"},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	// r.Metrics is nil (not set)

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

// TestRunner_Execute_BudgetExceededDuringReview verifies that budget exceeded
// after review session RecordSession triggers hard error (AC5, Task 3.8).
// Execute session is cheap, review session pushes cost over budget.
func TestRunner_Execute_BudgetExceededDuringReview(t *testing.T) {
	tmpDir := t.TempDir()
	// Execute session costs $1 (1 input token). Budget=$5.
	// Review returns SessionMetrics with 10 input tokens → $10 → cumulative $11 > $5 → exceeded.
	scenario := testutil.Scenario{
		Name: "budget-review-exceeded",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bre-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 1}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.BudgetMaxUSD = 5.0
	r.Cfg.BudgetWarnPct = 80
	r.Cfg.GatesEnabled = false
	r.Cfg.RunID = "budget-review-exceeded-run"
	r.Metrics = runner.NewMetricsCollector("budget-review-exceeded-run", budgetPricing())

	// ReviewFn returns high-cost SessionMetrics that push cumulative cost over budget.
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		return runner.ReviewResult{
			Clean: true,
			SessionMetrics: &session.SessionMetrics{
				InputTokens: 10,
			},
			Model: "test-model",
		}, nil
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error from budget exceeded during review, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("error = %q, want containing 'budget exceeded'", err.Error())
	}
	if !strings.Contains(err.Error(), "cost limit reached") {
		t.Errorf("error = %q, want containing 'cost limit reached' (no gates)", err.Error())
	}
}

// TestRunner_Execute_BudgetWarningNotRepeated verifies that budget warning is
// injected only once per task even across multiple iterations (AC2, Task 3.9).
// Two execute iterations both above warn threshold, but only one feedback injection.
func TestRunner_Execute_BudgetWarningNotRepeated(t *testing.T) {
	tmpDir := t.TempDir()
	// Two iterations: first no-commit (retry), second with commit.
	// Each costs $6 (6 input tokens). BudgetMaxUSD=100, BudgetWarnPct=5 → warnAt=$5.
	// After iter1: $6 >= $5 → warning. After iter2: $12 >= $5, but budgetWarned → no repeat.
	scenario := testutil.Scenario{
		Name: "budget-warn-once",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "bwo-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 6}},
			{Type: "execute", ExitCode: 0, SessionID: "bwo-002",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 6}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: []string{
			"aaa", "aaa", // first iteration: no commit → retry
			"aaa", "bbb", // second iteration: commit detected
		},
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 2
	r.Cfg.BudgetMaxUSD = 100.0
	r.Cfg.BudgetWarnPct = 5
	r.Cfg.RunID = "budget-warn-once-run"
	r.Metrics = runner.NewMetricsCollector("budget-warn-once-run", budgetPricing())

	// Capture tasks file during review to count warnings.
	var capturedContent string
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		data, _ := os.ReadFile(r.TasksFile)
		capturedContent = string(data)
		_ = os.WriteFile(r.TasksFile, []byte(allDoneTasks), 0644) // Error ignored: test helper
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Warning should appear exactly once despite two iterations above threshold.
	if count := strings.Count(capturedContent, "Budget warning"); count != 1 {
		t.Errorf("budget warning count = %d, want 1 (once per task), tasks content:\n%s", count, capturedContent)
	}
}

// --- DESIGN-4: Per-task budget cap tests ---

// TestRunner_Execute_TaskBudgetDisabled verifies that TaskBudgetMaxUSD==0 (default)
// produces no task budget skip even with high per-task cost.
func TestRunner_Execute_TaskBudgetDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "task-budget-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "tbd-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 100}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.TaskBudgetMaxUSD = 0 // disabled (default)
	r.Cfg.RunID = "task-budget-disabled-run"
	r.Metrics = runner.NewMetricsCollector("task-budget-disabled-run", budgetPricing())

	r.ReviewFn = func(_ context.Context, rc runner.RunConfig) (runner.ReviewResult, error) {
		_ = os.WriteFile(rc.TasksFile, []byte(allDoneTasks), 0644) // Error ignored: test helper
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

// TestRunner_Execute_TaskBudgetExceededNoGates verifies that per-task cost exceeding
// TaskBudgetMaxUSD without gates auto-skips the task.
func TestRunner_Execute_TaskBudgetExceededNoGates(t *testing.T) {
	tmpDir := t.TempDir()
	// Session costs $10 (10 input tokens). TaskBudgetMaxUSD=5 → exceeded after first session.
	scenario := testutil.Scenario{
		Name: "task-budget-exceeded",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "tbe-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.TaskBudgetMaxUSD = 5.0
	r.Cfg.GatesEnabled = false
	r.Cfg.RunID = "task-budget-exceeded-run"
	r.Metrics = runner.NewMetricsCollector("task-budget-exceeded-run", budgetPricing())

	_, err := r.Execute(context.Background())
	// Task should be skipped (no error), not hard error — unlike run budget
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (expected skip, not error)", err)
	}

	// Verify task was marked as done (SkipTask marks [x]) in file
	data, _ := os.ReadFile(r.TasksFile)
	content := string(data)
	if !strings.Contains(content, "[x]") {
		t.Errorf("task not marked as done (skipped) in tasks file, content:\n%s", content)
	}
}

// TestRunner_Execute_TaskBudgetExceededWithGatesQuit verifies that per-task budget exceeded
// with gates triggers emergency gate, and quit action returns error.
func TestRunner_Execute_TaskBudgetExceededWithGatesQuit(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "task-budget-gate-quit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "tbgq-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.TaskBudgetMaxUSD = 5.0
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "task-budget-gate-quit-run"
	r.Metrics = runner.NewMetricsCollector("task-budget-gate-quit-run", budgetPricing())
	r.EmergencyGatePromptFn = func(_ context.Context, text string) (*config.GateDecision, error) {
		if !strings.Contains(text, "task budget exceeded") {
			t.Errorf("emergency text = %q, want containing 'task budget exceeded'", text)
		}
		return &config.GateDecision{Action: config.ActionQuit}, nil
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "task budget exceeded") {
		t.Errorf("error = %q, want containing 'task budget exceeded'", err.Error())
	}
}

// TestRunner_Execute_TaskBudgetExceededWithGatesSkip verifies that per-task budget exceeded
// with gates and skip action skips the task.
func TestRunner_Execute_TaskBudgetExceededWithGatesSkip(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "task-budget-gate-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "tbgs-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.TaskBudgetMaxUSD = 5.0
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "task-budget-gate-skip-run"
	r.Metrics = runner.NewMetricsCollector("task-budget-gate-skip-run", budgetPricing())
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionSkip}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

// TestRunner_Execute_TaskBudgetExceededWithGatesRetry verifies that per-task budget exceeded
// with gates and retry action continues execution.
func TestRunner_Execute_TaskBudgetExceededWithGatesRetry(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "task-budget-gate-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "tbgr-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.TaskBudgetMaxUSD = 5.0
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "task-budget-gate-retry-run"
	r.Metrics = runner.NewMetricsCollector("task-budget-gate-retry-run", budgetPricing())
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return &config.GateDecision{Action: config.ActionRetry}, nil
	}

	r.ReviewFn = func(_ context.Context, rc runner.RunConfig) (runner.ReviewResult, error) {
		_ = os.WriteFile(rc.TasksFile, []byte(allDoneTasks), 0644) // Error ignored: test helper
		return runner.ReviewResult{Clean: true}, nil
	}

	_, err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
}

// TestRunner_Execute_TaskBudgetGateError verifies that gate prompt I/O failure
// during task budget check propagates as wrapped error.
func TestRunner_Execute_TaskBudgetGateError(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "task-budget-gate-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "tbge-001",
				Model: "test-model",
				Usage: map[string]int{"input_tokens": 10}},
		},
	}
	git := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, git)
	r.Cfg.MaxIterations = 1
	r.Cfg.TaskBudgetMaxUSD = 5.0
	r.Cfg.GatesEnabled = true
	r.Cfg.RunID = "task-budget-gate-error-run"
	r.Metrics = runner.NewMetricsCollector("task-budget-gate-error-run", budgetPricing())
	r.EmergencyGatePromptFn = func(_ context.Context, _ string) (*config.GateDecision, error) {
		return nil, fmt.Errorf("gate prompt failed")
	}

	_, err := r.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: task budget gate:") {
		t.Errorf("error = %q, want containing 'runner: task budget gate:'", err.Error())
	}
	if !strings.Contains(err.Error(), "gate prompt failed") {
		t.Errorf("error = %q, want containing 'gate prompt failed'", err.Error())
	}
}

// --- Story 9.2: Progressive severity filtering integration ---

// TestRunner_Execute_FindingsFiltered verifies that Execute() applies progressive
// severity filtering and findings budget to review findings, rewriting review-findings.md.
// With maxReviewIterations=1, ProgressiveParams(1,1) returns CRITICAL threshold + budget 1.
// ReviewFn returns 3 findings (CRITICAL, MEDIUM, LOW) → filtered to 1 CRITICAL.
// TestRunner_Execute_FindingsFiltered verifies the Execute() loop filters findings
// by severity and truncates to budget (AC#3, AC#5, AC#7).
// Uses maxReviewIterations=1 which maps to ProgressiveParams(1,1) = CRITICAL/1/true/true,
// so only CRITICAL findings survive with budget=1.
// AC#8 (multi-cycle escalation) is covered by TestRunner_Execute_ProgressiveReviewParams.
func TestRunner_Execute_FindingsFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name: "findings-filtered",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)

	// Override: maxReviewIterations=1 → ProgressiveParams(1,1) = CRITICAL/1/true/true
	r.Cfg.MaxReviewIterations = 1

	// ReviewFn returns mixed-severity findings (not clean).
	r.ReviewFn = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		return runner.ReviewResult{
			Clean: false,
			Findings: []runner.ReviewFinding{
				{Severity: "CRITICAL", Description: "null pointer deref", File: "a.go", Line: 10},
				{Severity: "MEDIUM", Description: "unused variable", File: "b.go", Line: 20},
				{Severity: "LOW", Description: "naming convention", File: "c.go", Line: 30},
			},
		}, nil
	}

	_, err := r.Execute(context.Background())
	// Review cycles exhausted (1/1) is expected — Execute returns error for exhausted cycles.
	// We care about the filtering side effect, not the error itself.
	_ = err

	// Verify review-findings.md was rewritten with only CRITICAL finding.
	findingsPath := filepath.Join(tmpDir, "review-findings.md")
	data, readErr := os.ReadFile(findingsPath)
	if readErr != nil {
		t.Fatalf("ReadFile(review-findings.md): %v", readErr)
	}
	content := string(data)

	// Must contain the CRITICAL finding.
	if !strings.Contains(content, "### [CRITICAL] null pointer deref") {
		t.Errorf("review-findings.md missing CRITICAL finding, got: %q", content)
	}
	// Must NOT contain MEDIUM or LOW findings.
	if strings.Contains(content, "MEDIUM") {
		t.Errorf("review-findings.md should not contain MEDIUM, got: %q", content)
	}
	if strings.Contains(content, "LOW") {
		t.Errorf("review-findings.md should not contain LOW, got: %q", content)
	}
	// Must have exactly 1 finding header.
	if count := strings.Count(content, "### ["); count != 1 {
		t.Errorf("review-findings.md: want 1 finding header, got %d in: %q", count, content)
	}
}

// TestRunner_Execute_ProgressiveReviewParams verifies Execute() populates RunConfig progressive fields
// per cycle according to ProgressiveParams (AC#1, AC#4, AC#5, AC#8).
func TestRunner_Execute_ProgressiveReviewParams(t *testing.T) {
	tmpDir := t.TempDir()
	// Need enough mock execute sessions for 5 review cycles (each cycle = 1 execute + 1 review).
	scenario := testutil.Scenario{
		Name: "progressive-params",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "exec-001"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-002"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-003"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-004"},
			{Type: "execute", ExitCode: 0, SessionID: "exec-005"},
		},
	}
	// 5 pairs for 5 execute cycles (before/after per execute).
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
			[2]string{"ddd", "eee"},
			[2]string{"eee", "fff"},
		),
	}
	r, _ := setupRunnerIntegration(t, tmpDir, oneOpenTask, scenario, mock)
	r.Cfg.MaxReviewIterations = 5
	r.Cfg.MaxIterations = 5

	// Capture RunConfig per review call.
	// Note: AC#4 env propagation (CLAUDE_CODE_EFFORT_LEVEL=high) is verified indirectly —
	// HighEffort=true on RunConfig directly triggers execEnv assignment (runner.go:904-907).
	// Full env verification would require mock subprocess inspection (out of scope for unit test).
	type capturedRC struct {
		Cycle           int
		MinSeverity     runner.SeverityLevel
		MaxFindings     int
		IncrementalDiff bool
		HighEffort      bool
		PrevFindings    string
	}
	var captured []capturedRC

	r.ReviewFn = func(_ context.Context, rc runner.RunConfig) (runner.ReviewResult, error) {
		captured = append(captured, capturedRC{
			Cycle:           rc.Cycle,
			MinSeverity:     rc.MinSeverity,
			MaxFindings:     rc.MaxFindings,
			IncrementalDiff: rc.IncrementalDiff,
			HighEffort:      rc.HighEffort,
			PrevFindings:    rc.PrevFindings,
		})
		// Return findings to continue looping (not clean).
		return runner.ReviewResult{
			Clean: false,
			Findings: []runner.ReviewFinding{
				{Severity: "HIGH", Description: "issue cycle " + fmt.Sprintf("%d", len(captured))},
			},
		}, nil
	}

	// Execute returns error (review cycles exhausted) — expected.
	_, _ = r.Execute(context.Background())

	// Verify we got 5 review calls.
	if len(captured) != 5 {
		t.Fatalf("expected 5 review calls, got %d", len(captured))
	}

	// AC#8: verify progressive params per cycle using ProgressiveParams(cycle, 5).
	for i, cap := range captured {
		cycle := i + 1
		want := runner.ProgressiveParams(cycle, 5)
		t.Run(fmt.Sprintf("cycle_%d", cycle), func(t *testing.T) {
			if cap.Cycle != cycle {
				t.Errorf("Cycle: got %d, want %d", cap.Cycle, cycle)
			}
			if cap.MinSeverity != want.MinSeverity {
				t.Errorf("MinSeverity: got %v, want %v", cap.MinSeverity, want.MinSeverity)
			}
			if cap.MaxFindings != want.MaxFindings {
				t.Errorf("MaxFindings: got %d, want %d", cap.MaxFindings, want.MaxFindings)
			}
			if cap.IncrementalDiff != want.IncrementalDiff {
				t.Errorf("IncrementalDiff: got %v, want %v", cap.IncrementalDiff, want.IncrementalDiff)
			}
			if cap.HighEffort != want.HighEffort {
				t.Errorf("HighEffort: got %v, want %v", cap.HighEffort, want.HighEffort)
			}
		})
	}

	// AC#2: cycle 3+ have previous findings from prior cycle.
	if captured[2].PrevFindings == "" {
		t.Error("cycle 3 should have PrevFindings from cycle 2")
	}
	if !strings.Contains(captured[2].PrevFindings, "issue cycle 2") {
		t.Errorf("cycle 3 PrevFindings should reference cycle 2 findings, got: %q", captured[2].PrevFindings)
	}
	// AC#3: cycle 1 has empty PrevFindings.
	if captured[0].PrevFindings != "" {
		t.Errorf("cycle 1 should have empty PrevFindings, got: %q", captured[0].PrevFindings)
	}
}
