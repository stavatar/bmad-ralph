# Архитектура `ralph plan` v2 — детальный проект

**Дата:** 2026-03-07
**Автор:** Архитектурный анализ на основе исследований variant-d-*
**Статус:** Проект (draft)

---

## 1. Обзор

Команда `ralph plan` заменяет связку "BMad stories + ralph bridge" одним шагом.
Принимает документы проекта (PRD, архитектуру, UX) и генерирует `sprint-tasks.md`
для выполнения командой `ralph run`.

```
БЫЛО:
  BMad AI (PRD -> Arch -> Epics -> Stories)
    -> ralph bridge (Stories -> sprint-tasks.md)
      -> ralph run (Tasks -> Code)

СТАЛО:
  ralph plan (PRD + Arch -> sprint-tasks.md)
    -> ralph run (Tasks -> Code)
```

---

## 2. Граф зависимостей пакетов

```
                         cmd/ralph/
                        /    |     \
                       /     |      \
                planner/  runner/  bridge/ (deprecated)
                  |    \    |   \     |
                  |     \   |    \    |
               session  config  gates session
                  |       |
                  |       |
               (stdlib) (yaml.v3)


Направление зависимостей (строго top-down):

  cmd/ralph
    -> planner   (NEW)
    -> runner    (без изменений)
    -> bridge    (deprecated, не удаляется)
    -> config    (leaf)

  planner
    -> session   (вызов Claude CLI)
    -> config    (Config, AssemblePrompt, constants)

  planner НЕ зависит от:
    -> runner    (параллельный пакет, не дочерний)
    -> bridge    (deprecated)
    -> gates     (planner не нуждается в gate prompt)
```

---

## 3. Структура пакета `planner/`

```
planner/
  planner.go           // Plan() — основная точка входа
  discover.go          // DiscoverDocs() — автодискавери документов проекта
  context.go           // CollectContext() — сбор codebase context для промпта
  format.go            // FormatTasks(), ParseLLMOutput() — JSON -> sprint-tasks.md
  merge.go             // MergeTasks() — детерминистический merge с existing tasks
  prompts/
    plan.md            // Go template — промпт планировщика (embed)
  planner_test.go
  discover_test.go
  format_test.go
  merge_test.go
```

---

## 4. Типы данных

### 4.1 PlanOptions — входные параметры команды

```go
// PlanOptions содержит параметры для Plan(), переданные из CLI.
// Pointer-поля: nil = флаг не указан (используется автодискавери).
type PlanOptions struct {
    PRDFiles  []string // --prd flag(s), или автодискавери
    ArchFiles []string // --arch flag(s), или автодискавери
    UXFiles   []string // --ux flag(s), или автодискавери

    // Backward compatibility: --from-stories использует bridge-like flow
    StoryFiles []string // --from-stories flag(s)

    // Output control
    OutputFile string // --output (default: sprint-tasks.md)
    DryRun     bool   // --dry-run: показать план без записи файла
    Merge      bool   // --merge: merge с существующим sprint-tasks.md (default: true)

    // LLM control (наследуется из config если не указано)
    MaxTurns *int    // --max-turns для plan session
    Model    *string // --model для plan session
}
```

### 4.2 DocType — тип обнаруженного документа

```go
// DocType классифицирует обнаруженный документ проекта.
type DocType int

const (
    DocPRD          DocType = iota // Product Requirements Document
    DocArchitecture                // Architecture / Technical Design
    DocUX                          // UX / UI spec
    DocEpics                       // Epic definitions
    DocUnknown                     // Не удалось классифицировать
)

// String возвращает человекочитаемое название типа.
func (d DocType) String() string {
    switch d {
    case DocPRD:
        return "PRD"
    case DocArchitecture:
        return "Architecture"
    case DocUX:
        return "UX"
    case DocEpics:
        return "Epics"
    default:
        return "Unknown"
    }
}
```

### 4.3 DiscoveredDoc — обнаруженный документ

```go
// DiscoveredDoc представляет файл, обнаруженный автодискавери.
type DiscoveredDoc struct {
    Path    string  // абсолютный путь к файлу
    Type    DocType // классификация документа
    Content string  // содержимое файла (загружается лениво)
    Score   float64 // уверенность классификации (0.0-1.0)
}
```

### 4.4 PlanContext — собранный контекст проекта

```go
// PlanContext содержит контекст проекта для инъекции в промпт.
// Собирается программно (Go), не через LLM.
type PlanContext struct {
    TechStack     string // содержимое go.mod / package.json
    DirTree       string // структура каталогов (max 3 уровня)
    ExistingTasks string // содержимое sprint-tasks.md (если есть)
    ClaudeMD      string // правила проекта из CLAUDE.md (если есть)
}
```

### 4.5 LLMTask — задача из LLM output (JSON)

