# Knowledge Extraction in Claude Code-Based Coding Agents: Patterns, Limitations, and Architectural Recommendations

**Исследовательский отчёт для проекта bmad-ralph**
**Дата: 2026-02-25**
**Версия: 2.0**

---

## Executive Summary

- Управление знаниями в coding agents на базе Claude Code является решённой задачей на уровне *хранения* (6 уровней памяти, CLAUDE.md cascade, auto memory) [S4], но остаётся открытой проблемой на уровне *эффективного использования* из-за универсальной деградации моделей при росте контекста [S5].
- Compaction — необходимый, но недостаточный механизм: он сохраняет структуру, но уничтожает конкретные инструкции и learned values [S1][S6]. Единственный надёжный способ обеспечить следование критическим правилам — hook-based enforcement через SessionStart/PreCompact events [S6][S13].
- GitHub Copilot демонстрирует, что citation-based validation (проверка памяти против актуального кода) повышает merge rate на 7% и обеспечивает self-healing при устаревших записях [S3][S18] — паттерн, применимый к bmad-ralph через интеграцию knowledge validation с git state.
- Context rot — универсальная проблема всех 18 протестированных frontier models: 30-50%+ потеря производительности при полном контексте vs компактном [S5]. Для bmad-ralph это означает жёсткий budget в 200 строк LEARNINGS.md — не произвольное ограничение, а инженерная необходимость.
- Рекомендации для Epic 6: topic-based sharding LEARNINGS.md, hook-based injection критических правил (обход CLAUDE.md framing), citation validation при загрузке знаний, distillation через `claude -p` с backup — и обязательный мониторинг feedback loop (больше знаний -> больше контекста -> больше ошибок).

---

## 1. Research Question and Scope

### Основной вопрос

Как эффективно извлекать, хранить и применять знания в coding agents на базе Claude Code, работающих в multi-session автономном режиме?

### Подвопросы

1. Какие механизмы памяти предоставляет Claude Code, и какие их реальные ограничения?
2. Какие паттерны knowledge extraction показали эффективность в production системах?
3. Как context rot влияет на способность агента использовать accumulated knowledge?
4. Какие архитектурные решения минимизируют потери знаний между сессиями?
5. Какие конкретные рекомендации применимы к bmad-ralph Epic 6?

### Scope

- **Включено:** Claude Code ecosystem (CLI, Agent SDK, hooks), GitHub Copilot memory (для сравнения), third-party инструменты (claude-mem, Claudeception, continuous-learning), академические исследования context rot
- **Период:** 2024-2026
- **Исключено:** Non-LLM memory systems, внутренняя архитектура Cursor/Copilot, fine-tuning подходы

---

## 2. Methodology

Исследование основано на 20 источниках, классифицированных по tier quality:

- **Tier A (10 источников):** Официальная документация Anthropic [S1][S2][S4][S8][S12][S13][S17], GitHub [S3][S18], академические исследования [S5]
- **Tier B (10 источников):** Инженерные блоги с конкретными экспериментами [S6][S7][S9][S10][S11][S14][S15][S16][S19][S20]
- **Tier C (исключены):** Общие AI обзоры без конкретных данных, маркетинговые материалы

Приоритет отдан источникам с измеримыми результатами (Chroma Research [S5], GitHub A/B тесты [S3]) и официальной документации с implementation details [S4][S13].

---

## 3. Key Findings

1. **Claude Code предоставляет 6 уровней памяти** с чётко определённой иерархией и lifecycle loading, но каждый уровень имеет специфические ограничения по объёму и reliability [S4].

2. **Context rot — универсальное явление:** все 18 протестированных frontier models деградируют при росте контекста, с 30-50%+ variance между compact и full context [S5].

3. **~150-200 инструкций — практический предел consistency** для CLAUDE.md [S14]. Превышение этого лимита приводит к selective ignoring, причём модель не сообщает о пропущенных инструкциях.

4. **CLAUDE.md обёрнут в disclaimer** ("may or may not be relevant"), что снижает adherence [S6]. Hooks обходят эту проблему, инжектируя instructions как clean system-reminder [S6][S13].

5. **Compaction уничтожает learned values:** Anthropic подтверждает, что compaction alone insufficient для long-running agents [S1]. Structured notes (NOTES.md, progress files) + git commits — рекомендованный паттерн [S1][S2].

