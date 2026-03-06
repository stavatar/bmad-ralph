package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmad-ralph/bmad-ralph/session"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	// LogLevelInfo is used for normal operational events.
	LogLevelInfo LogLevel = "INFO "
	// LogLevelWarn is used for recoverable conditions that need attention.
	LogLevelWarn LogLevel = "WARN "
	// LogLevelError is used for unrecoverable errors and exhaustion conditions.
	LogLevelError LogLevel = "ERROR"
)

// RunLogger writes structured log entries to a daily log file and to stderr.
// It also saves per-session raw output (stdout/stderr) to individual log files
// under <logDir>/sessions/<runID>/ for post-mortem debugging.
// Format: "2006-01-02T15:04:05 LEVEL [runner] message key=value key=value"
// Thread safety: not goroutine-safe — ralph is single-threaded in Execute.
type RunLogger struct {
	file    io.WriteCloser
	stderr  io.Writer
	runID   string
	sessDir string // directory for session log files; empty = disabled
	sessSeq int    // monotonic session sequence counter
}

// noopWriteCloser is a no-op writer for the NopLogger.
type noopWriteCloser struct{}

func (noopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (noopWriteCloser) Close() error                { return nil }

// NopLogger returns a RunLogger that discards all output.
// Used in tests that do not exercise logging.
func NopLogger() *RunLogger {
	return &RunLogger{
		file:   noopWriteCloser{},
		stderr: io.Discard,
	}
}

// OpenRunLogger creates a RunLogger that writes to:
//   - <projectRoot>/<logDir>/ralph-YYYY-MM-DD.log (appended)
//   - os.Stderr (duplicated for live terminal visibility)
//
// Session logs are saved to <projectRoot>/<logDir>/sessions/<runID>/.
// The log directory is created if it does not exist.
// Returns an error if the directory cannot be created or the file cannot be opened.
func OpenRunLogger(projectRoot, logDir, runID string) (*RunLogger, error) {
	dir := filepath.Join(projectRoot, logDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("runner: log: mkdir: %w", err)
	}

	date := time.Now().Format("2006-01-02")
	logPath := filepath.Join(dir, fmt.Sprintf("ralph-%s.log", date))

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("runner: log: open: %w", err)
	}

	sessDir := filepath.Join(dir, "sessions", runID)

	return &RunLogger{
		file:    f,
		stderr:  os.Stderr,
		runID:   runID,
		sessDir: sessDir,
	}, nil
}

// Close flushes and closes the underlying log file.
// After Close, the logger should not be used.
func (l *RunLogger) Close() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("runner: log: close: %w", err)
	}
	return nil
}

// Info logs an INFO-level message with optional key=value pairs.
func (l *RunLogger) Info(msg string, kvs ...string) {
	l.write(LogLevelInfo, msg, kvs)
}

// Warn logs a WARN-level message with optional key=value pairs.
func (l *RunLogger) Warn(msg string, kvs ...string) {
	l.write(LogLevelWarn, msg, kvs)
}

// Error logs an ERROR-level message with optional key=value pairs.
func (l *RunLogger) Error(msg string, kvs ...string) {
	l.write(LogLevelError, msg, kvs)
}

// write formats and emits a log line to both file and stderr.
// kvs must be an even-length slice of alternating key, value strings.
// Odd-length kvs are silently truncated to the nearest pair.
func (l *RunLogger) write(level LogLevel, msg string, kvs []string) {
	ts := time.Now().Format("2006-01-02T15:04:05")

	var sb strings.Builder
	sb.WriteString(ts)
	sb.WriteByte(' ')
	sb.WriteString(string(level))
	sb.WriteString(" [runner] ")
	sb.WriteString(msg)

	// Prepend run_id as first kv pair if set
	if l.runID != "" {
		sb.WriteString(" run_id=")
		sb.WriteString(l.runID)
	}

	for i := 0; i+1 < len(kvs); i += 2 {
		sb.WriteByte(' ')
		sb.WriteString(kvs[i])
		sb.WriteByte('=')
		// Quote values that contain spaces or are empty.
		val := kvs[i+1]
		if val == "" || strings.ContainsAny(val, " \t\n") {
			sb.WriteByte('"')
			sb.WriteString(val)
			sb.WriteByte('"')
		} else {
			sb.WriteString(val)
		}
	}

	sb.WriteByte('\n')
	line := sb.String()

	_, _ = fmt.Fprint(l.file, line)
	_, _ = fmt.Fprint(l.stderr, line)
}

