# Serena Memory Sync — Architecture Shard

**Epic:** 8 — Serena Memory Sync
**PRD:** [docs/prd/serena-memory-sync.md](../prd/serena-memory-sync.md)
**Research:** [docs/research/technical-serena-memory-sync-research.md](../research/technical-serena-memory-sync-research.md)
**Status:** Draft
**Date:** 2026-03-05

---

## Обзор

Serena Memory Sync встраивается в существующую архитектуру как **post-run hook** в `Runner.Execute()` — между завершением основного цикла `execute()` и финализацией метрик `Metrics.Finish()`. Единственный новый subprocess — короткая Claude-сессия с промптом, сфокусированным на обновлении memories. Zero new packages, zero new dependencies.

### Dependency Direction (без изменений)

```
cmd/ralph
├── runner (+ runSerenaSync method, serena-sync.md template)
│   ├── session (без изменений — переиспользование Execute)
│   ├── gates (без изменений)
│   └── config (+ 3 config fields: serena_sync_*)
└── bridge (без изменений)
```

Config остаётся leaf. Runner владеет sync-логикой. Session переиспользуется как есть. Новый код только в `runner/` и `config/`.

---

## Решение 1: Точка интеграции — Runner.Execute()

### Текущая структура Execute()

```go
func (r *Runner) Execute(ctx context.Context) (*RunMetrics, error) {
    log.Info("run started")

    runErr := r.execute(ctx)     // ← основной цикл

    if r.Metrics != nil {
        rm := r.Metrics.Finish() // ← финализация метрик
        return &rm, runErr
    }
    return nil, runErr
}
```

### Новая структура Execute()

```go
func (r *Runner) Execute(ctx context.Context) (*RunMetrics, error) {
    log.Info("run started")

    runErr := r.execute(ctx)     // ← основной цикл

    // Serena sync: после всех задач, до финализации метрик
    if r.Cfg.SerenaSyncEnabled && r.CodeIndexer != nil && r.CodeIndexer.Available(r.Cfg.ProjectRoot) {
        r.runSerenaSync(ctx)     // ← NEW: sync-сессия
    }

    if r.Metrics != nil {
        rm := r.Metrics.Finish() // ← финализация (включает sync метрики)
        return &rm, runErr
    }
    return nil, runErr
}
```

**Обоснование:**
- Sync после `execute()` — все изменения проверены ревью
- Sync до `Finish()` — метрики sync включены в RunMetrics
- Sync failure не влияет на `runErr` — best-effort операция
- Проверка `CodeIndexer.Available()` — runtime check, Serena может быть недоступна

---

## Решение 2: runSerenaSync() — injectable function

### Паттерн: injectable func field (как DistillFn, ReviewFn)

```go
// runner/runner.go — Runner struct расширение
type Runner struct {
    // ... existing 14 fields ...
    SerenaSyncFn  func(ctx context.Context, opts SerenaSyncOpts) error // NEW
}
```

```go
// runner/serena.go — SerenaSyncOpts и реализация
type SerenaSyncOpts struct {
    DiffSummary    string // git diff --stat firstCommit..HEAD
    Learnings      string // содержимое LEARNINGS.md
    CompletedTasks string // завершённые задачи из sprint-tasks.md
    MaxTurns       int    // из config.SerenaSyncMaxTurns
    ProjectRoot    string
}
```

**Реализация RealSerenaSync:**

```go
func RealSerenaSync(ctx context.Context, opts SerenaSyncOpts) error {
    // 1. Собрать промпт из шаблона serena-sync.md
    prompt := assembleSyncPrompt(opts)

    // 2. Запустить Claude session
    sessOpts := session.Options{
        Prompt:   prompt,
        MaxTurns: opts.MaxTurns,
        Dir:      opts.ProjectRoot,
        // --output-format json, как execute/review
    }
    raw, err := session.Execute(ctx, sessOpts)
    if err != nil {
        return fmt.Errorf("runner: serena sync: %w", err)
    }

    // 3. Проверить exit code
    result, err := session.ParseResult(raw, elapsed)
    if err != nil || result.IsError {
        return fmt.Errorf("runner: serena sync: session failed")
    }
    return nil
}
```

