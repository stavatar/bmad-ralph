# Ralph Loop v5 Final: "Everything is a Ralph Loop"
# Миграция sprint-orchestrator для brownfield NestJS+React

> **Дата:** 2026-02-22
> **Версия:** 5.0 final (UNION merge of v1+v2, resolves v3/v4 contradictions)
> **Аудитория:** Разработчик, мигрирующий с skill-orchestrator на Ralph Loop
> **Контекст:** MentorLearnPlatform (NestJS 10 + React 18, 1451 тест, 12 спринтов)
> **Источники:** 40 sources, evidence table in intermediate/ralph-v5/
> **Цитирование:** inline [Автор/Источник]

---

## 1. Executive Summary

### 1.1. Ключевое открытие: "Everything is a Ralph Loop"

Два предыдущих отчёта (v3 -- экосистема, 1215 строк; v4 -- gap-анализ, 812 строк) пришли к частично противоречивым выводам. v3 утверждал, что Ralph Loop **универсально достаточен** и skill-orchestrator подлежит полной замене. v4 показал, что 75% оркестрационной логики (~610 из 641 строки) **не покрывается** чистым Ralph Loop, и brownfield-проект требует сохранения review, knowledge extraction и compaction recovery.

**Оба отчёта правы на разных уровнях абстракции.** Ключевой инсайт, подтверждённый production-evidence от Carlini (Anthropic), ralphex (umputun), continuous-claude (Chowdhary) и hamelsmu: **не нужно оборачивать в Ralph Loop только фазу кодирования**. Нужно декомпозировать ВСЕ фазы (init, execute, review, finish) на атомарные задачи, и каждую атомарную задачу выполнять как отдельную итерацию с чистым контекстом.

Review -- это не "отдельный concern вне Ralph Loop" (v4), а **набор атомарных задач, каждая из которых IS a Ralph iteration**: запустить тесты, проверить AC, исправить findings, запустить E2E. ralphex доказывает это напрямую: 5 review-агентов работают как Task tool calls в свежих сессиях [S04, ralphex]. Carlini подтверждает: ВСЕ фазы (code, test, review, merge) для 100K-строчного компилятора были просто задачами в flat list [S07, Carlini/Anthropic].

Текущий sprint-orchestrator (641 строка SKILL.md через 4 файла) заменяется на:
- **sprint-loop.sh** (~30 строк bash) -- цикл
- **sprint-state.json** (~40 строк JSON) -- плоский список задач
- **PROMPT_sprint.md** (~40 строк) -- промпт для каждой итерации
- **CLAUDE.md** -- domain knowledge (уже есть, расширить)

### 1.2. Шесть противоречий -- одна таблица

