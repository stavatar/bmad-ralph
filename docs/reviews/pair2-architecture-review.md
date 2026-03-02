# Пара 2: Ревью архитектуры и интеграции — Epic 6 v4

**Дата:** 2026-03-02
**Ревьюеры:** Аналитик + Архитектор (Пара 2)
**Scope:** Архитектурная целостность, dependency direction, интеграция с существующим кодом, скрытая сложность
**Источники:** Epic 6 v4 (78 AC, 9 stories), консолидированный ревью (28 находок), 4 research reports, source code (runner/, config/, gates/, session/, cmd/ralph/)

---

## Критические проблемы (блокеры)

### [P2-C1] Snapshot-diff модель (C1) создаёт гонку данных при любом нештатном завершении сессии

- **Описание:** V4 принял решение: Claude пишет в LEARNINGS.md напрямую, Go делает snapshot перед сессией и diff после. Предыдущие ревью (Пара 1, Пара 4) рекомендовали pending-file pattern, но v4 ОТКЛОНИЛ его в пользу snapshot-diff. Это решение архитектурно опасно.
- **Почему плохо:**
  1. **Kill -9 / OOM / crash:** Если сессия Claude убита до завершения, LEARNINGS.md содержит частично записанные данные. Go не запустит diff (сессия не "завершилась"), файл остаётся в повреждённом состоянии. Crash recovery (M7) восстанавливает только .bak от distillation — не от обычной записи.
  2. **Параллельные file writes:** Claude может записать LEARNINGS.md в середине выполнения задачи (не только в конце). Если Go snapshot берётся в начале сессии, а Claude пишет несколько раз за сессию — diff покажет финальное состояние, но промежуточные записи могут быть затёрты самим Claude при переписывании файла.
  3. **Diff парсинг хрупок:** Определение "новых entries" через текстовый diff файла markdown с произвольным контентом — нетривиальная задача. Claude может отредактировать существующие entries (исправить опечатку), а diff покажет это как "удалённый + добавленный" entry, ломая деdup логику.
  4. **Pending-file pattern решал все эти проблемы** — транзакционная запись, атомарная обработка, нет partial state. V4 отверг его без достаточного обоснования.
- **Опции исправления:**
  A) Вернуть pending-file pattern (`.ralph/pending-lessons.md`) — Claude пишет туда, Go обрабатывает атомарно. Самый надёжный вариант.
  B) Добавить checksumming: Go считает SHA256 snapshot, после сессии проверяет. При несовпадении с ожидаемым diff — log warning, tag entire new content как `[needs-formatting]`. Не решает kill -9.
  C) Принять snapshot-diff, но добавить обязательный pre-session backup LEARNINGS.md (не только при distillation). При аварии — restore из backup. Дешевле pending-file, но менее надёжно.

### [P2-C2] [needs-formatting] tag нарушает архитектурный инвариант "Go NEVER пишет контент" из v4

- **Описание:** V4 заявляет (строка 17): "Go-side WriteLessons НЕ пишет контент — Go только post-validates." Одновременно Story 6.1 AC: "invalid entries tagged [needs-formatting] IN the file". Таgging entries в файле — это ЗАПИСЬ КОНТЕНТА. Go модифицирует LEARNINGS.md, добавляя `[needs-formatting]` тег к entries, написанным Claude.
- **Почему плохо:**
  1. **Прямое противоречие** с архитектурным принципом v4. Либо Go пишет, либо нет — полумеры создают путаницу.
  2. **Ломает idempotency:** Повторный запуск post-validation на том же файле увидит `[needs-formatting]` entries, попытается повторно их обработать.
  3. **Distillation зависимость:** ValidateDistillation criterion #7 требует обработки `[needs-formatting]` entries. Если Go пишет тег — это вводит связь между post-validation и distillation через побочный эффект в файле.
  4. **Консолидированный ревью (C4) уже определил** `[needs-formatting]` как "мёртвую ветку" и рекомендовал reject + log. V4 ПРОИГНОРИРОВАЛ это решение.
