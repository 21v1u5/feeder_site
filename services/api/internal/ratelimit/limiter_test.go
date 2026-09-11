package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T, windows []Window) *Limiter {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	return New(rdb, "test", windows)
}

func TestAllow_UnderLimit(t *testing.T) {
	l := newTestLimiter(t, []Window{{Period: time.Second, Limit: 3}})
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx, "region")
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !ok {
			t.Fatalf("request %d: expected allowed, got denied", i)
		}
	}
}

func TestAllow_RejectsOverLimit(t *testing.T) {
	l := newTestLimiter(t, []Window{{Period: time.Second, Limit: 2}})
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		ok, err := l.Allow(ctx, "region")
		if err != nil || !ok {
			t.Fatalf("request %d: expected allowed, got ok=%v err=%v", i, ok, err)
		}
	}

	ok, err := l.Allow(ctx, "region")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if ok {
		t.Fatal("expected 3rd request to be denied")
	}
}

func TestAllow_RespectsAllWindows(t *testing.T) {
	// A tight short window and a looser long window: the short one should
	// reject even though the long one still has room.
	l := newTestLimiter(t, []Window{
		{Period: time.Second, Limit: 1},
		{Period: time.Minute, Limit: 100},
	})
	ctx := context.Background()

	ok, err := l.Allow(ctx, "region")
	if err != nil || !ok {
		t.Fatalf("first request: expected allowed, got ok=%v err=%v", ok, err)
	}

	ok, err = l.Allow(ctx, "region")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if ok {
		t.Fatal("expected second request within the same second to be denied")
	}
}

func TestAllow_IndependentKeys(t *testing.T) {
	l := newTestLimiter(t, []Window{{Period: time.Second, Limit: 1}})
	ctx := context.Background()

	ok, err := l.Allow(ctx, "na1")
	if err != nil || !ok {
		t.Fatalf("na1: expected allowed, got ok=%v err=%v", ok, err)
	}

	ok, err = l.Allow(ctx, "euw1")
	if err != nil || !ok {
		t.Fatalf("euw1: expected allowed (independent key), got ok=%v err=%v", ok, err)
	}
}

func TestWait_UnblocksAfterWindowResets(t *testing.T) {
	l := newTestLimiter(t, []Window{{Period: 200 * time.Millisecond, Limit: 1}})
	ctx := context.Background()

	if ok, err := l.Allow(ctx, "region"); err != nil || !ok {
		t.Fatalf("first request: expected allowed, got ok=%v err=%v", ok, err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	if err := l.Wait(ctx2, "region"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("expected Wait to block until window reset, only waited %v", elapsed)
	}
}

func TestParseWindows(t *testing.T) {
	windows, err := ParseWindows("20:1s,100:2m")
	if err != nil {
		t.Fatalf("ParseWindows: %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	if windows[0].Limit != 20 || windows[0].Period != time.Second {
		t.Fatalf("unexpected first window: %+v", windows[0])
	}
	if windows[1].Limit != 100 || windows[1].Period != 2*time.Minute {
		t.Fatalf("unexpected second window: %+v", windows[1])
	}
}

func TestParseWindows_Empty(t *testing.T) {
	windows, err := ParseWindows("")
	if err != nil {
		t.Fatalf("ParseWindows: %v", err)
	}
	if windows != nil {
		t.Fatalf("expected nil windows for empty spec, got %+v", windows)
	}
}

func TestParseWindows_Invalid(t *testing.T) {
	if _, err := ParseWindows("nope"); err == nil {
		t.Fatal("expected error for malformed spec")
	}
	if _, err := ParseWindows("abc:1s"); err == nil {
		t.Fatal("expected error for non-numeric limit")
	}
	if _, err := ParseWindows("20:notaduration"); err == nil {
		t.Fatal("expected error for invalid period")
	}
}
