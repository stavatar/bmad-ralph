//go:build integration

package runner_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

// =============================================================================
// Gate integration tests (Story 5.6)
//
// These tests exercise Runner.Execute with injectable GatePromptFn and
// EmergencyGatePromptFn to validate gate behavior end-to-end across:
// - Normal gates ([GATE] tag), checkpoint gates, combined gates
// - Emergency gates (execute/review exhaustion)
// - All actions: approve, skip, quit, retry with feedback
// - Gates disabled: no prompts fire
// =============================================================================

// --- Task 2.2: Approve at [GATE] tagged task (AC1) ---

func TestRunner_Execute_GateIntegration_Approve(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "gate-approve",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gate-approve-1"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.GatePromptFn = gp.fn
	r.EmergencyGatePromptFn = emergencyGP.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC1: gate prompt fires once
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
	// AC1: emergency gate NOT called
	if emergencyGP.count != 0 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 0", emergencyGP.count)
	}
	// AC1: taskText contains [GATE]
	if !strings.Contains(gp.taskText, "[GATE]") {
		t.Errorf("taskText missing [GATE], got %q", gp.taskText)
	}
	// AC1: task remains [x]
	finalTasks, err := os.ReadFile(r.TasksFile)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	if !strings.Contains(string(finalTasks), "[x]") {
		t.Errorf("final tasks: want [x] marked, got %q", string(finalTasks))
	}
}

// --- Task 2.3: Quit at gate preserves state (AC2) ---

func TestRunner_Execute_GateIntegration_Quit(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "gate-quit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gate-quit-1"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionQuit},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.GatePromptFn = gp.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())

	// AC2: error wraps GateDecision
	if err == nil {
		t.Fatal("Execute: expected error, got nil")
	}
	var gd *config.GateDecision
	if !errors.As(err, &gd) {
		t.Fatalf("errors.As(err, &GateDecision): want true, got false; err = %v", err)
	}
	if gd.Action != config.ActionQuit {
		t.Errorf("GateDecision.Action = %q, want %q", gd.Action, config.ActionQuit)
	}
	// AC2: error contains "runner: gate:" prefix
	if !strings.Contains(err.Error(), "runner: gate:") {
		t.Errorf("error missing prefix, got %q", err.Error())
	}
	// AC2: inner error contains GateDecision message (format: "gate: <action>")
	if !strings.Contains(err.Error(), "gate: quit") {
		t.Errorf("error missing inner cause 'gate: quit', got %q", err.Error())
	}
	// AC2: completed tasks remain [x]
	finalTasks, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	// reviewAndMarkDoneFn marks all done; task should still be [x]
	if !strings.Contains(string(finalTasks), "[x]") {
		t.Errorf("final tasks: want [x] preserved, got %q", string(finalTasks))
	}
}

// --- Task 2.4: Retry with feedback at gate (AC3) ---

