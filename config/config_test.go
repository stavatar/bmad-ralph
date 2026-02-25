package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfigYAML creates .ralph/config.yaml with given content in dir.
func writeConfigYAML(t *testing.T, dir, content string) {
	t.Helper()
	ralphDir := filepath.Join(dir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ralphDir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// --- Task 4: Load tests ---

func TestConfig_Load_ValidFullConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `claude_command: "my-claude"
max_turns: 100
max_iterations: 5
max_review_iterations: 7
gates_enabled: true
gates_checkpoint: 3
review_every: 2
model_execute: "opus"
model_review: "sonnet"
review_min_severity: "HIGH"
always_extract: true
serena_enabled: false
serena_timeout: 30
learnings_budget: 500
log_dir: "/tmp/ralph-logs"
`
	writeConfigYAML(t, dir, yaml)
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ClaudeCommand != "my-claude" {
		t.Errorf("ClaudeCommand = %q, want %q", cfg.ClaudeCommand, "my-claude")
	}
	if cfg.MaxTurns != 100 {
		t.Errorf("MaxTurns = %d, want 100", cfg.MaxTurns)
	}
	if cfg.MaxIterations != 5 {
		t.Errorf("MaxIterations = %d, want 5", cfg.MaxIterations)
	}
	if cfg.MaxReviewIterations != 7 {
		t.Errorf("MaxReviewIterations = %d, want 7", cfg.MaxReviewIterations)
	}
	if !cfg.GatesEnabled {
		t.Error("GatesEnabled = false, want true")
	}
	if cfg.GatesCheckpoint != 3 {
		t.Errorf("GatesCheckpoint = %d, want 3", cfg.GatesCheckpoint)
	}
	if cfg.ReviewEvery != 2 {
		t.Errorf("ReviewEvery = %d, want 2", cfg.ReviewEvery)
	}
	if cfg.ModelExecute != "opus" {
		t.Errorf("ModelExecute = %q, want %q", cfg.ModelExecute, "opus")
	}
	if cfg.ModelReview != "sonnet" {
		t.Errorf("ModelReview = %q, want %q", cfg.ModelReview, "sonnet")
	}
	if cfg.ReviewMinSeverity != "HIGH" {
		t.Errorf("ReviewMinSeverity = %q, want %q", cfg.ReviewMinSeverity, "HIGH")
	}
	if !cfg.AlwaysExtract {
		t.Error("AlwaysExtract = false, want true")
	}
	if cfg.SerenaEnabled {
		t.Error("SerenaEnabled = true, want false")
	}
	if cfg.SerenaTimeout != 30 {
		t.Errorf("SerenaTimeout = %d, want 30", cfg.SerenaTimeout)
	}
	if cfg.LearningsBudget != 500 {
		t.Errorf("LearningsBudget = %d, want 500", cfg.LearningsBudget)
	}
	if cfg.LogDir != "/tmp/ralph-logs" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, "/tmp/ralph-logs")
	}
	if cfg.ProjectRoot != dir {
		t.Errorf("ProjectRoot = %q, want %q", cfg.ProjectRoot, dir)
	}
}