| # | v3 утверждал | v4 возражал | Вердикт v5 |
|---|-------------|-------------|------------|
| 1 | Review = test+lint в каждой итерации | Review -- separate concern, нужен Judge Agent | **v4 прав, НО** review декомпозируется в атомарные задачи, каждая -- Ralph iteration [ralphex, hamelsmu] |
| 2 | Sprint-finish = exit+auto-archive, тривиально | Finish -- 3-level knowledge extraction | **v4 прав, НО** finish тоже декомпозируется: extract -> update MEMORY -> generate summary [Osmani] |
| 3 | Знания из CLAUDE.md -> AGENTS.md | Оставить в CLAUDE.md | **v4 прав.** CLAUDE.md нативно поддерживается, AGENTS.md -- нет (issue #6235) [S22] |
| 4 | Compaction recovery не нужна с Ralph | Unsolved problem #1 | **Оба частично правы.** Intra-session: не нужна. Inter-session: нужен sprint-state.json [Huntley, Carlini] |
| 5 | for-loop универсально достаточен | Ломается для brownfield | **v4 прав для наивного for-loop**, но "everything is a Ralph Loop" делает Parsons правым на более высоком уровне абстракции [Stringer, Carlini] |
| 6 | Hybrid по типу задач | Hybrid по фазам | **Ни один.** Hybrid по АТОМАРНОСТИ задач -- каждая атомарная задача = свежая сессия [Stringer] |

### 1.3. Архитектурная рекомендация (3 предложения)

Вся оркестрация переносится в bash-цикл, который последовательно запускает Claude Code с чистым контекстом для каждой задачи из sprint-state.json. Domain knowledge (NestJS pitfalls, E2E rules, design system) остаётся в CLAUDE.md -- единственном файле, который Claude Code читает автоматически. Stateless atomic skills (nestjs-validation-pipe-whitelist, vitest-fake-timers-async, figma-screen-builder) сохраняются без изменений -- они уже соответствуют Ralph-паттерну.

**Что НЕ меняется:** CLAUDE.md как домен знаний, MEMORY.md как прогресс, Figma skills (stateless, one-shot), атомарные skills-хелперы (nestjs-validation-pipe-whitelist и подобные).

**Что удаляется:** sprint-run (~203 строки), sprint-preflight (~148), sprint-review (~170), sprint-finish (~120) -- итого ~641 строка orchestration logic. Заменяется на ~30 строк bash + ~40 строк prompt template + JSON state file.

### 1.4. Стоимость миграции

| Метрика | До (skill-orchestrator) | После (sprint-loop) | Delta |
|---------|------------------------|---------------------|-------|
| Строк оркестрации | ~641 | ~110 (bash + prompt + state) | -83% |
| Файлов SKILL.md (orchestrator) | 4 | 0 | -100% |
| Compaction incidents/sprint | 2-4 | 0 (свежий контекст) | -100% |
| Time to first commit | ~45 мин (4 gates) | ~15 мин (1 init task) | -67% |
| Отладка при сбое | Перечитать 4 SKILL.md | Посмотреть sprint-state.json | Simpler |
| Knowledge capture | По итогу спринта | Per-task discovery logging | Continuous |
| Vendor lock-in | Claude Code skills only | Bash (любой AI tool) | Portable |
| Domain knowledge в CLAUDE.md | ~200 строк | ~350 строк | +75% |
| Стоимость итерации | ~$1.50-2.50 (с compaction) | ~$0.80-1.50 (чистый контекст) | -40% [Stringer] |

---

## 2. Разрешение противоречий v3 vs v4

### 2.1. Противоречие 1: Замена Review

**v3 (S35):** "test+lint в каждой итерации = достаточно для review. Отдельная review-фаза -- over-engineering."

**v4 (S36):** "Review -- отдельный concern с другой ролью. Coding agent и reviewing agent имеют разные objectives. Нужен Judge Agent (Vercel pattern), не встроенный test+lint."

**Evidence:**

- **ralphex [S04]:** 4-phase pipeline: execute -> review1 (5 агентов в свежих сессиях) -> external review -> review2 (2 агента). Review IS Ralph iterations -- каждый reviewer = отдельный Task tool call с чистым контекстом. ralphex не противопоставляет review и Ralph Loop, а ВКЛЮЧАЕТ review в тот же loop [ralphex.umputun.dev].

- **hamelsmu [S08]:** Stop hook -> Codex review -> Claude fixes = замкнутый feedback loop. Review -- часть итерационного цикла, а не отдельная фаза. State хранится в `.claude/review-loop.local.md` [github.com/hamelsmu/claude-review-loop].

- **Vercel Labs Judge Agent [S40, DeepWiki]:** `markComplete(summary, filesModified)` -> `verifyCompletion` callback -> `runJudge()`. Judge имеет read-only доступ, возвращает APPROVE или requestChanges. Feedback инжектируется как user message в **следующую итерацию**. Это именно Ralph Loop -- review задача проходит через тот же iterative cycle [vercel-labs/ralph-loop-agent].

- **Carlini [S07, S25]:** 16 агентов, ~2000 сессий, ВСЕ фазы (code, test, review, merge) как задачи в flat list. Нет "отдельной review-фазы" -- есть review-задачи наравне с coding-задачами [anthropic.com/engineering/building-c-compiler, InfoQ].

- **Tessmann [S11]:** Agent Teams для creative work (architecture, UX decisions), Ralph Loop для mechanical (coding, testing, **reviewing**). Review -- mechanical task, значит Ralph [medium.com/@himeag].

**ВЕРДИКТ:** v4 прав, что review -- отдельный concern. Но это не значит, что review вне Ralph Loop. Review **декомпозируется** в атомарные задачи:

```
review-1: Run all tests + typecheck + lint
review-2: AC review -- check acceptance criteria for each story (Judge Agent с AC-checklist)
review-3: Quality review -- code quality check (Judge Agent с quality-checklist)
review-4: Fix review findings
review-5: Run E2E on localhost
```

Каждая из этих задач -- отдельная итерация Ralph Loop с чистым контекстом. Review agent получает свой prompt ("ты -- ревьюер, проверь AC"), свою свежую сессию. Это не "test+lint в каждой итерации" (v3) и не "Judge Agent как отдельная система" (v4). Это **review tasks в том же flat list**, что и coding tasks.

### 2.2. Противоречие 2: Замена Sprint-finish

**v3 (S35):** "Exit condition + auto-archive (snarktank pattern) = тривиально."

**v4 (S36):** "Finish -- 3-level knowledge extraction (episodic -> semantic -> procedural). Наш claudeception трансформирует опыт в reusable skills. Community альтернатива слабее."

**Evidence:**

- **Osmani's four memory channels [S10, addyosmani.com]:** Git history (episodic), progress.txt (append-only journal), AGENTS.md (semantic -- conventions, gotchas), PRD/Spec (procedural). Osmani НЕ имеет "finish phase" -- knowledge accumulates per-iteration. *"After each task, the loop can append key learnings"* [addyosmani.com/blog/self-improving-agents].

- **alfredolopez80 [S29]:** Три слоя memory: claude-mem MCP (persistent), local JSON (state), ledgers (transaction logs). Learning: repo curation -> pattern extraction -> rule validation. Автоматически в конце каждой итерации [github.com/alfredolopez80/multi-agent-ralph-loop].

- **alexlavaee [S34]:** *"AGENTS.md serves as Project Memory -- transforming agents from 'stateless tools into stateful team members.'"* 114 агентов populate AGENTS.md. Агенты улучшают свои собственные промпты на основе контекста проекта [alexlavaee.me/blog/ai-coding-infrastructure].

- **snarktank [S05]:** Auto-archiving + progress.txt append-only. Простая, но работающая модель для greenfield.

- **Carlini [S07]:** 16 агентов, каждый обновляет progress файлы как отдельную атомарную операцию. Нет монолитного "finish" -- есть набор post-iteration записей. Три типа agent memory: episodic (progress.txt), semantic (AGENTS.md), procedural (skills/patterns). Наш claudeception трансформирует episodic -> semantic -> procedural [anthropic.com/engineering].

**ВЕРДИКТ:** v4 прав, что finish -- не тривиально. Но finish ТОЖЕ декомпозируется:

```
finish-1: Extract discoveries from sprint -> append to MEMORY.md
finish-2: Update CLAUDE.md with new domain traps found during sprint
finish-3: Generate sprint summary report
```

Claudeception/skill-propagator остаётся -- но как **одна из атомарных задач** в state file, а не как отдельный многошаговый skill. Каждая задача получает свежий контекст и чистое description: "Прочитай discoveries.log, извлеки lessons learned, обнови MEMORY.md."

Ключевой insight от v4 сохраняется: трёхуровневая extraction (episodic -> semantic -> procedural) богаче community-подхода (append-only progress.txt). Но механизм execution меняется: не монолитный skill, а последовательность атомарных задач.

### 2.3. Противоречие 3: Расположение знаний

**v3 (S35):** "Перенести institutional knowledge из CLAUDE.md/MEMORY.md в AGENTS.md. AGENTS.md -- стандарт community."

**v4 (S36):** "CLAUDE.md нативно поддерживается Claude Code. AGENTS.md -- нет (GitHub issue #6235). Domain knowledge остаётся в CLAUDE.md."

**Evidence:**

- **GitHub issue #6235 [S22]:** AGENTS.md feature request для Claude Code. НЕ нативно поддерживается -- агент НЕ автоматически читает AGENTS.md при старте. CLAUDE.md -- единственный файл с гарантированным autoload [github.com/anthropics/claude-code/issues/6235].

- **HumanLayer CLAUDE.md guide [S23]:** *"CLAUDE.md best practices: minimal, universally applicable instructions."* CLAUDE.md должен содержать правила, а не весь state [humanlayer.dev/blog/writing-a-good-claude-md].

- **AiEngineerGuide AGENTS.md [S24]:** AGENTS.md -- cross-platform стандарт (Cursor, Codex, Amp). CLAUDE.md -- Claude-specific. Для multi-tool workflow AGENTS.md предпочтительнее [aiengineerguide.com].

- **Gend.co Skills guide [S37]:** Skills vs CLAUDE.md -- practical team guide. Skills для reusable patterns, CLAUDE.md для project-wide rules [gend.co].

- **snarktank, Huntley [S05, S01]:** AGENTS.md как институциональная память. *"AI coding tools automatically read AGENTS.md"* -- но это справедливо для Amp и Cursor, НЕ для Claude Code [github.com/snarktank/ralph, ghuntley.com/ralph].

**ВЕРДИКТ:** v4 прав. CLAUDE.md -- единственный файл с гарантированным autoload в Claude Code. Domain knowledge (NestJS pitfalls, E2E rules, Prisma gotchas) остаётся в CLAUDE.md. Sprint prompt (PROMPT_sprint.md) **явно инструктирует** Claude читать CLAUDE.md и MEMORY.md при старте каждой итерации -- это эквивалент того, что AGENTS.md делает автоматически в Amp/Cursor.

**Когда AGENTS.md имеет смысл:** если проект использует несколько AI tools (Claude Code + Cursor + Codex). В MentorLearnPlatform используется только Claude Code, поэтому AGENTS.md -- unnecessary indirection. AGENTS.md можно создать как АЛИАС или symlink для cross-tool совместимости, но source of truth -- CLAUDE.md.

Конкретно для MentorLearnPlatform: domain knowledge из четырёх SKILL.md файлов (NestJS pitfalls, E2E rules, compaction recovery, review checklists) переезжает в секции CLAUDE.md.

### 2.4. Противоречие 4: Compaction Recovery

**v3 (S35):** "Ralph Loop не нуждается в compaction recovery. Чистый контекст каждую итерацию."

**v4 (S36):** "Compaction recovery -- unsolved problem #1. Claude Code теряет контекст при compaction. review-state.json + structured recovery -- уникальное решение."

**Evidence:**

- **Huntley [S01]:** *"Compaction is the devil."* Именно поэтому Ralph Loop использует fresh context per iteration -- compaction просто не происходит внутри одной итерации [ghuntley.com/ralph].

- **AI Hero [S09]:** *"Plugin fills context window, bash loop preserves quality. Performance zones: first 40% = optimal."* Bash loop = no compaction by design [aihero.dev].

- **Mickel [S10]:** *"State in files not conversation. Forgetting > compaction."* Ralph как "malloc orchestrator" -- состояние в файлах, не в conversation memory [bytesizedbrainwaves.substack.com].

- **TorqSoftware [S26]:** Plan Mode + structured artifacts имеют более высокий приоритет сохранения при compaction. Но это mitigation, не solution [reading.torqsoftware.com].

- **claudefa.st [S27]:** Buffer ~33K tokens (16.5%). `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` env var. Compaction management -- активная область разработки [claudefa.st].

- **continuous-claude [S06]:** `SHARED_TASK_NOTES.md` как inter-session state. Parallel worktrees + shared state файл = persistence между сессиями [github.com/AnandChowdhary/continuous-claude].

**ВЕРДИКТ:** Оба частично правы.

- **Intra-session compaction:** Ralph Loop ЭЛИМИНИРУЕТ эту проблему. Каждая итерация = свежий контекст. Compaction handler в sprint-run, перечитывание CLAUDE.md/MEMORY.md/skill state -- всё это становится ненужным. v3 прав.

- **Inter-session state persistence:** Между итерациями нужно помнить, какие задачи выполнены, какие нет. Это решается тривиально -- sprint-state.json на диске. Claude читает его в начале каждой итерации. v4 прав, что state нужен, но формат другой: не review-state.json с workflow recovery, а sprint-state.json с flat task list.

Итого: intra-session recovery **удаляется** (600 строк domain knowledge + compaction handlers), inter-session state **упрощается** до JSON файла. Ralph Loop решает compaction не "обходом", а "устранением корневой причины". Вместо борьбы с потерей контекста в длинной сессии -- короткие сессии без накопления контекста.

### 2.5. Противоречие 5: Универсальность Parsons

**v3 (S35):** *"A for loop is a legitimate orchestration strategy"* [Parsons, S03] -- for-loop универсально достаточен для любого проекта.

**v4 (S36):** "Для brownfield (800+ тестов, Prisma schema, NestJS modules) plain for-loop ломается: breaking changes аффектят другие модули, acceptance criteria сложнее чем 'тесты проходят', architectural consistency требует review."

**Evidence:**

- **Parsons [S03]:** Inner loop (self-correcting) + outer loop (task selection) = enough. Bitter Lesson: общие методы, масштабируемые вычислениями, побеждают специализированные подходы [chrismdp.com/your-agent-orchestrator-is-too-clever].

- **Parsons production lessons [S03]:** Beads + git worktrees + decision table (working tree x tests). Two-layer architecture (PM + Builder). Это **больше**, чем простой for-loop -- это structured decomposition [chrismdp.com].

- **Carlini [S07]:** 100K LOC C-компилятор -- flat list задач + 16 параллельных агентов. НО: *"Most of his effort went into designing the environment around Claude -- the tests, the environment, the feedback."* Environment design = тот самый "orchestration", который Parsons упрощает [anthropic.com/engineering/building-c-compiler].

- **Stringer [S14]:** Atomic task decomposition: $0.77 decomposed vs $1.82 one-shot. Декомпозиция на атомарные задачи = ключ к efficiency [medium.com/@levi_stringer].

- **Paddo [S12]:** *"SDLC collapse"* -- phase boundaries dissolve. Harness automates execution, not decisions. Фазы (init, execute, review, finish) -- искусственное разделение. В Ralph Loop все фазы -- просто задачи в одном списке [paddo.dev/blog/ralph-wiggum-autonomous-loops].

- **Gordon Mickel [S10]:** "Ralph как malloc orchestrator" -- контекст = managed memory, state в файлах [bytesizedbrainwaves.substack.com].

**ВЕРДИКТ:** v4 прав для **наивного** for-loop (`for story in stories: ralph_loop(story)`). Brownfield breaking changes, inter-module dependencies, architectural review -- всё это реально.

НО "everything is a Ralph Loop" делает Parsons правым на **более высоком уровне абстракции**. Наивный for-loop ломается не потому, что for-loop плох, а потому, что задачи не атомарные. Если **ВСЕ фазы** (init, execute, review, finish) декомпозированы в атомарные задачи и выстроены в правильном порядке в flat list -- это тот же for-loop, но с правильной granularity.

```
# Наивный for-loop (v3) -- ломается для brownfield:
for story in stories:
    ralph_loop(story)

# "Everything is a Ralph Loop" (v5) -- работает:
while next_task := get_next_incomplete(state_file):
    claude_fresh_session(next_task)
    # task marks itself passes:true if successful
```

Разница: второй вариант включает review tasks, init tasks, finish tasks -- не только coding tasks. Brownfield не ломает Ralph -- brownfield требует **грамотной декомпозиции**. Зависимости между stories решаются ПОРЯДКОМ задач в sprint-state.json. Phase grouping -- это просто секции в JSON. Regression suite -- это задача "run-all-tests" между фазами.

### 2.6. Противоречие 6: Гранулярность Hybrid

**v3 (S35):** "Hybrid по типу задач: coding = Ralph Loop, bugfix = skill-orchestrator, Figma = skills."

**v4 (S36):** "Hybrid по фазам: init = skill, execute = Ralph, review = separate system, finish = skill."

**Evidence:**

- **Tessmann [S11]:** Hybrid: Agent Teams для creative work (architecture, design), Ralph Loop для mechanical (coding, testing, reviewing). Критерий -- не тип задачи и не фаза, а **степень неопределённости** [medium.com/@himeag].

- **claudefa.st [S13]:** Thread hierarchy: Base -> P -> C -> F -> B -> L-threads. Init -> Build -> Review -> Ship phases. Каждый thread = свежая сессия [claudefa.st].

- **Goose [S19]:** Worker/reviewer = два агента, но оба работают в loop. State files: task.md, work-summary.txt, review-result.txt (SHIP/REVISE). Не "фаза", а "задача" [block.github.io/goose].

- **O'Brien [S15]:** Hat System для specialized personas + backpressure gates. Персона (reviewer, coder, architect) -- не "фаза", а "шляпа" для одной итерации [mikeyobrien/ralph-orchestrator].

- **AI Hero [S09]:** Plugin fills context window, bash loop preserves quality. Performance zones: первые 40% контекста = оптимальное reasoning. Fresh context для каждой задачи гарантирует работу в оптимальной зоне [aihero.dev].

**ВЕРДИКТ:** Ни v3 (по типу задач), ни v4 (по фазам). Правильная гранулярность -- **по АТОМАРНОСТИ задачи**. Каждая атомарная задача получает свою свежую сессию, независимо от того, что это: coding, review, finish, или init.

Атомарная задача = задача, которую один агент может выполнить за одну сессию (1-2 часа "живого" разработчика, по chrismdp [S03]), с чётким критерием завершения.

| Задача | Фаза | Тип | Ralph-итерация? |
|--------|------|-----|-----------------|
| Прочитать MEMORY.md и создать ветку | init | mechanical | ДА (одна итерация) |
| Написать TDD-тесты для story | execute | creative | ДА (одна итерация) |
| Реализовать backend endpoint | execute | mechanical | ДА (одна итерация) |
| Проверить acceptance criteria | review | judgment | ДА (одна итерация) |
| Пофиксить review findings | review | mechanical | ДА (одна итерация) |
| Извлечь discoveries в MEMORY.md | finish | reflection | ДА (одна итерация) |

**Что остаётся вне Ralph Loop:**
- **Figma skills** (figma-screen-builder, figma-to-code) -- stateless, one-shot, interactive
- **Атомарные skills-хелперы** (nestjs-validation-pipe-whitelist, vitest-fake-timers-async) -- domain knowledge patterns, не orchestration
- **Human approval gates** -- `PAUSE` маркер в state file, loop ждёт человека

---

## 3. "Everything is a Ralph Loop" -- Evidence from Production

### 3.1. Nicholas Carlini: C Compiler (16 агентов, 2000 сессий)

**Масштаб:** 100K строк Rust, C-компилятор, собирающий Linux 6.9 на x86, ARM, RISC-V. $20K API costs [S07, S25].

**Архитектура:** 16 параллельных агентов, каждый в своём git worktree. ~2000 сессий Claude Code. Lock-based task claiming через текстовые файлы (аналог database locks, но в .txt) [anthropic.com/engineering/building-c-compiler, InfoQ].

**Ключевой инсайт для нас:** Carlini НЕ разделял работу на фазы (init -> code -> review -> finish). Все задачи -- code, test, review, documentation, merge -- лежали в **одном flat list**. Feature list как JSON с `passes: boolean` -- агенты модифицировали ТОЛЬКО поле `passes` [S07].

*"Most of his effort went into designing the environment around Claude -- the tests, the environment, the feedback -- so that it could orient itself without him"* [anthropic.com/engineering/building-c-compiler].

ВСЕ аспекты работы (не только кодирование) были организованы как атомарные задачи:

```
feature_list.json:
  - "Implement x86 register allocator"     <- coding
  - "Add test suite for ARM codegen"       <- testing
  - "Fix regression in RISC-V backend"     <- bugfix
  - "Verify Linux 6.9 boot on x86"        <- verification
```

Init, review, verification -- всё шло через тот же механизм: атомарная задача -> fresh session -> update state.

**Применение к нашему проекту:** Если 16 агентов могут построить компилятор за 2000 сессий из flat list -- наш sprint из 4-6 stories точно можно описать как flat list из ~15 атомарных задач (init + stories + review + finish).

### 3.2. ralphex: Review IS Ralph Loop (5+2 агентов)

**Архитектура [S04]:** 4-phase pipeline:
1. **Execute** -- coding agent реализует задачу
2. **Review 1** -- 5 review-агентов (security, performance, test coverage, architecture, code style) запускаются как **Task tool calls в свежих сессиях**
3. **External review** -- PR создаётся, ждёт human review
4. **Review 2** -- 2 дополнительных агента (integration, documentation)

[ralphex.umputun.dev]

**Критически важно:** Review-агенты в ralphex -- это НЕ отдельная система. Это **те же самые Task tool calls**, что и coding agent. Каждый reviewer получает свежую сессию, читает diff, пишет findings. Findings записываются в файл. Если findings серьёзные -- coding agent получает новую задачу "fix review findings" и цикл продолжается.

**Маппинг на наш проект:** Наши два параллельных reviewer-а (AC + Quality) в sprint-review -- это ровно паттерн ralphex. Разница: у нас они живут в skill-orchestrator, у ralphex -- в state file как задачи.

### 3.3. continuous-claude: PR Lifecycle as Loop

**Архитектура [S06]:** Полная автоматизация PR lifecycle:
```
create branch -> implement -> commit -> create PR -> wait CI -> fix CI failures -> merge
```

Каждый шаг -- одна итерация loop. `SHARED_TASK_NOTES.md` как межсессионная память. Parallel worktrees для одновременной работы над несколькими PR [github.com/AnandChowdhary/continuous-claude].

**Ключевой инсайт:** PR creation, CI monitoring, merge -- это всё "not coding", но всё это работает как Ralph iterations. Наш sprint-finish (create PR, run pr-review-toolkit, update MEMORY.md) -- тот же паттерн.

### 3.4. hamelsmu: Review = Stop Hook + Codex + Fix Loop

**Архитектура [S08]:** Claude coding -> Stop hook triggers -> Codex review (external model) -> findings -> Claude fixes -> repeat.

State хранится в `.claude/review-loop.local.md`. Двух-модельный review: Codex как "fresh eyes" reviewer, Claude как fixer. Loop продолжается пока reviewer не вернёт APPROVE [github.com/hamelsmu/claude-review-loop].

**Применение:** hamelsmu делает review ЧАСТЬЮ итеративного цикла, а не отдельной фазой. Это подтверждает "everything is a Ralph Loop" -- review task в том же списке, что и coding tasks.

### 3.5. Goose: Worker -> Reviewer -> SHIP/REVISE

**Архитектура [S19]:** Two-model (worker + reviewer) iterative loop. State files: `task.md`, `work-summary.txt`, `review-result.txt` (содержит `SHIP` или `REVISE`).

```
task.md -> worker -> work-summary.txt -> reviewer -> review-result.txt (SHIP / REVISE)
         |___________________ loop if REVISE ___________________________________|
```

Worker выполняет задачу -> пишет work-summary -> reviewer читает summary + diff -> возвращает SHIP или REVISE с feedback. REVISE -> worker получает feedback как новую задачу -> цикл продолжается [block.github.io/goose].

**Маппинг:** SHIP/REVISE -- binary exit condition, проще нашего multi-level review. Но паттерн тот же: review -- итерация в цикле, а не внешняя система. SHIP/REVISE -- эквивалент `passes: boolean`.

### 3.6. Tessmann: Agent Teams + Ralph = Всё через Loop

**Ключевое разделение [S11]:** Opus для judgment (архитектура, design decisions), Sonnet для iteration (coding, testing, reviewing). Agent Teams для creative/uncertain work, Ralph Loop для mechanical/deterministic.

> "Opus for judgment, Sonnet for iteration"

**Но:** даже Agent Teams -- это "loop", потому что Teams re-run до convergence. Tessmann не противопоставляет Teams и Ralph -- она показывает, что Teams = Ralph Loop с multiple agents per iteration. Creative работа декомпозируется: Opus принимает решение (1 итерация), Sonnet исполняет (N итераций). Judgment -- тоже атомарная задача.

### 3.7. Stringer: Atomic Decomposition = Каждый шаг -- итерация

**Evidence [S14]:** Atomic task decomposition: $0.77 (decomposed into 5-7 atomic tasks) vs $1.82 (one-shot implementation). Декомпозиция не только дешевле, но и качественнее -- каждый атомарный шаг получает full context window [medium.com/@levi_stringer].

*"No over-engineering"* -- ключевой принцип. Декомпозируй до атомарного уровня, не добавляй orchestration overhead сверху.

Причина: каждая fresh-сессия начинает в оптимальных первых 40% контекстного окна [S09, AI Hero]. Монолитная сессия быстро выходит за "smart zone" и деградирует.

### 3.8. Gordon Mickel: "Ralph как malloc orchestrator"

Gordon Mickel [S10] формулирует метафору: контекст = managed memory. Bash loop = allocator. Sprint-state.json = heap. Каждая итерация = malloc + работа + free. "Forgetting > compaction" -- лучше забыть и прочитать заново, чем пытаться сжать [bytesizedbrainwaves.substack.com].

### 3.9. Дополнительные подтверждения из community

Несколько дополнительных источников подкрепляют "everything is a Ralph Loop" с разных ракурсов:

- **Codacy [S28]:** развенчивает распространённые заблуждения о Ralph Loop -- он не "бесконтрольный цикл", а управляемый процесс с checkpoints и exit conditions [blog.codacy.com].
- **Geocodio [S30]:** *"Ship Features in Your Sleep"* -- production usage report показывает, что Ralph Loop работает для реальных feature deliveries, не только для hello-world demos [geocod.io].
- **Aseem Shrey [S31]:** Claude + Codex argue until plan is robust -- iterative plan refinement как ещё одна Ralph-итерация до начала coding [aseemshrey.in].
- **JP Caparas [S32]:** Comprehensive explainer Ralph Loop mechanics -- подтверждает, что цикл масштабируется от простых задач до полных SDLC workflows [blog.devgenius.io].
- **Alibaba Cloud [S33]:** Академическая перспектива -- Ralph как paradigm shift от ReAct (reason-act) к iterate-verify. Формализация loop как архитектурного паттерна [alibabacloud.com].
- **Agent Factory [S38]:** Tutorial по autonomous iteration workflows -- обучающий материал, показывающий entry-level adoption path [agentfactory.panaversity.org].
- **Anthropic Official Plugin [S16]:** Ralph-wiggum plugin (Boris Cherny) -- Stop hook approach для single-session loop. Работает внутри одной сессии [anthropics/claude-code/plugins/ralph-wiggum].
- **Anthropic Tasks [S17]:** Tasks v2.1.16 -- persistent tasks с dependencies (DAGs), broadcast across sessions. Нативная поддержка task management в Claude Code [venturebeat.com].
- **Anthropic Agent Teams [S18]:** Opus 4.6 native Agent Teams -- experimental feature для multi-agent collaboration с shared task list [code.claude.com].
- **claudefa.st task management [S21]:** Tasks как persistent dependency graphs across sessions -- формализация Ralph-паттерна на уровне Claude Code [claudefa.st].
- **The Register [S20]:** Media coverage Ralph Loop phenomenon -- подтверждение widespread adoption в индустрии [theregister.com].

### 3.9. Synthesis: КАЖДАЯ фаза декомпозируется

```
INIT фаза:
  +-- init-1: Read MEMORY.md + CLAUDE.md, create branch      <- 1 Ralph iteration
  +-- init-2: Reindex Serena if stale                         <- 1 Ralph iteration

EXECUTE фаза:
  +-- story-1: Implement nf-10-1 [description]                <- 1 Ralph iteration
  +-- story-2: Implement nf-10-2 [description]                <- 1 Ralph iteration
  +-- story-3: Implement nf-10-3 [description]                <- 1 Ralph iteration

REVIEW фаза:
  +-- review-1: Run all tests + typecheck + lint               <- 1 Ralph iteration
  +-- review-2: AC review for all stories                      <- 1 Ralph iteration
  +-- review-3: Quality review for all stories                 <- 1 Ralph iteration
  +-- review-4: Fix review findings                            <- 1 Ralph iteration
  +-- review-5: Run E2E on localhost                           <- 1 Ralph iteration

FINISH фаза:
  +-- finish-1: Extract discoveries -> MEMORY.md               <- 1 Ralph iteration
  +-- finish-2: Update CLAUDE.md with new domain traps         <- 1 Ralph iteration
  +-- finish-3: Generate sprint summary report                 <- 1 Ralph iteration
```

Каждый `<-` = один запуск `claude -p` с чистым контекстом. Нет compaction. Нет multi-step skills. Нет recovery handlers. Просто **13 итераций** (для типичного спринта из 3 stories) вместо 641 строки orchestration.

**Evidence подтверждает по каждой фазе:**
- **Init**: Carlini -- init через lock файлы; Anthropic Harness -- Initializer Agent как одна задача
- **Execute**: все реализации -- ONE feature per session [единодушный консенсус]
- **Review**: ralphex -- 5 review-агентов; hamelsmu -- Codex review loop; Vercel -- Judge Agent
- **Finish**: Osmani -- 4 memory channels обновляются поитерационно; alfredolopez80 -- automatic learning per iteration

---

## 4. Skills vs State Files: Настоящий Tradeoff

### 4.1. Что дают skills

Skills в Claude Code -- это frontmatter-annotated markdown files, автоматически загружаемые при вызове через Skill tool. Что они обеспечивают [S34, S37]:

| Возможность | Описание | Пример из sprint-run |
|-------------|----------|---------------------|
| **Discovery** | Новый разработчик находит скиллы через `/skills` или tab completion | `/sprint-run Sprint NF-10` |
| **Documentation** | SKILL.md -- self-documenting с frontmatter | `description: Sprint orchestrator...` |
| **Team reuse** | Стандартный интерфейс для всей команды | Все запускают одинаково |
| **Frontmatter routing** | Claude автоматически выбирает подходящий скилл | "Запусти спринт" -> sprint-run |
| **Embedded domain knowledge** | NestJS pitfalls, Prisma gotchas, E2E rules -- всё внутри skill | Инлайновые правила |
| **Compaction recovery** | При потере контекста skill перечитывается целиком | review-state.json recovery |
| **Structured workflow** | Gates, phases, steps -- контролируемый flow | 4 gates в preflight |

### 4.2. Что дают state files

State files (sprint-state.json + sprint-loop.sh + PROMPT_sprint.md) -- внешний loop с file-based persistence. Что они обеспечивают [S01, S03, S05, S10]:

| Возможность | Описание | Пример |
|-------------|----------|--------|
| **Простота** | JSON читается и редактируется вручную, ~25-30 строк bash vs ~641 строка SKILL.md | sprint-state.json |
| **Debuggability** | Видно какие задачи done, какие нет. `git diff sprint-state.json` показывает прогресс | `"passes": true/false` |
| **Cross-tool compatibility** | Bash loop работает с Claude Code, Amp, Cursor, Codex [S15, S24] | Любой агент читает JSON |
| **No compaction risk** | Каждая итерация = чистый контекст. State в файле, не в контексте. Compaction просто невозможна | Файл переживает restart |
| **Deterministic progress** | `passes: boolean` в JSON -- однозначный критерий завершения | Diff = progress log |
| **Resume by design** | Прерванный loop перезапускается, агент читает state file, продолжает | Ctrl+C safe |
| **Version control** | sprint-state.json коммитится в git -- full audit trail | Git history |
| **Composability** | Bash pipeline: `jq | xargs | claude` | Unix philosophy |

### 4.3. AGENTS.md vs CLAUDE.md vs State File

**Факт:** AGENTS.md не поддерживается нативно в Claude Code [S22, issue #6235]. CLAUDE.md -- единственный файл, который Claude Code загружает автоматически при каждой сессии.

| Файл | Autoload in CC | Cross-tool | Назначение в v5 |
|------|---------------|------------|-----------------|
| **CLAUDE.md** | Да (guaranteed) | Нет (CC-only) | Domain knowledge: rules, gotchas, conventions, skills registry |
| **MEMORY.md** | Нет (explicit read) | Нет | Project state: progress, history, VPS info |
| **AGENTS.md** | Нет [S22] | Да (Cursor, Amp, Codex) | Не используем (CC-only project). Опционально: symlink -> CLAUDE.md |
| **sprint-state.json** | Нет (explicit read) | Да (JSON, any tool) | Sprint orchestration: tasks, passes, phases |
| **PROMPT_sprint.md** | Нет (passed via -p) | Да (text prompt) | Iteration instructions: что делать с state file |
| **discoveries.log** | Нет (explicit read) | Да | Append-only per-task learning journal |

**Вердикт:** CLAUDE.md хранит **что нужно знать** (domain knowledge). sprint-state.json хранит **что нужно сделать** (orchestration). MEMORY.md хранит **что было сделано** (history). Каждый файл -- одна ответственность.

Domain knowledge (NestJS pitfalls, E2E rules, design system conventions, architectural traps) ДОЛЖЕН жить в CLAUDE.md. Не в SKILL.md, не в AGENTS.md, не в reference.md.

### 4.4. Когда skills ВСЁ ЕЩЁ нужны

**Atomic, stateless helpers** -- skills, которые решают ОДНУ проблему за ОДИН вызов:

| Skill | Тип | Почему оставить |
|-------|-----|-----------------|
| `nestjs-validation-pipe-whitelist` | Domain knowledge | Решает конкретный NestJS pitfall, один вызов |
| `vitest-fake-timers-async` | Domain knowledge | Паттерн для async polling тестов |
| `nestjs-roles-without-guards` | Domain knowledge | Auth bypass prevention |
| `git-bash-env-escaping` | Platform fix | Windows Git Bash escaping |
| `nestjs-cron-timezone` | Domain knowledge | Cron time zone pitfall |
| `nestjs-cookie-parser-oauth` | Domain knowledge | OAuth cookie-parser requirement |
| `oauth-token-expires-at` | Domain knowledge | Token expiry calculation |
| `vercel-ai-sdk-v6-factory-pattern` | Domain knowledge | SDK migration pattern |
| `nodejs-errno-catch-narrowing` | Domain knowledge | Error handling pattern |
| `figma-screen-builder` | Interactive tool | Stateless Figma automation |
| `figma-to-code` | Interactive tool | Stateless Figma -> React conversion |
| `figma-flow-builder` | Interactive tool | Stateless Figma prototyping |
| `ux-review` | Quality gate | Standalone UX assessment |
| `claudeception` | Knowledge extraction | Standalone session analysis |
| `skill-propagator` | Meta-skill | Standalone skill registration |
| `continue` | Utility | Session continuity |
| `reindex` | Utility | Serena reindex |
| `windows-hooks-fix` | Platform fix | Windows Git Bash hooks |

Общий признак: **нет multi-step workflow, нет state between invocations, нет compaction risk**.

### 4.5. Когда skills НЕ нужны (orchestrators)

**Multi-step orchestrators** -- skills, у которых есть phases, gates, recovery handlers, state files:

| Skill | Строк | Почему заменить |
|-------|-------|-----------------|
| `sprint-run` | ~203 | 5 шагов с зависимостями, compaction recovery, sub-skill invocations |
| `sprint-preflight` | ~148 | 4 blocking gates, sequential dependencies |
| `sprint-review` | ~170 | Parallel reviewers, fix loop, E2E, commit gate |
| `sprint-finish` | ~120 | PR creation, knowledge extraction, summary |

Общий признак: **multi-step workflow с state**, risk of compaction killing progress, embedded domain knowledge, сложный error recovery.

**Чёткое правило:** если skill содержит `while`, `for`, sequential steps, или state recovery -- это orchestrator, и он ДОЛЖЕН быть заменён на state file + bash loop. Если skill -- одноразовая операция с domain knowledge -- это helper, и он остаётся skill.

### 4.6. `.claude/rules/` как альтернатива для checklists

Claude Code поддерживает `.claude/rules/*.md` -- файлы, которые автоматически загружаются как дополнительные инструкции. Review checklists и E2E rules можно вынести сюда:

```
.claude/rules/
  e2e-rules.md          # E2E testing requirements
  review-checklist.md   # AC + quality review criteria
  nestjs-pitfalls.md    # NestJS domain traps
```

Преимущество перед CLAUDE.md: модульность. CLAUDE.md не раздувается.

Текущее распределение knowledge:

| Знание | Сейчас | Куда переносить |
|--------|--------|----------------|
| NestJS ValidationPipe whitelist | sprint-run SKILL.md | CLAUDE.md (уже есть частично) |
| E2E rules (15 строк) | sprint-run SKILL.md | CLAUDE.md или `.claude/rules/e2e-rules.md` |
| Compaction recovery (10 строк) | sprint-run SKILL.md + CLAUDE.md | УДАЛИТЬ (Ralph не нуждается) |
| Review checklists | sprint-review SKILL.md | CLAUDE.md или `.claude/rules/review-checklist.md` |
| Error recovery patterns | sprint-run SKILL.md | CLAUDE.md секция "Error Recovery" |
| Proactive compact rule | sprint-run SKILL.md | УДАЛИТЬ (Ralph не нуждается) |

---

## 5. Конкретная архитектура: sprint-loop.sh + sprint-state.json

### 5.1. sprint-loop.sh

```bash
#!/usr/bin/env bash
# sprint-loop.sh -- Ralph Loop orchestrator for sprints
# Usage: ./sprint-loop.sh [--max-iterations N] [--max-cost DOLLARS]
# Stop:  Ctrl+C (safe -- progress in sprint-state.json)

set -euo pipefail

STATE_FILE="sprint-state.json"
PROMPT_FILE="PROMPT_sprint.md"
MAX_ITER_COUNT=50
ITERATION=0

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --max-iterations) MAX_ITER_COUNT="$2"; shift 2 ;;
    --max-cost) MAX_COST="$2"; shift 2 ;;
    *) shift ;;
  esac
done

# Circuit breaker: 3 consecutive no-progress = stop
NO_PROGRESS_COUNT=0
MAX_NO_PROGRESS=3

echo "=== Sprint Loop started ==="
echo "State: $STATE_FILE"
echo "Prompt: $PROMPT_FILE"
echo "Max iterations: $MAX_ITER_COUNT"

mkdir -p logs

while true; do
  ITERATION=$((ITERATION + 1))

  # Check iteration limit
  if [[ $ITERATION -gt $MAX_ITER_COUNT ]]; then
    echo "STOP: Max iterations ($MAX_ITER_COUNT) reached"
    break
  fi

  # Check if all tasks done
  REMAINING=$(jq '[.tasks[] | select(.passes == false)] | length' "$STATE_FILE")
  if [[ "$REMAINING" -eq 0 ]]; then
    echo "=== Sprint COMPLETE after $ITERATION iterations ==="
    break
  fi

  echo "--- Iteration $ITERATION: $REMAINING tasks remaining ---"

  # Snapshot state before iteration
  BEFORE_HASH=$(md5sum "$STATE_FILE" | cut -d' ' -f1)

  # Run Claude with fresh context
  cat "$PROMPT_FILE" | claude \
    --dangerously-skip-permissions \
    --max-turns 30 \
    --output-format text \
    2>&1 | tee "logs/iteration-${ITERATION}.log"

  # Check progress (circuit breaker)
  AFTER_HASH=$(md5sum "$STATE_FILE" | cut -d' ' -f1)
  if [[ "$BEFORE_HASH" == "$AFTER_HASH" ]]; then
    NO_PROGRESS_COUNT=$((NO_PROGRESS_COUNT + 1))
    echo "WARNING: No progress ($NO_PROGRESS_COUNT/$MAX_NO_PROGRESS)"
    if [[ $NO_PROGRESS_COUNT -ge $MAX_NO_PROGRESS ]]; then
      echo "CIRCUIT BREAKER: $MAX_NO_PROGRESS iterations without progress. Stopping."
      break
    fi
  else
    NO_PROGRESS_COUNT=0
  fi

  # Check for HUMAN pause tasks
  NEXT_TASK=$(jq -r '.tasks[] | select(.passes == false) | .name' "$STATE_FILE" | head -1)
  if [[ "$NEXT_TASK" == HUMAN:* ]]; then
    echo "PAUSE: Human approval required -- $NEXT_TASK"
    echo "Set passes=true in sprint-state.json and restart loop"
    break
  fi

  # Brief cooldown (rate limiting)
  sleep 2
done

echo "Sprint loop finished. Total iterations: $ITERATION"
echo "Remaining tasks:"
jq '.tasks[] | select(.passes == false) | .id + ": " + .name' "$STATE_FILE"
```

**Ключевые решения:**
- **`cat PROMPT_FILE | claude`** -- чистый контекст каждую итерацию [Huntley, ghuntley.com/ralph]
- **`--dangerously-skip-permissions`** -- обязателен для автономной работы [S01, Huntley]. Требует sandboxing [Osmani]
- **`--max-turns 30`** -- ограничение глубины одной итерации
- **Circuit breaker** через md5sum state файла -- если 3 итерации без изменений = stop [frankbria, S05]
- **HUMAN pause detection** -- парсит имя задачи, если `HUMAN:*` -- останавливает loop
- **Логирование** каждой итерации в `logs/` -- аудит и отладка
- **Cooldown 2 сек** -- минимальный rate limiting (frankbria рекомендует 100/hour [S05])
- **Node.js/jq для JSON parsing** -- надёжнее чем bash string manipulation [S14]
- **Safe interrupt:** Ctrl+C в любой момент, прогресс сохранён в state file

### 5.2. sprint-state.json -- полная схема

```json
{
  "sprint": "NF-10",
  "branch": "sprint-nf-10",
  "created": "2026-02-22T12:00:00Z",
  "tasks": [
    {
      "id": "init-01", "phase": "init",
      "name": "Read MEMORY.md and create branch",
      "passes": false, "dependencies": [],
      "description": "Read MEMORY.md + CLAUDE.md. Create branch sprint-nf-10. Verify git status clean."
    },
    {
      "id": "init-02", "phase": "init",
      "name": "Read sprint plan and classify stories",
      "passes": false, "dependencies": ["init-01"],
      "description": "Read sprint plan. For each story: zone, SP, context source. Reindex Serena if stale."
    },
    {
      "id": "exec-01", "phase": "execute",
      "name": "Story nf-10-1: [title]",
      "passes": false, "dependencies": ["init-02"],
      "description": "TDD: write tests for AC, verify RED, implement, verify GREEN. Run tsc + lint + vitest.",
      "story_file": "docs/sprint-artifacts/stories/nf-10-1.md", "zone": "backend", "sp": 3
    },
    // ... exec-02, exec-03 -- same pattern, different story/zone
    {
      "id": "review-01", "phase": "review",
      "name": "Run full test suite (regression check)",
      "passes": false, "dependencies": ["exec-01", "exec-02", "exec-03"],
      "description": "Run ALL tests + typecheck + lint. 0 failures. Fix regressions. Commit fixes."
    },
    {
      "id": "review-02", "phase": "review",
      "name": "Review acceptance criteria",
      "passes": false, "dependencies": ["review-01"],
      "description": "For each story: verify ALL AC met. Check edge cases. Write to sprint-review-findings.md."
    },
    {
      "id": "review-03", "phase": "review",
      "name": "Review code quality",
      "passes": false, "dependencies": ["review-01"],
      "description": "No TODO/FIXME, no hardcoded env vars, no skipped tests. Write to sprint-review-findings.md."
    },
    {
      "id": "review-04", "phase": "review",
      "name": "Fix review findings",
      "passes": false, "dependencies": ["review-02", "review-03"],
      "description": "Read sprint-review-findings.md. Fix ALL findings. Run tests. Commit."
    },
    {
      "id": "review-05", "phase": "review",
      "name": "Run E2E tests",
      "passes": false, "dependencies": ["review-04"],
      "description": "npx playwright test. Retry up to 2 times for flaky. Fix code failures. Do not skip."
    },
    {
      "id": "finish-01", "phase": "finish",
      "name": "Extract discoveries",
      "passes": false, "dependencies": ["review-05"],
      "description": "Read git log + discoveries.log. Extract patterns/gotchas. Update MEMORY.md."
    },
    {
      "id": "finish-02", "phase": "finish",
      "name": "Update CLAUDE.md with new domain traps",
      "passes": false, "dependencies": ["finish-01"],
      "description": "If new pitfalls found, add to CLAUDE.md Lessons Learned."
    },
    {
      "id": "finish-03", "phase": "finish",
      "name": "Generate sprint summary",
      "passes": false, "dependencies": ["finish-02"],
      "description": "Stories completed, SP delivered, test delta, discoveries. Write sprint summary."
    }
  ]
}
```

**Design decisions:**

1. **Flat list, NOT nested phases.** Parsons: *"A for loop is a legitimate orchestration strategy"* [S03]. Phases -- metadata (`phase` field), не structural hierarchy. Фазы не имеют runtime-семантики -- это метаданные для человека. Loop просто идёт по порядку.

2. **Dependencies field.** Задачи выполняются последовательно (первая `passes: false` с удовлетворёнными dependencies). Dependencies -- soft ordering mechanism. В MVP можно игнорировать dependencies и просто идти сверху вниз -- задачи уже в правильном порядке.

3. **Task descriptions -- complete and self-contained.** Каждая задача содержит ВСЁ необходимое для выполнения. Agent не нуждается в контексте от предыдущих задач -- state file + git history + CLAUDE.md + MEMORY.md дают полный контекст [S07, Carlini].

4. **`passes: boolean` -- единственное мутабельное поле.** Agents can ONLY modify `passes` field. Anthropic Harness principle: предотвращает goal drift [S07]. Если задача оказалась слишком большой -- agent добавляет findings в sprint-review-findings.md, но НЕ модифицирует state file structure.

5. **No `status: "in-progress"`.** Только `passes: false` (не сделано) и `passes: true` (сделано). Если iteration прервана -- задача остаётся `false`, следующая iteration подхватит. Dirty tree handling по chrismdp [S03].

6. **`story_file` для rich context.** Если файл есть, агент прочитает его целиком. Если null -- использует описание из `description`. Это заменяет текущую "Story Context Detection" логику из sprint-run шага 2.

### 5.3. PROMPT_sprint.md (~40 строк)

```markdown
# Sprint Execution Prompt

You are executing a sprint for MentorLearnPlatform (NestJS+React).

## Your Task

1. Read `sprint-state.json` -- find the FIRST task where `passes: false`
   AND all dependencies have `passes: true`
2. If no such task exists, output "ALL_TASKS_COMPLETE" and exit
3. If the task has `story_file` -- read it for full context
4. If working tree is dirty from a previous interrupted iteration:
   - Run tests. If pass: commit and mark previous task passes:true
   - If fail: fix, test, commit, then mark passes:true
5. Execute EXACTLY that task (read its `description` carefully)
6. After completing the task:
   a. If all acceptance criteria met -> set `passes: true` in sprint-state.json
   b. `git add` changed files (specific files, NOT -A) + `git commit` with descriptive message
   c. If task CANNOT be completed (blocked, unclear) -> do NOT change passes, write reason to sprint-blockers.md
7. Exit

## Rules (from CLAUDE.md)

- Execute ONE task per session. Do not look ahead.
- Do NOT modify task descriptions or add new tasks.
- Do NOT skip tasks -- execute in order.
- NEVER skip or disable tests (RULE 0)
- NEVER commit secrets or .env files
- Run `tsc --noEmit && npm run lint -- --quiet` after ANY code change
- Run `npm run test` after ANY test-related change
- If tests fail: fix them before marking passes=true
- If stuck after 3 attempts: write to sprint-blockers.md, leave passes=false, exit
- Discoveries: append to docs/sprint-artifacts/discoveries.log with format `[task-id] [ISO date] discovery text`
- COMMIT before exiting. Uncommitted work is LOST.

## Project Context

- Read CLAUDE.md for domain knowledge (NestJS pitfalls, E2E rules, design system).
- Read MEMORY.md for project state (what's done, what's not).
- Monorepo: apps/backend, apps/frontend, packages/shared.
- Backend: NestJS 10 + Prisma ORM. Frontend: React 18 + Vite 5 + Tailwind + shadcn/ui.
- Testing: Vitest 4 (unit/integration), Playwright (E2E).
```

**Ключевые элементы prompt:**

- **Step 1 (Orient + Select)** = бывший Gate 1 из sprint-preflight + outer loop logic. Агент читает state file и выбирает задачу. Dirty tree handling по chrismdp decision table [S03]. Теперь -- часть каждой итерации. 0 строк orchestration, потому что агент делает это сам [S07, Anthropic Harness].

- **Step 5 (Execute)** = inner loop. TDD-first для coding tasks [S36, alexlavaee, ClaytonFarr]. Thorough для review tasks.

- **Rules** = backpressure. *"Backpressure beats direction"* [ClaytonFarr, S36]. Тесты, typecheck, lint -- не опциональны.

- **Step 6 (Complete)** = commit + update state. Git as source of truth [все источники].

**40 строк.** Не 200. Вся domain-specific информация -- в CLAUDE.md (автозагрузка) и story-файлах (по ссылке из sprint-state.json).

### 5.4. Маппинг текущих skills на новую архитектуру

Ключевые изменения при маппинге:

- **sprint-preflight** (148 строк): 4 blocking gates -> 2 задачи в JSON. Gate 1-3 объединены в `init-01`, Gate 4 (TDD) перенесён внутрь story tasks [alexlavaee, ClaytonFarr]
- **sprint-run** (203 строки): Orchestration flow -> sprint-loop.sh. Domain knowledge (NestJS pitfalls, E2E rules) -> CLAUDE.md. Proactive compact rule -> УДАЛЁН
- **sprint-review** (170 строк): 2 параллельных reviewer-агента -> 5 последовательных задач в fresh context. Параллелизм не даёт выигрыша: review ~2-5 минут, координация добавляет complexity [Osmani]
- **sprint-finish** (120 строк): PR creation + knowledge extraction + summary -> 3 отдельных задачи

#### Итого

| Текущий skill | Строк | Замена в v5 | Куда ушёл code |
|--------------|-------|-------------|----------------|
| **sprint-run** | ~203 | sprint-loop.sh (30 строк) | Orchestration -> bash loop. Domain knowledge -> CLAUDE.md |
| **sprint-preflight** | ~148 | init-1, init-2 в state.json | Gates 1-3 -> init tasks. Gate 4 (TDD) -> часть story tasks |
| **sprint-review** | ~170 | review-1..review-5 в state.json | Parallel reviewers -> sequential review tasks. Fix loop -> review-4 task |
| **sprint-finish** | ~120 | finish-1..finish-3 в state.json | PR creation, knowledge extraction -> finish tasks |
| **Domain knowledge** | ~500 | CLAUDE.md sections | NestJS pitfalls, E2E rules, Prisma gotchas -> CLAUDE.md Lessons Learned |

**Итого:** 641 строка -> 30 (bash) + 40 (prompt) + JSON state = ~110 строк. Выигрыш: ~6x меньше orchestration code, 0 compaction incidents.

### 5.5. Knowledge preservation: CLAUDE.md sections

Domain knowledge из skills переезжает в CLAUDE.md. Новая секция `## Sprint Execution Rules` включает:

- **E2E Tests**: Failing E2E = BUG (no exceptions). VPS E2E command. Unacceptable excuses list.
- **Testing**: `npm run typecheck && npm run lint -- --quiet && npm run test` after changes. ESLint `--quiet` для errors only (output truncation lesson from NF-9).
- **NestJS Pitfalls**: ValidationPipe whitelist, @Roles without guards, cookie-parser for OAuth, tokenExpiresAt.
- **Review Checklist**: AC verification (all met, tests exist, edge cases). Code quality (no TODO, no hardcoded env, no skipped tests, tsc clean, lint clean).
- **Knowledge Extraction**: Per-task discoveries.log append. After sprint: update MEMORY.md. New pitfalls: add to CLAUDE.md immediately.
- **Error Recovery**: 3 failed attempts -> STOP. Same approach 2x -> change strategy. WebSearch first. If blocked -> commit + blockers file.

### 5.6. Figma workflow: stays as skills

Figma skills (figma-screen-builder, figma-to-code, figma-flow-builder) **остаются без изменений**. Причина: они stateless, interactive (требуют Figma Desktop + MCP channel), one-shot. Нет multi-step state, нет compaction risk, нет sequential dependencies.

```
Sprint state file может содержать Figma tasks:
{
  "id": "story-ui-1",
  "phase": "execute",
  "task": "Implement UI for Screen 1: Dashboard. Use /figma-to-code skill with Figma frame 136:479.",
  "passes": false
}
```

Agent вызовет `/figma-to-code` через Skill tool -- это не нарушает "everything is a Ralph Loop", потому что skill вызывается ВНУТРИ одной итерации loop.

### 5.7. Circuit Breaker + Rate Limiting

**Circuit breaker** (из frankbria [S05]):
- MAX_ITERATIONS=50 в sprint-loop.sh -- hard limit
- `sleep 2` между итерациями -- rate limiting (configurable)
- Stuck detection через md5sum: если state файл не изменился за 3 итерации -- stop
- HUMAN pause detection: если задача начинается с `HUMAN:` -- stop и уведомить пользователя

**Для MentorLearnPlatform:** 50 итераций достаточно для спринта из 3-5 stories с review. Типичный спринт = 13-15 задач, каждая за 1-2 итерации = 15-30 итераций. Запас 20 итераций -- на retry и fix cycles.

**Cost estimation:** ~$0.80-1.50 per iteration (Opus 4.6, fresh context, ~first 40% of window). 30 итераций = $24-45. Ниже текущего skill-orchestrator ($50-100 per sprint) из-за отсутствия compaction waste.

**Аналоги community:** Vercel: `stopWhen: iterationCountIs(50)` + token budgets + cost caps [S39, tessl.io]. paddo.dev: *"A 50-iteration loop on a large codebase can easily cost $50-100+"* [S12].

### 5.8. Параллельные агенты (Carlini pattern)

Для независимых stories можно запустить параллельные worktrees [S07, S03, S06]:

```bash
# Создать worktrees для параллельных story
git worktree add ../mentorlearn-worker-1 sprint-nf-10
git worktree add ../mentorlearn-worker-2 sprint-nf-10

# Каждый worktree получает свой sprint-state.json с подмножеством задач
# worker-1: backend stories (независимые)
# worker-2: frontend stories (независимые)

# Запустить loop в каждом worktree
(cd ../mentorlearn-worker-1 && ./sprint-loop.sh) &
(cd ../mentorlearn-worker-2 && ./sprint-loop.sh) &
wait

# Merge results
cd ../mentorlearnplatform
git merge worker-1-branch worker-2-branch
```

**Для MentorLearnPlatform:** Параллелизм полезен для независимых frontend + backend stories. Но наши stories часто имеют зависимости (shared package, Prisma schema) -- sequential execution безопаснее. Параллелизм -- opt-in для конкретных спринтов.

Предусловие: stories в разных worktrees ДОЛЖНЫ быть независимыми (разные файлы). Если stories зависят друг от друга -- последовательное выполнение [Carlini, chrismdp].

### 5.9. Генерация sprint-state.json

sprint-state.json не создаётся вручную. Генератор -- отдельная задача (или скрипт):

```bash
# generate-sprint-state.sh -- создаёт sprint-state.json из epics doc
cat <<'PROMPT' | claude --output-format json
Read docs/sprint-plan-ai-pipeline.md and docs/epics-ai-pipeline.md.
For the current sprint (Sprint NF-10), generate sprint-state.json with:
- init tasks (read memory, create branch, classify stories)
- execute tasks (one per story, with zone, SP, story_file path)
- review tasks (test suite, AC review, quality review, fix findings, E2E)
- finish tasks (extract discoveries, update memory, generate summary)

Output format: JSON matching the schema in PROMPT_sprint.md.
PROMPT
```

Это "Initializer Agent" из Anthropic Harness [anthropic.com/engineering] -- запускается один раз, создаёт структуру для Coding Agent loop.

### 5.10. Discoveries log (новая практика)

Вместо монолитной extraction в конце спринта -- **per-iteration append-only log** [Osmani, addyosmani.com]:

```
# docs/sprint-artifacts/discoveries.log
# Format: [task-id] [timestamp] discovery text

[exec-01] [2026-02-22T14:30:00Z] Prisma @unique needed for upsert on computed field
[exec-02] [2026-02-22T15:00:00Z] TanStack Query v5 changed refetchOnWindowFocus default
[review-03] [2026-02-22T16:00:00Z] ESLint output truncation: use --quiet for errors only
```

Каждая итерация Ralph Loop, обнаружив что-то новое, append'ит в этот файл. Finish-задача `extract-discoveries` читает этот файл как input (вместо "вспоминания" из контекста).

### 5.11. Sprint blockers file

Если задача не может быть выполнена (3 попытки), агент пишет в sprint-blockers.md: task ID, описание blocker-а, что пробовал, рекомендация. Оставляет `passes: false`, выходит. Circuit breaker подхватит если stuck.

---

## 6. Migration Plan

### 6.1. Step 1: Create artifacts (Risk: LOW)

**Что создать:**

```
sprint-loop.sh                           # ~50 строк bash (из секции 5.1)
PROMPT_sprint.md                         # ~40 строк prompt (из секции 5.3)
logs/                                    # директория для iteration logs
docs/sprint-artifacts/discoveries.log    # append-only discovery journal
generate-sprint-state.sh                 # генератор sprint-state.json из epics doc
```

**Действия:**
1. Создать `sprint-loop.sh` в корне проекта (скрипт из секции 5.1)
2. Создать `PROMPT_sprint.md` (шаблон из секции 5.3)
3. Создать `logs/.gitkeep`
4. Создать пустой `docs/sprint-artifacts/discoveries.log`
5. Написать `generate-sprint-state.sh` -- генератор sprint-state.json из epics doc
6. Добавить `discoveries.log` (пустой, append-only)

**Как тестировать:** Запустить `sprint-loop.sh --max-iterations 1` с тестовым sprint-state.json (1 trivial task).

**Risk mitigation:** Старые skills остаются в `.claude/skills/` -- можно откатиться к `/sprint-run` в любой момент.

### 6.2. Step 2: Move domain knowledge (Risk: LOW)

Перенести domain knowledge из 4 SKILL.md файлов в CLAUDE.md секцию "Sprint Execution Rules" (см. секцию 5.5). Переносить: NestJS pitfalls, E2E rules, Prisma gotchas, testing patterns, git rules. НЕ переносить: orchestration flow, compaction recovery handlers, sub-agent coordination. Опционально: `.claude/rules/` для модульности (см. секцию 4.6).

**Валидация:** `wc -l CLAUDE.md` -- должен вырасти на ~100-150 строк. Проверить coherence и отсутствие дублирования.

### 6.3. Step 3: Test with one sprint (Risk: MEDIUM)

**Предусловие:** Steps 1-2 выполнены, sprint-state.json сгенерирован для реального спринта.

**Действия:**
1. Сгенерировать sprint-state.json для следующего спринта (2-3 story, ~10 SP)
2. `git checkout -b test-ralph-migration`
3. Запустить `./sprint-loop.sh --max-iterations 20`
4. Наблюдать: логи в `logs/`, прогресс в sprint-state.json, blockers в sprint-blockers.md
5. Измерить все метрики ниже

**Метрики для сравнения:**

| Метрика | Как измерять | Цель |
|---------|-------------|------|
| Строк orchestration | `wc -l sprint-*.SKILL.md` vs `wc -l sprint-loop.sh PROMPT_sprint.md` | -80% |
| Compaction incidents | Подсчёт "context lost" в логах | 0 (was 2-4) |
| Time to first commit | Время от старта до первого git commit | <15 min (was ~45 min) |
| Iterations per sprint | Подсчёт итераций loop | 15-30 (for 3-5 stories) |
| Cost per sprint | API usage | $24-45 (was $50-100 с compaction) |
| Test count delta | Тесты до vs после | >= 0 (no test reduction) |
| Review quality | Bugs found post-merge (1 week) | Same or better |
| Knowledge captured | Lines added to MEMORY.md + CLAUDE.md | >= current |
| Discoveries log entries | Lines in discoveries.log | >= 3 per sprint |

**Метрики успеха:**
- Все задачи `passes: true`
- 0 compaction incidents
- Cost <= текущего sprint-run с compaction (~$15-25)
- Тесты зелёные (existing + new)
- Discoveries log содержит полезные записи

**Если тест провалился:** не мигрировать, вернуться к skill-orchestrator, задокументировать причину.

### 6.4. Step 4: Delete orchestrator skills (Risk: LOW, после успешного Step 3)

Оставить atomic stateless skills (полный список в секции 4.4). Удалить 4 orchestrator skills (список в секции 4.5).

**Risk mitigation:** НЕ удалять сразу. Архивировать в `.claude/skills/archive/sprint-*-v1/`. Удалить через 2-3 успешных спринта.

**Обновить:** CLAUDE.md (убрать sprint-* из реестра, добавить sprint-loop.sh), MEMORY.md (workflow description), удалить review-state.json, reference.md, HOW-IT-WORKS.md.

**Валидация:** `grep -r "sprint-run\|sprint-preflight\|sprint-review\|sprint-finish" .claude/` -- 0 результатов (кроме archive).

### 6.5. Порядок и откат

```
Step 1 (create)  --> Step 2 (move knowledge) --> Step 3 (test) --> Step 4 (delete)
                                                      |
                                                 FAIL?|
                                                      v
                                                 ROLLBACK:
                                                 git checkout master
                                                 (skills intact)
```

Steps 1-2 -- обратимы (дополнительные файлы, не изменяют existing). Step 3 -- в feature branch. Step 4 -- только после успешного Step 3.

### 6.6. Итоговая таблица: что оставить, что удалить, что перенести

| Действие | Артефакты |
|----------|----------|
| **ARCHIVE** | sprint-run, sprint-preflight, sprint-review, sprint-finish SKILL.md -> `.claude/skills/archive/` |
| **MOVE** | Domain knowledge (500 строк) -> CLAUDE.md Sprint Execution Rules |
| **DELETE** | Compaction recovery, review-state.json, reference.md |
| **KEEP** | sprint-status.yaml, bmm-workflow-status.yaml, MEMORY.md, CLAUDE.md (enhanced) |
| **NEW** | sprint-loop.sh, PROMPT_sprint.md, sprint-state.json, discoveries.log, sprint-blockers.md |

---

## 7. Нерешённые проблемы и mitigation

### 7.1. Problem 1: Docker/Infra per-iteration

**Проблема:** Каждая итерация = чистая сессия. Docker, dev server, Serena MCP -- нужна повторная инициализация?

**Mitigation:** Init task (`init-01`, `init-02`) выполняет одноразовую подготовку. Последующие итерации **не перезапускают** инфраструктуру -- она продолжает работать в фоне. Docker containers, dev server, Serena -- запущены один раз и живут до конца спринта.

**В промпте:** Step 5 НЕ содержит "start Docker" -- это ответственность init tasks. Промпт содержит: "If tests fail with connection error, restart dev server."

Два подхода: pre-loop bash скрипт (`docker compose up -d && curl localhost:3000/health`) или init task в sprint-state.json с "check if running, start if not" description.

**Аналог community:** Anthropic Harness init.sh запускается один раз, последующие сессии используют живую инфраструктуру [S07].

### 7.2. Problem 2: E2E Stability

**Проблема:** E2E тесты на VPS нестабильны (network, timing, throttle limits). Одна failed E2E итерация может заблокировать весь спринт.

**Mitigation:**
1. E2E -- отдельная задача (`review-05`) с retry logic в description: *"Run E2E. If fail due to timeout/network: retry up to 2-3 times. If fail due to code: fix."*
2. E2E задача зависит от `review-04` (все findings исправлены) -- минимизирует false failures
3. `THROTTLE_LIMIT=1000` в VPS env -- уже настроено
4. Configurable: E2E можно пометить `"optional": true` для dev sprints, mandatory для release sprints

**Аналог community:** snarktank: browser verification как отдельный step, с retry [S05]. Anthropic: Puppeteer MCP для browser verification [S07].

### 7.3. Problem 3: Inter-task Dependencies

**Проблема:** Story-2 зависит от shared package change в story-1. Параллельное выполнение = git conflicts.

**Mitigation:**
1. **Default: sequential execution.** Tasks в sprint-state.json выстроены в правильном порядке. `dependencies` field обеспечивает порядок.
2. **Explicit dependency declaration.** Story-2 имеет `"dependencies": ["story-1"]` -- loop не возьмёт story-2 пока story-1 не `passes: true`.
3. **Git merge for parallel worktrees.** Если используется parallel execution (Carlini pattern) -- merge через `git merge` с ручным разрешением конфликтов.
4. **Phase grouping.** Independent stories можно группировать в одну фазу, dependent stories -- в разные фазы.

Для параллельного выполнения -- worktrees с НЕЗАВИСИМЫМИ stories (см. секцию 5.8).

**Аналог community:** Carlini -- lock-based task claiming [S07, S25]. chrismdp -- beads-sync branch [S03].

### 7.4. Problem 4: Human Approval Points

**Проблема:** Некоторые точки требуют human decision (approve architecture, review UX, merge to main).

**Mitigation:** `PAUSE` маркер: task с `"name": "HUMAN: Review UI changes"` + `"pause": true`. sprint-loop.sh обнаруживает `HUMAN:*` в имени задачи и останавливает loop (секция 5.1). Пользователь ревьюит, ставит `passes: true` вручную, перезапускает loop [S36, pubnub.com; S05, frankbria].

### 7.5. Problem 5: Cost Monitoring

**Проблема:** Без cost caps loop может потратить сотни долларов. Chris Parsons исчерпал Max5 за часы [S03].

**Mitigation:**
1. **--max-turns flag:** Claude Code поддерживает `--max-turns` -- ограничение числа turns per session
2. **MAX_ITERATIONS в sprint-loop.sh:** hard cap на количество итераций (50 default)
3. **sleep 2 между итерациями:** rate limiting, предотвращает burst spending
4. **Cost logging:** каждая итерация выводит token usage -- append в `sprint-cost.log`
5. **Pre-sprint estimate:** 15-30 итераций x $0.80-1.50/iteration = $12-45 expected. Set MAX_ITERATIONS = 2x expected
6. **Circuit breaker:** 3 iterations without progress = stop (не тратить деньги зря)

**Аналог community:** frankbria: `MAX_CALLS_PER_HOUR=100` [S05]. Vercel: `stopWhen: iterationCountIs(50)` + token budgets + cost caps [S39, tessl.io]. paddo.dev: *"A 50-iteration loop on a large codebase can easily cost $50-100+"* [S12]. Rate limiting code (hourly cap) может быть добавлен в sprint-loop.sh по аналогии с frankbria.

### 7.6. Problem 6: Brownfield Regression Safety

**Проблема:** 1451 тестов. Breaking change в одном модуле может пройти unit тесты модуля, но сломать integration тесты другого.

**Mitigation:**
1. **review-01 task** запускает ALL тесты (`npm run test` во ВСЕХ workspaces), не только тесты изменённых модулей
2. **TDD-first** в каждой story task -- regression suite растёт с каждой story
3. **E2E review-05** -- integration boundary test, ловит cross-module breakage
4. **Промпт:** *"After implementing, run ALL tests (not just related). Fix any failures."*
5. **Git checkpoint:** каждый commit -- точка возврата. `git reset --hard HEAD~1` если commit сломал всё [S02, Dex: "Code is cheap"]

**Аналог community:** Osmani: *"Without checks, an agent might merrily introduce bugs or failing builds while thinking it succeeded"* [S10]. Carlini: massive test suite как backbone [S07]. ClaytonFarr: *"Backpressure beats direction"* [S36].

### 7.7. Problem 7: State File Corruption

**Проблема:** Agent может сломать sprint-state.json (invalid JSON, wrong field modification).

**Mitigation:**
1. **Rule in prompt:** *"You may ONLY change `passes` field from `false` to `true`. Do NOT add, remove, or rename tasks."* [S07, Anthropic]
2. **Validation in loop:** sprint-loop.sh проверяет JSON validity после каждой итерации (`jq` parsing fails on invalid JSON)
3. **Git recovery:** sprint-state.json коммитится -- `git checkout sprint-state.json` восстанавливает последнюю рабочую версию
4. **Schema validation:** опционально -- JSON Schema для sprint-state.json

### 7.8. Problem 8: Large Story Decomposition

**Проблема:** Story на 8 SP не помещается в одну итерацию. Context overflow.

**Mitigation:** chrismdp bead-splitting pattern [chrismdp.com]. В PROMPT_sprint.md:

```markdown
- If a task is too large for one session: DECOMPOSE it.
  Write sub-tasks as comments in sprint-state.json description field.
  Mark the original task as passes=true.
  The next iteration will pick up the sub-tasks.
```

Проблема: агент не может добавлять задачи в sprint-state.json (rule: only modify `passes`). Лучший подход -- pre-decomposition. sprint-state.json генерируется с granularity 1-3 SP на задачу. Story на 8 SP = 3-4 задачи в JSON, не одна.

---

## 8. Sources

### Tier A: Primary sources (original implementations, official docs)

| ID | Source | Author | Key Claim |
|----|--------|--------|-----------|
| S01 | ghuntley.com/ralph | Geoffrey Huntley | Original Ralph Loop -- bash while loop, fresh context per iteration, "compaction is the devil" |
| S02 | humanlayer.dev/blog/brief-history-of-ralph | Dex Horthy / HumanLayer | "Code is cheap, rerunning loops is cheaper than rebasing." Plugin critique |
| S03 | chrismdp.com/your-agent-orchestrator-is-too-clever | Chris Parsons | "A for loop is a legitimate orchestration strategy." Inner+outer loop = enough |
| S04 | ralphex.umputun.dev | umputun | 4-phase pipeline: execute->review1(5 agents)->external->review2(2 agents). Fresh session per task |
| S05 | github.com/snarktank/ralph | snarktank | prd.json with `passes: boolean`, circuit breaker, progress.txt as memory |
| S06 | github.com/AnandChowdhary/continuous-claude | Anand Chowdhary | Full PR lifecycle automation, SHARED_TASK_NOTES.md, parallel worktrees |
| S07 | anthropic.com/engineering/building-c-compiler | Nicholas Carlini / Anthropic | 16 agents, ~2000 sessions, $20K, 100K-line C compiler. Lock-based task claiming |
| S08 | github.com/hamelsmu/claude-review-loop | Hamel Husain | Stop hook -> Codex review -> Claude fixes. State in .claude/review-loop.local.md |
| S16 | anthropics/claude-code/plugins/ralph-wiggum | Anthropic / Boris Cherny | Official plugin, Stop hook approach, /ralph-loop command |
| S17 | venturebeat.com -- Claude Code Tasks | VentureBeat | Tasks v2.1.16: persistent tasks, dependencies (DAGs), broadcast across sessions |
| S18 | code.claude.com/docs/en/agent-teams | Anthropic | Opus 4.6: native Agent Teams, shared task list, experimental |

### Tier B: Analysis, guides, practical reports

| ID | Source | Author | Key Claim |
|----|--------|--------|-----------|
| S09 | aihero.dev | AI Hero | Plugin fills context window, bash loop preserves quality |
| S10 | bytesizedbrainwaves.substack.com | Gordon Mickel | Ralph as "malloc orchestrator". State in files not conversation |
| S11 | medium.com/@himeag | Meag Tessmann | Hybrid: Agent Teams for creative, Ralph for mechanical |
| S12 | paddo.dev/blog/ralph-wiggum-autonomous-loops | paddo.dev | "SDLC collapse" -- phase boundaries dissolve |
| S13 | claudefa.st -- thread hierarchy | claudefa.st | Thread hierarchy: Base->P->C->F->B->L-threads |
| S14 | medium.com/@levi_stringer | Levi Stringer | Atomic task decomposition: $0.77 vs $1.82 |
| S15 | mikeyobrien/ralph-orchestrator | Mikey O'Brien | Git checkpointing, Hat System, 7 AI backends |
| S19 | block.github.io/goose | Block (Goose) | Two-model worker/reviewer, SHIP/REVISE loop |
| S20 | theregister.com | The Register | Media coverage of Ralph Loop phenomenon |
| S21 | claudefa.st -- task management | claudefa.st | Tasks as persistent dependency graphs |
| S23 | humanlayer.dev/blog/writing-a-good-claude-md | HumanLayer | CLAUDE.md best practices: minimal, universally applicable |
| S24 | aiengineerguide.com -- AGENTS.md | AiEngineerGuide | AGENTS.md = cross-platform standard |
| S25 | infoq.com/news/2026/02/claude-built-c-compiler | InfoQ | Carlini: lock-based scheme, isolated Docker containers |
| S26 | reading.torqsoftware.com | TorqSoftware | Plan Mode + structured artifacts survive compaction better |
| S27 | claudefa.st -- context buffer | claudefa.st | Buffer ~33K tokens, CLAUDE_AUTOCOMPACT_PCT_OVERRIDE |
| S28 | blog.codacy.com | Codacy | Common misconceptions about Ralph Loop |
| S30 | geocod.io | Geocodio | "Ship Features in Your Sleep" -- production usage |
| S31 | aseemshrey.in | Aseem Shrey | Claude + Codex argue until plan is robust |
| S32 | blog.devgenius.io | JP Caparas | Comprehensive explainer of Ralph Loop mechanics |
| S33 | alibabacloud.com | Alibaba Cloud | Academic perspective: Ralph as paradigm shift from ReAct |
| S34 | leehanchung.github.io | Lee Han Chung | Skills architecture first principles |
| S37 | gend.co | Gend.co | Skills vs CLAUDE.md -- practical team guide |
| S39 | tessl.io | Tessl | Technical analysis of Ralph pattern mechanics |
| S40 | vibecoding.app | vibecoding.app | PRD-driven loop review with recommendations |

### Tier C: Community resources, tutorials

| ID | Source | Author | Key Claim |
|----|--------|--------|-----------|
| S22 | github.com/anthropics/claude-code/issues/6235 | GitHub Issue | AGENTS.md not natively supported in Claude Code |
| S29 | alfredolopez80/multi-agent-ralph-loop | alfredolopez80 | Memory-driven planning + multi-agent |
| S38 | agentfactory.panaversity.org | Agent Factory | Tutorial: autonomous iteration workflows |

### Internal References

| ID | Source | Description |
|----|--------|-------------|
| S35 | docs/research/ralph-loop-v3-2026-02-21.md | v3 report: ecosystem, 1215 lines, 16+ implementations |
| S36 | docs/research/ralph-loop-gaps-2026-02-21.md | v4 report: gap analysis, 812 lines, 5 unsolved problems |

### Key blog posts and articles (full URLs)

- Geoffrey Huntley: https://ghuntley.com/ralph/, https://ghuntley.com/loop/
- Anthropic Engineering: https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents
- Anthropic C Compiler: https://www.anthropic.com/engineering/building-c-compiler
- Chris Parsons: https://www.chrismdp.com/your-agent-orchestrator-is-too-clever/
- Addy Osmani: https://addyosmani.com/blog/self-improving-agents/
- HumanLayer: https://www.humanlayer.dev/blog/brief-history-of-ralph
- AI Hero: https://aihero.dev/why-the-anthropic-ralph-plugin-sucks
- Levi Stringer: https://medium.com/@levi_stringer
- Nick Tune: https://www.oreilly.com/radar/auto-reviewing-claudes-code/
- PubNub: https://www.pubnub.com/blog/best-practices-for-claude-code-sub-agents/
- Clayton Farr: https://claytonfarr.github.io/ralph-playbook/
- vibecoding.app: https://vibecoding.app/blog/ralph-wiggum-loop-review
- alexop.dev: https://alexop.dev/posts/custom-tdd-workflow-claude-code-vue/
- Alex Lavaee: https://alexlavaee.me/blog/ai-coding-infrastructure/

### GitHub repositories

- snarktank/ralph: https://github.com/snarktank/ralph
- frankbria/ralph-claude-code: https://github.com/frankbria/ralph-claude-code
- hamelsmu/claude-review-loop: https://github.com/hamelsmu/claude-review-loop
- AnandChowdhary/continuous-claude: https://github.com/AnandChowdhary/continuous-claude
- vercel-labs/ralph-loop-agent: https://github.com/vercel-labs/ralph-loop-agent
- alfredolopez80/multi-agent-ralph-loop: https://github.com/alfredolopez80/multi-agent-ralph-loop
- ClaytonFarr/ralph-playbook: https://github.com/ClaytonFarr/ralph-playbook
- agrimsingh/ralph-wiggum-cursor: https://github.com/agrimsingh/ralph-wiggum-cursor
- anthropics/claude-code plugins/ralph-wiggum: https://github.com/anthropics/claude-code/blob/main/plugins/ralph-wiggum/README.md
- snwfdhmp/awesome-ralph: https://github.com/snwfdhmp/awesome-ralph
