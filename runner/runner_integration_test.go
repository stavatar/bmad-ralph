//go:build integration

package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

