package runner

import (
	"math"
	"os"
	"strings"

	"github.com/bmad-ralph/bmad-ralph/session"
)

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
