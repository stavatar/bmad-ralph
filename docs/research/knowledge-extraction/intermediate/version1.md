# Knowledge Extraction in Claude Code-Based Coding Agents: Patterns, Limitations, and Architectural Recommendations

**Исследовательский отчёт для проекта bmad-ralph**
**Дата:** 2026-02-25
**Версия:** 1.0 (draft)

---

## Executive Summary

- Управление знаниями в coding agents на базе Claude Code находится на раннем этапе: индустрия перешла от ad-hoc подходов (`CLAUDE.md` как «свалка правил») к структурированным системам с иерархией памяти, lifecycle hooks и citation-based validation, но зрелых решений с доказанной эффективностью пока единицы.
- **Context rot — универсальный и неустранимый феномен:** все 18 протестированных frontier моделей показывают деградацию производительности при росте контекста [S5]. Это означает, что стратегия «загрузить всё» гарантированно проигрывает стратегии «загрузить минимум релевантного».
- Наиболее эффективные паттерны: **citation-based validation** (GitHub Copilot, +7% merge rate) [S3], **hook-based enforcement** для обхода framing-проблемы CLAUDE.md [S6], **structured note-taking** с progress files для multi-session continuity [S1, S2], и **distillation как compression** для поддержания budget.
- Для bmad-ralph Epic 6 ключевые риски: dangerous feedback loop (больше ошибок → больше learnings → меньше context budget → больше ошибок), потеря critical rules при compaction [S6], и отсутствие механизма валидации accumulated knowledge.
- Рекомендации: topic-based sharding вместо монолитного LEARNINGS.md, citation-based validation по аналогии с Copilot, hook-based reinforcement для critical rules, и aggressive pruning с автоматическими метриками качества.

---

## 1. Research Question and Scope

### Основной вопрос

Как эффективно извлекать, хранить и применять знания в coding agents, построенных на Claude Code, в контексте автономной multi-session разработки?

### Подвопросы

1. Какие механизмы памяти предоставляет Claude Code и каковы их ограничения?
2. Как context rot влияет на эффективность накопленных знаний?
3. Какие паттерны knowledge persistence доказали эффективность (количественно или качественно)?
4. Как third-party решения (GitHub Copilot Memory, claude-mem, Claudeception) подходят к аналогичным проблемам?
5. Какие архитектурные решения оптимальны для knowledge system в bmad-ralph Epic 6?

### Scope

- **Временной диапазон:** 2024-2026 (период активного развития coding agents)
- **Фокус:** Claude Code ecosystem (основной), GitHub Copilot (для сравнения)
- **Исключения:** Non-LLM memory systems, внутренняя архитектура Cursor/Copilot (закрытая), чисто академические RAG-системы без практического применения в coding agents

---

## 2. Methodology

Исследование основано на анализе 20 источников, классифицированных по надёжности:

- **Tier A (10 источников):** Официальная документация Anthropic [S1, S2, S4, S8, S12, S13, S17], GitHub [S3, S18], академические исследования [S5]
- **Tier B (10 источников):** Инженерные блоги и open-source проекты с верифицируемыми утверждениями [S6, S7, S9, S10, S11, S14, S15, S16, S19, S20]

Критерии включения: наличие конкретных технических деталей или количественных данных. Tier C (анекдотические посты, неверифицируемые утверждения) исключены. При конфликте между источниками приоритет отдаётся Tier A.

---

## 3. Key Findings

1. **Claude Code реализует 6-уровневую иерархию памяти** с различными механизмами загрузки: от eager (CLAUDE.md — полностью при старте) до lazy (topic files — по запросу) [S4]. Это создаёт архитектурную возможность для tiered knowledge storage, но также усложняет предсказуемость того, какие знания реально окажутся в контексте.

2. **Context rot универсален и нелинеен.** Исследование Chroma (18 моделей, 8 длин контекста, 11 позиций needle) показало 30-50%+ degradation между compact (~300 tokens) и full (113k tokens) сценариями [S5]. Это фундаментальное ограничение делает budget management не optimization, а necessity.

