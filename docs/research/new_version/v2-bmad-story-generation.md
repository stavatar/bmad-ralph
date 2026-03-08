# Исследование: Генерация эпиков и stories в BMad Method

**Дата:** 2026-03-07
**Цель:** Детальный анализ pipeline генерации эпиков/stories в BMad v6 для оценки встраивания в ralph

---

## 1. Pipeline генерации: от PRD до готовой story

### Полная цепочка

```
PRD.md + Architecture.md + [UX.md]
        |
        v
  create-epics-and-stories (workflow, Phase 3)
  Агент: PM (John) или SM (Bob)
  Вход: PRD + Architecture + UX (опционально)
  Выход: docs/epics.md (или sharded epic-N-*.md)
        |
        v
  sprint-planning (workflow, Phase 4)
  Агент: SM (Bob)
  Вход: epic-файлы
  Выход: sprint-status.yaml (tracking всех epics/stories)
        |
        v
  create-story (workflow, Phase 4, повторяется для каждой story)
  Агент: SM (Bob)
  Вход: epics.md + sprint-status.yaml + Architecture + PRD + предыдущая story + git history
  Выход: docs/sprint-artifacts/{epic}-{story}-{title}.md
        |
        v
  validate-create-story (опционально, Phase 4)
  Агент: SM (Bob) через validate-workflow.xml
  Вход: созданная story + checklist.md + исходные документы
  Выход: улучшенная story + validation report
        |
        v
  dev-story (Phase 4)
  Агент: Dev (Amelia)
  Вход: story файл
  Выход: реализованный код
        |
        v
  code-review (Phase 4)
  Агент: Dev (Amelia) или отдельный reviewer
  Вход: story + код + git diff
  Выход: review findings, фиксы
```

### Предпосылки (Prerequisites)

Перед генерацией эпиков ОБЯЗАТЕЛЬНО должны существовать:
1. **PRD.md** -- Product Requirements Document с FR-ами и NFR-ами
2. **Architecture.md** -- технические решения, API контракты, модели данных
3. **UX Design.md** (рекомендуется если есть UI) -- паттерны взаимодействия

Эти документы создаются отдельными workflows:
- PRD: `2-plan-workflows/prd/workflow.md` (PM агент)
- Architecture: `3-solutioning/architecture/workflow.md` (Architect агент)
- UX: `2-plan-workflows/create-ux-design/workflow.md` (UX Designer агент)

---

## 2. Промпты для генерации

### 2.1 create-epics-and-stories

**Файлы промптов:**
| Файл | Строк | Назначение |
|------|-------|------------|
| `workflow.yaml` | 43 | Конфигурация: входы, выходы, пути |
| `instructions.md` | 387 | Основной промпт с 4 шагами |
| `epics-template.md` | 80 | Шаблон выходного файла |
| **workflow.xml** (core engine) | 234 | Runtime движок для всех workflows |
| **Итого промпт** | ~744 | Всё что загружается в контекст |

**4 шага instructions.md:**

1. **Step 0 -- Валидация и загрузка контекста** (~110 строк)
   - Проверка наличия PRD, Architecture, UX
   - Извлечение ВСЕХ FR-ов из PRD (полная инвентаризация)
   - Извлечение технического контекста из Architecture
   - Извлечение UX контекста (если есть)
   - Checkpoint: context_validation + fr_inventory

2. **Step 1 -- Проектирование структуры эпиков** (~57 строк)
   - Принцип: каждый эпик = USER VALUE (не технический слой)
   - Анти-паттерн: "Database Schema" -> "API Endpoints" -> "Frontend" (ЗАПРЕЩЕНО)
   - Правильно: "User Auth & Profile" -> "Content Creation" -> "Content Discovery"
   - Исключение: Foundation/setup эпик в начале допустим
   - Checkpoint: epics_structure_plan + epics_technical_context

