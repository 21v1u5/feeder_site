package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

// RiotMatchClient is the subset of *riot.Client the worker needs.
type RiotMatchClient interface {
	GetMatchByID(ctx context.Context, region, matchID string) (*riot.Match, error)
}

// MatchStore persists a fully-fetched match. The Postgres-backed
// implementation lands in a later stage; until then LoggingStore keeps the
// pipeline runnable end to end.
type MatchStore interface {
	SaveMatch(ctx context.Context, match *riot.Match) error
}

// Worker turns a MatchJob into a fetched, persisted match.
type Worker struct {
	riot  RiotMatchClient
	store MatchStore
}

func NewWorker(riotClient RiotMatchClient, store MatchStore) *Worker {
	return &Worker{riot: riotClient, store: store}
}

// HandleJob fetches the match and hands it to the store. A not-found match
// (e.g. a stale/remade game id) is treated as done rather than retried.
func (w *Worker) HandleJob(ctx context.Context, job MatchJob) error {
	match, err := w.riot.GetMatchByID(ctx, job.Region, job.MatchID)
	if err != nil {
		if errors.Is(err, riot.ErrNotFound) {
			log.Printf("ingestion: match %s not found upstream, skipping", job.MatchID)
			return nil
		}
		return fmt.Errorf("fetching match %s: %w", job.MatchID, err)
	}

	if err := w.store.SaveMatch(ctx, match); err != nil {
		return fmt.Errorf("saving match %s: %w", job.MatchID, err)
	}
	return nil
}

// LoggingStore is a placeholder MatchStore used until Postgres persistence
// (a later stage) is wired in.
type LoggingStore struct{}

func (LoggingStore) SaveMatch(ctx context.Context, match *riot.Match) error {
	log.Printf(
		"ingestion: fetched match %s (%d participants, %ds, patch %s)",
		match.Metadata.MatchID,
		len(match.Metadata.Participants),
		match.Info.GameDuration,
		match.Info.GameVersion,
	)
	return nil
}
