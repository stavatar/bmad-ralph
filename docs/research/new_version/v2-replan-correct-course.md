# Проектирование интеграции correct-course в ralph

## 1. Анализ текущего состояния

### 1.1 Текущий gate flow

Ralph использует двухуровневую gate-систему:

**Обычный gate** (`GatePromptFn`) — срабатывает после clean review:
- На задачах с `[GATE]` тегом
- На checkpoint-ах (каждые N задач, `GatesCheckpoint`)
- Меню: `[a]pprove [r]etry [s]kip [q]uit`

**Emergency gate** (`EmergencyGatePromptFn`) — срабатывает при исчерпании:
- Execute attempts exhausted
- Review cycles exhausted
- Budget exceeded (total / per-task)
- Similarity loop detected
- Меню: `[r]etry [s]kip [q]uit` (approve недоступен)

### 1.2 Gate decision flow в runner

```
GateDecision{Action, Feedback}  (config/errors.go)

Action constants (config/constants.go):
  ActionApprove = "approve"
  ActionRetry   = "retry"
  ActionSkip    = "skip"
  ActionQuit    = "quit"
```

Runner обрабатывает решения так:
- **approve** — продолжить к следующей задаче
- **retry** — `InjectFeedback()` в sprint-tasks.md, `RevertTask()` (`[x]`→`[ ]`), сброс счётчиков
- **skip** — `SkipTask()` (помечает `[x]`), `wasSkipped=true`, обходит инкремент completedTasks
- **quit** — возвращает `GateDecision` как error (exit code 2)

### 1.3 BMad correct-course workflow

6-шаговый тяжеловесный процесс:
1. Инициализация — сбор контекста, загрузка документов
2. Чеклист из 20 пунктов — Impact Analysis по 6 секциям
3. Конкретные Change Proposals (old → new для каждого артефакта)
4. Sprint Change Proposal документ (5 секций)
5. Финализация и роутинг (Minor/Moderate/Major)
6. Handoff и completion

**Ключевые элементы:**
- Impact Analysis: PRD, Epics, Architecture, UI/UX
- Sprint Change Proposal документ
- Классификация scope: Minor / Moderate / Major
- Agent routing по scope

---

## 2. Анализ применимости BMad correct-course

### 2.1 Что применимо к ralph CLI

| BMad элемент | Применимость | Обоснование |
|---|---|---|
| Сбор описания проблемы | Да | Пользователь описывает что изменить |
| Классификация scope | Да, упрощённо | 3 уровня: тактический/структурный/стратегический |
| Diff-показ изменений | Да | Критично для UX — пользователь видит что изменится |
| Подтверждение/отклонение | Да | Rollback если отклонено |
| Impact Analysis (20 чеклист) | Нет | Overkill для CLI, ralph работает внутри одного спринта |
| Sprint Change Proposal документ | Нет | Избыточен, достаточно diff в терминале |
| Agent routing по scope | Нет | ralph — один агент, нет PM/PO/SM ролей |
| PRD/Architecture review | Нет | ralph оперирует только sprint-tasks.md |

### 2.2 Вывод

BMad correct-course спроектирован для межкомандной координации (PM, PO, SM, архитектор, разработчик). Ralph — одноагентный CLI. Из 6 шагов workflow нужна **суть** шагов 1 и 3:

- **Шаг 1**: собрать описание проблемы (одна строка ввода пользователя)
- **Шаг 3**: показать конкретные изменения (diff в терминале)

Остальное (чеклист, SCP документ, routing) — overhead без ценности для CLI.

---

## 3. Архитектура: три уровня коррекции

### 3.1 Уровни

```
Уровень 1: Тактический (retry)    — УЖЕ РЕАЛИЗОВАН
  Scope: одна текущая задача
  Механизм: [r] → InjectFeedback() → RevertTask() → повтор
  Примеры: "добавь обработку ошибок", "используй другой подход"

Уровень 2: Структурный (replan)   — НОВЫЙ [c]
  Scope: sprint-tasks.md целиком
  Механизм: [c] → ввод описания → Claude replan → diff → подтверждение
  Примеры: "убери задачи по basic auth, добавь OAuth",
           "поменяй порядок — сначала API, потом UI"

Уровень 3: Стратегический (reinit) — ОТЛОЖЕН (v3+)
  Scope: requirements + sprint-tasks.md
  Механизм: пересоздание story/epic → новый sprint plan
  Примеры: полная смена технологического стека
  Причина отложить: требует bridge integration (другой pipeline)
```

