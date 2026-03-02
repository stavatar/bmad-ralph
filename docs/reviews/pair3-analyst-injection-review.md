# Пара 3 — Аналитик: Injection, Serena Integration, Final Integration

**Scope:** Stories 6.2, 6.7, 6.8
**Дата:** 2026-03-02
**Раунд:** 3 (adversarial review v5)

---

## 1. НАХОДКИ

### F1 (HIGH): Inject ALL из LEARNINGS.md — отсутствие верхней границы injection

**Что:** Решение v5-4 убрало "last 20%" filtering и заменило на "inject ALL". Но при этом v5-5 убрал injection circuit breaker (только stderr warning). В итоге:
- LEARNINGS.md растёт без ограничений
- Injection всегда включает ВСЁ содержимое
- Единственная защита — stderr warning, который не блокирует

**Почему это проблема:**
- Research: "Lost in the Middle" (Liu et al., 2023/TACL 2024) — производительность LLM падает >30% когда релевантная информация в середине контекста
- Research: SFEIR CLAUDE.md Optimization — ~15 правил = 94% compliance. 300+ строк = 60-100+ правил → beyond threshold
- Research: JetBrains "The Complexity Trap" (NeurIPS 2025) — observation masking (т.е. selective injection) лучше full injection: +2.6% solve rate при -52% cost
- Research: Chroma Context Rot (18 моделей) — degradation grows with context length
- Реальный risk scenario: 500 строк raw LEARNINGS.md = ~5K tokens. С ralph-*.md = ещё ~3-8K. Итого 8-13K tokens знаний = 6-10% effective context, что приближается к порогу деградации

**Severity: HIGH** — не CRITICAL потому что distillation должна сдерживать рост, но если distillation fails/skipped — soft degradation без hard stop.

**Опции:**
| Опция | Pros | Cons |
|-------|------|------|
| A: Добавить injection cap (200 строк), truncate oldest | Hard guarantee, simple | Потеря знаний при overflow |
| B: Injection cap + warning, но inject first N строк reversed | Newest-first, hard guarantee | Потеря старых знаний (но они в ralph-*.md) |
| C: Оставить как есть + trust distillation | Простота, zero knowledge loss | No safety net при stuck distillation |

**Рекомендация:** Опция B — injection cap = learnings_budget (200 строк по умолчанию). При overflow inject first `budget` строк из reversed content (= самые новые). Старое уже в ralph-*.md. Это consistent с двухуровневой архитектурой.

---

### F2 (HIGH): Self-review section — +12% claim из одного исследования, не reproduced

**Что:** Story 6.2 AC включает self-review step в execute prompt:
> "Re-read the top 5 most recent learnings. For each modified file, verify that the patterns from learnings are applied correctly."

Обосновано ссылкой на Live-SWE-agent (+12% quality from single reflection prompt).

**Почему это проблема:**
- Live-SWE-agent +12% — конкретное исследование в конкретном контексте (autonomous SWE benchmark), не обязательно transferable к knowledge reflection
- Self-review добавляет ~50 tokens в prompt + поведенческие инструкции, которые Claude может или не может следовать
- **Нет baseline для измерения:** Story 6.9 (trend tracking) отслеживает findings_per_task, но нет mechanism отличить эффект self-review от эффекта injected knowledge
- Research: Cursor rules — `alwaysApply: true` = единственный надёжный способ гарантировать применение правил. Self-review = дополнительная indirect инструкция, compliance неизвестна

**Severity: HIGH** — не потому что self-review вреден (он скорее полезен), а потому что это архитектурное решение с непроверяемым impact claim. Если не работает — это dead code в prompt, засоряющий context.

**Опции:**
| Опция | Pros | Cons |
|-------|------|------|
| A: Оставить, не claim +12% | Потенциально полезно | Непроверяемо |
| B: Убрать self-review | Меньше prompt tokens | Потеря возможного improvement |
| C: Добавить A/B metric (6.9) | Measurable | Complexity + low sample |

**Рекомендация:** Опция A — оставить self-review, убрать claim "+12% quality" из обоснований. Это дешёвая (~50 tokens) инструкция с потенциальным upside. Но НЕ строить архитектурные решения вокруг этого эффекта.

---

### F3 (HIGH): buildKnowledgeReplacements() — shared DRY скрывает различия между call sites

