package testutil_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/session"
)

func TestMain(m *testing.M) {
	if testutil.RunMockClaude() {
		return // acted as mock Claude subprocess — dead code after os.Exit
	}
	os.Exit(m.Run())
}

func TestRunMockClaude_SequentialResponses(t *testing.T) {
	scenario := testutil.Scenario{
		Name: "sequential",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "seq-exec-001", CreatesCommit: true},
			{Type: "review", ExitCode: 0, SessionID: "seq-review-001"},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	tests := []struct {
		name      string
		wantSID   string
		wantOut   string
		stepIndex int
	}{
		{
			name:      "first step execute",
			wantSID:   "seq-exec-001",
			wantOut:   "Mock output for step 0",
			stepIndex: 0,
		},
		{
			name:      "second step review",
			wantSID:   "seq-review-001",
			wantOut:   "Mock output for step 1",
			stepIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			raw, err := session.Execute(context.Background(), session.Options{
				Command:    os.Args[0],
				Dir:        dir,
				OutputJSON: true,
			})
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("Execute() step %d unexpected error: %v", tt.stepIndex, err)
			}

			result, parseErr := session.ParseResult(raw, elapsed)
			if parseErr != nil {
				t.Fatalf("ParseResult() step %d unexpected error: %v", tt.stepIndex, parseErr)
			}
			if result.SessionID != tt.wantSID {
				t.Errorf("step %d SessionID = %q, want %q", tt.stepIndex, result.SessionID, tt.wantSID)
			}
			if result.Output != tt.wantOut {
				t.Errorf("step %d Output = %q, want %q", tt.stepIndex, result.Output, tt.wantOut)
			}
			if result.ExitCode != 0 {
				t.Errorf("step %d ExitCode = %d, want 0", tt.stepIndex, result.ExitCode)
			}
		})
	}

	// Verify counter file is at 2 after both steps
	counterData, err := os.ReadFile(filepath.Join(stateDir, "counter"))
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if string(counterData) != "2" {
		t.Errorf("counter = %q, want %q", string(counterData), "2")
	}
}

func TestRunMockClaude_BeyondScenarioSteps(t *testing.T) {
	scenario := testutil.Scenario{
		Name: "single step",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "single-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	// First call should succeed
	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("Execute() first call unexpected error: %v", err)
	}
	if len(raw.Stdout) == 0 {
		t.Fatal("Execute() first call returned empty stdout")
	}

	// Second call should fail — beyond scenario
	raw2, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() second call expected error for beyond scenario, got nil")
	}
	// Verify "beyond scenario" appears in stderr (subprocess wrote it there)
	combined := string(raw2.Stderr) + err.Error()
	if !strings.Contains(combined, "beyond scenario") {
		t.Errorf("beyond-scenario message not found in stderr+error, got: %q", combined)
	}
}

func TestRunMockClaude_CustomOutputFile(t *testing.T) {
	// Create a custom output file
	tmpDir := t.TempDir()
	customOutput := "This is custom output from a file."
	outputPath := filepath.Join(tmpDir, "custom_output.txt")
	if err := os.WriteFile(outputPath, []byte(customOutput), 0644); err != nil {
		t.Fatalf("write custom output: %v", err)
	}

	// Scenario JSON must be in the same dir as the output file (output_file is relative to scenario dir)
	scenario := testutil.Scenario{
		Name: "custom output",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "custom-001", OutputFile: "custom_output.txt"},
		},
	}
	scenarioData, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal scenario: %v", err)
	}
	scenarioPath := filepath.Join(tmpDir, "scenario.json")
	if err := os.WriteFile(scenarioPath, scenarioData, 0644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}

	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	t.Setenv("MOCK_CLAUDE_SCENARIO", scenarioPath)
	t.Setenv("MOCK_CLAUDE_STATE_DIR", stateDir)

	dir := t.TempDir()
	start := time.Now()
	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	result, parseErr := session.ParseResult(raw, elapsed)
	if parseErr != nil {
		t.Fatalf("ParseResult() unexpected error: %v", parseErr)
	}
	if result.Output != customOutput {
		t.Errorf("Output = %q, want %q", result.Output, customOutput)
	}
	if result.SessionID != "custom-001" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "custom-001")
	}
}