### 3.2 Обоснование только Уровня 2 для v2

- Уровень 1 уже работает
- Уровень 3 требует интеграции с bridge (create-story/validate-story), это другой масштаб
- Уровень 2 — сладкое место: sprint-tasks.md уже есть, формат понятен, diff показываем

---

## 4. Детальный дизайн: action [c] Correct Course

### 4.1 Новый gate action

```go
// config/constants.go
const ActionCorrectCourse = "correct-course"

// config/errors.go — GateDecision без изменений,
// Feedback содержит описание пользователя
```

### 4.2 Изменения в gates/gates.go

Добавить `[c]` в меню обычного gate (не emergency):

```
🚦 HUMAN GATE: - [ ] Implement user login [GATE]
   [a]pprove  [r]etry with feedback  [c]orrect course  [s]kip  [q]uit
>
```

При выборе `[c]` — собрать многострочный feedback (аналогично `[r]`):

```
Опишите изменения в плане (пустая строка = отправить):
> Убрать задачи по basic auth, добавить OAuth flow
> Добавить задачу на refresh tokens
>
```

Возвращает `GateDecision{Action: ActionCorrectCourse, Feedback: "..."}`.

### 4.3 Новый пакет: planner/

**Обоснование отдельного пакета**: replan — самостоятельная ответственность (SRP), не относится ни к runner (execution loop), ни к gates (user interaction). Зависимости: `config`, `session` — вписывается в dependency graph.

```
cmd/ralph → runner → planner → session, config
```

Альтернатива — функции внутри `runner/`. Аргумент за: replan тесно связан с execute loop, нужен доступ к `Runner.TasksFile`, `Runner.Cfg`. Аргумент против: runner уже ~1900 строк.

**Рекомендация**: начать как функции в `runner/replan.go`. Если вырастет > 300 строк — выделить пакет.

### 4.4 Replan flow (runner/replan.go)

```go
// ReplanFunc — injectable dependency для тестирования
type ReplanFunc func(ctx context.Context, opts ReplanOpts) (*ReplanResult, error)

type ReplanOpts struct {
    TasksFile   string   // путь к sprint-tasks.md
    Feedback    string   // описание пользователя
    ProjectRoot string   // для доступа к docs/
    ClaudeCmd   string   // config.ClaudeCommand для вызова Claude
}

type ReplanResult struct {
    OriginalContent string     // исходный sprint-tasks.md
    NewContent      string     // новый sprint-tasks.md (от Claude)
    Added           []string   // новые задачи
    Removed         []string   // убранные задачи
    Modified        []string   // изменённые задачи
    Preserved       []string   // сохранённые [x] задачи
}
```

#### Алгоритм Replan:

```
1. Прочитать текущий sprint-tasks.md
2. Извлечь выполненные задачи ([x]) — они НЕПРИКОСНОВЕННЫ
3. Сформировать промпт для Claude:
   - Текущий sprint-tasks.md
   - Описание изменений от пользователя
   - Инструкция: "Сохрани все [x] задачи. Перепланируй [ ] задачи."
4. Вызвать Claude (через session.RunClaude)
5. Распарсить ответ → новый sprint-tasks.md
6. Вычислить diff (Added/Removed/Modified)
7. Вернуть ReplanResult
```

### 4.5 Prompt для Claude (runner/prompts/replan.md)

```markdown
# Task: Replan Sprint

## Current sprint-tasks.md
{{.CurrentTasks}}

## User request
{{.Feedback}}

## Rules
1. All tasks marked `- [x]` MUST remain unchanged (exact text preserved)
2. Replan ONLY uncompleted tasks `- [ ]`
3. Maintain the same markdown format
4. Preserve [GATE] tags where appropriate
5. Output the complete new sprint-tasks.md

## Output
Return ONLY the new sprint-tasks.md content, no commentary.
```

### 4.6 Diff display (runner/replan.go)

