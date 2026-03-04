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
}

// jsonResultMessage unmarshals the "result" element from Claude CLI JSON output.
// Only fields we need are mapped — unknown fields are silently ignored by encoding/json.
type jsonResultMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Result    string `json:"result"`
	IsError   bool   `json:"is_error"`
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
			return &SessionResult{
				SessionID: msg.SessionID,
				ExitCode:  raw.ExitCode,
				Output:    msg.Result,
				Duration:  elapsed,
			}, nil
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
				return &SessionResult{
					SessionID: msg.SessionID,
					ExitCode:  raw.ExitCode,
					Output:    msg.Result,
					Duration:  elapsed,
				}, nil
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
