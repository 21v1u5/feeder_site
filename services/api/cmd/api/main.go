package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/21v1u5/feeder_site/services/api/internal/config"
	"github.com/21v1u5/feeder_site/services/api/internal/ratelimit"
	"github.com/21v1u5/feeder_site/services/api/internal/riot"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
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

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Manual smoke-test endpoint for the Riot client + rate limiter wiring.
	// The fan-out/fan-in profile endpoint (summoner + league + matches) lands
	// in a later stage.
	router.GET("/riot/account/:region/:gameName/:tagLine", func(c *gin.Context) {
		account, err := riotClient.GetAccountByRiotID(
			c.Request.Context(),
			c.Param("region"),
			c.Param("gameName"),
			c.Param("tagLine"),
		)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, riot.ErrNotFound) {
				status = http.StatusNotFound
			} else if errors.Is(err, riot.ErrRateLimited) {
				status = http.StatusTooManyRequests
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, account)
	})

	addr := ":" + cfg.Port
	log.Printf("feeder-site api listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
