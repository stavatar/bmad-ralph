# Story 10.3: Core Context Functions

Status: Ready for Review

## Story

As a разработчик,
I want иметь функции для создания compaction counter и расчёта context fill %,
so that интегрировать их в runner pipeline.

## Acceptance Criteria

### AC1: CreateCompactCounter — happy path (FR82)
- `runner/context.go` (новый файл)
- `CreateCompactCounter()` возвращает non-empty path к temp file в `os.TempDir()`
- Имя файла соответствует паттерну `"ralph-compact-*"`
- Cleanup function удаляет файл при вызове

### AC2: CreateCompactCounter — error (NFR30)
- При ошибке `os.CreateTemp` (например, invalid TempDir)
- Возвращает `("", no-op cleanup)`
- Нет panic

### AC3: CreateCompactCounter — cleanup idempotent (FR82, NFR34)
- Cleanup вызванный дважды — нет ошибки (second call = no-op)

### AC4: CountCompactions — empty file (FR84)
- Temp file существует, но пуст (0 bytes)
- `CountCompactions(path)` возвращает 0

### AC5: CountCompactions — 1 compaction (FR84)
- Файл с содержимым `"1\n"`
- `CountCompactions(path)` возвращает 1

### AC6: CountCompactions — 3 compactions (FR84)
- Файл с содержимым `"1\n1\n1\n"`
- `CountCompactions(path)` возвращает 3

### AC7: CountCompactions — missing file (FR84, NFR30)
- Path к несуществующему файлу
- `CountCompactions(path)` возвращает 0 (graceful degradation)

### AC8: CountCompactions — empty path (FR84, NFR30)
- `CountCompactions("")` возвращает 0

### AC9: CountCompactions — corrupt file with blank lines (FR84)
- Файл с содержимым `"1\n\n1\n\n"`
- `CountCompactions(path)` возвращает 2 (только non-empty lines)

### AC10: EstimateMaxContextFill — happy path (FR85)
- `SessionMetrics`: cache_read=1456521, cache_creation=57388, input=2700, numTurns=25
- `contextWindow = 200000`
- `EstimateMaxContextFill(metrics, 200000)` возвращает approximately 60.7 (±0.1)

### AC11: EstimateMaxContextFill — uses metrics.ContextWindow (FR86)
- `SessionMetrics` с `ContextWindow=200000`
- `EstimateMaxContextFill(metrics, 100000)` возвращает ~60.7 (использует `metrics.ContextWindow=200000`, НЕ fallback=100000)

### AC12: EstimateMaxContextFill — fallback when ContextWindow=0 (FR86)
- `SessionMetrics` с `ContextWindow=0`
- `EstimateMaxContextFill(metrics, 200000)` использует `fallbackContextWindow=200000`

### AC13: EstimateMaxContextFill — guard max(numTurns, 2) (FR85)
- `SessionMetrics`: cache_read=20000, cache_creation=5000, input=500, numTurns=1
- `EstimateMaxContextFill(metrics, 200000)` возвращает ~12.8 (uses effective_turns=2, not 1)

### AC14: EstimateMaxContextFill — zero turns (FR85)
- `SessionMetrics` с numTurns=0
- Возвращает 0.0

### AC15: EstimateMaxContextFill — nil metrics (FR85)
- `EstimateMaxContextFill(nil, 200000)` возвращает 0.0

### AC16: EstimateMaxContextFill — zero context window both (FR85)
- `SessionMetrics` с `ContextWindow=0`, fallback=0
- Возвращает 0.0 (no division by zero)

## Tasks / Subtasks

