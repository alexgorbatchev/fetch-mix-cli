set dotenv-load := false

# fetch-mix-cli justfile

# Default recipe: list available recipes
default:
    @just --list

# Build the fetch-mix binary in bin/
build:
	mkdir -p bin
	go build -o bin/fetch-mix ./cmd/fetch-mix

# Install fetch-mix binary to GOPATH bin directory
install:
	go install ./cmd/fetch-mix

# Run in human-facing interactive mode
run *ARGS:
	go run ./cmd/fetch-mix {{ARGS}}

# Run in agent-facing token-conservative mode
run-ai *ARGS:
	AGENT=1 go run ./cmd/fetch-mix {{ARGS}}

# Run YouTube comment tracklist extraction pipeline
youtube *ARGS:
	go run ./cmd/fetch-mix youtube {{ARGS}}

# Run out-of-band progress socket demo
demo-progress *ARGS:
	./scripts/demo-progress-socket.sh {{ARGS}}

# Run all unit tests
test:
	go test -v ./...

# Format Go code
fmt:
	go fmt ./...

# Run static code analysis
vet:
	go vet ./...

# Run linters and code formatting checks
lint: fmt vet

# Run all checks (lint + test)
check: lint test

# Clean up build binaries and temporary files
clean:
	rm -rf bin fetch-mix coverage.out .tmp dist
