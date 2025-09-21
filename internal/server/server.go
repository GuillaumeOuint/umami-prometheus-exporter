package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/GuillaumeOuint/umami-prometheus-exporter/internal/updater"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHTTPServer builds an *http.Server serving /metrics and /healthz.
// addr should be in the form ":9465" or "0.0.0.0:9465".
func NewHTTPServer(addr string, u *updater.Updater, logger *log.Logger) *http.Server {
	if logger == nil {
		logger = log.Default()
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		last := int64(0)
		success := false
		if u != nil {
			last = u.LastFetchUnix()
			success = u.LastSuccess()
		}
		type resp struct {
			LastFetch int64 `json:"last_fetch"`
			Success   bool  `json:"success"`
		}
		res := resp{LastFetch: last, Success: success}
		if !success {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv
}

// Shutdown attempts a graceful shutdown with the given timeout.
func Shutdown(ctx context.Context, srv *http.Server, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}
	logger.Println("server: shutting down")
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