func TestRunner_Execute_GateIntegration_RetryWithFeedback(t *testing.T) {
	tmpDir := t.TempDir()

	// Need 2 execute steps: initial + retry after gate feedback
	scenario := testutil.Scenario{
		Name: "gate-retry",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gate-retry-1"},
			{Type: "execute", ExitCode: 0, SessionID: "gate-retry-2"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"}, // first execution
			[2]string{"bbb", "ccc"}, // retry execution
		),
	}

	sg := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionRetry, Feedback: "fix validation"},
			{Action: config.ActionApprove},
		},
	}

	reviewCount := 0
	var intermediateSnapshot string // captures tasks file at start of 2nd review
	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.Cfg.MaxIterations = 5 // enough room for retry
	r.GatePromptFn = sg.fn
	r.ReviewFn = func(ctx context.Context, rc runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		if reviewCount == 2 {
			// Capture intermediate state: task should be [ ] with feedback before re-execution marks it done
			data, err := os.ReadFile(rc.TasksFile)
			if err != nil {
				return runner.ReviewResult{}, err
			}
			intermediateSnapshot = string(data)
		}
		// Progressive: mark first open task done
		data, err := os.ReadFile(rc.TasksFile)
		if err != nil {
			return runner.ReviewResult{}, err
		}
		content := strings.Replace(string(data), "- [ ]", "- [x]", 1)
		// Error ignored: test helper in controlled tmpDir; failure surfaces via downstream assertions
		_ = os.WriteFile(rc.TasksFile, []byte(content), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC3: gate prompt called twice (retry + approve)
	if sg.calls != 2 {
		t.Errorf("GatePromptFn calls = %d, want 2", sg.calls)
	}
	// AC3: feedback injected into sprint-tasks.md
	finalTasks, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	finalStr := string(finalTasks)
	if !strings.Contains(finalStr, config.FeedbackPrefix) {
		t.Errorf("tasks missing FeedbackPrefix %q\ngot: %s", config.FeedbackPrefix, finalStr)
	}
	if !strings.Contains(finalStr, "fix validation") {
		t.Errorf("tasks missing feedback text 'fix validation'\ngot: %s", finalStr)
	}
	// AC3: task re-executed (2 review calls = 2 executions)
	if reviewCount != 2 {
		t.Errorf("reviewCount = %d, want 2", reviewCount)
	}
	// AC3: intermediate state — task reverted to [ ] with feedback visible before re-execution
	if intermediateSnapshot == "" {
		t.Fatal("intermediateSnapshot not captured at 2nd review")
	}
	if !strings.Contains(intermediateSnapshot, "- [ ]") {
		t.Errorf("intermediate: task not reverted to [ ]\ngot: %s", intermediateSnapshot)
	}
	if !strings.Contains(intermediateSnapshot, config.FeedbackPrefix) {
		t.Errorf("intermediate: feedback not visible at re-execution\ngot: %s", intermediateSnapshot)
	}
	// AC3: final task [x]
	if !strings.Contains(finalStr, "[x]") {
		t.Errorf("final tasks: want [x] marked, got %q", finalStr)
	}
}

// --- Task 2.5: Skip at gate (AC4) ---

func TestRunner_Execute_GateIntegration_Skip(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "gate-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gate-skip-1"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.GatePromptFn = gp.fn
	r.EmergencyGatePromptFn = emergencyGP.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC4: gate prompt called once
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1", gp.count)
	}
	// AC4: emergency gate NOT called
	if emergencyGP.count != 0 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 0", emergencyGP.count)
	}
	// AC4: task remains [x]
	finalTasks, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	if !strings.Contains(string(finalTasks), "[x]") {
		t.Errorf("final tasks: want [x], got %q", string(finalTasks))
	}
}

// --- Task 2.6: Combined GATE + checkpoint — single prompt (AC8) ---

func TestRunner_Execute_GateIntegration_CombinedGateCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "gate-combined",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "combined-1"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 1 // fires at every task → combined with [GATE]
	r.GatePromptFn = gp.fn
	r.EmergencyGatePromptFn = emergencyGP.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC8: ONE prompt, not two
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1 (combined, not 2 separate)", gp.count)
	}
	// AC8: emergency gate NOT called
	if emergencyGP.count != 0 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 0", emergencyGP.count)
	}
	// AC8: taskText contains both [GATE] and checkpoint suffix
	if !strings.Contains(gp.taskText, "[GATE]") {
		t.Errorf("taskText missing [GATE], got %q", gp.taskText)
	}
	if !strings.Contains(gp.taskText, "(checkpoint every 1)") {
		t.Errorf("taskText missing checkpoint suffix, got %q", gp.taskText)
	}
}

// --- Task 3.1: Checkpoint fires every N tasks (AC5) ---

func TestRunner_Execute_GateIntegration_CheckpointEveryN(t *testing.T) {
	tmpDir := t.TempDir()

	// 3 tasks, no [GATE] tags, checkpoint every 2
	scenario := testutil.Scenario{
		Name: "checkpoint-every-n",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "chk-1"},
			{Type: "execute", ExitCode: 0, SessionID: "chk-2"},
			{Type: "execute", ExitCode: 0, SessionID: "chk-3"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"},
			[2]string{"bbb", "ccc"},
			[2]string{"ccc", "ddd"},
		),
	}

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeOpenTasks, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 2
	r.GatePromptFn = gp.fn
	r.EmergencyGatePromptFn = emergencyGP.fn
	r.ReviewFn = progressiveReviewFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC5: checkpoint fires after task 2 only (completedTasks=2, 2%2==0)
	// Task 1: completed=1, 1%2!=0 → no gate
	// Task 2: completed=2, 2%2==0 → gate fires
	// Task 3: completed=3, 3%2!=0 → no gate
	if gp.count != 1 {
		t.Errorf("GatePromptFn count = %d, want 1 (only after task 2)", gp.count)
	}
	// AC5: emergency gate NOT called
	if emergencyGP.count != 0 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 0", emergencyGP.count)
	}
	// AC5: gate text contains checkpoint suffix but NOT [GATE] (checkpoint-only)
	if len(gp.taskTexts) != 1 {
		t.Fatalf("taskTexts length = %d, want 1", len(gp.taskTexts))
	}
	if !strings.Contains(gp.taskTexts[0], "(checkpoint every 2)") {
		t.Errorf("taskTexts[0] missing checkpoint suffix, got %q", gp.taskTexts[0])
	}
	if strings.Contains(gp.taskTexts[0], "[GATE]") {
		t.Errorf("taskTexts[0] should NOT contain [GATE] (checkpoint-only), got %q", gp.taskTexts[0])
	}
}

