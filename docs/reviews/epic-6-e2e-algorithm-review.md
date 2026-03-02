# Epic 6: Сквозной алгоритм знаний — Критический обзор (Консенсус)

**Авторы:** analyst-e2e + architect-e2e
**Дата:** 2026-02-28
**Scope:** Полный жизненный цикл знаний: рождение → валидация → инъекция → дистилляция → промоция
**Файлы проанализированы:** epic-6-*.md, 3 research reports (R1-R3), runner.go, knowledge.go, prompt.go, config.go, session.go, execute.md, review.md, defaults.yaml

---

## Executive Summary

Обнаружено **16 архитектурных разрывов** (2 CRITICAL, 5 HIGH, 7 MEDIUM, 2 LOW).
Фундаментальная проблема: **эпик описывает два взаимоисключающих пути записи знаний**
(Claude через file tools vs Go через WriteLessons), делая 6 quality gates мёртвым кодом
для основного пути. Без исправления GAP-1 и GAP-2 вся 3-слойная архитектура (Layer 1:
Go dedup, Layer 2: LLM distill, Layer 3: safety nets) РАЗРУШЕНА — Layer 1 не работает.

**Консенсус обоих рецензентов:** эпик в целом архитектурно здоров (tiered memory,
circuit breaker, progressive disclosure — всё подтверждено 3 исследованиями), но
требует **9 исправлений перед реализацией** (2 CRITICAL + 5 HIGH + 2 обязательных MEDIUM).

---

## Диаграмма жизненного цикла

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                    KNOWLEDGE LIFECYCLE                          │
  │                                                                 │
  │  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
  │  │  BIRTH   │───>│ VALIDATE │───>│  INJECT  │───>│   USE    │  │
  │  │ (Write)  │    │ (Gates)  │    │ (Prompt) │    │(Session) │  │
  │  └──────────┘    └──────────┘    └──────────┘    └──────────┘  │
  │       │               │               │               │        │
  │       │               │               │               ▼        │
  │       │               │               │         ┌──────────┐   │
  │       │               │               │         │  REVIEW  │   │
  │       │               │               │         │(Findings)│   │
  │       │               │               │         └────┬─────┘   │
  │       │               │               │              │         │
  │       │               │               │              ▼         │
  │       │               │               │     ┌────────────────┐ │
  │       │               │               │     │   DISTILL      │ │
  │       │               │               │     │ (>150 lines +  │ │
  │       │               │               │     │  cooldown + CB)│ │
  │       │               │               │     └───────┬────────┘ │
  │       │               │               │             │          │
  │       │               │               │             ▼          │
  │       │               │               │     ┌────────────────┐ │
  │       │               │               │     │   PROMOTE      │ │
  │       │               │               │     │ ralph-*.md     │ │
  │       │               │               │     │ (T1/T2/T3)    │ │
  │       │               │               │     └────────────────┘ │
  │       │                                                        │
  │  WHO WRITES? ◄── GAP-1: Claude file tools OR Go WriteLessons?  │
  └─────────────────────────────────────────────────────────────────┘
```

---

## КРИТИЧЕСКИЕ РАЗРЫВЫ (CRITICAL) — Блокируют реализацию

### GAP-1 [CRITICAL]: Кто фактически пишет в LEARNINGS.md — Claude или Go?

**Консенсус:** Оба рецензента нашли независимо. Фундаментальное архитектурное противоречие.

**Доказательство из эпика:**
- Story 6.1 AC: `WriteLessons(ctx, LessonsData)` — Go-функция с 6 quality gates (G1-G6)
- Story 6.3 Technical Notes: "Claude inside resume-extraction session writes to LEARNINGS.md **directly via file tools**"
- Story 6.4 Technical Notes: "Claude inside review session does the actual writing — **not Ralph's Go code**"

**Почему это критично:** Если Claude пишет напрямую, gates G1-G6 НИКОГДА НЕ ВЫЗЫВАЮТСЯ.
Вся Layer 1 (Go-level semantic dedup при каждой записи, 0 tokens) — мёртвый код.
Мусорные записи (без цитат, дублированные, тривиальные) накапливаются бесконтрольно.

**Три модели исправления:**

| Модель | Как работает | Pros | Cons |
|--------|-------------|------|------|
| **A: Claude пишет → Go валидирует** | Claude пишет в LEARNINGS.md, Go diff-ит до/после сессии, тегирует невалидные | Простая интеграция с текущим потоком | Нужен diff для определения "новых" записей; нет pre-write protection |
| **B: Только Go пишет** | Prompt инструктирует Claude вывести уроки в structured format → runner парсит → WriteLessons | Gates гарантированы; согласуется с ParseResult паттерном | Нужен протокол парсинга; Claude может не следовать формату |
| **C: Гибрид temp-файл** | Claude пишет в LEARNINGS.draft.md → Go читает, валидирует, мержит | Компромисс | Дополнительный файл; два источника правды |

**Консенсусная рекомендация: Модель B** — "Только Go пишет"
- Prompt инструктирует Claude вывести `## LESSON:` блоки в определённом формате (часть stdout)
- Runner парсит structured output и вызывает WriteLessons для каждого блока
- Все 6 gates гарантированно работают
- Паттерн уже используется: session.ParseResult парсит JSON output

