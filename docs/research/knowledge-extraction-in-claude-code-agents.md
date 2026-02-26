# Knowledge Extraction in Claude Code-Based Coding Agents: Patterns, Limitations, and Architectural Recommendations

**Исследовательский отчёт для проекта bmad-ralph**
**Дата:** 2026-02-26
**Версия:** 1.0

---

## Executive Summary

- Управление знаниями в coding agents на базе Claude Code находится на переходном этапе: инфраструктура хранения решена (6-уровневая memory hierarchy, hooks, skills), но **эффективное использование** остаётся открытой проблемой из-за универсальной деградации LLM при росте контекста [S5].
- **Context rot — фундаментальное и неустранимое ограничение:** все 18 протестированных frontier моделей показывают 30-50%+ degradation при полном контексте vs компактном [S5]. Это означает, что budget management — не оптимизация, а инженерная необходимость.
- **Citation-based validation — единственный количественно доказанный паттерн:** GitHub Copilot Memory показал 7% рост PR merge rate (90% vs 83%, p < 0.00001) с self-healing архитектурой [S3, S18].
- **Hook-based enforcement обходит CLAUDE.md framing problem.** Содержимое CLAUDE.md оборачивается disclaimer "may or may not be relevant" [S6], а при compaction может быть summarized away. Hooks through SessionStart bypass обе проблемы [S6, S13].
- Архитектура bmad-ralph Epic 6 (200-line LEARNINGS.md + distillation + CLAUDE.md section) в целом **корректна**, но нуждается в усилении: topic-based sharding, atomized fact format, citation references, hook-based enforcement для critical rules, и мониторинг feedback loop.

---

## 1. Research Question and Scope

### Основной вопрос

Как эффективно извлекать, хранить и применять знания в coding agents, построенных на Claude Code, в контексте автономной multi-session разработки?

### Подвопросы

1. Какие механизмы памяти предоставляет Claude Code и каковы их ограничения?
2. Как context rot влияет на эффективность накопленных знаний?
3. Какие паттерны knowledge persistence доказали эффективность?
4. Как third-party решения (GitHub Copilot Memory, claude-mem, Claudeception) подходят к проблеме?
5. Какие архитектурные рекомендации применимы к bmad-ralph Epic 6?

### Scope

- **Включено:** Claude Code ecosystem (CLI, Agent SDK, hooks, skills), GitHub Copilot memory (сравнение), third-party tools (claude-mem, Claudeception, continuous-learning), академические исследования context rot
- **Период:** 2024-2026
- **Исключено:** Non-LLM memory systems, внутренняя архитектура Cursor/Copilot (закрытая), fine-tuning подходы, generic RAG pipelines без привязки к coding agents

---

## 2. Methodology

Исследование основано на 20 источниках, классифицированных по надёжности:

- **Tier A (10 источников):** Официальная документация Anthropic [S1, S2, S4, S8, S12, S13, S17], GitHub [S3, S18], академические исследования [S5]
- **Tier B (10 источников):** Инженерные блоги и open-source проекты с верифицируемыми утверждениями [S6, S7, S9, S10, S11, S14, S15, S16, S19, S20]
- **Tier C (исключены):** Общие AI обзоры без конкретных данных, маркетинговые материалы

Приоритет отдан источникам с измеримыми результатами (Chroma Research [S5], GitHub A/B tests [S3]) и официальной документации с implementation details [S4, S13]. При конфликте между источниками приоритет — Tier A.

---

## 3. Key Findings

1. **Claude Code реализует 6-уровневую иерархию памяти** с различными механизмами загрузки: от eager (CLAUDE.md при старте) до lazy (topic files по запросу) [S4]. Это создаёт архитектурную возможность для tiered knowledge, но усложняет предсказуемость того, какие знания окажутся в контексте.

2. **Context rot универсален и нелинеен.** Chroma Research (18 моделей, 8 длин контекста, 11 позиций needle) показало 30-50%+ degradation между compact (~300 tokens) и full (113k tokens) сценариями [S5]. Claude показал наименьший hallucination rate, но наивысший abstention rate.

3. **CLAUDE.md instructions систематически подрываются framing.** Содержимое оборачивается disclaimer "may or may not be relevant", а при compaction summarize-ится вместе с low-priority контентом [S6]. Hook-based reinforcement через SessionStart и PreCompact обходит эти проблемы [S6, S13].

