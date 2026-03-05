# Build & CI

## Makefile Targets
- `make build` → `go build -o ralph ./cmd/ralph`
- `make test` → `go test ./...`
- `make lint` → `golangci-lint run`
- `make clean` → `rm -f ralph`

## CI: GitHub Actions (.github/workflows/ci.yml)
- Go 1.25 matrix
- golangci-lint v2 (7 linters) — NOT available in WSL, only CI
- Runs on push/PR

## Release: goreleaser v2 (.goreleaser.yaml)
- Platforms: linux/darwin, amd64/arm64
- CGO_ENABLED=0
- Binary name: ralph

## Go Module
- Module: `github.com/bmad-ralph/bmad-ralph`
- go.mod: `go 1.25`
- Only 3 direct deps: cobra, yaml.v3, fatih/color
- New deps require justification

## WSL Build Notes
- Build command: `"/mnt/c/Program Files/Go/bin/go.exe" build -o ralph ./cmd/ralph`
- Test command: `"/mnt/c/Program Files/Go/bin/go.exe" test ./...`
- Produces Windows binary (ralph.exe) by default
