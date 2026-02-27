package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
	"github.com/bmad-ralph/bmad-ralph/runner"
)

// --- Shared test data ---

const threeOpenTasks = `# Sprint Tasks

## Epic 1: Foundation

- [ ] Task one
- [ ] Task two
- [ ] Task three
`

const allDoneTasks = `# Sprint Tasks

- [x] Done one
- [x] Done two
`

const noMarkersTasks = `# Sprint Tasks

No tasks here
`

// gateOpenTask has a single task with [GATE] tag for gate detection tests.
const gateOpenTask = `# Sprint Tasks

- [ ] Setup project [GATE]
`

// nonGateOpenTask has a single task without [GATE] tag.
const nonGateOpenTask = `# Sprint Tasks

- [ ] Write tests
`

// fourOpenTasks has 4 tasks without [GATE] for checkpoint-only tests.
const fourOpenTasks = `# Sprint Tasks

- [ ] Task one
- [ ] Task two
- [ ] Task three
- [ ] Task four
`

// fourOpenTasksWithGate has task 2 with [GATE] for combined checkpoint + gate tests.
const fourOpenTasksWithGate = `# Sprint Tasks

- [ ] Task one
- [ ] Task two [GATE]
- [ ] Task three
- [ ] Task four
`

// twoTasksWithGate has task 1 with [GATE] for retry + checkpoint counter tests.
const twoTasksWithGate = `# Sprint Tasks

- [ ] Task one [GATE]
- [ ] Task two
`

// threeTasksWithGate has task 1 with [GATE] for skip + checkpoint counter tests.
const threeTasksWithGate = `# Sprint Tasks

- [ ] Task one [GATE]
- [ ] Task two
- [ ] Task three
`

// --- Shared ReviewFn helpers (DRY: used across 9+ tests) ---

// cleanReviewFn returns a clean review result with no error.
var cleanReviewFn runner.ReviewFunc = func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
	return runner.ReviewResult{Clean: true}, nil
}

// fatalReviewFn returns a ReviewFunc that fails the test if called.
func fatalReviewFn(t *testing.T) runner.ReviewFunc {
	t.Helper()
	return func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		t.Fatal("ReviewFn should not be called")
		return runner.ReviewResult{}, nil
	}
}

// --- Shared test helpers ---

// writeTasksFile writes inline task content to sprint-tasks.md in dir.
func writeTasksFile(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "sprint-tasks.md")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write tasks file: %v", err)
	}
	return p
}

// headCommitPairs generates a flat HeadCommits slice from before/after SHA pairs.
// Each pair represents one iteration: [before1, after1, before2, after2, ...].
func headCommitPairs(pairs ...[2]string) []string {
	result := make([]string, 0, len(pairs)*2)
	for _, p := range pairs {
		result = append(result, p[0], p[1])
	}
	return result
}

// copyFixtureToDir copies a testdata fixture into tmpDir and returns the destination path.
func copyFixtureToDir(t *testing.T, tmpDir, fixture string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	dst := filepath.Join(tmpDir, fixture)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write fixture %s: %v", fixture, err)
	}
	return dst
}

// assertArgsContainFlag verifies a flag exists in args.
func assertArgsContainFlag(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			return
		}
	}
	t.Errorf("args: want flag %q, not found in %v", flag, args)
}

// assertArgsContainFlagValue verifies a flag with specific value exists in args.
func assertArgsContainFlagValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Errorf("args: flag %q found at end of args, no value follows", flag)
				return
			}
			if args[i+1] != value {
				t.Errorf("args: flag %q value = %q, want %q", flag, args[i+1], value)
			}
			return
		}
	}
	t.Errorf("args: want flag %q with value %q, flag not found in %v", flag, value, args)
}

// assertArgsFlagAbsent verifies a flag is NOT in args.
func assertArgsFlagAbsent(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, a := range args {
		if a == flag {
			t.Errorf("args: want flag %q absent, but found in %v", flag, args)
			return
		}
	}
}

