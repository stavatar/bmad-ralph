package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/spf13/cobra"
)

func TestRootCmd_HasSubcommands(t *testing.T) {
	want := map[string]bool{"bridge": false, "run": false}

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

func TestEnsureLogDir_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		ProjectRoot: tmpDir,
		LogDir:      ".ralph/logs",
	}

	err := ensureLogDir(cfg)
	if err != nil {
		t.Fatalf("ensureLogDir() error = %v", err)
	}

	logDir := filepath.Join(tmpDir, ".ralph", "logs")
	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("log directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("log path exists but is not a directory")
	}

	// Verify log file was created with correct name format
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("failed to read log directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}

	logFile := entries[0].Name()
	if !strings.HasPrefix(logFile, "run-") || !strings.HasSuffix(logFile, ".log") {
		t.Errorf("log file name %q does not match pattern run-*.log", logFile)
	}

	// Verify log file content
	content, err := os.ReadFile(filepath.Join(logDir, logFile))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "INFO ralph run started") {
		t.Errorf("log file content = %q, want to contain %q", content, "INFO ralph run started")
	}
}

func TestEnsureLogDir_InvalidPath(t *testing.T) {
	// Use a file as ProjectRoot — MkdirAll fails when path component is a file
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ProjectRoot: blockingFile, // file, not directory
		LogDir:      ".ralph/logs",
	}

	err := ensureLogDir(cfg)
	if err == nil {
		t.Fatal("ensureLogDir() with file-as-root should return error")
	}
	if !strings.Contains(err.Error(), "ralph: create log dir:") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "ralph: create log dir:")
	}
}

func TestEnsureLogDir_ReadOnlyDir(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, ".ralph", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Make directory read-only to prevent file creation
	if err := os.Chmod(logDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(logDir, 0755) // Restore for cleanup
	})

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		LogDir:      ".ralph/logs",
	}

	err := ensureLogDir(cfg)
	if err == nil {
		// Some filesystems (WSL/NTFS) may not respect Unix permissions
		t.Skip("filesystem does not enforce read-only directory permissions")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("ralph: create log file:")) {
		t.Errorf("error = %q, want to contain %q", err.Error(), "ralph: create log file:")
	}
}
