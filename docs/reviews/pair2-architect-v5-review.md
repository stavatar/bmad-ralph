# Пара 2: Архитектор — Ревью Distillation Subsystem (Epic 6 v5)

**Дата:** 2026-03-02
**Роль:** Архитектор Пары 2, adversarial review
**Scope:** Stories 6.5a/b/c, 6.6 — DistillState, knowledge.go split, 2-generation backups, crash recovery, multi-file atomicity, trend tracking
**Метод:** Анализ кода + Deep Research (state management, crash recovery, WAL, atomic file ops on WSL/NTFS)

---

## 1. Архитектурные проблемы

### [P2A-C1] CRITICAL: Multi-file distillation write НЕ АТОМАРНА — нет intent/checkpoint механизма

**Severity:** CRITICAL
**Затрагивает:** Story 6.5b (multi-file write), Story 6.5c (crash recovery)

**Проблема:** Distillation записывает N+2 файла: LEARNINGS.md + N ralph-{category}.md + ralph-index.md + distill-state.json. Все записи последовательные (`os.WriteFile`). Crash между файлом 3 и файлом 5 оставляет систему в неконсистентном состоянии:

```
LEARNINGS.md     — уже перезаписан (compressed)
ralph-testing.md — уже обновлён
ralph-errors.md  — уже обновлён
ralph-config.md  — НЕ записан (old content или отсутствует)
ralph-index.md   — НЕ записан (stale TOC)
distill-state.json — НЕ обновлён (LastDistillTask = old value)
```

**Текущая защита:** `.bak` files + startup recovery. НО: recovery восстанавливает ВСЕ файлы из бэкапов — а если crash произошёл после LEARNINGS.md но до записи .bak? Спецификация не определяет порядок: backup → write или write → backup.

