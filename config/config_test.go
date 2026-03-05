package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// intPtr returns a pointer to the given int value.
func intPtr(v int) *int { return &v }

// boolPtr returns a pointer to the given bool value.
func boolPtr(v bool) *bool { return &v }

// strPtr returns a pointer to the given string value.
func strPtr(v string) *string { return &v }

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
serena_sync_enabled: true
serena_sync_max_turns: 10
serena_sync_trigger: "run"
learnings_budget: 500
stuck_threshold: 5
budget_max_usd: 25.0
budget_warn_pct: 70
model_pricing:
  custom-model:
    input_per_1m: 5.0
    output_per_1m: 25.0
    cache_per_1m: 0.50
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
	if !cfg.SerenaSyncEnabled {
		t.Error("SerenaSyncEnabled = false, want true")
	}
	if cfg.SerenaSyncMaxTurns != 10 {
		t.Errorf("SerenaSyncMaxTurns = %d, want 10", cfg.SerenaSyncMaxTurns)
	}
	if cfg.SerenaSyncTrigger != "run" {
		t.Errorf("SerenaSyncTrigger = %q, want %q", cfg.SerenaSyncTrigger, "run")
	}
	if cfg.LearningsBudget != 500 {
		t.Errorf("LearningsBudget = %d, want 500", cfg.LearningsBudget)
	}
	if cfg.StuckThreshold != 5 {
		t.Errorf("StuckThreshold = %d, want 5", cfg.StuckThreshold)
	}
	if cfg.BudgetMaxUSD != 25.0 {
		t.Errorf("BudgetMaxUSD = %f, want 25.0", cfg.BudgetMaxUSD)
	}
	if cfg.BudgetWarnPct != 70 {
		t.Errorf("BudgetWarnPct = %d, want 70", cfg.BudgetWarnPct)
	}
	if len(cfg.ModelPricing) != 1 {
		t.Fatalf("ModelPricing len = %d, want 1", len(cfg.ModelPricing))
	}
	if p, ok := cfg.ModelPricing["custom-model"]; !ok {
		t.Error("ModelPricing missing key \"custom-model\"")
	} else {
		if p.InputPer1M != 5.0 {
			t.Errorf("ModelPricing[custom-model].InputPer1M = %f, want 5.0", p.InputPer1M)
		}
		if p.OutputPer1M != 25.0 {
			t.Errorf("ModelPricing[custom-model].OutputPer1M = %f, want 25.0", p.OutputPer1M)
		}
		if p.CachePer1M != 0.50 {
			t.Errorf("ModelPricing[custom-model].CachePer1M = %f, want 0.50", p.CachePer1M)
		}
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
			"claude_command: cc\nreview_every: 5\nserena_enabled: false\n",
			func(t *testing.T, cfg *Config) {
				if cfg.ClaudeCommand != "cc" {
					t.Errorf("ClaudeCommand = %q, want %q", cfg.ClaudeCommand, "cc")
				}
				if cfg.ReviewEvery != 5 {
					t.Errorf("ReviewEvery = %d, want 5", cfg.ReviewEvery)
				}
				if cfg.SerenaEnabled {
					t.Error("SerenaEnabled = true, want false")
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

// TestConfig_Load_TypeMismatch covers the yaml.Unmarshal-into-cfg error path
// at config.go:130-132. The probe (map) succeeds on valid YAML, but struct
// unmarshal fails when a sequence is provided for an int field.
func TestConfig_Load_TypeMismatch(t *testing.T) {
	dir := t.TempDir()
	// Valid YAML — probe map succeeds; struct unmarshal fails ([]string into int).
	writeConfigYAML(t, dir, "max_turns: [a, b]\n")
	t.Chdir(dir)

	_, err := Load(CLIFlags{})
	if err == nil {
		t.Fatal("expected error for type mismatch, got nil")
	}
	// String check justified: yaml.v3 doesn't export type error types.
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
	if cfg.ModelExecute != "claude-opus-4-6" {
		t.Errorf("ModelExecute = %q, want %q", cfg.ModelExecute, "claude-opus-4-6")
	}
	if cfg.ModelReview != "claude-sonnet-4-6" {
		t.Errorf("ModelReview = %q, want %q", cfg.ModelReview, "claude-sonnet-4-6")
	}
	if cfg.ModelReviewLight != "claude-haiku-4-5-20251001" {
		t.Errorf("ModelReviewLight = %q, want %q", cfg.ModelReviewLight, "claude-haiku-4-5-20251001")
	}
	if cfg.ReviewLightMaxFiles != 5 {
		t.Errorf("ReviewLightMaxFiles = %d, want 5", cfg.ReviewLightMaxFiles)
	}
	if cfg.ReviewLightMaxLines != 50 {
		t.Errorf("ReviewLightMaxLines = %d, want 50", cfg.ReviewLightMaxLines)
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
	if cfg.LearningsBudget != 200 {
		t.Errorf("LearningsBudget = %d, want 200", cfg.LearningsBudget)
	}
	if cfg.DistillCooldown != 5 {
		t.Errorf("DistillCooldown = %d, want 5", cfg.DistillCooldown)
	}
	if cfg.DistillTimeout != 120 {
		t.Errorf("DistillTimeout = %d, want 120", cfg.DistillTimeout)
	}
	if cfg.StuckThreshold != 2 {
		t.Errorf("StuckThreshold = %d, want 2", cfg.StuckThreshold)
	}
	if cfg.SimilarityWindow != 0 {
		t.Errorf("SimilarityWindow = %d, want 0", cfg.SimilarityWindow)
	}
	if cfg.SimilarityWarn != 0.85 {
		t.Errorf("SimilarityWarn = %f, want 0.85", cfg.SimilarityWarn)
	}
	if cfg.SimilarityHard != 0.95 {
		t.Errorf("SimilarityHard = %f, want 0.95", cfg.SimilarityHard)
	}
	if cfg.BudgetMaxUSD != 0 {
		t.Errorf("BudgetMaxUSD = %f, want 0", cfg.BudgetMaxUSD)
	}
	if cfg.BudgetWarnPct != 80 {
		t.Errorf("BudgetWarnPct = %d, want 80", cfg.BudgetWarnPct)
	}
	if cfg.ModelPricing != nil {
		t.Errorf("ModelPricing = %v, want nil (no default)", cfg.ModelPricing)
	}
	if cfg.LogDir != ".ralph/logs" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, ".ralph/logs")
	}
	if cfg.StoriesDir != "docs/sprint-artifacts" {
		t.Errorf("StoriesDir = %q, want %q", cfg.StoriesDir, "docs/sprint-artifacts")
	}
	if cfg.ProjectRoot == "" {
		t.Error("ProjectRoot should not be empty")
	}
	if cfg.SerenaSyncEnabled {
		t.Error("SerenaSyncEnabled should default to false")
	}
	if cfg.SerenaSyncMaxTurns != 5 {
		t.Errorf("SerenaSyncMaxTurns = %d, want 5", cfg.SerenaSyncMaxTurns)
	}
	if cfg.SerenaSyncTrigger != "task" {
		t.Errorf("SerenaSyncTrigger = %q, want %q", cfg.SerenaSyncTrigger, "task")
	}
}

func TestConfig_Load_StoriesDirFromFile(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "stories_dir: custom/stories\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StoriesDir != "custom/stories" {
		t.Errorf("StoriesDir = %q, want %q", cfg.StoriesDir, "custom/stories")
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

// --- Story 1.4: CLI override and cascade tests ---

func TestConfig_Load_CLIOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "max_turns: 75\ngates_enabled: true\nmodel_execute: sonnet\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{
		MaxTurns:     intPtr(100),
		GatesEnabled: boolPtr(false),
		ModelExecute: strPtr("haiku"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 100 {
		t.Errorf("MaxTurns = %d, want 100 (CLI override)", cfg.MaxTurns)
	}
	if cfg.GatesEnabled {
		t.Error("GatesEnabled = true, want false (CLI override)")
	}
	if cfg.ModelExecute != "haiku" {
		t.Errorf("ModelExecute = %q, want %q (CLI override)", cfg.ModelExecute, "haiku")
	}
}

func TestConfig_Load_ConfigOverridesEmbedded(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "max_turns: 75\ngates_enabled: true\nmodel_execute: sonnet\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 75 {
		t.Errorf("MaxTurns = %d, want 75 (config override of embedded 50)", cfg.MaxTurns)
	}
	if !cfg.GatesEnabled {
		t.Error("GatesEnabled = false, want true (config override of embedded false)")
	}
	if cfg.ModelExecute != "sonnet" {
		t.Errorf("ModelExecute = %q, want %q (config override of embedded empty)", cfg.ModelExecute, "sonnet")
	}
}

func TestConfig_Load_EmbeddedDefaultUsed(t *testing.T) {
	dir := t.TempDir()
	// .ralph/ exists but no config.yaml → all embedded defaults
	if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50 (embedded default)", cfg.MaxTurns)
	}
	if cfg.GatesEnabled {
		t.Error("GatesEnabled = true, want false (embedded default)")
	}
	if cfg.ModelExecute != "claude-opus-4-6" {
		t.Errorf("ModelExecute = %q, want %q (embedded default)", cfg.ModelExecute, "claude-opus-4-6")
	}
}

func TestConfig_Load_CascadeThreeLevels(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string   // config file content ("" = no config file created)
		noConfig bool     // true = no .ralph/config.yaml at all
		flags    CLIFlags // CLI flags
		wantInt  int      // expected MaxTurns
		wantBool bool     // expected GatesEnabled
		wantStr  string   // expected ModelExecute
	}{
		// int cascade (MaxTurns: embedded=50)
		{
			name:    "int/CLI overrides config",
			yaml:    "max_turns: 75\n",
			flags:   CLIFlags{MaxTurns: intPtr(100)},
			wantInt: 100,
			wantStr: "claude-opus-4-6",
		},
		{
			name:    "int/config overrides embedded",
			yaml:    "max_turns: 75\n",
			flags:   CLIFlags{},
			wantInt: 75,
			wantStr: "claude-opus-4-6",
		},
		{
			name:     "int/embedded default used",
			noConfig: true,
			flags:    CLIFlags{},
			wantInt:  50,
			wantStr:  "claude-opus-4-6",
		},
		{
			name:     "int/CLI overrides embedded no config",
			noConfig: true,
			flags:    CLIFlags{MaxTurns: intPtr(100)},
			wantInt:  100,
			wantStr:  "claude-opus-4-6",
		},
		// bool cascade (GatesEnabled: embedded=false)
		{
			name:     "bool/CLI true overrides config false",
			yaml:     "gates_enabled: false\n",
			flags:    CLIFlags{GatesEnabled: boolPtr(true)},
			wantBool: true,
			wantInt:  50,
			wantStr:  "claude-opus-4-6",
		},
		{
			name:     "bool/CLI false overrides config true",
			yaml:     "gates_enabled: true\n",
			flags:    CLIFlags{GatesEnabled: boolPtr(false)},
			wantBool: false,
			wantInt:  50,
			wantStr:  "claude-opus-4-6",
		},
		{
			name:     "bool/config overrides embedded",
			yaml:     "gates_enabled: true\n",
			flags:    CLIFlags{},
			wantBool: true,
			wantInt:  50,
			wantStr:  "claude-opus-4-6",
		},
		{
			name:     "bool/embedded default used",
			noConfig: true,
			flags:    CLIFlags{},
			wantBool: false,
			wantInt:  50,
			wantStr:  "claude-opus-4-6",
		},
		// string cascade (ModelExecute: embedded="claude-opus-4-6")
		{
			name:    "string/CLI overrides config",
			yaml:    "model_execute: sonnet\n",
			flags:   CLIFlags{ModelExecute: strPtr("haiku")},
			wantStr: "haiku",
			wantInt: 50,
		},
		{
			name:    "string/config overrides embedded",
			yaml:    "model_execute: sonnet\n",
			flags:   CLIFlags{},
			wantStr: "sonnet",
			wantInt: 50,
		},
		{
			name:     "string/embedded default used",
			noConfig: true,
			flags:    CLIFlags{},
			wantStr:  "claude-opus-4-6",
			wantInt:  50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.noConfig {
				if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeConfigYAML(t, dir, tt.yaml)
			}
			t.Chdir(dir)

			cfg, err := Load(tt.flags)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.MaxTurns != tt.wantInt {
				t.Errorf("MaxTurns = %d, want %d", cfg.MaxTurns, tt.wantInt)
			}
			if cfg.GatesEnabled != tt.wantBool {
				t.Errorf("GatesEnabled = %v, want %v", cfg.GatesEnabled, tt.wantBool)
			}
			if cfg.ModelExecute != tt.wantStr {
				t.Errorf("ModelExecute = %q, want %q", cfg.ModelExecute, tt.wantStr)
			}
		})
	}
}

