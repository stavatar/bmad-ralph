# Вариант D: Migration Path — от BMad-зависимого Ralph к самодостаточному

## Дата: 2026-03-07

---

## Что ломается при отказе от BMad stories

### 1. Пакет bridge — полный удаление или переписывание

**Текущее состояние:**
- `bridge/bridge.go` (141 строка) — ядро: читает story файлы, собирает промпт, вызывает Claude, парсит результат, пишет sprint-tasks.md
- `bridge/prompts/bridge.md` (244 строки) — промпт для конвертации stories → tasks
- Тесты: 27 Test-функций (14 unit + 5 integration + 5 prompt + 3 format), ~1870 строк тестов
- Fixtures: 28 файлов в `bridge/testdata/` (story_*.md, mock_*.md, existing_*.md, golden files)
- `cmd/ralph/bridge.go` — CLI-команда с `storyFileRe` regex, `splitBySize`, `runBridge`

**При переходе на Вариант D:**
- `bridge.Run()` становится **ненужным** — его заменяет новый `planner`
- `bridge/prompts/bridge.md` — **полностью удаляется** (специфичен для story → task конвертации)
- Все 28 test fixtures — **удаляются**
- `cmd/ralph/bridge.go` — остаётся как legacy (Phase 2) или удаляется (Phase 3)

**Итого:** ~2010 строк Go + 244 строки промпта + 28 fixtures → всё подлежит удалению/замене.

### 2. Runner scan.go — НЕ зависит от stories

**Критическое наблюдение:** `ScanTasks()` парсит **только sprint-tasks.md**, используя regex из `config`:
- `config.TaskOpenRegex` — матчит `- [ ]`
- `config.TaskDoneRegex` — матчит `- [x]`
- `config.GateTagRegex` — матчит `[GATE]`

Runner **не знает** откуда взялся sprint-tasks.md. Ему безразлично, создал ли его bridge из stories, или planner из plain text, или человек руками. Формат sprint-tasks.md — это контракт между bridge/planner и runner, НЕ между stories и runner.

**Вывод:** `runner/scan.go` (69 строк) остаётся **без изменений**. Все 20+ ссылок на sprint-tasks.md в тестах runner'а работают, потому что тесты создают sprint-tasks.md напрямую (через fixtures), а не через bridge.

### 3. Тесты — минимальное влияние на runner

Анализ тестовых зависимостей:
- **Runner тесты (98 ссылок на sprint-tasks/TaskOpen/ScanTasks):** все используют **inline fixtures** или `test_helpers_test.go`, ни один не вызывает `bridge.Run()`. Ноль тестов сломается.
- **Bridge тесты (27 функций):** ВСЕ специфичны для bridge. При удалении bridge — удаляются вместе с ним.
- **Config тесты:** `StoriesDir` config field тестируется, но это config parsing — останется для backward compatibility.

**Итого:** 0 тестов в runner/config сломается. Все 27 bridge тестов удаляются вместе с пакетом.

### 4. Промпты execute.md и review.md — НЕ упоминают stories

**execute.md:** Упоминает `sprint-tasks.md`, `source:` поле, задачи — но **ни слова о stories как BMad-концепции**. Единственная ссылка: "If the task has a `source:` field, open the referenced file" — это generic, работает с любым source (story, PRD, requirement doc).

**review.md:** Промпт review-агентов упоминает "story's acceptance criteria" (`implementation.md`, `test-coverage.md`), но это **семантическое**, а не структурное: review-агенты оценивают, выполнены ли требования. При переходе на плоский текст/PRD, "story" → "requirement specification" — замена 2 строк в 2 файлах промптов.

---

## Фазы миграции (timeline)

### Phase 0: Текущее состояние (статус-кво)

```
BMad AI → stories (*.md) → ralph bridge → sprint-tasks.md → ralph run → код
```

**Усилия:** 0. Всё работает.

### Phase 1: `ralph plan` — новая команда

```
Человек → plain text / PRD / requirements → ralph plan → sprint-tasks.md → ralph run → код
```

**Что создаётся:**
1. Новый пакет `planner/` (или расширение bridge с новым промптом)
2. Промпт `planner/prompts/plan.md` — конвертация произвольного текста → sprint-tasks.md
3. CLI-команда `cmd/ralph/plan.go`
4. Тесты

**bridge остаётся без изменений.** Оба пути работают параллельно.

**Усилия:** 2-3 stories (1 epic), ~1-2 дня при текущей скорости (3.3 findings/story).

### Phase 2: bridge deprecated

```
ralph plan = рекомендованный путь
ralph bridge = deprecated (warning при запуске)
```

**Что меняется:**
- `cmd/ralph/bridge.go` — добавляется deprecation warning
- Документация — обновляется
- `ralph plan` получает `--from-stories` флаг для обратной совместимости (читает story файлы как plain text input)

