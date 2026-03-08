# Вариант D: Cost-Benefit Analysis — BMad-зависимость vs Самодостаточность

## Executive Summary

ralph сейчас жёстко зависит от BMad Method: цепочка PRD -> Architecture -> Epics -> Stories -> Bridge -> sprint-tasks.md -> Runner. Анализ показывает, что эта зависимость создаёт **3 AI-слоя "испорченного телефона"**, уникальный для индустрии **persistent task file антипаттерн**, и **когнитивную нагрузку**, непропорциональную получаемой ценности. Ни один из 8 рассмотренных конкурентов (Devin, Claude Code, Aider, SWE-Agent, OpenHands, AutoCodeRover, Cursor, Windsurf) не использует аналогичную внешнюю зависимость для task planning.

Рекомендация: **гибридный подход** — ralph принимает любой входной формат (BMad stories, GitHub Issues, plain text), имеет встроенный lite planner, и работает полностью автономно для типовых сценариев.

---

## 1. Стоимость зависимости от BMad

### 1.1. Шаги до первой задачи

Текущий pipeline от идеи до выполнения первой задачи ralph:

| # | Шаг | Кто выполняет | Время | Стоимость |
|---|------|---------------|-------|-----------|
| 1 | Написание PRD | BMad AI (pm-draft) | 15-45 мин | $0.50-2.00 |
| 2 | Architecture Document | BMad AI (architect) | 15-30 мин | $0.50-1.50 |
| 3 | Epic Decomposition | BMad AI (pm) | 10-20 мин | $0.30-1.00 |
| 4 | Story Creation (per story) | BMad AI (create-story) | 5-10 мин | $0.20-0.50 |
| 5 | Story Validation | BMad AI (validate-workflow) | 3-5 мин | $0.10-0.30 |
| 6 | `ralph bridge` (per batch) | Claude (bridge prompt) | 2-5 мин * 6 batch'ей | $0.60-3.00 |
| 7 | `ralph run` (первая задача) | Claude (execute prompt) | 5-15 мин | $0.30-1.50 |
| **ИТОГО** | | | **55-130 мин** | **$2.50-9.80** |

**Time-to-first-task: 55-130 минут** от идеи до первого коммита.

Для сравнения у конкурентов:

| Инструмент | Время от идеи до первого коммита |
|-----------|----------------------------------|
| Devin | 2-5 мин (Slack message -> PR) |
| Claude Code | 1-3 мин (текст -> код) |
| Aider | 1-2 мин (instruction -> commit) |
| SWE-Agent | 3-10 мин (GitHub Issue -> PR) |
| **ralph** | **55-130 мин** |

ralph **в 20-60x медленнее** конкурентов на этапе "от идеи до первого результата".

### 1.2. Cognitive Overhead

Пользователь ralph должен знать:

1. **BMad Method** — что такое PRD, Architecture Doc, Epics, Stories, AC, Dev Notes
2. **BMad агенты** — pm-draft, architect, pm, create-story, validate-workflow + их CLI
3. **BMad конфигурация** — `.bmad/bmm/config.yaml`, persona, workflows
4. **Формат story файлов** — структура AC, Dev Notes, Testing Standards, References
5. **Bridge семантика** — batching, merge mode, `[GATE]` маркеры, `source:` traceability
6. **sprint-tasks.md формат** — `- [ ]` / `- [x]`, source поля, epic headers
7. **ralph конфигурация** — `.ralph.yaml`, stories_dir, gates, reviews, etc.

**Общая поверхность знаний: ~7 концепций, ~15 файлов конфигурации, ~3 инструмента.**

У конкурентов:

| Инструмент | Что нужно знать |
|-----------|-----------------|
| Devin | Slack/GitHub integration, описание задачи |
| Claude Code | CLAUDE.md, описание задачи |
| Aider | `.aider.conf.yml`, описание задачи |
| **ralph + BMad** | BMad Method + BMad CLI + ralph CLI + 7 концепций |