6. **Citation-based validation** в GitHub Copilot memory обеспечивает self-healing: устаревшие memories автоматически корректируются при расхождении с актуальным кодом [S3][S18]. Измеренный эффект: 7% рост PR merge rate (90% vs 83%, p < 0.00001) [S3].

7. **Sub-agent architecture изолирует контексты:** focused sub-agents с 1000-2000 token summaries для lead agent — Anthropic-рекомендованный паттерн для управления context budget [S2].

8. **Hook-based enforcement — единственный reliable механизм** для critical rules: SessionStart hook fires on startup/resume/clear/compact, PreCompact получает custom_instructions [S13]. Cost: ~750 tokens per 50 turns (пренебрежимо мало) [S6].

9. **Third-party tools демонстрируют timing advantage:** extraction *during* session (не после) захватывает контекст, потерянный при compaction [S11]. Progressive disclosure (claude-mem) предотвращает context overload [S10].

10. **Dangerous feedback loop:** больше ошибок -> больше learnings -> больше context -> больше context rot -> больше ошибок. Distillation с hard budget — необходимый circuit breaker [S5][S14].

---

## 4. Analysis

### 4.1 Claude Code Memory Architecture

Claude Code реализует 6-уровневую иерархию памяти с чётко определённым порядком приоритетов и lifecycle [S4]:

| Уровень | Тип | Загрузка | Ограничения |
|---------|-----|----------|-------------|
| 1 | Managed policy | Always, highest priority | Не редактируемый пользователем |
| 2 | Project memory (CLAUDE.md) | Full at launch, child dirs on demand | "may or may not be relevant" framing |
| 3 | Project rules (.claude/rules/) | Glob-matched on demand | Per-file, YAML frontmatter |
| 4 | User memory (~/.claude/CLAUDE.md) | Full at launch | Shared across all projects |
| 5 | Project memory local (CLAUDE.local.md) | Full at launch | Not committed to git |
| 6 | Auto memory (~/.claude/projects/) | First 200 lines of MEMORY.md | Topic files on demand |

**Ключевое наблюдение для bmad-ralph:** Auto memory (уровень 6) загружает только первые 200 строк MEMORY.md при старте [S4]. Topic files (отдельные файлы в том же directory) загружаются on demand, когда Claude определяет их relevance. Это означает:

- MEMORY.md — это index/summary, а не хранилище всех знаний
- Topic-based sharding (отдельные файлы по темам) позволяет хранить значительно больше знаний, загружая только релевантные
- 200-line budget LEARNINGS.md в bmad-ralph архитектуре корректно соответствует auto memory constraint

**CLAUDE.md cascade:** файлы CLAUDE.md выше CWD загружаются полностью при запуске, дочерние директории — on demand [S4]. Для bmad-ralph это значит: корневой CLAUDE.md с секцией `## Ralph operational context` загружается всегда, без дополнительных действий.

**.claude/rules/ directory** поддерживает path-specific rules с glob patterns в YAML frontmatter [S4]. Это альтернативный механизм для context-dependent знаний: правило `*.go` применяется только при работе с Go файлами. Текущая архитектура bmad-ralph не использует этот механизм, но для Epic 6 он может быть полезен для language-specific learnings.

**Skills directory** (.claude/skills/) предоставляет SKILL.md файлы с 500-line limit и auto/manual invoke [S12]. Skills — более формализованный механизм, чем CLAUDE.md: они имеют frontmatter с описанием и glob patterns, загружаются on demand. Для bmad-ralph skills могут хранить distilled operational procedures, но не raw learnings.

**@import syntax** позволяет ссылаться на файлы из CLAUDE.md (max depth 5) [S4]. Это позволяет CLAUDE.md быть compact index, а подробные знания хранить в linked files. Однако каждый import увеличивает context при загрузке — нужен budget awareness.

### 4.2 Context Rot and Attention Limitations

Исследование Chroma Research [S5] предоставляет наиболее систематическую количественную оценку context rot:

**Масштаб тестирования:** 18 frontier models, 8 input lengths, 11 needle positions — наиболее масштабное исследование degradation на момент публикации.

**Универсальность деградации:** ВСЕ 18 протестированных моделей деградируют при росте контекста [S5]. Это не проблема конкретной модели — это фундаментальное ограничение transformer architecture. Для bmad-ralph это означает: никакая будущая модель не решит проблему context rot; architectural mitigation — единственный путь.

