package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

// copyFixtureToScenario copies a testdata fixture file into the mock Claude
// scenario directory so the mock can find it via ScenarioStep.OutputFile.
func copyFixtureToScenario(t *testing.T, fixtureName string) {
	t.Helper()
	scenarioDir := filepath.Dir(os.Getenv("MOCK_CLAUDE_SCENARIO"))
	content, err := os.ReadFile(filepath.Join("testdata", fixtureName))
	if err != nil {
		t.Fatalf("read fixture %q: %v", fixtureName, err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, fixtureName), content, 0644); err != nil {
		t.Fatalf("copy fixture %q to scenario dir: %v", fixtureName, err)
	}
}

func TestRun_Success(t *testing.T) {
	// Setup: write story file to temp dir
	projectDir := t.TempDir()
	storyDir := t.TempDir()
	storyPath := filepath.Join(storyDir, "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story: Test Auth\n\n### AC-1\nUser can log in."), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Setup mock Claude with output fixture
	scenario := testutil.Scenario{
		Name: "bridge-success",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "mock-session-1",
				OutputFile: "mock_bridge_output.md",
			},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_bridge_output.md")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	taskCount, promptLines, err := Run(context.Background(), cfg, []string{storyPath})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify task count matches fixture (3 tasks)
	if taskCount != 3 {
		t.Errorf("taskCount = %d, want 3", taskCount)
	}

	// Verify promptLines is positive
	if promptLines <= 0 {
		t.Errorf("promptLines = %d, want > 0", promptLines)
	}

	// Verify output file exists and content matches mock output exactly
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	outContent, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	fixtureContent, err := os.ReadFile("testdata/mock_bridge_output.md")
	if err != nil {
		t.Fatalf("read fixture for comparison: %v", err)
	}
	if string(outContent) != string(fixtureContent) {
		t.Errorf("output file content does not match fixture exactly:\ngot:\n%s\nwant:\n%s", string(outContent), string(fixtureContent))
	}

	// Independently count tasks via regex in output
	lines := strings.Split(string(outContent), "\n")
	regexCount := 0
	for _, line := range lines {
		if config.TaskOpenRegex.MatchString(line) {
			regexCount++
		}
	}
	if regexCount != taskCount {
		t.Errorf("regex task count = %d, Run returned %d", regexCount, taskCount)
	}

	// Verify mock invocation args
	args := testutil.ReadInvocationArgs(t, stateDir, 0)

	hasSkipPerms := false
	hasOutputJSON := false
	hasPromptFlag := false
	for i, arg := range args {
		switch arg {
		case "--dangerously-skip-permissions":
			hasSkipPerms = true
		case "--output-format":
			if i+1 < len(args) && args[i+1] == "json" {
				hasOutputJSON = true
			}
		case "-p":
			hasPromptFlag = true
		}
	}

	if !hasSkipPerms {
		t.Error("mock args missing --dangerously-skip-permissions")
	}
	if !hasOutputJSON {
		t.Error("mock args missing --output-format json")
	}
	if !hasPromptFlag {
		t.Error("mock args missing -p flag")
	}
}

func TestRun_StoryFileNotFound(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
	}

	_, _, err := Run(context.Background(), cfg, []string{"/nonexistent/story.md"})
	if err == nil {
		t.Fatal("expected error for missing story file")
	}

	if !strings.Contains(err.Error(), "bridge: read story:") {
		t.Errorf("error = %q, want prefix 'bridge: read story:'", err.Error())
	}

	// Verify no output file written
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Error("sprint-tasks.md should not exist after story read error")
	}
}

