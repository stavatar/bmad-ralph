# Task Completion Checklist

After completing any code change:

1. **Format**: `"/mnt/c/Program Files/Go/bin/go.exe" fmt ./...`
2. **Vet**: `"/mnt/c/Program Files/Go/bin/go.exe" vet ./...`
3. **Test**: `"/mnt/c/Program Files/Go/bin/go.exe" test ./...`
4. **Line endings**: verify with `file <changed-files>` — no CRLF
5. **Doc comments**: re-read ALL doc comments on modified functions, verify each claim
6. **Error wrapping**: grep function for ALL `return err` lines, verify each wraps
7. **Return values**: no uncaptured return values (`_ =` with comment if intentional)

After code review:
1. Update `.claude/rules/<topic>.md` with new patterns
2. Update `.claude/rules/wsl-ntfs.md` if WSL-specific patterns found
3. Update `memory/MEMORY.md` with project status
4. Update `.claude/violation-tracker.md` with violation counts
