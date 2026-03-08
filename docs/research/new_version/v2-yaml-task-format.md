# V2: YAML Task Format — полная спецификация

Дата: 2026-03-07
Статус: проектный документ (архитектура данных)
Основа: анализ из `variant-d-task-format.md`, текущий код `runner/scan.go` + `bridge/bridge.go`

## Мотивация

Текущий `sprint-tasks.md` имеет три фундаментальные проблемы:

1. **Хрупкий regex-парсинг** — 4 regex'а в `config/constants.go` (`TaskOpenRegex`, `TaskDoneRegex`, `GateTagRegex`, `SourceFieldRegex`), ломаются от `- []` (без пробела), `* [ ]`, `[X]` (заглавная)
2. **LLM merge mode** — 244-строчный промпт `bridge/prompts/bridge.md`, 12 правил merge, LLM теряет `[x]` статусы
3. **Бедные метаданные** — нет ID, зависимостей, оценки сложности, подсказок по файлам

---

## 1. YAML Schema (sprint-tasks.yaml)

### Полная схема

```yaml
# Sprint Tasks — генерируется ralph bridge, обновляется ralph merge
# Формат: v1 — НЕ редактировать вручную (используй ralph CLI)
version: 1

# Метаданные генерации — заполняет bridge при создании
generated: "2026-03-07T12:00:00Z"    # ISO 8601 — время последней генерации/merge

# Документы-источники — для трассировки и инвалидации кэша
source_docs:
  - path: "docs/stories/1-1-auth.md"
    hash: "abc123def"                 # SHA-256 первые 9 символов (достаточно для идентификации)
  - path: "docs/stories/1-2-jwt.md"
    hash: "456789abc"

# Задачи сгруппированы по эпикам
epics:
  - name: "Authentication & Security"
    tasks:
      - id: 1
        description: "Implement password hashing with bcrypt"
        status: done           # open | done | skipped
        gate: true
        tags: []               # SETUP | E2E (пустой массив если нет)
        source: "1-1-auth.md#AC-1,AC-2"
        depends_on: []         # список id задач-блокеров (пустой массив если нет)
        files_hint: []         # подсказка по затрагиваемым путям
        size: M                # S | M | L (опционально, default M)
        feedback: ""           # USER FEEDBACK от gate review (пустая строка если нет)

      - id: 2
        description: "Add JWT token generation and validation"
        status: open
        gate: false
        tags: []
        source: "1-2-jwt.md#AC-1,AC-2,AC-3"
        depends_on: [1]
        files_hint:
          - "internal/auth/jwt.go"
          - "internal/auth/jwt_test.go"
        size: L
        feedback: ""

      - id: 3
        description: "Add refresh token rotation"
        status: open
        gate: true
        tags: []
        source: "1-2-jwt.md#AC-4"
        depends_on: [2]
        files_hint: []
        size: M
        feedback: ""

  - name: "API Infrastructure"
    tasks:
      - id: 4
        description: "Configure API rate limiter"
        status: done
        gate: true
        tags: [SETUP]
        source: "1-5-api-security.md#SETUP"
        depends_on: []
        files_hint: []
        size: M
        feedback: "Use cursor-based pagination, not offset"

      - id: 5
        description: "Implement pagination for list endpoints"
        status: open
        gate: false
        tags: []
        source: "1-3-api-design.md#AC-2"
        depends_on: [4]
        files_hint: []
        size: M
        feedback: ""

      - id: 6
        description: "Full deployment pipeline test"
        status: open
        gate: false
        tags: [E2E]
        source: "2-1-deployment.md#E2E"
        depends_on: [4, 5]
        files_hint: []
        size: L
        feedback: ""
```

### Правила схемы