**Deep Research подтверждает:**
- `rename()` атомарна per-file, но группа rename — нет (Dan Luu: "Files are hard")
- На WSL/NTFS rename может не быть атомарной даже per-file (WSL#5087)
- Паттерн решения: **intent file** (checkpoint.json) со списком pending операций

**Рекомендация:** Добавить intent file `.ralph/distill-intent.json`:
```
Фаза 1: Backup всех файлов
Фаза 2: Записать новые файлы как .pending
Фаза 3: Записать intent.json: {files: [{pending: X, target: Y}, ...]}
Фаза 4: Rename .pending → target (серия rename)
Фаза 5: Обновить distill-state.json
Фаза 6: Удалить intent.json

Recovery: intent.json существует → завершить rename серию или откатить из .bak
```

**Effort:** MEDIUM (30-50 строк Go). **Impact на AC:** Добавить AC в Story 6.5c.

---

### [P2A-H1] HIGH: Backup rotation порядок операций не специфицирован — copy vs rename

**Severity:** HIGH
**Затрагивает:** Story 6.5b (backup), Story 6.5c (crash recovery), Story 6.6 (manual distill)

**Проблема:** V5 описывает "2-generation rotation: .bak + .bak.1". Но не специфицирован порядок:

Вариант A (ОПАСНЫЙ — rename):
```
1. rename(file.bak → file.bak.1)     # OK
2. rename(file → file.bak)            # ОПАСНО: file НЕ существует после этого!
3. writeFile(file, newData)            # Crash ТУТ → нет file, нет file.bak
```

Вариант B (БЕЗОПАСНЫЙ — copy):
```
1. rename(file.bak → file.bak.1)     # OK: потеря .bak.1 приемлема
2. copyFile(file → file.bak)          # БЕЗОПАСНО: file всё ещё существует
3. atomicWrite(file, newData)          # Crash → file = old data, file.bak = old data
```

**Deep Research подтверждает:** Шаг 2 ДОЛЖЕН быть copy, не rename. Иначе crash между 2 и 3 оставляет систему без текущего файла и без бэкапа.

**Рекомендация:** Явно специфицировать в AC Story 6.5b:
- Шаг 2 = `io.Copy` (не `os.Rename`)
- `atomicWriteFile()` helper: write-to-temp → fsync → rename
- Temp файл создаётся в том же каталоге (критично для WSL/NTFS — `/tmp` может быть tmpfs)

**Effort:** LOW. **Impact на AC:** Уточнение в Technical Notes Story 6.5b.

---

### [P2A-H2] HIGH: distill-state.json запись через os.WriteFile — partial write при crash

**Severity:** HIGH
**Затрагивает:** Story 6.5c (DistillState persistence)

**Проблема:** DistillState хранит MonotonicTaskCounter, LastDistillTask, Categories, Metrics. Это ЕДИНСТВЕННЫЙ persistent state для cooldown и circuit breaker. Если `os.WriteFile` прерван (kill -9, OOM, power loss), файл может быть:
- Пустой (0 байт): truncate прошёл, write — нет
- Усечённый JSON: `{"version":1,"monotonic_task_counter":15,"last_`
- Мусор: старые данные + частично новые (NTFS)

**Текущая защита в V5:** "Parse error → default CLOSED (fail-open)." Это значит: corrupted state → all cooldowns/CB reset → distillation может запуститься повторно. Не фатально, но теряет метрики и нарушает cooldown.

**Deep Research подтверждает:**
- Claude Code issue #28829: реальный баг corruption .claude.json при concurrent sessions
- На WSL fsync не гарантирует durability (WSL#3556)

**Рекомендация:**
1. Использовать `atomicWriteFile()` для distill-state.json (write-temp-rename)
2. Валидация при чтении: `json.Unmarshal` + проверка `Version > 0` + fallback на .bak
3. Temp file в `.ralph/` (тот же mount point)

```go
func (s *DistillState) Save(path string) error {
    data, _ := json.MarshalIndent(s, "", "  ")
    return atomicWriteFile(path, data, 0644)
}

func LoadDistillState(path string) (*DistillState, error) {
    // Каскад: primary → .bak → .bak.1 → default
    for _, p := range []string{path, path+".bak", path+".bak.1"} {
        if s, err := tryLoadState(p); err == nil {
            return s, nil
        }
    }
    return &DistillState{Version: 1}, nil // default: CLOSED, counters = 0
}
```

**Effort:** LOW (10-15 строк). **Impact на AC:** Добавить в Story 6.5c AC: "State written via atomic write-temp-rename pattern."

---

### [P2A-H3] HIGH: knowledge.go split на 4 файла — зависимости между файлами не описаны

**Severity:** HIGH
**Затрагивает:** Story 6.1, 6.2, 6.5a/b/c

**Проблема:** V5 решение [v5-9]: split knowledge.go на:
- `knowledge_write.go` — FileKnowledgeWriter, post-validation, gates
- `knowledge_read.go` — buildKnowledgeReplacements, ValidateLearnings
- `knowledge_distill.go` — AutoDistill, prompt, output parsing, multi-file write
- `knowledge_state.go` — DistillState, serialization, crash recovery

Декомпозиция логична по SRP. **НО:** зависимости между файлами не описаны:

```
knowledge_distill.go
  ├── calls: knowledge_state.go (LoadDistillState, SaveDistillState)
  ├── calls: knowledge_read.go (buildKnowledgeReplacements для prompt)
  ├── calls: knowledge_write.go (ValidateDistillation)
  └── calls: session.Execute (→ session package)

knowledge_write.go
  ├── calls: knowledge_read.go (BudgetCheck)
  └── calls: knowledge_state.go (MonotonicTaskCounter++)
```

Все 4 файла в одном пакете `runner/` — нет circular dependency risk. Но:
1. **knowledge_distill.go зависит от всех остальных 3** — это потенциальный "God file v2"
2. `AutoDistill` в knowledge_distill.go вызывает `session.Execute` — это runner → session зависимость, уже существующая. OK.
3. Метрики (Story 6.9) обновляются в runner.go (после review), но хранятся в knowledge_state.go (DistillState). Кто координирует запись?

**Рекомендация:** Добавить в Technical Notes:
- Dependency diagram между 4 файлами
- Единая точка записи DistillState: `knowledge_state.go:SaveDistillState()`. Все остальные файлы читают, но НЕ пишут напрямую.
- knowledge_distill.go не должен прямо вызывать ValidateDistillation — передавать как параметр (injectable, как ReviewFn)

---

### [P2A-H4] HIGH: Отсутствует file lock для защиты от concurrent ralph run + ralph distill

**Severity:** HIGH
**Затрагивает:** Story 6.5b, 6.6

**Проблема:** V5 принял решение L6: "Advisory note only. File lock → Growth." Но:
- `ralph run` может быть в середине execute, когда LEARNINGS.md модифицируется
- `ralph distill` читает LEARNINGS.md, записывает compressed версию
- Одновременный запуск: ralph run записывает ЧЕРЕЗ Claude → LEARNINGS.md, ralph distill читает → компрессирует → перезаписывает. Результат: потеря записей Claude.

**Deep Research подтверждает:**
- `gofrs/flock`: кросс-платформенный file lock, TryLock() non-blocking
- На WSL/NTFS flock работает через Windows LockFileEx

**Рекомендация:** Добавить advisory lock НА distill-state.json (единственный shared state):

```go
lock := flock.New(filepath.Join(ralphDir, ".lock"))
locked, err := lock.TryLock()
if !locked {
    return fmt.Errorf("another ralph instance is running (remove .ralph/.lock if stale)")
}
defer lock.Unlock()
```

Это не блокер для v1, но стоит добавить как отдельный AC в Story 6.6: "Advisory lock file .ralph/.lock checked at startup. If locked, warn and exit."

**Effort:** LOW (5 строк + 1 dep: gofrs/flock). Но **4 deps = limit** — нужно обоснование.

---

### [P2A-M1] MEDIUM: DistillState God Object — смешение 4 доменов в одном JSON

**Severity:** MEDIUM
**Затрагивает:** Story 6.5c, 6.9

**Проблема:** DistillState содержит:
1. Version, MonotonicTaskCounter — task tracking domain
2. LastDistillTask, ConsecutiveFailures, Categories — distillation domain
3. Metrics (FindingsPerTask, FirstCleanReviewRate, TotalTasks...) — analytics domain
4. TransitionCount, LastTransitionTime — circuit breaker domain (из J7)

К Story 6.9 это ~15+ полей. JSON без схемы, один файл, одна структура.

**Рекомендация (B из P2-H3 v4):** Вложенные structs:

```go
type DistillState struct {
    Version    int          `json:"version"`
    Tasks      TaskState    `json:"tasks"`
    Distill    DistillData  `json:"distill"`
    Metrics    MetricsData  `json:"metrics"`
}
```

Один файл, но структурированный. Миграция: при `Version == 0` читать flat → маппить в nested.

**Effort:** LOW. **Impact на AC:** Уточнить JSON schema в Story 6.5c.

---

### [P2A-M2] MEDIUM: buildKnowledgeReplacements вызывается 3 раза за iteration — нет кеширования

**Severity:** MEDIUM
**Затрагивает:** Story 6.2

**Проблема:** Из P2-H5 v4 (подтверждаю). 3 call sites (initial execute, retry execute, review) — каждый вызывает buildKnowledgeReplacements → N os.Stat + M os.ReadFile. На WSL/NTFS: ~60 filesystem ops × 5ms = 300ms × 3 = 900ms overhead per iteration.

**Рекомендация:** Cache результат per-iteration. Invalidate при:
- post-validation (новые entries в LEARNINGS.md)
- distillation (ralph-*.md обновлены)

```go
type knowledgeCache struct {
    replacements map[string]string
    valid        bool
}
```

Runner вызывает `buildKnowledgeReplacements` один раз, передаёт результат в RunConfig.

**Effort:** LOW. **Impact на AC:** Добавить в Story 6.2 Technical Notes.

---

### [P2A-M3] MEDIUM: Crash recovery при startup проверяет .bak — но не проверяет intent file

**Severity:** MEDIUM
**Затрагивает:** Story 6.5c

**Проблема:** V5 crash recovery: "finds .bak files → restore → log warning." Но:
1. Как определить "прерванная дистилляция"? Наличие .bak файлов? Но .bak файлы ВСЕГДА существуют после успешной дистилляции (не удаляются). Нужен маркер "в процессе".
2. Если бэкапы не удаляются (они не удаляются — L4: "backups preserved until next distill run"), то наличие .bak файлов ≠ прерванная дистилляция.

**Рекомендация:** Intent file решает эту проблему (см. P2A-C1): `.ralph/distill-intent.json` существует ТОЛЬКО между началом и завершением дистилляции. При startup: intent file → recovery needed. Нет intent file → .bak файлы = normal (от предыдущего успешного run).

**Effort:** Включено в P2A-C1.

---

### [P2A-M4] MEDIUM: distill_gate: auto — CB auto-skip, но ConsecutiveFailures не персистится явно

**Severity:** MEDIUM
**Затрагивает:** Story 6.5a

**Проблема:** Auto mode: CB auto-skip после 3 consecutive failures. ConsecutiveFailures хранится в DistillState. При restart: `ConsecutiveFailures` восстанавливается из JSON. OK.

Но: если crash между failure и записью DistillState → ConsecutiveFailures не инкрементировался → после recovery failure "забыта" → CB никогда не откроется при pattern "fail → crash → restart → fail → crash → restart → fail → crash...".

**Рекомендация:** Записывать DistillState ПЕРЕД (не после) каждой попыткой дистилляции с `ConsecutiveFailures++`. Если попытка успешна — обновить с `ConsecutiveFailures = 0`. Если crash — при recovery ConsecutiveFailures уже инкрементирован.

**Effort:** LOW (порядок вызовов).

---

### [P2A-L1] LOW: Trend tracking (6.9) — rolling window не нужен, но cumulative counters overflow-proof?

**Severity:** LOW
**Затрагивает:** Story 6.9

**Проблема:** `TotalTasks int, TotalFindings int, CleanFirstReviews int` — кумулятивные. При 100 задачах в день × 365 дней = 36,500. int64 overflow не проблема. Но:
- `FindingsPerTask float64` = TotalFindings / TotalTasks → деление на 0 при TotalTasks == 0
- Десятки тысяч задач: ранние задачи (без knowledge) dominate average → метрика нечувствительна

**Рекомендация:** Guard `TotalTasks == 0` → return 0.0. Для v1 достаточно. Exponential moving average — Growth phase.

---

## 2. Альтернативные подходы — с диаграммами

### Подход A: Текущий (V5) — Backup-Restore

```
┌──────────────────────────────────────────┐
│ DISTILLATION WRITE                        │
│                                          │
│ 1. Backup ALL files (.bak rotation)       │
│ 2. Write LEARNINGS.md (compressed)        │
│ 3. Write ralph-testing.md                 │
│ 4. Write ralph-errors.md                  │
│ ...                                      │
│ N. Write ralph-index.md                   │
│ N+1. Update distill-state.json            │
│                                          │
│ CRASH at step 4 → inconsistent state!     │
│ Recovery: restore ALL from .bak           │
│ Problem: HOW to detect crash happened?    │
└──────────────────────────────────────────┘
```

### Подход B: Intent File (РЕКОМЕНДУЕМЫЙ)

```
┌──────────────────────────────────────────┐
│ DISTILLATION WRITE (with intent)          │
│                                          │
│ Phase 1: PREPARE                          │
│   1. Backup ALL files (.bak rotation)     │
│   2. Write LEARNINGS.md.pending           │
│   3. Write ralph-testing.md.pending       │
│   4. Write ralph-errors.md.pending        │
│   ...                                    │
│   N. Write ralph-index.md.pending         │
│   N+1. Write distill-intent.json:         │
│         {files: [{pending→target}, ...]}  │
│                                          │
│ Phase 2: COMMIT                           │
│   1. rename(.pending → target) × N        │
│   2. Update distill-state.json            │
│   3. Delete distill-intent.json           │
│                                          │
│ CRASH at Phase 1 step 4:                  │
│   No intent.json → orphan .pending files  │
│   Recovery: delete .pending files → clean │
│                                          │
│ CRASH at Phase 2 step 1 (mid-rename):     │
│   intent.json EXISTS → resume Phase 2     │
│   For each entry: if .pending exists,     │
│   rename to target                        │
│                                          │
│ Result: ALWAYS consistent                 │
└──────────────────────────────────────────┘
```

### Подход C: Single Consolidated Output File

```
┌──────────────────────────────────────────┐
│ ALTERNATIVE: write ralph-distilled.json   │
│                                          │
│ 1 файл = 1 атомарная запись:              │
│ {                                        │
│   "learnings": "compressed content...",   │
│   "categories": {                        │
│     "testing": {                          │
│       "globs": ["*_test.go"],            │
│       "rules": "..."                     │
│     },                                   │
│     "errors": {...}                      │
│   },                                    │
│   "index": "..."                         │
│ }                                        │
│                                          │
│ Then: Go splits into ralph-*.md files     │
│ on READ (lazy materialization)            │
│                                          │
│ Pro: 1 atomic write, no inconsistency     │
│ Con: ralph-*.md не auto-loaded by Claude  │
│      Code → need materialization step     │
│ Verdict: REJECTED — Claude Code needs     │
│ actual .md files in .claude/rules/        │
└──────────────────────────────────────────┘
```

Подход C отвергнут: Claude Code auto-loads `.claude/rules/*.md` по glob-match — нужны реальные файлы. Подход B (Intent File) — рекомендуемый.

---

## 3. Research Findings — лучшие практики

### 3.1 Atomic File Operations на WSL/NTFS

| Аспект | Гарантия |
|--------|----------|
| `os.Rename` (same mount) | Атомарен при normal operation, НЕ при crash на WSL |
| `fsync` на WSL | Не гарантирует durability (WSL#3556) |
| `os.WriteFile` при crash | Может оставить 0-byte или truncated файл |
| Temp file в `/tmp` → rename на `/mnt/c/` | EXDEV error — разные mount points! |

**Источники:** Dan Luu "Files are hard", google/renameio docs, Michael Stapelberg "Atomically writing files in Go", WSL issues #3556, #5087.

### 3.2 WAL vs Backup-Rotate для CLI

WAL **избыточен** для bmad-ralph:
- Состояние невелико (~1KB JSON + ~10KB markdown)
- Обновления редкие (раз в 5+ задач)
- Потеря последнего distillation приемлема (повторить)
- Нет конкурентности (single goroutine)

Backup-rotate + intent file **достаточен**.

### 3.3 Circuit Breaker State Persistence

Go библиотеки (sony/gobreaker, eapache/go-resiliency) хранят CB state **только в памяти**. Файловая персистентность — custom implementation. bmad-ralph правильно использует task-count-based CB (не time-based) — подходит для CLI.

**Ключевое:** CB state записывать ПЕРЕД попыткой (optimistic failure counting), обнулять при success. Не наоборот.

### 3.4 Claude Code .claude.json corruption (Issue #28829)

Реальный production баг: concurrent sessions corrupt JSON state. Релевантно для `ralph run` + `ralph distill` race condition. Advisory lock — минимально необходимая защита.

---

## 4. Подтверждено — архитектурно правильные решения

### 4.1 DistillState в .ralph/distill-state.json
Решение [v5-8] — **правильное**. SRP соблюдён: state отдельно от данных. `.ralph/` — каноничное место для runtime state.

### 4.2 knowledge.go split на 4 файла
Решение [v5-9] — **правильное**. SRP по доменам: write/read/distill/state. Все в runner/ — нет circular dependency. Не нужен новый пакет.

### 4.3 2-generation backups
Решение L4 — **достаточное** при условии:
- Валидация при каждом чтении (catch corruption < 2 циклов)
- Copy (не rename) для создания .bak из current
- Intent file для detection of interrupted distillation

### 4.4 Trend tracking вместо A/B
Решение [v5-7] — **правильное**. A/B на 5-15 задачах = статистически бессмысленно. Trend = достаточный сигнал. Кумулятивные счётчики — адекватно для v1.

### 4.5 MonotonicTaskCounter
Решение H1 — **правильное**. Persisted, never resets, cross-session. Cooldown через разность — элегантно.

### 4.6 Config-driven distill_gate (human|auto)
Решение [v5-2] — **правильное**. Оба режима нужны. Human default — безопасно для v1.

### 4.7 Version field в DistillState
Решение [v5-8] — **правильное**. Forward compatibility. Missing Version → Version 0 → migrate.

### 4.8 Injectable DistillFn
Следует паттерну ReviewFn/GatePromptFn. Тестируемость сохранена.

---

## 5. Сводка

| ID | Проблема | Severity | Effort | Story |
|----|----------|----------|--------|-------|
| P2A-C1 | Multi-file write не атомарна — нет intent file | CRITICAL | MEDIUM | 6.5b/c |
| P2A-H1 | Backup rotation: copy vs rename не специфицирован | HIGH | LOW | 6.5b |
| P2A-H2 | distill-state.json: partial write при crash | HIGH | LOW | 6.5c |
| P2A-H3 | knowledge.go 4-file dependencies не описаны | HIGH | LOW | 6.5a/b/c |
| P2A-H4 | Нет file lock для concurrent run+distill | HIGH | LOW | 6.6 |
| P2A-M1 | DistillState God Object (4 домена) | MEDIUM | LOW | 6.5c/6.9 |
| P2A-M2 | buildKnowledgeReplacements 3x per iteration | MEDIUM | LOW | 6.2 |
| P2A-M3 | Crash recovery не отличает interrupted от normal | MEDIUM | LOW | 6.5c |
| P2A-M4 | ConsecutiveFailures записывается ПОСЛЕ попытки | MEDIUM | LOW | 6.5a |
| P2A-L1 | Trend tracking division by zero | LOW | LOW | 6.9 |

**Total: 1 CRITICAL, 4 HIGH, 4 MEDIUM, 1 LOW = 10 находок**

**Главный архитектурный риск:** Multi-file distillation write без intent/checkpoint механизма. Crash между файлами → неконсистентное состояние, не обнаруживаемое текущей recovery логикой.

**Вторичные риски:** Copy vs rename в backup rotation, partial write distill-state.json, concurrent access без lock.

**Позитив:** Основная архитектура (3-layer, injectable, MonotonicTaskCounter, 2-gen backups, trend tracking, knowledge.go split) — корректна и well-engineered.
