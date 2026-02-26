package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// TestNoOpKnowledgeWriter_WriteProgress_ReturnsNil verifies WriteProgress returns nil
// with non-zero ProgressData (AC4).
func TestNoOpKnowledgeWriter_WriteProgress_ReturnsNil(t *testing.T) {
	t.Parallel()
	kw := &runner.NoOpKnowledgeWriter{}
	data := runner.ProgressData{
		SessionID:       "test-session-123",
		TaskDescription: "Implement feature X",
	}
	err := kw.WriteProgress(context.Background(), data)
	if err != nil {
		t.Errorf("WriteProgress(non-zero data): want nil, got %v", err)
	}

	// Zero-value ProgressData
	if err := kw.WriteProgress(context.Background(), runner.ProgressData{}); err != nil {
		t.Errorf("WriteProgress(zero value): want nil, got %v", err)
	}
}

// TestNoOpKnowledgeWriter_WriteProgress_NoLearningsFile verifies that WriteProgress
// does NOT create a LEARNINGS.md file (AC5: FR17 deferred to Epic 6).
// Uses tmpDir as simulated ProjectRoot — real implementation (Epic 6) will write there.
func TestNoOpKnowledgeWriter_WriteProgress_NoLearningsFile(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Change to projectRoot so any file creation would land here
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	kw := &runner.NoOpKnowledgeWriter{}
	data := runner.ProgressData{
		SessionID:       "test-session-456",
		TaskDescription: "Another task",
	}
	if err := kw.WriteProgress(context.Background(), data); err != nil {
		t.Fatalf("WriteProgress: want nil, got %v", err)
	}

	// Verify LEARNINGS.md was NOT created in projectRoot
	learningsPath := filepath.Join(projectRoot, "LEARNINGS.md")
	if _, statErr := os.Stat(learningsPath); statErr == nil {
		t.Errorf("LEARNINGS.md exists at %s — no-op should not create files", learningsPath)
	}
}
