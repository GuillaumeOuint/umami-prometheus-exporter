package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus collectors used by the exporter.
type Metrics struct {
	FetchSuccess            prometheus.Gauge
	LastFetch               prometheus.Gauge
	WebsitePageviews        *prometheus.GaugeVec
	WebsiteVisitors         *prometheus.GaugeVec
	WebsiteVisits           *prometheus.GaugeVec
	WebsiteBounces          *prometheus.GaugeVec
	WebsiteTotaltimeSeconds *prometheus.GaugeVec
	WebsiteActiveVisitors   *prometheus.GaugeVec
	MetricValues            *prometheus.GaugeVec
}

// New creates and registers Prometheus metrics.
func New() *Metrics {
	m := &Metrics{
		FetchSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "umami_fetch_success",
			Help: "1 if last refresh to Umami API was successful, 0 otherwise",
		}),
		LastFetch: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "umami_last_fetch_timestamp_seconds",
			Help: "Unix timestamp of last successful fetch",
		}),
		WebsitePageviews: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_website_pageviews",
			Help: "Pageviews for website (current value)",
		}, []string{"website_id", "name", "domain"}),
		WebsiteVisitors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_website_visitors",
			Help: "Visitors for website (current value)",
		}, []string{"website_id", "name", "domain"}),
		WebsiteVisits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_website_visits",
			Help: "Visits for website (current value)",
		}, []string{"website_id", "name", "domain"}),
		WebsiteBounces: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_website_bounces",
			Help: "Bounces for website (current value)",
		}, []string{"website_id", "name", "domain"}),
		WebsiteTotaltimeSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_website_totaltime_seconds",
			Help: "Total time spent on website (seconds)",
		}, []string{"website_id", "name", "domain"}),
		WebsiteActiveVisitors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_website_active_visitors",
			Help: "Number of active visitors in last 5 minutes",
		}, []string{"website_id", "name", "domain"}),
		MetricValues: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "umami_metric_value",
			Help: "Metric value for a website for a given type and value (e.g. url /path => count)",
		}, []string{"website_id", "name", "domain", "type", "value"}),
	}

	prometheus.MustRegister(
		m.FetchSuccess,
		m.LastFetch,
		m.WebsitePageviews,
		m.WebsiteVisitors,
		m.WebsiteVisits,
		m.WebsiteBounces,
		m.WebsiteTotaltimeSeconds,
		m.WebsiteActiveVisitors,
		m.MetricValues,
	)
	return m
}
