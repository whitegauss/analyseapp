// Command api runs the Go API Gateway/BFF service described in PDR.md.
package main

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"analyseapp/api/internal/config"
	"analyseapp/api/internal/httpserver"
	"analyseapp/api/internal/logging"
)

func main() {
	logging.Init()
	cfg := config.Load()

	router := httpserver.NewRouter()

	log.Info().Str("port", cfg.Port).Msg("starting api server")
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal().Err(err).Msg("api server stopped")
	}
}
