# Story 12.3: Retry logic с InjectFeedback

Status: review

## Story

As a система,
I want автоматически перегенерировать план с feedback от reviewer,
so that обнаруженные проблемы исправляются без участия пользователя.

## Acceptance Criteria

1. **При `ISSUES:` от reviewer** → retry: `session.Execute` с `Options{InjectFeedback: reviewerOutput}` (Story 11.2)

2. **Прогресс:** `Retry генерации с feedback...` → `Retry завершён (Ns)`

3. **Результат retry проходит review** (ещё один `session.Execute` без InjectFeedback)

4. **Retry-review `OK`** → план принят, файл записан

5. **Retry-review `ISSUES:`** → gate [p/e/q] (FR17):
   ```
   Review выявил проблемы после retry:
   <issues текст>
   [p] proceed — принять план как есть
   [e] edit    — открыть файл в редакторе
   [q] quit    — выйти (exit 3)
   ```

6. **Максимум 1 retry:** после retry review не повторяется второй раз (max 3 сессии total)

7. **Тесты `plan_test.go`:**
   - `retry_success` → retry OK, файл записан
   - `retry_fail_gate_proceed` → gate p → файл записан
   - `retry_fail_gate_quit` → gate q → error возвращён

## Tasks / Subtasks

- [x] Task 1: Реализовать retry logic в `plan.Run()` (AC: #1, #3, #6)
  - [x] При ISSUES: вызвать generate с InjectFeedback
  - [x] После retry: вызвать review (чистая сессия)
  - [x] Max 1 retry (3 сессии total)
- [x] Task 2: Реализовать gate [p/e/q] (AC: #5)
  - [x] При retry-review ISSUES: показать report и gate prompt
  - [x] Gate input из `os.Stdin` (mockable via GateReader)
  - [x] [p] → writeAtomic, return nil
  - [x] [e] → return error "not implemented"
  - [x] [q] → return error "user quit"
- [x] Task 3: Progress output (AC: #2)
- [x] Task 4: Тесты (AC: #7)
  - [x] Mock сценарии с ordered responses (4-step scenarios)
  - [x] Gate mock через GateReader (strings.NewReader)

## Dev Notes

### Архитектурные ограничения

- **Gate input из `os.Stdin`** — в тестах подменяется через mock [Source: docs/epics.md#Story 12.3 Technical Notes]
- **`[e] edit` — not implemented in MVP** — no-op с сообщением [Source: docs/epics.md#Story 12.3]
- **Max 3 сессии:** generate → review → retry. Без второго retry-review [Source: docs/project-context.md#Review Flow]

### Существующий код

- `session/session.go` — `Options.InjectFeedback` (Story 11.2)
- `gates/gates.go` — `Prompt()` pattern для [p/e/q] gate (reference)

### References

- [Source: docs/epics.md#Story 12.3] — полные AC
- [Source: docs/epics.md#FR16] — max 1 retry
- [Source: docs/epics.md#FR17] — gate [p/e/q]
- [Source: docs/project-context.md#Review Flow] — max 3 сессии

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Refactored Run() into runGenerate() and runReview() helpers for DRY
- runGenerate() accepts optional feedback for InjectFeedback (empty = first generate)
- runReview() returns issues text (non-empty) or "" on OK
- Retry flow: generate → review ISSUES → retry generate with feedback → retry review
- Gate [p/e/q] via promptGate(): reads from GateReader (default os.Stdin)
- [p] proceed: accept plan, write file; [e] edit: returns error (not implemented MVP); [q] quit: returns error
- Progress output via ProgressWriter (default os.Stderr): "Retry генерации с feedback..." + timing
- Max 1 retry enforced by linear flow (no loop)
- Updated TestRun_ReviewIssues → TestRun_ReviewIssuesTriggersRetry (verifies retry is attempted)
- TestRun_RetrySuccess: 4-step mock, retry review OK, file written with retry content
- TestRun_RetryFailGateProceed: 4-step mock, gate "p", file written despite issues
- TestRun_RetryFailGateQuit: 4-step mock, gate "q", error returned, file NOT written
- Removed errors import (no longer needed after ErrReviewIssues removed from test)
- Full regression: go test ./... -count=1 — all packages pass

### File List

- plan/plan.go (modified): runGenerate(), runReview(), promptGate(), GateReader/ProgressWriter in PlanOpts, retry flow in Run()
- plan/plan_test.go (modified): TestRun_ReviewIssuesTriggersRetry (updated), TestRun_RetrySuccess, TestRun_RetryFailGateProceed, TestRun_RetryFailGateQuit

## Review Record

### Review Agent Model Used

claude-opus-4-6

### Review Findings

Status: review — 0H/3M/2L

#### [MEDIUM] M1: ErrReviewIssues — dead type с stale doc comment

`plan.go:87-94`: `ErrReviewIssues` определён и задокументирован как "returned when the reviewer finds issues", но `Run()` больше не возвращает этот тип — issues обрабатываются inline через retry + gate. Тип не используется ни в production code ни в тестах (errors import удалён в 12.3). Dead code + stale doc comment.

Fix: удалить `ErrReviewIssues` type и связанный `Error()` method, или оставить для будущего использования с актуализированным doc comment.

#### [MEDIUM] M2: promptGate hardcodes os.Stderr, игнорирует ProgressWriter

`plan.go:244-247`: `promptGate` пишет gate text напрямую в `os.Stderr`, хотя `PlanOpts.ProgressWriter` создан именно для этого. Inconsistency: progress output (строка 136) использует `ProgressWriter`, gate output — нет. Gate output попадает в test stdout неконтролируемо.

Fix: передать `io.Writer` в `promptGate` и использовать `ProgressWriter` из opts.

#### [MEDIUM] M3: SetupMockClaude error discarded в 7 местах

`plan_test.go`: строки 154, 210, 268, 330, 476, 555, 634 — `scenarioPath, _ := testutil.SetupMockClaude(...)`. По правилу #6 (never discard return values), ошибку нужно проверять или документировать `_ =` с комментарием.

Fix: если SetupMockClaude вызывает t.Fatal внутри при ошибке, добавить комментарий `// SetupMockClaude calls t.Fatal on error`. Если нет — проверять ошибку.

#### [LOW] L1: TestRun_RetrySuccess не проверяет отсутствие original content

`plan_test.go:516`: проверяет `"Improved task"` present, но не проверяет `"First task"` absent. Если writeAtomic по ошибке конкатенирует вместо перезаписи, тест не поймает.

Fix: добавить `if strings.Contains(string(data), "First task") { t.Error("original content should be replaced by retry") }`.

#### [LOW] L2: Completion Notes содержат неточность

Completion Notes (строка 98): "Removed errors import (no longer needed after ErrReviewIssues removed from test)" — но ErrReviewIssues type НЕ удалён из plan.go (только из тестов). Misleading.

### New Patterns Discovered

1. **Dead type after refactoring**: When a custom error type (e.g., ErrReviewIssues) is introduced in story N and the flow is refactored in story N+1, verify the type is still used or remove it. Stale types with misleading doc comments create confusion.

### Previous Review Issues Status

- M1 from 12.2 (AC #4 review progress output): still NOT implemented — no `AI review плана...` output in cmd/ or plan/
- M2 from 12.2 (SetupMockClaude error discarded): still present, now in 7 locations (was 2)