**Количественные показатели:**
- 30-50%+ performance variance между compact и full context [S5]
- "Lost in the middle" effect: модели хорошо обрабатывают начало и конец контекста, но слабо — середину [S5]
- Lower needle-question similarity = более крутая деградация при масштабировании [S5] — знания с low semantic overlap с текущей задачей теряются первыми

**Counterintuitive finding:** shuffled haystacks (перемешанные контексты) работают лучше, чем coherent organized ones [S5]. Возможное объяснение: организованный текст создаёт ложное чувство "контекст уже покрыт", в то время как хаотичный заставляет модель обрабатывать каждый фрагмент отдельно. Для bmad-ralph: монолитный narrative LEARNINGS.md может быть менее эффективен, чем atomized facts.

**Claude-specific поведение:** Claude models показывают lowest hallucination rates, но highest abstention [S5]. Когда Claude не находит ответ в контексте, он скорее откажется отвечать, чем выдумает. Для bmad-ralph это значит: потерянное в контексте знание проявится как "забытые" паттерны, а не как галлюцинации.

**Практический лимит инструкций:** ~150-200 инструкций — экспериментально определённая граница consistent following [S14]. Превышение приводит к:
- Selective ignoring без уведомления
- Непредсказуемый выбор игнорируемых инструкций
- Деградация при compaction (инструкции summarized away)

**CLAUDE.md framing problem:** CLAUDE.md инжектируется в контекст с оговоркой "may or may not be relevant to your tasks" [S6]. Эта обёртка системно снижает adherence — модель получает explicit permission игнорировать содержимое. Реальный CLAUDE.md bmad-ralph уже содержит эту обёртку (видно в system-reminder). Hooks обходят framing, инжектируя через clean system-reminder path [S6].

**Compaction уничтожает values:** при compaction CLAUDE.md values summarized away [S6]. Для long-running сессий bmad-ralph (multi-turn execute + review cycles) это означает: знания из LEARNINGS.md, загруженные в начале сессии, могут быть потеряны при compaction mid-session.

### 4.3 Knowledge Persistence Patterns

#### 4.3.1 Compaction: capabilities and limitations

Anthropic явно заявляет: **compaction alone is NOT sufficient** для long-running agents [S1]. Compaction в Claude Code:
- Суммаризует историю, сохраняя architecture decisions [S2]
- Discards redundant outputs [S2]
- Сохраняет 5 most recent files [S2]
- Теряет конкретные инструкции, numerical values, edge cases [S6]

Практическая рекомендация Anthropic для long-running agents: **two-agent architecture** (Initializer + Coding agent) с incremental progress tracking [S1]. Initializer agent при старте каждой сессии:
- Читает git log для понимания текущего состояния
- Читает progress files (claude-progress.txt, feature list JSON)
- Выбирает следующую задачу на основе текущего state

**Зафиксированные failure modes** [S1]:
1. Попытка сделать всё за одну сессию ("attempting everything at once")
2. Undocumented progress (прогресс только в context, не персистирован)
3. Premature completion (агент заявляет завершение без проверки)

Для bmad-ralph паттерн two-agent architecture уже реализован: runner (orchestrator) + Claude sessions (workers). Progress tracking через sprint-tasks.md + git commits. Failure modes 1-3 mitigation через gate system (Epic 5) + review cycle (Epic 4).

#### 4.3.2 Structured note-taking

Anthropic рекомендует structured notes как primary persistence mechanism [S1][S2]:

- **NOTES.md** с objectives, progress, insights — plain text, загружаемый при resume [S2]
- **claude-progress.txt** — human-readable progress для multi-session work [S1]
- **Feature list JSON** — machine-readable state для programmatic decisions [S1]
- **Git commits** как checkpoints с descriptive messages [S1]

Ключевой принцип: **dual representation** — human-readable (для review/debugging) + machine-readable (для programmatic consumption). bmad-ralph Epic 6 реализует это: LEARNINGS.md (human-readable lessons) + CLAUDE.md section (structured operational context).

#### 4.3.3 Just-in-time retrieval vs pre-loading

Anthropic описывает hybrid approach [S2]:

- **Pre-load critical:** architectural decisions, project conventions, current task context
- **Just-in-time retrieval:** code details, file contents, API references — загружаются по необходимости через tool use
- **Identifiers + tools:** вместо загрузки всего файла, загружается identifier + tool для on-demand доступа

