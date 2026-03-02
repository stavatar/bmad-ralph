# Epic 6 Extraction Pipeline — Критический обзор (Stories 6.1, 6.3, 6.4)

**Совместный consensus document**
**Пара 1:** analyst-extraction + architect-extraction
**Дата:** 2026-02-28
**Scope:** Stories 6.1 (FileKnowledgeWriter), 6.3 (Resume-Extraction Knowledge), 6.4 (Review Knowledge)

---

## Executive Summary

Extraction pipeline (Stories 6.1, 6.3, 6.4) содержит **1 CRITICAL, 2 HIGH и 4 MEDIUM** архитектурных проблем. Ключевая: фундаментальное противоречие между Go quality gates (Story 6.1) и Claude direct writes (Stories 6.3/6.4) — две взаимоисключающие модели записи в один файл.

**Главная рекомендация:** Pending-file pattern (`.ralph/pending-lessons.md`) — единое архитектурное решение, устраняющее 4 проблемы одновременно (write path, [needs-formatting] pollution, mutation asymmetry, thread safety).

Обзор основан на: epic-6 (859 строк, 59 AC), 3 исследовательских отчёта (R1: 20 источников, R2: 40 источников, R3: 22 источника), текущий scaffold (runner/knowledge.go, runner/runner.go, config/config.go, config/prompt.go), промпты (execute.md, review.md), web research 2025-2026.

---

## Issue 1 (CRITICAL): Двойной путь записи — Go quality gates vs Claude direct writes

### Описание проблемы

В дизайне фундаментальное противоречие между двумя моделями записи:

**Модель A (Story 6.1 AC):** Go-код через `FileKnowledgeWriter.WriteLessons()` с 6 quality gates (G1-G6). Go валидирует формат, citation, dedup, budget, cap, min content. Go записывает файл.

**Модель B (Stories 6.3 + 6.4 Technical Notes):**
- Story 6.3: *"Claude inside resume-extraction session writes to LEARNINGS.md directly via file tools"*
- Story 6.4: *"Claude inside review session does the actual writing — not Ralph's Go code"*

Эти модели **взаимоисключающие**. Если Claude пишет напрямую через file tools, quality gates в Go-коде **никогда не вызываются**. G1-G6 становятся мёртвым кодом. Mutex для thread safety бесполезен — Claude работает в отдельном процессе.

### Импакт

- 6 quality gates — самая существенная часть extraction pipeline. Без них LEARNINGS.md деградирует
- Research R1 [S5]: context rot 30-50% при неконтролируемом росте контекста
- Research DGM [S37]: качество знаний напрямую влияет на effectiveness (20%→50%)
- Dangerous feedback loop: плохие знания → больше ошибок → ещё больше плохих знаний

### Рассмотренные варианты

| Вариант | Описание | Плюсы | Минусы |
|---------|----------|-------|--------|
| A: Go-only write | Claude выводит в session output, Go парсит и вызывает WriteLessons | Gates гарантированы; единый write path | Нужен парсинг output; Claude может не выдать формат |
| B: Claude writes + Go post-validates | Claude пишет напрямую, Go читает и валидирует после | Claude имеет контекст; проще prompt | Две точки записи; race conditions; сложная diff логика |
| **C: Pending-file** | Claude → `.ralph/pending-lessons.md`, Go → gates → LEARNINGS.md | Gates работают; Claude пишет в контексте; нет race conditions; тестируемо | Дополнительный transient файл |

### Консенсус: Вариант C — Pending-file pattern

Claude в resume/review сессиях пишет в `.ralph/pending-lessons.md` (transient файл, внутри .ralph чтобы не засорять project root). После сессии Go-код:
1. Читает `.ralph/pending-lessons.md`
2. Парсит в `[]LessonEntry`
3. Применяет quality gates (G1-G6) к каждому entry
4. Валидные entries → append к LEARNINGS.md
5. Невалидные entries → log + reject
6. Удаляет `.ralph/pending-lessons.md`

**Бонус:** Pending-file pattern решает сразу 4 проблемы:
- Issue 1 (write path): единый Go write path с gates
- Issue 4 ([needs-formatting]): reject вместо pollution
- Issue 6 (mutation asymmetry): review пишет в transient file, не в LEARNINGS.md
- Issue 9 (thread safety): Go контролирует единственный write path, mutex не нужен

