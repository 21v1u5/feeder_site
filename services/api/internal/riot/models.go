package riot

// Account is the ACCOUNT-V1 response (regional routing: americas/europe/asia).
type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// Summoner is the SUMMONER-V4 response (platform routing: na1/euw1/kr/...).
type Summoner struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	PUUID         string `json:"puuid"`
	ProfileIconID int    `json:"profileIconId"`
	RevisionDate  int64  `json:"revisionDate"`
	SummonerLevel int64  `json:"summonerLevel"`
}

// LeagueEntry is one entry from LEAGUE-V4 (ranked queue standing).
type LeagueEntry struct {
	LeagueID     string `json:"leagueId"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	SummonerID   string `json:"summonerId"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
}

// Match is a trimmed-down MATCH-V5 response, keeping only the fields the
// stats pipeline needs.
type Match struct {
	Metadata MatchMetadata `json:"metadata"`
	Info     MatchInfo     `json:"info"`
}

type MatchMetadata struct {
	MatchID      string   `json:"matchId"`
	Participants []string `json:"participants"` // PUUIDs
}

type MatchInfo struct {
	GameCreation int64              `json:"gameCreation"`
	GameDuration int64              `json:"gameDuration"`
	GameVersion  string             `json:"gameVersion"`
	QueueID      int                `json:"queueId"`
	Participants []MatchParticipant `json:"participants"`
}

type MatchParticipant struct {
	PUUID        string `json:"puuid"`
	SummonerName string `json:"summonerName"`
	ChampionName string `json:"championName"`
	TeamPosition string `json:"teamPosition"`
	Win          bool   `json:"win"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
}