```
📋 Plan changes:
  ✚ Added (3):
    + - [ ] Implement OAuth2 authorization flow
    + - [ ] Add refresh token rotation
    + - [ ] Add OAuth scopes configuration

  ✖ Removed (2):
    - - [ ] Implement basic auth middleware
    - - [ ] Add password hashing

  ≡ Modified (1):
    ~ - [ ] Add login endpoint → - [ ] Add OAuth login endpoint

  ✓ Preserved (5 completed tasks unchanged)

Apply changes? [a]pply  [e]dit description  [c]ancel
>
```

### 4.7 Confirmation gate (runner/replan.go)

Три варианта:
- **[a]pply** — записать новый sprint-tasks.md, вернуться в execute loop
- **[e]dit** — повторить ввод описания, re-run Claude
- **[c]ancel** — отменить, sprint-tasks.md не изменён, вернуться в gate prompt

### 4.8 Rollback гарантия

Перед вызовом Claude:
```go
backup := originalContent  // в памяти, не на диске
```

При cancel или ошибке Claude — sprint-tasks.md остаётся нетронутым (запись только при [a]pply).

Надёжнее варианта с файловым backup: нет risk orphaned backup files, нет race condition.

---

## 5. Интеграция с runner execute loop

### 5.1 Точка вставки в Runner.execute()

Correct course доступен **только** в обычном gate (не emergency). Emergency gate — аварийная ситуация, replan там неуместен.

```go
// runner/runner.go, в блоке обработки обычного gate decision
// (после clean review, строки ~1575-1610)

case config.ActionCorrectCourse:
    completedTasks-- // отменить инкремент — задача не завершена

    replanResult, err := r.ReplanFn(ctx, ReplanOpts{
        TasksFile:   r.TasksFile,
        Feedback:    decision.Feedback,
        ProjectRoot: r.Cfg.ProjectRoot,
        ClaudeCmd:   r.Cfg.ClaudeCommand,
    })
    if err != nil {
        return fmt.Errorf("runner: correct course: %w", err)
    }

    // Показать diff и спросить подтверждение
    confirmed, err := r.showReplanDiff(ctx, replanResult)
    if err != nil {
        return fmt.Errorf("runner: correct course confirm: %w", err)
    }

    if confirmed {
        if err := os.WriteFile(r.TasksFile, []byte(replanResult.NewContent), 0644); err != nil {
            return fmt.Errorf("runner: correct course write: %w", err)
        }
        r.logger().Info("correct course applied",
            "added", len(replanResult.Added),
            "removed", len(replanResult.Removed),
        )
    } else {
        r.logger().Info("correct course cancelled")
    }

    // Пересканировать задачи — continue outer loop
    continue
```

### 5.2 Пересканирование после replan

После записи нового sprint-tasks.md, execute loop делает `continue` на внешний цикл. На следующей итерации `ScanTasks()` перечитывает файл и находит первую `[ ]` задачу. Это уже работает — никаких изменений в scan логике не нужно.

### 5.3 Runner struct — новое поле

```go
type Runner struct {
    // ... существующие поля ...
    ReplanFn  ReplanFunc  // вызывается при [c]orrect course, nil = не поддерживается
}
```

В `Run()`:
```go
if cfg.GatesEnabled {
    r.ReplanFn = RealReplan  // production implementation
}
```

### 5.4 Взаимодействие с метриками

- Gate decision `correct-course` записывается через `recordGateDecision()`
- Стоимость Claude-вызова для replan учитывается в `CumulativeCost()`
- В `RunMetrics` добавить поле `Replans int` для подсчёта

---

## 6. Программный merge: алгоритм сохранения [x] задач

### 6.1 Проблема

Claude может случайно:
- Убрать выполненную задачу
- Изменить текст выполненной задачи
- Переупорядочить выполненные задачи

### 6.2 Решение: post-processing валидация

```go
func ValidateReplan(original, replanned string) error {
    origResult, _ := ScanTasks(original)
    newResult, _ := ScanTasks(replanned)

    // Все [x] из оригинала должны присутствовать в replanned
    origDone := make(map[string]bool)
    for _, t := range origResult.DoneTasks {
        origDone[t.Text] = true
    }

    for text := range origDone {
        found := false
        for _, t := range newResult.DoneTasks {
            if t.Text == text {
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("replan: completed task lost: %q", text)
        }
    }
    return nil
}
```

### 6.3 Fallback: принудительная инъекция

Если Claude потерял [x] задачи — восстановить программно:

