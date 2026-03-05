# Suggested Commands

## Build & Run
```bash
# Build (must use full path to go.exe on this WSL system)
"/mnt/c/Program Files/Go/bin/go.exe" build -o ralph ./cmd/ralph

# Run
./ralph
```

## Testing
```bash
# Run all tests
"/mnt/c/Program Files/Go/bin/go.exe" test ./...

# Run tests for specific package
"/mnt/c/Program Files/Go/bin/go.exe" test ./runner/...
"/mnt/c/Program Files/Go/bin/go.exe" test ./config/...

# Run specific test
"/mnt/c/Program Files/Go/bin/go.exe" test ./runner/ -run TestRunner_Execute

# With coverage
"/mnt/c/Program Files/Go/bin/go.exe" test -coverprofile=cover.out ./...

# Update golden files
"/mnt/c/Program Files/Go/bin/go.exe" test ./... -update
```

## Formatting & Linting
```bash
# Format
"/mnt/c/Program Files/Go/bin/go.exe" fmt ./...

# Lint (only available in CI, not in WSL)
# golangci-lint run

# Vet
"/mnt/c/Program Files/Go/bin/go.exe" vet ./...
```

## Module Management
```bash
"/mnt/c/Program Files/Go/bin/go.exe" mod tidy
```

## Git (standard unix commands work in WSL)
```bash
git status
git diff
git log --oneline -10
git check-ignore -v <path>
```

## Verify Line Endings
```bash
file <filename>  # Must say "ASCII text" not "with CRLF line terminators"
```
