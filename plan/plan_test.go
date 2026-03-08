package plan

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
	"github.com/bmad-ralph/bmad-ralph/internal/testutil"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	if testutil.RunMockClaude() {
		return
	}
	os.Exit(m.Run())
}

// goldenTest compares got against a golden file, updating it if -update flag is set.
func goldenTest(t *testing.T, goldenFile, got string) {
	t.Helper()
	golden := filepath.Join("testdata", goldenFile)
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden file read: %v (run with -update to create)", err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden file %s.\nGot:\n%s\nWant:\n%s", golden, got, string(want))
	}
}

func TestPlanPrompt_Generate(t *testing.T) {
	inputs := []PlanInput{
		{
			File:    "requirements.md",
			Role:    "requirements",
			Content: []byte("# Requirements\n\nBuild a REST API."),
		},
		{
			File:    "tech-context.md",
			Role:    "technical_context",
			Content: []byte("# Tech\n\nGo 1.25, Cobra CLI."),
		},
	}

	got, err := GeneratePrompt(inputs, "docs/sprint-tasks.md", false, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify typed headers present
	if !strings.Contains(got, "<!-- file: requirements.md | role: requirements -->") {
		t.Error("missing typed header for requirements.md")
	}
	if !strings.Contains(got, "<!-- file: tech-context.md | role: technical_context -->") {
		t.Error("missing typed header for tech-context.md")
	}

	// Verify content injection
	if !strings.Contains(got, "Build a REST API.") {
		t.Error("missing content from requirements.md")
	}
	if !strings.Contains(got, "Go 1.25, Cobra CLI.") {
		t.Error("missing content from tech-context.md")
	}

	// Verify merge section absent
	if strings.Contains(got, "MERGE mode") {
		t.Error("merge section should be absent when merge=false")
	}

	// Verify task format markers
	if !strings.Contains(got, "- [ ]") {
		t.Error("missing task format example")
	}
	if !strings.Contains(got, "[GATE]") {
		t.Error("missing GATE tag reference")
	}
	if !strings.Contains(got, "source:") {
		t.Error("missing source reference instruction")
	}

	goldenTest(t, "TestPlanPrompt_Generate.golden", got)
}

func TestPlanPrompt_Generate_Merge(t *testing.T) {
	inputs := []PlanInput{
		{
			File:    "story.md",
			Role:    "requirements",
			Content: []byte("# New Story\n\nAdd feature X."),
		},
	}
	existing := "- [x] existing task\n  source: old-story.md#AC-1"

	got, err := GeneratePrompt(inputs, "docs/sprint-tasks.md", true, existing, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify merge section present
	if !strings.Contains(got, "MERGE mode") {
		t.Error("merge section should be present when merge=true")
	}

	// Verify existing content injected
	if !strings.Contains(got, "existing task") {
		t.Error("existing tasks content not injected")
	}
	if !strings.Contains(got, "- [x]") {
		t.Error("existing completed task marker not present")
	}

	// Verify typed header
	if !strings.Contains(got, "<!-- file: story.md | role: requirements -->") {
		t.Error("missing typed header for story.md")
	}
}

func TestRun_GenerateSuccess(t *testing.T) {
	projectDir := t.TempDir()
	taskContent := "- [ ] First task\n  source: req.md#AC-1"

	scenario := testutil.Scenario{
		Name: "generate_success",
		Steps: []testutil.ScenarioStep{
			{
				Type:      "execute",
				ExitCode:  0,
				SessionID: "plan-session-1",
				OutputFile: "output.txt",
			},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario) // second return is stateDir, unused; t.Fatal on error

	// Write the output file relative to scenario dir
	scenarioDir := filepath.Dir(scenarioPath)
	if err := os.WriteFile(filepath.Join(scenarioDir, "output.txt"), []byte(taskContent), 0644); err != nil {
		t.Fatalf("write output file: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{
				File:    "req.md",
				Role:    "requirements",
				Content: []byte("# Requirements"),
			},
		},
		NoReview: true,
	}

	err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify file was written
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if !strings.Contains(string(data), "First task") {
		t.Errorf("output file content = %q, want containing 'First task'", string(data))
	}
}

func TestRun_GenerateFailure(t *testing.T) {
	projectDir := t.TempDir()

	scenario := testutil.Scenario{
		Name: "generate_failure",
		Steps: []testutil.ScenarioStep{
			{
				Type:      "execute",
				ExitCode:  1,
				SessionID: "plan-session-fail",
				IsError:   true,
			},
		},
	}
	_, _ = testutil.SetupMockClaude(t, scenario)

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{
				File:    "req.md",
				Role:    "requirements",
				Content: []byte("# Requirements"),
			},
		},
		NoReview: true,
	}

	err := Run(context.Background(), cfg, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "plan: generate:") {
		t.Errorf("error = %q, want containing 'plan: generate:'", err.Error())
	}

	// Verify file was NOT created
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("output file should not exist after failure")
	}
}

