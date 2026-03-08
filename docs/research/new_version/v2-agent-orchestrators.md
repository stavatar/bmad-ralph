# Agent Orchestrator Patterns: Декомпозиция задач в AI-инструментах разработки

Исследование: 2026-03-07
Методология: deep web research по 11 инструментам/подходам

---

## Обзор 11 инструментов (сводная таблица)

| Инструмент | Входные данные | Промежуточные документы | Persistent task file | Single/Multi-agent | Upfront/Incremental |
|---|---|---|---|---|---|
| **Devin** | Plain text / Slack / issue | Plan (editable), notes.txt, knowledge entries | notes.txt + knowledge entries | Multi-agent (Devin 2.0) | Upfront plan, итеративная доработка |
| **SWE-Agent** | GitHub issue | Нет (прямая работа через ACI) | Нет | Single-agent | Incremental (ReAct loop) |
| **OpenHands** | Issue / plain text | Event stream (chronological log) | Event stream (replay-capable) | Multi-agent (AgentHub) | Incremental с делегированием |
| **AutoCodeRover** | GitHub issue | AST-контекст, fault localization | Нет | Single-agent | Двухфазный (context retrieval + patch) |
| **MetaGPT** | Однострочное требование | PRD, design docs, API specs, user stories | Структурированные артефакты по SOP | Multi-agent (5+ ролей) | Upfront (полный SOP pipeline) |
| **Gastown** | Задача от Mayor | Beads (git-backed issues) | Beads в Git | Multi-agent (20-30 параллельных) | Upfront декомпозиция Mayor'ом |
| **Claude Code** | Plain text / CLAUDE.md | Task list (TaskCreate), plan mode output | Нет (in-context tasks) | Single + sub-agents | Гибридный (plan mode + as-needed) |
| **Cursor Composer** | Natural language prompt | Diff preview, plan в sidebar | Нет | Multi-agent (до 8 параллельных) | Upfront plan, итеративный |
| **Windsurf Cascade** | Natural language prompt | Semantic model проекта | Нет | Single-agent | Upfront plan mode |
| **Copilot Workspace** | GitHub issue / prompt | Specification + Plan (оба editable) | Нет (session-only) | Single-agent (sub-agents внутри) | Upfront (spec -> plan -> impl) |
| **Kiro** | Natural language requirement | requirements.md, design.md, tasks.md | tasks.md (persistent, trackable) | Single-agent + hooks | Upfront (3-phase workflow) |

**Бонус: исследование ADaPT** -- академический фреймворк для as-needed decomposition (см. отдельный раздел).

---

## Детальный анализ каждого инструмента

### 1. Devin (Cognition)

**Входные данные:** Plain text через чат, Slack-интеграция, GitHub issues. Devin может анализировать codebase, находить зависимости, рисовать диаграммы архитектуры.

**Промежуточные артефакты:**
- **Interactive Plan** -- перед написанием кода Devin генерирует детальный план с кликабельными сниппетами. План полностью редактируемый: можно переставлять шаги, добавлять/удалять.
- **notes.txt** -- лог мыслей агента ("User wants X", "Hit issue Z, working on it"). Служит памятью для continuity.
- **Knowledge entries** -- структурированный persistent контекст ("this repo uses Tailwind", "API responds with XML").

**Persistent task file:** notes.txt и knowledge entries сохраняются между сессиями.

**Модель агентов:** С Devin 2.0 -- multi-agent. Один агент может диспатчить задачи другим агентам. Cloud-based IDE с параллельной работой.

**Планирование:** Upfront. Devin сначала исследует codebase, генерирует план, показывает его пользователю для одобрения, только потом пишет код. Но план итеративно доработывается на основе feedback'а.

**Ключевая особенность:** Devin Review -- специализированный review-режим, который анализирует PR и дает feedback с учетом контекста кодовой базы.

---

### 2. SWE-Agent (Princeton, NeurIPS 2024)

**Входные данные:** GitHub issue (natural language описание бага или feature request).

