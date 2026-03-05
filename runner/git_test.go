package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit executes a git command in dir with deterministic author/committer env vars.
// Returns trimmed stdout. Calls t.Fatalf on failure.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// initGitRepo creates a temporary git repo with an initial commit.
func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "test")

	// Create initial commit
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("init\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial commit")

	return dir
}

func TestExecGitClient_HealthCheck_CleanRepo(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck on clean repo: want nil, got %v", err)
	}
}

func TestExecGitClient_HealthCheck_DirtyTree(t *testing.T) {
	dir := initGitRepo(t)

	// Create uncommitted file
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	client := &ExecGitClient{Dir: dir}
	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck on dirty tree: want error, got nil")
	}
	if !errors.Is(err, ErrDirtyTree) {
		t.Errorf("errors.Is(err, ErrDirtyTree): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "dirty") {
		t.Errorf("error message: want 'dirty', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "runner: git health check:") {
		t.Errorf("error prefix: want 'runner: git health check:', got %q", err.Error())
	}
}

func TestExecGitClient_HealthCheck_DetachedHead(t *testing.T) {
	dir := initGitRepo(t)

	// Create second commit
	if err := os.WriteFile(filepath.Join(dir, "second.txt"), []byte("second\n"), 0644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-m", "second commit")

	// Get first commit SHA and checkout to detach HEAD
	firstSHA := runGit(t, dir, "rev-parse", "HEAD~1")
	runGit(t, dir, "checkout", firstSHA)

	client := &ExecGitClient{Dir: dir}
	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck on detached HEAD: want error, got nil")
	}
	if !errors.Is(err, ErrDetachedHead) {
		t.Errorf("errors.Is(err, ErrDetachedHead): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "detached") {
		t.Errorf("error message: want 'detached', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "runner: git health check:") {
		t.Errorf("error prefix: want 'runner: git health check:', got %q", err.Error())
	}
}

func TestExecGitClient_HealthCheck_MergeInProgress(t *testing.T) {
	dir := initGitRepo(t)

	// Get actual default branch name (may be "master" or "main")
	defaultBranch := runGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD")

	// Create conflicting branches
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("main content\n"), 0644); err != nil {
		t.Fatalf("write main README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "main change")

	// Create and switch to feature branch
	runGit(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("feature content\n"), 0644); err != nil {
		t.Fatalf("write feature README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "feature change")

	// Go back to default branch and make conflicting change
	runGit(t, dir, "checkout", defaultBranch)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("conflicting main\n"), 0644); err != nil {
		t.Fatalf("write conflicting README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "conflicting main change")

	// Attempt merge — should fail with conflict
	mergeCmd := exec.Command("git", "merge", "feature")
	mergeCmd.Dir = dir
	mergeCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	mergeOut, mergeErr := mergeCmd.CombinedOutput()
	if mergeErr == nil {
		t.Skipf("merge did not produce conflict (auto-merged); output: %s", mergeOut)
	}

	client := &ExecGitClient{Dir: dir}
	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck during merge: want error, got nil")
	}
	if !errors.Is(err, ErrMergeInProgress) {
		t.Errorf("errors.Is(err, ErrMergeInProgress): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "merge") {
		t.Errorf("error message: want 'merge', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "runner: git health check:") {
		t.Errorf("error prefix: want 'runner: git health check:', got %q", err.Error())
	}
}

func TestExecGitClient_HealthCheck_RebaseInProgress(t *testing.T) {
	dir := initGitRepo(t)

	// Create rebase-merge indicator directly in .git dir to simulate rebase state.
	// This avoids fragile real-rebase setup while exercising the rebase-merge code path.
	gitDir := filepath.Join(dir, ".git", "rebase-merge")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("create rebase-merge dir: %v", err)
	}

	client := &ExecGitClient{Dir: dir}
	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck during rebase: want error, got nil")
	}
	if !errors.Is(err, ErrMergeInProgress) {
		t.Errorf("errors.Is(err, ErrMergeInProgress): want true, got false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "merge or rebase") {
		t.Errorf("error message: want 'merge or rebase', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "runner: git health check:") {
		t.Errorf("error prefix: want 'runner: git health check:', got %q", err.Error())
	}
}

func TestExecGitClient_HeadCommit_Success(t *testing.T) {
	dir := initGitRepo(t)

	// Get expected SHA via runGit helper
	expectedSHA := runGit(t, dir, "rev-parse", "HEAD")

	client := &ExecGitClient{Dir: dir}
	sha, err := client.HeadCommit(context.Background())
	if err != nil {
		t.Fatalf("HeadCommit: unexpected error: %v", err)
	}
	if sha != expectedSHA {
		t.Errorf("HeadCommit SHA: got %q, want %q", sha, expectedSHA)
	}
	// Verify no trailing newline
	if strings.ContainsAny(sha, "\n\r ") {
		t.Errorf("HeadCommit SHA contains whitespace: %q", sha)
	}
	// Verify full 40-char hex SHA
	if len(sha) != 40 {
		t.Errorf("HeadCommit SHA length: got %d, want 40", len(sha))
	}
}

func TestExecGitClient_HeadCommit_NotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir() // No git init

	client := &ExecGitClient{Dir: dir}
	sha, err := client.HeadCommit(context.Background())
	if err == nil {
		t.Fatal("HeadCommit on non-repo: want error, got nil")
	}
	if sha != "" {
		t.Errorf("HeadCommit SHA on error: want empty, got %q", sha)
	}
	if !strings.Contains(err.Error(), "runner: git head commit:") {
		t.Errorf("error prefix: want 'runner: git head commit:', got %q", err.Error())
	}
}

func TestExecGitClient_HealthCheck_NotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir() // No git init

	client := &ExecGitClient{Dir: dir}
	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck on non-repo: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: git health check:") {
		t.Errorf("error prefix: want 'runner: git health check:', got %q", err.Error())
	}
}

func TestExecGitClient_HealthCheck_ContextCanceled(t *testing.T) {
	dir := initGitRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := &ExecGitClient{Dir: dir}
	err := client.HealthCheck(ctx)
	if err == nil {
		t.Fatal("HealthCheck with canceled context: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: git health check:") {
		t.Errorf("error prefix: want 'runner: git health check:', got %q", err.Error())
	}
}

func TestExecGitClient_RestoreClean_DirtyRepo(t *testing.T) {
	dir := initGitRepo(t)

	// Modify an existing tracked file (README.md from initGitRepo)
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("dirty modification\n"), 0644); err != nil {
		t.Fatalf("write dirty README: %v", err)
	}

	client := &ExecGitClient{Dir: dir}
	err := client.RestoreClean(context.Background())
	if err != nil {
		t.Fatalf("RestoreClean on dirty repo: want nil, got %v", err)
	}

	// Verify file is restored to committed content (TrimSpace for CRLF portability)
	content, readErr := os.ReadFile(readmePath)
	if readErr != nil {
		t.Fatalf("read README after RestoreClean: %v", readErr)
	}
	if strings.TrimSpace(string(content)) != "init" {
		t.Errorf("README content after RestoreClean: got %q, want %q", strings.TrimSpace(string(content)), "init")
	}

	// Verify HealthCheck now returns nil
	if hcErr := client.HealthCheck(context.Background()); hcErr != nil {
		t.Errorf("HealthCheck after RestoreClean: want nil, got %v", hcErr)
	}
}

func TestExecGitClient_RestoreClean_CleanRepo(t *testing.T) {
	dir := initGitRepo(t)

	client := &ExecGitClient{Dir: dir}
	err := client.RestoreClean(context.Background())
	if err != nil {
		t.Fatalf("RestoreClean on clean repo: want nil, got %v", err)
	}
}

func TestExecGitClient_RestoreClean_NotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir() // No git init

	client := &ExecGitClient{Dir: dir}
	err := client.RestoreClean(context.Background())
	if err == nil {
		t.Fatal("RestoreClean on non-repo: want error, got nil")
	}
	if !strings.Contains(err.Error(), "runner: git restore clean:") {
		t.Errorf("error prefix: want 'runner: git restore clean:', got %q", err.Error())
	}
}

func TestExecGitClient_HeadCommit_EmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()

	// git init but no commits
	runGit(t, dir, "init")

	client := &ExecGitClient{Dir: dir}
	sha, err := client.HeadCommit(context.Background())
	if err == nil {
		t.Fatal("HeadCommit on empty repo: want error, got nil")
	}
	if sha != "" {
		t.Errorf("HeadCommit SHA on error: want empty, got %q", sha)
	}
	if !strings.Contains(err.Error(), "runner: git head commit:") {
		t.Errorf("error prefix: want 'runner: git head commit:', got %q", err.Error())
	}
}

// TestExecGitClient_DiffStats_HappyPath verifies diff stats for a commit with known changes.
func TestExecGitClient_DiffStats_HappyPath(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}
	ctx := context.Background()

	// Get SHA before changes
	before, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit before: %v", err)
	}

	// Create files in two directories
	subDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "lib.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write lib.go: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add files")

	after, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit after: %v", err)
	}

	stats, err := client.DiffStats(ctx, before, after)
	if err != nil {
		t.Fatalf("DiffStats: %v", err)
	}
	if stats.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", stats.FilesChanged)
	}
	if stats.Insertions < 3 {
		t.Errorf("Insertions = %d, want >= 3", stats.Insertions)
	}
	if stats.Deletions != 0 {
		t.Errorf("Deletions = %d, want 0", stats.Deletions)
	}
	if len(stats.Packages) != 2 {
		t.Fatalf("Packages len = %d, want 2", len(stats.Packages))
	}
	// Packages should be sorted: "." and "pkg"
	if stats.Packages[0] != "." {
		t.Errorf("Packages[0] = %q, want %q", stats.Packages[0], ".")
	}
	if stats.Packages[1] != "pkg" {
		t.Errorf("Packages[1] = %q, want %q", stats.Packages[1], "pkg")
	}
}

