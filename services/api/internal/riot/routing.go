package riot

import "fmt"

// platformToRegion maps a platform routing value (used by SUMMONER-V4 and
// LEAGUE-V4) to the regional routing value that hosts the same player's
// ACCOUNT-V1 and MATCH-V5 data.
var platformToRegion = map[string]string{
	"na1": "americas",
	"br1": "americas",
	"la1": "americas",
	"la2": "americas",
	"oc1": "americas",

	"euw1": "europe",
	"eun1": "europe",
	"tr1":  "europe",
	"ru":   "europe",

	"kr":  "asia",
	"jp1": "asia",
}

// PlatformToRegion resolves a platform routing value (e.g. "na1") to its
// regional routing value (e.g. "americas").
func PlatformToRegion(platform string) (string, error) {
	region, ok := platformToRegion[platform]
	if !ok {
		return "", fmt.Errorf("riot: unknown platform %q", platform)
	}
	return region, nil
}
