package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

func TestMockGitClient_HealthCheck_Sequence(t *testing.T) {
	mock := &MockGitClient{
		HealthCheckErrors: []error{runner.ErrDirtyTree, nil},
	}

	// First call: ErrDirtyTree
	err := mock.HealthCheck(context.Background())
	if !errors.Is(err, runner.ErrDirtyTree) {
		t.Errorf("call 1: want ErrDirtyTree, got %v", err)
	}

	// Second call: nil
	err = mock.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("call 2: want nil, got %v", err)
	}

	if mock.HealthCheckCount != 2 {
		t.Errorf("HealthCheckCount: got %d, want 2", mock.HealthCheckCount)
	}

	// Call beyond slice length — returns nil (healthy by default)
	err = mock.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("call 3 (beyond length): want nil, got %v", err)
	}
	if mock.HealthCheckCount != 3 {
		t.Errorf("HealthCheckCount after beyond-length: got %d, want 3", mock.HealthCheckCount)
	}
}

func TestMockGitClient_HeadCommit_Sequence(t *testing.T) {
	mock := &MockGitClient{
		HeadCommits: []string{"abc", "abc", "def"},
	}

	expected := []string{"abc", "abc", "def"}
	for i, want := range expected {
		got, err := mock.HeadCommit(context.Background())
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if got != want {
			t.Errorf("call %d: got %q, want %q", i+1, got, want)
		}
	}

	if mock.HeadCommitCount != 3 {
		t.Errorf("HeadCommitCount: got %d, want 3", mock.HeadCommitCount)
	}

	// Call beyond slice length — returns last element
	got, err := mock.HeadCommit(context.Background())
	if err != nil {
		t.Fatalf("call 4: unexpected error: %v", err)
	}
	if got != "def" {
		t.Errorf("call 4 (beyond length): got %q, want %q", got, "def")
	}
}

func TestMockGitClient_HeadCommit_Error(t *testing.T) {
	commitErr := errors.New("mock head commit error")
	mock := &MockGitClient{
		HeadCommitErrors: []error{commitErr},
		HeadCommits:      []string{"abc"},
	}

	sha, err := mock.HeadCommit(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if sha != "" {
		t.Errorf("SHA on error: got %q, want empty", sha)
	}
	if !errors.Is(err, commitErr) {
		t.Errorf("error: got %v, want %v", err, commitErr)
	}
	if mock.HeadCommitCount != 1 {
		t.Errorf("HeadCommitCount: got %d, want 1", mock.HeadCommitCount)
	}
}

func TestMockGitClient_RestoreClean_CallTracking(t *testing.T) {
	mock := &MockGitClient{}

	err := mock.RestoreClean(context.Background())
	if err != nil {
		t.Errorf("RestoreClean: want nil, got %v", err)
	}
	if mock.RestoreCleanCount != 1 {
		t.Errorf("RestoreCleanCount: got %d, want 1", mock.RestoreCleanCount)
	}
}

func TestMockGitClient_RestoreClean_Error(t *testing.T) {
	restoreErr := errors.New("mock restore error")
	mock := &MockGitClient{
		RestoreCleanError: restoreErr,
	}

	err := mock.RestoreClean(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, restoreErr) {
		t.Errorf("error: got %v, want %v", err, restoreErr)
	}
	if mock.RestoreCleanCount != 1 {
		t.Errorf("RestoreCleanCount: got %d, want 1", mock.RestoreCleanCount)
	}
}

func TestMockGitClient_ZeroValue(t *testing.T) {
	mock := &MockGitClient{}

	// HealthCheck returns nil
	err := mock.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck zero value: want nil, got %v", err)
	}

	// HeadCommit returns ("", nil)
	sha, err := mock.HeadCommit(context.Background())
	if err != nil {
		t.Errorf("HeadCommit zero value: want nil error, got %v", err)
	}
	if sha != "" {
		t.Errorf("HeadCommit zero value SHA: want empty, got %q", sha)
	}

	// RestoreClean returns nil
	err = mock.RestoreClean(context.Background())
	if err != nil {
		t.Errorf("RestoreClean zero value: want nil, got %v", err)
	}

	// Verify all counts == 1
	if mock.HealthCheckCount != 1 {
		t.Errorf("HealthCheckCount: got %d, want 1", mock.HealthCheckCount)
	}
	if mock.HeadCommitCount != 1 {
		t.Errorf("HeadCommitCount: got %d, want 1", mock.HeadCommitCount)
	}
	if mock.RestoreCleanCount != 1 {
		t.Errorf("RestoreCleanCount: got %d, want 1", mock.RestoreCleanCount)
	}
}
