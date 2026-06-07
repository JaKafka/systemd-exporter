BIN    := systemd-exporter
MODULE := github.com/JaKafka/systemd-exporter

.PHONY: all build build-journal test test-integration lint clean tidy

## Run tidy, lint, clean, build and unit tests.
all: tidy lint clean build test

## Compile the binary to bin/systemd-exporter (journal support disabled).
build:
	go build -o bin/$(BIN) ./cmd/$(BIN)

## Compile with full journal support (requires libsystemd-dev installed).
build-journal:
	go build -tags journal -o bin/$(BIN) ./cmd/$(BIN)

## Run unit tests.
test:
	go test ./...

## Run unit + integration tests (requires a running systemd instance).
test-integration:
	go test -tags integration ./...

## Run golangci-lint.
lint:
	golangci-lint run ./...

## Remove build artifacts.
clean:
	rm -rf bin/

## Tidy go.mod and go.sum.
tidy:
	go mod tidy

.DEFAULT_GOAL := all
