package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/spf13/cobra"
)

func TestRootCmd_HasSubcommands(t *testing.T) {
	want := map[string]bool{"run": false, "distill": false, "plan": false, "replan": false, "init": false}

	for _, cmd := range rootCmd.Commands() {
		if _, ok := want[cmd.Name()]; ok {
			want[cmd.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered on root command", name)
		}
	}
}

func TestRootCmd_Version(t *testing.T) {
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version is empty")
	}
	if rootCmd.Version != version {
		t.Errorf("rootCmd.Version = %q, want %q", rootCmd.Version, version)
	}
}

func TestRunCmd_Flags(t *testing.T) {
	tests := []struct {
		flag     string
		flagType string
		defValue string
	}{
		{"max-turns", "int", "0"},
		{"gates", "bool", "false"},
		{"every", "int", "0"},
		{"model", "string", ""},
		{"always-extract", "bool", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			f := runCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag --%s not defined on run command", tt.flag)
			}
			if f.Value.Type() != tt.flagType {
				t.Errorf("flag --%s type = %q, want %q", tt.flag, f.Value.Type(), tt.flagType)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag --%s default = %q, want %q", tt.flag, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestBuildCLIFlags_WiringCorrectness(t *testing.T) {
	// Create a fresh command with the same flags as runCmd
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("max-turns", 0, "")
	cmd.Flags().Bool("gates", false, "")
	cmd.Flags().Int("every", 0, "")
	cmd.Flags().String("model", "", "")
	cmd.Flags().Bool("always-extract", false, "")

	// Set all flags explicitly
	cmd.Flags().Set("max-turns", "42")
	cmd.Flags().Set("gates", "true")
	cmd.Flags().Set("every", "5")
	cmd.Flags().Set("model", "claude-sonnet")
	cmd.Flags().Set("always-extract", "true")

	flags := buildCLIFlags(cmd)

	// Verify each flag maps to the correct CLIFlags field
	if flags.MaxTurns == nil || *flags.MaxTurns != 42 {
		t.Errorf("MaxTurns = %v, want 42", flags.MaxTurns)
	}
	if flags.GatesEnabled == nil || *flags.GatesEnabled != true {
		t.Errorf("GatesEnabled = %v, want true", flags.GatesEnabled)
	}
	if flags.GatesCheckpoint == nil || *flags.GatesCheckpoint != 5 {
		t.Errorf("GatesCheckpoint = %v, want 5", flags.GatesCheckpoint)
	}
	if flags.ModelExecute == nil || *flags.ModelExecute != "claude-sonnet" {
		t.Errorf("ModelExecute = %v, want claude-sonnet", flags.ModelExecute)
	}
	if flags.AlwaysExtract == nil || *flags.AlwaysExtract != true {
		t.Errorf("AlwaysExtract = %v, want true", flags.AlwaysExtract)
	}

	// Verify unmapped fields remain nil
	if flags.MaxIterations != nil {
		t.Errorf("MaxIterations = %v, want nil (not wired in Story 1.13)", flags.MaxIterations)
	}
	if flags.MaxReviewIterations != nil {
		t.Errorf("MaxReviewIterations = %v, want nil (not wired in Story 1.13)", flags.MaxReviewIterations)
	}
	if flags.ReviewEvery != nil {
		t.Errorf("ReviewEvery = %v, want nil (not wired in Story 1.13)", flags.ReviewEvery)
	}
	if flags.ModelReview != nil {
		t.Errorf("ModelReview = %v, want nil (not wired in Story 1.13)", flags.ModelReview)
	}
}

func TestBuildCLIFlags_NoFlagsChanged(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("max-turns", 0, "")
	cmd.Flags().Bool("gates", false, "")
	cmd.Flags().Int("every", 0, "")
	cmd.Flags().String("model", "", "")
	cmd.Flags().Bool("always-extract", false, "")

	// No flags set — all should be nil
	flags := buildCLIFlags(cmd)

	if flags.MaxTurns != nil {
		t.Errorf("MaxTurns = %v, want nil", flags.MaxTurns)
	}
	if flags.GatesEnabled != nil {
		t.Errorf("GatesEnabled = %v, want nil", flags.GatesEnabled)
	}
	if flags.GatesCheckpoint != nil {
		t.Errorf("GatesCheckpoint = %v, want nil", flags.GatesCheckpoint)
	}
	if flags.ModelExecute != nil {
		t.Errorf("ModelExecute = %v, want nil", flags.ModelExecute)
	}
	if flags.AlwaysExtract != nil {
		t.Errorf("AlwaysExtract = %v, want nil", flags.AlwaysExtract)
	}
}

// --- Distill command tests (Story 6.6) ---

func TestDistillCmd_SubcommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "distill" {
			found = true
			break
		}
	}
	if !found {
		t.Error("distill subcommand not registered on rootCmd")
	}
}