4. **Compaction alone is NOT sufficient** для multi-session work — Anthropic прямо заявляет это [S1]. Требуется explicit progress tracking через external artifacts (git history, progress files, feature list JSON).

5. **Citation-based validation — единственный паттерн с количественно доказанной эффективностью.** GitHub Copilot: 7% increase в PR merge rates (90% vs 83%, p < 0.00001), self-healing через adversarial memory correction [S3, S18].

6. **~150-200 инструкций — практический предел consistent following** для frontier LLM [S14]. Превышение приводит к selective ignoring без уведомления.

7. **Structured note-taking с progress files** — проверенный Anthropic-паттерн для session continuity [S1, S2]. Notes пишутся агентом (self-consistent context), не оркестратором.

8. **Third-party tools демонстрируют convergent evolution** к паттерну capture → compress → inject on demand. Три независимых проекта (claude-mem [S10], Claudeception [S9], continuous-learning [S11]) пришли к одной архитектуре.

9. **Counterintuitive: shuffled/unstructured контексты outperform organized** в needle-in-haystack тестах [S5]. Coherent text создаёт ложные ассоциации между adjacent tokens — модель "скользит" по знакомой структуре вместо точного поиска.

10. **Skills (SKILL.md) предоставляют 500-line budget** с auto/manual invocation и split-паттерном [S12] — альтернативный delivery mechanism, не используемый в текущем дизайне Epic 6.

---

## 4. Analysis

### 4.1. Claude Code Memory Architecture

Claude Code реализует шестиуровневую иерархию, каждый уровень которой имеет свою семантику, scope и механизм загрузки [S4]:

| Уровень | Тип | Загрузка | Scope | Budget |
|---------|-----|----------|-------|--------|
| 1 | Managed policy | Всегда, высший приоритет | Global | N/A |
| 2 | Project CLAUDE.md | Eager (выше cwd) / on demand (ниже cwd) | Repository | ~100-200 строк рекомендовано [S14] |
| 3 | Project rules (.claude/rules/) | По glob pattern match | Path-specific | Per-file |
| 4 | User memory (~/.claude/CLAUDE.md) | Eager (при запуске) | All projects | ~30-100 строк [S7] |
| 5 | Project local (CLAUDE.local.md) | Eager | Repository, per-user | Нет hard limit |
| 6 | Auto memory (MEMORY.md) | Первые 200 строк; topic files on demand | Project, per-user | 200 строк hard limit |

**Eager vs lazy loading.** CLAUDE.md файлы выше CWD загружаются полностью при старте, дочерние — on demand [S4]. Для bmad-ralph: корневой CLAUDE.md с `## Ralph operational context` загружается eager — baseline knowledge в каждой сессии. Но LEARNINGS.md, загружаемый через prompt assembly, наследует prompt framing.

**Auto memory 200-line budget.** MEMORY.md загружает первые 200 строк при старте, topic files — when Claude decides they're relevant [S4]. Для autonomous agent (без human guidance) JIT-retrieval рискован — агент может не "вспомнить" что ему нужно. Но 200-строчный budget LEARNINGS.md в bmad-ralph корректно соответствует этому constraint.

**@import и .claude/rules/.** `@import` syntax с max depth 5 и .claude/rules/ с glob patterns в YAML frontmatter предоставляют модульную организацию [S4]. Rules для `*.go` загружаются только при работе с Go — contextual filtering, напрямую применимый для topic-based knowledge.

**Skills directory.** SKILL.md с 500-line limit, auto/manual invocation и split-паттерном [S12] — ещё один delivery mechanism. Skills загружаются по match с task description. Для knowledge delivery: packaging stable patterns as skills vs volatile learnings in LEARNINGS.md.

### 4.2. Context Rot and Attention Limitations

Chroma Research 2025 [S5] — наиболее систематическое исследование:

| Метрика | Результат | Источник |
|---------|-----------|----------|
| Модели протестированы | 18 frontier (Claude, GPT-4, Gemini, Qwen) | [S5] |
| Performance variance (compact vs full) | 30-50%+ | [S5] |
| "Lost in the middle" degradation | >30% drop vs begin/end positions | [S5] |
| Semantic ambiguity amplification | Линейный рост degradation | [S5] |
| Claude hallucination rate | Наименьший среди 18 моделей | [S5] |
| Claude abstention rate | Наивысший | [S5] |
| Shuffled vs coherent | Shuffled показывает лучшую retrieval accuracy | [S5] |

