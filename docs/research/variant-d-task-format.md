# Вариант D: Task Format Analysis

Дата: 2026-03-07
Контекст: ralph использует `sprint-tasks.md` — markdown-чеклист с regex-парсингом.
При переходе к варианту D (самодостаточность) формат может измениться.

## Проблемы текущего sprint-tasks.md

### 1. Отсутствие зависимостей между задачами

Текущий формат — линейный список. Порядок задач неявно задаёт последовательность, но нет явной модели зависимостей:
- Задача B зависит от задачи A (B использует модуль, созданный в A) — это видно только из описания
- При параллельном выполнении (несколько агентов) невозможно определить, какие задачи независимы
- Перестановка задач требует понимания контекста, а не просто чтения метаданных

### 2. Отсутствие приоритетов и оценок сложности

- Нет way определить, какая задача критичнее
- Нет оценки effort/size (S/M/L/XL)
- Планирование спринта невозможно без внешних инструментов

### 3. Бедные метаданные

- Нет информации о затрагиваемых файлах (`files_hint`)
- Нет типа изменения (feature, refactor, bugfix, test)
- Нет ID задачи — ссылаться можно только по номеру строки (хрупко)
- `source:` поле привязано к AC, но не содержит семантической информации

### 4. Ненадёжность LLM-генерации markdown

Наблюдаемые проблемы:
- `- []` вместо `- [ ]` (отсутствие пробела) — regex `^\s*- \[ \]` не матчит
- `* [ ]` вместо `- [ ]` (звёздочка) — regex не матчит
- Лишние пробелы в `source:` — regex `^\s+source:\s+\S+#\S+` может не сработать
- `[x]` с заглавной `[X]` — regex `- \[x\]` не матчит (case-sensitive)
- Нестабильный отступ `source:` — табы vs пробелы
- Перенос длинных задач на несколько строк — парсер видит только одну строку

### 5. Merge Mode — самая сложная часть

Bridge prompt содержит 12 правил для Merge Mode (сохранить `[x]`, порядок, не дублировать).
Это сложная семантика для LLM — ошибки merge неизбежны при большом количестве задач.

### 6. Отсутствие идемпотентности

- Повторный запуск bridge с тем же story может породить дубликаты
- Определение "задача уже существует" через текстовое сравнение описания — хрупко

## Альтернативные форматы (таблица сравнения)

| Критерий | Markdown (текущий) | YAML | JSON | TOML | GitHub Issues |
|----------|-------------------|------|------|------|---------------|
| **Человекочитаемость** | Отлично | Хорошо | Средне | Хорошо | Отлично (Web UI) |
| **Машиночитаемость** | Плохо (regex) | Отлично (yaml.v3) | Отлично (encoding/json) | Хорошо (BurntSushi/toml) | Средне (API) |
| **LLM-генерация** | Средне (форматные ошибки) | Хорошо (17.7% лучше JSON по accuracy) | Хорошо (constrained decoding) | Средне | Н/Д (API вызовы) |
| **Токен-эффективность** | Лучше всех (~15% vs JSON) | Хорошо (~10% дороже markdown) | Дороже всех (скобки, кавычки) | Хорошо | Н/Д |
| **Зависимости** | Нет | Да (нативно) | Да (нативно) | Да | Да (linked issues) |
| **Merge/diff** | Хорошо (построчно) | Хорошо (построчно) | Плохо (скобки ломают diff) | Хорошо | Нет (внешний сервис) |
| **Schema validation** | Нет | Да (через Go struct) | Да (JSON Schema + constrained decoding) | Да (через Go struct) | Да (API schema) |
| **Новые зависимости** | Нет | Нет (yaml.v3 уже есть) | Нет (stdlib) | Да (новый пакет) | Да (gh CLI + API) |
| **Оффлайн работа** | Да | Да | Да | Да | Нет |
| **Сложность парсера** | Средне (regex, edge cases) | Низко (Unmarshal) | Низко (Unmarshal) | Низко (Unmarshal) | Высоко (HTTP API) |

### Детали по каждому формату

**YAML** — наиболее сбалансированный вариант:
- `yaml.v3` уже в зависимостях проекта — нулевые затраты на добавление
- Поддерживает комментарии (в отличие от JSON) — человек может добавить заметки
- Исследование ImprovingAgents.com показало: YAML превосходит JSON и XML по accuracy у GPT-4 на 17.7 п.п.
- Anthropic Claude показывает >90% accuracy на генерации YAML (StructEval benchmark)
- Indent-sensitive — но Go yaml.v3 парсер толерантен к вариациям

