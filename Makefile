.PHONY: test lint fmt build ci install-lint

GOFMT ?= $(shell go env GOROOT)/bin/gofmt
# Keep in sync with .github/workflows/ci.yml
GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_BIN ?= $(shell go env GOPATH)/bin/golangci-lint

fmt:
	$(GOFMT) -w .

test:
	go test ./...

install-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
		| sh -s -- -b "$$(go env GOPATH)/bin" $(GOLANGCI_LINT_VERSION)

lint:
	@command -v golangci-lint >/dev/null || { echo "run: make install-lint"; exit 1; }
	golangci-lint run

build:
	go build -o bin/tern ./cmd/tern

# Same checks as the CI "test" job (fmt + lint + unit tests)
ci: fmt
	@unformatted=$$($(GOFMT) -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted files:"; echo "$$unformatted"; exit 1; \
	fi
	@$(MAKE) lint
	@$(MAKE) test
	@echo "ci checks passed"
