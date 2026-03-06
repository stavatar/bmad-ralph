package runner_test

// coverage_external_test.go — unit tests for exported functions that need MockClaude
// and cannot be reached by Runner.Execute integration path.
// Covers: AutoDistill, RunOnce (parse+headcommit paths), RunReview (parse path),
// RealReview (execute paths: OK/ExitError/NonExitError/BuildKnowledgeError).

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

// minimalDistillOutputText returns outputText for MockClaude containing valid
// BEGIN/END distillation markers. When MockClaude JSON-encodes this string,
// newlines become \n escape sequences in raw stdout bytes — ParseDistillOutput
// still finds the markers and extracts non-empty content.
const minimalDistillOutputText = "BEGIN_DISTILLED_OUTPUT\n## CATEGORY: testing\n## testing: assertion-quality [r, runner.go:1] [freq:3] [stage:review]\nUse proper assertions.\nEND_DISTILLED_OUTPUT"

// writeLearningsForTest writes minimal LEARNINGS.md with no citations or
// [needs-formatting] so ValidateDistillation passes trivially.
func writeLearningsForTest(t *testing.T, dir string) {
	t.Helper()
	content := "# Learning: test-principle\nUse good testing principles.\n"
	if err := os.WriteFile(filepath.Join(dir, "LEARNINGS.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write LEARNINGS.md: %v", err)
	}
}

// autoDistillCfg creates a minimal config.Config for AutoDistill tests.
func autoDistillCfg(tmpDir string) *config.Config {
	return &config.Config{
		ClaudeCommand:   os.Args[0],
		ProjectRoot:     tmpDir,
		DistillTimeout:  30,
		LearningsBudget: 200,
	}
}

// setupDistillScenario writes a scenario JSON + output file to scenarioDir and
// sets MOCK_CLAUDE_SCENARIO / MOCK_CLAUDE_STATE_DIR env vars.
// outputText is written to "distill_output.txt" and referenced via OutputFile.
func setupDistillScenario(t *testing.T, outputText string) {
	t.Helper()
	scenarioDir := t.TempDir()
	outputFile := filepath.Join(scenarioDir, "distill_output.txt")
	if err := os.WriteFile(outputFile, []byte(outputText), 0644); err != nil {
		t.Fatalf("write distill output file: %v", err)
	}
	scenario := testutil.Scenario{
		Name: "distill-scenario",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "distill-1", OutputFile: "distill_output.txt"},
		},
	}
	data, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal scenario: %v", err)
	}
	scenarioPath := filepath.Join(scenarioDir, "scenario.json")
	if err := os.WriteFile(scenarioPath, data, 0644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	stateDir := filepath.Join(scenarioDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	t.Setenv("MOCK_CLAUDE_SCENARIO", scenarioPath)
	t.Setenv("MOCK_CLAUDE_STATE_DIR", stateDir)
}

// ===== AutoDistill =====

func TestAutoDistill_MissingLearningsGracefulSkip(t *testing.T) {
	tmpDir := t.TempDir()
	// No LEARNINGS.md → graceful skip (AC#4: os.ErrNotExist returns nil)
	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err != nil {
		t.Fatalf("AutoDistill: expected nil for missing LEARNINGS.md, got %v", err)
	}
}

func TestAutoDistill_ReadLearningsRealError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create LEARNINGS.md as a directory → os.ReadFile returns non-NotExist error (AC#6)
	learningsDir := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Fatal("AutoDistill: expected error for non-NotExist read failure")
	}
	if !strings.Contains(err.Error(), "runner: distill: read learnings:") {
		t.Errorf("AutoDistill error = %q, want containing %q", err.Error(), "runner: distill: read learnings:")
	}
}

func TestAutoDistill_ExecuteError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	// MockClaude exits with code 1 → session.Execute returns error
	scenario := testutil.Scenario{
		Name: "distill-exec-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "distill-err-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Fatal("AutoDistill: expected error when session.Execute fails")
	}
	if !strings.Contains(err.Error(), "runner: distill: execute:") {
		t.Errorf("AutoDistill error = %q, want containing %q", err.Error(), "runner: distill: execute:")
	}
}

