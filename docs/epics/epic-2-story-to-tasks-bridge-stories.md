# Epic 2: Story-to-Tasks Bridge — Stories

**Epic Goal:** `ralph bridge stories/auth.md` генерирует полноценный sprint-tasks.md с задачами, AC-derived тестами, `[GATE]` разметкой, служебными задачами, и `source:` трассировкой.

**PRD Coverage:** FR1, FR2, FR3, FR4, FR5, FR5a
**Stories:** 7 | **Estimated AC:** ~40

---

### Story 2.1: Shared Sprint-Tasks Format Contract

**As a** developer,
**I want** the sprint-tasks.md format defined once as a shared contract,
**So that** bridge output and runner parsing use identical format expectations.

**Acceptance Criteria:**

```gherkin
Given the format contract needs to be shared between bridge and runner
When config/shared/sprint-tasks-format.md is created
Then it defines:
  - Task syntax: "- [ ] Task description [GATE]"
  - Done syntax: "- [x] Task description"
  - Source field: "  source: stories/file.md#AC-N"
  - Feedback syntax: "> USER FEEDBACK: text"
  - Section headers: "## Epic Name" grouping
  - Service task prefix: "[SETUP]", "[VERIFY]", "[E2E]"
  - Source field exact syntax: "  source: <relative-path>#<AC-identifier>"
    Regex: `^\s+source:\s+\S+#\S+`
    Examples: "  source: stories/auth.md#AC-3", "  source: stories/api.md#AC-1"

And the file is embedded via go:embed in config package
And config.SprintTasksFormat() returns the embedded content as string
And bridge imports format for inclusion in bridge prompt
And runner uses same constants (TaskOpen, TaskDone from Story 1.6) to parse

And tests in BOTH bridge and config packages verify:
  - Format content is non-empty
  - Format contains key markers (TaskOpen, TaskDone, GateTag)
  - Structural Rule #8: shared contract tested from both sides
```

**Technical Notes:**
- Architecture: "config/shared/sprint-tasks-format.md — единый source of truth через go:embed shared file"
- Architecture: "Формат определяется один раз. Включается и в bridge prompt, и в execute prompt"
- Structural Rule #8: "Shared contracts = отдельная story с tests в обоих packages"
- This is the hub node identified in Graph of Thoughts (5 writers/readers)

**Prerequisites:** Story 1.1, Story 1.6 (constants)

---

### Story 2.2: Bridge Prompt Template

**As a** developer,
**I want** the bridge prompt template created as a text/template .md file,
**So that** Claude receives clear instructions for converting stories to sprint-tasks.md. (FR1, FR2, FR3, FR5, FR5a)

**Acceptance Criteria:**

```gherkin
Given the bridge prompt template is in bridge/prompts/bridge.md
When the template is assembled via AssemblePrompt (Epic 1 Story 1.10)
Then the prompt includes:
  - Sprint-tasks format contract (from Story 2.1)
  - Instructions to convert story → tasks
  - Instructions to derive test cases from objective AC (FR2)
  - Instructions to mark [GATE] on epic first tasks and milestones (FR3)
  - Instructions to generate service tasks: [SETUP], [VERIFY], [E2E] (FR5)
  - Instructions to add source: field on every task (FR5a)
  - Instructions to check test framework presence (Architecture: bridge checks)
  - Red-green principle reminder for test derivation

And the template uses text/template syntax:
  {{.StoryContent}} — placeholder for story file content (Stage 2 replace)
  {{if .HasExistingTasks}} — conditional for smart merge mode

And prompt includes negative examples (prohibited formats):
  "DO NOT use numbered lists. DO NOT add markdown headers outside
   the defined structure. Every task MUST start with exactly '- [ ]'"

And golden file snapshot test exists:
  bridge/testdata/TestPrompt_Bridge.golden
And test verifies assembled prompt matches golden file

