package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/runner"
	"github.com/bmad-ralph/bmad-ralph/session"
)

// TestOpenRunLogger_CreatesFileAndDir verifies that OpenRunLogger creates the
// log directory and a daily log file when they do not exist.
func TestOpenRunLogger_CreatesFileAndDir(t *testing.T) {
	dir := t.TempDir()
	logDir := ".ralph/logs"

	l, err := runner.OpenRunLogger(dir, logDir, "")
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

	_, err := runner.OpenRunLogger(dir, ".ralph/logs", "")
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
	l, err := runner.OpenRunLogger(dir, "logs", "")
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
	l, err := runner.OpenRunLogger(dir, "logs", "")
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
	l, err := runner.OpenRunLogger(dir, "logs", "")
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
	l, err := runner.OpenRunLogger(dir, "logs", "")
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

	l1, err := runner.OpenRunLogger(dir, logDir, "")
	if err != nil {
		t.Fatalf("first OpenRunLogger: %v", err)
	}
	l1.Info("first entry")
	if err := l1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	l2, err := runner.OpenRunLogger(dir, logDir, "")
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
	l, err := runner.OpenRunLogger(dir, "logs", "")
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

// TestRunLogger_RunID_IncludedInOutput verifies that run_id appears as first kv pair
// when runID is set, and is absent when runID is empty.
func TestRunLogger_RunID_IncludedInOutput(t *testing.T) {
	tests := []struct {
		name      string
		runID     string
		wantRunID bool
	}{
		{
			name:      "with runID",
			runID:     "abc-def-123",
			wantRunID: true,
		},
		{
			name:      "empty runID",
			runID:     "",
			wantRunID: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			l, err := runner.OpenRunLogger(dir, "logs", tc.runID)
			if err != nil {
				t.Fatalf("OpenRunLogger: %v", err)
			}
			defer l.Close() //nolint:errcheck

			l.Info("test msg", "key", "val")

			date := time.Now().Format("2006-01-02")
			logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
			data, readErr := os.ReadFile(logPath)
			if readErr != nil {
				t.Fatalf("read log file: %v", readErr)
			}

			content := string(data)
			hasRunID := strings.Contains(content, "run_id=abc-def-123")

			if tc.wantRunID && !hasRunID {
				t.Errorf("log line missing run_id=abc-def-123\ngot: %s", content)
			}
			if !tc.wantRunID && strings.Contains(content, "run_id=") {
				t.Errorf("log line should not contain run_id\ngot: %s", content)
			}

			// Verify run_id appears before other kv pairs
			if tc.wantRunID {
				runIDIdx := strings.Index(content, "run_id=")
				keyIdx := strings.Index(content, "key=val")
				if runIDIdx >= keyIdx {
					t.Errorf("run_id should appear before key=val\ngot: %s", content)
				}
			}
		})
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

// TestSaveSessionLog_WritesFile verifies that SaveSessionLog creates a file with
// the expected header, stdout section, and stderr section.
func TestSaveSessionLog_WritesFile(t *testing.T) {
	dir := t.TempDir()
	sessDir := filepath.Join(dir, "sessions", "run-123")

	raw := &session.RawResult{
		Stdout:   []byte(`{"result":"ok"}`),
		Stderr:   []byte("some warnings"),
		ExitCode: 0,
	}
	info := runner.SessionLogInfo{
		SessionType: "execute",
		Seq:         0,
		ExitCode:    0,
		Elapsed:     5 * time.Second,
	}

	err := runner.SaveSessionLog(sessDir, info, raw)
	if err != nil {
		t.Fatalf("SaveSessionLog: unexpected error: %v", err)
	}

	// Find the written file
	entries, readErr := os.ReadDir(sessDir)
	if readErr != nil {
		t.Fatalf("read session dir: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	content, readErr := os.ReadFile(filepath.Join(sessDir, entries[0].Name()))
	if readErr != nil {
		t.Fatalf("read session log: %v", readErr)
	}

	checks := []struct {
		label string
		want  string
	}{
		{"header type", "SESSION execute"},
		{"header seq", "seq=0"},
		{"header exit", "exit_code=0"},
		{"header elapsed", "elapsed=5.0s"},
		{"header compactions", "compactions=0"},
		{"header max_fill", "max_fill=0.0%"},
		{"stdout section", "=== STDOUT (15 bytes) ==="},
		{"stdout content", `{"result":"ok"}`},
		{"stderr section", "=== STDERR (13 bytes) ==="},
		{"stderr content", "some warnings"},
	}
	text := string(content)
	for _, c := range checks {
		if !strings.Contains(text, c.want) {
			t.Errorf("session log missing %s: %q\ngot:\n%s", c.label, c.want, text)
		}
	}

	// Verify filename format: execute-000-YYYYMMDDTHHMMSS.log
	name := entries[0].Name()
	if !strings.HasPrefix(name, "execute-000-") {
		t.Errorf("filename prefix = %q, want execute-000-*", name)
	}
	if !strings.HasSuffix(name, ".log") {
		t.Errorf("filename suffix = %q, want *.log", name)
	}
}

func TestSaveSessionLog_ContextFields(t *testing.T) {
	dir := t.TempDir()
	sessDir := filepath.Join(dir, "sessions", "run-ctx")

	raw := &session.RawResult{
		Stdout:   []byte("output"),
		Stderr:   []byte("errors"),
		ExitCode: 0,
	}
	info := runner.SessionLogInfo{
		SessionType: "execute",
		Seq:         2,
		ExitCode:    0,
		Elapsed:     10 * time.Second,
		Compactions: 3,
		MaxFillPct:  72.5,
	}

	err := runner.SaveSessionLog(sessDir, info, raw)
	if err != nil {
		t.Fatalf("SaveSessionLog: unexpected error: %v", err)
	}

	entries, readErr := os.ReadDir(sessDir)
	if readErr != nil {
		t.Fatalf("read session dir: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	content, readErr := os.ReadFile(filepath.Join(sessDir, entries[0].Name()))
	if readErr != nil {
		t.Fatalf("read session log: %v", readErr)
	}

	text := string(content)
	checks := []struct {
		label string
		want  string
	}{
		{"compactions in header", "compactions=3"},
		{"max_fill in header", "max_fill=72.5%"},
		{"header seq", "seq=2"},
		{"header elapsed", "elapsed=10.0s"},
	}
	for _, c := range checks {
		if !strings.Contains(text, c.want) {
			t.Errorf("session log missing %s: %q\ngot:\n%s", c.label, c.want, text)
		}
	}
}

// TestSaveSessionLog_NilRaw verifies that SaveSessionLog is a no-op when raw is nil.
func TestSaveSessionLog_NilRaw(t *testing.T) {
	dir := t.TempDir()
	info := runner.SessionLogInfo{SessionType: "execute", Seq: 0}
	err := runner.SaveSessionLog(dir, info, nil)
	if err != nil {
		t.Fatalf("SaveSessionLog with nil raw: unexpected error: %v", err)
	}
}

// TestSaveSessionLog_EmptyDir verifies that SaveSessionLog is a no-op when sessDir is empty.
func TestSaveSessionLog_EmptyDir(t *testing.T) {
	raw := &session.RawResult{Stdout: []byte("x"), Stderr: []byte("y")}
	info := runner.SessionLogInfo{SessionType: "execute", Seq: 0}
	err := runner.SaveSessionLog("", info, raw)
	if err != nil {
		t.Fatalf("SaveSessionLog with empty dir: unexpected error: %v", err)
	}
}

// TestSaveSessionLog_MkdirError verifies that SaveSessionLog returns a wrapped error
// when the session directory cannot be created.
func TestSaveSessionLog_MkdirError(t *testing.T) {
	dir := t.TempDir()
	// Block directory creation with a file
	blockPath := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockPath, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	sessDir := filepath.Join(blockPath, "sub")

	raw := &session.RawResult{Stdout: []byte("x")}
	info := runner.SessionLogInfo{SessionType: "execute", Seq: 0}
	err := runner.SaveSessionLog(sessDir, info, raw)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: session log: mkdir") {
		t.Errorf("error = %q, want prefix %q", err.Error(), "runner: session log: mkdir")
	}
}

// TestSaveSessionLog_NonZeroExit verifies header with non-zero exit code.
func TestSaveSessionLog_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	raw := &session.RawResult{
		Stdout:   []byte("partial output"),
		Stderr:   []byte("error details"),
		ExitCode: 1,
	}
	info := runner.SessionLogInfo{
		SessionType: "review",
		Seq:         5,
		ExitCode:    1,
		Elapsed:     30 * time.Second,
	}

	err := runner.SaveSessionLog(dir, info, raw)
	if err != nil {
		t.Fatalf("SaveSessionLog: unexpected error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	content, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	text := string(content)

	if !strings.Contains(text, "exit_code=1") {
		t.Errorf("missing exit_code=1 in header\ngot:\n%s", text)
	}
	if !strings.Contains(text, "SESSION review") {
		t.Errorf("missing SESSION review in header\ngot:\n%s", text)
	}
	if !strings.Contains(text, "seq=5") {
		t.Errorf("missing seq=5 in header\ngot:\n%s", text)
	}
	if !strings.HasPrefix(entries[0].Name(), "review-005-") {
		t.Errorf("filename prefix = %q, want review-005-*", entries[0].Name())
	}
}

// TestRunLogger_NextSeq_Increments verifies that NextSeq returns monotonically
// increasing sequence numbers starting from 0.
func TestRunLogger_NextSeq_Increments(t *testing.T) {
	l := runner.NopLogger()
	for i := 0; i < 5; i++ {
		got := l.NextSeq()
		if got != i {
			t.Errorf("NextSeq() call %d = %d, want %d", i, got, i)
		}
	}
}

// TestRunLogger_SaveSession_WritesFile verifies that SaveSession creates a session
// log file via the RunLogger convenience method.
func TestRunLogger_SaveSession_WritesFile(t *testing.T) {
	dir := t.TempDir()
	l, err := runner.OpenRunLogger(dir, ".ralph/logs", "test-run-id")
	if err != nil {
		t.Fatalf("OpenRunLogger: %v", err)
	}
	defer l.Close() //nolint:errcheck

	raw := &session.RawResult{
		Stdout:   []byte("session output"),
		Stderr:   []byte("session stderr"),
		ExitCode: 0,
	}
	l.SaveSession("execute", raw, 0, 10*time.Second)

	sessDir := filepath.Join(dir, ".ralph/logs/sessions/test-run-id")
	entries, readErr := os.ReadDir(sessDir)
	if readErr != nil {
		t.Fatalf("read session dir: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 session log, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "execute-000-") {
		t.Errorf("first session filename = %q, want execute-000-*", entries[0].Name())
	}

	// Second call should increment seq
	l.SaveSession("review", raw, 0, 5*time.Second)
	entries, _ = os.ReadDir(sessDir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 session logs, got %d", len(entries))
	}
}

// TestRunLogger_SaveSession_NopLogger verifies that SaveSession on NopLogger is a no-op
// (no panic, no files created).
func TestRunLogger_SaveSession_NopLogger(t *testing.T) {
	l := runner.NopLogger()
	raw := &session.RawResult{Stdout: []byte("x"), Stderr: []byte("y")}
	// Should not panic — NopLogger has empty sessDir so SaveSession returns early.
	l.SaveSession("execute", raw, 0, time.Second)
}
