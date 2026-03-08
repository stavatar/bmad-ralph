# Bridge Concept Analysis -- Critical Review

## Executive Summary

Текущий `ralph bridge` -- это LLM-powered конвертер story файлов в sprint-tasks.md, промежуточный чеклист для runner'а. Критический анализ показывает, что bridge решает **реальную проблему** (декомпозиция требований на атомарные задачи для изолированных сессий), но делает это **наиболее хрупким из возможных способов**: через LLM с жёстким regex-парсингом на выходе. Результат -- "испорченный телефон" из трёх AI-слоёв, batching-проблема с потерей контекста, и фундаментальное противоречие между creative generation и deterministic parsing. Альтернативы варьируются от программного парсинга AC (низкий риск) до полного устранения промежуточного слоя (высокий потенциал), с гибридным подходом как оптимальным компромиссом.

---

## Текущая архитектура: что работает и что нет

### Как это работает сейчас

```
BMad AI (stories с AC) --> bridge AI (sprint-tasks.md) --> runner AI (код)
     ^                        ^                              ^
   Claude                   Claude                        Claude
```

1. BMad Workflow создаёт story файлы (`docs/sprint-artifacts/9-1-*.md`) с Acceptance Criteria
2. `ralph bridge` читает story файлы, собирает промпт с format specification, отправляет Claude
3. Claude генерирует sprint-tasks.md -- чеклист `- [ ]` задач с `source:` traceability
4. `ralph run` парсит sprint-tasks.md regex'ами (`TaskOpenRegex`, `TaskDoneRegex`), берёт первую `- [ ]` задачу
5. Runner отправляет Claude промпт "прочитай sprint-tasks.md, найди первую открытую задачу, реализуй"

### Strengths / Weaknesses

| Аспект | Strength | Weakness |
|--------|----------|----------|
| **Декомпозиция** | Группирует AC по "unit of work", фильтрует MANUAL/VERIFY AC | LLM-генерация = недетерминизм. Те же stories дают разные задачи при каждом запуске |
| **Format contract** | Жёсткая спецификация sprint-tasks-format.md | 134 строки инструкций для Claude чтобы он правильно расставил `- [ ]` и `source:` -- хрупко |
| **Merge mode** | Поддержка инкрементального обновления | LLM должен "сохранить ВСЕ существующие задачи" -- высокий риск потери/мутации |
| **Gate marking** | `[GATE]` для human checkpoints | Эвристика "first in epic" через анализ story numbering -- ненадёжна |
| **Batching** | splitBySize для больших проектов (>80KB) | Каждый batch не знает о задачах других batch'ей. 6 batch'ей = 6 изолированных Claude-сессий |
| **Traceability** | `source: file.md#AC-N` | Runner'у traceability БЕСПОЛЕЗНА -- он открывает sprint-tasks.md, берёт текст задачи, и работает |
| **AC Classification** | 4 типа (Implementation, Behavioral, Verification, Manual) | Это domain knowledge в промпте -- любая ошибка классификации = неправильная задача |
| **Task granularity** | Подробные правила (unit of work, complexity ceiling) | 100+ строк правил granularity -- это engineering в prompt, а не в code |

---

## Фундаментальные вопросы и ответы

### A. Нужен ли bridge вообще?

**Что bridge реально делает:**
1. Читает story markdown
2. Классифицирует AC (implementation vs behavioral vs verification vs manual)
3. Группирует related AC в задачи по "unit of work"
4. Форматирует в `- [ ]` чеклист с `source:` полями
5. Расставляет `[GATE]` маркеры

**Что из этого ТРЕБУЕТ LLM:**
- Шаг 2 (классификация AC) -- можно запрограммировать по ключевым словам ("already implemented", "verify", "manual", "browser testing")
- Шаг 3 (группировка) -- ЕДИНСТВЕННЫЙ шаг, требующий "понимания" семантической связи между AC
- Шаг 4 (форматирование) -- тривиальная строковая операция
- Шаг 5 (gates) -- можно определить программно (first story in epic, deploy keywords)

**Вердикт:** Bridge решает реальную проблему (AC --> tasks), но использует "ядерное оружие" (LLM) для задачи, которая на 80% программная. Только группировка AC требует семантического понимания, и даже она может быть заменена простыми эвристиками для большинства случаев.

**Критический момент:** Runner'у sprint-tasks.md нужен лишь как чеклист. Но runner.go уже отправляет Claude ВЕСЬ sprint-tasks.md и говорит "прочитай и найди первую `- [ ]` задачу". То есть Claude в runner'е **сам** читает задачу и **сам** решает что делать. Bridge просто создаёт промежуточный артефакт, который другой Claude интерпретирует.

