# Канонический Ralph Loop: Исследование механизмов ревью кода

## Оглавление

1. [Введение и контекст](#введение-и-контекст)
2. [Два Ральфа: Huntley vs Carson](#два-ральфа-huntley-vs-carson)
3. [Архитектура канонического Ralph Loop (snarktank/ralph)](#архитектура-канонического-ralph-loop-snarktankralph)
4. [Как устроено ревью в каноническом Ralph](#как-устроено-ревью-в-каноническом-ralph)
5. [Философия "свежего контекста" и её связь с ревью](#философия-свежего-контекста-и-её-связь-с-ревью)
6. [Backpressure: оригинальная концепция Huntley](#backpressure-оригинальная-концепция-huntley)
7. [Сравнение: snarktank/ralph vs ghuntley/how-to-ralph-wiggum](#сравнение-snarktankralph-vs-ghuntleyhow-to-ralph-wiggum)
8. [Ревью как отдельный этап vs часть итерации](#ревью-как-отдельный-этап-vs-часть-итерации)
9. [Расширения: claude-review-loop и кросс-модельное ревью](#расширения-claude-review-loop-и-кросс-модельное-ревью)
10. [Выводы и рекомендации для bmad-ralph](#выводы-и-рекомендации-для-bmad-ralph)

---

## Введение и контекст

Ralph Wiggum Loop -- это паттерн автономной AI-разработки, названный в честь персонажа из "Симпсонов". Ключевая идея: запуск AI-агента в bash-цикле, где каждая итерация получает свежий контекст, а состояние сохраняется через файлы на диске и git-историю.

Данное исследование фокусируется на одном критическом вопросе: **как канонический Ralph реализует ревью кода** -- есть ли оно вообще, является ли оно частью итерации или отдельным этапом, и какова философия создателей на этот счёт.

**Важное уточнение**: существуют два "канонических" Ralph'а:

- **Geoffrey Huntley** (`ghuntley/how-to-ralph-wiggum`) -- первоначальный автор концепции, создатель термина "Ralph Wiggum Technique"
- **Ryan Carson** (`snarktank/ralph`) -- автор наиболее популярной реализации с 11.1k звёзд на GitHub

Оба подхода разделяют общую философию, но отличаются в деталях реализации.

---

## Два Ральфа: Huntley vs Carson

### Geoffrey Huntley -- оригинальный автор концепции

Huntley создал минимальный bash-цикл:

```bash
while :; do cat PROMPT.md | claude-code ; done
```

Его подход основан на трёх фазах:
1. **Phase 1 (Requirements)**: LLM-разговор для определения Jobs to Be Done (JTBD), создание спецификаций
2. **Phase 2 (Planning)**: Gap-анализ между спецификациями и кодом, генерация `IMPLEMENTATION_PLAN.md`
3. **Phase 3 (Building)**: Реализация задач из плана с валидацией и коммитами

Huntley использует **две сменяемые промпт-файла**:
- `PROMPT_plan.md` -- только анализ и планирование, без реализации
- `PROMPT_build.md` -- реализация задач из существующего плана

### Ryan Carson (snarktank) -- популярная реализация

Carson создал более структурированную систему с PRD-driven подходом:
- Использует `prd.json` вместо `IMPLEMENTATION_PLAN.md`
- Имеет навыки (skills) для генерации PRD (`/prd`) и конвертации в JSON (`/ralph`)
- Поддерживает два инструмента: Amp CLI и Claude Code

---

## Архитектура канонического Ralph Loop (snarktank/ralph)

### Структура репозитория

```
ralph/
├── .claude-plugin/          # Манифест для Claude Code marketplace
├── .github/workflows/       # GitHub Actions
├── flowchart/              # Интерактивная визуализация
├── skills/
│   ├── prd/               # Навык генерации PRD
│   └── ralph/             # Навык конвертации PRD -> JSON
├── AGENTS.md              # Документация для AI-агентов
├── CLAUDE.md              # Промпт-шаблон для Claude Code
├── prompt.md              # Промпт-шаблон для Amp
├── ralph.sh               # Главный скрипт цикла
└── prd.json.example       # Пример структуры задач
```

### Главный скрипт ralph.sh

Полный исходный код главного цикла:

```bash
#!/bin/bash
# Ralph Wiggum - Long-running AI agent loop
# Usage: ./ralph.sh [--tool amp|claude] [max_iterations]

set -e

# Parse arguments
TOOL="amp"  # Default to amp for backwards compatibility
MAX_ITERATIONS=10

while [[ $# -gt 0 ]]; do
  case $1 in
    --tool)
      TOOL="$2"
      shift 2
      ;;
    --tool=*)
      TOOL="${1#*=}"
      shift
      ;;
    *)
      if [[ "$1" =~ ^[0-9]+$ ]]; then
        MAX_ITERATIONS="$1"
      fi
      shift
      ;;
  esac
done

# Validate tool choice
if [[ "$TOOL" != "amp" && "$TOOL" != "claude" ]]; then
  echo "Error: Invalid tool '$TOOL'. Must be 'amp' or 'claude'."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRD_FILE="$SCRIPT_DIR/prd.json"
PROGRESS_FILE="$SCRIPT_DIR/progress.txt"
ARCHIVE_DIR="$SCRIPT_DIR/archive"
LAST_BRANCH_FILE="$SCRIPT_DIR/.last-branch"

# Archive previous run if branch changed
if [ -f "$PRD_FILE" ] && [ -f "$LAST_BRANCH_FILE" ]; then
  CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
  LAST_BRANCH=$(cat "$LAST_BRANCH_FILE" 2>/dev/null || echo "")

  if [ -n "$CURRENT_BRANCH" ] && [ -n "$LAST_BRANCH" ] && [ "$CURRENT_BRANCH" != "$LAST_BRANCH" ]; then
    DATE=$(date +%Y-%m-%d)
    FOLDER_NAME=$(echo "$LAST_BRANCH" | sed 's|^ralph/||')
    ARCHIVE_FOLDER="$ARCHIVE_DIR/$DATE-$FOLDER_NAME"

    echo "Archiving previous run: $LAST_BRANCH"
    mkdir -p "$ARCHIVE_FOLDER"
    [ -f "$PRD_FILE" ] && cp "$PRD_FILE" "$ARCHIVE_FOLDER/"
    [ -f "$PROGRESS_FILE" ] && cp "$PROGRESS_FILE" "$ARCHIVE_FOLDER/"
    echo "   Archived to: $ARCHIVE_FOLDER"

    echo "# Ralph Progress Log" > "$PROGRESS_FILE"
    echo "Started: $(date)" >> "$PROGRESS_FILE"
    echo "---" >> "$PROGRESS_FILE"
  fi
fi

# Track current branch
if [ -f "$PRD_FILE" ]; then
  CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
  if [ -n "$CURRENT_BRANCH" ]; then
    echo "$CURRENT_BRANCH" > "$LAST_BRANCH_FILE"
  fi
fi

# Initialize progress file if it doesn't exist
if [ ! -f "$PROGRESS_FILE" ]; then
  echo "# Ralph Progress Log" > "$PROGRESS_FILE"
  echo "Started: $(date)" >> "$PROGRESS_FILE"
  echo "---" >> "$PROGRESS_FILE"
fi

echo "Starting Ralph - Tool: $TOOL - Max iterations: $MAX_ITERATIONS"

for i in $(seq 1 $MAX_ITERATIONS); do
  echo ""
  echo "==============================================================="
  echo "  Ralph Iteration $i of $MAX_ITERATIONS ($TOOL)"
  echo "==============================================================="

  # Run the selected tool with the ralph prompt
  if [[ "$TOOL" == "amp" ]]; then
    OUTPUT=$(cat "$SCRIPT_DIR/prompt.md" | amp --dangerously-allow-all 2>&1 | tee /dev/stderr) || true
  else
    OUTPUT=$(claude --dangerously-skip-permissions --print < "$SCRIPT_DIR/CLAUDE.md" 2>&1 | tee /dev/stderr) || true
  fi

  # Check for completion signal
  if echo "$OUTPUT" | grep -q "<promise>COMPLETE</promise>"; then
    echo ""
    echo "Ralph completed all tasks!"
    echo "Completed at iteration $i of $MAX_ITERATIONS"
    exit 0
  fi

  echo "Iteration $i complete. Continuing..."
  sleep 2
done

echo ""
echo "Ralph reached max iterations ($MAX_ITERATIONS) without completing all tasks."
echo "Check $PROGRESS_FILE for status."
exit 1
```

### Ключевое наблюдение о ralph.sh

**В скрипте ralph.sh нет никакого механизма ревью кода.** Скрипт делает только три вещи:
1. Запускает AI-агент с промпт-файлом
2. Проверяет выход на наличие `<promise>COMPLETE</promise>`
3. Повторяет до завершения или исчерпания итераций

Вся логика quality gates делегирована **промпт-файлам** (CLAUDE.md / prompt.md), а не bash-скрипту.

---

## Как устроено ревью в каноническом Ralph

### Критический вывод: в каноническом Ralph НЕТ отдельного этапа code review

Канонический Ralph **не имеет выделенного этапа ревью кода**. Вместо этого используется концепция **backpressure** (обратного давления) -- автоматические проверки, встроенные в каждую итерацию.

### 10-шаговый процесс итерации (из CLAUDE.md)

```
1. Прочитать PRD в prd.json
2. Прочитать progress.txt (сначала секцию Codebase Patterns)
3. Проверить правильность ветки из PRD branchName
4. Выбрать историю с наивысшим приоритетом где passes: false
5. Реализовать эту единственную user story
6. Запустить проверки качества (typecheck, lint, test)
7. Обновить CLAUDE.md файлы если обнаружены полезные паттерны
8. Если проверки прошли, коммит: feat: [Story ID] - [Story Title]
9. Обновить PRD, установив passes: true
10. Добавить прогресс в progress.txt
```

### Quality Requirements из CLAUDE.md

Дословная цитата из промпт-файла:

> **Quality Requirements**
> - ALL commits must pass your project's quality checks (typecheck, lint, test)
> - Do NOT commit broken code
> - Keep changes focused and minimal
> - Follow existing code patterns

### Browser Testing (из prompt.md для Amp)

```markdown
### Browser Testing (Required for Frontend Stories)

For any story that changes UI, you MUST verify it works in the browser:

1. Load the `dev-browser` skill
2. Navigate to the relevant page
3. Verify the UI changes work as expected
4. Take a screenshot if helpful for the progress log

A frontend story is NOT complete until browser verification passes.
```

### Варианты из CLAUDE.md (для Claude Code)

```markdown
### Browser Testing (If Available)

For any story that changes UI, verify it works in the browser
if you have browser testing tools configured (e.g., via MCP):

1. Navigate to the relevant page
2. Verify the UI changes work as expected
3. Take a screenshot if helpful for the progress log

If no browser tools are available, note in your progress report
that manual browser verification is needed.
```

**Важное различие**: для Amp browser testing **обязателен** ("MUST"), для Claude Code -- **опционален** ("If Available").

### Механизм "Pass/Fail" на уровне истории

Структура `prd.json`:

```json
{
  "project": "MyApp",
  "branchName": "ralph/task-priority",
  "description": "Task Priority System",
  "userStories": [
    {
      "id": "US-001",
      "title": "Add priority field to database",
      "description": "As a developer, I need to store task priority...",
      "acceptanceCriteria": [
        "Add priority column to tasks table: 'high' | 'medium' | 'low'",
        "Generate and run migration successfully",
        "Typecheck passes"
      ],
      "priority": 1,
      "passes": false,
      "notes": ""
    }
  ]
}
```

Каждая история содержит:
- `acceptanceCriteria` -- объективно проверяемые критерии
- `passes` -- булево поле (false -> true при завершении)
- `notes` -- заметки для будущих итераций

**Агент сам решает, прошла ли история проверку**, и сам устанавливает `passes: true`. Нет внешнего валидатора или ревьюера.

---

## Философия "свежего контекста" и её связь с ревью

### Ключевой принцип: каждая итерация начинается с нуля

Из GitHub issue #125 в `anthropics/claude-plugins-official`:

> Оригинальная техника Ralph использует **внешний Bash-цикл**:
> ```bash
> while :; do cat PROMPT.md | npx --yes @sourcegraph/amp ; done
> ```
>
> Ключевые характеристики:
> - Каждая итерация запускается как **новый процесс** с **свежим контекстом**
> - "Самореферентность" Claude приходит исключительно от просмотра предыдущей работы **в файлах и git-истории**
> - Никакого накопления контекста разговора между итерациями

### Как свежий контекст связан с ревью

Свежий контекст выполняет **неявную функцию ревью**:

1. **Переоткрытие (Re-discovery)**: Каждая новая итерация заново "обнаруживает" работу предыдущей через файлы, что заставляет агента объективно оценивать текущее состояние кода вместо того, чтобы полагаться на память разговора
2. **Естественная верификация**: Когда агент читает `progress.txt` и `prd.json`, он получает сводку о проделанной работе и может заметить проблемы, которые предыдущая итерация пропустила
3. **Отсутствие предвзятости**: Свежий контекст означает отсутствие "привязанности" к предыдущим решениям -- агент может изменить подход, если видит, что предыдущий не работает

Как описал один из исследователей:

> Сила оригинальной техники в том, что Claude **переоткрывает** свою работу через файлы на каждой итерации, что вынуждает объективную оценку текущего состояния вместо опоры на память разговора.

### Четыре канала сохранения состояния

| Канал | Содержимое | Роль в "неявном ревью" |
|-------|-----------|----------------------|
| **Git History** | Закоммиченные изменения кода | Агент может увидеть diff и историю |
| **prd.json** | User stories со статусом `passes` | Показывает что завершено, а что нет |
| **progress.txt** | Append-only лог с learnings | Передаёт знания между итерациями |
| **AGENTS.md / CLAUDE.md** | Паттерны, специфичные для модуля | Автоматически загружается каждую итерацию |

---

## Backpressure: оригинальная концепция Huntley

Geoffrey Huntley ввёл термин **backpressure** (обратное давление) для описания механизма качества в Ralph.

### Три уровня backpressure

Из `ghuntley/how-to-ralph-wiggum`:

**1. Upstream steering (управление на входе):**
- Первые ~5,000 токенов выделяются на спецификации
- Каждая итерация загружает одни и те же файлы (PROMPT.md + AGENTS.md)
- Существующий код формирует то, что Ralph генерирует

**2. Downstream backpressure (давление на выходе):**
- Тесты, проверки типов, линтеры, сборка отклоняют невалидную работу
- `AGENTS.md` указывает конкретные команды валидации для проекта
- Промпт говорит "запусти тесты" обобщённо, AGENTS.md указывает конкретные команды

**3. LLM-as-judge (LLM как судья):**
- Для субъективных критериев (творческое качество, эстетика, UX)
- LLM-тесты с бинарным pass/fail
- "В конечном счёте сходится через итерацию"

### Метафора настройки гитары

Huntley описывает процесс как настройку инструмента:

> "Каждый раз, когда Ralph делает что-то плохое, Ralph настраивается -- как гитара."

Когда Ralph допускает ошибку, решение -- добавить уточняющие инструкции (signs), а не обвинять инструменты.

### Жизненный цикл итерации в режиме Building (Huntley)

```
1. Orient     -- Изучить specs/* (требования)
2. Read plan  -- Изучить IMPLEMENTATION_PLAN.md
3. Select     -- Выбрать самую важную задачу
4. Investigate -- Изучить релевантный src/* ("don't assume not implemented")
5. Implement  -- N субагентов для файловых операций
6. Validate   -- 1 субагент для build/tests (backpressure)
7. Update IMPLEMENTATION_PLAN.md -- Отметить задачу, записать находки/баги
8. Update AGENTS.md -- Если обнаружены операционные знания
9. Commit
10. Loop ends -- Контекст очищается; следующая итерация стартует заново
```

**Обратите внимание на шаг 6**: валидация ограничена **одним субагентом** для build/tests, тогда как для поиска и анализа можно использовать до 500 параллельных субагентов. Это намеренная инверсия: параллелизм для анализа, последовательность для проверки качества.

---

## Сравнение: snarktank/ralph vs ghuntley/how-to-ralph-wiggum

| Аспект | snarktank/ralph (Carson) | ghuntley/how-to-ralph-wiggum (Huntley) |
|--------|-------------------------|---------------------------------------|
| **Формат задач** | `prd.json` с user stories | `IMPLEMENTATION_PLAN.md` (markdown) |
| **Промпт-режимы** | Один файл (CLAUDE.md или prompt.md) | Два файла (PROMPT_plan.md, PROMPT_build.md) |
| **Фазы работы** | Единый цикл реализации | Три отдельные фазы (Requirements, Planning, Building) |
| **Quality gates** | typecheck, lint, test, browser | typecheck, lint, test + LLM-as-judge |
| **Ревью кода** | Нет (только self-check) | Нет (backpressure вместо ревью) |
| **Субагенты** | Не используются явно | До 500 Sonnet для анализа, 1 для build |
| **Модель** | amp или claude | Opus (основной) + Sonnet (субагенты) |
| **Завершение** | `<promise>COMPLETE</promise>` | Нет явного сигнала (план исчерпан) |
| **Инструменты** | Amp CLI, Claude Code | Claude Code с CLI-флагами |
| **Звёзды GitHub** | 11.1k | Меньше (обучающий репозиторий) |

### Ключевое различие в подходе к качеству

Carson полагается на **acceptance criteria в prd.json** -- каждая user story имеет объективно проверяемые критерии, которые агент должен выполнить.

Huntley полагается на **backpressure через инструменты** -- тесты, типы, линтеры, и потенциально LLM-as-judge для субъективных критериев.

**Ни один из них не имеет отдельного этапа code review в человеческом смысле.**

---

## Ревью как отдельный этап vs часть итерации

### Позиция канонического Ralph: ревью -- это НЕ отдельный этап

В каноническом Ralph **ревью не существует как отдельный этап**. Качество обеспечивается через:

1. **Автоматические проверки внутри итерации** (typecheck, lint, test)
2. **Свежий контекст** при каждой новой итерации (неявное "ревью" через переоткрытие)
3. **Acceptance criteria** в PRD, которые должны быть объективно верифицируемыми
4. **Backpressure** -- если тесты не проходят, коммит не происходит

### Философия Carson

Из промпт-файла `CLAUDE.md`:

```markdown
### Quality Requirements

- ALL commits must pass your project's quality checks (typecheck, lint, test)
- Do NOT commit broken code
- Keep changes focused and minimal
- Follow existing code patterns
```

Нет упоминания ревью кода. Качество определяется автоматическими проверками.

### Философия Huntley

Из `how-to-ralph-wiggum`:

> "Создание правильных сигналов и ворот для направления успешного вывода Ralph -- критически важно."
>
> "Backpressure может выходить за рамки валидации кода: некоторые критерии приёмки противятся программной проверке -- творческое качество, эстетика, ощущение от UX. LLM-as-judge тесты могут обеспечить backpressure для субъективных критериев с бинарным pass/fail."

Huntley идёт дальше Carson, предлагая **LLM-as-judge** как механизм для субъективных критериев, но это всё равно **часть итерации**, а не отдельный этап.

### Принцип "Let Ralph Ralph"

Из playbook Huntley:

> Доверяйте способности LLM к самоидентификации, самокоррекции и самоулучшению. Опирайтесь на конечную согласованность через итерацию. Используйте песочницы для автономии без риска безопасности.

Ключевая идея: **человек сидит НА цикле, а не В цикле**. Наблюдает за паттернами сбоев и реагирует корректировкой промптов, а не ревью отдельных изменений.

### Человеческое вмешательство: ai-pr-review

Интересно, что у snarktank есть **отдельный репозиторий** `snarktank/ai-pr-review` -- GitHub Actions workflow для AI-powered code review на уровне pull request. Это **не часть Ralph Loop**, а внешний инструмент, который может применяться к результатам работы Ralph.

Это подтверждает подход: Ralph работает автономно, а ревью (если оно нужно) -- это **отдельный внешний процесс** на уровне PR, а не часть цикла.

---

## Расширения: claude-review-loop и кросс-модельное ревью

### claude-review-loop (hamelsmu)

Hamel Husain создал `claude-review-loop` -- расширение Ralph, добавляющее **двухфазный цикл**:

1. **Task Phase**: Claude реализует задачу
2. **Review Phase**: При завершении, stop hook запускает Codex для независимого ревью, затем просит Claude адресовать обратную связь

Четыре параллельных Codex-субагента:

| Агент | Область |
|-------|---------|
| **Diff Review** | Качество кода, покрытие тестами, безопасность (OWASP top 10) |
| **Holistic Review** | Структура проекта, документация, архитектура |
| **Next.js Review** | Условный -- App Router, Server Components, кэширование |
| **UX Review** | Условный -- E2E-тестирование, доступность, адаптивность |

**Это НЕ часть канонического Ralph**, а стороннее расширение, показывающее один из путей эволюции паттерна.

### Кросс-модельное ревью (концепция)

Из исследований:

> Одна из реализаций расширяет технику кросс-модельным ревью: одна модель делает работу, другая модель проверяет её, и цикл продолжается пока задача не готова к отправке.

Это **эволюция** оригинальной идеи, но **не является частью канонического Ralph**.

---

## Выводы и рекомендации для bmad-ralph

### 1. Канонический Ralph НЕ содержит отдельного этапа code review

**Факт**: ни в `snarktank/ralph`, ни в `ghuntley/how-to-ralph-wiggum` нет механизма, где один агент ревьюирует работу другого или где выделяется отдельная фаза ревью кода.

### 2. Качество обеспечивается через backpressure, а не review

Канонический подход:
- Автоматические проверки (typecheck, lint, test) -- "жёсткие ворота"
- Свежий контекст при каждой итерации -- "неявное ревью"
- Acceptance criteria в PRD -- объективные критерии
- LLM-as-judge -- для субъективных критериев (Huntley)

### 3. Ревью существует только вне цикла

`snarktank/ai-pr-review` показывает, что Carson рассматривает ревью как **внешний процесс** на уровне Pull Request, а не как часть автономного цикла Ralph.

### 4. Свежий контекст -- это ключевая "ревью-функция"

Философская позиция: каждая новая итерация с чистым контекстом объективно переоценивает код. Это **замена** традиционному ревью, а не его отсутствие.

### 5. Расширения показывают спрос на ревью

Появление `claude-review-loop` и других расширений с кросс-модельным ревью показывает, что сообщество видит **пробел** в каноническом Ralph -- отсутствие формального ревью.

### 6. Рекомендация для bmad-ralph

Если bmad-ralph хочет добавить ревью в Ralph Loop, есть два пути:

**Путь A -- В рамках философии Ralph (backpressure)**:
- Добавить LLM-as-judge как дополнительный gate в промпт-файл
- Ревью как часть шага 6 (quality checks), а не отдельная фаза
- Сохранить принцип "одна итерация = один агент"

**Путь B -- Расширение за пределы Ralph (двухфазный цикл)**:
- По модели `claude-review-loop`: отдельная фаза ревью с другой моделью
- Нарушает каноническую философию, но добавляет реальную ценность
- Требует управления двумя контекстами

**Путь C -- Гибрид**:
- Backpressure (тесты, типы) как часть итерации
- Периодическое формальное ревью (например, каждые N итераций или при завершении epic)
- Ревью на уровне PR (как `snarktank/ai-pr-review`)

---

## Источники

### Первичные репозитории
- [snarktank/ralph](https://github.com/snarktank/ralph) -- Реализация Ryan Carson (11.1k звёзд)
- [ghuntley/how-to-ralph-wiggum](https://github.com/ghuntley/how-to-ralph-wiggum) -- Playbook Geoffrey Huntley
- [snarktank/ai-pr-review](https://github.com/snarktank/ai-pr-review) -- AI PR Review от snarktank

### Код и промпт-файлы
- [ralph.sh](https://github.com/snarktank/ralph/blob/main/ralph.sh) -- Главный скрипт цикла
- [CLAUDE.md](https://github.com/snarktank/ralph/blob/main/CLAUDE.md) -- Промпт для Claude Code
- [prompt.md](https://github.com/snarktank/ralph/blob/main/prompt.md) -- Промпт для Amp
- [prd.json.example](https://raw.githubusercontent.com/snarktank/ralph/main/prd.json.example) -- Пример PRD

### Блоги и статьи
- [ghuntley.com/ralph](https://ghuntley.com/ralph/) -- Оригинальная статья Huntley
- [A Brief History of Ralph](https://www.humanlayer.dev/blog/brief-history-of-ralph) -- История развития
- [The Ralph Wiggum Playbook](https://paddo.dev/blog/ralph-wiggum-playbook/) -- Анализ backpressure
- [Inventing the Ralph Wiggum Loop (подкаст)](https://devinterrupted.substack.com/p/inventing-the-ralph-wiggum-loop-creator) -- Интервью с создателем

### Расширения и плагины
- [Anthropic Ralph Wiggum Plugin](https://github.com/anthropics/claude-code/blob/main/plugins/ralph-wiggum/README.md) -- Официальный плагин
- [claude-review-loop](https://github.com/hamelsmu/claude-review-loop) -- Расширение с кросс-модельным ревью
- [Issue #125: Fresh context deviation](https://github.com/anthropics/claude-plugins-official/issues/125) -- Дискуссия о свежем контексте

### Прочие статьи
- [11 Tips For AI Coding With Ralph Wiggum](https://www.aihero.dev/tips-for-ai-coding-with-ralph-wiggum) -- Практические советы
- [Ralph Wiggum Loop Review (2026)](https://vibecoding.app/blog/ralph-wiggum-loop-review) -- Обзор паттерна
- [Fresh Context Pattern (Ralph Loop)](https://deepwiki.com/FlorianBruniaux/claude-code-ultimate-guide/9.5-fresh-context-pattern-(ralph-loop)) -- Анализ паттерна свежего контекста