Note: This story creates the BASIC prompt structure. Story 2.4 enriches
with detailed FR-specific instructions and golden file verification.
Dev agent: do NOT implement full FR3/FR5/FR5a details here — only skeleton.
```

**Technical Notes:**
- Architecture: "Bridge prompt template в bridge/prompts/bridge.md (text/template)"
- Architecture: "Bridge: проверка test framework"
- Prompt assembly from Epic 1 Story 1.10 (AssemblePrompt)
- go:embed in bridge package: `//go:embed prompts/bridge.md`
- Anti-pattern from guidelines: "Golden file = достаточно для промптов" → add scenario test

**Prerequisites:** Story 1.10 (prompt assembly), Story 2.1 (format contract)

---

### Story 2.3: Bridge Logic — Core Conversion

**As a** developer,
**I want** `bridge.Run(ctx, cfg)` to read story files and produce sprint-tasks.md,
**So that** the bridge command converts stories into actionable task lists. (FR1)

**Acceptance Criteria:**

```gherkin
Given one or more story file paths are provided as Cobra positional args
  (validated in cmd/ralph/bridge.go, passed to bridge.Run)
When bridge.Run(ctx, cfg, storyPaths []string) is called
Then each story file is read from disk
And story content is injected into bridge prompt via AssemblePrompt
And session.Execute is called with assembled prompt
And Claude output is written to sprint-tasks.md at config.ProjectRoot

Given Claude returns well-formed sprint-tasks.md content
When output is written
Then file is created/overwritten at sprint-tasks.md
And file is UTF-8 encoded with \n line endings

Given story file does not exist
When bridge.Run is called
Then descriptive error returned: "bridge: read story: %w"

Given session.Execute fails
When error is returned
Then bridge wraps: "bridge: execute: %w"
And no partial sprint-tasks.md is written (atomic: write only on success)

And if assembled prompt > 1500 lines, log warning:
  "large prompt — consider splitting story" (Pre-mortem: context window)
Note: concurrent bridge invocations NOT supported (exclusive repo access)

And bridge.Run returns:
  | Field     | Type   | Description                 |
  | TaskCount | int    | Number of tasks generated   |
  | Error     | error  | nil on success              |

And unit tests verify:
  - Successful story → sprint-tasks.md flow (with mock session)
  - Missing story file error
  - Session failure error
  - Output file creation
```

**Technical Notes:**
- Architecture: "bridge.Run(ctx, cfg) — entry point"
- Architecture: "cmd/ralph/bridge.go — wiring only, логика в bridge/"
- MUST NOT modify sprint-tasks.md (Mutation Asymmetry — bridge creates, not mutates)
- Smart merge is separate story (2.6) — this is create-only mode

**Prerequisites:** Story 1.7 (session), Story 2.2 (bridge prompt)

---

### Story 2.4: Service Tasks, Gate Marking, Source Traceability

**As a** developer,
**I want** bridge prompt enhanced with service task generation, gate marking, and source traceability instructions,
**So that** sprint-tasks.md includes all supporting infrastructure. (FR3, FR5, FR5a)

**Acceptance Criteria:**

```gherkin
Given a story with dependencies on new frameworks
When bridge generates sprint-tasks.md
Then [SETUP] service tasks are generated BEFORE implementation tasks:
  e.g. "- [ ] [SETUP] Install and configure testing framework"
  source: stories/auth.md#setup

Given a story spans multiple parts of a feature
When bridge generates sprint-tasks.md
Then [VERIFY] integration verification tasks appear after related tasks:
  e.g. "- [ ] [VERIFY] Verify login API + frontend integration"

Given a story is the first in an epic
When bridge generates sprint-tasks.md
Then the first task of the epic has [GATE] tag:
  e.g. "- [ ] Implement user model [GATE]"

Given user-visible milestones exist in stories
When bridge generates sprint-tasks.md
Then milestone tasks have [GATE] tag

And EVERY task has source: field:
  "  source: stories/auth.md#AC-3" (FR5a)
And source field is indented under the task line

And golden file tests verify:
  - Story with dependencies → [SETUP] tasks present
  - Multi-part story → [VERIFY] tasks present
  - First epic task → [GATE] tag present
  - All tasks have source: field
```