func TestRunMockClaude_ArgsLogging(t *testing.T) {
	scenario := testutil.Scenario{
		Name: "args logging",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "args-001"},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	_, err := session.Execute(context.Background(), session.Options{
		Command:    os.Args[0],
		Dir:        dir,
		Prompt:     "test prompt text",
		MaxTurns:   5,
		OutputJSON: true,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	args := testutil.ReadInvocationArgs(t, stateDir, 0)

	// Verify expected flags are present
	wantFlags := []string{"-p", "test prompt text", "--max-turns", "5", "--output-format", "json"}
	for _, want := range wantFlags {
		found := false
		for _, got := range args {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args missing %q, got: %v", want, args)
		}
	}
}

func TestRunMockClaude_NonZeroExitCode(t *testing.T) {
	scenario := testutil.Scenario{
		Name: "non zero exit",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 2, SessionID: "exit2-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for non-zero exit, got nil")
	}
	if raw.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", raw.ExitCode)
	}
	if !strings.Contains(err.Error(), "session: claude: exit 2:") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "session: claude: exit 2:")
	}
}

func TestRunMockClaude_MissingScenarioFile(t *testing.T) {
	t.Setenv("MOCK_CLAUDE_SCENARIO", "/nonexistent/path/scenario.json")
	t.Setenv("MOCK_CLAUDE_STATE_DIR", t.TempDir())
	dir := t.TempDir()

	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for missing scenario file, got nil")
	}
	// Verify stderr contains descriptive error about reading scenario
	if !strings.Contains(string(raw.Stderr), "read scenario") {
		t.Errorf("stderr = %q, want to contain %q", string(raw.Stderr), "read scenario")
	}
}

func TestRunMockClaude_MissingStateDir(t *testing.T) {
	// Create a valid scenario file
	tmpDir := t.TempDir()
	scenario := testutil.Scenario{
		Name:  "valid",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "x"}},
	}
	data, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	scenarioPath := filepath.Join(tmpDir, "scenario.json")
	if err := os.WriteFile(scenarioPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("MOCK_CLAUDE_SCENARIO", scenarioPath)
	// Intentionally NOT setting MOCK_CLAUDE_STATE_DIR
	t.Setenv("MOCK_CLAUDE_STATE_DIR", "")
	dir := t.TempDir()

	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for missing state dir, got nil")
	}
	// Verify stderr mentions MOCK_CLAUDE_STATE_DIR
	if !strings.Contains(string(raw.Stderr), "MOCK_CLAUDE_STATE_DIR") {
		t.Errorf("stderr = %q, want to contain %q", string(raw.Stderr), "MOCK_CLAUDE_STATE_DIR")
	}
}

func TestRunMockClaude_CorruptedCounter(t *testing.T) {
	scenario := testutil.Scenario{
		Name: "corrupted counter",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "corrupt-001"},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)

	// Write non-integer content to counter file
	counterPath := filepath.Join(stateDir, "counter")
	if err := os.WriteFile(counterPath, []byte("not-a-number"), 0644); err != nil {
		t.Fatalf("write corrupted counter: %v", err)
	}

	dir := t.TempDir()
	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for corrupted counter, got nil")
	}
	if !strings.Contains(string(raw.Stderr), "parse counter") {
		t.Errorf("stderr = %q, want to contain %q", string(raw.Stderr), "parse counter")
	}
}

func TestRunMockClaude_EmptyScenario(t *testing.T) {
	scenario := testutil.Scenario{
		Name:  "empty",
		Steps: nil,
	}
	testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for empty scenario, got nil")
	}
	// Combine stderr + error for assertion (stderr comes from subprocess, error from exec)
	combined := string(raw.Stderr) + err.Error()
	if !strings.Contains(combined, "beyond scenario") {
		t.Errorf("combined output = %q, want to contain %q", combined, "beyond scenario")
	}
	if !strings.Contains(combined, "has 0 steps") {
		t.Errorf("combined output = %q, want to contain %q", combined, "has 0 steps")
	}
}