| Поле | Тип | Обязательно | Default | Ограничения |
|------|-----|-------------|---------|-------------|
| `version` | int | да | — | Всегда `1` (для будущей миграции) |
| `generated` | string | да | — | ISO 8601 с часовым поясом |
| `source_docs` | []SourceDoc | нет | [] | Опционально, заполняется bridge |
| `epics` | []Epic | да | — | Минимум 1 эпик |
| `epic.name` | string | да | — | Непустая строка |
| `epic.tasks` | []Task | да | — | Минимум 1 задача в эпике |
| `task.id` | int | да | — | Уникальный в рамках всего файла, монотонно растёт |
| `task.description` | string | да | — | 1-500 символов |
| `task.status` | string | да | — | Enum: `open`, `done`, `skipped` |
| `task.gate` | bool | да | false | — |
| `task.tags` | []string | да | [] | Enum элементов: `SETUP`, `E2E` |
| `task.source` | string | да | — | Формат: `<filename>#<anchor>` |
| `task.depends_on` | []int | да | [] | Список id задач из этого же файла |
| `task.files_hint` | []string | нет | [] | Относительные пути от корня проекта |
| `task.size` | string | нет | "M" | Enum: `S`, `M`, `L` |
| `task.feedback` | string | нет | "" | USER FEEDBACK, пустая строка = нет |

### Почему целочисленные ID, а не строковые

Variant-D предлагал строковые ID (`auth-1`, `api-2`). Целочисленные лучше по нескольким причинам:

1. **Автоинкремент** — `maxID + 1` при merge, нет коллизий
2. **depends_on** читается легче: `depends_on: [1, 3]` vs `depends_on: ["auth-1", "api-3"]`
3. **Нет проблемы именования** — не нужно придумывать префикс при генерации
4. **YAML якоря** — целые числа не конфликтуют с YAML спецсимволами

---

## 2. Go Structs

```go
package config

import "time"

// SprintTasks — корневая структура sprint-tasks.yaml.
// Version = 1 для текущей схемы. Все поля обязательны для
// десериализации (yaml.v3 не пропускает отсутствующие при strict mode).
type SprintTasks struct {
    Version    int         `yaml:"version"`
    Generated  time.Time   `yaml:"generated"`
    SourceDocs []SourceDoc `yaml:"source_docs,omitempty"`
    Epics      []Epic      `yaml:"epics"`
}

// SourceDoc — ссылка на документ-источник с хешем для инвалидации.
type SourceDoc struct {
    Path string `yaml:"path"`
    Hash string `yaml:"hash"`
}

// Epic группирует задачи под именованным разделом.
type Epic struct {
    Name  string `yaml:"name"`
    Tasks []Task `yaml:"tasks"`
}

// TaskStatus — допустимые значения поля Task.Status.
const (
    TaskStatusOpen    = "open"
    TaskStatusDone    = "done"
    TaskStatusSkipped = "skipped"
)

// TaskSize — допустимые значения поля Task.Size.
const (
    TaskSizeS = "S"
    TaskSizeM = "M"
    TaskSizeL = "L"
)

// Task — единица работы в спринте.
// ID уникален в рамках всего файла (не только эпика).
// DependsOn содержит ID задач-блокеров — runner не начнёт
// задачу, пока все зависимости не имеют status=done.
type Task struct {
    ID          int      `yaml:"id"`
    Description string   `yaml:"description"`
    Status      string   `yaml:"status"`
    Gate        bool     `yaml:"gate"`
    Tags        []string `yaml:"tags"`
    Source      string   `yaml:"source"`
    DependsOn   []int    `yaml:"depends_on"`
    FilesHint   []string `yaml:"files_hint,omitempty"`
    Size        string   `yaml:"size,omitempty"`
    Feedback    string   `yaml:"feedback,omitempty"`
}

// IsOpen возвращает true если задача ещё не выполнена и не пропущена.
func (t Task) IsOpen() bool {
    return t.Status == TaskStatusOpen
}

// IsDone возвращает true если задача завершена.
func (t Task) IsDone() bool {
    return t.Status == TaskStatusDone
}

// IsBlocked возвращает true если хотя бы одна зависимость
// ещё не имеет статус done. Принимает индекс id → Task
// для проверки статусов зависимостей.
func (t Task) IsBlocked(index map[int]*Task) bool {
    for _, depID := range t.DependsOn {
        dep, ok := index[depID]
        if !ok || !dep.IsDone() {
            return true
        }
    }
    return false
}
```

### Валидация

