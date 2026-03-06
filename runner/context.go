package runner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmad-ralph/bmad-ralph/session"
)

// compactHookScript is the shell script content for the PreCompact hook.
const compactHookScript = "#!/bin/bash\n[ -n \"$RALPH_COMPACT_COUNTER\" ] && echo 1 >> \"$RALPH_COMPACT_COUNTER\"\n"

// compactHookCommand is the command path stored in settings.json for the hook.
const compactHookCommand = ".ralph/hooks/count-compact.sh"

// DefaultContextWindow is the fallback context window size (tokens) when
// the actual value is not available from Claude Code's modelUsage.
const DefaultContextWindow = 200000

// CreateCompactCounter creates a temporary file used by Claude Code's
// PreCompact hook to count context compactions. Returns the file path
// and a cleanup function that removes it. On error, returns ("", no-op).
func CreateCompactCounter() (string, func()) {
	f, err := os.CreateTemp("", "ralph-compact-*")
	if err != nil {
		return "", func() {}
	}
	path := f.Name()
	f.Close()
	return path, func() {
		os.Remove(path) //nolint:errcheck // cleanup best-effort, idempotent
	}
}

// CountCompactions reads the compact counter file and returns the number
// of compactions recorded (non-empty lines). Returns 0 on any error or
// empty/missing file (graceful degradation).
func CountCompactions(path string) int {
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if len(data) == 0 {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			count++
		}
	}
	return count
}

// EstimateMaxContextFill estimates the peak context window fill percentage
// using cumulative token counts from a session.
//
// Formula:
//
//	total_cumulative = cache_read + cache_creation + input_tokens
//	effective_turns = max(num_turns, 2)
//	estimated_max = 2 × total_cumulative / effective_turns
//	fill_pct = estimated_max / context_window × 100
//
// Returns 0.0 for nil metrics, zero turns, or zero context window.
// Uses metrics.ContextWindow if > 0, otherwise falls back to fallbackContextWindow.
func EstimateMaxContextFill(metrics *session.SessionMetrics, fallbackContextWindow int) float64 {
	if metrics == nil {
		return 0.0
	}
	if metrics.NumTurns == 0 {
		return 0.0
	}

	contextWindow := metrics.ContextWindow
	if contextWindow <= 0 {
		contextWindow = fallbackContextWindow
	}
	if contextWindow <= 0 {
		return 0.0
	}

	totalCumulative := float64(metrics.CacheReadTokens + metrics.CacheCreationTokens + metrics.InputTokens)
	effectiveTurns := math.Max(float64(metrics.NumTurns), 2.0)
	estimatedMax := 2.0 * totalCumulative / effectiveTurns

	return estimatedMax / float64(contextWindow) * 100.0
}

// EnsureCompactHook creates the PreCompact hook script and registers it in
// Claude Code's settings.json. The hook appends "1\n" to $RALPH_COMPACT_COUNTER
// on each compaction event.
//
// Script: .ralph/hooks/count-compact.sh (created/updated, chmod 0755)
// Settings: .claude/settings.json (additive merge, backup before first modification)
//
// Returns error on corrupt settings.json or I/O failure. Caller should log
// as warning and continue (compactions=0 fallback).
func EnsureCompactHook(projectRoot string) error {
	if err := ensureHookScript(projectRoot); err != nil {
		return fmt.Errorf("runner: ensure compact hook: script: %w", err)
	}
	if err := ensureHookSettings(projectRoot); err != nil {
		return fmt.Errorf("runner: ensure compact hook: settings: %w", err)
	}
	return nil
}

// LogContextWarnings logs warnings/errors based on context fill percentage and compactions.
// Silent when fillPct <= warnPct and compactions == 0 (AC1).
// Logs WARN when fillPct > warnPct but <= criticalPct (AC2).
// Logs ERROR when fillPct > criticalPct (AC3) or compactions > 0 (AC4).
// Both fill and compaction checks are independent — both may fire (AC5).
// No-op when log is nil.
func LogContextWarnings(log *RunLogger, fillPct float64, compactions int, maxTurns int, warnPct int, criticalPct int) {
	if log == nil {
		return
	}

	// Fill-level check.
	if fillPct > float64(criticalPct) {
		log.Error(fmt.Sprintf("context fill %.1f%% exceeds critical threshold — quality degradation likely, reduce max_turns (current: %d)", fillPct, maxTurns))
	} else if fillPct > float64(warnPct) {
		log.Warn(fmt.Sprintf("context fill %.1f%% — consider reducing max_turns (current: %d) or splitting task into smaller pieces", fillPct, maxTurns))
	}

	// Compaction check (independent of fill level).
	if compactions > 0 {
		log.Error(fmt.Sprintf("%d compaction(s) detected — context was compressed, quality degraded. Reduce max_turns (current: %d)", compactions, maxTurns))
	}
}

// ensureHookScript creates or updates .ralph/hooks/count-compact.sh.
// Skips write if file exists with correct content (idempotent).
func ensureHookScript(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".ralph", "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	scriptPath := filepath.Join(dir, "count-compact.sh")

	existing, err := os.ReadFile(scriptPath)
	if err == nil && string(existing) == compactHookScript {
		return nil // already correct
	}

	if err := os.WriteFile(scriptPath, []byte(compactHookScript), 0644); err != nil {
		return fmt.Errorf("write script: %w", err)
	}
	if err := os.Chmod(scriptPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

// ensureHookSettings reads .claude/settings.json, adds PreCompact hook entry
// if not already present, and writes back. Creates file if missing.
// Creates .claude/settings.json.bak before first modification.
func ensureHookSettings(projectRoot string) error {
	dir := filepath.Join(projectRoot, ".claude")
	settingsPath := filepath.Join(dir, "settings.json")
	bakPath := settingsPath + ".bak"

	// Read existing or start fresh.
	var data map[string]any
	existing, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(existing, &data); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}
	} else if os.IsNotExist(err) {
		data = map[string]any{}
	} else {
		return fmt.Errorf("read settings.json: %w", err)
	}

	// Navigate: hooks → PreCompact → []any
	hooks, ok := data["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		data["hooks"] = hooks
	}
	preCompact, _ := hooks["PreCompact"].([]any)

	// Idempotency: check if hook already registered.
	for _, entry := range preCompact {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooksArr, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooksArr {
			hMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hMap["command"].(string)
			if strings.Contains(cmd, "count-compact.sh") {
				return nil // already registered
			}
		}
	}

	// Append hook entry.
	hookEntry := map[string]any{
		"matcher": "auto",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": compactHookCommand,
			},
		},
	}
	preCompact = append(preCompact, hookEntry)
	hooks["PreCompact"] = preCompact

	// Marshal.
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings.json: %w", err)
	}
	out = append(out, '\n')

	// Backup before first modification (only if settings.json existed and .bak doesn't).
	if existing != nil {
		if _, err := os.Stat(bakPath); os.IsNotExist(err) {
			if err := os.WriteFile(bakPath, existing, 0644); err != nil {
				return fmt.Errorf("backup settings.json: %w", err)
			}
		}
	}

	// Create .claude/ dir if needed, write.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir .claude: %w", err)
	}
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}
	return nil
}
