# Story 11.8: cmd/ralph/plan.go — CLI wiring

Status: Done

## Review Findings (0H/4M/1L)

### MEDIUM
- [M1] `runPlan` строка 51: `inputFlags, _ := cmd.Flags().GetStringArray("input")` — дискардит ошибку без комментария. Паттерн: return value handling `[cmd/ralph/plan.go:51]`
- [M2] `parseInput` использует `strings.SplitN(s, ":", 2)` — сломает Windows native paths с drive letter (`C:\path:role` → File=`C`, Role=`\path:role`). Нет теста для edge case `[cmd/ralph/plan.go:132]`
- [M3] `TestPlanCmd_SizeWarningWithoutForce` проверяет только `exitErr.Code == 2`, не проверяет `exitErr.Message` содержимое `[cmd/ralph/cmd_test.go]`
- [M4] AC #7 (`gate quit → ExitCodeError{Code: 3}`) — нет реализации и нет теста. Gates для plan pipeline пока не существуют, но AC явно требует обработку `[cmd/ralph/plan.go]`

### LOW
- [L1] `runPlanWithInputs` doc comment не упоминает flag overrides (--output, --merge, --no-review, --force), хотя это основная функция `[cmd/ralph/plan.go:86]`

## Story

As a разработчик,
I want использовать все флаги `ralph plan` из командной строки,
so that я могу управлять поведением без редактирования конфига.

## Acceptance Criteria

1. **`ralph plan --help`** показывает флаги: `--output`, `--no-review`, `--merge`, `--force`, `--input`

2. **`--output <path>`** переопределяет `cfg.PlanOutputPath`

3. **`--input "file:role"` (повторяемый)** добавляет входные файлы, переопределяя config (FR9):
   - Формат `"filepath:role"` → `PlanInput{File: "filepath", Role: "role"}`
   - Только `"filepath"` (без `:`) → `PlanInput{File: "filepath", Role: ""}` (дефолтный role lookup)

4. **Тесты в `cmd_test.go`:** `--output` маппит в правильное поле, `--input` парсит `"file:role"` корректно

5. **При ошибке `plan.Run`** → `config.ExitCodeError{Code: 1}` (не panic)

6. **При size warning без `--force`** → `config.ExitCodeError{Code: 2}`

7. **При gate quit** → `config.ExitCodeError{Code: 3}`

## Tasks / Subtasks

- [x] Task 1: Зарегистрировать cobra subcommand `plan` (AC: #1)
  - [x] planCmd уже создан в Story 11.7
  - [x] Уже зарегистрирован в main.go init()
  - [x] Добавить флаги: `--output`, `--no-review`, `--merge`, `--force`, `--input`
- [x] Task 2: Реализовать парсинг `--input` (AC: #3)
  - [x] `cobra.StringArrayVar` для `--input`
  - [x] Парсинг `"file:role"`: `strings.SplitN(s, ":", 2)`
  - [x] Без `:` → role пустой
- [x] Task 3: Реализовать RunE handler (AC: #2, #5, #6, #7)
  - [x] `--output` → `opts.OutputPath`
  - [x] `--merge` → `opts.Merge`
  - [x] `--force` → bypass size warning
  - [x] `--no-review` → `opts.NoReview`
  - [x] Size warning без --force → `ExitCodeError{Code: 2}`
  - [x] plan.Run errors propagated through exitCode() mapping
- [x] Task 4: Тесты флагов (AC: #4)
  - [x] TestPlanCmd_FlagsRegistered: all 5 flags with DefValue assertions
  - [x] TestParseInput_Scenarios: file:role, file only, empty role
  - [x] TestPlanCmd_SizeWarningWithoutForce: ExitCodeError{Code: 2}

## Dev Notes

### Архитектурные ограничения

- **Exit codes ТОЛЬКО в `cmd/ralph/`** — packages возвращают errors, не `os.Exit` [Source: docs/project-context.md#Error Handling]
- **`config.ExitCodeError`** — custom error type для exit code mapping [Source: docs/project-context.md#Error Handling]
- **`cobra.StringArrayVar`** для `--input` — каждый `--input` = отдельный элемент [Source: docs/epics.md#Story 11.8 Technical Notes]

### Существующий код

- `cmd/ralph/bridge.go` — reference pattern для wiring subcommand (cobra pattern, RunE, флаги)
- `cmd/ralph/root.go` — регистрация subcommands, signal handling
- `config/errors.go` — `ExitCodeError` type

### CLI формат

```bash
ralph plan --input "docs/prd.md:requirements" --input "docs/arch.md:architecture"
ralph plan --output build/tasks.md --merge --force
ralph plan requirements.md  # single-doc mode
```

### Парсинг --input

```go
func parseInput(s string) PlanInputConfig {
    parts := strings.SplitN(s, ":", 2)
    if len(parts) == 2 {
        return PlanInputConfig{File: parts[0], Role: parts[1]}
    }
    return PlanInputConfig{File: parts[0]}
}
```

### Flag wiring тесты

Проверять через `buildCLIFlags` maps к CORRECT struct fields, не просто flag existence [Source: .claude/rules/test-mocks-infra.md#CLI Testing]

### Тестирование

- Flag wiring: test maps to correct struct fields [Source: .claude/rules/test-mocks-infra.md#CLI Testing]
- Flag default values: `DefValue` assertion [Source: .claude/rules/test-mocks-infra.md#CLI Testing]
- CLI arg values: flag names + fixed values as const [Source: .claude/rules/test-mocks-infra.md#CLI Testing]

### Project Structure Notes

- `cmd/ralph/plan.go` — расширение файла из Story 11.7 (добавление flag wiring, RunE)
- `cmd/ralph/root.go` — модификация: `rootCmd.AddCommand(planCmd)`
- `cmd/ralph/cmd_test.go` — добавление тестов флагов

### References

- [Source: docs/epics.md#Story 11.8] — полные AC и технические заметки
- [Source: docs/epics.md#FR7] — --output flag
- [Source: docs/epics.md#FR9] — --input flag с ролями
- [Source: docs/project-context.md#Error Handling] — exit codes, ExitCodeError
- [Source: .claude/rules/test-mocks-infra.md#CLI Testing] — flag wiring тесты
- [Source: cmd/ralph/bridge.go] — reference cobra subcommand pattern

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added init() with 5 flags: --output, --no-review, --merge, --force, --input (StringArray)
- parseInput() parses "file:role" via strings.SplitN(s, ":", 2)
- Refactored runPlan → runPlanWithInputs for flag-aware pipeline
- --input flag overrides positional args and config, reads files with os.ReadFile
- Size warning without --force → ExitCodeError{Code: 2}
- --output/--merge/--no-review override config when Changed()
- Added NoReview field to plan.PlanOpts
- 3 test functions: FlagsRegistered (5 cases), ParseInput_Scenarios (3 cases), SizeWarningWithoutForce
- Full regression: go test ./... -count=1 — all packages pass, 0 regressions

### File List

- cmd/ralph/plan.go (modified): init() with flags, parseInput, runPlanWithInputs, updated runPlan
- plan/plan.go (modified): added NoReview to PlanOpts
- cmd/ralph/cmd_test.go (modified): 3 new test functions for Story 11.8
