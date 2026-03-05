# Epics Structure Plan

### Party Mode Decisions Applied

1. **Epic 1 restructured:** CLI wiring идёт ПОСЛЕ walking skeleton (Winston: skeleton = architecture validation, CLI = cosmetics)
2. **Prompt assembly → Epic 1** (Winston: cross-cutting utility, нужна всем эпикам)
3. **Emergency gates (FR23/FR24) → Epic 3** (John: safety mechanism, не feature. Minimal emergency stop)
4. **Epic 5 = post-initial release** (John: gates default=off, первый релиз возможен после Epic 4)
5. **Interface-first для knowledge** (Bob: Epic 3 resume создаёт hook/interface, Epic 6 реализует)
6. **Serena explicit dependency** (Winston: Epic 6 Serena story модифицирует runner.Run() из Epic 3)
7. **Final integration test** (Bob: после Epic 6, полный flow bridge → run → review → knowledge)

### Epic Overview

| # | Epic | User Value | FRs | Est. Stories | Release |
|:-:|------|-----------|-----|:------------:|:-------:|
| 1 | Foundation & Project Infrastructure | Config, session, prompt assembly, test infra, walking skeleton, `ralph --help` | FR30-FR35 | 12-13 | — |
| 2 | Story-to-Tasks Bridge | `ralph bridge stories/auth.md` → sprint-tasks.md | FR1-FR5a | 7-8 | — |
| 3 | Core Execution Loop | `ralph run` автономно выполняет задачи + emergency stop при застревании | FR6-FR12, FR23, FR24, FR36-FR38 | 11-13 | — |
| 4 | Code Review Pipeline | Ревью после каждой задачи, 4 sub-агента, findings→fix цикл | FR13-FR18a | 8-10 | **v0.1** |
| 5 | Human Gates & Control | `--gates`: approve/retry/skip/quit, checkpoints, feedback injection | FR20-FR22, FR25 | 5-6 | **v0.2** |
| 6 | Knowledge Management & Polish | LEARNINGS, CLAUDE.md, distillation, Serena, final integration | FR26-FR29, FR39 | 8-9 | **v0.3** |
| 7 | Observability & Metrics | Token/cost tracking, review enrichment, stuck/budget/similarity detection, JSON report | FR42-FR56 | 10 | **v0.4** |
| | | | **57 FR** | **~61-69** | |

### Release Strategy (John PM)

- **v0.1 (Initial Release):** Epic 1-4 = bridge + run + review. Полноценный автономный workflow без interactive gates. **Manual smoke test** перед release: ralph на реальном маленьком проекте (3-5 задач)
- **v0.2:** + Epic 5 = interactive gates для power users
- **v0.4:** + Epic 7 = observability & metrics (token/cost tracking, review enrichment, JSON report)
- **v0.3:** + Epic 6 = knowledge management + Serena + final polish

### Dependencies

```
Epic 1: Foundation
  ├──→ Epic 2: Bridge (uses session, config, prompt assembly)
  ├──→ Epic 3: Execute (uses session, config, git, test infra, prompt assembly)
  │      │     includes emergency gates FR23/FR24 (safety mechanism)
  │      ├──→ Epic 4: Review (review runs after execute in loop)
  │      │      └── includes emergency gate FR24 trigger
  │      ├──→ Epic 5: Gates (interactive gates, post-initial release)
  │      └──→ Epic 6: Knowledge (implements knowledge interface from Epic 3)
  │             ├── depends on Epic 4 (review writes lessons)
  │             ├── Serena story modifies runner.Run() from Epic 3
  │             └── FINAL integration test (full flow, all epics)
```

Epic 2 и Epic 3 технически независимы (оба зависят от Epic 1), **parallel-capable** при sprint planning. Bridge логически первый — нужен sprint-tasks.md для run, но runner тестируется с hand-crafted/golden file sprint-tasks.md.

### Epic 1: Foundation & Project Infrastructure

**User Value:** Config загружается из YAML + CLI flags. Session вызывает Claude CLI. Prompt assembly работает. Test infrastructure готова. Walking skeleton проходит. `ralph --help` работает.

**PRD Coverage:** FR30 (config file), FR31 (CLI override), FR32 (custom prompts), FR33 (fallback chain), FR34 (per-agent model), FR35 (exit codes)

**Technical Context (Architecture):**
- Implementation sequence: config → session → cmd/ralph (первые в цепочке)
- Config: YAML + CLI flags + go:embed defaults cascade, 16 параметров
- Session: `os/exec.Command` + `--output-format json` + `--resume` support
- Prompt assembly: `text/template` + `strings.Replace` (двухэтапная сборка) — **cross-cutting, используется Epic 2-6**
- Cobra root command + subcommand wiring
- `signal.NotifyContext` для graceful shutdown
- `fatih/color` для цветного вывода
- Error types: sentinel errors (`ErrNoTasks`, `ErrDirtyTree`), custom types (`ExitCodeError`, `GateDecision`)
- Constants: `TaskOpen`, `TaskDone`, `GateTag`, `FeedbackPrefix` + regex patterns

