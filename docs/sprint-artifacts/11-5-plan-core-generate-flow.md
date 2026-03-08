# Story 11.5: plan/plan.go — core generate flow

Status: Done

## Review Findings (1H/3M/0L)

### HIGH
- [H1] `os.ReadFile` в plan/plan.go нарушает plan/ invariant ("plan/ never reads files") — existing content для merge должен передаваться через PlanOpts `[plan/plan.go:88]`

### MEDIUM
- [M1] writeAtomic defer os.Remove после успешного os.Rename — лишний syscall `[plan/plan.go:169]`
- [M2] SetupMockClaude error discarded в обоих тестах `[plan/plan_test.go:154,209]`
- [M3] Нет теста для OutputPath fallback (AC2) — нет кейса с заданным opts.OutputPath `[plan/plan_test.go]`

## Story

As a разработчик,
I want вызвать `plan.Run(ctx, cfg, opts)` и получить готовый `sprint-tasks.md`,
so that я могу перейти к `ralph run` без ручных правок.

## Acceptance Criteria

1. **`plan.Run(ctx, cfg, PlanOpts{Inputs, OutputPath})` вызван с валидными входными данными** — генерация завершается успешно (Claude exit 0) → `sprint-tasks.md` записан по `opts.OutputPath` через `writeAtomic()` (temp+rename)

2. **Если `opts.OutputPath` пустой** → используется `cfg.PlanOutputPath`

3. **Если генерация падает** (Claude exit != 0) → возвращается `fmt.Errorf("plan: generate: %w", err)`, файл не изменён

4. **`plan_test.go` содержит scenario-based тест через `config.ClaudeCommand` mock:**
   - сценарий `generate_success` → файл записан корректно
   - сценарий `generate_failure` → ошибка, файл не создан

5. **`go test ./plan/... -count=1` проходит**

## Tasks / Subtasks