func TestConfig_Load_CLIZeroOverridesNonZero(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "gates_checkpoint: 5\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{GatesCheckpoint: intPtr(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GatesCheckpoint != 0 {
		t.Errorf("GatesCheckpoint = %d, want 0 (CLI zero override)", cfg.GatesCheckpoint)
	}
}

func TestConfig_Load_CLIOverridesEmbeddedNoConfigFile(t *testing.T) {
	dir := t.TempDir()
	// .ralph/ exists but no config.yaml
	if err := os.MkdirAll(filepath.Join(dir, ".ralph"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{MaxTurns: intPtr(100), GatesEnabled: boolPtr(true)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 100 {
		t.Errorf("MaxTurns = %d, want 100 (CLI overrides embedded default of 50)", cfg.MaxTurns)
	}
	if !cfg.GatesEnabled {
		t.Error("GatesEnabled = false, want true (CLI overrides embedded default of false)")
	}
}

func TestConfig_Load_CLIOverridesEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfigYAML(t, dir, "---\n# empty config\n")
	t.Chdir(dir)

	cfg, err := Load(CLIFlags{MaxTurns: intPtr(200)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTurns != 200 {
		t.Errorf("MaxTurns = %d, want 200 (CLI override on empty config)", cfg.MaxTurns)
	}
}

func TestConfig_Load_AllCLIFlagsOverrideFullConfig(t *testing.T) {
	dir := t.TempDir()
	// Full config file with all parameters set to non-default values
	writeConfigYAML(t, dir, `claude_command: "custom-claude"
max_turns: 75
max_iterations: 5
max_review_iterations: 5
gates_enabled: true
gates_checkpoint: 3
review_every: 2
model_execute: "sonnet"
model_review: "opus"
review_min_severity: "HIGH"
always_extract: true
serena_enabled: false
serena_sync_enabled: true
serena_sync_max_turns: 10
serena_sync_trigger: "run"
learnings_budget: 500
log_dir: "/custom/logs"
`)
	t.Chdir(dir)

	// Set ALL 10 CLI flags to override config values
	cfg, err := Load(CLIFlags{
		MaxTurns:            intPtr(200),
		MaxIterations:       intPtr(10),
		MaxReviewIterations: intPtr(8),
		GatesEnabled:        boolPtr(false),
		GatesCheckpoint:     intPtr(0),
		ReviewEvery:         intPtr(5),
		ModelExecute:        strPtr("haiku"),
		ModelReview:         strPtr("haiku"),
		AlwaysExtract:       boolPtr(false),
		SerenaSyncEnabled:   boolPtr(false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 10 CLI-overridable fields must reflect CLI values
	if cfg.MaxTurns != 200 {
		t.Errorf("MaxTurns = %d, want 200", cfg.MaxTurns)
	}
	if cfg.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want 10", cfg.MaxIterations)
	}
	if cfg.MaxReviewIterations != 8 {
		t.Errorf("MaxReviewIterations = %d, want 8", cfg.MaxReviewIterations)
	}
	if cfg.GatesEnabled {
		t.Error("GatesEnabled = true, want false")
	}
	if cfg.GatesCheckpoint != 0 {
		t.Errorf("GatesCheckpoint = %d, want 0", cfg.GatesCheckpoint)
	}
	if cfg.ReviewEvery != 5 {
		t.Errorf("ReviewEvery = %d, want 5", cfg.ReviewEvery)
	}
	if cfg.ModelExecute != "haiku" {
		t.Errorf("ModelExecute = %q, want %q", cfg.ModelExecute, "haiku")
	}
	if cfg.ModelReview != "haiku" {
		t.Errorf("ModelReview = %q, want %q", cfg.ModelReview, "haiku")
	}
	if cfg.AlwaysExtract {
		t.Error("AlwaysExtract = true, want false")
	}
	if cfg.SerenaSyncEnabled {
		t.Error("SerenaSyncEnabled = true, want false (CLI override)")
	}

	// Config-only fields (no CLI flags) must retain config file values
	if cfg.ClaudeCommand != "custom-claude" {
		t.Errorf("ClaudeCommand = %q, want %q (config-only, not CLI-overridable)", cfg.ClaudeCommand, "custom-claude")
	}
	if cfg.ReviewMinSeverity != "HIGH" {
		t.Errorf("ReviewMinSeverity = %q, want %q (config-only)", cfg.ReviewMinSeverity, "HIGH")
	}
	if cfg.SerenaEnabled {
		t.Error("SerenaEnabled = true, want false (config-only, set to false in config)")
	}
	if cfg.LearningsBudget != 500 {
		t.Errorf("LearningsBudget = %d, want 500 (config-only)", cfg.LearningsBudget)
	}
	if cfg.LogDir != "/custom/logs" {
		t.Errorf("LogDir = %q, want %q (config-only)", cfg.LogDir, "/custom/logs")
	}
	if cfg.SerenaSyncMaxTurns != 10 {
		t.Errorf("SerenaSyncMaxTurns = %d, want 10 (config-only)", cfg.SerenaSyncMaxTurns)
	}
	if cfg.SerenaSyncTrigger != "run" {
		t.Errorf("SerenaSyncTrigger = %q, want %q (config-only)", cfg.SerenaSyncTrigger, "run")
	}
}

// --- Story 1.5: ResolvePath tests ---

// writeFile creates a file at the given path with the specified content,
// creating parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestConfig_ResolvePath(t *testing.T) {
	embedded := []byte("embedded content")

	tests := []struct {
		name           string
		setup          func(t *testing.T, projectDir, homeDir string)
		clearHome      bool // true = clear HOME/USERPROFILE so UserHomeDir fails
		embedded       []byte
		resolveName    string
		wantContent    string
		wantSource     string
		wantErr        bool
		wantErrContain string // substring that error message must contain
	}{
		{
			name: "project level found",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(projectDir, ".ralph", "agents", "quality.md"), "project content")
			},
			resolveName: "agents/quality.md",
			wantContent: "project content",
			wantSource:  "project",
		},
		{
			name: "global level fallback",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global content")
			},
			resolveName: "agents/quality.md",
			wantContent: "global content",
			wantSource:  "global",
		},
		{
			name:        "embedded fallback",
			setup:       func(t *testing.T, projectDir, homeDir string) {},
			embedded:    embedded,
			resolveName: "agents/quality.md",
			wantContent: "embedded content",
			wantSource:  "embedded",
		},
		{
			name: "project priority over global",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(projectDir, ".ralph", "agents", "quality.md"), "project wins")
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global loses")
			},
			resolveName: "agents/quality.md",
			wantContent: "project wins",
			wantSource:  "project",
		},
		{
			name:           "no file no embedded error",
			setup:          func(t *testing.T, projectDir, homeDir string) {},
			resolveName:    "agents/missing.md",
			wantErr:        true,
			wantErrContain: "config: resolve",
		},
		{
			name:           "empty embedded error",
			setup:          func(t *testing.T, projectDir, homeDir string) {},
			embedded:       []byte{},
			resolveName:    "agents/quality.md",
			wantErr:        true,
			wantErrContain: "config: resolve",
		},
		{
			name: "flat name without subdirectory",
			setup: func(t *testing.T, projectDir, homeDir string) {
				writeFile(t, filepath.Join(projectDir, ".ralph", "quality.md"), "flat content")
			},
			resolveName: "quality.md",
			wantContent: "flat content",
			wantSource:  "project",
		},
		{
			name: "UserHomeDir failure skips global uses embedded",
			setup: func(t *testing.T, projectDir, homeDir string) {
				// Global file exists but HOME/USERPROFILE cleared so UserHomeDir fails
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global content")
			},
			clearHome:   true,
			embedded:    embedded,
			resolveName: "agents/quality.md",
			wantContent: "embedded content",
			wantSource:  "embedded",
		},
		{
			name: "UserHomeDir failure no embedded error",
			setup: func(t *testing.T, projectDir, homeDir string) {
				// No project file, HOME cleared so global skipped, no embedded → error
			},
			clearHome:      true,
			resolveName:    "agents/quality.md",
			wantErr:        true,
			wantErrContain: "agents/quality.md",
		},
		{
			name: "unreadable project falls through to global",
			setup: func(t *testing.T, projectDir, homeDir string) {
				// Create project path as DIRECTORY so os.ReadFile fails with non-NotExist error
				if err := os.MkdirAll(filepath.Join(projectDir, ".ralph", "agents", "quality.md"), 0755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(homeDir, ".config", "ralph", "agents", "quality.md"), "global content")
			},
			resolveName: "agents/quality.md",
			wantContent: "global content",
			wantSource:  "global",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			homeDir := t.TempDir()

			if tt.clearHome {
				// Clear both HOME (Linux) and USERPROFILE (Windows) so
				// os.UserHomeDir() returns an error on all platforms.
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
				t.Setenv("HOMEDRIVE", "")
				t.Setenv("HOMEPATH", "")
			} else {
				// Set both HOME (Linux) and USERPROFILE (Windows) so
				// os.UserHomeDir() returns homeDir on all platforms.
				t.Setenv("HOME", homeDir)
				t.Setenv("USERPROFILE", homeDir)
			}

			tt.setup(t, projectDir, homeDir)

			cfg := &Config{ProjectRoot: projectDir}
			content, source, err := cfg.ResolvePath(tt.resolveName, tt.embedded)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("content = %q, want %q", string(content), tt.wantContent)
			}
			if source != tt.wantSource {
				t.Errorf("source = %q, want %q", source, tt.wantSource)
			}
		})
	}
}

