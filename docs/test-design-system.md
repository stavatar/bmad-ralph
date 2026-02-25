# System-Level Test Design — bmad-ralph

**Дата:** 2026-02-25
**Автор:** Степан (TEA Agent)
**Режим:** System-Level (Phase 3 — Solutioning Testability Review)
**Статус:** Draft

---

## Executive Summary

**Проект:** bmad-ralph — Go CLI-утилита (single binary), оркестрирующая Claude Code сессии для автономной разработки через Ralph Loop.

**Архитектура:** 6 Go packages (config → session → gates → bridge → runner → cmd/ralph), 3 external deps (Cobra, yaml.v3, fatih/color), subprocess management через `os/exec`.

**Оценка тестируемости:** PASS с замечаниями. Архитектура хорошо спроектирована для тестирования: interfaces для mocking (GitClient), injectable Claude command, scenario-based mock, golden files. Основное ограничение — невозможность unit-тестирования качества промптов и поведения LLM.

---

## Testability Assessment

### Controllability: PASS

Можем ли мы контролировать состояние системы для тестирования?

| Аспект | Оценка | Обоснование |
|--------|:------:|-------------|
| **State seeding** | ✅ | sprint-tasks.md, review-findings.md, LEARNINGS.md — plain text файлы, легко создаются в `t.TempDir()` |
| **External deps mocking** | ✅ | `GitClient` interface → `MockGitClient`. Claude CLI → scenario-based mock script через `config.ClaudeCommand` |
| **Error injection** | ✅ | Mock Claude возвращает заданные exit codes, mock git возвращает ошибки по сценарию |
| **Config override** | ✅ | `config.Config` struct — immutable после load, передаётся by pointer. Тесты конструируют config напрямую |
| **Prompt control** | ✅ | go:embed per-package + fallback chain. Тесты используют embedded defaults, golden files фиксируют snapshot |
| **Signal injection** | ✅ | `context.WithCancel` позволяет симулировать Ctrl+C в тестах без реальных сигналов |

**Вердикт:** Архитектура обеспечивает полную controllability через interfaces, injectable dependencies и file-based state.

### Observability: PASS с замечаниями

Можем ли мы инспектировать состояние системы?

| Аспект | Оценка | Обоснование |
|--------|:------:|-------------|
| **Return values** | ✅ | Packages возвращают results/errors, не логируют. Вся информация доступна через return values |
| **Exit codes** | ✅ | Cross-component contract (0-4). Каждый exit code тестируем через mock scenarios |
| **File state** | ✅ | sprint-tasks.md, review-findings.md, LEARNINGS.md — читаемы после каждого шага |
| **Git state** | ✅ | MockGitClient возвращает предопределённые ответы. Prod: `git log`, `git diff` через GitClient |
| **Claude session results** | ✅ | `--output-format json` → SessionResult struct (session_id, exit_code, output). Golden file тесты на JSON parsing |
| **Log file** | ⚠️ | MVP: простой text log (timestamp + level + msg). Нет structured log (Growth). Достаточно для post-mortem, но automated analysis затруднён |
| **Prompt quality** | ⚠️ | Golden file фиксирует структуру промпта, но НЕ гарантирует корректное поведение LLM. Prompt drift detection через snapshot — да. LLM behavior validation — нет (только real Claude smoke tests в Growth) |

**Вердикт:** Observability достаточна для MVP. Structured logging — Growth enhancement.

### Reliability (Test Isolation): PASS

Можем ли мы обеспечить надёжные, изолированные тесты?

| Аспект | Оценка | Обоснование |
|--------|:------:|-------------|
| **Test isolation** | ✅ | `t.TempDir()` для каждого теста. Нет shared state |
| **Determinism** | ✅ | Scenario-based mock Claude возвращает ответы по порядку. Нет race conditions в тестах |
| **Parallelism** | ✅ | `t.TempDir()` + no global state → `go test -parallel` безопасно |
| **Reproducibility** | ✅ | Golden files фиксируют expected output. `go test -update` для осознанного обновления |
| **Component coupling** | ✅ | Packages loosely coupled. config = leaf, session и gates не зависят друг от друга. Interfaces для mocking |
| **Cleanup** | ✅ | `t.TempDir()` автоматически удаляет temp dirs. Нет базы данных, нет network state |

**Вердикт:** Архитектура обеспечивает полную изоляцию тестов без специальных усилий.

