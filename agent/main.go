// Package main implements the GitPlane cluster agent that runs inside
// managed Kubernetes clusters and reports FluxCD status back to the
// GitPlane control plane.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	apiURL := os.Getenv("GITPLANE_API_URL")
	if apiURL == "" {
		slog.Error("GITPLANE_API_URL is required")
		os.Exit(1)
	}

	agentToken := os.Getenv("GITPLANE_AGENT_TOKEN")
	if agentToken == "" {
		slog.Error("GITPLANE_AGENT_TOKEN is required")
		os.Exit(1)
	}

	interval := 60 * time.Second
	if raw := os.Getenv("GITPLANE_REPORT_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			slog.Error("invalid GITPLANE_REPORT_INTERVAL", "value", raw, "error", err)
			os.Exit(1)
		}
		interval = parsed
	}

	slog.Info("starting gitplane agent",
		"api_url", apiURL,
		"report_interval", interval.String(),
	)

	reporter, err := NewReporter(apiURL, agentToken, interval)
	if err != nil {
		slog.Error("failed to create reporter", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go reporter.Run(ctx)

	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig.String())
	cancel()

	// Give in-flight work a moment to finish.
	time.Sleep(2 * time.Second)
	slog.Info("agent stopped")
}
