BIN    := systemd-exporter
MODULE := github.com/JaKafka/systemd-exporter

.PHONY: all build test test-integration test-integration-docker coverage lint clean tidy

IT_IMAGE     := systemd-exporter-it
IT_CONTAINER := systemd-exporter-it

## Runs tidy, lint, clean, build, then unit tests.
all: tidy lint clean build test

## Builds the binary at bin/systemd-exporter.
build:
	go build -o bin/$(BIN) ./cmd/$(BIN)

## Runs unit tests and writes coverage-unit.out.
test:
	go test -coverprofile=coverage-unit.out ./...

## Runs unit + integration tests against the host's systemd and writes coverage-integration.out.
## Requires a running systemd instance.
test-integration:
	go test -tags integration -coverprofile=coverage-integration.out ./...

## Runs integration tests inside a disposable container with a real systemd, and writes
## coverage-integration.out on the host.
test-integration-docker:
	docker build -t $(IT_IMAGE) -f docker/integration/Dockerfile .
	docker run -d --rm --name $(IT_CONTAINER) \
		--privileged --cgroupns=host \
		-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
		$(IT_IMAGE)
	docker exec $(IT_CONTAINER) go test -tags integration -coverprofile=/src/coverage-integration.out ./... ; \
		status=$$?; \
		docker cp $(IT_CONTAINER):/src/coverage-integration.out coverage-integration.out 2>/dev/null; \
		docker stop $(IT_CONTAINER) >/dev/null; \
		exit $$status

## File target backing `make coverage`: runs `make test` only if coverage-unit.out is missing.
coverage-unit.out:
	$(MAKE) test

## File target backing `make coverage`: runs `make test-integration-docker` only if
## coverage-integration.out is missing.
coverage-integration.out:
	$(MAKE) test-integration-docker

## Merges coverage-unit.out and coverage-integration.out into coverage.out and prints the
## per-function breakdown. Reuses existing coverage files instead of rerunning tests.
coverage: coverage-unit.out coverage-integration.out
	{ head -n1 coverage-unit.out; tail -n +2 coverage-unit.out; tail -n +2 coverage-integration.out; } > coverage.out
	go tool cover -func=coverage.out

## Runs golangci-lint.
lint:
	golangci-lint run ./...

## Removes build and coverage artifacts.
clean:
	rm -rf bin/ coverage.out coverage.html coverage-unit.out coverage-integration.out

## Tidies go.mod and go.sum.
tidy:
	go mod tidy

.DEFAULT_GOAL := all