func TestAutoDistill_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	// Default outputText = "Mock output for step 0" — no BEGIN/END markers
	scenario := testutil.Scenario{
		Name: "distill-parse-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "distill-parse-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Fatal("AutoDistill: expected error when output has no distillation markers")
	}
	if !strings.Contains(err.Error(), "runner: distill: parse:") {
		t.Errorf("AutoDistill error = %q, want containing %q", err.Error(), "runner: distill: parse:")
	}
}

func TestAutoDistill_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	// Ensure .ralph/ exists for state file
	if err := os.MkdirAll(filepath.Join(tmpDir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}

	setupDistillScenario(t, minimalDistillOutputText)

	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1, MonotonicTaskCounter: 5}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err != nil {
		t.Fatalf("AutoDistill: unexpected error: %v", err)
	}

	// LEARNINGS.md must exist (written by distillation pipeline)
	if _, err := os.Stat(filepath.Join(tmpDir, "LEARNINGS.md")); err != nil {
		t.Errorf("AutoDistill: LEARNINGS.md should exist after distillation: %v", err)
	}

	// distill-state.json must be saved
	statePath := filepath.Join(tmpDir, ".ralph", "distill-state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("AutoDistill: distill-state.json should exist: %v", err)
	}

	// Intent file must be deleted (step 12 — distillation complete)
	intentPath := filepath.Join(tmpDir, ".ralph", "distill-intent.json")
	if _, err := os.Stat(intentPath); err == nil {
		t.Error("AutoDistill: intent file should be deleted after successful distillation")
	}

	// Metrics must be set on state
	if state.Metrics == nil {
		t.Error("AutoDistill: state.Metrics should be non-nil after distillation")
	}
}

func TestAutoDistill_ValidateError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}

	setupDistillScenario(t, minimalDistillOutputText)

	// LearningsBudget=0: ValidateDistillation criterion 1 fails immediately
	// (output.CompressedLearnings has 1 statement-line counted via strings.Count+1,
	// and 1 > 0 → budget exceeded error).
	cfg := &config.Config{
		ClaudeCommand:   os.Args[0],
		ProjectRoot:     tmpDir,
		DistillTimeout:  30,
		LearningsBudget: 0,
	}
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Fatal("AutoDistill: expected error when ValidateDistillation fails (budget=0)")
	}
	if !strings.Contains(err.Error(), "runner: distill: validate:") {
		t.Errorf("AutoDistill validate error = %q, want containing %q", err.Error(), "runner: distill: validate:")
	}
}

// ===== RunOnce =====

func TestRunOnce_ReadTasksError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := testConfig(tmpDir, 3)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: filepath.Join(tmpDir, "nonexistent.md"),
	}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error for missing tasks file")
	}
	if !strings.Contains(err.Error(), "runner: read tasks:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: read tasks:")
	}
}

func TestRunOnce_AllDone(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, allDoneTasks)
	cfg := testConfig(tmpDir, 3)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: tasksPath,
	}

	err := runner.RunOnce(context.Background(), rc)
	if err != nil {
		t.Fatalf("RunOnce: expected nil when all tasks done, got: %v", err)
	}
}

func TestRunOnce_GitHealthError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)
	cfg := testConfig(tmpDir, 3)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{HealthCheckErrors: []error{runner.ErrDirtyTree}},
		TasksFile: tasksPath,
	}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error for git health check failure")
	}
	if !strings.Contains(err.Error(), "runner: git health:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: git health:")
	}
}

func TestRunOnce_ExecuteError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "runonce-exec-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "runonce-err-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := testConfig(tmpDir, 3)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: tasksPath,
	}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error when session.Execute fails")
	}
	if !strings.Contains(err.Error(), "runner: execute:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: execute:")
	}
}

func TestRunOnce_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "runonce-happy",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "runonce-ok-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg: cfg,
		Git: &testutil.MockGitClient{
			HeadCommits: []string{"aaa", "bbb"},
		},
		TasksFile: tasksPath,
	}

	err := runner.RunOnce(context.Background(), rc)
	if err != nil {
		t.Fatalf("RunOnce: unexpected error: %v", err)
	}
}

// ===== RunReview =====

func TestRunReview_ExecuteError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "runreview-exec-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "runreview-err-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: tasksPath,
	}

	err := runner.RunReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RunReview: expected error when session.Execute fails")
	}
	if !strings.Contains(err.Error(), "runner: review execute:") {
		t.Errorf("RunReview error = %q, want containing %q", err.Error(), "runner: review execute:")
	}
}

