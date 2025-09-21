package updater

import (
	"context"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	prommetrics "github.com/GuillaumeOuint/umami-prometheus-exporter/internal/metrics"
	"github.com/GuillaumeOuint/umami-prometheus-exporter/internal/umami"
)

// Updater periodically fetches data from Umami and updates Prometheus metrics.
type Updater struct {
	client      *umami.Client
	metrics     *prommetrics.Metrics
	interval    time.Duration
	concurrency int
	metricLimit int
	metricTypes []string
	logger      *log.Logger

	lastSuccess   int32
	lastFetchUnix int64
}

// New creates a new Updater instance.
func New(client *umami.Client, m *prommetrics.Metrics, interval time.Duration, concurrency, metricLimit int, metricTypes []string, logger *log.Logger) *Updater {
	if logger == nil {
		logger = log.Default()
	}
	return &Updater{
		client:      client,
		metrics:     m,
		interval:    interval,
		concurrency: concurrency,
		metricLimit: metricLimit,
		metricTypes: metricTypes,
		logger:      logger,
	}
}

// LastSuccess returns whether the last update was successful.
func (u *Updater) LastSuccess() bool {
	return atomic.LoadInt32(&u.lastSuccess) == 1
}

// LastFetchUnix returns the unix timestamp of the last successful fetch.
func (u *Updater) LastFetchUnix() int64 {
	return atomic.LoadInt64(&u.lastFetchUnix)
}

// fetchAndUpdate performs a single update cycle.
func (u *Updater) fetchAndUpdate(ctx context.Context) {
	u.logger.Println("updater: starting update")
	start := time.Now()

	websites, err := u.client.GetWebsites(ctx)
	if err != nil {
		u.logger.Printf("updater: failed to list websites: %v", err)
		if u.metrics != nil {
			u.metrics.FetchSuccess.Set(0)
		}
		atomic.StoreInt32(&u.lastSuccess, 0)
		return
	}

	// best-effort reset of dynamic metrics to avoid stale label values.
	if u.metrics != nil {
		func() {
			defer func() { _ = recover() }()
			u.metrics.WebsitePageviews.Reset()
			u.metrics.WebsiteVisitors.Reset()
			u.metrics.WebsiteVisits.Reset()
			u.metrics.WebsiteBounces.Reset()
			u.metrics.WebsiteTotaltimeSeconds.Reset()
			u.metrics.WebsiteActiveVisitors.Reset()
			u.metrics.MetricValues.Reset()
		}()
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, u.concurrency)

	for _, w := range websites {
		select {
		case <-ctx.Done():
			u.logger.Println("updater: context canceled, aborting update")
			return
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(w umami.Website) {
			defer wg.Done()
			defer func() { <-sem }()

			// Fetch summarized stats
			stats, err := u.client.GetWebsiteStats(ctx, w.ID)
			if err != nil {
				u.logger.Printf("updater: website %s stats error: %v", w.ID, err)
			} else if stats != nil {
				u.metrics.WebsitePageviews.WithLabelValues(w.ID, w.Name, w.Domain).Set(stats.Pageviews.Value)
				u.metrics.WebsiteVisitors.WithLabelValues(w.ID, w.Name, w.Domain).Set(stats.Visitors.Value)
				u.metrics.WebsiteVisits.WithLabelValues(w.ID, w.Name, w.Domain).Set(stats.Visits.Value)
				u.metrics.WebsiteBounces.WithLabelValues(w.ID, w.Name, w.Domain).Set(stats.Bounces.Value)
				u.metrics.WebsiteTotaltimeSeconds.WithLabelValues(w.ID, w.Name, w.Domain).Set(stats.Totaltime.Value)
			}

			// Active visitors
			if v, err := u.client.GetWebsiteActive(ctx, w.ID); err != nil {
				u.logger.Printf("updater: website %s active error: %v", w.ID, err)
			} else {
				u.metrics.WebsiteActiveVisitors.WithLabelValues(w.ID, w.Name, w.Domain).Set(v)
			}

			// Metrics by type (url, referrer, browser, ...)
			for _, typ := range u.metricTypes {
				entries, err := u.client.GetWebsiteMetrics(ctx, w.ID, typ, u.metricLimit)
				if err != nil {
					u.logger.Printf("updater: website %s metrics type %s error: %v", w.ID, typ, err)
					continue
				}
				for _, e := range entries {
					val := strings.TrimSpace(e.X)
					if val == "" {
						val = "<empty>"
					}
					u.metrics.MetricValues.WithLabelValues(w.ID, w.Name, w.Domain, typ, val).Set(e.Y)
				}
			}
		}(w)
	}

	wg.Wait()

	// update success indicators
	if u.metrics != nil {
		u.metrics.FetchSuccess.Set(1)
	}
	atomic.StoreInt32(&u.lastSuccess, 1)
	now := time.Now().Unix()
	if u.metrics != nil {
		u.metrics.LastFetch.Set(float64(now))
	}
	atomic.StoreInt64(&u.lastFetchUnix, now)
	u.logger.Printf("updater: finished update: websites=%d duration=%s", len(websites), time.Since(start))
}

// Start runs the updater loop until ctx is canceled.
func (u *Updater) Start(ctx context.Context) {
	// Immediate update
	u.fetchAndUpdate(ctx)

	if u.interval <= 0 {
		u.interval = time.Minute
	}

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			u.logger.Println("updater: stopping")
			return
		case <-ticker.C:
			u.fetchAndUpdate(ctx)
		}
	}
}
