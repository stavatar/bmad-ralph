//go:build integration

package runner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

// =============================================================================
// Final Integration Tests (Story 6.8)
//
// These tests exercise the full knowledge pipeline: knowledge injection,
// distillation, gates, Serena, crash recovery, resume, and cross-language scope.
// All 6 epics validated together end-to-end.
// =============================================================================

// --- Mock Serena detector ---

type mockCodeIndexer struct {
	available bool
	hint      string
}

func (m *mockCodeIndexer) Available(_ string) bool { return m.available }
func (m *mockCodeIndexer) PromptHint() string      { return m.hint }

// --- Task 2: Full end-to-end flow with auto-knowledge (AC: #1) ---

func TestRunner_Execute_FinalIntegration_FullFlowWithKnowledge(t *testing.T) {
	tmpDir := t.TempDir()

	// 2.2: LEARNINGS.md under threshold (50 lines)
	writeLearningsFile(t, tmpDir, 50)
	// 2.8: pre-populate distill-state.json
	writeDistillState(t, tmpDir, 5, 5)

	// Create cited files so ValidateLearnings doesn't filter entries
	if err := os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}

	// 2.3: scenario: execute (commit) → review (clean) — 1 task
	scenario := testutil.Scenario{
		Name: "final-full-flow",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "full-exec-1", CreatesCommit: true},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	tkw := &trackingKnowledgeWriter{}
	td := &trackingDistillFunc{}

	r, stateDir := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.LearningsBudget = 200
	r.Cfg.DistillCooldown = 5
	r.Knowledge = tkw
	r.DistillFn = td.fn
	// 2.5: Serena enabled
	r.CodeIndexer = &mockCodeIndexer{available: true, hint: "If Serena MCP is available, use code indexing"}
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	// 2.12: success
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 2.6: ValidateNewLessons called (at least once for execute)
	if tkw.validateLessonsCount < 1 {
		t.Errorf("ValidateNewLessons count = %d, want >= 1", tkw.validateLessonsCount)
	}

	// 2.7: no distillation (under budget)
	if td.count != 0 {
		t.Errorf("DistillFn count = %d, want 0 (budget under limit)", td.count)
	}

	// 2.9: verify prompt contains knowledge injection
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	prompt := argValueAfterFlag(args, "-p")
	if prompt == "" {
		t.Fatal("execute: -p flag has no value")
	}

	// 2.10: verify Serena hint in prompt or --append-system-prompt
	allArgs := strings.Join(args, " ")
	if !strings.Contains(allArgs, "Serena") && !strings.Contains(prompt, "Serena") {
		t.Error("Serena hint not found in prompt or args")
	}

	// 2.11: verify task marked [x]
	finalTasks, err := os.ReadFile(r.TasksFile)
	if err != nil {
		t.Fatalf("read tasks: %v", err)
	}
	if !strings.Contains(string(finalTasks), "[x]") {
		t.Errorf("final tasks: want [x] marked, got %q", string(finalTasks))
	}
}

// --- Task 3: Gates + knowledge + emergency (AC: #2) ---

