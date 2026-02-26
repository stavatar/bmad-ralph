#!/bin/bash
# PreToolUse hook: inject checklist before Edit/Write on Go files
# Returns additionalContext via JSON so Claude sees checklist before editing

FILE_PATH=$(python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get('tool_input', {}).get('file_path', ''))
except:
    pass
")

# Only show checklist for Go source files
if [[ "$FILE_PATH" == *.go ]]; then
  python3 -c "
import json
output = {
    'hookSpecificOutput': {
        'hookEventName': 'PreToolUse',
        'additionalContext': '''PRE-EDIT CHECKLIST — verify EACH before proceeding:
1. Doc comments still accurate after this change?
2. Error wrapping consistent (same fmt.Errorf prefix for ALL returns)?
3. No duplicate test cases (check existing table first)?
4. go fmt needed after this edit?
5. No scope beyond AC (YAGNI)?
6. Inner error assertions present for ALL table cases?'''
    }
}
print(json.dumps(output))
"
fi

exit 0
