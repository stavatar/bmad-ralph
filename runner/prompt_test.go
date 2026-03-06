package runner

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/bmad-ralph/bmad-ralph/config"
)

var update = flag.Bool("update", false, "update golden files")

// executeReplacements returns a base replacements map with all execute template
// placeholders set to empty. Callers override specific keys as needed.
func executeReplacements() map[string]string {
	return map[string]string{
		"__FORMAT_CONTRACT__":   "",
		"__RALPH_KNOWLEDGE__":   "",
		"__LEARNINGS_CONTENT__": "",
		"__FINDINGS_CONTENT__":  "",
		"__SERENA_HINT__":       "",
		"__TASK_CONTENT__":      "example task",
		"__TASK_HASH__":         "abc123",
	}
}

// reviewReplacements returns a base replacements map with all review template
// placeholders set to empty. Callers override specific keys as needed.
func reviewReplacements() map[string]string {
	return map[string]string{
		"__TASK_CONTENT__":      "",
		"__RALPH_KNOWLEDGE__":   "",
		"__LEARNINGS_CONTENT__": "",
		"__SERENA_HINT__":       "",
	}
}

// goldenTest compares got against a golden file, updating it if -update flag is set.
func goldenTest(t *testing.T, name, got string) {
	t.Helper()
	golden := filepath.Join("testdata", name+".golden")
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
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	if got != string(want) {
		t.Errorf("output mismatch (run with -update to refresh)\ngot:\n%s\nwant:\n%s", got, string(want))
	}
}

func TestPrompt_Execute_WithFindings(t *testing.T) {
	findingsText := "- [HIGH] Missing input validation in handler.go:42\n- [MED] Unused import in utils.go:3"
	data := config.TemplateData{
		HasFindings: true,
	}
	replacements := executeReplacements()
	replacements["__FORMAT_CONTRACT__"] = config.SprintTasksFormat()
	replacements["__FINDINGS_CONTENT__"] = findingsText

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// Verify all 8 required items from AC #1 (7 sections + format contract injection)
	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// AC #1 required sections
		{"999-rules guardrail section", "999-Rules Guardrails", true},
		{"ATDD instruction", "Acceptance Test-Driven Development", true},
		{"zero-skip policy", "Zero-Skip Policy", true},
		{"red-green cycle", "Red-Green Cycle", true},
		{"self-directing instruction", "sprint-tasks.md", true},
		{"source story context instruction", "source:` field, open the referenced file", true},
		{"source fallback on missing file", "file is missing, proceed with the task description", true},
		{"mutation asymmetry", "Mutation Asymmetry", true},
		{"commit on green only", "when ALL tests pass (green)", true},
		// Format contract injection (AC #1 item 8)
		{"format contract title", "Sprint Tasks Format Specification", true},
		// Findings section present (AC #2)
		{"findings header", "Review Findings", true},
		{"findings instruction", "MUST FIX FIRST", true},
		{"findings content injected", "Missing input validation", true},
		{"findings content severity", "[HIGH]", true},
		// No-findings proceed absent (AC #3)
		{"no proceed instruction", "No pending review findings", false},
		// Placeholders replaced
		{"no format placeholder", "__FORMAT_CONTRACT__", false},
		{"no findings placeholder", "__FINDINGS_CONTENT__", false},
		// 999-rules specific items (FR36)
		{"rule no delete files", "Do NOT delete files outside", true},
		{"rule no destructive commands", "destructive commands", true},
		{"rule no skip tests", "Do NOT skip", true},
		{"rule no disable linters", "Do NOT disable linters", true},
		// ATDD specific (FR37)
		{"ATDD every AC must have test", "Every acceptance criterion", true},
		// Zero-skip specific (FR38)
		{"zero-skip never xfail", "NEVER use xfail", true},
		// Red-green specific
		{"red phase", "failing test", true},
		{"green phase", "minimal code", true},
		// Self-directing specific (FR11)
		{"self-directing scan", "FIRST task marked", true},
		// Mutation asymmetry specific
		{"mutation no modify markers", "MUST NOT modify task status markers", true},
		// Commit rules (FR8)
		{"commit never with failing", "NEVER commit with failing tests", true},
		// Session completion (BUG-6)
		{"session completion section", "Session Completion", true},
		{"session stop after one task", "STOP the session immediately", true},
		{"session one task only", "exactly one task", true},
		// Rule 10 (BUG-6)
		{"rule 10 no extra tasks", "Do NOT work on tasks you did not select", true},
		// Task scope (DESIGN-2)
		{"task scope section", "Task Scope", true},
		{"task scope only required files", "directly required by the current task", true},
		{"task scope no drive-by", "no drive-by refactoring", true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	goldenTest(t, "TestPrompt_Execute_WithFindings", got)
}

func TestPrompt_Execute_WithoutFindings(t *testing.T) {
	data := config.TemplateData{
		HasFindings: false,
	}
	replacements := executeReplacements()
	replacements["__FORMAT_CONTRACT__"] = config.SprintTasksFormat()

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// All 8 required items still present (AC #1)
		{"999-rules guardrail section", "999-Rules Guardrails", true},
		{"ATDD instruction", "Acceptance Test-Driven Development", true},
		{"zero-skip policy", "Zero-Skip Policy", true},
		{"red-green cycle", "Red-Green Cycle", true},
		{"self-directing instruction", "sprint-tasks.md", true},
		{"source story context instruction", "source:` field, open the referenced file", true},
		{"source fallback on missing file", "file is missing, proceed with the task description", true},
		{"mutation asymmetry", "Mutation Asymmetry", true},
		{"commit on green only", "when ALL tests pass (green)", true},
		{"format contract title", "Sprint Tasks Format Specification", true},
		// No findings section (AC #3)
		{"no findings header", "Review Findings", false},
		{"no findings instruction", "MUST FIX FIRST", false},
		// Proceed instruction present (AC #3)
		{"proceed instruction", "No pending review findings", true},
		// Placeholders replaced / absent
		{"no format placeholder", "__FORMAT_CONTRACT__", false},
		{"no findings placeholder", "__FINDINGS_CONTENT__", false},
		// Gates absent by default
		{"no gates section", "GATES ARE ENABLED", false},
		// Session completion (BUG-6)
		{"session completion section", "Session Completion", true},
		{"session stop after one task", "STOP the session immediately", true},
		// Rule 10 (BUG-6)
		{"rule 10 no extra tasks", "Do NOT work on tasks you did not select", true},
		// Task scope (DESIGN-2)
		{"task scope section", "Task Scope", true},
		{"task scope only required files", "directly required by the current task", true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	goldenTest(t, "TestPrompt_Execute_WithoutFindings", got)
}

func TestPrompt_Execute_FormatContract(t *testing.T) {
	data := config.TemplateData{}
	replacements := executeReplacements()
	replacements["__FORMAT_CONTRACT__"] = config.SprintTasksFormat()

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	if !strings.Contains(got, "Sprint Tasks Format Specification") {
		t.Errorf("expected prompt to contain format contract title 'Sprint Tasks Format Specification'")
	}

	// Verify format contract markers from config/constants.go are present
	markers := []struct {
		name   string
		marker string
	}{
		{"TaskOpen marker", config.TaskOpen},
		{"TaskDone marker", config.TaskDone},
		{"GateTag marker", config.GateTag},
		{"FeedbackPrefix marker", config.FeedbackPrefix},
		{"source field syntax", "source:"},
	}
	for _, m := range markers {
		t.Run(m.name, func(t *testing.T) {
			if !strings.Contains(got, m.marker) {
				t.Errorf("assembled prompt does not contain format contract marker %q", m.marker)
			}
		})
	}
}

func TestPrompt_Execute_WithGates(t *testing.T) {
	data := config.TemplateData{
		GatesEnabled: true,
	}
	replacements := executeReplacements()
	replacements["__FORMAT_CONTRACT__"] = config.SprintTasksFormat()

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		{"gates section present", "GATES ARE ENABLED", true},
		{"gates pause instruction", "pause AFTER your session", true},
		{"gates GATE tag reference", "[GATE]", true},
		// Core sections still present with gates
		{"999-rules still present", "999-Rules Guardrails", true},
		{"self-directing still present", "FIRST task marked", true},
		{"proceed section present", "No pending review findings", true},
		// No findings section (HasFindings=false)
		{"no findings header", "Review Findings", false},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}
}

func TestPrompt_Execute_Injection(t *testing.T) {
	// AC #5: user content containing Go template syntax must be preserved literally
	dangerousContent := "This has {{.Dangerous}} template syntax and {{range .Items}}bad{{end}}"
	data := config.TemplateData{
		HasFindings: true,
	}
	replacements := executeReplacements()
	replacements["__FORMAT_CONTRACT__"] = dangerousContent
	replacements["__FINDINGS_CONTENT__"] = "finding with {{.Exploit}} attempt"

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v (template injection should not cause error)", err)
	}

	// Verify dangerous content preserved literally (not executed by template engine)
	if !strings.Contains(got, "{{.Dangerous}}") {
		t.Errorf("expected {{.Dangerous}} to be preserved literally in output")
	}
	if !strings.Contains(got, "{{range .Items}}bad{{end}}") {
		t.Errorf("expected {{range .Items}} to be preserved literally in output")
	}
	if !strings.Contains(got, "{{.Exploit}}") {
		t.Errorf("expected {{.Exploit}} to be preserved literally in output")
	}
}

