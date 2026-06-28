package config

import (
	"log"
	"os"
)

// Config holds all runtime configuration read from environment variables.
type Config struct {
	Addr            string
	DatabaseURL     string
	SessionHashKey  string
	SessionBlockKey string
}

// Load reads configuration from environment variables.
func Load() Config {
	c := Config{
		Addr:            getEnv("BUH_ADDR", ":8080"),
		DatabaseURL:     getEnv("BUH_DATABASE_URL", ""),
		SessionHashKey:  getEnv("BUH_SESSION_HASH_KEY", ""),
		SessionBlockKey: getEnv("BUH_SESSION_BLOCK_KEY", ""),
	}
	if c.DatabaseURL == "" {
		log.Fatal("config: BUH_DATABASE_URL is required")
	}
	return c
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