func TestRunReview_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "runreview-happy",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "runreview-ok-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: tasksPath,
	}

	err := runner.RunReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RunReview: unexpected error: %v", err)
	}
}

// ===== RunOnce additional paths =====

func TestRunOnce_ParseResultError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// MOCK_EXIT_EMPTY=0: test binary exits 0 with empty stdout →
	// session.Execute returns (&RawResult{Stdout:[]byte{}}, nil) →
	// session.ParseResult fails with "session: parse: empty output".
	t.Setenv("MOCK_EXIT_EMPTY", "0")

	cfg := testConfig(tmpDir, 3)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: tasksPath,
	}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error when ParseResult fails (empty stdout)")
	}
	if !strings.Contains(err.Error(), "runner: parse result:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: parse result:")
	}
}

func TestRunOnce_HeadCommitError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "runonce-headcommit-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "runonce-hc-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := testConfig(tmpDir, 3)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{HeadCommitErrors: []error{fmt.Errorf("head commit failed")}},
		TasksFile: tasksPath,
	}

	err := runner.RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error when HeadCommit fails")
	}
	if !strings.Contains(err.Error(), "runner: head commit:") {
		t.Errorf("RunOnce error = %q, want containing %q", err.Error(), "runner: head commit:")
	}
	if !strings.Contains(err.Error(), "head commit failed") {
		t.Errorf("RunOnce error = %q, want inner %q", err.Error(), "head commit failed")
	}
}

// ===== RunReview additional paths =====

func TestRunReview_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// MOCK_EXIT_EMPTY=0: exit 0 with empty stdout → ParseResult fails.
	t.Setenv("MOCK_EXIT_EMPTY", "0")

	cfg := testConfig(tmpDir, 1)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       &testutil.MockGitClient{},
		TasksFile: tasksPath,
	}

	err := runner.RunReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RunReview: expected error when ParseResult fails (empty stdout)")
	}
	if !strings.Contains(err.Error(), "runner: review parse:") {
		t.Errorf("RunReview error = %q, want containing %q", err.Error(), "runner: review parse:")
	}
}

// ===== RealReview with session.Execute =====

func realReviewCfg(tmpDir string) *config.Config {
	return &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   tmpDir,
		MaxTurns:      5,
	}
}

func TestRealReview_ExecuteOK_NotClean(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	scenario := testutil.Scenario{
		Name: "realreview-ok-notclean",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "rr-ok-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := realReviewCfg(tmpDir)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: tasksPath,
		Knowledge: nil, // skip ValidateNewLessons
	}

	result, err := runner.RealReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RealReview: unexpected error: %v", err)
	}
	// Task not marked [x] and no review-findings.md → Clean=false
	if result.Clean {
		t.Error("RealReview: want Clean=false when task not marked done")
	}
}

func TestRealReview_ExecuteExitError_NotClean(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// ExitCode:1 → ExitError → proceeds to DetermineReviewOutcome (not returned as error)
	scenario := testutil.Scenario{
		Name: "realreview-exit-error",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "rr-exit-1"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := realReviewCfg(tmpDir)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: tasksPath,
		Knowledge: nil,
	}

	result, err := runner.RealReview(context.Background(), rc)
	if err != nil {
		t.Fatalf("RealReview: unexpected error on ExitError path: %v", err)
	}
	if result.Clean {
		t.Error("RealReview: want Clean=false when exit error (task not marked done)")
	}
}

// TestRealReview_ExecuteNonExitError verifies the non-ExitError branch of RealReview:
// when session.Execute fails with a non-exec.ExitError (e.g. command not found),
// RealReview returns "runner: review: execute:" error.
func TestRealReview_ExecuteNonExitError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// Non-existent binary: session.Execute returns *os.PathError, not *exec.ExitError.
	cfg := &config.Config{
		ClaudeCommand: "/nonexistent/binary/for/test",
		ProjectRoot:   tmpDir,
		MaxTurns:      1,
	}
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: tasksPath,
	}

	_, err := runner.RealReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RealReview: expected error when command not found")
	}
	if !strings.Contains(err.Error(), "runner: review: execute:") {
		t.Errorf("RealReview error = %q, want containing %q", err.Error(), "runner: review: execute:")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("RealReview inner error = %q, want containing path %q", err.Error(), "nonexistent")
	}
}