**Усилия:** 1 story, ~0.5 дня.

### Phase 3: bridge удалён

```
ralph plan = единственный путь
bridge/ пакет удалён
```

**Что удаляется:**
- `bridge/` — весь пакет (~2010 строк Go + 244 строки промпта)
- `bridge/testdata/` — 28 fixtures
- `cmd/ralph/bridge.go` — CLI-команда
- `config.StoriesDir` — можно удалить или оставить для legacy config файлов

**Усилия:** 1 story, ~0.5 дня. Чистая операция удаления.

### Timeline суммарно

| Фаза | Усилия | Срок | Backward compatible |
|------|--------|------|---------------------|
| Phase 1 | 2-3 stories | 1-2 дня | Да, bridge работает |
| Phase 2 | 1 story | 0.5 дня | Да, bridge deprecated |
| Phase 3 | 1 story | 0.5 дня | Нет, bridge удалён |
| **Итого** | **4-5 stories** | **2-3 дня** | — |

---

## Backward Compatibility

### Стратегия: `ralph plan` с универсальным входом

**Рекомендуемый подход:** `ralph plan` принимает ЛЮБОЙ текстовый input:
- Plain text requirements
- PRD (markdown)
- BMad story файлы (как plain text — bridge-промпт не нужен)
- GitHub Issues (через `gh issue view N --json body`)

**НЕ рекомендуется:** `--from-stories` как отдельный флаг. Причина: story файл — это просто markdown с AC. Plan-промпт должен уметь работать с любым markdown, включая stories. Отдельный флаг = лишняя сложность.

**Рекомендуется:** `ralph bridge` остаётся до Phase 3 как alias/wrapper:
```go
// cmd/ralph/bridge.go (Phase 2)
func runBridge(cmd *cobra.Command, args []string) error {
    color.Yellow("Warning: 'ralph bridge' is deprecated, use 'ralph plan' instead")
    return runPlan(cmd, args) // delegate to plan
}
```

### Пользовательский сценарий миграции

1. **Существующий пользователь BMad:** `ralph bridge story1.md story2.md` → продолжает работать (Phase 1-2). В Phase 3: `ralph plan story1.md story2.md` — тот же результат, plan-промпт умеет читать stories.

2. **Новый пользователь без BMad:** `ralph plan requirements.txt` или `ralph plan prd.md` — работает сразу (Phase 1+).

3. **Пользователь с GitHub Issues:** `gh issue view 42 --json body -q .body | ralph plan --stdin` — новый use case (Phase 1+).

---

## Phase 1: ralph plan (scope)

### Архитектурное решение: новый пакет `planner/`

**Почему НЕ расширять bridge/:**
- bridge/ семантически привязан к "story → task" конвертации
- Промпт bridge.md (244 строки) содержит BMad-специфичную логику (AC Classification, story numbering, epic detection)
- Чистый separation of concerns: bridge = legacy, planner = future

**Почему НЕ добавлять в runner/:**
- runner/ уже 29K строк — нарушит SRP
- Dependency direction: `cmd/ralph → planner → session, config` (параллельно bridge)

### Новые файлы

```
planner/
├── planner.go              # Plan() — аналог bridge.Run()
├── planner_test.go          # Unit-тесты
├── planner_integration_test.go  # Integration с mock Claude
└── prompts/
    └── plan.md              # Промпт для декомпозиции

cmd/ralph/
├── plan.go                  # CLI-команда `ralph plan`
```

### Промпт plan.md — ключевые отличия от bridge.md

| Аспект | bridge.md (244 строки) | plan.md (оценка ~150 строк) |
|--------|----------------------|---------------------------|
| Input | Story с AC, Dev Notes, structured sections | Произвольный текст/markdown |
| AC Classification | 4 типа (Implementation/Behavioral/Verification/Manual) | Не нужно — нет AC как формальной концепции |
| Story numbering | Парсинг "Story 1.1" для gate detection | Не нужно — gates по семантике (deploy, security) |
| Source traceability | `source: filename.md#AC-N` | `source: filename.md#REQ-N` или `source: inline#1` |
| Format contract | Тот же `sprint-tasks-format.md` | Тот же — runner не меняется |
| Merge mode | Полностью | Полностью — переиспользуем |
| Task granularity | Те же правила | Те же — переиспользуем |
| Batching | splitBySize | Не нужно — один input file, обычно <50KB |

### CLI: `cmd/ralph/plan.go`

```go
// ralph plan [files...] [--stdin]
// Примеры:
//   ralph plan requirements.md
//   ralph plan story1.md story2.md
//   echo "Add auth" | ralph plan --stdin
//   gh issue view 42 --json body -q .body | ralph plan --stdin
```

