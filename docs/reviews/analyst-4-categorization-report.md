# Категоризация извлечённых знаний для CLI-агентов: один файл vs множество файлов

**Дата:** 2026-03-02 (v2 — расширенное исследование)
**Автор:** analyst-4 (knowledge-arch-research)
**Контекст:** bmad-ralph — Go CLI, ~122 правила в 8 файлах `.claude/rules/`

## Executive Summary

- Категоризация правил по темам **оправдана и даёт измеримые преимущества** начиная с ~50-80 правил (~2000-4000 токенов системного промпта) [S2, S4, S6]
- Ключевое преимущество — не сама категоризация, а **glob-scoped контекстная фильтрация**: загрузка только релевантных правил снижает noise и экономит context window [S3, S5, S8]
- «Lost in the middle» эффект подтверждён: LLM хуже следует инструкциям в середине длинного контекста — структурирование с заголовками частично нивелирует [S2]
- Практический консенсус экосистемы (Claude Code, Cursor, Copilot): **модульные файлы по темам > монолитный файл**, при условии что файлы < 500 строк и имеют описательные имена [S3, S5, S8, S9]
- Context rot — реальная проблема: даже при perfect retrieval производительность LLM деградирует с ростом контекста [S4, S6]
- Текущая архитектура bmad-ralph (7 тематических файлов + wsl-ntfs + glob scoping) **соответствует best practices** и не нуждается в структурном изменении

## Исследовательский вопрос и scope

**Основной вопрос:** Оправдана ли разбивка ~122 правил на 7+ тематических файлов, или один консолидированный файл был бы не хуже для LLM-агента?

**Scope:** CLI-агенты на базе LLM (Claude Code, Cursor, Copilot); glob-scoped загрузка правил; Go-проекты средней сложности.