func TestDistillCmd_HelpText(t *testing.T) {
	if distillCmd.Short == "" {
		t.Error("distillCmd.Short is empty")
	}
	if !strings.Contains(distillCmd.Long, "WARNING") {
		t.Error("distillCmd.Long does not contain advisory WARNING")
	}
	if !strings.Contains(distillCmd.Long, "concurrently") {
		t.Error("distillCmd.Long does not contain concurrent run advisory")
	}
}

func TestDistillCmd_MissingLearnings(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal ralph.yaml so config.Load works
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ralph.yaml"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Override working directory for config.Load
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	err := runDistill(distillCmd, nil)
	if err == nil {
		t.Fatal("runDistill: expected error for missing LEARNINGS.md")
	}

	var exitErr *config.ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.Code)
	}
	if !strings.Contains(exitErr.Message, "LEARNINGS.md not found") {
		t.Errorf("message = %q, want to contain %q", exitErr.Message, "LEARNINGS.md not found")
	}
}

func TestCountFileLines_ReturnsLineCount(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.md")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	count := countFileLines(path)
	if count != 3 {
		t.Errorf("countFileLines = %d, want 3", count)
	}
}

func TestCountFileLines_MissingFile(t *testing.T) {
	count := countFileLines("/nonexistent/file.md")
	if count != 0 {
		t.Errorf("countFileLines for missing file = %d, want 0", count)
	}
}

// --- runRun error path tests ---

// writeInvalidConfig creates a .ralph/config.yaml with invalid YAML so that
// config.Load fails with a parse error. detectProjectRootFrom uses .ralph/ as
// project root anchor, so the invalid file is found before any parent config.
func writeInvalidConfig(t *testing.T, dir string) {
	t.Helper()
	ralphDir := filepath.Join(dir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ralphDir, "config.yaml"), []byte("{invalid: yaml: content"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRunID_Format(t *testing.T) {
	id := generateRunID()

	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (36 chars, 4 dashes)
	if len(id) != 36 {
		t.Fatalf("generateRunID() len = %d, want 36", len(id))
	}

	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("generateRunID() parts = %d, want 5 dash-separated groups", len(parts))
	}
	wantLens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != wantLens[i] {
			t.Errorf("generateRunID() part[%d] len = %d, want %d", i, len(p), wantLens[i])
		}
	}

	// Version nibble = 4
	if parts[2][0] != '4' {
		t.Errorf("generateRunID() version nibble = %c, want '4'", parts[2][0])
	}

	// Variant nibble in {8, 9, a, b}
	variant := parts[3][0]
	if variant != '8' && variant != '9' && variant != 'a' && variant != 'b' {
		t.Errorf("generateRunID() variant nibble = %c, want one of 8/9/a/b", variant)
	}

	// Uniqueness: two calls should differ
	id2 := generateRunID()
	if id == id2 {
		t.Errorf("generateRunID() returned same ID twice: %s", id)
	}
}

func TestRunRun_ConfigLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	writeInvalidConfig(t, tmpDir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().Int("max-turns", 0, "")
	cmd.Flags().Bool("gates", false, "")
	cmd.Flags().Int("every", 0, "")
	cmd.Flags().String("model", "", "")
	cmd.Flags().Bool("always-extract", false, "")
	cmd.SetContext(context.Background())

	err := runRun(cmd, nil)
	if err == nil {
		t.Fatal("runRun: expected error when config.Load fails (invalid config.yaml)")
	}
	if !strings.Contains(err.Error(), "ralph: load config:") {
		t.Errorf("runRun error = %q, want containing %q", err.Error(), "ralph: load config:")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("runRun error = %q, want inner error containing %q", err.Error(), "config:")
	}
}

// writeValidConfig creates a .ralph/ directory so config.Load finds a project root
// and succeeds with defaults. No config.yaml needed (missing = defaults apply).
func writeValidConfig(t *testing.T, dir string) {
	t.Helper()
	ralphDir := filepath.Join(dir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatal(err)
	}
}

// --- runDistill error path tests ---

func TestRunDistill_ConfigLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	writeInvalidConfig(t, tmpDir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	err := runDistill(distillCmd, nil)
	if err == nil {
		t.Fatal("runDistill: expected error when config.Load fails (invalid config.yaml)")
	}
	if !strings.Contains(err.Error(), "ralph: load config:") {
		t.Errorf("runDistill error = %q, want containing %q", err.Error(), "ralph: load config:")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("runDistill error = %q, want inner error containing %q", err.Error(), "config:")
	}
}