// TestExecGitClient_DiffStats_IdenticalSHAs verifies early return for same before/after (AC3).
func TestExecGitClient_DiffStats_IdenticalSHAs(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}
	ctx := context.Background()

	sha, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}

	stats, err := client.DiffStats(ctx, sha, sha)
	if err != nil {
		t.Fatalf("DiffStats identical: %v", err)
	}
	if stats.FilesChanged != 0 {
		t.Errorf("FilesChanged = %d, want 0", stats.FilesChanged)
	}
	if stats.Insertions != 0 {
		t.Errorf("Insertions = %d, want 0", stats.Insertions)
	}
	if stats.Deletions != 0 {
		t.Errorf("Deletions = %d, want 0", stats.Deletions)
	}
	if len(stats.Packages) != 0 {
		t.Errorf("Packages = %v, want empty", stats.Packages)
	}
}

// TestExecGitClient_DiffStats_EmptyParams verifies error for empty before/after (AC4).
func TestExecGitClient_DiffStats_EmptyParams(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}
	ctx := context.Background()

	cases := []struct {
		name          string
		before, after string
	}{
		{"empty before", "", "abc123"},
		{"empty after", "abc123", ""},
		{"both empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.DiffStats(ctx, tc.before, tc.after)
			if err == nil {
				t.Fatal("DiffStats: expected error for empty params, got nil")
			}
			if !strings.Contains(err.Error(), "runner: diff stats:") {
				t.Errorf("error = %q, want containing %q", err.Error(), "runner: diff stats:")
			}
			if !strings.Contains(err.Error(), "non-empty") {
				t.Errorf("error = %q, want containing %q", err.Error(), "non-empty")
			}
		})
	}
}