**Универсальность.** Все 18 моделей деградируют — это фундаментальное свойство transformer attention, не артефакт конкретной модели [S5]. Никакая будущая модель не "решит" context rot; architectural mitigation — единственный путь.

**"Lost in the middle".** Модели хорошо attend к началу и концу контекста, хуже — к середине [S5]. Для LEARNINGS.md, загружаемого в начало промпта, это благоприятно. Но по мере роста промпта (sprint-tasks, code, tool outputs) LEARNINGS.md "сдвигается" к середине.

**Semantic ambiguity compounds.** При низком сходстве между инструкцией и текущей задачей, деградация круче [S5]. Абстрактные правила ("всегда используй errors.As") менее уязвимы (высокое сходство с конкретным error handling кодом), чем контекстно-зависимые ("в Story 1.8 обнаружено что truncated JSON...").

**Shuffled outperforms organized.** Перемешанные контексты лучше coherently organized [S5]. Гипотеза: организованный текст создаёт attention shortcuts — модель "скользит" по знакомой структуре. Это имеет значение для LEARNINGS.md: atomized independent facts могут быть эффективнее structured narrative.

**~150-200 instruction limit.** Frontier LLMs следуют ~150-200 инструкциям с reasonable consistency [S14]. Для 200-line LEARNINGS.md при 3-5 строках на инструкцию — budget ~40-60 distinct rules. Текущий CLAUDE.md bmad-ralph уже содержит ~50 правил в `## Testing`, что приближается к лимиту без учёта LEARNINGS.md.

**Framing problem.** CLAUDE.md content оборачивается "may or may not be relevant to your tasks" [S6]. Этот фрейминг подрывает authority — модель может "решить" что правило нерелевантно. При compaction values summarize-ятся вместе с low-priority контентом [S6]. Hook-based reinforcement обходит обе проблемы [S6, S13].

### 4.3. Knowledge Persistence Patterns

#### 4.3.1. Compaction: capabilities and limitations

Compaction в Claude Code: history summarize-ируется, architecture decisions сохраняются, redundant tool outputs discarded, 5 most recent files сохраняются [S2].

Anthropic явно заявляет: **compaction alone is NOT sufficient** [S1]. Причины:
- Compaction оптимизирует для recency, не для importance
- Inter-session state теряется полностью
- Compaction не имеет domain knowledge для различения "это мы уже исправили" от "это правило навсегда"

Для bmad-ralph: подтверждает правильность explicit knowledge files вместо reliance на compaction. Но within-session compaction остаётся проблемой: LEARNINGS.md content может быть summarized away mid-session.

#### 4.3.2. Structured note-taking

Anthropic рекомендует structured notes как primary persistence [S1, S2]:

| Artifact | Назначение | Persistence |
|----------|-----------|-------------|
| claude-progress.txt | Текущее состояние | Файловая система [S1] |
| Git history | История изменений | Git [S1] |
| Feature list JSON | Структурированный план | Файловая система [S1] |
| NOTES.md | Objectives, progress, insights | Файловая система [S2] |

**Failure modes без external artifacts [S1]:** attempting everything at once, undocumented progress, premature completion declarations.

Ключевое для bmad-ralph: в Anthropic-паттерне agent пишет notes для себя (self-consistency). В bmad-ralph один session (execute) пишет knowledge для другого session (следующий execute). Writer не знает точно контекст reader — additional challenge.

#### 4.3.3. Just-in-time retrieval vs pre-loading

| Аспект | Pre-loading | JIT Retrieval |
|--------|-------------|---------------|
| Когда загружается | При запуске сессии | По запросу agent |
| Token cost | Фиксированный | Variable (только нужное) |
| Reliability | Высокая (гарантированно в контексте) | Зависит от quality retrieval |
| Suitable for | Малый объём (<500 tokens) | Большой объём (>1000 tokens) |
| Риск | Wasted tokens если не используется | Missed context если retrieval fails |