**Альтернативная рекомендация (architect-e2e):** Если Модель B слишком инвазивна для review
(review уже пишет в review-findings.md через file tools), допустима Модель A с пост-валидацией:
Go снапшотит LEARNINGS.md до сессии, diff-ит после, прогоняет новые записи через gates.
Невалидные → [needs-formatting] tag. Это проще реализовать, но слабее защищает.

---

### GAP-2 [CRITICAL]: [needs-formatting] tag — мёртвая ветка

**Консенсус:** Прямое следствие GAP-1. Оба рецензента нашли.

**Механизм поломки:**
1. Story 6.1: WriteLessons добавляет [needs-formatting] при fail G1/G2
2. Story 6.5: Distillation prompt "fix all [needs-formatting] entries"
3. Но если Claude пишет напрямую → tag НИКОГДА не добавляется
4. Distillation не знает какие записи сломаны → не исправляет → мусор вечен

**Исправление:** Автоматически следует из GAP-1 fix. При Модели B — tags добавляются Go.
При Модели A — post-validation добавляет tags к невалидным записям.

---

## ВЫСОКОПРИОРИТЕТНЫЕ РАЗРЫВЫ (HIGH) — Требуют исправления в эпике

### GAP-3 [HIGH]: Cooldown/Circuit breaker — счётчик задач сбрасывается

**Консенсус:** Оба рецензента нашли независимо, совпадение до деталей.

**Проблема:** `completedTasks := 0` в runner.go:368 — локальная переменная, сбрасывается
при каждом `ralph run`. DistillState.LastDistillTask хранит глобальный номер задачи.
- Run 1: distill при task 5 → LastDistillTask=5
- Run 2: completedTasks=0, условие `0-5=-5 >= 10` → false → ВЕЧНЫЙ cooldown

**Консенсусная рекомендация:** `AttemptsSinceFailure int` в DistillState, инкрементируется
при каждой проверке бюджета (каждый clean review), сбрасывается при failure.
Не зависит от session boundaries. Для cooldown аналогично: `ChecksSinceLastDistill int`.

---

### GAP-4 [HIGH]: Протокол формата output дистилляции не определён

**Источник:** analyst-e2e (architect-e2e подтверждает — нашёл как часть GAP-10a)

**Проблема:** Story 6.5 говорит "output grouped by category for multi-file split",
но AC не определяет точный формат выхода. Claude может вывести `### Testing`
вместо `## CATEGORY: testing`. Go не парсит → failure → CB OPEN.

ValidateDistillation проверяет 7 критериев, но НЕ проверяет parsability.

**Консенсусная рекомендация:**
1. Добавить criterion #8 к ValidateDistillation: "output parseable into categories"
2. Fallback: если parsing fails → записать весь output в ralph-misc.md (no split, no data loss)
3. Явно определить формат в AC Story 6.5: `## CATEGORY: <name>\n<entries>\n`

---

### GAP-5 [HIGH]: "Shared Stage 2 map" не существует — ложное описание

**Источник:** analyst-e2e (architect-e2e подтверждает)

**Проблема:** Story 6.2 tech notes: "All 3 AssemblePrompt call sites get replacements
automatically via shared Stage 2 map — no per-call-site changes needed." Но текущий код
имеет РАЗНЫЕ maps на каждом call site (runner.go:101, :398, :644). Shared map не существует.

**Консенсусная рекомендация:** Создать `buildKnowledgeReplacements(projectRoot string)
(map[string]string, error)` в runner/knowledge.go. Каждый call site merge-ит этот map
со своим. Исправить tech notes — убрать "no per-call-site changes needed".

---

### GAP-6 [HIGH]: Conditional self-review невозможен без HasLearnings bool

**Консенсус:** Оба нашли. Stage 1/Stage 2 gap.

**Проблема:** Self-review conditioned на non-empty `__LEARNINGS_CONTENT__` — Stage 2 placeholder.
Template conditionals — Stage 1 (`{{if .Field}}`). Нет поля `HasLearnings` в TemplateData.

**Консенсусная рекомендация:** Добавить `HasLearnings bool` в TemplateData.
Runner sets true когда validated learnings content non-empty.
Template: `{{- if .HasLearnings}}...self-review section...{{- end}}`.

---

### GAP-7 [HIGH]: Отсутствие timeout для distillation session

**Источник:** architect-e2e (analyst-e2e не покрыл)

