# Epic 6: Конкурентный анализ — Knowledge Management в AI Coding Agents

**Совместный consensus document**
**Пара 5:** analyst-competitive + architect-competitive
**Дата:** 2026-02-28
**Scope:** Сравнение bmad-ralph Knowledge Management (Epic 6) с конкурентами: GitHub Copilot, Cursor, Aider, Cline, Continue, SWE-agent
**Восстановлено из:** сводка предыдущей сессии (оригинал передан через SendMessage)

---

## Executive Summary

bmad-ralph **ЛИДИРУЕТ** в 3 из 6 ключевых dimensions knowledge management. **ON PAR** в 2 dimensions. **Отстаёт** в 0 dimensions. 1 dimension — unique (нет у конкурентов для сравнения).

**Вердикт:** "Ship as-is" после исправления CRITICAL + HIGH issues, найденных другими парами. Никаких фундаментальных архитектурных изменений на основе конкурентного анализа не требуется.

---

## Methodology

Сравнение по 6 dimensions:
1. **Knowledge Extraction** — как система извлекает знания из рабочих сессий
2. **Knowledge Distillation** — как сжимает/оптимизирует знания
3. **Knowledge Injection** — как доставляет знания в контекст
4. **Quality Gates** — как обеспечивает качество знаний
5. **Violation Tracking** — как отслеживает повторяющиеся ошибки
6. **Progressive Disclosure** — как управляет объёмом контекста

Источники: публичная документация, GitHub repos, research papers (R1-R3), web research 2025-2026.

---

## Dimension 1: Knowledge Extraction

| System | Approach | Strengths | Weaknesses |
|--------|----------|-----------|------------|
| **bmad-ralph** | Multi-source: review findings + resume failures + session insights. Structured entries with category/topic/citation | 3 extraction sources; citation tracking; category-based routing | Зависит от LLM compliance с форматом |
| GitHub Copilot | `.github/copilot-instructions.md` — manual. Learning from accepted/rejected suggestions (implicit) | Massive scale; implicit learning | No explicit extraction; no project-level learning file |
| Cursor | `.cursor/rules` + `.cursorrules` — manual. Auto-indexing codebase for context | Good codebase awareness | Manual knowledge; no auto-extraction from failures |
| Aider | `.aider.conf.yml` conventions — manual. Git-aware (learns from commit history) | Git integration | No automated knowledge extraction |
| Cline | `.clinerules` — manual | Simple | Fully manual, no automation |
| Continue | `.continue/rules` — manual. Indexing via embeddings | Good indexing | Manual rules only |
| SWE-agent | AgentLab framework — learned strategies from benchmark runs | Research-grade learning | Not production; no project-level persistence |

**bmad-ralph position: ЛИДИРУЕТ**
- Единственная система с автоматическим извлечением из 3 источников
- Конкуренты полагаются на manual curation

---

## Dimension 2: Knowledge Distillation

| System | Approach | Details |
|--------|----------|---------|
| **bmad-ralph** | 3-layer: Go semantic dedup (0 tokens) → LLM compression (~8K tokens) → circuit breaker + validation | Automatic, multi-layer, with fallback |
| GitHub Copilot | None — instructions are static manual files | — |
| Cursor | None — rules are static; codebase index refreshed periodically | Index != distillation |
| Aider | None — conventions are static | — |
| Cline | None | — |
| Continue | Embedding re-indexing (not semantic distillation) | Different purpose |
| SWE-agent | AgentLab strategy refinement (research only) | Not comparable |

**bmad-ralph position: ЛИДИРУЕТ (уникально)**
- Ни один конкурент не имеет multi-layer distillation pipeline
- 3-layer architecture (Go dedup → LLM compression → safety nets) — unique в индустрии
- Circuit breaker для LLM distillation — не встречается нигде

---

## Dimension 3: Knowledge Injection

| System | Approach | Context Cost |
|--------|----------|-------------|
| **bmad-ralph** | File-based: LEARNINGS.md → prompt Stage 2 replacement. JIT validation (stale removal). Conditional self-review | ~3-6% context (200-600 lines) |
| GitHub Copilot | System prompt injection of `.github/copilot-instructions.md` | Fixed, small |
| Cursor | Rules injected into system prompt + RAG from codebase index | Variable, can be large |
| Aider | Repo map + conventions in system prompt | ~5-15% context |
| Cline | Rules in system prompt | Fixed, small |
| Continue | Rules + RAG retrieval from embeddings | Variable |

**bmad-ralph position: ON PAR**
- File-based injection сравним с Copilot/Cursor rules injection
- JIT validation — уникальное преимущество (stale removal)
- Self-review — уникальное преимущество (quality feedback loop)
- Injection circuit breaker — уникальная safety net

---

## Dimension 4: Quality Gates