// kv formats a key-value pair for use with RunLogger methods.
func kv(key, value string) []string {
	return []string{key, value}
}

// kvs merges multiple key-value pairs into a single slice for RunLogger methods.
func kvs(pairs ...[]string) []string {
	out := make([]string, 0, len(pairs)*2)
	for _, p := range pairs {
		out = append(out, p...)
	}
	return out
}

// SessionLogInfo holds metadata for a session log file header.
type SessionLogInfo struct {
	SessionType string        // e.g. "execute", "review", "once", "once-review", "resume", "sync"
	Seq         int           // monotonic sequence number within the run
	ExitCode    int           // process exit code
	Elapsed     time.Duration // wall-clock session duration
	Compactions int           // compaction events during session (Story 10.7 FR92)
	MaxFillPct  float64       // estimated max context fill percentage (Story 10.7 FR92)
}

// SaveSessionLog writes a session log file to sessDir with the given metadata and raw output.
// File format: <type>-<seq>-<timestamp>.log containing a header line, stdout section, and stderr section.
// Returns an error if the directory cannot be created or the file cannot be written.
func SaveSessionLog(sessDir string, info SessionLogInfo, raw *session.RawResult) error {
	if sessDir == "" || raw == nil {
		return nil
	}

	if err := os.MkdirAll(sessDir, 0755); err != nil {
		return fmt.Errorf("runner: session log: mkdir: %w", err)
	}

	ts := time.Now().Format("20060102T150405")
	filename := fmt.Sprintf("%s-%03d-%s.log", info.SessionType, info.Seq, ts)
	logPath := filepath.Join(sessDir, filename)

	var sb strings.Builder
	fmt.Fprintf(&sb, "=== SESSION %s seq=%d exit_code=%d elapsed=%.1fs compactions=%d max_fill=%.1f%% ===\n",
		info.SessionType, info.Seq, info.ExitCode, info.Elapsed.Seconds(), info.Compactions, info.MaxFillPct)
	fmt.Fprintf(&sb, "=== STDOUT (%d bytes) ===\n", len(raw.Stdout))
	sb.Write(raw.Stdout)
	if len(raw.Stdout) > 0 && raw.Stdout[len(raw.Stdout)-1] != '\n' {
		sb.WriteByte('\n')
	}
	fmt.Fprintf(&sb, "=== STDERR (%d bytes) ===\n", len(raw.Stderr))
	sb.Write(raw.Stderr)
	if len(raw.Stderr) > 0 && raw.Stderr[len(raw.Stderr)-1] != '\n' {
		sb.WriteByte('\n')
	}

	if err := os.WriteFile(logPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("runner: session log: write: %w", err)
	}
	return nil
}

// NextSeq returns the current session sequence number and increments it.
func (l *RunLogger) NextSeq() int {
	seq := l.sessSeq
	l.sessSeq++
	return seq
}

// SaveSession saves a session log file. Write failures are non-fatal: logged as warnings.
func (l *RunLogger) SaveSession(sessionType string, raw *session.RawResult, exitCode int, elapsed time.Duration) {
	if l.sessDir == "" {
		return
	}
	seq := l.NextSeq()
	info := SessionLogInfo{
		SessionType: sessionType,
		Seq:         seq,
		ExitCode:    exitCode,
		Elapsed:     elapsed,
	}
	if err := SaveSessionLog(l.sessDir, info, raw); err != nil {
		l.Warn("session log save failed", "error", err.Error(), "type", sessionType)
	}
}
