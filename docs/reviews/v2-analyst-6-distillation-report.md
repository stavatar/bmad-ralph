# Analyst-6 Report: Нужна ли дистилляция знаний?

**Дата:** 2026-03-02
**Роль:** analyst-6 (knowledge-arch-v2, раунд 2)
**Фокус:** ROI 3-слойной дистилляции vs простое накопление
**Baseline:** analyst-6 R1, analyst-3 (конкуренты), analyst-8 (синтез/SimpleMem)
**Контекст:** Ralph — НОВЫЙ проект, любой стек. LEARNINGS.md растёт с нуля.

---

## Executive Summary

**Вердикт: Дистилляция нужна, но в упрощённой форме. Confidence: 85%.**

3-слойная система (Go semantic dedup → LLM compression → Go post-validation) — overengineered для текущего масштаба. Полный отказ тоже не оптимален. Рекомендуется **2-слойная архитектура**: Go-level dedup + human-triggered LLM distillation (без автоматического post-validation).

**Конвергенция с раундом 1:** Все три отчёта R1 (analyst-6, analyst-3, analyst-8) сходятся:
- analyst-6 R1: ROI pipeline отрицательный, ручная чистка достаточна до ~500 строк
- analyst-3: Ни один mainstream tool не делает auto-distillation правил
- analyst-8: "Deepen, don't pivot" — file-based оптимален, SimpleMem подтверждает 3-stage compression ТОЛЬКО для operational knowledge

**Новый контекст (рост с нуля, стек-агностичность):** Ralph будет накапливать знания для РАЗНЫХ проектов на разных стеках. Это означает:
1. Категоризация важнее сжатия — Go-правила не нужны Python-проекту
2. Glob-scoping по стеку — ключевой механизм фильтрации
3. Дистилляция менее актуальна — при glob-scoping загружается subset, не весь файл

---

## 1. При каком объёме raw записи начинают мешать?

### Данные исследований

