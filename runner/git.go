package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// GitClient abstracts git operations for the runner.
// Consumer-side interface per naming convention (interfaces in consumer package).
type GitClient interface {
	HealthCheck(ctx context.Context) error
	HeadCommit(ctx context.Context) (string, error)
	RestoreClean(ctx context.Context) error
	DiffStats(ctx context.Context, before, after string) (*DiffStats, error)
	LogOneline(ctx context.Context, n int) ([]string, error)
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

// DiffStats returns file change statistics between two commits using git diff --numstat.
// Returns zero-value DiffStats when before == after (no git command executed).
// Returns error for empty inputs or git failures.
func (c *ExecGitClient) DiffStats(ctx context.Context, before, after string) (*DiffStats, error) {
	if before == "" || after == "" {
		return nil, fmt.Errorf("runner: diff stats: before and after SHAs must be non-empty")
	}
	if before == after {
		return &DiffStats{}, nil
	}

	cmd := exec.CommandContext(ctx, "git", "diff", "--numstat", before, after)
	cmd.Dir = c.Dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("runner: diff stats: %w", err)
	}

	var stats DiffStats
	pkgSet := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		stats.FilesChanged++
		// Binary files show "-" for insertions/deletions in numstat
		if parts[0] != "-" {
			if ins, err := strconv.Atoi(parts[0]); err == nil {
				stats.Insertions += ins
			}
		}
		if parts[1] != "-" {
			if del, err := strconv.Atoi(parts[1]); err == nil {
				stats.Deletions += del
			}
		}
		dir := filepath.ToSlash(filepath.Dir(parts[2]))
		pkgSet[dir] = true
	}

	// Sort packages for deterministic output
	stats.Packages = make([]string, 0, len(pkgSet))
	for pkg := range pkgSet {
		stats.Packages = append(stats.Packages, pkg)
	}
	sort.Strings(stats.Packages)
	return &stats, nil
}

// LogOneline returns the last n commit subjects via `git log --oneline -<n>`.
// Each line is one commit (abbreviated hash + subject).
func (e *ExecGitClient) LogOneline(ctx context.Context, n int) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", fmt.Sprintf("-%d", n))
	cmd.Dir = e.Dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("runner: git log oneline: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}