**Обоснование injectable func:**
- Тестируемость: в тестах `SerenaSyncFn = func(...) error { return nil }`
- Консистентность: тот же паттерн, что `ReviewFn`, `DistillFn`, `ResumeExtractFn`
- Не interface: одна реализация, нет альтернатив (как DistillFn)

---

## Решение 3: Промпт-шаблон serena-sync.md

### Размещение

```
runner/prompts/serena-sync.md   ← Go-шаблон (go:embed)
```

### Двухэтапная сборка (существующий паттерн)

**Этап 1: text/template** — условные блоки
```
{{if .HasLearnings}}
## Извлечённые уроки
__LEARNINGS_CONTENT__
{{end}}

{{if .HasCompletedTasks}}
## Завершённые задачи
__COMPLETED_TASKS__
{{end}}
```

**Этап 2: strings.Replace** — user content injection
```go
replacements := map[string]string{
    "__DIFF_SUMMARY__":    opts.DiffSummary,
    "__LEARNINGS_CONTENT__": opts.Learnings,
    "__COMPLETED_TASKS__":   opts.CompletedTasks,
}
```

### Содержимое промпта (концепция)

Промпт инструктирует Claude:
1. `list_memories` — увидеть текущие memories
2. `read_memory` — прочитать затронутые изменениями
3. `edit_memory` — точечные обновления (предпочтительно)
4. `write_memory` — полная перезапись (только при необходимости)
5. **ЗАПРЕТ:** удалять memories, создавать без явной необходимости

### TemplateData расширение

Не нужно. Sync-промпт использует **собственный** TemplateData-like struct (или прямую сборку), т.к. sync — отдельная от execute/review операция с другими полями. Но для консистентности:

```go
// Вариант A: Расширить TemplateData (минимальный diff)
type TemplateData struct {
    // ... existing fields ...
    HasCompletedTasks bool   // NEW: для serena-sync.md
}

// Вариант B: Отдельная структура (чище, но дублирование)
type SyncTemplateData struct {
    HasLearnings      bool
    HasCompletedTasks bool
}
```

**Выбор: Вариант A** — расширение TemplateData. Одно место для всех template fields. `HasCompletedTasks` = false в execute/review (безвредно). `serena-sync.md` не использует execute-only fields (тоже безвредно).

---

## Решение 4: Backup/Rollback механизм

### Файловые операции

```go
// runner/serena.go

const (
    serenaMemoriesDir   = ".serena/memories"
    serenaBackupDir     = ".serena/memories.bak"
)

// backupMemories copies .serena/memories/ → .serena/memories.bak/
func backupMemories(projectRoot string) error {
    src := filepath.Join(projectRoot, serenaMemoriesDir)
    dst := filepath.Join(projectRoot, serenaBackupDir)

    // Удалить предыдущий backup (если есть)
    os.RemoveAll(dst)

    // Рекурсивное копирование
    return copyDir(src, dst)
}

// rollbackMemories restores .serena/memories/ from .serena/memories.bak/
func rollbackMemories(projectRoot string) error {
    src := filepath.Join(projectRoot, serenaBackupDir)
    dst := filepath.Join(projectRoot, serenaMemoriesDir)

    os.RemoveAll(dst)
    return copyDir(src, dst)
}

// cleanupBackup removes .serena/memories.bak/ after successful sync
func cleanupBackup(projectRoot string) {
    os.RemoveAll(filepath.Join(projectRoot, serenaBackupDir))
}
```

### Валидация

```go
// validateMemories checks that memory count didn't decrease
func validateMemories(projectRoot string, countBefore int) error {
    countAfter, err := countMemoryFiles(projectRoot)
    if err != nil {
        return nil // skip validation on read error (best effort)
    }
    if countAfter < countBefore {
        return fmt.Errorf("runner: serena sync: memory count decreased: %d → %d", countBefore, countAfter)
    }
    return nil
}

func countMemoryFiles(projectRoot string) (int, error) {
    dir := filepath.Join(projectRoot, serenaMemoriesDir)
    entries, err := os.ReadDir(dir)
    if err != nil {
        return 0, err
    }
    count := 0
    for _, e := range entries {
        if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
            count++
        }
    }
    return count, nil
}
```

### Полный flow runSerenaSync

