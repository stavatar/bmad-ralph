# bmad-ralph v2 — Epic Breakdown

**Author:** Степан
**Date:** 2026-03-07
**Project Level:** CLI Tool
**Target Scale:** Solo developer, open source

---

## Overview

Decomposition PRD + Architecture для ralph v2. Центральное изменение: замена `ralph bridge` новой командой `ralph plan`. Документ создан workflow BMad create-epics-and-stories.

| Epic | Название | Scope | Зависит от |
|------|----------|-------|-----------|
| Epic 11 | ralph plan — Core Generation | MVP | — |
| Epic 12 | ralph plan — AI Review & Gates | MVP | Epic 11 |
| Epic 13 | ralph plan — Merge Mode | MVP | — (параллельно с 12) |
| Epic 14 | Bridge Deprecation & Removal | MVP | Epic 11 |
| Epic 15 | Growth — Acceptance Test | Growth gate | Epic 14 |
| Epic 16 | Growth — Replan & Init | Growth | Epic 15 |

---

## Functional Requirements Inventory

| FR | Описание | Epic | Scope |
|----|----------|------|-------|
| FR1 | `ralph plan` в корне BMad → sprint-tasks.md из автодискавери docs/ | 11 | MVP |
| FR2 | Явный список файлов: `ralph plan file1.md file2.md` | 11 | MVP |
| FR3 | Single-doc режим: `ralph plan requirements.md` без BMad | 11 | MVP |
| FR4 | Merge mode `--merge` — добавляет задачи, не трогает [x] | 13 | MVP |
| FR5 | Merge пропускает stories с уже существующими задачами | 13 | MVP |
| FR6 | Флаг `--no-review` отключает AI review | 12 | MVP |
| FR7 | Флаг `--output <path>` — альтернативный путь output | 11 | MVP |
| FR8 | Флаг `--force` — продолжить при >50K токенов | 12 | MVP |
| FR9 | `plan_input_files` в ralph.yaml настраивает входные файлы с ролями | 11 | MVP |
| FR10 | Автоматическое назначение ролей по именам файлов (BMad defaults) | 11 | MVP |
| FR11 | Явный маппинг в config переопределяет дефолтный | 11 | MVP |
| FR12 | Single-doc режим игнорирует все роли | 11 | MVP |
| FR13 | `plan_mode: bmad/single/auto` в конфиге | 11 | MVP |
| FR14 | Auto review в чистой Claude-сессии после генерации | 12 | MVP |
| FR15 | Reviewer проверяет FR coverage, гранулярность, source-refs, SETUP/GATE, дубли | 12 | MVP |
| FR16 | Максимум 1 автоматический retry при issues | 12 | MVP |
| FR17 | Gate [p/e/q] если после retry проблемы остаются | 12 | MVP |
| FR18 | Progress-индикатор по фазам (генерация / review / retry) | 11 | MVP |
| FR19 | Summary после завершения: кол-во задач, статус review, путь файла | 11 | MVP |
| FR20 | Typed headers `<!-- file: <name> \| role: <role> -->` в промпте LLM | 11 | MVP |
| FR21 | Size warning >50K токенов, exit 2 (без --force) | 11 | MVP |
| FR22 | Actionable error при Claude API ошибке, exit 1 | 11 | MVP |
| FR23 | Deprecation warning при `ralph bridge` + bridge deletion | 14 | MVP |
| FR24 | Gate задачи в `ralph run` (существующий, без изменений) | — | Existing |
| FR25 | Накопление знаний из sprint (существующий run mode) | — | Existing |
| FR26 | Инжекция знаний в следующий цикл (существующий) | — | Existing |
| FR27 | `ralph distill` (существующий) | — | Existing |
| FR28 | `ralph replan` — пересчёт плана | 16 | Growth |
| FR29 | `ralph init "описание"` — быстрый старт без BMad | 16 | Growth |

---

## FR Coverage Map