// --- Review Prompt Tests (Story 4.4) ---

func TestPrompt_Review(t *testing.T) {
	taskContent := "Implement user authentication for login endpoint"
	data := config.TemplateData{}
	replacements := reviewReplacements()
	replacements["__TASK_CONTENT__"] = taskContent

	got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// Structural assertions covering AC1-AC4
	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// AC1: Verification keywords + ordering/completeness constraints
		{"verification keyword CONFIRMED", "CONFIRMED", true},
		{"verification keyword FALSE POSITIVE", "FALSE POSITIVE", true},
		{"verification instruction", "verify EACH finding", true},
		{"collect before verify", "before proceeding to verification", true},
		{"no skip verification", "Do not skip verification", true},
		// AC2: Severity keywords + constraints
		{"severity CRITICAL", "CRITICAL", true},
		{"severity HIGH", "HIGH", true},
		{"severity MEDIUM", "MEDIUM", true},
		{"severity LOW", "LOW", true},
		{"severity exactly one", "exactly one severity", true},
		{"severity mandatory", "Severity is mandatory", true},
		// AC3: False positive exclusion rule
		{"false positive exclusion", "FALSE POSITIVE findings MUST NOT appear", true},
		// AC4: Finding structure 4-field format + mandatory constraint
		{"finding field description", "**Description**", true},
		{"finding field location", "**Location**", true},
		{"finding field reasoning", "**Reasoning**", true},
		{"finding field recommendation", "**Recommendation**", true},
		{"finding fields mandatory", "All 5 fields are mandatory", true},
		// Sub-agent names via file paths (AC1 orchestration)
		{"sub-agent quality path", "runner/prompts/agents/quality.md", true},
		{"sub-agent implementation path", "runner/prompts/agents/implementation.md", true},
		{"sub-agent simplification path", "runner/prompts/agents/simplification.md", true},
		{"sub-agent design-principles path", "runner/prompts/agents/design-principles.md", true},
		{"sub-agent test-coverage path", "runner/prompts/agents/test-coverage.md", true},
		// Placeholder replaced
		{"no task placeholder", "__TASK_CONTENT__", false},
		// Task content injected
		{"task content injected", taskContent, true},
		// Invariant guardrails
		{"invariant no modify source", "MUST NOT modify source code", true},
		{"invariant may write learnings", "MAY write to LEARNINGS.md", true},
		{"invariant mutation asymmetry", "Mutation Asymmetry", true},
		// Clean review handling (Story 4.5)
		{"clean review mark [x]", "mark [x]", true},
		{"clean review keyword", "CLEAN REVIEW", true},
		{"clean clear findings", "Clear review-findings", true},
		{"clean task checkbox open", "- [ ]", true},
		{"clean task checkbox done", "- [x]", true},
		{"clean sprint-tasks.md ref", "sprint-tasks.md", true},
		{"clean atomic operation", "atomically", true},
		{"clean no git commands", "MUST NOT run any git", true},
		{"clean only allowed files constraint", "ONLY modify sprint-tasks.md, review-findings.md, and LEARNINGS.md", true},
		// Findings write (Story 4.6)
		{"findings overwrite instruction", "overwrite review-findings", true},
		{"findings never appended", "never appended", true},
		{"findings severity format template", "[SEVERITY]", true},
		{"findings format keyword ЧТО", "ЧТО", true},
		{"findings format keyword ГДЕ", "ГДЕ", true},
		{"findings format keyword ПОЧЕМУ", "ПОЧЕМУ", true},
		{"findings format keyword КАК", "КАК", true},
		{"findings self-contained", "self-contained", true},
		{"findings only current task", "ONLY current task findings", true},
		{"findings no mark task done", "Do NOT mark [x]", true},
		// Knowledge Extraction section (Story 6.4)
		{"knowledge extraction section", "Knowledge Extraction", true},
		{"knowledge write on findings", "write lessons to LEARNINGS.md", true},
		{"knowledge atomized fact format", "atomized fact", true},
		{"knowledge categories", "testing, errors, architecture", true},
		{"knowledge no lessons on clean", "Do NOT write lessons on clean review", true},
		{"knowledge append only", "only append new ones", true},
		{"knowledge citation format", "category: topic [review, file:line]", true},
		// CLAUDE.md/config protection (Story 6.4)
		{"invariant no claude.md write", "MUST NOT write to CLAUDE.md", true},
		// Diff-only review instruction (DESIGN-5)
		{"diff-only review instruction", "ONLY the diff", true},
		{"diff-only no pre-existing", "Pre-existing code", true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	goldenTest(t, "TestPrompt_Review", got)
}

// --- Sub-Agent Prompt Tests (Story 4.2) ---

// scopeRef pairs a domain keyword with its owning agent name for OUT-OF-SCOPE validation.
type scopeRef struct {
	keyword string
	agent   string
}

// assertContains verifies text contains substr, reporting msg on failure.
func assertContains(t *testing.T, text, substr, msg string) {
	t.Helper()
	if !strings.Contains(text, substr) {
		t.Errorf("%s: expected to contain %q", msg, substr)
	}
}

// splitAgentSections extracts SCOPE and OUT-OF-SCOPE text from an agent prompt.
func splitAgentSections(t *testing.T, prompt string) (scope, outOfScope string) {
	t.Helper()
	scopeIdx := strings.Index(prompt, "## SCOPE")
	if scopeIdx == -1 {
		t.Fatal("prompt missing '## SCOPE' section")
	}
	outIdx := strings.Index(prompt, "## OUT-OF-SCOPE")
	if outIdx == -1 {
		t.Fatal("prompt missing '## OUT-OF-SCOPE' section")
	}
	scope = prompt[scopeIdx:outIdx]
	instrIdx := strings.Index(prompt, "## Instructions")
	if instrIdx == -1 {
		outOfScope = prompt[outIdx:]
	} else {
		outOfScope = prompt[outIdx:instrIdx]
	}
	return scope, outOfScope
}

// verifyAgentScope validates SCOPE keywords and OUT-OF-SCOPE cross-references for an agent prompt.
func verifyAgentScope(t *testing.T, prompt string, scopeKeywords []string, outOfScopeRefs []scopeRef) {
	t.Helper()
	scope, outOfScope := splitAgentSections(t, prompt)
	for _, kw := range scopeKeywords {
		assertContains(t, scope, kw, "SCOPE")
	}
	for _, ref := range outOfScopeRefs {
		assertContains(t, outOfScope, ref.keyword, "OUT-OF-SCOPE keyword")
		assertContains(t, outOfScope, ref.agent, "OUT-OF-SCOPE agent ref")
	}
}

// Task 3: Scope boundary validation (AC 2-6) — table-driven across all 5 agents.
func TestPrompt_Agent_Scope(t *testing.T) {
	cases := []struct {
		name           string
		prompt         string
		scopeKeywords  []string
		outOfScopeRefs []scopeRef
	}{
		{
			name:          "quality",
			prompt:        agentQualityPrompt,
			scopeKeywords: []string{"Bugs", "Security issues", "Performance problems", "Error handling"},
			outOfScopeRefs: []scopeRef{
				{"Acceptance criteria compliance", "implementation"},
				{"Code simplification", "simplification"},
				{"DRY/KISS/SRP", "design-principles"},
				{"Test coverage", "test-coverage"},
			},
		},
		{
			name:          "implementation",
			prompt:        agentImplementationPrompt,
			scopeKeywords: []string{"Acceptance criteria compliance", "Feature completeness", "Requirement satisfaction"},
			outOfScopeRefs: []scopeRef{
				{"Code quality", "quality"},
				{"Code simplification", "simplification"},
				{"DRY/KISS/SRP", "design-principles"},
				{"Test coverage", "test-coverage"},
			},
		},
		{
			name:          "simplification",
			prompt:        agentSimplificationPrompt,
			scopeKeywords: []string{"Code readability", "Verbose constructs", "Dead code", "Simpler alternatives"},
			outOfScopeRefs: []scopeRef{
				{"Bugs", "quality"},
				{"Acceptance criteria compliance", "implementation"},
				{"DRY/KISS/SRP", "design-principles"},
				{"Test coverage", "test-coverage"},
			},
		},
		{
			name:          "design-principles",
			prompt:        agentDesignPrinciplesPrompt,
			scopeKeywords: []string{"DRY", "KISS", "SRP"},
			outOfScopeRefs: []scopeRef{
				{"Bugs", "quality"},
				{"Acceptance criteria compliance", "implementation"},
				{"Expression-level simplicity", "simplification"},
				{"Test coverage", "test-coverage"},
			},
		},
		{
			name:          "test-coverage",
			prompt:        agentTestCoveragePrompt,
			scopeKeywords: []string{"ATDD", "Zero-skip", "Test quality"},
			outOfScopeRefs: []scopeRef{
				{"Code bugs", "quality"},
				{"Acceptance criteria compliance", "implementation"},
				{"Code simplification", "simplification"},
				{"DRY/KISS/SRP", "design-principles"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verifyAgentScope(t, c.prompt, c.scopeKeywords, c.outOfScopeRefs)
		})
	}
}

// Task 4: Golden file tests (AC 7) — table-driven across all 5 agents.
func TestPrompt_Agent_Golden(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"Quality", agentQualityPrompt},
		{"Implementation", agentImplementationPrompt},
		{"Simplification", agentSimplificationPrompt},
		{"DesignPrinciples", agentDesignPrinciplesPrompt},
		{"TestCoverage", agentTestCoveragePrompt},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goldenTest(t, "TestPrompt_Agent_"+c.name, c.prompt)
		})
	}
}

