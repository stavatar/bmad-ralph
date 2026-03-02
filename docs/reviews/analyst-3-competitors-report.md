# Анализ управления знаниями агентов у конкурентов

**Аналитик:** analyst-3
**Дата:** 2026-03-02
**Задача:** Как конкуренты хранят, категоризируют и подают извлечённые знания в контекст AI-агентов

---

## 1. Сравнительная таблица

| Инструмент | Формат хранения | Один файл / много | Динамическая фильтрация | Glob/path scoping | Авто-генерация знаний | Дистилляция/сжатие | Persistent memory |
|---|---|---|---|---|---|---|---|
| **Cursor** | `.cursor/rules/*.mdc` | Много (deprecated `.cursorrules`) | **Да** — YAML frontmatter: `alwaysApply`, `globs`, `description` | **Да** — glob в frontmatter | Агент может создавать `.mdc` файлы | Нет явной | Нет встроенной |
| **GitHub Copilot** | `.github/copilot-instructions.md` + `.github/instructions/*.instructions.md` | Много (основной + path-scoped) | **Да** — `applyTo` glob в frontmatter | **Да** — glob + `excludeAgent` | Нет | Нет | Нет |
| **Claude Code** | `CLAUDE.md` + `.claude/rules/*.md` | Много (иерархия: user/project/rules) | **Да** — `paths` в YAML frontmatter | **Да** — glob patterns | Auto-memory (`/home/.claude/`) | Нет явной | **Да** — auto memory |
| **Codex (OpenAI)** | `AGENTS.md` + `AGENTS.override.md` | Мало (иерархия директорий) | **Частично** — ближайший файл в дереве | **Нет** — directory proximity | Нет | Нет | Нет |
| **Aider** | `CONVENTIONS.md` + `.aider.conf.yml` | Несколько (read: [...]) | **Нет** — грузит всё | **Нет** | AiderDesk: skills в `.aider-desk/skills/` | Нет | AiderDesk: persistent memory |
| **Cline** | `.clinerules` файл или `.clinerules/` директория | Один или много | **Да** — toggleable rules (v3.13+) | **Нет** — ручной toggle | Агент может писать rules | **Да** — Auto-Compact | **Да** — Memory Bank |
| **Windsurf** | `.windsurfrules.md` + `global_rules.md` | Мало (global + project) | **Нет** — грузит всё | **Нет** | **Да** — авто-создание Memories | Нет явной | **Да** — Memories layer |
| **Devin** | Knowledge (UI), Playbooks | Много (через UI) | **Да** — авто-recall по релевантности | **Нет** — semantic recall | **Да** — DeepWiki авто-индексация | Нет явной (но summaries) | **Да** — Knowledge base |
| **Continue.dev** | `.continue/rules/*.md` + `.continuerules` | Много | **Да** — globs + regex в frontmatter | **Да** — glob + regex | Агент может создавать rules (`create_rule_block`) | Нет | Нет встроенной |
| **OpenHands** | SDK: Skills + Context injection | Программный API | **Да** — keyword/condition activation | **Да** — программная фильтрация | Нет | Нет | Через внешние store |
| **CrewAI** | Memory types (short/long-term) | Программный API | **Да** — RAG retrieval | **Нет** — semantic | **Да** — авто-извлечение | **Да** — ChromaDB RAG | **Да** — ChromaDB |
| **LangGraph** | InMemoryStore / external DB | Программный API | **Да** — tool-based retrieval | **Нет** — semantic | Через langmem tools | Нет явной | **Да** — persistent store |

---

## 2. Детальный анализ по вопросам

### 2.1. Кто разбивает знания на категории?

**Лидеры категоризации:**

- **Cursor** — пионер подхода. Перешёл от монолитного `.cursorrules` к директории `.cursor/rules/` с отдельными `.mdc` файлами по темам. Каждый файл — отдельная тема (testing, styling, architecture и т.д.)
- **Claude Code** — `CLAUDE.md` (корневой) + `.claude/rules/*.md` (тематические файлы). Поддерживает иерархию: user-level, project-level, rules-level
- **GitHub Copilot** — основной файл + `.github/instructions/` директория с именованными файлами (`models.instructions.md`, `testing.instructions.md`)
- **Continue.dev** — `.continue/rules/` с нумерованными файлами для контроля порядка загрузки
- **Cline** — `.clinerules/` директория с отдельными markdown файлами по темам

**Монолитный подход:**
- **Codex** — один `AGENTS.md` (но поддерживает иерархию по директориям)
- **Aider** — один `CONVENTIONS.md`
- **Windsurf** — один `.windsurfrules.md` + global rules