```go
// LLMTask — структура задачи в JSON output от LLM.
// Go-код парсит этот JSON и форматирует в sprint-tasks.md.
type LLMTask struct {
    Title            string   `json:"title"`
    TestScenarios    []string `json:"test_scenarios"`
    RequirementRefs  []string `json:"requirement_refs"`
    Dependencies     []string `json:"dependencies"`
    NeedsApproval    bool     `json:"needs_human_approval"`
    IsSetup          bool     `json:"is_setup"`
    IsE2E            bool     `json:"is_e2e"`
    Complexity       string   `json:"complexity"` // "small", "medium", "large"
    FilesHint        []string `json:"files_hint"` // подсказка: какие файлы затронуты
}

// LLMEpic — группировка задач по эпикам в JSON output.
type LLMEpic struct {
    Name  string    `json:"name"`
    Tasks []LLMTask `json:"tasks"`
}

// LLMPlanOutput — корневая структура JSON output от LLM.
type LLMPlanOutput struct {
    Analysis string    `json:"analysis"`
    Epics    []LLMEpic `json:"epics"`
}
```

### 4.6 PlanResult — результат Plan()

```go
// PlanResult содержит результат планирования для возврата в CLI.
type PlanResult struct {
    TaskCount   int           // количество сгенерированных задач
    EpicCount   int           // количество эпиков
    OutputPath  string        // путь к записанному файлу
    PromptLines int           // количество строк промпта (для warning)
    Duration    time.Duration // длительность LLM-вызова
    Analysis    string        // краткий анализ от LLM
}
```

---

## 5. Интерфейсы и сигнатуры функций

### 5.1 planner.go — основная логика

```go
package planner

import (
    "context"
    _ "embed"

    "github.com/bmad-ralph/bmad-ralph/config"
)

//go:embed prompts/plan.md
var planPrompt string

// PlanPrompt возвращает embedded промпт планировщика.
func PlanPrompt() string { return planPrompt }

// Plan выполняет полный цикл планирования:
//   1. Автодискавери документов (если не указаны явно)
//   2. Сбор контекста проекта (Go, программно)
//   3. Сборка промпта и вызов Claude CLI (LLM)
//   4. Парсинг JSON output, валидация (Go, программно)
//   5. Форматирование в sprint-tasks.md (Go, программно)
//   6. Merge с existing tasks (Go, программно)
//   7. Запись файла
//
// Возвращает (PlanResult, error).
func Plan(ctx context.Context, cfg *config.Config, opts PlanOptions) (*PlanResult, error)
```

### 5.2 discover.go — автодискавери документов

```go
// DiscoverDocs ищет документы проекта в стандартных местах.
//
// Стратегия поиска (по приоритету):
//   1. ralph.yaml секция plan.docs (явная конфигурация)
//   2. Стандартные пути: docs/prd.md, docs/architecture.md, docs/ux.md
//   3. Glob-поиск: docs/**/*.md с классификацией по имени файла
//   4. Корневые файлы: PRD.md, ARCHITECTURE.md, REQUIREMENTS.md
//
// Возвращает отсортированный список документов по типу и уверенности.
func DiscoverDocs(root string) ([]DiscoveredDoc, error)

// ClassifyDoc определяет тип документа по имени файла и содержимому.
//
// Стратегия классификации (по приоритету):
//   1. По имени файла: prd*.md -> DocPRD, arch*.md -> DocArchitecture
//   2. По имени каталога: docs/prd/ -> DocPRD, docs/architecture/ -> DocArchitecture
//   3. По содержимому (первые 500 байт): ищет ключевые слова
//
// Возвращает (DocType, score). Score < 0.5 = DocUnknown.
func ClassifyDoc(path string, content []byte) (DocType, float64)
```

### 5.3 context.go — сбор контекста проекта

```go
// CollectContext собирает контекст проекта для инъекции в промпт.
// Все операции программные (чтение файлов, tree), LLM не используется.
//
// Включает:
//   - go.mod / package.json (tech stack)
//   - Структуру каталогов (max 3 уровня, игнорируя .git, node_modules, vendor)
//   - Existing sprint-tasks.md (для merge mode)
//   - CLAUDE.md (правила проекта)
func CollectContext(cfg *config.Config) (*PlanContext, error)

// BuildDirTree строит текстовое представление структуры каталогов.
// maxDepth ограничивает глубину (рекомендуется 3).
// ignoreDirs = [".git", "node_modules", "vendor", ".ralph", "__pycache__"]
func BuildDirTree(root string, maxDepth int) (string, error)
```

### 5.4 format.go — парсинг и форматирование

```go
// ParseLLMOutput парсит JSON output от LLM в структурированные данные.
// Возвращает ошибку если JSON невалиден или обязательные поля отсутствуют.
//
// Валидация:
//   - title: не пустой, <= 500 символов
//   - requirement_refs: хотя бы один элемент для non-setup/non-e2e задач
//   - epics: хотя бы один эпик с хотя бы одной задачей
func ParseLLMOutput(data []byte) (*LLMPlanOutput, error)

// FormatTasks форматирует структурированные задачи в sprint-tasks.md формат.
//
// Каждая задача форматируется как:
//   - [ ] Task description [GATE]
//     source: <prd-file>#<requirement-ref>
//
// Gate marking (программные правила):
//   - Первая задача каждого эпика получает [GATE]
//   - Задачи с NeedsApproval=true получают [GATE]
//   - [SETUP] задачи получают [GATE]
//
// Source traceability (программно):
//   - RequirementRefs -> source: <prd-filename>#<ref>
//   - IsSetup -> source: <prd-filename>#SETUP
//   - IsE2E -> source: <prd-filename>#E2E
func FormatTasks(plan *LLMPlanOutput, prdFilename string) string

// FormatTaskLine форматирует одну строку задачи.
// Вызывается из FormatTasks для каждой задачи.
func FormatTaskLine(task LLMTask, isFirstInEpic bool) string
```

