# Project Structure & Boundaries

### Complete Project Directory Structure

```
bmad-ralph/
├── .github/
│   └── workflows/
│       └── ci.yml                  # test + lint + build (Go 1.25, 1.26 matrix)
├── .goreleaser.yml                 # Cross-platform binary builds + GitHub Releases
├── .golangci.yml                   # Linter configuration
├── .gitignore
├── Makefile                        # make build, make test, make lint, make release
├── go.mod                          # Go 1.25
├── go.sum
├── LICENSE
├── cmd/ralph/
│   ├── main.go                     # Cobra root, signal.NotifyContext, log file, var version = "dev"
│   ├── bridge.go                   # bridge subcommand (flag wiring → bridge.Run)
│   └── run.go                      # run subcommand (flag wiring → runner.Run)
├── bridge/
│   ├── bridge.go                   # bridge.Run(ctx, cfg) — story → sprint-tasks.md
│   ├── bridge_test.go
│   ├── prompts/                    # go:embed
│   │   └── bridge.md               # Bridge prompt template (text/template)
│   └── testdata/                   # Golden files: input stories → expected sprint-tasks.md
│       ├── TestBridge_SingleStory.golden
│       └── TestBridge_MultiStory.golden
├── runner/
│   ├── runner.go                   # runner.Run(ctx, cfg) — main loop
│   ├── runner_test.go
│   ├── git.go                      # GitClient interface + ExecGitClient implementation
│   ├── git_test.go
│   ├── scan.go                     # sprint-tasks.md scanning (TaskOpen, TaskDone, GateTag)
│   ├── scan_test.go
│   ├── knowledge.go                # LEARNINGS.md append, budget check (200 lines hard limit), distillation trigger
│   ├── knowledge_test.go
│   ├── prompts/                    # go:embed
│   │   ├── execute.md              # Execute prompt template
│   │   ├── review.md               # Review prompt template
│   │   ├── distillation.md         # LEARNINGS.md compression prompt
│   │   └── agents/
│   │       ├── quality.md
│   │       ├── implementation.md
│   │       ├── simplification.md
│   │       └── test-coverage.md
│   └── testdata/                   # Fixtures for runner integration tests
│       ├── sprint-tasks-basic.md       # Example sprint-tasks.md
│       ├── sprint-tasks-with-gate.md   # sprint-tasks.md with [GATE] tags
│       ├── review-findings-sample.md   # Example review-findings.md
│       ├── learnings-over-budget.md    # LEARNINGS.md exceeding budget
│       ├── TestPrompt_Execute.golden   # Assembled execute prompt snapshot
│       ├── TestPrompt_Review.golden    # Assembled review prompt snapshot
│       └── TestRunner_HappyPath.golden
├── session/
│   ├── session.go                  # session.Execute(ctx, opts) — Claude CLI invocation
│   ├── session_test.go
│   └── result.go                   # SessionResult struct (session_id, exit_code, output)
├── gates/
│   ├── gates.go                    # gates.Prompt(ctx, gate) — interactive stdin
│   └── gates_test.go
├── config/
│   ├── config.go                   # config.Load(flags) — YAML + CLI + defaults cascade
│   ├── config_test.go
│   ├── constants.go                # TaskOpen, TaskDone, GateTag, FeedbackPrefix + regex patterns
│   ├── shared/                     # go:embed
│   │   └── sprint-tasks-format.md  # Format contract (shared between bridge & execute)
│   └── testdata/
│       ├── TestConfig_Default.golden
│       └── TestConfig_Override.golden
├── internal/
│   └── testutil/
│       ├── mock_claude.go          # Scenario-based mock Claude CLI
│       ├── mock_git.go             # MockGitClient implementation
│       └── scenarios/              # JSON scenario files for integration tests
│           ├── happy_path.json
│           ├── review_findings.json
│           └── max_retries.json
└── docs/                           # Project documentation (не embed, не runtime)
```

**Version embedding:** `var version = "dev"` в `cmd/ralph/main.go`. goreleaser overrides через `-ldflags "-X main.version=v1.0.0"`. Cobra `rootCmd.Version = version`.

**Runner split boundary:** В MVP `runner/` содержит loop + git + scan + knowledge (~600-800 LOC est.). Когда превышает ~1000 LOC — выделять `review`, `knowledge`, `state` как отдельные packages (Growth packages list). Агенты НЕ должны split'ить runner в MVP.

### Runtime `.ralph/` Structure (в проекте пользователя)

```
user-project/
├── .ralph/
│   ├── config.yaml                 # User config
│   ├── agents/                     # Custom review agent overrides (.md)
│   ├── prompts/
│   │   └── execute.md              # Custom execute prompt override
│   └── logs/
│       ├── run-2026-02-24-143022.log
│       └── run-2026-02-24-160515.log
├── sprint-tasks.md                 # Bridge output, main state file
├── review-findings.md              # Transient — current task findings
├── LEARNINGS.md                    # Accumulated knowledge
└── CLAUDE.md                       # Operational context (ralph section)
```

### FR → Package Mapping

