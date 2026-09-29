//go:build integration

package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
)

func TestRun_Integration(t *testing.T) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		t.Skip("DATABASE_URL not set")
	}

	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer dbConn.Close()

	if err := dbConn.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	dbDir := filepath.Join("..", "..", "auth-service", "internal", "db")
	Run(dbConn, os.DirFS(dbDir))

	var exists bool
	err = dbConn.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'auth_users'
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("query table existence: %v", err)
	}
	if !exists {
		t.Fatal("expected auth_users table after migrations")
	}
}