**Technical Notes:**
- FR3: "[GATE] на первой задаче epic'а, user-visible milestones"
- FR5: "Служебные задачи: project setup, integration verification, e2e checkpoint"
- FR5a: "source: stories/file.md#AC-N — трассировка"
- These are prompt instructions, not bridge Go logic — Claude generates these based on prompt
- Scope split (Amelia): Story 2.2 = basic prompt skeleton. Story 2.4 = enriched FR-specific instructions + golden file verification of output quality

**Prerequisites:** Story 2.2, Story 2.3

---

### Story 2.5: Bridge Golden File Tests

**As a** developer,
**I want** comprehensive golden file tests for bridge output,
**So that** any regression in task generation is caught immediately.

**Acceptance Criteria:**

```gherkin
Given test story files exist in bridge/testdata/
When bridge is tested with mock Claude scenarios
Then golden files verify expected output:
  | Test Case                          | Input                    | Validates         |
  | TestBridge_SingleStory             | Simple 3-AC story        | Basic conversion  |
  | TestBridge_MultiStory              | 2 stories, 1 epic        | Multi-file merge  |
  | TestBridge_WithDependencies        | Story needing framework  | [SETUP] tasks     |
  | TestBridge_GateMarking             | First-of-epic story      | [GATE] placement  |
  | TestBridge_SourceTraceability      | Story with 5 ACs         | source: on all    |

And EVERY golden file output validated by:
  - config.TaskOpenRegex scan returns >0 tasks (parseable)
  - source format regex matches on every task line

And golden files are in bridge/testdata/TestName.golden
And tests use go test -update for golden file refresh
And mock Claude scenario returns deterministic output per test

And at least one test uses bridge golden file output
  as input for scanner (Story 2.1 cross-validation):
  bridge output → parsed by config.TaskOpen regex → valid tasks found
```

**Technical Notes:**
- Architecture: "Golden file тесты: input stories → expected sprint-tasks.md"
- Architecture: "bridge/testdata/TestBridge_SingleStory.golden"
- Reverse Engineering: "use bridge golden file output as input for runner"
- Mock Claude scenario files in `internal/testutil/scenarios/bridge_*.json`
- Note (Bob): 5 golden files = ~15 testdata files. May require 2 dev sessions if crafting is complex

**Prerequisites:** Story 2.3, Story 2.4 (**parallel-capable** with Story 2.6)

---

### Story 2.6: Smart Merge

**As a** developer,
**I want** bridge to merge new tasks into existing sprint-tasks.md without losing progress,
**So that** I can re-bridge after story updates without losing completed work. (FR4)

**Acceptance Criteria:**

```gherkin
Given sprint-tasks.md exists with some tasks marked [x] (completed)
And story files have been updated with new/changed requirements
When ralph bridge is run again
Then existing sprint-tasks.md is BACKED UP to sprint-tasks.md.bak
And bridge prompt includes existing sprint-tasks.md content (merge mode)
And Claude merges: new tasks added, completed [x] tasks preserved
And modified tasks updated but completion status preserved

Given the merge prompt is assembled
When {{if .HasExistingTasks}} evaluates to true
Then prompt includes existing sprint-tasks.md content
And explicit instruction: "MUST NOT change [x] status of completed tasks"
And explicit instruction: "MUST preserve source: fields of existing tasks"
And explicit instruction: "PRESERVE original task order. New tasks insert at logical position. NEVER reorder existing tasks"

Given merge fails (session error or output malformed)
When error occurs
Then backup file sprint-tasks.md.bak remains intact
And backup file content equals original sprint-tasks.md byte-for-byte (Murat)
And original sprint-tasks.md is NOT modified
And descriptive error returned

And golden file tests cover:
  - Merge with 2 completed + 3 new tasks
  - Merge where story changed an existing task description
  - Merge that adds [GATE] to previously un-gated task
  - Verify [x] status preserved in all cases
  - Verify existing task order unchanged after merge (Pre-mortem)

And Mandatory AC (Risk Heatmap):
  - Backup sprint-tasks.md before merge
  - Golden file tests for merge scenarios
```