// --- Story 13.2: Merge mode integration test ---

func TestRun_MergeMode(t *testing.T) {
	projectDir := t.TempDir()
	// Existing plan with completed task
	existingContent := "## Story 1.1\n\n- [x] Done task\n  source: prd.md#FR1\n"
	// Generated plan has existing story + new story
	generatedContent := "## Story 1.1\n\n- [ ] Dup task\n\n## Story 2.1\n\n- [ ] New task\n  source: prd.md#FR2\n"

	scenario := testutil.Scenario{
		Name: "merge_mode",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-generate",
				OutputFile: "output.txt",
			},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario) // second return is stateDir, unused; t.Fatal on error
	scenarioDir := filepath.Dir(scenarioPath)

	if err := os.WriteFile(filepath.Join(scenarioDir, "output.txt"), []byte(generatedContent), 0644); err != nil {
		t.Fatalf("write output: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{File: "prd.md", Role: "requirements", Content: []byte("# Requirements")},
		},
		NoReview:        true,
		Merge:           true,
		ExistingContent: []byte(existingContent),
	}

	err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("output file not created: %v", readErr)
	}

	result := string(data)
	// Completed task preserved
	if !strings.Contains(result, "- [x] Done task") {
		t.Error("completed task should be preserved in merge")
	}
	// New story appended
	if !strings.Contains(result, "## Story 2.1") {
		t.Error("new story should be appended")
	}
	if !strings.Contains(result, "New task") {
		t.Error("new task content should be present")
	}
	// Duplicate story NOT duplicated
	count := strings.Count(result, "## Story 1.1")
	if count != 1 {
		t.Errorf("Story 1.1 appears %d times, want 1 (dedup)", count)
	}
	// Dup task should not be present (from generated duplicate story)
	if strings.Contains(result, "Dup task") {
		t.Error("duplicate story's tasks should be skipped")
	}
}

// --- Story 12.2: Review flow tests ---

func TestRun_ReviewOK(t *testing.T) {
	projectDir := t.TempDir()
	taskContent := "- [ ] First task\n  source: req.md#AC-1"

	scenario := testutil.Scenario{
		Name: "review_ok",
		Steps: []testutil.ScenarioStep{
			{
				Type:      "execute",
				ExitCode:  0,
				SessionID: "plan-generate",
				OutputFile: "output.txt",
			},
			{
				Type:      "execute",
				ExitCode:  0,
				SessionID: "plan-review",
				OutputFile: "review-output.txt",
			},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario) // second return is stateDir, unused; t.Fatal on error
	scenarioDir := filepath.Dir(scenarioPath)

	if err := os.WriteFile(filepath.Join(scenarioDir, "output.txt"), []byte(taskContent), 0644); err != nil {
		t.Fatalf("write output file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "review-output.txt"), []byte("OK"), 0644); err != nil {
		t.Fatalf("write review output file: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{File: "req.md", Role: "requirements", Content: []byte("# Requirements")},
		},
	}

	err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify file was written
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("output file not created: %v", readErr)
	}
	if !strings.Contains(string(data), "First task") {
		t.Errorf("output content = %q, want containing 'First task'", string(data))
	}
}

func TestRun_ReviewIssuesTriggersRetry(t *testing.T) {
	projectDir := t.TempDir()
	taskContent := "- [ ] First task\n  source: req.md#AC-1"
	issuesText := "ISSUES:\n- FR3 not covered\n- Task 5 too large"

	// Only 2 steps: generate + review. Retry generate will fail (no more steps).
	scenario := testutil.Scenario{
		Name: "review_issues_retry",
		Steps: []testutil.ScenarioStep{
			{
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-generate",
				OutputFile: "output.txt",
			},
			{
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-review",
				OutputFile: "review-output.txt",
			},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario) // second return is stateDir, unused; t.Fatal on error
	scenarioDir := filepath.Dir(scenarioPath)

	if err := os.WriteFile(filepath.Join(scenarioDir, "output.txt"), []byte(taskContent), 0644); err != nil {
		t.Fatalf("write output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "review-output.txt"), []byte(issuesText), 0644); err != nil {
		t.Fatalf("write review-output: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{File: "req.md", Role: "requirements", Content: []byte("# Requirements")},
		},
	}

	err := Run(context.Background(), cfg, opts)
	if err == nil {
		t.Fatal("expected error from retry generate, got nil")
	}
	// Retry generate fails because mock has no more steps
	if !strings.Contains(err.Error(), "plan: generate:") {
		t.Errorf("error = %q, want containing 'plan: generate:'", err.Error())
	}

	// Verify file was NOT written
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Error("output file should not exist when retry failed")
	}
}

func TestResolveRole_Scenarios(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		explicit    string
		singleDoc   bool
		wantRole    string
	}{
		{
			name:     "default mapping prd.md",
			filename: "prd.md",
			wantRole: "requirements",
		},
		{
			name:     "default mapping architecture.md",
			filename: "architecture.md",
			wantRole: "technical_context",
		},
		{
			name:     "default mapping ux-design.md",
			filename: "ux-design.md",
			wantRole: "design_context",
		},
		{
			name:     "default mapping front-end-spec.md",
			filename: "front-end-spec.md",
			wantRole: "ui_spec",
		},
		{
			name:     "unknown file no default",
			filename: "random.md",
			wantRole: "",
		},
		{
			name:     "explicit override",
			filename: "prd.md",
			explicit: "custom_role",
			wantRole: "custom_role",
		},
		{
			name:      "single doc mode",
			filename:  "prd.md",
			singleDoc: true,
			wantRole:  "",
		},
		{
			name:      "single doc with explicit",
			filename:  "prd.md",
			explicit:  "custom_role",
			singleDoc: true,
			wantRole:  "",
		},
		{
			name:     "path with directory",
			filename: "docs/prd.md",
			wantRole: "requirements",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveRole(tt.filename, tt.explicit, tt.singleDoc)
			if got != tt.wantRole {
				t.Errorf("ResolveRole(%q, %q, %v) = %q, want %q",
					tt.filename, tt.explicit, tt.singleDoc, got, tt.wantRole)
			}
		})
	}
}

