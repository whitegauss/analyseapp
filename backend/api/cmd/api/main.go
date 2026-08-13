// Command api runs the Go API Gateway/BFF service described in PDR.md.
package main

import (
	"context"
	"net/http"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/config"
	"analyseapp/api/internal/db"
	"analyseapp/api/internal/httpserver"
	"analyseapp/api/internal/logging"
)

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

	router := httpserver.NewRouter(dbPool, jwks)

	log.Info().Str("port", cfg.Port).Msg("starting api server")
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal().Err(err).Msg("api server stopped")
	}
}
