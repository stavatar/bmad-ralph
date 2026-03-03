package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodeIndexerDetector detects code indexing tools and provides prompt hints.
// Minimal interface per M5/C3: no index commands, no timeout management, no progress output.
type CodeIndexerDetector interface {
	Available(projectRoot string) bool
	PromptHint() string
}

// NoOpCodeIndexerDetector is the default when Serena detection is disabled.
type NoOpCodeIndexerDetector struct{}

// Available always returns false for NoOpCodeIndexerDetector.
func (n *NoOpCodeIndexerDetector) Available(_ string) bool { return false }

// PromptHint always returns empty string for NoOpCodeIndexerDetector.
func (n *NoOpCodeIndexerDetector) PromptHint() string { return "" }

// SerenaMCPDetector detects Serena MCP server via config file inspection.
// Detection is file-based only — no exec.LookPath, no subprocess calls (C3).
type SerenaMCPDetector struct{}

// Compile-time interface checks.
var (
	_ CodeIndexerDetector = (*SerenaMCPDetector)(nil)
	_ CodeIndexerDetector = (*NoOpCodeIndexerDetector)(nil)
)

// Available checks .claude/settings.json and .mcp.json for Serena MCP config.
// Best-effort: any read/parse error returns false.
func (s *SerenaMCPDetector) Available(projectRoot string) bool {
	// Try .claude/settings.json first
	settingsPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if hasSerenamcp(settingsPath) {
		return true
	}

	// Fallback: .mcp.json
	mcpPath := filepath.Join(projectRoot, ".mcp.json")
	return hasSerenamcp(mcpPath)
}

// PromptHint returns the Serena prompt hint for injection into execute/review prompts.
func (s *SerenaMCPDetector) PromptHint() string {
	return "If Serena MCP tools available, use them for code navigation"
}

// hasSerenamcp reads a JSON config file and checks for Serena in mcpServers keys.
func hasSerenamcp(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false
	}

	servers, ok := parsed["mcpServers"]
	if !ok {
		return false
	}

	serversMap, ok := servers.(map[string]any)
	if !ok {
		return false
	}

	for key := range serversMap {
		if strings.Contains(strings.ToLower(key), "serena") {
			return true
		}
	}
	return false
}

// serenaDetectedMsg is the stderr log message emitted when Serena MCP is detected at startup.
const serenaDetectedMsg = "Serena MCP detected"

// DetectSerena runs Serena detection at startup and logs if found.
// Returns hint string for prompt injection (empty if unavailable).
func DetectSerena(indexer CodeIndexerDetector, projectRoot string) string {
	if indexer.Available(projectRoot) {
		fmt.Fprintf(os.Stderr, "%s\n", serenaDetectedMsg)
		return indexer.PromptHint()
	}
	return ""
}