```go
func ForcePreserveDoneTasks(original, replanned string) string {
    origResult, _ := ScanTasks(original)

    // Извлечь все [x] строки из оригинала
    var preserved []string
    for _, t := range origResult.DoneTasks {
        preserved = append(preserved, t.Text)
    }

    // Убрать из replanned любые [x] (Claude мог их испортить)
    // Вставить оригинальные [x] в начало файла (после заголовка)
    // Это грубо, но гарантирует сохранность
}
```

**Рекомендация**: использовать ValidateReplan() как guard. Если валидация провалилась — не показывать diff, а сообщить пользователю и предложить повторить с уточнённым описанием. ForcePreserveDoneTasks() — крайний fallback.

### 6.4 Diff вычисление

```go
func ComputeTaskDiff(original, replanned string) (added, removed, modified []string) {
    origOpen := extractOpenTasks(original)   // map[normalized_text]original_text
    newOpen := extractOpenTasks(replanned)

    for text := range newOpen {
        if _, exists := origOpen[text]; !exists {
            added = append(added, text)
        }
    }
    for text := range origOpen {
        if _, exists := newOpen[text]; !exists {
            removed = append(removed, text)
        }
    }
    // Modified — fuzzy match (Levenshtein distance < threshold)
    // между removed и added. Если пара найдена — это modification.
}
```

---

## 7. Sequence diagram

```
User          │  Gate Prompt  │  Runner      │  Claude       │  sprint-tasks.md
              │               │              │               │
 [задача done]│               │              │               │
              │  🚦 GATE      │              │               │
 ── [c] ─────>│               │              │               │
              │  "Опишите:" ──>              │               │
 ── feedback ─>               │              │               │
              │               │ read ────────────────────────> content
              │               │ prompt ──────> Claude        │
              │               │ <──────────── new content    │
              │               │ validate()   │               │
              │               │ diff()       │               │
              │  📋 Diff ────>│              │               │
 ── [a]pply ─>│               │              │               │
              │               │ write ───────────────────────> new content
              │               │ continue loop│               │
              │               │ ScanTasks() ─────────────────> re-read
              │               │ next [ ] task│               │
```

---

## 8. Boundary conditions и edge cases

### 8.1 Все задачи выполнены

Если все задачи `[x]` — replan добавляет новые `[ ]` задачи. Execute loop продолжает.

### 8.2 Claude возвращает невалидный markdown

ValidateReplan() отловит потерю [x] задач. Дополнительно — проверка что ScanTasks() парсит результат (есть хотя бы одна `[ ]` задача).

### 8.3 Пользователь отменяет несколько раз

Cancel возвращает в gate prompt. Пользователь может выбрать [a]pprove, [s]kip, [q]uit. Зацикливание невозможно — каждый cancel возвращает в основное меню.

### 8.4 Replan во время budget exceeded

Correct course недоступен в emergency gate. Если бюджет исчерпан — сначала разобраться с emergency, потом при следующем обычном gate можно делать replan.

### 8.5 Метрики и логирование

- Стоимость Claude-вызова для replan входит в общий бюджет
- В лог записывается: `correct-course applied|cancelled`, diff stats
- Gate decision `correct-course` учитывается в RunMetrics

---

## 9. Scope и оценка работ

### 9.1 Минимальный scope (v2.0)

| Компонент | Изменения | Оценка |
|---|---|---|
| `config/constants.go` | + `ActionCorrectCourse` | Тривиально |
| `config/constants_test.go` | + value test | Тривиально |
| `gates/gates.go` | + case `"c"` в меню | ~20 строк |
| `gates/gates_test.go` | + тесты для [c] | ~40 строк |
| `runner/replan.go` | ReplanFunc, ComputeTaskDiff, ValidateReplan, diff display | ~250 строк |
| `runner/replan_test.go` | Тесты | ~300 строк |
| `runner/prompts/replan.md` | Промпт для Claude | ~30 строк |
| `runner/prompt_test.go` | Тест промпта | ~30 строк |
| `runner/runner.go` | + case ActionCorrectCourse в gate handling, + ReplanFn поле | ~30 строк |
| `runner/runner_test.go` | + тесты для correct-course path | ~80 строк |
| `runner/metrics.go` | + Replans counter | ~5 строк |

**Итого**: ~800 строк кода + тестов. 3-4 story по текущему размеру.

### 9.2 Что не входит (v3+)

