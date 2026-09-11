package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

// requires a real Postgres reachable at POSTGRES_TEST_URL; skipped otherwise
// so `go test ./...` stays hermetic without a database.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping Postgres integration test")
	}

	db, err := Open(url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	t.Cleanup(func() {
		db.Exec("TRUNCATE match_participants, matches, accounts")
	})
	if _, err := db.Exec("TRUNCATE match_participants, matches, accounts"); err != nil {
		t.Fatalf("truncate before test: %v", err)
	}

	return db
}

func sampleMatch(matchID string) *riot.Match {
	return &riot.Match{
		Metadata: riot.MatchMetadata{
			MatchID:      matchID,
			Participants: []string{"puuid-1", "puuid-2"},
		},
		Info: riot.MatchInfo{
			GameCreation: 1700000000000,
			GameDuration: 1800,
			GameVersion:  "14.1.1",
			QueueID:      420,
			Participants: []riot.MatchParticipant{
				{PUUID: "puuid-1", SummonerName: "Alice", ChampionName: "Ahri", TeamPosition: "MIDDLE", Win: true, Kills: 10, Deaths: 2, Assists: 8},
				{PUUID: "puuid-2", SummonerName: "Bob", ChampionName: "Garen", TeamPosition: "TOP", Win: false, Kills: 3, Deaths: 5, Assists: 2},
			},
		},
	}
}

func TestSaveMatch_PersistsMatchAndParticipants(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	match := sampleMatch("BR1_1000000001")
	if err := store.SaveMatch(ctx, match); err != nil {
		t.Fatalf("SaveMatch: %v", err)
	}

	var platform, version string
	var duration, queueID int
	if err := db.QueryRow(
		"SELECT platform, game_duration, game_version, queue_id FROM matches WHERE match_id = $1",
		match.Metadata.MatchID,
	).Scan(&platform, &duration, &version, &queueID); err != nil {
		t.Fatalf("query match: %v", err)
	}
	if platform != "br1" || duration != 1800 || version != "14.1.1" || queueID != 420 {
		t.Fatalf("unexpected match row: platform=%s duration=%d version=%s queueID=%d", platform, duration, version, queueID)
	}

	var participantCount int
	if err := db.QueryRow(
		"SELECT count(*) FROM match_participants WHERE match_id = $1", match.Metadata.MatchID,
	).Scan(&participantCount); err != nil {
		t.Fatalf("count participants: %v", err)
	}
	if participantCount != 2 {
		t.Fatalf("participant count = %d, want 2", participantCount)
	}

	var championName string
	var win bool
	var kills int
	if err := db.QueryRow(
		"SELECT champion_name, win, kills FROM match_participants WHERE match_id = $1 AND puuid = $2",
		match.Metadata.MatchID, "puuid-1",
	).Scan(&championName, &win, &kills); err != nil {
		t.Fatalf("query participant: %v", err)
	}
	if championName != "Ahri" || !win || kills != 10 {
		t.Fatalf("unexpected participant row: champion=%s win=%v kills=%d", championName, win, kills)
	}
}

func TestSaveMatch_UpsertIsIdempotent(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	match := sampleMatch("BR1_1000000002")
	if err := store.SaveMatch(ctx, match); err != nil {
		t.Fatalf("first SaveMatch: %v", err)
	}
	// Simulate a requeued/retried ingestion job for the same match.
	if err := store.SaveMatch(ctx, match); err != nil {
		t.Fatalf("second SaveMatch: %v", err)
	}

	var matchCount, participantCount int
	db.QueryRow("SELECT count(*) FROM matches WHERE match_id = $1", match.Metadata.MatchID).Scan(&matchCount)
	db.QueryRow("SELECT count(*) FROM match_participants WHERE match_id = $1", match.Metadata.MatchID).Scan(&participantCount)

	if matchCount != 1 {
		t.Fatalf("matchCount = %d, want 1 (upsert should not duplicate)", matchCount)
	}
	if participantCount != 2 {
		t.Fatalf("participantCount = %d, want 2 (upsert should not duplicate)", participantCount)
	}
}

func TestUpsertAccount(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	account := AccountRecord{
		PUUID: "puuid-account-1", GameName: "Just A FEEDER", TagLine: "FLS",
		Platform: "br1", Region: "americas", ProfileIconID: 7, SummonerLevel: 534,
	}
	if err := store.UpsertAccount(ctx, account); err != nil {
		t.Fatalf("first UpsertAccount: %v", err)
	}

	account.SummonerLevel = 535 // simulate levelling up on a re-lookup
	if err := store.UpsertAccount(ctx, account); err != nil {
		t.Fatalf("second UpsertAccount: %v", err)
	}

	var count int
	var level int64
	db.QueryRow("SELECT count(*) FROM accounts WHERE puuid = $1", account.PUUID).Scan(&count)
	db.QueryRow("SELECT summoner_level FROM accounts WHERE puuid = $1", account.PUUID).Scan(&level)

	if count != 1 {
		t.Fatalf("account rows = %d, want 1 (upsert should not duplicate)", count)
	}
	if level != 535 {
		t.Fatalf("summoner_level = %d, want updated value 535", level)
	}
}
