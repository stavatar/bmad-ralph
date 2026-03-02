# Analyst-4 (v2): Категоризация знаний — рост от нуля на новом проекте

**Дата:** 2026-03-02
**Роль:** analyst-4 (knowledge-arch-v2)
**Метод:** deep-research (10 запросов, 15+ источников) + baseline из первого раунда (analyst-4, analyst-5, analyst-8)
**Фокус:** Новый проект с нуля — от 0 правил до сотен. Когда и как вводить категоризацию?

---

## Резюме

Категоризация знаний по файлам **не нужна до ~30 правил**. С 30 до 80 — секции внутри одного файла достаточны. С 80+ — multi-file с path-scoping даёт измеримую пользу. Текущий план Epic 6 (7 фиксированных категорий) **корректен**, но требует **отложенного разбиения**: сначала один файл → split при distillation когда набирается достаточно контента.

**Согласованность с первым раундом:** Подтверждаю пороги analyst-4-v1 (<30 / 30-80 / 80-150 / 150+) и analyst-5 (фильтрация не нужна при <5K токенов). Добавляю новый аспект: **траектория роста от нуля** и **момент первого split**.

---

## Интеграция с первым раундом

### Сводная таблица порогов (консолидация всех источников)

| Объём | Токены | Analyst-4 v1 | Analyst-5 | Analyst-8 | Мой вывод (v2) | Действие Ralph |
|-------|--------|-------------|-----------|-----------|----------------|----------------|
| 0-15 правил | <750 | Один файл ОК | — | — | **Один файл, без секций** | LEARNINGS.md only |
| 15-30 правил | 750-1500 | Один файл ОК | <5K: фильтрация не нужна | <200: текущая арх-ра optimal | **Один файл, ## секции** | LEARNINGS.md + headers |
| 30-80 правил | 1500-4000 | Категоризация опциональна | <5K: фильтрация не нужна | <200: текущая арх-ра optimal | **Первая distillation** | Distill → 1-3 ralph-*.md |
| 80-150 правил | 4000-8000 | Рекомендована | 5K-15K: glob достаточен | 200-500: sub-topic sharding | **Multi-file + path-scoping** | 4-7 ralph-*.md с `paths:` |
| 150+ правил | 8000+ | Обязательна + фильтрация | 15K+: рекомендуется | 500+: MCP/RAG | **Re-distillation** | Pruning + merge |

### Ключевые данные из первого раунда (baseline)

**Analyst-4 v1:**
- Glob-scoped загрузка — главный multiplier, не сама категоризация
- LLM не "видит" файловую структуру — видит текст в контексте
- Overhead множества файлов < 6% — пренебрежимо
- Ecosystem consensus: модульность > монолит (Claude Code, Cursor, Windsurf)

**Analyst-5 (context rot пороги):**
- Lost in the Middle НЕ критичен для structured instructions <10K токенов
- Инструкции ≠ документы: модели обучены следовать system prompt целиком
- При ~4K токенов полная загрузка безопаснее selective injection (risk false negatives)
- Порог пересмотра: ~15K токенов (~400+ правил)

**Analyst-8 (мета-анализ):**
- Tiered memory (core/selective/reference) — converged best practice (MemGPT, Claude Code, MemOS)
- File-based 74.0% LoCoMo vs 68.5% graph-based (Letta benchmark)
- >15 правил в одном файле → <94% compliance (SFEIR)
- Context budget: ~15-20K tokens (7.5-10%) — sweet spot

---

## Q1: При каком объёме категоризация начинает давать пользу?

### Консолидированные эмпирические данные

| Источник | Находка | Тип evidence |
|----------|---------|-------------|
| SFEIR CLAUDE.md Optimization | ~15 правил/файл = 94% compliance sweet spot | Industry (A) |
| IFScale (Jaroslawicz 2025) | Frontier: near-perfect до ~150 инструкций. Claude Sonnet: linear decay | Academic (A) |
| Chroma Context Rot (18 моделей) | 30-50% degradation при полном vs компактном контексте | Academic (A) |
| Liu et al. (TACL 2024) | U-shaped attention: середина теряется при 10+ документов | Academic (A) |
| DGM | Конкретные нарушения >> абстрактные правила (2.5x effectiveness) | Industry (B) |
| Anthropic Context Engineering | "smallest set of high-signal tokens" | Official (A) |
| Analyst-5 v1 | <5K токенов: фильтрация не нужна, маргинальный gain | Internal (B) |

