# Пара 3: Ревью решений и альтернатив — Epic 6 v4

**Дата:** 2026-03-02
**Scope:** Все 28 решений (C1-C4, H1-H8, M1-M12, L1-L6), их взаимодействие, пропущенные альтернативы, сравнение с конкурентами
**Метод:** Adversarial review каждого решения: правильность, альтернативы, противоречия, сценарии отказа

---

## Критические проблемы (блокеры)

### [P3-C1] C1 (Claude writes directly) КОНФЛИКТУЕТ с 6 quality gates — [needs-formatting] НЕ замена reject

**Почему плохо:**
V4 отказался от pending-file pattern (рекомендация обоих независимых пар review) в пользу "Claude пишет напрямую, Go делает snapshot-diff и тегирует [needs-formatting]". Это решение воскрешает ровно ту проблему, которую review нашёл: gates G1-G6 из Story 6.1 становятся POST-HOC тегированием, а не фильтрацией. Ключевые последствия:

1. **[needs-formatting] entries = мусор в контексте.** До следующей дистилляции (которая может не произойти из-за cooldown 5 задач или skip пользователя) невалидные entries ИНЪЕКТИРУЮТСЯ в каждый промпт. Research [R1 S5]: context rot 30-50% при засорении. SFEIR: 15 правил = 94% compliance; каждый мусорный entry ухудшает compliance ВСЕХ правил.

2. **Snapshot-diff хрупок.** Claude может перезаписать LEARNINGS.md полностью (не append). Claude может переформатировать существующие записи. Claude может удалить записи. Diff между snapshot и current покажет все эти изменения как "новые записи" или пропустит удаления. Go-код не контролирует поведение Claude с файлом.

3. **Зачем 6 quality gates если они не фильтруют?** G1 проверяет формат — но [needs-formatting] entry всё равно остаётся. G3 проверяет дубликат — но дубликат всё равно остаётся. G5 cap 5 entries — но Claude уже написал 10. Единственное действие gates — добавить тег к уже записанным данным.

4. **Pending-file pattern решал 4 проблемы одновременно** (C1, C4, M2, thread safety). V4 выбрал вариант, который решает только C1 частично, а C4 (мёртвая ветка [needs-formatting]) создаётся заново.

**Почему пользователь выбрал этот вариант:** Вероятно, простота — Claude пишет прямо в файл, нет промежуточных шагов. Но простота реализации != простота эксплуатации.

**Опции исправления:**
- A) **Вернуть pending-file pattern** (рекомендация 2 из 5 пар). Claude пишет в `.ralph/pending-lessons.md`, Go парсит, gates фильтруют, валидные append в LEARNINGS.md. Чистый LEARNINGS.md ВСЕГДА.
- B) **Claude writes + Go rewrites.** После snapshot-diff Go УДАЛЯЕТ невалидные entries из LEARNINGS.md (а не тегирует). Отклонённые — в лог. Требует перезапись файла, но LEARNINGS.md остаётся чистым.
- C) **Оставить as-is**, но УБРАТЬ gates G1-G6 из кода (они бесполезны как post-hoc теги). Упростить до: Claude пишет, дистилляция чистит. Честнее архитектурно.

---

### [P3-C2] Замена circuit breaker на human gate — БЛОКИРУЕТ автономность

**Почему плохо:**
Весь проект bmad-ralph — про АВТОНОМНУЮ разработку ("Ralph Loop"). Human gate на КАЖДЫЙ failure дистилляции означает:

1. **Ночные прогоны невозможны.** `ralph run` с 50 задачами ночью — на 6-й задаче дистилляция упала — ждём утра? Это уничтожает ценность автономного агента. Circuit breaker (CB) позволял: fail → CB OPEN → продолжаем без дистилляции → попробуем позже.

2. **Failure probability высокая.** Дистилляция = LLM compression (claude -p). LLM output недетерминистичен. Validation имеет 8 критериев. Вероятность хотя бы одного failure за 20 задач нетривиальна. Каждый раз — ручное вмешательство.

3. **"retry 5-10 times" — bizarre UX.** Пользователь на human gate получает опции "retry once, retry 5-10 times, skip". Зачем пользователь выбирает количество retry? Это должна быть автоматическая логика с threshold.

