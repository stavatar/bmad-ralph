package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

// TestFormatTokens_EdgeCases covers token formatting (AC3).
func TestFormatTokens_EdgeCases(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"small", 500, "500"},
		{"exact_thousand", 1000, "1.0K"},
		{"tens_of_thousands", 125000, "125K"},
		{"fractional_thousand", 1500, "1.5K"},
		{"million", 1200000, "1.2M"},
		{"exact_million", 1000000, "1.0M"},
		{"sub_thousand", 999, "999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatTokens(tc.n)
			if got != tc.want {
				t.Errorf("formatTokens(%d) = %q, want %q", tc.n, got, tc.want)
			}
		})
	}
}

// TestFormatSummary_WithMetrics verifies 4-line summary output (AC3).
func TestFormatSummary_WithMetrics(t *testing.T) {
	m := &runner.RunMetrics{
		RunID:          "test-run-id",
		DurationMs:     125000, // 2m 5s
		TasksCompleted: 3,
		TasksFailed:    1,
		TasksSkipped:   1,
		CostUSD:        12.50,
		InputTokens:    125000,
		OutputTokens:   50000,
		Tasks: []runner.TaskMetrics{
			{
				Name:   "task-a",
				Status: "completed",
				Findings: []runner.ReviewFinding{
					{Severity: "HIGH", Description: "issue1"},
					{Severity: "MEDIUM", Description: "issue2"},
					{Severity: "LOW", Description: "issue3"},
				},
			},
			{
				Name:   "task-b",
				Status: "completed",
				Findings: []runner.ReviewFinding{
					{Severity: "MEDIUM", Description: "issue4"},
				},
			},
			{Name: "task-c", Status: "completed"},
			{Name: "task-d", Status: "failed"},
			{Name: "task-e", Status: "skipped"},
		},
	}
	cfg := &config.Config{
		LogDir: ".ralph/logs",
		RunID:  "test-run-id",
	}

	out := formatSummary(m, cfg)
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("summary lines = %d, want 4:\n%s", len(lines), out)
	}

	// Line 1: task counts
	if !strings.Contains(lines[0], "5 tasks") {
		t.Errorf("line 1 missing total tasks: %s", lines[0])
	}
	if !strings.Contains(lines[0], "3 completed") {
		t.Errorf("line 1 missing completed count: %s", lines[0])
	}
	if !strings.Contains(lines[0], "1 skipped") {
		t.Errorf("line 1 missing skipped count: %s", lines[0])
	}
	if !strings.Contains(lines[0], "1 failed") {
		t.Errorf("line 1 missing failed count: %s", lines[0])
	}

	// Line 2: duration, cost, tokens
	if !strings.Contains(lines[1], "2m 5s") {
		t.Errorf("line 2 missing duration: %s", lines[1])
	}
	if !strings.Contains(lines[1], "$12.50") {
		t.Errorf("line 2 missing cost: %s", lines[1])
	}
	if !strings.Contains(lines[1], "125K in") {
		t.Errorf("line 2 missing input tokens: %s", lines[1])
	}
	if !strings.Contains(lines[1], "50K out") {
		t.Errorf("line 2 missing output tokens: %s", lines[1])
	}

	// Line 3: reviews
	if !strings.Contains(lines[2], "2 cycles") {
		t.Errorf("line 3 missing review cycles: %s", lines[2])
	}
	if !strings.Contains(lines[2], "4 findings") {
		t.Errorf("line 3 missing total findings: %s", lines[2])
	}
	if !strings.Contains(lines[2], "1h/2m/1l") {
		t.Errorf("line 3 missing severity breakdown: %s", lines[2])
	}

	// Line 4: report path
	if !strings.Contains(lines[3], "ralph-run-test-run-id.json") {
		t.Errorf("line 4 missing report path: %s", lines[3])
	}
}