**JSON** — максимальная надёжность через constrained decoding:
- Anthropic выпустили constrained decoding (ноябрь 2025) — модель физически не может выдать невалидный JSON
- JSON Schema валидация на стороне ralph — гарантия структуры
- Проблема: не поддерживает комментарии, плохой diff, больше токенов
- Подходит для API-сценариев, менее удобен для файлового хранения

**TOML** — хорош для конфигов, не идеален для списков:
- Вложенные массивы объектов в TOML неуклюжи: `[[tasks]]` менее интуитивно чем YAML
- Потребует новую зависимость (BurntSushi/toml или pelletier/go-toml)
- Нарушает правило "только 3 direct deps" из CLAUDE.md

**GitHub Issues** — внешняя зависимость:
- Каждая задача = issue, зависимости через linked issues
- Требует GitHub API, нет оффлайн-режима
- Не подходит для self-contained ralph: задачи должны жить в репозитории

## Proposed YAML Format (детальная спецификация)

### Файл: `sprint-tasks.yaml`

```yaml
# Sprint Tasks — автоматически генерируется и обновляется ralph
# Формат: v1
version: 1

epics:
  - name: "Authentication & Security"
    tasks:
      - id: "auth-1"
        description: "Implement password hashing with bcrypt"
        source: "stories/1-1-user-model.md#AC-2"
        status: done        # open | done | skipped
        gate: false
        tags: []             # SETUP, E2E, GATE
        depends_on: []       # список id задач-блокеров
        type: feature        # feature | refactor | bugfix | test | setup
        size: M              # S | M | L | XL (оценка сложности)
        files_hint:          # подсказка какие файлы затрагивает
          - "internal/auth/hash.go"
          - "internal/auth/hash_test.go"

      - id: "auth-2"
        description: "Add JWT token generation"
        source: "stories/1-2-user-auth.md#AC-1"
        status: done
        gate: false
        tags: []
        depends_on: ["auth-1"]
        type: feature
        size: M
        files_hint:
          - "internal/auth/jwt.go"

      - id: "auth-3"
        description: "Implement JWT token validation"
        source: "stories/1-2-user-auth.md#AC-3"
        status: open
        gate: false
        tags: []
        depends_on: ["auth-2"]
        type: feature
        size: S
        files_hint: []

      - id: "auth-4"
        description: "Add refresh token rotation"
        source: "stories/1-2-user-auth.md#AC-4"
        status: open
        gate: true
        tags: [GATE]
        depends_on: ["auth-3"]
        type: feature
        size: L
        files_hint: []

  - name: "API Infrastructure"
    tasks:
      - id: "api-1"
        description: "Configure API rate limiter"
        source: "stories/1-5-api-security.md#SETUP"
        status: done
        gate: true
        tags: [SETUP, GATE]
        depends_on: []
        type: setup
        size: M
        files_hint: []
        feedback: "Use cursor-based pagination, not offset"
```

### Go-структуры для парсинга

```go
// SprintTasks represents the top-level sprint-tasks.yaml structure.
type SprintTasks struct {
    Version int     `yaml:"version"`
    Epics   []Epic  `yaml:"epics"`
}

// Epic groups related tasks under a named section.
type Epic struct {
    Name  string `yaml:"name"`
    Tasks []Task `yaml:"tasks"`
}

// Task represents a single work item in the sprint.
type Task struct {
    ID          string   `yaml:"id"`
    Description string   `yaml:"description"`
    Source      string   `yaml:"source"`
    Status      string   `yaml:"status"`      // open | done | skipped
    Gate        bool     `yaml:"gate"`
    Tags        []string `yaml:"tags"`
    DependsOn   []string `yaml:"depends_on"`
    Type        string   `yaml:"type"`         // feature | refactor | bugfix | test | setup
    Size        string   `yaml:"size"`         // S | M | L | XL
    FilesHint   []string `yaml:"files_hint"`
    Feedback    string   `yaml:"feedback,omitempty"`
}
```

### Преимущества перед текущим форматом

1. **Детерминистический парсинг**: `yaml.Unmarshal` вместо regex — нет edge cases с пробелами
2. **Зависимости**: `depends_on` позволяет параллельное выполнение независимых задач
3. **ID задач**: стабильная ссылка на задачу (не номер строки)
4. **Метаданные**: `type`, `size`, `files_hint` — обогащают контекст для агента
5. **Feedback**: встроенное поле вместо `> USER FEEDBACK:` blockquote
6. **Schema validation**: Go struct с тегами — невалидная структура вызывает ошибку Unmarshal
7. **Merge**: программный merge через Go-код вместо LLM merge mode

### Компромиссы

