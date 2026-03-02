# Analyst-5: Динамическая инъекция знаний по этапам цикла Ralph

**Дата:** 2026-03-02
**Задача:** Исследовать, имеет ли смысл подавать РАЗНЫЕ знания на разных этапах цикла Ralph (execute/retry/review)
**Метод:** Deep research с evidence из академических и индустриальных источников

---

## Резюме (Executive Summary)

**Вердикт: НЕТ, при текущем объёме (~4K токенов) динамическая фильтрация НЕ оправдана.**

Текущие ~122 правила (~242 строки, ~4K токенов) находятся значительно ниже порога, при котором context rot и Lost in the Middle начинают значимо влиять на качество. Усложнение архитектуры ради фильтрации создаст больше проблем (maintenance overhead, risk of missing rules), чем решит. Claude Code уже имеет встроенный glob-based conditional loading, который достаточен.

**Однако**: при росте объёма правил до 15-20K+ токенов вопрос стоит пересмотреть.

---

## Q1: Selective Context Injection — улучшает ли качество?

### Evidence

1. **Anthropic (Context Engineering for AI Agents)**: "Good context engineering means finding the smallest possible set of high-signal tokens that maximize the likelihood of some desired outcome." Selective loading предпочтительнее full pre-loading — но речь идёт о *больших объёмах* данных (файлы, документация, history), а не о коротких наборах правил.

2. **PromptLayer research**: "Retrieve less than 1,000 tokens of high-similarity content. Quality beats quantity every time." Однако это про RAG-retrieval документов, не про instruction sets.

3. **OpenAI Agent Guide**: "Adjust spec detail to task complexity—don't over-spec a trivial one (the agent might get tangled or use up context on unnecessary instructions)." Задаёт принцип пропорциональности.

4. **Claude Code modular rules**: "Modular rules improve performance by reducing contextual noise by 40%." Но это при сравнении *монолитного CLAUDE.md на сотни строк* vs модульной системы, а не при фильтрации уже модульного набора в 4K токенов.

### Анализ для bmad-ralph

- Текущий объём правил: **~242 строки / ~4K токенов** — это порядка 2-3% от типичного контекстного окна Claude (200K).
- При таком объёме selective injection даёт **маргинальное** улучшение качества.
- Риск selective injection: если фильтр неправильно исключит правило, нарушение будет гарантировано. False negative фильтрации опаснее, чем marginal noise от полного набора.

### Вывод

**Selective injection улучшает качество при объёмах >10K токенов инструкций.** При ~4K токенов полная загрузка безопаснее и не создаёт заметного noise.

---

## Q2: "Lost in the Middle" — критичен ли для инструкций?

### Evidence

1. **Liu et al., 2024 (ACL TACL)**: Оригинальное исследование "Lost in the Middle" — performance падает на 30%+ когда релевантная информация в середине длинного контекста. **Но:** исследование тестировало *retrieval tasks* (multi-document QA, key-value retrieval), а НЕ instruction following.

2. **Chroma Context Rot Study (2025)**: С 20 документами (~4K токенов) accuracy падает с 70-75% до 55-60% для информации в середине. **Но:** это retrieval accuracy для *данных*, не для *инструкций* в system prompt.

3. **Anthropic рекомендации**: "Place long documents and inputs (~20K+ tokens) near the top of your prompt." Порог внимания начинается с ~20K+ токенов документов. System prompt правила — это structured instructions, не документы для retrieval.

4. **RoPE decay**: Rotary Position Embedding создаёт bias к началу и концу последовательности. Однако modern models (Claude 4.x, GPT-4.1) имеют существенные улучшения attention calibration именно для structured instructions.

### Критическое различие: Instructions vs Documents

Lost in the Middle исследовалось на задачах *поиска факта в массе документов*. System prompt instructions — это другой жанр:
- **Инструкции** — модель применяет их как *правила поведения* (imperative mode)
- **Документы** — модель ищет *конкретный факт* (retrieval mode)

Модели обучены следовать инструкциям system prompt целиком, а не "искать нужную инструкцию среди массы ненужных."

### Вывод

**Lost in the Middle НЕ критичен для structured instructions объёмом <10K токенов.** Эффект актуален для document retrieval в длинных контекстах (20K+), а не для наборов правил в system prompt.

---

## Q3: При каком объёме фильтрация помогает?

### Evidence

1. **Chroma study threshold**: "An ideal working window of 40% of the model's context capacity" — за пределами ~80K токенов (для 200K модели) начинается серьёзная деградация. 4K токенов = **2%** от окна.

2. **Context rot curve**: Деградация **градуальная**, не пороговая. Но заметный перелом происходит при:
   - ~10K-20K токенов: начинает влиять на recall отдельных фактов
   - ~50K+ токенов: instruction following заметно деградирует
   - ~80K+ токенов (40% window): требуется активная компрессия

3. **ACON research (2025)**: Context compression снижает input tokens на 40-60% при сохранении performance — но это для *multi-step agent trajectories* (десятки тысяч токенов history), не для коротких instruction sets.

