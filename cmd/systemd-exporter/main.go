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
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		runObserveAll(ctx)
	default:
		// TODO(JK): add server here
		slog.Error("server not implemented yet")
		os.Exit(1)
	}
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