1. **Менее читабелен** для человека при просмотре в текстовом редакторе
2. **Больше boilerplate**: пустые поля (`tags: []`, `depends_on: []`) занимают место
3. **Сложнее prompt для bridge**: LLM должна генерировать валидный YAML с правильными типами
4. **Version migration**: при изменении формата нужна миграция

### Минимальный формат (облегчённый)

Для сокращения boilerplate — только обязательные поля, остальные опциональные с defaults:

```yaml
version: 1
epics:
  - name: "Authentication"
    tasks:
      - id: auth-1
        description: "Implement password hashing"
        source: "1-1-user-model.md#AC-2"
        status: done

      - id: auth-2
        description: "Add JWT token generation"
        source: "1-2-user-auth.md#AC-1"
        status: open
        gate: true
        depends_on: [auth-1]
```

Необязательные поля (`tags`, `type`, `size`, `files_hint`, `feedback`) опускаются, Go struct имеет `omitempty`.

## Обратная совместимость

### Стратегия: dual-format с автодетектом

```
runner.LoadTasks() →
  if exists("sprint-tasks.yaml") → parseYAML()
  else if exists("sprint-tasks.md") → parseMarkdown() (legacy)
  else → error
```

### Migration path: `ralph migrate-tasks`

Команда конвертации:
1. Читает `sprint-tasks.md` через текущий regex-парсер
2. Генерирует уникальные ID для каждой задачи (из source + порядкового номера)
3. Определяет epic по `## Header`
4. Парсит `[GATE]`, `[SETUP]`, `[E2E]` теги
5. Парсит `> USER FEEDBACK:` в поле `feedback`
6. Записывает `sprint-tasks.yaml`
7. Опционально: удаляет или архивирует `sprint-tasks.md`

### Фазы перехода

| Фаза | Действие | Срок |
|------|----------|------|
| 1 | Добавить YAML-парсер рядом с regex | 1 story |
| 2 | Bridge генерирует YAML (новый prompt) | 1 story |
| 3 | `ralph migrate-tasks` CLI команда | 1 story |
| 4 | Regex-парсер → deprecated, затем удалён | 1 story |

### Влияние на кодовую базу

Затронутые компоненты при переходе на YAML:

- `config/constants.go` — regex'ы могут стать deprecated (или оставлены для legacy)
- `config/shared/sprint-tasks-format.md` — заменяется на YAML-спецификацию
- `runner/scan.go` — `ScanTasks()` переписывается с regex на `yaml.Unmarshal`
- `runner/runner.go` — логика выбора следующей задачи учитывает `depends_on`
- `bridge/prompts/bridge.md` — prompt для генерации YAML вместо markdown
- `bridge/bridge.go` — merge mode через Go-код вместо LLM
- `runner/prompts/execute.md` — формат-контракт обновляется

## Как это делают другие

### Devin (Cognition)

Devin не использует persistent task file. Вместо этого:
- Планирование в runtime: LLM создаёт план, хранит его в контекстном окне
- Каждый шаг выполняется и результат добавляется к контексту
- Нет файлового формата задач — весь state в памяти агента
- Плюс: гибкость; Минус: потеря плана при context overflow

### SWE-Agent (Princeton)

- Использует `ImplementationTask` объекты в JSON-подобном формате
- Иерархия: задача → logical_task → atomic_tasks (атомарные изменения кода)
- Формат ближе к AST-ориентированному, чем к чеклисту
- State хранится в event log, а не в файле

### Amazon Kiro

- Spec-driven development: prompt → requirements.md (EARS) → design.md → tasks
- Задачи в markdown, но генерируются системой (не LLM-free-form)
- Три фазы: Requirements → Design → Implementation Tasks
- Ближе всего к ralph по концепции, но использует IDE-интеграцию

### OpenHands (ex-OpenDevin)

- Event-sourced state model: все действия в event log
- Детерминистический replay из event stream
- TaskTrackerTool для управления задачами
- Нет файлового формата — state в runtime

### Linear / Jira

- GraphQL API (Linear) / REST API (Jira) для задач
- JSON-структуры с полями: title, description, priority, status, assignee, labels, dependencies
- Не self-contained: требуют внешний сервис
- Модель данных богаче, чем нужно для ralph

### Taskwarrior

- JSON-хранилище (`task.data`) с полями: description, status, priority, project, tags, depends, due, entry, modified
- CLI-first: `task add`, `task done`, `task modify`
- Зависимости через `depends:UUID`
- Минус: собственный формат JSON, не стандартный

### todo.txt

