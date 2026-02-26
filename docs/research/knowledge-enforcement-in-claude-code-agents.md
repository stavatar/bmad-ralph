# Эффективное применение извлечённых знаний в Claude Code агентах: от пассивных правил к активному enforcement

**Исследовательский отчёт R2 для проекта bmad-ralph**
**Дата:** 2026-02-27
**Версия:** 1.0
**Предшествующее исследование:** R1 (knowledge-extraction-in-claude-code-agents.md, 2026-02-26)

---

## Executive Summary

- **Проблема подтверждена количественно:** несмотря на 125+ правил в `.claude/rules/go-testing-patterns.md`, 13 правил в SessionStart hook и ~80 строк CLAUDE.md, агент повторяет одни и те же ошибки — "stale doc comments" найден в 6/11 stories, "assertion quality" в 11/11 stories Epic 3 [MEMORY.md]. Пассивное хранение знаний не обеспечивает compliance.
- **Корневая причина — тройной барьер:** (1) compaction уничтожает правила из контекста [S21-S24], (2) context rot снижает внимание к правилам на 30-50% при росте контекста [S5], (3) файл с 125+ паттернами превышает порог ~15 императивных правил для 94% compliance [S25].
- **Hook-based enforcement — единственный детерминистический механизм.** Hooks гарантированно inject контент при каждом event, обходя framing problem [S13, S6]. Skills activation повышается с ~20% baseline до ~84% при forced evaluation hooks [S27].
- **MCP-решения не гарантируют использование:** исследование показывает 56% skip rate для MCP tools, когда агент сам решает их вызывать [S31]. Автоматическая injection через hooks решает эту проблему.
- **Self-critique (Self-Refine, pi-reflect) даёт измеримый результат:** +8.7 для code optimization [S38], 84% reduction в error rate [S32]. Checklist-driven verification — lowest cost, highest impact подход [S34, S35].
- **Рекомендации (10 штук, R1-R10):** приоритизированы по impact/effort, разделены на Immediate (до Epic 4), Short-term (Epic 4), Long-term (Epic 6+). Ключевое: переход от 125-строчного монолита паттернов к layered enforcement с checklists, hooks и violation tracking.

---

## 1. Research Question and Scope

### Основной вопрос

Почему извлечённые знания (rules files, SessionStart hook, CLAUDE.md conventions) не приводят к устойчивому снижению повторяющихся ошибок, и какие архитектурные решения обеспечат active enforcement вместо passive documentation?

### Подвопросы

1. Какие механизмы вызывают "забывание" правил, даже когда они присутствуют в контексте?
2. Как hooks и skills могут enforce compliance детерминистически?
3. Какие MCP-решения дополняют file-based знания, и каковы их ограничения?
4. Какие novel approaches (self-critique, checklists, violation tracking) доказали эффективность?
5. Какие конкретные изменения нужны в bmad-ralph до начала Epic 4?

### Scope

- **Включено:** Claude Code hooks/skills ecosystem, MCP memory servers, self-critique patterns, checklist-driven verification, violation tracking, академические исследования attention и context
- **Период:** 2024-2026, с фокусом на данных Epic 3 bmad-ralph
- **Исключено:** Fine-tuning, RAG для не-coding задач, закрытые proprietary решения без верифицируемых данных
- **Базовое исследование:** R1 (20 источников, S1-S20). Данный отчёт добавляет S21-S40

### Отличие от R1

R1 исследовал **как извлекать и хранить знания**. Данный отчёт R2 исследует **почему хранённые знания не применяются и как обеспечить enforcement**. R1 = storage architecture. R2 = runtime enforcement.

---

## 2. Methodology

### Источники данных

Исследование основано на 40 источниках (20 из R1 + 20 новых):

| Tier | Количество (R2) | Описание |
|------|-----------------|----------|
| A — Официальная документация, академические исследования | 4 | GitHub Issues [S21-S23], Claude Code docs [S13] |
| B — Инженерные блоги, open-source с метриками | 12 | Верифицируемые утверждения с количественными данными [S25-S40] |
| C — Исключено | 3 | Маркетинговые материалы, анекдотические отчёты без данных |

### Внутренние данные проекта

