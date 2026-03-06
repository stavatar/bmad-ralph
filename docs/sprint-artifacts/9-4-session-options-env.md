# Story 9.4: Session Options.Env

Status: Ready for Review

## Story

As a разработчик,
I want передавать дополнительные переменные окружения в Claude Code сессию,
so that можно управлять extended thinking через `CLAUDE_CODE_EFFORT_LEVEL`.

## Acceptance Criteria

1. **Options.Env field (FR76):**
   - `session.Options` struct has new field: `Env map[string]string`
   - Field is optional (nil = no extra env vars)

2. **Env passed to subprocess:**
   - When `Options.Env = {"CLAUDE_CODE_EFFORT_LEVEL": "high"}`, `cmd.Env` includes all `os.Environ()` entries AND `CLAUDE_CODE_EFFORT_LEVEL=high`

3. **Nil Env preserves existing behavior:**
   - When `Options.Env = nil`, `cmd.Env = os.Environ()` (existing behavior unchanged)

4. **Multiple env vars:**
   - When `Options.Env = {"KEY1": "val1", "KEY2": "val2"}`, `cmd.Env` includes both `KEY1=val1` AND `KEY2=val2`

5. **Env overrides existing var:**
   - When `os.Environ()` contains `CLAUDE_CODE_EFFORT_LEVEL=low` and `Options.Env = {"CLAUDE_CODE_EFFORT_LEVEL": "high"}`, `cmd.Env` has `CLAUDE_CODE_EFFORT_LEVEL=high` (last wins via append order)

## Tasks / Subtasks

- [x] Task 1: Add `Env map[string]string` field to `Options` struct in `session/session.go` (AC: #1)
  - [x] Add field with doc comment explaining purpose
- [x] Task 2: Implement env merging in `Execute()` function (AC: #2, #3, #5)
  - [x] When `len(opts.Env) > 0`: `cmd.Env = append(os.Environ(), envToSlice(opts.Env)...)`
  - [x] When `opts.Env` is nil: keep existing `cmd.Env = os.Environ()` behavior
- [x] Task 3: Implement `envToSlice` helper function (AC: #4)
  - [x] Convert `map[string]string` to `[]string` with `KEY=VALUE` format
  - [x] Unexported function — internal to session package
- [x] Task 4: Write tests in `session/session_test.go` (AC: #1-#5)
  - [x] Test nil Env → cmd.Env == os.Environ()
  - [x] Test single env var → present in cmd.Env
  - [x] Test multiple env vars → all present
  - [x] Test override existing var → last value wins
  - [x] Test envToSlice function directly

## Dev Notes

### Architecture & Design

- **Single file modification:** `session/session.go` — only this file changes
- **No new dependencies** — uses only `os.Environ()` and string formatting
- **Zero-value safe:** nil `Env` map → no behavior change (guard: `len(opts.Env) > 0`)
- **Package boundary:** session does NOT import config — receives Env from caller (runner)

### Critical Implementation Detail

Current code at `session/session.go:68`:
```go
cmd.Env = os.Environ()
```

This is ALREADY set unconditionally. The change is:
```go
cmd.Env = os.Environ()
if len(opts.Env) > 0 {
    cmd.Env = append(cmd.Env, envToSlice(opts.Env)...)
}
```

Note: `append` to `os.Environ()` means custom vars are added AFTER system vars. Since Go `exec.Cmd` uses the last occurrence of a key when duplicates exist, this achieves "override" semantics for AC #5.

### envToSlice Implementation

```go
func envToSlice(m map[string]string) []string {
    s := make([]string, 0, len(m))
    for k, v := range m {
        s = append(s, k+"="+v)
    }
    return s
}
```

Simple key=value conversion. No sorting needed — order doesn't matter for env vars.

### Testing Strategy

- Tests cannot easily inspect `cmd.Env` without running the command. Two approaches:
  1. **Test envToSlice directly** — unit test for the helper
  2. **Test via buildArgs + self-reexec pattern** — mock Claude binary that prints env vars
  3. **Refactor Execute to extract cmd building** — allows inspection without running

Recommended: test `envToSlice` directly + integration test via TestMain self-reexec that echoes `$CLAUDE_CODE_EFFORT_LEVEL`.

### Existing Test Infrastructure

- `session/session_test.go` exists with TestMain self-reexec pattern for mock Claude
- Tests use `config.ClaudeCommand` override to point at test binary
- `t.TempDir()` for isolation

### Project Structure Notes

- Единственное изменение в пакете session — минимальный blast radius
- Поле Env будет использоваться Story 9.3 (Scope Lock + Effort Escalation) через runner
- Dependency direction сохраняется: runner → session (runner заполняет Env, session использует)

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.4]
- [Source: docs/architecture/ralph-run-robustness.md#session/session.go]
- [Source: docs/prd/ralph-run-robustness.md#FR76]
- [Source: session/session.go:38-48 — current Options struct]
- [Source: session/session.go:63-96 — current Execute function]
- [Source: session/session.go:68 — current cmd.Env = os.Environ()]

## Dev Agent Record

### Context Reference

### Agent Model Used

Claude Opus 4.6

### Debug Log References

### Completion Notes List

- Task 1: Added `Env map[string]string` field to Options struct with doc comment
- Task 2: Implemented env merging in Execute() — `cmd.Env = append(os.Environ(), envToSlice(opts.Env)...)` when len > 0, nil Env preserves existing behavior
- Task 3: Implemented unexported `envToSlice` helper converting map to `KEY=VALUE` slice
- Task 4: 5 test functions: envToSlice unit tests (4 cases: nil/empty/single/multiple), integration tests via self-reexec echo_env helper (single var, multiple vars, nil preserves, override existing)
- All tests pass, no regressions across session/runner/config packages

### Change Log

- 2026-03-06: Implemented Story 9.4 — Options.Env field, env merging, envToSlice helper, comprehensive tests

### File List

- session/session.go (modified)
- session/session_test.go (modified)
