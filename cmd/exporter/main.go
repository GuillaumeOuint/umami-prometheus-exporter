package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GuillaumeOuint/umami-prometheus-exporter/internal/config"
	prommetrics "github.com/GuillaumeOuint/umami-prometheus-exporter/internal/metrics"
	"github.com/GuillaumeOuint/umami-prometheus-exporter/internal/server"
	"github.com/GuillaumeOuint/umami-prometheus-exporter/internal/umami"
	"github.com/GuillaumeOuint/umami-prometheus-exporter/internal/updater"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}

	client := umami.New(cfg.UmamiURL, cfg.Username, cfg.Password, httpClient)

	metrics := prommetrics.New()

	upd := updater.New(client, metrics, cfg.Interval, cfg.Concurrency, cfg.MetricLimit, cfg.MetricTypes, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start updater loop
	go upd.Start(ctx)

	srv := server.NewHTTPServer(":"+cfg.Port, upd, logger)

	// Start HTTP server
	go func() {
		logger.Printf("server: starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server: listen error: %v", err)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	logger.Println("main: shutdown signal received")

	// Shutdown HTTP server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx, srv, logger); err != nil {
		logger.Printf("main: server shutdown error: %v", err)
	}

	// Allow updater to finish (it listens on ctx), then exit
	time.Sleep(100 * time.Millisecond)
	logger.Println("main: exiting")
}
