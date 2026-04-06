package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alicenstar/astrid/internal/config"
	"github.com/alicenstar/astrid/internal/database"
	"github.com/alicenstar/astrid/internal/handlers"
)

func main() {
	cfg := config.Load()

	db, err := database.ConnectPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	rdb, err := database.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	healthHandler := handlers.NewHealthHandler(
		handlers.NewPgPinger(db),
		handlers.PingerFunc(func() error {
			return rdb.Ping(context.Background()).Err()
		}),
	)
	r.Get("/healthz", healthHandler.ServeHTTP)

	// Routes will be added by feature tasks

	log.Printf("Astrid listening on %s", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), r); err != nil {
		log.Fatal(err)
	}
}
