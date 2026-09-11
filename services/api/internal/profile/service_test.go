package profile

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/21v1u5/feeder_site/services/api/internal/riot"
)

// fakeClient lets tests control latency and errors per method, to assert
// both the aggregated result and the fan-out concurrency/error semantics.
type fakeClient struct {
	accountDelay, summonerDelay, leagueDelay, matchIDsDelay time.Duration
	accountErr, summonerErr, leagueErr, matchIDsErr         error

	summonerCalls int32
	leagueCalls   int32
	matchIDsCalls int32
}

func (f *fakeClient) GetAccountByRiotID(ctx context.Context, region, gameName, tagLine string) (*riot.Account, error) {
	if f.accountDelay > 0 {
		time.Sleep(f.accountDelay)
	}
	if f.accountErr != nil {
		return nil, f.accountErr
	}
	return &riot.Account{PUUID: "puuid-123", GameName: gameName, TagLine: tagLine}, nil
}

func (f *fakeClient) GetSummonerByPUUID(ctx context.Context, platform, puuid string) (*riot.Summoner, error) {
	atomic.AddInt32(&f.summonerCalls, 1)
	if f.summonerDelay > 0 {
		time.Sleep(f.summonerDelay)
	}
	if f.summonerErr != nil {
		return nil, f.summonerErr
	}
	return &riot.Summoner{PUUID: puuid, SummonerLevel: 200}, nil
}

func (f *fakeClient) GetLeagueEntriesByPUUID(ctx context.Context, platform, puuid string) ([]riot.LeagueEntry, error) {
	atomic.AddInt32(&f.leagueCalls, 1)
	if f.leagueDelay > 0 {
		time.Sleep(f.leagueDelay)
	}
	if f.leagueErr != nil {
		return nil, f.leagueErr
	}
	return []riot.LeagueEntry{{QueueType: "RANKED_SOLO_5x5", Tier: "GOLD"}}, nil
}

func (f *fakeClient) GetMatchIDsByPUUID(ctx context.Context, region, puuid string, start, count int) ([]string, error) {
	atomic.AddInt32(&f.matchIDsCalls, 1)
	if f.matchIDsDelay > 0 {
		time.Sleep(f.matchIDsDelay)
	}
	if f.matchIDsErr != nil {
		return nil, f.matchIDsErr
	}
	return []string{"NA1_1", "NA1_2"}, nil
}

func TestGetProfile_Success(t *testing.T) {
	svc := NewService(&fakeClient{})

	profile, err := svc.GetProfile(context.Background(), "na1", "Faker", "KR1")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Account.PUUID != "puuid-123" {
		t.Errorf("account.puuid = %q", profile.Account.PUUID)
	}
	if profile.Summoner.SummonerLevel != 200 {
		t.Errorf("summoner.level = %d", profile.Summoner.SummonerLevel)
	}
	if len(profile.Leagues) != 1 || profile.Leagues[0].Tier != "GOLD" {
		t.Errorf("unexpected leagues: %+v", profile.Leagues)
	}
	if len(profile.RecentMatchIDs) != 2 {
		t.Errorf("unexpected match ids: %+v", profile.RecentMatchIDs)
	}
}

func TestGetProfile_FansOutConcurrently(t *testing.T) {
	const delay = 150 * time.Millisecond
	fc := &fakeClient{summonerDelay: delay, leagueDelay: delay, matchIDsDelay: delay}
	svc := NewService(fc)

	start := time.Now()
	if _, err := svc.GetProfile(context.Background(), "na1", "Faker", "KR1"); err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	elapsed := time.Since(start)

	// Sequential calls would take ~3*delay; fanning out should take ~1*delay.
	if elapsed >= 2*delay {
		t.Fatalf("expected concurrent fan-out (~%v), took %v", delay, elapsed)
	}
}

func TestGetProfile_AccountFailureShortCircuits(t *testing.T) {
	fc := &fakeClient{accountErr: riot.ErrNotFound}
	svc := NewService(fc)

	_, err := svc.GetProfile(context.Background(), "na1", "Unknown", "0000")
	if !errors.Is(err, riot.ErrNotFound) {
		t.Fatalf("expected wrapped ErrNotFound, got %v", err)
	}
	if fc.summonerCalls != 0 || fc.leagueCalls != 0 || fc.matchIDsCalls != 0 {
		t.Fatalf("expected no fan-out calls after account resolution failed, got summoner=%d league=%d matchIDs=%d",
			fc.summonerCalls, fc.leagueCalls, fc.matchIDsCalls)
	}
}

func TestGetProfile_PropagatesFanOutError(t *testing.T) {
	fc := &fakeClient{leagueErr: riot.ErrRateLimited}
	svc := NewService(fc)

	_, err := svc.GetProfile(context.Background(), "na1", "Faker", "KR1")
	if !errors.Is(err, riot.ErrRateLimited) {
		t.Fatalf("expected wrapped ErrRateLimited, got %v", err)
	}
}

func TestGetProfile_UnknownPlatform(t *testing.T) {
	fc := &fakeClient{}
	svc := NewService(fc)

	_, err := svc.GetProfile(context.Background(), "not-a-platform", "Faker", "KR1")
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
	if fc.summonerCalls != 0 {
		t.Fatal("expected no Riot calls for an unknown platform")
	}
}