// Task 5: Adversarial structure tests (AC 8)

// TestPrompt_Agent_ScopeExclusivity verifies each domain keyword appears in exactly
// one agent's SCOPE section — no overlap between the 5 agents.
func TestPrompt_Agent_ScopeExclusivity(t *testing.T) {
	agents := []struct {
		name   string
		prompt string
	}{
		{"quality", agentQualityPrompt},
		{"implementation", agentImplementationPrompt},
		{"simplification", agentSimplificationPrompt},
		{"design-principles", agentDesignPrinciplesPrompt},
		{"test-coverage", agentTestCoveragePrompt},
	}

	scopes := make(map[string]string)
	for _, a := range agents {
		scope, _ := splitAgentSections(t, a.prompt)
		scopes[a.name] = scope
	}

	domains := []struct {
		keyword string
		owner   string
	}{
		{"Bugs", "quality"},
		{"Security issues", "quality"},
		{"Performance problems", "quality"},
		{"Error handling", "quality"},
		{"Acceptance criteria compliance", "implementation"},
		{"Feature completeness", "implementation"},
		{"Requirement satisfaction", "implementation"},
		{"Code readability", "simplification"},
		{"Verbose constructs", "simplification"},
		{"Dead code", "simplification"},
		{"Simpler alternatives", "simplification"},
		{"DRY", "design-principles"},
		{"KISS", "design-principles"},
		{"SRP", "design-principles"},
		{"ATDD", "test-coverage"},
		{"Zero-skip", "test-coverage"},
		{"Test quality", "test-coverage"},
	}

	for _, d := range domains {
		t.Run(d.keyword, func(t *testing.T) {
			var found []string
			for name, scope := range scopes {
				if strings.Contains(scope, d.keyword) {
					found = append(found, name)
				}
			}
			if len(found) == 0 {
				t.Errorf("domain keyword %q not found in any agent's SCOPE", d.keyword)
			} else if len(found) > 1 {
				t.Errorf("domain keyword %q found in multiple agents' SCOPE: %v (expected only %q)", d.keyword, found, d.owner)
			} else if found[0] != d.owner {
				t.Errorf("domain keyword %q found in %q SCOPE, expected %q", d.keyword, found[0], d.owner)
			}
		})
	}
}