3. **Step 2 -- Создание stories для каждого эпика** (повторяется, ~100 строк)
   - User Story формат: As a / I want / So that
   - BDD Acceptance Criteria: Given/When/Then с полными деталями
   - Включение: API endpoints, DB операции, auth, error handling, performance
   - UX детали: экраны, формы, валидации, responsive, accessibility
   - Sizing: каждая story = 1 dev agent session
   - Checkpoint: per-epic с FR coverage

4. **Step 3 -- Финальная валидация** (~80 строк)
   - FR Coverage Matrix: каждый FR привязан к story
   - Architecture Integration: все API endpoints покрыты
   - UX Integration: все user flows реализованы
   - Story Quality: размер, тестируемость, зависимости
   - Checkpoint: final_validation + fr_coverage_matrix

**Ключевые инструкции в промпте:**
- "ABSOLUTELY NO TIME ESTIMATES" -- AI неспособен предсказывать время
- "EVERY story must be completable by a single dev agent in one focused session"
- "LIVING DOCUMENT: Write to epics.md continuously" -- не ждать конца
- "CHECKPOINT PROTOCOL: After EVERY template-output tag" -- сохранять + спрашивать

### 2.2 create-story

**Файлы промптов:**
| Файл | Строк | Назначение |
|------|-------|------------|
| `workflow.yaml` | 58 | Конфигурация с input_file_patterns |
| `instructions.xml` | 323 | XML-промпт, 6 шагов |
| `template.md` | 51 | Шаблон story-файла |
| `checklist.md` | 358 | Checklist для валидации (используется при validate-create-story) |
| **Итого промпт** | ~790 | Всё что загружается |

**6 шагов instructions.xml:**

1. **Step 1 -- Определение целевой story** (~144 строк)
   - Автоматическое обнаружение из sprint-status.yaml (первый "backlog")
   - Или пользователь указывает номер (e.g., "2-4" или "epic 1 story 5")
   - Обновление epic status на "in-progress" если первая story

2. **Step 2 -- Загрузка и анализ артефактов** (~40 строк)
   - Загрузка epics_content -- полный контекст эпика
   - Извлечение нашей story: AC, User Story, требования
   - Анализ предыдущей story: dev notes, review feedback, файлы
   - Git intelligence: последние 5 коммитов, паттерны

3. **Step 3 -- Архитектурный анализ** (~20 строк)
   - Систематический анализ Architecture для story-relevant требований
   - Извлечение: stack, code structure, API, DB, security, testing, deployment

4. **Step 4 -- Web research** (~15 строк)
   - Исследование актуальных версий библиотек/API
   - Безопасность, breaking changes, best practices

5. **Step 5 -- Создание story файла** (~50 строк)
   - Заполнение шаблона template.md
   - Секции: header, requirements, developer_context, technical_requirements,
     architecture_compliance, library_framework, file_structure, testing,
     previous_story_intelligence, git_intelligence, latest_tech, project_context
   - Статус: "ready-for-dev"

6. **Step 6 -- Обновление sprint-status** (~40 строк)
   - Валидация через checklist.md
   - Обновление sprint-status.yaml: "backlog" -> "ready-for-dev"
   - Отчёт пользователю

**Ключевые инструкции:**
- "ULTIMATE story context engine that prevents LLM developer mistakes"
- "EXHAUSTIVE ANALYSIS REQUIRED: do NOT be lazy or skim!"
- "ZERO USER INTERVENTION" -- полностью автоматический после выбора story
- "COMMON LLM MISTAKES TO PREVENT: reinventing wheels, wrong libraries, wrong file locations..."

### 2.3 sprint-planning

**Файлы промптов:**
| Файл | Строк | Назначение |
|------|-------|------------|
| `workflow.yaml` | 51 | Конфигурация |
| `instructions.md` | 234 | Инструкции, 5 шагов |
| `sprint-status-template.yaml` | template | Шаблон status-файла |
| `checklist.md` | checklist | Валидация |
| **Итого** | ~285+ | |

