# Analyst-6: Нужна ли 3-layer distillation pipeline?

**Дата**: 2026-03-02
**Вопрос**: Нужна ли 3-слойная pipeline дистилляции (Go dedup → LLM compression → Go validation) или достаточно простого накопления знаний?

## Текущее состояние проекта

| Параметр | Значение |
|---|---|
| CLAUDE.md | 83 строки |
| .claude/rules/ (9 файлов) | 242 строки |
| **Итого** | **~325 строк** |
| Доля 200K контекста | ~3-4% (≈2600 токенов) |
| Правил за 5 эпиков | ~122 (32 stories, 192 findings) |

## Вопрос 1: При каком объёме LLM теряет качество instruction-following?

### Ключевые данные из исследований

**Context Rot (Chroma Research, 2025)**: Исследование 18 LLM показало, что надёжность модели значительно снижается с ростом длины input, даже на простых задачах (retrieval, text replication). Производительность неоднородна — модели не обрабатывают контекст равномерно [Context Rot, Chroma Research].

**Prompt Bloat (MLOps Community, 2024)**: Деградация reasoning замечена уже при ~3000 токенов system prompt — значительно ниже размеров context window. Длинные инструкции создают "instruction dilution", модели демонстрируют primacy/recency effects [MLOps Community].

**Lost in the Middle (Liu et al., TACL 2024)**: Классическая U-образная кривая — модели лучше всего используют информацию в начале и конце контекста, но деградируют на материале из середины [Liu et al., TACL 2024].

**GPT-4 degradation**: 15.4% деградация при расширении от 4K до 128K токенов [DEV Community].

**Effective context**: Заявленные 1M токенов часто имеют effective length всего 4K-32K токенов [HackerNoon].

### Вывод для bmad-ralph

Наши ~2600 токенов правил = **менее 1.5% от 200K окна**. Это глубоко в "безопасной зоне":
- Деградация reasoning начинается при ~3000 токенов **system prompt** (не общего контекста)
- Наши правила загружаются через glob-scoped .claude/rules/ — не весь набор одновременно
- При текущем темпе (+~25 правил/эпик) до порога ~3000 токенов system prompt понадобится ещё ~3-5 эпиков

**Порог беспокойства**: ~500-800 строк правил (≈5000-6500 токенов), что при текущем темпе наступит через ~8-12 эпиков.

## Вопрос 2: ROI сложной дистилляции vs append + manual cleanup

### 3-Layer Pipeline: затраты

| Компонент | Сложность | Риски |
|---|---|---|
| Go dedup (lexical) | Средняя — точное дублирование, fuzzy matching | False positives на похожих но разных правилах |
| LLM compression | Высокая — API calls, prompt engineering, quality control | Semantic drift, потеря нюансов |
| Go validation | Высокая — формат, структура, coverage проверки | Brittleness при изменении формата |
| **Итого** | **~2-3 story points** | **Ongoing maintenance** |

### Простое накопление: затраты

| Компонент | Сложность | Риски |
|---|---|---|
| Append в topic файл | Минимальная — одна строка на паттерн | Постепенный рост |
| Manual cleanup (per epic) | Низкая — human review ~30 мин | Человеческий фактор |
| **Итого** | **~0.1 story points** | **Масштабируется до ~500 строк** |

### Анализ ROI

**Текущий объём (325 строк)**: pipeline сэкономит ~0 строк — нечего сжимать.

**При 500 строках** (~через 7 эпиков): pipeline может сэкономить ~100-150 строк (20-30% compression). Экономия ~1000 токенов = 0.5% контекста. **ROI отрицательный** — стоимость разработки pipeline >> экономия контекста.

**При 1000+ строках** (~через 15+ эпиков): потенциальная экономия ~300-400 строк. Но к этому моменту context window модели вероятно вырастут до 500K-1M effective tokens. **ROI остаётся сомнительным**.

**LLMLingua comparison**: LLMLingua достигает 20x сжатия с ~1.5% потерей качества на reasoning [Microsoft Research, EMNLP 2023]. Но это для RAG-контекста (facts), не для imperative instructions. Сжатие инструкций ("always use errors.As" → ???) качественно отличается от сжатия фактов.

## Вопрос 3: Улучшает ли сжатие инструкций compliance?

### Доказательства "ЗА" сжатие

- LLMLingua-2 (ACL 2024): 3-6x быстрее с improved out-of-domain performance [LLMLingua-2]
- Shorter prompts снижают instruction dilution [MLOps Community]
- Меньше токенов = больше пространства для рабочего контекста

### Доказательства "ПРОТИВ" сжатия

- LLMLingua оптимизирован для **retrieval context** (документы, few-shot examples), не для **imperative rules** [LLMLingua.com]
- Сжатие императивных инструкций удаляет redundancy, которая может быть **reinforcement**: "ALWAYS use errors.As" с повторением в разных контекстах усиливает compliance
- Claude Code best practices рекомендуют **30-100 строк на файл**, не сжатые правила [Claude Code Docs]
- Правила bmad-ralph уже атомизированы (1 правило = 1 строка) — дальнейшее сжатие потеряет readability

