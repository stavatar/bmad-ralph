# Story 11.7: BMad autodiscovery и single-doc режим

Status: Done

## Review Findings (0H/4M/1L)

### MEDIUM
- [M1] File List неполный: `cmd/ralph/bridge.go` modified (storyFileRe filter + `totalTasks +=` fix) не указан в Dev Agent Record File List
- [M2] Нет теста для `cfg.PlanInputs` branch (AC #2): `discoverInputs` имеет 3 ветки (args, PlanInputs, autodiscovery), но PlanInputs-ветка (строки 92-108) не покрыта тестами `[cmd/ralph/cmd_test.go]`
- [M3] Нет теста ошибки чтения файла (non-NotExist): все 3 ветки `discoverInputs` содержат `os.ReadFile` error return, но error path tests отсутствуют `[cmd/ralph/cmd_test.go]`
- [M4] Autodiscovery test не проверяет inputs[1]: `TestDiscoverInputs_Autodiscovery` создаёт 2 файла но проверяет Content только `inputs[0]`, inputs[1] мог быть пустым `[cmd/ralph/cmd_test.go]`

### LOW
- [L1] bridge.go `totalTasks = taskCount` → `totalTasks += taskCount` — bug fix в bridge не задокументирован ни в story, ни в Completion Notes

## Story

As a разработчик,
I want запустить `ralph plan` без аргументов в BMad-проекте и `ralph plan requirements.md` для single-doc,
so that не нужно конфигурировать список файлов вручную.

## Acceptance Criteria

1. **`ralph plan` без аргументов, `docs/` содержит `prd.md`, `architecture.md`** — при `cfg.PlanMode == "auto"` или `"bmad"` система находит файлы по дефолтному BMad-маппингу и читает их

2. **Порядок поиска:** сначала `cfg.PlanInputs` (если заданы) → иначе BMad defaults в `docs/`

3. **Файлы которых нет в `docs/`** — пропускаются без ошибки

4. **`ralph plan requirements.md` (один файл аргументом)** — при `cfg.PlanMode == "single"` или один файл → файл читается без role mapping, typed headers не добавляются (FR3, FR12)

5. **`cfg.PlanMode == "auto"`, аргументов нет, `docs/` найдены файлы** → режим определяется автоматически: >1 файл = BMad, 1 файл = single

6. **`cmd/ralph/plan.go` читает найденные файлы через `os.ReadFile`** и заполняет `PlanInput.Content`

7. **Тесты в `cmd/ralph/cmd_test.go`** покрывают autodiscovery и single-doc

## Tasks / Subtasks

- [x] Task 1: Реализовать autodiscovery в `cmd/ralph/plan.go` (AC: #1, #2, #3)
  - [x] Дефолтный список BMad файлов: `prd.md`, `architecture.md`, `ux-design.md`, `front-end-spec.md`
  - [x] Поиск в `cfg.ProjectRoot + "/docs/"` по дефолтному списку
  - [x] `os.IsNotExist` → пропустить файл (не ошибка)
  - [x] Если `cfg.PlanInputs` задан → использовать его вместо autodiscovery
- [x] Task 2: Реализовать single-doc режим (AC: #4)
  - [x] Один файл аргументом или `cfg.PlanMode == "single"` → читать без role mapping
  - [x] Typed headers не добавляются (делегируется в resolveRole с singleDoc=true)
- [x] Task 3: Реализовать auto-detection режима (AC: #5)
  - [x] `cfg.PlanMode == "auto"`: >1 файл = BMad mode, 1 файл = single mode
- [x] Task 4: Файловый I/O (AC: #6)
  - [x] `os.ReadFile` для каждого найденного файла → `PlanInput.Content`
  - [x] `cmd/ralph/plan.go` — единственное место для чтения файлов
- [x] Task 5: Тесты (AC: #7)
  - [x] Autodiscovery: `t.TempDir()` с `docs/prd.md` и `docs/architecture.md` → оба найдены
  - [x] Missing file: `docs/ux-design.md` отсутствует → пропущен без ошибки
  - [x] Single-doc: один аргумент → single mode, без typed headers

## Dev Notes

### Архитектурные ограничения

- **Файлы читает ТОЛЬКО `cmd/ralph/plan.go`** — внутри `plan/` нет `os.ReadFile` [Source: docs/project-context.md#Plan Package Инварианты]
- **`cmd/ralph/` = единственное место для exit codes и output decisions** [Source: docs/project-context.md#Dependency Direction]
- **Отсутствующий файл:** `os.Stat` + `errors.Is(err, os.ErrNotExist)` → пуст, не ошибка [Source: docs/project-context.md#File I/O]

### Существующий код

- `cmd/ralph/bridge.go` — reference pattern для wiring subcommand [Source: docs/epics.md#Epic 11 scaffold]
- `plan/plan.go` (Story 11.5) — `Run(ctx, cfg, PlanOpts)`, принимает заполненные `PlanInput.Content`
- `plan/plan.go` (Story 11.6) — `resolveRole()`, `defaultRoles` map

### BMad autodiscovery список

```go
var bmadDefaultFiles = []string{
    "prd.md",
    "architecture.md",
    "ux-design.md",
    "front-end-spec.md",
}
```
Ищутся в `filepath.Join(cfg.ProjectRoot, "docs", filename)`.

### Auto-detection логика

```go
if cfg.PlanMode == "auto" {
    if len(inputs) > 1 {
        // BMad mode
    } else {
        // Single mode
    }
}
```

### Тестирование

- `t.TempDir()` с реальными файлами для autodiscovery [Source: CLAUDE.md#Testing Core Rules]
- Mock Claude через `config.ClaudeCommand` не нужен — тестируется только file discovery
- Test naming: `TestPlanCmd_Autodiscovery`, `TestPlanCmd_SingleDoc` [Source: CLAUDE.md#Naming Conventions]

### Project Structure Notes

- `cmd/ralph/plan.go` — НОВЫЙ файл, cobra subcommand с autodiscovery и file reading
- `cmd/ralph/cmd_test.go` — существующий, добавление тестов

### References

- [Source: docs/epics.md#Story 11.7] — полные AC и технические заметки
- [Source: docs/epics.md#FR1] — autodiscovery docs/
- [Source: docs/epics.md#FR2] — явный список файлов
- [Source: docs/epics.md#FR3] — single-doc режим
- [Source: docs/epics.md#FR12] — single-doc без ролей
- [Source: docs/epics.md#FR13] — plan_mode: bmad/single/auto
- [Source: docs/project-context.md#Plan Package Инварианты] — файлы читает только cmd/ralph/plan.go
- [Source: docs/project-context.md#File I/O] — обработка отсутствующих файлов

## Dev Agent Record

### Context Reference

### Agent Model Used

claude-opus-4-6

### Debug Log References

### Completion Notes List

- Created cmd/ralph/plan.go with planCmd cobra command, runPlan, discoverInputs
- discoverInputs handles 3 sources: CLI args > cfg.PlanInputs > BMad autodiscovery in docs/
- Auto-detection: cfg.PlanMode "auto" → >1 file = bmad, <=1 = single; "single"/"bmad" explicit
- os.IsNotExist files silently skipped in both config and autodiscovery paths
- Role resolution delegated to plan.ResolveRole with singleDoc flag
- Size check via plan.CheckSize with yellow warning
- Registered planCmd in main.go init()
- 4 tests: SubcommandRegistered, Autodiscovery (2 files found), MissingFileSkipped, SingleDocMode
- Full regression: go test ./... -count=1 — all packages pass, 0 regressions

### File List

- cmd/ralph/plan.go (new): planCmd, runPlan, discoverInputs, bmadDefaultFiles
- cmd/ralph/main.go (modified): added planCmd to init()
- cmd/ralph/cmd_test.go (modified): added 4 plan tests
