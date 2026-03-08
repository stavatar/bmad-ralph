# Story 11.2: session.Options — поле InjectFeedback

Status: Done

## Review Findings (0H/3M/1L)

### MEDIUM
- [M1] Двойной `--append-system-prompt` при одновременном `AppendSystemPrompt` + `InjectFeedback` — неопределённое поведение CLI `[session/session.go:149-155]`
- [M2] Тест `InjectFeedback_Present` не изолирует от `AppendSystemPrompt`, `break` маскирует дубли `[session/session_test.go:109-130]`
- [M3] Тест `InjectFeedback_Empty` ложно-положителен при наличии `AppendSystemPrompt` `[session/session_test.go:133-144]`

### LOW
- [L1] Doc comment на `Options` struct устарел: не упоминает `plan/` как caller `[session/session.go:36]`

## Story

As a система,
I want передавать reviewer feedback в retry-сессию через поле `session.Options`,
so that Claude получает контекст предыдущего review при повторной генерации.

## Acceptance Criteria

1. **Поле `InjectFeedback string` добавлено в `session.Options`** — `session/session.go`

2. **Если `InjectFeedback != ""`** → передаётся через `--append-system-prompt` флаг в Claude CLI args

3. **Если `InjectFeedback == ""`** → флаг `--append-system-prompt` не передаётся (поведение без изменений)

4. **Тест в `session_test.go`:** `InjectFeedback` не пустой → `--append-system-prompt` присутствует в args

5. **Тест в `session_test.go`:** `InjectFeedback` пустой → `--append-system-prompt` отсутствует

6. **`go test ./session/... -count=1` проходит**

## Tasks / Subtasks

- [x] Task 1: Добавить поле `InjectFeedback` в `session.Options` (AC: #1)
  - [x] Добавить `InjectFeedback string` в struct `Options` в `session/session.go`
- [x] Task 2: Реализовать условную передачу флага (AC: #2, #3)
  - [x] В функции построения args (buildArgs или аналог): если `opts.InjectFeedback != ""` → `args = append(args, flagAppendSystemPrompt, opts.InjectFeedback)`
  - [x] Константа `flagAppendSystemPrompt = "--append-system-prompt"` уже определена в `session.go`
- [x] Task 3: Написать тесты (AC: #4, #5, #6)
  - [x] Тест: InjectFeedback задан → `--append-system-prompt` + значение присутствуют в аргументах
  - [x] Тест: InjectFeedback пустой → `--append-system-prompt` отсутствует в аргументах
  - [x] Использовать self-reexec mock pattern (существующий в проекте)

## Dev Notes

### Архитектурные ограничения

- **`session` = независимый пакет:** не зависит от `runner`, `gates`, `bridge`, `plan` [Source: docs/project-context.md#Dependency Direction]
- **Options struct:** `session.Options` содержит 9 полей, принимается `session.Execute(ctx, opts)` [Source: docs/project-context.md#Package Entry Points]
- **Subprocess через `exec.CommandContext(ctx)`:** без `exec.Command`, без `context.TODO()` [Source: docs/project-context.md#Subprocess]

### Существующий код

- `session/session.go` — `Options` struct (9 полей), `Execute()`, `flagAppendSystemPrompt` константа [Source: docs/epics.md#Epic 11 scaffold]
- `flagAppendSystemPrompt = "--append-system-prompt"` — УЖЕ определён, не нужно создавать заново
- Тесты в `session/session_test.go` используют self-reexec mock pattern через `config.ClaudeCommand` [Source: docs/project-context.md#Testing]

### Паттерн добавления

Найти в `session/session.go` функцию построения CLI аргументов для Claude (buildArgs или аналог). Добавить:
```go
if opts.InjectFeedback != "" {
    args = append(args, flagAppendSystemPrompt, opts.InjectFeedback)
}
```

### Обратная совместимость

- Пустая строка = default → поведение без изменений для `runner/` и `bridge/`
- Поле добавляется в struct без breaking change (Go backward compatible)
- Существующие тесты не должны сломаться — новое поле zero value = нет эффекта

### Тестирование

- Self-reexec mock pattern: TestMain + env var + `os.Args[0]` [Source: .claude/rules/wsl-ntfs.md#Go Binary]
- Table-driven, Go stdlib assertions [Source: CLAUDE.md#Testing Core Rules]
- Test naming: `TestOptions_InjectFeedback_Present`, `TestOptions_InjectFeedback_Empty` [Source: CLAUDE.md#Naming Conventions]

### Project Structure Notes

- `session/session.go` — существующий файл, модификация struct + args builder
- `session/session_test.go` — существующий файл, добавление тестов
- Нет новых файлов или пакетов

### References

- [Source: docs/epics.md#Story 11.2] — полные AC и технические заметки
- [Source: docs/project-context.md#Subprocess] — правила subprocess, exec.CommandContext
- [Source: docs/project-context.md#Dependency Direction] — session как независимый пакет
- [Source: session/session.go] — Options struct, flagAppendSystemPrompt константа
- [Source: CLAUDE.md#Architecture Rules] — dependency direction

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added `InjectFeedback string` field to `session.Options` in `session/session.go`
- Added conditional `--append-system-prompt` flag in `buildArgs()` when `InjectFeedback != ""`
- Added `TestBuildArgs_InjectFeedback_Present` — verifies flag + value present
- Added `TestBuildArgs_InjectFeedback_Empty` — verifies flag absent when empty
- Full regression: `go test ./... -count=1` — all packages pass, 0 regressions

### File List

- session/session.go (modified): added InjectFeedback field to Options, added handling in buildArgs()
- session/session_test.go (modified): added 2 test functions for InjectFeedback
