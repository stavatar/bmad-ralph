package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SessionResult contains parsed output from a Claude CLI session.
// Created by ParseResult from a RawResult after Execute completes.
type SessionResult struct {
	SessionID string        // From JSON "session_id" field
	ExitCode  int           // From process exit code (RawResult.ExitCode)
	Output    string        // From JSON "result" field (or raw stdout for non-JSON)
	Duration  time.Duration // Measured wall-clock time by caller
	Metrics   *SessionMetrics // Token usage metrics; nil when usage data absent
	Model     string          // Model name from JSON; empty when absent
	Truncated bool            // True when is_error=true with exit_code=0 (e.g. error_max_turns)
	Subtype   string          // Error subtype from JSON (e.g. "error_max_turns"); empty when absent
}

// jsonResultMessage unmarshals the "result" element from Claude CLI JSON output.
// Only fields we need are mapped — unknown fields are silently ignored by encoding/json.

// SessionMetrics holds token usage and turn count extracted from Claude Code JSON output.
// CostUSD is populated from Claude CLI's total_cost_usd when available; zero otherwise.
type SessionMetrics struct {
	InputTokens          int     `json:"input_tokens"`
	OutputTokens         int     `json:"output_tokens"`
	CacheReadTokens      int     `json:"cache_read_input_tokens"`
	CacheCreationTokens  int     `json:"cache_creation_input_tokens"`
	CostUSD              float64 `json:"cost_usd"`
	NumTurns             int     `json:"num_turns"`
}

// usageData maps the "usage" object in Claude Code JSON output.
type usageData struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
}

type jsonResultMessage struct {
	Type         string     `json:"type"`
	Subtype      string     `json:"subtype"`
	SessionID    string     `json:"session_id"`
	Result       string     `json:"result"`
	IsError      bool       `json:"is_error"`
	Usage        *usageData `json:"usage"`
	Model        string     `json:"model"`
	NumTurns     int        `json:"num_turns"`
	TotalCostUSD float64    `json:"total_cost_usd"`
}

// ParseResult transforms raw Claude CLI output into a structured SessionResult.
// The elapsed parameter is the measured wall-clock duration of the session.
//
// Supports two Claude CLI output formats:
//   - Array format (Claude CLI <2.x): [{"type":"result",...}, ...]
//   - Object format (Claude CLI 2.x+): {"type":"result",...}
//
// Empty stdout is an error. Non-JSON or malformed stdout (including truncated JSON)
// is a fallback mode — returns valid SessionResult with raw text as Output,
// empty SessionID, and nil error.
func ParseResult(raw *RawResult, elapsed time.Duration) (*SessionResult, error) {
	if raw == nil {
		return nil, fmt.Errorf("session: parse: nil result")
	}

	if len(raw.Stdout) == 0 || len(strings.TrimSpace(string(raw.Stdout))) == 0 {
		return nil, fmt.Errorf("session: parse: empty output")
	}

	trimmed := strings.TrimSpace(string(raw.Stdout))

	// Object format: Claude CLI 2.x outputs a single JSON object {"type":"result",...}
	if strings.HasPrefix(trimmed, "{") {
		var msg jsonResultMessage
		if err := json.Unmarshal(raw.Stdout, &msg); err == nil && msg.Type == "result" {
			return resultFromMessage(&msg, raw.ExitCode, elapsed), nil
		}
		// Valid JSON object but not a result message — fall through to plain-text fallback
		return &SessionResult{
			Output:   string(raw.Stdout),
			ExitCode: raw.ExitCode,
			Duration: elapsed,
		}, nil
	}

	// Array format: Claude CLI <2.x outputs [{"type":"result",...}, ...]
	if strings.HasPrefix(trimmed, "[") {
		var messages []json.RawMessage
		if err := json.Unmarshal(raw.Stdout, &messages); err != nil {
			// Malformed array — non-JSON fallback
			return &SessionResult{
				Output:   string(raw.Stdout),
				ExitCode: raw.ExitCode,
				Duration: elapsed,
			}, nil
		}

		if len(messages) == 0 {
			return nil, fmt.Errorf("session: parse: empty JSON array")
		}

		// Find last element with type == "result" by iterating from end
		for i := len(messages) - 1; i >= 0; i-- {
			var msg jsonResultMessage
			if err := json.Unmarshal(messages[i], &msg); err != nil {
				continue
			}
			if msg.Type == "result" {
				return resultFromMessage(&msg, raw.ExitCode, elapsed), nil
			}
		}

		return nil, fmt.Errorf("session: parse: no result message in JSON array")
	}

	// Non-JSON fallback: Claude may output plain text
	return &SessionResult{
		Output:   string(raw.Stdout),
		ExitCode: raw.ExitCode,
		Duration: elapsed,
	}, nil
}

// resultFromMessage builds a SessionResult from a parsed jsonResultMessage.
// Populates Metrics only when usage data is present (graceful degradation).
// Sets Truncated=true when is_error=true and exit_code=0 (e.g. error_max_turns).
func resultFromMessage(msg *jsonResultMessage, exitCode int, elapsed time.Duration) *SessionResult {
	r := &SessionResult{
		SessionID: msg.SessionID,
		ExitCode:  exitCode,
		Output:    msg.Result,
		Duration:  elapsed,
		Model:     msg.Model,
		Subtype:   msg.Subtype,
		Truncated: msg.IsError && exitCode == 0,
	}
	if msg.Usage != nil {
		r.Metrics = &SessionMetrics{
			InputTokens:         msg.Usage.InputTokens,
			OutputTokens:        msg.Usage.OutputTokens,
			CacheReadTokens:     msg.Usage.CacheReadTokens,
			CacheCreationTokens: msg.Usage.CacheCreationTokens,
			CostUSD:             msg.TotalCostUSD,
			NumTurns:            msg.NumTurns,
		}
	}
	return r
}
