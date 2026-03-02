# Пара 1: Ревью алгоритма и pipeline -- Epic 6 v4

**Дата:** 2026-03-02
**Фокус:** Полный pipeline знаний, snapshot-diff модель, дистилляция, freq:N, ANCHOR, backup/recovery
**Ревьюер:** Аналитик + Архитектор (Пара 1), Claude Opus 4.6
**Источники:** Epic 6 v4 (1108 строк, 78 AC, 9 stories), 7 research documents, source code (runner/, config/, session/)

---

## Критические проблемы (блокеры)

### [P1-C1] Snapshot-diff модель: Claude может перезаписать весь файл, убив snapshot-diff

- **Где:** Story 6.1 AC "Snapshot-diff post-validation model", Story 6.3, Story 6.4
- **Описание:** Модель v4 предполагает: Go делает snapshot LEARNINGS.md перед сессией, Claude **appends** записи, Go делает diff после сессии. Diff = новые записи. Но Claude через file tools может сделать **полный rewrite** файла (не append). Claude Code `Write` tool перезаписывает файл целиком. Сценарии:
  1. Claude решает "улучшить форматирование" существующих записей -- rewrite, snapshot-diff показывает ВСЕ записи как "новые"
  2. Claude удаляет "неактуальные" записи (AI hallucination о релевантности) -- diff неправильно интерпретирует удаления
  3. Claude переупорядочивает записи -- diff ломается (все записи кажутся изменёнными)
  4. Два append-а в одной сессии с промежуточной перестройкой -- diff не может разделить
- **Почему плохо:** Diff-based подход фундаментально хрупок когда "writer" (Claude) имеет full file access. Нет гарантии append-only поведения. Prompt instructions = best-effort, не enforcement. Все 6 quality gates становятся ненадёжными т.к. работают на неверном diff.
- **Severity:** CRITICAL -- quality gates (G1-G6) = основная ценность Story 6.1. Если diff ненадёжен, gates бесполезны.
- **Опции исправления:**
  A) **Checksum guard:** Перед diff проверить что snapshot-prefix сохранён в начале текущего файла. Если prefix не совпадает -- весь файл "tainted", log warning, пометить ВСЕ новые записи `[needs-formatting]`. Простое, не решает проблему полностью, но детектирует.
  B) **Line-based append detection:** Snapshot = N строк. После сессии: если первые N строк == snapshot, то diff = строки N+1..end. Если не совпадают -- detect rewrite, fallback к полной ревалидации всего файла (все записи проходят gates заново).
  C) **Вернуть pending-file (рекомендовано review v3):** Claude пишет в `.ralph/pending-lessons.md`, Go обрабатывает. Это гарантирует единый write path. V4 отверг pending-file, но проблемы остались.
  D) **Content hash sections:** Каждая существующая запись получает inline hash (скрытый HTML comment `<!-- sha:abc123 -->`). При diff Go сравнивает hashes, детектирует модификации vs новые записи.

### [P1-C2] [needs-formatting] остался в v4 несмотря на решение C4 из consolidated review

- **Где:** Story 6.1 AC "invalid entries tagged with [needs-formatting] IN the file", Story 6.5 criterion #7
- **Описание:** Consolidated review (28 решений) содержал C4: "[needs-formatting] tag -- мёртвая ветка". Рекомендация: "Убрать [needs-formatting] из формата, AC, и ValidateDistillation." V4 redesign context упоминает решение C4, но... `[needs-formatting]` tag **полностью сохранён** в v4:
  - Story 6.1: "invalid entries tagged with [needs-formatting] IN the file"
  - Story 6.5: criterion #7 "All [needs-formatting] entries either fixed or preserved"
  - Story 6.8: "FINAL -- [needs-formatting] tag and fix cycle"
  - Architecture summary: "[needs-formatting] tag preserves knowledge (fix at distillation, no loss)"
- **Почему плохо:**
  1. **Context pollution:** [needs-formatting] entries = бесполезный контекст (плохо отформатированные записи) который Claude видит при injection. Wasted tokens.
  2. **Бесконечное накопление:** Если дистилляция не запускается (файл < 150 строк), [needs-formatting] entries остаются навечно.
  3. **Логическое противоречие с P1-C1:** Если diff ненадёжен (P1-C1), Go не может точно определить какие entries "новые" -- тег может быть добавлен к записям которые Claude просто переформатировал.
  4. **Усложнение ValidateDistillation:** Criterion #7 добавляет ещё одну dimension проверки.
