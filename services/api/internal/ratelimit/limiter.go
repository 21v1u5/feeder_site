// Package ratelimit implements a Redis-backed multi-window rate limiter,
// used to keep outgoing Riot API traffic under the quotas enforced per
// routing value (e.g. 20 req/1s and 100 req/120s on a development key).
package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Window describes one rate limit rule: at most Limit requests per Period.
type Window struct {
	Period time.Duration
	Limit  int64
}

// checkAndIncr atomically checks every window's counter for key and, only if
// all of them are still under their limit, increments all of them. This
// avoids partially consuming a window when another window rejects the
// request.
var checkAndIncr = redis.NewScript(`
local n = #KEYS
for i = 1, n do
	local limit = tonumber(ARGV[2*i-1])
	local current = tonumber(redis.call('GET', KEYS[i]) or '0')
	if current >= limit then
		return 0
	end
end
for i = 1, n do
	local ttl = tonumber(ARGV[2*i])
	local newval = redis.call('INCR', KEYS[i])
	if newval == 1 then
		redis.call('EXPIRE', KEYS[i], ttl)
	end
end
return 1
`)

// Limiter enforces a fixed set of windows against a shared Redis instance,
// so multiple API instances draw from the same quota.
type Limiter struct {
	rdb     *redis.Client
	windows []Window
	prefix  string
}

func New(rdb *redis.Client, prefix string, windows []Window) *Limiter {
	return &Limiter{rdb: rdb, windows: windows, prefix: prefix}
}

// Allow reports whether a request tagged with key is allowed right now,
// consuming one unit from every configured window if so.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	if len(l.windows) == 0 {
		return true, nil
	}

	keys := make([]string, len(l.windows))
	args := make([]interface{}, 0, len(l.windows)*2)
	now := time.Now().UnixNano()

	for i, w := range l.windows {
		bucket := now / w.Period.Nanoseconds()
		keys[i] = fmt.Sprintf("%s:%s:%s:%d", l.prefix, key, w.Period, bucket)
		ttlSeconds := int64(w.Period.Seconds())
		if ttlSeconds < 1 {
			ttlSeconds = 1
		}
		args = append(args, w.Limit, ttlSeconds)
	}

	res, err := checkAndIncr.Run(ctx, l.rdb, keys, args...).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// Wait blocks, retrying with backoff, until a request tagged with key is
// allowed or ctx is done. Intended for background workers that can afford to
// queue; interactive request paths should prefer Allow and fail fast.
func (l *Limiter) Wait(ctx context.Context, key string) error {
	backoff := 50 * time.Millisecond
	const maxBackoff = 2 * time.Second

	for {
		ok, err := l.Allow(ctx, key)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// ParseWindows parses a spec like "20:1s,100:2m" (limit:period pairs
// separated by commas) into a slice of Window.
func ParseWindows(spec string) ([]Window, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}

	parts := strings.Split(spec, ",")
	windows := make([]Window, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.SplitN(part, ":", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("ratelimit: invalid window %q, expected limit:period", part)
		}
		limit, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ratelimit: invalid limit in %q: %w", part, err)
		}
		period, err := time.ParseDuration(fields[1])
		if err != nil {
			return nil, fmt.Errorf("ratelimit: invalid period in %q: %w", part, err)
		}
		windows = append(windows, Window{Period: period, Limit: limit})
	}
	return windows, nil
}