4. **CB был ПРАВИЛЬНЫМ решением** review (обоих пар 2 и 4 независимо). V4 удалил его ради "контроля пользователя", но контроль уже был: `ralph distill` = manual override.

**Контраргумент:** "Пользователь хочет знать о проблемах." Да, но для этого есть логи, stderr warning, metrics. Не блокировка pipeline.

**Опции исправления:**
- A) **Вернуть CB + human gate как fallback.** CB OPEN/HALF-OPEN/CLOSED logic автоматический. При CB OPEN более 72 часов — stderr warning "consider `ralph distill`". Human gate ТОЛЬКО если пользователь запустил интерактивный `ralph run` (не batch/CI).
- B) **CB automatic + notification.** CB logic as designed by review. При OPEN — notify через stderr и DistillState. Без блокировки.
- C) **Конфигурируемо.** `distill_gate: human|auto|hybrid`. Default = auto (CB). Human — для тех, кто хочет контроль.

---

### [P3-C3] Injection Budget / Overflow НЕ решён в v4

**Почему плохо:**
V4 убрал circuit breaker (C2), убрал injection circuit breaker (из consolidated review C2), убрал FIFO, убрал archive. Что происходит когда LEARNINGS.md растёт до 600+ строк?

1. **Пользователь выбирает "skip" на human gate 3 раза** → файл 400, 500, 600 строк → КАЖДАЯ сессия получает 600 строк мусора + невалидных entries → compliance падает с 94% до 40-50% (SFEIR research).

2. **"300+ lines = 3-4% context, linear decay"** — это НЕВЕРНАЯ метрика (подробно разобрано в J1 review). Правильная метрика — instruction count для compliance. 600 строк = 150-200 правил = beyond threshold.

3. **V3 имел injection CB** (stop injecting at 3x budget). V4 убрал его. Нет НИКАКОГО автоматического механизма защиты от overflow.

**Опции исправления:**
- A) **Вернуть injection circuit breaker** из консолидированного ревью. При lines > 3x budget: `__LEARNINGS_CONTENT__` = empty. Запись продолжается. Self-healing при distillation.
- B) **Мягкая деградация.** При > 2x budget: inject только last 20% (top-N entries). При > 3x: inject только ralph-critical.md (T1). Graceful degradation вместо cliff.
- C) **Hard warning в stderr.** Не блокировать, но LOUDLY предупреждать при каждой сессии: "LEARNINGS.md exceeds budget by 3x — knowledge quality degraded".

---

## Серьёзные проблемы

### [P3-H1] "Last 20% of entries injected" (H5) + "reverse read" (L3) — логическое противоречие

**Почему плохо:**
Story 6.2 AC: "only last 20% of entries injected" (H5). Тот же Story 6.2: "reverse read: split by \n##, reverse section order, rejoin" (L3). Это взаимно исключающие операции:
- Если мы берём только last 20% → зачем reverse? Мы уже выбрали newest.
- Если мы reverse ALL → зачем last 20%? Newest уже наверху.
- Если мы берём last 20% и потом reverse — порядок снова от oldest к newest (внутри 20%).

**Неясно:** H5 применяется к LEARNINGS.md (raw) или к ВСЕМ знаниям? Distilled rules (ralph-*.md) не имеют append-only порядка.

**Опции исправления:**
- A) **Убрать reverse read.** Inject last 20% as-is (newest at bottom, naturally).
- B) **Убрать "last 20%".** Inject всё, reversed (newest first). Проще, надёжнее.
- C) **Уточнить семантику.** Last 20% = injection budget. Reverse = display order внутри бюджета. Документировать чётко.

---

### [P3-H2] Story 6.9 (A/B testing) — метрики недостаточны и ненадёжны

**Почему плохо:**

1. **Метрики не изолированы.** "repeat violations count, findings per task, first clean review rate" зависят от МНОГИХ факторов: сложность задач, качество кода, настроение LLM. Attributing change to injection mode = confounded.

2. **Нет baseline.** Нет "no injection" mode для сравнения. Scoped vs flat сравнивает два варианта, но оба могут быть хуже "без знаний вообще" (если знания низкого качества).