---

## Architecturally Significant Requirements (ASRs)

Требования, которые определяют архитектурные решения и создают testability challenges.

| ASR ID | NFR/FR | Описание | Probability | Impact | Score | Тестовый challenge |
|--------|--------|----------|:-----------:|:------:|:-----:|-------------------|
| ASR-1 | NFR1 | Context window ≤40-50% per execute session | 2 | 3 | **6** | Невозможно автоматически измерить token usage. Proxy: LEARNINGS.md budget (200 строк), --max-turns как hard limit |
| ASR-2 | NFR10 | Crash recovery через sprint-tasks.md | 2 | 3 | **6** | Integration test: simulate crash (mock Claude exit != 0) → restart → verify resume from first `- [ ]` |
| ASR-3 | NFR13 | Graceful shutdown при Ctrl+C | 2 | 2 | **4** | Integration test: cancel context → verify Claude process termination → verify sprint-tasks.md не повреждён |
| ASR-4 | NFR2 | Overhead <5s между итерациями | 1 | 2 | **2** | Benchmark test: `go test -bench` на runner scanning + state check. Не критично — Go native performance |
| ASR-5 | NFR16-17 | Single binary, zero runtime deps | 1 | 3 | **3** | CI build: `go build` succeeds. `ldd` check на Linux binary. goreleaser cross-compile matrix |
| ASR-6 | FR9 | Resume-extraction при failure execute | 3 | 2 | **6** | Integration test: mock Claude fails (no commit) → resume-extraction → WIP commit → verify LEARNINGS.md updated |
| ASR-7 | FR15 | 4 параллельных review sub-агента | 2 | 2 | **4** | Review — Claude session (subprocess). Тестируем: промпт содержит Task tool instructions. Sub-agent behavior не тестируемо без real Claude |
| ASR-8 | FR36 | 999-rules guardrails в execute prompt | 2 | 3 | **6** | Golden file: промпт содержит 999-rules. Behavioral effectiveness — НЕ тестируемо unit/integration. Только real Claude smoke test (Growth) |
| ASR-9 | FR4 | Smart Merge не сбрасывает `[x]` | 2 | 3 | **6** | Golden file тесты: input sprint-tasks.md с `[x]` + new stories → output сохраняет `[x]`. Regression test |

**High-priority ASRs (Score ≥6): ASR-1, ASR-2, ASR-6, ASR-8, ASR-9** — требуют immediate mitigation через тесты.

---

## Test Levels Strategy

### Рекомендуемый split: 70% Unit / 25% Integration / 5% E2E (Growth)

Обоснование: Go CLI-утилита без UI, без базы данных, без network. Основная сложность — subprocess orchestration и file-based state management.

### Unit Tests (70%)

**Инструмент:** Go built-in `testing`
**Что тестируем:**

| Package | Что тестируется | Примеры |
|---------|----------------|---------|
| `config` | YAML parsing, defaults cascade, CLI override, fallback chain | `TestConfig_Default`, `TestConfig_Override`, `TestConfig_FallbackChain` |
| `config` | Constants (TaskOpen, TaskDone, GateTag), regex patterns | `TestConstants_Patterns` |
| `runner/scan.go` | sprint-tasks.md scanning: `- [ ]`, `- [x]`, `[GATE]`, feedback | `TestScan_OpenTasks`, `TestScan_GateTags`, `TestScan_EmptyFile` |
| `runner/knowledge.go` | LEARNINGS.md budget check (200 lines), distillation trigger | `TestKnowledge_BudgetCheck`, `TestKnowledge_OverBudget` |
| `runner/git.go` | Git health check (clean tree, not detached), dirty recovery | `TestGit_HealthCheck_Clean`, `TestGit_HealthCheck_Dirty` (via MockGitClient) |
| `session/result.go` | JSON parsing Claude output → SessionResult | `TestResult_ParseJSON`, `TestResult_TruncatedJSON`, `TestResult_EmptyOutput` |
| `gates` | Gate decision logic (approve/retry/skip/quit) | `TestGate_Approve`, `TestGate_RetryWithFeedback` |
| `bridge` | Prompt template assembly | `TestBridge_PromptAssembly` |
| `runner` | Prompt template assembly (execute, review) | `TestPrompt_Execute`, `TestPrompt_Review` |

