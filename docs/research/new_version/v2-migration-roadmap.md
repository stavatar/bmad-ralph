# Ralph v2: Roadmap миграции к самодостаточности

**Дата:** 2026-03-07
**Автор:** Project Manager
**Статус:** Драфт
**Основа:** variant-d-migration-path.md, variant-d-workflow-gaps.md, variant-d-task-format.md

---

## Обзор

Ralph v2 устраняет зависимость от BMad workflow. Цель: пользователь создаёт требования в любом формате, ralph сам декомпозирует их в задачи и выполняет.

**Текущий pipeline:**
```
BMad AI → stories (*.md) → ralph bridge → sprint-tasks.md → ralph run → код
```

**Целевой pipeline:**
```
Требования (любой формат) → ralph plan → sprint-tasks.md → ralph run → код
```

---

## Порядок реализации и зависимости

```
Epic 11 (ralph plan)  ←── КРИТИЧЕСКИЙ ПУТЬ, первый
     │
     ├── Epic 12 (ralph replan) ←── зависит от 11, можно параллелить с 13
     │
     ├── Epic 13 (YAML формат) ←── ОПЦИОНАЛЬНЫЙ, независим от 12
     │
     └── Epic 15 (Bridge Deprecation) ←── зависит от 11 (plan должен работать)

Epic 14 (ralph init)  ←── ПОЛНОСТЬЮ НЕЗАВИСИМ, можно делать параллельно с 11
```

### Минимальный viable path (MVP)

**Фаза 1 (обязательная):** Epic 11 → Epic 15
- Результат: `ralph plan` заменяет `ralph bridge`, bridge удалён
- Объём: ~11 stories, ~2-3 дня при текущей скорости

**Фаза 2 (рекомендуемая):** Epic 12
- Результат: mid-sprint коррекция без ручной правки sprint-tasks.md
- Объём: ~4 stories, ~1 день

**Фаза 3 (отложить):** Epic 13, Epic 14
- YAML формат — улучшение, не блокер
- `ralph init` — удобство, не необходимость

### Параллелизация

| Можно параллелить | Нельзя |
|-------------------|--------|
| Epic 14 + Epic 11 | Epic 12 до завершения 11.3 (нужен planner/) |
| Epic 12 + Epic 13 | Epic 15 до завершения 11.5 (plan должен работать) |
| Stories 11.1-11.3 (scaffold + prompts) — последовательно | 11.4-11.5 зависят от 11.3 |

---

## Epic 11: ralph plan (Core Planning)

**Цель:** Новая команда `ralph plan`, генерирующая sprint-tasks.md из произвольных текстовых требований через Claude.

**Пакет:** `planner/` (параллельно `bridge/`, та же позиция в dependency tree: `cmd/ralph → planner → session, config`).

**Оценка:** 6 stories, ~2 дня.

---

### Story 11.1: Пакет planner/ — scaffold

**Размер:** S (scaffold, без логики)

**Зависимости:** нет

**AC:**
1. Создан пакет `planner/` с файлом `planner.go`, содержащим заглушку `Plan(ctx, cfg, inputs) (int, error)` которая возвращает `ErrNotImplemented`
2. Создан `planner/prompts/` каталог с пустым `plan.md` (embedded через `//go:embed`)
3. Функция `PlanPrompt() string` экспортирует embedded промпт (аналог `bridge.BridgePrompt()`)
4. `go build ./...` проходит без ошибок, `go vet ./planner/...` чистый
5. Unit-тесты: `TestPlan_NotImplemented` проверяет заглушку, `TestPlanPrompt_NotEmpty` проверяет embed

---

### Story 11.2: Document autodiscovery

**Размер:** M (логика обнаружения + тесты)

**Зависимости:** 11.1

**AC:**
1. Функция `DiscoverInputs(cfg, explicitFiles) ([]InputDoc, error)` находит документы для планирования
2. `InputDoc` содержит `Path string`, `Content string`, `Source string` (имя файла для traceability)
3. При явных файлах (args) — читает их; при `--stdin` — читает stdin; без аргументов — ищет в `cfg.ProjectRoot` файлы `requirements.md`, `prd.md`, `*.requirements.md`
4. Валидация: пустой input (< 10 символов) — ошибка; слишком большой (> 500KB суммарно) — ошибка
5. Таблично-управляемые тесты покрывают: явные файлы, stdin, autodiscovery, ошибки валидации, несуществующий файл

