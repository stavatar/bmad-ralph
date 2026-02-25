.PHONY: build test lint clean

build:
	go build -o ralph ./cmd/ralph

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f ralph