Генерирует `sprint-status.yaml` из epic-файлов, отслеживая статус каждой story.

---

## 3. Валидация

### 3.1 Механизм валидации

BMad использует двухуровневую валидацию:

**Уровень 1: validate-workflow.xml (встроенная)**
- Универсальный framework для валидации любого workflow output
- 88 строк XML
- Загружает checklist + документ -> проверяет каждый пункт -> генерирует отчёт
- Маркировка: PASS / PARTIAL / FAIL / N/A
- Выходной файл: `validation-report-{timestamp}.md`

**Уровень 2: validate-create-story (checklist.md, 358 строк)**
- Называется "Story Context Quality Competition"
- Позиционируется как "competition" -- свежий LLM конкурирует с оригинальным
- Рекомендуется запускать в свежем контексте другой LLM

### 3.2 Checklist items (validate-create-story)

Checklist разбит на 5 категорий проверок:

**Step 2: Exhaustive Source Document Analysis**
- 2.1 Epics and Stories Analysis -- полнота извлечения контекста эпика
- 2.2 Architecture Deep-Dive -- все ли технические требования учтены
- 2.3 Previous Story Intelligence -- учтены ли learnings предыдущей story
- 2.4 Git History Analysis -- паттерны из последних коммитов
- 2.5 Latest Technical Research -- актуальные версии библиотек

**Step 3: Disaster Prevention Gap Analysis**
- 3.1 Reinvention Prevention -- дублирование функциональности
- 3.2 Technical Specification Disasters -- неверные библиотеки, API контракты
- 3.3 File Structure Disasters -- неверное расположение файлов
- 3.4 Regression Disasters -- breaking changes, тесты
- 3.5 Implementation Disasters -- нечёткие реализации, scope creep

**Step 4: LLM-Dev-Agent Optimization**
- Verbosity problems -- лишние токены
- Ambiguity issues -- неоднозначные инструкции
- Context overload -- нерелевантная информация
- Missing critical signals -- ключевые требования потерялись
- Poor structure -- неэффективная организация для LLM

**Step 5: Improvement Recommendations**
- 5.1 Critical Misses (Must Fix)
- 5.2 Enhancement Opportunities (Should Add)
- 5.3 Optimization Suggestions (Nice to Have)
- 5.4 LLM Optimization Improvements

### 3.3 Автоматическая vs ручная проверка

| Аспект | Автоматическая | Ручная |
|--------|---------------|--------|
| FR Coverage Matrix | Генерируется в create-epics | Проверяется пользователем на checkpoint |
| Story completeness | checklist.md в validate-create-story | Пользователь выбирает improvements |
| Sprint status | sprint-planning автоматически определяет | Пользователь может override |
| Architecture compliance | Извлекается в create-story | Validate-create-story ищет пропуски |

**По сути вся валидация -- это LLM-as-judge:**
Один LLM (в свежем контексте) проверяет output другого LLM. Нет програмной/rule-based валидации.

---

## 4. Формат story

### 4.1 Шаблон (template.md, 51 строка)

**Обязательные секции:**

```markdown
# Story {epic_num}.{story_num}: {story_title}
Status: drafted

## Story                          -- User Story (As a / I want / So that)
## Acceptance Criteria            -- Пронумерованные AC
## Tasks / Subtasks               -- Чеклист задач с привязкой к AC
## Dev Notes                      -- Архитектурные паттерны, файлы для изменения
### Project Structure Notes       -- Alignment с project structure
### References                    -- Ссылки на исходные документы
## Dev Agent Record               -- Заполняется при разработке
### Context Reference             -- Путь к story context XML
### Agent Model Used              -- Модель LLM
### Debug Log References          -- Логи
### Completion Notes List         -- Заметки о завершении
### File List                     -- Список изменённых файлов
```

### 4.2 Как задаётся формат

Формат задаётся ДВУМЯ механизмами:

1. **template.md** -- скелет с плейсхолдерами (51 строка)
   - Определяет обязательные секции и их порядок
   - Плейсхолдеры типа `{{role}}`, `{{action}}`, `{{benefit}}`

2. **instructions.xml** -- подробные инструкции ЧТО писать в каждую секцию (323 строки)
   - template-output теги указывают какой контент генерировать
   - Каждая секция наполняется на основе анализа артефактов

### 4.3 Реальный формат (из проекта bmad-ralph)

Реальные story-файлы содержат дополнительные секции, добавленные workflows:
- Более детальные AC с конкретными значениями (FR-ссылки, имена функций)
- Tasks с подзадачами, привязанными к конкретным AC (через `(AC: #N)`)
- Dev Notes с конкретными путями файлов, паттернами кода
- References со ссылками вида `[Source: docs/<file>.md#Section]`

### 4.4 Опциональные секции

- `### Project Structure Notes` -- может быть пустой
- `### Context Reference` -- заполняется context workflow (если используется)
- `### Debug Log References` -- заполняется при debug
- Вся секция `Dev Agent Record` -- заполняется при разработке, пуста при создании

---

## 5. Анализ встраивания в ralph

### 5.1 Что можно взять из create-epics-and-stories

**Можно встроить:**
- Принцип "User Value First" для структуры эпиков (описание в промпте)
- FR Coverage Matrix как проверочный механизм
- Шаблон epics-template.md (80 строк)
- Правила размера stories: "completable by single dev agent in one session"

**Объём промпта:** ~387 строк instructions + 80 строк template = **~467 строк**
(без workflow.xml engine, который добавляет ещё 234 строки)

**Что упрощается без BMad agents:**
- Не нужен workflow.xml engine (checkpoints, YOLO mode, elicitation)
- Не нужен discover_inputs protocol (шаблон поиска файлов)
- config.yaml резолюция переменных не нужна (ralph имеет свой config)

**Минимальный промпт для ralph:** ~250-300 строк (instructions stripped до сути)

### 5.2 Что можно взять из create-story

**Можно встроить:**
- Логику автоматического обнаружения следующей story из sprint-status
- Шаблон template.md (51 строка)
- Паттерн анализа предыдущей story (dev notes, review feedback)
- Git intelligence (последние 5 коммитов)
- Принцип "EXHAUSTIVE ANALYSIS" для контекста

**Объём промпта:** ~323 строк instructions + 51 строк template + 358 строк checklist = **~732 строки**

**Минимальный промпт для ralph:** ~200-250 строк (без web research, без validate)

### 5.3 Что можно взять из sprint-planning

**Можно встроить:**
- Логику парсинга epic-файлов -> sprint-status.yaml
- State machine для статусов: backlog -> drafted -> ready-for-dev -> in-progress -> review -> done
- Формат sprint-status.yaml

**Объём:** ~234 строки instructions, минимальный для ralph: ~100 строк

### 5.4 Что теряется без BMad agents

| Потеря | Критичность | Альтернатива в ralph |
|--------|------------|---------------------|
| PM агент (John) -- "WHY?" мышление | Средняя | Промпт с инструкциями для Claude |
| Architect агент (Winston) -- "boring tech" | Средняя | Architecture.md как входной документ |
| SM агент (Bob) -- checklist-driven мышление | Низкая | Автоматическая валидация |
| Интерактивные checkpoints (YOLO/Continue/Elicit) | Низкая | ralph работает автономно |
| Party-mode (multi-agent обсуждение) | Низкая | Не применимо к CLI |
| validate-create-story "Quality Competition" | Средняя | Можно реализовать как review pass |
| Web research для актуальных версий | Средняя | Claude имеет базовые знания |
| discover_inputs protocol (fuzzy file matching) | Низкая | ralph знает свои пути |

### 5.5 Основные инсайты для ralph

