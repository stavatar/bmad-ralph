package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestMain intercepts the test binary execution. When SESSION_TEST_HELPER is set,
// the binary acts as a test helper subprocess instead of running tests.
// This is the standard Go pattern for testing exec.Command (used by Go stdlib itself).
func TestMain(m *testing.M) {
	if scenario := os.Getenv("SESSION_TEST_HELPER"); scenario != "" {
		runTestHelper(scenario)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runTestHelper handles subprocess scenarios for Execute tests.
func runTestHelper(scenario string) {
	switch scenario {
	case "success":
		fmt.Fprint(os.Stdout, "hello stdout")
		fmt.Fprint(os.Stderr, "hello stderr")
	case "exit2":
		os.Exit(2)
	case "separate":
		fmt.Fprint(os.Stdout, "ONLY_STDOUT")
		fmt.Fprint(os.Stderr, "ONLY_STDERR")
	case "pwd":
		dir, _ := os.Getwd()
		fmt.Fprint(os.Stdout, dir)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "json_success":
		fmt.Fprint(os.Stdout, `[{"type":"system","subtype":"init","session_id":"integ-test-001","tools":[],"model":"claude-sonnet-4-5-20250514"},{"type":"result","subtype":"success","session_id":"integ-test-001","result":"Integration test output.","is_error":false,"duration_ms":1000,"num_turns":1}]`)
	case "resume_json":
		fmt.Fprint(os.Stdout, `[{"type":"system","subtype":"init","session_id":"resume-test-002","tools":[],"model":"claude-sonnet-4-5-20250514"},{"type":"result","subtype":"success","session_id":"resume-test-002","result":"Resumed session output.","is_error":false,"duration_ms":500,"num_turns":1}]`)
	case "json_non_json":
		fmt.Fprint(os.Stdout, "Error: not authenticated")
	default:
		fmt.Fprintf(os.Stderr, "unknown test helper scenario: %s\n", scenario)
		os.Exit(1)
	}
}

func TestBuildArgs_BasicPrompt(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "prompt only with skip permissions",
			opts: Options{
				Prompt:                     "hello world",
				DangerouslySkipPermissions: true,
			},
			want: []string{"-p", "hello world", "--dangerously-skip-permissions"},
		},
		{
			name: "prompt with max turns",
			opts: Options{
				Prompt:                     "test",
				MaxTurns:                   5,
				DangerouslySkipPermissions: true,
			},
			want: []string{"-p", "test", "--max-turns", "5", "--dangerously-skip-permissions"},
		},
		{
			name: "prompt with model",
			opts: Options{
				Prompt:                     "test",
				Model:                      "claude-sonnet-4-5-20250514",
				DangerouslySkipPermissions: true,
			},
			want: []string{"-p", "test", "--model", "claude-sonnet-4-5-20250514", "--dangerously-skip-permissions"},
		},
		{
			name: "prompt with output json",
			opts: Options{
				Prompt:                     "test",
				OutputJSON:                 true,
				DangerouslySkipPermissions: true,
			},
			want: []string{"-p", "test", "--output-format", "json", "--dangerously-skip-permissions"},
		},
		{
			name: "all fields set",
			opts: Options{
				Prompt:                     "do something",
				MaxTurns:                   10,
				Model:                      "claude-opus-4-20250514",
				OutputJSON:                 true,
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"-p", "do something",
				"--max-turns", "10",
				"--model", "claude-opus-4-20250514",
				"--output-format", "json",
				"--dangerously-skip-permissions",
			},
		},
		{
			name: "resume mode no prompt",
			opts: Options{
				Resume:                     "session-123",
				MaxTurns:                   10,
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"--resume", "session-123",
				"--max-turns", "10",
				"--dangerously-skip-permissions",
			},
		},
		{
			name: "resume overrides prompt",
			opts: Options{
				Prompt:                     "this should be ignored",
				Resume:                     "session-456",
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"--resume", "session-456",
				"--dangerously-skip-permissions",
			},
		},
		{
			name: "resume with max turns and output json",
			opts: Options{
				Resume:                     "abc-123",
				MaxTurns:                   10,
				OutputJSON:                 true,
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"--resume", "abc-123",
				"--max-turns", "10",
				"--output-format", "json",
				"--dangerously-skip-permissions",
			},
		},
		{
			name: "resume all fields set",
			opts: Options{
				Resume:                     "session-789",
				Prompt:                     "ignored",
				MaxTurns:                   5,
				Model:                      "claude-sonnet-4-5-20250514",
				OutputJSON:                 true,
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"--resume", "session-789",
				"--max-turns", "5",
				"--model", "claude-sonnet-4-5-20250514",
				"--output-format", "json",
				"--dangerously-skip-permissions",
			},
		},
		{
			name: "empty resume with prompt",
			opts: Options{
				Resume:                     "",
				Prompt:                     "test",
				DangerouslySkipPermissions: true,
			},
			want: []string{"-p", "test", "--dangerously-skip-permissions"},
		},
		{
			name: "empty prompt no resume",
			opts: Options{
				DangerouslySkipPermissions: true,
			},
			want: []string{"--dangerously-skip-permissions"},
		},
		{
			name: "skip permissions false",
			opts: Options{
				Prompt: "test",
			},
			want: []string{"-p", "test"},
		},
		{
			name: "zero value options",
			opts: Options{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArgs(tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("buildArgs() returned %d args, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildArgs()[%d] = %q, want %q\nfull got:  %v\nfull want: %v", i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

func TestExecute_Success(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "success")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if got := string(result.Stdout); got != "hello stdout" {
		t.Errorf("Stdout = %q, want %q", got, "hello stdout")
	}
	if got := string(result.Stderr); got != "hello stderr" {
		t.Errorf("Stderr = %q, want %q", got, "hello stderr")
	}
}

func TestExecute_NonZeroExit(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "exit2")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for non-zero exit, got nil")
	}
	if result.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", result.ExitCode)
	}
	if !strings.Contains(err.Error(), "session: claude: exit 2:") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "session: claude: exit 2:")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Error("error should wrap *exec.ExitError")
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "sleep")
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := Execute(ctx, Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err == nil {
		t.Fatal("Execute() expected error for context cancellation, got nil")
	}
	if !strings.Contains(err.Error(), "session: claude:") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "session: claude:")
	}
}

