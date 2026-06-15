package redis

import (
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestConnect(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	client := Connect(mr.Addr())
	defer client.Close()

	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}