Anthropic рекомендует **hybrid**: pre-load critical data + allow autonomous exploration [S2]. Для 200-строчного LEARNINGS.md (~3000-4000 tokens) pre-loading оптимален — JIT добавляет complexity без пропорционального benefit при таком объёме.

#### 4.3.4. Hook-based enforcement

| Hook Event | Trigger | Knowledge Application | Overhead |
|------------|---------|----------------------|----------|
| SessionStart | startup, resume, clear, **compact** | Re-inject critical rules после compaction | ~750 tokens/50 turns [S6] |
| PreCompact | Before compaction (manual/auto) | Instructions что сохранить | Minimal |
| UserPromptSubmit | Каждый промпт | Per-turn reinforcement | ~15 tokens/turn [S6] |
| PostToolUse | После tool execution | Capture results для memory | Variable [S10] |

**SessionStart fires on compact** [S13] — values автоматически восстанавливаются после compaction. PreCompact inject instructions для compactor. Двухуровневая защита: если PreCompact ignored, SessionStart восстановит rules.

stdout from hooks becomes context без "may or may not be relevant" framing [S6, S13] — чистый injection path.

**Async hooks** (с Jan 2026) позволяют non-blocking operations [S13].

**Важно для bmad-ralph:** hooks lifecycle в non-interactive `claude --print` mode требует проверки — hooks may or may not fire при programmatic invocation.

#### 4.3.5. Sub-agent architecture

Sub-agents решают context pollution: focused sub-agents работают в чистых windows, возвращают condensed summaries (1000-2000 tokens) lead agent [S2].

bmad-ralph уже реализует этот паттерн: bridge → execute → review sessions. Knowledge extraction через resume-extraction (`claude -p`) — ещё один sub-agent. Каждый видит focused context, knowledge files обеспечивают inter-agent communication.

### 4.4. Third-Party and Comparative Approaches

#### 4.4.1. GitHub Copilot Agentic Memory

Наиболее зрелая production-grade система с количественными данными [S3, S18]:

**Структура:** repo-scoped memories: `{subject, fact, citations, reason}` [S3].

**Citation-based JIT verification [S3, S18]:**
1. Agent встречает stored memory
2. Проверяет citations против текущего codebase (simple read operations)
3. Valid → memory используется; Invalid → memory отбрасывается или обновляется
4. No significant latency overhead

**Self-healing:** adversarial memories (deliberately injected false facts) → agents consistently detected contradictions → memory pool self-healed [S3].

**28-дневный TTL [S18]:** memories удаляются через 28 дней. Если validated и использована — refreshed. "Natural selection" для memories.

**Измеренный impact:**

| Метрика | Без memory | С memory | Improvement |
|---------|-----------|---------|-------------|
| PR merge rate | 83% | 90% | +7% (p < 0.00001) [S3] |
| Code review positive feedback | 75% | 77% | +2% [S3] |
| Cross-agent learning | Нет | Да | Qualitative [S3] |

**Ключевой insight для bmad-ralph:** citation-based validation — это чего не хватает текущему дизайну. Без validation mechanism, accumulated learnings могут содержать outdated rules, incorrect generalizations, или contradictions.

#### 4.4.2. claude-mem: Automatic Capture + Semantic Compression

claude-mem [S10] — плагин, реализующий автоматический knowledge capture:

**Pipeline:** PostToolUse hook captures tool outputs (1000-10000 tokens) → Agent SDK compresses to ~500-token semantic observations → categorized (decision, bugfix, feature, discovery) → SQLite storage с full-text + vector search.

**Progressive disclosure (3-layer retrieval) [S10]:**
1. Search: compact index (~50-100 tokens per result)
2. Timeline: chronological context
3. Get observations: full details only for filtered IDs

**~10x token savings** по сравнению с full context loading [S10].

Паттерн progressive disclosure применим к bmad-ralph: вместо загрузки полного LEARNINGS.md, inject summary + tool для retrieval деталей.

#### 4.4.3. Claudeception и Continuous-Learning

Claudeception [S9] — autonomous skill extraction из session transcripts. Continuous-learning [S11] — in-context extraction с **timing advantage**: extraction происходит пока контекст ещё доступен, до compaction. В bmad-ralph review session уже имеет timing advantage: Claude видит findings in-context.

**Convergent evolution:** три независимых проекта (claude-mem, Claudeception, continuous-learning) пришли к одной архитектуре — capture → compress → inject on demand.

