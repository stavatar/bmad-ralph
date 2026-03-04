package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// --- Task 6.1: SerenaMCPDetector Available with settings.json ---

func TestSerenaMCPDetector_Available_SettingsJson(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		json string
		want bool
	}{
		{
			name: "serena present",
			json: `{"mcpServers": {"serena": {"command": "serena", "args": ["--project", "."]}}}`,
			want: true,
		},
		{
			name: "serena-mcp present",
			json: `{"mcpServers": {"serena-mcp": {"command": "npx", "args": ["-y", "serena-mcp"]}}}`,
			want: true,
		},
		{
			name: "Serena uppercase present",
			json: `{"mcpServers": {"Serena": {"command": "serena"}}}`,
			want: true,
		},
		{
			name: "no serena in mcpServers",
			json: `{"mcpServers": {"other-tool": {"command": "other"}}}`,
			want: false,
		},
		{
			name: "empty mcpServers",
			json: `{"mcpServers": {}}`,
			want: false,
		},
		{
			name: "no mcpServers key",
			json: `{"otherKey": true}`,
			want: false,
		},
		{
			name: "mcpServers is array not object",
			json: `{"mcpServers": ["serena", "other"]}`,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			claudeDir := filepath.Join(tmpDir, ".claude")
			if err := os.MkdirAll(claudeDir, 0755); err != nil {
				t.Fatalf("mkdir .claude: %v", err)
			}
			if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(tc.json), 0644); err != nil {
				t.Fatalf("write settings.json: %v", err)
			}

			d := &runner.SerenaMCPDetector{}
			got := d.Available(tmpDir)
			if got != tc.want {
				t.Errorf("Available() = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- Task 6.2: Fallback to .mcp.json ---

func TestSerenaMCPDetector_Available_McpJson(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// No .claude/settings.json — fallback to .mcp.json
	mcpJSON := `{"mcpServers": {"serena-mcp": {"command": "npx", "args": ["-y", "serena-mcp"]}}}`
	if err := os.WriteFile(filepath.Join(tmpDir, ".mcp.json"), []byte(mcpJSON), 0644); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
	}

	d := &runner.SerenaMCPDetector{}
	if !d.Available(tmpDir) {
		t.Error("Available() = false, want true (fallback to .mcp.json)")
	}
}

// --- Task 6.3: Both missing ---

func TestSerenaMCPDetector_Available_BothMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	d := &runner.SerenaMCPDetector{}
	if d.Available(tmpDir) {
		t.Error("Available() = true, want false (both config files missing)")
	}
}

// --- Task 6.4: Malformed JSON ---

func TestSerenaMCPDetector_Available_MalformedJson(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	// Write malformed JSON
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := &runner.SerenaMCPDetector{}
	if d.Available(tmpDir) {
		t.Error("Available() = true, want false (malformed JSON should return false)")
	}
}

// --- Task 6.5: PromptHint ---

func TestSerenaMCPDetector_PromptHint(t *testing.T) {
	t.Parallel()
	d := &runner.SerenaMCPDetector{}
	hint := d.PromptHint()
	expected := "If Serena MCP tools available, use them for code navigation"
	if hint != expected {
		t.Errorf("PromptHint() = %q, want %q", hint, expected)
	}
}

// --- Task 6.6: NoOp Available ---

func TestNoOpCodeIndexerDetector_Available(t *testing.T) {
	t.Parallel()
	d := &runner.NoOpCodeIndexerDetector{}
	if d.Available("/any/path") {
		t.Error("Available() = true, want false")
	}
}

// --- Task 6.7: NoOp PromptHint ---

func TestNoOpCodeIndexerDetector_PromptHint(t *testing.T) {
	t.Parallel()
	d := &runner.NoOpCodeIndexerDetector{}
	if hint := d.PromptHint(); hint != "" {
		t.Errorf("PromptHint() = %q, want empty", hint)
	}
}

// --- Task 6.8: detectSerena with NoOp detector returns empty ---

func TestDetectSerena_NoOpDetector_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Write Serena config that WOULD be detected by SerenaMCPDetector
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"mcpServers": {"serena": {"command": "serena"}}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// NoOp detector ignores config — detectSerena returns empty hint
	hint := runner.DetectSerena(&runner.NoOpCodeIndexerDetector{}, tmpDir)
	if hint != "" {
		t.Errorf("DetectSerena(NoOp) = %q, want empty", hint)
	}
}

// --- Task 6.9: DetectSerena with real detector returns hint and logs to stderr ---

func TestDetectSerena_SerenaMCPDetector_ReturnsHintAndLogs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Write Serena config
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"mcpServers": {"serena": {"command": "serena"}}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Capture stderr to verify log output
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	hint := runner.DetectSerena(&runner.SerenaMCPDetector{}, tmpDir)

	// Restore stderr and read captured output
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	os.Stderr = oldStderr

	captured := make([]byte, 1024)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	// Verify hint content
	if !strings.Contains(hint, "Serena MCP tools") {
		t.Errorf("DetectSerena() hint should contain 'Serena MCP tools', got %q", hint)
	}
	if !strings.Contains(hint, "code navigation") {
		t.Errorf("DetectSerena() hint should contain 'code navigation', got %q", hint)
	}

	// Verify stderr log (L2 finding: detectSerena logs to stderr)
	if !strings.Contains(stderrOutput, "Serena MCP detected") {
		t.Errorf("DetectSerena() should log to stderr, got %q", stderrOutput)
	}
}
