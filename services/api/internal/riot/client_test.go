package riot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// noopLimiter never blocks, so tests exercise only the HTTP/JSON plumbing.
type noopLimiter struct{}

func (noopLimiter) Wait(ctx context.Context, key string) error { return nil }

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClient("test-api-key", noopLimiter{}, WithBaseURL(srv.URL))
	return client, srv
}

func TestGetAccountByRiotID(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Riot-Token"); got != "test-api-key" {
			t.Errorf("missing/wrong X-Riot-Token header: %q", got)
		}
		wantPath := "/riot/account/v1/accounts/by-riot-id/Faker/KR1"
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"puuid":"abc123","gameName":"Faker","tagLine":"KR1"}`))
	})

	account, err := client.GetAccountByRiotID(context.Background(), "asia", "Faker", "KR1")
	if err != nil {
		t.Fatalf("GetAccountByRiotID: %v", err)
	}
	if account.PUUID != "abc123" || account.GameName != "Faker" {
		t.Errorf("unexpected account: %+v", account)
	}
}

func TestGetMatchIDsByPUUID_QueryParams(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("start"); got != "0" {
			t.Errorf("start = %q, want 0", got)
		}
		if got := r.URL.Query().Get("count"); got != "20" {
			t.Errorf("count = %q, want 20", got)
		}
		w.Write([]byte(`["m1","m2"]`))
	})

	ids, err := client.GetMatchIDsByPUUID(context.Background(), "americas", "puuid", 0, 20)
	if err != nil {
		t.Fatalf("GetMatchIDsByPUUID: %v", err)
	}
	if len(ids) != 2 || ids[0] != "m1" {
		t.Errorf("unexpected ids: %+v", ids)
	}
}

func TestGet_NotFound(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetSummonerByPUUID(context.Background(), "na1", "missing")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_RateLimited(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.GetLeagueEntriesByPUUID(context.Background(), "na1", "puuid")
	if err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

// blockingLimiter lets the test assert the client actually consults the
// limiter before making the HTTP call.
type blockingLimiter struct{ called bool }

func (b *blockingLimiter) Wait(ctx context.Context, key string) error {
	b.called = true
	return nil
}

func TestGet_ConsultsLimiter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	limiter := &blockingLimiter{}
	client := NewClient("key", limiter, WithBaseURL(srv.URL))

	if _, err := client.GetSummonerByPUUID(context.Background(), "na1", "puuid"); err != nil {
		t.Fatalf("GetSummonerByPUUID: %v", err)
	}
	if !limiter.called {
		t.Fatal("expected client to call limiter.Wait before the request")
	}
}
