package postgres

import (
	"context"
	"testing"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

func seedMatchForPUUID(t *testing.T, store *Store, matchID, puuid, champion string, gameCreation int64) {
	t.Helper()
	match := &riot.Match{
		Metadata: riot.MatchMetadata{MatchID: matchID, Participants: []string{puuid}},
		Info: riot.MatchInfo{
			GameCreation: gameCreation,
			GameDuration: 1500,
			GameVersion:  "14.1.1",
			QueueID:      420,
			Participants: []riot.MatchParticipant{
				{PUUID: puuid, SummonerName: "Seed", ChampionName: champion, TeamPosition: "TOP", Win: true, Kills: 1, Deaths: 1, Assists: 1},
			},
		},
	}
	if err := store.SaveMatch(context.Background(), match); err != nil {
		t.Fatalf("seedMatchForPUUID(%s): %v", matchID, err)
	}
}

func TestMatchHistoryByPUUID_OrderedNewestFirst(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	seedMatchForPUUID(t, store, "H1_1", "puuid-history", "Ahri", 1700000000000)
	seedMatchForPUUID(t, store, "H1_2", "puuid-history", "Zed", 1700000500000)
	seedMatchForPUUID(t, store, "H1_3", "puuid-history", "Lux", 1700000200000)
	// A different player's match should never show up in puuid-history's list.
	seedMatchForPUUID(t, store, "H1_4", "someone-else", "Garen", 1700000900000)

	history, err := store.MatchHistoryByPUUID(ctx, "puuid-history", 10)
	if err != nil {
		t.Fatalf("MatchHistoryByPUUID: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 matches, got %d: %+v", len(history), history)
	}
	if history[0].MatchID != "H1_2" || history[1].MatchID != "H1_3" || history[2].MatchID != "H1_1" {
		t.Fatalf("unexpected order: %+v", history)
	}
}

func TestMatchHistoryByPUUID_RespectsLimit(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		seedMatchForPUUID(t, store, "H2_"+string(rune('a'+i)), "puuid-limit", "Ahri", int64(1700000000000+i*1000))
	}

	history, err := store.MatchHistoryByPUUID(ctx, "puuid-limit", 2)
	if err != nil {
		t.Fatalf("MatchHistoryByPUUID: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 matches (limit), got %d", len(history))
	}
}

func TestMatchHistoryByPUUID_EmptyForUnknownPlayer(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)

	history, err := store.MatchHistoryByPUUID(context.Background(), "nobody", 10)
	if err != nil {
		t.Fatalf("MatchHistoryByPUUID: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected no matches, got %+v", history)
	}
}