```go
func (r *Runner) runSerenaSync(ctx context.Context) {
    log := r.logger()
    log.Info("serena sync started")
    t0 := time.Now()

    // 1. Count memories before
    countBefore, err := countMemoryFiles(r.Cfg.ProjectRoot)
    if err != nil {
        log.Warn("serena sync skipped", "reason", "cannot count memories", "error", err.Error())
        r.recordSyncMetrics("skipped", time.Since(t0))
        return
    }

    // 2. Backup
    if err := backupMemories(r.Cfg.ProjectRoot); err != nil {
        log.Warn("serena sync skipped", "reason", "backup failed", "error", err.Error())
        r.recordSyncMetrics("skipped", time.Since(t0))
        return
    }

    // 3. Build sync opts
    opts := r.buildSyncOpts()

    // 4. Run sync session
    syncErr := r.SerenaSyncFn(ctx, opts)

    // 5. Validate
    if syncErr != nil {
        log.Warn("serena sync failed, rolling back", "error", syncErr.Error())
        rollbackMemories(r.Cfg.ProjectRoot)
        r.recordSyncMetrics("rollback", time.Since(t0))
        return
    }

    if err := validateMemories(r.Cfg.ProjectRoot, countBefore); err != nil {
        log.Warn("serena sync validation failed, rolling back", "error", err.Error())
        rollbackMemories(r.Cfg.ProjectRoot)
        r.recordSyncMetrics("rollback", time.Since(t0))
        return
    }

    // 6. Cleanup backup
    cleanupBackup(r.Cfg.ProjectRoot)
    log.Info("serena sync completed", "duration_ms", time.Since(t0).Milliseconds())
    r.recordSyncMetrics("success", time.Since(t0))
}
```

**Обоснование:**
- `os.RemoveAll` + `copyDir` — простейший backup без symlinks (NTFS-safe)
- Count валидация — минимальная, но ловит критический случай (удаление memories)
- Graceful skip на каждом этапе — sync не блокирует pipeline
- `rollbackMemories` вызывается И при session error, И при validation error

---

## Решение 5: Config расширение

### Новые поля в Config struct

```go
// config/config.go — additions to Config
SerenaSyncEnabled   bool   `yaml:"serena_sync_enabled"`    // default false
SerenaSyncMaxTurns  int    `yaml:"serena_sync_max_turns"`  // default 5
SerenaSyncTrigger   string `yaml:"serena_sync_trigger"`    // default "task" (valid: "run", "task")
```

### Defaults (config/defaults.yaml)

```yaml
serena_sync_enabled: false
serena_sync_max_turns: 5
serena_sync_trigger: "task"
```

### CLI флаг

```go
// cmd/ralph/run.go
flags.BoolVar(&cliFlags.SerenaSyncEnabled, "serena-sync", false, "Enable Serena memory sync after run")
```

CLI `--serena-sync` = `serena_sync_enabled: true`. Стандартный cascade: CLI > config > default.

### Validate()

```go
// В Config.Validate():
if c.SerenaSyncTrigger != "" && c.SerenaSyncTrigger != "run" && c.SerenaSyncTrigger != "task" {
    return fmt.Errorf("config: validate: invalid serena_sync_trigger %q (must be \"run\" or \"task\")", c.SerenaSyncTrigger)
}
if c.SerenaSyncMaxTurns < 1 {
    c.SerenaSyncMaxTurns = 5 // enforce minimum
}
```

---

## Решение 6: Метрики sync-сессии

### Расширение RunMetrics

```go
// runner/metrics.go — addition to RunMetrics
type RunMetrics struct {
    // ... existing fields ...
    SerenaSync *SerenaSyncMetrics `json:"serena_sync,omitempty"` // NEW
}

type SerenaSyncMetrics struct {
    Status     string  `json:"status"`      // success, skipped, failed, rollback
    DurationMs int64   `json:"duration_ms"`
    TokensIn   int     `json:"tokens_input,omitempty"`
    TokensOut  int     `json:"tokens_output,omitempty"`
    CostUSD    float64 `json:"cost_usd,omitempty"`
}
```

### recordSyncMetrics

