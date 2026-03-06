# Story 9.9: Agent Stats in Review Findings

Status: review

## Story

As a разработчик,
I want знать какие review sub-agents генерируют замечания,
so that можно анализировать эффективность review pipeline и настраивать агентов.

## Acceptance Criteria

1. **ReviewFinding.Agent field (FR77):**
   - `ReviewFinding` struct gets `Agent string` field with json tag `"agent,omitempty"`
   - Existing findings without Agent field -> Agent="" (backward compatible)

2. **Agent field в review prompt (FR77):**
   - `runner/prompts/review.md` findings format includes 5th field:
     ```
     ### [SEVERITY] Finding title
     - **Описание**: ...
     - **Файл**: ...
     - **Строка**: ...
     - **Агент**: <agent_name>
     ```

3. **Sub-agent prompts include agent name (FR77):**
   - quality.md -> "- **Агент**: quality"
   - implementation.md -> "- **Агент**: implementation"
   - simplification.md -> "- **Агент**: simplification"
   - design-principles.md -> "- **Агент**: design-principles"
   - test-coverage.md -> "- **Агент**: test-coverage"

4. **DetermineReviewOutcome parses Agent field (FR78):**
   - Parses `- **Агент**: implementation` from review-findings.md
   - Sets `ReviewFinding.Agent == "implementation"`

5. **DetermineReviewOutcome без Agent field (FR78):**
   - Old format without Agent field -> `ReviewFinding.Agent == ""` (empty, not "unknown")

6. **AgentFindingStats type (FR78):**
   - `runner/metrics.go` defines `AgentFindingStats` struct with fields: Critical, High, Medium, Low (all int, json tagged)

7. **RunMetrics.AgentStats (FR78):**
   - `RunMetrics` gets `AgentStats map[string]*AgentFindingStats` field (json key "agent_stats", omitempty)
   - Aggregated across all tasks and cycles in the run

8. **RecordAgentFinding accumulation (FR78):**
   - `RecordAgentFinding("implementation", "HIGH")` called 3 times -> AgentStats["implementation"] = {High: 3}
   - `RecordAgentFinding("quality", "MEDIUM")` called 2 times -> AgentStats["quality"] = {Medium: 2}

9. **Unknown agent handling (FR78):**
   - Finding without Agent field (Agent="") -> counted under AgentStats["unknown"]

10. **Agent regex parsing (FR78):**
    - `findingAgentRe` matches `- **Агент**: implementation` -> captures "implementation"
    - Matches `- **Агент**: design-principles` -> captures "design-principles"

## Tasks / Subtasks

