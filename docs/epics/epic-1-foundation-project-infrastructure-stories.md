# Epic 1: Foundation & Project Infrastructure — Stories

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
