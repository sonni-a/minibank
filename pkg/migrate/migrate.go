package migrate

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func Run(dbConn *sql.DB, fsys fs.FS) {
	driver, err := postgres.WithInstance(dbConn, &postgres.Config{})
	if err != nil {
		slog.Error("failed to create migrate driver", "error", err)
		os.Exit(1)
	}

	src, err := iofs.New(fsys, "migrations")
	if err != nil {
		slog.Error("failed to read migrations", "error", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		slog.Error("failed to create migrate instance", "error", err)
		os.Exit(1)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations applied")
}