- Одна строка = одна задача: `(A) 2026-03-07 Task description +Project @Context`
- Приоритеты: `(A)`, `(B)`, `(C)`
- Проекты: `+ProjectName`, контексты: `@context`
- Завершение: `x 2026-03-07 Task description`
- Плюс: максимально простой; Минус: нет зависимостей, нет вложенности

### Сводка по индустрии

| Инструмент | Формат задач | Зависимости | Persistent file |
|------------|-------------|-------------|-----------------|
| Devin | Нет (in-memory) | Нет | Нет |
| SWE-Agent | JSON-like объекты | Иерархические | Нет (event log) |
| Kiro | Markdown (generated) | Нет | Да |
| OpenHands | Event stream | Нет | Нет (runtime) |
| Linear | JSON (GraphQL) | Да (linked) | Нет (SaaS) |
| Taskwarrior | JSON | Да (depends) | Да |
| todo.txt | Plain text | Нет | Да |
| **ralph** | **Markdown checklist** | **Нет** | **Да** |

Вывод: ralph уникален тем, что использует persistent file-based tasks.
Большинство AI-агентов хранят state в runtime (event log / context window).
Файловый подход ralph даёт преимущество: задачи выживают при перезапуске агента.

## Рекомендация

### Основной вариант: YAML (sprint-tasks.yaml)

**Рекомендация: перейти на YAML формат** по следующим причинам:

1. **Нулевые затраты на зависимости**: `yaml.v3` уже в `go.mod`
2. **Детерминистический парсинг**: `yaml.Unmarshal` вместо хрупкого regex
3. **Расширяемость**: зависимости, метаданные, feedback — без ломки парсера
4. **LLM-совместимость**: YAML показывает лучшую accuracy чем JSON для nested data (ImprovingAgents.com benchmark, +17.7 п.п.)
5. **Программный merge**: Go-код вместо LLM merge mode — устраняет самую хрупкую часть текущей системы
6. **Git-friendly**: построчный diff работает хорошо

### Минимальный первый шаг

Начать с облегчённого формата (только обязательные поля), добавлять `depends_on`, `type`, `size` по мере потребности. Это соответствует принципу YAGNI.

### Альтернативный вариант: JSON + constrained decoding

Если Anthropic API доступен (не CLI-only режим), JSON + constrained decoding даёт 100% гарантию валидности. Но ralph работает через `claude` CLI, где constrained decoding недоступен — поэтому JSON теряет своё главное преимущество.

### Не рекомендуется

- **TOML**: новая зависимость + неуклюжий синтаксис для массивов объектов
- **GitHub Issues**: внешняя зависимость, нет оффлайн-режима
- **Оставить markdown as-is**: regex-парсинг хрупок, merge mode ненадёжен, нет расширяемости

## Источники

1. [ImprovingAgents.com — Which Nested Data Format Do LLMs Understand Best?](https://www.improvingagents.com/blog/best-nested-data-format/) — benchmark YAML vs JSON vs XML для LLM accuracy
2. [Anthropic — Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs) — constrained decoding для JSON
3. [StructEval: Benchmarking LLMs' Capabilities to Generate Structural Outputs](https://arxiv.org/html/2505.20139v1) — >90% accuracy на YAML/JSON генерации
4. [Medium — Beyond JSON: Picking the Right Format for LLM Pipelines](https://medium.com/@michael.hannecke/beyond-json-picking-the-right-format-for-llm-pipelines-b65f15f77f7d) — сравнение форматов для LLM pipelines
5. [OpenAI Community — Markdown is 15% more token efficient than JSON](https://community.openai.com/t/markdown-is-15-more-token-efficient-than-json/841742) — токен-эффективность форматов
6. [Todo.txt Format](https://github.com/todotxt/todo.txt) — спецификация todo.txt
7. [LWN.net — Managing tasks with todo.txt and Taskwarrior](https://lwn.net/Articles/824333/) — сравнение CLI task managers
8. [Kiro — Spec-driven development](https://kiro.dev/) — Amazon Kiro spec-driven подход
9. [OpenHands CodeAct 2.1](https://openhands.dev/blog/openhands-codeact-21-an-open-state-of-the-art-software-development-agent) — event-sourced state model
10. [Devin — Coding Agents 101](https://devin.ai/agents101) — in-memory planning
11. [WebcrawlerAPI — Markdown vs YAML for LLM Prompts](https://webcrawlerapi.com/blog/markdown-vs-yaml-choosing-the-right-format-for-llm-prompts) — практическое сравнение
12. [YAML Over JSON in LLM Applications](https://blog.tashif.codes/blog/JSON-YAML-LLM) — аргументы в пользу YAML