3. **CLAUDE.md instructions систематически подрываются framing.** Содержимое CLAUDE.md оборачивается дисклеймером «may or may not be relevant», а при compaction summarize-ится вместе с low-priority контентом [S6]. Hook-based reinforcement через SessionStart и PreCompact обходит эту проблему [S6, S13].

4. **Compaction alone is NOT sufficient для multi-session work** — требуется explicit progress tracking через файлы (git history, progress.txt, feature list JSON) [S1]. Это подтверждает архитектурное решение bmad-ralph использовать LEARNINGS.md + CLAUDE.md section вместо надежды на compaction.

5. **Citation-based validation — единственный паттерн с количественно доказанной эффективностью:** GitHub Copilot Memory показал 7% increase в PR merge rates (90% vs 83%, p < 0.00001) [S3, S18]. Adversarial testing подтвердил self-healing: deliberately injected false memories обнаруживались и корректировались [S3].

6. **Frontier LLMs следуют ~150-200 инструкциям с reasonable consistency** [S14]. Это создаёт hard ceiling для объёма LEARNINGS.md — не только по строкам, но и по количеству distinct rules.

7. **Structured note-taking с progress files** (NOTES.md, claude-progress.txt) — проверенный паттерн для session continuity [S1, S2]. Ключевое: notes пишутся агентом, не оркестратором, что обеспечивает self-consistency context.

8. **Third-party tools демонстрируют convergent evolution** к semantic compression + progressive disclosure: claude-mem [S10], Claudeception [S9], continuous-learning [S11] независимо пришли к паттерну «capture → compress → inject on demand».

9. **Counterintuitive finding: shuffled/unstructured haystacks outperform coherent organized ones** в needle-in-haystack тестах [S5]. Это ставит под вопрос интуицию «хорошо организованные правила = лучше следование».

10. **Skills system (SKILL.md) предоставляет 500-line budget** с auto/manual invocation и split-паттерном для reference files [S12]. Это альтернативный механизм knowledge delivery, не использованный в текущем дизайне Epic 6.

---

## 4. Analysis

### 4.1. Claude Code Memory Architecture

Claude Code реализует шестиуровневую иерархию памяти, каждый уровень которой имеет свою семантику, scope и механизм загрузки [S4]:

| Уровень | Scope | Загрузка | Лимит |
|---------|-------|----------|-------|
| Managed policy | Anthropic-controlled | Всегда | N/A |
| Project memory (CLAUDE.md) | Репозиторий | Eager (при старте) | Нет hard limit |
| Project rules (.claude/rules/) | Файл/паттерн | Conditional (glob match) | Нет hard limit |
| User memory (~/.claude/CLAUDE.md) | Глобальный | Eager | Нет hard limit |
| Project local (CLAUDE.local.md) | Локальный | Eager | Нет hard limit |
| Auto memory (~/.claude/projects/) | Проект, per-user | MEMORY.md: первые 200 строк; topic files: по запросу | 200 строк (MEMORY.md) |

Архитектурно важные детали:

**Eager vs lazy loading.** CLAUDE.md файлы выше CWD загружаются полностью при старте сессии, а CLAUDE.md из дочерних директорий — по требованию (on-demand) [S4]. Это означает, что root-level CLAUDE.md гарантированно в контексте, а subdirectory-level — нет. Для bmad-ralph это критично: `## Ralph operational context` section в корневом CLAUDE.md загружается eager, что обеспечивает baseline knowledge в каждой сессии.

**Auto memory и 200-line budget.** MEMORY.md загружает первые 200 строк при старте, topic files (отдельные .md в auto memory directory) загружаются только когда Claude решает, что они релевантны текущей задаче [S4]. Это JIT-модель: система не предзагружает всё, а полагается на агента для retrieval decision. Для autonomous agent (без human guidance) это рискованно — агент может не «вспомнить» что ему нужно.

