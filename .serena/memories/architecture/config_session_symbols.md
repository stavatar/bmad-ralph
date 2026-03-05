# Config, Session, Bridge, Gates — Symbol Map

## config/config.go
- `Config` struct: 18 fields (ClaudeCommand, MaxTurns, MaxIterations, GatesEnabled, etc.)
- `CLIFlags` struct: CLI override fields
- `Load(path) (*Config, error)` — YAML loading with defaults + validation
- `Config.Validate() error` — field validation
- `Config.ResolvePath(rel) string` — resolve relative to ProjectRoot
- `defaultConfig() Config` — embedded defaults.yaml
- `detectProjectRoot() string` — walks up for .claude/ or go.mod

## config/constants.go
- Task markers: TaskOpen `"- [ ]"`, TaskDone `"- [x]"`
- Gate/feedback: GateTag `"[GATE]"`, FeedbackPrefix `"[FEEDBACK]"`
- Actions: ActionApprove, ActionRetry, ActionSkip, ActionQuit
- Compiled regexes: TaskOpenRegex, TaskDoneRegex, GateTagRegex, SourceFieldRegex

## config/errors.go
- Sentinels: ErrNoTasks, ErrMaxRetries, ErrMaxReviewCycles
- `ExitCodeError{Code, Message}` — wraps exit codes from Claude
- `GateDecision{Action, Feedback}` — gate prompt result (implements error)

## config/prompt.go
- `TemplateData` — all fields for Go template rendering
- `AssemblePrompt(templateStr, data) (string, error)` — renders prompt with validation

## session/session.go
- `Options` struct: Command, Dir, Prompt, MaxTurns, Model, etc.
- `RawResult` struct: Stdout, Stderr, ExitCode
- `Execute(ctx, opts) (RawResult, error)` — runs Claude subprocess

## session/result.go
- `SessionResult{SessionID, ExitCode, Output, Duration}`
- `ParseResult(raw, elapsed) (SessionResult, error)` — JSON parsing

## bridge/bridge.go
- `BridgePrompt(cfg, tasksContent) (string, error)` — assembles bridge prompt
- `Run(ctx, cfg, prompt) error` — executes bridge session

## gates/gates.go
- `Gate{TaskText, Reader, Writer, Emergency}`
- `Prompt(gate) (*GateDecision, error)` — interactive terminal prompt
