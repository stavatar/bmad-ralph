# Story 10.7: Warning System + Summary + Session Log

Status: Ready for Review

## Story

As a пользователь Ralph,
I want видеть предупреждения о context fill и compactions в логе и summary,
so that принимать обоснованные решения о настройке max_turns.

## Acceptance Criteria

### AC1: LogContextWarnings — silent below warn (FR89)
- fillPct=40.0, compactions=0, warnPct=55, criticalPct=65
- `LogContextWarnings` — no warning logged (silent)

### AC2: LogContextWarnings — WARN above warn, below critical (FR89)
- fillPct=58.0, compactions=0, maxTurns=15, warnPct=55, criticalPct=65
- WARN: `"context fill 58.0%% — consider reducing max_turns (current: 15) or splitting task into smaller pieces"`

### AC3: LogContextWarnings — ERROR above critical (FR89)
- fillPct=70.0, compactions=0, maxTurns=15, warnPct=55, criticalPct=65
- ERROR: `"context fill 70.0%% exceeds critical threshold — quality degradation likely, reduce max_turns (current: 15)"`

### AC4: LogContextWarnings — ERROR on compaction (FR89)
- fillPct=30.0, compactions=2, maxTurns=15
- ERROR: `"2 compaction(s) detected — context was compressed, quality degraded. Reduce max_turns (current: 15)"`

### AC5: LogContextWarnings — both fill and compaction (FR89)
- fillPct=70.0, compactions=1, maxTurns=15, warnPct=55, criticalPct=65
- TWO messages logged: fill ERROR + compaction ERROR

### AC6: Summary line — normal (FR90)
- `RunMetrics`: MaxContextFillPct=42.7, TotalCompactions=0
- `formatSummary` содержит: `"Context: max 42.7% fill, 0 compactions"`

### AC7: Summary line — with compactions (FR90)
- `RunMetrics`: MaxContextFillPct=65.0, TotalCompactions=2
- Содержит: `"Context: max 65.0% fill, 2 compactions [!]"`
- Строка окрашена жёлтым (`fatih/color`)

### AC8: Summary line — critical fill (FR90)
- `RunMetrics`: MaxContextFillPct=72.0, TotalCompactions=0, criticalPct=65
- Содержит: `"Context: max 72.0% fill, 0 compactions [!]"`
- Строка окрашена красным (`fatih/color`)

### AC9: Session log header — extended (FR92)
- `SessionLogInfo` с Compactions=1, MaxFillPct=58.3
- Header: `"=== SESSION execute seq=3 exit_code=0 elapsed=45.2s compactions=1 max_fill=58.3% ==="`

### AC10: Session log header — zero values (FR92)
- `SessionLogInfo` с Compactions=0, MaxFillPct=0.0
- Header содержит: `"compactions=0 max_fill=0.0%"`

### AC11: SessionLogInfo new fields (FR92)
- `SessionLogInfo` struct получает:
  - `Compactions int`
  - `MaxFillPct float64`
- Поля включены в header format string

## Tasks / Subtasks

