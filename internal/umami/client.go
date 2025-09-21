package umami

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is a thin Umami API client used by the exporter.
// It is safe for concurrent use.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client

	mu    sync.RWMutex
	token string
}

// New creates a new Umami API client. If httpClient is nil a default one is created.
func New(baseURL, username, password string, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: httpClient,
	}
}

// Website represents a Umami tracked website.
type Website struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// StatValue represents a value with a previous value returned by Umami stats endpoints.
type StatValue struct {
	Value float64 `json:"value"`
	Prev  float64 `json:"prev"`
}

// WebsiteStats groups main site statistics returned by /stats.
type WebsiteStats struct {
	Pageviews StatValue `json:"pageviews"`
	Visitors  StatValue `json:"visitors"`
	Visits    StatValue `json:"visits"`
	Bounces   StatValue `json:"bounces"`
	Totaltime StatValue `json:"totaltime"`
}

// MetricEntry represents a single metric value returned by the /metrics endpoint.
type MetricEntry struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

// Login authenticates against Umami and stores the token in the client.
// The function is resilient and will try to discover common token keys in a JSON response
// or accept a raw string body.
func (c *Client) Login(ctx context.Context) error {
	payload := map[string]string{
		"username": c.username,
		"password": c.password,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	u := c.baseURL + "/api/auth/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("login failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	// Try to decode JSON, but accept raw token as fallback.
	var decoded interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		trim := strings.TrimSpace(string(body))
		if trim != "" {
			c.mu.Lock()
			c.token = trim
			c.mu.Unlock()
			return nil
		}
		return fmt.Errorf("login: cannot decode response: %w", err)
	}

	// Recursively search for token
	if token, ok := findToken(decoded); ok {
		c.mu.Lock()
		c.token = token
		c.mu.Unlock()
		return nil
	}

	return fmt.Errorf("login: token not found in response")
}

// findToken searches common token field names inside decoded JSON structures.
func findToken(v interface{}) (string, bool) {
	keys := []string{"token", "accessToken", "access_token", "jwt", "access_token"}
	switch t := v.(type) {
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return s, true
		}
	case map[string]interface{}:
		for _, k := range keys {
			if val, ok := t[k]; ok {
				if s, ok := val.(string); ok && s != "" {
					return s, true
				}
			}
		}
		// search nested maps
		for _, val := range t {
			if s, ok := findToken(val); ok {
				return s, true
			}
		}
	case []interface{}:
		for _, item := range t {
			if s, ok := findToken(item); ok {
				return s, true
			}
		}
	}
	return "", false
}

// ensureToken makes sure the client has a token, logging in if necessary.
func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.RLock()
	t := c.token
	c.mu.RUnlock()
	if t != "" {
		return nil
	}
	return c.Login(ctx)
}

// doRequest is a helper that performs authenticated requests to the Umami API.
// If result is non-nil the response body is decoded as JSON into result.
func (c *Client) doRequest(ctx context.Context, method, path string, query map[string]string, body interface{}, result interface{}) error {
	if err := c.ensureToken(ctx); err != nil {
		return err
	}

	u := c.baseURL
	if !strings.HasPrefix(path, "/") {
		u += "/"
	}
	u = strings.TrimRight(u, "/") + path

	if len(query) > 0 {
		vals := url.Values{}
		for k, v := range query {
			vals.Set(k, v)
		}
		u = u + "?" + vals.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	// If unauthorized, try to refresh token once.
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.Login(ctx); err != nil {
			return err
		}
		c.mu.RLock()
		token = c.token
		c.mu.RUnlock()
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: %s %s status=%d body=%s", method, u, resp.StatusCode, string(b))
	}

	if result != nil {
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(result); err != nil {
			return err
		}
	}
	return nil
}

// GetWebsites returns all tracked websites (up to a large pageSize).
func (c *Client) GetWebsites(ctx context.Context) ([]Website, error) {
	var resp struct {
		Data []Website `json:"data"`
	}
	q := map[string]string{"pageSize": strconv.Itoa(1000)}
	if err := c.doRequest(ctx, http.MethodGet, "/api/websites", q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetWebsiteStats fetches summarized stats for the website.
// It provides a default date range (last 30 days) as Umami expects numeric startAt/endAt.
func (c *Client) GetWebsiteStats(ctx context.Context, id string) (*WebsiteStats, error) {
	var ws WebsiteStats
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	q := map[string]string{
		"startAt": strconv.FormatInt(start.UnixMilli(), 10),
		"endAt":   strconv.FormatInt(now.UnixMilli(), 10),
	}
	if err := c.doRequest(ctx, http.MethodGet, "/api/websites/"+id+"/stats", q, nil, &ws); err != nil {
		return nil, err
	}
	return &ws, nil
}

// GetWebsiteActive returns number of active visitors for the website.
func (c *Client) GetWebsiteActive(ctx context.Context, id string) (float64, error) {
	var resp struct {
		Visitors float64 `json:"visitors"`
	}
	if err := c.doRequest(ctx, http.MethodGet, "/api/websites/"+id+"/active", nil, nil, &resp); err != nil {
		return 0, err
	}
	return resp.Visitors, nil
}

// GetWebsiteMetrics fetches metric entries for the given type (e.g. url, referrer).
// Adds a default date range (last 30 days) to conform with Umami API expectations.
func (c *Client) GetWebsiteMetrics(ctx context.Context, id, typ string, limit int) ([]MetricEntry, error) {
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	q := map[string]string{
		"type":    typ,
		"startAt": strconv.FormatInt(start.UnixMilli(), 10),
		"endAt":   strconv.FormatInt(now.UnixMilli(), 10),
	}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	var entries []MetricEntry
	if err := c.doRequest(ctx, http.MethodGet, "/api/websites/"+id+"/metrics", q, nil, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
