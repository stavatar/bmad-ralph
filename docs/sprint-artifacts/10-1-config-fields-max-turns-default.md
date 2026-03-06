# Story 10.1: Config Fields + max_turns Default

Status: Ready for Review

## Story

As a разработчик,
I want настраиваемые пороги context fill % и безопасный дефолт max_turns,
so that Ralph по умолчанию работает в зелёной зоне контекста (≤50% fill).

## Acceptance Criteria

### AC1: Config struct — новые поля (FR91)
- `config/config.go` — `Config` struct получает два новых поля:
  - `ContextWarnPct int` с тегом `yaml:"context_warn_pct"`
  - `ContextCriticalPct int` с тегом `yaml:"context_critical_pct"`
- Поля парсятся из ralph.yaml корректно
- Zero values до применения defaults

### AC2: Defaults (FR81, FR91)
- `config/defaults.yaml` обновляется:
  - `max_turns: 15` (было 50)
  - `context_warn_pct: 55`
  - `context_critical_pct: 65`
- После загрузки: `Config.MaxTurns == 15`, `Config.ContextWarnPct == 55`, `Config.ContextCriticalPct == 65`

### AC3: Validation — range (FR91)
- `Validate()` возвращает ошибку при `ContextWarnPct < 1` или `> 99`:
  - `"config: validate: context_warn_pct must be 1-99, got 0"`
- `Validate()` возвращает ошибку при `ContextCriticalPct < 1` или `> 99`:
  - `"config: validate: context_critical_pct must be 1-99, got 100"`

### AC4: Validation — ordering (FR91)
- `Validate()` возвращает ошибку при `ContextCriticalPct <= ContextWarnPct`:
  - `"config: validate: context_critical_pct (55) must be > context_warn_pct (65)"`
- Равные значения (`55 == 55`) тоже ошибка — сообщение содержит `"must be >"`

### AC5: Validation — happy path (FR91)
- `Validate()` НЕ возвращает ошибку при `ContextWarnPct = 55, ContextCriticalPct = 65`

### AC6: max_turns default change (FR81)
- `config/defaults.yaml` содержит `max_turns: 15`
- `Config` загруженный без user override: `MaxTurns == 15`
- Существующие тесты с явным `MaxTurns` override не затрагиваются

## Tasks / Subtasks