- [x] Task 1: LogContextWarnings function (AC: #1-#5)
  - [x] 1.1 `LogContextWarnings(log *RunLogger, fillPct float64, compactions int, maxTurns int, warnPct int, criticalPct int)` в `runner/context.go`
  - [x] 1.2 Silent when fillPct <= warnPct и compactions == 0
  - [x] 1.3 WARN when fillPct > warnPct && fillPct <= criticalPct
  - [x] 1.4 ERROR when fillPct > criticalPct
  - [x] 1.5 ERROR when compactions > 0 (independent of fill level)

- [x] Task 2: Summary context line (AC: #6-#8)
  - [x] 2.1 Добавить Context line в `formatSummary` (`cmd/ralph/run.go`)
  - [x] 2.2 Format: `"Context: max NN.N% fill, N compactions"`
  - [x] 2.3 `[!]` marker при compactions > 0 или fill > criticalPct
  - [x] 2.4 Жёлтый цвет при compactions > 0, красный при fill > criticalPct

- [x] Task 3: Session log header extension (AC: #9-#11)
  - [x] 3.1 Добавить `Compactions int` и `MaxFillPct float64` в `SessionLogInfo`
  - [x] 3.2 Обновить header format string в `SaveSessionLog`

- [x] Task 4: Тесты (AC: #1-#11)
  - [x] 4.1 `runner/context_test.go` — LogContextWarnings tests (silent, warn, error, compaction, both)
  - [x] 4.2 `cmd/ralph/run_test.go` — formatSummary context line tests
  - [x] 4.3 `runner/log_test.go` — session log header format tests

## Dev Notes

### Warning message texts
Тексты дословно из PRD FR89:
- WARN: `"context fill %.1f%% — consider reducing max_turns (current: %d) or splitting task into smaller pieces"`
- ERROR fill: `"context fill %.1f%% exceeds critical threshold — quality degradation likely, reduce max_turns (current: %d)"`
- ERROR compaction: `"%d compaction(s) detected — context was compressed, quality degraded. Reduce max_turns (current: %d)"`

**`%%`** — Go fmt escape для literal `%` символа.

### LogContextWarnings — RunLogger dependency
Функция принимает `*RunLogger`. Для логирования использовать `log.Warnf` / `log.Errorf` (existing methods). Если `log == nil` — no-op (nil-safe).

### formatSummary — existing pattern
`formatSummary` в `cmd/ralph/run.go:65` уже строит multi-line summary с `fatih/color`. Добавить Context line после existing lines. Использовать:
```go
yellow := color.New(color.FgYellow).SprintFunc()
red := color.New(color.FgRed).SprintFunc()
```

### Session log header format
Текущий формат (log.go:186):
```
=== SESSION %s seq=%d exit_code=%d elapsed=%.1fs ===
```
Новый:
```
=== SESSION %s seq=%d exit_code=%d elapsed=%.1fs compactions=%d max_fill=%.1f%% ===
```

### Caller updates
`SaveSessionLog` callers в `runner/runner.go` и `runner/serena.go` нужно обновить — передать `Compactions` и `MaxFillPct` в `SessionLogInfo`. Данные доступны после `CountCompactions` и `EstimateMaxContextFill` (Story 10.6).

### Project Structure Notes

- Файлы: `runner/context.go` (LogContextWarnings), `cmd/ralph/run.go` (formatSummary), `runner/log.go` (SessionLogInfo + header)
- Тесты: `runner/context_test.go`, `cmd/ralph/run_test.go`, `runner/log_test.go`
- `fatih/color` — уже зависимость проекта
- Dependency direction: `cmd/ralph` → `runner` (existing)

### References

- [Source: docs/prd/context-window-observability.md#FR89] — Warning texts
- [Source: docs/prd/context-window-observability.md#FR90] — Summary context line
- [Source: docs/prd/context-window-observability.md#FR92] — Session log header
- [Source: docs/architecture/context-window-observability.md#Решение 5] — Warning System
- [Source: docs/architecture/context-window-observability.md#Решение 7] — Session Log Extension
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.7] — AC, technical notes
- [Source: runner/log.go:165-170] — existing SessionLogInfo struct
- [Source: cmd/ralph/run.go:65-110] — existing formatSummary function

## Testing Standards

- Table-driven tests с `[]struct{name string; ...}` + `t.Run`
- Go stdlib assertions — без testify
- LogContextWarnings: verify log output через buffer capture (existing pattern in log tests)
- formatSummary: verify output strings via `strings.Contains`
- Session log: verify file content via `os.ReadFile` + `strings.Contains`
- Color tests: test underlying string content, not ANSI codes
- Naming: `TestLogContextWarnings_WarnLevel`, `TestFormatSummary_ContextLine`, `TestSaveSessionLog_ContextFields`

## Dev Agent Record

### Context Reference
- Story: docs/sprint-artifacts/10-7-warning-system-summary-session-log.md
- Source: runner/context.go (LogContextWarnings), cmd/ralph/run.go (formatSummary), runner/log.go (SessionLogInfo, SaveSessionLog)

### Agent Model Used
claude-opus-4-6

### Debug Log References
N/A

### Completion Notes List
- LogContextWarnings: fill warn/critical thresholds, compaction detection, nil-safe
- formatSummary: context line (line 4) with color coding — yellow for compactions, red for critical fill
- SessionLogInfo: extended with Compactions/MaxFillPct fields (FR92)
- SaveSessionLog header format: includes compactions=N max_fill=X.X%
- Tests: 5 LogContextWarnings cases + nil test, 3 formatSummary context line cases, existing+new session log tests
- All existing tests updated and passing

### File List
- runner/context.go
- cmd/ralph/run.go
- runner/log.go
- runner/context_test.go
- cmd/ralph/run_test.go
- runner/log_test.go
