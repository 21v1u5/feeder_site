package main

import (
	"log"

	"github.com/21v1u5/feeder_site/services/api/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	addr := ":" + cfg.Port
	log.Printf("feeder-site api listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
