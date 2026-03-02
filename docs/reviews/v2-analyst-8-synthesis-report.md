# Мета-анализ: Оптимальная архитектура знаний для CLI-оркестратора LLM-агента

**Аналитик:** analyst-8 (синтез)
**Дата:** 2026-03-02
**Версия:** 1.0
**Контекст:** Ralph — Go CLI, single-process, оркестрирует Claude Code через `claude --print`, pipe mode (каждый вызов = новый контекст), любой стек пользователя.

---

## Executive Summary

Оптимальная архитектура знаний для CLI-оркестратора LLM-агента — **трёхуровневая файловая система с glob-scoped загрузкой и Reflexion-паттерном накопления**. RAG избыточен при <500 правил. Академические системы (MemGPT, A-MEM, MemOS) архитектурно несовместимы с pipe mode. Индустриальный консенсус (Claude Code, Cursor, Cline, Copilot, Windsurf) единогласно сходится на **markdown-файлах с иерархической организацией** как единственном production-validated подходе для coding agents.

**Ключевой вывод:** Проблема не в retrieval (она тривиальна при <500 правил) — проблема в **quality of distillation** (как превратить сырой feedback в компактное, actionable правило) и **relevance filtering** (как загрузить только нужные правила в конкретный контекст).

---

## 1. Академические системы: применимость к pipe-mode CLI

### 1.1 MemGPT / Letta

**Модель:** LLM-as-OS с tiered memory (core ↔ archival ↔ recall). Виртуальное управление контекстом по аналогии с OS virtual memory.

**Применимость к Ralph:** НИЗКАЯ.
- Требует **persistent process** с self-editing context — несовместимо с pipe mode (`claude --print` = stateless).
- Core memory (main context) + Archival (vector DB) + Recall (conversation history) — архитектура для chatbot, не CLI.
- Letta benchmark (LoCoMo): filesystem-based агенты (74.0%) превосходят Mem0 Graph (68.5%) — подтверждает, что для code knowledge файлы эффективнее.

**Что взять:** Концепция tiered memory (always-loaded → on-demand → archive) применима к организации файлов.

