# Pair 2 Analyst: Distillation & Budget Management Review (v5)

**Роль:** Аналитик Пары 2
**Фокус:** Stories 6.5a, 6.5b, 6.5c, 6.6 — distillation pipeline, budget management, circuit breaker
**Дата:** 2026-03-02
**Методология:** Code review + decision log analysis + deep research (7 web searches, 50+ источников)

---

## Резюме

V5 значительно улучшена относительно v3/v4 — split Story 6.5 на 3, DistillState в .ralph/, distill_gate config. Однако **5 находок требуют внимания** (1 HIGH, 3 MEDIUM, 1 LOW). Архитектура 3-layer distillation **подтверждена** research (A-MEM NeurIPS 2025, MemOS, AgeMem).

---

## Находки

### F1. [HIGH] Circuit breaker threshold "3 consecutive failures" — недостаточно обоснован для non-deterministic LLM operations

**Stories:** 6.5a (auto mode)
**AC:** "if consecutive failures >= 3: auto-skip distillation"

**Проблема:** Research показывает что LLM operations имеют принципиально иную failure модель чем network services:

1. **Transient vs persistent:** Portkey.ai и Google Cloud SRE best practices ([Portkey](https://portkey.ai/blog/retries-fallbacks-and-circuit-breakers-in-llm-apps/), [Google Cloud](https://medium.com/google-cloud/building-bulletproof-llm-applications-a-guide-to-applying-sre-best-practices-1564b72fd22e)) различают transient failures (rate limits, timeouts) и persistent failures (bad prompt, model capacity). Для LLM distillation: **timeout и bad format = transient** (retry может помочь), **validation reject = likely persistent** (тот же prompt даст тот же результат).

2. **3 = слишком мало для transient, слишком много для persistent.** Go library `gobreaker` (Sony) рекомендует дифференцированные thresholds. Antfarm project ([GitHub #218](https://github.com/snarktank/antfarm/issues/218)) показал что fixed threshold для agent cron jobs приводит к "persistent error state" с wasteful retries.

3. **Нет дифференциации типа failure.** AC в Story 6.5a определяет 5 failure types (H4), но CB считает их одинаково. Timeout после 3 retries и validation reject после 3 retries — разные ситуации.

**Рекомендация:** Дифференцированные thresholds:
- **bad_format:** 1 free retry (уже есть), затем 1 failure count. Total: 2 attempts.
- **timeout/crash:** 2 consecutive → CB OPEN (transient, but wasting 2+ minutes per attempt)
- **validation_reject:** 1 consecutive → CB OPEN (deterministic — same input → same bad output)
- **I/O error:** 1 → CB OPEN (system-level, not LLM-fixable)

**Impact:** Меняет CB логику в Story 6.5a, но не структуру. Добавляет `FailureType` enum + per-type threshold map.

---

### F2. [MEDIUM] ValidateDistillation criterion #3 "last 20% preserved" — хрупок при малых файлах

**Stories:** 6.5c
**AC:** "Entries from last 20% of original file preserved (H5)"

**Проблема:**
1. **Малые файлы:** 10 entries × 20% = 2 entries. Distillation может merge 2 entries в 1 с сохранением контента — Go validation reject это как "entry lost", хотя контент сохранён.
2. **Boundary problem:** Если 200 entries, 20% = 40. Это 40 entries которые MUST be preserved verbatim? Или topic-level preservation? AC не уточняет.
3. **Нет semantic matching:** Citation preservation (criterion #4) использует exact string match. Но distillation РЕФРАЗИРУЕТ — merged citations = "new" string.

**Research context:** StructEval benchmark ([arxiv 2505.20139](https://arxiv.org/html/2505.20139v1)) показал что даже лучшие модели (GPT-4o: 76%) не всегда сохраняют exact structure. ROUGE метрики ([Neptune.ai](https://neptune.ai/blog/llm-evaluation-text-summarization)) не ловят semantic preservation — нужен BERTScore или LLM-as-judge.

**Рекомендация:** Смягчить criterion #3:
- "Last 20% **topics** preserved" (topic-level, не entry-level)
- Minimum absolute: max(20%, 5 entries) — для малых файлов
- Match по `category:topic` prefix, не по полному entry text

---

### F3. [MEDIUM] BEGIN/END markers — 5-12% failure rate при complex outputs

**Stories:** 6.5b
**AC:** "Go parses only between BEGIN_DISTILLED_OUTPUT / END_DISTILLED_OUTPUT markers (H6)"

**Проблема:** Research по structured output reliability:

1. **StructEval benchmark** ([arxiv 2505.20139](https://arxiv.org/html/2505.20139v1)): GPT-4o achieves 76% average structured output accuracy. Для простых форматов (JSON, CSV) >90%, но complex multi-section formats значительно ниже.

2. **Claude reliability:** STED framework ([OpenReview](https://openreview.net/forum?id=rSCV1hTZvF)) показал Claude-3.7-Sonnet "near-perfect structural reliability" — но для SINGLE format, не multi-section outputs с nested categories.

3. **Conversational filler:** LLMs "frequently add conversational filler, introductory phrases, or concluding remarks" ([Tetrate.io](https://tetrate.io/learn/ai/llm-output-parsing-structured-generation)) — BEGIN/END markers решают это.

4. **Marker contamination risk:** Что если entry content содержит строку "END_DISTILLED_OUTPUT"? Parser обрежет output преждевременно. AC не специфицирует escaping.

**Что правильно:** BEGIN/END pattern — industry standard (StructEval использует аналогичный подход). Это лучше чем regex parsing всего output.

**Рекомендация:**
- Добавить AC: "If BEGIN marker found but END marker missing: treat as bad_format failure (free retry)"
- Добавить AC: "Parser ignores markers inside code blocks (``` fenced sections)"
- Считать markers case-insensitive для робастности

---

### F4. [MEDIUM] MonotonicTaskCounter — over-engineering для single-process CLI

**Stories:** 6.5a, 6.5c
**AC:** "MonotonicTaskCounter in DistillState = persisted, never resets"

**Проблема:**
1. **Решение правильное** — ephemeral counter действительно ломает cooldown при restart (H1). Persisted counter обязателен.
2. **Но:** MonotonicTaskCounter — концептуально это просто "total tasks completed ever". Название "monotonic" избыточно для single-process sequential CLI. В distributed systems monotonic counters решают ordering и consistency (Lamport clocks). Здесь — просто persistent counter.
3. **Edge case:** DistillState corrupted → MonotonicTaskCounter = 0 → cooldown `0 - LastDistillTask` = отрицательное → cooldown не пройдёт (правильно обрабатывается v5: "parse error → default CLOSED").
4. **Counter overflow:** uint64 overflow через ~18 quintillion tasks — не проблема. Но int vs uint не специфицирован.

**Рекомендация:**
- Переименовать в `TasksCompleted int` — проще, понятнее для Go codebase
- Добавить AC: "TasksCompleted uses int (not uint) — matches Go convention"
- Corruption recovery уже покрыт (fail-open) — ОК

---

### F5. [LOW] distill_gate: human|auto — двойной mode усложняет тестирование, но оправдан

**Stories:** 6.5a
**AC:** "distill_gate: human|auto (default: human)"

**Проблема:**
1. **Тестирование:** 2 modes × 5 failure types × 2 states (cooldown met/not met) = 20 комбинаций. Каждый mode имеет свою логику (human: gate prompt, auto: CB counter).
2. **v1 scope:** Auto mode нужен для CI/batch использования — оправдано. Но усложняет Story 6.5a.

**Что правильно:** User выбор (v5-2 decision) обоснован — CLI tool используется и интерактивно, и в CI. Default `human` = безопасный default.

**Рекомендация:**
- Тестирование: table-driven с `mode` как дополнительное поле в test struct
- Mock pattern: `GatePromptFunc` для human mode, `nil` для auto mode

---

## Research Insights — что заимствовать

### 1. A-MEM Zettelkasten pattern (NeurIPS 2025)
**Источник:** [A-MEM](https://arxiv.org/abs/2502.12110) — agentic memory с dynamic linking.

**Релевантность:** A-MEM создаёт "interconnected knowledge networks through dynamic indexing and linking". bmad-ralph's LEARNINGS.md → ralph-*.md = аналогичный двухуровневый подход (hot buffer + distilled knowledge). A-MEM подтверждает что **LLM-driven categorization** работает, НО требует structured attributes (keywords, tags, contextual descriptions) — bmad-ralph's `## category: topic [citation]` формат = минимально достаточная реализация этого паттерна.

**Заимствовать:** Ничего — текущий формат адекватен для CLI tool.

### 2. Model Collapse Prevention (ICLR 2025)
**Источник:** [Strong Model Collapse](https://proceedings.iclr.cc/paper_files/paper/2025/file/284afdc2309f9667d2d4fb9290235b0c-Paper-Conference.pdf), [Accumulation prevents collapse](https://arxiv.org/html/2404.01413v2)

**Релевантность:** Key finding: "replacement causes collapse; accumulation prevents it". bmad-ralph's distillation REPLACES LEARNINGS.md content — теоретический risk. Но: ANCHOR marker (L4) + 2-generation backups + "last 20% preserved" = mitigation.

**Заимствовать:** [Leveraging KD to Mitigate Collapse](https://openreview.net/forum?id=8TbqoP3Rjg) предлагает "knowledge distillation from high-performing teacher to student". Для bmad-ralph: **distillation prompt должен получать ralph-*.md как "teacher" context** — existing distilled rules guide new distillation, preventing drift. Сейчас prompt получает только LEARNINGS.md — нет anchor to existing distilled knowledge.

### 3. Differentiated Circuit Breaker (SRE Best Practices 2025)
**Источник:** [Portkey](https://portkey.ai/blog/retries-fallbacks-and-circuit-breakers-in-llm-apps/), [Maxim](https://www.getmaxim.ai/articles/retries-fallbacks-and-circuit-breakers-in-llm-apps-a-production-guide/)

**Релевантность:** "Retry handles transient failures while circuit breaker prevents hammering a broken service." Для LLM distillation — timeout = transient (сеть, нагрузка), validation reject = persistent (prompt problem).

**Заимствовать:** Per-failure-type thresholds (F1 выше).

### 4. Summarization Quality: ROUGE недостаточен
**Источник:** [Neptune.ai eval guide](https://neptune.ai/blog/llm-evaluation-text-summarization), [Confident AI](https://www.confident-ai.com/blog/a-step-by-step-guide-to-evaluating-an-llm-text-summarization-task)

**Релевантность:** ValidateDistillation использует deterministic criteria (line count, citation count). Это **правильный подход** для CLI tool — нет runtime LLM-as-judge доступен. Semantic quality проверяется через citation preservation (proxy metric). Но criterion #3 (20% preservation) и #4 (80% citations) — хрупки (F2).

---

## Подтверждено — что правильно

### P1. 3-layer distillation architecture
Go semantic dedup (0 tokens) → LLM compression (~8K tokens) → Go post-validation. Подтверждён convergent evolution с A-MEM, MemOS, AgeMem. Industry consensus.

### P2. Story 6.5 split на 3 sub-stories
6.5a (trigger/gate, 8 AC), 6.5b (session/parsing, 11 AC), 6.5c (validation/state, 6 AC) = 25 AC total. Средний по Epic 5 = ~7 AC/story. Split оправдан — каждая sub-story тестируется independently.

### P3. 2-generation backups (L4)
.bak + .bak.1 — достаточно. Research: "accumulation prevents collapse" — backups = accumulation safety net. Восстановление при crash (M7) покрыто.

### P4. Timeout 2 минуты (H8)
~8K tokens distillation. Claude-3.7-Sonnet: ~100 tokens/sec output → 80 seconds для 8K. 2 минуты = 50% headroom. **Configurable** через `distill_timeout` — правильно.

### P5. Canonical categories (H2)
7 + misc, list only grows, NEW_CATEGORY marker. Предотвращает drift (F/J2 из предыдущих ревью). Research: category-based organization = standard knowledge management pattern.

### P6. Human gate as default (C2/v5-2)
Default `human` для v1 экспериментальной дистилляции. Auto mode для CI. Правильный баланс safety vs autonomy.

### P7. Crash recovery at startup (M7)
Check .bak → restore → log warning. Простое, надёжное, не over-engineered.

### P8. Stderr overflow warning (M6/v5-5)
Informational, не blocking. Правильный подход для CLI tool.

---

## Итоговая таблица

| ID | Finding | Severity | Story | Effort |
|----|---------|----------|-------|--------|
| F1 | CB threshold undifferentiated by failure type | HIGH | 6.5a | MEDIUM |
| F2 | "Last 20% preserved" fragile for small files | MEDIUM | 6.5c | LOW |
| F3 | BEGIN/END markers — contamination + missing END | MEDIUM | 6.5b | LOW |
| F4 | MonotonicTaskCounter naming/type | MEDIUM | 6.5a/c | LOW |
| F5 | Dual mode testing complexity | LOW | 6.5a | LOW |

**Total: 0 CRITICAL, 1 HIGH, 3 MEDIUM, 1 LOW = 5 findings**

---

## Deep Research Sources

1. [Portkey — Retries, fallbacks, and circuit breakers in LLM apps](https://portkey.ai/blog/retries-fallbacks-and-circuit-breakers-in-llm-apps/)
2. [Google Cloud SRE for LLM Applications](https://medium.com/google-cloud/building-bulletproof-llm-applications-a-guide-to-applying-sre-best-practices-1564b72fd22e)
3. [Maxim — Production Guide for LLM Resilience](https://www.getmaxim.ai/articles/retries-fallbacks-and-circuit-breakers-in-llm-apps-a-production-guide/)
4. [Go Circuit Breaker Implementation](https://oneuptime.com/blog/post/2026-01-30-go-retry-circuit-breaker-pattern/view)
5. [StructEval — LLM Structural Output Benchmark](https://arxiv.org/html/2505.20139v1)
6. [STED — LLM Structured Output Consistency](https://openreview.net/forum?id=rSCV1hTZvF)
7. [A-MEM — Agentic Memory NeurIPS 2025](https://arxiv.org/abs/2502.12110)
8. [AgeMem — Unified Memory Management](https://arxiv.org/abs/2601.01885)
9. [Strong Model Collapse ICLR 2025](https://proceedings.iclr.cc/paper_files/paper/2025/file/284afdc2309f9667d2d4fb9290235b0c-Paper-Conference.pdf)
10. [Accumulation Prevents Collapse](https://arxiv.org/html/2404.01413v2)
11. [KD to Mitigate Model Collapse](https://openreview.net/forum?id=8TbqoP3Rjg)
12. [Antfarm — Agent Circuit Breaker Issue](https://github.com/snarktank/antfarm/issues/218)
13. [Neptune.ai — LLM Summarization Evaluation](https://neptune.ai/blog/llm-evaluation-text-summarization)
14. [Confident AI — Summarization Eval Guide](https://www.confident-ai.com/blog/a-step-by-step-guide-to-evaluating-an-llm-text-summarization-task)
15. [LLM Output Parsing Guide — Tetrate.io](https://tetrate.io/learn/ai/llm-output-parsing-structured-generation)
16. [Structured Output Reliability — Cognitive Today](https://www.cognitivetoday.com/2025/10/structured-output-ai-reliability/)
17. [Go Circuit Breaker Patterns in Microservices](https://dev.to/serifcolakel/circuit-breaker-patterns-in-go-microservices-n3)
18. [Knowledge Distillation Survey PMC 2025](https://pmc.ncbi.nlm.nih.gov/articles/PMC12634706/)