### B. Нужны ли epics и stories от BMad?

**Цепочка документации:**
```
PRD (30+ страниц) --> Architecture (20+ страниц) --> Epics (stories list) --> Stories (AC + Dev Notes)
```

**Что из этого РЕАЛЬНО попадает в runner:**
- Runner читает sprint-tasks.md (созданный bridge из stories)
- Каждая задача = 1-2 предложения + `source:` ссылка
- Если source файл существует, runner'ский Claude читает его для контекста

**Парадокс:** Stories содержат 100+ строк детальной информации (AC, Dev Notes, Architecture, Implementation Details, Testing Standards, References), но bridge конвертирует их в 1-2 строчные задачи. Затем runner'ский Claude **снова** открывает story файл через source ссылку и читает его целиком. Это означает:

1. Bridge ТЕРЯЕТ информацию при конвертации (100 строк --> 2 строки)
2. Runner ВОССТАНАВЛИВАЕТ информацию, открывая source файл
3. Bridge по сути создаёт **оглавление** для stories, а не новую информацию

**Исследования подтверждают:** Структурированные requirements дают лучший результат чем plain text для LLM code generation (arxiv.org/pdf/2406.10101 -- "Requirements are All You Need"). BMad stories с AC -- хорошая практика. Но промежуточный step (bridge) не добавляет ценности, если runner сам может читать stories.

### C. Проблема "испорченного телефона"

Исследование Christopher Yee ("The Agentic Telephone Game: Cautionary Tale") напрямую описывает эту проблему:

> "Каждый проход через LLM вносит небольшие отклонения в точности или контексте, которые модель заполняет уверенным, формулированным рассуждением... потому что вывод хорошо читается, никто не ставит под сомнение его целостность."

В bmad-ralph три AI-слоя:
1. **BMad AI** интерпретирует человеческие требования --> stories с AC
2. **Bridge AI** интерпретирует AC --> task descriptions
3. **Runner AI** интерпретирует task descriptions --> код

Каждый слой:
- Теряет нюансы предыдущего
- Добавляет собственные интерпретации
- "Звучит правильно" на каждом шаге

**Конкретный пример из проекта:** Story 9.1 содержит детальные описания scaling algorithm (`position < 0.33 -> LOW, < 0.50 -> MEDIUM` и т.д.). Bridge конвертирует это в задачу "Implement ProgressiveParams function". Runner'ский Claude должен заново открыть story файл, найти algorithm details, и реализовать. Bridge не добавил информации -- он её потерял и заставил runner восстанавливать.

### D. Batching проблема

Текущая реализация (`splitBySize` с лимитом 80KB):
- 34 story файла --> 6 batch'ей
- Каждый batch = отдельная Claude-сессия
- Batch 3 не знает, какие задачи создал batch 1
- Merge mode предполагает, что Claude "сохранит ВСЕ существующие задачи" -- опасный антипаттерн

**Это фундаментальный лимит.** LLM не может работать с контекстом, который не видит. Merge mode -- попытка обойти это ограничение, но каждый последующий batch видит растущий sprint-tasks.md (все предыдущие задачи) + свои stories, что увеличивает давление на контекст.

**Альтернативное решение:** Если бы tasks генерировались ПРОГРАММНО (не LLM), batching был бы не нужен -- цикл по файлам, парсинг AC, генерация задач. O(n) операция без контекстных ограничений.

### E. Детерминизм vs Creativity

**Противоречие в bridge:**

| Что мы просим | Что мы требуем |
|---------------|----------------|
| "Будь креативным: группируй AC по unit of work" | "Формат MUST быть `- [ ]` с regex-парсингом" |
| "Классифицируй AC по 4 типам" | "Каждый `source:` MUST иметь `#AC-N` формат" |
| "Определи task granularity" | "5+ AC MUST produce 3+ tasks" (жёсткое правило) |
| "Расставь [GATE] marks" | "[GATE] MUST be at END of task line" |

Промпт bridge.md -- это 244 строки правил, ограничивающих "креативность" Claude до узкого коридора. По сути, мы описываем **алгоритм** на естественном языке и просим LLM его выполнить. Это как написать спецификацию merge sort в промпте вместо написания кода.

**ADaPT research (arxiv.org/abs/2311.05772)** показывает лучший подход: декомпозировать только когда executor не может справиться. Вместо upfront decomposition всех AC в задачи, runner мог бы пытаться реализовать story напрямую и декомпозировать только при неудаче.