```go
// Validate проверяет структурную целостность SprintTasks.
// Возвращает все найденные ошибки (не останавливается на первой).
func (st *SprintTasks) Validate() error {
    var errs []string

    if st.Version != 1 {
        errs = append(errs, fmt.Sprintf("unsupported version: %d", st.Version))
    }

    ids := make(map[int]bool)
    for _, epic := range st.Epics {
        if epic.Name == "" {
            errs = append(errs, "epic with empty name")
        }
        for _, task := range epic.Tasks {
            // Уникальность ID
            if ids[task.ID] {
                errs = append(errs, fmt.Sprintf("duplicate task id: %d", task.ID))
            }
            ids[task.ID] = true

            // Обязательные поля
            if task.Description == "" {
                errs = append(errs, fmt.Sprintf("task %d: empty description", task.ID))
            }

            // Enum валидация
            switch task.Status {
            case TaskStatusOpen, TaskStatusDone, TaskStatusSkipped:
                // ok
            default:
                errs = append(errs, fmt.Sprintf("task %d: invalid status %q", task.ID, task.Status))
            }

            // Source формат
            if !strings.Contains(task.Source, "#") {
                errs = append(errs, fmt.Sprintf("task %d: source missing # anchor", task.ID))
            }

            // Зависимости ссылаются на существующие ID
            for _, depID := range task.DependsOn {
                if !ids[depID] {
                    // depID может быть определён позже — проверим после полного обхода
                }
            }
        }
    }

    // Вторичная проверка зависимостей (после сбора всех ID)
    for _, epic := range st.Epics {
        for _, task := range epic.Tasks {
            for _, depID := range task.DependsOn {
                if !ids[depID] {
                    errs = append(errs, fmt.Sprintf("task %d: depends_on unknown id %d", task.ID, depID))
                }
            }
        }
    }

    // Циклические зависимости
    if cycle := detectCycle(st); cycle != nil {
        errs = append(errs, fmt.Sprintf("dependency cycle: %v", cycle))
    }

    if len(errs) > 0 {
        return fmt.Errorf("sprint-tasks validation: %s", strings.Join(errs, "; "))
    }
    return nil
}
```

---

## 3. Программный Merge

### Алгоритм MergeTasks

```
ВХОД: existing SprintTasks (текущий файл), incoming SprintTasks (новые задачи от bridge)
ВЫХОД: merged SprintTasks, diff MergeDiff

1. ПОСТРОИТЬ индексы:
   existingByID   = map[id → *Task]       (из existing)
   existingByKey  = map[epicName+source → *Task]  (для дедупликации)
   maxID          = max(all existing task IDs)

2. ДЛЯ КАЖДОГО эпика в incoming:
   a. НАЙТИ соответствующий эпик в existing по name (exact match)
      - Если не найден → создать новый эпик в merged

   b. ДЛЯ КАЖДОЙ задачи в incoming.epic.tasks:
      i.   ПОИСК дубликата по ключу (epicName + source):
           - Если найден И existing.status == "done":
             → СОХРАНИТЬ existing задачу как есть (done защищён)
             → Если incoming.description != existing.description:
               → СОЗДАТЬ новую задачу с новым ID, status=open
               → Записать в diff: "new work on completed task"
           - Если найден И existing.status == "open":
             → ОБНОВИТЬ description из incoming (если отличается)
             → НЕ менять status, gate, feedback
             → Записать в diff: "updated description"
           - Если НЕ найден:
             → maxID++
             → ПРИСВОИТЬ id = maxID
             → ДОБАВИТЬ задачу в соответствующий эпик
             → Записать в diff: "added new task"

3. ПЕРЕСЧИТАТЬ depends_on:
   - incoming задачи ссылаются на ID из incoming namespace
   - Нужна таблица маппинга: incomingID → mergedID
   - Переписать все depends_on через маппинг

4. ОБНОВИТЬ generated timestamp

5. ВЕРНУТЬ merged + diff
```

### Go-код ядра

