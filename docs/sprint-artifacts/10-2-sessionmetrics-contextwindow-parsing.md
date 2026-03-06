# Story 10.2: SessionMetrics ContextWindow Parsing

Status: Ready for Review

## Story

As a разработчик,
I want парсить `contextWindow` из JSON result Claude Code,
so that формула context fill использовала реальный размер контекстного окна модели.

## Acceptance Criteria

### AC1: modelUsageEntry struct (FR86)
- `session/result.go` — новый unexported struct `modelUsageEntry`:
  ```go
  type modelUsageEntry struct {
      InputTokens              int     `json:"inputTokens"`
      OutputTokens             int     `json:"outputTokens"`
      CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
      CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
      ContextWindow            int     `json:"contextWindow"`
      MaxOutputTokens          int     `json:"maxOutputTokens"`
      CostUSD                  float64 `json:"costUSD"`
  }
  ```
- JSON tags в **camelCase** (НЕ snake_case как в `usageData`) — верифицировано в Claude Code v2.1.56

### AC2: jsonResultMessage extension (FR86)
- `jsonResultMessage` struct получает новое поле:
  ```go
  ModelUsage map[string]modelUsageEntry `json:"modelUsage"`
  ```
- Поле парсит `modelUsage` map из JSON result

### AC3: SessionMetrics ContextWindow field (FR86)
- `SessionMetrics` struct получает новое поле:
  ```go
  ContextWindow int `json:"context_window"`
  ```
- Поле хранит parsed context window size

### AC4: resultFromMessage extraction (FR86)
- При наличии `modelUsage` в JSON с записью `{"claude-sonnet-4-6": {"contextWindow": 200000, ...}}`
- `resultFromMessage()` извлекает `SessionMetrics.ContextWindow == 200000`
- Итерация по `msg.ModelUsage` с `break` после первой записи (обычно одна модель)

### AC5: Multiple models in modelUsage (FR86)
- При `modelUsage` с 2 моделями: берётся `contextWindow` из первой записи (map iteration order)
- `SessionMetrics.ContextWindow == 200000`

### AC6: Missing modelUsage — backward compat (FR86)
- JSON result без поля `modelUsage` (старые версии Claude Code)
- `ParseResult()` — `SessionMetrics.ContextWindow == 0`
- Все остальные поля парсятся корректно (existing behavior preserved)

### AC7: Empty modelUsage (FR86)
- JSON result с `modelUsage: {}`
- `SessionMetrics.ContextWindow == 0`

### AC8: Existing golden files pass (backward compat)
- `testdata/result_success.json`, `result_success_object.json`, etc.
- `ParseResult()` — все existing assertions проходят
- `SessionMetrics.ContextWindow == 0` (нет modelUsage в golden files)

## Tasks / Subtasks

