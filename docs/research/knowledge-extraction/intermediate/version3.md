# Knowledge Extraction in Claude Code-Based Coding Agents: Patterns, Limitations, and Architectural Recommendations

**Версия:** 3.0
**Дата:** 2026-02-25
**Аудитория:** Разработчик bmad-ralph (Epic 6: Knowledge Management)
**Язык:** Русский (технические термины на английском)

---

## Executive Summary

- Управление знаниями в coding agents остаётся нерешённой инженерной проблемой: context rot универсален для всех 18 протестированных LLM, а деградация производительности при росте контекста составляет 30-50%+ [S5].
- Claude Code предлагает шестиуровневую memory hierarchy с жёсткими ограничениями: 200 строк MEMORY.md, ~150-200 инструкций в CLAUDE.md для устойчивого выполнения, разрушение инструкций при compaction [S4, S6, S14].
- GitHub Copilot продемонстрировал единственный количественно подтверждённый результат: citation-based validation даёт 7% рост merge rate PR (p < 0.00001) с self-healing архитектурой [S3, S18].
- Архитектура bmad-ralph (LEARNINGS.md 200 строк + distillation через `claude -p`) адресует правильную проблему (budget как circuit breaker), но нуждается в усилении: topic-based splitting, hook-based enforcement, и citation validation снизят риск dangerous feedback loop.
- Ключевой trade-off: pre-loading полного контекста (простота, предсказуемость) vs. JIT retrieval (token economy, сложность) — для 200-строчного бюджета pre-loading оптимален [S2].

---

## 1. Research Question and Scope

### Основной вопрос

Как эффективно извлекать, хранить и применять знания в coding agents, построенных на базе Claude Code, в контексте автономной оркестрации разработки?

### Подвопросы

1. Какие механизмы памяти предоставляет Claude Code нативно, и каковы их ограничения?
2. Как context rot влияет на эффективность knowledge injection?
3. Какие паттерны persistence знаний подтверждены практикой?
4. Как сравнимы подходы Claude Code, GitHub Copilot и third-party экосистемы?
5. Какие архитектурные рекомендации применимы к bmad-ralph Epic 6?

### Scope

- **Включено:** Claude Code ecosystem (native memory, hooks, skills), GitHub Copilot agentic memory (для сравнения), third-party extensions (claude-mem, Claudeception, continuous-learning), академические исследования context rot
- **Исключено:** Cursor internal architecture, не-LLM memory systems, generic RAG pipelines без привязки к coding agents
- **Период:** 2024-2026

### Методология

Источники классифицированы по Tier:
- **Tier A** (Anthropic official, GitHub official, peer-reviewed research): S1-S5, S8, S12, S13, S17, S18
- **Tier B** (авторитетные блоги, open-source проекты с документированным adoption): S6, S7, S9-S11, S14-S16, S19, S20

Tier C источники (анонимные посты, непроверяемые claims) исключены.

---

## 2. Key Findings

1. **Context rot универсален и неизбежен.** Все 18 протестированных моделей (включая GPT-4.1, Claude 4, Gemini 2.5) демонстрируют деградацию с ростом контекста. Claude показал наименьший уровень hallucination и наивысший abstention rate, но degradation pattern сохраняется [S5].

2. **Claude Code memory hierarchy — шестиуровневая, с жёсткими budget constraints.** 200 строк MEMORY.md загружаются в system prompt; CLAUDE.md выше cwd загружается при запуске; child directories — on demand. Это создаёт предсказуемую, но жёстко ограниченную систему [S4].

3. **CLAUDE.md инструкции подвержены systematic erosion.** Framing "may or may not be relevant" снижает приоритет; compaction уничтожает values; эффективный лимит ~150-200 инструкций [S6, S14]. Hook-based enforcement через SessionStart/UserPromptSubmit обходит эти ограничения [S6, S13].

4. **Compaction alone is insufficient для long-running agents.** Anthropic's собственное исследование показывает необходимость external artifacts: progress files, git history, structured feature lists [S1]. Structured note-taking (NOTES.md pattern) и sub-agent architecture дополняют compaction [S2].

5. **Citation-based validation — единственный количественно подтверждённый паттерн.** GitHub Copilot: 7% рост PR merge rate (90% vs 83%), self-healing через adversarial memory correction, 28-дневный TTL [S3, S18].

6. **Semantic compression достигает ~10x token savings.** claude-mem сжимает 1000-10000 token tool outputs до ~500-token semantic observations с progressive disclosure [S10].

7. **Autonomous skill extraction жизнеспособен, но не масштабирован.** Claudeception демонстрирует pattern extraction из сессий, но adoption и effectiveness metrics отсутствуют [S9, S11].