func TestRunMockClaude_WriteFiles(t *testing.T) {
	projectDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "write files",
		Steps: []testutil.ScenarioStep{
			{
				Type:      "review",
				ExitCode:  0,
				SessionID: "write-001",
				WriteFiles: map[string]string{
					"sprint-tasks.md":    "- [x] Task one\n",
					"review-findings.md": "## [HIGH] Bug found\n",
				},
				DeleteFiles: []string{"nonexistent.md"}, // idempotent delete
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", projectDir)

	dir := t.TempDir()
	_, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Verify files were written
	tasksData, err := os.ReadFile(filepath.Join(projectDir, "sprint-tasks.md"))
	if err != nil {
		t.Fatalf("read sprint-tasks.md: %v", err)
	}
	if string(tasksData) != "- [x] Task one\n" {
		t.Errorf("sprint-tasks.md = %q, want %q", string(tasksData), "- [x] Task one\n")
	}

	findingsData, err := os.ReadFile(filepath.Join(projectDir, "review-findings.md"))
	if err != nil {
		t.Fatalf("read review-findings.md: %v", err)
	}
	if string(findingsData) != "## [HIGH] Bug found\n" {
		t.Errorf("review-findings.md = %q, want %q", string(findingsData), "## [HIGH] Bug found\n")
	}
}

func TestScenarioStep_ZeroValue(t *testing.T) {
	var step testutil.ScenarioStep
	if step.Type != "" {
		t.Errorf("zero Type = %q, want empty", step.Type)
	}
	if step.ExitCode != 0 {
		t.Errorf("zero ExitCode = %d, want 0", step.ExitCode)
	}
	if step.SessionID != "" {
		t.Errorf("zero SessionID = %q, want empty", step.SessionID)
	}
	if step.OutputFile != "" {
		t.Errorf("zero OutputFile = %q, want empty", step.OutputFile)
	}
	if step.CreatesCommit {
		t.Error("zero CreatesCommit = true, want false")
	}
	if step.IsError {
		t.Error("zero IsError = true, want false")
	}
	if step.WriteFiles != nil {
		t.Errorf("zero WriteFiles = %v, want nil", step.WriteFiles)
	}
	if len(step.DeleteFiles) != 0 {
		t.Errorf("zero DeleteFiles len = %d, want 0", len(step.DeleteFiles))
	}
}

func TestScenario_ZeroValue(t *testing.T) {
	var s testutil.Scenario
	if s.Name != "" {
		t.Errorf("zero Scenario.Name = %q, want empty", s.Name)
	}
	if s.Steps != nil {
		t.Errorf("zero Scenario.Steps = %v, want nil", s.Steps)
	}
}

func TestRunMockClaude_IsErrorStep(t *testing.T) {
	// IsError: true → resultSubtype = "error" in JSON output → lines 140-142 covered.
	scenario := testutil.Scenario{
		Name: "is-error-step",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "err-step-001", IsError: true},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	start := time.Now()
	raw, err := session.Execute(context.Background(), session.Options{
		Command:    os.Args[0],
		Dir:        dir,
		OutputJSON: true,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	result, parseErr := session.ParseResult(raw, elapsed)
	if parseErr != nil {
		t.Fatalf("ParseResult() unexpected error: %v", parseErr)
	}
	if result == nil {
		t.Fatal("ParseResult() returned nil result")
	}
	if result.SessionID != "err-step-001" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "err-step-001")
	}
}

func TestRunMockClaude_CorruptScenarioJSON(t *testing.T) {
	// Scenario file is overwritten with invalid JSON after env vars are set.
	// Subprocess: json.Unmarshal fails → os.Exit(1) → lines 83-86 covered.
	scenario := testutil.Scenario{
		Name: "placeholder",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "corrupt-001"},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario)
	// Overwrite with corrupt JSON so subprocess parse fails
	if err := os.WriteFile(scenarioPath, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	raw, execErr := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if raw == nil {
		t.Fatal("Execute() returned nil raw result")
	}
	if raw.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 (subprocess exits non-zero on corrupt JSON)", raw.ExitCode)
	}
	// execErr is non-nil (ExitError for exit code 1) — expected, not a test failure
	_ = execErr
}

func TestRunMockClaude_MissingOutputFile(t *testing.T) {
	// Step.OutputFile references a non-existent file in the scenario dir →
	// os.ReadFile fails → os.Exit(1) → lines 130-133 covered.
	scenario := testutil.Scenario{
		Name: "missing-output-file",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "missing-001", OutputFile: "nonexistent.txt"},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	raw, execErr := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if raw == nil {
		t.Fatal("Execute() returned nil raw result")
	}
	if raw.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 (subprocess exits non-zero on missing output file)", raw.ExitCode)
	}
	// execErr is non-nil (ExitError for exit code 1) — expected, not a test failure
	_ = execErr
}

// --- Coverage: RunMockClaude error paths (mock_claude.go lines 92-95, 119-122, 175-178, 181-184) ---

// TestRunMockClaude_CounterReadError verifies that RunMockClaude writes to stderr and
// exits 1 when the counter file exists but is unreadable (non-ErrNotExist).
// Covers lines 92-95: err != nil && !errors.Is(err, os.ErrNotExist) branch.
func TestRunMockClaude_CounterReadError(t *testing.T) {
	scenario := testutil.Scenario{
		Name:  "counter-read-error",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "x"}},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	// Make counter a non-empty directory → os.ReadFile returns EISDIR (not ErrNotExist)
	counterDir := filepath.Join(stateDir, "counter")
	if err := os.MkdirAll(filepath.Join(counterDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir counter dir: %v", err)
	}

	raw, execErr := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if execErr == nil {
		t.Fatal("Execute() expected error when counter is a directory, got nil")
	}
	combined := string(raw.Stderr) + execErr.Error()
	if !strings.Contains(combined, "read counter") {
		t.Errorf("expected 'read counter' in stderr+error, got: %q", combined)
	}
}

