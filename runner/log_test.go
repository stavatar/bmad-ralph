package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// TestOpenRunLogger_CreatesFileAndDir verifies that OpenRunLogger creates the
// log directory and a daily log file when they do not exist.
func TestOpenRunLogger_CreatesFileAndDir(t *testing.T) {
	dir := t.TempDir()
	logDir := ".ralph/logs"

	l, err := runner.OpenRunLogger(dir, logDir)
	if err != nil {
		t.Fatalf("OpenRunLogger: unexpected error: %v", err)
	}
	defer func() {
		if closeErr := l.Close(); closeErr != nil {
			t.Errorf("Close: unexpected error: %v", closeErr)
		}
	}()

	date := time.Now().Format("2006-01-02")
	expectedPath := filepath.Join(dir, logDir, "ralph-"+date+".log")
	if _, statErr := os.Stat(expectedPath); statErr != nil {
		t.Errorf("log file not created at %s: %v", expectedPath, statErr)
	}
}

// TestOpenRunLogger_MkdirError verifies that OpenRunLogger returns a wrapped
// error when the log directory path is blocked by a file.
func TestOpenRunLogger_MkdirError(t *testing.T) {
	dir := t.TempDir()
	// Create a file where the directory should be — MkdirAll will fail.
	blockPath := filepath.Join(dir, ".ralph")
	if err := os.WriteFile(blockPath, []byte("block"), 0644); err != nil {
		t.Fatalf("setup: write block file: %v", err)
	}

	_, err := runner.OpenRunLogger(dir, ".ralph/logs")
	if err == nil {
		t.Fatal("OpenRunLogger: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: log: mkdir") {
		t.Errorf("OpenRunLogger error = %q, want prefix %q", err.Error(), "runner: log: mkdir")
	}
}

// TestRunLogger_Info_WritesFormattedLine verifies that Info writes a correctly
// formatted log line with timestamp, level, component tag, message, and key=value pairs.
func TestRunLogger_Info_WritesFormattedLine(t *testing.T) {
	dir := t.TempDir()
	l, err := runner.OpenRunLogger(dir, "logs")
	if err != nil {
		t.Fatalf("OpenRunLogger: %v", err)
	}
	defer l.Close() //nolint:errcheck

	l.Info("run started", "tasks_file", "sprint-tasks.md")

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log file: %v", readErr)
	}

	content := string(data)
	checks := []string{
		"INFO ",
		"[runner]",
		"run started",
		"tasks_file=sprint-tasks.md",
	}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("log line missing %q\ngot: %s", want, content)
		}
	}
}

// TestRunLogger_Warn_WritesFormattedLine verifies that Warn writes WARN level.
func TestRunLogger_Warn_WritesFormattedLine(t *testing.T) {
	dir := t.TempDir()
	l, err := runner.OpenRunLogger(dir, "logs")
	if err != nil {
		t.Fatalf("OpenRunLogger: %v", err)
	}
	defer l.Close() //nolint:errcheck

	l.Warn("dirty state detected", "recovering", "true")

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log file: %v", readErr)
	}

	content := string(data)
	if !strings.Contains(content, "WARN ") {
		t.Errorf("log line missing WARN level\ngot: %s", content)
	}
	if !strings.Contains(content, "dirty state detected") {
		t.Errorf("log line missing message\ngot: %s", content)
	}
	if !strings.Contains(content, "recovering=true") {
		t.Errorf("log line missing kv pair\ngot: %s", content)
	}
}

// TestRunLogger_Error_WritesFormattedLine verifies that Error writes ERROR level.
func TestRunLogger_Error_WritesFormattedLine(t *testing.T) {
	dir := t.TempDir()
	l, err := runner.OpenRunLogger(dir, "logs")
	if err != nil {
		t.Fatalf("OpenRunLogger: %v", err)
	}
	defer l.Close() //nolint:errcheck

	l.Error("execute attempts exhausted", "attempts", "3")

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log file: %v", readErr)
	}

	content := string(data)
	if !strings.Contains(content, "ERROR") {
		t.Errorf("log line missing ERROR level\ngot: %s", content)
	}
	if !strings.Contains(content, "execute attempts exhausted") {
		t.Errorf("log line missing message\ngot: %s", content)
	}
	if !strings.Contains(content, "attempts=3") {
		t.Errorf("log line missing kv pair\ngot: %s", content)
	}
}

// TestRunLogger_SpacedValue_Quoted verifies that values containing spaces are quoted.
func TestRunLogger_SpacedValue_Quoted(t *testing.T) {
	dir := t.TempDir()
	l, err := runner.OpenRunLogger(dir, "logs")
	if err != nil {
		t.Fatalf("OpenRunLogger: %v", err)
	}
	defer l.Close() //nolint:errcheck

	l.Info("task started", "task", "- [ ] Fix foo bar")

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log file: %v", readErr)
	}

	content := string(data)
	if !strings.Contains(content, `task="- [ ] Fix foo bar"`) {
		t.Errorf("spaced value not quoted\ngot: %s", content)
	}
}

// TestRunLogger_AppendMode verifies that OpenRunLogger appends to an existing file
// rather than truncating it.
func TestRunLogger_AppendMode(t *testing.T) {
	dir := t.TempDir()
	logDir := "logs"

	l1, err := runner.OpenRunLogger(dir, logDir)
	if err != nil {
		t.Fatalf("first OpenRunLogger: %v", err)
	}
	l1.Info("first entry")
	if err := l1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	l2, err := runner.OpenRunLogger(dir, logDir)
	if err != nil {
		t.Fatalf("second OpenRunLogger: %v", err)
	}
	l2.Info("second entry")
	if err := l2.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, logDir, "ralph-"+date+".log")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log file: %v", readErr)
	}

	content := string(data)
	if !strings.Contains(content, "first entry") {
		t.Errorf("append mode: first entry lost\ngot: %s", content)
	}
	if !strings.Contains(content, "second entry") {
		t.Errorf("append mode: second entry missing\ngot: %s", content)
	}
}

// TestRunLogger_TimestampFormat verifies that the timestamp matches the expected format.
func TestRunLogger_TimestampFormat(t *testing.T) {
	dir := t.TempDir()
	l, err := runner.OpenRunLogger(dir, "logs")
	if err != nil {
		t.Fatalf("OpenRunLogger: %v", err)
	}
	defer l.Close() //nolint:errcheck

	l.Info("ts check")

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read log file: %v", readErr)
	}

	// Timestamp must match: YYYY-MM-DDTHH:MM:SS
	line := strings.TrimSpace(string(data))
	if len(line) < 19 {
		t.Fatalf("log line too short: %q", line)
	}
	ts := line[:19]
	if _, parseErr := time.Parse("2006-01-02T15:04:05", ts); parseErr != nil {
		t.Errorf("timestamp %q does not match format 2006-01-02T15:04:05: %v", ts, parseErr)
	}
}

// TestNopLogger_NoOutput verifies that NopLogger does not panic and discards all output.
func TestNopLogger_NoOutput(t *testing.T) {
	l := runner.NopLogger()
	// None of these should panic.
	l.Info("run started", "tasks_file", "sprint-tasks.md")
	l.Warn("dirty state", "recovering", "true")
	l.Error("exhausted", "attempts", "3")
	if err := l.Close(); err != nil {
		t.Errorf("NopLogger.Close: unexpected error: %v", err)
	}
}