```go
func (r *Runner) recordSyncMetrics(status string, duration time.Duration) {
    if r.Metrics == nil {
        return
    }
    r.Metrics.RecordSerenaSync(status, duration.Milliseconds())
}
```

### В stdout summary

```
Serena sync: success ($0.05, 12s)
```

или

```
Serena sync: skipped (disabled)
```

**Обоснование:** `SerenaSyncMetrics` — отдельная структура (не TaskMetrics), т.к. sync — не задача. `omitempty` — при отключённом sync поле отсутствует в JSON.

---

## Решение 7: Per-task trigger (default) и batch trigger

По умолчанию (`serena_sync_trigger: "task"`) sync запускается в `execute()` после каждой успешной задачи (после knowledge extraction, перед gate):

```go
// В execute() loop, после knowledge extraction:
if r.Cfg.SerenaSyncEnabled && r.Cfg.SerenaSyncTrigger == "task" && serenaAvailable {
    r.runSerenaSync(ctx) // backup/sync/validate per task
}
```

При `trigger: "run"` (batch) — sync один раз в `Execute()` после цикла (как описано в Решении 1). Опциональная альтернатива для экономии на коротких прогонах (2-3 задачи).

**Обоснование:** Per-task = default, т.к. длинные прогоны (10+ задач, часы работы) приводят к сильному устареванию memories. Инкрементальный sync сохраняет актуальность на протяжении всего прогона. Batch = opt-in для коротких прогонов.

---

## Решение 8: Diff Summary для промпта

### Сбор diff summary

```go
func (r *Runner) buildSyncOpts() SerenaSyncOpts {
    // 1. Git diff summary: first commit of run vs HEAD
    diffSummary := ""
    if r.Metrics != nil && len(r.Metrics.tasks) > 0 {
        firstCommit := r.Metrics.tasks[0].CommitSHA
        if firstCommit != "" {
            // git diff --stat firstCommit..HEAD
            diffSummary, _ = r.Git.DiffStat(ctx, firstCommit)
        }
    }

    // 2. LEARNINGS.md content
    learnings, _ := os.ReadFile(filepath.Join(r.Cfg.ProjectRoot, "LEARNINGS.md"))

    // 3. Completed tasks (из sprint-tasks.md — строки с [x])
    completedTasks := extractCompletedTasks(r.TasksFile)

    return SerenaSyncOpts{
        DiffSummary:    diffSummary,
        Learnings:      string(learnings),
        CompletedTasks: completedTasks,
        MaxTurns:       r.Cfg.SerenaSyncMaxTurns,
        ProjectRoot:    r.Cfg.ProjectRoot,
    }
}
```

### GitClient расширение (minimal)

```go
// runner/git.go — новый метод (если нет общего DiffStats)
type GitClient interface {
    // ... existing methods ...
    DiffStat(ctx context.Context, fromCommit string) (string, error) // NEW: git diff --stat fromCommit..HEAD
}
```

**Альтернатива:** Если `DiffStats` уже есть из Epic 7, можно форматировать его output как текст для промпта. Не требуется новый метод — только форматирование `DiffStats` → string.

---

## Data Flow: Serena Sync

```
Runner.Execute(ctx)
  │
  ├── r.execute(ctx)                    ← основной цикл (без изменений)
  │     │
  │     └── [per-task loop]
  │           ├── execute session
  │           ├── review session
  │           ├── knowledge extraction
  │           ├── [if trigger=="task"] → r.runSerenaSync(ctx)  (FR64, Growth)
  │           └── gate
  │
  ├── [if SerenaSyncEnabled && Serena available]
  │     │
  │     └── r.runSerenaSync(ctx)        ← NEW (FR57)
  │           │
  │           ├── countMemoryFiles()    (validation baseline)
  │           ├── backupMemories()      (FR61: .serena/memories → .bak)
  │           ├── buildSyncOpts()       (FR58: diff + learnings + tasks)
  │           │     ├── git diff --stat
  │           │     ├── os.ReadFile(LEARNINGS.md)
  │           │     └── extractCompletedTasks()
  │           │
  │           ├── r.SerenaSyncFn(ctx, opts)  (FR57: Claude session)
  │           │     ├── assembleSyncPrompt(opts)  (FR59: template + replace)
  │           │     └── session.Execute(ctx, sessOpts)
  │           │
  │           ├── validateMemories()    (FR62: count check)
  │           │     ├── OK → cleanupBackup()
  │           │     └── FAIL → rollbackMemories()
  │           │
  │           └── recordSyncMetrics()   (FR65: status, duration, tokens)
  │
  └── r.Metrics.Finish()                ← RunMetrics (includes sync data)
```

