.PHONY: build clean install test lint lint-md fmt help

# Binary name
BINARY_NAME=github-release-version-checker
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"
BUILDFLAGS=-trimpath

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOINSTALL=$(GOCMD) install

# Build targets
all: test build

build: ## Build the binary
	@echo "Building ${BINARY_NAME}..."
	CGO_ENABLED=0 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME} .
	@echo "✅ Build complete: bin/${BINARY_NAME}"

build-all: ## Build for all platforms
	@echo "Building for multiple platforms..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-arm64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME}-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME}-linux-arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME}-windows-amd64.exe .
	@echo "✅ Multi-platform build complete"

install: ## Install the binary to GOPATH/bin
	CGO_ENABLED=0 $(GOINSTALL) ${BUILDFLAGS} ${LDFLAGS} .

size: build ## Show binary size
	@echo "Binary sizes:"
	@ls -lh bin/${BINARY_NAME} | awk '{print "  " $$9 ": " $$5}'
	@stat -f%z bin/${BINARY_NAME} 2>/dev/null | awk '{printf "  Size: %.2f MB\n", $$1/1024/1024}' || stat -c%s bin/${BINARY_NAME} | awk '{printf "  Size: %.2f MB\n", $$1/1024/1024}'

size-all: build-all ## Show binary sizes for all platforms
	@echo "Binary sizes by platform:"
	@for f in bin/${BINARY_NAME}-*; do \
		SIZE=$$(stat -f%z "$$f" 2>/dev/null || stat -c%s "$$f"); \
		SIZE_MB=$$(echo "scale=2; $$SIZE/1024/1024" | bc); \
		printf "  %s: %s MB\n" "$$(basename $$f)" "$$SIZE_MB"; \
	done

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf bin/

test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests with coverage report
	$(GOCMD) tool cover -html=coverage.out

lint: ## Run Go linter
	@GOBIN_DIR="$$(go env GOBIN)"; \
	if [ -z "$$GOBIN_DIR" ]; then \
		GOBIN_DIR="$$(go env GOPATH)/bin"; \
	fi; \
	GOLANGCI_LINT_BIN="$$GOBIN_DIR/golangci-lint"; \
	if [ ! -x "$$GOLANGCI_LINT_BIN" ] || ! "$$GOLANGCI_LINT_BIN" version 2>/dev/null | grep -q "version 2."; then \
		echo "Installing golangci-lint v2.11.4..."; \
		GOBIN="$$GOBIN_DIR" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4; \
	fi; \
	"$$GOLANGCI_LINT_BIN" run ./...

lint-md: ## Run markdown linter
	@which markdownlint > /dev/null || (echo "markdownlint not found. Install with: npm install -g markdownlint-cli" && exit 1)
	markdownlint '**/*.md' --ignore node_modules

fmt: ## Format code
	$(GOCMD) fmt ./...
	$(GOCMD) mod tidy

deps: ## Download dependencies
	$(GOGET) -v ./...
	$(GOCMD) mod download

run: ## Run the application (example)
	CGO_ENABLED=0 $(GOBUILD) ${BUILDFLAGS} ${LDFLAGS} -o bin/${BINARY_NAME} .
	./bin/${BINARY_NAME} -c 2.327.1 -v

docker-build: ## Build Docker image
	docker build -t ${BINARY_NAME}:${VERSION} .

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