3. **Sample size.** Типичный bmad-ralph sprint = 5-15 задач. Статистически значимое сравнение требует минимум 30+ задач на каждый mode. Реалистично = месяцы A/B.

4. **Нет автоматического switching.** Пользователь вручную меняет config → вручную анализирует JSON → вручную решает. Это не A/B test, это manual experiment.

5. **Определение "победителя" не определено.** Какая метрика важнее? Какой threshold для statistical significance? Кто/что принимает решение о переключении?

**Опции исправления:**
- A) **Отложить 6.9 на Growth phase.** Недостаточно данных для A/B на текущем масштабе. Пока использовать scoped (подтверждён research: progressive disclosure > flat).
- B) **Упростить.** Вместо A/B — просто metric collection. Без mode switching. Trend: findings/task over time. Если падает — knowledge system работает.
- C) **Добавить "no injection" baseline** и automated reporting (не сравнение в JSON вручную).

---

### [P3-H3] freq:N вся цепочка ненадёжна — слишком много ответственности на Claude

**Почему плохо:**
M11 решил: "Claude assigns freq:N, Go validates monotonicity". Но цепочка:
1. Claude в review/execute пишет entry без freq (первое появление)
2. Claude в дистилляции видит 3 entries об одном — должен написать [freq:3]
3. Go проверяет что new freq >= old freq

Проблемы:
- Claude должен СЧИТАТЬ дубликаты при дистилляции. LLMs плохо считают (M11 сам это признаёт).
- Go проверяет монотонность, но не корректность. Claude написал [freq:2] для 5 entries — Go не заметит.
- "Go corrects arithmetic errors" — КАК? Go не знает правильное число. Go знает только previous freq.
- Вся violation tracking (T1 promotion при freq >= 10) зависит от НЕТОЧНЫХ чисел Claude.

**Конкурентный контекст:** Ни один конкурент (Copilot, Cursor, Aider) не пытается использовать LLM для counting/tracking. Они либо не делают tracking вообще, либо делают его programmatic (accepted/rejected suggestions у Copilot).

**Опции исправления:**
- A) **Go-only counting.** Go парсит entries при post-validation, хранит frequency map в DistillState. При дистилляции: Go вставляет [freq:N] в input. Claude ТОЛЬКО сжимает текст, не считает. Go проверяет output.
- B) **Убрать freq:N из v1.** T1 promotion = ручное решение пользователя. Freq tracking — Growth phase.
- C) **Hybrid.** Go считает occurrences в LEARNINGS.md (grep по topic), Claude получает подсказку "[this topic appeared N times]". Go потом проверяет output.

---

### [P3-H4] [needs-formatting] — архитектурный зомби

**Почему плохо:**
Консолидированный ревью C4 назвал [needs-formatting] "мёртвой веткой" и рекомендовал убрать. V4 СОХРАНИЛ его. Consequences:
1. ValidateDistillation criterion #7: "All [needs-formatting] entries either fixed or preserved". Дистилляция ОБЯЗАНА разобраться с тегами — дополнительная сложность промпта.
2. [needs-formatting] entries injected в каждый промпт до дистилляции. Бесполезный контекст.
3. Integration test 6.8 имеет отдельный scenario для [needs-formatting] fix cycle — тестовая сложность.
4. Если Claude пишет правильно (что ожидается, учитывая format specification в промпте) — tag никогда не используется. Dead code path.

**Опции исправления:**
- A) **Убрать [needs-formatting].** Невалидные entries удаляются Go (вариант B из P3-C1). Нет тегов, нет мёртвого кода.
- B) **Оставить, но НЕ инъектировать.** [needs-formatting] entries ИСКЛЮЧАЮТСЯ из `__LEARNINGS_CONTENT__` (как stale entries). Фиксятся только при дистилляции. Нет контекст-мусора.

---

### [P3-H5] Scope hints detection (M4) — недетерминистичен, зависит от LLM

**Почему плохо:**
M4 решение: "Go scans top 2 levels, collects extensions, maps to known globs. Claude uses scope info to create globs, Go validates."

Последний шаг: "Claude uses scope info to create globs" — Claude НЕ создаёт globs, Go создаёт? Или Claude? Если Claude — всё тот же недетерминизм (разные globs каждый раз). Если Go — зачем Claude в цепочке?

