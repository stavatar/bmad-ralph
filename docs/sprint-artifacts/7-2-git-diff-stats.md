# Story 7.2: Git Diff Stats

Status: done

## Story

As a разработчик,
I want видеть git diff statistics (files changed, insertions, deletions, packages) после каждого commit,
so that оценивать объём изменений per task.

## Acceptance Criteria

1. **AC1: GitClient interface extension (FR44)**
   - `DiffStats(ctx context.Context, before, after string) (*DiffStats, error)` добавлен в `GitClient` interface
   - Interface теперь имеет 4 метода: `HealthCheck`, `HeadCommit`, `RestoreClean`, `DiffStats`
   - `DiffStats` struct уже определён в `runner/metrics.go` (Story 7.1), но ему не хватает `Packages` field — добавить: `Packages []string \`json:"packages"\`` к существующему struct. НЕ создавать дубликат в `runner/git.go`

2. **AC2: ExecGitClient.DiffStats implementation**
   - Выполняет `git diff --numstat <before> <after>` через `exec.CommandContext(ctx, ...)` с `cmd.Dir = c.Dir`
   - Парсит tab-separated output: `insertions<TAB>deletions<TAB>filename`
   - Skip пустых строк при parsing (trailing newline в git output)
   - `FilesChanged` = количество непустых строк в output
   - `Insertions`/`Deletions` = сумма соответствующих колонок
   - `Packages` = уникальные parent directories из filenames (отсортированные)
   - Binary файлы (`"-"` в numstat) считаются как `FilesChanged` но 0 insertions/deletions

3. **AC3: DiffStats with identical SHAs**
   - При `before == after` возвращает `&DiffStats{}` с нулями и пустым Packages
   - Не вызывает git команду (early return)

4. **AC4: DiffStats error handling**
   - При невалидном SHA или git ошибке — wrapped error: `"runner: diff stats: %w"`
   - Пустой `before` или `after` — ошибка (не panic)

5. **AC5: MockGitClient extension**
   - `DiffStatsResults []*DiffStats` (indexed slice, как `HeadCommits []string`) и `DiffStatsErrors []error` fields добавлены — поддерживает многократный вызов в execute loop (несколько commits)
   - `DiffStatsCount int` для tracking call count
   - `DiffStats(ctx, before, after)` метод возвращает `DiffStatsResults[DiffStatsCount]` / `DiffStatsErrors[DiffStatsCount]` (indexed), increment count
   - Beyond-length: если count >= len(slice) → возвращать last element (паттерн `HeadCommits`)

6. **AC6: Logging after commit (FR44)**
   - В execute loop, когда `headBefore != headAfter` (commit detected, в `else` ветке существующего `if headBefore == headAfter`):
     - Вызывается `r.Git.DiffStats(ctx, headBefore, headAfter)`
     - При успехе: `logger.Info("commit stats", kv("files", N), kv("insertions", N), kv("deletions", N), kv("packages", "runner,config"))`
     - При ошибке: `logger.Warn` с error, execution continues (NFR24 — best effort)
   - Если Runner уже имеет `Metrics *MetricsCollector` field (добавленный Story 7.1) И `r.Metrics != nil` — вызвать `r.Metrics.RecordGitDiff(stats)`. Если field ещё не добавлен — пропустить RecordGitDiff, оставить `// TODO(7.1): r.Metrics.RecordGitDiff(stats)` comment

## Tasks / Subtasks

