# v0.1 Manual Smoke Test Checklist

Pre-release validation with real Claude CLI. Run after all automated tests pass.

## Prerequisites

- Real Claude CLI installed and authenticated
- Small test project with sprint-tasks.md containing 1-2 simple tasks

## Checklist

- [ ] **1. Planted bug detection**: Create a file with an obvious bug (e.g., unused variable, missing error check). Run ralph. Verify review sub-agent detects the bug and review-findings.md contains the finding.

- [ ] **2. False positive resistance**: Create a clean, correct implementation. Run ralph review. Verify review produces no false positives — task marked [x], review-findings.md absent or empty.

- [ ] **3. Findings structure**: When findings are produced, verify review-findings.md contains all 4 required fields per finding: severity, title, description, and location.

- [ ] **4. Clean review behavior**: After a clean review, verify: task is marked [x] in sprint-tasks.md, review-findings.md is deleted or empty, runner proceeds to next task.

- [ ] **5. Non-clean review behavior**: After a review with findings, verify: task is NOT marked [x], review-findings.md contains confirmed findings, runner launches another execute session to fix findings.