Формулировка Anthropic: "smallest possible set of high-signal tokens" [S2]. Для bmad-ralph: LEARNINGS.md pre-loaded (200 lines — small, high-signal), project files loaded on demand через Claude Code built-in tools.

#### 4.3.4 Hook-based enforcement

Hooks — 12 lifecycle events в Claude Code [S13]:

| Hook Event | Trigger | Relevance для knowledge |
|------------|---------|-------------------------|
| SessionStart | startup, resume, /clear, /compact | Primary injection point |
| PreCompact | Before compaction | Save critical instructions |
| UserPromptSubmit | Before each prompt sent | Per-turn injection (expensive) |
| PreToolUse / PostToolUse | Before/after tool calls | Validation of knowledge-writing tools |

**SessionStart** — ключевой hook для bmad-ralph [S13]: fires on startup AND resume AND compact. Это значит: даже после compaction, SessionStart hook re-injects critical knowledge. Cost: ~750 tokens per 50 turns [S6] — negligible для multi-turn sessions.

**PreCompact** hook получает `trigger` и `custom_instructions` [S13]. Позволяет инструктировать compaction сохранить specific knowledge. Для bmad-ralph: PreCompact может инжектировать "preserve LEARNINGS.md content and architectural decisions."

**Async hooks** (с Jan 2026) [S13] позволяют non-blocking operations. Knowledge persistence (запись LEARNINGS.md) может быть async hook, не блокирующий основной flow.

**stdout injection:** hook stdout отправляется в context [S13]. Это позволяет hooks динамически загружать и инжектировать контекст — например, читать актуальную версию LEARNINGS.md и инжектировать при каждом SessionStart.

#### 4.3.5 Sub-agent architecture

Anthropic рекомендует sub-agents для context isolation [S2]:

- Focused sub-agents работают с limited context (одна задача)
- Lead agent получает 1000-2000 token summaries от sub-agents
- Каждый sub-agent может иметь свой набор загруженных знаний

bmad-ralph уже использует sub-agent pattern: execute session, review session, resume-extraction session — каждая с своим prompt и context. Epic 6 расширяет это: distillation session (`claude -p`) как ещё один specialized sub-agent.

### 4.4 Third-Party and Comparative Approaches

#### 4.4.1 GitHub Copilot Agentic Memory

GitHub Copilot реализует наиболее зрелую production memory system [S3][S18]:

**Структура:** repo-scoped memories с полями `{subject, fact, citations, reason}` [S3][S18]. Citations — ссылки на конкретные участки кода, которые обосновывают memory.

**Citation validation:** перед использованием memory, система проверяет: существует ли цитируемый код? Соответствует ли он факту? [S3][S18]. Если код изменился — memory помечается как stale и обновляется или удаляется.

**Self-healing:** adversarial memories (устаревшие или некорректные) автоматически детектируются и корректируются через citation validation [S3]. Это решает проблему knowledge rot — аналог context rot, но для persisted knowledge.

**Cross-agent sharing:** memories доступны всем agent types — review agent, coding agent, CLI agent [S3]. Знание, полученное при code review, используется при coding, и наоборот.

**Measured impact:** 7% improvement в PR merge rate (90% vs 83%), p < 0.00001 [S3]. Статистически значимый результат на production traffic.

**Implications для bmad-ralph:** citation-based validation — применимый паттерн. При загрузке LEARNINGS.md, каждый lesson может содержать reference (story ID, file path, commit hash). При следующей загрузке — проверка: файл/commit ещё существуют? Если нет — lesson помечается как stale. Реализация: проверка в distillation session или как pre-processing step в runner.

#### 4.4.2 claude-mem: Automatic Capture and Semantic Summarization

claude-mem plugin [S10] реализует:

- **Automatic capture:** перехват tool executions и их результатов
- **Semantic summarization:** сжатие captured data с сохранением семантики
- **Vector search:** retrieval по semantic similarity к текущей задаче
- **Progressive disclosure:** постепенное раскрытие знаний вместо dump-all-at-once

Progressive disclosure — важный паттерн: вместо загрузки всех 200 строк LEARNINGS.md, загрузить summary (20 строк) + provide tool для доступа к details. Снижает base context cost при сохранении полного access.

Для bmad-ralph: progressive disclosure может быть реализован через topic-based sharding — summary в LEARNINGS.md (200 строк), details в topic files (LEARNINGS-testing.md, LEARNINGS-architecture.md).