**Что:** Story 6.2 определяет `buildKnowledgeReplacements(projectRoot string) (map[string]string, error)` — shared function для 3 call sites (initial execute, retry execute, review).

**Почему это проблема:**
- **Initial execute** — нужен полный набор: LEARNINGS + ralph-*.md + Serena hint + HasLearnings bool
- **Retry execute** — то же + потенциально refresh (файл мог измениться после Claude wrote learnings)
- **Review** — нужен LEARNINGS + ralph-*.md, но НЕ нужен self-review step (review = reviewer, не implementer)

При shared function:
1. Review получит self-review instructions через HasLearnings=true, хотя review не должен "re-read learnings и verify patterns" — это execute concern
2. Retry execute может получить stale content если не re-read LEARNINGS.md (shared function может cache)
3. Нельзя customise injection per call site без breaking DRY

**Severity: HIGH** — self-review в review prompt = confusion (reviewer не применяет patterns, а проверяет код). Либо нужен per-site override, либо HasLearnings должен быть false для review.

**Опции:**
| Опция | Pros | Cons |
|-------|------|------|
| A: Shared function + per-site TemplateData override | DRY для file reads, custom bool flags | Slightly more complex |
| B: Separate functions per call site | Explicit, no confusion | DRY violation |
| C: Shared + HasLearnings only for execute | Minimal change | Need to document |

**Рекомендация:** Опция A — `buildKnowledgeReplacements()` returns map (file content). Caller sets TemplateData bools separately. Review sets `HasLearnings: false` even if content injected (no self-review prompt). Execute sets `HasLearnings: true`.

---

### F4 (MEDIUM): Serena MCP detection — brittle JSON parsing

**Что:** Story 6.7 предлагает детектировать Serena через парсинг `.claude/settings.json` и `.mcp.json`.

**Почему это проблема:**
- `.claude/settings.json` формат не документирован Anthropic — может меняться без предупреждения
- `.mcp.json` — формат MCP конфигурации, тоже без stability guarantees
- Research: CVE-2025-54135 (Cursor) — programmatic config file editing = confirmed risk class. Хотя ralph только ЧИТАЕТ (не пишет), зависимость от undocumented format = fragile
- **Что искать в JSON?** Story 6.7 не определяет: ключ "serena"? URL с "serena"? Любой MCP server с определённым capabilities? Regex match? Точная строка?
- Serena может быть переименована, forked, или заменена другим code indexer MCP server

**Severity: MEDIUM** — detection = best-effort (Available returns false on any error), failure graceful. Но fragility создаёт maintenance burden.

**Опции:**
| Опция | Pros | Cons |
|-------|------|------|
| A: Exact string match "serena" в MCP config | Simple, specific | Breaks on rename |
| B: Config field `code_indexer_mcp: "serena"` в ralph.yaml | Explicit, user-controlled | Manual config |
| C: Detect ANY MCP server presence | Broader applicability | False positives |

**Рекомендация:** Опция B — `code_indexer_mcp` config field (default: "serena"). User контролирует, не зависит от JSON format. Available() = проверяет наличие named server в config files. Более robust чем hardcoded string.

---

### F5 (MEDIUM): CodeIndexerDetector interface — нет version/capability negotiation

**Что:** Минимальный интерфейс `{Available(projectRoot) bool, PromptHint() string}` — всего 2 метода.

**Почему это проблема:**
- PromptHint() hardcoded: "If Serena MCP tools available, use them for code navigation"
- Что если Serena изменит API? Hint будет misleading
- Что если другой code indexer (e.g., Sourcegraph Cody MCP) будет доступен? Один interface, один hint
- **Нет capability detection:** Serena может поддерживать symbol search но не go-to-definition. Hint не отражает это

**Severity: MEDIUM** — YAGNI concern оправдан для v1, но interface слишком минимален для growth. Нет даже Name() метода.

**Опции:**
| Опция | Pros | Cons |
|-------|------|------|
| A: Добавить Name() string | Future-proof | YAGNI |
| B: Оставить 2 метода | Minimal, ship fast | Needs redesign later |
| C: PromptHint(capabilities) | Flexible | Over-engineering |

**Рекомендация:** Опция B для v1, но добавить TODO comment для Growth phase. 2 метода достаточно.

---

