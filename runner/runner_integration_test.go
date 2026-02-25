//go:build integration

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

// mockGitClient is a test-local stub implementing GitClient.
// Real MockGitClient deferred to Story 3.3 when GitClient interface is fully defined.
type mockGitClient struct {
	healthCheckErr  error
	hasNewCommit    bool
	hasNewCommitErr error
}

func (m *mockGitClient) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func (m *mockGitClient) HasNewCommit(ctx context.Context) (bool, error) {
	return m.hasNewCommit, m.hasNewCommitErr
}

func TestMain(m *testing.M) {
	if testutil.RunMockClaude() {
		return
	}
	os.Exit(m.Run())
}

func TestRunOnce_WalkingSkeleton_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Copy fixture to temp dir
	fixtureData, err := os.ReadFile("testdata/sprint-tasks-basic.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, fixtureData, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

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

	git := &mockGitClient{hasNewCommit: true}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	ctx := context.Background()

	// Execute
	if err := RunOnce(ctx, rc); err != nil {
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
	if !strings.Contains(promptValue, "Implement hello world") {
		t.Errorf("execute prompt: want task content, got %q", promptValue)
	}

	// Review
	if err := RunReview(ctx, rc); err != nil {
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

	fixtureData, err := os.ReadFile("testdata/sprint-tasks-basic.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, fixtureData, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// No mock Claude needed — should fail before session.Execute
	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &mockGitClient{healthCheckErr: errors.New("git not found")}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err = RunOnce(context.Background(), rc)
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

func TestRunOnce_WalkingSkeleton_NoOpenTasks(t *testing.T) {
	tmpDir := t.TempDir()

	// All tasks completed
	allDone := "# Sprint Tasks\n\n## Epic 1: Foundation\n\n- [x] Already done\n"
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, []byte(allDone), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand: "unused",
		MaxTurns:      5,
		ProjectRoot:   tmpDir,
	}

	git := &mockGitClient{hasNewCommit: true}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err := RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !errors.Is(err, config.ErrNoTasks) {
		t.Errorf("errors.Is(err, ErrNoTasks): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "runner:") {
		t.Errorf("error prefix: want 'runner:', got %q", err.Error())
	}
}

func TestRunOnce_WalkingSkeleton_SessionFails(t *testing.T) {
	tmpDir := t.TempDir()

	fixtureData, err := os.ReadFile("testdata/sprint-tasks-basic.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, fixtureData, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

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

	git := &mockGitClient{hasNewCommit: true}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err = RunOnce(context.Background(), rc)
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

	git := &mockGitClient{hasNewCommit: true}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: filepath.Join(tmpDir, "nonexistent.md")}

	err := RunOnce(context.Background(), rc)
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

func TestRunOnce_WalkingSkeleton_HasNewCommitFails(t *testing.T) {
	tmpDir := t.TempDir()

	fixtureData, err := os.ReadFile("testdata/sprint-tasks-basic.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tasksPath := filepath.Join(tmpDir, "sprint-tasks.md")
	if err := os.WriteFile(tasksPath, fixtureData, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Mock Claude succeeds so RunOnce reaches HasNewCommit
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

	git := &mockGitClient{hasNewCommitErr: errors.New("git diff failed")}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: tasksPath}

	err = RunOnce(context.Background(), rc)
	if err == nil {
		t.Fatal("RunOnce: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: check commit:") {
		t.Errorf("error prefix: want 'runner: check commit:', got %q", err.Error())
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

	git := &mockGitClient{hasNewCommit: true}
	rc := RunConfig{Cfg: cfg, Git: git, TasksFile: filepath.Join(tmpDir, "unused.md")}

	err := RunReview(context.Background(), rc)
	if err == nil {
		t.Fatal("RunReview: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: review execute:") {
		t.Errorf("error prefix: want 'runner: review execute:', got %q", err.Error())
	}
}

// --- Helper functions for args assertions ---

func assertArgsContainFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			return
		}
	}
	t.Errorf("args: want flag %q, not found in %v", flag, args)
}

func assertArgsContainFlagValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Errorf("args: flag %q found at end of args, no value follows", flag)
				return
			}
			if args[i+1] != value {
				t.Errorf("args: flag %q value = %q, want %q", flag, args[i+1], value)
			}
			return
		}
	}
	t.Errorf("args: want flag %q with value %q, flag not found in %v", flag, value, args)
}

func argValueAfterFlag(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
