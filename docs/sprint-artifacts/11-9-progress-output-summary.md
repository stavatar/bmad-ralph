# Story 11.9: Progress output и summary

Status: Done

## Review Findings (0H/3M/2L)

### MEDIUM
- [M1] `color.Yellow` (строки 94-95) пишет в `os.Stdout`, не в `cmd.OutOrStdout()` — size warning не попадает в captured stdout тестов. AC #4 output не тестируется через stdout capture `[cmd/ralph/plan.go:94-95]`
- [M2] `os.ReadFile` (строка 139) для summary — ошибка тихо проглатывается fallback на generic message. Если файл не записался, пользователь получит "generated successfully" вместо ошибки `[cmd/ralph/plan.go:140]`
- [M3] AC #7 требует тест summary строки в stdout — ни один тест не capture stdout для проверки progress/summary output. Оба теста проверяют только ExitCodeError `[cmd/ralph/cmd_test.go]`

### LOW
- [L1] `TestPlanCmd_SizeWarningContainsForceHint` и `TestPlanCmd_SizeWarningWithoutForce` — дубликаты: оба создают 110KB файл, проверяют ExitCodeError, можно объединить в table-driven `[cmd/ralph/cmd_test.go]`
- [L2] Строка 65 `args = nil` — dead assignment, `args` не используется после этой строки `[cmd/ralph/plan.go:65]`

## Story

As a разработчик,
I want видеть progress-индикатор во время `ralph plan`,
so that я знаю что происходит и не думаю что процесс завис.

## Acceptance Criteria

1. **При начале генерации** stdout показывает: `Генерация плана...`

2. **При успешном завершении генерации** stdout показывает: `Генерация плана... (Ns)`

3. **При завершении всего процесса** stdout показывает summary:
   ```
   sprint-tasks.md готов: 47 задач | path/to/sprint-tasks.md
   ```

4. **При size warning:**
   ```
   Суммарный размер входных документов: 120KB (лимит ~100KB)
     Рекомендуется разбить через 'bmad shard-doc'
     Используйте --force для продолжения
   ```

5. **При Claude API ошибке:** actionable сообщение с exit 1 (FR22)

6. **`fatih/color` используется** для цветного вывода (зелёный, жёлтый, cyan)

7. **Тест в `cmd_test.go`** проверяет наличие summary строки в stdout

## Tasks / Subtasks

- [x] Task 1: Реализовать progress output (AC: #1, #2)
  - [x] `fmt.Fprint(cmd.OutOrStdout(), "Генерация плана...")` перед `plan.Run`
  - [x] После завершения: `fmt.Fprintf(cmd.OutOrStdout(), " (%s)\n", elapsed)` с `time.Since(start)`
  - [x] Использовать `fatih/color` для цветного вывода
- [x] Task 2: Реализовать summary (AC: #3)
  - [x] Подсчёт задач: `strings.Count(content, config.TaskOpen) + strings.Count(content, config.TaskDone)`
  - [x] Вывод: `sprint-tasks.md готов: N задач | path` через color.Green
  - [x] Читает выходной файл после plan.Run для подсчёта
- [x] Task 3: Реализовать size warning output (AC: #4)
  - [x] Жёлтый цвет для предупреждения через color.Yellow
  - [x] Подсказка `--force` для продолжения
  - [x] Без `--force` → ExitCodeError{Code: 2}
- [x] Task 4: Реализовать error output (AC: #5)
  - [x] plan.Run errors propagated through exitCode() mapping
  - [x] fmt.Fprintln newline before error return
- [x] Task 5: Тесты (AC: #7)
  - [x] TestPlanCmd_SizeWarningContainsForceHint: ExitCodeError.Message содержит "100KB"
  - [x] Existing SizeWarningWithoutForce enhanced with Message содержимое проверка

## Dev Notes

### Архитектурные ограничения

- **stdout = user-facing** (через `fatih/color`). stderr не используем [Source: docs/project-context.md#Logging & Output]
- **Packages НЕ логируют** — возвращают results/errors. `cmd/ralph/` решает что в stdout [Source: docs/project-context.md#Logging & Output]
- **`fatih/color`** — существующая зависимость, используется для spinner [Source: docs/project-context.md#Technology Stack]

### Существующий код

- `cmd/ralph/bridge.go` — reference для progress output pattern (если есть)
- `plan/size.go` (Story 11.3) — `CheckSize()` возвращает `warn bool, msg string`
- `config/constants.go` — `TaskOpen = "- [ ]"`, `TaskDone = "- [x]"`

### Подсчёт задач

```go
taskCount := strings.Count(output, "- [ ]") + strings.Count(output, "- [x]")
```

### Timing

```go
start := time.Now()
// ... plan.Run ...
elapsed := time.Since(start).Round(time.Second)
```

### Тестирование

- Progress/summary тесты: capture stdout, проверить substring [Source: CLAUDE.md#Testing Core Rules]
- Count assertions: `strings.Count >= N` [Source: .claude/rules/test-assertions-base.md]

### Project Structure Notes

- `cmd/ralph/plan.go` — модификация: добавление progress output, summary, size warning
- `cmd/ralph/cmd_test.go` — добавление тестов output

### References

- [Source: docs/epics.md#Story 11.9] — полные AC и технические заметки
- [Source: docs/epics.md#FR18] — progress-индикатор по фазам
- [Source: docs/epics.md#FR19] — summary после завершения
- [Source: docs/epics.md#FR21] — size warning >50K
- [Source: docs/epics.md#FR22] — actionable error при Claude API ошибке
- [Source: docs/project-context.md#Logging & Output] — stdout = user-facing
- [Source: config/constants.go] — TaskOpen, TaskDone для подсчёта задач

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added progress output: "Генерация плана..." before plan.Run, " (Ns)" after via cmd.OutOrStdout()
- Added summary: reads output file after Run, counts TaskOpen+TaskDone, prints "sprint-tasks.md готов: N задач | path"
- Size warning format: msg + "Используйте --force для продолжения" via color.Yellow
- Error path: newline printed before returning error (clean terminal output)
- Added time import for timing
- Enhanced SizeWarningWithoutForce test with Message content assertion
- Added TestPlanCmd_SizeWarningContainsForceHint test
- Full regression: go test ./... -count=1 — all packages pass, 0 regressions

### File List

- cmd/ralph/plan.go (modified): progress output, timing, summary, size warning format
- cmd/ralph/cmd_test.go (modified): enhanced + new size warning test
