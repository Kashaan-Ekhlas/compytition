package main

import (
	"flag"
	"log/slog"
	"os"

	"compytition/internal/db"
)

func main() {
	down := flag.Bool("down", false, "roll back all migrations")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	var err error
	if *down {
		err = db.RollbackMigrations(url)
	} else {
		err = db.RunMigrations(url)
	}
	if err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations complete", "direction", map[bool]string{true: "down", false: "up"}[*down])
}