8. **500-строчный SKILL.md лимит и split pattern — official best practice.** Anthropic рекомендует progressive disclosure: SKILL.md как entry point, детали в подфайлах [S12].

9. **Hook lifecycle покрывает все критические моменты.** 12 events, включая SessionStart (startup/resume/clear/compact) и PreCompact, позволяют inject context и enforce rules на каждом boundary [S13].

10. **"Lost in the middle" effect подтверждён количественно.** Производительность LLM следует U-shaped curve: высокая для начала и конца контекста, низкая для середины. Shuffled context > coherent (counterintuitive) — т.к. разрушает ложные связи между adjacent tokens [S5].

---

## 3. Analysis

### 3.1. Claude Code Memory Architecture

Claude Code реализует шестиуровневую memory hierarchy, где каждый уровень имеет свои характеристики загрузки, persistence и scope [S4]:

| Уровень | Тип | Загрузка | Persistence | Scope | Budget |
|---------|-----|----------|-------------|-------|--------|
| 1 | Managed policy | Всегда | Anthropic-managed | Global | N/A |
| 2 | Project CLAUDE.md | При запуске (выше cwd) / on demand (ниже) | Git-versioned | Repository | ~100-200 строк рекомендовано [S14] |
| 3 | .claude/rules/*.md | По glob pattern match | Git-versioned | Path-specific | Per-file |
| 4 | User ~/.claude/CLAUDE.md | При запуске | User-local | All projects | ~30-100 строк [S7] |
| 5 | Local project settings | При запуске | .claude/settings.local.json | Repository | N/A |
| 6 | Auto memory (MEMORY.md) | Первые 200 строк при запуске; topic files on demand | User-local, per-project | Project | 200 строк hard limit |

**Ключевые архитектурные свойства:**

**Cascade loading.** CLAUDE.md файлы выше рабочей директории загружаются полностью при запуске сессии. Файлы в дочерних директориях загружаются on demand, когда Claude читает файлы в этих директориях [S4]. Это создаёт два радикально разных режима: root-level инструкции всегда в контексте, а context-specific инструкции появляются только при навигации.

**@import syntax.** Поддерживается импорт из CLAUDE.md с максимальной глубиной 5 уровней [S4]. Это позволяет модульную организацию, но создаёт риск: при глубоком импорте общий объём может превысить эффективный лимит attention.

**Auto memory architecture.** MEMORY.md действует как index — Claude записывает и читает файлы в `~/.claude/projects/<path>/memory/` директории, используя MEMORY.md для навигации [S4]. Topic files создаются автоматически, но первые 200 строк MEMORY.md — единственная часть, гарантированно загружаемая при каждом запуске.

**Rules directory.** `.claude/rules/*.md` файлы с glob patterns в frontmatter обеспечивают path-specific инструкции [S4, S13]. Это единственный механизм, позволяющий conditional loading без manual intervention.

**Comparison с Copilot memory:**

| Аспект | Claude Code Memory | GitHub Copilot Memory |
|--------|--------------------|-----------------------|
| Формат хранения | Markdown файлы (CLAUDE.md, MEMORY.md) | Structured records {subject, fact, citations, reason} [S3] |
| Валидация | Нет (доверие к содержимому) | Citation-based JIT verification [S3, S18] |
| Cross-agent sharing | Нет (per-session) | Да — shared memory pool между agents [S3] |
| TTL / Expiration | Нет (manual management) | 28-дневный auto-delete с renewal при использовании [S18] |
| Scope | Per-project, per-user | Per-repository [S18] |
| Self-healing | Нет | Adversarial memories auto-corrected при citation check failure [S3] |
| Quantified impact | Нет published data | 7% PR merge rate improvement (p < 0.00001) [S3] |

Этот comparison выявляет фундаментальное различие: Claude Code memory — это passive storage (файлы читаются как есть), а Copilot memory — active system с validation loop. bmad-ralph Epic 6 должен решить, какую модель адаптировать.

### 3.2. Context Rot and Attention Limitations

Исследование Chroma (2025) [S5] установило, что context rot — не баг конкретной модели, а фундаментальное свойство transformer attention:

**Масштаб тестирования.** 18 state-of-the-art моделей, 8 различных длин контекста, 11 позиций needle placement. Каждая комбинация тестировалась многократно для статистической значимости.

**Ключевые количественные результаты:**

| Метрика | Результат | Источник |
|---------|-----------|----------|
| Performance variance (compact vs full) | 30-50%+ | [S5] |
| "Lost in the middle" degradation | >30% drop vs begin/end positions | [S5] |
| Semantic ambiguity amplification | Линейный рост degradation с ростом similarity между needle и distractors | [S5] |
| Claude hallucination rate | Наименьший среди 18 моделей | [S5] |
| Claude abstention rate | Наивысший (предпочитает отказ ответу с низкой confidence) | [S5] |
| Shuffled vs coherent context | Shuffled показывает лучшую retrieval accuracy | [S5] |

**Counterintuitive finding: shuffled > coherent.** Перемешанный контекст даёт лучшую retrieval accuracy, чем семантически связный [S5]. Объяснение: coherent text создаёт ложные ассоциации между adjacent tokens, и модель "запутывается" в narrative flow вместо точного поиска target information. Это имеет прямое значение для LEARNINGS.md: разнородные, тематически несвязанные записи могут быть эффективнее, чем тщательно организованная иерархия.

**Практический лимит инструкций.** HumanLayer анализ [S14] показывает, что LLM надёжно следуют ~150-200 инструкциям в CLAUDE.md. Выше этого порога начинается selective attention — модель "выбирает" какие инструкции исполнять, и выбор непредсказуем.

**Framing problem.** CLAUDE.md injection в Claude Code содержит disclaimer "may or may not be relevant to your tasks" [S6]. Это system-level framing, которое:
- Снижает perceived authority инструкций
- Позволяет модели "рационализировать" игнорирование правил
- Не может быть отключено пользователем

Количественный impact framing не измерен, но поведенческий эффект подтверждён множеством независимых наблюдений [S6, S7, S14, S20].

**Compaction destruction.** При автоматической compaction (когда context window заполняется), compaction summarizer не отличает CLAUDE.md инструкции от обычного conversation context [S6]. Результат: values и rules paraphrased или dropped. Overhead: ~750 tokens на каждые ~50 turns для re-injection через hooks [S6].

**Импликации для bmad-ralph:**

LEARNINGS.md (200 строк) попадает в zone наибольшей эффективности: compact context с высоким signal-to-noise ratio. Но при injection в prompt вместе с другими данными (sprint-tasks, review-findings, CLAUDE.md section) суммарный контекст растёт, и effectiveness LEARNINGS.md будет зависеть от его position в prompt assembly.

### 3.3. Knowledge Persistence Patterns

Anthropic's собственные исследования [S1, S2] и third-party ecosystem [S9-S11] выявили четыре основных паттерна persistence знаний. Каждый имеет свои trade-offs:

#### 3.3.1. Compaction + External Artifacts

**Источник:** Anthropic Engineering [S1]

Anthropic's harness для long-running agents использует two-agent architecture: initializer agent настраивает среду при первом запуске, coding agent делает incremental progress в каждой сессии [S1].

Критический insight: **compaction alone is insufficient.** Context summarization теряет critical details, и каждая новая сессия начинает "с чистого листа" [S1]. Решение — external artifacts:

| Artifact | Назначение | Persistence | Примечание |
|----------|-----------|-------------|------------|
| claude-progress.txt | Текущее состояние работы | Файловая система | "Memory" для следующей сессии [S1] |
| Git history | История изменений | Git | Используется для понимания "что было сделано" [S1] |
| Feature list JSON | Структурированный план | Файловая система | Checkpoint для progress tracking [S1] |

**Failure modes без external artifacts [S1]:**
- No decomposition: agent пытается реализовать всё в одном context window
- No documentation: следующая сессия не может понять предыдущую работу
- Premature completion: agent считает задачу завершённой при partial implementation

#### 3.3.2. Structured Note-Taking (NOTES.md Pattern)

**Источник:** Anthropic Context Engineering [S2]

Structured note-taking — это паттерн, где agent регулярно записывает notes в persistent storage вне context window [S2]. Ключевые принципы:

**Minimal high-signal tokens.** Контекст должен содержать минимум токенов с максимальным signal-to-noise ratio [S2]. Это означает aggressive filtering на этапе записи, не только при чтении.

**Compaction strategy [S2]:**
- Summarize conversation, сохраняя architecture decisions
- Retain 5 most recent files (working set)
- Discard intermediate reasoning steps

**JIT retrieval vs pre-loading [S2]:**

| Аспект | Pre-loading | JIT Retrieval |
|--------|-------------|---------------|
| Когда загружается | При запуске сессии | По запросу agent |
| Token cost | Фиксированный (всегда в контексте) | Variable (только нужное) |
| Reliability | Высокая (гарантированно в контексте) | Зависит от quality retrieval |
| Latency | Нулевая (уже в контексте) | Tool call overhead |
| Suitable for | Малый объём (<500 tokens) | Большой объём (>1000 tokens) |
| Риск | Wasted tokens если не используется | Missed context если retrieval fails |

Anthropic рекомендует **hybrid approach**: pre-load critical context (architecture decisions, current task state), use JIT для details (file contents, documentation) [S2].

#### 3.3.3. Sub-Agent Architecture

**Источник:** Anthropic Context Engineering [S2]

Sub-agents решают проблему context pollution: lead agent координирует, а focused sub-agents работают в чистых context windows [S2].

Каждый sub-agent может использовать десятки тысяч токенов для exploration, но возвращает **condensed summaries (1000-2000 tokens)** [S2]. Это natural compression: вся работа sub-agent конденсируется в actionable result.

**Relevance для bmad-ralph:** bmad-ralph уже использует sub-agent pattern — execute session и review session работают в отдельных context windows, а runner координирует. Knowledge extraction через resume-extraction — это по сути sub-agent, чья задача — condensed summary неудачи.

#### 3.3.4. Hook-Based Enforcement

**Источник:** Claude Code Docs [S13], DEV Community [S6]

Hooks — единственный механизм, **гарантирующий** injection context без disclaimer framing [S6]:

| Hook Event | Когда | Применение для knowledge | Примечание |
|------------|-------|--------------------------|------------|
| SessionStart | Startup, resume, clear, **compact** | Re-inject critical rules и knowledge | Fires на КАЖДЫЙ context reset [S13] |
| UserPromptSubmit | Каждый промпт пользователя | Reinforce critical rules | ~750 tokens overhead per 50 turns [S6] |
| PreCompact | Перед compaction (manual + auto) | Backup transcript, save state | `trigger` + `custom_instructions` available [S13] |
| PostToolUse | После tool execution | Capture tool results для memory | Used by claude-mem [S10] |
| Stop | Agent останавливается | Session review, skill extraction | Used by Claudeception [S9] |

**Critical detail:** SessionStart fires на **compact** event [S13]. Это означает, что после каждой compaction hook может re-inject knowledge, компенсируя compaction destruction. bmad-ralph может использовать это для re-injection LEARNINGS.md и operational context.

**Known issue:** SessionStart hook с `compact` matcher — output может не попасть в контекст (GitHub issue #15174) [S13]. Workaround: добавить critical rules напрямую в CLAUDE.md, а hooks использовать для supplementary context.

### 3.4. Third-Party and Comparative Approaches

#### 3.4.1. GitHub Copilot Agentic Memory

GitHub Copilot реализует принципиально иную архитектуру памяти, заслуживающую детального анализа [S3, S18].

**Формат memory record:**

```json
{
  "subject": "repo-specific-topic",
  "fact": "concrete observation about codebase",
  "citations": ["file:line-range references"],
  "reason": "why this memory was created"
}
```

**Citation-based JIT verification [S3, S18]:**
1. Agent встречает stored memory
2. Проверяет citations против текущего codebase (simple read operations)
3. Если citations valid — memory используется
4. Если citations invalid — memory отбрасывается или обновляется
5. No significant latency overhead

**Self-healing mechanism [S3]:**
- Adversarial memories (противоречащие codebase) seeded для тестирования
- Agents consistently обнаруживали contradictions через citation check
- Corrected versions автоматически сохранялись
- Memory pool "самоочищался" без manual intervention

**28-дневный TTL [S18]:**
- Memories удаляются через 28 дней
- Если memory validated и использована — создаётся новая с тем же содержимым
- Это "natural selection" для memories: полезные выживают, бесполезные expire

**Количественные результаты [S3]:**

| Метрика | Без memory | С memory | Improvement |
|---------|-----------|---------|-------------|
| PR merge rate | 83% | 90% | +7% (p < 0.00001) |
| Code review positive feedback | baseline | +2% | Статистически значимо |
| Cross-agent learning | Нет | Да | Qualitative improvement |

**Важное ограничение:** Эти результаты получены в архитектуре Copilot (hosted agents, structured memory store), а не в CLI-based workflow. Direct transferability к Claude Code / bmad-ralph не гарантирована.

#### 3.4.2. claude-mem: Automatic Capture + Semantic Compression

claude-mem [S10] — третья архитектура, принципиально отличная и от Claude Code native, и от Copilot:

**Capture pipeline:**
1. PostToolUse hook captures tool outputs (1000-10000 tokens)
2. Agent SDK compresses to ~500-token semantic observations
3. Observations categorized: decision, bugfix, feature, refactor, discovery, change
4. Tagged с concepts и file references
5. Stored в SQLite с full-text search

**Progressive disclosure (3-layer retrieval) [S10]:**
1. **Search:** Compact index с IDs (~50-100 tokens per result)
2. **Timeline:** Chronological context around interesting results
3. **Get observations:** Full details только для filtered IDs (~500-1000 tokens)

**Token economics:** ~10x savings по сравнению с full context loading [S10].

**Сравнительная таблица трёх подходов:**

| Аспект | Claude Code Native | GitHub Copilot | claude-mem |
|--------|-------------------|----------------|-----------|
| Capture | Manual (CLAUDE.md, MEMORY.md) | Automatic (agent-triggered) [S3] | Automatic (hook-based) [S10] |
| Storage | Markdown файлы | Structured records + DB [S3] | SQLite + semantic tags [S10] |
| Retrieval | Full file load (pre-load) | JIT с citation check [S3, S18] | Progressive disclosure [S10] |
| Compression | Manual / distillation | Natural (28-day TTL) [S18] | Semantic (~10x) [S10] |
| Validation | None | Citation-based [S3] | None (trust content) |
| Cross-session | Да (file persistence) | Да (shared pool) [S3] | Да (SQLite) [S10] |
| Dependencies | None | GitHub infrastructure [S18] | Agent SDK, SQLite [S10] |
| Maturity | Production (Anthropic official) | Public preview (Jan 2026) [S18] | Community project [S10] |

#### 3.4.3. Claudeception: Autonomous Skill Extraction

Claudeception [S9] реализует идею из академических работ Voyager (Wang et al., 2023) и CASCADE (2024) — автономное создание reusable skills из coding sessions.

**Trigger conditions [S9]:**
- Agent завершил debugging с non-obvious решением
- Найден workaround через investigation / trial-and-error
- Resolved error с неочевидной root cause
- Обнаружены project-specific patterns через investigation

**Extraction criteria [S9]:**
- Знание требовало actual discovery (не просто чтение документации)
- Поможет с future tasks
- Имеет clear trigger conditions
- Verified to work

**Hook integration:** Stop event используется для evaluation каждой сессии на extractable knowledge [S9].

**Ограничения:** Нет published effectiveness metrics. Community adoption невелик. Extraction quality зависит от модели, а не от deterministic rules.

#### 3.4.4. Continuous-Learning Skill Pattern

Affaan-m continuous-learning pattern [S11] демонстрирует in-context approach: session review с pattern extraction и skill creation внутри active session.

**Timing advantage:** Extraction происходит пока весь контекст сессии ещё доступен, до compaction [S11]. Это минимизирует information loss.

**Limitations:** Потребляет tokens из рабочей сессии. Нет separation of concerns — extraction конкурирует с coding work за context window.

### 3.5. Implications for bmad-ralph Knowledge System

#### 3.5.1. Current Design Assessment

Текущий дизайн bmad-ralph Epic 6:

| Компонент | Дизайн | Оценка |
|-----------|--------|--------|
| LEARNINGS.md | 200-line hard limit, append + distillation | **Правильно** — hard budget как circuit breaker [S2, S5] |
| Distillation | `claude -p` single-shot session при превышении | **Правильно** — sub-agent pattern с clean context [S2] |
| CLAUDE.md section | `## Ralph operational context` — isolated section | **Правильно** — separation от project instructions [S4] |
| Trigger | After execute/review cycles | **Правильно** — timing совпадает с continuous-learning [S11] |
| Budget | 200 строк (~3000-4000 tokens) | **Правильно** — within pre-load sweet spot [S2, S5] |

**Risk: "Dangerous feedback loop"** — из project-context.md:
> больше ошибок -> больше learnings -> меньше context -> больше ошибок

Этот риск реален и подтверждён context rot research [S5]. Mitigations:

1. **Hard budget (уже в дизайне)** — circuit breaker предотвращает unbounded growth
2. **Distillation quality** — если distillation agent некачественно сжимает, critical patterns теряются
3. **Backup before distillation (уже в дизайне, Story 6.3)** — safety net

#### 3.5.2. Evidence-Based Assessment: What Will Work

**Высокая уверенность (подтверждено Tier A источниками):**

| Паттерн | Обоснование | Риск |
|---------|-------------|------|
| Hard line budget | Context rot research: compact > full [S5] | Потеря важных learnings при aggressive distillation |
| External file persistence | Anthropic harnesses: git + progress files [S1] | Файлы могут устареть без TTL |
| Pre-loading 200 строк | Hybrid approach: pre-load critical, JIT details [S2] | Wasted tokens если session не использует learnings |
| Sub-agent distillation | Clean context для compression task [S2] | Distillation quality не гарантирована |

**Средняя уверенность (подтверждено Tier B или Copilot data):**

| Паттерн | Обоснование | Риск |
|---------|-------------|------|
| Citation-like validation | Copilot: 7% improvement [S3] | Разная архитектура; не прямо transferable |
| Hook-based re-injection | Bypasses CLAUDE.md framing [S6, S13] | Known bug с compact matcher [S13] |
| Topic file splitting | SKILL.md 500-line pattern [S12] | Complexity overhead для 200-строчного бюджета |
| Semantic categorization | claude-mem type tags [S10] | Requires additional tooling |

**Низкая уверенность (unquantified, community-only):**

| Паттерн | Обоснование | Риск |
|---------|-------------|------|
| Autonomous skill extraction | Claudeception concept [S9] | No published metrics |
| In-session extraction | Continuous-learning timing [S11] | Competes for context |
| Shuffled content order | Context rot finding [S5] | Counterintuitive, may reduce human readability |

#### 3.5.3. Structural Recommendations

**Recommendation 1: Topic-based LEARNINGS.md с flat structure.**

Вместо монолитного LEARNINGS.md, использовать topic headers:

```markdown
## config: yaml parsing
- yaml.v3 #395: use map[string]any probe before struct unmarshal

## testing: error assertions
- errors.As over type assertions — future-proof for wrapping changes
- Always verify error message content, not just err != nil
```

Обоснование: контекст rot research показывает, что разнородные, тематически разделённые записи могут быть эффективнее monolithic text [S5]. Topic headers помогают distillation agent группировать и merging related entries.

**Recommendation 2: Lightweight citation в LEARNINGS.md.**

Адаптация Copilot citation pattern [S3] для file-based workflow:

```markdown
## runner: git operations
- HasNewCommit must check exit code, not just stdout [runner/git.go:47]
```

`[file:line]` reference позволяет distillation agent верифицировать relevance: если файл изменился — lesson может быть устаревшим. Это не полный JIT validation (как Copilot), но лучше чем ничего.

**Recommendation 3: Hook-based enforcement для critical operational rules.**

Вместо полагания только на CLAUDE.md `## Ralph operational context`, дублировать top-5 critical rules через SessionStart hook [S6, S13]:

```json
{
  "hooks": {
    "SessionStart": [{
      "type": "command",
      "command": "cat /path/to/.ralph/operational-rules.txt"
    }]
  }
}
```

Обоснование: CLAUDE.md rules теряются при compaction [S6]. SessionStart fires на compact event, восстанавливая rules [S13]. Overhead: ~750 tokens/50 turns — приемлемо для 5-10 critical rules.

**Recommendation 4: Distillation prompt должен preserve topic structure.**

Distillation (Story 6.2) — это не просто "сжать текст". Prompt должен:
- Сохранить topic headers (для future grouping)
- Merge duplicate entries within topics
- Preserve file citations [file:line] для validation
- Target ~100 строк (50% бюджета) — оставить headroom для новых learnings
- Prioritize recent entries (recency bias соответствует U-shaped attention [S5])

**Recommendation 5: Separate operational context от learnings.**

| File | Content | Loading | Budget |
|------|---------|---------|--------|
| LEARNINGS.md | Pattern discoveries, error types, workarounds | Pre-load first 200 lines [S4] | 200 строк hard |
| CLAUDE.md ## Ralph | Current task state, architecture reminders | Pre-load always [S4] | ~50 строк (из CLAUDE.md budget) |
| .ralph/rules/*.md | Path-specific rules (e.g., test naming) | On demand via glob match [S4, S12] | Per-file, ~50 строк |

Это использует native Claude Code cascade [S4] вместо custom tooling.

---

## 4. Comparative Architecture Summary

### 4.1. Three Architectures Compared

| Dimension | Claude Code Native | GitHub Copilot | bmad-ralph (proposed) |
|-----------|-------------------|----------------|-----------------------|
| **Memory Model** | File-based hierarchy [S4] | Structured DB records [S3] | File-based + budget constraint |
| **Capture** | Manual / auto-memory [S4] | Agent-triggered [S3] | Agent-triggered after execute/review |
| **Compression** | Manual editing / none | TTL-based natural selection [S18] | `claude -p` distillation |
| **Validation** | None | Citation JIT check [S3] | File:line citation (lightweight) |
| **Enforcement** | CLAUDE.md + hooks [S6, S13] | Built-in agent pipeline [S18] | Hooks + CLAUDE.md section |
| **Feedback Loop Risk** | Low (manual management) | Low (self-healing) [S3] | **High** (automated extraction) |
| **Quantified Benefit** | None published | 7% PR merge improvement [S3] | N/A (pre-implementation) |
| **Max Memory Size** | MEMORY.md 200 lines [S4] | 28-day rolling window [S18] | LEARNINGS.md 200 lines |

### 4.2. Third-Party Extension Comparison

| Dimension | claude-mem [S10] | Claudeception [S9] | Continuous-learning [S11] | bmad-ralph Epic 6 |
|-----------|-----------------|--------------------|--------------------------|--------------------|
| **Trigger** | PostToolUse hook | Stop event | Manual / hook | Post-execute, post-review |
| **Extraction** | Automatic semantic | Discovery-based | Session review | Claude-driven in-session |
| **Storage** | SQLite + tags | SKILL.md files | SKILL.md files | LEARNINGS.md + CLAUDE.md section |
| **Retrieval** | Progressive disclosure | On demand (skill activation) | On demand | Pre-load |
| **Compression** | ~10x semantic [S10] | N/A (skill format) | N/A | Distillation `claude -p` |
| **Dependencies** | Agent SDK, SQLite | None | None | None (vanilla Claude Code) |
| **Maturity** | Community | Community | Community | Designed, pre-implementation |

### 4.3. Token Economics Comparison

| Approach | Per-Session Token Cost | Coverage | Trade-off |
|----------|----------------------|----------|-----------|
| CLAUDE.md only (30 строк) | ~500 tokens | Global rules only | Minimal overhead, limited knowledge |
| LEARNINGS.md pre-load (200 строк) | ~3000-4000 tokens | Historical patterns | Fixed cost, may be irrelevant to current task |
| claude-mem progressive disclosure | ~200-1500 tokens (variable) | Task-relevant only | Low waste, requires tool infrastructure [S10] |
| Copilot JIT memory | ~100-500 tokens (variable) | Validated, relevant | Lowest waste, requires DB infrastructure [S3] |
| Full context (no management) | Grows unbounded | Everything | Context rot guaranteed [S5] |
| bmad-ralph proposed | ~4000-5000 tokens (LEARNINGS + CLAUDE.md) | Historical patterns + operational rules | Moderate, predictable, no infrastructure |

---

## 5. Risks and Limitations

### 5.1. Fundamental Risks

**R1: Context rot is universal and unsolved [S5].**
Ни один из исследованных подходов не решает context rot. Все — mitigation strategies. 200-строчный budget bmad-ralph — это compression trade-off: guaranteed inclusion в context за счёт lossy compression.

**R2: Distillation quality is non-deterministic.**
`claude -p` distillation session использует LLM для compression. LLM может:
- Потерять critical pattern при merging "similar" entries
- Искажённо интерпретировать technical details
- Непредсказуемо приоритизировать entries

Mitigation: backup before distillation (Story 6.3), но human review distillation output не предусмотрен в current design.

**R3: Feedback loop amplification.**
Automated knowledge extraction создаёт cycle:
```
errors -> learnings written -> more context consumed ->
less room for actual work -> more errors -> more learnings
```
Budget — circuit breaker, но distillation может создать second-order loop: low-quality distillation -> bad learnings survive -> agent learns wrong patterns.

**R4: No validation mechanism.**
В отличие от Copilot [S3, S18], LEARNINGS.md не имеет validation. Записанный pattern может:
- Устареть (code changed since learning was written)
- Быть incorrect (agent misdiagnosed root cause)
- Быть too specific (applies only to one case, not general)

File:line citations (Recommendation 2) — partial mitigation, но не automated.

**R5: CLAUDE.md framing undermines injected knowledge.**
"May or may not be relevant" framing [S6] ослабляет authority LEARNINGS.md content injected через prompt template. Hook-based re-injection bypasses framing [S13], но prompt-injected content не bypasses.

### 5.2. Research Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| Copilot data from different architecture [S3] | 7% improvement may not transfer | Treat as directional, not absolute |
| No Claude Code-specific effectiveness benchmarks | Cannot quantify LEARNINGS.md impact | Measure internally after implementation |
| claude-mem, Claudeception — community projects [S9, S10] | Uncertain longevity and reliability | Extract patterns, not implementations |
| Context rot research on retrieval tasks [S5] | Coding tasks may differ from NIAH benchmarks | Conservative interpretation |
| Auto memory accuracy concerns [S4, S7] | Claude may write inaccurate auto-memories | bmad-ralph controls extraction, not auto-memory |

---

## 6. Recommendations for bmad-ralph Epic 6

### R1: Keep 200-line LEARNINGS.md budget (validated)

Evidence: Context rot degrades performance 30-50%+ with increasing context [S5]. Compact representation outperforms full context across all 18 models [S5]. Anthropic's own harness architecture uses compact external artifacts [S1].

**Действие:** Confirm 200-line hard limit в Story 6.1. Distillation target: 100 строк (50% headroom).

### R2: Implement topic-based structure within LEARNINGS.md

Evidence: SKILL.md 500-line limit uses progressive disclosure pattern [S12]. Shuffled/diverse content shows better retrieval than coherent blocks [S5].

**Действие:** Define topic header format (`## category: area`) в Story 6.1. Instruct distillation prompt (Story 6.2) to preserve and merge by topic.

### R3: Add lightweight file:line citations

Evidence: Copilot citation-based validation achieves 7% improvement with self-healing [S3, S18]. File references enable staleness detection.

**Действие:** Включить в extraction instructions (Stories 6.6, 6.7): при записи lesson добавлять `[file:line]` reference. Distillation prompt (Story 6.2) должен сохранять citations, но не fail если они absent.

### R4: Use hooks for operational rule enforcement

Evidence: CLAUDE.md values destroyed by compaction [S6]. SessionStart hooks fire on compact event, restoring context [S13]. Hook output bypasses "may or may not be relevant" framing [S6].

**Действие:** В Story 6.5 (Knowledge Loading), дополнить prompt-based injection hook-based re-injection для top-5 operational rules. Использовать `.claude/settings.json` hooks configuration.

### R5: Measure extraction quality post-implementation

Evidence: Нет published effectiveness benchmarks для Claude Code knowledge systems. Copilot measured impact (7% improvement [S3]).

**Действие:** После implementation Epic 6, track:
- LEARNINGS.md line count over time (should stabilize near budget)
- Distillation frequency (too often = extraction is too verbose)
- Repeat error rate (should decrease if learnings effective)
- Distillation diff quality (manual spot-check of backup vs distilled)

### R6: Implement distillation quality safeguards

Evidence: LLM-based compression is non-deterministic. Backup mechanism (Story 6.3) addresses failure, but not quality degradation.

**Действие:** Distillation prompt (Story 6.2) должен включать explicit constraints:
- Preserve all file:line citations
- Never remove entries added in last 3 sessions (recency protection)
- Merge entries with identical file references
- Output line count MUST be under budget
- If unable to compress below budget, truncate oldest entries (not random)

### R7: Evaluate topic file splitting for Growth phase

Evidence: Claude Code supports topic files in auto-memory [S4]. SKILL.md split pattern [S12]. claude-mem achieves 10x token savings with progressive disclosure [S10].

**Действие:** В MVP оставить monolithic LEARNINGS.md (simple, predictable). В Growth phase оценить split на topic files (e.g., `learnings/testing.md`, `learnings/config.md`) с index в LEARNINGS.md. Pre-load index, JIT load topics [S2].

---

## Appendix A: Evidence Table

| ID | Title | Publisher | Date | Tier | Key Contribution |
|----|-------|-----------|------|------|------------------|
| S1 | Effective harnesses for long-running agents | Anthropic Engineering | 2025-11 | A | Two-agent architecture, compaction limitations |
| S2 | Effective context engineering for AI agents | Anthropic Engineering | 2025-09 | A | Context engineering principles, sub-agents |
| S3 | Building an agentic memory system for GitHub Copilot | GitHub Blog | 2025 | A | Citation validation, 7% improvement |
| S4 | Manage Claude's memory | Claude Code Docs | 2026 | A | 6 memory types, 200-line limit |
| S5 | Context Rot | Chroma Research | 2025-07 | A | Universal degradation, 30-50% variance |
| S6 | Your CLAUDE.md Instructions Are Being Ignored | DEV Community | 2025 | B | Hook enforcement, framing problem |
| S7 | Claude Code Best Practices: Memory Management | Code Centre | 2025 | B | Minimal CLAUDE.md, /clear strategy |
| S8 | Claude Code Best Practices | Anthropic Docs | 2026 | A | Official best practices |
| S9 | Claudeception | GitHub/blader | 2025 | B | Autonomous skill extraction |
| S10 | claude-mem plugin | GitHub/thedotmack | 2025 | B | Semantic compression, progressive disclosure |
| S11 | Continuous-learning skill | GitHub/affaan-m | 2025 | B | In-context timing advantage |
| S12 | Extend Claude with skills | Claude Code Docs | 2026 | A | SKILL.md 500-line limit, split pattern |
| S13 | Claude Code hooks reference | Claude Code Docs | 2026 | A | 12 lifecycle events, SessionStart/PreCompact |
| S14 | Writing a good CLAUDE.md | HumanLayer | 2025 | B | ~150-200 instruction limit |
| S15 | Cognee + Claude Agent SDK | Cognee | 2025 | B | External memory integration |
| S16 | Memory and Context Management in Agent SDK | GitHub/bgauryy | 2025 | B | SDK memory patterns |
| S17 | Anthropic Agent SDK overview | Claude API Docs | 2026 | A | Official SDK |
| S18 | About agentic memory for GitHub Copilot | GitHub Docs | 2026 | A | Citation validation, 28-day TTL |
| S19 | Claude Skills and CLAUDE.md guide | Gend.co | 2026 | B | Skills + CLAUDE.md synergy |
| S20 | Claude Code's Memory: Large Codebases | Thomas Landgraf | 2025 | B | Real-world experience |

## Appendix B: Source URLs

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
