// Command api runs the Go API Gateway/BFF service described in PDR.md.
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/config"
	"analyseapp/api/internal/db"
	"analyseapp/api/internal/httpserver"
	"analyseapp/api/internal/logging"
	"analyseapp/api/internal/worker"
)

// workerTimeout bounds how long a single analysis request may occupy a
// handler.
const workerTimeout = 10 * time.Second

func main() {
	logging.Init()
	cfg := config.Load()

	var dbPool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create db pool")
		}
		defer pool.Close()
		dbPool = pool
	} else {
		log.Warn().Msg("DATABASE_URL not set; /readyz will report not-ready")
	}

	var jwks keyfunc.Keyfunc
	if cfg.SupabaseURL != "" {
		k, err := auth.NewJWKS(context.Background(), cfg.SupabaseURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to fetch Supabase JWKS")
		}
		jwks = k
	} else {
		log.Warn().Msg("SUPABASE_URL not set; /api/v1 routes will not be mounted")
	}

	// WORKER_BASE_URL has a default, so any value at all lets the process
	// start. Validated here so a broken one stops startup with the variable
	// named, instead of surfacing once per request as a 502 that reads as
	// "the worker is down" (KAN-62).
	workerClient, err := worker.NewHTTPClient(cfg.WorkerBaseURL, workerTimeout)
	if err != nil {
		log.Fatal().Err(err).Str("WORKER_BASE_URL", cfg.WorkerBaseURL).Msg("invalid worker base URL")
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close redis client")
		}
	}()
	resultCache := &cache.RedisCache{Client: redisClient}

	router := httpserver.NewRouter(dbPool, jwks, workerClient, resultCache)

	log.Info().Str("port", cfg.Port).Msg("starting api server")
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal().Err(err).Msg("api server stopped")
	}
}