**Характеристики:**
- Быстрое выполнение (ms)
- Нет внешних зависимостей
- Table-driven по умолчанию (`[]struct{name; ...}` + `t.Run`)
- Golden files для промптов и bridge output
- `t.TempDir()` для каждого теста

### Integration Tests (25%)

**Инструмент:** Go built-in `testing` + mock Claude (scenario-based)
**Что тестируем:**

| Сценарий | Что проверяем | Mock scenario |
|----------|--------------|---------------|
| **Happy path** | Execute → commit → review clean → `[x]` | `happy_path.json`: execute(exit=0, commit=true) → review(exit=0, clean) |
| **Review findings** | Execute → commit → review finds issues → execute fix → review clean | `review_findings.json`: execute → review(findings) → execute(fix) → review(clean) |
| **Max retries** | Execute fails N times → emergency stop | `max_retries.json`: execute(exit=1, no commit) × N |
| **Resume-extraction** | Execute fails → resume-extraction → WIP commit → retry | `resume_extraction.json`: execute(exit=1) → resume(WIP commit) → retry |
| **Dirty state recovery** | Dirty working tree → git checkout → resume | Pre-condition: dirty files in t.TempDir() |
| **Human gate** | Gate detected → prompt user → continue/quit | `gate_approve.json`, `gate_quit.json` |
| **Graceful shutdown** | Context cancelled → Claude terminated → clean exit | Cancel context mid-execution |
| **Bridge happy path** | Stories → sprint-tasks.md with correct format | `bridge_basic.json`: Claude generates tasks |
| **Bridge smart merge** | Existing sprint-tasks.md + new stories → merged output | `bridge_merge.json`: preserved `[x]` tasks |
| **Knowledge budget** | LEARNINGS.md over 200 lines → distillation triggered | Setup file >200 lines → verify distillation session call |

**Характеристики:**
- Mock Claude: Go script, scenario JSON, ordered responses
- MockGitClient: implements GitClient interface
- Full loop тесты через `runner.Run(ctx, cfg)`
- Средняя скорость (секунды)

### E2E / Smoke Tests (5% — Growth)

**Инструмент:** Go integration tests с real Claude CLI
**Что тестируем:**

| Сценарий | Что проверяем |
|----------|--------------|
| **Claude JSON format** | Real `claude -p "echo test" --output-format json` → valid JSON → correct parsing |
| **Claude --resume** | Real session → terminate → `claude --resume <id>` → session continues |
| **One-task sprint** | Real bridge + run: 1 simple task → execute → review → `[x]` |

**Предпосылки:**
- Требуется реальный `claude` CLI и API key
- Запуск только в CI с секретами (не в обычном `go test`)
- Growth feature, не блокирует MVP

---

## NFR Testing Approach

### Security

**Relевантность для CLI-утилиты: LOW.** bmad-ralph не обрабатывает пользовательский ввод из сети, не хранит секреты, не имеет аутентификации.

| NFR | Подход | Инструмент |
|-----|--------|------------|
| NFR4 (нет деструктивных git ops) | Промпт 999-rules содержит запреты. Golden file тест на наличие правил в промпте | Go `testing` + golden files |
| NFR5 (нет API keys в config) | Scan config struct на отсутствие чувствительных полей | Code review + static analysis |
| NFR6 (`--dangerously-skip-permissions`) | Тест: session.Execute передаёт flag | Unit test session package |

**Митигация:** 999-rules в промпте — единственный security barrier. Effectiveness не тестируема без real Claude. Рекомендация: adversarial golden files (prompt содержит запрет на `git push --force`, `rm -rf`).

### Performance

**Релевантность: LOW-MEDIUM.** Bottleneck — Claude API, не ralph.

| NFR | Подход | Инструмент |
|-----|--------|------------|
| NFR2 (overhead <5s) | Benchmark: scanning sprint-tasks.md (1000 lines) < 10ms | `go test -bench` |
| NFR1 (context 40-50%) | Proxy: LEARNINGS.md budget (200 lines ≈ 3,500 tokens < 2% window). Промпт размер через golden file | Unit test knowledge.go |

**Нет нужды в k6 или load testing** — CLI-утилита, однопользовательский режим.

### Reliability

**Релевантность: HIGH.** Crash recovery и graceful shutdown — критические пути.