// TestPrompt_Agent_OutOfScopeCompleteness verifies each agent's OUT-OF-SCOPE section
// mentions all 4 other agents by name.
func TestPrompt_Agent_OutOfScopeCompleteness(t *testing.T) {
	agents := []struct {
		name   string
		prompt string
	}{
		{"quality", agentQualityPrompt},
		{"implementation", agentImplementationPrompt},
		{"simplification", agentSimplificationPrompt},
		{"design-principles", agentDesignPrinciplesPrompt},
		{"test-coverage", agentTestCoveragePrompt},
	}

	for _, a := range agents {
		t.Run(a.name, func(t *testing.T) {
			_, outOfScope := splitAgentSections(t, a.prompt)
			for _, other := range agents {
				if other.name == a.name {
					continue
				}
				assertContains(t, outOfScope, other.name,
					a.name+" OUT-OF-SCOPE must reference "+other.name)
			}
		})
	}
}

// TestPrompt_Agent_DetectionStructure verifies each prompt has an Instructions section
// with domain-specific guidance keywords enabling issue detection.
func TestPrompt_Agent_DetectionStructure(t *testing.T) {
	cases := []struct {
		name                string
		prompt              string
		instructionKeywords []string
	}{
		{"quality", agentQualityPrompt, []string{"security", "performance"}},
		{"implementation", agentImplementationPrompt, []string{"acceptance criteria", "satisfies", "outside the task's scope"}},
		{"simplification", agentSimplificationPrompt, []string{"verbose", "simpler"}},
		{"design-principles", agentDesignPrinciplesPrompt, []string{"DRY", "KISS", "SRP"}},
		{"test-coverage", agentTestCoveragePrompt, []string{"test coverage", "skip", "t.Errorf"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			instrIdx := strings.Index(c.prompt, "## Instructions")
			if instrIdx == -1 {
				t.Fatalf("%s prompt missing '## Instructions' section", c.name)
			}
			instructions := c.prompt[instrIdx:]
			for _, kw := range c.instructionKeywords {
				assertContains(t, instructions, kw,
					c.name+" Instructions section detection keyword")
			}
			// DESIGN-5: all agents must have diff-only review instruction
			assertContains(t, instructions, "ONLY the diff",
				c.name+" Instructions diff-only review instruction")
			assertContains(t, instructions, "Do NOT criticize pre-existing",
				c.name+" Instructions no pre-existing code criticism")
		})
	}
}