**Planned Stories (~12-13):**
1. Project scaffold — directory structure + go.mod + placeholder files
2. Error types & context pattern — sentinel errors, custom types, ctx propagation
3. Config struct + YAML parsing — 5-6 базовых параметров
4. Config CLI override + go:embed defaults
5. Config remaining params + fallback chain (project → global → embedded)
6. Constants + regex patterns (sprint-tasks markers)
7. Session basic — `os/exec` + stdout capture + exit code. **CLI args через constants** (не inline strings) для устойчивости к CLI breaking changes
8. Session JSON parsing + SessionResult + golden files. CLI flag constants shared с Story 7
9. Session `--resume` support. Uses CLI flag constants
10. Prompt assembly utility — text/template + strings.Replace, golden file tests. **Interface contract freeze:** сигнатура `AssemblePrompt()` фиксируется после Epic 1
11. Test infrastructure — mock Claude + mock git in `internal/testutil/`
12. Walking skeleton — minimal config → session → execute one task (integration test, architecture proof)
13. CLI wiring — Cobra root/bridge/run, signal handling, log file, colored output, exit code mapping

> **Порядок (Winston):** Walking skeleton (12) ДО CLI wiring (13). Skeleton = architecture validation. CLI = cosmetics.

---

### Epic 2: Story-to-Tasks Bridge

**User Value:** `ralph bridge stories/auth.md` генерирует полноценный sprint-tasks.md с задачами, AC-derived тестами, `[GATE]` разметкой, служебными задачами, и `source:` трассировкой.

**PRD Coverage:** FR1 (convert stories), FR2 (test cases), FR3 (gate marking), FR4 (smart merge), FR5 (service tasks), FR5a (source traceability)

**Technical Context (Architecture):**
- `bridge.Run(ctx, cfg)` — entry point
- Bridge prompt template в `bridge/prompts/bridge.md` (text/template) — **uses prompt assembly from Epic 1**
- Shared contract: `config/shared/sprint-tasks-format.md` (go:embed, used by bridge AND runner)
- Golden file тесты: input stories → expected sprint-tasks.md
- Smart Merge — критический промпт (Architecture: "не должен сбросить `[x]`")

**Planned Stories (~7-8):**
1. Shared sprint-tasks format contract — `config/shared/sprint-tasks-format.md` + tests в обоих packages
2. Bridge prompt template — `.md` файл с text/template, golden file snapshot
3. Bridge logic — `bridge.Run()`: read stories → session.Execute → write sprint-tasks.md
4. Service tasks + gate marking + source traceability в bridge prompt
5. Bridge golden file tests — input stories → expected output
6. Smart Merge — backup + merge logic + golden file tests (FR4)
7. Bridge integration test — full bridge flow с mock Claude scenario

---

### Epic 3: Core Execution Loop

**User Value:** `ralph run` автономно выполняет задачи из sprint-tasks.md. Fresh session на каждую задачу, commit detection, retry при failure, resume-extraction, dirty state recovery. Guardrails в execute prompt. Emergency stop при застревании AI.

**PRD Coverage:** FR6 (sequential execution + git health), FR7 (fresh sessions), FR8 (execute + commit), FR9 (retry + resume-extraction), FR10 (max turns), FR11 (self-directing), FR12 (resume from incomplete), FR23 (emergency gate execute), FR24 (emergency gate review — trigger point), FR36 (999-rules), FR37 (ATDD), FR38 (zero test skip)

**Technical Context (Architecture):**
- `runner.Run(ctx, cfg)` — main loop
- `scan.go`: sprint-tasks.md scanning (`TaskOpen`, `TaskDone`, `GateTag`)
- `git.go`: `GitClient` interface + `ExecGitClient` (health check, commit detection, dirty recovery)
- Execute prompt template: 999-rules, ATDD, red-green principle, self-directing instructions
- Resume-extraction: `claude --resume <session_id>` при no-commit
- Два счётчика: `execute_attempts`, `review_cycles`
- Emergency gate: minimal stop + message при `execute_attempts >= max_iterations` (FR23) — не interactive UI, просто stop с exit code + info
- **Knowledge interface:** resume-extraction вызывает `KnowledgeWriter` interface (реализация — Epic 6). В Epic 3 = no-op implementation. **Minimal interface** (1-2 метода max), extensible в Epic 6 (добавление методов, не изменение существующих)

