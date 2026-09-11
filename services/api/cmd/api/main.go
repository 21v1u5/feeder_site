package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/21v1u5/feeder_site/services/api/internal/config"
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
	// summoner, league and recent match data concurrently.
	router.GET("/api/profiles/:platform/:gameName/:tagLine", func(c *gin.Context) {
		p, err := profileService.GetProfile(
			c.Request.Context(),
			c.Param("platform"),
			c.Param("gameName"),
			c.Param("tagLine"),
		)
		if err != nil {
			c.JSON(riotErrorStatus(err), gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
	})

	addr := ":" + cfg.Port
	log.Printf("feeder-site api listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// loadDotEnv loads a .env file if one is found, checking the current
// directory first (docker/systemd deployments typically run from the repo
// root) and then one level up (covers `cd services/api && go run ./cmd/api`
// with a .env kept at the repo root). Missing files are not an error: in
// production the environment is usually injected directly.
func loadDotEnv() {
	if err := godotenv.Load(".env"); err == nil {
		return
	}
	if err := godotenv.Load("../.env"); err == nil {
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
