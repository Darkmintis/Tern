.PHONY: test lint fmt build

GOFMT ?= $(shell go env GOROOT)/bin/gofmt

fmt:
	$(GOFMT) -w .

test:
	go test ./...

lint:
	golangci-lint run

build:
	go build -o bin/tern ./cmd/tern