### 1.3. Vendor Lock-in

- **BMad** — сторонний набор workflow-ов для AI-генерации документации
- Нет гарантии обратной совместимости при обновлениях BMad
- Story формат может измениться — bridge prompt (244 строки) привязан к конкретной структуре
- ralph без BMad **неработоспособен** — нет альтернативного способа создать sprint-tasks.md
- BMad — niche инструмент без массовой adoption (в отличие от GitHub Issues, Jira, Linear)

### 1.4. Проблема "испорченного телефона" (3 AI-слоя)

```
BMad AI (stories с AC)  -->  Bridge AI (sprint-tasks.md)  -->  Runner AI (код)
     ^                            ^                              ^
   Claude                       Claude                        Claude
   Слой 1                       Слой 2                        Слой 3
```

Каждый слой:
- Теряет нюансы предыдущего
- Добавляет собственные интерпретации
- Снижает signal-to-noise ratio

Конкретный пример из проекта: Story 9.1 содержит детальный scaling algorithm (`position < 0.33 -> LOW`). Bridge сжимает до "Implement ProgressiveParams function". Runner открывает story заново для деталей. Bridge не добавил информации — он её потерял и заставил runner восстанавливать.

Исследование ADaPT (NAACL 2024) показывает: **as-needed decomposition на 28-33% эффективнее upfront decomposition**. Bridge делает именно upfront decomposition — конвертирует ВСЕ AC в задачи ДО начала работы.

---

## 2. Стоимость самодостаточности

### 2.1. Что ralph должен реализовать сам

| Компонент | Описание | Оценка сложности | LOC (оценка) |
|-----------|----------|-------------------|--------------|
| **Input Parser** | Чтение разных форматов: stories, issues, plain text | Средняя | 200-400 |
| **Task Decomposer** | Разбиение требований на атомарные задачи | Высокая | 300-500 |
| **Format Normalizer** | Приведение входных данных к единому внутреннему формату | Низкая | 100-200 |
| **Lite Planner** | Программная генерация sprint-tasks.md из structured input | Средняя | 200-300 |
| **Complexity Detector** | Определение: простой input (программно) vs сложный (LLM) | Средняя | 100-200 |
| **Progress Tracker** | Отслеживание выполнения без sprint-tasks.md (`.ralph-state.yaml`) | Средняя | 200-300 |
| **ИТОГО** | | | **1100-1900** |

Текущий bridge: ~143 LOC (`bridge.go`) + ~244 строк prompt (`bridge.md`) = ~387 строк.

**Самодостаточная версия: ~1100-1900 LOC кода, но 0 строк промптов для planning.** Промпты bridge (244 строки) полностью устраняются.

### 2.2. Объём промптов для удаления/замены

| Файл | Строк | Судьба |
|------|-------|--------|
| `bridge/prompts/bridge.md` | 244 | Удаляется полностью |
| `bridge/bridge.go` | 143 | Удаляется или рефакторится |
| `cmd/ralph/bridge.go` | 122 | Удаляется (CLI subcommand) |
| `config/sprint_tasks_format.go` | ~50 | Может остаться для backward compatibility |
| **ИТОГО удаляется** | **~559** | |

### 2.3. Риск: ralph становится "ещё одним BMad"?

**Нет, если чётко разграничить scope:**

| BMad | ralph (самодостаточный) |
|------|------------------------|
| PRD generation | НЕ нужно — ralph не генерирует PRD |
| Architecture docs | НЕ нужно — ralph не генерирует architecture |
| Epic decomposition | НЕ нужно — ralph не генерирует epics |
| Story creation + validation | ЧАСТИЧНО — ralph парсит requirements, не создаёт stories |
| Task decomposition | ДА — ralph разбивает input на атомарные задачи |

ralph НЕ должен воспроизводить BMad Method. ralph должен принимать **любое описание задачи** (от одной строки до полноценной story) и декомпозировать его в атомарные сессии. Это то, что делают все конкуренты.