**Проблема:** Distillation запускает claude -p через session.Execute. Если Claude зависнет
(infinite loop, network issue), runner заблокирован навечно. Текущий ctx из runner не имеет
timeout для distillation — тот же ctx что для всего run.

**Сценарий:** Auto-distillation triggers. claude -p зависает на 30 минут.
Runner не обрабатывает следующие задачи. Пользователь Ctrl+C → бэкапы не восстановлены.

**Консенсусная рекомендация:** `context.WithTimeout(ctx, 2*time.Minute)` для distillation.
Config field: `distill_timeout: 120` (seconds). При timeout → restore backups, CB failure++.

**Cross-reference:** При timeout → crash recovery handled by GAP-10 startup recovery (.bak check).

---

## СРЕДНЕ-ПРИОРИТЕТНЫЕ РАЗРЫВЫ (MEDIUM)

### GAP-8 [MEDIUM]: Resume-extraction не может inject knowledge prompt

**Источник:** analyst-e2e (architect-e2e подтверждает — уточнённый анализ)

**Проблема:** ResumeExtraction (runner.go:188) использует `--resume <sessionID>` без Prompt.
Resumed session не знает о LEARNINGS.md. Story 6.3 AC "prompt updated with knowledge
instructions" невыполнимо для --resume.

**Уточнение architect-e2e:** Проверил session.go:88-92 — Resume и Prompt взаимоисключающие
(`if opts.Resume != "" { ... } else if opts.Prompt != "" { ... }`). Нельзя подать оба.

**Консенсусная рекомендация:** Добавить "if session fails, extract insights and output
them in structured format" секцию в execute.md. Resumed session наследует инструкции.
Альтернативно: после resume-extraction, runner парсит output и вызывает WriteLessons
(зависит от GAP-1 Модель B — только если Go is the sole writer).

---

### GAP-9 [MEDIUM]: Scope hints auto-detection в pipe mode

**Источник:** architect-e2e (analyst-e2e не покрыл)

**Проблема:** Story 6.5: "scope hints auto-detected from project file types... determined by
distillation prompt based on project file types." Claude в pipe mode (-p) имеет file tools
(--dangerously-skip-permissions), НО distillation prompt подаёт только LEARNINGS.md content.
Claude должен будет СКАНИРОВАТЬ проект для определения типов файлов — это медленно и
ненадёжно. Либо Go-код должен определить типы файлов и передать в prompt.

**Консенсусная рекомендация:** Go-код сканирует extensions в projectRoot (O(1) walk),
передаёт в distillation prompt как context: "Project file types: .go, .md".
Distillation prompt использует эту информацию для scope hints.

---

### GAP-10 [MEDIUM]: Multi-file write atomicity / crash recovery

**Источник:** architect-e2e (analyst-e2e не покрыл как отдельный GAP)

**Проблема:** Distillation пишет: LEARNINGS.md + N ralph-*.md + ralph-index.md.
Backup создаётся перед distillation, restore при validation failure. Но при CRASH
(OOM, kill, Ctrl+C) бэкапы существуют, но никто их не восстанавливает.

**Консенсусная рекомендация:**
1. При старте runner.Run(): проверить наличие .bak файлов
2. Если .bak существуют → restore (предыдущий distill crashed)
3. Логировать: "Restored from backup — previous distillation crashed"

---

### GAP-11 [MEDIUM]: ValidateDistillation не проверяет YAML frontmatter

**Консенсус:** analyst-e2e упомянул в ответе #7, architect-e2e выделил как отдельный GAP

**Проблема:** 7 criteria ValidateDistillation проверяют контент, но не формат ralph-*.md.
Claude может написать невалидный YAML → broken scope hints → файлы не загружаются
Claude Code. Без проверки YAML — distillation считается успешной, но файлы бесполезны.

**Консенсусная рекомендация:** Criterion #9: "All ralph-*.md files have valid YAML
frontmatter with globs: field." Простая проверка: `yaml.Unmarshal(header, &struct{ Globs []string })`.

---

### GAP-12 [MEDIUM]: JIT validation line range = O(N * file_read)

**Источник:** analyst-e2e

**Проблема:** Tech notes: "line in range?" требует чтения файла. 50 entries × read = 250-500ms
на WSL/NTFS. Tech notes заявляют "~1ms" — верно только для stat.

**Консенсусная рекомендация:** Только file existence check (os.Stat, не line range).
Достаточно для staleness detection. Line range — Growth phase.

---

### GAP-13 [MEDIUM]: Бесконечный рост при CB OPEN — нет эскалации

**Консенсус:** Оба нашли. analyst-e2e как GAP-9, architect-e2e как #4/#5.

**Проблема:** CB OPEN → auto-distill заблокирован. LEARNINGS.md растёт бесконечно.
Лог "circuit breaker OPEN" в log, не в stderr. Пользователь не видит.
HALF-OPEN → fail → OPEN → бесконечный цикл без back-off.