// --- Story 12.3: Retry flow tests ---

func TestRun_RetrySuccess(t *testing.T) {
	projectDir := t.TempDir()
	taskContent := "- [ ] First task\n  source: req.md#AC-1"
	retryContent := "- [ ] Improved task\n  source: req.md#AC-1"
	issuesText := "ISSUES:\n- FR3 not covered"

	scenario := testutil.Scenario{
		Name: "retry_success",
		Steps: []testutil.ScenarioStep{
			{ // generate
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-generate",
				OutputFile: "output.txt",
			},
			{ // review → ISSUES
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-review-1",
				OutputFile: "review-output-1.txt",
			},
			{ // retry generate with feedback
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-retry",
				OutputFile: "retry-output.txt",
			},
			{ // retry review → OK
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-review-2",
				OutputFile: "review-output-2.txt",
			},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario) // second return is stateDir, unused; t.Fatal on error
	scenarioDir := filepath.Dir(scenarioPath)

	if err := os.WriteFile(filepath.Join(scenarioDir, "output.txt"), []byte(taskContent), 0644); err != nil {
		t.Fatalf("write output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "review-output-1.txt"), []byte(issuesText), 0644); err != nil {
		t.Fatalf("write review-output-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "retry-output.txt"), []byte(retryContent), 0644); err != nil {
		t.Fatalf("write retry-output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "review-output-2.txt"), []byte("OK"), 0644); err != nil {
		t.Fatalf("write review-output-2: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{File: "req.md", Role: "requirements", Content: []byte("# Requirements")},
		},
	}

	err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify retry output was written (not original)
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("output file not created: %v", readErr)
	}
	if !strings.Contains(string(data), "Improved task") {
		t.Errorf("output = %q, want containing 'Improved task'", string(data))
	}
	if strings.Contains(string(data), "First task") {
		t.Error("original content should be replaced by retry, not concatenated")
	}
}

func TestRun_RetryExhausted_AutoProceeds(t *testing.T) {
	projectDir := t.TempDir()
	retryContent := "- [ ] Retry task\n  source: req.md#AC-1"
	issuesText := "ISSUES:\n- FR3 not covered"

	scenario := testutil.Scenario{
		Name: "retry_exhausted_auto_proceed",
		Steps: []testutil.ScenarioStep{
			{ // generate
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-generate",
				OutputFile: "output.txt",
			},
			{ // review → ISSUES
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-review-1",
				OutputFile: "review-output-1.txt",
			},
			{ // retry generate
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-retry",
				OutputFile: "retry-output.txt",
			},
			{ // retry review → ISSUES again (MaxRetries=1 → exhausted, auto-proceed)
				Type:       "execute",
				ExitCode:   0,
				SessionID:  "plan-review-2",
				OutputFile: "review-output-2.txt",
			},
		},
	}
	scenarioPath, _ := testutil.SetupMockClaude(t, scenario)
	scenarioDir := filepath.Dir(scenarioPath)

	if err := os.WriteFile(filepath.Join(scenarioDir, "output.txt"), []byte("- [ ] Original"), 0644); err != nil {
		t.Fatalf("write output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "review-output-1.txt"), []byte(issuesText), 0644); err != nil {
		t.Fatalf("write review-output-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "retry-output.txt"), []byte(retryContent), 0644); err != nil {
		t.Fatalf("write retry-output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "review-output-2.txt"), []byte("ISSUES:\n- Still bad"), 0644); err != nil {
		t.Fatalf("write review-output-2: %v", err)
	}

	cfg := &config.Config{
		ClaudeCommand:  os.Args[0],
		ProjectRoot:    projectDir,
		PlanOutputPath: "sprint-tasks.md",
		MaxTurns:       5,
	}

	opts := PlanOpts{
		Inputs: []PlanInput{
			{File: "req.md", Role: "requirements", Content: []byte("# Requirements")},
		},
		MaxRetries: 1, // 1 retry → after retry review still fails → auto-proceed
	}

	err := Run(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("Run() error: %v (want auto-proceed, not gate error)", err)
	}

	// Verify retry output was written automatically despite remaining issues
	outPath := filepath.Join(projectDir, "sprint-tasks.md")
	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("output file not created: %v", readErr)
	}
	if !strings.Contains(string(data), "Retry task") {
		t.Errorf("output = %q, want containing 'Retry task'", string(data))
	}
}

// --- Story 12.1: Review prompt tests ---

func TestPlanPrompt_Review(t *testing.T) {
	inputs := []PlanInput{
		{
			File:    "prd.md",
			Role:    "requirements",
			Content: []byte("# PRD\n\nFR1: User login"),
		},
	}
	generatedPlan := "- [ ] Implement user login\n  source: prd.md#FR1"

	got, err := GenerateReviewPrompt(inputs, generatedPlan)
	if err != nil {
		t.Fatalf("GenerateReviewPrompt error: %v", err)
	}

	// Verify input document typed header present
	if !strings.Contains(got, "<!-- file: prd.md | role: requirements -->") {
		t.Error("missing typed header for prd.md")
	}

	// Verify input content injected
	if !strings.Contains(got, "FR1: User login") {
		t.Error("missing input content from prd.md")
	}

	// Verify generated plan injected
	if !strings.Contains(got, "Implement user login") {
		t.Error("missing generated plan content")
	}
	if !strings.Contains(got, "source: prd.md#FR1") {
		t.Error("missing source reference in injected plan")
	}

	// Verify review checklist instructions
	if !strings.Contains(got, "FR Coverage") {
		t.Error("missing FR Coverage checklist item")
	}
	if !strings.Contains(got, "Granularity") {
		t.Error("missing Granularity checklist item")
	}
	if !strings.Contains(got, "Source References") {
		t.Error("missing Source References checklist item")
	}
	if !strings.Contains(got, "SETUP and GATE Tasks") {
		t.Error("missing SETUP/GATE checklist item")
	}
	if !strings.Contains(got, "No Duplication") {
		t.Error("missing No Duplication checklist item")
	}

	// Verify response format instructions
	if !strings.Contains(got, "ISSUES:") {
		t.Error("missing ISSUES response format")
	}
	if !strings.Contains(got, "OK") {
		t.Error("missing OK response format")
	}

	// Golden file comparison
	goldenTest(t, "TestPlanPrompt_Review", got)
}

func TestPlanPrompt_Generate_Replan(t *testing.T) {
	inputs := []PlanInput{
		{File: "prd.md", Role: "requirements", Content: []byte("# PRD\nBuild auth")},
	}

	completedTasks := "- [x] Setup project scaffold\n  source: prd.md#AC-1\n- [x] Auth endpoint\n  source: prd.md#AC-2"

	got, err := GeneratePrompt(inputs, "docs/sprint-tasks.md", false, "", completedTasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Replan section must be present
	if !strings.Contains(got, "Replan Mode") {
		t.Error("missing Replan Mode section")
	}
	if !strings.Contains(got, "PRESERVED") {
		t.Error("missing PRESERVED instruction")
	}
	if !strings.Contains(got, "- [x] Setup project scaffold") {
		t.Error("missing completed task in prompt")
	}
	if !strings.Contains(got, "- [x] Auth endpoint") {
		t.Error("missing second completed task in prompt")
	}
	// Merge section must NOT be present
	if strings.Contains(got, "Merge Mode") {
		t.Error("unexpected Merge Mode section in replan prompt")
	}
}
