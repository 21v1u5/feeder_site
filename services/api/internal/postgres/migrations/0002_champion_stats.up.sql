CREATE MATERIALIZED VIEW IF NOT EXISTS champion_stats_by_patch AS
SELECT
    m.game_version                                                      AS patch,
    m.queue_id                                                          AS queue_id,
    mp.champion_name                                                    AS champion_name,
    count(*)                                                            AS games,
    sum(CASE WHEN mp.win THEN 1 ELSE 0 END)                             AS wins,
    round(100.0 * sum(CASE WHEN mp.win THEN 1 ELSE 0 END) / count(*), 2) AS win_rate,
    round(avg(mp.kills), 2)                                             AS avg_kills,
    round(avg(mp.deaths), 2)                                            AS avg_deaths,
    round(avg(mp.assists), 2)                                           AS avg_assists
FROM match_participants mp
JOIN matches m ON m.match_id = mp.match_id
GROUP BY m.game_version, m.queue_id, mp.champion_name
WITH DATA;

-- Required for REFRESH MATERIALIZED VIEW CONCURRENTLY, which lets readers
-- keep querying the view while a refresh is in progress.
CREATE UNIQUE INDEX IF NOT EXISTS idx_champion_stats_by_patch_pk
    ON champion_stats_by_patch (patch, queue_id, champion_name);