| NFR | Подход | Инструмент |
|-----|--------|------------|
| NFR10 (crash recovery) | Integration: mock crash → resume → verify state | Scenario-based mock |
| NFR11 (atomic commits) | Integration: verify commit only on green tests (mock) | Scenario-based mock |
| NFR12 (Claude CLI retry) | Integration: mock Claude returns errors → verify backoff | Scenario-based mock |
| NFR13 (graceful shutdown) | Integration: cancel context → verify clean termination | Context cancellation |
| NFR15 (knowledge limits) | Unit: LEARNINGS.md >200 lines → distillation trigger | Unit test knowledge.go |

### Maintainability

**Релевантность: MEDIUM.** Для Go CLI — стандартные инструменты.

| NFR | Подход | Инструмент |
|-----|--------|------------|
| NFR18 (prompt files) | Golden file тесты: изменение промпта = осознанное обновление snapshot | `go test -update` workflow |
| NFR19 (add review agent = add .md file) | Integration: custom agent dir с доп. файлом → verify loading | Unit test config fallback |
| NFR20 (isolated components) | golangci-lint: cyclic deps detection. Dependency direction enforced | `golangci-lint`, Go compiler |
| Code coverage | `go test -coverprofile` → ≥80% для runner и config | GitHub Actions CI |
| Linting | `golangci-lint` с `.golangci.yml` | GitHub Actions CI |

---

## Test Environment Requirements

### Local Development

```
Requirement: Go 1.25+, git
Command: go test ./...
Duration: <30 seconds (unit + integration with mocks)
```

- Все unit и integration тесты запускаются локально без внешних зависимостей
- Mock Claude: Go script в `internal/testutil/`, вызывается через `config.ClaudeCommand`
- MockGitClient: in-process mock, implements `GitClient` interface
- `t.TempDir()`: temp directories для каждого теста
- Golden files: `testdata/` в каждом package

### CI (GitHub Actions)

```yaml
# Go version matrix: 1.25, 1.26
# Jobs: test, lint, build
# Duration target: <3 minutes
```

- `go test -race ./...` — race detector
- `go test -coverprofile=coverage.out ./...` — coverage report
- `golangci-lint run` — linting
- `go build ./cmd/ralph` — build verification
- Cross-platform build: `GOOS=linux GOARCH=amd64`, `GOOS=darwin GOARCH=arm64`

### Smoke Test Environment (Growth)

```
Requirement: Go 1.25+, git, claude CLI, API key
Trigger: manual или nightly CI
Duration: ~5 minutes
```

- Real Claude CLI для JSON format validation
- Real session для `--resume` verification
- API key через CI secrets

---

## Testability Concerns

### CONCERN-1: Prompt Quality Not Testable (Score: 6)

**Проблема:** Golden files фиксируют структуру промпта, но не гарантируют корректное поведение LLM. 999-rules, ATDD instructions, review sub-agent prompts — всё зависит от интерпретации Claude.

**Impact:** Плохой промпт → плохой execute/review → каскад failures → потеря денег на API.

**Митигация (MVP):**
- Golden file snapshot тесты для каждого промпта (drift detection)
- Adversarial golden files: промпт содержит конкретные запреты (999-rules)
- Scenario-based integration: mock Claude output → verify ralph правильно обрабатывает результаты
- Prompt regression: `go test -update` workflow для осознанного обновления

**Митигация (Growth):**
- Real Claude smoke tests (one-task sprint)
- LLM-as-Judge для оценки quality review findings

**Рекомендация:** НЕ блокирует gate. Prompt quality — acknowledged risk, mitigation через golden files + real usage feedback.

### CONCERN-2: Review Sub-Agent False Positives (Score: 4)

**Проблема:** 4 review sub-агента могут генерировать false positive findings. При false positives — бесконечный execute→review цикл (до `max_review_iterations`).

**Impact:** Лишние execute-сессии → потеря API tokens. Не блокирует систему (emergency gate сработает).

**Митигация (MVP):**
- `max_review_iterations` = 3 (hard limit предотвращает бесконечный цикл)
- Emergency human gate при превышении лимита
- Review prompt включает explicit verification step (CONFIRMED vs FALSE POSITIVE)

**Митигация (Growth):**
- Cross-model review (разные bias у sonnet/haiku)
- `review_min_severity` filtering
- Review quality metrics из лог-файлов

**Рекомендация:** НЕ блокирует gate. Bounded risk через max_review_iterations.

### CONCERN-3: Claude CLI Contract Stability (Score: 4)