### 5.5 merge.go — детерминистический merge

```go
// MergeTasks объединяет новые задачи с существующим sprint-tasks.md.
//
// Алгоритм:
//   1. Парсит existing sprint-tasks.md (regex, как runner.ScanTasks)
//   2. Для каждого эпика в новых задачах:
//      a. Если эпик уже существует — добавляет новые задачи в конец секции
//      b. Если эпик новый — добавляет секцию в конец файла
//   3. Дедупликация: задача считается дублем если title совпадает
//      с точностью до whitespace normalization
//   4. НИКОГДА не изменяет [x] -> [ ] (completed задачи неприкосновенны)
//   5. НИКОГДА не переставляет существующие задачи
//
// Возвращает объединённое содержимое sprint-tasks.md.
func MergeTasks(existing string, newContent string) (string, error)
```

---

## 6. CLI команда `cmd/ralph/plan.go`

### 6.1 Cobra Command

```go
package main

import (
    "fmt"

    "github.com/fatih/color"
    "github.com/spf13/cobra"

    "github.com/bmad-ralph/bmad-ralph/config"
    "github.com/bmad-ralph/bmad-ralph/planner"
)

var planCmd = &cobra.Command{
    Use:   "plan [requirement-files...]",
    Short: "Decompose requirements into sprint-tasks.md",
    Long: `Plan reads project documents (PRD, architecture, UX specs) and
generates a structured sprint-tasks.md for execution by the run command.

If no files are provided, plan auto-discovers documents in the project.

Examples:
  ralph plan docs/prd.md
  ralph plan docs/prd.md --arch docs/architecture.md
  ralph plan --dry-run
  ralph plan --from-stories docs/sprint-artifacts/*.md`,
    Args: cobra.ArbitraryArgs,
    RunE: runPlan,
}

func init() {
    rootCmd.AddCommand(planCmd)

    planCmd.Flags().StringSlice("prd", nil, "PRD file(s)")
    planCmd.Flags().StringSlice("arch", nil, "Architecture file(s)")
    planCmd.Flags().StringSlice("ux", nil, "UX spec file(s)")
    planCmd.Flags().StringSlice("from-stories", nil,
        "Story file(s) — backward compat with bridge workflow")
    planCmd.Flags().StringP("output", "o", "sprint-tasks.md",
        "Output file path")
    planCmd.Flags().Bool("dry-run", false,
        "Show generated plan without writing file")
    planCmd.Flags().Bool("no-merge", false,
        "Overwrite existing sprint-tasks.md instead of merging")
}

func runPlan(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load(config.CLIFlags{})
    if err != nil {
        return fmt.Errorf("ralph: load config: %w", err)
    }

    opts := planner.PlanOptions{
        // ... map CLI flags to PlanOptions ...
        Merge: true,
    }

    // Positional args -> PRD files
    if len(args) > 0 {
        opts.PRDFiles = args
    }

    // --no-merge inverts default
    if noMerge, _ := cmd.Flags().GetBool("no-merge"); noMerge {
        opts.Merge = false
    }

    result, err := planner.Plan(cmd.Context(), cfg, opts)
    if err != nil {
        return err
    }

    if opts.DryRun {
        color.Cyan("Dry run: %d tasks in %d epics (not written)",
            result.TaskCount, result.EpicCount)
    } else {
        fmt.Printf("Generated %d tasks in %d epics -> %s\n",
            result.TaskCount, result.EpicCount, result.OutputPath)
    }

    return nil
}
```

### 6.2 Таблица флагов

```
Флаг               Тип           Default                     Описание
--prd              []string      (автодискавери)             PRD файл(ы)
--arch             []string      (автодискавери)             Файлы архитектуры
--ux               []string      (автодискавери)             UX спецификации
--from-stories     []string      (нет)                       Story файлы (backward compat)
-o, --output       string        "sprint-tasks.md"           Путь к output файлу
--dry-run          bool          false                       Показать без записи
--no-merge         bool          false                       Перезаписать вместо merge
--model            string        (из config)                 Модель для plan сессии
--max-turns        int           (из config)                 Максимум turns для Claude
```

---

## 7. Автодискавери документов

### 7.1 Стратегия классификации

```
Приоритет 1: Имя файла (regex match)

  prd*.md, requirements*.md, *-prd.md, *-requirements.md
    -> DocPRD (score: 0.9)

  arch*.md, *-architecture.md, *-design.md, technical-design*.md
    -> DocArchitecture (score: 0.9)

  ux*.md, ui-spec*.md, *-wireframes.md, *-mockups.md
    -> DocUX (score: 0.9)

  epic*.md, *-epics.md
    -> DocEpics (score: 0.9)


Приоритет 2: Имя каталога

  docs/prd/*, docs/requirements/*
    -> DocPRD (score: 0.8)

  docs/architecture/*, docs/design/*
    -> DocArchitecture (score: 0.8)

  docs/ux/*, docs/ui/*
    -> DocUX (score: 0.8)

  docs/epics/*
    -> DocEpics (score: 0.8)


Приоритет 3: Содержимое (первые 1000 байт)

  Ключевые фразы:
    "functional requirement", "FR-", "user story", "as a user"
      -> DocPRD (score: 0.6)
    "architecture", "system design", "component diagram", "dependency"
      -> DocArchitecture (score: 0.6)
    "wireframe", "mockup", "user flow", "screen"
      -> DocUX (score: 0.6)

  Score < 0.5 -> DocUnknown (файл игнорируется)
```

### 7.2 Стандартные пути поиска

```
Порядок поиска (от конкретного к общему):

1. ralph.yaml конфигурация:
   plan:
     prd: ["docs/prd/feature-x.md"]
     arch: ["docs/architecture/system.md"]

2. Стандартные файлы (корень проекта):
   PRD.md, REQUIREMENTS.md
   ARCHITECTURE.md, DESIGN.md

3. Стандартные каталоги:
   docs/prd/*.md
   docs/architecture/*.md
   docs/ux/*.md
   docs/epics/*.md

4. Общий поиск:
   docs/**/*.md (с классификацией по имени + содержимому)