Конкретно: для Go-проекта Go детектирует `.go` files → передаёт Claude → Claude пишет `globs: ["*.go", "**/*.go"]`. Но Claude может написать `globs: ["**/*.go"]` или `globs: ["*.go"]`. Go валидирует через `filepath.Match` — оба валидны. Но поведение Claude Code при загрузке разное.

**Опции исправления:**
- A) **Go-only scope hints.** Go детектирует extensions → Go маппит на предопределённую таблицу → Go вставляет в frontmatter. Claude НЕ участвует в генерации globs. Детерминистично.
- B) **Оставить as-is**, но добавить таблицу canonical globs в Go и OVERRIDE Claude's output canonical'ами если extension matched.

---

## Замечания

### [P3-M1] 78 AC для 9 stories — чрезмерная сложность

Story 6.5 (Auto-Distillation) имеет 18 AC. Для сравнения: средний Story в Epics 1-5 имел 5-8 AC. Story 6.5 пытается быть одновременно: budget checker, distillation trigger, LLM session manager, output parser, multi-file writer, state machine (cooldown), crash recovery, metrics collector, scope hint generator, category manager, freq validator. Это 11 concerns в одной Story.

**Рекомендация:** Разбить 6.5 на 3 stories: (a) Budget + trigger, (b) Distillation session + parsing, (c) Multi-file output + metrics. Каждая testable independently.

---

### [P3-M2] "No FIFO, no archive" + "no injection CB" + "no auto CB" = нет safety net для overflow

V3 имел 3 уровня защиты от overflow: FIFO (убран), injection CB (убран в v4), auto CB (заменён на human gate). V4 имеет НОЛЬ автоматических safety nets. Единственная защита — пользователь нажимает "retry" на human gate.

---

### [P3-M3] ValidateDistillation criterion #3 "last 20% preserved" — неточен

Как Go проверяет "last 20% preserved"? Нужно: (a) знать какие entries были в хвосте old content, (b) найти их в new content. Но entries при дистилляции MERGE-ятся, ПЕРЕФОРМАТИРУЮТСЯ, категории меняются. Entry "## testing: assertion-quality [review, tests/foo.go:42]" может стать "## testing: assertion patterns [review]" после merge. Go не может reliable match.

**Рекомендация:** Заменить criterion на "total entry count >= 20% of original" (количественная проверка, не content matching).

---

### [P3-M4] Serena detection (C3/6.7) — minimal value, 6 AC для одного if-statement

Story 6.7 = 6 AC для: прочитать JSON, проверить наличие ключа, вернуть строку. Это ~20 строк Go-кода. Стоит ли целая Story? Стоило бы inline в Story 6.2 (prompt injection) как один AC.

---

### [P3-M5] Config complexity creep

V4 добавляет: `distill_cooldown`, `distill_target_pct`, `distill_timeout`, `knowledge_injection` (scoped/flat), плюс существующие `learnings_budget`, `serena_enabled`, `serena_timeout`, `always_extract`. Это 8 новых/modified config fields для feature которая ещё не доказала ценность. Конкуренты (Cursor, Aider) имеют 0-2 config fields для rules.

---

### [P3-M6] ANCHOR marker + [freq:N] + [needs-formatting] + VIOLATION: — 4 inline marker системы

LEARNINGS.md entry может выглядеть так:
```
## testing: assertion-quality [review, tests/foo.go:42] [freq:7] [needs-formatting] [ANCHOR]
VIOLATION: strings.Contains used instead of errors.As
```
Это 4 разных inline marker системы, каждая с собственным parsing. Сложность парсинга растёт комбинаторно. Каждый marker — точка failure при LLM generation и Go parsing.

---

## Неучтённые идеи из ресерчей

### 1. MCP Memory Server (R3, S4) — полностью проигнорирован

R3 оценивает MCP Memory Server как "наиболее перспективную альтернативу для Growth phase". Weighted score 7.3/10 (второе место после file injection 8.7). Epic 6 не планирует НИКАКОЙ подготовки к MCP migration. Даже абстракция knowledge storage в интерфейс не предусмотрена.

