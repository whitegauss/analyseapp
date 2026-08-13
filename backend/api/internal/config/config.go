// Package config loads runtime configuration from environment variables.
package config

import "os"

// Config holds all environment-derived settings for the API service.
type Config struct {
	Port          string
	RedisAddr     string
	WorkerBaseURL string
	DatabaseURL   string
	SupabaseURL   string
}

// Load reads configuration from environment variables, falling back to
// local-development defaults when a variable is not set.
func Load() Config {
	return Config{
		Port:          getEnv("API_PORT", "8080"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		WorkerBaseURL: getEnv("WORKER_BASE_URL", "http://localhost:8001"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		SupabaseURL:   getEnv("SUPABASE_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
