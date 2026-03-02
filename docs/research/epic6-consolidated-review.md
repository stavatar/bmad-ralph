# Epic 6: Консолидированный обзор — все 5 пар агентов

**Дата:** 2026-02-28
**Координатор:** Claude (team lead epic6-final-review)
**Участники:** 10 агентов (5 пар аналитик + архитектор)
**Scope:** Epic 6 — Knowledge Management & Polish (8 stories, 59 AC)

---

## Методология

5 независимых пар провели adversarial review Epic 6, каждая со своим фокусом:

| Пара | Фокус | Файл отчёта |
|------|-------|-------------|
| 1 | Extraction pipeline (Stories 6.1, 6.3, 6.4) | `epic6-extraction-pipeline-review.md` |
| 2 | Distillation & budget (Stories 6.5, 6.6) | `review-6.5-6.6-consensus.md` |
| 3 | Injection & universality (Stories 6.7, 6.8) | `epic6-injection-universality-review.md` |
| 4 | Сквозной алгоритм (все Stories) | `../reviews/epic-6-e2e-algorithm-review.md` |
| 5 | Конкурентный анализ | `epic6-competitive-analysis.md` |

Каждая пара: независимый анализ → cross-review → consensus document.
Источники: Epic 6 (859 строк), 3 research reports (R1-R3, 62 источника), source code (runner/, config/, session/, prompts/), web research 2025-2026.

Raw findings: ~50. После дедупликации: **28 уникальных**.

---

## CRITICAL (4 проблемы)

### C1. Двойной write path — Go quality gates vs Claude direct writes

**Найдена:** Пары 1, 4 (независимо)
**Stories:** 6.1, 6.3, 6.4, 6.5, 6.8

**Проблема:** Фундаментальное противоречие:
- Story 6.1: Go-код через `WriteLessons()` с 6 quality gates (G1-G6)
- Story 6.3 Technical Notes: "Claude inside resume-extraction session writes to LEARNINGS.md directly via file tools"
- Story 6.4 Technical Notes: "Claude inside review session does the actual writing — not Ralph's Go code"

Если Claude пишет напрямую — gates G1-G6 **никогда не вызываются**. Layer 1 (Go semantic dedup, 0 tokens) = мёртвый код. Мусор накапливается бесконтрольно.

**Research backing:** Context rot 30-50% при неконтролируемом росте [R1, S5]. DGM: качество знаний → 20%→50% effectiveness [S37].

**Консенсус (Пара 1):** Pending-file pattern:
1. Claude пишет в `.ralph/pending-lessons.md` (transient file)
2. Go читает → парсит в `[]LessonEntry` → применяет G1-G6 к каждому entry
3. Валидные → append к LEARNINGS.md
4. Невалидные → log + reject
5. Удаляет `.ralph/pending-lessons.md`

**Альтернатива (Пара 4):** Model B — Go-only writes. Claude выводит `## LESSON:` блоки в stdout, runner парсит structured output. Паттерн уже используется (session.ParseResult).

**Бонус:** Решает сразу C1, C4, M2, и thread safety concern.

---

### C2. Unbounded LEARNINGS.md growth при stuck circuit breaker

**Найдена:** Пары 2, 4 (независимо)
**Stories:** 6.2, 6.5

**Проблема:** CB OPEN → auto-distill заблокирован → LEARNINGS.md растёт бесконечно → injection degrades ALL tasks.
- AC утверждает "300 lines = 3-4% context (200K)". Вводит в заблуждение: реальный budget ~70-100K (system prompt, tools занимают 30-50%).
- SFEIR [R2, S25]: ~15 правил = 94% compliance. 300+ строк = 60-100+ правил → beyond threshold.
- LLM Scaling Paradox [arxiv Feb 2026]: compressor LLMs overwrite source facts с own priors.

**Консенсус:**
1. **Soft warning** при Lines > 2× budget (400): log + stderr warning
2. **Injection circuit breaker** при Lines > 3× budget (600): STOP injecting (`__LEARNINGS_CONTENT__` = empty). Запись продолжается (zero knowledge loss)
3. Успешная distillation автоматически восстанавливает injection (self-healing)
4. НЕ FIFO, НЕ archive, НЕ truncation — automatic degradation of injection при overflow

---

### C3. Serena = MCP server, а не CLI

**Найдена:** Пара 3
**Stories:** 6.7, 6.8