#### 4.4.4. Сравнительная таблица

| Аспект | Claude Code Native | GitHub Copilot | claude-mem | bmad-ralph (current) |
|--------|-------------------|----------------|-----------|---------------------|
| Memory Model | File-based hierarchy [S4] | Structured DB [S3] | SQLite + tags [S10] | File-based + budget |
| Capture | Manual / auto [S4] | Agent-triggered [S3] | Auto (hook) [S10] | Agent after execute/review |
| Compression | Manual editing | TTL natural selection [S18] | ~10x semantic [S10] | `claude -p` distillation |
| Validation | None | Citation JIT check [S3] | None | **None** (gap) |
| Enforcement | CLAUDE.md + hooks [S6, S13] | Built-in pipeline [S18] | Hook-based [S10] | Prompt injection only |
| Cross-session | Да (files) | Да (shared pool) [S3] | Да (SQLite) | Да (LEARNINGS.md) |
| Quantified Impact | None published | 7% PR merge [S3] | None published | Pre-implementation |
| Max Budget | 200 lines MEMORY.md [S4] | 28-day rolling [S18] | Unlimited (compressed) | 200 lines LEARNINGS.md |

#### 4.4.5. Token Economics

| Approach | Per-Session Token Cost | Coverage | Trade-off |
|----------|----------------------|----------|-----------|
| CLAUDE.md only (~30 строк) | ~500 tokens | Global rules | Minimal overhead, limited knowledge |
| LEARNINGS.md pre-load (200 строк) | ~3000-4000 tokens | Historical patterns | Fixed cost, may be irrelevant |
| claude-mem progressive disclosure | ~200-1500 tokens (variable) | Task-relevant | Low waste, requires infra [S10] |
| Copilot JIT memory | ~100-500 tokens (variable) | Validated, relevant | Lowest waste, requires DB [S3] |
| bmad-ralph proposed (LEARNINGS + CLAUDE.md) | ~4000-5000 tokens | Patterns + operational | Moderate, predictable, no infra |

### 4.5. Implications for bmad-ralph Knowledge System

#### 4.5.1. Current design assessment

Epic 6 design:
- **LEARNINGS.md:** 200-line hard budget, append-only between distillations, loaded into every prompt (FR29)
- **CLAUDE.md `## Ralph operational context`:** managed section, loaded eager
- **Triggers:** after execute cycles (resume-extraction), after review cycles (on findings)
- **Distillation:** `claude -p` session при превышении budget, с backup

**Evidence-based confidence assessment:**

| Паттерн | Confidence | Обоснование | Риск |
|---------|------------|-------------|------|
| 200-line hard budget | **Высокая** | Context rot: compact > full [S5]; Auto memory = 200 lines [S4]; ~150-200 instruction limit [S14] | Потеря при aggressive distillation |
| External file persistence | **Высокая** | Anthropic harnesses: git + progress files [S1] | Файлы устаревают без TTL |
| Pre-loading 200 строк | **Высокая** | Hybrid approach: pre-load critical [S2]; 3000-4000 tokens = within sweet spot | Wasted если не используется |
| Sub-agent distillation | **Высокая** | Clean context для compression [S2] | Quality non-deterministic |
| Citation validation | **Средняя** | Copilot: 7% improvement [S3]; разная архитектура | Not directly transferable |
| Hook-based re-injection | **Средняя** | Bypasses framing [S6, S13]; known bug с compact matcher | Hooks в non-interactive mode? |
| Topic sharding | **Средняя** | Auto memory topic files [S4]; SKILL.md split [S12] | Complexity для 200-line budget |
| Autonomous skill extraction | **Низкая** | Claudeception [S9]; no published metrics | Unquantified |
| Shuffled content order | **Низкая** | Context rot finding [S5] | Counterintuitive, human readability |

#### 4.5.2. What works and what concerns

**Подтверждено evidence:**

1. **Explicit knowledge files > compaction reliance.** Anthropic: "compaction alone is NOT sufficient" [S1]. LEARNINGS.md + CLAUDE.md section — правильный подход.
2. **200-line budget обоснован.** Совпадает с auto memory limit [S4] и instruction following ceiling [S14].
3. **Distillation как compression** соответствует capture → compress паттерну [S9, S10, S11].
4. **Pre-loading для 200 строк** — within JIT/pre-load sweet spot [S2].
5. **Separate extraction sessions** — sub-agent pattern с clean context [S2].
6. **Review-triggered extraction** — timing advantage (in-context knowledge) [S11].

