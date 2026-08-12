// Package config loads runtime configuration from environment variables.
package config

import "os"

// Config holds all environment-derived settings for the API service.
type Config struct {
	Port               string
	RedisAddr          string
	WorkerBaseURL      string
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseServiceKey string
}

// Load reads configuration from environment variables, falling back to
// local-development defaults when a variable is not set.
func Load() Config {
	return Config{
		Port:               getEnv("API_PORT", "8080"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		WorkerBaseURL:      getEnv("WORKER_BASE_URL", "http://localhost:8001"),
		SupabaseURL:        getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:    getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceKey: getEnv("SUPABASE_SERVICE_ROLE_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