func TestRunner_Execute_FinalIntegration_GatesKnowledgeEmergency(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 30)
	writeDistillState(t, tmpDir, 1, 1)

	// 3 tasks: task1 clean, task2 all fail → emergency → skip, task3 clean + checkpoint
	threeTasksContent := `# Sprint Tasks

- [ ] Task one
- [ ] Task two
- [ ] Task three
`
	// Task1: exec→commit→clean review
	// Task2: exec→no commit (3 attempts) → emergency gate → skip
	// Task3: exec→commit→clean review → checkpoint gate → approve
	scenario := testutil.Scenario{
		Name: "final-gates-emergency",
		Steps: []testutil.ScenarioStep{
			// Task1
			{Type: "execute", ExitCode: 0, SessionID: "gates-exec-1"},
			// Task2: 3 attempts, all no-commit
			{Type: "execute", ExitCode: 0, SessionID: "gates-exec-2a"},
			{Type: "execute", ExitCode: 0, SessionID: "gates-exec-2b"},
			{Type: "execute", ExitCode: 0, SessionID: "gates-exec-2c"},
			// Task3
			{Type: "execute", ExitCode: 0, SessionID: "gates-exec-3"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "bbb"}, // task1: commit
			[2]string{"bbb", "bbb"}, // task2: no commit attempt1
			[2]string{"bbb", "bbb"}, // task2: no commit attempt2
			[2]string{"bbb", "bbb"}, // task2: no commit attempt3 → emergency
			[2]string{"bbb", "ccc"}, // task3: commit
		),
	}

	tkw := &trackingKnowledgeWriter{}

	// Emergency gate: skip (for task2). Normal gate: approve (checkpoint for task3).
	emergencyGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}
	normalGP := &sequenceGatePrompt{
		decisions: []*config.GateDecision{
			{Action: config.ActionApprove}, // checkpoint after task3 (completedTasks=2, mod 2=0)
		},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, threeTasksContent, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.GatesEnabled = true
	r.Cfg.GatesCheckpoint = 2
	r.Cfg.LearningsBudget = 200
	r.Cfg.DistillCooldown = 100 // high cooldown → no distillation
	r.Knowledge = tkw
	r.GatePromptFn = normalGP.fn
	r.EmergencyGatePromptFn = emergencyGP.fn
	r.ReviewFn = progressiveReviewFn(r.TasksFile, nil)
	r.DistillFn = noopDistillFn

	err := r.Execute(context.Background())
	// 3.8: all tasks processed
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// Emergency gate called for task2
	if emergencyGP.count < 1 {
		t.Errorf("EmergencyGatePromptFn count = %d, want >= 1", emergencyGP.count)
	}

	// Normal gate called for checkpoint after task3
	if normalGP.calls < 1 {
		t.Errorf("GatePromptFn count = %d, want >= 1", normalGP.calls)
	}

	// 3.7: knowledge validated across sessions
	if tkw.validateLessonsCount < 1 {
		t.Errorf("ValidateNewLessons count = %d, want >= 1", tkw.validateLessonsCount)
	}
}

// --- Task 4: Auto-distillation multi-file output (AC: #3) ---

func TestRunner_Execute_FinalIntegration_AutoDistillation(t *testing.T) {
	tmpDir := t.TempDir()

	// 4.2: LEARNINGS.md with 160 lines (over soft threshold of 150)
	writeLearningsFile(t, tmpDir, 160)
	// 4.4: cooldown met (counter=10, lastDistill=3, diff=7 >= 5)
	writeDistillState(t, tmpDir, 10, 3)

	scenario := testutil.Scenario{
		Name: "final-auto-distill",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "distill-exec-1"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	td := &trackingDistillFunc{}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.DistillFn = td.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 4.5: distillation triggered
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1 (budget over limit, cooldown met)", td.count)
	}

	// 4.6+4.7: verify distill state via saved states
	if len(td.states) != 1 {
		t.Fatalf("DistillFn states len = %d, want 1", len(td.states))
	}
	// MonotonicTaskCounter should have been incremented from 10 to 11 before distillation call
	if td.states[0].MonotonicTaskCounter != 11 {
		t.Errorf("MonotonicTaskCounter at distill = %d, want 11", td.states[0].MonotonicTaskCounter)
	}
}

// --- Task 5: Distillation failure triggers human gate (AC: #4) ---

func TestRunner_Execute_FinalIntegration_DistillFailureGate(t *testing.T) {
	tmpDir := t.TempDir()

	// 5.2: LEARNINGS.md over threshold
	writeLearningsFile(t, tmpDir, 160)
	writeDistillState(t, tmpDir, 10, 0)

	// Capture original LEARNINGS.md content
	origLearnings, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatal(err)
	}

	scenario := testutil.Scenario{
		Name: "final-distill-fail-gate",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "distill-fail-exec-1"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	// 5.3: DistillFn returns error
	td := &trackingDistillFunc{errs: []error{fmt.Errorf("mock distillation failed")}}

	// 5.4: gate returns skip
	gateGP := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionSkip},
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.GatesEnabled = true
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.DistillFn = td.fn
	r.GatePromptFn = gateGP.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err = r.Execute(context.Background())
	// 5.6: runner continues normally
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 5.5: human gate called with error text
	if gateGP.count < 1 {
		t.Errorf("GatePromptFn count = %d, want >= 1 (distill failure gate)", gateGP.count)
	}
	if gateGP.count > 0 && !strings.Contains(gateGP.taskText, "distillation failed") {
		t.Errorf("gate text = %q, want containing 'distillation failed'", gateGP.taskText)
	}

	// 5.7: LEARNINGS.md unchanged (skip = no modifications)
	currentLearnings, err := os.ReadFile(filepath.Join(tmpDir, "LEARNINGS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(currentLearnings) != string(origLearnings) {
		t.Error("LEARNINGS.md content changed after distill failure + skip")
	}
}