### F6 (MEDIUM): Story 6.8 integration tests — 12 scenarios недостаточно для 86 AC

**Что:** Story 6.8 определяет 12 integration test scenarios для финального integration test всех 6 эпиков.

**Почему это проблема:**
- 86 AC across 11 stories → 12 integration tests = ~14% coverage ratio
- **Missing scenarios:**
  - Concurrent LEARNINGS.md write + read (Claude writes during session while Go snapshots)
  - LEARNINGS.md с `{{` в содержимом (template injection protection)
  - Empty ralph-*.md после distillation (zero categories)
  - ValidateLearnings с ONLY stale entries (все entries invalid → HasLearnings = false)
  - Budget overflow + distillation failure + auto mode → CB skip + warning
  - Multiple sequential tasks with knowledge accumulation across all
- Нет edge case для primacy zone ordering verification (distilled BEFORE raw BEFORE guardrails)
- Нет теста на размер injected content (что prompt не превышает context window)

**Severity: MEDIUM** — integration tests = last line of defense. Missing scenarios = gaps, но unit tests (Stories 6.1-6.7) should cover most.

**Рекомендация:** Добавить минимум 3 сценария:
1. Template injection protection (`{{` в LEARNINGS.md)
2. All stale entries → HasLearnings false
3. Budget overflow cascade (overflow → distill fail → CB → continue)

---

### F7 (LOW): Primacy zone placement — нет тестового подтверждения ordering

**Что:** Story 6.2 определяет ordering: distilled (ralph-*.md) → raw (LEARNINGS.md) → guardrails. Placement "AFTER Format Reference, BEFORE 999-Rules" — primacy zone.

**Почему это проблема:**
- Research: "Lost in the Middle" — U-shaped attention curve. Beginning and end positions get most attention
- Knowledge placed в середину execute.md (после Format, перед Guardrails) может попасть в "lost zone"
- Нет integration test подтверждающего ordering в assembled prompt

**Severity: LOW** — placement reasonable (before guardrails = "near end"), и golden file tests должны catch ordering issues.

---

## 2. RESEARCH INSIGHTS — Лучшие практики Knowledge Injection

### 2.1. Cursor Rules (.cursor/rules)

- **Mechanism:** MDC (markdown) files с frontmatter: `alwaysApply: true/false`, `globs: ["*.py"]`
- **Injection:** Prepended to system prompt при каждом запросе
- **Key finding:** `alwaysApply: true` — единственный надёжный способ. Glob-based rules ненадёжны (Cursor 2.0 bugs)
- **Research (arXiv 2512.18925):** Empirical study 401 repos — 5 themes: Convention, Guideline, Project, Example, LLM Directive
- **Relevance для bmad-ralph:** ralph-*.md с YAML globs = аналог Cursor glob rules. Та же проблема — reliability зависит от loader. ralph-misc.md (always loaded) = аналог `alwaysApply: true`

### 2.2. GitHub Copilot Instructions

- **Mechanism:** `.github/copilot-instructions.md` — project-wide instructions
- **Injection:** Full file injected into system prompt
- **Key feature:** JIT citation validation (+7% PR merge rate), self-healing memories
- **Relevance:** bmad-ralph JIT validation (os.Stat) = inspired by Copilot approach

### 2.3. Aider Conventions

- **Mechanism:** `CONVENTIONS.md` loaded via `--read` flag или `.aider.conf.yml: read: [CONVENTIONS.md]`
- **Injection:** Marked read-only, cached if prompt caching enabled
- **Key feature:** Functions as system prompt for every edit. Simple, file-based
- **Relevance:** LEARNINGS.md = аналог CONVENTIONS.md. Aider: full file injection. bmad-ralph: full file injection (v5-4). Convergent design

### 2.4. Continue.dev Context Providers

- **Mechanism:** `.continue/rules` + context providers (@codebase, @docs)
- **Injection:** RAG-based retrieval for large codebases, file-based for rules
- **Key feature:** Extensible context provider system
- **Relevance:** ralph не использует RAG (confirmed unnecessary для file-based CLI by verification team)

### 2.5. Claude Code CLAUDE.md