**Planned Stories (~11-13):**
1. Execute prompt template — 999-rules, ATDD, self-directing, red-green. Golden file snapshot
2. Scanner — `scan.go`: parse `- [ ]`, `- [x]`, `[GATE]` + мягкая валидация. Table-driven tests
3. Git client interface + ExecGitClient — health check, commit detection (HEAD comparison)
4. Git client — dirty state recovery (`git checkout -- .`) + mock git tests
5. Runner loop skeleton — scan → session.Execute → commit check → next task. Happy path only
6. Runner retry logic — execute_attempts counter, no-commit detection → resume-extraction trigger
7. Resume-extraction integration — `--resume` call, WIP commit, progress writing + `KnowledgeWriter` interface (no-op impl)
8. Runner resume on re-run (FR12) — continue from first `- [ ]`, dirty tree recovery
9. Emergency stop — execute_attempts >= max_iterations → stop with exit code 1 + informative message (FR23)
10. Emergency stop — review_cycles >= max_review_iterations → stop (FR24 trigger point, actual gate in Epic 5)
11. Runner integration test — happy path + retry + resume + emergency stop scenarios. **Stub review step** (mock returning "clean") для валидации runner↔review interface. Использует **bridge golden file output** как input (не hand-crafted sprint-tasks.md)

---

### Epic 4: Code Review Pipeline

**User Value:** Автоматическое ревью кода после каждой задачи. 4 параллельных sub-агента находят проблемы, findings верифицируются, execute автоматически исправляет. Clean review = задача выполнена. **Initial release (v0.1) после этого эпика.**

**PRD Coverage:** FR13 (review after task), FR14 (fresh session), FR15 (4 sub-agents), FR16 (verification), FR17 (review writes/clears), FR18 (findings → fix), FR18a (review cycle), FR37 (ATDD via test-coverage agent), FR38 (zero skip via test-coverage agent)

**Technical Context (Architecture):**
- Review в отдельной fresh session
- 4 sub-agent промпта: `runner/prompts/agents/{quality,implementation,simplification,test-coverage}.md`
- Review prompt запускает sub-agents через Task tool
- Findings verification: CONFIRMED / FALSE POSITIVE
- review-findings.md: транзиентный (перезапись при findings, clear при clean)
- Review пишет `[x]` при clean + очищает review-findings.md (атомарная операция)
- **FR17 scope в Epic 4:** review = `[x]` + findings write/clear. Lessons writing (LEARNINGS.md, CLAUDE.md) **deferred to Epic 6** через KnowledgeWriter. v0.1 review не пишет lessons
- Execute→review loop: `review_cycles` counter, max `max_review_iterations` (default 3)
- **Bottleneck системы** — findings quality определяет всё

**Planned Stories (~8-10):**
1. Review prompt template + golden file snapshot. **Adversarial golden files:** тест где sub-агент должен найти заведомо вставленный баг + тест где код чистый (false positive resistance)
2. Sub-agent prompts — quality.md, implementation.md, simplification.md, test-coverage.md. Golden files + **adversarial test cases** per agent. **Explicit scope boundaries** между 4 агентами (нет overlap в зоне ответственности)
3. Review session logic — launch review, capture sub-agent output
4. Findings verification — parse sub-agent results, classify CONFIRMED/FALSE POSITIVE, severity
5. Clean review handling — mark `[x]` in sprint-tasks.md + clear review-findings.md (**atomic: write both or neither**). AC: MUST NOT modify git working tree
6. Findings write — overwrite review-findings.md с structured findings (ЧТО/ГДЕ/ПОЧЕМУ/КАК)
7. Execute→review loop — integrate review into runner loop, review_cycles counter
8. Review fix cycle — execute sees findings → fix → re-review (max iterations + FR24 emergency stop)
9. Review integration test — clean review + findings + fix cycle + emergency stop scenarios

---

### Epic 5: Human Gates & Control (Post-Initial Release, v0.2)

**User Value:** `ralph run --gates` — полный интерактивный контроль процесса. Approve/retry/skip/quit на gate-точках. Checkpoint gates каждые N задач. Feedback injection. Апгрейд emergency stop до interactive gate.

**PRD Coverage:** FR20 (--gates flag), FR21 (stop at gates), FR22 (approve/retry/skip/quit + feedback), FR25 (checkpoint gates)

> **Примечание:** FR23/FR24 (emergency gates) реализованы как minimal stop в Epic 3. В Epic 5 emergency stop апгрейдится до interactive gate с approve/retry/feedback/skip/quit опциями.