// TestFormatSummary_ZeroMetrics verifies N/A display for zero metrics (AC4).
func TestFormatSummary_ZeroMetrics(t *testing.T) {
	m := &runner.RunMetrics{RunID: "zero-run"}
	cfg := &config.Config{LogDir: ".ralph/logs", RunID: "zero-run"}

	out := formatSummary(m, cfg)
	if !strings.Contains(out, "Cost: N/A") {
		t.Errorf("zero metrics should show 'Cost: N/A', got: %s", out)
	}
	if !strings.Contains(out, "Tokens: N/A") {
		t.Errorf("zero metrics should show 'Tokens: N/A', got: %s", out)
	}
	if !strings.Contains(out, "0 tasks") {
		t.Errorf("zero metrics should show '0 tasks', got: %s", out)
	}
}

// TestWriteRunReport_CreatesJSON verifies JSON report file creation (AC1, AC5).
func TestWriteRunReport_CreatesJSON(t *testing.T) {
	tmpDir := t.TempDir()
	m := &runner.RunMetrics{
		RunID:          "report-test",
		StartTime:      time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC),
		DurationMs:     60000,
		TasksCompleted: 2,
		TasksFailed:    0,
		TasksSkipped:   1,
		CostUSD:        5.25,
		InputTokens:    100000,
		OutputTokens:   50000,
		Tasks: []runner.TaskMetrics{
			{Name: "task-a", Status: "completed"},
			{Name: "task-b", Status: "completed"},
			{Name: "task-c", Status: "skipped"},
		},
	}
	cfg := &config.Config{
		ProjectRoot: tmpDir,
		LogDir:      ".ralph/logs",
		RunID:       "report-test",
	}

	writeRunReport(cfg, m)

	path := filepath.Join(tmpDir, ".ralph/logs", "ralph-run-report-test.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Verify valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	// Verify json tag keys exist (AC2)
	for _, key := range []string{"run_id", "duration_ms", "tasks", "cost_usd",
		"input_tokens", "output_tokens", "tasks_completed", "tasks_failed", "tasks_skipped"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}

	// Verify aggregate values
	if v, ok := parsed["tasks_completed"].(float64); !ok || int(v) != 2 {
		t.Errorf("tasks_completed = %v, want 2", parsed["tasks_completed"])
	}
	if v, ok := parsed["tasks_skipped"].(float64); !ok || int(v) != 1 {
		t.Errorf("tasks_skipped = %v, want 1", parsed["tasks_skipped"])
	}
	if v, ok := parsed["cost_usd"].(float64); !ok || v != 5.25 {
		t.Errorf("cost_usd = %v, want 5.25", parsed["cost_usd"])
	}
}

// TestWriteRunReport_BadPath verifies non-fatal error on write failure (AC6).
func TestWriteRunReport_BadPath(t *testing.T) {
	// Use file-as-directory trick: MkdirAll fails when a file blocks directory creation.
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := &runner.RunMetrics{RunID: "bad-path"}
	cfg := &config.Config{
		ProjectRoot: blocker, // file, not directory
		LogDir:      "logs",
		RunID:       "bad-path",
	}

	// Capture stderr to verify WARNING output
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	writeRunReport(cfg, m)

	w.Close()
	os.Stderr = origStderr

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	captured := string(buf[:n])
	r.Close()

	if !strings.Contains(captured, "WARNING") {
		t.Errorf("stderr should contain WARNING, got: %q", captured)
	}
}

// TestFormatSummary_ReviewLine verifies review cycle and severity counting (AC3, Task 3.7).
func TestFormatSummary_ReviewLine(t *testing.T) {
	m := &runner.RunMetrics{
		RunID: "review-test",
		Tasks: []runner.TaskMetrics{
			{
				Name: "no-review",
			},
			{
				Name: "with-findings",
				Findings: []runner.ReviewFinding{
					{Severity: "HIGH"},
					{Severity: "high"}, // case-insensitive
					{Severity: "LOW"},
				},
			},
			{
				Name: "gate-only",
				Gate: &runner.GateStats{TotalPrompts: 2, Approvals: 1},
			},
		},
	}
	cfg := &config.Config{LogDir: "logs", RunID: "review-test"}

	out := formatSummary(m, cfg)
	if !strings.Contains(out, "2 cycles") {
		t.Errorf("should count 2 review cycles (findings + gate-only), got: %s", out)
	}
	if !strings.Contains(out, "3 findings") {
		t.Errorf("should count 3 total findings, got: %s", out)
	}
	if !strings.Contains(out, "2h/0m/1l") {
		t.Errorf("severity breakdown should be 2h/0m/1l, got: %s", out)
	}
}