- **Опции исправления:**
  A) **Reject + log (рекомендовано v3 review):** Невалидные entries не сохраняются. Они есть в session output и git history. Zero pollution.
  B) **Quarantine file:** Невалидные entries → `.ralph/quarantine-lessons.md` (не инжектируется в промпты, но доступен при distillation).
  C) **Оставить, но НЕ инжектировать:** При injection (Story 6.2) фильтровать entries с `[needs-formatting]` tag. Pollution на диске, но не в context window.

### [P1-C3] "Last 20% of entries injected" (H5) + "reverse read" (L3) = двойная путаница семантики

- **Где:** Story 6.2 AC "only last 20% of entries injected", Story 6.2 AC "content reversed"
- **Описание:** Два conflicting механизма:
  1. **H5:** "only last 20% of entries injected" -- значит при 100 entries инжектируется 20
  2. **L3:** "content reversed: split by \n##, reverse section order, rejoin" -- инвертирует ВСЕ записи
  Вопрос: reverse применяется ДО или ПОСЛЕ фильтрации 20%? Если после -- результат корректен (20 newest, показаны newest-first). Если до -- берётся 20% от reversed файла = 20% OLDEST entries (append-only, tail=newest, reverse=oldest first, 20% of that = oldest).
  Но есть фундаментальная проблема: **зачем инжектировать только 20%?** При 100 entries это 20 entries ~ 60-80 строк. При soft threshold 150 это 30 entries ~ 90-120 строк. Knowledge injection теряет 80% знаний. Research (SFEIR S25): ~15 rules = 94% compliance. 20% от 50 entries = 10 entries -- ниже порога эффективности.
- **Почему плохо:** Потеря 80% знаний при injection делает весь extraction pipeline бессмысленным. Если знания не инжектируются -- зачем их извлекать? H5 в consolidated review решал проблему "last 3 sessions" для distillation validation, а не для injection. V4 неправильно применил H5 к injection.
- **Опции исправления:**
  A) **Inject ALL, reverse for recency:** Инжектировать ВСЕ entries (reversed для recency). 150 строк = ~3000 tokens = 3% context. Это допустимо (v4 сам утверждает "300+ строк = 3-4% context").
  B) **Inject all до injection CB threshold:** Инжектировать все до 600 строк (3x budget). При превышении -- injection CB (empty). Между 200-600 -- all injected с warning.
  C) **20% only at distillation validation (H5 original intent):** Вернуть 20% к ValidateDistillation criterion #3, injection = full file.

---

## Серьёзные проблемы

### [P1-H1] Semantic dedup через prefix match не обрабатывает нормализацию дефисов (решение v3 не применено)

- **Где:** Story 6.1 AC "Semantic dedup merges similar entries"
- **Описание:** V3 extraction pipeline review (Issue 2) рекомендовал: "Normalize: ToLower, TrimSpace, replace all `-_` -> space, collapse multiple spaces". V4 AC сохраняет только "strings.ToLower + strings.TrimSpace normalization". Нормализация дефисов/подчёркиваний **потеряна**.
  - `assertion-quality` vs `assertion quality` = не совпадут
  - `test_patterns` vs `test patterns` = не совпадут
  - Это = 40-60% пропущенных дубликатов по research (NVIDIA SemDedup)
- **Почему плохо:** Дубликаты расходуют budget. При 200-line budget каждый дубликат = 2-4 строки потерянных.
- **Опции исправления:**
  A) **Добавить в AC:** "Normalization includes: ToLower, TrimSpace, replace `-_` with space, collapse multiple spaces to single space."
  B) **Levenshtein distance <= 2** в дополнение к prefix match. Ловит опечатки и вариации.

### [P1-H2] freq:N при первой дистилляции -- откуда берётся N?

- **Где:** Story 6.5 AC "assign freq:N to entries"
- **Описание:** При первой дистилляции LEARNINGS.md не содержит freq:N маркеров. Distillation prompt инструктирует "assign freq:N to entries". Как Claude определяет N для entries которые видит впервые? Варианты:
  1. Claude считает количество entries с одинаковым topic -- freq = count. Но topics не дублируются (G3 dedup filter).
  2. Claude ставит freq:1 всем -- бесполезно, все entries равны.
  3. Claude "угадывает" по важности -- недетерминистично, нет ground truth.
  Результат: при первой дистилляции freq:N = произвольные числа от Claude. Go проверяет монотонность, но на первой дистилляции нет "old freq" для сравнения -- все проходят.