```

### 7.3 ralph.yaml секция `plan:`

```yaml
# .ralph/config.yaml
plan:
  prd:
    - docs/prd/feature-x.md
    - docs/prd/feature-y.md
  architecture:
    - docs/architecture/system.md
  ux: []
  output: sprint-tasks.md
  model: claude-sonnet-4-20250514  # модель для planning (дешевле чем opus)
  max_turns: 5
```

Структура в config.Config:

```go
// PlanConfig содержит настройки команды plan.
// Вложенная секция plan: в ralph.yaml.
type PlanConfig struct {
    PRDFiles  []string `yaml:"prd"`
    ArchFiles []string `yaml:"architecture"`
    UXFiles   []string `yaml:"ux"`
    Output    string   `yaml:"output"`
    Model     string   `yaml:"model"`
    MaxTurns  int      `yaml:"max_turns"`
}

// В Config:
type Config struct {
    // ... существующие поля ...
    Plan PlanConfig `yaml:"plan"`
}
```

---

## 8. Flow диаграмма `ralph plan`

```
                    ralph plan docs/prd.md --arch docs/arch.md
                                    |
                                    v
                    +-------------------------------+
                    |  cmd/ralph/plan.go: runPlan()  |
                    |  1. config.Load()              |
                    |  2. Map CLI flags -> PlanOpts  |
                    +-------------------------------+
                                    |
                                    v
                    +-------------------------------+
                    |  planner.Plan(ctx, cfg, opts)  |
                    +-------------------------------+
                                    |
              +---------------------+---------------------+
              |                                           |
              v                                           v
  +------------------------+               +------------------------+
  |  discover.go:          |               |  context.go:           |
  |  DiscoverDocs(root)    |               |  CollectContext(cfg)   |
  |  (если файлы не указ.) |               |  - go.mod             |
  |  - по имени файла      |               |  - dir tree           |
  |  - по каталогу          |               |  - existing tasks     |
  |  - по содержимому       |               |  - CLAUDE.md          |
  +------------------------+               +------------------------+
              |                                           |
              +---------------------+---------------------+
                                    |
                                    v
                    +-------------------------------+
                    |  Сборка промпта:              |
                    |  config.AssemblePrompt(       |
                    |    planPrompt,                |
                    |    TemplateData{...},         |
                    |    {__PRD__: ...,             |
                    |     __ARCH__: ...,            |
                    |     __CONTEXT__: ...}         |
                    |  )                            |
                    +-------------------------------+
                                    |
                                    v
                    +-------------------------------+
                    |  session.Execute(ctx, opts)   |
                    |  - OutputJSON: true           |
                    |  - SkipPermissions: true      |
                    |  LLM выдаёт JSON:            |
                    |  {"analysis":..,"epics":[..]} |
                    +-------------------------------+
                                    |
                                    v
                    +-------------------------------+
                    |  format.go:                   |
                    |  ParseLLMOutput(raw.Stdout)   |
                    |  - json.Unmarshal             |
                    |  - валидация полей            |
                    |  - retry если JSON невалиден  |
                    +-------------------------------+
                                    |
                                    v
                    +-------------------------------+
                    |  format.go:                   |
                    |  FormatTasks(plan, prdFile)   |
                    |  - gate marking (программно)  |
                    |  - source: traceability       |
                    |  - sprint-tasks.md формат     |
                    +-------------------------------+
                                    |
                              +-----+-----+
                              |           |
                              v           v
                    (merge=true)     (merge=false)
                        |                 |
                        v                 |
              +------------------+        |
              |  merge.go:       |        |
              |  MergeTasks(     |        |
              |    existing,     |        |
              |    newContent    |        |
              |  )               |        |
              +------------------+        |
                        |                 |
                        +--------+--------+
                                 |
                                 v
                    +-------------------------------+
                    |  os.WriteFile(output, content)|
                    |  Возврат PlanResult           |
                    +-------------------------------+