func TestRun_SessionExecuteFails(t *testing.T) {
	projectDir := t.TempDir()
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Mock Claude returns exit code 1
	scenario := testutil.Scenario{
		Name: "bridge-session-fail",
		Steps: []testutil.ScenarioStep{
			{
				Type:      "execute",
				ExitCode:  1,
				SessionID: "mock-fail",
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	_, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err == nil {
		t.Fatal("expected error for session failure")
	}

	if !strings.Contains(err.Error(), "bridge: execute:") {
		t.Errorf("error = %q, want prefix 'bridge: execute:'", err.Error())
	}

	// Atomic write check: no partial output
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Error("sprint-tasks.md should not exist after session failure (atomic write)")
	}
}

func TestRun_ParseResultError(t *testing.T) {
	// Trigger "bridge: parse result:" error path — Execute succeeds (exit 0)
	// but stdout is empty, causing ParseResult to fail.
	// Uses BRIDGE_TEST_EMPTY_OUTPUT env var to make subprocess exit 0 with no output.
	projectDir := t.TempDir()
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	t.Setenv("BRIDGE_TEST_EMPTY_OUTPUT", "1")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	_, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err == nil {
		t.Fatal("expected error for parse result failure")
	}

	if !strings.Contains(err.Error(), "bridge: parse result:") {
		t.Errorf("error = %q, want prefix 'bridge: parse result:'", err.Error())
	}

	// Atomic write check: no output on parse failure
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Error("sprint-tasks.md should not exist after parse error")
	}
}

func TestRun_MultipleStories(t *testing.T) {
	projectDir := t.TempDir()
	storyDir := t.TempDir()

	// Write 2 story files
	story1 := filepath.Join(storyDir, "story1.md")
	story2 := filepath.Join(storyDir, "story2.md")
	if err := os.WriteFile(story1, []byte("## Story 1: Auth"), 0644); err != nil {
		t.Fatalf("write story1: %v", err)
	}
	if err := os.WriteFile(story2, []byte("## Story 2: Payments"), 0644); err != nil {
		t.Fatalf("write story2: %v", err)
	}

	// Setup mock Claude
	scenario := testutil.Scenario{
		Name: "bridge-multi",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "mock-multi",
				OutputFile: "mock_bridge_output.md",
			},
		},
	}
	_, stateDir := testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_bridge_output.md")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	taskCount, _, err := Run(context.Background(), cfg, []string{story1, story2})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if taskCount != 3 {
		t.Errorf("taskCount = %d, want 3", taskCount)
	}

	// Verify mock received prompt containing both stories and full separator
	args := testutil.ReadInvocationArgs(t, stateDir, 0)
	promptContent := ""
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			promptContent = args[i+1]
			break
		}
	}

	if !strings.Contains(promptContent, "Story 1: Auth") {
		t.Error("prompt missing Story 1 content")
	}
	if !strings.Contains(promptContent, "Story 2: Payments") {
		t.Error("prompt missing Story 2 content")
	}
	// Verify full separator (not just "---" which could match other content)
	if !strings.Contains(promptContent, "\n\n---\n\n") {
		t.Error("prompt missing full separator '\\n\\n---\\n\\n' between stories")
	}
}

func TestRun_NoStoryFiles(t *testing.T) {
	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   t.TempDir(),
	}

	_, _, err := Run(context.Background(), cfg, []string{})
	if err == nil {
		t.Fatal("expected error for no story files")
	}

	if !strings.Contains(err.Error(), "bridge: no story files") {
		t.Errorf("error = %q, want 'bridge: no story files'", err.Error())
	}
}

func TestRun_WriteFailure(t *testing.T) {
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Setup mock Claude with valid output
	scenario := testutil.Scenario{
		Name: "bridge-write-fail",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "mock-write-fail",
				OutputFile: "mock_bridge_output.md",
			},
		},
	}
	testutil.SetupMockClaude(t, scenario)
	copyFixtureToScenario(t, "mock_bridge_output.md")

	// Create valid project root but make sprint-tasks.md a directory so WriteFile fails
	projectRoot := t.TempDir()
	blockerPath := filepath.Join(projectRoot, "sprint-tasks.md")
	if err := os.MkdirAll(blockerPath, 0755); err != nil {
		t.Fatalf("create blocker dir: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectRoot,
		MaxTurns:      5,
	}

	taskCount, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err == nil {
		t.Fatal("expected error for write failure")
	}

	if !strings.Contains(err.Error(), "bridge: write tasks:") {
		t.Errorf("error = %q, want prefix 'bridge: write tasks:'", err.Error())
	}

	if taskCount != 0 {
		t.Errorf("taskCount = %d, want 0 on write failure", taskCount)
	}
}

