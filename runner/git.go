package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitClient abstracts git operations for the runner.
// Consumer-side interface per naming convention (interfaces in consumer package).
type GitClient interface {
	HealthCheck(ctx context.Context) error
	HeadCommit(ctx context.Context) (string, error)
	RestoreClean(ctx context.Context) error
}

// Sentinel errors for git health check failures.
var (
	ErrDirtyTree       = errors.New("git: working tree is dirty")
	ErrDetachedHead    = errors.New("git: HEAD is detached")
	ErrMergeInProgress = errors.New("git: merge or rebase in progress")
)

// ExecGitClient implements GitClient (HealthCheck, HeadCommit, RestoreClean)
// by shelling out to the git binary. Dir is set to config.ProjectRoot by the caller.
type ExecGitClient struct {
	Dir string
}

// HealthCheck validates git repository health: not in merge/rebase, not detached
// HEAD, clean working tree. Checks are ordered most-specific-first so that a merge
// conflict returns ErrMergeInProgress rather than the less informative ErrDirtyTree.
func (e *ExecGitClient) HealthCheck(ctx context.Context) error {
	// Check for merge/rebase in progress (most specific — must precede dirty-tree check)
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = e.Dir
	gitDirOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("runner: git health check: %w", err)
	}
	gitDir := strings.TrimSpace(string(gitDirOut))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(e.Dir, gitDir)
	}

	mergeIndicators := []string{
		filepath.Join(gitDir, "MERGE_HEAD"),
		filepath.Join(gitDir, "rebase-merge"),
		filepath.Join(gitDir, "rebase-apply"),
	}
	for _, indicator := range mergeIndicators {
		if _, err := os.Stat(indicator); err == nil {
			return fmt.Errorf("runner: git health check: %w", ErrMergeInProgress)
		}
	}

	// Check for detached HEAD
	cmd = exec.CommandContext(ctx, "git", "symbolic-ref", "-q", "HEAD")
	cmd.Dir = e.Dir
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("runner: git health check: %w", ErrDetachedHead)
		}
		return fmt.Errorf("runner: git health check: %w", err)
	}

	// Check for dirty working tree
	cmd = exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = e.Dir
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("runner: git health check: %w", err)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("runner: git health check: %w", ErrDirtyTree)
	}

	return nil
}

// HeadCommit returns the full 40-char hex SHA of HEAD.
func (e *ExecGitClient) HeadCommit(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = e.Dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("runner: git head commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RestoreClean discards all uncommitted changes to tracked files via git checkout.
func (e *ExecGitClient) RestoreClean(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "--", ".")
	cmd.Dir = e.Dir
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("runner: git restore clean: %w", err)
	}
	return nil
}