```

---

## 9. Разделение программного и LLM

### 9.1 Что делает Go-код (детерминистически)

```
+-------------------------------------------------------------------+
| Go-код (программный, детерминистический)                          |
+-------------------------------------------------------------------+
| 1. Автодискавери документов (regex по именам + keyword по тексту)  |
| 2. Сбор контекста: go.mod, dir tree, CLAUDE.md, existing tasks    |
| 3. Сборка промпта (config.AssemblePrompt — template + replace)    |
| 4. Парсинг JSON output (json.Unmarshal + validation)              |
| 5. Gate marking — первая задача эпика, [SETUP], NeedsApproval    |
| 6. Source traceability — requirement_refs -> source: field         |
| 7. Форматирование sprint-tasks.md (строковые операции)            |
| 8. Merge с existing tasks (diff по title, preserve [x])           |
| 9. Backup existing file перед записью                             |
+-------------------------------------------------------------------+
```

### 9.2 Что делает LLM (семантически)

```
+-------------------------------------------------------------------+
| LLM (семантический анализ, один вызов)                            |
+-------------------------------------------------------------------+
| 1. Анализ требований: понять ЧТО нужно реализовать               |
| 2. Группировка по "unit of work": связанные изменения = 1 задача  |
| 3. Определение зависимостей: B использует код из A -> A перед B   |
| 4. Эпик-группировка: задачи по доменным concern'ам               |
| 5. Оценка сложности: small/medium/large по количеству файлов     |
| 6. Классификация: skip уже реализованные, manual, verify-only     |
| 7. Описание тестовых сценариев: ключевые test cases для задачи    |
+-------------------------------------------------------------------+
```

### 9.3 JSON Schema для LLM output

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["analysis", "epics"],
  "properties": {
    "analysis": {
      "type": "string",
      "description": "1-3 sentences summarizing the requirements"
    },
    "epics": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["name", "tasks"],
        "properties": {
          "name": {
            "type": "string",
            "description": "Epic name (domain concern grouping)"
          },
          "tasks": {
            "type": "array",
            "minItems": 1,
            "items": {
              "type": "object",
              "required": ["title", "requirement_refs"],
              "properties": {
                "title": {
                  "type": "string",
                  "maxLength": 500,
                  "description": "Imperative sentence: what to implement"
                },
                "test_scenarios": {
                  "type": "array",
                  "items": { "type": "string" },
                  "description": "Key test cases"
                },
                "requirement_refs": {
                  "type": "array",
                  "items": { "type": "string" },
                  "description": "FR-1, FR-2 etc"
                },
                "dependencies": {
                  "type": "array",
                  "items": { "type": "string" },
                  "description": "Titles of prerequisite tasks"
                },
                "needs_human_approval": {
                  "type": "boolean",
                  "default": false
                },
                "is_setup": {
                  "type": "boolean",
                  "default": false
                },
                "is_e2e": {
                  "type": "boolean",
                  "default": false
                },
                "complexity": {
                  "type": "string",
                  "enum": ["small", "medium", "large"]
                },
                "files_hint": {
                  "type": "array",
                  "items": { "type": "string" },
                  "description": "Expected files to create/modify"
                }
              }
            }
          }
        }
      }
    }
  }
}
```

---

## 10. Формат вывода

### 10.1 Решение: sprint-tasks.md (markdown)

**Обоснование:** Runner (42 story, 92 FR) полностью построен на regex-парсинге
sprint-tasks.md. Замена формата = переписывание runner. Сохраняем markdown.

**Что меняется в sprint-tasks.md для plan:**

```diff
  ## Authentication & Security

- - [ ] Implement JWT token validation
-   source: stories/1-2-user-auth.md#AC-3
+ - [ ] Implement JWT token validation with tests for valid token,
+       expired token, and malformed token
+   source: prd.md#FR-3
```

Изменения:
1. `source:` ссылается на PRD (`prd.md#FR-3`), не на story (`stories/1-2.md#AC-3`)
2. Описание задачи включает test scenarios (inline, не в отдельных задачах)
3. Формат `- [ ]`, `- [x]`, `[GATE]`, `source:` — без изменений

### 10.2 Пример генерации

Входной PRD:
```markdown
# Feature: User Authentication

## FR-1: Login endpoint
POST /api/auth/login accepts email/password, returns JWT.

## FR-2: Token refresh
POST /api/auth/refresh accepts refresh token, returns new JWT pair.

## FR-3: Password reset
POST /api/auth/reset sends email with reset link.
```