---

## FR → Package/File Mapping

| FR | Package | File(s) | Тип изменения |
|----|---------|---------|---------------|
| FR57 | runner | runner.go, serena.go | runSerenaSync call in Execute(), SerenaSyncFn field |
| FR58 | runner | serena.go | buildSyncOpts(), SerenaSyncOpts struct |
| FR59 | runner | prompts/serena-sync.md (new), serena.go | Template + assembly function |
| FR60 | runner | serena.go | MaxTurns in session.Options |
| FR61 | runner | serena.go | backupMemories(), rollbackMemories(), cleanupBackup() |
| FR62 | runner | serena.go | validateMemories(), countMemoryFiles() |
| FR63 | config | config.go, defaults.yaml | 3 new fields + CLI flag + Validate |
| FR64 | runner | runner.go | Per-task trigger in execute() loop (Growth) |
| FR65 | runner | metrics.go, serena.go | SerenaSyncMetrics, RecordSerenaSync, recordSyncMetrics |
| FR66 | runner | serena.go | Graceful skip on MCP unavailable |

---

## Новые файлы

| File | Package | LOC est. | Content |
|------|---------|----------|---------|
| `runner/prompts/serena-sync.md` | runner (embed) | ~50 | Go-шаблон sync промпта |

---

## Изменения в существующих файлах

| File | Changes |
|------|---------|
| `runner/serena.go` | +SerenaSyncOpts, +RealSerenaSync, +backupMemories, +rollbackMemories, +cleanupBackup, +validateMemories, +countMemoryFiles, +buildSyncOpts, +extractCompletedTasks, +assembleSyncPrompt |
| `runner/runner.go` | +SerenaSyncFn field in Runner, +runSerenaSync call in Execute(), +recordSyncMetrics helper, +(Growth) per-task trigger in execute() |
| `runner/metrics.go` | +SerenaSyncMetrics struct, +RecordSerenaSync method, +SerenaSync field in RunMetrics |
| `config/config.go` | +3 fields (SerenaSyncEnabled, SerenaSyncMaxTurns, SerenaSyncTrigger), Validate extension |
| `config/defaults.yaml` | +3 default values |
| `config/prompt.go` | +HasCompletedTasks in TemplateData |
| `cmd/ralph/run.go` | +--serena-sync CLI flag, sync status in stdout summary |

---

## Риски и mitigation

| Риск | Mitigation |
|------|------------|
| NTFS backup quirks (WSL) | `filepath.Walk` + explicit copy, no symlinks. Тестируется на WSL |
| Sync corruption → rollback | Backup before, validate after, auto-rollback on any failure |
| Stale first commit SHA for diff | Fallback to empty diff if no commits in run |
| Large diff overwhelms sync prompt | `git diff --stat` (summary), не полный diff. Max ~200 lines |
| Serena MCP unavailable mid-sync | Session timeout + graceful error → rollback |
| Per-task trigger cost | Default = "task" (per-task). Batch "run" = opt-in для экономии. Max turns (5) ограничивает каждую sync-сессию |

---

## Testing Strategy

- **Unit:** `countMemoryFiles`, `validateMemories`, `backupMemories`/`rollbackMemories` (с t.TempDir), `assembleSyncPrompt`, `buildSyncOpts`, `extractCompletedTasks`
- **Integration:** Runner с SerenaSyncFn mock — verify sync called after execute, verify backup/rollback flow, verify metrics recorded
- **Template:** serena-sync.md golden file test — conditional blocks, replacements
- **Config:** New fields in Load() / Validate() / defaults / CLI flag tests
- **Edge cases:** Serena unavailable (skip), backup failure (skip), validation failure (rollback), empty run (no commits → skip sync)
- **Backward compat:** All existing tests pass with SerenaSyncFn == nil (не вызывается при SerenaSyncEnabled == false)