func TestConfig_Validate_HappyPath(t *testing.T) {
	cfg := Config{
		MaxTurns:            50,
		MaxIterations:       3,
		MaxReviewIterations: 3,
		ReviewMinSeverity:   "LOW",
		GatesCheckpoint:     0,
		DistillCooldown:     5,
		DistillTimeout:      120,
		LearningsBudget:     200,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestConfig_Validate_Errors(t *testing.T) {
	valid := Config{
		MaxTurns:            50,
		MaxIterations:       3,
		MaxReviewIterations: 3,
		ReviewMinSeverity:   "",
		GatesCheckpoint:     0,
		DistillCooldown:     5,
		DistillTimeout:      120,
		LearningsBudget:     200,
	}

	tests := []struct {
		name        string
		mutate      func(*Config)
		errContains string
	}{
		{
			name:        "MaxTurns zero",
			mutate:      func(c *Config) { c.MaxTurns = 0 },
			errContains: "max_turns must be > 0",
		},
		{
			name:        "MaxTurns negative",
			mutate:      func(c *Config) { c.MaxTurns = -1 },
			errContains: "max_turns must be > 0",
		},
		{
			name:        "MaxIterations zero",
			mutate:      func(c *Config) { c.MaxIterations = 0 },
			errContains: "max_iterations must be > 0",
		},
		{
			name:        "MaxReviewIterations zero",
			mutate:      func(c *Config) { c.MaxReviewIterations = 0 },
			errContains: "max_review_iterations must be > 0",
		},
		{
			name:        "ReviewMinSeverity invalid",
			mutate:      func(c *Config) { c.ReviewMinSeverity = "CRITICAL" },
			errContains: "review_min_severity must be HIGH, MEDIUM, LOW, or empty",
		},
		{
			name:        "GatesCheckpoint negative",
			mutate:      func(c *Config) { c.GatesCheckpoint = -1 },
			errContains: "gates_checkpoint must be >= 0",
		},
		{
			name:        "DistillCooldown negative",
			mutate:      func(c *Config) { c.DistillCooldown = -1 },
			errContains: "distill_cooldown must be >= 0",
		},
		{
			name:        "DistillTimeout zero",
			mutate:      func(c *Config) { c.DistillTimeout = 0 },
			errContains: "distill_timeout must be > 0",
		},
		{
			name:        "LearningsBudget zero",
			mutate:      func(c *Config) { c.LearningsBudget = 0 },
			errContains: "learnings_budget must be > 0",
		},
		{
			name:        "StuckThreshold negative",
			mutate:      func(c *Config) { c.StuckThreshold = -1 },
			errContains: "stuck_threshold must be >= 0",
		},
		{
			name:        "SimilarityWindow negative",
			mutate:      func(c *Config) { c.SimilarityWindow = -1 },
			errContains: "similarity_window must be >= 0",
		},
		{
			name: "SimilarityWarn zero when enabled",
			mutate: func(c *Config) {
				c.SimilarityWindow = 3
				c.SimilarityWarn = 0.0
				c.SimilarityHard = 0.95
			},
			errContains: "similarity_warn must be in (0.0, 1.0)",
		},
		{
			name: "SimilarityWarn one when enabled",
			mutate: func(c *Config) {
				c.SimilarityWindow = 3
				c.SimilarityWarn = 1.0
				c.SimilarityHard = 0.95
			},
			errContains: "similarity_warn must be in (0.0, 1.0)",
		},
		{
			name: "SimilarityHard zero when enabled",
			mutate: func(c *Config) {
				c.SimilarityWindow = 3
				c.SimilarityWarn = 0.5
				c.SimilarityHard = 0.0
			},
			errContains: "similarity_hard must be in (0.0, 1.0)",
		},
		{
			name: "SimilarityHard one when enabled",
			mutate: func(c *Config) {
				c.SimilarityWindow = 3
				c.SimilarityWarn = 0.5
				c.SimilarityHard = 1.0
			},
			errContains: "similarity_hard must be in (0.0, 1.0)",
		},
		{
			name: "SimilarityWarn >= SimilarityHard",
			mutate: func(c *Config) {
				c.SimilarityWindow = 3
				c.SimilarityWarn = 0.95
				c.SimilarityHard = 0.85
			},
			errContains: "similarity_warn must be less than similarity_hard",
		},
		{
			name: "SimilarityWarn == SimilarityHard",
			mutate: func(c *Config) {
				c.SimilarityWindow = 3
				c.SimilarityWarn = 0.85
				c.SimilarityHard = 0.85
			},
			errContains: "similarity_warn must be less than similarity_hard",
		},
		{
			name:        "BudgetMaxUSD negative",
			mutate:      func(c *Config) { c.BudgetMaxUSD = -1.0 },
			errContains: "budget_max_usd must be >= 0",
		},
		{
			name: "BudgetWarnPct zero when budget enabled",
			mutate: func(c *Config) {
				c.BudgetMaxUSD = 10.0
				c.BudgetWarnPct = 0
			},
			errContains: "budget_warn_pct must be 1-99",
		},
		{
			name: "BudgetWarnPct 100 when budget enabled",
			mutate: func(c *Config) {
				c.BudgetMaxUSD = 10.0
				c.BudgetWarnPct = 100
			},
			errContains: "budget_warn_pct must be 1-99",
		},
		{
			name:        "SerenaSyncTrigger invalid",
			mutate:      func(c *Config) { c.SerenaSyncTrigger = "invalid" },
			errContains: `invalid serena_sync_trigger "invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid // copy
			tt.mutate(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate: expected error, got nil")
			}
			if !strings.Contains(err.Error(), "config: validate:") {
				t.Errorf("error = %q, want containing %q", err.Error(), "config: validate:")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestConfig_Validate_SimilarityDisabledSkipsThresholdValidation(t *testing.T) {
	cfg := Config{
		MaxTurns:            1,
		MaxIterations:       1,
		MaxReviewIterations: 1,
		DistillTimeout:      60,
		LearningsBudget:     100,
		SimilarityWindow:    0,    // disabled
		SimilarityWarn:      0.0,  // would be invalid if enabled
		SimilarityHard:      -1.0, // would be invalid if enabled
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: similarity disabled should skip threshold validation, got: %v", err)
	}
}

func TestConfig_Validate_SimilarityEnabledValid(t *testing.T) {
	cfg := Config{
		MaxTurns:            1,
		MaxIterations:       1,
		MaxReviewIterations: 1,
		DistillTimeout:      60,
		LearningsBudget:     100,
		SimilarityWindow:    3,
		SimilarityWarn:      0.85,
		SimilarityHard:      0.95,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: valid similarity config should pass, got: %v", err)
	}
}

func TestConfig_Validate_EmptySeverity(t *testing.T) {
	cfg := Config{
		MaxTurns:            1,
		MaxIterations:       1,
		MaxReviewIterations: 1,
		ReviewMinSeverity:   "",
		GatesCheckpoint:     0,
		DistillCooldown:     0,
		DistillTimeout:      60,
		LearningsBudget:     100,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: empty severity should be valid, got: %v", err)
	}
}

// TestConfig_Validate_SerenaSyncTrigger covers AC#3: valid/invalid trigger values.
func TestConfig_Validate_SerenaSyncTrigger(t *testing.T) {
	base := Config{
		MaxTurns:            50,
		MaxIterations:       3,
		MaxReviewIterations: 3,
		DistillCooldown:     5,
		DistillTimeout:      120,
		LearningsBudget:     200,
		SerenaSyncMaxTurns:  5,
	}

	tests := []struct {
		name    string
		trigger string
		wantErr bool
		errMsg  string
	}{
		{"task valid", "task", false, ""},
		{"run valid", "run", false, ""},
		{"empty valid", "", false, ""},
		{"invalid value", "invalid", true, `config: validate: invalid serena_sync_trigger "invalid" (must be "run" or "task")`},
		{"unknown value", "hourly", true, `config: validate: invalid serena_sync_trigger "hourly" (must be "run" or "task")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			cfg.SerenaSyncTrigger = tt.trigger
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Validate: expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate: unexpected error: %v", err)
				}
			}
		})
	}
}