// --- Story 6.2: Knowledge Injection Prompt Tests ---

// Task 9.9: Execute prompt with knowledge sections
func TestPrompt_Execute_KnowledgeSections(t *testing.T) {
	data := config.TemplateData{
		HasLearnings: true,
	}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__":   config.SprintTasksFormat(),
		"__RALPH_KNOWLEDGE__":   "Distilled testing patterns here",
		"__LEARNINGS_CONTENT__": "Recent learning about assertions",
		"__FINDINGS_CONTENT__":  "",
		"__TASK_CONTENT__":      "Test task",
		"__TASK_HASH__":         "",
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		{"distilled knowledge section", "Distilled Knowledge", true},
		{"distilled content injected", "Distilled testing patterns here", true},
		{"recent learnings section", "Recent Learnings", true},
		{"learnings content injected", "Recent learning about assertions", true},
		{"self-review section present", "Self-Review", true},
		{"self-review instruction", "Re-read the top 5 most recent learnings", true},
		{"no knowledge placeholder", "__RALPH_KNOWLEDGE__", false},
		{"no learnings placeholder", "__LEARNINGS_CONTENT__", false},
		// Ordering: distilled BEFORE learnings BEFORE guardrails
		{"guardrails still present", "999-Rules Guardrails", true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	// Verify ordering: distilled → learnings → guardrails
	idxDistilled := strings.Index(got, "Distilled Knowledge")
	idxLearnings := strings.Index(got, "Recent Learnings")
	idxGuardrails := strings.Index(got, "999-Rules Guardrails")
	if idxDistilled >= idxLearnings {
		t.Errorf("Distilled Knowledge (at %d) should come BEFORE Recent Learnings (at %d)", idxDistilled, idxLearnings)
	}
	if idxLearnings >= idxGuardrails {
		t.Errorf("Recent Learnings (at %d) should come BEFORE 999-Rules Guardrails (at %d)", idxLearnings, idxGuardrails)
	}

	goldenTest(t, "TestPrompt_Execute_KnowledgeSections", got)
}

// Task 9.10: Self-review conditional on HasLearnings
func TestPrompt_Execute_SelfReview(t *testing.T) {
	cases := []struct {
		name         string
		hasLearnings bool
		wantReview   bool
	}{
		{"with learnings", true, true},
		{"without learnings", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := config.TemplateData{HasLearnings: tc.hasLearnings}
			replacements := map[string]string{
				"__FORMAT_CONTRACT__":   config.SprintTasksFormat(),
				"__RALPH_KNOWLEDGE__":   "",
				"__LEARNINGS_CONTENT__": "",
				"__TASK_CONTENT__":      "",
				"__TASK_HASH__":         "",
			}

			got, err := config.AssemblePrompt(executeTemplate, data, replacements)
			if err != nil {
				t.Fatalf("AssemblePrompt error: %v", err)
			}

			hasSelfReview := strings.Contains(got, "Self-Review")
			if tc.wantReview && !hasSelfReview {
				t.Errorf("expected Self-Review section to be present")
			}
			if !tc.wantReview && hasSelfReview {
				t.Errorf("expected Self-Review section to be absent")
			}
		})
	}
}

// Task 9.11: Execute prompt with no knowledge files
func TestPrompt_Execute_NoKnowledge(t *testing.T) {
	data := config.TemplateData{
		HasLearnings: false,
	}
	replacements := map[string]string{
		"__FORMAT_CONTRACT__":   config.SprintTasksFormat(),
		"__RALPH_KNOWLEDGE__":   "",
		"__LEARNINGS_CONTENT__": "",
		"__TASK_CONTENT__":      "",
		"__TASK_HASH__":         "",
	}

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		{"no knowledge placeholder", "__RALPH_KNOWLEDGE__", false},
		{"no learnings placeholder", "__LEARNINGS_CONTENT__", false},
		{"no self-review", "Self-Review", false},
		{"guardrails present", "999-Rules Guardrails", true},
		{"format contract present", "Sprint Tasks Format", true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}
}

// Task 9.12: Review prompt with knowledge sections
func TestPrompt_Review_KnowledgeSections(t *testing.T) {
	data := config.TemplateData{}
	replacements := map[string]string{
		"__TASK_CONTENT__":      "Implement feature X",
		"__RALPH_KNOWLEDGE__":   "Distilled review patterns",
		"__LEARNINGS_CONTENT__": "Recent review learning",
	}

	got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		{"distilled section", "Distilled Knowledge", true},
		{"distilled content", "Distilled review patterns", true},
		{"learnings section", "Recent Learnings", true},
		{"learnings content", "Recent review learning", true},
		{"no knowledge placeholder", "__RALPH_KNOWLEDGE__", false},
		{"no learnings placeholder", "__LEARNINGS_CONTENT__", false},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}

	goldenTest(t, "TestPrompt_Review_KnowledgeSections", got)
}

// Task 9.13: Review prompt — __LEARNINGS_CONTENT__ empty for review
func TestPrompt_Review_NoLearningsContent(t *testing.T) {
	data := config.TemplateData{}
	replacements := map[string]string{
		"__TASK_CONTENT__":      "Review task",
		"__RALPH_KNOWLEDGE__":   "Some rules",
		"__LEARNINGS_CONTENT__": "", // H7: review overrides to empty
	}

	got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// Section header exists but content is empty (placeholder replaced with "")
	if !strings.Contains(got, "Recent Learnings") {
		t.Errorf("expected 'Recent Learnings' section header to be present")
	}
	// No actual learnings content between section header and next section
	// The content should effectively be empty after the header
	if strings.Contains(got, "actual learnings text") {
		t.Errorf("should not contain actual learnings text in review mode")
	}
}

