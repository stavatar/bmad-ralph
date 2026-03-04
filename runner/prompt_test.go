package runner

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmad-ralph/bmad-ralph/config"
)

var update = flag.Bool("update", false, "update golden files")

// executeReplacements returns a base replacements map with all execute template
// placeholders set to empty. Callers override specific keys as needed.
func executeReplacements() map[string]string {
	return map[string]string{
		"__FORMAT_CONTRACT__":  "",
		"__RALPH_KNOWLEDGE__":  "",
		"__LEARNINGS_CONTENT__": "",
		"__FINDINGS_CONTENT__": "",
		"__SERENA_HINT__":      "",
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
		{"commit on green only", "Commit ONLY when ALL tests pass", true},
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
		{"commit on green only", "Commit ONLY when ALL tests pass", true},
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
		{"gates pause instruction", "pause execution and report status", true},
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
		{"finding fields mandatory", "All 4 fields are mandatory", true},
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
		{"implementation", agentImplementationPrompt, []string{"acceptance criteria", "satisfies"}},
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
