//go:build integration

package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRunObserveAll_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		runObserveAll(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runObserveAll did not return after context cancellation")
	}
}

func TestRunServe_Integration(t *testing.T) {
	const addr = ":19558"

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runServe(ctx, addr)
		close(done)
	}()

	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://localhost" + addr + "/metrics")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cancel()
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "systemd_units") {
		t.Errorf("expected response to contain %q, got: %s", "systemd_units", body)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runServe did not shut down after context cancellation")
	}
}
