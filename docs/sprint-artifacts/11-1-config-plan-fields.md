# Story 11.1: Config — расширение полями plan/

Status: Done

## Review Findings (0H/3M/2L)

### MEDIUM
- [M1] Magic strings для plan defaults — нет констант в `config/constants.go` `[config/config.go:99,105]`
- [M2] PlanMode не валидируется в `Validate()` — допустимые значения "bmad"|"single"|"auto" не проверяются `[config/config.go]`
- [M3] File List не включает `docs/project-context.md` — значительные изменения не задокументированы

### LOW
- [L1] `plan/plan.go` не имеет тестов — нет `TestPlanInput_ZeroValue` `[plan/]`
- [L2] Plan-поля в Config struct вставлены без группирующего комментария `[config/config.go:58-62]`

## Story

As a разработчик,
I want настраивать поведение `ralph plan` через `ralph.yaml`,
so that я могу переопределять входные файлы, путь output и режим планирования без изменения кода.

## Acceptance Criteria

1. **Config struct расширен plan-полями** — `config/config.go` содержит:
   ```go
   PlanInputs     []PlanInputConfig `yaml:"plan_inputs"`
   PlanOutputPath string            `yaml:"plan_output_path"`
   PlanMaxRetries int               `yaml:"plan_max_retries"`
   PlanMerge      bool              `yaml:"plan_merge"`
   PlanMode       string            `yaml:"plan_mode"` // "bmad" | "single" | "auto"
   ```
   С типом:
   ```go
   type PlanInputConfig struct {
       File string `yaml:"file"`
       Role string `yaml:"role"`
   }
   ```

2. **Defaults применяются в `config.Load()`** — если поля zero value после `yaml.Unmarshal`:
   - `PlanOutputPath` → `"docs/sprint-tasks.md"`
   - `PlanMaxRetries` → `1`
   - `PlanMode` → `"auto"`

3. **Тесты в `config/config_test.go`** — happy path (все поля из YAML) + defaults (поля не заданы → default значения)

4. **`go test ./config/... -count=1` проходит**

5. **Stub файл `plan/plan.go` создан** с типом:
   ```go
   type PlanInput struct {
       File    string // путь к файлу
       Role    string // семантическая роль
       Content []byte // содержимое — заполняется в cmd/ralph/plan.go
   }
   ```

## Tasks / Subtasks