**@import и .claude/rules/.** Поддержка `@import` syntax с max depth 5 позволяет модульную организацию правил [S4]. .claude/rules/ поддерживает path-specific rules с glob patterns в YAML frontmatter [S4] — это механизм contextual filtering, где правила для `*.go` файлов загружаются только при работе с Go кодом. Этот паттерн напрямую применим для topic-based knowledge в bmad-ralph.

**Skills directory.** SKILL.md формат с 500-line limit, auto/manual invocation и возможностью split на reference files [S12] предоставляет ещё один delivery mechanism. Skills загружаются при match условия (описание задачи совпадает с skill description) или по explicit invocation. Для knowledge delivery это означает возможность packaging knowledge as skills — но текущий дизайн Epic 6 этот механизм не использует.

### 4.2. Context Rot and Attention Limitations

Исследование Chroma Research 2025 [S5] предоставляет наиболее систематические данные о деградации LLM производительности при росте контекста. Ключевые результаты:

**Универсальность деградации.** Все 18 протестированных frontier моделей (включая Claude, GPT-4, Gemini) показали ухудшение при увеличении контекста [S5]. Это не артефакт конкретной модели или архитектуры — это фундаментальное свойство transformer attention.

**Количественная оценка.** Разница между compact (~300 tokens) и full (113k tokens) сценариями составляет 30-50%+ по accuracy [S5]. Для coding agent это означает: правило, надёжно выполняемое при 5k tokens контекста, может игнорироваться при 100k tokens. bmad-ralph сессии, накапливающие tool outputs и code changes, легко достигают 100k+ tokens.

**«Lost in the middle» effect.** Модели лучше attend к началу и концу контекста, хуже — к середине [S5]. Для LEARNINGS.md, загружаемого в начало промпта, это относительно благоприятно. Но по мере роста промпта (sprint-tasks, code context, tool outputs) LEARNINGS.md «сдвигается» к середине.

**Semantic ambiguity compounds.** При низком сходстве между «needle» (инструкция) и «question» (текущая задача), деградация ещё круче [S5]. Абстрактные правила типа «всегда используй errors.As» менее уязвимы (высокое сходство с конкретным error handling кодом), чем контекстно-зависимые типа «в Story 1.8 обнаружено что truncated JSON...» (низкое сходство с новой задачей).

**Counterintuitive: shuffled outperforms organized.** Перемешанные, неструктурированные контексты показали лучшие результаты чем coherently organized [S5]. Гипотеза авторов: организованный текст создаёт attention shortcuts — модель «скользит» по знакомой структуре вместо внимательного чтения. Это контр-интуитивно для knowledge management, где структурирование считается best practice.

**~150-200 instruction limit.** Frontier LLMs могут следовать приблизительно 150-200 инструкциям с reasonable consistency [S14]. Для LEARNINGS.md с 200-line budget это означает: если каждая строка — отдельная инструкция, мы у ceiling. Реалистичнее 3-5 строк на инструкцию — значит budget ~40-60 distinct rules. Текущий CLAUDE.md bmad-ralph уже содержит ~50 правил в `## Testing` section, что приближается к этому лимиту без учёта LEARNINGS.md.

**Framing problem.** CLAUDE.md content оборачивается disclaimer-ом «may or may not be relevant to your tasks» [S6]. Этот фрейминг активно подрывает authority инструкций — модель может «решить», что правило нерелевантно текущей задаче. При compaction CLAUDE.md values summarize-ятся вместе с другим low-priority контентом, что может полностью удалить critical rules [S6]. Hook-based reinforcement через SessionStart обходит framing problem: hooks deliver clean system-reminder messages без disclaimers [S6, S13].

### 4.3. Knowledge Persistence Patterns

#### 4.3.1. Compaction: возможности и ограничения

