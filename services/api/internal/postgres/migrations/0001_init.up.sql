CREATE TABLE IF NOT EXISTS accounts (
    puuid           TEXT PRIMARY KEY,
    game_name       TEXT NOT NULL,
    tag_line        TEXT NOT NULL,
    platform        TEXT NOT NULL,
    region          TEXT NOT NULL,
    profile_icon_id INT NOT NULL,
    summoner_level  BIGINT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS matches (
    match_id      TEXT PRIMARY KEY,
    platform      TEXT NOT NULL,
    game_creation TIMESTAMPTZ NOT NULL,
    game_duration INT NOT NULL,
    game_version  TEXT NOT NULL,
    queue_id      INT NOT NULL,
    ingested_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS match_participants (
    match_id      TEXT NOT NULL REFERENCES matches (match_id) ON DELETE CASCADE,
    puuid         TEXT NOT NULL,
    summoner_name TEXT NOT NULL,
    champion_name TEXT NOT NULL,
    team_position TEXT NOT NULL,
    win           BOOLEAN NOT NULL,
    kills         INT NOT NULL,
    deaths        INT NOT NULL,
    assists       INT NOT NULL,
    PRIMARY KEY (match_id, puuid)
);

CREATE INDEX IF NOT EXISTS idx_match_participants_puuid ON match_participants (puuid);
CREATE INDEX IF NOT EXISTS idx_match_participants_champion ON match_participants (champion_name);
CREATE INDEX IF NOT EXISTS idx_matches_queue_version ON matches (queue_id, game_version);