---

### Story 11.3: Промпт plan.md

**Размер:** M (промпт + prompt тесты)

**Зависимости:** 11.1

**AC:**
1. `planner/prompts/plan.md` — Go template, принимающий `TemplateData{HasExistingTasks bool}` + placeholders `__INPUT_CONTENT__`, `__FORMAT_CONTRACT__`, `__EXISTING_TASKS__`
2. Промпт инструктирует Claude: декомпозировать требования в sprint-tasks.md формат (`- [ ]` чеклист с `source:` полем)
3. Промпт переиспользует `config.SprintTasksFormat()` (format contract) без дублирования
4. Промпт содержит правила granularity (1 задача = 1 коммит, атомарные изменения) и gate marking (`[GATE]` для deploy/security задач)
5. Prompt-тесты (golden files): assembly без existing tasks, assembly с existing tasks, наличие format contract, наличие granularity rules

---

### Story 11.4: Core planner.Plan() логика

**Размер:** M (основная логика + unit тесты)

**Зависимости:** 11.2, 11.3

**AC:**
1. `Plan(ctx, cfg, inputs []InputDoc) (int, error)` собирает промпт через `config.AssemblePrompt`, вызывает `session.Execute`, парсит результат, пишет sprint-tasks.md
2. Merge mode: при существующем sprint-tasks.md создаёт `.bak` backup, инжектирует existing content в промпт (аналогично bridge)
3. Подсчёт задач: возвращает количество open tasks (`config.TaskOpenRegex`) в сгенерированном output
4. Все ошибки обёрнуты с префиксом `"planner:"` (единообразно с bridge/runner)
5. Unit-тесты с mock Claude: happy path, merge mode, session error, parse error, write error

---

### Story 11.5: CLI cmd/ralph/plan.go

**Размер:** S (CLI обвязка)

**Зависимости:** 11.4

**AC:**
1. Команда `ralph plan [files...] [--stdin]` зарегистрирована в cobra (аналогично bridge/run/distill)
2. С файлами: передаёт args как explicit files; с `--stdin`: читает stdin; без аргументов и без --stdin: вызывает `DiscoverInputs` для autodiscovery
3. Вывод: `"Generated N tasks in sprint-tasks.md"` (формат идентичен bridge output)
4. Exit codes: 0 = успех, 1 = ошибка (через стандартный `mapError` в exit.go)
5. Тесты: flag parsing, --stdin flag presence, help text содержит examples

---

### Story 11.6: Integration тесты planner

**Размер:** M (integration с mock Claude)

**Зависимости:** 11.4, 11.5

**AC:**
1. Integration тест: `ralph plan requirements.md` с mock Claude → проверка sprint-tasks.md создан, содержит задачи, формат корректен
2. Integration тест: merge mode — existing sprint-tasks.md + plan → backup создан, merged output корректен
3. Integration тест: stdin mode — pipe content в `ralph plan --stdin` → sprint-tasks.md создан
4. Mock Claude возвращает предопределённый sprint-tasks.md content (scenario-based JSON через `config.ClaudeCommand`)
5. Все тесты используют `t.TempDir()` для изоляции, проверяют содержимое файлов (не только counts)

---

## Epic 12: ralph replan (Course Correction)

**Цель:** Механизм обновления sprint-tasks.md mid-sprint с сохранением прогресса. Упрощённая альтернатива BMad correct-course.

**Оценка:** 4 stories, ~1 день.

---

### Story 12.1: Replan core логика

**Размер:** M

**Зависимости:** Epic 11 (planner/ пакет)

**AC:**
1. Функция `Replan(ctx, cfg, inputs []InputDoc) (ReplanResult, error)` вызывает `Plan()` с сохранением прогресса done-задач
2. `ReplanResult` содержит: `Added int`, `Removed int`, `Kept int`, `DonePreserved int`
3. Перед вызовом Claude: сканирует текущий sprint-tasks.md, извлекает `- [x]` задачи
4. После генерации нового sprint-tasks.md: программно восстанавливает done-задачи (match по тексту задачи)
5. Создаёт backup текущего sprint-tasks.md как `.replan.bak` (отличается от merge `.bak`)

