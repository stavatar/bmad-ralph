# Исследование: Gastown и фреймворки оркестрации агентов

**Дата:** 2026-03-07
**Цель:** Детальный анализ Gastown и связанных подходов к оркестрации AI-агентов для разработки ПО

---

## 1. Gastown (Gas Town)

**Автор:** Steve Yegge
**GitHub:** [steveyegge/gastown](https://github.com/steveyegge/gastown)
**Язык:** Go
**Stars:** ~11k (март 2026)
**Статус:** Экспериментальный, не consumer-ready
**Релиз:** 1 января 2026

### 1.1 Что это

Gas Town — система оркестрации множества Claude Code агентов с persistent work tracking.
Координирует 20-30 параллельных Claude Code инстансов, работающих над одной кодовой базой.
Построена на метафоре "города" с мэром, рабочими и наблюдателями.

Steve Yegge создал систему за вторую половину 2025 года — 75k строк, 2000 коммитов,
по его собственному признанию "100% vibecoded" за ~17 дней активной разработки.

### 1.2 Ключевые концепции

#### Beads (бусины) — persistent issue tracking
- Атомарные единицы работы, хранимые как JSON в Git рядом с кодом
- Каждый bead имеет ID (формат: `gt-abc12`), описание, статус, назначенного агента
- "Beads" и "issues" взаимозаменяемы — beads = данные, issues = отслеживаемая работа
- **Ключевая идея:** "AI-агенты эфемерны, но контекст работы должен быть постоянным"
- Решает проблему потери контекста при перезапуске сессий

#### Convoys (конвои) — группировка работ
- Convoy объединяет несколько beads, назначенных агентам
- `gt sling` — назначение работы конкретному агенту
- `gt convoy list` — мониторинг прогресса
- Используются даже для единичных задач

#### Agents (роли агентов)
- **The Mayor (мэр):** координатор, распределяет задачи, НЕ пишет код сам
- **Polecats (хорьки):** временные рабочие-исполнители, выполняют изолированные задачи, "умирают" после завершения
- **The Witness (свидетель):** агент-супервизор, мониторит Polecats, решает проблемы, подталкивает застрявших рабочих
- **The Refinery (нефтеперерабатывающий завод):** управляет merge queue, решает конфликты, может "переосмыслить" реализацию при несовместимых изменениях

#### Hooks (крючки)
- Git worktree-based persistent storage
- Переживают крэши агентов
- Каждый агент имеет "hook" указывающий на текущую работу, автоматически переключается на следующую задачу после завершения

#### Molecules (молекулы) и Formulas (формулы)
- Molecules оборачивают работу шаблонами оркестрации
- Formulas — TOML-определённые повторяемые воркфлоу
- Каждый вид работы (код, дизайн, UX) проходит через свой шаблон

#### Seancing (сеанс)
- Новые сессии могут "воскрешать" предыдущие как отдельные инстансы
- Позволяет спросить предшественников о незавершённой работе
- Сохраняет знания через перезапуски

### 1.3 Декомпозиция задач
- Mayor разбивает высокоуровневые фичи на атомарные задачи
- Назначает через индивидуальные очереди задач
- Suprevisor-агенты периодически "пингуют" рабочих (heartbeat), предотвращая зависания
- Dynamic re-assignment при мониторинге прогресса в реальном времени

### 1.4 Обработка context window limits
- Эфемерные сессии с persistent identities
- Хранение важных данных (идентичности агентов, задачи) в Git, а не в памяти агента
- Свободное убийство и перезапуск сессий с восстановлением из beads ledger
- Seancing для передачи знаний между сессиями

### 1.5 Re-planning / course correction
- Witness мониторит и подталкивает застрявших агентов
- Refinery может переосмыслить реализацию при конфликтах
- Эскалация к человеку когда автоматическое решение невозможно

### 1.6 Human-in-the-loop
- `--human` флаг для явного включения человеческого контроля
- Mayor работает как интерфейс между человеком и агентами
- TUI-дашборд (три панели: дерево агентов, панель конвоев, поток событий)

### 1.7 Стоимость и критика
- **Стоимость:** $2,000-$5,000/мес, ранний пользователь сообщил о $100/час
- **Критика (Maggie Appleton):**
  - "Количество перекрывающихся и ad hoc концепций ошеломляет"
  - Компоненты кажутся произвольными (polecats, convoys, molecules, deacons, witnesses, protomolecules)
  - Система подходит мозгу Yegge, но лишена когерентного UX для широкого применения
  - Vibecoding-подход жертвует связностью ради экспериментов
- **Фундаментальное наблюдение:** когда агенты берут на себя реализацию, дизайн становится узким местом — скорость ограничена человеческой способностью к планированию, а не скоростью кодирования

### 1.8 Установка
- Homebrew: `brew install gastown`
- npm: `npm install -g @gastown/gt`
- Go: `go install github.com/steveyegge/gastown/cmd/gt@latest`
- Docker Compose

---

## 2. Связанные проекты

### 2.1 Goosetown (Block/Square)

**GitHub:** [block/goosetown](https://github.com/block/goosetown)
**Основа:** надстройка над [Goose](https://github.com/block/goose) (~27k stars)
**Вдохновлён:** Gas Town (прямое признание в названии)

#### Архитектура
- Orchestrator-агент разбивает задачу на фазы: research, build, review
- Запускает параллельных delegates через extension `summon` / функцию `delegate()`
- **Town Wall** — append-only лог, где каждый агент постит что делает и что нашёл

#### Ключевые компоненты
- **Skills** — Markdown-файлы описывающие "как сделать X" (deploy, test и т.д.)
- **Subagents** — эфемерные инстансы с чистым контекстом, возвращают summary
- **Beads** — заимствованы из Gas Town, атомарные единицы работы
- **gtwall** — shared communication layer

#### Отличие от Gas Town
- Более лёгкий подход: каждый subagent работает в чистом контексте
- Основная сессия остаётся быстрой и фокусированной
- Менее сложная иерархия ролей

### 2.2 Claude Code Agent Teams (Anthropic)

**Документация:** [code.claude.com/docs/en/agent-teams](https://code.claude.com/docs/en/agent-teams)
**Статус:** Экспериментальный (требует `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`)
**Встроен в:** Claude Code

#### Архитектура
- **TeammateTool** — ядро оркестрации, 13 операций с определёнными схемами
- Team Lead (не кодит) планирует, делегирует, координирует
- Специализированные агенты: Frontend, Backend, Testing, Docs
- Каждый агент работает в независимом Git Worktree

#### Плюсы
- Нативная интеграция с Claude Code (без внешних зависимостей)
- Изоляция через worktrees предотвращает перезапись кода

### 2.3 Oh My Claude Code (OMC)

**GitHub:** [Yeachan-Heo/oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode)
**npm:** `oh-my-claude-sisyphus`
**Тип:** Плагин для Claude Code

#### Возможности
- 32 специализированных агента, 40+ skills
- 5 режимов исполнения:
  - **Autopilot** — автоматическое выполнение
  - **Ultrapilot** — максимальная автоматизация
  - **Swarm** — N агентов тянут из общего пула задач (как sprint backlog)
  - **Pipeline** — последовательный конвейер
  - **Ecomode** — параллельное исполнение с 30-50% экономией токенов (smart model routing между Haiku/Sonnet/Opus)

---

## 3. Универсальные фреймворки оркестрации

### 3.1 CrewAI

**GitHub:** [crewAIInc/crewAI](https://github.com/crewAIInc/crewAI) (~26k stars)
**Язык:** Python
**Статус:** Production-ready, используется PwC, IBM, NVIDIA
**Модель:** Role-based multi-agent

#### Декомпозиция задач
- Ролевая модель: каждый агент имеет expertise, цель, контекст
- Специальный planning agent создаёт step-by-step план для всех задач
- Поддержка sequential, parallel и conditional execution

#### Persistence
- Shared memory: short-term, long-term, entity и contextual memory
- Не git-backed (в отличие от Gas Town)

#### Re-planning
- Planning agent пересоздаёт план перед каждой итерацией
- Dynamic decision-making: conditional logic, event-driven workflows
- Агенты адаптируются к промежуточным результатам

#### Human-in-the-loop
- Поддерживается, но не центральная фича
- Фокус на автономности агентов

#### Плюсы
- Интуитивная абстракция для команд, мыслящих в терминах ролей
- 100+ готовых инструментов
- Независим от LangChain (переписан с нуля)
- 1.4B agentic automations в enterprise

#### Минусы
- Менее гибкий чем LangGraph для нестандартных workflow
- Не заточен под кодовые агенты (general-purpose)

### 3.2 LangGraph (LangChain)

**GitHub:** [langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) (~12k stars)
**Язык:** Python, JavaScript
**Версия:** 1.0.6 (март 2026)
**Статус:** Production-ready

#### Архитектура
- Граф-based: узлы = функции, рёбра = поток выполнения
- Stateful: состояние сохраняется между шагами через checkpointing
- Compile-time валидация checkpointer type

#### Декомпозиция задач
- DAG-based workflows с условным ветвлением и циклами
- Узлы графа = этапы обработки, рёбра = логика маршрутизации

#### Persistence
- Checkpointing: `AsyncPostgresSaver` для production
- Durable execution: агент может упасть и возобновиться с точного места
- Short-term (thread-scoped) и long-term memory

#### Context window management
- Автоматическое сохранение в memory при приближении к лимиту (~200k tokens)
- Разделение short-term и long-term memory

#### Re-planning
- Conditional branching и loops нативно
- Агент может вернуться к предыдущим шагам на основе промежуточных результатов

#### Human-in-the-loop
- `interrupt()` функция для паузы на предопределённых checkpoint'ах
- `Command(resume=...)` для продолжения после одобрения
- Первоклассная поддержка HITL-паттернов

#### Плюсы
- 30-40% ниже latency чем альтернативы в сложных workflow
- Самый гибкий из всех фреймворков (граф = полный контроль)
- Durable execution для long-running процессов

#### Минусы
- Выше порог входа чем CrewAI (более низкоуровневый)
- Зависимость от LangChain экосистемы

### 3.3 Microsoft Agent Framework (AutoGen + Semantic Kernel)

**GitHub:** [microsoft/autogen](https://github.com/microsoft/autogen) (~40k stars)
**Статус:** AutoGen v0.4 → миграция в Microsoft Agent Framework (GA Q1 2026)

#### Эволюция
- AutoGen v0.4: полная переработка — async, event-driven архитектура
- Semantic Kernel: enterprise-ready, production-grade
- **Слияние:** Microsoft Agent Framework объединяет оба в единый SDK

#### Декомпозиция задач
- Graph-based API для multi-step, multi-agent workflows
- Conversation patterns для координации агентов
- Thread management для параллельных разговоров

#### Persistence
- Conversation thread management через middleware
- State persistence через checkpointing

#### Re-planning
- Event-driven архитектура позволяет реагировать на изменения
- Middleware pipeline для обработки промежуточных результатов

#### Human-in-the-loop
- `UserProxyAgent` — агент-представитель человека в цепочке
- Человек может одобрять/отклонять действия агентов

#### Плюсы
- Microsoft backing, enterprise focus
- Миграция к единому стандарту (Agent Framework 1.0)
- Observability и diagnostics из коробки

#### Минусы
- Перманентная миграция: AutoGen → Agent Framework (API breaking changes)
- Более сложный чем CrewAI

### 3.4 Agency Swarm (VRSEN)

**GitHub:** [VRSEN/agency-swarm](https://github.com/VRSEN/agency-swarm) (~4k stars)
**Язык:** Python
**Основа:** OpenAI Agents SDK + Responses API

#### Подход
- Организационная метафора: агентства с определёнными ролями
- Каждый агент имеет tailored instructions, tools, capabilities
- Полный контроль над guiding prompts

#### Multi-LLM
- OpenAI нативно (GPT-5, GPT-4o)
- Через LiteLLM: Anthropic, Google, Grok, Azure, OpenRouter

#### Минусы
- Привязка к OpenAI Agents SDK
- Менее масштабный чем CrewAI/LangGraph

### 3.5 PraisonAI

**GitHub:** [MervinPraison/PraisonAI](https://github.com/MervinPraison/PraisonAI) (~5.5k stars)
**Язык:** Python
**Статус:** Production-ready

#### Подход
- Low-code/no-code multi-agent framework
- Интегрирует PraisonAI Agents, AG2 (AutoGen) и CrewAI
- Self-reflection (саморефлексия агентов)
- A2A Protocol для agent-to-agent communication

#### Плюсы
- 100+ поддерживаемых LLM
- Низкий порог входа
- Агрегация лучших фич из CrewAI и AutoGen

#### Минусы
- Менее зрелый чем CrewAI/LangGraph
- Meta-framework (зависит от других фреймворков)

### 3.6 TaskWeaver (Microsoft Research)

**GitHub:** [microsoft/TaskWeaver](https://github.com/microsoft/TaskWeaver) (~5.3k stars)
**Язык:** Python
**Фокус:** Data analytics

#### Подход
- Code-first: преобразует запросы пользователя в Python-программы
- Stateful: сохраняет историю чата И историю выполнения кода (включая in-memory данные)
- Plugins как функции для координации задач

#### Отличие
- Заточен под аналитику данных, НЕ под кодовые агенты
- Работа с rich data structures (DataFrames, словари)
- Sandbox execution для безопасности

### 3.7 OpenAI Swarm (Agents SDK)

**GitHub:** [openai/swarm](https://github.com/openai/swarm) (~20k stars)
**Статус:** Educational → production (через Agents SDK)

#### Подход
- Минималистичный: агент за 20 строк кода
- Handoff patterns для маршрутизации между агентами
- Guardrails для валидации inputs/outputs
- Привязан только к OpenAI моделям

---

## 4. Сравнительная таблица

| Критерий | Gas Town | CrewAI | LangGraph | MS Agent Framework | Claude Code Teams |
|---|---|---|---|---|---|
| **Фокус** | Кодовые агенты | General-purpose | General-purpose | Enterprise | Кодовые агенты |
| **Язык** | Go | Python | Python/JS | Python/.NET | TypeScript |
| **Stars** | ~11k | ~26k | ~12k | ~40k | встроен |
| **Multi-agent** | 20-30 параллельных | Роли + crews | Граф-узлы | Conversations | TeammateTool |
| **Persistence** | Git-backed beads | Shared memory | Checkpointing (Postgres) | Thread management | Git Worktrees |
| **Декомпозиция** | Mayor → atomic tasks | Planning agent | DAG nodes | Graph-based API | Team Lead → delegates |
| **Re-planning** | Witness + Refinery | Planning per iteration | Conditional loops | Event-driven | N/A |
| **HITL** | --human flag, TUI | Базовая | interrupt() + Command | UserProxyAgent | Approve/reject |
| **Context limits** | Ephemeral sessions + seancing | Memory tiers | Checkpoint + memory | Middleware | Worktree isolation |
| **Стоимость** | $2-5k/мес | Зависит от LLM | Зависит от LLM | Зависит от LLM | API costs |
| **Зрелость** | Experimental | Production | Production | GA Q1 2026 | Experimental |

---

## 5. Паттерны оркестрации

### 5.1 Sequential Pipeline
- **Кто использует:** CrewAI (sequential mode), OMC (Pipeline mode), bmad-ralph
- **Плюсы:** предсказуемость, простота отладки, линейный поток данных
- **Минусы:** нет параллелизма, блокировка на медленных шагах

### 5.2 DAG (Directed Acyclic Graph)
- **Кто использует:** LangGraph, MS Agent Framework
- **Плюсы:** гибкость, условное ветвление, параллелизм зависимых задач
- **Минусы:** сложность проектирования, debug графов нетривиален

### 5.3 Event-Driven
- **Кто использует:** AutoGen v0.4, MS Agent Framework
- **Плюсы:** реактивность, loose coupling между агентами
- **Минусы:** сложно предсказать поведение, order of events

### 5.4 Hierarchical Supervision
- **Кто использует:** Gas Town (Mayor → Witness → Polecats), Claude Code Teams (Lead → specialists)
- **Плюсы:** масштабируемость, чёткая цепочка команд
- **Минусы:** bottleneck на уровне Mayor/Lead, стоимость supervisor-агентов

### 5.5 Swarm / Task Pool
- **Кто использует:** OMC (Swarm mode), OpenAI Swarm, Gas Town (convoy-based)
- **Плюсы:** автоматический load balancing, agents claim tasks
- **Минусы:** координация доступа к shared ресурсам, merge conflicts

### 5.6 Task persistence подходы
| Подход | Реализация | Плюсы | Минусы |
|---|---|---|---|
| Git-backed JSON | Gas Town beads | Версионирование, persistent | Merge conflicts |
| Database checkpoints | LangGraph + Postgres | Надёжно, queryable | Требует инфраструктуру |
| In-memory + memory tiers | CrewAI | Простота | Потеря при крэше |
| File-based state | bmad-ralph | Простота, git-trackable | Нет rich querying |
| Worktree isolation | Claude Code Teams | Git-native изоляция | Сложный merge |

---

## 6. Релевантность для bmad-ralph

### 6.1 Что bmad-ralph уже делает хорошо
- Sequential pipeline (creator → validator → developer → reviewer)
- File-based task persistence (sprint-status.yaml, story files)
- Human-in-the-loop через tmux-based agent coordination
- Knowledge extraction и persistent rules (.claude/rules/)
- Context window observability (Epic 10)

### 6.2 Идеи из Gas Town
- **Beads-подобная система:** атомарные единицы работы с persistent ID — bmad-ralph уже использует story files, но без формализованного ID-tracking
- **Seancing:** восстановление контекста предыдущих сессий — аналог knowledge distillation в Epic 6
- **Witness pattern:** автоматический мониторинг + nudging — bmad-ralph не имеет supervisor-агента
- **Refinery pattern:** автоматическое разрешение merge-конфликтов — потенциально полезно

### 6.3 Идеи из других фреймворков
- **LangGraph checkpointing:** durable execution с точным восстановлением — bmad-ralph теряет состояние при крэше mid-task
- **CrewAI planning agent:** автоматическое планирование перед каждой итерацией — bmad-ralph планирует вручную
- **OMC Ecomode:** smart model routing для экономии — bmad-ralph использует одну модель

### 6.4 Чего НЕ стоит заимствовать
- Сложность Gas Town (polecats, molecules, protomolecules) — избыточная терминология
- 20-30 параллельных агентов — стоимость $100/час неоправдана для большинства проектов
- Vibecoding-подход к оркестрации — bmad-ralph ценит предсказуемость и тестируемость
- Привязка к одному LLM-провайдеру (Gas Town = Claude only, Agency Swarm = OpenAI only)

---

## 7. Выводы

1. **Gas Town** — амбициозный эксперимент Steve Yegge, показывающий _возможности_ массовой параллелизации кодовых агентов, но с серьёзными проблемами UX, стоимости и архитектурной когерентности. Ценность — в идеях (beads, seancing, hierarchical supervision), а не в прямом заимствовании кода.

2. **Тренд 2026:** формируются два лагеря:
   - **General-purpose frameworks** (CrewAI, LangGraph, MS Agent Framework) — широкий охват, не заточены под код
   - **Code-specific orchestrators** (Gas Town, Claude Code Teams, Goosetown, OMC) — оптимизированы для кодовых workflow

3. **Persistence — ключевой differentiator:** все серьёзные фреймворки решают проблему контекстной потери, но разными способами (git-backed, database, memory tiers). bmad-ralph's file-based approach (story files + sprint-status.yaml) находится между простотой и функциональностью.

4. **Human-in-the-loop остаётся незаменимым:** даже Gas Town с 30 агентами признаёт, что дизайн, архитектура и domain knowledge остаются человеческими задачами. bmad-ralph's подход (явные gates + review pipeline) хорошо масштабируется.

5. **Стоимость параллелизации:** Gas Town демонстрирует, что massive parallelism (20-30 агентов) создаёт больше проблем чем решает — потерянная работа, повторные исправления, design rework. Sequential pipeline bmad-ralph более экономичен и предсказуем.

---

## Источники

- [Gas Town GitHub](https://github.com/steveyegge/gastown)
- [Gas Town's Agent Patterns, Design Bottlenecks — Maggie Appleton](https://maggieappleton.com/gastown)
- [Welcome to Gas Town — Steve Yegge (Medium)](https://steve-yegge.medium.com/welcome-to-gas-town-4f25ee16dd04)
- [Gas Town Explained: Goosetown — Block/Goose](https://block.github.io/goose/blog/2026/02/19/gastown-explained-goosetown/)
- [CrewAI GitHub](https://github.com/crewAIInc/crewAI)
- [LangGraph GitHub](https://github.com/langchain-ai/langgraph)
- [Microsoft Agent Framework Overview](https://learn.microsoft.com/en-us/agent-framework/overview/)
- [AutoGen GitHub](https://github.com/microsoft/autogen)
- [Claude Code Agent Teams Docs](https://code.claude.com/docs/en/agent-teams)
- [Oh My Claude Code GitHub](https://github.com/Yeachan-Heo/oh-my-claudecode)
- [Agency Swarm GitHub](https://github.com/VRSEN/agency-swarm)
- [PraisonAI GitHub](https://github.com/MervinPraison/PraisonAI)
- [TaskWeaver GitHub](https://github.com/microsoft/TaskWeaver)
- [OpenAI Swarm GitHub](https://github.com/openai/swarm)
- [What Is Gastown — TWiT.TV](https://twit.tv/posts/tech/what-gastown-how-steve-yegges-ai-coding-agents-are-changing-software-development)
- [LangGraph vs CrewAI vs AutoGen Guide 2026](https://dev.to/pockit_tools/langgraph-vs-crewai-vs-autogen-the-complete-multi-agent-ai-orchestration-guide-for-2026-2d63)
- [Claude Code Swarms — Addy Osmani](https://addyosmani.com/blog/claude-code-agent-teams/)