### 2.4. Качество: может ли ralph дать качество BMad validate-workflow?

**Валидация BMad проверяет:**
- Полнота AC (каждое требование покрыто)
- Testability AC (каждый AC тестируем)
- Dev Notes consistency (ссылки на architecture соответствуют)
- Story scope (не слишком большая, не слишком маленькая)

**Ответ:** ralph НЕ НУЖНО валидировать requirements так же глубоко, потому что:
1. Runner'ский Claude сам определяет что делать из task description
2. Code review (уже встроенный в ralph) ловит проблемы реализации
3. Stuck detection (уже встроенный) ловит задачи, которые Claude не может выполнить
4. Качество input — ответственность пользователя, а не инструмента

---

## 3. Гибридные варианты

| Вариант | Описание | Совместим с BMad? | Time-to-first-task | Сложность реализации |
|---------|----------|-------------------|--------------------|--------------------|
| **H1: BMad stories + fallback** | ralph работает с BMad stories ЕСЛИ они есть, но может принять plain text | Да | 5 мин (plain text), 55 мин (BMad) | Средняя |
| **H2: Multi-format input** | ralph принимает stories, GitHub Issues, YAML tasks, plain text | Да | 2-5 мин | Средняя-Высокая |
| **H3: Lite planner + BMad adapter** | Встроенный программный planner + опциональный BMad import | Да | 2-5 мин | Средняя |
| **H4: Direct story runner** | Runner работает напрямую со story файлами (без bridge) | Частично | 30 мин (BMad stories) | Низкая |
| **H5: Issue-driven** | ralph получает GitHub Issue URL, сам скачивает и работает | Нет | 2-3 мин | Средняя |

### Рекомендуемый гибрид: H3 (Lite Planner + BMad Adapter)

```
Вход: любой формат
  ├── plain text ("добавь авторизацию") --> Lite Planner --> tasks
  ├── GitHub Issue (#42) --> Issue Parser --> tasks
  ├── BMad story (9-1-*.md) --> Story Parser --> tasks (программный, без LLM)
  └── YAML tasks (tasks.yaml) --> Direct load --> tasks
        |
        v
  Internal Task List --> Runner (без изменений)
```

**Преимущества:**
- Нулевой vendor lock-in
- Time-to-first-task: 2-5 мин для 80% use cases
- BMad compatibility для существующих проектов
- Программный парсинг stories (0 LLM calls для planning)

---

## 4. Конкурентный анализ

### 4.1. Сводная таблица

| Инструмент | Входной формат | Внешний workflow? | Task file? | AI слоёв | Time-to-first |
|-----------|---------------|-------------------|------------|----------|---------------|
| **Devin** | Slack msg / Issue | Нет | Нет (in-session plan) | 1 | 2-5 мин |
| **Claude Code** | Текст / CLAUDE.md | Нет | plan.md (optional, in-session) | 1 | 1-3 мин |
| **Aider** | Текст / /architect | Нет | Нет | 1-2 | 1-2 мин |
| **SWE-Agent** | GitHub Issue | Нет | Нет | 1 | 3-10 мин |
| **OpenHands** | GitHub Issue | Нет | Нет | 1 | 3-10 мин |
| **AutoCodeRover** | Bug report | Нет | Нет (fixed pipeline) | 1 | 5-15 мин |
| **Cursor Composer** | Текст | Нет | Нет (in-session) | 1 | 1-2 мин |
| **MetaGPT/MGX** | PRD текст | Нет | Да (in-memory, structured) | 3-4 | 10-30 мин |
| **ralph + BMad** | BMad story files | **ДА (BMad Method)** | **Да (sprint-tasks.md, persistent)** | **3** | **55-130 мин** |

### 4.2. Ключевые наблюдения

**1. ralph — единственный инструмент с обязательной внешней зависимостью для planning.**
Все остальные принимают plain text / issue и работают самостоятельно.

