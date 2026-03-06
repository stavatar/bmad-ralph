# Story 10.4: EnsureCompactHook

Status: Ready for Review

## Story

As a разработчик,
I want чтобы Ralph автоматически устанавливал PreCompact hook при запуске,
so that compaction events записывались в counter file.

## Acceptance Criteria

### AC1: Create hook script — fresh (FR83)
- `projectRoot` без `.ralph/hooks/` директории
- `EnsureCompactHook(projectRoot)` создаёт `.ralph/hooks/count-compact.sh` с содержимым:
  ```bash
  #!/bin/bash
  [ -n "$RALPH_COMPACT_COUNTER" ] && echo 1 >> "$RALPH_COMPACT_COUNTER"
  ```
- Файл executable (`chmod +x`, mode `0755`)

### AC2: Hook script exists — same content (FR83)
- `.ralph/hooks/count-compact.sh` уже существует с корректным содержимым
- `EnsureCompactHook(projectRoot)` — файл НЕ модифицирован (no unnecessary write)

### AC3: Hook script exists — outdated content (FR83)
- `.ralph/hooks/count-compact.sh` существует с старым/другим содержимым
- `EnsureCompactHook(projectRoot)` — файл перезаписан текущей версией
- Файл executable

### AC4: Create settings.json — fresh (FR83, NFR35)
- `projectRoot` без `.claude/settings.json`
- `EnsureCompactHook(projectRoot)` создаёт `.claude/settings.json` с:
  ```json
  {
    "hooks": {
      "PreCompact": [
        {
          "matcher": "auto",
          "hooks": [
            {
              "type": "command",
              "command": ".ralph/hooks/count-compact.sh"
            }
          ]
        }
      ]
    }
  }
  ```
- Файл форматирован `json.MarshalIndent` с `"  "` indent

### AC5: Additive merge — existing settings.json without PreCompact (FR83, NFR35)
- `.claude/settings.json` с другими настройками: `{"permissions":{"allow":["Read"]}}`
- `EnsureCompactHook(projectRoot)` — PreCompact hook entry добавлена
- Existing "permissions" preserved unchanged

### AC6: Additive merge — hooks key exists without PreCompact (FR83, NFR35)
- `.claude/settings.json` с `{"hooks":{}}`
- PreCompact array создан внутри hooks object
- Ralph's hook entry добавлена

### AC7: Additive merge — PreCompact exists as empty array (FR83, NFR35)
- `.claude/settings.json` с `{"hooks":{"PreCompact":[]}}`
- Ralph's hook entry appended к empty PreCompact array

### AC8: Additive merge — PreCompact exists with other hooks (FR83, NFR35)
- `.claude/settings.json` с PreCompact содержащим пользовательский hook
- Ralph's hook entry appended к PreCompact array
- Пользовательский hook preserved unchanged

### AC9: Idempotent — hook already registered (FR83, NFR35)
- `.claude/settings.json` с PreCompact содержащим Ralph's `count-compact.sh` hook
- `EnsureCompactHook(projectRoot)` — НУЛЕВЫЕ изменения (idempotent)

### AC10: Backup before first modification (FR83)
- `.claude/settings.json` существует, `.claude/settings.json.bak` НЕ существует
- `EnsureCompactHook` модифицирует settings.json
- `.claude/settings.json.bak` создан с оригинальным содержимым

### AC11: Backup already exists (FR83)
- `.claude/settings.json.bak` уже существует
- `EnsureCompactHook` модифицирует settings.json
- `.claude/settings.json.bak` НЕ перезаписан (preserve original backup)