// Task 9.14: Review prompt invariant updated
func TestPrompt_Review_InvariantUpdated(t *testing.T) {
	data := config.TemplateData{}
	replacements := map[string]string{
		"__TASK_CONTENT__":      "Review task",
		"__RALPH_KNOWLEDGE__":   "",
		"__LEARNINGS_CONTENT__": "",
	}

	got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// M2: "MUST NOT write LEARNINGS.md" should be REMOVED
	if strings.Contains(got, "MUST NOT write LEARNINGS") {
		t.Errorf("review prompt should NOT contain old invariant 'MUST NOT write LEARNINGS'")
	}
	// New invariant: "MAY write to LEARNINGS.md"
	if !strings.Contains(got, "MAY write to LEARNINGS.md") {
		t.Errorf("review prompt should contain new invariant 'MAY write to LEARNINGS.md'")
	}
}

// --- Story 8.2: Sync prompt template tests ---

// TestSyncPrompt_TemplateParse verifies serena-sync.md compiles as a Go template (AC#1).
func TestSyncPrompt_TemplateParse(t *testing.T) {
	if serenaSyncTemplate == "" {
		t.Fatal("serenaSyncTemplate is empty — go:embed failed")
	}

	// AC#1: Template compiles via text/template.Parse
	_, err := template.New("sync").Parse(serenaSyncTemplate)
	if err != nil {
		t.Fatalf("template.Parse failed: %v", err)
	}

	// Verify template contains expected Stage 2 placeholders
	placeholders := []string{
		"__DIFF_SUMMARY__",
		"__LEARNINGS_CONTENT__",
		"__COMPLETED_TASKS__",
		"__PROJECT_ROOT__",
		"__MAX_TURNS__",
	}
	for _, ph := range placeholders {
		if !strings.Contains(serenaSyncTemplate, ph) {
			t.Errorf("sync template missing placeholder: %q", ph)
		}
	}
}

// TestAssembleSyncPrompt_AllSections verifies full assembly with all conditionals true (AC#2).
func TestAssembleSyncPrompt_AllSections(t *testing.T) {
	got, err := assembleSyncPrompt(SerenaSyncOpts{
		DiffSummary:    "added file foo.go",
		Learnings:      "lesson: always test",
		CompletedTasks: "task 1: done",
		MaxTurns:       5,
		ProjectRoot:    "/my/project",
	})
	if err != nil {
		t.Fatalf("assembleSyncPrompt: unexpected error: %v", err)
	}

	// No unresolved template directives
	if strings.Contains(got, "{{") {
		t.Error("output contains unresolved template directive '{{'")
	}
	// No unreplaced placeholders
	if strings.Contains(got, "__DIFF_SUMMARY__") || strings.Contains(got, "__LEARNINGS_CONTENT__") || strings.Contains(got, "__COMPLETED_TASKS__") {
		t.Error("output contains unreplaced __PLACEHOLDER__")
	}

	// Verify content injected
	if !strings.Contains(got, "added file foo.go") {
		t.Error("output missing diff summary content")
	}
	if !strings.Contains(got, "lesson: always test") {
		t.Error("output missing learnings content")
	}
	if !strings.Contains(got, "task 1: done") {
		t.Error("output missing completed tasks content")
	}
	if !strings.Contains(got, "/my/project") {
		t.Error("output missing project root")
	}
	if !strings.Contains(got, "Максимум ходов: 5") {
		t.Error("output missing max turns value")
	}
}

// TestAssembleSyncPrompt_NoLearnings verifies learnings section absent when empty (AC#5).
func TestAssembleSyncPrompt_NoLearnings(t *testing.T) {
	got, err := assembleSyncPrompt(SerenaSyncOpts{
		DiffSummary:    "some diff",
		Learnings:      "",
		CompletedTasks: "task 1: done",
		MaxTurns:       5,
		ProjectRoot:    "/proj",
	})
	if err != nil {
		t.Fatalf("assembleSyncPrompt: unexpected error: %v", err)
	}

	if strings.Contains(got, "Извлечённые уроки") {
		t.Error("output should NOT contain learnings section header when Learnings is empty")
	}
	// Completed tasks section should still be present
	if !strings.Contains(got, "Завершённые задачи") {
		t.Error("output should contain completed tasks section header")
	}
}

// TestAssembleSyncPrompt_NoCompletedTasks verifies completed tasks section absent when empty (AC#5).
func TestAssembleSyncPrompt_NoCompletedTasks(t *testing.T) {
	got, err := assembleSyncPrompt(SerenaSyncOpts{
		DiffSummary:    "some diff",
		Learnings:      "lesson: test well",
		CompletedTasks: "",
		MaxTurns:       5,
		ProjectRoot:    "/proj",
	})
	if err != nil {
		t.Fatalf("assembleSyncPrompt: unexpected error: %v", err)
	}

	if strings.Contains(got, "Завершённые задачи") {
		t.Error("output should NOT contain completed tasks section header when CompletedTasks is empty")
	}
	// Learnings section should still be present
	if !strings.Contains(got, "Извлечённые уроки") {
		t.Error("output should contain learnings section header")
	}
}

// TestAssembleSyncPrompt_BothSectionsAbsent verifies output when both optional sections are disabled (AC#5).
func TestAssembleSyncPrompt_BothSectionsAbsent(t *testing.T) {
	got, err := assembleSyncPrompt(SerenaSyncOpts{
		DiffSummary: "some diff",
		MaxTurns:    5,
		ProjectRoot: "/proj",
	})
	if err != nil {
		t.Fatalf("assembleSyncPrompt: unexpected error: %v", err)
	}

	if strings.Contains(got, "Извлечённые уроки") {
		t.Error("output should NOT contain learnings section header")
	}
	if strings.Contains(got, "Завершённые задачи") {
		t.Error("output should NOT contain completed tasks section header")
	}
	// Core sections still present
	if !strings.Contains(got, "Diff summary") {
		t.Error("output should contain diff summary section")
	}
	if !strings.Contains(got, "Инструкции") {
		t.Error("output should contain instructions section")
	}
}

