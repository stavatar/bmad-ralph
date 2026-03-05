# WSL/NTFS Environment Setup

## Go Binary
- Project uses Windows Go 1.26.0 for building: `"/mnt/c/Program Files/Go/bin/go.exe"`
- For Serena/gopls: native Linux Go 1.25 installed at `~/.local/go/`
- Symlink: `~/.local/bin/go` → `~/.local/go/bin/go` (native ELF)
- Native gopls 0.21.1: `~/.local/bin/gopls` (ELF binary)
- `~/.local/bin` is already in PATH

## CRLF Handling
- Write/Edit tools on NTFS create CRLF
- Auto-fixed by PostToolUse hook: `.claude/hooks/fix-crlf.sh`
- `.gitattributes` enforces LF on git add
- Verify: `file <filename>` — must say "ASCII text" not "with CRLF"

## Serena MCP Server
- Config: `.mcp.json` in project root
- Launched via `uvx` from `git+https://github.com/oraios/serena`
- `env.GOROOT` set to `/home/stepan/.local/go` in `.mcp.json`
- Dashboard: http://127.0.0.1:24282/dashboard/

## UserHomeDir in Tests
- Windows Go `os.UserHomeDir()` uses USERPROFILE not HOME
- Tests: `t.Setenv` both HOME and USERPROFILE
- Failure tests: also clear HOMEDRIVE/HOMEPATH

## Known Limitations
- golangci-lint not available in WSL — only CI catches lint
- Broken symlink + os.Stat may not work on WSL/NTFS
- os.MkdirAll on nonexistent root paths succeeds on WSL