**Проблема:** Story 6.7 целиком основана на несуществующем CLI:
- `exec.LookPath("serena")` — бессмысленно (Serena запускается через MCP config)
- `serena index --full` — команда не существует
- `serena query <symbol>` — команда не существует

Serena (github.com/mcp-sh/serena) — MCP-сервер, работает через JSON-RPC. 11 AC в Story 6.7 нереализуемы.

**Консенсус:**
- **Детекция:** Проверять `.claude/settings.json` или `.mcp.json` на наличие Serena MCP server config. НЕ `exec.LookPath`
- **Взаимодействие:** Ralph не вызывает Serena напрямую. Prompt instructions: "If Serena MCP tools available, use them"
- **Interface:** Минимальный `CodeIndexerDetector{Available(), PromptHint()}` вместо полного CodeIndexer

---

### C4. [needs-formatting] tag — мёртвая ветка

**Найдена:** Пары 1, 4 (независимо)
**Stories:** 6.1, 6.5, 6.8

**Проблема:** Двойная поломка:
1. Если Claude пишет напрямую (C1) — tag никогда не добавляется Go-кодом
2. Distillation не знает какие записи сломаны → не исправляет → мусор вечен
3. Pollution: [needs-formatting] entries = бесполезный контекст до distillation

**Консенсус:** Автоматически решается при C1 fix:
- Pending-file pattern: невалидные entries → reject + log warning → LEARNINGS.md = всегда чистый файл
- Убрать [needs-formatting] из формата, AC, и ValidateDistillation

---

## HIGH (8 проблем)

### H1. Cooldown counter сбрасывается при restart

**Найдена:** Пары 2, 4 (независимо)
**Stories:** 6.5

**Проблема:** `completedTasks := 0` (in-memory) vs `DistillState.LastDistillTask` (persisted).
- Run 1: distill при task 5 → LastDistillTask=5
- Run 2: completedTasks=0 → `0-5=-5 >= 10` = false → **вечный cooldown**

**Консенсус:** `MonotonicTaskCounter` в DistillState (persisted, инкрементируется при каждом clean review).
- Cooldown: `MonotonicTaskCounter - LastDistillTask >= 5`
- HALF-OPEN probe: `MonotonicTaskCounter - LastFailureTask >= 10`

---

### H2. Non-deterministic category naming

**Найдена:** Пары 2, 3
**Stories:** 6.1, 6.3, 6.4, 6.5

**Проблема:** Claude называет категории по-разному: "testing" / "tests" / "test-patterns" → duplicate ralph-*.md files, knowledge fragmentation. Scope hints тоже LLM-зависимы: `*.test.go` vs `*_test.go`.

**Консенсус:**
- **Canonical category list** в Go: `testing, errors, config, cli, architecture, performance, security, misc`
- Go rejects unknown → merge into `misc`
- **Core + extension:** prompt инструктирует "Use core categories when possible. If doesn't fit — use descriptive 1-2 word category"
- **Go-side glob validation:** `filepath.Match` syntax check

---

### H3. HasLearnings bool отсутствует в TemplateData

**Найдена:** Пара 4
**Stories:** 6.2

**Проблема:** Self-review conditioned на non-empty `__LEARNINGS_CONTENT__` — Stage 2 placeholder. Template conditionals = Stage 1 (`{{if .Field}}`). Нет поля `HasLearnings` в TemplateData → conditional невозможен.

**Консенсус:** Добавить `HasLearnings bool` в TemplateData. Runner sets true когда validated content non-empty. Template: `{{- if .HasLearnings}}...self-review section...{{- end}}`.

---

### H4. Circuit breaker "failure" не определена

**Найдена:** Пара 2
**Stories:** 6.5

**Проблема:** "auto-distillation failed 3 times consecutively" — что считается failure?

**Консенсус:** Failure = ANY of:
- `claude -p` non-zero exit code
- ValidateDistillation rejected output
- I/O error при записи файлов
- `claude -p` timeout (>120s default)

---

### H5. "Last 3 sessions" — нет session markers

**Найдена:** Пара 2
**Stories:** 6.5

**Проблема:** ValidateDistillation criterion #3: "Recent entries (last 3 sessions) preserved". Формат `## category: topic [source, file:line]` НЕ содержит session ID или timestamp. Невозможно определить "last 3 sessions".

