# Interfaces & Dependency Injection

## Key Interfaces (defined in consumer packages)

### runner.GitClient (runner/git.go)
```go
type GitClient interface {
    HealthCheck() error
    HeadCommit() (string, error)
    RestoreClean() error
}
```
- Real: `ExecGitClient` (runner/git.go)
- Mock: `MockGitClient` (internal/testutil/mock_git.go)

### runner.CodeIndexerDetector (runner/serena.go)
```go
type CodeIndexerDetector interface {
    Available() bool
    PromptHint() string
}
```
- Real: `SerenaMCPDetector` — checks .mcp.json for serena
- Noop: `NoOpCodeIndexerDetector`

### runner.KnowledgeWriter (runner/knowledge.go)
```go
type KnowledgeWriter interface {
    WriteProgress(data ProgressData) error
    ValidateNewLessons(data LessonsData) error
}
```
- Real: `FileKnowledgeWriter` (runner/knowledge_write.go)
- Noop: `NoOpKnowledgeWriter`

## Runner Injectable Fields
Runner uses function/struct fields for testability:
- `ReviewFn`: `func(cfg *config.Config, ...) (ReviewResult, error)` — code review
- `GatePromptFn`: `func(taskText string, ...) (config.GateDecision, error)` — human gate
- `EmergencyGatePromptFn`: same signature, emergency variant
- `ResumeExtractFn`: `func(cfg *config.Config, ...) error` — resume extraction
- `DistillFn`: `func(cfg *config.Config, ...) error` — knowledge distillation
- `SleepFn`: `func(time.Duration)` — for testing time-dependent code
- `Metrics`: `*MetricsCollector` — nil = no-op (nil-safe injectable pattern)
- `Similarity`: `*SimilarityDetector` — nil = disabled

## Test Mocks (runner/test_helpers_test.go)
- `trackingGatePrompt`: records calls + task texts
- `sequenceGatePrompt`: returns pre-defined sequence of decisions
- `trackingSleep`: records sleep durations
