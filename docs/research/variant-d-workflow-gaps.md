# Вариант D: Workflow Gaps Analysis

Анализ того, что произойдёт с BMad workflows, если ralph станет самодостаточным CLI, не зависящим от BMad.

---

## BMad Workflows --> Ralph Mapping

### Полный список BMad workflows (Phase 4 Implementation)

| BMad Workflow | Что делает | Ralph эквивалент | Статус |
|---------------|-----------|-----------------|--------|
| **sprint-planning** | Извлекает все epics/stories из epic файлов, создаёт sprint-status.yaml | Нет эквивалента. Ralph читает sprint-tasks.md, а не sprint-status.yaml | НЕ РЕАЛИЗОВАН, НЕ ИСПОЛЬЗУЕТСЯ ralph напрямую |
| **create-story** | Создаёт story файл из epics с полным контекстом (AC, Dev Notes, Architecture, предыдущие stories, git intelligence) | Нет эквивалента. Ralph получает готовые story файлы | НЕ РЕАЛИЗОВАН, ИСПОЛЬЗУЕТСЯ (как input pipeline) |
| **dev-story** | Реализует story: читает задачи, TDD цикл, обновляет sprint-status.yaml | `ralph run` (execute mode) -- runner/prompts/execute.md | ЧАСТИЧНО РЕАЛИЗОВАН |
| **code-review** | Adversarial review с 5 sub-agents, findings, knowledge extraction, violation tracker | `ralph run` (review mode) -- runner/prompts/review.md | РЕАЛИЗОВАН (собственная реализация) |
| **correct-course** | Навигация изменений mid-sprint: impact analysis, artifact conflicts, Sprint Change Proposal | Нет эквивалента | НЕ РЕАЛИЗОВАН, НЕ ИСПОЛЬЗУЕТСЯ |
| **retrospective** | Ретро после завершения epic: Party Mode, agent dialogue, lessons learned, next epic prep | Нет эквивалента | НЕ РЕАЛИЗОВАН, НЕ ИСПОЛЬЗУЕТСЯ |
| **create-excalidraw-*** | Генерация диаграмм (dataflow, flowchart, wireframe) | Не нужен | НЕ НУЖЕН |
| **document-project** | Индексация brownfield-проекта | Не нужен (ralph не brownfield tool) | НЕ НУЖЕН |
| **workflow-init** | Инициализация BMad в проекте | `ralph init` мог бы заменить | НЕ НУЖЕН |
| **workflow-status** | Статус текущих workflows | `ralph status` мог бы заменить | НЕ НУЖЕН |
| **create-tech-spec** | Создание технической спецификации | Не нужен (pre-implementation) | НЕ НУЖЕН |
| **quick-dev** | Быстрая разработка без stories | Не нужен (ralph = loop orchestrator) | НЕ НУЖЕН |
| **implementation-readiness** | Проверка готовности к реализации | Можно встроить в `ralph plan` | ОПЦИОНАЛЬНО |
| **create-epics-and-stories** | Создание epics из PRD | Нет эквивалента, pre-implementation | НЕ НУЖЕН (pre-ralph) |

### Ключевые наблюдения

1. **Ralph УЖЕ реализует:** dev-story (execute mode) и code-review (review mode) -- два ЯДРА implementation pipeline
2. **Ralph НЕ реализует но зависит от:** create-story (создание story файлов --> bridge --> sprint-tasks.md)
3. **Ralph НЕ использует:** correct-course, retrospective, sprint-planning, create-excalidraw-*, document-project
4. **Ralph НЕ нужны:** 9 из 14 workflows -- они либо pre-implementation (PRD/epics/architecture), либо UI-specific (excalidraw), либо BMad-specific (workflow-init)

### Что ralph реально берёт от BMad в runtime

```
BMad создаёт ЗАРАНЕЕ (до ralph):
  PRD --> Architecture --> Epics --> Stories (create-story)

ralph получает ГОТОВЫЕ артефакты:
  story файлы --> bridge --> sprint-tasks.md --> runner (execute + review loop)
```

ralph зависит от BMad только через INPUT PIPELINE: готовые story файлы. Если ralph будет сам генерировать задачи из PRD/stories, зависимость от BMad исчезает полностью.

---

## Correct-Course Problem

### Что делает correct-course

Correct-course -- это ИНТЕРАКТИВНЫЙ workflow для обработки значительных изменений mid-sprint. Его структура:

1. **Инициализация:** пользователь описывает, что изменилось и почему
2. **Impact Analysis чеклист (6 секций, 20 check-items):**
   - Секция 1: Понимание триггера (что сломалось/изменилось)
   - Секция 2: Оценка влияния на epics (текущие и будущие)
   - Секция 3: Конфликты артефактов (PRD, Architecture, UI/UX)
   - Секция 4: Оценка путей решения (Direct Adjustment / Rollback / MVP Review)
   - Секция 5: Компоненты Sprint Change Proposal
   - Секция 6: Финальная проверка и handoff
3. **Sprint Change Proposal:** документ с Issue Summary, Impact Analysis, Recommended Approach, Detailed Changes, Implementation Handoff
4. **Routing:** Minor/Moderate/Major scope --> соответствующие agents

### Ключевые характеристики correct-course

- **Интерактивный:** требует участия пользователя на каждом шаге (ask/check/halt)
- **Document-centric:** читает PRD, epics, architecture, UI/UX для impact analysis
- **Multi-agent:** routing на PM/SM/Architect в зависимости от scope
- **Output:** Sprint Change Proposal markdown документ
- **Scope:** ЗНАЧИТЕЛЬНЫЕ изменения (стратегические, архитектурные), не мелкие правки

### Проблема: если ralph сам генерирует задачи из PRD

**Сценарий:** Пользователь меняет PRD mid-sprint. ralph уже сгенерировал sprint-tasks.md и частично выполнил задачи.

**Текущий BMad подход:**
1. Пользователь вызывает `/correct-course`
2. Workflow анализирует impact изменения на все артефакты
3. Генерирует Sprint Change Proposal
4. Handoff на re-planning (create-story заново, bridge заново)

**Проблема в контексте ralph:**
- ralph работает АВТОНОМНО (minimal human intervention)
- correct-course требует ИНТЕРАКТИВНОГО участия
- correct-course предполагает MULTI-AGENT team (PM, SM, Architect, Dev)
- ralph = single CLI tool, не team of agents

### Может ли gate system обрабатывать course corrections?

**Частично.** Gate system (gates.Prompt) уже предоставляет:
- Human approval/reject/feedback на каждой задаче
- Feedback injection в следующую Claude-сессию
- Skip/abort для отмены задач

**Но gate system НЕ покрывает:**
- Impact analysis на PRD/Architecture/Epics
- Пересоздание task list (sprint-tasks.md)
- Structural changes (добавить/удалить/переупорядочить задачи)
- Document-level analysis (конфликты между артефактами)

**Вывод:** Gates = тактическая коррекция (задача-уровень), correct-course = стратегическая коррекция (sprint-уровень).

### Нужна ли команда `ralph replan`?

**Да, но в упрощённой форме.** Correct-course BMad workflow избыточен для ralph:
- Интерактивный checklist из 20 пунктов -- overkill для CLI tool
- Multi-agent routing не применим (ralph = один процесс)
- Document-centric analysis (PRD/Architecture) -- пользователь сам решает что менять

**`ralph replan` должен быть:**
1. Перечитать source documents (PRD/stories/epics)
2. Регенерировать sprint-tasks.md (программно, не через LLM)
3. Сохранить прогресс уже выполненных задач (`- [x]` остаются)
4. Показать diff (что добавилось/изменилось/удалилось)
5. Спросить подтверждение

**Это ~200 строк Go кода**, а не 280-строчный LLM workflow.

---

## Что встроить в ralph

### 1. create-story --> `ralph plan` (ВЫСОКИЙ ПРИОРИТЕТ)

**Что берём из create-story:**
- Парсинг story файлов (AC, Dev Notes, Dependencies)
- Генерация sprint-tasks.md из AC (программный парсинг, не LLM)
- Traceability: `source: file.md#AC-N`
- Gate marking: `[GATE]` для first story in epic / deploy tasks

**Что НЕ берём:**
- Web research для latest tech (шаг 4 create-story) -- ralph = executor, не researcher
- Sprint-status.yaml management -- ralph использует sprint-tasks.md
- "Ultimate context engine" rhetoric -- избыточно
- Party Mode / agent personas -- ralph = CLI tool

**Реализация:** Программный парсер story файлов (альтернатива E2 из bridge-concept-analysis.md). Убираем LLM из planning, оставляем LLM только для execution.

### 2. code-review --> УЖЕ ВСТРОЕН

Ralph уже имеет собственную реализацию code review (runner/prompts/review.md):
- 5 sub-agents (quality, implementation, simplification, design-principles, test-coverage)
- Verification каждого finding
- Severity assignment (CRITICAL/HIGH/MEDIUM/LOW)
- Clean review handling (mark task done)
- Knowledge extraction (LEARNINGS.md)
- Incremental review (cycle-based)