**Вызывает concern:**

1. **Dangerous feedback loop.** Больше ошибок → больше learnings → context rot [S5] → больше ошибок. Distillation breaks loop через compression, но не через quality gate. Compressed LEARNINGS.md может быть compact but wrong.

2. **Отсутствие citation validation.** В отличие от Copilot [S3], LEARNINGS.md не имеет механизма проверки актуальности. Правило из Story 1.8 может стать irrelevant после refactoring, но останется в LEARNINGS.md.

3. **Monolithic LEARNINGS.md.** Все знания в одном файле для каждого prompt — и testing rules, и error handling, и CLI wiring. Larger context = worse performance [S5]. Topic-based sharding (аналог .claude/rules/ [S4]) загружает только relevant knowledge.

4. **CLAUDE.md framing problem.** LEARNINGS.md content, injected через prompt assembly, наследует "may or may not be relevant" framing [S6]. Hooks обходят это [S6, S13], но bmad-ralph invoke-ит Claude через `--print`/`--resume` — hooks behavior в этих modes не подтверждён.

5. **No quality metrics.** Нет механизма measurement: помогают ли learnings? Без метрик невозможно отличить valuable knowledge от noise.

#### 4.5.3. Structural recommendations

**Atomized facts over narrative.** Context rot research [S5] показывает: shuffled (atomized) контекст работает лучше organized narrative. LEARNINGS.md должен содержать independent facts, а не связное повествование:

Плохо (narrative):
```
During Story 1.8, we discovered that json.Unmarshal cannot distinguish truncated
JSON from non-JSON, because both fail the same way. This led us to accept fallback
behavior and document the deviation from spec.
```

Хорошо (atomized):
```
## json.Unmarshal: truncated vs non-JSON indistinguishable [Story 1.8, session/parse.go]
Both fail identically. Accept fallback behavior, document deviation.
```

Atomized format: shorter (fewer tokens), independently retrievable, resistant to "lost in the middle" [S5].

**Citation metadata.** Каждый learning сохраняется с source reference:

```
## Testing: always test zero values [Story 1.2, config/errors.go:42]
ExitCodeError{} and GateDecision{} — test zero-value behavior catches uninitialized field bugs.
```

При distillation, citation проверяется: файл ещё существует? Строка изменилась? Если да — learning marked as potentially stale [S3].

**Topic-based structure within LEARNINGS.md.** Topic headers (`## category: area`) позволяют distillation agent группировать и merge entries:

```markdown
## config: yaml parsing
- yaml.v3 #395: use map[string]any probe before struct unmarshal [Story 1.3]

## testing: error assertions
- errors.As over type assertions [Story 1.7, runner/errors.go]
- Always verify error message content, not just err != nil [Story 1.5]
```

Разнородные, тематически разделённые записи эффективнее monolithic text [S5]. В Growth phase: split на topic files (`learnings/testing.md`) с index в LEARNINGS.md [S2, S4].

**Hook-based enforcement для critical rules.** SessionStart hook inject top-N critical rules:

```json
{
  "hooks": {
    "SessionStart": [{
      "command": "cat knowledge/CRITICAL_RULES.md",
      "timeout": 1000
    }],
    "PreCompact": [{
      "command": "echo 'PRESERVE: critical learnings must survive compaction'",
      "timeout": 1000
    }]
  }
}
```

SessionStart fires on startup, resume, clear, AND compact [S13] — двухуровневая защита. Cost: ~750 tokens/50 turns [S6] — negligible.

**Pre-requisite:** проверить hooks lifecycle в non-interactive `claude --print`/`--resume` invocation mode.

#### 4.5.4. Distillation risks and mitigations

Distillation через `claude -p` — LLM-based compression. Risks:

1. **Information loss.** LLM может "решить" что rule unimportant. Mitigation: backup (Story 6.3) + diff review.
2. **Semantic drift.** "Always use errors.As" → "Prefer errors.As when possible" — subtle meaning change. Mitigation: structured format с fixed fields (finding, source, file).
3. **Hallucination during compression.** LLM может "додумать" rule. Mitigation: post-distillation validation — каждый learning в output traceable к learning в input. Claude: lowest hallucination rate [S5], favorable для distillation.
4. **Loss of context.** Compressed rule теряет reasoning. Mitigation: citation preservation — source и file fields не сжимаются.