LLM output (JSON):
```json
{
  "analysis": "3 authentication endpoints with JWT flow",
  "epics": [{
    "name": "Authentication",
    "tasks": [
      {
        "title": "Implement POST /api/auth/login with JWT generation, bcrypt password verification, and rate limiting",
        "test_scenarios": ["valid credentials -> 200 + tokens", "invalid password -> 401", "unknown email -> 404", "rate limit exceeded -> 429"],
        "requirement_refs": ["FR-1"],
        "dependencies": [],
        "needs_human_approval": false,
        "is_setup": false,
        "is_e2e": false,
        "complexity": "medium",
        "files_hint": ["auth/handler.go", "auth/handler_test.go"]
      },
      {
        "title": "Implement POST /api/auth/refresh with token rotation and old-token invalidation",
        "test_scenarios": ["valid refresh -> new pair", "expired refresh -> 401", "reused refresh -> 401 + invalidate family"],
        "requirement_refs": ["FR-2"],
        "dependencies": ["Implement POST /api/auth/login with JWT generation, bcrypt password verification, and rate limiting"],
        "needs_human_approval": false,
        "is_setup": false,
        "is_e2e": false,
        "complexity": "medium",
        "files_hint": ["auth/refresh.go", "auth/refresh_test.go"]
      },
      {
        "title": "Implement POST /api/auth/reset with email sending, secure token generation, and expiration",
        "test_scenarios": ["valid email -> 202 + email sent", "unknown email -> 202 (no leak)", "expired reset token -> 400"],
        "requirement_refs": ["FR-3"],
        "dependencies": [],
        "needs_human_approval": true,
        "is_setup": false,
        "is_e2e": false,
        "complexity": "medium",
        "files_hint": ["auth/reset.go", "auth/reset_test.go", "email/sender.go"]
      }
    ]
  }]
}
```

Go формирует sprint-tasks.md:
```markdown
## Authentication

- [ ] Implement POST /api/auth/login with JWT generation, bcrypt password verification, and rate limiting; tests: valid credentials -> 200 + tokens, invalid password -> 401, unknown email -> 404, rate limit exceeded -> 429 [GATE]
  source: prd.md#FR-1
- [ ] Implement POST /api/auth/refresh with token rotation and old-token invalidation; tests: valid refresh -> new pair, expired refresh -> 401, reused refresh -> 401 + invalidate family
  source: prd.md#FR-2
- [ ] Implement POST /api/auth/reset with email sending, secure token generation, and expiration; tests: valid email -> 202 + email sent, unknown email -> 202 (no leak), expired reset token -> 400 [GATE]
  source: prd.md#FR-3
```

---

## 11. Команда `ralph replan`

### 11.1 Решение: тот же пакет `planner/`

`replan` — это `Plan()` с merge mode + feedback injection. Отдельный пакет не нужен.

```
cmd/ralph/
  replan.go      // cobra command, вызывает planner.Replan()

planner/
  replan.go      // Replan() — обёртка над Plan() с дополнительным контекстом
```

### 11.2 Разница plan vs replan

```
+------------------+------------------------------------------+------------------------------------------+
| Аспект           | ralph plan                               | ralph replan                             |
+------------------+------------------------------------------+------------------------------------------+
| Input            | PRD + Arch (документы)                   | Existing sprint-tasks.md + feedback      |
| Промпт           | plan.md                                  | replan.md (расширенный plan.md)          |
| Контекст для LLM | PRD content + project context            | PRD + existing tasks + feedback + stats  |
| Output           | sprint-tasks.md (new или merge)          | sprint-tasks.md (modified)               |
| Gate trigger     | Нет (пользователь запускает вручную)     | [c] correct-course из gate prompt        |
| Merge            | Опциональный (--no-merge)                | Обязательный (всегда merge)              |
| Backup           | Да (.bak)                                | Да (.bak)                                |
+------------------+------------------------------------------+------------------------------------------+
```

### 11.3 Сигнатура Replan

```go
// ReplanOptions содержит параметры для Replan().
type ReplanOptions struct {
    Feedback    string   // фидбек от пользователя (из gate или CLI)
    PRDFiles    []string // оригинальные PRD (для context)
    OutputFile  string   // default: sprint-tasks.md
    KeepDone    bool     // сохранить [x] задачи (default: true)
    MaxTurns    *int
    Model       *string
}

// ReplanStats содержит статистику текущего прогресса для инъекции в промпт.
type ReplanStats struct {
    TotalTasks int
    DoneTasks  int
    OpenTasks  int
    SkippedTasks int
    Feedback   string // user feedback
}

// Replan корректирует существующий sprint-tasks.md на основе фидбека.
//
// Алгоритм:
//   1. Читает existing sprint-tasks.md
//   2. Собирает статистику прогресса (done/open/skipped)
//   3. Читает PRD для контекста (если указан)
//   4. Отправляет LLM: existing tasks + stats + feedback + PRD
//   5. LLM выдаёт JSON с modified task list
//   6. Go форматирует и merge'ит (preserve [x])
//
// Возвращает (PlanResult, error).
func Replan(ctx context.Context, cfg *config.Config, opts ReplanOptions) (*PlanResult, error)
```

### 11.4 Интеграция с Gate System

Новое действие `[c] correct-course` в gate prompt:

