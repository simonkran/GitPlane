// Package main is the entrypoint for the GitPlane API server.
package main

import (
	"database/sql"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/simonkran/gitplane/api"
	"github.com/simonkran/gitplane/migrations"
)

func runMigrations(db *sql.DB) error {
	// Create tracking table if it doesn't exist.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return err
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		// Only apply "up" migrations.
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		data, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}

		log.Printf("applying migration: %s", name)
		if _, err := db.Exec(string(data)); err != nil {
			return err
		}

		if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost:5432/gitplane?sslmode=disable"
	}

	jwtSecret := os.Getenv("GITPLANE_JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("GITPLANE_JWT_SECRET environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	log.Printf("connected to database")

	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	srv := api.NewServer(db)
	log.Printf("starting GitPlane API server on :%s", port)
	if err := srv.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
