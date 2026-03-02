# Epic 6: Decision Log — Все решения и обоснования

**Дата:** 2026-03-02
**Формат:** Хронологический лог решений по Epic 6 через 3 раунда ревью

---

## Раунд 1: 10-agent review (5 пар аналитик+архитектор) → 28 решений (v3 → v4)

### Источники
- `docs/research/epic6-consolidated-review.md` — консолидированные 28 находок
- `docs/research/epic6-extraction-pipeline-review.md`
- `docs/research/epic6-injection-universality-review.md`
- `docs/research/review-6.5-6.6-consensus.md`
- `docs/research/review-6.5-6.6-architect-analysis.md`

### CRITICAL (4 решения)

#### C1: Write path — кто пишет в LEARNINGS.md?
- **Вопрос:** Pending-file (Claude → `.ralph/pending-lessons.md` → Go обрабатывает) или прямая запись (Claude → LEARNINGS.md → Go post-validates)?
- **Опции:** A) Pending-file pattern. B) Claude пишет напрямую, Go делает snapshot-diff.
- **Решение:** **B — Claude пишет напрямую.** Snapshot-diff post-validation.
- **Почему:** Проще. Нет промежуточного файла. Аналогично подходу OpenAI harness-engineering — модель пишет прямо в файлы, не нужна промежуточная абстракция. Post-validation gates ловят проблемы формата после факта.

#### C2: Unbounded growth — что делать при failure дистилляции?
- **Вопрос:** Автоматический circuit breaker или human gate?
- **Опции:** A) CB OPEN/HALF-OPEN/CLOSED автомат. B) Human GATE на каждый failure.
- **Решение:** **B — Human GATE.**
- **Почему:** Для v1 дистилляция — экспериментальная функция. Пользователь должен контролировать. CB state machine — преждевременная оптимизация для 78 AC и 9 stories. Human gate проще реализовать и тестировать.

#### C3: Serena — CLI или MCP?
- **Вопрос:** Serena — CLI binary (exec.LookPath) или MCP server (JSON config)?
- **Решение:** **MCP server.** Детекция через `.claude/settings.json` / `.mcp.json`. Минимальный интерфейс: `CodeIndexerDetector{Available() bool, PromptHint() string}`.
- **Почему:** Serena реально MCP server, не CLI. Не вызывать напрямую, только prompt hint.

#### C4: [needs-formatting] — мёртвая ветка?
- **Вопрос:** Убрать tag или оставить?
- **Решение:** **Оставить.** Go тегирует невалидные entries. Фиксятся при дистилляции.
- **Почему:** Знания ценнее формата. [needs-formatting] сохраняет знания (zero knowledge loss). Claude разберётся при дистилляции. Лучше грязно сохранить, чем потерять.

### HIGH (8 решений)

#### H1: Cooldown counter — ephemeral или persistent?
- **Решение:** **MonotonicTaskCounter в DistillState.** Persisted, never resets. Cooldown: counter - lastDistillTask >= 5.
- **Почему:** Ephemeral counter сбрасывается при перезапуске — fatal bug для cooldown логики.

#### H2: Category drift — фиксированные или динамические?
- **Решение:** **7 canonical + misc. NEW_CATEGORY marker.** List only grows.
- **Почему:** Баланс между контролем и гибкостью. Категории: testing, errors, config, cli, architecture, performance, security + misc.

#### H3: HasLearnings — как определить наличие знаний?
- **Решение:** **bool в TemplateData.** Conditional self-review section в execute prompt.
- **Почему:** Простое, тестируемое. Self-review (+12% quality по Live-SWE-agent research).

#### H4: Failure definition — что считать ошибкой дистилляции?
- **Решение:** **5 сценариев:** crash (non-zero exit), hang (timeout >2min), bad format (missing markers), validation reject (8 criteria), I/O error.
- **Почему:** Исчерпывающий список. Bad format получает 1 бесплатный retry с reinforced prompt.

#### H5: Session markers — как определить "недавние" entries?
- **Решение:** **Last 20% entries = recent** (для дистилляции). Append-only → tail = newest.
- **Почему:** Не требует изменения формата файла. Простая арифметика.

#### H6: Output format — как парсить вывод дистилляции?
- **Решение:** **BEGIN/END markers + ## CATEGORY: sections + NEW_CATEGORY: marker.**
- **Почему:** Standard LLM structured output pattern. Go парсит только между маркерами — preamble/postamble игнорируются.

#### H7: Shared function — DRY для 3 call sites?
- **Решение:** **buildKnowledgeReplacements()** — shared function возвращает map для Stage 2.
- **Почему:** 3 call sites (initial, retry, review) — DRY threshold.

#### H8: Timeout — сколько ждать дистилляцию?
- **Решение:** **2 минуты.** `context.WithTimeout`. Configurable: `distill_timeout: 120`.
- **Почему:** Дистилляция = ~8K tokens. 2 минуты с запасом. Timeout = failure → gate.

### MEDIUM (12 решений)

#### M1: Resume + Prompt compatibility
- **Решение:** Fix `else if` в session.go. `--resume` и `-p` совместимы.
- **Почему:** Officially compatible в Claude CLI.