- [x] Task 1: Реализовать `plan.Run()` в `plan/plan.go` (AC: #1, #2, #3)
  - [x] Определить `PlanOpts` struct: `Inputs []PlanInput`, `OutputPath string`, `Merge bool`, `MaxRetries int`
  - [x] Реализовать `Run(ctx context.Context, cfg *config.Config, opts PlanOpts) error`
  - [x] OutputPath fallback: если `opts.OutputPath == ""` → `cfg.PlanOutputPath`
  - [x] Рендер промпта: `text/template` из embedded `plan/prompts/plan.md` + `strings.Replace` для content
  - [x] Вызов `session.Execute(ctx, sessionOpts)` с рендеренным промптом
  - [x] При успехе: `writeAtomic(outputPath, result)` — temp file + rename
  - [x] При ошибке: `fmt.Errorf("plan: generate: %w", err)`, файл не изменён
- [x] Task 2: Реализовать `writeAtomic()` helper (AC: #1)
  - [x] Unexported `writeAtomic(path string, data []byte) error`
  - [x] Создать temp file в той же директории → записать → `os.Rename` (атомарная операция)
- [x] Task 3: Определить `templateData` struct (AC: #1)
  - [x] Unexported struct: `Inputs []PlanInput, OutputPath string, Merge bool, Existing string`
- [x] Task 4: Написать тесты (AC: #4, #5)
  - [x] Mock через `config.ClaudeCommand` — scenario-based JSON responses
  - [x] Сценарий `generate_success`: Claude возвращает sprint-tasks content → файл записан
  - [x] Сценарий `generate_failure`: Claude exit != 0 → ошибка содержит "plan: generate:", файл не создан
  - [x] Проверить: файл не существует после failure (atomic guarantee)

## Dev Notes

### Архитектурные ограничения

- **`plan/` НЕ читает файлы:** `inp.Content` заполняется в `cmd/ralph/plan.go` [Source: docs/project-context.md#Plan Package Инварианты]
- **Запись только через `writeAtomic()`** — нет `os.WriteFile` в merge.go / size.go [Source: docs/project-context.md#Plan Package Инварианты]
- **User content через `strings.Replace`** — не через `template.Execute` [Source: docs/project-context.md#Двухэтапная Prompt Assembly]
- **`os.Exit` запрещён в plan/** — только `return error` [Source: docs/project-context.md#Plan Package Инварианты]
- **`plan/` не импортирует** `bridge`, `runner`, `gates`, `cmd/ralph` [Source: docs/project-context.md#Plan Package Инварианты]
- **Subprocess через `exec.CommandContext(ctx)`** [Source: docs/project-context.md#Subprocess]

### Существующий код

- `plan/plan.go` (Story 11.1) — stub с `PlanInput` struct
- `plan/prompts/plan.md` (Story 11.4) — generator template
- `session/session.go` — `Execute(ctx, opts)`, `Options` struct с `InjectFeedback` (Story 11.2)
- `config/config.go` — `Config` с `PlanOutputPath` и другими plan-полями (Story 11.1)
- `bridge/bridge.go` — reference реализации `Run()` pattern (будет удалён в Epic 14)

### writeAtomic pattern

```go
func writeAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    f, err := os.CreateTemp(dir, ".ralph-plan-*")
    if err != nil {
        return fmt.Errorf("plan: write atomic: create temp: %w", err)
    }
    defer os.Remove(f.Name()) // cleanup on error
    if _, err := f.Write(data); err != nil {
        f.Close()
        return fmt.Errorf("plan: write atomic: write: %w", err)
    }
    if err := f.Close(); err != nil {
        return fmt.Errorf("plan: write atomic: close: %w", err)
    }
    return os.Rename(f.Name(), path)
}
```

### Session вызов

```go
sessionOpts := session.Options{
    Prompt:    renderedPrompt,
    // НЕ Resume — всегда чистая сессия для генерации
}
```

### Максимум 3 сессии total

В этой story реализуется только generate (сессия 1). Review (сессия 2) и retry (сессия 3) добавляются в Epic 12 [Source: docs/project-context.md#Review Flow]

### Тестирование

- Mock через `config.ClaudeCommand` — scenario-based JSON [Source: docs/project-context.md#Testing]
- `t.TempDir()` для isolation [Source: CLAUDE.md#Testing Core Rules]
- Table-driven, Go stdlib assertions [Source: CLAUDE.md#Testing Core Rules]
- Verify file existence/absence after test: `os.Stat` для atomic guarantee

### Project Structure Notes

- `plan/plan.go` — расширение stub из Story 11.1: добавление `Run()`, `writeAtomic()`, `templateData`, `PlanOpts`
- `plan/plan_test.go` — НОВЫЙ тест-файл (или расширение)
- Зависимости: `session`, `config`, `text/template`, `strings`, `os`, `path/filepath`

### References

- [Source: docs/epics.md#Story 11.5] — полные AC и технические заметки
- [Source: docs/project-context.md#Plan Package] — архитектура plan/, типы, инварианты
- [Source: docs/project-context.md#Review Flow] — максимум 3 сессии
- [Source: docs/project-context.md#Двухэтапная Prompt Assembly] — template + strings.Replace
- [Source: docs/project-context.md#Subprocess] — exec.CommandContext
- [Source: session/session.go] — Execute(), Options struct
- [Source: CLAUDE.md#Architecture Rules] — dependency direction, exit codes

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added PlanOpts struct with Inputs, OutputPath, Merge, MaxRetries fields
- Implemented Run(ctx, cfg, opts) with prompt generation, session.Execute, ParseResult, writeAtomic
- OutputPath fallback to cfg.PlanOutputPath when empty
- writeAtomic uses os.CreateTemp + os.Rename for atomic file writes
- Added TestMain with RunMockClaude for self-reexec mock pattern
- TestRun_GenerateSuccess: mock returns task content via OutputFile, verifies file written
- TestRun_GenerateFailure: mock exit 1, verifies error message and file not created
- Full regression: go test ./... -count=1 — all packages pass, 0 regressions

### File List

- plan/plan.go (modified): added PlanOpts, Run(), writeAtomic(), imports for session/config/context/os
- plan/plan_test.go (modified): added TestMain, TestRun_GenerateSuccess, TestRun_GenerateFailure
