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
	want := map[string]bool{"bridge": false, "run": false, "distill": false}

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

func TestBridgeCmd_Usage(t *testing.T) {
	if bridgeCmd.Use == "" {
		t.Error("bridgeCmd.Use is empty")
	}
	if bridgeCmd.Use != "bridge [story-files...]" {
		t.Errorf("bridgeCmd.Use = %q, want %q", bridgeCmd.Use, "bridge [story-files...]")
	}
	if !strings.Contains(bridgeCmd.Long, "auto-discovers") {
		t.Error("bridgeCmd.Long does not mention auto-discovers")
	}
	if !strings.Contains(bridgeCmd.Long, "StoriesDir") {
		t.Error("bridgeCmd.Long does not mention StoriesDir")
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

// --- runBridge error path tests ---

func TestRunBridge_ConfigLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	writeInvalidConfig(t, tmpDir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := &cobra.Command{Use: "bridge"}
	cmd.SetContext(context.Background())

	err := runBridge(cmd, []string{"story.md"})
	if err == nil {
		t.Fatal("runBridge: expected error when config.Load fails (invalid config.yaml)")
	}
	if !strings.Contains(err.Error(), "ralph: load config:") {
		t.Errorf("runBridge error = %q, want containing %q", err.Error(), "ralph: load config:")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("runBridge error = %q, want inner error containing %q", err.Error(), "config:")
	}
}

func TestRunBridge_StoryReadError(t *testing.T) {
	tmpDir := t.TempDir()
	writeValidConfig(t, tmpDir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := &cobra.Command{Use: "bridge"}
	cmd.SetContext(context.Background())

	err := runBridge(cmd, []string{filepath.Join(tmpDir, "nonexistent-story.md")})
	if err == nil {
		t.Fatal("runBridge: expected error for nonexistent story file")
	}
	if !strings.Contains(err.Error(), "bridge: read story:") {
		t.Errorf("runBridge error = %q, want containing %q", err.Error(), "bridge: read story:")
	}
}

// --- runBridge autodiscovery tests ---

func TestRunBridge_AutoDiscover_FindsMdFiles(t *testing.T) {
	tmpDir := t.TempDir()
	writeValidConfig(t, tmpDir)

	// Create StoriesDir with .md entries as directories — bridge.Run will fail
	// with "bridge: read story:" (not "no story files found"), confirming that
	// autodiscovery found the files and passed them to bridge.Run. Using
	// directories avoids Windows file-lock issues during TempDir cleanup.
	storiesDir := filepath.Join(tmpDir, "docs", "sprint-artifacts")
	for _, name := range []string{"story-a.md", "story-b.md"} {
		if err := os.MkdirAll(filepath.Join(storiesDir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := &cobra.Command{Use: "bridge"}
	cmd.SetContext(context.Background())

	// No args — autodiscovery must find the .md entries and pass them to
	// bridge.Run, which fails with "bridge: read story:" (not "no story files found").
	err := runBridge(cmd, []string{})
	if err == nil {
		t.Fatal("runBridge: expected error from bridge.Run reading directory-as-file, got nil")
	}
	if strings.Contains(err.Error(), "no story files found") {
		t.Errorf("runBridge: got 'no story files found' but .md entries exist in StoriesDir: %v", err)
	}
	if !strings.Contains(err.Error(), "bridge: read story:") {
		t.Errorf("runBridge error = %q, want containing %q (autodiscovery passed files to bridge.Run)", err.Error(), "bridge: read story:")
	}
}

func TestRunBridge_AutoDiscover_NoFiles_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	writeValidConfig(t, tmpDir)

	// Create StoriesDir but leave it empty (no .md files)
	storiesDir := filepath.Join(tmpDir, "docs", "sprint-artifacts")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := &cobra.Command{Use: "bridge"}
	cmd.SetContext(context.Background())

	err := runBridge(cmd, []string{})
	if err == nil {
		t.Fatal("runBridge: expected error when no .md files found, got nil")
	}
	if !strings.Contains(err.Error(), "no story files found") {
		t.Errorf("runBridge error = %q, want containing %q", err.Error(), "no story files found")
	}
	if !strings.Contains(err.Error(), "ralph bridge <file.md>") {
		t.Errorf("runBridge error = %q, want hint containing %q", err.Error(), "ralph bridge <file.md>")
	}
}

func TestRunBridge_AutoDiscover_StoriesDirMissing_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	writeValidConfig(t, tmpDir)
	// StoriesDir does not exist at all — glob returns no matches (not an error from filepath.Glob)

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) }) //nolint:errcheck

	cmd := &cobra.Command{Use: "bridge"}
	cmd.SetContext(context.Background())

	err := runBridge(cmd, []string{})
	if err == nil {
		t.Fatal("runBridge: expected error when StoriesDir missing, got nil")
	}
	if !strings.Contains(err.Error(), "no story files found") {
		t.Errorf("runBridge error = %q, want containing %q", err.Error(), "no story files found")
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

func TestSplitBySize_Cases(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create test files of known sizes
	writeFile := func(name string, size int) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, size), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	f100 := writeFile("f100", 100)
	f200 := writeFile("f200", 200)
	f300 := writeFile("f300", 300)

	tests := []struct {
		name       string
		files      []string
		maxBytes   int64
		wantBatches int
		wantFiles   []int // files per batch
	}{
		{
			name:        "empty input",
			files:       nil,
			maxBytes:    1000,
			wantBatches: 0,
		},
		{
			name:        "all fit in one batch",
			files:       []string{f100, f200},
			maxBytes:    500,
			wantBatches: 1,
			wantFiles:   []int{2},
		},
		{
			name:        "split into two batches",
			files:       []string{f100, f200, f300},
			maxBytes:    350,
			wantBatches: 2,
			wantFiles:   []int{2, 1},
		},
		{
			name:        "each file in own batch",
			files:       []string{f100, f200, f300},
			maxBytes:    100,
			wantBatches: 3,
			wantFiles:   []int{1, 1, 1},
		},
		{
			name:        "nonexistent file treated as zero size",
			files:       []string{filepath.Join(dir, "missing"), f100},
			maxBytes:    500,
			wantBatches: 1,
			wantFiles:   []int{2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitBySize(tc.files, tc.maxBytes)
			if len(got) != tc.wantBatches {
				t.Fatalf("splitBySize batches = %d, want %d", len(got), tc.wantBatches)
			}
			for i, wantCount := range tc.wantFiles {
				if len(got[i]) != wantCount {
					t.Errorf("batch[%d] files = %d, want %d", i, len(got[i]), wantCount)
				}
			}
		})
	}
}