Compaction в Claude Code — механизм сжатия истории разговора при приближении к context window limit [S2, S8]. При compaction: history summarize-ируется, architectural decisions сохраняются, redundant tool outputs discarded, 5 most recently accessed files сохраняются [S2].

Anthropic явно заявляет: **compaction alone is NOT sufficient для multi-session work** [S1]. Причины:
- Compaction оптимизирует для recency, не для importance — недавний tool output может вытеснить critical architectural decision
- Inter-session state теряется полностью — каждая новая сессия начинает с нуля
- Compaction не имеет domain knowledge чтобы отличить «эту ошибку мы уже исправили» от «это правило мы должны помнить всегда»

Для bmad-ralph это подтверждает правильность решения использовать explicit knowledge files (LEARNINGS.md, CLAUDE.md section) вместо reliance на compaction. Однако within-session compaction остаётся проблемой: если execute session с LEARNINGS.md в промпте делает compact, LEARNINGS.md content может быть summarized away.

#### 4.3.2. Structured note-taking

Anthropic рекомендует structured note-taking как primary persistence mechanism для long-running agents [S1, S2]:

- **Initializer + Coding agent architecture** [S1]: Initializer agent читает progress files и git logs, выбирает следующую задачу, writing detailed instructions. Coding agent выполняет одну конкретную задачу. Progress tracked через: git commit history, claude-progress.txt, feature list JSON, init.sh.
- **NOTES.md pattern** [S2]: агент пишет persistent notes с progress, objectives, insights. Notes пережидают session boundaries и compaction.
- **Failure modes** [S1]: full implementation at once (вместо single feature), undocumented half-implementations, premature completion declarations. Все три — следствие потери контекста о текущем прогрессе.

Для bmad-ralph: текущий дизайн Epic 6 где Claude inside session пишет в LEARNINGS.md — это вариация NOTES.md pattern. Ключевое отличие: в Anthropic-паттерне agent пишет notes для себя (self-consistency), в bmad-ralph один session (execute) пишет knowledge для другого session (следующий execute). Это создаёт additional challenge: writer не знает точно, какой контекст будет у reader.

#### 4.3.3. Just-in-time retrieval vs pre-loading

Context engineering framework [S2] описывает спектр стратегий:

- **Pre-loading (eager):** критические данные загружаются в промпт до начала работы. Простая, предсказуемая, но расходует budget на потенциально нерелевантную информацию.
- **Just-in-time (JIT):** lightweight identifiers в промпте + tools для загрузки данных по требованию. Экономит budget, но зависит от способности агента «вспомнить» что ему нужно.
- **Hybrid:** pre-retrieve critical data + allow autonomous exploration [S2]. Рекомендуемый подход.

bmad-ralph Epic 6 design использует pre-loading: LEARNINGS.md полностью загружается в каждый prompt [FR29]. Это safe default, но при росте LEARNINGS.md к 200-line budget, pre-loading становится expensive. Альтернатива: topic-based sharding (аналог auto memory topic files [S4]) с pre-loading только summary и JIT-retrieval для деталей.

#### 4.3.4. Hook-based enforcement

Claude Code Hooks [S13] предоставляют 12 lifecycle events: SessionStart, PreToolUse, PostToolUse, PreCompact и другие. Ключевые для knowledge management:

- **SessionStart:** fires on startup, resume, clear, AND compact [S13]. Это означает: values injected через SessionStart hook автоматически восстанавливаются после compaction — решение проблемы compaction destroys CLAUDE.md values [S6].
- **PreCompact:** receives trigger (manual/auto) + custom_instructions [S13]. Позволяет inject instructions о том, что НЕЛЬЗЯ summarize away при compaction.
- **stdout from hooks becomes context** that Claude sees [S13]. Hook может прочитать LEARNINGS.md и inject его как system-reminder при каждом SessionStart — обходя и framing problem, и compaction problem.

