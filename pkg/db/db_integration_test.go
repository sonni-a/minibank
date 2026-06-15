//go:build integration

package db

import (
	"os"
	"testing"
)

func TestConnect_Integration(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}

	conn := Connect()
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}
