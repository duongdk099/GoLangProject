package config

import (
	"os"
	"time"
)

type Config struct {
	Address      string
	DatabaseURL  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Load() Config {
	port := envOrDefault("PORT", "8080")
	return Config{
		Address:      ":" + port,
		DatabaseURL:  envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
