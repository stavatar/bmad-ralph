//go:build integration

package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

// --- Helpers (Task 1) ---

// extractPromptFromArgs iterates args to find -p flag and returns the next arg.
// Returns empty string if not found.
func extractPromptFromArgs(args []string) string {
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// verifyMockFlags checks args contain expected Claude CLI flags:
// --dangerously-skip-permissions, --output-format json, -p, --max-turns.
func verifyMockFlags(t *testing.T, args []string) {
	t.Helper()
	hasSkipPerms := false
	hasOutputJSON := false
	hasPromptFlag := false
	hasMaxTurns := false
	maxTurnsValue := ""
	for i, arg := range args {
		switch arg {
		case "--dangerously-skip-permissions":
			hasSkipPerms = true
		case "--output-format":
			if i+1 < len(args) && args[i+1] == "json" {
				hasOutputJSON = true
			}
		case "-p":
			hasPromptFlag = true
		case "--max-turns":
			hasMaxTurns = true
			if i+1 < len(args) {
				maxTurnsValue = args[i+1]
			}
		}
	}
	if !hasSkipPerms {
		t.Error("mock args missing --dangerously-skip-permissions")
	}
	if !hasOutputJSON {
		t.Error("mock args missing --output-format json")
	}
	if !hasPromptFlag {
		t.Error("mock args missing -p flag")
	}
	if !hasMaxTurns {
		t.Error("mock args missing --max-turns flag")
	}
	if maxTurnsValue != "5" {
		t.Errorf("--max-turns value = %q, want %q", maxTurnsValue, "5")
	}
}

// findGoTool returns the path to the Go toolchain binary.
// Priority: GOEXE env → exec.LookPath("go") → runtime.GOROOT()/bin/go[.exe].
func findGoTool() (string, error) {
	// 1. GOEXE env (explicit override for non-standard paths like WSL)
	if goExe := os.Getenv("GOEXE"); goExe != "" {
		if _, err := os.Stat(goExe); err == nil {
			return goExe, nil
		}
	}
	// 2. exec.LookPath (standard PATH lookup)
	if goPath, err := exec.LookPath("go"); err == nil {
		return goPath, nil
	}
	// 3-4. runtime.GOROOT fallback (may not work on WSL — returns Windows path)
	goRoot := runtime.GOROOT()
	if goRoot != "" {
		for _, name := range []string{"go", "go.exe"} {
			goPath := filepath.Join(goRoot, "bin", name)
			if _, err := os.Stat(goPath); err == nil {
				return goPath, nil
			}
		}
	}
	return "", fmt.Errorf("go tool not found: set GOEXE or add Go to PATH")
}

// Package-level shared state for ralph binary build (sync.Once).
var (
	buildRalphOnce sync.Once
	ralphBinPath   string
	ralphBuildErr  error
)

// ensureRalphBin builds the ralph binary once per test run via sync.Once.
// Uses os.MkdirTemp (not t.TempDir) because the binary is shared across tests;
// temp dir is NOT auto-cleaned (OS responsibility, acceptable for integration tests).
// Returns binary path or calls t.Skipf if build fails.
func ensureRalphBin(t *testing.T) string {
	t.Helper()
	buildRalphOnce.Do(func() {
		goTool, err := findGoTool()
		if err != nil {
			ralphBuildErr = err
			return
		}

		tmpDir, err := os.MkdirTemp("", "ralph-integration-*")
		if err != nil {
			ralphBuildErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		binName := "ralph"
		if runtime.GOOS == "windows" {
			binName = "ralph.exe"
		}
		binPath := filepath.Join(tmpDir, binName)

		// Find module root: tests run from bridge/, one level up is project root
		wd, err := os.Getwd()
		if err != nil {
			ralphBuildErr = fmt.Errorf("getwd: %w", err)
			return
		}
		moduleRoot := filepath.Dir(wd)

		cmd := exec.Command(goTool, "build", "-o", binPath, "./cmd/ralph")
		cmd.Dir = moduleRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			ralphBuildErr = fmt.Errorf("build ralph: %w: %s", err, output)
			return
		}

		ralphBinPath = binPath
	})

	if ralphBuildErr != nil {
		t.Skipf("cannot build ralph binary: %v", ralphBuildErr)
	}
	return ralphBinPath
}

// setupCLIProject creates a TempDir with .ralph/config.yaml containing mockCfg.
// Used by CLI tests to set up a valid project root for config.Load detection.
func setupCLIProject(t *testing.T, mockCfg string) string {
	t.Helper()
	projectDir := t.TempDir()
	ralphDir := filepath.Join(projectDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		t.Fatalf("create .ralph dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ralphDir, "config.yaml"), []byte(mockCfg), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	return projectDir
}

// copyStoryFixture reads a fixture from testdata/ and writes it to destDir.
// Returns the full path to the written file. Deduplicates repeated copy boilerplate.
func copyStoryFixture(t *testing.T, fixtureName, destDir string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", fixtureName))
	if err != nil {
		t.Fatalf("read story fixture %s: %v", fixtureName, err)
	}
	destPath := filepath.Join(destDir, fixtureName)
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Fatalf("write story fixture %s: %v", fixtureName, err)
	}
	return destPath
}

// --- Integration Tests ---

// TestBridge_Integration_CreateFlow validates the complete bridge create flow
// end-to-end: story read → prompt assembly → mock Claude → output parse → file write.
func TestBridge_Integration_CreateFlow(t *testing.T) {
	projectDir := t.TempDir()
	storyDir := t.TempDir()

	// Copy story fixture to storyDir
	storyPath := copyStoryFixture(t, "story_single_3ac.md", storyDir)

	// Setup mock Claude with 1-step scenario
	scenario := testutil.Scenario{
		Name: "create-flow",
		Steps: []testutil.ScenarioStep{{
			Type:       "execute",
			ExitCode:   0,
			SessionID:  "integ-create",
			OutputFile: "mock_bridge_output.md",
		}},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_bridge_output.md")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	taskCount, promptLines, err := Run(context.Background(), cfg, []string{storyPath})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify flow results (AC #1)
	if taskCount != 3 {
		t.Errorf("taskCount = %d, want 3", taskCount)
	}
	if promptLines <= 0 {
		t.Errorf("promptLines = %d, want > 0", promptLines)
	}

	// Verify output file exists and matches fixture byte-for-byte (deterministic mock)
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	outContent, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	fixtureContent, err := os.ReadFile(filepath.Join("testdata", "mock_bridge_output.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if string(outContent) != string(fixtureContent) {
		t.Errorf("output does not match fixture byte-for-byte")
	}

	// Verify prompt content (AC #2): format contract, story content, FR instructions
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	prompt := extractPromptFromArgs(args)

	promptChecks := []struct {
		name   string
		substr string
	}{
		{"format contract", "Sprint Tasks Format Specification"},
		{"story content", "User Login Authentication"},
		{"gate instruction", "[GATE]"},
		{"setup instruction", "[SETUP]"},
		{"verify instruction", "[VERIFY]"},
		{"e2e instruction", "[E2E]"},
		{"source traceability", "source:"},
		{"AC classification", "AC Classification"},
		{"granularity rule", "Task Granularity Rule"},
		{"testing within tasks", "Testing Within Tasks"},
		{"conversion instructions", "Group related ACs into tasks"},
		{"negative examples", "DO NOT"},
	}
	for _, pc := range promptChecks {
		if !strings.Contains(prompt, pc.substr) {
			t.Errorf("prompt missing %s (%q)", pc.name, pc.substr)
		}
	}

	// Verify mock invocation flags
	verifyMockFlags(t, args)
}

// TestBridge_Integration_MergeFlow validates the multi-step create→modify→merge flow:
// first bridge creates sprint-tasks.md, then test marks tasks [x], second bridge merges.
func TestBridge_Integration_MergeFlow(t *testing.T) {
	projectDir := t.TempDir()
	storyDir := t.TempDir()

	// Copy story fixture
	storyPath := copyStoryFixture(t, "story_single_3ac.md", storyDir)

	// Setup 2-step mock: step 0 → create, step 1 → merge
	scenario := testutil.Scenario{
		Name: "merge-flow",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "integ-create", OutputFile: "mock_single_story.md"},
			{Type: "execute", ExitCode: 0, SessionID: "integ-merge", OutputFile: "mock_merge_completed.md"},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_single_story.md")
	copyFixtureToScenario(t, "mock_merge_completed.md")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	// Step 1 — Create
	taskCount, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err != nil {
		t.Fatalf("Step 1 (create) Run() error: %v", err)
	}
	if taskCount != 3 {
		t.Errorf("Step 1 taskCount = %d, want 3", taskCount)
	}

	// Verify sprint-tasks.md created with open tasks
	sprintTasksPath := filepath.Join(projectDir, "sprint-tasks.md")
	content, err := os.ReadFile(sprintTasksPath)
	if err != nil {
		t.Fatalf("read sprint-tasks.md after step 1: %v", err)
	}
	if !strings.Contains(string(content), "- [ ]") {
		t.Error("step 1 output missing open tasks")
	}

	// Between steps: mark some tasks as completed (prefix match via strings.Replace)
	modified := strings.Replace(string(content), "- [ ] Implement login", "- [x] Implement login", 1)
	modified = strings.Replace(modified, "- [ ] Add request validation", "- [x] Add request validation", 1)
	if modified == string(content) {
		t.Fatal("between-steps replacement did not change content")
	}
	if err := os.WriteFile(sprintTasksPath, []byte(modified), 0644); err != nil {
		t.Fatalf("write modified sprint-tasks.md: %v", err)
	}

	// Step 2 — Merge (auto-detects existing file → merge mode)
	mergeTaskCount, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err != nil {
		t.Fatalf("Step 2 (merge) Run() error: %v", err)
	}
	// mock_merge_completed.md has 3 open tasks — verify merge reported them
	if mergeTaskCount != 3 {
		t.Errorf("Step 2 mergeTaskCount = %d, want 3", mergeTaskCount)
	}

	// Verify merge results
	mergedContent, err := os.ReadFile(sprintTasksPath)
	if err != nil {
		t.Fatalf("read sprint-tasks.md after merge: %v", err)
	}
	mergedStr := string(mergedContent)

	if !strings.Contains(mergedStr, "- [x]") {
		t.Error("merged output missing completed tasks")
	}
	if !strings.Contains(mergedStr, "- [ ]") {
		t.Error("merged output missing open tasks")
	}

	// Verify .bak file matches pre-merge modified content byte-for-byte
	bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read .bak file: %v", err)
	}
	if string(bakContent) != modified {
		t.Errorf(".bak content does not match pre-merge modified content")
	}

	// Verify merge prompt contains "Merge Mode" and existing [x] tasks
	step1Args := testutil.ReadInvocationArgs(t, stateDir, 1)
	mergePrompt := extractPromptFromArgs(step1Args)
	if !strings.Contains(mergePrompt, "Merge Mode") {
		t.Error("merge prompt missing 'Merge Mode'")
	}
	if !strings.Contains(mergePrompt, "[x]") {
		t.Error("merge prompt missing '[x]' (existing tasks not injected)")
	}
}

// TestBridge_Integration_PromptParserContract validates prompt-to-parser contract:
// mock Claude returns bridge output → bridge parses → output consumable by config regex patterns.
func TestBridge_Integration_PromptParserContract(t *testing.T) {
	projectDir := t.TempDir()
	storyDir := t.TempDir()

	// Copy richest story fixture for maximum coverage
	storyPath := copyStoryFixture(t, "story_5ac_traceability.md", storyDir)

	// Setup mock with richest output fixture
	scenario := testutil.Scenario{
		Name: "parser-contract",
		Steps: []testutil.ScenarioStep{{
			Type:       "execute",
			ExitCode:   0,
			SessionID:  "integ-contract",
			OutputFile: "mock_source_traceability.md",
		}},
	}
	testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_source_traceability.md")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	taskCount, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Read output
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	outContent, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	output := string(outContent)

	// Cross-validate: every task has source field (1:1 mapping)
	validatedCount := validateTaskSourcePairs(t, output)

	// Regex validation: scan with config regex patterns — all must find >= 1 match
	lines := strings.Split(output, "\n")
	taskOpenCount, sourceCount, gateCount := 0, 0, 0
	for _, line := range lines {
		if config.TaskOpenRegex.MatchString(line) {
			taskOpenCount++
		}
		if config.SourceFieldRegex.MatchString(line) {
			sourceCount++
		}
		if config.GateTagRegex.MatchString(line) {
			gateCount++
		}
	}

	if taskOpenCount == 0 {
		t.Error("TaskOpenRegex found no matches in output")
	}
	if sourceCount == 0 {
		t.Error("SourceFieldRegex found no matches in output")
	}
	if gateCount == 0 {
		t.Error("GateTagRegex found no matches in output")
	}

	// Verify task count from Run matches validated count from regex scan
	if taskCount != validatedCount {
		t.Errorf("Run taskCount = %d, validated count = %d", taskCount, validatedCount)
	}
	if taskCount != taskOpenCount {
		t.Errorf("Run taskCount = %d, regex TaskOpenRegex count = %d", taskCount, taskOpenCount)
	}
	// Cross-validate: every open task should have a source field (1:1 mapping)
	if sourceCount != taskOpenCount {
		t.Errorf("sourceCount = %d, taskOpenCount = %d — every task must have a source field", sourceCount, taskOpenCount)
	}
}

// TestBridge_CLI_Success validates the full CLI path via compiled ralph binary:
// exec.Command("ralph", "bridge", storyFile) → Cobra → config.Load → bridge.Run → output.
func TestBridge_CLI_Success(t *testing.T) {
	ralphBin := ensureRalphBin(t)

	// os.Executable() gives absolute path to test binary (will be mock Claude).
	// NOT os.Args[0] — absolute path required because ralph runs from different working dir.
	testBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	// Setup mock scenario
	scenario := testutil.Scenario{
		Name: "cli-success",
		Steps: []testutil.ScenarioStep{{
			Type:       "execute",
			ExitCode:   0,
			SessionID:  "cli-success",
			OutputFile: "mock_bridge_output.md",
		}},
	}
	testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_bridge_output.md")

	// Create project with config pointing to test binary as Claude
	projectDir := setupCLIProject(t, fmt.Sprintf("claude_command: %q\nmax_turns: 5\n", testBin))

	// Create story file in project dir
	storyPath := filepath.Join(projectDir, "story.md")
	if err := os.WriteFile(storyPath, []byte("# Story: Test\n\n## AC-1\nTest acceptance criteria."), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Run ralph binary
	cmd := exec.Command(ralphBin, "bridge", storyPath)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ralph bridge failed: %v\noutput: %s", err, output)
	}

	// Verify success: exit code 0, output contains "Generated" and "tasks"
	outStr := string(output)
	if !strings.Contains(outStr, "Generated") {
		t.Errorf("stdout missing 'Generated': %q", outStr)
	}
	if !strings.Contains(outStr, "tasks") {
		t.Errorf("stdout missing 'tasks': %q", outStr)
	}

	// Verify sprint-tasks.md created in project dir
	sprintTasksPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, err := os.Stat(sprintTasksPath); err != nil {
		t.Errorf("sprint-tasks.md not created: %v", err)
	}
}

// TestBridge_CLI_Failure validates exit code mapping: bridge error → exit code 4 (exitFatal).
func TestBridge_CLI_Failure(t *testing.T) {
	ralphBin := ensureRalphBin(t)

	testBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	// Setup mock scenario with exit code 1 (session failure)
	scenario := testutil.Scenario{
		Name: "cli-failure",
		Steps: []testutil.ScenarioStep{{
			Type:      "execute",
			ExitCode:  1,
			SessionID: "cli-fail",
		}},
	}
	testutil.SetupMockClaude(t, scenario)

	// Create project with config
	projectDir := setupCLIProject(t, fmt.Sprintf("claude_command: %q\nmax_turns: 5\n", testBin))

	// Create story file
	storyPath := filepath.Join(projectDir, "story.md")
	if err := os.WriteFile(storyPath, []byte("# Story: Test\n\n## AC-1\nTest acceptance criteria."), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Run ralph binary, expect failure
	cmd := exec.Command(ralphBin, "bridge", storyPath)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected non-zero exit from ralph bridge")
	}

	// Verify exit code 4 (exitFatal mapping for bridge errors)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 4 {
		t.Errorf("exit code = %d, want 4 (exitFatal)", exitErr.ExitCode())
	}

	// Verify error output (color.Red("Error: %v", err) writes to stdout)
	outStr := string(output)
	if !strings.Contains(outStr, "Error:") {
		t.Errorf("output missing 'Error:': %q", outStr)
	}

	// Verify no sprint-tasks.md created
	sprintTasksPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, statErr := os.Stat(sprintTasksPath); statErr == nil {
		t.Error("sprint-tasks.md should not exist after failure")
	}
}
