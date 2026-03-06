package runner

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/session"
)

func TestCreateCompactCounter_HappyPath(t *testing.T) {
	path, cleanup := CreateCompactCounter()
	defer cleanup()

	if path == "" {
		t.Fatal("CreateCompactCounter: path is empty, want non-empty")
	}
	if !strings.Contains(filepath.Base(path), "ralph-compact-") {
		t.Errorf("path base = %q, want containing %q", filepath.Base(path), "ralph-compact-")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist at %q: %v", path, err)
	}

	// Cleanup removes the file.
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed after cleanup, got err: %v", err)
	}
}

func TestCreateCompactCounter_CleanupIdempotent(t *testing.T) {
	path, cleanup := CreateCompactCounter()
	cleanup()
	// Second call should not panic.
	cleanup()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed, got err: %v", err)
	}
}

func TestCountCompactions_Cases(t *testing.T) {
	tests := []struct {
		name    string
		content string // "" means use missing file; "EMPTY" means empty file
		missing bool
		want    int
	}{
		{
			name:    "empty file",
			content: "",
			want:    0,
		},
		{
			name:    "1 compaction",
			content: "1\n",
			want:    1,
		},
		{
			name:    "3 compactions",
			content: "1\n1\n1\n",
			want:    3,
		},
		{
			name:    "missing file",
			missing: true,
			want:    0,
		},
		{
			name:    "empty path",
			content: "", // won't be used
			missing: true,
			want:    0,
		},
		{
			name:    "corrupt with blank lines",
			content: "1\n\n1\n\n",
			want:    2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			if tc.name == "empty path" {
				path = ""
			} else if tc.missing {
				path = filepath.Join(t.TempDir(), "nonexistent")
			} else {
				dir := t.TempDir()
				path = filepath.Join(dir, "compact-counter")
				if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			}

			got := CountCompactions(path)
			if got != tc.want {
				t.Errorf("CountCompactions = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEstimateMaxContextFill_Formula(t *testing.T) {
	tests := []struct {
		name                  string
		metrics               *session.SessionMetrics
		fallbackContextWindow int
		want                  float64
		tolerance             float64
	}{
		{
			name: "happy path real session",
			metrics: &session.SessionMetrics{
				CacheReadTokens:     1456521,
				CacheCreationTokens: 57388,
				InputTokens:         2700,
				NumTurns:            25,
			},
			fallbackContextWindow: 200000,
			want:                  60.66,
			tolerance:             0.1,
		},
		{
			name: "uses metrics.ContextWindow over fallback",
			metrics: &session.SessionMetrics{
				CacheReadTokens:     1456521,
				CacheCreationTokens: 57388,
				InputTokens:         2700,
				NumTurns:            25,
				ContextWindow:       200000,
			},
			fallbackContextWindow: 100000,
			want:                  60.66,
			tolerance:             0.1,
		},
		{
			name: "fallback when ContextWindow is 0",
			metrics: &session.SessionMetrics{
				CacheReadTokens:     1456521,
				CacheCreationTokens: 57388,
				InputTokens:         2700,
				NumTurns:            25,
				ContextWindow:       0,
			},
			fallbackContextWindow: 200000,
			want:                  60.66,
			tolerance:             0.1,
		},
		{
			name: "guard max numTurns 2",
			metrics: &session.SessionMetrics{
				CacheReadTokens:     20000,
				CacheCreationTokens: 5000,
				InputTokens:         500,
				NumTurns:            1,
			},
			fallbackContextWindow: 200000,
			want:                  12.75,
			tolerance:             0.1,
		},
		{
			name: "zero turns",
			metrics: &session.SessionMetrics{
				CacheReadTokens: 1000,
				NumTurns:        0,
			},
			fallbackContextWindow: 200000,
			want:                  0.0,
			tolerance:             0.001,
		},
		{
			name:                  "nil metrics",
			metrics:               nil,
			fallbackContextWindow: 200000,
			want:                  0.0,
			tolerance:             0.001,
		},
		{
			name: "zero context window both",
			metrics: &session.SessionMetrics{
				CacheReadTokens: 1000,
				NumTurns:        5,
				ContextWindow:   0,
			},
			fallbackContextWindow: 0,
			want:                  0.0,
			tolerance:             0.001,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateMaxContextFill(tc.metrics, tc.fallbackContextWindow)
			if math.Abs(got-tc.want) > tc.tolerance {
				t.Errorf("EstimateMaxContextFill = %f, want %f (±%f)", got, tc.want, tc.tolerance)
			}
		})
	}
}

func TestDefaultContextWindow_Value(t *testing.T) {
	if DefaultContextWindow != 200000 {
		t.Errorf("DefaultContextWindow = %d, want 200000", DefaultContextWindow)
	}
}
