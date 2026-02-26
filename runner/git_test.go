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