Практический пример из [S6]: hook-based motto reminder стоит ~750 tokens за 50+ turns при 200k budget — negligible cost. Для bmad-ralph: SessionStart hook может inject top-priority rules из LEARNINGS.md после каждого compact, обеспечивая persistence без framing disclaimer.

**Async hooks** доступны с января 2026 [S13], что позволяет non-blocking operations — например, background distillation trigger.

#### 4.3.5. Sub-agent architecture для context isolation

Context engineering [S2] описывает sub-agent pattern: специализированные агенты с focused context, condensed summaries (1000-2000 tokens) для lead agent. Anthropic long-running agents [S1] используют двухуровневую архитектуру: Initializer agent (setup, context gathering) + Coding agent (incremental progress).

bmad-ralph уже реализует вариацию этого паттерна: bridge session (аналог Initializer), execute session (coding), review session (quality). Knowledge extraction — ещё один sub-agent (resume-extraction session с `claude -p`). Каждый sub-agent видит только свой focused context, а knowledge files обеспечивают inter-agent communication.

### 4.4. Third-Party and Comparative Approaches

#### 4.4.1. GitHub Copilot Agentic Memory

Copilot Memory [S3, S18] — наиболее зрелая production-grade система с количественными данными. Архитектура:

- **Repository-scoped memories** со структурой: `{subject, fact, citations, reason}` [S3, S18]
- **Citation-based validation:** перед использованием memory, агент верифицирует что citations exist и code matches текущему состоянию [S3]. Это self-healing mechanism — устаревшие memories обнаруживаются и не применяются.
- **Cross-agent sharing:** memory от code review agent доступна coding agent и CLI [S3]
- **Результаты:** 7% increase в PR merge rates (90% vs 83%), p-value < 0.00001; 2% increase в positive code review feedback [S3]
- **Adversarial testing:** deliberately injected false memories → agents consistently detected and corrected [S3]. Memory pool self-heals through citation verification.

**Ключевой insight для bmad-ralph:** citation-based validation — это то, чего не хватает текущему дизайну LEARNINGS.md. Без механизма validation, accumulated learnings могут содержать outdated rules (code pattern изменился), incorrect generalizations (из конкретного bug сделан слишком широкий вывод), или contradictions (два правила конфликтуют). Citation позволяет агенту проверить: «это правило ещё актуально?» перед применением.

#### 4.4.2. claude-mem

claude-mem [S10] — плагин для Claude Code, реализующий автоматический capture knowledge:

- Перехватывает tool executions, compresses их в semantic summaries
- Vector + fulltext search для retrieval
- Progressive disclosure injection: сначала краткая summary, по запросу — полные детали
- Автоматический capture без explicit agent action

Паттерн progressive disclosure напрямую применим к bmad-ralph: вместо загрузки полного LEARNINGS.md (200 строк), inject summary (20-30 строк) + tool для retrieval деталей по конкретному topic.

#### 4.4.3. Claudeception

Claudeception [S9] реализует autonomous skill extraction:

- Отдельный Claude session reviews завершённые сессии
- Извлекает reusable patterns, saves как skills
- Continuous learning: каждая завершённая сессия может порождать новые skills

Это ближайший аналог bmad-ralph resume-extraction pattern (Story 6.6), но в Claudeception extraction выполняется над полным session transcript, а в bmad-ralph — над resumed session (Claude рассматривает предыдущую незавершённую сессию). Преимущество Claudeception: full context available для extraction. Преимущество bmad-ralph: extraction integrated в workflow (не post-hoc).

#### 4.4.4. Continuous-learning skill pattern

continuous-learning [S11] использует post-session extraction с ключевым timing advantage: всё ещё в context когда extraction происходит. Извлечённые паттерны сохраняются как skills в SKILL.md формате.

Общее наблюдение по third-party: **convergent evolution** к паттерну capture → compress → inject on demand. Три независимых проекта (claude-mem, Claudeception, continuous-learning) пришли к одной архитектуре, что indicates это natural fit для problem domain.