- [x] Task 1: Add Agent field to ReviewFinding (AC: #1)
  - [ ] In `runner/metrics.go`: add `Agent string` with json tag
- [x] Task 2: Define AgentFindingStats struct and RunMetrics.AgentStats (AC: #6, #7)
  - [ ] In `runner/metrics.go`: AgentFindingStats with Critical/High/Medium/Low int fields
  - [ ] Add AgentStats map to RunMetrics
- [x] Task 3: Implement RecordAgentFinding on MetricsCollector (AC: #8, #9)
  - [ ] Accumulate per-agent severity counts
  - [ ] Empty agent -> "unknown" key
  - [ ] Nil receiver no-op pattern
- [x] Task 4: Add findingAgentRe regex and parse Agent in DetermineReviewOutcome (AC: #4, #5, #10)
  - [ ] Define `findingAgentRe = regexp.MustCompile(...)` at package scope
  - [ ] After finding severity header, search following lines for Agent field
  - [ ] Set Agent="" when field absent (backward compatible)
- [x] Task 5: Update review.md prompt with Agent field in findings format (AC: #2)
  - [ ] Add 5th field `- **Агент**: <agent_name>` to findings format template
- [x] Task 6: Update all 5 sub-agent prompts (AC: #3)
  - [ ] quality.md: add `- **Агент**: quality` to finding format
  - [ ] implementation.md: add `- **Агент**: implementation`
  - [ ] simplification.md: add `- **Агент**: simplification`
  - [ ] design-principles.md: add `- **Агент**: design-principles`
  - [ ] test-coverage.md: add `- **Агент**: test-coverage`
- [x] Task 7: Call RecordAgentFinding from Execute() (AC: #8)
  - [ ] After parsing findings, iterate and call RecordAgentFinding for each
- [x] Task 8: Wire AgentStats into Finish() aggregation (AC: #7)
  - [ ] Accumulate agent stats from collector into RunMetrics
- [x] Task 9: Write comprehensive tests (AC: #1-#10)
  - [ ] Test ReviewFinding JSON serialization with Agent field
  - [ ] Test DetermineReviewOutcome parses Agent field
  - [ ] Test DetermineReviewOutcome backward compat (no Agent field)
  - [ ] Test findingAgentRe regex patterns
  - [ ] Test RecordAgentFinding accumulation
  - [ ] Test unknown agent handling
  - [ ] Test AgentStats in Finish() output
  - [ ] Test sub-agent prompts contain correct agent names

## Dev Notes

### Architecture & Design

- **Primary files:** `runner/metrics.go`, `runner/runner.go`
- **Prompt files:** `runner/prompts/review.md`, `runner/prompts/agents/*.md` (all 5)
- **Dependency:** Story 9.1 (ParseSeverity for severity categorization in RecordAgentFinding)
- **No new packages or dependencies**

### Critical Implementation Details

**findingAgentRe regex:**
```go
var findingAgentRe = regexp.MustCompile(`(?m)^\s*-\s*\*\*Агент\*\*:\s*(\S+)`)
```

**Parsing logic in DetermineReviewOutcome:**
Current code at `runner/runner.go:291-295`:
```go
matches := findingSeverityRe.FindAllStringSubmatch(string(findingsData), -1)
for _, m := range matches {
    findings = append(findings, ReviewFinding{
        Severity:    m[1],
        Description: strings.TrimSpace(m[2]),
    })
}
```

New approach: parse each finding block (from `### [SEVERITY]` to next `###`), extract agent from within the block:
```go
// For each finding, search text between current ### and next ### for agent field
agentMatch := findingAgentRe.FindStringSubmatch(findingBlock)
if agentMatch != nil {
    finding.Agent = agentMatch[1]
}
```

**RecordAgentFinding:**
```go
func (mc *MetricsCollector) RecordAgentFinding(agent, severity string) {
    if mc == nil { return }
    if agent == "" { agent = "unknown" }
    if mc.agentStats == nil {
        mc.agentStats = make(map[string]*AgentFindingStats)
    }
    stats := mc.agentStats[agent]
    if stats == nil {
        stats = &AgentFindingStats{}
        mc.agentStats[agent] = stats
    }
    switch strings.ToUpper(severity) {
    case "CRITICAL": stats.Critical++
    case "HIGH": stats.High++
    case "MEDIUM": stats.Medium++
    case "LOW": stats.Low++
    }
}
```

### Existing Scaffold Context

- `runner/metrics.go:20-25` — current ReviewFinding (4 fields: Severity, Description, File, Line)
- `runner/metrics.go:87-104` — current RunMetrics struct
- `runner/metrics.go:126+` — MetricsCollector struct
- `runner/runner.go:65-66` — `findingSeverityRe` regex
- `runner/runner.go:291-295` — current finding parsing in DetermineReviewOutcome
- `runner/prompts/agents/` — 5 sub-agent prompt files

### Testing Standards

- Table-driven tests for regex patterns
- Verify JSON serialization of ReviewFinding with Agent field (omitempty behavior)
- Test backward compatibility: findings without Agent field
- Test RecordAgentFinding with nil receiver (no-op)
- Test Finish() aggregates AgentStats correctly
- Test all 5 agent prompts contain correct "- **Агент**: <name>"

### References

- [Source: docs/epics/epic-9-ralph-run-robustness-stories.md#Story 9.9]
- [Source: docs/architecture/ralph-run-robustness.md#Область 5]
- [Source: docs/prd/ralph-run-robustness.md#FR77, FR78]
- [Source: runner/metrics.go:20-25 — ReviewFinding struct]
- [Source: runner/metrics.go:87-104 — RunMetrics struct]
- [Source: runner/runner.go:65-66 — findingSeverityRe regex]
- [Source: runner/runner.go:291-295 — finding parsing in DetermineReviewOutcome]
- [Source: runner/prompts/agents/*.md — 5 sub-agent prompt files]

## Dev Agent Record

### Context Reference

### Agent Model Used

### Debug Log References

### Completion Notes List

- Agent field added to ReviewFinding with `json:"agent,omitempty"` (backward compatible)
- AgentFindingStats struct with Critical/High/Medium/Low int fields, JSON tagged
- RecordAgentFinding with nil receiver no-op, empty agent → "unknown", case-insensitive severity
- findingAgentRe regex parses `- **Агент**: <name>` from finding blocks
- DetermineReviewOutcome splits findings into blocks between ### headers, extracts agent per block
- review.md updated: 5 fields (was 4), added `- **Агент**: <agent_name>`
- All 5 sub-agent prompts updated with correct agent name instruction
- Execute() calls RecordAgentFinding for each finding after RecordReview
- Finish() passes agentStats map directly to RunMetrics.AgentStats
- 15 new tests: JSON serialization, accumulation, nil receiver, case insensitivity, empty agent, Finish aggregation, regex patterns, agent parsing, backward compat, prompt assertions
- Updated existing test assertion "4 fields" → "5 fields" in prompt_test.go
- Updated 3 golden files (review prompt + agent prompts changed)

### File List

- runner/metrics.go — Agent field on ReviewFinding, AgentFindingStats struct, AgentStats on RunMetrics, agentStats on MetricsCollector, RecordAgentFinding method, Finish() wiring
- runner/runner.go — findingAgentRe regex, DetermineReviewOutcome block-based agent parsing, Execute() RecordAgentFinding calls, doc comment update
- runner/prompts/review.md — 5th field Агент in findings format, Finding Structure updated to 5 fields
- runner/prompts/agents/quality.md — added `- **Агент**: quality` instruction
- runner/prompts/agents/implementation.md — added `- **Агент**: implementation` instruction
- runner/prompts/agents/simplification.md — added `- **Агент**: simplification` instruction
- runner/prompts/agents/design-principles.md — added `- **Агент**: design-principles` instruction
- runner/prompts/agents/test-coverage.md — added `- **Агент**: test-coverage` instruction
- runner/metrics_test.go — 9 new tests (JSON, accumulation, nil, case, empty, Finish x2)
- runner/runner_test.go — 2 new tests (AgentParsing, AgentBackwardCompat)
- runner/coverage_internal_test.go — 1 new test (FindingAgentRe_Patterns, 8 table cases)
- runner/prompt_test.go — 2 new tests (SubAgents_AgentField, Review_AgentFieldInFormat), 1 updated assertion
- runner/testdata/review-findings-with-agent.md — new fixture with agent fields
- runner/testdata/TestPrompt_Agent_Golden/* — updated golden files
- runner/testdata/TestPrompt_Review_KnowledgeSections.golden — updated golden
- docs/sprint-artifacts/9-9-agent-stats-review-findings.md — status + tasks
- docs/sprint-artifacts/sprint-status.yaml — status update
