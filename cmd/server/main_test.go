package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"barterswap/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunFailsOnUnreachableDatabase(t *testing.T) {
	cfg := config.Config{
		Address:      "127.0.0.1:0",
		DatabaseURL:  "postgres://bad:bad@127.0.0.1:1/none?sslmode=disable&connect_timeout=1",
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		IdleTimeout:  time.Second,
	}
	if err := run(context.Background(), discardLogger(), cfg); err == nil {
		t.Fatal("run() with an unreachable database expected an error")
	}
}

func TestRunServesAndShutsDown(t *testing.T) {
	if os.Getenv("RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set RUN_POSTGRES_INTEGRATION=1 to run the server integration test")
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for the server integration test")
	}

	address := freeAddress(t)
	cfg := config.Config{
		Address:      address,
		DatabaseURL:  databaseURL,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- run(ctx, discardLogger(), cfg) }()

	healthURL := "http://" + address + "/healthz"
	if !waitForHealthy(healthURL, 15*time.Second) {
		cancel()
		t.Fatalf("server did not become healthy at %s", healthURL)
	}

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("run() returned error = %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run() did not return after shutdown")
	}
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()
	return address
}

func waitForHealthy(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
