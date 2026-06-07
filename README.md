# systemd-exporter

A Go exporter that collects systemd unit metrics for Prometheus monitoring.

It connects to the systemd D-Bus API, maintains a cached in-memory snapshot of
all unit states, and updates that cache only when state changes arrive — keeping
the overhead on the running system minimal.

## Table of contents

- [systemd-exporter](#systemd-exporter)
  - [Table of contents](#table-of-contents)
  - [Architecture](#architecture)
  - [systemd package (`internal/systemd`)](#systemd-package-internalsystemd)
    - [Collector](#collector)
      - [Available statistics](#available-statistics)
      - [Caching strategy](#caching-strategy)
    - [Service log access](#service-log-access)
  - [CLI](#cli)
    - [observe](#observe)
    - [Global flags](#global-flags)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
    - [From source](#from-source)
    - [go install](#go-install)
  - [Development](#development)
    - [Setup](#setup)
    - [Commands](#commands)
    - [Testing](#testing)
  - [License](#license)

---

## Architecture

```text
systemd-exporter/
├── cmd/
│   └── systemd-exporter/   ← binary entry point (main package)
│       └── main.go
└── internal/
    └── systemd/            ← D-Bus collector and journal log reader
        ├── types.go        ← UnitState, Stats, Snapshot, LogEntry
        ├── collector.go    ← Collector (caching, D-Bus subscription)
        ├── logs.go         ← GetServiceLogs (journal reader, requires -tags journal)
        └── logs_stub.go    ← GetServiceLogs stub when journal tag is absent
```

---

## systemd package (`internal/systemd`)

This package is the core of the exporter. It is responsible for all
communication with systemd via D-Bus and the journal.

### Collector

`Collector` connects to the **system D-Bus** and maintains a snapshot of all
units loaded by systemd. By default it collects all unit types (`.service`,
`.timer`, `.socket`, `.device`, etc.). Use options to restrict the scope.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// all unit types
c, err := systemd.New(ctx)

// only services
c, err := systemd.New(ctx, systemd.WithUnitType(".service"))

// custom filter
c, err := systemd.New(ctx, systemd.WithNameFilter(func(name string) bool {
    return strings.HasPrefix(name, "docker")
}))

defer c.Close()

snap := c.Snapshot()
fmt.Printf("total=%d active=%d failed=%d\n",
    snap.Stats.Total, snap.Stats.Active, snap.Stats.Failed)
```

#### Available statistics

| Field | Description |
| --- | --- |
| `Stats.Total` | All tracked units |
| `Stats.Active` | Units with `ActiveState == "active"` (running + completed oneshot) |
| `Stats.Failed` | Units with `ActiveState == "failed"` |
| `Stats.Dead` | Inactive units in sub-state `dead` |
| `Stats.Oneshot` | Active units in sub-state `exited` (completed oneshot) |

Each `UnitState` carries `Name`, `Description`, `LoadState`, `ActiveState`,
and `SubState` directly from the D-Bus `UnitStatus`.

#### Caching strategy

1. **Initial load** — on `New(ctx)`, `ListUnits()` is called once over D-Bus
   to build the initial `Snapshot`.
2. **Incremental updates** — a background goroutine calls
   `SubscribeUnitsCustom` (go-systemd) which polls `ListUnits` every **1 s**
   and pushes only the changed units into a channel.
3. **Copy-on-write** — `applyUpdates` copies the units map before mutating
   it, so callers that already hold a `Snapshot` value are never affected.
4. **Error recovery** — if the subscription channel signals an error, a full
   `refresh()` is performed as a fallback.

The result is that `Collector.Snapshot()` is always a cheap read-lock with no
D-Bus call, and the background goroutine touches D-Bus only once per second
regardless of how many callers exist.

### Service log access

`GetServiceLogs` reads the systemd journal for a specific unit. Requires
building with `-tags journal` and `libsystemd-dev` installed (see below).

```go
entries, err := systemd.GetServiceLogs("sshd.service", 50)
for _, e := range entries {
    fmt.Printf("[%s] %s\n", e.Timestamp.Format(time.RFC3339), e.Message)
}
```

Each `LogEntry` contains `Timestamp`, `Priority` (syslog level 0–7),
`Message`, and `Unit`.

> **Note:** reading the journal requires membership in the `systemd-journal`
> group or root privileges.

---

## CLI

```text
systemd-exporter <command> [flags]

Commands:
  observe              watch all unit states (stats + table)
  observe <unit>       show journal logs for a specific unit
```

### observe

```bash
# all units — live stats and state table, runs until Ctrl+C
systemd-exporter observe

# journal logs for a unit (last 50 lines)
systemd-exporter observe sshd.service

# journal logs — custom line count
systemd-exporter observe -n 100 docker.service
```

### Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `-log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |

---

## Prerequisites

- **Linux** with systemd
- **Go 1.23+**
- `golangci-lint` ≥ v2.1.6 (development)
- `pre-commit` (development)
- `libsystemd-dev` (for journal log support)

```bash
# golangci-lint
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6

# journal support headers (Debian/Ubuntu)
sudo apt update && sudo apt install -y libsystemd-dev
```

---

## Installation

### From source

```bash
git clone https://github.com/JaKafka/systemd-exporter
cd systemd-exporter
make build           # no journal support
make build-journal   # with journal support (requires libsystemd-dev)
```

### go install

```bash
go install github.com/JaKafka/systemd-exporter/cmd/systemd-exporter@latest
```

---

## Development

### Setup

```bash
pre-commit install
```

### Commands

| Command | Description |
| --- | --- |
| `make build` | Compile binary (no journal support) |
| `make build-journal` | Compile with journal support (requires `libsystemd-dev`) |
| `make test` | Run unit tests |
| `make test-integration` | Run integration tests (requires running systemd) |
| `make lint` | Run `golangci-lint` |
| `make tidy` | Run `go mod tidy` |
| `make clean` | Remove build artifacts |

### Testing

```bash
# unit tests — no systemd required
go test ./...

# integration tests — requires running systemd
go test -tags integration ./...
```

---

## License

MIT License — Copyright (c) 2026 Wiktor Szewczyk and Jakub Kawka.
See [LICENSE](LICENSE) for the full text.
