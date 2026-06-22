package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/JaKafka/systemd-exporter/internal/systemd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// version is stamped at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	listenAddr := flag.String("web.listen-address", ":9558", "address to expose the /metrics endpoint on")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(*logLevel),
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch flag.Arg(0) {
	case "observe":
		runObserveAll(ctx)
	case "", "serve":
		runServe(ctx, *listenAddr)
	default:
		slog.Error("unknown command", "command", flag.Arg(0))
		os.Exit(1)
	}
}

func runServe(ctx context.Context, addr string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	slog.Info("starting", "version", version)

	collector, err := systemd.New(ctx)
	if err != nil {
		slog.Error("connect to systemd D-Bus", "err", err)
		os.Exit(1)
	}
	defer collector.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(systemd.NewExporter(collector))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("serving metrics", "addr", addr, "path", "/metrics")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "err", err)
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