### 4.5. Implications for bmad-ralph Knowledge System

#### 4.5.1. Current design assessment

Epic 6 design [bmad-ralph]:
- **LEARNINGS.md:** 200-line hard budget, append-only between distillations, загружается в каждый prompt через strings.Replace (FR29)
- **CLAUDE.md `## Ralph operational context`:** section managed by ralph, loaded eager
- **Knowledge extraction triggers:** после каждого execute cycle (resume-extraction при failure или --always-extract), после review cycles с findings
- **Distillation:** отдельная `claude -p` сессия при превышении budget, с backup

**Что подтверждается evidence:**

1. **Explicit knowledge files > compaction reliance.** Anthropic прямо говорит compaction insufficient [S1]. LEARNINGS.md + CLAUDE.md section — правильный подход.
2. **200-line budget обоснован.** 200 строк ≈ 150-200 distinct instructions при разумной плотности, что совпадает с instruction following ceiling [S14]. Auto memory тоже использует 200-line limit для MEMORY.md [S4].
3. **Distillation как compression.** Паттерн «accumulate → compress when over budget» соответствует capture → compress из third-party tools [S9, S10, S11].
4. **Pre-loading LEARNINGS.md.** Для файла до 200 строк, pre-loading обоснован — JIT-retrieval добавляет complexity без пропорционального benefit при таком объёме.
5. **Separate extraction sessions.** Использование `claude -p` для distillation изолирует context — extraction agent не загрязнён предыдущей работой [S2].

**Что вызывает concern:**

1. **Dangerous feedback loop.** Больше ошибок → больше learnings → менее эффективный context (context rot [S5]) → больше ошибок. Без pruning механизма, LEARNINGS.md деградирует в noise. Distillation частично решает проблему (compression), но не решает quality — distillation сжимает контент, но не валидирует его.

2. **Отсутствие citation-based validation.** В отличие от Copilot Memory [S3], LEARNINGS.md не имеет механизма проверки актуальности правила. Правило «в Story 1.8 обнаружено что truncated JSON becomes fallback» может стать irrelevant после refactoring, но останется в LEARNINGS.md до manual cleanup.

3. **Monolithic LEARNINGS.md.** Все знания в одном файле загружаются в каждый prompt — и rules для error handling, и rules для testing, и rules для CLI wiring. Context rot research [S5] показывает: larger context = worse performance. Topic-based sharding (аналог .claude/rules/ с glob patterns [S4]) позволяет загружать только relevant knowledge.

4. **Framing problem для CLAUDE.md section.** Ralph operational context в CLAUDE.md получает «may or may not be relevant» disclaimer [S6]. Для critical operational rules (например, «не мутировать config в runtime») это undermines enforcement.

5. **No quality metrics.** Нет механизма для measurement: помогают ли accumulated learnings? Copilot измеряет merge rates [S3]. Без метрик невозможно отличить valuable knowledge от noise.

#### 4.5.2. Structural recommendations

**Topic-based sharding.** Вместо монолитного LEARNINGS.md, использовать directory с topic files (по аналогии с auto memory topic files [S4] и .claude/rules/ [S4]):

```
knowledge/
  testing.md          # testing patterns and pitfalls
  error-handling.md   # error wrapping conventions
  windows-wsl.md      # environment-specific knowledge
  SUMMARY.md          # 20-30 line digest, always loaded
```

SUMMARY.md pre-loaded в каждый prompt (аналог первых 200 строк MEMORY.md [S4]). Topic files loaded JIT или по glob match к текущей задаче. Budget распределяется: 50 строк SUMMARY.md + 150 строк topic file.

**Citation-based validation.** Каждый learning сохраняется со структурой:

```
- **Finding:** Always test zero values for custom error types
- **Source:** Story 1.2 review, ExitCodeError{} panic
- **File:** internal/runner/errors.go:42
- **Verified:** 2026-02-25
```

