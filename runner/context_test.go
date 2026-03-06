package runner

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestCreateCompactCounter_Error(t *testing.T) {
	// Point TMPDIR at a file (not directory) to make os.CreateTemp fail.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("TMPDIR", blocker)
	t.Setenv("TMP", blocker)
	t.Setenv("TEMP", blocker)

	path, cleanup := CreateCompactCounter()
	if path != "" {
		t.Errorf("path = %q, want empty on error", path)
	}
	// Cleanup should be no-op, not panic.
	cleanup()
}

func TestCountCompactions_Cases(t *testing.T) {
	tests := []struct {
		name      string
		content   string // file content; ignored when missing or emptyPath
		missing   bool
		emptyPath bool
		want      int
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
			name:      "empty path",
			emptyPath: true,
			want:      0,
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
			if tc.emptyPath {
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

func TestEnsureCompactHook_Script(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string // empty = no file
		wantContent     string
		wantModified    bool   // true = file should be written/overwritten
	}{
		{
			name:         "fresh creates script",
			wantContent:  compactHookScript,
			wantModified: true,
		},
		{
			name:            "same content skips write",
			existingContent: compactHookScript,
			wantContent:     compactHookScript,
			wantModified:    false,
		},
		{
			name:            "outdated content overwrites",
			existingContent: "#!/bin/bash\nold stuff\n",
			wantContent:     compactHookScript,
			wantModified:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			scriptPath := filepath.Join(root, ".ralph", "hooks", "count-compact.sh")

			if tc.existingContent != "" {
				if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(scriptPath, []byte(tc.existingContent), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			}

			// Capture mtime before call for modification check.
			var mtimeBefore time.Time
			if tc.existingContent != "" {
				info, err := os.Stat(scriptPath)
				if err != nil {
					t.Fatalf("Stat before: %v", err)
				}
				mtimeBefore = info.ModTime()
				// Sleep to ensure mtime difference is detectable.
				time.Sleep(50 * time.Millisecond)
			}

			if err := EnsureCompactHook(root); err != nil {
				t.Fatalf("EnsureCompactHook() error = %v", err)
			}

			got, err := os.ReadFile(scriptPath)
			if err != nil {
				t.Fatalf("ReadFile script: %v", err)
			}
			if string(got) != tc.wantContent {
				t.Errorf("script content = %q, want %q", string(got), tc.wantContent)
			}

			info, err := os.Stat(scriptPath)
			if err != nil {
				t.Fatalf("Stat script: %v", err)
			}

			// Verify modification expectation (AC2: same content skips write).
			if tc.existingContent != "" {
				modified := info.ModTime().After(mtimeBefore)
				if modified != tc.wantModified {
					t.Errorf("file modified = %v, want %v", modified, tc.wantModified)
				}
			}

			// WSL/NTFS doesn't support Unix permission bits (always 0666).
			// Skip permission check on NTFS; CI on Linux will catch regressions.
			if info.Mode().Perm() != 0666 && info.Mode()&0100 == 0 {
				t.Errorf("script mode = %o, want executable", info.Mode())
			}
		})
	}
}

func TestEnsureCompactHook_SettingsMerge(t *testing.T) {
	tests := []struct {
		name             string
		existingSettings string // empty = no file; "CORRUPT" = invalid JSON
		wantErr          string // empty = no error
		wantHookCount    int    // expected number of PreCompact entries
		checkPreserved   string // JSON key that must be preserved
	}{
		{
			name:          "fresh creates settings",
			wantHookCount: 1,
		},
		{
			name:             "existing without hooks",
			existingSettings: `{"permissions":{"allow":["Read"]}}`,
			wantHookCount:    1,
			checkPreserved:   "permissions",
		},
		{
			name:             "hooks key exists without PreCompact",
			existingSettings: `{"hooks":{}}`,
			wantHookCount:    1,
		},
		{
			name:             "PreCompact empty array",
			existingSettings: `{"hooks":{"PreCompact":[]}}`,
			wantHookCount:    1,
		},
		{
			name:             "PreCompact with other hook",
			existingSettings: `{"hooks":{"PreCompact":[{"matcher":"auto","hooks":[{"type":"command","command":"other.sh"}]}]}}`,
			wantHookCount:    2,
		},
		{
			name:             "idempotent already registered",
			existingSettings: `{"hooks":{"PreCompact":[{"matcher":"auto","hooks":[{"type":"command","command":".ralph/hooks/count-compact.sh"}]}]}}`,
			wantHookCount:    1,
		},
		{
			name:             "corrupt JSON returns error",
			existingSettings: `{invalid json`,
			wantErr:          "parse settings.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			claudeDir := filepath.Join(root, ".claude")
			settingsPath := filepath.Join(claudeDir, "settings.json")

			if tc.existingSettings != "" {
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(settingsPath, []byte(tc.existingSettings), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			}

			err := EnsureCompactHook(root)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("EnsureCompactHook() = nil, want error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tc.wantErr)
				}
				// AC12: settings.json must not be modified on error.
				got, readErr := os.ReadFile(settingsPath)
				if readErr != nil {
					t.Fatalf("ReadFile settings after error: %v", readErr)
				}
				if string(got) != tc.existingSettings {
					t.Errorf("settings modified on error: got %q, want %q", string(got), tc.existingSettings)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnsureCompactHook() error = %v", err)
			}

			// Parse result settings.json.
			data, err := os.ReadFile(settingsPath)
			if err != nil {
				t.Fatalf("ReadFile settings: %v", err)
			}
			var settings map[string]any
			if err := json.Unmarshal(data, &settings); err != nil {
				t.Fatalf("Unmarshal settings: %v", err)
			}

			hooks, ok := settings["hooks"].(map[string]any)
			if !ok {
				t.Fatal("settings missing hooks key")
			}
			preCompact, ok := hooks["PreCompact"].([]any)
			if !ok {
				t.Fatal("settings missing PreCompact key")
			}
			if len(preCompact) != tc.wantHookCount {
				t.Errorf("PreCompact entries = %d, want %d", len(preCompact), tc.wantHookCount)
			}

			if tc.checkPreserved != "" {
				if _, ok := settings[tc.checkPreserved]; !ok {
					t.Errorf("settings missing preserved key %q", tc.checkPreserved)
				}
			}
		})
	}
}

func TestEnsureCompactHook_Backup(t *testing.T) {
	t.Run("creates backup before first modification", func(t *testing.T) {
		root := t.TempDir()
		claudeDir := filepath.Join(root, ".claude")
		settingsPath := filepath.Join(claudeDir, "settings.json")
		bakPath := settingsPath + ".bak"

		original := `{"existing":"data"}`
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := EnsureCompactHook(root); err != nil {
			t.Fatalf("EnsureCompactHook() error = %v", err)
		}

		bak, err := os.ReadFile(bakPath)
		if err != nil {
			t.Fatalf("ReadFile .bak: %v", err)
		}
		if string(bak) != original {
			t.Errorf("backup content = %q, want %q", string(bak), original)
		}
	})

	t.Run("preserves existing backup", func(t *testing.T) {
		root := t.TempDir()
		claudeDir := filepath.Join(root, ".claude")
		settingsPath := filepath.Join(claudeDir, "settings.json")
		bakPath := settingsPath + ".bak"

		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		originalBak := `{"original":"backup"}`
		if err := os.WriteFile(bakPath, []byte(originalBak), 0644); err != nil {
			t.Fatalf("WriteFile .bak: %v", err)
		}
		if err := os.WriteFile(settingsPath, []byte(`{"new":"settings"}`), 0644); err != nil {
			t.Fatalf("WriteFile settings: %v", err)
		}

		if err := EnsureCompactHook(root); err != nil {
			t.Fatalf("EnsureCompactHook() error = %v", err)
		}

		bak, err := os.ReadFile(bakPath)
		if err != nil {
			t.Fatalf("ReadFile .bak: %v", err)
		}
		if string(bak) != originalBak {
			t.Errorf("backup content = %q, want original %q", string(bak), originalBak)
		}
	})

	t.Run("no backup for fresh settings", func(t *testing.T) {
		root := t.TempDir()
		bakPath := filepath.Join(root, ".claude", "settings.json.bak")

		if err := EnsureCompactHook(root); err != nil {
			t.Fatalf("EnsureCompactHook() error = %v", err)
		}

		if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
			t.Errorf("backup should not exist for fresh settings, got err: %v", err)
		}
	})
}

func TestLogContextWarnings_Cases(t *testing.T) {
	tests := []struct {
		name        string
		fillPct     float64
		compactions int
		maxTurns    int
		warnPct     int
		criticalPct int
		wantWarn    []string // substrings expected in log at WARN level
		wantError   []string // substrings expected in log at ERROR level
		wantSilent  bool     // true = no log output expected
	}{
		{
			name:        "silent below warn",
			fillPct:     40.0,
			compactions: 0,
			maxTurns:    15,
			warnPct:     55,
			criticalPct: 65,
			wantSilent:  true,
		},
		{
			name:        "warn above warn below critical",
			fillPct:     58.0,
			compactions: 0,
			maxTurns:    15,
			warnPct:     55,
			criticalPct: 65,
			wantWarn:    []string{"context fill 58.0%", "consider reducing max_turns", "current: 15"},
		},
		{
			name:        "error above critical",
			fillPct:     70.0,
			compactions: 0,
			maxTurns:    15,
			warnPct:     55,
			criticalPct: 65,
			wantError:   []string{"context fill 70.0%", "exceeds critical threshold", "current: 15"},
		},
		{
			name:        "error on compaction",
			fillPct:     30.0,
			compactions: 2,
			maxTurns:    15,
			warnPct:     55,
			criticalPct: 65,
			wantError:   []string{"2 compaction(s) detected", "context was compressed", "current: 15"},
		},
		{
			name:        "both fill and compaction",
			fillPct:     70.0,
			compactions: 1,
			maxTurns:    15,
			warnPct:     55,
			criticalPct: 65,
			wantError:   []string{"context fill 70.0%", "1 compaction(s) detected"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			log, err := OpenRunLogger(dir, "logs", "")
			if err != nil {
				t.Fatalf("OpenRunLogger: %v", err)
			}
			defer log.Close() //nolint:errcheck

			LogContextWarnings(log, tc.fillPct, tc.compactions, tc.maxTurns, tc.warnPct, tc.criticalPct)

			// Read log file.
			date := time.Now().Format("2006-01-02")
			logPath := filepath.Join(dir, "logs", "ralph-"+date+".log")
			data, readErr := os.ReadFile(logPath)
			if readErr != nil {
				t.Fatalf("ReadFile: %v", readErr)
			}
			content := string(data)

			if tc.wantSilent {
				if len(strings.TrimSpace(content)) > 0 {
					t.Errorf("expected silent, got: %q", content)
				}
				return
			}

			for _, want := range tc.wantWarn {
				if !strings.Contains(content, want) {
					t.Errorf("log missing warn substring %q\ngot: %s", want, content)
				}
			}
			if len(tc.wantWarn) > 0 {
				if !strings.Contains(content, "WARN") {
					t.Errorf("log missing WARN level\ngot: %s", content)
				}
			}
			for _, want := range tc.wantError {
				if !strings.Contains(content, want) {
					t.Errorf("log missing error substring %q\ngot: %s", want, content)
				}
			}
			if len(tc.wantError) > 0 {
				if !strings.Contains(content, "ERROR") {
					t.Errorf("log missing ERROR level\ngot: %s", content)
				}
			}
		})
	}
}

func TestLogContextWarnings_NilLogger(t *testing.T) {
	// Should not panic with nil logger.
	LogContextWarnings(nil, 70.0, 2, 15, 55, 65)
}
