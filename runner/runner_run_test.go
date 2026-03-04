package runner

// runner_run_test.go — unit tests for the Run entry point (runner.go:979).
// Uses initGitRepo from git_test.go (same package runner).
// Run requires a git repo because Execute calls RecoverDirtyState → ExecGitClient.HealthCheck.

import (
	"context"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

func Test_buildTemplateData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		gatesEnabled bool
		serenaHint   string
		hasFindings  bool
		hasLearnings bool
		wantGates    bool
		wantSerena   bool
		wantFindings bool
		wantLearn    bool
	}{
		{
			name:         "all flags set",
			gatesEnabled: true,
			serenaHint:   "serena available",
			hasFindings:  true,
			hasLearnings: true,
			wantGates:    true,
			wantSerena:   true,
			wantFindings: true,
			wantLearn:    true,
		},
		{
			name:         "all flags off",
			gatesEnabled: false,
			serenaHint:   "",
			hasFindings:  false,
			hasLearnings: false,
			wantGates:    false,
			wantSerena:   false,
			wantFindings: false,
			wantLearn:    false,
		},
		{
			name:         "execute pattern gates and findings",
			gatesEnabled: true,
			serenaHint:   "hint",
			hasFindings:  true,
			hasLearnings: false,
			wantGates:    true,
			wantSerena:   true,
			wantFindings: true,
			wantLearn:    false,
		},
		{
			name:         "review pattern serena only",
			gatesEnabled: false,
			serenaHint:   "serena",
			hasFindings:  false,
			hasLearnings: false,
			wantGates:    false,
			wantSerena:   true,
			wantFindings: false,
			wantLearn:    false,
		},
		{
			name:         "once pattern gates and learnings",
			gatesEnabled: true,
			serenaHint:   "",
			hasFindings:  false,
			hasLearnings: true,
			wantGates:    true,
			wantSerena:   false,
			wantFindings: false,
			wantLearn:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{GatesEnabled: tc.gatesEnabled}
			got := buildTemplateData(cfg, tc.serenaHint, tc.hasFindings, tc.hasLearnings)

			if got.GatesEnabled != tc.wantGates {
				t.Errorf("GatesEnabled = %v, want %v", got.GatesEnabled, tc.wantGates)
			}
			if got.SerenaEnabled != tc.wantSerena {
				t.Errorf("SerenaEnabled = %v, want %v", got.SerenaEnabled, tc.wantSerena)
			}
			if got.HasFindings != tc.wantFindings {
				t.Errorf("HasFindings = %v, want %v", got.HasFindings, tc.wantFindings)
			}
			if got.HasLearnings != tc.wantLearn {
				t.Errorf("HasLearnings = %v, want %v", got.HasLearnings, tc.wantLearn)
			}
		})
	}
}

// TestRun_NoTasksFile verifies Run returns "runner: read tasks:" when sprint-tasks.md
// is absent. Covers Run setup code: indexer init, Runner struct, ResumeExtractFn,
// DistillFn, and the Execute delegation (lines 980–1008, except closure bodies).
func TestRun_NoTasksFile(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	cfg := &config.Config{
		ProjectRoot:   dir,
		MaxIterations: 1,
		ClaudeCommand: "/nonexistent/command",
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run() expected error when sprint-tasks.md missing, got nil")
	}
	if !strings.Contains(err.Error(), "runner: read tasks:") {
		t.Errorf("Run() error = %q, want to contain %q", err.Error(), "runner: read tasks:")
	}
}

// TestRun_SerenaEnabled verifies Run assigns SerenaMCPDetector when cfg.SerenaEnabled=true.
// Covers the SerenaEnabled branch (line 982): indexer = &SerenaMCPDetector{}.
func TestRun_SerenaEnabled(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	cfg := &config.Config{
		ProjectRoot:   dir,
		MaxIterations: 1,
		SerenaEnabled: true,
		ClaudeCommand: "/nonexistent/command",
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run() SerenaEnabled expected error when sprint-tasks.md missing, got nil")
	}
	if !strings.Contains(err.Error(), "runner: read tasks:") {
		t.Errorf("Run() SerenaEnabled error = %q, want to contain %q", err.Error(), "runner: read tasks:")
	}
}

// TestRun_GatesEnabled verifies Run assigns GatePromptFn and EmergencyGatePromptFn when
// cfg.GatesEnabled=true. Covers GatesEnabled closure assignments (lines 994–1001).
func TestRun_GatesEnabled(t *testing.T) {
	t.Parallel()
	dir := initGitRepo(t)

	cfg := &config.Config{
		ProjectRoot:   dir,
		MaxIterations: 1,
		GatesEnabled:  true,
		ClaudeCommand: "/nonexistent/command",
	}

	err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("Run() GatesEnabled expected error when sprint-tasks.md missing, got nil")
	}
	if !strings.Contains(err.Error(), "runner: read tasks:") {
		t.Errorf("Run() GatesEnabled error = %q, want to contain %q", err.Error(), "runner: read tasks:")
	}
}

func Test_kvs_MergesPairs(t *testing.T) {
	t.Parallel()
	result := kvs(
		kv("a", "1"),
		kv("b", "2"),
		kv("c", "3"),
	)
	want := []string{"a", "1", "b", "2", "c", "3"}
	if len(result) != len(want) {
		t.Fatalf("kvs len = %d, want %d", len(result), len(want))
	}
	for i, v := range result {
		if v != want[i] {
			t.Errorf("kvs[%d] = %q, want %q", i, v, want[i])
		}
	}
}