// TestAssembleSyncPrompt_Instructions verifies key prompt instructions (AC#4).
func TestAssembleSyncPrompt_Instructions(t *testing.T) {
	got, err := assembleSyncPrompt(SerenaSyncOpts{
		DiffSummary: "diff",
		MaxTurns:    5,
		ProjectRoot: "/proj",
	})
	if err != nil {
		t.Fatalf("assembleSyncPrompt: unexpected error: %v", err)
	}

	// Key instructions per AC#4
	instructions := []string{
		"list_memories",
		"read_memory",
		"edit_memory",
		"write_memory",
	}
	for _, instr := range instructions {
		if !strings.Contains(got, instr) {
			t.Errorf("output missing instruction keyword: %q", instr)
		}
	}

	// Constraints per AC#4: delete_memory must be in prohibition context
	if !strings.Contains(got, "ЗАПРЕЩЕНО удалять") {
		t.Error("output should contain prohibition of deleting memories")
	}
	if !strings.Contains(got, "delete_memory") {
		t.Error("output should mention delete_memory in constraints")
	}
}

// --- Story 9.3: Progressive review prompt tests ---

// TestPrompt_Review_IncrementalMode verifies incremental diff section appears when IncrementalDiff=true (AC#2).
func TestPrompt_Review_IncrementalMode(t *testing.T) {
	prevFindings := "### [HIGH] Missing error handling\n"
	data := config.TemplateData{
		IncrementalDiff:  true,
		Cycle:            3,
		MinSeverityLabel: "MEDIUM",
		MaxFindings:      3,
	}
	replacements := reviewReplacements()
	replacements["__TASK_CONTENT__"] = "Fix authentication bug"
	replacements["__PREV_FINDINGS__"] = prevFindings

	got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name    string
		substr  string
		present bool
	}{
		// AC#2: incremental diff instruction
		{"incremental diff instruction", "git diff HEAD~1..HEAD", true},
		{"incremental review header", "Incremental Review (Cycle 3)", true},
		{"previous findings injected", "Missing error handling", true},
		{"severity threshold instruction", "уровня MEDIUM+", true},
		// AC#7: findings budget instruction
		{"budget instruction", "НЕ БОЛЕЕ 3", true},
		{"prioritize instruction", "Приоритизируй по severity", true},
		// Placeholders replaced
		{"no prev findings placeholder", "__PREV_FINDINGS__", false},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			found := strings.Contains(got, c.substr)
			if c.present && !found {
				t.Errorf("expected prompt to contain %q, but it does not", c.substr)
			}
			if !c.present && found {
				t.Errorf("expected prompt NOT to contain %q, but it does", c.substr)
			}
		})
	}
}

// TestPrompt_Review_FullDiffMode verifies incremental section absent when IncrementalDiff=false (AC#3).
func TestPrompt_Review_FullDiffMode(t *testing.T) {
	data := config.TemplateData{
		IncrementalDiff: false,
	}
	replacements := reviewReplacements()
	replacements["__TASK_CONTENT__"] = "Implement feature X"

	got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	checks := []struct {
		name   string
		substr string
	}{
		{"no incremental header", "Incremental Review"},
		{"no git diff instruction", "git diff HEAD~1..HEAD"},
		{"no prev findings placeholder", "__PREV_FINDINGS__"},
		{"no budget instruction", "НЕ БОЛЕЕ"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if strings.Contains(got, c.substr) {
				t.Errorf("full diff mode should NOT contain %q", c.substr)
			}
		})
	}
}

// TestPrompt_Review_BudgetInstruction verifies different budget values render correctly (AC#7).
func TestPrompt_Review_BudgetInstruction(t *testing.T) {
	cases := []struct {
		name        string
		maxFindings int
		wantBudget  string
	}{
		{"budget 1", 1, "НЕ БОЛЕЕ 1"},
		{"budget 5", 5, "НЕ БОЛЕЕ 5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := config.TemplateData{
				IncrementalDiff:  true,
				Cycle:            4,
				MinSeverityLabel: "HIGH",
				MaxFindings:      tc.maxFindings,
			}
			replacements := reviewReplacements()
			replacements["__TASK_CONTENT__"] = "Fix bug"
			replacements["__PREV_FINDINGS__"] = "prev"

			got, err := config.AssemblePrompt(reviewTemplate, data, replacements)
			if err != nil {
				t.Fatalf("AssemblePrompt error: %v", err)
			}
			if count := strings.Count(got, tc.wantBudget); count != 1 {
				t.Errorf("expected prompt to contain %q exactly once, got %d times", tc.wantBudget, count)
			}
		})
	}
}

// --- Story 9.6: Scope Creep Protection Prompts ---

// TestPrompt_Execute_ScopeBoundarySection verifies execute.md contains
// SCOPE BOUNDARY section with task text and scope creep prevention (AC#1, AC#2, AC#5).
func TestPrompt_Execute_ScopeBoundarySection(t *testing.T) {
	data := config.TemplateData{}
	replacements := executeReplacements()
	replacements["__TASK_CONTENT__"] = "Implement login feature"

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// AC#1: section header present
	if !strings.Contains(got, "## SCOPE BOUNDARY (MANDATORY)") {
		t.Error("execute prompt missing '## SCOPE BOUNDARY (MANDATORY)' section")
	}
	// AC#1: contains task-specific instruction
	if !strings.Contains(got, "Реализуй ТОЛЬКО текущую задачу: Implement login feature") {
		t.Error("scope boundary missing task-specific instruction with actual task text")
	}
	// AC#1: contains prohibition
	if !strings.Contains(got, "НЕ реализуй другие задачи из sprint-tasks.md") {
		t.Error("scope boundary missing prohibition instruction")
	}
	// AC#1: contains pre-commit check
	if !strings.Contains(got, "Перед коммитом проверь") {
		t.Error("scope boundary missing pre-commit check instruction")
	}
	// AC#1: contains rollback instruction (section-scoped phrase)
	if !strings.Contains(got, "откати их через git checkout") {
		t.Error("scope boundary missing git checkout rollback instruction")
	}
	// AC#5: uniqueness — SCOPE BOUNDARY appears exactly once
	if count := strings.Count(got, "SCOPE BOUNDARY"); count != 1 {
		t.Errorf("SCOPE BOUNDARY should appear once, got %d", count)
	}
}

