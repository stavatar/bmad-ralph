package testutil

import (
	"context"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// Compile-time interface check.
var _ runner.GitClient = (*MockGitClient)(nil)

// MockGitClient implements runner.GitClient with sequence-based responses
// and call tracking for unit tests. Zero value returns healthy/empty/nil.
type MockGitClient struct {
	HealthCheckErrors []error             // sequence of errors to return; returns nil beyond slice length
	HeadCommits       []string            // sequence of SHAs to return
	HeadCommitErrors  []error             // sequence of errors for HeadCommit (parallel to HeadCommits)
	RestoreCleanError error               // single error value for RestoreClean
	DiffStatsResults  []*runner.DiffStats // sequence of DiffStats to return (indexed)
	DiffStatsErrors   []error             // sequence of errors for DiffStats (indexed)

	HealthCheckCount  int // tracks HealthCheck calls
	HeadCommitCount   int // tracks HeadCommit calls
	RestoreCleanCount int // tracks RestoreClean calls
	DiffStatsCount    int // tracks DiffStats calls
}

// HealthCheck returns the next error from HealthCheckErrors sequence.
// Returns nil if index exceeds slice length or slice is empty.
func (m *MockGitClient) HealthCheck(_ context.Context) error {
	idx := m.HealthCheckCount
	m.HealthCheckCount++
	if idx < len(m.HealthCheckErrors) {
		return m.HealthCheckErrors[idx]
	}
	return nil
}

// HeadCommit returns the next SHA from HeadCommits sequence.
// If HeadCommitErrors has a non-nil entry at the current index, returns ("", err).
// If index exceeds slice length, returns the last element (or "" if empty).
func (m *MockGitClient) HeadCommit(_ context.Context) (string, error) {
	idx := m.HeadCommitCount
	m.HeadCommitCount++
	if idx < len(m.HeadCommitErrors) && m.HeadCommitErrors[idx] != nil {
		return "", m.HeadCommitErrors[idx]
	}
	if len(m.HeadCommits) == 0 {
		return "", nil
	}
	if idx < len(m.HeadCommits) {
		return m.HeadCommits[idx], nil
	}
	return m.HeadCommits[len(m.HeadCommits)-1], nil
}

// RestoreClean returns RestoreCleanError (nil by default).
func (m *MockGitClient) RestoreClean(_ context.Context) error {
	m.RestoreCleanCount++
	return m.RestoreCleanError
}

// DiffStats returns the next result from DiffStatsResults/DiffStatsErrors sequence.
// If index exceeds slice length, returns the last element (same pattern as HeadCommit).
func (m *MockGitClient) DiffStats(_ context.Context, _, _ string) (*runner.DiffStats, error) {
	idx := m.DiffStatsCount
	m.DiffStatsCount++
	if idx < len(m.DiffStatsErrors) && m.DiffStatsErrors[idx] != nil {
		return nil, m.DiffStatsErrors[idx]
	}
	if len(m.DiffStatsResults) == 0 {
		return &runner.DiffStats{}, nil
	}
	if idx < len(m.DiffStatsResults) {
		return m.DiffStatsResults[idx], nil
	}
	return m.DiffStatsResults[len(m.DiffStatsResults)-1], nil
}
