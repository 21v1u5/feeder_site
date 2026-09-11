// Package riot is a thin client for the Riot Games API, gated by a
// centralized rate limiter so callers never need to reason about the
// per-application quota themselves.
package riot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ErrNotFound is returned when the Riot API responds 404 (e.g. unknown
// summoner or match id).
var ErrNotFound = errors.New("riot: resource not found")

// ErrRateLimited is returned when the Riot API itself responds 429, which
// should only happen if the local limiter's windows are misconfigured
// relative to the actual key quota.
var ErrRateLimited = errors.New("riot: rate limited by upstream")

// RateLimiter is the subset of ratelimit.Limiter the client depends on, kept
// as an interface so it can be faked in tests.
type RateLimiter interface {
	Wait(ctx context.Context, key string) error
}

type Client struct {
	httpClient *http.Client
	apiKey     string
	limiter    RateLimiter
	baseScheme string
	baseHost   string // e.g. "api.riotgames.com", overridable in tests
}

type Option func(*Client)

// WithHTTPClient overrides the default HTTP client (useful for tests/timeouts).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithBaseURL points the client at a different host/scheme, e.g. a local
// httptest server in unit tests.
func WithBaseURL(rawURL string) Option {
	return func(c *Client) {
		u, err := url.Parse(rawURL)
		if err != nil {
			return
		}
		c.baseScheme = u.Scheme
		c.baseHost = u.Host
	}
}

func NewClient(apiKey string, limiter RateLimiter, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		limiter:    limiter,
		baseScheme: "https",
		baseHost:   "", // set per-request from the routing value unless overridden
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetAccountByRiotID resolves a Riot ID (gameName#tagLine) to a PUUID.
// region is a regional routing value: americas, europe, asia.
func (c *Client) GetAccountByRiotID(ctx context.Context, region, gameName, tagLine string) (*Account, error) {
	path := fmt.Sprintf("/riot/account/v1/accounts/by-riot-id/%s/%s", url.PathEscape(gameName), url.PathEscape(tagLine))
	var account Account
	if err := c.get(ctx, region, path, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

// GetSummonerByPUUID fetches summoner data. platform is a platform routing
// value: na1, euw1, kr, ...
func (c *Client) GetSummonerByPUUID(ctx context.Context, platform, puuid string) (*Summoner, error) {
	path := fmt.Sprintf("/lol/summoner/v4/summoners/by-puuid/%s", url.PathEscape(puuid))
	var summoner Summoner
	if err := c.get(ctx, platform, path, &summoner); err != nil {
		return nil, err
	}
	return &summoner, nil
}

// GetLeagueEntriesByPUUID fetches ranked standings across queues.
func (c *Client) GetLeagueEntriesByPUUID(ctx context.Context, platform, puuid string) ([]LeagueEntry, error) {
	path := fmt.Sprintf("/lol/league/v4/entries/by-puuid/%s", url.PathEscape(puuid))
	var entries []LeagueEntry
	if err := c.get(ctx, platform, path, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetMatchIDsByPUUID lists recent match ids. region is a regional routing
// value: americas, europe, asia.
func (c *Client) GetMatchIDsByPUUID(ctx context.Context, region, puuid string, start, count int) ([]string, error) {
	path := fmt.Sprintf("/lol/match/v5/matches/by-puuid/%s/ids", url.PathEscape(puuid))
	path += fmt.Sprintf("?start=%d&count=%d", start, count)
	var ids []string
	if err := c.get(ctx, region, path, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// GetMatchByID fetches full match details.
func (c *Client) GetMatchByID(ctx context.Context, region, matchID string) (*Match, error) {
	path := fmt.Sprintf("/lol/match/v5/matches/%s", url.PathEscape(matchID))
	var match Match
	if err := c.get(ctx, region, path, &match); err != nil {
		return nil, err
	}
	return &match, nil
}

func (c *Client) get(ctx context.Context, routing, path string, out interface{}) error {
	if err := c.limiter.Wait(ctx, routing); err != nil {
		return fmt.Errorf("riot: rate limiter wait: %w", err)
	}

	host := c.baseHost
	if host == "" {
		host = routing + ".api.riotgames.com"
	}
	parsedPath, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("riot: invalid path %q: %w", path, err)
	}
	reqURL := url.URL{
		Scheme:   c.baseScheme,
		Host:     host,
		Path:     parsedPath.Path,
		RawQuery: parsedPath.RawQuery,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("riot: build request: %w", err)
	}
	req.Header.Set("X-Riot-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("riot: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("riot: read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("riot: decode response: %w", err)
		}
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return fmt.Errorf("riot: unexpected status %s: %s", resp.Status, truncate(body, 200))
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(" + strconv.Itoa(len(b)-n) + " more bytes)"
}