// validateTaskSourcePairs verifies every task line (matching config.TaskOpenRegex)
// is immediately followed by a source line (matching config.SourceFieldRegex).
// Returns the number of task lines found. Used by golden file tests and cross-validation.
func validateTaskSourcePairs(t *testing.T, output string) int {
	t.Helper()
	lines := strings.Split(output, "\n")
	taskCount := 0
	for i, line := range lines {
		if config.TaskOpenRegex.MatchString(line) {
			taskCount++
			if i+1 >= len(lines) || !config.SourceFieldRegex.MatchString(lines[i+1]) {
				t.Errorf("task at line %d has no valid source field on next line", i+1)
			}
		}
	}
	if taskCount == 0 {
		t.Errorf("golden file has no parseable tasks")
	}
	return taskCount
}

func TestRun_GoldenFiles(t *testing.T) {
	cases := []struct {
		name          string
		storyFiles    []string
		mockOutput    string
		goldenFile    string
		wantTaskCount int
		extraCheck    func(t *testing.T, output string)
	}{
		{
			name:          "SingleStory",
			storyFiles:    []string{"story_single_3ac.md"},
			mockOutput:    "mock_single_story.md",
			goldenFile:    "TestBridge_SingleStory.golden",
			wantTaskCount: 3,
		},
		{
			name:       "MultiStory",
			storyFiles: []string{"story_multi_a.md", "story_multi_b.md"},
			mockOutput: "mock_multi_story.md",
			goldenFile: "TestBridge_MultiStory.golden",
			wantTaskCount: 4,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "registration.md") {
					t.Errorf("output missing source reference to registration.md")
				}
				if !strings.Contains(output, "profile.md") {
					t.Errorf("output missing source reference to profile.md")
				}
			},
		},
		{
			name:          "WithDependencies",
			storyFiles:    []string{"story_with_deps.md"},
			mockOutput:    "mock_with_deps.md",
			goldenFile:    "TestBridge_WithDependencies.golden",
			wantTaskCount: 4,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "[SETUP]") {
					t.Errorf("output missing [SETUP] task prefix")
				}
				if !strings.Contains(output, "[E2E]") {
					t.Errorf("output missing [E2E] task prefix")
				}
			},
		},
		{
			name:          "GateMarking",
			storyFiles:    []string{"story_first_of_epic.md"},
			mockOutput:    "mock_gate_marking.md",
			goldenFile:    "TestBridge_GateMarking.golden",
			wantTaskCount: 4,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				// AC: [GATE] on first task AND milestone task — expect >= 2
				gateCount := strings.Count(output, "[GATE]")
				if gateCount < 2 {
					t.Errorf("[GATE] count = %d, want >= 2 (first task + milestone)", gateCount)
				}
			},
		},
		{
			name:          "SourceTraceability",
			storyFiles:    []string{"story_5ac_traceability.md"},
			mockOutput:    "mock_source_traceability.md",
			goldenFile:    "TestBridge_SourceTraceability.golden",
			wantTaskCount: 8,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				for i := 1; i <= 5; i++ {
					ac := fmt.Sprintf("#AC-%d", i)
					if !strings.Contains(output, ac) {
						t.Errorf("output missing source reference %s", ac)
					}
				}
				// Richest fixture: verify service identifiers individually
				for _, svc := range []string{"#SETUP", "#E2E"} {
					if !strings.Contains(output, svc) {
						t.Errorf("output missing service identifier %s", svc)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			storyDir := t.TempDir()

			// Copy story fixtures to storyDir
			var storyPaths []string
			for _, sf := range tc.storyFiles {
				content, err := os.ReadFile(filepath.Join("testdata", sf))
				if err != nil {
					t.Fatalf("read story fixture %q: %v", sf, err)
				}
				dest := filepath.Join(storyDir, sf)
				if err := os.WriteFile(dest, content, 0644); err != nil {
					t.Fatalf("copy story fixture %q: %v", sf, err)
				}
				storyPaths = append(storyPaths, dest)
			}

			// Setup mock Claude with output fixture
			scenario := testutil.Scenario{
				Name: "bridge-golden-" + tc.name,
				Steps: []testutil.ScenarioStep{{
					Type:       "execute",
					ExitCode:   0,
					SessionID:  "mock-golden-" + tc.name,
					OutputFile: tc.mockOutput,
				}},
			}
			testutil.SetupMockClaude(t, scenario)
			copyFixtureToScenario(t, tc.mockOutput)

			cfg := &config.Config{
				ClaudeCommand: os.Args[0],
				ProjectRoot:   projectDir,
				MaxTurns:      5,
			}

			taskCount, _, err := Run(context.Background(), cfg, storyPaths)
			if err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			if taskCount != tc.wantTaskCount {
				t.Errorf("taskCount = %d, want %d", taskCount, tc.wantTaskCount)
			}

			// Read output and compare to golden file
			outPath := filepath.Join(projectDir, "sprint-tasks.md")
			outContent, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			got := string(outContent)
			goldenTest(t, tc.goldenFile, got)

			// Regex validation: every task has a valid source field
			validatedCount := validateTaskSourcePairs(t, got)
			if validatedCount != tc.wantTaskCount {
				t.Errorf("validateTaskSourcePairs count = %d, want %d", validatedCount, tc.wantTaskCount)
			}

			// Per-scenario extra checks
			if tc.extraCheck != nil {
				tc.extraCheck(t, got)
			}
		})
	}
}