- **Mechanism:** CLAUDE.md + `.claude/rules/*.md` — "naively dropped into context up front"
- **Injection:** Full content at conversation start. `.claude/rules/` files loaded by glob match
- **Key feature:** Hybrid model — static instructions + JIT retrieval via glob/grep tools
- **Relevance:** ralph-*.md goes into `.claude/rules/` = leverages existing Claude Code infrastructure. No custom loader needed. Smart architecture choice

### 2.6. Context Size Research

| Source | Key Finding | Threshold |
|--------|------------|-----------|
| SFEIR CLAUDE.md Optimization | ~15 rules = 94% compliance | ~15 rules |
| Lost in the Middle (TACL 2024) | >30% performance drop in middle positions | Position-dependent |
| JetBrains Complexity Trap (NeurIPS 2025) | Observation masking +2.6% solve rate, -52% cost | Less = better |
| Chroma Context Rot | Degradation grows with context | Linear decay |
| Anthropic Context Engineering | "smallest set of high-signal tokens" | Quality > quantity |

**Synthesis:** Все sources сходятся: **меньше = лучше.** Inject минимум high-signal content. bmad-ralph's progressive disclosure (T1/T2/T3 tiers) — правильный подход. Но "inject ALL from LEARNINGS.md" без cap = risk.

---

## 3. ПОДТВЕРЖДЕНО — что правильно

### 3.1. Stage 2 injection для user content (__LEARNINGS_CONTENT__)
Правильный выбор. Stage 1 (text/template) для structure, Stage 2 (strings.Replace) для user content. Защита от `{{` в LEARNINGS.md. Proven pattern в проекте (5 эпиков).

### 3.2. Multi-file ralph-*.md в .claude/rules/
Использование существующей Claude Code инфраструктуры — excellent architectural decision. Не нужен custom loader. `.claude/rules/` = proven (9 файлов, 122 паттерна). Convergent с Cursor rules, Continue rules.

### 3.3. HasLearnings bool conditional
Правильная абстракция: Stage 1 bool для structural conditional, Stage 2 string для content. Clean separation.

### 3.4. JIT citation validation (os.Stat only)
M9 верное решение — os.Stat достаточно для v1. Line range validation = O(N×file_read) на WSL/NTFS, не оправдано. Inspired by Copilot (+7% merge rate).

### 3.5. Serena = MCP server (C3)
Правильное исправление CRITICAL бага из v3. MCP detection через config files — единственный viable подход. Minimal interface — YAGNI-compliant.

### 3.6. Reverse read ordering (L3)
Newest-first injection = exploit recency bias в LLM attention. Research-backed (Lost in the Middle — end position = high attention).

### 3.7. Prompt placement в primacy zone
Knowledge после Format Reference, перед Guardrails — near-end position. Consistent с research: end of context gets high attention.

### 3.8. Self-review conditional на HasLearnings
Не показывать self-review когда нет knowledge = correct. Zero overhead для новых проектов.

### 3.9. Trend tracking вместо A/B (Story 6.9)
Правильное упрощение. A/B на 5-15 задачах = статистически бессмысленно. Trend = достаточный сигнал.

### 3.10. buildKnowledgeReplacements DRY
DRY threshold (3 call sites) достаточен. Shared file reading logic = correct. Но нужен per-site TemplateData customization (F3).

---

## 4. СВОДНАЯ ТАБЛИЦА

| # | Severity | Находка | Затронутые Stories | Рекомендация |
|---|----------|---------|-------------------|--------------|
| F1 | **HIGH** | Inject ALL без верхней границы | 6.2 | Injection cap = learnings_budget, newest-first |
| F2 | **HIGH** | Self-review +12% claim необоснован | 6.2 | Оставить, убрать claim |
| F3 | **HIGH** | Shared function скрывает site-specific needs | 6.2 | Shared reads + per-site TemplateData |
| F4 | MEDIUM | Serena detection — brittle JSON parsing | 6.7 | Config field `code_indexer_mcp` |
| F5 | MEDIUM | CodeIndexerDetector слишком минимален | 6.7 | OK для v1, TODO для Growth |
| F6 | MEDIUM | 12 integration tests < 86 AC | 6.8 | +3 edge case scenarios |
| F7 | LOW | Primacy zone ordering без теста | 6.2, 6.8 | Golden file tests cover |

---

*Отчёт подготовлен аналитиком Пары 3. Deep research: 6 web searches, 12+ sources. Архитектору Пары 3 для cross-review и consensus.*
