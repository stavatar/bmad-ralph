---
globs: ["**/*.go", "Makefile", "*.sh"]
---

# WSL/NTFS Patterns — bmad-ralph

## Go Binary

- Full path required: `"/mnt/c/Program Files/Go/bin/go.exe"` (Windows Go, NOT in WSL PATH)
- `go.exe` cannot execute bash scripts: use Go test binary self-reexec pattern (TestMain + env var + `os.Args[0]`)

## File System

- Write/Edit tools on NTFS create CRLF: always `sed -i 's/\r$//'` after every Write
- `.gitattributes` enforces LF on git add, disk files remain CRLF until converted
- `os.MkdirAll` on nonexistent root paths succeeds on WSL — use file-as-directory trick for failure tests
- `os.SameFile()` for path comparison — Windows 8.3 short names differ from long names

## UserHomeDir

- Windows Go `os.UserHomeDir()` uses `USERPROFILE` not `HOME`
- Tests: `t.Setenv` both `HOME` and `USERPROFILE`
- For failure tests: also clear `HOMEDRIVE`/`HOMEPATH`

## Test Platform Issues

- Broken symlink + `os.Stat` may not work on WSL/NTFS: use `t.Skipf` not `t.Logf`, document as coverage gap
- golangci-lint not in WSL — only CI catches lint issues
- Tests that always skip/fallthrough are not coverage — document clearly in function comment
