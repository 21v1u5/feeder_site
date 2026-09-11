package profile

import "github.com/21v1u5/feeder_site/services/api/internal/riot"

// Profile is the aggregated payload returned to the frontend after the
// fan-out/fan-in across SUMMONER-V4, LEAGUE-V4 and MATCH-V5.
type Profile struct {
	Account        *riot.Account      `json:"account"`
	Summoner       *riot.Summoner     `json:"summoner"`
	Leagues        []riot.LeagueEntry `json:"leagues"`
	RecentMatchIDs []string           `json:"recentMatchIds"`
}
