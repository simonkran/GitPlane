// Package main is the entrypoint for the GitPlane API server.
package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/simonkran/gitplane/api"
)

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

	srv := api.NewServer(db)
	log.Printf("starting GitPlane API server on :%s", port)
	if err := srv.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
