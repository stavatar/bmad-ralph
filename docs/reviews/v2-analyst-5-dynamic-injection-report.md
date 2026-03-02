# Analyst-5 (v2): Динамическая инъекция знаний по этапам и итерациям

**Контекст:** Новый проект (Ralph v2) — CLI-оркестратор, любой стек, pipe mode (`claude --print`), рост от 0 правил.
**Baseline:** Отчёты раунда 1 (analyst-5, analyst-7, analyst-4) + проектные исследования R1/R2/R3.
**Источники:** 82+ уникальных источника (20 R1 + 20 R2 + 22 R3 + 13 новых web search + 7 перекрёстных).

---

## Резюме

**Этапная фильтрация (execute/review/retry) — ДА, проектировать с первого дня, активировать при ~80 правилах.**

Три ключевых вывода:

1. **Stage-фильтрация**: архитектурно заложить сразу (теги в frontmatter), но фактически включать фильтрацию только при >80 правилах. До этого — грузить всё (compliance потери <10%).
2. **Iteration-фильтрация**: НЕТ. Номер retry не релевантен для свежего контекста. Вместо этого — content-based routing по тексту ошибки (отложить до Growth).
3. **Критический порог**: ~80 правил / ~5K токенов — момент, когда фильтрация начинает окупаться. При 200+ — обязательна.

**Ключевое расхождение с первым раундом:** Analyst-5 v1 оценивал ~122 правила bmad-ralph в ~4K токенов и заключил "НЕТ фильтрации нужно". Новый IFScale benchmark показывает, что для Claude (linear decay) даже при 50 инструкциях adherence уже падает на ~5-10%. Для нового проекта с нуля — заложить stage-теги с первого дня как zero-cost investment.

---

## Вопрос 1: Разные этапы — разные знания?

### Консолидированная доказательная база

**Новое исследование (v2):**