### Требуемые изменения в AC

- Stories 6.3/6.4 Technical Notes: заменить "Claude writes to LEARNINGS.md directly" на "Claude writes to `.ralph/pending-lessons.md`"
- Story 6.1: добавить `ProcessPendingLessons(ctx, pendingPath) error` — читает pending, применяет gates, append к LEARNINGS.md
- Story 6.3/6.4 prompts: инструкция "write insights to `.ralph/pending-lessons.md` in format: ..."
- Runner.Execute: после review/resume-extraction вызывать ProcessPendingLessons

---

## Issue 2 (HIGH): Semantic dedup по строковому префиксу — хрупкий механизм

### Описание проблемы

Dedup через `strings.ToLower + strings.TrimSpace` на `## category: topic` prefix — pure string matching, не семантическая дедупликация. Пропускает очевидные дубликаты:

- `## testing: assertion-quality` vs `## testing: assertion quality` (дефис vs пробел)
- `## error-handling: wrap errors` vs `## errors: consistent wrapping` (разные категории, один смысл)

Research: NVIDIA SemDedup и MinishLab semhash (2025-2026) показывают, что string-based dedup пропускает 40-60% семантических дубликатов. При 200-line budget каждый дубликат — потеря 2-4 строк.

### Консенсус: Normalized prefix match (v1) + стандартизированные категории в prompt

**Go-level (v1):**
1. Normalize: ToLower, TrimSpace, replace all `-_` → space, collapse multiple spaces
2. Prefix match на нормализованной строке
3. Если совпало — merge facts under existing heading

**Prompt-level (defense in depth):**
Extraction prompt содержит стандартизированный список категорий: "Use EXACTLY these category names: testing, errors, config, cli, architecture, performance, documentation". Снижает variance на уровне генерации.

**Planned evolution:** При >100 entries или >5% дупликатов по metrics — добавить fuzzy matching (Levenshtein или Jaccard). Не сейчас — YAGNI при 40-60 entries.

### Требуемые изменения в AC

- Story 6.1 AC (semantic dedup): нормализация включает замену `-_` на пробел + collapse multiple spaces (помимо ToLower + TrimSpace)
- Stories 6.3/6.4 prompts: стандартизированный список категорий

---

## Issue 3 (HIGH): Citation format — универсальность под вопросом

### Описание проблемы

Формат `[source, file:line]` предполагает каждый lesson привязан к конкретному файлу. Проблемные сценарии для универсального CLI:

- **Non-file lessons:** "Always run tests before commit" — к какому файлу привязать?
- **Cross-project patterns:** "Error wrapping is important" — universal, нет файла
- **Generated code:** Line numbers бесполезны — файл регенерируется
- **Monorepo:** Длинные пути раздувают заголовки, waste budget
- **Windows paths:** `C:\path` конфликтует с `:` разделителем в `file:line`

JIT citation validation (Story 6.2) делает `os.Stat(file)` для каждой citation. Entries без файловой привязки будут **всегда** фильтроваться как stale.

### Консенсус: Citation optional, category required

**Формат:** `## category: topic [optional-citation]`

- Citation `[source, file:line]` — необязательна
- Если citation есть — JIT validates (os.Stat)
- Если citation отсутствует — entry всегда валиден при JIT check
- Category — обязательна (для dedup, routing, distillation)

**Quality gate G2:** проверяет формат citation **если она присутствует**, не отклоняет при отсутствии. Примеры валидных заголовков:

```
## testing: assertion-quality [review, tests/test_auth.py:42]   ← citation present
## errors: always wrap with context [review]                     ← source-only
## architecture: dependency direction top-down                   ← no citation (universal)
```

### Требуемые изменения в AC

- Story 6.1 AC: G2 "Citation present" → "Citation valid IF present" (not required)
- Story 6.2 AC: ValidateLearnings — entries без citation = always valid
- Stories 6.3/6.4 prompts: "Citation optional — include [source, file:line] when lesson is file-specific"

---

## Issue 4 (MEDIUM): [needs-formatting] tag — pollution вместо качества

### Описание проблемы

Невалидные entries сохраняются в LEARNINGS.md с тегом `[needs-formatting]` и "исправляются при distillation":