**Промежуточные артефакты:** Не создает. Работает напрямую через Agent-Computer Interface (ACI) -- набор команд для навигации, просмотра, редактирования и исполнения кода.

**Persistent task file:** Нет. Все состояние в контекстном окне LLM.

**Модель агентов:** Single-agent. Один LLM (GPT-4o или Claude) взаимодействует с компьютером через ACI.

**Планирование:** Incremental (ReAct loop). Агент не создает план заранее -- он последовательно анализирует issue, ищет релевантный код, локализует баг, пишет патч. Каждый шаг -- observation -> thought -> action.

**Ключевая особенность:** ACI design. Ключевой вклад SWE-Agent -- не в планировании, а в дизайне интерфейса взаимодействия агента с компьютером. Команды специально оптимизированы для LLM:
- Linter, блокирующий синтаксически некорректные правки
- File viewer с scrolling и search
- Directory search
- Все feedback в формате, удобном для LLM

**Производительность:** 12.29% на SWE-bench (полный тест-сет), ~19% на SWE-bench-lite (300 issues).

---

### 3. OpenHands (ex-OpenDevin, ICLR 2025)

**Входные данные:** Issue, plain text, задачи через API.

**Промежуточные артефакты:**
- **Event stream** -- хронологическая коллекция actions и observations. Ключевой архитектурный компонент.
- Event stream поддерживает **deterministic replay** -- можно воспроизвести всю сессию.

**Persistent task file:** Event stream сохраняется и может быть replayed.

**Модель агентов:** Multi-agent с иерархической делегацией:
- **CodeActAgent** -- генералист для кода и debugging'а
- **BrowserAgent** -- специалист по web-навигации
- **Micro-agents** -- легковесные агенты из natural language описания
- Агенты могут делегировать sub-tasks другим через built-in примитивы.

**Планирование:** Incremental с делегированием. Агент начинает работу и при необходимости спавнит sub-agents для специализированных задач.

**Ключевая особенность (V1, 2025):**
- Модульный SDK с четкими границами
- Event-sourced state model
- Typed tool system с MCP-интеграцией
- Workspace abstraction: один и тот же агент работает локально или в containerized environment

---

### 4. AutoCodeRover (NUS, ISSTA 2024)

**Входные данные:** GitHub issue (natural language описание).

**Промежуточные артефакты:**
- **AST-контекст** -- представление программы через abstract syntax tree, а не как коллекция файлов
- **Fault localization результаты** -- spectrum-based fault localization с использованием тестов

**Persistent task file:** Нет.

**Модель агентов:** Single-agent.

**Планирование:** Двухфазный процесс:
1. **Context Retrieval** -- стратифицированная стратегия: LLM анализирует описание issue, извлекает ключевые слова (файлы, классы, методы), использует code search API для построения контекста. Если тесты доступны, применяется fault localization.
2. **Patch Generation** -- на основе собранного контекста LLM генерирует патч.

**Ключевая особенность:** Работа на уровне AST, а не файлов. Это позволяет LLM лучше понимать структуру программы (классы, методы, их отношения). Средняя стоимость решения -- $0.43, среднее время -- 4 минуты (vs 2.68 дней у разработчиков).

---

### 5. MetaGPT (ICLR 2024)

**Входные данные:** Однострочное требование (natural language, one-line).

**Промежуточные артефакты (SOP pipeline):**
- **Competitive Analysis** -- анализ конкурентов
- **PRD (Product Requirements Document)** -- требования продукта
- **User Stories** -- пользовательские истории
- **Data Structures** -- структуры данных
- **API Specifications** -- спецификации API
- **Design Documents** -- архитектурные документы, flowcharts, interface specs

**Persistent task file:** Все артефакты структурированы и передаются между ролями по SOP.

**Модель агентов:** Multi-agent с фиксированными ролями:
- **Product Manager** -- генерирует PRD и user stories
- **Architect** -- создает архитектурный дизайн
- **Project Manager** -- декомпозирует на задачи
- **Engineer** -- пишет код
- (возможны дополнительные роли)