_Полная матрица покрытия — см. [FR Coverage Matrix](#fr-coverage-matrix) в конце документа._

---

## Epic 11: ralph plan — Core Generation

**Scope:** FR1, FR2, FR3, FR7, FR9, FR10, FR11, FR12, FR13, FR18, FR19, FR20, FR21, FR22
**Stories:** 9
**Release milestone:** v1.0 (MVP)
**PRD:** [docs/prd.md](prd.md)
**Architecture:** [docs/architecture.md](architecture.md)

**Context:**
Центральное изменение v2: замена `ralph bridge` командой `ralph plan`. Разработчик запускает `ralph plan` и получает корректный `sprint-tasks.md` с typed headers за ≤20 минут без AI review. Первый рабочий `sprint-tasks.md` без единого ручного исправления source-ссылок.

**Dependency structure:**
```
11.1 Config fields + PlanInput stub ──────────────────────────────────────────────────┐
11.2 session.InjectFeedback ───────────────────────────────────────────────────────────┼──→ 11.5 plan.go core ──→ 11.6 role mapping ──→ 11.7 autodiscovery ──→ 11.8 CLI wiring ──→ 11.9 progress/summary
11.3 size.go (←11.1) — независим от 11.5                                              │
11.4 prompts/plan.md (←11.1) ─────────────────────────────────────────────────────────┘
```

**Existing scaffold:**
- `session/session.go` — Options struct (9 fields), Execute(), `flagAppendSystemPrompt` константа
- `config/config.go` — Config struct (25+ fields), Load(), defaults cascade, Validate()
- `config/defaults.yaml` — существующие дефолты
- `cmd/ralph/root.go` — cobra root, регистрация subcommands
- `cmd/ralph/bridge.go` — reference pattern для wiring subcommand

---

### Story 11.1: Config — расширение полями plan/

Как разработчик, я хочу настраивать поведение `ralph plan` через `ralph.yaml`, чтобы переопределять входные файлы, путь output и режим планирования.

**Acceptance Criteria:**

**Given** файл `config/config.go` с типом `Config`
**When** добавляются новые поля
**Then** `Config` содержит:
```go
PlanInputs     []PlanInputConfig `yaml:"plan_inputs"`
PlanOutputPath string            `yaml:"plan_output_path"`
PlanMaxRetries int               `yaml:"plan_max_retries"`
PlanMerge      bool              `yaml:"plan_merge"`
PlanMode       string            `yaml:"plan_mode"` // "bmad" | "single" | "auto"

type PlanInputConfig struct {
    File string `yaml:"file"`
    Role string `yaml:"role"`
}
```

**And** `config.Load()` применяет defaults если поля не заданы:
- `PlanOutputPath` → `"docs/sprint-tasks.md"`
- `PlanMaxRetries` → `1`
- `PlanMode` → `"auto"`

**And** все новые поля покрыты тестами в `config/config_test.go` (happy path + defaults)

**And** `go test ./config/... -count=1` проходит

**And** файл `plan/plan.go` создан с типом:
```go
type PlanInput struct {
    File    string // путь к файлу
    Role    string // семантическая роль
    Content []byte // содержимое — заполняется в cmd/ralph/plan.go
}
```

**Technical Notes:**
- Defaults в `config.Load()` после `yaml.Unmarshal` — если поле zero value, подставить default
- `PlanInputConfig` — config-тип (yaml); `PlanInput` — plan-тип (runtime, с Content)
- Новые поля добавить в `config/config_test.go` в существующий all-fields тест
- `plan/plan.go` создаётся как stub с типом `PlanInput` — функция `Run` добавляется в Story 11.5

**Prerequisites:** —

---

### Story 11.2: session.Options — поле InjectFeedback

Как система, я хочу передавать reviewer feedback в retry-сессию через поле `session.Options`, чтобы Claude получал контекст предыдущего review.

**Acceptance Criteria:**

**Given** `session/session.go`, тип `Options`
**When** добавляется поле `InjectFeedback string`
**Then** если `InjectFeedback != ""` → передаётся через `--append-system-prompt` флаг в Claude CLI

**And** если `InjectFeedback == ""` → флаг не передаётся (поведение без изменений)

**And** `session_test.go` содержит тест: `InjectFeedback` не пустой → `--append-system-prompt` присутствует в args

**And** `session_test.go` содержит тест: `InjectFeedback` пустой → `--append-system-prompt` отсутствует

**And** `go test ./session/... -count=1` проходит

**Technical Notes:**
- `flagAppendSystemPrompt = "--append-system-prompt"` уже определён в `session.go` как константа
- Добавить ветку в buildArgs (или аналогичную функцию): `if opts.InjectFeedback != "" { args = append(args, flagAppendSystemPrompt, opts.InjectFeedback) }`
- Тест: self-reexec mock pattern (существующий в проекте)

**Prerequisites:** —

---

### Story 11.3: plan/size.go — CheckSize pure function

Как система, я хочу проверять суммарный размер входных документов, чтобы предупреждать пользователя до отправки в Claude.

**Acceptance Criteria:**

**Given** функция `CheckSize(inputs []PlanInput) (warn bool, msg string)`
**When** сумма `len(inp.Content)` по всем inputs ≤ `MaxInputBytes` (100_000)
**Then** `warn == false`, `msg == ""`

**When** сумма > `MaxInputBytes`
**Then** `warn == true`, `msg` содержит суммарный размер в KB и рекомендацию `bmad shard-doc`

**And** `plan/size.go` содержит `const MaxInputBytes = 100_000`

**And** `plan/size_test.go` покрывает: below threshold, exactly threshold, above threshold, empty inputs

**And** `go test ./plan/... -count=1` проходит

**Technical Notes:**
- Pure function: без I/O, без зависимостей кроме stdlib
- `msg` формат: `"суммарный размер входных документов: %dKB (лимит ~100KB). Рекомендуется разбить через 'bmad shard-doc'"`
- Threshold считается по `len(inp.Content)`, не по размеру файла на диске

**Prerequisites:** Story 11.1 (создаёт `plan/plan.go` stub с типом `PlanInput`)

---

### Story 11.4: plan/prompts/plan.md — generator prompt

Как система, я хочу иметь промпт для генератора sprint-tasks.md, чтобы Claude получал чёткие инструкции по созданию задач из входных документов.

**Acceptance Criteria:**

**Given** файл `plan/prompts/plan.md` как `text/template`
**Then** шаблон использует переменные: `{{.Inputs}}`, `{{.OutputPath}}`, `{{.Merge}}`, `{{.Existing}}`

**And** промпт содержит инструкции:
- Как интерпретировать typed headers `<!-- file: <name> | role: <role> -->`
- Формат задач sprint-tasks.md (SETUP, GATE, обычные задачи)
- Требование source-ссылок с реальными именами файлов из typed headers
- Инструкция для merge mode (если `{{.Merge}}`)

**And** `plan/plan_test.go` содержит `TestPlanPrompt_Generate` с golden file `plan/testdata/TestPlanPrompt_Generate.golden`

**And** тест проходит с `-update` для регенерации golden и без `-update` для валидации

**Technical Notes:**
- User content (содержимое файлов) вставляется через `strings.Replace` ПОСЛЕ `template.Execute` — не через шаблонные переменные
- Placeholder для content: `__CONTENT_N__` где N — индекс PlanInput
- Merge секция: `{{if .Merge}}...{{end}}`

**Prerequisites:** Story 11.1

---

### Story 11.5: plan/plan.go — core generate flow

Как разработчик, я хочу вызвать `plan.Run(ctx, cfg, opts)` и получить готовый `sprint-tasks.md`, чтобы перейти к `ralph run` без ручных правок.

**Acceptance Criteria:**

**Given** `plan.Run(ctx, cfg, PlanOpts{Inputs, OutputPath})` вызван с валидными входными данными
**When** генерация завершается успешно (Claude exit 0)
**Then** `sprint-tasks.md` записан по `opts.OutputPath` через `writeAtomic()` (temp+rename)

**And** если `opts.OutputPath` пустой → используется `cfg.PlanOutputPath`

**And** если генерация падает (Claude exit != 0) → возвращается `fmt.Errorf("plan: generate: %w", err)`, файл не изменён

**And** `plan_test.go` содержит scenario-based тест через `config.ClaudeCommand` mock:
  - сценарий "generate_success" → файл записан корректно
  - сценарий "generate_failure" → ошибка, файл не создан

**And** `go test ./plan/... -count=1` проходит

**Technical Notes:**
- `writeAtomic(path string, data []byte) error` — unexported helper в `plan/plan.go`
- `templateData` struct в `plan/plan.go` (unexported): `Inputs []PlanInput, OutputPath string, Merge bool, Existing string`
- User content вставляется через `strings.Replace` после `template.Execute`
- `plan/` не читает файлы — `inp.Content` заполняется в `cmd/ralph/plan.go`

**Prerequisites:** Stories 11.1, 11.2, 11.4

---

### Story 11.6: Role mapping и typed headers

Как система, я хочу автоматически назначать роли входным файлам по именам и передавать их LLM через typed headers, чтобы Claude понимал семантику каждого документа.

**Acceptance Criteria:**

**Given** входной файл `prd.md` без явной роли в config
**Then** система назначает роль `requirements`

**And** маппинг по умолчанию:
```
prd.md           → requirements
architecture.md  → technical_context
ux-design.md     → design_context
front-end-spec.md → ui_spec
```

**And** явная роль в `cfg.PlanInputs[].Role` переопределяет дефолт (FR11)

**And** в single-doc режиме (`cfg.PlanMode == "single"`) роли не назначаются и typed headers не добавляются (FR12)

**And** промпт для Claude содержит typed headers формата:
```
<!-- file: prd.md | role: requirements -->
<content здесь>
<!-- end file: prd.md -->
```

**And** `plan_test.go` содержит тесты:
- BMad autodiscovery: `prd.md` → role `requirements`
- Explicit override: custom role в config → custom role в header
- Single-doc: нет typed headers в промпте

**Technical Notes:**
- `defaultRoles` map[string]string в `plan/plan.go` (unexported)
- Функция `resolveRole(filename string, explicitRole string, singleDoc bool) string`
- Typed headers вставляются как часть user content, не через template

**Prerequisites:** Story 11.5

---

### Story 11.7: BMad autodiscovery и single-doc режим

Как разработчик, я хочу запустить `ralph plan` без аргументов в BMad-проекте и `ralph plan requirements.md` для single-doc, чтобы не конфигурировать список файлов вручную.

**Acceptance Criteria:**

**Given** `ralph plan` без аргументов, `docs/` содержит `prd.md`, `architecture.md`
**When** `cfg.PlanMode == "auto"` или `"bmad"`
**Then** система находит файлы по дефолтному BMad-маппингу и читает их

**And** порядок поиска: сначала `cfg.PlanInputs` (если заданы) → иначе BMad defaults в `docs/`

**And** файлы которых нет в `docs/` — пропускаются без ошибки

**Given** `ralph plan requirements.md` (один файл аргументом)
**When** `cfg.PlanMode == "single"` или один файл
**Then** файл читается без role mapping, typed headers не добавляются (FR3, FR12)

**Given** `cfg.PlanMode == "auto"`, аргументов нет, `docs/` найдены файлы
**Then** режим определяется автоматически как BMad (>1 файл = BMad, 1 файл = single)

**And** `cmd/ralph/plan.go` читает найденные файлы через `os.ReadFile` и заполняет `PlanInput.Content`

**And** тесты в `cmd/ralph/cmd_test.go` покрывают autodiscovery и single-doc

**Technical Notes:**
- Autodiscovery ищет файлы в `cfg.ProjectRoot + "/docs/"` по дефолтному списку
- Если файл не найден через `os.IsNotExist` → просто пропустить (не ошибка)
- `cmd/ralph/plan.go` — единственное место где файлы читаются через `os.ReadFile`

**Prerequisites:** Stories 11.5, 11.6

---

### Story 11.8: cmd/ralph/plan.go — CLI wiring

Как разработчик, я хочу использовать все флаги `ralph plan` из командной строки, чтобы управлять поведением без редактирования конфига.

**Acceptance Criteria:**

**Given** `ralph plan --help`
**Then** показаны флаги: `--output`, `--no-review`, `--merge`, `--force`, `--input`

**And** `--output <path>` переопределяет `cfg.PlanOutputPath`

**And** `--input "file:role"` (повторяемый) добавляет входные файлы, переопределяя config (FR9)
- Формат: `"filepath:role"` → `PlanInput{File: "filepath", Role: "role"}`
- Только `"filepath"` (без `:`) → `PlanInput{File: "filepath", Role: ""}` (дефолтный role lookup)

**And** `cmd_test.go` содержит тесты флагов: `--output` маппит в правильное поле, `--input` парсит `"file:role"` корректно

**And** при ошибке `plan.Run` → `cmd/ralph/plan.go` возвращает `config.ExitCodeError{Code: 1}` (не panic)

**And** при size warning без `--force` → `config.ExitCodeError{Code: 2}`

**And** при gate quit → `config.ExitCodeError{Code: 3}`

**Technical Notes:**
- `cobra.StringArrayVar` для `--input` (каждый `--input` = отдельный элемент)
- Парсинг `"file:role"`: `strings.SplitN(s, ":", 2)`
- `os.Exit` только через `config.ExitCodeError` в `cmd/ralph/`

**Prerequisites:** Stories 11.5, 11.7

---

### Story 11.9: Progress output и summary

Как разработчик, я хочу видеть progress-индикатор во время `ralph plan`, чтобы знать что происходит и не думать что процесс завис.

**Acceptance Criteria:**

**Given** `ralph plan` запущен
**When** начинается генерация
**Then** stdout показывает: `⏳ Генерация плана...`

**When** генерация завершена успешно
**Then** stdout показывает: `✓ Генерация плана... (Ns)`

**When** завершён весь процесс
**Then** stdout показывает summary:
```
→ sprint-tasks.md готов: 47 задач | path/to/sprint-tasks.md
```

**And** при size warning:
```
⚠ Суммарный размер входных документов: 120KB (лимит ~100KB)
  Рекомендуется разбить через 'bmad shard-doc'
  Используйте --force для продолжения
```

**And** при Claude API ошибке: actionable сообщение с exit 1 (FR22)

**And** `fatih/color` используется для `✓` (зелёный), `⚠` (жёлтый), `→` (cyan)

**And** тест в `cmd_test.go` проверяет наличие summary строки в stdout

**Technical Notes:**
- Progress через `fmt.Fprintf(os.Stdout, ...)` в `cmd/ralph/plan.go`
- Подсчёт задач: `strings.Count(output, "- [ ]") + strings.Count(output, "- [x]")`
- Timing: `time.Since(start)` округлённый до секунды

**Prerequisites:** Story 11.8

---

## Epic 12: ralph plan — AI Review & Gates

**Scope:** FR6, FR8, FR14, FR15, FR16, FR17
**Stories:** 4
**Release milestone:** v1.0 (MVP)
**PRD:** [docs/prd.md](prd.md)
**Architecture:** [docs/architecture.md](architecture.md)

**Context:**
После генерации план автоматически проверяется AI-reviewer в отдельной чистой Claude-сессии. Разработчик уверен в качестве плана без ручного review — reviewer обнаруживает проблемы до `ralph run`.

**Зависит от:** Epic 11

**Dependency structure:**
```
11.4 prompts/plan.md ──────────────────────→ 12.1 review.md
11.5 plan.go core ──┐
11.2 InjectFeedback ┤
12.1 review.md ─────┴──→ 12.2 review flow + gate ──→ 12.3 retry с InjectFeedback
11.8 CLI wiring ────┐
11.3 size.go ───────┼──→ 12.4 --no-review + --force
12.2 review flow ───┘
```

**Existing scaffold:**
- `plan/plan.go` — Run(), writeAtomic(), PlanInput, PlanOpts (после Epic 11)
- `session/session.go` — Options.InjectFeedback (после Story 11.2)
- `gates/gates.go` — gates.Prompt() pattern (reference для gate [p/e/q])
- `plan/prompts/plan.md` — generator prompt (после Story 11.4)

---

### Story 12.1: plan/prompts/review.md — reviewer prompt

Как система, я хочу иметь промпт для reviewer, чтобы Claude в чистой сессии объективно оценивал качество сгенерированного плана.

**Acceptance Criteria:**

**Given** файл `plan/prompts/review.md` как `text/template`
**Then** промпт инструктирует reviewer проверить:
- Покрытие всех FR из входных документов
- Гранулярность задач (не слишком крупные, не слишком мелкие)
- Корректность source-ссылок (реальные имена файлов из typed headers)
- Наличие SETUP и GATE задач
- Отсутствие дублирования и противоречий

**And** промпт требует ответ в формате: `OK` или `ISSUES:\n<список проблем>`

**And** `plan/plan_test.go` содержит `TestPlanPrompt_Review` с golden file `plan/testdata/TestPlanPrompt_Review.golden`

**And** тест проходит с `-update` и без

**Technical Notes:**
- Шаблон принимает `templateData` (те же поля что plan.md)
- Reviewer получает сгенерированный план как часть промпта (через strings.Replace)
- Формат ответа строго определён — gate parsing зависит от него

**Prerequisites:** Story 11.4

---

### Story 12.2: Review flow — чистая сессия и gate

Как система, я хочу автоматически запускать reviewer в отдельной Claude-сессии после генерации, чтобы получить объективную оценку плана.

**Acceptance Criteria:**

**Given** генерация успешно завершена (Story 11.5)
**When** reviewer сессия запускается
**Then** `session.Execute` вызывается с новыми `session.Options` без Resume (чистый контекст)

**And** stdout review-сессии парсится:
- Начинается с `OK` → gate = proceed, план принимается
- Начинается с `ISSUES:` → gate = edit, reviewer feedback сохраняется для retry

**And** при gate = proceed: `writeAtomic` записывает файл, `plan.Run` возвращает `nil`

**And** прогресс в stdout: `⏳ AI review плана...` → `✓ AI review плана... (Ns)`

**And** тест `plan_test.go` через mock сценарии:
- `review_ok` → файл записан, нет retry
- `review_issues` → retry triggered (Story 12.3)

**Technical Notes:**
- Review сессия = отдельный `session.Execute` вызов, не продолжение generate сессии
- Gate parsing: `strings.HasPrefix(strings.TrimSpace(output), "OK")`
- Feedback для retry: весь stdout review-сессии при `ISSUES:`

**Prerequisites:** Stories 11.5, 12.1

---

### Story 12.3: Retry logic с InjectFeedback

Как система, я хочу автоматически перегенерировать план с feedback от reviewer, чтобы исправить обнаруженные проблемы без участия пользователя.

**Acceptance Criteria:**

**Given** reviewer вернул `ISSUES:` (Story 12.2)
**When** выполняется retry
**Then** `session.Execute` вызывается с `session.Options{InjectFeedback: reviewerOutput}` (Story 11.2)

**And** прогресс: `⏳ Retry генерации с feedback...` → `✓ Retry завершён (Ns)`

**And** результат retry снова проходит review (ещё один `session.Execute` без InjectFeedback)

**And** если retry-review вернул `OK` → план принят, файл записан

**And** если retry-review вернул `ISSUES:` → показывается report и gate [p/e/q] (FR17):
```
⚠ Review выявил проблемы после retry:
<issues текст>
[p] proceed — принять план как есть
[e] edit    — открыть файл в редакторе
[q] quit    — выйти (exit 3)
```

**And** максимум 1 retry: после retry review не повторяется второй раз (max 3 сессии total)

**And** тест `plan_test.go`:
- `retry_success` → retry OK, файл записан
- `retry_fail_gate_proceed` → gate p → файл записан
- `retry_fail_gate_quit` → gate q → `error` возвращён

**Technical Notes:**
- Gate input читается из `os.Stdin`; в тестах подменяется через mock
- `[e] edit` — не реализован в MVP, показывается как опция но может быть no-op с сообщением

**Prerequisites:** Stories 11.2, 12.2

---

### Story 12.4: Флаги --no-review и --force

Как разработчик, я хочу отключить review или игнорировать size warning через флаги, чтобы использовать ralph plan в CI/CD без интерактивных вопросов.

**Acceptance Criteria:**

**Given** `ralph plan --no-review`
**Then** review-сессия не запускается, план записывается сразу после генерации (FR6)

**And** summary показывает: `→ sprint-tasks.md готов: 47 задач (review пропущен)`

**Given** `ralph plan --force` при документах >100KB
**Then** size warning выводится на stdout но процесс продолжается с exit 0 (FR8)

**Given** `ralph plan` без `--force` при документах >100KB
**Then** процесс останавливается с exit 2, actionable сообщение с `--force` подсказкой

**And** тесты в `cmd_test.go` и `plan_test.go`:
- `--no-review`: review session не вызывается (mock call count = 0)
- `--force`: процесс продолжается после size warning
- без `--force`: exit 2

**Technical Notes:**
- `PlanOpts.NoReview bool` поле для передачи флага в `plan.Run`
- `plan.Run` проверяет `opts.NoReview` перед запуском review сессии
- `--force` влияет только на size check, не на другие валидации

**Prerequisites:** Stories 11.8, 11.3, 12.2

---

## Epic 13: ralph plan — Merge Mode

**Scope:** FR4, FR5
**Stories:** 2
**Release milestone:** v1.0 (MVP, параллельно с Epic 12)
**PRD:** [docs/prd.md](prd.md)
**Architecture:** [docs/architecture.md](architecture.md)

**Context:**
Разработчик добавляет новые задачи к частично выполненному `sprint-tasks.md` без потери прогресса. Середина спринта — клиент попросил фичу. Добавляешь задачи без пересоздания плана с нуля. Независим от Epic 12 — может разрабатываться параллельно.

**Зависит от:** — (параллельно с Epic 12)

**Dependency structure:**
```
11.1 (PlanInput stub) ──→ 13.1 MergeInto pure function
11.8 CLI wiring ─────────────────────────────────────→ 13.2 merge integration (←13.1)
13.1 MergeInto ─────────────────────────────────────↗
```

**Existing scaffold:**
- `plan/plan.go` — Run(), PlanOpts, writeAtomic() (после Epic 11)
- `plan/plan_test.go` — mock pattern через config.ClaudeCommand (reference)
- `bridge/bridge.go` — reference реализации merge (Smart Merge из Epic 2, Story 2.6)

---

### Story 13.1: plan/merge.go — MergeInto pure function

Как система, я хочу иметь функцию для слияния нового плана с существующим, чтобы атомарно добавлять только новые задачи.

**Acceptance Criteria:**

**Given** `MergeInto(existing, generated []byte) ([]byte, error)`
**When** `existing` содержит story `## Story 1.1`, `generated` тоже содержит `## Story 1.1`
**Then** результат содержит `## Story 1.1` ровно один раз (дедупликация по заголовку, FR5)

**When** `generated` содержит `## Story 2.1` которого нет в `existing`
**Then** результат содержит все задачи из `existing` плюс новые задачи из `## Story 2.1` в конце

**And** выполненные задачи `- [x]` из `existing` не модифицируются (FR4)

**And** порядок: весь `existing` → новые stories из `generated` (append-only)

**And** `plan/merge_test.go` покрывает:
- Дедупликация story: дубль пропускается
- Новая story: добавляется в конец
- Пустой existing: результат = generated
- Пустой generated: результат = existing
- Выполненные [x] задачи остаются нетронутыми

**And** `go test ./plan/... -count=1` проходит

**Technical Notes:**
- Pure function: без I/O, без зависимостей кроме `bytes`/`strings`
- Дедупликация по regex `^## Story \d+\.\d+` (заголовки story)
- Append-only инвариант: никогда не удалять строки из `existing`

**Prerequisites:** Story 11.1 (создаёт `plan/plan.go` stub с типом `PlanInput`)

---

### Story 13.2: Merge mode интеграция

Как разработчик, я хочу запустить `ralph plan --merge` и получить обновлённый `sprint-tasks.md` с новыми задачами, не теряя выполненные.

**Acceptance Criteria:**

**Given** `ralph plan --merge` при существующем `sprint-tasks.md`
**When** генерация завершается
**Then** `cmd/ralph/plan.go` читает существующий файл → передаёт как `PlanInput` с ролью `existing_plan`

**And** `plan.Run` вызывает `MergeInto(existing, generated)` вместо прямой записи generated

**And** результат записывается через `writeAtomic` (атомарно)

**And** при `--merge` без существующего файла → merge как создание нового (не ошибка)

**And** summary: `→ sprint-tasks.md обновлён: +12 новых задач | path/to/sprint-tasks.md`

**And** тест через mock сценарий `merge_mode`: проверяет что `MergeInto` вызван, [x] задачи не модифицированы

**Technical Notes:**
- `PlanOpts.Merge bool` — передаётся в `plan.Run`
- `plan.Run` при `opts.Merge == true` читает existing через `os.ReadFile(opts.OutputPath)` — единственное исключение из правила "plan/ не читает файлы"
- Подсчёт новых задач: разница `strings.Count` между existing и result

**Prerequisites:** Stories 11.8, 13.1

---

## Epic 14: Bridge Deprecation & Removal

**Scope:** FR23, NFR19
**Stories:** 2
**Release milestone:** v1.0 (MVP gate, после Epic 11 CI green)
**PRD:** [docs/prd.md](prd.md)
**Architecture:** [docs/architecture.md](architecture.md)

**Context:**
`ralph bridge` удаляется полностью — -2844 строк мёртвого кода. Выполняется в два шага: сначала deprecation warning (пользователь видит инструкцию по миграции), затем отдельный коммит с полным удалением bridge после CI green.

**Зависит от:** Epic 11 (CI green)

**Dependency structure:**
```
14.1 deprecation warning (независима) ──→ 14.2 bridge deletion (требует Epic 11 CI green)
```

**Existing scaffold:**
- `cmd/ralph/bridge.go` — существующий bridge subcommand (удаляется в 14.2)
- `bridge/bridge.go`, `bridge/bridge_test.go`, `bridge/prompt_test.go` (удаляются в 14.2)
- `bridge/prompts/bridge.md`, `bridge/testdata/` (удаляются в 14.2)
- `cmd/ralph/root.go` — регистрация bridge subcommand (удаляется в 14.2)

---

### Story 14.1: Deprecation warning для ralph bridge

Как разработчик, я хочу видеть deprecation warning при вызове `ralph bridge`, чтобы знать о миграции на `ralph plan` и иметь готовую команду для copy-paste.

**Acceptance Criteria:**

**Given** пользователь запускает `ralph bridge [аргументы]`
**Then** stdout показывает:
```
⚠ ralph bridge устарел и будет удалён в следующем релизе.
  Используйте: ralph plan [файлы...]
  Документация: docs/architecture.md
```

**And** после warning `ralph bridge` продолжает работать как прежде (не сломан)

**And** warning выводится через `fatih/color` жёлтым

**And** тест в `cmd_test.go`: вызов `ralph bridge` → stdout содержит "устарел" и "ralph plan"

**Technical Notes:**
- Warning добавляется в `cmd/ralph/bridge.go` в `PersistentPreRun` или начало `RunE`
- Bridge функциональность не изменяется — только warning

**Prerequisites:** — (независимая story)

---

### Story 14.2: Bridge package deletion

Как разработчик, я хочу удалить весь код `ralph bridge`, чтобы убрать 2844 строк мёртвого кода и упростить кодовую базу.

**Acceptance Criteria:**

**Given** Epic 11 fully implemented, CI green
**When** выполняется deletion
**Then** удалены файлы:
- `bridge/bridge.go`
- `bridge/bridge_test.go`
- `bridge/prompt_test.go`
- `bridge/prompts/bridge.md`
- `bridge/testdata/` (вся директория)
- `cmd/ralph/bridge.go`

**And** регистрация bridge subcommand удалена из `cmd/ralph/root.go`

**And** `go build ./...` проходит без ошибок

**And** `go test ./...` проходит без ошибок

**And** `go vet ./...` без предупреждений

**And** нетто-эффект: ≥ -1000 строк кода (NFR19: 0 мёртвого кода)

**Technical Notes:**
- Выполнять в отдельном коммите после зелёного CI
- Проверить: `grep -r "bridge" --include="*.go" .` после deletion → только комментарии/история

**Prerequisites:** Story 14.1, Epic 11 CI green

---

## Epic 15: Growth — Acceptance Test & Research

**Scope:** Success Criteria из PRD (NFR1-NFR3, measurable outcomes)
**Stories:** 2
**Release milestone:** Growth gate
**PRD:** [docs/prd.md](prd.md)
**Architecture:** [docs/architecture.md](architecture.md)

**Context:**
Объективное подтверждение качества ralph plan на реальном проекте. Gate для входа в Growth-фичи: без подтверждения на learnPracticsCodePlatform и без метрик качества Growth-истории не начинаются.

**Зависит от:** Epic 14

**Dependency structure:**
```
Epic 14 complete ──→ 15.1 acceptance test (learnPracticsCodePlatform) ──→ 15.2 research metrics (gate decision)
```

**Existing scaffold:**
- learnPracticsCodePlatform — внешний репозиторий с реальными BMad-документами
- `docs/sprint-artifacts/` — место для `acceptance-test-results.md`
- Предыдущий bridge output (sprint-tasks.old.md, 295 строк) — baseline для сравнения

---

### Story 15.1: Acceptance test на learnPracticsCodePlatform

Как разработчик, я хочу запустить `ralph plan` на реальном BMad-проекте и сравнить output с предыдущим bridge, чтобы объективно подтвердить качество.

**Acceptance Criteria:**

**Given** репозиторий learnPracticsCodePlatform с готовыми BMad-документами
**When** `ralph plan docs/` выполнен
**Then** `sprint-tasks.md` сгенерирован без ошибок

**And** количество битых source-ссылок = 0 (было 100% с bridge, FR20)

**And** задач не меньше чем в `sprint-tasks.old.md` (предыдущий bridge output, 295 строк)

**And** результаты задокументированы в `docs/sprint-artifacts/acceptance-test-results.md`:
- Количество задач: было X, стало Y
- Битые ссылки: 0
- Стоимость запуска: $N
- Время выполнения: N сек

**Technical Notes:**
- Сравнение с `sprint-tasks.old.md` — ручное или через diff
- Метрика "битые ссылки": grep source-ref на несуществующие файлы

**Prerequisites:** Epic 14 complete

---

### Story 15.2: Research — метрики качества плана

Как разработчик, я хочу иметь объективные метрики качества `ralph plan`, чтобы принимать решение о запуске Growth фич на основе данных.

**Acceptance Criteria:**

**Given** результаты acceptance test (Story 15.1)
**When** исследование завершено
**Then** документ `docs/research/plan-quality-metrics.md` содержит:
- Определение "хорошего плана" (измеримые критерии)
- Как автоматически измерить качество
- Пороговые значения для входа в Growth (например: 0 битых ссылок на 3+ проектах)
- Рекомендация: продолжать ли Growth фичи

**And** решение о запуске Growth зафиксировано в `docs/sprint-artifacts/sprint-status.yaml`

**Technical Notes:**
- Research story: не требует кода, требует анализа и документирования
- Вопрос из PRD: "Как автоматически измерить качество плана?"

**Prerequisites:** Story 15.1

---

## Epic 16: Growth — Replan & Init

**Scope:** FR28, FR29
**Stories:** 2
**Release milestone:** Growth (после Epic 15 gate)
**PRD:** [docs/prd.md](prd.md)
**Architecture:** [docs/architecture.md](architecture.md)

**Context:**
Расширение ralph plan для новых сценариев: `ralph replan` — коррекция курса в середине спринта без перезапуска; `ralph init "описание"` — быстрый старт без BMad за 2-3 минуты. Обе story переиспользуют пакет `plan/` без рефакторинга.

**Зависит от:** Epic 15 (acceptance test + research подтвердили готовность)

**Dependency structure:**
```
Epic 15 gate ──→ 16.1 ralph replan (независимы)
Epic 15 gate ──→ 16.2 ralph init  (независимы)
```

**Existing scaffold:**
- `plan/plan.go` — Run(), PlanOpts (после Epic 11) — переиспользуется без изменений
- `cmd/ralph/root.go` — pattern регистрации subcommands
- `session/session.go` — Execute() — единственная Claude-сессия для `ralph init`

---

### Story 16.1: ralph replan

Как разработчик, я хочу запустить `ralph replan` в середине спринта, чтобы пересчитать незавершённые задачи без потери выполненных.

**Acceptance Criteria:**

**Given** `ralph replan` при существующем `sprint-tasks.md` с [x] и [ ] задачами
**When** выполнен
**Then** выполненные задачи [x] сохранены в начале файла

**And** незавершённые задачи [ ] заменены новыми на основе текущих входных документов

**And** реализуется через `plan.Run` с `PlanOpts.Merge = false` и передачей списка выполненных задач в промпт

**And** exit codes соответствуют `ralph plan` (0/1/2/3)

**Technical Notes:**
- `ralph replan` = новый cobra subcommand в `cmd/ralph/`
- Переиспользует `plan/` пакет без рефакторинга (как задумано в архитектуре)
- Промпт для replan: отдельный `plan/prompts/replan.md` или параметр в `plan.md`

**Prerequisites:** Epic 15

---

### Story 16.2: ralph init

Как разработчик без BMad, я хочу запустить `ralph init "описание"`, чтобы получить минимальный набор документов за 2-3 минуты и сразу перейти к `ralph plan`.

**Acceptance Criteria:**

**Given** `ralph init "API сервис для управления задачами"` в пустой директории
**When** выполнен
**Then** созданы файлы в `docs/`:
- `docs/prd.md` — минимальный PRD на основе описания
- `docs/architecture.md` — базовая архитектура

**And** время выполнения ≤ 3 минуты

**And** сгенерированные файлы достаточны для запуска `ralph plan docs/`

**And** показывается инструкция: `→ Документы готовы. Запустите: ralph plan docs/`

**Technical Notes:**
- `ralph init` = новый cobra subcommand
- Реализуется через одну Claude-сессию с промптом для генерации документов
- Промпт: `cmd/ralph/prompts/init.md` (отдельно от `plan/prompts/`)

**Prerequisites:** Epic 15

---

## FR Coverage Matrix

| FR | Epic | Story | Статус |
|----|------|-------|--------|
| FR1 | 11 | 11.7 | MVP |
| FR2 | 11 | 11.7, 11.8 | MVP |
| FR3 | 11 | 11.7 | MVP |
| FR4 | 13 | 13.2 | MVP |
| FR5 | 13 | 13.1 | MVP |
| FR6 | 12 | 12.4 | MVP |
| FR7 | 11 | 11.8 | MVP |
| FR8 | 12 | 12.4 | MVP |
| FR9 | 11 | 11.8 | MVP |
| FR10 | 11 | 11.6 | MVP |
| FR11 | 11 | 11.6 | MVP |
| FR12 | 11 | 11.6, 11.7 | MVP |
| FR13 | 11 | 11.7 | MVP |
| FR14 | 12 | 12.2 | MVP |
| FR15 | 12 | 12.1 | MVP |
| FR16 | 12 | 12.3 | MVP |
| FR17 | 12 | 12.3 | MVP |
| FR18 | 11 | 11.9 | MVP |
| FR19 | 11 | 11.9 | MVP |
| FR20 | 11 | 11.6 | MVP |
| FR21 | 11 | 11.3, 11.9 | MVP |
| FR22 | 11 | 11.9 | MVP |
| FR23 | 14 | 14.1 | MVP |
| FR24-27 | — | existing | Existing |
| FR28 | 16 | 16.1 | Growth |
| FR29 | 16 | 16.2 | Growth |

---

## Summary

**Эпиков:** 6 (4 MVP + 1 Growth gate + 1 Growth features)
**Stories:** 19 (15 MVP + 2 Growth gate + 2 Growth)
**FR покрытие:** 22/22 MVP требований, 2/2 Growth требований

**Порядок имплементации:**
```
Epic 11 (9 stories) → Epic 12 (4 stories) → Epic 14 (2 stories) → Epic 15 (2 stories) → Epic 16 (2 stories)
Epic 13 (2 stories) — параллельно с Epic 12
```

**Технический долг:** Stories 11.2 требует изменения в `session/` — проверить обратную совместимость с `runner/` и `bridge/` (до удаления bridge).

---

_Для имплементации: используй `bmad:bmm:workflows:dev-story` для выполнения каждой story._
