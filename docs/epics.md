# bmad-ralph - Epic Breakdown

**Author:** Степан
**Date:** 2026-02-24
**Project Level:** CLI Tool
**Target Scale:** Medium orchestration complexity

---

## Context Validation

### Loaded Documents

| Document | Status | Source |
|----------|--------|--------|
| **PRD** | Loaded | `docs/prd.md` |
| **Architecture** | Loaded | `docs/architecture.md` |
| **UX Design** | N/A | CLI-утилита, UI отсутствует |

### PRD Summary

- **42 MVP FR** в 7 категориях: Bridge (6), Execute (7), Review (7), Gates (6), Knowledge (7), Config (6), Guardrails (4)
- **4 Growth FR:** FR16a (severity filtering), FR19 (batch review), FR40 (version check), FR41 (context budget)
- **20 NFR** в 6 категориях: Performance, Security, Integration, Reliability, Portability, Maintainability
- **2 команды:** `ralph bridge` (one-shot) + `ralph run` (long-running orchestrator)
- **3 типа сессий:** execute, review, resume-extraction
- **MVP = 7 компонентов:** bridge, run loop, 4 review agents, human gates, knowledge extraction, guardrails, config

### Architecture Summary

- **Go 1.25** — single binary, zero runtime deps
- **3 external deps:** Cobra, yaml.v3, fatih/color
- **6 packages:** config, session, gates, bridge, runner, cmd/ralph
- **Implementation sequence:** config → session → gates → bridge → runner → cmd/ralph
- **Testing:** Go built-in + golden files + scenario-based mock Claude
- **56+ implementation patterns** в 7 категориях

## Overview

Данный документ содержит полную декомпозицию требований bmad-ralph из [PRD](./prd.md) на эпики и пользовательские истории, готовые к реализации.

### Epics Overview

| Epic | Name | Stories | FRs | AC Count | Release |
|:----:|------|:-------:|:---:|:--------:|:-------:|
| 1 | Foundation & Project Infrastructure | 13 | Config, Session, CLI | ~58 | — |
| 2 | Story-to-Tasks Bridge | 7 | FR1-FR5a | ~37 | — |
| 3 | Core Execution Loop | 11 | FR6-FR12, FR23, FR24, FR36-FR38 | ~66 | — |
| 4 | Code Review Pipeline | 8 | FR13-FR18a, FR37, FR38 | ~46 | v0.1 |
| 5 | Human Gates & Control | 6 | FR20-FR22, FR25, FR23/24 upgrade | ~36 | v0.2 |
| 6 | Knowledge Management & Polish | 9 | FR26-FR29, FR28a/b, FR39 | ~46 | v0.3 |
| | **Total** | **54** | **42 MVP FR** | **~289** | |

---

## Functional Requirements Inventory

