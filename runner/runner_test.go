package runner_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

func TestRecoverDirtyState_Scenarios(t *testing.T) {
	tests := []struct {
		name                 string
		mock                 *testutil.MockGitClient
		wantRecovered        bool
		wantErr              bool
		wantErrContains      string
		wantErrContainsInner string
		wantErrIs            error
		wantHealthCheckCount int
		wantRestoreCount     int
	}{
		{
			name:                 "clean repo",
			mock:                 &testutil.MockGitClient{},
			wantRecovered:        false,
			wantErr:              false,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "dirty tree recovery succeeds",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrDirtyTree},
			},
			wantRecovered:        true,
			wantErr:              false,
			wantHealthCheckCount: 1,
			wantRestoreCount:     1,
		},
		{
			name: "dirty tree recovery fails",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrDirtyTree},
				RestoreCleanError: errors.New("restore failed"),
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrContainsInner: "restore failed",
			wantHealthCheckCount: 1,
			wantRestoreCount:     1,
		},
		{
			name: "detached HEAD",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrDetachedHead},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            runner.ErrDetachedHead,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "merge in progress",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{runner.ErrMergeInProgress},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            runner.ErrMergeInProgress,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "context canceled",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{context.Canceled},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            context.Canceled,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
		{
			name: "context deadline exceeded",
			mock: &testutil.MockGitClient{
				HealthCheckErrors: []error{context.DeadlineExceeded},
			},
			wantRecovered:        false,
			wantErr:              true,
			wantErrContains:      "runner: dirty state recovery:",
			wantErrIs:            context.DeadlineExceeded,
			wantHealthCheckCount: 1,
			wantRestoreCount:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recovered, err := runner.RecoverDirtyState(context.Background(), tc.mock)

			if recovered != tc.wantRecovered {
				t.Errorf("recovered: got %v, want %v", recovered, tc.wantRecovered)
			}

			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil error, got %v", err)
			}

			if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error message: want containing %q, got %q", tc.wantErrContains, err.Error())
			}

			if tc.wantErrContainsInner != "" && !strings.Contains(err.Error(), tc.wantErrContainsInner) {
				t.Errorf("inner error: want containing %q, got %q", tc.wantErrContainsInner, err.Error())
			}

			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("errors.Is(err, %v): want true, got false; err = %v", tc.wantErrIs, err)
			}

			if tc.mock.HealthCheckCount != tc.wantHealthCheckCount {
				t.Errorf("HealthCheckCount: got %d, want %d", tc.mock.HealthCheckCount, tc.wantHealthCheckCount)
			}

			if tc.mock.RestoreCleanCount != tc.wantRestoreCount {
				t.Errorf("RestoreCleanCount: got %d, want %d", tc.mock.RestoreCleanCount, tc.wantRestoreCount)
			}
		})
	}
}
