package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

// Store persists accounts, matches and match participants. It implements
// ingestion.MatchStore.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// AccountRecord is the subset of profile data worth persisting per account.
type AccountRecord struct {
	PUUID         string
	GameName      string
	TagLine       string
	Platform      string
	Region        string
	ProfileIconID int
	SummonerLevel int64
}

// UpsertAccount records the latest known account/summoner snapshot.
func (s *Store) UpsertAccount(ctx context.Context, a AccountRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO accounts (puuid, game_name, tag_line, platform, region, profile_icon_id, summoner_level)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (puuid) DO UPDATE SET
			game_name       = EXCLUDED.game_name,
			tag_line        = EXCLUDED.tag_line,
			platform        = EXCLUDED.platform,
			region          = EXCLUDED.region,
			profile_icon_id = EXCLUDED.profile_icon_id,
			summoner_level  = EXCLUDED.summoner_level,
			updated_at      = now()
	`, a.PUUID, a.GameName, a.TagLine, a.Platform, a.Region, a.ProfileIconID, a.SummonerLevel)
	if err != nil {
		return fmt.Errorf("postgres: upsert account: %w", err)
	}
	return nil
}

// SaveMatch upserts a match and all of its participants in one transaction,
// so a retried ingestion job (e.g. after a transient Riot API error) never
// leaves partial data behind. Implements ingestion.MatchStore.
func (s *Store) SaveMatch(ctx context.Context, match *riot.Match) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO matches (match_id, platform, game_creation, game_duration, game_version, queue_id)
		VALUES ($1, $2, to_timestamp($3::double precision / 1000), $4, $5, $6)
		ON CONFLICT (match_id) DO UPDATE SET
			game_duration = EXCLUDED.game_duration,
			game_version  = EXCLUDED.game_version,
			queue_id      = EXCLUDED.queue_id,
			ingested_at   = now()
	`,
		match.Metadata.MatchID,
		platformFromMatchID(match.Metadata.MatchID),
		match.Info.GameCreation,
		match.Info.GameDuration,
		match.Info.GameVersion,
		match.Info.QueueID,
	)
	if err != nil {
		return fmt.Errorf("postgres: upsert match %s: %w", match.Metadata.MatchID, err)
	}

	for _, p := range match.Info.Participants {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO match_participants
				(match_id, puuid, summoner_name, champion_name, team_position, win, kills, deaths, assists)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (match_id, puuid) DO UPDATE SET
				summoner_name = EXCLUDED.summoner_name,
				champion_name = EXCLUDED.champion_name,
				team_position = EXCLUDED.team_position,
				win           = EXCLUDED.win,
				kills         = EXCLUDED.kills,
				deaths        = EXCLUDED.deaths,
				assists       = EXCLUDED.assists
		`,
			match.Metadata.MatchID, p.PUUID, p.SummonerName, p.ChampionName, p.TeamPosition, p.Win, p.Kills, p.Deaths, p.Assists,
		)
		if err != nil {
			return fmt.Errorf("postgres: upsert participant %s in match %s: %w", p.PUUID, match.Metadata.MatchID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: commit match %s: %w", match.Metadata.MatchID, err)
	}
	return nil
}

// platformFromMatchID extracts the originating platform from a match id's
// "{PLATFORM}_{id}" format (e.g. "BR1_3281477985" -> "br1").
func platformFromMatchID(matchID string) string {
	if idx := strings.IndexByte(matchID, '_'); idx > 0 {
		return strings.ToLower(matchID[:idx])
	}
	return ""
}