### Вывод

Для **императивных инструкций** (правила кодирования, паттерны) сжатие **ухудшает** compliance:
- Удаляет контекст ("when function has N error returns" → "test all error paths" — теряется "when")
- Удаляет file:line citations — ключевой navigation aid
- Читаемость для human review падает

Для **фактического контекста** (документация, RAG) сжатие эффективно, но это не наш use case.

## Вопрос 4: Риск model collapse при итеративной LLM-компрессии

### Данные исследований

**Nature (2024)**: AI модели collapse при обучении на рекурсивно сгенерированных данных. Лексическое, синтаксическое и семантическое разнообразие последовательно снижается [Shumailov et al., Nature 2024].

**Knowledge Collapse (2025)**: Fluency сохраняется, но факты деградируют при recursive synthetic training [arXiv 2509.04796].

**Instruction-following collapse**: Prompt structure может ускорить или замедлить дегенерацию. Few-shot промпты создают structural dependencies через exemplars, которые повреждаются при recursive training [aclanthology.org].

**Mitigation**: Iterative accumulation (добавление к оригинальным данным, а не замена) стабилизирует поведение модели и снижает drift [Wikipedia: Model Collapse].

### Применение к нашему случаю

3-layer pipeline с LLM compression — это **iterative rewriting** правил:
1. Человек пишет правило из code review → v1
2. LLM сжимает → v2 (потенциальная потеря нюансов)
3. Через N эпиков, LLM пересжимает → v3 (drift усиливается)
4. После M итераций правило может потерять оригинальный смысл

**Оценка риска для bmad-ralph**: СРЕДНИЙ при >3 итерациях перекомпрессии. Но при текущем темпе (~1 cleanup в эпик) до опасного drift потребуется ~10+ итераций на одном правиле, что маловероятно.

Более реальный risk: **silent semantic narrowing** — LLM убирает "ненужный" контекст, который оказывается критичным для edge cases.

## Рекомендации

### Рекомендация: ПРОСТОЕ НАКОПЛЕНИЕ + ручная гигиена

**Обоснование**:

1. **Объём безопасен**: 325 строк = 3-4% контекста. Порог деградации далеко (~8-12 эпиков)
2. **ROI pipeline отрицательный**: стоимость разработки (2-3 SP) >> экономия контекста (0.5%)
3. **Сжатие вредит для правил**: императивные инструкции ≠ RAG-контекст
4. **Model collapse реален**: iterative LLM compression создаёт semantic drift
5. **Glob-scoping уже работает**: не все 242 строки загружаются одновременно
6. **Context windows растут**: 200K → 500K-1M в 2026-2027

### Конкретный план

| Объём | Действие | Триггер |
|---|---|---|
| < 500 строк | Append + manual dedup при retro | Каждый эпик |
| 500-800 строк | Ручная ревизия: удаление устаревших, merge related | Полугодовой review |
| > 800 строк | Пересмотр: архивация старых правил + возможная автоматизация Go dedup (без LLM) | По факту |

### Что НЕ делать

- **НЕ** строить LLM compression pipeline — ROI отрицательный, risk semantic drift
- **НЕ** автоматизировать Go validation на данном этапе — формат стабилен, manual review достаточен
- **НЕ** сжимать атомизированные правила — каждое уже 1 строка с citations

### Что МОЖНО сделать (low-cost improvements)

1. **Go dedup detector** (read-only): скрипт, находящий >80% fuzzy-similar строки — подсказка для ручного merge. ~0.5 SP.
2. **Автоматический line count check** в CI: warning при >500 строк total. ~0.1 SP.
3. **Staleness detector**: правила ссылающиеся на файлы/строки, которых больше нет. ~0.5 SP.

## Источники

- [Context Rot — Chroma Research](https://research.trychroma.com/context-rot)
- [Lost in the Middle — Liu et al., TACL 2024](https://aclanthology.org/2024.tacl-1.9/)
- [Prompt Bloat — MLOps Community](https://mlops.community/the-impact-of-prompt-bloat-on-llm-output-quality/)
- [LLMLingua — Microsoft Research](https://www.microsoft.com/en-us/research/blog/llmlingua-innovating-llm-efficiency-with-prompt-compression/)
- [LLMLingua-2 — ACL 2024](https://llmlingua.com/llmlingua2.html)
- [Model Collapse — Nature 2024](https://www.nature.com/articles/s41586-024-07566-y)
- [Knowledge Collapse — arXiv 2025](https://arxiv.org/html/2509.04796v1)
- [Claude Code Best Practices](https://code.claude.com/docs/en/best-practices)
- [Claude Code Memory](https://code.claude.com/docs/en/memory)
- [Prompt Length vs Context Window — DEV Community](https://dev.to/superorange0707/prompt-length-vs-context-window-the-real-limits-behind-llm-performance-3h20)
- [Long System Prompts — Data Science Collective](https://medium.com/data-science-collective/why-long-system-prompts-hurt-context-windows-and-how-to-fix-it-7a3696e1cdf9)
- [Prompt Compression Survey — NAACL 2025](https://github.com/ZongqianLi/Prompt-Compression-Survey)
