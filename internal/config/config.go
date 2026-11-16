// Package config loads runtime configuration from the environment.
package config

import (
	"os"
	"strconv"
)

// Config mirrors the environment variables used by the Node version.
type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBTimezone string
	Port       string
}

// Load reads configuration from the environment, applying the same
// defaults as src/config/db.js and index.js.
func Load() Config {
	return Config{
		DBHost:     env("DB_HOST", "localhost"),
		DBPort:     envInt("DB_PORT", 3306),
		DBUser:     env("DB_USERNAME", "root"),
		DBPassword: env("DB_PASSWORD", "root"),
		DBName:     env("DB_DATABASE", "sacrop"),
		DBTimezone: env("DB_TIMEZONE", "UTC"),
		Port:       env("PORT", "3000"),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v == 0 {
		return fallback
	}
	return v
}
