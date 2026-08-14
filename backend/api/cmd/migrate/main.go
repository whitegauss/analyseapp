// Command migrate applies the SQL migrations in internal/migrations against
// DATABASE_URL. Run separately from the api server (not on every pod boot)
// so multiple replicas don't race to apply schema changes.
package main

import (
	"database/sql"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"analyseapp/api/internal/config"
	"analyseapp/api/internal/migrations"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, "sql"); err != nil {
		log.Fatalf("migrate up: %v", err)
	}
	log.Println("migrations applied")
}