#### M2: Mutation asymmetry — review CAN write LEARNINGS.md
- **Решение:** Обновить prompt invariants. Убрать "MUST NOT write LEARNINGS.md" из review.
- **Почему:** Epic 6 разрешает review писать знания при findings.

#### M3: LessonsData struct — внутренняя валидация
- **Решение:** `LessonEntry{Category, Topic, Content, Citation}`, `LessonsData{Source, Entries}`.
- **Почему:** Per-entry validation вместо full-text parsing.

#### M4: Scope hints — как определить язык проекта?
- **Решение:** Go сканирует extensions top 2 levels → передаёт Claude → Go validates globs.
- **Почему:** Deterministic scan + LLM для human-readable globs. Go validates syntax.

#### M5: CodeIndexer — минимальный интерфейс
- **Решение:** Следствие C3. 2 метода, не больше.
- **Почему:** YAGNI.

#### M6: Injection CB — убрать
- **Решение:** **Убран.** Нет автоматической защиты от overflow (на тот момент).
- **Почему:** Human gate (C2) даёт пользователю контроль. Если skip — его выбор.

#### M7: Crash recovery — восстановление при аварии
- **Решение:** Check .bak at startup → restore if found → log warning.
- **Почему:** Простое, надёжное. Не over-engineered.

#### M8: YAML validation + A/B testing
- **Решение:** ValidateDistillation проверяет YAML frontmatter. Новая Story 6.9 для A/B.
- **Почему:** Invalid frontmatter = файл невидим для Claude Code. Критично.

#### M9: JIT validation — os.Stat only
- **Решение:** Только проверка существования файла. Line range → Growth.
- **Почему:** Минимально, быстро. Line range слишком fragile (файлы меняются).

#### M10: CB escalation
- **Решение:** Закрыт — разрешён через C2 (GATE).

#### M11: freq:N counting
- **Решение:** Claude назначает, Go проверяет монотонность (new >= old).
- **Почему:** Приблизительный сигнал — точность до единицы не нужна. T1 promotion при ~10, не обязательно ровно 10. Проще чем Go-only counting.

#### M12: Cross-language
- **Решение:** Go, Python, JS/TS, Java, mixed stacks в тестах.
- **Почему:** bmad-ralph должен работать для любых проектов, не только Go.

### LOW (4 решения)

#### L1: G5/G6 constants
- **Решение:** Named constants. G6: 10→20 chars.
- **Почему:** Магические числа → константы. 20 chars минимум для осмысленного контента.

#### L2: Thread safety
- **Решение:** YAGNI, no mutex.
- **Почему:** Sequential architecture. Один goroutine.

#### L3: Reverse ordering
- **Решение:** Split by `\n## `, reverse, rejoin.
- **Почему:** Newest-first при injection. Append-only file → tail = newest.

#### L4: Backup + ANCHOR
- **Решение:** .bak + .bak.1 (2-generation). ANCHOR auto при freq >= 10.
- **Почему:** 2 поколения = откат на 2 дистилляции. ANCHOR = защита от model collapse для high-frequency patterns.

#### L5: Misc monolith
- **Решение:** ralph-misc.md always loaded via Stage 2. No globs. H2 prevents growth.
- **Почему:** Misc-знания релевантны всегда. NEW_CATEGORY предотвращает бесконтрольный рост.

#### L6: Race condition
- **Решение:** Advisory note only. File lock → Growth.
- **Почему:** CLI tool, single user. Advisory достаточно.

---

## Раунд 2: 6-agent review (3 пары аналитик+архитектор) → 11 корректировок (v4 → v5)

### Источники
- `docs/reviews/pair1-algorithm-review.md` — Пара 1: алгоритм и pipeline (17 находок)
- `docs/reviews/pair2-architecture-review.md` — Пара 2: архитектура и интеграция (16 находок)
- `docs/reviews/pair3-decisions-review.md` — Пара 3: решения и альтернативы (13+ находок)

### Оспаривались 7 решений из Раунда 1 + найдены 4 новые проблемы

#### [v5-1] C1: Snapshot-diff хрупок? → Оставлен + line-count guard
- **Оспаривание:** Все 3 пары считали snapshot-diff фундаментально хрупким (Claude может rewrite файл).
- **Контраргумент пользователя:** В harness-engineering (OpenAI) и подобных проектах модель пишет прямо в файлы, нет pending-file, нет diff-валидации. Prompt instructions достаточны на практике (99%+ compliance).
- **Решение:** **Оставить snapshot-diff.** Добавить line-count guard: если строк стало меньше → log warning, full revalidation. 1 строка кода, ловит rewrite/delete.
- **Почему:** Практика > теория. Академические edge cases не оправдывают complexity pending-file.

#### [v5-2] C2: Human gate блокирует автономность → distill_gate config
- **Оспаривание:** 2 из 3 пар: human gate блокирует ночные прогоны, CI, автономность.
- **Решение:** **Config flag `distill_gate: human|auto`, default `human`.** Human = как было. Auto = CB auto-skip после 3 consecutive failures.
- **Почему:** Оба режима нужны. Interactive = human. CI/batch = auto. Пользователь выбирает.

