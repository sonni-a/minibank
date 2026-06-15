package db

import (
	"database/sql"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func Connect() *sql.DB {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		slog.Error("DATABASE_URL not set")
		os.Exit(1)
	}

	var db *sql.DB
	var err error

	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			slog.Warn("failed to open db", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = db.Ping()
		if err == nil {
			slog.Info("connected to db")
			return db
		}

		slog.Warn("db not ready, retrying", "error", err)
		time.Sleep(2 * time.Second)
	}

	slog.Error("could not connect to db", "error", err)
	os.Exit(1)
	return nil
}
