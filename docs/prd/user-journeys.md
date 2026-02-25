# User Journeys

### Architecture Decision: Two-Phase Iteration Model

На основании deep research (4 отчёта в `docs/research/deep-research-ralph-review/`) принято архитектурное решение:

**Три типа сессий:**

1. **EXECUTE** — свежая сессия: Claude читает sprint-tasks.md, берёт первую `- [ ]` задачу. Если review-findings.md не пуст — сначала исправляет findings. Если пуст — реализует задачу с нуля. Запускает тесты (unit + e2e), коммитит при green. Execute-промпт включает инструкцию записать learnings в LEARNINGS.md перед завершением (best effort)
2. **REVIEW** — свежая сессия: 4 параллельных sub-агента через Task tool, верификация находок. **Review ТОЛЬКО анализирует — ничего не фиксит.** При findings — перезаписывает review-findings.md (только текущие проблемы, без task ID) + записывает уроки в LEARNINGS.md и обновляет секцию ralph в CLAUDE.md. При clean review — ставит `[x]`, очищает review-findings.md, distillation LEARNINGS.md если превышен бюджет
3. **RESUME-EXTRACTION** (`claude --resume <session-id>`) — краткое возобновление execute-сессии при неуспехе (нет коммита). Execute имеет полный контекст — знает что пыталась, где остановилась. Задачи: (1) коммитит WIP-состояние, (2) пишет прогресс в sprint-tasks.md (под текущей задачей), (3) записывает знания в LEARNINGS.md + CLAUDE.md секцию. По умолчанию: при неуспехе. `--always-extract`: после каждого execute

**Loop:** execute → [resume-extraction при неуспехе] → review (только при наличии коммита). Если review нашёл проблемы → перезаписывает review-findings.md + записывает уроки → следующий execute адресует findings → review проверяет фиксы (max `max_review_iterations` циклов, default 3). После лимита — emergency human gate.

**Ключевые упрощения:**
- Ralph не различает "первый execute" и "fix после review" — одна и та же сессия. Execute смотрит: review-findings.md пуст? → реализовать. Не пуст? → исправить
- review-findings.md — транзиентный файл для текущей задачи. Перезаписывается review при findings (только актуальные проблемы). Не требует task ID
- sprint-tasks.md — open format. Bridge создаёт структуру, Claude пишет в ней свободно. Ralph парсит только `- [ ]`, `- [x]`, `[GATE]`. Resume-extraction пишет прогресс прямо в sprint-tasks.md под текущей задачей
- Review сама записывает findings-знания в LEARNINGS.md + CLAUDE.md (без отдельной extraction-сессии)
- После clean review: review-findings.md очищается → distillation LEARNINGS.md при превышении бюджета → следующая задача

**Принцип:** "One context window, one activity, one goal" (Huntley). Review = только ревью. Реализация/фиксы = только execute.

**Обоснование:**
- Канонический Ralph (snarktank, ghuntley): ревью отсутствует, только backpressure
- Farr Playbook: ревью = backpressure внутри итерации, нет sub-агентов
- Ralphex (umputun): свежие сессии между фазами — прецедент. Мы идём дальше: review НЕ фиксит (чистое разделение ответственности)
- Community consensus: resume для ревью нарушает философию Ralph; свежий контекст + параллельные sub-агенты = best practice

**Частота ревью (MVP):** review после каждой задачи (review-every 1). Batch review (`--review-every N`) — Growth feature

### Journey 1: "Запустил и ушёл" (Happy Path)

Алексей — mid-level фронтенд, первый раз пробует bmad-ralph. У него BMad stories для CRUD-модуля (8 задач, 1 epic).

```
$ ralph bridge stories/crud-module.md
✓ Parsed 8 acceptance criteria
✓ Generated sprint-tasks.md (8 tasks + 2 service tasks)
✓ Human gates: TASK-1 (first in epic), TASK-6 (new UI screen)

🚦 HUMAN GATE: Review sprint-tasks.md before run
   [a]pprove  [r]etry with feedback  [s]kip  [q]uit
> a

$ ralph run
⏳ TASK-1: Setup project structure
  → EXECUTE: fresh session... ✓ (unit green, committed)
  → REVIEW: 4 sub-agents... ✓ clean (0 findings)
🚦 HUMAN GATE: First task in epic — verify direction
> a

⏳ TASK-2: Implement data model
  → EXECUTE: fresh session... ✓ (unit green, committed)
  → REVIEW: 4 sub-agents... found 2 issues → review-findings.md
  → EXECUTE: fresh session, sees findings... fixes, tests green ✓ committed
  → REVIEW: 4 sub-agents... ✓ clean, marks [x]
⏳ TASK-3: API endpoints
  → EXECUTE: fresh session... ✓
  → REVIEW: 4 sub-agents... ✓ clean
⏳ TASK-4..5: Validation + edge case tests
  → EXECUTE + REVIEW: ✓ (2 tasks, no issues)
⏳ TASK-6: List + Detail UI  [UI task]
  → EXECUTE: fresh session... ✓ (unit + e2e green — UI task triggers e2e)
  → REVIEW: 4 sub-agents... ✓ clean
🚦 HUMAN GATE: New UI screen — check in browser
> a

⏳ TASK-7..8: Form UI + E2E tests
  → EXECUTE + REVIEW: ✓

✅ Sprint complete! 8/8 tasks done.
   Commits: 10 (8 tasks + 2 fix commits)
   LEARNINGS.md updated with 3 new patterns
```

**Aha-moment:** Одна команда — 8 задач выполнены автономно. Проверил направление на первой задаче, глянул UI на шестой, и всё.