```
Текущий gate:
  [a]pprove  [r]etry with feedback  [s]kip  [q]uit

Расширенный gate:
  [a]pprove  [r]etry with feedback  [s]kip  [c]orrect-course  [q]uit
```

**Flow при нажатии [c]:**

```
1. Пользователь нажимает [c] на gate
2. Gate собирает feedback (как при [r])
3. Gate возвращает GateDecision{Action: ActionCorrectCourse, Feedback: "..."}
4. Runner приостанавливает execution
5. Runner вызывает planner.Replan() с feedback
6. planner.Replan() модифицирует sprint-tasks.md
7. Runner перечитывает sprint-tasks.md (ScanTasks)
8. Runner продолжает с обновлённым планом
```

Новая константа в `config/`:

```go
const ActionCorrectCourse = "correct-course"
```

Изменения в gates:
```go
// В gates.Prompt() добавляется case "c":
case "c":
    // Feedback collection (аналогично "r")
    feedback := collectFeedback(...)
    return &config.GateDecision{
        Action:   config.ActionCorrectCourse,
        Feedback: feedback,
    }, nil
```

Изменения в runner:
```go
// В runner.Execute() добавляется обработка ActionCorrectCourse:
case config.ActionCorrectCourse:
    replanResult, err := r.ReplanFn(ctx, cfg, planner.ReplanOptions{
        Feedback: decision.Feedback,
        PRDFiles: cfg.Plan.PRDFiles,
    })
    if err != nil {
        return fmt.Errorf("runner: correct-course: %w", err)
    }
    // Перечитать sprint-tasks.md
    scanResult, err = ScanTasks(readSprintTasks())
    // Продолжить выполнение
```

### 11.5 CLI команда replan

```go
var replanCmd = &cobra.Command{
    Use:   "replan",
    Short: "Correct course on existing sprint-tasks.md",
    Long: `Replan reads the current sprint-tasks.md, accepts feedback,
and adjusts the remaining tasks. Completed tasks are preserved.

Examples:
  ralph replan --feedback "Split the auth task into login + register"
  ralph replan --prd docs/prd.md --feedback "Add caching layer"`,
    RunE: runReplan,
}
```

---

## 12. Промпт plan.md — дизайн

### 12.1 Структура (архитектурные решения)

```
plan.md (Go template, ~80-100 строк)

Секции:
  1. Role (2 строки)
     "You are a software project planner..."

  2. Project Context (injected через __PROJECT_CONTEXT__)
     - tech stack, dir tree, CLAUDE.md
     - Собирается программно в context.go

  3. Requirements (injected через __REQUIREMENTS__)
     - PRD content
     - Architecture content (если есть)

  4. Decomposition Rules (~40 строк — ядро из bridge.md)
     - Task Granularity (unit of work)
     - Complexity Ceiling (4+ файлов -> split)
     - Minimum Decomposition (5+ FR -> 3+ задач)
     - Testing (inline test scenarios)
     - Dependency Ordering
     - Classification (skip already-implemented, manual)

  5. Output Schema (~15 строк)
     - JSON schema description
     - Правила заполнения полей

  6. {{if .HasExistingTasks}} Existing Tasks {{end}}
     - Список title'ов существующих задач (не полный файл)
     - "Do NOT re-generate these"
```

### 12.2 Ключевое отличие от bridge.md

```
bridge.md (244 строки)              plan.md (~80-100 строк)
----------------------------        ----------------------------
AC Classification (30 строк)  ->   Classification (5 строк "skip if...")
Format Contract (inline)       ->   JSON Schema (Go парсит)
Gate Marking (25 строк)        ->   Go-код (программно)
Source Traceability (25 строк) ->   Go-код (программно)
Merge Mode (15 строк)         ->   Go-код (программно)
Prohibited Formats (5 строк)  ->   Не нужно (JSON schema)
Few-shot (30 строк)           ->   Не нужно (JSON + schema desc)
Granularity Rule (40 строк)   ->   Granularity Rule (~25 строк, ядро)
Testing (5 строк)             ->   Testing (5 строк, сохранено)
Ordering (5 строк)            ->   Ordering (5 строк, сохранено)
```

**Итого:** 5 из 10 секций bridge.md перенесены в Go-код. Промпт сокращён на ~60%.

---

## 13. Интеграция с существующей инфраструктурой

### 13.1 Переиспользуемые компоненты

```
+-------------------------------+--------------------------------------+
| Компонент                     | Как используется в planner           |
+-------------------------------+--------------------------------------+
| session.Execute()             | LLM вызов (без изменений)            |
| session.ParseResult()         | Парсинг raw output                   |
| session.Options               | Prompt, MaxTurns, OutputJSON         |
| config.AssemblePrompt()       | Сборка промпта (template + replace)  |
| config.TemplateData           | Новое поле: HasExistingTasks (уже)   |
| config.Config                 | Новое поле: Plan PlanConfig          |
| config.TaskOpenRegex          | Подсчёт задач в output               |
| config.SprintTasksFormat()    | НЕ используется (JSON вместо markdown)|
| config.CLIFlags               | Расширение для plan-specific флагов  |
+-------------------------------+--------------------------------------+
```

### 13.2 Изменения в существующих пакетах

