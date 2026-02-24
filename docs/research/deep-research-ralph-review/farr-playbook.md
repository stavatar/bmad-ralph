# Clayton Farr Ralph Playbook: Глубокий анализ реализации code review

> Источник: [ClaytonFarr/ralph-playbook](https://github.com/ClaytonFarr/ralph-playbook) |
> Руководство: [claytonfarr.github.io/ralph-playbook](https://claytonfarr.github.io/ralph-playbook/)
> Дата анализа: 2026-02-24

---

## 1. Общее описание

Ralph Playbook Клейтона Фарра -- один из наиболее полных гайдов по методологии Ralph Wiggum (автор оригинала -- Geoff Huntley). Репозиторий содержит 820+ звезд, 225 форков и представляет собой комплексную систему автономной AI-разработки на базе Claude.

### Структура репозитория

```
project-root/
├── loop.sh                  # Основной цикл оркестрации
├── loop_streamed.sh         # Версия с потоковым выводом
├── parse_stream.js          # Парсер потокового JSON-вывода Claude
├── PROMPT_build.md          # Инструкции режима строительства
├── PROMPT_plan.md           # Инструкции режима планирования
├── PROMPT_specs.md          # Инструкции генерации спецификаций
├── AGENTS.md                # Операционные правила (загружается каждую итерацию)
├── IMPLEMENTATION_PLAN.md   # Приоритизированный список задач (генерируется/обновляется Ralph)
├── specs/                   # Требования (один файл на JTBD-тему)
│   ├── [topic-a].md
│   └── [topic-b].md
├── src/                     # Исходный код приложения
└── src/lib/                 # Разделяемые утилиты и компоненты
    ├── llm-review.ts        # Ядро LLM-as-Judge фикстуры
    └── llm-review.test.ts   # Эталонные примеры LLM-рецензий
```

---

## 2. Ключевой принцип: Review через Backpressure, а не через отдельную фазу

### 2.1. Отсутствие классического Code Review

**Playbook не использует традиционный code review.** Вместо этого применяется парадигма **автономной самокоррекции через механизмы обратного давления (backpressure)**.

Центральная философия:

> "Backpressure beats direction -- instead of telling the agent what to do, engineer an environment where wrong outputs get rejected automatically."

Человек находится **вне цикла** (outside the loop), а не внутри:

> "Ralph should be doing _all_ of the work, including deciding which planned work to implement next and how to implement it. Your job is now to sit on the loop, not in it."

### 2.2. Review встроен в итерацию, а не является отдельной фазой

В PROMPT_build.md описан 8-шаговый процесс внутри каждой итерации:

1. **Orient** -- изучение specs/требований
2. **Read plan** -- изучение `IMPLEMENTATION_PLAN.md`
3. **Select** -- выбор наиболее важной задачи
4. **Investigate** -- проверка того, что уже реализовано
5. **Implement** -- реализация (N субагентов для файловых операций)
6. **Validate** -- 1 субагент для build/tests (**backpressure**)
7. **Update** -- пометить задачу как завершенную, зафиксировать открытия
8. **Commit** -- формализация изменений

**Шаг 6 (Validate) -- это и есть встроенный "review".** Он происходит внутри каждой итерации, не выделен в отдельную фазу.

---

## 3. Структура PROMPT_build.md и инструкции ревью

### 3.1. Полный текст PROMPT_build.md

```markdown
0a. Study `specs/*` with up to 500 parallel Sonnet subagents to learn the application specifications.
0b. Study @IMPLEMENTATION_PLAN.md.
0c. For reference, the application source code is in `src/*`.

1. Your task is to implement functionality per the specifications using parallel subagents.
   Follow @IMPLEMENTATION_PLAN.md and choose the most important item to address.
   Before making changes, search the codebase (don't assume not implemented) using Sonnet subagents.
   You may use up to 500 parallel Sonnet subagents for searches/reads and only 1 Sonnet
   subagent for build/tests. Use Opus subagents when complex reasoning is needed
   (debugging, architectural decisions).

2. After implementing functionality or resolving problems, run the tests for that unit
   of code that was improved. If functionality is missing then it's your job to add it
   as per the application specifications. Ultrathink.

3. When you discover issues, immediately update @IMPLEMENTATION_PLAN.md with your findings
   using a subagent. When resolved, update and remove the item.

4. When the tests pass, update @IMPLEMENTATION_PLAN.md, then `git add -A` then
   `git commit` with a message describing the changes. After the commit, `git push`.

99999. Important: When authoring documentation, capture the why --
       tests and implementation importance.
999999. Important: Single sources of truth, no migrations/adapters.
        If tests unrelated to your work fail, resolve them as part of the increment.
9999999. As soon as there are no build or test errors create a git tag.
         If there are no git tags start at 0.0.0 and increment patch by 1
         for example 0.0.1 if 0.0.0 does not exist.
99999999. You may add extra logging if required to debug issues.
999999999. Keep @IMPLEMENTATION_PLAN.md current with learnings using a subagent --
           future work depends on this to avoid duplicating efforts.
           Update especially after finishing your turn.
9999999999. When you learn something new about how to run the application,
            update @AGENTS.md using a subagent but keep it brief.
99999999999. For any bugs you notice, resolve them or document them in
             @IMPLEMENTATION_PLAN.md using a subagent even if it is
             unrelated to the current piece of work.
999999999999. Implement functionality completely. Placeholders and stubs
              waste efforts and time redoing the same work.
9999999999999. When @IMPLEMENTATION_PLAN.md becomes large periodically
               clean out the items that are completed from the file using a subagent.
99999999999999. If you find inconsistencies in the specs/* then use an Opus 4.6
                subagent with 'ultrathink' requested to update the specs.
999999999999999. IMPORTANT: Keep @AGENTS.md operational only --
                 status updates and progress notes belong in
                 IMPLEMENTATION_PLAN.md. A bloated AGENTS.md pollutes
                 every future loop's context.
```

### 3.2. Анализ инструкций, связанных с review

**Инструкция 2** -- ядро review-логики:
- "Run the tests for that unit of code that was improved"
- Тесты запускаются **после каждой реализации**
- Если функциональность отсутствует -- агент обязан её добавить
- Директива "Ultrathink" активирует глубокое рассуждение

**Инструкция 4** -- completion gate:
- Коммит происходит **только когда тесты проходят**
- Это единственное условие завершения задачи

**Инструкция 999999** -- кросс-ревью:
- "If tests unrelated to your work fail, resolve them as part of the increment"
- Агент обязан починить даже чужие сломанные тесты

---

## 4. Концепция "Completion Signal"

### 4.1. Определенный vs Неопределенный сигнал завершения

Playbook проводит четкое разграничение:

| Подход | Completion Signal | Проблема |
|--------|-------------------|----------|
| Стандартный | "Seems done?" | Субъективная оценка, агент может заявить о завершении без проверки |
| Ralph Playbook | "Required Tests Pass" | Объективный, программно верифицируемый сигнал |

### 4.2. Механика completion signal

```
Итерация N:
  1. Агент выбирает задачу из IMPLEMENTATION_PLAN.md
  2. Реализует функциональность
  3. Запускает тесты (backpressure)
     ├── Тесты ПРОВАЛЕНЫ --> цикл самокоррекции внутри итерации
     │                       (фиксит и перезапускает)
     └── Тесты ПРОШЛИ -->
          4. Обновляет IMPLEMENTATION_PLAN.md
          5. git add -A && git commit && git push
          6. Агент завершает работу (exit)

  loop.sh перезапускает итерацию N+1 с чистым контекстом
```

### 4.3. Роль тестов как completion signal

Тесты выступают в двойной роли:
1. **Валидация корректности** -- классическая функция
2. **Сигнал завершения** -- без прохождения тестов коммит невозможен

Это создает замкнутый цикл: агент не может "обмануть" систему, заявив о завершении без реальной работающей реализации.

---

## 5. Acceptance-Driven Backpressure: усиление через привязку тестов к критериям приемки

### 5.1. Суть усиления

Стандартный Ralph просто говорит "run tests". Enhancement **Acceptance-Driven Backpressure** связывает:

```
Specs (acceptance criteria) --> Planning (test requirements) --> Building (tests as guardrails)
```

### 5.2. Модификации промптов

**PROMPT_plan.md -- добавление:**
> "For each task in the plan, derive required tests from acceptance criteria in specs -- what specific outcomes need verification (behavior, performance, edge cases)."

**PROMPT_build.md -- модификация инструкции 1:**
> "Tasks include required tests -- implement tests as part of task scope."

**PROMPT_build.md -- модификация инструкции 2:**
> "Run all required tests specified in the task definition. All required tests must exist and pass before the task is considered complete."

**PROMPT_build.md -- новый guardrail:**
> "Required tests derived from acceptance criteria must exist and pass before committing. Tests are part of implementation scope, not optional."

### 5.3. Разграничение уровней

| Уровень | Что описывает | Пример |
|---------|---------------|--------|
| Acceptance Criteria (в specs) | Наблюдаемые результаты поведения | "Extracts 5-10 dominant colors from any uploaded image" |
| Test Requirements (в плане) | Точки верификации, выведенные из критериев | "Required tests: Extract 5-10 colors, process <5MB in <100ms" |
| Implementation Approach | Технические решения (на усмотрение Ralph) | Конкретный алгоритм, структуры данных |

### 5.4. Валидация тем спецификаций: правило "One Sentence Without 'And'"

```
OK:  "The color extraction system analyzes images to identify dominant colors"
BAD: "The user system handles authentication, profiles, and billing"
     --> три отдельных темы, нужны отдельные спецификации
```

---

## 6. LLM-as-Judge: паттерны для субъективных критериев

### 6.1. Проблема

Некоторые acceptance criteria не поддаются программной проверке:
- Творческое качество
- Эстетика и визуальная иерархия
- UX-ощущения
- Тон и стиль контента

### 6.2. Решение: Non-Deterministic Backpressure

Playbook предлагает фикстуру `llm-review.ts` в `src/lib/`:

```typescript
// src/lib/llm-review.ts

interface ReviewResult {
  pass: boolean;
  feedback?: string; // Only present when pass=false
}

function createReview(config: {
  criteria: string;    // What to evaluate
  artifact: string;    // Text content OR screenshot path
  intelligence?: "fast" | "smart";
}): Promise<ReviewResult>;
```

### 6.3. Уровни интеллекта

| Уровень | Модель | Применение |
|---------|--------|------------|
| `"fast"` (по умолчанию) | Быстрые модели (Gemini Flash и т.п.) | Прямолинейные оценки |
| `"smart"` | Мощные модели (GPT 5.1, Opus и т.п.) | Нюансированные эстетические/творческие суждения |

### 6.4. Мультимодальность

Автоматическое определение типа артефакта:
- **Текст** -- строка передается как текстовый ввод
- **Изображение** -- расширения .png, .jpg, .jpeg направляются как vision-ввод

### 6.5. Примеры тестов (`llm-review.test.ts`)

```typescript
// Текстовая оценка тона
test("welcome message tone", async () => {
  const message = generateWelcomeMessage();
  const result = await createReview({
    criteria: "warm, conversational tone appropriate for design professionals",
    artifact: message,
  });
  expect(result.pass).toBe(true);
});

// Визуальная оценка иерархии
test("dashboard visual hierarchy", async () => {
  await page.screenshot({ path: "./tmp/dashboard.png" });
  const result = await createReview({
    criteria: "clear visual hierarchy with obvious primary action",
    artifact: "./tmp/dashboard.png",
  });
  expect(result.pass).toBe(true);
});
```

### 6.6. Философия недетерминизма

LLM-рецензии **недетерминистичны** -- один и тот же артефакт может получить разные оценки при разных запусках. Это совпадает с философией Ralph:

> "Deterministically bad in an undeterministic world" -- цикл обеспечивает eventual consistency через итерации.

Ключевое: бинарный pass/fail. Если провал -- итерация, если успех -- commit. Цикл итерирует до прохождения.

### 6.7. Как Ralph обнаруживает паттерн

Ralph узнает паттерн LLM-рецензий из кода в `src/lib/` во время Phase 0c (`Study src/lib/*`). Файлы `llm-review.ts` и `llm-review.test.ts` служат **эталонными примерами**, которые агент обнаруживает при исследовании кодовой базы.

Это элегантное решение: паттерн встроен в код, а не в промпт, что экономит контекст.

---

## 7. Серия 999-правил (Guardrail Rules) и их связь с review

### 7.1. Принцип нумерации

Правила пронумерованы с использованием возрастающего количества девяток. **Чем больше число -- тем критичнее правило.** Это создает иерархию приоритетов:

```
99999          --> базовый уровень
999999         --> повышенный
9999999        --> высокий
...
999999999999999 --> максимальный приоритет
```

### 7.2. Полная таблица 999-правил

| Номер | Правило | Связь с review |
|-------|---------|----------------|
| 99999 | Документация: "capture the why" -- описывать важность тестов и реализации | Обязывает объяснять причины, а не просто описывать изменения |
| 999999 | Single sources of truth. Чинить все падающие тесты, даже не свои | **Кросс-ревью**: агент обязан обращать внимание на целостность всей системы |
| 9999999 | Создавать git tag при отсутствии ошибок (семантическое версионирование 0.0.x) | Метрика здоровья: наличие тега = все тесты проходят |
| 99999999 | Разрешено добавлять логирование для отладки | Инструментарий для self-review |
| 999999999 | Держать IMPLEMENTATION_PLAN.md актуальным | Память между итерациями: предотвращает дублирование |
| 9999999999 | Обновлять AGENTS.md при открытии нового операционного знания | Накопление операционных learnings |
| 99999999999 | Фиксировать и/или документировать баги, даже не связанные с текущей задачей | **Проактивный review**: агент ищет проблемы за пределами своей задачи |
| 999999999999 | Полная реализация: никаких заглушек и плейсхолдеров | Качественный гейт: предотвращает "ложные" completions |
| 9999999999999 | Периодически чистить завершенные пункты из плана | Поддержание чистоты контекста |
| 99999999999999 | Исправлять несоответствия в specs/* с помощью Opus + ultrathink | **Восходящий review**: агент может корректировать спецификации |
| 999999999999999 | AGENTS.md -- только операционные заметки, без прогресса | Защита контекста от загрязнения |

### 7.3. Review-значимые правила

Три правила напрямую связаны с функцией review:

1. **999999** -- чинить чужие тесты = непрерывная интеграционная проверка
2. **99999999999** -- фиксировать баги за пределами своей задачи = проактивный аудит
3. **99999999999999** -- корректировать спецификации = обратная связь от реализации к требованиям

---

## 8. PROMPT_plan.md: планирование как подготовка к review

### 8.1. Полный текст

```markdown
0a. Study `specs/*` with up to 250 parallel Sonnet subagents to learn
    the application specifications.
0b. Study @IMPLEMENTATION_PLAN.md (if present) to understand the plan so far.
0c. Study `src/lib/*` with up to 250 parallel Sonnet subagents to understand
    shared utilities & components.
0d. For reference, the application source code is in `src/*`.

1. Study @IMPLEMENTATION_PLAN.md (if present; it may be incorrect) and use up to
   500 Sonnet subagents to study existing source code in `src/*` and compare it
   against `specs/*`. Use an Opus subagent to analyze findings, prioritize tasks,
   and create/update @IMPLEMENTATION_PLAN.md as a bullet point list sorted in
   priority of items yet to be implemented. Ultrathink. Consider searching for
   TODO, minimal implementations, placeholders, skipped/flaky tests, and
   inconsistent patterns. Study @IMPLEMENTATION_PLAN.md to determine starting
   point for research and keep it up to date with items considered
   complete/incomplete using subagents.

IMPORTANT: Plan only. Do NOT implement anything. Do NOT assume functionality
is missing; confirm with code search first. Treat `src/lib` as the project's
standard library for shared utilities and components. Prefer consolidated,
idiomatic implementations there over ad-hoc copies.

ULTIMATE GOAL: We want to achieve [project-specific goal]. Consider missing
elements and plan accordingly. If an element is missing, search first to confirm
it doesn't exist, then if needed author the specification at specs/FILENAME.md.
If you create a new element then document the plan to implement it in
@IMPLEMENTATION_PLAN.md using a subagent.
```

### 8.2. Связь планирования с review

Планирование выполняет функцию **pre-review** (предварительной проверки):

1. **Gap-анализ**: сравнение кодовой базы с спецификациями
2. **Поиск проблем**: TODO, минимальные реализации, плейсхолдеры, flaky-тесты
3. **Верификация**: "Don't assume functionality is missing; confirm with code search first"

С enhancement Acceptance-Driven Backpressure, планирование также:
4. **Выводит test requirements** из acceptance criteria для каждой задачи
5. Это гарантирует, что на этапе building агент не сможет обойти проверки

---

## 9. AGENTS.md: операционный контекст для review

### 9.1. Шаблон

```markdown
## Build & Run

Succinct rules for how to BUILD the project:

## Validation

Run these after implementing to get immediate feedback:

- Tests: `[test command]`
- Typecheck: `[typecheck command]`
- Lint: `[lint command]`

## Operational Notes

Succinct learnings about how to RUN the project:

...

### Codebase Patterns

...
```

### 9.2. Роль в review-процессе

AGENTS.md -- это **мост между промптом и проектом**:
- PROMPT_build.md говорит "run tests" абстрактно
- AGENTS.md указывает **конкретные команды** для данного проекта
- Это делает backpressure project-specific

AGENTS.md обновляется агентом по мере обнаружения новых знаний (правило 9999999999), создавая **накопительную базу операционных знаний**.

---

## 10. Оркестрация: loop.sh и потоковый вывод

### 10.1. Основной цикл (loop.sh)

```bash
#!/bin/bash
# Usage: ./loop.sh [plan] [max_iterations]

# Parse arguments
if [ "$1" = "plan" ]; then
    MODE="plan"
    PROMPT_FILE="PROMPT_plan.md"
    MAX_ITERATIONS=${2:-0}
elif [[ "$1" =~ ^[0-9]+$ ]]; then
    MODE="build"
    PROMPT_FILE="PROMPT_build.md"
    MAX_ITERATIONS=$1
else
    MODE="build"
    PROMPT_FILE="PROMPT_build.md"
    MAX_ITERATIONS=0
fi

ITERATION=0
CURRENT_BRANCH=$(git branch --show-current)

# Verify prompt file exists
if [ ! -f "$PROMPT_FILE" ]; then
    echo "Error: $PROMPT_FILE not found"
    exit 1
fi

while true; do
    if [ $MAX_ITERATIONS -gt 0 ] && [ $ITERATION -ge $MAX_ITERATIONS ]; then
        echo "Reached max iterations: $MAX_ITERATIONS"
        break
    fi

    cat "$PROMPT_FILE" | claude -p \
        --dangerously-skip-permissions \
        --output-format=stream-json \
        --model opus \
        --verbose

    git push origin "$CURRENT_BRANCH" || {
        echo "Failed to push. Creating remote branch..."
        git push -u origin "$CURRENT_BRANCH"
    }

    ITERATION=$((ITERATION + 1))
done
```

### 10.2. Ключевые флаги Claude CLI

| Флаг | Назначение |
|------|------------|
| `-p` | Headless-режим (неинтерактивный) |
| `--dangerously-skip-permissions` | Авто-одобрение всех tool-вызовов |
| `--output-format=stream-json` | Структурированный вывод для логирования |
| `--model opus` | Использование Opus для сложных рассуждений |
| `--verbose` | Детальное логирование |

### 10.3. Потоковая версия (loop_streamed.sh)

Отличие от базовой версии -- добавление `--include-partial-messages` и передача вывода через `parse_stream.js`:

```bash
FULL_PROMPT="$(cat "$PROMPT_FILE")

Execute the instructions above."

claude -p "$FULL_PROMPT" \
    --dangerously-skip-permissions \
    --model opus \
    --verbose \
    --output-format stream-json \
    --include-partial-messages | node "$SCRIPT_DIR/parse_stream.js"
```

`parse_stream.js` парсит JSON-поток и выводит цветной форматированный вывод в терминал: cyan для инструментов, green для результатов, red для ошибок, yellow для системных сообщений.

---

## 11. Роль человека: стратегический review вне цикла

### 11.1. Позиция человека

```
+-------------------------------------------+
|              HUMAN OVERSIGHT               |
|  - Наблюдение за первыми итерациями        |
|  - Реактивная настройка guardrails         |
|  - Верификация плана перед building        |
|  - Ctrl+C как крайняя мера                 |
+-------------------------------------------+
         |                    ^
         v                    |
+-------------------------------------------+
|              RALPH LOOP                    |
|  Plan --> Build --> Test --> Commit --> ... |
|  (полностью автономный)                    |
+-------------------------------------------+
```

### 11.2. Когда человек вмешивается

1. **Перед building**: проверка плана -- "If wrong, re-run planning loop to regenerate plan"
2. **При наблюдении за паттернами ошибок**: добавление guardrails в AGENTS.md или промпт
3. **При регенерации плана**: если Ralph идет по кругу или дублирует работу
4. **Аварийная остановка**: Ctrl+C останавливает цикл

### 11.3. Человек не выполняет

- Task-by-task approval (пошаговое одобрение)
- Code review отдельных коммитов
- Ручную проверку тестов
- Утверждение архитектурных решений (доверено Ralph + Opus subagents)

---

## 12. Сравнительная таблица: подходы к review в Ralph Playbook

| Аспект | Традиционный CR | Ralph Playbook |
|--------|------------------|----------------|
| Кто проводит review | Человек-разработчик | Автоматические тесты + LLM-as-Judge |
| Когда происходит | После PR, перед merge | Внутри каждой итерации (шаг 6 из 8) |
| Блокирующий ли | Да, ждет одобрения | Да, коммит невозможен без прохождения тестов |
| Субъективные критерии | Комментарии ревьюера | LLM-as-Judge с бинарным pass/fail |
| Кросс-модульная проверка | По желанию ревьюера | Обязательно (правило 999999) |
| Восходящая обратная связь | Ревьюер -> автор | Агент -> спецификации (правило 99999999999999) |
| Completion signal | "LGTM" | Required Tests Pass |
| Накопление знаний | Код-ревью комментарии | IMPLEMENTATION_PLAN.md + AGENTS.md |

---

## 13. Архитектурные принципы review-системы

### 13.1. Backpressure beats Direction

Вместо того чтобы указывать агенту, что делать, среда инженерится так, чтобы неверные результаты отвергались автоматически:

```
Upstream Steering (детерминистичная настройка):
  ├── ~5000 токенов на спецификации
  ├── Одни и те же файлы контекста каждую итерацию
  └── Существующие паттерны кода формируют генерируемый код

Downstream Steering (backpressure):
  ├── Тесты отвергают невалидные реализации
  ├── Линтеры обеспечивают стилевую согласованность
  ├── Build failures заставляют фиксить до коммита
  └── LLM-as-Judge оценивает субъективные критерии
```

### 13.2. Context efficiency

- Каждая итерация получает чистое контекстное окно (~176K usable tokens из 200K)
- "Smart zone" -- 40-60% утилизации контекста
- Одна задача на итерацию = 100% использование "умной зоны"
- Learnings персистируются в файлах, а не в контексте

### 13.3. Eventual consistency через итерации

Недетерминизм LLM управляем:
- Тот же артефакт может получить разные LLM-оценки
- Цикл итерирует до прохождения
- "Eventual consistency achieved through iteration"

---

## 14. Выводы для проекта bmad-ralph

### 14.1. Что можно заимствовать

1. **Acceptance-Driven Backpressure** -- привязка тестов к критериям приемки уже на этапе планирования
2. **LLM-as-Judge фикстура** (`createReview`) -- для субъективных критериев качества
3. **999-серия guardrails** -- приоритизированная система инвариантов с возрастающей критичностью
4. **Разделение ролей**: планирование (PROMPT_plan) отдельно от строительства (PROMPT_build)
5. **Файл AGENTS.md** как накопительная база операционных знаний
6. **Completion signal** = тесты проходят, а не субъективная оценка "готово"

### 14.2. Ограничения подхода

1. **Нет human-in-the-loop review** -- полное доверие автоматизации может пропустить архитектурные проблемы
2. **LLM-as-Judge недетерминистичен** -- один и тот же код может то проходить, то не проходить review
3. **Нет PR-review** -- коммиты идут напрямую, без pull request gate
4. **Отсутствие ретроспективы** -- нет механизма для пост-итерационного анализа качества кода
5. **Зависимость от качества тестов** -- если тесты поверхностные, backpressure слабый

### 14.3. Возможные расширения

1. **Добавление review-фазы** между build и commit: отдельный LLM-рецензент с собственным промптом
2. **PR-gate**: коммиты в feature-ветку, merge через PR с автоматическим review
3. **Metrics collection**: сбор статистики прохождения тестов, LLM-оценок для анализа трендов
4. **Multi-agent review**: один агент реализует, другой рецензирует (с разными system prompts)

---

## Источники

- [ClaytonFarr/ralph-playbook -- GitHub](https://github.com/ClaytonFarr/ralph-playbook)
- [The Ralph Playbook -- GitHub Pages Guide](https://claytonfarr.github.io/ralph-playbook/)
- [PROMPT_build.md](https://github.com/ClaytonFarr/ralph-playbook/blob/main/files/PROMPT_build.md)
- [PROMPT_plan.md](https://github.com/ClaytonFarr/ralph-playbook/blob/main/files/PROMPT_plan.md)
- [AGENTS.md Template](https://github.com/ClaytonFarr/ralph-playbook/blob/main/files/AGENTS.md)
- [loop.sh](https://github.com/ClaytonFarr/ralph-playbook/blob/main/files/loop.sh)
- [loop_streamed.sh](https://github.com/ClaytonFarr/ralph-playbook/blob/main/files/loop_streamed.sh)
- [Geoff Huntley -- How to Ralph Wiggum](https://github.com/ghuntley/how-to-ralph-wiggum)
- [The Ralph Wiggum Playbook -- paddo.dev](https://paddo.dev/blog/ralph-wiggum-playbook/)