**Проблема:** Ralph зависит от формата Claude CLI output (`--output-format json`), exit codes, `--resume` flag. Claude CLI может измениться.

**Impact:** Breaking change → ralph неработоспособен.

**Митигация (MVP):**
- Golden file тесты на JSON parsing (edge cases: truncated, empty, unexpected fields)
- `config.ClaudeCommand` — injectable, позволяет подменить
- Documented CLI parameters only (NFR7)

**Митигация (Growth):**
- Real Claude smoke test в CI (nightly) — early warning
- Version check (FR40)

**Рекомендация:** НЕ блокирует gate. Стандартный risk для CLI tools, mitigation через golden files + abstraction.

---

## Recommendations for Sprint 0

### Foundation Test Infrastructure (Epic 1 stories)

1. **Mock Claude script** (`internal/testutil/mock_claude.go`)
   - Scenario-based: JSON file → ordered responses
   - Supports: exit codes, session IDs, stdout/stderr, `creates_commit` flag
   - Priority: P0 — разблокирует ВСЕ integration tests

2. **MockGitClient** (`internal/testutil/mock_git.go`)
   - Implements `GitClient` interface
   - Configurable: health check results, HEAD changes, errors
   - Priority: P0 — разблокирует runner tests

3. **Golden file infrastructure**
   - `go test -update` flag для обновления golden files
   - Pattern: `testdata/TestName.golden` per package
   - Priority: P0 — разблокирует prompt tests

4. **CI pipeline** (`.github/workflows/ci.yml`)
   - Go 1.25/1.26 matrix
   - `go test -race -coverprofile ./...`
   - `golangci-lint run`
   - Coverage threshold: 80% for runner, config
   - Priority: P1 — нужен до merge первого PR

### Testing Patterns to Enforce

| Паттерн | Описание | Enforcement |
|---------|----------|-------------|
| **Table-driven** | `[]struct{name, ...}` + `t.Run` для >2 cases | Code review |
| **Golden files** | Prompt snapshots, bridge output, JSON parsing | `go test -update` workflow |
| **Scenario mocks** | Mock Claude с JSON scenario файлами | `internal/testutil/scenarios/` |
| **No global state** | `t.TempDir()`, config by parameter | golangci-lint + review |
| **No testify** | Go stdlib `if got != want { t.Errorf }` | Code review |
| **Error wrapping** | `fmt.Errorf("pkg: op: %w", err)` | golangci-lint |

### Critical Prompt Tests (Golden Files)

| Промпт | Package | Risk | Тестовый подход |
|--------|---------|------|----------------|
| **Execute prompt** | `runner/prompts/execute.md` | HIGH | Golden file: includes 999-rules, ATDD, self-directing instructions. Adversarial: contains `git push --force` prohibition |
| **Review prompt** | `runner/prompts/review.md` | HIGH | Golden file: includes Task tool setup for 4 sub-agents, verification instructions, `[x]` marking |
| **Bridge prompt** | `bridge/prompts/bridge.md` | MEDIUM | Golden file: includes sprint-tasks.md format contract, `[GATE]` marking, `source:` field |
| **Distillation prompt** | `runner/prompts/distillation.md` | MEDIUM | Golden file: includes 200-line target, criteria for "valuable" knowledge |
| **Sub-agent prompts** | `runner/prompts/agents/*.md` | MEDIUM | Golden files: quality, implementation, simplification, test-coverage |

### Test Coverage Targets

| Package | Target | Обоснование |
|---------|:------:|-------------|
| `config` | ≥85% | Фундамент. YAML parsing, fallback chain, constants |
| `runner` | ≥80% | Critical path. Loop, scan, knowledge, git |
| `session` | ≥80% | Claude CLI invocation, JSON parsing |
| `bridge` | ≥75% | Bridge logic, prompt assembly |
| `gates` | ≥70% | Interactive logic, acceptance via scenario tests |
| `cmd/ralph` | ≥50% | Wiring only, tested through integration |

---

## Risk Assessment Summary

### High-Priority Risks (Score ≥6)