---

## Как это делают другие

### Devin (Cognition AI)

- **Подход:** Planner-Worker. Получает требование, САМА сканирует codebase, предлагает plan (Interactive Planning в 2.0)
- **Task list:** НЕТ промежуточного артефакта. Plan живёт в сессии, человек может корректировать
- **Ключевое отличие:** Нет bridge-шага. Devin САМ декомпозирует, САМ выполняет, САМ итерирует

### SWE-Agent / OpenHands

- **Подход:** Agentic executor -- получает issue (GitHub Issue), напрямую работает с кодом
- **Task list:** НЕТ. Issue = единственный input. Agent сам решает что делать
- **Декомпозиция:** Implicit, в процессе выполнения

### AutoCodeRover

- **Подход:** Structured pipeline (fault localization --> context retrieval --> patch generation)
- **Task list:** НЕТ. Три фиксированных фазы, не configurable task list
- **Эффективность:** 6 шагов в среднем vs 29 у OpenHands -- structured pipeline побеждает free-form

### Aider

- **Подход:** Direct code mode (по умолчанию) или Architect mode (optional)
- **Task list:** НЕТ. Repository map + прямая генерация кода
- **Architect mode:** Двухмодельный подход (architect предлагает, editor применяет), но без промежуточного файла

### Cursor / Windsurf

- **Cursor Composer:** Описываешь задачу --> plan --> edits --> diff для approval. Нет persistent task list
- **Windsurf Cascade:** Persistent context в сессии, multi-step планирование. Нет external task file

### MetaGPT / MGX

- **Подход:** Multi-agent с SOPs (Standard Operating Procedures)
- **Task list:** ДА -- PRD --> System Design --> Tasks. Наиболее близок к bmad-ralph
- **Ключевое отличие:** SOPs = ПРОГРАММНЫЙ enforcement, не LLM-prompt enforcement. Agents общаются через structured outputs, не через markdown файлы

### Claude Code (best practices от Anthropic)

- **Подход:** CLAUDE.md + plan.md + direct execution
- **Task list:** plan.md создаётся В ТОЙ ЖЕ сессии, которая будет выполнять. Нет отдельного bridge
- **Антрopyc рекомендация:** "Никогда не позволяй Claude писать код, пока не одобришь план"

### Сводная таблица