4. **Практический порог для instructions**: PromptLayer отмечает 30% drop при ~113K токенов vs 300 токенов focused. Но при сравнении 4K vs 2K разница *статистически незначима*.

### Шкала для bmad-ralph

| Объём правил | Фильтрация | Обоснование |
|---|---|---|
| <5K токенов | НЕ нужна | Marginal noise, high false-negative risk |
| 5K-15K токенов | Опциональна | Glob-based scoping достаточен |
| 15K-30K токенов | Рекомендуется | Context rot начинает влиять |
| 30K+ токенов | Обязательна | Без фильтрации instruction following деградирует |

### Вывод

**~4K токенов (~122 правила) — значительно ниже порога.** Фильтрация начнёт давать заметное улучшение при ~15K+ токенов инструкций. До этого порога Claude Code glob-based loading (уже реализован) достаточен.

---

## Q4: Полный контекст vs фильтрованный — что лучше?

### Evidence

1. **Anthropic (Context Engineering)**: Рекомендует "lightweight identifiers + dynamic loading at runtime" — но для *файлов и документов*, не для коротких rule sets. Claude Code сам реализует этот паттерн через grep/glob.

2. **Agent Skills (Anthropic)**: "Progressive disclosure is the core design principle" — skills загружаются on-demand. Но skills — это *тысячи строк* per skill, а rules — десятки строк per file.

3. **HumanLayer (Writing a good CLAUDE.md)**: "An LLM will perform better on a task when its context window is full of focused, relevant context." Ключевое слово — **focused**. При 4K токенов правил контекст уже focused (только правила, без bloat).

4. **Setec blog (Modular Rules)**: Claude Code glob patterns уже реализуют conditional loading: тестовые правила грузятся только при работе с тестами. Это *правильный уровень* фильтрации для текущего объёма.

### Сравнение стратегий для bmad-ralph

| Стратегия | Pros | Cons | Рекомендация |
|---|---|---|---|
| **Полная загрузка всех правил** | Простота, нет false negatives, нет maintenance | ~4K токенов "шума" для неактуальных правил | **Текущий подход — ОК** |
| **Glob-based filtering** (текущее) | Автоматическое, без custom кода | Не фильтрует по этапу цикла | **Оптимально для текущего объёма** |
| **Этап-specific injection** (execute/review/retry) | Минимальный noise per stage | Сложность, maintenance, risk of missing rules | **Преждевременная оптимизация** |

### Вывод

**Для ~4K токенов полный контекст + glob-based filtering = оптимальная стратегия.** Дополнительная фильтрация по этапам цикла — преждевременная оптимизация с отрицательным ROI.

---

## Рекомендации

### Сейчас (при ~4K токенов правил)

1. **Сохранить текущий подход**: полная загрузка всех правил через glob-based scoping.
2. **НЕ реализовывать** динамическую фильтрацию по этапам цикла.
3. **Причина**: overhead реализации + risk false negatives > marginal quality gain.

### Порог пересмотра

Пересмотреть решение когда:
- Объём правил превысит **~15K токенов** (~400+ правил, ~600+ строк)
- Появятся конкретные метрики деградации instruction following (повторяющиеся violations одних и тех же правил)
- Claude Code изменит механизм загрузки правил

### Что стоит сделать вместо фильтрации

1. **Качество правил > количество**: дистилляция и слияние дубликатов (Epic 6 Story 6.3)
2. **Правильный glob scoping**: убедиться что scope hints в файлах правил точно отражают релевантность
3. **Мониторинг объёма**: отслеживать рост правил после каждого epic и сравнивать с порогом 15K

---

## Источники

1. [Anthropic — Effective Context Engineering for AI Agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) — ключевой гайд по context engineering
2. [Chroma — Context Rot: How Increasing Input Tokens Impacts LLM Performance](https://research.trychroma.com/context-rot) — исследование 18 LLM моделей
3. [Liu et al. (2024) — Lost in the Middle: How Language Models Use Long Contexts (ACL TACL)](https://aclanthology.org/2024.tacl-1.9/) — оригинальное исследование Lost in the Middle
4. [PromptLayer — Why LLMs Get Distracted and How to Write Shorter Prompts](https://blog.promptlayer.com/why-llms-get-distracted-and-how-to-write-shorter-prompts/) — практические пороги деградации
5. [Anthropic — Equipping Agents with Agent Skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills) — progressive disclosure pattern
6. [Setec Blog — Modular Rules in Claude Code](https://claude-blog.setec.rs/blog/claude-code-rules-directory) — glob-based conditional loading
7. [ACON: Optimizing Context Compression for Long-horizon LLM Agents (2025)](https://arxiv.org/abs/2510.00615) — context compression для агентов
8. [OpenAI — A Practical Guide to Building Agents (2025)](https://cdn.openai.com/business-guides-and-resources/a-practical-guide-to-building-agents.pdf) — instruction proportionality
9. [Claude API Docs — Long Context Prompting Tips](https://docs.anthropic.com/en/docs/build-with-claude/prompt-engineering/long-context-tips) — placement strategies
10. [Morphllm — Claude Code Best Practices (2026)](https://www.morphllm.com/claude-code-best-practices) — modular rules performance data