1. **Pollution:** LEARNINGS.md содержит неструктурированный контент до distillation
2. **Accumulation:** если circuit breaker OPEN — [needs-formatting] entries накапливаются бесконечно
3. **Wasted budget:** каждый [needs-formatting] entry = 2-4 строки бесполезного контекста
4. **Complexity:** ValidateDistillation criterion #7 усложняется

### Консенсус: Reject + log (при pending-file pattern)

При pending-file pattern (Issue 1) отпадает необходимость в [needs-formatting]:

1. Go читает `.ralph/pending-lessons.md`
2. Каждый entry проходит quality gates
3. Валидные → LEARNINGS.md (всегда чистый файл)
4. Невалидные → log warning с preview (первые 50 chars) → удаляются
5. Невалидные entries не теряются — они в session output, git history, логах

**LEARNINGS.md = всегда чистый файл.** Нет [needs-formatting] тегов. Нет pollution. Нет accumulation.

### Требуемые изменения в AC

- Story 6.1: убрать AC "[needs-formatting] tag" — заменить на "invalid entries logged and rejected"
- Story 6.1: убрать [needs-formatting] из entry format
- Story 6.5: убрать ValidateDistillation criterion #7 ([needs-formatting] handling)
- Story 6.8: убрать [needs-formatting] test scenario → заменить на rejected entry test

---

## Issue 5 (MEDIUM): Resume-extraction через --resume несовместим с knowledge instructions

### Описание проблемы

Resume-extraction использует `session.Options{Resume: sessionID}` → `claude --resume <id>`. В resumed session:

1. Claude продолжает предыдущий контекст — нельзя inject произвольный prompt
2. Текущий `ResumeExtraction` (runner.go:188) не передаёт Prompt — только Resume ID
3. Claude в resumed session не знает формат LEARNINGS.md / pending-lessons.md

Если Claude не знает формат — все записи невалидны → все отклоняются → knowledge extraction бесполезна для resume.

### Консенсус: Отдельная extraction session после resume

Поток:
1. Resume session завершается (extract WIP state, как сейчас)
2. Go код получает session output
3. **Новый шаг:** Go запускает отдельную `claude -p` session с extraction prompt:
   - Input: session output / контекст предыдущей сессии
   - Prompt: "Extract failure insights. Write to `.ralph/pending-lessons.md` in format: ..."
   - Output: pending-lessons.md (обрабатывается Go quality gates)

Token cost: ~2-5K tokens (minor, ~0.1× one execute session). Reliability: full control over prompt.

### Требуемые изменения в AC

- Story 6.3: добавить AC "Separate extraction session runs after resume completes"
- Story 6.3: убрать "Claude inside resume-extraction session writes to LEARNINGS.md directly"
- Story 6.3: добавить extraction prompt template (можно embed как `runner/prompts/extract-resume.md`)

---

## Issue 6 (MEDIUM): Снятие инварианта MUST NOT write LEARNINGS.md — Mutation Asymmetry risk

### Описание проблемы

Review prompt (review.md:120-121) содержит инвариант:
```
- **MUST NOT write LEARNINGS.md or CLAUDE.md**: knowledge extraction is deferred to Epic 6
```

Story 6.4 удаляет этот инвариант без обновления Mutation Asymmetry документации. Mutation Asymmetry — архитектурный принцип проекта:
- Execute sessions: modify source code, commit
- Review sessions: ONLY modify sprint-tasks.md and review-findings.md

Расширение scope review session требует явной документации.

### Консенсус: Явное обновление Mutation Asymmetry

При pending-file pattern это смягчается (review пишет в `.ralph/pending-lessons.md`, transient file), но Mutation Asymmetry всё равно нужно обновить:

**review.md Prompt Invariants (новая версия):**
```
- **MUST NOT modify source code** (FR17)
- **MAY write `.ralph/pending-lessons.md`**: knowledge extraction on confirmed findings
- **Mutation Scope**: sprint-tasks.md, review-findings.md, .ralph/pending-lessons.md — NOTHING ELSE
```

**execute.md Mutation Asymmetry (дополнение):**
```
- Execute sessions MAY write .ralph/pending-lessons.md (from resume-extraction insights)
```

### Требуемые изменения в AC