Флаги:
- `--stdin` — читать input из stdin
- Позиционные args — файлы с требованиями
- Без аргументов и без --stdin — ошибка (в отличие от bridge, который ищет в StoriesDir)

### Тесты: оценка объёма

| Категория | Количество | Строки (оценка) |
|-----------|-----------|----------------|
| Unit тесты planner.go | 8-10 функций | ~400 |
| Prompt тесты | 4-5 функций | ~200 |
| Integration тесты | 3-4 функции | ~300 |
| Fixtures (testdata/) | 5-8 файлов | ~100 |
| Golden files | 3-5 файлов | ~150 |
| **Итого** | **~20 тестов** | **~1150 строк** |

Для сравнения: bridge имеет 27 тестов / ~1870 строк. planner проще (нет batching, нет storyFileRe, нет AC Classification), поэтому ~60% объёма bridge.

### Acceptance Criteria для stories Phase 1

**Story P1.1: planner.Plan() core**
- AC1: `planner.Plan(ctx, cfg, inputFiles)` читает файлы, собирает промпт, вызывает Claude, пишет sprint-tasks.md
- AC2: Промпт plan.md включает format contract и merge mode
- AC3: Merge mode: при существующем sprint-tasks.md создаёт backup и inject existing content
- AC4: Возвращает (taskCount, error)

**Story P1.2: CLI `ralph plan`**
- AC1: `ralph plan file.md` вызывает `planner.Plan()`
- AC2: `ralph plan --stdin` читает из stdin
- AC3: Без аргументов — ошибка с usage message
- AC4: Вывод: "Generated N tasks in sprint-tasks.md"

**Story P1.3: Промпт plan.md + тесты**
- AC1: plan.md конвертирует произвольный текст в sprint-tasks.md format
- AC2: plan.md поддерживает merge mode (HasExistingTasks conditional)
- AC3: Source traceability: для файлового input — `source: filename#REQ-N`, для stdin — `source: inline#N`
- AC4: Golden file тесты на prompt assembly

---

## Оценка объёма работы

### Новый код

| Компонент | Строки Go | Строки промпт | Строки тестов |
|-----------|----------|--------------|--------------|
| `planner/planner.go` | ~80 | — | ~400 |
| `planner/prompts/plan.md` | — | ~150 | ~200 |
| `cmd/ralph/plan.go` | ~40 | — | ~100 |
| Integration тесты | — | — | ~300 |
| Fixtures + golden | — | — | ~250 |
| **Итого Phase 1** | **~120** | **~150** | **~1250** |

### Удаляемый код (Phase 3)

| Компонент | Строки |
|-----------|--------|
| `bridge/bridge.go` | 141 |
| `bridge/prompts/bridge.md` | 244 |
| `bridge/*_test.go` | 1870 |
| `bridge/testdata/*` | ~500 |
| `cmd/ralph/bridge.go` | 89 |
| **Итого удаление** | **~2844** |

### Нетто-эффект (Phase 3 done)

- Новый код: ~120 Go + ~150 промпт + ~1250 тестов = ~1520 строк
- Удалённый код: ~2844 строк
- **Итого: -1324 строки** (упрощение)

---

## Риски и митигации

### Р1: Регрессия качества task decomposition

**Риск:** Plan-промпт без BMad-специфичных правил (AC Classification, story numbering) может генерировать менее структурированные задачи.

**Митигация:**
- Task granularity rules (50+ строк в bridge.md) переиспользуются в plan.md без изменений
- Format contract (sprint-tasks-format.md) остаётся тем же
- Качество зависит от format contract + granularity rules, НЕ от AC Classification
- AC Classification полезна только для BMad stories — для plain text она не применима

**Вероятность:** Низкая. Основные правила качества (granularity, format, gates) не зависят от stories.

### Р2: Потеря BMad-специфичных преимуществ

**Риск:** Validated stories с формализованными AC дают более предсказуемый результат, чем plain text.

**Митигация:**
- `ralph plan` может принимать stories как input — ничего не теряется для BMad-пользователей
- Plan-промпт будет инструктировать Claude извлекать structured requirements из любого формата
- ADaPT research показывает: as-needed decomposition лучше upfront (bridge = upfront)

**Вероятность:** Средняя. Для хорошо структурированного plain text — нет проблемы. Для плохо написанных требований — возможна деградация. Митигация: `ralph plan --review` для человеческой проверки перед `ralph run`.

### Р3: Увеличение maintenance burden (два пути)

**Риск:** В Phase 1-2 поддерживаются оба пути (bridge + plan), удваивая maintenance.