**Исключения:** RAG-подходы (исследуются отдельно в задаче #7), fine-tuning, мультимодальные агенты.

## Методология

- Веб-поиск по 7 тематическим направлениям (instruction following, chunking, context rot, lost-in-the-middle, rules organization, ecosystem practices, system prompt length)
- Глубокий анализ содержимого 12 источников через WebFetch
- Composite evidence из: 3 академических papers, 5 practitioner guides, 4 developer experience reports
- **Ограничения:** нет прямых A/B тестов «один файл vs много файлов» для CLI-агентов; выводы основаны на convergent evidence из смежных областей

---

## Ключевые находки

1. **Context rot реален и количественно измерим.** Производительность LLM деградирует с ростом входного контекста даже при perfect retrieval — точность падает на 20-35 пунктов при длинных инструкциях [S4, S6]

2. **«Lost in the middle» эффект подтверждён академически.** U-образная кривая внимания: начало и конец контекста получают больше attention, середина теряется. Instruction-tuning частично компенсирует, но не устраняет [S2]

3. **Chunking — established best practice в prompt engineering.** Разбивка длинных инструкций на фокусированные блоки улучшает точность следования, потому что модель работает с конкретным scope, а не с размытым набором. «Keep chunks concise yet contextual, order them logically, group related information together» [S1, S7]

4. **Glob-scoped загрузка — главный multiplier.** Не категоризация сама по себе, а контекстная фильтрация даёт основной выигрыш: загружаются только правила, релевантные текущему файлу [S3, S5, S8]

5. **Cursor рекомендует файлы ≤500 строк.** Cursor официально рекомендует разбивать правила на файлы не более 500 строк. Монолитный подход прямо назван «LLM anti-pattern, bloating the model's context with huge blocks of text» [S9]

6. **Экосистемный консенсус: модульность > монолит.** Claude Code, Cursor, Windsurf конвергируют к модульной организации. Copilot — единственный holdout с монолитным `.github/copilot-instructions.md`, и community активно запрашивает directory-based rules [S10]

7. **Описательные имена файлов = semantic anchors.** Имя файла `test-error-patterns.md` информативнее чем раздел «Errors» в монолитном файле — оно работает как метаданные для загрузчика и для разработчика [S5, S11]

---

## Анализ

### Секция 1: Как LLM обрабатывает структурированные vs неструктурированные инструкции

Transformer-модели обрабатывают все входные токены параллельно через self-attention, но attention distribution неравномерна. Исследование «Lost in the Middle» (Liu et al., TACL 2024) [S2] обнаружило **U-shaped performance curve**: информация в начале (primacy bias) и конце (recency bias) контекста получает значительно больше внимания, чем в середине.

Для набора из 122 правил в одном файле (~5000-6000 токенов) это означает: правила в позициях 40-80 будут систематически хуже «запоминаться». Разбивка на 7 файлов по ~15-20 правил создаёт 7 «начал» и 7 «концов», более равномерно распределяя attention. Однако этот аргумент **работает только если файлы не конкатенируются в один блок** — при конкатенации эффект исчезает.

Markdown-заголовки (`## Section`) и scope-маркеры (`# Scope: ...`) работают как **attention anchors**. Prompt engineering best practices рекомендуют: «Separate different sections of prompts with clear markers (triple quotes, XML tags, headings) to help models distinguish instructions from content» [S7, S12].

Structured prompts с секционированием «significantly improve instruction following compared to flat text» (Sahoo et al., 2024). Но ключевой insight: выигрыш даёт **структура внутри текста** (заголовки, разделители), а не разделение на физические файлы.

**Вывод:** Структурирование объективно помогает LLM, но физическое разделение на файлы ≈ эквивалентно секциям в одном файле. Преимущество файлов проявляется только при **selective loading**.

### Секция 2: Пороговые значения — когда категоризация начинает помогать

Прямых benchmark-данных «N правил в 1 файле vs K файлах» не обнаружено. Composite evidence из IFScale benchmark, context rot исследований и prompt engineering practice позволяет обозначить пороги:

| Объём правил | Токены (~) | Рекомендация | Обоснование |
|---|---|---|---|
| < 30 правил | < 1500 | Один файл достаточен | Весь контент в зоне высокого attention [S2] |
| 30-80 правил | 1500-4000 | Категоризация опциональна | «Lost in the middle» начинает проявляться [S2, S4] |
| 80-150 правил | 4000-8000 | Категоризация рекомендована | Context rot measurable, noise снижает precision [S4, S6] |
| > 150 правил | > 8000 | Категоризация + фильтрация обязательна | Без glob-scoping — значимая деградация [S3, S4] |

Контекстная рекомендация: «Only use 70-80% of the full context window to avoid accuracy drop» [S6]. Системный промпт конкурирует с кодом и историей разговора за context budget. При 200K context window системный промпт >8000 токенов — это всего 4%, но на практике системные инструкции получают **привилегированное attention** и конкурируют непропорционально.

**Текущий статус bmad-ralph:** 122 правила в 8 файлах (в среднем ~15 правил/файл) — далеко ниже любого порога перегрузки одного файла, но в зоне где glob-scoped фильтрация даёт ощутимый выигрыш.

### Секция 3: Cognitive load от множества файлов vs один файл

Ключевой вопрос: не создаёт ли множество файлов «cognitive overhead» на LLM?

**LLM не «видит» файловую структуру.** Модель получает текст, вставленный в контекст — будь то из 1 файла или из 8. Cognitive load определяется **объёмом текста в контексте**, не количеством файлов-источников. 7 файлов по 20 правил ≡ 1 файл с 140 правилами для модели (при полной загрузке).

**Overhead множества файлов минимален.** 8 файлов добавляют ~8 заголовков + ~8 scope-комментариев ≈ 200-300 токенов overhead. При 5000+ токенов содержания это <6% — пренебрежимо.

**Что реально влияет на quality:**
- **Общий объём правил в контексте** — ключевой фактор деградации [S4, S6]
- **Наличие нерелевантных правил** — context rot усиливается «distractor» контентом [S4]
- **Позиция критических правил** — начало и конец контекста ≫ середина [S2]
- **Непротиворечивость** — конфликтующие правила из разных файлов сложнее обнаружить при maintenance

**Вывод:** Количество файлов не создаёт значимой нагрузки на LLM. Фильтрация по glob-паттернам превращает множество файлов из maintenance convenience в **performance advantage**.

### Секция 4: Glob-scoped загрузка — ключевой enabler

Glob-scoped загрузка — аналог «just-in-time» delivery правил. Вместо загрузки всех 122 правил при каждом запросе, агент получает только релевантные:

**Пример для bmad-ralph (с hypothetical `paths` frontmatter):**
- Работа с `runner/runner.go` → `code-quality-patterns.md` (~28 правил)
- Работа с `runner/runner_test.go` → все `test-*.md` + `code-quality-patterns.md` (~94+28 правил)
- Работа с `.goreleaser.yaml` → только базовые правила из CLAUDE.md
- Работа с WSL-специфичным кодом → `wsl-ntfs.md` (~12 правил)

Это реализует принцип **precision over recall**: лучше загрузить 30 точно релевантных правил, чем 122 «на всякий случай». Context rot research [S4, S6] подтверждает: меньше нерелевантного контекста = лучше следование релевантным инструкциям.

**Реализация в Claude Code:**
- `paths` frontmatter в `.claude/rules/*.md` — правила загружаются условно [S5]
- Правила без `paths` загружаются безусловно (always-on)
- Symlinks поддерживаются для sharing между проектами [S11]
- Рекурсивное обнаружение `.md` файлов, без manifest [S5]

**Важный нюанс для bmad-ralph:** текущие файлы используют `# Scope:` комментарии, но не `paths` frontmatter. Scope-комментарии — hint для LLM, но не enforcement-механизм загрузчика. Для полной реализации преимуществ нужен frontmatter.

### Секция 5: Экосистемный consensus — практика индустрии

| Инструмент | Подход к правилам | Категоризация | Фильтрация | Статус |
|---|---|---|---|---|
| Claude Code | `.claude/rules/*.md` | Да, по темам | `paths` frontmatter | Mature |
| Cursor | `.cursor/rules/*.mdc` | Да, ≤500 строк/файл | Glob frontmatter | Mature |
| Windsurf | `.windsurf/rules/` | Да, directory-based | Аналогично Cursor | Mature |
| Copilot | `.github/copilot-instructions.md` | Нет (один файл) | По file pattern | Legacy |

Copilot — единственный инструмент с монолитным подходом, и community активно запрашивает directory-based rules. Issue #13582 на GitHub Copilot (vscode-copilot-release) запрашивал аналог `.cursor/rules/`, но был закрыт как «not planned» [S10]. Practitioner прямо называет монолитный подход «LLM anti-pattern» [S9].

**L0-L6 Maturity Model** (CleverHoods, 2025) [S8]:
- L1 (Basic): один CLAUDE.md
- L3 (Structured): множество файлов по темам с cross-references
- L4 (Abstracted): path-scoped rules с conditional loading
- L5 (Maintained): L4 + active upkeep, governance

bmad-ralph находится на уровне **L3-L4**: множество тематических файлов + scope comments (но без full glob enforcement). Это продвинутый, но обоснованный уровень зрелости для проекта с 122 правилами.

---

## Риски и ограничения

| Риск | Severity | Mitigation |
|---|---|---|
| Нет прямых A/B тестов single vs multi-file | Medium | Composite evidence из 12 источников convergent |
| Модели улучшаются — context rot может стать менее острой | Low | Текущие state-of-the-art всё ещё подвержены [S4] |
| Glob-scoping может пропустить нужное правило (precision > recall) | Medium | Always-loaded ядро для критических правил |
| Maintenance cost множества файлов | Low | Индекс-файл (`go-testing-patterns.md`) решает |
| `# Scope:` comments ≠ enforcement | Medium | Рекомендация: добавить `paths` frontmatter |

---

## Рекомендации

1. **Сохранить текущую архитектуру.** 8 тематических файлов для 122 правил — оптимальный баланс. Не консолидировать в один файл

2. **Добавить `paths` frontmatter** к файлам `.claude/rules/` для реализации полноценного glob-scoped фильтрования:
   ```yaml
   ---
   paths: "**/*_test.go"
   ---
   ```

3. **Не дробить дальше.** При ~15 правилах/файл дальнейшая декомпозиция создаст overhead без выигрыша. Порог для split: > 30 правил в файле

4. **Двухуровневая архитектура:**
   - **Ядро** (always-loaded): CLAUDE.md + архитектурные правила (~30 правил)
   - **Контекстные** (glob-scoped): testing, errors, config, WSL — по matching паттерну

5. **Мониторить рост.** При достижении 200+ правил — рассмотреть distillation (см. задачу #6) или archival устаревших

6. **Индекс-файл обязателен.** `go-testing-patterns.md` как index — правильный паттерн. При добавлении новых файлов — обновлять индекс

---

## Appendix A: Evidence Table

| ID | Claim | Источники | Quality |
|---|---|---|---|
| C1 | Context rot деградирует accuracy на 20-35 пунктов | S4, S6 | A |
| C2 | U-shaped attention bias (lost in the middle) | S2 | A |
| C3 | Chunking улучшает instruction following | S1, S7, S12 | B |
| C4 | Glob-scoped loading снижает noise | S3, S5, S8 | B |
| C5 | Cursor рекомендует ≤500 строк/файл | S9 | B |
| C6 | Монолитный промпт = LLM anti-pattern | S9 | B |
| C7 | Ecosystem convergence к модульности | S3, S5, S8, S9, S10 | A |
| C8 | Описательные имена = semantic anchors | S5, S11 | B |
| C9 | Instruction-tuning частично компенсирует positional bias | S2 | A |
| C10 | 70-80% context window = safe zone | S6 | B |
| C11 | Файловая структура не видна LLM | Архитектурный факт | A |
| C12 | Overhead множества файлов < 6% | Расчёт | B |

## Appendix B: Источники

- **[S1]** AiSDR. «What is Chunking in Prompt Engineering?» https://aisdr.com/blog/what-is-chunking-in-prompt-engineering/
- **[S2]** Liu et al. «Lost in the Middle: How Language Models Use Long Contexts.» TACL 2024. https://aclanthology.org/2024.tacl-1.9/
- **[S3]** Claude Code Docs. «How Claude remembers your project.» https://code.claude.com/docs/en/memory
- **[S4]** Chroma Research. «Context Rot: How Increasing Input Tokens Impacts LLM Performance.» https://research.trychroma.com/context-rot
- **[S5]** ClaudeFast. «Claude Code Rules Directory: Modular Instructions That Scale.» https://claudefa.st/blog/guide/mechanics/rules-directory
- **[S6]** Demiliani. «Understanding LLM performance degradation: a deep dive into Context Window limits.» Nov 2025. https://demiliani.com/2025/11/02/understanding-llm-performance-degradation-a-deep-dive-into-context-window-limits/
- **[S7]** Lakera. «The Ultimate Guide to Prompt Engineering in 2026.» https://www.lakera.ai/blog/prompt-engineering-guide
- **[S8]** CleverHoods. «CLAUDE.md best practices — From Basic to Adaptive.» https://dev.to/cleverhoods/claudemd-best-practices-from-basic-to-adaptive-9lm
- **[S9]** Saplin. «Cursor-like Semantic Rules in GitHub Copilot.» https://dev.to/maximsaplin/cursor-like-semantic-rules-in-github-copilot-b56
- **[S10]** GitHub Issue #13582. «Allow defining `.github/rules` directory similar to cursor & windsurf.» https://github.com/microsoft/vscode-copilot-release/issues/13582
- **[S11]** Setec. «Modular Rules in Claude Code: Organizing Project Instructions with .claude/rules/.» https://claude-blog.setec.rs/blog/claude-code-rules-directory
- **[S12]** OpenAI. «Best practices for prompt engineering with the OpenAI API.» https://help.openai.com/en/articles/6654000-best-practices-for-prompt-engineering-with-the-openai-api