- Story 6.4 AC: явно указать обновление Prompt Invariants в review.md
- Story 6.4 AC: добавить "update Mutation Asymmetry documentation"
- Story 6.3 AC: аналогичное обновление для execute.md (resume knowledge)

---

## Issue 7 (MEDIUM): LessonsData struct слишком плоский

### Описание проблемы

AC определяет `LessonsData{Source string, Content string}`. Content — одна строка. Проблемы:

1. Как передать множественные entries? Content = pre-formatted markdown с `## ` разделителями — хрупко
2. Если 3 из 5 entries валидны — нельзя сохранить 3 и отклонить 2 при одном Content string
3. Quality gates должны работать per-entry, не per-batch

### Консенсус: Structured entries вместо Content string

```go
type LessonEntry struct {
    Category string // required: "testing", "errors", etc.
    Topic    string // required: "assertion-quality"
    Content  string // required: atomized fact, ≥20 chars
    Citation string // optional: "file:line" or empty
}

type LessonsData struct {
    Source  string        // "review", "resume"
    Entries []LessonEntry // each entry passes gates independently
}
```

При pending-file pattern: Go парсит `.ralph/pending-lessons.md` → создаёт `[]LessonEntry` → каждый entry проходит gates отдельно. 3 valid + 2 invalid = 3 written + 2 logged.

### Требуемые изменения в AC

- Story 6.1 AC: `LessonsData` struct — `Entries []LessonEntry` вместо `Content string`
- Story 6.1 AC: add `LessonEntry` struct definition
- Story 6.1 AC: quality gates apply per entry, not per batch

---

## Issue 8 (LOW): Arbitrary numbers G5=5 и G6=10

### Описание проблемы

- G5: max 5 entries per WriteLessons — arbitrary, no research backing
- G6: entry content ≥ 10 chars — too short. "errors.As" = 9 chars, но это valid topic (не lesson content)

### Консенсус: Hardcoded const, G6 увеличить до 20

```go
const maxEntriesPerWrite = 5   // G5: prevents over-extraction
const minContentLength   = 20  // G6: rejects trivial entries
```

Named constants в `knowledge.go`. Не config fields — нет реальных пользователей для кастомизации. TODO comment для будущей конфигурирования.

### Требуемые изменения в AC

- Story 6.1 AC: G6 "≥ 10 chars" → "≥ 20 chars"
- Story 6.1 Technical Notes: G5 и G6 = named constants, не magic numbers

---

## Issue 9 (LOW): Thread safety — unnecessary mutex

### Описание проблемы

AC требует "simple mutex" для thread-safe writes. Но:
- Runner.Execute — sequential (один outer loop)
- Нет goroutines вызывающих WriteLessons параллельно
- При pending-file pattern Go контролирует единственный write path

### Консенсус: Убрать mutex, добавить когда реально нужен

YAGNI. Текущая архитектура strictly sequential. Pending-file pattern усиливает это — единственный writer. Defer thread safety до Growth phase.

CLAUDE.md: "Avoid over-engineering. Only make changes that are directly requested or clearly necessary."

### Требуемые изменения в AC

- Story 6.1 AC: убрать "Thread-safe writes" scenario или переформулировать как "Sequential writes (mutex deferred to Growth phase)"

---

## Issue 10 (LOW): BudgetCheck semantics — G4 не gate

### Описание проблемы

G4 описан как "Budget check: total lines < hard limit" — звучит как rejection. Но "OverBudget is informational only (no forced action — file stays as-is)". Противоречие.

### Консенсус: G4 = soft warning, переименовать

G4 — не gate (не блокирует write), а warning. При OverBudget: write всё равно происходит, BudgetStatus.OverBudget=true. Согласуется с "no forced truncation" и "300+ lines = 3-4% context, linear decay".

### Требуемые изменения в AC

- Story 6.1 AC: G4 "Budget check" → "Budget warning: log if total lines ≥ hard limit"
- Story 6.1 Technical Notes: clarify G4 does NOT reject writes

---

## Issue 11 (LOW): Reverse read ordering — underspecified

### Описание проблемы

"Append-only write, reverse read (newest-first at injection)" — не уточнено что reverse-ить: строки, секции, блоки.

### Консенсус: Специфицировать section-level reverse

"Split by `\n## ` boundaries, reverse section order, rejoin with `\n## ` separator." Каждая секция = один entry с header + body. Reverse = newest sections first at injection point.