1. **Промпты для генерации эпиков (~300 строк чистого контента)** содержат:
   - Извлечение FR inventory из PRD
   - User Value принцип для структуры эпиков
   - Подробный пример "хорошей" story с BDD AC
   - FR Coverage Matrix для валидации полноты

2. **Промпты для генерации stories (~250 строк чистого контента)** содержат:
   - 6-шаговый анализ: arterfacts, architecture, git, web, compose, validate
   - Паттерн "предотвращения LLM-ошибок" (reinventing wheels, wrong libs)
   - Template с обязательными секциями
   - Автоматическое обновление sprint-status

3. **Валидация stories (~350 строк чистого контента)** содержит:
   - 5-категорийный disaster prevention analysis
   - LLM optimization analysis (token efficiency, clarity)
   - Interactive improvement process

4. **Суммарный объём для полного встраивания:** ~900-1000 строк промптов
   - create-epics: ~300 строк
   - create-story: ~250 строк
   - validate-story: ~350 строк
   - sprint-planning: ~100 строк

5. **Минимальный MVP:** ~400-500 строк
   - create-story (stripped): ~200 строк
   - story template: ~50 строк
   - sprint-status management: ~100 строк
   - basic validation: ~50 строк

---

## 6. Архитектура BMad Workflow Engine

### 6.1 workflow.xml -- универсальный движок

Все workflows BMad выполняются через единый runtime engine (`workflow.xml`, 234 строки):

- Загрузка workflow.yaml -> резолюция переменных -> загрузка instructions
- Пошаговое выполнение с checkpoints (template-output tags)
- Интерактивные режимы: Normal (подтверждение каждого шага) vs YOLO (автоматический)
- Протоколы: discover_inputs (fuzzy file matching для PRD/Architecture/UX/Epics)
- Поддержка: action, check, ask, goto, invoke-workflow, invoke-task, invoke-protocol

### 6.2 Агенты как роли

Каждый агент -- это промпт-файл с:
- Persona (роль, identity, communication_style, principles)
- Menu (список доступных workflows)
- Menu-handlers (как исполнять workflow/exec/validate-workflow)
- Activation steps (инициализация: загрузка config, приветствие)

**Для ralph это означает:** агенты не нужны как отдельные сущности.
Их knowledge встроен в instructions каждого workflow.

### 6.3 Потоки данных

```
config.yaml
    |
    v
workflow.yaml (конфигурация) ---> workflow.xml (engine)
    |                                    |
    v                                    v
instructions.md/xml (промпт)     template.md (формат вывода)
    |                                    |
    v                                    v
input_file_patterns              default_output_file
(PRD, Architecture, Epics)       (docs/epics.md или story.md)
    |
    v
checklist.md (для validate)
```

---

## 7. Рекомендации

### 7.1 Для ralph v2

1. **Начать с create-story** -- самый ценный workflow для автоматизации.
   Story-файлы уже существуют в проекте, ralph может генерировать новые.

2. **sprint-status.yaml** -- простой state machine, ralph уже отчасти реализует tracking.

3. **create-epics** -- можно добавить позже. Это одноразовая операция в начале проекта,
   менее критично для автоматизации чем per-story генерация.

4. **validate-create-story** -- можно реализовать как отдельный review pass в ralph.
   Принцип "Quality Competition" хорошо ложится на парадигму adversarial review.

### 7.2 Стратегия встраивания

**Phase 1: Story Generator (~250 строк промпта)**
- Генерация story из epic-файла с полным контекстом
- Git intelligence (предыдущие коммиты)
- Previous story analysis
- Template-based output

**Phase 2: Sprint Tracking (~100 строк)**
- Парсинг epic-файлов -> sprint-status.yaml
- State machine: backlog -> done
- Auto-discover next story

**Phase 3: Epic Generator (~300 строк)**
- PRD -> epic breakdown
- FR Coverage Matrix
- User Value principle

**Phase 4: Story Validator (~350 строк)**
- Adversarial review pass
- Disaster prevention analysis
- LLM optimization pass