- [x] Task 1: Добавить поля в Config struct (AC: #1)
  - [x] 1.1 Добавить `ContextWarnPct int \`yaml:"context_warn_pct"\`` в `config/config.go`
  - [x] 1.2 Добавить `ContextCriticalPct int \`yaml:"context_critical_pct"\`` в `config/config.go`

- [x] Task 2: Обновить defaults.yaml (AC: #2, #6)
  - [x] 2.1 Изменить `max_turns: 50` → `max_turns: 15`
  - [x] 2.2 Добавить `context_warn_pct: 55`
  - [x] 2.3 Добавить `context_critical_pct: 65`

- [x] Task 3: Добавить валидацию в Validate() (AC: #3, #4, #5)
  - [x] 3.1 Range check для `ContextWarnPct` (1-99)
  - [x] 3.2 Range check для `ContextCriticalPct` (1-99)
  - [x] 3.3 Ordering check: `ContextCriticalPct > ContextWarnPct`

- [x] Task 4: Тесты (AC: #1-#6)
  - [x] 4.1 Table-driven validation tests: range errors, ordering error, happy path
  - [x] 4.2 Defaults loading test: verify MaxTurns==15, ContextWarnPct==55, ContextCriticalPct==65
  - [x] 4.3 Проверить что существующие тесты проходят с новым max_turns default

## Dev Notes

### Паттерн валидации
Следовать существующему паттерну `BudgetWarnPct` в `Validate()` (config.go:206-208):
```go
if c.BudgetMaxUSD > 0 && (c.BudgetWarnPct < 1 || c.BudgetWarnPct > 99) {
```

**Отличие:** `ContextWarnPct` и `ContextCriticalPct` валидируются ВСЕГДА (не условно как BudgetWarnPct, который проверяется только при `BudgetMaxUSD > 0`). Context observability — всегда активна.

### Error message format
Строго следовать паттерну `"config: validate: <field> must be <range>, got <value>"`:
- `"config: validate: context_warn_pct must be 1-99, got %d"`
- `"config: validate: context_critical_pct must be 1-99, got %d"`
- `"config: validate: context_critical_pct (%d) must be > context_warn_pct (%d)"`

### max_turns breaking default
`max_turns: 50 → 15` — breaking default, но допустимо:
- Ralph поддерживает resume — задачи продолжаются в следующей итерации
- При `max_iterations: 3` и `max_turns: 15` агент получает до 45 turns (3 × 15) со свежим контекстом
- Пользователь может вернуть 50 через config или CLI `--max-turns`

### Размещение полей в struct
Новые поля добавляются после `BudgetWarnPct` / `TaskBudgetMaxUSD` группы (аналогичная группа "пороги"):
```go
ContextWarnPct    int `yaml:"context_warn_pct"`
ContextCriticalPct int `yaml:"context_critical_pct"`
```

### Тесты — что проверить
- Каждый error message дословно через `strings.Contains` (не bare `err != nil`)
- Table-driven cases: `ContextWarnPct=0`, `ContextWarnPct=100`, `ContextCriticalPct=0`, `ContextCriticalPct=100`, `Warn=65/Critical=55` (reversed), `Warn=55/Critical=55` (equal), happy `Warn=55/Critical=65`
- Defaults test: `defaultConfig()` возвращает `MaxTurns==15, ContextWarnPct==55, ContextCriticalPct==65`
- Проверить что `config/config_test.go` существующие тесты не ломаются от `max_turns: 15`

### Project Structure Notes

- Изменяемые файлы: `config/config.go`, `config/defaults.yaml`
- Тестовые файлы: `config/config_test.go`
- Dependency direction: `config` — leaf package, без зависимостей
- Паттерн: `Config` parsed once, passed by pointer, never mutated at runtime

### References

- [Source: docs/prd/context-window-observability.md#FR81] — max_turns 50→15
- [Source: docs/prd/context-window-observability.md#FR91] — ContextWarnPct, ContextCriticalPct, validation
- [Source: docs/architecture/context-window-observability.md#Решение 6] — Config fields, defaults, validation code
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.1] — AC, technical notes
- [Source: config/config.go:206-208] — BudgetWarnPct validation pattern
- [Source: config/defaults.yaml] — current defaults (max_turns: 50)

## Testing Standards

- Table-driven tests с `[]struct{name string; ...}` + `t.Run`
- Go stdlib assertions (`if got != want { t.Errorf }`) — без testify
- Error tests: `strings.Contains(err.Error(), "expected substring")` — не bare `err != nil`
- Каждый validation error case: проверить точный текст сообщения
- Naming: `TestConfig_Validate_ContextWarnPctRange`, `TestConfig_Validate_ContextCriticalOrdering`
- `t.TempDir()` для тестов с файлами
- `errors.As(err, &target)` для custom error types, не type assertions
- Coverage: config package >80%

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used
Claude Opus 4.6

### Debug Log References

### Completion Notes List
- Added ContextWarnPct and ContextCriticalPct fields to Config struct after TaskBudgetMaxUSD
- Updated defaults.yaml: max_turns 50→15, added context_warn_pct: 55, context_critical_pct: 65
- Added 3 validation checks in Validate(): range for warn (1-99), range for critical (1-99), ordering (critical > warn)
- Added 6 new table-driven test cases in TestConfig_Validate_Errors
- Updated TestConfig_Load_DefaultsComplete with new field assertions and MaxTurns=15
- Updated all existing tests referencing MaxTurns=50 embedded default to 15
- Added ContextWarnPct/ContextCriticalPct to all test Config structs that call Validate()
- All tests pass (config, runner, session, bridge, gates, cmd/ralph)

### File List
- config/config.go (modified: added ContextWarnPct, ContextCriticalPct fields + validation)
- config/defaults.yaml (modified: max_turns 50→15, added context_warn_pct, context_critical_pct)
- config/config_test.go (modified: new validation tests, updated defaults assertions, MaxTurns 50→15)
