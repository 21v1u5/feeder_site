package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/21v1u5/feeder_site/services/api/internal/config"
	"github.com/21v1u5/feeder_site/services/api/internal/ingestion"
	"github.com/21v1u5/feeder_site/services/api/internal/postgres"
	"github.com/21v1u5/feeder_site/services/api/internal/profile"
	"github.com/21v1u5/feeder_site/services/api/internal/ratelimit"
	"github.com/21v1u5/feeder_site/services/api/internal/riot"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	loadDotEnv()
	cfg := config.Load()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	windows, err := ratelimit.ParseWindows(cfg.RiotRateLimitSpec)
	if err != nil {
		log.Fatalf("invalid RIOT_RATE_LIMIT_WINDOWS: %v", err)
	}
	limiter := ratelimit.New(rdb, "riot-rl", windows)

	riotClient := riot.NewClient(cfg.RiotAPIKey, limiter)
	profileService := profile.NewService(riotClient)

	db, err := postgres.Open(cfg.PostgresURL)
	if err != nil {
		log.Fatalf("connecting to Postgres: %v", err)
	}
	defer db.Close()
	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("running Postgres migrations: %v", err)
	}
	store := postgres.NewStore(db)

	ingestionQueue, err := ingestion.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("connecting to RabbitMQ: %v", err)
	}
	defer ingestionQueue.Close()

	ingestionWorker := ingestion.NewWorker(riotClient, store)
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	go func() {
		if err := ingestionQueue.Consume(workerCtx, cfg.IngestionWorkers, ingestionWorker.HandleJob); err != nil {
			log.Printf("ingestion consumer stopped: %v", err)
		}
	}()

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Manual smoke-test endpoint for the Riot client + rate limiter wiring.
	router.GET("/riot/account/:region/:gameName/:tagLine", func(c *gin.Context) {
		account, err := riotClient.GetAccountByRiotID(
			c.Request.Context(),
			c.Param("region"),
			c.Param("gameName"),
			c.Param("tagLine"),
		)
		if err != nil {
			c.JSON(riotErrorStatus(err), gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, account)
	})

	// Fan-out/fan-in profile lookup: resolves the Riot ID and fetches
	// summoner, league and recent match data concurrently. Full match
	// details are enqueued for background ingestion rather than fetched
	// inline, so the response stays fast regardless of history size.
	router.GET("/api/profiles/:platform/:gameName/:tagLine", func(c *gin.Context) {
		platform := c.Param("platform")
		p, err := profileService.GetProfile(
			c.Request.Context(),
			platform,
			c.Param("gameName"),
			c.Param("tagLine"),
		)
		if err != nil {
			c.JSON(riotErrorStatus(err), gin.H{"error": err.Error()})
			return
		}

		if region, err := riot.PlatformToRegion(platform); err == nil {
			go persistAccountAndEnqueueMatches(store, ingestionQueue, platform, region, p)
		}

		c.JSON(http.StatusOK, p)
	})

	addr := ":" + cfg.Port
	log.Printf("feeder-site api listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// persistAccountAndEnqueueMatches upserts the account/summoner snapshot and
// publishes one ingestion job per recent match id. It runs detached from
// the HTTP request (own background context with a timeout) since the
// request is already done by the time this executes.
func persistAccountAndEnqueueMatches(store *postgres.Store, queue *ingestion.Queue, platform, region string, p *profile.Profile) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if p.Summoner != nil {
		account := postgres.AccountRecord{
			PUUID:         p.Account.PUUID,
			GameName:      p.Account.GameName,
			TagLine:       p.Account.TagLine,
			Platform:      platform,
			Region:        region,
			ProfileIconID: p.Summoner.ProfileIconID,
			SummonerLevel: p.Summoner.SummonerLevel,
		}
		if err := store.UpsertAccount(ctx, account); err != nil {
			log.Printf("persisting account %s: %v", p.Account.PUUID, err)
		}
	}

	for _, matchID := range p.RecentMatchIDs {
		job := ingestion.MatchJob{Region: region, MatchID: matchID}
		if err := queue.Publish(ctx, job); err != nil {
			log.Printf("enqueue match ingestion for %s failed: %v", matchID, err)
		}
	}
}

// loadDotEnv loads a .env file if one is found, checking the current
// directory first (docker/systemd deployments typically run from the repo
// root) and then two levels up (covers `cd services/api && go run ./cmd/api`
// with a .env kept at the repo root: services/api -> services -> repo root).
// Missing files are not an error: in production the environment is usually
// injected directly.
func loadDotEnv() {
	if err := godotenv.Load(".env"); err == nil {
		return
	}
	if err := godotenv.Load("../../.env"); err == nil {
		return
	}
	log.Println("no .env file found, relying on process environment")
}

func riotErrorStatus(err error) int {
	switch {
	case errors.Is(err, riot.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, riot.ErrRateLimited):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}