**Митигация:**
- Phase 2 минимальна (deprecation warning + delegate to plan)
- Phase 1→Phase 3 можно пройти за 2-3 дня — окно с двумя путями короткое
- planner.Plan() и bridge.Run() разделяют 0 кода (кроме config и session) — нет risk conflicting changes
- Решение: быстрый переход Phase 1→3 (без затяжного Phase 2)

**Вероятность:** Низкая при быстром переходе.

### Р4: Потеря batching для больших проектов

**Риск:** bridge поддерживает batching 34+ stories. plan работает с одним input.

**Митигация:**
- Batching bridge — сам по себе антипаттерн (каждый batch не видит контекст других)
- Типичный PRD/requirements doc: 10-50KB — помещается в один контекст
- При необходимости: `ralph plan file1.md file2.md` обрабатывает файлы последовательно (как bridge batches, но проще)

**Вероятность:** Низкая. В bridge-concept-analysis.md batching признан проблемой, не преимуществом.

### Р5: stdin input может быть пустым или слишком большим

**Риск:** `ralph plan --stdin` без валидации может получить 0 байт или 1MB+.

**Митигация:**
- Валидация: минимум 10 символов, максимум 500KB
- Ошибка с человекочитаемым сообщением

**Вероятность:** Низкая. Стандартная input validation.

---

## Рекомендация

### Оптимальный путь: Phase 1 → Phase 3 (быстрый переход, без затяжного Phase 2)

1. **Реализовать `ralph plan`** (Phase 1) — 2-3 stories, 1-2 дня
2. **Сразу deprecate bridge** (Phase 2) — 1 story, 0.5 дня
3. **Удалить bridge** (Phase 3) — 1 story, 0.5 дня
4. **Общий срок: 2-3 дня, 4-5 stories, 1 мини-эпик**

### Почему быстрый переход, а не постепенный

- Bridge concept analysis (существующий research) уже показал: bridge — уникальный антипаттерн среди всех рассмотренных инструментов
- Runner **не зависит** от bridge — scan.go парсит sprint-tasks.md, а не stories
- 0 тестов runner'а сломается
- Нетто-эффект: **-1324 строки кода** (упрощение)
- Двойной maintenance (bridge + plan) — бессмысленная трата при коротком окне перехода

### Принципиальное решение: промпт vs программный парсинг

Bridge concept analysis рекомендует вариант E2 (программный парсинг). Однако для **Варианта D** (ralph сам декомпозирует) LLM **необходим** — программный парсинг произвольного текста невозможен. Разница с bridge:

| Аспект | bridge (текущий) | plan (Вариант D) |
|--------|-----------------|-----------------|
| Input | Формализованные stories с AC | Произвольный текст |
| LLM необходимость | Нет — 80% программный парсинг | Да — нет формальной структуры |
| Промпт сложность | 244 строки (AC Classification, story numbering) | ~150 строк (только format + granularity) |
| Batching | 6 batch'ей | 1 вызов (input обычно <50KB) |
| "Испорченный телефон" | 3 AI слоя (BMad → bridge → runner) | 2 AI слоя (plan → runner) |

**Вариант D убирает один AI-слой** (BMad stories) и **упрощает промпт** (нет AC Classification). Это чистое улучшение.

---

## Источники

### Код проекта (проанализировано)
- `bridge/bridge.go` — ядро bridge, функция `Run()` (141 строка)
- `bridge/prompts/bridge.md` — промпт bridge (244 строки)
- `cmd/ralph/bridge.go` — CLI bridge с `storyFileRe`, `splitBySize` (89 строк)
- `runner/scan.go` — `ScanTasks()` парсит sprint-tasks.md (69 строк)
- `runner/runner.go` — `RunOnce()`, `Execute()` (29K строк пакет)
- `runner/prompts/execute.md` — промпт выполнения (131 строка)
- `runner/prompts/review.md` — промпт review (177 строк)
- `runner/prompts/agents/implementation.md` — review agent (упоминает "story's acceptance criteria")
- `runner/prompts/agents/test-coverage.md` — review agent (упоминает "story's acceptance criteria")
- `config/format.go` — embedded sprint-tasks-format.md
- `config/shared/sprint-tasks-format.md` — format contract (134 строки)
- `bridge/testdata/` — 28 test fixtures

### Существующие исследования проекта
- `docs/research/bridge-concept-analysis.md` — критический анализ bridge (355 строк)
- `docs/research/bridge-performance-analysis.md` — оптимизация скорости bridge (69 строк)

### Внешние исследования (из bridge-concept-analysis.md)
- ADaPT: As-Needed Decomposition and Planning with Language Models (arxiv.org/abs/2311.05772)
- The Agentic Telephone Game: Cautionary Tale (christopheryee.org)
- Requirements are All You Need (arxiv.org/pdf/2406.10101)
