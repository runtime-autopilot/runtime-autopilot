BIN         := runtime-autopilot
BUILD_DIR   := ./bin
MODULE      := github.com/runtime-autopilot/runtime-autopilot
GO_FILES    := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: all build test lint clean tidy fmt adapters

all: build

## build: compile the binary for the current platform
build:
	go build -o $(BUILD_DIR)/$(BIN) ./cmd/runtime-autopilot

## test: run Go unit tests with the race detector
test:
	go test ./... -race -cover

## test-integration: run Go integration tests (requires real cgroup access)
test-integration:
	go test ./... -race -tags integration

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	gofmt -w $(GO_FILES)
	goimports -w $(GO_FILES)

## tidy: synchronise go.mod and go.sum
tidy:
	go mod tidy

## clean: remove build artefacts
clean:
	rm -rf $(BUILD_DIR)

## adapter-laravel: run Laravel PHPUnit tests
adapter-laravel:
	cd adapters/laravel && composer install --no-interaction && vendor/bin/phpunit

## adapter-django: run Django pytest tests
adapter-django:
	cd adapters/django && pip install -e ".[dev]" && pytest -v

## adapter-fastapi: run FastAPI pytest tests
adapter-fastapi:
	cd adapters/fastapi && pip install -e ".[dev]" fastapi httpx && pytest -v

## adapter-all: run all adapter tests
adapter-all: adapter-laravel adapter-django adapter-fastapi 

## help: print this help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