- **Почему плохо:** Freq:N становится ненадёжным сигналом с самого начала. T1 promotion (freq>=10) будет зависеть от произвольных чисел первой дистилляции.
- **Опции исправления:**
  A) **Go инициализирует freq:1** для всех новых entries при первой дистилляции. Distillation prompt инструктируется только increment (merge = sum freqs). Go проверяет: new_freq >= old_freq.
  B) **Dual counter:** Go считает сколько раз entry появлялся в LEARNINGS.md (через post-validation dedup counter). Это deterministic ground truth. LLM freq:N = advisory, Go freq = authoritative.
  C) **Prompt specification:** "Entries without existing [freq:N] marker get freq:1. Merged entries: freq = sum of individual freqs."

### [P1-H3] Cooldown монотонный счётчик (H1) -- race condition при crash во время increment

- **Где:** Story 6.5 AC "MonotonicTaskCounter in DistillState"
- **Описание:** MonotonicTaskCounter инкрементируется при каждом clean review и persisted в `LEARNINGS.md.state`. Если crash между clean review и persist:
  1. Review чистый -> Go решает increment counter
  2. Go записывает LEARNINGS.md.state с counter+1
  3. **Crash** перед продолжением loop
  4. Restart: counter уже counter+1, но task не считается завершённым в sprint-tasks.md
  Или обратный сценарий: task отмечен [x], но counter не сохранён.
  Несогласованность counter и фактического прогресса не фатальна (cooldown off by 1), но accumulates.
- **Почему плохо:** Мелкая проблема, но cooldown = 5 tasks. Ошибка на +-1 при каждом crash = дрифт.
- **Опции исправления:**
  A) **Tolerate:** Counter -- best-effort. Cooldown +-1 task допустим. Document.
  B) **Derive from sprint-tasks.md:** Считать [x] entries вместо отдельного counter. Но sprint-tasks.md обновляется Claude, не Go.

### [P1-H4] ValidateDistillation criterion #3 "Last 20% entries preserved" -- не работает после multi-round distillation

- **Где:** Story 6.5 AC "Post-validation rejects bad distillation", criterion #3
- **Описание:** Criterion #3: "Last 20% entries preserved". Это проверяет что distillation output содержит записи из конца input файла. Но после второй дистилляции:
  - Первая дистилляция: 150 entries -> 80 entries. Last 20% (30 entries) preserved.
  - Второй цикл: file grows to 160. Entries 81-160 = new. Entries 1-80 = distilled.
  - Вторая дистилляция: "last 20%" = entries 129-160 (32 entries). Но entries 1-80 уже были distilled раз -- они compressed, merged. Может не содержать original text.
  - Criterion #3 сравнивает text match? Если да -- compressed entries из первой дистилляции не совпадут с оригиналами.
- **Почему плохо:** Criterion #3 будет false-reject после второй дистилляции если проверяет exact text match. Или будет бесполезен если проверяет только "entry count >= 20% of input".
- **Опции исправления:**
  A) **Header-based matching:** Проверять сохранность `## category: topic` заголовков last 20%, не full text. Headers stable across distillations.
  B) **Skip criterion #3 after first distillation:** Если DistillState shows previous distillation happened, relax #3 to "at least 50% of entries from input's tail quarter preserved."
  C) **Remove criterion #3:** Other criteria (citation >=80%, category >=80%, no duplicates) уже покрывают quality. #3 = redundant.

### [P1-H5] Human GATE на distillation failure (C2 v4) -- блокирует автономный режим

- **Где:** Story 6.5 AC "Human GATE on distillation failure"
- **Описание:** V4 заменил автоматический circuit breaker на human gate при КАЖДОМ failure. Ralph -- инструмент для **автономной** разработки (Ralph Loop). Human gate блокирует loop до ответа пользователя. Сценарии:
  1. Ночной run: 20 задач, distillation fails на задаче 5 -- loop стоит до утра (15 задач не выполнены)
  2. CI environment: нет stdin -- gate ждёт бесконечно
  3. Пользователь в DND: gate ждёт часы
- **Почему плохо:** Distillation -- оптимизация, не core flow. Блокирование core loop из-за оптимизации = нарушение приоритетов. V3 circuit breaker (auto-skip после 3 failures) решал это корректно.
- **Опции исправления:**
  A) **Timeout на gate:** Если ответ не получен за N минут (configurable, default 5) -- auto-skip. Записать в лог.
  B) **Hybrid:** Первый failure = human gate. Второй failure подряд = auto-skip. Третий+ = auto-skip + disable distillation до `ralph distill`.
  C) **Config flag:** `distill_gate: human|auto|off`. Default `auto` (circuit breaker как в v3). `human` для interactive. `off` для CI.

### [P1-H6] Backup .bak + .bak.1 -- нет атомарности при multi-file backup