```
config/config.go:
  + PlanConfig struct
  + Plan PlanConfig field in Config
  + Validate: plan-specific validation

config/constants.go:
  + ActionCorrectCourse = "correct-course"

config/prompt.go:
  (без изменений — AssemblePrompt универсальный)

gates/gates.go:
  + case "c": correct-course action

runner/runner.go:
  + ReplanFn field on Runner struct (injectable, testable)
  + ActionCorrectCourse handling in gate decision switch

session/:
  (без изменений)

bridge/:
  + Deprecated comment in bridge.go
  (код не удаляется, bridge продолжает работать)
```

### 13.3 Backward Compatibility

```
ralph bridge <stories>   -- продолжает работать (deprecated warning)
ralph plan <prd>         -- новый способ
ralph plan --from-stories <stories>  -- вызывает bridge-like flow через planner

sprint-tasks.md формат   -- без изменений
runner.Execute()         -- без изменений
gates.Prompt()           -- расширен новым action [c]
```

---

## 14. Фазы реализации

### Фаза 1: Минимальный `ralph plan` (3-5 stories)

```
Story P.1: planner/ пакет scaffold
  - planner.go, format.go с типами
  - Пустой Plan() с session.Execute() вызовом
  - plan.md промпт (draft)

Story P.2: ParseLLMOutput + FormatTasks
  - JSON парсинг + валидация
  - Форматирование в sprint-tasks.md
  - Gate marking (программно)
  - Source traceability (программно)
  - Тесты с golden files

Story P.3: CollectContext + промпт assembly
  - context.go: go.mod, dir tree, existing tasks
  - Интеграция с config.AssemblePrompt
  - Полный Plan() flow

Story P.4: cmd/ralph/plan.go CLI
  - Cobra command с флагами
  - Positional args -> PRD files
  - --output, --dry-run, --no-merge

Story P.5: DiscoverDocs автодискавери
  - discover.go с классификацией
  - ralph.yaml секция plan:
  - Fallback на стандартные пути
```

### Фаза 2: Merge + Replan (2-3 stories)

```
Story P.6: MergeTasks
  - Детерминистический merge
  - Preserve [x], preserve order
  - Backup .bak

Story P.7: ralph replan + correct-course gate
  - replan.go
  - cmd/ralph/replan.go
  - ActionCorrectCourse в gates + runner
```

### Фаза 3: Полировка (1-2 stories)

```
Story P.8: --from-stories backward compat
  - Обёртка: stories -> bridge-like промпт через planner

Story P.9: ADaPT-стратегия для крупных PRD
  - Определение сложности PRD
  - Two-step для 10+ FR
  - Multi-step для 20+ FR
```

---

## 15. Риски и митигации

```
+--------------------------------------------+--------------------------------------------+
| Риск                                       | Митигация                                  |
+--------------------------------------------+--------------------------------------------+
| LLM выдаёт невалидный JSON                | Retry с error feedback (max 2 retries)     |
|                                            | + lenient parsing (trailing comma etc)     |
+--------------------------------------------+--------------------------------------------+
| Автодискавери находит неверные файлы       | Score threshold (< 0.5 = ignore)           |
|                                            | + --prd/--arch override всегда доступен    |
+--------------------------------------------+--------------------------------------------+
| Merge ломает existing tasks                | Backup .bak перед каждой записью           |
|                                            | + интеграционные тесты с golden files      |
+--------------------------------------------+--------------------------------------------+
| Промпт 80 строк недостаточен для           | Начать с few-shot примеров                 |
| качественной декомпозиции                  | + итерировать по результатам               |
+--------------------------------------------+--------------------------------------------+
| correct-course gate усложняет runner       | Injectable ReplanFn (testable)             |
|                                            | + отдельная story для интеграции           |
+--------------------------------------------+--------------------------------------------+
| Config struct растёт (PlanConfig)          | Nested struct (не плоские поля)            |
|                                            | + Plan section в yaml                      |
+--------------------------------------------+--------------------------------------------+
```

---

## 16. Итого: ключевые архитектурные решения

1. **Новый пакет `planner/`** параллелен `runner/` и `bridge/`. Зависит от `session` и `config`. Не зависит от `runner`.

2. **LLM выдаёт JSON**, Go форматирует sprint-tasks.md. Это перемещает 5 из 10 секций bridge.md из LLM в Go-код, сокращая промпт на ~60%.

3. **sprint-tasks.md сохраняется** как output формат. Runner работает без изменений. `source:` ссылается на PRD (`prd.md#FR-3`) вместо stories.

4. **Gate marking, source traceability, merge** — программные (Go), не LLM. Детерминистические операции не должны зависеть от вероятностного LLM.

5. **`ralph replan`** — тот же пакет planner/, с дополнительным контекстом (existing tasks + feedback). Интеграция с gate system через новый action `[c] correct-course`.

6. **Автодискавери** по 3-уровневой стратегии (имя файла -> имя каталога -> содержимое), с override через ralph.yaml и CLI флаги.

7. **Backward compatibility:** bridge продолжает работать. `--from-stories` в plan вызывает bridge-like flow.