// validateMergeTaskSourcePairs verifies every task line (both open and done)
// is immediately followed by a source line. Returns (openCount, doneCount).
// Unlike validateTaskSourcePairs which only checks open tasks, this checks both
// TaskOpenRegex and TaskDoneRegex matches — required for merge test validation.
func validateMergeTaskSourcePairs(t *testing.T, output string) (int, int) {
	t.Helper()
	lines := strings.Split(output, "\n")
	openCount, doneCount := 0, 0
	for i, line := range lines {
		isOpen := config.TaskOpenRegex.MatchString(line)
		isDone := config.TaskDoneRegex.MatchString(line)
		if isOpen || isDone {
			if isOpen {
				openCount++
			} else {
				doneCount++
			}
			if i+1 >= len(lines) || !config.SourceFieldRegex.MatchString(lines[i+1]) {
				t.Errorf("task at line %d has no valid source field on next line", i+1)
			}
		}
	}
	if openCount+doneCount == 0 {
		t.Errorf("merge output has no parseable tasks")
	}
	return openCount, doneCount
}

func TestRun_MergeGoldenFiles(t *testing.T) {
	cases := []struct {
		name          string
		storyFiles    []string
		existingFile  string
		mockOutput    string
		goldenFile    string
		wantTaskCount int
		wantDoneCount int
		extraCheck    func(t *testing.T, output string)
	}{
		{
			name:          "MergeWithCompleted",
			storyFiles:    []string{"story_single_3ac.md"},
			existingFile:  "existing_completed.md",
			mockOutput:    "mock_merge_completed.md",
			goldenFile:    "TestBridge_MergeWithCompleted.golden",
			wantTaskCount: 3,
			wantDoneCount: 2,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				// Verify [x] count preserved
				doneCount := strings.Count(output, "- [x]")
				if doneCount < 2 {
					t.Errorf("[x] count = %d, want >= 2", doneCount)
				}
				// Verify new open tasks added
				openCount := strings.Count(output, "- [ ]")
				if openCount < 3 {
					t.Errorf("[ ] count = %d, want >= 3", openCount)
				}
				// Verify task order preservation: [x] tasks from existing fixture
				// appear in original order
				task1Desc := "Implement user login endpoint"
				task2Desc := "Add input validation for login"
				idx1 := strings.Index(output, task1Desc)
				idx2 := strings.Index(output, task2Desc)
				if idx1 < 0 {
					t.Errorf("output missing first [x] task description %q", task1Desc)
				}
				if idx2 < 0 {
					t.Errorf("output missing second [x] task description %q", task2Desc)
				}
				if idx1 >= 0 && idx2 >= 0 && idx1 >= idx2 {
					t.Errorf("task order not preserved: %q (idx %d) should appear before %q (idx %d)",
						task1Desc, idx1, task2Desc, idx2)
				}
			},
		},
		{
			name:          "MergeDescriptionChange",
			storyFiles:    []string{"story_single_3ac.md"},
			existingFile:  "existing_changed_desc.md",
			mockOutput:    "mock_merge_description.md",
			goldenFile:    "TestBridge_MergeDescriptionChange.golden",
			wantTaskCount: 3,
			wantDoneCount: 2,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				// Verify [x] count preserved
				doneCount := strings.Count(output, "- [x]")
				if doneCount < 2 {
					t.Errorf("[x] count = %d, want >= 2", doneCount)
				}
				// Verify updated description present (from mock output)
				if !strings.Contains(output, "OAuth2") {
					t.Errorf("output missing updated description substring 'OAuth2'")
				}
			},
		},
		{
			name:          "MergeAddGate",
			storyFiles:    []string{"story_single_3ac.md"},
			existingFile:  "existing_no_gate.md",
			mockOutput:    "mock_merge_gate.md",
			goldenFile:    "TestBridge_MergeAddGate.golden",
			wantTaskCount: 3,
			wantDoneCount: 2,
			extraCheck: func(t *testing.T, output string) {
				t.Helper()
				// Verify [GATE] present (was absent in existing)
				if !strings.Contains(output, "[GATE]") {
					t.Errorf("output missing [GATE] (should have been added by merge)")
				}
				// Verify [x] status preserved
				doneCount := strings.Count(output, "- [x]")
				if doneCount < 2 {
					t.Errorf("[x] count = %d, want >= 2", doneCount)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			storyDir := t.TempDir()

			// Copy existing fixture to projectDir as sprint-tasks.md (triggers merge mode)
			existingContent, err := os.ReadFile(filepath.Join("testdata", tc.existingFile))
			if err != nil {
				t.Fatalf("read existing fixture %q: %v", tc.existingFile, err)
			}
			existingPath := filepath.Join(projectDir, "sprint-tasks.md")
			if err := os.WriteFile(existingPath, existingContent, 0644); err != nil {
				t.Fatalf("write existing sprint-tasks.md: %v", err)
			}

			// Copy story fixtures to storyDir
			var storyPaths []string
			for _, sf := range tc.storyFiles {
				content, err := os.ReadFile(filepath.Join("testdata", sf))
				if err != nil {
					t.Fatalf("read story fixture %q: %v", sf, err)
				}
				dest := filepath.Join(storyDir, sf)
				if err := os.WriteFile(dest, content, 0644); err != nil {
					t.Fatalf("copy story fixture %q: %v", sf, err)
				}
				storyPaths = append(storyPaths, dest)
			}

			// Setup mock Claude with merge output fixture
			scenario := testutil.Scenario{
				Name: "bridge-merge-" + tc.name,
				Steps: []testutil.ScenarioStep{{
					Type:       "execute",
					ExitCode:   0,
					SessionID:  "mock-merge-" + tc.name,
					OutputFile: tc.mockOutput,
				}},
			}
			testutil.SetupMockClaude(t, scenario)
			copyFixtureToScenario(t, tc.mockOutput)

			cfg := &config.Config{
				ClaudeCommand: os.Args[0],
				ProjectRoot:   projectDir,
				MaxTurns:      5,
			}

			taskCount, _, runErr := Run(context.Background(), cfg, storyPaths)
			if runErr != nil {
				t.Fatalf("Run() error: %v", runErr)
			}

			if taskCount != tc.wantTaskCount {
				t.Errorf("taskCount = %d, want %d", taskCount, tc.wantTaskCount)
			}

			// Verify .bak file exists and matches original byte-for-byte
			bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
			bakContent, bakErr := os.ReadFile(bakPath)
			if bakErr != nil {
				t.Fatalf("read .bak file: %v", bakErr)
			}
			if string(bakContent) != string(existingContent) {
				t.Errorf(".bak content does not match original existing fixture")
			}

			// Read output and compare to golden file
			outPath := filepath.Join(projectDir, "sprint-tasks.md")
			outContent, readErr := os.ReadFile(outPath)
			if readErr != nil {
				t.Fatalf("read output: %v", readErr)
			}
			got := string(outContent)
			goldenTest(t, tc.goldenFile, got)

			// Regex validation: every task (open + done) has valid source field
			openCount, doneCount := validateMergeTaskSourcePairs(t, got)
			if openCount != tc.wantTaskCount {
				t.Errorf("open task count = %d, want %d", openCount, tc.wantTaskCount)
			}
			if doneCount != tc.wantDoneCount {
				t.Errorf("done task count = %d, want %d", doneCount, tc.wantDoneCount)
			}

			// Per-scenario extra checks
			if tc.extraCheck != nil {
				tc.extraCheck(t, got)
			}
		})
	}
}