---

### Story 12.2: Replan промпт + контекст

**Размер:** S

**Зависимости:** 12.1

**AC:**
1. Replan использует тот же `plan.md` промпт с дополнительным контекстом: список уже выполненных задач (done tasks) и инструкция не дублировать их
2. Done tasks инжектируются как `__DONE_TASKS__` placeholder в промпт
3. Промпт содержит инструкцию: "Следующие задачи уже выполнены. НЕ генерировать их повторно, НЕ отменять их"
4. Prompt-тест: assembly с done tasks содержит done task list
5. Prompt-тест: assembly без done tasks НЕ содержит done task секцию

---

### Story 12.3: Diff display

**Размер:** S

**Зависимости:** 12.1

**AC:**
1. `FormatDiff(old, new ScanResult) string` генерирует человекочитаемый diff между старым и новым sprint-tasks.md
2. Формат: `+ Новая задача`, `- Удалённая задача`, `= Сохранённая задача`, `[x] Done (preserved)`
3. Цветной вывод через `fatih/color`: зелёный для `+`, красный для `-`, жёлтый для `=`
4. Unit-тесты: пустой diff (ничего не изменилось), только добавления, только удаления, смешанный
5. Diff выводится в stdout после replan, перед записью файла

---

### Story 12.4: CLI cmd/ralph/replan.go

**Размер:** S

**Зависимости:** 12.1, 12.3

**AC:**
1. Команда `ralph replan [files...] [--stdin] [--yes]` зарегистрирована в cobra
2. Без `--yes`: показывает diff, спрашивает подтверждение (y/n); с `--yes`: применяет без подтверждения
3. Вывод: diff + `"Replan complete: +N added, -N removed, =N kept, [x]N preserved"`
4. Если sprint-tasks.md не существует — ошибка `"no existing sprint-tasks.md to replan; use 'ralph plan' first"`
5. Тесты: flag parsing, --yes flag, help text, ошибка при отсутствии sprint-tasks.md

---

## Epic 13: YAML Task Format (Опциональный)

**Цель:** Машиночитаемый YAML формат для задач, устраняющий regex-хрупкость markdown чеклиста. Обратно совместим через dual format support.

**Оценка:** 4 stories, ~1.5 дня.

**Решение GO/NO-GO:** после Epic 11. Если markdown формат работает стабильно — отложить на неопределённый срок.

---

### Story 13.1: YAML schema + Go structs

**Размер:** S

**Зависимости:** нет (может начаться параллельно с Epic 11)

**AC:**
1. `config/taskformat/` (или `config/`) содержит Go structs: `TaskFile{Tasks []Task}`, `Task{ID, Description, Status, Source, Gate, DependsOn, Type, Size}`
2. `Status` = enum: `open`, `done`, `skipped`, `error`
3. `Type` = enum: `feature`, `refactor`, `bugfix`, `test`, `docs`
4. `Size` = enum: `S`, `M`, `L`, `XL`
5. Пример YAML файл в `config/shared/sprint-tasks-format.yaml` + unit тесты marshal/unmarshal round-trip

---

### Story 13.2: YAML scanner (дополнение scan.go)

**Размер:** M

**Зависимости:** 13.1

**AC:**
1. `ScanTasksYAML(content string) (ScanResult, error)` парсит YAML task файл и возвращает `ScanResult` (тот же тип, что и markdown scanner)
2. `ScanResult.OpenTasks` и `DoneTasks` заполняются из YAML Status field
3. `HasGate` заполняется из `Task.Gate bool` field (не regex, а структурное поле)
4. `ScanTasks` автоматически определяет формат (YAML vs markdown) по первой строке: `tasks:` → YAML, иначе markdown
5. Таблично-управляемые тесты: YAML happy path, YAML пустой, YAML с ошибками парсинга, автоопределение формата

---

### Story 13.3: YAML генерация в planner

**Размер:** M

**Зависимости:** 13.1, Epic 11

**AC:**
1. `ralph plan --format yaml` генерирует sprint-tasks.yaml вместо sprint-tasks.md
2. Промпт plan.md получает `{{if .YAMLFormat}}` секцию с YAML format contract
3. Программный post-processing: если Claude output не является valid YAML — fallback на markdown
4. YAML output проходит `yaml.Unmarshal` + schema validation перед записью
5. Unit-тесты: YAML generation, fallback на markdown, schema validation error