- **"Lost in the Middle" (Liu et al., 2023/2024, TACL):** LLM performance drops 30%+ when relevant info sits in the middle of long context. U-shaped attention curve — models attend strongly to beginning and end, poorly to middle. [arxiv.org/abs/2307.03172](https://arxiv.org/abs/2307.03172)
- **Chroma Research (2025):** All 18 tested frontier models exhibit context rot. Degradation grows non-linearly with context size. [morphllm.com/context-rot](https://www.morphllm.com/context-rot)
- **Claude Code context window:** 200K tokens (Opus 4.6 supports 1M in beta). CLAUDE.md + .claude/rules/ loaded into working memory, consuming tokens per message. [code.claude.com/docs/en/memory](https://code.claude.com/docs/en/memory)

### Практические пороги для Ralph

| Размер LEARNINGS.md | Токены (~) | % от 200K окна | Оценка влияния |
|---|---|---|---|
| 100 строк | ~4K | 2% | Незначительно |
| 200 строк | ~8K | 4% | Минимально |
| 300 строк | ~12K | 6% | Ощутимо (cumulative с rules) |
| 500 строк | ~20K | 10% | Заметная деградация |
| 1000+ строк | ~40K+ | 20%+ | Критическое |

**Ключевой факт:** В Ralph уже загружаются `.claude/rules/` (~122 правила, 7 файлов). При glob-scoping не все грузятся одновременно, но при работе с Go тестами грузится 5-6 файлов (~300 строк rules). Добавление 200+ строк LEARNINGS.md увеличивает static context на ~50%.

### Вывод по Q1

**Порог проблемы: ~200-300 строк LEARNINGS.md** (совокупно с rules). До этого порога дистилляция — оптимизация. После — необходимость. Текущий лимит 200 строк в Epic 6 — **корректный**, но запас невелик при активной разработке (5-10 entries/story × 10+ stories = 50-100 entries).

---

## 2. LLM compression инструкций — сохраняется ли качество?

### Исследования

- **Extractive vs Abstractive (2024, EMNLP):** Extractive compression (выбор существующих фраз) значительно превосходит abstractive (перефразирование). Extractive: +7.89 F1 при 4.5x сжатии. Abstractive: -4.69 F1 при аналогичном сжатии. [arxiv.org/abs/2407.08892](https://arxiv.org/abs/2407.08892)
- **Style-Compress (2024):** LLM-based compression preserves task performance when style matches task type. CoT reasoning favors abstractive, QA favors extractive. [aclanthology.org/2024.findings-emnlp.851](https://aclanthology.org/2024.findings-emnlp.851/)
- **LLMLingua (Microsoft):** Token pruning achieves up to 20x compression with minimal accuracy degradation for retrieval tasks, but tested on factual QA, NOT imperative instructions. [microsoft.com/en-us/research/blog/llmlingua](https://www.microsoft.com/en-us/research/blog/llmlingua-innovating-llm-efficiency-with-prompt-compression/)

### Риски для императивных правил

Coding rules имеют специфику, которую general compression исследования не покрывают:

1. **Точность формулировок критична:** "Always use `errors.As`, not type assertions" — перефразирование может потерять "not type assertions"
2. **Citations потеря:** File:line references при abstractive compression будут потеряны
3. **Контекстуальные нюансы:** "Platform-agnostic inner error assertions: file-as-directory produces 'is a directory' on Linux but 'Incorrect function.' on Windows/WSL" — сжатие потеряет Windows-specific detail
4. **VIOLATION examples:** Concrete examples (2.5x effectiveness по DGM research) — первые кандидаты на удаление при сжатии

### Model Collapse применимость

- **Model collapse (Nature, 2024):** При рекурсивном обучении на собственных выходах модели теряют распределение хвостов. [nature.com/articles/s41586-024-07566-y](https://www.nature.com/articles/s41586-024-07566-y)
- **Применимость к дистилляции правил: СРЕДНЯЯ.** Каждый цикл LLM compression → re-injection → next LLM compression — это рекурсивная обработка. Но это не training collapse, а inference-level. Риск: progressive smoothing нюансов за N итераций, потеря edge-case правил.
- **Mitigation в Epic 6:** Post-validation quality gate (7+ criteria) — **адекватная защита**, но добавляет сложность.

### Вывод по Q2

**LLM compression императивных правил рискованна.** Extractive подход (dedup + merge) безопаснее abstractive (переформулирование). Текущий дизайн Epic 6 с post-validation частично компенсирует риск, но добавляет ~1 story сложности ради страховки от проблемы, которую можно решить проще.

---

## 3. ROI: 3-слойная дистилляция vs простое накопление

### Сложность 3-слойной системы (Epic 6 текущий дизайн)

| Компонент | Stories | AC | Сложность |
|---|---|---|---|
| Layer 1: Go semantic dedup | 6.1 (часть) | ~5 AC | Средняя |
| Layer 2: LLM distillation | 6.3 | ~12 AC | Высокая |
| Layer 3: Post-validation | 6.3 (часть) + 6.4 | ~8 AC | Высокая |
| Distillation CLI | 6.3 | ~3 AC | Низкая |
| Circuit breaker / gate | 6.3 | ~4 AC | Средняя |
| **Итого** | **~3 stories** | **~32 AC** | **Высокая** |

### Альтернатива: Простое накопление + ручная чистка

| Компонент | Stories | AC | Сложность |
|---|---|---|---|
| Go-level dedup (при записи) | 6.1 (часть) | ~5 AC | Средняя |
| Budget warning (stderr) | 0.5 | ~2 AC | Низкая |
| Human prompt при NearLimit | 0.5 | ~2 AC | Низкая |
| **Итого** | **~1 story** | **~9 AC** | **Низкая** |

### Когда автоматическая дистилляция окупается?

- **При >500 entries/файл** — ручная чистка становится утомительной
- **При >10 проектов на Ralph** — масштаб оправдывает автоматизацию
- **При 200-строковом лимите** — дистилляция нужна после ~40-50 stories (при 4-5 entries/story)

### Вывод по Q3

**ROI 3-слойной дистилляции отрицательный для MVP.** Сложность ~32 AC ради проблемы, которая наступит через 40+ stories. Ralph сейчас на Epic 6 из ~8-10 эпиков — к концу проекта будет ~80-100 entries, что укладывается в 200-строчный лимит с Go-level dedup.

---

## 4. Cline Auto-Compact — как работает, теряет ли детали?

### Механизм

- При приближении к лимиту контекста Cline автоматически суммаризует историю разговора
- Создаёт comprehensive summary, сохраняет технические решения и изменения кода
- Заменяет историю сообщений суммари, продолжает с того же места
- Задача на 5M токенов может завершиться в окне 200K через auto-compact
- [docs.cline.bot/features/auto-compact](https://docs.cline.bot/features/auto-compact)

### Потери

- **Прогрессивная эрозия деталей** при множественных compaction-ах в длинной сессии
- **Context rot** (Chroma research) подтверждает: LLM не обрабатывают контекст равномерно
- **Recovery:** checkpoints позволяют откатиться к состоянию до суммаризации

### Применимость к Ralph

Cline Auto-Compact — это **session-level** compression (сжатие диалога). Ralph distillation — **cross-session** knowledge compression. Разные проблемы:
- Auto-Compact теряет session context (приемлемо — сессия закончится)
- Distillation теряет knowledge content (неприемлемо — знания должны жить вечно)

### Вывод по Q4

Cline Auto-Compact подтверждает что LLM summarization работает для **ephemeral context** (диалог), но **не** является прецедентом для compression persistent knowledge (правила/паттерны).

---

## 5. Как другие AI coding tools управляют правилами?

### Обзор

| Tool | Файл правил | Размер | Auto-distillation | Управление |
|---|---|---|---|---|
| Claude Code | CLAUDE.md + .claude/rules/ | <200 строк рекомендуется | Нет | Ручное, glob-scoped |
| Cursor | .cursorrules | Нет лимита (рекомендуют коротко) | Нет | Ручное |
| Windsurf | .windsurfrules + global_rules.md | Нет лимита | Нет | Ручное |
| Aider | --read CONVENTIONS.md | Нет лимита | Нет | Ручное |
| GitHub Copilot | .github/copilot-instructions.md | Нет документированного лимита | Self-healing (JIT validation) | Автоматическое |

### Ключевой вывод

**Ни один из mainstream AI coding tools не реализует автоматическую дистилляцию правил.** Все полагаются на ручное управление файлами правил. Единственное исключение — GitHub Copilot Memory с JIT citation validation (удаление записей со stale references), но это **валидация, не компрессия**.

SFEIR research: ~15 хорошо сформулированных правил = 94% compliance. Больше правил ≠ лучше.

### Вывод по Q5

Индустрия сходится на **ручном управлении** правилами с рекомендацией "keep it short". Автоматическая дистилляция — нерешённая задача, которую никто из лидеров рынка не пытается решить автоматически.

---

## 6. Исследования по автоматическому сжатию наборов правил

### Что существует

- **Prompt compression (LLMLingua, ICAE, 500xCompressor):** Работает для factual context (RAG, QA). НЕ тестировалось на imperative coding rules. [arxiv.org/html/2503.19114](https://arxiv.org/html/2503.19114)
- **Information Preservation in Prompt Compression (2025):** Holistic evaluation framework — downstream performance alone insufficient. Need to measure grounding to original text + type/amount of preserved information.
- **DGM (Concrete violations >> abstract rules):** 2.5x effectiveness. Сжатие удаляет concrete examples первыми — прямое противоречие.
- **Live-SWE-agent step-reflection:** +12% quality from single reflection step. Не compression, а structured reasoning.

### Чего НЕ существует

- Исследований по compression **coding convention sets** (правил разработки)
- Benchmark-ов extractive vs abstractive для **imperative instructions**
- Доказательств что автоматическое сжатие правил сохраняет enforcement quality

### Вывод по Q6

**Исследовательская база для автоматического сжатия правил отсутствует.** Существующие prompt compression техники оптимизированы для factual content retrieval, не для imperative instruction enforcement. Ralph будет пионером без empirical backing.

---

## 7. Порог: ручная чистка vs автоматическая дистилляция

### Прагматический анализ

| Метрика | Ручная чистка достаточна | Автоматизация нужна |
|---|---|---|
| Entries в файле | < 200 | > 500 |
| Stories до порога | ~40-50 | ~100+ |
| Частота чистки | При ретроспективах (каждый эпик) | Каждые 5-10 tasks |
| Время на чистку | 10-15 мин/ретро | N/A (автоматически) |
| Риск потери данных | Минимальный (human oversight) | Средний (LLM smoothing) |

### Текущая ситуация Ralph

- 5 эпиков завершено → 122 правила в .claude/rules/ (ручная экстракция при ретро)
- Ручная экстракция работает: правила структурированы, file:line citations, тематические файлы
- Ретроспектива каждый эпик = естественный момент для чистки
- **Нет свидетельств что ручной процесс не справляется**

### Вывод по Q7

**Ручная чистка при ретроспективах достаточна для масштаба Ralph.** Автоматическая дистилляция оправдана только если Ralph будет управлять проектами с 500+ entries или если ретроспективы прекратятся.

---

## Сводная рекомендация

### Что ОСТАВИТЬ из текущего дизайна Epic 6

1. **Go-level semantic dedup (Layer 1)** — простой, надёжный, 0 risk
2. **Budget monitoring** — stderr warning при NearLimit
3. **Human gate** при budget overflow — "LEARNINGS.md at 180/200 lines, distill now?"
4. **`ralph distill` CLI** — manual trigger для human-initiated distillation
5. **JIT citation validation** — удаление stale entries (как Copilot Memory)

### Что УПРОСТИТЬ или ОТЛОЖИТЬ

1. **Auto-trigger distillation (Layer 2)** → ОТЛОЖИТЬ. Заменить human gate: "Budget near limit, run `ralph distill`?"
2. **Go post-validation of LLM output (Layer 3)** → УПРОСТИТЬ до line-count check (before >= after) + format check
3. **Circuit breaker / cooldown** → УБРАТЬ. Не нужен если distillation human-triggered
4. **7-criteria quality gate** → СОКРАТИТЬ до 3: line count preserved, format valid, no empty entries
5. **Trend tracking (Story 6.9)** → ОТЛОЖИТЬ в Growth. Недостаточно данных для trends при <100 entries

### Экономия

- **Текущий:** ~3 stories, ~32 AC (distillation + post-validation + circuit breaker)
- **Упрощённый:** ~1.5 stories, ~12 AC (dedup + budget gate + simple CLI distill)
- **Экономия:** ~1.5 stories, ~20 AC, значительно меньше edge cases и failure modes

### Confidence: 85%

Повышена с 82% (R1) благодаря конвергенции 3 независимых отчётов R1. Ограничения: отсутствие research по compression imperative coding rules, неизвестна реальная скорость накопления знаний для стек-агностичного use case.

---

## 8. Cross-reference с отчётами раунда 1

### 8.1 Конвергенция выводов

| Вопрос | analyst-6 R1 | analyst-3 R1 | analyst-8 R1 | analyst-6 R2 (этот) |
|---|---|---|---|---|
| Порог проблемы | ~500-800 строк | — | ~500 записей | ~200-300 строк (с rules) |
| Auto-distillation нужна? | Нет (ROI отрицательный) | Нет (индустрия не делает) | Только operational knowledge | Нет для MVP |
| LLM compression безопасна? | Нет (semantic drift) | Devin: "summaries not comprehensive" | "LLM distillation unreliable" | Нет (extractive лучше) |
| Ручная чистка достаточна? | Да (до ~500 строк) | Да (индустриальный паттерн) | Да (hybrid: retro merge) | Да (до 40+ stories) |
| Рекомендация | Append + manual dedup | Glob-scoped правила | Deepen, don't pivot | 2-layer: dedup + human gate |

**Конвергенция: 5/5 вопросов — все 4 анализа согласны.** Это даёт высокую confidence что 3-слойная auto-distillation — overengineering.

### 8.2 Дополнительные данные из R1

**Из analyst-3 (конкуренты):**
- Cline Auto-Compact: session-level, НЕ knowledge-level. Devin: "summaries weren't comprehensive enough"
- Cursor token tax: 20 global rules × 100 tokens = 2000 tokens — glob-scoping решает, не compression
- AGENTS.md стандарт (Linux Foundation): НЕ включает distillation — only format

**Из analyst-8 (синтез):**
- SimpleMem Stage 1 (compression) ≈ текущий pipeline bmad-ralph (review → extract → atomize). **Уже реализовано** вручную при ретро
- MemOS: scheduling (что загрузить) важнее объёма хранилища. Glob-scoping = простой scheduling
- Letta benchmark: file-based 74% vs graph-based 68.5% — file-based превосходит при <500 записей
- Context budget: 7.5-10% knowledge overhead = sweet spot. При 200 строк LEARNINGS = ~4% — в пределах

### 8.3 Пересмотр для нового проекта (рост с нуля, стек-агностичность)

**Ключевой сдвиг:** Ralph управляет знаниями для РАЗНЫХ проектов. Это меняет расчёт:

1. **Категоризация > сжатие.** Go-правила бесполезны для Python-проекта. Glob-scoping по `.go`, `.py`, `.rs` файлам — естественная фильтрация. При glob-scoping эффективный размер загружаемых знаний = subset для текущего стека, не весь LEARNINGS.md.

2. **Скорость роста ниже.** Новый проект начинает с 0 entries. При 4-5 entries/story, первые 40 stories дадут 160-200 entries — это ~6-12 месяцев работы до порога.

3. **Стек-специфичные знания устаревают быстрее.** Go 1.25 паттерны могут не работать в Go 1.27. Staleness detection (JIT citation validation) важнее compression.

4. **Multi-project scenario.** Если Ralph используется на 3+ проектах одновременно, каждый проект имеет свой LEARNINGS.md. Дистилляция на уровне одного проекта вообще не нужна до ~200 entries в каждом.

**Вывод:** Для нового проекта, растущего с нуля, дистилляция нужна ещё ПОЗЖЕ чем для bmad-ralph (который уже имеет 122 правила). Go-level dedup + budget monitoring + human gate — достаточно на 12+ месяцев.

---

## 9. Evidence from Project Research (R1/R2/R3)

Три глубоких исследования проекта (82 источника суммарно) содержат критические данные, напрямую относящиеся к вопросу дистилляции.

### 9.1 Из R1 (Knowledge Extraction, 20 источников)

**Ключевые findings для дистилляции:**

1. **~150-200 инструкций — практический предел** consistent following для frontier LLM [R1-S14]. При 3-5 строках на правило, 200-строчный LEARNINGS.md ≈ 40-60 distinct rules. Текущий CLAUDE.md bmad-ralph уже содержит ~50 правил — приближаемся к лимиту БЕЗ учёта LEARNINGS.md.

2. **Shuffled/atomized контекст работает ЛУЧШЕ organized narrative** [R1-S5, Chroma]. Counterintuitive: организованный текст создаёт attention shortcuts. Atomized independent facts эффективнее structured narrative. **Вывод для дистилляции:** LLM compression, которая перефразирует и объединяет правила в связный текст, может УХУДШИТЬ compliance.

3. **Convergent evolution:** Три независимых проекта (claude-mem, Claudeception, continuous-learning) пришли к одной архитектуре: **capture → compress → inject on demand** [R1-S9, S10, S11]. Но "compress" здесь = extractive (выбор релевантного), НЕ abstractive (переписывание). Это принципиально отличается от LLM distillation.

4. **Citation-based JIT validation — единственный количественно доказанный паттерн:** Copilot Memory: +7% PR merge rate, self-healing [R1-S3, S18]. Это ВАЛИДАЦИЯ (удаление stale), не КОМПРЕССИЯ (переписывание).

5. **Dangerous feedback loop:** Больше ошибок → больше learnings → context rot → больше ошибок. Budget = circuit breaker. Но low-quality distillation может создать second-order loop: **bad compressed rules survive → agent learns wrong patterns** [R1-§5.5].

### 9.2 Из R2 (Knowledge Enforcement, 40 источников)

**Ключевые findings для дистилляции:**

1. **15 императивных правил = 94% compliance; bloated file = значительное падение** [R2-S25, SFEIR]. Это КРИТИЧЕСКИЙ порог. 125+ паттернов в одном файле = ~40-50% compliance. **Вывод:** проблема решается **sharding** (разбиение на файлы по 15 правил), НЕ compression (сжатие в один файл).

2. **Тройной барьер compliance** [R2-§4.1]:
   - Compaction уничтожает правила [R2-S21-S24]
   - Context rot: 30-50% degradation [R2-S5]
   - Volume ceiling: >15 правил = compliance падает [R2-S25]

   **Дистилляция НЕ решает барьеры 1 и 2.** Compressed файл всё равно подвержен compaction и context rot. Только hooks обходят compaction, а sharding снижает volume.

3. **DGM: конкретные violations 2.5x эффективнее абстрактных правил** [R2-S37]. SWE-bench: 20% → 50% при хранении "failed attempts" вместо правил. **Вывод для дистилляции:** LLM compression удаляет конкретные примеры первыми (они "избыточны") — прямое противоречие с DGM research.

4. **Skills activation: ~20% baseline → ~84% с forced evaluation hooks** [R2-S27]. **Вывод:** enforcement mechanism (hooks) важнее content optimization (distillation). Те же правила с hooks работают в 4x лучше.

5. **MCP tools: 56% skip rate** без forced invocation [R2-S31]. Если distillation result хранится как MCP-accessible knowledge — agent может не retriev-ить его в половине случаев.

### 9.3 Из R3 (Alternative Methods, 22 источника)

**Ключевые findings для дистилляции:**

1. **File-based injection: 74.0% LoCoMo vs Mem0 Graph: 68.5%** [R3-S1, Letta benchmark]. Простой filesystem превосходит сложные memory tools при текущем масштабе. **Дистилляция добавляет complexity без доказанного benefit.**

2. **RAG break-even: >500 записей** [R3-§4.1.3]. При <500 записях pre-loading эффективнее. При 200-строчном LEARNINGS.md = 40-60 entries — ДАЛЕКО от break-even. Selective retrieval (что делает distillation) не нужен.

3. **Pre-loading 200 строк = ~3000-4000 tokens — within sweet spot** [R3-§4.1.3, R1]. Token economics: fixed cost, predictable, zero latency. Distillation экономит токены, но при 3-4K — экономия маргинальна.

4. **Atomized fact format IS the optimal chunking strategy** [R3-§4.6.2]. Каждый `## ` header = one retrievable unit. **Вывод:** текущий format не нуждается в compression — он уже оптимально структурирован.

5. **Growth path validates deferred distillation** [R3-§6]:
   - <300 entries: file-based optimal, ship as-is
   - 300-500: file + MCP evaluation
   - 500-2000: hybrid (file + selective retrieval)
   - >2000: full RAG

   **Автоматическая LLM distillation не появляется ни на одном этапе R3 roadmap.**

### 9.4 Синтез: что R1+R2+R3 говорят о дистилляции

| Evidence | Поддерживает distillation | Против distillation |
|---|---|---|
| 150-200 instruction limit [R1-S14] | Нужно оставаться в бюджете | Sharding решает без compression |
| 15 rules = 94% compliance [R2-S25] | — | Sharding по 15 правил/файл = решение |
| Shuffled > organized [R1-S5] | — | LLM compression делает текст БОЛЕЕ связным = хуже |
| DGM 2.5x concrete [R2-S37] | — | Compression удаляет примеры = хуже |
| File-based 74% > graph 68.5% [R3-S1] | — | Простота > сложность |
| Citation validation +7% [R1-S3] | JIT validation = extractive | Не compression |
| Convergent capture→compress→inject [R1] | Pattern exists | "Compress" = select, not rewrite |
| Feedback loop risk [R1-§5.5] | Distillation can break loop | Bad distillation amplifies loop |
| Hook enforcement 4x [R2-S27] | — | Hooks important, not content |
| Atomized format optimal [R3-§4.6.2] | — | Already compressed enough |

**Счёт: 1-2 "за" vs 8-9 "против".** Evidence overwhelmingly supports that:
- **Sharding** (разбиение на маленькие glob-scoped файлы) решает volume problem
- **Hook enforcement** решает compliance problem
- **JIT citation validation** решает staleness problem
- **LLM distillation** не решает ни одну из трёх корневых проблем, добавляя risk и complexity

### 9.5 Обновлённая confidence

**Confidence: 88%** (повышена с 85% благодаря 82 источникам из 3 исследований + 3 отчётов R1-аналитиков).

Единственный сценарий, где LLM distillation оправдана: LEARNINGS.md вырос до >300 строк И sharding невозможен (все записи в одной категории). При текущей архитектуре с multi-file categorization — этот сценарий маловероятен.

---

## Источники

1. [Lost in the Middle (Liu et al., 2023/2024)](https://arxiv.org/abs/2307.03172) — TACL, attention degradation
2. [Context Rot (Morph/Chroma, 2025)](https://www.morphllm.com/context-rot) — 18 models tested
3. [Cline Auto-Compact docs](https://docs.cline.bot/features/auto-compact) — session compression
4. [Cline Context Engineering blog](https://cline.ghost.io/how-to-think-about-context-engineering-in-cline/)
5. [Model Collapse (Nature, 2024)](https://www.nature.com/articles/s41586-024-07566-y) — recursive training degradation
6. [Prompt Compression Methods (ICML, 2024)](https://arxiv.org/abs/2407.08892) — extractive vs abstractive
7. [Style-Compress (EMNLP, 2024)](https://aclanthology.org/2024.findings-emnlp.851/) — task-specific compression
8. [LLMLingua (Microsoft)](https://www.microsoft.com/en-us/research/blog/llmlingua-innovating-llm-efficiency-with-prompt-compression/) — token pruning
9. [Information Preservation in Prompt Compression (2025)](https://arxiv.org/html/2503.19114) — holistic evaluation
10. [Claude Code Memory docs](https://code.claude.com/docs/en/memory) — 6-level hierarchy
11. [Writing a good CLAUDE.md (HumanLayer)](https://www.humanlayer.dev/blog/writing-a-good-claude-md) — <200 lines best practice
12. [Aider Conventions](https://aider.chat/docs/usage/conventions.html) — --read convention files
13. [AI Coding Rules directory](https://aicodingrules.com/) — 7000+ rules across tools
14. [Ruler (cross-tool rules)](https://github.com/intellectronica/ruler) — universal rule sync
15. [Context Buffer Management (Claude Code)](https://claudefa.st/blog/guide/mechanics/context-buffer-management) — 33K-45K reserved

### Project Research (R1/R2/R3)

16. R1: `docs/research/knowledge-extraction-in-claude-code-agents.md` — 20 sources, 6-level memory hierarchy, context rot, extraction patterns
17. R2: `docs/research/knowledge-enforcement-in-claude-code-agents.md` — 40 sources, triple barrier, hook enforcement, 15 rules = 94% compliance
18. R3: `docs/research/alternative-knowledge-methods-for-cli-agents.md` — 22 sources, file-based 74% > graph 68.5%, RAG break-even >500

### Round 1 Analyst Reports

19. `docs/reviews/analyst-6-distillation-report.md` — ROI analysis, 500-800 line threshold
20. `docs/reviews/analyst-3-competitors-report.md` — 12 tools compared, no auto-distillation in industry
21. `docs/reviews/analyst-8-synthesis-report.md` — SimpleMem, MemGPT, 5 convergent patterns, "deepen don't pivot"
