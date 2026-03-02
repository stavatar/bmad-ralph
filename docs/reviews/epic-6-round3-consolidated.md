# Epic 6: Раунд 3 — Консолидированный отчёт (8+1 агентов)

**Дата:** 2026-03-02
**Координатор:** Claude (team lead epic6-review-r3)
**Участники:** 9 агентов (4 пары аналитик+архитектор + 1 воскресший analyst-2)
**Scope:** Epic 6 v5 — Knowledge Management & Polish (11 stories, 86 AC)
**Метод:** Adversarial review + deep research (80+ web citations, 6 research reports)

---

## Методология

4 независимые пары провели adversarial review Epic 6 v5, каждая со своим фокусом:

| Пара | Фокус | Аналитик | Архитектор |
|------|-------|----------|------------|
| 1 | Write path & extraction (6.1-6.4) | analyst-1 | architect-1 |
| 2 | Distillation & budget (6.5a/b/c, 6.6) | analyst-2 + analyst-2-respawn | architect-2 |
| 3 | Injection, Serena, integration (6.2, 6.7, 6.8) | analyst-3 | architect-3 |
| 4 | E2E algorithm + competitive analysis | analyst-4 | architect-4 |

Каждый агент: чтение эпика + decision log + предыдущих ревью + исходного кода + deep research.
Raw findings: ~55. После дедупликации: **30 уникальных** (2 CRITICAL, 9 HIGH, 15 MEDIUM, 4 LOW).

---

## CRITICAL (2)

### CR1. Multi-file distillation write НЕ АТОМАРНА — нет intent/checkpoint механизма

**Нашёл:** architect-2
**Stories:** 6.5b, 6.5c

**Проблема:** Distillation записывает LEARNINGS.md + N ralph-*.md + ralph-index.md + distill-state.json. Все записи последовательные (os.WriteFile). Crash между файлами → неконсистентное состояние:

```
LEARNINGS.md     — уже перезаписан (compressed)
ralph-testing.md — уже обновлён
ralph-errors.md  — уже обновлён
ralph-config.md  — НЕ записан (old content или отсутствует)
distill-state.json — НЕ обновлён (LastDistillTask = old value)
```

Текущая защита (.bak + startup recovery) не может отличить прерванную дистилляцию от нормальной — .bak файлы ВСЕГДА существуют после успешной дистилляции.

