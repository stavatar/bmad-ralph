# Story 1.2: Error Types & Context Pattern

Status: Done

## Story

As a developer,
I want shared error types, sentinel errors, and context propagation pattern established,
so that all packages use consistent error handling from day one.

## Acceptance Criteria

```gherkin
Given the error types package needs to be established
When error types are defined in config package
Then the following sentinel errors exist:
  | Error          | Message                  | Package |
  | ErrNoTasks     | "no tasks found"         | config  |
  | ErrMaxRetries  | "max retries exceeded"   | config  |
  Note: ErrDirtyTree and ErrDetachedHead defined in runner package (Story 3.3)
  Note: ErrMaxReviewCycles defined in runner package (Story 3.10)

And custom error type ExitCodeError exists with fields:
  | Field    | Type   |
  | Code     | int    |
  | Message  | string |
And ExitCodeError implements error interface
And ExitCodeError is checkable via errors.As

And custom error type GateDecision exists with fields:
  | Field    | Type   |
  | Action   | string |
  | Feedback | string |
And GateDecision implements error interface

And error wrapping follows pattern "package: operation: %w"
And all errors are testable via errors.Is or errors.As
And no panic() exists in any production code path

And context pattern is documented:
  - main() creates root ctx via signal.NotifyContext
  - All functions accept ctx as first parameter
  - No context.TODO() in production code
```

## Tasks / Subtasks

- [x] Task 1: Create sentinel errors in config package (AC: sentinel errors)
  - [x] 1.1 Create `config/errors.go` with package declaration and imports
  - [x] 1.2 Define `var ErrNoTasks = errors.New("no tasks found")`
  - [x] 1.3 Define `var ErrMaxRetries = errors.New("max retries exceeded")`
- [x] Task 2: Create ExitCodeError custom type (AC: ExitCodeError)
  - [x] 2.1 Define `type ExitCodeError struct { Code int; Message string }` in `config/errors.go`
  - [x] 2.2 Implement `func (e *ExitCodeError) Error() string` returning `fmt.Sprintf("exit code %d: %s", e.Code, e.Message)`
- [x] Task 3: Create GateDecision custom type (AC: GateDecision)
  - [x] 3.1 Define `type GateDecision struct { Action string; Feedback string }` in `config/errors.go`
  - [x] 3.2 Implement `func (e *GateDecision) Error() string` returning `fmt.Sprintf("gate: %s", e.Action)`
- [x] Task 4: Write table-driven tests (AC: testable via errors.Is/errors.As)
  - [x] 4.1 Create `config/errors_test.go`
  - [x] 4.2 Test `errors.Is(wrappedErr, ErrNoTasks)` returns true
  - [x] 4.3 Test `errors.Is(wrappedErr, ErrMaxRetries)` returns true
  - [x] 4.4 Test `errors.As` extracts `*ExitCodeError` with correct Code and Message (POINTER wrap)
  - [x] 4.5 Test `errors.As` extracts `*GateDecision` with correct Action and Feedback (POINTER wrap)
  - [x] 4.6 Test error wrapping pattern: `fmt.Errorf("config: load: %w", ErrNoTasks)` unwraps correctly
  - [x] 4.7 Test ExitCodeError.Error() returns expected string format
  - [x] 4.8 Test GateDecision.Error() returns expected string format
  - [x] 4.9 Negative: `errors.Is(ErrNoTasks, ErrMaxRetries)` returns false
  - [x] 4.10 Negative: `errors.As(wrappedSentinel, &exitCodeErr)` returns false (sentinel != custom type)
- [x] Task 5: Verify no panic in production code (AC: no panic)
  - [x] 5.1 Grep `config/*.go` (excluding _test.go) for `panic(` — must return zero results
  - [x] 5.2 Spot-check other packages unchanged from scaffold (no panic introduced)
- [x] Task 6: Validation
  - [x] 6.1 `go build ./...` passes
  - [x] 6.2 `go test ./config/...` passes with all tests green
  - [x] 6.3 `go vet ./...` passes

## Dev Notes

### Exact File to Create: `config/errors.go`

Current state of `config/config.go` is just `package config` (placeholder from Story 1.1). Create a NEW file `config/errors.go` for error types — do NOT modify `config/config.go`.

```go
package config

import (
	"errors"
	"fmt"
)

// Sentinel errors for control flow.
var (
	ErrNoTasks    = errors.New("no tasks found")
	ErrMaxRetries = errors.New("max retries exceeded")
)

// ExitCodeError represents a Claude CLI exit with a specific code.
// Used in cmd/ralph for exit code mapping:
//   0=success, 1=partial, 2=user quit, 3=interrupted, 4=fatal
type ExitCodeError struct {
	Code    int
	Message string
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d: %s", e.Code, e.Message)
}

// GateDecision represents a user decision at a human gate.
// Action values: "approve", "skip", "quit", "retry".
type GateDecision struct {
	Action   string
	Feedback string
}

func (e *GateDecision) Error() string {
	return fmt.Sprintf("gate: %s", e.Action)
}
```

