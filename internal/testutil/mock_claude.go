package testutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// ScenarioStep defines a single mock Claude CLI response in a test scenario.
// Each step corresponds to one session.Execute invocation.
type ScenarioStep struct {
	Type          string            `json:"type"`                     // "execute" or "review" (metadata, not part of JSON output)
	ExitCode      int               `json:"exit_code"`                // Process exit code
	SessionID     string            `json:"session_id"`               // Mock session ID for JSON response
	OutputFile    string            `json:"output_file,omitempty"`    // File with custom output text (optional, relative to scenario dir)
	CreatesCommit bool              `json:"creates_commit,omitempty"` // Signal for future MockGitClient (stored, not acted on)
	IsError       bool              `json:"is_error,omitempty"`       // When true, JSON output uses subtype:"error" and is_error:true (matches real CLI)
	WriteFiles    map[string]string `json:"write_files,omitempty"`    // relPath → content: files to write in MOCK_CLAUDE_PROJECT_ROOT
	DeleteFiles   []string          `json:"delete_files,omitempty"`   // relPaths to remove from MOCK_CLAUDE_PROJECT_ROOT
	Model         string            `json:"model,omitempty"`          // Override model in JSON response (for metrics tests)
	Usage         map[string]int    `json:"usage,omitempty"`          // Token usage data for JSON response (for metrics tests)
}

// Scenario defines an ordered sequence of mock Claude CLI responses for a test.
type Scenario struct {
	Name  string         `json:"name"`
	Steps []ScenarioStep `json:"steps"`
}

// mockSystemMessage matches the Claude CLI "system" init message format.
type mockSystemMessage struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Tools     []any  `json:"tools"`
	Model     string `json:"model"`
}

// mockResultMessage matches the Claude CLI "result" message format.
// Do NOT use omitempty on is_error, duration_ms, num_turns — real CLI always outputs them.
type mockResultMessage struct {
	Type      string         `json:"type"`
	Subtype   string         `json:"subtype"`
	SessionID string         `json:"session_id"`
	Result    string         `json:"result"`
	IsError   bool           `json:"is_error"`
	Duration  int            `json:"duration_ms"`
	NumTurns  int            `json:"num_turns"`
	Model     string         `json:"model,omitempty"`
	Usage     map[string]int `json:"usage,omitempty"`
}

