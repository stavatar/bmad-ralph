package runner

import "context"

// ProgressData carries resume session outcome for knowledge tracking.
// Epic 6 may add fields (backward-compatible).
type ProgressData struct {
	SessionID       string // from resumed session's SessionResult
	TaskDescription string // first open task text from ScanResult, or ""
}

// KnowledgeWriter records execution progress and validates lessons.
// Epic 3: WriteProgress. Epic 6: ValidateNewLessons (post-validation of LEARNINGS.md).
type KnowledgeWriter interface {
	WriteProgress(ctx context.Context, data ProgressData) error
	ValidateNewLessons(ctx context.Context, data LessonsData) error
}

// NoOpKnowledgeWriter is the default KnowledgeWriter — returns nil for all methods.
type NoOpKnowledgeWriter struct{}

// WriteProgress is a no-op that discards progress data.
func (n *NoOpKnowledgeWriter) WriteProgress(_ context.Context, _ ProgressData) error {
	return nil
}

// ValidateNewLessons is a no-op that skips lesson validation.
func (n *NoOpKnowledgeWriter) ValidateNewLessons(_ context.Context, _ LessonsData) error {
	return nil
}

// Compile-time interface check.
var _ KnowledgeWriter = (*NoOpKnowledgeWriter)(nil)