**Консенсус:** Заменить "last 3 sessions" на "last 20% of entries" (append-only → tail = most recent). Простое, не требует изменения формата.

---

### H6. Output format distillation не специфицирован

**Найдена:** Пары 2, 4
**Stories:** 6.5

**Проблема:** Claude может вывести свободный текст с preamble, Go parser сломается. Parsability не проверяется в ValidateDistillation.

**Консенсус:**
- **BEGIN/END markers:** `BEGIN_DISTILLED_OUTPUT` / `END_DISTILLED_OUTPUT` — Go парсит только между ними
- **ValidateDistillation criterion #8:** "output parseable into categories"
- **Fallback:** если parsing fails → весь output в ralph-misc.md (no split, no data loss)
- **Формат секций:** `## CATEGORY: <name>\n<entries>\n`

---

### H7. "Shared Stage 2 map" не существует

**Найдена:** Пара 4
**Stories:** 6.2

**Проблема:** Tech notes: "All 3 AssemblePrompt call sites get replacements automatically via shared Stage 2 map." Но текущий код имеет РАЗНЫЕ maps на 3 call sites (runner.go:101, :398, :644).

**Консенсус:** Создать `buildKnowledgeReplacements(projectRoot string) (map[string]string, error)` в runner/knowledge.go. Каждый call site merge-ит этот map со своим. Исправить tech notes.

---

### H8. Отсутствие timeout для distillation session

**Найдена:** Пара 4
**Stories:** 6.5

**Проблема:** `claude -p` может зависнуть навечно. Runner заблокирован. Ctrl+C → бэкапы не восстановлены.

**Консенсус:** `context.WithTimeout(ctx, 2*time.Minute)` для distillation. Config field: `distill_timeout: 120`. При timeout → restore backups, CB failure++.

---

## MEDIUM (12 проблем)

### M1. Resume-extraction несовместимо с --resume

**Найдена:** Пары 1, 4
**Stories:** 6.3

Resume и Prompt взаимоисключающие в session.go. Resumed session не знает формат LEARNINGS.md.

**Консенсус (Пара 1):** Отдельная extraction session после resume:
1. Resume session завершается
2. Go запускает отдельную `claude -p` session с extraction prompt
3. Output → `.ralph/pending-lessons.md` → Go quality gates

**Альтернатива (Пара 4):** Добавить knowledge instructions в execute.md → resumed session наследует.

---

### M2. Mutation Asymmetry — review пишет за пределами scope

**Найдена:** Пара 1
**Stories:** 6.3, 6.4

Архитектурный принцип: review sessions ONLY modify sprint-tasks.md и review-findings.md. Story 6.4 расширяет scope без документации.

**Консенсус:** При pending-file pattern смягчается (transient file), но нужно обновить Prompt Invariants в review.md и execute.md.

---

### M3. LessonsData struct слишком плоский

**Найдена:** Пара 1
**Stories:** 6.1

`LessonsData{Source, Content string}` — один Content string не позволяет per-entry validation.

**Консенсус:** Structured entries:
```
LessonEntry{Category, Topic, Content, Citation string}
LessonsData{Source string, Entries []LessonEntry}
```
Каждый entry проходит gates отдельно. 3 valid + 2 invalid = 3 written + 2 rejected.

---

### M4. Scope hints detection — алгоритм не определён

**Найдена:** Пары 3, 4
**Stories:** 6.5, 6.8

Как Go определяет file types? Что с monorepo? Маппинг extension → glob?

**Консенсус:**
1. Walk top 2 levels of project root
2. Collect unique file extensions
3. Map to known language globs (table в Go code)
4. Monorepo: ALL detected → combined scope hints
5. Unknown → no scope hint (catch-all ralph-misc.md)

---

### M5. CodeIndexer over-engineering для v1

**Найдена:** Пара 3
**Stories:** 6.7

При MCP-based подходе Go не вызывает indexer напрямую. Full interface = мёртвый код.

**Консенсус:** Minimal interface:
```
CodeIndexerDetector{Available(projectRoot) bool, PromptHint() string}
```
Полный CodeIndexer с Index()/Query() — Growth phase.

---

### M6. Injection CB threshold (3x) не обоснован

**Найдена:** Пара 3
**Stories:** 6.2

3x = 600 lines при 200-line budget = ~6% context < 10% degradation threshold. Допустим, но нужен named constant + config field.

---

### M7. Crash recovery — .bak не восстанавливаются при startup