При загрузке (или distillation), citation проверяется: файл ещё существует? Строка 42 ещё содержит ExitCodeError? Если нет — learning marked as potentially stale [S3].

**Hook-based reinforcement.** Top-5 critical rules inject через SessionStart hook, обходя framing problem [S6, S13]. Cost: ~200 tokens при 200k budget — negligible [S6]. Rules restore после каждого compact [S13].

#### 4.5.3. Distillation as compression: risks and mitigations

Distillation (Story 6.3) — `claude -p` session получающая полный LEARNINGS.md и сжимающая его. Risks:

1. **Information loss.** LLM может «решить» что rule unimportant и удалить его. Mitigation: backup mechanism (уже в дизайне Story 6.3) + diff review.
2. **Semantic drift.** При перефразировании, meaning может shift. «Always use errors.As» → «Prefer errors.As when possible» — subtle but meaningful change. Mitigation: structured format с fixed fields (finding, source, file) сохраняет machine-parseable metadata даже при перефразировке прозы.
3. **Hallucination during compression.** LLM может «додумать» rule которого не было. Mitigation: post-distillation validation — check что каждый learning в output traceable к learning в input.
4. **Loss of context.** Compressed rule теряет context (почему правило появилось, в каком scenario). Mitigation: citation preservation — source и file fields не сжимаются.

Claude модели показывают lowest hallucination rates но highest abstention under ambiguity [S5] — это favorable для distillation (скорее откажется сжимать чем придумает новое).

#### 4.5.4. Hook-based enforcement for critical rules

Практическая architecture для bmad-ralph hooks (`.claude/hooks/`):

```json
{
  "hooks": {
    "SessionStart": [{
      "command": "cat knowledge/CRITICAL_RULES.md",
      "timeout": 1000
    }],
    "PreCompact": [{
      "command": "echo 'PRESERVE: knowledge/CRITICAL_RULES.md content must survive compaction'",
      "timeout": 1000
    }]
  }
}
```

SessionStart fires на startup, resume, clear, AND compact [S13] — values автоматически восстанавливаются. PreCompact inject instructions для compactor. Это двухуровневая защита: даже если PreCompact instruction ignored, SessionStart восстановит rules после compact.

Однако bmad-ralph запускает Claude через `session.Execute` с explicit prompt — hooks may or may not fire в этом контексте (зависит от того, использует ли `claude` CLI hook system при `--print` или `--resume` flags). Это требует проверки: hooks lifecycle in non-interactive mode.

---

## 5. Risks and Limitations

### 5.1. Context rot как фундаментальное ограничение

Context rot [S5] — не проблема которую можно «решить», а constraint с которым нужно design. Все стратегии knowledge management работают в рамках degrading attention. Implications:

- **Budget is not optional.** 200-line limit — не arbitrary number, а approximation к attention capacity [S14]. Exceeding budget гарантирует снижение compliance.
- **More is not better.** Добавление «полезного» knowledge может снизить general performance за счёт context dilution [S5]. Каждый learning должен justify своё место.
- **Position matters.** «Lost in the middle» effect [S5] означает: critical rules должны быть в начале или конце промпта, не в середине между sprint-tasks и code context.

### 5.2. Auto memory accuracy

Auto memory (MEMORY.md) пишется Claude автономно — без human validation [S4]. Это означает: agent может записать incorrect generalization, и она будет loaded в каждую последующую сессию. bmad-ralph аналогично: Claude inside execute/review session пишет в LEARNINGS.md — без validation layer.

### 5.3. Отсутствие comprehensive benchmarks

Единственные количественные данные — от GitHub Copilot (7% merge rate increase) [S3], но Copilot architecture принципиально отличается от Claude Code. Нет A/B тестов для: LEARNINGS.md with/without, distillation quality, hook-based reinforcement effectiveness. Все рекомендации основаны на first-principles reasoning и qualitative evidence.