- [x] Task 1: Добавить `PlanInputConfig` struct в `config/config.go` (AC: #1)
  - [x] Определить `PlanInputConfig` struct с полями `File`, `Role` и yaml-тегами
  - [x] Добавить plan-поля в `Config` struct: `PlanInputs`, `PlanOutputPath`, `PlanMaxRetries`, `PlanMerge`, `PlanMode`
- [x] Task 2: Реализовать defaults в `config.Load()` (AC: #2)
  - [x] После `yaml.Unmarshal` проверить zero-value и подставить defaults для `PlanOutputPath`, `PlanMaxRetries`, `PlanMode`
  - [x] Паттерн аналогичен существующим defaults (см. `defaultConfig()` и cascade)
- [x] Task 3: Добавить тесты в `config/config_test.go` (AC: #3, #4)
  - [x] Тест happy path: YAML с явными plan-полями → все распарсены корректно
  - [x] Тест defaults: YAML без plan-полей → `PlanOutputPath == "docs/sprint-tasks.md"`, `PlanMaxRetries == 1`, `PlanMode == "auto"`
  - [x] Добавить plan-поля в существующий all-fields тест (если есть)
- [x] Task 4: Создать `plan/plan.go` stub (AC: #5)
  - [x] Создать директорию `plan/`
  - [x] Создать `plan/plan.go` с `package plan` и типом `PlanInput`

## Dev Notes

### Архитектурные ограничения

- **Config immutability:** `config.Config` парсится один раз, передаётся by pointer, НИКОГДА не мутируется в runtime [Source: docs/project-context.md#Config Immutability]
- **Dependency direction:** `config` = leaf package, не зависит ни от кого. `plan/` зависит от `config`, но не наоборот [Source: docs/project-context.md#Dependency Direction]
- **Exit codes:** ТОЛЬКО в `cmd/ralph/`. Packages возвращают errors [Source: docs/project-context.md#Error Handling]
- **3 direct deps only:** Никаких новых зависимостей не требуется для этой story [Source: docs/project-context.md#Technology Stack]

### Существующий код

- `config/config.go` содержит `Config` struct (55 строк, ~33 поля) с `yaml` тегами, `CLIFlags` struct, `defaultConfig()`, `Load()` [Source: config/config.go]
- Defaults cascade: `defaultConfig()` из embedded `defaults.yaml` → user YAML → CLI flags override [Source: config/config.go]
- Defaults для новых plan-полей нужно добавить ПОСЛЕ `yaml.Unmarshal` в `Load()` — аналогично существующему паттерну для zero-value check
- `PlanInputConfig` — config-тип (yaml); `PlanInput` — plan-тип (runtime, с Content) — различные struct для разных слоёв
- `plan/plan.go` создаётся как stub — функция `Run` добавляется в Story 11.5

### Паттерн defaults

Посмотреть существующий `Load()` в `config/config.go` — после unmarshal есть секция defaults assignment с проверкой zero value. Добавить аналогичную проверку для plan-полей:
```go
if cfg.PlanOutputPath == "" {
    cfg.PlanOutputPath = "docs/sprint-tasks.md"
}
if cfg.PlanMaxRetries == 0 {
    cfg.PlanMaxRetries = 1
}
if cfg.PlanMode == "" {
    cfg.PlanMode = "auto"
}
```

### Тестирование

- Table-driven по умолчанию, Go stdlib assertions (без testify) [Source: docs/project-context.md#Testing]
- `t.TempDir()` для изоляции [Source: docs/project-context.md#Testing]
- Golden files с `-update` flag если нужно [Source: docs/project-context.md#Testing]
- Тест naming: `Test<Type>_<Method>_<Scenario>` [Source: CLAUDE.md#Naming Conventions]
- `errors.As(err, &target)` не type assertions [Source: CLAUDE.md#Testing Core Rules]
- Добавить plan-поля в существующий all-fields override тест [Source: CLAUDE.md#Testing Core Rules]

### Project Structure Notes

- `config/config.go` — существующий файл, добавить struct и поля
- `config/config_test.go` — существующий файл, добавить тесты
- `plan/plan.go` — НОВЫЙ файл, новый пакет `plan`
- Структура `plan/` соответствует архитектуре: `cmd/ralph → plan → session, config` [Source: docs/project-context.md#Dependency Direction]

### References

- [Source: docs/epics.md#Story 11.1] — полные AC и технические заметки
- [Source: docs/project-context.md#Config Immutability] — правила иммутабельности конфига
- [Source: docs/project-context.md#Plan Package] — архитектура plan/, типы, инварианты
- [Source: docs/project-context.md#Dependency Direction] — граф зависимостей пакетов
- [Source: config/config.go] — существующий Config struct (33 поля), Load(), CLIFlags
- [Source: CLAUDE.md#Architecture Rules] — dependency direction, exit codes, config immutability
- [Source: CLAUDE.md#Naming Conventions] — именование типов, ошибок, тестов
- [Source: CLAUDE.md#Testing Core Rules] — table-driven, stdlib assertions, coverage

## Dev Agent Record

### Context Reference

<!-- Path(s) to story context XML will be added here by context workflow -->

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Added `PlanInputConfig` struct and 5 plan fields to `Config` struct in `config/config.go`
- Created `applyPlanDefaults()` helper called on all 3 return paths in `Load()` for zero-value defaults
- Added plan fields to existing `TestConfig_Load_ValidFullConfig` all-fields test
- Added `TestConfig_Load_PlanFieldsDefaults` — verifies defaults when no plan YAML present
- Added `TestConfig_Load_PlanFieldsFromYAML` — verifies explicit plan YAML values parsed correctly
- Created `plan/plan.go` stub with `PlanInput{File, Role, Content}` type
- Full regression: `go test ./... -count=1` — all packages pass, 0 regressions

### File List

- config/config.go (modified): added PlanInputConfig type, 5 plan fields to Config, applyPlanDefaults()
- config/config_test.go (modified): added plan fields to all-fields test, 2 new test functions
- plan/plan.go (new): package plan stub with PlanInput type