| Risk ID | ASR | Category | Description | Probability | Impact | Score | Mitigation |
|---------|-----|----------|-------------|:-----------:|:------:|:-----:|------------|
| R-001 | ASR-8 | TECH | 999-rules и prompt quality не unit-testable | 2 | 3 | **6** | Golden files + adversarial tests + real smoke (Growth) |
| R-002 | ASR-1 | PERF | Context window overflow не автоматически детектируем | 2 | 3 | **6** | LEARNINGS.md budget (200 lines) + --max-turns |
| R-003 | ASR-2 | DATA | Crash recovery может потерять progress | 2 | 3 | **6** | Integration tests: mock crash → resume → verify state |
| R-004 | ASR-6 | TECH | Resume-extraction failure → lost progress | 3 | 2 | **6** | Integration test: mock resume failure → fallback `git checkout` → retry |
| R-005 | ASR-9 | DATA | Smart Merge сбрасывает `[x]` | 2 | 3 | **6** | Golden file тесты merge scenarios + backup sprint-tasks.md before merge |

### Medium-Priority Risks (Score 3-4)

| Risk ID | Category | Description | Probability | Impact | Score | Mitigation |
|---------|----------|-------------|:-----------:|:------:|:-----:|------------|
| R-006 | BUS | Review false positives → лишние execute cycles | 2 | 2 | **4** | max_review_iterations=3 + emergency gate |
| R-007 | TECH | Claude CLI format change breaks parsing | 2 | 2 | **4** | Golden files + version check (Growth) |
| R-008 | OPS | Graceful shutdown не завершает Claude корректно | 2 | 2 | **4** | Integration test: context cancel → verify termination |
| R-009 | TECH | Review sub-agent prompts drift | 1 | 3 | **3** | Golden file snapshot per sub-agent prompt |

### Low-Priority Risks (Score 1-2)

| Risk ID | Category | Description | Probability | Impact | Score | Action |
|---------|----------|-------------|:-----------:|:------:|:-----:|--------|
| R-010 | PERF | Ralph overhead exceeds 5s | 1 | 1 | **1** | Benchmark test (Go native — unlikely) |
| R-011 | OPS | goreleaser build fails cross-platform | 1 | 2 | **2** | CI matrix (linux/darwin/amd64/arm64) |

---

## Quality Gate Criteria

### Pass/Fail Thresholds

- **Unit test pass rate:** 100% (no exceptions)
- **Integration test pass rate:** 100% (mock-based, deterministic)
- **Coverage:** ≥80% для `runner`, `config`, `session`
- **Linting:** Zero errors from `golangci-lint`
- **Golden files:** All golden files up to date (CI без `-update` flag)
- **Build:** `go build ./cmd/ralph` succeeds on Go 1.25 и 1.26

### Non-Negotiable Requirements

- [ ] Mock Claude infrastructure exists and works (scenario-based)
- [ ] MockGitClient implements GitClient interface
- [ ] Golden file tests exist for ALL prompt files
- [ ] All high-priority risks (R-001 through R-005) have corresponding tests
- [ ] CI pipeline runs tests + lint + build on every push
- [ ] No `os.Exit()` outside `cmd/ralph/`
- [ ] No `exec.Command()` without `exec.CommandContext(ctx)`

---

## Appendix

### Knowledge Base References

- `risk-governance.md` — Risk classification framework (TECH, SEC, PERF, DATA, BUS, OPS)
- `probability-impact.md` — Risk scoring methodology (probability × impact)
- `test-levels-framework.md` — Test level selection (unit vs integration vs E2E)
- `test-quality.md` — Quality standards (deterministic, isolated, <300 lines, <1.5 min)
- `nfr-criteria.md` — NFR validation approach (security, performance, reliability, maintainability)

### Related Documents

- PRD: `docs/prd.md`
- Architecture: `docs/architecture.md`
- Epics: `docs/epics.md`

### Testing Strategy from Architecture (Cross-Reference)

| Уровень | Инструмент | Что тестирует | Охват |
|---------|-----------|---------------|-------|
| Unit tests | Go `testing` | Config, state scanning, prompt assembly | 70% |
| Integration tests | Go + mock Claude | Full scenarios: execute→review loop, gates, retry, shutdown | 25% |
| Prompt snapshot | Go + golden files | Diff промптов с baseline | Included in unit |
| Golden file (bridge) | Go + testdata/ | Input story → expected sprint-tasks.md | Included in integration |
| Smoke tests (Growth) | Go + real Claude | JSON format validation, --resume, one-task sprint | 5% |

---

**Generated by:** BMad TEA Agent — Test Architect Module
**Workflow:** `.bmad/bmm/testarch/test-design`
**Version:** 4.0 (BMad v6)