| Tool | Intermediate task file? | Who decomposes? | Batching? |
|------|------------------------|-----------------|-----------|
| Devin | Нет (in-session plan) | Сам агент | Нет |
| SWE-Agent | Нет | Сам агент | Нет |
| AutoCodeRover | Нет (fixed pipeline) | Фиксированный алгоритм | Нет |
| Aider | Нет | Сам агент / Architect mode | Нет |
| Cursor | Нет | Сам агент | Нет |
| Windsurf | Нет | Сам агент | Нет |
| MetaGPT | Да (structured outputs) | Product Manager agent | Нет (in-memory) |
| Claude Code | plan.md (optional) | Сам агент в той же сессии | Нет |
| **bmad-ralph** | **Да (sprint-tasks.md)** | **Отдельный LLM-вызов (bridge)** | **Да (6 batch'ей)** |

**Вывод:** bmad-ralph -- ЕДИНСТВЕННЫЙ инструмент из рассмотренных, который использует отдельный LLM-вызов для создания persistent task file как промежуточного артефакта.

---

## Альтернативные архитектуры

### A: Bridge упрощается (программный парсинг AC --> tasks)

**Суть:** Убрать LLM из bridge, заменить программным парсингом.

**Алгоритм:**
1. Parse story markdown (regex для `## Acceptance Criteria` секции)
2. Извлечь AC по номерам (1, 2, 3...)
3. Классифицировать по keywords ("already implemented" --> skip, "manual" --> skip)
4. Каждый Implementation AC = одна задача (или группировка по файлу из Dev Notes)
5. Форматировать в sprint-tasks.md

| Плюсы | Минусы |
|-------|--------|
| Детерминизм | Грубая группировка (1 AC = 1 task без семантической группировки) |
| Нет batching проблемы | Не понимает "unit of work" семантически |
| Мгновенное выполнение (нет Claude call) | Не может обработать нестандартные story форматы |
| Нет потери информации | Нужна жёсткая структура story файлов |
| Тестируемый, предсказуемый | Потеря гибкости для edge cases |

**Сложность перехода:** Низкая. Новая Go-функция `ParseStoryToTasks()` заменяет `bridge.Run()`.
**Влияние на качество:** Нейтральное. Runner всё равно читает source файл для деталей.

### B: Bridge убирается, runner работает напрямую со stories

**Суть:** Runner получает список story файлов вместо sprint-tasks.md. Каждая story = одна Claude-сессия.

**Алгоритм:**
1. `ralph run` сканирует `docs/sprint-artifacts/` для story файлов
2. Для каждой story: отправляет Claude промпт "реализуй эту story"
3. Story файл содержит ВСЕ необходимое (AC, Dev Notes, Testing Standards)
4. Progress tracking: отметка `Status: done` в самом story файле

| Плюсы | Минусы |
|-------|--------|
| Нет потери информации (полный story context) | Story может быть слишком большой для одной сессии |
| Нет "испорченного телефона" (2 AI слоя вместо 3) | Нет checkpoint'ов внутри story (всё или ничего) |
| Нет batching проблемы | Сложная story может повиснуть |
| Story = единственный source of truth | Нужен механизм "частичного прогресса" |
| Runner'ский Claude видит ВСЮ информацию | Теряем granular tracking (какой AC выполнен) |

**Сложность перехода:** Средняя. Новый scan механизм, новый промпт, изменение progress tracking.
**Влияние на качество:** Потенциально выше -- Claude видит полный контекст с первого раза.

### C: Stories убираются, runner работает с epics напрямую

**Суть:** Epic файл содержит все stories. Runner обрабатывает epic целиком.

| Плюсы | Минусы |
|-------|--------|
| Минимум документации | Epic слишком большой для одного context window |
| Полный контекст связей между stories | Нет granular control |
| | Человек теряет story-level review points |

**Сложность перехода:** Высокая. Полный редизайн workflow.
**Влияние на качество:** Скорее всего отрицательное -- слишком большой scope для одной сессии.

### D: Всё убирается, runner работает с PRD напрямую (как Devin)

**Суть:** PRD --> Claude Code сессия. Claude сам декомпозирует, сам выполняет.

| Плюсы | Минусы |
|-------|--------|
| Максимально простой pipeline | PRD = 30+ страниц, не влезет в контекст с кодом |
| Подход Devin, работает для коммерческих продуктов | Нет human checkpoints |
| | Непредсказуемая декомпозиция |
| | Невозможно track progress |

**Сложность перехода:** Очень высокая. Полный отказ от BMad workflow.
**Влияние на качество:** Отрицательное для больших проектов, положительное для маленьких.

### E: Гибрид -- программный парсинг + LLM только для сложных решений (РЕКОМЕНДУЕМЫЙ)

**Суть:** Программный парсинг AC создаёт task skeleton. LLM вызывается ТОЛЬКО когда нужна семантическая группировка (ADaPT pattern -- "decompose as-needed").

**Алгоритм:**
1. Программно парсить story файл: извлечь AC, классифицировать по keywords
2. Простые случаи (1-3 AC, одна concern) --> программная генерация task
3. Сложные случаи (5+ AC, multiple concerns) --> LLM для группировки
4. Sprint-tasks.md генерируется программно из результата
5. Runner работает как раньше

**Или радикальнее (вариант E2):**
1. Отказ от sprint-tasks.md как артефакта
2. Runner напрямую парсит story файлы программно
3. Каждый AC (или группа) = одна Claude-сессия
4. Progress tracking через `.ralph-state.yaml` (внутренний файл)
5. LLM вообще НЕ участвует в planning -- только в execution

| Плюсы | Минусы |
|-------|--------|
| Детерминизм для 80% случаев | Сложнее чем чистый программный парсинг |
| LLM только когда нужно (ADaPT) | Нужен "complexity detector" для AC |
| Нет batching проблемы | Программный парсинг требует стабильной story структуры |
| Сохраняется гибкость для edge cases | |
| Быстрее и дешевле (меньше Claude calls) | |

**Сложность перехода:** Средняя.
**Влияние на качество:** Положительное -- детерминизм + полный контекст.

### Сводная таблица альтернатив

| Критерий | A (программный bridge) | B (runner со stories) | C (epics) | D (PRD/Devin) | E (гибрид) |
|----------|----------------------|---------------------|-----------|---------------|------------|
| Детерминизм | **5/5** | 3/5 | 2/5 | 1/5 | **4/5** |
| Полнота контекста | 3/5 | **5/5** | 4/5 | 4/5 | **4/5** |
| Сложность перехода | **Низкая** | Средняя | Высокая | Очень высокая | Средняя |
| Cost (Claude calls) | **$0** | = текущему | = текущему | < текущему | **~$0** |
| Human control | 4/5 | 3/5 | 2/5 | 1/5 | **4/5** |
| Scalability | **5/5** | 4/5 | 2/5 | 2/5 | **5/5** |

---

## Рекомендация

### Краткосрочно: вариант E2 (программный парсинг + direct story access)

1. **Убрать LLM из bridge** полностью. Заменить программным парсингом story файлов
2. **Генерировать tasks программно** из AC: каждый implementation AC = задача, behavioral AC объединяются с implementation AC по контексту (один файл / одна функция)
3. **Сохранить sprint-tasks.md** как tracking артефакт, но генерировать его ДЕТЕРМИНИСТИЧЕСКИ
4. **Runner включает source file content** в промпт напрямую (не только ссылку), устраняя "испорченный телефон"

### Долгосрочно: вариант B (runner напрямую со stories)

1. **Отказ от sprint-tasks.md** как промежуточного артефакта
2. **Runner работает со story файлами напрямую**, прогресс отслеживается через внутреннее состояние
3. **Каждая story = scope одной или нескольких Claude-сессий**, с checkpoint'ами между AC

### Что НЕ делать

- НЕ пытаться "починить" bridge промпт добавлением ещё правил -- 244 строки правил уже показывают, что задача не подходит для LLM
- НЕ переходить на Devin-style (PRD --> code) -- теряется human control, который в проекте критичен
- НЕ добавлять "валидацию" bridge output (lint task list) -- это лечит симптом, а не причину

### Обоснование

Из всех рассмотренных инструментов (Devin, SWE-Agent, OpenHands, AutoCodeRover, Aider, Cursor, Windsurf, MetaGPT) **ни один** не использует отдельный LLM-вызов для создания persistent task file. Это уникальный антипаттерн bmad-ralph. Ближайший аналог (MetaGPT) использует structured outputs между agents, но они in-memory, не через файл.

Исследование ADaPT показывает, что upfront decomposition **хуже** чем as-needed decomposition. Bridge делает именно upfront decomposition -- конвертирует ВСЕ AC в задачи ДО начала работы, вместо того чтобы позволить runner'у декомпозировать по мере необходимости.

Цена bridge: ~$0.10-0.50 за вызов * 6 batch'ей = $0.60-3.00 за sprint planning. Программный парсинг = $0.

---

## Источники

### Исследования и статьи
- [Requirements are All You Need: From Requirements to Code with LLMs](https://arxiv.org/pdf/2406.10101) -- structured requirements vs plain text для LLM
- [ADaPT: As-Needed Decomposition and Planning with Language Models](https://arxiv.org/abs/2311.05772) -- as-needed decomposition превосходит upfront planning
- [The Agentic Telephone Game: Cautionary Tale](https://www.christopheryee.org/blog/agentic-telephone-game-cautionary-tale/) -- потеря информации при цепочках LLM-обработки
- [Why Do Multi-Agent LLM Systems Fail?](https://arxiv.org/pdf/2503.13657) -- анализ ошибок в мультиагентных системах
- [Task Decomposition for Coding Agents: Architectures and Future Directions](https://mgx.dev/insights/task-decomposition-for-coding-agents-architectures-advancements-and-future-directions/a95f933f2c6541fc9e1fb352b429da15)
- [Long-Running AI Agents and Task Decomposition 2026](https://zylos.ai/research/2026-01-16-long-running-ai-agents)
- [AI Agentic Programming: A Survey](https://arxiv.org/html/2508.11126v1)

### Инструменты и документация
- [Devin 2.0 Interactive Planning](https://docs.devin.ai/work-with-devin/interactive-planning)
- [Devin 2.0 Technical Design](https://medium.com/@takafumi.endo/agent-native-development-a-deep-dive-into-devin-2-0s-technical-design-3451587d23c0)
- [Claude Code Best Practices (Anthropic)](https://www.anthropic.com/engineering/claude-code-best-practices)
- [Claude Code Common Workflows](https://code.claude.com/docs/en/common-workflows)
- [Aider Architect Mode](https://aider.chat/docs/usage/modes.html)
- [Cursor vs Windsurf vs Claude Code Comparison 2026](https://dev.to/pockit_tools/cursor-vs-windsurf-vs-claude-code-in-2026-the-honest-comparison-after-using-all-three-3gof)
- [MetaGPT / MGX Multi-Agent Framework](https://github.com/FoundationAgents/MetaGPT)
- [Claude Code Task Management](https://claudefa.st/blog/guide/development/task-management)
- [SWE-Agent Trajectories Analysis](https://software-lab.org/publications/ase2025_trajectories.pdf)