#### 4.4.3 Claudeception: Autonomous Skill Extraction

Claudeception [S9] реализует паттерн session review → skill extraction:

- После завершения сессии, отдельный agent анализирует transcript
- Извлекает reusable patterns и skills
- Сохраняет как structured skill files

Этот паттерн — прямой аналог bmad-ralph resume-extraction (Story 6.6). Разница: Claudeception анализирует full session transcript, а bmad-ralph resume-extraction фокусируется на failure analysis.

#### 4.4.4 Continuous-learning Skill Pattern

continuous-learning skill [S11] демонстрирует timing advantage:

- Knowledge extraction **during** session (in-context) vs **after** session (post-hoc)
- In-context extraction захватывает reasoning и context, потерянные при compaction
- Skill files с SKILL.md format, 500-line limit [S12]

**Timing advantage** — ключевое наблюдение: знания, извлечённые *во время* сессии, богаче, чем извлечённые из результата сессии. bmad-ralph review session уже имеет in-context advantage: Claude при review видит findings и может сразу записать lessons в LEARNINGS.md (Story 6.7). Resume-extraction — post-hoc анализ, менее богатый контекстом.

### 4.5 Implications for bmad-ralph Knowledge System

#### 4.5.1 Current Design Assessment

Текущая архитектура Epic 6:
- **LEARNINGS.md:** 200-line budget, distilled via `claude -p`, loaded into every session
- **CLAUDE.md `## Ralph operational context`:** managed section, project-visible
- **Triggers:** after execute cycles (via --always-extract), after review cycles (on findings)

**Что будет работать (evidence-based):**

1. **200-line budget LEARNINGS.md** корректно соответствует 200-line auto memory limit [S4] и ~150-200 instruction following limit [S14]. Evidence: Chroma Research показывает 30-50% degradation при полном контексте [S5] — compact context critical.

2. **Distillation через `claude -p`** — sound approach. Anthropic рекомендует structured notes + periodic compression [S1][S2]. Backup before distillation (Story 6.3) — правильная safety measure.

3. **Review-triggered knowledge extraction** (Story 6.7) использует timing advantage [S11]: Claude видит findings in-context и может записать high-quality lessons. Это лучший момент для extraction.

4. **Separation LEARNINGS.md + CLAUDE.md section** — dual representation pattern [S1]: LEARNINGS.md = accumulated lessons (append-only до distillation), CLAUDE.md section = current operational state (overwritten).

**Что может не работать (evidence-based risks):**

1. **CLAUDE.md framing problem** [S6]: LEARNINGS.md content, инжектированный в prompt через strings.Replace, наследует CLAUDE.md disclaimer ("may or may not be relevant"). Mitigation: hook-based injection через SessionStart bypass framing.

2. **Compaction mid-session** [S1][S6]: long execute sessions могут trigger compaction, теряя LEARNINGS.md content загруженный в начале. Mitigation: PreCompact hook с custom_instructions для сохранения knowledge.

3. **No citation validation** [S3][S18]: LEARNINGS.md lessons не привязаны к конкретному коду. Устаревшие lessons остаются до distillation. Mitigation: добавить source references (story ID, file path) для каждого lesson.

4. **Dangerous feedback loop** (identified in architecture): больше ошибок -> больше learnings -> больше tokens -> больше context rot [S5] -> больше ошибок. Mitigation: 200-line hard limit + distillation — но distillation quality зависит от Claude, создавая secondary risk.

#### 4.5.2 Structural Recommendations

**Recommendation 1: Topic-based Sharding**

Вместо монолитного LEARNINGS.md, использовать:
- `LEARNINGS.md` — 200-line summary/index (всегда загружается)
- `memory/learnings-testing.md` — детальные testing patterns (загружаются on demand)
- `memory/learnings-architecture.md` — architectural decisions (загружаются on demand)

Обоснование: Claude Code auto memory поддерживает topic files [S4]. Progressive disclosure (claude-mem pattern [S10]) снижает base context cost. Chroma Research показывает: compact context >> full context [S5].

**Recommendation 2: Hook-based Critical Rule Injection**

SessionStart hook для инжекции топ-10 critical rules из LEARNINGS.md. Обходит CLAUDE.md framing problem [S6]. Fires on startup/resume/compact [S13] — guaranteed re-injection.

Конкретная реализация для bmad-ralph:
```json
{
  "hooks": {
    "SessionStart": [{
      "type": "command",
      "command": "cat /path/to/project/LEARNINGS-critical.md"
    }]
  }
}
```