- **Опции исправления:**
  A) Reject + log (как рекомендовали все 5 пар): невалидные entries = удалить из файла, записать warning в лог. LEARNINGS.md = всегда чистый.
  B) Если нужна zero-knowledge-loss гарантия: записать отвергнутые entries в отдельный `.ralph/rejected-lessons.log` (не в LEARNINGS.md).
  C) Принять `[needs-formatting]` tag, но обновить архитектурный принцип: "Go post-validates AND tags, but does NOT write lesson content."

### [P2-C3] DistillState хранится в `LEARNINGS.md.state` — coupling состояния с данными нарушает SRP

- **Описание:** `DistillState` (MonotonicTaskCounter, LastDistillTask, Categories, Metrics) персистируется в `{projectRoot}/LEARNINGS.md.state`. Этот файл — state машина для distillation, привязанная по имени к LEARNINGS.md.
- **Почему плохо:**
  1. **SRP нарушение:** DistillState хранит категории, метрики, счётчики задач — это runner state, не LEARNINGS.md state. Имя файла вводит в заблуждение.
  2. **Backup fragility:** При distillation backup-ится LEARNINGS.md + ralph-*.md. State файл НЕ упомянут в backup. Если distillation падает между записью state и файлов — рассинхронизация MonotonicTaskCounter с реальным состоянием.
  3. **Location mismatch:** LEARNINGS.md в project root, ralph-*.md в `.claude/rules/`, state в project root. Три разных места для одной подсистемы. `.ralph/distill-state.json` было бы каноничнее.
  4. **Race condition (L6):** `ralph distill` и `ralph run` могут одновременно читать/писать state file. Advisory note — не решение; JSON parse error на half-written file = crash или data loss.
- **Опции исправления:**
  A) Переместить в `.ralph/distill-state.json`. Включить в backup при distillation. State и данные — в разных файлах, но в предсказуемых местах.
  B) Использовать flock на state file (единственный файл, требующий блокировки). Остальные файлы — через backup/restore.
  C) Минимум: переименовать в `.ralph/knowledge-state.json`, включить в backup, документировать формат.

---

## Серьёзные проблемы

### [P2-H1] Dependency direction: knowledge.go в runner/ становится "жирным" пакетом с 4+ ответственностями

- **Описание:** Epic 6 расширяет `runner/knowledge.go` с текущих 27 строк (1 interface, 1 struct, 1 no-op) до пакета с:
  - `FileKnowledgeWriter` (post-validation, 6 gates, diff parsing)
  - `BudgetCheck` (free function)
  - `buildKnowledgeReplacements` (shared function для 3 call sites)
  - `AutoDistill` (LLM session, backup, restore, timeout, validation)
  - `ValidateDistillation` (8 criteria)
  - `ValidateLearnings` (JIT citation validation)
  - `ProcessDistillOutput` (parsing BEGIN/END markers, multi-file write)
  - Scope hint detection (file extension scanning)
  - DistillState serialization
  - Category management (canonical list + NEW_CATEGORY)
  - freq:N validation + correction
- **Почему плохо:** Это минимум 500-800 строк в одном файле. runner/runner.go уже 764 строки. Добавление такого объёма в runner/ увеличивает coupling и затрудняет тестирование. `AutoDistill` запускает `claude -p` сессию — это session-level операция, живущая в runner.
- **Опции исправления:**
  A) Создать пакет `knowledge/` на уровне runner. Dependency: `runner → knowledge → config, session`. Не нарушает направление зависимостей. Knowledge = лист, как gates.
  B) Если не хочется нового пакета: split knowledge.go на knowledge_write.go, knowledge_read.go, knowledge_distill.go, knowledge_state.go. Минимум 4 файла.
  C) Оставить в одном файле, но жёстко ограничить: только types + interfaces в knowledge.go, реализация в отдельных файлах.

### [P2-H2] Human GATE для distillation (C2) не интегрируется с существующей gates архитектурой

- **Описание:** Существующая gates система: `gates.Gate` struct → `gates.Prompt()` → `*config.GateDecision`. GatePromptFunc в runner — injectable closure. Story 6.5 описывает human gate с опциями "retry once, retry 5-10 times, skip" — это ДРУГОЙ набор actions (не approve/retry/skip/quit).
- **Почему плохо:**
  1. **Два разных формата gate:** Существующий gate: approve/retry/skip/quit. Distillation gate: retry-once/retry-N/skip. Это два разных UI pattern, но оба называются "human gate".
  2. **GateDecision struct:** `Action string, Feedback string` — Action принимает строки из `config.ActionApprove` и др. Distillation gate actions ("retry 5-10 times") не вписываются в эту модель без расширения.
  3. **Emergency gate уже есть** (Story 5.5): `Gate{Emergency: true}` — без [a]pprove. Distillation gate = третий variant, с количественным retry.
  4. **Injectable function:** Runner уже имеет `GatePromptFn`. Distillation gate = ещё один injectable fn? Или reuse GatePromptFn с другими опциями? Не специфицировано.