**Консенсусная рекомендация:**
1. **Обязательно:** stderr warning при CB OPEN с instructions `ralph distill`
2. **Nice-to-have:** escalating probe — после 20 checks → force HALF-OPEN (не ждать)

---

### GAP-14 [MEDIUM]: Semantic dedup — near-duplicates проникают

**Источник:** analyst-e2e (architect-e2e подтверждает)

**Проблема:** `strings.ToLower + TrimSpace` не нормализует дефисы vs пробелы.
`"testing: assertion-quality"` != `"testing: assertion quality"`.

**Консенсусная рекомендация:** Добавить `strings.ReplaceAll(heading, "-", " ")` перед
сравнением. Дёшево, ловит 90% near-duplicates.

---

## НИЗКОПРИОРИТЕТНЫЕ РАЗРЫВЫ (LOW)

### GAP-15 [LOW]: Lossy frequency counting при merge

**Источник:** analyst-e2e

**Проблема:** При merge двух entries [freq:3] + [freq:2] — Claude может не сложить.
Entry никогда не достигнет freq:10 для T1 promotion.

**Рекомендация:** Explicit instruction в distillation prompt: "When merging entries,
SUM their [freq:N] values."

---

### GAP-16 [LOW]: Distillation session может писать в LEARNINGS.md

**Источник:** architect-e2e

**Проблема:** Distillation session (claude -p --dangerously-skip-permissions) имеет file
tools. Может случайно записать в LEARNINGS.md (circular). Prompt не запрещает явно.

**Рекомендация:** Добавить invariant в distillation prompt: "MUST NOT write any files.
Output ALL content to stdout ONLY."

---

## СВОДКА

### Матрица severity × fix complexity

| Severity | Count | Easy fix (AC update) | Medium fix (redesign) |
|----------|-------|---------------------|----------------------|
| CRITICAL | 2 | 0 | 2 (GAP-1, GAP-2) |
| HIGH | 5 | 4 (GAP-3,4,5,6) | 1 (GAP-7) |
| MEDIUM | 7 | 6 | 1 (GAP-10) |
| LOW | 2 | 2 | 0 |
| **Total** | **16** | **12** | **4** |

### Обязательные исправления перед реализацией (9 штук)

1. **GAP-1**: Определить ЕДИНУЮ модель записи (рекомендация: Модель B — Go writes)
2. **GAP-2**: Автоматически решается GAP-1
3. **GAP-3**: Заменить task counter на AttemptsSinceFailure / ChecksSinceLastDistill
4. **GAP-4**: Определить формат output дистилляции + criterion #8 + fallback
5. **GAP-5**: Создать buildKnowledgeReplacements(), исправить tech notes
6. **GAP-6**: Добавить HasLearnings в TemplateData
7. **GAP-7**: context.WithTimeout для distillation + config field
8. **GAP-10**: Crash recovery — проверка .bak при старте
9. **GAP-11**: Criterion #9 — YAML frontmatter validation

### Рекомендуемые исправления (7 штук, при реализации)

- GAP-8: Resume knowledge instructions в execute.md
- GAP-9: Go-code file type detection для scope hints
- GAP-12: File existence only (drop line range)
- GAP-13: Stderr warning при CB OPEN
- GAP-14: Normalize hyphens в dedup
- GAP-15: SUM instruction для freq counting
- GAP-16: File write invariant в distillation prompt

---

## Позитивные аспекты эпика (что НЕ нужно менять)

Оба рецензента отмечают:

1. **Tiered memory (hot/distilled/promoted)** — подтверждён 3 исследованиями (MemGPT, MemOS, Copilot)
2. **Circuit breaker pattern** — правильный подход к non-deterministic LLM operations
3. **JIT citation validation** — вдохновлён GitHub Copilot (+7% merge rate)
4. **[needs-formatting] вместо reject** — zero knowledge loss design (правильно)
5. **File-based injection** — оптимален для <500 entries (R3: 74.0% LoCoMo)
6. **Self-review step** — подтверждён Live-SWE-agent (+12% quality)
7. **Progressive disclosure T1-T3** — R2-R6 proven pattern
8. **No FIFO/archive** — правильное решение (300 lines = 3-4% context)
9. **Backup перед distillation** — critical safety net
10. **ralph distill как manual escape hatch** — правильный fallback
11. **Mutex в WriteLessons** (Story 6.1 AC: thread-safe writes) — правильный defensive design

**Вердикт:** Эпик архитектурно здоров. 16 GAP-ов — это gaps в СПЕЦИФИКАЦИИ (AC/tech notes),
не в архитектурных решениях. 12 из 16 — easy AC updates. 4 требуют minor redesign.
После исправлений эпик готов к реализации.