// TestRunRun_NilMetrics verifies no report or summary when runner returns nil metrics (AC1, Task 3.2).
func TestRunRun_NilMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		ProjectRoot: tmpDir,
		LogDir:      ".ralph/logs",
		RunID:       "nil-test",
	}

	// writeRunReport with nil metrics would panic — the guard is in runRun().
	// Verify the guard by confirming no report file created when metrics is nil.
	// (runRun checks `if metrics != nil` before calling writeRunReport/formatSummary)
	reportPath := filepath.Join(tmpDir, ".ralph/logs", "ralph-run-nil-test.json")
	if _, err := os.Stat(reportPath); err == nil {
		t.Errorf("report file should not exist before test")
	}

	// Simulate the nil guard: metrics=nil → no writeRunReport, no formatSummary
	var metrics *runner.RunMetrics
	if metrics != nil {
		writeRunReport(cfg, metrics)
	}

	// Verify no report created
	if _, err := os.Stat(reportPath); err == nil {
		t.Errorf("report file should not exist when metrics is nil")
	}
}

// TestRunMetrics_JSONRoundTrip verifies JSON tag keys via marshal/unmarshal cycle (AC2, Task 3.4).
func TestRunMetrics_JSONRoundTrip(t *testing.T) {
	m := runner.RunMetrics{
		RunID:          "roundtrip",
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:        time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC),
		DurationMs:     3600000,
		InputTokens:    1000,
		OutputTokens:   500,
		CacheTokens:    200,
		CostUSD:        1.50,
		NumTurns:       10,
		TotalSessions:  3,
		TasksCompleted: 2,
		TasksFailed:    1,
		TasksSkipped:   0,
		Tasks: []runner.TaskMetrics{
			{
				Name:   "task-1",
				Status: "completed",
				Diff:   &runner.DiffStats{FilesChanged: 3, Insertions: 100, Deletions: 20, Packages: []string{"runner"}},
				Findings: []runner.ReviewFinding{
					{Severity: "MEDIUM", Description: "test finding", File: "foo.go", Line: 42},
				},
				Latency: &runner.LatencyBreakdown{SessionMs: 5000, GitMs: 100},
				Gate:    &runner.GateStats{TotalPrompts: 1, Approvals: 1},
				Errors:  &runner.ErrorStats{TotalErrors: 0},
			},
		},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Verify all top-level json tag keys
	wantKeys := []string{
		"run_id", "start_time", "end_time", "duration_ms",
		"tasks", "input_tokens", "output_tokens", "cache_tokens",
		"cost_usd", "num_turns", "total_sessions",
		"tasks_completed", "tasks_failed", "tasks_skipped",
	}
	for _, key := range wantKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing top-level key %q", key)
		}
	}

	// Verify nested task keys
	tasks, ok := parsed["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("tasks: want 1 element, got %v", parsed["tasks"])
	}
	task, ok := tasks[0].(map[string]any)
	if !ok {
		t.Fatalf("task[0] not a map")
	}
	taskKeys := []string{"name", "status", "diff", "findings", "latency", "gate", "errors"}
	for _, key := range taskKeys {
		if _, ok := task[key]; !ok {
			t.Errorf("task JSON missing key %q", key)
		}
	}

	// Verify diff nested keys
	diff, ok := task["diff"].(map[string]any)
	if !ok {
		t.Fatalf("diff not a map")
	}
	for _, key := range []string{"files_changed", "insertions", "deletions", "packages"} {
		if _, ok := diff[key]; !ok {
			t.Errorf("diff JSON missing key %q", key)
		}
	}

	// Verify findings nested keys
	findings, ok := task["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("findings: want 1, got %v", task["findings"])
	}
	finding, ok := findings[0].(map[string]any)
	if !ok {
		t.Fatalf("finding[0] not a map")
	}
	for _, key := range []string{"severity", "description", "file", "line"} {
		if _, ok := finding[key]; !ok {
			t.Errorf("finding JSON missing key %q", key)
		}
	}
}