- Уровень 3 (стратегический) — пересоздание requirements
- Интеграция с bridge (create-story pipeline)
- History of replans (кроме лога)
- Undo последнего replan
- Интерактивное редактирование diff (построчное принятие/отклонение)
- Множественные replan промпты (iterative refinement в рамках одного correct-course)

---

## 10. Решения и trade-offs

### 10.1 Почему не отдельный пакет planner/

- Runner уже имеет все нужные зависимости (TasksFile, Cfg, session)
- ReplanFn как injectable dependency сохраняет тестируемость
- Если replan вырастет > 300 строк — выделить пакет, но не преждевременно

### 10.2 Почему не файловый backup

- In-memory backup надёжнее: нет orphaned files, нет race conditions
- Sprint-tasks.md типично < 10KB — в памяти тривиально
- Запись нового файла только при явном [a]pply

### 10.3 Почему correct-course только в обычном gate

- Emergency gate — аварийная ситуация (бюджет, зацикливание)
- Replan в аварии — усугубляет проблему (ещё один Claude-вызов при превышении бюджета)
- Пользователь может: quit → исправить sprint-tasks.md вручную → ralph run

### 10.4 Почему не полный BMad correct-course

- Ralph — CLI для одного разработчика, не межкомандная координация
- 20-пунктовый чеклист занимает 15-20 минут, пользователь хочет < 30 секунд
- Sprint Change Proposal документ некому читать в контексте solo-dev CLI
- Impact Analysis по PRD/Architecture — ralph не модифицирует эти файлы

### 10.5 Fuzzy match для Modified detection

Два подхода:
- **Простой**: Removed + Added с Levenshtein < 50% = Modified. Просто, понятно.
- **Без fuzzy**: Только Added/Removed, без Modified. Ещё проще, но менее информативно.

**Рекомендация**: начать без fuzzy (только Added/Removed). Modified усложняет diff display и тесты. Добавить в v2.1 если пользователи просят.

---

## 11. Dependency graph (обновлённый)

```
cmd/ralph
  ├── runner
  │     ├── session
  │     ├── gates       (GatePromptFunc)
  │     ├── config
  │     └── replan.go   ← НОВЫЙ (internal, не пакет)
  │           ├── session  (RunClaude для replan)
  │           └── config
  └── bridge
        ├── session
        └── config
```

Направление зависимостей не меняется. `replan.go` — файл внутри `runner/`, не нарушает архитектуру.

---

## 12. Рекомендуемый план эпика

### Story 1: ActionCorrectCourse constant + gate menu
- Добавить `ActionCorrectCourse` в config/constants.go
- Добавить `case "c"` в gates/gates.go (Prompt function)
- Обычный gate: показывать [c] в меню
- Emergency gate: НЕ показывать [c]
- Тесты: value test, gate integration test

### Story 2: ReplanFunc interface + prompt template
- Определить `ReplanFunc`, `ReplanOpts`, `ReplanResult` в runner/replan.go
- Создать runner/prompts/replan.md
- Тест промпта (template rendering)
- `ReplanFn` поле в Runner struct

### Story 3: ValidateReplan + ComputeTaskDiff
- Реализовать ValidateReplan() — проверка сохранности [x] задач
- Реализовать ComputeTaskDiff() — Added/Removed списки
- Тесты: happy path, потеря [x] задачи, пустой replan

### Story 4: RealReplan + diff display + confirmation
- Реализовать RealReplan() — вызов Claude, парсинг ответа
- Diff display в терминале
- Confirmation prompt ([a]pply/[e]dit/[c]ancel)
- Integration с runner execute loop (case ActionCorrectCourse)
- Метрики: Replans counter

---

## 13. Заключение

Интеграция correct-course в ralph — это добавление **одного нового gate action** `[c]` с чётким, ограниченным scope:

1. Пользователь описывает изменения (текстовый ввод)
2. Claude перепланирует sprint-tasks.md (с инвариантом: [x] задачи неприкосновенны)
3. Пользователь видит diff и подтверждает/отклоняет
4. Execute loop продолжает с новым планом

Из BMad correct-course взяты два принципа: **сбор описания проблемы** и **показ конкретных изменений**. Всё остальное (чеклист, SCP, routing) — отброшено как overkill для CLI.

Реализация: ~800 строк, 4 story, ~2 дня работы при текущей скорости.