**2. ralph — единственный с persistent task file как промежуточным артефактом.**
MetaGPT использует structured outputs между агентами, но они in-memory. Claude Code может создать plan.md, но в той же сессии, которая будет выполнять.

**3. 3 AI-слоя — максимум среди конкурентов.**
MetaGPT имеет 3-4 агента, но они работают в рамках одного orchestrator'а. ralph имеет 3 **изолированных** AI-сессии (BMad, bridge, runner) с потерей контекста на каждом переходе.

**4. Все успешные инструменты самодостаточны.**
Devin получает Slack message и делает PR. Claude Code получает текст и пишет код. Aider получает instruction и делает commit. Ни один не требует предварительного прохождения через внешний methodology framework.

### 4.3. Почему ralph должен быть исключением?

**Нет объективных причин для исключения.** Аргументы в пользу BMad-зависимости:

| Аргумент | Контраргумент |
|----------|---------------|
| "BMad даёт более качественные requirements" | Runner'ский Claude всё равно перечитывает story файл через source ссылку — bridge не добавляет качества |
| "Структурированные AC лучше plain text" | Верно (arxiv.org/pdf/2406.10101), но ralph может сам генерировать structured AC из plain text |
| "Validate-workflow ловит проблемы до реализации" | Code review ralph'а ловит проблемы после реализации — двойная валидация избыточна для типовых задач |
| "BMad даёт human checkpoints (stories)" | ralph gates дают human checkpoints на уровне задач — более гранулярно |
| "История проекта bmad-ralph доказывает качество" | 10 эпиков за 10 дней — впечатляет, но time-to-first-task 55+ мин для КАЖДОГО нового проекта — барьер adoption |

---

## 5. ROI расчёт: BMad vs Self-sufficient vs Hybrid

### 5.1. Стоимость разработки

| Вариант | Разработка (часы) | Поддержка (часы/мес) | Удалённый код | Новый код |
|---------|-------------------|---------------------|---------------|-----------|
| **BMad (текущий)** | 0 (готово) | 2-4 (синхр. промптов) | 0 | 0 |
| **Self-sufficient** | 40-60 | 1-2 | ~559 строк | ~1500 строк |
| **Hybrid (H3)** | 24-40 | 1-2 | ~244 строки (bridge prompt) | ~1000 строк |

### 5.2. Операционная стоимость на проект

| Вариант | Setup (мин) | Per-sprint (мин) | Claude cost/sprint | Cognitive load |
|---------|-------------|------------------|--------------------|----------------|
| **BMad** | 120-300 | 30-60 | $3-10 (bridge) | Высокая (7 концепций) |
| **Self-sufficient** | 5-15 | 5-15 | $0 (no bridge) | Низкая (2 концепции) |
| **Hybrid** | 5-15 (new), 30-60 (BMad import) | 5-15 | $0 | Средняя (3 концепции) |

### 5.3. Breakeven

При стоимости разработки hybrid (H3) в **32 часа** (серединная оценка):

- **Экономия на setup**: ~2 часа на проект (BMad setup vs plain text)
- **Экономия на sprint**: ~30 мин на sprint * 10 sprints = 5 часов/проект
- **Экономия Claude calls**: $5/sprint * 10 = $50/проект
- **Breakeven**: 32 часа / 7 часов экономии = **~5 проектов**

Для self-sufficient варианта: 50 часов / 7 часов = **~7 проектов.**

### 5.4. Нематериальные выгоды (не поддаются прямому расчёту)

| Фактор | BMad | Self-sufficient | Hybrid |
|--------|------|-----------------|--------|
| Adoption barrier | Высокий | Низкий | Низкий |
| Community growth potential | Низкий (niche) | Высокий | Высокий |
| Developer experience (DX) | Сложный | Простой | Простой |
| Конкурентоспособность | Отстаёт | Наравне | Наравне |
| Flexibility | Жёсткий | Максимальная | Высокая |

---

## 6. Рекомендация

### Категорическая рекомендация: Hybrid (H3) — Lite Planner + BMad Adapter