Distillation prompt constraints:
- Preserve all file:line citations
- Never remove entries added in last 3 sessions (recency protection)
- Merge entries with identical file references
- Output line count MUST be under budget
- If unable to compress below budget, truncate oldest entries (not random)

---

## 5. Risks and Limitations

### 5.1. Context rot — фундаментальное ограничение

Context rot [S5] — не проблема которую можно "решить", а constraint с которым нужно design. Implications:
- **Budget is not optional.** 200-line limit — approximation к attention capacity [S14]. Exceeding budget гарантирует снижение compliance.
- **More is not better.** Добавление "полезного" knowledge может снизить general performance за счёт context dilution [S5].
- **Position matters.** Critical rules должны быть в начале или конце промпта, не в середине [S5].

### 5.2. Auto memory accuracy

Auto memory и LEARNINGS.md — обе пишутся Claude автономно, без human validation [S4]. Agent может записать incorrect generalization, и она будет loaded в каждую следующую сессию.

### 5.3. Отсутствие benchmarks

Единственные количественные данные — от GitHub Copilot (7%) [S3], но Copilot architecture принципиально отличается. Нет A/B тестов для LEARNINGS.md effectiveness.

### 5.4. Architectural differences

Copilot = hosted agents, structured DB, multi-user. Claude Code / bmad-ralph = CLI-based, file storage, single-user. Citation validation pattern — transferable; абсолютные числа — нет.

### 5.5. Dangerous feedback loop

```
errors → learnings written → more context consumed →
less room for actual work → more errors → more learnings
```

Budget — circuit breaker, но low-quality distillation может создать second-order loop: bad learnings survive → agent learns wrong patterns.

### 5.6. Research limitations

| Ограничение | Impact | Mitigation |
|-------------|--------|------------|
| Copilot data from different architecture [S3] | 7% may not transfer | Directional, not absolute |
| No Claude Code effectiveness benchmarks | Cannot quantify impact | Measure internally |
| Community tools [S9, S10] — uncertain longevity | Patterns, not implementations | Extract patterns |
| Context rot research on retrieval tasks [S5] | Coding may differ | Conservative interpretation |

---

## 6. Recommendations for bmad-ralph Epic 6

### R1: Keep 200-line budget, add topic structure (Priority: HIGH)

Реализовать LEARNINGS.md с topic headers (`## category: area`) внутри 200-line budget. При Growth phase — split на topic files с index.

**Evidence:** Context rot: compact > full [S5]. Auto memory = 200 lines [S4]. ~150-200 instruction limit [S14]. Shuffled/diverse content > coherent blocks [S5].

**Действие для Epic 6:** Story 6.1 (KnowledgeWriter) — format convention для atomized facts с topic headers. Story 6.2 (Distillation) — preserve topic structure, merge within topics.

### R2: Add SessionStart hook for critical knowledge (Priority: HIGH)

SessionStart hook inject top-priority rules, bypassing CLAUDE.md framing. Fires on startup/resume/clear/compact.

**Evidence:** CLAUDE.md values destroyed by compaction [S6]. Hooks bypass framing [S6, S13]. Cost: ~750 tokens/50 turns [S6].

**Действие для Epic 6:** Pre-requisite check — hooks in `claude --print`/`--resume` mode. If supported: Story 6.5 hook generation. If not: include critical rules in prompt assembly with higher-priority placement.

### R3: Add lightweight file:line citations (Priority: MEDIUM)

Каждый learning содержит `[Story X.Y, file.go:line]`. При distillation — validate references, remove stale lessons.

**Evidence:** Copilot citation validation: 7% improvement, self-healing [S3, S18]. File references enable staleness detection.

**Действие для Epic 6:** Stories 6.6, 6.7 — extraction instructions include source references. Story 6.2 — distillation instructions preserve citations, flag invalid.

### R4: Implement distillation quality guard (Priority: MEDIUM)

Post-distillation validation: каждый learning traceable к input, citations preserved, no entries from last 3 sessions removed, output under budget.

