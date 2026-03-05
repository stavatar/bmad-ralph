# Story 7.8: Similarity Detection

Status: Done

## Story

As a разработчик,
I want чтобы ralph обнаруживал повторяющиеся diff-паттерны (зацикливание),
so that не тратить ресурсы на бесплодные попытки.

## Acceptance Criteria

1. **AC1: SimilarityDetector struct (FR51)**
   - `runner/similarity.go` (НОВЫЙ файл) определяет `SimilarityDetector`
   - `NewSimilarityDetector(window, warnAt, hardAt)` — конструктор
   - Initialized с empty history slice, stored window/warnAt/hardAt

2. **AC2: JaccardSimilarity function (FR51)**
   - `JaccardSimilarity(a, b []string) float64`
   - Возвращает `|intersection| / |union|`
   - Empty slices → 0.0 (не NaN, не panic)
   - Identical slices → 1.0

3. **AC3: Push and Check cycle (FR51)**
   - `Push(diffLines []string)` добавляет в sliding window
   - `Push` с nil/empty slice — no-op (skip push, не добавлять пустую запись)
   - `Check() (level string, score float64)`:
     - Вычисляет avg pairwise Jaccard similarity по ВСЕМ парам в window (не только consecutive)
     - avg similarity > hardAt → `("hard", 0.96)` (hardAt проверяется ПЕРВЫМ)
     - avg similarity > warnAt → `("warn", 0.87)`
     - diffs diverse → `("", 0.5)`
   - Window < 2 entries → `("", 0.0)` (недостаточно данных для сравнения)
   - Window size ограничен: при overflow — удаление oldest (FIFO)

4. **AC4: Warning threshold action (FR51)**
   - similarity "warn": `logger.Warn("similarity warning", kv("score", 0.87), kv("threshold", 0.85))`
   - `InjectFeedback(r.TasksFile, taskDesc, "Recent changes are very similar. Try a fundamentally different approach.")` — 3 params

5. **AC5: Hard threshold action (FR51)**
   - similarity "hard": `logger.Error("similarity loop detected", kv("score", 0.96), kv("threshold", 0.95))`
   - `r.EmergencyGatePromptFn(ctx, taskText + "\nSimilarity loop detected (score: X.XX). Diffs are repeating.")` — передаётся ctx и enriched taskText

6. **AC6: Similarity disabled by default (NFR23)**
   - `Config.SimilarityWindow == 0` (default) → detector не создаётся, проверки не выполняются

7. **AC7: Config fields (FR51)**
   - `SimilarityWindow int` yaml:"similarity_window" (default 0 = disabled)
   - `SimilarityWarn float64` yaml:"similarity_warn" (default 0.85)
   - `SimilarityHard float64` yaml:"similarity_hard" (default 0.95)

8. **AC8: Config validation**
   - При `SimilarityWindow > 0`: `SimilarityWarn` must be in (0.0, 1.0), `SimilarityHard` must be in (0.0, 1.0)
   - `SimilarityWarn < SimilarityHard` — warn must be strictly less than hard
   - Violation → `fmt.Errorf("config: validate: similarity_warn must be less than similarity_hard")`
   - При `SimilarityWindow == 0` — validation of warn/hard skipped (feature disabled)
   - `SimilarityWindow < 0` → validation error

9. **AC9: Diff lines source**
   - При commit detected: используется `Packages []string` из `DiffStats` (Story 7.2) как similarity signal
   - DiffStats.Packages = уникальные parent directories из changed files (sorted)
   - НЕ добавлять `DiffLines` в GitClient — DiffStats уже доступен после commit detection (Story 7.2 вызывает `r.Git.DiffStats(ctx, headBefore, headAfter)`)
   - Если DiffStats вызов fails (best-effort per NFR24) — skip similarity check для этой итерации

## Tasks / Subtasks