**Планирование:** Полностью upfront по SOP (Standard Operating Procedure). Каждая роль получает вход от предыдущей, генерирует структурированный выход, передает следующей. SOP определяет:
- Ответственности каждого участника
- Стандарты промежуточных выходов
- Порядок взаимодействия

**Ключевая особенность:** Структурированные промежуточные выходы значительно повышают success rate генерации кода. SOP снижает ambiguity и ошибки при коммуникации между агентами.

**Развитие (2025):** MGX (MetaGPT X) -- продукт для natural language programming. AFlow (ICLR 2025, oral, top 1.8%) -- автоматизация генерации agentic workflows.

---

### 6. Gastown (Steve Yegge)

**Входные данные:** High-level задача от пользователя к Mayor (через natural language).

**Промежуточные артефакты:**
- **Beads** -- git-backed issue tracking система. Каждый bead содержит ID (prefix + 5-char alphanumeric), описание задачи, статус, assignee.
- Все состояние (identities агентов, work assignments, orchestration state) сохраняется в Git.

**Persistent task file:** Beads в Git -- полностью persistent, version-controlled.

**Модель агентов:** Multi-agent (20-30 параллельных Claude Code инстансов):
- **Mayor** -- orchestrator, распределяет работу, имеет полный контекст workspace'а
- **Polecats** -- ephemeral workers, получают задачу, выполняют, исчезают. Каждый в своем Git worktree.
- **Witness + Deacon** -- мониторинг здоровья системы
- **Refinery** -- управление merge'ами

**Планирование:** Upfront декомпозиция Mayor'ом. Mayor получает high-level задачу, разбивает на beads, распределяет по Polecats. Каждый Polecat работает изолированно в своем worktree.

**Ключевая особенность:** "Kubernetes for AI coding agents". Координационная модель с ролями, async communication (mail system), validation gates вместо доверия. Все через Git -- нет отдельной базы данных для состояния.

**Статус:** Pre-v1, не для production. Но координационная модель работает и масштабируется.

---

### 7. Claude Code (Anthropic, native)

