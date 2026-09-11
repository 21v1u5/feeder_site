package postgres

import (
	"context"
	"testing"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

// seedMatch inserts a single-participant match so tests can control exact
// win/loss counts per champion without needing 5v5 realism.
func seedMatch(t *testing.T, store *Store, matchID, patch string, queueID int, champion string, win bool) {
	t.Helper()
	match := &riot.Match{
		Metadata: riot.MatchMetadata{MatchID: matchID, Participants: []string{"puuid-1"}},
		Info: riot.MatchInfo{
			GameCreation: 1700000000000,
			GameDuration: 1500,
			GameVersion:  patch,
			QueueID:      queueID,
			Participants: []riot.MatchParticipant{
				{PUUID: "puuid-1", SummonerName: "Seed", ChampionName: champion, TeamPosition: "TOP", Win: win, Kills: 5, Deaths: 3, Assists: 2},
			},
		},
	}
	if err := store.SaveMatch(context.Background(), match); err != nil {
		t.Fatalf("seedMatch(%s): %v", matchID, err)
	}
}

func TestTierList_AggregatesWinRateAndOrdering(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	const patch = "14.1.1"
	const queueID = 420

	// Ahri: 3 games, 3 wins -> 100% win rate.
	seedMatch(t, store, "T1_1", patch, queueID, "Ahri", true)
	seedMatch(t, store, "T1_2", patch, queueID, "Ahri", true)
	seedMatch(t, store, "T1_3", patch, queueID, "Ahri", true)

	// Garen: 4 games, 1 win -> 25% win rate.
	seedMatch(t, store, "T1_4", patch, queueID, "Garen", true)
	seedMatch(t, store, "T1_5", patch, queueID, "Garen", false)
	seedMatch(t, store, "T1_6", patch, queueID, "Garen", false)
	seedMatch(t, store, "T1_7", patch, queueID, "Garen", false)

	if err := store.RefreshChampionStats(ctx); err != nil {
		t.Fatalf("RefreshChampionStats: %v", err)
	}

	stats, err := store.TierList(ctx, queueID, patch, 1)
	if err != nil {
		t.Fatalf("TierList: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 champions, got %d: %+v", len(stats), stats)
	}

	// Ordered by win rate descending: Ahri (100%) before Garen (25%).
	if stats[0].Champion != "Ahri" || stats[0].Games != 3 || stats[0].Wins != 3 || stats[0].WinRate != 100 {
		t.Fatalf("unexpected first row: %+v", stats[0])
	}
	if stats[1].Champion != "Garen" || stats[1].Games != 4 || stats[1].Wins != 1 || stats[1].WinRate != 25 {
		t.Fatalf("unexpected second row: %+v", stats[1])
	}
}

func TestTierList_FiltersByMinGames(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	const patch = "14.2.1"
	const queueID = 420

	seedMatch(t, store, "T2_1", patch, queueID, "Zed", true) // only 1 game

	if err := store.RefreshChampionStats(ctx); err != nil {
		t.Fatalf("RefreshChampionStats: %v", err)
	}

	stats, err := store.TierList(ctx, queueID, patch, 5)
	if err != nil {
		t.Fatalf("TierList: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected champions with <5 games to be filtered out, got %+v", stats)
	}

	stats, err = store.TierList(ctx, queueID, patch, 1)
	if err != nil {
		t.Fatalf("TierList: %v", err)
	}
	if len(stats) != 1 || stats[0].Champion != "Zed" {
		t.Fatalf("expected Zed with minGames=1, got %+v", stats)
	}
}

func TestTierList_IsolatedByQueueAndPatch(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	seedMatch(t, store, "T3_1", "14.3.1", 420, "Lux", true)
	seedMatch(t, store, "T3_2", "14.3.1", 440, "Lux", false) // different queue
	seedMatch(t, store, "T3_3", "14.4.1", 420, "Lux", false) // different patch

	if err := store.RefreshChampionStats(ctx); err != nil {
		t.Fatalf("RefreshChampionStats: %v", err)
	}

	stats, err := store.TierList(ctx, 420, "14.3.1", 1)
	if err != nil {
		t.Fatalf("TierList: %v", err)
	}
	if len(stats) != 1 || stats[0].Games != 1 || stats[0].WinRate != 100 {
		t.Fatalf("expected only the matching queue+patch row, got %+v", stats)
	}
}

func TestLatestPatch(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	patch, err := store.LatestPatch(ctx)
	if err != nil {
		t.Fatalf("LatestPatch (empty db): %v", err)
	}
	if patch != "" {
		t.Fatalf("expected empty patch on empty db, got %q", patch)
	}

	seedMatch(t, store, "T4_1", "14.5.1", 420, "Ashe", true)

	patch, err = store.LatestPatch(ctx)
	if err != nil {
		t.Fatalf("LatestPatch: %v", err)
	}
	if patch != "14.5.1" {
		t.Fatalf("LatestPatch = %q, want 14.5.1", patch)
	}
}
