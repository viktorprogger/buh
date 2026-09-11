package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// Config holds all runtime configuration read from environment variables.
type Config struct {
	Addr                string
	DatabaseURL         string
	SessionHashKey      string
	SessionBlockKey     string
	VisitLoggingEnabled bool
	visitLoggingValue   string
}

// Load reads configuration from environment variables.
func Load() Config {
	c := Config{
		Addr:              getEnv("BUH_ADDR", ":8080"),
		DatabaseURL:       getEnv("BUH_DATABASE_URL", ""),
		SessionHashKey:    getEnv("BUH_SESSION_HASH_KEY", ""),
		SessionBlockKey:   getEnv("BUH_SESSION_BLOCK_KEY", ""),
		visitLoggingValue: getEnv("BUH_VISIT_LOGGING_ENABLED", "false"),
	}
	c.VisitLoggingEnabled, _ = strconv.ParseBool(c.visitLoggingValue)
	if c.DatabaseURL == "" {
		log.Fatal("config: BUH_DATABASE_URL is required")
	}
	return c
}

// ValidateVisitLogging validates the visit logging setting.
func (c Config) ValidateVisitLogging() error {
	if c.visitLoggingValue == "" {
		return nil
	}
	if _, err := strconv.ParseBool(c.visitLoggingValue); err != nil {
		return fmt.Errorf("BUH_VISIT_LOGGING_ENABLED must be true or false")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