**Входные данные:** Plain text prompt, CLAUDE.md файл проекта, .claude/rules/*.md для контекстных правил.

**Промежуточные артефакты:**
- **Plan mode** -- агент генерирует план перед кодированием (отдельный режим)
- **Task system** -- TaskCreate, TaskUpdate, TaskList, с поддержкой зависимостей (blockedBy)
- Sub-agents для параллельного выполнения задач

**Persistent task file:** Нет стандартного. Задачи существуют in-context. Но пользователи создают свои persistent файлы (todo.md, tasks в CLAUDE.md).

**Модель агентов:** Single-agent с sub-agents. Основной агент может спавнить дочерние для параллельной работы. 40+ внутренних промптов, динамически компонуемых.

**Планирование:** Гибридный подход:
- **Plan mode** -- upfront анализ и планирование перед кодированием
- **As-needed** -- для простых задач агент работает без явного плана
- Рекомендуемый workflow: research phase (plan mode) -> review plan -> implement

**Ключевая особенность:** Минимальная инфраструктура. Вся "оркестрация" через промпты и CLAUDE.md. Нет отдельного task tracker, нет ролей -- один агент делает все. Но это масштабируется плохо для больших задач, отсюда появляются wrapper'ы (ralph, Gastown, ccpm).

---

### 8. Cursor Composer / Agent Mode

**Входные данные:** Natural language prompt в IDE.

**Промежуточные артефакты:**
- **Diff preview** -- перед применением показывает полный diff всех изменений
- **Plan в sidebar** -- агенты видны как объекты в editor, могут запускать multi-step "plans"

**Persistent task file:** Нет.

**Модель агентов:** Multi-agent (до 8 параллельных):
- Каждый агент в изолированной копии codebase (git worktrees или remote machines)
- Cursor 2.0: агенты как first-class объекты в IDE

**Планирование:** Upfront. Composer генерирует план, показывает diff для review перед apply. 25 tool calls -- checkpoint для review.

**Ключевая особенность:** Composer -- MoE модель, обученная через RL в live coding environments с доступом к полному tooling harness (semantic search, file editing, grep, terminal). Это не generic LLM с промптами, а специализированная модель для кодирования.

---

### 9. Windsurf Cascade

**Входные данные:** Natural language prompt в IDE.

**Промежуточные артефакты:**
- **Semantic model проекта** -- динамическая модель с tracking зависимостей и logic flows между файлами
- **Plan mode output** -- структурированный план перед написанием кода

**Persistent task file:** Нет.

**Модель агентов:** Single-agent. Cascade -- единый agentic engine.

**Планирование:** Upfront с Plan Mode. Cascade анализирует полный scope задачи, строит план, затем выполняет. Для сложных задач -- автоматически понимает cross-module requirements.

**Ключевая особенность:** Hybrid inference -- легковесные задачи на локальных моделях, сложные на cloud. Cascade строит semantic model, а не просто анализирует файлы.

---

### 10. GitHub Copilot Workspace

**Входные данные:** GitHub issue или natural language prompt.

**Промежуточные артефакты:**
- **Specification** -- два списка: "current state" и "desired state" codebase'а. Оба редактируемые.
- **Plan** -- список файлов для create/modify/delete + bullet-point действия для каждого файла. Редактируемый.
- **Implementation** -- дифы по каждому файлу.

**Persistent task file:** Нет (session-only).

**Модель агентов:** Single-agent (с внутренними sub-agents). Sunset в мае 2025, технология перешла в Copilot Coding Agent (GA с сентября 2025).

**Планирование:** Строго upfront, 3-phase:
1. **Specification** -- что есть сейчас vs что нужно
2. **Plan** -- какие файлы менять и как
3. **Implementation** -- генерация кода

Две точки steering'а: после spec и после plan. Все editable, все можно regenerate.

**Ключевая особенность:** Самый explicit spec->plan->impl pipeline среди всех инструментов. Каждый шаг видим и редактируем пользователем. Наследие перешло в Copilot Coding Agent.

---

### 11. Amazon Kiro

**Входные данные:** Natural language описание требований.

**Промежуточные артефакты (3-phase workflow):**
- **requirements.md** -- user stories с acceptance criteria в EARS (Easy Approach to Requirements Syntax) нотации
- **design.md** -- техническая архитектура, sequence diagrams, implementation considerations
- **tasks.md** -- implementation plan с discrete, trackable задачами, зависимостями, checkbox'ами

**Persistent task file:** tasks.md -- persistent, trackable, обновляется в real-time по мере выполнения.

**Модель агентов:** Single-agent + automation hooks (агенты, триггерящиеся на file save и другие события).

**Планирование:** Полностью upfront, 3-phase:
1. **Requirements** -- что строить (user stories, AC)
2. **Design** -- как строить (архитектура, диаграммы)
3. **Tasks** -- дискретные шаги реализации

**Ключевая особенность:** Спецификации синхронизируются с evolving codebase. Можно написать код и попросить Kiro обновить specs, или обновить specs и regenerate tasks. Двунаправленная синхронизация spec<->code.

**Формат tasks.md:**
```markdown
## Implementation Plan
- [ ] Task description
  - Implementation step 1
  - Implementation step 2
  - Refs: REQ-001, REQ-002
```

---

### Бонус: ADaPT (NAACL 2024, Allen AI)

**Суть исследования:** Сравнение upfront decomposition vs as-needed decomposition.

**Проблема upfront:** Plan-and-Execute подход создает фиксированный план без учета сложности каждого шага. Если шаг оказывается сложнее ожидаемого, план ломается.

**Решение ADaPT:**
- Separate Planner и Executor модули
- Executor пробует выполнить задачу
- Если fails -- Planner рекурсивно декомпозирует failed sub-task
- Декомпозиция происходит **только когда нужно** (as-needed)
- Рекурсивная структура адаптируется к сложности задачи

**Результаты:** +28.3% success rate в ALFWorld, +27% в WebShop, +33% в TextCraft по сравнению с upfront decomposition.

**Вывод:** As-needed decomposition превосходит upfront для задач с непредсказуемой сложностью отдельных шагов.

---

## Паттерны и антипаттерны

### Паттерны (что работает)

1. **Spec-first pipeline (Copilot Workspace, Kiro, MetaGPT)**
   - Явная спецификация "что есть" vs "что нужно" перед планированием
   - Промежуточные документы снижают ambiguity
   - Пользователь может steering на каждом этапе

2. **Git-backed state (Gastown, Kiro)**
   - Состояние задач в Git, а не в in-memory или отдельной DB
   - Version control для task state -- возможность rollback
   - Естественная интеграция с development workflow

3. **ACI-optimized tools (SWE-Agent)**
   - Дизайн интерфейса агента важнее, чем сложность планирования
   - Linter-gates на edit commands предотвращают синтаксические ошибки
   - Feedback format оптимизирован для LLM

4. **Role-based SOP (MetaGPT, Gastown)**
   - Фиксированные роли с определенными ответственностями
   - Стандарты для промежуточных выходов
   - Каждая роль работает только в своей зоне

5. **Editable intermediate artifacts (Copilot Workspace, Devin, Kiro)**
   - Все промежуточные документы редактируемые
   - Пользователь может корректировать на любом этапе
   - Двунаправленная коммуникация human<->agent

6. **Persistent knowledge (Devin, Kiro)**
   - Knowledge entries переживают сессии
   - Агент учится из предыдущих взаимодействий
   - Контекст проекта сохраняется

7. **As-needed decomposition (ADaPT, Claude Code)**
   - Не все задачи нужно декомпозировать заранее
   - Рекурсивная декомпозиция по failure
   - Адаптация к реальной сложности задачи

### Антипаттерны (что не работает)

1. **Overplanning** -- слишком детальный upfront план для простых задач. SWE-Agent показывает, что для single-issue fixes plan не нужен.

2. **Планы без steering points** -- если пользователь не может вмешаться между spec и impl, ошибки amplify'ятся.

3. **In-memory-only state** -- потеря контекста при перезапуске (Claude Code без persistent tasks).

4. **Trust without validation** -- Gastown учит: validation gates вместо слепого доверия агентам.

5. **Monolithic agent для сложных задач** -- один агент не масштабируется. MetaGPT и Gastown показывают преимущества разделения ролей.

6. **Fidelity loss между ролями** -- когда промежуточные артефакты не структурированы, информация теряется при передаче между агентами.

---

## Что ralph может заимствовать

### Высокий приоритет

1. **Persistent task file (Kiro pattern)**
   ralph уже имеет sprint-status.yaml и story файлы, но нет runtime task tracking. Kiro tasks.md -- простой markdown с checkbox'ами, обновляемый в real-time. ralph может создавать аналогичный файл при Execute и обновлять по мере прогресса.

2. **Spec-first workflow (Copilot Workspace pattern)**
   Формат "current state -> desired state" для каждой задачи. ralph может генерировать мини-spec перед каждой сессией Claude Code, давая агенту четкий контекст "что есть" и "что нужно".

3. **As-needed decomposition (ADaPT)**
   Вместо декомпозиции всех задач заранее -- пробовать выполнить task целиком, и только при failure (превышение итераций, context overflow) декомпозировать на sub-tasks. Это совместимо с текущим retry-механизмом ralph.

### Средний приоритет

4. **Knowledge entries (Devin pattern)**
   ralph уже имеет knowledge management (Epic 6), но knowledge entries Devin более targeted -- "this repo uses Tailwind", "API responds with XML". ralph может генерировать такие entries автоматически из результатов review.

5. **Git-backed task state (Gastown pattern)**
   Beads -- вдохновение для version-controlled task tracking. ralph может хранить task state в git-tracked файлах вместо in-memory. Это дает history, rollback, и persistence между сессиями.

6. **ACI improvements (SWE-Agent pattern)**
   Оптимизация prompt'ов и feedback format'ов для LLM. ralph уже делает это через prompt templates, но можно добавить linter-gates (проверки перед отправкой кода в Claude).

### Низкий приоритет (для будущих версий)

7. **Multi-agent с ролями (MetaGPT/Gastown pattern)**
   ralph уже использует multi-agent workflow через BMad team (creator, validator, developer, reviewer), но это manual orchestration. Автоматизация role dispatch -- следующий шаг.

8. **Bi-directional spec sync (Kiro pattern)**
   Синхронизация specs<->code. После review обновлять spec на основе реального кода. Сложная фича, но ценная для долгосрочных проектов.

---

## Ключевые выводы

1. **Тренд 2025-2026:** Движение от single-agent ReAct loops к structured multi-phase pipelines с persistent state.

2. **Spec-driven > prompt-driven:** Все наиболее успешные инструменты (Kiro, Copilot Workspace, MetaGPT) создают explicit промежуточные документы, а не полагаются на один промпт.

3. **Hybrid decomposition побеждает:** Ни чистый upfront, ни чистый incremental не оптимальны. Лучшие инструменты (Devin, Claude Code) используют upfront plan + as-needed refinement.

4. **Persistent state критичен:** Gastown (Beads в Git) и Kiro (tasks.md) показывают, что без persistent state масштабирование невозможно.

5. **Steering points обязательны:** Copilot Workspace с двумя explicit steering points (spec и plan) -- золотой стандарт human-in-the-loop.

6. **ralph уже на правильном пути:** BMad workflow с ролями (creator/validator/developer/reviewer) и sprint artifacts (yaml, story files) -- это по сути MetaGPT SOP, реализованный через manual orchestration. Следующий шаг -- автоматизация.

---

## Источники с URL

### Devin (Cognition)
- [Devin 2.0 Blog](https://cognition.ai/blog/devin-2)
- [Interactive Planning Docs](https://docs.devin.ai/work-with-devin/interactive-planning)
- [Devin Review: AI to Stop Slop](https://cognition.ai/blog/devin-review)
- [Devin Annual Performance Review 2025](https://cognition.ai/blog/devin-annual-performance-review-2025)
- [Devin 2.2](https://cognition.ai/blog/introducing-devin-2-2)

### SWE-Agent (Princeton)
- [GitHub: SWE-agent](https://github.com/SWE-agent/SWE-agent)
- [ACI Documentation](https://github.com/SWE-agent/SWE-agent/blob/main/docs/background/aci.md)
- [SWE-agent Documentation](https://swe-agent.com/)
- [NeurIPS 2024 Paper](https://collaborate.princeton.edu/en/publications/swe-agent-agent-computer-interfaces-enable-automated-software-eng/)

### OpenHands
- [OpenHands Platform](https://openhands.dev/)
- [ICLR 2025 Paper](https://openreview.net/pdf/95990590797cff8b93c33af989ecf4ac58bde9bb.pdf)
- [Agent SDK Paper](https://arxiv.org/html/2511.03690v1)
- [GitHub: OpenHands](https://github.com/OpenHands/OpenHands)
- [Architecture Analysis (MGX)](https://mgx.dev/insights/a-comprehensive-analysis-of-opendevin-openhands-architecture-development-use-cases-and-challenges/62fee7b52567490da851f0ed7cb2bf9f)

### AutoCodeRover
- [Paper (arXiv)](https://arxiv.org/abs/2404.05427)
- [ISSTA 2024](https://dl.acm.org/doi/10.1145/3650212.3680384)
- [NUS Feature Article](https://www.comp.nus.edu.sg/features/ai-in-sw-development-autocoderover/)

### MetaGPT
- [GitHub: MetaGPT](https://github.com/FoundationAgents/MetaGPT)
- [ICLR 2024 Paper](https://proceedings.iclr.cc/paper_files/paper/2024/file/6507b115562bb0a305f1958ccc87355a-Paper-Conference.pdf)
- [IBM Overview](https://www.ibm.com/think/topics/metagpt)
- [IBM PRD Automation Tutorial](https://www.ibm.com/think/tutorials/multi-agent-prd-ai-automation-metagpt-ollama-deepseek)

### Gastown
- [GitHub: Gastown](https://github.com/steveyegge/gastown)
- [Steve Yegge: Welcome to Gas Town](https://steve-yegge.medium.com/welcome-to-gas-town-4f25ee16dd04)
- [Maggie Appleton: Gas Town's Agent Patterns](https://maggieappleton.com/gastown)
- [Cloud Native Now: Kubernetes for AI Coding Agents](https://cloudnativenow.com/features/gas-town-what-kubernetes-for-ai-coding-agents-actually-looks-like/)
- [Software Engineering Daily Podcast](https://softwareengineeringdaily.com/2026/02/12/gas-town-beads-and-the-rise-of-agentic-development-with-steve-yegge/)
- [Two Kinds of Multi-Agent](https://paddo.dev/blog/gastown-two-kinds-of-multi-agent/)

### Claude Code
- [How Claude Code Works](https://code.claude.com/docs/en/how-claude-code-works)
- [Claude Code Tasks System](https://claudecode.jp/en/news/claude-code-tasks-system)
- [Plan Mode](https://codewithmukesh.com/blog/plan-mode-claude-code/)
- [Art of Planning with Claude Code](https://algarch.com/blog/the-art-of-planning-how-to-10x-your-productivity-with-claude-code)
- [Deep-Plan Plugin](https://pierce-lamb.medium.com/building-deep-plan-a-claude-code-plugin-for-comprehensive-planning-30e0921eb841)

### Cursor Composer
- [Cursor 2.0 Guide](https://www.digitalapplied.com/blog/cursor-2-0-agent-first-architecture-guide)
- [Cursor Features](https://cursor.com/features)
- [InfoQ: Composer Multi-Agent](https://www.infoq.com/news/2025/11/cursor-composer-multiagent/)
- [Agent Architectures Comparison](https://cuckoo.network/blog/2025/06/03/coding-agent)

### Windsurf Cascade
- [Windsurf vs Cursor Comparison](https://designrevision.com/blog/windsurf-vs-cursor)
- [Cascade vs Composer Side-by-Side](https://egghead.io/windsurf-cascade-vs-cursor-composer-agent-side-by-side-comparison~jbct2)

### GitHub Copilot Workspace
- [GitHub Next: Copilot Workspace](https://githubnext.com/projects/copilot-workspace)
- [User Manual](https://github.com/githubnext/copilot-workspace-user-manual)
- [Spec-Driven Workflow](https://github.com/github/awesome-copilot/blob/main/instructions/spec-driven-workflow-v1.instructions.md)
- [Copilot Workspace Guide](https://createaiagent.net/tools/github-copilot-workspace/)

### Amazon Kiro
- [Kiro.dev](https://kiro.dev/)
- [Specs Documentation](https://kiro.dev/docs/specs/)
- [Feature Specs](https://kiro.dev/docs/specs/feature-specs/)
- [From Chat to Specs Deep Dive](https://kiro.dev/blog/from-chat-to-specs-deep-dive/)
- [Martin Fowler: Spec-Driven Development Tools](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)
- [InfoQ: Kiro Spec-Driven Agent](https://www.infoq.com/news/2025/08/aws-kiro-spec-driven-agent/)

### ADaPT
- [Paper (NAACL 2024 Findings)](https://aclanthology.org/2024.findings-naacl.264/)
- [arXiv](https://arxiv.org/abs/2311.05772)
- [GitHub: ADaPT](https://github.com/archiki/ADaPT)
- [Allen AI Project Page](https://allenai.github.io/adaptllm/)
- [Prompt Engineering Analysis](https://promptengineering.org/adapt-dynamic-decomposition-and-planning-for-llms-in-complex-decision-making/)

### Дополнительные источники
- [Long-Running AI Agents and Task Decomposition (Zylos Research)](https://zylos.ai/research/2026-01-16-long-running-ai-agents)
- [How to Build Planning Into Your Agent (Arize AI)](https://arize.com/blog/how-to-build-planning-into-your-agent)
- [Context Engineering for Multi-Agent LLM Code Assistants](https://arxiv.org/html/2508.08322v1)