// TestPrompt_Implementation_ScopeCompliance verifies implementation agent
// contains scope creep check instructions (AC#3).
func TestPrompt_Implementation_ScopeCompliance(t *testing.T) {
	// AC#3: contains scope compliance instruction
	if !strings.Contains(agentImplementationPrompt, "Verify ALL changes in the diff relate to the current task") {
		t.Error("implementation agent missing scope compliance check instruction")
	}
	// AC#3: scope creep as HIGH severity
	if !strings.Contains(agentImplementationPrompt, "Scope creep") {
		t.Error("implementation agent missing 'Scope creep' finding format")
	}
	if !strings.Contains(agentImplementationPrompt, "Severity: HIGH") {
		t.Error("implementation agent missing HIGH severity for scope creep")
	}
	// AC#3: finding format — verify full template
	if !strings.Contains(agentImplementationPrompt, "Scope creep: изменения в") {
		t.Error("implementation agent missing scope creep finding format prefix")
	}
	if !strings.Contains(agentImplementationPrompt, "реализуют задачу") {
		t.Error("implementation agent missing 'реализуют задачу' in finding format")
	}
	if !strings.Contains(agentImplementationPrompt, "а не текущую") {
		t.Error("implementation agent missing 'а не текущую' in finding format")
	}
}

// TestPrompt_OtherAgents_NoScopeCreep verifies that quality, simplification,
// design-principles, and test-coverage agents do NOT contain scope creep
// check instructions (AC#4).
func TestPrompt_OtherAgents_NoScopeCreep(t *testing.T) {
	agents := []struct {
		name    string
		content string
	}{
		{"quality", agentQualityPrompt},
		{"simplification", agentSimplificationPrompt},
		{"design-principles", agentDesignPrinciplesPrompt},
		{"test-coverage", agentTestCoveragePrompt},
	}
	for _, agent := range agents {
		t.Run(agent.name, func(t *testing.T) {
			if strings.Contains(agent.content, "Scope creep") {
				t.Errorf("%s agent should NOT contain 'Scope creep' instructions", agent.name)
			}
			if strings.Contains(agent.content, "scope creep") {
				t.Errorf("%s agent should NOT contain 'scope creep' instructions", agent.name)
			}
		})
	}
}

// --- Story 9.9: Agent Stats in Review Findings ---

// TestPrompt_SubAgents_AgentField verifies all 5 sub-agent prompts contain correct agent name (AC#3).
func TestPrompt_SubAgents_AgentField(t *testing.T) {
	t.Parallel()
	agents := []struct {
		name   string
		prompt string
		want   string
	}{
		{"quality", agentQualityPrompt, "- **Агент**: quality"},
		{"implementation", agentImplementationPrompt, "- **Агент**: implementation"},
		{"simplification", agentSimplificationPrompt, "- **Агент**: simplification"},
		{"design-principles", agentDesignPrinciplesPrompt, "- **Агент**: design-principles"},
		{"test-coverage", agentTestCoveragePrompt, "- **Агент**: test-coverage"},
	}
	for _, a := range agents {
		t.Run(a.name, func(t *testing.T) {
			if !strings.Contains(a.prompt, a.want) {
				t.Errorf("%s agent missing %q", a.name, a.want)
			}
		})
	}
}

// TestPrompt_Review_AgentFieldInFormat verifies review.md includes Agent field in findings format (AC#2).
func TestPrompt_Review_AgentFieldInFormat(t *testing.T) {
	t.Parallel()
	if !strings.Contains(reviewTemplate, "**Агент**") {
		t.Error("review.md missing **Агент** field in findings format")
	}
	if !strings.Contains(reviewTemplate, "<agent_name>") {
		t.Error("review.md missing <agent_name> placeholder")
	}
	if !strings.Contains(reviewTemplate, "5 fields") {
		t.Error("review.md should mention 5 fields (was 4)")
	}
}

// --- Story 9.7: Pre-flight Check + TaskHash + LogOneline ---

// TestPrompt_Execute_TaskHashMarker verifies execute.md contains [task:__TASK_HASH__]
// instruction in Commit Rules and __TASK_HASH__ is replaced (AC#8).
func TestPrompt_Execute_TaskHashMarker(t *testing.T) {
	data := config.TemplateData{}
	replacements := executeReplacements()
	replacements["__TASK_HASH__"] = "a1b2c3"

	got, err := config.AssemblePrompt(executeTemplate, data, replacements)
	if err != nil {
		t.Fatalf("AssemblePrompt error: %v", err)
	}

	// AC#8: commit rules contain task marker instruction
	if !strings.Contains(got, "[task:a1b2c3]") {
		t.Error("execute prompt missing [task:<hash>] marker after replacement")
	}
	// AC#8: instruction text present
	if !strings.Contains(got, "В конце commit message добавь маркер") {
		t.Error("execute prompt missing task marker instruction text")
	}
	// AC#8: example present
	if !strings.Contains(got, "feat: add user validation [task:a1b2c3]") {
		t.Error("execute prompt missing task marker example")
	}
	// Verify no unreplaced __TASK_HASH__ remains
	if strings.Contains(got, "__TASK_HASH__") {
		t.Error("execute prompt still contains unreplaced __TASK_HASH__")
	}
}