### 5.4. Architectural differences Copilot vs Claude Code

GitHub Copilot Memory использует repository-scoped memories, maintained by GitHub infrastructure, shared across agents [S3, S18]. Claude Code memory — file-based, maintained by user/agent, no infrastructure layer. Citation-based validation в Copilot опирается на GitHub API для code verification — в Claude Code требуется custom implementation.

### 5.5. Dangerous feedback loop specifics

Механизм feedback loop в bmad-ralph:

1. Execute session делает ошибку → review находит finding → lesson written в LEARNINGS.md
2. LEARNINGS.md grows → context rot increases → next execute session менее effective
3. More errors → more findings → more lessons → LEARNINGS.md grows further

Distillation (Story 6.3) breaking этот loop через compression, но не через quality gate. Compressed LEARNINGS.md может быть compact but wrong — и цикл продолжается.

Mitigation: aggressive pruning (удаление lessons старше N sessions без re-occurrence), quality metrics (track whether specific learning reduced error rate), и hard ceiling не только на lines но и на distinct rules count.

---

## 6. Recommendations

### Рекомендация 1: Topic-based sharding с budget allocation

Заменить монолитный LEARNINGS.md на directory structure с topic files и summary. SUMMARY.md (30-50 строк) pre-loaded в каждый prompt. Topic files loaded по relevance (определяется по типу session: execute → testing + error-handling topics, review → quality + patterns topics). Total budget 200 строк распределяется: 50 SUMMARY + 150 topic. Это использует JIT-retrieval pattern [S2] и topic file mechanism из auto memory [S4].

### Рекомендация 2: Citation metadata для каждого learning

Каждый learning сохраняется с: source (story/session), file reference, date. При distillation (Story 6.2), prompt включает instruction: preserve citations, mark stale (file not found/changed) learnings для removal. Это lightweight adaptation of Copilot citation-based validation [S3] без infrastructure dependency.

### Рекомендация 3: Hook-based reinforcement для top-N rules

Выделить 5-10 critical rules в отдельный файл (CRITICAL_RULES.md). SessionStart hook inject их при каждом session start, включая после compaction [S13]. Cost: ~200-400 tokens из 200k budget = negligible [S6]. Requires: проверить что hooks fire в non-interactive `claude` invocation mode (Epic 6 pre-requisite).

### Рекомендация 4: Distillation с quality gate

После distillation (Story 6.3), добавить validation step: каждый learning в output traceable к learning в input (no hallucinations), citations verified (files exist), no contradictions (pairwise check). Validation может быть отдельная `claude -p` session или simple programmatic check (citation file existence via `os.Stat`).

### Рекомендация 5: Expiry mechanism для learnings

Каждый learning получает TTL (default: 10 sessions). Если learning не referenced (ни triggered ни used) за TTL — marked для removal при следующей distillation. Это prevents indefinite accumulation и addresses context rot [S5]. При re-occurrence (та же ошибка обнаружена снова) — TTL resets.

### Рекомендация 6: Metrics collection для feedback loop detection

Track per-session: количество findings в review, количество retries, LEARNINGS.md size. Если trend показывает рост findings при росте LEARNINGS.md — feedback loop detected, trigger aggressive pruning. Простая implementation: append line в log file, analysis при distillation.

### Рекомендация 7: Skills mechanism для stable knowledge

Proven, stable knowledge (не session-specific learnings, а architectural patterns) переместить из LEARNINGS.md в SKILL.md формат [S12]. Skills имеют 500-line budget, auto-invocation по task match, и split-паттерн для reference files. Это разделяет volatile knowledge (LEARNINGS.md, high churn) от stable knowledge (skills, low churn), уменьшая context dilution.

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
| S19 | Claude Skills and CLAUDE.md: a practical 2026 guide for teams | Gend.co | 2026 | B |
| S20 | Claude Code's Memory: Working with AI in Large Codebases | Thomas Landgraf | 2025 | B |

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