- **Где:** Story 6.5 AC "backup created: LEARNINGS.md.bak + .bak.1"
- **Описание:** Distillation создаёт backup LEARNINGS.md + ALL ralph-*.md. Но backup -- последовательный: rename .bak -> .bak.1, copy -> .bak, для каждого файла. При crash посреди backup:
  1. LEARNINGS.md.bak -> .bak.1 (done)
  2. LEARNINGS.md -> .bak (done)
  3. ralph-testing.md.bak -> .bak.1 (**crash**)
  Состояние: LEARNINGS.md.bak = current (correct), LEARNINGS.md.bak.1 = previous (correct), но ralph-testing.md.bak = **previous** (not rotated). Восстановление startup (M7) найдёт .bak файлы, но .bak и .bak.1 для ralph-testing.md = из разных поколений.
- **Почему плохо:** Multi-file backup без транзакционности = inconsistent restore. При 8 ralph-*.md файлах вероятность crash посередине растёт.
- **Опции исправления:**
  A) **Generation marker file:** Before backup, write `distill-backup-gen-N`. After ALL backups complete, write `distill-backup-complete-N`. Startup checks: if gen exists but complete doesn't -- incomplete backup, restore only files with matching gen timestamp.
  B) **Single backup directory:** `cp -r .claude/rules/ .claude/rules.bak/` + `cp LEARNINGS.md LEARNINGS.md.bak`. Atomic per-directory snapshot. Restore = `mv .claude/rules.bak/ .claude/rules/`.
  C) **Git stash:** `git stash push -- LEARNINGS.md .claude/rules/ralph-*.md` перед distillation. Restore = `git stash pop`. Используем уже имеющийся git.

---

## Замечания

### [P1-M1] Citation optional (extraction review) vs Citation required (v4 AC G2)

- **Где:** Story 6.1 AC G2 "Citation present: [source, file:line] parsed successfully"
- **Описание:** V3 extraction pipeline review (Issue 3) рекомендовал: "Citation optional, category required". V4 AC G2 говорит "Citation present" -- формулировка неоднозначна. Если citation required, universal entries (architecture patterns без привязки к файлу) будут отклонены или получат `[needs-formatting]` tag.
- **Опция:** Уточнить: "G2: Citation valid IF present (not required). Entries without citation pass G2 automatically."

### [P1-M2] Ralph-misc.md без globs (L5) -- Claude Code не загрузит автоматически

- **Где:** Story 6.5 AC "ralph-misc.md has NO globs in frontmatter"
- **Описание:** `.claude/rules/` файлы загружаются Claude Code на основе YAML frontmatter `globs:`. Без frontmatter файл **не загружается** Claude Code автоматически. L5 говорит "always loaded" но механизм -- через Stage 2 injection (`__RALPH_KNOWLEDGE__`). Это значит ralph-misc.md загружается ТОЛЬКО в ralph-managed sessions. При прямом `claude` вызове без ralph -- misc rules невидимы.
- **Опция:** Добавить `globs: ["*"]` (wide but not `**`) или документировать ограничение.

### [P1-M3] G5 cap 5 entries per validation call -- при diff с 10 новыми entries

- **Где:** Story 6.1 AC G5 "Entry cap: max 5 new entries per validation call"
- **Описание:** Если Claude написал 10 entries за сессию, post-validation найдёт 10 новых через diff. G5 cap = 5. Что с оставшимися 5? AC не описывает:
  - Вариант 1: первые 5 validated, остальные 5 tagged [needs-formatting] -- arbitrary, loses order
  - Вариант 2: все 10 tagged [needs-formatting] (cap exceeded)
  - Вариант 3: G5 не применяется к post-validation (только к WriteLessons API)
- **Опция:** Уточнить: "G5 applied per-call. If >5 new entries detected, validate first 5 (recency order), tag remaining as [needs-formatting]" или "G5 applies only to WriteLessons API, not to post-validation diff."

### [P1-M4] MonotonicTaskCounter increment vs clean review + emergency skip

- **Где:** Story 6.5 AC "MonotonicTaskCounter incremented at each clean review"
- **Описание:** Runner.Execute имеет два пути завершения задачи: clean review (completedTasks++) и emergency skip (SkipTask, bypasses counter). V4 инкрементирует MonotonicTaskCounter при clean review. Но emergency-skipped задачи тоже "consumed" -- если emergency skip = 50% задач, cooldown фактически 2x slower.
- **Опция:** Инкрементировать при ЛЮБОМ завершении задачи (clean review OR emergency skip).

