#!/bin/bash
# verify-knowledge-extraction.sh
# Called at the end of code-review workflow Step 6 to verify
# that knowledge extraction was actually performed.
# Exit 0 always (informational), output status for the agent to act on.

set -euo pipefail

rules_modified=0
tracker_modified=0

# Check unstaged + staged + untracked changes to rules files
if git diff --name-only -- '.claude/rules/' 2>/dev/null | grep -q .; then
    rules_modified=1
fi
if git diff --cached --name-only -- '.claude/rules/' 2>/dev/null | grep -q .; then
    rules_modified=1
fi
if git ls-files --others --exclude-standard -- '.claude/rules/' 2>/dev/null | grep -q .; then
    rules_modified=1
fi

# Check violation tracker (modified, staged, or new untracked)
if git diff --name-only -- '.claude/violation-tracker.md' 2>/dev/null | grep -q .; then
    tracker_modified=1
fi
if git diff --cached --name-only -- '.claude/violation-tracker.md' 2>/dev/null | grep -q .; then
    tracker_modified=1
fi
if git ls-files --others --exclude-standard -- '.claude/violation-tracker.md' 2>/dev/null | grep -q .; then
    tracker_modified=1
fi

echo "=== Knowledge Extraction Verification ==="

if [ "$rules_modified" -eq 1 ]; then
    echo "OK: .claude/rules/*.md — files modified"
    git diff --name-only -- '.claude/rules/' 2>/dev/null | sed 's/^/  /'
    git diff --cached --name-only -- '.claude/rules/' 2>/dev/null | sed 's/^/  (staged) /'
else
    echo "WARN: .claude/rules/*.md — NO files modified"
    echo "  If all findings match existing patterns, state which ones explicitly."
    echo "  Otherwise, add new patterns to the appropriate topic file."
fi

echo ""

if [ "$tracker_modified" -eq 1 ]; then
    echo "OK: .claude/violation-tracker.md — updated"
else
    echo "WARN: .claude/violation-tracker.md — NOT updated"
    echo "  Violation counts must be updated after every code review."
fi

echo ""

if [ "$rules_modified" -eq 1 ] && [ "$tracker_modified" -eq 1 ]; then
    echo "RESULT: Knowledge extraction COMPLETE"
else
    echo "RESULT: Knowledge extraction INCOMPLETE — address warnings above"
fi
