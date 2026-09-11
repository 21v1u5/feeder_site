// Package ingestion implements background ingestion of full match details:
// producers enqueue match ids as soon as a profile is looked up, and a
// worker pool consumes them from RabbitMQ, fetching MATCH-V5 data (through
// the same rate-limited Riot client) without blocking the profile response.
package ingestion

// MatchJob is the message body queued for each match id worth ingesting.
type MatchJob struct {
	Region  string `json:"region"`
	MatchID string `json:"matchId"`
}