- **Опции исправления:**
  A) Расширить `config.GateDecision` с полем `RetryCount int`. Action = "retry" + RetryCount = 5. Создать `DistillGate` вариант в gates/ с соответствующим UI. Одна система, три варианта.
  B) Отдельный `DistillGateFunc` injectable — проще, но дублирует gate инфраструктуру.
  C) Упростить distillation gate до стандартного retry/skip (без "retry N times"). Retry = одна попытка. Если нужно N — пользователь вводит retry N раз вручную. Минимальное изменение.

### [P2-H3] A/B testing (Story 6.9) добавляет метрики в DistillState — смешение concerns

- **Описание:** Story 6.9 хранит A/B метрики (repeat_violations, findings_per_task, first_clean_review_rate) в DistillState. DistillState уже содержит MonotonicTaskCounter, LastDistillTask, Categories, distillation metrics.
- **Почему плохо:**
  1. **DistillState = God object.** Счётчики задач, категории, метрики дистилляции, метрики A/B — четыре разных домена в одном JSON.
  2. **A/B метрики не связаны с distillation.** Они связаны с injection mode. Хранить injection metrics рядом с distillation state = conceptual leak.
  3. **Размер:** DistillState растёт с каждой story. К Story 6.9 это ~15+ полей. JSON без схемы — хрупкий контракт.
- **Опции исправления:**
  A) Отдельный файл: `.ralph/ab-metrics.json`. DistillState хранит только distillation-related state.
  B) Вложенные structs: `DistillState{Distill: DistillData, Tasks: TaskData, AB: ABMetrics}`. JSON организован, но всё ещё один файл.
  C) Принять как technical debt для v1. Refactor в Growth phase при добавлении новых метрик.

### [P2-H4] Cross-language scope hints (M4/M12): explosion правил при mixed stacks

- **Описание:** Go сканирует top 2 levels, собирает extensions, мапит на language globs. Monorepo: ALL detected → combined scope hints. Distillation создаёт ralph-{category}.md с globs.
- **Почему плохо:**
  1. **Fullstack monorepo:** Go + Python + JS/TS + Java = 4+ языка. Каждый category file получает globs для ВСЕХ языков? Или Go определяет per-category language? Не специфицировано.
  2. **Пример:** `ralph-testing.md` — для Go тестов нужны `["*_test.go"]`, для Python — `["test_*.py", "*_test.py"]`, для JS — `["*.test.ts", "*.spec.ts"]`. Если ВСЕ globs в одном файле — файл загружается ВСЕГДА (effectively `["**"]`), defeating progressive disclosure.
  3. **Category drift:** "testing" в Go-проекте = `*_test.go`, в Python = `pytest`, в JS = `jest/vitest`. Одна категория, разные ecosystems. При distillation Claude может не разделить корректно.
  4. **Top 2 levels:** В monorepo с `services/auth/`, `services/api/`, `libs/common/` — extensions на 2 уровнях не достаточны. `services/auth/src/` — 3 уровня.
- **Опции исправления:**
  A) Per-category language detection: если entry citations ссылаются на `.py` файлы, category file получает Python globs. Citations = ground truth для language scope.
  B) Не пытаться cross-language в v1. Scope hints = project-wide (все detected languages). Progressive disclosure через categories, не через language scoping. Проще, честнее.
  C) Top 3 levels вместо 2. Или `go.mod`/`package.json`/`Cargo.toml` detection как proxy для language (точнее, чем extension scan).

### [P2-H5] `buildKnowledgeReplacements` (H7) создаёт скрытую зависимость runner → filesystem при каждом prompt assembly

