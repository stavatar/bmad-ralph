package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/runner"
)

// --- Shared test data ---

const threeOpenTasks = `# Sprint Tasks

## Epic 1: Foundation

- [ ] Task one
- [ ] Task two
- [ ] Task three
`

const allDoneTasks = `# Sprint Tasks

- [x] Done one
- [x] Done two
`

const noMarkersTasks = `# Sprint Tasks

No tasks here
`

// --- Shared ReviewFn helpers (DRY: used across 9+ tests) ---

// cleanReviewFn returns a clean review result with no error.
var cleanReviewFn runner.ReviewFunc = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
	return runner.ReviewResult{Clean: true}, nil
}

// fatalReviewFn returns a ReviewFunc that fails the test if called.
func fatalReviewFn(t *testing.T) runner.ReviewFunc {
	t.Helper()
	return func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		t.Fatal("ReviewFn should not be called")
		return runner.ReviewResult{}, nil
	}
}

// --- Shared test helpers ---

// writeTasksFile writes inline task content to sprint-tasks.md in dir.
func writeTasksFile(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write tasks file: %v", err)
	}
	return p
}

// headCommitPairs generates a flat HeadCommits slice from before/after SHA pairs.
// Each pair represents one iteration: [before1, after1, before2, after2, ...].
func headCommitPairs(pairs ...[2]string) []string {
	result := make([]string, 0, len(pairs)*2)
	for _, p := range pairs {
		result = append(result, p[0], p[1])
	}
	return result
}

// copyFixtureToDir copies a testdata fixture into tmpDir and returns the destination path.
func copyFixtureToDir(t *testing.T, tmpDir, fixture string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	dst := filepath.Join(tmpDir, fixture)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write fixture %s: %v", fixture, err)
	}
	return dst
}

// assertArgsContainFlag verifies a flag exists in args.
func assertArgsContainFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			return
		}
	}
	t.Errorf("args: want flag %q, not found in %v", flag, args)
}

// assertArgsContainFlagValue verifies a flag with specific value exists in args.
func assertArgsContainFlagValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Errorf("args: flag %q found at end of args, no value follows", flag)
				return
			}
			if args[i+1] != value {
				t.Errorf("args: flag %q value = %q, want %q", flag, args[i+1], value)
			}
			return
		}
	}
	t.Errorf("args: want flag %q with value %q, flag not found in %v", flag, value, args)
}

// assertArgsFlagAbsent verifies a flag is NOT in args.
func assertArgsFlagAbsent(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			t.Errorf("args: want flag %q absent, but found in %v", flag, args)
			return
		}
	}
}

// argValueAfterFlag returns the value following the given flag in args, or "" if not found.
func argValueAfterFlag(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
