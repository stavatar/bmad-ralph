# Управление контекстным окном LLM: критичность и выгоды

**Дата:** 2026-03-06
**Контекст:** Обоснование архитектурного решения bmad-ralph — свежая сессия per task, контроль заполнения контекста

---

## Executive Summary

Исследования 2024-2026 годов однозначно подтверждают: **деградация качества LLM при заполнении контекстного окна — доказанный факт**, а не теоретический риск. Эффект называется **context rot** (Chroma Research, 2025) и проявляется **задолго до** достижения лимита окна. Стратегия bmad-ralph — свежая сессия per task с контролируемым контекстом — является **научно обоснованным** подходом к максимизации качества агентных сессий.

---

## 1. Context Rot: доказательная база

### 1.1. Исследование Chroma (18 моделей, 2025)

Chroma Research протестировала 18 state-of-the-art моделей (GPT-4.1, Claude 4, Gemini 2.5, Qwen3) на задачах retrieval и text replication при различных длинах контекста.

**Ключевой вывод:**

> *"Across all experiments, model performance consistently degrades with increasing input length."*
> — [Context Rot, Chroma Research](https://research.trychroma.com/context-rot)

Деградация начинается **значительно раньше** лимита окна:

> *"A model with 200K tokens of capacity can start degrading at 50K."*
> — [Understanding AI](https://www.understandingai.org/p/context-rot-the-emerging-challenge)

**Результаты по моделям:**
- **Claude Opus 4** — самая медленная деградация, но начинает отказываться от задач (2.89% refusals)
- **Claude Sonnet 4** — наименьший уровень галлюцинаций
- **GPT модели** — наибольший уровень галлюцинаций (~2.55% refusal rate)
- **Gemini 2.5 Pro** — наибольшая вариативность, генерирует случайный вывод начиная с 500-750 слов контекста

**Критический факт:** Focused prompts (~300 tokens) **драматически** превосходят full prompts (~113k tokens) на задаче LongMemEval — даже при включённом reasoning/thinking mode.

### 1.2. Исследование "Lost in the Middle" (Stanford/ACL, 2023-2024)

Фундаментальная работа Liu et al. (Stanford) доказала **U-shaped performance curve**:

> *"Performance is often highest when relevant information occurs at the beginning or end of the input context, and significantly degrades when models must access relevant information in the middle of long contexts."*
> — [Liu et al., ACL 2024](https://arxiv.org/abs/2307.03172)

**Масштаб деградации:**

> *"Performance can degrade by more than 30% when relevant information shifts from the start or end positions to the middle of the context window."*

**Причина** — в архитектуре Transformer: Rotary Position Embedding (RoPE) создаёт long-term decay, приоритизируя начало и конец последовательности.

### 1.3. NoLiMa Benchmark (2025)

> *"At 32k tokens, 11 out of 12 tested models dropped below 50% of their performance in short contexts."*
> — [16x Engineer](https://eval.16x.engineer/blog/llm-context-management-guide)

Конкретные цифры для coding-релевантных моделей:

| Модель | 8k tokens | 32k tokens | 120k tokens |
|--------|-----------|------------|-------------|
| Claude Sonnet 4 (Thinking) | 97.2% | 91.7% | 81.3% |
| DeepSeek v3.1 (Thinking) | 80.6% | 63.9% | 62.5% |
| Gemini 2.5 Pro | 80.6% | 91.7% | 87.5% |

Claude Sonnet 4 теряет **~16 процентных пунктов** при переходе с 8k на 120k tokens.

### 1.4. Академическое исследование (Ponnusamy et al., 2025)

> *"Performance deteriorates in non-linear ways corresponding to Key-Value cache expansion. [...] architectural advantages may be obscured by infrastructure constraints when processing high token volumes."*
> — [Context Discipline and Performance Correlation, arXiv 2601.11564](https://ui.adsabs.harvard.edu/abs/2026arXiv260111564A/abstract)

Llama-3.1-70B и Qwen1.5-14B показывают **нелинейную** деградацию — не плавное снижение, а резкие провалы при определённых объёмах контекста.

---

## 2. Влияние на coding agents

### 2.1. JetBrains Research: observation masking (2025)

JetBrains протестировала стратегии управления контекстом на 500 задачах SWE-bench Verified:

> *"With Qwen3-Coder 480B: observation masking achieved **2.6% higher solve rates** while costing **52% less**."*
> — [JetBrains Research Blog](https://blog.jetbrains.com/research/2025/12/efficient-context-management/)

**Ключевое открытие:** LLM-суммаризация (сжатие истории) оказалась **хуже** простого маскирования (удаления старых observation):

> *"LLM-generated summaries caused agents to run 13-15% longer than masking approaches. Summaries obscure failure signals that normally trigger task abandonment."*

Агенты с суммаризацией **не понимают, что застряли**, потому что сжатая история скрывает признаки провала.

### 2.2. Anthropic: context editing (2025-2026)

Anthropic внедрила context editing в Claude platform:

> *"Context editing alone delivered a **29% improvement** in agentic search performance. Combining memory tool with context editing improved performance by **39%** over baseline."*
> — [Anthropic Blog](https://claude.com/blog/context-management)

> *"In a 100-turn web search evaluation, context editing reduced token consumption by **84%** while enabling completion of workflows that would otherwise fail."*

### 2.3. Claude Code: compaction threshold

Claude Code улучшил качество, перейдя на **раннее срабатывание** auto-compact:

> *"Claude Code improved by triggering auto-compact earlier (around 75% context usage instead of 90%+), reserving approximately 25% of the context window as 'working memory' for reasoning."*
> — [HyperDev](https://hyperdev.matsuoka.com/p/how-claude-code-got-better-by-protecting)

**Старый паттерн:** заполнить до 90% → начать новую операцию → кончился контекст → принудительный compact → потеря контекста

**Новый паттерн:** достичь 75% → завершить текущую операцию → graceful compact → чистый старт

Но даже это не идеально — пользователи сообщают о проблемах:

> *"Auto-compact triggering at 8-12% remaining context instead of 95%+, causing constant interruptions."*
> — [GitHub Issue #13112](https://github.com/anthropics/claude-code/issues/13112)

> *"The autocompact buffer consuming 45k tokens—22.5% of the context window gone before writing a single line of code."*

---

## 3. Количественная модель: почему 50% — оптимальный порог

### 3.1. Бюджет контекстного окна

Для Claude Code с 200k token window:

| Компонент | Tokens | % окна |
|-----------|--------|--------|
| System prompt + tools | ~15-20k | ~10% |
| CLAUDE.md + rules | ~5-10k | ~5% |
| MCP tools (если есть) | ~10-15k | ~7% |
| **Overhead до начала работы** | **~30-45k** | **~22%** |
| Задача + контекст (промпт ralph) | ~5-15k | ~7% |
| **Свободно для работы** | **~140-165k** | **~70%** |

Если агент использует 50% окна на чтение кода и reasoning → остаётся ~20% на генерацию решения. Это **достаточно**.

Если агент заполняет 80% → остаётся ~0% — compaction неизбежен, потеря контекста, деградация.

### 3.2. Зоны качества

На основе исследований можно выделить три зоны:

```
0%────────25%────────50%────────75%────────100%
│  Зелёная зона  │  Жёлтая зона  │ Красная зона │
│ Макс. качество │  Деградация   │   Провал     │
│ retrieval ~95% │  retrieval    │  retrieval   │
│ reasoning 100% │  ~80-90%      │  <70%        │
│                │  "lost in     │  compaction   │
│                │   middle"     │  галлюцинации │
```

- **0-50%**: Модель работает на максимуме. Attention mechanism не перегружен. Информация из начала и конца prompt одинаково доступна.
- **50-75%**: Начинается «lost in the middle». Информация из середины prompt теряется. Recency bias усиливается. Reasoning quality снижается.
- **75-100%**: Compaction triggers. Потеря контекста. Галлюцинации. Модель «забывает» ранние инструкции.

### 3.3. Экономика

JetBrains: observation masking = **52% экономия** при **+2.6% solve rate**.
Anthropic: context editing = **84% снижение token consumption** при **+29% performance**.

Держать контекст компактным — это не только качество, но и **деньги**. Каждый token в промпте оплачивается, а при caching — ещё и cache miss penalty.

---

## 4. Стратегии управления контекстом (ранжированные по эффективности)

### 4.1. Свежая сессия per task (bmad-ralph подход)

**Принцип:** Каждая задача получает чистую сессию с минимальным, точно подобранным контекстом.

**Выгоды:**
- Контекст всегда в «зелёной зоне» (0-50%)
- Нет накопления мусора от предыдущих задач
- Промпт содержит ровно то, что нужно
- `--max-turns` ограничивает рост внутри сессии
- Предсказуемая стоимость per task

**Подтверждение из исследований:**

> *"Focused prompts (~300 tokens) dramatically outperform full prompts (~113k tokens) across all model families."*
> — Chroma Research (LongMemEval)

> *"Start new sessions for each distinct task to maintain focused, relevant context."*
> — [16x Engineer](https://eval.16x.engineer/blog/llm-context-management-guide)

**Это именно то, что делает bmad-ralph:** одна задача = одна свежая сессия.

### 4.2. Observation masking (SWE-Agent подход)

**Принцип:** Сохранять reasoning и actions, но заменять старые environment observations плейсхолдерами.

**Выгоды:**
- Работает внутри одной длинной сессии
- Дешевле суммаризации
- +2.6% solve rate, -52% cost

**Ограничения:**
- Требует доступа к prompt internals (не всегда возможно с CLI-агентами)
- Всё равно хуже свежей сессии для отдельных задач

### 4.3. LLM-суммаризация (OpenHands/Cursor подход)

**Принцип:** Отдельная модель сжимает старую историю в summary.

**Выгоды:**
- Теоретически бесконечные сессии
- Сохраняет ключевые решения

**Проблемы:**
- Агенты работают на **13-15% дольше** (JetBrains)
- Суммаризация **скрывает failure signals**
- Дополнительные API-вызовы = +7% стоимости
- Информация неизбежно теряется при сжатии

### 4.4. Auto-compaction (Claude Code built-in)

**Принцип:** Claude Code сам сжимает историю при приближении к лимиту.

**Выгоды:**
- Zero-config, работает автоматически
- Сохраняет code patterns и key decisions

**Проблемы:**
- Непредсказуемый момент срабатывания
- Потеря контекста при неудачном compaction
- 22.5% overhead на buffer
- Пользователи сообщают о багах и degraded performance

---

## 5. Выводы для bmad-ralph

### 5.1. Текущая архитектура — научно обоснована

Решение «одна задача = одна свежая сессия» подтверждается:
- Chroma Research: focused prompts >> full prompts
- JetBrains: observation masking > sumarization > no management
- Anthropic: context editing = +29% performance
- NoLiMa: 11/12 моделей теряют >50% качества на 32k tokens

### 5.2. `--max-turns` — критический guard rail

Ограничение количества turns не даёт контексту расти бесконтрольно. Если средняя длина turn = 3-5k tokens, то `--max-turns 15` ≈ 45-75k tokens рабочего контекста — это **~35-40%** от 200k окна Claude. Безопасная «зелёная зона».

### 5.3. Раздельные сессии для execute и review — двойная выгода

1. **Execute** получает свежий контекст с задачей + knowledge
2. **Review** получает свежий контекст с diff + findings
3. Ни одна сессия не несёт «мусор» от другой
4. Каждая работает в оптимальной зоне контекста

### 5.4. Что можно улучшить

- **Мониторинг**: отслеживать `num_turns` и cost per session — если turns близко к max, задача слишком большая
- **Adaptive max-turns**: маленькая задача → меньше turns, большая → больше (но с потолком)
- **Prompt size audit**: периодически проверять размер промпта (execute.md + injected content) — не допускать раздувания

---

## Источники

1. [Context Rot: How Increasing Input Tokens Impacts LLM Performance — Chroma Research (2025)](https://research.trychroma.com/context-rot)
2. [Lost in the Middle: How Language Models Use Long Contexts — Liu et al., Stanford/ACL (2023-2024)](https://arxiv.org/abs/2307.03172)
3. [Context Discipline and Performance Correlation — Ponnusamy et al., arXiv (2025)](https://ui.adsabs.harvard.edu/abs/2026arXiv260111564A/abstract)
4. [Cutting Through the Noise: Smarter Context Management for LLM-Powered Agents — JetBrains Research (2025)](https://blog.jetbrains.com/research/2025/12/efficient-context-management/)
5. [Managing Context on the Claude Developer Platform — Anthropic (2025)](https://claude.com/blog/context-management)
6. [How Claude Code Got Better by Protecting More Context — HyperDev (2026)](https://hyperdev.matsuoka.com/p/how-claude-code-got-better-by-protecting)
7. [LLM Context Management: How to Improve Performance and Lower Costs — 16x Engineer (2026)](https://eval.16x.engineer/blog/llm-context-management-guide)
8. [Context rot: the emerging challenge — Understanding AI (2025)](https://www.understandingai.org/p/context-rot-the-emerging-challenge)
9. [NoLiMa Benchmark — via 16x Engineer (2025)](https://eval.16x.engineer/blog/llm-context-management-guide)
10. [Context Compaction — Claude API Docs](https://platform.claude.com/docs/en/build-with-claude/compaction)
11. [Auto-Compact Bug Report — GitHub Issue #13112](https://github.com/anthropics/claude-code/issues/13112)