func TestConfig_Load_PartialConfig(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		check func(t *testing.T, cfg *Config)
	}{
		{
			"only max_turns set",
			"max_turns: 100\n",
			func(t *testing.T, cfg *Config) {
				if cfg.MaxTurns != 100 {
					t.Errorf("MaxTurns = %d, want 100", cfg.MaxTurns)
				}
				if cfg.MaxIterations != 3 {
					t.Errorf("default MaxIterations = %d, want 3", cfg.MaxIterations)
				}
				if !cfg.SerenaEnabled {
					t.Error("SerenaEnabled default should be true")
				}
				if cfg.ClaudeCommand != "claude" {
					t.Errorf("ClaudeCommand default = %q, want %q", cfg.ClaudeCommand, "claude")
				}
			},
		},
		{
			"bool explicit false vs absent",
			"gates_enabled: false\n",
			func(t *testing.T, cfg *Config) {
				if cfg.GatesEnabled {
					t.Error("GatesEnabled should be false (explicitly set)")
				}
				if !cfg.SerenaEnabled {
					t.Error("SerenaEnabled default should be true (absent from YAML)")
				}
			},
		},
		{
			"several fields set",
			"claude_command: cc\nreview_every: 5\nserena_timeout: 60\n",
			func(t *testing.T, cfg *Config) {
				if cfg.ClaudeCommand != "cc" {
					t.Errorf("ClaudeCommand = %q, want %q", cfg.ClaudeCommand, "cc")
				}
				if cfg.ReviewEvery != 5 {
					t.Errorf("ReviewEvery = %d, want 5", cfg.ReviewEvery)
				}
				if cfg.SerenaTimeout != 60 {
					t.Errorf("SerenaTimeout = %d, want 60", cfg.SerenaTimeout)
				}
				if cfg.MaxTurns != 50 {
					t.Errorf("default MaxTurns = %d, want 50", cfg.MaxTurns)
				}
				if cfg.ReviewMinSeverity != "LOW" {
					t.Errorf("default ReviewMinSeverity = %q, want %q", cfg.ReviewMinSeverity, "LOW")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfigYAML(t, dir, tt.yaml)
			t.Chdir(dir)
			cfg, err := Load(CLIFlags{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestConfig_Load_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50 (default)", cfg.MaxTurns)
	}
	if !cfg.SerenaEnabled {
		t.Error("SerenaEnabled should be true (default)")
	}
}

func TestConfig_Load_EmptyDocumentMarker(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "---\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("defaults lost: MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if !cfg.SerenaEnabled {
		t.Error("defaults lost: SerenaEnabled should be true")
	}
	if cfg.LearningsBudget != 200 {
		t.Errorf("defaults lost: LearningsBudget = %d, want 200", cfg.LearningsBudget)
	}
}

func TestConfig_Load_MissingFile(t *testing.T) {
	dir := t.TempDir()
	// .ralph/ exists but no config.yaml
	if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("default MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if !cfg.SerenaEnabled {
		t.Error("SerenaEnabled default should be true")
	}
}

func TestConfig_Load_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "max_turns: [invalid\n  broken:\n")
	t.Chdir(dir)

	_, err := Load(CLIFlags{})
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
	// String check justified: yaml.v3 doesn't export its syntax error type,
	// so errors.As is not possible. We verify our wrapping prefix is present.
	if !strings.Contains(err.Error(), "config: parse yaml:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "config: parse yaml:")
	}
}

func TestConfig_Load_UnknownFields(t *testing.T) {
	dir := t.TempDir()
	yaml := "max_turns: 77\nunknown_field: hello\nanother_unknown: 42\n"
	writeConfigYAML(t, dir, yaml)
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error for unknown fields: %v", err)
	}
	if cfg.MaxTurns != 77 {
		t.Errorf("MaxTurns = %d, want 77", cfg.MaxTurns)
	}
}

func TestConfig_Load_DefaultsComplete(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ClaudeCommand != "claude" {
		t.Errorf("ClaudeCommand = %q, want %q", cfg.ClaudeCommand, "claude")
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if cfg.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", cfg.MaxIterations)
	}
	if cfg.MaxReviewIterations != 3 {
		t.Errorf("MaxReviewIterations = %d, want 3", cfg.MaxReviewIterations)
	}
	if cfg.GatesEnabled {
		t.Error("GatesEnabled should default to false")
	}
	if cfg.GatesCheckpoint != 0 {
		t.Errorf("GatesCheckpoint = %d, want 0", cfg.GatesCheckpoint)
	}
	if cfg.ReviewEvery != 1 {
		t.Errorf("ReviewEvery = %d, want 1", cfg.ReviewEvery)
	}
	if cfg.ModelExecute != "" {
		t.Errorf("ModelExecute = %q, want empty", cfg.ModelExecute)
	}
	if cfg.ModelReview != "" {
		t.Errorf("ModelReview = %q, want empty", cfg.ModelReview)
	}
	if cfg.ReviewMinSeverity != "LOW" {
		t.Errorf("ReviewMinSeverity = %q, want %q", cfg.ReviewMinSeverity, "LOW")
	}
	if cfg.AlwaysExtract {
		t.Error("AlwaysExtract should default to false")
	}
	if !cfg.SerenaEnabled {
		t.Error("SerenaEnabled should default to true")
	}
	if cfg.SerenaTimeout != 10 {
		t.Errorf("SerenaTimeout = %d, want 10", cfg.SerenaTimeout)
	}
	if cfg.LearningsBudget != 200 {
		t.Errorf("LearningsBudget = %d, want 200", cfg.LearningsBudget)
	}
	if cfg.LogDir != ".ralph/logs" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, ".ralph/logs")
	}
	if cfg.ProjectRoot == "" {
		t.Error("ProjectRoot should not be empty")
	}
}

func TestConfig_Load_UnreadableConfig(t *testing.T) {
	dir := t.TempDir()
	ralphDir := filepath.Join(dir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	// config.yaml as directory triggers non-NotExist read error
	if err := os.MkdirAll(filepath.Join(ralphDir, "config.yaml"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	_, err := Load(CLIFlags{})
	if err == nil {
		t.Fatal("expected error for unreadable config, got nil")
	}
	// String check justified: os package doesn't export "is a directory" error
	// type consistently across platforms. We verify our wrapping prefix.
	if !strings.Contains(err.Error(), "config: read:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "config: read:")
	}
}

func TestConfig_Load_CommentsOnlyYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "---\n# This config is entirely comments\n# No actual keys\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("defaults lost: MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if !cfg.SerenaEnabled {
		t.Error("defaults lost: SerenaEnabled should be true")
	}
}

func TestConfig_Load_GitFallbackRoot(t *testing.T) {
	dir := t.TempDir()
	// Only .git/ exists — no .ralph/, detectProjectRoot falls back to .git
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ProjectRoot != dir {
		t.Errorf("ProjectRoot = %q, want %q", cfg.ProjectRoot, dir)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50 (default)", cfg.MaxTurns)
	}
}

// --- Task 5: detectProjectRootFrom tests ---

func TestConfig_DetectProjectRootFrom_RalphInCWD(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}

	got := detectProjectRootFrom(dir)
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestConfig_DetectProjectRootFrom_RalphInParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(filepath.Join(parent, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}

	got := detectProjectRootFrom(child)
	if got != parent {
		t.Errorf("got %q, want %q (parent with .ralph/)", got, parent)
	}
}

func TestConfig_DetectProjectRootFrom_GitFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	got := detectProjectRootFrom(dir)
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestConfig_DetectProjectRootFrom_NeitherFound(t *testing.T) {
	dir := t.TempDir()

	got := detectProjectRootFrom(dir)
	if got != dir {
		t.Errorf("got %q, want %q (CWD fallback)", got, dir)
	}
}

func TestConfig_DetectProjectRootFrom_RalphPriority(t *testing.T) {
	// .ralph/ in grandparent, .git/ in parent -> returns grandparent
	grandparent := t.TempDir()
	parent := filepath.Join(grandparent, "sub")
	child := filepath.Join(parent, "deep")
	if err := os.MkdirAll(filepath.Join(grandparent, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(parent, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}

	got := detectProjectRootFrom(child)
	if got != grandparent {
		t.Errorf("got %q, want %q (grandparent with .ralph/ takes priority)", got, grandparent)
	}
}