// RunMockClaude is the self-reexec handler for mock Claude subprocess behavior.
// Returns false if MOCK_CLAUDE_SCENARIO env var is empty (normal test execution).
// When acting as mock, calls os.Exit and never returns.
//
// Usage in TestMain:
//
//	if testutil.RunMockClaude() {
//	    return // dead code, but documents intent
//	}
//	os.Exit(m.Run())
func RunMockClaude() bool {
	scenarioPath := os.Getenv("MOCK_CLAUDE_SCENARIO")
	if scenarioPath == "" {
		return false
	}

	stateDir := os.Getenv("MOCK_CLAUDE_STATE_DIR")
	if stateDir == "" {
		fmt.Fprintf(os.Stderr, "mock_claude: MOCK_CLAUDE_STATE_DIR not set\n")
		os.Exit(1)
	}

	// Read and parse scenario
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mock_claude: read scenario: %v\n", err)
		os.Exit(1)
	}

	var scenario Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		fmt.Fprintf(os.Stderr, "mock_claude: parse scenario: %v\n", err)
		os.Exit(1)
	}

	// Read step counter (0 if file missing; fail on other read errors)
	stepNum := 0
	counterPath := filepath.Join(stateDir, "counter")
	counterData, err := os.ReadFile(counterPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "mock_claude: read counter: %v\n", err)
		os.Exit(1)
	}
	if err == nil {
		stepNum, err = strconv.Atoi(string(counterData))
		if err != nil {
			fmt.Fprintf(os.Stderr, "mock_claude: parse counter: %v\n", err)
			os.Exit(1)
		}
	}

	// Bounds check
	if stepNum >= len(scenario.Steps) {
		fmt.Fprintf(os.Stderr, "mock_claude: step %d: beyond scenario (has %d steps)\n", stepNum, len(scenario.Steps))
		os.Exit(1)
	}

	step := scenario.Steps[stepNum]

	// Log received args
	argsJSON, err := json.Marshal(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mock_claude: marshal args: %v\n", err)
		os.Exit(1)
	}
	invocationPath := filepath.Join(stateDir, fmt.Sprintf("invocation_%d.json", stepNum))
	if err := os.WriteFile(invocationPath, argsJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "mock_claude: write invocation log: %v\n", err)
		os.Exit(1)
	}

	// Resolve output text
	outputText := fmt.Sprintf("Mock output for step %d", stepNum)
	if step.OutputFile != "" {
		scenarioDir := filepath.Dir(scenarioPath)
		outputPath := filepath.Join(scenarioDir, step.OutputFile)
		content, err := os.ReadFile(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mock_claude: read output file %q: %v\n", step.OutputFile, err)
			os.Exit(1)
		}
		outputText = string(content)
	}

	// Build Claude CLI JSON array
	sessionID := step.SessionID
	resultSubtype := "success"
	if step.IsError {
		resultSubtype = "error"
	}
	model := "mock-claude"
	if step.Model != "" {
		model = step.Model
	}
	messages := []any{
		mockSystemMessage{
			Type:      "system",
			Subtype:   "init",
			SessionID: sessionID,
			Tools:     []any{},
			Model:     model,
		},
		mockResultMessage{
			Type:      "result",
			Subtype:   resultSubtype,
			SessionID: sessionID,
			Result:    outputText,
			IsError:   step.IsError,
			Duration:  100,
			NumTurns:  1,
			Model:     model,
			Usage:     step.Usage,
		},
	}

	output, err := json.Marshal(messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mock_claude: marshal output: %v\n", err)
		os.Exit(1)
	}

	// Write to stdout — os.Stdout is unbuffered *os.File, writes flush immediately
	os.Stdout.Write(output)

	// Apply file side effects (review integration tests: mock writes [x] or findings)
	projectRoot := os.Getenv("MOCK_CLAUDE_PROJECT_ROOT")
	if projectRoot != "" {
		for relPath, content := range step.WriteFiles {
			if err := os.WriteFile(filepath.Join(projectRoot, relPath), []byte(content), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "mock_claude: write file %q: %v\n", relPath, err)
				os.Exit(1)
			}
		}
		for _, relPath := range step.DeleteFiles {
			if err := os.Remove(filepath.Join(projectRoot, relPath)); err != nil && !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "mock_claude: delete file %q: %v\n", relPath, err)
				os.Exit(1)
			}
		}
	}

	// Increment counter
	if err := os.WriteFile(counterPath, []byte(strconv.Itoa(stepNum+1)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "mock_claude: write counter: %v\n", err)
		os.Exit(1)
	}

	os.Exit(step.ExitCode)
	return true // unreachable, but satisfies return type
}

// SetupMockClaude creates a temporary scenario environment for mock Claude tests.
// It writes the scenario JSON to a temp directory, creates the state subdirectory,
// and sets the required env vars via t.Setenv.
// Returns (scenarioPath, stateDir) for test assertions.
func SetupMockClaude(t *testing.T, scenario Scenario) (string, string) {
	t.Helper()

	dir := t.TempDir()

	// Write scenario JSON
	data, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("SetupMockClaude: marshal scenario: %v", err)
	}
	scenarioPath := filepath.Join(dir, "scenario.json")
	if err := os.WriteFile(scenarioPath, data, 0644); err != nil {
		t.Fatalf("SetupMockClaude: write scenario: %v", err)
	}

	// Create state directory
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("SetupMockClaude: create state dir: %v", err)
	}

	// Set env vars
	t.Setenv("MOCK_CLAUDE_SCENARIO", scenarioPath)
	t.Setenv("MOCK_CLAUDE_STATE_DIR", stateDir)

	return scenarioPath, stateDir
}

// ReadInvocationArgs reads the logged CLI args for a specific invocation step.
// Returns the args slice that mock Claude received (os.Args[1:]).
func ReadInvocationArgs(t *testing.T, stateDir string, step int) []string {
	t.Helper()

	path := filepath.Join(stateDir, fmt.Sprintf("invocation_%d.json", step))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadInvocationArgs: read step %d: %v", step, err)
	}

	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatalf("ReadInvocationArgs: parse step %d: %v", step, err)
	}

	return args
}
