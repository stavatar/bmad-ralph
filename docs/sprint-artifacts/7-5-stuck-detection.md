# Story 7.5: Stuck Detection

Status: done

## Story

As a разработчик,
I want чтобы ralph обнаруживал застревание (нет commit за N попыток подряд) и подкидывал hint в prompt,
so that AI попробовал другой подход раньше, чем исчерпаются лимиты.

## Acceptance Criteria

1. **AC1: Config field (FR49)**
   - `Config` struct расширен: `StuckThreshold int` yaml:"stuck_threshold"
   - Default: `stuck_threshold: 2` в `defaults.yaml`
   - `StuckThreshold == 0` отключает stuck detection

2. **AC2: Stuck detection triggers feedback injection (FR49)**
   - При `StuckThreshold == 2` и 2 consecutive execute с `headAfter == headBefore`:
     - `InjectFeedback` вызывается с message содержащим "no commit" и attempt count
     - `logger.Warn("stuck detected", kv("task", taskText), kv("no_commit_count", 2))`
     - MetricsCollector records stuck event если available (nil guard)

3. **AC3: Counter resets on successful commit**
   - При `consecutiveNoCommit == 2` (threshold reached) и следующий execute produces commit:
     - `consecutiveNoCommit` сбрасывается в 0
     - Больше нет stuck feedback injection

4. **AC4: Stuck detection disabled**
   - При `StuckThreshold == 0` и многократном `headAfter == headBefore`:
     - Нет stuck feedback injection
     - Нет stuck warning в log

5. **AC5: Stuck detection does NOT replace MaxIterations**
   - При `StuckThreshold == 2` и `MaxIterations == 3`:
     - Stuck detected at attempt 2 → feedback injected, loop continues
     - MaxIterations enforced at attempt 3 (emergency gate или stop)

6. **AC6: Feedback message content**
   - Message: `"No commit in last N attempts. Consider a different approach."`
   - Формат совместим с existing `FeedbackPrefix` pattern
   - N = текущий `consecutiveNoCommit` count

## Tasks / Subtasks

