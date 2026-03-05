# Story 8.3: Backup/Rollback Memories

Status: done

## Story

As a разработчик,
I want чтобы sync создавал backup memories перед обновлением и автоматически восстанавливал при ошибке,
so that мои данные были защищены от порчи sync-сессией.

## Acceptance Criteria

1. **backupMemories copies directory (FR61):** `backupMemories(projectRoot)` копирует `.serena/memories/` → `.serena/memories.bak/`. Предыдущий `.bak/` удаляется перед копированием (clean backup). Возвращает nil при успехе.
2. **backupMemories error handling:** Если `.serena/memories/` не существует → возвращает error: `"runner: serena sync: backup: ..."`. `.serena/memories.bak/` не создаётся.
3. **rollbackMemories restores from backup (FR61):** `rollbackMemories(projectRoot)` восстанавливает `.serena/memories/` из `.serena/memories.bak/`. Содержимое `.serena/memories/` полностью заменяется содержимым `.bak/`. `.bak/` сохраняется (не удаляется rollback-ом).
4. **cleanupBackup removes backup (FR61):** `cleanupBackup(projectRoot)` удаляет `.serena/memories.bak/`. Идемпотентна — нет ошибки при отсутствующем `.bak/`.
5. **validateMemories count check (FR62):** `validateMemories(projectRoot, countBefore)` — если countAfter >= countBefore → nil (valid). Если countAfter < countBefore → error: `"runner: serena sync: memory count decreased: 5 → 4"`.
6. **validateMemories graceful on read error (FR62):** Если `.serena/memories/` unreadable → возвращает nil (skip validation, best effort). Без panic.
7. **countMemoryFiles counts .md files:** `countMemoryFiles(projectRoot)` — считает только `.md` файлы в `.serena/memories/`. Игнорирует поддиректории и файлы с другими расширениями.
8. **NTFS compatibility (NFR27):** `filepath.Walk` для обхода, `os.MkdirAll` для создания директорий, `os.ReadFile` + `os.WriteFile` для копирования (без `os.Link`, без symlinks).

## Tasks / Subtasks

