#!/bin/bash
# PostToolUse hook: auto-fix CRLF after Write/Edit on NTFS
# Reads tool_input.file_path from stdin JSON via python3

FILE_PATH=$(python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get('tool_input', {}).get('file_path', ''))
except:
    pass
")

if [ -n "$FILE_PATH" ] && [ -f "$FILE_PATH" ]; then
  # Only fix text files (skip binaries)
  if file "$FILE_PATH" | grep -q "text"; then
    sed -i 's/\r$//' "$FILE_PATH"
  fi
fi

exit 0