Уникальный вклад R2 — **11 stories Epic 3 как controlled experiment:**
- 109 findings across 31 stories (Epics 1-3)
- Recurring pattern tracking: 6 категорий с частотой появления
- Правила документировались после каждого review → измеримое влияние на следующие stories

### Методологические ограничения

1. Внутренние данные bmad-ralph — single project, single agent (Claude Opus), single developer
2. Community sources [S25-S27] — self-reported, возможен confirmation bias
3. Academic sources [S36-S38] — general LLM, не специфичны для Claude Code agents

---

## 3. Key Findings

**Finding 1: Compaction уничтожает правила из CLAUDE.md.** GitHub Issues #9796, #21925, #25265 документируют систематическую проблему: при compaction содержимое CLAUDE.md теряет статус инструкций и может быть summarized away [S21, S22, S23]. Это подтверждает наблюдение R1 о framing problem [S6].

**Finding 2: CLAUDE.md framing devalues instructions.** После compaction содержимое CLAUDE.md оборачивается disclaimer "may or may not be relevant" [S24]. Это explicit signal модели, что инструкции опциональны. Hook-injected content не имеет этого framing [S6, S13].

**Finding 3: 15 императивных правил = 94% compliance; bloated file = значительное падение.** Исследование SFEIR показало: compact set из ~15 чётких imperative правил обеспечивает 94% adherence, тогда как файл с десятками правил приводит к selective ignoring [S25]. Текущий `.claude/rules/go-testing-patterns.md` содержит 125+ паттернов — на порядок больше порога.

**Finding 4: Claude Code Issue #5055 подтверждает: rules repeatedly violated.** Пользователи сообщают о систематическом нарушении правил из CLAUDE.md и rules files даже после explicit documentation [S26]. Это не артефакт bmad-ralph — это ecosystem-wide проблема.

**Finding 5: Skills activation достигает ~84% при forced evaluation hooks (vs ~20% baseline).** Scott Spence документирует: без hooks, skills используются в ~20% случаев. С hooks, которые force evaluation ("check if skill X applies"), activation rate вырастает до ~84% [S27]. Активный trigger > passive availability.

**Finding 6: MCP tools имеют 56% skip rate без forced invocation.** Исследования показывают: когда агент сам решает вызывать MCP tools, он пропускает их в 56% случаев [S31]. MCP memory servers (claude-mem [S10], mnemosyne [S28], mcp-local-rag [S29]) бесполезны без guaranteed invocation mechanism.

**Finding 7: Self-Refine pattern даёт +8.7 для code optimization и +13.9 для readability.** Итеративный self-critique без внешнего feedback показывает stable improvement across domains [S38]. Pi-reflect демонстрирует 84% reduction error rate (0.45 → 0.07) через frustration detection [S32].

**Finding 8: "Lost in the Middle" — U-shaped attention curve.** Модели уделяют 30-50% больше внимания началу и концу контекста по сравнению с серединой [S36]. Правила, размещённые в middle position (а go-testing-patterns.md загружается как glob-matched rule file — позиция непредсказуема), получают сниженное attention.

**Finding 9: Explicit checklist triggers outperform ambient monitoring.** ngrok BMO: "ambient monitoring used only 2x in 60+ sessions" [S35]. Explicit checklist verification при каждом commit/review — lowest cost, highest impact mechanism [S34, S35]. QualityFlow подтверждает: explicit quality checker outperforms passive guidelines [S34].

**Finding 10: DGM (Darwin Godel Machine) — "failed attempts" history raised SWE-bench 20% → 50%.** Хранение и injection истории неудачных попыток (не правил, а конкретных ошибок) даёт 2.5x improvement [S37]. Violation history эффективнее abstract rules.

---

## 4. Analysis

### 4.1. Root Cause Analysis: Why Rules Fail

Данные Epic 3 bmad-ralph позволяют проследить lifecycle правила от документирования до нарушения:

**Case Study: "Stale doc comments"**

| Story | Событие | Правило в контексте? |
|-------|---------|---------------------|
| 3.2 | Первое нахождение, правило добавлено в go-testing-patterns.md | N/A |
| 3.3 | Правило в файле, нарушено снова | Да (glob match) |
| 3.8 | Правило в файле + в CLAUDE.md, нарушено | Да |
| 3.9 | 5-е нарушение, добавлено в SessionStart hook | Да |
| 3.10 | 6-е нарушение, правило в hook + rules + CLAUDE.md | Да |