**Evidence:** LLM compression is non-deterministic. Backup (Story 6.3) handles failure, not quality degradation.

**Действие для Epic 6:** Story 6.3 — add validation step after distillation, before overwrite. Simple: line count check + topic header preservation check.

### R5: Use atomized fact format (Priority: MEDIUM)

One-line header + one-line description. Independent facts, not narrative.

**Evidence:** Shuffled haystacks > organized [S5]. Fewer tokens per fact = more facts within budget. Independent facts resist "lost in the middle" [S5].

**Действие для Epic 6:** Format convention in Stories 6.2, 6.6, 6.7 prompts.

### R6: Instrument knowledge effectiveness (Priority: LOW)

Track: LEARNINGS.md line count, review findings count, distillation frequency, repeat error rate. Detect feedback loop.

**Evidence:** Dangerous feedback loop in architecture. No existing benchmarks [S3]. Self-measurement — единственный путь.

**Действие для Epic 6:** NFR14 log format — add `knowledge_lines_total` and `review_findings_count` fields.

### R7: Evaluate skills for stable knowledge (Priority: LOW)

Proven, stable patterns (not session-specific learnings, but architectural conventions) → SKILL.md format. Skills = 500-line budget, auto-invocation, low churn. Separates volatile learnings from stable knowledge.

**Evidence:** Skills architecture [S12]. Claudeception skill extraction [S9]. 500-line budget > 200-line LEARNINGS.md budget.

**Действие для Epic 6:** Evaluation item для post-Epic 6 iteration.

---

## Appendix A: Evidence Table

| ID | Title | Publisher | Date | Tier | Key Contribution |
|----|-------|-----------|------|------|------------------|
| S1 | Effective harnesses for long-running agents | Anthropic Engineering | 2025-11 | A | Two-agent architecture, compaction limitations |
| S2 | Effective context engineering for AI agents | Anthropic Engineering | 2025-09 | A | Context engineering, sub-agents, JIT retrieval |
| S3 | Building an agentic memory system for GitHub Copilot | GitHub Blog | 2025 | A | Citation validation, 7% improvement, self-healing |
| S4 | Manage Claude's memory | Claude Code Docs | 2026 | A | 6 memory types, 200-line limit, cascade |
| S5 | Context Rot: How Increasing Input Tokens Impacts LLM Performance | Chroma Research | 2025 | A | Universal degradation, 30-50% variance, 18 models |
| S6 | Your CLAUDE.md Instructions Are Being Ignored | DEV Community | 2025 | B | Hook enforcement, framing problem |
| S7 | Claude Code Best Practices: Memory Management | Code Centre | 2025 | B | Minimal CLAUDE.md, /clear strategy |
| S8 | Claude Code Best Practices | Anthropic Docs | 2026 | A | Official best practices |
| S9 | Claudeception - Autonomous Skill Extraction | GitHub/blader | 2025 | B | Session review, skill extraction |
| S10 | claude-mem plugin | GitHub/thedotmack | 2025 | B | Semantic compression, progressive disclosure, ~10x savings |
| S11 | Continuous-learning skill | GitHub/affaan-m | 2025 | B | In-context timing advantage |
| S12 | Extend Claude with skills | Claude Code Docs | 2026 | A | SKILL.md 500-line limit, split pattern |
| S13 | Claude Code hooks reference | Claude Code Docs | 2026 | A | 12 lifecycle events, SessionStart/PreCompact |
| S14 | Writing a good CLAUDE.md | HumanLayer Blog | 2025 | B | ~150-200 instruction limit |
| S15 | Cognee + Claude Agent SDK memory integration | Cognee | 2025 | B | External memory integration |
| S16 | Memory and Context Management in Claude Agent SDK | GitHub/bgauryy | 2025 | B | SDK memory patterns |
| S17 | Anthropic Agent SDK overview | Claude API Docs | 2026 | A | Official SDK |
| S18 | About agentic memory for GitHub Copilot | GitHub Docs | 2026 | A | Citation validation, 28-day TTL |
| S19 | Claude Skills and CLAUDE.md: practical 2026 guide | Gend.co | 2026 | B | Skills + CLAUDE.md synergy |
| S20 | Claude Code's Memory: Large Codebases | Thomas Landgraf | 2025 | B | Real-world experience |

## Appendix B: Sources

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
