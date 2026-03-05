package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadTestdata reads a file from the testdata directory.
func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to load testdata/%s: %v", name, err)
	}
	return data
}

func TestParseResult_Success(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		exitCode int
		stderr   []byte
		wantSID  string
		wantOut  string
		wantExit int
	}{
		{
			name:     "normal success",
			fixture:  "result_success.json",
			exitCode: 0,
			wantSID:  "abc-123-def-456",
			wantOut:  "Implementation complete. All tests pass.",
			wantExit: 0,
		},
		{
			name:     "non-zero exit with valid JSON",
			fixture:  "result_success.json",
			exitCode: 2,
			wantSID:  "abc-123-def-456",
			wantOut:  "Implementation complete. All tests pass.",
			wantExit: 2,
		},
		{
			name:     "extra fields ignored",
			fixture:  "result_extra_fields.json",
			exitCode: 0,
			wantSID:  "abc-123-def-456",
			wantOut:  "Done.",
			wantExit: 0,
		},
		{
			name:     "is_error true still parses",
			fixture:  "result_is_error.json",
			exitCode: 1,
			wantSID:  "abc-123-def-456",
			wantOut:  "Error: task failed validation",
			wantExit: 1,
		},
		{
			name:     "object format success (Claude CLI 2.x)",
			fixture:  "result_success_object.json",
			exitCode: 0,
			wantSID:  "abc-123-def-456",
			wantOut:  "Implementation complete. All tests pass.",
			wantExit: 0,
		},
		{
			name:     "object format non-zero exit",
			fixture:  "result_success_object.json",
			exitCode: 2,
			wantSID:  "abc-123-def-456",
			wantOut:  "Implementation complete. All tests pass.",
			wantExit: 2,
		},
		{
			name:     "object format is_error true still parses",
			fixture:  "result_is_error_object.json",
			exitCode: 1,
			wantSID:  "abc-123-def-456",
			wantOut:  "Error: task failed validation",
			wantExit: 1,
		},
		{
			name:     "with stderr no contamination",
			fixture:  "result_success.json",
			exitCode: 0,
			stderr:   []byte("warning: something happened"),
			wantSID:  "abc-123-def-456",
			wantOut:  "Implementation complete. All tests pass.",
			wantExit: 0,
		},
	}

	elapsed := 3 * time.Second

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &RawResult{
				Stdout:   loadTestdata(t, tt.fixture),
				Stderr:   tt.stderr,
				ExitCode: tt.exitCode,
			}

			result, err := ParseResult(raw, elapsed)
			if err != nil {
				t.Fatalf("ParseResult() unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("ParseResult() returned nil result")
			}
			if result.SessionID != tt.wantSID {
				t.Errorf("SessionID = %q, want %q", result.SessionID, tt.wantSID)
			}
			if result.Output != tt.wantOut {
				t.Errorf("Output = %q, want %q", result.Output, tt.wantOut)
			}
			if result.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExit)
			}
			if result.Duration != elapsed {
				t.Errorf("Duration = %v, want %v", result.Duration, elapsed)
			}
			// Verify no stderr contamination in Output
			if tt.stderr != nil && strings.Contains(result.Output, string(tt.stderr)) {
				t.Error("Output contains stderr content — parsing contaminated")
			}
		})
	}
}