// argValueAfterFlag returns the value following the given flag in args, or "" if not found.
func argValueAfterFlag(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// testConfig creates a config.Config with standard test defaults.
// ClaudeCommand is set to os.Args[0] (self-reexec mock pattern).
func testConfig(tmpDir string, maxIter int) *config.Config {
	return &config.Config{
		ClaudeCommand:       os.Args[0],
		MaxTurns:            5,
		MaxIterations:       maxIter,
		MaxReviewIterations: 3,
		ProjectRoot:         tmpDir,
	}
}

// --- Gate-related test helpers (Story 5.2) ---

// trackingGatePrompt records GatePromptFn calls for assertion.
type trackingGatePrompt struct {
	count     int
	taskText  string               // last taskText received
	taskTexts []string             // all taskTexts received (accumulated)
	decision  *config.GateDecision // fixed decision to return
	err       error                // fixed error to return
}

func (tg *trackingGatePrompt) fn(_ context.Context, taskText string) (*config.GateDecision, error) {
	tg.count++
	tg.taskText = taskText
	tg.taskTexts = append(tg.taskTexts, taskText)
	return tg.decision, tg.err
}

// setupGateTest creates a Runner with standard gate test defaults.
// Returns the Runner and a trackingGatePrompt for call-count assertions.
// Callers override GatePromptFn, cfg fields, etc. as needed after construction.
func setupGateTest(t *testing.T, tasks string, gatesEnabled bool) (*runner.Runner, *trackingGatePrompt) {
	t.Helper()
	tmpDir := t.TempDir()
	tasksPath := writeTasksFile(t, tmpDir, tasks)

	scenario := testutil.Scenario{
		Name: "gate-test",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 0, SessionID: "gate-001"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	mock := &testutil.MockGitClient{
		HeadCommits: headCommitPairs([2]string{"aaa", "bbb"}),
	}

	cfg := testConfig(tmpDir, 1)
	cfg.GatesEnabled = gatesEnabled

	gp := &trackingGatePrompt{
		decision: &config.GateDecision{Action: config.ActionApprove},
	}

	r := &runner.Runner{
		Cfg:             cfg,
		Git:             mock,
		TasksFile:       tasksPath,
		ReviewFn:        cleanReviewFn,
		GatePromptFn:    gp.fn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}
	return r, gp
}

// gateOpenTaskDone has a single [GATE] task marked done (post-review state).
const gateOpenTaskDone = `# Sprint Tasks

- [x] Setup project [GATE]
`

// sequenceGatePrompt returns different decisions per call.
// Used in gate retry tests where first call returns retry, second returns approve.
type sequenceGatePrompt struct {
	calls     int
	decisions []*config.GateDecision
	taskTexts []string
}

func (sg *sequenceGatePrompt) fn(_ context.Context, taskText string) (*config.GateDecision, error) {
	idx := sg.calls
	sg.calls++
	sg.taskTexts = append(sg.taskTexts, taskText)
	if idx < len(sg.decisions) {
		return sg.decisions[idx], nil
	}
	return &config.GateDecision{Action: config.ActionApprove}, nil
}

// --- Retry-related test helpers (Story 3.6) ---

// trackingResumeExtract records ResumeExtractFn calls for assertion.
type trackingResumeExtract struct {
	count      int
	sessionIDs []string
	err        error // inject error for error-path tests
}

func (tr *trackingResumeExtract) fn(_ context.Context, _ runner.RunConfig, sid string) error {
	tr.count++
	tr.sessionIDs = append(tr.sessionIDs, sid)
	return tr.err
}

// trackingSleep records SleepFn calls for backoff duration assertions.
type trackingSleep struct {
	count     int
	durations []time.Duration
}

func (ts *trackingSleep) fn(d time.Duration) {
	ts.count++
	ts.durations = append(ts.durations, d)
}

// noopSleepFn is a no-op sleep for tests that don't care about timing.
var noopSleepFn = func(time.Duration) {}

// noopResumeExtractFn is a no-op resume extract for tests that don't care about it.
var noopResumeExtractFn runner.ResumeExtractFunc = func(_ context.Context, _ runner.RunConfig, _ string) error {
	return nil
}

// --- KnowledgeWriter test helpers (Story 3.7) ---

// trackingKnowledgeWriter records WriteProgress calls for assertion.
type trackingKnowledgeWriter struct {
	writeProgressCount int
	writeProgressData  []runner.ProgressData
	writeProgressErr   error // inject error for error-path tests
}

func (tk *trackingKnowledgeWriter) WriteProgress(_ context.Context, data runner.ProgressData) error {
	tk.writeProgressCount++
	tk.writeProgressData = append(tk.writeProgressData, data)
	return tk.writeProgressErr
}

// setupRunnerIntegration creates a Runner with all fields initialized for integration tests.
// All 7 Execute integration tests share identical boilerplate — helper is DRY-justified.
// Defaults: cleanReviewFn, noopResumeExtractFn, noopSleepFn, NoOpKnowledgeWriter.
// Callers override ReviewFn/ResumeExtractFn/SleepFn as needed after construction.
func setupRunnerIntegration(t *testing.T, tmpDir, tasksContent string, scenario testutil.Scenario, git *testutil.MockGitClient) (*runner.Runner, string) {
	t.Helper()
	tasksPath := writeTasksFile(t, tmpDir, tasksContent)
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	cfg := testConfig(tmpDir, 3) // default MaxIterations=3; callers adjust via r.Cfg.MaxIterations
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             git,
		TasksFile:       tasksPath,
		ReviewFn:        cleanReviewFn,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}
	return r, stateDir
}

// setupReviewIntegration creates a Runner with RealReview for full review pipeline
// integration tests. Unlike setupRunnerIntegration, uses real review session via
// MockClaude subprocess with file side effects.
// Sets MOCK_CLAUDE_PROJECT_ROOT so MockClaude subprocess can write to tmpDir.
func setupReviewIntegration(t *testing.T, tmpDir, tasksContent string, scenario testutil.Scenario, git *testutil.MockGitClient) (*runner.Runner, string) {
	t.Helper()
	tasksPath := writeTasksFile(t, tmpDir, tasksContent)
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	t.Setenv("MOCK_CLAUDE_PROJECT_ROOT", tmpDir)
	cfg := testConfig(tmpDir, 3)
	r := &runner.Runner{
		Cfg:             cfg,
		Git:             git,
		TasksFile:       tasksPath,
		ReviewFn:        runner.RealReview,
		ResumeExtractFn: noopResumeExtractFn,
		SleepFn:         noopSleepFn,
		Knowledge:       &runner.NoOpKnowledgeWriter{},
	}
	return r, stateDir
}

// progressiveReviewFn returns a ReviewFunc that marks the first open task [ ] as [x]
// on each call, simulating progressive task completion. Used in multi-task checkpoint tests.
func progressiveReviewFn(tasksPath string, counter *int) runner.ReviewFunc {
	return func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		if counter != nil {
			*counter++
		}
		data, err := os.ReadFile(tasksPath)
		if err != nil {
			return runner.ReviewResult{}, err
		}
		content := strings.Replace(string(data), "- [ ]", "- [x]", 1)
		// Error ignored: test helper in controlled tmpDir; failure surfaces via downstream assertions
		_ = os.WriteFile(tasksPath, []byte(content), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}
}

// reviewAndMarkDoneFn returns a ReviewFunc that increments counter (if non-nil)
// and writes allDoneTasks to tasksPath. Used in retry tests where review triggers
// outer loop exit via task completion.
func reviewAndMarkDoneFn(tasksPath string, counter *int) runner.ReviewFunc {
	return func(_ context.Context, _ runner.RunConfig) (runner.ReviewResult, error) {
		if counter != nil {
			*counter++
		}
		// Error ignored: test helper in controlled tmpDir; failure surfaces via downstream assertions
		_ = os.WriteFile(tasksPath, []byte(allDoneTasks), 0644)
		return runner.ReviewResult{Clean: true}, nil
	}
}