Cost: ~750 tokens per 50 turns [S6]. При типичной execute session в 5-15 turns — negligible overhead.

**Recommendation 3: Citation Validation**

Добавить source reference к каждому lesson в LEARNINGS.md:

```markdown
## Testing Pattern: Always test zero values [Story 1.2, config/errors.go]
ExitCodeError{} and GateDecision{} — test zero-value behavior catches uninitialized field bugs.
```

При distillation (Story 6.3): `claude -p` prompt включает инструкцию проверить — существуют ли referenced files/stories? Удалить lessons с невалидными references. Паттерн из GitHub Copilot [S3][S18] адаптирован для file-based knowledge.

**Recommendation 4: PreCompact Knowledge Preservation**

PreCompact hook инжектирует: "When compacting, preserve the following critical knowledge from LEARNINGS.md: [top-10 lessons]" [S13]. Mitigates compaction destroys values problem [S6].

**Recommendation 5: Distillation Quality Guard**

После distillation, budget check + content validation:
- Результат под 200 строк? (budget check — Story 6.3 already covers)
- Результат не пустой? (basic sanity)
- Key topics preserved? (count section headers before/after)

Если validation fails — restore from backup (Story 6.3 already covers backup/restore).

**Recommendation 6: Monitoring Feedback Loop**

Метрика: track learnings count и review findings count over time. Если findings count растёт одновременно с learnings count — feedback loop detected. Logging в NFR14 log format.

Конкретная реализация: runner logs `knowledge_lines_total` и `review_findings_count` per task. Тренд-анализ при retrospective.

**Recommendation 7: Atomized Facts over Narrative**

Chroma Research показывает: shuffled (atomized) контекст работает лучше organized narrative [S5]. LEARNINGS.md должен быть набором independent facts, а не связным повествованием:

Плохо (narrative):
```
During Story 1.8, we discovered that json.Unmarshal cannot distinguish truncated
JSON from non-JSON, because both fail the same way. This led us to accept fallback
behavior and document the deviation from spec.
```

Хорошо (atomized):
```
## json.Unmarshal: truncated vs non-JSON indistinguishable [Story 1.8]
Both fail identically. Accept fallback behavior, document deviation.
```

Atomized format: shorter (fewer tokens), independently retrievable, resistant to "lost in the middle" [S5].

---

## 5. Risks and Limitations

### 5.1 Context Rot is Universal and Unsolved

Все 18 frontier models деградируют при росте контекста [S5]. Нет evidence что будущие модели решат эту проблему — это может быть fundamental limitation transformer architecture. Архитектурные mitigations (budget limits, sharding, hooks) — необходимы независимо от развития моделей.

### 5.2 Auto Memory Accuracy

Claude Code auto memory (MEMORY.md) управляется моделью автономно [S4]. Нет гарантии, что модель корректно определяет, какие facts сохранять. В bmad-ralph LEARNINGS.md записывается Claude during review/resume-extraction — та же проблема. Mitigation: human review при retrospective + citation validation.

### 5.3 No Comprehensive Benchmarks for Knowledge Effectiveness

GitHub Copilot 7% improvement [S3] — единственный количественный benchmark для coding agent memory. Нет comparable данных для Claude Code memory. Anthropic предоставляет qualitative guidance [S1][S2], но не quantitative benchmarks. Рекомендация: bmad-ralph should instrument и measure собственный knowledge effectiveness.

### 5.4 Copilot Architecture Differences

GitHub Copilot memory data [S3][S18] получена в другой архитектуре (cloud-based, repo-scoped, multi-user). Direct applicability к bmad-ralph (local CLI, single-user, session-based) ограничена. Citation validation pattern — transferable; абсолютные числа — нет.

### 5.5 Distillation Quality Risk

Distillation через `claude -p` [Story 6.2-6.3] зависит от способности Claude сжимать без потери critical information. Нет данных о distillation accuracy. Backup mechanism (Story 6.3) — необходимый safety net. Рекомендация: при первых запусках, human review distillation results.

### 5.6 Hook Ecosystem Maturity

Hooks API в Claude Code — relative recent addition [S13]. Async hooks — с Jan 2026 [S13]. Ecosystem всё ещё развивается. Risk: API changes may break hook-based knowledge injection. Mitigation: minimal, focused hooks; abstract hook logic into replaceable shell scripts.