- [x] Task 1: Добавить modelUsageEntry struct (AC: #1)
  - [x] 1.1 Определить struct с camelCase json tags в `session/result.go`

- [x] Task 2: Расширить jsonResultMessage (AC: #2)
  - [x] 2.1 Добавить `ModelUsage map[string]modelUsageEntry` поле

- [x] Task 3: Расширить SessionMetrics (AC: #3)
  - [x] 3.1 Добавить `ContextWindow int` поле с `json:"context_window"` тегом

- [x] Task 4: Извлечение в resultFromMessage (AC: #4, #5)
  - [x] 4.1 После existing metrics population: итерировать `msg.ModelUsage`, взять `ContextWindow` из первой записи, `break`
  - [x] 4.2 Обеспечить ContextWindow extraction работает НЕЗАВИСИМО от `msg.Usage != nil` — modelUsage может быть при nil usage

- [x] Task 5: Тесты (AC: #1-#8)
  - [x] 5.1 Новый golden file с `modelUsage` — verify ContextWindow parsing
  - [x] 5.2 Golden file с двумя моделями в modelUsage
  - [x] 5.3 Golden file с empty `modelUsage: {}`
  - [x] 5.4 Verify existing golden files проходят с ContextWindow == 0
  - [x] 5.5 Table-driven tests для всех сценариев

## Dev Notes

### camelCase vs snake_case
**КРИТИЧНО:** `modelUsage` использует camelCase json tags (`inputTokens`, `contextWindow`), в отличие от `usage` который использует snake_case (`input_tokens`). Это НЕ баг — Claude Code реально использует разные стили для разных полей. Верифицировано в исходниках Claude Code v2.1.56.

### Extraction logic
```go
// В resultFromMessage, ПОСЛЕ existing metrics population:
if len(msg.ModelUsage) > 0 {
    for _, entry := range msg.ModelUsage {
        if r.Metrics == nil {
            r.Metrics = &SessionMetrics{}
        }
        r.Metrics.ContextWindow = entry.ContextWindow
        break
    }
}
```

**ВАЖНО:** `modelUsage` может быть при `msg.Usage == nil`. Extraction MUST handle case where `r.Metrics` is nil — создать минимальный `SessionMetrics` с only `ContextWindow`.

### JSON structure example
```json
{
  "type": "result",
  "session_id": "abc-123",
  "result": "Done",
  "usage": {"input_tokens": 100, "output_tokens": 50, ...},
  "modelUsage": {
    "claude-sonnet-4-6-20250514": {
      "inputTokens": 12345,
      "outputTokens": 6789,
      "cacheReadInputTokens": 500,
      "cacheCreationInputTokens": 100,
      "contextWindow": 200000,
      "maxOutputTokens": 16384,
      "costUSD": 0.03
    }
  }
}
```

### Backward compatibility
- Zero value `ContextWindow == 0` означает "неизвестно"
- Все потребители (в будущих stories) проверяют: `if contextWindow == 0 { contextWindow = 200000 }`
- Существующие golden files НЕ содержат `modelUsage` → `ContextWindow == 0`
- Все existing tests должны проходить БЕЗ изменений

### Project Structure Notes

- Изменяемый файл: `session/result.go`
- Тестовый файл: `session/result_test.go`
- Golden files: `session/testdata/`
- Dependency direction: `session` — не зависит от `runner` или `config`
- `modelUsageEntry` — unexported (используется только внутри session package)

### References

- [Source: docs/prd/context-window-observability.md#FR86] — ContextWindow from modelUsage
- [Source: docs/architecture/context-window-observability.md#Решение 3] — SessionMetrics extension, modelUsageEntry struct
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.2] — AC, technical notes
- [Source: session/result.go:28-35] — existing SessionMetrics struct
- [Source: session/result.go:45-55] — existing jsonResultMessage struct
- [Source: session/result.go:133-154] — resultFromMessage function

## Testing Standards

- Table-driven tests с `[]struct{name string; ...}` + `t.Run`
- Go stdlib assertions — без testify
- Golden files для JSON input: `testdata/result_with_modelusage.json`, etc.
- Error tests: `strings.Contains(err.Error(), "expected substring")`
- Naming: `TestParseResult_WithModelUsage`, `TestParseResult_EmptyModelUsage`
- Backward compat: все existing tests MUST pass unchanged
- Verify ContextWindow field value, не только != 0

## Dev Agent Record

### Context Reference

### Agent Model Used
Claude Opus 4.6

### Debug Log References

### Completion Notes List
- Added modelUsageEntry struct with camelCase JSON tags (inputTokens, contextWindow, etc.)
- Extended jsonResultMessage with ModelUsage map[string]modelUsageEntry field
- Added ContextWindow int field to SessionMetrics with json:"context_window" tag
- Implemented extraction in resultFromMessage: iterates modelUsage, takes first entry's contextWindow
- Handles nil Metrics case: creates minimal SessionMetrics when modelUsage present but usage nil
- 3 new golden files: result_with_modelusage.json, result_with_modelusage_multi.json, result_with_modelusage_empty.json
- Table-driven TestParseResult_ContextWindow with 6 cases covering all ACs
- Added ContextWindow to TestSessionMetrics_ZeroValue
- All existing tests pass unchanged (backward compat)

### File List
- session/result.go (modified: modelUsageEntry struct, jsonResultMessage.ModelUsage, SessionMetrics.ContextWindow, resultFromMessage extraction)
- session/result_test.go (modified: TestParseResult_ContextWindow, TestSessionMetrics_ZeroValue)
- session/testdata/result_with_modelusage.json (new)
- session/testdata/result_with_modelusage_multi.json (new)
- session/testdata/result_with_modelusage_empty.json (new)