// TestRunMockClaude_InvocationWriteError verifies that RunMockClaude writes to stderr
// and exits 1 when the invocation log file cannot be written (non-ErrNotExist).
// Covers lines 119-122: os.WriteFile(invocationPath) error branch.
func TestRunMockClaude_InvocationWriteError(t *testing.T) {
	scenario := testutil.Scenario{
		Name:  "invocation-write-error",
		Steps: []testutil.ScenarioStep{{Type: "execute", ExitCode: 0, SessionID: "x"}},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	// Make invocation_0.json a non-empty directory → WriteFile returns EISDIR
	invDir := filepath.Join(stateDir, "invocation_0.json")
	if err := os.MkdirAll(filepath.Join(invDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir invocation dir: %v", err)
	}

	raw, execErr := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if execErr == nil {
		t.Fatal("Execute() expected error when invocation path is a directory, got nil")
	}
	combined := string(raw.Stderr) + execErr.Error()
	if !strings.Contains(combined, "write invocation log") {
		t.Errorf("expected 'write invocation log' in stderr+error, got: %q", combined)
	}
}

// TestRunMockClaude_WriteFileSideEffectError verifies that RunMockClaude writes to
// stderr and exits 1 when a WriteFiles side-effect target is a non-empty directory.
// Covers lines 175-178: os.WriteFile(filepath.Join(projectRoot, relPath)) error branch.
func TestRunMockClaude_WriteFileSideEffectError(t *testing.T) {
	projectRoot := t.TempDir()

	// Make the target path a non-empty directory → WriteFile returns EISDIR
	targetDir := filepath.Join(projectRoot, "output.txt")
	if err := os.MkdirAll(filepath.Join(targetDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}

	scenario := testutil.Scenario{
		Name: "write-side-effect-error",
		Steps: []testutil.ScenarioStep{{
			Type:       "execute",
			ExitCode:   0,
			SessionID:  "x",
			WriteFiles: map[string]string{"output.txt": "content"},
		}},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", projectRoot)
	dir := t.TempDir()

	raw, execErr := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if execErr == nil {
		t.Fatal("Execute() expected error when WriteFiles target is a directory, got nil")
	}
	combined := string(raw.Stderr) + execErr.Error()
	if !strings.Contains(combined, "write file") {
		t.Errorf("expected 'write file' in stderr+error, got: %q", combined)
	}
}

// TestRunMockClaude_DeleteFileSideEffectError verifies that RunMockClaude writes to
// stderr and exits 1 when a DeleteFiles target is a non-empty directory (ENOTEMPTY,
// not ErrNotExist). Covers lines 181-184: os.Remove error != ErrNotExist branch.
func TestRunMockClaude_DeleteFileSideEffectError(t *testing.T) {
	projectRoot := t.TempDir()

	// Make the target a non-empty directory → os.Remove returns ENOTEMPTY (not ErrNotExist)
	targetDir := filepath.Join(projectRoot, "to-delete")
	if err := os.MkdirAll(filepath.Join(targetDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}

	scenario := testutil.Scenario{
		Name: "delete-side-effect-error",
		Steps: []testutil.ScenarioStep{{
			Type:        "execute",
			ExitCode:    0,
			SessionID:   "x",
			DeleteFiles: []string{"to-delete"},
		}},
	}
	testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", projectRoot)
	dir := t.TempDir()

	raw, execErr := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if execErr == nil {
		t.Fatal("Execute() expected error when DeleteFiles target is non-empty directory, got nil")
	}
	combined := string(raw.Stderr) + execErr.Error()
	if !strings.Contains(combined, "delete file") {
		t.Errorf("expected 'delete file' in stderr+error, got: %q", combined)
	}
}

func TestScenarioStep_ZeroValue_MockResponse(t *testing.T) {
	// Zero-value ScenarioStep should produce valid JSON response with ExitCode=0,
	// empty SessionID, default output
	scenario := testutil.Scenario{
		Name: "zero value step",
		Steps: []testutil.ScenarioStep{
			{}, // zero value
		},
	}
	testutil.SetupMockClaude(t, scenario)
	dir := t.TempDir()

	start := time.Now()
	raw, err := session.Execute(context.Background(), session.Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute() unexpected error for zero-value step: %v", err)
	}

	result, parseErr := session.ParseResult(raw, elapsed)
	if parseErr != nil {
		t.Fatalf("ParseResult() unexpected error: %v", parseErr)
	}
	if result.SessionID != "" {
		t.Errorf("SessionID = %q, want empty for zero-value step", result.SessionID)
	}
	if result.Output != "Mock output for step 0" {
		t.Errorf("Output = %q, want %q", result.Output, "Mock output for step 0")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}
