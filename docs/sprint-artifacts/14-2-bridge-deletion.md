# Story 14.2: Bridge package deletion

Status: review

## Story

As a разработчик,
I want удалить весь код `ralph bridge`,
so that убрать 2844 строк мёртвого кода и упростить кодовую базу.

## Acceptance Criteria

1. **Удалены файлы:**
   - `bridge/bridge.go`
   - `bridge/bridge_test.go`
   - `bridge/prompt_test.go`
   - `bridge/prompts/bridge.md`
   - `bridge/testdata/` (вся директория)
   - `cmd/ralph/bridge.go`

2. **Регистрация bridge subcommand удалена** из `cmd/ralph/root.go`

3. **`go build ./...` проходит** без ошибок

4. **`go test ./...` проходит** без ошибок

5. **`go vet ./...`** без предупреждений

6. **Нетто-эффект:** >= -1000 строк кода (NFR19: 0 мёртвого кода)

## Tasks / Subtasks

- [x] Task 1: Удалить bridge файлы (AC: #1)
- [x] Task 2: Удалить регистрацию из main.go init() (AC: #2)
- [x] Task 3: Верификация (AC: #3, #4, #5, #6)
  - [x] `go build ./...`
  - [x] `go test ./...`
  - [x] `go vet ./...`
  - [x] Remaining "bridge" references: only in comment

## Dev Notes

### Архитектурные ограничения

- **Выполнять в ОТДЕЛЬНОМ коммите** после зелёного CI [Source: docs/project-context.md#Bridge Removal Order]
- **Prerequisite:** Epic 11 fully implemented, CI green

### References

- [Source: docs/epics.md#Story 14.2] — полные AC
- [Source: docs/project-context.md#Bridge Removal Order] — порядок удаления

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Deleted bridge/ directory (bridge.go, bridge_test.go, bridge_integration_test.go, format_test.go, prompt_test.go, prompts/bridge.md, testdata/)
- Deleted cmd/ralph/bridge.go
- Removed bridgeCmd from cmd/ralph/main.go init(), added planCmd
- Removed all bridge tests from cmd_test.go (TestBridgeCmd_Usage, TestRunBridge_*, TestStoryFileRe_Matching, TestSplitBySize_Cases)
- Updated TestRootCmd_HasSubcommands: removed "bridge", added "plan"
- go build ./... — OK
- go test ./... -count=1 — all packages pass
- go vet ./... — OK
- Net effect: ~-3207 lines (AC #6 >= -1000 satisfied)

### File List

- bridge/ (deleted): entire package removed
- cmd/ralph/bridge.go (deleted): bridge subcommand
- cmd/ralph/main.go (modified): removed bridgeCmd, added planCmd in init()
- cmd/ralph/cmd_test.go (modified): removed all bridge-related tests, updated subcommand list

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/2M/2L

#### [MEDIUM] M1: Стале комментарии "bridge" в 5 Go-файлах

AC #6 (NFR19: 0 мёртвого кода) — после удаления bridge/ пакета остались stale references:
- `config/constants.go:6` — "Used by bridge (generation)"
- `config/format.go:9` — "Used by bridge (prompt generation)"
- `config/prompt.go:33` — "bridge merge mode"
- `runner/runner.go:129` — "BridgeGoldenFile: bridge output"
- `session/session.go:36` — "runner/bridge"

Fix: обновить комментарии, убрав упоминания bridge.

#### [MEDIUM] M2: Дублированный section comment в cmd_test.go

Строки 324-326 содержат два подряд section comment:
```
// --- runDistill error path tests (bridge tests removed in Story 14.2) ---
// --- runDistill error path tests ---
```
Мусор от удаления bridge тестов. Fix: оставить один comment без скобок.

#### [LOW] L1: AC #2 ссылается на несуществующий root.go

AC #2: "Регистрация bridge subcommand удалена из `cmd/ralph/root.go`". Фактически файл называется `cmd/ralph/main.go`. Minor formulation issue в story definition.

#### [LOW] L2: Task 3 checkbox "only in comment" неточен

Task 3 checklist говорит "Remaining bridge references: only in comment". На деле 5 файлов с stale bridge комментариями (см. M1). Формулировка создаёт впечатление, что это один комментарий.

### Carry-over Fixes Verification

- H1 from 14.1 (bridge.go deleted instead of deprecated): FIXED — bridge/ полностью удалён в scope 14.2
- L1 from 14.1 (bridge package inconsistency): FIXED — bridge/ package удалён

### New Patterns Discovered

0 new patterns.