**Рекомендация:** Добавить `KnowledgeStore` interface в 6.1 для будущей замены file-based на MCP-based. Не реализовывать MCP, но подготовить seam.

### 2. chromem-go как fallback RAG (R3, S3)

R3 находит единственную pure-Go vector store compatible с CGO_ENABLED=0. При >500 entries file-based деградирует. Epic 6 не готовит path для перехода.

### 3. "No new citations may appear" validation criterion (J8 из review-6.5-6.6)

Консолидированный ревью рекомендовал: distillation output НЕ ДОЛЖЕН содержать citations, которых не было в input (prevents hallucinated rules). V4 не включил этот criterion. LLM Scaling Paradox (Feb 2026): compressor LLMs overwrite source facts.

**Рекомендация:** Добавить criterion #9 в ValidateDistillation.

### 4. Model Collapse anchor set (J14)

Research (Nature 2024): self-referencing distillation = mathematical variance growth. Ранний collapse = потеря edge-case patterns (ВЫГЛЯДИТ как улучшение). V4 имеет ANCHOR marker, но только для freq >= 10. Research рекомендует anchor LAST 20% entries unconditionally (не только high-frequency).

### 5. Cross-project knowledge sharing

Competitive analysis отмечает: "ни один конкурент не делает cross-project knowledge sharing — potential first-mover advantage". Epic 6 не планирует даже export/import learnings между проектами.

### 6. Letta Context Repositories (R3, S6)

Git-based versioning of memory. LEARNINGS.md уже в git, но нет mechanism для "rollback to knowledge state at commit X". Могло бы помочь при model collapse detection.

---

## Противоречия между решениями

### 1. C1 (Claude пишет) vs M11 (Claude считает freq) vs H2 (Claude выбирает categories) — слишком много доверия Claude

Три решения делегируют Claude: (a) запись знаний, (b) подсчёт frequency, (c) выбор категорий. При этом quality gates существуют ИМЕННО потому что Claude ненадёжен. Если Claude достаточно надёжен чтобы правильно писать, считать и категоризировать — зачем 6 quality gates и 8 validation criteria?

Архитектурная позиция должна быть consistent: либо "Claude надёжен, минимум gates" (простота), либо "Claude ненадёжен, maximum programmatic control" (надёжность). V4 пытается сидеть на двух стульях.

### 2. "No forced truncation" (design principle) vs "inject only last 20%" (H5)

V4 гордится: "no FIFO, no archive, no forced truncation — file stays as-is". Но H5 inject-ит ТОЛЬКО last 20% entries. Это значит 80% entries ЗАПИСЫВАЮТСЯ но НИКОГДА не используются (до дистилляции). Это фактически archive — entries exist but aren't read. Нечестное именование.

### 3. Human gate (C2) vs автономность проекта

bmad-ralph mission statement: "Go CLI tool orchestrating Claude Code sessions for autonomous development." Human gate на distillation failure = НЕ autonomous. Contradiction с миссией проекта.

### 4. L2 (no mutex, YAGNI) vs L6 (advisory about concurrent runs)

Если concurrent runs — реальный риск (L6 предупреждает), то "no mutex, YAGNI" (L2) — неверная оценка. Или concurrent runs нереальны (убрать L6 advisory), или они реальны (добавить file lock, не advisory).

### 5. "Last 20% injected" (H5) vs "reverse read newest-first" (L3) vs "self-review top 5 most recent"

Три разных определения "что значит recent" в трёх разных AC. Last 20% = tail. Reverse = reorder. Top 5 most recent = yet another subset. Несогласованные алгоритмы.

---

## Что хорошо

1. **3-layer distillation architecture** (Go dedup -> LLM compression -> validation) — архитектурно правильная. Separation of concerns чёткое. Go handles deterministic ops, LLM handles semantic compression.

2. **Multi-file ralph-{category}.md with globs** — использует native Claude Code infrastructure (.claude/rules/ с YAML frontmatter). Не изобретает собственный injection mechanism. Progressive disclosure через существующий tooling.

3. **Snapshot-diff model (идея)** — подход "let Claude work, validate after" правильный для LLM orchestration. Проблема в реализации (tag vs reject), не в идее.