- **Описание:** `buildKnowledgeReplacements(projectRoot)` вызывается для КАЖДОГО из 3 AssemblePrompt call sites. Функция читает LEARNINGS.md, все ralph-*.md файлы, запускает ValidateLearnings (N * os.Stat calls), конкатенирует, реверсирует.
- **Почему плохо:**
  1. **Latency:** Каждый prompt assembly = N stat calls + M file reads. При 50 LEARNINGS.md entries + 8 ralph-*.md файлов = ~60 filesystem operations. На WSL/NTFS = 60-300ms. Три call sites = до 900ms overhead per iteration (если не cached).
  2. **Нет кеширования:** Knowledge content не меняется МЕЖДУ execute и review в одной iteration. Три вызова buildKnowledgeReplacements читают одни и те же файлы трижды.
  3. **Error handling:** Если os.Stat fails (network NTFS glitch), промпт собирается без knowledge? Или ошибка? Не специфицировано.
- **Опции исправления:**
  A) Cache buildKnowledgeReplacements результат per-iteration. Invalidate при post-validation (новые entries) или distillation (файлы обновились).
  B) Одно чтение в начале iteration, результат передаётся в RunConfig struct. Чисто, тестируемо, нет скрытого IO.
  C) Ленивая загрузка с TTL (~30s). Для CLI-tool-without-server это overengineering.

---

## Замечания

### [P2-M1] `config.TemplateData` растёт без ограничений — нет валидации полей

- **Описание:** TemplateData уже имеет 8 полей. Epic 6 добавляет `HasLearnings bool`. Story 6.9 может добавить `InjectionMode string`. Нет compile-time или runtime проверки, что все поля заполнены корректно.
- **Рекомендация:** Добавить `Validate() error` метод на TemplateData. Проверяет: HasLearnings=true requires LearningsContent!="". GatesEnabled=true requires HasExistingTasks=false (или что архитектурно корректно). Дёшево, ловит баги.

### [P2-M2] Story 6.3: `--resume` + `-p` compatibility (M1) меняет session.go контракт

- **Описание:** Session.go текущий код исключает Resume и Prompt. Story 6.3 убирает mutual exclusivity. Это меняет контракт session пакета.
- **Рекомендация:** Убедиться, что `claude --resume <id> -p <prompt>` действительно работает в текущей версии Claude CLI. Если нет — fallback к отдельной extraction session (как рекомендовала Пара 1). Проверить ДО реализации, не во время.

### [P2-M3] ValidateDistillation 8 критериев — deterministic, но expensive

- **Описание:** Criterion #3 (last 20% preserved), #4 (citation >= 80%), #6 (category >= 80%) требуют парсинга OLD и NEW content, extraction всех citations, comparison. Это ~100-200 строк Go code только для валидации.
- **Рекомендация:** Каждый criterion = отдельная функция с unit test. НЕ один monolithic ValidateDistillation. Table-driven test с per-criterion failure scenarios.

### [P2-M4] ralph-index.md — metadata файл в .claude/rules/ может загружаться Claude Code

- **Описание:** `.claude/rules/ralph-index.md` = auto-generated TOC. Если Claude Code auto-loads все `.md` файлы из `.claude/rules/`, index file тоже загрузится. Это waste tokens (TOC = metadata, не правила).
- **Рекомендация:** Либо дать ralph-index.md пустой globs: `globs: []` (never auto-loaded), либо переместить в `.ralph/knowledge-index.md` (вне rules/).

### [P2-M5] `CodeIndexerDetector` interface — consumer-side, но определён в runner (правильно)

- **Описание:** Interface `CodeIndexerDetector{Available(projectRoot) bool, PromptHint() string}` определяется и потребляется в runner. Реализация тоже в runner (reads JSON config files). Это правильно по naming convention проекта (interfaces in consumer package). Но если появится вторая реализация (не Serena), интерфейс придётся вынести.
- **Рекомендация:** OK для v1. Документировать: interface расширяется при добавлении нового code indexer.

### [P2-M6] No timeout / no fallback при MCP detection (Story 6.7)

- **Описание:** `Available(projectRoot)` читает `.claude/settings.json` или `.mcp.json` и парсит JSON. На сетевом NTFS чтение может зависнуть. Нет timeout.
- **Рекомендация:** `os.ReadFile` с предварительным `os.Stat` и size check (< 1MB). Panic-safe: any error → false. Документировать: detection = best-effort.