### 2.2. Кто динамически фильтрует контекст?

**Три механизма фильтрации обнаружены:**

#### A. Glob-based (файловые паттерны)
- **Cursor** — YAML frontmatter с `globs: ["**/*.tsx"]` → правило активируется только при работе с matching-файлами
- **GitHub Copilot** — `applyTo: "app/models/**/*.rb"` в frontmatter `.instructions.md` файлов
- **Claude Code** — `paths` в YAML frontmatter `.claude/rules/*.md`
- **Continue.dev** — `globs` + `regex` в YAML конфигурации правил

#### B. Semantic/AI-driven (по описанию)
- **Cursor** — `description` в frontmatter → агент решает, подтягивать ли правило
- **Devin** — Knowledge автоматически вспоминается по релевантности к текущей задаче
- **Continue.dev** — если `alwaysApply: false` и нет globs, агент решает по description

#### C. Manual toggle
- **Cline** — v3.13+ toggleable popover позволяет включать/выключать отдельные файлы правил

#### D. RAG-based (vector retrieval)
- **CrewAI** — ChromaDB для short-term memory с RAG retrieval
- **LangGraph** — InMemoryStore + langmem tools для semantic search

### 2.3. Кто грузит всё сразу?

- **Aider** — `CONVENTIONS.md` целиком в каждый запрос
- **Windsurf** — `.windsurfrules.md` + `global_rules.md` в каждый запрос
- **Codex** — `AGENTS.md` целиком (но ближайший к рабочей директории)
- **Claude Code** — `CLAUDE.md` всегда, `.claude/rules/` без `paths` — тоже всегда

### 2.4. Какой подход показывает лучшие результаты?

**Выводы из индустрии:**

1. **Glob-scoped правила** (Cursor, Copilot, Claude Code, Continue.dev) — **консенсус лидеров**. Позволяют масштабировать правила без раздувания контекста. Cursor явно указывает проблему "token tax": 20 global rules × 100 tokens = 2000 tokens в каждом запросе

2. **Semantic recall** (Devin) — хорош для больших баз знаний (100+ правил), но непредсказуем — агент может не вспомнить критическое правило

3. **RAG** (CrewAI) — теоретически масштабируется лучше всего, но overhead инфраструктуры (ChromaDB) и latency делают его непрактичным для CLI-инструментов

4. **"Всё сразу"** (Aider, Windsurf ранние версии) — работает для малых проектов (<50 правил), становится проблемой при масштабировании

**Тренд 2025-2026:** индустрия конвергирует к **glob-scoped правилам с YAML frontmatter**. Это подтверждается тем, что Cursor, GitHub Copilot, Claude Code и Continue.dev независимо пришли к почти идентичному формату.

### 2.5. Кто делает дистилляцию/сжатие?

**Явная дистилляция:**
- **Cline** — Auto-Compact: при приближении к лимиту контекста создаёт сжатое резюме и заменяет историю. Сохраняет решения, изменения кода, состояние
- **Cline** — Deep Planning: исследует codebase, затем создаёт `implementation_plan.md` — дистиллированный контекст для новой сессии
- **CrewAI** — RAG retrieval по сути является формой дистилляции (возвращает только релевантные фрагменты)

**Неявная дистилляция:**
- **Devin** — пишет CHANGELOG.md и SUMMARY.md как заметки для себя. Cognition признаёт: "summaries weren't comprehensive enough" — модель теряет детали
- **Windsurf** — Memories создаются автоматически из паттернов работы, но без явного механизма сжатия

**Нет дистилляции:**
- **Cursor**, **Copilot**, **Claude Code**, **Codex**, **Aider**, **Continue.dev** — правила подаются as-is, без трансформации

---

## 3. Архитектурные паттерны

### Паттерн A: "Статические правила с glob-скопингом"
**Используют:** Cursor, GitHub Copilot, Claude Code, Continue.dev
- Правила — markdown файлы в git
- YAML frontmatter контролирует когда применять
- Разработчик курирует вручную
- **Плюсы:** предсказуемо, версионируется, O(1) lookup
- **Минусы:** требует ручного обслуживания, не масштабируется на 1000+ правил

### Паттерн B: "Semantic knowledge recall"
**Используют:** Devin, OpenHands SDK
- Знания в базе (UI или API)
- Агент сам решает что вспоминать
- **Плюсы:** масштабируется, не требует ручной категоризации
- **Минусы:** непредсказуемо, может пропустить критическое правило

### Паттерн C: "Memory Bank + Auto-Compact"
**Используют:** Cline
- Структурированные markdown файлы как persistent memory
- Автоматическое сжатие при переполнении контекста
- **Плюсы:** переживает сессии, адаптивно
- **Минусы:** потеря деталей при сжатии, сложность реализации