```go
package tasks

import (
    "fmt"
    "time"
)

// MergeDiff описывает изменения, произведённые merge'ем.
type MergeDiff struct {
    Added       []DiffEntry // новые задачи
    Updated     []DiffEntry // обновлённые описания open задач
    NewWorkDone []DiffEntry // новая работа над done задачами
    Preserved   int         // количество сохранённых без изменений
}

// DiffEntry — одна запись об изменении.
type DiffEntry struct {
    TaskID      int
    EpicName    string
    Description string
    OldDesc     string // только для Updated
}

// MergeTasks объединяет existing и incoming задачи.
// Инварианты:
//   - done задачи НИКОГДА не меняют status
//   - done задачи НИКОГДА не меняют description
//   - ID уникальны в пределах файла
//   - depends_on ссылки корректно переписаны
func MergeTasks(existing, incoming *SprintTasks) (*SprintTasks, *MergeDiff) {
    diff := &MergeDiff{}
    merged := &SprintTasks{
        Version:   1,
        Generated: time.Now().UTC(),
    }

    // Копировать source_docs из incoming (актуальные ссылки)
    merged.SourceDocs = incoming.SourceDocs

    // 1. Построить индексы existing
    type taskKey struct {
        epicName string
        source   string
    }
    existingByKey := make(map[taskKey]*Task)
    maxID := 0

    // Глубокое копирование existing эпиков в merged
    epicIndex := make(map[string]int) // epicName → index в merged.Epics
    for _, epic := range existing.Epics {
        mergedEpic := Epic{
            Name:  epic.Name,
            Tasks: make([]Task, len(epic.Tasks)),
        }
        copy(mergedEpic.Tasks, epic.Tasks)

        epicIndex[epic.Name] = len(merged.Epics)
        merged.Epics = append(merged.Epics, mergedEpic)

        for i := range mergedEpic.Tasks {
            t := &mergedEpic.Tasks[i]
            existingByKey[taskKey{epic.Name, t.Source}] = t
            if t.ID > maxID {
                maxID = t.ID
            }
        }
    }

    // 2. Маппинг incoming ID → merged ID (для depends_on)
    idMap := make(map[int]int)

    // 3. Обработать incoming задачи
    for _, inEpic := range incoming.Epics {
        for _, inTask := range inEpic.Tasks {
            key := taskKey{inEpic.Name, inTask.Source}
            existing, found := existingByKey[key]

            if found && existing.IsDone() {
                // Done задача — сохранить. Если описание изменилось,
                // создать НОВУЮ open задачу для дополнительной работы.
                idMap[inTask.ID] = existing.ID
                diff.Preserved++

                if inTask.Description != existing.Description {
                    maxID++
                    newTask := Task{
                        ID:          maxID,
                        Description: inTask.Description,
                        Status:      TaskStatusOpen,
                        Gate:        inTask.Gate,
                        Tags:        inTask.Tags,
                        Source:      inTask.Source,
                        DependsOn:   nil, // переписать позже
                        FilesHint:   inTask.FilesHint,
                        Size:        inTask.Size,
                    }
                    idMap[inTask.ID] = newTask.ID

                    // Добавить в соответствующий эпик
                    idx, ok := epicIndex[inEpic.Name]
                    if !ok {
                        idx = len(merged.Epics)
                        epicIndex[inEpic.Name] = idx
                        merged.Epics = append(merged.Epics, Epic{Name: inEpic.Name})
                    }
                    merged.Epics[idx].Tasks = append(merged.Epics[idx].Tasks, newTask)

                    diff.NewWorkDone = append(diff.NewWorkDone, DiffEntry{
                        TaskID:      newTask.ID,
                        EpicName:    inEpic.Name,
                        Description: inTask.Description,
                        OldDesc:     existing.Description,
                    })
                }

            } else if found && existing.IsOpen() {
                // Open задача — обновить описание, сохранить остальное
                idMap[inTask.ID] = existing.ID

                if inTask.Description != existing.Description {
                    diff.Updated = append(diff.Updated, DiffEntry{
                        TaskID:      existing.ID,
                        EpicName:    inEpic.Name,
                        Description: inTask.Description,
                        OldDesc:     existing.Description,
                    })
                    existing.Description = inTask.Description
                } else {
                    diff.Preserved++
                }

            } else {
                // Новая задача
                maxID++
                newTask := inTask
                newTask.ID = maxID
                newTask.Status = TaskStatusOpen
                idMap[inTask.ID] = newTask.ID

                idx, ok := epicIndex[inEpic.Name]
                if !ok {
                    idx = len(merged.Epics)
                    epicIndex[inEpic.Name] = idx
                    merged.Epics = append(merged.Epics, Epic{Name: inEpic.Name})
                }
                merged.Epics[idx].Tasks = append(merged.Epics[idx].Tasks, newTask)

                diff.Added = append(diff.Added, DiffEntry{
                    TaskID:      newTask.ID,
                    EpicName:    inEpic.Name,
                    Description: newTask.Description,
                })
            }
        }
    }

    // 4. Переписать depends_on через idMap
    for ei := range merged.Epics {
        for ti := range merged.Epics[ei].Tasks {
            task := &merged.Epics[ei].Tasks[ti]
            for di, depID := range task.DependsOn {
                if mapped, ok := idMap[depID]; ok {
                    task.DependsOn[di] = mapped
                }
            }
        }
    }

    return merged, diff
}
```