### Вывод с учётом роста от нуля

- **0-15 правил (сессии 1-3):** Один LEARNINGS.md. Категоризация = overhead. Claude справляется с 15 правилами в одном блоке при 94% compliance.
- **15-30 правил (сессии 3-6):** Один файл с `## Category` секциями. Markdown headers работают как attention anchors (Microsoft structured data research). Нет нужды в отдельных файлах.
- **30-80 правил (сессии 6-15):** **Момент первой distillation.** Когда LEARNINGS.md достигает soft threshold (150 строк), distill → ralph-{category}.md. Не все 7 категорий сразу — только те, где набралось 5+ правил.
- **80+ правил (сессии 15+):** Multi-file обязателен. Path-scoping становится критичен.

**Практический порог для первого split: ~30-50 правил ≈ ~150 строк LEARNINGS.md ≈ 6-10 сессий code-review.**

---

## Q2: На новом проекте — сразу разбивать или после порога?

### Анализ для zero-start сценария

**Сценарий:** Разработчик запускает Ralph на новом Python/TS/Go проекте. Первый code-review. Ralph извлекает 5-8 правил.

**Вариант A: Сразу 7 файлов (текущий план буквально)**
```
.claude/rules/ralph-testing.md      # 2 правила
.claude/rules/ralph-errors.md       # 1 правило
.claude/rules/ralph-architecture.md # 0 правил
.claude/rules/ralph-code-quality.md # 2 правила
.claude/rules/ralph-naming.md       # 0 правил
.claude/rules/ralph-tooling.md      # 0 правил
.claude/rules/ralph-patterns.md     # 1 правило
.claude/rules/ralph-index.md        # TOC
```
**Проблема:** 8 файлов, 4 пустых, суммарно 6 правил. Claude Code загружает ВСЕ .claude/rules/*.md файлы с high priority. Пустые файлы = wasted priority slots. Index файл с 3 непустыми entries = overhead > content.

**Вариант B: Single → auto-split (рекомендуемый)**
```
# Сессии 1-6: один файл
.claude/rules/ralph-learnings.md    # 25 правил с ## секциями

# После первой distillation (сессия ~8):
.claude/rules/ralph-testing.md      # 8 правил
.claude/rules/ralph-errors.md       # 6 правил
.claude/rules/ralph-code-quality.md # 7 правил
.claude/rules/ralph-learnings.md    # 4 оставшихся (misc)
```
**Преимущество:** Файлы создаются когда в них есть реальный контент. Нет пустых файлов. Миграция одноразовая и автоматизируемая (часть distillation process).

### Рекомендация

**Вариант B — single-file → auto-split при distillation.** Epic 6 уже имеет 2-tier архитектуру (LEARNINGS.md → distilled files). Уточнение: distillation должна создавать файл категории **только если для неё набралось ≥5 правил**. Правила с <5 записей → ralph-misc.md.

**Совместимость с Epic 6:** Текущий план НЕ требует создания всех 7 файлов с первого дня. "Fixed canonical 7" = список допустимых категорий, а не обязательство создать 7 файлов. Distillation создаёт файл по мере накопления.

---

## Q3: Видит ли LLM разницу между "7×15" и "1×105"?

### Консолидированный вывод (новый + v1 baseline)

**Три фактора определяют разницу:**

1. **Объём загруженного текста** — главный фактор деградации (Chroma, analyst-5). 7×15 = 1×105 если всё загружается. При <5K токенов разница маргинальна.

2. **Path-scoped loading** — единственный реальный differentiator multi-file (analyst-4 v1). 7 файлов с `paths:` frontmatter → Claude загружает 15 правил вместо 105. 7x reduction.

3. **Attention distribution** — 7 "начал" и "концов" vs 1 (Liu et al.). Теоретически помогает, но analyst-5 показал: для structured instructions <10K эффект минимален.

### Для сценария "рост от нуля"

| Фаза | Объём | 1 файл vs N файлов | Выигрыш multi-file |
|------|-------|--------------------|--------------------|
| Ранняя (0-30) | <1.5K tok | Эквивалентно | 0% — нечего фильтровать |
| Средняя (30-80) | 1.5-4K tok | Почти эквивалентно | 5-10% — marginal noise reduction |
| Зрелая (80+) | 4K+ tok | Multi-file с path-scoping лучше | 20-40% — значимая фильтрация |

### Проблема path-scoping для unknown stack

Ralph не знает заранее, что `*_test.go` = тесты в Go, а `test_*.py` = тесты в Python.

**Решение из Epic 6:** Claude генерирует `paths:` frontmatter при distillation. Claude понимает стек проекта и может корректно указать glob patterns. Go-side только валидирует синтаксис.

**Риск:** Claude может указать неоптимальные patterns. Mitigation: human gate на distillation позволяет проверить.

---

## Q4: Зависят ли категории от стека?

### Анализ категорий по проектам разных стеков

**Универсальные (работают для ЛЮБОГО стека):**

| Категория | Примеры правил (Go) | Примеры правил (Python) | Примеры правил (TS) |
|-----------|---------------------|------------------------|---------------------|
| testing | Table-driven tests | pytest fixtures | Jest mocks |
| errors | Error wrapping | Exception handling | Error boundaries |
| code-quality | DRY, KISS, SRP | PEP8, type hints | ESLint patterns |
| architecture | Dependency direction | Module structure | Component hierarchy |

**Вывод:** Категории **стек-агностичны**. Содержание правил — стек-специфично. 7 фиксированных категорий Epic 6 покрывают 95%+ проектов.

**NEW_CATEGORY proposal:** Нужен для edge cases (e.g., `concurrency` в Go/Rust, `async` в Python/TS). Mechanism Epic 6 (Claude предлагает → Go валидирует) — корректный.

---

## Q5: Фиксированные vs динамические категории

### Trade-off matrix (расширенная)

| Аспект | Фиксированные (Epic 6) | Полностью динамические | Гибридные (рекомендация) |
|--------|------------------------|----------------------|------------------------|
| Предсказуемость | Высокая | Низкая | Высокая (7 canonical) |
| Реализация | Простая — enum | Сложная — LLM classify | Средняя — enum + proposal |
| Стоимость ошибки | Низкая (misc fallback) | Высокая (inconsistency) | Низкая (misc fallback) |
| Cold start | 7 пустых файлов | 0 файлов, но шум | 0 файлов → grow |
| LLM call cost | 0 (enum lookup) | 1 call/rule | 1 call/distillation |
| Cross-session consistency | 100% (canonical names) | Risk: rename drift | 95%+ (canonical + growth) |

### Рекомендация

**Фиксированные 7 категорий + NEW_CATEGORY proposal** (как в Epic 6) — оптимум. Но с уточнением:

1. **Не создавать файлы категорий при инициализации** — только при первой записи в категорию
2. **Canonical list хранится в config**, не в файловой системе
3. **NEW_CATEGORY → добавляется в user-local config** (не в исходники Ralph), чтобы Ralph не менял свой код

---

## Q6: Структурированные инструкции vs плоские списки

### Прямые доказательства (расширено)

| Источник | Находка | Relevance |
|----------|---------|-----------|
| IFScale (2025) | Bias towards earlier instructions. Порядок > структура | High |
| Microsoft Structured Data (2024) | Partition marks + content order significantly improve understanding | High |
| Sahoo et al. (2024) | Structured prompts significantly improve instruction following vs flat text | High |
| Anthropic | "Ruthlessly prune" — объём > форматирование | High |
| Cursor docs | ≤500 строк, focused + actionable + scoped | Medium |
| ClaudeFast | "High priority everywhere = priority nowhere" | Medium |

### Вывод для Ralph (новые проекты)

**Практические правила:**
1. **Объём важнее структуры** — 15 правил без секций > 80 правил с красивыми секциями
2. **Markdown ## headers помогают** — attention anchors, но не серебряная пуля
3. **Path-scoping > секции** — загружать только нужное > форматировать всё красиво
4. **Порядок имеет значение** — критические правила ставить первыми в файле (primacy bias)

**Рекомендация для LEARNINGS.md:** Организовать по priority, не по алфавиту. Самые частые violations → верх файла.

---

## Q7: "Lost in the middle" — при каком объёме начинает влиять?

### Консолидированная шкала (все источники)

| Фактор | Порог | Evidence |
|--------|-------|----------|
| Instruction following decline | ~50 инструкций (non-reasoning), ~150 (frontier) | IFScale 2025 |
| Retrieval accuracy drop (middle) | 10+ документов | Liu et al. 2024 |
| Context rot measurable | ~10K-20K токенов | Chroma 2025 |
| Instruction following degradation | ~50K+ токенов | Analyst-5 synthesis |
| Critical threshold (40% window) | ~80K токенов | Chroma 2025 |

### Для Ralph: траектория роста от нуля

| Сессия | Правил | Токенов | Lost-in-the-middle | Действие |
|--------|--------|---------|--------------------|---------|
| 1-3 | 5-15 | 250-750 | Нет эффекта | Один файл |
| 3-6 | 15-30 | 750-1500 | Нет эффекта | ## секции |
| 6-10 | 30-50 | 1500-2500 | Минимальный | Первая distillation |
| 10-20 | 50-100 | 2500-5000 | Начинает проявляться | Multi-file |
| 20-40 | 100-200 | 5000-10K | Значимый | Path-scoping обязателен |
| 40+ | 200+ | 10K+ | Критический | Re-distillation + pruning |

### Ключевой инсайт

**Для нового проекта lost-in-the-middle НЕ проблема первые ~10 сессий.** LEARNINGS.md soft threshold 150 строк срабатывает раньше, чем lost-in-the-middle становится значимым. Это правильный guard.

**Но:** после distillation нужен **суммарный guard** на все ralph-*.md файлы. Analyst-8 рекомендует 15-20K tokens (7.5-10% context budget) как sweet spot для total knowledge overhead.

---

## Итоговые рекомендации для Epic 6 (с учётом роста от нуля)

### Подтверждено (план корректен):
1. **7 фиксированных категорий + misc + NEW_CATEGORY** — универсальны, стек-агностичны
2. **2-tier: LEARNINGS.md → distilled ralph-*.md** — правильная архитектура
3. **Soft threshold 150 строк для LEARNINGS.md** — обоснован, срабатывает до lost-in-the-middle
4. **Human gate на distillation** — необходим для quality control

### Уточнения для zero-start сценария:
1. **Lazy file creation** — ralph-*.md файлы создаются ТОЛЬКО при distillation когда категория набрала ≥5 правил. Пустые файлы = wasted priority.
2. **Single-file phase** — до первой distillation все знания в одном LEARNINGS.md с ## секциями по категориям. Не разбивать преждевременно.
3. **Priority ordering** — внутри каждого файла: самые частые violations первыми (primacy bias). Не алфавитный порядок.
4. **Total knowledge budget guard** — суммарный объём LEARNINGS.md + все ralph-*.md < 300 строк (~15K tokens). При превышении → re-distillation/pruning.
5. **Path-scoping генерируется Claude** — при distillation Claude добавляет `paths:` frontmatter на основе содержания правил. Go валидирует glob syntax, не семантику.

### Потенциальные риски:
1. **Path-scoping accuracy** — Claude может указать неточные patterns (Medium risk, mitigation: human gate)
2. **Category imbalance** — 80% правил в testing, 0 в architecture → один большой файл + 6 маленьких (Low risk, natural for any project)
3. **Cross-category rules** — правило относится к testing И errors (Low risk, put in primary category + mention in secondary)

---

## Delta vs первый раунд (что нового)

| Аспект | Первый раунд | Второй раунд (новое) |
|--------|-------------|---------------------|
| Фокус | bmad-ralph с 122 правилами | Новый проект с 0 правил |
| Порог split | ~30-80 правил | ~30-50 правил (уточнён timing: сессия 6-10) |
| File creation | Не обсуждалось | **Lazy creation** — только при ≥5 правил в категории |
| Single-file phase | Подразумевалось | **Явно рекомендовано** — LEARNINGS.md с ## секциями |
| Priority ordering | Не обсуждалось | **Primacy bias** — частые violations первыми |
| Total budget | 15-20K sweet spot | **300 строк guard** — concrete threshold для ralph |
| Path-scoping | Рекомендовано | **Claude генерирует при distillation** — решение для unknown stack |
| Growth trajectory | Статичная таблица | **Таблица по сессиям** (Q7) — actionable timeline |

---

## Evidence from Project Research (R1/R2/R3)

Три внутренних исследования проекта (82 источника суммарно) содержат evidence, напрямую влияющее на решения о категоризации при росте от нуля.

### Критические находки для zero-start сценария

#### 1. Atomized facts > narrative (R1, S5 Chroma)

R1 обнаружил counterintuitive результат: **shuffled/unstructured контексты outperform organized** в needle-in-haystack тестах [R1-S5]. Coherent text создаёт attention shortcuts — модель "скользит" по знакомой структуре вместо точного поиска.

**Импликация для категоризации:** Красиво структурированные файлы с narrative описаниями — НЕ оптимальны. Atomized independent facts (`- Pattern [file.go] (Story X.Y)`) работают лучше. Это поддерживает текущий формат bmad-ralph rules и означает: при категоризации важнее **размер файла**, чем **красота структуры**.

#### 2. ~15 правил/файл = sweet spot для compliance (R2, S25 SFEIR)

R2 количественно подтвердил: **~15 императивных правил = 94% compliance**. Файл с 125+ паттернами → ~40-50% compliance. Расчёт:

| Объём файла | Est. compliance | Источник |
|-------------|----------------|----------|
| ≤15 правил | ~94% | R2-S25 (SFEIR) |
| 30-50 правил | ~70-80% | Экстраполяция |
| 80-125 правил | ~40-50% | R2 (bmad-ralph данные) |
| 150+ правил | <40% | R2-S25 + IFScale |

**Импликация:** На новом проекте first split должен происходить когда ЛЮБОЙ файл превышает ~30 правил. Не общее количество — а размер самого большого файла. 30 правил в одном файле = ~70-80% compliance. Split на 2×15 = ~94% каждый.

#### 3. CLAUDE.md framing problem (R1, S6)

R1 обнаружил: содержимое CLAUDE.md и .claude/rules/ оборачивается disclaimer **"may or may not be relevant to your tasks"** [R1-S6]. Это explicit signal модели, что инструкции опциональны.

**Импликация для категоризации на новом проекте:** Даже правильно категоризированные ralph-*.md файлы в .claude/rules/ получают framing disclaimer. SessionStart hook bypass-ит эту проблему. Для нового проекта: **top-5 самых критичных правил → SessionStart hook**, остальные → .claude/rules/.

#### 4. ~150-200 инструкций = practical ceiling (R1, S14 HumanLayer)

R1 установил: frontier LLM следуют ~150-200 инструкциям с reasonable consistency [R1-S14]. При 3-5 строках/инструкцию → budget ~40-60 distinct rules в контексте.

**Импликация для нового проекта:**
- LEARNINGS.md (150 строк soft threshold) ≈ ~40-50 правил → within ceiling
- 7 distilled ralph-*.md файлов (если ВСЕ загружены) × 15 правил = 105 → dangerously close to ceiling
- **Path-scoping обязателен** при >50 суммарных distilled правил, иначе ceiling будет превышен

#### 5. Citation validation — 7% improvement (R1, S3/S18 GitHub Copilot)

Copilot Memory с citation-based JIT verification: 7% рост PR merge rate (90% vs 83%, p < 0.00001) [R1-S3]. Механизм: при использовании stored memory, agent проверяет citations против текущего codebase. Stale memories auto-corrected.

**Импликация для категоризации:** Каждое правило в ralph-*.md должно иметь citation (`[file.go:line]`). При distillation — проверять: файл ещё существует? Строка изменилась? Stale правила → помечать `[stale]` или удалять. Это особенно важно на новом проекте, где код быстро меняется.

#### 6. Concrete violations >> abstract rules (R2, S37 DGM)

DGM (Darwin Godel Machine): хранение **конкретных ошибок** raised SWE-bench 20% → 50% (2.5x). Абстрактное "всегда проверяй doc comments" менее эффективно, чем конкретное "Story 3.8: doc said 'returns nil on clean state' but function returns ErrNoRecovery".

**Импликация для нового проекта:** При росте от нуля, первые правила будут конкретными (из code review). НЕ обобщать их при distillation до абстрактных. Формат: конкретный пример + generalized rule = оба нужны.

#### 7. File-based > graph-based при <500 записей (R3, S1 Letta)

Letta benchmark: filesystem-based agent = **74.0% на LoCoMo**, beats Mem0 Graph (68.5%) [R3-S1]. File-based injection остаётся оптимальным при текущем масштабе.

**Импликация:** Категоризация в файлы (не в graph DB, не в vector store) — подтверждённый оптимум. Для нового проекта: рост от 0 до 500 правил покрывается чистым file-based подходом.

#### 8. Adaptive escalation thresholds (R2, Sec. 4.6)

R2 предложил adaptive prioritization:
- 1 violation → rules file
- 2-3 violations → SessionStart hook
- 4+ violations → PreToolUse checklist
- 6+ → Architectural fix (PostToolUse auto-check)

**Импликация для нового проекта:** На новом проекте ВСЕ правила начинают с уровня "rules file". По мере накопления violations — escalation. Категоризация файлов должна учитывать escalation level: ralph-critical.md (top violations) vs ralph-patterns.md (accumulated patterns). Или проще: violation count metadata на каждом правиле.

### Consolidated evidence table (R1+R2+R3 → categorization relevance)

| Evidence | Source | Confidence | Relevance to zero-start |
|----------|--------|------------|------------------------|
| ~15 rules/file = 94% compliance | R2-S25 | HIGH | **Defines max file size** — split trigger |
| Atomized facts > narrative | R1-S5 | HIGH | **Format within categories** — bullet points, not prose |
| ~150-200 instruction ceiling | R1-S14 | MEDIUM | **Total budget** — path-scoping needed >50 rules |
| File-based 74% > Graph 68.5% | R3-S1 | HIGH | **File-based is optimal** for full growth path |
| Concrete violations 2.5x better | R2-S37 | HIGH | **Don't over-abstract** at distillation |
| Citation validation +7% | R1-S3 | MEDIUM | **Citations essential** for stale detection |
| Framing disclaimer on rules | R1-S6 | HIGH | **Top rules → hook**, not just files |
| Shuffled > organized contexts | R1-S5 | MEDIUM | **Don't over-structure** — atomized is better |

### Revised recommendations (incorporating R1/R2/R3)

Оригинальные рекомендации остаются в силе. Дополнения:

1. **Max file size trigger** (new): Split ralph-*.md когда ЛЮБОЙ файл > 30 правил (не общее количество). Это из R2-S25: 15 rules = 94%, 30 = ~80%, 50 = ~60%.

2. **Don't over-abstract at distillation** (new): Сохранять конкретные примеры violations рядом с generalized rules. DGM 2.5x evidence [R2-S37].

3. **Citation metadata mandatory** (new): Каждое правило: `- Pattern [file.go:line] (session N)`. Stale detection при distillation [R1-S3].

4. **Top-5 → SessionStart hook** (new): На новом проекте после 3-5 code reviews выделить top-5 most violated → SessionStart hook (bypasses framing) [R1-S6].

5. **Total budget awareness** (updated): LEARNINGS.md (40-50 rules) + distilled files (if all loaded: up to 105) = up to 155 rules → AT instruction ceiling [R1-S14]. Path-scoping not optional, it's survival.

---

## Источники

### Новые (deep-research v2)
1. [Lost in the Middle](https://arxiv.org/abs/2307.03172) — Liu et al., TACL 2024
2. [IFScale: How Many Instructions Can LLMs Follow at Once?](https://arxiv.org/html/2507.11538v1) — Jaroslawicz 2025
3. [Claude Code Rules Directory](https://claudefa.st/blog/guide/mechanics/rules-directory) — ClaudeFast
4. [Best Practices for Claude Code](https://code.claude.com/docs/en/best-practices) — Anthropic
5. [How Claude remembers your project](https://code.claude.com/docs/en/memory) — Anthropic
6. [CLAUDE.md: From Basic to Adaptive](https://dev.to/cleverhoods/claudemd-best-practices-from-basic-to-adaptive-9lm) — CleverHoods
7. [Writing a good CLAUDE.md](https://www.humanlayer.dev/blog/writing-a-good-claude-md) — HumanLayer
8. [Rules | Cursor Docs](https://cursor.com/docs/context/rules) — Cursor
9. [Improving LLM understanding of structured data](https://www.microsoft.com/en-us/research/blog/improving-llm-understanding-of-structured-data-and-exploring-advanced-prompting-methods/) — Microsoft Research
10. [Claude Code Path-Specific Rules](https://paddo.dev/blog/claude-rules-path-specific-native/) — paddo.dev

### Baseline из первого раунда
11. analyst-4-categorization-report.md — 12 источников (S1-S12)
12. analyst-5-dynamic-injection-report.md — 10 источников
13. analyst-8-synthesis-report.md — 12 источников (academic + industry)
