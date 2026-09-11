package ingestion

import (
	"context"
	"errors"
	"testing"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

type fakeRiotMatchClient struct {
	match *riot.Match
	err   error
	calls int
}

func (f *fakeRiotMatchClient) GetMatchByID(ctx context.Context, region, matchID string) (*riot.Match, error) {
	f.calls++
	return f.match, f.err
}

type fakeMatchStore struct {
	saved []*riot.Match
	err   error
}

func (f *fakeMatchStore) SaveMatch(ctx context.Context, match *riot.Match) error {
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, match)
	return nil
}

func TestHandleJob_Success(t *testing.T) {
	match := &riot.Match{Metadata: riot.MatchMetadata{MatchID: "NA1_1"}}
	client := &fakeRiotMatchClient{match: match}
	store := &fakeMatchStore{}
	w := NewWorker(client, store)

	if err := w.HandleJob(context.Background(), MatchJob{Region: "americas", MatchID: "NA1_1"}); err != nil {
		t.Fatalf("HandleJob: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0].Metadata.MatchID != "NA1_1" {
		t.Fatalf("expected match to be saved, got %+v", store.saved)
	}
}

func TestHandleJob_NotFoundIsNotAnError(t *testing.T) {
	client := &fakeRiotMatchClient{err: riot.ErrNotFound}
	store := &fakeMatchStore{}
	w := NewWorker(client, store)

	err := w.HandleJob(context.Background(), MatchJob{Region: "americas", MatchID: "NA1_gone"})
	if err != nil {
		t.Fatalf("expected nil error for not-found match, got %v", err)
	}
	if len(store.saved) != 0 {
		t.Fatalf("expected nothing saved for a not-found match, got %+v", store.saved)
	}
}

func TestHandleJob_FetchErrorPropagates(t *testing.T) {
	client := &fakeRiotMatchClient{err: riot.ErrRateLimited}
	store := &fakeMatchStore{}
	w := NewWorker(client, store)

	err := w.HandleJob(context.Background(), MatchJob{Region: "americas", MatchID: "NA1_1"})
	if !errors.Is(err, riot.ErrRateLimited) {
		t.Fatalf("expected wrapped ErrRateLimited, got %v", err)
	}
}

func TestHandleJob_StoreErrorPropagates(t *testing.T) {
	match := &riot.Match{Metadata: riot.MatchMetadata{MatchID: "NA1_1"}}
	client := &fakeRiotMatchClient{match: match}
	storeErr := errors.New("disk full")
	store := &fakeMatchStore{err: storeErr}
	w := NewWorker(client, store)

	err := w.HandleJob(context.Background(), MatchJob{Region: "americas", MatchID: "NA1_1"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected wrapped store error, got %v", err)
	}
}