### Bridge (Планирование задач)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR1** | Конвертация BMad story-файлов в структурированный sprint-tasks.md | MVP |
| **FR2** | Вывод тест-кейсов из объективных AC. Субъективные AC помечаются для ручной/LLM-as-Judge верификации (post-MVP) | MVP |
| **FR3** | Разметка точек human gate тегом `[GATE]` в sprint-tasks.md (первая задача epic'а, user-visible milestones) | MVP |
| **FR4** | Smart Merge при повторном запуске bridge с существующим sprint-tasks.md | MVP |
| **FR5** | Генерация служебных задач: project setup, integration verification, e2e checkpoint (Growth) | MVP |
| **FR5a** | Поле `source:` в каждой задаче — трассировка задача → story + AC | MVP |

### Execute (Автономное выполнение)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR6** | Последовательное выполнение задач из sprint-tasks.md. Git health check при старте (clean state, не detached HEAD) | MVP |
| **FR7** | Каждое выполнение задачи — в свежей сессии Claude Code | MVP |
| **FR8** | Execute: читает задачу, реализует код, запускает unit-тесты, коммитит при green. e2e только для UI-задач и checkpoint-ов | MVP |
| **FR9** | Retry до max iterations. Успешность по наличию git коммита. Resume-extraction при failure. Два счётчика: `execute_attempts` и `review_cycles` | MVP |
| **FR10** | Настраиваемый max turns per execute session (`--max-turns`) | MVP |
| **FR11** | Claude self-directing: читает sprint-tasks.md, берёт первую `- [ ]`. Review ставит `[x]`. Execute НЕ меняет статус задач. Ralph сканирует только для loop control | MVP |
| **FR12** | Продолжение с первой незавершённой при re-run. Dirty tree → `git checkout -- .`. Мягкая валидация формата | MVP |

### Review (Ревью кода)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR13** | Review после каждой выполненной задачи | MVP |
| **FR14** | Review в отдельной свежей сессии Claude Code | MVP |
| **FR15** | 4 параллельных sub-агента через Task tool: quality, implementation, simplification, test-coverage | MVP |
| **FR16** | Верификация findings sub-агентов: CONFIRMED / FALSE POSITIVE. Severity: CRITICAL/HIGH/MEDIUM/LOW | MVP |
| **FR16a** | Severity filtering: findings ниже порога (`review_min_severity`) в лог, не блокируют pipeline | Growth |
| **FR17** | Review ТОЛЬКО анализирует. Clean → `[x]` + clear findings. Findings → overwrite review-findings.md + lessons в LEARNINGS.md + CLAUDE.md | MVP |
| **FR18** | При findings → следующий execute адресует findings (единый тип execute-сессии) | MVP |
| **FR18a** | Повторный review для верификации фиксов (цикл execute→review, до `max_review_iterations`, default 3) | MVP |
| **FR19** | Batch review (`--review-every N`) с аннотированным diff и маппингом TASK→AC→тесты | Growth |

### Gates (Контроль качества)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR20** | Включение human gates через CLI-флаг `--gates` | MVP |
| **FR21** | Остановка на размеченных точках human gate | MVP |
| **FR22** | Approve, retry (feedback → fix-задача), skip, quit. Ralph добавляет feedback в sprint-tasks.md | MVP |
| **FR23** | Emergency human gate при исчерпании max execute attempts | MVP |
| **FR24** | Emergency human gate при превышении max review iterations | MVP |
| **FR25** | Периодические checkpoint gates каждые N задач (`--every N`) | MVP |

### Knowledge (Управление знаниями)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR26** | Операционные знания → секция `## Ralph operational context` в CLAUDE.md. Обновление через review и resume-extraction | MVP |
| **FR27** | Паттерны и выводы → LEARNINGS.md (append с hard limit 200 строк) | MVP |
| **FR28** | Resume-extraction при неудачном execute (`claude --resume`): WIP commit + progress в sprint-tasks.md + знания в LEARNINGS.md | MVP |
| **FR28a** | Review записывает lessons при findings (без отдельной extraction). Distillation при превышении бюджета LEARNINGS.md | MVP |
| **FR28b** | `--always-extract` — extraction знаний после каждого execute (не только failure) | MVP |
| **FR29** | Knowledge files загружаются в контекст каждой новой сессии | MVP |

### Config (Конфигурация и кастомизация)

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR30** | Config файл `.ralph/config.yaml` в корне проекта (16 параметров) | MVP |
| **FR31** | CLI flags override config file override embedded defaults | MVP |
| **FR32** | Кастомизация промптов review-агентов через `.md` файлы | MVP |
| **FR33** | Fallback chain: `.ralph/agents/` (project) → `~/.config/ralph/agents/` (global) → embedded defaults | MVP |
| **FR34** | Per-agent model configuration (execute: opus, review agents: sonnet/haiku) | MVP |
| **FR35** | Информативные exit codes (0-4) для интеграции со скриптами | MVP |

### Guardrails и ATDD

| FR | Описание | Приоритет |
|----|----------|-----------|
| **FR36** | 999-series guardrail-правила в execute-промпте. Последний барьер: даже при опасном finding → execute откажется | MVP |
| **FR37** | ATDD enforcement: каждый AC покрыт тестом | MVP |
| **FR38** | Zero test skip: unit на каждый execute, e2e на UI-задачах и checkpoint-ах. Падения исправляются или эскалируются | MVP |
| **FR39** | Serena MCP detection + integration (best effort). Full index при старте, incremental перед execute, graceful fallback | MVP |
| **FR40** | CLI version check + session adapter для multi-LLM | Growth |
| **FR41** | Context budget calculator: подсчёт размера контекста перед сессией, warning при >40% context window | Growth |

### Итого

| Категория | MVP | Growth | Всего |
|-----------|:---:|:------:|:-----:|
| Bridge | 6 | 0 | 6 |
| Execute | 7 | 0 | 7 |
| Review | 7 | 2 | 9 |
| Gates | 6 | 0 | 6 |
| Knowledge | 6 | 0 | 6 |
| Config | 6 | 0 | 6 |
| Guardrails/ATDD | 4 | 2 | 6 |
| **Всего** | **42** | **4** | **46** |

> **Примечание (Party Mode):** FR28b (`--always-extract`) и FR39 (Serena) — MVP, но nice-to-have внутри MVP. Планируются в последнем эпике чтобы не блокировать core flow.

---

## Story Sizing Guidelines (Pre-mortem)

Правила декомпозиции stories, выведенные из pre-mortem анализа. Каждая story должна быть выполнима **одним dev agent за одну сессию** (~25-35 turns).

### Target Story Size

| Параметр | Target | Max |
|----------|--------|-----|
| Production файлов | 1-2 | 3 |
| Acceptance criteria | 3-5 | 7 |
| Dev agent turns | ~25 | 35 |

Тесты пишутся **внутри** story, не отдельной story.

### Анти-паттерны декомпозиции

| Анти-паттерн | Правило |
|-------------|---------|
| "Весь package = одна story" | Max 1-3 файла, 3-7 AC |
| "Промпт + логика вместе" | Промпт-файлы = отдельная story от бизнес-логики |
| "Happy path + все edge cases" | Happy path first, edge cases = follow-up story |
| "Test infra + business logic" | Mock/testutil инфраструктура = foundation story |
| "N sub-components = 1 story" | Группировка по 3-5 элементов max |

### Structural Rules (Pre-mortem #2)

| # | Правило | Обоснование |
|---|---------|-------------|
| 1 | **Walking skeleton** в Epic 1 — минимальный e2e pass (config → session → execute one task) | Ранняя валидация integration path |
| 2 | **Project scaffold** story — directory structure + placeholder файлы из Architecture | Устраняет naming/path mismatches между stories |
| 3 | **Test infrastructure** story — mock Claude + mock git ДО бизнес-логики | Разблокирует все тестовые stories |
| 4 | **Промпты = first-class stories** с golden file AC, ДО logic stories | Промпты готовы когда логика их embed-ит |
| 5 | **Cross-cutting foundation** — error types, ctx pattern, exit code types | Единообразие error handling с первого дня |
| 6 | **Integration story** в конце каждого эпика | End-to-end валидация, не только unit |
| 7 | **Explicit prerequisites** — "Requires: Story X.Y", не "previous story" | Чёткий dependency graph |
| 8 | **Shared contracts** (sprint-tasks format) = отдельная story с tests в обоих packages | Предотвращает format divergence между bridge и runner |

### Анти-паттерны декомпозиции (дополнение)

| Анти-паттерн | Правило |
|-------------|---------|
| "Промпты = afterthought" | Промпт stories = first-class, с AC на golden file, ДО logic |
| "Dependencies implicit" | Каждая story: explicit "Requires: Story N.M" |
| "Cross-cutting ничейные" | Foundation story для error types, ctx, exit codes |
| "Всё с mock, нет integration" | Integration story в конце каждого эпика |
| "Scaffold по ходу дела" | Project scaffold = одна из первых stories |
| "Golden file = достаточно для промптов" | Scenario-based integration tests для КАЖДОГО типа промпта: mock Claude output → проверка контракта промпт↔парсер |

### Risk Heatmap по FR-кластерам (Failure Mode Analysis)

| Кластер | Risk | Ключевая угроза |
|---------|------|-----------------|
| **Review (FR13-FR18a)** | HIGHEST | False positive findings → бесконечный fix-цикл; state inconsistency при `[x]` + clear findings |
| **Execute (FR6-FR12)** | HIGH | Каскад: bad JSON → no session_id → no resume → lost progress |
| **Knowledge (FR26-FR29)** | HIGH | Перезапись CLAUDE.md вне ralph-секции; distillation убивает ценные паттерны |
| **Bridge (FR1-FR5a)** | MEDIUM | Smart Merge (FR4) может сбросить `[x]` → потеря прогресса |
| **Gates (FR20-FR25)** | MEDIUM | Checkpoint gate (FR25) interaction с review loop не специфицирован |
| **Guardrails (FR36-FR39)** | MEDIUM | 999-rules не тестируемы unit-тестами, только prompt quality |
| **Config (FR30-FR35)** | LOW | Straightforward, низкий risk |

### Mandatory AC для high-risk stories

| Область | Обязательные AC в stories |
|---------|--------------------------|
| **Review stories** | Формат findings (ЧТО/ГДЕ/ПОЧЕМУ/КАК); идемпотентность `[x]` + clear; isolation (no code writes) |
| **Session JSON parsing** | Golden files на edge cases (truncated JSON, unexpected fields, empty output) |
| **FR25 checkpoint gates** | Считает только `[x]` задачи, не execute attempts |
| **FR4 Smart Merge** | Backup sprint-tasks.md перед merge; golden file тесты merge-сценариев |
| **FR9 resume-extraction** | session_id capture + fallback при parse error |
| **FR26 CLAUDE.md** | Обновляет ТОЛЬКО секцию `## Ralph operational context`, не затрагивает остальное |
| **Prompt assembly** | Interface contract freeze: сигнатура `AssemblePrompt()` фиксируется после Epic 1, только internal changes |
| **Epic 3 integration** | Stub review step (mock "clean") + bridge golden file output как input runner'а |
| **Review prompts** | Adversarial golden files: bug-injection test + clean-code false positive resistance test per sub-agent |
| **Mutation Asymmetry** | Execute stories: AC "MUST NOT modify sprint-tasks.md". Review stories: AC "MUST NOT modify git working tree" |
| **Review atomicity** | `[x]` + clear findings = atomic operation (write both or neither). Epic 4 Story 5 |
| **Distillation backup** | Backup LEARNINGS.md before distillation overwrite. Epic 6 Story 3 |

### Epic Size Threshold Rule (Comparative Analysis)

> **Threshold:** Если Epic 3 при детальной декомпозиции (Step 2) превысит 13 stories → split:
> - **Epic 3a "Task Execution":** Scanner + Git client + Runner loop + Execute prompt + Retry (Stories 1-6). User value: "задачи выполняются"
> - **Epic 3b "Failure Recovery":** Resume-extraction + Emergency stops + Runner resume-on-rerun + Integration test (Stories 7-11). User value: "сбои обрабатываются"
> - Dependencies при split: Epic 4 → Epic 3a (runner loop). Epic 5 → Epic 3b (emergency gate upgrade). Epic 6 → Epic 3b (KnowledgeWriter)
>
> Текущая 6-epic структура подтверждена как оптимальная (score 65/75 vs альтернатив 43-60).

### Estimated Story Count

~45-55 stories для 42 MVP FR (вместо наивных ~30).

---

## FR Coverage Map

| FR | Epic | Stories (planned) |
|----|:----:|-------------------|
| FR1 | 2 | Bridge prompt, Bridge logic |
| FR2 | 2 | Bridge prompt (AC-derived tests) |
| FR3 | 2 | Bridge prompt (gate marking) |
| FR4 | 2 | Smart Merge |
| FR5 | 2 | Bridge prompt (service tasks) |
| FR5a | 2 | Bridge prompt (source traceability) |
| FR6 | 3 | Git client (health check), Runner loop |
| FR7 | 3 | Runner loop (fresh session) |
| FR8 | 3 | Execute prompt, Runner loop (commit detection) |
| FR9 | 3 | Retry logic, Resume-extraction |
| FR10 | 3 | Config (max_turns), Session (--max-turns flag) |
| FR11 | 3 | Execute prompt (self-directing), Scanner |
| FR12 | 3 | Runner loop (resume), Git client (dirty recovery) |
| FR13 | 4 | Review integration в runner loop |
| FR14 | 4 | Review session (fresh session) |
| FR15 | 4 | Review prompt + 4 sub-agent prompts |
| FR16 | 4 | Findings verification |
| FR17 | 4+6 | Clean review / Findings write (Epic 4). Lessons → LEARNINGS.md + CLAUDE.md deferred to Epic 6 via KnowledgeWriter |
| FR18 | 4 | Execute→review loop в runner |
| FR18a | 4 | Review cycle counter в runner |
| FR20 | 5 | Basic gate prompt |
| FR21 | 5 | Gate detection в runner loop |
| FR22 | 5 | Retry with feedback |
| FR23 | 3 | Emergency gate (execute attempts) — safety mechanism |
| FR24 | 3+4 | Emergency gate (review iterations) — safety mechanism |
| FR25 | 5 | Checkpoint gates |
| FR26 | 6 | CLAUDE.md section management |
| FR27 | 6 | LEARNINGS.md append + budget |
| FR28 | 6 | Resume-extraction knowledge writing |
| FR28a | 6 | Review lessons writing + distillation |
| FR28b | 6 | --always-extract |
| FR29 | 6 | Knowledge loading в session context |
| FR30 | 1 | Config struct + YAML parsing |
| FR31 | 1 | CLI flags override |
| FR32 | 1 | Config fallback chain (agent files) |
| FR33 | 1 | Fallback chain (project → global → embedded) |
| FR34 | 1 | Per-agent model config |
| FR35 | 1 | Exit code types + mapping |
| FR36 | 3 | Execute prompt (999-rules) |
| FR37 | 3+4 | Execute prompt (ATDD) + test-coverage agent |
| FR38 | 3+4 | Execute prompt (zero skip) + test-coverage agent |
| FR39 | 6 | Serena integration |

---

## Epics Structure Plan

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
| | | | **42 MVP FR** | **~51-59** | |

### Release Strategy (John PM)

- **v0.1 (Initial Release):** Epic 1-4 = bridge + run + review. Полноценный автономный workflow без interactive gates. **Manual smoke test** перед release: ralph на реальном маленьком проекте (3-5 задач)
- **v0.2:** + Epic 5 = interactive gates для power users
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

## Epic 1: Foundation & Project Infrastructure — Stories

**Epic Goal:** Config загружается из YAML + CLI flags. Session вызывает Claude CLI. Prompt assembly работает. Test infrastructure готова. Walking skeleton проходит. `ralph --help` работает.

**PRD Coverage:** FR30, FR31, FR32, FR33, FR34, FR35
**Stories:** 13 | **Estimated AC:** ~65

---

### Story 1.1: Project Scaffold

**As a** developer,
**I want** the project directory structure, go.mod, and placeholder files created according to Architecture,
**So that** all subsequent stories have a consistent foundation with correct paths and naming.

**Acceptance Criteria:**

```gherkin
Given the project is being initialized
When the scaffold is created
Then the following directory structure exists:
  | Path                              | Type      |
  | cmd/ralph/                        | directory |
  | bridge/                           | directory |
  | bridge/prompts/                   | directory |
  | bridge/testdata/                  | directory |
  | runner/                           | directory |
  | runner/prompts/                   | directory |
  | runner/prompts/agents/            | directory |
  | runner/testdata/                  | directory |
  | session/                          | directory |
  | gates/                            | directory |
  | config/                           | directory |
  | config/shared/                    | directory |
  | config/testdata/                  | directory |
  | internal/testutil/                | directory |
  | internal/testutil/scenarios/      | directory |

And go.mod exists with module path and Go 1.25
And go.sum exists (after go mod tidy with cobra + yaml.v3 + fatih/color)
And Makefile exists with targets: build, test, lint
And .gitignore includes: binary, .ralph/, *.log
And .golangci.yml exists with basic linter config
And .github/workflows/ci.yml exists with: go test ./..., golangci-lint, build check
And .goreleaser.yml exists with basic Go binary release config (ldflags for version)
And cmd/ralph/main.go contains package main with empty main()
And each package directory has a placeholder .go file with correct package declaration
```

**Technical Notes:**
- Architecture: "Go 1.25 в go.mod"
- 3 external deps: `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `github.com/fatih/color`
- Makefile: `make build` → `go build ./cmd/ralph`, `make test` → `go test ./...`, `make lint` → `golangci-lint run`
- Placeholder files = minimal valid Go file (`package X`) to enable `go build ./...`

**Prerequisites:** None (first story)

---

### Story 1.2: Error Types & Context Pattern

**As a** developer,
**I want** shared error types, sentinel errors, and context propagation pattern established,
**So that** all packages use consistent error handling from day one.

**Acceptance Criteria:**

```gherkin
Given the error types package needs to be established
When error types are defined in config package
Then the following sentinel errors exist:
  | Error          | Message                  | Package |
  | ErrNoTasks     | "no tasks found"         | config  |
  | ErrMaxRetries  | "max retries exceeded"   | config  |
  Note: ErrDirtyTree and ErrDetachedHead defined in runner package (Story 3.3)
  Note: ErrMaxReviewCycles defined in runner package (Story 3.10)

And custom error type ExitCodeError exists with fields:
  | Field    | Type   |
  | Code     | int    |
  | Message  | string |
And ExitCodeError implements error interface
And ExitCodeError is checkable via errors.As

And custom error type GateDecision exists with fields:
  | Field    | Type   |
  | Action   | string |
  | Feedback | string |
And GateDecision implements error interface

And error wrapping follows pattern "package: operation: %w"
And all errors are testable via errors.Is or errors.As
And no panic() exists in any production code path

And context pattern is documented:
  - main() creates root ctx via signal.NotifyContext
  - All functions accept ctx as first parameter
  - No context.TODO() in production code
```

**Technical Notes:**
- Architecture: sentinel errors в config package scope, custom types для branch logic
- `errors.Is` / `errors.As` always, never string matching
- Exit code mapping (0-4) only in `cmd/ralph/` — packages don't know about exit codes
- Pattern: `fmt.Errorf("runner: execute task %s: %w", id, err)`

**Prerequisites:** Story 1.1 (**parallel-capable** с Story 1.6 и Story 1.10 — все depend only on 1.1)

---

### Story 1.3: Config Struct + YAML Parsing

**As a** developer,
**I want** config loaded from `.ralph/config.yaml` with strongly-typed struct,
**So that** all parameters are available as typed Go values. (FR30)

**Acceptance Criteria:**

```gherkin
Given a .ralph/config.yaml file exists in the project root
When config.Load() is called
Then Config struct is populated with all 16 parameters:
  | Parameter             | Type   | Default            |
  | claude_command        | string | "claude"           |
  | max_turns             | int    | 50                 |
  | max_iterations        | int    | 3                  |
  | max_review_iterations | int    | 3                  |
  | gates_enabled         | bool   | false              |
  | gates_checkpoint      | int    | 0                  |
  | review_every          | int    | 1                  |
  | model_execute         | string | ""                 |
  | model_review          | string | ""                 |
  | review_min_severity   | string | "LOW"              |
  | always_extract        | bool   | false              |
  | serena_enabled        | bool   | true               |
  | serena_timeout        | int    | 10                 |
  | learnings_budget      | int    | 200                |
  | log_dir               | string | ".ralph/logs"      |
  | project_root          | string | (auto-detected)    |

And YAML field tags MUST match PRD config table names exactly (snake_case):
  e.g. MaxTurns → yaml:"max_turns", NOT yaml:"maxTurns"

And missing config file results in all defaults applied (no error)
And malformed YAML returns descriptive error with line number
And unknown fields are silently ignored (forward compatibility)
And Config struct is immutable after Load() (passed by pointer, never mutated)

And project root auto-detection follows priority:
  - Walk up from CWD looking for .ralph/ directory
  - If .ralph/ not found, fall back to .git/ directory
  - If neither found, use CWD with warning
  - .ralph/ takes priority over .git/ if both exist at different levels

And table-driven tests cover:
  - Valid config with all fields
  - Partial config (missing fields → defaults)
  - Empty file (all defaults)
  - Malformed YAML (error case)
  - Unknown fields (ignored)
  - Project root: .ralph/ found
  - Project root: only .git/ found (fallback)
  - Project root: neither found (CWD + warning)
```

**Technical Notes:**
- Architecture: "yaml.v3 для YAML config, без Viper"
- Architecture: "config.Config парсится один раз при старте и дальше read-only"
- `config.Load(flags CLIFlags) (*Config, error)` — entry point
- Project root auto-detection: walk up from CWD looking for `.ralph/` or `.git/`
- Golden file: `config/testdata/TestConfig_Default.golden`

**Prerequisites:** Story 1.1, Story 1.2

---

### Story 1.4: Config CLI Override + go:embed Defaults

**As a** developer,
**I want** CLI flags to override config file values, with embedded defaults as fallback,
**So that** the three-level cascade works: CLI > config file > embedded defaults. (FR31)

**Acceptance Criteria:**

```gherkin
Given embedded defaults exist via go:embed
And a config file exists with some overrides
And CLI flags are provided for some parameters
When config.Load(flags) is called
Then CLI flags take highest priority
And config file values override embedded defaults
And embedded defaults fill remaining gaps

Given CLI flag --max-turns=100 is set
And config file has max_turns: 50
When config is loaded
Then config.MaxTurns == 100

Given CLI flag --max-turns is NOT set
And config file has max_turns: 50
When config is loaded
Then config.MaxTurns == 50

Given CLI flag --max-turns is NOT set
And config file does NOT have max_turns
When config is loaded
Then config.MaxTurns == 50 (embedded default)

And go:embed defaults are defined in config package as embedded YAML
And CLIFlags uses pointer fields for "was set" tracking:
  *int for numeric flags, *string for string flags, *bool for boolean flags
  nil = flag not set (use config/default), non-nil = flag explicitly set
And table-driven tests cover all three cascade levels for at least 3 parameters
```

**Technical Notes:**
- Architecture: "CLI flags override config file override embedded defaults"
- go:embed: `//go:embed defaults.yaml` в config package
- CLIFlags needs "was this flag set?" tracking (zero value vs explicitly set). Use pointer fields or separate bool flags
- Architecture: "Config immutability — парсится один раз при старте"

**Prerequisites:** Story 1.3

---

### Story 1.5: Config Remaining Params + Fallback Chain

**As a** developer,
**I want** custom prompt file fallback chain (project → global → embedded) working,
**So that** users can customize agent prompts at project or global level. (FR32, FR33)

**Acceptance Criteria:**

```gherkin
Given custom prompt file exists at .ralph/agents/quality.md (project level)
When prompt fallback chain resolves "quality.md"
Then project-level file is returned

Given no project-level file exists
And custom prompt file exists at ~/.config/ralph/agents/quality.md (global level)
When prompt fallback chain resolves "quality.md"
Then global-level file is returned

Given neither project nor global file exists
When prompt fallback chain resolves "quality.md"
Then embedded default is returned (go:embed)

And config.ResolvePath(name string) returns resolved file path or embedded content
And per-agent model configuration is supported (FR34):
  | Config Key     | Description                     |
  | model_execute  | Model for execute sessions      |
  | model_review   | Model for review sub-agents     |
And model config maps to --model flag in session invocation

And table-driven tests cover:
  - Project-level override found
  - Global-level fallback
  - Embedded default fallback
  - Per-agent model resolution
```

**Technical Notes:**
- Architecture: "Fallback chain: project → global → embedded"
- Architecture: `.ralph/agents/` (project) → `~/.config/ralph/agents/` (global) → embedded
- `config.ResolvePath(name)` returns `(content []byte, source string, error)`
- Per-agent model: FR34 — execute uses `model_execute`, review uses `model_review`

**Prerequisites:** Story 1.4

---

### Story 1.6: Constants & Regex Patterns

**As a** developer,
**I want** all sprint-tasks.md markers and regex patterns defined as constants,
**So that** bridge, runner, and scanner use identical patterns without duplication.

**Acceptance Criteria:**

```gherkin
Given constants need to be defined for sprint-tasks.md parsing
When constants.go is created in config package
Then the following string constants exist:
  | Constant       | Value              |
  | TaskOpen       | "- [ ]"            |
  | TaskDone       | "- [x]"            |
  | GateTag        | "[GATE]"           |
  | FeedbackPrefix | "> USER FEEDBACK:" |

And the following compiled regex patterns exist:
  | Pattern          | Matches                        |
  | TaskOpenRegex    | Lines starting with "- [ ]"    |
  | TaskDoneRegex    | Lines starting with "- [x]"    |
  | GateTagRegex     | Lines containing "[GATE]"      |

And regex patterns are compiled via regexp.MustCompile at package scope
And all patterns have unit tests with positive and negative cases
And edge cases tested: indented tasks, tasks with trailing content, empty lines
```

**Technical Notes:**
- Architecture: "String constants для маркеров sprint-tasks.md"
- Architecture: "Regex patterns для scan тоже как var с regexp.MustCompile в package scope"
- These constants are imported by both `bridge` and `runner` packages
- `config/constants.go` — separate file for clarity

**Prerequisites:** Story 1.1

---

### Story 1.7: Session Basic — os/exec + stdout Capture

**As a** developer,
**I want** a session package that invokes Claude CLI and captures output,
**So that** all Claude interactions go through a single abstraction.

**Acceptance Criteria:**

```gherkin
Given session.Execute(ctx, opts) is called with valid options
When Claude CLI is invoked
Then os/exec.CommandContext is used (never exec.Command)
And cmd.Dir is set to config.ProjectRoot
And environment inherits os.Environ()
And stdout and stderr captured via SEPARATE buffers:
  cmd.Stdout = &stdoutBuf, cmd.Stderr = &stderrBuf
  NEVER use CombinedOutput() (JSON parsing breaks from mixed stderr)
And exit code is extracted from exec.ExitError
And error is wrapped as "session: claude: exit %d: %w"

And SessionOptions struct contains:
  | Field      | Type   | Description                   |
  | Prompt     | string | Assembled prompt content       |
  | MaxTurns   | int    | --max-turns flag value         |
  | Model      | string | --model flag (optional)        |
  | OutputJSON | bool   | --output-format json           |
  | Resume     | string | --resume session_id (optional) |
  | DangerouslySkipPermissions | bool | Always true for MVP |

And CLI args are constructed via constants (not inline strings):
  | Constant              | Value                        |
  | flagPrompt            | "-p"                         |
  | flagMaxTurns          | "--max-turns"                |
  | flagModel             | "--model"                    |
  | flagOutputFormat      | "--output-format"            |
  | flagResume            | "--resume"                   |
  | flagSkipPermissions   | "--dangerously-skip-permissions" |

And unit tests verify:
  - Correct CLI args construction for various option combinations
  - Exit code extraction
  - Error wrapping format
  - Context cancellation propagated to subprocess
```

**Technical Notes:**
- Architecture: "os/exec.Command через session package — единая точка вызова"
- Architecture: "CLI args через constants для устойчивости к CLI breaking changes" (Chaos Monkey)
- Architecture: "Все subprocess через exec.CommandContext(ctx)"
- Flag constants defined at top of `session/session.go` (or separate `session/flags.go`)
- Session does NOT parse JSON yet — that's Story 1.8
- `config.ClaudeCommand` allows mock substitution in tests
- Cancellation test approach: mock Claude with `sleep`, `time.AfterFunc` cancels ctx, verify process killed (Amelia)

**Prerequisites:** Story 1.2, Story 1.3

---

### Story 1.8: Session JSON Parsing + SessionResult

**As a** developer,
**I want** Claude CLI JSON output parsed into a structured SessionResult,
**So that** session_id, exit code, and output are reliably extracted.

**Acceptance Criteria:**

```gherkin
Given Claude CLI returns JSON output (--output-format json)
When session output is parsed
Then SessionResult struct is populated:
  | Field      | Type   | Source              |
  | SessionID  | string | JSON field          |
  | ExitCode   | int    | Process exit code   |
  | Output     | string | Parsed from JSON    |
  | Duration   | time.Duration | Measured     |

And golden file tests cover:
  - Normal successful response
  - Response with warnings in stderr
  - Truncated JSON (partial output)
  - Unexpected JSON fields (ignored, no error)
  - Empty JSON output (error with descriptive message)
  - Non-JSON output (fallback: raw stdout as Output, empty SessionID)

Note: golden file JSON structures are best-guess from --output-format json docs.
MUST be verified against real Claude CLI output before v0.1 smoke test.

And SessionResult.HasCommit field is NOT in session package
  (commit detection is GitClient responsibility in runner)

And scenario-based integration test contracts are validated:
  - mock Claude returns predefined JSON → parser handles correctly
```

**Technical Notes:**
- Architecture: "`--output-format json` для structured parsing"
- Architecture: "Golden file с example JSON response. Парсинг session_id, result, exit_code"
- Mandatory AC (session JSON parsing): "golden files на edge cases (truncated JSON, unexpected fields, empty output)"
- `session/result.go` — SessionResult struct definition

**Prerequisites:** Story 1.7

---

### Story 1.9: Session --resume Support

**As a** developer,
**I want** session to support `--resume <session_id>` for resume-extraction,
**So that** failed execute sessions can be resumed to extract progress and knowledge.

**Acceptance Criteria:**

```gherkin
Given a previous session with SessionID "abc-123"
When session.Execute(ctx, opts) is called with opts.Resume = "abc-123"
Then CLI args include "--resume" "abc-123"
And "-p" flag is NOT included (resume uses previous prompt)
And "--max-turns" IS included (limits resume duration)
And "--output-format json" IS included

Given opts.Resume is empty string
When CLI args are constructed
Then "--resume" flag is NOT included
And "-p" flag IS included with opts.Prompt

And unit tests verify:
  - Resume args construction (no -p, has --resume)
  - Normal args construction (has -p, no --resume)
  - Resume with max-turns combination
```

**Technical Notes:**
- Architecture: "`--resume` используется для resume-extraction"
- Architecture: "Resume-extraction через `claude --resume` при неуспешном execute"
- Uses CLI flag constants from Story 1.7

**Prerequisites:** Story 1.7, Story 1.8

---

### Story 1.10: Prompt Assembly Utility

**As a** developer,
**I want** a two-stage prompt assembly utility (text/template + strings.Replace),
**So that** all epics use a single mechanism for building Claude prompts. **Interface contract freeze after this story.**

**Acceptance Criteria:**

```gherkin
Given a prompt template with {{.Variable}} placeholders
And user content with potential {{ characters
When AssemblePrompt(template, data, replacements) is called
Then Stage 1: text/template processes structural placeholders
And Stage 2: strings.Replace injects user content safely

Given user content contains "{{.Malicious}}" text
When injected via Stage 2 (strings.Replace)
Then template engine does NOT process it (security: no injection)

And function signature is:
  AssemblePrompt(tmplContent string, data TemplateData, replacements map[string]string) (string, error)

And TemplateData struct supports:
  | Field           | Type   | Used by        |
  | SerenaEnabled   | bool   | Execute prompt |
  | GatesEnabled    | bool   | Execute prompt |
  | TaskContent     | string | Stage 2 replace |
  | LearningsContent| string | Stage 2 replace |
  | ClaudeMdContent | string | Stage 2 replace |
  | FindingsContent | string | Stage 2 replace |

And golden file tests verify:
  - Simple template with one variable
  - Template with conditional ({{if .SerenaEnabled}})
  - User content with {{ characters (no injection)
  - Empty replacements (Stage 2 is no-op)
  - Invalid template syntax (descriptive error)

And Stage 2 replacements applied in deterministic order (sorted by key)
And replacements MUST NOT contain other replacement placeholders (flat, not recursive)
And this function lives in config package (cross-cutting utility)
And prompt assembly MUST NOT import other project packages (Winston: leaf constraint)
  If this constraint is violated in future → extract to internal/prompt/ package
And interface contract: AssemblePrompt() signature is FROZEN after this story
```

**Technical Notes:**
- Architecture: "`text/template` + двухэтапная сборка. Этап 1: structure. Этап 2: strings.Replace для user content"
- Architecture: "Двухэтапность защищает от {{ в user-контролируемых файлах"
- Reverse Engineering AC: "interface contract freeze — сигнатура не меняется после Epic 1"
- Located in `config/` package — used by bridge, runner, and review (cross-cutting)

**Prerequisites:** Story 1.1

---

### Story 1.11: Test Infrastructure — Mock Claude + Mock Git

**As a** developer,
**I want** scenario-based mock Claude and mock git infrastructure,
**So that** all subsequent epics can write integration tests without real Claude/git calls.

**Acceptance Criteria:**

```gherkin
Given integration tests need mock Claude CLI
When mock_claude.go is created in internal/testutil/
Then mock Claude reads scenario JSON file
And returns responses in order per scenario steps:
  | Field          | Type   | Description                |
  | type           | string | "execute" or "review"      |
  | exit_code      | int    | Process exit code          |
  | session_id     | string | Mock session ID            |
  | output_file    | string | File with mock output      |
  | creates_commit | bool   | Signals to mock git        |

And mock Claude is substituted via config.ClaudeCommand path
And mock Claude validates it received expected flags

And at least one example scenario JSON file exists:
  - scenarios/happy_path.json (1 execute success + 1 review clean)

And table-driven tests verify:
  - Mock Claude returns correct sequence of responses
  - Mock Claude fails on unexpected call (beyond scenario steps)
```

**Technical Notes:**
- Architecture: "Mock Claude: Go-скрипт, возвращающий предопределённые ответы по сценарию"
- Architecture: "MockGitClient implements GitClient interface"
- Architecture: "Scenario-based mock: `[{input_match, output, exit_code}, ...]`"
- Mock Claude = standalone Go binary: `internal/testutil/cmd/mock_claude/main.go` (supersedes Architecture path `testutil/mock_claude.go` — standalone binary needed for ClaudeCommand substitution)
- Compiled via `go build` in TestMain or test setup, binary path → `config.ClaudeCommand`
- `internal/testutil/scenarios/` — JSON scenario files
- **MockGitClient deferred to Epic 3** (Bob SM: interface не определён до Story 3.3, спекулятивный mock опасен)

**Prerequisites:** Story 1.7, Story 1.8

---

### Story 1.12: Walking Skeleton — Minimal End-to-End Pass

**As a** developer,
**I want** a minimal integration test proving config → session → execute one task works,
**So that** the architecture is validated before building features on top.

**Acceptance Criteria:**

```gherkin
Given a valid config with mock Claude command
And a sprint-tasks.md with one task: "- [ ] Implement hello world"
And mock Claude scenario: 1 execute (exit 0, creates commit)
And mock git: HealthCheck OK, HasNewCommit returns true
When the walking skeleton integration test runs
Then config loads successfully
And session.Execute is called with assembled prompt
And mock Claude receives correct flags (-p, --max-turns, --output-format json, --dangerously-skip-permissions)
And mock git confirms commit exists
And test passes end-to-end

And the test includes a stub review step:
  - Mock Claude scenario includes 1 review (exit 0, clean)
  - Validates runner↔review integration point exists
  (Reverse Engineering: stub review for interface validation)

And the test uses a hand-crafted sprint-tasks.md fixture matching shared format contract (Story 2.1 format)
  (Bridge golden files don't exist yet in Epic 1 — hand-crafted fixture validates architecture)
  (Story 3.11 will use real bridge golden files from Story 2.5)

And test is in runner/runner_integration_test.go (build tag: integration)
And test uses t.TempDir() for isolation
```

**Technical Notes:**
- Architecture: "Walking skeleton = architecture validation"
- Structural Rule #1: "Walking skeleton в Epic 1 — минимальный e2e pass"
- Reverse Engineering: "stub review step (mock returning clean)"
- Hand-crafted sprint-tasks.md fixture (NOT bridge golden files — bridge is Epic 2). Story 3.11 uses real bridge golden files
- This is NOT the full runner loop — just proving the pieces connect
- Build tag `//go:build integration` to separate from unit tests

**Prerequisites:** Story 1.3, Story 1.7, Story 1.8, Story 1.10, Story 1.11

---

### Story 1.13: CLI Wiring — Cobra, Signal Handling, Exit Codes

**As a** developer,
**I want** `ralph --help`, `ralph bridge`, and `ralph run` commands wired with Cobra,
**So that** the CLI is usable with proper help, flags, signal handling, and exit codes. (FR35)

**Acceptance Criteria:**

```gherkin
Given the ralph binary is built
When ralph --help is executed
Then help text shows available commands: bridge, run
And help text shows global flags
And version is displayed (var version = "dev")

When ralph bridge --help is executed
Then bridge-specific flags are documented

When ralph run --help is executed
Then run-specific flags are documented:
  | Flag           | Type   | Default | Description           |
  | --max-turns    | int    | 50      | Max turns per session |
  | --gates        | bool   | false   | Enable human gates    |
  | --every        | int    | 0       | Checkpoint gate interval |
  | --model        | string | ""      | Override model        |
  | --always-extract | bool | false   | Extract after every execute |

And signal handling is implemented:
  - signal.NotifyContext(ctx, os.Interrupt) in main()
  - First Ctrl+C propagates cancellation to all subprocesses with graceful message
  - Second Ctrl+C (NFR13): force kill via os.Exit — double signal handler pattern

And exit code mapping works:
  | Exit Code | Condition                    |
  | 0         | All tasks completed          |
  | 1         | Partial success (limits, gates off) |
  | 2         | User quit (at gate)          |
  | 3         | Interrupted (Ctrl+C)         |
  | 4         | Fatal error                  |

And log file is created at .ralph/logs/run-{timestamp}.log
And colored output uses fatih/color (auto-disabled in non-TTY)
And cmd/ralph/*.go contains ONLY wiring — no business logic
And bridge.go calls bridge.Run(ctx, cfg)
And run.go calls runner.Run(ctx, cfg)

And tests verify:
  - Exit code mapping from errors to codes
  - Flag parsing for key parameters
```

**Technical Notes:**
- Architecture: "Cobra root command + subcommand wiring"
- Architecture: "signal.NotifyContext для graceful shutdown"
- Architecture: "`fatih/color` для цветного вывода"
- Architecture: "cmd/ralph/*.go — только Cobra wiring, бизнес-логика в packages"
- Architecture: "Exit code mapping только в cmd/ralph/ — packages не знают про exit codes"
- Winston (Party Mode): "Walking skeleton ДО CLI wiring. CLI = cosmetics"
- `var version = "dev"` — overridden by goreleaser ldflags

**Prerequisites:** Story 1.2, Story 1.3, Story 1.12

---

### Epic 1 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 1.1 | Project Scaffold | — | ~15 | 8 |
| 1.2 | Error Types & Context Pattern | — | 2 | 7 |
| 1.3 | Config Struct + YAML Parsing | FR30 | 2 | 8 |
| 1.4 | Config CLI Override + go:embed | FR31 | 2 | 5 |
| 1.5 | Fallback Chain + Per-Agent Model | FR32,FR33,FR34 | 2 | 4 |
| 1.6 | Constants & Regex Patterns | — | 2 | 4 |
| 1.7 | Session Basic | — | 2 | 5 |
| 1.8 | Session JSON Parsing | — | 3 | 6 |
| 1.9 | Session --resume | — | 1 | 3 |
| 1.10 | Prompt Assembly Utility | — | 2 | 6 |
| 1.11 | Test Infrastructure (MockClaude only) | — | 3 | 3 |
| 1.12 | Walking Skeleton | — | 2 | 4 |
| 1.13 | CLI Wiring | FR35 | 4 | 7 |
| | **Total** | **FR30-FR35** | | **~68** |

**FR Coverage:** FR30 (1.3), FR31 (1.4), FR32 (1.5), FR33 (1.5), FR34 (1.5), FR35 (1.13)

**Architecture Sections Referenced:** Project Structure, Core Architectural Decisions, Implementation Patterns (Naming, Structural, Error Handling, Subprocess, Testing), Starter Template, Testing Strategy

**Dependency Graph:**
```
1.1 ──→ 1.2 ──→ 1.3 ──→ 1.4 ──→ 1.5
  │       │       │ ╲
  │       │       │  ╲
  │       │       └──→ 1.7 ──→ 1.8 ──→ 1.9
  │       │              │       │
  │       │              └───────┴──→ 1.11
  │       │                            │
  ├──→ 1.6                             │
  ├──→ 1.10                            │
  │       │         1.3 ───────────────│──╮
  │       └────────────────────────────┴──→ 1.12 ──→ 1.13
```
Note: 1.12 depends on 1.3 (config), 1.7+1.8 (session), 1.10 (prompt), 1.11 (mock)

---

## Epic 2: Story-to-Tasks Bridge — Stories

**Epic Goal:** `ralph bridge stories/auth.md` генерирует полноценный sprint-tasks.md с задачами, AC-derived тестами, `[GATE]` разметкой, служебными задачами, и `source:` трассировкой.

**PRD Coverage:** FR1, FR2, FR3, FR4, FR5, FR5a
**Stories:** 7 | **Estimated AC:** ~40

---

### Story 2.1: Shared Sprint-Tasks Format Contract

**As a** developer,
**I want** the sprint-tasks.md format defined once as a shared contract,
**So that** bridge output and runner parsing use identical format expectations.

**Acceptance Criteria:**

```gherkin
Given the format contract needs to be shared between bridge and runner
When config/shared/sprint-tasks-format.md is created
Then it defines:
  - Task syntax: "- [ ] Task description [GATE]"
  - Done syntax: "- [x] Task description"
  - Source field: "  source: stories/file.md#AC-N"
  - Feedback syntax: "> USER FEEDBACK: text"
  - Section headers: "## Epic Name" grouping
  - Service task prefix: "[SETUP]", "[VERIFY]", "[E2E]"
  - Source field exact syntax: "  source: <relative-path>#<AC-identifier>"
    Regex: `^\s+source:\s+\S+#\S+`
    Examples: "  source: stories/auth.md#AC-3", "  source: stories/api.md#AC-1"

And the file is embedded via go:embed in config package
And config.SprintTasksFormat() returns the embedded content as string
And bridge imports format for inclusion in bridge prompt
And runner uses same constants (TaskOpen, TaskDone from Story 1.6) to parse

And tests in BOTH bridge and config packages verify:
  - Format content is non-empty
  - Format contains key markers (TaskOpen, TaskDone, GateTag)
  - Structural Rule #8: shared contract tested from both sides
```

**Technical Notes:**
- Architecture: "config/shared/sprint-tasks-format.md — единый source of truth через go:embed shared file"
- Architecture: "Формат определяется один раз. Включается и в bridge prompt, и в execute prompt"
- Structural Rule #8: "Shared contracts = отдельная story с tests в обоих packages"
- This is the hub node identified in Graph of Thoughts (5 writers/readers)

**Prerequisites:** Story 1.1, Story 1.6 (constants)

---

### Story 2.2: Bridge Prompt Template

**As a** developer,
**I want** the bridge prompt template created as a text/template .md file,
**So that** Claude receives clear instructions for converting stories to sprint-tasks.md. (FR1, FR2, FR3, FR5, FR5a)

**Acceptance Criteria:**

```gherkin
Given the bridge prompt template is in bridge/prompts/bridge.md
When the template is assembled via AssemblePrompt (Epic 1 Story 1.10)
Then the prompt includes:
  - Sprint-tasks format contract (from Story 2.1)
  - Instructions to convert story → tasks
  - Instructions to derive test cases from objective AC (FR2)
  - Instructions to mark [GATE] on epic first tasks and milestones (FR3)
  - Instructions to generate service tasks: [SETUP], [VERIFY], [E2E] (FR5)
  - Instructions to add source: field on every task (FR5a)
  - Instructions to check test framework presence (Architecture: bridge checks)
  - Red-green principle reminder for test derivation

And the template uses text/template syntax:
  {{.StoryContent}} — placeholder for story file content (Stage 2 replace)
  {{if .HasExistingTasks}} — conditional for smart merge mode

And prompt includes negative examples (prohibited formats):
  "DO NOT use numbered lists. DO NOT add markdown headers outside
   the defined structure. Every task MUST start with exactly '- [ ]'"

And golden file snapshot test exists:
  bridge/testdata/TestPrompt_Bridge.golden
And test verifies assembled prompt matches golden file

Note: This story creates the BASIC prompt structure. Story 2.4 enriches
with detailed FR-specific instructions and golden file verification.
Dev agent: do NOT implement full FR3/FR5/FR5a details here — only skeleton.
```

**Technical Notes:**
- Architecture: "Bridge prompt template в bridge/prompts/bridge.md (text/template)"
- Architecture: "Bridge: проверка test framework"
- Prompt assembly from Epic 1 Story 1.10 (AssemblePrompt)
- go:embed in bridge package: `//go:embed prompts/bridge.md`
- Anti-pattern from guidelines: "Golden file = достаточно для промптов" → add scenario test

**Prerequisites:** Story 1.10 (prompt assembly), Story 2.1 (format contract)

---

### Story 2.3: Bridge Logic — Core Conversion

**As a** developer,
**I want** `bridge.Run(ctx, cfg)` to read story files and produce sprint-tasks.md,
**So that** the bridge command converts stories into actionable task lists. (FR1)

**Acceptance Criteria:**

```gherkin
Given one or more story file paths are provided as Cobra positional args
  (validated in cmd/ralph/bridge.go, passed to bridge.Run)
When bridge.Run(ctx, cfg, storyPaths []string) is called
Then each story file is read from disk
And story content is injected into bridge prompt via AssemblePrompt
And session.Execute is called with assembled prompt
And Claude output is written to sprint-tasks.md at config.ProjectRoot

Given Claude returns well-formed sprint-tasks.md content
When output is written
Then file is created/overwritten at sprint-tasks.md
And file is UTF-8 encoded with \n line endings

Given story file does not exist
When bridge.Run is called
Then descriptive error returned: "bridge: read story: %w"

Given session.Execute fails
When error is returned
Then bridge wraps: "bridge: execute: %w"
And no partial sprint-tasks.md is written (atomic: write only on success)

And if assembled prompt > 1500 lines, log warning:
  "large prompt — consider splitting story" (Pre-mortem: context window)
Note: concurrent bridge invocations NOT supported (exclusive repo access)

And bridge.Run returns:
  | Field     | Type   | Description                 |
  | TaskCount | int    | Number of tasks generated   |
  | Error     | error  | nil on success              |

And unit tests verify:
  - Successful story → sprint-tasks.md flow (with mock session)
  - Missing story file error
  - Session failure error
  - Output file creation
```

**Technical Notes:**
- Architecture: "bridge.Run(ctx, cfg) — entry point"
- Architecture: "cmd/ralph/bridge.go — wiring only, логика в bridge/"
- MUST NOT modify sprint-tasks.md (Mutation Asymmetry — bridge creates, not mutates)
- Smart merge is separate story (2.6) — this is create-only mode

**Prerequisites:** Story 1.7 (session), Story 2.2 (bridge prompt)

---

### Story 2.4: Service Tasks, Gate Marking, Source Traceability

**As a** developer,
**I want** bridge prompt enhanced with service task generation, gate marking, and source traceability instructions,
**So that** sprint-tasks.md includes all supporting infrastructure. (FR3, FR5, FR5a)

**Acceptance Criteria:**

```gherkin
Given a story with dependencies on new frameworks
When bridge generates sprint-tasks.md
Then [SETUP] service tasks are generated BEFORE implementation tasks:
  e.g. "- [ ] [SETUP] Install and configure testing framework"
  source: stories/auth.md#setup

Given a story spans multiple parts of a feature
When bridge generates sprint-tasks.md
Then [VERIFY] integration verification tasks appear after related tasks:
  e.g. "- [ ] [VERIFY] Verify login API + frontend integration"

Given a story is the first in an epic
When bridge generates sprint-tasks.md
Then the first task of the epic has [GATE] tag:
  e.g. "- [ ] Implement user model [GATE]"

Given user-visible milestones exist in stories
When bridge generates sprint-tasks.md
Then milestone tasks have [GATE] tag

And EVERY task has source: field:
  "  source: stories/auth.md#AC-3" (FR5a)
And source field is indented under the task line

And golden file tests verify:
  - Story with dependencies → [SETUP] tasks present
  - Multi-part story → [VERIFY] tasks present
  - First epic task → [GATE] tag present
  - All tasks have source: field
```

**Technical Notes:**
- FR3: "[GATE] на первой задаче epic'а, user-visible milestones"
- FR5: "Служебные задачи: project setup, integration verification, e2e checkpoint"
- FR5a: "source: stories/file.md#AC-N — трассировка"
- These are prompt instructions, not bridge Go logic — Claude generates these based on prompt
- Scope split (Amelia): Story 2.2 = basic prompt skeleton. Story 2.4 = enriched FR-specific instructions + golden file verification of output quality

**Prerequisites:** Story 2.2, Story 2.3

---

### Story 2.5: Bridge Golden File Tests

**As a** developer,
**I want** comprehensive golden file tests for bridge output,
**So that** any regression in task generation is caught immediately.

**Acceptance Criteria:**

```gherkin
Given test story files exist in bridge/testdata/
When bridge is tested with mock Claude scenarios
Then golden files verify expected output:
  | Test Case                          | Input                    | Validates         |
  | TestBridge_SingleStory             | Simple 3-AC story        | Basic conversion  |
  | TestBridge_MultiStory              | 2 stories, 1 epic        | Multi-file merge  |
  | TestBridge_WithDependencies        | Story needing framework  | [SETUP] tasks     |
  | TestBridge_GateMarking             | First-of-epic story      | [GATE] placement  |
  | TestBridge_SourceTraceability      | Story with 5 ACs         | source: on all    |

And EVERY golden file output validated by:
  - config.TaskOpenRegex scan returns >0 tasks (parseable)
  - source format regex matches on every task line

And golden files are in bridge/testdata/TestName.golden
And tests use go test -update for golden file refresh
And mock Claude scenario returns deterministic output per test

And at least one test uses bridge golden file output
  as input for scanner (Story 2.1 cross-validation):
  bridge output → parsed by config.TaskOpen regex → valid tasks found
```

**Technical Notes:**
- Architecture: "Golden file тесты: input stories → expected sprint-tasks.md"
- Architecture: "bridge/testdata/TestBridge_SingleStory.golden"
- Reverse Engineering: "use bridge golden file output as input for runner"
- Mock Claude scenario files in `internal/testutil/scenarios/bridge_*.json`
- Note (Bob): 5 golden files = ~15 testdata files. May require 2 dev sessions if crafting is complex

**Prerequisites:** Story 2.3, Story 2.4 (**parallel-capable** with Story 2.6)

---

### Story 2.6: Smart Merge

**As a** developer,
**I want** bridge to merge new tasks into existing sprint-tasks.md without losing progress,
**So that** I can re-bridge after story updates without losing completed work. (FR4)

**Acceptance Criteria:**

```gherkin
Given sprint-tasks.md exists with some tasks marked [x] (completed)
And story files have been updated with new/changed requirements
When ralph bridge is run again
Then existing sprint-tasks.md is BACKED UP to sprint-tasks.md.bak
And bridge prompt includes existing sprint-tasks.md content (merge mode)
And Claude merges: new tasks added, completed [x] tasks preserved
And modified tasks updated but completion status preserved

Given the merge prompt is assembled
When {{if .HasExistingTasks}} evaluates to true
Then prompt includes existing sprint-tasks.md content
And explicit instruction: "MUST NOT change [x] status of completed tasks"
And explicit instruction: "MUST preserve source: fields of existing tasks"
And explicit instruction: "PRESERVE original task order. New tasks insert at logical position. NEVER reorder existing tasks"

Given merge fails (session error or output malformed)
When error occurs
Then backup file sprint-tasks.md.bak remains intact
And backup file content equals original sprint-tasks.md byte-for-byte (Murat)
And original sprint-tasks.md is NOT modified
And descriptive error returned

And golden file tests cover:
  - Merge with 2 completed + 3 new tasks
  - Merge where story changed an existing task description
  - Merge that adds [GATE] to previously un-gated task
  - Verify [x] status preserved in all cases
  - Verify existing task order unchanged after merge (Pre-mortem)

And Mandatory AC (Risk Heatmap):
  - Backup sprint-tasks.md before merge
  - Golden file tests for merge scenarios
```

**Technical Notes:**
- Architecture: "Smart Merge — критический промпт (не должен сбросить [x])"
- Risk Heatmap: "Backup sprint-tasks.md перед merge; golden file тесты merge-сценариев"
- Mandatory AC (FR4 Smart Merge): backup + golden file verification
- Merge is prompt-driven (Claude does the merge), not Go code merge

**Prerequisites:** Story 2.3, Story 2.4 (**parallel-capable** with Story 2.5)

---

### Story 2.7: Bridge Integration Test

**As a** developer,
**I want** a full bridge integration test with mock Claude,
**So that** the complete bridge flow is validated end-to-end.

**Acceptance Criteria:**

```gherkin
Given a mock Claude scenario for bridge flow
And test story files in t.TempDir()
When bridge integration test runs
Then the full flow executes:
  1. Story files read from disk
  2. Bridge prompt assembled (with format contract)
  3. Mock Claude invoked with correct flags
  4. Output parsed and written to sprint-tasks.md
  5. Sprint-tasks.md contains expected tasks

And mock Claude validates received prompt contains:
  - Sprint-tasks format contract text
  - Story content
  - FR instructions (test cases, gates, service tasks, source)

And a second test covers smart merge flow:
  1. First bridge creates sprint-tasks.md
  2. Mock marks some tasks [x]
  3. Second bridge with updated story
  4. Merged output preserves [x] tasks
  5. Backup file exists

And scenario-based test validates prompt↔parser contract:
  mock Claude returns predefined bridge output → bridge parses correctly
  (Devil's Advocate guideline)

And CLI-level test via compiled binary (Murat):
  exec.Command("ralph", "bridge", "testdata/story.md") with mock Claude
  Validates full path: Cobra arg parsing → bridge.Run() → output
  Tests exit code mapping on success and failure

And test is in bridge/bridge_integration_test.go
And uses t.TempDir() for isolation
```

**Technical Notes:**
- Architecture: "Integration тесты через mock Claude"
- Devil's Advocate: "scenario-based integration tests для КАЖДОГО типа промпта"
- Uses mock Claude from Epic 1 Story 1.11
- Build tag: `//go:build integration`

**Prerequisites:** Story 2.5, Story 2.6, Story 1.11 (mock Claude)

---

### Epic 2 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 2.1 | Shared Format Contract | — | 2 | 4 |
| 2.2 | Bridge Prompt Template | FR1,FR2 | 2 | 5 |
| 2.3 | Bridge Logic | FR1 | 2 | 7 |
| 2.4 | Service Tasks + Gates + Source | FR3,FR5,FR5a | 1 | 5 |
| 2.5 | Bridge Golden File Tests | — | 5+ testdata | 4 |
| 2.6 | Smart Merge | FR4 | 2 | 7 |
| 2.7 | Bridge Integration Test | — | 1 | 5 |
| | **Total** | **FR1-FR5a** | | **~37** |

**FR Coverage:** FR1 (2.2, 2.3), FR2 (2.2), FR3 (2.4), FR4 (2.6), FR5 (2.4), FR5a (2.4)

**Architecture Sections Referenced:** Project Structure (bridge/), Bridge prompt, Shared contract, Golden files, Testing patterns

**Dependency Graph:**
```
1.6 ──→ 2.1 ──→ 2.2 ──→ 2.3 ──→ 2.4 ─┬→ 2.5 ─┬→ 2.7
1.10 ─────────→ 2.2       ↑            └→ 2.6 ─┘
1.7, 1.8 ─────────────────┘            1.11 ──→ 2.7
```
Note: 2.3 also depends on 1.7/1.8 (session.Execute). 2.5 и 2.6 parallel-capable (Bob)

---

## Epic 3: Core Execution Loop — Stories

**Scope:** FR6, FR7, FR8, FR9, FR10, FR11, FR12, FR23, FR24, FR36, FR37, FR38
**Stories:** 11
**Threshold check:** 11 < 13 → split не требуется

**Invariants (из Epics Structure Plan):**
- **Mutation Asymmetry:** Execute sessions MUST NOT modify sprint-tasks.md task status
- **Review atomicity:** `[x]` + clear findings = atomic operation (Epic 4, но runner готовит контракт)
- **KnowledgeWriter:** minimal interface (1-2 метода), no-op impl в Epic 3, реализация в Epic 6
- **Emergency gates:** minimal stop с exit code + informative message (не interactive UI — это Epic 5)
- **FR17 lessons deferred:** resume-extraction НЕ пишет LEARNINGS.md в Epic 3, только WIP commit + progress

---

### Story 3.1: Execute Prompt Template

**User Story:**
Как разработчик, я хочу чтобы execute-промпт содержал 999-rules guardrails, ATDD-инструкции, red-green cycle и self-directing поведение, чтобы Claude Code автономно и безопасно выполнял задачи.

**Acceptance Criteria:**

```gherkin
Scenario: Execute prompt assembled with all required sections
  Given config with project root and sprint-tasks path
  And optional review-findings.md exists
  When prompt is assembled via text/template + strings.Replace
  Then prompt contains 999-rules guardrail section (FR36)
  And prompt contains ATDD instruction: every AC must have test (FR37)
  And prompt contains zero-skip policy: never skip/xfail tests (FR38)
  And prompt contains red-green cycle instruction
  And prompt contains self-directing instruction: read sprint-tasks.md, take first `- [ ]` (FR11)
  And prompt contains instruction: MUST NOT modify task status markers (Mutation Asymmetry)
  And prompt contains instruction: commit on green tests only (FR8)
  And sprint-tasks-format.md content is injected via strings.Replace (not template)

Scenario: Execute prompt includes review-findings when present
  Given review-findings.md exists with CONFIRMED findings
  When prompt is assembled
  Then review-findings content is injected via strings.Replace
  And prompt contains instruction to fix findings before continuing

Scenario: Execute prompt without review-findings
  Given review-findings.md does not exist
  When prompt is assembled
  Then prompt does not contain findings section
  And prompt instructs Claude to proceed with next task

Scenario: Golden file snapshot matches baseline
  Given execute prompt template in runner/prompts/execute.md
  When assembled with test fixture data
  Then output matches runner/testdata/TestPrompt_Execute.golden
  And golden file is updateable via `go test -update`

Scenario: User content cannot break template
  Given LEARNINGS.md contains Go template syntax `{{.Dangerous}}`
  When prompt is assembled (stage 2: strings.Replace)
  Then template syntax in user content is preserved literally
  And no template execution error occurs
```

**Technical Notes:**
- Architecture: `runner/prompts/execute.md` — Go template file, NOT pure Markdown
- Two-stage assembly: (1) `text/template` for structure, (2) `strings.Replace` for user content (LEARNINGS, findings)
- Deterministic assembly order: template variables sorted or explicitly ordered (Graph of Thoughts finding)
- `config/shared/sprint-tasks-format.md` injected via strings.Replace (shared contract from Story 2.1)
- 999-rules = hard guardrails from Farr Playbook — execute refuses dangerous actions even from review-findings
- Architecture pattern: "Prompt файлы = .md файлы — Go templates, не чистый Markdown"

**Prerequisites:** Story 1.10 (prompt assembly), Story 2.1 (shared format contract)

---

### Story 3.2: Sprint-Tasks Scanner

**User Story:**
Как runner loop, мне нужно сканировать sprint-tasks.md для определения текущего состояния: есть ли открытые задачи, закрытые задачи, gate-маркеры, чтобы управлять потоком выполнения.

**Acceptance Criteria:**

```gherkin
Scenario: Scanner finds open tasks
  Given sprint-tasks.md contains lines with "- [ ]" markers
  When scanner parses the file
  Then returns list of TaskOpen entries with line numbers
  And uses config.TaskOpen constant for matching

Scenario: Scanner finds completed tasks
  Given sprint-tasks.md contains lines with "- [x]" markers
  When scanner parses the file
  Then returns list of TaskDone entries with line numbers
  And uses config.TaskDone constant for matching

Scenario: Scanner detects gate markers
  Given sprint-tasks.md contains "[GATE]" tag on a task line
  When scanner parses the file
  Then marks affected tasks with GateTag flag
  And uses config.GateTag constant for matching

Scenario: Soft validation — no tasks found
  Given sprint-tasks.md contains neither "- [ ]" nor "- [x]"
  When scanner parses the file
  Then returns ErrNoTasks sentinel error
  And error message recommends checking file contents (FR12)

Scenario: Scanner uses string constants
  Given config package defines TaskOpen, TaskDone, GateTag, FeedbackPrefix
  When scanner matches lines
  Then ONLY config constants and config regex patterns are used
  And no hardcoded marker strings exist in scan.go

Scenario: Table-driven tests cover edge cases
  Given test table with cases: empty file, only completed, mixed, gates, malformed lines
  When tests run
  Then all cases pass with correct counts and line numbers
```

**Technical Notes:**
- Architecture: `runner/scan.go` — line scanning + regex, no AST parser
- Pattern: `strings.Split(content, "\n")` + regex match
- Constants in `config/constants.go`: `TaskOpen`, `TaskDone`, `GateTag`, `FeedbackPrefix`
- Regex patterns as `var` with `regexp.MustCompile` in package scope
- Scanner returns structured result (not just bool) for loop control
- Ralph scans ONLY for loop control — does NOT extract task descriptions into prompt (FR11)

**Prerequisites:** Story 1.6 (config constants)

---

### Story 3.3: GitClient Interface + ExecGitClient

**User Story:**
Как runner, мне нужен GitClient interface с реализацией ExecGitClient для проверки здоровья git-репозитория при старте и обнаружения новых коммитов после execute-сессии.

**Acceptance Criteria:**

```gherkin
Scenario: Git health check passes on clean repo
  Given project directory is a git repo
  And working tree is clean (no uncommitted changes)
  And HEAD is not detached
  And not in merge/rebase state
  When GitClient.HealthCheck(ctx) is called
  Then returns nil error

Scenario: Git health check fails on dirty tree
  Given working tree has uncommitted changes
  When GitClient.HealthCheck(ctx) is called
  Then returns ErrDirtyTree sentinel error

Scenario: Git health check fails on detached HEAD
  Given HEAD is detached
  When GitClient.HealthCheck(ctx) is called
  Then returns error indicating detached HEAD

Scenario: Git health check fails during merge/rebase
  Given repo is in merge or rebase state
  When GitClient.HealthCheck(ctx) is called
  Then returns error indicating merge/rebase in progress

Scenario: Commit detection via HEAD comparison
  Given initial HEAD is "abc123"
  When execute session completes
  And GitClient.HeadCommit(ctx) returns "def456"
  Then commit is detected (new HEAD != old HEAD)

Scenario: No commit detection
  Given initial HEAD is "abc123"
  When execute session completes
  And GitClient.HeadCommit(ctx) still returns "abc123"
  Then no commit detected (trigger retry/resume-extraction)

Scenario: ExecGitClient uses exec.CommandContext
  Given ExecGitClient implementation
  When any git command is executed
  Then uses exec.CommandContext(ctx, "git", ...) with context propagation
  And cmd.Dir is set to config.ProjectRoot
```

**Technical Notes:**
- Architecture: `runner/git.go` — `GitClient` interface + `ExecGitClient`
- Interface defined in consumer package (`runner/`), NOT separate `git/` package
- Methods: `HealthCheck(ctx) error`, `HeadCommit(ctx) (string, error)` (2 methods only; `HealthCheck` returns `ErrDirtyTree` for dirty state — no separate `IsClean` method needed)
- Sentinel errors: `ErrDirtyTree`, `ErrDetachedHead` — в runner package scope
- Pattern: `cmd.Output()` for git, extract via `strings.TrimSpace`
- Health check runs at `ralph run` startup (FR6)
- No wall-clock timeout for git — cancellation only via ctx (Architecture decision)

**Prerequisites:** Story 1.8 (session package с exec.CommandContext pattern)

---

### Story 3.4: MockGitClient + Dirty State Recovery

**User Story:**
Как runner, мне нужно восстанавливать чистое состояние git при обнаружении dirty working tree (прерванная сессия), а как разработчик тестов, мне нужен MockGitClient для unit-тестирования runner без реального git.

**Acceptance Criteria:**

```gherkin
Scenario: Dirty state recovery via git checkout
  Given working tree is dirty (interrupted session)
  When runner detects dirty state at startup (FR12)
  Then executes `git checkout -- .` to restore clean state
  And logs warning about recovery action
  And proceeds with execution after recovery

Scenario: MockGitClient implements GitClient interface
  Given MockGitClient in internal/testutil/mock_git.go
  When used in tests
  Then implements all GitClient methods
  And returns preconfigured responses per test scenario
  And tracks method call counts for assertions

Scenario: MockGitClient simulates commit detection
  Given MockGitClient configured with HEAD sequence ["abc", "abc", "def"]
  When HeadCommit called three times
  Then returns "abc", "abc", "def" in order
  And test can verify commit detection logic

Scenario: MockGitClient simulates health check failure
  Given MockGitClient configured with HealthCheck returning ErrDirtyTree
  When runner calls HealthCheck at startup
  Then receives ErrDirtyTree
  And test can verify dirty state recovery path

Scenario: Recovery only runs when dirty tree detected
  Given working tree is clean
  When runner starts
  Then git checkout is NOT executed
  And execution proceeds normally
```

**Technical Notes:**
- Architecture: `internal/testutil/mock_git.go` — `MockGitClient`
- Deferred from Epic 1 Story 1.11 (Bob SM decision: MockGitClient belongs with GitClient in Epic 3)
- Pattern: `Mock` prefix naming convention
- Dirty recovery: `git checkout -- .` — discards uncommitted changes (Architecture: "Crash recovery через git checkout")
- MockGitClient stores call sequence for scenario-based testing
- Recovery logged as WARN level (Architecture: packages return errors, cmd/ logs)

**Prerequisites:** Story 3.3 (GitClient interface), Story 1.11 (mock Claude infra pattern)

---

### Story 3.5: Runner Loop Skeleton — Happy Path

**User Story:**
Как пользователь `ralph run`, я хочу чтобы система последовательно выполняла задачи из sprint-tasks.md, запуская свежую Claude Code сессию на каждую задачу и коммитя при green тестах.

**Acceptance Criteria:**

```gherkin
Scenario: Runner executes tasks sequentially
  Given sprint-tasks.md contains 3 open tasks (- [ ])
  And MockGitClient returns health check OK
  And MockClaude returns exit 0 with new commit for each execute
  When runner.Run(ctx, cfg) is called
  Then executes 3 sequential Claude sessions (FR6)
  And each session is fresh (new invocation, not --resume) (FR7)
  And passes --max-turns from config (FR10)

Scenario: Runner detects commit after execute
  Given execute session completes
  And HEAD changed from "abc" to "def"
  When runner checks for commit
  Then detects new commit (FR8)
  And proceeds to next phase (review in Epic 4, stub for now)

Scenario: Runner stops when all tasks complete
  Given sprint-tasks.md has no remaining "- [ ]" tasks
  When runner scans for next task
  Then returns successfully (exit code 0)
  And logs completion message

Scenario: Runner runs health check at startup
  Given ralph run invoked
  When runner starts
  Then calls GitClient.HealthCheck first (FR6)
  And fails with informative error if health check fails

Scenario: Runner uses session CLI flag constants
  Given config specifies max_turns=25 and model="sonnet"
  When runner invokes session.Execute
  Then passes --max-turns 25 (uses SessionFlagMaxTurns constant)
  And passes --model sonnet (uses SessionFlagModel constant)

Scenario: Runner stub for review step (configurable)
  Given execute completed with commit
  When review phase would run
  Then stub returns "clean" by default (no findings)
  And task advances to next
  And stub is clearly marked as placeholder for Epic 4
  And stub accepts configurable response sequence for testing (e.g., findings N times then clean)
  And this enables Story 3.10 review_cycles counter to be tested within Epic 3

Scenario: Mutation Asymmetry enforced
  Given runner loop completes a task
  When checking sprint-tasks.md modifications
  Then runner process itself NEVER writes task status markers
  And only Claude sessions (review in Epic 4) modify task status
```

**Technical Notes:**
- Architecture: `runner/runner.go` — `runner.Run(ctx, cfg)` main entry point
- Structural pattern: "Каждый package имеет одну главную exported функцию"
- Review step is a configurable stub returning "clean" by default — Epic 4 replaces with real review
- Review stub follows defined function signature: `func(ctx, ReviewOpts) (ReviewResult, error)`. `ReviewResult` contains: `Clean bool`, `FindingsPath string`. This establishes the seam for Epic 4 replacement
- Runner passes `config.ProjectRoot` as `cmd.Dir` for all sessions
- Context propagation: `exec.CommandContext(ctx)` for cancellation
- Happy path only — no retry, no resume, no emergency stops (Stories 3.6-3.10)
- Log file opened by `cmd/ralph`, runner returns results/errors

**Prerequisites:** Story 3.1 (execute prompt), Story 3.2 (scanner), Story 3.3 (GitClient), Story 1.8 (session)

---

### Story 3.6: Runner Retry Logic

**User Story:**
Как пользователь, я хочу чтобы система повторяла неудачные execute-сессии (нет коммита) до настраиваемого максимума, чтобы транзиентные ошибки AI не блокировали прогресс.

**Acceptance Criteria:**

```gherkin
Scenario: No commit triggers retry
  Given execute session completes with exit code 0
  But HEAD has not changed (no commit)
  When runner checks for commit
  Then increments execute_attempts counter (FR9)
  And triggers resume-extraction (Story 3.7)
  And retries execute after resume-extraction

Scenario: execute_attempts counter increments correctly
  Given task has execute_attempts = 0
  When first execute fails (no commit)
  Then execute_attempts becomes 1
  And when second execute fails
  Then execute_attempts becomes 2

Scenario: Commit resets counter
  Given execute_attempts = 2
  When execute session produces a commit
  Then execute_attempts resets to 0
  And proceeds to review phase

Scenario: Counter is per-task
  Given task A has execute_attempts = 2
  When task A succeeds and task B starts
  Then task B has execute_attempts = 0

Scenario: Max iterations configurable
  Given config has max_iterations = 5 (default 3)
  When execute_attempts reaches 5
  Then emergency stop triggers (Story 3.9)

Scenario: Non-zero exit code also triggers retry
  Given execute session returns non-zero exit code
  When runner processes result
  Then treats as failure (no commit expected)
  And increments execute_attempts
  And triggers resume-extraction
```

**Technical Notes:**
- Architecture: два независимых счётчика — `execute_attempts` (max_iterations) и `review_cycles` (max_review_iterations)
- `execute_attempts` counter: per-task, resets on commit
- Default `max_iterations` = 3 (from config)
- No-commit detection: compare HEAD before/after via GitClient.HeadCommit
- Resume-extraction trigger is the link to Story 3.7
- Emergency stop trigger is the link to Story 3.9
- NFR12: retry timing uses exponential backoff between attempts (e.g., 1s, 2s, 4s) to handle transient Claude CLI failures

**Prerequisites:** Story 3.5 (runner loop skeleton)

---

### Story 3.7: Resume-Extraction Integration

**User Story:**
Как система, я хочу возобновлять прерванные execute-сессии через `claude --resume`, чтобы сохранить WIP-прогресс и записать состояние выполнения перед retry.

**Acceptance Criteria:**

```gherkin
Scenario: Resume-extraction invokes --resume with session_id
  Given execute session returned session_id "abc-123" but no commit
  When resume-extraction triggers
  Then invokes claude --resume abc-123 (FR9)
  And session type is resume-extraction (not fresh execute)

Scenario: Resume-extraction creates WIP commit
  Given resume-extraction session completes
  When checking git state
  Then WIP commit exists (partial progress saved)
  And commit message indicates WIP status

Scenario: Resume-extraction writes progress to sprint-tasks.md
  Given resume-extraction session runs
  When it interacts with sprint-tasks.md
  Then progress notes are added (but NOT task status change)
  And Mutation Asymmetry preserved: no `[x]` marking

Scenario: KnowledgeWriter interface defined (no-op)
  Given KnowledgeWriter interface in runner/knowledge.go
  When resume-extraction calls KnowledgeWriter.WriteProgress(ctx, data)
  Then no-op implementation returns nil
  And interface has maximum 2 methods
  And interface is designed for extension in Epic 6

Scenario: KnowledgeWriter no-op does not write LEARNINGS.md
  Given no-op KnowledgeWriter implementation
  When resume-extraction completes
  Then LEARNINGS.md is NOT created or modified
  And FR17 lessons deferred to Epic 6

Scenario: Resume-extraction session uses correct flags
  Given session_id from previous execute
  When resume-extraction invoked
  Then uses --resume flag (uses SessionFlagResume constant)
  And inherits max_turns from config
```

**Technical Notes:**
- Architecture: `claude --resume <session_id>` — resume previous session
- FR17 lessons writing deferred to Epic 6 (Chaos Monkey finding) — no LEARNINGS.md in Epic 3
- KnowledgeWriter interface: `WriteProgress(ctx, ProgressData) error` + `WriteLessons(ctx, LessonsData) error` (max 2)
- `ProgressData` struct: `SessionID string`, `TaskDescription string` (minimum fields, establishes contract for Epic 6)
- `LessonsData` struct: `SessionID string`, `Content string` (minimum fields)
- No-op implementation: both methods return nil, structs defined but unused until Epic 6
- Interface in `runner/knowledge.go`, no-op impl + data structs in same file
- Extensible in Epic 6: add struct fields (not change method signatures), or wrap with richer impl
- Resume-extraction can write progress notes into sprint-tasks under the task but NOT change `- [ ]` / `- [x]`

**Prerequisites:** Story 3.6 (retry logic triggers resume-extraction), Story 1.9 (session --resume support)

---

### Story 3.8: Runner Resume on Re-run

**User Story:**
Как пользователь, я хочу чтобы при повторном запуске `ralph run` система продолжала с первой незавершённой задачи, восстанавливая dirty state если нужно.

**Acceptance Criteria:**

```gherkin
Scenario: Resume from first incomplete task
  Given sprint-tasks.md has tasks: [x] task1, [x] task2, [ ] task3, [ ] task4
  When ralph run starts
  Then scanner finds first "- [ ]" = task3
  And execution begins from task3 (FR12)

Scenario: All tasks completed
  Given sprint-tasks.md has only "- [x]" tasks
  When ralph run starts
  Then reports "all tasks completed"
  And exits with code 0

Scenario: Dirty tree recovery on re-run
  Given working tree is dirty (interrupted previous run)
  When ralph run starts
  And GitClient.HealthCheck returns ErrDirtyTree
  Then executes git checkout -- . for recovery (FR12)
  And logs warning about recovery
  And proceeds with first incomplete task

Scenario: Soft validation warning
  Given sprint-tasks.md contains no "- [ ]" and no "- [x]" markers
  When scanner parses file
  Then outputs warning recommending file check (FR12)
  And exits (no tasks to process)

Scenario: Re-run after partial completion
  Given 5 tasks total, 3 completed in previous run
  When ralph run re-invoked
  Then starts from task 4 (first "- [ ]")
  And does not re-execute completed tasks
```

**Technical Notes:**
- Architecture: "При повторном запуске ralph run продолжает с первой незавершённой задачи"
- Scanner already finds first `- [ ]` (Story 3.2) — this story wires it into runner startup
- Dirty recovery calls GitClient methods from Story 3.3/3.4
- Soft validation: ErrNoTasks from scanner → formatted warning
- No special "resume state" file — sprint-tasks.md IS the state (hub node)

**Prerequisites:** Story 3.5 (runner loop), Story 3.4 (dirty state recovery), Story 3.2 (scanner)

---

### Story 3.9: Emergency Stop — Execute Attempts Exhausted

**User Story:**
Как пользователь, я хочу чтобы система автоматически останавливалась когда AI исчерпал максимум попыток выполнения задачи, чтобы я не тратил ресурсы впустую.

**Acceptance Criteria:**

```gherkin
Scenario: Emergency stop when execute_attempts reaches max
  Given max_iterations = 3 (config default)
  And current task has execute_attempts = 3
  When runner checks counter before next attempt
  Then triggers emergency stop (FR23)
  And exits with code 1

Scenario: Informative stop message
  Given emergency stop triggered by execute_attempts
  When stop message is generated
  Then includes task name/description that failed
  And includes number of attempts made
  And includes suggestion to check logs
  And message uses fatih/color for visibility (via cmd/)

Scenario: Configurable max_iterations
  Given config has max_iterations = 5
  When execute_attempts reaches 5
  Then emergency stop triggers
  And does not trigger at 3 or 4

Scenario: Emergency stop is non-interactive
  Given emergency stop triggers
  When system stops
  Then does NOT prompt for user input (не interactive gate)
  And simply exits with error code + message
  And interactive gate upgrade is in Epic 5
```

**Technical Notes:**
- Architecture: "Emergency gate: minimal stop + message при execute_attempts >= max_iterations (FR23) — не interactive UI, просто stop с exit code + info"
- Returns sentinel error `ErrMaxRetries` — `cmd/ralph` maps to exit code 1
- Pattern: packages return errors, `cmd/ralph/` decides output format and exit codes
- Interactive upgrade (approve/retry/skip/quit) deferred to Epic 5
- Runner returns structured error with task info + attempt count for cmd/ formatting

**Prerequisites:** Story 3.6 (execute_attempts counter)

---

### Story 3.10: Emergency Stop — Review Cycles Trigger Point

**User Story:**
Как runner, мне нужно отслеживать счётчик review_cycles и останавливаться при превышении максимума, чтобы предотвратить бесконечные циклы execute→review.

**Acceptance Criteria:**

```gherkin
Scenario: review_cycles counter increments via configurable stub
  Given task enters review phase (configurable stub from Story 3.5)
  And review stub configured to return findings 2 times then clean
  When execute→review cycle repeats
  Then review_cycles counter increments to 2
  And on third review (clean) counter resets to 0

Scenario: Emergency stop at max_review_iterations
  Given max_review_iterations = 3 (config default)
  And review_cycles reaches 3
  When runner checks counter
  Then triggers emergency stop (FR24)
  And exits with code 1
  And message indicates review cycle exhaustion

Scenario: Counter is per-task
  Given task A had review_cycles = 2
  When task A completes (clean review) and task B starts
  Then task B has review_cycles = 0

Scenario: Counter resets on clean review
  Given review_cycles = 1
  When review returns clean (no findings)
  Then review_cycles resets to 0
  And task marked complete (by review in Epic 4)

Scenario: Configurable max_review_iterations
  Given config has max_review_iterations = 5
  When review_cycles reaches 5
  Then emergency stop triggers

Scenario: Trigger point prepared for Epic 4
  Given review stub returns "clean" in Epic 3
  When review is replaced with real review in Epic 4
  Then review_cycles counter already integrated
  And no runner loop changes needed for FR24
```

**Technical Notes:**
- Architecture: два независимых счётчика — `execute_attempts` (FR23) и `review_cycles` (FR24)
- In Epic 3: review is a configurable stub (Story 3.5), review_cycles counter and max check are real code tested via stub sequences
- In Epic 4: real review replaces stub, counter logic already works
- Actual interactive gate for FR24 in Epic 5
- Same pattern as Story 3.9: returns structured error, cmd/ formats output
- Declares `ErrMaxReviewCycles` sentinel error in runner package (alongside ErrDirtyTree, ErrDetachedHead from Story 3.3)

**Prerequisites:** Story 3.5 (runner loop with review stub), Story 3.9 (emergency stop pattern)

---

### Story 3.11: Runner Integration Test

**User Story:**
Как разработчик, я хочу комплексный integration test runner-а, который проверяет full flow через scenario-based mock Claude и MockGitClient, чтобы гарантировать корректную работу всех компонентов вместе.

**Acceptance Criteria:**

```gherkin
Scenario: Happy path — all tasks complete
  Given scenario JSON with 3 execute steps (all produce commits)
  And MockGitClient returns health OK + commit sequence
  And sprint-tasks.md fixture with 3 open tasks
  When runner.Run executes
  Then 3 sessions launched sequentially
  And runner exits with code 0

Scenario: Retry + resume-extraction flow
  Given scenario JSON: execute (no commit) → resume-extraction → execute (commit)
  And MockGitClient returns no-commit then commit
  When runner.Run executes
  Then execute_attempts increments to 1
  And resume-extraction invoked with session_id
  And retry succeeds with commit

Scenario: Emergency stop on max retries
  Given scenario JSON: 3 executes all without commit
  And max_iterations = 3
  When runner.Run executes
  Then execute_attempts reaches 3
  And runner returns ErrMaxRetries
  And error contains task info

Scenario: Resume on re-run after partial completion
  Given sprint-tasks.md with 2 completed + 1 open task
  And scenario JSON for 1 execute (commit)
  When runner.Run executes
  Then starts from task 3 (first open)
  And completes successfully

Scenario: Dirty tree recovery at startup
  Given MockGitClient.HealthCheck returns ErrDirtyTree
  When runner starts
  Then recovery executed (git checkout -- .)
  And then proceeds normally

Scenario: Resume-extraction failure triggers recovery
  Given scenario JSON: execute (no commit) → resume-extraction (exit code non-zero)
  And MockGitClient shows dirty tree after failed resume
  When runner processes resume-extraction failure
  Then triggers dirty state recovery (git checkout -- .)
  And retries execute from clean state
  And execute_attempts is correctly tracked through the recovery

Scenario: Uses bridge golden file output as input
  Given sprint-tasks.md from bridge golden file testdata (not hand-crafted)
  When used as runner test input
  Then validates bridge→runner data contract
  And proves end-to-end compatibility

Scenario: Test isolation
  Given integration test
  When test runs
  Then uses t.TempDir() for all file operations
  And no shared state between test cases
  And build tag //go:build integration
```

**Technical Notes:**
- Architecture: "scenario = ordered sequence of mock responses"
- Scenario JSON format: `[{type, exit_code, session_id, creates_commit, output_file}]`
- Uses MockClaude from Story 1.11 + MockGitClient from Story 3.4
- Bridge golden file output (Story 2.5) used as sprint-tasks.md input — validates contract
- Build tag: `//go:build integration`
- Test file: `runner/runner_integration_test.go`
- Review step uses stub returning "clean" — real review integration in Epic 4
- Scenarios cover: happy path, retry, resume, emergency stop, re-run, dirty recovery

**Prerequisites:** Story 3.5-3.10 (all runner stories), Story 3.4 (MockGitClient), Story 1.11 (MockClaude), Story 2.5 (bridge golden files)

---

### Epic 3 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 3.1 | Execute Prompt Template | FR8,FR11,FR36,FR37,FR38 | 2 + testdata | 5 |
| 3.2 | Sprint-Tasks Scanner | FR6,FR11,FR12 | 2 | 6 |
| 3.3 | GitClient Interface + ExecGitClient | FR6 | 2 | 7 |
| 3.4 | MockGitClient + Dirty State Recovery | FR12 | 2 | 5 |
| 3.5 | Runner Loop Skeleton | FR6,FR7,FR8,FR10,FR11 | 2 | 8 |
| 3.6 | Runner Retry Logic | FR9 | 1 | 6 |
| 3.7 | Resume-Extraction Integration | FR9 | 2 | 6 |
| 3.8 | Runner Resume on Re-run | FR12 | 1 | 5 |
| 3.9 | Emergency Stop — Execute | FR23 | 1 | 4 |
| 3.10 | Emergency Stop — Review Cycles | FR24 | 1 | 6 |
| 3.11 | Runner Integration Test | — | 1 + scenarios | 8 |
| | **Total** | **FR6-FR12,FR23,FR24,FR36-FR38** | | **~66** |

**FR Coverage:** FR6 (3.2, 3.3, 3.5), FR7 (3.5), FR8 (3.1, 3.5), FR9 (3.6, 3.7), FR10 (3.5), FR11 (3.1, 3.2, 3.5), FR12 (3.2, 3.4, 3.8), FR23 (3.9), FR24 (3.10), FR36 (3.1), FR37 (3.1), FR38 (3.1)

**Architecture Sections Referenced:** Runner package (runner.go, git.go, scan.go, knowledge.go), Subprocess Patterns, Testing Patterns, Error Handling, File I/O, Project Structure

**Dependency Graph:**
```
1.6 ─────→ 3.2
1.8 ─────→ 3.3 ──→ 3.4
1.10 ────→ 3.1
2.1 ─────→ 3.1
1.8 ─────→ 3.5 ←── 3.1, 3.2, 3.3
             │
             ├──→ 3.6 ──→ 3.7
             │           ↗
             │     1.8 ─┘
             ├──→ 3.8 ←── 3.4
             └──→ 3.9 ←── 3.6
                  3.10 ←── 3.5, 3.9 (pattern)
1.11 ────→ 3.4, 3.11
2.5 ─────→ 3.11
3.5-3.10 → 3.11
```
Note: 3.3 и 3.1 parallel-capable (оба зависят от разных Epic 1 stories). 3.9 и 3.10 partially parallel (3.10 reuses pattern from 3.9)

---

## Epic 4: Code Review Pipeline — Stories

**Scope:** FR13, FR14, FR15, FR16, FR17, FR18, FR18a, FR37, FR38
**Stories:** 8
**Release milestone:** v0.1 после завершения этого эпика

**Invariants (из Epics Structure Plan + Epic 3):**
- **Review atomicity:** `[x]` + clear review-findings.md = atomic (write both or neither)
- **Mutation Asymmetry:** Review sessions write `[x]` and findings. Execute sessions MUST NOT write `[x]`
- **FR17 scope Epic 4:** review = `[x]` + findings write/clear ONLY. Lessons (LEARNINGS.md, CLAUDE.md) **deferred to Epic 6**
- **FR16a (severity filtering):** Growth — not in Epic 4. All CONFIRMED findings block pipeline
- **ReviewResult contract (Story 3.5):** `func(ctx, ReviewOpts) (ReviewResult, error)` — `ReviewResult{Clean bool, FindingsPath string}`
- **review_cycles counter:** already built in Epic 3 Story 3.10 — Epic 4 replaces stub with real review
- **Bottleneck:** findings quality determines everything — adversarial tests critical

---

### Story 4.1: Review Prompt Template

**User Story:**
Как система, я хочу review-промпт который инструктирует Claude Code запустить 4 параллельных sub-агента через Task tool, верифицировать их findings, и записать результат, чтобы обеспечить качественное ревью кода.

**Acceptance Criteria:**

```gherkin
Scenario: Review prompt assembled with all required sections
  Given config with project root and sprint-tasks path
  When review prompt is assembled via text/template + strings.Replace
  Then prompt instructs Claude to launch 4 sub-agents via Task tool (FR15)
  And prompt names sub-agents: quality, implementation, simplification, test-coverage
  And prompt instructs verification of sub-agent findings (FR16)
  And prompt instructs CONFIRMED/FALSE POSITIVE classification
  And prompt instructs severity assignment: CRITICAL/HIGH/MEDIUM/LOW
  And prompt instructs atomic [x] + clear on clean review (FR17)
  And prompt instructs findings write on non-clean review (FR17)
  And prompt contains instruction: MUST NOT modify source code (FR17)
  And prompt contains instruction: MUST NOT write LEARNINGS.md or CLAUDE.md (Epic 6)

Scenario: Review prompt includes sprint-tasks-format contract
  Given shared sprint-tasks-format.md (Story 2.1)
  When review prompt is assembled
  Then sprint-tasks-format content injected via strings.Replace
  And review knows exact format for [x] marking

Scenario: Golden file snapshot matches baseline
  Given review prompt template in runner/prompts/review.md
  When assembled with test fixture data
  Then output matches runner/testdata/TestPrompt_Review.golden
  And golden file updateable via `go test -update`

Scenario: Adversarial golden file — planted bug detection
  Given review prompt + test fixture with code containing deliberately planted bug
  When review prompt instructs sub-agents
  Then prompt structure enables detection of planted bugs
  And golden file captures prompt that should trigger finding

Scenario: Adversarial golden file — false positive resistance
  Given review prompt + test fixture with clean, correct code
  When review prompt instructs sub-agents
  Then prompt structure discourages false positives
  And golden file captures prompt that should yield clean review
```

**Technical Notes:**
- Architecture: `runner/prompts/review.md` — Go template file
- Two-stage assembly same as execute prompt (Story 3.1)
- Review prompt is the orchestrator — Claude inside the session uses Task tool to spawn sub-agents
- From Ralph's perspective: one `session.Execute` call with review prompt
- Adversarial golden files validate prompt quality, not Claude behavior
- FR17 lessons scope explicitly excluded — prompt MUST NOT instruct lessons writing

**Prerequisites:** Story 3.1 (execute prompt pattern), Story 2.1 (shared format contract)

---

### Story 4.2: Sub-Agent Prompts

**User Story:**
Как review-сессия, я хочу 4 специализированных sub-agent промпта с чёткими scope boundaries, чтобы каждый агент проверял свою зону ответственности без overlap.

**Acceptance Criteria:**

```gherkin
Scenario: Four sub-agent prompts exist with explicit scopes
  Given runner/prompts/agents/ directory
  Then contains quality.md, implementation.md, simplification.md, test-coverage.md
  And each prompt defines explicit SCOPE section (what this agent checks)
  And each prompt defines explicit OUT-OF-SCOPE section (what other agents check)
  And no overlap between 4 agents' scope boundaries

Scenario: Quality agent scope
  Given quality.md prompt
  Then scope includes: bugs, security issues, performance problems, error handling
  And out-of-scope: AC compliance (implementation), code simplicity (simplification), test coverage (test-coverage)

Scenario: Implementation agent scope
  Given implementation.md prompt
  Then scope includes: acceptance criteria compliance, feature completeness, requirement satisfaction
  And out-of-scope: code quality (quality), simplification opportunities (simplification), test coverage (test-coverage)

Scenario: Simplification agent scope
  Given simplification.md prompt
  Then scope includes: code readability, unnecessary complexity, over-engineering, dead code
  And out-of-scope: bugs (quality), AC compliance (implementation), test coverage (test-coverage)

Scenario: Test-coverage agent scope
  Given test-coverage.md prompt
  Then scope includes: test coverage for each AC (FR37 ATDD), no skip/xfail (FR38), test quality
  And out-of-scope: code bugs (quality), AC compliance (implementation), code simplicity (simplification)

Scenario: Golden file snapshot per agent
  Given each sub-agent prompt
  When assembled with test fixture
  Then matches runner/testdata/TestPrompt_Agent_<name>.golden

Scenario: Adversarial test — agent detects planted issue in its scope
  Given test fixture code with scope-specific planted issue per agent
  When prompt structure analyzed
  Then each agent's prompt is structured to detect issues in its scope
  And adversarial golden files capture expected detection behavior
```

**Technical Notes:**
- Architecture: `runner/prompts/agents/{quality,implementation,simplification,test-coverage}.md`
- Sub-agents are Claude Task tool agents INSIDE the review session — not separate Ralph processes
- Explicit scope boundaries = Devil's Advocate finding from epics elicitation (4 sub-agents overlap concern)
- FR37 (ATDD) enforced by test-coverage agent specifically
- FR38 (zero skip) enforced by test-coverage agent specifically
- Each prompt is `go:embed` in runner package

**Prerequisites:** Story 4.1 (review prompt references sub-agents)

---

### Story 4.3: Review Session Logic

**User Story:**
Как runner, мне нужно запускать review session после успешного execute (commit detected), передавая review prompt в fresh Claude Code session.

**Acceptance Criteria:**

```gherkin
Scenario: Review launches after commit detection
  Given execute session completed with new commit (FR8)
  When runner proceeds to review phase
  Then launches fresh Claude Code session with review prompt (FR13, FR14)
  And session uses --max-turns from config
  And session is independent of execute session context

Scenario: Review replaces stub from Story 3.5
  Given runner loop with ReviewResult contract
  When review session completes
  Then returns ReviewResult{Clean, FindingsPath}
  And matches function signature from Story 3.5 contract

Scenario: Review session captures output
  Given review session runs
  When Claude processes review prompt
  Then session.Execute returns SessionResult with session_id
  And runner can determine review outcome from file state

Scenario: Review session uses fresh context
  Given execute session was session abc-123
  When review session launches
  Then review session gets new session_id (not --resume)
  And review has no memory of execute session internals (FR14)

Scenario: Review outcome determined from file state
  Given review session completed
  When runner checks sprint-tasks.md for [x] on current task
  And checks review-findings.md via os.Stat + os.ErrNotExist
  Then computes ReviewResult{Clean: true} if [x] present and findings absent/empty
  Or computes ReviewResult{Clean: false, FindingsPath} if findings file non-empty
  And this is the Go code bridge between Claude behavior and ReviewResult contract
```

**Technical Notes:**
- From Ralph's perspective: `session.Execute(ctx, reviewOpts)` — one call
- Claude inside session orchestrates 4 sub-agents via Task tool (not Ralph's concern)
- Review outcome determined by checking: (1) sprint-tasks.md for `[x]`, (2) review-findings.md existence/content
- **File-state-to-ReviewResult logic** lives in this story as `determineReviewOutcome(projectRoot, taskLine) (ReviewResult, error)` function in runner
- **Current task identification:** `determineCurrentTask(sprintTasksPath) (taskLine, error)` parses sprint-tasks.md, finds first `- [ ]` (same logic as scanner Story 3.2 but returns the task line string for `[x]` check after review)
- This story replaces the configurable review stub from Story 3.5 with real implementation
- Session flags: same pattern as execute (SessionFlagMaxTurns, SessionFlagModel constants)

**Prerequisites:** Story 4.1 (review prompt), Story 3.5 (ReviewResult contract)

---

### Story 4.4: Findings Verification Logic

**User Story:**
Как review prompt, я хочу инструктировать Claude верифицировать каждый sub-agent finding и классифицировать его, чтобы только реальные проблемы попадали в review-findings.md.

**Acceptance Criteria:**

```gherkin
Scenario: Each finding verified independently
  Given 4 sub-agents produced findings
  When review session verifies each finding
  Then each classified as CONFIRMED or FALSE POSITIVE (FR16)
  And verification happens BEFORE writing to review-findings.md

Scenario: Severity assigned to confirmed findings
  Given finding classified as CONFIRMED
  When severity assigned
  Then exactly one of: CRITICAL, HIGH, MEDIUM, LOW (FR16)
  And severity is mandatory for every CONFIRMED finding

Scenario: False positives excluded from findings file
  Given finding classified as FALSE POSITIVE
  When review-findings.md is written
  Then FALSE POSITIVE findings are NOT included
  And only CONFIRMED findings appear in file

Scenario: Finding structure complete
  Given CONFIRMED finding
  When written to review-findings.md
  Then contains: ЧТО не так (description)
  And contains: ГДЕ в коде (file path + line range)
  And contains: ПОЧЕМУ это проблема (reasoning)
  And contains: КАК исправить (actionable recommendation)
```

**Technical Notes:**
- Verification is Claude's responsibility inside the review session — Ralph does not parse findings
- Review prompt (Story 4.1) contains verification instructions
- This story defines the finding format contract that review-findings.md must follow
- Finding format = ЧТО/ГДЕ/ПОЧЕМУ/КАК structure for execute session consumption
- FR16a (severity filtering) is Growth — in Epic 4 ALL confirmed findings block
- **Deliverable:** dedicated verification section in `runner/prompts/review.md`. Golden file `TestPrompt_Review.golden` updated to include verification instructions. PR = review.md diff + golden file update

**Prerequisites:** Story 4.3 (review session runs, sub-agents produce findings via 4.1+4.2)

---

### Story 4.5: Clean Review Handling

**User Story:**
Как review session, при отсутствии findings я должен атомарно отметить задачу `[x]` в sprint-tasks.md и очистить review-findings.md, чтобы runner мог перейти к следующей задаче.

**Acceptance Criteria:**

```gherkin
Scenario: Atomic [x] marking + findings clear on clean review
  Given review found no CONFIRMED findings (clean review)
  When review session writes results
  Then marks current task [x] in sprint-tasks.md (FR17)
  And clears review-findings.md content (FR17)
  And both operations happen together (atomic: both or neither)

Scenario: Review MUST NOT modify git working tree
  Given clean review handling
  When review session runs
  Then MUST NOT run git commands
  And MUST NOT modify source code files
  And ONLY modifies sprint-tasks.md and review-findings.md

Scenario: Runner detects clean review via file state
  Given review session completed
  When runner checks review outcome
  Then detects [x] on current task in sprint-tasks.md
  And review-findings.md is empty or absent
  And ReviewResult.Clean = true

Scenario: review_cycles resets on clean review
  Given review_cycles = 2 for current task
  When clean review detected
  Then review_cycles resets to 0 (Story 3.10 counter)
  And runner proceeds to next task

Scenario: review-findings.md absence equals clean
  Given review-findings.md does not exist after review
  When runner checks
  Then treats as clean review (Architecture: "Отсутствующий review-findings.md = пуст")
```

**Technical Notes:**
- Architecture: "review-findings.md: транзиентный (перезапись при findings, clear при clean)"
- Atomicity: review prompt instructs Claude to do both operations in sequence — not Ralph's enforcement
- Architecture pattern: `os.Stat` + `errors.Is(err, os.ErrNotExist)` — absent = empty
- Review does NOT create commit — only modifies sprint-tasks.md and review-findings.md
- The [x] marking is the ONLY place task status changes — Mutation Asymmetry enforced
- **Deliverable:** dedicated clean-handling section in `runner/prompts/review.md`. Golden file `TestPrompt_Review.golden` updated. PR = review.md diff + golden file update

**Prerequisites:** Story 4.3 (review session logic), Story 3.10 (review_cycles counter)

---

### Story 4.6: Findings Write

**User Story:**
Как review session, при обнаружении CONFIRMED findings я должен перезаписать review-findings.md со структурированными findings, чтобы следующая execute-сессия знала что исправлять.

**Acceptance Criteria:**

```gherkin
Scenario: Findings written to review-findings.md
  Given review found 2 CONFIRMED findings
  When review session writes findings
  Then review-findings.md contains structured findings (FR17)
  And previous content fully replaced (overwrite, not append)
  And findings are for current task only (no task ID in file)

Scenario: Finding format matches contract
  Given CONFIRMED finding with severity HIGH
  When written to review-findings.md
  Then format: ЧТО/ГДЕ/ПОЧЕМУ/КАК structure (Story 4.4)
  And severity clearly indicated
  And file is self-contained (execute session needs no other context)

Scenario: Only current task findings in file
  Given review for task 3
  When findings written
  Then review-findings.md contains ONLY task 3 findings
  And no historical findings from previous tasks
  And file is transient — represents current state only

Scenario: Runner detects findings via file state
  Given review session wrote findings
  When runner checks review outcome
  Then review-findings.md exists and is non-empty
  And ReviewResult.Clean = false
  And ReviewResult.FindingsPath points to review-findings.md
```

**Technical Notes:**
- Architecture: review-findings.md is transient — overwrite on findings, clear on clean
- File path: `{projectRoot}/review-findings.md` (resolved via config.ProjectRoot)
- Execute prompt (Story 3.1) already handles non-empty review-findings.md case
- File format designed for Claude consumption, not human — structured enough for AI to parse
- No task ID in file because it's always about the current task
- **Deliverable:** dedicated findings-write section in `runner/prompts/review.md`. Golden file `TestPrompt_Review.golden` updated. PR = review.md diff + golden file update

**Prerequisites:** Story 4.4 (finding format), Story 4.3 (review session)

---

### Story 4.7: Execute→Review Loop Integration

**User Story:**
Как runner, мне нужно интегрировать реальную review фазу в основной loop, заменив stub на настоящий review с review_cycles counter.

**Acceptance Criteria:**

```gherkin
Scenario: Review replaces stub in runner loop
  Given runner loop from Story 3.5 with configurable review stub
  When Epic 4 integration complete
  Then real review session replaces stub
  And ReviewResult contract preserved (Story 3.5)
  And runner loop code changes are minimal (seam design)

Scenario: Execute→review cycle with review_cycles counter
  Given execute completed with commit
  When review finds 1 CONFIRMED finding
  Then review_cycles increments to 1 (Story 3.10)
  And next execute launched (reads review-findings.md)
  And after fix commit → review again (cycle 2)

Scenario: Clean review after fix cycle
  Given review_cycles = 1 (one previous findings cycle)
  When second review is clean
  Then [x] marked, review-findings.md cleared
  And review_cycles resets to 0
  And runner proceeds to next task

Scenario: Max review iterations triggers emergency stop
  Given max_review_iterations = 3 (config)
  And review_cycles reaches 3 without clean review
  When runner checks counter (Story 3.10 logic)
  Then triggers emergency stop with exit code 1 (FR24)
  And message includes: task name, cycles completed, remaining findings

Scenario: Full task lifecycle in runner loop
  Given sprint-tasks.md with 1 open task
  When runner executes full cycle:
    execute → commit → review → findings → execute → commit → review → clean
  Then task marked [x]
  And 2 execute sessions + 2 review sessions launched
  And review_cycles = 0 at end

Scenario: Execute sees findings and fixes them (FR18)
  Given review-findings.md contains 2 CONFIRMED findings
  When next execute session launches
  Then execute prompt includes review-findings content (Story 3.1)
  And Claude addresses findings instead of implementing from scratch
  And runs tests after fix
  And commits on green tests

Scenario: Ralph does not distinguish first execute from fix execute
  Given review-findings.md is non-empty
  When ralph launches execute
  Then uses same execute prompt template (Story 3.1)
  And same session type (fresh, not --resume)
  And ralph makes no distinction between "first" and "fix" (FR18)

Scenario: Each fix iteration creates separate commit
  Given execute→review→execute→review cycle
  When each execute fixes issues
  Then each fix execute creates its own commit (FR18)
  And commits are separate from initial implementation commit
```

**Technical Notes:**
- This story wires together: Story 3.5 (loop), Story 3.10 (review_cycles), Story 4.3 (review session)
- Minimal runner.go changes — review stub already has correct interface (ReviewResult)
- Emergency stop logic already exists from Story 3.9/3.10 — just activated with real review
- Runner determines review outcome by checking file state, not parsing Claude output
- Architecture: "Ralph НЕ различает первый execute и fix execute — это одна и та же сессия по типу"
- Execute prompt (Story 3.1) already handles review-findings.md injection
- The intelligence is in the prompt — Claude decides whether to implement fresh or fix based on review-findings.md
- Commit detection same as Story 3.5 — HEAD comparison via GitClient

**Prerequisites:** Story 4.3 (review session), Story 4.5 (clean handling), Story 4.6 (findings write), Story 3.10 (review_cycles), Story 3.1 (execute prompt with findings injection)

---

### Story 4.8: Review Integration Test

**User Story:**
Как разработчик, я хочу комплексный integration test review pipeline, покрывающий clean review, findings cycle, fix cycle и emergency stop, чтобы гарантировать корректность перед v0.1 release.

**Acceptance Criteria:**

```gherkin
Scenario: Clean review — single pass
  Given scenario JSON: execute (commit) → review (clean)
  And MockGitClient returns commit after execute
  When runner.Run executes single task
  Then 1 execute + 1 review session launched
  And task marked [x]
  And review-findings.md absent or empty
  And exit code 0

Scenario: Findings → fix → clean review
  Given scenario JSON: execute (commit) → review (findings) → execute (commit) → review (clean)
  And review-findings.md created after first review
  When runner.Run executes
  Then 2 execute + 2 review sessions
  And review_cycles = 1 after first findings, 0 after clean
  And task marked [x] at end

Scenario: Emergency stop on max review cycles
  Given scenario JSON: 3 cycles of execute (commit) → review (findings)
  And max_review_iterations = 3
  When runner.Run executes
  Then review_cycles reaches 3
  And runner returns ErrMaxReviewCycles
  And error contains task info + cycles count

Scenario: Multi-task with mixed outcomes
  Given sprint-tasks.md with 3 tasks
  And scenario JSON: task1 (clean), task2 (1 fix cycle), task3 (clean)
  When runner.Run executes
  Then all 3 tasks marked [x]
  And review_cycles properly reset between tasks

Scenario: Review determines outcome via file state
  Given mock review session that writes [x] to sprint-tasks.md
  And mock review session that writes review-findings.md
  When runner checks outcome
  Then correctly determines clean vs findings from file state
  And does not parse Claude session output

Scenario: Bridge golden file as input
  Given sprint-tasks.md from bridge golden file testdata (Story 2.5)
  When used as runner test input
  Then validates full pipeline: bridge output → execute → review

Scenario: v0.1 smoke test note
  Given all integration tests pass
  When considering release readiness
  Then manual smoke test with real Claude CLI recommended before v0.1 tag
  And documented in test file comments

Scenario: Manual prompt validation checklist documented
  Given review prompt from 4.1 and sub-agent prompts from 4.2
  When manual testing before v0.1 tag
  Then checklist covers: (1) planted bug detected by correct sub-agent
  And (2) clean code yields no false positives
  And (3) findings contain all 4 fields (ЧТО/ГДЕ/ПОЧЕМУ/КАК)
  And (4) clean review produces [x] + empty findings
  And (5) findings review does NOT mark [x]
  And checklist is in runner/testdata/manual_smoke_checklist.md
```

**Technical Notes:**
- Uses MockClaude (Story 1.11) + MockGitClient (Story 3.4) + t.TempDir()
- Scenario JSON extended: `{type: "review", clean: true/false, creates_findings: bool}`
- Build tag: `//go:build integration`
- Test file: `runner/runner_review_integration_test.go` (separate from Story 3.11 runner_integration_test.go)
- v0.1 manual smoke test = Chaos Monkey finding from epics elicitation
- Bridge golden file input validates end-to-end contract
- Manual smoke checklist = formal artifact for v0.1 release gate (Murat TEA recommendation)

**Prerequisites:** Story 4.7 (execute→review loop + fix cycle), Story 3.11 (runner integration test pattern), Story 1.11 (MockClaude), Story 3.4 (MockGitClient)

---

### Epic 4 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 4.1 | Review Prompt Template | FR13,FR15,FR16,FR17 | 2 + testdata | 5 |
| 4.2 | Sub-Agent Prompts | FR15,FR37,FR38 | 4 + testdata | 7 |
| 4.3 | Review Session Logic | FR13,FR14 | 1 | 5 |
| 4.4 | Findings Verification Logic | FR16 | review.md section | 4 |
| 4.5 | Clean Review Handling | FR17 | review.md section | 5 |
| 4.6 | Findings Write | FR17 | review.md section | 4 |
| 4.7 | Execute→Review Loop + Fix Cycle | FR18,FR18a,FR24 | 1 | 8 |
| 4.8 | Review Integration Test | — | 1 + scenarios + checklist | 8 |
| | **Total** | **FR13-FR18a,FR37,FR38** | | **~46** |

**FR Coverage:** FR13 (4.1, 4.3), FR14 (4.3), FR15 (4.1, 4.2), FR16 (4.1, 4.4), FR17 (4.1, 4.5, 4.6), FR18 (4.7), FR18a (4.7), FR37 (4.2), FR38 (4.2)

**Architecture Sections Referenced:** Runner package (prompts/review.md, prompts/agents/), Package Boundaries (review-findings.md writer/reader), Testing Patterns (scenario-based mock, golden files), Data Flow (execute→review cycle)

**Dependency Graph:**
```
3.1 ──→ 4.1 ──→ 4.2
2.1 ──→ 4.1     │
3.5 ──→ 4.3 ←── 4.1 + 4.2
             4.3 ──→ 4.4 ──→ 4.6 ─┐
                │                   │
                ├──→ 4.5 ──────────┤
                                    └──→ 4.7 ──→ 4.8
                            3.10 ──→ 4.7
                            3.1 ───→ 4.7
                            1.11, 3.4 ──→ 4.8
                            2.5 ──→ 4.8
```
Note: 4.4 и 4.5 parallel-capable (prompt-driven, зависят от 4.3 но не друг от друга). 4.6 logically depends on 4.4 (findings must be verified before written). v0.1 release после 4.8. Story 4.8 = бывшая 4.9 (Story 4.8 "Review Fix Cycle" merged в Story 4.7)

---

## Epic 5: Human Gates & Control — Stories

**Scope:** FR20, FR21, FR22, FR25
**Stories:** 6
**Release milestone:** v0.2 (post-initial release)

**Context (из Epics Structure Plan):**
- FR23/FR24 (emergency gates) реализованы как minimal stop в Epic 3 (Stories 3.9, 3.10)
- В Epic 5 emergency stop апгрейдится до interactive gate с approve/retry/feedback/skip/quit
- `gates` package: `gates.Prompt(ctx, gate)` — interactive stdin + `fatih/color`
- `gates` и `session` НЕ зависят друг от друга (Architecture dependency direction)
- Feedback injection: `> USER FEEDBACK: ...` в sprint-tasks.md (config.FeedbackPrefix constant)
- `[GATE]` tag scanning already implemented in Story 3.2 (scanner)

---

### Story 5.1: Basic Gate Prompt

**User Story:**
Как разработчик, я хочу интерактивный gate prompt с цветными опциями approve/skip/quit, чтобы контролировать выполнение на gate-точках.

**Acceptance Criteria:**

```gherkin
Scenario: Gate prompt displays with color
  Given gate triggered for task "TASK-1 — Setup project structure"
  When gates.Prompt(ctx, gate) is called
  Then displays colored prompt via fatih/color:
    🚦 HUMAN GATE: TASK-1 — Setup project structure
       [a]pprove  [r]etry with feedback  [s]kip  [q]uit
    > _
  And waits for user input via io.Reader (not raw fmt.Scan — injectable for testing)

Scenario: Approve action
  Given gate prompt displayed
  When user enters "a"
  Then returns GateDecision{Action: Approve}
  And runner proceeds to next task

Scenario: Skip action
  Given gate prompt displayed
  When user enters "s"
  Then returns GateDecision{Action: Skip}
  And current task marked [x] (skipped)
  And runner proceeds to next task

Scenario: Quit action
  Given gate prompt displayed
  When user enters "q"
  Then returns GateDecision{Action: Quit}
  And runner exits with code 2 (user quit — matches exit code table in Story 1.13)
  And sprint-tasks.md state preserved (incomplete tasks remain [ ])

Scenario: Invalid input re-prompts
  Given gate prompt displayed
  When user enters invalid input (e.g., "x")
  Then displays error message
  And re-displays prompt
  And does not advance or quit

Scenario: Context cancellation during prompt
  Given gate prompt waiting for input
  When ctx is cancelled (Ctrl+C)
  Then returns context.Canceled error
  And runner handles graceful shutdown
```

**Technical Notes:**
- Architecture: `gates/gates.go` — `gates.Prompt(ctx, gate)` entry point
- Structural pattern: one main exported function per package
- `GateDecision` custom error type: `Action` enum (Approve, Retry, Skip, Quit) + optional `Feedback` string
- `fatih/color` for colored output, `io.Reader` + `io.Writer` for testable I/O (not raw `fmt.Scan`/`os.Stdout`)
- `gates.Prompt` accepts `io.Reader` + `io.Writer` parameters (or struct with these fields) for dependency injection
- Gates package does NOT depend on runner or session (Architecture dependency direction)
- Retry action defined here but feedback input handled in Story 5.3

**Prerequisites:** Story 1.5 (fatih/color dependency), Story 1.6 (config constants: FeedbackPrefix, GateTag)

---

### Story 5.2: Gate Detection in Runner

**User Story:**
Как пользователь `ralph run --gates`, я хочу чтобы система останавливалась на задачах с `[GATE]` тегом для моего одобрения перед продолжением.

**Acceptance Criteria:**

```gherkin
Scenario: Gates enabled via --gates flag
  Given ralph run invoked with --gates flag (FR20)
  When runner starts
  Then gates_enabled = true in config
  And runner will check for gate tags during execution

Scenario: Gates disabled by default
  Given ralph run invoked without --gates flag
  When runner encounters [GATE] tagged task
  Then skips gate prompt
  And executes task normally without stopping

Scenario: Stop at GATE-tagged task
  Given gates_enabled = true
  And current task has [GATE] tag (detected by scanner Story 3.2)
  When task completes review (marked [x])
  Then runner calls gates.Prompt AFTER task completion (FR21)
  And waits for developer input before next task

Scenario: Gate prompt shows after task completion, not before
  Given gates_enabled and task with [GATE]
  When runner reaches gate
  Then task is already executed and reviewed
  And gate prompt shows AFTER [x] marking
  And developer approves the completed work, not pre-approves

Scenario: Approve continues to next task
  Given gate prompt returns Approve
  When runner processes decision
  Then proceeds to next task in sprint-tasks.md

Scenario: Quit at gate preserves state
  Given gate prompt returns Quit
  When runner processes decision
  Then exits with code 2 (user quit — matches exit code table in Story 1.13)
  And all completed tasks remain [x]
  And incomplete tasks remain [ ]
  And re-run continues from first [ ] (FR12)
```

**Technical Notes:**
- Architecture: runner calls `gates.Prompt` when `cfg.GatesEnabled && task.HasGateTag`
- Scanner (Story 3.2) already detects `[GATE]` tags — this story wires detection to gates.Prompt
- Gate triggers AFTER task completion (execute + review + [x]) — developer approves finished work
- `--gates` flag wired in `cmd/ralph/run.go` (Story 1.3 Cobra structure)
- Skip at gate: runner marks [x] and continues (task already completed by review)

**Prerequisites:** Story 5.1 (gate prompt), Story 3.2 (scanner with GateTag), Story 4.7 (execute→review loop)

---

### Story 5.3: Retry with Feedback

**User Story:**
Как разработчик, я хочу на gate-точке выбрать retry с обратной связью, чтобы AI учёл мои комментарии при повторной реализации задачи.

**Acceptance Criteria:**

```gherkin
Scenario: Retry action prompts for feedback
  Given gate prompt displayed
  When user enters "r"
  Then system prompts for feedback text input
  And user types multi-line feedback (Enter twice to submit)

Scenario: Feedback injected into sprint-tasks.md
  Given user provided feedback "Need to add validation for email field"
  When feedback injection runs
  Then sprint-tasks.md updated with indented line under current task:
    > USER FEEDBACK: Need to add validation for email field
  And uses config.FeedbackPrefix constant (FR22)

Scenario: Execute sees feedback on retry
  Given feedback injected into sprint-tasks.md
  When next execute session launches (fresh session)
  Then Claude reads sprint-tasks.md and sees feedback line
  And addresses feedback in implementation (self-directing model)

Scenario: Retry resets task for re-execution
  Given retry with feedback selected
  When runner processes decision
  Then current task [x] reverted to [ ] in sprint-tasks.md
  And execute_attempts reset to 0
  And review_cycles reset to 0
  And fresh execute cycle starts for this task

Scenario: Ralph writes feedback programmatically
  Given feedback text from user
  When ralph injects feedback
  Then ralph (not Claude) writes the feedback line via os.WriteFile
  And feedback line is indented under the task
  And existing feedback lines preserved (append, not overwrite)

Scenario: GateDecision includes feedback
  Given user chose retry with feedback
  When gates.Prompt returns
  Then GateDecision{Action: Retry, Feedback: "user text"}
  And runner uses Feedback field for injection
```

**Technical Notes:**
- Architecture: "Ralph программно добавляет feedback в sprint-tasks.md"
- Feedback format: `> USER FEEDBACK: <text>` — config.FeedbackPrefix constant
- Ralph writes this line (not Claude) — one of the few places Ralph modifies sprint-tasks.md content
- This does NOT violate Mutation Asymmetry: feedback is content injection, not task status change
- Execute prompt (Story 3.1) already handles self-directing model — Claude reads sprint-tasks.md
- Retry reverts [x] → [ ]: this is the ONLY place Ralph changes task status markers (exception to Mutation Asymmetry, documented)

**Prerequisites:** Story 5.1 (gate prompt with retry action), Story 5.2 (gate detection in runner)

---

### Story 5.4: Checkpoint Gates

**User Story:**
Как разработчик, я хочу периодические checkpoint gates каждые N задач (`--gates --every N`), чтобы регулярно проверять прогресс AI даже без `[GATE]` разметки.

**Acceptance Criteria:**

```gherkin
Scenario: Checkpoint every N tasks
  Given ralph run --gates --every 5
  When 5th task completes (marked [x])
  Then checkpoint gate prompt fires
  And prompt indicates "checkpoint every 5"
  And same options: approve/retry/skip/quit (FR25)

Scenario: Checkpoint counter counts completed tasks
  Given --every 3 configured
  And tasks 1,2 completed, task 3 skipped via [s]kip
  When counting for checkpoint
  Then skipped tasks count toward checkpoint (3 tasks processed)
  And checkpoint fires after task 3

Scenario: Checkpoint independent of [GATE] tags
  Given --every 5 configured
  And task 3 has [GATE] tag
  When task 3 completes
  Then [GATE] gate fires at task 3 (Story 5.2)
  And checkpoint counter continues (3/5)
  And next checkpoint at task 5 (not reset by GATE)

Scenario: Combined GATE + checkpoint — single prompt
  Given --every 5 configured
  And task 5 has [GATE] tag
  When task 5 completes
  Then ONE combined gate prompt (not two)
  And prompt indicates both: "[GATE] + checkpoint every 5"

Scenario: Config file support
  Given config.yaml has gates_checkpoint: 5
  And no --every flag on CLI
  When runner starts
  Then checkpoint every 5 tasks active
  And CLI --every overrides config value

Scenario: --every 0 disables checkpoints
  Given --gates --every 0
  When tasks complete
  Then no checkpoint gates fire
  And only [GATE] tagged tasks trigger gates
```

**Technical Notes:**
- Architecture: "Checkpoint gates: каждые N [x] задач (считает completed, не attempts)"
- CLI: `--gates --every N` — `--every` only valid with `--gates`
- Config: `gates_checkpoint` in YAML, default 0 (off)
- Counter: increment on each task completion (including skip), reset never (cumulative)
- Combined gate: if task has [GATE] AND hits checkpoint, merge into single prompt
- `--every 1` ≈ gate after every task

**Prerequisites:** Story 5.2 (gate detection in runner), Story 1.13 (Cobra flag wiring for --every)

---

### Story 5.5: Emergency Gate Upgrade

**User Story:**
Как разработчик, я хочу чтобы emergency stops (исчерпание попыток execute/review) стали интерактивными gates с опциями retry/feedback/skip/quit, чтобы я мог решить как поступить вместо автоматического прекращения.

**Acceptance Criteria:**

```gherkin
Scenario: Emergency gate replaces stop when gates enabled
  Given gates_enabled = true
  And execute_attempts reaches max_iterations (FR23)
  When emergency triggers
  Then shows interactive emergency gate (not just exit)
  And prompt includes: task info, attempts count, failure context
  And options: [r]etry with feedback, [s]kip task, [q]uit

Scenario: Emergency gate for review cycles when gates enabled
  Given gates_enabled = true
  And review_cycles reaches max_review_iterations (FR24)
  When emergency triggers
  Then shows interactive emergency gate
  And prompt includes: task info, review cycles count, remaining findings
  And options: [r]etry with feedback, [s]kip task, [q]uit

Scenario: Non-interactive stop preserved when gates disabled
  Given gates_enabled = false
  And execute_attempts reaches max_iterations
  When emergency triggers
  Then original behavior: exit code 1 + informative message (Epic 3)
  And no interactive prompt

Scenario: Retry at emergency gate resets counters
  Given emergency gate for execute_attempts
  When developer chooses [r]etry with feedback
  Then execute_attempts resets to 0
  And feedback injected (Story 5.3)
  And fresh execute cycle starts

Scenario: Skip at emergency gate advances to next task
  Given emergency gate displayed
  When developer chooses [s]kip
  Then current task marked [x] (skipped)
  And runner proceeds to next task
  And counters reset for next task
```

**Technical Notes:**
- This story modifies runner loop logic from Stories 3.9/3.10 — adds conditional: `if gatesEnabled { gates.Prompt(emergency) } else { return ErrMaxRetries }`
- Emergency gate prompt has different styling: 🚨 instead of 🚦
- Approve option NOT available at emergency gate (nothing to approve — task failed)
- Feedback from retry goes through same injection as Story 5.3
- Emergency gates use same `gates.Prompt` function — different `GateType` enum value

**Prerequisites:** Story 5.3 (retry with feedback), Story 3.9 (execute emergency stop), Story 3.10 (review emergency stop)

---

### Story 5.6: Gates Integration Test

**User Story:**
Как разработчик, я хочу комплексный integration test gates system, покрывающий все gate types и actions, чтобы гарантировать корректность интерактивного контроля.

**Acceptance Criteria:**

```gherkin
Scenario: Approve at GATE tag
  Given sprint-tasks.md with [GATE] tagged task
  And gates_enabled = true
  And mock stdin returns "a"
  When runner.Run executes
  Then gate prompt fires after task completion
  And approve continues to next task

Scenario: Quit at gate
  Given gate triggered
  And mock stdin returns "q"
  When runner processes quit
  Then exits with code 2 (user quit)
  And state preserved in sprint-tasks.md

Scenario: Retry with feedback
  Given gate triggered
  And mock stdin returns "r" then "fix validation"
  When runner processes retry
  Then feedback injected into sprint-tasks.md
  And task re-executed with feedback visible

Scenario: Skip at gate
  Given gate triggered
  And mock stdin returns "s"
  When runner processes skip
  Then task remains [x] and runner continues

Scenario: Checkpoint gate fires every N
  Given --every 3 and 5 tasks
  And mock stdin returns "a" for all gates
  When runner.Run executes
  Then checkpoint fires after task 3
  And checkpoint fires after task 5 (if > 5 tasks, after 6...)

Scenario: Emergency gate upgrade
  Given gates_enabled = true
  And execute fails max_iterations times
  And mock stdin returns "s" (skip)
  When emergency gate fires
  Then shows emergency prompt (not regular exit)
  And skip advances to next task

Scenario: Gates disabled — no prompts
  Given gates_enabled = false
  And sprint-tasks.md with [GATE] tagged tasks
  When runner.Run executes
  Then no gate prompts fire
  And all tasks executed normally
```

**Technical Notes:**
- Mock stdin: `io.Reader` injection into gates package for testing (not os.Stdin directly)
- gates package should accept `io.Reader` + `io.Writer` for testability
- Build tag: `//go:build integration`
- Test file: `gates/gates_integration_test.go` + runner-level gate scenarios in `runner/runner_gates_integration_test.go`
- Combined GATE + checkpoint scenario covers edge case from Story 5.4

**Prerequisites:** Story 5.1-5.5 (all gate stories), Story 1.11 (MockClaude), Story 3.4 (MockGitClient)

---

### Epic 5 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 5.1 | Basic Gate Prompt | FR21,FR22 | 2 | 6 |
| 5.2 | Gate Detection in Runner | FR20,FR21 | 1 | 6 |
| 5.3 | Retry with Feedback | FR22 | 1 | 6 |
| 5.4 | Checkpoint Gates | FR25 | 1 | 6 |
| 5.5 | Emergency Gate Upgrade | FR23,FR24 | 1 | 5 |
| 5.6 | Gates Integration Test | — | 2 | 7 |
| | **Total** | **FR20-FR22,FR25 + FR23/FR24 upgrade** | | **~36** |

**FR Coverage:** FR20 (5.2), FR21 (5.1, 5.2), FR22 (5.1, 5.3), FR23 upgrade (5.5), FR24 upgrade (5.5), FR25 (5.4)

**Architecture Sections Referenced:** Gates package (gates.go), Subprocess Patterns, CLI UX & Output (fatih/color, io.Reader/io.Writer), Dependency Direction (gates independent of session)

**Dependency Graph:**
```
1.5, 1.6 ──→ 5.1 ──→ 5.2 ──→ 5.3 ──→ 5.5
3.2 ────────→ 5.2     │              ↗
4.7 ────────→ 5.2     ├──→ 5.4    3.9, 3.10
1.3 ────────→ 5.4     │
                       └──→ 5.5 ──→ 5.6
                       5.1-5.5 ──→ 5.6
                       1.11, 3.4 ──→ 5.6
```
Note: 5.3 и 5.4 partially parallel-capable (оба зависят от 5.2, но 5.5 зависит от 5.3)

---

## Epic 6: Knowledge Management & Polish — Stories

**Scope:** FR26, FR27, FR28, FR28a, FR28b, FR29, FR39
**Stories:** 9
**Release milestone:** v0.3

**Context (из Epics Structure Plan + предыдущих эпиков):**
- **KnowledgeWriter interface** определён в Epic 3 Story 3.7: `WriteProgress(ctx, ProgressData) error` + `WriteLessons(ctx, LessonsData) error`
- Epic 3 содержит no-op implementation — Epic 6 заменяет на реальную
- Extensible: добавление полей в structs, не изменение method signatures (Story 3.7 contract)
- FR17 lessons deferred from Epic 4 — реализуются здесь
- `runner/knowledge.go` уже содержит interface + no-op + data structs
- LEARNINGS.md budget: 200 lines hard limit (Architecture)
- Distillation: `claude -p` session (не interactive)
- Serena: best-effort, graceful fallback, modifies `runner.Run()`

---

### Story 6.1: KnowledgeWriter Implementation — LEARNINGS.md

**User Story:**
Как система, я хочу реальную реализацию KnowledgeWriter которая записывает паттерны и выводы в LEARNINGS.md с проверкой бюджета, чтобы знания накапливались между сессиями.

**Acceptance Criteria:**

```gherkin
Scenario: WriteLessons appends to LEARNINGS.md
  Given LEARNINGS.md exists with 50 lines
  And LessonsData contains new lesson content
  When KnowledgeWriter.WriteLessons(ctx, data) called
  Then lesson appended to LEARNINGS.md (FR27)
  And existing content preserved
  And file written via os.WriteFile with 0644

Scenario: WriteLessons creates LEARNINGS.md if absent
  Given LEARNINGS.md does not exist
  When WriteLessons called
  Then creates LEARNINGS.md with lesson content
  And no error

Scenario: Budget check returns line count
  Given LEARNINGS.md has 180 lines
  When BudgetCheck(ctx, learningsPath) called (free function, not interface method)
  Then returns BudgetStatus{Lines: 180, Limit: 200, OverBudget: false}

Scenario: Budget exceeded detection
  Given LEARNINGS.md has 210 lines
  When BudgetCheck(ctx, learningsPath) called
  Then returns BudgetStatus{OverBudget: true}
  And triggers distillation (Story 6.3)

Scenario: Replaces no-op from Epic 3
  Given KnowledgeWriter interface from Story 3.7
  When real implementation provided
  Then same interface methods: WriteProgress + WriteLessons
  And ProgressData/LessonsData structs extended with new fields (not renamed)
  And no-op impl removed or replaced

Scenario: Thread-safe writes
  Given multiple callers could write lessons
  When concurrent writes attempted
  Then writes are serialized (simple mutex or sequential calls)
  And no data corruption
```

**Technical Notes:**
- Architecture: `runner/knowledge.go` — LEARNINGS.md append, budget check (200 lines hard limit)
- Extends Epic 3 structs: `LessonsData` gets `Source string` field (resume/review/distillation)
- Line counting: `strings.Count(content, "\n")` — simple, no parsing
- Budget = 200 lines (Architecture constant, could be in config later)
- File path: `{projectRoot}/LEARNINGS.md` via config
- BudgetCheck is a free function `BudgetCheck(ctx, path) (BudgetStatus, error)`, NOT a KnowledgeWriter method — preserves "max 2 methods" interface contract from Epic 3

**Prerequisites:** Story 3.7 (KnowledgeWriter interface + no-op)

---

### Story 6.2: Distillation Prompt Template

**User Story:**
Как система, я хочу distillation-промпт который инструктирует Claude сжать раздутый LEARNINGS.md, сохраняя ключевые паттерны и удаляя дублирование.

**Acceptance Criteria:**

```gherkin
Scenario: Distillation prompt assembled
  Given LEARNINGS.md content (over budget)
  When distillation prompt assembled
  Then contains: full LEARNINGS.md content for compression
  And instructs: preserve key patterns, remove duplicates, merge similar lessons
  And instructs: output compressed LEARNINGS.md (replace entirely)
  And instructs: target under 200 lines budget
  And instructs: keep chronological grouping where meaningful

Scenario: Golden file snapshot
  Given distillation prompt template in runner/prompts/distillation.md
  When assembled with test fixture
  Then matches runner/testdata/TestPrompt_Distillation.golden
  And updateable via `go test -update`

Scenario: Distillation uses claude -p (non-interactive)
  Given distillation prompt
  When session invoked
  Then uses `claude -p` flag (pipe mode, non-interactive)
  And no --resume, no --max-turns (single-shot)
```

**Technical Notes:**
- Architecture: `runner/prompts/distillation.md` — Go template
- `claude -p` = pipe mode: stdin prompt → stdout response → exit
- Distillation is a compression task — input = full LEARNINGS.md, output = compressed version
- session.Execute with pipe mode option (new session type variant)
- Template is simpler than execute/review — no sub-agents, no sprint-tasks

**Prerequisites:** Story 1.10 (prompt assembly pattern)

---

### Story 6.3: Distillation Trigger

**User Story:**
Как runner, после clean review я хочу проверить размер LEARNINGS.md и при превышении бюджета запустить distillation session с backup, чтобы файл оставался компактным и полезным.

**Acceptance Criteria:**

```gherkin
Scenario: Budget check after clean review triggers distillation
  Given clean review completed (task marked [x])
  And LEARNINGS.md has 220 lines (exceeds 200 budget)
  When runner checks budget (FR28a)
  Then triggers distillation session

Scenario: Backup before distillation
  Given distillation about to start
  When backup created
  Then copies LEARNINGS.md to LEARNINGS.md.bak (byte-for-byte)
  And backup exists before distillation session runs
  And distillation backup preserved until next successful distillation

Scenario: Distillation replaces LEARNINGS.md
  Given distillation session completes successfully
  When output received
  Then LEARNINGS.md replaced with compressed content
  And new line count under 200 budget
  And backup remains as safety net

Scenario: Distillation failure preserves original
  Given distillation session fails (non-zero exit)
  When error handled
  Then original LEARNINGS.md.bak restored
  And warning logged
  And runner continues (distillation failure is non-fatal)

Scenario: No distillation when under budget
  Given clean review completed
  And LEARNINGS.md has 150 lines
  When runner checks budget
  Then no distillation triggered
  And runner proceeds to next task

Scenario: First clean review without prior LEARNINGS — no distillation
  Given clean review completed
  And LEARNINGS.md does not exist
  When runner checks budget
  Then no distillation needed
  And runner proceeds normally
```

**Technical Notes:**
- Architecture: "Distillation: отдельная `claude -p` сессия при превышении бюджета"
- Backup = distillation backup requirement (Graph of Thoughts finding from epics elicitation)
- Distillation is non-fatal: failure → restore backup, log warning, continue
- Trigger point: after clean review, before advancing to next task
- Budget check uses `BudgetCheck()` free function from Story 6.1 (not a KnowledgeWriter method)

**Prerequisites:** Story 6.1 (budget check), Story 6.2 (distillation prompt)

---

### Story 6.4: CLAUDE.md Section Management

**User Story:**
Как система, я хочу безопасно читать и обновлять ТОЛЬКО секцию `## Ralph operational context` в CLAUDE.md, не затрагивая остальное содержимое проекта.

**Acceptance Criteria:**

```gherkin
Scenario: Read ralph section from CLAUDE.md
  Given CLAUDE.md exists with multiple sections including "## Ralph operational context"
  When ReadRalphSection(ctx) called
  Then returns content between "## Ralph operational context" and next ## heading (or EOF)
  And does not return other sections

Scenario: Write ralph section preserves other content
  Given CLAUDE.md has: ## Project setup, ## Ralph operational context, ## Guidelines
  When WriteRalphSection(ctx, newContent) called
  Then "## Ralph operational context" section replaced with newContent
  And "## Project setup" section unchanged
  And "## Guidelines" section unchanged

Scenario: Create section if missing
  Given CLAUDE.md exists but has no "## Ralph operational context" section
  When WriteRalphSection called
  Then appends "## Ralph operational context" + content at end of file
  And existing content preserved

Scenario: Create CLAUDE.md if missing
  Given CLAUDE.md does not exist
  When WriteRalphSection called
  Then creates CLAUDE.md with "## Ralph operational context" section
  And content written correctly

Scenario: Section boundary detection
  Given "## Ralph operational context" followed by "## Another section"
  When reading ralph section
  Then stops at "## Another section" heading
  And does not include content from other sections
```

**Technical Notes:**
- Architecture: "CLAUDE.md: обновление ТОЛЬКО секции `## Ralph operational context`"
- Section detection: line scanning for `## Ralph operational context` heading
- End detection: next `## ` heading or EOF
- File path: `{projectRoot}/CLAUDE.md` via config (paths.claude_md)
- This is utility code used by both resume-extraction knowledge (Story 6.6) and review knowledge (Story 6.7)
- FR26: "Существующий контент проекта вне секции ralph не затрагивается"

**Prerequisites:** None (utility code, no epic dependencies)

---

### Story 6.5: Knowledge Loading in Session Context

**User Story:**
Как execute и review сессии, я хочу чтобы LEARNINGS.md и ralph section из CLAUDE.md загружались в prompt assembly, чтобы каждая сессия имела доступ к накопленным знаниям.

**Acceptance Criteria:**

```gherkin
Scenario: Execute prompt includes LEARNINGS.md content
  Given LEARNINGS.md exists with lessons
  When execute prompt assembled (Story 3.1)
  Then LEARNINGS.md content injected via strings.Replace (FR29)
  And content available to Claude in session context

Scenario: Execute prompt includes ralph section from CLAUDE.md
  Given CLAUDE.md has "## Ralph operational context" section
  When execute prompt assembled
  Then ralph section content injected via strings.Replace (FR29)

Scenario: Review prompt includes knowledge files
  Given LEARNINGS.md and CLAUDE.md ralph section exist
  When review prompt assembled (Story 4.1)
  Then both contents injected into review prompt (FR29)

Scenario: Missing knowledge files handled gracefully
  Given LEARNINGS.md does not exist
  And CLAUDE.md has no ralph section
  When prompts assembled
  Then knowledge placeholders replaced with empty string
  And no error

Scenario: Golden file update with knowledge injection
  Given execute prompt golden file from Story 3.1
  When knowledge injection added
  Then golden file updated to include knowledge sections
  And `go test -update` refreshes baseline
```

**Technical Notes:**
- Architecture: "Knowledge files загружаются в prompt assembly (strings.Replace)"
- Modifies execute prompt template (Story 3.1) and review prompt template (Story 4.1) — adds knowledge placeholders
- Uses strings.Replace stage 2 (user content injection) — safe from template injection
- ReadRalphSection from Story 6.4 used to extract CLAUDE.md section
- LEARNINGS.md read via os.ReadFile (whole file, small)
- Config paths: `paths.learnings_md`, `paths.claude_md`
- NFR14: log file events should include: session type (execute/review/resume), task name, duration, outcome (commit/findings/error)

**Prerequisites:** Story 6.4 (CLAUDE.md section reader), Story 3.1 (execute prompt), Story 4.1 (review prompt)

---

### Story 6.6: Resume-Extraction Knowledge

**User Story:**
Как resume-extraction сессия, я хочу записывать причины неудачи в LEARNINGS.md и обновлять ralph section в CLAUDE.md, чтобы будущие сессии учились на ошибках.

**Acceptance Criteria:**

```gherkin
Scenario: Resume-extraction writes to LEARNINGS.md
  Given resume-extraction completed (Story 3.7)
  When KnowledgeWriter.WriteLessons called with source="resume"
  Then failure reasons appended to LEARNINGS.md (FR28)
  And lessons include: what was attempted, where stuck, extracted insights

Scenario: Resume-extraction updates CLAUDE.md ralph section
  Given resume-extraction completed
  When WriteRalphSection called
  Then ralph operational context updated with failure insights (FR26, FR28)
  And existing project content preserved

Scenario: Replaces no-op behavior from Epic 3
  Given Epic 3 KnowledgeWriter no-op returned nil
  When Epic 6 real implementation active
  Then WriteLessons actually writes to LEARNINGS.md
  And WriteProgress still writes progress to sprint-tasks.md (unchanged from Epic 3)

Scenario: Resume-extraction prompt updated with knowledge instructions
  Given resume-extraction invoked via --resume
  When prompt assembled
  Then includes instructions to extract failure insights
  And includes instructions to write to LEARNINGS.md
  And includes instructions to update CLAUDE.md ralph section
```

**Technical Notes:**
- This story replaces no-op from Epic 3 Story 3.7 with real KnowledgeWriter
- FR28: resume-extraction пишет причины неудачи + извлечённые знания
- Resume-extraction prompt needs update to include knowledge-writing instructions
- Claude inside resume-extraction session reads/writes LEARNINGS.md and CLAUDE.md directly
- Ralph's KnowledgeWriter provides Go-side budget check; Claude does the actual writing

**Prerequisites:** Story 6.1 (KnowledgeWriter impl), Story 6.4 (CLAUDE.md section), Story 3.7 (resume-extraction)

---

### Story 6.7: Review Knowledge

**User Story:**
Как review сессия с findings, я хочу записывать уроки (типы ошибок, упускаемые паттерны) в LEARNINGS.md и обновлять CLAUDE.md, чтобы будущие execute сессии не повторяли те же ошибки.

**Acceptance Criteria:**

```gherkin
Scenario: Review with findings writes lessons
  Given review found CONFIRMED findings
  When review session processes findings (FR28a)
  Then lessons appended to LEARNINGS.md
  And lessons include: error types, what agent forgets, patterns for future sessions
  And ralph section in CLAUDE.md updated

Scenario: Clean review does NOT write lessons
  Given review is clean (no findings)
  When review session completes
  Then no new content added to LEARNINGS.md
  And CLAUDE.md not modified (beyond [x] + clear findings)

Scenario: Review prompt updated with knowledge instructions
  Given review prompt from Story 4.1
  When Epic 6 integration
  Then prompt includes: write lessons to LEARNINGS.md on findings
  And prompt includes: update ralph section in CLAUDE.md on findings
  And prompt includes: do NOT write lessons on clean review

Scenario: FR17 lessons scope now implemented
  Given FR17 lessons deferred from Epic 4
  When Epic 6 review knowledge active
  Then review writes lessons on findings (previously deferred)
  And review writes [x] + clears findings on clean (unchanged from Epic 4)
```

**Technical Notes:**
- FR28a: "Review-сессия при наличии findings сама записывает уроки в LEARNINGS.md"
- This completes the FR17 deferred scope from Epic 4
- Review prompt (Story 4.1) gets additional instructions for knowledge writing
- Claude inside review session does the actual writing — not Ralph's Go code
- Budget check + distillation trigger after clean review (Story 6.3)

**Prerequisites:** Story 6.1 (KnowledgeWriter), Story 6.4 (CLAUDE.md section), Story 4.1 (review prompt)

---

### Story 6.8: Serena Integration

**User Story:**
Как runner, я хочу обнаруживать Serena MCP и использовать её для индексации проекта перед execute сессиями, чтобы улучшить token economy и review accuracy.

**Acceptance Criteria:**

```gherkin
Scenario: Serena detected at startup
  Given Serena MCP available in environment
  When ralph run starts
  Then detects Serena availability
  And logs "Serena MCP detected"

Scenario: Full index at ralph run startup
  Given Serena detected
  When runner initializes
  Then triggers Serena full index (FR39)
  And timeout: 60 seconds
  And progress output displayed

Scenario: Incremental index before each execute
  Given Serena available
  When execute session about to launch
  Then triggers Serena incremental index
  And timeout: configurable (default 10s from config)
  And progress output displayed

Scenario: Timeout graceful fallback
  Given Serena full index running
  When 60 second timeout exceeded
  Then cancels index operation
  And outputs "Serena timeout — falling back to standard file reading"
  And runner continues without Serena index

Scenario: Serena unavailable graceful fallback
  Given Serena MCP not available in environment
  When ralph run starts
  Then outputs "Serena MCP not available — using standard file reading"
  And runner operates normally without Serena
  And no error

Scenario: Serena configurable
  Given config with serena_enabled: false
  When ralph run starts
  Then skips Serena detection entirely
  And no Serena-related output

Scenario: Serena timeout configurable
  Given config with serena_timeout: 20
  When incremental index runs
  Then uses 20s timeout instead of default 10s
```

**Technical Notes:**
- Architecture: "Serena: detect → full index (60s timeout) → incremental (10s) → graceful fallback"
- Modifies `runner.Run()` from Epic 3 — adds Serena calls before execute
- Detection: check if `serena` CLI available via `exec.LookPath` or similar
- Full index: `serena index --full` (or equivalent CLI command)
- Incremental: `serena index` (or equivalent)
- All Serena calls via `exec.CommandContext(ctx)` with timeout context
- Config: `serena_enabled` (default true), `serena_timeout` (default 10s)
- Best-effort: any Serena failure = log warning + continue

**Prerequisites:** Story 3.5 (runner loop — insertion point for Serena calls)

---

### Story 6.9: --always-extract Flag + Final Integration Test

**User Story:**
Как разработчик, я хочу `--always-extract` для извлечения знаний из каждого execute, и финальный end-to-end integration test всего продукта, чтобы все 6 эпиков работали вместе.

**Acceptance Criteria:**

```gherkin
Scenario: --always-extract runs resume-extraction after every execute
  Given ralph run --always-extract
  And execute session completes with commit (success)
  When runner processes successful execute
  Then resume-extraction still runs (FR28b)
  And extracts knowledge from successful execution process
  And writes to LEARNINGS.md + CLAUDE.md

Scenario: Without --always-extract — only on failure
  Given ralph run without --always-extract
  And execute session completes with commit
  When runner processes result
  Then NO resume-extraction (standard behavior)
  And proceeds directly to review

Scenario: Config file support for always-extract
  Given config.yaml has always_extract: true
  And no --always-extract flag
  When runner starts
  Then always-extract enabled
  And CLI flag overrides config value

Scenario: FINAL — full end-to-end flow
  Given scenario JSON covering full flow:
    bridge → execute (commit) → review (findings) → execute fix (commit) → review (clean) → knowledge written → Serena indexed
  And MockClaude + MockGitClient + mock Serena
  And sprint-tasks.md from bridge golden file
  When runner.Run executes with all features
  Then bridge output feeds runner
  And execute sessions launch with knowledge context
  And review finds and verifies findings
  And fix cycle produces clean review
  And [x] marked + review-findings cleared
  And LEARNINGS.md written with lessons
  And CLAUDE.md ralph section updated
  And Serena incremental index called before each execute
  And budget check runs after clean review
  And all 6 epics work together

Scenario: FINAL — gates + knowledge + emergency
  Given gates_enabled = true, --every 2
  And scenario with 3 tasks: task1 (clean), task2 (emergency→skip), task3 (clean)
  And mock stdin for gate actions
  When runner.Run executes
  Then checkpoint gate fires after task 2
  And emergency gate fires for task 2 (max retries)
  And skip advances to task 3
  And knowledge written throughout

Scenario: FINAL — resume + always-extract
  Given --always-extract enabled
  And scenario: execute (no commit) → resume-extraction → retry → execute (commit) → always-extract
  When runner.Run executes
  Then resume-extraction runs on failure (writes knowledge)
  And always-extract runs on success (extracts positive knowledge)
  And LEARNINGS.md accumulates from both sources
```

**Technical Notes:**
- `--always-extract` flag: `cmd/ralph/run.go` Cobra flag, config key `always_extract` (default false)
- Modifies runner loop: after successful commit detection, if always_extract → run resume-extraction
- FINAL integration test: most comprehensive test in the project — covers all 6 epics
- Test file: `runner/runner_final_integration_test.go`
- Build tag: `//go:build integration`
- Mock Serena: mock binary that exits 0 (or test without Serena for simplicity)
- This is the "Bob's Final Integration Test" from epics structure plan

**Prerequisites:** Story 6.1-6.8 (all Epic 6 stories), Story 5.6 (gates integration), Story 4.8 (review integration), Story 3.11 (runner integration)

---

### Epic 6 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 6.1 | KnowledgeWriter — LEARNINGS.md | FR27 | 1 | 6 |
| 6.2 | Distillation Prompt Template | FR28a | 2 + testdata | 3 |
| 6.3 | Distillation Trigger | FR28a | 1 | 6 |
| 6.4 | CLAUDE.md Section Management | FR26 | 1 | 5 |
| 6.5 | Knowledge Loading in Context | FR29 | 2 (prompt updates) | 5 |
| 6.6 | Resume-Extraction Knowledge | FR28 | 1 (prompt update) | 4 |
| 6.7 | Review Knowledge | FR28a | 1 (prompt update) | 4 |
| 6.8 | Serena Integration | FR39 | 1 | 7 |
| 6.9 | --always-extract + Final Test | FR28b | 2 | 6 |
| | **Total** | **FR26-FR29,FR28a,FR28b,FR39** | | **~46** |

**FR Coverage:** FR26 (6.4, 6.6, 6.7), FR27 (6.1), FR28 (6.6), FR28a (6.3, 6.7), FR28b (6.9), FR29 (6.5), FR39 (6.8)

**Architecture Sections Referenced:** Runner package (knowledge.go, prompts/distillation.md), Package Boundaries (LEARNINGS.md, CLAUDE.md writers), Subprocess Patterns (Serena CLI), File I/O Patterns, Data Flow

**Dependency Graph:**
```
3.7 ────→ 6.1 ──→ 6.3
1.10 ───→ 6.2 ──→ 6.3
                    │
6.4 (independent) ──┤
                    │
3.1 ──→ 6.5 ←── 6.4
4.1 ──→ 6.5
          │
6.1, 6.4 → 6.6 ←── 3.7
6.1, 6.4 → 6.7 ←── 4.1
          │
3.5 ────→ 6.8 (independent of knowledge)
          │
6.1-6.8 → 6.9
```
Note: 6.4 полностью independent (нет epic dependencies). 6.2 и 6.4 parallel-capable. 6.6 и 6.7 parallel-capable. 6.8 independent от knowledge stories

---

## FR Coverage Matrix

| FR | Описание | Stories | Epic |
|:--:|---------|---------|:----:|
| FR1 | Bridge: story → sprint-tasks.md | 2.2, 2.3 | 2 |
| FR2 | Bridge prompt template | 2.2 | 2 |
| FR3 | Service tasks + gate marking | 2.4 | 2 |
| FR4 | Smart merge (incremental) | 2.6 | 2 |
| FR5 | Source traceability | 2.4 | 2 |
| FR5a | Story path from Cobra args | 2.4 | 2 |
| FR6 | Sequential execution + git health | 3.2, 3.3, 3.5 | 3 |
| FR7 | Fresh session per task | 3.5 | 3 |
| FR8 | Execute + commit on green | 3.1, 3.5 | 3 |
| FR9 | Retry + resume-extraction | 3.6, 3.7 | 3 |
| FR10 | Configurable max turns | 3.5 | 3 |
| FR11 | Self-directing execute | 3.1, 3.2, 3.5 | 3 |
| FR12 | Resume from incomplete | 3.2, 3.4, 3.8 | 3 |
| FR13 | Review after task | 4.1, 4.3 | 4 |
| FR14 | Fresh review session | 4.3 | 4 |
| FR15 | 4 parallel sub-agents | 4.1, 4.2 | 4 |
| FR16 | Findings verification | 4.1, 4.4 | 4 |
| FR17 | Review writes [x] / findings | 4.1, 4.5, 4.6 (+ 6.7 lessons) | 4, 6 |
| FR18 | Execute→review fix loop | 4.7 | 4 |
| FR18a | Review iteration limit | 4.7 | 4 |
| FR20 | --gates flag | 5.2 | 5 |
| FR21 | Stop at gate points | 5.1, 5.2 | 5 |
| FR22 | Approve/retry/skip/quit | 5.1, 5.3 | 5 |
| FR23 | Emergency gate — execute | 3.9 (stop), 5.5 (upgrade) | 3, 5 |
| FR24 | Emergency gate — review | 3.10 (stop), 5.5 (upgrade) | 3, 5 |
| FR25 | Checkpoint gates | 5.4 | 5 |
| FR26 | CLAUDE.md section | 6.4, 6.6, 6.7 | 6 |
| FR27 | LEARNINGS.md | 6.1 | 6 |
| FR28 | Resume-extraction knowledge | 6.6 | 6 |
| FR28a | Review lessons + distillation | 6.3, 6.7 | 6 |
| FR28b | --always-extract | 6.9 | 6 |
| FR29 | Knowledge in session context | 6.5 | 6 |
| FR30 | Config файл .ralph/config.yaml | 1.3 | 1 |
| FR31 | CLI flags override config | 1.4 | 1 |
| FR32 | Custom review-agent prompts | 1.5 | 1 |
| FR33 | Fallback chain (project → global → embedded) | 1.5 | 1 |
| FR34 | Per-agent model config | 1.5 | 1 |
| FR35 | Exit codes (0-4) | 1.13 | 1 |
| FR36 | 999-rules guardrails | 3.1 | 3 |
| FR37 | ATDD enforcement | 3.1, 4.2 | 3, 4 |
| FR38 | Zero test skip | 3.1, 4.2 | 3, 4 |
| FR39 | Serena integration | 6.8 | 6 |

**Coverage: 42/42 MVP FR covered (100%)**
**Growth FR (not in scope): FR16a, FR19, FR40, FR41**

---

## Summary

### Final Statistics

- **6 epics**, **54 stories**, **~289 acceptance criteria**
- **42/42 MVP FR covered** (100%)
- **3 release milestones:** v0.1 (Epics 1-4), v0.2 (+Epic 5), v0.3 (+Epic 6)
- **Average story size:** ~5.4 AC per story (target: single dev agent session ~25-35 turns)

### Key Architectural Invariants Enforced

1. **Mutation Asymmetry:** Execute MUST NOT modify sprint-tasks.md task status — only review marks [x]
2. **Review Atomicity:** [x] + clear review-findings.md = atomic operation
3. **FR17 Lessons Deferred:** v0.1 review = [x] + findings only; lessons writing → Epic 6
4. **KnowledgeWriter Contract:** Minimal interface (Epic 3 no-op → Epic 6 real impl), extensible via struct fields
5. **Emergency Gates Progressive:** Epic 3 = minimal stop; Epic 5 = interactive gate upgrade
6. **sprint-tasks.md = Hub Node:** 5 writers/readers, single source of state

### Quality Gates

- **Per-epic:** Party Mode review (3 agents) + Advanced Elicitation (multiple methods)
- **v0.1 gate:** Manual smoke test checklist (`runner/testdata/manual_smoke_checklist.md`)
- **Final gate:** End-to-end integration test (Story 6.9) covering all 6 epics together
- **Adversarial tests:** Review prompt quality validated via planted-bug detection + false-positive resistance

---

_For implementation: Use the `create-story` workflow to generate individual story implementation plans from this epic breakdown._

_This document will be updated after UX Design and Architecture workflows to incorporate interaction details and technical decisions._