4. **MonotonicTaskCounter (H1)** — правильное решение для persisted cooldown. Cross-session counting через persisted state. Простое и надёжное.

5. **BEGIN/END markers protocol (H6)** — standard LLM structured output pattern. Правильный подход к parsing LLM output. Fallback на misc.md при parse failure — graceful degradation.

6. **JIT citation validation через os.Stat (M9)** — минимально viable, быстрый, не over-engineered. Line range validation правильно отложена на Growth.

7. **Canonical categories (H2) + NEW_CATEGORY mechanism** — правильный баланс между структурой и гибкостью. Grow-only list предотвращает потерю категорий.

8. **Решение не трогать CLAUDE.md** — FR26 через невмешательство. Zero risk. CVE-2025-59536, CVE-2026-21852 подтверждают: programmatic config editing = risk class. Правильное решение.

9. **Serena как MCP, не CLI (C3)** — правильная коррекция фундаментальной ошибки в v3. Minimal interface (2 methods) — YAGNI.

10. **2-generation backups (L4)** — простое, надёжное, проверенное. Crash recovery at startup (M7) — правильный safety net.

---

## Сравнение с конкурентами: что bmad-ralph упускает

### 1. OpenAI Codex (harness-engineering pattern)

OpenAI Codex использует sandbox containers с internet-isolated execution. Knowledge = deterministic (containerized tools), не LLM-generated rules. bmad-ralph делает ставку на LLM-generated knowledge — это ВЫСОКИЙ risk / HIGH reward подход. Codex approach: trust tooling, not memory.

### 2. Cursor .cursor/rules — simplicity wins

Cursor: один directory, plain text files, загружаются all-at-once. Нет distillation, нет freq tracking, нет quality gates. И работает. Потому что правила пишет ЧЕЛОВЕК. bmad-ralph's complexity = cost of auto-generation. Вопрос: оправдана ли эта cost?

### 3. Aider .aider.conf.yml — git-aware

Aider использует git commit history для context (repo map). Implicit knowledge = structured (commit messages, diffs). bmad-ralph's explicit knowledge = unstructured text, validated post-hoc. Aider approach: knowledge embedded in artifacts, not separate files.

### 4. Continue — RAG-based selection

Continue использует embeddings для selection релевантных правил. bmad-ralph использует glob patterns. Embeddings = semantic relevance. Globs = file-type relevance. Для code patterns semantic > file-type (ошибка error wrapping релевантна для всех файлов, не только *.go).

### 5. Windsurf (Codeium) — Cascade memory

Windsurf Cascade поддерживает persistent memory между sessions, automatic context awareness. Ключевое отличие: memory management ВНУТРИ модели (не external pipeline). Simpler architecture, less control, but less overhead.

### Главный insight

Конкуренты выбирают ОДНУ из двух стратегий:
- **Human-curated rules** (Cursor, Aider, Cline) — простота, надёжность, manual effort
- **Embedded memory** (Windsurf, GitHub Copilot) — integrated в модель, automatic, black-box

bmad-ralph выбирает ТРЕТЬЮ: **LLM-generated + programmatically-validated rules**. Это уникально, но и самое рискованное. Если quality gates и validation недостаточны (а P3-C1 показывает, что в v4 gates ослаблены) — knowledge system может вредить больше чем помогать.

---

## Итоговая оценка

**Блокеры:** 3 (P3-C1, P3-C2, P3-C3) — все касаются ослабления safety nets в v4 относительно v3/review recommendations.

**Серьёзные:** 5 (P3-H1 через P3-H5) — логические противоречия, ненадёжные метрики, architectural zombies.

**Общий вектор проблем:** V4 сделал систему МЕНЕЕ автономной (human gates) и МЕНЕЕ защищённой (убраны CB, injection CB, pending-file) по сравнению с рекомендациями 10-agent review. Paradox: 10 агентов ревьюили, нашли 28 проблем, но решения пользователя по некоторым из них УХУДШИЛИ архитектуру по сравнению с рекомендациями тех же агентов.

**Рекомендация:** Перед реализацией — вернуть 3 safety net механизма: (1) pending-file ИЛИ post-validation с delete (а не tag), (2) auto CB с human gate как escalation (а не замена), (3) injection budget protection.
