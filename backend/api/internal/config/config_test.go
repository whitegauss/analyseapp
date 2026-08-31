package config

import (
	"os"
	"testing"
)

// envKeys is every variable Load reads. Tests clear all of them first so a
// result never depends on what the developer happens to have exported.
var envKeys = []string{"API_PORT", "REDIS_ADDR", "WORKER_BASE_URL", "DATABASE_URL", "SUPABASE_URL"}

const (
	defaultPort   = "8080"
	defaultRedis  = "localhost:6379"
	defaultWorker = "http://localhost:8001"
)

// defaults is Load's result with nothing set. DatabaseURL and SupabaseURL fall
// back to "", which main.go degrades on rather than failing to start.
func defaults() Config {
	return Config{Port: defaultPort, RedisAddr: defaultRedis, WorkerBaseURL: defaultWorker}
}

// clearEnv unsets every key for the duration of the test. t.Setenv runs first
// so the original value -- including "was never set" -- is restored on cleanup;
// a bare os.Unsetenv would leak into the tests that follow.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range envKeys {
		t.Setenv(key, "restored-on-cleanup")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Run("unset variables fall back to the development defaults", func(t *testing.T) {
		clearEnv(t)

		if got, want := Load(), defaults(); got != want {
			t.Errorf("Load() = %+v, want %+v", got, want)
		}
	})

	// getEnv's condition is `ok && v != ""`, so an exported-but-empty variable
	// is indistinguishable from an unset one -- as happens under Compose, where
	// an unset host variable is passed through as "".
	t.Run("a variable set to the empty string falls back to the default too", func(t *testing.T) {
		for _, key := range envKeys {
			t.Setenv(key, "")
		}

		if got, want := Load(), defaults(); got != want {
			t.Errorf("Load() = %+v, want %+v", got, want)
		}
	})
}

func TestLoadReadsEachVariable(t *testing.T) {
	// One variable per case with the whole expected Config spelled out, so a
	// value landing in the wrong field fails instead of passing quietly.
	tests := []struct {
		name, key, value string
		want             Config
	}{
		{name: "API_PORT sets Port", key: "API_PORT", value: "9090",
			want: Config{Port: "9090", RedisAddr: defaultRedis, WorkerBaseURL: defaultWorker}},
		{name: "REDIS_ADDR sets RedisAddr", key: "REDIS_ADDR", value: "redis:6379",
			want: Config{Port: defaultPort, RedisAddr: "redis:6379", WorkerBaseURL: defaultWorker}},
		{name: "WORKER_BASE_URL sets WorkerBaseURL", key: "WORKER_BASE_URL", value: "http://worker:8001",
			want: Config{Port: defaultPort, RedisAddr: defaultRedis, WorkerBaseURL: "http://worker:8001"}},
		{name: "DATABASE_URL sets DatabaseURL", key: "DATABASE_URL", value: "postgres://db:5432/app",
			want: Config{Port: defaultPort, RedisAddr: defaultRedis, WorkerBaseURL: defaultWorker, DatabaseURL: "postgres://db:5432/app"}},
		{name: "SUPABASE_URL sets SupabaseURL", key: "SUPABASE_URL", value: "https://project.supabase.co",
			want: Config{Port: defaultPort, RedisAddr: defaultRedis, WorkerBaseURL: defaultWorker, SupabaseURL: "https://project.supabase.co"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(tt.key, tt.value)

			if got := Load(); got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