#### [v5-3] C4: [needs-formatting] — мёртвая ветка? → Оставлен, инъектируется
- **Оспаривание:** Все 3 пары рекомендовали убрать.
- **Контраргумент пользователя:** Знания валидные, формат просто непонятен. Пусть остаётся, Claude потом разберётся при дистилляции.
- **Решение:** **Оставить.** Инъектировать как есть (без фильтрации).
- **Почему:** Потеря знаний хуже чем мусор в контексте. 1-2 плохо отформатированные записи не ломают compliance.

#### [v5-4] H5: "Last 20% injected" → Inject ALL
- **Оспаривание:** Пары 1, 3: потеря 80% знаний при injection.
- **Контраргумент пользователя:** LEARNINGS.md = буфер свежих записей. Старые знания уже дистиллированы в ralph-*.md. "20%" не нужно — буфер и так содержит только недавнее.
- **Решение:** **Inject ALL из LEARNINGS.md.** Убрать "last 20%" из injection.
- **Почему:** Двухуровневая архитектура: LEARNINGS.md (hot buffer) + ralph-*.md (distilled). Нет потери — старое уже в rules.

#### [v5-5] M6: Нет защиты от overflow → stderr warning
- **Оспаривание:** Пара 3: убраны все safety nets (FIFO, injection CB, auto CB).
- **Решение:** **Stderr warning** при LEARNINGS.md > budget. Формат: `⚠ LEARNINGS.md: {lines}/{budget} lines ({ratio}x budget)`.
- **Почему:** В human mode пользователь сам решает skip → его ответственность. В auto mode CB скипает молча → нужен warning чтобы пользователь заметил. Не блокирует, просто информирует.

#### [v5-6] M11: freq:N — LLM плохо считает → Оставлен
- **Оспаривание:** Пары 1, 3: LLM ненадёжно считает, Go должен.
- **Решение:** **Оставить как есть.** Claude считает, Go проверяет монотонность.
- **Почему:** freq:N = приблизительный сигнал. Точность до единицы не нужна. freq:7 вместо freq:5 → T1 promotion чуть раньше, не катастрофа. Go-only counting добавляет значительную сложность (frequency map, topic matching) ради точности которая не критична.

#### [v5-7] A/B testing → Trend tracking
- **Оспаривание:** Пары 1, 3: метрики confounded, sample size недостаточен, нет baseline.
- **Решение:** **Упростить до trend tracking.** findings_per_task + first_clean_review_rate over time. Без mode switching.
- **Почему:** Scoped injection подтверждён research (progressive disclosure). A/B на 5-15 задачах = статистически бессмысленно. Trend = достаточный сигнал "система работает".

### Новые проблемы (не из Раунда 1)

#### [v5-8] DistillState: неправильное расположение, нет backup, нет Version
- **Находка:** Пара 2: `LEARNINGS.md.state` привязан по имени к data file, не в backup, нет версионирования.
- **Решение:** Переместить в `.ralph/distill-state.json`. Включить в backup. Добавить `Version int`.
- **Почему:** SRP — state отдельно от данных. Backup — consistency. Version — forward compatibility.

#### [v5-9] knowledge.go → God file
- **Находка:** Пара 2: 500-800 строк с 10+ ответственностями в одном файле.
- **Решение:** Split на 4 файла: knowledge_write.go, knowledge_read.go, knowledge_distill.go, knowledge_state.go.
- **Почему:** Maintainability. Каждый файл = одна ответственность. Не нужен новый пакет — достаточно split внутри runner/.

#### [v5-10] Story 6.5 слишком большая
- **Находка:** Пара 3: 18 AC, 11 concerns. Среднее по Epics 1-5 = 5-8 AC.
- **Решение:** Split на 3: 6.5a (Budget + trigger, 8 AC), 6.5b (Session + parsing, 11 AC), 6.5c (Validation + state, 6 AC).
- **Почему:** Testability. Каждая story тестируется independently. Реализуется последовательно.

#### [v5-11] Подтверждено без изменений
Следующие решения Раунда 1 подтверждены всеми 3 парами как правильные:
- **C3** (Serena = MCP)
- **H1** (MonotonicTaskCounter)
- **H2** (Canonical categories)
- **H6** (BEGIN/END markers)
- **H8** (Timeout 2 min)
- **L4** (2-generation backups) — расширен на DistillState
- **L6** (Advisory note)
- **M7** (Crash recovery)
- **M9** (JIT os.Stat only)

---

## Итоговая статистика

| Раунд | Агенты | Находки | Решения | Результат |
|-------|--------|---------|---------|-----------|
| Раунд 1 (v3→v4) | 10 (5 пар) | 28 | 28 | v4: 9 stories, 78 AC |
| Раунд 2 (v4→v5) | 6 (3 пары) | ~46 (dedup) | 11 корректировок | v5: 11 stories, 86 AC |

**Принцип:** Пользователь принимает все решения. Агенты находят проблемы и предлагают опции. Решения принимаются с учётом практического опыта (harness-engineering, Cursor rules) а не только теоретической чистоты.