### Требуемые изменения в AC

- Story 6.2 AC / Technical Notes: уточнить reverse algorithm

---

## Issue 12 (LOW): Entry prioritization — deferred

### Описание проблемы

Все entries в LEARNINGS.md равноценны. DGM research: concrete violations 2.5x эффективнее абстрактных правил. `VIOLATION:` marker — текстовый, без программной логики. `[freq:N]` появляется только при distillation (6.5).

### Консенсус: Deferred to Stories 6.2/6.5

Не блокер для extraction pipeline. Приоритизация реализуется в:
- Story 6.5: `[freq:N]` при distillation
- Story 6.2: injection ordering (newest-first = implicit recency prioritization)

Пометка: рассмотреть explicit priority weight при Growth phase.

---

## Сводная таблица

| # | Severity | Issue | Consensus Decision | Stories Affected |
|---|----------|-------|--------------------|--------------------|
| 1 | **CRITICAL** | Write path confusion | **Pending-file pattern** (.ralph/pending-lessons.md) | 6.1, 6.3, 6.4, 6.5, 6.8 |
| 2 | **HIGH** | Weak semantic dedup | Normalized prefix match + standardized categories | 6.1, 6.3, 6.4 |
| 3 | **HIGH** | Citation universality | Citation optional, category required | 6.1, 6.2, 6.3, 6.4 |
| 4 | MEDIUM | [needs-formatting] pollution | Reject+log (pending-file eliminates need) | 6.1, 6.5, 6.8 |
| 5 | MEDIUM | Resume mode incompatible | Separate extraction session after resume | 6.3 |
| 6 | MEDIUM | Mutation Asymmetry | Explicit Prompt Invariants update | 6.3, 6.4 |
| 7 | MEDIUM | LessonsData too flat | `Entries []LessonEntry` structured API | 6.1 |
| 8 | LOW | Arbitrary G5/G6 | Hardcoded const, G6: 10→20 | 6.1 |
| 9 | LOW | Thread safety YAGNI | Remove mutex, defer to Growth | 6.1 |
| 10 | LOW | BudgetCheck semantics | G4 = soft warning, not gate | 6.1 |
| 11 | LOW | Reverse read underspecified | Section-level reverse | 6.2 |
| 12 | LOW | Entry prioritization | Deferred to 6.2/6.5 | — |

---

## Архитектурная диаграмма: Pending-file pattern (рекомендуемый поток)

```
Claude Session (review/resume)
    │
    ▼
.ralph/pending-lessons.md   ← Claude writes here (transient)
    │
    ▼
Go: ProcessPendingLessons()
    │
    ├── Parse → []LessonEntry
    │
    ├── For each entry:
    │   ├── G1: Format check (category: topic header)
    │   ├── G2: Citation valid IF present
    │   ├── G3: Not duplicate (normalized prefix match)
    │   ├── G4: Budget warning (log if ≥ limit)
    │   ├── G5: Entry cap (≤5 per call)
    │   └── G6: Min content (≥20 chars)
    │
    ├── Valid entries → append to LEARNINGS.md
    ├── Invalid entries → log warning + reject
    │
    └── Delete .ralph/pending-lessons.md
```

---

## Research backing

| Решение | Обоснование | Источник |
|---------|-------------|---------|
| Quality gates on write | Context rot 30-50% при неконтролируемом росте | R1 [S5] Chroma |
| Pending-file pattern | Separation of concerns, atomic processing | DGM [S37], QualityFlow [S34] |
| Citation optional | Universal CLI ≠ file-specific knowledge | GitHub Copilot [S3, S18] — citations enhance but not required |
| Standardized categories | ~15 rules = 94% compliance при focused sets | R2 [S25] SFEIR |
| Reject vs tag | Clean knowledge base > zero knowledge loss illusion | R1 [S5]: wasted tokens = context rot |
| Separate extraction session | Full prompt control > resumed session instructions | Anthropic [S1]: explicit harnesses for long-running agents |
| Named constants | Code readability, easy to find and modify | Project convention (CLAUDE.md) |

---

*Документ подготовлен совместно analyst-extraction и architect-extraction на основе независимых анализов с последующим обсуждением разногласий. Все 12 пунктов согласованы обеими сторонами.*