func TestRunDistill_LoadStateError(t *testing.T) {
	// Invalid distill-state.json → runner.LoadDistillState fails → runDistill returns
	// "ralph: distill: runner: distill state: load:".
	tmpDir := t.TempDir()
	writeValidConfig(t, tmpDir)

	// Create LEARNINGS.md so os.Stat succeeds.
	if err := os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create invalid distill-state.json to make LoadDistillState fail.
	ralphDir := filepath.Join(tmpDir, ".ralph")
	if err := os.WriteFile(filepath.Join(ralphDir, "distill-state.json"), []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	err := runDistill(distillCmd, nil)
	if err == nil {
		t.Fatal("runDistill: expected error for invalid distill-state.json")
	}
	if !strings.Contains(err.Error(), "ralph: distill:") {
		t.Errorf("runDistill error = %q, want containing %q", err.Error(), "ralph: distill:")
	}
	if !strings.Contains(err.Error(), "runner: distill state: load:") {
		t.Errorf("runDistill error = %q, want inner error containing %q", err.Error(), "runner: distill state: load:")
	}
}

// --- Replan tests (Story 16.1) ---

func TestReplanCmd_SubcommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "replan" {
			found = true
			break
		}
	}
	if !found {
		t.Error("replan subcommand not registered on rootCmd")
	}
}

func TestExtractCompletedTasks_Mixed(t *testing.T) {
	content := "- [x] Task A done\n  source: prd.md#AC-1\n- [ ] Task B open\n  source: prd.md#AC-2\n- [x] Task C done\n  source: prd.md#AC-3\n"
	got := extractCompletedTasks(content)
	if !strings.Contains(got, "- [x] Task A done") {
		t.Errorf("extractCompletedTasks: missing Task A, got %q", got)
	}
	if !strings.Contains(got, "- [x] Task C done") {
		t.Errorf("extractCompletedTasks: missing Task C, got %q", got)
	}
	if strings.Contains(got, "- [ ] Task B") {
		t.Errorf("extractCompletedTasks: should not contain open task B, got %q", got)
	}
	if strings.Count(got, "- [x]") != 2 {
		t.Errorf("extractCompletedTasks: want 2 completed tasks, got %d", strings.Count(got, "- [x]"))
	}
}

func TestExtractCompletedTasks_None(t *testing.T) {
	content := "- [ ] Task A open\n- [ ] Task B open\n"
	got := extractCompletedTasks(content)
	if got != "" {
		t.Errorf("extractCompletedTasks: want empty, got %q", got)
	}
}

func TestRunReplan_NoExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	writeValidConfig(t, tmpDir)

	// Create a docs/ dir with prd.md so autodiscovery finds input
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "prd.md"), []byte("# PRD"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := replanCmd
	cmd.SetContext(context.Background())

	err := runReplan(cmd, nil)
	if err == nil {
		t.Fatal("runReplan: expected error when sprint-tasks.md missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("runReplan error = %q, want containing %q", err.Error(), "not found")
	}
	if !strings.Contains(err.Error(), "ralph plan") {
		t.Errorf("runReplan error = %q, want containing %q", err.Error(), "ralph plan")
	}
}

// --- Init tests (Story 16.2) ---

func TestInitCmd_SubcommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("init subcommand not registered on rootCmd")
	}
}

func TestSplitInitOutput_Valid(t *testing.T) {
	output := "# PRD\n\nSome content\n\n===FILE_SEPARATOR===\n\n# Architecture\n\nArch content"
	prd, arch, err := splitInitOutput(output)
	if err != nil {
		t.Fatalf("splitInitOutput: unexpected error: %v", err)
	}
	if !strings.Contains(prd, "# PRD") {
		t.Errorf("prd = %q, want containing '# PRD'", prd)
	}
	if !strings.Contains(arch, "# Architecture") {
		t.Errorf("arch = %q, want containing '# Architecture'", arch)
	}
}

func TestSplitInitOutput_NoSeparator(t *testing.T) {
	_, _, err := splitInitOutput("just some content without separator")
	if err == nil {
		t.Fatal("splitInitOutput: expected error when separator missing")
	}
	if !strings.Contains(err.Error(), "separator") {
		t.Errorf("error = %q, want containing 'separator'", err.Error())
	}
}

func TestSplitInitOutput_EmptyPrd(t *testing.T) {
	_, _, err := splitInitOutput("===FILE_SEPARATOR===\n# Arch content")
	if err == nil {
		t.Fatal("splitInitOutput: expected error when prd empty")
	}
	if !strings.Contains(err.Error(), "prd.md content is empty") {
		t.Errorf("error = %q, want containing 'prd.md content is empty'", err.Error())
	}
}

func TestSplitInitOutput_EmptyArch(t *testing.T) {
	_, _, err := splitInitOutput("# PRD content\n===FILE_SEPARATOR===\n  ")
	if err == nil {
		t.Fatal("splitInitOutput: expected error when arch empty")
	}
	if !strings.Contains(err.Error(), "architecture.md content is empty") {
		t.Errorf("error = %q, want containing 'architecture.md content is empty'", err.Error())
	}
}

func TestRunInit_ConfigLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	writeInvalidConfig(t, tmpDir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := initCmd
	cmd.SetContext(context.Background())

	err := runInit(cmd, []string{"test project"})
	if err == nil {
		t.Fatal("runInit: expected error when config.Load fails")
	}
	if !strings.Contains(err.Error(), "ralph: load config:") {
		t.Errorf("runInit error = %q, want containing %q", err.Error(), "ralph: load config:")
	}
}