**Обоснование:**

1. **Конкурентный паритет.** Все успешные инструменты (Devin, Claude Code, Aider, SWE-Agent) самодостаточны. ralph с BMad-зависимостью — аномалия рынка.

2. **ADaPT research.** As-needed decomposition на 28-33% эффективнее upfront decomposition (bridge). Lite planner + fallback к LLM decomposition = ADaPT подход.

3. **Elimination антипаттерна.** ralph — единственный инструмент из 9 рассмотренных с persistent task file, созданным отдельным LLM-вызовом. Это уникальный антипаттерн, подтверждённый собственным bridge-concept-analysis.md.

4. **ROI.** Breakeven через 5 проектов, нематериальные выгоды (adoption, DX, конкурентоспособность) значительны.

5. **Backward compatibility.** BMad stories продолжают работать через Story Parser — существующие проекты не ломаются.

### План реализации

| Фаза | Что | Когда | Результат |
|------|-----|-------|-----------|
| **Фаза 1** | Plain text input → программный task generation | 2-3 дня | `ralph run "добавь auth"` работает |
| **Фаза 2** | Story parser (BMad adapter) без LLM | 1-2 дня | `ralph run --stories docs/` работает без bridge |
| **Фаза 3** | GitHub Issue input | 1-2 дня | `ralph run --issue 42` работает |
| **Фаза 4** | Deprecation bridge subcommand | 1 день | Warning при `ralph bridge` |

**Общий timeline: 5-8 дней разработки.**

### Что НЕ делать

- НЕ воспроизводить BMad Method внутри ralph (PRD, Architecture, Epics — не scope ralph)
- НЕ создавать "story validator" внутри ralph — это scope пользователя
- НЕ удалять bridge сразу — deprecate с warning, удалить через 2-3 релиза
- НЕ делать ralph "ещё одним project management tool" — ralph = executor, не planner

---

## 7. Источники

### Исследования
- [ADaPT: As-Needed Decomposition and Planning with Language Models](https://arxiv.org/abs/2311.05772) — NAACL 2024, as-needed decomposition на 28-33% эффективнее upfront planning
- [Requirements are All You Need: From Requirements to Code with LLMs](https://arxiv.org/pdf/2406.10101) — structured requirements vs plain text для LLM code generation
- [The Agentic Telephone Game: Cautionary Tale](https://www.christopheryee.org/blog/agentic-telephone-game-cautionary-tale/) — потеря информации при цепочках LLM-обработки
- [Why Do Multi-Agent LLM Systems Fail?](https://arxiv.org/pdf/2503.13657) — анализ ошибок в мультиагентных системах

### Инструменты и документация
- [Devin AI Guide 2026](https://aitoolsdevpro.com/ai-tools/devin-guide/) — Devin workflow и capabilities
- [Devin Review 2026](https://ai-coding-flow.com/blog/devin-review-2026/) — производительность и adoption
- [Claude Code and the Architecture of Autonomous Software Engineering](https://catalaize.substack.com/p/claude-code-and-the-architecture) — Claude Code как автономный агент
- [Enabling Claude Code to Work More Autonomously](https://www.anthropic.com/news/enabling-claude-code-to-work-more-autonomously) — best practices от Anthropic
- [Aider AI Coding Tool](https://aider.chat/) — self-contained CLI для AI-assisted coding
- [OpenHands vs SWE-Agent 2026](https://localaimaster.com/blog/openhands-vs-swe-agent) — сравнение open-source coding agents
- [SWE-Agent GitHub](https://github.com/SWE-agent/SWE-agent) — direct issue execution
- [MetaGPT Multi-Agent Framework](https://github.com/FoundationAgents/MetaGPT) — structured multi-agent outputs

### Внутренние исследования проекта
- `docs/research/bridge-concept-analysis.md` — критический анализ bridge, сводная таблица альтернатив
- `docs/research/bridge-performance-analysis.md` — оптимизация скорости bridge, batching проблема
