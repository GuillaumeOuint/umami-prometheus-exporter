package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds exporter configuration read from environment variables.
type Config struct {
	UmamiURL    string
	Username    string
	Password    string
	Port        string
	Interval    time.Duration
	Concurrency int
	MetricLimit int
	MetricTypes []string
	HTTPTimeout time.Duration
}

// LoadFromEnv reads configuration from environment variables and returns a Config.
// Required environment variables:
//   - UMAMI_URL
//   - UMAMI_USERNAME
//   - UMAMI_PASSWORD
//
// Optional environment variables and defaults:
//   - EXPORTER_PORT (default "9465")
//   - UMAMI_REFRESH_INTERVAL (default "1m")
//   - UMAMI_CONCURRENCY (default 5)
//   - UMAMI_METRIC_LIMIT (default 100)
//   - UMAMI_METRIC_TYPES (comma-separated, default "url,referrer,browser,os,device,country,event")
//   - UMAMI_HTTP_TIMEOUT (default "15s")
func LoadFromEnv() (*Config, error) {
	u := strings.TrimSpace(os.Getenv("UMAMI_URL"))
	if u == "" {
		return nil, fmt.Errorf("UMAMI_URL is required")
	}

	// Validate and normalize URL. If scheme is missing try https:// prefix.
	if _, err := url.ParseRequestURI(u); err != nil {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u2 := "https://" + u
			if _, err2 := url.ParseRequestURI(u2); err2 == nil {
				u = u2
			} else {
				return nil, fmt.Errorf("UMAMI_URL invalid: %v", err)
			}
		} else {
			return nil, fmt.Errorf("UMAMI_URL invalid: %v", err)
		}
	}

	username := os.Getenv("UMAMI_USERNAME")
	password := os.Getenv("UMAMI_PASSWORD")
	if username == "" || password == "" {
		return nil, fmt.Errorf("UMAMI_USERNAME and UMAMI_PASSWORD are required")
	}

	port := os.Getenv("EXPORTER_PORT")
	if port == "" {
		port = "9465"
	}

	interval := time.Minute
	if s := os.Getenv("UMAMI_REFRESH_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			interval = d
		}
	}

	concurrency := 5
	if s := os.Getenv("UMAMI_CONCURRENCY"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			concurrency = v
		}
	}

	metricLimit := 100
	if s := os.Getenv("UMAMI_METRIC_LIMIT"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			metricLimit = v
		}
	}

	metricTypes := []string{"url", "referrer", "browser", "os", "device", "country", "event"}
	if s := os.Getenv("UMAMI_METRIC_TYPES"); s != "" {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			metricTypes = out
		}
	}

	timeout := 15 * time.Second
	if s := os.Getenv("UMAMI_HTTP_TIMEOUT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			timeout = d
		}
	}

	return &Config{
		UmamiURL:    u,
		Username:    username,
		Password:    password,
		Port:        port,
		Interval:    interval,
		Concurrency: concurrency,
		MetricLimit: metricLimit,
		MetricTypes: metricTypes,
		HTTPTimeout: timeout,
	}, nil
}