**Найдена:** Пара 4
**Stories:** 6.5

При crash во время distillation .bak файлы существуют, но никто не восстанавливает.

**Консенсус:** При старте runner.Run(): проверить .bak → restore → log warning.

---

### M8. ValidateDistillation не проверяет YAML frontmatter

**Найдена:** Пара 4
**Stories:** 6.5

Невалидный YAML в ralph-*.md → файлы не загружаются Claude Code. Distillation считается успешной.

**Консенсус:** Criterion #9: "All ralph-*.md have valid YAML frontmatter with globs: field."

---

### M9. JIT validation line range = O(N * file_read)

**Найдена:** Пара 4
**Stories:** 6.2

50 entries × read = 250-500ms на WSL/NTFS. Tech notes заявляют "~1ms".

**Консенсус:** Только os.Stat (file existence), не line range. Line range — Growth phase.

---

### M10. CB OPEN — нет эскалации, пользователь не видит

**Найдена:** Пары 2, 4
**Stories:** 6.5

Лог "circuit breaker OPEN" в log, не в stderr.

**Консенсус:**
- Stderr warning при CB OPEN: "knowledge distillation blocked — consider `ralph distill`"
- Time-based half-open: `min(10 tasks, 72 hours)` — не ждать бесконечно

---

### M11. freq:N ненадёжен при LLM counting

**Найдена:** Пара 2
**Stories:** 6.5

LLMs плохо инкрементируют числа. При merge [freq:3] + [freq:2] Claude может не сложить.

**Консенсус:** Go-side: parse freq:N в output, проверить N ≥ N в input (monotonic). Go handles increments при WriteLessons (dedup merge counts).

---

### M12. Cross-language test scenarios отсутствуют

**Найдена:** Пара 3
**Stories:** 6.8

Все тестовые примеры в AC подразумевают Go-проект. bmad-ralph — универсальный CLI.

**Консенсус:** Минимум 2 языковых сценария: Go + один non-Go (Python или JS/TS). Тестировать scope hints, citations, categories для non-Go проектов.

---

## LOW (6 проблем)

### L1. Arbitrary G5=5 и G6=10 constants

**Найдена:** Пара 1. **Fix:** Named constants. G6: 10→20 chars.

### L2. Thread safety — unnecessary mutex

**Найдена:** Пара 1. **Fix:** YAGNI. Текущая архитектура sequential. Defer.

### L3. Reverse read ordering — underspecified

**Найдена:** Пара 1. **Fix:** "Split by `\n## `, reverse section order, rejoin."

### L4. Backup single-generation + Model Collapse risk

**Найдена:** Пара 2. **Fix:** 2-generation backups (.bak + .bak.1). [ANCHOR] entries.

### L5. ralph-misc.md может стать монолитом

**Найдена:** Пара 2. **Fix:** ralph-misc.md НЕ получает `globs: ["**"]`. Loaded только через prompt injection.

### L6. Concurrent ralph distill + ralph run race condition

**Найдена:** Пара 2. **Fix:** Advisory note в help. File lock = LOW priority.

---

## Что подтверждено как ПРАВИЛЬНОЕ (все 5 пар сходятся)

Эти решения **не нужно менять**:

1. **3-layer distillation** (Go dedup → LLM compression → validation) — подтверждён 3 research reports + competitive analysis
2. **Circuit breaker pattern** для LLM distillation — правильный (с исправлениями H1/H4)
3. **No FIFO / no archive** при 200-300 lines — правильное решение с injection CB как safety net (C2 fix)
4. **Progressive disclosure T1/T2/T3** — proven pattern (R2-R6)
5. **JIT citation validation** — вдохновлён Copilot (+7% merge rate)
6. **Backup перед distillation** — critical safety net
7. **Multi-file ralph-{category}.md** — правильная архитектура
8. **File-based injection** — оптимален для <500 entries (R3: 74.0% LoCoMo)
9. **Self-review step** — подтверждён Live-SWE-agent (+12% quality)
10. **`ralph distill` как manual escape hatch** — правильный fallback
11. **Tiered memory (hot/distilled/promoted)** — convergent evolution с MemGPT, MemOS, Copilot

---

## Конкурентная позиция (Пара 5)

Сравнение с GitHub Copilot, Cursor, Aider, Cline, Continue, SWE-agent по 6 dimensions:

| Dimension | bmad-ralph | Позиция |
|-----------|-----------|---------|
| Knowledge Extraction | Auto-extraction из 3 источников (review, resume, session) | **ЛИДИРУЕТ** — все конкуренты manual only |
| Knowledge Distillation | 3-layer pipeline (Go dedup → LLM compress → validate) | **ЛИДИРУЕТ (уникально)** — ни у кого нет |
| Knowledge Injection | File-based + JIT validation + self-review + injection CB | **ON PAR** с Copilot/Cursor |
| Quality Gates | 6 gates на запись + 7 criteria distillation = 13 checkpoints | **ЛИДИРУЕТ** — ни у кого нет программных gates |
| Violation Tracking | freq:N + T1 promotion + escalation thresholds | **ЛИДИРУЕТ (уникально)** — closed-loop |
| Progressive Disclosure | T1/T2/T3 tiering через glob-scoped files | **ON PAR** с Continue (RAG) |

**Уникальные преимущества (трудно скопировать):**
- 3-layer distillation pipeline — архитектурная инновация
- Quality gates на content — deep integration с write path
- Violation tracking → T1 promotion — closed-loop, value accumulates
- Circuit breaker для LLM operations — infrastructure pattern

**Вердикт:** Ship as-is после CRITICAL + HIGH fixes. Никаких фундаментальных изменений на основе competitive analysis.

---

## Матрица: Severity × Stories

| Story | CRITICAL | HIGH | MEDIUM | LOW | Total |
|-------|----------|------|--------|-----|-------|
| 6.1 | C1, C4 | H2 | M3 | L1, L2 | 6 |
| 6.2 | C2 | H3, H5, H7 | M6, M9 | L3 | 7 |
| 6.3 | C1 | | M1, M2 | | 3 |
| 6.4 | C1, C4 | | M2 | | 3 |
| 6.5 | C2, C4 | H1, H2, H4, H6, H8 | M4, M7, M8, M10, M11 | L4, L5 | 13 |
| 6.6 | | | | L6 | 1 |
| 6.7 | C3 | | M5 | | 2 |
| 6.8 | | | M4, M12 | | 2 |

**Story 6.5 — наибольшая концентрация проблем (13).** Это ожидаемо: auto-distillation — самый сложный subsystem.

---

## Рекомендуемый порядок исправлений

### Блок 1: Корневые проблемы (решают каскад)
1. **C1** (write path) → автоматически решает C4, M2, L2
2. **C3** (Serena) → полная переработка Story 6.7
3. **C2** (injection CB) → вместе с H1 (counter)

### Блок 2: HIGH-priority AC amendments
4. **H2** (canonical categories) + **H6** (BEGIN/END markers) — связаны (distillation output)
5. **H3** (HasLearnings bool)
6. **H4** (failure definition)
7. **H5** (last 20% вместо last 3 sessions)
8. **H7** (buildKnowledgeReplacements)
9. **H8** (timeout)

### Блок 3: MEDIUM при реализации
10. M1-M12 — при реализации соответствующих stories

### Блок 4: LOW — при наличии времени или Growth phase
11. L1-L6

---

## Индекс отчётов агентов

Все отчёты хранятся в `docs/research/` и `docs/reviews/`:

| Файл | Содержание |
|------|-----------|
| `docs/research/epic6-extraction-pipeline-review.md` | Пара 1: 12 findings (1C, 2H, 4M, 3L, 2 deferred) |
| `docs/research/review-6.5-6.6-consensus.md` | Пара 2: 14 findings (1C, 4H, 7M, 2L) |
| `docs/research/review-6.5-6.6-architect-analysis.md` | Пара 2: architect's independent analysis |
| `docs/research/epic6-injection-universality-review.md` | Пара 3: 7 findings (1C, 3H, 3M) |
| `docs/reviews/epic-6-e2e-algorithm-review.md` | Пара 4: 16 findings (2C, 5H, 7M, 2L) |
| `docs/research/epic6-competitive-analysis.md` | Пара 5: competitive positioning across 6 dimensions |
| **`docs/research/epic6-consolidated-review.md`** | **Этот файл: дедуплицированная сводка всех 5 пар** |

---

*Документ подготовлен на основе consensus reports от 10 агентов (5 пар). Дедупликация: ~50 raw findings → 28 уникальных (4C + 8H + 12M + 6L = 30, минус 2 автоматически решаемых = 28 требующих действий).*