- [x] Task 1: Extend GitClient interface + DiffStats struct (AC: #1)
  - [x] 1.1 Добавить `Packages []string \`json:"packages"\`` field к существующему `DiffStats` struct в `runner/metrics.go` (struct уже имеет FilesChanged, Insertions, Deletions — добавить 4-е поле)
  - [x] 1.2 Добавить `DiffStats(ctx context.Context, before, after string) (*DiffStats, error)` в `GitClient` interface в `runner/git.go`
  - [x] 1.3 Убедиться что компиляция проходит (ExecGitClient и MockGitClient реализуют интерфейс)

- [x] Task 2: Implement ExecGitClient.DiffStats (AC: #2, #3, #4)
  - [x] 2.1 Early return для `before == after` → `&DiffStats{}, nil`
  - [x] 2.2 Validate inputs: пустой before/after → error
  - [x] 2.3 Execute `exec.CommandContext(ctx, "git", "diff", "--numstat", before, after)` с `cmd.Dir = c.Dir`
  - [x] 2.4 Parse output: `strings.Split(output, "\n")`, для каждой строки split по `\t`
  - [x] 2.5 Handle binary files: `"-"` in insertions/deletions → 0, но FilesChanged++
  - [x] 2.6 Compute Packages: `filepath.Dir(filename)` для каждого файла, deduplicate, sort
  - [x] 2.7 Error wrapping: `fmt.Errorf("runner: diff stats: %w", err)`

- [x] Task 3: Extend MockGitClient (AC: #5)
  - [x] 3.1 Добавить fields: `DiffStatsResults []*DiffStats`, `DiffStatsErrors []error`, `DiffStatsCount int` (indexed slices, как HeadCommits/HeadCommitErrors)
  - [x] 3.2 Реализовать метод `DiffStats(ctx, before, after) (*DiffStats, error)` — indexed return по DiffStatsCount, increment count, beyond-length returns last element
  - [x] 3.3 Обновить mock_git_test.go с тестами для DiffStats

- [x] Task 4: Integrate into runner execute loop (AC: #6)
  - [x] 4.1 В `runner/runner.go` execute(): в `else` ветке `if headBefore == headAfter` (строка ~690, commit detected), вызвать `r.Git.DiffStats(ctx, headBefore, headAfter)`
  - [x] 4.2 При успехе: log via `r.logger().Info("commit stats", kv("files", stats.FilesChanged), kv("insertions", stats.Insertions), kv("deletions", stats.Deletions), kv("packages", strings.Join(stats.Packages, ",")))`
  - [x] 4.3 При ошибке: `r.logger().Warn("diff stats failed", kv("error", err.Error()))` — продолжить execution (NFR24 best-effort)
  - [x] 4.4 Если Runner имеет `Metrics` field (Story 7.1) и `r.Metrics != nil`: вызвать `r.Metrics.RecordGitDiff(stats)`. Если field не существует — оставить TODO comment

- [x] Task 5: Unit и integration тесты (AC: #1-#6)
  - [x] 5.1 Real git тесты ExecGitClient.DiffStats: create repo, make commits, verify stats
  - [x] 5.2 Тест identical SHAs → zero stats
  - [x] 5.3 Тест binary файлы в numstat output: TestExecGitClient_DiffStats_BinaryFile
  - [x] 5.4 Тест error paths: invalid SHA, empty params
  - [x] 5.5 Тест Packages extraction и sorting
  - [x] 5.6 Runner integration: TestRunner_Execute_DiffStatsIntegration (verify DiffStatsCount, TaskMetrics.Diff fields), TestRunner_Execute_DiffStatsError (best-effort NFR24), TestRunner_Execute_DiffStatsWithoutMetrics (nil Metrics guard)
  - [x] 5.7 MockGitClient.DiffStats beyond-length test: вызов count > len(slice) → returns last element (паттерн HeadCommits)

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `runner/metrics.go` | Extend: +Packages field к существующему DiffStats struct | ~2 |
| `runner/git.go` | Extend: +interface method, +ExecGitClient.DiffStats implementation | ~45 |
| `runner/git_test.go` | Add: real git тесты DiffStats | ~80 |
| `internal/testutil/mock_git.go` | Extend: +DiffStats fields/method | ~15 |
| `internal/testutil/mock_git_test.go` | Add: DiffStats mock tests | ~20 |
| `runner/runner.go` | Extend: diff stats call after commit detection | ~15 |
| `runner/runner_test.go` | Add: тесты diff stats integration | ~40 |

### Architecture Compliance

- **Dependency direction:** DiffStats struct в `runner/metrics.go`, метод в `runner/git.go` — тот же package, без новых зависимостей
- **GitClient interface:** в `runner/` (consumer package) — паттерн соблюдён
- **MockGitClient:** в `internal/testutil/` — существующий паттерн
- **Error wrapping:** `"runner: diff stats: %w"` — package prefix + operation

### Key Technical Decisions

1. **DiffStats struct расширяется в `runner/metrics.go`** (уже определён Story 7.1, добавляем Packages field). Оба файла в одном package `runner` — тип доступен в git.go напрямую
2. **`git diff --numstat`** — numstat формат даёт machine-parseable output (не `--stat`)
3. **Packages = `filepath.Dir()`** от filename — дешёвый способ определить touched packages
4. **Best-effort (NFR24):** ошибка DiffStats не прерывает execution — warn и continue

### Existing Code Context

- `GitClient` interface (runner/git.go:L1-L5): `HealthCheck(ctx)`, `HeadCommit(ctx)`, `RestoreClean(ctx)` — добавить `DiffStats(ctx, before, after)`
- `ExecGitClient` struct (runner/git.go): `Dir string` field — для `cmd.Dir`
- `DiffStats` struct (runner/metrics.go:L10-L14): уже имеет `FilesChanged`, `Insertions`, `Deletions` — **НЕ хватает `Packages []string`**, добавить
- `MockGitClient` fields (internal/testutil/mock_git.go): indexed slices (`HeadCommits []string`, `HeadCommitErrors []error`) + count tracking (`HeadCommitCount int`). `RestoreCleanError` — НЕ indexed (одноразовый вызов). DiffStats ДОЛЖЕН быть indexed (вызывается многократно в execute loop — по одному на каждый commit)
- Execute loop в `runner.go` (строка ~690): `if headBefore == headAfter { ... } else { ... }` — DiffStats вставляется в `else` ветку (commit detected)
- `Runner.Execute` (runner/runner.go:L473): текущая сигнатура `Execute(ctx context.Context) error` — Story 7.1 может изменить на `(*RunMetrics, error)`, но DiffStats интеграция внутри loop, не зависит от return type
- Runner struct (runner/runner.go:L419-L432): НЕ имеет `Metrics` field пока. Если Story 7.1 реализована первой — будет `Metrics *MetricsCollector`
- `runGit(t, dir, args...)` helper в `git_test.go` — использовать для real git тестов
- **НЕ hardcode'ить branch name** "master": использовать `git rev-parse --abbrev-ref HEAD` после init

### References

- [Source: docs/architecture/observability-metrics.md#Решение 4] — GitClient расширение
- [Source: docs/prd/observability-metrics.md#FR44] — git diff stats
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.2] — полное описание стори

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6

### Completion Notes List

- All 5 tasks completed, all AC verified
- DiffStats struct extended with Packages field (reused from Story 7.1)
- ExecGitClient.DiffStats: `git diff --numstat` parsing with binary file handling
- MockGitClient extended with indexed DiffStats (same pattern as HeadCommits)
- Runner execute loop: best-effort DiffStats after commit detection (NFR24)
- MetricsCollector.RecordGitDiff integration (nil-guarded)
- nopGitClient in coverage_internal_test.go updated for interface compliance
- 10 test functions total: 6 real git (HappyPath, IdenticalSHAs, EmptyParams, InvalidSHA, DeletionsAndModifications, BinaryFile) + 2 mock (Sequence, Error) + 3 runner integration (DiffStatsIntegration, DiffStatsError, DiffStatsWithoutMetrics)

### File List

| File | Action | Description |
|------|--------|-------------|
| `runner/metrics.go` | Modified | +Packages field to DiffStats struct |
| `runner/git.go` | Modified | +DiffStats method on GitClient interface, +ExecGitClient.DiffStats implementation, +sort import |
| `runner/git_test.go` | Modified | +5 test functions: HappyPath, IdenticalSHAs, EmptyParams, InvalidSHA, DeletionsAndModifications |
| `runner/runner.go` | Modified | +DiffStats call in commit detection else-branch, +log/warn, +MetricsCollector integration |
| `runner/coverage_internal_test.go` | Modified | +DiffStats method on nopGitClient |
| `internal/testutil/mock_git.go` | Modified | +DiffStats fields/method on MockGitClient |
| `internal/testutil/mock_git_test.go` | Modified | +2 test functions: Sequence, Error; +fmt/strings imports |
| `runner/runner_test.go` | Modified | +3 integration tests: DiffStatsIntegration, DiffStatsError, DiffStatsWithoutMetrics |