**Research:** Dan Luu "Files are hard" — rename() атомарна per-file, но группа rename — нет. На WSL/NTFS rename может не быть атомарной даже per-file (WSL#5087).

**Рекомендация:** Intent file `.ralph/distill-intent.json`:
- Phase 1: Backup → write .pending файлы → write intent.json
- Phase 2: Rename .pending → target → update state → delete intent.json
- Recovery: intent.json существует → завершить rename или откатить из .bak

**Effort:** MEDIUM (30-50 строк Go). Добавить AC в Story 6.5c.

---

### CR2. ValidateDistillation criterion #3 "last 20% preserved" — фундаментально сломан

**Нашли:** analyst-2 (CRITICAL), analyst-2-respawn (MEDIUM)
**Stories:** 6.5c

**Проблема:**
1. **Не масштабируется:** 20% от 50 записей = 10 (слишком много anchor). 20% от 300 = 60 (невозможно сжать до <=100 при сохранении 60).
2. **Конфликт с compression target:** "compress to <=100 lines" + "preserve 20% unchanged" = если 200 строк и 40 записей, 8 записей = ~40-60 строк preserved. Остаётся 40-60 строк на 32 записи — нереально.
3. **Позиция ≠ рецентность:** Append-only допускает merge-append при G3 dedup, нарушая строгий порядок.

**Research:** Model collapse mitigation (ICLR 2025) рекомендует anchor set, но для iterative retraining — не для one-shot text compression.

**Рекомендация:** Убрать criterion #3 целиком. Citation preservation (#4, >=80%) + ANCHOR маркер (L4) = достаточная защита.

---

## HIGH (9)

### H1. Snapshot-diff: line-count guard недостаточен

**Нашли:** analyst-1, architect-1, analyst-4, architect-4 (4 агента — КОНСЕНСУС)
**Stories:** 6.1

**Проблема:** Не ловит: in-place replacement (строки те же, контент другой), delete+append (net zero), reorder.

**4 предложенных решения:**

| Агент | Решение | Сложность |
|-------|---------|-----------|
| analyst-1 | Append-only marker (`<!-- RALPH_MARKER -->`) | 5 строк |
| architect-1 | Content hash guard (sha256) | 1 строка |
| analyst-4 | Header-based matching (`## category: topic`) | 10 строк |
| architect-4 | Prefix-match guard (первые N строк snapshot = current) | 5-10 строк |

**Рекомендация:** Content hash — самый простой и универсальный. `sha256(snapshot) != sha256(current)` при одинаковом line count → full revalidation.

---

### H2. Inject ALL без safety mechanism — prompt overflow

**Нашли:** analyst-3, architect-3, architect-4 (3 агента — КОНСЕНСУС)
**Stories:** 6.2

**Проблема:** v5 убрал "last 20%" и injection CB. Единственная защита — stderr warning. Если distillation fails/skipped → LEARNINGS.md растёт бесконтрольно.

**Research:** SFEIR: ~15 правил = 94% compliance. JetBrains NeurIPS 2025: observation masking лучше full injection (+2.6% solve, -52% cost). "Lost in the Middle": >30% performance drop.

**Рекомендация:** Graceful degradation: при >3× budget → inject ONLY ralph-*.md, skip raw LEARNINGS.md. Или injection cap = learnings_budget (200 строк), newest-first.

---

### H3. buildKnowledgeReplacements: 3 вызова без кеша, медленно на WSL

**Нашли:** architect-1, architect-2, analyst-3, architect-3 (4 агента — КОНСЕНСУС)
**Stories:** 6.2

**Проблема:** 3 вызова per task × N os.Stat (5-10ms each на NTFS) = до 1.5 сек overhead. Shared function скрывает что review НЕ должен получать self-review (HasLearnings=false).

**Рекомендация:** Cache per task + per-site TemplateData bools (review: HasLearnings=false).

---

### H4. [needs-formatting] entries загрязняют prompt

**Нашли:** analyst-1, architect-1, analyst-4, architect-4 (4 агента — КОНСЕНСУС)
**Stories:** 6.1, 6.2, 6.5b

**Проблема:** Невалидные entries инъектируются в каждый промпт. Wasted tokens + потенциальный feedback loop.

**Позиции:**
- analyst-4: reject + log (индустрия не использует tagging)
- architect-4, analyst-1, architect-1: оставить, но фильтровать при injection

**Рекомендация:** Entries с [needs-formatting] НЕ инжектируются в prompt, но остаются в файле для дистилляции. Компромисс: знания не теряются, context не загрязняется.

---

### H5. ralph-index.md — redundant

**Нашли:** architect-3, architect-4 (2 агента)
**Stories:** 6.5b

**Проблема:** Claude Code АВТОМАТИЧЕСКИ загружает все `.claude/rules/*.md`. Index = бесполезные 100-200 tokens + maintenance burden. Ни один конкурент не имеет index файлов.

**Рекомендация:** Убрать ralph-index.md из AC.

---

### H6. Backup rotation: copy, не rename

**Нашёл:** architect-2
**Stories:** 6.5b

**Проблема:** `rename(file → file.bak)` при crash → нет ни file, ни file.bak.

**Рекомендация:** `copy(file → file.bak)` + `atomicWriteFile(file, newData)` (write-temp-rename).

---

### H7. distill-state.json: partial write при crash

**Нашёл:** architect-2
**Stories:** 6.5c

**Проблема:** `os.WriteFile` может оставить пустой/truncated JSON. DistillState = единственный persistent state.

**Рекомендация:** `atomicWriteFile()` (write-temp-rename) + cascade read: primary → .bak → default.

---

### H8. CB threshold не различает типы failures

**Нашли:** analyst-2, analyst-2-respawn
**Stories:** 6.5a

**Проблема:** "3 consecutive failures" одинаково для всех. Но validation_reject = deterministic → 1 достаточно. Timeout = transient → нужно больше.

**Research:** Portkey.ai, Google Cloud SRE: "circuit breakers with failure rate windows, not consecutive counts."

**Рекомендация:** Per-type thresholds: validation=1 → OPEN, timeout=2, bad_format=1+retry, I/O=1.

---

### H9. MonotonicTaskCounter: "clean review" как единица — stuck при persistent failures

**Нашёл:** analyst-2
**Stories:** 6.5a

**Проблема:** Если все задачи fail review → counter НЕ инкрементируется → distillation НИКОГДА не триггерится → LEARNINGS.md растёт (review sessions тоже пишут знания).

**Рекомендация:** Size-based fallback trigger: lines > 2× budget → force distill check, независимо от counter.

---

## MEDIUM (15)

### M1. session.go `else if` блокирует --resume + -p совместимость

**Нашли:** analyst-1, architect-1
**Stories:** 6.3

Resume и Prompt взаимоисключающие в коде. Story 6.3 требует совместимости. `--resume -p` поведение НЕ документировано Claude CLI.

**Рекомендация:** Два независимых `if` или отдельная extraction session.

---

### M2. review.md: инвариант удаляется без замены

**Нашли:** analyst-1, architect-1
**Stories:** 6.4

Story 6.4 удаляет "MUST NOT write LEARNINGS.md" без чёткого replacement.

**Рекомендация:** MAY append LEARNINGS.md + MUST NOT write CLAUDE.md + explicit Mutation Scope list.

---

### M3. ValidateNewLessons: кто парсит raw diff в []LessonEntry?

**Нашёл:** architect-1
**Stories:** 6.1

Snapshot-diff даёт raw текст, ValidateNewLessons ожидает structured entries. Парсинг не специфицирован.

**Рекомендация:** Explicit `ParseNewEntries(rawDiff)` function с тестами.

---

### M4. Distillation prompt не получает ralph-*.md как anchor

**Нашёл:** analyst-2 (research)
**Stories:** 6.5b

Model Collapse research (ICLR 2025): "replacement causes collapse; accumulation prevents it." Prompt получает только LEARNINGS.md — нет anchor к existing distilled rules.

**Рекомендация:** Include existing ralph-*.md в distillation prompt как "teacher context".

---

### M5. BEGIN/END markers: contamination + missing END

**Нашли:** analyst-2, analyst-2-respawn
**Stories:** 6.5b

Если entry content содержит "END_DISTILLED_OUTPUT" → parser обрежет. Timeout → END отсутствует.

**Рекомендация:** Missing END → bad_format (free retry). Ignore markers inside code blocks.

---

### M6. ConsecutiveFailures записывается ПОСЛЕ попытки

**Нашёл:** architect-2
**Stories:** 6.5a

Crash между failure и state update → failure "забыта" → CB может не открыться.

**Рекомендация:** Increment BEFORE attempt, reset to 0 on success.

---

### M7. Crash recovery не отличает interrupted от normal

**Нашёл:** architect-2
**Stories:** 6.5c

.bak файлы ВСЕГДА существуют. Наличие .bak ≠ прерванная дистилляция.

**Рекомендация:** Решается через intent file (CR1).

---

### M8. DistillState — God Object (4 домена в одном JSON)

**Нашёл:** architect-2
**Stories:** 6.5c, 6.9

Tasks + distillation + metrics + circuit breaker = 15+ полей.

**Рекомендация:** Вложенные structs: `DistillState{Tasks, Distill, Metrics}`.

---

### M9. Config surface explosion: 8+ новых полей без validation

**Нашёл:** architect-4
**Stories:** 6.2, 6.5a

20+ полей в flat Config struct. Нет Validate().

**Рекомендация:** Nested `KnowledgeConfig struct` + Validate(). Hardcode cooldown/target_pct для v1.

---

### M10. MCP config parsing хрупкий

**Нашли:** analyst-3, architect-3
**Stories:** 6.7

Story 6.7 смешивает `.mcp.json` (Cursor) и `.claude/settings.json` (Claude Code). Формат НЕ стандартизирован.

**Рекомендация:** Парсить ТОЛЬКО `.claude/settings.json`. Defensive JSON. Или config field `code_indexer_mcp`.

---

### M11. Cross-language extension→glob mapping не определена

**Нашли:** architect-3, analyst-4
**Stories:** 6.5b, 6.8

Go-centric примеры в AC. Маппинг .py/.ts/.java → globs не определён.

**Рекомендация:** Go-only hardcoded table для v1 (4 языка). Без LLM в цепочке.

---

### M12. Story 6.5 sub-story dependency order — circular

**Нашёл:** analyst-2
**Stories:** 6.5a/b/c

6.5a (trigger) зависит от 6.5c (DistillState типы). 6.5b (pipeline) зависит от 6.5c (ValidateDistillation).

**Рекомендация:** Переупорядочить: **6.5c → 6.5a → 6.5b** (types first, then consumers).

---

### M13. Validation thresholds вызывают false rejections на правильных merges

**Нашёл:** analyst-2
**Stories:** 6.5c

Citation preservation >=80%: distillation merge 3 пары → 70% < 80% → REJECT на хорошую дистилляцию.

**Рекомендация:** Считать unique TOPICS (не citations). Снизить category threshold до 60%.

---

### M14. Timeout 2 min — на грани для больших файлов

**Нашёл:** analyst-2
**Stories:** 6.5a

Claude 4.5 Sonnet: 0.015s/token × 8K output = 120s = ровно timeout. P99 = 2-3× P50.

**Рекомендация:** Default 180s (3 min) или оставить 120s configurable.

---

### M15. Integration tests: 12 scenarios < 86 AC

**Нашли:** analyst-3, architect-3
**Stories:** 6.8

~14% coverage ratio. Missing: template injection protection, all-stale entries, budget overflow cascade.

**Рекомендация:** +3 edge case scenarios минимум.

---

## LOW (4)

### L1. Citation как untyped string — парсинг не специфицирован

**Нашёл:** architect-1 | **Story:** 6.1

### L2. MonotonicTaskCounter — over-naming для single-process CLI

**Нашёл:** analyst-2 | **Story:** 6.5a
Рекомендация: переименовать в `TasksCompleted int`.

### L3. Dual distill_gate mode — 20 test combinations

**Нашёл:** analyst-2 | **Story:** 6.5a

### L4. Trend tracking division by zero

**Нашёл:** architect-2 | **Story:** 6.9
Рекомендация: guard `TotalTasks == 0 → return 0.0`.

---

## ОСПОРЕННЫЕ РЕШЕНИЯ (агенты не согласны)

### S1: distill_gate default — `human` vs `auto`

- **analyst-4:** Default `auto` — bmad-ralph для автономной разработки. Codex, Devin = autonomous.
- **architect-4, analyst-2, architect-2:** Default `human` — безопасно для v1.
- **Счёт:** 3 vs 1 за `human` default.

### S2: freq:N — Claude считает vs Go считает

- **analyst-4 (оспаривает):** Ни один конкурент не делегирует подсчёт LLM. Go init freq:1 + hint.
- **Остальные:** Приняли v5 решение (Go validates monotonicity).

### S3: [needs-formatting] — оставить vs убрать

- **analyst-4:** Убрать. Reject + log.
- **analyst-1, architect-1, architect-4:** Оставить + фильтровать при injection.
- **Счёт:** 3 vs 1 за "оставить + фильтровать".

---

## ЧТО ВСЕ АГЕНТЫ ПОДТВЕРДИЛИ КАК ПРАВИЛЬНОЕ (полный консенсус)

| # | Решение | Подтвердили |
|---|---------|-------------|
| 1 | **3-layer distillation pipeline** (Go dedup → LLM compress → validate) | Все 8+1. Convergent с SimpleMem, A-MEM, MemOS |
| 2 | **File-based injection** (LEARNINGS.md + ralph-*.md) | Все. R3: 74% LoCoMo |
| 3 | **Progressive disclosure T1/T2/T3** через globs | Все. Cursor/Continue аналогично |
| 4 | **2-stage template** (Go template + string replace) | Все. Security + simplicity |
| 5 | **MonotonicTaskCounter** persisted | Все. Cross-session cooldown |
| 6 | **BEGIN/END markers** для distillation output | Все. Industry standard |
| 7 | **Canonical categories** (7 + misc + NEW_CATEGORY) | Все |
| 8 | **Serena = MCP, не CLI** | Все |
| 9 | **Injectable functions** (DistillFn, ReviewFn, etc.) | Все. Testability |
| 10 | **No FIFO/archive** при 200-300 lines | Все |
| 11 | **Self-review step** conditional на HasLearnings | Все (analyst-3 оспорил claim +12%, не step) |
| 12 | **Snapshot-diff model** (идея) | Все подтвердили МОДЕЛЬ, оспорили РЕАЛИЗАЦИЮ |
| 13 | **knowledge_*.go split в runner/** (не отдельный пакет) | Все |
| 14 | **2-generation backups** | 7 из 8 (analyst-4: 1-gen для v1) |

---

## РЕКОМЕНДАЦИИ ПО УПРОЩЕНИЮ V1

| Что | Действие | Экономия AC | Кто предложил |
|-----|----------|-------------|---------------|
| **Story 6.9** (Trend Tracking) | Отложить на Growth | ~8 AC | analyst-4, architect-4 |
| **Story 6.7** (Serena) | Merge в 6.2 как 2 AC | ~4 AC | analyst-4, architect-3 |
| **ralph-index.md** | Убрать | ~2 AC | architect-3, architect-4 |
| **ANCHOR marker** | Убрать из v1 | ~3 AC | analyst-4 |
| `distill_cooldown` config | Hardcode = 5 | 1 AC | architect-4 |
| `distill_target_pct` config | Hardcode = 60% | 1 AC | architect-4 |
| **2-gen backups** | 1-gen для v1 | ~1 AC | analyst-4 |
| **Cross-language** scope hints | Go-only hardcoded table | ~2 AC | analyst-4 |
| **Итого** | | **~22 AC** | |

**Было:** 86 AC, 11 stories. **Рекомендация:** ~64 AC, 9 stories.

---

## ЧТО ЗАИМСТВОВАТЬ У КОНКУРЕНТОВ (топ-5 из research)

| # | Что | Откуда | Как применить |
|---|-----|--------|--------------|
| 1 | Distillation prompt gets existing rules as anchor | Model Collapse ICLR 2025 | ralph-*.md в distill prompt как "teacher context" |
| 2 | Per-failure-type CB thresholds | Portkey, Google Cloud SRE | validation=1, timeout=2, bad_format=1+retry |
| 3 | Atomic file operations (write-temp-rename) | Dan Luu "Files are hard", renameio | atomicWriteFile() + intent file |
| 4 | Stat cache для WSL performance | wsl-ntfs patterns | citationCache map[string]bool per session |
| 5 | Budget-aware injection | JetBrains NeurIPS, SFEIR, HumanLayer ACE | Graceful degradation при overflow |

---

## КОНКУРЕНТНАЯ ПОЗИЦИЯ (обновлено)

| Dimension | bmad-ralph | Copilot | Cursor | Devin | CrewAI | Codex |
|-----------|-----------|---------|--------|-------|--------|-------|
| Auto-extraction | **LEAD** (3 sources) | Learning | None | Wiki | Auto (RAG) | Doc-gardening |
| Distillation | **UNIQUE** (3-layer) | None | None | None | None | None |
| Scoped injection | **LEAD** (auto globs) | Instructions | Manual rules | N/A | Scope+category | AGENTS.md |
| Quality gates | **LEAD** (13 checks) | None | None | N/A | LLM scoring | Linters |
| Violation tracking | **UNIQUE** | None | None | N/A | None | None |
| Progressive disclosure | **ON PAR** | Basic | Good | N/A | Good | Good |

**Уникальная позиция:** Ни один из конкурентов не реализует автоматическую 3-layer distillation pipeline для project-specific rules. Это architectural innovation.

---

## СВОДНАЯ МАТРИЦА

| Severity | Count | IDs |
|----------|-------|-----|
| CRITICAL | 2 | CR1 (multi-file atomicity), CR2 ("last 20%" criterion) |
| HIGH | 9 | H1-H9 |
| MEDIUM | 15 | M1-M15 |
| LOW | 4 | L1-L4 |
| **Total** | **30** | |
| Подтверждено правильным | 14 | Не трогать |
| Оспорено | 3 | S1-S3 |
| Упрощения | -22 AC | 86→64, 11→9 stories |

---

## ИНДЕКС ОТЧЁТОВ АГЕНТОВ

| Файл | Содержание |
|------|-----------|
| `docs/reviews/pair3-analyst-injection-review.md` | Пара 3 analyst: 7 findings (3H, 3M, 1L) |
| `docs/reviews/pair3-architect-report.md` | Пара 3 architect: 8 findings (2C, 3H, 3M) |
| `docs/reviews/pair2-architect-v5-review.md` | Пара 2 architect: 10 findings (1C, 4H, 4M, 1L) |
| `docs/reviews/pair2-analyst-v5-review.md` | Пара 2 analyst (original): 8 findings (1C, 4H, 3M) |
| `docs/reviews/pair4-analyst-review.md` | Пара 4 analyst: 5 gaps + 6 contested + 5 simplifications |
| Inline (team lead context) | Пара 1 analyst: 7 findings (2H, 3M, 2L) |
| Inline (team lead context) | Пара 1 architect: 7 findings (2H, 4M, 1L) |
| Inline (team lead context) | Пара 4 architect: 7 findings (3H, 3M, 1L) |
| Inline (team lead context) | Пара 2 analyst-respawn: 5 findings (1H, 3M, 1L) |

---

## ИСТОРИЯ РАУНДОВ

| Раунд | Агенты | Raw findings | Уникальных | Результат |
|-------|--------|-------------|------------|-----------|
| 1 (v3→v4) | 10 (5 пар) | ~50 | 28 | v4: 9 stories, 78 AC |
| 2 (v4→v5) | 6 (3 пары) | ~46 | 11 корректировок | v5: 11 stories, 86 AC |
| 3 (v5→v6?) | 9 (4 пары+1) | ~55 | 30 | Pending user decisions |
| **Total** | **25 агентов** | **~151** | **~65 уникальных** | |

**Принцип:** Агенты находят проблемы и предлагают опции. Пользователь принимает решения.

---

*Консолидация подготовлена team lead на основе 9 отчётов агентов, 80+ web citations, анализа 86 AC и 39 предыдущих решений.*
