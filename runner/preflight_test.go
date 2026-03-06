package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTaskHash_TableDriven verifies TaskHash strips prefixes and returns deterministic 6-char hex (AC#1, AC#2).
func TestTaskHash_TableDriven(t *testing.T) {
	t.Parallel()

	// Pre-compute expected hash for "Add validation"
	h := sha256.Sum256([]byte("Add validation"))
	expectedHash := hex.EncodeToString(h[:])[:6]

	cases := []struct {
		name     string
		input    string
		wantHash string
	}{
		{"open prefix stripped", "- [ ] Add validation", expectedHash},
		{"done prefix stripped", "- [x] Add validation", expectedHash},
		{"no prefix same hash", "Add validation", expectedHash},
		{"different text different hash", "- [ ] Delete user", ""},
		{"empty string", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TaskHash(tc.input)
			if len(got) != 6 {
				t.Fatalf("TaskHash(%q) length = %d, want 6", tc.input, len(got))
			}
			// Verify lowercase hex
			for _, c := range got {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("TaskHash(%q) = %q contains non-lowercase-hex char %q", tc.input, got, string(c))
				}
			}
			if tc.wantHash != "" && got != tc.wantHash {
				t.Errorf("TaskHash(%q) = %q, want %q", tc.input, got, tc.wantHash)
			}
		})
	}

	// AC#2: open and done prefix produce same hash
	openHash := TaskHash("- [ ] Add validation")
	doneHash := TaskHash("- [x] Add validation")
	bareHash := TaskHash("Add validation")
	if openHash != doneHash {
		t.Errorf("open hash %q != done hash %q", openHash, doneHash)
	}
	if openHash != bareHash {
		t.Errorf("open hash %q != bare hash %q", openHash, bareHash)
	}
}

// TestTaskHash_Determinism verifies same input produces same output (AC#1).
func TestTaskHash_Determinism(t *testing.T) {
	t.Parallel()
	h1 := TaskHash("- [ ] Implement login")
	h2 := TaskHash("- [ ] Implement login")
	if h1 != h2 {
		t.Errorf("non-deterministic: %q != %q", h1, h2)
	}
}

// mockLogOnelineGit implements GitClient for PreFlightCheck tests.
type mockLogOnelineGit struct {
	lines []string
	err   error
}

func (m *mockLogOnelineGit) HealthCheck(_ context.Context) error                        { return nil }
func (m *mockLogOnelineGit) HeadCommit(_ context.Context) (string, error)               { return "", nil }
func (m *mockLogOnelineGit) RestoreClean(_ context.Context) error                       { return nil }
func (m *mockLogOnelineGit) DiffStats(_ context.Context, _, _ string) (*DiffStats, error) { return nil, nil }
func (m *mockLogOnelineGit) LogOneline(_ context.Context, _ int) ([]string, error) {
	return m.lines, m.err
}

// TestPreFlightCheck_Skip verifies skip when commit found and no findings (AC#4).
func TestPreFlightCheck_Skip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskText := "- [ ] Add validation"
	hash := TaskHash(taskText)
	marker := "[task:" + hash + "]"

	mock := &mockLogOnelineGit{
		lines: []string{
			"abc1234 feat: add validation " + marker,
			"def5678 fix: typo",
		},
	}

	skip, reason := PreFlightCheck(context.Background(), mock, taskText, tmpDir)
	if !skip {
		t.Errorf("expected skip=true, got false; reason=%q", reason)
	}
	if !strings.Contains(reason, "commit found, no findings") {
		t.Errorf("reason should contain 'commit found, no findings', got %q", reason)
	}
}

// TestPreFlightCheck_ProceedWithFindings verifies proceed when commit found but findings exist (AC#5).
func TestPreFlightCheck_ProceedWithFindings(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskText := "- [ ] Add validation"
	hash := TaskHash(taskText)
	marker := "[task:" + hash + "]"

	// Create review-findings.md with content
	findingsPath := filepath.Join(tmpDir, "review-findings.md")
	if err := os.WriteFile(findingsPath, []byte("### Finding 1\nSome issue"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mock := &mockLogOnelineGit{
		lines: []string{"abc1234 feat: add validation " + marker},
	}

	skip, reason := PreFlightCheck(context.Background(), mock, taskText, tmpDir)
	if skip {
		t.Errorf("expected skip=false when findings exist, got true")
	}
	if !strings.Contains(reason, "commit found but findings exist") {
		t.Errorf("reason should contain 'commit found but findings exist', got %q", reason)
	}
}

// TestPreFlightCheck_ProceedNoCommit verifies proceed when no matching commit (AC#6).
func TestPreFlightCheck_ProceedNoCommit(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskText := "- [ ] Add validation"

	mock := &mockLogOnelineGit{
		lines: []string{"abc1234 fix: unrelated change", "def5678 refactor: cleanup"},
	}

	skip, reason := PreFlightCheck(context.Background(), mock, taskText, tmpDir)
	if skip {
		t.Errorf("expected skip=false when no matching commit, got true")
	}
	if !strings.Contains(reason, "no matching commit") {
		t.Errorf("reason should contain 'no matching commit', got %q", reason)
	}
}

// TestPreFlightCheck_GitError verifies graceful proceed on git error (AC#7).
func TestPreFlightCheck_GitError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskText := "- [ ] Add validation"

	mock := &mockLogOnelineGit{
		err: errors.New("git not available"),
	}

	skip, reason := PreFlightCheck(context.Background(), mock, taskText, tmpDir)
	if skip {
		t.Errorf("expected skip=false on git error, got true")
	}
	if !strings.Contains(reason, "git log error") {
		t.Errorf("reason should contain 'git log error', got %q", reason)
	}
}