### Паттерн D: "RAG-based memory"
**Используют:** CrewAI, LangGraph
- Vector DB для хранения и retrieval
- Semantic search по релевантности
- **Плюсы:** масштабируется на миллионы фактов
- **Минусы:** инфраструктурный overhead, latency, не подходит CLI

### Паттерн E: "Монолитный файл"
**Используют:** Codex (AGENTS.md), Aider (CONVENTIONS.md), Windsurf (.windsurfrules)
- Один файл загружается целиком
- **Плюсы:** простота, нулевой overhead
- **Минусы:** "token tax", не масштабируется

---

## 4. Ключевые выводы

### 4.1. Индустриальный консенсус (2025-2026)

1. **Glob-scoped правила — де-факто стандарт** для IDE-интегрированных агентов. 4 из 5 ведущих инструментов (Cursor, Copilot, Claude Code, Continue.dev) используют YAML frontmatter + glob patterns

2. **AGENTS.md как inter-tool стандарт** — Linux Foundation стандартизирует формат для кросс-инструментальной совместимости (поддержка: Codex, Cursor, Amp, Jules)

3. **Многоуровневая иерархия** — user → organization → project → directory → file. Поддерживают: Copilot (personal/repo/org), Claude Code (user/project/rules), Cursor (user/project)

4. **Auto-memory — редкость**: только Claude Code (auto memory), Windsurf (Memories), Cline (Memory Bank) имеют persistent learning. Большинство полагаются на ручную курацию

5. **Дистилляция — нерешённая проблема**: только Cline (Auto-Compact) имеет явный механизм. Devin признаёт потерю деталей при summarization

### 4.2. Рекомендации для bmad-ralph

| Аспект | Текущее состояние bmad-ralph | Индустриальная практика | Рекомендация |
|---|---|---|---|
| Категоризация | `.claude/rules/*.md` (7 файлов, ~122 правила) | Cursor: `.mdc`, Copilot: `.instructions.md` | **Уже на уровне лидеров** |
| Glob-скопинг | Scope hints в файлах | Cursor/Copilot/Continue: YAML frontmatter | Уже реализовано через Claude Code native |
| Дистилляция | Нет (R4 deferred) | Cline Auto-Compact, Devin summaries | Рассмотреть для правил >200 |
| Auto-memory | `memory/MEMORY.md` | Windsurf Memories, Cline Memory Bank | **Уже реализовано** |
| Лимиты | MEMORY.md ~200 строк | Cursor: token tax при 20+ rules | Мониторить размер rules |

### 4.3. Что bmad-ralph делает лучше конкурентов

- **Violation tracker** + escalation thresholds — ни один конкурент не отслеживает частоту нарушений правил
- **Extraction protocol** (обязательное обновление 4 мест после code-review) — систематическая экстракция знаний
- **Research-backed подход** (R1-R7) — осознанное применение исследований, а не ad-hoc

---

## 5. Источники

- [Cursor Rules Documentation](https://cursor.com/docs/context/rules)
- [Cursor Dynamic Context Discovery](https://cursor.com/blog/dynamic-context-discovery)
- [GitHub Copilot Custom Instructions](https://docs.github.com/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot)
- [GitHub Copilot Path-Scoped Instructions](https://github.blog/changelog/2025-07-23-github-copilot-coding-agent-now-supports-instructions-md-custom-instructions/)
- [Codex AGENTS.md Guide](https://developers.openai.com/codex/guides/agents-md/)
- [AGENTS.md Standard](https://agents.md/)
- [Aider Conventions](https://aider.chat/docs/usage/conventions.html)
- [Cline .clinerules](https://cline.bot/blog/clinerules-version-controlled-shareable-and-ai-editable-instructions)
- [Cline Memory Bank](https://docs.cline.bot/features/memory-bank)
- [Windsurf Rules & Workflows](https://www.paulmduvall.com/using-windsurf-rules-workflows-and-memories/)
- [Devin Playbooks](https://docs.devin.ai/product-guides/creating-playbooks)
- [Continue.dev Rules](https://docs.continue.dev/customize/deep-dives/rules)
- [Claude Code Memory](https://code.claude.com/docs/en/memory)
- [Claude Code Rules Directory](https://claudefa.st/blog/guide/mechanics/rules-directory)
- [CrewAI Memory](https://docs.crewai.com/en/concepts/memory)
- [OpenHands SDK](https://docs.openhands.dev/sdk)
- [LangGraph vs CrewAI Comparison](https://www.zenml.io/blog/langgraph-vs-crewai)