**IFScale benchmark** ([Jaroslawicz et al., 2025](https://arxiv.org/html/2507.11538v1)):
- 20 моделей, 10-500 инструкций. **Claude Sonnet 4 — linear decay**: каждая инструкция стабильно снижает точность
- При 500 инструкциях лучшие модели = **68%** accuracy
- Модели чаще **пропускают** (omission) чем искажают (modification)
- Три паттерна: threshold decay (o3, gemini), linear decay (claude, gpt-4.1), exponential decay (llama)

**Prompt bloat** ([MLOps Community, 2025](https://mlops.community/the-impact-of-prompt-bloat-on-llm-output-quality/)):
- Reasoning degradation с **~3000 токенов** (Goldberg et al.)
- LLM **не умеют игнорировать** нерелевантные инструкции
- Семантически похожие нерелевантные инструкции **особенно вредны**

**Из первого раунда (baseline):**

**Compliance data** (analyst-7, push vs pull модели):

| Модель доставки | Compliance | Пример |
|----------------|-----------|--------|
| Push (eager, SessionStart) | **~90-94%** | 15 critical rules через hook |
| Push (conditional, glob) | **~60-90%** | `.claude/rules/test-*.md` по паттерну файла |
| Pull (explicit) | **~30-50%** | Агент сам запрашивает Read |
| Pull (implicit) | **~20-40%** | Агент ищет Grep |

**Anthropic context engineering** ([Anthropic, 2025](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)):
- "Every token competes for the model's attention"
- "Find the smallest possible set of high-signal tokens"
- Sub-agent архитектуры с конденсированными саммари (1000-2000 токенов)

### Вывод: ДА, но с нюансами для pipe mode

В pipe mode (`claude --print`) нет glob-scoped загрузки Claude Code — Ralph **сам** собирает промпт. Это значит:
- **Ralph контролирует что загружается** — полная свобода фильтрации
- Нет "framing" от Claude Code ("may or may not be relevant")
- Каждый вызов = чистый контекст, нет compaction

Логика разделения по этапам для ЛЮБОГО проекта:

| Этап | Релевантные знания | Шум при загрузке всего |
|------|-------------------|----------------------|
| **execute** | Coding conventions, error patterns, architecture, naming | Review checklists, finding templates, severity ratings |
| **review** | Quality checklists, DRY/KISS/SRP, doc accuracy, assertion patterns | Build setup, scaffold patterns, platform workarounds |
| **retry** | Common mistakes, error wrapping, platform gotchas, debugging tips | Review templates, scaffold patterns |

**Механизм**: frontmatter-теги `stages: [execute, review, retry, all]` + фильтрация в prompt assembler Ralph.

### Практическая рекомендация

**День 1**: добавить поле `stages` в формат знаний. Cost = 0 (просто metadata).
**Порог активации**: включить фильтрацию когда total rules × (1 - stage_overlap%) > 50. На практике это ~80 правил при ~40% overlap между этапами.

---

## Вопрос 2: Разные итерации — разные знания?

### Доказательная база

**SCOPE framework** ([arxiv, 2025](https://arxiv.org/html/2512.15374v1)):
- Промпт эволюционирует на основе **step-level feedback**, не номера итерации
- Адаптация по **содержанию ошибки**, а не по порядковому номеру

**Multi-turn degradation** ([Multi-IF benchmark](https://arxiv.org/html/2410.15553v2)):
- Точность: o1-preview 0.877 → 0.707 (turn 1 → turn 3)
- **Не применимо к Ralph**: каждый `claude --print` = новый контекст

**Retry logic best practices** ([SparkCo, 2025](https://sparkco.ai/blog/mastering-retry-logic-agents-a-deep-dive-into-2025-best-practices)):
- Escalate to **alternative strategies**, не повторять
- Fail-over fast: переключение подхода > повторная попытка

**Из первого раунда (analyst-7):**
- Pull-based (агент решает что нужно) = **30-50% compliance**
- Iteration-based routing потребует либо pull (агент оценивает ситуацию = low compliance), либо сложный text analysis ошибки (high implementation cost)

### Вывод: НЕТ для номера итерации

Три аргумента против iteration-based:

1. **Stateless context**: `claude --print` = новый контекст. Retry #5 идентичен retry #1 с точки зрения модели. Только фидбек (текст ошибки) отличается.

2. **Нет evidence**: Ни одно исследование не показало улучшение от "дай больше правил на поздних итерациях". SCOPE адаптирует по содержанию, не по номеру.

3. **Высокая сложность, низкий ROI**: анализ текста ошибки для routing = NLP-задача, которую сам LLM мог бы решить проще.

**Единственное обоснованное действие на retry**: если конкретная ошибка повторяется — Ralph может добавить в промпт **конкретный fix-hint** (не набор правил, а текст "Предыдущая попытка не сработала из-за X. Попробуй Y"). Это уже реализовано в Ralph через feedback injection.

---

## Вопрос 3: При каком объёме фильтрация начинает помогать?

### Консолидированные пороги (три источника)

**IFScale (новое, v2):**

| Инструкций | Лучшая модель | Средняя | Claude (linear decay) |
|-----------|---------------|---------|----------------------|
| 10 | ~99% | ~95% | ~97% |
| 50 | ~95% | ~85% | ~90% |
| 100 | ~90% | ~75% | ~82% |
| 150 | ~85% | ~65% | ~75% |
| 250 | ~75% | ~50% | ~62% |
| 500 | ~68% | ~35% | ~45% |

**Analyst-4 (v1, категоризация):**

| Правил | Токенов | Рекомендация |
|--------|---------|-------------|
| <30 | <1500 | Один файл достаточен |
| 30-80 | 1500-4000 | Категоризация опциональна |
| 80-150 | 4000-8000 | Категоризация рекомендована |
| >150 | >8000 | Обязательна фильтрация |

**Analyst-5 (v1, динамическая инъекция):**

| Токенов | Рекомендация |
|---------|-------------|
| <5K | Фильтрация НЕ нужна |
| 5-15K | Опциональна (glob достаточен) |
| 15-30K | Рекомендуется |
| >30K | Обязательна |

### Синтез: единая шкала для нового проекта (рост от 0)

| Фаза проекта | Правил | Токенов (~) | Stage-фильтрация | Topic-фильтрация | Compliance без фильтрации |
|-------------|--------|-------------|-------------------|-------------------|--------------------------|
| **Start** (0-30) | 0-30 | 0-2K | Не нужна | Не нужна | ~95%+ |
| **Early** (1-2 мес) | 30-80 | 2-5K | Не нужна (но теги уже есть) | Опциональна | ~85-90% |
| **Growth** (3-6 мес) | 80-200 | 5-15K | **Включить** | Рекомендуется | ~70-82% |
| **Mature** (6+ мес) | 200-500 | 15-40K | Обязательна | Обязательна | ~45-62% |
| **Scale** (1+ год) | 500+ | 40K+ | Обязательна + приоритизация | Обязательна + RAG | <45% |

**Ключевой порог: ~80 правил / ~5K токенов — момент включения stage-фильтрации.**

Почему 80, а не 50 или 150:
- IFScale: Claude при 80 инструкциях ≈ 85% adherence → потеря 15% = ощутимо
- При stage-фильтрации (загружаем ~60% правил) → adherence ≈ 90% → +5% gain
- До 80: gain <5% → не окупает complexity
- После 150: без фильтрации <75% → критично

### Pipe mode vs Claude Code CLI — важное различие

В Claude Code CLI правила загружаются через встроенный mechanism (CLAUDE.md + .claude/rules/ с glob). В pipe mode (`claude --print`) Ralph контролирует промпт полностью. Это значит:

- **Нет overhead от Claude Code framing** ("may or may not be relevant")
- **Нет compaction** — каждый вызов = чистый промпт
- **Точный token budget** — Ralph знает сколько токенов тратит на правила
- **Можно оптимизировать placement** — правила внизу промпта (ближе к задаче)

---

## Вопрос 4: Context rot и бюджет токенов

### Lost in the Middle — инструкции vs данные

**Консолидированный вывод из обоих раундов:**

| Аспект | Retrieval data | Instructions |
|--------|---------------|-------------|
| Lost in the Middle severity | Высокая (30%+ drop) | Умеренная (instruction-tuning компенсирует) |
| Порог проявления | ~20K+ токенов | ~3K+ токенов (reasoning degradation) |
| Mitigation | Placement + query-aware | Structured headers + minimal set |
| Основной источник | [Liu et al., TACL 2024](https://aclanthology.org/2024.tacl-1.9/) | [Goldberg et al., cited in MLOps](https://mlops.community/the-impact-of-prompt-bloat-on-llm-output-quality/) |

**Ключевое уточнение (v1 analyst-5):** Lost in the Middle исследовался на retrieval tasks. Для structured instructions эффект слабее, потому что:
- Инструкции = imperative mode (правила поведения), не retrieval mode (поиск факта)
- Instruction fine-tuning создаёт bias к началу контекста
- Structured headers работают как attention anchors (v1 analyst-4)

**Но IFScale (v2 новое) показывает**: instruction-following degradation реальна даже для инструкций. 500 keyword-inclusion инструкций → 68% у лучших моделей. Это не Lost in the Middle, а **instruction capacity limit**.

### Бюджет токенов для pipe mode (`claude --print`)

**Рекомендуемое распределение 200K окна:**

| Компонент | Токены | % | Обоснование |
|-----------|--------|---|-------------|
| System/rules knowledge | **5-15K** | 2.5-7.5% | Каждый лишний токен = -adherence |
| Task description | 1-5K | 0.5-2.5% | Конкретная задача + AC |
| Context (code files, history) | 20-80K | 10-40% | Основной рабочий материал |
| Feedback (при retry) | 1-10K | 0.5-5% | Предыдущая ошибка + diff |
| Working space (thinking + output) | 80-120K | 40-60% | Для reasoning + code generation |

**Правило: rules ≤ 10% контекстного окна (≤20K при 200K).**

### Placement optimization для pipe mode

Anthropic рекомендует: **данные вверху, инструкции внизу** ([platform.claude.com](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/long-context-tips)):
- Queries at the end → +30% quality
- Instructions рядом с задачей → лучше adherence

Для Ralph prompt assembly order:

```
1. [TOP] Code context (файлы проекта, история, diff)
2. [MIDDLE] Knowledge rules (отфильтрованные по stage)
3. [BOTTOM] Task description + specific instructions
```

---

## Архитектурные рекомендации для нового проекта

### Tier 0: День 1 (zero-cost foundations)

1. **Stage-теги в формате знаний:**
   ```yaml
   ---
   stages: [execute, review, retry]  # или [all]
   priority: core  # core | extended
   ---
   ```
   Стоимость: 0 (просто metadata, не влияет на загрузку пока не активирована фильтрация).

2. **Token counting в prompt assembler:**
   ```go
   knowledgeTokens := estimateTokens(knowledgeBlock)
   if knowledgeTokens > maxKnowledgeBudget {
       log.Warn("knowledge budget exceeded: %d > %d", knowledgeTokens, maxKnowledgeBudget)
   }
   ```
   Даже без фильтрации — мониторинг роста.

3. **Placement order:** код → знания → задача (по Anthropic best practice).

### Tier 1: При достижении ~80 правил (Growth phase)

4. **Stage-фильтрация в prompt assembler:**
   ```go
   func filterByStage(rules []Rule, stage Stage) []Rule {
       var filtered []Rule
       for _, r := range rules {
           if r.MatchesStage(stage) {
               filtered = append(filtered, r)
           }
       }
       return filtered
   }
   ```

5. **Core/extended split:**
   - Core (~20 правил, всегда загружаются): naming, error wrapping, architecture
   - Extended (остальные, фильтруются по stage)

6. **Adherence metrics:** считать % соблюдения правил по review results.

### Tier 2: При достижении ~200 правил (Mature phase)

7. **Topic + stage фильтрация:** двумерная матрица (topic × stage)
8. **Priority-based truncation:** при превышении token budget — отбрасывать low-priority правила
9. **Violation-frequency boost:** часто нарушаемые правила → повышенный приоритет

### Tier 3: >500 правил (Scale phase) — отложить

10. **Content-based retry routing:** NLP-анализ ошибки → подбор правил
11. **RAG (BM25 или embeddings):** когда даже stage-фильтрация не помещается в budget
12. **MCP knowledge tool:** если MCP работает в pipe mode

### Что НЕ реализовывать

- **Iteration-based escalation**: нет evidence, высокая сложность, нулевой ROI
- **Pull-based retrieval (агент запрашивает)**: compliance 30-50% vs push 90-94% (analyst-7)
- **Semantic routing до 500+ правил**: embedding API = внешняя зависимость (analyst-7)

---

## Evidence из проектных исследований R1/R2/R3

### R1: Knowledge Extraction (20 источников)

**Ключевые данные для stage-фильтрации:**

1. **Context rot = фундаментальное ограничение** (Chroma Research, 18 моделей): 30-50% degradation при полном контексте vs компактном [R1-S5]. Это не проблема конкретной модели — все 18 frontier моделей деградируют. Для нового проекта: planning для context budget с первого дня.

2. **~150-200 инструкций = практический ceiling** [R1-S14 (HumanLayer)]. При 3-5 строках на правило = budget ~40-60 distinct rules per prompt. **Критично для stage-фильтрации**: если этап видит 40 правил вместо 120 — помещается в ceiling.

3. **Shuffled/atomized facts > organized narrative** [R1-S5 (Chroma)]. Organized text создаёт attention shortcuts — модель "скользит" по знакомой структуре. Atomized independent facts устойчивее к Lost in the Middle. **Импликация для stage-фильтрации**: после фильтрации правила из разных топиков перемешиваются — это *хорошо*, не плохо.

4. **CLAUDE.md framing problem** [R1-S6]: содержимое оборачивается "may or may not be relevant". **В pipe mode (`claude --print`) этой проблемы нет** — Ralph сам собирает промпт без framing. Преимущество pipe mode для knowledge injection.

5. **6-уровневая иерархия Claude Code** [R1-S4]: от managed policy до auto memory. В pipe mode Ralph обходит эту иерархию — он контролирует весь prompt. Свобода = ответственность: нет auto-загрузки, но полный контроль placement.

### R2: Knowledge Enforcement (40 источников)

**Критические данные для тройного барьера:**

6. **Triple barrier количественно** [R2]:
   - Compaction уничтожает правила (GitHub Issues #9796, #21925, #25265) — **не применимо к pipe mode** (нет compaction)
   - Context rot: -30-50% при полном контексте [R2-S5]
   - Volume ceiling: **15 imperative rules = 94% compliance** (SFEIR study) [R2-S25]. Файл с 125+ правилами → ~40-50% compliance

7. **Compliance по уровням enforcement** [R2]:

   | Механизм | Compliance | Tokens/turn |
   |----------|-----------|-------------|
   | .claude/rules/ (glob) | ~40-60% | 0 |
   | CLAUDE.md (с framing) | ~70-80% | 0 |
   | SessionStart hook | ~90-94% | ~15 |
   | PreToolUse hook | ~100% | ~20 |
   | PostToolUse hook (deterministic) | 100% | ~5 |

   **Для pipe mode**: аналог SessionStart = inject в начало промпта. Аналог PreToolUse = невозможен (нет hooks в --print). Следовательно, **правила в промпте = ceiling ~90-94%** при ≤15 правилах.

8. **15 правил = 94% — ключевое число** [R2-S25]. При stage-фильтрации:
   - execute видит ~40 из 120 правил → compliance ~70%
   - Если core = 15 правил + stage-specific = 25 → compliance ~80-85%
   - Если core = 15 правил only → compliance ~90-94%

   **Вывод для архитектуры**: core rules (≤15, всегда в промпте) + stage-specific (≤25, по этапу) = optimal split.

9. **Skills activation: 20% → 84% с hooks** [R2-S27 (Scott Spence)]. Без explicit trigger — passive knowledge используется в 1/5 случаев. **В pipe mode hooks невозможны**, но Ralph может эмулировать: inject правила как imperative instructions ("Ты ДОЛЖЕН...") вместо passive guidelines ("Рекомендуется...").

10. **DGM: конкретные failed attempts > абстрактные правила** [R2-S37]. SWE-bench: 20% → 50% при хранении истории неудачных попыток. **Для retry stage**: вместо набора правил — inject конкретную ошибку с предыдущей итерации + violation example.

11. **Self-Refine: +8.7 code optimization, +13.9 readability** [R2-S38]. **Для review stage**: inject self-critique prompt ("Перед завершением: перечитай top-5 violations и проверь каждый файл").

### R3: Alternative Methods (22 источника)

**Пороги масштаба:**

12. **Filesystem agent = 74.0% LoCoMo > Mem0 Graph 68.5%** [R3-S1 (Letta benchmark)]. Файловая система побеждает специализированные memory tools. **Для нового проекта**: не усложнять до тех пор, пока файлы работают. Pipe mode + файлы = optimal при <500 правил.

13. **RAG не оправдан при <500 записях** [R3]. Pre-loading 200 строк = ~3-4K токенов — within sweet spot. RAG экономит ~2K токенов, но добавляет: dependency (embedding API), latency (200-500ms), risk пропуска (recall < 100%).

14. **Break-even точки** [R3]:

    | Правил | Подход | Обоснование |
    |--------|--------|-------------|
    | <200 | File pre-loading | 100% recall, 0 deps, <1ms |
    | 200-500 | File + distillation | Compression keeps within budget |
    | 500-2000 | Hybrid (file core + selective retrieval) | Pre-loading saturates context |
    | >2000 | Full RAG or Graph RAG | File-based non-viable |

15. **MCP в pipe mode — не верифицировано** [R3]. Критический unknown: работает ли MCP в `claude --print`? Если нет — MCP knowledge server невозможен, и file injection = единственный путь.

16. **Convergent evolution** [R3]: MemGPT, MemOS, claude-mem, Copilot — все приходят к tiered memory + compression + selective injection. bmad-ralph уже реализует этот паттерн.

### Синтез: что R1/R2/R3 означают для stage-фильтрации

| Находка | Импликация для stage-фильтрации |
|---------|--------------------------------|
| 15 rules = 94% compliance [R2-S25] | Core ≤15 rules always loaded; stage-specific = bonus |
| 150-200 instruction ceiling [R1-S14] | Stage filter keeps each prompt under ceiling |
| Context rot 30-50% [R1-S5, R2-S5] | Fewer rules = less rot = higher quality per rule |
| Atomized > organized [R1-S5] | Post-filter shuffle is beneficial |
| Pipe mode = no framing [R1-S6] | Ralph injects as authoritative, not "maybe relevant" |
| Pipe mode = no hooks [R2] | Must embed all enforcement in prompt text |
| DGM failed attempts [R2-S37] | Retry stage: inject specific error, not generic rules |
| Self-Refine [R2-S38] | Review stage: inject self-critique step |
| RAG not needed <500 [R3] | File-based stage filter sufficient for years |

**Пересмотренная архитектура (с учётом R1/R2/R3):**

```
Prompt structure for each stage:

[CODE CONTEXT — files, diff, history]  ← top (data)

[CORE RULES — ≤15, always loaded]      ← high-attention zone
[STAGE RULES — ≤25, filtered by stage] ← mid (benefits from atomization)
[VIOLATION EXAMPLES — top-3 recent]    ← concrete > abstract [R2-S37]

[TASK — specific AC + instructions]    ← bottom (query position = +30%)
[SELF-REVIEW — for review/retry only]  ← bottom [R2-S38]
```

Token budget estimate:
- Core rules (15 × ~30 tokens): ~450 tokens
- Stage rules (25 × ~30 tokens): ~750 tokens
- Violation examples (3 × ~100 tokens): ~300 tokens
- Total knowledge: **~1500 tokens** (vs ~4000+ без фильтрации)
- Savings: **60%+ token reduction** для знаний

---

## Расхождения с первым раундом

| Вопрос | V1 (первый раунд) | V2 (текущий) | Причина расхождения |
|--------|-------------------|-------------|---------------------|
| Stage-фильтрация нужна? | НЕТ при 4K токенов | ДА, но активировать при >80 правил | IFScale: linear decay с первой инструкции для Claude |
| Порог фильтрации | 15K+ токенов | **~5K / ~80 правил** | IFScale даёт более точные пороги для instruction following |
| Iteration-based | Не рассматривалось | НЕТ | SCOPE + retry best practices: содержание > номер |
| Placement | Не рассматривалось | Код → знания → задача | Anthropic: queries at end +30% quality |
| Token budget | Не измерялось | ≤10% окна (≤20K) | Anthropic context engineering + IFScale |

**Важно**: V1 оценивал ситуацию bmad-ralph (122 правила уже есть, Claude Code CLI с glob). V2 проектирует архитектуру для нового проекта (0 правил, pipe mode, рост). Выводы не противоречат — они для разных контекстов.

---

## Источники

### Новые (v2)
1. [IFScale: How Many Instructions Can LLMs Follow at Once?](https://arxiv.org/html/2507.11538v1) — Jaroslawicz et al., 2025. 20 моделей, 10-500 инструкций, три паттерна деградации.
2. [Effective Context Engineering for AI Agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) — Anthropic, 2025. Token budget, sub-agents.
3. [The Impact of Prompt Bloat on LLM Output Quality](https://mlops.community/the-impact-of-prompt-bloat-on-llm-output-quality/) — MLOps Community, 2025. 3000-token threshold.
4. [SCOPE: Prompt Evolution for Enhancing Agent Effectiveness](https://arxiv.org/html/2512.15374v1) — 2025. Step-level feedback adaptation.
5. [Prompting Best Practices — Claude API Docs](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/long-context-tips) — Anthropic, 2025-2026. Placement, structure.
6. [RAG-MCP: Mitigating Prompt Bloat in LLM Tool Selection](https://arxiv.org/html/2505.03275v1) — 2025. 50%+ token reduction via retrieval.
7. [Mastering Retry Logic Agents](https://sparkco.ai/blog/mastering-retry-logic-agents-a-deep-dive-into-2025-best-practices) — SparkCo, 2025. Escalation strategies.
8. [Understanding LLM Performance Degradation](https://demiliani.com/2025/11/02/understanding-llm-performance-degradation-a-deep-dive-into-context-window-limits/) — Demiliani, 2025.

### Из первого раунда (baseline)
9. [Lost in the Middle](https://aclanthology.org/2024.tacl-1.9/) — Liu et al., TACL 2024. U-shaped attention.
10. [Context Rot — Chroma Research](https://research.trychroma.com/context-rot) — 18 LLM models, 20-35 point accuracy drop.
11. analyst-5 v1: `docs/reviews/analyst-5-dynamic-injection-report.md` — push compliance 90-94%.
12. analyst-7: `docs/reviews/analyst-7-alternatives-report.md` — 5-tier architecture, pull compliance 30-50%.
13. analyst-4: `docs/reviews/analyst-4-categorization-report.md` — пороги 30/80/150 правил.

### Из проектных исследований R1/R2/R3
14. R1: `docs/research/knowledge-extraction-in-claude-code-agents.md` — 20 источников. 6-уровневая иерархия Claude Code, context rot, atomized facts, framing problem.
15. R2: `docs/research/knowledge-enforcement-in-claude-code-agents.md` — 40 источников. Тройной барьер, 15 rules = 94% compliance, hooks enforcement, Self-Refine, DGM failed attempts.
16. R3: `docs/research/alternative-knowledge-methods-for-cli-agents.md` — 22 источника. File-based = 74% LoCoMo, RAG <500 не оправдан, convergent evolution tiered memory.

### Ключевые внешние источники (через R1/R2/R3)
17. [R1-S5] Chroma Research — Context Rot: 30-50% degradation, 18 моделей. https://research.trychroma.com/context-rot
18. [R2-S25] SFEIR — 15 imperative rules = 94% compliance. https://www.sfeir.dev/ia/claude-code-claude-md/
19. [R2-S37] DGM — Failed attempts history: SWE-bench 20%→50%. https://arxiv.org/abs/2505.22827
20. [R2-S38] Self-Refine — +8.7 code optimization. https://arxiv.org/abs/2303.17651
21. [R3-S1] Letta — Filesystem agent 74.0% LoCoMo > Mem0 Graph 68.5%. https://www.letta.com/blog/benchmarking-ai-agent-memory
22. [R1-S14] HumanLayer — ~150-200 instruction limit. https://humanlayer.dev/blog/writing-a-good-claude-md
23. [R2-S27] Scott Spence — Skills activation 20%→84% with hooks. https://scottspence.com/posts/claude-code-skills
