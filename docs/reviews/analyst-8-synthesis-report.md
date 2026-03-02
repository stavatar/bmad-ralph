# Мета-анализ: оптимальная архитектура управления знаниями для LLM-агента (CLI)

**Аналитик:** analyst-8
**Дата:** 2026-03-02
**Контекст:** bmad-ralph — single-process Go CLI, Claude Code (200K контекст), .claude/rules/ автозагрузка
**Ограничения:** нет внешних БД, нет RAG, файловая система как единственное хранилище, 3 зависимости (cobra, yaml.v3, fatih/color)

---

## Executive Summary

- **File-based injection остаётся оптимальным** для knowledge bases <500 записей. Letta benchmark: filesystem-based агенты (74.0% на LoCoMo) превосходят специализированные memory tools (Mem0 Graph: 68.5%) [R3-S1].
- **Context rot — фундаментальное ограничение:** 30-50% degradation при полном контексте vs компактном (Chroma Research, 18 моделей) [R1-S5]. Budget management — инженерная необходимость, не оптимизация.
- **Трёхуровневая иерархия** (always-loaded core → glob-scoped topic files → on-demand deep reference) — паттерн, подтверждённый MemGPT/Letta, Claude Code, GitHub Copilot Spaces.
- **Hook-based enforcement** — единственный детерминистический механизм. Skills activation: ~20% baseline → ~84% с forced evaluation hooks [R2-S27].
- **Дистилляция необходима**, но только для operational knowledge (review findings). Structural rules (coding conventions) стабильны и не требуют дистилляции.
- **Рекомендация:** сохранить и усилить текущую архитектуру bmad-ralph (CLAUDE.md + .claude/rules/ + hooks), добавив topic-based sharding, violation tracking, и periodic compression operational knowledge.

---

## 1. Background

### 1.1 Проблема

LLM-агент, работающий в multi-session режиме, должен:
1. Накапливать знания из опыта (review findings, bug patterns)
2. Применять их консистентно в будущих сессиях
3. Не деградировать по мере роста knowledge base (context rot)

### 1.2 Ограничения bmad-ralph

| Ограничение | Следствие |
|---|---|
| Single-process Go CLI | Нет persistent server, нет фоновых процессов |
| 3 зависимости | Нет vector stores, нет graph DB |
| Claude Code runtime | 200K контекст, автозагрузка CLAUDE.md + .claude/rules/, hooks |
| Файловая система | Единственное хранилище между сессиями |

### 1.3 Предшествующие исследования проекта

- **R1** (knowledge-extraction): 20 источников, 6-уровневая иерархия памяти Claude Code, context rot quantified
- **R2** (knowledge-enforcement): 40 источников, тройной барьер (compaction + context rot + rule overload), hook-based enforcement
- **R3** (alternative-methods): 22 источника, RAG избыточен при <500 записей, file-based оптимален

---

## 2. Academic Survey

### 2.1 MemGPT / Letta — LLM-as-Operating-System