**Technical Notes:**
- Architecture: "Smart Merge — критический промпт (не должен сбросить [x])"
- Risk Heatmap: "Backup sprint-tasks.md перед merge; golden file тесты merge-сценариев"
- Mandatory AC (FR4 Smart Merge): backup + golden file verification
- Merge is prompt-driven (Claude does the merge), not Go code merge

**Prerequisites:** Story 2.3, Story 2.4 (**parallel-capable** with Story 2.5)

---

### Story 2.7: Bridge Integration Test

**As a** developer,
**I want** a full bridge integration test with mock Claude,
**So that** the complete bridge flow is validated end-to-end.

**Acceptance Criteria:**

```gherkin
Given a mock Claude scenario for bridge flow
And test story files in t.TempDir()
When bridge integration test runs
Then the full flow executes:
  1. Story files read from disk
  2. Bridge prompt assembled (with format contract)
  3. Mock Claude invoked with correct flags
  4. Output parsed and written to sprint-tasks.md
  5. Sprint-tasks.md contains expected tasks

And mock Claude validates received prompt contains:
  - Sprint-tasks format contract text
  - Story content
  - FR instructions (test cases, gates, service tasks, source)

And a second test covers smart merge flow:
  1. First bridge creates sprint-tasks.md
  2. Mock marks some tasks [x]
  3. Second bridge with updated story
  4. Merged output preserves [x] tasks
  5. Backup file exists

And scenario-based test validates prompt↔parser contract:
  mock Claude returns predefined bridge output → bridge parses correctly
  (Devil's Advocate guideline)

And CLI-level test via compiled binary (Murat):
  exec.Command("ralph", "bridge", "testdata/story.md") with mock Claude
  Validates full path: Cobra arg parsing → bridge.Run() → output
  Tests exit code mapping on success and failure

And test is in bridge/bridge_integration_test.go
And uses t.TempDir() for isolation
```

**Technical Notes:**
- Architecture: "Integration тесты через mock Claude"
- Devil's Advocate: "scenario-based integration tests для КАЖДОГО типа промпта"
- Uses mock Claude from Epic 1 Story 1.11
- Build tag: `//go:build integration`

**Prerequisites:** Story 2.5, Story 2.6, Story 1.11 (mock Claude)

---

### Epic 2 Summary

| Story | Title | FRs | Files | AC Count |
|:-----:|-------|:---:|:-----:|:--------:|
| 2.1 | Shared Format Contract | — | 2 | 4 |
| 2.2 | Bridge Prompt Template | FR1,FR2 | 2 | 5 |
| 2.3 | Bridge Logic | FR1 | 2 | 7 |
| 2.4 | Service Tasks + Gates + Source | FR3,FR5,FR5a | 1 | 5 |
| 2.5 | Bridge Golden File Tests | — | 5+ testdata | 4 |
| 2.6 | Smart Merge | FR4 | 2 | 7 |
| 2.7 | Bridge Integration Test | — | 1 | 5 |
| | **Total** | **FR1-FR5a** | | **~37** |

**FR Coverage:** FR1 (2.2, 2.3), FR2 (2.2), FR3 (2.4), FR4 (2.6), FR5 (2.4), FR5a (2.4)

**Architecture Sections Referenced:** Project Structure (bridge/), Bridge prompt, Shared contract, Golden files, Testing patterns

**Dependency Graph:**
```
1.6 ──→ 2.1 ──→ 2.2 ──→ 2.3 ──→ 2.4 ─┬→ 2.5 ─┬→ 2.7
1.10 ─────────→ 2.2       ↑            └→ 2.6 ─┘
1.7, 1.8 ─────────────────┘            1.11 ──→ 2.7
```
Note: 2.3 also depends on 1.7/1.8 (session.Execute). 2.5 и 2.6 parallel-capable (Bob)

---
