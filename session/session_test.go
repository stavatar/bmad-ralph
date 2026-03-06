package session

import (
	"bytes"
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
	case "echo_stdin":
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(os.Stdin)
		fmt.Fprint(os.Stdout, buf.String())
	case "echo_env":
		// Print requested env var values, one per line.
		// Caller sets ECHO_ENV_KEYS=KEY1,KEY2 to request specific vars.
		keys := strings.Split(os.Getenv("ECHO_ENV_KEYS"), ",")
		for _, k := range keys {
			fmt.Fprintf(os.Stdout, "%s=%s\n", k, os.Getenv(k))
		}
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

// --- Task 9.15-9.16: AppendSystemPrompt flag tests ---

func TestBuildArgs_AppendSystemPrompt(t *testing.T) {
	content := "Critical rules content"
	got := buildArgs(Options{
		Prompt:                     "test",
		AppendSystemPrompt:         &content,
		DangerouslySkipPermissions: true,
	})

	foundFlag := false
	for i, arg := range got {
		if arg == "--append-system-prompt" {
			foundFlag = true
			if i+1 >= len(got) {
				t.Fatal("--append-system-prompt flag missing value")
			}
			if got[i+1] != content {
				t.Errorf("--append-system-prompt value = %q, want %q", got[i+1], content)
			}
			break
		}
	}
	if !foundFlag {
		t.Errorf("expected --append-system-prompt flag in args: %v", got)
	}
}

func TestBuildArgs_AppendSystemPrompt_Nil(t *testing.T) {
	got := buildArgs(Options{
		Prompt:                     "test",
		AppendSystemPrompt:         nil,
		DangerouslySkipPermissions: true,
	})

	for _, arg := range got {
		if arg == "--append-system-prompt" {
			t.Errorf("--append-system-prompt flag should be absent when nil, got args: %v", got)
		}
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
			name: "resume with prompt both present",
			opts: Options{
				Prompt:                     "extraction prompt",
				Resume:                     "session-456",
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"--resume", "session-456",
				"-p", "extraction prompt",
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
				Prompt:                     "extract insights",
				MaxTurns:                   5,
				Model:                      "claude-sonnet-4-5-20250514",
				OutputJSON:                 true,
				DangerouslySkipPermissions: true,
			},
			want: []string{
				"--resume", "session-789",
				"-p", "extract insights",
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

func TestExecute_PromptViaStdin(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "echo_stdin")
	dir := t.TempDir()

	prompt := strings.Repeat("x", maxPromptArgLen+1)
	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
		Prompt:  prompt,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if got := string(result.Stdout); got != prompt {
		t.Errorf("stdin content = %q, want %q", got, prompt)
	}
}

func TestExecute_EmptyPromptNoStdin(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "echo_stdin")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
		Prompt:  "",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// No stdin written — subprocess reads empty stdin, outputs nothing.
	if got := string(result.Stdout); got != "" {
		t.Errorf("stdout = %q, want empty when no prompt", got)
	}
}

// --- envToSlice unit tests ---

func TestEnvToSlice_AllCases(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want int // expected length
	}{
		{"nil map", nil, 0},
		{"empty map", map[string]string{}, 0},
		{"single entry", map[string]string{"KEY": "val"}, 1},
		{"multiple entries", map[string]string{"A": "1", "B": "2", "C": "3"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envToSlice(tt.m)
			if len(got) != tt.want {
				t.Fatalf("envToSlice() returned %d entries, want %d: %v", len(got), tt.want, got)
			}
			// Verify KEY=VALUE format for non-empty maps.
			for _, entry := range got {
				if !strings.Contains(entry, "=") {
					t.Errorf("entry %q missing '=' separator", entry)
				}
			}
			// Verify all keys present.
			for k, v := range tt.m {
				found := false
				expected := k + "=" + v
				for _, entry := range got {
					if entry == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in result, got %v", expected, got)
				}
			}
		})
	}
}

// --- Execute Env integration tests ---

func TestExecute_EnvSingleVar(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "echo_env")
	t.Setenv("ECHO_ENV_KEYS", "CLAUDE_CODE_EFFORT_LEVEL")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
		Env:     map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "high"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	got := string(result.Stdout)
	if !strings.Contains(got, "CLAUDE_CODE_EFFORT_LEVEL=high") {
		t.Errorf("stdout = %q, want containing %q", got, "CLAUDE_CODE_EFFORT_LEVEL=high")
	}
}

func TestExecute_EnvMultipleVars(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "echo_env")
	t.Setenv("ECHO_ENV_KEYS", "KEY1,KEY2")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
		Env:     map[string]string{"KEY1": "val1", "KEY2": "val2"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	got := string(result.Stdout)
	if !strings.Contains(got, "KEY1=val1") {
		t.Errorf("stdout = %q, want containing %q", got, "KEY1=val1")
	}
	if !strings.Contains(got, "KEY2=val2") {
		t.Errorf("stdout = %q, want containing %q", got, "KEY2=val2")
	}
}

func TestExecute_EnvNilPreservesExisting(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "echo_env")
	t.Setenv("ECHO_ENV_KEYS", "HOME")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
		Env:     nil,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	got := string(result.Stdout)
	// HOME should still be present from os.Environ().
	if !strings.Contains(got, "HOME=") {
		t.Errorf("stdout = %q, want containing HOME= (from os.Environ)", got)
	}
}

func TestExecute_EnvOverridesExisting(t *testing.T) {
	t.Setenv("SESSION_TEST_HELPER", "echo_env")
	t.Setenv("ECHO_ENV_KEYS", "CLAUDE_CODE_EFFORT_LEVEL")
	// Set an existing value that should be overridden.
	t.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "low")
	dir := t.TempDir()

	result, err := Execute(context.Background(), Options{
		Command: os.Args[0],
		Dir:     dir,
		Env:     map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "high"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	got := string(result.Stdout)
	// Last value wins — appended after os.Environ(), so "high" should be the effective value.
	if !strings.Contains(got, "CLAUDE_CODE_EFFORT_LEVEL=high") {
		t.Errorf("stdout = %q, want containing %q (override)", got, "CLAUDE_CODE_EFFORT_LEVEL=high")
	}
}