### Ключ дедупликации

Дедупликация по паре `(epicName, source)`:
- `epicName` — к какому эпику относится задача
- `source` — `"1-2-jwt.md#AC-1,AC-2"` — привязка к acceptance criteria

Почему не по `description`:
- Description может меняться при обновлении story (это корректное обновление)
- Source стабилен — один AC = одна задача, даже если описание переформулировано

---

## 4. LLM Output Format

### Claude генерирует JSON, Go конвертирует в YAML

Причины:
- JSON Schema + constrained decoding (Anthropic API) — 100% валидность структуры
- Через `claude` CLI constrained decoding недоступен, но JSON парсер строже YAML (нет indent-чувствительности)
- Go `encoding/json` → `gopkg.in/yaml.v3` конвертация тривиальна через промежуточные Go struct'ы

### JSON Schema для bridge output

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["version", "epics"],
  "properties": {
    "version": { "type": "integer", "const": 1 },
    "epics": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["name", "tasks"],
        "properties": {
          "name": { "type": "string", "minLength": 1 },
          "tasks": {
            "type": "array",
            "minItems": 1,
            "items": {
              "type": "object",
              "required": ["id", "description", "source"],
              "properties": {
                "id": { "type": "integer", "minimum": 1 },
                "description": { "type": "string", "minLength": 1, "maxLength": 500 },
                "source": { "type": "string", "pattern": "^.+#.+$" },
                "gate": { "type": "boolean", "default": false },
                "tags": {
                  "type": "array",
                  "items": { "type": "string", "enum": ["SETUP", "E2E"] },
                  "default": []
                },
                "depends_on": {
                  "type": "array",
                  "items": { "type": "integer", "minimum": 1 },
                  "default": []
                },
                "files_hint": {
                  "type": "array",
                  "items": { "type": "string" },
                  "default": []
                },
                "size": { "type": "string", "enum": ["S", "M", "L"], "default": "M" }
              }
            }
          }
        }
      }
    }
  }
}
```

### Bridge prompt (упрощённый фрагмент)

Новый промпт bridge заменяет markdown-инструкции на JSON output:

```
Output ONLY a JSON object with this structure (no explanations, no code fences):
{
  "version": 1,
  "epics": [
    {
      "name": "Epic Name",
      "tasks": [
        {
          "id": 1,
          "description": "Task description",
          "source": "story-file.md#AC-1,AC-2",
          "gate": true,
          "tags": [],
          "depends_on": [],
          "files_hint": ["src/auth/"],
          "size": "M"
        }
      ]
    }
  ]
}
```

Bridge **НЕ генерирует** поля `status`, `feedback` — они управляются только программно:
- `status` устанавливается Go-кодом (bridge всегда создаёт `open`, runner ставит `done`/`skipped`)
- `feedback` заполняется gate-промптом

### Конвейер обработки

```
Story файлы → Bridge Prompt → Claude CLI → JSON stdout
    → json.Unmarshal → SprintTasks struct
    → Validate()
    → MergeTasks(existing, incoming) если файл существует
    → yaml.Marshal → sprint-tasks.yaml
```

---

## 5. Адаптация Runner (scan.go)

### Новый ScanTasks

```go
package runner

import (
    "fmt"
    "os"

    "github.com/bmad-ralph/bmad-ralph/config"
    "gopkg.in/yaml.v3"
)