func TestRun_MergeSessionFailure(t *testing.T) {
	projectDir := t.TempDir()
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Create existing sprint-tasks.md to trigger merge mode
	existingContent := "## Epic: Auth\n\n- [x] Existing task\n  source: stories/auth.md#AC-1\n"
	existingPath := filepath.Join(projectDir, "sprint-tasks.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	// Mock Claude returns exit code 1
	scenario := testutil.Scenario{
		Name: "bridge-merge-session-fail",
		Steps: []testutil.ScenarioStep{
			{Type: "execute", ExitCode: 1, SessionID: "mock-merge-fail"},
		},
	}
	testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	_, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err == nil {
		t.Fatal("expected error for session failure in merge mode")
	}

	if !strings.Contains(err.Error(), "bridge: execute:") {
		t.Errorf("error = %q, want prefix 'bridge: execute:'", err.Error())
	}

	// Verify .bak file exists and matches original
	bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
	bakContent, bakErr := os.ReadFile(bakPath)
	if bakErr != nil {
		t.Fatalf("read .bak file: %v", bakErr)
	}
	if string(bakContent) != existingContent {
		t.Errorf(".bak content does not match original")
	}

	// Verify original sprint-tasks.md unchanged
	afterContent, readErr := os.ReadFile(existingPath)
	if readErr != nil {
		t.Fatalf("read original after failure: %v", readErr)
	}
	if string(afterContent) != existingContent {
		t.Errorf("original sprint-tasks.md was modified during session failure")
	}
}

func TestRun_MergeBackupFailure(t *testing.T) {
	projectDir := t.TempDir()
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// 1. Create existing file FIRST (triggers merge mode)
	existingContent := "## Epic: Auth\n\n- [x] Existing task\n  source: stories/auth.md#AC-1\n"
	existingPath := filepath.Join(projectDir, "sprint-tasks.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	// 2. Block backup path with directory
	bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
	if err := os.MkdirAll(bakPath, 0755); err != nil {
		t.Fatalf("create blocker dir: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	_, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err == nil {
		t.Fatal("expected error for backup failure")
	}

	if !strings.Contains(err.Error(), "bridge: merge: backup:") {
		t.Errorf("error = %q, want prefix 'bridge: merge: backup:'", err.Error())
	}

	// Verify original sprint-tasks.md content unchanged
	afterContent, readErr := os.ReadFile(existingPath)
	if readErr != nil {
		t.Fatalf("read original after backup failure: %v", readErr)
	}
	if string(afterContent) != existingContent {
		t.Errorf("original sprint-tasks.md was modified during backup failure")
	}
}

func TestRun_MergeParseError(t *testing.T) {
	projectDir := t.TempDir()
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Create existing sprint-tasks.md to trigger merge mode
	existingContent := "## Epic: Auth\n\n- [x] Existing task\n  source: stories/auth.md#AC-1\n"
	existingPath := filepath.Join(projectDir, "sprint-tasks.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	// BRIDGE_TEST_EMPTY_OUTPUT makes subprocess exit 0 with empty stdout → ParseResult fails
	t.Setenv("BRIDGE_TEST_EMPTY_OUTPUT", "1")

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	_, _, err := Run(context.Background(), cfg, []string{storyPath})
	if err == nil {
		t.Fatal("expected error for parse result failure in merge mode")
	}

	if !strings.Contains(err.Error(), "bridge: parse result:") {
		t.Errorf("error = %q, want prefix 'bridge: parse result:'", err.Error())
	}

	// Verify .bak file exists and matches original byte-for-byte
	bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
	bakContent, bakErr := os.ReadFile(bakPath)
	if bakErr != nil {
		t.Fatalf("read .bak file: %v", bakErr)
	}
	if string(bakContent) != existingContent {
		t.Errorf(".bak content does not match original")
	}

	// Verify original sprint-tasks.md unchanged
	afterContent, readErr := os.ReadFile(existingPath)
	if readErr != nil {
		t.Fatalf("read original after parse error: %v", readErr)
	}
	if string(afterContent) != existingContent {
		t.Errorf("original sprint-tasks.md was modified during parse error")
	}
}

// TestRun_MergeReadExistingFailure documents the "bridge: merge: read existing:"
// error path coverage gap. This path requires os.Stat to succeed (file exists, not
// a dir) but os.ReadFile to fail — a condition that's unreliable to trigger on
// WSL/NTFS (permissions don't work, broken symlinks fail Stat too).
// The test attempts a symlink approach but accepts the gap if it can't trigger
// the exact condition.
func TestRun_MergeReadExistingFailure(t *testing.T) {
	projectDir := t.TempDir()
	storyPath := filepath.Join(t.TempDir(), "story.md")
	if err := os.WriteFile(storyPath, []byte("## Story\n\nContent"), 0644); err != nil {
		t.Fatalf("write story: %v", err)
	}

	// Broken symlink: os.Stat follows symlinks and returns error → merge mode
	// NOT triggered. This approach cannot reliably trigger the error path.
	existingPath := filepath.Join(projectDir, "sprint-tasks.md")
	if err := os.Symlink("/nonexistent/target/file", existingPath); err != nil {
		t.Skipf("cannot create symlink on this filesystem: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand: os.Args[0],
		ProjectRoot:   projectDir,
		MaxTurns:      5,
	}

	_, _, err := Run(context.Background(), cfg, []string{storyPath})

	// The "bridge: merge: read existing:" path requires Stat success + ReadFile failure.
	if err != nil && strings.Contains(err.Error(), "bridge: merge: read existing:") {
		// Verify no .bak file created (backup happens AFTER successful read)
		bakPath := filepath.Join(projectDir, "sprint-tasks.md.bak")
		if _, statErr := os.Stat(bakPath); statErr == nil {
			t.Error(".bak file should not exist when read fails before backup")
		}
		return
	}

	// Accepted gap: Stat fails on broken symlink → no merge mode → create mode.
	t.Skipf("could not trigger merge read error path; accepted coverage gap on WSL/NTFS")
}

// TestRun_CrossValidation_BridgeToScanner documents the bridge→runner contract:
// bridge output is consumable by config regex patterns used by the runner scanner.
// Loads the richest golden file and validates with bridge-relevant regex patterns
// (TaskOpenRegex, SourceFieldRegex, GateTagRegex). TaskDoneRegex is not tested
// because bridge only produces open tasks — runner marks them done.
func TestRun_CrossValidation_BridgeToScanner(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "TestBridge_SourceTraceability.golden"))
	if err != nil {
		t.Fatalf("read golden file: %v (run TestRun_GoldenFiles with -update first)", err)
	}
	output := string(golden)

	// Validate all tasks have source fields (1:1 mapping)
	taskCount := validateTaskSourcePairs(t, output)

	lines := strings.Split(output, "\n")
	sourceCount := 0
	for _, line := range lines {
		if config.SourceFieldRegex.MatchString(line) {
			sourceCount++
		}
	}
	if taskCount != sourceCount {
		t.Errorf("task lines = %d, source lines = %d — not 1:1", taskCount, sourceCount)
	}

	// Verify GateTagRegex finds at least one match
	hasGate := false
	for _, line := range lines {
		if config.GateTagRegex.MatchString(line) {
			hasGate = true
			break
		}
	}
	if !hasGate {
		t.Errorf("golden file has no [GATE] match for GateTagRegex")
	}

	// Verify at least one service prefix exists
	hasService := strings.Contains(output, "[SETUP]") ||
		strings.Contains(output, "[E2E]")
	if !hasService {
		t.Errorf("golden file missing service prefixes ([SETUP] or [E2E])")
	}
}
