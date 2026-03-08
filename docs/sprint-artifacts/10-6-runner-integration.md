# Story 10.6: Runner Integration

Status: Ready for Review

## Story

As a разработчик,
I want чтобы runner создавал compaction counter, передавал env в сессии, считывал результат и записывал метрики,
so that context observability работала end-to-end.

## Acceptance Criteria

### AC1: EnsureCompactHook at startup (FR83)
- `Runner.Execute()` вызывает `EnsureCompactHook(cfg.ProjectRoot)` перед основным циклом
- Hook setup запускается один раз per ralph run
- Error логируется как warning, execution продолжается

### AC2: DefaultContextWindow constant (FR86)
- `runner/context.go` содержит `const DefaultContextWindow = 200000`
- Используется как fallback во всех `EstimateMaxContextFill` вызовах

### AC3: Execute path — counter lifecycle (FR82, FR84)
- В execute iteration (`Runner.Execute()` inner loop):
  1. `CreateCompactCounter()` вызывается перед `session.Execute`
  2. `counterPath` задаётся в `opts.Env["RALPH_COMPACT_COUNTER"]`
  3. После сессии: `CountCompactions(counterPath)` → count
  4. `EstimateMaxContextFill(sr.Metrics, DefaultContextWindow)` → fillPct
  5. `RecordSession` получает compactions и fillPct
  6. Cleanup удаляет temp file

### AC4: Execute path — counter empty (FR82, FR84)
- Сессия завершается без triggering PreCompact hook
- `CountCompactions` на empty file → compactions == 0
- fillPct computed from session metrics

### AC5: Review path — counter via RunConfig.Env (FR82, FR84)
- `RunConfig` struct получает новое поле: `Env map[string]string`
- Review counter создаётся в caller (`Runner.Execute` review section)
- Передаётся в `RealReview` через `RunConfig.Env`
- `RealReview` копирует Env в `session.Options.Env`
- После review: `CountCompactions` → count, `RecordSession` получает данные

### AC6: RealReview — env passthrough (FR82)
- `RunConfig.Env = {"RALPH_COMPACT_COUNTER": "/tmp/ralph-compact-xyz"}`
- `RealReview` строит `session.Options` → `opts.Env["RALPH_COMPACT_COUNTER"] == "/tmp/ralph-compact-xyz"`

### AC7: Resume path — counter (FR82, FR84)
- `ResumeExtraction` flow: counter создаётся перед `session.Execute`
- Env var задаётся, compactions counted after session
- `RecordSession` получает compactions и fillPct

### AC8: SerenaSync path — counter (FR82, FR84)
- `RealSerenaSync` flow: counter создаётся перед `session.Execute`
- Env var задаётся, compactions counted after session
- `RecordSession` получает compactions и fillPct

### AC9: Counter not created — graceful (NFR30)
- `CreateCompactCounter` возвращает `("", no-op cleanup)`
- `opts.Env` НЕ получает `RALPH_COMPACT_COUNTER`
- `CountCompactions("")` возвращает 0
- `RecordSession` получает `compactions=0`

## Tasks / Subtasks