// --- Task 4.1: Emergency gate at execute exhaustion — skip (AC6) ---

func TestRunner_Execute_GateIntegration_EmergencyExecuteSkip(t *testing.T) {
	tmpDir := t.TempDir()

	// MaxIterations=2, all executions return same SHA → no commit → exhaustion
	scenario := testutil.Scenario{
		Name: "emergency-exec-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "emg-1"},
			{Type: "execute", ExitCode: 0, SessionID: "emg-2"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"}, // same = no commit
			[2]string{"aaa", "aaa"}, // same = no commit → exhaustion
		),
	}

	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	normalGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.Cfg.MaxIterations = 2
	r.GatePromptFn = normalGP.fn
	r.EmergencyGatePromptFn = emergencyGP.fn

	err := r.Execute(context.Background())
	// AC6: no ErrMaxRetries — emergency gate handles it
	if err != nil {
		t.Fatalf("Execute: unexpected error (want nil for skip): %v", err)
	}

	// AC6: EmergencyGatePromptFn called
	if emergencyGP.count != 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 1", emergencyGP.count)
	}
	// AC6: emergency text contains "execute attempts exhausted" and "2/2"
	if !strings.Contains(emergencyGP.taskText, "execute attempts exhausted") {
		t.Errorf("emergency taskText missing 'execute attempts exhausted', got %q", emergencyGP.taskText)
	}
	if !strings.Contains(emergencyGP.taskText, "2/2") {
		t.Errorf("emergency taskText missing '2/2', got %q", emergencyGP.taskText)
	}
	// AC6: normal gate NOT called (emergency skip bypasses gate check)
	if normalGP.count != 0 {
		t.Errorf("GatePromptFn count = %d, want 0 (emergency skip bypasses)", normalGP.count)
	}
	// AC6: task marked [x] via SkipTask
	finalTasks, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	if !strings.Contains(string(finalTasks), "[x]") {
		t.Errorf("final tasks: want [x] (skip marks done), got %q", string(finalTasks))
	}
}

// --- Task 4.2: Emergency gate at review exhaustion — skip (AC6) ---

func TestRunner_Execute_GateIntegration_EmergencyReviewSkip(t *testing.T) {
	tmpDir := t.TempDir()

	// Execute succeeds, but review always non-clean → review cycles exhausted
	scenario := testutil.Scenario{
		Name: "emergency-review-skip",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "emg-rev-1"},
			{Type: "execute", ExitCode: 0, SessionID: "emg-rev-2"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"}, // commit detected
			[2]string{"bbb", "ccc"}, // commit detected (review cycle retry)
		),
	}

	reviewCount := 0
	nonCleanReviewFn := func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		reviewCount++
		return runner.ReviewResult{Clean: false}, nil
	}

	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	normalGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.Cfg.MaxReviewIterations = 2
	r.ReviewFn = nonCleanReviewFn
	r.GatePromptFn = normalGP.fn
	r.EmergencyGatePromptFn = emergencyGP.fn

	err := r.Execute(context.Background())
	// AC6: no ErrMaxReviewCycles — emergency gate handles it
	if err != nil {
		t.Fatalf("Execute: unexpected error (want nil for skip): %v", err)
	}

	// AC6: review called exactly MaxReviewIterations times before emergency
	if reviewCount != 2 {
		t.Errorf("reviewCount = %d, want 2 (MaxReviewIterations)", reviewCount)
	}
	// AC6: EmergencyGatePromptFn called
	if emergencyGP.count != 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 1", emergencyGP.count)
	}
	// AC6: emergency text contains "review cycles exhausted" and "2/2"
	if !strings.Contains(emergencyGP.taskText, "review cycles exhausted") {
		t.Errorf("emergency taskText missing 'review cycles exhausted', got %q", emergencyGP.taskText)
	}
	if !strings.Contains(emergencyGP.taskText, "2/2") {
		t.Errorf("emergency taskText missing '2/2', got %q", emergencyGP.taskText)
	}
	// AC6: normal gate NOT called
	if normalGP.count != 0 {
		t.Errorf("GatePromptFn count = %d, want 0 (emergency skip bypasses)", normalGP.count)
	}
	// AC6: task marked [x] via SkipTask
	finalTasks, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	if !strings.Contains(string(finalTasks), "[x]") {
		t.Errorf("final tasks: want [x] (skip marks done), got %q", string(finalTasks))
	}
}

// --- Task 5.1: Gates disabled — no prompts fire (AC7) ---

