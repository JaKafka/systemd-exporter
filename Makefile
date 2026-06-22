BIN    := systemd-exporter
MODULE := github.com/JaKafka/systemd-exporter

.PHONY: all build test test-integration test-integration-docker lint clean tidy

IT_IMAGE     := systemd-exporter-it
IT_CONTAINER := systemd-exporter-it

## Run tidy, lint, clean, build and unit tests.
all: tidy lint clean build test

## Compile the binary to bin/systemd-exporter.
build:
	go build -o bin/$(BIN) ./cmd/$(BIN)

## Run unit tests.
test:
	go test ./...

## Run unit + integration tests (requires a running systemd instance).
test-integration:
	go test -tags integration ./...

## Run integration tests inside a disposable container that boots real systemd.
test-integration-docker:
	docker build -t $(IT_IMAGE) -f docker/integration/Dockerfile .
	docker run -d --rm --name $(IT_CONTAINER) \
		--privileged --cgroupns=host \
		-v /sys/fs/cgroup:/sys/fs/cgroup:rw \
		$(IT_IMAGE)
	docker exec $(IT_CONTAINER) go test -tags integration ./... ; \
		status=$$?; \
		docker stop $(IT_CONTAINER) >/dev/null; \
		exit $$status

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