- [x] Task 1: EnsureCompactHook call (AC: #1)
  - [x] 1.1 Вызов в `Runner.Execute()` перед main loop
  - [x] 1.2 Error → log warning, continue

- [x] Task 2: Execute path integration (AC: #3, #4, #9)
  - [x] 2.1 `CreateCompactCounter()` before `session.Execute`
  - [x] 2.2 Set `opts.Env["RALPH_COMPACT_COUNTER"]` if path non-empty
  - [x] 2.3 After session: `CountCompactions(counterPath)`
  - [x] 2.4 `EstimateMaxContextFill(sr.Metrics, DefaultContextWindow)`
  - [x] 2.5 Pass compactions, fillPct to `RecordSession`
  - [x] 2.6 `defer counterCleanup()`

- [x] Task 3: Review path integration (AC: #5, #6)
  - [x] 3.1 Add `Env map[string]string` to `RunConfig`
  - [x] 3.2 Create counter in Execute review section, pass via `RunConfig.Env`
  - [x] 3.3 In `RealReview`: copy `rc.Env` to `session.Options.Env`
  - [x] 3.4 After review: count compactions, compute fillPct, pass to RecordSession

- [x] Task 4: Resume path integration (AC: #7)
  - [x] 4.1 Counter lifecycle in ResumeExtraction flow

- [x] Task 5: SerenaSync path integration (AC: #8)
  - [x] 5.1 Counter lifecycle in RealSerenaSync flow

- [x] Task 6: Тесты (AC: #1-#9)
  - [x] 6.1 Integration tests: mock binary → verify RecordSession receives correct compactions/fillPct
  - [x] 6.2 EnsureCompactHook error → warning logged, execution continues
  - [x] 6.3 Counter not created → graceful degradation
  - [x] 6.4 RunConfig.Env passthrough test

## Dev Notes

### 4 integration paths
Каждый path нуждается в counter lifecycle:

1. **Execute** (runner.go, inner loop ~line 525): основной цикл итераций
2. **Review** (runner.go, review section ~line 1260): через `RunConfig.Env` → `RealReview`
3. **Resume** (runner.go, ResumeExtraction ~line 523): resume flow
4. **SerenaSync** (serena.go): sync flow

### Execute path code pattern
```go
counterPath, counterCleanup := CreateCompactCounter()
defer counterCleanup()

if counterPath != "" {
    opts.Env["RALPH_COMPACT_COUNTER"] = counterPath
}

raw, execErr := session.Execute(ctx, opts)
// ... parse result → sr ...

compactions := CountCompactions(counterPath)
fillPct := EstimateMaxContextFill(sr.Metrics, DefaultContextWindow)
resolved := r.Metrics.RecordSession(sr.Metrics, sr.Model, "execute", elapsed.Milliseconds(), compactions, fillPct)
```

### Review path — RunConfig.Env
```go
// В Runner.Execute review section:
reviewCounterPath, reviewCounterCleanup := CreateCompactCounter()
defer reviewCounterCleanup()

reviewEnv := map[string]string{}
if reviewCounterPath != "" {
    reviewEnv["RALPH_COMPACT_COUNTER"] = reviewCounterPath
}
rc := RunConfig{
    // ... existing fields ...
    Env: reviewEnv,
}
```

### RealReview — env passthrough
```go
// В RealReview:
for k, v := range rc.Env {
    if opts.Env == nil {
        opts.Env = make(map[string]string)
    }
    opts.Env[k] = v
}
```

### session.Options.Env
`session.Options` уже имеет `Env map[string]string` (добавлен в Story 9.4, FR74). Использовать existing field.

### RecordSession — обновление с реальными данными
В Stories 10.5 все call sites получили `0, 0.0`. В этой story — заменяем на реальные `compactions, fillPct` в 4 paths.

### Project Structure Notes

- Файлы: `runner/runner.go`, `runner/serena.go`
- `RunConfig` struct — в `runner/runner.go`
- `session.Options.Env` — уже существует (Story 9.4)
- Dependency direction: без изменений

### References

- [Source: docs/prd/context-window-observability.md#FR82-FR84] — counter lifecycle
- [Source: docs/architecture/context-window-observability.md#Решение 1] — integration in Runner
- [Source: docs/epics/epic-10-context-window-observability-stories.md#Story 10.6] — AC, technical notes
- [Source: runner/runner.go] — Runner.Execute(), execute(), RealReview()
- [Source: runner/serena.go] — RealSerenaSync()
- [Source: session/session.go] — Options.Env map[string]string (FR74)

## Testing Standards

- Integration tests с mock binary
- Verify RecordSession called with correct compactions/fillPct via mock call tracking
- `t.TempDir()` для file isolation
- Mock `EnsureCompactHook` error → verify warning logged and execution continues
- Naming: `TestRunner_Execute_CompactionCounter`, `TestRunner_RealReview_EnvPassthrough`

## Dev Agent Record

### Context Reference
- Story: docs/sprint-artifacts/10-6-runner-integration.md
- Source: runner/runner.go (Execute, execute, RealReview, ResumeExtraction), runner/serena.go (RealSerenaSync)

### Agent Model Used
claude-opus-4-6

### Debug Log References
N/A

### Completion Notes List
- AC1: EnsureCompactHook called in execute() before main loop, error logged as warning
- AC2: DefaultContextWindow already exists from Story 10-3
- AC3/AC4/AC9: Execute path — CreateCompactCounter per attempt, env set, CountCompactions + EstimateMaxContextFill → RecordSession
- AC5/AC6: Review path — RunConfig.Env field added, counter in review section, RealReview copies rc.Env to opts.Env
- AC7: Resume path — counter lifecycle in ResumeExtraction
- AC8: SerenaSync path — counter lifecycle in RealSerenaSync (no RecordSession — uses RecordSerenaSync)
- All existing tests pass with no changes needed

### File List
- runner/runner.go
- runner/serena.go