// TestConfig_Validate_SerenaSyncMaxTurns covers AC#4: max turns correction behavior.
func TestConfig_Validate_SerenaSyncMaxTurns(t *testing.T) {
	base := Config{
		MaxTurns:            50,
		MaxIterations:       3,
		MaxReviewIterations: 3,
		DistillCooldown:     5,
		DistillTimeout:      120,
		LearningsBudget:     200,
		SerenaSyncTrigger:   "task",
	}

	tests := []struct {
		name     string
		maxTurns int
		want     int
	}{
		{"zero corrected to 5", 0, 5},
		{"negative corrected to 5", -10, 5},
		{"one stays 1", 1, 1},
		{"five stays 5", 5, 5},
		{"hundred stays 100", 100, 100},
		{"beyond range stays 200", 200, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			cfg.SerenaSyncMaxTurns = tt.maxTurns
			err := cfg.Validate()
			if err != nil {
				t.Fatalf("Validate: unexpected error: %v", err)
			}
			if cfg.SerenaSyncMaxTurns != tt.want {
				t.Errorf("SerenaSyncMaxTurns = %d, want %d", cfg.SerenaSyncMaxTurns, tt.want)
			}
		})
	}
}

// TestConfig_Load_SerenaSyncFields covers AC#5: round-trip with/without serena_sync fields.
func TestConfig_Load_SerenaSyncFields(t *testing.T) {
	t.Run("all fields from YAML", func(t *testing.T) {
		dir := t.TempDir()
		writeConfigYAML(t, dir, `serena_sync_enabled: true
serena_sync_max_turns: 10
serena_sync_trigger: "run"
`)
		t.Chdir(dir)

		cfg, err := Load(CLIFlags{})
		if err != nil {
			t.Fatalf("Load: unexpected error: %v", err)
		}
		if !cfg.SerenaSyncEnabled {
			t.Error("SerenaSyncEnabled = false, want true")
		}
		if cfg.SerenaSyncMaxTurns != 10 {
			t.Errorf("SerenaSyncMaxTurns = %d, want 10", cfg.SerenaSyncMaxTurns)
		}
		if cfg.SerenaSyncTrigger != "run" {
			t.Errorf("SerenaSyncTrigger = %q, want %q", cfg.SerenaSyncTrigger, "run")
		}
	})

	t.Run("defaults when absent", func(t *testing.T) {
		dir := t.TempDir()
		writeConfigYAML(t, dir, "max_turns: 50\n")
		t.Chdir(dir)

		cfg, err := Load(CLIFlags{})
		if err != nil {
			t.Fatalf("Load: unexpected error: %v", err)
		}
		if cfg.SerenaSyncEnabled {
			t.Error("SerenaSyncEnabled = true, want false (default)")
		}
		if cfg.SerenaSyncMaxTurns != 5 {
			t.Errorf("SerenaSyncMaxTurns = %d, want 5 (default)", cfg.SerenaSyncMaxTurns)
		}
		if cfg.SerenaSyncTrigger != "task" {
			t.Errorf("SerenaSyncTrigger = %q, want %q (default)", cfg.SerenaSyncTrigger, "task")
		}
	})
}

// TestConfig_ApplyCLIFlags_SerenaSyncEnabled covers AC#2: CLI override for SerenaSyncEnabled.
func TestConfig_ApplyCLIFlags_SerenaSyncEnabled(t *testing.T) {
	tests := []struct {
		name       string
		configVal  bool
		cliVal     *bool
		wantResult bool
	}{
		{"CLI true overrides config false", false, boolPtr(true), true},
		{"CLI false overrides config true", true, boolPtr(false), false},
		{"CLI nil keeps config value", true, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			yaml := "serena_sync_enabled: " + fmt.Sprintf("%t", tt.configVal) + "\n"
			writeConfigYAML(t, dir, yaml)
			t.Chdir(dir)

			cfg, err := Load(CLIFlags{SerenaSyncEnabled: tt.cliVal})
			if err != nil {
				t.Fatalf("Load: unexpected error: %v", err)
			}
			if cfg.SerenaSyncEnabled != tt.wantResult {
				t.Errorf("SerenaSyncEnabled = %v, want %v", cfg.SerenaSyncEnabled, tt.wantResult)
			}
		})
	}
}