func TestParseResult_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		raw     *RawResult
		wantErr string
	}{
		{
			name:    "nil RawResult",
			raw:     nil,
			wantErr: "session: parse: nil result",
		},
		{
			name:    "empty output",
			raw:     &RawResult{Stdout: []byte{}},
			wantErr: "session: parse: empty output",
		},
		{
			name:    "whitespace-only output",
			raw:     &RawResult{Stdout: []byte("   \n\t  \n")},
			wantErr: "session: parse: empty output",
		},
		{
			name:    "whitespace-only from golden file",
			raw:     &RawResult{Stdout: loadTestdata(t, "result_empty.json")},
			wantErr: "session: parse: empty output",
		},
		{
			name: "empty JSON array",
			raw:  &RawResult{Stdout: []byte("[]")},

			wantErr: "session: parse: empty JSON array",
		},
		{
			name: "JSON array without result element",
			raw: &RawResult{Stdout: []byte(`[
				{"type":"system","subtype":"init","session_id":"abc-123"},
				{"type":"assistant","message":{"content":[]}}
			]`)},
			wantErr: "session: parse: no result message in JSON array",
		},
		{
			// Array element is a JSON number (not an object): inner Unmarshal into
			// jsonResultMessage fails → continue branch executed → loop ends with
			// no result → "no result message" error.
			name:    "JSON array with non-object element",
			raw:     &RawResult{Stdout: []byte(`[42]`)},
			wantErr: "session: parse: no result message in JSON array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseResult(tt.raw, time.Second)
			if err == nil {
				t.Fatalf("ParseResult() expected error, got nil (result: %+v)", result)
			}
			if result != nil {
				t.Errorf("ParseResult() returned non-nil result on error: %+v", result)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseResult_ObjectFormatNonResultFallback(t *testing.T) {
	// JSON object starting with '{' but type != "result" hits plain-text fallback.
	stdout := []byte(`{"type":"system","session_id":"abc-123","subtype":"init"}`)
	raw := &RawResult{Stdout: stdout, ExitCode: 0}

	result, err := ParseResult(raw, time.Second)
	if err != nil {
		t.Fatalf("ParseResult() unexpected error for non-result object: %v", err)
	}
	if result == nil {
		t.Fatal("ParseResult() returned nil result")
	}
	if result.SessionID != "" {
		t.Errorf("SessionID = %q, want empty (plain-text fallback)", result.SessionID)
	}
	if result.Output != string(stdout) {
		t.Errorf("Output = %q, want raw stdout", result.Output)
	}
}

func TestParseResult_TruncatedJSONFallback(t *testing.T) {
	// Truncated JSON triggers non-JSON fallback (json.Unmarshal fails),
	// which returns a valid SessionResult with nil error.
	// Deviation from subtask 3.2 spec: json.Unmarshal cannot distinguish
	// truncated JSON from non-JSON, so both hit the fallback path.
	raw := &RawResult{Stdout: loadTestdata(t, "result_truncated.json")}

	result, err := ParseResult(raw, time.Second)
	if err != nil {
		t.Fatalf("ParseResult() unexpected error for truncated JSON: %v", err)
	}
	if result == nil {
		t.Fatal("ParseResult() returned nil result for truncated JSON")
	}
	// Truncated JSON hits the non-JSON fallback path
	if result.SessionID != "" {
		t.Errorf("SessionID = %q, want empty (non-JSON fallback)", result.SessionID)
	}
	if result.Output != string(raw.Stdout) {
		t.Errorf("Output = %q, want raw stdout", result.Output)
	}
}

func TestParseResult_NonJSONFallback(t *testing.T) {
	stdout := loadTestdata(t, "result_non_json.txt")
	raw := &RawResult{
		Stdout:   stdout,
		ExitCode: 1,
	}
	elapsed := 7 * time.Second

	result, err := ParseResult(raw, elapsed)
	if err != nil {
		t.Fatalf("ParseResult() non-JSON should not return error, got: %v", err)
	}
	if result == nil {
		t.Fatal("ParseResult() returned nil result for non-JSON")
	}
	if result.Output != string(stdout) {
		t.Errorf("Output = %q, want %q", result.Output, string(stdout))
	}
	if result.SessionID != "" {
		t.Errorf("SessionID = %q, want empty string for non-JSON", result.SessionID)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Duration != elapsed {
		t.Errorf("Duration = %v, want %v", result.Duration, elapsed)
	}
}

func TestParseResult_DurationPassthrough(t *testing.T) {
	// Verify Duration is the measured elapsed parameter, NOT parsed from JSON duration_ms.
	// The golden file has duration_ms=8500 but we pass 5s — must get 5s back.
	raw := &RawResult{
		Stdout: loadTestdata(t, "result_success.json"),
	}
	elapsed := 5 * time.Second

	result, err := ParseResult(raw, elapsed)
	if err != nil {
		t.Fatalf("ParseResult() unexpected error: %v", err)
	}
	if result.Duration != elapsed {
		t.Errorf("Duration = %v, want %v (should be caller-measured, not JSON duration_ms)", result.Duration, elapsed)
	}
}

func TestExecuteAndParse_Integration(t *testing.T) {
	dir := t.TempDir()

	t.Run("json success round-trip", func(t *testing.T) {
		t.Setenv("SESSION_TEST_HELPER", "json_success")

		start := time.Now()
		raw, err := Execute(context.Background(), Options{
			Command: os.Args[0],
			Dir:     dir,
		})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		result, parseErr := ParseResult(raw, elapsed)
		if parseErr != nil {
			t.Fatalf("ParseResult() unexpected error: %v", parseErr)
		}
		if result == nil {
			t.Fatal("ParseResult() returned nil result")
		}
		if result.SessionID != "integ-test-001" {
			t.Errorf("SessionID = %q, want %q", result.SessionID, "integ-test-001")
		}
		if result.Output != "Integration test output." {
			t.Errorf("Output = %q, want %q", result.Output, "Integration test output.")
		}
		if result.ExitCode != 0 {
			t.Errorf("ExitCode = %d, want 0", result.ExitCode)
		}
		if result.Duration <= 0 {
			t.Errorf("Duration = %v, want > 0 (measured)", result.Duration)
		}
	})

	t.Run("resume json round-trip", func(t *testing.T) {
		t.Setenv("SESSION_TEST_HELPER", "resume_json")

		start := time.Now()
		// Resume/MaxTurns/OutputJSON are self-documenting: subprocess routes on
		// SESSION_TEST_HELPER env var, not CLI args. Flag construction is tested
		// by buildArgs unit tests in session_test.go.
		raw, err := Execute(context.Background(), Options{
			Command:    os.Args[0],
			Dir:        dir,
			Resume:     "abc-123",
			MaxTurns:   10,
			OutputJSON: true,
		})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		result, parseErr := ParseResult(raw, elapsed)
		if parseErr != nil {
			t.Fatalf("ParseResult() unexpected error: %v", parseErr)
		}
		if result == nil {
			t.Fatal("ParseResult() returned nil result")
		}
		if result.SessionID != "resume-test-002" {
			t.Errorf("SessionID = %q, want %q", result.SessionID, "resume-test-002")
		}
		if result.Output != "Resumed session output." {
			t.Errorf("Output = %q, want %q", result.Output, "Resumed session output.")
		}
		if result.ExitCode != 0 {
			t.Errorf("ExitCode = %d, want 0", result.ExitCode)
		}
		if result.Duration <= 0 {
			t.Errorf("Duration = %v, want > 0 (measured)", result.Duration)
		}
	})

	t.Run("non-JSON fallback round-trip", func(t *testing.T) {
		t.Setenv("SESSION_TEST_HELPER", "json_non_json")

		start := time.Now()
		raw, err := Execute(context.Background(), Options{
			Command: os.Args[0],
			Dir:     dir,
		})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}

		result, parseErr := ParseResult(raw, elapsed)
		if parseErr != nil {
			t.Fatalf("ParseResult() should not error for non-JSON, got: %v", parseErr)
		}
		if result == nil {
			t.Fatal("ParseResult() returned nil result")
		}
		if result.SessionID != "" {
			t.Errorf("SessionID = %q, want empty for non-JSON", result.SessionID)
		}
		if result.Output != "Error: not authenticated" {
			t.Errorf("Output = %q, want %q", result.Output, "Error: not authenticated")
		}
		if result.Duration <= 0 {
			t.Errorf("Duration = %v, want > 0 (measured)", result.Duration)
		}
	})
}

func TestSessionResult_ZeroValue(t *testing.T) {
	// Zero-value SessionResult should be safe to use with sensible defaults.
	var r SessionResult

	if r.SessionID != "" {
		t.Errorf("zero SessionID = %q, want empty", r.SessionID)
	}
	if r.ExitCode != 0 {
		t.Errorf("zero ExitCode = %d, want 0", r.ExitCode)
	}
	if r.Output != "" {
		t.Errorf("zero Output = %q, want empty", r.Output)
	}
	if r.Duration != 0 {
		t.Errorf("zero Duration = %v, want 0", r.Duration)
	}
	if r.Metrics != nil {
		t.Errorf("zero Metrics = %+v, want nil", r.Metrics)
	}
	if r.Model != "" {
		t.Errorf("zero Model = %q, want empty", r.Model)
	}
}

func TestParseResult_UsageMetrics(t *testing.T) {
	tests := []struct {
		name            string
		json            string
		wantMetrics     bool
		wantInput       int
		wantOutput      int
		wantCacheRead   int
		wantNumTurns    int
		wantCostUSD     float64
		wantModel       string
	}{
		{
			name: "with usage data",
			json: `{"type":"result","session_id":"s1","result":"ok","usage":{"input_tokens":1000,"output_tokens":500,"cache_read_tokens":200},"model":"claude-sonnet-4-20250514","num_turns":3}`,
			wantMetrics:   true,
			wantInput:     1000,
			wantOutput:    500,
			wantCacheRead: 200,
			wantNumTurns:  3,
			wantCostUSD:   0.0,
			wantModel:     "claude-sonnet-4-20250514",
		},
		{
			name:        "without usage data graceful degradation",
			json:        `{"type":"result","session_id":"s2","result":"ok"}`,
			wantMetrics: false,
			wantModel:   "",
		},
		{
			name: "usage with zero tokens",
			json: `{"type":"result","session_id":"s3","result":"ok","usage":{"input_tokens":0,"output_tokens":0,"cache_read_tokens":0},"model":"claude-sonnet-4-20250514","num_turns":0}`,
			wantMetrics:   true,
			wantInput:     0,
			wantOutput:    0,
			wantCacheRead: 0,
			wantNumTurns:  0,
			wantCostUSD:   0.0,
			wantModel:     "claude-sonnet-4-20250514",
		},
		{
			name: "model without usage",
			json: `{"type":"result","session_id":"s4","result":"ok","model":"claude-sonnet-4-20250514"}`,
			wantMetrics: false,
			wantModel:   "claude-sonnet-4-20250514",
		},
		{
			name: "array format with usage",
			json: `[{"type":"result","session_id":"s5","result":"ok","usage":{"input_tokens":2000,"output_tokens":800,"cache_read_tokens":100},"model":"claude-opus-4-20250514","num_turns":5}]`,
			wantMetrics:   true,
			wantInput:     2000,
			wantOutput:    800,
			wantCacheRead: 100,
			wantNumTurns:  5,
			wantCostUSD:   0.0,
			wantModel:     "claude-opus-4-20250514",
		},
		{
			name:        "array format without usage",
			json:        `[{"type":"result","session_id":"s6","result":"ok"}]`,
			wantMetrics: false,
			wantModel:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := &RawResult{Stdout: []byte(tc.json), ExitCode: 0}
			result, err := ParseResult(raw, time.Second)
			if err != nil {
				t.Fatalf("ParseResult() error = %v, want nil", err)
			}

			if tc.wantModel != result.Model {
				t.Errorf("Model = %q, want %q", result.Model, tc.wantModel)
			}

			if tc.wantMetrics {
				if result.Metrics == nil {
					t.Fatalf("Metrics = nil, want non-nil")
				}
				if result.Metrics.InputTokens != tc.wantInput {
					t.Errorf("InputTokens = %d, want %d", result.Metrics.InputTokens, tc.wantInput)
				}
				if result.Metrics.OutputTokens != tc.wantOutput {
					t.Errorf("OutputTokens = %d, want %d", result.Metrics.OutputTokens, tc.wantOutput)
				}
				if result.Metrics.CacheReadTokens != tc.wantCacheRead {
					t.Errorf("CacheReadTokens = %d, want %d", result.Metrics.CacheReadTokens, tc.wantCacheRead)
				}
				if result.Metrics.NumTurns != tc.wantNumTurns {
					t.Errorf("NumTurns = %d, want %d", result.Metrics.NumTurns, tc.wantNumTurns)
				}
				if result.Metrics.CostUSD != tc.wantCostUSD {
					t.Errorf("CostUSD = %f, want %f", result.Metrics.CostUSD, tc.wantCostUSD)
				}
			} else {
				if result.Metrics != nil {
					t.Errorf("Metrics = %+v, want nil", result.Metrics)
				}
			}
		})
	}
}

func TestSessionMetrics_ZeroValue(t *testing.T) {
	var m SessionMetrics
	if m.InputTokens != 0 {
		t.Errorf("zero InputTokens = %d, want 0", m.InputTokens)
	}
	if m.OutputTokens != 0 {
		t.Errorf("zero OutputTokens = %d, want 0", m.OutputTokens)
	}
	if m.CacheReadTokens != 0 {
		t.Errorf("zero CacheReadTokens = %d, want 0", m.CacheReadTokens)
	}
	if m.CostUSD != 0 {
		t.Errorf("zero CostUSD = %f, want 0", m.CostUSD)
	}
	if m.NumTurns != 0 {
		t.Errorf("zero NumTurns = %d, want 0", m.NumTurns)
	}
}