// TestExecGitClient_DiffStats_InvalidSHA verifies error wrapping for invalid SHA (AC4).
func TestExecGitClient_DiffStats_InvalidSHA(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}
	ctx := context.Background()

	_, err := client.DiffStats(ctx, "invalid_sha_1", "invalid_sha_2")
	if err == nil {
		t.Fatal("DiffStats: expected error for invalid SHA, got nil")
	}
	if !strings.Contains(err.Error(), "runner: diff stats:") {
		t.Errorf("error = %q, want containing %q", err.Error(), "runner: diff stats:")
	}
}

// TestExecGitClient_DiffStats_DeletionsAndModifications verifies stats for modified/deleted files.
func TestExecGitClient_DiffStats_DeletionsAndModifications(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}
	ctx := context.Background()

	before, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit before: %v", err)
	}

	// Modify README.md (1 deletion, 1 insertion for the change)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("updated\nline two\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "modify readme")

	after, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit after: %v", err)
	}

	stats, err := client.DiffStats(ctx, before, after)
	if err != nil {
		t.Fatalf("DiffStats: %v", err)
	}
	if stats.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", stats.FilesChanged)
	}
	if stats.Insertions < 1 {
		t.Errorf("Insertions = %d, want >= 1", stats.Insertions)
	}
	if stats.Deletions < 1 {
		t.Errorf("Deletions = %d, want >= 1", stats.Deletions)
	}
	if len(stats.Packages) != 1 || stats.Packages[0] != "." {
		t.Errorf("Packages = %v, want [\".\"]", stats.Packages)
	}
}

// TestExecGitClient_DiffStats_BinaryFile verifies binary files counted as changed but 0 insertions/deletions.
func TestExecGitClient_DiffStats_BinaryFile(t *testing.T) {
	dir := initGitRepo(t)
	client := &ExecGitClient{Dir: dir}
	ctx := context.Background()

	before, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit before: %v", err)
	}

	// Create a binary file (random bytes that git detects as binary)
	binData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x89, 0x50, 0x4E, 0x47}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), binData, 0644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add binary")

	after, err := client.HeadCommit(ctx)
	if err != nil {
		t.Fatalf("HeadCommit after: %v", err)
	}

	stats, err := client.DiffStats(ctx, before, after)
	if err != nil {
		t.Fatalf("DiffStats: %v", err)
	}
	if stats.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", stats.FilesChanged)
	}
	if stats.Insertions != 0 {
		t.Errorf("Insertions = %d, want 0 (binary)", stats.Insertions)
	}
	if stats.Deletions != 0 {
		t.Errorf("Deletions = %d, want 0 (binary)", stats.Deletions)
	}
	if len(stats.Packages) != 1 || stats.Packages[0] != "." {
		t.Errorf("Packages = %v, want [\".\"]", stats.Packages)
	}
}
