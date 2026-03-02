# Анализ конкурентов: Управление знаниями в AI Coding Agents (2025-2026)

**Аналитик:** analyst-3 | **Дата:** 2026-03-02 | **Контекст:** Ralph — Go CLI оркестратор Claude Code
**Раунд:** 2 (расширение baseline из `docs/reviews/analyst-3-competitors-report.md`)
**Фокус раунда 2:** Cold-start на НОВОМ проекте (любой стек), автоматическое извлечение знаний

---

> **Изменения относительно раунда 1:** Добавлены 5 новых инструментов (Kilo Code, Augment Code, Live-SWE-agent, CodeRabbit, claude-mem). Углублён анализ cold-start проблемы (раздел 2.6). Добавлена секция "Архитектурные паттерны" из R1 с уточнениями. Уточнены данные по Cursor (self-improving rules), Devin (advanced mode playbooks), Windsurf (Memories).

---

## 1. Сравнительная таблица

| Инструмент | Автоматическое извлечение знаний | Категоризация | Контекстная подача | Дистилляция/сжатие | Строит с нуля | Формат хранения |
|---|---|---|---|---|---|---|
| **Claude Code** (CLAUDE.md + auto memory) | Да — auto memory сохраняет build commands, debugging insights, паттерны | Да — MEMORY.md (индекс) + topic файлы + .claude/rules/*.md с glob | Да — rules с `paths:` frontmatter, skills по запросу | Частично — 200-строк лимит MEMORY.md, topic files по запросу | Да — /init генерирует CLAUDE.md из codebase | Markdown файлы в .claude/ |
| **Cursor** (.cursor/rules/) | Частично — /Generate Cursor Rules, Memories (авто) | Да — .cursor/rules/*.mdc, множество файлов | Да — Auto Attached (по glob), Agent Requested (по описанию), Manual | Нет явного механизма | Да — /Generate Rules из проекта | .mdc файлы с frontmatter |
| **Cline** (Memory Bank) | Нет — пользователь ведёт markdown-файлы вручную | Да — 6 файлов: projectbrief, productContext, activeContext, systemPatterns, techContext, progress | Нет — все файлы загружаются в начале каждой сессии | Да — Auto-Compact сжимает контекст при заполнении окна | Частично — структура предопределена, заполняется вручную | Markdown в memory-bank/ |
| **Kilo Code** (Memory Bank) | Нет — ручное документирование в context.md, brief.md, history.md | Да — context.md, brief.md, history.md | Частично — агент синтезирует при старте задачи | Нет | Частично — предопределённая структура | Markdown в репозитории |
| **Aider** (CONVENTIONS.md) | Нет — полностью ручной файл | Нет — один файл CONVENTIONS.md | Нет — загружается целиком через --read | Нет | Нет | Один Markdown файл |
| **GitHub Copilot** (AGENTS.md) | Нет — ручная курация | Да — .github/copilot-instructions.md + .github/instructions/*.md + AGENTS.md | Да — excludeAgent по типу агента, directory-based precedence | Нет | Нет | Markdown файлы |
| **OpenAI Codex** (AGENTS.md) | Нет — ручная курация | Да — AGENTS.md + AGENTS.override.md, directory-level hierarchy + Skills | Да — directory walk от root к cwd, Skills загружаются по метаданным | Нет | Нет | Markdown + YAML метаданные |
| **Devin** (Knowledge + Playbooks) | Частично — "advanced mode" может создавать/улучшать playbooks автоматически | Да — Knowledge (общие правила) vs Playbooks (процедуры задач) | Да — Playbooks привязаны к типам задач | Нет | Нет — требует ручную настройку Knowledge | .devin.md файлы + веб-UI |
| **Windsurf** | Частично — Memories (авто, как chat history) | Частично — AI Rules (global/project), Memories отдельно | Нет явного механизма | Нет | Нет | Встроенные настройки IDE |
| **Continue.dev** | Нет | Да — .continue/rules, MCP серверы | Частично — через MCP (Context7, DeepWiki) | Нет | Нет | YAML config + rules |
| **Augment Code** | Да — Memories автоматически обновляются | Нет явной категоризации | Да — Context Engine с семантической картой | Да — Context Engine обрабатывает 200k токенов | Да — индексирует codebase автоматически | Внутренний индекс (проприетарный) |
| **CrewAI** | Программатически — RAG + memory tools | Да — short-term, long-term, entity memory типы | Да — memory search по релевантности | Да — through RAG storage | Да — строит из взаимодействий | Программный API (Python) |
| **LangGraph** | Программатически — persistent state + LangMem | Да — через LangMem инструменты | Да — search_memory по запросу | Да — memory compaction | Да — через взаимодействия | Key-value store / DB |
| **SWE-agent / OpenHands** | Нет стандартного — micro-agents в Git | Нет стандартного | Нет стандартного | Нет | Нет | Код в Git |
| **Live-SWE-agent** | Да — создаёт инструменты на лету | Нет — самомодификация кода агента | Нет — ad-hoc | Нет | Да — начинает с минимального набора | Код Python |
| **CodeRabbit** (обзор) | Да — учится из PR-историй и feedback | Внутренний knowledge graph | Да — по контексту PR | Да — consolidation patterns | Да — из истории репозитория | Проприетарный граф |
| **claude-mem** (плагин) | Да — перехватывает ВСЕ tool invocations | Семантические summaries | Да — progressive disclosure (3-step) | Да — ~10x сжатие токенов | Да — с первой сессии | SQLite + Chroma vector DB |

---

## 2. Детальный анализ по вопросам

### 2.1. Кто АВТОМАТИЧЕСКИ извлекает знания?

**Полностью автоматические:**

1. **Claude Code auto memory** — Claude сам решает, что сохранить. Триггеры: коррекции пользователя, build commands, debugging insights. Сохраняет в `~/.claude/projects/<project>/memory/MEMORY.md` + topic-файлы. Первые 200 строк MEMORY.md загружаются в каждую сессию.

2. **claude-mem** (плагин) — перехватывает PostToolUse хуки, создаёт семантические summaries через Claude agent-sdk, хранит в SQLite+Chroma, инжектирует через progressive disclosure. Самый агрессивный подход к автоматическому извлечению.

3. **Augment Code** — Memories обновляются автоматически, Context Engine строит семантическую карту кодовой базы. Проприетарный, без доступа к внутренней механике.

4. **CodeRabbit** — строит knowledge graph из PR-истории, review-комментариев, принятых/отклонённых предложений. Учится непрерывно.

5. **Live-SWE-agent** — создаёт собственные инструменты на лету (self-evolving), но это не knowledge management в традиционном смысле, а самомодификация scaffold.

**Частично автоматические:**

6. **Cursor Memories** — сохраняет решения из чат-сессий автоматически, но не структурированно. `/Generate Cursor Rules` генерирует правила из проекта, но по запросу.

7. **Devin** — advanced mode может автоматически создавать и улучшать Playbooks, но это экспериментальная функция.

8. **Windsurf Memories** — запоминает решения как chat history, но не экстрагирует структурированные знания.

**Не автоматические:**
- Cline Memory Bank, Kilo Code, Aider, Copilot, Codex, SWE-agent, OpenHands — все требуют ручного создания/обновления файлов.

### 2.2. Категоризация знаний

**Многофайловые системы с категориями:**

| Система | Структура | Принцип разделения |
|---|---|---|
| Claude Code | MEMORY.md (индекс) + topic-файлы + .claude/rules/*.md | По теме (debugging.md, patterns.md) + по scope (glob paths) |
| Cursor | .cursor/rules/*.mdc | По теме, свободная структура |
| Cline | 6 предопределённых файлов | По типу информации (контекст, прогресс, паттерны) |
| Kilo Code | 3 файла (context, brief, history) | По временному горизонту |
| Copilot | instructions.md + instructions/*.md + AGENTS.md | По уровню (общие vs агент-специфичные) |
| Codex | AGENTS.md hierarchy + Skills | По директории + по задаче (Skills) |
| Devin | Knowledge + Playbooks | По назначению (общие правила vs процедуры) |
| CrewAI/LangGraph | short-term / long-term / entity memory | По типу памяти (программные абстракции) |

**Однофайловые:**
- Aider (CONVENTIONS.md) — один файл, загружается целиком
- SWE-agent / OpenHands — нет стандартной системы

**Вывод для Ralph:** Многофайловая структура с категоризацией — индустриальный стандарт 2025-2026. Однофайловый подход (Aider) считается устаревшим. Лучшие системы комбинируют индексный файл + topic-файлы (Claude Code, Kilo Code).

### 2.3. Контекстная подача знаний

**Лидеры контекстной подачи:**

1. **Claude Code .claude/rules/** — `paths:` frontmatter с glob-паттернами. Правила загружаются ТОЛЬКО когда Claude читает файлы, совпадающие с паттерном. Skills загружаются по запросу. Это самый гранулярный подход из файловых систем.

2. **Cursor** — 4 режима активации: Always, Apply Intelligently (AI решает), File-specific (glob), Manual (@mention). "Apply Intelligently" — уникальная фича: AI сам решает по описанию правила.

3. **Codex** — directory walk от root к cwd, Skills загружаются по метаданным (name, description), полный SKILL.md только при использовании. Progressive disclosure для Skills.

4. **CrewAI/LangGraph** — программный search по релевантности через vector store.

5. **claude-mem** — 3-step progressive disclosure: compact indices → chronological context → full details.

6. **Augment Code** — семантическая карта кода, cross-service tracking.

**Без контекстной подачи (всё загружается):**
- Cline Memory Bank — все 6 файлов каждую сессию
- Aider — CONVENTIONS.md целиком
- Kilo Code — все файлы при старте задачи

**Вывод для Ralph:** Контекстная подача — ключевой дифференциатор. Ralph уже использует glob-scoped rules (.claude/rules/), что на уровне лучших практик. Потенциал: добавить "agent-requested" активацию (как Cursor) или progressive disclosure (как claude-mem).

### 2.4. Дистилляция и сжатие

**Кто сжимает:**

1. **Cline Auto-Compact** — при заполнении context window создаёт comprehensive summary, заменяет историю. Это сжатие СЕССИИ, не накопленных знаний.

2. **Claude Code** — 200-строк лимит на MEMORY.md, topic-файлы по запросу. Claude сам управляет размером, перенося детали в topic-файлы. Это мягкая дистилляция.

3. **claude-mem** — ~10x сжатие токенов через семантические summaries (5000 → 500 токенов). Самый агрессивный подход.

4. **Augment Code** — Context Engine обрабатывает 200k токенов, извлекая семантическую суть. Проприетарный.

5. **CrewAI/LangGraph** — memory compaction через RAG, управляется программно.

6. **CodeRabbit** — consolidation из PR-истории, уменьшение false positives.

**Никто не сжимает:**
- Cursor, Aider, Copilot, Codex, Devin, Windsurf, SWE-agent — знания растут линейно.

**Вывод для Ralph:** Дистилляция — самый недоразвитый аспект индустрии. Только claude-mem и CrewAI/LangGraph реально сжимают накопленные знания. Claude Code auto memory — единственная мейнстрим система с мягкой дистилляцией (topic-файлы). Это область для инноваций Ralph.

### 2.5. Какой подход лучше на практике?

**По результатам исследования, лучшие практики 2025-2026:**

1. **Иерархическая система файлов** (Claude Code, Cursor, Codex) — побеждает однофайловые (Aider) и предопределённые структуры (Cline). Причина: масштабируемость + организация.

2. **Glob-scoped правила** (Claude Code, Cursor) — побеждают "загрузить всё" (Cline, Aider). Причина: экономия контекста, релевантность.

3. **Автоматическое извлечение** (Claude Code auto memory, Augment Memories) — побеждает ручную курацию (Aider, Copilot). Причина: знания накапливаются без усилий.

4. **Разделение инструкций и памяти** (Claude Code: CLAUDE.md vs MEMORY.md) — побеждает смешение (Cline). Причина: ясная ответственность, разные lifecycle.

5. **Progressive disclosure** (Codex Skills, claude-mem) — побеждает flat loading. Причина: эффективность контекста при росте знаний.

**Benchmark данные:**
- Live-SWE-agent (самоэволюция) достигает 79.2% на SWE-bench Verified — демонстрируя ценность адаптивного knowledge management
- Augment Code: 70.6% SWE-bench с Context Engine
- CodeRabbit: снижение false positives через обучение из feedback

### 2.6. Построение знаний с нуля на новом проекте

**Кто начинает с нуля:**

1. **Claude Code** — `/init` анализирует codebase и генерирует стартовый CLAUDE.md. Auto memory начинает копить с первой сессии. .claude/rules/ создаются по мере необходимости.

2. **Augment Code** — автоматически индексирует весь codebase, строит семантическую карту. Полностью автоматический cold start.

3. **Cursor** — `/Generate Cursor Rules` создаёт правила из проекта. Memories начинают копить автоматически.

4. **CodeRabbit** — начинает учиться из первого PR. Knowledge graph строится постепенно.

5. **claude-mem** — начинает захват с первой сессии, строит SQLite+vector store постепенно.

6. **CrewAI/LangGraph** — программное API позволяет строить memory с нуля.

7. **Live-SWE-agent** — начинает с минимального набора shell tools, строит инструменты на лету.

**Кто НЕ может начать с нуля:**
- Cline Memory Bank — требует ручного создания 6 файлов
- Aider — CONVENTIONS.md полностью ручной
- Devin — Knowledge создаётся вручную
- Copilot/Codex — AGENTS.md пишется вручную

**Вывод для Ralph:** Автоматический cold start — конкурентное преимущество. Ralph должен уметь: (1) проанализировать новый codebase и сгенерировать начальные rules, (2) начать копить знания из code-review с первого цикла, (3) постепенно категоризировать и сжимать.

### 2.7. УГЛУБЛЁННЫЙ АНАЛИЗ: Cold-Start на новом проекте

Это ключевой вопрос для Ralph: пользователь запускает Ralph на НОВОМ проекте (любой стек). Какие знания нужны с первого цикла и откуда их взять?

#### Фазы cold-start

| Фаза | Что нужно знать | Кто решает лучше всего | Как |
|---|---|---|---|
| **0. До первого запуска** | Стек, структура, build commands | Augment Code, Claude Code `/init` | Автоматический scan codebase |
| **1. Первая сессия** | Coding conventions, architecture | Cursor `/Generate Rules`, Augment indexing | Генерация из анализа кода |
| **2. Первый code-review** | Типичные ошибки, anti-patterns | CodeRabbit (из PR), Ralph (из review) | Извлечение из findings |
| **3. Первые 5 сессий** | Workflow patterns, debugging tricks | Claude Code auto memory, Windsurf Memories | Накопление из коррекций |
| **4. Зрелость (10+ сессий)** | Refined patterns, дистилляция | claude-mem, CrewAI compaction | Сжатие и оптимизация |

#### Детальное сравнение cold-start стратегий

**1. Claude Code `/init`**
- Анализирует codebase: package.json/go.mod, README, структуру директорий
- Генерирует CLAUDE.md с build commands, test instructions, conventions
- НЕ генерирует .claude/rules/ — только корневой файл
- Auto memory начинает копить с первой коррекции пользователя
- **Ограничение:** `/init` — one-shot, не обновляет существующий CLAUDE.md инкрементально
- **Для Ralph:** Ralph может вызывать `/init`-подобный scan, но rules файлы создавать из code-review findings

**2. Augment Code Context Engine**
- Полностью автоматический: индексирует 400K+ файлов без ручной настройки
- Строит семантическую карту: зависимости, архитектурные паттерны, эволюция кода
- Memories обновляются автоматически из сессий
- **Ограничение:** проприетарный, нет файлового интерфейса, не портабельный
- **Для Ralph:** модель "индексирование → semantic map" недоступна для CLI, но подход "Memories автоматически" — применим

**3. Cursor `/Generate Cursor Rules` + Self-Improving Rules**
- Генерирует .cursor/rules/ из анализа проекта по запросу пользователя
- Self-improving rules: cursor-rules.mdc учит Cursor создавать/обновлять правила при изменении codebase
- Memories (встроенные) начинают копить решения автоматически
- **Ограничение:** правила — статические после генерации, не обновляются из code-review
- **Для Ralph:** паттерн "meta-rule учит создавать правила" — интересен для Ralph review prompt

**4. CodeRabbit (knowledge graph из PR)**
- Начинает учиться с ПЕРВОГО PR без настройки
- Строит knowledge graph: patterns, conventions, review outcomes
- Адаптируется из feedback: принятые/отклонённые предложения корректируют модель
- **Ограничение:** SaaS, привязан к PR workflow, не работает для general coding sessions
- **Для Ralph:** ближайший аналог — Ralph тоже извлекает из review, но может начать раньше (с первого code-review цикла, до PR)

**5. claude-mem (плагин Claude Code)**
- Начинает захват с ПЕРВОЙ tool invocation
- Нет "настройки" — работает out of the box через hooks
- Progressive disclosure: не нагружает контекст, подаёт по запросу
- **Ограничение:** зависит от SQLite+Chroma, extra infra, не стандартизирован
- **Для Ralph:** архитектура hooks → capture → compress → inject — возможная модель для Ralph knowledge pipeline

**6. Kilo Code / Roo Code Memory Bank**
- Предопределённая структура (context.md, brief.md, history.md) — быстрый старт
- Агент синтезирует при старте каждой задачи
- **Ограничение:** файлы заполняются ВРУЧНУЮ, нет auto-extraction
- **Для Ralph:** шаблонная структура для нового проекта — хороший паттерн, но автоматизация заполнения критична

#### Cold-start матрица: что доступно когда

| Момент | Augment | Claude Code | Cursor | Ralph (текущий) | Ralph (цель) |
|---|---|---|---|---|---|
| **Установка (0 сессий)** | Полный индекс | /init → CLAUDE.md | /Generate Rules | Ничего | Scan → initial rules |
| **После 1й сессии** | Memories | Auto memory | Memories | Ничего | Первые findings → rules |
| **После 1го review** | N/A | Auto memory растёт | N/A | Ручной протокол | Авто-extraction |
| **После 5 сессий** | Rich context | MEMORY.md + topics | Rules + Memories | ~20 правил (ручных) | ~20 правил (авто) |
| **После 10+ сессий** | Full semantic map | Mature rules | Mature rules | ~50 правил | ~50 правил + дистилляция |

#### Рекомендация для Ralph: трёхфазный cold-start

**Фаза 1: Bootstrap (первый запуск на новом проекте)**
- Scan: go.mod/package.json → стек, deps
- Scan: директории → architecture layout
- Scan: existing CI config → build/test commands
- Генерировать: starter `.claude/rules/project-conventions.md`
- Источник вдохновения: Claude Code `/init`, Augment indexing

**Фаза 2: Learning (первые 3-5 code-review циклов)**
- Каждый finding из review → candidate для rules
- Группировка: если finding повторяется 2+ раз → promote в rule
- Формат: markdown с file:line citations (как текущие .claude/rules/)
- Источник вдохновения: CodeRabbit learning from feedback, Ralph текущий протокол

**Фаза 3: Maturity (10+ циклов)**
- Дистилляция: объединение похожих rules, удаление устаревших
- Метрики: violation frequency → escalation (уже есть в Ralph)
- Архивация: low-frequency rules в отдельный файл
- Источник вдохновения: claude-mem compression, CrewAI compaction

---

## 3. Архитектурные паттерны (из R1 + уточнения R2)

### Паттерн A: "Статические правила с glob-скопингом"
**Используют:** Cursor, GitHub Copilot, Claude Code, Continue.dev
- Правила — markdown файлы в git, YAML frontmatter контролирует активацию
- **Плюсы:** предсказуемо, версионируется, O(1) lookup
- **Минусы:** требует ручного обслуживания, не масштабируется на 1000+ правил
- **R2 уточнение:** Cursor добавил "self-improving rules" — meta-rule обучает создавать новые правила

### Паттерн B: "Semantic knowledge recall"
**Используют:** Devin, Augment Code, OpenHands SDK
- Знания в базе, агент сам решает что вспоминать
- **Плюсы:** масштабируется, не требует ручной категоризации
- **Минусы:** непредсказуемо, может пропустить критическое правило
- **R2 уточнение:** Augment Context Engine (400K+ файлов) — самый зрелый вариант, но проприетарный

### Паттерн C: "Memory Bank + Auto-Compact"
**Используют:** Cline, Kilo Code
- Структурированные markdown файлы как persistent memory
- **Плюсы:** переживает сессии, адаптивно
- **Минусы:** Cline — потеря деталей при сжатии; Kilo Code — ручное заполнение
- **R2 уточнение:** Kilo Code (форк Roo Code) добавил Orchestrator mode для координации агентов

### Паттерн D: "RAG-based memory"
**Используют:** CrewAI, LangGraph
- Vector DB для хранения и retrieval
- **Плюсы:** масштабируется на миллионы фактов
- **Минусы:** инфраструктурный overhead, latency, не подходит CLI
- **R2 уточнение:** LangMem tools теперь интегрируются с CrewAI — конвергенция фреймворков

### Паттерн E: "Hook-based capture + compress"
**Используют:** claude-mem (НОВЫЙ паттерн, не в R1)
- Перехват tool invocations через hooks → semantic compression → progressive injection
- **Плюсы:** полностью автоматический, ~10x сжатие, начинает с нуля
- **Минусы:** extra инфраструктура (SQLite+Chroma), не стандартизирован
- **Релевантность для Ralph:** Ralph уже использует hooks (.claude/hooks/), паттерн capture→compress→inject потенциально применим

### Паттерн F: "Self-evolving scaffold"
**Используют:** Live-SWE-agent (НОВЫЙ паттерн, не в R1)
- Агент модифицирует собственный код/инструменты на лету
- **Плюсы:** адаптивность, 79.2% SWE-bench
- **Минусы:** академический, непредсказуемость, сложность отладки
- **Релевантность для Ralph:** пока не применим, но показывает направление эволюции

---

## 4. Ключевые паттерны и тренды

### Конвергенция к файловым системам
Все инструменты 2025-2026 используют markdown-файлы в репозитории (CLAUDE.md, AGENTS.md, .cursorrules, .clinerules, .devin.md). Это де-факто стандарт. Преимущества: версионирование, human-readable, портабельность.

### Двухуровневая архитектура
Лидеры разделяют:
- **Статические инструкции** (CLAUDE.md, AGENTS.md) — пишет человек, редко меняются
- **Динамическая память** (MEMORY.md, Memories) — пишет агент, часто обновляется

### Scope-based loading
Тренд 2025-2026: загружать только релевантные знания. Реализации:
- Glob-паттерны (Claude Code, Cursor)
- Directory hierarchy (Codex)
- Semantic search (Augment, CrewAI)
- Progressive disclosure (claude-mem, Codex Skills)

### Автоматизация vs ручная курация
Спектр: Aider (100% ручной) → Cline (ручной с структурой) → Claude Code (авто + ручной) → Augment (100% авто). Тренд движется к автоматизации.

### Self-evolving agents
Live-SWE-agent показывает будущее: агенты, которые модифицируют собственные инструменты. Пока академический, но показывает направление.

---

## 5. Evidence from Project Research (R1/R2/R3)

Проект bmad-ralph провёл три глубоких исследования (82 источника суммарно), результаты которых критически важны для архитектурных решений. Ниже — ключевые данные, релевантные для конкурентного анализа и cold-start проблемы.

### 5.1. R1: Фундаментальные ограничения context (20 источников)

**Context rot — непреодолимый барьер для ВСЕХ инструментов:**
- 18 frontier моделей (Claude, GPT-4, Gemini, Qwen) показывают **30-50% degradation** при полном контексте vs компактном [R1-S5, Chroma Research]
- "Lost in the Middle": модели хуже attend к середине контекста на >30% [R1-S5]
- **~150-200 инструкций** — практический предел для consistent following [R1-S14]
- **Counterintuitive:** shuffled/atomized контекст **лучше** organized narrative [R1-S5]

**Импликации для конкурентов:**

| Конкурент | Как подвержен context rot | Решение |
|---|---|---|
| Cline Memory Bank (6 файлов, все загружаются) | Сильно — все файлы в контекст | Auto-Compact как workaround |
| Aider (CONVENTIONS.md целиком) | Сильно — фиксированный overhead | Нет решения |
| Cursor (.cursor/rules/) | Умеренно — glob-filtering снижает объём | Auto Attached + Agent Requested |
| Claude Code (.claude/rules/) | Умеренно — paths: frontmatter | Glob + 200-строк лимит MEMORY.md |
| Augment Code | Слабо — семантическая карта, не flat text | Context Engine обрабатывает 200K токенов |
| claude-mem | Слабо — ~10x сжатие + progressive disclosure | 3-step retrieval |

**Вывод для Ralph на новом проекте:** knowledge budget жёсткий с первого дня. Даже 0 накопленных знаний + code context уже конкурируют за attention. Ralph должен быть агрессивен в сжатии и селективен в подаче с самого начала.

### 5.2. R2: Тройной барьер compliance (40 источников)

**Даже присутствие правила в контексте НЕ гарантирует его выполнение:**

R2 документирует case study из Epic 3 bmad-ralph:
- "Stale doc comments" — правило присутствовало в контексте в **4 из 5 случаев нарушения**
- "Assertion quality" — нарушено в **11/11 stories** Epic 3, несмотря на rules file

**Тройной барьер:**
1. **Compaction** уничтожает правила из CLAUDE.md (GitHub Issues #9796, #21925, #25265) [R2-S21-S23]
2. **Context rot** снижает внимание к правилам на 30-50% [R2-S5]
3. **Volume ceiling**: **~15 правил = 94% compliance**; 125+ правил = ~40-50% compliance [R2-S25, SFEIR study]

**Количественные данные enforcement механизмов:**

| Механизм | Гарантия compliance | Источник |
|---|---|---|
| CLAUDE.md правило | ~70-80% | [R2-S24] |
| .claude/rules/ правило | ~40-60% | [R2-S25] |
| SessionStart hook | ~90-94% | [R2-S25] |
| PreToolUse hook | ~100% | [R2-S13] |
| PostToolUse auto-fix | 100% (deterministic) | [R2-S13] |
| Explicit checklist prompt | ~95% | [R2-S34, S35] |

**Как конкуренты решают enforcement:**

| Конкурент | Enforcement механизм | Compliance estimate |
|---|---|---|
| Claude Code (native) | CLAUDE.md + rules + hooks | ~70-94% (зависит от механизма) |
| Cursor | Rules activation modes (Always/Intelligent/Glob/Manual) | ~70-85% |
| Copilot | instructions.md + AGENTS.md (passive) | ~60-80% |
| Devin | Knowledge auto-recall (semantic) | Непредсказуем |
| CodeRabbit | Persistent knowledge graph + feedback loop | Высокий (continuous learning) |
| Cline | .clinerules + toggle (passive) | ~50-70% |

**Ключевой finding для cold-start:** DGM (Darwin Godel Machine) показал: **хранение конкретных ошибок** (не абстрактных правил) повысило SWE-bench с 20% до 50% [R2-S37]. На новом проекте Ralph должен начинать с конкретных findings, не с абстрактных rules.

### 5.3. R3: RAG не нужен при <500 записях (22 источника)

**File-based подход побеждает sophisticated memory tools:**

| Подход | Benchmark (LoCoMo) | Источник |
|---|---|---|
| Letta filesystem agent | **74.0%** | [R3-S1] |
| Mem0 Graph | 68.5% | [R3-S1] |

**Сравнительная оценка подходов (weighted scoring):**

| Подход | Оценка (из 10) | Лучший при |
|---|---|---|
| File injection | **8.7** | <500 записей |
| RAG (chromem-go) | 7.8 | 500-100K записей |
| MCP Memory | 7.3 | 10K записей |
| Graph RAG | 4.8 | >2K записей |
| MemGPT-style | 4.6 | Enterprise |

**Импликации для конкурентов:**
- CrewAI/LangGraph с RAG — **overkill** для масштаба типичного проекта (<500 правил)
- Augment Code Context Engine — эффективен за счёт проприетарной оптимизации, но привязан к платформе
- **File-based** (Claude Code, Cursor, Copilot, Ralph) — оптимален для текущего масштаба

**Timeline для cold-start на новом проекте:**

| Фаза проекта | Ожидаемый объём знаний | Оптимальный подход |
|---|---|---|
| Первые 5 сессий | 10-30 записей | File injection (единственный разумный) |
| 1-3 месяца | 50-200 записей | File injection + distillation |
| 3-6 месяцев | 200-500 записей | File injection + selective loading |
| 6-12 месяцев | 500+ записей | Hybrid (file + MCP/RAG) |

### 5.4. Синтез: что значит R1/R2/R3 для cold-start Ralph

**Принципы, подтверждённые 82 источниками:**

1. **Budget is not optional** — даже на пустом проекте. Context rot начинается с первого токена знаний. Ralph должен считать каждый токен knowledge с первой сессии.

2. **Concrete > Abstract** — DGM: конкретные ошибки дают 2.5x improvement vs абстрактные правила [R2-S37]. На новом проекте: сохранять "в файле X строка Y была ошибка Z", не "всегда проверяйте Y".

3. **Atomized facts > Narrative** — shuffled atomized content outperforms organized narrative [R1-S5]. Формат: `## category [source]\nОдна строка факта.\n`

4. **15 правил = sweet spot** — для SessionStart/critical rules. Больше → compliance падает [R2-S25]. На новом проекте: первые 15 слотов — самые ценные, заполнять осторожно.

5. **Hook enforcement > passive rules** — 90-94% vs 40-60% [R2-S25]. На новом проекте: если правило важно, оно идёт в hook, не в rules file.

6. **File-based побеждает до 500 записей** — validated by Letta benchmark [R3-S1]. На новом проекте: НЕ начинать с RAG/MCP, начинать с файлов.

7. **Citation validation** — Copilot: 7% PR merge improvement, self-healing через проверку citations [R1-S3]. Каждое знание должно содержать file:line reference для проверки актуальности.

---

## 6. Выводы и рекомендации для Ralph

### 6.1. Что Ralph уже делает хорошо (относительно конкурентов)

| Фича Ralph | Аналог у конкурентов | Статус |
|---|---|---|
| CLAUDE.md + .claude/rules/ с glob | Cursor .mdc с glob, Codex directory walk | На уровне лидеров |
| MEMORY.md auto memory + topic-файлы | Augment Memories, Cursor Memories | На уровне лидеров |
| Разделение инструкций/памяти | Claude Code native | На уровне лидеров |
| Извлечение знаний из code-review | CodeRabbit (авто), Ralph (полуавто протокол) | Уникальная ниша |

### 6.2. Что Ralph может улучшить

| Возможность | Лучший пример | Сложность для Ralph |
|---|---|---|
| **Автоматическое извлечение из code-review** — Ralph находит ошибки в review, должен автоматически сохранять паттерн | CodeRabbit (авто из PR), claude-mem (авто из сессий) | Средняя — уже есть протокол в CLAUDE.md, нужна автоматизация |
| **Cold start на новом проекте** — анализ codebase → начальные rules | Claude Code /init, Augment Code indexing | Средняя — нужен scan codebase |
| **Дистилляция при росте** — [needs-formatting] tag, объединение дублей | claude-mem (~10x сжатие), CrewAI compaction | Высокая — нужна LLM-обработка |
| **Progressive disclosure** — загружать детальные rules по запросу | claude-mem (3-step), Codex Skills (metadata first) | Низкая — уже есть glob, нужны topic-files по запросу |
| **Feedback loop** — подтверждение/отклонение извлечённых знаний | CodeRabbit (обучение из feedback), Cursor (Apply Intelligently) | Средняя |

### 6.3. Уникальные преимущества Ralph

1. **Code-review как источник знаний** — ни один конкурент (кроме CodeRabbit) не извлекает знания из adversarial code review. Ralph делает это структурированно (findings → rules → violation-tracker).

2. **Violation tracking с escalation** — уникальная фича. Ни один конкурент не отслеживает частоту нарушений и не эскалирует правила.

3. **Multi-epic knowledge accumulation** — Ralph накопил 122 паттерна за 5 эпиков. Это демонстрирует жизнеспособность подхода на практике.

### 6.4. Архитектурные рекомендации

1. **Формат знаний** — оставить markdown в .claude/rules/ (индустриальный стандарт)
2. **Категоризация** — текущая структура (7 topic-файлов + index) превосходит все открытые конкуренты
3. **Автоматическое извлечение** — приоритет Epic 6: автоматизировать extraction из code-review findings
4. **Дистилляция** — запланировать в Epic 6 Story 6.3: периодическое сжатие + удаление устаревших паттернов
5. **Cold start** — добавить в roadmap: scan нового проекта → generate initial rules
6. **Контекстная подача** — уже реализовано через glob; потенциал: "agent-requested" активация по описанию

---

## 7. Источники

- [Cursor Rules Documentation](https://cursor.com/docs/context/rules)
- [Cline Memory Bank](https://docs.cline.bot/features/memory-bank)
- [Cline Memory Bank Blog](https://cline.bot/blog/memory-bank-how-to-make-cline-an-ai-agent-that-never-forgets)
- [Devin Playbooks](https://docs.devin.ai/product-guides/creating-playbooks)
- [Devin 2025 Performance Review](https://cognition.ai/blog/devin-annual-performance-review-2025)
- [DeepWiki](https://cognition.ai/blog/deepwiki)
- [Aider Conventions](https://aider.chat/docs/usage/conventions.html)
- [GitHub Copilot AGENTS.md Best Practices](https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/)
- [OpenAI Codex AGENTS.md](https://developers.openai.com/codex/guides/agents-md/)
- [Codex Skills](https://developers.openai.com/codex/skills/)
- [Claude Code Memory](https://code.claude.com/docs/en/memory)
- [claude-mem GitHub](https://github.com/thedotmack/claude-mem)
- [Kilo Code Memory Bank](https://blog.kilo.ai/p/how-memory-bank-changes-everything)
- [Roo Code Memory Bank](https://github.com/GreatScottyMac/roo-code-memory-bank)
- [Augment Code](https://www.augmentcode.com/)
- [Live-SWE-agent Paper](https://arxiv.org/abs/2511.13646)
- [AI Agent Memory: LangGraph, CrewAI, AutoGen](https://dev.to/foxgem/ai-agent-memory-a-comparative-analysis-of-langgraph-crewai-and-autogen-31dp)
- [LangMem + CrewAI](https://langchain-ai.github.io/langmem/guides/use_tools_in_crewai/)
- [Cursor Self-Improving Rules](https://www.sashido.io/en/blog/cursor-self-improving-rules)
- [CodeRabbit AI Code Review](https://www.coderabbit.ai/)
- [Windsurf Context Guide](https://www.kzsoftworks.com/blog/how-to-give-windsurf-the-right-context-for-smarter-ai-coding)
- [Continue.dev Codebase Awareness](https://docs.continue.dev/guides/codebase-documentation-awareness)
- [Complete Guide to AI Agent Memory Files](https://hackernoon.com/the-complete-guide-to-ai-agent-memory-files-claudemd-agentsmd-and-beyond)
- [AGENTS.md Standard](https://agents.md/)
- [GitHub Copilot Custom Instructions](https://docs.github.com/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot)
- [Copilot Agent-Specific Instructions](https://github.blog/changelog/2025-11-12-copilot-code-review-and-coding-agent-now-support-agent-specific-instructions/)
- [Continue.dev Rules](https://docs.continue.dev/customize/deep-dives/rules)
- [CrewAI Memory](https://docs.crewai.com/en/concepts/memory)
- [Cursor Self-Improving Rules](https://forum.cursor.com/t/how-to-force-your-cursor-ai-agent-to-always-follow-your-rules-using-auto-rule-generation-techniques/80199)
- [Devin 2025 Annual Performance Review](https://cognition.ai/blog/devin-annual-performance-review-2025)
- [Augment Code Context Engine](https://blog.codacy.com/ai-giants-how-augment-code-solved-the-large-codebase-problem)
- [Mem0 + Claude Code](https://mem0.ai/blog/claude-code-memory)
- [Qodo AI Code Review](https://www.qodo.ai/blog/ai-code-review-agents/)
