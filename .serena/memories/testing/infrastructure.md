# Test Infrastructure

## Mock Claude (internal/testutil/mock_claude.go)
- `Scenario{Name, Steps[]ScenarioStep}` — defines Claude subprocess behavior
- `ScenarioStep` — Type, ExitCode, SessionID, OutputFile, CreatesCommit, IsError, WriteFiles, DeleteFiles
- `RunMockClaude()` — called from TestMain, reads MOCK_CLAUDE_SCENARIO env var
- `SetupMockClaude(t, dir, scenarios) string` — writes scenario JSON, returns mock command
- `ReadInvocationArgs(t, dir) [][]string` — reads what args mock was called with
- Pattern: self-reexec via TestMain + env var (go.exe can't run bash scripts on WSL)

## Mock Git (internal/testutil/mock_git.go)
- `MockGitClient` — configurable error sequences for HealthCheck/HeadCommit/RestoreClean
- Fields: HealthCheckErrors[], HeadCommits[], HeadCommitErrors[], RestoreCleanError
- Tracks call counts: HealthCheckCount, HeadCommitCount, RestoreCleanCount

## Test Helpers (runner/test_helpers_test.go)
- DRY helpers: testConfig, trackingSleep, trackingGatePrompt, sequenceGatePrompt
- Fixture constants: threeOpenTasks, etc.
- ReviewFn closures for various test scenarios

## Golden Files
- Location: `<pkg>/testdata/TestName.golden`
- Update flag: `-update` on go test
- Used for prompt rendering tests

## Key Test Files
- `runner/runner_test.go` — unit tests for Runner methods
- `runner/runner_integration_test.go` — full loop integration with mock Claude
- `runner/runner_review_integration_test.go` — review cycle integration
- `runner/runner_gates_integration_test.go` — gate prompt integration
- `runner/runner_final_integration_test.go` — end-to-end scenarios
- `runner/metrics_test.go` — MetricsCollector unit tests (lifecycle, nil-safe, aggregation)
- `runner/similarity_test.go` — SimilarityDetector tests (Jaccard, window, thresholds)
- `runner/prompt_test.go` — template rendering + content assertions
- `runner/testmain_test.go` — TestMain with mock Claude dispatch
- `gates/gates_test.go` + `gates/gates_integration_test.go`
- `config/pricing_test.go` — Pricing tests (merge, most expensive, cost calculation)

## Testing Patterns
- Table-driven by default, Go stdlib assertions (no testify)
- t.TempDir() for filesystem isolation
- errors.As for type checks, errors.Is for sentinels
- strings.Contains for error message verification
- Coverage targets: runner, config >80%