### [P2-M7] Двойная инъекция: `__LEARNINGS_CONTENT__` + `__RALPH_KNOWLEDGE__` — потенциальное дублирование

- **Описание:** LEARNINGS.md = raw hot entries. ralph-*.md = distilled entries. После distillation часть entries из LEARNINGS.md попадает в ralph-*.md. При следующей injection ОБА инъецируются. Entries, прошедшие distillation, присутствуют в ОБОИХ источниках до следующей очистки LEARNINGS.md.
- **Рекомендация:** Distillation ЗАМЕНЯЕТ LEARNINGS.md (compressed output). Если это так — дублирования нет. Но если Claude между distillations записывает новые entries в LEARNINGS.md, а ralph-*.md уже содержат предыдущие — частичное дублирование неизбежно. Добавить dedup при injection (buildKnowledgeReplacements проверяет пересечение).

### [P2-M8] `freq:N` correction by Go — скрытая мутация LLM output

- **Описание:** Story 6.5 (M11): "Go validates monotonicity, corrects Claude's arithmetic errors." Go модифицирует LLM output перед записью. Это нарушает принцип "LLM generates, Go validates".
- **Рекомендация:** Go ВАЛИДИРУЕТ freq:N. При нарушении монотонности — log warning + use MAX(old, new). НЕ "correct" — а выбрать безопасное значение. Семантика: Go не исправляет Claude, Go защищает инвариант.

---

## Что хорошо

1. **Dependency direction сохранена.** `runner → config`, `runner → session`, `runner → gates` — Epic 6 не добавляет обратных зависимостей. Новый код живёт в runner/knowledge.go, читает config, вызывает session. `config` остаётся leaf package (только HasLearnings bool добавлен в TemplateData).

2. **Injectable functions pattern.** DistillFn следует установленному паттерну ReviewFn/GatePromptFn/ResumeExtractFn. Тестируемость сохранена. Mock injection для integration tests — прямолинейный.

3. **MCP решение (C3) архитектурно верное.** Minimal interface, prompt-based integration, no direct calls. Ralph не берёт на себя ответственность за Serena lifecycle. Best-effort detection = правильный подход для optional dependency.

4. **Stage 2 injection для user content.** `__LEARNINGS_CONTENT__` и `__RALPH_KNOWLEDGE__` через strings.Replace в Stage 2 — безопасно от template injection. User content с `{{` не ломает assembly. Паттерн уже proven в Epics 1-5.

5. **No forced truncation.** Решение "300+ lines = 3-4% context, linear decay, user decides" — правильное для CLI tool с single developer. Injection circuit breaker убран в пользу human gate — это honest UX: пользователь видит проблему и решает сам.

6. **2-generation backups.** .bak + .bak.1 — минимальная, но достаточная защита от model collapse и crash recovery. Не overengineered.

7. **Cooldown через MonotonicTaskCounter.** Persisted, never resets, cross-session compatible. Решает проблему ephemeral counter (H1 из консолидированного ревью). Архитектурно чисто.

8. **FR26 satisfied vacuously.** Ralph НЕ модифицирует CLAUDE.md проекта. Zero corruption risk. Это правильное архитектурное решение — не пытаться быть умнее, чем нужно.

---

## Сводка

| Severity | Count | IDs |
|----------|-------|-----|
| CRITICAL | 3 | P2-C1, P2-C2, P2-C3 |
| HIGH | 5 | P2-H1, P2-H2, P2-H3, P2-H4, P2-H5 |
| MEDIUM | 8 | P2-M1 -- P2-M8 |

**Главный архитектурный риск:** V4 отклонил pending-file pattern в пользу snapshot-diff, но не обосновал это решение и не устранил проблемы, которые pending-file решал (crash safety, diff parsing fragility, partial writes). Одновременно сохранил `[needs-formatting]` tag, который все 5 пар рекомендовали удалить, и который противоречит заявленному принципу "Go не пишет контент".

**Вторичный риск:** runner/knowledge.go рискует стать 800+ строк God file с 10+ responsibilities. Нужна разбивка до начала реализации.

**Позитив:** Dependency direction не нарушена. Injectable pattern сохранён. MCP и Stage 2 injection — архитектурно верны. MonotonicTaskCounter — правильное решение.