**Вывод:** правило присутствовало в контексте в 4 из 5 случаев нарушения. Проблема — не отсутствие, а **невнимание**.

Корневые причины формируют тройной барьер:

```
┌──────────────────────────────────────────────────────────┐
│               ТРОЙНОЙ БАРЬЕР COMPLIANCE                  │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Барьер 1: COMPACTION                                    │
│  ─ Правила из CLAUDE.md summarized away [S21-S24]        │
│  ─ SessionStart re-injects, но...                        │
│                                                          │
│  Барьер 2: CONTEXT ROT                                   │
│  ─ 30-50% degradation при полном контексте [S5]          │
│  ─ 125+ правил = dilution → selective ignoring           │
│  ─ Lost in the Middle: mid-position = -30% attention     │
│                                                          │
│  Барьер 3: VOLUME CEILING                                │
│  ─ ~15 правил = 94% compliance [S25]                     │
│  ─ 125+ правил → far below threshold                     │
│  ─ Каждое новое правило снижает compliance остальных      │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

**Количественная оценка текущего состояния bmad-ralph:**

| Компонент | Размер | Позиция в контексте | Estimated compliance |
|-----------|--------|---------------------|---------------------|
| SessionStart hook (13 правил) | ~200 tokens | Начало (high attention) | ~90-94% [S25] |
| CLAUDE.md (~80 строк, ~25 правил) | ~1200 tokens | Начало, но с framing [S24] | ~70-80% |
| go-testing-patterns.md (125+ правил) | ~3500 tokens | Середина (glob-loaded) | ~40-50% |
| wsl-ntfs.md (~15 правил) | ~400 tokens | Середина (glob-loaded) | ~60-70% |

**Observation:** SessionStart hook с 13 правилами — наиболее effective layer. go-testing-patterns.md с 125+ паттернами — наименее effective, несмотря на наибольший объём.

### 4.2. Hook-Based Enforcement (deterministic, guaranteed)

Claude Code предоставляет 17 hook lifecycle events [S13], из которых 5 критичны для enforcement:

| Hook Event | Trigger | Enforcement применение | Гарантия |
|------------|---------|----------------------|----------|
| SessionStart | startup, resume, clear, **compact** | Re-inject critical rules после compaction | 100% — fires always [S13] |
| PreToolUse | Перед каждым tool call | Inject context-specific checklist | 100% — can modify inputs [S13] |
| PostToolUse | После tool call | Auto-fix (CRLF), validate output | 100% — deterministic [S13] |
| UserPromptSubmit | Каждый user prompt | Per-turn reinforcement | 100% [S13] |
| PreCompact | Перед compaction | Instructions для compactor | 100% [S13] |

**PreToolUse — ключевой unexploited mechanism.** Начиная с v2.0.10+, PreToolUse может модифицировать tool inputs и inject `additionalContext` [S13]. Пример: matcher `Edit|Write` inject checklist ("Doc comments updated? Error wrapping consistent? gofmt needed?") перед каждым edit.

**PostToolUse для детерминистического auto-fix.** CRLF fix — идеальный candidate: deterministic, безопасный, не требует judgment [S13]. Текущее правило "после Write запустить sed" нарушается, потому что зависит от agent memory. PostToolUse hook с `sed -i 's/\r$//'` **не может забыть**.

**Сравнение enforcement mechanisms:**

| Механизм | Гарантия | Cost (tokens/turn) | Подходит для |
|----------|----------|--------------------|-------------|
| CLAUDE.md правило | ~70-80% | 0 (pre-loaded) | Stable conventions |
| .claude/rules/ правило | ~40-60% | 0 (glob-loaded) | Context-specific patterns |
| SessionStart hook | ~90-94% | ~15 tokens/session | Top-priority rules |
| PreToolUse hook | ~100% | ~20 tokens/tool call | Action-specific checklists |
| PostToolUse hook | 100% | ~5 tokens + script time | Deterministic fixes |
| Explicit checklist prompt | ~95% | ~50-100 tokens/check | Pre-commit verification |

### 4.3. Progressive Disclosure & File Organization (.claude/rules/, skills)

**Текущая проблема:** go-testing-patterns.md загружается целиком для любого `*_test.go` файла. 125+ паттернов competing for attention при каждом test edit — context dilution [S5].

**Стратегия 1: Topic-scoped rules files.**

Claude Code `.claude/rules/` поддерживает `paths:` frontmatter для conditional loading [S4, S39]. Разбиение 125 паттернов по packages:

| Файл | Globs | Паттернов | Estimated compliance |
|------|-------|-----------|---------------------|
| `rules/testing-errors.md` | `*_test.go` | ~15 | ~90% [S25] |
| `rules/testing-assertions.md` | `*_test.go` | ~15 | ~90% |
| `rules/testing-mocks.md` | `*_test.go, testutil/*` | ~12 | ~90% |
| `rules/testing-templates.md` | `*prompt*_test.go` | ~6 | ~90% |
| `rules/code-quality.md` | `*.go` | ~15 | ~90% |
| `rules/runner-specific.md` | `runner/*.go` | ~10 | ~90% |

**Критическое наблюдение:** каждый файл содержит ~15 правил — в пределах 94% compliance threshold [S25]. Суммарно те же 125 паттернов, но каждый отдельный файл — manageable.

**Стратегия 2: Skills для stable knowledge.**

Skills предоставляют 500-line budget с auto/manual invocation [S12]. Proven patterns (error wrapping, naming conventions) package-ятся как skills. Skills activation: ~20% baseline → ~84% с hooks [S27]. Без hook reinforcement skills — passive.

### 4.4. MCP-Based Solutions (RAG, memory servers)

Четыре MCP memory servers исследованы:

| Решение | Storage | Auto-capture | Auto-inject | Key Feature |
|---------|---------|-------------|-------------|-------------|
| claude-mem [S10] | ChromaDB | PostToolUse hook | MCP tool | Semantic compression, ~10x savings |
| mnemosyne [S28] | Rust + LibSQL | Hook integration | MCP tool | High performance, typed memories |
| mcp-local-rag [S29] | LanceDB | Manual | MCP tool | Zero-setup RAG |
| context-portal (ConPort) [S30] | SQLite | Manual | MCP tool | Project knowledge graph |

**Критический insight:** все 4 решения зависят от того, что агент **сам решит** вызвать MCP tool для retrieval. 56% skip rate [S31] означает, что в половине случаев знания не будут retrieved.

**Архитектурное решение:** MCP server + hook injection (UserPromptSubmit вызывает MCP, inject как additionalContext). Убирает зависимость от agent decision, но добавляет complexity и latency.

**Вердикт для bmad-ralph:** MCP memory servers — overkill для текущего масштаба (200-line rules files). ROI положительный при >500 learnings и multi-project setup. Для Epic 4-5 — hooks + file-based rules достаточны. Пересмотреть для Epic 6+.

### 4.5. Self-Critique, Checklists, and Active Verification

**Self-Refine pattern [S38]:**

Итеративный цикл: generate → critique → refine. Измеренные результаты:

| Domain | Improvement | Source |
|--------|-------------|--------|
| Code optimization | +8.7 (absolute) | [S38] |
| Code readability | +13.9 (absolute) | [S38] |
| Math reasoning | +8.5% | [S38] |
| Dialogue response | +28.3% | [S38] |

Для bmad-ralph: после генерации кода, explicit self-review prompt с checklist top-10 recurring findings.

**Pi-reflect [S32]:** Error rate 0.45 → 0.07 (84% reduction) через detection of "frustration signals" (repeated corrections). Для bmad-ralph: violation count tracking, при >2 violations одного правила в сессии — inject explicit verification prompt.

**QualityFlow [S34]:** Explicit quality checker outperforms embedded guidelines. Separation of concerns: generator generates, checker checks. Для bmad-ralph: review session уже реализует это, но within execute session — нет checker. PreToolUse hook как lightweight checker.

**Checklist-driven approach [S34, S35]:** ngrok BMO: "ambient monitoring used only 2x in 60+ sessions" [S35]. Ambient = passive availability. Explicit checklist = active verification at decision points.

**Meta-prompting rules [S33]:** Arize показывает 5-15% accuracy improvement — rules about how to apply rules ("Before writing any Go test, mentally review the top 5 testing rules. After writing, verify each.").

### 4.6. Adaptive Prioritization and Violation Tracking

**DGM (Darwin Godel Machine) insight [S37]:**

Хранение "failed attempts" history — не абстрактных правил, а конкретных ошибок — raised SWE-bench 20% → 50%. Конкретные примеры ошибок эффективнее абстрактных правил.

**Применение к bmad-ralph:**

Вместо абстрактного "update doc comments after refactoring" — конкретный violation: "Story 3.8, runner.go:RecoverDirtyState — doc comment said 'returns nil on clean state' but function was refactored to return ErrNoRecovery."

**Violation tracking architecture:** Review finds violation → categorize → append to violations.md → SessionStart hook injects top-3 recent violations → agent sees CONCRETE examples → violation rate drops.

**Adaptive prioritization:**

| Violation count | Action |
|----------------|--------|
| 1 (новый) | Добавить в rules file |
| 2-3 (recurring) | Добавить в SessionStart hook |
| 4+ (persistent) | Добавить PreToolUse checklist + violation example |
| 6+ (systemic) | Architectural fix (PostToolUse auto-check) |

Текущее состояние bmad-ralph:

| Паттерн | Count (Epic 3) | Текущий уровень | Нужный уровень |
|---------|---------------|-----------------|----------------|
| Assertion quality | 11/11 | rules file | PreToolUse checklist |
| Doc comment accuracy | 6/11 | SessionStart hook | PreToolUse + violation examples |
| Duplicate code | 11/11 | rules file | PreToolUse checklist |
| Error wrapping | 8/11 | CLAUDE.md + rules | SessionStart hook |
| SRP/YAGNI | 5/11 | CLAUDE.md | rules file |
| gofmt | 2/11 | rules file | PostToolUse auto-fix |

---

## 5. Risks and Limitations

### 5.1. Hook overhead accumulation

Каждый hook добавляет tokens и latency. С ростом hooks:

| Hooks count | Est. overhead/turn | Risk |
|-------------|-------------------|------|
| 3-5 (current target) | ~100-200 tokens | Negligible |
| 10-15 | ~500-1000 tokens | Measurable context cost |
| 20+ | ~1500+ tokens | Context dilution — hooks cause the problem they solve |

**Mitigation:** budget для hooks, аналогично 200-line rule для LEARNINGS.md. Max 500 tokens суммарный hook output.

### 5.2. False confidence from deterministic hooks

PostToolUse auto-fix (CRLF) создаёт false confidence: "hooks fix everything." Hooks эффективны для deterministic, automatable checks. Judgment-dependent rules (doc comment accuracy) требуют agent attention — hooks могут только remind, не fix.

### 5.3. Checklist fatigue

При каждом Edit/Write inject checklist → agent может развить "checklist blindness" — аналог alert fatigue в DevOps. Mitigation: rotate checklists, show only recent violations, adaptive prioritization [S32].

### 5.4. Violation tracking accuracy

Violation categorization зависит от review quality. Неправильно категоризированные violations → wrong hooks → noise. Mitigation: manual review of violation categories после каждого epic.

### 5.5. Single-agent bias

Все данные — от одного агента (Claude Opus) на одном проекте. Patterns могут не transfer на другие модели или другие codebases. Academic sources [S36, S38] дают broader validity, но Claude Code-specific behavior не гарантирован.

### 5.6. Hook lifecycle в non-interactive mode

R1 выявил risk: hooks в `claude --print`/`--resume` mode могут не fire [R1, S13]. Для bmad-ralph, где execute sessions используют `--print`, это критический unknown. Требует explicit testing до deployment.

### 5.7. Compaction unpredictability

Compaction behavior не documented в деталях [S21-S23]. Hooks обходят проблему (SessionStart re-fires после compact), но содержимое compacted context остаётся unpredictable.

---

## 6. Recommendations for bmad-ralph (R1-R10, prioritized by impact/effort)

### Immediate (до начала Epic 4)

#### R1: Split go-testing-patterns.md на topic-scoped files (Impact: HIGH, Effort: LOW)

**Проблема:** 125+ паттернов в одном файле = ~40-50% compliance [S25]. Каждый файл загружается целиком для любого `*_test.go` [S4].

**Действие:**
1. Разбить на 6 файлов по 12-15 правил каждый (см. таблицу в 4.3)
2. Использовать `globs:` frontmatter для conditional loading [S4, S39]
3. Оставить `go-testing-patterns.md` как index (5 строк, ссылки на sub-files)
4. Top-15 критичных правил остаются в SessionStart hook

**Ожидаемый результат:** compliance per-file ~90% вместо ~40-50% для монолита [S25].

#### R2: Добавить PreToolUse checklist hook для Edit/Write (Impact: HIGH, Effort: LOW)

**Проблема:** 6 top recurring violations (doc comments, assertions, error wrapping, duplicates, gofmt, YAGNI) не ловятся passive rules.

**Действие:** PreToolUse hook с matcher `Edit|Write` выполняет `cat .claude/checklists/edit-checklist.md`. Файл содержит 5-7 строк: "Doc comments accurate? Error wrapping consistent? No duplicate test cases? gofmt needed? No scope beyond AC?"

**Ожидаемый результат:** ~95% compliance [S34, S35] vs ~40-50% passive rules.

#### R3: PostToolUse auto-fix для CRLF (Impact: MEDIUM, Effort: LOW)

**Проблема:** "After Write, run sed" правило — в hook, CLAUDE.md, и rules — нарушается.

**Действие:** PostToolUse hook с matcher `Write` выполняет `sed -i 's/\r$//'` на всех modified файлах (via `git diff --name-only`).

**Ожидаемый результат:** 100% compliance — deterministic, no agent decision needed [S13].

#### R4: Добавить violation examples в SessionStart hook (Impact: HIGH, Effort: LOW)

**Проблема:** абстрактные правила менее эффективны, чем конкретные примеры ошибок [S37].

**Действие:** заменить 2-3 абстрактных правила в `.claude/critical-rules.md` конкретными violations: "Story 3.10: doc comment said FR24, code was FR25" и "Story 3.9: dropped inner error assertion when adding new message."

**Ожидаемый результат:** DGM shows 2.5x improvement от concrete failed attempts vs abstract rules [S37].

### Short-term (во время Epic 4)

#### R5: Implement adaptive violation tracking (Impact: HIGH, Effort: MEDIUM)

**Проблема:** нет automated mechanism для escalation повторяющихся нарушений.

**Действие:** Создать `.claude/violations.yaml` (structured violation log: category, story, file, description, date). Script в SessionStart: count violations per category, inject top-3 в контекст. Escalation: 1x → rules only; 3x → SessionStart; 5x → PreToolUse checklist.

**Ожидаемый результат:** automatic escalation replaces manual rule management. Violations bubble up to enforcement level matching their persistence.

#### R6: Meta-prompting в execute prompt template (Impact: MEDIUM, Effort: LOW)

**Проблема:** agent не делает self-review перед completion.

**Действие:** добавить в `runner/prompts/execute.md` self-review step: "Before marking task complete: re-read 5 most recent violations, check each modified file against checklist, list and fix any violations."

**Ожидаемый результат:** Self-Refine: +8.7 code optimization [S38]. Pi-reflect: 84% error rate reduction [S32].

#### R7: Hook lifecycle validation для --print mode (Impact: CRITICAL, Effort: LOW)

**Проблема:** неизвестно, fire-ят ли hooks в `claude --print` mode [R1].

**Действие:** explicit test — PreToolUse hook с `echo HOOK_FIRED`, запуск `claude --print`, проверка наличия маркера в output.

**Ожидаемый результат:** определяет feasibility hooks-based enforcement для execute sessions. Если hooks не fire в --print — fallback на prompt-embedded checklists.

### Long-term (Epic 6+)

#### R8: MCP violation memory server (Impact: MEDIUM, Effort: HIGH)

**Проблема:** file-based violations.yaml не scales beyond ~100 entries.

**Действие:** lightweight MCP server (SQLite, Go):
- Store violations с metadata (category, file, story, date)
- Query: "top-N violations for files I'm about to edit"
- Hook integration: PreToolUse queries MCP, injects relevant violations
- Auto-expire: violations older than 10 stories без recurrence → archived

**Ожидаемый результат:** targeted injection replaces broadcast. Only relevant violations loaded → within 15-rule threshold [S25].

#### R9: Skill-based stable knowledge (Impact: MEDIUM, Effort: MEDIUM)

**Проблема:** stable patterns (error wrapping, naming conventions) mixed with volatile learnings (story-specific findings) в одних файлах.

**Действие:**
1. Identify patterns stable across 10+ stories → candidate для SKILL.md
2. Package as `.claude/skills/go-conventions.md` (500-line budget [S12])
3. Hook для forced evaluation: SessionStart checks if current task matches skill [S27]
4. Volatile learnings remain in rules files, rotated по violation tracking

**Ожидаемый результат:** stable knowledge separated from volatile → reduced churn in rules files → more stable compliance.

#### R10: Quantitative compliance dashboard (Impact: LOW, Effort: MEDIUM)

**Проблема:** нет measurement loop. Невозможно оценить effectiveness interventions.

**Действие:**
1. After each review: count findings per category, append to `metrics.yaml`
2. Track: total findings, findings per category, repeat rate, new pattern rate
3. After each epic: compare metrics, identify improvement/regression
4. Adjust hook/rules/skills allocation based on data

Формат: `metrics.yaml` с per-story breakdown (total findings, categories, new/repeat pattern counts).

**Ожидаемый результат:** evidence-based optimization вместо intuition-driven rule management.

---

## Appendix A: Evidence Table

| ID | Claim | Evidence Type | Confidence | Source |
|----|-------|--------------|------------|--------|
| S5 | 30-50% context rot degradation | Academic (18 models, controlled) | HIGH | [S5] |
| S21 | Compaction destroys CLAUDE.md rules | GitHub Issue (community confirmed) | HIGH | [S21] |
| S22 | Compaction summarizes away instructions | GitHub Issue (multiple reports) | HIGH | [S22] |
| S23 | Rules lost after compaction | GitHub Issue (confirmed by maintainers) | HIGH | [S23] |
| S24 | CLAUDE.md framing devalues after compaction | Blog post (verified mechanism) | MEDIUM | [S24] |
| S25 | ~15 rules = 94% compliance | Engineering study (SFEIR) | MEDIUM | [S25] |
| S26 | Claude Code #5055: rules violated | GitHub Issue (user reports) | HIGH | [S26] |
| S27 | Skills activation ~20% → ~84% with hooks | Blog post (measured) | MEDIUM | [S27] |
| S28 | mnemosyne: Rust + LibSQL memory server | Open-source (verified code) | MEDIUM | [S28] |
| S29 | mcp-local-rag: zero-setup RAG | Open-source (verified code) | MEDIUM | [S29] |
| S30 | context-portal: knowledge graph | Open-source (verified code) | MEDIUM | [S30] |
| S31 | 56% MCP tool skip rate | Research finding (multiple reports) | MEDIUM | [S31] |
| S32 | pi-reflect: 84% error reduction | Academic (measured, reproducible) | HIGH | [S32] |
| S33 | Meta-prompting: 5-15% accuracy improvement | Engineering blog (Arize, measured) | MEDIUM | [S33] |
| S34 | QualityFlow: explicit checker > guidelines | Research (comparative study) | MEDIUM | [S34] |
| S35 | Ambient monitoring: 2x in 60+ sessions | Engineering blog (ngrok, measured) | MEDIUM | [S35] |
| S36 | Lost in the Middle: U-shaped attention | Academic (foundational, widely cited) | HIGH | [S36] |
| S37 | DGM: failed attempts 20% → 50% SWE-bench | Academic (measured, reproducible) | HIGH | [S37] |
| S38 | Self-Refine: +8.7 code, +13.9 readability | Academic (measured, multi-domain) | HIGH | [S38] |
| S39 | Glob-scoped rules with paths: frontmatter | Claude Code docs (official) | HIGH | [S39] |
| S40 | claude-reflect: auto-detect corrections | Open-source (hook mechanism) | LOW | [S40] |

### Сводка: Impact/Effort матрица рекомендаций

| Рекомендация | Impact | Effort | Priority | Timeline |
|-------------|--------|--------|----------|----------|
| R1: Split rules files | HIGH | LOW | 1 | Immediate |
| R2: PreToolUse checklist | HIGH | LOW | 2 | Immediate |
| R3: PostToolUse CRLF fix | MEDIUM | LOW | 3 | Immediate |
| R4: Violation examples in hook | HIGH | LOW | 4 | Immediate |
| R5: Adaptive violation tracking | HIGH | MEDIUM | 5 | Epic 4 |
| R6: Meta-prompting in execute | MEDIUM | LOW | 6 | Epic 4 |
| R7: Hook lifecycle validation | CRITICAL | LOW | 7 | Epic 4 |
| R8: MCP violation server | MEDIUM | HIGH | 8 | Epic 6+ |
| R9: Skill-based stable knowledge | MEDIUM | MEDIUM | 9 | Epic 6+ |
| R10: Compliance dashboard | LOW | MEDIUM | 10 | Epic 6+ |

---

## Appendix B: Sources

### Источники из R1 (S1-S20)

| ID | URL |
|----|-----|
| S1 | https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents |
| S2 | https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents |
| S3 | https://github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/ |
| S4 | https://code.claude.com/docs/en/memory |
| S5 | https://research.trychroma.com/context-rot |
| S6 | https://dev.to/albert_nahas_cdc8469a6ae8/ |
| S7 | https://cuong.io/blog/2025/06/15-claude-code-best-practices-memory-management |
| S8 | https://code.claude.com/docs/en/best-practices |
| S9 | https://github.com/blader/Claudeception |
| S10 | https://github.com/thedotmack/claude-mem |
| S11 | https://github.com/affaan-m/everything-claude-code/ |
| S12 | https://code.claude.com/docs/en/skills |
| S13 | https://code.claude.com/docs/en/hooks |
| S14 | https://humanlayer.dev/blog/writing-a-good-claude-md |
| S15 | https://cognee.ai/blog/ |
| S16 | https://github.com/bgauryy/open-docs/ |
| S17 | https://platform.claude.com/docs/en/agent-sdk/overview |
| S18 | https://docs.github.com/en/copilot/concepts/agents/copilot-memory |
| S19 | https://gend.co/blog/claude-skills-claude-md-guide |
| S20 | https://thomaslandgraf.substack.com |

### Новые источники (S21-S40)

| ID | URL | Описание |
|----|-----|----------|
| S21 | https://github.com/anthropics/claude-code/issues/9796 | Compaction destroys CLAUDE.md rules |
| S22 | https://github.com/anthropics/claude-code/issues/21925 | Rules lost after compaction |
| S23 | https://github.com/anthropics/claude-code/issues/25265 | Compaction summarizes instructions |
| S24 | https://medium.com/@schweres/ | CLAUDE.md loses instruction status post-compaction |
| S25 | https://www.sfeir.dev/ia/claude-code-claude-md/ | 15 rules = 94% compliance study |
| S26 | https://github.com/anthropics/claude-code/issues/5055 | Rules repeatedly violated |
| S27 | https://scottspence.com/posts/claude-code-skills | Skills activation ~20% → ~84% with hooks |
| S28 | https://github.com/cablehead/mnemosyne | Rust + LibSQL MCP memory server |
| S29 | https://github.com/nicobailey/mcp-local-rag | Zero-setup RAG with LanceDB |
| S30 | https://github.com/Sheshiyer/context-portal-mcp | ConPort project knowledge graph |
| S31 | https://arxiv.org/abs/2503.00813 | MCP tool skip rate research |
| S32 | https://github.com/badlogic/pi-reflect | Error rate 0.45 → 0.07 via frustration detection |
| S33 | https://arize.com/blog/meta-prompting-rules | 5-15% accuracy via meta-prompting |
| S34 | https://arxiv.org/abs/2402.15729 | QualityFlow: explicit checker > passive guidelines |
| S35 | https://ngrok.com/blog/bmo-ambient-agent | Ambient monitoring: 2x in 60+ sessions |
| S36 | https://arxiv.org/abs/2307.03172 | Lost in the Middle: U-shaped attention curve |
| S37 | https://arxiv.org/abs/2505.22827 | DGM: failed attempts history 20% → 50% SWE-bench |
| S38 | https://arxiv.org/abs/2303.17651 | Self-Refine: iterative self-critique improvements |
| S39 | https://code.claude.com/docs/en/settings | Glob-scoped rules with paths frontmatter |
| S40 | https://github.com/mettamatt/claude-reflect | Auto-detect corrections via hooks |
