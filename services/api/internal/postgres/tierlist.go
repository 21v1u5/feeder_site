package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ChampionStat is one row of the champion_stats_by_patch materialized view:
// aggregated win rate and combat averages for a champion, in a given queue
// and patch.
type ChampionStat struct {
	Patch      string  `json:"patch"`
	QueueID    int     `json:"queueId"`
	Champion   string  `json:"champion"`
	Games      int     `json:"games"`
	Wins       int     `json:"wins"`
	WinRate    float64 `json:"winRate"`
	AvgKills   float64 `json:"avgKills"`
	AvgDeaths  float64 `json:"avgDeaths"`
	AvgAssists float64 `json:"avgAssists"`
}

// RefreshChampionStats recomputes the champion_stats_by_patch materialized
// view from the current matches/match_participants data. CONCURRENTLY keeps
// the previous snapshot readable to other queries while it runs, at the
// cost of requiring the unique index created alongside the view.
func (s *Store) RefreshChampionStats(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY champion_stats_by_patch"); err != nil {
		return fmt.Errorf("postgres: refresh champion_stats_by_patch: %w", err)
	}
	return nil
}

// LatestPatch returns the game_version of the most recently played ingested
// match, used as the default patch for tier list lookups when the caller
// doesn't pin one.
func (s *Store) LatestPatch(ctx context.Context) (string, error) {
	var patch string
	err := s.db.QueryRowContext(ctx,
		"SELECT game_version FROM matches ORDER BY game_creation DESC LIMIT 1",
	).Scan(&patch)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("postgres: latest patch: %w", err)
	}
	return patch, nil
}

// TierList returns champion stats for a queue/patch, ordered by win rate
// descending, keeping only champions with at least minGames sampled games
// so a single lucky/unlucky game doesn't skew the list.
func (s *Store) TierList(ctx context.Context, queueID int, patch string, minGames int) ([]ChampionStat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT patch, queue_id, champion_name, games, wins, win_rate, avg_kills, avg_deaths, avg_assists
		FROM champion_stats_by_patch
		WHERE queue_id = $1 AND patch = $2 AND games >= $3
		ORDER BY win_rate DESC
	`, queueID, patch, minGames)
	if err != nil {
		return nil, fmt.Errorf("postgres: query tier list: %w", err)
	}
	defer rows.Close()

	var stats []ChampionStat
	for rows.Next() {
		var stat ChampionStat
		if err := rows.Scan(
			&stat.Patch, &stat.QueueID, &stat.Champion, &stat.Games, &stat.Wins,
			&stat.WinRate, &stat.AvgKills, &stat.AvgDeaths, &stat.AvgAssists,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan tier list row: %w", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate tier list rows: %w", err)
	}
	return stats, nil
}