### AC12: Corrupt settings.json — graceful (NFR30)
- `.claude/settings.json` с invalid JSON content
- `EnsureCompactHook(projectRoot)` возвращает error (non-fatal, caller logs as warning)
- settings.json НЕ модифицирован (don't corrupt further)

### AC13: Error return is non-fatal (NFR30)
- Любая ошибка от `EnsureCompactHook`
- Caller (`Runner.Execute`) логирует warning, продолжает execution (compactions=0 fallback)

## Tasks / Subtasks

- [x] Task 1: Hook script management (AC: #1-#3)
  - [x] 1.1 `EnsureCompactHook(projectRoot string) error` в `runner/context.go`
  - [x] 1.2 Создать `.ralph/hooks/` dir с `os.MkdirAll`
  - [x] 1.3 Записать script если не существует или содержимое отличается
  - [x] 1.4 `os.Chmod(path, 0755)` для executable

- [x] Task 2: settings.json additive merge (AC: #4-#9)
  - [x] 2.1 Прочитать или создать `.claude/settings.json`
  - [x] 2.2 `json.Unmarshal` в `map[string]any`
  - [x] 2.3 Навигация: `hooks` → `PreCompact` → `[]any`
  - [x] 2.4 Check: есть ли запись с `command` содержащим `count-compact.sh`
  - [x] 2.5 Если нет → append; если есть → skip
  - [x] 2.6 `json.MarshalIndent` → write back

- [x] Task 3: Backup logic (AC: #10-#11)
  - [x] 3.1 Перед первой модификацией settings.json — создать `.bak` если не существует
  - [x] 3.2 Если `.bak` уже есть — не трогать

- [x] Task 4: Error handling (AC: #12-#13)
  - [x] 4.1 Corrupt JSON → return error (не модифицировать файл)
  - [x] 4.2 Caller responsibility: log warning, continue

- [x] Task 5: Тесты (AC: #1-#13)
  - [x] 5.1 Table-driven tests для script creation/update/skip
  - [x] 5.2 Table-driven tests для settings.json merge (fresh/existing/with hooks/idempotent)
  - [x] 5.3 Backup tests
  - [x] 5.4 Corrupt JSON test
  - [x] 5.5 Все тесты с `t.TempDir()` для изоляции

## Dev Notes

### settings.json navigation
```go
// Навигация по map[string]any:
data := map[string]any{}  // from json.Unmarshal

// hooks → map[string]any
hooks, ok := data["hooks"].(map[string]any)
if !ok {
    hooks = map[string]any{}
    data["hooks"] = hooks
}

// PreCompact → []any
preCompact, ok := hooks["PreCompact"].([]any)
if !ok {
    preCompact = []any{}
}
```

### Idempotency check
Проверить содержит ли любая запись в PreCompact array `command` с `count-compact.sh`:
```go
for _, entry := range preCompact {
    entryMap, ok := entry.(map[string]any)
    if !ok { continue }
    hooksArr, ok := entryMap["hooks"].([]any)
    if !ok { continue }
    for _, h := range hooksArr {
        hMap, ok := h.(map[string]any)
        if !ok { continue }
        cmd, _ := hMap["command"].(string)
        if strings.Contains(cmd, "count-compact.sh") {
            return nil // already registered
        }
    }
}
```

### Hook entry structure
```go
hookEntry := map[string]any{
    "matcher": "auto",
    "hooks": []any{
        map[string]any{
            "type":    "command",
            "command": ".ralph/hooks/count-compact.sh",
        },
    },
}
```

### Script content
Содержимое hook-скрипта — строковая константа:
```go
const compactHookScript = "#!/bin/bash\n[ -n \"$RALPH_COMPACT_COUNTER\" ] && echo 1 >> \"$RALPH_COMPACT_COUNTER\"\n"
```

### File permissions
- `os.Chmod(scriptPath, 0755)` — работает на WSL/NTFS
- `os.WriteFile(settingsPath, data, 0644)` — стандартный permission

### Project Structure Notes

- Файл: `runner/context.go` (дополнение к Story 10.3 — тот же файл)
- Тесты: `runner/context_test.go` (дополнение)
- Импорты: `encoding/json`, `os`, `path/filepath`, `strings`
- НЕ добавляет новых зависимостей
- `.ralph/hooks/` — Ralph's own directory (не `.claude/`)
- `.claude/settings.json` — Claude Code's config (аддитивный merge, never overwrite)

### References

- [Source: docs/prd/context-window-observability.md#FR83] — EnsureCompactHook requirements
- [Source: docs/prd/context-window-observability.md#NFR30] — graceful degradation
- [Source: docs/prd/context-window-observability.md#NFR35] — additive merge requirement
- [Source: docs/architecture/context-window-observability.md#Решение 1] — Hook Setup: EnsureCompactHook
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.4] — AC, technical notes

## Testing Standards

- Table-driven tests с `[]struct{name string; ...}` + `t.Run`
- Go stdlib assertions — без testify
- `t.TempDir()` для КАЖДОГО теста — полная изоляция
- Verify file content через `os.ReadFile` + comparison
- Verify file permissions через `os.Stat().Mode()`
- JSON comparison: unmarshal both → compare fields (не string comparison)
- Error tests: `strings.Contains(err.Error(), "expected")`
- Naming: `TestEnsureCompactHook_FreshScript`, `TestEnsureCompactHook_SettingsMerge`

## Dev Agent Record

### Context Reference

### Agent Model Used
Claude Opus 4.6

### Debug Log References

### Completion Notes List
- EnsureCompactHook in runner/context.go: creates .ralph/hooks/count-compact.sh (idempotent, chmod 0755)
- Settings.json additive merge: navigates hooks→PreCompact→[]any, appends if not present
- Idempotency: checks command containing "count-compact.sh" before appending
- Backup: creates .claude/settings.json.bak before first modification, preserves existing backup
- Corrupt JSON: returns wrapped error, caller logs as warning
- Constants: compactHookScript, compactHookCommand (unexported, internal)
- 13 test cases: 3 script (fresh/same/outdated), 7 settings merge (fresh/existing/hooks/empty/other/idempotent/corrupt), 3 backup
- WSL/NTFS: chmod is no-op (mode always 0666), permission test skips on NTFS
- All 13 ACs covered, all tests pass, zero regressions

### File List
- runner/context.go (modified: added EnsureCompactHook, ensureHookScript, ensureHookSettings, constants)
- runner/context_test.go (modified: added TestEnsureCompactHook_Script, _SettingsMerge, _Backup)