**Источник:** [MemGPT: Towards LLMs as Operating Systems](https://arxiv.org/abs/2310.08560) (NeurIPS 2023 Workshop → Letta framework)

**Ключевая идея:** Контекстное окно LLM = RAM, внешнее хранилище = disk. LLM сам управляет перемещением данных между уровнями через tool calls.

**Трёхуровневая архитектура:**
1. **Core Memory** (in-context, аналог RAM) — всегда виден агенту, embedded в system prompt
2. **Recall Memory** (searchable history) — полная история взаимодействий, semantic search
3. **Archival Memory** (processed knowledge) — обработанные и индексированные знания

**Релевантность для bmad-ralph:**
- Core Memory ≈ CLAUDE.md (~80 строк, always loaded)
- Recall Memory ≈ conversation context (managed by Claude Code compaction)
- Archival Memory ≈ .claude/rules/ (glob-scoped, loaded on demand)
- **Паттерн подтверждён:** tiered memory с explicit management — production-validated подход

**Ограничения:** Letta — hosted platform с Python runtime, не embeddable Go library. Но архитектурные принципы переносимы.

### 2.2 A-MEM — Agentic Memory (Zettelkasten для LLM)

**Источник:** [A-MEM: Agentic Memory for LLM Agents](https://arxiv.org/abs/2502.12110) (NeurIPS 2025)

**Ключевая идея:** Каждый memory unit — "atomic note" с контекстным описанием, ключевыми словами, тегами и связями с другими notes (по принципу Zettelkasten).

**Механизм:**
1. При добавлении новой памяти — генерация structured attributes
2. Анализ связей с существующими notes по semantic similarity
3. Динамическое обновление существующих notes при добавлении новых

**Релевантность для bmad-ralph:**
- Atomic notes ≈ atomized facts в .claude/rules/ (текущий паттерн: `- Pattern description [file.go] (Story X.Y)`)
- Linkage ≈ cross-references между файлами правил (index в go-testing-patterns.md)
- **Ключевой инсайт:** structured metadata (tags, keywords) на каждом fact — это то, чего не хватает текущей реализации. Теги позволили бы glob-scoped загрузку по семантике, а не только по имени файла.

**Ограничения:** Требует LLM call при каждом добавлении памяти (дорого для CLI tool). Selective top-k retrieval предполагает embedding search.

### 2.3 SimpleMem — Semantic Lossless Compression

**Источник:** [SimpleMem: Efficient Lifelong Memory for LLM Agents](https://arxiv.org/abs/2601.02553) (2026)

**Three-stage pipeline:**
1. **Semantic Structured Compression** — raw interactions → compact multi-view indexed memory units
2. **Online Semantic Synthesis** — intra-session merging related context → unified representations
3. **Intent-Aware Retrieval** — infers search intent → dynamic retrieval scope

**Метрики:** +26.4% F1, -30x token consumption vs baselines.

**Релевантность для bmad-ralph:**
- Stage 1 (compression) ≈ review findings → distilled rules в .claude/rules/
- Stage 2 (synthesis) ≈ merging duplicate rules при retro
- **Ключевой инсайт:** compression должна быть semantic (preserving meaning), не просто truncation. Текущий подход bmad-ralph (code-review → extract patterns → atomize) уже реализует Stage 1.

**Ограничения:** Использует LanceDB + text-embedding-3-small. Не применимо directly к file-based architecture.

### 2.4 MemOS — Memory Operating System

**Источник:** [MemOS: An Operating System for Memory-Augmented Generation](https://arxiv.org/abs/2505.22101) (2025)

**Три типа памяти:**
1. **Parametric Memory** — embedded в параметрах модели (не управляемо извне)
2. **Activation Memory** — рабочая память в контексте (context window)
3. **Plaintext Memory** — файлы, документы, structured data

**MemCube** — unified abstraction для scheduling across memory types.

**Метрики:** +159% temporal reasoning vs OpenAI global memory, +38.97% accuracy, -60.95% tokens.

**Релевантность для bmad-ralph:**
- Plaintext Memory = единственный управляемый тип для CLI tool
- **Ключевой инсайт:** MemOS подтверждает, что scheduling (что загрузить и когда) важнее объёма хранилища. Claude Code .claude/rules/ с glob patterns — простая форма scheduling.

### 2.5 Reflexion — Verbal Reinforcement Learning

**Источник:** [Reflexion: Language Agents with Verbal Reinforcement Learning](https://arxiv.org/abs/2303.11366) (NeurIPS 2023)

**Ключевая идея:** Агент не обновляет weights, а накапливает текстовые reflections в episodic memory buffer. Эти reflections добавляются как context в следующих попытках.

**Релевантность для bmad-ralph:**
- Reflections ≈ review findings, distilled into rules
- Episodic memory buffer ≈ violation-tracker.md (frequency data per pattern)
- **Ключевой инсайт:** verbal reinforcement работает без fine-tuning. Именно это делает bmad-ralph — текстовые правила вместо обновления модели.

---

## 3. Industry Survey

### 3.1 Claude Code — Native Memory Architecture

**Источник:** [Claude Code Memory Docs](https://code.claude.com/docs/en/memory)

**Иерархия памяти (6 уровней):**
1. `~/.claude/CLAUDE.md` — global preferences (always loaded)
2. `/project/CLAUDE.md` — project instructions (always loaded)
3. `/project/subdir/CLAUDE.md` — directory-specific (loaded in context)
4. `.claude/rules/*.md` — glob-scoped topic files (loaded by file pattern match)
5. `.claude/settings.json` hooks — deterministic injection at events
6. Auto memory (`~/.claude/projects/*/memory/`) — Claude writes, always loaded

**Ключевые ограничения:**
- CLAUDE.md framing: "may or may not be relevant" disclaimer [R1-S6]
- Compaction может summarize rules [R2-S21-S24]
- >15 императивных правил в одном файле → <94% compliance [R2-S25]

**Best practices (подтверждены проектом):**
- CLAUDE.md < 200 строк (sweet spot: ~80)
- Topic-based sharding в .claude/rules/
- Hook-based enforcement для critical rules
- Glob-scoped loading: правила загружаются только при работе с matching файлами

### 3.2 GitHub Copilot — Spaces + Codebase Indexing

**Источник:** [GitHub Copilot Spaces](https://github.blog/ai-and-ml/github-copilot/github-copilot-spaces-bring-the-right-context-to-every-suggestion/)

**Подход:** Persistent context containers (Spaces) с repositories, issues, docs, custom instructions. Replaced Knowledge Bases (sunset Nov 2025).

**Copilot Memory:** Self-healing architecture, 7% рост PR merge rate (90% vs 83%, p < 0.00001) [R1-S3, S18].

**Релевантность:** Copilot доказал, что persistent curated context > dynamic retrieval для code tasks. Spaces ≈ CLAUDE.md + .claude/rules/ по функции.

### 3.3 Cursor — Fast Context + Codebase Graph

**Подход:** Proprietary Fast Context для rapid codebase understanding. Semantic indexing для cross-file reasoning. Tab completion с учётом project structure.

**Ограничения информации:** Закрытая архитектура, нет публичных бенчмарков memory management.

**Релевантность:** Cursor оптимизирует code completion (latency-critical), не knowledge persistence. Не применимо к bmad-ralph use case.

### 3.4 Windsurf / Devin — Cascade Engine + Wiki

**Подход:**
- Windsurf: Cascade Engine — preprocesses codebase into dependency graph, static analysis + runtime heuristics
- Devin 2.0: Wiki для auto-generating documentation, Interactive Planning

**Релевантность:**
- Devin Wiki ≈ auto-generated reference docs. Паттерн "generate docs from code" vs "maintain rules manually" — trade-off, не замена.
- **Ключевой инсайт:** industrial tools фокусируются на code understanding (graphs, indexes), не на cross-session learning. Knowledge persistence — secondary concern для IDE-based tools, primary для autonomous agents.

### 3.5 Letta (Production) — Tiered Memory Framework

**Источник:** [Letta Agent Memory](https://www.letta.com/blog/agent-memory)

**Production architecture:**
- Core Memory: always in system prompt, editable by agent
- Recall Memory: full conversation history, searchable
- Archival Memory: processed knowledge, indexed, persistent

**Метрика:** 74.0% на LoCoMo benchmark (filesystem-based agent) vs 68.5% (Mem0 Graph) [R3-S1].

**Ключевой инсайт:** File-based agent outperforms graph-based memory at this scale. Подтверждает решение bmad-ralph не использовать RAG.

---

## 4. Context Engineering Patterns (Cross-cutting)

**Источник:** [Context Engineering for Agents](https://rlancemartin.github.io/2025/06/23/context_engineering/) (LangChain), [FlowHunt Guide](https://www.flowhunt.io/blog/context-engineering/)

Четыре паттерна управления контекстом:

| Паттерн | Описание | Реализация в bmad-ralph |
|---|---|---|
| **Write** | Сохранение контекста вне context window | .claude/rules/, violation-tracker.md |
| **Select** | Загрузка нужного контекста в window | Glob-scoped rules, hooks |
| **Compress** | Оставить только необходимые токены | Atomized facts, distillation |
| **Isolate** | Разделение контекста между агентами | Subagent teams, task isolation |

**"Context rot" — confirmed pattern:** По мере роста input tokens, LLM performance деградирует. Splitting работы между subagents (каждый со своим 100K window) и compression findings назад к lead agent — более эффективно чем один длинный контекст.

---

## 5. Synthesis: Convergent Patterns

Анализ академических и индустриальных подходов выявляет **5 конвергентных паттернов**, которые повторяются независимо:

### Pattern 1: Tiered Memory Hierarchy

| Система | Tier 1 (always loaded) | Tier 2 (selective) | Tier 3 (on-demand) |
|---|---|---|---|
| MemGPT/Letta | Core Memory | Recall Memory | Archival Memory |
| Claude Code | CLAUDE.md | .claude/rules/ (glob) | Web/file search |
| MemOS | Activation Memory | — | Plaintext Memory |
| GitHub Copilot | Spaces instructions | Codebase index | Full repo search |
| **bmad-ralph** | CLAUDE.md (~80 lines) | .claude/rules/ (7 topic files) | docs/research/ |

**Вывод:** Трёхуровневая иерархия — converged best practice. bmad-ralph уже реализует этот паттерн.

### Pattern 2: Atomic, Structured Knowledge Units

| Система | Unit structure |
|---|---|
| A-MEM | Note + keywords + tags + links |
| SimpleMem | Compressed memory unit + multi-view index |
| Reflexion | Verbal reflection + episodic buffer |
| **bmad-ralph** | `- Pattern [file.go] (Story X.Y)` |

**Вывод:** bmad-ralph формат close to optimal. Можно усилить добавлением severity/frequency metadata.

### Pattern 3: Selective Loading (Scheduling)

| Система | Механизм selection |
|---|---|
| MemOS | MemCube scheduler |
| Claude Code | Glob pattern matching |
| SimpleMem | Intent-aware retrieval |
| **bmad-ralph** | Glob scope hints в заголовках rules files |

**Вывод:** Glob-based scheduling достаточен при <500 записей. Semantic retrieval не оправдан.

### Pattern 4: Compression / Distillation

| Система | Механизм |
|---|---|
| SimpleMem | Semantic structured compression |
| MemGPT | LLM-driven summarization |
| Reflexion | Verbal reflection → buffer |
| **bmad-ralph** | Code review → extract → atomize → merge at retro |

**Вывод:** bmad-ralph pipeline (review → extract → atomize) — validated form of semantic compression. Отсутствует automated periodic compression operational knowledge.

### Pattern 5: Deterministic Enforcement

| Система | Механизм |
|---|---|
| Claude Code hooks | SessionStart, PreToolUse injection |
| MemGPT | Self-directed memory editing via tools |
| GitHub Copilot | Self-healing memory architecture |
| **bmad-ralph** | SessionStart critical rules + PreToolUse checklist |

**Вывод:** Hook-based enforcement — uniquely available in Claude Code ecosystem. bmad-ralph already leverages this advantage.

---

## 6. Recommended Architecture

### 6.1 Overview: "Enhanced File-Based Tiered Memory"

Архитектура сохраняет текущий подход bmad-ralph и усиливает его на основании convergent patterns:

```
┌─────────────────────────────────────────────────────┐
│                    TIER 1: CORE                      │
│  Always loaded. Budget: <100 lines, ~1500 tokens     │
│                                                       │
│  CLAUDE.md (~80 lines)                                │
│  ├── Project overview                                 │
│  ├── Critical constraints (env, deps, naming)         │
│  ├── Pointers to Tier 2 files                         │
│  └── Knowledge extraction protocol                    │
│                                                       │
│  memory/MEMORY.md (~200 lines)                        │
│  ├── Project status                                   │
│  ├── Critical learnings                               │
│  └── Metrics trends                                   │
│                                                       │
│  SessionStart hook (15 critical rules)                │
│  └── Bypasses CLAUDE.md framing problem               │
├───────────────────────────────────────────────────────┤
│                  TIER 2: SELECTIVE                     │
│  Glob-scoped. Budget: ~15 rules per file              │
│                                                       │
│  .claude/rules/                                       │
│  ├── go-testing-patterns.md (index → 7 topic files)   │
│  ├── test-naming-structure.md (12 rules)              │
│  ├── test-error-patterns.md (11 rules)                │
│  ├── test-assertions-base.md (23 rules)               │
│  ├── test-assertions-prompt.md (12 rules)             │
│  ├── test-mocks-infra.md (22 rules)                   │
│  ├── test-templates-review.md (14 rules)              │
│  ├── code-quality-patterns.md (28 rules)              │
│  └── wsl-ntfs.md (platform-specific)                  │
│                                                       │
│  Scope hints: `# Scope: <description>` в каждом файле │
│  Loading: автоматически по glob match                  │
├───────────────────────────────────────────────────────┤
│                  TIER 3: REFERENCE                     │
│  On-demand. Not auto-loaded                           │
│                                                       │
│  docs/research/ (R1, R2, R3 reports)                  │
│  docs/epics/ (epic specifications)                    │
│  docs/architecture/ (system design)                   │
│  .claude/violation-tracker.md (frequency data)        │
│                                                       │
│  Access: agent reads when needed for specific task    │
└───────────────────────────────────────────────────────┘
```

### 6.2 Обоснование каждого компонента

| Компонент | Обоснование | Академический аналог |
|---|---|---|
| CLAUDE.md <100 lines | Context rot: <94% compliance при >15 rules [R2-S25] | MemGPT Core Memory |
| SessionStart hook | Bypasses framing disclaimer; deterministic [R2-S6] | MemOS Activation Memory |
| .claude/rules/ sharding | ~15 rules/file = compliance sweet spot [R2-S25] | A-MEM topic clustering |
| Glob scope hints | Selective loading → reduced context pressure | SimpleMem intent-aware retrieval |
| Atomic fact format | `- Pattern [file] (Story)` = searchable, mergeable | A-MEM atomic notes |
| Violation tracker | Frequency data → escalation → enforcement priority | Reflexion episodic buffer |
| Code review pipeline | Extract → atomize → merge = semantic compression | SimpleMem Stage 1 |
| Retro merge | Periodic dedup + consolidation = synthesis | SimpleMem Stage 2 |

### 6.3 Что НЕ нужно добавлять

| Отвергнутый подход | Причина | Источник |
|---|---|---|
| RAG / Vector search | <500 записей, overhead не оправдан, 74% file-based vs 68.5% graph [R3-S1] | R3, Letta benchmark |
| Graph database | Требует зависимость, overkill для ~122 patterns | R3, MemOS |
| MCP Memory Server | Требует Node.js runtime, 56% skip rate [R2-S31] | R2, R3 |
| LLM-driven memory management | Дорого (LLM call на каждый memory op), unreliable в CLI | A-MEM, MemGPT |
| Embedding-based retrieval | Требует external API (OpenAI/Ollama), добавляет latency | SimpleMem, chromem-go |
| Automated distillation (LLM) | LLM distillation unreliable без human verification [R1] | Deferred to future |

### 6.4 Growth Path

| Масштаб | Рекомендация |
|---|---|
| <200 rules (текущий) | Текущая архитектура optimal |
| 200-500 rules | Добавить sub-topic sharding (e.g., test-assertions-base → test-assertions-count, test-assertions-symmetric) |
| 500-1000 rules | MCP Memory Server (JSONL knowledge graph) + Claude Code native integration |
| >1000 rules | RAG с chromem-go (CGO_ENABLED=0) + embedding API |

---

## 7. Trade-offs

### 7.1 Simplicity vs Sophistication

**Выбор: Simplicity.**

File-based approach проще, но:
- (+) Zero dependencies
- (+) Human-readable, editable, git-tracked
- (+) Proven: 74% LoCoMo vs 68.5% graph-based
- (-) No semantic search (compensated by glob-scoped loading)
- (-) Manual curation required (compensated by code-review pipeline)

### 7.2 Accumulation vs Distillation

**Выбор: Hybrid.**

- **Structural rules** (coding conventions, naming, testing patterns): accumulate, не distill. Они стабильны и формируют vocabulary.
- **Operational knowledge** (specific bug patterns, error messages): distill periodically при retro. Они устаревают и создают noise.
- **Metrics/status**: overwrite (latest only). MEMORY.md — snapshot, не log.

### 7.3 Automation vs Human Control

**Выбор: Human-in-the-loop.**

- Automated: extraction (code-review → findings), loading (glob-scoped), enforcement (hooks)
- Human-controlled: curation (retro merge decisions), escalation (violation thresholds), architecture (file structure)
- **Обоснование:** LLM-automated distillation unreliable [R1], human verification essential для quality

### 7.4 Granularity of Loading

**Выбор: File-level granularity (current).**

- File-per-topic (current): simple, predictable, glob-matchable
- Rule-level granularity: would require custom loader, over-engineering
- Section-level: partial loading not supported by Claude Code natively

### 7.5 Context Budget Allocation

**Рекомендованное распределение (из 200K):**

| Component | Budget | % |
|---|---|---|
| System prompt + CLAUDE.md | ~5K tokens | 2.5% |
| Auto memory (MEMORY.md) | ~3K tokens | 1.5% |
| SessionStart hook rules | ~2K tokens | 1% |
| .claude/rules/ (loaded set) | ~5-10K tokens | 2.5-5% |
| **Total knowledge overhead** | **~15-20K tokens** | **7.5-10%** |
| Available for task work | ~180K tokens | 90% |

Это within sweet spot: достаточно для compliance, не создаёт context rot pressure.

---

## 8. Evidence Table

| ID | Source | Type | Quality | Key Finding |
|---|---|---|---|---|
| MemGPT | [arxiv.org/abs/2310.08560](https://arxiv.org/abs/2310.08560) | Academic (NeurIPS 2023) | A | Tiered memory (core/recall/archival) validated |
| A-MEM | [arxiv.org/abs/2502.12110](https://arxiv.org/abs/2502.12110) | Academic (NeurIPS 2025) | A | Atomic notes + structured metadata + linking |
| SimpleMem | [arxiv.org/abs/2601.02553](https://arxiv.org/abs/2601.02553) | Academic (2026) | A | 3-stage pipeline: compress → synthesize → retrieve. +26.4% F1 |
| MemOS | [arxiv.org/abs/2505.22101](https://arxiv.org/abs/2505.22101) | Academic (2025) | A | Memory scheduling > storage volume. +159% temporal reasoning |
| Reflexion | [arxiv.org/abs/2303.11366](https://arxiv.org/abs/2303.11366) | Academic (NeurIPS 2023) | A | Verbal reinforcement without fine-tuning. Episodic memory buffer |
| Claude Code | [code.claude.com/docs/en/memory](https://code.claude.com/docs/en/memory) | Official docs | A | 6-level memory hierarchy, glob-scoped rules, hooks |
| Copilot Spaces | [github.blog](https://github.blog/ai-and-ml/github-copilot/github-copilot-spaces-bring-the-right-context-to-every-suggestion/) | Official blog | A | Persistent curated context > dynamic retrieval |
| Letta | [letta.com/blog/agent-memory](https://www.letta.com/blog/agent-memory) | Industry (OSS) | B | 74.0% LoCoMo file-based vs 68.5% graph-based |
| Context Eng | [rlancemartin.github.io](https://rlancemartin.github.io/2025/06/23/context_engineering/) | Industry blog | B | Write/Select/Compress/Isolate framework |
| R1 bmad-ralph | docs/research/knowledge-extraction-*.md | Internal | B | Context rot 30-50%, CLAUDE.md framing problem |
| R2 bmad-ralph | docs/research/knowledge-enforcement-*.md | Internal | B | Triple barrier, hook-based enforcement, >15 rules = <94% compliance |
| R3 bmad-ralph | docs/research/alternative-knowledge-methods-*.md | Internal | B | RAG unnecessary <500 records, file-based optimal |

---

## 9. Conclusions

1. **Текущая архитектура bmad-ralph уже близка к оптимальной** — она реализует все 5 convergent patterns (tiered hierarchy, atomic units, selective loading, compression pipeline, deterministic enforcement).

2. **Главные gaps — не архитектурные, а operational:**
   - Отсутствие automated periodic compression operational knowledge
   - Violation tracker не интегрирован в enforcement loop (thresholds → hook injection)
   - Нет severity/frequency metadata на atomic facts для priority loading

3. **Оптимальная стратегия — "deepen, don't pivot":** усиливать file-based approach, не переключаться на RAG/graph/MCP. Академические данные и production benchmarks однозначно подтверждают file-based superiority при текущем масштабе.

4. **Context budget management — ключевой рычаг:** 7.5-10% бюджета на knowledge (~15-20K tokens) оставляет 90% для task work. Рост knowledge base выше ~30K tokens потребует architectural pivot.