**Technical Context (Architecture):**
- `gates.Prompt(ctx, gate)` — interactive stdin
- `fatih/color` для цветных опций
- `io.Reader` + `io.Writer` для тестируемого ввода/вывода (injectable, не raw `fmt.Scan`)
- Feedback injection: `> USER FEEDBACK: ...` в sprint-tasks.md (индентированная строка под задачей)
- Checkpoint gates: каждые N `[x]` задач (считает completed, не attempts)
- Runner loop вызывает `gates.Prompt` в соответствующих точках
- Апгрейд emergency stop из Epic 3 → interactive emergency gate

**Planned Stories (~5-6):**
1. Basic gate prompt — approve/skip/quit + fatih/color + tests
2. Gate detection в runner — scan `[GATE]` tag, call gates.Prompt when gates_enabled
3. Retry with feedback — feedback input → injection `> USER FEEDBACK:` в sprint-tasks.md
4. Checkpoint gates — `--every N` logic, count completed `[x]` tasks
5. Emergency gate upgrade — replace Epic 3 stop with interactive gate (approve/retry/feedback/skip/quit)
6. Gates integration test — approve + feedback + checkpoint + emergency scenarios

---

### Epic 6: Knowledge Management & Polish (v0.3)

**User Value:** Система учится и улучшается. LEARNINGS.md накапливает паттерны с автоматической дистилляцией. CLAUDE.md обновляется. Serena ускоряет работу. --always-extract для максимального обучения. **Финальный integration test всего продукта.**

**PRD Coverage:** FR26 (CLAUDE.md section), FR27 (LEARNINGS.md), FR28 (resume-extraction knowledge), FR28a (review lessons + distillation), FR28b (--always-extract), FR29 (knowledge in session context), FR39 (Serena)

**Technical Context (Architecture):**
- `knowledge.go` в runner: LEARNINGS.md append, budget check (200 lines hard limit), distillation trigger
- Distillation: отдельная `claude -p` сессия при превышении бюджета
- CLAUDE.md: обновление ТОЛЬКО секции `## Ralph operational context`
- **`KnowledgeWriter` interface** (определён в Epic 3) → реализация в `knowledge.go`
- resume-extraction вызывает `KnowledgeWriter.WriteFromResume()`
- review вызывает `KnowledgeWriter.WriteFromReview()`
- `--always-extract`: resume-extraction после каждого execute
- Serena: detect → full index (60s timeout) → incremental (10s) → graceful fallback. **Modifies `runner.Run()` from Epic 3** — adds Serena calls before execute
- Knowledge files загружаются в prompt assembly (strings.Replace)

**Planned Stories (~8-9):**
1. `KnowledgeWriter` implementation — LEARNINGS.md append + line count budget check (200 lines)
2. Distillation prompt template + golden file
3. Distillation trigger — budget exceeded → **backup LEARNINGS.md** → launch distillation session
4. CLAUDE.md section management — read/find/replace ТОЛЬКО ralph section
5. Knowledge loading в session context — include LEARNINGS.md + CLAUDE.md section в prompt assembly
6. Resume-extraction knowledge — implement `KnowledgeWriter.WriteFromResume()` (replaces no-op from Epic 3)
7. Review knowledge — implement `KnowledgeWriter.WriteFromReview()` (lessons on findings)
8. Serena integration — detect, full index, incremental index, graceful fallback. **Modifies runner.Run()**
9. --always-extract flag + **FINAL integration test** (full flow: bridge → run → review → knowledge → gates)

> **Bob's Final Integration Test (Story 9):** Полный end-to-end тест с mock Claude: bridge generates sprint-tasks.md → run executes → review → findings → fix → clean → knowledge written → Serena called. Все 6 эпиков работают вместе.

---

### Technical Context Summary

| Architectural Decision | Где применяется | Epics |
|----------------------|-----------------|:-----:|
| Cobra CLI framework | cmd/ralph wiring | 1 |
| yaml.v3 config parsing | Config package | 1 |
| fatih/color output | CLI output, gates | 1, 5 |
| os/exec + CommandContext | Session, git client | 1, 3 |
| --output-format json | Session JSON parsing | 1 |
| text/template + strings.Replace | **Prompt assembly (foundation)** | **1**, 2, 3, 4, 6 |
| go:embed per-package | Prompts, shared contracts | 1, 2, 3, 4, 6 |
| GitClient interface | Git operations | 3 |
| KnowledgeWriter interface | Knowledge hook (Epic 3 no-op → Epic 6 impl) | 3, 6 |
| Scenario-based mock Claude | Integration tests | 1, 2, 3, 4, 5, 6 |
| Golden file tests | Prompts, bridge output | 2, 3, 4, 6 |
| signal.NotifyContext | Graceful shutdown | 1 |
| sprint-tasks-format.md | Shared contract | 2, 3 |

---
