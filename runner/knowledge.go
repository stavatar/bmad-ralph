package runner

import "context"

// ProgressData carries resume session outcome for knowledge tracking.
// Epic 6 may add fields (backward-compatible).
type ProgressData struct {
	SessionID       string // from resumed session's SessionResult
	TaskDescription string // first open task text from ScanResult, or ""
}

// KnowledgeWriter records execution progress and lessons.
// Epic 3: 1 method (WriteProgress). Epic 6 adds WriteLessons.
type KnowledgeWriter interface {
	WriteProgress(ctx context.Context, data ProgressData) error
}

// NoOpKnowledgeWriter is the default KnowledgeWriter — returns nil for WriteProgress.
type NoOpKnowledgeWriter struct{}

// WriteProgress is a no-op that discards progress data. Real implementation in Epic 6.
func (n *NoOpKnowledgeWriter) WriteProgress(_ context.Context, _ ProgressData) error {
	return nil
}

// Compile-time interface check.
var _ KnowledgeWriter = (*NoOpKnowledgeWriter)(nil)