// ScanResult v2 — обогащённый результат сканирования.
// Совместим с текущим API через методы HasOpenTasks()/HasDoneTasks().
type ScanResult struct {
    OpenTasks  []TaskEntry
    DoneTasks  []TaskEntry
    AllTasks   *config.SprintTasks // полная структура для расширенного доступа
}

// TaskEntry v2 — расширенный за счёт структурированных данных.
// Поле Text сохранено для обратной совместимости с prompt templates.
type TaskEntry struct {
    LineNum int    // не используется в YAML-режиме (всегда 0)
    Text    string // description задачи (для шаблонов)
    HasGate bool
    TaskID  int    // ID задачи из YAML
}

// ScanTasksYAML парсит sprint-tasks.yaml и возвращает ScanResult.
// Заменяет regex-парсинг из ScanTasks.
func ScanTasksYAML(path string) (ScanResult, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return ScanResult{}, fmt.Errorf("runner: scan tasks: %w", err)
    }

    var st config.SprintTasks
    if err := yaml.Unmarshal(data, &st); err != nil {
        return ScanResult{}, fmt.Errorf("runner: scan tasks: parse yaml: %w", err)
    }

    if err := st.Validate(); err != nil {
        return ScanResult{}, fmt.Errorf("runner: scan tasks: %w", err)
    }

    // Построить индекс для проверки зависимостей
    taskIndex := make(map[int]*config.Task)
    for ei := range st.Epics {
        for ti := range st.Epics[ei].Tasks {
            t := &st.Epics[ei].Tasks[ti]
            taskIndex[t.ID] = t
        }
    }

    var result ScanResult
    result.AllTasks = &st

    for _, epic := range st.Epics {
        for _, task := range epic.Tasks {
            // Пропустить заблокированные задачи (зависимости не выполнены)
            if task.IsOpen() && task.IsBlocked(taskIndex) {
                continue // не добавляем в OpenTasks — runner не должен их брать
            }

            entry := TaskEntry{
                Text:    task.Description,
                HasGate: task.Gate,
                TaskID:  task.ID,
            }

            switch task.Status {
            case config.TaskStatusOpen:
                result.OpenTasks = append(result.OpenTasks, entry)
            case config.TaskStatusDone:
                result.DoneTasks = append(result.DoneTasks, entry)
            case config.TaskStatusSkipped:
                result.DoneTasks = append(result.DoneTasks, entry)
            }
        }
    }

    if !result.HasAnyTasks() {
        return ScanResult{}, fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)
    }

    return result, nil
}
```

### Dual-format автодетект

```go
// ScanTasksAuto определяет формат и вызывает соответствующий парсер.
func ScanTasksAuto(projectRoot string) (ScanResult, error) {
    yamlPath := filepath.Join(projectRoot, "sprint-tasks.yaml")
    mdPath := filepath.Join(projectRoot, "sprint-tasks.md")

    // YAML имеет приоритет
    if _, err := os.Stat(yamlPath); err == nil {
        return ScanTasksYAML(yamlPath)
    }

    // Fallback на markdown (legacy)
    if _, err := os.Stat(mdPath); err == nil {
        data, err := os.ReadFile(mdPath)
        if err != nil {
            return ScanResult{}, fmt.Errorf("runner: scan tasks: %w", err)
        }
        return ScanTasks(string(data)) // текущий regex-парсер
    }

    return ScanResult{}, fmt.Errorf("runner: scan tasks: %w", config.ErrNoTasks)
}
```

### Обновление статуса задачи

Текущий runner помечает задачу как done через текстовую замену `- [ ]` → `- [x]`.
С YAML это становится программной операцией:

```go
// MarkTaskDone находит задачу по ID и ставит status=done.
// Перезаписывает sprint-tasks.yaml.
func MarkTaskDone(path string, taskID int) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("runner: mark done: %w", err)
    }

    var st config.SprintTasks
    if err := yaml.Unmarshal(data, &st); err != nil {
        return fmt.Errorf("runner: mark done: parse: %w", err)
    }

    found := false
    for ei := range st.Epics {
        for ti := range st.Epics[ei].Tasks {
            if st.Epics[ei].Tasks[ti].ID == taskID {
                st.Epics[ei].Tasks[ti].Status = config.TaskStatusDone
                found = true
                break
            }
        }
        if found {
            break
        }
    }

    if !found {
        return fmt.Errorf("runner: mark done: task %d not found", taskID)
    }

    out, err := yaml.Marshal(&st)
    if err != nil {
        return fmt.Errorf("runner: mark done: marshal: %w", err)
    }

    return os.WriteFile(path, out, 0644)
}
```

---

## 6. Migration Path

### Команда `ralph migrate-tasks`

```go
// MigrateFromMarkdown конвертирует sprint-tasks.md → sprint-tasks.yaml.
func MigrateFromMarkdown(mdPath, yamlPath string) error {
    data, err := os.ReadFile(mdPath)
    if err != nil {
        return fmt.Errorf("migrate: read md: %w", err)
    }

    lines := strings.Split(string(data), "\n")
    st := config.SprintTasks{
        Version:   1,
        Generated: time.Now().UTC(),
    }

    var currentEpic *config.Epic
    taskID := 0

    for i := 0; i < len(lines); i++ {
        line := lines[i]

        // Epic header: ## Name
        if strings.HasPrefix(strings.TrimSpace(line), "## ") {
            name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "## "))
            st.Epics = append(st.Epics, config.Epic{Name: name})
            currentEpic = &st.Epics[len(st.Epics)-1]
            continue
        }

        // Задача (open или done)
        isOpen := config.TaskOpenRegex.MatchString(line)
        isDone := config.TaskDoneRegex.MatchString(line)
        if !isOpen && !isDone {
            continue
        }

        taskID++
        task := config.Task{
            ID:   taskID,
            Tags: []string{},
            DependsOn: []int{},
        }

        // Status
        if isDone {
            task.Status = config.TaskStatusDone
        } else {
            task.Status = config.TaskStatusOpen
        }

        // Description (убрать checkbox marker)
        desc := strings.TrimSpace(line)
        if idx := strings.Index(desc, "] "); idx >= 0 {
            desc = strings.TrimSpace(desc[idx+2:])
        }

        // Gate tag
        if config.GateTagRegex.MatchString(desc) {
            task.Gate = true
            desc = strings.TrimSpace(strings.ReplaceAll(desc, "[GATE]", ""))
        }

        // Service tags
        for _, tag := range []string{"SETUP", "E2E"} {
            marker := "[" + tag + "]"
            if strings.Contains(desc, marker) {
                task.Tags = append(task.Tags, tag)
                desc = strings.TrimSpace(strings.ReplaceAll(desc, marker, ""))
            }
        }

        task.Description = desc
        task.Size = config.TaskSizeM // default

        // Source (следующая строка)
        if i+1 < len(lines) && config.SourceFieldRegex.MatchString(lines[i+1]) {
            srcLine := strings.TrimSpace(lines[i+1])
            srcLine = strings.TrimPrefix(srcLine, "source:")
            task.Source = strings.TrimSpace(srcLine)
            i++ // пропустить source строку
        }

        // Feedback (следующая строка после source)
        if i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "> USER FEEDBACK:") {
            fb := strings.TrimSpace(lines[i+1])
            fb = strings.TrimPrefix(fb, "> USER FEEDBACK:")
            task.Feedback = strings.TrimSpace(fb)
            i++
        }

        // Добавить в текущий эпик (или создать default)
        if currentEpic == nil {
            st.Epics = append(st.Epics, config.Epic{Name: "Default"})
            currentEpic = &st.Epics[len(st.Epics)-1]
        }
        currentEpic.Tasks = append(currentEpic.Tasks, task)
    }

    // Валидация
    if err := st.Validate(); err != nil {
        return fmt.Errorf("migrate: validation: %w", err)
    }

    // Сериализация
    out, err := yaml.Marshal(&st)
    if err != nil {
        return fmt.Errorf("migrate: marshal: %w", err)
    }

    return os.WriteFile(yamlPath, out, 0644)
}
```

### Фазы перехода

| Фаза | Что происходит | Код | Обратная совместимость |
|------|---------------|-----|----------------------|
| 0 (текущее) | sprint-tasks.md + regex | scan.go | — |
| 1 | Добавить YAML structs + парсер | config/tasks.go, runner/scan_yaml.go | Читает оба формата (ScanTasksAuto) |
| 2 | Bridge генерирует JSON → конвертация в YAML | bridge/bridge.go, новый промпт | Merge mode через Go-код, не LLM |
| 3 | `ralph migrate-tasks` CLI | cmd/ralph/migrate.go | Конвертация md → yaml |
| 4 | Удалить regex-парсер | constants.go чистка, scan.go чистка | Только YAML |

### Когда убрать markdown support

Markdown-парсер можно удалить когда:
1. Все проекты-пользователи ralph перешли на YAML (отслеживать через warning в логе при использовании md-формата)
2. Прошло минимум 2 релиза с dual support
3. `ralph migrate-tasks` включена в документацию миграции

Критерий: если за 30 дней ни одного md-файла не прочитано (метрика в логах) — безопасно удалять.

---

## 7. Сравнение: текущий формат vs V2

### Парсинг

| Аспект | Markdown (текущий) | YAML V2 |
|--------|-------------------|---------|
| Парсер | 4 regex, 70 строк scan.go | yaml.Unmarshal, ~30 строк |
| Edge cases | `- []`, `* [ ]`, `[X]`, табы vs пробелы | Нет (yaml.v3 толерантен) |
| Валидация | Нет (regex матчит или нет) | Полная: типы, enum, зависимости, циклы |
| Ошибки парсинга | Молча пропускает невалидные строки | Явная ошибка с позицией |

### Merge

| Аспект | LLM Merge (текущий) | Программный Merge V2 |
|--------|---------------------|---------------------|
| Надёжность | ~85% (LLM теряет [x]) | 100% (Go-код, детерминистичный) |
| Скорость | 30-60 сек (Claude API call) | <10 мс |
| Стоимость | ~0.01-0.05 USD за merge | 0 USD |
| Diff | Нет (LLM возвращает весь файл) | MergeDiff struct с деталями |
| Тестируемость | Невозможно unit-test LLM | Полное покрытие unit-тестами |

### Метаданные

| Аспект | Markdown | YAML V2 |
|--------|---------|---------|
| ID задачи | Нет (номер строки) | Уникальный int |
| Зависимости | Нет (неявный порядок) | depends_on: [id, ...] |
| Сложность | Нет | size: S/M/L |
| Файлы | Нет | files_hint: [...] |
| Feedback | `> USER FEEDBACK:` blockquote | feedback: "..." поле |

---

## 8. Открытые вопросы

### 8.1. Нужны ли depends_on в MVP?

Зависимости добавляют сложность: циклические зависимости, блокировки, переписывание ID при merge. Можно отложить на v1.1 и в v1.0 оставить пустой массив. Runner по-прежнему будет брать первую open задачу по порядку.

**Рекомендация**: включить depends_on в schema с первого дня (поле есть), но runner.Execute в v1.0 игнорирует его. Это позволяет bridge и пользователям заполнять зависимости без ломки runner'а. Активация в runner — отдельная story.

### 8.2. YAML vs JSON для хранения

Файл на диске — YAML (человекочитаемость, комментарии, git diff). LLM output — JSON (строгость парсинга). Конвертация через Go struct (единый тип для обоих форматов).

### 8.3. Как runner помечает задачу done?

Текущий подход: текстовая замена `- [ ]` → `- [x]` в файле. Новый подход: `MarkTaskDone(path, taskID)` — yaml.Unmarshal, изменить поле, yaml.Marshal, записать. Атомарность через запись во временный файл + rename.

### 8.4. Размер файла

10 эпиков x 10 задач x ~150 байт на задачу = ~15 KB YAML. Это меньше типичного sprint-tasks.md (>20 KB из-за markdown разметки). Проблем с размером нет.

---

## Источники

- `config/constants.go` — текущие regex паттерны
- `runner/scan.go` — текущий парсер ScanTasks
- `config/shared/sprint-tasks-format.md` — спецификация markdown формата
- `bridge/prompts/bridge.md` — промпт bridge с merge mode (244 строки)
- `bridge/bridge.go` — реализация bridge Run с merge detection
- `docs/research/variant-d-task-format.md` — предыдущий анализ форматов