---

## 6. Recommendations for bmad-ralph Epic 6

### R1: Implement LEARNINGS.md with Topic Sharding (Priority: HIGH)

Реализовать LEARNINGS.md как 200-line index file + topic files в `memory/` directory.

**Evidence:** Auto memory loads first 200 lines + topic files on demand [S4]. Progressive disclosure pattern из claude-mem [S10] снижает base context cost. Chroma Research: compact context 30-50% effective over full [S5].

**Конкретно для Epic 6:** Story 6.1 (KnowledgeWriter) должен поддерживать write to specific topic file. BudgetCheck проверяет main LEARNINGS.md (200 lines). Topic files — без hard limit, но с distillation.

### R2: Add SessionStart Hook for Critical Knowledge (Priority: HIGH)

Создать SessionStart hook, который `cat` top-priority rules из LEARNINGS.md в каждую сессию. Обходит CLAUDE.md "may or may not be relevant" framing [S6].

**Evidence:** Hook cost: ~750 tokens/50 turns [S6]. SessionStart fires on startup/resume/clear/compact [S13] — guaranteed re-injection. Единственный reliable mechanism для critical instructions.

**Конкретно для Epic 6:** Не требует изменения Stories. Ralph может генерировать hook config (.claude/settings.json) при initialization. Hook command: `head -20 LEARNINGS.md` (top-priority lessons first).

### R3: Add Citation References to Lessons (Priority: MEDIUM)

Каждый lesson в LEARNINGS.md должен содержать source reference: story ID, file path, или commit hash. При distillation — validate references, remove stale lessons.

**Evidence:** GitHub Copilot citation-based validation: 7% merge rate improvement, self-healing [S3][S18]. Citation validation solves knowledge rot — complement to context rot mitigation.

**Конкретно для Epic 6:** Story 6.6 (Resume-Extraction Knowledge) и Story 6.7 (Review Knowledge) — добавить instructions в prompts: "include source reference [Story X.Y, file.go] for each lesson." Story 6.2 (Distillation Prompt) — добавить instructions: "verify references, remove lessons with invalid references."

### R4: Implement PreCompact Hook for Knowledge Preservation (Priority: MEDIUM)

PreCompact hook инжектирует critical knowledge перед compaction. Prevents compaction from destroying learned values [S6].

**Evidence:** Compaction destroys values — confirmed by Anthropic [S1] and practitioners [S6]. PreCompact hook receives trigger + custom_instructions [S13].

**Конкретно для Epic 6:** New hook, not in current stories. Low implementation cost. Command: `echo "CRITICAL: Preserve these learnings during compaction:" && head -20 LEARNINGS.md`.

### R5: Use Atomized Fact Format in LEARNINGS.md (Priority: MEDIUM)

Формат каждого lesson: one-line header + one-line description. Independent facts, не narrative.

**Evidence:** Shuffled haystacks outperform organized ones [S5]. Lower semantic similarity between facts = less "lost in the middle" effect [S5]. Atomized format = fewer tokens per fact = more facts within 200-line budget.

**Конкретно для Epic 6:** Format convention в distillation prompt (Story 6.2). Review/resume-extraction prompts (Stories 6.6, 6.7) — instructions для atomized format.

### R6: Instrument Knowledge Effectiveness (Priority: LOW)

Добавить logging: `knowledge_lines_total`, `review_findings_count`, `distillation_count`, `lessons_per_story`. Track trends для detecting feedback loop.

**Evidence:** Dangerous feedback loop identified in architecture. No existing benchmarks для Claude Code knowledge effectiveness [S3]. Self-measurement — единственный путь к optimization.

**Конкретно для Epic 6:** NFR14 уже определяет log format. Добавить fields в Story 6.5 (Knowledge Loading) logging.

### R7: Evaluate In-Context Extraction Over Post-Hoc (Priority: LOW)

Текущая архитектура: review session writes lessons in-context (good) [S11], resume-extraction is post-hoc (less rich). Evaluate: should --always-extract (Story 6.9) extract in-context during execute session instead of separate post-hoc session?

**Evidence:** Timing advantage: in-context extraction captures reasoning lost after session [S11]. continuous-learning skill pattern [S11] and Claudeception [S9] both demonstrate in-context extraction superiority.

**Конкретно для Epic 6:** Not a change in current stories — evaluation item для future iteration. May require prompt modification in execute prompt (Story 3.1) — adds complexity, needs careful assessment.