### [P1-M5] ValidateDistillation criterion #4: citation preservation >=80% -- проблема optional citations

- **Где:** Story 6.5 criterion #4
- **Описание:** Если citations optional (P1-M1), многие entries не имеют citations. Citation preservation считает только entries с citations. Если 10 entries, 3 с citations: потеря 1 citation = 66%. Threshold 80% = reject. Но потерянная citation может быть от merged entry (valid operation).
- **Опция:** "Citation count >= 80% OR absolute loss <= 3 citations."

### [P1-M6] DistillState JSON -- нет версионирования формата

- **Где:** Story 6.5 Technical Notes "DistillState persisted in LEARNINGS.md.state (JSON)"
- **Описание:** DistillState JSON не имеет version field. При добавлении новых полей (Story 6.9 metrics) старые .state файлы не содержат новых полей. Go unmarshal оставит zero values. Это может быть ОК для int (0) но проблемно для slices (nil vs empty).
- **Опция:** Добавить `Version int` поле. При upgrade -- migration logic.

### [P1-M7] Scope hints auto-detection (M4) -- Go scans "top 2 levels" неточно

- **Где:** Story 6.5 AC "Go scans top 2 levels of project"
- **Описание:** "Top 2 levels" = project root + 1 level deep. Но:
  - `node_modules/`, `vendor/`, `.git/` -- должны быть excluded (тысячи файлов)
  - `.hidden` directories -- exclude?
  - Symlinks на WSL/NTFS -- follow or skip?
  AC не определяет exclusion list.
- **Опция:** Добавить в AC: "Exclude: .git, node_modules, vendor, __pycache__, .tox, dist, build, target, .venv. Skip hidden directories (`.` prefix). Skip symlinks on WSL."

### [P1-M8] Story 6.9 A/B testing -- метрики не детерминистичны

- **Где:** Story 6.9 AC "Metrics collection for A/B comparison"
- **Описание:** Метрики: repeat_violations, findings_per_task, first_clean_review_rate. Все зависят от Claude output (недетерминистичного). Comparison between modes на одном проекте -- confounded by task difficulty variance. Statistical significance при N=10-20 tasks = невозможна.
- **Опция:** Документировать как "directional signal, not statistically significant comparison". Или отложить до Growth phase.

---

## Что хорошо

1. **Замена circuit breaker на human gate (C2)** -- правильно для v1. Автоматический CB при 78 AC и 9 stories = premature optimization. Human gate проще реализовать и тестировать. НО требует timeout (P1-H5).

2. **BEGIN/END markers (H6)** -- правильный паттерн для parsing LLM output. Решает preamble/postamble проблему.

3. **Canonical categories (H2) с NEW_CATEGORY extension** -- хороший баланс между контролем и гибкостью. List grows, never shrinks -- monotonic, предсказуемый.

4. **2-generation backups (L4)** -- лучше чем single .bak. Позволяет откат на 2 поколения.

5. **MonotonicTaskCounter (H1)** -- правильная замена ephemeral counter. Cross-session persistence решает fatal bug v3.

6. **DistillFunc injectable** -- следует established pattern (ReviewFn, GatePromptFn). Тестируемость обеспечена.

7. **Serena как MCP (C3)** -- правильное решение. Minimal CodeIndexerDetector interface = YAGNI. Prompt hint = достаточно.

8. **YAML frontmatter validation (M8)** в ValidateDistillation -- критично. Invalid frontmatter = файл невидим для Claude Code.

9. **Advisory note вместо file lock (L6)** -- правильный выбор для CLI tool, single user.

10. **Story 6.9 A/B testing** -- ценная идея, но execution details нужно уточнить (P1-M8).

---

## Сводка

| Severity | Count | IDs |
|----------|-------|-----|
| CRITICAL | 3 | P1-C1, P1-C2, P1-C3 |
| HIGH | 6 | P1-H1..P1-H6 |
| MEDIUM | 8 | P1-M1..P1-M8 |
| **Total** | **17** | |

**Главный риск:** Snapshot-diff модель (P1-C1) -- фундаментальная хрупкость. Claude имеет полный write access к файлу, а Go пытается вычислить "что нового" через text diff. Это architectural mismatch. Без надёжного механизма определения "новых" записей вся post-validation pipeline = house of cards.

**Второй по важности:** "Last 20% injected" (P1-C3) -- если реализовано буквально, 80% знаний теряются при injection. Это делает весь extraction pipeline бесполезным.

**Рекомендация:** Решить P1-C1 (snapshot-diff guard) и P1-C3 (injection scope) перед реализацией. Остальные проблемы решаемы в процессе.