// TestAutoDistill_BackupError verifies AutoDistill returns the backup error when
// BackupDistillationFiles fails (LEARNINGS.md.bak is a regular file and
// LEARNINGS.md.bak.1 is a non-empty directory → rotate rename returns EISDIR).
// Line 801-803: if err := BackupDistillationFiles(...); err != nil { return err }
func TestAutoDistill_BackupError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	bakPath := learningsPath + ".bak"
	bak1Path := learningsPath + ".bak.1"

	// bak exists as regular file; bak.1 as non-empty dir → rotate rename → EISDIR
	if err := os.WriteFile(bakPath, []byte("old backup"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bak1Path, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Skip("filesystem allows rename of file onto non-empty directory — backup succeeded")
	}
	if !strings.Contains(err.Error(), "runner: distill: backup:") {
		t.Errorf("AutoDistill error = %q, want containing %q", err.Error(), "runner: distill: backup:")
	}
}

// TestAutoDistill_WriteDistillOutputError verifies AutoDistill returns the write error
// when WriteDistillOutput fails because LEARNINGS.md.pending is a non-empty directory.
// Line 855-857: if writeErr != nil { return writeErr }
func TestAutoDistill_WriteDistillOutputError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	// Pre-create LEARNINGS.md.pending as non-empty dir → first WriteFile call fails
	pendingPath := filepath.Join(tmpDir, "LEARNINGS.md.pending")
	if err := os.MkdirAll(filepath.Join(pendingPath, "child"), 0755); err != nil {
		t.Fatal(err)
	}

	setupDistillScenario(t, minimalDistillOutputText)

	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Fatal("AutoDistill: expected write error when LEARNINGS.md.pending is a directory")
	}
	if !strings.Contains(err.Error(), "runner: distill: write:") {
		t.Errorf("AutoDistill error = %q, want containing %q", err.Error(), "runner: distill: write:")
	}
}

// TestAutoDistill_WriteIntentFileError verifies AutoDistill returns the intent error
// when WriteIntentFile fails because distill-intent.json is a non-empty directory.
// Line 864-866: if err := WriteIntentFile(cfg.ProjectRoot, intent); err != nil { return err }
func TestAutoDistill_WriteIntentFileError(t *testing.T) {
	tmpDir := t.TempDir()
	writeLearningsForTest(t, tmpDir)

	// Pre-create .ralph/distill-intent.json as non-empty dir → WriteIntentFile fails
	if err := os.MkdirAll(filepath.Join(tmpDir, ".ralph", "distill-intent.json", "child"), 0755); err != nil {
		t.Fatal(err)
	}

	setupDistillScenario(t, minimalDistillOutputText)

	cfg := autoDistillCfg(tmpDir)
	state := &runner.DistillState{Version: 1}

	err := runner.AutoDistill(context.Background(), cfg, state)
	if err == nil {
		t.Fatal("AutoDistill: expected error when distill-intent.json is a directory")
	}
	if !strings.Contains(err.Error(), "runner: distill: intent:") {
		t.Errorf("AutoDistill error = %q, want containing %q", err.Error(), "runner: distill: intent:")
	}
}

// TestRealReview_BuildKnowledgeError verifies the buildKnowledgeReplacements error branch:
// when LEARNINGS.md is a directory, ReadFile returns EISDIR (not ErrNotExist),
// causing buildKnowledgeReplacements to fail → RealReview returns "runner: review: build knowledge:".
func TestRealReview_BuildKnowledgeError(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, threeOpenTasks)

	// Make LEARNINGS.md a directory → ReadFile fails with EISDIR (not os.ErrNotExist).
	if err := os.MkdirAll(filepath.Join(tmpDir, "LEARNINGS.md"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := realReviewCfg(tmpDir)
	rc := runner.RunConfig{
		Cfg:       cfg,
		Git:       nil,
		TasksFile: tasksPath,
	}

	_, err := runner.RealReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RealReview: expected error when buildKnowledgeReplacements fails")
	}
	if !strings.Contains(err.Error(), "runner: review: build knowledge:") {
		t.Errorf("RealReview error = %q, want containing %q", err.Error(), "runner: review: build knowledge:")
	}
	if !strings.Contains(err.Error(), "runner: build knowledge: read learnings:") {
		t.Errorf("RealReview inner error = %q, want containing %q", err.Error(), "runner: build knowledge: read learnings:")
	}
}