func TestExecute_CommandNotFound(t *testing.T) {
	_, err := Execute(context.Background(), Options{
		Command: "/nonexistent/command/that/does/not/exist",
		Dir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("Execute() expected error for missing command, got nil")
	}
	if !strings.Contains(err.Error(), "session: claude:") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "session: claude:")
	}
}

func TestExecute_SeparateStdoutStderr(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "separate")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	stdout := string(result.Stdout)
	stderr := string(result.Stderr)

	if stdout != "ONLY_STDOUT" {
		t.Errorf("Stdout = %q, want %q", stdout, "ONLY_STDOUT")
	}
	if stderr != "ONLY_STDERR" {
		t.Errorf("Stderr = %q, want %q", stderr, "ONLY_STDERR")
	}
	if strings.Contains(stdout, "ONLY_STDERR") {
		t.Error("stdout buffer contains stderr content — buffers are mixed")
	}
	if strings.Contains(stderr, "ONLY_STDOUT") {
		t.Error("stderr buffer contains stdout content — buffers are mixed")
	}
}

func TestExecute_WorkingDir(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "pwd")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	got := strings.TrimSpace(string(result.Stdout))
	// Compare via os.SameFile to handle Windows 8.3 short name differences
	gotInfo, statErr := os.Stat(got)
	if statErr != nil {
		t.Fatalf("cannot stat returned path %q: %v", got, statErr)
	}
	wantInfo, statErr := os.Stat(dir)
	if statErr != nil {
		t.Fatalf("cannot stat expected path %q: %v", dir, statErr)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Errorf("working dir = %q, want %q", got, dir)
	}
}