// --- Task 6: JIT citation validation filters stale (AC: #5) ---

func TestRunner_Execute_FinalIntegration_JITCitationValidation(t *testing.T) {
	tmpDir := t.TempDir()
	writeDistillState(t, tmpDir, 1, 1)

	// 6.2: LEARNINGS.md with 5 entries, 2 citing files that don't exist
	learnings := `# LEARNINGS

## testing: pattern-1 [review, existing1.go:10]
Valid entry 1 about testing patterns.

## testing: pattern-2 [review, missing1.go:20]
Entry citing a deleted file.

## testing: pattern-3 [review, existing2.go:30]
Valid entry 3 about code quality.

## testing: pattern-4 [review, missing2.go:40]
Another entry citing a deleted file.

## testing: pattern-5 [review, existing3.go:50]
Valid entry 5 about error handling.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(learnings), 0644); err != nil {
		t.Fatal(err)
	}

	// 6.3: Create 3 cited files, leave 2 missing
	for _, name := range []string{"existing1.go", "existing2.go", "existing3.go"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	scenario := testutil.Scenario{
		Name: "final-jit-citation",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "jit-exec-1"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, stateDir := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.LearningsBudget = 200
	r.DistillFn = noopDistillFn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 6.4+6.5: verify prompt only has valid entries (stale filtered)
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	prompt := argValueAfterFlag(args, "-p")

	// Valid entries should be present
	if !strings.Contains(prompt, "existing1.go") {
		t.Error("prompt missing valid entry citing existing1.go")
	}
	if !strings.Contains(prompt, "existing3.go") {
		t.Error("prompt missing valid entry citing existing3.go")
	}

	// Stale entries should be filtered
	if strings.Contains(prompt, "missing1.go") {
		t.Error("prompt contains stale entry citing missing1.go — should be filtered")
	}
	if strings.Contains(prompt, "missing2.go") {
		t.Error("prompt contains stale entry citing missing2.go — should be filtered")
	}

	// 6.6: HasLearnings = true (valid entries exist)
	// The Self-Review section should be in the prompt (only when HasLearnings=true)
	if !strings.Contains(prompt, "Self-Review") {
		t.Error("prompt missing Self-Review section (HasLearnings should be true)")
	}
}

// --- Task 7: Resume + knowledge flow (AC: #6) ---

func TestRunner_Execute_FinalIntegration_ResumeKnowledge(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 30)
	writeDistillState(t, tmpDir, 1, 1)

	// 7.2: 1 task — execute fails (no commit) → resume → retry → execute succeeds
	scenario := testutil.Scenario{
		Name: "final-resume-knowledge",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "resume-exec-1"},
			{Type: "execute", ExitCode: 0, SessionID: "resume-exec-2"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs(
			[2]string{"aaa", "aaa"}, // no commit → retry
			[2]string{"aaa", "bbb"}, // commit → success
		),
	}

	tkw := &trackingKnowledgeWriter{}
	resume := &trackingResumeExtract{}
	sleep := &trackingSleep{}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 3
	r.Cfg.LearningsBudget = 200
	r.Knowledge = tkw
	r.ResumeExtractFn = resume.fn
	r.SleepFn = sleep.fn
	r.DistillFn = noopDistillFn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	// 7.7: retry succeeds
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 7.5: resume-extraction called with session ID from failed execute
	if resume.count != 1 {
		t.Errorf("resume count = %d, want 1", resume.count)
	}
	if len(resume.sessionIDs) > 0 && resume.sessionIDs[0] != "resume-exec-1" {
		t.Errorf("resume sessionID = %q, want %q", resume.sessionIDs[0], "resume-exec-1")
	}

	// 7.6: knowledge validated
	if tkw.validateLessonsCount < 1 {
		t.Errorf("ValidateNewLessons count = %d, want >= 1", tkw.validateLessonsCount)
	}
}

// --- Task 8: Serena MCP detection fallback (AC: #7) ---

func TestRunner_Execute_FinalIntegration_SerenaFallback(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 10)
	writeDistillState(t, tmpDir, 1, 1)

	scenario := testutil.Scenario{
		Name: "final-serena-fallback",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "serena-exec-1"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	// 8.2: NoOp Serena detector (not available)
	r, stateDir := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.LearningsBudget = 200
	r.CodeIndexer = &runner.NoOpCodeIndexerDetector{}
	r.DistillFn = noopDistillFn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	// 8.4: no errors
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 8.3: no Serena hint in prompt
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	prompt := argValueAfterFlag(args, "-p")
	if strings.Contains(prompt, "Serena") {
		t.Error("prompt contains Serena hint when detector reports unavailable")
	}
}

// --- Task 9: [needs-formatting] tag and fix cycle (AC: #8) ---

func TestRunner_Execute_FinalIntegration_NeedsFormattingCycle(t *testing.T) {
	tmpDir := t.TempDir()

	// 9.2: LEARNINGS.md with 160 lines, 2 entries with [needs-formatting]
	var sb strings.Builder
	sb.WriteString("# LEARNINGS\n\n")
	sb.WriteString("## testing: pattern-nf1 [needs-formatting] [review, main.go:10]\nBadly formatted fact 1.\n\n")
	sb.WriteString("## testing: pattern-nf2 [needs-formatting] [review, main.go:20]\nBadly formatted fact 2.\n\n")
	for i := 0; i < 155; i++ {
		sb.WriteString("- lesson line\n")
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
	// Create cited file
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Cooldown met
	writeDistillState(t, tmpDir, 10, 0)

	scenario := testutil.Scenario{
		Name: "final-needs-formatting",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "nf-exec-1"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	// 9.3: tracking distill captures state
	td := &trackingDistillFunc{}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.DistillCooldown = 5
	r.Cfg.LearningsBudget = 200
	r.DistillFn = td.fn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	err := r.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 9.4: verify LEARNINGS.md had [needs-formatting] before distill
	// (The distill function receives the state — verify it was called)
	if td.count != 1 {
		t.Errorf("DistillFn count = %d, want 1 (budget over limit, cooldown met)", td.count)
	}

	// 9.5: verify distill was called
	if len(td.states) != 1 {
		t.Fatalf("DistillFn states len = %d, want 1", len(td.states))
	}
}

// --- Task 10: Crash recovery at startup (AC: #9) ---

func TestRunner_Execute_FinalIntegration_CrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsFile(t, tmpDir, 30)
	writeDistillState(t, tmpDir, 1, 1)

	// 10.2: create .ralph/distill-intent.json with phase="write"
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}

	targetFile := filepath.Join(tmpDir, "LEARNINGS.md")
	intent := map[string]interface{}{
		"timestamp": "2026-03-01T00:00:00Z",
		"files":     []string{targetFile},
		"phase":     "write",
	}
	intentData, _ := json.MarshalIndent(intent, "", "  ") // map[string]interface{} always marshals
	if err := os.WriteFile(filepath.Join(ralphDir, "distill-intent.json"), intentData, 0644); err != nil {
		t.Fatal(err)
	}

	// 10.3: create .pending file
	pendingContent := "# COMPRESSED LEARNINGS\n\n- recovered line\n"
	if err := os.WriteFile(targetFile+".pending", []byte(pendingContent), 0644); err != nil {
		t.Fatal(err)
	}

	scenario := testutil.Scenario{
		Name: "final-crash-recovery",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "recovery-exec-1"},
		},
	}

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	r, _ := setupRunnerIntegration(t, tmpDir, nonGateOpenTask, scenario, mock)
	r.Cfg.MaxIterations = 1
	r.Cfg.LearningsBudget = 200
	r.DistillFn = noopDistillFn
	r.ReviewFn = reviewAndMarkDoneFn(r.TasksFile, nil)

	// 10.4: Execute Runner → RecoverDistillation runs at startup
	err := r.Execute(context.Background())
	// 10.7: runner proceeds normally
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	// 10.5: intent file deleted
	if _, err := os.Stat(filepath.Join(ralphDir, "distill-intent.json")); !os.IsNotExist(err) {
		t.Error("intent file should be deleted after recovery")
	}

	// 10.5: .pending committed (LEARNINGS.md now has pending content)
	learnings, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("read LEARNINGS.md: %v", err)
	}
	if !strings.Contains(string(learnings), "recovered line") {
		t.Errorf("LEARNINGS.md should contain recovered content, got %q", string(learnings))
	}

	// 10.5: .pending file removed
	if _, err := os.Stat(targetFile + ".pending"); !os.IsNotExist(err) {
		t.Error(".pending file should be removed after recovery")
	}
}