**IMPORTANT:** This is reference code. The developer should use this as the implementation guide, not a blind copy-paste target.

### Error Wrapping Convention (ALL packages MUST follow)

Pattern: `fmt.Errorf("package: operation: %w", err)`

Examples for each package:
- `fmt.Errorf("config: load: %w", err)`
- `fmt.Errorf("runner: execute task %s: %w", id, err)`
- `fmt.Errorf("session: claude: exit %d: %w", code, err)`
- `fmt.Errorf("bridge: convert: %w", err)`
- `fmt.Errorf("gates: prompt: %w", err)`

### Context Propagation Pattern (Convention — NO CODE in this story)

This story DOCUMENTS the pattern. Actual implementation:
- `signal.NotifyContext` in main() — Story 1.13
- `ctx` parameter threading — Story 1.7+ (session.Execute, etc.)

**AC "context pattern is documented" satisfied by:** existing `docs/project-context.md#Subprocess` section + convention below in this story's Dev Notes. No separate deliverable file needed.

Convention to enforce from now on:
1. `main()` creates root `ctx` via `signal.NotifyContext(context.Background(), os.Interrupt)`
2. ALL exported functions accept `ctx context.Context` as FIRST parameter
3. `context.TODO()` FORBIDDEN in production code (only acceptable in tests if needed)
4. ALL subprocess calls use `exec.CommandContext(ctx, ...)`

### Error Types Scope — What Goes WHERE

| Error | Package | Story | Why Here |
|-------|---------|-------|----------|
| `ErrNoTasks` | config | 1.2 (this) | Cross-package sentinel, used by runner scan |
| `ErrMaxRetries` | config | 1.2 (this) | Cross-package sentinel, used by runner loop |
| `ExitCodeError` | config | 1.2 (this) | Cross-package type, mapped in cmd/ralph |
| `GateDecision` | config | 1.2 (this) | Cross-package type, used by runner + gates |
| `ErrDirtyTree` | runner | 3.3 | Runner-specific, git health check |
| `ErrDetachedHead` | runner | 3.3 | Runner-specific, git health check |
| `ErrMaxReviewCycles` | runner | 3.10 | Runner-specific, review loop |

**Why config?** config is the leaf package — all other packages import it. Putting shared errors here avoids circular imports. Runner-specific errors stay in runner (defined in later stories).

### Testing Pattern (MANDATORY)

Use table-driven tests. Go stdlib assertions ONLY — NO testify.

```go
// Test naming convention: Test<Type>_<Method>_<Scenario>
func TestErrNoTasks_Is_WrappedUnwraps(t *testing.T) {
	tests := []struct {
		name string
		err  error
		target error
	}{
		{"ErrNoTasks unwraps", fmt.Errorf("config: scan: %w", ErrNoTasks), ErrNoTasks},
		{"ErrMaxRetries unwraps", fmt.Errorf("runner: loop: %w", ErrMaxRetries), ErrMaxRetries},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.target) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.target)
			}
		})
	}
}

func TestExitCodeError_As_Wrapped(t *testing.T) {
	// CRITICAL: wrap as POINTER (&ExitCodeError), not value
	wrapped := fmt.Errorf("session: claude: %w", &ExitCodeError{Code: 1, Message: "test"})
	var target *ExitCodeError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As failed to extract *ExitCodeError")
	}
	if target.Code != 1 {
		t.Errorf("Code = %d, want 1", target.Code)
	}
	if target.Message != "test" {
		t.Errorf("Message = %q, want %q", target.Message, "test")
	}
}

func TestGateDecision_As_Wrapped(t *testing.T) {
	wrapped := fmt.Errorf("gates: prompt: %w", &GateDecision{Action: "quit", Feedback: "done"})
	var target *GateDecision
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As failed to extract *GateDecision")
	}
	if target.Action != "quit" {
		t.Errorf("Action = %q, want %q", target.Action, "quit")
	}
}
```

### Anti-Patterns (FORBIDDEN)

