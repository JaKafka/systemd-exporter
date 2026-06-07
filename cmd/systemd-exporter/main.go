package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/JaKafka/systemd-exporter/internal/systemd"
)

// version is stamped at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(*logLevel),
	}))
	slog.SetDefault(logger)

	switch flag.Arg(0) {
	case "observe":
		runObserve(flag.Args()[1:])
	default:
		// TODO(JK): add server here
		slog.Error("server not implemented yet")
		os.Exit(1)
	}
}

func runObserve(args []string) {
	fs := flag.NewFlagSet("observe", flag.ContinueOnError)
	n := fs.Int("n", 50, "number of log lines to show (when observing a specific unit)")
	if err := fs.Parse(args); err != nil {
		slog.Error("parse flags", "err", err)
		os.Exit(1)
	}

	unit := fs.Arg(0)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if unit != "" {
		runObserveUnit(ctx, unit, *n)
		return
	}

	runObserveAll(ctx)
}

func runObserveAll(ctx context.Context) {
	slog.Info("starting", "version", version)

	collector, err := systemd.New(ctx)
	if err != nil {
		slog.Error("connect to systemd D-Bus", "err", err)
		os.Exit(1)
	}
	defer collector.Close()

	snap := collector.Snapshot()

	fmt.Printf("Stats:\n")
	fmt.Printf("  total:   %d\n", snap.Stats.Total)
	fmt.Printf("  active:  %d\n", snap.Stats.Active)
	fmt.Printf("  failed:  %d\n", snap.Stats.Failed)
	fmt.Printf("  dead:    %d\n", snap.Stats.Dead)
	fmt.Printf("  oneshot: %d\n\n", snap.Stats.Oneshot)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tACTIVE\tSUB\tLOAD")
	for _, u := range snap.Units {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Name, u.ActiveState, u.SubState, u.LoadState)
	}
	_ = w.Flush()

	slog.Info("running — press Ctrl+C to stop")
	<-ctx.Done()
	slog.Info("shutting down")
}

func runObserveUnit(ctx context.Context, unit string, n int) {
	ch, err := systemd.StreamServiceLogs(ctx, unit, n)
	if err != nil {
		slog.Error("open journal", "unit", unit, "err", err)
		os.Exit(1)
	}

	slog.Info("streaming logs — press Ctrl+C to stop", "unit", unit)

	for entry := range ch {
		fmt.Printf("%s  %-6s  %s\n",
			entry.Timestamp.Format(time.RFC3339),
			priorityName(entry.Priority),
			entry.Message,
		)
	}
}

func priorityName(p int) string {
	switch p {
	case 0:
		return "EMERG"
	case 1:
		return "ALERT"
	case 2:
		return "CRIT"
	case 3:
		return "ERR"
	case 4:
		return "WARN"
	case 5:
		return "NOTICE"
	case 6:
		return "INFO"
	case 7:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
