// Package profile implements the fan-out/fan-in orchestration behind a
// player profile lookup: resolve the Riot ID to a PUUID, then fetch
// summoner, league and recent match data concurrently.
package profile

import (
	"context"
	"fmt"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
	"golang.org/x/sync/errgroup"
)

const recentMatchCount = 20

// RiotClient is the subset of *riot.Client the service depends on, kept as
// an interface so the fan-out orchestration can be unit tested without HTTP.
type RiotClient interface {
	GetAccountByRiotID(ctx context.Context, region, gameName, tagLine string) (*riot.Account, error)
	GetSummonerByPUUID(ctx context.Context, platform, puuid string) (*riot.Summoner, error)
	GetLeagueEntriesByPUUID(ctx context.Context, platform, puuid string) ([]riot.LeagueEntry, error)
	GetMatchIDsByPUUID(ctx context.Context, region, puuid string, start, count int) ([]string, error)
}

type Service struct {
	client RiotClient
}

func NewService(client RiotClient) *Service {
	return &Service{client: client}
}

// GetProfile resolves gameName#tagLine on the given platform (e.g. "na1")
// and fans out to summoner, league and match-history lookups in parallel.
func (svc *Service) GetProfile(ctx context.Context, platform, gameName, tagLine string) (*Profile, error) {
	region, err := riot.PlatformToRegion(platform)
	if err != nil {
		return nil, err
	}

	account, err := svc.client.GetAccountByRiotID(ctx, region, gameName, tagLine)
	if err != nil {
		return nil, fmt.Errorf("resolving riot id: %w", err)
	}

	var (
		summoner *riot.Summoner
		leagues  []riot.LeagueEntry
		matchIDs []string
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		s, err := svc.client.GetSummonerByPUUID(gctx, platform, account.PUUID)
		if err != nil {
			return fmt.Errorf("summoner lookup: %w", err)
		}
		summoner = s
		return nil
	})

	g.Go(func() error {
		l, err := svc.client.GetLeagueEntriesByPUUID(gctx, platform, account.PUUID)
		if err != nil {
			return fmt.Errorf("league entries lookup: %w", err)
		}
		leagues = l
		return nil
	})

	g.Go(func() error {
		ids, err := svc.client.GetMatchIDsByPUUID(gctx, region, account.PUUID, 0, recentMatchCount)
		if err != nil {
			return fmt.Errorf("match history lookup: %w", err)
		}
		matchIDs = ids
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &Profile{
		Account:        account,
		Summoner:       summoner,
		Leagues:        leagues,
		RecentMatchIDs: matchIDs,
	}, nil
}