- [x] Task 1: Create runner/similarity.go (AC: #1, #2, #3)
  - [x] 1.1 `JaccardSimilarity(a, b []string) float64` — set intersection/union
  - [x] 1.2 `SimilarityDetector` struct: window, warnAt, hardAt, history [][]string
  - [x] 1.3 `Push(diffLines)` — append + trim to window size
  - [x] 1.4 `Check() (string, float64)` — compute avg pairwise similarity in window
  - [x] 1.5 Тесты: Jaccard edge cases (empty, identical, disjoint, partial overlap)
  - [x] 1.6 Тесты: Push/Check cycle (warn, hard, diverse)

- [x] Task 2: Config fields + validation (AC: #6, #7, #8)
  - [x] 2.1 Добавить SimilarityWindow, SimilarityWarn, SimilarityHard в Config
  - [x] 2.2 Defaults: window=0, warn=0.85, hard=0.95 в defaults.yaml
  - [x] 2.3 Validate (в Config.Validate()): при SimilarityWindow > 0: warn 0-1, hard 0-1, warn < hard. При SimilarityWindow < 0: error
  - [x] 2.4 Тесты: default values, override, validation errors (invalid range, warn >= hard, negative window)

- [x] Task 3: Data source for similarity (AC: #9)
  - [x] 3.1 Reuse `DiffStats.Packages []string` из Story 7.2 как similarity signal
  - [x] 3.2 Story 7.2 вызывает `r.Git.DiffStats(ctx, headBefore, headAfter)` после commit detection — stats уже доступны
  - [x] 3.3 Передавать `stats.Packages` в `detector.Push()` (nil/empty Packages → skip push, per AC3)
  - [x] 3.4 НЕ добавлять новый метод в GitClient — всё нужное уже есть в DiffStats

- [x] Task 4: Runner integration (AC: #4, #5, #6)
  - [x] 4.1 В runner: если cfg.SimilarityWindow > 0 → create detector
  - [x] 4.2 После commit: push diff lines, check thresholds
  - [x] 4.3 "warn" → logger.Warn + InjectFeedback
  - [x] 4.4 "hard" → logger.Error + EmergencyGatePromptFn
  - [x] 4.5 SimilarityWindow == 0 → skip all similarity logic

- [x] Task 5: Тесты (AC: #1-#9)
  - [x] 5.1 `TestJaccardSimilarity_EdgeCases` — table-driven: empty, identical, disjoint, partial overlap (AC2)
  - [x] 5.2 `TestSimilarityDetector_Push_WindowOverflow` — FIFO eviction (AC3)
  - [x] 5.3 `TestSimilarityDetector_Check_Thresholds` — table-driven: warn, hard, diverse, insufficient data (AC3)
  - [x] 5.4 `TestSimilarityDetector_Push_EmptySlice` — no-op for nil/empty (AC3)
  - [x] 5.5 Runner integration: similar diffs → warn log + InjectFeedback (AC4)
  - [x] 5.6 Runner integration: hard threshold → EmergencyGatePromptFn (AC5)
  - [x] 5.7 Runner: SimilarityWindow == 0 → no detector, no detection (AC6)
  - [x] 5.8 Config validation: invalid ranges, warn >= hard, negative window (AC8)

## Dev Notes

### Изменяемые файлы

| Файл | Тип изменения | LOC est. |
|------|---------------|----------|
| `runner/similarity.go` (NEW) | Create: JaccardSimilarity, SimilarityDetector | ~80 |
| `runner/similarity_test.go` (NEW) | Create: unit тесты | ~100 |
| `config/config.go` | Extend: +3 similarity fields | ~5 |
| `config/defaults.yaml` | Extend: +similarity defaults | ~3 |
| `runner/runner.go` | Extend: detector creation, post-commit check | ~20 |
| `runner/runner_test.go` | Add: similarity integration tests | ~40 |

### Architecture Compliance

- **New file `runner/similarity.go`** — pure computation, no external deps
- **Disabled by default** (NFR23) — zero behavior change without config
- **Reuses EmergencyGatePromptFn** and InjectFeedback — existing infrastructure
- **Data source deviation from Architecture Решение 8:** Architecture says "full diff lines via `git diff`", this story uses `DiffStats.Packages` (directory names). Rationale: Packages is cheaper (already computed by Story 7.2), sufficient for loop detection (same dirs = same area of code), avoids adding `DiffLines` method to GitClient

### Key Technical Decisions

1. **JaccardSimilarity = set-based** — treats Packages lists as sets, computes |intersection|/|union|
2. **Avg pairwise similarity** в window — not just consecutive, but all pairs in window
3. **Packages (dir list) as signal** — from DiffStats (Story 7.2), cheaper than full diff, sufficient for loop detection
4. **Sliding window** — old entries dropped when window full, FIFO
5. **Config validation** — warn/hard thresholds must be 0-1, warn < hard (при SimilarityWindow > 0)

### Existing Code Context (дополнение)

- `DiffStats` struct (`runner/metrics.go:10-15`): `FilesChanged int`, `Insertions int`, `Deletions int`, `Packages []string` — Packages уже есть (Story 7.2)
- Story 7.2 вызывает `r.Git.DiffStats(ctx, headBefore, headAfter)` после commit detection в else-ветке `if headBefore == headAfter` — `stats` variable доступна для similarity push
- `InjectFeedback(tasksFile string, taskDesc string, feedback string) error` (`runner/runner.go:312`) — 3 params, error wrapping `"runner: inject feedback:"`
- `EmergencyGatePromptFn` field на Runner (`runner/runner.go:425`): тип `GatePromptFunc = func(ctx context.Context, taskText string) (*config.GateDecision, error)` — emergency gate, только retry/skip/quit (НЕТ approve, per Story 5.5)
- `Config.Validate()` — уже существует в `config/config.go`, добавлять similarity validation в этот метод

### Prerequisites

- Story 7.2 (DiffStats with Packages field — source data for similarity)

### References

- [Source: docs/architecture/observability-metrics.md#Решение 8] — Similarity Detection
- [Source: docs/prd/observability-metrics.md#FR51] — similarity detection
- [Source: docs/epics/epic-7-observability-metrics-stories.md#Story 7.8] — полное описание

## Dev Agent Record

### Context Reference

### Agent Model Used
Claude Opus 4.6

### Debug Log References

### Completion Notes List
- Task 1: Created `runner/similarity.go` with `JaccardSimilarity`, `SimilarityDetector`, `NewSimilarityDetector`, `Push`, `Check`. Unit tests: 11 Jaccard edge cases (table-driven), Push empty/nil no-op, window overflow FIFO, Check thresholds (6 table cases including priority).
- Task 2: Added `SimilarityWindow`, `SimilarityWarn`, `SimilarityHard` to `Config` struct. Defaults in `defaults.yaml` (0, 0.85, 0.95). Validation in `Config.Validate()`: negative window, range checks, warn < hard (8 new error table cases + 2 happy path tests + defaults assertions).
- Task 3: Confirmed `DiffStats.Packages` from Story 7.2 already available in commit detection path. No new GitClient methods needed.
- Task 4: Added `Similarity *SimilarityDetector` field to `Runner`. Post-commit integration: push `diffStats.Packages`, check thresholds, warn → `InjectFeedback`, hard → `EmergencyGatePromptFn` with enriched taskText. Uses `config.ActionQuit` + `decision` wrapping (consistent with existing emergency gate pattern).
- Task 5: All tests pass — 4 unit tests (similarity_test.go), 3 runner integration tests (warn feedback injection, hard gate trigger, disabled when window=0), 8 config validation error cases, 2 config happy path, 3 config defaults assertions.

### File List
- `runner/similarity.go` (NEW) — JaccardSimilarity, SimilarityDetector, NewSimilarityDetector, Push, Check
- `runner/similarity_test.go` (NEW) — TestJaccardSimilarity_EdgeCases, TestSimilarityDetector_Push_EmptySlice, TestSimilarityDetector_Push_WindowOverflow, TestSimilarityDetector_Check_Thresholds
- `config/config.go` (MODIFIED) — +3 Config fields (SimilarityWindow, SimilarityWarn, SimilarityHard), +validation in Validate()
- `config/defaults.yaml` (MODIFIED) — +3 similarity defaults
- `config/config_test.go` (MODIFIED) — +8 validation error cases, +2 happy path tests, +3 defaults assertions
- `runner/runner.go` (MODIFIED) — +Similarity field on Runner, +similarity detection in Execute post-commit path
- `runner/runner_test.go` (MODIFIED) — +3 integration tests (warn, hard, disabled)

## Change Log
- 2026-03-05: Implemented Story 7.8 — Similarity Detection (all 5 tasks, 9 ACs satisfied)
- 2026-03-05: Code review — 6 findings (1C/0H/3M/2L), all fixed. C1: Run() missing SimilarityDetector init (dead feature). M1: doc "consecutive" misleading. M2: missing hard+no-gates test. M3: enriched text assertions incomplete. L1: test name misleading. L2: doc claim without validation.