**Отличие от BMad code-review:**
- BMad version: интерактивная (ask user fix/action-items/details)
- Ralph version: автономная (findings --> review-findings.md --> execute session fixes)
- BMad version: обновляет sprint-status.yaml, story файл, violation tracker
- Ralph version: обновляет sprint-tasks.md, review-findings.md, LEARNINGS.md

Ralph version ЛУЧШЕ для автономного pipeline, BMad version ЛУЧШЕ для интерактивной работы.

### 3. correct-course --> `ralph replan` (СРЕДНИЙ ПРИОРИТЕТ)

**Минимальный `ralph replan`:**
```
ralph replan [--source docs/stories/] [--keep-done]
```

Алгоритм:
1. Перечитать source documents
2. Регенерировать задачи из AC (программный парсинг)
3. Merge с текущим sprint-tasks.md: сохранить `- [x]` задачи
4. Показать diff пользователю
5. Записать обновлённый sprint-tasks.md

**Что НЕ берём из correct-course:**
- 20-пунктовый impact analysis checklist
- Sprint Change Proposal документ
- Multi-agent routing (PM/SM/Architect)
- Artifact conflict analysis (PRD vs Architecture vs UI/UX)
- Incremental/Batch mode selection

**Почему:** ralph пользователь САМ решает что менять в PRD/stories. ralph только ПЕРЕГЕНЕРИРУЕТ задачи. Impact analysis -- ответственность пользователя, не CLI tool.

### 4. retrospective --> `ralph retro` (НИЗКИЙ ПРИОРИТЕТ)

**Что можно встроить:**
- Автоматический анализ метрик из `.ralph-session.log` и `RunMetrics`
- Статистика: tasks completed, review findings count, gate rejections, cost
- Trend analysis: сравнение с предыдущими sprint'ами

**Что НЕ берём:**
- Party Mode (agent personas dialogue) -- игровой элемент BMad
- Psychological safety / No Blame rhetoric
- Agent manifest loading
- Interactive retrospective facilitation

**Реализация:** `ralph retro` = программный отчёт из метрик. Не Claude session, не interactive workflow.

```
ralph retro
  Tasks: 15 completed, 2 skipped
  Reviews: avg 4.3 findings/task, 0 CRITICAL, 12 MEDIUM, 8 LOW
  Cost: $12.50 (45 sessions, avg $0.28/session)
  Duration: 3h 22m wall clock
  Learnings: 8 new entries in LEARNINGS.md
```

---

## Что НЕ брать из BMad

### Категория 1: Pre-Implementation Workflows (человеческая ответственность)

| Workflow | Почему не нужен ralph |
|----------|-----------------------|
| create-epics-and-stories | Создание epics из PRD -- до ralph. Пользователь или отдельный инструмент |
| sprint-planning | Создание sprint-status.yaml -- ralph использует свой sprint-tasks.md |
| create-tech-spec | Техническая спецификация -- до ralph |
| implementation-readiness | Проверка готовности -- пользователь сам решает когда запускать ralph |

**Принцип:** ralph -- EXECUTOR, не PLANNER. Planning = ответственность пользователя (или BMad для тех, кто хочет).

### Категория 2: UI/Visualization Workflows

| Workflow | Почему не нужен ralph |
|----------|-----------------------|
| create-excalidraw-dataflow | Генерация диаграмм -- не имеет отношения к code execution |
| create-excalidraw-diagram | То же |
| create-excalidraw-flowchart | То же |
| create-excalidraw-wireframe | То же |

### Категория 3: BMad-specific Infrastructure

| Workflow | Почему не нужен ralph |
|----------|-----------------------|
| workflow-init | Инициализация BMad -- ralph имеет свой `ralph init` |
| workflow-status | Статус BMad workflows -- ralph имеет свой tracking |
| document-project | Brownfield indexing -- ralph не brownfield tool |
| quick-dev | Быстрая разработка -- ralph уже проще этого |

### Категория 4: Interactive/Multi-Agent Элементы (несовместимы с CLI)

| Элемент | Почему не берём |
|---------|----------------|
| Party Mode (agent personas) | ralph = CLI, не role-play |
| Multi-agent routing (PM/SM/Architect) | ralph = single process |
| Interactive checklist facilitation | ralph = autonomous execution |
| User skill level adaptation | ralph = developer tool, не обучающая платформа |
| Sprint Change Proposal document | Избыточная документация для CLI tool |

---

## Рекомендация по workflow архитектуре

### Целевая архитектура ralph (Вариант D)