- [x] Task 1: Add StuckThreshold to Config (AC: #1)
  - [x] 1.1 Добавить `StuckThreshold int` yaml:"stuck_threshold" в Config struct
  - [x] 1.2 Добавить `stuck_threshold: 2` в `config/defaults.yaml`
  - [x] 1.3 Тесты: default value, override в yaml, zero = disabled

- [x] Task 2: Implement stuck detection in execute loop (AC: #2, #3, #4, #5, #6)
  - [x] 2.1 В `runner.execute()`: добавить `var consecutiveNoCommit int` рядом с `completedTasks` (перед outer task loop) — cross-task scope, сбрасывается ТОЛЬКО при successful commit
  - [x] 2.2 Inside `else` branch of `if execErr != nil` (successful execution path), в блоке `if headBefore == headAfter`:
    - `headAfter == headBefore` → `consecutiveNoCommit++`
    - `headAfter != headBefore` → `consecutiveNoCommit = 0`
    - NB: exec-error path НЕ меняет counter (headAfter check не выполняется при exec error)
  - [x] 2.3 Check (сразу после increment): `if r.Cfg.StuckThreshold > 0 && consecutiveNoCommit >= r.Cfg.StuckThreshold`
  - [x] 2.4 При stuck: call `InjectFeedback(r.TasksFile, taskDescription(taskText), msg)` (3 params: tasksFile, taskDesc, feedback) и `r.logger().Warn(...)`
  - [x] 2.5 Message: `fmt.Sprintf("No commit in last %d attempts. Consider a different approach.", consecutiveNoCommit)`
  - [x] 2.6 Nil guard: `if r.Metrics != nil { r.Metrics.RecordRetry("stuck") }` — NB: RecordRetry currently no-op stub (empty body in metrics.go), will be instrumented in later story

- [x] Task 3: Тесты (AC: #1-#6)
  - [x] 3.1 Тест: 2 no-commit attempts → stuck feedback injected (verify InjectFeedback called + logger.Warn)
  - [x] 3.2 Тест: commit after stuck → counter reset, no more feedback
  - [x] 3.3 Тест: StuckThreshold == 0 → no detection
  - [x] 3.4 Тест: stuck does NOT terminate loop (MaxIterations still enforced)
  - [x] 3.5 Тест: feedback message content — `strings.Contains(msg, "No commit in last 2 attempts")` AND `strings.Contains(msg, "Consider a different approach")`
  - [x] 3.6 Тест: Metrics == nil → stuck detection works without panic (nil guard)
  - [x] 3.7 Тест: exec-error interleaved — sequence no-commit → exec-error → no-commit → counter == 2 (exec-error does NOT reset or increment counter)

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `config/config.go` | Extend: +StuckThreshold field | ~3 |
| `config/defaults.yaml` | Extend: +stuck_threshold: 2 | ~1 |
| `runner/runner.go` | Extend: stuck detection in execute() | ~15 |
| `runner/runner_test.go` | Add: stuck detection тесты (7 cases) | ~80 |

### Architecture Compliance

- **Minimal scope:** stuck detection — local counter в execute(), no new files
- **Uses existing InjectFeedback:** не создаёт новый mechanism
- **Config immutability:** StuckThreshold читается, не мутируется

### Existing Code Context

- `execute()` loop structure: outer `for i` (tasks) → inner `for` (review cycles) → inner-inner `for` (retry)
- `headBefore`/`headAfter` comparison inside inner-inner loop, in the `else` branch of `if execErr != nil` (i.e., successful execution path only). Find it by searching for `headBefore == headAfter` in runner.go
- NB: line numbers are PRE-7.1 estimates and WILL shift after Story 7.1 modifies execute(). Use code patterns to locate insertion points, not line numbers
- `InjectFeedback(tasksFile, taskDesc, feedback string) error` — 3 параметра! taskDesc нужен для нахождения задачи в файле. Вызов: `InjectFeedback(r.TasksFile, taskDescription(taskText), msg)`
- `FeedbackPrefix = "> USER FEEDBACK:"` — в `config/constants.go`
- `consecutiveNoCommit` — новый local var в execute(), не field в Runner. Scope: весь run (cross-task), сбрасывается ТОЛЬКО при successful commit. Объявлять рядом с `completedTasks` (перед outer task loop)
- exec-error path (ExitError detected): `needsRetry = true` but headAfter NOT checked → counter unchanged. Stuck detection only tracks the no-commit case
- `MaxIterations` enforcement и emergency gate — уже в loop, stuck detection вставляется ДО emergency check (in the `headBefore == headAfter` block, before `needsRetry` handling)
- `taskDescription(taskText)` — helper function для извлечения task description для InjectFeedback
- Architecture Решение 6 uses "Try a different approach" but Epic AC6 says "Consider a different approach" — follow Epic/AC (this story)
- `MetricsCollector.RecordRetry(reason string)` exists but is currently empty no-op stub — call it anyway for forward compatibility

### References

- [Source: docs/architecture/observability-metrics.md#Решение 6] — Stuck Detection
- [Source: docs/prd/observability-metrics.md#FR49] — stuck detection requirement
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.5] — полное описание

## Dev Agent Record

### Context Reference

- docs/architecture/observability-metrics.md#Решение 6
- docs/prd/observability-metrics.md#FR49

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- StuckThreshold field added to Config with yaml tag and validation (>= 0)
- Default stuck_threshold: 2 in defaults.yaml
- Stuck detection in execute(): consecutiveNoCommit counter, cross-task scope
- InjectFeedback called with stuck message, error non-fatal (logged)
- Metrics.RecordRetry("stuck") with nil guard
- Counter resets to 0 on successful commit only
- 8 tests: feedback injection, counter reset, disabled, loop continues, nil metrics, exec-error interleave, message content, plus existing tests unaffected
- Code review fixes: Execute doc comment updated, consecutiveNoCommit doc comment added, Metrics positive path exercised in test

### File List

- config/config.go — Added StuckThreshold field to Config struct, validation in Validate()
- config/defaults.yaml — Added stuck_threshold: 2 default
- runner/runner.go — Stuck detection logic in execute(), doc comments updated
- runner/runner_test.go — 8 stuck detection tests (AC1-AC6)
