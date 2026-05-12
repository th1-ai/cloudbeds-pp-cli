.PHONY: build test lint install clean

build:
	go build -o bin/cloudbeds-pp-cli ./cmd/cloudbeds-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cloudbeds-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cloudbeds-pp-mcp ./cmd/cloudbeds-pp-mcp

install-mcp:
	go install ./cmd/cloudbeds-pp-mcp

build-all: build build-mcp