### Journey 2: "Ревью нашёл критический баг" (Execute → Review Loop)

Марина — senior backend, auth-модуль. Review находит race condition.

```
⏳ TASK-3: JWT token validation
  → EXECUTE: fresh session... ✓ (unit green, committed)
  → REVIEW: 4 sub-agents...
    quality:        ⚠ CRITICAL — race condition in token refresh
    implementation: ✓ AC met
    simplification: ✓ clean
    test-coverage:  ⚠ HIGH — missing e2e test for concurrent refresh
  → Verifying findings... 2 confirmed → review-findings.md

  → EXECUTE: fresh session, sees review-findings.md
    Fixing: race condition + adding e2e test... tests green ✓ committed

  → REVIEW (cycle 2/3): 4 sub-agents...
    quality:        ✓ race condition fixed
    test-coverage:  ✓ e2e concurrent test added, all e2e green
  → ✓ clean, marks [x]

⏳ TASK-4: ...continues
```

**Ключевое:** Review ТОЛЬКО нашёл и верифицировал проблемы — записал в файл. Следующий execute (та же сессия по типу, свежий контекст) увидел findings и исправил. Повторный review проверил фиксы (cycle 2 из max 3). Ralph не различает "первый execute" и "fix" — это одна и та же сессия.

### Journey 3: "AI застрял" (Emergency Human Gate)

Дмитрий — mid-level, интеграция со сторонним API. API вернул неожиданный формат.

```
⏳ TASK-4: Parse partner API response
  → EXECUTE: fresh session... ✗ (tests red — unexpected XML instead of JSON)
  → EXECUTE (retry 1): fresh session with failure context... ✗
  → EXECUTE (retry 2): fresh session... ✗

🚨 EMERGENCY HUMAN GATE: AI stuck after 3 retries
   Task: TASK-4 — Parse partner API response
   Error: Expected JSON, got XML. Test: test_parse_response

   [f]eedback (add context, retry)
   [s]kip task
   [q]uit sprint
> f
> "API changed to XML. Use xml2js, schema in docs/partner-api-v2.xsd"

⏳ TASK-4 (retry with feedback):
  → EXECUTE: fresh session + user feedback... ✓ (unit green)
  → REVIEW: 4 sub-agents... ✓ clean
```

**Ключевое:** AI честно признал что застрял. Человек дал контекст — одна итерация и задача решена.

### Journey 4 (Growth): "Batch Review" (Review Every N)

Олег — senior full-stack, рефакторинг (20 задач). Хочет экономить на ревью.

```
$ ralph run --review-every 5
⏳ TASK-1..4: Execute only (unit tests, no review yet)
⏳ TASK-5: Execute ✓ (unit green)
  → e2e checkpoint (review-every 5)... e2e green ✓
  → REVIEW (batch: TASK-1..5):
    Аннотированный diff по задачам + маппинг TASK → AC → tests
    4 sub-agents review cumulative diff...
    quality:        ⚠ HIGH — circular import (TASK-3)
    implementation: ✓
    simplification: ⚠ MEDIUM — duplicate util (TASK-2 & TASK-3)
    test-coverage:  ✓ all AC covered
  → 2 confirmed findings → review-findings.md

  → EXECUTE: fresh session, sees findings... fixes both issues, unit green ✓ committed
  → REVIEW (cycle 2/3): ✓ clean, marks [x] for TASK-1..5

⏳ TASK-6..10: next batch, review + e2e after TASK-10
```

**Ключевое:** При batch review, review-сессия получает аннотированный diff (разбит по задачам) и маппинг TASK → AC → expected tests, чтобы sub-агенты не потеряли контекст.

### Journey 5 (Phase 2): "Correct Flow" — курс поменялся

Степан использует bmad-ralph на своём проекте. На human gate понимает, что story нуждается в правке.

```
🚦 HUMAN GATE: TASK-6 — Payment form
   [a]pprove  [r]etry  [c]orrect  [s]kip  [q]uit
> c
> "Нужен не Stripe, а Tinkoff Pay. Переделай story."

→ Claude правит оригинальную BMad story (source of truth)
→ Автоматический re-bridge: sprint-tasks.md обновлён
→ TASK-6 перегенерирован с новыми AC
→ Продолжение ralph run с обновлённой задачей
```

**Ключевое (Phase 2):** Source of truth — BMad story. Correct flow правит story, а не sprint-tasks.md напрямую.

### Requirements Summary

| Требование | Journey | Приоритет |
|-----------|---------|-----------|
| Двухфазная итерация: execute → review (свежие сессии, единый тип execute) | 1, 2 | MVP |
| 4 параллельных review sub-агента через Task tool | 1, 2 | MVP |
| Execute→review loop с max_review_iterations (default 3) | 2 | MVP |
| Emergency human gate на max_review_iterations exceeded | 2 | MVP |
| Emergency human gate при N execute retry failures | 3 | MVP |
| User feedback → retry с контекстом | 3 | MVP |
| Review после каждой задачи (review-every 1) | 1, 2 | MVP |
| Review-сессия ставит `[x]` при clean review | 1, 2 | MVP |
| `--review-every N` (batch review) + аннотированный diff | 4 | Growth |
| Human gates на first-in-epic и user-visible milestones | 1 | MVP |
| E2e тесты запускаются в execute-фазе, test-coverage агент верифицирует покрытие | 1, 2 | MVP |
| Knowledge extraction (LEARNINGS.md) | 1 | MVP |
| Correct flow: правка BMad story → re-bridge | 5 | MVP Phase 2 |