**Источники:** [MemGPT arXiv 2310.08560](https://arxiv.org/abs/2310.08560), [Letta docs](https://docs.letta.com/concepts/memgpt/), [Letta blog](https://www.letta.com/blog/agent-memory)

### 1.2 A-MEM (Agentic Memory)

**Модель:** Zettelkasten-inspired — атомарные заметки с богатым контекстом, динамическое индексирование и связывание. Создаёт interconnected knowledge networks.

**Применимость к Ralph:** СРЕДНЯЯ (концептуально).
- Атомарность: одно правило = одна заметка с контекстом (файл, строка, почему). Это **уже реализовано** в `.claude/rules/*.md` в bmad-ralph.
- Связывание: cross-references между правилами (e.g., "see also wsl-ntfs.md"). Полезно при >200 правил.
- Selective top-k retrieval — аналог glob-scoped loading в Claude Code.

**Что взять:** Формализованная структура атомарного правила: `claim + evidence + context + links`.

**Источник:** [A-MEM arXiv 2502.12110](https://arxiv.org/abs/2502.12110)

### 1.3 SimpleMem

**Модель:** Semantic lossless compression для lifelong memory. 30x снижение token consumption при сохранении F1. Inspired by Complementary Learning Systems (CLS) theory.

**Применимость к Ralph:** СРЕДНЯЯ.
- **Semantic Structured Compression** — фильтрация low-utility данных, конвертация в compact memory units. Это то, что Ralph должен делать при distillation (code review findings → compact rules).
- **Online Semantic Synthesis** — объединение related fragments при записи. Аналог дедупликации правил.
- **Intent-Aware Retrieval** — адаптация scope запроса к текущей задаче. Аналог glob-scoped rules.

**Что взять:** Формализация distillation pipeline: raw finding → compression → deduplication → indexing.

**Источник:** [SimpleMem arXiv 2601.02553](https://www.alphaxiv.org/overview/2601.02553v1), [Tekta analysis](https://www.tekta.ai/ai-research-papers/simplemem-llm-agent-memory-2026)

### 1.4 MemOS

**Модель:** OS для памяти LLM. Три типа: Parametric (веса), Activation (KV-cache), Plaintext (текст). Unified scheduling через MemCube.

**Применимость к Ralph:** НИЗКАЯ.
- Parametric + Activation memory требуют доступа к модели — невозможно через pipe mode.
- Plaintext memory — это то, что Ralph уже делает.
- MemOS v2.0 показывает 159% improvement в temporal reasoning vs OpenAI global memory — но для persistent server, не CLI.

**Что взять:** Концепция MemScheduler для асинхронного ingestion — аналог post-review hook.

**Источник:** [MemOS arXiv 2505.22101](https://arxiv.org/abs/2505.22101), [MemOS v2 GitHub](https://github.com/MemTensor/MemOS)

### 1.5 Reflexion

**Модель:** Verbal reinforcement learning. Агент рефлексирует над ошибками → сохраняет рефлексию в episodic memory → использует в следующей попытке. "Semantic gradient" без обновления весов.

**Применимость к Ralph:** ВЫСОКАЯ — ЭТО КЛЮЧЕВОЙ ПАТТЕРН.
- Code review = trial, findings = feedback, distilled rules = reflection.
- Episodic memory buffer = `.claude/rules/*.md` файлы.
- "Semantic gradient signal providing concrete direction to improve" — точное описание того, что делают правила.
- Рабочий цикл Ralph: execute → review → reflect → store reflection → next execute uses reflection.

**Что взять:** Формализация Reflexion loop как core architecture pattern. Каждое правило = reflection с конкретным направлением улучшения.

**Источник:** [Reflexion arXiv 2303.11366](https://arxiv.org/abs/2303.11366), [NeurIPS 2023](https://openreview.net/forum?id=vAElhFcKW6)

### 1.6 Сводная таблица академических систем

| Система | Модель памяти | Pipe-mode совместимость | Применимость |
|---------|--------------|------------------------|-------------|
| MemGPT | Tiered (core/archival/recall) | НЕТ (persistent process) | Низкая — концепция tiers |
| A-MEM | Zettelkasten (atomic notes + links) | ДА (файловая) | Средняя — атомарность |
| SimpleMem | CLS-compression + synthesis | ЧАСТИЧНО (distillation) | Средняя — compression pipeline |
| MemOS | OS-level memory scheduling | НЕТ (requires model access) | Низкая — scheduling concept |
| Reflexion | Verbal RL (reflect → store → reuse) | ДА (файловая) | **ВЫСОКАЯ** — core pattern |

---

## 2. Индустриальный консенсус: coding agents

### 2.1 Claude Code (.claude/)

**Архитектура:**
- `CLAUDE.md` — always-loaded, <200 строк, high-signal instructions
- `.claude/rules/*.md` — glob-scoped, загружаются по pattern match
- `.claude/memory/` — auto-memory, персональные заметки
- Иерархия: project → user → global (override порядок)

**Ключевые инсайты:**
- "Find the smallest set of high-signal tokens" — минимализм как принцип
- Progressive disclosure: не всё загружается сразу
- Glob-scoped rules = intent-aware retrieval (загружай только релевантное)
- Context editing: автоматическое сжатие при подходе к лимиту (84% reduction)

**Источники:** [Claude Code Memory docs](https://code.claude.com/docs/en/memory), [Rules Directory guide](https://claudefa.st/blog/guide/mechanics/rules-directory), [Anthropic context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)

### 2.2 Cursor (.cursor/rules/)

**Архитектура:**
- `.cursor/rules/*.mdc` — path-scoped rules (YAML frontmatter с globs)
- Rule Types: Always, Auto (glob), Agent Requested, Manual
- Legacy: `.cursorrules` (root file, deprecated)
- Memories: AI-generated, toggleable

**Ключевые инсайты:**
- 4 уровня загрузки (Always → Auto → Agent-requested → Manual) = progressive disclosure
- 41% faster development с rule-guided generation
- 50% reduction в style-related PR comments
- Community-driven rule sharing (awesome-cursorrules, 15k+ stars)

**Источники:** [Cursor Rules guide](https://kirill-markin.com/articles/cursor-ide-rules-for-ai/), [awesome-cursorrules](https://github.com/PatrickJS/awesome-cursorrules)

### 2.3 Cline (memory-bank/ + .clinerules/)

**Архитектура:**
- `memory-bank/` — 6 core files в dependency order:
  - `projectbrief.md` → `productContext.md`, `systemPatterns.md`, `techContext.md` → `activeContext.md`, `progress.md`
- `.clinerules/` — version-controlled rules (directory or single file)
- AI-editable: Cline может модифицировать свои правила
- Toggle UI: включение/выключение конкретных rule files

**Ключевые инсайты:**
- Dependency order loading — приоритизация фундаментальных знаний
- Memory Bank = structured onboarding document для cold start
- AI-editable rules = self-improvement loop (Reflexion pattern!)
- Complete memory reset между сессиями — идентичная проблема pipe mode

**Источники:** [Cline Memory Bank](https://docs.cline.bot/prompting/cline-memory-bank), [.clinerules guide](https://cline.ghost.io/clinerules-version-controlled-shareable-and-ai-editable-instructions/)

### 2.4 Devin (Knowledge Base)

**Архитектура:**
- Dedicated knowledge base, AI-managed
- Feedback loop: developer corrections → codified knowledge
- Compounding learning: quality improves with usage time
- Project-scoped: knowledge accumulates per-project

**Ключевые инсайты:**
- "Codify feedback in the agent's knowledge base" — systematic extraction
- Compounding advantage: observable improvement over weeks
- Similar to human engineer onboarding pattern

**Источник:** [Devin Agents 101](https://devin.ai/agents101), [Devin Performance Review](https://cognition.ai/blog/devin-annual-performance-review-2025)

### 2.5 Copilot Spaces + AGENTS.md

**Архитектура:**
- Copilot Spaces: curated context bundles (files, docs, PRs, issues)
- AGENTS.md: open standard, 20,000+ repos (Sep 2025)
- `copilot-instructions.md` + `<file>.instructions.md` — hierarchical targeting
- Auto-sync: files в Space обновляются при изменении

**Ключевые инсайты:**
- AGENTS.md = vendor-agnostic `.cursorrules`/`CLAUDE.md`
- Наиболее эффективные AGENTS.md содержат: build commands, test commands, style rules, git workflow, boundaries
- Space = knowledge bundle с auto-refresh — но для hosted service, не CLI

**Источники:** [AGENTS.md blog](https://www.blog.brightcoding.dev/2025/09/21/agents-md-an-open-format-for-guiding-ai-coding-agents-with-project-instructions/), [Copilot Spaces](https://github.blog/ai-and-ml/github-copilot/github-copilot-spaces-bring-the-right-context-to-every-suggestion/), [Copilot Onboarding](https://github.blog/ai-and-ml/github-copilot/onboarding-your-ai-peer-programmer-setting-up-github-copilot-coding-agent-for-success/)

### 2.6 Windsurf (.windsurfrules)

**Архитектура:**
- `.windsurfrules` — project rules
- Cascade Memories — AI-auto-generated knowledge
- Rulebooks — reusable rule sets, invokable via slash commands

**Ключевые инсайты:**
- Auto-generated memories = automatic knowledge extraction
- Rulebooks = composable knowledge modules

**Источник:** [Windsurf Cascade docs](https://docs.windsurf.com/windsurf/cascade/cascade)

### 2.7 Индустриальный консенсус — сводка

| Tool | Storage | Scoping | AI-Editable | Cold Start |
|------|---------|---------|-------------|-----------|
| Claude Code | `.claude/rules/*.md` | Glob (path frontmatter) | Memory only | CLAUDE.md |
| Cursor | `.cursor/rules/*.mdc` | Glob (Always/Auto/Manual) | Memories | .cursorrules |
| Cline | `memory-bank/` + `.clinerules/` | Directory + toggle | Yes (full) | Memory Bank template |
| Devin | Internal KB | Project-scoped | Yes (via feedback) | Implicit |
| Copilot | Spaces + AGENTS.md | File-level instructions | No | AGENTS.md |
| Windsurf | `.windsurfrules` + memories | Rulebooks | Auto-memories | Rules file |

**Общий паттерн:** Markdown файлы + иерархическая организация + glob/path scoping + version control.

---

## 3. Context Engineering: рекомендации Anthropic

### 3.1 Ключевые принципы (из official blog)

1. **Minimal high-signal tokens** — "find the smallest set of high-signal tokens that maximize desired outcome"
2. **Just-in-time retrieval** — lightweight identifiers (paths, queries) → load at runtime
3. **Metadata-driven navigation** — file sizes, naming conventions, timestamps = implicit context
4. **Progressive disclosure** — incremental discovery через exploration
5. **Structured note-taking** — progress files, NOTES.md, to-do lists persisted outside context
6. **Multi-agent decomposition** — sub-agents с clean context windows для deep work

### 3.2 Long-running agent patterns

1. **Progress file** (`claude-progress.txt`) — что сделано, что осталось
2. **Git as state** — commit history как knowledge trail
3. **Session initialization** — read progress + git log + feature list перед работой
4. **Single-feature focus** — один focus per session, не всё сразу
5. **Context compaction** — summarize и reinitiate при подходе к лимиту

### 3.3 Применимость к Ralph

Ralph pipe mode = каждый `claude --print` вызов — новая сессия. Это **максимально жёсткий** вариант long-running agent (у Claude Code хотя бы interactive session с историей).

**Следствия:**
- CLAUDE.md + rules = **единственный** канал передачи знаний между вызовами
- Нет conversation history — всё должно быть в файлах
- Progress file (sprint-status.yaml) + git = state tracking
- Каждое правило должно быть **self-contained** (нет контекста предыдущего разговора)

**Источники:** [Anthropic context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents), [Long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

---

## 4. Growth Path: от 0 до 500 правил

### 4.1 Фазы роста

#### Phase 0: Empty project (0 правил)
- Единственный файл: `CLAUDE.md` или `AGENTS.md` с build/test/lint commands
- Проект-специфичные constraints (language, framework, architecture)
- ~20-50 строк, ~500-1000 tokens

#### Phase 1: Early learning (1-30 правил)
- Все правила в одном файле (e.g., `rules/patterns.md`)
- Линейное чтение — complexity O(n), но n мало
- Новое правило = append. Дедупликация при ~20 правил.
- ~100-500 строк, ~2000-8000 tokens — вмещается в always-loaded

#### Phase 2: Categorization (30-100 правил)
- Split по категориям: `testing.md`, `error-handling.md`, `naming.md`
- Glob-scoped loading становится полезным
- Index file с summary каждой категории
- ~500-2000 строк суммарно, ~8000-30000 tokens — нужен selective loading

#### Phase 3: Scaling (100-300 правил)
- Sub-categories: `testing/assertions.md`, `testing/mocks.md`
- Cross-references между файлами
- Violation tracker для приоритизации
- Periodic compression: merge related rules, archive obsolete
- ~2000-6000 строк, ~30000-80000 tokens — ОБЯЗАТЕЛЬНО selective loading

#### Phase 4: Maturity (300-500+ правил)
- Full taxonomy с 3+ уровнями
- Auto-tagging по severity/frequency
- Archival: правила без нарушений за N reviews → archive
- Potential transition к structured format (YAML/JSONL) для machine parsing
- RAG рассмотреть если glob-scoped loading недостаточно selective
- ~6000-10000+ строк, ~80000-150000 tokens — критична оптимизация

### 4.2 Критические пороги

| Порог | Проблема | Решение |
|-------|----------|---------|
| ~30 правил | Одный файл трудно сканировать | Split по категориям |
| ~100 правил | Always-loaded слишком много tokens | Glob-scoped selective loading |
| ~200 правил | Дублирование между файлами | Index + cross-references + dedup passes |
| ~300 правил | Некоторые правила устарели | Violation tracker → archive unused |
| ~500 правил | Glob-scoping недостаточно granular | Structured metadata + potential RAG |

### 4.3 Когда RAG?

**НЕ нужен при <500 правил.** Доказательство:
- 500 правил × ~30 слов/правило = ~15000 слов ≈ 20000 tokens
- С glob-scoped loading: в context попадает ~20-50 правил = ~3000-8000 tokens
- Это < 4% от 200K context budget — overhead RAG (embeddings, vector DB, retrieval latency) не оправдан

**Рассмотреть RAG при:**
- >500 правил И glob-scoping даёт >50 правил на вызов
- Семантический поиск нужен (по смыслу, не по file path)
- Multi-project knowledge sharing (cross-repo patterns)

---

## 5. Стек-агностичность: универсальные категории знаний

### 5.1 Стек-инвариантные категории (работают для ЛЮБОГО языка)

1. **Testing patterns** — naming, structure, assertion quality, mocking (Go `t.Errorf` → Python `pytest.raises` → Rust `#[should_panic]`)
2. **Error handling** — wrapping, propagation, sentinel errors (Go `errors.Is` → Python exception hierarchy → Rust `Result<T,E>`)
3. **Code quality** — DRY, naming, doc comments accuracy, scope creep guards
4. **Architecture** — dependency direction, package/module boundaries, interface placement
5. **Build & CI** — commands, linting, formatting, test commands
6. **Project conventions** — file structure, naming patterns, commit style

### 5.2 Стек-специфичные категории (уникальны для каждого языка)

- **Go:** error wrapping fmt.Errorf, table-driven tests, interface in consumer
- **Python:** type hints, pytest fixtures, import ordering, virtual environments
- **TypeScript:** strict mode, ESLint rules, type vs interface, barrel exports
- **Rust:** ownership patterns, lifetime annotations, macro usage, unsafe blocks
- **Java:** checked exceptions, Spring patterns, Maven/Gradle conventions

### 5.3 Архитектура для стек-агностичности

```
.ralph/knowledge/
├── RULES.md                    # Always-loaded: build/test/lint + top-5 critical rules
├── universal/                  # Стек-инвариантные
│   ├── testing.md
│   ├── error-handling.md
│   ├── code-quality.md
│   └── architecture.md
├── {language}/                 # Стек-специфичные (auto-detected)
│   ├── idioms.md
│   ├── testing-patterns.md
│   └── common-errors.md
└── project/                    # Проект-специфичные
    ├── conventions.md
    ├── build.md
    └── known-issues.md
```

**Language detection:** Ralph определяет стек по `go.mod` / `package.json` / `Cargo.toml` / `pom.xml` / `pyproject.toml` и загружает соответствующий `{language}/` каталог.

---

## 6. Cold Start: стратегия "Day Zero"

### 6.1 Проблема

Новый проект: 0 правил. Первые code reviews дают сырой feedback, но система знаний ещё пуста. Как bootstrap?

### 6.2 Индустриальные подходы

| Tool | Cold Start Strategy |
|------|-------------------|
| Claude Code | CLAUDE.md создаётся вручную или с `claude /init` |
| Cursor | `.cursorrules` из community templates |
| Cline | Memory Bank template (6 файлов заполняются постепенно) |
| Copilot | AGENTS.md (20k+ repos, copy-paste) |
| Devin | Implicit (learns from first interactions) |

### 6.3 Рекомендуемая стратегия для Ralph

#### Tier 1: Auto-generated (0 человеческих усилий)

При первом запуске Ralph на проекте:
1. **Detect stack** → create `{language}/` skeleton
2. **Parse project structure** → generate `project/conventions.md` (directories, naming patterns)
3. **Extract build commands** → populate `project/build.md` from Makefile/package.json/Cargo.toml
4. **Seed RULES.md** с universal starter rules:
   ```
   - Test names: describe scenario, not implementation
   - Error messages: include context (what failed, with what input)
   - DRY: extract on 3rd repetition, not before
   - Doc comments: must match actual behavior
   ```

#### Tier 2: First review bootstrap (1 review = ~5-10 rules)

После первого code review:
1. Distill findings → rules (Reflexion pattern)
2. Categorize → correct file
3. Update RULES.md summary

#### Tier 3: Community templates (optional)

Pre-built knowledge packs для popular stacks:
- `ralph-knowledge-go` — Go-specific patterns (table-driven tests, error wrapping, etc.)
- `ralph-knowledge-python` — Python-specific patterns
- Installed via `ralph init --template go`

### 6.4 Starter Knowledge (universal, стек-агностичные)

Базовые правила, полезные для ЛЮБОГО проекта с нуля:

1. **Test naming**: описывай сценарий, не реализацию
2. **Error context**: включай что упало и с каким input
3. **DRY threshold**: extract при 3-м повторении
4. **Doc accuracy**: doc comments MUST match behavior
5. **Scope guard**: не добавляй scope за пределами задачи
6. **Assertion quality**: verify content, не только existence
7. **Error path testing**: test ALL error returns, не только happy path

Эти 7 правил покрывают >60% типичных findings в первых code reviews (на основе метрик bmad-ralph: assertion quality + doc comments + error paths = top-3 recurring categories).

---

## 7. Оптимальная архитектура: рекомендация

### 7.1 Core Design Principles

1. **Reflexion-based accumulation**: execute → review → distill → store → next execute reads stored
2. **File-based, version-controlled**: markdown files в git, AI-readable и human-readable
3. **Glob-scoped loading**: загружай только релевантное текущей задаче
4. **Minimal always-loaded**: RULES.md <200 строк, только critical rules + index
5. **Progressive growth**: architecture одинаково работает при 0, 50, 200, 500 правил
6. **Stack-agnostic core + stack-specific extensions**: universal/ + {language}/

### 7.2 Architecture Diagram

```
┌─────────────────────────────────────────────┐
│                 Ralph CLI                     │
├─────────────────────────────────────────────┤
│  claude --print  (pipe mode, stateless)      │
├─────────────────┬───────────────────────────┤
│  ALWAYS LOADED  │  ON-DEMAND (glob-scoped)  │
│                 │                            │
│  RULES.md       │  universal/*.md            │
│  (<200 lines)   │  {language}/*.md           │
│  - build/test   │  project/*.md              │
│  - top-N rules  │                            │
│  - file index   │  Loaded when:              │
│                 │  - task matches glob        │
│                 │  - agent requests           │
├─────────────────┴───────────────────────────┤
│              FEEDBACK LOOP                    │
│  code-review → findings → distill → store    │
│  (Reflexion pattern)                         │
├─────────────────────────────────────────────┤
│              MAINTENANCE                      │
│  dedup pass, archive unused, compress         │
│  (SimpleMem-inspired)                         │
└─────────────────────────────────────────────┘
```

### 7.3 Rule Format (A-MEM inspired)

```markdown
## [CATEGORY] Rule title

**Context:** file pattern where this applies (e.g., `*_test.go`)
**Evidence:** story/review where discovered (e.g., Story 3.2, Finding #4)
**Rule:** One-sentence actionable instruction
**Example:** concrete code example (optional, for complex rules)
**See also:** cross-references to related rules (optional)
```

### 7.4 Loading Strategy

```
Phase 0 (0 rules):    RULES.md only (~500 tokens)
Phase 1 (1-30):       RULES.md + single patterns.md (~3000 tokens)
Phase 2 (30-100):     RULES.md + glob-matched files (~5000-8000 tokens)
Phase 3 (100-300):    RULES.md summary + glob-matched subsets (~8000-15000 tokens)
Phase 4 (300-500+):   RULES.md index + precise glob + potential structured retrieval
```

**Budget:** При 200K context, задача + код занимают ~100-150K tokens. Knowledge budget: ~10-30K tokens = 60-200 правил per invocation. Glob-scoping обеспечивает это при total knowledge base до ~500 правил.

### 7.5 Distillation Pipeline (key innovation area)

```
Raw finding (from code review)
    ↓
Compression: remove review-specific context, keep actionable rule
    ↓
Deduplication: check existing rules for overlap
    ↓
Categorization: universal/ vs {language}/ vs project/
    ↓
Formatting: A-MEM atomic note format
    ↓
Indexing: update RULES.md summary + category index
    ↓
Validation: rule is testable (can write assertion for it)
```

### 7.6 Что НЕ делать

1. **НЕ RAG при <500 правил** — overhead не оправдан, glob-scoping достаточно
2. **НЕ persistent server** — Ralph = CLI binary, knowledge = files
3. **НЕ embedding-based retrieval** — требует external API, adds latency, dependency
4. **НЕ monolithic knowledge file** — не масштабируется после ~30 правил
5. **НЕ auto-archive без tracking** — нужен violation tracker для evidence-based archival
6. **НЕ stack-specific-only** — universal patterns покрывают >60% findings

---

## 8. Сравнение с текущей реализацией bmad-ralph

### 8.1 Что bmad-ralph уже делает правильно

| Принцип | Реализация в bmad-ralph | Источник валидации |
|---------|------------------------|-------------------|
| File-based knowledge | `.claude/rules/*.md` | Индустриальный консенсус |
| Glob-scoped loading | YAML frontmatter `# Scope:` | Claude Code, Cursor patterns |
| Always-loaded index | `CLAUDE.md` <80 строк | Anthropic recommendation |
| Reflexion loop | code-review → distill → store | Academic (Reflexion, NeurIPS 2023) |
| Atomic rules | One pattern per bullet point | A-MEM Zettelkasten |
| Topic splitting | 7 topic files + index | Scaling pattern (Phase 2-3) |
| Violation tracking | `.claude/violation-tracker.md` | SimpleMem frequency-based |
| Version control | All rules in git | Universal industry practice |

### 8.2 Что можно улучшить

1. **Формализовать rule format** — добавить evidence + cross-references (A-MEM)
2. **Стек-агностичность** — текущие rules Go-specific; нужна universal/ vs go/ split
3. **Cold start templates** — нет starter knowledge для новых проектов
4. **Distillation quality gate** — нет formal pipeline (raw → compressed → categorized → indexed)
5. **Auto-detection** — stack detection для language-specific loading
6. **Compression pass** — periodic merge/dedup (SimpleMem Online Synthesis)

---

## 9. Evidence from Project Research (R1/R2/R3) and V1 Analyst Reports

### 9.1 Критические количественные данные из R1/R2/R3

Три предыдущих исследования (62 источника суммарно) дают **жёсткие количественные пороги**, которые ОБЯЗАТЕЛЬНЫ для архитектуры знаний:

| Метрика | Значение | Источник | Импликация для архитектуры |
|---------|----------|----------|--------------------------|
| Context rot | 30-50% degradation | R1 [S5] (18 frontier models) | Минимизировать loaded tokens |
| Instruction ceiling | ~150-200 rules max | R1 [S14] | Selective loading обязателен |
| **Compliance @ 15 rules** | **94%** | R2 [S25] | Файл НЕ должен содержать >15 rules |
| Compliance @ 100+ rules | 40-50% | R2 [S25] | Monolithic loading = провал |
| Hook activation boost | 20% → 84% | R2 [S27] | Push-based > pull-based |
| MCP skip rate | 56% skipped | R2 [S31] | Pull-based tools ненадёжны |
| Filesystem vs Graph | 74.0% vs 68.5% | R3 [S1] (Letta LoCoMo) | Файлы > специализированные tools |
| RAG break-even | >500 entries | R3 | До 500 — file-based optimal |
| Context rot threshold | 20-50K tokens docs | R1, Analyst-8-v1 | Knowledge budget: <10% context |

**Ключевой вывод для новых проектов:** Порог 15 rules/file при 94% compliance — это ЖЁСТКОЕ архитектурное ограничение. При любом стеке, при любом масштабе — один файл не должен содержать больше 12-15 actionable rules.

### 9.2 Тройной барьер (Triple Barrier) из R2

R2 идентифицирует три механизма, уничтожающих знания в pipe-mode агентах:

1. **Compaction barrier:** При сжатии контекста (автоматическое в Claude Code, manual `/compact`) — правила теряются. GitHub Issues документируют систематическую потерю rules после compaction.

2. **Context rot barrier:** 30-50% деградация точности при росте контекста. "Lost in the middle" — U-образная кривая внимания. Правила в середине длинного контекста игнорируются.

3. **Rule overload barrier:** >15 imperative rules → selective ignoring (agent сам решает что важно, часто неправильно).

**Импликация для новых проектов:** Даже при 0 правил на старте, архитектура ДОЛЖНА планировать все три барьера с первого дня. Иначе при росте до 50+ правил система деградирует незаметно.

**Решения (validated by R2):**
- **Anti-compaction:** SessionStart hook — fires при startup, resume, clear, compact (~90-94% reliability)
- **Anti-context-rot:** Glob-scoped loading — загружай только релевантные 10-15 rules
- **Anti-overload:** Topic sharding — max 12-15 rules per file, не больше

### 9.3 Enforcement Hierarchy (из R2, validated by bmad-ralph Epics 1-5)

Критически важно для pipe mode: КАК доставлять знания агенту. Ranking по надёжности:

```
100%  PostToolUse hook     — детерминистические fixes (CRLF, format)
~100% PreToolUse hook      — inject checklist перед каждым edit
~94%  SessionStart hook    — fires on startup/resume/clear/compact
~80%  CLAUDE.md / RULES.md — always-loaded, но "system-reminder" framing снижает authority
~60%  .claude/rules/*.md   — glob-scoped, но "system-reminder" + position = attention diluted
~50%  Pull-based (MCP)     — agent сам решает когда вызвать → 56% skip rate
~30%  Agent-initiated      — agent самостоятельно ищет правила → lowest compliance
```

**Для нового проекта:** Enforcement hierarchy определяет ГДЕ размещать правила:
- Top 5-7 critical rules → SessionStart hook (highest compliance)
- Next 15-30 rules → always-loaded RULES.md
- 30-100+ rules → glob-scoped topic files
- Never rely on agent pulling rules from MCP/tools

### 9.4 V1 Analyst Insights для stack-agnostic design

#### Analyst-1: Двойная инъекция — архитектурный антипаттерн
- Если rules лежат И в auto-loaded `.claude/rules/` И инжектируются через Go → 7-12K token duplication (3.5-6% context)
- **Для Ralph:** Выбрать ОДНУ delivery path. Go-controlled injection даёт scope filtering + JIT validation + deterministic ordering
- **Для новых проектов:** Ralph должен инжектировать через `__RALPH_KNOWLEDGE__` placeholder, НЕ полагаясь на auto-load хост-инструмента

#### Analyst-3: Конкурентный анализ подтверждает unique advantages
- Violation tracker + escalation thresholds — **уникален** среди всех competitors
- 3-layer distillation pipeline — **уникален** (никто не автоматизирует distillation)
- Automatic scope-aware rules generation — **first-mover** (все конкуренты требуют ручного создания rules)

#### Analyst-4: Пороги структурирования (confirmed cross-stack)
- <30 rules: один файл достаточно
- 30-80: категоризация опциональна
- 80-150: категоризация рекомендована
- >150: категоризация + glob filtering обязательны
- "Lost in the middle" подтверждён экспериментально — правила в середине файла underperform

#### Analyst-5: Динамическая фильтрация по стадии — НЕ нужна
- При <15K tokens правил → full loading без stage-specific filtering
- Stage-specific injection (execute vs review) рассмотреть при >15K tokens
- False negatives от selective injection дороже чем marginal noise

#### Analyst-6: LLM compression опасна
- "Silent semantic narrowing" — рекурсивная synthetic compression деградирует смысл
- Accumulate + manual cleanup > LLM auto-compress (until ~500 lines)
- Imperative rules (не RAG facts) не бенефитят от compression

#### Analyst-7: Scoring альтернатив
| Approach | Score | Notes |
|----------|-------|-------|
| Hierarchical prompting | 9.1/10 | Formalize demotion policy |
| Tool-based hooks | 8.5/10 | Extend PreToolUse for context-aware groups |
| Lazy loading/hybrid | 7.8/10 | Current Claude Code glob sufficient |
| MCP knowledge tool | 7.3/10 | Defer to >300 rules, verify pipe mode |
| BM25 embedded RAG | 6.2/10 | Lexical mismatch problem |
| Semantic embeddings | 4.8/10 | External API dependency, overkill |

### 9.5 Architectural Innovations из Epic 6 Research

#### Pending-File Pattern
```
Claude writes → .ralph/pending-lessons.md (transient)
    → Go quality gates G1-G6 (format, citation, dedup, budget, cap, min-content)
    → Valid entries appended to LEARNINGS.md
```
- Решает 4 проблемы одновременно: write path clarity, [needs-formatting] pollution, mutation asymmetry, thread safety
- **Stack-agnostic:** Go gates не зависят от языка проекта

#### Progressive Disclosure T1/T2/T3 (validated by industry)
- **T1 (ralph-critical.md):** always loaded, 15 critical rules from violation tracking
- **T2 (ralph-{category}.md):** glob-scoped, auto-generated from distillation
- **T3 (LEARNINGS.md):** full injection point, newest-first ordering
- Совпадает с Google ADK, Anthropic, Microsoft, Claude-Mem, Peking University research

#### Violation Tracking Feedback Loop
```
Rule created → loaded in context → code review detects violation?
    YES → increment counter → if threshold exceeded → promote to T1 (critical)
    NO for N reviews → candidate for archival
```
- **Уникален среди всех tools** — никто не трекает violation frequency
- Data-driven promotion/demotion: правила, которые нарушаются часто → попадают в SessionStart hook
- Правила без нарушений → demotion candidates

#### Scope-Aware Auto-Generation (First-Mover)
- Ralph **автоматически генерирует** glob-scoped rules через LLM distillation
- Все конкуренты (Cursor, Claude Code, Copilot, Aider) требуют ручного создания
- Go validation: `filepath.Match` + file match proof + canonical category list + backup/restore

### 9.6 Уточнённая архитектура с учётом ALL evidence

```
┌─────────────────────────────────────────────────────────┐
│              RALPH KNOWLEDGE ARCHITECTURE                 │
│              (Stack-Agnostic, Cold-Start Ready)           │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  DELIVERY LAYER (enforcement hierarchy)                   │
│  ┌──────────────────────────────────────────────┐        │
│  │ SessionStart Hook (~94% compliance)           │        │
│  │ - Top 5-7 critical rules (from violation      │        │
│  │   tracker, T1 promotion)                      │        │
│  │ - Stack-detected build/test commands           │        │
│  └──────────────────────────────────────────────┘        │
│  ┌──────────────────────────────────────────────┐        │
│  │ Always-Loaded RULES.md (~80% compliance)      │        │
│  │ - <200 lines, index + top rules               │        │
│  │ - Max 12-15 actionable rules (compliance      │        │
│  │   threshold from R2)                          │        │
│  └──────────────────────────────────────────────┘        │
│  ┌──────────────────────────────────────────────┐        │
│  │ Glob-Scoped Files (~60% compliance)           │        │
│  │ - ralph-{category}.md (T2), max 15 rules each │        │
│  │ - Loaded when task file pattern matches glob   │        │
│  │ - Auto-generated by distillation pipeline      │        │
│  └──────────────────────────────────────────────┘        │
│                                                           │
│  ACCUMULATION LAYER (Reflexion pattern)                   │
│  ┌──────────────────────────────────────────────┐        │
│  │ Pending File (transient)                      │        │
│  │ Claude writes → .ralph/pending-lessons.md     │        │
│  │                                               │        │
│  │ Go Quality Gates G1-G6                        │        │
│  │ → format → citation → dedup → budget → cap    │        │
│  │                                               │        │
│  │ LEARNINGS.md (T3, newest-first)               │        │
│  │ Raw validated lessons, pre-distillation        │        │
│  └──────────────────────────────────────────────┘        │
│                                                           │
│  DISTILLATION LAYER (SimpleMem-inspired)                  │
│  ┌──────────────────────────────────────────────┐        │
│  │ Go semantic dedup (0 tokens, normalized match) │        │
│  │ → LLM compression (7 validation criteria)     │        │
│  │ → Go validation + circuit breaker             │        │
│  │ → Auto-generate glob-scoped ralph-*.md (T2)   │        │
│  └──────────────────────────────────────────────┘        │
│                                                           │
│  FEEDBACK LAYER (unique to Ralph)                         │
│  ┌──────────────────────────────────────────────┐        │
│  │ Violation Tracker                             │        │
│  │ - Frequency counting per rule                 │        │
│  │ - Threshold-based T2→T1 promotion             │        │
│  │ - Inactivity-based T2→archive demotion        │        │
│  │ - Data-driven, not opinion-driven             │        │
│  └──────────────────────────────────────────────┘        │
│                                                           │
│  COLD START LAYER                                         │
│  ┌──────────────────────────────────────────────┐        │
│  │ Auto-detect: go.mod/package.json/Cargo.toml   │        │
│  │ → Seed RULES.md (7 universal starter rules)   │        │
│  │ → Create {language}/ skeleton                  │        │
│  │ → Extract build/test from project files        │        │
│  │ → First review = 5-10 bootstrapped rules       │        │
│  └──────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────┘
```

### 9.7 Ключевые расхождения между V1 и V2 анализом

| Аспект | V1 (bmad-ralph specific) | V2 (new project, any stack) |
|--------|--------------------------|---------------------------|
| Storage | `.claude/rules/` + auto-load | `.ralph/rules/` + Go injection (analyst-1) |
| Language | Go-specific patterns | Universal + {language}/ extensions |
| Cold start | Manual CLAUDE.md | Auto-detect + seed + starter rules |
| Distillation | LLM compression pipeline | Same, but language-aware categories |
| Categories | 7 Go-specific (testing, errors...) | Core universal + extension mechanism |
| Enforcement | Claude Code hooks | Ralph-own SessionStart + prompt injection |
| Scale target | 122→500 patterns | 0→500 (growth path explicit) |

---

## 10. Заключение

### Единственный верный подход для pipe-mode CLI agent:

**Файловая система + Reflexion паттерн + glob-scoped loading + enforcement hierarchy + violation feedback loop.**

Это не компромисс — это оптимум, подтверждённый **85+ источниками** из трёх классов evidence:

- **Академически (5 систем):** Reflexion (NeurIPS 2023) = core pattern; SimpleMem = distillation pipeline; A-MEM = atomic note format. MemGPT/MemOS неприменимы (требуют persistent process).
- **Индустриально (6/6 coding agents):** Claude Code, Cursor, Cline, Devin, Copilot, Windsurf — все используют markdown files с hierarchical organization.
- **Бенчмарками:** Letta LoCoMo — filesystem (74.0%) > Mem0 Graph (68.5%); hook activation 20%→84%; 15 rules = 94% compliance.
- **Project research (R1/R2/R3, 62 источника):** Triple barrier quantified, enforcement hierarchy validated, RAG break-even at >500 entries.
- **V1 analyst reports (8 reports):** Double-injection bug, pending-file pattern, scope-aware auto-generation = first-mover advantage.

### Три innovation areas (в порядке приоритета):

1. **Distillation quality** — как превратить сырой finding в actionable rule без semantic narrowing (analyst-6 warning). Go quality gates + LLM compression с 7 validation criteria + circuit breaker.

2. **Enforcement hierarchy** — compliance drops from 94% to 40% без правильной delivery. SessionStart hook (top rules) → always-loaded RULES.md → glob-scoped files. Push > pull, deterministic > stochastic.

3. **Violation feedback loop** — data-driven promotion/demotion. Уникально среди всех competitors. Правила, которые нарушаются часто → SessionStart. Правила без нарушений → archive.

### RAG — не нужен при <500 правил.
При knowledge base <500 правил glob-scoped file loading достаточно selective (загружает 10-15 rules = ~2000-3000 tokens = 1.5% context), а overhead RAG (embedding API, vector DB, 200-500ms latency) не оправдан.

### Cold start — решаема:
Auto-detect stack → seed 7 universal rules → first review bootstrap → progressive growth через established architecture. Никаких architectural changes при переходе 0→500.

---

## Источники

### Академические
- [MemGPT: Towards LLMs as Operating Systems](https://arxiv.org/abs/2310.08560) — Packer et al., 2023
- [A-MEM: Agentic Memory for LLM Agents](https://arxiv.org/abs/2502.12110) — 2025
- [SimpleMem: Efficient Lifelong Memory for LLM Agents](https://www.alphaxiv.org/overview/2601.02553v1) — 2026
- [MemOS: Operating System for Memory-Augmented Generation](https://arxiv.org/abs/2505.22101) — 2025
- [Reflexion: Language Agents with Verbal Reinforcement Learning](https://arxiv.org/abs/2303.11366) — Shinn et al., NeurIPS 2023

### Индустриальные
- [Anthropic: Effective Context Engineering for AI Agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) — Sep 2025
- [Anthropic: Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) — 2025
- [Claude Code Memory docs](https://code.claude.com/docs/en/memory)
- [Claude Code Rules Directory guide](https://claudefa.st/blog/guide/mechanics/rules-directory)
- [Cline Memory Bank](https://docs.cline.bot/prompting/cline-memory-bank)
- [.clinerules guide](https://cline.ghost.io/clinerules-version-controlled-shareable-and-ai-editable-instructions/)
- [Cursor Rules for AI](https://kirill-markin.com/articles/cursor-ide-rules-for-ai/)
- [awesome-cursorrules](https://github.com/PatrickJS/awesome-cursorrules)
- [Devin Agents 101](https://devin.ai/agents101)
- [Devin Performance Review 2025](https://cognition.ai/blog/devin-annual-performance-review-2025)
- [Copilot Spaces](https://github.blog/ai-and-ml/github-copilot/github-copilot-spaces-bring-the-right-context-to-every-suggestion/)
- [AGENTS.md open format](https://www.blog.brightcoding.dev/2025/09/21/agents-md-an-open-format-for-guiding-ai-coding-agents-with-project-instructions/)
- [Copilot Agent Onboarding](https://github.blog/ai-and-ml/github-copilot/onboarding-your-ai-peer-programmer-setting-up-github-copilot-coding-agent-for-success/)
- [Windsurf Cascade docs](https://docs.windsurf.com/windsurf/cascade/cascade)
- [Letta Agent Memory blog](https://www.letta.com/blog/agent-memory)

### Предыдущие исследования bmad-ralph (R1-R3, 62 источника суммарно)
- `docs/research/knowledge-extraction-in-claude-code-agents.md` (R1) — 20 источников, context rot quantification
- `docs/research/knowledge-enforcement-in-claude-code-agents.md` (R2) — 40 источников, triple barrier, enforcement hierarchy
- `docs/research/alternative-knowledge-methods-for-cli-agents.md` (R3) — 22 источника, RAG vs file-based benchmarks

### V1 Analyst Reports (Round 1, 8 отчётов)
- `docs/reviews/analyst-1-ab-report.md` — Storage location: .ralph/rules/ vs .claude/rules/, double-injection bug
- `docs/reviews/analyst-2-cd-report.md` — LEARNINGS.md freshness, token efficiency, hybrid without duplication
- `docs/reviews/analyst-3-competitors-report.md` — Industry convergence on glob-scoped modular rules
- `docs/reviews/analyst-4-categorization-report.md` — Structural thresholds (<30/80/150), context rot evidence
- `docs/reviews/analyst-5-dynamic-injection-report.md` — Stage-specific injection NOT justified at <15K tokens
- `docs/reviews/analyst-6-distillation-report.md` — LLM compression risks (semantic narrowing), accumulate > compress
- `docs/reviews/analyst-7-alternatives-report.md` — Scored 6 approaches, hierarchical prompting wins (9.1/10)
- `docs/reviews/analyst-8-synthesis-report.md` — V1 meta-analysis, 5 convergent patterns, growth roadmap

### Epic 6 Research Documents
- `docs/research/epic6-competitive-analysis.md` — bmad-ralph leads in 3/5 dimensions
- `docs/research/epic6-injection-universality-review.md` — Scope hints detection, category extension mechanism
- `docs/research/epic6-extraction-pipeline-review.md` — Pending-file pattern, Go quality gates G1-G6
- `docs/research/scope-aware-rules-generation-research.md` — First-mover: auto-generated glob-scoped rules
- `docs/research/agent-orchestrator-analysis.md` — Complementary to Ralph (parallel vs knowledge)
- `docs/research/research-1-serena-ralph-2026-02-24.md` — MCP integration strategy