| FR Category | Package | Ключевые FR |
|-------------|---------|-------------|
| **Bridge** | `bridge` | FR1-FR5a: story → sprint-tasks.md, AC-derived tests, human gates, smart merge |
| **Execute loop** | `runner` | FR6-FR12: sequential execution, fresh sessions, retry, resume-extraction |
| **Review** | `runner` | FR13-FR19: 4 sub-agents, verification, findings → execute fix cycle |
| **Human gates** | `gates` | FR20-FR25: approve/retry/skip/quit, emergency gate, checkpoint |
| **Knowledge** | `runner` (knowledge.go) | FR26-FR29: LEARNINGS.md, CLAUDE.md section, distillation |
| **Config** | `config` | FR30-FR35: YAML + CLI cascade, agent files fallback |
| **Guardrails** | `runner` (prompts) | FR36-FR41: 999-rules in execute prompt, ATDD, Serena |
| **CLI** | `cmd/ralph` | Exit codes, flag parsing, colored output, log file |
| **Claude CLI** | `session` | All session invocations, --output-format json, --resume |

**Принцип:** Architecture doc маппит FR → packages. Story files при реализации указывают конкретные файлы. Это level of detail для stories, не для architecture.

### Package Boundaries — Кто что читает/пишет

| File | Creator | Reader | Writer |
|------|---------|--------|--------|
| `sprint-tasks.md` | `bridge` (через Claude) | `runner` (scan), Claude (execute/review) | Claude (execute: progress, review: `[x]`), ralph (feedback) |
| `review-findings.md` | Claude (review) | Claude (execute) | Claude (review: overwrite/clear) |
| `LEARNINGS.md` | Claude (execute, review, resume-extraction) | `runner` (budget check), Claude (all sessions) | Claude sessions (append), ralph → distillation session (rewrite при превышении бюджета) |
| `CLAUDE.md` section | Claude (review, resume-extraction) | Claude (all sessions) | Claude (update ralph section) |
| `.ralph/config.yaml` | User | `config` | Never (read-only) |
| `.ralph/logs/*.log` | `cmd/ralph` | User (post-mortem) | `cmd/ralph` (append) |
| Git state | Claude (execute: commit) | `runner` (HEAD check, health check) | Claude (commit), resume-extraction (WIP commit) |

### Data Flow

```
Stories (.md) ──→ bridge ──→ sprint-tasks.md
                                  │
                    ┌─────────────┘
                    ▼
              runner.Run loop:
                    │
              Serena incremental index (best effort, timeout)
                    │
              ┌─────┴──────┐
              ▼             │
         session.Execute    │
         (execute prompt)   │
              │             │
         commit? ──NO──→ session.Execute(--resume)
              │           resume-extraction
              │             │ WIP commit
              │             │ progress → sprint-tasks.md
              │             │ knowledge → LEARNINGS.md
              │             └──→ retry execute
              │YES
              ▼
         session.Execute
         (review prompt)
         4 sub-agents (+ Serena MCP inside sessions)
              │
         findings? ──YES──→ review-findings.md
              │              knowledge → LEARNINGS.md
              │              retry execute (fix)
              │NO (clean)
              ▼
         mark [x], clear review-findings.md
         budget check LEARNINGS.md
              │
         over budget? ──YES──→ session.Execute(distillation)
              │
              ▼
         next task (or GATE → gates.Prompt)
```

### Integration Points

| Point | From | To | Mechanism |
|-------|------|----|-----------|
| **CLI → Packages** | `cmd/ralph` | `runner.Run`, `bridge.Run` | Direct function call |
| **Runner → Claude** | `runner` | Claude CLI | `session.Execute` (os/exec) |
| **Runner → Git** | `runner` | git CLI | `GitClient` interface (os/exec) |
| **Runner → Gates** | `runner` | `gates` | `gates.Prompt` (stdin/stdout) |
| **Runner → Config** | `runner` | `config` | `config.Config` struct (read-only) |
| **Runner → Serena** | `runner` | Serena CLI | `os/exec` incremental index перед execute (best effort, timeout) |
| **Claude → Serena** | Claude sessions | Serena MCP | Внутри sessions: semantic code retrieval (Claude manages) |
| **Claude → Files** | Claude sessions | Filesystem | Direct read/write (sprint-tasks.md, LEARNINGS.md, etc.) |
| **Ralph → Files** | `runner`, `cmd/ralph` | Filesystem | `os.ReadFile`/`os.WriteFile` (scan, log, feedback) |

### Test Scenario Format

Integration тесты runner используют scenario-based mock Claude. Принцип: scenario = ordered sequence of mock responses, каждый response соответствует одному вызову `session.Execute`:

```json
{
  "name": "happy_path",
  "steps": [
    {"type": "execute", "exit_code": 0, "session_id": "abc-123",
     "creates_commit": true},
    {"type": "review", "exit_code": 0, "session_id": "def-456",
     "output_file": "review_clean.txt", "creates_commit": false}
  ]
}
```

Mock Claude script читает scenario JSON, возвращает responses по порядку. `creates_commit` — mock git отвечает что HEAD изменился. Exact schema определяется при реализации; принцип фиксируется здесь.

**Golden files для промптов:** собранный промпт (после text/template + strings.Replace) = golden file в `testdata/` того package, который собирает промпт. `runner/testdata/TestPrompt_Execute.golden`, `bridge/testdata/TestPrompt_Bridge.golden`.
