package main

import (
	"log"

	"github.com/cleeryy/clarr/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "clarr"})
	})

	addr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("clarr starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