---

### Story 13.4: Dual format + миграция

**Размер:** S

**Зависимости:** 13.2, 13.3

**AC:**
1. `ralph run` автоматически определяет формат: проверяет `sprint-tasks.yaml` (приоритет), затем `sprint-tasks.md`
2. `ralph migrate-tasks` конвертирует существующий sprint-tasks.md → sprint-tasks.yaml программно (без LLM)
3. Конвертация сохраняет все метаданные: status, source, gate tag, description
4. При наличии обоих файлов — предупреждение, использует YAML
5. Integration тест: migrate → run → задачи корректно распознаны

---

## Epic 14: ralph init (Quick Start)

**Цель:** Быстрый старт для новых пользователей. Генерация минимальной конфигурации из описания проекта.

**Оценка:** 3 stories, ~1 день.

**Может выполняться ПАРАЛЛЕЛЬНО с Epic 11.**

---

### Story 14.1: Базовый init

**Размер:** S

**Зависимости:** нет

**AC:**
1. `ralph init` создаёт `.ralph.yaml` с дефолтными значениями в текущей директории
2. Если `.ralph.yaml` уже существует — ошибка `"config already exists; use --force to overwrite"`
3. `--force` перезаписывает существующий конфиг
4. Созданный конфиг содержит закомментированные примеры всех доступных полей
5. Unit-тесты: создание, уже существует (без --force), --force перезапись

---

### Story 14.2: Interactive init

**Размер:** M

**Зависимости:** 14.1

**AC:**
1. `ralph init --interactive` запрашивает: project description (free text), language/framework, test command, max iterations
2. Ответы записываются в `.ralph.yaml` (не defaults, а введённые значения)
3. Project description сохраняется как `requirements.md` в project root (input для `ralph plan`)
4. При `--interactive` + pipe (не tty) — ошибка `"interactive mode requires a terminal"`
5. Тесты: mock stdin с ответами, non-tty detection

---

### Story 14.3: Brownfield scan

**Размер:** M

**Зависимости:** 14.1

**AC:**
1. `ralph init --scan` анализирует существующий проект: определяет язык (go.mod/package.json/Cargo.toml), test framework, build system
2. Результаты scan записываются как комментарии/defaults в `.ralph.yaml`
3. Scan определяет: language, test command suggestion, project root, наличие CI конфига
4. При невозможности определить язык — предупреждение, defaults без language-specific настроек
5. Таблично-управляемые тесты: Go проект (go.mod), Node проект (package.json), Python (pyproject.toml), неизвестный проект

---

## Epic 15: Bridge Deprecation

**Цель:** Удаление bridge/ пакета после стабилизации `ralph plan`.

**Оценка:** 2 stories, ~0.5 дня.

**Зависимости:** Epic 11 полностью завершён и стабилен.

---

### Story 15.1: Deprecation warning + delegation

**Размер:** S

**Зависимости:** Epic 11 (все stories)

**AC:**
1. `ralph bridge` выводит жёлтое предупреждение: `"Warning: 'ralph bridge' is deprecated, use 'ralph plan' instead"`
2. После предупреждения `ralph bridge` делегирует к `runPlan()` — та же логика, что `ralph plan`
3. Story файлы (аргументы bridge) передаются как input files в planner
4. `--help` для bridge содержит deprecation notice
5. Unit-тест: вызов bridge → предупреждение в output + delegation к plan

---

### Story 15.2: Bridge removal

**Размер:** S

**Зависимости:** 15.1

**AC:**
1. Удалён пакет `bridge/` целиком: `bridge.go`, `prompts/bridge.md`, все `*_test.go`, весь `testdata/`
2. Удалён `cmd/ralph/bridge.go`
3. `config.StoriesDir` помечен как deprecated (оставлен для backward compat конфиг-файлов, не используется в коде)
4. `go build ./...` проходит, `go vet ./...` чистый, нет import references на `bridge`
5. Нетто-эффект: удалено ~2800 строк кода (bridge Go + промпт + тесты + fixtures)

---

## Сводная таблица