- [x] Task 1: Constants and helper copyDir (AC: #8)
  - [x] 1.1 Define constants `serenaMemoriesDir = ".serena/memories"` and `serenaBackupDir = ".serena/memories.bak"` in `runner/serena.go`
  - [x] 1.2 Implement `copyDir(src, dst string) error` — recursive file copy via `filepath.Walk`, `os.ReadFile`/`os.WriteFile`, `os.MkdirAll` for subdirs

- [x] Task 2: backupMemories function (AC: #1, #2)
  - [x] 2.1 Implement `backupMemories(projectRoot string) error` in `runner/serena.go`
  - [x] 2.2 Remove previous backup via `os.RemoveAll(dst)` before copy
  - [x] 2.3 Verify source exists before copying (error if not)
  - [x] 2.4 Error wrapping: `fmt.Errorf("runner: serena sync: backup: %w", err)`

- [x] Task 3: rollbackMemories function (AC: #3)
  - [x] 3.1 Implement `rollbackMemories(projectRoot string) error` in `runner/serena.go`
  - [x] 3.2 `os.RemoveAll(dst)` before restore (replace current memories entirely)
  - [x] 3.3 Copy from `.bak/` to `memories/` — `.bak/` preserved after rollback

- [x] Task 4: cleanupBackup function (AC: #4)
  - [x] 4.1 Implement `cleanupBackup(projectRoot string)` in `runner/serena.go` (no return value)
  - [x] 4.2 `os.RemoveAll` — idempotent, no error on missing dir

- [x] Task 5: countMemoryFiles and validateMemories (AC: #5, #6, #7)
  - [x] 5.1 Implement `countMemoryFiles(projectRoot string) (int, error)` — `os.ReadDir`, filter `.md` + `!IsDir()`
  - [x] 5.2 Implement `validateMemories(projectRoot string, countBefore int) error`
  - [x] 5.3 On read error → return nil (skip validation, best effort)
  - [x] 5.4 On count decrease → error with exact format: `"runner: serena sync: memory count decreased: %d → %d"`

- [x] Task 6: Tests (AC: #1-#8)
  - [x] 6.1 Test backupMemories: creates exact copy of 5 .md files in `.bak/`
  - [x] 6.2 Test backupMemories: removes previous `.bak/` before copy (clean)
  - [x] 6.3 Test backupMemories: error when `.serena/memories/` doesn't exist
  - [x] 6.4 Test rollbackMemories: restores from `.bak/`, verifies content matches
  - [x] 6.5 Test rollbackMemories: `.bak/` preserved after restore
  - [x] 6.6 Test cleanupBackup: removes `.bak/` directory
  - [x] 6.7 Test cleanupBackup: no error when `.bak/` doesn't exist
  - [x] 6.8 Test countMemoryFiles: counts only .md, not .txt, not dirs
  - [x] 6.9 Test validateMemories: count equal → nil
  - [x] 6.10 Test validateMemories: count increased → nil
  - [x] 6.11 Test validateMemories: count decreased → error with format
  - [x] 6.12 Test validateMemories: read error → nil (graceful skip)
  - [x] 6.13 Test copyDir: recursive copy with subdirectories (NTFS pattern)

## Dev Notes

### Architecture Compliance

- **All functions in `runner/serena.go`.** Constants, copyDir helper, backup/rollback/cleanup/validate/count — same file as existing Serena code.
- **No new packages or imports** beyond `os`, `filepath`, `strings`, `fmt` (already in serena.go).
- **Error wrapping:** `"runner: serena sync: backup: %w"` — consistent with project pattern.
- **Functions are package-level** (not methods on Runner) — they only need projectRoot string.
- **Best effort pattern:** validateMemories returns nil on read error. Same philosophy as FR66 (graceful skip).

### Implementation Patterns

**copyDir helper** — per architecture doc (Decision 4):
```go
func copyDir(src, dst string) error {
    return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        relPath, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, relPath)
        if info.IsDir() {
            return os.MkdirAll(target, info.Mode())
        }
        data, err := os.ReadFile(path)
        if err != nil { return err }
        return os.WriteFile(target, data, info.Mode())
    })
}
```
- **No `os.Link`** — NTFS doesn't support hard links reliably via WSL.
- **No symlinks** — `filepath.Walk` follows symlinks but we skip them (NTFS-safe).
- **File permissions:** Use `info.Mode()` from source file for consistency.

**backupMemories** — verify source exists first:
```go
func backupMemories(projectRoot string) error {
    src := filepath.Join(projectRoot, serenaMemoriesDir)
    if _, err := os.Stat(src); err != nil {
        return fmt.Errorf("runner: serena sync: backup: %w", err)
    }
    dst := filepath.Join(projectRoot, serenaBackupDir)
    os.RemoveAll(dst) // clean previous backup
    if err := copyDir(src, dst); err != nil {
        return fmt.Errorf("runner: serena sync: backup: %w", err)
    }
    return nil
}
```

**countMemoryFiles** — pattern from architecture doc:
```go
func countMemoryFiles(projectRoot string) (int, error) {
    dir := filepath.Join(projectRoot, serenaMemoriesDir)
    entries, err := os.ReadDir(dir)
    if err != nil { return 0, err }
    count := 0
    for _, e := range entries {
        if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") { count++ }
    }
    return count, nil
}
```
Note: Only counts top-level `.md` files, not recursive. Serena memories are flat directory structure.

### Critical Constraints

- **Error format must match AC exactly:** `"runner: serena sync: memory count decreased: 5 → 4"` — note the arrow character `→` (Unicode U+2192), not `->`.
- **cleanupBackup has no return value** — `os.RemoveAll` errors are intentionally swallowed (idempotent cleanup).
- **rollbackMemories preserves `.bak/`** — AC #3 explicitly says ".bak/ preserved (not deleted by rollback)". Only cleanupBackup removes it.
- **validateMemories returns nil on error** — not an error wrapper, literally `return nil`. Best effort.
- **filepath.Join not string concatenation** — per `.claude/rules/code-quality-patterns.md` (Story 6.1 pattern).

### Testing Standards

- **Table-driven** for validateMemories (count scenarios).
- **t.TempDir()** for all file operation tests — each test gets isolated directory.
- **Verify file contents** after backup/rollback — not just file count. `os.ReadFile` on both sides, compare bytes.
- **Test naming:** `TestBackupMemories_CopiesFiles`, `TestBackupMemories_MissingSource`, `TestRollbackMemories_RestoresContent`, `TestValidateMemories_CountDecrease`.
- **Error message assertions:** `strings.Contains(err.Error(), "memory count decreased")` and `strings.Contains(err.Error(), "5")`.
- **NTFS notes:** `os.MkdirAll` on nonexistent root works on WSL — use file-as-directory trick for failure tests if needed (per `.claude/rules/wsl-ntfs.md`).

### Project Structure Notes

- `runner/serena.go` — ALL new code: constants, copyDir, backupMemories, rollbackMemories, cleanupBackup, validateMemories, countMemoryFiles
- `runner/serena_test.go` — NEW test file for backup/rollback/validate tests
- No changes to any other files

### References

- [Source: docs/epics/epic-8-serena-memory-sync-stories.md#Story 8.3] — AC and technical notes
- [Source: docs/prd/serena-memory-sync.md#FR61] — Backup memories before sync
- [Source: docs/prd/serena-memory-sync.md#FR62] — Validate memory count after sync
- [Source: docs/architecture/serena-memory-sync.md#Decision 4] — Backup/Rollback mechanism details
- [Source: runner/serena.go] — Existing Serena code (87 lines, all detection logic)
- [Source: .claude/rules/code-quality-patterns.md] — filepath.Join pattern, silent error swallowing pattern
- [Source: .claude/rules/wsl-ntfs.md] — NTFS compatibility patterns

## Dev Agent Record

### Context Reference

### Agent Model Used
Claude Opus 4.6

### Debug Log References
N/A

### Completion Notes List
- All 6 tasks (13 subtasks) completed
- 7 functions implemented: copyDir, backupMemories, rollbackMemories, cleanupBackup, countMemoryFiles, validateMemories + constants
- 13 test cases covering all ACs: backup copy/clean/error, rollback restore/preserve, cleanup/idempotent, count filter, validate scenarios/read-error, recursive copy
- NTFS-compatible: filepath.Walk + os.ReadFile/WriteFile, no symlinks/hard links
- Full test suite passes: `go test ./...` — 0 failures

### File List
- `runner/serena.go` — Added constants, copyDir, backupMemories, rollbackMemories, cleanupBackup, countMemoryFiles, validateMemories
- `runner/serena_sync_test.go` — NEW: 13 test functions for backup/rollback/validate

### Review Record
- **Reviewer:** Claude Opus 4.6
- **Findings:** 0H / 3M / 2L (5 total)
- **All fixed:** M1 (validateMemories error assertion "5" → "5 → 4"), M2 (backupMemories missing inner error path check + platform-agnostic path assertion), M3 (rollbackMemories missing backup test), L1 (count self-resolved by M3+L2 additions), L2 (countMemoryFiles empty dir test)
