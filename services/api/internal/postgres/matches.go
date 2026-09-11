package postgres

import (
	"context"
	"fmt"
	"time"
)

// MatchSummary is one persisted match from a given player's perspective,
// joining matches with their row in match_participants.
type MatchSummary struct {
	MatchID      string    `json:"matchId"`
	Platform     string    `json:"platform"`
	GameCreation time.Time `json:"gameCreation"`
	GameDuration int       `json:"gameDuration"`
	GameVersion  string    `json:"gameVersion"`
	QueueID      int       `json:"queueId"`
	ChampionName string    `json:"championName"`
	Win          bool      `json:"win"`
	Kills        int       `json:"kills"`
	Deaths       int       `json:"deaths"`
	Assists      int       `json:"assists"`
}

// MatchHistoryByPUUID returns the most recent ingested matches for a
// player, newest first. Only matches the background workers have already
// fetched show up here - a just-looked-up profile may show none yet.
func (s *Store) MatchHistoryByPUUID(ctx context.Context, puuid string, limit int) ([]MatchSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.match_id, m.platform, m.game_creation, m.game_duration, m.game_version, m.queue_id,
		       mp.champion_name, mp.win, mp.kills, mp.deaths, mp.assists
		FROM match_participants mp
		JOIN matches m ON m.match_id = mp.match_id
		WHERE mp.puuid = $1
		ORDER BY m.game_creation DESC
		LIMIT $2
	`, puuid, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query match history: %w", err)
	}
	defer rows.Close()

	var matches []MatchSummary
	for rows.Next() {
		var m MatchSummary
		if err := rows.Scan(
			&m.MatchID, &m.Platform, &m.GameCreation, &m.GameDuration, &m.GameVersion, &m.QueueID,
			&m.ChampionName, &m.Win, &m.Kills, &m.Deaths, &m.Assists,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan match history row: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate match history rows: %w", err)
	}
	return matches, nil
}