- [x] Task 1: Создать runner/context.go (AC: #1-#3)
  - [x] 1.1 `CreateCompactCounter() (path string, cleanup func())` — `os.CreateTemp("", "ralph-compact-*")`
  - [x] 1.2 Cleanup = `os.Remove(path)` с ignore error (idempotent)
  - [x] 1.3 Error path: return `("", func() {})` — no-op cleanup

- [x] Task 2: CountCompactions function (AC: #4-#9)
  - [x] 2.1 `CountCompactions(path string) int`
  - [x] 2.2 Empty path → return 0
  - [x] 2.3 `os.ReadFile` → error → return 0
  - [x] 2.4 Count non-empty lines: `strings.Split` + filter empty strings

- [x] Task 3: EstimateMaxContextFill function (AC: #10-#16)
  - [x] 3.1 `EstimateMaxContextFill(metrics *session.SessionMetrics, fallbackContextWindow int) float64`
  - [x] 3.2 Nil metrics → return 0.0
  - [x] 3.3 numTurns == 0 → return 0.0
  - [x] 3.4 contextWindow: `metrics.ContextWindow > 0` → use it, else fallback
  - [x] 3.5 contextWindow == 0 (both) → return 0.0
  - [x] 3.6 Formula: `2 × (cache_read + cache_creation + input) / max(numTurns, 2) / contextWindow × 100`

- [x] Task 4: DefaultContextWindow constant (AC: part of integration)
  - [x] 4.1 `const DefaultContextWindow = 200000`

- [x] Task 5: Тесты (AC: #1-#16)
  - [x] 5.1 `runner/context_test.go` — новый файл
  - [x] 5.2 Table-driven tests для CreateCompactCounter
  - [x] 5.3 Table-driven tests для CountCompactions (6 cases)
  - [x] 5.4 Table-driven tests для EstimateMaxContextFill (7 cases with numerical verification)

## Dev Notes

### Новый файл runner/context.go
Группирует 3 функции + 1 константу единой подсистемы context observability. Не раздувает `metrics.go` (уже ~300 LOC). Аналогично `similarity.go`.

### Формула EstimateMaxContextFill
```
total_cumulative = cache_read + cache_creation + input_tokens
effective_turns = max(num_turns, 2)
estimated_max = 2 × total_cumulative / effective_turns
fill_pct = estimated_max / context_window × 100
```

**Guard `max(numTurns, 2)`:** при `numTurns == 1` формула даёт `2 × total / 1` = двойной переcчёт. Minimum 2 предотвращает завышение.

**Числовой пример (реальная сессия #4):**
```
cache_read = 1,456,521    cache_creation = 57,388    input = 2,700
num_turns = 25            context_window = 200,000
total = 1,516,609
estimated_max = 2 × 1,516,609 / 25 = 121,329
fill_pct = 121,329 / 200,000 × 100 = 60.66...%
```

### SessionMetrics field access
Функция принимает `*session.SessionMetrics` — доступ к:
- `metrics.CacheReadTokens` (cache_read)
- `metrics.CacheCreationTokens` (cache_creation)
- `metrics.InputTokens` (input)
- `metrics.NumTurns`
- `metrics.ContextWindow` (добавлен в Story 10.2)

### CreateCompactCounter lifecycle
- `os.CreateTemp("", "ralph-compact-*")` — OS temp dir, уникальное имя
- Caller MUST `defer cleanup()`
- При crash процесса — OS cleanup через стандартный механизм temp-файлов

### Тесты — числовая точность
Для float comparison использовать `math.Abs(got - want) < 0.1` (±0.1), НЕ exact equality.

### Project Structure Notes

- Новые файлы: `runner/context.go`, `runner/context_test.go`
- Import: `session` package (для `*session.SessionMetrics`)
- Dependency direction: `runner` → `session` (существующая, не новая)
- `DefaultContextWindow` — exported constant для use в runner integration (Story 10.6)

### References

- [Source: docs/prd/context-window-observability.md#FR82] — CreateCompactCounter
- [Source: docs/prd/context-window-observability.md#FR84] — CountCompactions
- [Source: docs/prd/context-window-observability.md#FR85] — EstimateMaxContextFill formula
- [Source: docs/prd/context-window-observability.md#FR86] — ContextWindow from metrics
- [Source: docs/architecture/context-window-observability.md#Решение 1] — context.go functions
- [Source: docs/architecture/context-window-observability.md#Решение 2] — Formula details
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.3] — AC, technical notes
- [Source: session/result.go:28-35] — SessionMetrics struct (fields for formula)

## Testing Standards

- Table-driven tests с `[]struct{name string; ...}` + `t.Run`
- Go stdlib assertions — без testify
- `t.TempDir()` для file-based tests (CreateCompactCounter, CountCompactions)
- Float comparison: `math.Abs(got - want) < 0.1` — не exact equality
- Error tests: `strings.Contains` для error messages
- Naming: `TestCreateCompactCounter_HappyPath`, `TestCountCompactions_EdgeCases`, `TestEstimateMaxContextFill_Formula`
- Coverage: каждый AC — минимум один test case

## Dev Agent Record

### Context Reference

### Agent Model Used
Claude Opus 4.6

### Debug Log References

### Completion Notes List
- Created runner/context.go with CreateCompactCounter, CountCompactions, EstimateMaxContextFill, DefaultContextWindow
- CreateCompactCounter: os.CreateTemp with ralph-compact-* pattern, idempotent cleanup via os.Remove
- CountCompactions: reads file, counts non-empty lines, graceful degradation on errors
- EstimateMaxContextFill: formula 2×total/max(turns,2)/ctxWindow×100, nil-safe, uses metrics.ContextWindow with fallback
- DefaultContextWindow = 200000
- 16 test cases covering all 16 ACs: happy path, edge cases, numerical verification (±0.1 tolerance)
- All existing tests pass (no regressions)

### File List
- runner/context.go (new: CreateCompactCounter, CountCompactions, EstimateMaxContextFill, DefaultContextWindow)
- runner/context_test.go (new: 4 test functions, 16 cases total)
