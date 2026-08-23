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

# Run tracklist acquisition pipeline
run *ARGS:
	go run ./cmd/fetch-mix {{ARGS}}

# Run YouTube comment tracklist extraction pipeline
youtube *ARGS:
	go run ./cmd/fetch-mix youtube {{ARGS}}

# Run all unit tests
test:
	go test -v ./...

# Run static code analysis
vet:
	go vet ./...

# Format Go code
fmt:
	go fmt ./...

# Clean up build binaries and temporary files
clean:
	rm -rf bin fetch-mix coverage.out .tmp dist