- `if err.Error() == "no tasks found"` — use `errors.Is(err, ErrNoTasks)`
- `os.Exit(1)` in any package — return error, cmd/ralph maps to exit codes
- `panic("unexpected")` — return error
- `errors.New("no tasks found")` in multiple places — import `config.ErrNoTasks`
- String matching on error messages
- Pointer receiver on sentinel errors (they're `var`, not `type`)
- Adding ANY errors NOT listed in AC (ErrDirtyTree etc. come in later stories)
- Wrapping custom error types as VALUE instead of POINTER:
  - WRONG: `fmt.Errorf("session: %w", ExitCodeError{Code: 1, Message: "fail"})` — errors.As will NOT find it
  - RIGHT: `fmt.Errorf("session: %w", &ExitCodeError{Code: 1, Message: "fail"})` — errors.As works

### Previous Story Intelligence (Story 1.1)

**Learnings:**
- Write tool creates CRLF on Windows NTFS — run `sed -i 's/\r$//' <file>` after creating files
- `.gitattributes` with `* text=auto eol=lf` exists — enforces LF on `git add`
- `config/config.go` currently contains only `package config` — leave it untouched
- `go build ./...` and `go vet ./...` must pass after changes
- Go binary path: `"/mnt/c/Program Files/Go/bin/go.exe"` (Windows Go via WSL)

**Code review found:**
- CRITICAL `.gitignore` bug (fixed): pattern without `/` matched directories
- CRLF issue (fixed): all files converted to LF

### Project Structure Notes

- `config/errors.go` — new file, follows architecture: config is leaf package
- `config/errors_test.go` — co-located test file per testing patterns
- No new directories needed — `config/` already exists from Story 1.1
- No new dependencies — uses only `errors` and `fmt` from stdlib
- Dependency direction preserved: config depends on nothing

### References

- [Source: docs/epics/epic-1-foundation-project-infrastructure-stories.md#Story 1.2]
- [Source: docs/project-context.md#Error Handling]
- [Source: docs/architecture/implementation-patterns-consistency-rules.md#Error Handling Patterns]
- [Source: docs/architecture/core-architectural-decisions.md#External Dependencies]
- [Source: docs/architecture/project-structure-boundaries.md#Complete Project Directory Structure]
- [Source: docs/sprint-artifacts/1-1-project-scaffold.md#Completion Notes]

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

Claude Opus 4.6 (claude-opus-4-6)

### Debug Log References

None — clean implementation, no issues encountered.

### Completion Notes List

- Created `config/errors.go` with sentinel errors (ErrNoTasks, ErrMaxRetries) and custom error types (ExitCodeError, GateDecision)
- All types implement `error` interface with pointer receivers
- Created comprehensive table-driven tests in `config/errors_test.go` covering:
  - errors.Is for sentinel error unwrapping (positive, negative, double-wrapped cases)
  - errors.As for custom type extraction with pointer wrapping (ExitCodeError, GateDecision) — table-driven with multiple scenarios including zero values
  - Error() string format validation for both custom types including zero value behavior
  - Negative tests: sentinel-to-sentinel cross-check, sentinel-to-custom-type extraction
- Verified zero `panic()` calls across entire codebase
- All validation gates passed: `go build`, `go test`, `go vet`
- Context propagation pattern documented in Dev Notes and project-context.md (no code — convention only for this story)
- CRLF fixed on all created files via `sed -i 's/\r$//'`
- `config/config.go` left untouched as required

## Senior Developer Review (AI)

**Review Date:** 2026-02-25
**Review Outcome:** Approve (after fixes)
**Reviewer Model:** Claude Opus 4.6 (same session as dev — ideally use different LLM)

### Action Items

- [x] [M1] Test naming convention: renamed `TestSentinelErrors_Is_WrappedUnwraps` → `TestErrNoTasks_Is_WrappedUnwraps`, `TestSentinelAs_Negative` → `TestErrNoTasks_As_NegativeCrossType`
- [x] [M2] Converted `TestExitCodeError_As_Wrapped` → `TestExitCodeError_As_Extraction` (table-driven, 4 cases), `TestGateDecision_As_Wrapped` → `TestGateDecision_As_Extraction` (table-driven, 4 cases)
- [x] [L1] Added zero value edge case tests for ExitCodeError and GateDecision (both As and Error format)
- [x] [L2] Added double-wrapped error unwrapping tests (ErrNoTasks, ErrMaxRetries)
- [x] [L3] golangci-lint not installed in WSL — noted, CI will cover. No action needed.
- [x] [L4] GateDecision.Error() discards Feedback — per AC specification, documented as design note. No code change.

**Summary:** 6 findings (0 Critical, 2 Medium, 4 Low). All 4 actionable items fixed. Tests expanded from 15 to 22 subtests. Coverage remains 100%.

### Change Log

- 2026-02-25: Implemented error types and tests (Story 1.2 — all tasks complete)
- 2026-02-25: Code review fixes — test naming convention, table-driven conversion, edge case coverage (6 findings addressed)

### File List

- `config/errors.go` (new) — sentinel errors and custom error types
- `config/errors_test.go` (new) — comprehensive table-driven tests
- `docs/sprint-artifacts/sprint-status.yaml` (modified) — story status updated
- `docs/sprint-artifacts/1-2-error-types-and-context-pattern.md` (modified) — story file updated