| System | Quality Mechanism |
|--------|-------------------|
| **bmad-ralph** | 6 quality gates (G1-G6): format, citation, dedup, budget, cap, min content. + ValidateDistillation (7 criteria) |
| GitHub Copilot | None on instructions (manual file) |
| Cursor | Syntax validation on rules files |
| Aider | None |
| Cline | None |
| Continue | Schema validation on config |
| SWE-agent | Benchmark-based evaluation (research) |

**bmad-ralph position: ЛИДИРУЕТ**
- 6 quality gates на запись + 7 criteria на distillation = 13 quality checkpoints
- Ни один конкурент не имеет программных quality gates на knowledge content
- ValidateDistillation с backup/restore — уникальный safety mechanism

---

## Dimension 5: Violation Tracking

| System | Tracking |
|--------|---------|
| **bmad-ralph** | `.claude/violation-tracker.md` — frequency counts, escalation thresholds, T1 promotion triggers |
| GitHub Copilot | None (implicit via accepted/rejected suggestions at scale) |
| Cursor | None |
| Aider | None |
| Cline | None |
| Continue | None |

**bmad-ralph position: ЛИДИРУЕТ (уникально)**
- Единственная система с explicit violation tracking
- `[freq:N]` tags + T1 promotion — closed-loop enforcement
- Escalation thresholds — прогрессивное усиление правил

---

## Dimension 6: Progressive Disclosure

| System | Approach |
|--------|---------|
| **bmad-ralph** | T1 (ralph-critical.md, always loaded) → T2 (ralph-{category}.md, glob-scoped) → T3 (LEARNINGS.md, full) |
| GitHub Copilot | Single file, always loaded |
| Cursor | Multiple rule files, all loaded |
| Aider | Single config, always loaded |
| Cline | Single file |
| Continue | Multiple files, RAG-based selection |

**bmad-ralph position: ON PAR with Continue**
- T1/T2/T3 tiering сравнимо с Continue's RAG-based selection
- Glob-scoped loading (Claude Code native) — efficient
- Но Continue использует embeddings для relevance — потенциально точнее

---

## Сводная матрица

| Dimension | bmad-ralph | Copilot | Cursor | Aider | Cline | Continue |
|-----------|-----------|---------|--------|-------|-------|----------|
| Extraction | **LEAD** | Weak | Weak | Weak | Weak | Weak |
| Distillation | **LEAD (unique)** | None | None | None | None | None |
| Injection | On par | Good | Good | Good | Basic | Good |
| Quality Gates | **LEAD** | None | Basic | None | None | Basic |
| Violation Tracking | **LEAD (unique)** | None | None | None | None | None |
| Progressive Disclosure | On par | Basic | Basic | Basic | Basic | Good |

---

## Конкурентные риски

### Что может догнать bmad-ralph

1. **Cursor / Continue + embeddings** — если добавят auto-extraction из сессий, быстро догонят по Dimension 1
2. **GitHub Copilot Workspace** — имеет ресурсы для внедрения knowledge management at scale
3. **Aider git integration** — commit history = implicit knowledge base, может быть формализован

### Уникальные преимущества bmad-ralph (трудно скопировать)

1. **3-layer distillation pipeline** — architectural innovation, не просто feature
2. **Quality gates на content** — требует deep integration с write path
3. **Violation tracking → T1 promotion** — closed-loop, accumulates value over time
4. **Circuit breaker для LLM operations** — infrastructure pattern applied to knowledge management

---

## Рекомендации

### Что делать (подтверждено анализом)

1. **Ship as-is** после исправления CRITICAL/HIGH issues — конкурентная позиция сильная
2. **Документировать уникальность** — 3-layer distillation и violation tracking = marketing differentiators
3. **Не усложнять** — текущая архитектура достаточна для v1

### Чего НЕ делать

1. **Не добавлять embeddings/RAG** — R3 research подтверждает: file-based injection достаточен для <500 entries (74.0% LoCoMo). Embeddings = over-engineering для v1
2. **Не копировать Cursor's codebase indexing** — Serena/MCP integration покрывает этот gap через prompt-based approach
3. **Не добавлять multi-user support** — bmad-ralph = single-developer tool, конкуренты тоже single-user

### Что рассмотреть в Growth phase

1. **Embedding-based retrieval** для LEARNINGS.md >500 entries (если рост подтвердится)
2. **Cross-project knowledge sharing** — ни один конкурент не делает этого; potential first-mover advantage
3. **Benchmark integration** — SWE-bench compatibility для measuring knowledge effectiveness

---

## Conclusion

bmad-ralph Epic 6 knowledge management architecture — **state-of-the-art для single-developer AI coding agents**. Ни один существующий инструмент не имеет comparable 3-layer distillation pipeline или programmatic quality gates на knowledge content.

Основной конкурентный риск — не другие инструменты, а **собственная сложность**: 59 AC, 8 stories, multiple interacting subsystems. Упрощение реализации (без потери ключевых преимуществ) — главный приоритет.

---

*Документ восстановлен из сводки предыдущей сессии. Оригинальные findings были переданы через SendMessage и вошли в итоговую сводку, но не были записаны в файл агентами.*