| Epic | Stories | Размер | Зависимости | Приоритет | Оценка (дни) |
|------|---------|--------|-------------|-----------|-------------|
| **11: ralph plan** | 6 | L | нет | КРИТИЧЕСКИЙ | 2 |
| **12: ralph replan** | 4 | M | Epic 11 | ВЫСОКИЙ | 1 |
| **13: YAML format** | 4 | M | нет (13.3 → Epic 11) | НИЗКИЙ | 1.5 |
| **14: ralph init** | 3 | M | нет | СРЕДНИЙ | 1 |
| **15: Bridge removal** | 2 | S | Epic 11 | ВЫСОКИЙ | 0.5 |
| **Итого** | **19** | — | — | — | **6 дней** |

### MVP path (минимум для самодостаточности)

```
Epic 11 (2 дня) → Epic 15 (0.5 дня) = 2.5 дня, 8 stories
```

### Рекомендуемый path

```
Epic 11 (2 дня) → Epic 15 (0.5 дня) → Epic 12 (1 день) = 3.5 дня, 12 stories
        ↕ параллельно
    Epic 14 (1 день)
```

### Что отложить

- **Epic 13 (YAML format):** Отложить до момента, когда regex-хрупкость markdown станет реальной проблемой в продакшене. Текущий формат работает стабильно 10 эпиков. GO/NO-GO решение после 1-2 месяцев использования `ralph plan`.
- **Epic 14 (ralph init):** Отложить, если целевая аудитория — опытные разработчики, знакомые с ручной настройкой. Приоритет повышается при публичном релизе.

---

## Архитектурные решения

### Почему новый пакет planner/, а не расширение bridge/

1. **Семантика:** bridge = "story → task" (BMad-специфичный), planner = "requirements → task" (универсальный)
2. **Чистое удаление:** bridge удаляется целиком в Epic 15, без выделения "общего" кода
3. **SRP:** runner/ уже ~1200 LOC, добавлять planning в runner нарушает single responsibility
4. **Dependency tree:** `cmd/ralph → planner → session, config` — параллельно bridge, без новых зависимостей

### Почему LLM для plan, а не программный парсинг

1. **Input непредсказуем:** plain text, PRD, markdown, GitHub Issues — формализованного формата нет
2. **Декомпозиция = семантическая задача:** разбить "добавить авторизацию" на атомарные задачи невозможно regex-парсером
3. **ADaPT research подтверждает:** as-needed decomposition через LLM эффективнее upfront программного разбора
4. **Bridge concept analysis:** bridge = антипаттерн не из-за LLM, а из-за лишнего AI-слоя. Plan убирает слой (BMad stories), не добавляет

### Формат sprint-tasks.md сохраняется

1. **Runner не меняется:** scan.go парсит sprint-tasks.md — 0 изменений в runner
2. **Человекочитаемость:** markdown чеклист легко читать и редактировать вручную
3. **Проверено:** 10 эпиков, 80+ stories, ~500 задач — формат стабилен
4. **YAML (Epic 13):** опция на будущее, не блокер

---

## Риски

| Риск | Вероятность | Влияние | Митигация |
|------|------------|---------|-----------|
| Plan-промпт генерирует менее качественные задачи, чем bridge | Низкая | Среднее | Переиспользуем granularity rules + format contract из bridge |
| Stdin input слишком большой / пустой | Низкая | Низкое | Валидация: min 10 символов, max 500KB |
| Двойной maintenance в период bridge+plan | Средняя | Низкое | Быстрый переход Phase 1→3 (2.5 дня) |
| YAML формат создаёт migration burden | Низкая | Среднее | Отложен. GO/NO-GO через 1-2 месяца |
| Replan теряет done tasks при fuzzy matching | Средняя | Высокое | Exact text matching + backup .replan.bak |

---

## Метрики успеха

1. **ralph plan генерирует ≥ 90% задач, которые бы создал bridge** (ручная проверка на 3 существующих stories)
2. **0 тестов runner/ сломано** после удаления bridge
3. **Нетто-эффект: ≥ -1000 строк кода** (упрощение)
4. **Среднее quality findings ≤ 4.0/story** (ниже текущего среднего 3.3)
5. **ralph plan работает с stdin** (GitHub Issues, pipe content)