func TestRunner_Execute_GateIntegration_GatesDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	// MaxIterations=2, same SHA → exhaust → should get ErrMaxRetries (not emergency gate)
	scenario := testutil.Scenario{
		Name: "gates-disabled",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "disabled-1"},
			{Type: "execute", ExitCode: 0, SessionID: "disabled-2"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"},
			[2]string{"aaa", "aaa"},
		),
	}

	normalGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, gateOpenTask, scenario, mock)
	r.Cfg.GatesEnabled = false
	r.Cfg.GatesCheckpoint = 2
	r.Cfg.MaxIterations = 2
	r.GatePromptFn = normalGP.fn
	r.EmergencyGatePromptFn = emergencyGP.fn

	err := r.Execute(context.Background())
	// AC7: returns ErrMaxRetries (original behavior, no emergency gate)
	if !errors.Is(err, config.ErrMaxRetries) {
		t.Fatalf("errors.Is(err, ErrMaxRetries): want true, got false; err = %v", err)
	}
	// AC7: no gate prompts fire
	if normalGP.count != 0 {
		t.Errorf("GatePromptFn count = %d, want 0", normalGP.count)
	}
	if emergencyGP.count != 0 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 0", emergencyGP.count)
	}
}

// --- Task 6.1: Multi-task end-to-end scenario (AC1, AC5, AC8) ---

func TestRunner_Execute_GateIntegration_MultiTaskScenario(t *testing.T) {
	tmpDir := t.TempDir()

	// 4 tasks: task1 plain, task2 [GATE], task3 plain, task4 plain
	// GatesCheckpoint=2
	// Expected gates:
	//   task1: completed=1, no [GATE], 1%2!=0 → no gate
	//   task2: completed=2, [GATE] + 2%2==0 → gate (combined)
	//   task3: completed=3, no [GATE], 3%2!=0 → no gate
	//   task4: completed=4, no [GATE], 4%2==0 → gate (checkpoint)
	scenario := testutil.Scenario{
		Name: "multi-task-gates",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "mt-1"},
			{Type: "execute", ExitCode: 0, SessionID: "mt-2"},
			{Type: "execute", ExitCode: 0, SessionID: "mt-3"},
			{Type: "execute", ExitCode: 0, SessionID: "mt-4"},
		},
	}
	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"a1", "b1"},
			[2]string{"a2", "b2"},
			[2]string{"a3", "b3"},
			[2]string{"a4", "b4"},
		),
	}

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, fourOpenTasksWithGate, scenario, mock)
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 2
	r.Cfg.MaxIterations = 10 // enough for 4 tasks
	r.GatePromptFn = gp.fn
	r.EmergencyGatePromptFn = emergencyGP.fn
	r.ReviewFn = progressiveReviewFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// AC: GatePromptFn called exactly 2 times (after task 2 and task 4)
	if gp.count != 2 {
		t.Errorf("GatePromptFn count = %d, want 2", gp.count)
	}
	// AC: emergency gate NOT called
	if emergencyGP.count != 0 {
		t.Errorf("EmergencyGatePromptFn count = %d, want 0", emergencyGP.count)
	}

	// Verify taskTexts for each gate call
	if len(gp.taskTexts) != 2 {
		t.Fatalf("taskTexts length = %d, want 2", len(gp.taskTexts))
	}

	// Task 2: combined [GATE] + checkpoint
	if !strings.Contains(gp.taskTexts[0], "[GATE]") {
		t.Errorf("taskTexts[0] missing [GATE], got %q", gp.taskTexts[0])
	}
	if !strings.Contains(gp.taskTexts[0], "(checkpoint every 2)") {
		t.Errorf("taskTexts[0] missing checkpoint suffix, got %q", gp.taskTexts[0])
	}
	// Task 4: checkpoint only (no [GATE])
	if strings.Contains(gp.taskTexts[1], "[GATE]") {
		t.Errorf("taskTexts[1] should NOT contain [GATE], got %q", gp.taskTexts[1])
	}
	if !strings.Contains(gp.taskTexts[1], "(checkpoint every 2)") {
		t.Errorf("taskTexts[1] missing checkpoint suffix, got %q", gp.taskTexts[1])
	}

	// All 4 tasks marked done
	finalTasks, readErr := os.ReadFile(r.TasksFile)
	if readErr != nil {
		t.Fatalf("read tasks: %v", readErr)
	}
	finalStr := string(finalTasks)
	openCount := strings.Count(finalStr, "- [ ]")
	if openCount != 0 {
		t.Errorf("open tasks remaining = %d, want 0\ngot: %s", openCount, finalStr)
	}
	doneCount := strings.Count(finalStr, "- [x]")
	if doneCount != 4 {
		t.Errorf("done tasks = %d, want 4\ngot: %s", doneCount, finalStr)
	}
}