```
Входные документы (создаются пользователем/BMad/другим инструментом):
  PRD, Architecture, Epics, Stories
           |
           v
  ralph plan (программный парсинг)
    - Читает story файлы
    - Извлекает AC программно
    - Генерирует sprint-tasks.md детерминистически
    - Расставляет [GATE] маркеры
           |
           v
  ralph run (execution loop) -- УЖЕ РЕАЛИЗОВАН
    - Execute mode: реализация задач
    - Review mode: adversarial code review
    - Gate system: human checkpoints
    - Knowledge management: LEARNINGS.md
    - Metrics: RunMetrics, session log
           |
           v
  ralph replan (при изменениях) -- НОВОЕ
    - Перечитывает source documents
    - Регенерирует sprint-tasks.md
    - Сохраняет прогресс (- [x])
    - Показывает diff
           |
           v
  ralph retro (после завершения) -- ОПЦИОНАЛЬНО
    - Метрики из session log
    - Статистика findings
    - Trend analysis
```

### Что ralph заменяет

| BMad Workflow | ralph Команда | Сложность |
|--------------|---------------|-----------|
| bridge (LLM) | `ralph plan` (программный) | Средняя (~400 LOC) |
| create-story (context) | НЕ ЗАМЕНЯЕТ -- stories создаются вне ralph | -- |
| dev-story | `ralph run` (execute) | УЖЕ РЕАЛИЗОВАН |
| code-review | `ralph run` (review) | УЖЕ РЕАЛИЗОВАН |
| correct-course | `ralph replan` | Низкая (~200 LOC) |
| retrospective | `ralph retro` | Низкая (~150 LOC) |

### Что ralph НЕ заменяет (и не должен)

- Создание PRD, Architecture, Epics -- pre-ralph planning
- Создание story файлов с полным контекстом (create-story) -- ОПЦИОНАЛЬНО может быть отдельной командой, но это не core функциональность ralph
- Визуализация (excalidraw) -- не имеет отношения
- Brownfield documentation -- не core

### Критическое решение: stories vs sprint-tasks.md

Два варианта для Варианта D:

**D1: ralph сохраняет sprint-tasks.md**
- `ralph plan` генерирует sprint-tasks.md из stories (программно)
- `ralph run` работает как сейчас
- Минимальные изменения в runner
- sprint-tasks.md = tracking артефакт

**D2: ralph работает со stories напрямую (без sprint-tasks.md)**
- `ralph run` парсит story файлы напрямую
- Прогресс отслеживается через внутренний state файл (.ralph-state.yaml)
- Устраняет "испорченный телефон" полностью
- Значительные изменения в runner (scan, prompt, state management)

**Рекомендация: D1 (сохранить sprint-tasks.md).** Причины:
1. Минимальные изменения в существующем коде
2. sprint-tasks.md = человекочитаемый tracking артефакт
3. Программная генерация устраняет проблемы bridge (детерминизм, скорость, стоимость)
4. Runner промпт (execute.md) уже оптимизирован для sprint-tasks.md формата

### Граница ответственности

```
ДО ralph (пользователь/BMad/другой инструмент):
  - Создание PRD
  - Создание Architecture
  - Создание Epics
  - Создание Stories (с AC, Dev Notes)

ralph (автономный CLI):
  - ralph plan: stories --> sprint-tasks.md (программно)
  - ralph run: sprint-tasks.md --> code (Claude loop)
  - ralph replan: обновление sprint-tasks.md при изменениях
  - ralph retro: отчёт по метрикам

ПОСЛЕ ralph (пользователь):
  - Merge/deploy
  - Impact assessment при значительных изменениях
  - Решение о course correction (вызвать ralph replan)
```

### Ответ на вопрос: "что произойдёт с correct-course?"

**correct-course НЕ НУЖЕН ralph в его полной BMad форме.** Его функциональность декомпозируется:

1. **Тактическая коррекция** (задача-уровень): уже покрыта gate system (approve/reject/feedback)
2. **Структурная коррекция** (sprint-уровень): покрывается `ralph replan` (перегенерация задач)
3. **Стратегическая коррекция** (project-уровень): остаётся ответственностью пользователя (изменить PRD/stories, затем `ralph replan`)
4. **Impact analysis** (artifact conflicts): НЕ НУЖЕН ralph -- это задача пользователя/BMad до вызова ralph

Correct-course предполагает team of AI agents, interactive facilitation, document-level analysis. ralph -- single autonomous CLI tool. Упрощение correct-course до `ralph replan` = правильная абстракция для CLI.