---

## 7. Conclusion

Знания, накопленные в ходе этого исследования, формулируются в одном предложении: **knowledge management в coding agents — это compression problem under attention constraints, решаемая архитектурно через budget limits, hook-based enforcement, и citation validation, а не через увеличение context window.**

bmad-ralph Epic 6 архитектура в целом корректна: 200-line LEARNINGS.md, distillation, dual persistence (LEARNINGS.md + CLAUDE.md section), in-context extraction при review. Основные дополнения по результатам исследования:

1. Hook-based injection для обхода CLAUDE.md framing (высокий приоритет)
2. Citation references для self-healing knowledge (средний приоритет)
3. Atomized fact format для resistance к "lost in the middle" (средний приоритет)
4. Topic sharding для progressive disclosure (высокий приоритет)

Все рекомендации совместимы с текущей Story structure Epic 6 и могут быть интегрированы как дополнительные acceptance criteria или as follow-up refinements.

---

## Appendix A: Evidence Table

| ID | Title | Publisher | Date | Tier |
|----|-------|-----------|------|------|
| S1 | Effective harnesses for long-running agents | Anthropic Engineering | 2025-11 | A |
| S2 | Effective context engineering for AI agents | Anthropic Engineering | 2025 | A |
| S3 | Building an agentic memory system for GitHub Copilot | GitHub Blog | 2025 | A |
| S4 | Manage Claude's memory | Claude Code Docs | 2026 | A |
| S5 | Context Rot: How Increasing Input Tokens Impacts LLM Performance | Chroma Research | 2025 | A |
| S6 | Your CLAUDE.md Instructions Are Being Ignored | DEV Community | 2025 | B |
| S7 | Claude Code Best Practices: Memory Management | Code Centre | 2025 | B |
| S8 | Claude Code Best Practices | Anthropic Docs | 2026 | A |
| S9 | Claudeception - Autonomous Skill Extraction | GitHub/blader | 2025 | B |
| S10 | claude-mem plugin | GitHub/thedotmack | 2025 | B |
| S11 | Continuous-learning skill | GitHub/affaan-m | 2025 | B |
| S12 | Extend Claude with skills | Claude Code Docs | 2026 | A |
| S13 | Claude Code hooks reference | Claude Code Docs | 2026 | A |
| S14 | Writing a good CLAUDE.md | HumanLayer Blog | 2025 | B |
| S15 | Cognee + Claude Agent SDK memory integration | Cognee | 2025 | B |
| S16 | Memory and Context Management in Claude Agent SDK | GitHub/bgauryy | 2025 | B |
| S17 | Anthropic Agent SDK overview | Claude API Docs | 2026 | A |
| S18 | About agentic memory for GitHub Copilot | GitHub Docs | 2026 | A |
| S19 | Claude Skills and CLAUDE.md: practical 2026 guide | Gend.co | 2026 | B |
| S20 | Claude Code's Memory: Large Codebases | Thomas Landgraf | 2025 | B |

## Appendix B: Source URLs

1. [S1] anthropic.com/engineering/effective-harnesses-for-long-running-agents
2. [S2] anthropic.com/engineering/effective-context-engineering-for-ai-agents
3. [S3] github.blog/ai-and-ml/github-copilot/building-an-agentic-memory-system-for-github-copilot/
4. [S4] code.claude.com/docs/en/memory
5. [S5] research.trychroma.com/context-rot
6. [S6] dev.to/albert_nahas_cdc8469a6ae8/
7. [S7] cuong.io/blog/2025/06/15-claude-code-best-practices-memory-management
8. [S8] code.claude.com/docs/en/best-practices
9. [S9] github.com/blader/Claudeception
10. [S10] github.com/thedotmack/claude-mem
11. [S11] github.com/affaan-m/everything-claude-code/
12. [S12] code.claude.com/docs/en/skills
13. [S13] code.claude.com/docs/en/hooks
14. [S14] humanlayer.dev/blog/writing-a-good-claude-md
15. [S15] cognee.ai/blog/
16. [S16] github.com/bgauryy/open-docs/
17. [S17] platform.claude.com/docs/en/agent-sdk/overview
18. [S18] docs.github.com/en/copilot/concepts/agents/copilot-memory
19. [S19] gend.co/blog/claude-skills-claude-md-guide
20. [S20] thomaslandgraf.substack.com
